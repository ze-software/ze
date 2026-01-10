# Spec: Route Family Keyword Validation

## MANDATORY READING (BEFORE IMPLEMENTATION)

```
┌─────────────────────────────────────────────────────────────────┐
│  STOP. Read these files FIRST before ANY implementation:        │
│                                                                 │
│  1. .claude/ESSENTIAL_PROTOCOLS.md - Session rules, TDD         │
│  2. .claude/INDEX.md - Find what docs to load                   │
│  3. docs/plan/CLAUDE_CONTINUATION.md - Current state                 │
│  4. THIS SPEC FILE - Design requirements                        │
│  5. pkg/config/static_*.go - Current implementation             │
│                                                                 │
│  DO NOT PROCEED until all are read and understood.              │
└─────────────────────────────────────────────────────────────────┘
```

## Task

Complete keyword validation for remaining BGP address families (FlowSpec, VPLS, L2VPN/EVPN).

## Current State

| Family | Status |
|--------|--------|
| IPv4/IPv6 Unicast | ✅ Validated |
| MPLS (Labeled Unicast) | ✅ Validated |
| L3VPN (IPv4/IPv6 VPN) | ✅ Validated |
| FlowSpec | ✅ Validated |
| VPLS | ✅ Validated |
| L2VPN/EVPN | ✅ Validated |

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- **FIRST:** Run `git status` - if modified files exist, ASK user before proceeding
- **FIRST:** Read `docs/plan/CLAUDE_CONTINUATION.md` for current state
- **FIRST:** Read `.claude/ESSENTIAL_PROTOCOLS.md` for session rules
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof
- For BGP code: Read RFC first, check ExaBGP reference
- Tests passing is NOT permission to commit - wait for user

### From ESSENTIAL_PROTOCOLS.md
- TDD is BLOCKING: Tests must exist and fail before implementation
- Check ExaBGP before implementing for API compatibility
- RFC compliance is NON-NEGOTIABLE

### RFCs to Read
- RFC 5575: FlowSpec
- RFC 4761: VPLS
- RFC 7432: EVPN

## Codebase Context

### Files to Modify

| File | Changes |
|------|---------|
| `pkg/plugin/route.go` | Add keyword sets, update handlers |
| `pkg/plugin/route_parse_test.go` | Add validation tests |

### Keyword Sets to Implement

```go
var FlowSpecKeywords = KeywordSet{
    "rd", "next-hop",
    "source", "destination", "protocol", "port",
    "source-port", "destination-port",
    "icmp-type", "icmp-code", "tcp-flags",
    "packet-length", "dscp", "fragment",
    "rate-limit", "redirect", "mark", "action",
    "extended-community",
}

var VPLSKeywords = KeywordSet{
    "rd", "rt", "ve-id", "ve-block-offset", "ve-block-size",
    "label-block-offset", "label-block-size", "next-hop",
    "extended-community",
}

var L2VPNKeywords = KeywordSet{
    "rd", "rt", "next-hop", "label", "esi", "ethernet-tag",
    "mac", "ip", "extended-community",
}
```

## Implementation Steps

### Phase 1: FlowSpec Validation
1. Read RFC 5575 FlowSpec sections
2. Check ExaBGP FlowSpec handler
3. Write tests for FlowSpec keyword validation - MUST FAIL
4. Define `FlowSpecKeywords` set
5. Update `handleAnnounceFlow` to validate keywords
6. Run tests - MUST PASS
7. Run `make test`

### Phase 2: VPLS Validation
1. Read RFC 4761 VPLS sections
2. Check ExaBGP VPLS handler
3. Write tests for VPLS keyword validation - MUST FAIL
4. Define `VPLSKeywords` set
5. Update `handleAnnounceVPLS` to validate keywords
6. Run tests - MUST PASS
7. Run `make test`

### Phase 3: L2VPN/EVPN Validation
1. Read RFC 7432 EVPN sections
2. Check ExaBGP L2VPN handler
3. Write tests for L2VPN keyword validation - MUST FAIL
4. Define `L2VPNKeywords` set
5. Update `handleAnnounceL2VPN` to validate keywords
6. Run tests - MUST PASS
7. Run `make test`

## Verification Checklist

- [x] RFC sections read for each family
- [x] ExaBGP handlers checked for compatibility
- [x] TDD followed: Tests shown to FAIL first
- [x] Invalid keywords rejected with clear error
- [x] Valid keywords accepted and parsed
- [x] `make test` passes
- [x] `make lint` passes

## Priority

| Phase | Priority | Reason |
|-------|----------|--------|
| FlowSpec | Medium | Used for DDoS mitigation |
| VPLS | Low | Specialized use case |
| L2VPN/EVPN | Low | Specialized use case |
