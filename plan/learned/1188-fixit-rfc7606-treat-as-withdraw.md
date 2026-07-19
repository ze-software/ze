# 1188 -- fixit-rfc7606-treat-as-withdraw

## Context

RFC 7606 Section 2 treat-as-withdraw must remove the previously-installed route,
not merely decline to install the malformed one. The CORE of this spec was already
implemented and committed by a prior session: `message.SynthesizeWithdraw`
(`internal/component/bgp/message/rfc7606_withdraw.go`) rewrites a malformed UPDATE's
announced routes into withdrawals, and `enforceRFC7606`
(`internal/component/bgp/reactor/session_validation.go:160-179`) calls it on the
`RFC7606ActionTreatAsWithdraw` branch, rebuilds the WireUpdate (ctxID/sourceID
preserved), and falls through to the normal dispatch path. The spec's aspirational
test names never existed; coverage had real gaps. This session closed the RIB-boundary
gaps (AC-1 Loc-RIB propagation, AC-5, AC-6) and fixed a latent ADD-PATH bug those tests
exposed on the production receive path.

## Decisions

- Proved AC-1/AC-5 at the RIB boundary by feeding the REAL `message.SynthesizeWithdraw`
  output through `RIBManager.handleReceivedStructured` (the DirectBridge path the reactor
  actually uses), then asserting Adj-RIB-In removal AND the best-change Withdraw published
  on the EventBus. This pins the Loc-RIB propagation the reactor tests cannot see; the
  reactor test only proves a withdraw-shaped message was dispatched.
- Fixed the CODE rather than the test for AC-6: `handleReceivedStructured` never called
  `peerRIB.SetAddPath(fam, true)`, so the structured receive path created non-ADD-PATH
  FamilyRIBs and collapsed sibling paths of a prefix. The JSON path (`insertPoolNLRIs`,
  `rib.go:948-949`) already did this; the structured path (the primary internal-plugin
  path) silently diverged. Added `SetAddPath` in the IPv4-NLRI and MP_REACH announce
  blocks, BEFORE the first insert (the FamilyRIB is created lazily reading
  `peerRIB.addPath[fam]`), mirroring the JSON path exactly.
- Did NOT expand into the reactor for AC-8/AC-9 (out of this park's scope; the committed
  single-body synthesis design diverges from the spec's D-8/D-5). Documented both with
  producer evidence in DECISION.md and the drain recipe rather than half-fixing or hiding
  them.

## Consequences

- ADD-PATH received routes now key by (prefix, path-id) on the structured path, so
  sibling paths of a prefix are stored independently instead of overwriting each other.
  This is a correctness fix for all ADD-PATH sessions on the DirectBridge path, not just
  the treat-as-withdraw case. The full `rib` package test suite stays green.
- A treat-as-withdraw re-advertisement of an installed prefix now removes it from the
  Adj-RIB-In and publishes the withdraw to Loc-RIB consumers; a treat-as-withdraw for a
  never-installed prefix is a silent no-op (no spurious best-change event), pinned by test.

## Gotchas

- `attribute.AttributesWire.GetRaw` (`internal/core/bgp/attribute/wire.go`) returns the
  FIRST matching attribute only. `SynthesizeWithdraw`'s `withdrawMPAttrs` emits BOTH
  MP_UNREACH attributes for a two-family UPDATE (kept MP_UNREACH + converted MP_REACH),
  so the RIB reads only one family and the other stays stale (AC-8 unmet end-to-end).
- `processMessage` runs `validateUpdateFamilies` on the SYNTHESIZED body
  (`session_read.go:188`, AFTER `enforceRFC7606` at `:162`). Since `SynthesizeWithdraw`
  has no negotiation knowledge, a non-negotiated MP_REACH becomes a non-negotiated
  MP_UNREACH and strict-mode teardown fires, contradicting AC-9 (spec D-5 not implemented).
- The `rib` plugin's `storage.PeerRIB` add-path keying is set per family by `SetAddPath`
  BEFORE the first insert; setting it after the FamilyRIB exists does not retroactively
  convert it. Both receive paths must call it on the announce side.
- RFC-tagged test files cannot be edited/renamed/deleted without recorded user approval
  (`.claude/hooks/pretool-writeedit.py`). Get the test right the first time; do not add an
  `RFC requirement:` tag to a test still in flux.

## Files

- internal/component/bgp/plugins/rib/rib_structured.go (SetAddPath on structured receive path: IPv4 + MP_REACH announce blocks)
- internal/component/bgp/plugins/rib/rib_structured_test.go (NEW: TestRIBTreatAsWithdrawRemovesInstalledRoute AC-1/AC-5; TestRIBTreatAsWithdrawAddPathPreservesPathID AC-6)
- DECISION.md (findings + AC-8/AC-9 divergences, append-only)
