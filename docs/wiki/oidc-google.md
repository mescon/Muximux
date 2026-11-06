# OIDC: Google

This guide sets up Muximux to sign users in with Google accounts. It uses a Google Cloud OAuth 2.0 client. It works for both consumer Gmail accounts and Google Workspace accounts.

> **Note:** Google's standard OIDC tokens **do not** include group memberships. Workspace can expose groups, but only through a separate Google Cloud API call. Muximux does not currently make this call. In practice:
>
> - Sign-in works fine for any Google account you allow.
> - `admin_groups` will not promote users to admin based on Workspace groups. Instead, decide admin status by **email allowlist** (see Step 4). Or manage the Muximux user record manually after first login.
> - Per-app `allowed_groups` filtering also will not apply to users who sign in with Google, because Google sends no groups.
>
> If you need group-based filtering, put Keycloak / Authentik / Pocket ID / Zitadel in front of Google as a federated identity layer (a provider that signs users in through another provider). Most of those can use Google as an upstream identity source. They still send their own `groups` claim to Muximux.

---

## Prerequisites

- A Google account that owns (or has Editor on) a Google Cloud project. A project is free to create.
- Muximux on a stable HTTPS URL. Google won't accept HTTP redirect URIs except for `localhost`.

---

## Step 1: Create the OAuth Consent Screen

1. Open [Google Cloud Console](https://console.cloud.google.com).
2. Pick or create a project for Muximux.
3. In the left nav, go to **APIs & Services > OAuth consent screen**.
4. Choose the user type:
   - **External** if you want any Google account to sign in (typical for personal/homelab).
   - **Internal** if your project is in a Google Workspace and you only want users from that org.
5. Click **Create**.
6. Fill in these fields:
   - App name: `Muximux`
   - User support email: your email
   - Developer contact: your email
7. On the **Scopes** screen, click **Add or Remove Scopes**.
8. Add `openid`, `email`, and `profile`. No other scopes are needed.
9. If you chose **External**, set who can sign in. Do one of these:
   - On **Test users**, add the Google accounts that will sign in. This applies only to the External user type while the app is in testing mode.
   - Click **Publish App** to take the consent screen to production. This removes the test-user restriction. For an unverified app, Google still warns first-time users.

---

## Step 2: Create the OAuth Client

1. Open **APIs & Services > Credentials**.
2. Click **Create Credentials > OAuth client ID**.
3. Application type: **Web application**.
4. Name: `Muximux`.
5. Authorized redirect URIs: `https://muximux.example.com/api/auth/oidc/callback`. Use your real Muximux URL.
6. Click **Create**.
7. Copy the **Client ID** and **Client Secret** that Google shows.

---

## Step 3: Configure Muximux

```yaml
auth:
  method: oidc
  oidc:
    enabled: true
    issuer_url: https://accounts.google.com
    client_id: <client id from Step 2>
    client_secret: ${GOOGLE_CLIENT_SECRET}
    redirect_url: https://muximux.example.com/api/auth/oidc/callback
    scopes:
      - openid
      - profile
      - email
    # Google emits 'email' as the most stable identifier; using it as the
    # username avoids collisions when users change their display name.
    username_claim: email
    email_claim: email
    display_name_claim: name
    # Leave admin_groups unset; Google does not emit groups in the ID token.
```

Set `GOOGLE_CLIENT_SECRET` in the environment. Restart Muximux.

---

## Step 4: Decide Who Becomes Admin

`admin_groups` doesn't apply, so you set admin status per user. There are two approaches:

**Option 1: First user becomes admin, edit roles after.**
Muximux gives the first user who signs in the user role. An existing admin can promote that user in **Settings > Security > Users** in the Muximux UI. This admin is typically the operator who first set up Muximux through the wizard. This works because an OIDC user shows up in the user list after their first sign-in. Then an admin can change their role.

**Option 2: Restrict at the consent screen.**
If your Google Cloud project has User type **Internal**, only members of your Workspace can sign in. If it's **External** with **Test users** set, only the listed users can sign in. Either way, combine these IdP-side restrictions with manual role assignment in Muximux. This keeps the set of admins small.

For a Workspace-only deployment, **Internal** + manual role assignment in Muximux is the simplest pattern.

### Using Settings

You can use the Settings page instead of editing `config.yaml`:

1. Open **Settings > Security**.
2. Choose **Single sign-on (OIDC)**.
3. For the issuer URL, enter `https://accounts.google.com`.
4. Enter the client ID and secret from Step 2.
5. For the username claim, enter `email`.
6. Leave admin groups empty, because Google does not send groups.
7. Copy the **Callback URL (register at the provider)** and **Back-channel logout URL** rows into the provider.
8. Press **Test connection**.
9. Press **Update Method**.

See [Authentication](authentication#setting-up-oidc-in-settings) for how the form handles the client secret and SSO-only sign-in.

---

## Step 5: Validate

1. Visit `https://muximux.example.com/login`.
2. Click **Login with SSO**. Google's consent screen appears.
3. Approve. You should land back on the Muximux dashboard.
4. Check the role. The first sign-in lands as a regular user. To grant admin, use an existing admin account to change the role in **Settings > Security > Users**.

---

## Sign-out

Google does not publish an `end_session_endpoint` in its discovery document. It also does not offer OIDC back-channel logout (the provider tells Muximux directly that a session ended). So provider sign-out is not available. `provider_logout` has nothing to call, so logging out of Muximux only ends the Muximux session. `logout_url` cannot replace the missing endpoint in any useful way. The user stays signed in to their Google account, which is usually what you want.

If you want Muximux to be SSO-only, `auto_redirect` and `disable_local_login` still work with Google. See [Authentication](authentication#sso-only-sign-in).

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `redirect_uri_mismatch` from Google | The URI in your OAuth client doesn't exactly match what Muximux sends. | In **Credentials**, edit the OAuth client and add the exact URL, including `https://`, host, and `/api/auth/oidc/callback`. No wildcards. |
| `access_denied` immediately after consent | The Google account isn't in the project's test-user list (External + testing) or isn't part of the Workspace (Internal). | Add the user to test users, or publish the app, or switch User type. |
| Sign-in works but you wanted groups | Google doesn't emit groups in standard OIDC tokens. | Use a federated IdP (Keycloak, Authentik, etc.) with Google as an upstream identity source, or assign Muximux roles manually. |
| Browser warns "Google hasn't verified this app" | The OAuth consent screen is still in unverified state. | Either complete Google's verification process (only required if you're publishing to a wide audience) or accept the warning for trusted users. |

---

## See Also

- [Authentication overview](authentication) for the full `auth.oidc` reference.
- Other identity providers: [Microsoft Entra ID](oidc-microsoft-entra-id), [Keycloak](oidc-keycloak), [Authentik](oidc-authentik), [Pocket ID](oidc-pocket-id), [Zitadel](oidc-zitadel), [Authelia](forward-auth-authelia), [Cloudflare Access](forward-auth-cloudflare-access).
