package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/logging"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	sessionStore          *auth.SessionStore
	userStore             *auth.UserStore
	oidcProvider          *auth.OIDCProvider
	setupChecker          func() bool
	config                *config.Config
	configPath            string
	authMiddleware        *auth.Middleware
	configMu              *sync.RWMutex
	bypassRules           []auth.BypassRule
	changePasswordLimiter *userAttemptLimiter
	discoveryProbe        DockerLifecycleProbe
}

// userAttemptLimiter is a minimal per-key attempt counter with a sliding
// window. Used to throttle sensitive per-user actions (currently the
// current-password check in ChangePassword). Map growth is bounded by
// purging stale entries on every allow() call.
type userAttemptLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	max      int
	window   time.Duration
}

func newUserAttemptLimiter(max int, window time.Duration) *userAttemptLimiter {
	return &userAttemptLimiter{
		attempts: make(map[string][]time.Time),
		max:      max,
		window:   window,
	}
}

func (l *userAttemptLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-l.window)

	// Prune stale entries across the whole map so a flood of distinct
	// usernames cannot grow it without bound.
	for k, ts := range l.attempts {
		valid := ts[:0]
		for _, t := range ts {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(l.attempts, k)
		} else {
			l.attempts[k] = valid
		}
	}

	if len(l.attempts[key]) >= l.max {
		return false
	}
	l.attempts[key] = append(l.attempts[key], now)
	return true
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(sessionStore *auth.SessionStore, userStore *auth.UserStore,
	cfg *config.Config, configPath string, authMiddleware *auth.Middleware, configMu *sync.RWMutex) *AuthHandler {
	return &AuthHandler{
		sessionStore:          sessionStore,
		userStore:             userStore,
		config:                cfg,
		configPath:            configPath,
		authMiddleware:        authMiddleware,
		configMu:              configMu,
		changePasswordLimiter: newUserAttemptLimiter(5, 15*time.Minute),
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

// SetDockerLifecycleProbe wires the discovery service so /api/auth/me
// can compute can_use_docker_lifecycle. Nil probe is treated as
// "socket not writable".
func (h *AuthHandler) SetDockerLifecycleProbe(p DockerLifecycleProbe) {
	h.discoveryProbe = p
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
	Username              string   `json:"username"`
	Role                  string   `json:"role"`
	Email                 string   `json:"email,omitempty"`
	DisplayName           string   `json:"display_name,omitempty"`
	Groups                []string `json:"groups,omitempty"`
	CanUseDockerLifecycle bool     `json:"can_use_docker_lifecycle"`
}

// DockerLifecycleProbe is the minimal Service surface the AuthHandler
// needs to compute the can_use_docker_lifecycle flag. Defined as an
// interface so tests can supply a stub without constructing a full
// discovery.Service.
type DockerLifecycleProbe interface {
	SocketWritable() bool
}

// lifecycleDenyReason is the first failing rung of the user-level Docker
// lifecycle gate, or denyNone when the user may use the controls. The
// string values double as the audit-log "reason" field.
type lifecycleDenyReason string

const (
	denyNone              lifecycleDenyReason = ""
	denyLifecycleDisabled lifecycleDenyReason = "lifecycle_disabled"
	denySocketReadonly    lifecycleDenyReason = "socket_readonly"
	denyMinRole           lifecycleDenyReason = "min_role_not_met"
	denyNotInGroup        lifecycleDenyReason = "not_in_allowed_groups"
)

// evaluateLifecycleGate runs the user-level lifecycle gate ladder (the
// part independent of any specific app): lifecycle_enabled -> socket
// writable -> min-role floor -> group allowlist. It returns the first
// failing reason, or denyNone if the user may use the controls. Both the
// advisory flag (ComputeCanUseDockerLifecycle, surfaced to the frontend)
// and the server-side enforcement (dockerAction) go through this single
// ladder so they can never diverge.
func evaluateLifecycleGate(u *auth.User, d *config.DiscoveryDockerConfig, socketWritable bool) lifecycleDenyReason {
	if u == nil || d == nil || !d.LifecycleEnabled {
		return denyLifecycleDisabled
	}
	if !socketWritable {
		return denySocketReadonly
	}
	minRole := d.LifecycleMinRole
	if minRole == "" {
		minRole = auth.RoleAdmin
	}
	if !auth.HasMinRole(u.Role, minRole) {
		return denyMinRole
	}
	if len(d.LifecycleAllowedGroups) > 0 && !auth.InAnyGroup(u, d.LifecycleAllowedGroups) {
		return denyNotInGroup
	}
	return denyNone
}

// ComputeCanUseDockerLifecycle returns the per-user permission flag
// surfaced to the frontend via /api/auth/me. Centralised here so the
// UI's gating logic stays single-source-of-truth: the frontend never
// re-implements the role / group check.
func ComputeCanUseDockerLifecycle(u *auth.User, d *config.DiscoveryDockerConfig, socketWritable bool) bool {
	return evaluateLifecycleGate(u, d, socketWritable) == denyNone
}

// buildUserResponse assembles a UserResponse including the computed
// lifecycle permission flag. Use this instead of literal struct
// constructors so the flag stays in sync across login, OIDC, /me,
// and password-change response paths.
func (h *AuthHandler) buildUserResponse(u *auth.User) UserResponse {
	var writable bool
	if h.discoveryProbe != nil {
		writable = h.discoveryProbe.SocketWritable()
	}
	var dockerCfg config.DiscoveryDockerConfig
	h.configMu.RLock()
	if h.config != nil {
		dockerCfg = h.config.Discovery.Docker
	}
	h.configMu.RUnlock()
	return UserResponse{
		Username:              u.Username,
		Role:                  u.Role,
		Email:                 u.Email,
		DisplayName:           u.DisplayName,
		Groups:                u.Groups,
		CanUseDockerLifecycle: ComputeCanUseDockerLifecycle(u, &dockerCfg, writable),
	}
}

// Login handles POST /api/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, http.StatusBadRequest, LoginResponse{
			Success: false,
			Message: errInvalidBody,
		})
		return
	}

	// Validate input
	if req.Username == "" || req.Password == "" {
		sendJSON(w, http.StatusBadRequest, LoginResponse{
			Success: false,
			Message: "Username and password are required",
		})
		return
	}

	// Authenticate
	user, rehashed, err := h.userStore.Authenticate(req.Username, req.Password)
	if err != nil {
		// Warn (not Info) so default-warn production logging
		// captures every failed attempt, which is the only signal
		// for brute-force monitoring.
		logging.From(r.Context()).Warn("Login failed", "source", "audit", "user", req.Username)
		sendJSON(w, http.StatusUnauthorized, LoginResponse{
			Success: false,
			Message: "Invalid username or password",
		})
		return
	}

	// If Authenticate upgraded the user's bcrypt hash, persist it now
	// so the upgrade survives a restart. A persist failure here is
	// non-fatal - the operator gets a Warn, the user still logs in,
	// and the next successful login retries the upgrade.
	if rehashed {
		if err := h.syncUsersToConfig(); err != nil {
			logging.From(r.Context()).Warn("Failed to persist upgraded password hash; will retry on next login",
				"source", "audit",
				"user", user.Username,
				"error", err)
		}
	}

	// Create session
	session, err := h.sessionStore.Create(user.ID, user.Username, user.Role)
	if err != nil {
		logging.From(r.Context()).Error("Failed to create session", "source", "auth", "user", user.Username, "error", err)
		sendJSON(w, http.StatusInternalServerError, LoginResponse{
			Success: false,
			Message: "Failed to create session",
		})
		return
	}

	// Carry the user's group memberships on the session so gateway sites
	// with an allowed_groups gate accept builtin users. The gateway
	// forward-auth check (sessionInAllowedGroups) reads groups from
	// session.Data, which the OIDC path populates too; without this a
	// builtin member of an allowed group is denied at a require_auth gate.
	if len(user.Groups) > 0 {
		session.Data["groups"] = user.Groups
	}

	// Set session cookie
	h.sessionStore.SetCookie(w, session)
	logging.From(r.Context()).Info("User logged in", "source", "audit", "user", user.Username)

	ur := h.buildUserResponse(user)
	sendJSON(w, http.StatusOK, LoginResponse{
		Success: true,
		User:    &ur,
	})
}

// Logout handles POST /api/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	// Get current session
	session := h.sessionStore.GetFromRequest(r)
	username := "unknown"
	if session != nil {
		username = session.Username
		h.sessionStore.Delete(session.ID)
	}

	// Clear cookie
	h.sessionStore.ClearCookie(w)
	logging.From(r.Context()).Info("User logged out", "source", "audit", "user", username)

	sendJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// Me handles GET /api/auth/me - returns current user info
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		sendJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"authenticated": false,
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"authenticated": true,
		"user":          h.buildUserResponse(user),
	})
}

// ChangePassword handles POST /api/auth/password
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		respondError(w, r, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Per-user rate limit on the current-password check (findings.md H1).
	// A thief with a short-lived session cookie would otherwise
	// brute-force the existing password here unthrottled and persist
	// their access beyond the cookie's lifetime.
	if !h.changePasswordLimiter.allow(user.Username) {
		w.Header().Set("Retry-After", "60")
		sendJSON(w, http.StatusTooManyRequests, map[string]interface{}{
			"success": false,
			"message": "Too many password change attempts; try again later",
		})
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidBody)
		return
	}

	if len(req.NewPassword) < 8 {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Password must be at least 8 characters",
		})
		return
	}

	// Verify current password
	_, _, err := h.userStore.Authenticate(user.Username, req.CurrentPassword)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Current password is incorrect",
		})
		return
	}

	// Hash new password
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	// Update user. Keep a snapshot of the previous state so we can
	// revert cleanly if the save fails (findings.md H7).
	fullUser := h.userStore.Get(user.Username)
	if fullUser == nil {
		respondError(w, r, http.StatusInternalServerError, errUserNotFound)
		return
	}
	prev := *fullUser
	fullUser.PasswordHash = hash
	if err := h.userStore.Update(fullUser); err != nil {
		respondError(w, r, http.StatusInternalServerError, "Failed to update password")
		return
	}

	if err := h.syncUsersToConfig(); err != nil {
		if rbErr := h.userStore.Update(&prev); rbErr != nil {
			logging.From(r.Context()).Error("Password change divergence: persist failed and in-memory rollback also failed",
				"source", "audit",
				"user", user.Username,
				"persist_error", err,
				"rollback_error", rbErr)
			sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
				"success":  false,
				"mismatch": true,
				"message":  "password change could not be persisted and rollback failed; restart Muximux to recover",
			})
			return
		}
		logging.From(r.Context()).Error("Failed to persist password change; reverted", "source", "auth", "user", user.Username, "error", err)
		respondError(w, r, http.StatusInternalServerError, "Failed to persist password change")
		return
	}

	// Invalidate all other sessions for this user
	currentSession := auth.GetSessionFromContext(r.Context())
	exceptID := ""
	if currentSession != nil {
		exceptID = currentSession.ID
	}
	h.sessionStore.DeleteByUserID(user.ID, exceptID)

	logging.From(r.Context()).Info("Password changed", "source", "audit", "user", user.Username)
	sendJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// AuthStatus handles GET /api/auth/status - returns auth configuration status
func (h *AuthHandler) AuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	// Get current user if authenticated
	user := auth.GetUserFromContext(r.Context())

	var authMethod string
	var logoutURL string
	if h.config != nil {
		h.configMu.RLock()
		authMethod = h.config.Auth.Method
		logoutURL = h.config.Auth.LogoutURL
		h.configMu.RUnlock()
	}

	response := map[string]interface{}{
		"authenticated": user != nil,
		"oidc_enabled":  h.oidcProvider != nil && h.oidcProvider.Enabled(),
		"auth_method":   authMethod,
	}

	if authMethod == "forward_auth" && logoutURL != "" {
		response["logout_url"] = logoutURL
	}

	if user != nil {
		response["user"] = h.buildUserResponse(user)
	}

	if h.setupChecker != nil {
		response["setup_required"] = h.setupChecker()
	}

	sendJSON(w, http.StatusOK, response)
}

// OIDCLogin handles GET /api/auth/oidc/login - redirects to OIDC provider
func (h *AuthHandler) OIDCLogin(w http.ResponseWriter, r *http.Request) {
	if h.oidcProvider == nil || !h.oidcProvider.Enabled() {
		respondError(w, r, http.StatusNotFound, "OIDC not configured")
		return
	}

	h.oidcProvider.HandleLogin(w, r)
}

// OIDCCallback handles GET /api/auth/oidc/callback - OIDC callback
func (h *AuthHandler) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	if h.oidcProvider == nil || !h.oidcProvider.Enabled() {
		respondError(w, r, http.StatusNotFound, "OIDC not configured")
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
			Groups:       u.Groups,
		})
	}
	h.config.Auth.Users = cfgUsers
	return h.config.Save(h.configPath)
}

// ListUsers handles GET /api/auth/users
func (h *AuthHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users := h.userStore.List()
	resp := make([]UserResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, UserResponse{
			Username:    u.Username,
			Role:        u.Role,
			Email:       u.Email,
			DisplayName: u.DisplayName,
			Groups:      u.Groups,
		})
	}
	sendJSON(w, http.StatusOK, resp)
}

// CreateUser handles POST /api/auth/users
func (h *AuthHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string   `json:"username"`
		Password    string   `json:"password"`
		Role        string   `json:"role"`
		Email       string   `json:"email"`
		DisplayName string   `json:"display_name"`
		Groups      []string `json:"groups"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidBody)
		return
	}

	if strings.TrimSpace(req.Username) == "" {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "Username is required"})
		return
	}
	if len(req.Password) < 8 {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "Password must be at least 8 characters"})
		return
	}
	// Validate role
	if req.Role != auth.RoleAdmin && req.Role != auth.RolePowerUser && req.Role != auth.RoleUser {
		req.Role = auth.RoleUser
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	user := &auth.User{
		ID:           req.Username,
		Username:     req.Username,
		PasswordHash: hash,
		Role:         req.Role,
		Email:        req.Email,
		DisplayName:  req.DisplayName,
		Groups:       sanitizeGroupList(req.Groups),
	}

	if err := h.userStore.Add(user); err != nil {
		sendJSON(w, http.StatusConflict, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	// Roll the in-memory state back if we cannot persist to disk, so a
	// restart does not quietly drop the new account (findings.md H7).
	if err := h.syncUsersToConfig(); err != nil {
		_ = h.userStore.Delete(user.Username)
		logging.From(r.Context()).Error("Failed to persist user creation; reverted", "source", "auth", "user", user.Username, "error", err)
		respondError(w, r, http.StatusInternalServerError, "Failed to persist user")
		return
	}

	logging.From(r.Context()).Info("User created", "source", "audit", "user", user.Username, "role", user.Role)
	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"user":    h.buildUserResponse(user),
	})
}

// UpdateUser handles PUT /api/auth/users/{username}
func (h *AuthHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimPrefix(r.URL.Path, "/api/auth/users/")
	if username == "" {
		respondError(w, r, http.StatusBadRequest, "Username required")
		return
	}

	var req struct {
		Role        string    `json:"role"`
		Email       *string   `json:"email"`
		DisplayName *string   `json:"display_name"`
		Groups      *[]string `json:"groups"` // pointer so omission and explicit []/null both round-trip cleanly
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidBody)
		return
	}

	prev := h.userStore.Get(username)
	if prev == nil {
		respondError(w, r, http.StatusNotFound, errUserNotFound)
		return
	}
	user := *prev // snapshot for rollback on save failure

	if req.Role != "" {
		if req.Role != auth.RoleAdmin && req.Role != auth.RolePowerUser && req.Role != auth.RoleUser {
			sendJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "Invalid role"})
			return
		}
		user.Role = req.Role
	}
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.DisplayName != nil {
		user.DisplayName = *req.DisplayName
	}
	if req.Groups != nil {
		user.Groups = sanitizeGroupList(*req.Groups)
	}

	// Atomic last-admin guard: reject the update if it would demote the
	// only remaining admin (findings.md H11). Without this, an admin
	// could demote themselves and the instance would have no one able to
	// access admin-gated endpoints.
	if err := h.userStore.UpdateIfNotLastAdminDemotion(&user); err != nil {
		if err.Error() == "user not found" {
			respondError(w, r, http.StatusNotFound, errUserNotFound)
			return
		}
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	if err := h.syncUsersToConfig(); err != nil {
		// Best-effort rollback: restore the prior user record in the
		// in-memory store so the live API view matches what's on
		// disk. If the rollback itself fails, we are in a divergence
		// state — surface a 503 with mismatch=true so the UI can
		// pin a sticky banner asking the operator to restart.
		if rbErr := h.userStore.Update(prev); rbErr != nil {
			logging.From(r.Context()).Error("User update divergence: persist failed and in-memory rollback also failed",
				"source", "audit",
				"user", username,
				"persist_error", err,
				"rollback_error", rbErr)
			sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
				"success":  false,
				"mismatch": true,
				"message":  "user update could not be persisted and rollback failed; restart Muximux to recover",
			})
			return
		}
		logging.From(r.Context()).Error("Failed to persist user update; reverted", "source", "auth", "user", username, "error", err)
		respondError(w, r, http.StatusInternalServerError, "Failed to persist user update")
		return
	}

	logging.From(r.Context()).Info("User updated", "source", "audit", "user", username, "role", user.Role)
	ur := h.buildUserResponse(&user)
	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"user":    ur,
	})
}

// sanitizeGroupList trims whitespace, drops empties, and de-duplicates
// a group list submitted by an admin. Group matching elsewhere is
// case-insensitive, but we preserve the operator's chosen casing so it
// is easier to read when the file is opened or exported.
func sanitizeGroupList(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, g := range in {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		key := strings.ToLower(g)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, g)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// DeleteUser handles DELETE /api/auth/users/{username}
func (h *AuthHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimPrefix(r.URL.Path, "/api/auth/users/")
	if username == "" {
		respondError(w, r, http.StatusBadRequest, "Username required")
		return
	}

	// Can't delete self
	currentUser := auth.GetUserFromContext(r.Context())
	if currentUser != nil && currentUser.Username == username {
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "Cannot delete your own account"})
		return
	}

	// Snapshot the target before deletion so we can re-add on save failure.
	prev := h.userStore.Get(username)

	if err := h.userStore.DeleteIfNotLastAdmin(username); err != nil {
		if err.Error() == "user not found" {
			respondError(w, r, http.StatusNotFound, errUserNotFound)
			return
		}
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	if err := h.syncUsersToConfig(); err != nil {
		var rbErr error
		if prev != nil {
			rbErr = h.userStore.Add(prev) // best-effort rollback
		}
		if rbErr != nil {
			logging.From(r.Context()).Error("User deletion divergence: persist failed and in-memory rollback also failed",
				"source", "audit",
				"user", username,
				"persist_error", err,
				"rollback_error", rbErr)
			sendJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
				"success":  false,
				"mismatch": true,
				"message":  "user deletion could not be persisted and rollback failed; restart Muximux to recover",
			})
			return
		}
		logging.From(r.Context()).Error("Failed to persist user deletion; reverted", "source", "auth", "user", username, "error", err)
		respondError(w, r, http.StatusInternalServerError, "Failed to persist user deletion")
		return
	}

	// Revoke the deleted user's live sessions so the account cannot keep
	// acting (with its captured role) until the session's absolute expiry.
	// Mirrors ChangePassword. Keyed by user ID (== username for builtin
	// users). Best-effort: the user is already gone from the store, so a
	// lingering session would fail the GetByID lookup anyway, but this
	// closes the userFromSession fallback path immediately.
	if prev != nil {
		h.sessionStore.DeleteByUserID(prev.ID, "")
	}

	logging.From(r.Context()).Info("User deleted", "source", "audit", "user", username)
	sendJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// UpdateAuthMethod handles PUT /api/auth/method
func (h *AuthHandler) UpdateAuthMethod(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Method                 string            `json:"method"`
		TrustedProxies         []string          `json:"trusted_proxies"`
		Headers                map[string]string `json:"headers"`
		LogoutURL              string            `json:"logout_url"`
		ForwardAuthAdminGroups []string          `json:"forward_auth_admin_groups"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidBody)
		return
	}

	// Validate and build auth config per method
	var authCfg auth.AuthConfig
	switch req.Method {
	case "builtin":
		if h.userStore.Count() == 0 {
			sendJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "At least one user is required for builtin auth"})
			return
		}
		authCfg = auth.AuthConfig{Method: auth.AuthMethodBuiltin}

	case "forward_auth":
		if len(req.TrustedProxies) == 0 {
			sendJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "Trusted proxies required for forward_auth"})
			return
		}
		authCfg = auth.AuthConfig{
			Method:                 auth.AuthMethodForwardAuth,
			TrustedProxies:         req.TrustedProxies,
			Headers:                auth.ForwardAuthHeadersFromMap(req.Headers),
			ForwardAuthAdminGroups: req.ForwardAuthAdminGroups,
		}

	case "none":
		authCfg = auth.AuthConfig{Method: auth.AuthMethodNone}

	default:
		sendJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "Invalid method"})
		return
	}

	authCfg.BypassRules = h.bypassRules

	// Mutate, save, then push to middleware - all under one lock
	// scope. Two bugs the old ordering had:
	//   - APIKeyHash and BasePath were read off h.config *outside*
	//     h.configMu, so a concurrent GenerateAPIKey/DeleteAPIKey
	//     could rotate the hash between the read and the
	//     UpdateConfig call (codebase review H4);
	//   - UpdateConfig fired *before* Save, so a save failure left
	//     the live middleware running the new auth method (e.g.
	//     "none" - which exposes every API to anyone) while disk
	//     still said "builtin", a silent security downgrade until
	//     restart (codebase review C3-shf).
	if err := func() error {
		h.configMu.Lock()
		defer h.configMu.Unlock()

		// Capture priors so we can roll back the in-memory shape on
		// save failure.
		priorMethod := h.config.Auth.Method
		priorTrustedProxies := append([]string(nil), h.config.Auth.TrustedProxies...)
		priorHeaders := h.config.Auth.Headers
		priorLogoutURL := h.config.Auth.LogoutURL
		priorFwAdminGroups := append([]string(nil), h.config.Auth.ForwardAuthAdminGroups...)

		// Reads of APIKeyHash and BasePath happen here, under the
		// lock, so a concurrent rotation can't slip a stale hash
		// into authCfg.
		authCfg.APIKeyHash = h.config.Auth.APIKeyHash
		authCfg.BasePath = h.config.Server.NormalizedBasePath()

		// Apply the new config in memory.
		h.config.Auth.Method = req.Method
		if req.Method == "forward_auth" {
			h.config.Auth.TrustedProxies = req.TrustedProxies
			h.config.Auth.LogoutURL = req.LogoutURL
			if req.Headers != nil {
				h.config.Auth.Headers = req.Headers
			}
			h.config.Auth.ForwardAuthAdminGroups = req.ForwardAuthAdminGroups
		} else {
			// Clear forward-auth fields so stale values don't linger in YAML
			h.config.Auth.TrustedProxies = nil
			h.config.Auth.Headers = nil
			h.config.Auth.LogoutURL = ""
			h.config.Auth.ForwardAuthAdminGroups = nil
		}

		// Persist BEFORE pushing to middleware. If Save fails the
		// running auth method has not changed yet.
		if err := h.config.Save(h.configPath); err != nil {
			h.config.Auth.Method = priorMethod
			h.config.Auth.TrustedProxies = priorTrustedProxies
			h.config.Auth.Headers = priorHeaders
			h.config.Auth.LogoutURL = priorLogoutURL
			h.config.Auth.ForwardAuthAdminGroups = priorFwAdminGroups
			return err
		}
		h.authMiddleware.UpdateConfig(&authCfg)
		return nil
	}(); err != nil {
		respondError(w, r, http.StatusInternalServerError, "Failed to save config", "source", "auth", "method", req.Method, "error", err)
		return
	}

	logging.From(r.Context()).Info("Auth method changed", "source", "audit", "method", req.Method)
	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"method":  req.Method,
	})
}

// apiKeyByteLen is the random byte budget for a generated API key.
// 32 bytes encoded as base64url is 43 chars; with the muximux_ prefix
// the user-visible key is 51 characters.
const apiKeyByteLen = 32

// APIKeyStatus reports whether an API key is configured. The plaintext
// is never exposed because only the bcrypt hash is stored. Admin only.
func (h *AuthHandler) APIKeyStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}
	h.configMu.RLock()
	configured := h.config.Auth.APIKeyHash != ""
	h.configMu.RUnlock()
	sendJSON(w, http.StatusOK, map[string]interface{}{
		"configured": configured,
	})
}

// GenerateAPIKey creates a new random API key, replaces any existing
// key, and returns the plaintext exactly once. The plaintext is not
// recoverable afterwards because only the bcrypt hash is persisted.
// Admin only.
func (h *AuthHandler) GenerateAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	raw := make([]byte, apiKeyByteLen)
	if _, err := rand.Read(raw); err != nil {
		respondError(w, r, http.StatusInternalServerError, "Failed to generate key", "source", "auth", "error", err)
		return
	}
	plaintext := "muximux_" + base64.RawURLEncoding.EncodeToString(raw)

	// SHA-256, not bcrypt: an API key is a high-entropy token, so a fast
	// constant-time hash is equally brute-force-resistant without bcrypt's
	// per-verify CPU cost (an unauthenticated DoS vector on the bypass
	// path). Existing bcrypt hashes still verify; this rotation migrates.
	hash := auth.HashAPIKey(plaintext)

	h.configMu.Lock()
	prev := h.config.Auth.APIKeyHash
	h.config.Auth.APIKeyHash = hash
	saveErr := h.config.Save(h.configPath)
	if saveErr != nil {
		// Roll back the in-memory mutation so a failed disk write does
		// not leave the live config diverged from what is on disk.
		// Emit an audit log so monitoring can correlate the rotation
		// attempt with the disk failure; the generic "Failed to save
		// config" response below would otherwise make the rollback
		// invisible to ops.
		h.config.Auth.APIKeyHash = prev
		logging.From(r.Context()).Error("API key rotation rolled back due to save failure",
			"source", "audit",
			"previously_configured", prev != "",
			"error", saveErr)
	} else {
		h.refreshAuthSnapshotLocked()
	}
	h.configMu.Unlock()

	if saveErr != nil {
		respondError(w, r, http.StatusInternalServerError, "Failed to save config", "source", "auth", "error", saveErr)
		return
	}

	rotated := prev != ""
	logging.From(r.Context()).Info("API key generated", "source", "audit", "rotated", rotated)

	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"key":        plaintext,
		"warning":    "This is the only time the key will be shown. Store it securely.",
		"rotated":    rotated,
		"configured": true,
	})
}

// UpgradeAPIKeyHash migrates a legacy bcrypt api_key_hash to the fast
// sha256: form after the auth middleware verifies the key against the
// legacy hash. It is wired as the middleware's onAPIKeyUpgrade hook and
// runs off the request goroutine. The compare-and-swap on oldHash makes
// it safe and idempotent: it only writes when the stored hash is still
// exactly the legacy value the middleware verified against, so a
// concurrent rotation (or a second concurrent legacy request) is a no-op
// rather than a clobber. A save failure rolls back the in-memory change
// and is logged; the next successful verify retries.
func (h *AuthHandler) UpgradeAPIKeyHash(oldHash, newHash string) {
	if oldHash == "" || newHash == "" || oldHash == newHash {
		return
	}
	h.configMu.Lock()
	defer h.configMu.Unlock()
	if h.config.Auth.APIKeyHash != oldHash {
		// Rotated or already migrated since the verify; nothing to do.
		return
	}
	h.config.Auth.APIKeyHash = newHash
	if err := h.config.Save(h.configPath); err != nil {
		h.config.Auth.APIKeyHash = oldHash
		logging.Error("Legacy API key auto-upgrade rolled back due to save failure",
			"source", "audit", "error", err)
		return
	}
	h.refreshAuthSnapshotLocked()
	logging.Audit("Legacy API key hash auto-upgraded to sha256 on use")
}

// DeleteAPIKey clears the configured API key. Bypass rules that
// require the key (for example /api/appearance) immediately stop
// authenticating with X-Api-Key once this returns. Admin only.
func (h *AuthHandler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	h.configMu.Lock()
	prev := h.config.Auth.APIKeyHash
	if prev == "" {
		h.configMu.Unlock()
		sendJSON(w, http.StatusOK, map[string]interface{}{
			"success":    true,
			"configured": false,
		})
		return
	}
	h.config.Auth.APIKeyHash = ""
	saveErr := h.config.Save(h.configPath)
	if saveErr != nil {
		h.config.Auth.APIKeyHash = prev
		logging.From(r.Context()).Error("API key deletion rolled back due to save failure",
			"source", "audit",
			"error", saveErr)
	} else {
		h.refreshAuthSnapshotLocked()
	}
	h.configMu.Unlock()

	if saveErr != nil {
		respondError(w, r, http.StatusInternalServerError, "Failed to save config", "source", "auth", "error", saveErr)
		return
	}

	logging.From(r.Context()).Info("API key deleted", "source", "audit")
	sendJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"configured": false,
	})
}

// refreshAuthSnapshotLocked rebuilds the middleware's auth config from
// the current h.config and pushes it. Caller must hold h.configMu's
// write lock.
func (h *AuthHandler) refreshAuthSnapshotLocked() {
	authCfg := auth.AuthConfig{
		Method:                 auth.AuthMethod(h.config.Auth.Method),
		BypassRules:            h.bypassRules,
		APIKeyHash:             h.config.Auth.APIKeyHash,
		BasePath:               h.config.Server.NormalizedBasePath(),
		TrustedProxies:         h.config.Auth.TrustedProxies,
		Headers:                auth.ForwardAuthHeadersFromMap(h.config.Auth.Headers),
		ForwardAuthAdminGroups: h.config.Auth.ForwardAuthAdminGroups,
	}
	h.authMiddleware.UpdateConfig(&authCfg)
}
