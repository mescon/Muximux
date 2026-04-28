# OIDC: Zitadel

[Zitadel](https://zitadel.com) is a self-hosted (or managed) cloud-native identity platform with a multi-organization model. It's used as a Keycloak alternative in Kubernetes and self-hosted setups.

This guide sets up Muximux to authenticate against Zitadel with group memberships emitted in the ID token via Zitadel's project-roles mechanism.

---

## Prerequisites

- A Zitadel instance, either self-hosted or on `zitadel.cloud`. You need an account with permission to create projects and applications in your organization.
- Muximux on a stable URL.

---

## Step 1: Create the Project

In Zitadel, applications live inside projects, and projects belong to organizations.

1. Sign in to Zitadel admin, pick the org you want Muximux under, and open **Projects > Create**.
2. Name: `Muximux`.
3. Open the new project. On the **General** tab:
   - Enable **Assert Roles on Authentication**. This is what makes role memberships ride in the ID token.
   - Optionally enable **Check Roles on Authentication** to refuse sign-in for users with no role at all in this project.

---

## Step 2: Define Roles

Roles in Zitadel are project-scoped strings; Muximux will receive them in the `urn:zitadel:iam:org:project:roles` claim. To use them as Muximux groups, set `groups_claim` to that exact path.

1. In the project, open **Roles > New**.
2. Create at least:
   - Key: `muximux-admin`, Display Name: `Muximux Admin`
   - Key: `muximux-user`, Display Name: `Muximux User`
3. Add any other roles you'll later use for per-app filtering, e.g. `engineering`, `on-call`.

---

## Step 3: Create the OIDC Application

1. Inside the project, open **New Application > Web > Next**.
2. Authentication method: **CODE** (authorization code with a client secret).
3. Redirect URIs: `https://muximux.example.com/api/auth/oidc/callback`
4. Post Logout Redirect URIs: `https://muximux.example.com/login` (or wherever you want users to land after logout).
5. Continue and copy the **Client ID** and **Client Secret** Zitadel generates. Both are needed for `config.yaml`.

After creation, open the application's **Token Settings** tab and confirm:

- **Auth Token Type**: `JWT`
- **Add user roles to the access token**: enabled (only relevant if you'll consume access tokens; harmless either way)
- **User roles inside ID Token**: enabled. This is the important toggle for Muximux.

---

## Step 4: Grant Roles to Users

Users get project roles via **Authorizations**, either directly or through a Zitadel project grant.

1. In the project, open **Authorizations > New**.
2. Pick the user, choose the role you want them to have (e.g. `muximux-admin`).
3. Save. Repeat for any other users.

For groups of users, create a Zitadel **Group**, assign roles to the group, and add users to the group.

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

The unusual scope (`urn:zitadel:iam:org:project:id:zitadel:aud`) tells Zitadel that Muximux's audience is the Zitadel project itself; without it, ID token validation can fail in strict-audience mode. Replace `zitadel` in that scope name with your project's resource ID if you have a different one.

`issuer_url` is the base URL of your Zitadel instance, no trailing path.

Set `ZITADEL_CLIENT_SECRET` in the environment. Restart Muximux.

---

## Step 6: Validate

1. Visit `https://muximux.example.com/login` and click **Login with SSO**.
2. Sign in to Zitadel and approve the consent screen if shown.
3. Confirm a `muximux-admin` user sees the Settings gear.
4. To inspect what Zitadel actually emitted: in Zitadel, open **Project > Application > Token Inspector** and exchange a fresh code; the decoded ID token shows the `urn:zitadel:iam:org:project:roles` map.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Sign-in fails with `audience not allowed` or `invalid token` | The audience scope from Step 5 is missing or names the wrong project. | Open the application's **URLs** tab in Zitadel; the scope it expects is shown there. Copy verbatim. |
| Sign-in works but admin gear is missing | Either **User roles inside ID Token** is off, or `groups_claim` doesn't match the URN. | Re-check Step 3's token settings, then verify `groups_claim:` is exactly `urn:zitadel:iam:org:project:roles`. |
| User has no role and sign-in is refused | **Check Roles on Authentication** is on and the user has no project authorization. | Either grant the user a role (Step 4), or turn off **Check Roles on Authentication** to allow role-less sign-in. |
| `unauthorized_client` | Authentication method on the application doesn't match (e.g. set to `NONE` but Muximux is sending a secret). | Edit the application and set Authentication method to **CODE**. |

---

## See Also

- [Authentication overview](authentication) for the full `auth.oidc` reference.
- Other identity providers: [Microsoft Entra ID](oidc-microsoft-entra-id), [Keycloak](oidc-keycloak), [Authentik](oidc-authentik), [Pocket ID](oidc-pocket-id), [Google](oidc-google), [Authelia](forward-auth-authelia), [Cloudflare Access](forward-auth-cloudflare-access).
