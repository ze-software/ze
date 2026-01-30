# Spec: 03 - EVPN Family Plugin

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/evpn/plugin.go` - current plugin implementation
4. `internal/plugin/evpn/types.go` - EVPN types
5. `internal/plugin/flowspec/types.go` - reference for correct dependency pattern

## Task

Complete the EVPN plugin migration by **breaking the import cycle** and integrating the plugin with the codebase.

**Current State:** The evpn plugin package exists but creates an import cycle because `nlri/evpn.go` re-exports types from the evpn package while evpn imports nlri.

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/plugin/evpn/plugin.go` - Plugin implementation with decode mode
- [x] `internal/plugin/evpn/types.go` - EVPN types that import from nlri
- [x] `internal/plugin/bgp/nlri/evpn.go` - Re-exports that cause the cycle
- [x] `internal/plugin/flowspec/types.go` - Reference for correct pattern

**Behavior to preserve:**
- EVPN types: `EVPN`, `EVPNType1-5`, `EVPNGeneric`, `ESI`, `EVPNRouteType`
- Functions: `ParseEVPN`, `NewEVPNType1-5`, `ParseESIString`
- Wire format: All route types encode/decode correctly
- JSON output format from decode mode

**Behavior to change:**
- Import path for EVPN types: `nlri.EVPN*` → `evpn.EVPN*`
- Delete re-export layer in `nlri/evpn.go`

## The Import Cycle Problem

### Current Dependency Graph (BROKEN)
```
internal/plugin/bgp/nlri/evpn.go
    imports → internal/plugin/evpn  (for re-exporting types)

internal/plugin/evpn/types.go
    imports → internal/plugin/bgp/nlri  (for Family, RouteDistinguisher, etc.)

RESULT: nlri → evpn → nlri = CYCLE!
```

### Target Dependency Graph (CORRECT - like flowspec)
```
internal/plugin/evpn/types.go
    imports → internal/plugin/bgp/nlri  (one-way, for shared types)

internal/plugin/bgp/nlri/
    does NOT import evpn  (no re-exports)

Consumers that need EVPN types:
    import → internal/plugin/evpn  (directly)
```

### Why FlowSpec Works
The flowspec plugin follows this pattern:
- `flowspec/types.go` imports `nlri` for `Family`, `RouteDistinguisher`, etc.
- `nlri` does NOT re-export flowspec types
- Consumers import `flowspec` directly when they need FlowSpec types

**This is the pattern we must follow for EVPN.**

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/cli/plugin-modes.md` - **CRITICAL**: Plugin CLI/Engine mode interface spec
- [ ] `internal/plugin/flowspec/types.go` - correct dependency pattern
- [ ] `internal/plugin/evpn/plugin.go` - current plugin implementation
- [ ] `internal/plugin/evpn/types.go` - current types with imports

### RFC Summaries
- [ ] `rfc/short/rfc7432.md` - EVPN base
- [ ] `rfc/short/rfc9136.md` - EVPN updates (Type 5)

## Files to Modify

- `cmd/ze/bgp/decode.go` - change `nlri.EVPN*` → `evpn.*`, add evpn import
- `cmd/ze/bgp/encode.go` - change `nlri.EVPN*` → `evpn.*`, add evpn import
- `internal/plugin/update_text.go` - change `nlri.NewEVPNType*` → `evpn.NewEVPNType*`
- `internal/plugin/update_text_test.go` - change `nlri.EVPNType*` → `evpn.EVPNType*`
- `internal/plugin/text.go` - change `nlri.EVPNType2` → `evpn.EVPNType2`
- `internal/plugin/json_test.go` - change `nlri.EVPN*` → `evpn.*`
- `internal/plugin/bgp/message/update_build.go` - change `nlri.EVPN*` → `evpn.*`

## Files to Delete

- `internal/plugin/bgp/nlri/evpn.go` - re-exports that create the import cycle

## Files Analysis

### Already Implemented (Keep)
| File | Status | Notes |
|------|--------|-------|
| `internal/plugin/evpn/plugin.go` | ✅ Done | Decode mode, event loop, startup protocol |
| `internal/plugin/evpn/types.go` | ✅ Done | All 5 route types, ParseEVPN, constructors |
| `internal/plugin/inprocess.go` | ✅ Done | evpn registered in internalPluginRunners |
| `cmd/ze/bgp/plugin_evpn.go` | ✅ Done | CLI entry point with --decode flag |

### To Delete (Breaks Cycle)
| File | Reason |
|------|--------|
| `internal/plugin/bgp/nlri/evpn.go` | Re-exports that create the import cycle |
| `internal/plugin/bgp/nlri/evpn_test.go` | Tests for moved code (already deleted per git status) |

### To Modify (Update Imports)
| File | Changes |
|------|---------|
| `cmd/ze/bgp/decode.go` | `nlri.EVPN*` → `evpn.*`, add evpn import |
| `cmd/ze/bgp/encode.go` | `nlri.EVPN*` → `evpn.*`, add evpn import |
| `internal/plugin/update_text.go` | `nlri.NewEVPNType*` → `evpn.NewEVPNType*` |
| `internal/plugin/update_text_test.go` | `nlri.EVPNType*` → `evpn.EVPNType*` |
| `internal/plugin/text.go` | `nlri.EVPNType2` → `evpn.EVPNType2` |
| `internal/plugin/json_test.go` | `nlri.EVPNType*` → `evpn.EVPNType*` |
| `internal/plugin/bgp/message/update_build.go` | `nlri.EVPN*` → `evpn.*` |

## Import Changes Detail

### cmd/ze/bgp/decode.go
**Current imports:**
```
"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/nlri"
```
**Add:**
```
"codeberg.org/thomas-mangin/ze/internal/plugin/evpn"
```
**Replace patterns:**
- `nlri.ParseEVPN` → `evpn.ParseEVPN`
- `nlri.ESI` → `evpn.ESI`
- `nlri.EVPN` → `evpn.EVPN`
- `nlri.EVPNGeneric` → `evpn.EVPNGeneric`
- `nlri.EVPNType1` → `evpn.EVPNType1`
- `nlri.EVPNType2` → `evpn.EVPNType2`
- `nlri.EVPNType3` → `evpn.EVPNType3`
- `nlri.EVPNType4` → `evpn.EVPNType4`
- `nlri.EVPNType5` → `evpn.EVPNType5`

### cmd/ze/bgp/encode.go
**Add import:**
```
"codeberg.org/thomas-mangin/ze/internal/plugin/evpn"
```
**Replace patterns:**
- `nlri.EVPN` → `evpn.EVPN`
- `nlri.NewEVPNType1` → `evpn.NewEVPNType1`
- `nlri.NewEVPNType2` → `evpn.NewEVPNType2`
- `nlri.NewEVPNType3` → `evpn.NewEVPNType3`
- `nlri.NewEVPNType4` → `evpn.NewEVPNType4`
- `nlri.NewEVPNType5` → `evpn.NewEVPNType5`

### internal/plugin/update_text.go
**Add import:**
```
"codeberg.org/thomas-mangin/ze/internal/plugin/evpn"
```
**Replace patterns:**
- `nlri.NewEVPNType2` → `evpn.NewEVPNType2`
- `nlri.NewEVPNType3` → `evpn.NewEVPNType3`
- `nlri.NewEVPNType5` → `evpn.NewEVPNType5`

### internal/plugin/update_text_test.go
**Add import:**
```
"codeberg.org/thomas-mangin/ze/internal/plugin/evpn"
```
**Replace patterns:**
- `*nlri.EVPNType2` → `*evpn.EVPNType2`
- `*nlri.EVPNType3` → `*evpn.EVPNType3`
- `*nlri.EVPNType5` → `*evpn.EVPNType5`

### internal/plugin/text.go
**Add import:**
```
"codeberg.org/thomas-mangin/ze/internal/plugin/evpn"
```
**Replace patterns:**
- `*nlri.EVPNType2` → `*evpn.EVPNType2`

### internal/plugin/json_test.go
**Add import:**
```
"codeberg.org/thomas-mangin/ze/internal/plugin/evpn"
```
**Replace patterns:**
- `*nlri.EVPNType2` → `*evpn.EVPNType2`
- `nlri.EVPNRouteType2` → `evpn.EVPNRouteType2`
- `nlri.ParseEVPN` → `evpn.ParseEVPN`

### internal/plugin/bgp/message/update_build.go
**Add import:**
```
"codeberg.org/thomas-mangin/ze/internal/plugin/evpn"
```
**Replace patterns:**
- `nlri.EVPN` → `evpn.EVPN`
- `nlri.NewEVPNType1` → `evpn.NewEVPNType1`
- `nlri.NewEVPNType2` → `evpn.NewEVPNType2`
- `nlri.NewEVPNType3` → `evpn.NewEVPNType3`
- `nlri.NewEVPNType4` → `evpn.NewEVPNType4`
- `nlri.NewEVPNType5` → `evpn.NewEVPNType5`

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

### Phase 1: Break the Cycle

1. **Delete `nlri/evpn.go`** - This is the file that creates the cycle
   ```bash
   rm internal/plugin/bgp/nlri/evpn.go
   ```
   → **Review:** This will cause compilation errors. That's expected and correct.

### Phase 2: Update Consumers (One at a Time)

2. **Update `cmd/ze/bgp/decode.go`**
   - Add import: `"codeberg.org/thomas-mangin/ze/internal/plugin/evpn"`
   - Replace all `nlri.EVPN*` with `evpn.*`
   - Test: `go build ./cmd/ze/bgp/...`
   → **Review:** File compiles? All EVPN references updated?

3. **Update `cmd/ze/bgp/encode.go`**
   - Add import: `"codeberg.org/thomas-mangin/ze/internal/plugin/evpn"`
   - Replace all `nlri.EVPN*` with `evpn.*`
   - Test: `go build ./cmd/ze/bgp/...`
   → **Review:** File compiles?

4. **Update `internal/plugin/update_text.go`**
   - Add import: `"codeberg.org/thomas-mangin/ze/internal/plugin/evpn"`
   - Replace all `nlri.NewEVPNType*` with `evpn.NewEVPNType*`
   - Test: `go build ./internal/plugin/...`
   → **Review:** File compiles?

5. **Update `internal/plugin/update_text_test.go`**
   - Add import: `"codeberg.org/thomas-mangin/ze/internal/plugin/evpn"`
   - Replace all `nlri.EVPNType*` with `evpn.EVPNType*`
   - Test: `go test ./internal/plugin/... -run EVPN`
   → **Review:** Tests compile and pass?

6. **Update `internal/plugin/text.go`**
   - Add import: `"codeberg.org/thomas-mangin/ze/internal/plugin/evpn"`
   - Replace `*nlri.EVPNType2` with `*evpn.EVPNType2`
   - Test: `go build ./internal/plugin/...`
   → **Review:** File compiles?

7. **Update `internal/plugin/json_test.go`**
   - Add import: `"codeberg.org/thomas-mangin/ze/internal/plugin/evpn"`
   - Replace all `nlri.EVPN*` with `evpn.*`
   - Test: `go test ./internal/plugin/... -run EVPN`
   → **Review:** Tests compile and pass?

8. **Update `internal/plugin/bgp/message/update_build.go`**
   - Add import: `"codeberg.org/thomas-mangin/ze/internal/plugin/evpn"`
   - Replace all `nlri.EVPN*` with `evpn.*`
   - Test: `go build ./internal/plugin/bgp/message/...`
   → **Review:** File compiles?

### Phase 3: Verification

9. **Full build** - `go build ./...`
   → **Review:** No compilation errors?

10. **Run lint** - `make lint`
    → **Review:** No new lint errors? (paste output)

11. **Run tests** - `make test`
    → **Review:** All tests pass? (paste output)

12. **Run functional** - `make functional`
    → **Review:** All functional tests pass? (paste output)

### Phase 4: Add EVPN-Specific Tests (if missing)

13. **Check test coverage** for evpn package
    ```bash
    go test -cover ./internal/plugin/evpn/...
    ```
    → **Review:** Coverage adequate? Add tests if < 80%

14. **Add functional test** if missing: `test/decode/evpn-*.ci`
    → **Review:** Tests decode mode via CLI?

## 🧪 TDD Test Plan

### Unit Tests (Existing in evpn package)
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Wire roundtrip Type 1 | `internal/plugin/evpn/types_test.go` | Ethernet Auto-Discovery | Check |
| Wire roundtrip Type 2 | `internal/plugin/evpn/types_test.go` | MAC/IP Advertisement | Check |
| Wire roundtrip Type 3 | `internal/plugin/evpn/types_test.go` | Inclusive Multicast | Check |
| Wire roundtrip Type 4 | `internal/plugin/evpn/types_test.go` | Ethernet Segment | Check |
| Wire roundtrip Type 5 | `internal/plugin/evpn/types_test.go` | IP Prefix | Check |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Route type | 1-5 | 5 | 0 (EVPNGeneric) | 6+ (EVPNGeneric) |
| Ethernet tag | 0-0xFFFFFFFF | 0xFFFFFFFF | N/A | N/A (32-bit saturates) |
| MPLS label | 0-0xFFFFF | 0xFFFFF | N/A | 0x100000 (truncated) |
| ESI length | 10 bytes | 10 | 9 (ErrEVPNTruncated) | 11 (ignored) |
| MAC len (Type 2) | 48 bits | 48 | 47 (ErrEVPNInvalidAddress) | 49 (invalid) |
| IP len (Type 2) | 0,32,128 | 128 | 1-31 (invalid) | 129+ (invalid) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| decode-evpn | `test/decode/evpn-*.ci` | Decode EVPN NLRI via CLI | Check if exists |

## Design Decisions

### Why Not Keep Re-exports?
**Options considered:**
1. Keep `nlri/evpn.go` with re-exports → Creates import cycle (REJECTED)
2. Extract shared types to `internal/plugin/bgp/types/` → Over-engineering (REJECTED)
3. Delete re-exports, update consumers → Simple, follows flowspec pattern (CHOSEN)

**Decision:** Follow the flowspec pattern - no re-exports, consumers import directly.

### Import Hierarchy
```
                   ┌─────────────────┐
                   │ nlri (shared    │
                   │ types: Family,  │
                   │ RouteDistinguisher)
                   └────────┬────────┘
                            │
              ┌─────────────┼─────────────┐
              ▼             ▼             ▼
         ┌────────┐   ┌──────────┐   ┌─────────┐
         │flowspec│   │   evpn   │   │  ipvpn  │
         └────────┘   └──────────┘   └─────────┘
              │             │             │
              └─────────────┼─────────────┘
                            ▼
                    ┌────────────────┐
                    │ Consumers      │
                    │ (decode.go,    │
                    │  encode.go,    │
                    │  update_text,  │
                    │  etc.)         │
                    └────────────────┘
```

## RFC Documentation

### Reference Comments
- RFC 7432 Section 7 - NLRI format
- RFC 7432 Section 7.1 - Route type 1 (Ethernet Auto-Discovery)
- RFC 7432 Section 7.2 - Route type 2 (MAC/IP Advertisement)
- RFC 7432 Section 7.3 - Route type 3 (Inclusive Multicast)
- RFC 7432 Section 7.4 - Route type 4 (Ethernet Segment)
- RFC 9136 Section 3 - Route type 5 (IP Prefix)

## Implementation Summary

### What Was Implemented
- Broke import cycle by deleting `nlri/evpn.go` (re-exports that caused cycle)
- Updated all consumers to import `evpn` package directly:
  - `cmd/ze/bgp/decode.go` - EVPN JSON decoding
  - `cmd/ze/bgp/encode.go` - EVPN encoding
  - `internal/plugin/update_text.go` - text format EVPN construction
  - `internal/plugin/text.go` - EVPN Type2 handling
  - `internal/plugin/json_test.go` - EVPN test cases
  - `internal/plugin/bgp/message/update_build.go` - UPDATE message building
- Added `plugin_test.go` with unit tests for plugin.go functions
- Registered evpn plugin in `inprocess.go` with family→plugin mapping
- CLI entry point `plugin_evpn.go` with three modes:
  - CLI mode: `--json <hex>` or `--text <hex>` (use `-` for stdin)
  - Engine decode mode: `--decode` flag, protocol commands on stdin
  - Engine mode: full plugin with startup protocol
- Added `RunCLIDecode()` function for direct CLI hex decoding

### Bugs Found/Fixed
- Import cycle: `nlri → evpn → nlri` caused by re-exports in `nlri/evpn.go`
- Fixed by following flowspec pattern: evpn imports nlri (one-way), no re-exports
- Text formatter used wrong keys (`route-type-name` vs `name`, `originator-ip` vs `originator`)
- Fixed by aligning formatter keys with evpnToJSON output

### Design Insights
- Family plugins should own their types (evpn owns EVPN*, flowspec owns FlowSpec*)
- Shared types (Family, RouteDistinguisher) belong in nlri package
- Consumers import family-specific packages directly, not through nlri
- CLI mode uses `--json <hex>` / `--text <hex>` - format explicit in flag name
- Engine decode mode uses `--decode` (bool) for backwards compatibility

### Deviations from Plan
- Did not simplify verbose IP length switch statements (hook blocks `default:` pattern)
- Coverage achieved: 86.0% (exceeds 80% target)
- CLI uses `--json`/`--text` flags instead of single `--decode <hex>` flag

## CLI Mode Implementation (COMPLETED)

CLI mode implemented with `--json <hex>` and `--text <hex>` flags for human use.

### Implemented CLI Interface

```bash
# CLI Mode: --json or --text with hex value
ze bgp plugin evpn --json 02210001252C...   # JSON output
ze bgp plugin evpn --text 02210001252C...   # text output
ze bgp plugin evpn --json -                 # read hex from stdin

# Engine Decode Mode: protocol commands on stdin
ze bgp plugin evpn --decode                 # reads "decode nlri ..." from stdin

# Engine Mode: full plugin with startup protocol
ze bgp plugin evpn                          # startup protocol, event loop
```

### Implemented Flags

| Flag | Type | Description |
|------|------|-------------|
| `--json <hex\|->` | string | CLI mode: decode hex, output JSON (use `-` for stdin) |
| `--text <hex\|->` | string | CLI mode: decode hex, output text (use `-` for stdin) |
| `--decode` | bool | Engine decode protocol mode |

### Design Decision

- `--json <hex>` / `--text <hex>`: Clean CLI for humans, format is explicit in flag name
- `--decode` (bool): Engine compatibility, protocol commands on stdin
- No flags: Full plugin mode with startup protocol

This pattern will be applied to all decode-capable plugins (flowspec, hostname, etc.).

## Checklist

### 🏗️ Design
- [x] No premature abstraction (following existing flowspec pattern)
- [x] No speculative features (only breaking the cycle)
- [x] Single responsibility (evpn package owns EVPN types)
- [x] Explicit behavior (direct imports, no re-export magic)
- [x] Minimal coupling (one-way nlri→evpn dependency)
- [x] Next-developer test (follows existing flowspec pattern)

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (verified during development)
- [x] Implementation complete
- [x] Tests PASS (86.0% coverage)
- [x] Boundary tests cover all numeric inputs (Type5 length validation)
- [x] Feature code integrated into codebase (`internal/plugin/evpn/`)
- [x] Functional tests verify end-user behavior (`test/decode/bgp-evpn-1.ci`)

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes (51 tests)
- [x] `make functional` passes (80 tests)

### Documentation
- [x] Required docs read
- [x] RFC summaries read (7432, 9136)
- [x] RFC references added to code (types.go has RFC section comments)
- [x] RFC constraint comments added (IP length validation comments)

### Completion
- [x] Architecture docs updated with learnings (`docs/architecture/cli/plugin-modes.md`)
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/`
- [ ] All files committed together
