#!/bin/bash
# PreToolUse hook: Block auto-registration in init()
# BLOCKING: Explicit over implicit (design-principles.md)

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

# Skip test files
if [[ "$FILE_PATH" =~ _test\.go$ ]]; then
    exit 0
fi

# Skip register.go and register_*.go files — init() registration is the
# established pattern for plugins (internal/plugins/*/register.go), schema
# packages, and per-module RPC registration (server/register_*.go)
BASENAME="$(basename "$FILE_PATH")"
if [[ "$BASENAME" == "register.go" || "$BASENAME" == register_*.go ]]; then
    exit 0
fi

# Skip any file that calls RegisterRPCs() — this is the canonical RPC
# self-registration API. Handler files across all packages use init() +
# RegisterRPCs() to register where commands are defined.
if echo "$CONTENT" | grep -qE 'RegisterRPCs\('; then
    exit 0
fi

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()

# Check for init() functions that do registration
# First, find init() functions
if echo "$CONTENT" | grep -qE '^func init\(\)'; then
    # Check if init contains registration patterns
    INIT_BODY=$(echo "$CONTENT" | sed -n '/^func init()/,/^func /p' | head -30)

    REGISTER_PATTERNS=(
        'Register'
        'Add.*Handler'
        'Subscribe'
        'Hook'
        'global.*='
        'default.*='
    )

    for pattern in "${REGISTER_PATTERNS[@]}"; do
        if echo "$INIT_BODY" | grep -qiE "$pattern"; then
            ERRORS+=("Auto-registration in init(): $pattern")
            ERRORS+=("→ Use explicit registration call instead")
            break
        fi
    done
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}❌ BLOCKED: Implicit behavior in init()${RESET}" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}✗${RESET} $err" >&2
    done
    echo "" >&2
    echo -e "  ${YELLOW}Explicit > Implicit: Use explicit registration calls${RESET}" >&2
    echo -e "  ${YELLOW}See .claude/rules/design-principles.md${RESET}" >&2
    exit 2
fi

exit 0
