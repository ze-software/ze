# Spec: Link-Local Nexthop Capability (Code 77)

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `rfc/short/rfc8950.md` - Extended nexthop (related)
3. `internal/plugin/bgp/capability/` - Existing capability code

## Task

Implement BGP Link-Local Nexthop Capability (code 77) as defined in draft-ietf-idr-linklocal-capability.

## Status

**DEFERRED** - Wait until draft becomes RFC.

## Reference

| Field | Value |
|-------|-------|
| Draft | [draft-ietf-idr-linklocal-capability-02](https://datatracker.ietf.org/doc/draft-ietf-idr-linklocal-capability/) |
| Capability Code | 77 (0x4D) |
| Capability Length | 0 |
| Updates | RFC 2545 |
| Status | IETF Working Group Draft |

## Purpose

Allow IPv6 link-local-only next hops (without global address) in BGP UPDATE messages.

**Use case:** Data center fabrics (RFC 7938) where BGP runs on point-to-point links and only link-local addresses are available.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/capabilities.md` - Capability wire format
- [ ] `rfc/short/rfc8950.md` - Extended nexthop (related capability)
- [ ] `rfc/short/rfc2545.md` - IPv6 nexthop encoding (updated by this draft)

### Source Files
- [ ] `internal/plugin/bgp/capability/capability.go` - Capability parsing
- [ ] `internal/plugin/bgp/capability/negotiated.go` - Capability negotiation
- [ ] `internal/plugin/bgp/capability/encoding.go` - Capability encoding

## Wire Format

### Capability Advertisement

```
+---------------------------+
| Cap Code = 77 (1 octet)   |
+---------------------------+
| Cap Length = 0 (1 octet)  |
+---------------------------+
```

No capability value - presence indicates support.

### MP_REACH_NLRI Next Hop Encoding

When link-local capability is negotiated:

| Next Hop Length | Content |
|-----------------|---------|
| 16 | IPv6 global address (standard) |
| 16 | IPv6 link-local address (NEW - with this capability) |
| 32 | IPv6 global + IPv6 link-local |

**Key change:** Length 16 can now be link-local (fe80::/10) not just global.

## Implementation Plan

### Phase 1: Capability Parsing/Encoding

1. Add `LinkLocalNextHop` capability type
2. Add parsing in `capability.go`
3. Add encoding in `encoding.go`
4. Add negotiation in `negotiated.go`

### Phase 2: Config Support

1. Add `link-local-nexthop` to capability config block
2. Update YANG schema

### Phase 3: Next Hop Handling

1. Update MP_REACH_NLRI parsing to accept link-local with length 16
2. Update next hop validation to allow link-local when negotiated
3. Update UPDATE building to use link-local when appropriate

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| `TestLinkLocalCapParse` | `capability_test.go` | Parse cap code 77 |
| `TestLinkLocalCapEncode` | `encoding_test.go` | Encode cap code 77 |
| `TestLinkLocalCapNegotiate` | `negotiated_test.go` | Negotiate link-local |
| `TestLinkLocalNextHop` | `mpnlri_test.go` | Parse link-local NH |

### Functional Tests

| Test | File | Validates |
|------|------|-----------|
| `linklocal-cap` | `test/encode/linklocal-cap.ci` | Capability in OPEN |
| `linklocal-nexthop` | `test/encode/linklocal-nexthop.ci` | Link-local in UPDATE |

## Files to Modify

- `internal/plugin/bgp/capability/capability.go` - Add LinkLocalNextHop type
- `internal/plugin/bgp/capability/encoding.go` - Encode cap 77
- `internal/plugin/bgp/capability/negotiated.go` - Negotiate link-local
- `internal/plugin/bgp/schema/ze-bgp.yang` - Config syntax
- `internal/config/bgp.go` - Parse config

## Files to Create

- `test/encode/linklocal-cap.ci` - Capability test
- `test/encode/linklocal-nexthop.ci` - Next hop test

## Implementation Steps

1. **Write unit tests** - Create tests for capability parsing/encoding
   → **Review:** Tests cover parse, encode, negotiate?

2. **Run tests** - Verify FAIL (paste output)
   → **Review:** Tests fail for the right reason?

3. **Implement capability** - Add LinkLocalNextHop to capability system
   → **Review:** Follows existing capability patterns?

4. **Run tests** - Verify PASS (paste output)
   → **Review:** All tests pass?

5. **Add config support** - YANG schema and config parsing
   → **Review:** Config syntax matches other capabilities?

6. **Add functional tests** - End-to-end capability and nexthop tests
   → **Review:** Tests verify wire format?

7. **Verify all** - `make lint && make test && make functional`
   → **Review:** Zero issues?

## Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2025-01-28 | DEFER | Wait for draft to become RFC |

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes
