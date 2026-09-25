# Spec: the link-flap test cannot build the stimulus it exists to measure

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | iface |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Current remaining work

The instrument and stimulus repair exist in the current tree. Source read on
2026-09-19: `flapCommit08` and the round loop in
`internal/test/fixture/plugin_fixture_08_flap.go` poll the apply-start counter,
and `flapBlocked08` reads queued-while-blocked events. The netlink monitor's
`start` sets `monitorReceiveBufferBytes` on link, address and neighbour
subscriptions. The five-of-five and twelve-run results below remain dated
2026-09-04 evidence, not current measurements.

Remaining work is to reconcile that evidence with the current tree, retain
discrimination of the overlap guard and zero-drop assertion, obtain the clean
independent review the latest update says is missing, and complete the normal
verification and closure gates. This is no longer unstarted stimulus design.

## The instrument landed and the test PASSES (2026-09-04 evening, session 2d2bc99a)

This section records the first instrument run on 2026-09-04. Its one-run
limitations were superseded by the later five-of-five update below.

`d0affb5e4b` built `ze_iface_link_events_queued_while_blocked_total`, which is
exactly the instrument the section below says does not exist, and wired it into
the fixture (`plugin_fixture_08_flap.go:224`). Measured once on the arm64 QEMU
guest, runtime kernel 7.2:

```
27.7s    1/1  PASS  322  iface-link-flap-during-commit
```

The daemon and fixture binaries were both cross-built AFTER the counter landed
(`bin/ze-linux-arm64` 14:01, `bin/le-test-linux-arm64` 14:10, counter committed
13:06), so this is not a stale binary reporting on old code.

**Why this green is not the third false one.** Both earlier greens came from
reading `ze_iface_link_worker_blocked_total` over a TIME WINDOW, and the file
below explains why no window works: wide enough to see the 1 Hz resync's block
and the guard is true in every round that took the lock at all, narrow enough to
exclude it and it reads zero through a genuine hold. The new counter has no
window. The queue marks the wait and a push during it is counted whatever else
happens, so a pass means rounds genuinely overlapped rather than that the guard
went unfalsifiable. A run that did not overlap would have ended with
`only N of 3 wanted rounds overlapped`, and it did not.

**What is still NOT established, and blocks closing this file:**

- ONE run, on an idle VM. The standard this test was held to before was six.
- The second failure this file names, two of three runs dying on
  `kernel dropped 1054` and `207 netlink notifications` with the zero-drops
  assertion suspected of being load-dependent, did not appear here. One quiet
  run does not disprove a load-dependent failure.
- The checklist below requires `./le verify worktree` green. The population was
  22 stages red on 2026-09-04 morning and has not been re-measured since.

**How to run it**, because reconstructing this cost six guest boots and the
recipe is now in `docs/architecture/testing/qemu-integration.md`: `le test qemu
all-tests` is a GUEST action, the Alpine guest needs `packages "coreutils
iproute2"` or BusyBox `timeout` and `ip` defeat it, the binaries need canonical
names, and a single test is `le test bgp plugin iface-link-flap-during-commit`.

The following account preserves the retraction and later repair sequence.

## Historical retraction and subsequent repair (2026-09-04)

**Retracted the same day.** The claim was "the test could always build its
stimulus, the counter reading it was blind", on the evidence of four green runs
after `055b97a29`. An independent review found why those runs were green, and a
re-run confirmed it: the fixture read its block counter over a window that
opened BEFORE the SIGHUP, so a block recorded during the lead, before the burst
existed, satisfied it. The reload's hold contains a 1 Hz resync tick with
certainty, so the check was true in every round that took the lock at all, and
the wanted-rounds guard could no longer fail for the condition it names. The
green measured the guard going unfalsifiable, not the stimulus returning.

Narrowed to the burst window, the honest answer was `0 of 3 wanted rounds
overlapped`: the stimulus was not being built, which is what this file said in
the first place.

**That is now fixed, and the test is five of five green.** Three things were
needed and none of them was the lead:

1. An instrument that can answer the question.
   `ze_iface_link_events_queued_while_blocked_total` counts events that ARRIVED
   while the worker was waiting, which the block counter cannot: the worker
   takes the lock once per drained entry, so at most one block exists per
   contiguous hold and the resync usually takes it.
2. A burst synchronised on the apply rather than on a clock. It fires when
   `ze_iface_config_apply_started_total` moves, which is why that counter is
   incremented before the lock and not after it. This removes the race the
   advice below asks for rather than widening it.
3. A netlink receive buffer. Two runs in three had been dying on `kernel
   dropped 1054` and `207 netlink notifications`, and that was a PRODUCT defect
   found through this test: all three of the monitor's sockets kept
   `net.core.rmem_default`, so the kernel discarded what it could not queue
   before ze ever saw it. Fixed at `monitorReceiveBufferBytes`, and zero drops
   in twelve runs since.

The latest 2026-09-04 update recorded a remaining independent-review obligation:
`./le commit create` refused removal without the artifact. One independent
review had returned issues including two blockers, and the author reported each
finding fixed. A second clean review was still owed; no such review or new run
is claimed by this reconciliation.

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

Before that final repair, the investigation had established that the block
counter could not answer whether events arrived during the hold, and had not
yet diagnosed the netlink drops. Those were the open questions the
queued-while-blocked instrument, apply-start synchronisation and receive buffer
subsequently addressed. They are retained as history rather than new tasks.

The full account is in `plan/journal/gate-fires-outside-its-population.md`.

The next section retains the earlier reasoning where useful, with the current
remaining goal stated first.

## Task

Complete evidence and review for `test/plugin/iface-link-flap-during-commit.ci`.
The test must prove that a link which flaps while a config commit holds
`dhcpMu` reaches the metric live carrier calls for, without carrier self-heal.
The current fixture observes apply-start before bursting and counts events
queued while the worker is blocked; it no longer schedules the burst by a
fixed lead. Preserve all per-round assertions, zero drops and the 101-transition
bound. No new stimulus mechanism is planned unless current evidence shows a
surviving defect.

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
3. **Which design restored the stimulus?** The final 2026-09-04 update records
   apply-start synchronisation and the queued-while-blocked instrument. The
   earlier `zetest` rendezvous and participant-order approaches remain rejected
   alternatives in the journal; neither is current implementation work.

The advice the file opened with still stands and is the one thing to carry
forward: prefer the answer that removes a timing race over one that widens a
window (`ai/rules/simplicity.md`). It just was not a timing race this time, and
reaching for the timing knob first cost two QEMU cycles before the instrument
was questioned.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/platform-linux.md` - why this test is QEMU-gated and what that
  costs per iteration
  → Constraint: runtime proof needs a Linux guest with the test's network capabilities and current daemon and fixture binaries.
- [ ] `docs/architecture/testing/interop.md` - the four vacuity traps; assertion
  (2) of this test exists to defeat one of them
  → Constraint: the overlap guard must fail when no burst overlaps; a normal green alone is insufficient.

**Key insights:** (minimal context to resume after compaction)
- The three assertions and why the test needs all three are written in the
  `.ci` header. Read it before changing any of them.
- `ze_iface_config_apply_started_total` and
  `ze_iface_link_events_queued_while_blocked_total` now exist in `rate.go`.
  The current fixture reads both; the absent-instrument premise is historical.
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
- [ ] `internal/component/iface/rate.go` - apply-start and queued-while-blocked metrics
- [ ] `internal/plugins/iface/netlink/monitor_linux.go` - receive-buffer binding for all three subscriptions

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
- No additional behaviour change is planned. Re-open a producing defect only if the remaining proof identifies one.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `le test bgp plugin iface-link-flap-during-commit` inside the supported Linux/QEMU environment.

### Transformation Path
1. The fixture sends SIGHUP and observes the apply-start counter.
2. The fixture bursts carrier transitions and reads events queued during the worker's lock wait.
3. The test checks overlap, resulting live-carrier metric, lack of self-heal and lack of netlink drops.

### Boundaries Crossed
| Boundary | From | To |
|----------|------|-----|
| Fixture to daemon | SIGHUP and metrics | apply-start and queued-event observations |

### Integration Points
- Native flap fixture, iface queue/metrics and netlink subscriptions.

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Linux plugin suite runs the flap scenario | → | apply-start, queued-event and netlink paths | `test/plugin/iface-link-flap-during-commit.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| Existing queue and subscription tests | `internal/component/iface/queued_while_blocked_test.go` and `internal/plugins/iface/netlink/monitor_subscribe_test.go` | Inspect their behavioural coverage during independent review; file existence is not a pass |

### Functional Tests
| Test | File | Validates |
|------|------|-----------|
| `iface-link-flap-during-commit` | `test/plugin/iface-link-flap-during-commit.ci` | the burst genuinely overlaps the hold, carrier state converges without self-heal, and no notification drops are hidden |

## Files to Modify

- This spec for current evidence and review disposition.
- The native fixture, iface queue/metrics or netlink monitor only if the remaining proof identifies a surviving defect.

## Implementation Steps

1. Compare the current producers with the final repair and review findings recorded above.
2. Record current Linux/QEMU results, including overlap-guard discrimination and the load-dependent zero-drop concern; do not reuse the historical run counts as fresh evidence.
3. Obtain a clean independent review and `./le verify worktree` before normal closure.

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
