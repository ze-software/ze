# Spec: mandatory-local-address

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/bgp/schema/ze-bgp-conf.yang` - YANG peer-fields grouping
4. `internal/config/bgp.go` - PeerConfig parsing

## Task

Make `local-address` mandatory for all peers. Without explicit local address, TCP source IP selection is OS-dependent, causing:
- Inconsistent next-hop self behavior
- Connection issues with multi-homed routers
- Difficulty debugging connection problems

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config validation patterns

### Source Files
- [ ] `internal/plugin/bgp/schema/ze-bgp-conf.yang` - YANG schema (line 110-113: local-address leaf)
- [ ] `internal/config/bgp.go` - Go parsing (lines 917-927: local-address parsing)
- [ ] `internal/config/bgp_test.go` - existing tests with local-address

**Key insights:**
- `peer-as` already uses `mandatory true;` pattern (line 122)
- `local-address` accepts IP address string or "auto" keyword
- "auto" sets `LocalAddressAuto=true` flag for OS-selected binding
- Most existing tests already include `local-address`

## Design Decisions

### "auto" Remains Valid

The `local-address auto;` syntax is still accepted. It means:
- Explicit acknowledgment that OS chooses source IP
- User consciously opted for non-deterministic behavior
- Better than silent default

### Validation Location

Validation happens in Go config parsing, not YANG:
- YANG `mandatory true;` only checks presence, not value validity
- Go parsing already validates IP format
- Add check: reject if neither `LocalAddress` nor `LocalAddressAuto` is set

## Current Behavior

**Source files read:**
- [ ] `internal/plugin/bgp/schema/ze-bgp-conf.yang` - `local-address` leaf is optional (no `mandatory` keyword)
- [ ] `internal/plugin/bgp/reactor/config.go` - `parsePeerFromTree()` reads `local-address` via `mapString()`, only validates format if present; silently skips if absent

**Behavior to preserve:**
- `local-address auto;` accepted — sets no `LocalAddress`, OS picks source IP
- `local-address <ip>;` accepted — validates IP and sets `ps.LocalAddress`
- Invalid IP format rejected with `invalid local-address` error

**Behavior to change:**
- Missing `local-address` key now returns error instead of silently defaulting to OS selection

## Data Flow

### Entry Point
- Config file parsed into Tree, then resolved into `map[string]any` peer tree

### Transformation Path
1. Config file → `config.LoadReactor()` → Tree
2. Tree → `reactor.PeersFromTree()` → iterates peer entries
3. Per-peer `map[string]any` → `parsePeerFromTree()` → validates `local-address` presence
4. Returns `PeerSettings` or error

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → Reactor | `map[string]any` peer tree | [x] |

### Integration Points
- `parsePeerFromTree()` already handles `local-address` parsing — validation added at the same location

### Architectural Verification
- [x] No bypassed layers
- [x] No unintended coupling
- [x] No duplicated functionality
- [x] Zero-copy not applicable (config parsing)

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPeerMissingLocalAddress` | `internal/config/bgp_test.go` | Peer without local-address rejected | |
| `TestPeerLocalAddressAuto` | `internal/config/bgp_test.go` | "auto" is accepted | ✅ exists |
| `TestPeerLocalAddressIP` | `internal/config/bgp_test.go` | IP address is accepted | ✅ exists |

### Boundary Tests
N/A - this is presence validation, not numeric range.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `peer-missing-local-address` | `test/data/parse/invalid/peer-missing-local-address.conf` | Config rejected with clear error | |

## Files to Modify

- `internal/plugin/bgp/schema/ze-bgp-conf.yang` - Add `mandatory true;` to local-address leaf
- `internal/config/bgp.go` - Add validation after parsing peer config
- `internal/config/bgp_test.go` - Add rejection test

## Files to Create

- `test/data/parse/invalid/peer-missing-local-address.conf` - Invalid config
- `test/data/parse/invalid/peer-missing-local-address.expect` - Expected error

## Implementation Steps

1. **Write unit test** - Test that peer without local-address is rejected
   → **Review:** Does test check error message mentions "local-address"?

2. **Run tests** - Verify FAIL (paste output)
   → **Review:** Test fails because validation doesn't exist yet?

3. **Add YANG mandatory** - Add `mandatory true;` to local-address leaf
   → **Review:** Matches peer-as pattern at line 122?

4. **Add Go validation** - Check in `parsePeerConfig()` after parsing
   → **Review:** Clear error message? Mentions "local-address required"?

5. **Run tests** - Verify PASS (paste output)
   → **Review:** All existing tests still pass?

6. **Add functional test** - Create invalid config test
   → **Review:** Error expectation matches actual message?

7. **Verify all** - `make lint && make test && make functional` (paste output)

## Error Message

```
peer 192.0.2.1: local-address is required (use IP address or "auto")
```

## Implementation Summary

### What Was Implemented
- YANG schema: added `mandatory true;` to `local-address` leaf in `ze-bgp-conf.yang`
- Go validation: `parsePeerFromTree()` in `reactor/config.go` now rejects peers missing `local-address`
- Error message: `peer <addr>: local-address is required (use IP address or "auto")`
- Unit test: `TestParsePeerMissingLocalAddress` in `reactor/config_test.go`
- Functional test: `test/parse/missing-local-address.ci`
- Fixed all existing tests that were missing `local-address` in their configs

### Bugs Found/Fixed
- Two functional tests (`graceful-restart-no-process.ci`, `route-refresh-no-process.ci`) failed because mandatory validation fires before capability-specific checks. Fixed by adding `local-address` to their test configs.

### Design Insights
- Validation lives in `reactor/config.go:parsePeerFromTree()`, not `config/bgp.go` — the spec originally listed `config/bgp.go` but actual peer parsing happens in reactor.

### Documentation Updates
- None — no architectural changes

### Deviations from Plan
- Validation added in `reactor/config.go` (not `config/bgp.go` as spec listed) — this is where `parsePeerFromTree()` lives
- Functional test created as `test/parse/missing-local-address.ci` (not `test/data/parse/invalid/` as spec listed) — matches existing test directory structure
- Two additional functional test files fixed: `graceful-restart-no-process.ci`, `route-refresh-no-process.ci`

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Make local-address mandatory | ✅ Done | `reactor/config.go:80-83` | Returns error if missing |
| "auto" still accepted | ✅ Done | `reactor/config.go:84` | Existing `v != "auto"` check preserved |
| Clear error message | ✅ Done | `reactor/config.go:82` | `peer <addr>: local-address is required (use IP address or "auto")` |
| YANG schema updated | ✅ Done | `ze-bgp-conf.yang:112` | `mandatory true;` added |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestParsePeerMissingLocalAddress` | ✅ Done | `reactor/config_test.go:590` | Verifies error contains "local-address is required" |
| `TestPeerLocalAddressAuto` | ✅ Exists | `reactor/config_test.go:601` | Pre-existing, still passes |
| `TestPeerLocalAddressIP` | ✅ Exists | `reactor/config_test.go` | Pre-existing via `TestParsePeerFromTree` |
| `missing-local-address` functional | ✅ Done | `test/parse/missing-local-address.ci` | exit:code=1 + stderr:contains check |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugin/bgp/schema/ze-bgp-conf.yang` | ✅ Modified | `mandatory true;` + updated description |
| `internal/plugin/bgp/reactor/config.go` | ✅ Modified | 🔄 Was listed as `config/bgp.go` in spec |
| `internal/plugin/bgp/reactor/config_test.go` | ✅ Modified | 🔄 Was listed as `config/bgp_test.go` in spec |
| `internal/config/loader_test.go` | ✅ Modified | Added `local-address` to test configs |
| `internal/config/peers_test.go` | ✅ Modified | Added `local-address` to test configs |
| `internal/config/reader_test.go` | ✅ Modified | Added `local-address` to test configs |
| `test/parse/missing-local-address.ci` | ✅ Created | Functional test |
| `test/parse/graceful-restart-no-process.ci` | ✅ Modified | Added `local-address` so capability check runs |
| `test/parse/route-refresh-no-process.ci` | ✅ Modified | Added `local-address` so capability check runs |

### Audit Summary
- **Total items:** 13
- **Done:** 13
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (file paths differ from spec — documented in Deviations)

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL
- [x] Implementation complete
- [x] Tests PASS
- [x] Feature code integrated into codebase (`internal/*`)
- [x] Functional tests verify end-user behavior (`.ci` files)

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] Required docs read
- [x] YANG schema updated
- [x] Error message is clear and actionable
