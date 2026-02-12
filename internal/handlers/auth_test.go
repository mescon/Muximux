package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mescon/muximux/v3/internal/auth"
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

	handler := NewAuthHandler(sessionStore, userStore)
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

func TestSetOIDCProvider(t *testing.T) {
	ss := auth.NewSessionStore("test", time.Hour, false)
	us := auth.NewUserStore()
	handler := NewAuthHandler(ss, us)

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
