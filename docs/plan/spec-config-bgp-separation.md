# Spec: Config BGP Separation

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/config/syntax.md` - config parsing pipeline
4. `internal/component/config/loader.go` - main config loader (orchestrates BGP extraction)
5. `internal/component/bgp/` - existing BGP component directory

## Task

Separate BGP-specific code from `internal/component/config/` into `internal/component/bgp/config/`.

The generic config package (`internal/component/config/`) currently contains ~15 BGP-specific files that import deep BGP subsystem types (`reactor`, `capability`, `nlri`, `message`). These should live under the BGP component, leaving the config package content-agnostic.

Also: remove dead environment fields (Phase 5 deferred from `docs/learned/334-yang-reorganisation.md`).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config parsing pipeline
  → Constraint: File → Tree → ResolveBGPTree() → map[string]any → PeersFromTree()
- [ ] `docs/architecture/core-design.md` - BGP subsystem boundaries
  → Constraint: config is generic, BGP subsystem owns BGP-specific logic

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/bgp.go` - BGP route config types
- [ ] `internal/component/config/bgp_routes.go` - route extraction from config trees
- [ ] `internal/component/config/peers.go` - peer settings extraction
- [ ] `internal/component/config/routeattr.go` - route attribute types
- [ ] `internal/component/config/routeattr_community.go` - community parsing
- [ ] `internal/component/config/routeattr_prefixsid.go` - prefix-SID parsing
- [ ] `internal/component/config/loader.go` - main config loader
- [ ] `internal/component/config/loader_routes.go` - BGP route type converters
- [ ] `internal/component/config/plugins.go` - plugin config extraction
- [ ] `internal/component/config/environment.go` - environment variable parsing
- [ ] `internal/component/bgp/` - existing BGP component directory

### BGP-Specific Files (move to `internal/component/bgp/config/`)

| File | Lines | Imports BGP? | Purpose |
|------|-------|-------------|---------|
| `bgp.go` | ~140 | No | BGP route config types: FamilyMode, StaticRouteConfig, MVPNRouteConfig, VPLSRouteConfig, FlowSpecRouteConfig, MUPRouteConfig |
| `bgp_util.go` | ~130 | No | IP glob matching, peer glob patterns, environment extraction helpers |
| `bgp_routes.go` | ~900 | Yes (message) | Route extraction from config trees, NLRI add/del/eor operations |
| `peers.go` | ~300 | Yes (reactor, capability, bgptypes) | Peer settings extraction from config |
| `routeattr.go` | ~400 | Yes (parse) | Route attribute types: Origin, PathID, community parsing |
| `routeattr_community.go` | ~500 | No | Community and extended-community attribute parsing |
| `routeattr_prefixsid.go` | ~300 | No | BGP Prefix-SID attribute parsing |
| `loader_routes.go` | ~350 | Yes (registry, reactor) | BGP route type converters: MVPN, VPLS, FlowSpec, MUP |
| `loader_prefix.go` | ~80 | No | Prefix splitting logic for route expansion |
| `plugins.go` | ~130 | Yes (plugin, reactor) | Plugin config extraction, reactor.PluginConfig |
| `validators.go` | ~80 | No | Validator registration (BGP-specific validators) |
| `validators_register.go` | ~20 | No | Validator init() |

### Test Files (move with source)

| File | Purpose |
|------|---------|
| `bgp_test.go` | BGP parsing tests (imports reactor, capability) |
| `bgp_util_test.go` | IP matching and environment extraction tests |
| `bgp_routes_test.go` | Route extraction and NLRI tests (imports reactor, message) |
| `routeattr_test.go` | Route attribute tests |
| `loader_test.go` | Integration tests (imports reactor, capability, nlri) |
| `plugins_test.go` | Plugin config tests |
| `validators_test.go` | Validator tests |
| `extended_test.go` | Extended config tests (imports reactor) |
| `serialize_test.go` | Serialization tests (imports reactor) |

### Generic Config Files (stay in `internal/component/config/`)

| File | Purpose | Stays because |
|------|---------|---------------|
| `loader.go` | Main config file loader | Orchestrator; will import bgp/config for BGP calls |
| `environment.go` | Environment variable parsing | Generic (all subsystems) |
| `provider.go` | Config provider interface | Generic |
| `reader.go` | Block state management | Generic parser infrastructure |
| `resolve.go` | Template resolution | Generic |
| `tree.go` | Tree data structure | Generic |
| `schema.go` | Schema definition | Generic |
| `serialize.go` | Config serialization | Generic |
| `parser.go` | Main parser dispatcher | Generic |
| `parser_list.go` | List parsing | Generic |
| `parser_freeform.go` | Freeform parsing | Generic |
| `setparser.go` | Set-based parser | Generic |
| `tokenizer.go` | Lexical tokenizer | Generic |
| `diff.go` | Config diffing | Generic |
| `probe.go` | File probing | Generic |
| `yang_schema.go` | YANG module handling | Generic |

### Subdirectories (stay in `internal/component/config/`)

| Directory | Purpose | Stays because |
|-----------|---------|---------------|
| `editor/` | TUI config editor | Generic — couples through config, not BGP directly |
| `yang/` | YANG schema tools | Content-agnostic |
| `migration/` | Ze internal config migration | Generic format evolution |
| `env/` | Environment variable parsing | Generic |

**Behavior to preserve:**
- All config parsing produces identical output
- All existing tests pass unchanged (just moved)
- Import paths are the only change — no logic modifications

**Behavior to change:** None — pure file relocation

## Data Flow (MANDATORY)

### Entry Point
- Config file → tokenizer → parser → Tree → schema validation
- BGP extraction: Tree → peers.go functions → reactor.PeerSettings

### Transformation Path
1. Config file parsed by generic config package → Tree
2. `loader.go` calls BGP-specific extractors (now in `bgp/config/`)
3. BGP extractors call `reactor.PeersFromTree()`, build PeerSettings
4. Route types converted by `loader_routes.go` → reactor route types

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| config → bgp/config | Function calls for BGP extraction | [ ] |
| bgp/config → reactor | `PeersFromTree()`, `PluginConfig` | [ ] |
| config → Tree | Generic parser output | [ ] |

### Integration Points
- `internal/component/config/loader.go` — calls BGP extractors, must import new package
- `internal/component/config/serialize.go` — may reference BGP types for serialization
- `cmd/ze/bgp/` — may import config BGP types directly
- Any file importing config package for BGP types

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling (config package loses BGP imports)
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config file with peer block | → | bgp/config peer extraction | `TestPeerSettingsFromConfig` (existing, moved) |
| Config file with routes | → | bgp/config route extraction | `TestRouteExtraction` (existing, moved) |
| `ze config check` with full config | → | Schema validation + BGP extraction | Existing functional tests |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `internal/component/config/*.go` (non-test) | No imports of `reactor`, `capability`, `nlri`, `message`, `bgptypes` |
| AC-2 | `internal/component/bgp/config/` | Contains all BGP-specific config files |
| AC-3 | `make ze-test` | All tests pass |
| AC-4 | `go build ./...` | No import cycles |
| AC-5 | Dead environment fields | tcp.delay, tcp.acl, reactor.speed, reactor.cache-ttl removed from YANG |
| AC-6 | Dead environment containers | bgp, cache, api containers removed from YANG |
| AC-7 | `environment.go` | Corresponding struct fields and envOptions entries removed |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| All existing BGP config tests | `internal/component/bgp/config/*_test.go` | Moved tests still pass | |
| `TestEnvironmentLoadUsedFields` | `internal/component/config/environment_test.go` | AC-5/6/7: only used fields remain | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A — pure refactoring, no new numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing parse tests | `test/parse/` | Config parsing unchanged | |
| Existing encode tests | `test/encode/` | BGP encoding unchanged | |

### Future
- None deferred

## Files to Modify

### Move to `internal/component/bgp/config/`
- `bgp.go` — BGP route config types
- `bgp_util.go` — BGP utility functions
- `bgp_routes.go` — route extraction
- `peers.go` — peer settings extraction
- `routeattr.go` — route attribute types
- `routeattr_community.go` — community parsing
- `routeattr_prefixsid.go` — prefix-SID parsing
- `loader_routes.go` — route type converters
- `loader_prefix.go` — prefix splitting
- `plugins.go` — plugin config extraction
- `validators.go` — BGP validators
- `validators_register.go` — validator init
- All corresponding `*_test.go` files

### Update importers
- `internal/component/config/loader.go` — import `bgp/config` for BGP extraction calls
- `internal/component/config/serialize.go` — reference BGP types from new location
- Any external consumers of BGP config types

### Dead field cleanup
- `internal/component/bgp/schema/ze-bgp-conf.yang` — remove unused leaves/containers
- `internal/component/config/environment.go` — remove struct fields and envOptions entries

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (dead fields) | Yes | `ze-bgp-conf.yang`, `ze-plugin-conf.yang` |
| Config editor | No | YANG-driven, automatic |
| CLI commands | No | No new commands |
| Architecture docs | Yes | `docs/architecture/config/syntax.md` |

## Files to Create
- `internal/component/bgp/config/` — new package (files moved here)

## Implementation Steps

### Phase 1: BGP Config File Move
1. Create `internal/component/bgp/config/` package
2. `git mv` all BGP-specific files from `config/` to `bgp/config/`
3. Update package declarations
4. Update all import paths
5. Verify build and tests

### Phase 2: Dead Environment Field Cleanup
1. Remove unused YANG leaves: tcp.delay, tcp.acl, reactor.speed, reactor.cache-ttl
2. Remove unused YANG containers: bgp, cache, api
3. Remove corresponding `envOptions` entries and struct fields in `environment.go`
4. Update environment tests
5. Verify `make ze-test`

### Failure Routing

| Failure | Route To |
|---------|----------|
| Import cycle after move | Check dependency direction: config → bgp/config → reactor |
| Test compilation error | Update import paths in test files |
| Serialization breaks | Check serialize.go references BGP types correctly |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Implementation Summary

### What Was Implemented
- [To be filled after implementation]

### Documentation Updates
- [To be filled]

### Deviations from Plan
- [To be filled]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `docs/learned/NNN-<name>.md`
- [ ] Summary included in commit
