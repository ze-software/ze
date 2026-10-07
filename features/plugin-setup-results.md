# Plugin Setup Results

## Meta

| Field | Value |
|-------|-------|
| Name | Plugin Setup Results |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/plugin/registry/setup.go, internal/component/plugin/register.go, cmd/ze/hub/startup_gate.go |
| Real-path tests | test/parse/show-plugin-list.ci, test/parse/show-plugin-list-memlock.ci, cmd/ze/hub/startup_gate_test.go::TestRunRefusesOnHardSetupFailure, cmd/ze/hub/startup_gate_test.go::TestRunRefusalNamesEveryHardFailure, cmd/ze/hub/startup_gate_test.go::TestCLIVerbUnaffectedByHardSetupFailure |
| Docs | docs/guide/status.md, docs/features/introspection.md |
| Doc review | 2026-10-07: unknown outcome for a plugin that recorded nothing read in registry/setup.go; refusal naming every hard failure covered by startup_gate_test.go |
| Defect review | 2026-10-07: journal 2026-09-17 row fixed; no immediate spec names these paths |

## Description

Every registered plugin records, from its own `init()`, what its setup achieved: `succeeded`, `soft-failure`, or `hard-failure`, with the reason an operator acts on. `show plugin list` carries that record in its `outcome` and `reason` columns, in any `ze` process with no daemon running, and lists a plugin that recorded nothing as `unknown` rather than omitting it, so a feature that is absent for an environment reason can say why. The daemon reads the same registry once, at the first statement of `hub.run`, and refuses to start when a plugin recorded a hard failure, naming every failing plugin rather than the first. A CLI verb never reaches that gate, so the command that reports the fault keeps working on a host where the daemon will not boot. This is not `show health`, which runs a probe now: this replays an outcome decided once, before `main()`. <!-- source: internal/component/plugin/registry/setup.go -- RecordSetup, SetupResults, HardSetupFailures --> <!-- source: internal/component/plugin/register.go -- pluginRows, the outcome joined onto each show plugin list row --> <!-- source: cmd/ze/hub/startup_gate.go -- hardSetupFailure, the refusal run applies -->
