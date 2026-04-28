# OIDC: Authentik

This guide sets up Muximux to authenticate against Authentik, including emitting Authentik group memberships in the ID token so they're available to `admin_groups` (and, in a future release, per-app `allowed_groups`).

The end state: users sign in with their Authentik account, Muximux receives their groups as `groups: ["Engineering", "Muximux-Admins"]`, and admins land on Settings automatically.

---

## Prerequisites

- A running Authentik instance (any 2024.x release works; older versions also do, but the UI labels here come from current Authentik).
- An admin account on the Authentik side.
- Muximux on a stable URL (HTTPS recommended).

---

## Step 1: Create the OAuth2 / OpenID Provider

1. In Authentik admin, open **Applications > Providers** and click **Create**.
2. Select **OAuth2/OpenID Provider** and click **Next**.
3. Name: `Muximux Provider`.
4. **Authorization flow**: pick `default-provider-authorization-implicit-consent` (or any flow that suits your consent policy).
5. **Protocol settings**:
   - Client type: **Confidential**
   - Client ID: leave the auto-generated value or override (e.g. `muximux`).
   - Client Secret: copy this value, you'll paste it into `config.yaml`.
   - Redirect URIs / Origins: `https://muximux.example.com/api/auth/oidc/callback`. Use **Strict** matching mode if available.
   - Signing Key: pick `authentik Self-signed Certificate` unless you've configured your own.
6. Under **Advanced protocol settings**:
   - Subject mode: `Based on the User's username` (or `Based on the User's UPN` if your users come from a directory).
7. Save.

---

## Step 2: Create the Application

1. Open **Applications > Applications** and click **Create**.
2. Name: `Muximux`. Slug: `muximux`.
3. **Provider**: pick the `Muximux Provider` you just created.
4. Optional: launch URL `https://muximux.example.com/`.
5. Save.

You can now bind policies to this application if you want to restrict who can authenticate (Authentik > Applications > muximux > Policy / Group / User Bindings). This works the same way for any OIDC client and is independent of Muximux.

---

## Step 3: Make Sure Groups Are in the ID Token

By default, Authentik includes a `groups` claim in the userinfo endpoint via the `goauthentik.io/providers/oauth2/scope-openid-userinfo` property mapping. Verify or add it:

1. Open the **Muximux Provider** (Applications > Providers > Muximux Provider).
2. Scroll to **Scopes** and confirm the following scopes are in the **Selected** list:
   - `email`
   - `openid`
   - `profile`
3. Open the property mapping list and confirm a `groups` mapping is selected. If you don't see one, create a new one under **Customisation > Property Mappings > Create > Scope Mapping**:
   - Name: `OAuth2 Groups`
   - Scope name: `groups`
   - Expression:
     ```python
     return {"groups": [group.name for group in user.ak_groups.all()]}
     ```
   - Add `groups` to the provider's scope list.
4. Save.

If you'd rather use the full Authentik group path (rare), use `group.path` instead of `group.name` in the expression and adjust your `admin_groups` to match.

---

## Step 4: Identify or Create the Admin Group

For Muximux to promote anyone to admin, Authentik needs a group whose name you'll list in `admin_groups`. Either reuse one you already have or create a new one:

1. **Directory > Groups > Create**.
2. Name: `Muximux-Admins`.
3. Add members on the **Users** tab.

---

## Step 5: Configure Muximux

```yaml
auth:
  method: oidc
  oidc:
    enabled: true
    issuer_url: https://authentik.example.com/application/o/muximux/
    client_id: <client id from Step 1>
    client_secret: ${AUTHENTIK_CLIENT_SECRET}
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

The Authentik `issuer_url` is application-scoped, hence the `/application/o/<application-slug>/` suffix. The trailing slash matters; copy the **OpenID Configuration Issuer** value from the provider's overview page if you want to be sure.

Set `AUTHENTIK_CLIENT_SECRET` in the environment. Restart Muximux.

---

## Step 6: Validate

1. Visit `https://muximux.example.com/login` and click **Login with SSO**.
2. Authentik prompts for sign-in (and consent the first time, depending on the flow you chose). Approving redirects you back to Muximux.
3. Sign in as a member of `Muximux-Admins` and confirm the Settings gear is visible.
4. To inspect what Muximux actually receives: open **Authentik admin > Events > Logs** after signing in. The `Token issued` event shows the claims that were placed into the ID token, including `groups`.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `invalid_redirect_uri` from Authentik | Redirect URI not on the provider's allow list. | Add `https://muximux.example.com/api/auth/oidc/callback` exactly to **Redirect URIs / Origins** on the provider. |
| Sign-in works but admin promotion doesn't | Either the `groups` scope isn't being requested by Muximux, or the property mapping isn't producing groups. | Confirm `scopes:` in `config.yaml` includes `groups`. Then check **Events > Logs** for the latest `Token issued` event and look for the `groups` claim. |
| `error: unauthorized_client` | The client type or client authentication setting on the provider doesn't match what Muximux is sending. | Set the provider to **Confidential** and use the client secret. |
| `Failed to fetch discovery document` in Muximux logs | Wrong issuer URL. | Use the **OpenID Configuration Issuer** from the provider overview page; this is the canonical value Muximux should use. |
| Admin works for some users but not others | Some users are in groups whose names contain spaces or special characters that don't match `admin_groups` exactly. | Group matching is case-insensitive but otherwise literal. Make sure `admin_groups` matches the exact name Authentik reports. |

---

## See Also

- [Authentication overview](authentication) for the rest of `auth.oidc`.
- Other identity providers: [Microsoft Entra ID](oidc-microsoft-entra-id), [Keycloak](oidc-keycloak), [Pocket ID](oidc-pocket-id), [Zitadel](oidc-zitadel), [Google](oidc-google), [Authelia](forward-auth-authelia), [Cloudflare Access](forward-auth-cloudflare-access).
