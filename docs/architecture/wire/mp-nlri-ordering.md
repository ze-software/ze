# MP_REACH_NLRI and MP_UNREACH_NLRI Ordering

## Key Insight

**MP_REACH_NLRI and MP_UNREACH_NLRI are NOT attributes in the semantic sense.**

They are NLRI encoded as attributes - a wire format hack to carry non-IPv4 address families in the Path Attributes field (RFC 4760).

<!-- source: internal/component/bgp/attribute/attribute.go -- AttrMPReachNLRI (14), AttrMPUnreachNLRI (15) -->

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
<!-- source: internal/component/bgp/attribute/mpnlri.go -- MP_REACH_NLRI, MP_UNREACH_NLRI encoding -->

**Rationale:** Withdrawals logically precede announcements. Regular path
attributes describe the NLRI in MP_REACH, so they appear between the two.

## Compatibility

This ordering is valid because:

1. All regular attributes remain in RFC order
2. MP attributes at end is common in implementations
3. BGP speakers MUST accept attributes in any order (RFC 4271 Section 5)
4. Withdrawals before announcements is logical (remove old, add new)

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
