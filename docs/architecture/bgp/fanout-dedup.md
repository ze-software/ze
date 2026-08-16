# Fan-Out Dedup: share the plan, not the buffer

One received route that fans out to N destinations did per-destination work N
times. The forward-body cache keyed on the materialized wire POINTER, so it
could dedupe only destinations that had already produced the same pointer. That
is sharing downstream of the copy it exists to avoid.

The fix fingerprints the EDIT SET, which is upstream of the copy, and confirms
every hint with a full equality check.

<!-- source: internal/component/bgp/filterapi/fingerprint.go -- edit-set fingerprint -->
<!-- source: internal/component/bgp/reactor/forward_dedup.go -- fwdDedupTable, per-policy-group materialization -->

## The decisions

**Share the PLAN, not the BUFFER.** The design first assumed one shared
materialization referenced by several forward items, and spent two assumptions,
one risk and half a blast radius on making that safe.
`BenchmarkFanoutRebuildOnly`, run before any of that code, measured the rebuild
at 416ns and a flat copy of its result at 2.1ns. Sharing the plan and copying
per destination keeps 99.5% of the win and leaves the one-buffer-one-item
ownership model untouched.

**The fingerprint is a hint. Full equality decides.** There is no fast-path
bypass. A collision would send one destination another destination's wire, which
is the worst failure available on this path.

**The identity carries the BASE as well as the digest.** Two destinations in one
policy group can hold identical edit sets over DIFFERENT bases. A
filter-supplied raw export override reaches that state.

**No adaptive threshold.** Any cutoff L silently disables sharing for a group
size at or above L, which is a silent cap. The trade is +3% worst case against
-29% best case, and it is recorded here rather than hidden behind a constant.

## Measured

Per-destination cost, interleaved A/B, medians of 6:

| Group shape (destinations, groups) | Change |
|------------------------------------|--------|
| (2, 1) | -10.5% |
| (10, 2) | -14.4% |
| (100, 2) | -28.6% |
| (2, 2) | +3.3% |
| (100, 100) | +2.8% |

The last two rows are the case where no two destinations share a group.
Allocations per operation are identical in both arms.

About 120ns per destination, `buildFwdBody` plus the wire wrapper, is NOT
recovered. Taking it needs the shared buffer this design exists to avoid. It was
measured and left.

The route-server rail's four-slot cache, which stopped caching beyond four
bodies without saying so, is gone. No cap is left to be silent about.

## The negative half is expressible now

AC-1 and AC-2 of this work are negatives: one destination's bytes must never
reach another. They were recorded at closure as inexpressible in the `.ci`
harness. The only wire assertion was `expect=bgp`, which says what a peer DID
receive. The nearest thing written is `test/draft/plugin/wire-edit-fanout-dedup.ci`,
which pins each peer's exact frames. It is a DRAFT, so no gate runs it
(`test/draft/README.md`), and its only negatives are two `reject=stderr` crash
guards rather than a wire negative. The statement "peer C never got peer A's
wire" had nowhere to go.

`reject=bgp:conn=N:pattern=<hex>` is that statement
(`docs/architecture/testing/ci-format.md`). Every frame the peer's message loop
reads is checked against it and it is never consumed. The block carrying it must
also deliver something on the same connection, so the rejection can discriminate.
Its own proof is `internal/test/peer/reject_test.go`, plus the nine
RFC-behaviour tests that carry one. `test/plugin/wellknown-no-advertise-egress.ci`
is among them, and each was shown red by breaking the suppression it names.

The AC-1/AC-2 negatives are therefore EXPRESSIBLE now. They are not yet
EXPRESSED. Promoting the draft to a live fixture that carries the rejections is
separate work. Until that happens the fan-out negatives rest on the draft's
pinned frames rather than on a gate.

<!-- source: internal/test/peer/reject.go -- the wire rejection -->

## Traps

**A guard field that no test exercises is decorative.** `fwdDedupTable.begin`
guards on `e.fp != fp || e.id != id`, and the whole reactor suite stayed green
with the base half of that guard removed. One test built an identity, and it
passed the same base for both entries. Mutate each half of a compound guard and
confirm something goes red.

**Reaching a real read-pool borrow needs an IBGP destination on a 2-byte send
context.** An eBGP destination folds the RFC 6793 width change into the edit
set, so the wire is rebuilt and nothing is borrowed. A fixture that does not pin
all three preconditions goes quiet instead of failing.

**An ordering assertion needs a happens-before, not a timer.** A sentinel batch
dispatched to the same peer's worker cannot be handled until the forwarded
batch's `done()` has run, because the worker takes one batch at a time.
