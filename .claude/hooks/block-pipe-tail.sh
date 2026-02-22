#!/bin/bash
# BLOCKING HOOK: Prevents piping through tail
# The testing.md rule says: Never | tail
# Use Read tool or redirect to file + grep for failures instead
# Exit code 2 = BLOCK the command

COMMAND="$CLAUDE_TOOL_INPUT_command"

if [[ "$COMMAND" == *"| tail"* ]]; then
    echo "❌ Blocked: '| tail' — captures output to file instead, or use Read tool" >&2
    exit 2
fi

exit 0
