# 843 -- verify-debugging-protocol

Spec: `plan/spec-verify-debugging-protocol.md` (closed). Implemented in commit
`7cc0829f6` ("verify: add failure routing protocol"); the spec file was committed
alongside the code.

## Context

`make ze-verify` was a fail-fast dependency chain. When a stage failed, a reader
(human or AI) had to scroll one giant combined log to work out which failures
were related, which scope to rerun, and where the full evidence was. Worse, the
documented triage workflow named artifacts (`tmp/ze-verify.log`,
`tmp/ze-verify-failures.log`) that the chain did not actually produce -- the env
var and helper scripts existed but nothing wrote the files.

## Decisions

- **A tested protocol runner owns verify, not shell.** `scripts/status/verify_run.go`
  runs every stage, preserves the combined log and per-stage logs under `tmp/verify/`,
  and writes a compact text + JSON failure index (`tmp/ze-verify-failures.{log,json}`).
  Shell wrappers (`verify-status.sh`) stay thin; `verify-summary.sh` was too weak for
  `TEST FAILURE:` blocks and stage-local grouping.
- **Stage boundary is the first routing boundary.** The first protocol version never
  merges groups across stages -- a compile or setup error cascades and would create
  false aggregation. Group by stage first, then by conservative stage-local heuristics
  (suite + failure kind + subsystem prefix).
- **Prefer native failure manifests over text parsing.** Where a runner already owns
  structured state, emit groups from it: `internal/test/runner/failure_group.go`
  (`GroupFunctionalFailures`, `PrintFailureGroups`) emits `VERIFY FAILURE GROUP: {json}`
  lines under a `VERIFY FAILURE INDEX` header. Parse rendered text only for external or
  one-line stages (lint, wiring-docs, ExaBGP).
- **Verify mode is an env-controlled rendering mode.** `ZE_VERIFY_MODE=1`
  (`verifyModeEnabled()`) adds machine-readable artifacts and log-safe rendering
  without making normal interactive runs verbose. The detail gate is
  `(verbose || verifyModeEnabled())`.
- **The compact index is a routing artifact, not a replacement for the full log.**
  Groups cap inline members and excerpt lines and point at the full stage log.

## Gotchas (from the mistake log)

- **Docs named artifacts that were never produced.** `docs/functional-tests.md` and
  `ai/rules/git-safety.md` described a `tmp/ze-verify*.log` workflow that the chain did
  not emit. Lesson (escalation candidate): verification-workflow docs must have an
  executable source path and a test/fixture proving the artifact exists -- a future
  doc-anchor check could enforce this.
- **Rerun commands were hard-coded to `ze-test bgp <suite>`.** `report.go`/`display.go`
  misdirected debugging for non-BGP `.ci` suites (ui, managed, web, firewall, policy,
  l2tp, install). Rerun commands are now suite-aware (`FormatRerunCommand`).
- **Saved ExaBGP logs were polluted by carriage-return status updates.** Verify mode
  switches `test/exabgp-compat/bin/functional` to newline-safe output with exact
  repo-root reproducer commands.

## Consequences

- `make ze-verify` now produces a compact, routeable failure index first, with full
  per-stage logs preserved. Read `tmp/ze-verify-failures.log` before the combined log.
- The `VERIFY FAILURE GROUP: {json}` token family is the house convention for
  machine-readable test output; any new per-test or per-step machine output should
  extend it rather than invent a parallel format. See the planned test-runner work
  (`plan/spec-test-runner-unify.md`, `-web-parallel`, `-trace-mode`) and
  `docs/architecture/testing/runner-architecture.md`.

## Files

None recorded.
