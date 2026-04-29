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
)

declare -A PREFIX_GROUP
PREFIX_GROUP=(
    [internal/component/bgp/]=bgp
    [internal/core/]=core
    [internal/plugins/]=plugins
    [internal/component/config/]=config
    [internal/component/cli/]=cli
)

# Detect which groups have changes
declare -A hit
rest=0

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
        rest=1
    fi
done <<< "$changed"

if [ "$rest" = "1" ]; then
    hit[rest]=1
fi

if [ "$mode" = "pkgs" ]; then
    for group in "${!hit[@]}"; do
        if [ "$group" = "rest" ]; then
            # "rest" = everything not in a named group. Emit all ZE_PACKAGES
            # and let the caller de-dup or just run everything.
            echo "ALL"
        else
            echo "${GROUP_PKG[$group]}"
        fi
    done
else
    for group in "${!hit[@]}"; do
        echo "$group"
    done
fi
