# OIDC: Zitadel

[Zitadel](https://zitadel.com) is a cloud-native identity platform. You can host it yourself or use the managed service. It has a multi-organization model: one instance can hold several separate organizations. People use it as a Keycloak alternative in Kubernetes and self-hosted setups.

This guide sets up Muximux to sign users in with Zitadel. Zitadel puts group memberships in the ID token (the signed sign-in record that Zitadel sends to Muximux). It does this through its project roles.

---

## Prerequisites

- A Zitadel instance, either self-hosted or on `zitadel.cloud`. You need an account with permission to create projects and applications in your organization.
- Muximux on a stable URL.

---

## Step 1: Create the Project

In Zitadel, applications live inside projects. Projects belong to organizations.

1. Sign in to Zitadel admin.
2. Pick the org you want Muximux under.
3. Open **Projects > Create**.
4. Name: `Muximux`.
5. Open the new project.
6. On the **General** tab, enable **Assert Roles on Authentication**. This setting puts role memberships in the ID token.
7. Optionally, enable **Check Roles on Authentication**. Then Zitadel refuses sign-in for users with no role at all in this project.

---

## Step 2: Define Roles

Roles in Zitadel are project-scoped strings. Muximux receives them in the `urn:zitadel:iam:org:project:roles` claim. To use them as Muximux groups, set `groups_claim` to that exact path.

1. In the project, open **Roles > New**.
2. Create at least:
   - Key: `muximux-admin`, Display Name: `Muximux Admin`
   - Key: `muximux-user`, Display Name: `Muximux User`
3. Add any other roles you will later use for per-app filtering, e.g. `engineering`, `on-call`.

---

## Step 3: Create the OIDC Application

1. Inside the project, open **New Application > Web > Next**.
2. Authentication method: **CODE** (authorization code with a client secret).
3. Redirect URIs: `https://muximux.example.com/api/auth/oidc/callback`
4. Post Logout Redirect URIs: `https://muximux.example.com/login` (or the page where you want users to land after logout).
5. Continue.
6. Copy the **Client ID** and **Client Secret** that Zitadel generates. You need both for `config.yaml`.
7. Open the application's **Token Settings** tab.
8. Confirm these settings:
   - **Auth Token Type**: `JWT`
   - **Add user roles to the access token**: enabled. This matters only if you will use access tokens. It is harmless either way.
   - **User roles inside ID Token**: enabled. This is the important toggle for Muximux.

---

## Step 4: Grant Roles to Users

Users get project roles through **Authorizations**. They get them either directly or through a Zitadel project grant.

1. In the project, open **Authorizations > New**.
2. Pick the user.
3. Choose the role you want the user to have (e.g. `muximux-admin`).
4. Save.
5. Repeat for any other users.

Zitadel assigns project roles to each user. It does not assign them to a group of users. So repeat the steps above for every user who needs a role.

---

## Step 5: Configure Muximux

```yaml
auth:
  method: oidc
  oidc:
    enabled: true
    issuer_url: https://your-zitadel.example.com
    client_id: <client id from Step 3>
    client_secret: ${ZITADEL_CLIENT_SECRET}
    redirect_url: https://muximux.example.com/api/auth/oidc/callback
    scopes:
      - openid
      - profile
      - email
      - urn:zitadel:iam:org:project:id:zitadel:aud
    username_claim: preferred_username
    email_claim: email
    # The Zitadel claim that carries project roles. Muximux treats its keys
    # as the user's group list; the values are role display metadata that
    # Muximux ignores.
    groups_claim: "urn:zitadel:iam:org:project:roles"
    display_name_claim: name
    admin_groups:
      - muximux-admin
```

The unusual scope (`urn:zitadel:iam:org:project:id:zitadel:aud`) adds the Zitadel project to the token audience. The audience is the `aud` claim: the list of intended receivers of the token. The scope does not replace the client ID in that list. Muximux accepts an ID token only if its `aud` claim contains the `client_id` from your config. If validation fails, decode the ID token and check its `aud` claim. If the client ID is missing, a change to the project in this scope does not fix it.

`issuer_url` is the base URL of your Zitadel instance, with no trailing path.

Set `ZITADEL_CLIENT_SECRET` in the environment. Restart Muximux.

### Using Settings

You can use the Settings page instead of editing `config.yaml`:

1. Open **Settings > Security**.
2. Choose **Single sign-on (OIDC)**.
3. For the issuer URL, enter the base URL of your Zitadel instance.
4. Enter the client ID from Step 3.
5. Enter the client secret from Step 3. The connection test does not check the secret, but Zitadel needs it when Muximux signs a user in.
6. Add the extra audience scope shown above to the scopes.
7. For the groups claim, enter `urn:zitadel:iam:org:project:roles`.
8. Under admin groups, enter your role key (for example `muximux-admin`).
9. Copy the **Callback URL (register at the provider)** and **Back-channel logout URL** rows into the provider.
10. Press **Test connection**.
11. Press **Update Method**.

See [Authentication](authentication#setting-up-oidc-in-settings) for how the form handles the client secret and SSO-only sign-in.

---

## Step 6: Validate

1. Visit `https://muximux.example.com/login`.
2. Click **Login with SSO**.
3. Sign in to Zitadel.
4. If Zitadel shows the consent screen, approve it.
5. Confirm that a `muximux-admin` user sees the Settings gear.
6. To see what Zitadel actually sent, open **Project > Application > Token Inspector** in Zitadel.
7. Exchange a fresh code. The decoded ID token shows the `urn:zitadel:iam:org:project:roles` map.

---

## Sign-out

Zitadel supports RP-initiated logout (Muximux sends the user to the provider to sign out) at `/oidc/v1/end_session`. Discovery lists it as `end_session_endpoint`. Zitadel also supports OIDC back-channel logout (the provider tells Muximux directly that a session ended).

- **Return address.** The `post_logout_redirect_uri` must exactly match one of the post-logout URIs registered on the application. Add `https://muximux.example.com/login?logged_out=1` (or your `post_logout_redirect_url`) to the application's post-logout redirect URIs. These are in the **Redirect Settings** section of the application in Console. Then set `provider_logout: true`.
- **Back-channel logout.** Register `https://muximux.example.com/api/auth/oidc/backchannel-logout` as the application's back-channel logout URI (`backchannel_logout_uri`). We could not confirm the label of this field in Console from the documentation. Check the application settings in your Zitadel version. After you set it, Zitadel notifies Muximux for sessions that started after you saved the setting.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Sign-in fails with `audience not allowed` or `invalid token` | The ID token's `aud` claim does not contain the `client_id` that Muximux has. | Check that `client_id` is the Client ID of this application in Zitadel. Decode the ID token and check that its `aud` claim contains that ID. |
| Sign-in works but admin gear is missing | Either **User roles inside ID Token** is off, or `groups_claim` doesn't match the URN. | Re-check Step 3's token settings, then verify `groups_claim:` is exactly `urn:zitadel:iam:org:project:roles`. |
| User has no role and sign-in is refused | **Check Roles on Authentication** is on and the user has no project authorization. | Either grant the user a role (Step 4), or turn off **Check Roles on Authentication** to allow role-less sign-in. |
| `unauthorized_client` | Authentication method on the application doesn't match (e.g. set to `NONE` but Muximux is sending a secret). | Edit the application and set Authentication method to **CODE**. |

---

## See Also

- [Authentication overview](authentication) for the full `auth.oidc` reference.
- Other identity providers: [Microsoft Entra ID](oidc-microsoft-entra-id), [Keycloak](oidc-keycloak), [Authentik](oidc-authentik), [Pocket ID](oidc-pocket-id), [Google](oidc-google), [Authelia](forward-auth-authelia), [Cloudflare Access](forward-auth-cloudflare-access).
