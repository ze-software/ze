# 885 -- Dynamic CoS via RADIUS

## Context

BNG subscribers need per-session 802.1p QoS profiles assigned dynamically via RADIUS. The static CoS plugin (spec-cos-plugin) defined named profiles and cos.Lookup(), but had no mechanism to apply them at session-up based on RADIUS attributes. Filter-Id (RFC 2865 attr 11) was already used for shaper rate; adding "cos:" prefix convention lets both coexist.

## Decisions

- Chose "cos:" prefix in Filter-Id over a dedicated VSA (vendor-specific attribute) because Filter-Id is already extracted and multi-valued; VSA requires vendor ID registration.
- Chose L2TP event subscription (l2tpevents.SessionUp) over subscriber-level events because L2TP metadata store (AuthMetadata) carries the CoSProfile; subscriber events don't have tunnel/session IDs for metadata lookup.
- Chose per-session sync.Map state (like shaper) over a global map because session count is bounded and the pattern is established.
- Chose ConfigureEventBus wiring (like shaper) over OnStarted for EventBus delivery because ConfigureEventBus is the engine-side hook for typed event subscription.
- Chose nil for static map revert over reading kernel state because there is no "read current QoS maps" backend method; AC-3 allows "or cleared" when no static config exists.

## Consequences

- UpdateVLANQoSMap is now part of the Backend interface; any new backend implementation must implement it.
- CoA handler accepts both rate and CoS Filter-Id values in the same request; extractRate and extractCoSProfile iterate FindAllAttr independently.
- AccessInterface added to SessionUpPayload; empty for pure L2TP LNS (handler skips gracefully).
- VPP QosEgressMapRow.Outputs must be explicitly allocated (make([]byte, 256)); govpp generates slices not arrays.

## Gotchas

- govpp binapi generates `[]byte` for fixed-size `u8[256]` arrays. The existing enableVLANQoS had a latent nil-slice panic on `QosEgressMapRow.Outputs` that never triggered because no unit test exercised the VPP QoS path with a mock channel. Fixed in both enableVLANQoS and UpdateVLANQoSMap.
- extractAuthMetadata used FindAttr (first match) for Filter-Id. With multi-valued Filter-Id (rate + cos), must use FindAllAttr. Also changed CoA extractRate to FindAllAttr for consistency.
- L2TP LNS sessions have no AccessInterface (the access VLAN is at the remote LAC). The handler must check for empty AccessInterface and skip, not crash.

## Files

- `internal/component/l2tp/events/events.go` -- AccessInterface in SessionUpPayload, SessionCoSChange event
- `internal/component/l2tp/session_metadata.go` -- CoSProfile field in AuthMetadata
- `internal/component/l2tp/session.go` -- accessInterface field in L2TPSession
- `internal/component/l2tp/reactor_kernel.go` -- emit AccessInterface in SessionUp
- `internal/component/l2tp/subscriber_bridge.go` -- propagate AccessInterface to subscriber.Session
- `internal/component/l2tp/subscriber_bridge_test.go` -- AccessInterface propagation tests
- `internal/component/iface/backend.go` -- UpdateVLANQoSMap interface method
- `internal/plugins/iface/netlink/manage_linux.go` -- netlink UpdateVLANQoSMap (LinkModify)
- `internal/plugins/iface/netlink/backend_other.go` -- non-linux stub
- `internal/plugins/iface/vpp/ifacevpp.go` -- VPP UpdateVLANQoSMap + enableVLANQoS nil-slice fix
- `internal/plugins/iface/vpp/ifacevpp_test.go` -- VPP QoS update tests
- `internal/plugins/cos/filter.go` -- ParseCoSFilterID
- `internal/plugins/cos/filter_test.go` -- filter parsing tests
- `internal/plugins/cos/handler.go` -- cosHandler (session-up/down/CoA)
- `internal/plugins/cos/handler_test.go` -- 8 handler tests covering AC-2 through AC-13
- `internal/plugins/cos/handler_helpers_test.go` -- test metadata helpers
- `internal/plugins/cos/session_state.go` -- per-session CoS state
- `internal/plugins/cos/register.go` -- ConfigureEventBus wiring, lifecycle cleanup
- `internal/component/l2tp/plugins/auth_radius/extract.go` -- FindAllAttr for Filter-Id, CoSProfile extraction
- `internal/component/l2tp/plugins/auth_radius/coa.go` -- CoA CoS change path, extractCoSProfile
- `internal/component/iface/config_test.go` -- fakeBackend UpdateVLANQoSMap stub
- `internal/component/iface/migrate_linux_test.go` -- mockMigrateBackend stub
