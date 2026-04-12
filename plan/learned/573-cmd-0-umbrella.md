# 573 -- cmd-0 Vendor Parity Commands (Umbrella)

## Context

Gap analysis comparing Ze's CLI commands against Junos, Arista EOS, Cisco IOS-XR, and VyOS identified missing features needed for production BGP deployments. The umbrella organized 9 child specs by component boundary, covering config knobs (route reflection, next-hop, session policy, multipath), filter plugins (prefix-list, AS-path, community-match, route-modify), policy introspection (show policy), and operational commands (uptime, interface, ping, rib best reason).

## Decisions

- **Filter plugins, not route-maps.** Ze separates match from modify for composability: `filter import [ prefix-list:X as-path-list:Y modify:Z ]`. Each filter does one thing. This is more explicit than monolithic route-maps and enables operators to reason about filter chain behavior.
- **Named filters under `bgp/policy` with `ze:filter` extension.** All four filter plugins (prefix-list, as-path-list, community-match, modify) follow the same registration pattern: YANG augment with `ze:filter`, `FilterTypes` in registration, `OnFilterUpdate` handler, text delta mechanism. Zero changes to `filter_chain.go` or `config/peers.go` for any new filter type.
- **Community-match as separate plugin from tag/strip.** The existing `bgp-filter-community` uses `IngressFilter`/`EgressFilter` (structural mutation) with no `FilterTypes`. Match-and-act requires a separate `bgp-filter-community-match` plugin with `FilterTypes + OnFilterUpdate`.
- **Modify returns pre-built text delta.** The engine already has `textDeltaToModOps` -> `buildModifiedPayload`. The modify plugin just returns a delta string; no wire-level code in the plugin.
- **`show policy list/chain` as operational commands.** Filter types queried from `registry.FilterTypesMap()`. Peer chains queried from `PeerInfo.ImportFilters/ExportFilters` (new fields added to the public API).

## Consequences

- Ze now has feature parity with vendor NOS platforms for the core BGP policy toolbox: prefix filtering, AS-path filtering, community filtering, route attribute modification, and policy introspection.
- The filter framework scales to additional types without framework changes. Three filter types (cmd-4, cmd-5, cmd-6) were added with zero modifications to `filter_chain.go`.
- `show policy test` (dry-run) and `show policy detail` remain open for a future spec.
- AS-path prepend (`textDeltaToModOps` skips `as-path`) and export-path modify (delta discarded on egress) are deferred.
- Config authors must use `[0-9]` instead of `\d` in regex strings because ze's config parser consumes backslashes.

## Gotchas

- The existing `bgp-filter-community` tag/strip plugin and the new `bgp-filter-community-match` accept/reject plugin are separate plugins with different registration patterns. They can coexist in the same deployment.
- `extractCommunityField` for standard community required word-boundary matching (`cutOnWordBoundary`) to avoid false-matching `"community "` inside `"extended-community "` or `"large-community "`.
- Observer-based `.ci` dispatch testing of `show policy list` hangs because `dispatch-command` responses arrive after the MuxConn closes during shutdown. The handler works via standalone CLI. Polling-based observers (like `show errors`) avoid this by making many short calls.
- The `make generate` codegen picks up all plugins in `internal/component/bgp/plugins/*/register.go`. Adding a new filter plugin requires only `make generate` + updating the inventory test counts.

## Files

### Child spec learned summaries
- `plan/learned/548-cmd-4-prefix-filter.md` (pre-existing)
- `plan/learned/569-cmd-5-aspath-filter.md`
- `plan/learned/570-cmd-6-community-match.md`
- `plan/learned/571-cmd-7-route-modify.md`
- `plan/learned/572-cmd-8-policy-show.md`

### Key files created/modified across all child specs
- `internal/component/bgp/plugins/filter_aspath/` -- AS-path regex filter
- `internal/component/bgp/plugins/filter_community_match/` -- community match filter
- `internal/component/bgp/plugins/filter_modify/` -- route attribute modifier
- `internal/component/cmd/show/show_policy.go` -- policy introspection commands
- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` -- YANG for show policy
- `internal/component/plugin/types_bgp.go` -- ImportFilters/ExportFilters on PeerInfo
