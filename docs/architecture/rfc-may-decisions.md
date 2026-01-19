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
- Config: `internal/config/bgp.go` - `NeighborConfig.IgnoreFamilyMismatch`
- Validation: `internal/reactor/session.go` - `handleUpdate()` (pending)

---

## Template for Future Decisions

### RFC NNNN Section X.Y - Feature Name

**RFC Text:**
> "The speaker MAY ..."

**Decision:** [Always/Never/Config option]

**Implementation:** [Description]

**Rationale:** [Why this choice]

**Files:** [Affected files]
