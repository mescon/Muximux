# Authelia (forward auth or OIDC)

Authelia can sit in front of Muximux as either a **forward-auth provider** or an **OIDC provider**. This guide covers both. Forward auth is the original Authelia integration model and is the most common. OIDC is the more recent and more portable approach.

If you're not sure which to pick:

- Pick **forward auth** if Authelia already protects other apps in your stack through your reverse proxy (a server that forwards requests to another server, such as Traefik, Nginx, Caddy). It's the lighter setup.
- Pick **OIDC** if you want Muximux to sign users in with Authelia the same way it would with Keycloak or Entra ID. This works independently of your reverse proxy.

---

## Option A: Forward Auth (Traefik / Nginx / Caddy)

In this mode, your reverse proxy calls Authelia on every request. Authelia decides whether to allow the request and adds headers like `Remote-User` and `Remote-Groups`. Muximux trusts those headers.

### Step 1: Configure Authelia

If Authelia already protects other apps, you only need to add a rule for Muximux's host. Add it in `configuration.yml`:

```yaml
access_control:
  default_policy: deny
  rules:
    - domain: muximux.example.com
      policy: two_factor    # or one_factor; pick what suits your risk model
```

The headers themselves need no special Authelia config. Authelia sends `Remote-User`, `Remote-Email`, `Remote-Name`, and `Remote-Groups` by default.

### Step 2: Configure the Reverse Proxy

The exact syntax depends on your proxy. The pattern is the same for all proxies. Forward Muximux's traffic to Authelia's `/api/verify` endpoint. Then copy the auth headers into the request that goes on to Muximux.

**Traefik (file provider):**

```yaml
http:
  routers:
    muximux:
      rule: "Host(`muximux.example.com`)"
      service: muximux
      middlewares: [authelia]
      tls:
        certResolver: default

  services:
    muximux:
      loadBalancer:
        servers:
          - url: "http://muximux:8080"

  middlewares:
    authelia:
      forwardAuth:
        address: "http://authelia:9091/api/verify?rd=https://auth.example.com"
        trustForwardHeader: true
        authResponseHeaders:
          - Remote-User
          - Remote-Groups
          - Remote-Email
          - Remote-Name
```

**Nginx:** see Authelia's [official Nginx integration page](https://www.authelia.com/integration/proxies/nginx/) for the exact snippet. Then point the protected `location` at Muximux.

**Caddy:** Authelia's [Caddy integration page](https://www.authelia.com/integration/proxies/caddy/) covers `forward_auth` directives.

### Step 3: Configure Muximux

```yaml
auth:
  method: forward_auth
  trusted_proxies:
    - 172.16.0.0/12       # whatever subnet your reverse proxy lives in
  headers:
    user: Remote-User
    email: Remote-Email
    groups: Remote-Groups
    name: Remote-Name
  logout_url: https://auth.example.com/logout
```

`trusted_proxies` is required. Muximux only accepts auth headers that arrived from a trusted hop. Setting it to `0.0.0.0/0` removes the protection. Restrict it to your reverse proxy's actual address or subnet.

You can promote users to admin through groups. By default, Authelia puts your group memberships into `Remote-Groups` as a comma-separated list. Muximux looks for any of `admin`, `admins`, or `administrators` (case-insensitive). If your admin group has a different name, set `forward_auth_admin_groups` on the `auth` block to that name:

```yaml
auth:
  method: forward_auth
  forward_auth_admin_groups:
    - dashboard-admins
```

### Step 4: Validate

1. Open `https://muximux.example.com/`. You should see Authelia's login page.
2. Sign in. Complete 2FA if Authelia asks for it. You should land on the Muximux dashboard.
3. Sign in as a user whose `groups` field in Authelia includes `admin` (or `admins` / `administrators`). The Settings gear should appear.

---

## Option B: Authelia as an OIDC Provider

Authelia can also act as an OIDC provider. In that mode you don't need any forward-auth setup. Muximux talks to Authelia directly.

### Step 1: Configure the OIDC Client in Authelia

Add a client to Authelia's `configuration.yml`:

```yaml
identity_providers:
  oidc:
    hmac_secret: <a long random string>
    issuer_private_key: |
      -----BEGIN PRIVATE KEY-----
      ...
      -----END PRIVATE KEY-----
    clients:
      - id: muximux
        description: Muximux dashboard
        secret: '<bcrypt-hashed secret, see Authelia docs>'
        public: false
        authorization_policy: two_factor
        redirect_uris:
          - https://muximux.example.com/api/auth/oidc/callback
        scopes:
          - openid
          - profile
          - email
          - groups
        userinfo_signing_algorithm: none
```

Authelia stores client secrets as hashes. Use `authelia hash-password '<secret>'` to generate one.

### Step 2: Configure Muximux

```yaml
auth:
  method: oidc
  oidc:
    enabled: true
    issuer_url: https://auth.example.com
    client_id: muximux
    client_secret: ${AUTHELIA_CLIENT_SECRET}     # the plaintext, not the hash
    redirect_url: https://muximux.example.com/api/auth/oidc/callback
    scopes:
      - openid
      - profile
      - email
      - groups
    username_claim: preferred_username
    email_claim: email
    groups_claim: groups
    display_name_claim: name
    admin_groups:
      - Muximux-Admins
```

`issuer_url` is the public base URL of your Authelia instance, with no trailing path. Authelia exposes the OIDC discovery document at `https://auth.example.com/.well-known/openid-configuration`.

### Step 3: Validate

Validate the same way as for any other OIDC provider:

1. Visit `/login`.
2. Click **Login with SSO**.
3. Sign in to Authelia. Authelia sends you back to Muximux.
4. Sign in as a member of the group listed in `admin_groups`. Members of that group see the Settings gear.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Forward auth: `Remote-User` empty | The reverse proxy isn't forwarding the header, or the rule that protects Muximux didn't match. | Check the reverse-proxy logs for the response headers from `/api/verify`. They should include `Remote-User`. |
| Forward auth: requests bypass Authelia | A request reached Muximux from outside the trusted-proxy range. | Lock down access at the reverse proxy so direct connections to Muximux's port aren't possible from outside the proxy network. |
| Forward auth: admin gear missing for known admin user | Authelia is sending `Remote-Groups: foo` and Muximux's check is for `admin`/`admins`/`administrators`. | Either rename the admin group in Authelia, or set `forward_auth_admin_groups` in Muximux to your group's name. |
| OIDC: `unauthorized_client` from Authelia | Client secret in `config.yaml` doesn't match Authelia's stored hash. | Regenerate the hash with `authelia hash-password` and verify the plaintext you put in Muximux's environment matches. |
| OIDC: no `groups` claim in token | `groups` not in the client's scopes list in Authelia. | Add `groups` to both Authelia's client config (Step 1) and Muximux's `scopes:`. |

---

## See Also

- [Authentication overview](authentication) for the rest of `auth.oidc` and `auth.method: forward_auth`.
- Other identity providers: [Microsoft Entra ID](oidc-microsoft-entra-id), [Keycloak](oidc-keycloak), [Authentik](oidc-authentik), [Pocket ID](oidc-pocket-id), [Zitadel](oidc-zitadel), [Google](oidc-google), [Cloudflare Access](forward-auth-cloudflare-access).
