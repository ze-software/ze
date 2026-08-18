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

An entry whose selector names no present device is UNBOUND, and every phase
skips it. No phase falls back to the logical name: that fallback is what let an
aliased entry configure whatever else carried its name, and it is the same
fallback `resolveOS` now refuses for a name that HAS a selector. A `mac/match`
that names more than one device refuses the apply, because nothing
distinguishes the candidates. A VLAN sub-interface is excluded from MAC
matching entirely: it inherits its parent's address, so leaving it in made a
parent's own selector ambiguous the moment ze created a VLAN on it.

<!-- source: internal/component/iface/config_apply.go -- validateSelectors, devicesWithMAC, isStackedDevice -->
<!-- source: internal/component/iface/dispatch.go -- resolveOS -->

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
outside its allowlist. It runs as a `ze-precommit-verify` stage through
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
