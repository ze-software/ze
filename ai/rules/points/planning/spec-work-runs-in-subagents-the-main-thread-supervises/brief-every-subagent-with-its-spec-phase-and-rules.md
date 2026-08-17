---
kind: directive
level: MUST
stage:
---
- **Give every subagent the spec path, its phase, the rules that govern it, the parent session ID, and the exact per-session scratch path.** Name `plan/<spec>.md`, the applicable `ai/rules/` files, and what the report MUST contain.
- **When a delegation API does not run `.claude/hooks/subagent-context.sh`, the main thread MUST put the parent session ID and exact per-session scratch path in the shared task context.** The OMP `task` delegation API uses this fallback.
- **The subagent MUST use the provided scratch path.** When its environment does not contain `CLAUDE_CODE_SESSION_ID`, it MUST set that variable to the parent session ID for shell commands. It MUST NOT resolve a fresh session ID in this case.
- **A subagent cannot ask the user.** The main thread MUST NOT give it work that needs an answer from the user.
- **A subagent CAN resolve symbols.** It uses the LSP tool where its registry carries one, or `gopls` from Bash where it does not (`ai/rules/context-economy.md`).
