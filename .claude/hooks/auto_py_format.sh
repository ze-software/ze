#!/bin/bash
# PostToolUse hook: Auto-format Python files after Write/Edit operations.
# Runs `ruff format` (in-place) and reports `ruff check` findings as warnings.
# Non-blocking: format always succeeds; lint findings are advisory only.

set -e

INPUT=$(cat)

TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then
    exit 0
fi

if [[ ! "$FILE_PATH" =~ \.py$ ]]; then
    exit 0
fi

if [[ ! -f "$FILE_PATH" ]]; then
    exit 0
fi

# Skip vendored / third-party / scratch trees.
case "$FILE_PATH" in
    */vendor/*|*/third_party/*|*/tmp/*)
        exit 0
        ;;
esac

if ! command -v ruff &> /dev/null; then
    # ruff not installed -- advise once, do not block.
    echo "⚠ ruff not found; run 'make ze-setup' to install the Python formatter" >&2
    exit 0
fi

YELLOW='\033[33m'
DIM='\033[2m'
RESET='\033[0m'

ruff format --quiet "$FILE_PATH" 2>/dev/null || true

LINT_OUTPUT=$(ruff check --quiet "$FILE_PATH" 2>&1) || true
if [[ -n "$LINT_OUTPUT" ]]; then
    ISSUE_COUNT=$(echo "$LINT_OUTPUT" | grep -cE "^[^:]+:[0-9]+:" || true)
    if [[ "$ISSUE_COUNT" -gt 0 ]]; then
        echo -e "${YELLOW}⚠ ruff: ${ISSUE_COUNT} issues${RESET}" >&2
        echo "$LINT_OUTPUT" | head -3 | while read -r line; do
            echo -e "  ${DIM}${line}${RESET}" >&2
        done
    fi
fi

exit 0
