package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionStore(t *testing.T) {
	store := NewSessionStore("test_session", time.Hour, false)

	t.Run("create and get session", func(t *testing.T) {
		session, err := store.Create("user123", "testuser", RoleUser)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		if session.ID == "" {
			t.Error("Expected non-empty session ID")
		}
		if session.Username != "testuser" {
			t.Errorf("Expected username testuser, got %s", session.Username)
		}

		// Retrieve session
		retrieved := store.Get(session.ID)
		if retrieved == nil {
			t.Fatal("Expected to find session")
			return
		}
		if retrieved.Username != session.Username {
			t.Errorf("Expected username %s, got %s", session.Username, retrieved.Username)
		}
	})

	t.Run("get nonexistent session", func(t *testing.T) {
		session := store.Get("nonexistent-id")
		if session != nil {
			t.Error("Expected session not found")
		}
	})

	t.Run("delete session", func(t *testing.T) {
		session, _ := store.Create("user456", "deletetest", RoleUser)

		store.Delete(session.ID)

		retrieved := store.Get(session.ID)
		if retrieved != nil {
			t.Error("Expected session to be deleted")
		}
	})

	t.Run("refresh session", func(t *testing.T) {
		session, _ := store.Create("user789", "refreshtest", RoleUser)
		originalExpiry := session.ExpiresAt

		time.Sleep(10 * time.Millisecond)
		store.Refresh(session.ID)

		refreshed := store.Get(session.ID)
		if refreshed == nil {
			t.Fatal("Expected to find session")
			return
		}
		if !refreshed.ExpiresAt.After(originalExpiry) {
			t.Error("Expected expiry to be extended")
		}
	})

	t.Run("session expiration", func(t *testing.T) {
		shortStore := NewSessionStore("short_session", 10*time.Millisecond, false)
		session, _ := shortStore.Create("userexp", "expiretest", RoleUser)

		// Verify session exists initially
		retrieved := shortStore.Get(session.ID)
		if retrieved == nil {
			t.Fatal("Expected session to exist initially")
		}

		// Wait for expiration
		time.Sleep(20 * time.Millisecond)

		// Session should be expired
		expired := shortStore.Get(session.ID)
		if expired != nil {
			t.Error("Expected expired session to be invalid")
		}
	})

	t.Run("count sessions", func(t *testing.T) {
		countStore := NewSessionStore("count_session", time.Hour, false)

		if _, err := countStore.Create("u1", "user1", RoleUser); err != nil {
			t.Fatalf("failed to create session: %v", err)
		}
		if _, err := countStore.Create("u2", "user2", RoleUser); err != nil {
			t.Fatalf("failed to create session: %v", err)
		}
		if _, err := countStore.Create("u3", "user3", RoleAdmin); err != nil {
			t.Fatalf("failed to create session: %v", err)
		}

		if countStore.Count() != 3 {
			t.Errorf("Expected 3 sessions, got %d", countStore.Count())
		}
	})
}

func TestSessionIsExpired(t *testing.T) {
	session := &Session{
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	if !session.IsExpired() {
		t.Error("Expected session to be expired")
	}

	session.ExpiresAt = time.Now().Add(time.Hour)
	if session.IsExpired() {
		t.Error("Expected session to not be expired")
	}
}

func TestSessionConcurrency(t *testing.T) {
	store := NewSessionStore("concurrent_session", time.Hour, false)

	done := make(chan bool)

	// Create sessions concurrently
	for i := 0; i < 100; i++ {
		go func() {
			session, _ := store.Create("user", "concurrent", RoleUser)
			store.Get(session.ID)
			store.Refresh(session.ID)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	// Verify count is correct
	if store.Count() != 100 {
		t.Errorf("Expected 100 sessions, got %d", store.Count())
	}
}

// --- cleanup (inline logic test) ---

func TestSessionCleanup(t *testing.T) {
	store := NewSessionStore("test_session", time.Hour, false)

	// Create sessions with varying expiry times
	s1, _ := store.Create("user1", "alice", RoleUser)
	s2, _ := store.Create("user2", "bob", RoleUser)
	s3, _ := store.Create("user3", "carol", RoleUser)

	// Manually expire s2 and s3
	store.mu.Lock()
	store.sessions[s2.ID].ExpiresAt = time.Now().Add(-time.Minute)
	store.sessions[s3.ID].ExpiresAt = time.Now().Add(-time.Hour)
	store.mu.Unlock()

	// Simulate what cleanup() does on each tick
	store.mu.Lock()
	for id, session := range store.sessions {
		if session.IsExpired() {
			delete(store.sessions, id)
		}
	}
	store.mu.Unlock()

	// s1 should remain, s2 and s3 should be cleaned up
	if store.Get(s1.ID) == nil {
		t.Error("expected s1 to still exist")
	}
	if store.Get(s2.ID) != nil {
		t.Error("expected s2 to be cleaned up")
	}
	if store.Get(s3.ID) != nil {
		t.Error("expected s3 to be cleaned up")
	}
	if store.Count() != 1 {
		t.Errorf("expected 1 remaining session, got %d", store.Count())
	}
}

func TestSessionCleanup_NoExpired(t *testing.T) {
	store := NewSessionStore("test_session", time.Hour, false)

	_, _ = store.Create("user1", "alice", RoleUser)
	_, _ = store.Create("user2", "bob", RoleUser)

	// Simulate cleanup -- nothing should be removed
	store.mu.Lock()
	for id, session := range store.sessions {
		if session.IsExpired() {
			delete(store.sessions, id)
		}
	}
	store.mu.Unlock()

	if store.Count() != 2 {
		t.Errorf("expected 2 sessions, got %d", store.Count())
	}
}

func TestSessionCleanup_AllExpired(t *testing.T) {
	store := NewSessionStore("test_session", time.Hour, false)

	s1, _ := store.Create("user1", "alice", RoleUser)
	s2, _ := store.Create("user2", "bob", RoleUser)

	// Expire all sessions
	store.mu.Lock()
	store.sessions[s1.ID].ExpiresAt = time.Now().Add(-time.Minute)
	store.sessions[s2.ID].ExpiresAt = time.Now().Add(-time.Minute)
	store.mu.Unlock()

	// Simulate cleanup
	store.mu.Lock()
	for id, session := range store.sessions {
		if session.IsExpired() {
			delete(store.sessions, id)
		}
	}
	store.mu.Unlock()

	if store.Count() != 0 {
		t.Errorf("expected 0 sessions after cleanup, got %d", store.Count())
	}
}

func TestSessionCleanup_EmptyStore(t *testing.T) {
	store := NewSessionStore("test_session", time.Hour, false)

	// Simulate cleanup on empty store -- should not panic
	store.mu.Lock()
	for id, session := range store.sessions {
		if session.IsExpired() {
			delete(store.sessions, id)
		}
	}
	store.mu.Unlock()

	if store.Count() != 0 {
		t.Errorf("expected 0 sessions, got %d", store.Count())
	}
}

// --- DeleteByUserID ---

func TestDeleteByUserID(t *testing.T) {
	store := NewSessionStore("test_session", time.Hour, false)

	s1, _ := store.Create("user1", "alice", RoleUser)
	s2, _ := store.Create("user1", "alice", RoleUser)
	s3, _ := store.Create("user2", "bob", RoleUser)

	// Delete all sessions for user1 except s1
	store.DeleteByUserID("user1", s1.ID)

	// s1 should still exist
	if store.Get(s1.ID) == nil {
		t.Error("expected s1 to still exist")
	}
	// s2 should be deleted
	if store.Get(s2.ID) != nil {
		t.Error("expected s2 to be deleted")
	}
	// s3 should still exist (different user)
	if store.Get(s3.ID) == nil {
		t.Error("expected s3 to still exist")
	}
}

func TestDeleteByUserID_NoExcept(t *testing.T) {
	store := NewSessionStore("test_session", time.Hour, false)

	s1, _ := store.Create("user1", "alice", RoleUser)
	s2, _ := store.Create("user1", "alice", RoleUser)

	// Delete all sessions for user1 with empty except
	store.DeleteByUserID("user1", "")

	if store.Get(s1.ID) != nil {
		t.Error("expected s1 to be deleted")
	}
	if store.Get(s2.ID) != nil {
		t.Error("expected s2 to be deleted")
	}
}

// --- GetFromRequest ---

func TestGetFromRequest(t *testing.T) {
	store := NewSessionStore("test_cookie", time.Hour, false)
	session, _ := store.Create("user1", "alice", RoleUser)

	t.Run("valid cookie returns session", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "test_cookie", Value: session.ID})

		got := store.GetFromRequest(req)
		if got == nil {
			t.Fatal("expected session from request")
			return
		}
		if got.ID != session.ID {
			t.Errorf("expected session ID %s, got %s", session.ID, got.ID)
		}
	})

	t.Run("no cookie returns nil", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		got := store.GetFromRequest(req)
		if got != nil {
			t.Error("expected nil for request without cookie")
		}
	})

	t.Run("wrong cookie name returns nil", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "wrong_cookie", Value: session.ID})
		got := store.GetFromRequest(req)
		if got != nil {
			t.Error("expected nil for wrong cookie name")
		}
	})

	t.Run("invalid session ID returns nil", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "test_cookie", Value: "invalid-id"})
		got := store.GetFromRequest(req)
		if got != nil {
			t.Error("expected nil for invalid session ID")
		}
	})
}

// --- SetCookie ---

func TestSetCookie(t *testing.T) {
	store := NewSessionStore("muximux_session", time.Hour, true)
	session, _ := store.Create("user1", "alice", RoleUser)

	rec := httptest.NewRecorder()
	store.SetCookie(rec, session)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}

	c := cookies[0]
	if c.Name != "muximux_session" {
		t.Errorf("expected cookie name muximux_session, got %s", c.Name)
	}
	if c.Value != session.ID {
		t.Errorf("expected cookie value %s, got %s", session.ID, c.Value)
	}
	if !c.HttpOnly {
		t.Error("expected HttpOnly cookie")
	}
	if !c.Secure {
		t.Error("expected Secure cookie")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("expected SameSite Lax, got %v", c.SameSite)
	}
	if c.Path != "/" {
		t.Errorf("expected path /, got %s", c.Path)
	}
}

func TestSetCookie_NotSecure(t *testing.T) {
	store := NewSessionStore("test", time.Hour, false)
	session, _ := store.Create("user1", "alice", RoleUser)

	rec := httptest.NewRecorder()
	store.SetCookie(rec, session)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].Secure {
		t.Error("expected non-Secure cookie when configured with secure=false")
	}
}

// --- ClearCookie ---

func TestClearCookie(t *testing.T) {
	store := NewSessionStore("muximux_session", time.Hour, true)

	rec := httptest.NewRecorder()
	store.ClearCookie(rec)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}

	c := cookies[0]
	if c.Name != "muximux_session" {
		t.Errorf("expected cookie name muximux_session, got %s", c.Name)
	}
	if c.Value != "" {
		t.Errorf("expected empty cookie value, got %s", c.Value)
	}
	if c.MaxAge != -1 {
		t.Errorf("expected MaxAge -1, got %d", c.MaxAge)
	}
}

// --- Refresh non-existent session ---

func TestRefresh_NonExistent(t *testing.T) {
	store := NewSessionStore("test", time.Hour, false)
	// Should not panic
	store.Refresh("non-existent-id")
}

// --- generateSessionID ---

func TestGenerateSessionID(t *testing.T) {
	id1, err := generateSessionID()
	if err != nil {
		t.Fatalf("generateSessionID failed: %v", err)
	}
	if id1 == "" {
		t.Error("expected non-empty session ID")
	}

	id2, err := generateSessionID()
	if err != nil {
		t.Fatalf("generateSessionID failed: %v", err)
	}

	if id1 == id2 {
		t.Error("expected unique session IDs")
	}
}
