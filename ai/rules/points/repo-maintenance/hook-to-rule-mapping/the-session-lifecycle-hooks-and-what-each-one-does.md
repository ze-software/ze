---
kind: table
level:
stage:
---
| Native kind | Event | What it does |
|---|---|---|
| `session-start` | SessionStart | Validates the raw session ID, publishes the accepted ID, and prints status. Deletes nothing; `./le session reap` owns proof-based cleanup. |
| `compaction-reminder` | UserPromptSubmit | Detects compaction and reminds the session to read the post-compaction rule. |
| `verify-claim-reminder` | UserPromptSubmit | Reminds the session to read the producing function before making a code claim. |
| `delegation-reminder` | UserPromptSubmit | States that requested parallel delegation needs no permission. |
| `block-premature-stop` | Stop | Runs the native stop-phrase and spec-closure checks. Blocking. |
| `session-end-summary` | Stop | Calls `./le session end-summary`; preserves handoffs and never releases a spec claim. |
| `session-end-deferrals` | Stop | Prints the open deferral count. Advisory. |
| `pre-compact-save` | PreCompact | Saves session state before compaction. |
| `subagent-context` | SubagentStart | Validates the parent session ID and emits the parent context through the hook protocol. |
