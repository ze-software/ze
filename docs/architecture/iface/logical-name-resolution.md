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

## Resolution path

`Resolve(name)` translates the logical name through `effectiveOSName(name)`,
which reads the config `os-name` override and falls back to the name itself,
then looks up the OS device. The mapping is published to the resolver from the
iface config-apply path, together with a reverse map from OS name to logical
names, so a kernel-name monitor event reaches every logical name bound to it.

<!-- source: internal/component/iface/register.go -- setResolverConfig, bindResolverEvents -->
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

<!-- source: internal/component/iface/resolve.go -- recordBinding, cache invalidation -->

## Event reality differs from the older prose

Events are emitted under `ifaceevents.Namespace` with the types `created`,
`up`, `down` and `addr-*`. They are not the `iface.Topic*` string constants.
The payload delivered to an in-process subscriber is a JSON string: the monitor
marshals the payload and emits the string, so a `Subscribe` handler unmarshals
it. Verify a claim about the event stream against `monitor_linux.go`.

<!-- source: internal/plugins/iface/netlink/monitor_linux.go -- event emission and payload marshalling -->

The resolver fan-out sends non-blocking under its own mutex, the same lock
`cancel` holds when it closes a channel, so there is no send-on-closed race. A
drop is logged. An empty `default:` branch is refused by the pretool hook.

## The consumer side is a standing gate, not a one-time migration

`scripts/checks/iface_resolution.go` rejects new direct kernel name resolution
outside its allowlist. It runs as a `ze-verify` stage through
`stagesForMode`, and the allowlist records every legitimate direct-resolution
site.

<!-- source: scripts/checks/iface_resolution.go -- direct-resolution guard and allowlist -->
<!-- source: scripts/status/verify_run.go -- stagesForMode -->

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
