# Spec: rib-inject

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/1 |
| Updated | 2026-04-01 |

## Task

Add `rib inject` and `rib withdraw` commands to the RIB plugin for direct adj-rib-in manipulation without a live BGP session. Primary use cases: looking glass graph visualization, testing, demonstration.

## Required Reading

- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - existing command pattern, selector convention
  -> Constraint: `rib clear in` takes selector as args[0], not via dispatcher peer extraction
  -> Constraint: Commands return (status, data, error) triple
- [ ] `internal/component/bgp/plugins/rib/rib_nlri.go` - prefixToWire, parseFamily, formatNLRIAsPrefix
  -> Constraint: Only simple prefix families (IPv4/IPv6 unicast/multicast) supported by prefixToWire
- [ ] `internal/component/bgp/attribute/builder.go` - Builder API for wire-format attributes
  -> Constraint: SetOrigin takes uint8, but OriginIGP is type Origin (named uint8), requires cast
- [ ] `internal/component/bgp/plugins/rib/storage/peerrib.go` - Insert/Remove/Lookup/FamilyLen
  -> Constraint: Insert takes (family, attrBytes, nlriBytes), both in wire format
- [ ] `internal/component/bgp/plugins/rib/rib.go` - RunRIBPlugin SDK Registration, command list
  -> Constraint: Every command name must appear in both doRegisterBuiltinCommands and SDK Registration
- [ ] `internal/component/bgp/plugins/rib/protocol_test.go` - command count assertion
  -> Constraint: TestRIBPluginFiveStageProtocol asserts exact command count
- [ ] `internal/component/bgp/plugins/cmd/rib/schema/ze-rib-cmd.yang` - CLI command tree
- [ ] `internal/component/bgp/plugins/cmd/rib/rib.go` - CLI proxy handlers
- [ ] `docs/guide/command-reference.md` - RIB command documentation
- [ ] `test/plugin/api-rib-clear-in.ci` - existing .ci pattern for RIB commands

## Current Behavior

**Source files read:**
- [ ] `rib_commands.go` - 16 commands registered (14 original + inject/withdraw)
- [ ] `rib.go` - RunRIBPlugin registers 16 command names in SDK Registration
- [ ] `rib_nlri.go` - prefixToWire converts "10.0.0.0/24" to wire bytes [24, 10, 0, 0]
- [ ] `builder.go` - Builder.Build() produces wire-format path attributes from setter calls
- [ ] `protocol_test.go` - asserts exactly 16 registered commands

**Behavior to preserve:**
- All existing RIB commands unchanged
- PeerRIB.Insert/Remove API unchanged
- Wire attribute format unchanged
- Existing selector convention (args[0], not dispatcher peer prefix)

## Data Flow

### Entry Point
- CLI: `rib inject 10.0.0.1 ipv4/unicast 10.0.0.0/24 origin igp nhop 1.1.1.1 aspath 64500,64501`
- Dispatched to RIB plugin via command registry -> handleCommand("rib inject", "", args)

### Transformation Path
1. Validate peer address (must be valid IP)
2. Validate family (must be simple prefix family)
3. Validate remaining args form complete key-value pairs
4. Parse optional key-value attribute pairs from args[3:]
5. Build wire-format attributes via attribute.Builder
6. Convert prefix to wire NLRI bytes via prefixToWire
7. Insert into ribInPool[peer].Insert(family, attrBytes, nlriBytes) under r.mu.Lock()

### Boundary Crossings
- CLI -> dispatcher -> plugin command registry -> RIBManager.handleCommand
- Text args -> wire bytes (attribute.Builder + prefixToWire)
- Wire bytes -> pool-deduplicated storage (PeerRIB.Insert -> FamilyRIB.Insert -> ParseAttributes)

## Acceptance Criteria

| AC | Expected Behavior |
|----|-------------------|
| AC-1 | `rib inject 10.0.0.1 ipv4/unicast 10.0.0.0/24` inserts route into adj-rib-in for peer 10.0.0.1 |
| AC-2 | `rib inject 10.0.0.1 ipv4/unicast 10.0.0.0/24 origin igp nhop 1.1.1.1 aspath 64500,64501 localpref 100 med 50` sets all attributes |
| AC-3 | `rib withdraw 10.0.0.1 ipv4/unicast 10.0.0.0/24` removes route from adj-rib-in |
| AC-4 | Injected routes appear in `rib show` output for that peer |
| AC-5 | Missing peer argument returns error with usage hint |
| AC-6 | Invalid prefix returns error |
| AC-7 | Invalid ASN in aspath returns error |
| AC-8 | Unknown attribute keyword returns error |
| AC-9 | IPv6 prefix works: `rib inject 10.0.0.1 ipv6/unicast 2001:db8::/32` |
| AC-10 | Withdraw of non-existent route returns existed=false (no error) |
| AC-11 | Multiple injects to same prefix = implicit withdraw of old attributes |
| AC-12 | Invalid peer address (not an IP) returns error |
| AC-13 | IPv6 next-hop rejected with explicit error |
| AC-14 | IPv4-mapped IPv6 next-hop (::ffff:x.x.x.x) accepted |
| AC-15 | Trailing attribute key without value returns error |
| AC-16 | Non-simple family (l2vpn/evpn) rejected with clear error |
| AC-17 | IPv4 multicast family works |
| AC-18 | IPv6 prefix in IPv4 family rejected (family mismatch) |

## TDD Plan

| Test | File | Validates |
|------|------|-----------|
| TestInjectRoute_Basic | rib_test.go | AC-1 |
| TestInjectRoute_AllAttributes | rib_test.go | AC-2 |
| TestWithdrawRoute_Basic | rib_test.go | AC-3 |
| TestInjectRoute_VisibleInShow | rib_test.go | AC-4 |
| TestInjectRoute_MissingPeer | rib_test.go | AC-5 |
| TestInjectRoute_InvalidPrefix | rib_test.go | AC-6 |
| TestInjectRoute_InvalidASPath | rib_test.go | AC-7 |
| TestInjectRoute_UnknownAttr | rib_test.go | AC-8 |
| TestInjectRoute_IPv6 | rib_test.go | AC-9 |
| TestWithdrawRoute_NonExistent | rib_test.go | AC-10 |
| TestInjectRoute_ImplicitWithdraw | rib_test.go | AC-11 |
| TestInjectRoute_InvalidPeerAddress | rib_test.go | AC-12 |
| TestInjectRoute_IPv6NextHopRejected | rib_test.go | AC-13 |
| TestInjectRoute_IPv4MappedIPv6NextHop | rib_test.go | AC-14 |
| TestInjectRoute_TrailingKeyNoValue | rib_test.go | AC-15 |
| TestInjectRoute_NonSimpleFamily | rib_test.go | AC-16 |
| TestInjectRoute_IPv4Multicast | rib_test.go | AC-17 |
| TestInjectRoute_FamilyMismatch | rib_test.go | AC-18 |
| TestInjectRoute_OriginIncomplete | rib_test.go | all origin values |
| TestInjectRoute_NoAttributes | rib_test.go | default origin set |
| TestInjectRoute_SingleASN | rib_test.go | single ASN parsing |
| TestInjectRoute_DuplicateAttr | rib_test.go | last-wins behavior |
| TestInjectRoute_InvalidFamily | rib_test.go | unparseable family |
| TestWithdrawRoute_InvalidPeerAddress | rib_test.go | AC-12 for withdraw |
| TestWithdrawRoute_MissingArgs | rib_test.go | too few args |
| TestParseASNList | rib_test.go | helper (7 subtests) |

## Syntax

    rib inject <peer> <family> <prefix> [origin <igp|egp|incomplete>] [nhop <ip>] [aspath <asn,asn,...>] [localpref <n>] [med <n>]
    rib withdraw <peer> <family> <prefix>

Defaults: origin=igp, no next-hop, empty as-path, no localpref, no med.

The peer address is a label for the RIB entry. It does not require a live BGP session. Must be a valid IP address.

## Next-Hop Validation Design

Next-hop validation should use the peer's negotiated OPEN capabilities to decide what formats are valid.

| Scenario | IPv4 nhop | IPv6 nhop |
|----------|-----------|-----------|
| Real peer, no extended-nexthop | Valid | Rejected |
| Real peer, extended-nexthop negotiated (RFC 5549) | Valid | Valid for families listed in capability |
| Injected route (no session) | Valid | Rejected (default policy, no negotiation) |
| IPv4-mapped IPv6 (::ffff:x.x.x.x) | Valid (To4 extracts IPv4) | N/A (parsed as IPv4) |

Current state: IPv4 only, IPv6 rejected with explicit error. IPv4-mapped IPv6 accepted.

Future: when RFC 5549 extended next-hop encoding lands, check PackContext.ExtendedNextHop for the peer. For injected routes with no session, either keep the default (IPv4 only) or add an optional flag to override.

## Validation Summary

| Input | Validation | Error on failure |
|-------|-----------|-----------------|
| peer (args[0]) | net.ParseIP != nil | "invalid peer address: X" |
| family (args[1]) | parseFamily + isSimplePrefixFamily | "unknown family" or "only supports simple prefix families" |
| prefix (args[2]) | prefixToWire (net.ParseCIDR internally) | "invalid prefix" |
| attr args count | len(attrArgs) % 2 == 0 | "attribute X has no value" |
| attr keyword | known set: origin, nhop, aspath, localpref, med | "unknown attribute: X" |
| origin value | igp, egp, incomplete | "unknown origin: X" |
| nhop | net.ParseIP + To4 != nil | "invalid next-hop IP" or "IPv6 next-hop not supported" |
| aspath | parseASNList (comma-separated uint32) | "invalid ASN" |
| localpref | strconv.ParseUint 32-bit | "invalid localpref" |
| med | strconv.ParseUint 32-bit | "invalid med" |
| family vs prefix AFI | prefixToWire checks IP vs family AFI | "IP address mismatch for family" |

## Files to Create/Modify

| File | Change | Status |
|------|--------|--------|
| `internal/component/bgp/plugins/rib/rib_commands.go` | injectRoute, withdrawRoute, parseASNList, injectOriginValues | DONE |
| `internal/component/bgp/plugins/rib/rib.go` | Register rib inject/withdraw in SDK | DONE |
| `internal/component/bgp/plugins/rib/rib_test.go` | 25 unit tests covering all ACs | DONE |
| `internal/component/bgp/plugins/rib/protocol_test.go` | Update command count 14 -> 16 | DONE |
| `internal/component/bgp/plugins/rib/schema/ze-rib-api.yang` | rpc inject, rpc withdraw | DONE |
| `internal/component/bgp/plugins/cmd/rib/schema/ze-rib-cmd.yang` | container inject, container withdraw | DONE |
| `internal/component/bgp/plugins/cmd/rib/rib.go` | forwardRibInject, forwardRibWithdraw proxy handlers | DONE |
| `docs/guide/command-reference.md` | rib inject/withdraw syntax | DONE |
| `docs/features.md` | Add rib inject/withdraw to feature list | TODO |
| `docs/architecture/api/commands.md` | Add rib inject/withdraw RPCs | TODO |
| `test/plugin/api-rib-inject.ci` | Functional test: inject + show | TODO |
| `test/plugin/api-rib-withdraw.ci` | Functional test: inject + withdraw + show | TODO |

## Wiring Test

| Feature | Test | Type | Status |
|---------|------|------|--------|
| rib inject reachable from CLI | test/plugin/api-rib-inject.ci | functional | TODO |
| rib withdraw reachable from CLI | test/plugin/api-rib-withdraw.ci | functional | TODO |
| rib inject unit | TestInjectRoute_Basic | unit | DONE |
| rib withdraw unit | TestWithdrawRoute_Basic | unit | DONE |
| YANG schema registered | TestRIBPluginFiveStageProtocol | unit | DONE |
| CLI proxy wired | forwardRibInject in cmd/rib/rib.go | code | DONE |

## Documentation Update Checklist

| # | Category | Needed | File | What |
|---|----------|--------|------|------|
| 1 | Feature list | Yes | docs/features.md | Add rib inject/withdraw to RIB section |
| 2 | User guide | No | | |
| 3 | Config syntax | No | | |
| 4 | CLI reference | Done | docs/guide/command-reference.md | Syntax and description |
| 5 | API/RPC docs | Yes | docs/architecture/api/commands.md | Add rib inject/withdraw RPCs |
| 6 | Plugin guide | No | | |
| 7 | Wire format | No | | |
| 8 | Plugin SDK rules | No | | |
| 9 | RFC compliance | No | | |
| 10 | Test infra | No | | |
| 11 | Comparison | No | | |
| 12 | Architecture | No | | |
