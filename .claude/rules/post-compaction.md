# Post-Compaction Recovery

**BLOCKING:** After context compaction, complete recovery before writing any code.

## Tiered Recovery

### Tier 1 — Essential (always do)

```
[ ] 1. Read .claude/selected-spec → read the spec if set
[ ] 2. Read .claude/session-state.md (if exists)
[ ] 3. Check git status for modified files
```

**Do NOT re-read full source files yet.** Check session-state's File Digests section first.

### Tier 2 — Use file digests (preferred)

If session-state.md has a **File Digests** section:
- Read the digests (compact summaries of files already read this session)
- Only re-read a full source file when the digest lacks detail needed for the current edit
- This replaces re-reading 10+ full files with reading ~20 lines of summaries

If the spec's Required Reading has `→ Decision:` / `→ Constraint:` lines:
- Read those annotations for immediate context
- Full doc re-reads only when annotations are insufficient for the current task

### Tier 3 — No digests available (fallback)

Only if session-state.md has no File Digests section:
- Re-read source files listed in Current Behavior
- Re-read architecture docs listed in Required Reading
- **Write digests immediately** so the next compaction doesn't repeat this

## Writing File Digests

After reading a source file for the first time in a session, add a digest to session-state.md:

```markdown
## File Digests
- `reactor/peer.go` (380L): Peer struct, FSM state transitions. Key: Run(), handleOpen(), handleUpdate(). Uses wire.SessionBuffer.
- `reactor/session.go` (420L): Session struct, read/write loops. Key: handleRead(), handleWrite(), Close().
```

Each digest: file path, line count, key types/functions, 1-2 sentences of what it does.

## Checkboxes Are Stale

A `[x]` in a spec means you read it in a previous session. You don't remember the content.

Re-read files you need for your **current task** — not "ALL files ever listed."

## Rules

Until Tier 1 recovery is complete:
- Do not write any code
- Do not make design decisions
- Do not claim anything is "done"

After recovery, update `.claude/session-state.md` with current progress, key decisions, and file digests.
