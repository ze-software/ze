# Deferrals: functional-suites-missing-their-make-target

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-09 | the kernel-profile-fixtures-leak-into-registry spec (commit `1acf1a627`), verification | `./le functional` in the retired `mk/test-functional.mk` (current producer: `internal/le/functional/suites.go`) prints `make ze-<suite>-test` for each failed suite, and three of the 24 suite names in its `all_suites` line have no such target: `ldp`, `rsvpte` and `install`. Those three suites cannot be run through make at all | Found while verifying two `.ci` changes in the install suite, which had to be run by hand-building the isolated binary pair. The verification did not depend on the target existing, so it gets a spec rather than a fix folded into the work in hand (`ai/rules/completion.md`) | `plan/spec-functional-suites-missing-their-make-target.md` | deferred |
