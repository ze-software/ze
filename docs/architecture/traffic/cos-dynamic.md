# Dynamic Class of Service via RADIUS

BNG subscribers need a per-session 802.1p QoS profile assigned by RADIUS. The
static plugin ([cos-plugin](cos-plugin.md)) defines the named profiles and
`cos.Lookup()`. This adds the mechanism that applies one at session up.

## Decisions

### The `cos:` prefix in Filter-Id is the primary carrier

<!-- source: internal/core/cos/cos.go -- ParseFilterID, filterPrefix -->
<!-- source: internal/component/l2tp/plugins/authradius/extract.go -- the Filter-Id walk -->
<!-- source: internal/component/l2tp/plugins/authradius/extract_vsa.go -- extractVSACoSProfile -->

Filter-Id (RFC 2865 attribute 11) already carries the shaper rate and is already
extracted. It is multi-valued, so a `cos:` prefix lets both values coexist, and
that avoided registering a vendor ID for a new VSA.

`extract.go` falls back to a vendor-specific attribute when no Filter-Id value
carries the prefix. The prefix is the primary path, and the VSA is the
compatibility path for a server that cannot send it.

### Subscribe to the L2TP session event, not the subscriber event

<!-- source: internal/plugins/cos/handler.go -- cosHandler, SessionUp handling -->
<!-- source: internal/component/l2tp/session_metadata.go -- AuthMetadata.CoSProfile -->

The L2TP metadata store carries `CoSProfile` in `AuthMetadata`. Subscriber events
have no tunnel or session ID, so they cannot look that metadata up.

### Per-session state in a `sync.Map`

<!-- source: internal/plugins/cos/session_state.go -- per-session CoS state -->

The session count is bounded and the shaper already does this. A global map buys
nothing.

### `ConfigureEventBus` for delivery

<!-- source: internal/plugins/cos/register.go -- ConfigureEventBus, lifecycle cleanup -->

`ConfigureEventBus` is the engine-side hook for a typed event subscription, and
it is what the shaper uses. `OnStarted` is not that hook.

### Reverting the static map passes nil

There is no backend method that reads the current QoS maps, so a revert cannot
restore what it did not record. Passing nil clears the map on both backends,
because `UpdateVLANQoSMap` is a full replace (`../iface/vlan-qos-map.md`), which
is the correct end state when no static config exists.

## Consequences

- `UpdateVLANQoSMap` is part of the `Backend` interface. Every new backend
  implementation must provide it.
- The CoA handler accepts a rate and a CoS Filter-Id in one request.
  `readCoAChange` walks every Filter-Id and Vendor-Specific attribute once and
  records the rate and the profile through `setRate` and `setCoSProfile`, which
  refuse a second value of either with NAK 407.
- `AccessInterface` was added to `SessionUpPayload`. It is empty for a pure L2TP
  LNS, where the access VLAN is at the remote LAC, and the handler skips that
  case instead of failing.

<!-- source: internal/component/l2tp/plugins/authradius/coa.go -- readCoAChange, setRate, setCoSProfile -->

## Traps

<!-- source: internal/plugins/iface/vpp/ifacevpp.go -- enableVLANQoS, UpdateVLANQoSMap -->
<!-- source: internal/component/l2tp/plugins/authradius/extract.go -- FindAllAttr for Filter-Id -->
<!-- source: internal/component/l2tp/plugins/authradius/coa.go -- readCoAChange walks pkt.Attrs -->

- **govpp binapi generates `[]byte` for a fixed-size `u8[256]` array.**
  `QosEgressMapRow.Outputs` must be explicitly allocated with
  `make([]byte, 256)`. `enableVLANQoS` carried a latent nil-slice panic that
  never fired, because no unit test exercised the VPP QoS path with a mock
  channel.
- **A multi-valued attribute needs `FindAllAttr`.** `extractAuthMetadata` used
  `FindAttr`, which returns the first match, so a Filter-Id carrying both a rate
  and a CoS profile lost one of them. The CoA path reads every attribute for
  the same reason: `readCoAChange` walks the whole attribute list.
