# 1225 - RFC 7606 Section 5.1: one NLRI-bearing field per relayed UPDATE

**Spec:** `plan/spec-rfc7606-5-1-2-relay-shape.md`
**Date:** 2026-07-20

## What changed

Ze already originated compliant UPDATEs and both splitters already emitted one NLRI-bearing
component per message. The relay did not: `buildFwdBody` emitted an UPDATE whole whenever it
FIT, in whatever shape it arrived. Both of its fits branches now split on shape as well as
size, so every UPDATE ze puts on the wire carries at most one of Withdrawn Routes, NLRI,
MP_REACH_NLRI and MP_UNREACH_NLRI.

## Design decisions worth keeping

**One definition of "mixed", not two.** `message.NLRIBearingFieldCount(withdrawn, attrs, nlri)`
takes the three wire sections, so the parsed path (`*message.Update`) and the lazy wire path
(`*wireu.WireUpdate`) share it. The spec had assumed each splitter would decide for itself;
that would have left two places to get the RFC wrong independently.

**Two entry points, because the callers differ.** `SplitWireUpdate` has exactly one non-test
caller (the relay), so its fast path was changed in place. `Splitter.Split` is shared with
origination (`peer_send.go`), which is compliant by construction, so making it walk the
attributes on every send to re-learn that would be pure cost. `SplitCompliant` is the relay's
entry point and both share `splitByShape`. Asymmetric on purpose; the reason is in the doc
comment so the next reader does not "fix" it.

**Cache the verdict per message, not per peer.** The forward loop runs once per destination
over the SAME `*wireu.WireUpdate` pointer, so an uncached attribute walk turns a per-message
cost into per-message-times-peers. Measured: 3.3ns cached against 51.7ns recomputed, both
allocation-free.

## The mistake worth remembering

**An allocation assertion cannot pin a property that does not allocate.** The first version of
the cache test used `testing.AllocsPerRun` and asserted zero. Removing the cache entirely still
passed: the attribute walk is allocation-free either way, so the test proved nothing. This is
the second time in two specs that a test claimed to pin a mechanism it could not observe (see
[1224](1224-rfc7606-close-gaps.md), where a "no log line appears" test could not see the
`Enabled()` guard).

The general shape: **when a test asserts the ABSENCE of something, ask what would still be
absent if the mechanism were deleted.** If the answer is "the same thing", the test is inert.
The fix here was a cold/warm timing ratio, chosen only after benchmarking both variants to
confirm the gap was 16x and therefore separable with a wide threshold.

Corollary that cost a second round: **a fixture at the extreme cannot pin a boundary.** The
mixed fixture carries all four fields, so a `> 1` becoming `> 2` survived inside the wireu
package and was killed only by a reactor test in a different package. Boundary tests belong in
the package that owns the comparison.

## Test-harness knowledge (`test/plugin`)

- `Checker.check` (`internal/test/peer/checker.go`) matches an arriving message against
  EVERY pending rule for the current connection, not just the head. Declaration order in a
  `.ci` is therefore not arrival order, and a `.ci` cannot assert message ordering.
- An unmatched KEEPALIVE or EOR is silently accepted. **Declaring an EOR expectation opts INTO
  a race rather than out of it**: whichever of the EOR and the real message arrives first meets
  the rule. Two flaky failures came from that before the rules were left undeclared.
- Rules are held per connection and the checker advances to the next connection's block only
  when the current one empties, so a frame arriving on connection B while A's rules are pending
  is matched against A's rules.
- `bgp-rs-reactor-fastpath.ci` fails about 1 in 6 on an unmodified tree for exactly this
  reason. Do not inherit its `rs-fast-path enable` if the test does not need it: with the fast
  path on, the receiver intermittently gets a second, unprepended announce (75% stable);
  without it, 20/20.

## The two biggest lessons (owner-driven)

**Never park a blocker; never reduce coverage to reach green.** When a pre-existing
duplicate-NEXT_HOP defect blocked the interop scenario, the first instinct was to park the
scenario in `tmp/` and offer to drop the deliverable. The owner rejected that outright: "a BGP
daemon that cannot interoperate is NOTHING." The defect was in scope the moment this work's
interop depended on that path. It got fixed at source, and the scenario shipped. New standing
rule: `ai/rules/completion.md` (+ a DANGER prohibition in `ai/INSTRUCTIONS.md`). "Pre-existing"
is not an escape hatch; the entry point that first reaches a bug owns it.

**A passing interop test proves nothing until you prove it discriminates.** The first
scenario PASSED with ALL THREE fixes reverted. Two reasons compounded: (1) RFC 7606 Section
5.1's third bullet makes every receiver accept any field combination, so a conforming FRR
installs the split and the unsplit form identically -- the §5.1 sender change is
NOT peer-discriminable; (2) the asserted route reached FRR by a verbatim `buildFwdBody` path,
which never duplicates NEXT_HOP, so the dedup fix was never exercised either. Empirically
(re-verified 2026-07-20): the REPLAY route (10.0.0.0/24) is forwarded verbatim and stays
installed even with the dedup reverted -- it is VACUOUS. The discriminator is the LIVE-relayed
split announce (Path 2, 203.0.113.0/24): it is re-encoded through `buildWireModeUpdate`, so
reverting the dedup makes FRR lose it (`attribute type 3 appears twice - discard`) and the
scenario goes RED. The independent-speaker scenario 48 exercises the dedup directly and
unambiguously via the adj-rib-in delta-replay. New rule:
`interop-and-goal-validation.md` "Prove the test discriminates" -- revert the fix, rebuild the
image, confirm RED, restore. A regression/interop test added to already-working code never had
a red phase and is unproven until you force one.

**Ze is a general BGP speaker; route-server is a CONFIGURED mode.** The scenario framing
implied "ze = route server". `no bgp enforce-first-as` on the FRR client is correct RFC 7947
RS-client config for that mode, not a workaround -- but only because ze is configured as an RS
here. Reworded throughout.

## The duplicate-NEXT_HOP defect (found via interop, fixed)

`buildWireModeUpdate` (reactor_api_batch.go) inserted a NEXT_HOP unconditionally, while
`writeMandatoryAttrs` had already copied the full stored attribute block -- NEXT_HOP included.
The route server's replay-on-peer-up re-encodes every stored route through this path
(`formatHexCommand`, adj_rib_in/rib.go, emits `attr set <attrs-incl-NH> nhop set <nh>`), so
every replayed IPv4 route carried NEXT_HOP twice; same for MP families (a second MP_REACH).
RFC 7606 Section 3(g) makes a duplicated attribute treat-as-withdraw, so FRR discarded the lot.
Fix: `stripAttribute` removes any pre-existing NEXT_HOP/MP_REACH before writing the
authoritative one, safe because an unset next-hop errors out before the builder
(`resolveNextHop` -> `ErrNextHopUnset`; `NextHopUnset` is the zero value, nexthop.go). This
is a distinct bug from the §5.1 relay-shape change and lives on the origination/re-advertise
builder, not `buildFwdBody`.

## Interop: DELIVERED (`test/interop/scenarios/47-rfc7606-relay-shape-frr`)

The scenario drives ze (configured as a route server) via a `ze-test peer` injector sidecar
added to `test/interop/interop.py`. The injector emits wire bytes no conforming daemon would
produce. It exercises two paths:

1. **Replay** (`buildBatchAnnounceUpdate`): the injector announces a route before FRR connects
   (harness start order: injector + ze, then FRR), so FRR gets it via replay-on-peer-up.
   Asserting FRR installs it discriminates the NEXT_HOP-dedup fix -- RED when reverted, with
   FRR logging `attribute type 3 appears twice - discard attribute`.
2. **Live split** (`buildFwdBody`): after FRR is up, a mixed UPDATE is forwarded live and split.
   Asserting FRR installs the announce proves the split output is ACCEPTED. It cannot prove FRR
   would reject the unsplit form (Section 5.1 third bullet); that is unit + `.ci`.

A second pre-existing observation, not pursued and not blocking: ze's attribute-discard marker
(type 253) goes out on the wire when an eBGP peer sends LOCAL_PREF (same bytes
`bgp-rs-fastpath-ebgp-shared.ci` flags). The fixture avoids LOCAL_PREF to sidestep it.

**The coverage lesson.** Every `.ci` in the plugin suite talks to ze's own test peer, which is
tolerant by construction: it asserts the bytes it was told to expect and nothing else. A real
daemon applies RFC 7606 to what it receives. The duplicate-NEXT_HOP defect made ze's
route-server output unusable to FRR while the entire plugin `.ci` suite stayed green.
`ze-peer` cannot find that class of bug, by design -- a real peer in an interop test can.

## Related

- [1224](1224-rfc7606-close-gaps.md) - the five gaps closed just before this one, and the
  inert-test mistake this repeats
- [1223](1223-rfc-gate-regression-ratchets.md) - the ratchets that forced the audit verdict to
  be re-judged when the tagged tests changed
- `docs/architecture/wire/mp-nlri-ordering.md` - where requirement 2 is enforced, and the
  deliberate requirement-1 divergence that stays

## Files

None recorded.
