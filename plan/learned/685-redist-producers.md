# 685 -- redist-producers

## Context

Ze had redistribution infrastructure (event bus, filter chain, bgp-redistribute consumer) but only two producers: L2TP subscriber /32//128 routes and static config-driven routes. Operators needed `redistribute { import connected }` to advertise directly connected interface prefixes into BGP, and L2TP needed RADIUS Framed-Route/Framed-IPv6-Route support to inject per-subscriber static routes learned from the AAA server.

## Decisions

- **Connected plugin at `internal/plugins/connected/`** following the static plugin pattern (events subpackage, YANG schema, ConfigureEventBus callback), over extending the iface component. The connected plugin is a pure observe-and-emit producer with no kernel programming (kernel already has connected routes).
- **Reference-counted prefix tracking** over simple emit-on-every-event. Multiple addresses on the same prefix (e.g., 10.0.0.1/24 and 10.0.0.2/24) emit one ActionAdd; ActionRemove only when the last address is removed.
- **EventBus subscription to `(interface, addr-added/removed)`** over polling or netlink direct subscription. Reuses the existing iface monitor's event pipeline.
- **Framed-Route parsing in `l2tpauthradius/extract.go`** using `FindAllAttr` (multi-valued), over a separate package. RFC 2865 Section 5.22 text format: "prefix/len gateway metric". Gateway field intentionally ignored (BNG: subscriber is always next-hop, emitted as NextHop=zero for per-peer nhop-self substitution).
- **RouteObserver interface extended with `tunnelID` parameter** over a separate metadata-lookup mechanism. Needed so the observer can call `LoadSessionMetadata(tunnelID, sessionID)` to read framed routes.
- **`framedEmitted` flag on routeRecord** over emitting on every NCP event. Dual-stack subscribers fire OnSessionIPUp twice (IPCP + IPv6CP); framed routes emit only on the first call.
- **Same `l2tp` ProtocolID for framed routes** over a separate protocol. These routes are L2TP-sourced; operators configure `redistribute { import l2tp }` and get both subscriber /32 and framed routes.

## Consequences

- `redistribute { import connected }` is now functional. Operators can advertise interface prefixes into BGP without static route duplication.
- L2TP subscribers with RADIUS Framed-Route attributes automatically have their per-subscriber static routes advertised and withdrawn alongside the subscriber /32 or /128.
- The `maxFramedRoutesPerSession` cap (64) bounds memory per session from RADIUS.
- Future protocol producers (OSPF, ISIS) join by the same pattern: events subpackage + `redistevents.RegisterProtocol/RegisterProducer`.

## Gotchas

- The iface monitor emits address payloads as `string(json)` not `[]byte`. The connected plugin's `parsePayload` handles both via type assertion.
- `addrPayload.Unit` must be `int` to match `iface.AddrPayload` and `netlink.addrEventPayload`. An initial `string` type caused silent JSON unmarshal zero-value.
- `OnSessionDown` is called before `ClearSessionMetadata` in all teardown paths, but the observer reads framed routes from its own `routeRecord` copy (populated at `OnSessionIPUp` time), so the ordering is safe.
- The `TeardownTunnelByID` path does not call `ClearSessionMetadata` per-session (pre-existing; not introduced here).

## Files

- `internal/plugins/connected/{connected,register,eventbus,logger}.go` -- new connected plugin
- `internal/plugins/connected/events/events.go` -- redistevents producer registration
- `internal/plugins/connected/schema/{ze-connected-conf.yang,embed,register}.go` -- YANG config
- `internal/plugins/connected/connected_test.go` -- 9 unit tests
- `internal/component/radius/dict.go` -- AttrFramedRoute (22), AttrFramedIPv6Route (99)
- `internal/plugins/l2tpauthradius/extract.go` -- extractFramedRoutes, parseFramedRoute
- `internal/component/l2tp/session_metadata.go` -- FramedRoute type, AuthMetadata.FramedRoutes
- `internal/component/l2tp/route_observer.go` -- RouteObserver interface + tunnelID, framed route emit
- `internal/component/l2tp/{reactor,teardown}.go` -- pass tunnelID to observer
- `internal/component/plugin/all/all.go` -- connected plugin blank import (make generate)
- `{cmd/ze/main_test,internal/component/plugin/all/all_test}.go` -- expected plugin list
- `docs/{features,comparison,guide/plugins}.md` -- documentation updates
