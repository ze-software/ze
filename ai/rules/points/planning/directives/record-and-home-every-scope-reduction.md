---
kind: directive
level: MUST
stage:
---
**Every decision to not perform in-scope work MUST be recorded as a row in `plan/deferrals/<source>.md` AND MUST name a destination `plan/spec-*.md` that exists on disk.** A destination that is prose is a deletion with a polite name, and `plan/known-failures/` is never one. The row stays live until the work LANDS, not until it is filed. `docs/contributing/spec-workflow.md` carries the shard format, the status vocabulary, and the order in which a destination is chosen.
