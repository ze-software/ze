#!/bin/bash
# BLOCKING HOOK: Enforce tmp files go to project tmp/, block /tmp
# Rule: testing.md — Use project tmp/ for scratch files, never /tmp
# Exit code 2 = BLOCK the operation

INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

# --- Write/Edit: check file_path ---
if [[ -n "$FILE_PATH" ]]; then
    # Block writes to system /tmp
    if [[ "$FILE_PATH" == /tmp/* || "$FILE_PATH" == /tmp ]]; then
        echo "❌ Blocked: writing to /tmp is forbidden" >&2
        echo "Use project tmp/ instead: tmp/<subfolder>/<file>" >&2
        exit 2
    fi
fi

# --- Bash: check command for absolute /tmp references ---
# Only match /tmp when it starts a path token -- preceded by start-of-string,
# whitespace, or a common shell arg delimiter (= ' " $ ( ` : ,). This avoids
# false positives on legitimate paths like test/tmp/ or ~/tmp/.
if [[ -n "$COMMAND" ]]; then
    if [[ "$COMMAND" =~ (^|[[:space:]=\'\"\$\(\`:,])/tmp(/|[[:space:]]|$) ]]; then
        echo "❌ Blocked: /tmp access is forbidden" >&2
        echo "Use project tmp/ instead: tmp/<subfolder>/" >&2
        exit 2
    fi
fi

exit 0
