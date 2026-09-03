# RFC MAY Clause Decisions

This file documents decisions made for RFC "MAY" clauses (optional behavior).

---

## RFC 4760 Section 7 - Non-Negotiated AFI/SAFI Handling

**RFC Text:** Section 7, "Error Handling". Corrected on 2026-09-03: this entry
cited Section 6 and quoted a sentence that is not in RFC 4760. Section 6 defines
SAFI values, and no text anywhere in the document reads "MAY treat this as an
error". The MAY this entry decides is the last of the three remedies below.

> "If a BGP speaker receives from a neighbor an UPDATE message that contains the
> MP_REACH_NLRI or MP_UNREACH_NLRI attribute, and if the speaker determines that
> the attribute is incorrect, the speaker MUST delete all the BGP routes received
> from that neighbor whose AFI/SAFI is the same as the one carried in the
> incorrect MP_REACH_NLRI or MP_UNREACH_NLRI attribute. For the duration of the
> BGP session over which the UPDATE message was received, the speaker then SHOULD
> ignore all the subsequent routes with that AFI/SAFI received over that session.
>
> In addition, the speaker MAY terminate the BGP session over which the UPDATE
> message was received. The session SHOULD be terminated with the Notification
> message code/subcode indicating 'UPDATE Message Error'/'Optional Attribute
> Error'."

Section 7 grades three remedies, and the operator chooses only the last:

| Level | Action | Ze |
|-------|--------|----|
| MUST | delete every route from that neighbor for that AFI/SAFI | both modes: none is ever taken |
| SHOULD | ignore subsequent routes with that AFI/SAFI for the session | ignore mode, for the life of the session |
| MAY | terminate the session, with "UPDATE Message Error"/"Optional Attribute Error" | default only |

**Decision:** Config option with strict default

**Implementation:**
- **Default behavior:** take the MAY. NOTIFICATION 3/9, then close.
- **Config option:** `family { ignore-mismatch enable; }`, or `mode ignore` on one
  family, declines the MAY. The UPDATE is DROPPED and the session stays up, so no
  route of that AFI/SAFI reaches the RIB or the forward rails.

The drop is per MESSAGE, not per attribute. An UPDATE carrying an unnegotiated MP
attribute alongside a negotiated IPv4 NLRI field loses the IPv4 half too. No
conformant peer sends one: Section 8 obliges the sender to advertise the family
before it uses it. Rebuilding the message on the receive path to strip one
attribute is not worth what it costs.

**Rationale:**
1. RFC-correct default (terminate) ensures protocol compliance
2. Config option allows compatibility with buggy peers
3. User explicitly opts into lenient mode
4. Lenient never means permissive: until 2026-09-03 the lenient branch logged and
   let the whole UPDATE through, which installed unnegotiated NLRI and is the one
   outcome Section 7's MUST forbids

**Config Example:**
```
peer upstream1 {
    remote {
        ip 192.0.2.1;
        as 65001;
    }
    family {
        ipv4/unicast;
        ipv6/unicast;
        ignore-mismatch enable;  # For buggy peers
    }
}
```

**Files:**
- Config: `internal/component/bgp/reactor/config.go` - `ignore-mismatch` and the
  per-family `mode ignore`, which set `PeerSettings.IgnoreFamilyMismatch` and
  `PeerSettings.IgnoreFamilies`
- Validation: `internal/component/bgp/reactor/session_validation.go` -
  `validateUpdateFamilies`, which answers refuse, drop or accept
- Enforcement: `internal/component/bgp/reactor/session_read.go` - `processMessage`,
  which turns that answer into a NOTIFICATION or a silent drop before dispatch
<!-- source: internal/component/bgp/reactor/session_validation.go -- validateUpdateFamilies -->
<!-- source: internal/component/bgp/reactor/config.go -- IgnoreFamilyMismatch -->

---

## RFC 7606 Section 5.1 - MP Attribute Placement

**RFC Text (RFC 7606, updates RFC 4271 and RFC 4760):**
> "The MP_REACH_NLRI or MP_UNREACH_NLRI attribute (if present) SHALL be
> encoded as the very first path attribute in an UPDATE message."
>
> "An UPDATE message MUST NOT contain more than one of the following:
> non-empty Withdrawn Routes field, non-empty Network Layer Reachability
> Information field, MP_REACH_NLRI attribute, and MP_UNREACH_NLRI attribute."

**Decision:** Half-compliant. MP_UNREACH first (compliant). MP_REACH last
(intentionally non-compliant -- better for streaming parsers).

**Implementation:**
Ze orders attributes as: MP_UNREACH_NLRI (15) first (when present), regular
attributes by type code, MP_REACH_NLRI (14) last (when present). RFC 7606
prohibits both in the same UPDATE, so only one is present per message.

MP_UNREACH is placed first per RFC 7606. MP_REACH is placed last to maintain
the withdrawal-first principle from ze's original design. RFC 7606 says
MP_REACH SHALL be first, and other implementations are optimized for that
ordering. Ze's non-compliance may prevent fast-path optimizations in receivers
that expect MP_REACH first. This is a conscious trade-off.

**History:** RFC 4271 Section 5 recommended (SHOULD) ordering by type code.
RFC 4760 assigned type code 14 to MP_REACH (announcements) and 15 to
MP_UNREACH (withdrawals), which would place announcements before withdrawals
when sorted by type code. This contradicted RFC 4271's Withdrawn-before-NLRI
wire format design. RFC 7606 resolved this by requiring MP attributes first
(SHALL, overriding the SHOULD) and prohibiting both in the same UPDATE.

**Files:**
- `docs/architecture/wire/mp-nlri-ordering.md` - full analysis
- `internal/component/bgp/message/update_build.go` - attribute ordering
<!-- source: internal/component/bgp/message/update_build.go -- attribute ordering -->

---

## Template for Future Decisions

### RFC NNNN Section X.Y - Feature Name

**RFC Text:**
> "The speaker MAY ..."

**Decision:** [Always/Never/Config option]

**Implementation:** [Description]

**Rationale:** [Why this choice]

**Files:** [Affected files]
