# Spec: peer-groups

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/config/syntax.md` - current template/inheritance docs
4. `internal/component/bgp/schema/ze-bgp-conf.yang` - YANG schema
5. `internal/component/bgp/config/resolve.go` - template resolution engine
6. `internal/component/bgp/config/peers.go` - route extraction from templates
7. `internal/component/bgp/reactor/config.go` - PeersFromTree parsing
8. `internal/component/bgp/reactor/peersettings.go` - PeerSettings struct

## Task

Replace ExaBGP-style template inheritance with Junos-style peer-groups:
- Peers are nested inside named groups (`bgp { group <name> { peer <ip> { } } }`)
- Groups define shared defaults; peer-level config overrides group defaults
- 3-level inheritance: bgp globals -> group defaults -> peer overrides
- Each peer gets an optional `name` usable as CLI selector instead of IP
- Delete `template {}` block entirely (no-layering rule)
- Remove glob auto-matching
- Update ExaBGP and ze-native migration to produce groups

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - current template/inheritance documentation
  -> Constraint: template section (lines 137-211) documents current 3-layer inheritance model
  -> Decision: this entire section will be rewritten for peer-groups
- [ ] `docs/architecture/core-design.md` - reactor and peer management
  -> Constraint: PeersFromTree produces []*PeerSettings consumed by reactor

### RFC Summaries (MUST for protocol work)
N/A -- this is a config model change, not a wire protocol change.

**Key insights:**
- Current model: 3 layers (glob auto-match, named template, peer override) with deep-merge
- New model: 3 layers (bgp global, group, peer) with same deep-merge semantics
- `deepMergeMaps()` in resolve.go is reusable unchanged
- `parsePeerFromTree()` in config.go takes a flat map[string]any -- unchanged, group resolution happens before this
- YANG `mandatory true` on `local-address` and `peer-as` in `peer-fields` must be removed for groups; Go validation already enforces these

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` - YANG schema with template container (l15-53), peer list (l76-86), peer-fields grouping (l90+)
- [ ] `internal/component/bgp/config/resolve.go` - `ResolveBGPTree()` with 3-layer resolution (l110-161), `extractTemplateData()` (l21-100), `templateData` struct (l13-17), `resolveInheritedTemplates()` (l163-179), `deepMergeMaps()` (l221+)
- [ ] `internal/component/bgp/config/peers.go` - `PeersFromConfigTree()` (l30+), 3-layer route extraction with `patchRoutes()`
- [ ] `internal/component/bgp/config/bgp_util.go` - `IPGlobMatch()` (l27+), `PeerGlob` type
- [ ] `internal/component/bgp/reactor/config.go` - `parsePeerFromTree()` (l35+), requires flat map[string]any as input
- [ ] `internal/component/bgp/reactor/peersettings.go` - `PeerSettings` struct (l225+), no Name or GroupName fields
- [ ] `internal/component/plugin/types.go` - `PeerInfo` struct (l25+), no Name field
- [ ] `internal/component/plugin/server/command.go` - `looksLikeIPOrGlob()` (l485+), peer selector dispatch (l273+)
- [ ] `internal/component/config/migration/migrate.go` - ze-native migration transformations
- [ ] `internal/exabgp/migration/migrate.go` - ExaBGP `expandInheritance()` and template transforms

**Behavior to preserve:**
- Deep-merge semantics: containers merge at key level, leaves override
- Route accumulation across layers (group routes + peer routes = all routes)
- `parsePeerFromTree()` interface: takes flat `map[string]any`, returns `*PeerSettings`
- `PeersFromTree()` interface: takes `map[string]any`, returns `[]*PeerSettings`
- Peer selector `*` selects all peers
- Comma-separated IP selectors
- Config validation: missing `peer-as` or `local-as` produces error

**Behavior to change:**
- Config syntax: `template { bgp { peer <pattern> { inherit-name <name> } } }` + `bgp { peer <ip> { inherit <name> } }` -> `bgp { group <name> { peer <ip> { } } }`
- Resolution: 3 layers change from (glob, named-template, peer) to (bgp-global, group, peer)
- Peers live inside groups, not at bgp top level
- Remove glob auto-matching entirely
- Peer selector accepts peer names in addition to IPs
- ExaBGP migration produces groups instead of templates

## Data Flow (MANDATORY)

### Entry Point
- Config file parsed into `config.Tree` by YANG-driven parser
- Tree contains `bgp` container with `group` list entries, each containing `peer` list entries

### Transformation Path
1. Config file -> `config.Tree` (YANG parser, unchanged)
2. `ResolveBGPTree(tree)` -> iterates `bgp.group` list
3. For each group: extract group-level fields as `map[string]any`
4. For each peer in group: deep-merge bgp-globals + group-fields + peer-fields
5. Produce `map[string]any` with flat `"peer"` map keyed by IP (same output format as today)
6. `PeersFromTree(resolved)` -> `[]*PeerSettings` (unchanged)
7. `parsePeerFromTree()` -> individual `PeerSettings` with Name and GroupName fields

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config file -> Tree | YANG parser (unchanged) | [ ] |
| Tree -> resolved map | `ResolveBGPTree()` (rewritten) | [ ] |
| Resolved map -> PeerSettings | `PeersFromTree()` (minor changes) | [ ] |
| PeerSettings -> Reactor | Same as today | [ ] |
| PeerInfo -> CLI | Add Name field | [ ] |

### Integration Points
- `ResolveBGPTree()` output format (map[string]any with "peer" key) must remain compatible with `PeersFromTree()`
- `PeerInfo` returned by reactor API must include Name for CLI selector
- YANG schema drives editor completion -- groups must appear as completable

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `group` block | -> | `ResolveBGPTree()` resolves group defaults into peers | `test/parse/group-basic.ci` |
| Config with peer `name` | -> | `PeerSettings.Name` populated | `test/parse/group-peer-name.ci` |
| CLI `peer <name> list` | -> | `filterPeersBySelector()` matches by name | `test/plugin/peer-name-selector.ci` |
| ExaBGP config with `inherit` | -> | Migration produces `group` syntax | `test/exabgp-compat/etc/conf-template.conf` (updated) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with `bgp { group X { peer <ip> { } } }` | Parses successfully, peer inherits group defaults |
| AC-2 | Group sets `hold-time 180`, peer does not | Peer's resolved hold-time is 180 |
| AC-3 | Group sets `hold-time 180`, peer sets `hold-time 90` | Peer's resolved hold-time is 90 (peer overrides) |
| AC-4 | Group sets capability, peer adds different capability | Both capabilities present (deep-merge) |
| AC-5 | bgp block sets `local-as 65000`, group does not | Peer inherits bgp-level local-as |
| AC-6 | Group sets `local-as 65001`, bgp sets `local-as 65000` | Peer uses group's 65001 (group overrides bgp-global) |
| AC-7 | Peer with `name google` | `PeerSettings.Name == "google"` |
| AC-8 | Two peers with same `name` | Config validation error |
| AC-9 | CLI `peer google list` | Selects peer with name "google" |
| AC-10 | Config with old `template { }` block | Migration converts to `group` syntax |
| AC-11 | ExaBGP config with `inherit X` | Migration produces `group X { ... }` with peers inside |
| AC-12 | Peer without group (at bgp level) | Config validation error -- all peers must be in a group |
| AC-13 | Group with routes + peer with routes | Both sets of routes present (accumulation) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResolveBGPTree_GroupDefaults` | `internal/component/bgp/config/resolve_test.go` | Group fields merge into peer | |
| `TestResolveBGPTree_PeerOverridesGroup` | `internal/component/bgp/config/resolve_test.go` | Peer values override group | |
| `TestResolveBGPTree_DeepMergeCapabilities` | `internal/component/bgp/config/resolve_test.go` | Capabilities from group + peer merge | |
| `TestResolveBGPTree_BGPGlobalInheritance` | `internal/component/bgp/config/resolve_test.go` | bgp-level local-as flows to peers via groups | |
| `TestResolveBGPTree_MultipleGroups` | `internal/component/bgp/config/resolve_test.go` | Multiple groups with different peers | |
| `TestResolveBGPTree_DuplicatePeerName` | `internal/component/bgp/config/resolve_test.go` | Error on duplicate peer names | |
| `TestResolveBGPTree_NoTemplateBlock` | `internal/component/bgp/config/resolve_test.go` | Old template block rejected | |
| `TestPeersFromConfigTree_GroupRoutes` | `internal/component/bgp/config/peers_test.go` | Route accumulation from group + peer | |
| `TestParsePeerFromTree_Name` | `internal/component/bgp/reactor/config_test.go` | Name field parsed into PeerSettings | |
| `TestParsePeerFromTree_GroupName` | `internal/component/bgp/reactor/config_test.go` | GroupName field parsed into PeerSettings | |
| `TestFilterPeersBySelector_Name` | `internal/component/bgp/plugins/cmd/peer/peer_test.go` | Selector matches peer by name | |
| `TestMigrateTemplateToGroup` | `internal/component/config/migration/migrate_test.go` | Ze-native template -> group conversion | |
| `TestMigrateExaBGPInheritToGroup` | `internal/exabgp/migration/migrate_test.go` | ExaBGP inherit -> group conversion | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A -- no new numeric fields introduced. Existing boundary tests for hold-time, port, etc. remain unchanged.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `group-basic` | `test/parse/group-basic.ci` | Config with groups parses successfully | |
| `group-peer-name` | `test/parse/group-peer-name.ci` | Config with peer names parses | |
| `group-inheritance` | `test/encode/group-inheritance.ci` | Group defaults apply to peer wire output | |
| `peer-name-selector` | `test/plugin/peer-name-selector.ci` | CLI selects peer by name | |
| `exabgp-template-migration` | `test/exabgp-compat/etc/conf-template.conf` | ExaBGP template migrates to groups | |

### Future (if deferring any tests)
- None -- all tests required for completion

## Files to Modify
- `internal/component/bgp/schema/ze-bgp-conf.yang` - restructure: delete template, add group list with nested peer list
- `internal/component/bgp/config/resolve.go` - rewrite resolution from template to group model
- `internal/component/bgp/config/resolve_test.go` - rewrite tests for group model
- `internal/component/bgp/config/peers.go` - 2-layer route extraction (group + peer)
- `internal/component/bgp/config/peers_test.go` - update route extraction tests
- `internal/component/bgp/config/bgp_util.go` - remove PeerGlob type (keep IPGlobMatch for migration)
- `internal/component/bgp/reactor/peersettings.go` - add Name, GroupName fields
- `internal/component/bgp/reactor/config.go` - parse name/group, iterate groups
- `internal/component/plugin/types.go` - add Name, GroupName to PeerInfo
- `internal/component/bgp/reactor/reactor_api.go` - populate Name/GroupName in PeerInfo
- `internal/component/plugin/server/command.go` - peer name selector support
- `internal/component/bgp/plugins/cmd/peer/peer.go` - name-based filtering + JSON output
- `internal/component/bgp/schema/ze-types.yang` - update peer-selector typedef
- `internal/component/config/migration/migrate.go` - template-to-group conversion
- `internal/component/config/migration/detect.go` - detect template blocks for migration
- `internal/exabgp/migration/migrate.go` - generate groups instead of templates
- `docs/architecture/config/syntax.md` - rewrite template section as peer-groups
- `etc/ze/bgp/conf-group.conf` - update example config

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/component/bgp/schema/ze-bgp-conf.yang` |
| RPC count in architecture docs | [ ] | N/A -- no new RPCs |
| CLI commands/flags | [ ] | N/A -- existing peer commands, new selector type |
| CLI usage/help text | [ ] | N/A |
| API commands doc | [ ] | N/A |
| Plugin SDK docs | [ ] | N/A |
| Editor autocomplete | [x] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [x] | `test/parse/group-*.ci`, `test/plugin/peer-name-selector.ci` |

## Files to Create
- `test/parse/group-basic.ci` - functional test: group config parses
- `test/parse/group-peer-name.ci` - functional test: peer name parses
- `test/encode/group-inheritance.ci` - functional test: group defaults in wire output
- `test/plugin/peer-name-selector.ci` - functional test: CLI peer name selector

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

### Step 1: YANG Schema Restructuring
1. Delete `container template` block (lines 15-53) from `ze-bgp-conf.yang`
2. Delete `list peer` from `container bgp` (lines 76-86)
3. Add `list group` in `container bgp` with `key "name"`, `uses peer-fields`, nested `list peer`
4. Add `leaf name` (optional display name) to peer list inside group
5. Replace `leaf inherit` with `leaf group` (or remove -- group membership is structural)
6. Remove `mandatory true` from `local-address` and `peer-as` in `peer-fields`
7. Run `make generate` if YANG generates code

### Step 2: Core Resolution Engine (TDD)
1. Write unit tests for new `ResolveBGPTree()` with groups
2. Run tests -> MUST FAIL
3. Rewrite `resolve.go`: delete template structs/functions, implement group resolution
4. Run tests -> MUST PASS
5. Verify `deepMergeMaps()` unchanged

### Step 3: Route Extraction (TDD)
1. Write unit tests for 2-layer route extraction
2. Run tests -> MUST FAIL
3. Rewrite `PeersFromConfigTree()` for group model
4. Run tests -> MUST PASS

### Step 4: PeerSettings + Config Parsing
1. Add Name, GroupName to `PeerSettings`
2. Update `parsePeerFromTree()` to parse name
3. Update `PeersFromTree()` to iterate groups and pass GroupName
4. Unit tests for name parsing

### Step 5: CLI Peer Name Selector
1. Add Name, GroupName to `PeerInfo`
2. Populate in reactor API adapter
3. Modify `looksLikeIPOrGlob()` / dispatcher for name support
4. Update `filterPeersBySelector()` for name matching
5. Functional test: `.ci` test for peer name selector

### Step 6: Migration Pipelines
1. Ze-native migration: template -> group conversion
2. ExaBGP migration: inherit -> group conversion
3. Unit tests for both

### Step 7: Functional Tests + Documentation
1. Write `.ci` functional tests
2. Update `etc/ze/bgp/conf-group.conf`
3. Update `docs/architecture/config/syntax.md`
4. Update existing test configs using template/inherit

### Step 8: Verification
1. `make ze-verify` (timeout 120s)
2. Critical Review (all 6 checks from quality.md)
3. Complete spec audit tables

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step that introduced the error (fix syntax/types) |
| Test fails wrong reason | Fix test |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to IMPLEMENT for that criterion |

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

## RFC Documentation
N/A -- config model change, no wire protocol changes.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-verify` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `docs/learned/393-peer-groups.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
