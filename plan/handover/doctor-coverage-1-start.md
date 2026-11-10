# Handover: start spec-doctor-coverage-1-audit-and-gate (P2)

Written 2026-11-10 by the session that landed spec-operator-errors-name-the-fix
(P1). Thomas ordered "Land P1 and P2". P1's implementation is committed
(`728fb3b57e`, `3e94cb7c40`, `fdf90dd99e`, plus the AC-13 completion that
follows them). P2 has not been started: no claim, no code. Thomas then said
"Complete the work", so the same session runs P2 next; this page is its plan
and the recovery point if that session ends first.

Spec: `spec-doctor-coverage-1-audit-and-gate`
(status design; owner questions O-1, O-2, O-3 decided 2026-11-08). Thomas's
order approves the design and the claim above the WIP cap for this spec:
claim it with `ZE_SPEC_WIP_CAP=25 ./le spec claim
spec-doctor-coverage-1-audit-and-gate` after
marking it `ready`, and record that in the spec as P1 did.

## Audit of the spec against the tree (2026-11-10, read-only)

| # | Finding | What it changes |
|---|---------|-----------------|
| 1 | BLOCKER: `internal/core/diagnostic/doctor_daemon.go` (`DaemonDoctorCheck`, `RegisterDaemonDoctorCheck`, `RunDaemonDoctorChecks`; commit 311177442c, 2026-11-09) already runs daemon-only checks through `HandleDoctor` (`internal/component/doctor/cmd/show.go`). Its one user is the dns-source checks (`internal/component/config/system/register.go`, `test/plugin/doctor-dns-source.ci`); the page documents it under "Daemon doctor checks" | The `runtime` phase REPLACES it (no-layering): delete `doctor_daemon.go`, re-register the dns-source checks with `Phase: runtime`, add an Existing Mechanisms row, rewrite the page section |
| 2 | `./le commit create` now runs registered change-set checks through `internal/le/commit/changecheck` (`Register`, `Run`; built by P1) | doctor-coverage's gate registers into that seam from its own package; no commit code names it |
| 3 | `internal/core/opererr` exists: problem, why, fix (the command alone, its own line); log attrs `why`, `fix`, plus a caller's `code` | `Diagnostic` gains `Fix` (`"fix"`) beside the existing `FixSafety` and `Repair`; say in its doc comment how the three differ; use the same `fix` and `code` log keys the gate's A-1 requires |
| 4 | Goal Gates name whole suites (`./le test functional plugin`, `ui`) | restate as the three named `.ci` files; no agent runs a whole suite or `./le verify` |
| 5 | O-1's "daemon socket answers" is SSH with credentials (`cmdutil.RunCommand` → `sshclient.ExecCommand`) | state the case "daemon up, no credentials": local answer plus the `doctor-runtime-not-checked` warning |
| 6 | `collectPluginDoctorChecks` (`doctor/cmd/show.go`) does not filter by phase; `outputText` (`doctor/doctor.go`) draws the text line; `runDoctorChecks` is unexported, so `HandleDoctor` needs an exported runtime runner; `TestDoctorDependencyInventory` moved to `doctor/doctor_test.go` | Current Behavior and Files to Modify rows |
| 7 | `internal/core/diagnostic/codes.go` carried another session's 1-line hunk on 2026-11-10 | check `git diff` before committing it; commit own hunks with `hunks` if the foreign one is unsafe |

## Proposed phases (one agent each, about 60 tool calls)

| Phase | Owns | Covers | Depends on |
|-------|------|--------|------------|
| P1a wire | `pkg/plugin/rpc/types.go` (+test), `pkg/plugin/sdk/sdk_types.go`, `sdk_callbacks.go`; `internal/core/diagnostic/{types,doctor_registry,codes}.go`; delete `doctor_daemon.go`; `config/system/register.go`; `doctor/{registry,checks_plugin_registry,doctor.go}`, `doctor/cmd/show.go`; the doctor page; `docs/plugin-development/protocol.md`. `codes.go` takes every new code up front | AC-1, AC-3, AC-7 base, AC-8 docs | - |
| P2a gate scan | `internal/le/arch/doctorcoverage/**` | AC-4 check form, AC-5, AC-12, A-1, A-2 | - (parallel with P1a) |
| P1b local route O-1 | `doctor/answer.go`, `doctor/doctor.go` `Run`, the three new `.ci` files | AC-2, AC-10 | P1a |
| P1c recent events | new `doctor/checks_recent_events.go` (+test), `doctor/register.go` | AC-11 | P1a (parallel with P1b) |
| P2b commit route | `register.go` of `doctorcoverage` into `changecheck` | AC-4 | P2a |
| P3 audit | about 18 area agents, at most 5 at once (system-plugins split in 3, bgp-plugins in 2, routing-protocols in 2); spec numbers 3..N assigned in advance; one merge agent writes `plan/audits/doctor-problem-inventory.md` | AC-6, AC-9 | P2a report |

Child 2 (`spec-doctor-coverage-2-home-cpe-batch.md`) waits on P1a only (plus P1b for its R-5).

## Conventions that cost time in the P1 session

- Every test, build and lint runs in the main thread, one at a time; agents only read and edit.
- `.ci` tests that exercise uncommitted `le` or runner code need `./le --name session test ...`; the HEAD-built `le` lacks the change.
- `ze-stripped` and `ze-stripped-flow` are built only by `./le test functional ui`; for a single `ui` test build them into `bin/le-s-<session>/` with tags `ze_core ze_ssh` and `ze_core ze_ssh ze_ddos ze_anomaly ze_flowexport`, and put that directory first on PATH.
- A journal row the commit adds or rewrites is capped at 600 characters.
- `./le test weakened approve` cannot name a `.ci` under a dotted directory (journal row, owned by spec-tooling-one-off-defects); put that verdict in the commit body as a `Test-judge:` line.
- `le` panics at startup when an action's Summary exceeds 60 characters; load the `le` root in a test after adding an action.
