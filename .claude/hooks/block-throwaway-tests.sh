#!/bin/bash
# PreToolUse hook: Block throwaway test files
# BLOCKING: No temporary tests - add to proper test location (testing.md)

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# Only process Write tool
if [[ "$TOOL_NAME" != "Write" ]]; then
    exit 0
fi

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()

# Block test files in /tmp or similar
if [[ "$FILE_PATH" =~ ^/tmp/ ]] || [[ "$FILE_PATH" =~ ^/var/tmp/ ]]; then
    if [[ "$FILE_PATH" =~ \.(go|py|sh)$ ]]; then
        ERRORS+=("Throwaway test file in $FILE_PATH")
        ERRORS+=("→ Add to test/ for functional tests")
        ERRORS+=("→ Add to internal/*/...test.go for unit tests")
    fi
fi

# Block ad-hoc test files outside proper locations
if [[ "$FILE_PATH" =~ test_.*\.(go|py|sh)$ ]] || [[ "$FILE_PATH" =~ _test_.*\.(go|py|sh)$ ]]; then
    if [[ ! "$FILE_PATH" =~ ^.*/internal/ ]] && [[ ! "$FILE_PATH" =~ ^.*/test/ ]] && [[ ! "$FILE_PATH" =~ ^.*/cmd/ ]]; then
        ERRORS+=("Test file in wrong location: $FILE_PATH")
        ERRORS+=("→ Unit tests go in internal/*/_test.go")
        ERRORS+=("→ Functional tests go in test/")
    fi
fi

# Block creating main.go for testing
if [[ "$FILE_PATH" =~ /main\.go$ ]] && [[ ! "$FILE_PATH" =~ ^.*/cmd/ ]]; then
    ERRORS+=("main.go outside cmd/ directory")
    ERRORS+=("→ Is this a throwaway test? Add proper tests instead")
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}❌ BLOCKED: Throwaway test file${RESET}" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}✗${RESET} $err" >&2
    done
    echo "" >&2
    echo -e "  ${YELLOW}Tests must be permanent, not throwaway${RESET}" >&2
    echo -e "  ${YELLOW}See ai/rules/testing.md${RESET}" >&2
    exit 2
fi

exit 0
