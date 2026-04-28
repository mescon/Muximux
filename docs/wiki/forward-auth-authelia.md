# Authelia (forward auth or OIDC)

Authelia can sit in front of Muximux as either a **forward-auth provider** or an **OIDC provider**. This guide covers both. Forward auth is the original Authelia integration model and is the most common; OIDC is the more recent, more portable approach.

If you're not sure which to pick:

- Pick **forward auth** if Authelia is already protecting other apps in your stack via your reverse proxy (Traefik, Nginx, Caddy). It's the lighter setup.
- Pick **OIDC** if you want Muximux to authenticate against Authelia the same way it would authenticate against Keycloak or Entra ID, independent of your reverse proxy.

---

## Option A: Forward Auth (Traefik / Nginx / Caddy)

In this mode, your reverse proxy calls Authelia on every request. Authelia decides whether to allow the request and adds headers like `Remote-User` and `Remote-Groups`. Muximux trusts those headers.

### Step 1: Configure Authelia

If you already have Authelia protecting other apps, you only need to add a rule for Muximux's host. In `configuration.yml`:

```yaml
access_control:
  default_policy: deny
  rules:
    - domain: muximux.example.com
      policy: two_factor    # or one_factor; pick what suits your risk model
```

No special Authelia config is required for the headers themselves; Authelia emits `Remote-User`, `Remote-Email`, `Remote-Name`, and `Remote-Groups` by default.

### Step 2: Configure the Reverse Proxy

The exact syntax depends on your proxy. The pattern is the same: forward Muximux's traffic to Authelia's `/api/verify` endpoint and copy the auth headers into the upstream request.

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

**Nginx:** see Authelia's [official Nginx integration page](https://www.authelia.com/integration/proxies/nginx/) for the verbatim snippet, then point the protected `location` at Muximux.

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

`trusted_proxies` is required: Muximux only honours auth headers that arrived from a trusted hop. Setting it to `0.0.0.0/0` defeats the protection; restrict it to your reverse proxy's actual address or subnet.

For admin promotion via groups, Authelia by default puts your group memberships into `Remote-Groups` as a comma-separated list. Muximux looks for any of `admin`, `admins`, or `administrators` (case-insensitive). If your admin group has a different name, you'll need OIDC's `admin_groups` setting (or wait for the upcoming per-app `allowed_groups` work).

### Step 4: Validate

1. Open `https://muximux.example.com/`. You should be redirected to Authelia's login page.
2. After you sign in (and complete 2FA if required), you should land on the Muximux dashboard.
3. Sign in as a user whose `groups` field in Authelia includes `admin` (or `admins` / `administrators`). The Settings gear should appear.

---

## Option B: Authelia as an OIDC Provider

Authelia can also act as an OIDC provider. In that mode you don't need any forward-auth wiring; Muximux talks to Authelia directly.

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

Authelia stores client secrets as hashes; use `authelia hash-password '<secret>'` to generate one.

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

`issuer_url` is the public base URL of your Authelia instance, no trailing path. Authelia exposes the OIDC discovery document at `https://auth.example.com/.well-known/openid-configuration`.

### Step 3: Validate

Same as the OIDC case in any other provider: visit `/login`, click **Login with SSO**, sign in to Authelia, get redirected back to Muximux. Members of the group listed in `admin_groups` see the Settings gear.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Forward auth: `Remote-User` empty | The reverse proxy isn't forwarding the header, or the rule that protects Muximux didn't match. | Check the reverse-proxy logs for the response headers from `/api/verify`. They should include `Remote-User`. |
| Forward auth: requests bypass Authelia | A request reached Muximux from outside the trusted-proxy range. | Lock down access at the reverse proxy so direct connections to Muximux's port aren't possible from outside the proxy network. |
| Forward auth: admin gear missing for known admin user | Authelia is sending `Remote-Groups: foo` and Muximux's check is for `admin`/`admins`/`administrators`. | Either rename the admin group in Authelia, or switch to the OIDC mode where `admin_groups` is configurable. |
| OIDC: `unauthorized_client` from Authelia | Client secret in `config.yaml` doesn't match Authelia's stored hash. | Regenerate the hash with `authelia hash-password` and verify the plaintext you put in Muximux's environment matches. |
| OIDC: no `groups` claim in token | `groups` not in the client's scopes list in Authelia. | Add `groups` to both Authelia's client config (Step 1) and Muximux's `scopes:`. |

---

## See Also

- [Authentication overview](authentication) for the rest of `auth.oidc` and `auth.method: forward_auth`.
- Other identity providers: [Microsoft Entra ID](oidc-microsoft-entra-id), [Keycloak](oidc-keycloak), [Authentik](oidc-authentik), [Pocket ID](oidc-pocket-id), [Zitadel](oidc-zitadel), [Google](oidc-google), [Cloudflare Access](forward-auth-cloudflare-access).
