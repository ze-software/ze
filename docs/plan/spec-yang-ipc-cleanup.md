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

<!-- Fill this section AFTER implementation, before moving to done -->

### What Was Implemented
- [To be filled]

### Bugs Found/Fixed
- [To be filled]

### Design Insights
- [To be filled]

### Deviations from Plan
- [To be filled]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Update YANG origin leaf type (string → enum in ze-types.yang) | | | |
| YANG validation for API text command values | | | |
| YANG validation for CLI decode inputs | | | |
| Remove orphaned code (NOT encodeAlphaSerial — it's active) | | | |
| Update documentation | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestUpdateText_OriginValidation_YANG | | | |
| TestUpdateText_MEDRange_YANG | | | |
| TestUpdateText_LocalPrefRange_YANG | | | |
| TestDecodeInput_ValidFamily_YANG | | | |
| TestDecodeInput_ValidHex_YANG | | | |
| TestDecodeOutput_Unchanged | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| internal/yang/modules/ze-types.yang | | |
| internal/plugin/update_text.go | | |
| cmd/ze/bgp/decode.go | | |
| docs/architecture/api/architecture.md | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Checklist

### 🏗️ Design (see `rules/design-principles.md`)
- [ ] No premature abstraction (3+ concrete use cases exist?)
- [ ] No speculative features (is this needed NOW?)
- [ ] Single responsibility (each component does ONE thing?)
- [ ] Explicit behavior (no hidden magic or conventions?)
- [ ] Minimal coupling (components isolated, dependencies minimal?)
- [ ] Next-developer test (would they understand this quickly?)

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs (last valid, first invalid above/below)
- [ ] Feature code integrated into codebase (`internal/*`, `cmd/*`)
- [ ] Functional tests verify end-user behavior (`.ci` files)

### Verification
- [ ] `make lint` passes (26 linters including `govet`, `staticcheck`, `gosec`, `gocritic`)
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation (during implementation)
- [ ] Required docs read
- [ ] RFC summaries read (all referenced RFCs)
- [ ] RFC references added to code
- [ ] RFC constraint comments added (quoted requirement + explanation)

### Completion (after tests pass - see Completion Checklist)
- [ ] Architecture docs updated with learnings
- [ ] Implementation Audit completed (all items have status + location)
- [ ] All Partial/Skipped items have user approval
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
