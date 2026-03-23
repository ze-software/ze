# RFC MAY Clause Decisions

This file documents decisions made for RFC "MAY" clauses (optional behavior).

---

## RFC 4760 Section 6 - Non-Negotiated AFI/SAFI Handling

**RFC Text:**
> "If a BGP speaker receives an UPDATE with MP_REACH_NLRI or MP_UNREACH_NLRI
> where the AFI/SAFI do not match those negotiated in OPEN, the speaker
> MAY treat this as an error."

**Decision:** Config option with strict default

**Implementation:**
- **Default behavior:** Treat as error → send NOTIFICATION
- **Config option:** `family { ignore-mismatch enable; }` → log warning, skip NLRI

**Rationale:**
1. RFC-correct default (error) ensures protocol compliance
2. Config option allows compatibility with buggy peers
3. User explicitly opts into lenient mode

**Config Example:**
```
peer 192.0.2.1 {
    family {
        ipv4/unicast;
        ipv6/unicast;
        ignore-mismatch enable;  # For buggy peers
    }
}
```

**Files:**
- Config: `internal/component/config/bgp.go` - `NeighborConfig.IgnoreFamilyMismatch`
- Validation: `internal/component/bgp/reactor/session.go` - `handleUpdate()` (pending)
<!-- source: internal/component/bgp/reactor/session_validation.go -- enforceRFC7606 -->
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
