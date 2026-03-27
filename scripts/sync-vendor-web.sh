#!/usr/bin/env bash
# Sync vendored web assets to all consumer directories.
# Source of truth: third_party/web/htmx/
# See third_party/web/MANIFEST.md for the asset inventory.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VENDOR="$ROOT/third_party/web/htmx"

CONSUMERS=(
    "$ROOT/internal/chaos/web/assets"
    "$ROOT/internal/component/web/assets"
)

if [ ! -d "$VENDOR" ]; then
    echo "error: vendor directory not found: $VENDOR" >&2
    exit 1
fi

changed=0

for dest in "${CONSUMERS[@]}"; do
    if [ ! -d "$dest" ]; then
        echo "warning: consumer directory not found, skipping: $dest" >&2
        continue
    fi

    for file in htmx.min.js sse.js; do
        src="$VENDOR/$file"
        dst="$dest/$file"

        if [ ! -f "$src" ]; then
            echo "warning: vendor file not found: $src" >&2
            continue
        fi

        if [ ! -f "$dst" ] || ! diff -q "$src" "$dst" > /dev/null 2>&1; then
            cp "$src" "$dst"
            echo "synced: $dst"
            changed=$((changed + 1))
        fi
    done
done

if [ "$changed" -eq 0 ]; then
    echo "all consumer copies are up to date"
else
    echo "synced $changed file(s)"
fi
