#!/usr/bin/env bash
#
# Check Go docstring coverage for exported identifiers.
# Usage: scripts/check-docstrings.sh [THRESHOLD]
#   THRESHOLD — minimum coverage percentage (default: 80)
#
# Scans all non-test .go files and reports how many exported identifiers
# (functions, types, vars, consts) have a preceding doc comment.

set -euo pipefail

THRESHOLD="${1:-80}"
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

total=0
documented=0

# Find all non-test Go files
while IFS= read -r file; do
    # Read file into array for line-by-line scanning
    mapfile -t lines < "$file"
    num_lines=${#lines[@]}

    for ((i = 0; i < num_lines; i++)); do
        line="${lines[$i]}"

        # Match exported identifiers: func, type, var, const starting with uppercase
        if echo "$line" | grep -qE '^(func|type|var|const) [A-Z]'; then
            total=$((total + 1))

            # Check if preceding line is a doc comment
            if [ "$i" -gt 0 ]; then
                prev="${lines[$((i - 1))]}"
                if echo "$prev" | grep -qE '^//[[:space:]]'; then
                    documented=$((documented + 1))
                fi
            fi
        fi

        # Match exported method receivers: func (x Type) ExportedName(
        if echo "$line" | grep -qE '^func \([^)]+\) [A-Z]'; then
            total=$((total + 1))

            if [ "$i" -gt 0 ]; then
                prev="${lines[$((i - 1))]}"
                if echo "$prev" | grep -qE '^//[[:space:]]'; then
                    documented=$((documented + 1))
                fi
            fi
        fi
    done
done < <(find "$REPO_ROOT" -name '*.go' ! -name '*_test.go' -type f)

if [ "$total" -eq 0 ]; then
    echo "No exported identifiers found."
    exit 0
fi

pct=$((documented * 100 / total))

echo "Docstring coverage: ${documented}/${total} exported identifiers (${pct}%)"

if [ "$pct" -lt "$THRESHOLD" ]; then
    echo "FAIL: ${pct}% is below the ${THRESHOLD}% threshold"

    # Report undocumented identifiers
    echo ""
    echo "Undocumented identifiers:"
    while IFS= read -r file; do
        mapfile -t lines < "$file"
        num_lines=${#lines[@]}
        rel_path="${file#"$REPO_ROOT"/}"

        for ((i = 0; i < num_lines; i++)); do
            line="${lines[$i]}"
            is_exported=false

            if echo "$line" | grep -qE '^(func|type|var|const) [A-Z]'; then
                is_exported=true
            elif echo "$line" | grep -qE '^func \([^)]+\) [A-Z]'; then
                is_exported=true
            fi

            if [ "$is_exported" = true ]; then
                has_doc=false
                if [ "$i" -gt 0 ]; then
                    prev="${lines[$((i - 1))]}"
                    if echo "$prev" | grep -qE '^//[[:space:]]'; then
                        has_doc=true
                    fi
                fi
                if [ "$has_doc" = false ]; then
                    # Extract identifier name
                    name=$(echo "$line" | sed -E 's/^(func |type |var |const )//; s/^(\([^)]+\) )//; s/[( {].*//; s/ .*//')
                    echo "  ${rel_path}:$((i + 1)): ${name}"
                fi
            fi
        done
    done < <(find "$REPO_ROOT" -name '*.go' ! -name '*_test.go' -type f)

    exit 1
fi

echo "PASS: ${pct}% meets the ${THRESHOLD}% threshold"
