# Freebuff registry patch (M-247)

Provenance: `/mnt/workspace/clouds/freebuff/current` is a symlink to release
`ea1dfe600d5f-idle` — not a git checkout (no `.git`, no VERSION file).

## Apply
```
/mnt/workspace/portal/patches/apply-patch [RELEASE_DIR]
```
Defaults to `current`. Idempotent: re-running is a no-op if already applied.

## Re-apply after an upgrade
1. Point `current` at the new release.
2. `/mnt/workspace/portal/patches/apply-patch` (patch is forward-only).
3. If a previous apply left rejects, reverse with
   `patch -R -p1 < freebuff-registry.patch` from the release root, then re-apply.
4. Restart the service (see below) and verify with `bin/ports list --app freebuff`.

## Service run/restart
Freebuff Web runs as the user systemd unit `freebuff-web.service`
(`~/.config/systemd/user/freebuff-web.service`), EnvironmentFile
`/mnt/workspace/clouds/freebuff/env/freebuff-web.env`, entry
`bun /mnt/workspace/clouds/freebuff/current/freebuff/server/src/index.ts`.
Restart: `systemctl --user restart freebuff-web.service`.
The unit has no ExecStartPre health check; readiness is observed via the
banner on stdout (~3-6s on this host). Downtime not separately measurable
beyond that; observed restart completes in ~5s.
EOF

# Freebuff multi-engine patch (M-322)

Provenance: `/mnt/workspace/clouds/freebuff/current` is a symlink to a release,
not a git checkout. `freebuff-multi-engine.patch` fronts daemon-spawned engines.

- `proxy.ts`: `launchId` injection (HTTP + WebSocket); version badge stamped
  on documents when `launchId` or `versionLabel` is set.
- `multi-engine.ts` (new): watches daemon `registry.json` (0600), one loopback
  listener per engine, registers `freebuff.proxy[<account>]` with the ports
  registry and heartbeats it every 25s (registry TTL is 60s). Display version
  prefers registry `version` (PR ddvnguyen/multica-x#193), else
  `FREEBUFF_APP_VERSION` from the engine's `/proc/<pid>/environ`
  (same-user, cmdline-guarded), else `unknown`.
- `index.ts`: single-engine proxy also stamps the badge
  (`FREEBUFF_WEB_VERSION` env override, else bundle version;
  `readAppVersion` is darwin-only).
- Badge text is the version tag only: `<semver>-<YYMMMDD.HHMM>` where the
  stamp is the engine process start time (server-local, via `/proc/<pid>`),
  plain `<semver>` when unknown. No product prefix, no account email (the
  portal tab and registration title still carry the account).

## Apply
```
/mnt/workspace/portal/patches/apply-multi-engine [RELEASE_DIR]
```
Idempotent: re-running is a no-op if already applied. Restart
`freebuff-multi-engine.service` + `freebuff-web.service` afterwards.
