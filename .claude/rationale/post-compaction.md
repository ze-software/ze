# Post-Compaction Rationale

Why: `.claude/rules/post-compaction.md`

## Why "Checkboxes Are Stale"
A `[x]` in a spec means you read it in a previous session. You don't remember the content. Re-read files you need for your current task, not "ALL files ever listed."

## Why Tiered Recovery
- Tier 1 (essential): spec + session-state + git status -- always needed
- Tier 2 (file digests): compact summaries replace re-reading 10+ full files with ~20 lines
- Tier 3 (fallback): only when no digests available -- re-read source + write digests immediately

## Rules Until Tier 1 Complete
- Do not write any code
- Do not make design decisions
- Do not claim anything is "done"
- After recovery: update per-spec session state with current progress, key decisions, and file digests

## File Digest Format
```
## File Digests
- `reactor/peer.go` (380L): Peer struct, FSM state transitions. Key: Run(), handleOpen(), handleUpdate(). Uses wire.SessionBuffer.
- `reactor/session.go` (420L): Session struct, read/write loops. Key: handleRead(), handleWrite(), Close().
```
Each digest: file path, line count, key types/functions, 1-2 sentences of what it does.
