#!/usr/bin/env bash
# Check npm registry for newer versions of vendored web assets.
# Compares against versions recorded in vendor/web/MANIFEST.md.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MANIFEST="$ROOT/vendor/web/MANIFEST.md"

if [ ! -f "$MANIFEST" ]; then
    echo "error: MANIFEST.md not found: $MANIFEST" >&2
    exit 1
fi

check_npm_version() {
    local pkg="$1"
    local current="$2"
    local latest

    latest=$(curl -s "https://registry.npmjs.org/$pkg/latest" 2>/dev/null | grep -oP '"version"\s*:\s*"\K[^"]+' | head -1)

    if [ -z "$latest" ]; then
        echo "  $pkg: could not fetch latest version (network error?)"
        return
    fi

    if [ "$current" = "$latest" ]; then
        echo "  $pkg: $current (up to date)"
    else
        echo "  $pkg: $current -> $latest available"
    fi
}

echo "checking vendored web assets against npm registry..."
echo ""

# Extract versions from MANIFEST.md table rows
htmx_version=$(grep -P 'htmx\.min\.js' "$MANIFEST" | grep -oP '\d+\.\d+\.\d+' | head -1)
sse_version=$(grep -P 'sse\.js' "$MANIFEST" | grep -oP '\d+\.\d+\.\d+' | head -1)

if [ -n "$htmx_version" ]; then
    check_npm_version "htmx.org" "$htmx_version"
else
    echo "  htmx.org: version not found in MANIFEST.md"
fi

if [ -n "$sse_version" ]; then
    check_npm_version "htmx-ext-sse" "$sse_version"
else
    echo "  htmx-ext-sse: version not found in MANIFEST.md"
fi

# Check that consumer copies match vendor copies
echo ""
echo "checking consumer copies..."

VENDOR="$ROOT/vendor/web/htmx"
CONSUMERS=(
    "$ROOT/internal/chaos/web/assets"
    "$ROOT/internal/component/web/assets"
)

drift=0
for dest in "${CONSUMERS[@]}"; do
    for file in htmx.min.js sse.js; do
        src="$VENDOR/$file"
        dst="$dest/$file"
        if [ -f "$dst" ]; then
            if ! diff -q "$src" "$dst" > /dev/null 2>&1; then
                echo "  DRIFT: $dst differs from vendor copy"
                drift=1
            fi
        else
            echo "  MISSING: $dst"
            drift=1
        fi
    done
done

if [ "$drift" -eq 0 ]; then
    echo "  all consumer copies match vendor"
fi
