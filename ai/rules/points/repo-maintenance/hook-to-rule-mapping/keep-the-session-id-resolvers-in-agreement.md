---
kind: directive
level: MUST
stage:
---
**Every marker is keyed by session id**, and the id is resolved in TWO places that MUST agree: `.claude/hooks/lib/session-id.sh` (`_session_id`, used by the shell hooks that WRITE `.lsp-loaded-*` / `.lsp-invoked-*` / `.source-read-*` / `.session-*`) and a port inside `pretool-writeedit.py` (`session_id()`, which READS them). Disagreement fails CLOSED: the reader looks for a file nothing wrote and blocks work that was actually done. Both read `$CLAUDE_CODE_SESSION_ID` first; an id that is not a safe filename component is rejected by both rather than rewritten. `make ze-hook-test` (section `session-id`) locks this. Before 2026-07-16 neither end had an env lookup, so with no `--session-id` in argv and no access token every concurrent session shared ONE marker set, and `spec-session.sh claim` then silently overwrote another session's spec claim. If you touch either resolver, change BOTH and re-run the test.
