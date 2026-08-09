# RFC 7606 Section 5.1: one NLRI-bearing field per relayed UPDATE

Ze originated compliant UPDATEs and both splitters emitted one NLRI-bearing
component per message. The relay did not. `buildFwdBody` emitted an UPDATE whole
whenever it fit, in whatever shape it arrived. Both of its fit branches now
split on shape as well as on size.

## The requirement

> "An UPDATE message MUST NOT contain more than one of the following: non-empty
> Withdrawn Routes field, non-empty Network Layer Reachability Information
> field, MP_REACH_NLRI attribute, and MP_UNREACH_NLRI attribute." (RFC 7606
> Section 5.1)

> "Since older BGP speakers may not implement these restrictions, an
> implementation MUST still be prepared to receive these fields in any position
> or combination." (RFC 7606 Section 5.1)

Requirement ids: `RFC7606-5.1-2` (the sender obligation this page covers) and
`RFC7606-5.1-3` (the receiver obligation). Checklist: `rfc/short/rfc7606.md`.

The first requirement of that section, `RFC7606-5.1-1`, is a recorded and
publicly disclosed divergence. See `docs/architecture/wire/mp-nlri-ordering.md`.

## The decisions

**One definition of "mixed", not two.** `NLRIBearingFieldCount(withdrawn, attrs,
nlri)` takes the three wire sections, so the parsed path (`*message.Update`) and
the lazy wire path (`*wireu.WireUpdate`) share it. Letting each splitter decide
for itself would have left two places to get the RFC wrong independently.
<!-- source: internal/component/bgp/message/rfc7606_shape.go -- NLRIBearingFieldCount, MixesNLRIFields -->

**Two entry points, because the callers differ.** `SplitWireUpdate` has one
non-test caller, the relay, so its fast path was changed in place.
`Splitter.Split` is shared with origination, which is compliant by construction,
so making it walk the attributes on every send to re-learn that would be pure
cost. `SplitCompliant` is the relay's entry point. Both share `splitByShape`.
The asymmetry is deliberate and its reason sits in the doc comment, so the next
reader does not "fix" it.
<!-- source: internal/component/bgp/message/update_split.go -- Split, SplitCompliant -->
<!-- source: internal/component/bgp/wireu/split.go -- SplitWireUpdate -->

**Cache the verdict per message, not per peer.** The forward loop runs once per
destination over the same `*wireu.WireUpdate` pointer. An uncached attribute
walk turns a per-message cost into a per-message-times-peers cost. Measured:
3.3ns cached against 51.7ns recomputed, both allocation-free.

## Traps a later author will meet

**An allocation assertion cannot pin a property that does not allocate.** The
first cache test used `testing.AllocsPerRun` and asserted zero. Deleting the
cache still passed, because the attribute walk is allocation-free either way.
When a test asserts the ABSENCE of something, ask what would still be absent if
the mechanism were deleted. The replacement is a cold-to-warm timing ratio,
chosen after benchmarking both variants to confirm the gap is 16x and therefore
separable with a wide threshold.

**A fixture at the extreme cannot pin a boundary.** The mixed fixture carries
all four fields, so changing `> 1` to `> 2` survived every test inside the
`wireu` package. A reactor test in another package killed it. A boundary test
belongs in the package that owns the comparison.

**A Section 5.1 sender change is not peer-discriminable.** The third bullet
makes every conforming receiver accept any field combination, so FRR installs
the split and the unsplit form identically. Interop proves the split output is
ACCEPTED. It cannot prove a peer would reject the unsplit form. That part is
unit and `.ci` coverage.

## The duplicate NEXT_HOP defect this work uncovered

The announce builder inserted a NEXT_HOP unconditionally while the mandatory
attribute writer had already copied the stored attribute block, NEXT_HOP
included. The route server re-encodes every stored route through this path on
peer-up replay, so every replayed IPv4 route carried NEXT_HOP twice, and MP
families carried a second MP_REACH. RFC 7606 Section 3(g) makes a duplicated
attribute treat-as-withdraw, so FRR discarded the lot.

The attribute plan a contribution adds now REPLACES a NEXT_HOP the base already
carries, rather than appending a second one. The defect lives on the origination
and re-advertise builder, not on `buildFwdBody`.
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- buildBatchAnnounceUpdate, the NEXT_HOP contribution -->
<!-- source: internal/component/bgp/reactor/announce_build.go -- announceAttrs, the replacing attribute plan -->
