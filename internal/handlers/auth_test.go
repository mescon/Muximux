package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
)

// setupAuthTest creates the auth handler with a session store and user store
// pre-populated with a test user.
func setupAuthTest(t *testing.T) (*AuthHandler, *auth.SessionStore) {
	t.Helper()

	sessionStore := auth.NewSessionStore("muximux_session", 24*time.Hour, false)
	userStore := auth.NewUserStore()

	// Create a test user with a known password hash
	hash, err := auth.HashPassword("testpass123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	userStore.LoadFromConfig([]auth.UserConfig{
		{
			Username:     "admin",
			PasswordHash: hash,
			Role:         "admin",
			Email:        "admin@example.com",
			DisplayName:  "Admin User",
		},
	})

	handler := NewAuthHandler(sessionStore, userStore, nil, "", nil, &sync.RWMutex{})
	return handler, sessionStore
}

func TestLogin(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		body, _ := json.Marshal(LoginRequest{
			Username: "admin",
			Password: "testpass123",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.Login(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp LoginResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if !resp.Success {
			t.Error("expected success=true")
		}
		if resp.User == nil {
			t.Fatal("expected user in response")
		}
		if resp.User.Username != "admin" {
			t.Errorf("expected username 'admin', got %q", resp.User.Username)
		}
		if resp.User.Role != "admin" {
			t.Errorf("expected role 'admin', got %q", resp.User.Role)
		}

		// Check session cookie was set
		cookies := w.Result().Cookies()
		found := false
		for _, c := range cookies {
			if c.Name == "muximux_session" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected session cookie to be set")
		}
	})

	t.Run("bad credentials", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		body, _ := json.Marshal(LoginRequest{
			Username: "admin",
			Password: "wrongpass",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.Login(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}

		var resp LoginResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Success {
			t.Error("expected success=false")
		}
	})

	t.Run("bad JSON", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader([]byte("not json")))
		w := httptest.NewRecorder()

		handler.Login(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
		w := httptest.NewRecorder()

		handler.Login(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})

	t.Run("empty username", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		body, _ := json.Marshal(LoginRequest{
			Username: "",
			Password: "testpass123",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.Login(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("empty password", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		body, _ := json.Marshal(LoginRequest{
			Username: "admin",
			Password: "",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.Login(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

func TestLogout(t *testing.T) {
	t.Run("with session", func(t *testing.T) {
		handler, sessionStore := setupAuthTest(t)

		// Create a session first
		session, err := sessionStore.Create("admin", "admin", "admin")
		if err != nil {
			t.Fatalf("failed to create session: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
		req.AddCookie(&http.Cookie{
			Name:  "muximux_session",
			Value: session.ID,
		})
		w := httptest.NewRecorder()

		handler.Logout(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		// Verify session is deleted
		if sessionStore.Get(session.ID) != nil {
			t.Error("expected session to be deleted")
		}
	})

	t.Run("without session", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
		w := httptest.NewRecorder()

		handler.Logout(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		req := httptest.NewRequest(http.MethodGet, "/api/auth/logout", nil)
		w := httptest.NewRecorder()

		handler.Logout(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})
}

func TestMe(t *testing.T) {
	t.Run("authenticated", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		user := &auth.User{
			ID:          "admin",
			Username:    "admin",
			Role:        "admin",
			Email:       "admin@example.com",
			DisplayName: "Admin User",
		}

		req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		ctx := context.WithValue(req.Context(), auth.ContextKeyUser, user)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.Me(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["authenticated"] != true {
			t.Error("expected authenticated=true")
		}
		userMap, ok := resp["user"].(map[string]interface{})
		if !ok {
			t.Fatal("expected user object in response")
		}
		if userMap["username"] != "admin" {
			t.Errorf("expected username 'admin', got %v", userMap["username"])
		}
	})

	t.Run("unauthenticated", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		w := httptest.NewRecorder()

		handler.Me(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["authenticated"] != false {
			t.Error("expected authenticated=false")
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		req := httptest.NewRequest(http.MethodPost, "/api/auth/me", nil)
		w := httptest.NewRecorder()

		handler.Me(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})
}

func TestChangePassword(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler, sessionStore := setupAuthTest(t)

		user := &auth.User{
			ID:       "admin",
			Username: "admin",
			Role:     "admin",
		}

		// Create a session to use as the current session
		session, _ := sessionStore.Create("admin", "admin", "admin")

		body, _ := json.Marshal(map[string]string{
			"current_password": "testpass123",
			"new_password":     "newpassword123",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/password", bytes.NewReader(body))
		ctx := context.WithValue(req.Context(), auth.ContextKeyUser, user)
		ctx = context.WithValue(ctx, auth.ContextKeySession, session)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.ChangePassword(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("unauthenticated", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		body, _ := json.Marshal(map[string]string{
			"current_password": "testpass123",
			"new_password":     "newpassword123",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/password", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.ChangePassword(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		req := httptest.NewRequest(http.MethodGet, "/api/auth/password", nil)
		w := httptest.NewRecorder()

		handler.ChangePassword(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})

	t.Run("password too short", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		user := &auth.User{
			ID:       "admin",
			Username: "admin",
			Role:     "admin",
		}

		body, _ := json.Marshal(map[string]string{
			"current_password": "testpass123",
			"new_password":     "short",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/password", bytes.NewReader(body))
		ctx := context.WithValue(req.Context(), auth.ContextKeyUser, user)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.ChangePassword(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("wrong current password", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		user := &auth.User{
			ID:       "admin",
			Username: "admin",
			Role:     "admin",
		}

		body, _ := json.Marshal(map[string]string{
			"current_password": "wrongcurrent",
			"new_password":     "newpassword123",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/password", bytes.NewReader(body))
		ctx := context.WithValue(req.Context(), auth.ContextKeyUser, user)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.ChangePassword(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("bad JSON", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		user := &auth.User{
			ID:       "admin",
			Username: "admin",
			Role:     "admin",
		}

		req := httptest.NewRequest(http.MethodPost, "/api/auth/password", bytes.NewReader([]byte("not json")))
		ctx := context.WithValue(req.Context(), auth.ContextKeyUser, user)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.ChangePassword(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

func TestAuthStatus(t *testing.T) {
	t.Run("unauthenticated no oidc", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
		w := httptest.NewRecorder()

		handler.AuthStatus(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["authenticated"] != false {
			t.Error("expected authenticated=false")
		}
		if resp["oidc_enabled"] != false {
			t.Error("expected oidc_enabled=false")
		}
	})

	t.Run("authenticated", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		user := &auth.User{
			ID:          "admin",
			Username:    "admin",
			Role:        "admin",
			Email:       "admin@example.com",
			DisplayName: "Admin User",
		}

		req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
		ctx := context.WithValue(req.Context(), auth.ContextKeyUser, user)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.AuthStatus(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["authenticated"] != true {
			t.Error("expected authenticated=true")
		}
		userMap, ok := resp["user"].(map[string]interface{})
		if !ok {
			t.Fatal("expected user object in response")
		}
		if userMap["username"] != "admin" {
			t.Errorf("expected username 'admin', got %v", userMap["username"])
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		req := httptest.NewRequest(http.MethodPost, "/api/auth/status", nil)
		w := httptest.NewRecorder()

		handler.AuthStatus(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", w.Code)
		}
	})
}

func TestOIDCLogin(t *testing.T) {
	t.Run("oidc not configured", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/login", nil)
		w := httptest.NewRecorder()

		handler.OIDCLogin(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
}

func TestOIDCCallback(t *testing.T) {
	t.Run("oidc not configured", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback", nil)
		w := httptest.NewRecorder()

		handler.OIDCCallback(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
}

func TestAuthStatus_SetupRequired(t *testing.T) {
	t.Run("setup_required true", func(t *testing.T) {
		handler, _ := setupAuthTest(t)
		handler.SetSetupChecker(func() bool { return true })

		req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
		w := httptest.NewRecorder()
		handler.AuthStatus(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["setup_required"] != true {
			t.Errorf("expected setup_required=true, got %v", resp["setup_required"])
		}
	})

	t.Run("setup_required false", func(t *testing.T) {
		handler, _ := setupAuthTest(t)
		handler.SetSetupChecker(func() bool { return false })

		req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
		w := httptest.NewRecorder()
		handler.AuthStatus(w, req)

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["setup_required"] != false {
			t.Errorf("expected setup_required=false, got %v", resp["setup_required"])
		}
	})

	t.Run("no checker set", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
		w := httptest.NewRecorder()
		handler.AuthStatus(w, req)

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if _, ok := resp["setup_required"]; ok {
			t.Error("expected no setup_required field when checker is nil")
		}
	})
}

func TestSetOIDCProvider(t *testing.T) {
	ss := auth.NewSessionStore("test", time.Hour, false)
	us := auth.NewUserStore()
	handler := NewAuthHandler(ss, us, nil, "", nil, &sync.RWMutex{})

	// Initially nil
	if handler.oidcProvider != nil {
		t.Error("expected nil oidcProvider initially")
	}

	// Set a provider
	provider := &auth.OIDCProvider{}
	handler.SetOIDCProvider(provider)

	if handler.oidcProvider != provider {
		t.Error("expected oidcProvider to be set")
	}
}

func TestSetBypassRules(t *testing.T) {
	handler, _ := setupAuthTest(t)

	rules := []auth.BypassRule{
		{Path: "/public/*", Methods: []string{"GET"}},
		{Path: "/api/health", Methods: []string{"GET"}, RequireAPIKey: true},
	}

	handler.SetBypassRules(rules)

	if len(handler.bypassRules) != 2 {
		t.Fatalf("expected 2 bypass rules, got %d", len(handler.bypassRules))
	}
	if handler.bypassRules[0].Path != "/public/*" {
		t.Errorf("expected first rule path '/public/*', got %q", handler.bypassRules[0].Path)
	}
	if handler.bypassRules[1].RequireAPIKey != true {
		t.Error("expected second rule to require API key")
	}
}

func TestListUsers(t *testing.T) {
	t.Run("returns users without password hashes", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		req := httptest.NewRequest(http.MethodGet, "/api/auth/users", nil)
		w := httptest.NewRecorder()

		handler.ListUsers(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var resp []UserResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp) != 1 {
			t.Fatalf("expected 1 user, got %d", len(resp))
		}
		if resp[0].Username != "admin" {
			t.Errorf("expected username 'admin', got %q", resp[0].Username)
		}
		if resp[0].Role != "admin" {
			t.Errorf("expected role 'admin', got %q", resp[0].Role)
		}
		if resp[0].Email != "admin@example.com" {
			t.Errorf("expected email 'admin@example.com', got %q", resp[0].Email)
		}
		if resp[0].DisplayName != "Admin User" {
			t.Errorf("expected display_name 'Admin User', got %q", resp[0].DisplayName)
		}
	})

	t.Run("returns multiple users", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		// Add a second user
		hash, _ := auth.HashPassword("secondpass123")
		if err := handler.userStore.Add(&auth.User{
			ID:           "viewer",
			Username:     "viewer",
			PasswordHash: hash,
			Role:         "user",
			Email:        "viewer@example.com",
		}); err != nil {
			t.Fatalf("failed to add user: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/auth/users", nil)
		w := httptest.NewRecorder()

		handler.ListUsers(w, req)

		var resp []UserResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp) != 2 {
			t.Fatalf("expected 2 users, got %d", len(resp))
		}
	})
}

func TestCreateUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		body, _ := json.Marshal(map[string]string{
			"username":     "newuser",
			"password":     "password123",
			"role":         "user",
			"email":        "new@example.com",
			"display_name": "New User",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/users", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.CreateUser(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["success"] != true {
			t.Error("expected success=true")
		}
		userMap, ok := resp["user"].(map[string]interface{})
		if !ok {
			t.Fatal("expected user object in response")
		}
		if userMap["username"] != "newuser" {
			t.Errorf("expected username 'newuser', got %v", userMap["username"])
		}
		if userMap["role"] != "user" {
			t.Errorf("expected role 'user', got %v", userMap["role"])
		}

		// Verify user was actually added to the store
		stored := handler.userStore.Get("newuser")
		if stored == nil {
			t.Fatal("expected user to be in the store")
		}
	})

	t.Run("invalid role defaults to user", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		body, _ := json.Marshal(map[string]string{
			"username": "roletest",
			"password": "password123",
			"role":     "superadmin",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/users", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.CreateUser(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		userMap := resp["user"].(map[string]interface{})
		if userMap["role"] != "user" {
			t.Errorf("expected role to default to 'user', got %v", userMap["role"])
		}
	})

	t.Run("empty username", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		body, _ := json.Marshal(map[string]string{
			"username": "",
			"password": "password123",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/users", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.CreateUser(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("whitespace only username", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		body, _ := json.Marshal(map[string]string{
			"username": "   ",
			"password": "password123",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/users", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.CreateUser(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("short password", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		body, _ := json.Marshal(map[string]string{
			"username": "shortpw",
			"password": "short",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/users", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.CreateUser(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("duplicate user", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		body, _ := json.Marshal(map[string]string{
			"username": "admin",
			"password": "password123",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/users", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.CreateUser(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("expected status 409, got %d", w.Code)
		}
	})

	t.Run("bad JSON", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		req := httptest.NewRequest(http.MethodPost, "/api/auth/users", bytes.NewReader([]byte("not json")))
		w := httptest.NewRecorder()

		handler.CreateUser(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

func TestUpdateUser(t *testing.T) {
	t.Run("success update role", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		// Add a user to update
		hash, _ := auth.HashPassword("password123")
		if err := handler.userStore.Add(&auth.User{
			ID:           "testuser",
			Username:     "testuser",
			PasswordHash: hash,
			Role:         "user",
		}); err != nil {
			t.Fatalf("failed to add user: %v", err)
		}

		body, _ := json.Marshal(map[string]string{
			"role":         "admin",
			"email":        "updated@example.com",
			"display_name": "Updated User",
		})
		req := httptest.NewRequest(http.MethodPut, "/api/auth/users/testuser", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.UpdateUser(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["success"] != true {
			t.Error("expected success=true")
		}
		userMap := resp["user"].(map[string]interface{})
		if userMap["role"] != "admin" {
			t.Errorf("expected role 'admin', got %v", userMap["role"])
		}
		if userMap["email"] != "updated@example.com" {
			t.Errorf("expected email 'updated@example.com', got %v", userMap["email"])
		}

		// Verify store was updated
		stored := handler.userStore.Get("testuser")
		if stored.Role != "admin" {
			t.Errorf("expected stored role 'admin', got %q", stored.Role)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		body, _ := json.Marshal(map[string]string{"role": "user"})
		req := httptest.NewRequest(http.MethodPut, "/api/auth/users/nonexistent", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.UpdateUser(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("empty username", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		body, _ := json.Marshal(map[string]string{"role": "user"})
		req := httptest.NewRequest(http.MethodPut, "/api/auth/users/", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.UpdateUser(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("invalid role", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		body, _ := json.Marshal(map[string]string{"role": "superadmin"})
		req := httptest.NewRequest(http.MethodPut, "/api/auth/users/admin", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.UpdateUser(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("bad JSON", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		req := httptest.NewRequest(http.MethodPut, "/api/auth/users/admin", bytes.NewReader([]byte("not json")))
		w := httptest.NewRecorder()

		handler.UpdateUser(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

func TestDeleteUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		// Add a second user to delete
		hash, _ := auth.HashPassword("password123")
		if err := handler.userStore.Add(&auth.User{
			ID:           "victim",
			Username:     "victim",
			PasswordHash: hash,
			Role:         "user",
		}); err != nil {
			t.Fatalf("failed to add user: %v", err)
		}

		// Set current user in context
		currentUser := &auth.User{ID: "admin", Username: "admin", Role: "admin"}
		req := httptest.NewRequest(http.MethodDelete, "/api/auth/users/victim", nil)
		ctx := context.WithValue(req.Context(), auth.ContextKeyUser, currentUser)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.DeleteUser(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["success"] != true {
			t.Error("expected success=true")
		}

		// Verify user was removed
		if handler.userStore.Get("victim") != nil {
			t.Error("expected user to be deleted")
		}
	})

	t.Run("cannot delete self", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		currentUser := &auth.User{ID: "admin", Username: "admin", Role: "admin"}
		req := httptest.NewRequest(http.MethodDelete, "/api/auth/users/admin", nil)
		ctx := context.WithValue(req.Context(), auth.ContextKeyUser, currentUser)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.DeleteUser(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["success"] != false {
			t.Error("expected success=false")
		}
	})

	t.Run("cannot delete last admin", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		// "admin" is the only admin user; try to delete as a different user
		hash, _ := auth.HashPassword("password123")
		if err := handler.userStore.Add(&auth.User{
			ID:           "operator",
			Username:     "operator",
			PasswordHash: hash,
			Role:         "user",
		}); err != nil {
			t.Fatalf("failed to add user: %v", err)
		}

		currentUser := &auth.User{ID: "operator", Username: "operator", Role: "user"}
		req := httptest.NewRequest(http.MethodDelete, "/api/auth/users/admin", nil)
		ctx := context.WithValue(req.Context(), auth.ContextKeyUser, currentUser)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.DeleteUser(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["message"] != "Cannot delete the last admin user" {
			t.Errorf("expected last admin message, got %v", resp["message"])
		}
	})

	t.Run("user not found", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		currentUser := &auth.User{ID: "admin", Username: "admin", Role: "admin"}
		req := httptest.NewRequest(http.MethodDelete, "/api/auth/users/nonexistent", nil)
		ctx := context.WithValue(req.Context(), auth.ContextKeyUser, currentUser)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.DeleteUser(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("empty username", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		req := httptest.NewRequest(http.MethodDelete, "/api/auth/users/", nil)
		w := httptest.NewRecorder()

		handler.DeleteUser(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

// setupAuthTestWithConfig creates an auth handler with a real config, temp config
// file, and auth middleware - needed for UpdateAuthMethod and syncUsersToConfig tests.
func setupAuthTestWithConfig(t *testing.T) (*AuthHandler, string) {
	t.Helper()

	sessionStore := auth.NewSessionStore("muximux_session", 24*time.Hour, false)
	userStore := auth.NewUserStore()

	hash, err := auth.HashPassword("testpass123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	userStore.LoadFromConfig([]auth.UserConfig{
		{
			Username:     "admin",
			PasswordHash: hash,
			Role:         "admin",
			Email:        "admin@example.com",
			DisplayName:  "Admin User",
		},
	})

	cfg := &config.Config{
		Auth: config.AuthConfig{Method: "none"},
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}

	middleware := auth.NewMiddleware(&auth.AuthConfig{
		Method: auth.AuthMethodNone,
	}, sessionStore, userStore)

	handler := NewAuthHandler(sessionStore, userStore, cfg, configPath, middleware, &sync.RWMutex{})
	return handler, configPath
}

func TestUpdateAuthMethod(t *testing.T) {
	t.Run("switch to none", func(t *testing.T) {
		handler, _ := setupAuthTestWithConfig(t)

		body, _ := json.Marshal(map[string]string{"method": "none"})
		req := httptest.NewRequest(http.MethodPut, "/api/auth/method", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.UpdateAuthMethod(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["success"] != true {
			t.Error("expected success=true")
		}
		if resp["method"] != "none" {
			t.Errorf("expected method 'none', got %v", resp["method"])
		}

		// Verify config was updated
		if handler.config.Auth.Method != "none" {
			t.Errorf("expected config method 'none', got %q", handler.config.Auth.Method)
		}
	})

	t.Run("switch to builtin with users", func(t *testing.T) {
		handler, _ := setupAuthTestWithConfig(t)

		body, _ := json.Marshal(map[string]string{"method": "builtin"})
		req := httptest.NewRequest(http.MethodPut, "/api/auth/method", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.UpdateAuthMethod(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["success"] != true {
			t.Error("expected success=true")
		}
		if resp["method"] != "builtin" {
			t.Errorf("expected method 'builtin', got %v", resp["method"])
		}

		if handler.config.Auth.Method != "builtin" {
			t.Errorf("expected config method 'builtin', got %q", handler.config.Auth.Method)
		}
	})

	t.Run("builtin no users", func(t *testing.T) {
		handler, _ := setupAuthTestWithConfig(t)

		// Remove all users so the store is empty
		if err := handler.userStore.Delete("admin"); err != nil {
			t.Fatalf("failed to delete user: %v", err)
		}

		body, _ := json.Marshal(map[string]string{"method": "builtin"})
		req := httptest.NewRequest(http.MethodPut, "/api/auth/method", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.UpdateAuthMethod(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["success"] != false {
			t.Error("expected success=false")
		}
	})

	t.Run("forward_auth with proxies", func(t *testing.T) {
		handler, _ := setupAuthTestWithConfig(t)

		body, _ := json.Marshal(map[string]interface{}{
			"method":          "forward_auth",
			"trusted_proxies": []string{"10.0.0.0/8"},
			"headers": map[string]string{
				"user":  "X-Auth-User",
				"email": "X-Auth-Email",
			},
		})
		req := httptest.NewRequest(http.MethodPut, "/api/auth/method", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.UpdateAuthMethod(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["success"] != true {
			t.Error("expected success=true")
		}
		if resp["method"] != "forward_auth" {
			t.Errorf("expected method 'forward_auth', got %v", resp["method"])
		}

		if handler.config.Auth.Method != "forward_auth" {
			t.Errorf("expected config method 'forward_auth', got %q", handler.config.Auth.Method)
		}
		if len(handler.config.Auth.TrustedProxies) != 1 || handler.config.Auth.TrustedProxies[0] != "10.0.0.0/8" {
			t.Errorf("expected trusted proxies to be set, got %v", handler.config.Auth.TrustedProxies)
		}
	})

	t.Run("forward_auth no proxies", func(t *testing.T) {
		handler, _ := setupAuthTestWithConfig(t)

		body, _ := json.Marshal(map[string]interface{}{
			"method":          "forward_auth",
			"trusted_proxies": []string{},
		})
		req := httptest.NewRequest(http.MethodPut, "/api/auth/method", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.UpdateAuthMethod(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("invalid method", func(t *testing.T) {
		handler, _ := setupAuthTestWithConfig(t)

		body, _ := json.Marshal(map[string]string{"method": "magic"})
		req := httptest.NewRequest(http.MethodPut, "/api/auth/method", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.UpdateAuthMethod(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["success"] != false {
			t.Error("expected success=false")
		}
	})

	t.Run("bad JSON", func(t *testing.T) {
		handler, _ := setupAuthTestWithConfig(t)

		req := httptest.NewRequest(http.MethodPut, "/api/auth/method", bytes.NewReader([]byte("not json")))
		w := httptest.NewRecorder()

		handler.UpdateAuthMethod(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("config file is persisted", func(t *testing.T) {
		handler, configPath := setupAuthTestWithConfig(t)

		body, _ := json.Marshal(map[string]string{"method": "builtin"})
		req := httptest.NewRequest(http.MethodPut, "/api/auth/method", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.UpdateAuthMethod(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		// Verify the config file was written
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read config file: %v", err)
		}
		if len(data) == 0 {
			t.Error("expected config file to have content")
		}
	})
}

func TestSyncUsersToConfig(t *testing.T) {
	t.Run("nil config returns nil", func(t *testing.T) {
		handler, _ := setupAuthTest(t)

		err := handler.syncUsersToConfig()
		if err != nil {
			t.Errorf("expected nil error with nil config, got %v", err)
		}
	})

	t.Run("persists users to config file", func(t *testing.T) {
		handler, configPath := setupAuthTestWithConfig(t)

		// Add another user before syncing
		hash, _ := auth.HashPassword("password123")
		if err := handler.userStore.Add(&auth.User{
			ID:           "newuser",
			Username:     "newuser",
			PasswordHash: hash,
			Role:         "user",
			Email:        "new@example.com",
			DisplayName:  "New User",
		}); err != nil {
			t.Fatalf("failed to add user: %v", err)
		}

		err := handler.syncUsersToConfig()
		if err != nil {
			t.Fatalf("syncUsersToConfig failed: %v", err)
		}

		// Verify config file was written
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read config file: %v", err)
		}
		content := string(data)

		// Check that users are in the file
		if !contains(content, "admin") {
			t.Error("expected config file to contain admin user")
		}
		if !contains(content, "newuser") {
			t.Error("expected config file to contain newuser")
		}

		// Verify the in-memory config was also updated
		if len(handler.config.Auth.Users) != 2 {
			t.Errorf("expected 2 users in config, got %d", len(handler.config.Auth.Users))
		}
	})
}

// contains is a test helper for checking substring presence.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
