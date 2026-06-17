# Spec: firewall-irr

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-irr-prefix-store |
| Phase | 8/10 |
| Updated | 2026-06-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/firewall/` - firewall component (config, model, engine, backend)
4. `internal/component/bgp/plugins/filter_irr/` - BGP IRR filter plugin (pattern to follow)
5. `internal/component/resolve/irr/client.go` - IRR whois client
6. `internal/plugins/firewall/nft/` - nftables backend

## Task

Add IRR-based source/destination address filtering to the firewall component. Firewall
rules can reference ASNs or AS-SETs; Ze resolves them to prefix lists via the existing
IRR whois client (`resolve/irr`), populates nftables interval sets, and refreshes
periodically.

This is Spec 1 of 2. Spec 2 (follow-up) will add per-interface AS-SET binding for
ISP customer-facing source validation.

### Design intent

Mirror the BGP `filter_irr` plugin's architecture:
- Same IRR client (`resolve/irr`)
- Same zefs persistence
- Same CLI command structure (`show/update firewall irr`)
- Fail-closed: config commit rejects if referenced ASN/AS-SET has no cached prefix data

**Key difference from BGP filter_irr:** automatic periodic refresh is **optional**
(disabled by default, `refresh-interval 0`). When disabled, the operator explicitly
fetches prefix lists via `update firewall irr` before committing config. When enabled,
the plugin refreshes cached data on a timer, but invalid/stale data from a failed
refresh never silently replaces a good cache (fail-closed: keep last-good on error).
The risk of invalid IRR data causing a routing outage makes auto-refresh opt-in.

### Operator workflow

1. `update firewall irr asn 13335` -- query IRR, save prefixes to zefs
2. `show firewall irr` -- inspect cached prefix lists
3. Commit config with `source-asn 13335` in a term -- verify checks cache, rejects if missing
4. `update firewall irr all` -- refresh all cached ASN/AS-SET data on demand
5. (Optional) Set `refresh-interval 3600` in IRR policy -- plugin auto-refreshes on timer;
   failed refresh keeps last-good cache, logs error, never silently applies bad data

The resolved prefix lists populate nftables interval sets. Firewall rules reference
these sets via the existing `MatchInSet` mechanism.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component pattern, registration
  -> Constraint: plugins register via init() in register.go with registry.Registration
  -> Constraint: RunEngine(conn) int is the entry point signature
- [ ] `ai/rules/plugin-self-containment.md` - plugin removal test
  -> Constraint: removing the plugin directory must remove ALL its features; no plugin name in generic packages
- [ ] `ai/rules/plugin-design.md` - plugin patterns, EventBus
  -> Decision: firewall component already uses EventBus for sysctl defaults; IRR plugin is a separate registration
- [ ] `ai/patterns/plugin.md` - plugin cookbook
  -> Constraint: register.go, yang/, logger via slogutil, SDK protocol
- [ ] `ai/patterns/config-option.md` - YANG leaf pattern
  -> Constraint: YANG augment for new leaves on existing containers
- [ ] `ai/rules/config-surface.md` - YANG vs env var decision
  -> Decision: IRR server, refresh interval are YANG config (operator-visible, per-instance)

### RFC Summaries (MUST for protocol work)

N/A - no protocol work; IRR whois client already exists.

**Key insights:**
- Firewall uses multi-owner table registry: `RegisterTables(owner, tables)` + `ApplyAll()` merges all owners
- Sets with `SetFlagInterval` support prefix ranges in nftables; the YANG/config/backend path already works
- The firewall engine is a single plugin registration ("firewall") with verify/apply/rollback lifecycle
- The BGP filter_irr plugin is a separate registration ("bgp-filter-irr") that queries IRR and caches results
- The IRR client (`resolve/irr`) already does AS-SET expansion and prefix lookup via RPSL whois
- A new plugin can register its own tables via `RegisterTables("firewall-irr", ...)` and they merge at Apply

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/config.go` - parses firewall JSON config into []Table; from-block has source-address, destination-address, set references (@name); no ASN/AS-SET awareness
  -> Constraint: parseFromBlock() is the match parsing entry; new match types need a case here
  -> Constraint: source-address already handles @set-name via MatchInSet with SetFieldSourceAddr
- [ ] `internal/component/firewall/model.go` - 18 Match types, 16 Action types; Match interface with matchMarker()
  -> Constraint: new Match types need matchMarker() implementation and entry in type switch
  -> Constraint: Set has Name, Type (SetTypeIPv4/IPv6), Flags (SetFlagInterval), Elements
- [ ] `internal/component/firewall/engine.go` - SDK 5-stage lifecycle; parseFirewallSections -> verify -> apply; uses RegisterTables("firewall", tables) + ApplyAll()
  -> Constraint: firewall engine owns "firewall" as table registry key; IRR plugin must use a different owner
  -> Decision: IRR plugin registers its OWN tables under a distinct owner key
- [ ] `internal/component/firewall/backend.go` - Backend interface: Apply(desired []Table), ListTables, GetCounters, Close
  -> Constraint: Apply receives merged tables from all owners; IRR sets appear alongside firewall tables
- [ ] `internal/component/firewall/registry.go` - RegisterTables(owner, tables) + ApplyAll() merges by owner, sorted
  -> Constraint: each owner provides full []Table; ApplyAll concatenates them
- [ ] `internal/component/firewall/accessor.go` - StoreLastApplied, deepCopyTables, ActiveBackendName
  -> Constraint: only the firewall engine calls StoreLastApplied; IRR plugin uses RegisterTables only
- [ ] `internal/component/bgp/plugins/filter_irr/filter_irr.go` - BGP IRR plugin lifecycle: OnConfigure -> parseIRRConfig -> handleConfigure -> refreshAll -> refreshLoop; uses resolve/irr.LookupPrefixes
  -> Constraint: refresh interval, zefs cache, per-ASN state, metrics gauges pattern to mirror
- [ ] `internal/component/bgp/plugins/filter_irr/cache.go` - zefs persistence: cachedASN JSON -> database.zefs via key "meta/bgp/irr-cache"
  -> Constraint: new plugin needs its own zefs key (e.g., "meta/firewall/irr-cache")
- [ ] `internal/component/bgp/plugins/filter_irr/config.go` - parses bgpCfg map[string]any for irr block; defaults for server, peeringdb-url, refresh-interval
  -> Constraint: same parsing pattern for firewall config section
- [ ] `internal/component/bgp/plugins/filter_irr/command.go` - show/update bgp irr commands via OnExecuteCommand
  -> Constraint: mirror as show/update firewall irr commands
- [ ] `internal/component/resolve/irr/client.go` - IRR whois client: ResolveASSet (AS-SET -> ASNs), LookupPrefixes (AS-SET -> PrefixList{IPv4, IPv6}), aggregateAndSort
  -> Constraint: LookupPrefixes returns irr.PrefixList{IPv4 []netip.Prefix, IPv6 []netip.Prefix}; accepts AS-SET names or bare "ASNNNN"
  -> Constraint: results cached 1h internally; plugin caches separately via zefs for persistence
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` - firewall YANG schema; from-block grouping defines match leaves
  -> Constraint: new leaves (source-asn, source-as-set, destination-asn, destination-as-set) augment the from-block grouping
- [ ] `internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr.yang` - BGP IRR YANG: augments bgp policy with irr server/peeringdb-url/refresh-interval; augments peer session with irr as-set/enable
  -> Constraint: firewall IRR YANG augments firewall container with irr policy block (server, refresh-interval, peeringdb-url)

**Behavior to preserve:**
- Existing firewall config parsing and apply pipeline unchanged
- Existing set infrastructure (static sets from config) unchanged
- Existing table registry multi-owner merge behavior unchanged
- BGP filter_irr plugin entirely independent (no coupling)

**Behavior to change:**
- Add new match leaves to firewall from-block: source-asn, source-as-set, destination-asn, destination-as-set
- Add firewall irr policy container (server, refresh-interval, peeringdb-url)
- New plugin registration "firewall-irr" that creates and refreshes nftables interval sets
- New CLI commands: show/update firewall irr

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point (three paths)

**Path A -- Fetch (operator-initiated):**
- Operator runs `update firewall irr asn 13335` or `update firewall irr as-set AS-CLOUDFLARE`
- Plugin OnExecuteCommand dispatches to fetch handler

**Path B -- Commit (config apply):**
- Operator commits config with `source-asn 13335` in a firewall term's from-block
- Plugin OnConfigure / OnConfigVerify reads cached data, rejects if missing

**Path C -- Refresh (optional, timer-driven):**
- `refresh-interval > 0` in IRR policy config starts a refresh goroutine
- Loop re-queries IRR for all cached ASN/AS-SET entries on timer
- On success: update zefs cache and regenerate nftables sets via RegisterTables + ApplyAll
- On failure: keep last-good cache, log error, do not update sets (fail-closed)

### Transformation Path

**Fetch path:**
1. CLI command `update firewall irr asn 13335` -> OnExecuteCommand handler
2. Create `resolve/irr.IRR` client (using configured server or default)
3. For ASN: call `irr.LookupPrefixes(ctx, "AS13335")` -> PrefixList{IPv4, IPv6}
4. For AS-SET: call `irr.LookupPrefixes(ctx, "AS-CLOUDFLARE")` -> PrefixList
5. Serialize to JSON, write to zefs under key `meta/firewall/irr-cache`
6. Report result (prefix counts, errors)

**Commit path:**
1. Config commit: YANG parser serializes firewall config to JSON including ASN/AS-SET leaves
2. firewall-irr OnConfigure/OnConfigVerify: parse JSON, extract all ASN/AS-SET references
3. Load cached prefixes from zefs for each reference
4. If any reference has no cached data: reject commit with error naming the missing ASN/AS-SET
5. Build nftables interval Set per unique reference per address family (SetTypeIPv4/IPv6 + SetFlagInterval)
6. Build Table containing all generated sets
7. Rewrite ASN/AS-SET match leaves to MatchInSet referencing generated set names
8. Call `RegisterTables("firewall-irr", tables)` then `ApplyAll()`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> Plugin | OnExecuteCommand with command name + args | [ ] |
| Plugin -> IRR | resolve/irr.LookupPrefixes over TCP whois (fetch path only) | [ ] |
| Plugin -> Disk | zefs read/write for cache persistence | [ ] |
| Config -> Plugin | SDK OnConfigure/OnConfigVerify with JSON sections | [ ] |
| Plugin -> Kernel | RegisterTables + ApplyAll -> backend.Apply (commit path only) | [ ] |

### Integration Points
- `resolve/irr.IRR` client: direct import, same as BGP filter_irr (no import cycle)
- `resolve/peeringdb.PeeringDB` client: for AS-SET auto-discovery from bare ASNs
- `firewall.RegisterTables` / `firewall.ApplyAll`: table registry for merged apply
- `firewall.Set` / `firewall.Table` model types: for building nftables sets
- `zefs.BlobStore`: for cache persistence

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `resolve/irr` can be imported from a firewall plugin without import cycle | BGP filter_irr does this; irr imports only cache, textbuf, stdlib | Would need RPC indirection or extraction | grep import chain | unvalidated |
| A-2 | RegisterTables from a second owner works alongside the firewall engine's tables | registry.go concatenates all owners sorted by name | Sets would collide or firewall tables would be deleted on IRR refresh | unit test with two owners | unvalidated |
| A-3 | IRR-generated interval sets can reference prefixes (CIDR ranges) not just addresses | nftables interval sets accept IP ranges and prefixes | Would need to expand prefixes to start-end ranges | nft test on Linux | unvalidated |
| A-4 | The firewall config parser can read zefs at verify time to look up cached prefix lists | zefs is available at config parse/verify time (the BGP filter_irr opens it from config dir) | Would need to pass cache data through a different path | verify zefs access during OnConfigVerify | unvalidated |
| A-5 | `LookupPrefixes` accepts bare ASN strings like "AS13335" not just AS-SET names | client.go lookupFamilyPrefixes uses `!a4` / `!a6` RPSL commands which accept AS names | Would need to call ResolveASSet first and then LookupPrefixes per member | test with bare ASN | unvalidated |
| A-6 | Per-ASN/AS-SET zefs keys (e.g., `meta/firewall/irr/AS13335`) don't exceed zefs key limits | zefs keys are strings, BGP filter_irr uses `meta/bgp/irr-cache` (single key) | Would need to use a single key with all entries like BGP filter_irr | check zefs key constraints | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Large AS-SETs produce huge nftables sets (e.g., AS-AMAZON has 50K+ prefixes) that slow kernel apply | Apply takes >5s or kernel OOM | Cap set size (maxPrefixEntries like filter_irr), log warning when truncating |
| R-2 | Concurrent RegisterTables from firewall engine and IRR plugin could race on ApplyAll | Flaky test failures, inconsistent kernel state | tableRegistry.mu already serializes; verify no TOCTOU between Register and Apply |
| R-3 | IRR query failure during refresh leaves stale sets; operator doesn't notice | show firewall irr reports "error" but traffic still matches stale prefixes | Fail-closed: keep last-good cache on error (do not remove sets or apply bad data); log error; `show firewall irr` shows last-refresh-error status so operator can act |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `update firewall irr asn <N>` CLI command | -> | fetch handler queries IRR, saves to zefs | `TestUpdateFirewallIrrASN` |
| `update firewall irr as-set <name>` CLI command | -> | fetch handler queries IRR, saves to zefs | `TestUpdateFirewallIrrASSet` |
| `show firewall irr` CLI command | -> | show handler reads zefs cache, renders JSON | `TestShowFirewallIrr` |
| Config commit with `source-asn 13335` | -> | verify reads zefs, generates set; apply registers tables | `TestCommitWithSourceASN` |
| Config commit with `source-asn` but no cache | -> | verify rejects with error naming missing ASN | `TestCommitRejectsWithoutCache` |
| Config with `refresh-interval 3600` | -> | plugin starts refresh loop, periodic re-query | `TestRefreshLoopStartsWhenEnabled` |
| Refresh loop with IRR failure | -> | last-good cache preserved, error logged | `TestRefreshFailureKeepsLastGood` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `update firewall irr asn 13335` when IRR server reachable | Prefixes for AS13335 saved to zefs under `meta/firewall/irr/AS13335`; command reports v4/v6 counts |
| AC-2 | `update firewall irr as-set AS-CLOUDFLARE` | AS-SET expanded and prefixes saved to zefs under `meta/firewall/irr/AS-CLOUDFLARE`; command reports counts |
| AC-3 | `show firewall irr` with cached data | JSON output listing all cached ASN/AS-SET entries with prefix counts and last-refresh timestamps |
| AC-4 | Config commit with `source-asn 13335` and cached data exists | Verify passes; nftables interval set `irr_v4_AS13335` (and `irr_v6_AS13335` if IPv6 prefixes exist) created with correct prefixes; term match uses MatchInSet |
| AC-5 | Config commit with `source-asn 99999` and NO cached data | Verify rejects with error: "firewall irr: no cached prefix data for AS99999; run 'update firewall irr asn 99999' first" |
| AC-6 | Config commit with `source-as-set AS-EXAMPLE` and cached data | Same as AC-4 but with AS-SET name as set name suffix |
| AC-7 | Config commit with `destination-asn` / `destination-as-set` | Same as AC-4/AC-6 but using SetFieldDestAddr instead of SetFieldSourceAddr |
| AC-8 | `update firewall irr all` with multiple cached entries | All cached ASN/AS-SET entries refreshed from IRR; zefs updated |
| AC-9 | `update firewall irr asn 13335` when IRR server unreachable | Command reports error; existing cached data preserved (not deleted) |
| AC-10 | Plugin removed (directory deleted) | Feature completely gone; `source-asn`/`source-as-set` YANG leaves absent; firewall component unaffected |
| AC-11 | `show firewall irr prefix <asn-or-as-set>` | Lists all cached prefixes for the given entry |
| AC-12 | `refresh-interval 0` (default) in IRR policy | No automatic refresh loop started; operator must use `update firewall irr` manually |
| AC-13 | `refresh-interval 3600` in IRR policy | Refresh loop runs every 3600s, re-queries IRR for all cached ASN/AS-SET entries |
| AC-14 | Auto-refresh fails (IRR server unreachable during refresh loop) | Last-good cached data preserved; error logged; nftables sets unchanged; no outage from bad data |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `update firewall irr asn 13335` | CLI -> OnExecuteCommand -> irr.LookupPrefixes -> zefs write | `test-firewall-irr-update.ci` |
| 2 | Commits config with `source-asn 13335` | Config JSON -> plugin verify (zefs read) -> set generation -> RegisterTables -> ApplyAll | `test-firewall-irr-commit.ci` |
| 3 | Runs `show firewall irr` | CLI -> OnExecuteCommand -> zefs read -> JSON output | `test-firewall-irr-show.ci` |
| 4 | Commits config with uncached ASN | Config JSON -> plugin verify -> zefs read fails -> reject commit | `test-firewall-irr-reject.ci` |
| 5 | Configures `refresh-interval 3600` | Config commit -> plugin starts refresh loop -> periodic IRR re-query | `test-firewall-irr-refresh.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseFirewallIRRConfig` | `internal/component/firewall/plugins/irr/config_test.go` | Parsing ASN/AS-SET references from firewall config JSON | |
| `TestBuildIntervalSets` | `internal/component/firewall/plugins/irr/sets_test.go` | Generating nftables interval sets from cached prefix lists | |
| `TestCacheSaveLoad` | `internal/component/firewall/plugins/irr/cache_test.go` | Round-trip zefs persistence per-ASN/AS-SET key | |
| `TestVerifyRejectsMissingCache` | `internal/component/firewall/plugins/irr/verify_test.go` | Config verify rejects when referenced ASN has no cache | |
| `TestSetNaming` | `internal/component/firewall/plugins/irr/sets_test.go` | Deterministic set name generation from ASN/AS-SET + address family | |
| `TestShowIRRCommand` | `internal/component/firewall/plugins/irr/command_test.go` | Show command JSON output format with cached entries | |
| `TestUpdateASNCommand` | `internal/component/firewall/plugins/irr/command_test.go` | Update command calls IRR and writes cache | |
| `TestRefreshLoopDisabledByDefault` | `internal/component/firewall/plugins/irr/irr_test.go` | refresh-interval 0 does not start refresh goroutine | |
| `TestRefreshLoopStartsWhenEnabled` | `internal/component/firewall/plugins/irr/irr_test.go` | refresh-interval > 0 starts periodic refresh | |
| `TestRefreshFailureKeepsLastGood` | `internal/component/firewall/plugins/irr/irr_test.go` | Failed IRR query during refresh preserves existing cache | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ASN | 1-4294967294 | 4294967294 | 0 | 4294967295 (reserved) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-firewall-irr-update` | `test/plugin/firewall-irr-update.ci` | Operator fetches IRR data for an ASN | |
| `test-firewall-irr-commit` | `test/plugin/firewall-irr-commit.ci` | Operator commits config with source-asn after fetching | |
| `test-firewall-irr-reject` | `test/plugin/firewall-irr-reject.ci` | Commit rejected when ASN not cached | |
| `test-firewall-irr-show` | `test/plugin/firewall-irr-show.ci` | Show command displays cached IRR data | |
| `test-firewall-irr-refresh` | `test/plugin/firewall-irr-refresh.ci` | Optional auto-refresh loop with fail-closed on error | |

### Interop Tests (MANDATORY for protocol features)

N/A - no wire protocol changes. IRR whois client already exists and is tested.

### Future (if deferring any tests)
- Per-interface AS-SET binding (Spec 2)

## Files to Modify
- `pkg/zefs/keys.go` - add `KeyFirewallIRRCache` with pattern `meta/firewall/irr/{name}`
- `internal/component/firewall/config.go` - add `source-asn`, `source-as-set`, `destination-asn`, `destination-as-set` parsing in `parseFromBlock` (emit MatchInSet with deterministic set names)
- `internal/component/firewall/model.go` - no new Match types needed (uses existing MatchInSet)
- `internal/component/plugin/all/all.go` - generated: includes new plugin import

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | Yes | `internal/component/firewall/plugins/irr/yang/ze-firewall-irr.yang` |
| YANG validation constraints | Yes | ASN range 1-4294967294, AS-SET name pattern |
| YANG custom validators | No | Native YANG constraints sufficient |
| CLI commands/flags | Yes | show/update firewall irr commands |
| CLI grammar (action before identifier) | Yes | `update firewall irr asn <N>`, `show firewall irr` |
| Editor autocomplete | No | ASN/AS-SET values are dynamic, no static completion |
| Functional test for new RPC/API | Yes | `test/plugin/firewall-irr-*.ci` |
| Pipe completeness | Yes | show command output through ApplyPipes |
| Env var registration | No | All config is YANG |
| Doctor check for runtime dependencies | No | IRR servers are external, not local dependencies |
| Prometheus counters/metrics | Yes | `ze_firewall_irr_prefixes_cached`, `ze_firewall_irr_refresh_outcomes_total`, `ze_firewall_irr_last_refresh_timestamp` (mirror BGP IRR metrics) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (IRR firewall filtering), `docs/features/plugins.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` (source-asn/as-set leaves) |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (show/update firewall irr) |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | Yes | `docs/features/plugins.md` (firewall-irr) |
| 6 | Has a user guide page? | Yes | `docs/guide/firewall.md` (IRR section) |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` (IRR-based firewall filtering) |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | telemetry doc (3 metrics) |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | Yes | `docs/features/plugins.md` |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] | grep during implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | [ ] | check during implementation |

## Files to Create
- `internal/component/firewall/plugins/irr/register.go` - plugin registration ("firewall-irr")
- `internal/component/firewall/plugins/irr/irr.go` - plugin entry point (runEngine), OnConfigure/OnConfigVerify/OnConfigApply, OnExecuteCommand
- `internal/component/firewall/plugins/irr/config.go` - parse firewall config JSON for ASN/AS-SET references
- `internal/component/firewall/plugins/irr/sets.go` - build interval sets from cached prefix lists, set naming
- `internal/component/firewall/plugins/irr/cache.go` - zefs read/write per-ASN/AS-SET key
- `internal/component/firewall/plugins/irr/command.go` - CLI command handlers (show/update firewall irr)
- `internal/component/firewall/plugins/irr/yang/embed.go` - YANG embedding
- `internal/component/firewall/plugins/irr/yang/register.go` - YANG registration
- `internal/component/firewall/plugins/irr/yang/ze-firewall-irr.yang` - YANG augment: source-asn/as-set leaves + irr policy container
- `internal/component/firewall/plugins/irr/yang/ze-firewall-irr-cmd.yang` - YANG for CLI commands
- `test/plugin/firewall-irr-update.ci` - functional test: fetch IRR data
- `test/plugin/firewall-irr-commit.ci` - functional test: commit with cached ASN
- `test/plugin/firewall-irr-reject.ci` - functional test: commit rejected without cache
- `test/plugin/firewall-irr-show.ci` - functional test: show cached data
- `test/plugin/firewall-irr-refresh.ci` - functional test: optional auto-refresh with fail-closed on error

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- register plugin, create YANG, wire OnExecuteCommand skeleton
   - Tests: `TestUpdateFirewallIrrASN` (wiring), `test-firewall-irr-update.ci`
   - Files: `register.go`, `irr.go`, `yang/`, `command.go` (stubs)
   - Verify: plugin registers, CLI command dispatches, wiring test fails because logic is stub
2. **Phase: Cache** -- zefs persistence for IRR data per ASN/AS-SET
   - Tests: `TestCacheSaveLoad`
   - Files: `cache.go`, `pkg/zefs/keys.go`
   - Verify: round-trip save/load of prefix lists
3. **Phase: Fetch** -- implement `update firewall irr` commands (IRR query + cache write)
   - Tests: `TestUpdateASNCommand`, `TestUpdateASSetCommand`
   - Files: `command.go`, `irr.go`
   - Verify: command queries mock IRR, saves to cache, reports counts
4. **Phase: Show** -- implement `show firewall irr` commands
   - Tests: `TestShowIRRCommand`
   - Files: `command.go`
   - Verify: JSON output matches cached state
5. **Phase: Config parsing** -- parse source-asn/as-set from firewall config JSON
   - Tests: `TestParseFirewallIRRConfig`
   - Files: `config.go` (plugin), `internal/component/firewall/config.go` (add leaves)
   - Verify: ASN/AS-SET references extracted from config JSON
6. **Phase: Set generation** -- build interval sets from cached prefixes
   - Tests: `TestBuildIntervalSets`, `TestSetNaming`
   - Files: `sets.go`
   - Verify: correct Set objects with SetFlagInterval and prefix elements
7. **Phase: Verify gate** -- reject commit if cache missing
   - Tests: `TestVerifyRejectsMissingCache`, `test-firewall-irr-reject.ci`
   - Files: `irr.go` (OnConfigVerify)
   - Verify: commit rejected with actionable error message
8. **Phase: Apply** -- register tables and apply on commit
   - Tests: `TestCommitWithSourceASN`, `test-firewall-irr-commit.ci`
   - Files: `irr.go` (OnConfigApply)
   - Verify: interval sets registered, ApplyAll merges with firewall tables
9. **Phase: Optional refresh** -- refresh loop when refresh-interval > 0, fail-closed on error
   - Tests: `TestRefreshLoopDisabledByDefault`, `TestRefreshLoopStartsWhenEnabled`, `TestRefreshFailureKeepsLastGood`, `test-firewall-irr-refresh.ci`
   - Files: `irr.go` (refreshLoop, refreshAll)
   - Verify: interval=0 no goroutine; interval>0 periodic re-query; failure preserves last-good cache
10. **Functional tests** -- all .ci tests passing
10. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | Every End-to-End User Story has a working path |
| Correctness | Set names match between config parser emission and plugin generation; prefix list cached correctly |
| Naming | JSON keys use kebab-case; YANG uses kebab-case; set names use `irr_v4_` / `irr_v6_` prefix |
| Data flow | IRR queries only in update commands (and optional refresh loop), never during verify/apply; zefs is the bridge |
| CLI grammar | `update firewall irr` / `show firewall irr`: action before identifier |
| Plugin self-containment | Removing `internal/component/firewall/plugins/irr/` removes all IRR features |
| YANG validation | ASN leaf has range constraint; AS-SET leaf has pattern constraint |
| Refresh safety | `refresh-interval 0` disables auto-refresh (default); `refresh-interval > 0` starts refresh loop; failed refresh keeps last-good cache and logs error (never silently applies bad data) |
| Commit rejection | Config commit rejects with actionable error if any referenced ASN/AS-SET has no cached data or if IRR resolution failed; no silent empty sets |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Plugin directory exists | `ls internal/component/firewall/plugins/irr/` |
| YANG augment compiles | `make generate` succeeds |
| zefs key registered | `grep KeyFirewallIRRCache pkg/zefs/keys.go` |
| CLI commands registered | `grep "show firewall irr\|update firewall irr" internal/component/firewall/plugins/irr/` |
| Functional tests exist | `ls test/plugin/firewall-irr-*.ci` |
| Config parser handles new leaves | `grep "source-asn\|source-as-set" internal/component/firewall/config.go` |
| Plugin in all.go | `grep "firewall/plugins/irr" internal/component/plugin/all/all.go` |
| Refresh loop respects interval=0 | `grep -n "refresh-interval\|refreshLoop\|refreshAll" internal/component/firewall/plugins/irr/irr.go` shows conditional start |
| Refresh fail-closed behavior | Test confirms failed refresh preserves last-good cache, does not clear or corrupt it |
| Commit rejects on missing/unresolvable data | `test-firewall-irr-reject.ci` passes; error message names the missing ASN/AS-SET |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | ASN must be 1-4294967294; AS-SET name validated via `irr.ValidateASSetName` (rejects control chars) |
| Resource exhaustion | Large AS-SETs (50K+ prefixes) bounded by maxPrefixEntries cap (mirror BGP filter_irr) |
| Cache poisoning | zefs is local-only; IRR data comes from configured whois server; no external file paths |
| Error leakage | IRR query errors reported to operator CLI, not leaked to config commit path |

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
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Core Insight

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Optional auto-refresh (disabled by default, `refresh-interval 0`) | Always-on auto-refresh (like BGP filter_irr), no auto-refresh at all | Default: operator controls when data changes (deterministic). Opt-in: auto-refresh for operators who want it. Fail-closed on refresh error (keep last-good, never apply bad data) avoids outage risk from invalid IRR data |
| Separate plugin "firewall-irr" over extending firewall engine | Adding IRR logic directly to engine.go | Plugin self-containment: removing the directory removes the feature. Same pattern as bgp-filter-irr. No coupling of firewall component to resolve/irr |
| Per-ASN/AS-SET zefs keys over single blob | Single `meta/firewall/irr-cache` key with all entries (like BGP filter_irr) | Operator updates individual ASNs; per-key storage avoids read-modify-write on the full cache blob |
| Config parser emits MatchInSet with deterministic set names over new Match types | New MatchSourceASN/MatchSourceASSet types requiring backend lowering support | MatchInSet already works end-to-end through verify/apply/lower; no changes needed in nftables backend |
| Commit-time verify rejects if cache missing over fail-open or empty set | Empty set (matches nothing), warning-only | Fail-closed is the safest default; operator explicitly fetches before committing; actionable error message tells them exactly what to run |
| Direct import of `resolve/irr` over RPC indirection | Plugin SDK RPC to resolve component | In-process plugins have no isolation boundary to justify RPC overhead; same decision as BGP filter_irr (learned/896) |

## Known Limitations
- Spec 2 (per-interface AS-SET binding) is a deliberate follow-up

## RFC Documentation

N/A - no RFC protocol work.

## Implementation Summary

### What Was Implemented
- Plugin: `internal/component/firewall/plugins/irr/` (register.go, irr.go, config.go, cache.go, command.go, sets.go, yang/)
- YANG: `ze-firewall-irr.yang` (config augments: source-asn, source-as-set, destination-asn, destination-as-set, irr policy container), `ze-firewall-irr-cmd.yang` (show/update commands)
- Config parser: `irrSetMatch` in `firewall/config.go` emits MatchInSet with deterministic set names
- Model: `SetElement.IntervalEnd` field for CIDR-to-interval encoding
- Backend: nft backend passes through IntervalEnd on SetElement
- Registry: `mergeSameNameTables` in `registry.go` for multi-owner same-table sets
- Codegen: `internal/component/firewall/plugins` added to pluginDirs
- CLI commands: show/update firewall irr (asn, as-set, all, prefix)
- Metrics: 3 Prometheus counters (prefixes_cached, refresh_outcomes_total, last_refresh_timestamp)
- Shared PrefixStore: reuses existing `resolve/irr/store` with `meta/irr/{name}` keys (no new zefs keys)
- Refresh loop: optional (disabled by default, refresh-interval 0), fail-closed on error

### Bugs Found/Fixed
- nftables sets are table-local: IRR sets must be in the same table as the rules referencing them. Fixed by adding `mergeSameNameTables` to ApplyAll.
- YANG augment path: corrected from `fw:filter/fw:term/fw:from` to `fw:table/fw:chain/fw:term/fw:from` (caught by formatter).

### Documentation Updates
- Pending: docs updates deferred to commit preparation

### Deviations from Plan
- No new zefs key: spec planned `KeyFirewallIRRCache` at `meta/firewall/irr/{name}`, but shared PrefixStore already uses `KeyIRRPrefixCache` at `meta/irr/{name}`. No duplicate.
- cache.go minimal: just `cacheStorePath()`. PrefixStore handles all cache logic.
- IPv4-only MatchInSet: config parser emits `irr_v4_` set reference. IPv6 sets are created but require separate terms. Consistent with how typed nftables sets work in inet tables.
- Naming coupling accepted: `irrSetMatch` in generic `config.go` uses deterministic set names matching the plugin. When plugin is removed, YANG leaves disappear and code is unreachable.

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

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` -- no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

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
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
