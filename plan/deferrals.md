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

> 2026-07-08: split. `spec-followup-bgp-rib-arch` (named above) was split into one child
> spec per work item under the `spec-rib-arch-*` prefix (`spec-rib-arch-0-umbrella` indexes
> `-1`..`-8`), per `ai/rules/planning.md` "Spec Sets". The umbrella file was renamed via
> `git mv` (history preserved); see `git log --follow plan/spec-rib-arch-0-umbrella.md`.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-09 | spec-followup-web-cli-ux AC-8/9 | Nushell shell-generator glue for the new flag inventory + `ze config show` config-section completion | AC-8 scopes completion to bash/zsh/fish (all wired + tested); nushell's single-completer model needs separate, un-runnable-here wiring | `plan/spec-followup-web-cli-ux.md` (AC-8/9 follow-up) | deferred |
| 2026-07-09 | spec-followup-web-cli-ux AC-5 | Subprocess-plugin web-route extensions (out-of-process plugins registering Go `http.Handler`s) | Architectural: ze plugins are subprocesses (JSON/text IPC), cannot register in-process handlers; AC-5 scoped to in-tree component pages by user decision | none (permanent exclusion; AC-5 covers in-tree pages) | cancelled |
