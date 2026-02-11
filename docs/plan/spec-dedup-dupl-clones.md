# Spec: dedup-dupl-clones

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. Source files listed in "Current Behavior" below
4. `.claude/rules/design-principles.md` - no premature abstraction

## Task

Eliminate duplicate code detected by `dupl` (token-based clone detection, threshold 100-150).

**34 clone groups** found at threshold 100, **7 groups** at threshold 150. This spec targets **Tier 1** (high-value, large duplications) and **Tier 2** (medium-value, clearly fixable). Tier 3 items are intentional duplications or too small to justify abstraction.

### What is NOT in scope

- BGP-LS `Bytes()`/`WriteTo()` parallelism (already `//nolint:dupl`, intentional)
- Plugin `register.go` CLIHandler boilerplate (pattern-by-design, per-plugin customization)
- Small fragments near threshold in reactor, parser, peer (structural similarity, not true duplication)
- Research code (`research/cmd/community-defaults/`)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - understand component boundaries for safe extraction
- [ ] `.claude/rules/design-principles.md` - no premature abstraction, three concrete uses rule

### RFC Summaries
N/A — this is a refactoring task, no protocol changes.

**Key insights:**
- Refactoring must preserve ALL existing behavior and test expectations
- Extracted helpers must respect package boundaries (no circular imports)
- Test infrastructure duplication (`test/peer` ↔ `test/runner`) is the largest win

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/peer/decode.go` - BGP message decode helpers for ze-peer test tool
- [ ] `internal/test/runner/decode.go` - identical decode helpers for ze-test runner
- [ ] `internal/config/serialize.go` - config tree serialization with 5 repeated "values not in schema" blocks
- [ ] `internal/plugin/bgpls/plugin.go` - BGP-LS decode dispatch loop
- [ ] `internal/plugin/evpn/plugin.go` - EVPN decode dispatch loop (identical structure)
- [ ] `internal/plugin/vpn/vpn.go` - VPN decode dispatch loop (identical structure)
- [ ] `internal/plugin/bgp/attribute/text.go:135-188` - `ParseBracketedList` (exported)
- [ ] `internal/plugin/route.go:388-441` - `parseBracketedList` (unexported copy)
- [ ] `internal/plugin/decode.go:12-57` - `parseIPv4Prefixes` / `parseIPv6Prefixes`
- [ ] `internal/plugin/wire_extract.go:94-156` - `ExtractAllRawNLRI` / `ExtractAllRawWithdrawn`
- [ ] `pkg/plugin/sdk/sdk.go:550-620` - three handler wrappers (encode/decode NLRI, decode capability)
- [ ] `internal/plugin/refresh.go:22-97` - `handleBoRR` / `handleEoRR`
- [ ] `internal/plugin/route.go:1429-1493` - `handleWatchdogAnnounce` / `handleWatchdogWithdraw`
- [ ] `cmd/ze/bgp/decode.go:273-301` - `parseCapabilities` (CLI)
- [ ] `internal/plugin/bgp/reactor/session.go:1397-1424` - `parseCapabilities` (reactor)
- [ ] `cmd/ze/bgp/plugin_test_cmd.go:188-225` - `extractSubtree`
- [ ] `internal/plugin/server.go:609-646` - `extractConfigSubtree` (identical logic)
- [ ] `internal/plugin/rib_handler.go:117-142` - command lookup (builtin then plugin)
- [ ] `internal/plugin/system.go:226-251` - command lookup (same pattern)
- [ ] `internal/plugin/hub.go:75-102,147-174` - `RouteCommit` / `RouteRollback`
- [ ] `internal/config/loader.go:478-494,572-589` - cluster-list parsing (two routes)
- [ ] `internal/plugin/bgp/reactor/reactor.go:5079-5113` - extended community target/origin
- [ ] `internal/plugin/bgp/attribute/builder_parse.go:201-227` - `parseSingleLargeCommunity`
- [ ] `internal/plugin/update_text.go:726-744` - `parseLargeCommunityText` (same logic)
- [ ] `internal/plugin/bgp/message/update_split.go:180-259` - MP_UNREACH/MP_REACH split blocks
- [ ] `internal/plugin/rpc_plugin.go:114-151,173-207` - RPC handler pairs
- [ ] `internal/config/parser.go:572-596,1004-1028,1187-1211` - block parsing loops (3 sites)
- [ ] `internal/plugin/flowspec/plugin.go:1218-1284` - TCP flags / fragment bitmask parsers
- [ ] `cmd/ze/schema/main.go:258-309` - `cmdMethods` / `cmdEvents`

**Behavior to preserve:**
- All function signatures that are called externally
- All error messages (tests may match on them)
- All JSON output formats
- All command dispatch behavior

**Behavior to change:**
- Duplicate implementations replaced with calls to shared functions
- Some unexported functions become exported (or moved to shared packages)

## Data Flow (MANDATORY)

### Entry Point
- N/A — pure refactoring, no new data entry points

### Transformation Path
- Code is being moved/consolidated, not changing behavior

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| test/peer ↔ test/runner | Extract shared decode package | [ ] |
| attribute ↔ route (plugin) | Delete private copy, use exported function | [ ] |
| bgpls ↔ evpn ↔ vpn plugins | Extract shared decode loop helper | [ ] |
| decode.go CLI ↔ session.go reactor | Extract shared parseCapabilities | [ ] |
| plugin_test_cmd ↔ server | Extract shared extractSubtree | [ ] |

### Integration Points
- `attribute.ParseBracketedList` - already exported, `route.go` will call it directly
- `capability.Parse` - used by both `parseCapabilities` copies, shared version will continue to use it
- `internal/plugin/cli.BaseConfig` / `cli.RunPlugin` - decode loop helper lives alongside existing CLI shared code
- `internal/test/peer.DecodedMessage` / `internal/test/runner.DecodedMessage` - shared decode package will define the type once

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality (that's what we're removing)
- [ ] Zero-copy preserved where applicable

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSharedDecodeOpen` | `internal/test/decode/decode_test.go` | Shared decode functions work identically | |
| `TestParseBracketedListFromRoute` | `internal/plugin/route_test.go` | route.go callers work with exported function | |
| `TestParseCapabilitiesShared` | shared location TBD | Both CLI and reactor get same results | |
| `TestExtractSubtreeShared` | shared location TBD | Both server and test_cmd get same results | |
| `TestSerializeNonSchemaValues` | `internal/config/serialize_test.go` | Extracted helper preserves output | |

### Boundary Tests (MANDATORY for numeric inputs)

N/A — no new numeric inputs; existing boundary tests unchanged.

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing functional tests | `test/` | All pass unchanged (refactoring) | |

### Future
- Additional dedup opportunities may emerge as codebase evolves

## Files to Modify

### Tier 1: High-Value (estimated ~400 lines removed)

- `internal/test/peer/decode.go` - Remove duplicated functions, import shared package
- `internal/test/runner/decode.go` - Remove duplicated functions, import shared package
- `internal/config/serialize.go` - Extract repeated "values not in schema" block to helper
- `internal/plugin/bgpls/plugin.go` - Use shared decode dispatch loop
- `internal/plugin/evpn/plugin.go` - Use shared decode dispatch loop
- `internal/plugin/vpn/vpn.go` - Use shared decode dispatch loop
- `internal/plugin/route.go` - Delete `parseBracketedList`, call `attribute.ParseBracketedList`

### Tier 2: Medium-Value (estimated ~200 lines removed)

- `internal/plugin/decode.go` - Unify `parseIPv4Prefixes` / `parseIPv6Prefixes` via generic
- `internal/plugin/refresh.go` - Parameterize `handleBoRR` / `handleEoRR`
- `internal/plugin/route.go` - Parameterize `handleWatchdogAnnounce` / `handleWatchdogWithdraw`
- `internal/plugin/update_text.go` - Delete `parseLargeCommunityText`, call `parseSingleLargeCommunity`
- `cmd/ze/bgp/decode.go` - Extract `parseCapabilities` to shared location
- `internal/plugin/bgp/reactor/session.go` - Use shared `parseCapabilities`
- `cmd/ze/bgp/plugin_test_cmd.go` - Extract `extractSubtree` to shared location
- `internal/plugin/server.go` - Use shared `extractSubtree`
- `internal/plugin/rib_handler.go` - Extract command lookup helper
- `internal/plugin/system.go` - Use shared command lookup helper
- `internal/plugin/hub.go` - Parameterize `RouteCommit` / `RouteRollback`
- `internal/config/loader.go` - Extract cluster-list parsing helper
- `internal/plugin/bgp/reactor/reactor.go` - Extract ext-community target/origin builder
- `internal/plugin/bgp/message/update_split.go` - Parameterize MP_REACH/MP_UNREACH split
- `internal/plugin/rpc_plugin.go` - Extract common RPC handler pattern
- `internal/config/parser.go` - Extract block parsing loop
- `internal/plugin/flowspec/plugin.go` - Parameterize bitmask component parsers
- `cmd/ze/schema/main.go` - Parameterize `cmdMethods` / `cmdEvents`
- `pkg/plugin/sdk/sdk.go` - Generic handler wrapper or extract pattern

## Files to Create

- `internal/test/decode/decode.go` - Shared BGP message decode helpers (from test/peer + test/runner)
- `internal/test/decode/decode_test.go` - Tests for shared decode functions
- `internal/plugin/cli/decode_loop.go` - Shared decode dispatch loop (for bgpls/evpn/vpn)

## Implementation Steps

Steps are organized into independent groups that can be done in any order. Each group is self-contained.

### Group A: Test Infrastructure (largest win, ~280 lines)

1. Create `internal/test/decode/` package with shared functions from `test/peer/decode.go` and `test/runner/decode.go`
2. Write tests for shared functions
3. Update `test/peer/decode.go` to import and delegate to shared package
4. Update `test/runner/decode.go` to import and delegate to shared package
5. Verify: `go test ./internal/test/...`
   → **Review:** Do both test tools still work? Run functional tests.

### Group B: Config Serialize (5 internal clones → 1 helper)

1. Extract "serialize values not in schema" pattern to a helper function
2. Replace all 5 call sites in `serialize.go`
3. Verify: `go test ./internal/config/...`
   → **Review:** Does config roundtrip still work?

### Group C: Plugin Decode Loop (3 plugins → shared helper)

1. Create `internal/plugin/cli/decode_loop.go` with shared decode dispatch function
2. Update bgpls, evpn, vpn to use shared loop
3. Verify: `go test ./internal/plugin/bgpls/... ./internal/plugin/evpn/... ./internal/plugin/vpn/...`
   → **Review:** Do all decode modes still work?

### Group D: Direct Deletions (exact duplicates)

1. Delete `parseBracketedList` from `route.go`, import `attribute.ParseBracketedList`
2. Delete `parseLargeCommunityText` from `update_text.go`, call `parseSingleLargeCommunity`
3. Verify: `go test ./internal/plugin/...`
   → **Review:** No behavior change?

### Group E: Cross-Package Shared Functions

1. Extract `parseCapabilities` to shared location (e.g., `capability` package)
2. Update `cmd/ze/bgp/decode.go` and `reactor/session.go` to use it
3. Extract `extractSubtree` / `extractConfigSubtree` to shared location
4. Update `plugin_test_cmd.go` and `server.go`
5. Verify: `go test ./cmd/ze/bgp/... ./internal/plugin/...`
   → **Review:** Import cycle check?

### Group F: Parameterize Paired Handlers

1. Parameterize `handleBoRR`/`handleEoRR` (differ only in name + reactor method)
2. Parameterize `handleWatchdogAnnounce`/`handleWatchdogWithdraw`
3. Parameterize `RouteCommit`/`RouteRollback` in `hub.go`
4. Parameterize `cmdMethods`/`cmdEvents` in `schema/main.go`
5. Verify: `go test ./internal/plugin/... ./cmd/ze/schema/...`
   → **Review:** All command dispatch still correct?

### Group G: Internal Pattern Consolidation

1. Unify `parseIPv4Prefixes`/`parseIPv6Prefixes` (generic or parameterized)
2. Extract cluster-list parsing helper in `loader.go`
3. Extract ext-community target/origin builder in `reactor.go`
4. Parameterize MP_REACH/MP_UNREACH split in `update_split.go`
5. Extract command lookup pattern from `rib_handler.go`/`system.go`
6. Parameterize bitmask component parsers in `flowspec/plugin.go`
7. Extract RPC handler pattern in `rpc_plugin.go`
8. Generic handler wrapper in `sdk.go` (or accept 3-way structural similarity)
9. Verify: `make test && make lint`
   → **Review:** Any new import cycles? All lints clean?

### Final Verification

1. `make verify` — full lint + test + functional
2. Review dupl output — confirm clone count significantly reduced
   → **Review:** How many clone groups remain? Are they all intentional?

## RFC Documentation

N/A — refactoring task, no protocol changes.

## Implementation Summary

<!-- Fill this section AFTER implementation, before moving to done -->

### What Was Implemented
- (TBD)

### Bugs Found/Fixed
- (TBD)

### Investigation → Test Rule
- (TBD)

### Design Insights
- (TBD)

### Documentation Updates
- (TBD)

### Deviations from Plan
- (TBD)

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Tier 1: Test decode dedup | | | |
| Tier 1: Serialize helper | | | |
| Tier 1: Decode loop dedup | | | |
| Tier 1: Delete exact dupes | | | |
| Tier 2: Cross-package shared | | | |
| Tier 2: Parameterize handlers | | | |
| Tier 2: Internal consolidation | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestSharedDecodeOpen | | | |
| TestParseBracketedListFromRoute | | | |
| TestParseCapabilitiesShared | | | |
| TestExtractSubtreeShared | | | |
| TestSerializeNonSchemaValues | | | |
| All existing tests pass | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| internal/test/decode/decode.go | | |
| internal/test/decode/decode_test.go | | |
| internal/plugin/cli/decode_loop.go | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

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
- [ ] Boundary tests cover all numeric inputs
- [ ] Feature code integrated into codebase
- [ ] Functional tests verify end-user behavior

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation (during implementation)
- [ ] Required docs read
- [ ] RFC summaries read (all referenced RFCs)
- [ ] RFC references added to code
- [ ] RFC constraint comments added

### Completion (after tests pass)
- [ ] Architecture docs updated with learnings
- [ ] Implementation Audit completed
- [ ] All Partial/Skipped items have user approval
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
