# Deferrals: relax-token-gate-is-per-file-not-per-change

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-09 | the kernel-profile-fixtures-leak-into-registry spec (commit `1acf1a627`), phase 2 | `c_test_weakening` in `.claude/hooks/pretool-writeedit.py` searches the WHOLE new file for a `test-relax:` token on the Write branch, so one pre-existing token disables the weakening check for every later overwrite of that file; `run_audit` in `scripts/dev/audit-test-relaxation.py` attributes added reasons by tail slice, so a reason added above a middle hunk prints a different, pre-existing reason | Found while moving `test/install/kernel-compose.ci` off the tracked source tree. That move does not depend on the gate being fixed, so it gets a spec rather than a fix folded into the work in hand (`ai/rules/completion.md`) | `plan/spec-relax-token-gate-is-per-file-not-per-change.md` | done |
