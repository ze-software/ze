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
- **A brief whose agent will write Go MUST name `docs/contributing/ze-go-style.md` as a PRECONDITION, in the brief's opening, and MUST NOT file it under a closing heading.** The owner directive is that the guide is read in full before any code, and a subagent inherits the session-start checklist through no mechanism you can verify from the main thread, so the brief is the only place the requirement reliably reaches it.
- **A precondition MUST be written where it is read BEFORE the work, never in a "before you finish" or "when done" list.** Measured 2026-08-19: three fix agents were briefed with "Read `docs/contributing/ze-go-style.md` before writing Go" under a heading reading "Before you finish", which reads as a closing checklist item and arrives after the code exists. The instruction was present and still bought nothing.
- **The brief MUST require the agent to REPORT whether it read the guide before writing, and MUST say that "no" carries no penalty.** The main thread cannot check: subagent transcripts live under `/tmp`, which `check_system_tmp` (`.claude/hooks/pretool-bash.py`) refuses, so a reported fact is the only evidence available and a brief that invites a reassuring answer gets one.
