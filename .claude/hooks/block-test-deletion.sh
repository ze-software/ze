#!/bin/bash
# PreToolUse hook: Require user approval for test deletion
# BLOCKING: User must explicitly approve any test removal

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()

# Check Edit tool - detect removing tests
if [[ "$TOOL_NAME" == "Edit" ]]; then
    FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
    OLD_STRING=$(echo "$INPUT" | jq -r '.tool_input.old_string // empty')
    NEW_STRING=$(echo "$INPUT" | jq -r '.tool_input.new_string // empty')

    # Check if this is a test file
    IS_TEST=false
    if [[ "$FILE_PATH" =~ _test\.go$ ]]; then
        IS_TEST=true
    elif [[ "$FILE_PATH" =~ \.ci$ ]] && [[ "$FILE_PATH" =~ /test/ ]]; then
        IS_TEST=true
    fi

    if [[ "$IS_TEST" == "true" ]]; then
        TRIMMED_NEW=$(echo "$NEW_STRING" | tr -d '[:space:]')

        # Block emptying file
        if [[ -z "$TRIMMED_NEW" ]] && [[ -n "$OLD_STRING" ]]; then
            ERRORS+=("Attempting to empty test file: $FILE_PATH")
        fi

        # Block removing test functions (func Test*, func Fuzz*, func Benchmark*)
        if echo "$OLD_STRING" | grep -qE '^func (Test|Fuzz|Benchmark)'; then
            if ! echo "$NEW_STRING" | grep -qE '^func (Test|Fuzz|Benchmark)'; then
                ERRORS+=("Attempting to delete test function in: $FILE_PATH")
            fi
        fi

        # Block removing t.Run test cases
        OLD_TRUN_COUNT=$(echo "$OLD_STRING" | grep -cE 't\.Run\(' || true)
        NEW_TRUN_COUNT=$(echo "$NEW_STRING" | grep -cE 't\.Run\(' || true)
        if [[ "$OLD_TRUN_COUNT" -gt "$NEW_TRUN_COUNT" ]]; then
            REMOVED=$((OLD_TRUN_COUNT - NEW_TRUN_COUNT))
            ERRORS+=("Attempting to remove $REMOVED t.Run() test case(s) in: $FILE_PATH")
        fi

        # Block removing table-driven test entries (common patterns)
        # Pattern: {name: "...", or {"...",
        OLD_TABLE_COUNT=$(echo "$OLD_STRING" | grep -cE '\{[[:space:]]*(name|Name)[[:space:]]*:' || true)
        NEW_TABLE_COUNT=$(echo "$NEW_STRING" | grep -cE '\{[[:space:]]*(name|Name)[[:space:]]*:' || true)
        if [[ "$OLD_TABLE_COUNT" -gt "$NEW_TABLE_COUNT" ]]; then
            REMOVED=$((OLD_TABLE_COUNT - NEW_TABLE_COUNT))
            ERRORS+=("Attempting to remove $REMOVED table-driven test case(s) in: $FILE_PATH")
        fi

        # Block removing test assertions
        OLD_ASSERT_COUNT=$(echo "$OLD_STRING" | grep -cE '(t\.(Error|Fatal|Fail)|assert\.|require\.)' || true)
        NEW_ASSERT_COUNT=$(echo "$NEW_STRING" | grep -cE '(t\.(Error|Fatal|Fail)|assert\.|require\.)' || true)
        if [[ "$OLD_ASSERT_COUNT" -gt 0 ]] && [[ "$NEW_ASSERT_COUNT" -eq 0 ]] && [[ -n "$NEW_STRING" ]]; then
            ERRORS+=("Attempting to remove all test assertions in: $FILE_PATH")
        fi

        # Block removing .ci test lines (functional tests)
        if [[ "$FILE_PATH" =~ \.ci$ ]]; then
            # Count non-comment, non-empty lines
            OLD_LINES=$(echo "$OLD_STRING" | grep -cvE '^[[:space:]]*(#|$)' || true)
            NEW_LINES=$(echo "$NEW_STRING" | grep -cvE '^[[:space:]]*(#|$)' || true)
            if [[ "$OLD_LINES" -gt "$NEW_LINES" ]]; then
                REMOVED=$((OLD_LINES - NEW_LINES))
                ERRORS+=("Attempting to remove $REMOVED test line(s) from: $FILE_PATH")
            fi
        fi
    fi
fi

# Check Bash tool - detect rm/git rm on test files
if [[ "$TOOL_NAME" == "Bash" ]]; then
    COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

    # Check for rm commands on test files
    RM_PATTERN='(^|[[:space:]]|&&|\|)(rm|git rm)[[:space:]]'
    if [[ "$COMMAND" =~ $RM_PATTERN ]]; then
        if [[ "$COMMAND" =~ _test\.go ]] || [[ "$COMMAND" =~ \.ci ]]; then
            ERRORS+=("Attempting to delete test file via: $COMMAND")
        fi
        if [[ "$COMMAND" =~ rm.*-r.*test/ ]] || [[ "$COMMAND" =~ rm.*-r.*internal/.*test ]]; then
            ERRORS+=("Attempting recursive deletion in test directory: $COMMAND")
        fi
    fi

    # Check for git checkout discarding test changes
    CHECKOUT_PATTERN='git checkout.*(_test\.go|\.ci)'
    if [[ "$COMMAND" =~ $CHECKOUT_PATTERN ]]; then
        DISCARD_PATTERN='git checkout (--|[.])'
        if [[ "$COMMAND" =~ $DISCARD_PATTERN ]]; then
            ERRORS+=("Attempting to discard test file changes: $COMMAND")
        fi
    fi
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${YELLOW}${BOLD}❓ Test deletion - user approval required${RESET}" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  → $err" >&2
    done
    echo "" >&2
    echo -e "  ${BOLD}Allow this test deletion?${RESET}" >&2
    exit 2
fi

exit 0
