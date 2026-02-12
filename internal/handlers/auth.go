package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/mescon/muximux/internal/auth"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	sessionStore *auth.SessionStore
	userStore    *auth.UserStore
	oidcProvider *auth.OIDCProvider
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(sessionStore *auth.SessionStore, userStore *auth.UserStore) *AuthHandler {
	return &AuthHandler{
		sessionStore: sessionStore,
		userStore:    userStore,
	}
}

// SetOIDCProvider sets the OIDC provider for OIDC authentication
func (h *AuthHandler) SetOIDCProvider(provider *auth.OIDCProvider) {
	h.oidcProvider = provider
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(LoginResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate input
	if req.Username == "" || req.Password == "" {
		w.Header().Set("Content-Type", "application/json")
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
		w.Header().Set("Content-Type", "application/json")
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(LoginResponse{
			Success: false,
			Message: "Failed to create session",
		})
		return
	}

	// Set session cookie
	h.sessionStore.SetCookie(w, session)

	w.Header().Set("Content-Type", "application/json")
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get current session
	session := h.sessionStore.GetFromRequest(r)
	if session != nil {
		h.sessionStore.Delete(session.ID)
	}

	// Clear cookie
	h.sessionStore.ClearCookie(w)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// Me handles GET /api/auth/me - returns current user info
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"authenticated": false,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
		w.Header().Set("Content-Type", "application/json")
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
		w.Header().Set("Content-Type", "application/json")
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

	// Invalidate all other sessions for this user
	currentSession := auth.GetSessionFromContext(r.Context())
	exceptID := ""
	if currentSession != nil {
		exceptID = currentSession.ID
	}
	h.sessionStore.DeleteByUserID(user.ID, exceptID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// AuthStatus handles GET /api/auth/status - returns auth configuration status
func (h *AuthHandler) AuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

	w.Header().Set("Content-Type", "application/json")
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
