# OIDC: Microsoft Entra ID

This guide walks through setting up Muximux to authenticate users against Microsoft Entra ID (formerly Azure AD), including how to surface Entra group memberships so they can be used by Muximux's `admin_groups` and per-app `allowed_groups` features.

The end state: users sign in to Muximux with their corporate Microsoft account, and Muximux receives their group memberships as readable names like `Engineering` rather than GUIDs.

---

## Prerequisites

- A Microsoft Entra ID tenant where you can create app registrations. You typically need the **Cloud Application Administrator** role (or higher).
- Muximux already serving on a stable HTTPS URL. Self-signed is fine for testing, but production needs a real certificate so the browser will follow the redirect back from `login.microsoftonline.com`.
- Your Muximux base URL (referred to as `https://muximux.example.com` below). Substitute your real one everywhere.

---

## Step 1: Register the Application in Entra ID

1. In the [Entra admin center](https://entra.microsoft.com), open **Applications > App registrations** and click **New registration**.
2. Fill in:
   - **Name**: `Muximux` (or whatever you'd like users to see on the consent screen).
   - **Supported account types**: pick **Accounts in this organizational directory only** unless you specifically need multi-tenant or personal Microsoft accounts.
   - **Redirect URI**: choose **Web** and enter `https://muximux.example.com/api/auth/oidc/callback`. The path is fixed; the host must match your Muximux URL exactly.
3. Click **Register**.

On the resulting Overview page, note these two values, you'll paste them into `config.yaml` later:

- **Application (client) ID**
- **Directory (tenant) ID**

---

## Step 2: Create a Client Secret

1. In the app registration, open **Certificates & secrets > Client secrets** and click **New client secret**.
2. Description: `Muximux OIDC`. Expires: pick whatever your policy allows; rotate before it lapses.
3. Click **Add**, then **immediately copy the Value** (not the Secret ID). Entra hides it after you leave the page.

---

## Step 3: Configure Group Claims

This is the step most homelab and small-team setups skip and then can't figure out why `admin_groups` isn't matching. Entra needs to be told to emit groups in the ID token, and you should pick the format.

1. In the app registration, open **Token configuration**.
2. Click **Add groups claim**.
3. **Select group types to include**: pick **Security groups** unless you also use Microsoft 365 / mail-enabled groups for access decisions.
4. Under **ID** (and optionally **Access** / **SAML** if you'll use them later), expand the section and choose how groups are emitted:
   - **Group ID** (the default) emits group GUIDs. Muximux will work but your config will look like `admin_groups: [22e90a9d-1c5d-4f0b-9c5f-...]`.
   - **`sAMAccountName`** or **Cloud-only group display names** is what you want for readable names like `Engineering`. Pick `sAMAccountName` if your tenant is hybrid (synced from on-prem AD); pick **Cloud-only group display names** for cloud-only Entra tenants.
5. Click **Add**.

> **Note on group overage.** When a user is a member of more than ~200 groups, Entra falls back to a `_claim_names` claim that points to Microsoft Graph. Muximux does **not** follow that pointer. If your tenant has users with that many groups, narrow the **Groups assigned to the application** filter on the **Groups claim** dialog so only the groups Muximux cares about are emitted.

---

## Step 4: Grant API Permissions

By default, Entra's **OpenID Connect** permissions cover everything Muximux needs. Verify by opening **API permissions**: you should see `User.Read`, `openid`, `profile`, and `email` under Microsoft Graph. If `email` is missing, click **Add a permission > Microsoft Graph > Delegated permissions** and add it.

If your tenant requires admin consent for these scopes (most do not for the basics), click **Grant admin consent for <tenant>** at the top of the API permissions page.

---

## Step 5: Configure Muximux

Edit `config.yaml`:

```yaml
auth:
  method: oidc
  oidc:
    enabled: true
    issuer_url: https://login.microsoftonline.com/<TENANT_ID>/v2.0
    client_id: <APPLICATION_CLIENT_ID>
    client_secret: ${ENTRA_CLIENT_SECRET}
    redirect_url: https://muximux.example.com/api/auth/oidc/callback
    scopes:
      - openid
      - profile
      - email
    # Entra puts the user principal name in `preferred_username`. Leave at default.
    # Entra emits `groups` once the claim is configured in Step 3.
    groups_claim: groups
    admin_groups:
      - Muximux-Admins      # match the display name (or GUID) you saw in Step 3
```

Replace `<TENANT_ID>` and `<APPLICATION_CLIENT_ID>` with the values from Step 1. Set `ENTRA_CLIENT_SECRET` in the environment so the secret stays out of the YAML.

Restart Muximux. The `Login with SSO` button on `/login` now sends the user to Entra.

---

## Step 6: Validate

1. Sign in with a regular (non-admin) user. They should land on the dashboard.
2. Sign in with a user who is a member of the group you listed under `admin_groups`. They should see the **Settings** gear in the navigation, which is admin-only.
3. Open Muximux's logs (`docker logs muximux` or systemd journal) and look for `OIDC user logged in`. The line shows the resolved username and role; if a user who should be admin shows `role=user`, the group claim is missing or misnamed.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `redirect_uri mismatch` from Entra | The URI in the app registration doesn't match what Muximux sends. | Copy the **Redirect URL** in your Muximux config (`redirect_url:` line) into Entra exactly, including scheme, host, port, and path. |
| User signs in but isn't promoted to admin even though they're in the right group | Group claim isn't configured (Step 3 skipped), or `admin_groups` has GUIDs but Entra is emitting names (or vice versa). | Add `?debug=true` to your Muximux URL after sign-in and check the browser console, or set log level to `debug` and look at the `OIDC user logged in` line. The `groups` claim either isn't there or has values that don't match `admin_groups`. |
| `AADSTS50011: The redirect URI specified in the request does not match` | Trailing slash mismatch, or HTTPS vs HTTP. | Both sides must match byte-for-byte. |
| Entra returns `interaction_required` even after consent | Conditional Access policy is blocking the session. | Check **Microsoft Entra ID > Sign-in logs** for the user; the failure reason is logged there. |
| Group claim contains GUIDs you didn't expect | Token configuration emits Group ID instead of display name. | Re-open **Token configuration > groups claim** and switch to `sAMAccountName` or **Cloud-only group display names**. |

---

## See Also

- [Authentication overview](authentication) for the rest of `auth.oidc` and how OIDC interacts with the API key.
- Other identity providers: [Keycloak](oidc-keycloak), [Authentik](oidc-authentik), [Pocket ID](oidc-pocket-id), [Zitadel](oidc-zitadel), [Google](oidc-google), [Authelia](forward-auth-authelia), [Cloudflare Access](forward-auth-cloudflare-access).
