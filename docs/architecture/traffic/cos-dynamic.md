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

### CoA waits for the owner to apply its maps

<!-- source: internal/plugins/cos/session_transaction.go -- executeSessionTransaction, prepareSessionChange -->
<!-- source: internal/plugins/cos/register.go -- commandDecls, executeCommand -->

RADIUS dispatches the hidden `request class-of-service session` command through
the existing command transport. Its arguments are `transaction` and one JSON
value. Prepare carries `operation`, `tunnel-id`, `session-id`, `interface` and
`profile`; its answer carries an opaque `token`. Apply, rollback and commit
carry only `operation` and that token. The authenticated request's peer context
must remain the same for every step.

Prepare changes nothing. It reads the actual VLAN ingress and egress maps with
the interface backend's `GetLink`, copies the proposed profile and records the
session's previous CoS state. Apply installs the maps before publishing the new
profile. A failed apply restores the captured maps before reporting failure;
rollback restores both the actual maps and the session state. RADIUS commits
only after every owner accepted the request, then acknowledges the CoA.

The post-commit session event consumes one transaction receipt when its token
matches the retained committed snapshot and both current profile maps equal
the maps that command installed. That notification needs no second backend
write. Later legacy events still apply their profile. If the first notification
was lost and the profile's maps changed, the old receipt cannot suppress the
updated maps.

<!-- source: internal/plugins/cos/handler.go -- cosHandler.consumeCommandReceipt -->

The owner holds one pending change and one completed token with its snapshot.
The last committed change remains compensatable until the next prepare, so a
later owner's failed commit can restore every part of the request. This bounds
retained snapshots and makes retries of the last apply, rollback or commit idempotent.
An outstanding transaction refuses another prepare, so another request cannot
overwrite the state needed to compensate it. A pending change expires after one
minute: the next request restores its maps before admitting new work. Session
departure invalidates its tokens, so a delayed apply cannot restore a departed
subscriber. An external CoS process has no
interface backend wiring and refuses the operation; it cannot acknowledge maps
it did not install.

The netlink backend reports both VLAN maps in `GetLink`, including an empty map
when none is installed. VPP's `detailsToInfo` reports link identity and VLAN
attributes but does not report the maps. Its `UpdateVLANQoSMap` can program
identity ingress and egress marking, but the atomic CoA path refuses prepare
when it cannot capture the previous maps to restore. Empty unreported fields
are never treated as evidence that VPP has no installed mapping. This limitation
does not change the existing configuration or session event paths.

<!-- source: internal/plugins/iface/netlink/show_linux.go -- linkToInfo -->
<!-- source: internal/plugins/iface/vpp/query.go -- detailsToInfo -->
<!-- source: internal/plugins/iface/vpp/ifacevpp.go -- UpdateVLANQoSMap -->

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
