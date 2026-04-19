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

# Restrict duplicate check to the SAME PACKAGE (same directory). In Go,
# `type Config struct` in package a vs package b defines two unrelated
# types — flagging them as duplicates produces false positives on every
# generic name (Config, Manager, State, ...). Real redefinitions happen
# within one package, which is the only place Go itself rejects them.
PKG_DIR=$(dirname "$FILE_PATH")
# Normalize to relative path for grep
REL_PKG_DIR="${PKG_DIR#$(pwd)/}"
# If the dir does not exist yet (new package), skip — nothing to collide with.
if [[ ! -d "$REL_PKG_DIR" ]]; then
    exit 0
fi

# Extract type/struct names from content being written
TYPES=$(echo "$CONTENT" | grep -oE 'type[[:space:]]+[A-Z][a-zA-Z0-9]*[[:space:]]+struct' | awk '{print $2}' | head -5)

# Check if types already exist in the SAME package (BLOCKING)
for t in $TYPES; do
    # Exclude files with //go:build tags (build-tag pairs define the same type intentionally).
    EXISTING=$(grep -l "type[[:space:]]\+$t[[:space:]]\+struct" "$REL_PKG_DIR"/*.go 2>/dev/null | grep -v "_test.go" | while read -r f; do head -3 "$f" | grep -q '//go:build' || echo "$f"; done | head -3)
    if [[ -n "$EXISTING" ]]; then
        ERRORS+=("Type '$t' ALREADY EXISTS in this package:")
        while IFS= read -r f; do
            [[ -n "$f" ]] && ERRORS+=("  → $f")
        done <<< "$EXISTING"
        ERRORS+=("Extend existing type or use different name")
    fi
done

# Extract exported function names (not methods)
FUNCS=$(echo "$CONTENT" | grep -oE '^func[[:space:]]+[A-Z][a-zA-Z0-9]*\(' | sed 's/func[[:space:]]*//;s/(//' | head -5)

# Check if functions already exist in the SAME package (BLOCKING)
for fn in $FUNCS; do
    # Exclude files with //go:build tags (build-tag pairs define the same function intentionally).
    EXISTING=$(grep -l "^func[[:space:]]\+$fn[[:space:]]*(" "$REL_PKG_DIR"/*.go 2>/dev/null | grep -v "_test.go" | while read -r f; do head -3 "$f" | grep -q '//go:build' || echo "$f"; done | head -3)
    if [[ -n "$EXISTING" ]]; then
        ERRORS+=("Function '$fn' ALREADY EXISTS in this package:")
        while IFS= read -r file; do
            [[ -n "$file" ]] && ERRORS+=("  → $file")
        done <<< "$EXISTING"
        ERRORS+=("Extend existing function or use different name")
    fi
done

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}❌ Duplicate: ${ERRORS[0]}${RESET}" >&2
    exit 2
fi

exit 0
