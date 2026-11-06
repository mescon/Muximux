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
- `Cf-Access-Jwt-Assertion`: a signed JWT that proves Cloudflare signed the user in. Apps behind Cloudflare can use it to check the source on their own. Muximux's forward auth uses the email header directly.

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

`trusted_proxies` is the most important field. Muximux only trusts identity headers from these IPs. Use Cloudflare's published IP ranges. The list above is current as of writing. Check https://www.cloudflare.com/ips/ for the canonical list. If Cloudflare's tunnel terminates locally on the same host, restrict it to `127.0.0.1/32` instead.

The `logout_url` ends in `/cdn-cgi/access/logout`. It uses your Access team domain, which you can see at the top of the Zero Trust dashboard. Opening this URL logs the user out of Cloudflare Access. Without it, Muximux's logout button only ends the local session. Then the next request silently signs the user back in.

---

## Step 4: Validate

1. Open `https://muximux.example.com` in an incognito window. Cloudflare Access should ask you to sign in.
2. Sign in. You should land on the Muximux dashboard. Your email shows in the top-right user menu.
3. Visit Muximux from a machine that bypasses Cloudflare (e.g. directly on the LAN, or through a VPN that reaches the origin server). Muximux should refuse the request, because the source IP isn't in `trusted_proxies`.

---

## On Groups and Admin Promotion

By default, Cloudflare Access does not send a "groups" header. Muximux's forward auth promotes anyone with `admin`/`admins`/`administrators` in `Remote-Groups` to admin. Without extra setup, this promotion won't work.

Two options:

- **Manage admin status in Muximux directly.** The first sign-in lands as a regular user. Promote the user in **Settings > Security > Users** in the Muximux UI.
- **Make a groups header from Access policy.** Cloudflare Access can add custom headers based on which policy matched. Open **Access > Applications > Muximux > Policies > Edit policy**. Expand **Custom Headers** and set, for example, `Cf-Access-Groups: admin`. Then add `headers.groups: Cf-Access-Groups` to Muximux's config. You need a separate policy for each group. This gets hard to manage past two or three groups.

If group-based access is central to your setup, the OIDC providers (Keycloak, Authentik, etc.) are a better fit. Cloudflare Access suits you when you want a single sign-on layer at the edge with simple allow/deny rules.

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
