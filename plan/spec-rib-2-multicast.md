# Spec: Multicast RIB Support (SAFI 2)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/6 |
| Updated | 2026-05-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. RFC 4760 (`rfc/short/rfc4760.md`) - multiprotocol extensions
4. `internal/core/rib/locrib/manager.go` - Loc-RIB keyed by family.Family
5. `internal/core/rib/store/store_bart.go` - BART store (has LPM via underlying trie)
6. `internal/component/bgp/plugins/rib/storage/familyrib.go` - per-peer per-family storage
7. `internal/component/bgp/plugins/rib/rib_commands.go` - command registration pattern

## Task

Ze already carries multicast NLRI (SAFI 2) at the wire level, stores them in per-peer
FamilyRIBs keyed by `family.IPv4Multicast`/`family.IPv6Multicast`, mirrors best-path
changes into the Loc-RIB (which also keys by family), and runs best-path selection.
The multicast data path from wire to Loc-RIB is fully functional.

What is **missing**: a Reverse Path Forwarding (RPF) lookup interface that external
multicast routing daemons (PIM-SM, IGMP proxy) need to determine the upstream
interface for a multicast source address. RPF asks: "given source address S, what
is the best next-hop in the MRIB toward S?" This is a longest-prefix-match (LPM)
query against multicast family entries in the Loc-RIB.

Secondary: a CLI command to query the MRIB directly (`bgp rib rpf <family> <source>`)
so operators can diagnose multicast path selection without external tools.

This spec does NOT cover:
- PIM/IGMP daemon implementation (external, out of scope)
- Multicast FIB programming (sysrib already handles family-aware route installation)
- Any wire encoding changes (multicast NLRI encoding is identical to unicast)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - RIB architecture, cross-protocol design
  -> Constraint: Loc-RIB already keys by family.Family; multicast families get separate shards
- [ ] `docs/architecture/plugin/rib-storage-design.md` - per-peer storage internals

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4760.md` - Multiprotocol Extensions for BGP-4
  -> Constraint: SAFI 2 (multicast) uses same NLRI encoding as SAFI 1 (unicast); only semantic difference
  -> Constraint: RFC 4760 obsoletes RFC 2858; no separate summary needed
- [ ] `rfc/short/rfc7911.md` - ADD-PATH (applies to multicast SAFI too)
  -> Constraint: path-id handling is family-agnostic, already works for multicast

**Key insights:**
- Wire encoding is identical for unicast and multicast SAFIs (prefix-len + prefix bytes)
- RPF is not defined by any BGP RFC; it is a local routing concept (PIM-SM RFC 7761 Section 4.1)
- BART trie's `LookupPrefixLPM(pfx)` returns (matchedPrefix, val, ok); use host-route /32|/128 as query
- Loc-RIB is sharded by prefix hash; LPM must query all shards and pick longest match
- `store.Store` wraps `bart.Table` but only exposes exact-match; LPM needs new method

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/family/registry.go` - defines IPv4Multicast, IPv6Multicast as registered families
- [ ] `internal/core/rib/locrib/manager.go` - Loc-RIB Insert/Remove/Lookup/Best all take family.Family; multicast already stored separately
- [ ] `internal/core/rib/locrib/shard.go` - familyShards owns per-prefix shards; store is bart.Table-backed
- [ ] `internal/core/rib/store/store_bart.go` - Store exposes exact-match Lookup(pfx), NOT LPM
  -> Constraint: bart.Table has `LookupPrefixLPM(pfx) (lpmPfx, val, ok)` that returns the matched prefix
  -> Constraint: For addr-based RPF, convert addr to /32 or /128 host prefix and call LookupPrefixLPM
- [ ] `internal/component/bgp/plugins/rib/storage/familyrib.go` - FamilyRIB keyed by family; isCIDRFamily includes SAFIMulticast
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - best-path changes mirror to Loc-RIB per-family
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - command registration and dispatch pattern

**Behavior to preserve:**
- Wire-level multicast NLRI parsing (already works, same encoding as unicast)
- Capability negotiation for SAFI 2 (already works)
- Per-peer FamilyRIB storage keyed by family (already separate)
- Best-path selection for multicast families (already runs)
- Loc-RIB mirroring of multicast best-path changes (already wired)
- JSON event format for multicast routes

**Behavior to change:**
- `store.Store` does not expose LPM lookup; add `LookupLPM(addr netip.Addr) (T, netip.Prefix, bool)`
- `locrib.RIB` does not expose LPM; add `LPM(family, addr) (Path, netip.Prefix, bool)`
- No RPF query command exists in the RIB plugin
- No CLI command for multicast RPF resolution

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- RPF query: plugin command `bgp rib rpf <family> <source-address>` or Go call `locrib.RIB.LPM()`
- Wire bytes: multicast NLRI in MP_REACH/MP_UNREACH with SAFI 2 (existing, no change)

### Transformation Path
1. RPF query arrives as plugin command string (or direct Go call)
2. Parse family (must be CIDR family) and source address
3. Call `locrib.RIB.LPM(family, sourceAddr)` which:
   a. Looks up the familyShards for the given family
   b. Queries each shard's store via `store.Store.LookupLPM(addr)`
   c. Each shard calls `bart.Table.LookupPrefixLPM(hostPrefix)` for LPM
   d. Collects results from all shards, picks the one with longest prefix
   e. Returns best Path from the matched PathGroup and the matching prefix
4. Format response as JSON: source, matched-prefix, next-hop, protocol, metric, found

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> Plugin | Text command `bgp rib rpf ipv4/multicast 10.1.2.3` | [ ] |
| Plugin -> Loc-RIB | Direct Go call to `locrib.RIB.LPM()` | [ ] |
| Loc-RIB -> Store | `store.Store.LookupLPM(addr)` per shard | [ ] |
| Store -> BART | `bart.Table.LookupPrefixLPM(hostPfx)` | [ ] |

### Integration Points
- `locrib.RIB` - new `LPM` method, uses existing familyShards infrastructure
- `store.Store` - new `LookupLPM` method exposing bart's `LookupPrefixLPM`
- RIB plugin command table - new `bgp rib rpf` command
- sysrib already subscribes to locrib changes; multicast routes flow to FIB without change

### Architectural Verification
- [ ] No bypassed layers (RPF queries go through locrib, not directly to plugin storage)
- [ ] No unintended coupling (LPM is a generic store capability, not multicast-specific)
- [ ] No duplicated functionality (reuses bart's built-in LPM, no parallel trie)
- [ ] Zero-copy preserved where applicable (Path is a value type, no alloc)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `bgp rib rpf ipv4/multicast <addr>` command | -> | `locrib.RIB.LPM()` -> `store.Store.LookupLPM()` | `TestRPFCommand_IPv4Multicast` |
| `bgp rib rpf ipv6/multicast <addr>` command | -> | `locrib.RIB.LPM()` -> `store.Store.LookupLPM()` | `TestRPFCommand_IPv6Multicast` |
| Direct Go API `locrib.RIB.LPM(family, addr)` | -> | `store.Store.LookupLPM()` -> `bart.Table.LookupPrefixLPM()` | `TestLocRIB_LPM` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Insert multicast routes 224.0.0.0/4, 224.1.0.0/16, 224.1.2.0/24 into Loc-RIB (ipv4/multicast), then LPM for 224.1.2.5 | Returns best path for 224.1.2.0/24 (most specific match) |
| AC-2 | LPM query for address with no covering multicast route | Returns (zero Path, invalid prefix, false) |
| AC-3 | LPM query against unicast family (not multicast) | Works correctly (LPM is family-agnostic; validates generic utility) |
| AC-4 | Command `bgp rib rpf ipv4/multicast 224.1.2.5` with routes installed | Returns JSON with matched-prefix, next-hop, protocol source |
| AC-5 | Command `bgp rib rpf ipv4/multicast 10.99.99.99` with no covering route | Returns JSON with `"found": false` |
| AC-6 | Command with non-CIDR family (e.g., l2vpn/evpn) | Returns error: RPF only supports CIDR families |
| AC-7 | Multiple protocols advertise same multicast prefix; LPM returns system-best | LPM returns the Path selected by Loc-RIB best-path (lowest admin distance) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStoreLookupLPM` | `internal/core/rib/store/store_bart_test.go` | LPM returns most specific match, no-match returns false | Pass |
| `TestStoreLookupLPM_Invalid` | `internal/core/rib/store/store_bart_test.go` | Invalid addr returns (zero, invalid prefix, false) | Pass |
| `TestLocRIB_LPM` | `internal/core/rib/locrib/locrib_test.go` | LPM through Loc-RIB, picks longest match across shards | Pass |
| `TestLocRIB_LPM_NoFamily` | `internal/core/rib/locrib/locrib_test.go` | LPM for family with no entries returns false | Pass |
| `TestLocRIB_LPM_BestPath` | `internal/core/rib/locrib/locrib_test.go` | LPM returns best from PathGroup, not arbitrary path | Pass |
| `TestRPFCommand_IPv4Multicast` | `internal/component/bgp/plugins/rib/rib_test.go` | End-to-end command -> JSON response | Pass |
| `TestRPFCommand_IPv6Multicast` | `internal/component/bgp/plugins/rib/rib_test.go` | IPv6 multicast RPF works | Pass |
| `TestRPFCommand_NoRoute` | `internal/component/bgp/plugins/rib/rib_test.go` | No-match returns found:false | Pass |
| `TestRPFCommand_NonCIDR` | `internal/component/bgp/plugins/rib/rib_test.go` | Non-CIDR family rejected with error | Pass |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Source address | valid netip.Addr | any valid IPv4/IPv6 | invalid (zero value) | N/A |
| Prefix length (in store) | 0-32 (v4), 0-128 (v6) | /32, /128 | N/A (0 is default route) | rejected by netip.Prefix |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-rpf-multicast` | `test/plugin/rpf-multicast.ci` | Inject multicast routes via bgp rib inject, query RPF, verify JSON response | |

### Future (if deferring any tests)
- None planned

## Files to Modify
- `internal/core/rib/store/store_bart.go` - add `LookupLPM(addr netip.Addr) (T, netip.Prefix, bool)`
- `internal/core/rib/locrib/manager.go` - add `LPM(fam family.Family, addr netip.Addr) (Path, netip.Prefix, bool)`
- `internal/core/rib/locrib/shard.go` - expose LPM on shard (call store.LookupLPM per shard)
- `internal/component/bgp/plugins/rib/rib_commands.go` - register `bgp rib rpf` command

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A (command-based, not YANG-modeled) |
| RPC count in architecture docs | No | N/A |
| CLI commands/flags | Yes | `internal/component/bgp/plugins/rib/rib_commands.go` |
| CLI usage/help text | Yes | Help string in command registration |
| API commands doc | Yes | `docs/architecture/api/commands.md` |
| Plugin SDK docs | No | No SDK change |
| Editor autocomplete | No | Not YANG-driven |
| Functional test for new RPC/API | Yes | `test/plugin/rpf-multicast.ci` |

## Files to Create
- `test/plugin/rpf-multicast.ci` - functional test for RPF command

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - add RPF lookup |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` - add `bgp rib rpf` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` - add `bgp rib rpf` |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | RPF is not RFC-defined BGP behavior |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - multicast RPF support |
| 12 | Internal architecture changed? | No | LPM is natural extension of existing BART |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Store LPM** -- add `LookupLPM` to `store.Store`
   - Tests: `TestStoreLookupLPM`, `TestStoreLookupLPM_Invalid`
   - Files: `internal/core/rib/store/store_bart.go`, `store_bart_test.go`
   - Design: `LookupLPM(addr netip.Addr) (T, netip.Prefix, bool)` delegates to
     `bart.Table.LookupPrefixLPM(netip.PrefixFrom(addr, bits))` where bits is 32 or 128
   - Also add to `store_map.go` (build tag `maprib`): linear scan all entries, pick longest match
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Loc-RIB LPM** -- add `LPM` to `locrib.RIB`
   - Tests: `TestLocRIB_LPM`, `TestLocRIB_LPM_NoFamily`, `TestLocRIB_LPM_BestPath`
   - Files: `internal/core/rib/locrib/manager.go`, `shard.go`, `locrib_test.go`
   - Design: Query all shards in the family. Each shard returns its LPM result. Pick
     the result with the longest prefix (most specific). From the matching PathGroup,
     return `best()`. Locking: take shard.mu.RLock per shard during query.
   - Verify: tests fail -> implement -> tests pass

3. **Phase: RPF command** -- register `bgp rib rpf` in RIB plugin
   - Tests: `TestRPFCommand_IPv4Multicast`, `TestRPFCommand_IPv6Multicast`,
     `TestRPFCommand_NoRoute`, `TestRPFCommand_NonCIDR`
   - Files: `internal/component/bgp/plugins/rib/rib_commands.go`
   - Design: Command syntax `bgp rib rpf <family> <source-addr>`. Validates family is CIDR.
     Calls `locrib.Default().LPM(fam, addr)`. Returns JSON:
     ```json
     {"source":"224.1.2.5","family":"ipv4/multicast","found":true,"matched-prefix":"224.1.2.0/24","next-hop":"10.0.0.1","protocol":"bgp","metric":0}
     ```
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Functional test** -- `.ci` test proving end-to-end RPF
   - Tests: `test-rpf-multicast`
   - Files: `test/plugin/rpf-multicast.ci`
   - Verify: functional test passes

5. **Phase: Full verification** -- `make ze-verify`

6. **Phase: Complete spec** -- audit, learned summary, commit

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | LPM returns most-specific match across all shards; best path from PathGroup |
| Naming | Command is `bgp rib rpf`, JSON keys use kebab-case (`matched-prefix`, `next-hop`) |
| Data flow | RPF queries go through locrib.LPM, not directly to per-peer storage |
| Rule: no-layering | No duplicate LPM implementation; reuses bart.Table.LookupPrefixLPM |
| Rule: wiring-completeness | `LPM()` called from rpf command; rpf command registered in command table |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `store.Store.LookupLPM` method exists | `grep -n "func.*Store.*LookupLPM" internal/core/rib/store/store_bart.go` |
| `locrib.RIB.LPM` method exists | `grep -n "func.*RIB.*LPM" internal/core/rib/locrib/manager.go` |
| `bgp rib rpf` command registered | `grep -n "bgp rib rpf" internal/component/bgp/plugins/rib/rib_commands.go` |
| Functional test exists | `ls test/plugin/rpf-multicast.ci` |
| All unit tests pass | `go test ./internal/core/rib/... ./internal/component/bgp/plugins/rib/...` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Source address parsed via `netip.ParseAddr`; invalid rejected before LPM |
| Resource exhaustion | LPM is O(depth) per shard (max 4 levels IPv4, 16 IPv6); N shards queried sequentially; no amplification |
| Information leakage | RPF response shows only best-path metadata (next-hop, prefix); same info as `bgp rib show best` |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design Insights

- The Loc-RIB is sharded by prefix hash. An LPM query for an address cannot predict which
  shard holds the covering prefix. Solution: query all shards, take the result with the
  longest prefix. Multicast tables are typically small (<100K prefixes) and shard count is
  low (default GOMAXPROCS, 4-16); each bart LPM is O(tree-depth), so total cost is negligible.

- `bart.Table.LookupPrefixLPM(pfx)` returns `(matchedPfx, val, ok)`. For RPF with an address:
  convert addr to host prefix (`netip.PrefixFrom(addr, addr.BitLen())`) and call LookupPrefixLPM.
  This gives both the matched route prefix and the PathGroup value.

- The `store.Store.LookupLPM` method is generic and useful beyond multicast: FIB debugging,
  connected-route lookup, redistribute filtering. Exposing it on Store (not just locrib)
  makes it testable independently.

- The map-backed store (`store_map.go`, build tag `maprib`) cannot do O(1) LPM. It must
  iterate all entries and find the longest prefix that contains the address. Acceptable
  because `maprib` is only used in tests, never production.

## RFC Documentation

No RFC-mandated behavior in this spec. RPF is a local routing concept, not a BGP protocol
requirement. No `// RFC NNNN Section X.Y` comments needed for the LPM/RPF code.

## Implementation Summary

### What Was Implemented
- `store.Store.LookupLPM` on both BART and map backends
- `locrib.RIB.LPM` querying all shards, picking longest match
- `bgp rib rpf` plugin command with JSON response
- CLI proxy `forwardRibRPF` + YANG schema entry
- 12 unit tests + 1 functional test

### Bugs Found/Fixed
- Zero NextHop in Path would produce `"invalid IP"` in JSON; fixed to emit empty string

### Documentation Updates
- `docs/architecture/api/commands.md` - added `rib rpf` line
- `docs/guide/command-reference.md` - added table row
- `docs/features.md` - added RPF Lookup feature entry

### Deviations from Plan
- Spec listed `shard.go` as needing modification; LPM was implemented directly in `manager.go` using the existing shard iteration pattern
- Added CLI proxy wiring (not in original spec) after review caught the gap

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| LPM on store | Done | `internal/core/rib/store/store_bart.go:119` | Both BART and map backends |
| LPM on Loc-RIB | Done | `internal/core/rib/locrib/manager.go:318` | Queries all shards |
| RPF command | Done | `internal/component/bgp/plugins/rib/rib_commands.go:405` | JSON response |
| CLI proxy | Done | `internal/component/bgp/plugins/cmd/rib/rib.go:44` | YANG + handler |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestLocRIB_LPM` | Most specific match across shards |
| AC-2 | Done | `TestLocRIB_LPM_NoMatch` | Returns false |
| AC-3 | Done | `TestLocRIB_LPM` (uses famV4 unicast) | Family-agnostic |
| AC-4 | Done | `TestRPFCommand_IPv4Multicast` | JSON with matched-prefix, next-hop |
| AC-5 | Done | `TestRPFCommand_NoRoute` | found:false JSON |
| AC-6 | Done | `TestRPFCommand_NonCIDR` | Error returned |
| AC-7 | Done | `TestLocRIB_LPM_BestPath` | Best by admin distance |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestStoreLookupLPM` | Pass | `internal/core/rib/store/store_bart_test.go` | |
| `TestStoreLookupLPM_Invalid` | Pass | `internal/core/rib/store/store_bart_test.go` | |
| `TestStoreLookupLPM_IPv6` | Pass | `internal/core/rib/store/store_bart_test.go` | |
| `TestLocRIB_LPM` | Pass | `internal/core/rib/locrib/locrib_test.go` | |
| `TestLocRIB_LPM_NoFamily` | Pass | `internal/core/rib/locrib/locrib_test.go` | |
| `TestLocRIB_LPM_BestPath` | Pass | `internal/core/rib/locrib/locrib_test.go` | |
| `TestLocRIB_LPM_NoMatch` | Pass | `internal/core/rib/locrib/locrib_test.go` | |
| `TestLocRIB_LPM_InvalidAddr` | Pass | `internal/core/rib/locrib/locrib_test.go` | |
| `TestRPFCommand_IPv4Multicast` | Pass | `internal/component/bgp/plugins/rib/rib_test.go` | |
| `TestRPFCommand_IPv6Multicast` | Pass | `internal/component/bgp/plugins/rib/rib_test.go` | |
| `TestRPFCommand_NoRoute` | Pass | `internal/component/bgp/plugins/rib/rib_test.go` | |
| `TestRPFCommand_NonCIDR` | Pass | `internal/component/bgp/plugins/rib/rib_test.go` | |
| `test-rpf-multicast` | Pass | `test/plugin/rpf-multicast.ci` | Functional |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/core/rib/store/store_bart.go` | Modified | LookupLPM added |
| `internal/core/rib/store/store_map.go` | Modified | LookupLPM added |
| `internal/core/rib/locrib/manager.go` | Modified | LPM added |
| `internal/component/bgp/plugins/rib/rib_commands.go` | Modified | rpfLookup + registration |
| `internal/component/bgp/plugins/cmd/rib/rib.go` | Modified | CLI proxy |
| `internal/component/bgp/plugins/cmd/rib/schema/ze-rib-cmd.yang` | Modified | YANG entry |
| `test/plugin/rpf-multicast.ci` | Created | Functional test |

### Audit Summary
- **Total items:** 24
- **Done:** 24
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (shard.go not modified; LPM in manager.go instead)

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | Zero NextHop produces "invalid IP" in JSON | rib_commands.go:450 | Fixed: emit empty string |

### Fixes applied
- `rib_commands.go`: guard `best.NextHop.IsValid()` before `.String()`; always emit key with empty string for blackhole routes

### Run 2 (post-fix)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | No findings | - | - |

### Run 3 (post-proxy addition)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | No findings | - | - |

### Final status
- [x] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [x] All NOTEs recorded above

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/rib/store/store_bart.go` | Yes | LookupLPM at line 119 |
| `internal/core/rib/locrib/manager.go` | Yes | LPM at line 318 |
| `internal/component/bgp/plugins/rib/rib_commands.go` | Yes | rpfLookup at line 405 |
| `internal/component/bgp/plugins/cmd/rib/rib.go` | Yes | forwardRibRPF at line 79 |
| `test/plugin/rpf-multicast.ci` | Yes | Created |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | LPM most specific | `TestLocRIB_LPM` passes: asserts Instance=3 for /24 match |
| AC-2 | No match returns false | `TestLocRIB_LPM_NoMatch` passes |
| AC-3 | Works for unicast | `TestLocRIB_LPM` uses famV4 (unicast) |
| AC-4 | JSON response | `TestRPFCommand_IPv4Multicast` asserts found=true, matched-prefix, next-hop |
| AC-5 | No-route JSON | `TestRPFCommand_NoRoute` asserts found=false |
| AC-6 | Non-CIDR error | `TestRPFCommand_NonCIDR` asserts error contains "CIDR families" |
| AC-7 | Best path selection | `TestLocRIB_LPM_BestPath` asserts idStatic wins (lowest admin distance) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `bgp rib rpf ipv4/multicast <addr>` | `test/plugin/rpf-multicast.ci` | Pass (ze-test bgp plugin 263) |
| CLI proxy `ze-rib-api:rpf` | `TestRibProxyRPCRegistration` | Pass (9 RPCs registered) |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (LPM is generic, useful for multiple consumers)
- [ ] No speculative features (only RPF command + Go API)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit
