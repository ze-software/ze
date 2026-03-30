# Spec: peer-yang-reorg

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-30 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/config/syntax.md` - current config syntax reference
4. `internal/component/bgp/schema/ze-bgp-conf.yang` - current YANG schema
5. `internal/component/bgp/reactor/config.go` - peer config parsing
6. `internal/component/bgp/config/resolve.go` - 3-level inheritance resolution

## Task

Reorganize the YANG peer configuration to separate concerns into logical containers:

- **`connection`** -- transport-level: IPs, ports, connect/accept, MD5, TTL, link-local
- **`session`** -- BGP session: ASN, router-id, link-local next-hop, families, capabilities, add-path
- **`behaviour`** -- operational knobs: group-updates, manual-eor, auto-flush
- **`rib`** -- RIB config: adj-rib-in/out (moved from peer-level booleans), existing `rib > out` unchanged
- Prefix enforcement (teardown, idle-timeout, updated) moves per-family into `family > prefix`

Additionally, create a config migration tool using the ze-engine to convert old-format configs to the new structure. The tool itself is disposable, but it lays the foundation for future config migration infrastructure.

## Current → New Mapping

### connection (NEW container)

| Current Location | New Location | Notes |
|-----------------|--------------|-------|
| `local > ip` (augmented) | `connection > local > ip` | augment path changes |
| `local > connect` | `connection > local > connect` | |
| `remote > ip` (augmented) | `connection > remote > ip` | augment path changes |
| `remote > accept` | `connection > remote > accept` | |
| `port` (peer-level leaf) | `connection > local > port` + `connection > remote > port` | split into two: local bind port, remote connection port |
| `md5-password` (peer-level leaf) | `connection > md5 > password` | |
| `md5-ip` (peer-level leaf) | `connection > md5 > ip` | |
| `ttl-security` (peer-level leaf) | `connection > ttl > max` | renamed |
| `outgoing-ttl` (peer-level leaf) | `connection > ttl > set` | renamed |
| `incoming-ttl` (peer-level leaf) | `connection > ttl > min` | renamed |
| `link-local` (peer-level leaf, IPv6 addr) | `connection > link-local` (boolean) | type change: IPv6 address to boolean |

### session (NEW container)

| Current Location | New Location | Notes |
|-----------------|--------------|-------|
| `local > as` | `session > asn > local` | |
| `remote > as` | `session > asn > remote` | |
| `router-id` (peer-level leaf) | `session > router-id` | |
| `link-local` (peer-level leaf) | `session > link-local` | IPv6 address for MP_REACH_NLRI encoding (RFC 2545) |
| `family` (peer-level list) | `session > family` | |
| `capability` (peer-level container) | `session > capability` | |
| `add-path` (peer-level list) | `session > add-path` | |

### family > prefix (MERGED)

| Current Location | New Location | Notes |
|-----------------|--------------|-------|
| `family > prefix > maximum` | `session > family > prefix > maximum` | unchanged, moves with family |
| `family > prefix > warning` | `session > family > prefix > warning` | unchanged, moves with family |
| `prefix > teardown` (peer-level) | `session > family > prefix > teardown` | moved per-family |
| `prefix > idle-timeout` (peer-level) | `session > family > prefix > idle-timeout` | moved per-family |
| `prefix > updated` (peer-level) | `session > family > prefix > updated` | moved per-family |

### behaviour (NEW container)

| Current Location | New Location | Notes |
|-----------------|--------------|-------|
| `group-updates` (peer-level leaf) | `behaviour > group-updates` | |
| `manual-eor` (peer-level leaf) | `behaviour > manual-eor` | |
| `auto-flush` (peer-level leaf) | `behaviour > auto-flush` | |

### rib (MODIFIED container)

| Current Location | New Location | Notes |
|-----------------|--------------|-------|
| `adj-rib-in` (peer-level leaf) | `rib > adj > in` | moved from flat leaf to nested container |
| `adj-rib-out` (peer-level leaf) | `rib > adj > out` | moved from flat leaf to nested container |
| `rib > out > group-updates` | `rib > out > group-updates` | unchanged |
| `rib > out > auto-commit-delay` | `rib > out > auto-commit-delay` | unchanged |
| `rib > out > max-batch-size` | `rib > out > max-batch-size` | unchanged |

### Unchanged (stay at peer-level)

| Field | Reason |
|-------|--------|
| `description` | Peer metadata |
| `timer` | Contains both connection timers (connect-retry) and session timers (hold times); keeping together is simpler |
| `process` | Plugin binding, orthogonal to connection/session/behaviour |
| `redistribution` | Filter policy, orthogonal |
| `update` | Route announcements, orthogonal |

## New YANG Structure (peer-fields grouping)

```
grouping peer-fields {
    container connection {
        container local {
            leaf ip;           # augmented at peer level only
            leaf port;
            leaf connect;
        }
        container remote {
            leaf ip;           # augmented at peer level only
            leaf port;
            leaf accept;
        }
        container md5 {
            leaf ip;
            leaf password;
        }
        container ttl {
            leaf max;          # GTSM (RFC 5082)
            leaf set;          # outgoing TTL
            leaf min;          # minimum incoming TTL
        }
        leaf link-local;       # boolean: auto-discover IPv6 link-local for TCP
    }

    container session {
        container asn {
            leaf local;
            leaf remote;
        }
        leaf router-id;
        leaf link-local;       # IPv6 address for MP_REACH_NLRI (RFC 2545)
        list family {
            leaf name;
            leaf mode;
            container prefix {
                leaf maximum;
                leaf warning;
                leaf teardown;
                leaf idle-timeout;
                leaf updated;
            }
        }
        container capability { ... }  # unchanged internally
        list add-path { ... }         # unchanged internally
    }

    container behaviour {
        leaf group-updates;
        leaf manual-eor;
        leaf auto-flush;
    }

    leaf description;

    leaf router-id;  # REMOVED from peer-level (moved to session)

    container timer { ... }  # unchanged

    container rib {
        container adj {
            leaf in;
            leaf out;
        }
        container out { ... }  # unchanged
    }

    # Removed from peer-level:
    # - leaf link-local (split: bool in connection, IPv6 in session)
    # - leaf port (moved to connection > local/remote)
    # - leaf md5-password, md5-ip (moved to connection > md5)
    # - leaf ttl-security, outgoing-ttl, incoming-ttl (moved to connection > ttl)
    # - leaf group-updates, manual-eor, auto-flush (moved to behaviour)
    # - leaf adj-rib-in, adj-rib-out (moved to rib > adj)
    # - container prefix (merged per-family)

    uses update-block;
    list process { ... }          # unchanged
    container redistribution { ... } # unchanged
}
```

## Plugin YANG Augment Path Changes

Plugins that augment peer paths must update their augment targets:

| Plugin | Current Augment Target | New Augment Target |
|--------|----------------------|-------------------|
| Graceful Restart | `/bgp:peer/bgp:capability` | `/bgp:peer/bgp:session/bgp:capability` |
| Hostname | `/bgp:peer/bgp:capability` + `/bgp:peer` (legacy) | `/bgp:peer/bgp:session/bgp:capability` + `/bgp:peer/bgp:session` (legacy) |
| Software Version | `/bgp:peer/bgp:capability` | `/bgp:peer/bgp:session/bgp:capability` |
| Link-Local NH | `/bgp:peer/bgp:capability` | `/bgp:peer/bgp:session/bgp:capability` |
| Community Filter | `/bgp:peer` | `/bgp:peer` (unchanged -- filter is policy, stays at peer-level) |
| BGP Role | `/bgp:peer` | `/bgp:peer` (unchanged -- role is policy, stays at peer-level) |

Each plugin has 3 augment paths (standalone peer, grouped peer, group). All 3 must be updated.

## ExaBGP Migration Bridge

`internal/exabgp/migration/migrate.go` maps ExaBGP fields to ze config tree. All field mappings must target the new paths:

| ExaBGP Field | Current Ze Target | New Ze Target |
|-------------|-------------------|---------------|
| `peer-as` | `remote > as` | `session > asn > remote` |
| `local-as` | `local > as` | `session > asn > local` |
| `local-address` | `local > ip` | `connection > local > ip` |
| `router-id` | `router-id` | `session > router-id` |
| `hold-time` | `timer > receive-hold-time` | `timer > receive-hold-time` (unchanged) |
| `passive` | `local > connect false` / `remote > accept true` | `connection > local > connect false` / `connection > remote > accept true` |
| `ttl-security` | `ttl-security` | `connection > ttl > max` |
| `md5-password` | `md5-password` | `connection > md5 > password` |
| `group-updates` | `group-updates` | `behaviour > group-updates` |
| `auto-flush` | `auto-flush` | `behaviour > auto-flush` |
| `local-link-local` | `link-local` | `connection > link-local true` + `session > link-local <addr>` |
| `capability` | `capability` | `session > capability` |
| `family` | `family` | `session > family` |
| `host-name`/`domain-name` | peer-level (legacy) | `session` (legacy) |

## Config Migration Tool

A new CLI subcommand: `ze bgp config migrate-format` (or similar name -- open to discussion).

**Purpose:** Read a ze config file in the old peer format and output it in the new format.

**Implementation:** Use the ze config engine (YANG parser + tree) to:
1. Parse the old-format config into a tree
2. Walk the tree and relocate fields per the mapping table above
3. Output the new-format config

This tool is foundational -- it establishes the pattern for future config format migrations. The tool itself may not be kept long-term, but the migration infrastructure it creates will be reused.

**Approach:** The ze config engine already parses config into `map[string]any` trees. The migration is a tree-to-tree transformation: walk the old tree, build a new tree with relocated keys. The YANG schema drives both parsing and output.

## 3-Level Inheritance Impact

`ResolveBGPTree()` deep-merges containers at key level. The new nested containers (`connection`, `session`, `behaviour`) must participate in the same merge:

- BGP-level `local > as` becomes BGP-level `session > asn > local` (inherited by all peers)
- Group-level `session > capability` deep-merges with peer-level `session > capability`
- Peer-level `connection > local > ip` overrides (no inheritance -- peer-only via augment)

The deep-merge logic in `resolve.go` is container-name-agnostic (it merges any `map[string]any`), so nesting deeper does not break the algorithm. But the merge targets change: what was `remote` at group-level is now `session > asn > remote`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - current config syntax
  -> Decision: YANG-driven parser dispatches by node type
  -> Constraint: 3-level inheritance with deep-merge for containers
- [ ] `docs/architecture/core-design.md` - system architecture

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - BGP-4 (hold time, connect-retry, connection semantics)
- [ ] `rfc/short/rfc5082.md` - GTSM (ttl-security semantics)
- [ ] `rfc/short/rfc2545.md` - IPv6 link-local next-hop in MP_REACH_NLRI

**Key insights:**
- YANG-driven parsing means YANG changes drive config parsing automatically
- 3-level inheritance is container-name-agnostic deep-merge
- Plugin YANG augments target specific paths that will change

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` - peer-fields grouping with flat structure
- [ ] `internal/component/bgp/config/resolve.go` - ResolveBGPTree, 3-level inheritance, deep-merge
- [ ] `internal/component/bgp/config/peers.go` - PeersFromConfigTree, builds PeerSettings
- [ ] `internal/component/bgp/reactor/config.go` - PeersFromTree, parsePeerFromTree
- [ ] `internal/component/bgp/reactor/peersettings.go` - PeerSettings struct
- [ ] `internal/exabgp/migration/migrate.go` - ExaBGP to ze migration field mappings
- [ ] Plugin YANG files (6 plugins with peer augments)

**Behavior to preserve:**
- 3-level inheritance (bgp globals > group defaults > peer overrides)
- Deep-merge for containers at key level
- All peer field semantics (types, defaults, validation)
- ExaBGP migration produces valid new-format configs
- All existing functional tests pass with updated configs
- Plugin augment behavior (capabilities added to correct container)

**Behavior to change:**
- Flat peer-fields grouping -> nested connection/session/behaviour/rib structure
- Separate peer-level prefix enforcement -> per-family prefix enforcement
- `link-local` (single IPv6 leaf) -> split: `connection > link-local` (bool) + `session > link-local` (IPv6)
- `port` (single leaf) -> split: `connection > local > port` + `connection > remote > port`
- `ttl-security`/`outgoing-ttl`/`incoming-ttl` -> `connection > ttl > max`/`set`/`min`
- `md5-password`/`md5-ip` -> `connection > md5 > password`/`ip`

## Data Flow (MANDATORY)

### Entry Point
- Config file text parsed by YANG-driven parser into `map[string]any` tree
- YANG schema defines structure; parser dispatches by node type

### Transformation Path
1. Config file -> YANG parser -> raw tree (`map[string]any`)
2. `ResolveBGPTree()` -> resolved tree (3-level inheritance applied)
3. `PeersFromConfigTree()` -> `PeerSettings` structs
4. `PeersFromTree()` -> reactor peer configuration

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config file -> Tree | YANG-driven parser | [ ] |
| Tree -> Resolved tree | ResolveBGPTree deep-merge | [ ] |
| Resolved tree -> PeerSettings | Field-by-field extraction in config.go/peers.go | [ ] |
| ExaBGP -> Ze tree | migrate.go field mapping | [ ] |

### Integration Points
- YANG schema drives parsing (change schema = change parsing)
- `parsePeerFromTree()` extracts fields by string key from tree -- all key names must match new YANG paths
- `ResolveBGPTree()` merge logic works on container names -- new containers must be listed for merge
- Plugin YANG augments target absolute paths -- must match new structure
- `.ci` test configs must use new syntax

### Architectural Verification
- [ ] No bypassed layers (YANG drives parsing, no hardcoded paths)
- [ ] No unintended coupling (each container is a self-contained concern)
- [ ] No duplicated functionality (migration tool reuses config engine)
- [ ] Zero-copy preserved where applicable (config parsing is not hot path)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config file with new `connection {}` syntax | -> | YANG parser + ResolveBGPTree + PeersFromTree | `test/parse/peer-connection-session.ci` |
| Config file with new `session {}` syntax | -> | YANG parser + ResolveBGPTree + PeersFromTree | `test/parse/peer-connection-session.ci` |
| Config file with per-family prefix enforcement | -> | PeersFromTree prefix extraction | `test/parse/peer-family-prefix.ci` |
| ExaBGP config with old fields | -> | migrate.go new field targets | ExaBGP compat tests |
| `ze bgp config migrate-format` | -> | migration tool | `test/parse/migrate-format.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with `connection { local { ip ...; port ...; connect true; } remote { ip ...; port ...; accept true; } }` | Parses correctly, peer has correct address/port/connection mode |
| AC-2 | Config with `connection { md5 { ip ...; password ...; } }` | Parses correctly, peer has MD5 auth configured |
| AC-3 | Config with `connection { ttl { max 1; set 64; min 128; } }` | Parses correctly, peer has GTSM + TTL values |
| AC-4 | Config with `connection { link-local true; }` | Parses correctly, peer auto-discovers link-local |
| AC-5 | Config with `session { asn { local 65000; remote 65001; } router-id 1.2.3.4; }` | Parses correctly, peer has correct ASN and router-id |
| AC-6 | Config with `session { family { ipv4/unicast { prefix { maximum 1000; teardown true; idle-timeout 30; } } } }` | Parses correctly, per-family prefix enforcement works |
| AC-7 | Config with `session { capability { ... } add-path { ... } }` | Parses correctly, capabilities and add-path negotiated |
| AC-8 | Config with `behaviour { group-updates true; manual-eor false; auto-flush true; }` | Parses correctly, behaviour flags set |
| AC-9 | Config with `rib { adj { in true; out true; } }` | Parses correctly, adj-rib flags set |
| AC-10 | 3-level inheritance: bgp-level `session { asn { local 65000; } }`, group-level `session { capability { ... } }`, peer overrides | Inheritance works correctly with new nesting |
| AC-11 | ExaBGP config migrated via `ze bgp config migrate` | Output uses new format (connection/session/behaviour) |
| AC-12 | Old-format ze config via `ze bgp config migrate-format` | Output uses new format |
| AC-13 | Old-format config fields (`remote { as; }`, `local { as; }`, peer-level `md5-password`, etc.) | Rejected by parser (no backwards compat -- ze has no releases) |
| AC-14 | Plugin YANG augments (GR, hostname, softver, llnh) | Augment into `session > capability` correctly |
| AC-15 | Config with `session { link-local fe80::1; }` | Parses correctly, IPv6 link-local used in MP_REACH_NLRI encoding |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPeersFromTree_ConnectionFields` | `internal/component/bgp/reactor/config_test.go` | connection container parsing | |
| `TestPeersFromTree_SessionFields` | `internal/component/bgp/reactor/config_test.go` | session container parsing | |
| `TestPeersFromTree_BehaviourFields` | `internal/component/bgp/reactor/config_test.go` | behaviour container parsing | |
| `TestPeersFromTree_RibAdj` | `internal/component/bgp/reactor/config_test.go` | rib > adj parsing | |
| `TestPeersFromTree_PerFamilyPrefix` | `internal/component/bgp/reactor/config_test.go` | per-family prefix enforcement | |
| `TestResolveBGPTree_NestedInheritance` | `internal/component/bgp/config/resolve_test.go` | 3-level merge with new containers | |
| `TestMigration_NewFieldPaths` | `internal/exabgp/migration/migrate_test.go` | ExaBGP migration targets new paths | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `ttl > max` | 0-255 | 255 | N/A (uint8) | 256 |
| `ttl > set` | 0-255 | 255 | N/A (uint8) | 256 |
| `ttl > min` | 0-255 | 255 | N/A (uint8) | 256 |
| `connection > local > port` | 1-65535 | 65535 | 0 | 65536 |
| `connection > remote > port` | 1-65535 | 65535 | 0 | 65536 |
| `family > prefix > maximum` | 1-max | max uint32 | 0 | N/A |
| `family > prefix > idle-timeout` | 0-65535 | 65535 | N/A (uint16) | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `peer-connection-session` | `test/parse/peer-connection-session.ci` | Config with full new structure parses | |
| `peer-family-prefix` | `test/parse/peer-family-prefix.ci` | Per-family prefix enforcement parses | |
| `peer-behaviour` | `test/parse/peer-behaviour.ci` | Behaviour container parses | |
| `peer-rib-adj` | `test/parse/peer-rib-adj.ci` | RIB adj container parses | |
| `migrate-format` | `test/parse/migrate-format.ci` | Old format migrated to new | |
| Updated ExaBGP compat tests | `test/exabgp-compat/encoding/conf-*.ci` | ExaBGP migration outputs new format | |

### Future (if deferring any tests)
- None -- no deferrals planned

## Files to Modify

### YANG Schemas
- `internal/component/bgp/schema/ze-bgp-conf.yang` - restructure peer-fields grouping, update augments
- `internal/component/bgp/plugins/gr/schema/ze-graceful-restart.yang` - update augment paths
- `internal/component/bgp/plugins/hostname/schema/ze-hostname.yang` - update augment paths
- `internal/component/bgp/plugins/softver/schema/ze-softver.yang` - update augment paths
- `internal/component/bgp/plugins/llnh/schema/ze-link-local-nexthop.yang` - update augment paths

### Config Parsing
- `internal/component/bgp/config/resolve.go` - update merge targets for new containers
- `internal/component/bgp/config/peers.go` - update field extraction paths
- `internal/component/bgp/reactor/config.go` - update parsePeerFromTree field paths
- `internal/component/bgp/reactor/peersettings.go` - update struct if needed (LinkLocalBool field)

### ExaBGP Migration
- `internal/exabgp/migration/migrate.go` - update all field target paths

### Tests
- `internal/component/bgp/reactor/config_test.go` - update tree keys in test data
- `internal/component/bgp/config/resolve_test.go` - update tree keys in test data
- `internal/component/bgp/config/peers_test.go` - update tree keys
- `internal/exabgp/migration/migrate_test.go` - update expected output paths
- `test/parse/*.ci` - update config syntax in parse tests
- `test/ui/cli-completion-*.ci` - update completion paths
- `test/exabgp-compat/encoding/conf-*.ci` - update expected output format
- All other `.ci` files with peer config

### Documentation
- `docs/architecture/config/syntax.md` - update peer keywords section

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (restructure) | [x] | `internal/component/bgp/schema/ze-bgp-conf.yang` |
| CLI commands/flags | [x] | Migration tool subcommand |
| Editor autocomplete | [x] | YANG-driven (automatic if YANG updated) |
| Functional test for new structure | [x] | `test/parse/peer-connection-session.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - config restructuring |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` - new peer structure |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` - migrate-format |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/configuration.md` |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A (config structure, not architecture) |

## Files to Create
- `test/parse/peer-connection-session.ci` - functional test for new structure
- `test/parse/peer-family-prefix.ci` - functional test for per-family prefix
- `test/parse/peer-behaviour.ci` - functional test for behaviour container
- `test/parse/peer-rib-adj.ci` - functional test for rib adj container
- `test/parse/migrate-format.ci` - functional test for migration tool
- Migration tool code (location TBD -- likely `cmd/ze/bgp/` or `internal/component/config/`)

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: YANG schema** -- Restructure `peer-fields` grouping, update augments in base file and all 4 plugin YANG files
   - Tests: YANG parsing (existing tests should still compile)
   - Files: `ze-bgp-conf.yang`, 4 plugin `.yang` files
   - Verify: `make ze-lint` passes (YANG validation)

2. **Phase: Config resolution** -- Update `ResolveBGPTree` merge targets, update `PeersFromConfigTree` and `parsePeerFromTree` field extraction
   - Tests: `TestPeersFromTree_ConnectionFields`, `TestPeersFromTree_SessionFields`, `TestPeersFromTree_BehaviourFields`, `TestPeersFromTree_RibAdj`, `TestPeersFromTree_PerFamilyPrefix`, `TestResolveBGPTree_NestedInheritance`
   - Files: `resolve.go`, `peers.go`, `reactor/config.go`, `peersettings.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: ExaBGP migration** -- Update field target paths in migrate.go
   - Tests: `TestMigration_NewFieldPaths`
   - Files: `migrate.go`, `migrate_test.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Migration tool** -- Create `ze bgp config migrate-format` using ze config engine
   - Tests: `test/parse/migrate-format.ci`
   - Files: new migration tool code
   - Verify: tool reads old format, outputs new format

5. **Phase: Update all tests** -- Update `.ci` configs, Go test data, completion tests
   - Tests: all existing tests updated to new syntax
   - Files: `test/parse/*.ci`, `test/ui/*.ci`, `test/exabgp-compat/encoding/conf-*.ci`, Go test files
   - Verify: `make ze-verify` passes

6. **Phase: Documentation** -- Update syntax.md, configuration guide, command reference
   - Files: `docs/architecture/config/syntax.md`, `docs/guide/configuration.md`, `docs/guide/command-reference.md`
   - Verify: docs match new YANG structure

7. **Complete spec** -- Fill audit tables, write learned summary to `plan/learned/497-peer-yang-reorg.md`, delete spec from `plan/`. BLOCKING: summary is part of the commit, not a follow-up.

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every field from the mapping table has been relocated |
| Correctness | 3-level inheritance works with new nesting depth |
| Naming | YANG uses kebab-case, JSON output uses kebab-case, Go field names follow convention |
| Data flow | Config file -> YANG parse -> resolve -> PeerSettings all use new paths consistently |
| Rule: no-layering | Old field locations fully removed from YANG (no dual paths) |
| Rule: compatibility | No backwards compat needed (ze has no releases) |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| YANG schema restructured | `grep 'container connection' ze-bgp-conf.yang` |
| Plugin augments updated | `grep 'session/bgp:capability' ze-graceful-restart.yang` |
| Config parsing updated | `go test -run TestPeersFromTree ./internal/component/bgp/reactor/...` |
| ExaBGP migration updated | `go test -run TestMigration ./internal/exabgp/migration/...` |
| Migration tool exists | `ze bgp config migrate-format --help` |
| Per-family prefix enforcement | `go test -run TestPerFamilyPrefix` |
| All .ci tests updated | `make ze-functional-test` passes |
| Docs updated | `grep 'connection' docs/architecture/config/syntax.md` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | TTL values (0-255), port values (1-65535), prefix limits -- all carried over from current validation |
| Sensitive fields | `md5 > password` still has `ze:sensitive` annotation |
| Migration tool | Does not execute config, only transforms it |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
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

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer constraints, message ordering, any MUST/MUST NOT.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |

### Tests from TDD Plan
| Test | Status | Location | Notes |

### Files from Plan
| File | Status | Notes |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
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
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/497-peer-yang-reorg.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
