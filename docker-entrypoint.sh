#!/bin/sh
#
# Docker entrypoint with PUID/PGID support.
# Creates a runtime user matching the requested UID/GID so bind-mount
# permissions work correctly on the host (linuxserver.io convention).

set -e

PUID="${PUID:-1000}"
PGID="${PGID:-1000}"

# Create group if GID doesn't exist
if ! getent group "$PGID" >/dev/null 2>&1; then
    addgroup -g "$PGID" muximux
fi
GROUP_NAME=$(getent group "$PGID" | cut -d: -f1)

# Create user if UID doesn't exist
if ! getent passwd "$PUID" >/dev/null 2>&1; then
    adduser -D -u "$PUID" -G "$GROUP_NAME" -h /app muximux
fi
USER_NAME=$(getent passwd "$PUID" | cut -d: -f1)

# Ensure the data directory is owned by the runtime user
chown -R "$PUID:$PGID" /app/data 2>/dev/null || true

echo "Starting Muximux as ${USER_NAME}(${PUID}):${GROUP_NAME}(${PGID})"

exec su-exec "$PUID:$PGID" "$@"
