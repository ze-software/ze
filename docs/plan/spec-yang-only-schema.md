# Spec: YANG-Only Schema - Eradicate All Go-Based Schema Code

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `internal/config/yang_schema.go` - current YANG loading
4. `internal/config/bgp.go` - code to DELETE
5. `internal/yang/loader.go` - module loading
6. `internal/config/schema.go` - schema node types

## Task

Completely remove all Go-based schema definitions. YANG becomes the ONLY source of schema truth. No exceptions, no fallbacks, no "temporary" bridges.

## Problem Statement

Current state is broken:
1. `BGPSchema()` defines schema in Go code
2. `YANGSchema()` loads YANG then OVERWRITES with `BGPSchema()` - pointless
3. `syntaxHints` map is hardcoded Go, not YANG - parallel maintenance
4. Module loading is hardcoded - BGP loads even when not used
5. Every "fix" has preserved Go code instead of fixing YANG

This spec defines complete removal of Go schema code.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config syntax overview
- [ ] `docs/architecture/core-design.md` - system architecture

### Source Files to Understand
- [ ] `internal/config/bgp.go` - Go schema definitions (TO DELETE)
- [ ] `internal/config/yang_schema.go` - YANG-to-schema conversion
- [ ] `internal/config/schema.go` - Node types (Leaf, Container, List, Flex, etc.)
- [ ] `internal/config/parser.go` - how schema is used during parsing
- [ ] `internal/yang/loader.go` - YANG module loading
- [ ] `internal/yang/modules/*.yang` - current YANG models

## Analysis: What Must Be Deleted

### Functions to DELETE from internal/config/bgp.go

| Function | Lines (approx) | Purpose | Replacement |
|----------|----------------|---------|-------------|
| `BGPSchema()` | 311-350 | Main schema builder | YANG only |
| `peerFields()` | 177-307 | Peer field definitions | YANG grouping |
| `templatePeerFields()` | 352-359 | Template fields | YANG grouping |
| `routeAttributes()` | 145-173 | Route attribute fields | YANG grouping |
| `flowRouteAttributes()` | 92-103 | FlowSpec fields | YANG grouping |
| `mcastVpnAttributes()` | 105-123 | MCAST-VPN fields | YANG grouping |
| `vplsAttributes()` | 125-143 | VPLS fields | YANG grouping |
| `environmentBlock()` | 381-449 | Environment schema | ze-hub.yang |
| `PluginOnlySchema()` | 361-379 | Plugin-only schema | YANG subset |

**KEEP:** `LegacyBGPSchema()` - needed for migration tool parsing old ExaBGP syntax

### Code to DELETE from internal/config/yang_schema.go

| Item | Lines | Replacement |
|------|-------|-------------|
| `syntaxHints` map | 22-130 | YANG extensions |
| `SyntaxHint` type | 10-18 | YANG extensions |
| All `Syntax*` constants | 12-18 | YANG extensions |

### Constants to KEEP in internal/config/bgp.go

These are runtime values, not schema:
- `configTrue`, `configFalse`, `configEnable`, `configDisable`, `configRequire`
- `addPathSend`, `addPathReceive`, etc.
- `FamilyMode` type and methods
- `FamilyConfig` struct
- `ParseFamilyMode()` function

## Design: YANG Extensions for Special Syntax

### Extension Module Structure

Create file `internal/yang/modules/ze-extensions.yang` with these extensions:

| Extension | Argument | Purpose |
|-----------|----------|---------|
| `syntax` | mode string | Parsing mode: flex, freeform, inline-list, family-block, multi-leaf, array |
| `key-type` | type string | For inline-list: string, prefix, ip, uint32 |
| `route-attributes` | (none) | Node accepts standard route attributes |

### Syntax Mode Values

| Mode | Behavior |
|------|----------|
| `flex` | Accepts flag (;), value (X;), or block ({ }) |
| `freeform` | Accepts arbitrary word sequences until ; or { } |
| `inline-list` | List with key + attributes on same line |
| `family-block` | Special family syntax (ipv4/unicast;) |
| `multi-leaf` | Multiple space-separated values on one line |
| `array` | Bracketed array syntax [ a b c ] |

### Extension Application in YANG

Each special-syntax node in YANG gets the appropriate extension. Examples:

| YANG Path | Extension to Add |
|-----------|------------------|
| `bgp/peer/capability/route-refresh` | `ze:syntax "flex"` |
| `bgp/peer/capability/graceful-restart` | `ze:syntax "flex"` |
| `bgp/peer/family` | `ze:syntax "family-block"` |
| `bgp/peer/static/route` | `ze:syntax "inline-list"` and `ze:key-type "prefix"` and `ze:route-attributes` |
| `bgp/peer/api/receive` | `ze:syntax "freeform"` |
| `bgp/peer/api/send` | `ze:syntax "freeform"` |
| `bgp/listen` | `ze:syntax "multi-leaf"` |
| `bgp/peer/process/processes` | `ze:syntax "array"` |

### Extension Processing Logic

The `yangToNode()` function reads extensions from the YANG entry:

| Step | Action |
|------|--------|
| 1 | Iterate over entry.Exts looking for "ze:syntax" |
| 2 | If found, extract the argument (mode string) |
| 3 | Switch on mode to create appropriate Node type |
| 4 | If "ze:route-attributes" present, add route attribute fields |
| 5 | If no extension, use standard YANG-to-Node conversion |

## Design: Dynamic Module Loading

### Current Problem

Module list is hardcoded in loader.go - BGP always loads even when not configured.

### Solution Architecture

| Component | Responsibility |
|-----------|---------------|
| Core modules | Always loaded: ze-extensions, ze-types, ze-hub |
| Plugin registry | Plugins register their YANG content at init |
| Config loader | Two-pass: first find plugins, then load their YANG |

### Loading Sequence

| Step | Action |
|------|--------|
| 1 | Loader loads core modules (extensions, types, hub) |
| 2 | First config pass with minimal schema finds plugin block |
| 3 | For each configured plugin, load its registered YANG |
| 4 | Resolve all YANG imports/dependencies |
| 5 | Build complete schema from all loaded YANG |
| 6 | Second config pass with full schema |

### Plugin YANG Registration

| Plugin | YANG Module | Registration Location |
|--------|-------------|----------------------|
| BGP | ze-bgp.yang | `internal/plugin/bgp/init.go` |
| Future plugins | ze-xxx.yang | `internal/plugin/xxx/init.go` |

## Design: Route Attributes Grouping

### Grouping Definition

Create grouping `route-attributes` in ze-types.yang containing:

| Field | Type | Description |
|-------|------|-------------|
| next-hop | ip-address or "self" | Next hop address |
| origin | enum (igp/egp/incomplete) | Origin attribute |
| local-preference | uint32 | LOCAL_PREF |
| med | uint32 | MED |
| community | list of community | Standard communities |
| extended-community | list of ext-community | Extended communities |
| large-community | list of large-community | Large communities |
| as-path | list of string | AS path segments |
| label | string | MPLS label |
| labels | list of string | MPLS label stack |
| rd | route-distinguisher | Route distinguisher |
| aggregator | string | Aggregator attribute |
| atomic-aggregate | empty | Atomic aggregate flag |
| originator-id | ipv4-address | Originator ID |
| cluster-list | string | Cluster list |
| path-information | string | Path ID |
| name | string | Route name |
| watchdog | string | Watchdog reference |
| withdraw | empty | Withdraw flag |

### Grouping Usage

Any list that needs route attributes uses `uses route-attributes` in YANG.

## Files to Create

| File | Purpose |
|------|---------|
| `internal/yang/modules/ze-extensions.yang` | Custom extensions for syntax modes |
| `internal/plugin/registry.go` | Plugin YANG registration |

## Files to Modify

| File | Changes |
|------|---------|
| `internal/config/bgp.go` | DELETE most functions, keep LegacyBGPSchema and constants |
| `internal/config/yang_schema.go` | DELETE syntaxHints, add extension processing |
| `internal/yang/loader.go` | Split LoadCore/LoadPlugin, remove hardcoded list |
| `internal/yang/modules/ze-types.yang` | Add route-attributes grouping |
| `internal/yang/modules/ze-hub.yang` | Complete environment block |
| `internal/yang/modules/ze-bgp.yang` | Complete with extensions, all fields |

## Implementation Steps

### Step 1: Create YANG Extensions Module

Create `ze-extensions.yang` with syntax, key-type, route-attributes extensions.

**Verify:** YANG loader accepts the new module without errors.

### Step 2: Update ze-types.yang

Add `route-attributes` grouping with all common fields listed in the table above.

**Verify:** YANG loader accepts the updated module.

### Step 3: Expand ze-bgp.yang with Extensions

Add `ze:syntax` extensions to all special syntax nodes per the Extension Application table.

**Verify:** YANG loader accepts the updated module.

### Step 4: Update yang_schema.go Extension Processing

- DELETE `syntaxHints` map entirely
- DELETE `SyntaxHint` type and all constants
- ADD function to read `ze:syntax` extension from YANG entry
- ADD function to check for `ze:route-attributes` extension
- UPDATE `yangToNode()` to use extensions instead of hardcoded map

**Verify:** Code compiles successfully.

### Step 5: DELETE Go Schema Functions

From `internal/config/bgp.go`, DELETE all functions listed in the deletion table.

**Verify:** Code compiles (expect errors from callers).

### Step 6: Fix Compilation Errors

Update all code that called deleted functions to use YANG-based alternatives.

**Verify:** `go build ./...` succeeds.

### Step 7: Run Tests, Fix YANG

For each test failure:
- "unknown field X" → Add field X to YANG
- "expected '{' after X" → Add `ze:syntax "flex"` to X in YANG
- "expected value for X" → Check type definition in YANG

**Verify:** `make test` passes.

### Step 8: Implement Dynamic Module Loading

Create plugin registry, move ze-bgp.yang to plugin, update loader.

**Verify:** `make test && make functional` passes.

### Step 9: Final Cleanup

Remove dead code, update comments, run full suite.

**Verify:** `make test && make lint && make functional` all pass.

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestYANGExtensionParsing` | `yang_schema_test.go` | Extensions read from YANG entries | |
| `TestYANGSchemaHasAllPeerFields` | `yang_schema_test.go` | All peer fields exist in schema | |
| `TestYANGSchemaHasAllCapabilities` | `yang_schema_test.go` | All capability fields exist | |
| `TestNoSyntaxHintsMap` | `yang_schema_test.go` | syntaxHints map does not exist | |

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| All existing | `internal/config/*_test.go` | Existing tests pass with YANG-only | |
| All functional | `make functional` | Functional tests pass | |

## Mandatory Verification: Go Code Removal

**BLOCKING:** Before claiming completion, MUST verify these deletions by searching codebase:

### Functions That MUST NOT Exist After Implementation

| Function | Search Command | Expected Result |
|----------|----------------|-----------------|
| `BGPSchema()` | `grep -r "func BGPSchema" internal/` | No matches (except LegacyBGPSchema) |
| `peerFields()` | `grep -r "func peerFields" internal/` | No matches |
| `templatePeerFields()` | `grep -r "func templatePeerFields" internal/` | No matches |
| `routeAttributes()` | `grep -r "func routeAttributes" internal/` | No matches |
| `flowRouteAttributes()` | `grep -r "func flowRouteAttributes" internal/` | No matches |
| `mcastVpnAttributes()` | `grep -r "func mcastVpnAttributes" internal/` | No matches |
| `vplsAttributes()` | `grep -r "func vplsAttributes" internal/` | No matches |
| `environmentBlock()` | `grep -r "func environmentBlock" internal/` | No matches |
| `PluginOnlySchema()` | `grep -r "func PluginOnlySchema" internal/` | No matches |
| `syntaxHints` | `grep -r "syntaxHints" internal/` | No matches |
| `SyntaxHint` | `grep -r "type SyntaxHint" internal/` | No matches |

### Callers That MUST NOT Exist After Implementation

| Call Pattern | Search Command | Expected Result |
|--------------|----------------|-----------------|
| `BGPSchema()` calls | `grep -r "BGPSchema()" internal/ cmd/` | No matches |
| `PluginOnlySchema()` calls | `grep -r "PluginOnlySchema()" internal/ cmd/` | No matches |
| `syntaxHints[` | `grep -r "syntaxHints\[" internal/` | No matches |

### Verification Script

Run this script and paste output in Implementation Summary:

    grep -rn "func BGPSchema\|func peerFields\|func routeAttributes\|func environmentBlock\|syntaxHints\|SyntaxHint" internal/ cmd/ | grep -v "_test.go" | grep -v "LegacyBGPSchema"

**Expected output:** No matches (empty output).

If any matches found, the implementation is INCOMPLETE.

## Checklist

### 🏗️ Design
- [ ] No Go-based schema definitions (except LegacyBGPSchema for migration)
- [ ] All syntax modes defined via YANG extensions
- [ ] Dynamic module loading based on configured plugins
- [ ] Single source of truth (YANG only)

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)

### 🧪 Implementation
- [ ] ze-extensions.yang created
- [ ] ze-types.yang has route-attributes grouping
- [ ] ze-bgp.yang complete with extensions
- [ ] syntaxHints map deleted
- [ ] BGPSchema() deleted
- [ ] All helper functions deleted
- [ ] Extension processing in yang_schema.go

### Go Code Removal Verification
- [ ] `BGPSchema()` function deleted
- [ ] `peerFields()` function deleted
- [ ] `routeAttributes()` function deleted
- [ ] `environmentBlock()` function deleted
- [ ] `PluginOnlySchema()` function deleted
- [ ] All other helper functions deleted
- [ ] `syntaxHints` map deleted
- [ ] `SyntaxHint` type deleted
- [ ] Verification script run (output pasted below)

### Verification
- [ ] `go build ./...` succeeds
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] Extension processing documented in yang_schema.go

## Implementation Summary

<!-- Fill after implementation -->

### What Was Implemented

### Bugs Found/Fixed

### Design Insights

### Deviations from Plan
