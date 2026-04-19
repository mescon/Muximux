# Security

[![OWASP ASVS](https://img.shields.io/badge/OWASP_ASVS-Level_2-005571?logo=owasp)](#security-standards)
[![Security](https://sonarcloud.io/api/project_badges/measure?project=mescon_Muximux&metric=security_rating)](https://sonarcloud.io/summary/overall?id=mescon_Muximux)
[![CodeQL](https://github.com/mescon/Muximux/actions/workflows/codeql.yml/badge.svg)](https://github.com/mescon/Muximux/actions/workflows/codeql.yml)
[![Codecov](https://codecov.io/gh/mescon/Muximux/branch/main/graph/badge.svg)](https://codecov.io/gh/mescon/Muximux)

Muximux is designed with security as a first-class concern. This page documents the security measures built into the application, the standards they're based on, and how they protect your deployment.

## Security Standards

Muximux's security posture is evaluated against the [OWASP Application Security Verification Standard (ASVS) 4.0](https://owasp.org/www-project-application-security-verification-standard/), Level 2. ASVS is the industry standard for web application security requirements, maintained by the Open Worldwide Application Security Project.

**Why Level 2?** ASVS defines three levels: L1 (minimum), L2 (standard -- recommended for applications that handle sensitive data), and L3 (advanced -- banking, healthcare, critical infrastructure). As a self-hosted application portal that manages access to your services, Level 2 is the appropriate target.

Many ASVS requirements (SOAP, GraphQL, LDAP, SMS/OTP, HSM, etc.) are not applicable to Muximux's architecture. The controls below cover all requirements that apply to a single-binary Go web application with cookie-based sessions.

---

## What's Protected

### Authentication (ASVS V2)

- **Bcrypt password hashing** -- All user passwords are hashed with bcrypt (cost 12) before storage. Plaintext passwords never touch disk. Successful logins silently re-hash passwords stored below the current target cost, so older accounts migrate forward on their own.
- **Bcrypt API key hashing** -- API keys are stored as bcrypt hashes in `config.yaml`. The original key cannot be recovered from the hash. Verification uses `bcrypt.CompareHashAndPassword`, which is constant-time and resistant to timing attacks.
- **Rate limiting** -- Login endpoints are rate-limited to 5 attempts per minute per IP address to prevent brute-force attacks.
- **OIDC ID token validation** -- When using OpenID Connect, Muximux validates the ID token's cryptographic signature (via JWKS), issuer, audience, expiry, and nonce. This prevents token forgery and replay attacks.
- **Nonce binding** -- OIDC authorization requests include a cryptographically random nonce that is verified in the callback to prevent replay attacks.
- **Environment variable secrets** -- Sensitive config values (OIDC client secrets, API keys) support `${ENV_VAR}` syntax to keep secrets out of config files. A startup warning is logged if the OIDC client secret is stored as plaintext.

### Session Management (ASVS V3)

- **HttpOnly cookies** -- Session cookies are set with `HttpOnly`, preventing JavaScript access and mitigating XSS-based session theft.
- **SameSite=Lax** -- Session cookies use `SameSite=Lax`, which prevents cross-site request forgery via form submissions while allowing normal navigation.
- **Configurable Secure flag** -- When `secure_cookies: true` is set (recommended for HTTPS deployments), cookies are only sent over encrypted connections.
- **Configurable session lifetime** -- `session_max_age` controls how long sessions last (default: 24h). Sessions are automatically refreshed on activity.
- **Session invalidation** -- Changing a user's password invalidates all other sessions for that user.
- **Cryptographic session tokens** -- Session IDs are generated using `crypto/rand` with 32 bytes of entropy.

### Access Control (ASVS V4)

- **Role-based access control** -- Three roles (`admin`, `power-user`, `user`) with enforced privilege separation. Admin operations require the `admin` role; regular users cannot escalate privileges.
- **Per-app access control** -- Apps can be restricted by role or by specific usernames using the `access` and `min_role` fields.
- **Forward auth proxy validation** -- When using forward auth, only connections from `trusted_proxies` CIDR ranges are accepted. Auth headers from untrusted sources are rejected. The check uses the TCP source address (`RemoteAddr`), not forwarded headers.

### Input Validation (ASVS V5)

- **CSRF protection** -- State-changing API requests (POST, PUT) require `application/json` Content-Type, which triggers CORS preflight from cross-origin requests. Browsers cannot send cross-origin JSON requests without server approval.
- **Environment variable expansion safety** -- Only `${VAR}` syntax is expanded; bare `$VAR` references are left untouched. This prevents corruption of values like bcrypt hashes (`$2a$10$...`).

### Cryptography (ASVS V6)

- **TLS support** -- Automatic HTTPS via Let's Encrypt (specify `tls.domain` and `tls.email`) or manual certificates (`tls.cert` and `tls.key`). Caddy handles certificate management, renewal, and OCSP stapling.
- **No custom cryptography** -- All cryptographic operations use Go standard library (`crypto/rand`, `crypto/sha256`) or well-established libraries (`golang.org/x/crypto/bcrypt`, `coreos/go-oidc/v3`).

### Logging & Monitoring (ASVS V7)

- **Structured audit logging** -- Security-relevant events are logged with `source=audit` and a unique `request_id` for correlation, making them easy to filter and trace. Audited events include:
  - User login (success and failure)
  - User logout
  - Password changes
  - User creation, modification, and deletion
  - Auth method changes
  - API key updates
  - Config import/export/restore
  - OIDC authentication events
  - API key authentication failures
- **Centralized error logging** -- All HTTP error responses are logged at appropriate severity: 5xx at ERROR, 401/403 at WARN, other 4xx at DEBUG. No silent errors.
- **Real-time log streaming** -- The built-in log viewer supports WebSocket-based live streaming with source-based filtering, so you can monitor audit events in real time.
- **File logging** -- Logs are written to both stdout and `data/muximux.log` for persistence.

### Data Protection (ASVS V8)

- **Sensitive data stripping** -- Config exports automatically strip password hashes, API key hashes, and OIDC client secrets before download.
- **No plaintext secrets** -- Passwords and API keys are stored as one-way bcrypt hashes. OIDC client secrets support environment variable references.
- **File permissions** -- Config files are written with `0600` permissions (owner read/write only).

### HTTP Security Headers (ASVS V14)

Every response from Muximux includes:

| Header | Value | Purpose |
|--------|-------|---------|
| `Content-Security-Policy` | See below | Controls which resources the browser can load |
| `X-Content-Type-Options` | `nosniff` | Prevents MIME-type sniffing |
| `X-Frame-Options` | `SAMEORIGIN` | Prevents clickjacking of the Muximux UI |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | Limits referrer information leakage |
| `Permissions-Policy` | `camera=*, microphone=*, geolocation=*, …` | Opens delegatable APIs at the document level so per-app iframe `allow` attributes can scope them. See [Architectural Trade-offs](#architectural-trade-offs). |
| `Strict-Transport-Security` | `max-age=31536000` (on TLS only) | Pins the browser to HTTPS for a year once the site has been seen over TLS. Not sent on plain-HTTP requests. |

### Content Security Policy

```
default-src 'self';
script-src 'self';
style-src 'self' 'unsafe-inline' https://fonts.googleapis.com;
font-src 'self' https://fonts.gstatic.com;
img-src 'self' data: blob: https:;
connect-src 'self';
frame-src *;
frame-ancestors 'self';
form-action 'self';
manifest-src 'self' blob:;
object-src 'none';
base-uri 'self'
```

**Design decisions:**

- **`frame-src *`** -- Muximux's core feature is embedding arbitrary web applications in iframes. Restricting frame sources would break the primary use case.
- **`frame-ancestors 'self'`** -- Modern clickjacking protection that replaces reliance on the deprecated `X-Frame-Options` alone. Muximux will only load inside its own origin.
- **`form-action 'self'`** -- Blocks a form-hijack that would POST user submissions to an attacker origin.
- **`connect-src 'self'`** -- Same-origin fetches and WebSocket (`ws://`/`wss://` on the same host) both fall under `'self'`; dropping the wildcard `ws: wss:` closed a stealth exfiltration lane for any injected script.
- **`style-src 'unsafe-inline'`** -- Required for Svelte component scoped styles and CSS custom property theming. Risk mitigated by the strict `script-src 'self'` policy, which prevents inline script execution.
- **`img-src https:`** -- Allows external icon URLs (url-type app icons, favicons). Icons from Dashboard Icons and Lucide are served through Muximux's backend API, so they're covered by `'self'`.
- **HSTS on TLS only** -- Muximux emits `Strict-Transport-Security: max-age=31536000` on requests that actually arrived over TLS. Plain-HTTP deployments in trusted networks are unaffected; TLS deployments get automatic HTTPS pinning without extra configuration.

### Proxied Response Isolation

When Muximux proxies requests to backend applications, it strips security-sensitive headers from the proxied responses to prevent backend apps from interfering with Muximux's own security headers. This includes stripping any `Content-Security-Policy`, `X-Frame-Options`, and similar headers that would conflict with the dashboard's iframe embedding.

---

## Architecture Security Notes

### Single Binary, No External Dependencies

Muximux runs as a single Go binary with an embedded web frontend. There are no external database servers, message queues, or caches to secure. All state lives in one YAML file with strict file permissions.

### Reverse Proxy Security

When `proxy: true` is enabled for an app, Muximux acts as a reverse proxy. This means:

- All requests to that app go through Muximux's authentication middleware
- Backend apps don't need their own authentication (though defense-in-depth is always recommended)
- TLS certificate verification for backends is configurable per-app (`proxy_skip_tls_verify`)

When `proxy: false`, the browser connects directly to the app. Muximux has no control over those requests.

### Forward Auth Header Validation

Forward auth mode is inherently trust-based -- Muximux trusts headers from the auth proxy. The `trusted_proxies` check ensures that only connections from known proxy IPs are accepted. If an attacker connects directly (bypassing the proxy), the auth headers are rejected.

---

## What ASVS Level 2 Requires That Doesn't Apply

For transparency, here are ASVS categories that are not applicable to Muximux's architecture:

| Category | Why N/A |
|----------|---------|
| SOAP/XML web services | Muximux uses JSON REST APIs only |
| GraphQL | Not used |
| LDAP injection | No LDAP integration |
| SMS/phone OTP | Not offered as an auth method |
| Hardware security modules | Self-hosted single-binary app |
| File upload malware scanning | No user file uploads (only config import and icon caching) |
| Multi-tenant data isolation | Single-tenant application |
| WebSocket message authentication | WebSocket is used for read-only log streaming, not commands |

---

## Recommendations for Deployers

1. **Enable HTTPS** -- Use `tls.domain` for automatic Let's Encrypt certificates, or place Muximux behind a TLS-terminating reverse proxy.
2. **Set `secure_cookies: true`** when serving over HTTPS.
3. **Use environment variables for secrets** -- Store OIDC client secrets and API keys as `${ENV_VAR}` references rather than plaintext in config.
4. **Enable authentication** -- The default is `auth: none`. Switch to `builtin`, `forward_auth`, or `oidc` for any internet-facing deployment.
5. **Use the reverse proxy for sensitive apps** -- Set `proxy: true` on apps that should be gated behind Muximux's authentication.
6. **Restrict network access** -- In a homelab, consider running Muximux on an internal network or behind a VPN for defense-in-depth.
7. **Monitor audit logs** -- Filter for `source=audit` in the log viewer to track authentication and configuration events.

---

## Architectural Trade-offs

These are design choices whose security implications are worth calling out so operators can weigh them against their threat model.

### Proxied apps share Muximux's origin

When an app is configured with `proxy: true`, the reverse proxy serves it at `/proxy/{slug}/` on Muximux's own origin. This enables a few headline features: same-origin iframe embedding without X-Frame-Options friction, transparent cookie scoping, and the runtime request interceptor that rewrites fetch/XHR/WebSocket URLs for SPAs with hard-coded paths.

The trade-off is that a compromised or intentionally malicious proxied app is not meaningfully isolated from Muximux itself:

- The iframe sandbox (`allow-scripts allow-same-origin`) specifically disables the "null origin" protection for same-origin content. An attacker who can run script inside the iframe can reach into Muximux's origin.
- Muximux deliberately strips `Content-Security-Policy` and `X-Frame-Options` from proxied responses, because the interceptor script needs to run and upstream CSP would block it. This means any reflected-XSS primitive the upstream had is no longer constrained by its own CSP when accessed through Muximux.
- Same-origin also means the session cookie is attached to `/proxy/...` requests, so cookie stripping in the proxy's Director (introduced as part of the batch-2 hardening) is load-bearing; Muximux's session no longer leaks to backends, but the proxied iframe itself is still same-origin from the browser's point of view.

Practically, this means: **trust a proxied app the way you trust its upstream, plus Muximux**. If you put a random untrusted web service behind `proxy: true`, any compromise of that service is a compromise of your Muximux admin surface. For apps you manage (Sonarr, Jellyfin, Home Assistant, etc.), this is fine; for third-party or multi-tenant apps, either leave `proxy: false` (iframes them as cross-origin) or don't embed them at all.

### Permissions-Policy is permissive

The HTTP `Permissions-Policy` header is the ceiling for iframe `allow` delegations. An iframe can only grant features the parent page is itself permitted to use, so Muximux ships with every delegatable feature set to `*`. This lets per-app iframe `allow` attributes scope camera/microphone/WebAuthn/etc. down to specific app origins. Muximux's own frontend never calls these APIs, so the wide ceiling does not broaden Muximux's own attack surface -- the narrowing happens per-iframe.

If you want to tighten this, per-app delegation is controlled by the `permissions` array on each app.
