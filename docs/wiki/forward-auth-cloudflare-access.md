# Cloudflare Access (forward auth)

Cloudflare Access (part of Cloudflare Zero Trust) sits in front of self-hosted apps published through Cloudflare Tunnel. It signs users in with the identity source you configure (Google, GitHub, Entra ID, SAML, etc.). Then it passes the result to your app in HTTP headers and a signed JWT (a token that proves who the user is).

Muximux works with Cloudflare Access through **forward auth** (a proxy in front of Muximux signs the user in and passes the identity in headers). You don't need OIDC config. You tell Muximux to trust Cloudflare's headers. Cloudflare does the actual sign-in.

---

## Prerequisites

- A Cloudflare account with Zero Trust enabled.
- A domain on Cloudflare (any plan, including the free plan).
- Muximux running somewhere that a Cloudflare Tunnel can reach. It doesn't need to be exposed to the internet, because the tunnel handles incoming traffic.
- `cloudflared` installed and authenticated against your Cloudflare account.

---

## Step 1: Publish Muximux Through a Tunnel

If you already publish Muximux through a Cloudflare Tunnel, skip this step.

1. In the Cloudflare Zero Trust dashboard, open **Networks > Tunnels**.
2. Create a tunnel.
3. Install the `cloudflared` connector on the host that can reach Muximux. Cloudflare gives you a one-line install command for the tunnel.
4. Add a public hostname:
   - Subdomain + Domain: `muximux.example.com`
   - Service: `http://muximux:8080` (or the address that `cloudflared` uses to reach Muximux on its network)
5. Save. The hostname now resolves to Muximux.

---

## Step 2: Create an Access Application

1. Open **Zero Trust > Access > Applications**.
2. Click **Add an application > Self-hosted**.
3. Fill in these fields:
   - Application name: `Muximux`
   - Session duration: pick a value that suits your use case (24h is typical).
   - Application domain: `muximux.example.com`
4. Continue to **Identity providers**.
5. Pick at least one identity provider (Google, Entra ID, Authentik, etc.).
6. Continue to **Policies**.
7. Create policies that decide who can sign in. The simplest policy is:
   - Action: **Allow**
   - Selector: **Emails ending in** `example.com` (or a rule that fits your case).
8. Save.

Cloudflare Access now catches every request to `muximux.example.com`. It sends users who are not signed in to a sign-in page. It forwards requests from signed-in users to Muximux.

---

## Step 3: Tell Muximux to Trust Cloudflare's Headers

Cloudflare Access injects two headers on every authenticated request:

- `Cf-Access-Authenticated-User-Email`: the email of the signed-in user.
- `Cf-Access-Jwt-Assertion`: a signed JWT (JSON Web Token) that proves Cloudflare signed the user in. Muximux can check this token on every request. We recommend it: see `cloudflare_access` below.

Edit `config.yaml`:

```yaml
auth:
  method: forward_auth
  trusted_proxies:
    - 127.0.0.1/32         # the address cloudflared connects from, see below
  headers:
    user: Cf-Access-Authenticated-User-Email
    email: Cf-Access-Authenticated-User-Email
    name: Cf-Access-Authenticated-User-Email
  logout_url: https://example.cloudflareaccess.com/cdn-cgi/access/logout
  cloudflare_access:                     # recommended: check the signed token
    team_domain: example.cloudflareaccess.com
    audience: "<AUD tag of the Muximux application>"
```

`cloudflare_access` makes Muximux check the `Cf-Access-Jwt-Assertion` token on every request. Muximux checks its signature against your team's public keys, and checks the issuer, the audience and the expiry. Then it takes the user's email, name and groups from the token, not from any header. A request without a valid token is refused.

- `team_domain` is your Access team domain. It is shown at the top of the Zero Trust dashboard.
- `audience` is the **Application Audience (AUD) Tag**. Open **Access > Applications**, select the Muximux application, and copy the tag from its **Overview** tab.

`trusted_proxies` is the most important field. Muximux only trusts identity headers from these IPs. Muximux checks the address of the direct connection. With a Cloudflare Tunnel, that is always the `cloudflared` connector, not a Cloudflare address. So list the address that the connector uses to reach Muximux:

- If `cloudflared` runs on the same host as Muximux, outside Docker or with host networking, use `127.0.0.1/32`.
  This applies only when Muximux also runs outside Docker or with host networking. If Muximux runs in a container and `cloudflared` connects to a port that Docker publishes, Docker passes the connection on from the gateway address of the bridge network (for example `172.17.0.1`). Then Muximux does not see `127.0.0.1`, and it rejects the identity headers. In that case, trust the gateway address, or put `cloudflared` on the same Docker network as Muximux (see the next item).
- If `cloudflared` runs in its own container on a Docker bridge network, use the address of the connector container on that network, for example `172.18.0.5/32`. Give the container a fixed address (`ipv4_address` in Docker Compose), because Docker can give it a new address when it restarts.
- If `cloudflared` runs on a different machine, use the address of that machine.

Cloudflare's published IP ranges do not apply here, because tunnel requests never come to Muximux directly from them.

> **Security.** Without `cloudflare_access`, Muximux does not check the `Cf-Access-Jwt-Assertion` token. It trusts the email header from every address in `trusted_proxies`. So a process that can connect to Muximux from one of these addresses can sign in as any user. With `cloudflare_access`, that process also needs a valid token that Cloudflare signed. In both cases, `trusted_proxies` still applies. Make sure that only the connector and processes that you trust can connect from these addresses. For example, do not trust a full Docker network that other containers share. Do not trust `127.0.0.1` on a host where untrusted processes run.

> **Protect every route.** Access sets the email header itself on a route that it protects, so a client cannot fake it there. On a tunnel route (public hostname) without Access, the client can send its own `Cf-Access-Authenticated-User-Email` and sign in as anyone. So protect every route that reaches Muximux with Access, or remove the route. With `cloudflare_access`, such a request has no valid token, so Muximux refuses it. Protect the route anyway.

> **Remove the groups header (without `cloudflare_access`).** With `cloudflare_access`, Muximux ignores the groups header and takes groups only from the token. Without it: Cloudflare passes request headers from the browser on to Muximux. Access does not set a groups header. Muximux reads groups from `Remote-Groups` unless `headers.groups` names another header. So a signed-in user can send `Remote-Groups: admin` and become an admin. Add a Cloudflare rule that removes that header from every request to Muximux: **Rules > Transform Rules > Modify Request Header**, action **Remove**. If you send groups from an Access policy (see [On Groups and Admin Promotion](#on-groups-and-admin-promotion)), check that a copy of that header sent by the client cannot reach Muximux either.

The `logout_url` ends in `/cdn-cgi/access/logout`. It uses your Access team domain, which you can see at the top of the Zero Trust dashboard. Opening this URL logs the user out of Cloudflare Access. Without it, Muximux's logout button only ends the local session. Then the next request silently signs the user back in.

---

## Step 4: Validate

1. Open `https://muximux.example.com` in an incognito window. Cloudflare Access should ask you to sign in.
2. Sign in. You should land on the Muximux dashboard. Your email shows in the top-right user menu.
3. Visit Muximux from a machine that bypasses Cloudflare (e.g. directly on the LAN, or through a VPN that reaches the origin server). Muximux should refuse the request, because the source IP isn't in `trusted_proxies`.

---

## On Groups and Admin Promotion

By default, Cloudflare Access does not send a "groups" header. Muximux's forward auth promotes anyone with `admin`/`admins`/`administrators` in `Remote-Groups` to admin. Without extra setup, this promotion won't work.

In forward auth mode, Muximux takes the role from the groups header on every request. A role that you set on a local user has no effect, and the UI does not show user management in this mode. So without a groups header, every user signs in as a regular user, and nobody is an admin.

To get admins:

- **With `cloudflare_access`** (recommended), Muximux reads groups from the token: from a `groups` claim, or from `groups` under `custom`. Access puts identity-provider attributes there when you pass them through as OIDC claims in the identity provider's settings in Zero Trust. Decode a token from your own sign-in to check which claims it carries. Headers do not count in this mode, so the policy-header method below does not work with it.
- **Without `cloudflare_access`, make a groups header from Access policy.** Cloudflare Access can add custom headers based on which policy matched. Open **Access > Applications > Muximux > Policies > Edit policy**. Expand **Custom Headers** and set, for example, `Cf-Access-Groups: admin`. Then add `headers.groups: Cf-Access-Groups` to Muximux's config. You need a separate policy for each group. This gets hard to manage past two or three groups.

If group-based access is central to your setup, the OIDC providers (Keycloak, Authentik, etc.) are a better fit. Cloudflare Access suits you when you want a single sign-on layer at the edge with simple allow/deny rules.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Cloudflare Access prompts for sign-in repeatedly | The browser is blocking the `CF_Authorization` cookie, or the application's session timed out. | Check browser cookie settings for `*.cloudflareaccess.com`. Increase the session duration in the Access application settings. |
| Muximux returns 401 even after Cloudflare lets you through | `trusted_proxies` doesn't include the source IP of the inbound request. | Add the address that the `cloudflared` connector connects from. On the same host without Docker, this is `127.0.0.1/32`. In a separate container, it is the container's address on the Docker network. The Muximux log line "Forward auth request not from trusted proxy" shows the address. |
| `Cf-Access-Authenticated-User-Email` is empty | Request reached Muximux without going through Cloudflare. | Block direct access to Muximux's port from anywhere except Cloudflare or the tunnel connector. |
| Logout doesn't actually sign the user out | `logout_url` is missing or wrong. | Set it to `https://<your-team>.cloudflareaccess.com/cdn-cgi/access/logout`. |

---

## See Also

- [Authentication overview](authentication) for the full forward-auth reference.
- Other identity providers: [Microsoft Entra ID](oidc-microsoft-entra-id), [Keycloak](oidc-keycloak), [Authentik](oidc-authentik), [Pocket ID](oidc-pocket-id), [Zitadel](oidc-zitadel), [Google](oidc-google), [Authelia](forward-auth-authelia).
