---
kind: fence
level:
stage:
---
```
[ ] 1. Documentation updates -- check Documentation Update Checklist below.
      Every question must be answered Yes/No. Every Yes requires a file path.
      BLOCKING: code that changes documented behavior without updating docs is not done.
[ ] 2. Env var check -- if YANG config leaves were added under `environment/`,
      verify matching `ze.<name>.<leaf>` env vars are registered via `env.MustRegister()`.
      Run `ze env registered` (or grep for `MustRegister`) to confirm.
[ ] 3. Dead code check -- search unused functions/types, ASK before removing
[ ] 3. File modularity check -- for each modified .go file:
      Line count: >1000 → review concerns, split only when the separation is right (rules/file-modularity.md)
      // Design: topic annotation still matches file's actual concern?
      If split: copy to new files, adjust annotation per new concern
      // Related: still accurate? Add/update for new couplings
      (rules/design-doc-references.md, rules/related-refs.md)
[ ] 4. Implementation Audit (BLOCKING -- rules/implementation-audit.md)
[ ] 5. Pre-Commit Verification (BLOCKING -- do NOT trust the audit)
      Re-read spec from scratch. For each item, independently verify:
      - Files Exist: `ls` every file from "Files to Create" -- paste output
      - AC Verified: for each AC-N, grep/test for fresh evidence -- do NOT copy from audit
      - Wiring Verified: read each .ci file, confirm it tests the claimed path
      - Assumptions Resolved: every A-N row is `confirmed` or `broken` with evidence --
        none `unvalidated`; broken ones have Mistake Log + Deviations entries
      Fill the "## Pre-Commit Verification" section in the spec.
      The closure review and `./le commit create` consume the completed spec.
[ ] 6. Critical Review (BLOCKING -- rules/quality.md)
[ ] 7. Review Mistake Log -- check MEMORY.md, promote if seen before
[ ] 7. Update spec -- Implementation Summary, Documentation Updates, Deviations
[ ] 7. Write journal row: append a row to `plan/journal/<class>.md` naming the spec in the Spec column
[ ] 7. Verify: `./le verify worktree` + git status + git diff, no unintended changes
[ ] 7. Executive Summary Report -- present to user with what was done and what is left (including deferred).
        BLOCKING: journal row (step 10) must exist. Name the file in the report.
        Do NOT ask to commit. The user will tell you when to commit.
[ ] 7. Commit (when user says so) -- ONE helper-generated script, TWO commits (per Spec Closure below):
        - **Commit A:** `./le commit create replace` with `--file` for all implementation files (code, tests, docs, schema)
          + `--file plan/journal/<class>.md` + `--file plan/spec-<name>.md` (preserves edits)
        - **Commit B:** `./le commit create append remove plan/spec-<name>.md` (spec closure)
        Run the generated script yourself and the work is done. There is no
        second step. If spec closure or the journal row is missing, it never happens.
        Disjoint systems (e.g., CLI and BGP encoding) get separate commits.
```
