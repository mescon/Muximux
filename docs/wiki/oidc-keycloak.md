# OIDC: Keycloak

This guide sets up Muximux to authenticate against a Keycloak realm, with group memberships emitted in the ID token so Muximux can promote users to admin and (in a future release) gate per-app visibility.

The end state: users sign in with their Keycloak account, Muximux receives their group list as `groups: ["developers", "Muximux-Admins"]`, and admins land on the Settings page automatically.

---

## Prerequisites

- A running Keycloak instance (any 19.x or newer release works). You need realm-administrator access on the realm Muximux will use.
- Muximux on a stable URL (HTTPS in production, HTTP works for localhost testing).
- The base URL of your Keycloak instance, including the realm path. The full **issuer URL** for a realm called `homelab` running at `https://auth.example.com` is `https://auth.example.com/realms/homelab`. Note the format; it changed in Keycloak 17 (older versions used `/auth/realms/...`).

---

## Step 1: Create the Client

1. Sign in to the Keycloak admin console and select the realm.
2. Open **Clients** and click **Create client**.
3. **General settings**:
   - Client type: `OpenID Connect`
   - Client ID: `muximux`
4. **Capability config**:
   - Client authentication: **On** (gives you a client secret).
   - Authentication flow: leave **Standard flow** checked, uncheck the rest.
5. **Login settings**:
   - Root URL: `https://muximux.example.com`
   - Valid redirect URIs: `https://muximux.example.com/api/auth/oidc/callback`
   - Web origins: `https://muximux.example.com`
6. Save.

Switch to the **Credentials** tab and copy the **Client secret**. You'll need it in `config.yaml`.

---

## Step 2: Add a Group Membership Mapper

Out of the box Keycloak does not include groups in the ID token. Muximux needs them for `admin_groups` and per-app `allowed_groups` to work.

1. Open the `muximux` client and go to **Client scopes**.
2. Click on **muximux-dedicated** in the list (the client's dedicated scope).
3. Click **Add mapper > By configuration > Group Membership**.
4. Fill in:
   - Name: `groups`
   - Token Claim Name: `groups`
   - **Full group path**: **Off** (this is the important one; with it on, Keycloak emits `/Engineering` instead of `Engineering` and `admin_groups` won't match what you'd write naturally)
   - Add to ID token: **On**
   - Add to access token: **On**
   - Add to userinfo: **On**
5. Save.

---

## Step 3: Create or Identify Admin Groups

If you want some users to log in as Muximux admins, the group has to exist in Keycloak.

1. Open **Groups > Create group**.
2. Name: `Muximux-Admins` (or whatever you'd like to recognize).
3. Add members on the **Members** tab, or assign the group to users via **Users > <user> > Groups**.

You can repeat this for any other groups you'd like to use later for per-app filtering, e.g. `Engineering`, `On-Call`.

---

## Step 4: Configure Muximux

Edit `config.yaml`:

```yaml
auth:
  method: oidc
  oidc:
    enabled: true
    issuer_url: https://auth.example.com/realms/homelab
    client_id: muximux
    client_secret: ${KEYCLOAK_CLIENT_SECRET}
    redirect_url: https://muximux.example.com/api/auth/oidc/callback
    scopes:
      - openid
      - profile
      - email
    # Defaults match Keycloak conventions; spelled out here for clarity.
    username_claim: preferred_username
    email_claim: email
    groups_claim: groups
    display_name_claim: name
    admin_groups:
      - Muximux-Admins
```

Set `KEYCLOAK_CLIENT_SECRET` in the environment. Restart Muximux.

---

## Step 5: Validate

1. Visit `https://muximux.example.com/login` and click **Login with SSO**. You should be redirected to Keycloak.
2. After Keycloak sign-in you should land back on the Muximux dashboard.
3. Sign in as a member of `Muximux-Admins` and confirm the Settings gear appears in the navigation.
4. To inspect the actual claims Keycloak is sending: in Keycloak admin, open **Clients > muximux > Client scopes > Evaluate**, pick a user, and click **Generated ID Token**. The `groups` claim should be there with the names you expect (no leading slash).

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `Invalid parameter: redirect_uri` | The URI in Muximux's `redirect_url` doesn't match the **Valid redirect URIs** list on the client. | Set the client's redirect URI to exactly `https://muximux.example.com/api/auth/oidc/callback`. Wildcards aren't great here; use the exact value. |
| Sign-in works but admin promotion doesn't | The `groups` mapper either isn't attached or has **Full group path** on. | Re-check the mapper in Step 2. Names must come through as plain strings (`Engineering`), not paths (`/Engineering`). |
| `404 Not Found` from Keycloak when Muximux fetches discovery | Wrong issuer URL format. Keycloak 17+ dropped the `/auth/` prefix. | Use `https://<host>/realms/<realm>`, not `https://<host>/auth/realms/<realm>`. |
| `Invalid token issuer` after upgrading Keycloak | The issuer in tokens changed because the public-facing URL of Keycloak changed. | Set `KC_HOSTNAME_URL` (or the older `KEYCLOAK_FRONTEND_URL`) on the Keycloak side to its public-facing URL, and make sure `issuer_url:` in Muximux matches. |
| Browser console shows CORS errors during sign-in | Web origin isn't whitelisted. | Add `https://muximux.example.com` to the client's **Web origins**. |

---

## See Also

- [Authentication overview](authentication) for the rest of `auth.oidc`.
- Other identity providers: [Microsoft Entra ID](oidc-microsoft-entra-id), [Authentik](oidc-authentik), [Pocket ID](oidc-pocket-id), [Zitadel](oidc-zitadel), [Google](oidc-google), [Authelia](forward-auth-authelia), [Cloudflare Access](forward-auth-cloudflare-access).
