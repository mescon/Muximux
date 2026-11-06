package safeerr

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

const secret = "s3cr3t-token"

func TestDescribe_NeverContainsTheURL(t *testing.T) {
	_, parseErr := url.Parse("http://exa mple.com/" + secret + "?token=" + secret) //nolint:staticcheck // deliberately invalid
	cases := map[string]error{
		"parse":   parseErr,
		"refused": &url.Error{Op: "Post", URL: "http://127.0.0.1:1/" + secret, Err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}},
		"dns":     &url.Error{Op: "Get", URL: "http://nx.invalid/?t=" + secret, Err: &net.DNSError{Err: "no such host", Name: "nx.invalid"}},
		"timeout": &url.Error{Op: "Post", URL: "http://h/" + secret, Err: context.DeadlineExceeded},
		"nested":  &url.Error{Op: "Get", URL: "http://h/", Err: errors.New(`failed to parse Location header "http://x/?token=` + secret + `"`)},
		"plain":   errors.New("something about http://h/?token=" + secret),
	}
	want := map[string]string{
		"parse": "invalid URL", "timeout": "Post: timed out", "nested": "Get: request failed", "plain": "request failed",
	}
	for name, err := range cases {
		got := Describe(err)
		if strings.Contains(got, secret) {
			t.Errorf("%s: %q leaks the secret", name, got)
		}
		if w, ok := want[name]; ok && got != w {
			t.Errorf("%s: got %q, want %q", name, got, w)
		}
	}
	if !strings.Contains(Describe(cases["refused"]), "connection refused") {
		t.Errorf("a connection error should keep its cause: %q", Describe(cases["refused"]))
	}
	if Describe(nil) != "" {
		t.Error("nil error should give an empty string")
	}
}

// A real malformed redirect from a real server: the client's nested error
// quotes the Location header.
func TestDescribe_RealBadRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "http://bad host/?token="+secret)
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get(srv.URL + "/?token=" + secret)
	if err == nil {
		resp.Body.Close()
		t.Skip("client accepted the malformed Location header")
	}
	if !strings.Contains(err.Error(), secret) {
		t.Logf("raw error does not contain the secret here: %v", err)
	}
	if got := Describe(err); strings.Contains(got, secret) {
		t.Errorf("Describe leaks the secret: %q", got)
	}
}
