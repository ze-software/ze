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

**Effective opaque treatment is part of that identity.** Equal edits over one
base can still produce different bytes when a registered selector preserves
opaque attributes for one destination but not another. Both rails normalize
an owned materialization in place before publishing it to dedup. A hit copies
the completed treatment, never a payload that still needs a second allocation.
Unmodified shared input uses the received-update entry's existing adopted
read-buffer lifetime when ordinary treatment changes bytes; preserving it or
finding it already normalized acquires no payload buffer.

**Publish only after the destination body succeeds.** The dedup entry borrows
the first destination item's outgoing slot; it does not own another buffer.
Both rails keep a candidate private until body building succeeds. On failure,
they abandon the candidate before returning the slot. Successful items stay in
the pending fan-out until every destination has copied any dedup hit, so their
buffers cannot be recycled during lookup. An encoding failure must not leave
the next destination reading another UPDATE from a returned slot.

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

The edit-set table does not stop sharing after four materializations. The
separate fast-rail body cache retains four stack slots; a miss beyond those
slots still builds and forwards the body.

## The negative half is expressible now

AC-1 and AC-2 of this work are negatives: one destination's bytes must never
reach another. They were recorded at closure as inexpressible in the `.ci`
harness. The only wire assertion was `expect=bgp`, which says what a peer DID
receive. The 2026-08-02 closure account named a gitignored draft,
`test/draft/plugin/wire-edit-fanout-dedup.ci`, that pinned each peer's exact frames. <!-- doc-links: ignore (historical draft, absent from the current checkout) -->
It recorded two `reject=stderr` crash guards and no wire negative. The
statement "peer C never got peer A's wire" had nowhere to go. That draft ran
in no gate, and it is absent from the checkout inspected on 2026-09-19.

`reject=bgp:conn=N:pattern=<hex>` is that statement
(`docs/architecture/testing/ci-format.md`). Every frame the peer's message loop
reads is checked against it and it is never consumed. The block carrying it must
also deliver something on the same connection, so the rejection can discriminate.
Its own proof is `internal/test/peer/reject_test.go`, plus the nine
RFC-behaviour tests that carry one. `test/plugin/wellknown-no-advertise-egress.ci`
is among them, and each was shown red by breaking the suppression it names.

The AC-1/AC-2 negatives are expressible with the current harness, but no live
fan-out fixture of that name supplies their socket proof. Recovery or
reconstruction is owned by
`plan/spec-wire-edit-5-fanout-dedup-deferred-fanout-ci.md`, which retains the
recorded community-suppression and hex-decoding blockers. The historical
draft's pinned frames are not current executed evidence; the holder requires
the socket assertions and their discrimination proof before promotion.

<!-- source: internal/test/peer/reject.go -- the wire rejection -->

## Cached copy-on-modify socket carrier

`test/plugin/bgp-rs-mod-copy.ci` addresses one edited eBGP recipient and one
internal control through the general `ForwardCached` rail. It deliberately
does not enable `rs-fast-path`. The source is internal, so its ORIGIN=IGP,
NEXT_HOP=1.1.1.1 and LOCAL_PREF=100 survive ingress. Its first UPDATE announces
10.0.0.0/24 with four-octet AS_SEQUENCE `[65002,65001]`.

The eBGP recipient has remote AS 65002, `session/as-override true` and
`session/rs-client true`. The last setting excludes the independent local-AS
prepend from this proof. Its exact frame requires `[65000,65001]` and no
LOCAL_PREF. The internal control requires the original path and LOCAL_PREF,
plus the legitimate reflection attributes: ORIGINATOR_ID 1.2.3.5 and
CLUSTER_LIST `[10.0.0.1]`. These come from the source's OPEN identifier and the
configured reflector cluster, not from the source UPDATE. This is isolation of
the source attributes, not a claim that the reflected frame is byte-identical.

A second UPDATE announces 10.0.1.0/24 with `[65003,65001]`. The target ASN is
absent, so both recipients must retain this populated path; eBGP still omits
LOCAL_PREF and the internal control still carries reflection attributes.
Persistent wire rejections also forbid LOCAL_PREF on the external socket and
the first recipient's overridden path on the internal socket.

One peer process maps connections by remote IP and completes all handshakes
before running scripts. The dedicated `bgpRSModCopy03` observer waits for EOR
from all three named addresses before sending a readiness fence only to the
source. The source sends both subjects after that fence, then expects a second
source-only fence before its script completes. The observer sends that completion
fence only after two non-EOR UPDATEs reach each intended recipient, allowing the
mapped peer harness to advance from the source to the recipient scripts. Exact
socket expectations remain the verdict on the bytes. The existing 15-second peer
and 10-second daemon deadlines and crash/cache guards remain unchanged.

The old one-peer fixture supplied no such proof: an external empty AS_PATH
failed first-AS validation, LOCAL_PREF was discarded on ingress, no destination
matched, and its observer accepted EOR alone. Adding a delay or retaining that
observer would not repair the missing stimulus and recipient.

AS override is part of `wireu.ASPathEdit`, not a second AS_PATH Set appended
after the eBGP prepend. A later Set would discard the prepend generator.
The composer resolves the effective policy Set, generator and Prepend fragments,
reconstructs a legacy AS4_PATH when needed, replaces the peer ASN, then applies
the protocol prepend and projects both outgoing path attributes together.
It reads segment values through the attribute spans, without stripping another
TLV header. Matching-width values retain the one-copy generator path; overrides
touch the destination-owned bytes, never the shared receive body.

`TestForwardASOverrideComposition` observes actual Session-written UPDATEs on
both rails, for ASN4 and ASN2 recipients. It requires the ordinary eBGP prepend
alongside override, preserves the RS-client and absent-target controls, and
exercises policy Set, generated Set and Prepend operations.
`TestASOverrideComposesLegacyAS4Projection` checks the real non-mappable ASN
through AS_PATH/AS4_PATH reconstruction, including a policy-provided companion.
`TestASOverrideUsesAttributeValue` retains both TLV header forms, source
immutability, repeated targets and the empty-path control.
<!-- source: internal/component/bgp/wireu/aspath_compose.go -- recordComposed -->
<!-- source: internal/component/bgp/wireu/aspath_slot.go -- ASPathIntent, ASPathEdit -->
<!-- test: internal/component/bgp/reactor/forward_as_override_composition_test.go TestForwardASOverrideComposition, TestASOverrideComposesLegacyAS4Projection -->
<!-- test: internal/component/bgp/reactor/filter_delta_test.go TestASOverrideUsesAttributeValue -->

For semantic discrimination, use an isolated Go overlay of
`internal/component/bgp/reactor/reactor_api_forward.go` that bypasses only the
`intent.OverridePeerAS, intent.OverrideLocalAS = facts.peerAS, facts.localAS`
assignment in `forwardUpdateSection`. Leave LOCAL_PREF handling, reflection,
materialization and the fast rail unchanged. The first external exact frame
must fail; the internal frames and target-AS-absent second external frame must
remain unchanged. This identifies AS replacement rather than generic forwarding
as the changed behavior. A separate absent-recipient control can remove only
`edited-ebgp-recipient` from a draft configuration and reduce the peer harness's
`tcp_connections` to two, leaving the observer unchanged: it must fail the
three-address EOR gate, never credit the internal recipient for the missing
target. Rebuild the isolated binaries after a producer change before crediting
either control.

<!-- source: internal/test/fixture/plugin_fixture_03_mod_copy.go -- bgpRSModCopy03 -->
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- forwardUpdateSection -->

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
