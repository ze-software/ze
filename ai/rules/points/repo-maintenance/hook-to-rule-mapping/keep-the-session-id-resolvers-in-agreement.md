---
kind: directive
level: MUST
stage:
---
**Every marker is keyed by session id**, and every consumer MUST use
`.claude/hooks/lib/session_id.py`. Bash hooks call it through
`.claude/hooks/lib/session-id.sh` (`_session_id`). Python callers import
`session_id()` and reuse `_sid_safe()` for direct values.

`session-start.sh` and `subagent-context.sh` pass hook JSON to
`--hook-session-id`. This mode validates the decoded raw string before shell
normalization. It returns status 0 for a safe id, status 1 for an absent field,
and status 2 for malformed JSON or an invalid field. SessionStart has an empty
matcher so startup, resume, clear, compact, and fork events republish an
accepted id through `$CLAUDE_ENV_FILE`. SubagentStart falls back to `_session_id`
only for status 1. It emits its complete context as JSON
`hookSpecificOutput.additionalContext`. Status 2 adds no parent id, path, spec,
or state. For a restricted subagent Bash call, `pretool-bash.py` prefixes the
command with the accepted parent id from the PreToolUse payload.

The hook MUST NOT persist `$ZE_SESSION_ID`. `mk/helper-session.mk` derives it. Unsafe
ids are rejected rather than rewritten. The validator rejects dot entries.
`make ze-unit-hook-test` (section `session-id`) locks this behavior.
