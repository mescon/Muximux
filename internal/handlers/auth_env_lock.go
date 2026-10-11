package handlers

import (
	"strings"

	"github.com/mescon/muximux/v3/internal/config"
)

// authMethodEnvConflict returns why the environment does not allow
// switching the auth method to method, or "" when it does. A method set by
// MUXIMUX_AUTH_METHOD cannot change (re-saving the same method to edit
// other settings is fine); OIDC turned off by MUXIMUX_OIDC_ENABLED=false
// cannot be chosen; and while variables configure OIDC without
// MUXIMUX_AUTH_METHOD, leaving OIDC is refused because the next start
// would fail closed. Messages name variables, never values. The caller
// holds configMu.
func authMethodEnvConflict(cfg *config.Config, method string) string {
	if src, ok := cfg.EnvOverrides()[string(config.OverrideAuthMethod)]; ok {
		if method != cfg.Auth.Method {
			return "the auth method is set by " + src + "; change it there and restart"
		}
		return ""
	}
	if method == "oidc" {
		if src, ok := cfg.EnvOverrides()[string(config.OverrideOIDCEnabled)]; ok && !cfg.Auth.OIDC.Enabled {
			return "single sign-on is turned off by " + src + "; change it there and restart"
		}
		return ""
	}
	if vars := cfg.OIDCEnvConfiguringVars(); len(vars) > 0 {
		return "single sign-on is configured by " + strings.Join(vars, ", ") +
			"; remove those variables and restart before choosing another method"
	}
	return ""
}
