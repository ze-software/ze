# Spec: filter-irr

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/9 |
| Updated | 2026-06-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/plugin-self-containment.md` - command ownership
4. `internal/component/resolve/irr/client.go` - IRR whois client
5. `internal/component/resolve/peeringdb/client.go` - PeeringDB AS-SET discovery
6. `internal/component/bgp/plugins/filter_prefix/filter_prefix.go` - prefix-list filter plugin (pattern to follow)
7. `internal/component/bgp/plugins/rpki/rpki.go` - RPKI plugin (pattern for external data + BGP filter)
8. `internal/component/bgp/plugins/rpki/rpki_config.go` - config-from-OnConfigure pattern

## Task

Build a `bgp-filter-irr` plugin that generates prefix-list filters for eBGP peers
from IRR data, the way bgpq4 generates prefix-lists from IRR databases but
integrated into the BGP engine as live filters.

The plugin works automatically from the peer's remote ASN (already configured under
`session { asn { remote } }`):

1. Discovers the peer's AS-SET from PeeringDB using the remote ASN
2. Queries IRR for that AS-SET's prefixes
3. Builds a prefix-list and applies it as an import filter for that peer
4. Refreshes periodically (like RPKI refreshes VRPs)

Operators can override the auto-discovered AS-SET with an explicit one per peer
(via a leaf the plugin augments onto the peer config). The operator adds one line
to the peer's filter chain: `import bgp-filter-irr:$remote_as`. The `$remote_as`
variable is already resolved by the reactor, so the plugin receives filter-update
RPCs keyed by ASN.

`update bgp irr` triggers an immediate refresh outside the timer cycle:

- `update bgp irr all` -- refresh all peers
- `update bgp irr asn <asn>` -- refresh peers with this remote ASN
- `update bgp irr as-set <as-set>` -- refresh peers using this AS-SET

This is Ze's built-in equivalent of running bgpq4 externally and pasting the output
into router config. The key differences: Ze does it live, discovers AS-SETs
discovers AS-SETs automatically, refreshes without operator intervention, and
filters inside the BGP engine rather than as static config. The operator's only
config is the peer's ASN (already required) and one filter reference line.

### What exists today

| Piece | Location | Status |
|-------|----------|--------|
| IRR whois client (recursive AS-SET expansion, IPv4/IPv6 prefix lookup, aggregation, 1h cache) | `internal/component/resolve/irr/client.go` | Working |
| PeeringDB client (AS-SET name discovery by ASN) | `internal/component/resolve/peeringdb/client.go` | Working |
| Resolver container (`Resolvers.IRR`) | `internal/component/resolve/resolvers.go` | Working |
| RPC: `ze-resolve:irr-expand`, `ze-resolve:irr-prefix` | `internal/component/resolve/cmd/resolve.go` | Working |
| CLI: `ze resolve irr as-set/prefix` | `internal/component/resolve/cli/cmd_irr.go` | Working |
| Mock IRR server for tests | `internal/test/mock/irr/irr.go` | Working |
| `filter_prefix` plugin (ge/le/action prefix matching) | `internal/component/bgp/plugins/filter_prefix/` | Working (pattern to follow) |
| RPKI plugin (external data -> BGP validation) | `internal/component/bgp/plugins/rpki/` | Working (pattern to follow) |
| `update` verb root | `internal/component/cmd/update/` | Working |
| Deferrals entry | `plan/deferrals.md` | `spec-filter-irr (to be created)` |

### What this spec builds

A new BGP plugin `bgp-filter-irr` that:
1. Auto-discovers AS-SET from PeeringDB using each peer's remote ASN
2. Queries IRR for the AS-SET's prefixes and stores results in zefs (not in the config tree -- prefix lists are enormous and would make config unreadable)
3. Applies the resolved prefix-list as an import filter for each peer
4. Refreshes both automatically (configurable interval) and manually (`update bgp irr`)
5. Provides `show bgp irr` to inspect per-peer IRR state (AS-SET, prefix count, last refresh)
6. Provides `show bgp irr prefix <peer>` to list all prefixes for a peer
7. Provides `show bgp irr check <peer> <prefix>` to test whether a specific prefix would be accepted or rejected

### Scope boundary

- This spec does NOT restore `update bgp peer prefix` (PeeringDB max-prefix). That is `spec-update-bgp-prefix.md`.
- This spec does NOT generate router config text (no Cisco/Juniper/BIRD output). Ze IS the router.
- This spec does NOT replace RPKI validation. IRR and RPKI are complementary.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugin-self-containment.md` - plugin owns its full surface
  -> Constraint: YANG schema, handler, CLI all in the plugin's package
- [ ] `docs/architecture/core-design.md` - plugin registration, OnConfigure, filter chain
  -> Constraint: filters loaded at Stage 2 via OnConfigure; filter names come from config, not compile-time
- [ ] `ai/rules/plugin-design.md` - plugin boundaries, cross-boundary value types
  -> Constraint: plugin communicates via SDK RPC; no direct import of resolve package from BGP plugin

### Source Files (MANDATORY before implementation)
- [ ] `internal/component/bgp/plugins/filter_prefix/filter_prefix.go` - pattern: OnConfigure loads lists, OnFilterUpdate evaluates, atomic pointer swap
  -> Decision: follow same architecture for IRR-derived lists
- [ ] `internal/component/bgp/plugins/filter_prefix/config.go` - parses `bgp { policy { prefix-list NAME { } } }`
  -> Constraint: IRR config lives under `bgp { policy { irr { } } }`, parallel structure
- [ ] `internal/component/bgp/plugins/filter_prefix/match.go` - prefix matching with ge/le, partition, modify delta
  -> Decision: reuse the matching logic; IRR plugin builds the same `prefixEntry` structures
- [ ] `internal/component/bgp/plugins/rpki/rpki.go` - pattern: long-lived goroutine, external data source, periodic refresh, atomic state swap
  -> Decision: follow same refresh-loop pattern for IRR queries
- [ ] `internal/component/bgp/plugins/rpki/rpki_config.go` - parses config from OnConfigure JSON
  -> Constraint: IRR config parsed the same way
- [ ] `internal/component/resolve/irr/client.go` - `ResolveASSet(ctx, name)` and `LookupPrefixes(ctx, name)` with 1h cache, aggregation, recursive expansion
  -> Decision: plugin instantiates its own `irr.IRR` client (not via Resolvers container, since BGP plugins don't have access to it)
- [ ] `internal/component/resolve/peeringdb/client.go` - `LookupASSet(ctx, asn)` returns AS-SET names for an ASN
  -> Decision: optional auto-discovery; requires separate HTTP client
- [ ] `internal/test/mock/irr/irr.go` - fake IRR whois server: AS-TEST -> AS65001,AS65002,AS65003; prefixes 10.0.0.0/24, 10.0.1.0/24, 172.16.0.0/16, 2001:db8::/32
  -> Constraint: functional tests use this mock

**Key insights:**
- `filter_prefix` loads static lists from config. `filter_irr` loads dynamic lists from IRR queries but uses the same matching algorithm.
- The IRR client is a leaf package with no BGP dependencies; the BGP plugin can import it directly without import cycles.
- RPKI plugin is the closest pattern: external data source, long-lived goroutine, periodic refresh, atomic state swap, Prometheus metrics.
- Filter names are config-driven (Stage 2), so `bgp-filter-irr:NAME` follows existing conventions.
- The IRR client's built-in 1h cache prevents hammering the whois server, but the plugin should also have its own configurable refresh interval.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/filter_prefix/filter_prefix.go` - loads prefix-lists from static config; no external queries
- [ ] `internal/component/bgp/plugins/rpki/rpki.go` - connects to RTR caches, maintains VRP cache, validates on every UPDATE
- [ ] `internal/component/resolve/irr/client.go` - standalone IRR whois client; queries RADB/RIPE; caches results for 1h
- [ ] No `bgp-filter-irr` plugin exists today
- [ ] No BGP config leaf for per-peer AS-SET exists today
- [ ] No `update bgp irr` or `show bgp irr` commands exist today

**Behavior to preserve:**
- Static prefix-lists (`bgp-filter-prefix:NAME`) continue to work unchanged
- IRR client cache TTL (1h) continues to protect whois servers
- Existing filter chain dispatch (`CallFilterUpdate`) unchanged
- RPKI validation independent and complementary

**Behavior to change:**
- None. This is purely additive.

## Data Flow (MANDATORY)

### Entry Point
- Config: peer's `session { asn { remote } }` is the input; optional `irr { as-set }` override
- Global: `bgp { policy { irr { server; refresh-interval } } }` for IRR settings
- Runtime: OnConfigure delivers the BGP config; plugin parses peer ASNs + IRR section
- Manual: `update bgp irr` triggers immediate re-query of all peers

### Transformation Path
1. OnConfigure delivers BGP config JSON to plugin
2. Plugin parses global IRR settings (server, refresh interval) from `bgp { policy { irr { } } }`
3. Plugin walks peer configs, extracts remote ASN and optional `irr { as-set }` override
4. For each peer with a remote ASN:
   a. If explicit `as-set` configured: use it directly
   b. Else: query PeeringDB `LookupASSet(ctx, asn)` to discover AS-SET name
5. Query IRR: `LookupPrefixes(ctx, asSet)` returns `PrefixList{IPv4, IPv6}`
6. Convert aggregated prefixes to prefix-list entries (accept all, le=max, ge=prefix-length)
7. Persist resolved prefixes to zefs (operational data, not config); survives restart
8. Load from zefs on startup (stale data better than no data while waiting for first IRR query)
9. Store per-ASN prefix-list atomically in memory; operator references via `import bgp-filter-irr:$remote_as` in peer config
10. OnFilterUpdate: look up list by peer name, evaluate against UPDATE's NLRI
11. Refresh goroutine: re-queries PeeringDB + IRR at configured interval, swaps lists on change
12. `update bgp irr`: triggers immediate refresh (manual), independent of timer

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Plugin | OnConfigure JSON delivery | [ ] |
| Plugin -> IRR server | TCP whois query via `irr.IRR` client | [ ] |
| Plugin -> PeeringDB | HTTP GET for AS-SET auto-discovery (optional) | [ ] |
| Engine -> Plugin | `CallFilterUpdate` RPC for each UPDATE | [ ] |
| CLI -> Plugin | `update bgp irr` via RPC dispatch | [ ] |

### Integration Points
- `sdk.OnConfigure` - receives BGP config with IRR section
- `sdk.OnFilterUpdate` - evaluates UPDATEs against IRR-derived prefix-lists
- `irr.NewIRR(server)` - creates IRR client (direct import, no import cycle)
- `irr.LookupPrefixes(ctx, asSet)` - fetches and aggregates prefixes
- `pluginserver.RegisterRPCs` - registers `update bgp irr` and `show bgp irr`
- Plugin registration via `register.go` + `InternalPluginRunner`

### Architectural Verification
- [ ] No bypassed layers (uses OnConfigure + filter RPC, same as filter_prefix)
- [ ] No unintended coupling (IRR client is a leaf package; no import cycle)
- [ ] No duplicated functionality (reuses IRR client and matching algorithm; does not recreate)
- [ ] Zero-copy preserved where applicable (filter evaluation uses text protocol, no wire copies)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | BGP plugin can import `internal/component/resolve/irr` without import cycle | `irr` package imports only `cache`, `textbuf`, stdlib | Would need RPC-based indirection or copy the client | `go list -f '{{.Imports}}' ./internal/component/resolve/irr` | confirmed |
| A-2 | OnConfigure delivers config changes on every `config commit`, allowing runtime updates | filter_prefix relies on this; RPKI relies on this | Plugin would only see initial config, no live updates | Traced: reactor Reload -> deliverConfigRPC re-delivers to all plugins with WantsConfig | confirmed |
| A-3 | A BGP plugin can spawn a long-lived goroutine for periodic refresh | RPKI plugin does this (RTRSession.Run) | Would need cron or external trigger only | RPKI startSessions spawns goroutines from OnConfigure, governed by SDK context | confirmed |
| A-4 | `filter_prefix` matching logic can be reused by importing the package or extracting shared types | The types are package-private (`prefixEntry`, `prefixList`) | Would need to either export them, duplicate, or extract to a shared package | LSP confirms all types unexported | broken -- duplicate the ~30 lines of matching logic; avoids coupling two independent plugins |
| A-5 | IRR whois queries complete within a reasonable timeout (10s default in client) and do not block OnConfigure | Large AS-SETs (AS-CLOUDFLARE) may take time | OnConfigure could block, delaying peer startup | Follow RPKI pattern: parse config in OnConfigure, run queries in background goroutine | confirmed (mitigated) |
| A-6 | zefs blob store is available to BGP plugins at runtime | Config storage uses zefs; but BGP plugins may not have direct access | Would need to pass Storage interface through plugin server | `bgp/grmarker` imports `pkg/zefs` directly; precedent exists | confirmed |
| A-7 | Plugin can auto-inject a filter into a peer's import chain without explicit config reference | Requires reactor API or OnConfigure feedback to add filter dynamically | If not possible, operator must add `import bgp-filter-irr:$remote_as` manually | No dynamic injection API exists; ImportFilters comes from config only | broken -- use `import bgp-filter-irr:$remote_as` in config; `$remote_as` already resolved by `resolveFilterVars` |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Large AS-SETs (thousands of prefixes) cause slow startup | First configure takes >5s | Run IRR queries in background goroutine; use stale data until fresh arrives; log warning |
| R-2 | IRR server unreachable at startup | OnConfigure IRR query fails | Start with empty (reject-all) prefix-list; retry in background; log clearly; do not silently accept all |
| R-3 | IRR data is stale or wrong (IRR has no authentication) | Routes rejected that should be accepted | Operators monitor via `show bgp irr`; combine with RPKI for defense in depth |
| R-4 | Importing filter_prefix's private types forces code duplication | Package types are unexported | Extract shared matching types to `internal/component/bgp/plugins/filter_common/` or duplicate the small subset needed |

## Proposed Config Schema

The AS-SET lives under the peer, not in a separate policy block. The plugin
augments the peer config with an `irr` container. Global IRR settings (server,
refresh interval) live under `bgp { policy { irr { } } }`.

```
bgp {
    policy {
        irr {
            server "whois.radb.net";        // IRR whois server (default: whois.radb.net)
            refresh-interval 3600;           // seconds between re-queries (default: 3600)
        }
    }

    peer customer1 {
        session {
            asn {
                remote 65001;               // already exists -- this is the input
            }
            irr {
                // default: auto-discover AS-SET from PeeringDB using remote ASN
                // operator can override:
                as-set AS-CUSTOMER1;        // explicit AS-SET (overrides PeeringDB discovery)
                // enable disable;          // opt out of IRR filtering for this peer
            }
            filter {
                import bgp-filter-irr:$remote_as;  // operator adds this line
            }
        }
    }
}
```

The `$remote_as` variable is already resolved by the reactor's `resolveFilterVars`
(reactor_dynamic.go:346). This means the filter-irr plugin receives filter-update
RPCs keyed by ASN, which it uses to look up the IRR-resolved prefix-list.

The flow is:
1. Peer has `session { asn { remote 65001 } }` and `filter { import bgp-filter-irr:$remote_as }`
2. Reactor resolves `$remote_as` to `65001`, so the filter name becomes `bgp-filter-irr:65001`
3. Plugin sees peer ASN 65001 in OnConfigure, queries PeeringDB -> discovers AS-SET name
4. Queries IRR for that AS-SET's prefixes -> builds prefix-list keyed by ASN
5. On filter-update RPC for `bgp-filter-irr:65001`, evaluates against the resolved list
6. `update bgp irr all` refreshes all ASNs

The `irr { as-set ... }` leaf is augmented onto the peer YANG by the filter-irr
plugin. Removing the plugin removes the leaf (and the filter reference becomes a
no-op that the engine handles gracefully via fail-closed defaults). Operators who
know their peer's AS-SET can set it explicitly; others get auto-discovery from
PeeringDB.

Works naturally with peer groups: all peers in a group inherit the same
`import bgp-filter-irr:$remote_as` and each gets their own ASN-keyed prefix-list.
Also works with dynamic peers for the same reason.

(This is illustrative, not code. Final YANG schema designed during implementation.)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Peer config with `import bgp-filter-irr:$remote_as` + `irr { as-set AS-TEST }` | -> | IRR query + filter evaluation in `bgp-filter-irr` plugin | `test/plugin/filter-irr.ci` |
| CLI `update bgp irr all` | -> | Manual refresh handler (all peers) | `test/plugin/filter-irr-update.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer with `session { asn { remote 65001 } }`, `filter { import bgp-filter-irr:$remote_as }`, and no explicit AS-SET | Plugin auto-discovers AS-SET from PeeringDB via ASN 65001, queries IRR, builds prefix-list; reactor resolves filter to `bgp-filter-irr:65001` |
| AC-2 | UPDATE with prefix in IRR-resolved list | UPDATE accepted |
| AC-3 | UPDATE with prefix NOT in IRR-resolved list | UPDATE rejected (implicit deny) |
| AC-4 | IRR server unreachable at startup | Plugin starts with empty list (reject-all), logs warning, retries in background |
| AC-5 | Refresh interval elapses | Plugin re-queries IRR, atomically swaps prefix-list if changed |
| AC-6a | `update bgp irr all` | Immediate re-query of all peers' AS-SETs, prefix-lists updated |
| AC-6b | `update bgp irr asn 65001` | Refresh only peers with remote ASN 65001 |
| AC-6c | `update bgp irr as-set AS-CUSTOMER1` | Refresh only peers using AS-SET AS-CUSTOMER1 (explicit or discovered) |
| AC-7 | `show bgp irr` command | Displays per-peer: AS-SET name (discovered or explicit), prefix count (v4/v6), last refresh time, server, status |
| AC-8 | Peer with explicit `irr { as-set AS-CUSTOMER1 }` | Uses explicit AS-SET, skips PeeringDB discovery |
| AC-9 | Config change (peer added/removed, AS-SET changed) via `config commit` | OnConfigure re-parses; new peers queried, removed peers dropped |
| AC-10 | Multiple peers with same remote ASN (same AS-SET) | Single IRR query, shared prefix data, per-peer filter evaluation |
| AC-11 | Plugin removed (directory + import deleted) | Build succeeds, no orphaned commands, schema, or peer config leaves |
| AC-12 | IPv4 and IPv6 prefixes from IRR | Both families filtered correctly |
| AC-13 | IRR returns aggregated/collapsed prefixes | Aggregation from `irr.LookupPrefixes` preserved (no re-expansion) |
| AC-14 | Peer with `irr { enable disable }` | IRR filtering opt-out; no PeeringDB query, no filter applied |
| AC-15 | PeeringDB returns no AS-SET for a peer's ASN | Peer logged as "no AS-SET found", no filter applied (fail-open for discovery, not for filtering) |
| AC-16 | Resolved prefix-lists stored in zefs | Prefixes NOT in config tree; stored as operational data in zefs blob store; survives restart |
| AC-17 | `show bgp irr prefix <peer>` | Lists all IPv4 and IPv6 prefixes in the IRR-resolved list for this peer |
| AC-18 | `show bgp irr check <peer> <prefix>` | Reports whether the prefix would be accepted or rejected by the IRR filter, and which entry matches |
| AC-19 | Automatic refresh enabled by default | Plugin periodically re-queries IRR at configurable interval (default 3600s) without operator action |
| AC-20 | Manual refresh via `update bgp irr all/asn/as-set` | Immediate re-query scoped by selector, independent of timer |
| AC-21 | `show bgp irr` shows last refresh timestamp and next scheduled refresh | Operator can see when data was last updated and when it will refresh |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseIRRConfig` | `internal/component/bgp/plugins/filter_irr/config_test.go` | Config parsing: server, interval, named entries, as-set, defaults | |
| `TestIRRPrefixListFromLookup` | same or `filter_irr_test.go` | Converts `irr.PrefixList` to internal prefix-list entries correctly | |
| `TestIRRFilterAccept` | `filter_irr_test.go` | UPDATE with matching prefix accepted | |
| `TestIRRFilterReject` | same | UPDATE with non-matching prefix rejected | |
| `TestIRRFilterEmptyList` | same | Empty IRR result -> reject all (fail-closed) | |
| `TestIRRRefreshSwapsAtomically` | same | Concurrent filter evaluation during refresh sees consistent state | |
| `TestIRRAutoDiscovery` | same | `as-set auto` calls PeeringDB, then IRR | |
| `TestIRRServerUnreachable` | same | Unreachable server -> empty list, error logged, retry scheduled | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| refresh-interval | 60-86400 | 86400 | 59 | 86401 |
| prefix ge | 0-128 | 128 | N/A | 129 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `filter-irr` | `test/plugin/filter-irr.ci` | Peer with IRR filter: accepted prefixes pass, others rejected | |
| `filter-irr-update` | `test/plugin/filter-irr-update.ci` | `update bgp irr` refreshes prefix-list mid-session | |

### Interop Tests
N/A. IRR filtering is management-plane (whois protocol + internal filter). The BGP wire behavior is standard accept/reject already covered by filter_prefix interop.

## Files to Create

- `internal/component/bgp/plugins/filter_irr/register.go` - plugin registration
- `internal/component/bgp/plugins/filter_irr/filter_irr.go` - plugin entry point: OnConfigure, OnFilterUpdate, refresh loop
- `internal/component/bgp/plugins/filter_irr/config.go` - parse `bgp { policy { irr { } } }`
- `internal/component/bgp/plugins/filter_irr/match.go` - prefix matching (reuse or extract from filter_prefix)
- `internal/component/bgp/plugins/filter_irr/refresh.go` - background IRR query goroutine
- `internal/component/bgp/plugins/filter_irr/filter_irr_test.go` - unit tests
- `internal/component/bgp/plugins/filter_irr/config_test.go` - config parsing tests
- `internal/component/bgp/plugins/filter_irr/yang/embed.go` - YANG embed
- `internal/component/bgp/plugins/filter_irr/yang/register.go` - YANG registration
- `internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr.yang` - config schema
- `internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr-cmd.yang` - command schema (`update bgp irr`, `show bgp irr`)
- `test/plugin/filter-irr.ci` - functional test: filter evaluation
- `test/plugin/filter-irr-update.ci` - functional test: manual refresh

## Files to Modify

- `internal/component/plugin/all/all.go` - generated: `make generate` adds blank import after plugin created

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (config + commands) | [x] | `filter_irr/yang/ze-filter-irr.yang`, `ze-filter-irr-cmd.yang` |
| YANG validation constraints | [x] | `refresh-interval` range, `as-set` string pattern, `server` format |
| CLI commands | [x] | `update bgp irr`, `show bgp irr` via YANG command schema |
| CLI grammar (action before identifier) | [x] | `update bgp irr` has no selector (refreshes all); `show bgp irr` likewise |
| Functional test for new RPC/API | [x] | `test/plugin/filter-irr.ci`, `test/plugin/filter-irr-update.ci` |
| Pipe completeness | [ ] | `show bgp irr` routes through standard pipe handling |
| Doctor check for runtime dependencies | [ ] | IRR server is optional external; absence = empty list (logged). No doctor check needed |
| Prometheus counters/metrics | [x] | `ze_irr_prefixes_cached` (gauge), `ze_irr_refresh_outcomes_total` (counter: success/error), `ze_irr_last_refresh_timestamp` (gauge) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features/cli-commands.md`, `docs/features.md` |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` (new `bgp { policy { irr { } } }` section) |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` (`update bgp irr`, `show bgp irr`) |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md`, `docs/features/plugins.md` |
| 6 | Has a user guide page? | [ ] | Could warrant one; decide during implementation |
| 7 | Wire format changed? | [ ] | No |
| 8 | Plugin SDK/protocol changed? | [ ] | No |
| 9 | RFC behavior implemented? | [ ] | No (IRR/RPSL is informational, not standards-track) |
| 10 | Test infrastructure changed? | [ ] | No (mock IRR server already exists) |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` (Ze now has built-in IRR filtering, unlike peers that need bgpq4) |
| 12 | Internal architecture changed? | [ ] | No |
| 13 | Route metadata keys added/changed? | [ ] | No |
| 14 | Prometheus counters added/changed? | [x] | `docs/plugin-development/metrics.md` or telemetry doc |
| 15 | Registered plugin? | [x] | `docs/plugin-overview.md`, `docs/features/plugins.md` |
| 16 | Source anchors? | [ ] | Grep during implementation |
| 17 | Existing docs? | [ ] | No existing IRR docs |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Create, TDD Test Plan, validate assumptions A-1..A-5 |
| 3. Wiring phase | Register plugin, stub handler, failing wiring test |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-verify` |
| 7-14. | Standard flow per planning.md |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Validate assumptions** -- resolve A-1 through A-5 before writing code
   - Verify `irr` package can be imported from a BGP plugin (no cycle)
   - Verify OnConfigure re-delivers on config commit
   - Verify RPKI goroutine pattern is reusable
   - Assess filter_prefix type visibility; decide share vs duplicate vs extract
   - Test IRR query latency for a real-world AS-SET

2. **Phase: Wiring** -- register plugin skeleton
   - Create `register.go` with `InternalPluginRunner`
   - Create stub `filter_irr.go` with OnConfigure + OnFilterUpdate that log and reject
   - Create YANG config schema (`ze-filter-irr.yang`)
   - Run `make generate` to add blank import
   - Write failing functional test `filter-irr.ci` with mock IRR server
   - Verify: plugin loads, config parsed, filter rejects everything

3. **Phase: Config parsing** -- parse IRR section from OnConfigure
   - Tests: `TestParseIRRConfig`
   - Parse server, refresh-interval, named prefix-list entries with as-set
   - Verify: config parsed correctly, defaults applied

4. **Phase: IRR query + prefix-list build** -- core feature
   - Tests: `TestIRRPrefixListFromLookup`, `TestIRRFilterAccept`, `TestIRRFilterReject`, `TestIRRFilterEmptyList`
   - Instantiate `irr.NewIRR(server)`, call `LookupPrefixes`, convert to prefix entries
   - Store atomically, evaluate in OnFilterUpdate
   - Verify: functional test passes with mock IRR

5. **Phase: Refresh loop** -- periodic re-query
   - Tests: `TestIRRRefreshSwapsAtomically`
   - Background goroutine with configurable interval
   - Atomic swap on change, no-op on unchanged
   - Prometheus metrics
   - Verify: refresh test passes

6. **Phase: Commands** -- `update bgp irr` + `show bgp irr`
   - Create command YANG schema
   - Register RPC handlers
   - `update bgp irr`: trigger immediate refresh
   - `show bgp irr`: display per-list status
   - Write `filter-irr-update.ci` functional test
   - Verify: both commands work

7. **Phase: Auto-discovery** -- optional PeeringDB AS-SET lookup
   - Tests: `TestIRRAutoDiscovery`
   - When `as-set auto`: query PeeringDB for peer ASN, get AS-SET name, then query IRR
   - Verify: auto-discovery works in test

8. **Phase: Documentation** -- update docs
9. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | IRR prefixes aggregated; implicit deny on no match; fail-closed on error |
| Naming | Plugin `bgp-filter-irr`, YANG kebab-case, JSON kebab-case |
| Data flow | Config -> IRR query -> atomic prefix-list -> filter evaluation; no blocking OnConfigure |
| Plugin self-containment | Removing filter_irr/ removes all IRR filter commands and schema |
| Metrics | `ze_irr_*` counters registered and reported |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Plugin directory | `ls internal/component/bgp/plugins/filter_irr/` |
| Plugin registered | `grep filter_irr internal/component/plugin/all/all.go` |
| YANG schema | `ls internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr.yang` |
| Command schema | `ls internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr-cmd.yang` |
| Functional tests | `ls test/plugin/filter-irr*.ci` |
| Unit tests | `ls internal/component/bgp/plugins/filter_irr/*_test.go` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | AS-SET names validated via `irr.ValidateASSetName` (rejects control chars, injection) |
| Rate limiting | IRR queries limited by refresh interval (min 60s) + client cache (1h TTL) |
| Resource exhaustion | Max prefix count cap per list (guard against absurdly large AS-SETs) |
| Fail-closed | Empty list = reject all; query error = keep stale data, log, retry |
| Context propagation | All queries use cancellable context |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Import cycle with resolve/irr | Copy minimal IRR client subset into plugin, or use RPC indirection |
| filter_prefix types unexported | Extract shared types to filter_common, or duplicate small matching logic |
| OnConfigure blocks on slow IRR query | Move query to background goroutine; OnConfigure only parses config |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| New plugin `bgp-filter-irr` | Extend `filter_prefix` to support IRR sources | Separation of concerns: static lists vs dynamic external data; independent lifecycles; RPKI is a separate plugin from RIB for the same reason |
| Direct import of `resolve/irr` | RPC-based indirection via `ze-resolve:irr-prefix` | BGP plugins are in-process; RPC adds latency and complexity for no isolation benefit. Verify no import cycle first (A-1) |
| Background refresh goroutine | Cron-based, manual-only | Matches RPKI plugin pattern; operators expect live data; manual refresh available as supplement |
| Fail-closed on empty/error | Fail-open (accept all when IRR unavailable) | Network security: accepting unvalidated routes is worse than temporarily rejecting valid ones. Matches RPKI behavior |
| AS-SET config under the peer | Separate `bgp { policy { irr { } } }` with named lists | Operators think per-peer: "this peer's AS-SET is X". Plugin augments the peer YANG. Global settings (server, interval) stay under policy. Removing the plugin removes the per-peer leaf |
| Auto-discover AS-SET from PeeringDB by default | Require explicit AS-SET config | Zero-config for the common case. PeeringDB has AS-SET data for most networks. Explicit override available for exceptions |
| Operator adds `import bgp-filter-irr:$remote_as` to peer filter chain | Auto-inject filter without config reference | No dynamic injection API exists in the reactor. `$remote_as` variable resolution already works. One config line per peer/group is minimal, explicit, and works with groups and dynamic peers |
| Store resolved prefixes in zefs, not in config tree | Inline prefix-list in config | IRR prefix-lists are enormous (thousands of entries) and would make config unreadable. Stored as operational state in zefs, inspectable via dedicated commands |

## Known Limitations

- IRR data has no authentication (unlike RPKI ROAs). Operators should combine IRR filtering with RPKI validation for defense in depth.
- No max-length inference from IRR data. IRR returns exact prefixes; the plugin accepts exact match only (ge=prefix-length, le=max-family). Operators who need ge/le flexibility should use static prefix-lists.
- No incremental update. On refresh, the entire prefix-list is replaced. For typical AS-SET sizes (hundreds to low thousands of prefixes), this is fast enough.
- No AS-path filter generation from IRR. This spec covers prefix-lists only. AS-path filters from IRR AS-SET expansion would be a future spec.
- Resolved prefixes stored in zefs, not config. This is deliberate: IRR prefix-lists can contain thousands of entries and would make the config unreadable. They are operational state, not operator-authored config. `show bgp irr prefix <peer>` inspects the data; `show bgp irr check <peer> <prefix>` tests specific prefixes.

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A-4: filter_prefix types can be reused | All types (`prefixEntry`, `prefixList`, `evaluatePrefix`) are unexported | LSP workspace symbol search | Low: duplicate ~30 lines of matching logic; avoids coupling two independent plugins |
| A-7: Plugin can auto-inject filters into peer import chains | No dynamic injection API; `ImportFilters` comes from config only | Traced `reactor_notify.go:461` and `PeerSettings.ImportFilters` | Medium: operator must add `import bgp-filter-irr:$remote_as` to config; `$remote_as` variable already resolved by reactor |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

bgpq4 is an offline tool: query IRR, generate config text, paste into router. This
works but has three problems: (1) stale data between manual refreshes, (2) operator
toil to schedule and apply, (3) no integration with the routing engine's view of
what peers are configured. Ze's approach integrates the query loop into the BGP
engine itself: config names the AS-SET, the plugin handles the rest. The operator's
only job is to set the AS-SET and choose a refresh interval.

The plugin architecture follows the same split as RPKI: external data source
maintained by a background goroutine, feeding into the filter evaluation path that
runs synchronously on every UPDATE. This keeps the hot path (filter evaluation)
lock-free via atomic pointer swap, while the cold path (IRR query) runs
asynchronously.

## Review Gate

### Run 1 (post-merge audit, 2026-06-13) -- see plan/learned/898-filter-irr-fixes.md
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | `update bgp irr {all,asn,as-set}` rejected at registration: `update` not in the registry verb set, so all 3 commands were dead | `filter_irr.go:113-115` | Fixed: added `update` to `commandVerbs` (`command_registry.go`) |
| 2 | BLOCKER | Functional tests 158/159 never gate-run; used a non-existent `portvar=` mechanism so `$IRR_PORT` was never substituted | `filter-irr.ci`, `filter-irr-update.ci` | Fixed: bound mock IRR to the reserved `$PORT2` |
| 3 | ISSUE | Manual refresh was fire-and-forget; could not report failure and a failed refresh risked clobbering last-known-good state | `filter_irr.go` `updateASN`/`updateASSet`/all | Fixed: synchronous, errors when AS-SET undetermined, preserves existing prefix-list; unit tests added |
| 4 | BLOCKER | Multi-NLRI UPDATE with one out-of-list prefix rejected the whole update, dropping legitimate routes (no modify path) | `match.go` `evaluateUpdate` | Fixed: added `partitionUpdate` + `FilterModify` (mirrors filter_prefix); unit tests + deterministic functional test |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE  (formal gate still to run before closing)

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
- [ ] AC-1..AC-21 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated
- [ ] Critical Review passes
- [ ] Risks & Assumptions: every A-N confirmed or broken

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for numeric inputs
- [ ] Functional tests for end-to-end behavior
