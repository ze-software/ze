---
kind: directive
level: MUST
stage:
applies: recording a design decision Thomas states, or changing a page's Design decisions section
---
**A design decision Thomas states MUST be written, in the turn he states it, as a row of the `## Design decisions` section of the page that documents its subject, with his record as its Source; a row once written MUST change or leave its page only with him.** The section's format is `docs/architecture/README.md`, "Page format", and `./le doc consistency` judges it. A changed or removed row reaches a commit only through an approved `departs` row of a spec or `./le commit create ... decision-change "<his words>"` run from his terminal; `judgeDecisionRows` (`internal/le/doc/consistency/decisionschange.go`) refuses every other such commit, and a malformed added row.
