# Post-Compaction Recovery

**BLOCKING:** Complete before writing any code after compaction.
Rationale: `.claude/rationale/post-compaction.md`

## Tier 1 — Always

```
[ ] 1. Read .claude/selected-spec → read spec if set
[ ] 2. Read .claude/session-state.md (if exists)
[ ] 3. Check git status
```

## Tier 2 — File digests available

Use session-state.md digests. Only re-read full file when digest lacks needed detail.
Use spec `→ Decision:` / `→ Constraint:` annotations first.

## Tier 3 — No digests

Re-read source files from Current Behavior + architecture docs. Write digests immediately.

## Digest Format

```
- `reactor/peer.go` (380L): Peer struct, FSM transitions. Key: Run(), handleOpen(). Uses wire.SessionBuffer.
```

`[x]` in specs = read in previous session. Re-read only what's needed.
Until Tier 1 complete: no code, no design decisions, no claiming "done".
