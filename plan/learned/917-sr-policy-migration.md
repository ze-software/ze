# 917 -- SR-Policy Migration

## Context

Ze had SR-Policy wire encoding/decoding and CLI text parsing, but the ExaBGP config migration pipeline could not build SR-Policy routes from migrated config. The flex migration correctly split tokens but `extractRoutesFromUpdateBlock` had no handler for `ipv4/sr-policy` or `ipv6/sr-policy`, so content fell through to the standard prefix parser and failed on `netip.ParsePrefix("distinguisher")`.

## Decisions

- Added a generic `InProcessConfigRouteParser` registration callback to the plugin registry, over adding another hardcoded switch case in `extractRoutesFromUpdateBlock`. The user explicitly flagged the monolithic switch as an anti-pattern that kept recurring.
- The `PluginRoute` type carries pre-built NLRI + attribute wire bytes so the central config code needs no family-specific knowledge.
- TunnelEncap sub-TLV building lives entirely in the srpolicy plugin (`config.go`), over putting it in the central `bgp/config/` package. Follows the registration pattern.
- The `BuildPlugin` UPDATE builder uses `fullRawAttribute` to include plugin attributes in the sorted attribute list (before MP_REACH), over passing them as `rawAttrs` to `packAttributesOrderedInto` which appends them after MP_REACH.

## Consequences

- Future families with config route parsers can register via `InProcessConfigRouteParser` without touching central dispatch code. No new switch cases needed.
- The `PluginRoute` / `PluginParams` / `BuildPlugin` path is a generic reactor announce mechanism. Any plugin that produces pre-built NLRI + attributes can use it.
- The ExaBGP TunnelEncap sub-TLV encoding uses draft numbering (segment type-A = sub-sub-TLV type 1, type-B = type 13) which differs from RFC 9830 final assignments. This is intentional: ze must match ExaBGP's wire format for migration compat.

- MPLS S-bit (bottom-of-stack) in Type-A segments and Binding SID: RFC 9830 §2.4.4.2.1 says "The S bit MUST be zero upon transmission." Ze previously set S=1 on every label entry (`label<<4 | 1`). Fixed to `label<<4`. ExaBGP sets S=1 on the last segment (is_last); both were non-compliant, just differently. The S-bit difference affects wire bytes only for multi-segment lists and is ignored by receivers per the same RFC section.
- Bridge passthrough: `convertAnnounceSRPolicy` initially dropped all tunnel-encap tokens (preference, priority, binding-sid, segment-list, etc.), forwarding only NLRI fields. Fixed to pass through all non-NLRI tokens verbatim.
- Core keyword gaps vs ExaBGP: Ze lacked `priority` (sub-TLV type 15) and rejected `binding-sid null` (no-label form, 2-byte value). Added both for ExaBGP parity.

## Gotchas

- `packAttributesOrderedInto` puts MP_REACH_NLRI last and appends `rawAttrs` after that. Passing plugin attributes via `rawAttrs` puts them after MP_REACH, which doesn't match ExaBGP ordering. They must go through the sorted attribute list.
- `WriteAttrTo` writes its own header (flags+code+len), then calls `WriteTo` for the value. A `fullRawAttribute` that carries complete wire bytes (header+value) in its `data` field must strip the header in `WriteTo` and return only the value length from `Len()`, or the header gets written twice.
- Segment Type-B with endpoint-behavior has 2 reserved bytes (EB flags) between the endpoint-behavior uint16 and the SID structure fields. Missing these shifts the SID structure by 2 bytes.
- The central dispatcher must filter `eor` and `del` operations before calling the plugin parser, since the parser only handles `add` content. EOR with no content otherwise triggers a confusing "requires distinguisher, color, and endpoint" error.

## Files

- `internal/component/plugin/registry/registry.go` -- PluginRoute type, InProcessConfigRouteParser, ConfigRouteParserByFamily
- `internal/component/bgp/plugins/nlri/srpolicy/config.go` -- SR-Policy config parser + TunnelEncap builder (NEW)
- `internal/component/bgp/plugins/nlri/srpolicy/config_test.go` -- unit tests (NEW)
- `internal/component/bgp/plugins/nlri/srpolicy/register.go` -- registration
- `internal/component/bgp/config/bgp.go` -- PluginRouteConfig type
- `internal/component/bgp/config/bgp_routes.go` -- registry dispatch, extractOp/extractContentTokens
- `internal/component/bgp/config/loader_routes.go` -- convertPluginRoute
- `internal/component/bgp/config/peers.go` -- PluginRoutes dispatch
- `internal/component/bgp/reactor/peersettings.go` -- PluginRoute type
- `internal/component/bgp/reactor/peer_initial_sync.go` -- sendPluginRoutesVia
- `internal/component/bgp/reactor/peer_static_routes.go` -- toPluginParams
- `internal/component/bgp/message/update_build_plugin.go` -- BuildPlugin + fullRawAttribute (NEW)
- `internal/component/bgp/message/update_build_plugin_test.go` -- BuildPlugin tests (NEW)
