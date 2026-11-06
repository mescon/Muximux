# OIDC: Microsoft Entra ID

This guide shows how to set up Muximux to sign users in with Microsoft Entra ID (formerly Azure AD). It also shows how to send Entra group memberships to Muximux. Muximux uses these groups for its `admin_groups` and per-app `allowed_groups` features.

When you're done, users sign in to Muximux with their corporate Microsoft account. Muximux receives their group memberships as readable names like `Engineering`, not as GUIDs (long random IDs).

---

## Prerequisites

- A Microsoft Entra ID tenant (your organization's Entra directory) where you can create app registrations. You typically need the **Cloud Application Administrator** role (or higher).
- Muximux already serving on a stable HTTPS URL. A self-signed certificate is fine for testing. Production needs a real certificate, so that the browser follows the redirect back from `login.microsoftonline.com`.
- Your Muximux base URL. This guide uses `https://muximux.example.com`. Put your real URL in its place everywhere.

---

## Step 1: Register the Application in Entra ID

1. Open the [Entra admin center](https://entra.microsoft.com).
2. Open **Applications > App registrations**.
3. Click **New registration**.
4. Fill in these fields:
   - **Name**: `Muximux` (or the name you want users to see on the consent screen).
   - **Supported account types**: pick **Accounts in this organizational directory only**. Pick another option only if you need multi-tenant or personal Microsoft accounts.
   - **Redirect URI**: choose **Web** and enter `https://muximux.example.com/api/auth/oidc/callback`. The path is fixed. The host must match your Muximux URL exactly.
5. Click **Register**.

Entra shows the Overview page. Write down these two values. You will paste them into `config.yaml` later:

- **Application (client) ID**
- **Directory (tenant) ID**

---

## Step 2: Create a Client Secret

1. In the app registration, open **Certificates & secrets > Client secrets**.
2. Click **New client secret**.
3. For **Description**, enter `Muximux OIDC`.
4. For **Expires**, pick a value your policy allows. Rotate the secret before it expires.
5. Click **Add**.
6. **Immediately copy the Value** (not the Secret ID). Entra hides it after you leave the page.

---

## Step 3: Configure Group Claims

Many homelab and small-team setups skip this step. Then they cannot find out why `admin_groups` does not match. You must tell Entra to put groups in the ID token (the signed sign-in record that Entra sends to Muximux). You should also pick the group format.

1. In the app registration, open **Token configuration**.
2. Click **Add groups claim**.
3. Under **Select group types to include**, pick **Security groups**, unless you also use Microsoft 365 / mail-enabled groups for access decisions.
4. Expand the **ID** section. (Optionally also expand **Access** / **SAML** if you will use them later.)
5. In each section you expanded, choose how Entra sends groups:
   - **Group ID** (the default) sends group GUIDs. Muximux works, but your config will look like `admin_groups: [22e90a9d-1c5d-4f0b-9c5f-...]`.
   - **`sAMAccountName`** or **Cloud-only group display names** sends readable names like `Engineering`. Pick `sAMAccountName` if your tenant is hybrid (synced from on-prem AD). Pick **Cloud-only group display names** for cloud-only Entra tenants.
6. Click **Add**.

> **Note on group overage.** A user can be a member of more than ~200 groups. For that user, Entra sends a `_claim_names` claim instead, which points to Microsoft Graph. Muximux does **not** follow that pointer. If your tenant has users with that many groups, narrow the **Groups assigned to the application** filter on the **Groups claim** dialog. Then Entra sends only the groups Muximux cares about.

---

## Step 4: Grant API Permissions

By default, Entra's **OpenID Connect** permissions cover everything Muximux needs. To check this:

1. Open **API permissions**.
2. Look under Microsoft Graph. You should see `User.Read`, `openid`, `profile`, and `email`.
3. If `email` is missing, click **Add a permission > Microsoft Graph > Delegated permissions** and add it.

Your tenant can require admin consent for these scopes. Most tenants do not for the basic scopes. If yours does, click **Grant admin consent for <tenant>** at the top of the API permissions page.

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

Replace `<TENANT_ID>` and `<APPLICATION_CLIENT_ID>` with the values from Step 1. Set `ENTRA_CLIENT_SECRET` in the environment, so that the secret stays out of the YAML.

Restart Muximux. The `Login with SSO` button on `/login` now sends the user to Entra.

### Using Settings

You can use the Settings page instead of editing `config.yaml`:

1. Open **Settings > Security**.
2. Choose **Single sign-on (OIDC)**.
3. For the issuer URL, enter `https://login.microsoftonline.com/<TENANT_ID>/v2.0`.
4. For the client ID, enter the application (client) ID.
5. Enter the client secret value from Step 2.
6. For the groups claim, enter `groups`.
7. Under admin groups, enter the group name or GUID from Step 3.
8. Copy the **Callback URL (register at the provider)** and **Back-channel logout URL** rows into the provider.
9. Press **Test connection**.
10. Press **Update Method**.

See [Authentication](authentication#setting-up-oidc-in-settings) for how the form handles the client secret and SSO-only sign-in.

---

## Step 6: Validate

1. Sign in with a regular (non-admin) user. The user should land on the dashboard.
2. Sign in with a user who is a member of the group in `admin_groups`. The user should see the **Settings** gear in the navigation. Only admins see this gear.
3. Open Muximux's logs (`docker logs muximux` or systemd journal).
4. Look for `OIDC user logged in`. The line shows the resolved username and role. A user who should be admin can show `role=user`. In that case, the group claim is missing or has the wrong name.

---

## Sign-out

Entra ID offers RP-initiated logout (Muximux sends the user to the provider to sign out). It does this through the `end_session_endpoint` in its discovery document. So `provider_logout: true` works without setting `logout_url`.

Entra ID does not offer OIDC back-channel logout (the provider tells Muximux directly that a session ended). It offers **front-channel** logout, which Muximux does not implement. So signing out at Microsoft does not end the Muximux session.

- **Return address.** Microsoft requires the `post_logout_redirect_uri` to match a redirect URI registered on the app. In the app registration, open **Authentication**. Add `https://muximux.example.com/login?logged_out=1` (or your `post_logout_redirect_url`) under **Redirect URIs**.
- **Front-channel logout URL.** Leave the **Front-channel logout URL** field empty. Muximux has no endpoint for it.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `redirect_uri mismatch` from Entra | The URI in the app registration doesn't match what Muximux sends. | Copy the **Redirect URL** in your Muximux config (`redirect_url:` line) into Entra exactly, including scheme, host, port, and path. |
| User signs in but isn't promoted to admin even though they're in the right group | Group claim isn't configured (Step 3 skipped), or `admin_groups` has GUIDs but Entra is emitting names (or vice versa). | Add `?debug=true` to your Muximux URL after sign-in and check the browser console. Or set log level to `debug` and look at the `OIDC user logged in` line. Either the `groups` claim is missing, or its values don't match `admin_groups`. |
| `AADSTS50011: The redirect URI specified in the request does not match` | Trailing slash mismatch, or HTTPS vs HTTP. | Both sides must match byte-for-byte. |
| Entra returns `interaction_required` even after consent | Conditional Access policy is blocking the session. | Check **Microsoft Entra ID > Sign-in logs** for the user. The failure reason is logged there. |
| Group claim contains GUIDs you didn't expect | Token configuration emits Group ID instead of display name. | Re-open **Token configuration > groups claim** and switch to `sAMAccountName` or **Cloud-only group display names**. |

---

## See Also

- [Authentication overview](authentication) for the rest of `auth.oidc` and how OIDC interacts with the API key.
- Other identity providers: [Keycloak](oidc-keycloak), [Authentik](oidc-authentik), [Pocket ID](oidc-pocket-id), [Zitadel](oidc-zitadel), [Google](oidc-google), [Authelia](forward-auth-authelia), [Cloudflare Access](forward-auth-cloudflare-access).
