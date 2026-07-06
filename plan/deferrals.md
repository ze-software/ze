# Deferrals

Tracked deferrals from implementation sessions. Every decision to not perform in-scope work
must be recorded here with a destination (receiving spec or explicit cancellation).

A row lives here only while the work has **no home**. Once it is moved into a spec, it is
resolved (`ai/rules/deferral-tracking.md`: "Moved to another spec → `done`, Destination =
receiving spec") and the spec becomes the tracker. Run `/ze-status` for the live backlog.

> 2026-07-06: backlog triage. The accumulated log (220 rows) was cleared. 113 rows were already
> resolved (done/cancelled). Every remaining row was verified against the codebase with a
> producing `file:line`: 24 were already implemented (closed) and 83 were migrated into 13
> consolidated umbrella skeleton specs, named `spec-finish-<subsystem>` (subsystem shipped,
> residual/test bits left) or `spec-followup-<subsystem>` (additional/future work).
> Finish: `spec-finish-l2tp`, `spec-finish-ci-coverage`, `spec-finish-report-bus`,
> `spec-finish-vpp-stub`. Followup: `spec-followup-test-infra`, `spec-followup-hooks`,
> `spec-followup-vpp-traffic`, `spec-followup-vpp-iface`, `spec-followup-l2tp-call`,
> `spec-followup-bgp-rib-arch`, `spec-followup-bgp-feature`, `spec-followup-web-cli-ux`,
> `spec-followup-subsystem`. Plus one re-point to `spec-firewall-dynamic-address-group`.
> The pre-triage revision of this file (in git history)
> preserves every closed row with its evidence. The log below is intentionally empty.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
