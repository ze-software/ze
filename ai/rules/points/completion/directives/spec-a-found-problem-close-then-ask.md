---
kind: directive
level: MUST
stage:
rationale: ai/rationale/found-problem-spec-first.md
---
**An unrelated problem found while working on something else MUST be recorded in `plan/journal/<class>.md` without expanding the implementation scope.** Search `plan/journal/` for the symptom first and use the existing five columns, `| Date | Spec | Surface | Symptom | Fix |`. Commit the row and close the work in hand against its agreed criteria; a finding is not authorization to create or implement another spec.
**A row MUST NOT exceed 600 characters, and a class with 10 or more rows is due for a fix pass.** The detail goes in the fix commit, a spec, or `ai/rationale/`: `./le commit create` refuses a longer row added or rewritten against HEAD (`ValidateFile`, `internal/le/spec/journal/validate.go`), and `./le spec journal report` lists the due classes first.
**Defects exposed by scoped proof and defects that block an agreed acceptance criterion MUST follow the fix obligation above.** An unrelated problem's size or proximity to edited code does not authorize its repair. For absent RFC requirements, use the gap records and scope policy in `ai/rules/rfc-compliance.md`.
**Reports MUST state findings that limit the claimed baseline, including unresolved defects and unverified behavior.** Record enough evidence to distinguish an existing-capability defect from an absent feature, but do not turn unrelated discovery into an uncommissioned investigation.
