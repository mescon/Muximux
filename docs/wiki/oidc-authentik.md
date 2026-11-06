# OIDC: Authentik

This guide sets up Muximux to sign users in with Authentik. It also shows how to put Authentik group memberships in the ID token (the signed sign-in record that Authentik sends to Muximux). Then the groups are available to `admin_groups` and per-app `allowed_groups`.

When you're done, users sign in with their Authentik account. Muximux receives their groups as `groups: ["Engineering", "Muximux-Admins"]`. Admins land on Settings automatically.

---

## Prerequisites

- A running Authentik instance. Any 2024.x release works. Older versions also work, but the UI labels here come from current Authentik.
- An admin account on the Authentik side.
- Muximux on a stable URL (HTTPS recommended).

---

## Step 1: Create the OAuth2 / OpenID Provider

1. In Authentik admin, open **Applications > Providers**.
2. Click **Create**.
3. Select **OAuth2/OpenID Provider**.
4. Click **Next**.
5. Name: `Muximux Provider`.
6. **Authorization flow**: pick `default-provider-authorization-implicit-consent` (or any flow that suits your consent policy).
7. **Protocol settings**:
   - Client type: **Confidential**
   - Client ID: leave the auto-generated value or override (e.g. `muximux`).
   - Client Secret: copy this value. You will paste it into `config.yaml`.
   - Redirect URIs / Origins: `https://muximux.example.com/api/auth/oidc/callback`. Use **Strict** matching mode if available.
   - Signing Key: pick `authentik Self-signed Certificate` unless you've configured your own.
8. Under **Advanced protocol settings**:
   - Subject mode: `Based on the User's username` (or `Based on the User's UPN` if your users come from a directory).
9. Save.

---

## Step 2: Create the Application

1. Open **Applications > Applications**.
2. Click **Create**.
3. Name: `Muximux`. Slug: `muximux`.
4. **Provider**: pick the `Muximux Provider` you just created.
5. Optional: launch URL `https://muximux.example.com/`.
6. Save.

You can now bind policies to this application to restrict who can sign in. Use Authentik > Applications > muximux > Policy / Group / User Bindings. This works the same way for any OIDC client and does not depend on Muximux.

---

## Step 3: Make Sure Groups Are in the ID Token

By default, Authentik includes a `groups` claim in the userinfo endpoint. The `goauthentik.io/providers/oauth2/scope-openid-userinfo` property mapping (a rule that adds data to the token) does this. Check for this claim, or add it:

1. Open the **Muximux Provider** (Applications > Providers > Muximux Provider).
2. Scroll to **Scopes** and confirm the following scopes are in the **Selected** list:
   - `email`
   - `openid`
   - `profile`
3. Open the property mapping list and confirm that a `groups` mapping is selected. If you don't see one, create a new one under **Customisation > Property Mappings > Create > Scope Mapping**:
   - Name: `OAuth2 Groups`
   - Scope name: `groups`
   - Expression:
     ```python
     return {"groups": [group.name for group in user.ak_groups.all()]}
     ```
   - Add `groups` to the provider's scope list.
4. Save.

You can use the full Authentik group path instead (rare). To do this, use `group.path` instead of `group.name` in the expression. Then change your `admin_groups` to match.

---

## Step 4: Identify or Create the Admin Group

Muximux promotes a user to admin only through a group. Authentik needs a group whose name you list in `admin_groups`. Reuse a group you already have, or create a new one:

1. Open **Directory > Groups > Create**.
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

The Authentik `issuer_url` (the address Muximux uses to find the provider) is application-scoped. That is why it has the `/application/o/<application-slug>/` suffix. The trailing slash matters. To be sure, copy the **OpenID Configuration Issuer** value from the provider's overview page.

Set `AUTHENTIK_CLIENT_SECRET` in the environment. Restart Muximux.

### Using Settings

You can use the Settings page instead of editing `config.yaml`:

1. Open **Settings > Security**.
2. Choose **Single sign-on (OIDC)**.
3. For the issuer URL, enter the application-scoped **OpenID Configuration Issuer** value (`https://authentik.example.com/application/o/muximux/`, trailing slash included).
4. Enter the client ID and secret from the provider.
5. Add `groups` to the scopes.
6. For the groups claim, enter `groups`.
7. Under admin groups, enter your admin group (for example `Muximux-Admins`).
8. Copy the **Callback URL (register at the provider)** and **Back-channel logout URL** rows into the provider.
9. Press **Test connection**.
10. Press **Update Method**.

See [Authentication](authentication#setting-up-oidc-in-settings) for how the form handles the client secret and SSO-only sign-in.

---

## Step 6: Validate

1. Visit `https://muximux.example.com/login` and click **Login with SSO**.
2. Sign in at Authentik. The first time, Authentik can also ask for consent, depending on the flow you chose.
3. Approve. Authentik sends you back to Muximux.
4. Sign in as a member of `Muximux-Admins`.
5. Confirm that the Settings gear is visible.
6. To see what Muximux actually receives, open **Authentik admin > Events > Logs** after you sign in. The `Token issued` event shows the claims that Authentik put into the ID token, including `groups`.

---

## Sign-out

authentik supports RP-initiated logout (Muximux sends the user to the provider to sign out). Its OIDC discovery document lists an `end_session_endpoint`. authentik also supports OIDC back-channel logout (the provider tells Muximux directly that a session ended).

- **Back-channel logout.** Edit the OAuth2/OpenID provider (**Applications > Providers**). Enter `https://muximux.example.com/api/auth/oidc/backchannel-logout` in the **Logout URI** field. Choose **Back-channel** as the **Logout Method**. The authentik documentation lists this feature as available from authentik 2025.8.
- **Post-logout redirect.** authentik added `post_logout_redirect_uri` support for OAuth2 providers in release 2026.5. In that release, you can give the provider's redirect URIs a logout type. On older releases, or if you cannot find the setting, check the authentik documentation for your version. The exact label could not be confirmed here. Register `https://muximux.example.com/login?logged_out=1` (or your `post_logout_redirect_url`) there. If no post-logout redirect URIs are saved on the provider, authentik skips the matching.

See [Authentication](authentication#oidc-openid-connect) for what each Muximux option does.

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
