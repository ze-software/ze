#!/bin/bash
# Stop hook: Print open deferrals as a session-end reminder
# Advisory only -- ensures visibility, not blocking.

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || exit 0

DEFERRALS_FILE="plan/deferrals.md"
if [[ ! -f "$DEFERRALS_FILE" ]]; then
    exit 0
fi

# Count open deferrals
OPEN_COUNT=$(awk -F'|' '
NR <= 2 { next }
NF < 7 { next }
{
    status = $7; gsub(/^[ \t]+|[ \t]+$/, "", status)
}
tolower(status) == "open" { count++ }
END { print count+0 }
' "$DEFERRALS_FILE")

if [[ "$OPEN_COUNT" -eq 0 ]]; then
    exit 0
fi

CYAN='\033[36m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

echo -e "${CYAN}${BOLD}Open deferrals: $OPEN_COUNT${RESET}" >&2

# Print each open deferral compactly
awk -F'|' '
NR <= 2 { next }
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
' "$DEFERRALS_FILE" >&2

exit 0
