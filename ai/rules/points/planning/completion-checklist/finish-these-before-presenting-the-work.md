---
kind: directive
level: MUST
stage:
---
- **Once the tests pass, these MUST be finished IN ORDER, and none of them MAY be skipped.** Documentation updates (`ai/rules/writing.md`), the env-var registration check for any new YANG leaf under `environment/`, the dead-code and file-modularity review of every changed `.go`, the implementation audit, the Pre-Commit Verification section re-derived from the spec rather than copied from the audit, the critical review, the spec's own Implementation Summary and Deviations, the `plan/journal/<class>.md` row naming the spec, `./le verify worktree`, and the executive summary.
- **Pre-Commit Verification MUST NOT trust the audit.** The spec MUST be re-read from scratch, every file in "Files to Create" MUST be listed, every AC MUST get fresh evidence, every `.ci` file MUST be read to confirm it tests the claimed path, and every assumption MUST read `confirmed` or `broken` with evidence.
- **You MUST NOT ask to commit.** The owner says when. `plan/TEMPLATE.md` carries the checklist rows, and `docs/contributing/spec-workflow.md` carries the closure commit shape.
