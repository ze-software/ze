# Post-Compaction Recovery

**BLOCKING:** After context compaction, complete recovery before writing any code.

## Tiered Recovery

### Tier 1 — Essential (always do)

```
[ ] 1. Read .claude/selected-spec → read the spec if set
[ ] 2. Read .claude/session-state.md (if exists)
[ ] 3. Check git status for modified files
[ ] 4. Read source files you are about to modify
```

### Tier 2 — Spec has checkpoint annotations

If the spec's Required Reading has `→ Decision:` / `→ Constraint:` lines:
- Read those annotations for immediate context
- Full doc re-reads only when annotations are insufficient for the current task

### Tier 3 — Spec lacks checkpoint annotations

- Re-read architecture docs listed in Required Reading
- Re-read source files listed in Current Behavior

## Checkboxes Are Stale

A `[x]` in a spec means you read it in a previous session. You don't remember the content.

Re-read files you need for your **current task** — not "ALL files ever listed."

## Rules

Until Tier 1 recovery is complete:
- Do not write any code
- Do not make design decisions
- Do not claim anything is "done"

After recovery, update `.claude/session-state.md` with current progress and key decisions.
