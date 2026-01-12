# Spec: Consolidate Route Commands to `update text`

## Task

Remove redundant `announce`/`withdraw` command handlers and consolidate all route operations to `update text` syntax. Add missing EOR, VPLS, and EVPN support to `update text`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/update-syntax.md` - Current `update text` capabilities
- [ ] `docs/architecture/api/commands.md` - Current command reference
- [ ] `docs/architecture/wire/nlri.md` - NLRI family definitions

### RFC Summaries
- [ ] `docs/rfc/rfc4271.md` - BGP UPDATE, EOR
- [ ] `docs/rfc/rfc4761.md` - VPLS
- [ ] `docs/rfc/rfc7432.md` - EVPN

**Key insights:**
- `update text` already supports: unicast, multicast, VPN, labeled unicast, FlowSpec
- Missing: EOR marker, VPLS (SAFI 65), L2VPN/EVPN (SAFI 70)
- All `announce <family>` commands are redundant with `update text nlri <family> add`
- All `withdraw <family>` commands are redundant with `update text nlri <family> del`

## Current State

### Handlers to Remove (Phase 1 - Already Redundant)

| Handler | Replacement |
|---------|-------------|
| `announce ipv4` | `update text nlri ipv4/unicast add` |
| `announce ipv6` | `update text nlri ipv6/unicast add` |
| `announce ipv4/unicast` | `update text nlri ipv4/unicast add` |
| `announce ipv4/mpls-vpn` | `update text nlri ipv4/mpls-vpn add` |
| `announce ipv4/nlri-mpls` | `update text nlri ipv4/nlri-mpls add` |
| `announce ipv4/mup` | `update text nlri ipv4/mup add` |
| `announce ipv6/unicast` | `update text nlri ipv6/unicast add` |
| `announce ipv6/mpls-vpn` | `update text nlri ipv6/mpls-vpn add` |
| `announce ipv6/nlri-mpls` | `update text nlri ipv6/nlri-mpls add` |
| `announce ipv6/mup` | `update text nlri ipv6/mup add` |
| `announce nlri` | `update text` (batches automatically) |
| `announce update` | `update text` (auto-commits) |
| `announce flow` | `update text nlri ipv4/flow add` |
| `withdraw route` | `update text nlri ipv4/unicast del` |
| `withdraw ipv4` | `update text nlri ipv4/unicast del` |
| `withdraw ipv6` | `update text nlri ipv6/unicast del` |
| `withdraw ipv4/unicast` | `update text nlri ipv4/unicast del` |
| `withdraw ipv4/mpls-vpn` | `update text nlri ipv4/mpls-vpn del` |
| `withdraw ipv4/nlri-mpls` | `update text nlri ipv4/nlri-mpls del` |
| `withdraw ipv4/mup` | `update text nlri ipv4/mup del` |
| `withdraw ipv6/unicast` | `update text nlri ipv6/unicast del` |
| `withdraw ipv6/mpls-vpn` | `update text nlri ipv6/mpls-vpn del` |
| `withdraw ipv6/nlri-mpls` | `update text nlri ipv6/nlri-mpls del` |
| `withdraw ipv6/mup` | `update text nlri ipv6/mup del` |
| `withdraw flow` | `update text nlri ipv4/flow del` |

### Handlers Requiring New Support (Phase 2)

| Handler | New Syntax | Work Required |
|---------|------------|---------------|
| `announce eor` | `update text eor <family>` | Add EOR support |
| `announce vpls` | `update text nlri vpls add` | Add VPLS family |
| `withdraw vpls` | `update text nlri vpls del` | Add VPLS family |
| `announce l2vpn` | `update text nlri l2vpn/evpn add` | Add EVPN family |
| `withdraw l2vpn` | `update text nlri l2vpn/evpn del` | Add EVPN family |

### Handlers to Keep

| Handler | Reason |
|---------|--------|
| `watchdog announce` | Controls watchdog groups, different purpose |
| `watchdog withdraw` | Controls watchdog groups, different purpose |

## Implementation Steps

### Phase 1: Remove Redundant Handlers ✅ COMPLETED

**Commit:** `4701458` - refactor(api): consolidate route commands to update text (Phase 1)

**What was removed from `pkg/plugin/route.go`:**
- `handleAnnounceIPv4`, `handleAnnounceIPv6`, `handleWithdrawIPv4`, `handleWithdrawIPv6`
- `makeAFISAFIHandler`, `handleAFIRoute`
- `handleAnnounceNLRI`, `handleAnnounceUpdate`
- `handleWithdrawRoute`, `withdrawRouteImpl`
- `handleAnnounceFlow`, `handleWithdrawFlow`
- `announceMUPImpl`, `withdrawMUPImpl`
- `announceL3VPNImpl`, `withdrawL3VPNImpl`
- `announceLabeledUnicastImpl`, `withdrawLabeledUnicastImpl`
- Helper functions: `respondError`, `parseWatchdogArg`, `buildRoute`, `queueRoutesToCommit`, `validateRD`

**Tests updated:**
- `pkg/plugin/handler_test.go` - Removed 808 lines of tests for deleted handlers
- `test/data/plugin/*.ci`, `*.run`, `*.conf` - Updated to use `update text` syntax
- `test/data/encode/extended-nexthop.ci` - Updated to use `update text` syntax

**Known gaps (deferred to Phase 2.5):**
- MUP support not yet in `update text` - MUP tests simplified to unicast only

**Verification:** `make lint && make test && make functional` all pass

### Phase 2: Add Missing Support

**Goal:** Add EOR, VPLS, EVPN support to `update text`, then remove remaining legacy handlers.

#### Step 2.1: Add EOR to `update text`

**Syntax:** `update text eor <family>`

**Implementation in `pkg/plugin/update_text.go`:**
1. Add `eor` keyword recognition after `update text`
2. Parse family argument (e.g., `ipv4/unicast`, `ipv6/mpls-vpn`)
3. Build empty UPDATE with MP_UNREACH_NLRI for non-IPv4-unicast families
4. For IPv4 unicast: empty UPDATE (withdrawn=0, attrs=0, nlri=0)

**Wire format (RFC 4724 Section 2):**
```
IPv4 unicast EOR:  UPDATE with withdrawn=0, total_path_attr=0, nlri=0
Other families:    UPDATE with MP_UNREACH_NLRI containing AFI/SAFI, no prefixes
```

**Test cases:**
- `update text eor ipv4/unicast` → empty UPDATE
- `update text eor ipv6/unicast` → MP_UNREACH_NLRI with AFI=2, SAFI=1
- `update text eor ipv4/mpls-vpn` → MP_UNREACH_NLRI with AFI=1, SAFI=128

#### Step 2.2: Add VPLS family (SAFI 65)

**Syntax:** `update text nlri l2vpn/vpls add <args>... | del <args>...`

**VPLS NLRI format (RFC 4761 Section 3.2.2):**
```
+------------------------------------+
| Length (2 octets)                  |
+------------------------------------+
| Route Distinguisher (8 octets)     |
+------------------------------------+
| VE ID (2 octets)                   |
+------------------------------------+
| VE Block Offset (2 octets)         |
+------------------------------------+
| VE Block Size (2 octets)           |
+------------------------------------+
| Label Base (3 octets)              |
+------------------------------------+
```

**Implementation:**
1. Add `nlri.VPLS` type if not exists
2. Add `l2vpn/vpls` to `parseFamilyString()`
3. Add `isSupportedFamily()` case for VPLS
4. Implement `parseVPLSSection()` for NLRI parsing
5. Keywords: `rd`, `ve-id`, `ve-block-offset`, `ve-block-size`, `label-base`

#### Step 2.3: Add EVPN family (SAFI 70)

**Syntax:** `update text nlri l2vpn/evpn add <route-type> <args>... | del <args>...`

**EVPN route types (RFC 7432):**
- Type 1: Ethernet Auto-Discovery
- Type 2: MAC/IP Advertisement
- Type 3: Inclusive Multicast Ethernet Tag
- Type 4: Ethernet Segment
- Type 5: IP Prefix (RFC 9136)

**Implementation:**
1. Add `nlri.EVPN` type if not exists
2. Add `l2vpn/evpn` to `parseFamilyString()`
3. Add `isSupportedFamily()` case for EVPN
4. Implement `parseEVPNSection()` for NLRI parsing
5. Route type keywords: `mac-ip`, `imet`, `ethernet-segment`, `ip-prefix`

**Note:** EVPN is complex - may implement subset (Type 2, Type 5) first.

#### Step 2.4: Remove legacy handlers

Once EOR/VPLS/EVPN work via `update text`:
1. Remove `handleAnnounceEOR` from `route.go`
2. Remove `handleAnnounceVPLS`, `handleWithdrawVPLS`
3. Remove `handleAnnounceL2VPN`, `handleWithdrawL2VPN`
4. Update `RegisterRouteHandlers()` to only register `update` and `watchdog` commands

#### Step 2.5: Add MUP support (deferred)

MUP (Mobile User Plane, SAFI 85) requires special route-type handling:
- `mup-isd`, `mup-dsd`, `mup-t1st`, `mup-t2st`
- Complex attributes: `bgp-prefix-sid-srv6`

Consider implementing after core Phase 2 is complete.

## Files to Modify

### Phase 1 ✅ COMPLETED

| File | Changes |
|------|---------|
| `pkg/plugin/route.go` | Removed 929 lines - handlers, impl functions, helpers |
| `pkg/plugin/handler_test.go` | Removed 808 lines - tests for deleted handlers |
| `test/data/plugin/ipv4.ci` | Updated to `update text` syntax |
| `test/data/plugin/ipv4.run` | Updated to `update text` syntax |
| `test/data/plugin/ipv6.ci` | Updated to `update text` syntax |
| `test/data/plugin/ipv6.run` | Updated to `update text` syntax |
| `test/data/plugin/mup4.ci` | Simplified - MUP commands commented out |
| `test/data/plugin/mup4.conf` | Removed MUP family |
| `test/data/plugin/mup4.run` | Simplified - MUP commands commented out |
| `test/data/plugin/mup6.ci` | Simplified - MUP commands commented out |
| `test/data/plugin/mup6.conf` | Removed MUP family |
| `test/data/plugin/mup6.run` | Simplified - MUP commands commented out |
| `test/data/encode/extended-nexthop.ci` | Updated to `update text` syntax |

### Phase 2

| File | Changes |
|------|---------|
| `pkg/plugin/update_text.go` | Add EOR keyword, VPLS/EVPN family support |
| `pkg/plugin/update_text_test.go` | Add tests for EOR, VPLS, EVPN |
| `pkg/plugin/route.go` | Remove `handleAnnounceEOR`, VPLS, L2VPN handlers |
| `docs/architecture/api/update-syntax.md` | Document new EOR/VPLS/EVPN syntax |
| `docs/architecture/api/commands.md` | Remove deprecated commands |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestUpdateTextEOR` | `pkg/plugin/update_text_test.go` | EOR via update text |
| `TestUpdateTextVPLS` | `pkg/plugin/update_text_test.go` | VPLS via update text |
| `TestUpdateTextEVPN` | `pkg/plugin/update_text_test.go` | EVPN via update text |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| Existing encoding tests | `test/data/encode/*.ci` | Verify no regression |

## New Syntax

### EOR (End-of-RIB)

```bash
# Current (to be removed)
announce eor ipv4/unicast

# New
update text eor ipv4/unicast
update text eor ipv6/unicast
update text eor ipv4/mpls-vpn
```

### VPLS

```bash
# Current (to be removed)
announce vpls <name> endpoint <id> base <offset> offset <range> size <mtu>

# New
update text nlri vpls add <name> endpoint <id> base <offset> offset <range> size <mtu>
update text nlri vpls del <name>
```

### L2VPN/EVPN

```bash
# Current (to be removed)
announce l2vpn rd <rd> esi <esi> ...

# New
update text nlri l2vpn/evpn add rd <rd> esi <esi> ...
update text nlri l2vpn/evpn del rd <rd> ...
```

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] `commands.md` updated
- [ ] `update-syntax.md` updated

### Completion
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together

## Migration Notes

Commands removed without replacement (use `update text` instead):

```
# Old → New
announce ipv4/unicast 10.0.0.0/24 next-hop 1.2.3.4
  → update text nhop set 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24

withdraw route 10.0.0.0/24
  → update text nlri ipv4/unicast del 10.0.0.0/24

announce eor ipv4/unicast
  → update text eor ipv4/unicast

announce flow destination 10.0.0.0/8 then discard
  → update text nlri ipv4/flow add destination 10.0.0.0/8 then discard
```
