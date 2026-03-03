# 296 — RIB-02: Adj-RIB-In Storage Plugin + Shared BGP Types

## Objective

Extract common BGP types (Event, Route, FormatRouteCommand, ParseNLRIValue) from bgp-rib into a shared package, then create bgp-adj-rib-in plugin storing received routes as raw hex wire bytes for fast replay to reconnecting peers.

## Decisions

- Raw hex wire bytes (AttrHex, NHopHex, NLRIHex) instead of parsed Route structs or wire-byte pool handles — format=full events provide these directly; replay uses `update hex attr set ... nhop set ... nlri FAM add ...` which the engine decodes to wire bytes, skipping the entire text formatting + re-parsing pipeline
- Next-hop wire hex derived from parsed next-hop IP: `net.ParseIP(op.NextHop).To4()` + hex encode — cheap, avoids needing raw next-hop bytes from format=full (they're part of MP_REACH_NLRI which is excluded from raw.attributes)
- All address families stored, no `isSimplePrefixFamily` filter — adj-rib-in must store VPN, EVPN, FlowSpec for complete replay
- Shared package at `internal/plugin/bgp/shared/` (infrastructure), not `internal/plugins/` (implementations) — correct package boundary
- bgp-rib uses type aliases to shared types, not reimports — zero API breakage
- Functional test deferred to rib-03: requires dispatch-command trigger from another plugin (bgp-rr); bgp-rr didn't exist yet in this spec's context

## Patterns

- `internal/plugin/` = infrastructure to support plugins; `internal/plugins/` = plugin implementations
- Monotonic sequence index (`seqCounter++`) per insert enables incremental replay with from-index filter
- Complex family NLRI (VPN/EVPN): use raw NLRI blob from format=full events; `prefixToWireHex` only works for simple prefix families (bare IPv4/IPv6 bytes)

## Gotchas

- VPN/EVPN NLRI bug: `prefixToWireHex` produces bare IPv4 prefix bytes for complex families (missing RD+labels). Fix: use raw NLRI blob from format=full events when family is not simple prefix format — discovered during Critical Review by tracing VPN code path
- gocritic `ifElseChain` lint requires `switch` for 3+ branches in NLRI hex selection

## Files

- `internal/plugin/bgp/shared/` — event.go, route.go, format.go, nlri.go (extracted from bgp-rib)
- `internal/plugins/bgp-adj-rib-in/` — rib.go (AdjRIBInManager, RawRoute), rib_commands.go, register.go, schema/
- `internal/plugins/bgp-rib/event.go` — type aliases to shared package
