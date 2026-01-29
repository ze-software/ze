#!/bin/bash
# PreToolUse hook: Block adding linter exclusions
# BLOCKING: Fix code, don't add exclusions (quality.md)

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // .tool_input.new_string // empty')

# Only process Write/Edit
if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then
    exit 0
fi

# Only check golangci config
if [[ ! "$FILE_PATH" =~ \.golangci ]] && [[ "$FILE_PATH" != *"golangci.yml"* ]] && [[ "$FILE_PATH" != *"golangci.yaml"* ]]; then
    exit 0
fi

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()

# Check for new exclusions being added
# Detect patterns that add exclusions
if echo "$CONTENT" | grep -qE 'exclude-rules:|exclude:|issues-exclude:|skip-files:|skip-dirs:'; then
    # Check if this is adding to exclusions (not just reading existing)
    if echo "$CONTENT" | grep -qE '^\s*-\s*(path|text|linters|source):'; then
        ERRORS+=("Adding linter exclusions")
        ERRORS+=("→ Fix the code instead of excluding it")
        ERRORS+=("→ See .claude/rules/quality.md")
    fi
fi

# Check for disabling linters
if echo "$CONTENT" | grep -qE 'disable:\s*$|disable:.*-' | grep -vE '#.*disable'; then
    ERRORS+=("Disabling linters")
    ERRORS+=("→ Fix the code, don't disable the linter")
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}❌ BLOCKED: Adding linter exclusions${RESET}" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}✗${RESET} $err" >&2
    done
    echo "" >&2
    echo -e "  ${YELLOW}Allowed exclusions: fieldalignment, test file dupl/goconst/prealloc/gosec${RESET}" >&2
    echo -e "  ${YELLOW}Everything else: FIX THE CODE${RESET}" >&2
    exit 2
fi

exit 0
