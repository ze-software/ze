#!/bin/bash
# PostToolUse hook on Agent|Task: record that this session delegated at least
# once (ai/rules/planning.md).
#
# The consumer is block-premature-stop.sh:213-217. It reads this marker at Stop
# and warns (exit 1, never blocks) when the session claimed a spec and never
# delegated. That hook was registered on no event from 41e5fa44f (2026-06-29)
# until Thomas re-registered it on 2026-07-31, so this marker had no reader for
# a month. Check the Stop array in .claude/settings.json before you describe
# either side as live.
#
# The consumer also touch -c's this marker, because two reapers delete it at 24h
# (lib/state-file.sh:92 and the unfiltered find at session-start.sh:22). Without
# that, a session older than a day that delegated yesterday would lose only this
# marker and be told it never delegated.
#
# Companion to mark-lsp-invoked.sh / mark-source-read.sh, same marker convention.
# Marker path: tmp/session/.agent-spawned-<SID>. Content: ISO-8601 timestamp.
#
# The id is the PARENT session's: subagents inherit $CLAUDE_CODE_SESSION_ID
# deliberately (.claude/hooks/lib/session_id.py, "Precedence" 1), and this hook
# fires in the parent anyway, so the marker always lands on the supervising
# session rather than the agent it launched.
#
# Non-blocking: this hook only records; it never rejects a spawn.

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

source .claude/hooks/lib/session-id.sh
SID=$(_session_id)

mkdir -p tmp/session
date -Iseconds > "tmp/session/.agent-spawned-${SID}"

exit 0
