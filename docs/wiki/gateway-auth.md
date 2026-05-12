# Gateway Auth Gate

Muximux can optionally act as a security gate in front of your gateway sites. When enabled per-site, requests to that subdomain run through Muximux's session check before reaching the backend. Anonymous visitors are redirected to Muximux's login page; authenticated visitors are forwarded based on their role and group membership.

The gate sits **in front of** the backend - it does not replace the backend's own authentication if it has one. It's a layered defence: log in to Muximux once, gain access to every gated subdomain you have permission for.

```
Browser -> sonarr.example.com (Caddy)
             |
             v
           forward_auth /api/auth/forward (Muximux Go server)
             |
             | session cookie valid + permitted ?
             |       no  -> 302 to /login -> back to original URL after sign-in
             |       yes -> 200 with X-Muximux-User / X-Muximux-Role headers
             v
           reverse_proxy http://10.0.0.5:8989  (the backend)
```

---

## When to enable

✅ You want a single sign-on layer in front of multiple homelab apps
✅ One or more apps lack their own auth (Plex, Grafana, custom Flask app)
✅ You're already using Muximux as your dashboard with built-in / OIDC / forward-auth users
✅ All gated subdomains share a parent domain (`*.example.com`)

❌ You're hosting public-facing sites that need to be reachable anonymously
❌ Each backend already has battle-tested SSO you want to keep authoritative
❌ Your gated subdomains span different parent domains (`app.alpha.com` + `app.beta.org`)

---

## Topology requirement

The gate uses Muximux's session cookie as the auth source. For the cookie to be visible at every gated subdomain, you need:

1. **Muximux's dashboard and every gated site to share a parent domain.** Example: `muximux.example.com`, `sonarr.example.com`, `grafana.example.com` all share `.example.com`.
2. **A consistent transport.** Either all HTTPS (recommended) or all plain HTTP for local-only setups - browsers reject mixed-Secure cookies across the same domain.
3. **`server.session_cookie_domain` configured** so Muximux issues the cookie with the right scope.

The validator at config load enforces this. If a site has `require_auth: true` but isn't a subdomain of `session_cookie_domain`, Muximux exits at startup with a specific error pointing at the offending site.

---

## Setup

### 1. Set the cookie scope

In `config.yaml`:

```yaml
server:
  session_cookie_domain: ".example.com"   # or "example.com", browser normalises
  tls:
    domain: "muximux.example.com"
```

The leading dot is the browser-canonical form but optional - browsers treat both equally.

### 2. Enable Require Auth per site

In Settings → Gateway, edit a site and tick **Require Muximux login**:

```yaml
server:
  gateway_sites:
    - domain: sonarr.example.com
      backend_url: http://10.0.0.5:8989
      tls: auto
      require_auth: true          # gate this site
      min_role: user              # optional; "user" / "power-user" / "admin"
      allowed_groups:             # optional; case-insensitive; admins bypass
        - family
        - admins
```

### 3. Reload

Restart Muximux (or, for an in-flight gateway-site mutation, the Gateway tab's save does this for you). Caddy's reload picks up the new `forward_auth` directive on every gated site.

---

## Permission model

Identical to the App access rules used by the dashboard:

| Setting | Meaning |
|---|---|
| `require_auth: false` | No gate; anonymous traffic reaches the backend. |
| `require_auth: true`, no `min_role`, no `allowed_groups` | Any authenticated Muximux user can reach the backend. |
| `require_auth: true`, `min_role: power-user` | Users at power-user level or higher pass. Admins always pass. |
| `require_auth: true`, `allowed_groups: [family]` | Users in the `family` group pass. Admins bypass the group check. |
| `require_auth: true`, both set | User must satisfy both: meet the role bar AND be in at least one allowed group. Admins still bypass both. |

Group matching is case-insensitive (`Family` and `family` are the same group).

---

## Behaviour by response

When Caddy calls `/api/auth/forward` for a gated site:

| Muximux returns | Caddy serves | User sees |
|---|---|---|
| `200 OK` + `X-Muximux-User` + `X-Muximux-Role` | Reverse-proxies to the backend with those headers attached | The app, normally |
| `302 Found` → `/login?next=<url>` | Redirects | Muximux login page; after sign-in, lands back at the original URL |
| `403 Forbidden` | Returns body verbatim | Small "you're signed in but lack permission" page with a sign-out link |
| `500` / `503` | Returns body | "gateway misconfigured" message (rare; surfaces operator config errors) |

Backends receive `X-Muximux-User` and `X-Muximux-Role` headers on every forwarded request. Apps that honour these (rare but increasing) can use them as an external-auth source.

---

## Sign-out

`/logout` on Muximux invalidates the session cookie. Because the cookie is scoped to `.example.com`, this clears it for **every gated subdomain at once**. Next request to any gated site triggers the login redirect again.

There is intentionally no per-site logout: the gate is a centralised SSO layer, and "log out of one app but not the others" doesn't compose cleanly with that model. If your backend has its own logout, that's separate - the gate doesn't propagate.

---

## How it interacts with the other auth modes

| Muximux auth method | Gate behaviour |
|---|---|
| `auth.method: builtin` | Native users; session check is straightforward. |
| `auth.method: oidc` | Sessions backed by OIDC tokens; gate uses the same session check. Operators get OIDC users surfaced to gated apps via `X-Muximux-User`. |
| `auth.method: forwardauth` (Authelia / Authentik) | Muximux trusts an upstream `Remote-User` header to create a session, then the gate reads that session. Daisy-chained forward-auth works but adds a network hop; consider whether the upstream is already gating the same set of apps. |

---

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| Browser loops between gated site and login | Cookie isn't reaching the gated host. Verify `session_cookie_domain` covers both. Check the browser's cookie inspector. |
| 503 "configure server.tls.domain" on the gate | The gate doesn't know where to send the login redirect. Set `server.tls.domain` to your dashboard host. |
| Startup error "session_cookie_domain is required when any gateway site has require_auth=true" | You enabled require_auth without setting the cookie scope. Either set `session_cookie_domain` or unset the auth flag. |
| Startup error "gateway site X is not under server.session_cookie_domain" | A gated subdomain doesn't share the parent domain. Move the site or widen the cookie scope. |
| 403 on a gated site you should have access to | Your account's role or groups don't satisfy the site's rules. The forbidden page names the reason (`role_insufficient` or `group_mismatch`). |
| Backend doesn't recognise X-Muximux-User | Most backends ignore the header. The gate still enforces; the header is purely informational for backends that opt in. |
| WebSocket connection drops the gate | Caddy's `forward_auth` runs on the initial HTTP `Upgrade` request; once the WebSocket is up, the connection bypasses forward_auth (standard Caddy behaviour). |

---

## Configuration reference

```yaml
server:
  # Required when any site below has require_auth=true.
  # The leading dot is optional; ".example.com" and "example.com" are equivalent.
  session_cookie_domain: ".example.com"

  gateway_sites:
    - domain: sonarr.example.com
      backend_url: http://10.0.0.5:8989
      tls: auto
      require_auth: true             # default false
      min_role: user                 # default ""; "user" / "power-user" / "admin"
      allowed_groups: [family]       # default []; case-insensitive; admins bypass
```

---

## Audit log

Every forward-auth result is recorded with `source=audit`:

```
forward-auth allowed source=audit host=sonarr.example.com user=alice role=user
forward-auth denied  source=audit host=sonarr.example.com user="" result=no_session
forward-auth denied  source=audit host=admin.example.com user=alice role=user result=role_insufficient
forward-auth denied  source=audit host=family.example.com user=bob role=user result=group_mismatch
```

Use this for both observability and post-incident review. Failed access attempts are first-class events, not just operational noise.
