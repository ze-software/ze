#!/bin/bash
# PostToolUse hook: Auto-lint Go files after Write/Edit operations.
# BLOCKING: Rejects changes with lint issues.

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

# Run goimports if available (auto-fix).
# -local matches golangci-lint's goimports.local-prefixes setting
# to maintain consistent 3-group import ordering (stdlib / third-party / local).
if command -v goimports &> /dev/null; then
    goimports -local codeberg.org/thomas-mangin/ze -w "$FILE_PATH" 2>/dev/null || true
fi

# Run golangci-lint if available
if command -v golangci-lint &> /dev/null; then
    # Get relative path for cleaner output
    REL_PATH="${FILE_PATH#$PROJECT_ROOT/}"

    # Run linter on the package containing the file (not just the file).
    # File-level linting causes typecheck failures for multi-file packages
    # because cross-file types (e.g. PeerState in peer.go) appear undefined.
    PACKAGE_DIR=$(dirname "$REL_PATH")
    OUTPUT=$(golangci-lint run --new-from-rev=HEAD --timeout=30s "./${PACKAGE_DIR}/..." 2>&1) || true

    if [[ -n "$OUTPUT" && ! "$OUTPUT" =~ "no issues" && ! "$OUTPUT" =~ "^0 issues" ]]; then
        ISSUE_COUNT=$(echo "$OUTPUT" | grep -c ":")
        if [[ "$ISSUE_COUNT" -eq 0 ]]; then
            exit 0
        fi
        echo -e "${YELLOW}⚠ lint: ${ISSUE_COUNT} issues${RESET}" >&2
        echo "$OUTPUT" | head -3 | while read -r line; do
            echo -e "  ${DIM}${line}${RESET}" >&2
        done
        exit 2
    fi
fi

exit 0
