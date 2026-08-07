# 1355 - A regression test that fails against HEAD can still be vacuous

**Date:** 2026-08-07
**Scope:** testing, agent workflow, review

## What Changed

The interop harness gained two-contract removals, a failure-note helper that
words its own claim, and six committed regression tests driving `run.py` and one
scenario's `check.py` through a probe. The Review Gate for
`spec-wire-edit-4-api-origin-deferred-bird-interop` ran fifteen rounds to get
there, and six consecutive rounds found the SAME defect class in the tests
written to close the previous instance of it.

## The Failure

An assertion anchored on a string that the BROKEN path also emits.

Six instances, each found by review after the previous one was "fixed":

| Round | The anchor | Why the broken path satisfied it |
|-------|-----------|----------------------------------|
| 8 | `process plugin failed` | the swallowed path prints it too, inside "could not be read" |
| 9 | `would race this scenario` | two producers, container and network; deleting either half left the other printing it |
| 9 | `timed out` | same, on the cleanup reports |
| 10 | the "not found" exemption | no mode produced docker's ordinary answer, so the clause was unpinned |
| 12 | `RENDERED_LEFT=none` | read an in-process attribute, not the disk, so a quiet removal failure still reported success |
| 13 | `RENDERED_ON_DISK=False` | also true of a copy that was never created |

## The Mechanism, and the thing that hid it

**Failing against HEAD proves a test detects the ORIGINAL bug. It does not prove
the test detects the FIX being undone later.** Those are different questions, and
only the second is what a regression test is for.

`ai/rules/interop-and-goal-validation.md` requires reverting the change and
showing the test fails. Every one of the six above passed that check. HEAD lacked
the whole mechanism, so the anchor was absent for a reason unrelated to the
property under test. Re-add the mechanism and delete one BRANCH of it, and the
surviving branch prints the same words.

The tell is a producer count. An anchor with more than one reachable producer on
the path its test drives is not an assertion about the branch it names.

## What Fixed It

Stop spot-checking, and measure every branch.

Break exactly ONE branch, on a copy, and run EVERY mode against it. Require an
exact diagonal: each break caught by the mode that owns it and by no other.
A break that reds nothing is a vacuous test. A break that reds several modes is
an anchor with several producers.

For the eleven branches of the two removal contracts:

| Break | Modes that go RED |
|-------|-------------------|
| container timeout / OSError / exit-code raise | `setup-container-timeout` / `-oserror` / `-error` |
| container cleanup report, timeout / OSError | `teardown-container-timeout` / `-oserror` |
| network timeout / OSError / exit-code raise | `setup-network-timeout` / `-oserror` / `-error`+`-notfound` |
| network cleanup report, timeout / OSError | `teardown-network-timeout` / `-oserror` |
| the "not found" exemption | `setup-network-absent` |

The matrix is cheap: each cell is one subprocess of about 0.05s. Re-run it after
any change to the anchors or the branches. It caught the round 13 hole (an
assertion true of a copy that never existed) as soon as the precondition was
added, because the no-render break moved from "no mode notices" to "all four".

## Three findings worth carrying beyond this spec

**A guard that reads one failure shape is a guard with holes.** The pre-clean
denied on `TimeoutExpired` and never on `result.returncode`, so every failure
docker ANSWERS with passed through: removal-already-in-progress, a device-busy
driver, a network with active endpoints. `subprocess.run` adds a third shape,
`OSError`, for a binary that cannot run. An object survives all three.

**The same function can owe OPPOSITE answers at two call sites.** `docker_rm`
runs as cleanup from `run.py`'s `finally`, where raising loses the run's summary
and every tally with it, and as a pre-clean from `Scenario.setup`, where
continuing leaves a scenario racing another one's daemon. One `strict` parameter,
the shape `docker_logs` already used. Applying either contract to both sites is a
defect, and both directions were shipped and caught here.

**An exemption matched on a bare substring exempts things it never meant to.**
`"not found" in stderr` was written for a missing network. A misconfigured
`DOCKER_CONTEXT` answers `context ... not found` having removed nothing, and
exempted itself. Anchor the whole phrase, including the object's own name.

## Cost

Fifteen rounds. The feature was correct from round 5: `git diff internal/` carries
nothing, and the scenario has passed all ten assertions throughout. Every round
after that reviewed evidence code, not product code.

That ratio is the point rather than an embarrassment. The interop proof was cheap
and right. Making the evidence trustworthy was neither, and the six repeats say
the vacuity class is not obvious to the author even when the author has just been
shown the previous instance. Reach for the matrix on the FIRST regression test,
not the sixth.

## Files

- `test/interop/interop.py` -- `docker_rm` and `Scenario._remove_network` gained the
  two contracts, `strict` for the pre-clean and silent-continue for cleanup, each
  denying on all three failure shapes; `docker_logs` gained `strict`;
  `observer_fail_line`, `raise_if_observer_failed` and `observer_failure_note` are new
- `test/interop/run.py` -- `main`'s per-scenario handler: the interrupt catch, the
  counters ahead of the note read, and the note printed as it comes back
- `test/interop/run_test.go` -- `pythonOrSkip`, `probeEnv`, and the runner tests
- `test/interop/scenario55_check_test.go` (new) -- the scenario's failure-path tests
- `test/interop/testdata/runner_probe.py`, `test/interop/testdata/check_except_probe.py`
  (both new) -- the probes the Go tests drive, starting no container
- `test/interop/scenarios/55-wire-edit-api-origin-bird/check.py`,
  `announce-api-origin.py` -- `_check_session_budget`, the strict log read, and the
  guard whose reasoning three rounds corrected
- `docs/architecture/testing/interop.md` -- the process-plugin failure section
- `internal/component/bgp/reactor/announce_build.go` -- read, not changed:
  `(*announceAttrs).emit` is the writer both rails converge on
- `internal/component/bgp/reactor/peer.go` -- read, not changed: `Peer.ShouldQueue`
  decides which rail an announce takes

