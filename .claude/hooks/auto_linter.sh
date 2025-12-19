#!/bin/bash
# PostToolUse hook: Auto-lint Go files after Write/Edit operations.
# Advisory mode: shows warnings but doesn't block.

set -e

# Read JSON input from stdin
INPUT=$(cat)

# Extract tool name and file path using jq
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# Only process Write/Edit tools
if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then
    exit 0
fi

# Only process .go files
if [[ ! "$FILE_PATH" =~ \.go$ ]]; then
    exit 0
fi

# Check if file exists
if [[ ! -f "$FILE_PATH" ]]; then
    exit 0
fi

# Find project root (has go.mod)
PROJECT_ROOT=$(dirname "$FILE_PATH")
while [[ "$PROJECT_ROOT" != "/" ]]; do
    if [[ -f "$PROJECT_ROOT/go.mod" ]]; then
        break
    fi
    PROJECT_ROOT=$(dirname "$PROJECT_ROOT")
done

if [[ ! -f "$PROJECT_ROOT/go.mod" ]]; then
    # No go.mod found, skip
    exit 0
fi

cd "$PROJECT_ROOT"

# ANSI color codes
YELLOW='\033[33m'
CYAN='\033[36m'
RED='\033[31m'
GREEN='\033[32m'
BOLD='\033[1m'
DIM='\033[2m'
RESET='\033[0m'

# Run gofmt (auto-fix)
if command -v gofmt &> /dev/null; then
    gofmt -w "$FILE_PATH" 2>/dev/null || true
fi

# Run goimports if available (auto-fix)
if command -v goimports &> /dev/null; then
    goimports -w "$FILE_PATH" 2>/dev/null || true
fi

# Run golangci-lint if available
if command -v golangci-lint &> /dev/null; then
    # Get relative path for cleaner output
    REL_PATH="${FILE_PATH#$PROJECT_ROOT/}"

    # Run linter on the specific file
    OUTPUT=$(golangci-lint run --new-from-rev=HEAD --timeout=30s "$REL_PATH" 2>&1) || true

    if [[ -n "$OUTPUT" && ! "$OUTPUT" =~ "no issues" ]]; then
        # Count issues
        ISSUE_COUNT=$(echo "$OUTPUT" | grep -c ":" || echo "0")

        echo -e "${YELLOW}⚠️  golangci-lint found ${BOLD}${ISSUE_COUNT}${RESET}${YELLOW} issue(s) in ${CYAN}$(basename "$FILE_PATH")${RESET}${YELLOW}:${RESET}" >&2

        # Show first 5 issues
        echo "$OUTPUT" | head -5 | while read -r line; do
            echo -e "   ${DIM}${line}${RESET}" >&2
        done

        TOTAL=$(echo "$OUTPUT" | wc -l)
        if [[ $TOTAL -gt 5 ]]; then
            echo -e "   ${DIM}... and $((TOTAL - 5)) more${RESET}" >&2
        fi

        echo -e "   ${GREEN}Run: golangci-lint run ${REL_PATH}${RESET}" >&2

        # Non-blocking exit (advisory)
        exit 1
    fi
fi

exit 0
