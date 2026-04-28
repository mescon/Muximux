# OIDC: Google

This guide sets up Muximux to authenticate with Google accounts via Google Cloud's OAuth 2.0 client. It works for both consumer Gmail accounts and Google Workspace accounts.

> **Heads up: groups don't come for free.** Google's standard OIDC tokens **do not** include group memberships. Workspace can expose groups, but only via a separate Google Cloud API call that Muximux does not currently make. Practically, this means:
>
> - Sign-in works fine for any Google account you allow.
> - `admin_groups` will not promote users to admin based on Workspace groups. Instead, decide admin status by **email allowlist** (see Step 4) or by managing the Muximux user record manually after first login.
> - The future per-app `allowed_groups` filtering will likewise not apply to Google-authenticated users.
>
> If you need group-based filtering, use Keycloak / Authentik / Pocket ID / Zitadel as a federated identity layer in front of Google instead. Most of those can use Google as an upstream identity source while still emitting their own `groups` claim downstream.

---

## Prerequisites

- A Google account that owns (or has Editor on) a Google Cloud project. This is free to create.
- Muximux on a stable HTTPS URL. Google won't accept HTTP redirect URIs except for `localhost`.

---

## Step 1: Create the OAuth Consent Screen

1. Open [Google Cloud Console](https://console.cloud.google.com), pick or create a project for Muximux.
2. In the left nav, go to **APIs & Services > OAuth consent screen**.
3. User type:
   - **External** if you want any Google account to sign in (typical for personal/homelab).
   - **Internal** if your project is in a Google Workspace and you only want users from that org.
4. Click **Create** and fill in:
   - App name: `Muximux`
   - User support email: your email
   - Developer contact: your email
5. On the **Scopes** screen, click **Add or Remove Scopes** and add `openid`, `email`, and `profile`. No other scopes are needed.
6. On **Test users** (only relevant for External user type while in testing mode), add the Google accounts that will sign in. Or click **Publish App** to take the consent screen to production, which removes the test-user restriction (though for an unverified app Google will still warn first-time users).

---

## Step 2: Create the OAuth Client

1. In **APIs & Services > Credentials**, click **Create Credentials > OAuth client ID**.
2. Application type: **Web application**.
3. Name: `Muximux`.
4. Authorized redirect URIs: `https://muximux.example.com/api/auth/oidc/callback`. Substitute your real Muximux URL.
5. Click **Create**. Copy the **Client ID** and **Client Secret** that Google shows.

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

Since `admin_groups` doesn't apply, admin status is per-user. Two approaches:

**Option 1: First user becomes admin, edit roles after.**
The first user to sign in is given the user role. An existing admin (typically the operator who first set up Muximux through the wizard) can promote them via **Settings > Security > Users** in the Muximux UI. This works because once an OIDC user has signed in once, they show up in the user list and their role can be changed.

**Option 2: Restrict at the consent screen.**
If your Google Cloud project has User type **Internal**, only members of your Workspace can sign in. If it's **External** with **Test users** set, only those listed can sign in. Either way, you can keep the admin set tight by combining IdP-side restrictions with manual role assignment in Muximux.

For a Workspace-only deployment, **Internal** + manual role assignment in Muximux is the simplest pattern.

---

## Step 5: Validate

1. Visit `https://muximux.example.com/login` and click **Login with SSO**.
2. Google's consent screen appears. After approval, you should land back on the Muximux dashboard.
3. The first sign-in lands as a regular user. To grant admin, use an existing admin account to change the role in **Settings > Security > Users**.

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
