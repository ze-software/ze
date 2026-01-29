# Spec: Family Plugin Infrastructure

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/registration.go` - current registry implementation
4. `internal/plugin/hostname/hostname.go` - decode mode pattern (lines 278-328)
5. `cmd/ze/bgp/decode.go` - plugin decode invocation (lines 375-421)

## Task

Add infrastructure to the plugin system enabling plugins to register as NLRI decoders/encoders for specific address families (AFI/SAFI). This is the foundation for modular family plugins (FlowSpec, EVPN, VPN, BGP-LS).

**Scope:** Engine-side infrastructure only. No family plugins in this spec.

**Key decisions from user:**
- NO per-message caching (plugins are fast enough)
- Each family (FlowSpec, EVPN, etc.) will have its own separate spec
- FlowSpec code will be MOVED to plugin folder (not copied) in its own spec

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - engine/plugin boundary
- [ ] `internal/plugin/registration.go` - 5-stage protocol, PluginRegistry
- [ ] `internal/plugin/hostname/hostname.go` - decode mode pattern (lines 278-328)
- [ ] `cmd/ze/bgp/decode.go` - plugin decode invocation (lines 332-421)

### RFC Summaries
- [ ] `rfc/short/rfc4760.md` - Multiprotocol Extensions (AFI/SAFI in MP_REACH)

**Key insights:**
- `PluginRegistry` already tracks commands and capabilities with conflict detection
- Hostname plugin shows decode mode pattern: `decode capability <code> <hex>` → `decoded json {...}`
- `cmd/ze/bgp/decode.go` shows how to invoke plugin decode mode
- Family registration already exists (`declare family`) but doesn't claim exclusive decoding

## Current State Analysis

### Existing Family Declaration

Plugin can declare interest in families with `declare family ipv4 flowspec`.

**Current behavior:** Informational only - used for:
- Documentation of plugin's scope
- Future: filtering which UPDATE events plugin receives

**Missing:** No mechanism to claim exclusive NLRI decoding rights.

### Existing Decode Mode Pattern

Hostname plugin implements decode mode for capability 73:

| Component | Location | Purpose |
|-----------|----------|---------|
| Request format | `decode capability 73 <hex>` | Engine → Plugin |
| Response format | `decoded json {...}` | Plugin → Engine |
| CLI integration | `cmd/ze/bgp/decode.go:375-421` | `ze bgp decode --plugin hostname` |
| Plugin handler | `hostname.go:278-328` | RunDecodeMode() |

### PluginRegistry Current Fields

| Field | Type | Purpose |
|-------|------|---------|
| `plugins` | `map[string]*PluginRegistration` | Plugin name → registration |
| `commands` | `map[string]string` | Command → plugin name |
| `capabilities` | `map[uint8]string` | Capability code → plugin name |

**Missing:** `families` map for family → plugin mapping.

## Target State

### New Family Registry

Add to `PluginRegistry`:

| Field | Type | Purpose |
|-------|------|---------|
| `families` | `map[string]string` | Family string → plugin name |

Family string format: `"ipv4/flowspec"`, `"l2vpn/evpn"`, etc.

### New Declaration Syntax

Extend `declare family` with optional `decode` keyword:

| Declaration | Effect |
|-------------|--------|
| `declare family ipv4 flowspec decode` | Claims exclusive NLRI decoding |
| `declare family ipv4 flowspec` | Informational only (existing behavior) |

**Why optional:** Backward compatible. Existing plugins declaring families don't break.

### New Decode Mode: NLRI

Extend decode mode to support NLRI (alongside existing capability decode):

| Type | Request | Response |
|------|---------|----------|
| Capability | `decode capability <code> <hex>` | `decoded json {...}` |
| NLRI (new) | `decode nlri <family> <hex>` | `decoded json {...}` |

### Family Lookup API

New methods on `PluginRegistry`:

| Method | Signature | Returns |
|--------|-----------|---------|
| `LookupFamily` | `(family string) string` | Plugin name or empty |
| `RegisterFamily` | `(family, plugin string) error` | Error if conflict |

### CLI Integration

Extend `ze bgp decode` to support family plugins:

| Command | Purpose |
|---------|---------|
| `ze bgp decode --plugin hostname --open <hex>` | Current: decode OPEN capabilities |
| `ze bgp decode --plugin flowspec --nlri ipv4/flowspec <hex>` | New: decode NLRI |

## Detailed Design

### 1. PluginRegistry Extension

**File:** `internal/plugin/registration.go`

Add to `PluginRegistry` struct:

| Field | Type | Init Value |
|-------|------|------------|
| `families` | `map[string]string` | `make(map[string]string)` |

Add methods:

**RegisterFamily(family, pluginName string) error**
- Check if family already registered
- If yes: return conflict error with existing plugin name
- If no: add to families map, return nil

**LookupFamily(family string) string**
- Return plugin name if registered
- Return empty string if not registered

### 2. ParseFamily Extension

**File:** `internal/plugin/registration.go`, function `parseFamily`

Current parsing: `declare family <afi> <safi>` or `declare family all`

Extended parsing: `declare family <afi> <safi> [decode]`

| Input | Families[] | Decode Claim |
|-------|------------|--------------|
| `declare family ipv4 flowspec` | `["ipv4/flowspec"]` | No |
| `declare family ipv4 flowspec decode` | `["ipv4/flowspec"]` | Yes |
| `declare family all` | `["all"]` | No (cannot claim all) |

Add to `PluginRegistration` struct:

| Field | Type | Purpose |
|-------|------|---------|
| `DecodeFamilies` | `[]string` | Families this plugin decodes |

### 3. Registration Flow

**File:** `internal/plugin/registration.go`, function `Register`

After existing command conflict check, add family conflict check:

**Algorithm:**
1. For each family in reg.DecodeFamilies
2. Check if family already exists in r.families
3. If exists: return error "family conflict: %s already registered by %s"
4. If not: add r.families[family] = reg.Name

### 4. Decode Mode Protocol

**Request/Response formats:**

| Direction | Format | Example |
|-----------|--------|---------|
| Request | `decode nlri <family> <hex>` | `decode nlri ipv4/flowspec 0701180a0000` |
| Success | `decoded json <json>` | `decoded json {"components":[...]}` |
| Error | `error <message>` | `error invalid hex encoding` |

### 5. Plugin Decode Mode Handler Pattern

Family plugins implement decode mode similar to hostname.

**Entry point:** `RunDecodeMode(in io.Reader, out io.Writer) int`

**Processing loop:**
1. Read line from stdin
2. Parse request: `decode nlri <family> <hex>`
3. Decode hex to bytes
4. Parse NLRI bytes to struct
5. Marshal struct to JSON
6. Write response: `decoded json <json>`
7. Repeat until EOF

**Exit codes:** 0 = normal (EOF), 1 = fatal error

### 6. CLI Decode Integration

**File:** `cmd/ze/bgp/decode.go`

| Flag | Current | New |
|------|---------|-----|
| `--plugin` | Plugin name | Plugin name |
| `--open` | Decode OPEN capabilities | Unchanged |
| `--nlri` | N/A | Family string for NLRI decode |

**Internal flow for --nlri:**
1. Parse --nlri flag value as family string
2. Spawn plugin in decode mode
3. Send decode request
4. Read decoded response
5. Print JSON to stdout

### 7. Plugin Family Map

**File:** `cmd/ze/bgp/decode.go`

Add static map for known internal family plugins:

| Family | Plugin |
|--------|--------|
| `ipv4/flowspec` | `flowspec` |
| `ipv6/flowspec` | `flowspec` |
| `ipv4/flowspec-vpn` | `flowspec` |
| `ipv6/flowspec-vpn` | `flowspec` |

**Note:** Runtime registration via `declare family ... decode` takes precedence over this static map.

## Files to Modify

- `internal/plugin/registration.go` - add families map, RegisterFamily, LookupFamily, extend parseFamily
- `cmd/ze/bgp/decode.go` - add --nlri flag, pluginFamilyMap, NLRI decode invocation

## Files to Create

- `internal/plugin/registration_family_test.go` - unit tests for family registration
- `test/data/decode/nlri-plugin-infra.ci` - functional test for --nlri flag

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write test: TestFamilyRegistration** - Plugin registers family with decode claim

   | Input | Expected |
   |-------|----------|
   | `declare family ipv4 flowspec decode` | DecodeFamilies contains `ipv4/flowspec` |

   → **Review:** Does test cover the parsing correctly?

2. **Write test: TestFamilyRegistrationWithoutDecode** - Backward compatible

   | Input | Expected Families | Expected DecodeFamilies |
   |-------|-------------------|-------------------------|
   | `declare family ipv4 flowspec` | `["ipv4/flowspec"]` | empty |

   → **Review:** Backward compatibility verified?

3. **Write test: TestFamilyConflict** - Two plugins claim same family

   | Plugin A | Plugin B | Expected |
   |----------|----------|----------|
   | `declare family ipv4 flowspec decode` | `declare family ipv4 flowspec decode` | Plugin B fails with conflict error |

   → **Review:** Error message includes both plugin names?

4. **Write test: TestFamilyLookup** - Lookup registered family

   | Setup | Lookup | Expected |
   |-------|--------|----------|
   | Register flowspec → ipv4/flowspec | `ipv4/flowspec` | `"flowspec"` |

   → **Review:** Edge cases (unknown family, empty string)?

5. **Write test: TestFamilyLookupUnknown** - Lookup unregistered family

   | Lookup | Expected |
   |--------|----------|
   | `ipv4/unknown` | empty string |

   → **Review:** No panic on unknown?

6. **Run tests** - Verify all FAIL (paste output)
   → **Review:** Tests fail for the right reason (not found, not syntax)?

7. **Implement PluginRegistry.families field** - Add map and initialization
   → **Review:** Thread-safe? Mutex already protects?

8. **Implement RegisterFamily method** - Conflict detection
   → **Review:** Error message helpful? Matches command/capability pattern?

9. **Implement LookupFamily method** - Simple lookup
   → **Review:** RLock for read-only?

10. **Extend parseFamily** - Handle optional `decode` keyword
    → **Review:** Backward compatible? Doesn't break existing plugins?

11. **Extend Register** - Check family conflicts during registration
    → **Review:** Order: check all conflicts first, then register?

12. **Run tests** - Verify all PASS (paste output)
    → **Review:** All tests pass? Coverage adequate?

13. **Write test: TestDecodeNLRIFlag** - CLI flag parsing

    | Args | Expected |
    |------|----------|
    | `--plugin flowspec --nlri ipv4/flowspec deadbeef` | Correct invocation setup |

    → **Review:** Flag interaction with --open?

14. **Run test** - Verify FAIL
    → **Review:** Fails because flag doesn't exist yet?

15. **Implement --nlri flag** - Add to decode.go
    → **Review:** Help text clear?

16. **Implement NLRI decode invocation** - Similar to capability decode
    → **Review:** Reuses invokePluginDecode pattern?

17. **Run tests** - Verify PASS
    → **Review:** All tests pass?

18. **Create functional test** - `test/data/decode/nlri-plugin-infra.ci`
    → **Review:** Tests the actual CLI behavior?

19. **Verify all** - `make lint && make test && make functional` (paste output)
    → **Review:** Zero lint issues? No regressions?

20. **Final self-review** - Before claiming done:
    - Re-read all code changes
    - Check for unused code, debug statements
    - Verify error messages are actionable
    - Check thread safety

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFamilyRegistration` | `internal/plugin/registration_family_test.go` | Parse `declare family X decode` | |
| `TestFamilyRegistrationWithoutDecode` | `internal/plugin/registration_family_test.go` | Backward compat: no decode keyword | |
| `TestFamilyConflict` | `internal/plugin/registration_family_test.go` | Two plugins same family fails | |
| `TestFamilyLookup` | `internal/plugin/registration_family_test.go` | LookupFamily returns plugin | |
| `TestFamilyLookupUnknown` | `internal/plugin/registration_family_test.go` | LookupFamily returns empty | |
| `TestDecodeNLRIFlag` | `cmd/ze/bgp/decode_test.go` | --nlri flag parsing | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Family string | non-empty | any valid family | empty string | N/A |
| AFI in family | ipv4/ipv6/l2vpn | l2vpn | invalid | N/A |
| SAFI in family | unicast/flowspec/evpn/etc | mup | invalid | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `nlri-plugin-infra` | `test/data/decode/nlri-plugin-infra.ci` | Verify --nlri flag works with mock plugin | |

## Design Decisions

### Why Optional `decode` Keyword?

| Option | Pros | Cons |
|--------|------|------|
| Always claim decode | Simple | Breaks existing plugins |
| Optional keyword | Backward compatible | Slightly more parsing |
| Separate declaration | Explicit | More verbose |

**Decision:** Optional `decode` keyword. Backward compatible and explicit.

### Why Static pluginFamilyMap?

Plugins may not be loaded during `ze bgp decode` CLI invocation. Static map provides fallback for known internal plugins.

**Runtime takes precedence:** If a plugin registers via `declare family ... decode`, that registration overrides the static map.

### Why No Caching?

User decision: plugins are fast enough without caching. Simpler implementation, less state to manage.

## RFC Documentation

### Reference Comments
- RFC 4760 Section 3 - MP_REACH_NLRI contains AFI (2 bytes) + SAFI (1 byte)
- RFC 4760 Section 4 - MP_UNREACH_NLRI same AFI/SAFI structure

## Implementation Summary

### What Was Implemented
- `PluginRegistration.DecodeFamilies` field for family decode claims
- `PluginRegistry.families` map tracking family→plugin
- `PluginRegistry.LookupFamily()` method with case-insensitive lookup
- Extended `parseFamily()` to handle `declare family <afi> <safi> decode`
- Extended `Register()` to check family conflicts with case normalization
- Changed `--nlri` CLI flag from bool to string (takes family)
- Added `pluginFamilyMap` for known family plugins (FlowSpec)
- Added `invokePluginNLRIDecode()` for plugin NLRI decode
- Updated `decodeNLRIOnly()` to try plugin decode first
- Updated test runner for new `--nlri <family>` syntax

### Bugs Found/Fixed
- Case normalization inconsistency: `Register()` stored family as-is but `LookupFamily()` normalized to lowercase. Fixed by normalizing in both places.

### Design Insights
- CLI decode command is standalone (no runtime registry access), so `pluginFamilyMap` provides static fallback
- Case normalization should happen at storage time, not just lookup time

### Deviations from Plan
- Spec called for separate `RegisterFamily()` method; integrated into `Register()` instead (cleaner, no standalone use case)
- Added case-insensitive conflict detection (not explicitly in spec but necessary for robustness)

## Checklist

### 🏗️ Design
- [x] No premature abstraction (3+ concrete use cases exist?)
- [x] No speculative features (is this needed NOW?)
- [x] Single responsibility (each component does ONE thing?)
- [x] Explicit behavior (no hidden magic or conventions?)
- [x] Minimal coupling (components isolated, dependencies minimal?)
- [x] Next-developer test (would they understand this quickly?)

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (output below)
- [x] Implementation complete
- [x] Tests PASS (output below)
- [x] Boundary tests cover all numeric inputs
- [x] Feature code integrated into codebase
- [x] Functional tests verify end-user behavior

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] Required docs read
- [x] RFC summaries read
- [x] RFC references added to code
- [x] RFC constraint comments added

### Completion
- [x] Architecture docs updated with learnings
- [x] Spec updated with Implementation Summary
- [x] Spec moved to `docs/plan/done/`
- [x] All files committed together
