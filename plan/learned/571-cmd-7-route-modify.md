# 571 -- cmd-7 Route Attribute Modifier Plugin

## Context

Ze's vendor parity audit (cmd-0-umbrella) identified route attribute modification as the "set" half of route-maps. Ze separates match from modify for composability: match filters (prefix-list, as-path-list, community-match) do accept/reject, and the modify filter sets attributes. Operators compose them in chains like `filter import prefix-list:X modify:PREFER-LOCAL` for conditional modification.

## Decisions

- Created a new `bgp-filter-modify` plugin returning `action: "modify"` with a pre-built text delta over implementing wire-level modification in the plugin, because the engine already has the complete text-delta-to-wire pipeline (`applyFilterDelta` -> `textDeltaToModOps` -> `buildModifiedPayload`). The plugin just returns the delta string; the engine does all the heavy lifting.
- Pre-built the delta string at config load time over computing it per-UPDATE, because the modifier definition is static (config-driven) and the same delta applies to every route.
- Deferred as-path-prepend over implementing it, because `textDeltaToModOps` explicitly skips `as-path` (line 198) to avoid clobbering EBGP AS prepend that happens at the wire layer. AS-path prepend needs a different mechanism (AttrModPrepend op type exists but has no handler for code 2).
- Deferred export modify over implementing it, because the egress `PolicyFilterChain` call in `reactor_api_forward.go:420` discards the delta (underscore placeholder). Wiring it requires the same `textDeltaToModOps` + `buildModifiedPayload` integration that the import path has at `reactor_notify.go:395-410`.
- Used a `set` container in YANG over flat leaves, because operators expect `modify NAME { set { local-preference 200; med 50; } }` (explicit "set" verb matches Junos/IOS-XR mental model).

## Consequences

- Route attribute modification is now available via `filter import [ modify:NAME ]`. This is the fourth production filter type.
- Composable chains like `filter import [ prefix-list:X modify:PREFER-LOCAL ]` work: match filters accept/reject, and the modifier sets attributes on accepted routes.
- Only import-path modification works. Export-path modify is a framework gap (delta discarded), not a plugin gap.
- AS-path prepend requires a separate mechanism. Tracked as a deferral.

## Gotchas

- The plugin is deceptively simple (returns a pre-built string) because the engine does all the work. The complexity is in `filter_delta.go` and `forward_build.go`, not in the plugin.
- `textDeltaToModOps` skips `as-path` and `nlri` -- these cannot be modified via the text delta mechanism. This is intentional (AS prepend happens at the wire layer, NLRI override is handled separately).
- The export path silently discards modify deltas. A future spec needs to wire `textDeltaToModOps` + `buildModifiedPayload` into `reactor_api_forward.go` (same pattern as `reactor_notify.go:395-410`).

## Files

- `internal/component/bgp/plugins/filter_modify/` -- complete plugin (7 Go files + 1 test file + 3 schema files)
- `test/plugin/route-modify-localpref.ci` -- functional test
- `test/parse/modify-config.ci` -- config parse test
- `cmd/ze/main_test.go`, `internal/component/plugin/all/all_test.go` -- plugin inventory updates
- `internal/component/plugin/all/all.go` -- generated (make generate)
