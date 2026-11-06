# Security

[![OWASP ASVS](https://img.shields.io/badge/OWASP_ASVS-Level_2-005571?logo=owasp)](#security-standards)
[![Security](https://sonarcloud.io/api/project_badges/measure?project=mescon_Muximux&metric=security_rating)](https://sonarcloud.io/summary/overall?id=mescon_Muximux)
[![CodeQL](https://github.com/mescon/Muximux/actions/workflows/codeql.yml/badge.svg)](https://github.com/mescon/Muximux/actions/workflows/codeql.yml)
[![Codecov](https://codecov.io/gh/mescon/Muximux/branch/main/graph/badge.svg)](https://codecov.io/gh/mescon/Muximux)

This page covers the security measures built into Muximux, the standards they're based on, and how they protect your deployment.

## Security Standards

Muximux's security posture is evaluated against the [OWASP Application Security Verification Standard (ASVS) 4.0](https://owasp.org/www-project-application-security-verification-standard/), Level 2. ASVS is the industry standard for web application security requirements. The Open Worldwide Application Security Project maintains it.

ASVS defines three levels: L1 (minimum), L2 (standard, recommended for applications that handle sensitive data), and L3 (advanced: banking, healthcare, critical infrastructure). Muximux is a self-hosted application portal that manages access to your services. Level 2 is therefore the right target.

Many ASVS requirements (SOAP, GraphQL, LDAP, SMS/OTP, HSM, etc.) do not apply to Muximux's architecture. The controls below cover all requirements that apply to a single-binary Go web application with cookie-based sessions.

---

## What's Protected

### Authentication (ASVS V2)

- **Bcrypt password hashing** -- Muximux hashes all user passwords with bcrypt (cost 12) before storage. Plaintext passwords never touch disk. A password may be stored below the current target cost. On a successful login, Muximux silently re-hashes it, so older accounts migrate forward on their own.
- **Bcrypt API key hashing** -- Muximux stores API keys as bcrypt hashes in `config.yaml`. Nobody can recover the original key from the hash. Verification uses `bcrypt.CompareHashAndPassword`, which is constant-time and resistant to timing attacks.
- **Rate limiting** -- Login endpoints allow 5 attempts per minute per IP address. This prevents brute-force attacks.
- **OIDC ID token validation** -- When you use OpenID Connect (OIDC, a standard for single sign-on), Muximux validates the ID token. It checks the token's cryptographic signature (via JWKS), issuer, audience, expiry, and nonce. This prevents token forgery and replay attacks.
- **Nonce binding** -- OIDC authorization requests include a cryptographically random nonce (a single-use value). Muximux verifies it in the callback to prevent replay attacks.
- **Environment variable secrets** -- Sensitive config values (OIDC client secrets, API keys) support `${ENV_VAR}` syntax. This keeps secrets out of config files. Muximux logs a startup warning if the OIDC client secret is stored as plaintext.

### Session Management (ASVS V3)

- **HttpOnly cookies** -- Muximux sets session cookies with `HttpOnly`. JavaScript cannot read them, which mitigates XSS-based session theft.
- **SameSite=Lax** -- Session cookies use `SameSite=Lax`. This blocks the cookie on cross-site POST form submissions, but still allows normal navigation. A cross-site top-level GET request (a link or a GET form) still carries the cookie. So if a proxied app changes state on a GET request, `SameSite=Lax` does not protect that app from cross-site request forgery.
- **Configurable Secure flag** -- When you set `secure_cookies: true` (recommended for HTTPS deployments), the browser sends cookies only over encrypted connections.
- **Configurable session lifetime** -- `session_max_age` controls how long sessions last (default: 24h). Muximux automatically refreshes sessions on activity.
- **Session invalidation** -- Changing a user's password invalidates all other sessions for that user.
- **Cryptographic session tokens** -- Muximux generates session IDs with `crypto/rand`, using 32 bytes of entropy.

### Access Control (ASVS V4)

- **Role-based access control** -- Three roles (`admin`, `power-user`, `user`) with enforced privilege separation. Admin operations require the `admin` role; regular users cannot escalate privileges.
- **Per-app access control** -- You can restrict apps by role or by specific usernames with the `access` and `min_role` fields.
- **Forward auth proxy validation** -- With forward auth, Muximux accepts only connections from `trusted_proxies` CIDR ranges. It rejects auth headers from untrusted sources. The check uses the TCP source address (`RemoteAddr`), not forwarded headers.

### Input Validation (ASVS V5)

- **CSRF protection** -- CSRF (cross-site request forgery) is an attack where another site sends requests in your name. State-changing API requests (POST, PUT, DELETE, PATCH) must carry one of two things: a CSRF-safe Content-Type (`application/json` or `application/x-yaml`), or an `X-Requested-With` header. `multipart/form-data` is not on the list, because a cross-origin page can send it without a preflight. So file uploads, such as icon uploads, must carry `X-Requested-With`. A cross-origin page cannot send the accepted types or the header without a CORS preflight (a check request that the browser sends first), and Muximux never grants that preflight. This defeats the classic browser-form CSRF vector.
- **Environment variable expansion safety** -- Muximux expands only `${VAR}` syntax. It leaves bare `$VAR` references untouched. This prevents corruption of values like bcrypt hashes (`$2a$10$...`).

### Cryptography (ASVS V6)

- **TLS support** -- Use automatic HTTPS via Let's Encrypt (specify `tls.domain` and `tls.email`), or manual certificates (`tls.cert` and `tls.key`). Caddy handles certificate management, renewal, and OCSP stapling.
- **No custom cryptography** -- All cryptographic operations use Go standard library (`crypto/rand`, `crypto/sha256`) or well-established libraries (`golang.org/x/crypto/bcrypt`, `coreos/go-oidc/v3`).

### Logging & Monitoring (ASVS V7)

- **Structured audit logging** -- Muximux logs security-relevant events with `source=audit` and a unique `request_id` for correlation. This makes them easy to filter and trace. Audited events include:
  - User login (success and failure)
  - User logout
  - Password changes
  - User creation, modification, and deletion
  - Auth method changes
  - API key updates
  - Config import/export/restore
  - OIDC authentication events
  - API key authentication failures
- **Centralized error logging** -- Muximux logs all HTTP error responses at an appropriate severity: 5xx at ERROR, 401/403 at WARN, other 4xx at DEBUG.
- **OIDC back-channel logout** -- `/api/auth/oidc/backchannel-logout` accepts a provider's logout token only after it checks these things: its signature, issuer, audience, event claim, absence of a nonce, freshness (at most 10 minutes old) and a one-time `jti`. It then ends the matching sessions.
- **Environment references kept on save** -- When Settings saves the file, it keeps the `${VAR}` references in `config.yaml`. Secrets held in the environment are therefore not written back in plain text. This holds when you rename a list item (an app, user or gateway site), or when you reorder or remove items, in separate saves. If you rename an item and move it in the same save, Settings can write its value in plain text. Check `config.yaml` after such an edit. A value you change in Settings replaces its reference.
- **Real-time log streaming** -- The built-in log viewer supports WebSocket-based live streaming with source-based filtering. You can monitor audit events in real time.
- **File logging** -- Muximux writes logs to both stdout and `data/muximux.log` for persistence.

### Data Protection (ASVS V8)

- **Sensitive data stripping** -- Config exports automatically strip password hashes, API key hashes, and OIDC client secrets before download.
- **No plaintext secrets** -- Muximux stores passwords and API keys as one-way bcrypt hashes. OIDC client secrets support environment variable references.
- **File permissions** -- Muximux writes config files with `0600` permissions (owner read/write only).

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
- **`frame-ancestors 'self'`** -- Modern clickjacking protection (protection against a page that hides your site in a frame to trick clicks). It replaces reliance on the deprecated `X-Frame-Options` alone. Muximux will only load inside its own origin.
- **`form-action 'self'`** -- Blocks a form-hijack that would POST user submissions to an attacker origin.
- **`connect-src 'self'`** -- Same-origin fetches and WebSocket (`ws://`/`wss://` on the same host) both fall under `'self'`. Leaving out the wildcard `ws: wss:` closes a WebSocket exfiltration path for any injected script.
- **`style-src 'unsafe-inline'`** -- Required for Svelte component scoped styles and CSS custom property theming. The strict `script-src 'self'` policy mitigates the risk, because it prevents inline script execution.
- **`img-src https:`** -- Allows external icon URLs (url-type app icons, favicons). Muximux's backend API serves icons from Dashboard Icons and Lucide, so `'self'` covers them.
- **HSTS on TLS only** -- Muximux emits `Strict-Transport-Security: max-age=31536000` on requests that actually arrived over TLS. Plain-HTTP deployments in trusted networks are unaffected. TLS deployments get automatic HTTPS pinning without extra configuration.

### Proxied Response Isolation

When Muximux proxies requests to backend applications, it strips security-sensitive headers from the proxied responses. This prevents backend apps from interfering with Muximux's own security headers. Muximux strips any `Content-Security-Policy`, `X-Frame-Options`, and similar headers that would conflict with the dashboard's iframe embedding.

---

## Architecture Security Notes

### Single Binary, No External Dependencies

Muximux runs as a single Go binary with an embedded web frontend. There are no external database servers, message queues, or caches to secure. All state lives in one YAML file with strict file permissions.

### Reverse Proxy Security

When you enable `proxy: true` for an app, Muximux acts as a reverse proxy (a server that forwards requests to another server). This means:

- All requests to that app go through Muximux's authentication middleware
- Backend apps don't need their own authentication (though defense-in-depth is always recommended)
- TLS certificate verification for backends is configurable per-app (`proxy_skip_tls_verify`)

When `proxy: false`, the browser connects directly to the app. Muximux has no control over those requests.

### Forward Auth Header Validation

Forward auth mode is trust-based: Muximux trusts headers from the auth proxy. The `trusted_proxies` check makes sure that Muximux accepts only connections from known proxy IPs. If an attacker connects directly (bypassing the proxy), Muximux rejects the auth headers.

---

## What ASVS Level 2 Requires That Doesn't Apply

These ASVS categories do not apply to Muximux's architecture:

| Category | Why N/A |
|----------|---------|
| SOAP/XML web services | Muximux uses JSON REST APIs only |
| GraphQL | Not used |
| LDAP injection | No LDAP integration |
| SMS/phone OTP | Not offered as an auth method |
| Hardware security modules | Self-hosted single-binary app |
| File upload malware scanning | No untrusted user file uploads. Custom-icon upload (PNG/SVG/JPG/WebP/GIF, 2 MB cap, admin-only, type-validated) and config import are the only upload paths; both are size-limited and validated. |
| Multi-tenant data isolation | Single-tenant application |
| WebSocket message authentication | WebSocket is used for read-only log streaming, not commands |

---

## Reporting a Vulnerability

Please do not open a public issue for a security problem. Report it privately through the repository's Security tab (**Report a vulnerability**). This opens an advisory that only you and the maintainer can see. [SECURITY.md](https://github.com/mescon/Muximux/blob/main/SECURITY.md) lists what to include, what is in scope, and what to expect. A release fixes each confirmed issue. The issue is then published as a GitHub Security Advisory with a CVE, so that Dependabot and `govulncheck` flag affected versions. The reporter is credited unless they prefer not to be. Past advisories are listed under the repository's Security tab.

## Recommendations for Deployers

1. **Enable HTTPS** -- Use `tls.domain` for automatic Let's Encrypt certificates, or place Muximux behind a TLS-terminating reverse proxy.
2. **Set `secure_cookies: true`** when serving over HTTPS.
3. **Use environment variables for secrets** -- Store OIDC client secrets and API keys as `${ENV_VAR}` references rather than plaintext in config.
4. **Enable authentication** -- The default is `auth: none`. Switch to `builtin`, `forward_auth`, or `oidc` for any internet-facing deployment.
5. **Use the reverse proxy for sensitive apps** -- Set `proxy: true` on apps that should be gated behind Muximux's authentication.
6. **Restrict network access** -- In a homelab, consider running Muximux on an internal network or behind a VPN for defense-in-depth.
7. **Monitor audit logs** -- Filter for `source=audit` in the log viewer to track authentication and configuration events.
8. **Treat the Docker socket as privileged** -- A `:ro` mount does not
   limit Docker API calls. Socket access is root on the host either way.
   Put a socket proxy in front that filters by endpoint, not only by
   method. (A `GET` can attach to a container. A broad container `POST`
   can create one.) Allow the start/stop/restart endpoints only if you
   enable lifecycle controls (`discovery.docker.lifecycle_enabled`).
   These controls let authorized users start/stop/restart containers
   (role-gated, audit-logged). Muximux's role checks do not cover requests
   sent to the proxy directly. Keep the proxy reachable only by Muximux.
9. **Auto-import is opt-in** -- It is off by default. When you enable
   `discovery.docker.auto_import`, Muximux imports every container with
   `muximux.*` labels without a click. This includes any container exposed
   via public gateway subdomains. Review labels before you turn it on.
   Use `muximux.app.enabled=false` to opt a container out. Set
   `MUXIMUX_DISCOVERY_REQUIRE_EXPLICIT_ENABLE=true` to require
   `muximux.app.enabled=true` per container.
10. **Verify what you download** -- Release binaries and container images
    carry signed SLSA provenance (a signed record of how the file was
    built). Check it before deploying. Pin the signing workflow, not only
    the repository: see [Verifying a Download](installation.md#verifying-a-download).

---

## Architectural Trade-offs

These design choices have security implications that you should weigh against your threat model.

### Proxied apps share Muximux's origin

When an app has `proxy: true`, the reverse proxy serves it at `/proxy/{slug}/` on Muximux's own origin. Three features depend on this:

- same-origin iframe embedding without X-Frame-Options friction;
- transparent cookie scoping;
- the runtime request interceptor, which rewrites fetch/XHR/WebSocket URLs for SPAs with hard-coded paths.

The trade-off: a compromised or intentionally malicious proxied app is not meaningfully isolated from Muximux itself:

- The iframe sandbox (`allow-scripts allow-same-origin`) specifically disables the "null origin" protection for same-origin content. An attacker who can run script inside the iframe can reach into Muximux's origin.
- Muximux deliberately strips `Content-Security-Policy` and `X-Frame-Options` from proxied responses. The interceptor script needs to run, and upstream CSP would block it. As a result, when you access the app through Muximux, the upstream's own CSP no longer constrains any reflected-XSS primitive the upstream had.
- Same-origin also means the browser attaches the session cookie to `/proxy/...` requests. The proxy's Director therefore has to strip the cookie, and it does. Muximux's session no longer leaks to backends. But the browser still treats the proxied iframe itself as same-origin.

In practice, **trust a proxied app the way you trust its upstream, plus Muximux**. You may put a random untrusted web service behind `proxy: true`. Then any compromise of that service is a compromise of your Muximux admin surface. For apps you manage (Sonarr, Jellyfin, Home Assistant, etc.), this is fine. For third-party or multi-tenant apps, either leave `proxy: false` (iframes them as cross-origin) or don't embed them at all.

### Permissions-Policy is permissive

The HTTP `Permissions-Policy` header is the ceiling (upper limit) for iframe `allow` delegations. An iframe can only grant features that the parent page itself may use. For this reason, Muximux ships with every delegatable feature set to `*`. Per-app iframe `allow` attributes can then scope camera/microphone/WebAuthn/etc. down to specific app origins. Muximux's own frontend never calls these APIs. The wide ceiling therefore does not broaden Muximux's own attack surface. The narrowing happens per iframe.

To tighten this, use the `permissions` array on each app. It controls per-app delegation.

For proxied apps (`proxy: true`), the array is not a boundary. The iframe is same-origin with Muximux. It therefore inherits the features whose browser default is `'self'` (camera, microphone, geolocation, passkeys and most others), even when `permissions` leaves them out. The browser's own permission prompt still applies. Script in the iframe can also call these APIs through the parent page. A non-proxied, cross-origin app gets only what `permissions` delegates, plus fullscreen (every app frame allows it) and the few features browsers allow everywhere, such as picture-in-picture.
