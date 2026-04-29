#!/usr/bin/env bash
# verify-summary.sh -- append a failure summary for one verify stage.
#
# Usage: verify-summary.sh <failures-log> <stage> <stage-log>
#
# Appends a short block to <failures-log> that names the stage, points
# at its full log, and extracts the key failure lines. Called by the
# Makefile when a ze-verify stage fails. Keeps the full stage log
# in place -- this is a pointer, not a replacement.

set -e

FAILURES="$1"
STAGE="$2"
LOG="$3"

if [ -z "$FAILURES" ] || [ -z "$STAGE" ] || [ -z "$LOG" ]; then
    echo "usage: $0 <failures-log> <stage> <stage-log>" >&2
    exit 2
fi

mkdir -p "$(dirname "$FAILURES")"

{
    printf '\n### Stage: %s\n' "$STAGE"
    printf 'Full log: %s\n\n' "$LOG"
    printf 'Key lines:\n'
    if [ ! -f "$LOG" ]; then
        echo "(stage log missing: $LOG)"
    else
        case "$STAGE" in
            lint)
                # golangci-lint output is already compact; cap at 100 lines.
                head -100 "$LOG"
                ;;
            *)
                # Go test / functional / exabgp: extract FAIL/panic/fatal lines + a bit of context.
                grep -nE '^(--- FAIL:|FAIL[[:space:]]|^panic:|^fatal error:|^Error:|\[FAIL\])' "$LOG" \
                    | head -80 \
                    || echo "(no obvious FAIL lines found; see full log)"
                ;;
        esac
    fi
} >> "$FAILURES"
