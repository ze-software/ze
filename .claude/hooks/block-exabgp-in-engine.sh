#!/bin/bash
# PreToolUse hook: Block ExaBGP format awareness in engine
# BLOCKING: ExaBGP compat is external only (compatibility.md)

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // .tool_input.new_string // empty')

# Only process Write/Edit for Go files
if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then
    exit 0
fi

if [[ ! "$FILE_PATH" =~ \.go$ ]]; then
    exit 0
fi

# Skip the exabgp package itself - that's where compat code belongs
if [[ "$FILE_PATH" =~ /exabgp/ ]]; then
    exit 0
fi

# Skip cmd/ze/bgp/exabgp.go - CLI wrapper is allowed
if [[ "$FILE_PATH" =~ cmd/ze/bgp/exabgp ]]; then
    exit 0
fi

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()

# Check for ExaBGP format awareness in engine code
EXABGP_PATTERNS=(
    'exabgp.*format'
    'ExaBGP.*JSON'
    'exabgp.*json'
    '"neighbor".*"announce"'  # ExaBGP JSON format
    'exabgp.*compat'
    'ExaBGPCompat'
)

for pattern in "${EXABGP_PATTERNS[@]}"; do
    if echo "$CONTENT" | grep -qiE "$pattern"; then
        ERRORS+=("ExaBGP format awareness in engine code")
        ERRORS+=("→ ExaBGP compat belongs in internal/exabgp/ only")
        ERRORS+=("→ Engine must not know about ExaBGP formats")
        break
    fi
done

# Check for importing exabgp package in wrong places
if echo "$CONTENT" | grep -qE '".*internal/exabgp"'; then
    # Only allowed in cmd/ze/bgp/ and internal/exabgp itself
    if [[ ! "$FILE_PATH" =~ cmd/ze/ ]]; then
        ERRORS+=("Importing internal/exabgp in engine code")
        ERRORS+=("→ Only cmd/ze/bgp/ may import internal/exabgp")
    fi
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}❌ BLOCKED: ExaBGP in engine${RESET}" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}✗${RESET} $err" >&2
    done
    echo "" >&2
    echo -e "  ${YELLOW}ExaBGP compatibility is external tools only${RESET}" >&2
    echo -e "  ${YELLOW}See .claude/rules/compatibility.md${RESET}" >&2
    exit 2
fi

exit 0
