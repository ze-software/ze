# 1261 -- migrate-plugin-sleeps

## Context

`test/plugin/*.ci` carried 305 blind `time.sleep()` calls (460 across all `test/**/*.ci`), each a fixed timer standing in for "wait until the daemon reaches this state" -- the primary source of load-dependent flakiness and hidden races. The Layer 1 outbound barrier (`quiesce`/`wait_for_ack`) and Layer 2 payload predicates (`dispatch_until`/`wait_until`/`wait_for_event`, learned 1120) already existed but the tests did not use them. The goal, scoped by the user as one full campaign, was to convert every convertible sleep (~225) to a deterministic wait that returns exactly when the awaited state is observed. Landed as commit edfe4c0e1: test/plugin 305 -> 91 real sleeps, 214 eliminated.

## Decisions

- Migrate with the existing Layer 1+2 primitives over building the originally planned "Layer 3" FIB/tc/listener quiescers: feasibility agents showed those quiescers would cover only ~1-40 sleeps at high build cost, while the real hotspot was ~225 BGP-state tests convertible with what already existed. The Mistake Log records this redirect.
- Batch by primitive/risk (7 batches, each file verified 3x green) over one big sweep, catching per-test surprises early.
- Anchor-then-quiesce for outbound-after-inbound tests over a bare `quiesce()`: the engine barrier is OUTBOUND-only (`reactor_api.go` FlushForwardPool/DrainPeerSync), so a barrier run before the awaited inbound route arrives is vacuously green; a `dispatch_until` on the inbound `show` anchors it first (R-1).
- Flakes exposed by removing a sleep are fixed at the source, never by re-adding the sleep (R-3, the `feedback_sleep_hides_races` doctrine).

## Consequences

- **The residue is explicitly handed to `spec-fixit-migrate-sleeps-infra`:** the remaining sleeps after this spec were all DEFER (~35: fib-kernel/vpp async EventBus, external-warn stderr visibility, reject/negative tests with no positive edge to poll, RS reflection inbound gap, control-message path, and the bgp-redistribute group whose forward-pool `quiesce` blocks ~10s and overruns the 15s test timeout) or KEEP (~56: raw-UDP peer-driver pacing, deliberate timers, standalone-driver backoffs), each recorded with its reason in the migration worklist. The ratchet target was ~235; achieved 246, the ~11-sleep gap being exactly the un-convertible bgp-redistribute group. That follow-on spec owns converting them by building the missing infrastructure.
- The `test/.ci-sleep-baseline` drop was STAGED at implementation time (held for the shared-file Layer 2 landing, R-4); the baseline has since been reworked into composable signed-delta form and ratcheted further by the follow-on work (test/plugin is at 48 sleeps at closure time -- below this spec's 91, evidence the handoff is being consumed).
- Converted tests are deterministic, not merely faster: they return when state is observed. Four genuinely under-synchronized tests were surfaced and fixed (below), which a sleep-tuning pass would have re-masked.

## Gotchas

- Removing blind sleeps surfaced 4 real races the sleeps were masking: `bfd-show-profile` (missing `wait_for_post_startup`), `nexthop-self`, `nexthop-unchanged`, `rr-basic` (single-shot RIB read before the UPDATE landed). All fixed with a deterministic poll or readiness barrier -- the intended outcome, not collateral damage.
- Ratchet-counting trap: `verify_wiring_docs.py` counts `time.sleep(` across FULL file text including comments, so test-relax notes that quoted the removed call inflated the count; 78 files' comments had to be rephrased so raw == real.
- The `.ci` line-count hook blocks any edit reducing non-comment lines; every conversion carries a `// test-relax:` token, which both exempts the edit and creates the audit trail.
- The engine quiesce barrier does NOT cover inbound processing; inbound assertions must use `dispatch_until` on a `show`, never `quiesce`. DrainPeerSync returns immediately when nothing is queued yet, which is what makes the vacuous-barrier race possible.

## Files

- ~155 `test/plugin/*.ci` files converted (commit edfe4c0e1; 41 direct + ~114 via six parallel self-verifying agents)
- `test/scripts/ze_api.py` (primitives consumed, not modified: `dispatch_until`, `dispatch_until_done`, `wait_until`, `wait_for_event`, `quiesce`, `wait_for_ack`, `wait_for_post_startup`)
- `test/plugin/prefix-filter-accept.ci` (the converted reference)
- `test/.ci-sleep-baseline` (ratchet; staged then landed in delta form)
- `plan/spec-fixit-migrate-sleeps-infra.md` (follow-on home for the DEFER/KEEP residue)
