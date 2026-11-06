# HTTP Actions

An app with `open_mode: http_action` doesn't navigate when you click it. Instead, Muximux sends an HTTP request to the configured URL through its server-side relay (the Muximux server sends the request for you). Muximux writes the result to the audit log and shows a toast (a short pop-up message).

Common uses:

- Trigger an n8n / Node-RED webhook (restart a stack, kick off a workflow).
- Hit Home Assistant `/api/services/...` to run an automation.
- Call Sonarr / Radarr command APIs (refresh, RSS sync).
- Reload nginx, rotate a key, post to a chat channel.

## Quick example

```yaml
apps:
  - name: Restart Sonarr
    url: https://n8n.local/webhook/restart-sonarr
    icon:
      type: lucide
      name: rotate-ccw
    color: "#3498db"
    group: Automations
    enabled: true
    open_mode: http_action
    http_action_method: POST
    http_action_headers:
      Authorization: Bearer abc123
      X-Source: muximux
    http_action_confirm: true        # show prompt before firing
    http_action_show_toast: true     # default; set false for silent fires
```

## Fields

| Field                       | Type   | Default | Notes                                                                       |
|-----------------------------|--------|---------|-----------------------------------------------------------------------------|
| `http_action_method`        | string | `POST`  | One of `GET`, `POST`, `PUT`, `DELETE`, `PATCH`                              |
| `http_action_headers`       | map    | (none)  | Sent verbatim. Keys must match `[A-Za-z0-9_-]+`. No CR/LF/NUL in values.    |
| `http_action_confirm`       | bool   | `false` | When true, click shows a confirmation modal with method + URL               |
| `http_action_show_toast`    | bool   | `true`  | When false, fires silently (audit log still records every fire)             |

The `url`, `min_role`, `allowed_groups`, `enabled`, and `icon` fields work the same as every other app. The server enforces per-app access control, including the `access` list. A user who can't see the app in the menu can't fire its action either. A disabled action (`enabled: false`) can't be fired, even by an admin.

## How it works

When you click an `http_action` app:

1. The browser POSTs `/api/app-action/{name}` to Muximux (with the session cookie).
2. The backend looks up the app and checks your role, group memberships and the app's `access` list. Then it sends the configured method/headers to the app's `url`.
3. Muximux shows the result (status + latency) as a toast. The audit log records method, host, caller, status, and latency.

The relay timeout is 10 seconds, and you cannot change it. The relay does not follow redirects. If the target returns a redirect, Muximux reports that 3xx status and does not request the new location. So configure the final URL directly. Muximux never returns the response body to the browser and never logs it.

## Security model

- **No URL allowlist.** Operators trust the URLs they configure. The validator only checks for an `http://` or `https://` scheme and a non-empty hostname.
- **Audit log records the host only.** Webhook URLs often contain secrets in the query string (`?token=...`). So we never log the path or query string, only the hostname. This also applies when a request fails: the error in the audit log and in the response leaves out the URL. Non-admins never receive the path or query string either: the app list sends them only the action's scheme and host. Health check errors leave out the URL in the same way.
- **CSRF protection** (protection against requests that other sites forge in your name) is the same as on every other state-changing endpoint. The request must carry the `X-Requested-With: XMLHttpRequest` header or a `Content-Type: application/json` header. The Muximux UI sends `X-Requested-With`.
- **Per-app role + group gate** mirrors the access control that hides the app from the menu.

## Discovery labels

You can declare an http_action via container labels:

| Label                                  | Type     | Example                                  |
|----------------------------------------|----------|------------------------------------------|
| `muximux.app.open_mode`                | string   | `http_action`                            |
| `muximux.app.http_action_method`       | string   | `POST`                                   |
| `muximux.app.http_action_headers`      | csv      | `Authorization=Bearer abc,X-Source=mxm`  |
| `muximux.app.http_action_confirm`      | boolish  | `true`                                   |
| `muximux.app.http_action_show_toast`   | boolish  | `false`                                  |

Headers are `Key=Value` pairs, separated by commas. A value can contain `=`, because Muximux splits each pair at the first `=` only.
