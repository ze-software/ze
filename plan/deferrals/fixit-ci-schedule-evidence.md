# Deferrals: fixit-ci-schedule-evidence

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-27 | spec-fixit-ci-schedule-evidence | Run `./le qemu run command "./le qemu all-tests"` in an automated pipeline (AC-6). The retired target was `ze-qemu-integration-test`. Nothing automated executed the QEMU suites at the time, so `ai/rules/platform-linux.md` was enforced by review alone | GitHub-hosted runners do not reliably provide nested virt / KVM, so the action cannot be scheduled until a KVM-capable or self-hosted runner exists. Not a code gap -- infrastructure constraint. | future CI spec when KVM runner exists | deferred |
