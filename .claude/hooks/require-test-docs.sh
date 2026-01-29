#!/bin/bash
# PostToolUse hook: Require test documentation
# WARNING: Tests should have VALIDATES/PREVENTS comments (tdd.md)

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# Only process Write/Edit for test files
if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then
    exit 0
fi

if [[ ! "$FILE_PATH" =~ _test\.go$ ]]; then
    exit 0
fi

# Check if file exists
if [[ ! -f "$FILE_PATH" ]]; then
    exit 0
fi

YELLOW='\033[33m'
RESET='\033[0m'

# Check for test documentation
HAS_VALIDATES=$(grep -c 'VALIDATES:' "$FILE_PATH" 2>/dev/null || echo "0")
HAS_PREVENTS=$(grep -c 'PREVENTS:' "$FILE_PATH" 2>/dev/null || echo "0")

# Count test functions
TEST_COUNT=$(grep -cE '^func Test[A-Z]' "$FILE_PATH" 2>/dev/null || echo "0")

if [[ "$TEST_COUNT" -gt 0 ]]; then
    if [[ "$HAS_VALIDATES" -eq 0 ]] && [[ "$HAS_PREVENTS" -eq 0 ]]; then
        echo -e "${YELLOW}⚠️  Test file without documentation: $(basename "$FILE_PATH")${RESET}" >&2
        echo -e "  ${YELLOW}→ Add VALIDATES: and PREVENTS: comments to tests${RESET}" >&2
        echo -e "  ${YELLOW}→ Format: // VALIDATES: [what correct behavior looks like]${RESET}" >&2
        echo -e "  ${YELLOW}→ Format: // PREVENTS: [what bug this catches]${RESET}" >&2
        echo -e "  ${YELLOW}→ See .claude/rules/tdd.md${RESET}" >&2
    fi
fi

exit 0
