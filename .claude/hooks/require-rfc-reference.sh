#!/bin/bash
# PostToolUse hook: Require RFC references in BGP code
# WARNING: BGP protocol code should have RFC comments (rfc-compliance.md)

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

# Only check BGP-related paths
if [[ ! "$FILE_PATH" =~ /bgp/ ]] && [[ ! "$FILE_PATH" =~ /nlri/ ]] && [[ ! "$FILE_PATH" =~ /attribute/ ]] && [[ ! "$FILE_PATH" =~ /capability/ ]]; then
    exit 0
fi

# Skip test files
if [[ "$FILE_PATH" =~ _test\.go$ ]]; then
    exit 0
fi

# Check if file exists
if [[ ! -f "$FILE_PATH" ]]; then
    exit 0
fi

YELLOW='\033[33m'
RESET='\033[0m'

# Check for RFC references in file
RFC_REFS=$(grep -cE '// RFC [0-9]+|// rfc[0-9]+|RFC [0-9]+ Section' "$FILE_PATH" 2>/dev/null || echo "0")

if [[ "$RFC_REFS" -eq 0 ]]; then
    echo -e "${YELLOW}⚠ Add RFC comments to $(basename "$FILE_PATH")${RESET}" >&2
fi

exit 0
