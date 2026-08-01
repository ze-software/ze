# 776: SRv6 Prefix-SID (RFC 9252, RFC 8669)

Spec: spec-fib-depth-4-srv6

## Context

Ze needed SRv6 ingress PE support: receive BGP routes with PrefixSID attribute
(code 40), extract SRv6 SIDs, and program FIB backends (kernel SEG6, VPP SR steer).
The spec originally listed `bgp-nlri-srv6` as a blocker, assuming a new NLRI family
plugin was required. Analysis showed this was wrong: SRv6 for unicast/VPN uses
attribute code 40 (optional-transitive), not a new address family.

## Decisions

**Lazy extraction from OtherAttrs, not eager parsing.** The wire parser already
stores unrecognized attributes (including PrefixSID) as opaque bytes in OtherAttrs.
Rather than adding a full attribute parser at UPDATE reception time, the SRv6 SID
is extracted at best-path emission time from OtherAttrs. This avoids allocation on
the per-UPDATE hot path for the 99%+ of routes that carry no SRv6 SID.

**RFC 7606 validator catches malformed TLVs early.** A `validatePrefixSIDAttr`
registered at `attrValidators[40]` detects structural errors (TLV overrun, Service
TLV too short, SID Info Sub-TLV length < 21) at UPDATE reception. Malformed SRv6
Service TLVs trigger treat-as-withdraw (RFC 9252 Section 3.4); structural attribute
errors trigger attribute-discard (RFC 8669 Section 6).

**SID resolvability reuses NH resolver.** The SRv6 SID is tracked via the same
`nhResolver.Track()/Resolve()` mechanism as next-hops. When the covering route for
a SID appears or disappears, `cascadeRecompute` re-evaluates all dependent prefixes.
No new resolution framework was needed.

**Transposition handled at best-path emission.** For VPN (SAFI 128) and EVPN
(SAFI 70), SID function bits are split between the NLRI label field and the SID
Information Sub-TLV. `lookupSRv6SIDForBest` calls `ApplyTransposition` only when
`HasTranspos && needsTransposition(fam)`, combining the partial SID with the NLRI
label bits at the bit offset specified by the SID Structure Sub-Sub-TLV.

**EBGP filtering via existing attr-discard mechanism.** RFC 8669 Section 4 requires
discarding PrefixSID from EBGP unless configured. Implemented as an extension to
`enforceRFC7606`: if EBGP and `AcceptSRv6PrefixSID` is false, code 40 is added to
`DiscardEntries`. Uses the in-place `ApplyAttrDiscard` rewriting (no copy).

**Propagation uses existing suppress-on-NH-change.** RFC 9252 Section 3.3 requires
stripping PrefixSID when re-advertising with a changed next-hop. Implemented as
`mods.Op(40, AttrModSuppress, nil)` in `applyFactsNextHop`. The zero-copy forward
path (ContextID match) already preserves PrefixSID unchanged.

## Consequences

- Routes with SRv6 Service TLVs but no extractable valid SID are excluded from
  best-path selection (`IsSRv6Ineligible` in `gatherCandidatesLocked`).
- SRv6 SID resolution suppresses FIB emission until a covering route exists. The
  route stays in sysrib's `best` map but no outgoing event is emitted until the
  SID becomes reachable.
- fakefib test plugin extended with `srv6-sid` parameter for functional testing.
- VPP SR steer requires vendored `go.fd.io/govpp/binapi/sr` and `sr_types`.
- Ze does not originate local SRv6 SIDs. It is ingress-PE only.

## Traps

- **Reserved bytes in RFC 9252 wire format.** Both the Service TLV value and the
  SID Info Sub-TLV value start with a 1-byte Reserved field before the payload.
  Early implementation missed these, causing off-by-one SID extraction. Caught by
  RFC compliance review against `rfc/short/rfc9252.md`.
- **Spec blocker was a misconception.** The dependency on `bgp-nlri-srv6` assumed
  SRv6 needed a new NLRI family plugin. In reality, SRv6 for unicast/VPN is carried
  in attribute 40, which the existing opaque-attribute path already stores. The real
  gap was: PrefixSID attribute parser + RIB/sysrib plumbing, not a new NLRI plugin.
- **TransposLen=0 is valid.** When SID Structure Sub-Sub-TLV has TransposLen=0, it
  means no transposition (entire SID in Sub-TLV). The parser must return
  HasTranspos=false in this case, not apply zero-length transposition.

## Files

None recorded.
