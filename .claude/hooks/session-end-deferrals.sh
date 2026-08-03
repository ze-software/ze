#!/bin/bash
# Stop hook: Print open deferrals as a session-end reminder
# Advisory only -- ensures visibility, not blocking.
#
# Deferrals are sharded one file per source under plan/deferrals/ so concurrent
# sessions never cross-commit each other's rows (ai/rules/planning.md,
# ai/rules/git-safety.md). The open-count is a fold over the directory.

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || exit 0

DEFERRALS_DIR="plan/deferrals"
if [[ ! -d "$DEFERRALS_DIR" ]]; then
    exit 0
fi

# A shard heading / note line has no '|', so awk -F'|' gives NF<7 and is skipped;
# the header and separator rows carry status "Status"/"------" and never match
# "open". Fold every shard together.
COUNT_OPEN() {
    awk -F'|' '
    NF < 7 { next }
    {
        status = $7; gsub(/^[ \t]+|[ \t]+$/, "", status)
    }
    tolower(status) == "open" { count++ }
    END { print count+0 }
    ' "$DEFERRALS_DIR"/*.md 2>/dev/null
}

OPEN_COUNT=$(COUNT_OPEN)

if [[ -z "$OPEN_COUNT" || "$OPEN_COUNT" -eq 0 ]]; then
    exit 0
fi

CYAN='\033[36m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

echo -e "${CYAN}${BOLD}Open deferrals: $OPEN_COUNT${RESET}" >&2

# Print each open deferral compactly, across all shards.
awk -F'|' '
NF < 7 { next }
{
    status = $7; gsub(/^[ \t]+|[ \t]+$/, "", status)
    what = $4; gsub(/^[ \t]+|[ \t]+$/, "", what)
    dest = $6; gsub(/^[ \t]+|[ \t]+$/, "", dest)
    source = $3; gsub(/^[ \t]+|[ \t]+$/, "", source)
}
tolower(status) == "open" {
    printf "  - %s [%s] -> %s\n", what, source, dest
}
' "$DEFERRALS_DIR"/*.md 2>/dev/null >&2

exit 0
