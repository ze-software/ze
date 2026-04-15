# Post-Compaction Recovery

**BLOCKING:** Complete before writing any code after compaction.
Rationale: `.claude/rationale/post-compaction.md`

## Tier 1 — Always

```
[ ] 1. Read tmp/session/selected-spec → read spec if set
[ ] 2. Read per-spec session state (tmp/session/session-state-<spec-stem>-<SID>.md) if exists
[ ] 3. Check git status
```

## Tier 2 — File digests available

Use per-spec session state digests. Only re-read full file when digest lacks needed detail.
Use spec `→ Decision:` / `→ Constraint:` annotations first.

## Tier 3 — No digests

Re-read source files from Current Behavior + architecture docs. Write digests immediately.

## Digest Format

```
- `internal/component/bgp/reactor/peer.go` (380L): Peer struct, FSM transitions. Key: Run(), handleOpen(). Uses wire.SessionBuffer.
```

Spec checkboxes are always `[ ]` -- never tick them. Use `-> Decision:` / `-> Constraint:` annotations and per-spec session state digests to recover knowledge after compaction. If a reading entry has no annotation, re-read the file.
Until Tier 1 complete: no code, no design decisions, no claiming "done".
