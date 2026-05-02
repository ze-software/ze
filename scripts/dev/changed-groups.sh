#!/usr/bin/env bash
# changed-groups.sh -- emit the test group(s) containing modified .go files.
#
# Groups map directory prefixes to short names used by make targets
# (ze-test-bgp, ze-test-core, etc.). If no .go files changed, prints
# nothing. If a change doesn't match any group, prints "rest".
#
# Usage: scripts/dev/changed-groups.sh          → one group name per line
#        scripts/dev/changed-groups.sh --pkgs   → Go package patterns per line

set -e

mode="groups"
if [ "${1:-}" = "--pkgs" ]; then
    mode="pkgs"
fi

# Collect changed .go files (unstaged + staged + untracked)
changed=$({
    git diff --name-only -- '*.go'
    git diff --cached --name-only -- '*.go'
    git ls-files --others --exclude-standard -- '*.go'
} 2>/dev/null | sort -u)

if [ -z "$changed" ]; then
    exit 0
fi

# Map: directory prefix → group name → Go package pattern
# Order matters: first match wins (most specific first).
declare -A GROUP_PKG
GROUP_PKG=(
    [bgp]="./internal/component/bgp/..."
    [core]="./internal/core/..."
    [plugins]="./internal/plugins/..."
    [config]="./internal/component/config/..."
    [cli]="./internal/component/cli/..."
    [l2tp]="./internal/component/l2tp/..."
    [ppp]="./internal/component/ppp/..."
    [web]="./internal/component/web/..."
    [api]="./internal/component/api/..."
    [cmd]="./cmd/..."
    [test]="./internal/test/..."
)

declare -A PREFIX_GROUP
PREFIX_GROUP=(
    [internal/component/bgp/]=bgp
    [internal/core/]=core
    [internal/plugins/]=plugins
    [internal/component/config/]=config
    [internal/component/cli/]=cli
    [internal/component/l2tp/]=l2tp
    [internal/component/ppp/]=ppp
    [internal/component/web/]=web
    [internal/component/api/]=api
    [cmd/]=cmd
    [internal/test/]=test
)

# Detect which groups have changes
declare -A hit
rest_pkgs=()

while IFS= read -r file; do
    matched=0
    for prefix in "${!PREFIX_GROUP[@]}"; do
        if [[ "$file" == ${prefix}* ]]; then
            hit[${PREFIX_GROUP[$prefix]}]=1
            matched=1
            break
        fi
    done
    if [ "$matched" = "0" ]; then
        # Derive the Go package directory from the file path.
        pkg_dir=$(dirname "$file")
        rest_pkgs+=("./$pkg_dir")
    fi
done <<< "$changed"

# Deduplicate unmapped package directories.
if [ ${#rest_pkgs[@]} -gt 0 ]; then
    mapfile -t rest_pkgs < <(printf '%s\n' "${rest_pkgs[@]}" | sort -u)
fi

if [ "$mode" = "pkgs" ]; then
    for group in "${!hit[@]}"; do
        echo "${GROUP_PKG[$group]}"
    done
    for pkg in "${rest_pkgs[@]}"; do
        echo "$pkg"
    done
else
    for group in "${!hit[@]}"; do
        echo "$group"
    done
    if [ ${#rest_pkgs[@]} -gt 0 ]; then
        echo "rest"
    fi
fi
