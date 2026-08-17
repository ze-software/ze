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
# The consumer also touch -c's this marker, so its mtime dates the delegation
# rather than the claim. Nothing deletes it: no reaper survives under
# tmp/session/ (owner decision 2026-08-03), so the marker outlives the session
# and the mtime is the only thing that says when an Agent last ran.
#
# Companion to mark-lsp-invoked.sh / mark-source-read.sh, same marker convention.
# Marker path: tmp/session/.agent-spawned-<SID>. Content: ISO-8601 timestamp.
#
# This hook fires in the parent process after the spawn. The resolver therefore
# names the supervising session's marker without depending on a subagent
# environment.
#
# Non-blocking: this hook only records; it never rejects a spawn.

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

source .claude/hooks/lib/session-id.sh
SID=$(_session_id)

mkdir -p tmp/session
date -Iseconds > "tmp/session/.agent-spawned-${SID}"

exit 0
