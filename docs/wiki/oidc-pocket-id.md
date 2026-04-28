# OIDC: Pocket ID

[Pocket ID](https://github.com/stonith404/pocket-id) is a lightweight self-hosted OIDC provider that authenticates users with passkeys instead of passwords. It's a popular choice in homelab setups because the entire IdP is one small container.

This guide gets Muximux signing in against Pocket ID with group memberships flowing through to `admin_groups`.

---

## Prerequisites

- A running Pocket ID instance, accessible to whichever browser will use Muximux. The default port is `1411`; you'll typically reverse-proxy it at something like `https://id.example.com`.
- Admin access to Pocket ID.
- Muximux on a stable URL.

---

## Step 1: Register the OIDC Client

1. In Pocket ID admin, open **OIDC Clients** and click **Add OIDC Client**.
2. Fill in:
   - Name: `Muximux`
   - Callback URLs: `https://muximux.example.com/api/auth/oidc/callback`
   - Logout Callback URLs: leave empty unless you've configured a Muximux logout redirect.
   - Public Client: **Off** (we want a confidential client with a secret)
   - PKCE: leave at the default.
3. Save. Pocket ID shows you a **Client ID** and **Client Secret**. Copy both.

---

## Step 2: Allow the Client to See Group Memberships

Pocket ID emits the `groups` claim only when the client is allowed to read it.

1. In the same OIDC client edit page, find the **Allowed User Groups** section.
2. Add the groups you want Muximux to see. Anyone who belongs to one of these groups can sign in to Muximux; everyone else gets refused at the IdP. Add at least:
   - `Muximux-Admins` (or whatever you want admin status to map to)
   - any other group you'll later use for per-app filtering
3. Save.

If you don't have those groups yet, create them under **Groups** first and assign members.

---

## Step 3: Configure Muximux

Edit `config.yaml`:

```yaml
auth:
  method: oidc
  oidc:
    enabled: true
    issuer_url: https://id.example.com
    client_id: <client id from Step 1>
    client_secret: ${POCKETID_CLIENT_SECRET}
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

`issuer_url` is the public URL of Pocket ID, with no trailing path. Pocket ID exposes the OIDC discovery document at `https://id.example.com/.well-known/openid-configuration`.

Set `POCKETID_CLIENT_SECRET` in the environment. Restart Muximux.

---

## Step 4: Validate

1. Visit `https://muximux.example.com/login` and click **Login with SSO**. Pocket ID prompts you for a passkey.
2. After successful sign-in you should land back on the Muximux dashboard.
3. Sign in as a member of `Muximux-Admins` and confirm the Settings gear is visible.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `unauthorized_client` from Pocket ID | The client is set to **Public** but Muximux is sending a client secret. | Switch the client to confidential (turn **Public Client** off). |
| Sign-in fails with "user is not allowed to use this client" | The user isn't in any group listed under **Allowed User Groups**. | Add the user (or one of their groups) to the client's allow list in Step 2. |
| Sign-in works but admin gear is missing | The user's groups don't include any value listed in `admin_groups`. | Confirm the group exists in Pocket ID and the user is a member. Check the audit log entry on Muximux: `OIDC user logged in` lists the resolved role. |
| `Failed to fetch discovery document` | `issuer_url` doesn't match Pocket ID's public URL exactly. | Use the URL shown in your browser when you visit Pocket ID's UI, no trailing path. |

---

## See Also

- [Authentication overview](authentication) for the full `auth.oidc` reference.
- Other identity providers: [Microsoft Entra ID](oidc-microsoft-entra-id), [Keycloak](oidc-keycloak), [Authentik](oidc-authentik), [Zitadel](oidc-zitadel), [Google](oidc-google), [Authelia](forward-auth-authelia), [Cloudflare Access](forward-auth-cloudflare-access).
