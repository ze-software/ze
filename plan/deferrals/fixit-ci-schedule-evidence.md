# Deferrals: fixit-ci-schedule-evidence

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-27 | spec-fixit-ci-schedule-evidence | Run `make ze-qemu-integration-test` in an automated pipeline (AC-6). It is deliberately absent from `evidence-nightly.yml`, so today NOTHING automated executes the QEMU suites and `ai/rules/platform-linux.md` is enforced by review alone | GitHub-hosted runners do not reliably provide nested virt / KVM, so the target cannot be scheduled until a KVM-capable or self-hosted runner exists. Not a code gap: the follow-up is self-announcing because `TestEvidenceNightlyRunsFuzzAndIntegration` FAILS if the target is added without also updating the guard. `plan/learned/1253-fixit-ci-schedule-evidence.md` records the decision and required this row at closure | `plan/spec-release-evidence-gate.md` | deferred |
