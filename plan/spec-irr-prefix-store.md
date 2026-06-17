# Spec: irr-prefix-store

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 10/10 (implemented; verified for scope) |
| Updated | 2026-06-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/resolve/irr/client.go` - IRR whois client
4. `internal/component/bgp/plugins/filter_irr/cache.go` - current BGP-specific cache
5. `internal/component/bgp/plugins/filter_irr/filter_irr.go` - refreshASN flow (lines 302-357)
6. `internal/component/resolve/peeringdb/client.go` - PeeringDB AS-SET discovery

## Task

Extract the IRR prefix resolution and persistence into a shared store in a new
`internal/component/resolve/irr/store/` subpackage so both the BGP `filter_irr`
plugin and the upcoming `firewall-irr` plugin (spec-firewall-irr.md) use the same
cached data. Both are process-isolated SDK plugins, so they do NOT share a struct
instance: each constructs its own `PrefixStore` and they share data through the
zefs file on disk (`meta/irr/{name}`).

Today the BGP filter_irr plugin owns the full pipeline: IRR query, PeeringDB
AS-SET discovery, zefs persistence, and refresh. This spec moves the
"ASN/AS-SET -> prefix list -> zefs" layer into `resolve/irr/store` as a `PrefixStore`,
then rewires the BGP plugin to consume it. The firewall-irr spec will also
consume it.

### What moves

| Piece | From | To |
|-------|------|----|
| Per-ASN/AS-SET prefix resolution (IRR query + PeeringDB fallback) | `filter_irr/filter_irr.go` refreshASN() | `resolve/irr/store/store.go` Refresh(name) |
| zefs persistence (save/load per entry) | `filter_irr/cache.go` (single-blob `meta/bgp/irr-cache`) | `resolve/irr/store/store.go` (per-key `meta/irr/{name}`) |
| Cached entry type | `filter_irr/cache.go` cachedASN (unexported) | `resolve/irr/store/store.go` CachedEntry (exported) |

### What stays

| Piece | Where | Why |
|-------|-------|-----|
| BGP filter matching (evaluatePrefix, partitionUpdate) | `filter_irr/match.go` | BGP-specific: matches against UPDATE NLRI text |
| BGP filter lifecycle (OnFilterUpdate, OnConfigure) | `filter_irr/filter_irr.go` | Plugin-specific: SDK callbacks, per-peer state |
| Auto-refresh timer | `filter_irr/filter_irr.go` refreshLoop | Policy decision per consumer (BGP always-on, firewall opt-in) |
| CLI commands | `filter_irr/command.go` | Plugin-specific: `show/update bgp irr` |
| Metrics | `filter_irr/filter_irr.go` | Plugin-specific metric names |
| prefixListFromIRR (converts PrefixList to prefixEntry) | `filter_irr/filter_irr.go` | BGP-specific type conversion |

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugin-self-containment.md` - plugin removal test
  -> Constraint: BGP filter_irr must still pass self-containment after refactor; removing it must still remove all BGP IRR features
- [ ] `ai/rules/plugin-design.md` - plugin patterns
  -> Constraint: resolve/irr/store is a shared library subpackage, not a plugin; no registration, no SDK lifecycle
- [ ] `ai/patterns/config-option.md` - YANG leaf pattern
  -> Decision: no new YANG; PrefixStore is configured programmatically by its callers

### RFC Summaries (MUST for protocol work)

N/A - no protocol work.

**Key insights:**
- `resolve/irr` (the IRR client) is already imported by both BGP filter_irr and the resolve CLI; the new `PrefixStore` lives in a `resolve/irr/store` subpackage that imports both `resolve/irr` and `resolve/peeringdb` directly (no cycle: store is a child of irr, and irr never imports store)
- The IRR client (`IRR` struct) has in-memory caching (1h TTL via `resolve/cache`)
- The BGP filter_irr plugin adds zefs persistence on top of the in-memory cache
- PeeringDB AS-SET discovery (`pdbClient.LookupASSet`) is called when no AS-SET is configured
- The zefs key today is a single blob (`meta/bgp/irr-cache`) containing all ASNs; the new design uses per-entry keys (`meta/irr/{name}`)
- `PrefixList` type already lives in `resolve/irr` and is exported

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/resolve/irr/client.go` - IRR whois client: ResolveASSet, LookupPrefixes, PrefixList type; in-memory cache (1h TTL)
  -> Constraint: PrefixList{IPv4 []netip.Prefix, IPv6 []netip.Prefix} is the shared currency type
  -> Constraint: LookupPrefixes accepts AS-SET names ("AS-CLOUDFLARE") and bare ASN references ("AS13335")
- [ ] `internal/component/bgp/plugins/filter_irr/cache.go` - zefs persistence: cachedASN{ASN, ASSet, IPv4, IPv6 []string} serialized as JSON; single blob under `meta/bgp/irr-cache`; openStore reads database.zefs from config dir
  -> Constraint: cache stores prefix strings, not netip.Prefix; conversion happens at load time
  -> Constraint: openStore uses env `ze.config.dir` -> `paths.DefaultConfigDir()` -> `database.zefs`
- [ ] `internal/component/bgp/plugins/filter_irr/filter_irr.go` - refreshASN(): if no AS-SET, discover via PeeringDB; then call irrClient.LookupPrefixes; build prefixEntry list; update asnState; save cache
  -> Constraint: PeeringDB fallback is part of the resolution flow, not a separate step
  -> Constraint: maxPrefixEntries (500K) caps the total across all families
- [ ] `internal/component/resolve/peeringdb/client.go` - PeeringDB client: LookupASSet(ctx, asn) returns []string (AS-SET names)
  -> Constraint: PeeringDB client already lives in resolve/; PrefixStore can import it directly
- [ ] `internal/component/resolve/resolvers.go` - Resolvers struct holds DNS, Cymru, PeeringDB, IRR instances; created at hub startup (cmd/ze/hub/main_system.go:91)
  -> Decision: Resolvers is NOT modified. The BGP filter_irr plugin is a process-isolated SDK plugin (filter_irr.go:127-128 builds its own irr.IRR and peeringdb.PeeringDB from JSON config); it cannot receive the hub's in-process Resolvers. Each consumer constructs its own PrefixStore. Cross-consumer sharing is via the zefs file, not a shared struct. A Resolvers field is out of scope (revisit only if a future in-hub consumer needs it).
- [ ] `pkg/zefs/keys.go` - KeyIRRCache pattern `meta/bgp/irr-cache`; MustRegister with KeyEntry{Pattern, Description}
  -> Constraint: new key `meta/irr/{name}` replaces old `meta/bgp/irr-cache`; migration needed for existing data

**Behavior to preserve:**
- BGP filter_irr plugin continues to work identically from the operator's perspective
- `show bgp irr` / `update bgp irr` CLI commands unchanged
- Auto-refresh timer in BGP filter_irr continues to work
- zefs data survives the migration (old format readable, new format written)
- Existing in-memory cache in IRR client (1h TTL) continues to work independently

**Behavior to change:**
- BGP filter_irr's cache.go replaced by calls to shared PrefixStore
- zefs key changes from `meta/bgp/irr-cache` (single blob) to `meta/irr/{name}` (per entry)
- One-time migration: old blob read and split into per-entry keys on first load
- PeeringDB AS-SET discovery moved from BGP plugin to PrefixStore.Refresh()

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Consumer calls `store.Refresh(ctx, "AS13335")` or `store.Refresh(ctx, "AS-CLOUDFLARE")`
- Consumer calls `store.Get("AS13335")` to read cached data without network

### Transformation Path
1. `store.Refresh(name)`: determine if name is ASN or AS-SET
2. If bare ASN and no AS-SET known: call `pdbClient.LookupASSet(ctx, asn)` for discovery
3. Call `irrClient.LookupPrefixes(ctx, name)` -> `irr.PrefixList{IPv4, IPv6}`
4. Build `CachedEntry{Name, ASSet, IPv4, IPv6, RefreshedAt}` from result
5. Serialize to JSON, write to zefs under `meta/irr/{name}`
6. Update in-memory map
7. Consumer reads via `store.Get(name)` -> `*CachedEntry`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Store -> IRR | irr.LookupPrefixes over TCP whois | [ ] |
| Store -> PeeringDB | peeringdb.LookupASSet over HTTPS | [ ] |
| Store -> Disk | zefs read/write per key | [ ] |
| Consumer -> Store | Go function calls (in-process) | [ ] |

### Integration Points
- `resolve/irr.IRR` client: LookupPrefixes (store subpackage imports resolve/irr)
- `resolve/peeringdb.PeeringDB` client: LookupASSet for AS-SET discovery
- `zefs.BlobStore`: persistence
- BGP `filter_irr` plugin: rewired to call PrefixStore instead of managing its own cache
- Future: firewall-irr plugin (spec-firewall-irr.md)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | PrefixStore (in `resolve/irr/store` subpackage) imports peeringdb without a cycle | peeringdb imports resolve/irr; store is a child package of irr, and irr never imports store, so store->peeringdb->irr is acyclic | Fall back to an ASSetLooker interface in package irr | grep: peeringdb/client.go:22 imports irr; irr does not import store | validated |
| A-5 | zefs keys tolerate ':' in a segment (AS-SET names like `AS3333:AS-FOO`, client.go:276) | zefs is a single-file blob store; keys are internal tree segments split only on '/'; decode validates via fs.ValidPath which permits ':' | Sanitize/encode the name before using it as a key | TestPrefixStoreColonKey (write+read `AS-FOO:AS-BAR`); store.go validateName also rejects '..' which would panic Key() | validated |
| A-2 | Per-entry zefs keys work without key count limits | zefs is a single-file blob store with internal tree keys; count is bounded by the format (maxEntryCount), not the filesystem | Would need to revert to single-blob approach | TestPrefixStoreConcurrentPersist exercises multiple per-entry keys; zefs maxEntryCount far exceeds any IRR fleet | confirmed |
| A-3 | Old `meta/bgp/irr-cache` data can be migrated to new per-entry keys | Old format is JSON array of {asn, as-set, ipv4, ipv6}; straightforward to split | Would need manual migration or fresh fetch | TestMigrateOldCache | confirmed |
| A-4 | PrefixStore does not need its own YANG config; callers pass config values | IRR server and PeeringDB URL are caller concerns (BGP has its own YANG for these) | Would need shared YANG or env vars | store.New takes clients + path; no YANG added; BGP builds it in handleConfigure (filter_irr.go:127) | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Import cycle | Compilation fails | Resolved by placing PrefixStore in the `resolve/irr/store` subpackage (store->peeringdb->irr is acyclic). No interface needed. Fallback if a cycle still appears: ASSetLooker interface in package irr. |
| R-2 | Migration from old single-blob to per-entry keys loses data on error | Old cache disappears, fresh fetch needed | Read old blob first, write new keys, only delete old key after all new keys written |
| R-3 | BGP filter_irr regression after rewiring | Existing functional tests fail | Run `test/plugin/filter-irr*.ci` after every change |
| R-4 | Cross-process concurrency: BGP and firewall plugins both read/write `meta/irr/{name}` in one database.zefs | Stale reads / lost writes between processes | In-process refreshes serialized by PrefixStore fileMu (no per-key loss; regression `TestPrefixStoreConcurrentPersist`). zefs `Lock` is IN-PROCESS only, NOT a file lock -- two writer PROCESSES clobber each other on flush, so a single writer process per store file is required until zefs gains an flock (firewall-irr prerequisite). Each consumer applies only entries it enrolled. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `store.Refresh(ctx, "AS13335")` | -> | IRR query + zefs write | `TestPrefixStoreRefresh` |
| `store.Get("AS13335")` | -> | In-memory + zefs read | `TestPrefixStoreGet` |
| `store.RefreshAll(ctx)` | -> | DESCOPED to spec-firewall-irr (Review Gate Run 2) | n/a |
| BGP filter_irr refreshASN | -> | Calls store.Refresh | `TestFilterIRRUsesStore` |
| BGP `update bgp irr asn` | -> | Calls store.Refresh | existing `test/plugin/filter-irr-update.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `store.Refresh(ctx, "AS13335")` with reachable IRR | Prefixes saved to zefs under `meta/irr/AS13335`; in-memory cache updated; returns CachedEntry with v4/v6 counts |
| AC-2 | `store.Refresh(ctx, "AS-CLOUDFLARE")` with reachable IRR | AS-SET prefixes saved under `meta/irr/AS-CLOUDFLARE` |
| AC-3 | `store.Refresh(ctx, "AS13335")` with no AS-SET configured and PeeringDB available | Discovers AS-SET via PeeringDB, uses it for lookup, saves result |
| AC-4 | `store.Get("AS13335")` after Refresh | Returns CachedEntry with prefixes |
| AC-5 | `store.Get("AS99999")` with no cache | Returns nil |
| AC-6 | `store.RefreshAll(ctx)` with 3 cached entries | DESCOPED (user-approved, /ze-review Run 2): RefreshAll has no in-tree consumer; moved to spec-firewall-irr |
| AC-7 | `store.Refresh(ctx, "AS13335")` with unreachable IRR | Returns error; existing cached data preserved |
| AC-8 | BGP filter_irr `update bgp irr asn` after rewiring | Same behavior as before; existing functional tests pass |
| AC-9 | BGP filter_irr `show bgp irr` after rewiring | Same JSON output as before |
| AC-10 | First startup with old `meta/bgp/irr-cache` blob | Migrated to per-entry keys; old key removed after successful migration |
| AC-11 | `store.List()` | DESCOPED (user-approved, /ze-review Run 2): List has no in-tree consumer; moved to spec-firewall-irr |
| AC-12 | `store.Refresh(ctx, "AS13335")` with no AS-SET and PeeringDB returns empty or errors | Falls back to querying IRR with the literal "AS13335" name (preserves filter_irr.go:317-329 behavior) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `update bgp irr asn 13335` | CLI -> filter_irr command -> store.Refresh -> IRR query -> zefs | existing `test/plugin/filter-irr-update.ci` still passes |
| 2 | Runs `show bgp irr` | CLI -> filter_irr command -> in-memory byASN (seeded via store.Get at configure) -> JSON output | existing `test/plugin/filter-irr.ci` still passes |
| 3 | Boots with old single-blob cache | Plugin startup -> store.Open -> migration -> per-entry keys | `TestMigrateOldCache` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPrefixStoreRefresh` | `internal/component/resolve/irr/store/store_test.go` | Refresh queries IRR, saves to zefs, updates in-memory | |
| `TestPrefixStoreGet` | `internal/component/resolve/irr/store/store_test.go` | Get returns cached entry or nil | |
| `TestPrefixStoreRefreshAll` | (removed) | RefreshAll refreshes all entries | DESCOPED to firewall-irr (Run 2) |
| `TestPrefixStoreList` | (removed) | List returns all cached entry metadata | DESCOPED to firewall-irr (Run 2) |
| `TestPrefixStorePersistence` | `internal/component/resolve/irr/store/store_test.go` | Save to zefs, create new store, load from zefs | |
| `TestPrefixStoreRefreshError` | `internal/component/resolve/irr/store/store_test.go` | Failed refresh preserves existing cache (use a fresh client: LookupPrefixes serves a 1h in-memory cache, so a warm cache would mask the error) | |
| `TestPrefixStorePeeringDBFallback` | `internal/component/resolve/irr/store/store_test.go` | Bare ASN triggers PeeringDB AS-SET discovery | |
| `TestMigrateOldCache` | `internal/component/resolve/irr/store/store_test.go` | Old single-blob format migrated to per-entry keys | |
| `TestFilterIRRUsesStore` | `internal/component/bgp/plugins/filter_irr/filter_irr_test.go` | BGP plugin uses PrefixStore for refresh | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ASN (in name) | 1-4294967294 | 4294967294 | 0 | 4294967295 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing `filter-irr` | `test/plugin/filter-irr.ci` | BGP IRR filter still works after rewiring | |
| existing `filter-irr-update` | `test/plugin/filter-irr-update.ci` | BGP IRR update command still works | |

### Interop Tests (MANDATORY for protocol features)

N/A - no wire protocol changes.

### Future (if deferring any tests)
- firewall-irr consumer tests (spec-firewall-irr.md)

## Files to Modify
- `internal/component/bgp/plugins/filter_irr/filter_irr.go` - replace direct IRR/PeeringDB calls with PrefixStore; remove refreshASN resolution logic (keep refresh timer, keep prefixListFromIRR conversion, keep per-peer state); construct the store in handleConfigure next to NewIRR/NewPeeringDB (filter_irr.go:127-128); keep the enrollment gate so the plugin applies only its configured ASNs, never store.List() wholesale (preserves cache.go:127-134 semantics)
- `internal/component/bgp/plugins/filter_irr/cache.go` - replace with thin wrapper calling PrefixStore; remove openStore, saveCacheTo, loadCacheFrom, buildCacheEntries, applyCacheData
- `internal/component/bgp/plugins/filter_irr/cache_test.go` - update to test via PrefixStore
- `internal/component/bgp/plugins/filter_irr/command.go` - update bgp irr commands to use PrefixStore for refresh
- `pkg/zefs/keys.go` - add `KeyIRRPrefixCache` with pattern `meta/irr/{name}`; keep old `KeyIRRCache` for migration

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | PrefixStore is configured programmatically |
| YANG validation constraints | No | |
| YANG custom validators | No | |
| CLI commands/flags | No | Existing commands rewired |
| CLI grammar (action before identifier) | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | No | Existing tests cover |
| Pipe completeness | No | |
| Env var registration | No | |
| Doctor check for runtime dependencies | No | |
| Prometheus counters/metrics | No | Metrics stay in consumer plugins |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Internal refactor |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | Internal refactor of existing plugin |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/resolve.md` if it exists; add PrefixStore section |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] | grep during implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Files to Create
- `internal/component/resolve/irr/store/store.go` - PrefixStore struct: Refresh, Get, Open (with migration); RefreshAll/List descoped to firewall-irr (Run 2)
- `internal/component/resolve/irr/store/store_test.go` - unit tests for PrefixStore

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

1. **Phase: Wiring (MANDATORY FIRST)** -- create PrefixStore with stub API
   - Tests: `TestPrefixStoreRefresh`, `TestPrefixStoreGet` (failing stubs)
   - Files: `internal/component/resolve/irr/store/store.go`
   - Verify: PrefixStore compiles, stub methods exist, tests fail
2. **Phase: Core store** -- implement Refresh, Get, List with in-memory map
   - Tests: `TestPrefixStoreRefresh`, `TestPrefixStoreGet`, `TestPrefixStoreList`
   - Files: `store.go`
   - Verify: store resolves via IRR client, caches in memory
3. **Phase: PeeringDB fallback** -- add AS-SET discovery for bare ASNs
   - Tests: `TestPrefixStorePeeringDBFallback`
   - Files: `store.go`
   - Verify: bare ASN triggers PeeringDB lookup before IRR query
4. **Phase: Persistence** -- zefs save/load per-entry keys
   - Tests: `TestPrefixStorePersistence`
   - Files: `store.go`, `pkg/zefs/keys.go`
   - Verify: data survives store close/reopen
5. **Phase: Migration** -- read old `meta/bgp/irr-cache` blob (ALL entries, config-independent), split into per-entry keys (key = "AS"+asn, preserve the ASSet field)
   - Tests: `TestMigrateOldCache`
   - Files: `store.go`
   - Verify: old format migrated; old key removed after success
6. **Phase: Error handling** -- failed refresh preserves cache
   - Tests: `TestPrefixStoreRefreshError`
   - Files: `store.go`
   - Verify: unreachable IRR returns error, cached data untouched
7. **Phase: RefreshAll** -- refresh all cached entries (consider batching zefs writes under one Lock to avoid N lock/write/release cycles)
   - Tests: `TestPrefixStoreRefreshAll`
   - Files: `store.go`
   - Verify: all entries refreshed
8. **Phase: Rewire BGP filter_irr** -- replace direct IRR/PeeringDB/cache calls with PrefixStore
   - Tests: `TestFilterIRRUsesStore`, existing functional tests
   - Files: `filter_irr.go`, `cache.go`, `command.go`
   - Verify: all existing `test/plugin/filter-irr*.ci` pass; `show bgp irr` output unchanged
9. **Functional tests** -- all existing .ci tests passing
10. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | Every End-to-End User Story has a working path |
| Correctness | BGP filter_irr behavior identical before and after; no data loss in migration |
| Naming | CachedEntry fields use Go conventions; zefs key pattern uses kebab-case path |
| Data flow | Resolution (IRR + PeeringDB) in PrefixStore only; BGP plugin does not call IRR client directly |
| No regression | All existing `test/plugin/filter-irr*.ci` pass without modification |
| Import cycle | `go build ./...` clean; store subpackage `resolve/irr/store` imports peeringdb directly; package `resolve/irr` does NOT import the store |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `store.go` exists | `ls internal/component/resolve/irr/store/store.go` |
| `store_test.go` exists | `ls internal/component/resolve/irr/store/store_test.go` |
| zefs key registered | `grep KeyIRRPrefixCache pkg/zefs/keys.go` |
| BGP filter_irr uses PrefixStore | `grep PrefixStore internal/component/bgp/plugins/filter_irr/` |
| Old cache code removed | `grep -c openStore internal/component/bgp/plugins/filter_irr/cache.go` returns 0 |
| Existing tests pass | `make ze-functional-test` includes filter-irr tests |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Entry names validated via irr.ValidateASSetName (rejects control chars, injection) |
| Resource exhaustion | maxPrefixEntries cap preserved from BGP filter_irr (500K) |
| Cache integrity | Failed refresh never overwrites good data; migration is atomic (write new before delete old) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Import cycle | Verify store is in the `resolve/irr/store` subpackage (not package irr); fallback R-1: ASSetLooker interface |
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

The IRR prefix resolution pipeline (query + PeeringDB fallback + persistence) is a
reusable service, not a plugin concern. Moving it to a `resolve/irr/store` subpackage
follows the same pattern as the IRR client itself: shared infrastructure consumed by
multiple plugins, sharing cached data through the zefs file since the consumers are
process-isolated plugins.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| PrefixStore in `resolve/irr/store` subpackage over same package as IRR client | Same package `resolve/irr`; `resolve/irrstore/` sibling | The store needs only exported symbols (irr.PrefixList, irr.LookupPrefixes, irr.ValidateASSetName, peeringdb.LookupASSet), so there is no export ceremony to avoid. Same-package would force a peeringdb import cycle; the subpackage is acyclic and cleanly separates persistence/orchestration from the whois client. (The earlier "uses unexported aggregateAndSort" rationale was wrong: LookupPrefixes already aggregates internally, client.go:193-194.) |
| Per-entry zefs keys (`meta/irr/{name}`) over single blob | Keep single blob like BGP filter_irr | Per-entry allows independent update of individual ASNs; both BGP and firewall consumers benefit; avoids read-modify-write on large blob |
| Import resolve/peeringdb directly from the store subpackage; no interface | ASSetLooker interface in package irr | The subpackage breaks the cycle structurally, so no interface is needed. A single-implementation interface here would be premature abstraction. The store takes a concrete `*peeringdb.PeeringDB` (nil = PeeringDB disabled), testable via httptest as peeringdb/client_test.go already does. |
| Each consumer constructs its own PrefixStore; sharing is via the zefs file | One PrefixStore as a shared `resolve.Resolvers` field | filter_irr is a process-isolated SDK plugin (filter_irr.go:127-128 builds its own clients from config); it cannot reach the hub's Resolvers. Shared state lives on disk under `meta/irr/{name}`, accessed under store.Lock(). |
| One-time migration from old blob to per-entry keys over breaking change | Delete old key, require fresh fetch | Operators have cached data; forcing re-fetch is disruptive and unnecessary |
| PrefixStore does not own refresh timer; callers manage their own | Store owns a shared timer | BGP wants always-on refresh; firewall wants opt-in; forcing one policy doesn't fit both |

## Known Limitations
- PrefixStore does not own the refresh timer; each consumer manages its own refresh policy
- No YANG config for the store itself; callers pass IRR server and PeeringDB URL
- Migration only handles the current `meta/bgp/irr-cache` format
- Consumers share cached data through the zefs file, not a shared in-process struct. In-process writes are serialized by the PrefixStore fileMu; cross-process writes are NOT safe (zefs `Lock` is in-process only), so exactly one process may write a given store file until zefs gains a file lock (R-4; firewall-irr prerequisite)
- The shared store may hold entries a given consumer never enrolled (AS-SETs, the other plugin's ASNs); each consumer applies only entries it owns, never store.List() wholesale (preserves cache.go:127-134)

## RFC Documentation

N/A - no RFC protocol work.

## Implementation Summary

### What Was Implemented
- New `resolve/irr/store` subpackage: `PrefixStore` with `New`, `Open` (one-time legacy migration), `Refresh`, `Get`; exported `CachedEntry` persisted as JSON under `meta/irr/{name}`. (`List`/`RefreshAll` were built then removed in /ze-review -- descoped to firewall-irr, see Review Gate Run 2.)
- Resolution pipeline moved into the store: PeeringDB AS-SET discovery for bare ASNs (fallback to literal `AS<asn>`), IRR `LookupPrefixes`, zefs persistence (per-entry keys, batched writes under one lock).
- `pkg/zefs/keys.go`: registered `KeyIRRPrefixCache` (`meta/irr/{name}`); kept `KeyIRRCache` for migration.
- Rewired BGP `filter_irr`: `refreshASN` delegates to `store.Refresh`; `cache.go` reduced to `cacheStorePath` + enrollment-gated `loadFromStore` (removed openStore/saveCache/loadCache/applyCacheData/buildCacheEntries/cachedASN); plugin now holds `*store.PrefixStore` instead of its own irr/pdb clients.
- Tests: 9 store unit tests + `TestParseBareASNBoundaries`; filter_irr `TestFilterIRRUsesStore`, `TestLoadFromStoreEnrolledOnly`/`MissingFile`, `TestExtractASNFromFilterBoundaries`; updated `TestUpdateASNFailurePreservesState`.
- Docs: `docs/architecture/resolve.md` PrefixStore section + dependency/source-anchor updates.

### Bugs Found/Fixed
- Caught a self-inflicted regression during functional testing: the first `Open` took a zefs *write* lock on the shared `database.zefs` on every configure (old loadCache did lock-free reads). Reworked so the common path is read-only and a write lock is taken only when a legacy blob must be migrated -- reduces R-4 contention and the modify-before-UPDATE race window.

### Documentation Updates
- `docs/architecture/resolve.md`: added `resolve/irr/store/` to the structure table, a `store.go` source anchor, a dependency note (`store -> peeringdb -> irr` acyclic), and an "IRR Prefix Store" section. Source-anchor doc check passes for the new anchor.
- `docs/architecture/zefs-format.md`: no change needed (documents the key API, not a key list).

### Deviations from Plan
- `Refresh(ctx, name, asSet)` takes a third `asSet` hint (spec showed two args). Needed so the BGP plugin keys entries by the stable ASN identity (`AS<asn>`) while still querying with a configured/previously-discovered AS-SET; also keeps migration (keyed by `AS<asn>`) consistent. ACs unaffected (AC-1 is `Refresh(ctx, "AS13335", "")`).
- On a lookup error, `Refresh` returns a non-nil `CachedEntry` carrying the resolved AS-SET (no prefixes) plus the error, so the BGP plugin still records the fallback AS-SET on failure (preserves behavior asserted by `TestUpdateASNFailurePreservesState`); cached data untouched (AC-7).
- `RefreshAll` is a store API (tested directly); the BGP "update bgp irr all" path keeps its own per-ASN `refreshAllNow` loop, so operator behavior is identical.
- Open uses a read-first strategy (write lock only for migration) -- see Bugs Found/Fixed.

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
- **Total items:** 12 ACs (AC-6/AC-11 descoped to firewall-irr, user-approved), store unit tests + boundary test, 3 functional tests, 7 files
- **Done:** 10 in-scope ACs implemented + tested; all files created/modified (paths updated to the subpackage); store + filter_irr + zefs unit tests green; all three `test/plugin/filter-irr*.ci` pass; scoped golangci-lint 0 issues; doc anchor valid
- **Partial:** none
- **Skipped:** none
- **Descoped (user-approved):** AC-6 (`RefreshAll`) + AC-11 (`List`) moved to spec-firewall-irr -- no in-tree consumer; see Review Gate Run 2
- **Changed:** `Refresh` signature (+`asSet` hint); `Open` read-first lock strategy -- see Deviations
- **Out of scope (pre-existing, NOT this spec):** repo-wide `ze-lint-changed`/`ze-doc-test`/`ze-unit-test` failures from the dirty tree -- `internal/analyze/inject.go` goconst, `bgp-capa` plugin inventory + DESIGN.md drift, `chaos/inprocess` race, `tmp/lease-test`+`tmp/review` scratch builds

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Shared PrefixStore works | unit test | `TestPrefixStoreRefresh` passes |
| BGP filter_irr unchanged behavior | functional test | `test/plugin/filter-irr*.ci` pass |
| Migration works | unit test | `TestMigrateOldCache` passes |

## Review Gate

### Run 1 (initial) -- spec audit (pre-implementation)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | "Resolvers gets a PrefixStore field" contradicts the consumer architecture: filter_irr is a process-isolated SDK plugin (`RunEngine: runFilterIRR` over net.Conn) that builds its own clients from JSON config; it cannot receive the hub's in-process `resolve.Resolvers`. The only concrete consumer must construct its own PrefixStore in handleConfigure. Sharing with firewall-irr happens via the zefs file on disk, not a shared struct. | spec line 88; filter_irr.go:70-128 | Drop the Resolvers-field decision (or move to a hypothetical future in-hub consumer). Keep resolvers.go / main_system.go:91 out of Files to Modify. State that the BGP plugin builds PrefixStore next to NewIRR/NewPeeringDB (filter_irr.go:127-128). |
| 2 | ISSUE | "Same package" rationale is false: design says PrefixStore needs unexported `aggregateAndSort`, but `LookupPrefixes` already aggregates internally (client.go:193-194) and returns a ready PrefixList. Same-package placement is what creates the peeringdb cycle; a `resolve/irr/store` subpackage imports peeringdb directly with no cycle and no ASSetLooker workaround. | spec design table; client.go:173-199 | Fix the rationale or switch to a subpackage. May eliminate R-1/ASSetLooker entirely. User decision on package structure. |
| 3 | ISSUE | Cross-process zefs concurrency unaddressed. Two plugin processes (BGP + firewall) reading/writing `meta/irr/{name}` in one database.zefs is multi-writer; today only BGP writes (single blob under store.Lock). | Risks table | Add a risk + consistency story (per-key last-writer-wins; each plugin tolerates the other's entries). |
| 4 | NOTE | Missing AC: PeeringDB fallback also fires on *empty* result, falling back to literal "AS<asn>" name. AC-3 only covers "PeeringDB available". | filter_irr.go:317-329 | Add AC for "no AS-SET + PeeringDB empty -> query IRR with AS<asn>". |
| 5 | NOTE | `LookupPrefixes` serves a 1h in-memory cache without network (client.go:178-180), so Refresh can be a silent no-op within TTL; AC-7 (unreachable->error) only holds with a cold/expired cache. | client.go:178-197 | Test must force a fresh client / expired cache. |
| 6 | NOTE | Migration mapping unspecified: old cachedASN keyed by ASN(uint32)+ASSet field; new keys are name strings. | cache.go:18-23,117-134 | State: key = "AS"+asn, preserve ASSet field, read ALL old entries (migration is NOT config-gated unlike loadCacheFrom). |
| 7 | NOTE | AS-SET names legally contain ':' (client.go:264,276), so AC-2 yields keys like `meta/irr/AS3333:AS-FOO`. A-2 covers key count, not charset. (No '/' in AS-SET names, so no separator collision.) | client.go:276; zefs/keys.go | Validate ':' is zefs-key-safe or add an assumption. |
| 8 | NOTE | Shared store holds entries the BGP plugin never enrolled (AS-SETs, firewall ASNs). The enrollment-gated apply must stay in the plugin; don't apply store.List() blindly. | cache.go:127-134 | State explicitly in the rewire phase. |
| 9 | NOTE | Per-entry keys make RefreshAll do N Lock/Write/Release cycles vs today's single-blob write. | spec phase 7 | Consider batching RefreshAll writes under one lock. |

Confirmed non-issues: `filter_irr/cmd_irr.go` is pure RPC forwarding (no cache/refresh) -- correctly omitted from Files to Modify. 2-consumer count (BGP + firewall-irr spec) clears the premature-abstraction bar.

### Fixes applied
All 9 findings resolved in this spec (pre-implementation):
- #1 (BLOCKER): removed the "Resolvers gets a PrefixStore field" decision (Current Behavior + Design Decisions); resolvers.go / main_system.go stay out of scope; each consumer builds its own store, sharing via the zefs file.
- #2 (ISSUE): moved PrefixStore to a new `resolve/irr/store` subpackage (Task, Files to Create, TDD paths, Design Decisions, Core Insight); corrected the false `aggregateAndSort` rationale.
- #2/#3 (ISSUE): dropped the ASSetLooker interface (subpackage imports peeringdb directly); A-1 now validated; R-1 mitigation rewritten with interface as fallback only.
- #3 (ISSUE): added R-4 (cross-process zefs concurrency) and a Known Limitation.
- #4 (NOTE): added AC-12 (PeeringDB empty/error -> fall back to literal "AS<asn>").
- #5 (NOTE): TestPrefixStoreRefreshError must use a fresh client (1h cache caveat).
- #6 (NOTE): migration phase now states key = "AS"+asn, preserve ASSet, read ALL entries config-independent.
- #7 (NOTE): added A-5 (zefs key ':' tolerance for AS-SET names).
- #8 (NOTE): Files to Modify + Known Limitations state the enrollment gate stays in the plugin (no blind store.List()).
- #9 (NOTE): RefreshAll phase notes batching writes under one Lock.

### Run 2 -- /ze-review (post-implementation)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | `validateName` accepted `"."` (valid AS-SET char, invalid zefs segment); key `meta/irr/.` fails fs.ValidPath at decode and makes the whole shared store unreadable | store.go validateName | FIXED: reject `"."`/`".."`; regression `TestPrefixStoreRejectsBadName` |
| 2 | ISSUE | `List`/`RefreshAll` have no in-tree caller (firewall-irr only; AC-6/AC-11) | store.go List/RefreshAll | REMOVED with user approval: descoped to spec-firewall-irr (adds them wired to its consumer). Methods + their tests deleted (`test-relax` recorded; relax audit clean). ze-validate no longer flags them. AC-6/AC-11 moved below. |
| 3 | NOTE | `CachedEntry` flagged unwired by validate.py | store.go CachedEntry | Confirmed false positive: consumed via inference (`entry.PrefixList()`/`.ASSet`) in filter_irr; no change |
| 4 | NOTE | `Open` silently ignored corrupt-file errors | store.go Open | FIXED: warn-log when an existing file fails to open (non-IsNotExist) |

### Run 3 -- /ze-review (fresh pass, concurrency lens)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | Lost-update race: zefs `Lock` is in-process only (lock.go) and each persist opens a fresh BlobStore, so concurrent persists (in-process: manual `update bgp irr asn` vs background refresh; or cross-process) clobber each other's keys via whole-file atomic-rename flush. The old single-blob cache wrote full snapshots so never lost keys. | store.go persist/Open | FIXED (in-process): PrefixStore `fileMu` serializes open->write->flush; regression `TestPrefixStoreConcurrentPersist` (passes under -race). Cross-process flock deferred to firewall-irr; R-4 + docs/912 wording corrected (was wrongly "coordinated by the zefs write lock"). |

### Run 4 -- /ze-review (fresh pass)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE -> fixed | Pre-existing data race: `plug.prefixStore` assigned in handleConfigure outside `plug.mu` and read in refreshASN outside `plug.mu` (the old `irrClient`/`pdbClient` fields had the identical pattern) | filter_irr.go handleConfigure/refreshASN | FIXED (user-approved): assign under `plug.mu`, capture into a local under the RLock; regression `TestRefreshASNStoreFieldRace` (-race). No other new findings; review converged. |

### Run 5 -- /ze-review-deep (11 parallel agents)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | (refuted) | Agent-claimed HIGH deadlock: store.mu/fileMu lock-order inversion | store.go Refresh/Open | REFUTED -- Refresh does s.mu.Unlock() before persist takes fileMu, so the two are never held together; Open is the only site holding both (fileMu->mu) and nothing holds mu while waiting for fileMu. No change. |
| 2 | LOW | Open keyed the in-memory map by the blob's JSON Name, not the zefs key segment; a corrupt/tampered file could land an entry in another name's slot | store.go Open | FIXED -- verify segment == e.Name, skip mismatches; regression `TestPrefixStoreOpenNameKeyMismatch` |
| 3 | MED/LOW | Test gaps (migrate multi-prefix/malformed; refreshASN nil-store; loadFromStore preservation) | tests | FIXED -- `TestMigrateMultiPrefix`, `TestRefreshASNNilStore`, `TestLoadFromStorePreservesExistingList` (cacheStorePath env left: trivial cold path) |
| 4 | LOW | "atomic rename" wording overstated (zefs pwrites in place for small updates) | resolve.md, store.go pkg comment | FIXED wording |
| 5 | LOW | Spec hygiene: stale RefreshAll/List rows after descope (Wiring, TDD, A-2, User Story 2, Files-to-Create) | this spec | FIXED |
| 6 | LOW (pre-existing class) | filter_irr reconfigure races: plug.config/refreshStop published outside plug.mu; refreshASN mutates a stale st after byASN swap | filter_irr.go | FIXED for consistency with the Run-4 prefixStore fix: config/refreshStop now set under plug.mu; refreshASN re-reads st under the write lock; `TestRefreshASNStoreFieldRace` extended to swap byASN |
| 7 | NOTE (deferred) | parseBareASN accepts lowercase/plain-decimal (firewall-only; BGP uses canonical asnName); refreshAll not cancellable on reconfigure (harm mitigated by st re-read); v4/v6 counts overstate when 500K cap fires (pre-existing) | store.go, filter_irr.go | Deferred -- latent, no current-consumer impact |
| 8 | MED/LOW (pre-existing, other files -- deferred) | command.go:55 `server` JSON injection (`.Str` not `.Quoted`); config.go:83 as-set unvalidated at parse; irr/client.go ~80 MB intermediate alloc on a 4 MB whois response | command.go, config.go, irr/client.go | Deferred to a separate change (files unmodified by this diff); flagged for follow-up |

Clean lenses: Error Handling, API Compatibility, Project Rules; Concurrency (no new -- 1 refuted, rest pre-existing); Data Flow (1 pre-existing count finding). All in-scope fixes verified: store + filter_irr pass `go test -race`, scoped golangci-lint 0, all three `filter-irr*.ci` pass.

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
- [ ] AC-1..AC-N all demonstrated
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
