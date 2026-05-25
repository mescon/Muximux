# HTTP Actions

An app with `open_mode: http_action` doesn't navigate when you click it. It fires an HTTP request against the configured URL via Muximux's server-side relay, audit-logs the result, and shows a toast.

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

The `url`, `min_role`, `allowed_groups`, `enabled`, and `icon` fields work the same as every other app. Per-app access control is enforced server-side: a user who can't see the app in the menu can't fire its action either.

## How it works

When you click an `http_action` app:

1. The browser POSTs `/api/app-action/{name}` to Muximux (with the session cookie).
2. The backend looks up the app, verifies your role and group memberships, and fires the configured method/headers against the app's `url`.
3. The result (status + latency) is shown as a toast. Audit log records method, host, caller, status, and latency.

The relay timeout is 10 seconds, hard-coded. Redirects are followed. The response body is never returned to the browser or logged.

## Security model

- **No URL allowlist.** Operators trust the URLs they configure. The validator only enforces `http://` or `https://` scheme and a non-empty hostname.
- **Audit log records the host only.** Webhook URLs commonly embed secrets in query strings (`?token=...`); we never log the path or query string, only the hostname.
- **CSRF protection** is the same as every other state-changing endpoint: the `X-Requested-With: XMLHttpRequest` header on the request is required.
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

Headers are `Key=Value` pairs, comma-separated. Values can contain `=`; the split is on the first `=` only.
