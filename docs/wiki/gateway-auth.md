# Gateway Auth Gate

Muximux can optionally act as a security gate in front of your gateway sites. You turn the gate on per site. Then requests to that subdomain go through Muximux's session check before they reach the backend (the app behind the site). Muximux sends anonymous visitors to its login page. It forwards signed-in visitors based on their role and group membership.

The gate sits **in front of** the backend. If the backend has its own authentication, the gate does not replace it. The two work as separate layers. Log in to Muximux once, and you can reach every gated subdomain you have permission for.

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
           reverse_proxy http://192.168.1.5:8989  (the backend)
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

The gate uses Muximux's session cookie as the auth source. The cookie must be visible at every gated subdomain. For this, you need:

1. **Muximux's dashboard and every gated site to share a parent domain.** Example: `muximux.example.com`, `sonarr.example.com`, `grafana.example.com` all share `.example.com`.
2. **A consistent transport.** Use either all HTTPS (recommended), or all plain HTTP for local-only setups. Browsers reject mixed-Secure cookies across the same domain.
3. **`server.session_cookie_domain` configured**, so that Muximux issues the cookie with the right scope.

Muximux checks this when it loads the config. A site can have `require_auth: true` but not be a subdomain of `session_cookie_domain`. Then Muximux exits at startup, with an error that names that site.

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

The leading dot is the browser-canonical form, but it is optional. Browsers treat both forms the same.

### 2. Enable Require Auth per site

In Settings → Gateway, edit a site. Tick **Require Muximux login**. The result in YAML is:

```yaml
server:
  gateway_sites:
    - domain: sonarr.example.com
      backend_url: http://192.168.1.5:8989
      tls: auto
      require_auth: true          # gate this site
      min_role: user              # optional; "user" / "power-user" / "admin"
      allowed_groups:             # optional; case-insensitive; admins bypass
        - family
        - admins
```

### 3. Reload

Restart Muximux. If you changed the gateway site in the Gateway tab, saving there does this for you. Caddy's reload picks up the new `forward_auth` directive on every gated site.

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

Backends receive `X-Muximux-User` and `X-Muximux-Role` headers on every forwarded request. Some apps accept these headers (rare, but becoming more common). They can use them as an external-auth source.

---

## Sign-out

`/logout` on Muximux makes the session cookie invalid. The cookie is scoped to `.example.com`, so this clears it for **every gated subdomain at once**. The next request to any gated site sends the user to the login page again.

There is no per-site logout, on purpose. The gate is a central SSO layer. "Log out of one app but not the others" does not fit that model. If your backend has its own logout, that is separate. The gate does not pass the logout on to it.

---

## Hardening at the gate endpoint

The gate endpoint has some extra layers of protection (defence in depth):

- **Only reachable from inside Muximux.** `/api/auth/forward` receives Caddy's forward_auth call. So in normal operation, requests reach it only over the loopback interface. The endpoint refuses callers that are not on loopback. So a public visitor can't call it directly to probe session validity or gateway-site configuration.
- **Login-redirect target is locked to your configured sites.** The gate can send an anonymous visitor to `/login`. Muximux then builds the URL to return to after login from the configured `gateway_sites` domain. It does not use a header value that the caller controls. This design blocks off-site redirect attacks through header injection.
- **Audit log on every decision.** Allows, denials, and permission-denied responses each leave an audit-log line with the host and the session's user/role.

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
      backend_url: http://192.168.1.5:8989
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

Use this for both observability and post-incident review. Muximux records failed access attempts as audit events, the same as allowed ones.
