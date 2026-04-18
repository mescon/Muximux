package auth

const (
	errUserNotFound = "user not found"
	errAuthFailed   = "Authentication failed. Please try again."
)

// ForwardAuthHeadersFromMap creates ForwardAuthHeaders from a string map.
func ForwardAuthHeadersFromMap(m map[string]string) ForwardAuthHeaders {
	if m == nil {
		return ForwardAuthHeaders{}
	}
	return ForwardAuthHeaders{
		User:   m["user"],
		Email:  m["email"],
		Groups: m["groups"],
		Name:   m["name"],
	}
}
