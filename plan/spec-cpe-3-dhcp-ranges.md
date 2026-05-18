# Spec: DHCP Server Multiple Address Ranges

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/6 |
| Updated | 2026-05-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/plugins/dhcpserver/config.go` - config parsing
4. `internal/plugins/dhcpserver/pool.go` - address pool allocation
5. `internal/plugins/dhcpserver/handler.go` - DHCP packet handling
6. `internal/plugins/dhcpserver/schema/ze-dhcp-server-conf.yang` - YANG schema

## Task

Extend the DHCP server to support multiple named address ranges per subnet. Currently the schema uses `container range` (single start/stop), preventing subnets with disjoint pools. VyOS uses `range <name> { start ... stop ... }` as a keyed list. Ze should match this model.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - YANG conventions
  -> Constraint: lists use `key "name"`, containers are for singletons

### RFC Summaries
- [ ] `rfc/short/rfc2131.md` - DHCP base protocol
  -> Constraint: RFC 2131 Section 4.3.1 says server SHOULD allocate from any available pool in the subnet

**Key insights:**
- The pool allocator uses a bitmap for O(1) allocation
- Multiple ranges need either multiple bitmaps or a composite pool with segments
- VyOS keyed list `range <name>` allows named ranges; ze should match for migration parity

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/dhcpserver/schema/ze-dhcp-server-conf.yang` - `container range` with single `start`/`stop` leaves
  -> Constraint: changing container to list changes config tree serialization
- [ ] `internal/plugins/dhcpserver/config.go` - `subnetConfig` has flat `RangeStart`/`RangeStop netip.Addr` (lines 38-39); `parseSubnet()` reads `data["range"]` as a single map
- [ ] `internal/plugins/dhcpserver/pool.go` - `pool` struct takes one start/stop pair; bitmap covers one contiguous range; `newPool(start, stop, statics)`
- [ ] `internal/plugins/dhcpserver/handler.go` - `newDHCPHandler()` calls `newPool(sub.RangeStart, sub.RangeStop, sub.StaticMappings)` (line 70)

**Behavior to preserve:**
- Static mappings within any range excluded from dynamic allocation
- Bitmap-based allocation
- Pool stats (total/allocated/available)
- MAC-to-address affinity within a pool
- All existing validation (range within subnet, start <= stop, etc.)

**Behavior to change:**
- YANG: `container range` -> `list range` keyed by name
- Go: `subnetConfig.RangeStart/RangeStop` -> `subnetConfig.Ranges []addressRange`
- Go: `newPool()` accepts `[]addressRange` and builds a composite pool with one segment per range
- Config parser: new list format is primary; detect and accept old single-range format for migration

## Data Flow (MANDATORY)

### Entry Point
- Config tree JSON -> `parseConfig()` -> `subnetConfig` -> `newDHCPHandler()` -> `newPool()`

### Transformation Path
1. Config tree delivers `data["range"]` as `map[string]any`
2. `parseSubnet()` detects format: if keys include `start`/`stop` directly, it's the old single-range format; if keys are names mapping to objects, it's the new list format
3. Parser produces `[]addressRange`, validates each range within subnet, checks for overlaps
4. `newPool()` accepts `[]addressRange`, sorts by start address, builds one bitmap segment per range
5. `pool.allocate()` iterates segments in order, allocating from first with space

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG -> config tree | list serialization as `map[name]->object` | [ ] |
| Config tree -> Go struct | `parseSubnet` format detection | [ ] |
| Go struct -> pool | `addressRange` slice | [ ] |

### Integration Points
- `newDHCPHandler()` in `handler.go` passes ranges to pool constructor
- Existing tests in `config_test.go`, `pool_test.go`, `handler_test.go`

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] Extends existing pool, doesn't recreate
- [ ] Zero-copy not applicable (config parsing)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `range { pool1 { ... } pool2 { ... } }` | -> | `parseSubnet()` producing `[]addressRange` | `TestParseConfigMultipleRanges` |
| `subnetConfig` with 2 ranges | -> | `newPool()` composite allocation | `TestPoolMultipleRanges` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with single named range | Parses correctly, single-range pool works as before |
| AC-2 | Config with two named ranges in same subnet | Both ranges allocatable; first range filled first |
| AC-3 | Static mapping IP within second range | IP excluded from dynamic allocation in that range |
| AC-4 | All addresses in first range exhausted | Allocation continues from second range |
| AC-5 | Overlapping ranges | Config parse error: ranges must not overlap |
| AC-6 | Range outside subnet | Config parse error (existing validation preserved) |
| AC-7 | Old single-range format `range { start ... stop ... }` | Still accepted via format detection |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseConfigMultipleRanges` | `config_test.go` | Two named ranges parsed | |
| `TestParseConfigSingleNamedRange` | `config_test.go` | Single named range (new format) | |
| `TestParseConfigOldRangeFormat` | `config_test.go` | Backward compat detection | |
| `TestParseConfigOverlappingRanges` | `config_test.go` | Overlap detection error | |
| `TestPoolMultipleRanges` | `pool_test.go` | Allocation across two disjoint ranges | |
| `TestPoolMultipleRangesExhaustion` | `pool_test.go` | First range full, second used | |
| `TestPoolMultipleRangesStatic` | `pool_test.go` | Static in second range excluded | |
| `TestPoolMultipleRangesStats` | `pool_test.go` | Total = sum of all range sizes | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Ranges per subnet | 1..10 | 10 | 0 (static-only subnet) | 11 |
| Range start/stop | within subnet | last subnet addr | N/A | outside subnet |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-dhcp-multi-range` | `test/plugin/dhcp-multi-range.ci` | Subnet with 2 ranges serves from both | |

## Files to Modify
- `internal/plugins/dhcpserver/schema/ze-dhcp-server-conf.yang` - `container range` -> `list range`
- `internal/plugins/dhcpserver/config.go` - new `addressRange` type, update `subnetConfig`, format detection, overlap validation
- `internal/plugins/dhcpserver/pool.go` - composite pool with segments
- `internal/plugins/dhcpserver/handler.go` - pass `[]addressRange` to pool

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [x] | `ze-dhcp-server-conf.yang` |
| CLI commands/flags | [ ] | N/A |
| Editor autocomplete | [x] | YANG-driven (automatic) |
| Functional test | [x] | `test/plugin/dhcp-multi-range.ci` |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` (DHCP example) |

## Files to Create
- `test/plugin/dhcp-multi-range.ci`

## Implementation Steps

### Implementation Phases

1. **Phase: Wiring** - Register new YANG list, create failing config parse test
   - Tests: `TestParseConfigMultipleRanges` (fails: parser doesn't handle list format)
   - Files: `ze-dhcp-server-conf.yang`, `config_test.go`
2. **Phase: Config parser** - Add `addressRange`, format detection, overlap validation
   - Tests: All config_test additions
   - Files: `config.go`, `config_test.go`
3. **Phase: Pool** - Composite pool with segments
   - Tests: `TestPoolMultipleRanges*`
   - Files: `pool.go`, `pool_test.go`
4. **Phase: Handler wiring** - Update `newDHCPHandler`
   - Tests: handler_test with multi-range
   - Files: `handler.go`
5. **Functional tests**
6. **Full verification** - `make ze-verify`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 7 ACs implemented |
| Correctness | Overlap detection handles adjacent ranges (stop1 == start2 is OK, stop1 > start2 is overlap) |
| Naming | YANG: `list range`, key `name`, leaves `start`/`stop` |
| Data flow | Ranges sorted by start address before pool construction |
| Backward compat | Old format detection tested |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| YANG list range | `grep "list range" ze-dhcp-server-conf.yang` |
| Multi-range parsing | `go test ./internal/plugins/dhcpserver/ -run MultipleRanges` |
| Overlap detection | `go test ./internal/plugins/dhcpserver/ -run OverlappingRanges` |
| Composite pool | `go test ./internal/plugins/dhcpserver/ -run PoolMultiple` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Ranges within subnet, no overlap, start <= stop |
| Resource exhaustion | Max 10 ranges per subnet caps bitmap allocation |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Old format detection fails | Research config tree serialization of list vs container |
| Pool bitmap math wrong | Fix in pool phase, add edge case tests |
| Overlap detection edge case | Add test, fix comparison |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| "stop1 == start2 is OK" for overlap | With inclusive ranges, same IP in two segments | Critical review of overlap logic | Fixed to use strict `>` check |

## Design Insights

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Backward compatibility proven

### Quality Gates
- [ ] Implementation Audit complete
