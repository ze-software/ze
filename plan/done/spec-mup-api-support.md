# Spec: MUP API Support

## SOURCE FILES (read before implementation)

```
┌─────────────────────────────────────────────────────────────────┐
│  Read these source files before implementing:                   │
│                                                                 │
│  1. pkg/api/route.go - parseSAFI(), handleAnnounceIPv4/IPv6     │
│  2. pkg/api/types.go - ReactorInterface                         │
│  3. pkg/reactor/reactor.go - AnnounceRoute implementation       │
│  4. pkg/reactor/peer.go:2473 - sendMUPRoutes() for reference    │
│  5. pkg/config/loader.go:1397 - convertMUPRoute(), buildMUPNLRI │
│  6. pkg/config/bgp.go:2138 - parseMUPFromInline()               │
│                                                                 │
│  NOTE: Protocol files (.claude/ESSENTIAL_PROTOCOLS.md,          │
│  .claude/INDEX.md, plan/CLAUDE_CONTINUATION.md) should have     │
│  been read at SESSION START, before /prep was invoked.          │
│                                                                 │
│  ON COMPLETION: Update design docs listed in Documentation      │
│  Impact section to match any design changes made.               │
└─────────────────────────────────────────────────────────────────┘
```

## Task

Add MUP SAFI support to API parser to fix mup4/mup6 tests.

## Current State

- Tests: 12/14 passed (mup4, mup6 timeout because API doesn't parse MUP)
- Last commit: `cbcc427` (fix: apply API bindings from match templates)
- Static MUP routes from config work (sent on session established)
- API MUP routes not supported (missing SAFI in parser)

## Context Loaded

```
📖 Context Loading Verification
├── .claude/zebgp/api/ARCHITECTURE.md - API structure, command dispatch
├── pkg/api/route.go:207 - parseSAFI() only supports unicast/nlri-mpls/mpls-vpn
├── pkg/api/route.go:951 - handleAnnounceIPv4() routes by SAFI
├── pkg/config/bgp.go:2138 - parseMUPFromInline() parses MUP text format
├── pkg/config/loader.go:1397 - convertMUPRoute() builds reactor.MUPRoute
├── pkg/reactor/peer.go:2474 - sendMUPRoutes() sends MUP routes
├── pkg/reactor/peersettings.go:155 - MUPRoute struct definition
└── test/data/api/mup4.ci - expected wire format for MUP commands
```

## Problem Analysis

### User Flow (current - BROKEN)

```
API Command: announce ipv4/mup mup-isd 10.0.1.0/24 rd 100:100 next-hop 2001::1 ...
                │
                ▼
        parseSAFI(["mup", "mup-isd", ...])
                │
                ▼
        ERROR: "unsupported SAFI: mup"
                │
                ▼
        Falls through to announceRouteImpl() (unicast)
                │
                ▼
        Sends IPv4 unicast UPDATE instead of MUP UPDATE
```

### User Flow (after fix)

```
API Command: announce ipv4/mup mup-isd 10.0.1.0/24 rd 100:100 next-hop 2001::1 ...
                │
                ▼
        parseSAFI(["mup", "mup-isd", ...])
                │
                ▼
        safi = "mup", rest = ["mup-isd", "10.0.1.0/24", ...]
                │
                ▼
        handleAnnounceIPv4 switch: case "mup"
                │
                ▼
        announceMUPImpl(ctx, rest, isIPv6=false)
                │
                ▼
        parseMUPFromArgs() - parse route type, prefix, RD, next-hop, ext-comm, prefix-sid
                │
                ▼
        reactor.AnnounceMUPRoute(peerSelector, MUPRouteSpec)
                │
                ▼
        Peer.SendMUPUpdate(route) - MP_REACH_NLRI with SAFI=85
```

### Key Files

| File | Changes |
|------|---------|
| `pkg/api/route.go` | Add "mup" to parseSAFI(), add announceMUPImpl() |
| `pkg/api/types.go` | Add AnnounceMUPRoute() to ReactorInterface |
| `pkg/reactor/reactor.go` | Implement AnnounceMUPRoute() |
| `pkg/api/route_test.go` | Add tests for MUP parsing |

## Goal Achievement

```
🎯 User's actual goal: API MUP commands produce correct MUP UPDATE messages

| Check | Status |
|-------|--------|
| parseSAFI("mup") works? | ❌ → ✅ |
| MUP attributes parsed? | ❌ → ✅ |
| MUP UPDATE sent? | ❌ → ✅ |
| mup4 test passes? | ❌ → ✅ |
| mup6 test passes? | ❌ → ✅ |

Plan achieves goal: YES
```

## Embedded Rules

- TDD: test must fail before impl
- Verify: make test && make lint before done
- RFC: MUP follows draft-mpmz-bess-mup-safi (SAFI 85)

## Documentation Impact

- [ ] `.claude/zebgp/api/ARCHITECTURE.md` - Add MUP to supported families table
- [ ] `plan/CLAUDE_CONTINUATION.md` - Update after impl

## Implementation Steps

### Phase 1: Tests (TDD)

1. Add test for `parseSAFI("mup")` returning "mup"
2. Add test for `parseMUPRouteType()` parsing mup-isd, mup-dsd, etc.
3. Add test for `parseMUPFromArgs()` extracting all attributes
4. Verify tests FAIL (no implementation yet)

### Phase 2: API Parser Implementation

1. **Add MUP to parseSAFI()** at route.go:213:
   ```go
   case SAFINameMUP:  // "mup"
       return safi, args[1:], nil
   ```

2. **Add SAFI constant**:
   ```go
   const SAFINameMUP = "mup"
   ```

3. **Add MUP route handling** in handleAnnounceIPv4/IPv6:
   ```go
   case SAFINameMUP:
       return announceMUPImpl(ctx, rest, false)  // false = IPv4
   ```

4. **Implement announceMUPImpl()**:
   - Parse route type (mup-isd, mup-dsd, mup-t1st, mup-t2st)
   - Parse prefix/address based on route type
   - Parse rd, next-hop, extended-community, bgp-prefix-sid-srv6
   - Build MUPRouteSpec
   - Call reactor.AnnounceMUPRoute()

5. **Implement withdrawMUPImpl()** similarly for withdraw commands

### Phase 3: Reactor Implementation

1. **Add to ReactorInterface** in types.go:
   ```go
   AnnounceMUPRoute(peerSelector string, route MUPRouteSpec) error
   WithdrawMUPRoute(peerSelector string, route MUPRouteSpec) error
   ```

2. **Add MUPRouteSpec** to types.go:
   ```go
   type MUPRouteSpec struct {
       RouteType  string  // mup-isd, mup-dsd, mup-t1st, mup-t2st
       IsIPv6     bool
       Prefix     string  // For ISD, T1ST
       Address    string  // For DSD, T2ST
       RD         string
       TEID       string
       QFI        uint8
       Endpoint   string
       Source     string
       NextHop    string
       ExtCommunity string
       PrefixSID  string
   }
   ```

3. **Implement AnnounceMUPRoute** in reactor.go:
   - Get matching peers (like AnnounceRoute)
   - Convert MUPRouteSpec to reactor.MUPRoute using existing helpers
   - Build MUP UPDATE using existing BuildMUP() method
   - Send to peers

### Phase 4: Verification

1. Run `make test` - all unit tests pass
2. Run `make lint` - no new issues
3. Run `go run ./test/cmd/functional api mup4` - passes
4. Run `go run ./test/cmd/functional api mup6` - passes
5. Run `go run ./test/cmd/functional api --all` - 14/14 pass

### Phase 5: Documentation Updates

1. Update `.claude/zebgp/api/ARCHITECTURE.md` with MUP family support
2. Update `plan/CLAUDE_CONTINUATION.md` with completed status

## Checklist

- [ ] Tests fail first (TDD)
- [ ] Tests pass after impl
- [ ] make test passes
- [ ] make lint passes
- [ ] mup4 test passes
- [ ] mup6 test passes
- [ ] Goal achieved (14/14 API tests)
- [ ] Documentation updated
- [ ] Spec moved to plan/done/
- [ ] plan/README.md updated
