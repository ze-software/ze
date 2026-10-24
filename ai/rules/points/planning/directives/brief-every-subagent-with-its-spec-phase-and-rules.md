---
kind: directive
level: MUST
stage:
---
**Every subagent brief MUST carry the spec path, the phase it owns, the `ai/rules/` files that govern it, the parent session ID, the exact per-session scratch path, and what the report has to contain; a brief whose agent will write Go MUST name `docs/contributing/ze-go-style.md` as a precondition in its opening, never under a closing heading, where it is read after the code already exists.** A subagent that finds no `CLAUDE_CODE_SESSION_ID` sets it to the parent's rather than resolving a fresh one, so it shares the parent's session directory and scratch, while the `ZE_AGENT_ID` its hook exports gives it a commit namespace and a spec claim of its own (`docs/contributing/running-commands.md`, "Session identity, and what each agent owns"). It resolves symbols with `gopls` from Bash when its registry carries no LSP tool.
