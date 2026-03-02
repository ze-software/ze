#!/bin/bash
# PreToolUse hook: Enforce cross-reference comments per related-refs.md
# Blocking (exit 2) — prevents writing .go files without back-references when siblings reference them
# Also blocks stale refs (pointing to non-existent files)
# Keywords: // Detail: (hub→leaf), // Overview: (leaf→hub), // Related: (peer↔peer)
# See .claude/rules/related-refs.md

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

# Skip exempt files per related-refs.md
BASE=$(basename "$FILE_PATH")
if [[ "$BASE" =~ _test\.go$ ]] || \
   [[ "$BASE" =~ _gen\.go$ ]] || \
   [[ "$BASE" == "register.go" ]] || \
   [[ "$BASE" == "embed.go" ]] || \
   [[ "$BASE" == "doc.go" ]]; then
    exit 0
fi

DIR=$(dirname "$FILE_PATH")

RED='\033[31m'
BOLD='\033[1m'
RESET='\033[0m'

# Cross-reference pattern: matches // Detail:, // Overview:, // Related:
XREF_PATTERN='// (Detail|Overview|Related):'

# --- Gather content to check ---

CONTENT=""
if [[ "$TOOL_NAME" == "Write" ]]; then
    CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // empty')
elif [[ "$TOOL_NAME" == "Edit" ]]; then
    # For Edit: use file on disk (pre-edit state)
    if [[ -f "$FILE_PATH" ]]; then
        CONTENT=$(cat "$FILE_PATH")
    fi
    # Also check if the edit is adding refs
    NEW_STRING=$(echo "$INPUT" | jq -r '.tool_input.new_string // empty')
fi

# --- Check 1: Siblings reference this file → must have a back-reference ---

# Find siblings that reference this file with any of the three keywords
SIBLINGS_REF_ME=$(grep -rlE "// (Detail|Overview|Related): ${BASE} " "$DIR"/*.go 2>/dev/null | grep -v "_test\.go" | grep -v "_gen\.go" || true)

if [[ -n "$SIBLINGS_REF_ME" ]]; then
    HAS_XREF=false

    if echo "$CONTENT" | grep -qE "$XREF_PATTERN"; then
        HAS_XREF=true
    fi

    # For Edit: also check if new_string adds it
    if [[ "$TOOL_NAME" == "Edit" && "$HAS_XREF" == "false" ]]; then
        if echo "$NEW_STRING" | grep -qE "$XREF_PATTERN"; then
            HAS_XREF=true
        fi
    fi

    if [[ "$HAS_XREF" == "false" ]]; then
        echo -e "${RED}${BOLD}✘ BLOCKED: Missing cross-reference comment${RESET}" >&2
        echo "" >&2
        echo -e "  ${RED}!${RESET} File: $BASE" >&2
        echo -e "  ${RED}→${RESET} Referenced by:" >&2
        while IFS= read -r sib; do
            [[ -n "$sib" ]] && echo -e "  ${RED}→${RESET}   $(basename "$sib")" >&2
        done <<< "$SIBLINGS_REF_ME"
        echo -e "  ${RED}→${RESET} Add back-reference: // Overview: / // Detail: / // Related:" >&2
        echo -e "  ${RED}→${RESET} See .claude/rules/related-refs.md" >&2
        exit 2
    fi
fi

# --- Check 2: Stale refs — cross-ref points to non-existent file ---

# Extract referenced filenames from content (Write) or file+new_string (Edit)
CHECK_CONTENT="$CONTENT"
if [[ "$TOOL_NAME" == "Edit" && -n "$NEW_STRING" ]]; then
    CHECK_CONTENT="$CONTENT
$NEW_STRING"
fi

STALE=()
while IFS= read -r ref_file; do
    [[ -z "$ref_file" ]] && continue
    if [[ ! -f "$DIR/$ref_file" ]]; then
        STALE+=("$ref_file")
    fi
done < <(echo "$CHECK_CONTENT" | grep -E '// (Detail|Overview|Related):' | sed -E 's#.*// (Detail|Overview|Related): ([^ ]*\.go).*#\2#' 2>/dev/null || true)

if [[ ${#STALE[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}✘ BLOCKED: Stale cross-references${RESET}" >&2
    echo "" >&2
    echo -e "  ${RED}!${RESET} File: $BASE" >&2
    for s in "${STALE[@]}"; do
        echo -e "  ${RED}→${RESET} $s does not exist in $(basename "$DIR")/" >&2
    done
    echo -e "  ${RED}→${RESET} Remove or update stale entries" >&2
    exit 2
fi

exit 0
