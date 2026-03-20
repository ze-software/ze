#!/bin/bash
# PreToolUse hook: Check documentation drift on git commit
# WARNING (exit 1): Docs claim different counts than the live registry/filesystem
#
# Checks docs/DESIGN.md, docs/comparison.md against:
# - Plugin registry (names, count)
# - Family registry (names, count)
# - Filesystem (.ci test counts, interop scenarios, fuzz targets)
#
# Advisory only -- does not block commits, just warns about stale docs.

INPUT=$(cat)
COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

# Only trigger on git commit commands.
if [[ "$COMMAND" != *"git commit"* ]]; then
    exit 0
fi

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || exit 0

# Check if any doc files or plugin files are staged.
STAGED=$(git diff --cached --name-only 2>/dev/null || true)
if [[ -z "$STAGED" ]]; then
    exit 0
fi

# Run the checker. Timeout after 15s (it compiles + queries registry).
OUTPUT=$(timeout 15 go run scripts/check-doc-drift.go 2>&1) || {
    RC=$?
    if [[ $RC -eq 1 ]]; then
        # Drift detected -- advisory warning.
        echo "$OUTPUT" >&2
        exit 1
    fi
    # Compilation error or timeout -- don't block.
    exit 0
}

exit 0
