# Authentication

## Overview

Muximux supports four authentication methods, configured via `auth.method` in `config.yaml`:

| Method | Description |
|--------|-------------|
| `none` | No authentication (default) |
| `builtin` | Username/password login managed by Muximux |
| `forward_auth` | Delegates to an external auth proxy (Authelia, Authentik, etc.) |
| `oidc` | OpenID Connect with an identity provider (Keycloak, Auth0, Okta, etc.) |

The default is `none`.

---

## No Authentication

```yaml
auth:
  method: none
```

Anyone who can reach Muximux can use it. This is suitable for trusted networks or when authentication is already handled externally (e.g., VPN-only access).

---

## Built-in Authentication

Username/password login with bcrypt-hashed passwords and cookie-based sessions.

```yaml
auth:
  method: builtin
  session_max_age: 24h        # How long sessions last (default: 24h)
  secure_cookies: true         # Set true if serving over HTTPS
  api_key: "your-secret-key"   # Optional: for API access without login
  users:
    - username: admin
      password_hash: "$2a$10$..."  # bcrypt hash
      role: admin
      email: admin@example.com     # Optional
      display_name: "Admin User"   # Optional
    - username: viewer
      password_hash: "$2a$10$..."
      role: user
```

### Generating Password Hashes

The `hashpw` utility is a separate binary built from `cmd/hashpw/`:

```bash
# Build and run
go build -o hashpw ./cmd/hashpw
./hashpw
```

This prompts for a password and outputs a bcrypt hash to paste into `config.yaml`.

You can also pass the password as an argument (useful for scripting):

```bash
./hashpw 'my-secret-password'
```

**Docker users:** The hashpw utility is not included in the Docker image. Generate hashes on your host machine, or use any bcrypt tool:

```bash
# Using htpasswd (from apache2-utils)
htpasswd -nbBC 10 "" 'my-secret-password' | cut -d: -f2

# Using Python
python3 -c "import bcrypt; print(bcrypt.hashpw(b'my-secret-password', bcrypt.gensalt()).decode())"
```

### Roles

- **`admin`** -- Full access. Can modify settings, manage apps, groups, themes, and icons.
- **`user`** -- Read-only dashboard access. Can view and interact with apps but cannot change configuration.

### Sessions

- Cookie-based (cookie name: `muximux_session`)
- Automatically refreshed on activity
- Can be invalidated by changing the user's password (all other sessions for that user are invalidated)
- `session_max_age` accepts duration strings like `1h`, `24h`, `7d`

---

## Forward Auth (Authelia / Authentik / Traefik Forward Auth)

For use behind a reverse proxy that handles authentication. Muximux reads user identity from HTTP headers set by the auth proxy.

```yaml
auth:
  method: forward_auth
  trusted_proxies:
    - 10.0.0.0/8
    - 172.16.0.0/12
    - 192.168.0.0/16
  headers:
    user: Remote-User        # Header containing username (default: Remote-User)
    email: Remote-Email      # Header containing email (default: Remote-Email)
    groups: Remote-Groups    # Header containing groups (default: Remote-Groups)
    name: Remote-Name        # Header containing display name (default: Remote-Name)
```

> **IMPORTANT:** `trusted_proxies` is required. Muximux will reject all forward auth requests if no trusted proxies are configured. This prevents users from spoofing auth headers by connecting directly.

Only the direct TCP connection IP (from `RemoteAddr`) is checked against trusted proxies -- forwarded headers like `X-Forwarded-For` are **not** trusted for this check.

### Admin Detection

Users whose groups contain "admin", "admins", or "administrators" (exact match) are automatically assigned the admin role.

### Typical Setup with Authelia

Your reverse proxy (Nginx, Traefik, Caddy) authenticates users via Authelia, then forwards the request to Muximux with identity headers. Muximux reads these headers and creates a session.

---

## OIDC (OpenID Connect)

Direct integration with identity providers like Authentik, Keycloak, Auth0, Okta, and others.

```yaml
auth:
  method: oidc
  oidc:
    enabled: true
    issuer_url: https://auth.example.com     # OIDC discovery endpoint base
    client_id: muximux
    client_secret: ${OIDC_CLIENT_SECRET}      # Supports env var expansion
    redirect_url: https://muximux.example.com/api/auth/oidc/callback
    scopes:                                   # Default: [openid, profile, email]
      - openid
      - profile
      - email
    username_claim: preferred_username         # Default: preferred_username
    email_claim: email                        # Default: email
    groups_claim: groups                       # Default: groups
    display_name_claim: name                  # Default: name
    admin_groups:                             # Groups that grant admin role
      - admins
      - muximux-admins
```

### How It Works

1. User clicks "Login with SSO" on the login page.
2. The browser redirects to the OIDC provider's login page.
3. After authentication, the provider redirects back to Muximux's callback URL.
4. Muximux exchanges the authorization code for tokens.
5. User info is fetched from the provider's userinfo endpoint.
6. A local session is created.

### Setting Up Your OIDC Provider

- Create a new application/client in your provider.
- Set the redirect/callback URL to: `https://your-muximux-domain/api/auth/oidc/callback`
- Note the client ID and client secret.
- The issuer URL is usually the base URL of your provider (e.g., `https://auth.example.com/application/o/muximux/` for Authentik).

### Environment Variables

Use `${VAR_NAME}` syntax in `config.yaml` to reference environment variables. This is useful for secrets:

```yaml
client_secret: ${OIDC_CLIENT_SECRET}
```

---

## API Key Authentication

When `api_key` is set in the auth config, you can authenticate API requests using the `X-Api-Key` header instead of a session cookie. This is useful for integrations, scripts, and automated tools.

```bash
curl -H "X-Api-Key: your-secret-key" https://muximux.example.com/api/apps
```

The API key is checked using constant-time comparison to prevent timing attacks.

---

## Changing Passwords

Users can change their own password via the Settings panel or the `POST /api/auth/password` endpoint. Changing a password invalidates all other sessions for that user (except the current one).

**Password requirements:** minimum 8 characters.
