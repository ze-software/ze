# Spec: Route Reflection via API - Overview

## Status

- **Phase 1 (PackWithContext):** ✅ COMPLETE - merged
- **Phase 2:** Split into self-contained specs (see below)

## Architecture

ZeBGP implements route reflection through the API, not internally:

```
Peer A → Receive → Store (wire + route-id + role) → API output
                                                        ↓
                                           External process decides
                                                        ↓
                            API command: "peer !<source> forward route-id 123"
                                                        ↓
Peer B,C ← Zero-copy forward ← Lookup by route-id
```

## Self-Contained Specs

| Spec | Description | Dependencies |
|------|-------------|--------------|
| `spec-attributes-wire.md` | AttributesWire type - wire-canonical storage with lazy parsing | None |
| `spec-route-id-forwarding.md` | Route ID, forward command, `!<ip>` selector | spec-attributes-wire |
| `spec-api-attribute-filter.md` | `attributes <list>` config for partial parsing | spec-attributes-wire |
| `spec-rfc9234-role.md` | RFC 9234 Role capability for API policy | None |

## Implementation Order

```
1. spec-attributes-wire.md      (foundation - lazy parsing type)
        ↓
2. spec-route-id-forwarding.md  (uses AttributesWire.PackFor)
   spec-api-attribute-filter.md (uses AttributesWire.GetMultiple)
        ↓
3. spec-rfc9234-role.md         (independent, can be done anytime)
```

## Key Concepts

### Wire-Canonical Storage

Routes store wire bytes as canonical form. Parsing is lazy (on demand):
```go
attrs.Get(AttrASPath)        // Parse just AS_PATH
attrs.GetMultiple(codes)     // Parse subset for API
attrs.Packed()               // Zero-copy forward
```

### Route ID

Each received route gets unique ID for API reference:
```json
{ "route-id": 12345, "nlri": {...} }
```

Forward by ID:
```
peer !10.0.0.1 forward route-id 12345
```

### RFC 9234 Role

Routes tagged with source peer's role for policy:
```json
{ "route-id": 12345, "tag": { "role": "customer" } }
```

Policy without attribute parsing:
- Customer routes → can go anywhere
- Provider/Peer routes → customers only

### Attribute Filtering

API config limits which attributes are parsed/output:
```
api minimal {
    content {
        attributes as-path next-hop;
    }
}
```

## Summary Table

| Feature | Spec | Status |
|---------|------|--------|
| AttributesWire type | spec-attributes-wire.md | Ready |
| Route ID field | spec-route-id-forwarding.md | Ready |
| `forward route-id` command | spec-route-id-forwarding.md | Ready |
| `!<ip>` selector | spec-route-id-forwarding.md | Ready |
| `attributes <list>` config | spec-api-attribute-filter.md | Ready |
| RFC 9234 Role capability | spec-rfc9234-role.md | Ready |
| OTC attribute | spec-rfc9234-role.md | Ready |
| `peer [role X]` selector | spec-rfc9234-role.md | Ready |

---

**Created:** 2025-12-28
**Updated:** 2026-01-01
