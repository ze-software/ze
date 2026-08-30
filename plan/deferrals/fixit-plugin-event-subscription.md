# Deferrals: fixit-plugin-event-subscription

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-19 | spec-fixit-plugin-event-subscription functional-proof | functional .ci for the SDK-fork end-to-end path not authored ~~(unrunnable here)~~ | ~~live-server/QEMU constraint, deferred to CI~~ **CORRECTED 2026-08-30: "unrunnable here" no longer holds.** The `test/ipsec/*.ci` suite runs natively on this machine, so the test can be written and run locally. **Triaged the same day as an improvement, not a release defect:** the path works, this is coverage | plan/future/spec-ci-coverage-remaining-surfaces.md | deferred |

