# Logical Interface Names and the Shared Resolver

A ze interface name is a logical key. The kernel device name is a separate
fact. `internal/component/iface/resolve.go` maps one to the other, so
`isis { interface uplink {} }` with `interface { ethernet uplink { os-name eth0 } }`
reaches kernel device `eth0`.

<!-- source: internal/component/iface/resolve.go -- Resolve, Addresses, Subscribe, classifyAddresses -->

Before this resolver, every consumer resolved a configured name straight
against the kernel, which forced `name == kernel device`. The data model held a
hidden `os-name` leaf that nothing parsed.

## Where the resolver lives, and what it returns

The resolver lives in the iface component, not in a standalone service and not
as per-consumer translation. iface already owns interface knowledge and the
Monitor event source.

`Binding` is a pure value type: `Ifindex`, `OsName`, `OperMAC`, `PermMAC`,
`MTU`, `State`. It is not a `netlink.Link` and not a `*net.Interface`, so a
consumer does not couple to `vishvananda/netlink`. It carries exactly what the
older per-consumer ioctl wrappers produced, which is what let those wrappers be
deleted.

<!-- source: internal/component/iface/iface.go -- Binding, AddrInfo, LinkEvent -->

The kernel device is never renamed. The binding is a map only.

## The config apply path translates, and does not go through the resolver

The apply path is the one consumer that does NOT call `Resolve`. It takes one
interface listing per apply and binds every ethernet entry from it, so each
logical name resolves once and Phase 2, Phase 2c and the address reconcile all
key their work by the same kernel device. Two properties come out of that
choice and neither comes free from the dispatch wrappers:

- The apply resolves against the config it is applying, including the rollback
  re-apply, rather than against whatever mapping happens to be published.
- The prune step resolves the PREVIOUS config with the previous config's own
  selectors, so it names the devices that config actually made.

The sites the wrappers cannot reach were the other half of the reason. The
per-interface sysctl keys and the ethtool offload ioctls take a device string
and make no backend call, so a translation that lives in the dispatch layer
never touches them.

<!-- source: internal/component/iface/config_apply.go -- bindDevices, deviceFor, unitOSName -->

Two sites name a SECOND interface rather than the entry's own, and each resolves
through the same map. A bridge member is enslaved in Phase 2a, after the map
exists, rather than beside the bridge create in Phase 1: enslaving by the
logical name put whatever device carried that name into the bridge. A mirror
destination is resolved when the mirror spec is built, so the tc filter points
at the capture port's kernel device. Either one, when its selector has no answer
yet, is left undone with a warning and the commit still succeeds, exactly as
every other unbound setting defers.

<!-- source: internal/component/iface/config_apply.go -- the Phase 2a bridge member loop -->
<!-- source: internal/component/iface/config_mirror.go -- mirrorSpecFor, mirrorDestination -->

An entry whose selector names no present device is UNBOUND, and every phase
skips it. No phase falls back to the logical name: that fallback is what let an
aliased entry configure whatever else carried its name, and it is the same
fallback `ResolveDevice` now refuses for a name that HAS a selector. A `mac/match`
that names more than one device refuses the apply, because nothing
distinguishes the candidates.

## The plugin-facing registries take the resolved device too

A registry a plugin fills is a second entry point holding a name nobody
translated, and fixing the apply path does not reach it.
`iface.MacvlanSpec.Parent` is documented as an OS device name and lands in
`netlink.LinkByName`, so a plugin that fills it from a configured interface name
builds on whatever wears that name. VRRP did exactly that until 2026-08-23.

`ResolveDevice` is exported for those callers. It is the same function the
by-name dispatch ops use, so a plugin takes the one answer rather than composing
`Resolve` with a selector check of its own: a name with NO selector is its own
kernel device, a name WITH one that resolves gives the device, and a name with
one that answers nothing or several is refused. VRRP binds each group's parent
through it once per apply and hands the result to its macvlan, its per-device
sysctls, its transport and its readiness probe.

A binding outcome may CREATE a virtual router and may MOVE one; it may not
destroy one. `ResolveDevice` refuses on any resolution failure once the name
carries a selector, and "the backend is not loaded" and "the interface listing
failed" are among them, so an error here does not mean the device is gone. A
consumer that tears state down on it converts one transient netlink read into a
permanent outage, because the per-name cache is dropped on every iface apply and
nothing re-runs the consumer until its own config changes. Take the error as
"could not ask this pass" and keep what is already running.

<!-- source: internal/component/iface/macvlan.go -- MacvlanSpec.Parent -->
<!-- source: internal/plugins/vrrp/engine.go -- apply, the binding step -->
<!-- source: internal/plugins/vrrp/groups.go -- parentDevice, deviceResolver -->

A consumer that must stay PURE is the exception, and it holds no resolver at
all: VRRP's config verifier and its `ze doctor` check judge the CONFIGURATION,
so asking the kernel there would make `ze config validate` refuse a
configuration whose NIC has not enumerated yet. They read no device, and every
device-bearing value they produce stays empty until an apply binds it. That is
why VRRP's group extraction carries the unit's VLAN tag rather than a composed
device name: the tag is a config fact, and the device is not.

A device answers a MAC selector only when the address it is matched on is its
OWN. Linux gives a device an address it did not bring in two ways, and both are
excluded. A VLAN sub-interface inherits its parent's address, so leaving it in
made a parent's own selector ambiguous the moment ze created a VLAN on it. A
bridge or a bond wears a member's address, so leaving it in did the same the
moment ze put the selected hardware into a bridge the same config file asked
for. The second exclusion reads IFLA_MASTER from the members rather than the
link type, because the type says what a device is and only the membership says
whose address it wears: a bridge with no port keeps the address the kernel gave
it, and still answers for it.

`ze doctor` counts the same population, and reaches it another way. The predicate
takes the backend's interface listing, and the doctor runs on a box whose daemon
may be down, so it reads the relation from sysfs instead: the kernel writes a
`lower_<name>` link into the directory of a device standing on another one, for a
stacked device and for an aggregator that has a member alike, and writes none for
a port or a veth. A doctor that counted every device carrying the address called
a bridged port ambiguous, at error severity, while the daemon bound to it.

<!-- source: internal/component/iface/config_apply.go -- validateSelectors, devicesWithMAC, isStackedDevice, aggregatingDevices -->
<!-- source: internal/component/doctor/checks_linux.go -- netDevicesWithAddress, hasLowerDevice -->
<!-- source: internal/component/iface/dispatch.go -- ResolveDevice -->

## The mapping is published before the apply, not after

`applyAndPublish` is the single entry point every config apply goes through. It
calls `setResolverConfig` and then `applyConfig`, in that order, because the
apply is itself a consumer: the by-name dispatch ops it reaches translate
through the resolver, and so does every consumer reacting to what it does.
Publishing afterwards ran each apply against the mapping of the commit before
it, and the reload path published nothing at all, which left every consumer on
the mapping the daemon booted with until it restarted.

<!-- source: internal/component/iface/config_apply.go -- applyAndPublish -->
<!-- source: internal/component/iface/register.go -- OnConfigure, OnConfigApply and its rollback -->

## Resolution path

`Resolve(name)` translates the logical name through `effectiveOSName(name)`,
which reads the config `os-name` override and falls back to the name itself,
then looks up the OS device. The mapping is published to the resolver from the
iface config-apply path BEFORE the apply runs, together with a reverse map from
OS name to logical names, so a kernel-name monitor event reaches every logical
name bound to it.

<!-- source: internal/component/iface/resolve.go -- setResolverConfig, bindResolverEvents -->
<!-- source: internal/component/iface/config.go -- ifaceEntry.OSName, osNameMap -->

`osNameMap` emits real overrides only. It skips the identity mapping and the
absent leaf, so every configuration where `name == os-name`, and every `ze init`
output, stays a no-op.

`os-name` applies to matched kinds only. The map is built from `Ethernet`
entries, the physical kind ze matches. Created kinds such as veth, bridge and
tunnel are made by ze under the logical name, so aliasing them would break
creation.

## Cache invalidation rides the monitor events

The cache is keyed by the logical name and the cached ifindex is a hint. Any
`created`, `up` or `down` event for the device drops the entry. RTM_DELLINK
arrives as `down`, so invalidate-on-down covers deletion. There is no deleted
topic to subscribe to.

Invalidation happens inside `onLinkEvent` itself, not over a subscriber
channel, so it is never subject to the discard below.

A `created` or `up` event also wakes every `mac/match` name that does not
currently know its device, and that set is how a freshly appeared device
reaches a binding it was never bound to. It is deliberately a superset of the
names whose selector equals the device's MAC: knowing which ones match means
reading the MAC, and the only way to read it here is a backend call on the
monitor's read loop, which the event bus forbids a subscriber to make. Each
woken name re-resolves and reaches the same answer. The set is empty in the
steady state, because a successful resolve caches its binding.

<!-- source: internal/component/iface/resolve.go -- recordBinding, cache invalidation, logicalsForLocked -->

## Event reality differs from the older prose

Events are emitted under `ifaceevents.Namespace` with the types `created`,
`up`, `down` and `addr-*`. They are not the `iface.Topic*` string constants.
The payload delivered to an in-process subscriber is a JSON string: the monitor
marshals the payload and emits the string, so a `Subscribe` handler unmarshals
it. Verify a claim about the event stream against `monitor_linux.go`.

<!-- source: internal/plugins/iface/netlink/monitor_linux.go -- event emission and payload marshalling -->

The resolver fan-out sends non-blocking under its own mutex, the same lock
`cancel` holds when it closes a channel, so there is no send-on-closed race. A
full subscriber channel loses its OLDEST buffered event, never the one that
just arrived, so no subscriber can be left believing the wrong final state.
Each discard is logged and counted in `ze_iface_resolver_events_dropped_total`.
`docs/architecture/iface/management.md` carries why that direction matters. An
empty `default:` branch is refused by the pretool hook.

<!-- source: internal/component/iface/resolve.go -- sendLatest, onLinkEvent -->

## The consumer side is a standing gate, not a one-time migration

`internal/le/ifaceresolution.Answer` rejects new direct kernel name resolution
outside its allowlist. It runs as a `./le verify current mode full` stage through
`stagesForMode`, and the allowlist records every legitimate direct-resolution
site.

<!-- source: internal/le/ifaceresolution/ifaceresolution.go -- Answer -->
<!-- source: internal/le/verify/engine/run.go -- Run, RunMode -->

The gate proved load-bearing: the VPP tunnel, mirror and LCP files had to pass
it. A consumer that needs interface identity imports iface, which is an
accepted infrastructure dependency.

## Migrating a consumer couples its tests to a backend

Once a consumer calls `iface.Resolve` or `iface.Addresses`, its Linux
integration tests fail at open with "no backend loaded" unless they call
`iface.LoadBackend("netlink")` inside the netns before the first resolve. This
mirrors production, where the iface component is always loaded.

`AddrInfo` carries `LinkLocal`, set by `classifyAddresses`, so a consumer
splits IPv4, IPv6 link-local and IPv6 global with no re-parsing.
`classifyAddresses` is pure and host-testable.
