#!/bin/bash
# PreToolUse hook: Require test file before implementation
# BLOCKING: TDD - test file must exist before editing impl (tdd.md)

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# Only process Write/Edit for Go files
if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then
    exit 0
fi

if [[ ! "$FILE_PATH" =~ \.go$ ]]; then
    exit 0
fi

# Skip test files themselves
if [[ "$FILE_PATH" =~ _test\.go$ ]]; then
    exit 0
fi

# Skip generated files
if [[ "$FILE_PATH" =~ _gen\.go$ ]] || [[ "$FILE_PATH" =~ \.pb\.go$ ]]; then
    exit 0
fi

# Skip main packages (cmd/)
if [[ "$FILE_PATH" =~ /cmd/ ]]; then
    exit 0
fi

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

# Calculate expected test file path
TEST_FILE="${FILE_PATH%.go}_test.go"

# For new files being created, test should exist first
if [[ "$TOOL_NAME" == "Write" && ! -f "$FILE_PATH" ]]; then
    if [[ ! -f "$TEST_FILE" ]]; then
        echo -e "${RED}${BOLD}❌ BLOCKED: TDD - Write test first${RESET}" >&2
        echo "" >&2
        echo -e "  ${RED}✗${RESET} Creating new file without test" >&2
        echo -e "  ${RED}✗${RESET} Expected test: $TEST_FILE" >&2
        echo "" >&2
        echo -e "  ${YELLOW}TDD requires: test FAIL → implement → test PASS${RESET}" >&2
        echo -e "  ${YELLOW}See ai/rules/tdd.md${RESET}" >&2
        exit 1
    fi
fi

# For existing files, warn if no test (but don't block)
if [[ -f "$FILE_PATH" && ! -f "$TEST_FILE" ]]; then
    echo -e "${YELLOW}⚠️  No test file found: $TEST_FILE${RESET}" >&2
    echo -e "  ${YELLOW}Consider adding tests for this file${RESET}" >&2
fi

exit 0
