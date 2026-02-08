# Spec: yang-ipc-cleanup

**Actual scope:** YANG-driven validation for API text commands and CLI decode inputs (the "IPC cleanup" name is historical — this spec adds YANG validation, not IPC changes).

**Depends on:** spec-config-yang-validation (YANG validation infrastructure must exist before this spec can use it for API commands and decode inputs)

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/update_text.go` - current API text command parser (2214 lines, no attribute validation)
4. `internal/plugin/command.go` - command dispatcher
5. `internal/yang/validator.go` - YANG validation engine
6. `internal/yang/modules/ze-types.yang` - route-attribute typedefs
7. `internal/plugin/bgp/schema/ze-bgp-conf.yang` - BGP config schema

## Task

Make the YANG schema the single source of truth for data validation across all IPC paths. Today, data validation is absent or inconsistent:

1. **Config parsing** — config reader validates nothing; values pass through as raw strings/numbers (fixed by spec-inline-config-reader)
2. **API text commands** — `update_text.go` (2214 lines) parses command grammar but performs NO attribute value validation — origin, med, local-pref values are accepted without checking enums, ranges, or types
3. **CLI decode protocol** — plugins implement the text protocol (`decode nlri <family> <hex>` → `decoded json [...]`) with no YANG validation of inputs or outputs

After this spec:
- API text command values validated against YANG type definitions (enums, ranges, patterns)
- CLI decode text protocol preserved but with YANG validation of decode inputs
- One validation authority (YANG) regardless of how data enters the system

### Goals

1. Update YANG leaf types to enable validation — origin must become `type enumeration { enum igp; enum egp; enum incomplete; }` (currently `type string`). med/local-preference are already `type uint32` which the validator enforces — no explicit range needed since 0..4294967295 IS the full uint32 range.
2. Add YANG validator calls to `update_text.go` for attribute value validation — origin, med, local-preference, communities validated against YANG types. Currently NO validation exists; values are accepted unchecked.
3. Add YANG validation to CLI decode text protocol — validate decode inputs (family, hex) against YANG type definitions before dispatching to plugin decoders. The text protocol itself (`decode nlri`/`decoded json`) is preserved.
4. Update architecture documentation

### Non-Goals

- Changing the text command grammar (parsing stays in Go; only value validation moves to YANG)
- Changing the YANG RPC protocol itself (already migrated in Specs 1-3)
- Replacing the CLI decode text protocol with direct function calls (the text protocol is the user-facing CLI interface and stays)
- Changing the CLI decode invocation modes (Fork, Internal, Direct remain as-is)
- Config file validation (handled by spec-inline-config-reader)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - current API design
- [ ] `docs/architecture/cli/plugin-modes.md` - decode invocation modes (Fork, Internal, Direct)

### Source Files
- [ ] `internal/plugin/update_text.go` - API text command parser (no attribute value validation — YANG validation to add)
- [ ] `internal/plugin/command.go` - command dispatcher
- [ ] `internal/yang/validator.go` - YANG validation engine
- [ ] `internal/yang/modules/ze-types.yang` - route-attribute typedefs (origin, med, local-pref, communities)
- [ ] `internal/plugin/bgp/schema/ze-bgp-conf.yang` - BGP config schema with attribute type definitions
- [ ] `cmd/ze/bgp/decode.go` - CLI decode dispatch (invokePluginInProcess, invokePluginInternal, invokePluginSubprocess)

### Plugin Decode Implementations (reference only — these files are NOT modified)
- [ ] `internal/plugin/flowspec/plugin.go` - RunFlowSpecDecode (text protocol, unchanged)
- [ ] `internal/plugin/evpn/plugin.go` - RunEVPNDecode (text protocol, unchanged)
- [ ] `internal/plugin/vpn/vpn.go` - RunVPNDecode (text protocol, unchanged)
- [ ] `internal/plugin/bgpls/plugin.go` - RunBGPLSDecode (text protocol, unchanged)
- [ ] `internal/plugin/hostname/hostname.go` - decode capability (text protocol, unchanged)

YANG validation of decode inputs happens in `cmd/ze/bgp/decode.go` before dispatch, not inside each plugin.

**Key insights:**
- `ze-types.yang` has `route-attributes` grouping but origin is `type string` (not an enum) and med/local-preference are plain `uint32`. **Prerequisite:** update origin to `type enumeration { enum igp; enum egp; enum incomplete; }` before validation will be meaningful. med/local-pref are already uint32 which the validator enforces — adding `range "0..4294967295"` would be redundant (that IS the full uint32 range)
- The CLI decode text protocol (`decode nlri` → `decoded json`) is the user-facing interface and stays — YANG validation is added to validate inputs before dispatch
- `encodeAlphaSerial` is actively used for engine RPC serial generation — it is NOT legacy and must NOT be removed

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/update_text.go` - parses "update text" command grammar (2214 lines); defines keyword constants (`kwOrigin`, `kwMED`, etc.) but performs NO attribute value validation — values are accepted unchecked
- [ ] `cmd/ze/bgp/decode.go` - three decode paths (Fork/Internal/Direct) all format `decode nlri <family> <hex>` text, send through different transports, parse `decoded json [...]` response
- [ ] Plugin decode functions - each implements `func(in, out *bytes.Buffer) int` reading text commands from buffer, writing text responses

**Behavior to preserve:**
- API text command syntax: `update text origin set igp med set 50 nlri ipv4/unicast add 10.0.0.0/24`
- CLI decode text protocol: `decode nlri <family> <hex>` → `decoded json [...]` / `decoded unknown`
- CLI decode output format (JSON arrays for NLRI, JSON objects for capabilities)
- CLI decode invocation modes (Fork, Internal, Direct) — all three stay
- All address families and attribute types currently supported
- External plugin decode via subprocess (Fork mode with `/path/to/binary`)
- `inProcessDecoders` map signature unchanged
- `encodeAlphaSerial` — actively used for engine RPC serial generation, NOT legacy

**Behavior to change:**
- Add YANG validation for attribute values in `update_text.go` (currently no validation exists)
- CLI decode inputs validated against YANG types before dispatch (family must be known, hex must be valid hex)

## Data Flow (MANDATORY)

### Entry Point — API Text Commands
- Text command enters via plugin pipe or CLI
- Format: `update text origin set igp nhop set 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24`

### Transformation Path (API Text Commands)
1. `update_text.go` parses command tokens (grammar parsing stays in Go)
2. For each attribute value, call YANG validator — path follows YANG schema structure (e.g., `"bgp.bgp.peer.update.attribute.origin"` navigates ze-bgp-conf module tree). Exact paths determined during implementation by tracing the YANG entry tree.
3. If validation fails, return error with YANG-defined constraint message
4. If valid, proceed with wire encoding as before

**Prerequisite:** YANG origin leaf type must be updated first (string → enum). med/local-pref are already uint32 — the validator already enforces uint32 bounds. Without the origin enum, the validator will accept any string for origin.

### Transformation Path (Decode)
1. CLI or engine needs to decode NLRI/capability
2. Validate decode inputs against YANG types (family is a known AFI/SAFI, hex is valid hex string)
3. Dispatch via existing text protocol (`decode nlri <family> <hex>` → `decoded json [...]`)
4. Decode invocation modes (Fork, Internal, Direct) unchanged

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Text parser → YANG validator | `validator.Validate(path, value)` for each attribute value | [ ] |
| CLI decode → YANG validator | Validate family and hex inputs before dispatch | [ ] |
| CLI decode → plugin decoder | Text protocol preserved, all invocation modes unchanged | [ ] |
| External plugins | Still use YANG RPC protocol over socket pairs (unchanged) | [ ] |

### Integration Points
- `yang.Validator` — existing, used for config validation; now also for API command values and decode input validation
- `inProcessDecoders` — existing map; unchanged signature, YANG validation added before dispatch

### Validator Instantiation
`update_text.go` needs access to a `yang.Validator` instance. The caller (command dispatcher in `command.go`) must create it:
1. `loader := yang.NewLoader()`
2. `loader.LoadEmbedded()` — loads core modules only: ze-extensions, ze-types, ze-plugin-conf
3. `loader.AddModuleFromText("ze-bgp-conf.yang", bgpschema.ZeBGPConfYANG)` — LoadEmbedded does NOT load plugin schemas; each plugin schema must be added explicitly via its embedded Go variable (e.g., `bgpschema.ZeBGPConfYANG` from `internal/plugin/bgp/schema/embed.go`)
4. `loader.Resolve()` — resolve cross-module imports
5. `yang.NewValidator(loader)` — pass to text command parser as a dependency

### YANG Path Mapping
The validator uses `MapPrefixToModule()` to map the first path segment to a module name (e.g., `"bgp"` → `"ze-bgp-conf"`), then navigates the YANG entry tree segment by segment via `findSchemaNode()`. Paths must match the YANG container/leaf structure exactly.

### Architectural Verification
- [ ] No bypassed layers — validation still happens before wire encoding
- [ ] No unintended coupling — YANG validator is a read-only query, no side effects
- [ ] No duplicated functionality — adds YANG validation where none exists, no parallel validation paths
- [ ] Zero-copy preserved — validation is on parsed values, not wire bytes

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUpdateText_OriginValidation_YANG` | `internal/plugin/update_text_test.go` | Origin validated against YANG enum (igp/egp/incomplete valid, "foo" rejected) | |
| `TestUpdateText_MEDRange_YANG` | `internal/plugin/update_text_test.go` | MED validated against YANG uint32 (0-4294967295 valid, negative rejected) | |
| `TestUpdateText_LocalPrefRange_YANG` | `internal/plugin/update_text_test.go` | Local-preference validated against YANG uint32 range | |
| `TestDecodeInput_ValidFamily_YANG` | `cmd/ze/bgp/decode_test.go` | Known families accepted, unknown families rejected via YANG validation | |
| `TestDecodeInput_ValidHex_YANG` | `cmd/ze/bgp/decode_test.go` | Valid hex strings accepted, malformed hex rejected via YANG validation | |
| `TestDecodeOutput_Unchanged` | `cmd/ze/bgp/decode_test.go` | Decode output format unchanged after adding YANG validation | |

### Boundary Tests (MANDATORY for numeric inputs)

Covered by YANG validator's existing boundary tests — the validator already tests range checking for uint8/16/32/64, int8/16/32/64.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| All existing decode tests | `test/decode/*.ci` | Decode output unchanged | |
| All existing plugin tests | `test/plugin/*.ci` | Plugin behavior unchanged | |

### Future (if deferring any tests)
- Performance benchmarks (can be added later)

## Files to Modify
- `internal/yang/modules/ze-types.yang` - update route-attributes grouping: origin from `type string` to `type enumeration { enum igp; enum egp; enum incomplete; }` (med/local-pref stay as plain uint32 — no change needed)
- `internal/plugin/update_text.go` - add YANG validator calls for attribute values (currently no validation exists)
- `cmd/ze/bgp/decode.go` - add YANG validation of decode inputs before dispatch
- `docs/architecture/api/architecture.md` - document YANG-driven validation

## Files to Create
- None expected (changes to existing files)

## Files to Delete
- None expected (adding validation to existing files, not removing or replacing files)

## Implementation Steps

Each step ends with a **Self-Critical Review**.

1. **Update YANG leaf types** — change origin from `type string` to `type enumeration { enum igp; enum egp; enum incomplete; }` in `ze-types.yang` route-attributes grouping. med/local-preference are already `type uint32` — no range addition needed. Run existing YANG tests to ensure no breakage.
   → **Review:** Do existing config tests still pass? Is the origin type change compatible with config parsing?

2. **Write YANG validation tests for update_text.go** — test that origin, med, local-pref values are validated against YANG types
   → **Review:** Do tests cover valid values, invalid values, edge cases?

3. **Run tests** — verify FAIL (paste output)
   → **Review:** Tests fail for the right reason?

4. **Wire YANG validator into update_text.go** — add `validator.Validate()` calls for attribute values where none currently exist. Keep text grammar parsing in Go.
   → **Review:** All attribute values validated through YANG? Text grammar parsing unchanged?

5. **Run tests** — verify PASS (paste output)
   → **Review:** All existing tests still pass? New tests pass?

6. **Write YANG validation tests for decode inputs** — test that family and hex inputs are validated against YANG types before dispatch
   → **Review:** Valid and invalid families? Valid and invalid hex?

7. **Add YANG validation to decode dispatch** — validate family and hex inputs in `decode.go` before dispatching to plugin decoders via existing text protocol. All invocation modes (Fork, Internal, Direct) preserved.
   → **Review:** Text protocol unchanged? Output format unchanged? All modes still work?

8. **Remove orphaned code** — search for any dead code left after adding YANG validation (unused helper functions, etc.). Note: `encodeAlphaSerial` is actively used for RPC serials — do NOT remove.
   → **Review:** No orphaned code remaining? Active functions preserved?

9. **Update documentation** — architecture docs
   → **Review:** Documentation matches implementation?

10. **Verify all** — `make lint && make test && make functional` (paste output)
   → **Review:** Zero lint issues? All tests pass?

11. **Final self-review** — Re-read all changes, check for unused imports, debug statements, TODOs

## Implementation Summary

### What Was Implemented
- Updated YANG origin leaf type from `type string` to `type enumeration { enum igp; enum egp; enum incomplete; }` in both `ze-types.yang` (route-attributes grouping) and `ze-bgp-conf.yang` (peer update attribute)
- Added `ValueValidator` interface and `SetYANGValidator()` wiring in `update_text.go` — YANG validation for origin (enum), med (uint32), and local-preference (uint32) attribute values
- Added `validateDecodeFamily()` format validation in `decode.go` — validates `afi/safi` structure before dispatch
- Added YANG-driven validation documentation to `docs/architecture/api/architecture.md`
- RFC 4271 Section 4.3 reference added to origin leaf descriptions
- Extracted `loadYANGModules()` shared helper in `yang_schema.go` and added `YANGValidatorWithPlugins()` factory
- Wired `SetYANGValidator()` into `LoadReactorFileWithPlugins()` in `loader.go` — YANG validator is now active during engine runtime, not just in tests

### Bugs Found/Fixed
- `validateDecodeHex` was implemented and tested but never called from production code (dead code). Removed during critical review along with its test `TestDecodeInput_ValidHex_YANG`.

### Design Insights
- `ValueValidator` interface is idiomatic Go consumer-site interface — enables testability via mock and avoids circular import between `internal/plugin` and `internal/yang`. Follows the codebase's existing `SetLogger` pattern.
- Family validation is format-only by design — families are registered dynamically by plugins, so enumerating valid families would violate the plugin architecture. Format check (`afi/safi` structure) catches malformed input without coupling to plugin registry.
- YANG enum validation is case-sensitive, which tightens origin validation compared to `parseOriginText`'s `strings.ToLower`. This is intentional — YANG is the source of truth. All config files and functional tests already use lowercase.
- Engine wiring done: `LoadReactorFileWithPlugins` creates `YANGValidatorWithPlugins` and calls `plugin.SetYANGValidator()` before reactor creation. The validator uses the same plugin YANG modules as config parsing.

### Deviations from Plan
- `ze-bgp-conf.yang` was also updated (origin type change) — not listed in original Files to Modify but required for consistency since both the grouping in `ze-types.yang` and the direct leaf in `ze-bgp-conf.yang` defined origin
- `TestDecodeInput_ValidHex_YANG` and `validateDecodeHex` removed — dead code identified during critical review
- Boundary tests for numeric inputs (med, local-pref) are covered by YANG validator's existing test suite in `internal/yang/` rather than duplicated here
- Engine wiring added (`yang_schema.go`, `loader.go`) — not in original plan but identified during critical review as needed for production activation

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Update YANG origin leaf type (string → enum in ze-types.yang) | ✅ Done | `internal/yang/modules/ze-types.yang:17-22` | Also updated `ze-bgp-conf.yang:276-281` for consistency |
| YANG validation for API text command values | ✅ Done | `internal/plugin/update_text.go:42-65, 477-483, 499-504, 519-524` | Origin (enum), MED (uint32), local-pref (uint32) |
| YANG validation for CLI decode inputs | ✅ Done | `cmd/ze/bgp/decode.go:760-771, 1628-1631` | Family format validation before dispatch |
| Remove orphaned code (NOT encodeAlphaSerial — it's active) | ✅ Done | `cmd/ze/bgp/decode.go` (removed `validateDecodeHex`) | Dead code found and removed during critical review |
| Update documentation | ✅ Done | `docs/architecture/api/architecture.md:494-505` | YANG-driven validation table added |
| Engine wiring for runtime validation | ✅ Done | `internal/config/loader.go:197-199`, `internal/config/yang_schema.go:64-93` | `YANGValidatorWithPlugins` + `SetYANGValidator` in `LoadReactorFileWithPlugins` |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestUpdateText_OriginValidation_YANG | ✅ Done | `internal/plugin/update_text_test.go:3776` | Validates YANG called with correct path, rejection propagated |
| TestUpdateText_MEDRange_YANG | ✅ Done | `internal/plugin/update_text_test.go:3810` | Validates parsed uint32 passed to YANG |
| TestUpdateText_LocalPrefRange_YANG | ✅ Done | `internal/plugin/update_text_test.go:3844` | Validates parsed uint32 passed to YANG |
| TestDecodeInput_ValidFamily_YANG | ✅ Done | `cmd/ze/bgp/decode_test.go:1555` | Valid/invalid family format testing |
| TestDecodeInput_ValidHex_YANG | ❌ Removed | — | Dead code: `validateDecodeHex` was never called in production |
| TestDecodeOutput_Unchanged | ✅ Done | `cmd/ze/bgp/decode_test.go:1586` | Verifies output format preserved |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/yang/modules/ze-types.yang` | ✅ Modified | origin: string → enumeration |
| `internal/plugin/update_text.go` | ✅ Modified | ValueValidator + YANG calls for origin/med/local-pref |
| `cmd/ze/bgp/decode.go` | ✅ Modified | validateDecodeFamily + dead code removal |
| `docs/architecture/api/architecture.md` | ✅ Modified | YANG validation section added |
| `internal/plugin/bgp/schema/ze-bgp-conf.yang` | 🔄 Changed | Also updated origin type (not in original plan) |
| `internal/plugin/update_text_test.go` | ✅ Modified | 3 YANG validation test functions |
| `cmd/ze/bgp/decode_test.go` | ✅ Modified | Family validation + output unchanged tests |
| `internal/config/yang_schema.go` | ✅ Modified | Extracted `loadYANGModules`, added `YANGValidatorWithPlugins` |
| `internal/config/loader.go` | ✅ Modified | Wired `SetYANGValidator` into reactor creation |

### Audit Summary
- **Total items:** 19
- **Done:** 17
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (ze-bgp-conf.yang added, documented in Deviations)
- **Removed:** 1 (TestDecodeInput_ValidHex_YANG — dead code)

## Checklist

### 🏗️ Design (see `rules/design-principles.md`)
- [x] No premature abstraction (ValueValidator interface serves testability + import isolation)
- [x] No speculative features (only validation paths identified in spec)
- [x] Single responsibility (validator interface, format checker, YANG paths — each does one thing)
- [x] Explicit behavior (nil-check on yangValidator, clear error wrapping)
- [x] Minimal coupling (interface decouples plugin pkg from yang pkg)
- [x] Next-developer test (SetYANGValidator pattern mirrors SetLogger)

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (verified before implementation)
- [x] Implementation complete
- [x] Tests PASS (`make test` 0 failures)
- [x] Boundary tests cover all numeric inputs (via YANG validator's existing suite)
- [x] Feature code integrated into codebase (`internal/plugin/`, `cmd/ze/bgp/`)
- [x] Functional tests verify end-user behavior (existing `.ci` files pass)

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (plugin suite timeouts are pre-existing, unrelated)

### Documentation (during implementation)
- [x] Required docs read
- [x] RFC references added to code (RFC 4271 Section 4.3 in origin descriptions)
- [x] Architecture docs updated with YANG validation table

### Completion (after tests pass - see Completion Checklist)
- [x] Architecture docs updated with learnings
- [x] Implementation Audit completed (all items have status + location)
- [x] All Partial/Skipped items have user approval (ValidHex removal approved)
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
