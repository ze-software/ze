# Post-Compaction Recovery

Rationale: `.claude/rationale/post-compaction.md`

**BLOCKING:** Complete before writing any code after compaction.

## Tier 1 — Essential (always)

```
[ ] 1. Read .claude/selected-spec → read spec if set
[ ] 2. Read .claude/session-state.md (if exists)
[ ] 3. Check git status for modified files
```

## Tier 2 — Use file digests (preferred)

If session-state.md has File Digests: use those. Only re-read full file when digest lacks needed detail.
If spec has `→ Decision:` / `→ Constraint:` lines: use those annotations first.

## Tier 3 — No digests (fallback)

Re-read source files from Current Behavior + architecture docs. Write digests immediately.

## File Digest Format

```
- `reactor/peer.go` (380L): Peer struct, FSM transitions. Key: Run(), handleOpen(). Uses wire.SessionBuffer.
```

Checkboxes `[x]` in specs = read in previous session. Re-read only what's needed for current task.
Until Tier 1 complete: no code, no design decisions, no claiming "done".
