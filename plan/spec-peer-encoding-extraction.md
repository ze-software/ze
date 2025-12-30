# Spec: Peer Encoding Extraction

## MANDATORY READING (BEFORE IMPLEMENTATION)

```
┌─────────────────────────────────────────────────────────────────┐
│  STOP. Read these files FIRST before ANY implementation:        │
│                                                                 │
│  1. .claude/ESSENTIAL_PROTOCOLS.md - Session rules, TDD         │
│  2. .claude/INDEX.md - Find what docs to load                   │
│  3. plan/CLAUDE_CONTINUATION.md - Current state                 │
│  4. THIS SPEC FILE - Design requirements                        │
│  5. pkg/reactor/peer.go - Current implementation                │
│                                                                 │
│  DO NOT PROCEED until all are read and understood.              │
└─────────────────────────────────────────────────────────────────┘
```

## Task

Extract ~900 LOC of UPDATE building logic from `pkg/reactor/peer.go` to `pkg/bgp/message/` for better separation of concerns and testability.

## Problem

`peer.go` (1725 LOC) mixes wire encoding with connection management:
- Violates SRP: UPDATE building mixed with TCP lifecycle
- Has bugs: Inconsistent attribute ordering between functions (MED/LOCAL_PREF)
- Duplicates code: Doesn't use existing `PackAttributesOrdered()`
- High risk: New AFIs require modifying connection code

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- **FIRST:** Run `git status` - if modified files exist, ASK user before proceeding
- **FIRST:** Read `plan/CLAUDE_CONTINUATION.md` for current state
- **FIRST:** Read `.claude/ESSENTIAL_PROTOCOLS.md` for session rules
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof
- For BGP code: Read RFC first, check ExaBGP reference
- Tests passing is NOT permission to commit - wait for user

### From ESSENTIAL_PROTOCOLS.md
- TDD is BLOCKING: Tests must exist and fail before implementation
- RFC 4271 compliance is NON-NEGOTIABLE for attribute ordering
- Use `PackAttributesOrdered()` for RFC 4271 Appendix F.3 compliance
- One function = one commit during refactoring
- PASTE EXACT OUTPUT - no summaries

### From RFC 4271
- Section 5: Path attributes SHOULD be ordered by type code
- Appendix F.3: Canonical attribute ordering (ORIGIN=1, AS_PATH=2, NEXT_HOP=3, MED=4, LOCAL_PREF=5...)

## Codebase Context

### Files to Modify

| File | Change |
|------|--------|
| `pkg/bgp/message/update_build.go` | Create - new builder functions |
| `pkg/bgp/message/update_build_test.go` | Create - TDD tests |
| `pkg/reactor/peer.go` | Modify - use new builders, remove extracted code |

### What Should Move to `message/`

| Function | Purpose |
|----------|---------|
| `buildStaticRouteUpdate` | IPv4/IPv6/VPN UPDATE |
| `buildGroupedUpdate` | Grouped UPDATE |
| `buildMPReachNLRI*` | MP_REACH variants |
| `buildVPNNLRIBytes` | VPN NLRI |
| `build*Update` (MVPN, VPLS, FlowSpec, MUP) | Family-specific UPDATEs |
| `routeGroupKey` | Grouping key |
| `groupRoutesByAttributes` | Route grouping |

### What Stays in Reactor

| Function | Reason |
|----------|--------|
| `sendInitialRoutes` | Uses PeerSettings, orchestrates |
| `send*Routes` | Uses PeerSettings, sends EOR |

## Implementation Steps

### Phase 0: Fix Attribute Ordering Bug (IMMEDIATE)
1. Write test to verify attribute ordering in `buildGroupedUpdate`
2. Run test - MUST FAIL showing MED/LOCAL_PREF out of order
3. Fix ordering OR use `attribute.PackAttributesOrdered()`
4. Run test - MUST PASS

### Phase 1: Create UpdateBuilder Context Type
1. Write test for `UpdateBuilder` struct with `LocalAS`, `IsIBGP`, `ASN4` fields
2. Create `pkg/bgp/message/update_build.go` with `UpdateBuilder` struct
3. Add `UnicastParams` struct with primitive types (no reactor imports)

### Phase 2: Extract Unicast Builder (TDD)
1. Write `TestBuildUnicast_IPv4` - MUST FAIL
2. Write `TestBuildUnicast_IPv6` - MUST FAIL
3. Write `TestBuildUnicast_AttributeOrder` - MUST FAIL
4. Implement `BuildUnicast()` using `PackAttributesOrdered()`
5. Run tests - MUST PASS
6. Update `peer.go` to use builder
7. Run full `make test`

### Phase 3-7: Extract Remaining Builders (TDD each)
- Phase 3: VPN builder
- Phase 4: MVPN builder
- Phase 5: VPLS builder
- Phase 6: FlowSpec builder
- Phase 7: MUP builder

### Phase 8: Cleanup & Verification
1. Remove extracted functions from `peer.go`
2. Add `toXxxParams()` conversion helpers in reactor
3. Verify `make test && make lint` passes
4. Verify byte-identical output (regression tests)

## Verification Checklist

- [ ] Phase 0: Attribute ordering bug fixed with test
- [ ] TDD followed: Each test shown to FAIL first, then PASS
- [ ] `peer.go` reduced to ~750 LOC
- [ ] No wire format changes (byte-identical output verified)
- [ ] All builders use `PackAttributesOrdered()`
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] Each builder has comprehensive tests with VALIDATES/PREVENTS docs

## Effort Estimate

| Phase | Effort | Risk |
|-------|--------|------|
| 0. Fix ordering bug | 0.5h | Low |
| 1. Create UpdateBuilder | 1h | Low |
| 2. Unicast extraction | 3h | Medium |
| 3-7. Other builders | 8h | Low-Medium |
| 8. Cleanup & verify | 2h | Low |
| **Total** | **~15h** | |

## Success Criteria

1. `peer.go` reduced to ~750 LOC (connection + orchestration)
2. All existing tests pass
3. No wire format changes (byte-identical output)
4. Attribute ordering is correct in all builders (test verified)
5. New address families can be added in `message/` without touching `peer.go`
