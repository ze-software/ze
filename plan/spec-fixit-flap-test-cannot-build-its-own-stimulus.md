# Spec: the link-flap test cannot build the stimulus it exists to measure

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | iface |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-09-03 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## NOT RESOLVED. This section claimed it was, on 2026-09-04, and that was wrong.

**Retracted the same day.** The claim was "the test could always build its
stimulus, the counter reading it was blind", on the evidence of four green runs
after `055b97a29`. An independent review found why those runs were green, and a
re-run confirmed it: the fixture read its block counter over a window that
opened BEFORE the SIGHUP, so a block recorded during the lead, before the burst
existed, satisfied it. The reload's hold contains a 1 Hz resync tick with
certainty, so the check was true in every round that took the lock at all, and
the wanted-rounds guard could no longer fail for the condition it names. The
green measured the guard going unfalsifiable, not the stimulus returning.

Narrowed to the burst window, the honest answer is `0 of 3 wanted rounds
overlapped`. The stimulus is NOT being built, which is what this file said in
the first place.

**What IS established**, and it is worth keeping:

- The counter blindness is real. `pushResync`
  (`internal/component/iface/link_queue.go`) builds a key with no interface
  name, so its block counts under `name=""`, and reading
  `{name="zeflapv0"}` alone misses it. Verified at source.
- The reload does reach the apply and does take `dhcpMu`. `hostname` is in the
  iface subtree, no gate on the SIGHUP path skips it, and `DHCPClient.Stop`
  still waits on forty clients in turn.
- **The counter cannot answer the question the test asks.** The worker takes the
  lock once per drained entry, so at most one block is counted per contiguous
  hold; when the resync wins that race the burst's own entry never TryLocks. A
  wide window over-reports, a narrow one under-reports. No reading of
  `ze_iface_link_worker_blocked_total` establishes "the burst met a held lock".

So the work this file names is REAL and is not the work it originally described.
It is not "restore a lost stimulus by tuning a lead". It is "give this test an
instrument that can see events queued while the worker is blocked", because the
one it has cannot. An event-queued-while-blocked counter is the shape that
answers it; `ze_iface_config_apply_started_total` (added in this work) answers
the weaker question of whether the apply began at all.

A second, independent failure showed up in the same re-runs and is not
diagnosed: two of three runs died on `kernel dropped 1054` and `207 netlink
notifications`. The zero-drops assertion is load-dependent and the round loop
now completes more rounds than it used to, which is the obvious suspect and is
NOT established.

The full account is in `plan/journal/gate-fires-outside-its-population.md`.

Read the rest of this file knowing the section above supersedes it. The Task
section still carries the reasoning as it stood at two earlier moments, each
marked where it was wrong, because the sequence is the useful part: a true
premise, a real defect found underneath it, and a green that was neither.

## Task

`test/plugin/iface-link-flap-during-commit.ci` measures one thing: a link that
flaps while a config commit holds `dhcpMu` must reach the metric live carrier
calls for, without the carrier self-heal repairing it. It builds that scenario
by timing. It sends SIGHUP, waits a fixed lead, then drives 101 carrier
transitions, and it needs the burst to land while the reload still holds the
lock.

**The paragraph that stood here was wrong, and the wrongness is the point of
this file.** It read: "That no longer happens. Measured four times on the arm64
QEMU VM on 2026-09-03, `ze_iface_link_worker_blocked_total` did not move in any
of six attempts ... The test is red for the scenario never being built rather
than for the product regressing."

The four measurements are real. The conclusion drawn from them was not, and the
correction drawn next was not either. The counter did not move partly because it
could not see a resync's block, which lands on the empty label; that defect is
real. But "the scenario was built on every one of those runs", written here on
2026-09-04, does not follow and a narrow-window re-run contradicts it.

The lead was suspected next and was also innocent. One second, chosen when forty
DHCP clients held the lock for a measured 1.1 to 3.3 s, and 100 ms on the same
host, both gave "zero of six" for the same reason: neither changes what the
counter can see. `flapCommitLead08` stays at one second and carries that warning.

Goal, as attempted in `055b97a29`: read a fact about the DAEMON from every series
of the counter rather than from one interface's label. That produced four green
runs and the green was an artefact of the window, not of the fix. See the top.
The three questions this file opened are answered, and the answers are worth
keeping because each was a candidate cause that turned out to be innocent:

1. **Does the reload reach `reconcileDHCP` and take the lock?** Yes. `hostname`
   is declared at `internal/component/iface/yang/ze-iface-conf.yang` inside the
   `interface` subtree, so the flip is a real change to iface's declared root
   despite the config file being named `ze-bgp.conf` (the runner names any first
   `stdin=` block that). No gate on the SIGHUP path skips the apply: the
   unchanged-config test and the per-plugin subtree test both pass on this diff.
2. **How long is the hold?** Unchanged in mechanism. `DHCPClient.Stop`
   (`internal/plugins/iface/dhcp/dhcp_linux.go`) closes the stop channel and
   then waits on done, forty times in sequence, and nothing has moved that work
   out from under the lock since the 1.1 to 3.3 s figure was taken.
3. **Which design restores the stimulus?** Still open, and now the whole
   question. Two were surveyed
   and are recorded in the journal row for whoever meets this class again: a
   `zetest` rendezvous inside the apply removes the timing race but costs a
   production call site with an empty body, and a participant fixture holding
   the reload is a trap, because participant order comes from ranging a Go map
   and is randomized per reload.

The advice the file opened with still stands and is the one thing to carry
forward: prefer the answer that removes a timing race over one that widens a
window (`ai/rules/simplicity.md`). It just was not a timing race this time, and
reaching for the timing knob first cost two QEMU cycles before the instrument
was questioned.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/platform-linux.md` - why this test is QEMU-gated and what that
  costs per iteration
  → Decision: <to be filled>
  → Constraint: <to be filled>
- [ ] `docs/architecture/testing/interop.md` - the four vacuity traps; assertion
  (2) of this test exists to defeat one of them
  → Decision: <to be filled>
  → Constraint: <to be filled>

**Key insights:** (minimal context to resume after compaction)
- The three assertions and why the test needs all three are written in the
  `.ci` header. Read it before changing any of them.
- There is no config-apply counter to poll: `internal/component/iface` publishes
  owned devices, coalescing, worker blocks and resyncs, and nothing about
  applies. That gap is real and is the follow-on work this file names.
- **CORRECTED 2026-09-04.** This bullet claimed per-round stderr from the
  fixture does not reach the run output, and that is FALSE. The relay carries
  every line: `attachStderrRelay` has no cap and does not stop at ready. What
  drops them is the report. `printGenericReport` calls
  `truncateOutput(rec.ClientOutput, 30)` and `truncateOutput`
  (`internal/test/runner/report.go`) keeps `lines[:maxLines]`, the FIRST 30, so
  a line printed before the round loop survives and every line inside it is
  cut. A diagnostic added inside the loop DOES print; it is simply never shown.
  The daemon also runs at WARN in every `.ci` run, because the runner's
  `SLOG_LEVEL` is dead code and `slogutil` reads only `ze.log*`; the knob for
  this component is `ze.log.interface`, not `ze.log.iface`.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/test/fixture/plugin_fixture_08_flap.go` - the round loop, the
  lead, the retry budget, and the end-of-run checks
- [ ] `internal/component/iface/register.go` - the reload path takes `dhcpMu`
  around `reconcileDHCP` and `suppressRAForConfig`
- [ ] `internal/component/iface/link_queue.go` - the worker that takes the same
  lock per apply, and `resyncCarrierState`
- [ ] `internal/component/iface/rate.go` - the four `ze_iface_*` metrics that
  exist, which is the evidence for "no apply counter"

**Behavior to preserve:**
- All three per-round assertions in the `.ci` header, and the reasons given
  there for each. Assertion (2) is what stops the test going vacuous the day
  the drain keeps up, and it must not be deleted to make the test green.
- The zero-drops check and the 101-transition bound. The bound is the kernel's
  netlink socket, not the queue, and at 401 transitions loss was measured.
- The bounded-retry design: a round that does not overlap is a miss, not a
  failure, and `stalled < flapWanted08` is the one place that turns repeated
  misses into a red.

**Behavior to change:**
- <to be filled>

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- <to be filled>

### Transformation Path
1. <to be filled>

### Boundaries Crossed
| Boundary | From | To |
|----------|------|-----|
| <to be filled> | <to be filled> | <to be filled> |

### Integration Points
- <to be filled>

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| <to be filled> | → | <to be filled> | <to be filled> |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| <to be filled> | <to be filled> | <to be filled> |

### Functional Tests
| Test | File | Validates |
|------|------|-----------|
| <to be filled> | `test/plugin/iface-link-flap-during-commit.ci` | the burst lands while the worker is held, on a host where the reload is faster than the lead that used to cover it |

## Files to Modify

- `internal/test/fixture/plugin_fixture_08_flap.go` - <what changes>
- `test/plugin/iface-link-flap-during-commit.ci` - <what changes, if the
  stimulus is config rather than fixture>

## Implementation Steps

1. <to be filled>

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before implementation
- [ ] Tests PASS after implementation
- [ ] `./le verify worktree` green

## Notes

The scoping half of this defect is already fixed, in `297b79044`: the
coalescing assertion fired on rounds the fixture had itself measured as not
overlapping, so a missed round failed the run with `coalesced counter did not
move; burst never outran worker` instead of retrying. That message is why the
real cause stayed hidden. It is recorded in
`plan/journal/gate-fires-outside-its-population.md`, which also carries both
measured leads and both dead-end instruments.

An iteration here costs a QEMU boot plus a cross-build, about three minutes, and
the failure mode is silent: a run that does not overlap looks exactly like a run
whose product regressed until the end-of-run message is read. Whoever takes this
should get the daemon's own log out of a keep-alive VM first, and answer
question 1 before touching a constant.
