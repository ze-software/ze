---
kind: directive
level: MUST
stage:
---
- **A code review MUST run against the diff before the final verification, and its findings MUST be written into the spec's `## Review Gate` section.** Any finding above NOTE MUST be fixed and the review MUST re-run, so the loop ends only when the review returns NOTEs or nothing. The final clean output MUST be pasted into the spec. A NOTE-only finding does not block.
