# Planning Rationale

Why: `ai/rules/planning.md`

## Why Spec Selection Is Single-Tracked

Multiple concurrent specs lead to partial implementations, conflicting changes, and context confusion after compaction. One spec at a time ensures focus.

## Why Append-Only Editing

After context compaction, deleted spec content is lost forever. You'll re-investigate solved problems and remake decisions. Strikethrough preserves history while marking superseded content.

## Why Pre-Spec Verification Exists

Historical failure: specs written without reading source code led to invented JSON formats that conflicted with existing output. The checklist prevents designing against imagined behavior.

## Why Single Commit

All changes for a feature belong together. The spec documents what was done; committing it with the code preserves the connection.

## Why Implementation Plan Format

Presenting the plan to the user BEFORE coding catches misunderstandings early. The format ensures all concerns (data flow, existing behavior, tests, design principles) are addressed before writing a line of code.

## Why Failure Routing Table

Without explicit routing, failures lead to ad-hoc debugging. The table provides deterministic recovery paths that route back to the correct phase rather than patching forward.

## Why Completion Checklist Order

The order matters: review docs → check dead code → audit → review mistakes → update spec → move → verify → commit. Each step depends on the previous. Skipping or reordering leads to incomplete features.
