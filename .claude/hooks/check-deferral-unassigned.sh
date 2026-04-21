#!/bin/bash
# BLOCKING HOOK: No open deferrals without a destination
# Every deferral must name a receiving spec or be explicitly cancelled.
# Exit code 2 = BLOCK the commit.

INPUT=$(cat)
COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

# Only trigger on git commit commands
if [[ "$COMMAND" != *"git commit"* ]]; then
    exit 0
fi

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || exit 0

DEFERRALS_FILE="plan/deferrals.md"
if [[ ! -f "$DEFERRALS_FILE" ]]; then
    exit 0
fi

# Parse table: find open deferrals with empty/placeholder destination
# Table: | Date | Source | What | Reason | Destination | Status |
# awk fields: $1=empty $2=Date $3=Source $4=What $5=Reason $6=Destination $7=Status

UNASSIGNED=$(awk -F'|' '
NR <= 2 { next }                                    # skip header + separator
NF < 7 { next }                                     # skip malformed rows
{
    status = $7; gsub(/^[ \t]+|[ \t]+$/, "", status)
    dest = $6; gsub(/^[ \t]+|[ \t]+$/, "", dest)
    what = $4; gsub(/^[ \t]+|[ \t]+$/, "", what)
    tolower_status = tolower(status)
    tolower_dest = tolower(dest)
}
tolower_status == "open" && (dest == "" || tolower_dest == "-" || tolower_dest == "unassigned" || tolower_dest == "tbd" || tolower_dest == "none") {
    printf "  - %s (destination: \"%s\")\n", what, dest
}
' "$DEFERRALS_FILE")

if [[ -z "$UNASSIGNED" ]]; then
    exit 0
fi

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

echo -e "${RED}${BOLD}  BLOCKED: Open deferrals without destination${RESET}" >&2
echo "" >&2
echo "$UNASSIGNED" >&2
echo "" >&2
echo -e "  ${YELLOW}Every open deferral must name a receiving spec or be cancelled.${RESET}" >&2
echo -e "  ${YELLOW}Update the Destination column in plan/deferrals.md${RESET}" >&2
echo -e "  ${YELLOW}See ai/rules/deferral-tracking.md${RESET}" >&2
exit 2
