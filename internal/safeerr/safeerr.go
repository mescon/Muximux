// Package safeerr turns outbound HTTP errors into text that is safe to log
// and show. Go's HTTP client and URL parser quote whole URLs (and, for a
// bad redirect, the Location header) in their errors, and a webhook or
// health URL often carries a secret in its path or query string.
package safeerr

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/url"
)

// Describe returns a short description of err that never contains a URL,
// path or query. Timeouts and connection, DNS and TLS failures keep their
// detail (those errors name only a host and port); anything else becomes
// "request failed".
func Describe(err error) string {
	if err == nil {
		return ""
	}
	op := ""
	var ue *url.Error
	if errors.As(err, &ue) {
		op = ue.Op + ": "
		if ue.Op == "parse" {
			return "invalid URL"
		}
	}
	var ne net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()) {
		return op + "timed out"
	}
	if errors.Is(err, context.Canceled) {
		return op + "canceled"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return op + dnsErr.Error()
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return op + opErr.Error()
	}
	var unknownAuth x509.UnknownAuthorityError
	var hostErr x509.HostnameError
	var certInvalid x509.CertificateInvalidError
	var recordErr tls.RecordHeaderError
	switch {
	case errors.As(err, &unknownAuth), errors.As(err, &hostErr), errors.As(err, &certInvalid):
		return op + "TLS certificate not trusted"
	case errors.As(err, &recordErr):
		return op + "TLS handshake failed"
	}
	return op + "request failed"
}
