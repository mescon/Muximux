#!/bin/bash
# Hook script: Ensures web/dist/ is synced to internal/server/dist/ before go build.
# Called by Claude Code PreToolUse hook when a Bash command runs.

PROJECT_DIR="/home/mescon/Projects/muximux3"
WEB_DIST="$PROJECT_DIR/web/dist"
SERVER_DIST="$PROJECT_DIR/internal/server/dist"

INPUT=$(cat)
COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty' 2>/dev/null)
[[ -z "$COMMAND" ]] && exit 0

# Only act on go build / make build commands that reference this project
case "$COMMAND" in
  *"go build"*|*"make build"*|*"make backend"*) ;;
  *) exit 0 ;;
esac

# Check if web/dist exists
[[ ! -f "$WEB_DIST/index.html" ]] && exit 0

# Compare by checking if the JS bundle filename matches
WEB_JS=$(basename "$(ls "$WEB_DIST/assets/"index-*.js 2>/dev/null | head -1)" 2>/dev/null)
SERVER_JS=$(basename "$(ls "$SERVER_DIST/assets/"index-*.js 2>/dev/null | head -1)" 2>/dev/null)

if [[ "$WEB_JS" != "$SERVER_JS" ]]; then
  echo "Syncing web/dist/ → internal/server/dist/" >&2
  rm -rf "$SERVER_DIST"
  cp -r "$WEB_DIST" "$SERVER_DIST"
fi

exit 0
