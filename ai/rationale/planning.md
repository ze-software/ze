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

## History moved from rule points (2026-09-26)

- `ai/rules/points/planning/directives/give-a-phase-an-agent-that-can-produce-its-artifact.md`: "Measured 2026-09-04: two spec phases in one session came back as text because their briefs named `ze-read`, and a 593-line spec crossed three contexts to reach `plan/`."
- `ai/rules/points/planning/spec-metadata-blocking/route-the-lesson-write-a-journal-row-only-when-nothing-governs-it.md`: "We do not SAVE a lesson, we UPDATE the system with it (owner directive, 2026-08-10)."
- `ai/rules/points/planning/work-phases/review-is-independent-of-the-author.md`: "`/ze-close` MUST run every lens itself (owner directive, 2026-08-15)."
- `ai/rules/points/planning/work-phases/the-round-cap-and-who-authorises-a-sixth-pass.md`: "THE SIXTH IS THOMAS'S DECISION (owner ruling, 2026-08-17)."
