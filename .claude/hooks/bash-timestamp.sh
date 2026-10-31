#!/bin/sh
# PreToolUse:Bash: print "▶ HH:MM:SS", but only when this session has not
# printed one in the last QUIET seconds.
QUIET=${CLAUDE_BASH_STAMP_QUIET:-60}

sid=$(jq -r '.session_id // "default"' 2>/dev/null)
stamp="${TMPDIR:-/tmp}/claude-bash-stamp-${sid:-default}"

now=$(date +%s)
last=$(cat "$stamp" 2>/dev/null || echo 0)
[ $((now - last)) -lt "$QUIET" ] && exit 0

echo "$now" > "$stamp"
jq -nc --arg t "$(date +%T)" '{systemMessage:("▶ "+$t)}'
