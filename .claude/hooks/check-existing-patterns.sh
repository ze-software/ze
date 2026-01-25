#!/bin/bash
# PreToolUse hook: Block duplicate code/patterns
# BLOCKING: Rejects new files with types/functions that already exist

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // empty')

# Only process Write tool for new Go files
if [[ "$TOOL_NAME" != "Write" ]]; then
    exit 0
fi

# Only new Go files in internal/
if [[ ! "$FILE_PATH" =~ ^.*/internal/.*\.go$ ]] || [[ -f "$FILE_PATH" ]]; then
    exit 0
fi

# Skip test files
if [[ "$FILE_PATH" =~ _test\.go$ ]]; then
    exit 0
fi

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()

# Extract type/struct names from content being written
TYPES=$(echo "$CONTENT" | grep -oE 'type\s+[A-Z][a-zA-Z0-9]*\s+struct' | awk '{print $2}' | head -5)

# Check if types already exist (BLOCKING)
for t in $TYPES; do
    EXISTING=$(grep -rl "type[[:space:]]\+$t[[:space:]]\+struct" internal/ 2>/dev/null | grep -v "_test.go" | head -3)
    if [[ -n "$EXISTING" ]]; then
        ERRORS+=("Type '$t' ALREADY EXISTS:")
        while IFS= read -r f; do
            [[ -n "$f" ]] && ERRORS+=("  → $f")
        done <<< "$EXISTING"
        ERRORS+=("Extend existing type or use different name")
    fi
done

# Extract exported function names (not methods)
FUNCS=$(echo "$CONTENT" | grep -oE '^func\s+[A-Z][a-zA-Z0-9]*\(' | sed 's/func\s*//;s/(//' | head -5)

# Check if functions already exist (BLOCKING)
for fn in $FUNCS; do
    EXISTING=$(grep -rl "^func[[:space:]]\+$fn[[:space:]]*(" internal/ 2>/dev/null | grep -v "_test.go" | head -3)
    if [[ -n "$EXISTING" ]]; then
        ERRORS+=("Function '$fn' ALREADY EXISTS:")
        while IFS= read -r file; do
            [[ -n "$file" ]] && ERRORS+=("  → $file")
        done <<< "$EXISTING"
        ERRORS+=("Extend existing function or use different name")
    fi
done

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}❌ BLOCKED: Duplicate code detected${RESET}" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}✗${RESET} $err" >&2
    done
    echo "" >&2
    echo -e "  ${YELLOW}Read existing code first, then extend it${RESET}" >&2
    echo -e "  ${YELLOW}See: .claude/rules/understand-first.md${RESET}" >&2
    exit 2
fi

exit 0
