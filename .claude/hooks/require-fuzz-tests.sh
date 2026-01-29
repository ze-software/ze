#!/bin/bash
# PostToolUse hook: Require fuzz tests for wire format parsing
# WARNING: Wire format parsing MUST have fuzz tests (tdd.md)

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

# Skip test files
if [[ "$FILE_PATH" =~ _test\.go$ ]]; then
    exit 0
fi

# Only check wire format related paths
if [[ ! "$FILE_PATH" =~ /message/ ]] && [[ ! "$FILE_PATH" =~ /nlri/ ]] && [[ ! "$FILE_PATH" =~ /attribute/ ]] && [[ ! "$FILE_PATH" =~ /capability/ ]]; then
    exit 0
fi

# Check if file exists
if [[ ! -f "$FILE_PATH" ]]; then
    exit 0
fi

YELLOW='\033[33m'
RESET='\033[0m'

# Check for Parse functions (wire format parsing)
HAS_PARSE=$(grep -cE '^func Parse[A-Z]|^func \([^)]+\) Parse' "$FILE_PATH" 2>/dev/null || echo "0")

if [[ "$HAS_PARSE" -gt 0 ]]; then
    # Check for corresponding fuzz test
    TEST_FILE="${FILE_PATH%.go}_test.go"
    DIR=$(dirname "$FILE_PATH")

    HAS_FUZZ=0
    if [[ -f "$TEST_FILE" ]]; then
        HAS_FUZZ=$(grep -cE '^func Fuzz[A-Z]' "$TEST_FILE" 2>/dev/null || echo "0")
    fi

    # Also check for fuzz tests in any test file in same directory
    if [[ "$HAS_FUZZ" -eq 0 ]]; then
        HAS_FUZZ=$(grep -rlE '^func Fuzz[A-Z]' "$DIR" 2>/dev/null | grep -c '_test.go' || echo "0")
    fi

    if [[ "$HAS_FUZZ" -eq 0 ]]; then
        echo -e "${YELLOW}⚠️  Wire format parsing without fuzz tests: $(basename "$FILE_PATH")${RESET}" >&2
        echo -e "  ${YELLOW}→ Found Parse* functions but no Fuzz* tests${RESET}" >&2
        echo -e "  ${YELLOW}→ Add: func FuzzParseName(f *testing.F) { ... }${RESET}" >&2
        echo -e "  ${YELLOW}→ See .claude/rules/tdd.md (Fuzzing section)${RESET}" >&2
    fi
fi

exit 0
