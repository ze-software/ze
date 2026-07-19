# 1219 - fixit: stop a named background process mid-.ci-test

Spec: `plan/spec-fixit-runner-kill-background.md`
Scope: `internal/test/runner/*` (+ `internal/test/cli/register.go`, docs)

> Number PROVISIONAL: `.counter` was 1183 while 1184/1185 already existed
> (concurrent agents). Re-allocate via `commit_helper learned-next` /
> `learned_numbers.py --fix` at drain.

## Problem

The `.ci` functional runner could START a background process (`cmd=background`)
and KILL it only at teardown/timeout. Nothing could stop ONE named process at a
chosen step, so any behavior that fires only on peer death (the motivating case:
IKEv2 DPD tearing down an SA when the responder goes silent) could not be
observed end to end. Background processes were tracked in a bare
`[]*exec.Cmd` (`bgProcs`) with no name field.

## Fix (the primitive)

Two additive grammar pieces + one executor intercept:

- `cmd=background:...:name=NAME` — optional handle (parseCmdExec gains a `:name=`
  marker). `cmd=stop:seq=N:name=NAME[:signal=kill|term]` — new directive
  (parseCmdStop), dispatched by `parseCmd` `case "stop"`.
- `runner_exec.go`: a `namedBg map[string]*exec.Cmd` populated at the
  `modeBackground` append; a `modeStop` intercept at the TOP of the command loop
  (before exec/arg parsing, since a stop runs no binary).
- `stopNamedBackground` (runner_exec_util.go): lookup → **fail closed** on unknown
  name → signal → **reap (Wait) before returning** → prune from BOTH `bgProcs`
  and `namedBg`. Pruning is what keeps teardown idempotent and `fgProc` selection
  ("last non-peer background process") correct after a mid-test kill.

## Design decisions (defaults the spec pre-resolved, confirmed in code)

- **Default signal = SIGKILL.** A SIGTERM'd ze IKE responder would send a clean
  peer DELETE (a DIFFERENT teardown branch, `established.go` `StateDead`) and
  defeat the DPD proof; SIGKILL leaves it silent, the exact DPD trigger. The
  directive still exposes `signal=term` (graceful), which delegates to the
  existing `terminateGracefully`.
- **Reap before advancing.** AC-1 ("terminated at step N, before step N+1") is
  only deterministic if the OS has reaped the process before the next step. So
  the stop step calls `Wait`; the wiring `.ci` then observes "gone" with no sleep.
- **Fail closed on unknown name.** A `.ci`-supplied name that matches no tracked
  process FAILS the test; it can never signal an arbitrary PID (only processes
  the runner itself started are in `namedBg`).

## Gotchas / traps hit

- **File-size ratchet.** Adding parseCmdStop pushed `record_parse.go` over the
  hard 1000-line limit. Split parseCmdExec + parseCmdStop into a NEW file
  `record_parse_cmd.go`; kept `nextMarker` in `record_parse.go` (shared with
  parseHTTP). `:seq=` literal → `markerSeq` const (goconst fired at 3 uses).
- **unparam surfaced by touching the file.** `terminateGracefully(cmd, timeout)`
  always got `2*time.Second` across all 4 call sites — a latent unparam that only
  fails `ze-lint-changed` once you edit the package. Fixed at source: const
  `teardownGraceTimeout`, dropped the param, updated all callers, and removed the
  now-duplicated SIGTERM-escalate logic by delegating the stop `term` path to it.
- **AC-5 marker collision check.** Before adding `:name=` as a reserved marker,
  confirm no existing `cmd=` exec line contains the literal (only comments and the
  unrelated `expect=event:...:name=` directive do — safe).
- **Orphan-suite guard.** A new `test/<dir>` with `.ci` files MUST be registered
  via `registerCIRoot` or `TestCIRootsRegistered` flags it. Added a `runner` root.

## Not done here (honest)

- **AC-4** (IPsec DPD end-to-end; restore `ipsec-dpd-timeout.ci`) NOT implemented.
  Per the spec's Failure Routing it also depends on A-3
  (`spec-fixit-plugin-event-subscription`), unconfirmed here (A-1 is validated
  from source). AC-1/AC-2/AC-3/AC-5 landed; AC-4 routed onward.
- The functional `.ci` was NOT run end to end (functional suites forbidden in the
  job that produced this — they kill live servers). Behavior is proven by scoped
  unit tests (kill/reap/prune/fail-closed + full `parseAndAdd` pipeline); the green
  `ze-test runner` run is left to CI / the drain pass.
