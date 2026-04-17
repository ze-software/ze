#!/bin/bash
# PreToolUse hook: Block nolint abuse
# BLOCKING: nolint comments must have justification (quality.md)

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // .tool_input.new_string // empty')

# Only process Write/Edit for Go files
if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then
    exit 0
fi

if [[ ! "$FILE_PATH" =~ \.go$ ]]; then
    exit 0
fi

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()

# Check for nolint without explanation
# Valid: //nolint:errcheck // reason here
# Invalid: //nolint:errcheck (no reason)

# Find nolint directives
NOLINT_LINES=$(echo "$CONTENT" | grep -nE '//[[:space:]]*nolint' || true)

if [[ -n "$NOLINT_LINES" ]]; then
    while IFS= read -r line; do
        [[ -z "$line" ]] && continue

        # Check if there's a comment after the nolint directive
        # Pattern: //nolint:something // reason
        if ! echo "$line" | grep -qE '//[[:space:]]*nolint:[a-zA-Z,]+[[:space:]]+//'; then
            # No justification comment
            LINENUM=$(echo "$line" | cut -d: -f1)
            ERRORS+=("Line $LINENUM: nolint without justification")
            ERRORS+=("  $line")
            ERRORS+=("  → Add: //nolint:linter // reason for disabling")
        fi
    done <<< "$NOLINT_LINES"
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}❌ BLOCKED: nolint without justification${RESET}" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}✗${RESET} $err" >&2
    done
    echo "" >&2
    echo -e "  ${YELLOW}Format: //nolint:linter // reason for disabling${RESET}" >&2
    echo -e "  ${YELLOW}See .claude/rules/quality.md${RESET}" >&2
    exit 2
fi

exit 0
