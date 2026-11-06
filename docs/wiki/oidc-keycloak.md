# OIDC: Keycloak

This guide sets up Muximux to sign users in with a Keycloak realm (a separate space of users and settings in Keycloak). Keycloak puts group memberships in the ID token (the signed sign-in record that Keycloak sends to Muximux). Muximux uses these groups to promote users to admin and to control which apps each user sees.

When you're done, users sign in with their Keycloak account. Muximux receives their group list as `groups: ["developers", "Muximux-Admins"]`. Admins see the Settings gear, which opens the Settings page.

---

## Prerequisites

- A running Keycloak instance. Any 19.x or newer release works. You need realm-administrator access on the realm Muximux will use.
- Muximux on a stable URL. Use HTTPS in production. HTTP works for localhost testing.
- The base URL of your Keycloak instance, including the realm path. Take a realm called `homelab` running at `https://auth.example.com`. Its full **issuer URL** (the address Muximux uses to find the provider) is `https://auth.example.com/realms/homelab`. Note the format. It changed in Keycloak 17. Older versions used `/auth/realms/...`.

---

## Step 1: Create the Client

1. Sign in to the Keycloak admin console.
2. Select the realm.
3. Open **Clients**.
4. Click **Create client**.
5. **General settings**:
   - Client type: `OpenID Connect`
   - Client ID: `muximux`
6. **Capability config**:
   - Client authentication: **On** (gives you a client secret).
   - Authentication flow: leave **Standard flow** checked. Uncheck the rest.
7. **Login settings**:
   - Root URL: `https://muximux.example.com`
   - Valid redirect URIs: `https://muximux.example.com/api/auth/oidc/callback`
   - Web origins: `https://muximux.example.com`
8. Save.
9. Switch to the **Credentials** tab.
10. Copy the **Client secret**. You will need it in `config.yaml`.

---

## Step 2: Add a Group Membership Mapper

By default, Keycloak does not include groups in the ID token. Muximux needs them, or `admin_groups` and per-app `allowed_groups` do not work.

1. Open the `muximux` client.
2. Go to **Client scopes**.
3. Click on **muximux-dedicated** in the list (the client's dedicated scope).
4. Click **Add mapper > By configuration > Group Membership**.
5. Fill in these fields:
   - Name: `groups`
   - Token Claim Name: `groups`
   - **Full group path**: **Off**. This is the important one. When it is on, Keycloak sends `/Engineering` instead of `Engineering`. Then `admin_groups` won't match the names you would naturally write.
   - Add to ID token: **On**
   - Add to access token: **On**
   - Add to userinfo: **On**
6. Save.

---

## Step 3: Create or Identify Admin Groups

If you want some users to log in as Muximux admins, the admin group must exist in Keycloak.

1. Open **Groups > Create group**.
2. Name: `Muximux-Admins` (or another name you will recognize).
3. Add members on the **Members** tab. Or assign the group to users in **Users > <user> > Groups**.

You can repeat this for other groups you want to use later for per-app filtering, e.g. `Engineering`, `On-Call`.

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

### Using Settings

You can use the Settings page instead of editing `config.yaml`:

1. Open **Settings > Security**.
2. Choose **Single sign-on (OIDC)**.
3. For the issuer URL, enter the realm URL (`https://auth.example.com/realms/homelab`).
4. Enter the client ID from Step 1.
5. Enter the client secret from the client's Credentials tab.
6. For the groups claim, enter `groups`.
7. Under admin groups, enter your admin group (for example `Muximux-Admins`).
8. Copy the **Callback URL (register at the provider)** and **Back-channel logout URL** rows into the provider.
9. Press **Test connection**.
10. Press **Update Method**.

See [Authentication](authentication#setting-up-oidc-in-settings) for how the form handles the client secret and SSO-only sign-in.

---

## Step 5: Validate

1. Visit `https://muximux.example.com/login`.
2. Click **Login with SSO**. Muximux should send you to Keycloak.
3. Sign in at Keycloak. You should land back on the Muximux dashboard.
4. Sign in as a member of `Muximux-Admins`.
5. Confirm that the Settings gear appears in the navigation.
6. To see the actual claims Keycloak sends, open **Clients > muximux > Client scopes > Evaluate** in Keycloak admin.
7. Pick a user.
8. Click **Generated ID Token**. The `groups` claim should be there with the names you expect (no leading slash).

---

## Sign-out

Keycloak supports RP-initiated logout (Muximux sends the user to the provider to sign out). Its discovery document lists an `end_session_endpoint`. Keycloak also supports OIDC back-channel logout (the provider tells Muximux directly that a session ended).

- **Sign out at Keycloak from Muximux.** Set `provider_logout: true` under `auth.oidc`. In the client's settings, add Muximux's return address (`https://muximux.example.com/login?logged_out=1`, or your `post_logout_redirect_url`) to **Valid post logout redirect URIs**. A value of `+` reuses the **Valid redirect URIs** list. That list holds only the callback URL, so add the return address explicitly.
- **Sign out of Muximux when the user signs out at Keycloak.** Open the logout settings in the client's settings. Set **Backchannel logout URL** to `https://muximux.example.com/api/auth/oidc/backchannel-logout`. Turn **Backchannel logout session required** on, so that Keycloak includes the session ID (`sid`) in the logout token. Leave **Front channel logout** off. Keycloak uses the back-channel URL only when front-channel logout is off.

See [Authentication](authentication#oidc-openid-connect) for what each option does.

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
