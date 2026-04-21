#!/bin/bash
# PostToolUse hook: Suggest // RFC: header when BGP code references RFCs
# Non-blocking (exit 0) — advisory only, since not all files need RFC refs
# See ai/rules/design-doc-references.md

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

# Skip test files, generated files, exempt files
BASE=$(basename "$FILE_PATH")
if [[ "$BASE" =~ _test\.go$ ]] || \
   [[ "$BASE" =~ _gen\.go$ ]] || \
   [[ "$BASE" == "register.go" ]] || \
   [[ "$BASE" == "embed.go" ]] || \
   [[ "$BASE" == "doc.go" ]]; then
    exit 0
fi

# Check if file exists
if [[ ! -f "$FILE_PATH" ]]; then
    exit 0
fi

# Already has // RFC: header — nothing to suggest
if head -10 "$FILE_PATH" | grep -q '// RFC:'; then
    exit 0
fi

# Check if file body references RFCs (suggesting it implements RFC behavior)
BODY_RFCS=$(grep -cE 'RFC [0-9]{4}|rfc[0-9]{4}' "$FILE_PATH" 2>/dev/null || echo "0")

if [[ "$BODY_RFCS" -ge 2 ]]; then
    YELLOW='\033[33m'
    RESET='\033[0m'
    echo -e "${YELLOW}⚠ $(basename "$FILE_PATH") references RFCs but has no // RFC: rfc/short/rfcNNNN.md header${RESET}" >&2
fi

exit 0
