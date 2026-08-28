---
kind: directive
level: MUST
stage:
---
**Every marker is keyed by session ID**, and every native hook consumer MUST use
the resolver in `internal/le/hookruntime/session.go`.

Session-start and subagent-context actions read hook JSON and validate the raw
session string before they publish an ID or derive a state path. An absent ID
and an invalid ID are distinct results. Invalid IDs are rejected, never
rewritten, and dot entries are forbidden.

The hook MUST NOT persist `$ZE_SESSION_ID`. Native session and spec lifecycle
commands resolve the current harness session themselves. `./le hook-check
session-id` locks this behavior.
