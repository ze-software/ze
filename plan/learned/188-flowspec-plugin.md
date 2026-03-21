# 188 — FlowSpec Plugin

## Objective

Create a FlowSpec NLRI plugin that handles decode/encode, then generalise the pattern for all family plugins and implement auto-loading of plugins for unclaimed families.

## Decisions

- Phase 1 keeps FlowSpec types in `nlri/` (thin wrapper only); types move to plugin in Phase 2 — avoids breaking all consumers at once.
- Auto-load detection (Phase 6) is family-based, not name-based: plugin name is informational only; what matters is whether it registered the family.
- Engine infers Multiprotocol capabilities from `declare family ... decode` (Phase 5) — simpler than requiring plugins to explicitly inject capabilities.
- Two-phase startup: explicit plugins launch first, then engine scans unclaimed families and auto-loads matching plugins — explicit wins over auto.
- Generic `EncodeNLRI()`/`DecodeNLRI()` on Server (Phase 4) means engine never imports plugin packages.

## Patterns

- Family plugin owns its types; `nlri` package owns shared types (Family, RouteDistinguisher) used by multiple families.
- `"encode"` keyword registration bug: must register both `"encode"` and `"decode"` keywords independently.

## Gotchas

- Auto-load must check family claim, not plugin name — name matching would break if a plugin handles multiple families under a non-obvious name.
- Phase 4 introduced a bug where `"encode"` keyword was not registered; always verify both directions when adding a two-way protocol feature.

## Files

- `internal/component/bgp/plugins/nlri/flowspec/` — FlowSpec plugin implementation
- `internal/component/plugin/registry/` — family lookup, auto-load logic
- `internal/component/bgp/subsystem.go` — capability inference from declared families
