package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	sessionStore   *auth.SessionStore
	userStore      *auth.UserStore
	oidcProvider   *auth.OIDCProvider
	setupChecker   func() bool
	config         *config.Config
	configPath     string
	authMiddleware *auth.Middleware
	configMu       sync.Mutex
	bypassRules    []auth.BypassRule
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(sessionStore *auth.SessionStore, userStore *auth.UserStore,
	cfg *config.Config, configPath string, authMiddleware *auth.Middleware) *AuthHandler {
	return &AuthHandler{
		sessionStore:   sessionStore,
		userStore:      userStore,
		config:         cfg,
		configPath:     configPath,
		authMiddleware: authMiddleware,
	}
}

// SetBypassRules stores the bypass rules so UpdateAuthMethod can rebuild AuthConfig.
func (h *AuthHandler) SetBypassRules(rules []auth.BypassRule) {
	h.bypassRules = rules
}

// SetOIDCProvider sets the OIDC provider for OIDC authentication
func (h *AuthHandler) SetOIDCProvider(provider *auth.OIDCProvider) {
	h.oidcProvider = provider
}

// SetSetupChecker sets the function used to check if setup is required.
func (h *AuthHandler) SetSetupChecker(fn func() bool) {
	h.setupChecker = fn
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	RememberMe bool   `json:"remember_me"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Success bool          `json:"success"`
	Message string        `json:"message,omitempty"`
	User    *UserResponse `json:"user,omitempty"`
}

// UserResponse represents a user in API responses
type UserResponse struct {
	Username    string `json:"username"`
	Role        string `json:"role"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
}

// Login handles POST /api/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set(headerContentType, contentTypeJSON)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(LoginResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate input
	if req.Username == "" || req.Password == "" {
		w.Header().Set(headerContentType, contentTypeJSON)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(LoginResponse{
			Success: false,
			Message: "Username and password are required",
		})
		return
	}

	// Authenticate
	user, err := h.userStore.Authenticate(req.Username, req.Password)
	if err != nil {
		w.Header().Set(headerContentType, contentTypeJSON)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(LoginResponse{
			Success: false,
			Message: "Invalid username or password",
		})
		return
	}

	// Create session
	session, err := h.sessionStore.Create(user.ID, user.Username, user.Role)
	if err != nil {
		w.Header().Set(headerContentType, contentTypeJSON)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(LoginResponse{
			Success: false,
			Message: "Failed to create session",
		})
		return
	}

	// Set session cookie
	h.sessionStore.SetCookie(w, session)

	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(LoginResponse{
		Success: true,
		User: &UserResponse{
			Username:    user.Username,
			Role:        user.Role,
			Email:       user.Email,
			DisplayName: user.DisplayName,
		},
	})
}

// Logout handles POST /api/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	// Get current session
	session := h.sessionStore.GetFromRequest(r)
	if session != nil {
		h.sessionStore.Delete(session.ID)
	}

	// Clear cookie
	h.sessionStore.ClearCookie(w)

	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// Me handles GET /api/auth/me - returns current user info
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		w.Header().Set(headerContentType, contentTypeJSON)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"authenticated": false,
		})
		return
	}

	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"authenticated": true,
		"user": UserResponse{
			Username:    user.Username,
			Role:        user.Role,
			Email:       user.Email,
			DisplayName: user.DisplayName,
		},
	})
}

// ChangePassword handles POST /api/auth/password
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.NewPassword) < 8 {
		w.Header().Set(headerContentType, contentTypeJSON)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Password must be at least 8 characters",
		})
		return
	}

	// Verify current password
	_, err := h.userStore.Authenticate(user.Username, req.CurrentPassword)
	if err != nil {
		w.Header().Set(headerContentType, contentTypeJSON)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Current password is incorrect",
		})
		return
	}

	// Hash new password
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	// Update user
	fullUser := h.userStore.Get(user.Username)
	if fullUser == nil {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}
	fullUser.PasswordHash = hash
	if err := h.userStore.Update(fullUser); err != nil {
		http.Error(w, "Failed to update password", http.StatusInternalServerError)
		return
	}

	if err := h.syncUsersToConfig(); err != nil {
		log.Printf("Failed to persist password change: %v", err)
	}

	// Invalidate all other sessions for this user
	currentSession := auth.GetSessionFromContext(r.Context())
	exceptID := ""
	if currentSession != nil {
		exceptID = currentSession.ID
	}
	h.sessionStore.DeleteByUserID(user.ID, exceptID)

	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// AuthStatus handles GET /api/auth/status - returns auth configuration status
func (h *AuthHandler) AuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	// Get current user if authenticated
	user := auth.GetUserFromContext(r.Context())

	response := map[string]interface{}{
		"authenticated": user != nil,
		"oidc_enabled":  h.oidcProvider != nil && h.oidcProvider.Enabled(),
	}

	if user != nil {
		response["user"] = UserResponse{
			Username:    user.Username,
			Role:        user.Role,
			Email:       user.Email,
			DisplayName: user.DisplayName,
		}
	}

	if h.setupChecker != nil {
		response["setup_required"] = h.setupChecker()
	}

	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(response)
}

// OIDCLogin handles GET /api/auth/oidc/login - redirects to OIDC provider
func (h *AuthHandler) OIDCLogin(w http.ResponseWriter, r *http.Request) {
	if h.oidcProvider == nil || !h.oidcProvider.Enabled() {
		http.Error(w, "OIDC not configured", http.StatusNotFound)
		return
	}

	h.oidcProvider.HandleLogin(w, r)
}

// OIDCCallback handles GET /api/auth/oidc/callback - OIDC callback
func (h *AuthHandler) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	if h.oidcProvider == nil || !h.oidcProvider.Enabled() {
		http.Error(w, "OIDC not configured", http.StatusNotFound)
		return
	}

	h.oidcProvider.HandleCallback(w, r)
}

// syncUsersToConfig persists the current user store to the config file.
func (h *AuthHandler) syncUsersToConfig() error {
	if h.config == nil {
		return nil
	}

	h.configMu.Lock()
	defer h.configMu.Unlock()

	users := h.userStore.ListWithHashes()
	cfgUsers := make([]config.UserConfig, 0, len(users))
	for _, u := range users {
		cfgUsers = append(cfgUsers, config.UserConfig{
			Username:     u.Username,
			PasswordHash: u.PasswordHash,
			Role:         u.Role,
			Email:        u.Email,
			DisplayName:  u.DisplayName,
		})
	}
	h.config.Auth.Users = cfgUsers
	return h.config.Save(h.configPath)
}

// ListUsers handles GET /api/auth/users
func (h *AuthHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users := h.userStore.List()
	var resp []UserResponse
	for _, u := range users {
		resp = append(resp, UserResponse{
			Username:    u.Username,
			Role:        u.Role,
			Email:       u.Email,
			DisplayName: u.DisplayName,
		})
	}
	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(resp)
}

// CreateUser handles POST /api/auth/users
func (h *AuthHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		Role        string `json:"role"`
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Username) == "" {
		w.Header().Set(headerContentType, contentTypeJSON)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Username is required"})
		return
	}
	if len(req.Password) < 8 {
		w.Header().Set(headerContentType, contentTypeJSON)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Password must be at least 8 characters"})
		return
	}
	// Validate role
	if req.Role != auth.RoleAdmin && req.Role != auth.RoleUser && req.Role != auth.RoleGuest {
		req.Role = auth.RoleUser
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	user := &auth.User{
		ID:           req.Username,
		Username:     req.Username,
		PasswordHash: hash,
		Role:         req.Role,
		Email:        req.Email,
		DisplayName:  req.DisplayName,
	}

	if err := h.userStore.Add(user); err != nil {
		w.Header().Set(headerContentType, contentTypeJSON)
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	if err := h.syncUsersToConfig(); err != nil {
		log.Printf("Failed to persist user creation: %v", err)
	}

	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"user": UserResponse{
			Username:    user.Username,
			Role:        user.Role,
			Email:       user.Email,
			DisplayName: user.DisplayName,
		},
	})
}

// UpdateUser handles PUT /api/auth/users/{username}
func (h *AuthHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimPrefix(r.URL.Path, "/api/auth/users/")
	if username == "" {
		http.Error(w, "Username required", http.StatusBadRequest)
		return
	}

	var req struct {
		Role        string `json:"role"`
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user := h.userStore.Get(username)
	if user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if req.Role != "" {
		if req.Role != auth.RoleAdmin && req.Role != auth.RoleUser && req.Role != auth.RoleGuest {
			w.Header().Set(headerContentType, contentTypeJSON)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Invalid role"})
			return
		}
		user.Role = req.Role
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.DisplayName != "" {
		user.DisplayName = req.DisplayName
	}

	if err := h.userStore.Update(user); err != nil {
		http.Error(w, "Failed to update user", http.StatusInternalServerError)
		return
	}

	if err := h.syncUsersToConfig(); err != nil {
		log.Printf("Failed to persist user update: %v", err)
	}

	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"user": UserResponse{
			Username:    user.Username,
			Role:        user.Role,
			Email:       user.Email,
			DisplayName: user.DisplayName,
		},
	})
}

// DeleteUser handles DELETE /api/auth/users/{username}
func (h *AuthHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimPrefix(r.URL.Path, "/api/auth/users/")
	if username == "" {
		http.Error(w, "Username required", http.StatusBadRequest)
		return
	}

	// Can't delete self
	currentUser := auth.GetUserFromContext(r.Context())
	if currentUser != nil && currentUser.Username == username {
		w.Header().Set(headerContentType, contentTypeJSON)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Cannot delete your own account"})
		return
	}

	// Check if this is the last admin
	targetUser := h.userStore.Get(username)
	if targetUser == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if targetUser.Role == auth.RoleAdmin {
		// Count admins
		adminCount := 0
		for _, u := range h.userStore.List() {
			if u.Role == auth.RoleAdmin {
				adminCount++
			}
		}
		if adminCount <= 1 {
			w.Header().Set(headerContentType, contentTypeJSON)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Cannot delete the last admin user"})
			return
		}
	}

	if err := h.userStore.Delete(username); err != nil {
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	if err := h.syncUsersToConfig(); err != nil {
		log.Printf("Failed to persist user deletion: %v", err)
	}

	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// UpdateAuthMethod handles PUT /api/auth/method
func (h *AuthHandler) UpdateAuthMethod(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Method         string            `json:"method"`
		TrustedProxies []string          `json:"trusted_proxies"`
		Headers        map[string]string `json:"headers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	switch req.Method {
	case "builtin":
		if h.userStore.Count() == 0 {
			w.Header().Set(headerContentType, contentTypeJSON)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "At least one user is required for builtin auth"})
			return
		}

		h.configMu.Lock()
		h.config.Auth.Method = "builtin"
		h.authMiddleware.UpdateConfig(auth.AuthConfig{
			Method:      auth.AuthMethodBuiltin,
			BypassRules: h.bypassRules,
			APIKey:      h.config.Auth.APIKey,
		})
		err := h.config.Save(h.configPath)
		h.configMu.Unlock()
		if err != nil {
			http.Error(w, "Failed to save config", http.StatusInternalServerError)
			return
		}

	case "forward_auth":
		if len(req.TrustedProxies) == 0 {
			w.Header().Set(headerContentType, contentTypeJSON)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Trusted proxies required for forward_auth"})
			return
		}

		headers := auth.ForwardAuthHeaders{}
		if req.Headers != nil {
			headers.User = req.Headers["user"]
			headers.Email = req.Headers["email"]
			headers.Groups = req.Headers["groups"]
			headers.Name = req.Headers["name"]
		}

		h.configMu.Lock()
		h.config.Auth.Method = "forward_auth"
		h.config.Auth.TrustedProxies = req.TrustedProxies
		if req.Headers != nil {
			h.config.Auth.Headers = req.Headers
		}
		h.authMiddleware.UpdateConfig(auth.AuthConfig{
			Method:         auth.AuthMethodForwardAuth,
			TrustedProxies: req.TrustedProxies,
			Headers:        headers,
			BypassRules:    h.bypassRules,
			APIKey:         h.config.Auth.APIKey,
		})
		err := h.config.Save(h.configPath)
		h.configMu.Unlock()
		if err != nil {
			http.Error(w, "Failed to save config", http.StatusInternalServerError)
			return
		}

	case "none":
		h.configMu.Lock()
		h.config.Auth.Method = "none"
		h.authMiddleware.UpdateConfig(auth.AuthConfig{
			Method:      auth.AuthMethodNone,
			BypassRules: h.bypassRules,
			APIKey:      h.config.Auth.APIKey,
		})
		err := h.config.Save(h.configPath)
		h.configMu.Unlock()
		if err != nil {
			http.Error(w, "Failed to save config", http.StatusInternalServerError)
			return
		}

	default:
		w.Header().Set(headerContentType, contentTypeJSON)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Invalid method"})
		return
	}

	w.Header().Set(headerContentType, contentTypeJSON)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"method":  req.Method,
	})
}
