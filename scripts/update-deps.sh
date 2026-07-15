#!/usr/bin/env bash
#
# Refresh Go and frontend dependencies to their latest minor/patch releases,
# then verify the build and tests still pass. Run this right before tagging a
# release so the build never ships behind what Dependabot would open PRs for.
#
# Major upgrades are intentionally NOT pulled in here: `go get -u` stays within
# each module's current major (Go semantic import versioning), and `npm update`
# stays within each package's `^` range. Majors keep arriving as reviewable
# Dependabot PRs -- you want eyes on those anyway. GitHub Actions pins and the
# Docker base image are likewise left to Dependabot (they aren't Go/npm deps).
#
# Usage:
#   scripts/update-deps.sh
#
# Then review the printed diff, commit the dependency files, and tag the
# release. Pushing runs the full coverage gate via the pre-push hook. If a
# bump breaks a test, this script fails here -- before you commit -- so you
# can back it out or investigate.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

step() { printf '\n\033[0;36m▸ %s\033[0m\n' "$1"; }

step "Updating Go modules (minor/patch, no majors)"
go get -u ./...
go mod tidy

step "Updating frontend packages (minor/patch, within semver ranges)"
( cd web && npm update )

step "Verifying Go build + tests"
# Mirror the pre-push hook's package selection: skip cmd/ (main entry points)
# and stray Go files vendored under web/node_modules.
GO_PACKAGES=$(go list ./... | grep -v '/cmd/' | grep -v 'node_modules')
go test -count=1 $GO_PACKAGES

step "Verifying frontend build + tests"
( cd web && npm run build && npx vitest run )
# The production build rewrites this generated file; keep it out of the diff.
git checkout -- web/src/lib/paraglide/.prettierignore 2>/dev/null || true

step "Done"
if git --no-pager diff --quiet -- go.mod go.sum web/package.json web/package-lock.json; then
  printf 'No dependency updates were available -- everything is already current.\n'
else
  printf 'Dependency changes (verified green):\n\n'
  git --no-pager diff --stat -- go.mod go.sum web/package.json web/package-lock.json
  printf '\nNext: commit these files, then tag the release.\n'
fi
