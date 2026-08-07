---
kind: table
level:
stage:
---
| Situation | Do |
|-----------|-----|
| A tagged test fails after your change | Fix YOUR code. The test is the requirement |
| You believe the test is genuinely wrong | STOP. Show the user the RFC text beside the test and ask. Do not edit first and explain after |
| The summary misquotes the RFC | Fix `rfc/short/rfcNNNN.md` (keep the id), then re-run `/ze-rfc-audit` |
| Reformat / comment / re-tag | Allowed; behavior must be unchanged |
| You added, moved, deleted, or re-tagged a tagged test (or an edit shifted its line) | Run `make ze-rfc-index` and commit `ai/RFC-REQUIREMENTS.md` in the SAME commit. The ledger records each test's `file:line`, and `ze-rfc-check` (both verify modes) fails on a stale ledger, so a skipped regen lands on the next session as a cross-commit diff |
