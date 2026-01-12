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

### Phase 1: Remove Redundant Handlers

1. **Baseline verification** - `make lint && make test && make functional` (paste output)
2. **Remove family-explicit handlers** - Delete `handleAnnounceIPv4/IPv6`, `handleWithdrawIPv4/IPv6`
3. **Remove slash-format handlers** - Delete `makeAFISAFIHandler` registrations for announce/withdraw
4. **Remove batch handlers** - Delete `handleAnnounceNLRI`, `handleAnnounceUpdate`
5. **Remove withdraw route** - Delete `handleWithdrawRoute`
6. **Remove flow handlers** - Delete `handleAnnounceFlow`, `handleWithdrawFlow`
7. **Update registration** - Remove all deleted handlers from `RegisterRouteHandlers`
8. **Update tests** - Remove/update tests for deleted handlers
9. **Verify Phase 1** - `make lint && make test && make functional` (paste output)

### Phase 2: Add Missing Support

10. **Add EOR to update text** - Implement `update text eor <family>` syntax
11. **Add VPLS family** - Add VPLS to `isSupportedFamily`, implement NLRI parsing
12. **Add EVPN family** - Add EVPN to `isSupportedFamily`, implement NLRI parsing
13. **Remove EOR handler** - Delete `handleAnnounceEOR`
14. **Remove VPLS handlers** - Delete `handleAnnounceVPLS`, `handleWithdrawVPLS`
15. **Remove L2VPN handlers** - Delete `handleAnnounceL2VPN`, `handleWithdrawL2VPN`
16. **Update documentation** - Update commands.md, update-syntax.md
17. **Final verification** - `make lint && make test && make functional` (paste output)

## Files to Modify

### Phase 1
- `pkg/plugin/route.go` - Remove handlers and registrations
- `pkg/plugin/route_test.go` - Update tests (if exists)
- `docs/architecture/api/commands.md` - Remove deprecated commands

### Phase 2
- `pkg/plugin/update_text.go` - Add EOR, VPLS, EVPN support
- `pkg/plugin/update_text_test.go` - Add tests
- `pkg/plugin/route.go` - Remove remaining handlers
- `docs/architecture/api/update-syntax.md` - Document new syntax
- `docs/architecture/api/commands.md` - Final cleanup

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
