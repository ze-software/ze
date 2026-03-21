# Spec: peer-local-remote

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/4 |
| Updated | 2026-03-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/schema/ze-bgp-conf.yang` - YANG schema (the primary change)
4. `internal/component/bgp/reactor/config.go` - config tree parsing
5. `internal/component/bgp/reactor/reactor_api.go` - API peer add handler

## Task

Restructure BGP peer configuration for better autocomplete and semantic clarity:

1. **Peer keyed by name** (was: keyed by IP address). Name is mandatory (it is the key). IP moves to `remote > ip` (mandatory).
2. **`local` container**: `as` and `ip` children replace flat `local-as` and `local-address` leaves.
3. **`remote` container**: `as` and `ip` children replace flat `peer-as` and the former list key `address`.
4. **Global `local > as`** at bgp level replaces flat `local-as`.

Config before/after:
| Before | After |
|--------|-------|
| `peer 10.0.0.2 { peer-as 65002; local-as 65001; local-address 10.0.0.1; }` | `peer upstream-1 { remote { as 65002; ip 10.0.0.2; } local { as 65001; ip 10.0.0.1; } }` |
| `bgp { local-as 65001; }` | `bgp { local { as 65001; } }` |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config parsing pipeline
  → Constraint: Schema is auto-generated from YANG. Changing YANG cascades to schema.
- [ ] `docs/architecture/core-design.md` - reactor config loading
  → Constraint: PeersFromTree reads flat map keys, needs nested map navigation.

### RFC Summaries
- N/A (no protocol wire format changes, config-only)

**Key insights:**
- Schema auto-generated from YANG via `yang_schema.go` `yangToNode()`. YANG containers become `ContainerNode`, lists become `ListNode`.
- `parsePeerFromTree()` in `reactor/config.go` reads config map with helpers `mapString`, `mapUint32`, `mapMap`. Nested containers become nested `map[string]any`.
- `PeerSettings` struct fields (Address, LocalAS, PeerAS, LocalAddress) stay the same -- only the config path to populate them changes.
- Plugin YANG augments reference `/bgp:bgp/bgp:peer` which still exists -- key change is transparent to augments.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` - peer list keyed by `address`, flat `local-as`/`local-address`/`peer-as` in `peer-fields` grouping
  → Constraint: `peer-fields` grouping used by standalone peers, group-level peers, and peers inside groups
- [ ] `internal/component/bgp/reactor/config.go` - `parsePeerFromTree(addr, tree, localAS, routerID)` reads "peer-as", "local-as", "local-address" from flat map
  → Constraint: `PeersFromTree` iterates `peerMap` where key=IP address, passes key as `addr` to `parsePeerFromTree`
- [ ] `internal/component/bgp/reactor/reactor_api.go:429-480` - `parsePeersFromTree` also reads peer addresses as map keys and "local-as"/"peer-as" as flat fields
  → Constraint: Two separate parsePeersFromTree functions (config.go and reactor_api.go) -- both need updating
- [ ] `internal/component/config/yang_schema.go` - YANG-to-schema auto-conversion
  → Constraint: Schema built from YANG at runtime; no hand-coded schema changes needed

**Behavior to preserve:**
- PeerSettings struct fields and their semantics (Address, LocalAS, PeerAS, LocalAddress)
- Plugin YANG augment paths work unchanged
- Config validation (required fields, IP format, ASN range)
- Set-format parsing and serialization round-trip
- Hierarchical format parsing and serialization round-trip

**Behavior to change:**
- Peer list key: `address` (IP) to `name` (human-readable, mandatory)
- `local-as` leaf to `local > as` container path (peer-fields and bgp level)
- `local-address` leaf to `local > ip` container path (peer-fields)
- `peer-as` leaf to `remote > as` container path (peer-fields)
- Peer `address` leaf to `remote > ip` (mandatory, unique across peers)

## Data Flow (MANDATORY)

### Entry Point
- Config file (hierarchical or set format) parsed by `parser.go` / `setparser.go`
- Schema from YANG determines valid paths

### Transformation Path
1. YANG modules loaded, resolved, converted to Schema via `yang_schema.go`
2. Config text parsed against Schema into Tree
3. Tree.ToMap() produces `map[string]any`
4. `PeersFromTree(bgpTree)` iterates peer map, calls `parsePeerFromTree` per peer
5. `parsePeerFromTree` reads nested maps for `local`/`remote` containers, produces `*PeerSettings`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG to Schema | Auto-conversion in `yang_schema.go` | [ ] |
| Schema to Tree | Parser validates against schema | [ ] |
| Tree to PeerSettings | `config.go` reads map keys | [ ] |
| API peer-add | `reactor_api.go` reads map keys | [ ] |

### Integration Points
- `parsePeerFromTree` - reads nested `local`/`remote` maps instead of flat keys
- `PeersFromTree` - peer map key is now name, not address
- `parsePeersFromTree` (reactor_api.go) - same changes for API path
- Plugin augments on `/bgp:bgp/bgp:peer` - unchanged (key type transparent)

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved (N/A -- config parsing, not wire)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config file with `peer name { remote { ip ...; as ...; } local { as ...; ip ...; } }` | -> | `parsePeerFromTree` | `TestParsePeerLocalRemote` in `config_test.go` |
| Config file parsed + serialized back | -> | SetParser + SerializeSet | `TestSetParserLocalRemote` in `setparser_test.go` |
| Hierarchical config parse | -> | Full pipeline | `test/parse/local-remote-*.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with `peer myname { remote { ip 10.0.0.2; as 65002; } local { as 65001; ip 10.0.0.1; } }` | Parsed to PeerSettings with Address=10.0.0.2, PeerAS=65002, LocalAS=65001, LocalAddress=10.0.0.1, Name=myname |
| AC-2 | Config with `bgp { local { as 65001; } }` as global default | Global local-as=65001 applied to peers without local override |
| AC-3 | Peer missing `remote > ip` | Parse error: mandatory field |
| AC-4 | Peer missing `remote > as` | Parse error: missing required remote as |
| AC-5 | Two peers with same `remote > ip` | YANG unique constraint violation or validation error |
| AC-6 | Set-format round-trip: parse then serialize | Output matches input structure |
| AC-7 | Old flat keys `local-as`, `peer-as`, `local-address` at peer level | YANG validation rejects unknown keys |
| AC-8 | Functional test: config with local/remote parses successfully | `ze bgp config check` exits 0 |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParsePeerLocalRemote` | `reactor/config_test.go` | AC-1: nested local/remote parsed correctly | |
| `TestParsePeerGlobalLocalAS` | `reactor/config_test.go` | AC-2: global local > as default | |
| `TestParsePeerMissingRemoteIP` | `reactor/config_test.go` | AC-3: error on missing remote ip | |
| `TestParsePeerMissingRemoteAS` | `reactor/config_test.go` | AC-4: error on missing remote as | |
| `TestSetParserLocalRemote` | `config/setparser_test.go` | AC-6: set format parse | |
| `TestSerializeSetLocalRemote` | `config/serialize_set_test.go` | AC-6: set format serialize | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `local-remote-basic` | `test/parse/local-remote-basic.ci` | Config with local/remote containers parses | |

### Future
- Duplicate `remote > ip` validation (AC-5) can be deferred if YANG `unique` not supported by goyang

## Files to Modify

- `internal/component/bgp/schema/ze-bgp-conf.yang` - YANG schema restructure
- `internal/component/bgp/reactor/config.go` - parsePeerFromTree, PeersFromTree
- `internal/component/bgp/reactor/config_test.go` - update test data
- `internal/component/bgp/reactor/reactor_api.go` - parsePeersFromTree for API path
- `internal/component/config/setparser_test.go` - update set-format examples
- `internal/component/config/serialize_set_test.go` - update expected output
- All .ci functional test files with peer configuration

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [x] | `internal/component/bgp/schema/ze-bgp-conf.yang` |
| CLI commands/flags | [ ] | N/A |
| Editor autocomplete | [x] | YANG-driven (automatic) |
| Functional test | [x] | `test/parse/local-remote-basic.ci` |

## Files to Create
- `test/parse/local-remote-basic.ci` - functional test for new config structure

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-verify` |
| 5-12 | Standard flow |

### Implementation Phases

1. **Phase: YANG schema** - Restructure `ze-bgp-conf.yang`
   - Add `local` container (as, ip) and `remote` container (as, ip) to peer-fields
   - Remove flat `local-as`, `local-address`, `peer-as` from peer-fields
   - Change peer list key from `address` to `name`, remove standalone `leaf address`
   - Add `unique "remote/ip"` to peer lists
   - Change bgp-level `local-as` to `local > as` container
   - Files: `ze-bgp-conf.yang`

2. **Phase: Config resolution** - Update Go code to read nested maps
   - `config.go`: parsePeerFromTree reads `remote > ip`, `remote > as`, `local > as`, `local > ip`
   - `config.go`: PeersFromTree reads `local > as` from bgp level, peer key is name
   - `reactor_api.go`: parsePeersFromTree same changes
   - Tests: update all config_test.go test data
   - Files: `config.go`, `config_test.go`, `reactor_api.go`

3. **Phase: Functional tests** - Update all .ci files
   - Update peer config blocks in all functional tests
   - Create new `test/parse/local-remote-basic.ci`
   - Files: `test/**/*.ci`

4. **Phase: Verification** - Full test suite
   - `make ze-verify`

### Critical Review Checklist
| Check | What to verify |
|-------|---------------|
| Completeness | All AC-N have tests |
| Correctness | Nested map navigation correct, error messages reference new paths |
| Naming | YANG uses kebab-case consistently (local, remote, as, ip) |
| Data flow | Both config.go and reactor_api.go updated |
| Rule: no-layering | Old flat keys fully removed, not kept alongside new containers |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| YANG has local/remote containers | `grep "container local" ze-bgp-conf.yang` |
| peer list key is name | `grep 'key "name"' ze-bgp-conf.yang` |
| parsePeerFromTree reads nested maps | `grep "remote" config.go` |
| Functional test exists | `ls test/parse/local-remote-basic.ci` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | remote > ip validated as IP address, remote > as validated as uint32 |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## RFC Documentation

N/A -- config-only change, no wire format impact.

## Implementation Summary

### What Was Implemented

### Bugs Found/Fixed

### Documentation Updates

### Deviations from Plan

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

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end

### Quality Gates
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit
