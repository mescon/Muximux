package auth

import (
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

		countStore.Create("u1", "user1", RoleUser)
		countStore.Create("u2", "user2", RoleUser)
		countStore.Create("u3", "user3", RoleAdmin)

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
		go func(n int) {
			session, _ := store.Create("user", "concurrent", RoleUser)
			store.Get(session.ID)
			store.Refresh(session.ID)
			done <- true
		}(i)
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
