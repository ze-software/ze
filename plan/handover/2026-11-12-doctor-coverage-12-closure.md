# Handover: doctor-coverage-12-plugin-infra, closure

Spec: `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md`
(in-progress, claimed above the WIP cap under the owner's one-off permission
recorded in the spec; the claim was released when this was written).

Claim it again with `ZE_SPEC_WIP_CAP=<in-progress count + 1> ./le spec claim
plan/release/validation/spec-doctor-coverage-12-plugin-infra.md`.

## What blocks closure

| Item | State | What unblocks it |
|------|-------|------------------|
| AC-40, AC-44 | Five reload-refused lines carry no code, because `doctor-config-reload-refused` is declared nowhere: `server/reload.go` (`reloadConfig`), `server/startup_autoload.go` (`selectAutoLoad`, two in `startAutoLoaded`), `server/failure_policy.go` (`stopDaemonOnStartupFailure`). `TestNotProblemLinesCarryCodes` is red on exactly these five | The owner's choice, asked 2026-11-12: (a) wait for `spec-doctor-coverage-11-hub-engine.md` AC-34 to register the code (the recorded design, A-5), or (b) declare it here. Then each line takes the code, or `config-plugin-verify` where a verifier refused, and raises it on the bus (R-7) |
| Review Gate, `/ze-close` | Not started | AC-40 |

## What this session did (2026-11-12)

Everything else the previous handoff listed is done and committed. In order:

| Commit | What |
|--------|------|
| eb6e1d9a80, c3692ce544 | Three functional drafts promoted. Two had barriers that were not barriers (a foreground `ze` is started, not awaited; `ready=` holds one state) and were repaired first; new fixture `daemon/await-answer` |
| 0a249617d2, 47f112ceba, ad304d17e4 | Spec bookkeeping; journal row for the IKE-tag red of `TestDoctorCoverageSourceHoldsEveryRegisteredCode` (owned by doctor-coverage-3) |
| e442cb88c0 | Data race in the plugin cleanup hook list (older than this spec) |
| e329b99f01 | R-10: a test per producer line, each observed red under its break |
| 909da87de2 | R-8, R-9: `request plugin restart` acts on a running plugin; bounded records say what they dropped |
| ce86ac4a64 | The restart command was unreachable from the CLI since it was added (it claimed a selector its leaf could not satisfy); found by the first end-to-end test of it |
| 115b64f109, 20c8cf2b5e | A sampling race in `TestDrainCallerLifecycle`; lint cleanups |
| 029401af1b | `show` enricher registration sorted in place a list a concurrent `show` iterates |
| ae5d1d84fd | R-12: a restarted plugin was blamed for its predecessor's cut enricher call |
| 0becf0044f | New event code `doctor-plugin-restart-overlap` (AC-48) |
| 325ec5d0d6 | Two independent reviews found the restart could hang holding the config transaction lock; every wait on its path is now bounded and a self-restart is refused |
| 9325dde3da | R-11: restart of an external plugin that ignores its stop |
| 105d71fa11 | `spec-plugin-restart-bounded-and-configuration-true`: nine defects the reviews found and this spec does not fix |

## What the next session must know

- The restart of a running plugin is a choice the previous session made
  without the owner, to make five doctor fix lines true. The owner may reverse
  it. What it forecloses: the command no longer refuses to stop a working
  plugin an operator names.
- No review round was run after the last hardening edit (325ec5d0d6). The
  Review Gate should read `server/restart.go`, `server/reload_tx.go`
  (`restartPluginProcess`), `server/register_plugin_restart.go` and
  `process/manager.go` (`Respawn`) first.
- Known reds, none caused by this spec's last state: the five AC-40 lines; the
  two IPC fixtures in `plugin/process` (spec-registry-run-engine-test-seam);
  `TestDoctorCoverageSourceHoldsEveryRegisteredCode` under the feature tags;
  `refuse-by-default` on `stopRemovedProcess` (`server/startup_removal.go`,
  a wait error with only a debug line).
- `TestReloadFailureKeepsPreExistingDependency` failed once in a full-package
  run and passed 200 isolated runs; the cause proposed from source is a row of
  the restart spec, to reproduce or refute.
- Testing: feature tags from `feature-gates.txt` prefixed with `ze_core`
  (VPP builds again); the server package takes about 150 s under `-race`;
  functional runs need `./le --name session ...` to see uncommitted fixtures.
  Other sessions' half-edits broke the shared build about ten times in this
  session: a bounded watcher that retries the real run is the way through.
