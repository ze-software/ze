#!/bin/bash
# PreToolUse hook: Block utils/helpers packages
# BLOCKING: No "utils" or "helpers" packages (design-principles.md)

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // .tool_input.new_string // empty')

# Only process Write for new Go files
if [[ "$TOOL_NAME" != "Write" ]]; then
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

# Check if creating file in utils/helpers directory
if [[ "$FILE_PATH" =~ /utils/ ]] || [[ "$FILE_PATH" =~ /helpers/ ]] || [[ "$FILE_PATH" =~ /common/ ]] || [[ "$FILE_PATH" =~ /misc/ ]]; then
    ERRORS+=("Creating file in forbidden package directory")
    ERRORS+=("→ 'utils', 'helpers', 'common', 'misc' are anti-patterns")
    ERRORS+=("→ Put code in domain-specific packages")
fi

# Check for package declaration
if echo "$CONTENT" | grep -qE '^package[[:space:]]+(utils|helpers|common|misc)[[:space:]]*$'; then
    ERRORS+=("Package name 'utils/helpers/common/misc' forbidden")
    ERRORS+=("→ Use domain-specific package name")
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}❌ BLOCKED: Forbidden package pattern${RESET}" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}✗${RESET} $err" >&2
    done
    echo "" >&2
    echo -e "  ${YELLOW}Where does new code go in 'utils'? Nowhere good.${RESET}" >&2
    echo -e "  ${YELLOW}See .claude/rules/design-principles.md${RESET}" >&2
    exit 2
fi

exit 0
