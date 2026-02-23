package auth

import "testing"

func TestForwardAuthHeadersFromMap(t *testing.T) {
	t.Run("nil map returns empty struct", func(t *testing.T) {
		got := ForwardAuthHeadersFromMap(nil)
		if got != (ForwardAuthHeaders{}) {
			t.Errorf("expected zero-value ForwardAuthHeaders, got %+v", got)
		}
	})

	t.Run("empty map returns empty struct", func(t *testing.T) {
		got := ForwardAuthHeadersFromMap(map[string]string{})
		if got != (ForwardAuthHeaders{}) {
			t.Errorf("expected zero-value ForwardAuthHeaders, got %+v", got)
		}
	})

	t.Run("all keys present", func(t *testing.T) {
		m := map[string]string{
			"user":   "Remote-User",
			"email":  "Remote-Email",
			"groups": "Remote-Groups",
			"name":   "Remote-Name",
		}
		got := ForwardAuthHeadersFromMap(m)
		if got.User != "Remote-User" {
			t.Errorf("expected User 'Remote-User', got %q", got.User)
		}
		if got.Email != "Remote-Email" {
			t.Errorf("expected Email 'Remote-Email', got %q", got.Email)
		}
		if got.Groups != "Remote-Groups" {
			t.Errorf("expected Groups 'Remote-Groups', got %q", got.Groups)
		}
		if got.Name != "Remote-Name" {
			t.Errorf("expected Name 'Remote-Name', got %q", got.Name)
		}
	})

	t.Run("partial keys", func(t *testing.T) {
		m := map[string]string{
			"user":  "X-User",
			"email": "X-Email",
		}
		got := ForwardAuthHeadersFromMap(m)
		if got.User != "X-User" {
			t.Errorf("expected User 'X-User', got %q", got.User)
		}
		if got.Email != "X-Email" {
			t.Errorf("expected Email 'X-Email', got %q", got.Email)
		}
		if got.Groups != "" {
			t.Errorf("expected empty Groups, got %q", got.Groups)
		}
		if got.Name != "" {
			t.Errorf("expected empty Name, got %q", got.Name)
		}
	})

	t.Run("extra keys are ignored", func(t *testing.T) {
		m := map[string]string{
			"user":    "MyUser",
			"email":   "MyEmail",
			"groups":  "MyGroups",
			"name":    "MyName",
			"unknown": "ignored",
		}
		got := ForwardAuthHeadersFromMap(m)
		if got.User != "MyUser" {
			t.Errorf("expected User 'MyUser', got %q", got.User)
		}
		// Struct should only have the four known fields; extra keys don't cause errors.
		if got.Email != "MyEmail" || got.Groups != "MyGroups" || got.Name != "MyName" {
			t.Errorf("unexpected result: %+v", got)
		}
	})
}
