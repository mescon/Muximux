# Cloudflare Access (forward auth)

Cloudflare Access (part of Cloudflare Zero Trust) sits in front of self-hosted apps published through Cloudflare Tunnel. It authenticates users with whatever identity source you configure (Google, GitHub, Entra ID, SAML, etc.), then passes the result to your app via HTTP headers and a signed JWT.

Muximux integrates with Cloudflare Access using **forward auth**. You don't need OIDC config; you tell Muximux to trust Cloudflare's headers, and Cloudflare handles the actual authentication.

---

## Prerequisites

- A Cloudflare account with Zero Trust enabled.
- A domain on Cloudflare (any plan, including the free plan).
- Muximux running somewhere reachable from a Cloudflare Tunnel (it doesn't need to be internet-exposed; the tunnel handles ingress).
- `cloudflared` installed and authenticated against your Cloudflare account.

---

## Step 1: Publish Muximux Through a Tunnel

If you already publish Muximux through a Cloudflare Tunnel, skip this step.

1. In the Cloudflare Zero Trust dashboard, open **Networks > Tunnels** and create a tunnel.
2. Install the `cloudflared` connector on the host that can reach Muximux. Cloudflare gives you a one-line install command keyed to the tunnel.
3. Add a public hostname:
   - Subdomain + Domain: `muximux.example.com`
   - Service: `http://muximux:8080` (or whatever address `cloudflared` uses to reach Muximux on its network)
4. Save. The hostname now resolves to Muximux.

---

## Step 2: Create an Access Application

1. In **Zero Trust > Access > Applications**, click **Add an application > Self-hosted**.
2. Fill in:
   - Application name: `Muximux`
   - Session duration: pick something reasonable for your use case (24h is typical).
   - Application domain: `muximux.example.com`
3. Continue to **Identity providers** and pick at least one (Google, Entra ID, Authentik, etc.).
4. Continue to **Policies**. Create policies that decide who can sign in. The simplest policy:
   - Action: **Allow**
   - Selector: **Emails ending in** `example.com` (or whatever rule fits).
5. Save.

Cloudflare Access now intercepts every request to `muximux.example.com`, sends unauthenticated users to a sign-in page, and forwards authenticated requests to Muximux.

---

## Step 3: Tell Muximux to Trust Cloudflare's Headers

Cloudflare Access injects two headers on every authenticated request:

- `Cf-Access-Authenticated-User-Email`: the email of the signed-in user.
- `Cf-Access-Jwt-Assertion`: a signed JWT proving Cloudflare authenticated the user. Useful for downstream apps that want to verify the source independently. Muximux's forward auth uses the email header directly.

Edit `config.yaml`:

```yaml
auth:
  method: forward_auth
  trusted_proxies:
    - 173.245.48.0/20      # Cloudflare IPv4 ranges, see below for the full list
    - 103.21.244.0/22
    - 103.22.200.0/22
    - 103.31.4.0/22
    - 141.101.64.0/18
    - 108.162.192.0/18
    - 190.93.240.0/20
    - 188.114.96.0/20
    - 197.234.240.0/22
    - 198.41.128.0/17
    - 162.158.0.0/15
    - 104.16.0.0/13
    - 104.24.0.0/14
    - 172.64.0.0/13
    - 131.0.72.0/22
    # IPv6 ranges
    - 2400:cb00::/32
    - 2606:4700::/32
    - 2803:f800::/32
    - 2405:b500::/32
    - 2405:8100::/32
    - 2a06:98c0::/29
    - 2c0f:f248::/32
  headers:
    user: Cf-Access-Authenticated-User-Email
    email: Cf-Access-Authenticated-User-Email
    name: Cf-Access-Authenticated-User-Email
  logout_url: https://example.cloudflareaccess.com/cdn-cgi/access/logout
```

`trusted_proxies` is the most important field: Muximux only trusts identity headers from these IPs. Use Cloudflare's published IP ranges (the list above is current as of writing; check https://www.cloudflare.com/ips/ for the canonical list) or, if Cloudflare's tunnel terminates locally on the same host, restrict to `127.0.0.1/32`.

The `logout_url` ends in `/cdn-cgi/access/logout` and uses your Access team domain (visible at the top of the Zero Trust dashboard). Hitting it logs the user out of Cloudflare Access; without it, Muximux's logout button only kills the local session and the next request silently signs them back in.

---

## Step 4: Validate

1. Open `https://muximux.example.com` in an incognito window. You should be redirected to Cloudflare Access for sign-in.
2. After signing in, you should land on the Muximux dashboard with your email shown in the top-right user menu.
3. Visit Muximux on a machine that bypasses Cloudflare (e.g. directly on the LAN, or a VPN that lets you hit the origin). Muximux should refuse the request because the source IP isn't in `trusted_proxies`.

---

## On Groups and Admin Promotion

Cloudflare Access does not by default emit a "groups" header. Muximux's forward auth admin promotion (anyone with `admin`/`admins`/`administrators` in `Remote-Groups`) won't work out of the box.

Two options:

- **Manage admin status in Muximux directly.** First sign-in lands as a regular user; promote them via **Settings > Security > Users** in the Muximux UI.
- **Synthesize a groups header from Access policy.** Cloudflare Access can inject custom headers based on which policy matched. In **Access > Applications > Muximux > Policies > Edit policy**, expand **Custom Headers** and set, for example, `Cf-Access-Groups: admin`. Then add `headers.groups: Cf-Access-Groups` to Muximux's config. This requires defining a separate policy per group, which gets cumbersome past two or three groups.

The OIDC providers (Keycloak, Authentik, etc.) are a better fit if group-based access is central to your setup. Cloudflare Access shines when you want a single sign-on layer at the edge with simple allow/deny rules.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Cloudflare Access prompts for sign-in repeatedly | The browser is blocking the `CF_Authorization` cookie, or the application's session timed out. | Check browser cookie settings for `*.cloudflareaccess.com`. Increase the session duration in the Access application settings. |
| Muximux returns 401 even after Cloudflare lets you through | `trusted_proxies` doesn't include the source IP of the inbound request. | If `cloudflared` runs on the same host as Muximux, use `127.0.0.1/32`. Otherwise list Cloudflare's IP ranges. |
| `Cf-Access-Authenticated-User-Email` is empty | Request reached Muximux without going through Cloudflare. | Block direct access to Muximux's port from anywhere except Cloudflare or the tunnel connector. |
| Logout doesn't actually sign the user out | `logout_url` is missing or wrong. | Set it to `https://<your-team>.cloudflareaccess.com/cdn-cgi/access/logout`. |

---

## See Also

- [Authentication overview](authentication) for the full forward-auth reference.
- Other identity providers: [Microsoft Entra ID](oidc-microsoft-entra-id), [Keycloak](oidc-keycloak), [Authentik](oidc-authentik), [Pocket ID](oidc-pocket-id), [Zitadel](oidc-zitadel), [Google](oidc-google), [Authelia](forward-auth-authelia).
