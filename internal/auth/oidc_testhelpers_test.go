package auth

import "context"

// testAuthorizationURL starts a sign-in without a browser binding, so
// tests can drive HandleCallback without the binding cookie. Production
// code starts sign-ins only through HandleLogin, which always binds them.
func (p *OIDCProvider) testAuthorizationURL(ctx context.Context, redirectAfterLogin string) (string, error) {
	return p.authorizationURL(ctx, redirectAfterLogin, loginOptions{})
}
