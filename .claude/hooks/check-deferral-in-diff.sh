#!/bin/bash
# BLOCKING HOOK: Deferral language in staged diff requires plan/deferrals.md update
# Catches: writing deferral intent in specs/code/docs without logging it.
# Exit code 2 = BLOCK the commit.

INPUT=$(cat)
COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

# Only trigger on git commit commands
if [[ "$COMMAND" != *"git commit"* ]]; then
    exit 0
fi

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || exit 0

# Get staged diff (added lines only, skip binary files)
DIFF=$(git diff --cached --no-color -U0 --diff-filter=AM 2>/dev/null || true)
if [[ -z "$DIFF" ]]; then
    exit 0
fi

# Extract only added lines (skip diff headers)
ADDED=$(echo "$DIFF" | grep '^+' | grep -v '^+++' | grep -v '^+$' || true)
if [[ -z "$ADDED" ]]; then
    exit 0
fi

# Exclude Go defer statements (legitimate keyword)
ADDED=$(echo "$ADDED" | grep -v '^\+[[:space:]]*defer [a-zA-Z]' || true)
if [[ -z "$ADDED" ]]; then
    exit 0
fi

# High-confidence deferral phrases
DEFERRAL_PATTERNS=(
    "deferred to"
    "deferred for"
    "defer to"
    "out of scope"
    "future work"
    "future spec"
    "handle later"
    "address later"
    "will be handled later"
    "will be done later"
    "will be addressed later"
    "skip for now"
    "skipping for now"
    "postpone"
    "not yet implemented"
    "not yet wired"
    "follow.up work"
)

HITS=()

for pattern in "${DEFERRAL_PATTERNS[@]}"; do
    MATCHES=$(echo "$ADDED" | grep -i "$pattern" || true)
    if [[ -n "$MATCHES" ]]; then
        HITS+=("  Pattern: '$pattern'")
        while IFS= read -r line; do
            [[ -n "$line" ]] && HITS+=("    ${line:1}")  # Strip leading +
        done <<< "$(echo "$MATCHES" | head -3)"
    fi
done

if [[ ${#HITS[@]} -eq 0 ]]; then
    exit 0
fi

# Deferral language found -- is plan/deferrals.md also staged?
DEFERRALS_STAGED=$(git diff --cached --name-only 2>/dev/null | grep '^plan/deferrals\.md$' || true)

if [[ -n "$DEFERRALS_STAGED" ]]; then
    # deferrals.md is being updated in this commit -- OK
    exit 0
fi

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

echo -e "${RED}${BOLD}  BLOCKED: Deferral language in staged changes without log entry${RESET}" >&2
echo "" >&2
for hit in "${HITS[@]}"; do
    echo -e "  ${RED}${hit}${RESET}" >&2
done
echo "" >&2
echo -e "  ${YELLOW}Record each deferral in plan/deferrals.md before committing.${RESET}" >&2
echo -e "  ${YELLOW}See ai/rules/deferral-tracking.md${RESET}" >&2
exit 2
