---
kind: directive
level: MUST
stage:
---
- **Every session marker is keyed by session ID, and every native hook consumer MUST resolve that ID through `internal/le/hookruntime/session.go`.** An absent ID and an invalid ID are distinct results: an invalid ID MUST be rejected rather than rewritten, and a dot entry MUST NOT be accepted.
- **A hook MUST NOT persist `$ZE_SESSION_ID`.** Native session and spec lifecycle commands resolve the current harness session themselves. `./le hook-check session-id` locks this behavior.
