#!/bin/bash
# PreToolUse hook: Block version fields in config
# BLOCKING: Config must not have version numbers (config-design.md)

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // .tool_input.new_string // empty')

# Only process Write/Edit
if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then
    exit 0
fi

# Check config-related files
if [[ ! "$FILE_PATH" =~ /config/ ]] && [[ ! "$FILE_PATH" =~ \.conf$ ]] && [[ ! "$FILE_PATH" =~ config.*\.go$ ]]; then
    exit 0
fi

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()

# Check for version fields in config
VERSION_PATTERNS=(
    'version\s*[=:]\s*[0-9]'
    'Version\s*[=:]\s*[0-9]'
    '"version"\s*:'
    'version\s+[0-9]+\s*;'
    'config.?version'
    'schema.?version'
)

for pattern in "${VERSION_PATTERNS[@]}"; do
    if echo "$CONTENT" | grep -qiE "$pattern"; then
        ERRORS+=("Version field detected in config")
        ERRORS+=("→ Config must NOT have version numbers")
        ERRORS+=("→ Design for migration instead")
        break
    fi
done

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}❌ BLOCKED: Version in config${RESET}" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}✗${RESET} $err" >&2
    done
    echo "" >&2
    echo -e "  ${YELLOW}See .claude/rules/config-design.md${RESET}" >&2
    exit 2
fi

exit 0
