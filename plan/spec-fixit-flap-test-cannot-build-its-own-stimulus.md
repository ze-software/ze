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

## RESOLVED 2026-09-04, and the title is wrong

**The test could always build its stimulus. The counter reading it was blind.**
Fixed in `055b97a29`; `iface-link-flap-during-commit` is four of four green in
QEMU on the arm64 runtime kernel 7.2.

The premise below, that the reload stopped holding `dhcpMu` across the burst, is
FALSE and is kept only so the reasoning is legible. `hostname` belongs to the
iface subtree despite the config being named `ze-bgp.conf`, no gate on the
SIGHUP path skips the apply, and `DHCPClient.Stop` still waits on each of forty
clients in turn, unchanged since the 1.1 to 3.3 s hold was measured. What failed
was `ze_iface_link_worker_blocked_total{name="zeflapv0"}`: the worker labels a
block with the name on the queue entry it was about to handle, the carrier
resync pushed every second carries no name, so the block lands on the empty
label and the burst coalesces behind it. The fixture read the named series.

The full account, both defects, the four measurements and the two designs
surveyed and rejected, is in `plan/journal/gate-fires-outside-its-population.md`.

This file is NOT closed, because `./le commit create` refuses a `remove` of a
spec with no independent-review artifact, and manufacturing one to delete a
skeleton resolved the same evening is not what that gate is for. It is left for
the owner to close or drop. One piece of follow-on work is real and is recorded
in the journal row rather than here: `internal/component/iface` publishes no
config-apply counter, which is why answering "did the reload reach the apply"
cost three QEMU cycles and two agents.

## Task

`test/plugin/iface-link-flap-during-commit.ci` measures one thing: a link that
flaps while a config commit holds `dhcpMu` must reach the metric live carrier
calls for, without the carrier self-heal repairing it. It builds that scenario
by timing. It sends SIGHUP, waits a fixed lead, then drives 101 carrier
transitions, and it needs the burst to land while the reload still holds the
lock.

That no longer happens. Measured four times on the arm64 QEMU VM on 2026-09-03,
`ze_iface_link_worker_blocked_total` did not move in any of six attempts, and
the run ends at `only 0 of 3 wanted rounds overlapped a commit in 6 attempts`.
The test is red, and it is red for the scenario never being built rather than
for the product regressing.

The lead is not the knob. It was one second, chosen when forty DHCP clients held
the lock for a measured 1.1 to 3.3 s; 100 ms on the same host also gave zero of
six. Both values are recorded on `flapCommitLead08`
(`internal/test/fixture/plugin_fixture_08_flap.go`).

Goal: restore the stimulus, on evidence rather than by tuning a literal. Answer
first whether the reload takes `dhcpMu` at all on a current host, and only then
choose between a longer hold, a synchronised burst, and a different way of
blocking the worker.

Open at design time, and the order matters:

1. Does the reload reach `reconcileDHCP` and take the lock? A SIGHUP over an
   unchanged config does not reach the iface apply at all, and the fixture flips
   a DHCP hostname each round precisely to force a real change. Whether that
   flip still produces a config the loader treats as changed is unestablished.
2. If the lock is taken, how long is it held for forty clients today? The 1.1
   to 3.3 s figure is from 2026-08-22 and predates whatever made the reload
   faster.
3. Only then: whether to lengthen the hold (more clients), synchronise the burst
   against a signal, or block the worker by a mechanism that does not depend on
   how long an unrelated reconcile happens to take.

Prefer the answer that removes the timing race rather than one that widens the
window (`ai/rules/simplicity.md`). A test whose stimulus is a race will come
back the next time the daemon gets faster, and this is the second time.

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
- Two instruments do NOT work here, both measured on 2026-09-03. There is no
  config-apply counter to poll: `internal/component/iface` publishes owned
  devices, coalescing, worker blocks and resyncs, and nothing about applies.
  And per-round stderr from this fixture does not reach the run output, though
  the one line printed before the round loop does, so a diagnostic added inside
  the loop prints nothing.

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
