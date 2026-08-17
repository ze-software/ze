# Deferrals: fixit-vacuous-eor-family-tests

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-09 | spec-fixit-vacuous-eor-family-tests (review pass 1) | `scripts/dev/audit-test-relaxation.py` `run_audit` derives a file's newly added `test-relax:` tokens with a positional slice, so a token inserted above an existing one makes the audit print the OLD token's reason | Found while running the review skill's step-0 audit over this spec's diff. Nothing in this spec depends on the audit's printed text, so the defect blocks no goal here and gets a spec rather than a same-session fix (`ai/rules/completion.md`) | `spec-fixit-relax-audit-reports-the-wrong-token`, closed 2026-08-17 | done |
