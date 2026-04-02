# Spec: Resolution Component

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 8/8 |
| Updated | 2026-04-01 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/dns/resolver.go` - existing DNS resolver (moves to resolve/dns/)
4. `internal/component/dns/cache.go` - existing DNS cache (moves with resolver)
5. `internal/component/web/decorator_asn.go` - existing Cymru ASN decorator (moves to resolve/cymru/)
6. `internal/component/bgp/peeringdb/client.go` - existing PeeringDB client (moves to resolve/peeringdb/)
7. `internal/component/bgp/irr/client.go` - existing IRR whois client (moves to resolve/irr/)
8. `internal/component/bgp/irr/rir.go` - RIR delegation table (moves with IRR)
9. `internal/component/bgp/plugins/cmd/peer/prefix_update.go` - PeeringDB consumer (switches to Resolvers)
10. `cmd/ze/hub/main.go:442,666` - decorator/DNS wiring (switches to Resolvers)

## Task

Create a resolution component at `internal/component/resolve/` with four resolvers: DNS,
Team Cymru, PeeringDB, and IRR. Existing code is migrated from scattered locations into a
unified tree. All consumers (web decorator, LG graph, CLI prefix update) access resolvers
through a typed `Resolvers` container created at hub startup.

Hub startup creates resolver instances explicitly with resolved config, then passes the
`Resolvers` struct to consumers. No init() registration, no string-based dispatch. Each
resolver exposes its own typed API (returning native Go types, not stringified maps).
Cymru imports resolve/dns directly (sibling import, no cycle). Cymru, PeeringDB, and IRR
share a common TTL cache implementation from `resolve/cache/`. DNS keeps its existing
TTL-from-response cache.

CLI exposure (`ze resolve` commands) is out of scope -- consumers are programmatic (web
decorator, prefix update, LG graph). CLI resolution is a candidate for a follow-up spec
when a user workflow requires it.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/web-interface.md` - web decorator wiring for ASN names
  -> Constraint: Decorator registry uses YANG `ze:decorate` extension to trigger resolution at render time
- [ ] `docs/architecture/plugin-rib.md` - RIB pipeline (future consumer of cymru for graph terminal)

### RFC Summaries (MUST for protocol work)
Not protocol work -- no RFC references needed. Team Cymru DNS format is a de facto standard, not an RFC.
IRR uses RPSL whois protocol (RFC 2622) but the existing implementation already handles it.

**Key insights:**
- Explicit construction: hub/main.go creates each resolver with resolved config, stores in a `Resolvers` struct. No init() registration, no factory indirection.
- DNS receives `ResolverConfig` from `environment/dns` YANG config.
- PeeringDB receives base URL from system config.
- IRR receives whois server address from system config.
- Cymru receives a `*dns.Resolver` directly (sibling import, no cycle).
- Typed APIs: each resolver keeps its existing Go types. DNS returns `[]string`, PeeringDB returns `(uint32, uint32, error)` for prefix counts, IRR returns `[]netip.Prefix` and `[]uint32`, Cymru returns `(string, error)`.
- DNS resolver already has TTL-based LRU cache (CacheSize, CacheTTL). Keep as-is.
- Cymru, PeeringDB, and IRR have no caching today. Add 1h TTL via shared `resolve/cache/` package (3 users, same implementation).
- Two DNS resolvers currently instantiated in hub/main.go (lines 442 and 666). Should become one shared instance in Resolvers.
- PeeringDB client created per invocation (prefix_update.go:85, calls `peeringdb.NewPeeringDB()`). Should become persistent in Resolvers.
- PeeringDB rate limiting (1s between API calls) stays in the resolver.
- Web decorator becomes a thin wrapper: calls `resolvers.Cymru.LookupASNName(ctx, asn)`.
- `ApplyMargin` stays in `prefix_update.go` (consumer arithmetic, not resolution).
- CLI exposure (`ze resolve` commands) deferred to follow-up spec -- no current user workflow needs it.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/dns/resolver.go` (198L) - DNS resolver with miekg/dns, system DNS discovery, configurable timeout/cache
  -> Constraint: Uses third-party `github.com/miekg/dns` for wire protocol
  -> Constraint: ResolverConfig has Server, Timeout, CacheSize, CacheTTL fields
- [ ] `internal/component/dns/cache.go` (137L) - LRU+TTL cache, concurrent-safe, keyed by (name, qtype)
  -> Constraint: TTL from DNS response, capped by maxTTL. Empty results not cached.
- [ ] `internal/component/dns/schema/ze-dns-conf.yang` (48L) - DNS config YANG module
  -> Constraint: Defines environment/dns container with server, timeout, cache-size, cache-ttl leaves
- [ ] `internal/component/web/decorator_asn.go` (108L) - Team Cymru ASN name resolution via TXT DNS
  -> Constraint: Graceful degradation -- all errors return empty string, not error
  -> Constraint: Query format: `TXT AS<asn>.asn.cymru.com.`
  -> Constraint: Response parsing: `"ASN | CC | RIR | Date | LABEL - Org Name, CC"`
- [ ] `internal/component/web/decorator.go` (77L) - Decorator interface + DecoratorRegistry
  -> Constraint: Registry not concurrent-safe for writes (must register before serving)
- [ ] `internal/component/bgp/peeringdb/client.go` (204L) - PeeringDB HTTP client
  -> Constraint: Queries `/api/net?asn=<asn>`, parses `info_prefixes4/6` and `irr_as_set`
  -> Constraint: localhost TLS skip for testing, 1MB response limit, 10s timeout
  -> Constraint: Imports `internal/component/bgp/irr` for AS-SET name validation
- [ ] `internal/component/bgp/irr/client.go` (342L) - IRR whois client for RPSL queries
  -> Constraint: Two operations: AS-SET expansion (recursive, max depth 32), prefix lookup per family
  -> Constraint: Returns `PrefixList{IPv4, IPv6 []netip.Prefix}` and `[]uint32` (ASNs)
  -> Constraint: Uses RPSL `!i` command for AS-SET member expansion
  -> Constraint: Default server whois.radb.net:43, 10s timeout, 4MB response limit
  -> Constraint: `validateASSetName()` prevents whois injection
- [ ] `internal/component/bgp/irr/rir.go` (200L) - ASN-to-RIR lookup table
  -> Constraint: Generated table in rir_table.go (410KB), maps ASN ranges to RIRs
- [ ] `internal/component/bgp/plugins/cmd/peer/prefix_update.go` (216L) - PeeringDB consumer
  -> Constraint: Creates PeeringDB client per invocation, 1s rate limit between requests
  -> Constraint: Reads PeeringDB URL from system config via `system.ExtractSystemConfig()`
  -> Constraint: `ApplyMargin` is consumer arithmetic, stays here
- [ ] `cmd/ze/hub/main.go:442,666` - Two separate DNS+decorator setups (web UI and LG)
  -> Constraint: Both create independent DNS resolvers with default config
- [ ] `internal/component/lg/server.go` (253L) - LG server with `decorateASN` callback field
  -> Constraint: `LGServerConfig.DecorateASN` injected at creation, stored as function pointer
- [ ] `internal/component/lg/handler_ui.go` - LG UI handlers using `s.resolveASN()` at lines 210, 547
  -> Constraint: Uses same `s.resolveASN()` path as handler_graph.go
- [ ] `internal/component/lg/handler_graph.go` (64L) - Graph topology with ASN decoration
  -> Constraint: `decorateGraphNodes()` calls `s.resolveASN()` for each graph node

**Behavior to preserve:**
- DNS resolver API: Resolve, ResolveTXT, ResolveA, ResolveAAAA, ResolvePTR
- DNS cache: TTL-based with LRU eviction, concurrent-safe
- DNS config: server, timeout, cache-size, cache-ttl via YANG
- Cymru graceful degradation: errors return empty string
- Cymru parse format: "ASN | CC | RIR | Date | LABEL - Org Name, CC"
- PeeringDB: LookupASN (prefix counts), LookupASSet (IRR AS-SETs)
- PeeringDB localhost TLS skip for testing
- PeeringDB rate limiting: 1s between API calls (stays in resolver)
- IRR: ResolveASSet (recursive expansion), LookupPrefixes (per-family prefix lists)
- IRR: AS-SET name validation via validateASSetName
- IRR: RIR delegation table lookup
- AS-SET validation via irr package (PeeringDB imports resolve/irr after move -- genuine data dependency)
- Web decorator integration via YANG `ze:decorate` extension
- LG server `decorateASN` callback pattern

**Behavior to change:**
- DNS moves from `internal/component/dns/` to `internal/component/resolve/dns/`
- DNS config YANG moves to `internal/component/resolve/dns/schema/ze-dns-conf.yang`
- PeeringDB moves from `internal/component/bgp/peeringdb/` to `internal/component/resolve/peeringdb/`
- IRR moves from `internal/component/bgp/irr/` to `internal/component/resolve/irr/`
- Cymru extracted from `internal/component/web/decorator_asn.go` to `internal/component/resolve/cymru/`
- Web decorator becomes thin wrapper calling `resolvers.Cymru.LookupASNName(ctx, asn)`
- Hub startup creates `Resolvers` struct with one shared DNS instance instead of two
- PeeringDB prefix_update uses `Resolvers.PeeringDB` instead of creating client inline
- `ApplyMargin` stays in `prefix_update.go` (not moved with PeeringDB)
- Cymru, PeeringDB, and IRR get 1h result caching via shared `resolve/cache/` package
- Each resolver keeps its typed API (no `map[string][]string` conversion)
- `context.Context` as first parameter in resolver methods
- Old directories (`dns/`, `bgp/peeringdb/`, `bgp/irr/`) removed after move

## Data Flow (MANDATORY)

### Entry Point
- Go typed call: `resolvers.Cymru.LookupASNName(ctx, asn)` (or PeeringDB, IRR, DNS methods)
- Web decorator: `Decorate(value)` calls `resolvers.Cymru` internally

### Transformation Path
1. Consumer calls `resolvers.Cymru.LookupASNName(ctx, 65300)`
2. Cymru checks its 1h cache (shared `resolve/cache/` TTL cache) -- if hit, returns cached string
3. On miss: calls `cymru.dns.ResolveTXT(ctx, "AS65300.asn.cymru.com.")` (direct import)
4. DNS resolver checks its TTL cache -- if hit, returns cached records
5. On miss: sends UDP query via miekg/dns, caches result with response TTL
6. DNS returns `[]string{"ASN | CC | RIR | Date | Org"}`
7. Cymru parses TXT response, caches `"Org Name"` for 1h
8. Consumer receives `("Org Name", nil)`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Consumer -> Resolvers | Typed method call on specific resolver | [ ] |
| Cymru -> DNS | Direct import: `cymru.dns.ResolveTXT(ctx, ...)` | [ ] |
| PeeringDB -> HTTP | Direct HTTP GET to PeeringDB API, rate-limited 1s | [ ] |
| IRR -> Whois | Direct TCP connection to IRR server (RPSL protocol) | [ ] |
| Web decorator -> Cymru | `Decorate()` calls `resolvers.Cymru.LookupASNName(ctx, ...)` | [ ] |

### Integration Points
- `cmd/ze/hub/main.go` - Hub startup creates `Resolvers` struct with all instances
- `internal/component/web/decorator_asn.go` - Becomes wrapper around `resolvers.Cymru`
- `internal/component/bgp/plugins/cmd/peer/prefix_update.go` - Switches to `resolvers.PeeringDB`
- `internal/component/lg/server.go` - `decorateASN` callback wired through Resolvers
- `internal/component/lg/handler_ui.go` - Uses `resolveASN()` at lines 210, 547
- `internal/component/lg/handler_graph.go` - Graph ASN decoration uses Resolvers

### Architectural Verification
- [ ] No duplicated functionality (one DNS instance shared, not two)
- [ ] Cymru imports resolve/dns directly (sibling, no cycle) -- documented exception to "consumers don't import resolvers"
- [ ] PeeringDB imports resolve/irr for AS-SET validation (genuine data dependency, not architectural coupling)
- [ ] Zero-copy preserved where applicable (N/A -- string data, not wire bytes)
- [ ] context.Context flows from caller through to resolver

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Web UI ASN field decoration | -> | Decorator calls `resolvers.Cymru` | existing web UI .ci tests (verify ASN name appears) |
| `ze update bgp peer * prefix` | -> | prefix_update calls `resolvers.PeeringDB` | existing prefix_update .ci tests |
| LG graph ASN decoration | -> | LG callback calls `resolvers.Cymru` | existing LG .ci tests (verify graph node names) |
| Hub startup single DNS | -> | `Resolvers` struct shares one DNS instance | unit test: hub creates Resolvers, single DNS |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | DNS ResolveA called with hostname | Returns `[]string` of A records |
| AC-2 | DNS ResolveTXT called with hostname | Returns `[]string` of TXT records |
| AC-3 | Cymru LookupASNName called with valid ASN | Returns `(string, nil)` with org name |
| AC-4 | Cymru with invalid ASN (non-numeric, out of range) | Returns `("", nil)` (graceful degradation) |
| AC-5 | PeeringDB LookupMaxPrefix called with ASN | Returns `(ipv4Count, ipv6Count uint32, nil)` |
| AC-6 | PeeringDB LookupASSet called with ASN | Returns `([]string, nil)` of IRR AS-SET names |
| AC-7 | PeeringDB with unknown ASN | Returns error "not found" |
| AC-8 | Second call to same Cymru/PeeringDB/IRR query within 1h | Returns cached result, no network call |
| AC-9 | DNS query repeated within TTL | Returns cached result from DNS cache |
| AC-10 | Web UI ASN field still shows org name | Web decorator works through `resolvers.Cymru` |
| AC-11 | `ze update bgp peer * prefix` still works | Prefix update uses `resolvers.PeeringDB` instead of inline client |
| AC-12 | Hub startup creates single shared DNS instance | No duplicate DNS resolvers |
| AC-13 | IRR ResolveASSet called with AS-SET name | Returns `([]uint32, nil)` of member ASNs |
| AC-14 | IRR LookupPrefixes called with ASN | Returns `(PrefixList, nil)` with typed `[]netip.Prefix` per family |
| AC-15 | IRR with invalid AS-SET name | Returns error from validateASSetName |
| AC-16 | Resolver called with cancelled context | Returns context error, no hanging |
| AC-17 | PeeringDB requests within 1s of each other | Rate-limited, second request waits |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTTLCache` | `internal/component/resolve/cache/cache_test.go` | Shared TTL cache: store, hit, expiry, concurrent access | |
| `TestDNSResolveA` | `internal/component/resolve/dns/resolver_test.go` | A record resolution, returns `[]string` (existing, migrated) | |
| `TestDNSResolveTXT` | `internal/component/resolve/dns/resolver_test.go` | TXT record resolution, returns `[]string` (existing, migrated) | |
| `TestDNSCache` | `internal/component/resolve/dns/cache_test.go` | TTL-based cache (existing, migrated) | |
| `TestCymruASNName` | `internal/component/resolve/cymru/cymru_test.go` | ASN to org name, returns `(string, nil)` | |
| `TestCymruGracefulDegradation` | `internal/component/resolve/cymru/cymru_test.go` | Invalid ASN, DNS failure, malformed response return `("", nil)` | |
| `TestCymruCache` | `internal/component/resolve/cymru/cymru_test.go` | Second call returns cached, no DNS query | |
| `TestPeeringDBMaxPrefix` | `internal/component/resolve/peeringdb/client_test.go` | Prefix count lookup, returns `(ipv4, ipv6 uint32, nil)` | |
| `TestPeeringDBASSet` | `internal/component/resolve/peeringdb/client_test.go` | AS-SET lookup, returns `([]string, nil)` | |
| `TestPeeringDBCache` | `internal/component/resolve/peeringdb/client_test.go` | Second call returns cached, no HTTP request | |
| `TestPeeringDBRateLimit` | `internal/component/resolve/peeringdb/client_test.go` | Rapid calls rate-limited to 1s between API requests | |
| `TestIRRResolveASSet` | `internal/component/resolve/irr/client_test.go` | AS-SET expansion, returns `([]uint32, nil)` (existing, migrated) | |
| `TestIRRLookupPrefixes` | `internal/component/resolve/irr/client_test.go` | Prefix lookup, returns `(PrefixList, nil)` with `[]netip.Prefix` (existing, migrated) | |
| `TestIRRCache` | `internal/component/resolve/irr/client_test.go` | Second call returns cached, no whois query | |
| `TestIRRValidateASSetName` | `internal/component/resolve/irr/client_test.go` | Invalid AS-SET name rejected (existing, migrated) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ASN (Cymru) | 0-4294967295 | 4294967295 | N/A (non-numeric returns empty) | 4294967296 (graceful empty) |
| ASN (PeeringDB) | 0-4294967295 | 4294967295 | N/A | N/A (typed uint32) |
| AS-SET name (IRR) | validated string | N/A | N/A (empty rejected) | N/A (control chars rejected) |
| IRR recursion depth | 0-32 | 32 | N/A | 33 (stops recursion) |
| Cache TTL (Cymru/PeeringDB/IRR) | 1h hardcoded | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Web UI ASN decoration | existing web UI .ci tests | Web UI page shows ASN org name after resolver migration | |
| Prefix update | existing prefix_update .ci tests | `ze update bgp peer * prefix` works through Resolvers.PeeringDB | |
| LG graph decoration | existing LG .ci tests | LG graph nodes show ASN names after resolver migration | |

### Future (not this spec)
- CLI exposure: `ze resolve` commands (follow-up spec when user workflow requires it)
- NTP resolver plugin (mentioned by user as future addition)

## Files to Modify

- `cmd/ze/hub/main.go` - Replace two DNS+decorator setups with single `Resolvers` construction
- `internal/component/web/decorator_asn.go` - Becomes thin wrapper calling `resolvers.Cymru.LookupASNName(ctx, asn)`
- `internal/component/web/decorator.go` - Decorator interface unchanged; ASN decorator impl simplified
- `internal/component/bgp/plugins/cmd/peer/prefix_update.go` - Replace per-invocation PeeringDB client with `resolvers.PeeringDB.LookupMaxPrefix(ctx, asn)`. `ApplyMargin` stays here.
- `internal/component/lg/server.go` - `decorateASN` callback wired through Resolvers
- `internal/component/lg/handler_ui.go` - Lines 210, 547 use `s.resolveASN()` (same callback path as graph)
- `internal/component/lg/handler_graph.go` - `decorateGraphNodes` uses Resolvers through `s.resolveASN()` callback

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | N/A -- CLI exposure deferred to follow-up spec |
| CLI commands/flags | [ ] | N/A -- deferred |
| Editor autocomplete | [ ] | N/A -- deferred |
| Functional test for consumer paths | [ ] | Existing .ci tests verify web UI, prefix update, LG still work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | N/A -- internal reorganization, no new user-facing features |
| 2 | Config syntax changed? | [ ] | N/A |
| 3 | CLI command added/changed? | [ ] | N/A -- CLI exposure deferred |
| 4 | API/RPC added/changed? | [ ] | N/A -- no new RPCs |
| 5 | Plugin added/changed? | [ ] | N/A (component, not plugin) |
| 6 | Has a user guide page? | [ ] | N/A |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A -- no user-visible change |
| 12 | Internal architecture changed? | [ ] | New `docs/architecture/resolve.md` describing component |

## Files to Create

- `internal/component/resolve/resolvers.go` - `Resolvers` struct: typed container holding DNS, Cymru, PeeringDB, IRR instances. Constructor takes resolved config.
- `internal/component/resolve/resolvers_test.go` - Resolvers construction tests
- `internal/component/resolve/cache/cache.go` - Shared TTL cache: generic map+mutex+expiry, used by Cymru, PeeringDB, IRR (3 users)
- `internal/component/resolve/cache/cache_test.go` - Cache unit tests (store, hit, expiry, concurrent access)
- `internal/component/resolve/dns/resolver.go` - DNS resolver (moved from `internal/component/dns/`)
- `internal/component/resolve/dns/cache.go` - DNS cache (moved from `internal/component/dns/`, keeps existing TTL-from-response logic)
- `internal/component/resolve/dns/schema/ze-dns-conf.yang` - DNS config YANG (moved from `internal/component/dns/schema/`)
- `internal/component/resolve/dns/resolver_test.go` - DNS tests (moved)
- `internal/component/resolve/dns/cache_test.go` - Cache tests (moved)
- `internal/component/resolve/cymru/cymru.go` - Team Cymru ASN resolver (extracted from web/decorator_asn.go). Imports resolve/dns directly.
- `internal/component/resolve/cymru/cymru_test.go` - Cymru tests (extracted + cache tests)
- `internal/component/resolve/peeringdb/client.go` - PeeringDB client (moved from bgp/peeringdb/). Without `ApplyMargin` (stays in prefix_update.go)
- `internal/component/resolve/peeringdb/client_test.go` - PeeringDB tests (moved + cache + rate limit tests)
- `internal/component/resolve/irr/client.go` - IRR whois client (moved from bgp/irr/)
- `internal/component/resolve/irr/rir.go` - RIR delegation table (moved from bgp/irr/)
- `internal/component/resolve/irr/rir_table.go` - Generated RIR table (moved from bgp/irr/)
- `internal/component/resolve/irr/client_test.go` - IRR tests (moved + cache tests)
- `internal/component/resolve/irr/rir_test.go` - RIR tests (moved)
- `docs/architecture/resolve.md` - Resolution component architecture doc

## Files to Delete

- `internal/component/dns/resolver.go` - moved to resolve/dns/
- `internal/component/dns/cache.go` - moved to resolve/dns/
- `internal/component/dns/resolver_test.go` - moved to resolve/dns/
- `internal/component/dns/cache_test.go` - moved to resolve/dns/
- `internal/component/dns/schema/ze-dns-conf.yang` - moved to resolve/dns/schema/
- `internal/component/bgp/peeringdb/client.go` - moved to resolve/peeringdb/
- `internal/component/bgp/peeringdb/client_test.go` - moved to resolve/peeringdb/
- `internal/component/bgp/irr/client.go` - moved to resolve/irr/
- `internal/component/bgp/irr/client_test.go` - moved to resolve/irr/
- `internal/component/bgp/irr/rir.go` - moved to resolve/irr/
- `internal/component/bgp/irr/rir_table.go` - moved to resolve/irr/
- `internal/component/bgp/irr/rir_test.go` - moved to resolve/irr/
- `internal/component/dns/` - empty directory after moves
- `internal/component/bgp/peeringdb/` - empty directory after moves
- `internal/component/bgp/irr/` - empty directory after moves

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

1. **Phase: Shared cache** -- Create `resolve/cache/` with generic TTL cache
   - Tests: TestTTLCache (store, hit, expiry, concurrent access)
   - Files: `resolve/cache/cache.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Move DNS** -- Move dns package to resolve/dns
   - Tests: TestDNSResolveA, TestDNSResolveTXT, TestDNSCache (all existing, migrated)
   - Files: `resolve/dns/resolver.go`, `resolve/dns/cache.go`, `resolve/dns/schema/ze-dns-conf.yang`
   - DNS keeps its existing TTL-from-response cache (does not use shared cache)
   - Verify: all existing DNS tests pass at new location; old location deleted

3. **Phase: Extract Cymru** -- Extract from web/decorator_asn.go, add 1h cache via shared cache
   - Tests: TestCymruASNName, TestCymruGracefulDegradation, TestCymruCache
   - Files: `resolve/cymru/cymru.go`
   - Cymru imports resolve/dns directly (sibling, no cycle). Receives `*dns.Resolver` at construction.
   - Verify: Cymru resolves ASN name, calls DNS directly, caches result

4. **Phase: Move IRR** -- Move from bgp/irr, add 1h cache via shared cache
   - Tests: TestIRRResolveASSet, TestIRRLookupPrefixes, TestIRRCache, TestIRRValidateASSetName
   - Files: `resolve/irr/client.go`, `resolve/irr/rir.go`, `resolve/irr/rir_table.go`
   - Receives whois server address at construction
   - Verify: all existing IRR tests pass; cache prevents repeated whois queries

5. **Phase: Move PeeringDB** -- Move from bgp/peeringdb, add 1h cache via shared cache
   - Tests: TestPeeringDBMaxPrefix, TestPeeringDBASSet, TestPeeringDBCache, TestPeeringDBRateLimit
   - Files: `resolve/peeringdb/client.go`
   - Receives base URL at construction; rate limiting (1s) stays in resolver
   - PeeringDB imports `resolve/irr` for AS-SET validation (genuine data dependency, IRR moved first)
   - `ApplyMargin` stays in `prefix_update.go`
   - Verify: all existing PeeringDB tests pass; cache prevents repeated HTTP calls

6. **Phase: Resolvers container + rewire consumers** -- Create `Resolvers` struct, rewire hub, decorator, LG, prefix update
   - Tests: AC-10, AC-11, AC-12 (existing behavior through new path)
   - Files: `resolve/resolvers.go`, `hub/main.go`, `web/decorator_asn.go`, `lg/server.go`, `lg/handler_ui.go`, `lg/handler_graph.go`, `prefix_update.go`
   - Hub creates `Resolvers` once with one shared DNS instance, passes to consumers
   - Verify: web UI ASN names still show; prefix update still works; one DNS instance

7. **Phase: Delete old locations** -- Remove moved files and empty directories
   - Tests: `make ze-verify` (no broken imports)
   - Files: delete `internal/component/dns/`, `internal/component/bgp/peeringdb/`, `internal/component/bgp/irr/`
   - Verify: clean compile, no references to old paths

8. **Full verification** -- `make ze-verify`
9. **Complete spec** -- Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Cymru parse format matches existing behavior exactly |
| Correctness | Each resolver returns its own typed Go values (no string conversion) |
| Correctness | `context.Context` propagated from caller through to resolver |
| Naming | Package names: `resolve/dns`, `resolve/cymru`, `resolve/peeringdb`, `resolve/irr`, `resolve/cache` |
| Data flow | Consumers access resolvers through `Resolvers` struct, not direct package imports |
| Data flow | Cymru imports resolve/dns directly (sibling, documented exception) |
| Data flow | PeeringDB imports resolve/irr (not bgp/irr) for AS-SET validation |
| Rule: no-layering | Old `internal/component/dns/`, `bgp/peeringdb/`, `bgp/irr/` fully deleted (including directories) |
| Rule: no-layering | Web decorator_asn.go simplified to `resolvers.Cymru` wrapper, not kept alongside |
| Cache | DNS: existing TTL cache unchanged. Cymru + PeeringDB + IRR: 1h via shared resolve/cache |
| Rate limit | PeeringDB: 1s between API calls preserved in resolver |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Resolvers container exists | `ls internal/component/resolve/resolvers.go` |
| Shared cache exists | `ls internal/component/resolve/cache/cache.go` |
| DNS resolver moved | `ls internal/component/resolve/dns/resolver.go` and `! ls internal/component/dns/resolver.go` |
| DNS config YANG moved | `ls internal/component/resolve/dns/schema/ze-dns-conf.yang` and `! ls internal/component/dns/schema/ze-dns-conf.yang` |
| Cymru resolver exists | `ls internal/component/resolve/cymru/cymru.go` |
| PeeringDB moved | `ls internal/component/resolve/peeringdb/client.go` and `! ls internal/component/bgp/peeringdb/client.go` |
| IRR moved | `ls internal/component/resolve/irr/client.go` and `! ls internal/component/bgp/irr/client.go` |
| Old paths deleted | `! ls internal/component/dns/` and `! ls internal/component/bgp/peeringdb/` and `! ls internal/component/bgp/irr/` |
| Consumers use Resolvers struct | `grep -r "resolvers\." cmd/ze/hub/ internal/component/web/ internal/component/lg/ internal/component/bgp/plugins/cmd/peer/` shows typed access |
| Single DNS instance in hub | `grep -c "dns.NewResolver" cmd/ze/hub/main.go` returns 1 |
| ApplyMargin in prefix_update | `grep "ApplyMargin" internal/component/bgp/plugins/cmd/peer/prefix_update.go` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | ASN range validation in Cymru (0-4294967295). Hostname validation in DNS (no injection). |
| Input validation | AS-SET name validation in IRR (validateASSetName prevents whois injection) |
| DNS amplification | DNS resolver uses UDP with timeout, no recursive queries to arbitrary servers |
| HTTP SSRF | PeeringDB URL comes from system config (operator-controlled), not user input |
| Whois injection | IRR: validateASSetName rejects control characters and pipe chars |
| Response limits | PeeringDB: 1MB response limit preserved. DNS: miekg/dns handles message size. IRR: 4MB limit preserved. |
| Cache poisoning | DNS cache trusts miekg/dns response validation. Cymru/PeeringDB/IRR cache keyed by exact input. |
| TLS | PeeringDB: localhost TLS skip preserved for testing only. All other URLs require valid TLS. |
| Context cancellation | All resolvers respect context cancellation. No hanging on cancelled context. |
| Rate limiting | PeeringDB: 1s rate limit prevents API abuse |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Import cycle after move | Check dependency direction -- only allowed: cymru->dns, peeringdb->irr |
| Import cycle resolve/irr | IRR must move before PeeringDB (phase 4 before phase 5) |
| Existing consumer test breaks | Trace broken import, update to new path |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

### Resolvers Container

The `Resolvers` struct is a typed container holding resolver instances, created explicitly
at hub startup. No string-based dispatch, no init() registration.

Hub startup reads resolved config and creates each resolver:
- DNS gets `ResolverConfig` from `environment/dns` YANG config
- Cymru gets a `*dns.Resolver` (the shared DNS instance)
- PeeringDB gets base URL from system config
- IRR gets whois server address from system config

Consumers receive the `Resolvers` struct (or the specific resolver they need) and call
typed methods directly. No string keys, no type assertions, no parsing round-trips.

**Typed API per resolver:**

| Resolver | Method | Returns |
|----------|--------|---------|
| dns | ResolveA, ResolveAAAA, ResolveTXT, ResolvePTR | `([]string, error)` |
| cymru | LookupASNName(ctx, asn uint32) | `(string, error)` -- empty string on error (graceful) |
| peeringdb | LookupMaxPrefix(ctx, asn uint32) | `(ipv4, ipv6 uint32, error)` |
| peeringdb | LookupASSet(ctx, asn uint32) | `([]string, error)` |
| irr | ResolveASSet(ctx, name string) | `([]uint32, error)` |
| irr | LookupPrefixes(ctx, asn uint32, family) | `([]netip.Prefix, error)` |

### Cache Strategy

DNS keeps its existing TTL-based cache -- response TTL determines lifetime, with a configurable cap.
This is correct for DNS because TTLs vary per record and are authoritative.

Cymru, PeeringDB, and IRR use a shared TTL cache from `resolve/cache/`. ASN org names,
prefix counts, and AS-SET memberships change rarely. 1h TTL hardcoded. The shared cache
is a generic map with expiry timestamps, protected by a mutex. Three concrete users with
identical requirements -- meets the abstraction threshold.

### Cymru-DNS Direct Import

Cymru resolves ASN names by querying DNS TXT records. It imports `resolve/dns` directly
(sibling packages under `resolve/`, no import cycle). This is a documented exception to
the general "consumers don't import resolvers" rule -- Cymru is not a consumer, it is a
peer resolver with a genuine data dependency on DNS. Benefits:
- Compile-time safety (DNS type is known)
- No runtime "is DNS registered?" failure mode
- Simpler than string-based dispatch for one known dependency

### PeeringDB Rate Limiting

The 1s rate limit between PeeringDB API calls stays in the PeeringDB resolver. This is a
PeeringDB API courtesy, not a consumer concern. All callers get rate-limited automatically.
The cache reduces actual API calls further -- only cache misses hit the rate limiter.

### IRR Placement

IRR (Internet Routing Registry) uses RPSL whois protocol (RFC 2622), not BGP wire format.
It's external data resolution, same category as DNS/Cymru/PeeringDB. Moving it to `resolve/irr/`
removes the `resolve -> bgp` dependency that would exist if PeeringDB imported `bgp/irr`.
IRR must move before PeeringDB (implementation phase 4 before 5) so that PeeringDB can
import `resolve/irr` cleanly.

### PeeringDB-IRR Dependency

PeeringDB imports `resolve/irr` for AS-SET name validation. This is a genuine data dependency
(PeeringDB returns IRR AS-SET names that must be validated), not architectural coupling. Both
are sibling resolvers under `resolve/`. Documented as an intentional exception.

### Consumer Migration

The web decorator (`decorator_asn.go`) shrinks to a Decorate() method that calls
`resolvers.Cymru.LookupASNName(ctx, asn)` and returns the string directly.
The decorator interface and registry remain in the web package (they serve a different
purpose -- YANG-driven display annotation).

The LG server (`server.go`) keeps its `decorateASN` callback pattern. The callback is
wired at hub startup to call through `resolvers.Cymru` instead of through a direct
DNS resolver. `handler_ui.go` (lines 210, 547) and `handler_graph.go` all use the same
`s.resolveASN()` path, so rewiring the callback at the server level fixes all consumers.

The prefix update command (`prefix_update.go`) switches from creating a PeeringDB client
per invocation to calling `resolvers.PeeringDB.LookupMaxPrefix(ctx, asn)`. Returns typed
`(uint32, uint32, error)` -- no string parsing needed. `ApplyMargin` stays in
prefix_update.go as consumer arithmetic.

Hub startup (`main.go`) creates `Resolvers` once with one shared DNS instance, replacing
two separate DNS resolver instantiations.

## RFC Documentation

Not applicable -- resolution is not protocol work. Team Cymru DNS format documented inline.
IRR uses RPSL whois (RFC 2622) but the existing implementation already handles it.

## Implementation Summary

### What Was Implemented
- [To be filled after implementation]

### Bugs Found/Fixed
- [To be filled after implementation]

### Documentation Updates
- [To be filled after implementation]

### Deviations from Plan
- [To be filled after implementation]

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
- [ ] AC-1..AC-17 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all checks in `rules/quality.md` -- no failures)

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
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-resolve-component.md`
- [ ] **Summary included in commit**

