# Interface Management

ze creates, deletes and configures its own interfaces through netlink. The
config model is YANG, the operator surface is `ze interface`, and the monitor
sees every management change with no extra wiring.

<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- interface-physical, interface-unit -->
<!-- source: internal/component/iface/cli/main.go -- CLI dispatch -->
<!-- source: internal/plugins/iface/netlink/manage_linux.go -- link create, delete, address management -->

## Two layers, not one flat list

The schema separates physical properties from logical ones, which mirrors the
JunOS IFD and IFL model:

| Layer | Holds |
|-------|-------|
| interface | MTU, description, disable |
| unit | addresses, VLAN, VRF, sysctl |

Interface kinds are grouped type-first (ethernet, dummy, veth, bridge,
loopback), aligned with VyOS. Each kind is its own YANG list, so a kind can
carry its own constraint. A veth requires a peer name, and a flat interface
list cannot state that.

A VLAN unit creates a real OS subinterface named `<parent>.<vlan-id>`. A
non-VLAN unit above 0 is logical grouping only: its addresses go on the parent.

Validation happens at the YANG layer, so an invalid MTU, an out-of-range VLAN
ID or an unknown interface kind is rejected before any netlink call.

## Name validation is a security control, not a length check

An interface name reaches a `/proc/sys/net/ipv4/conf/<name>/` path, so a name
is restricted to alphanumeric characters, hyphen and dot. Length alone
(IFNAMSIZ is 15) does not stop path traversal.

<!-- source: internal/component/iface/config_sysctl.go -- name validation and sysctl writes -->

The VLAN composite name can exceed IFNAMSIZ when the parent name is long.
Validation checks the combined name, not the parent alone.

## Sysctl writes go through an overridable root

The sysctl root is a variable, not the literal `/proc/sys`. A unit test
redirects writes to a temporary directory, so it needs neither root nor
`/proc`.

`accept_ra` must be `2`, not `1`, when `forwarding` is true. With `1` the
kernel ignores Router Advertisements on a forwarding interface. Neither setting
states this on its own.

## Partial creation is cleaned up

If `LinkSetUp` fails after `LinkAdd` succeeded, the created interface is
deleted. Without that, a failed apply leaves a down interface behind and they
accumulate.

## Address origin comes from the kernel

ze does not run a Router Advertisement client. `addrOrigin()` maps the
`IFA_F_*` flags the kernel reports to `AddrInfo.Origin`, which is one of
static, slaac, temporary or dynamic. ze classifies what the kernel
autoconfigured and rides the existing coalesced netlink monitor. `AddrInfo`
also carries the valid and preferred lifetimes, and both reach `show interface`
JSON and the address event stream with no new surface.

<!-- source: internal/plugins/iface/netlink/slaac_linux.go -- addrOrigin -->
<!-- source: internal/component/iface/iface.go -- AddrInfo -->

A manually added IPv6 address with a finite valid and preferred lifetime has
`IFA_F_PERMANENT` clear, so it is flag-equivalent to a SLAAC address. An
integration test classifies `origin=slaac` against the real kernel with no
radvd and no RA daemon.

## Carrier events are queued per interface, never dropped

The event bus dispatches a subscriber synchronously on the emitter's goroutine,
and for a link event that goroutine is the netlink monitor's own read loop. A
subscriber that does netlink work there stops the loop, and the kernel-side
subscription queue overflows behind it. So the `up`, `down`, `router-discovered`
and `router-lost` subscribers do no work at all: each pushes onto a queue, and
one worker goroutine does every route call.

The queue holds ONE pending entry per subject: an interface for a carrier
transition, an (interface, router) pair for a router transition. A second push
for a subject that is still pending replaces the first. Nothing the consumers
read is lost by that, because they act on the state a subject ended in, and the
memory it costs is bounded by the number of subjects with an unconsumed event.

The replaced design was a 16-deep channel with a `default:` branch that dropped
the event when the buffer was full. A config commit was exactly when it filled,
because the commit holds the lock the worker takes across DHCP client stop and
start. The route handlers are idempotent by their recorded metric state, which
is what makes them safe against a duplicate event and helpless against a missing
one: a dropped `up` after an applied `down` left the default route at
`route-priority + 1024` with the link up, and nothing read live carrier state
afterwards to repair it.

Two things read live carrier state now. The worker applies the queued final
state, and a resync compares the recorded metric state against the interface
list the rate tracker already dumps every second, moving a route only where the
two definitely contradict each other. A recorded state of "unknown" is not a
contradiction: a DHCP lease sets it on every renewal, and repairing it would
re-install the route the client has just installed.

<!-- source: internal/component/iface/link_queue.go -- the queue, the worker and the carrier resync -->
<!-- source: internal/component/iface/register.go -- the subscribers that push and the handlers the worker calls -->
<!-- source: internal/component/plugin/server/engine_event.go -- EmitEngineEvent dispatches subscribers on the caller's goroutine -->
