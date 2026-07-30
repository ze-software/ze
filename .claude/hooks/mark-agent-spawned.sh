#!/bin/bash
# PostToolUse hook on Agent|Task: record that this session delegated at least
# once (ai/rules/spec-delegation.md).
#
# NOTE: the marker currently has NO live consumer. It was written for
# block-premature-stop.sh, which has been registered on no event since
# 41e5fa44f (2026-06-29) and therefore never reads it. Check the Stop array in
# .claude/settings.json before you cite that hook. The marker stays because it
# is cheap and because a future Stop gate would want exactly this signal.
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
