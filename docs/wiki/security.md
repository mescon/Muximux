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

- **Bcrypt password hashing** -- All user passwords are hashed with bcrypt (cost 10) before storage. Plaintext passwords never touch disk.
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

- **Structured audit logging** -- Security-relevant events are logged with `source=audit`, making them easy to filter from operational logs. Audited events include:
  - User login (success and failure)
  - User logout
  - Password changes
  - User creation, modification, and deletion
  - Auth method changes
  - API key updates
  - Config import/export/restore
  - OIDC authentication events
  - API key authentication failures
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
| `Permissions-Policy` | `camera=(), microphone=(), geolocation=()` | Disables unnecessary browser APIs |

### Content Security Policy

```
default-src 'self';
script-src 'self';
style-src 'self' 'unsafe-inline' https://fonts.googleapis.com;
font-src 'self' https://fonts.gstatic.com;
img-src 'self' data: blob: https:;
connect-src 'self' ws: wss:;
frame-src *;
object-src 'none';
base-uri 'self'
```

**Design decisions:**

- **`frame-src *`** -- Muximux's core feature is embedding arbitrary web applications in iframes. Restricting frame sources would break the primary use case.
- **`style-src 'unsafe-inline'`** -- Required for Svelte component scoped styles and CSS custom property theming. The risk is mitigated by the strict `script-src 'self'` policy, which prevents inline script execution.
- **`img-src https:`** -- Allows external icon URLs (url-type app icons, favicons). Icons from Dashboard Icons and Lucide are served through Muximux's backend API, so they're covered by `'self'`.
- **`connect-src ws: wss:`** -- WebSocket connections for real-time log streaming.
- **No HSTS** -- Muximux can run over plain HTTP in trusted networks. HSTS is left to the reverse proxy or TLS configuration.

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
