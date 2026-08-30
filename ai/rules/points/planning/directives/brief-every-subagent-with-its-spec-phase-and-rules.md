---
kind: directive
level: MUST
stage:
---
**Every subagent brief MUST carry the spec path, the phase it owns, the `ai/rules/` files that govern it, the parent session ID, the exact per-session scratch path, and what the report has to contain; a brief whose agent will write Go MUST name `docs/contributing/ze-go-style.md` as a PRECONDITION in its OPENING, never under a closing heading, where it is read after the code already exists.** A subagent that finds no `CLAUDE_CODE_SESSION_ID` sets it to the parent's rather than resolving a fresh one, and it resolves symbols with `gopls` from Bash when its registry carries no LSP tool.
