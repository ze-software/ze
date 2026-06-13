# 896 -- filter-irr

## Context

Ze had IRR whois and PeeringDB clients (in `resolve/irr` and `resolve/peeringdb`) but no integration with the BGP filter chain. Operators had to run bgpq4 externally, generate config, paste it in. This spec adds a `bgp-filter-irr` plugin that queries IRR for AS-SET prefixes and applies them as live import filters, refreshing automatically.

## Decisions

- New plugin `bgp-filter-irr` over extending `filter_prefix`: separation of concerns between static config-driven lists and dynamic external data, matching the RPKI/RIB split precedent
- Direct import of `resolve/irr` over RPC indirection: BGP plugins are in-process, RPC adds latency for no isolation benefit; verified no import cycle (irr imports only cache, textbuf, stdlib)
- Duplicated ~30 lines of prefix matching from `filter_prefix` over shared package or export: all filter_prefix types are unexported (A-4 broken); coupling two independent plugins through shared types adds fragility for minimal savings
- Fail-closed on empty/error over fail-open: accepting unvalidated routes is worse than temporarily rejecting valid ones, matches RPKI behavior
- Operator adds `import bgp-filter-irr:$remote_as` over auto-injection: no dynamic filter injection API exists in the reactor (A-7 broken); `$remote_as` variable resolution already works via `resolveFilterVars`
- `uint32` for refresh-interval over `uint16`: YANG range 60-86400 exceeds uint16 max (65535); caught during implementation
- Separate `refreshStop` channel per configure cycle: prevents goroutine leak when OnConfigure fires multiple times (config commit)
- zefs persistence via main `database.zefs` store over separate file: reuses existing infrastructure; key `meta/bgp/irr-cache` registered in keys.go

## Consequences

- Operators can replace external bgpq4 with one config line per peer/group
- PeeringDB auto-discovery means zero-config for the common case (peer has irr_as_set registered)
- Removing the `filter_irr/` directory cleanly removes all IRR filter features (plugin self-containment test passes)
- Future AS-path filter generation from IRR would be a separate plugin following the same pattern

## Gotchas

- `filter_prefix` types are all unexported: future shared matching logic would require either exporting types or extracting to `filter_common/`
- The per-peer `irr { as-set }` YANG augment must cover standalone peers, grouped peers, AND group-level session (three augment paths)
- `handleConfigure` spawns goroutines: without stopping the previous refresh loop, multiple loops accumulate on every config commit
- `strings.Cut(text, "nlri ")` matches substrings like "no nlri here": test case must use strings that genuinely lack the `nlri ` token

## Files

- Created: `internal/component/bgp/plugins/filter_irr/` (register.go, filter_irr.go, config.go, match.go, cache.go, yang/)
- Created: `test/plugin/filter-irr.ci`, `test/plugin/filter-irr-update.ci`
- Modified: `internal/component/plugin/all/all.go` (generated), `internal/component/plugin/all/all_test.go` (snapshot)
- Modified: `docs/guide/configuration.md`, `docs/features/plugins.md`
- Modified: `pkg/zefs/keys.go` (KeyIRRCache)
