#!/bin/sh
#
# Docker entrypoint with PUID/PGID support.
# Creates a runtime user matching the requested UID/GID so bind-mount
# permissions work correctly on the host (linuxserver.io convention).
#
# Docker socket access is detected automatically: if the operator
# bind-mounts /var/run/docker.sock into the container, this script
# reads the socket's group ownership and adds the runtime user to a
# matching group inside the container before dropping privileges.
# Operators don't need to set anything; mount the socket and the
# discovery feature works.
#
# DOCKER_GID is supported as an explicit override for unusual setups
# (rootless docker, socket-proxy sidecars, custom mount paths).
# DOCKER_SOCKET overrides the auto-detection path from
# /var/run/docker.sock.

set -e

PUID="${PUID:-1000}"
PGID="${PGID:-1000}"
DOCKER_SOCKET="${DOCKER_SOCKET:-/var/run/docker.sock}"

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

# Auto-detect the Docker socket's group ownership and grant the
# runtime user access. Mounting the socket is sufficient on its own;
# no extra env var needed. Explicit DOCKER_GID still wins if set.
DOCKER_NOTE=""
DOCKER_GROUP_GID=""
if [ -n "$DOCKER_GID" ]; then
    DOCKER_GROUP_GID="$DOCKER_GID"
elif [ -S "$DOCKER_SOCKET" ]; then
    DOCKER_GROUP_GID=$(stat -c '%g' "$DOCKER_SOCKET" 2>/dev/null || echo "")
fi
if [ -n "$DOCKER_GROUP_GID" ] && [ "$DOCKER_GROUP_GID" != "0" ] && [ "$DOCKER_GROUP_GID" != "$PGID" ]; then
    if ! getent group "$DOCKER_GROUP_GID" >/dev/null 2>&1; then
        addgroup -g "$DOCKER_GROUP_GID" docker_host
    fi
    DOCKER_GROUP_NAME=$(getent group "$DOCKER_GROUP_GID" | cut -d: -f1)
    addgroup "$USER_NAME" "$DOCKER_GROUP_NAME" 2>/dev/null || true
    DOCKER_NOTE=" +${DOCKER_GROUP_NAME}(${DOCKER_GROUP_GID})"
elif [ -n "$DOCKER_GROUP_GID" ] && [ "$DOCKER_GROUP_GID" = "0" ]; then
    DOCKER_NOTE=" (docker socket owned by root; consider adjusting host perms)"
fi

# Ensure the data directory is owned by the runtime user
chown -R "$PUID:$PGID" /app/data 2>/dev/null || true

echo "Starting Muximux as ${USER_NAME}(${PUID}):${GROUP_NAME}(${PGID})${DOCKER_NOTE}"

# Use the username form of su-exec so initgroups(3) runs and the
# user's supplementary groups (including the docker group above)
# are carried into the muximux process. The numeric form (UID:GID)
# would skip initgroups and silently drop all supplementary groups.
exec su-exec "$USER_NAME" "$@"
