package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	password := "testpassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if hash == "" {
		t.Error("Expected non-empty hash")
	}

	// Verify the hash is valid bcrypt
	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		t.Error("Generated hash does not verify against original password")
	}
}

func TestUserStore(t *testing.T) {
	store := NewUserStore()

	// Load users from config
	configs := []UserConfig{
		{
			Username:     "admin",
			PasswordHash: mustHash("adminpass"),
			Role:         RoleAdmin,
		},
		{
			Username:     "user",
			PasswordHash: mustHash("userpass"),
			Role:         RoleUser,
		},
	}
	store.LoadFromConfig(configs)

	t.Run("authenticate valid user", func(t *testing.T) {
		user, err := store.Authenticate("admin", "adminpass")
		if err != nil {
			t.Fatalf("Authenticate failed: %v", err)
		}
		if user.Username != "admin" {
			t.Errorf("Expected username admin, got %s", user.Username)
		}
		if user.Role != RoleAdmin {
			t.Errorf("Expected role admin, got %s", user.Role)
		}
	})

	t.Run("authenticate wrong password", func(t *testing.T) {
		_, err := store.Authenticate("admin", "wrongpass")
		if err == nil {
			t.Error("Expected error for wrong password")
		}
	})

	t.Run("authenticate nonexistent user", func(t *testing.T) {
		_, err := store.Authenticate("nobody", "password")
		if err == nil {
			t.Error("Expected error for nonexistent user")
		}
	})

	t.Run("get user", func(t *testing.T) {
		user := store.Get("user")
		if user == nil {
			t.Fatal("Expected to find user")
		}
		if user.Username != "user" {
			t.Errorf("Expected username user, got %s", user.Username)
		}
	})

	t.Run("get nonexistent user", func(t *testing.T) {
		user := store.Get("nobody")
		if user != nil {
			t.Error("Expected user not found")
		}
	})

	t.Run("count users", func(t *testing.T) {
		if store.Count() != 2 {
			t.Errorf("Expected 2 users, got %d", store.Count())
		}
	})

	t.Run("has admin", func(t *testing.T) {
		if !store.HasAdmin() {
			t.Error("Expected HasAdmin to return true")
		}
	})
}

func TestUserStoreOperations(t *testing.T) {
	store := NewUserStore()

	t.Run("add user", func(t *testing.T) {
		user := &User{
			ID:       "newuser",
			Username: "newuser",
			Role:     RoleUser,
		}
		err := store.Add(user)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}

		retrieved := store.Get("newuser")
		if retrieved == nil {
			t.Error("Expected to find added user")
		}
	})

	t.Run("add duplicate user", func(t *testing.T) {
		user := &User{
			ID:       "newuser",
			Username: "newuser",
			Role:     RoleUser,
		}
		err := store.Add(user)
		if err == nil {
			t.Error("Expected error for duplicate user")
		}
	})

	t.Run("update user", func(t *testing.T) {
		user := &User{
			ID:       "newuser",
			Username: "newuser",
			Role:     RoleAdmin,
			Email:    "new@example.com",
		}
		err := store.Update(user)
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		retrieved := store.Get("newuser")
		if retrieved.Role != RoleAdmin {
			t.Errorf("Expected role admin, got %s", retrieved.Role)
		}
	})

	t.Run("update nonexistent user", func(t *testing.T) {
		user := &User{Username: "nobody"}
		err := store.Update(user)
		if err == nil {
			t.Error("Expected error for nonexistent user")
		}
	})

	t.Run("delete user", func(t *testing.T) {
		err := store.Delete("newuser")
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		retrieved := store.Get("newuser")
		if retrieved != nil {
			t.Error("Expected user to be deleted")
		}
	})

	t.Run("delete nonexistent user", func(t *testing.T) {
		err := store.Delete("nobody")
		if err == nil {
			t.Error("Expected error for nonexistent user")
		}
	})
}

func TestUserStoreList(t *testing.T) {
	store := NewUserStore()
	store.LoadFromConfig([]UserConfig{
		{Username: "user1", PasswordHash: mustHash("pass1"), Role: RoleUser},
		{Username: "user2", PasswordHash: mustHash("pass2"), Role: RoleAdmin},
	})

	users := store.List()
	if len(users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(users))
	}

	// Verify password hashes are not exposed in list
	for _, u := range users {
		if u.PasswordHash != "" {
			t.Error("Expected password hash to be empty in list")
		}
	}
}

func TestUserStoreListWithHashes(t *testing.T) {
	store := NewUserStore()
	store.LoadFromConfig([]UserConfig{
		{Username: "user1", PasswordHash: mustHash("pass1"), Role: RoleUser, Email: "u1@example.com"},
		{Username: "user2", PasswordHash: mustHash("pass2"), Role: RoleAdmin, DisplayName: "User Two"},
	})

	users := store.ListWithHashes()
	if len(users) != 2 {
		t.Fatalf("Expected 2 users, got %d", len(users))
	}

	// Verify password hashes ARE included (unlike List())
	for _, u := range users {
		if u.PasswordHash == "" {
			t.Errorf("Expected non-empty password hash for user %q", u.Username)
		}
	}

	// Verify all other fields are present
	found := make(map[string]*User)
	for _, u := range users {
		found[u.Username] = u
	}

	u1, ok := found["user1"]
	if !ok {
		t.Fatal("Expected to find user1")
	}
	if u1.ID != "user1" {
		t.Errorf("Expected ID 'user1', got %q", u1.ID)
	}
	if u1.Role != RoleUser {
		t.Errorf("Expected role 'user', got %q", u1.Role)
	}
	if u1.Email != "u1@example.com" {
		t.Errorf("Expected email 'u1@example.com', got %q", u1.Email)
	}

	u2, ok := found["user2"]
	if !ok {
		t.Fatal("Expected to find user2")
	}
	if u2.Role != RoleAdmin {
		t.Errorf("Expected role 'admin', got %q", u2.Role)
	}
	if u2.DisplayName != "User Two" {
		t.Errorf("Expected display name 'User Two', got %q", u2.DisplayName)
	}

	// Verify that the original password can be verified against the hash
	err := bcrypt.CompareHashAndPassword([]byte(u1.PasswordHash), []byte("pass1"))
	if err != nil {
		t.Error("Expected user1 hash to verify against original password")
	}
}

func mustHash(password string) string {
	hash, err := HashPassword(password)
	if err != nil {
		panic(err)
	}
	return hash
}
