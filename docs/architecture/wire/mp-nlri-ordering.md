# MP_REACH_NLRI and MP_UNREACH_NLRI Ordering

## Key Insight

**MP_REACH_NLRI and MP_UNREACH_NLRI are NOT attributes in the semantic sense.**

They are NLRI encoded as attributes - a wire format hack to carry non-IPv4 address families in the Path Attributes field (RFC 4760).

<!-- source: internal/core/bgp/attribute/attribute.go -- AttrMPReachNLRI (14), AttrMPUnreachNLRI (15) -->

## Why This Matters

Path attributes (ORIGIN, AS_PATH, NEXT_HOP, COMMUNITIES, etc.) **apply to** NLRI. Semantically:

```
Attributes → describe → NLRI
```

The RFC 4271 ordering requirement for attributes is noted as overly strict, since the attributes describe the NLRI that follows. Placing MP_REACH/MP_UNREACH at the end makes logical sense.

## Generation Strategy

When building UPDATE messages:

1. **MP_UNREACH_NLRI first** (withdrawals)
2. **Regular path attributes** (ordered by type code per RFC 4271)
3. **MP_REACH_NLRI last** (announcements)

```
┌─────────────────────────────────────────┐
│ MP_UNREACH_NLRI (15) - withdrawals      │
├─────────────────────────────────────────┤
│ Regular Path Attributes (ordered)       │
│   - ORIGIN (1)                          │
│   - AS_PATH (2)                         │
│   - NEXT_HOP (3) - for IPv4 only        │
│   - ... other attributes in order ...   │
├─────────────────────────────────────────┤
│ MP_REACH_NLRI (14) - announcements      │
└─────────────────────────────────────────┘
```

<!-- source: internal/component/bgp/message/update_build.go -- attribute ordering in UPDATE building -->
<!-- source: internal/core/bgp/attribute/mpnlri.go -- MP_REACH_NLRI, MP_UNREACH_NLRI encoding -->

**Rationale:** Withdrawals logically precede announcements. Regular path
attributes describe the NLRI in MP_REACH, so they appear between the two.

## RFC Compliance Analysis

**RFC 4271 Section 5** recommends (SHOULD) ordering path attributes by ascending
type code. RFC 4760 assigned type code 14 to MP_REACH_NLRI and 15 to
MP_UNREACH_NLRI. Strict type-code ordering would place announcements (14)
before withdrawals (15), contradicting the withdrawal-first principle that
RFC 4271 Section 4.3 established by placing Withdrawn Routes before NLRI in
the UPDATE wire format. This was an oversight in RFC 4760's type code
assignment.

**RFC 7606 Section 5.1** (the major BGP error handling fixup RFC, updates both
RFC 4271 and RFC 4760) addresses this with two requirements:

1. "The MP_REACH_NLRI or MP_UNREACH_NLRI attribute (if present) SHALL be
   encoded as the very first path attribute in an UPDATE message."

2. "An UPDATE message MUST NOT contain more than one of the following:
   non-empty Withdrawn Routes field, non-empty Network Layer Reachability
   Information field, MP_REACH_NLRI attribute, and MP_UNREACH_NLRI attribute."

Requirement 2 means MP_REACH and MP_UNREACH **cannot appear in the same
UPDATE** per RFC 7606. Each UPDATE carries either announcements or withdrawals
for multiprotocol families, never both. This eliminates the intra-message
ordering question entirely.

Requirement 1 means whichever MP attribute is present goes first, before all
regular attributes. This supersedes RFC 4271's type-code ordering for these
attributes.

### Where requirement 2 is enforced

The obligation binds the SENDER, and ze is the sender of the bytes it relays
even when another speaker composed them. "We only forward what we received" is
a policy choice, not an exemption. So the restriction is enforced at every point
where ze puts an UPDATE on the wire, not only where it builds one:

| Point | Enforcement |
|-------|-------------|
| Origination | Compliant by construction: announcements set NLRI without Withdrawn Routes, withdrawals are withdraw-only |
| Re-chunking an oversized UPDATE | `buildCombinedUpdates` and `Splitter.splitUpdateWithMP` drain each component into its own message |
| Relay, same context | `buildFwdBody` splits when the shape mixes, as well as when the size overflows |
| Relay, re-encoded for the destination | Same, via `Splitter.SplitCompliant` |

`message.NLRIBearingFieldCount` is the single definition of how many of the four
fields an UPDATE carries; both the wire and parsed paths call it rather than
each keeping their own notion of "mixed".

The relay check has to be cheap, because a route reflector runs it once per
destination. `wireu.WireUpdate.MixesNLRIFields` caches the verdict on the
received UPDATE, whose pointer is shared across the per-peer forward loop, so
the attribute walk is paid once per message rather than once per peer (measured
3.3ns against 51.7ns, both allocation-free). A compliant single-field UPDATE
therefore still takes the zero-copy path: the received bytes are handed to the
destination without a parse or a copy.

<!-- source: internal/component/bgp/message/rfc7606_shape.go -- NLRIBearingFieldCount, the single definition -->
<!-- source: internal/component/bgp/reactor/forward_body.go -- buildFwdBody, both relay branches -->
<!-- source: internal/component/bgp/wireu/split.go -- SplitWireUpdate, shape as well as size -->

Receiving is deliberately unaffected: requirement 3 of the same section obliges
ze to accept these fields in any position or combination, and it does.

**Ze meets requirement 2 and diverges on half of requirement 1:**

- **MP_UNREACH first:** Compliant. Withdrawal is the first attribute, matching
  both RFC 7606's SHALL and RFC 4271's withdrawal-first wire format.

- **MP_REACH last:** Intentionally non-compliant. RFC 7606 says it SHALL be
  first, but ze places it after all regular path attributes. In theory, a
  streaming parser could benefit from having attributes parsed before NLRI
  arrives. In practice, receivers are optimized for MP_REACH first (what
  RFC 7606 mandates and what other implementations send). Ze's ordering
  may prevent those fast-path optimizations. This is a conscious trade-off
  that prioritizes the withdrawal-first principle from ze's original design
  over alignment with receiver expectations.

## Compatibility

Ze must still handle legacy UPDATEs that combine MP_REACH and MP_UNREACH (pre-
RFC 7606 implementations). RFC 7606 Section 5.1 notes: "Since older BGP
speakers may not implement these restrictions, an implementation MUST still be
prepared to receive these fields in any position or combination."

For ze's own outbound UPDATEs, the ordering is:
1. RFC 4271 Section 5: receivers MUST accept attributes in any order
2. RFC 7606 Section 5.1: MP attributes SHALL be first (ze complies)
3. The SHOULD for type-code ordering is overridden by the SHALL in RFC 7606

## Implementation Notes

- When splitting large MP_REACH_NLRI, regenerate the MP attribute with chunked NLRI
- Regular attributes can be copied verbatim to each split UPDATE
- MP_UNREACH_NLRI splitting follows the same pattern

<!-- source: internal/component/bgp/message/update_split.go -- UPDATE message splitting -->
<!-- source: internal/component/bgp/message/chunk_mp_nlri.go -- MP NLRI chunking -->

## References

- RFC 4760 Section 3: MP_REACH_NLRI format
- RFC 4760 Section 4: MP_UNREACH_NLRI format
- RFC 4271 Section 5: "A BGP speaker MUST be prepared to accept attributes in any order"

---

**Created:** 2026-01-01
