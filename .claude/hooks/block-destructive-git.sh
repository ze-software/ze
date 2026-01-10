#!/bin/bash
# BLOCKING HOOK: Prevents destructive git commands
# This hook is called by Claude Code before any Bash command
# Exit code 2 = BLOCK the command

COMMAND="$CLAUDE_TOOL_INPUT_command"

# List of destructive git command patterns
DESTRUCTIVE_PATTERNS=(
    "git reset"
    "git checkout --"
    "git checkout -f"
    "git checkout HEAD"
    "git restore"
    "git revert"
    "git stash drop"
    "git stash clear"
    "git clean"
    "git push --force"
    "git push -f"
)

for pattern in "${DESTRUCTIVE_PATTERNS[@]}"; do
    if [[ "$COMMAND" == *"$pattern"* ]]; then
        echo "BLOCKED: Destructive git command detected: $pattern"
        echo "This command would discard work and is permanently blocked."
        echo "To use this command, run it manually in your terminal."
        exit 2
    fi
done

exit 0
