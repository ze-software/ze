#!/bin/bash
# PreToolUse hook: Enforce cross-reference comments per related-refs.md
# Blocking (exit 2) — prevents writing .go files without back-references when siblings reference them
# Also blocks stale refs (pointing to non-existent files)
# Keywords: // Detail: (hub→leaf), // Overview: (leaf→hub), // Related: (peer↔peer)
# See ai/rules/related-refs.md

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

# --- Gather post-op content to check ---
# For Write: tool_input.content is the full new file.
# For Edit: simulate post-edit state by applying old_string→new_string to disk.
# Bash ${var//pat/repl} treats pat as a glob, so any old_string containing *, ?,
# or [ would break. Python does literal replacement; it also supports replace_all.

CONTENT=$(echo "$INPUT" | python3 -c '
import json, sys
data = json.load(sys.stdin)
tool = data.get("tool_name", "")
ti = data.get("tool_input", {})
if tool == "Write":
    sys.stdout.write(ti.get("content", ""))
    sys.exit(0)
if tool == "Edit":
    path = ti.get("file_path", "")
    try:
        with open(path, "r") as f:
            content = f.read()
    except OSError:
        content = ""
    old = ti.get("old_string", "")
    new = ti.get("new_string", "")
    if ti.get("replace_all", False):
        content = content.replace(old, new)
    else:
        content = content.replace(old, new, 1)
    sys.stdout.write(content)
')

# --- Check 1: Siblings reference this file → must have a back-reference ---

# Find siblings that reference this file with any of the three keywords
SIBLINGS_REF_ME=$(grep -rlE "// (Detail|Overview|Related): ${BASE} " "$DIR"/*.go 2>/dev/null | grep -v "_test\.go" | grep -v "_gen\.go" || true)

if [[ -n "$SIBLINGS_REF_ME" ]]; then
    if ! echo "$CONTENT" | grep -qE "$XREF_PATTERN"; then
        echo -e "${RED}${BOLD}✘ BLOCKED: Missing cross-reference comment${RESET}" >&2
        echo "" >&2
        echo -e "  ${RED}!${RESET} File: $BASE" >&2
        echo -e "  ${RED}→${RESET} Referenced by:" >&2
        while IFS= read -r sib; do
            [[ -n "$sib" ]] && echo -e "  ${RED}→${RESET}   $(basename "$sib")" >&2
        done <<< "$SIBLINGS_REF_ME"
        echo -e "  ${RED}→${RESET} Add back-reference: // Overview: / // Detail: / // Related:" >&2
        echo -e "  ${RED}→${RESET} See ai/rules/related-refs.md" >&2
        exit 2
    fi
fi

# --- Check 2: Stale refs — cross-ref points to non-existent file ---

STALE=()
while IFS= read -r ref_file; do
    [[ -z "$ref_file" ]] && continue
    if [[ ! -f "$DIR/$ref_file" ]]; then
        STALE+=("$ref_file")
    fi
done < <(echo "$CONTENT" | grep -E '// (Detail|Overview|Related):' | sed -E 's#.*// (Detail|Overview|Related): ([^ ]*\.go).*#\2#' 2>/dev/null || true)

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
