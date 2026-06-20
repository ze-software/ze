# 950 -- iface-resolve-2-resolver (+ IS-IS migration)

## Context

Second unit of the interface logical-name effort (`spec-iface-resolve-0-umbrella`).
Sub-spec 1 (949) made the OS device name and permanent MAC visible in `show
interface` but changed no resolution: every consumer still resolved a configured
name straight against the kernel, forcing `name == kernel device`. This unit builds
the shared `iface` resolver (`Resolve` / `Addresses` / `Subscribe`) that maps a
logical name to its kernel device via the `os-name` selector, and -- because the
repo wiring gate forbids landing an exported API with no production caller -- it
also migrates IS-IS (the umbrella's designated proof consumer, sub-spec 3) onto the
resolver. After this, `isis { interface uplink {} }` with `interface { ethernet
uplink { os-name eth0 } }` resolves to kernel `eth0`; the logical name is finally
decoupled from the kernel device name.

## Decisions

- **Merged sub-spec 2 + 3 (user-approved).** The resolver API cannot pass
  `ze-verify-wiring-docs` (every new exported symbol needs a non-test production
  caller) with no consumer, so the unit ships the API *and* its first consumer
  (IS-IS) together. Two AskUserQuestion gates confirmed: merge IS-IS, and migrate
  IS-IS's event path too (so `Subscribe` is wired, not just `Resolve`/`Addresses`).
- **`Binding` is a pure value type** (`Ifindex/OsName/OperMAC/PermMAC/MTU/State`) over
  returning a `netlink.Link`/`*net.Interface`, so consumers don't couple to
  vishvananda/netlink (cross-boundary value types). It carries exactly what the old
  IS-IS/PPPoE ioctl wrappers produced (ifindex, oper MAC, MTU) so those wrappers can
  be deleted.
- **os-name translation is the resolver's job, fed by config.** `Resolve(name)` ->
  `effectiveOSName(name)` (config `os-name` override, else the name itself) ->
  `GetInterface(osName)`. The mapping is published to the resolver from the iface
  config-apply path (`setResolverConfig` in register.go), with a reverse map
  (os -> logical) so a kernel-name monitor event reaches the logical name(s) bound
  to it. `osNameMap` only emits real overrides (skips identity and absent), so every
  `name == os-name` config (and every `ze init` output) stays a no-op -- backward
  compatible.
- **Un-hid the `os-name` YANG leaf.** It was `ze:hidden` and parsed by nothing
  (dormant). Promoting it to a real selector: removed `ze:hidden`, added
  `ifaceEntry.OSName` + a `parseIfaceEntry` read.
- **Cache invalidation via Monitor events, not a separate delete event.** The cache
  is keyed by logical name; any `created`/`up`/`down` event for the device drops the
  entry (the cached ifindex is a hint). RTM_DELLINK arrives as `down`, so invalidate-
  on-down handles deletion -- no `TopicDeleted` exists.
- **IS-IS event path: per-circuit `iface.Subscribe` + reader goroutine, dropping the
  EventBus worker-queue.** A reader goroutine (one per enabled circuit) can call
  `HandleLinkUp/Down` directly because it is off the EventBus's synchronous path (the
  resolver fan-out is non-blocking, drops on a full channel). The existing rescan
  backstop stays as the recovery for a dropped up-event. `t.subscribe` is injectable
  so the path is host-testable without a live resolver.

## Consequences

- IS-IS consumers (`circuits.go` 4 helpers, `redist_wiring.go` 1 helper, transport
  `resolveInterface`, transport link-event path) now go through `iface.Resolve` /
  `iface.Addresses` / `iface.Subscribe`. IS-IS now depends on the `iface` component
  being loaded (it always is in production); the integration tests must load the
  netlink backend to mirror that.
- `AddrInfo` gained a `LinkLocal bool` (omitempty), set by the resolver's
  `classifyAddresses` (pure, host-testable) so consumers split v4 / v6-LL / v6-global
  without re-parsing. `InterfaceInfo` size is unchanged (it holds `[]AddrInfo`), so
  no gocritic rangeValCopy cascade this time (contrast 949).
- The `Subscribe` API + the `os-name` selector are now real; LDP/DHCP (sub-spec 6)
  and the remaining consumers (sub-specs 4-7) still use direct kernel lookups until
  migrated. PPPoE still has its own duplicate ioctl wrapper (sub-spec 6).

## Gotchas

- **The iface monitor's event reality differs from the spec's wording.** Events are
  emitted under `ifaceevents.Namespace` with types `created`/`up`/`down`/`addr-*`
  (the `internal/component/iface/events` constants), NOT the `iface.Topic*` string
  constants in iface.go. The payload delivered to in-process subscribers is a **JSON
  string** (the monitor `json.Marshal`s then `Emit(..., string(data))`), so a
  Subscribe handler must `json.Unmarshal` it -- `iface.StatePayload`/`LinkPayload`
  match the shape, or decode `{name,index}` directly. And RTM_DELLINK is reported as
  `down`, not a distinct deleted event. Verify against `monitor_linux.go`, not the
  spec prose.
- **Migrating a resolver consumer couples its tests to the iface backend.** Once
  IS-IS's `OpenCircuit`/address helpers call `iface.Resolve`/`Addresses`, the
  `integration && linux` tests (`adjacency_integration`, `frr_interop`, transport
  `transport_integration`) fail at circuit-open with "no backend loaded" unless they
  `iface.LoadBackend("netlink")` (blank-import `internal/plugins/iface/netlink` for
  its init registration). Load it in the shared helper (`startRealEngine`, transport
  `withVethPair`), inside the netns, before the first resolve. This mirrors
  production, where the iface component is always loaded.
- **A non-blocking channel send under a mutex is the safe fan-out pattern, but an
  empty `default:` is hook-blocked.** The resolver fan-out does
  `select { case c <- ev: default: <log drop> }` under `r.mu` (the same lock `cancel`
  holds when it closes a channel, so no send-on-closed race). The pretool hook blocks
  an empty `default:` -- put a `loggerPtr.Load().Debug(...)` drop log in it.
- **os-name applies to matched kinds only.** `osNameMap` builds from `Ethernet`
  entries (the matched physical kind); created kinds (veth/bridge/tunnel/...) are made
  by Ze under the logical name, so aliasing them would break creation.
- **Pre-existing repo drift can block your verify gate.** `ze-verify-wiring-docs`
  failed on `docs/DESIGN.md` Shipped-Plugins drift (bgp-capa, bgp-nlri-srpolicy,
  firewall-irr, mrt missing; family count 21 vs 23) -- all from prior commits, none in
  this diff. The wiring check itself passed. Fixed the table to leave the gate green;
  `ze-doc-drift` (`scripts/docvalid/doc_drift.go`) is a checker, not an auto-fixer.

## Verification (all green)

- Host unit: `resolve_test.go` (9, incl. `TestResolveByOsName`, `TestNoNetlinkLeakInAPI`),
  transport `TestISISTransportEventOpensAndCloses` / `LateEnableSubscribes` / rescan.
- Docker linux unit: full `iface` + `iface/netlink` + `isis` trees.
- QEMU integration (verbose PASS): `TestResolveRemapsLogicalNameToOSDevice`,
  `TestAddressesRemapAndClassify`, `TestISISAdjacencyUpVeth`, `TestISISTransportRawSocketCap`.

## Files

- `internal/component/iface/iface.go` -- `AddrInfo.LinkLocal`, `Binding`, `LinkEvent`, `LinkEventKind`
- `internal/component/iface/resolve.go` -- resolver: `Resolve`/`Addresses`/`Subscribe`, cache, os-name translate + reverse map, event bind, `classifyAddresses`
- `internal/component/iface/config.go` -- `ifaceEntry.OSName`, `parseIfaceEntry` os-name read, `ifaceConfig.osNameMap`
- `internal/component/iface/register.go` -- `setResolverConfig` + `bindResolverEvents` wiring
- `internal/component/iface/yang/ze-iface-conf.yang` -- un-hid `os-name` leaf
- `internal/component/iface/resolve_test.go`, `config_osname_test.go`, `resolve_integration_linux_test.go`
- `internal/plugins/isis/transport/backend_linux.go` -- `resolveInterface` -> `iface.Resolve`
- `internal/plugins/isis/transport/transport.go` -- per-circuit `iface.Subscribe` event path (dropped EventBus worker-queue), `subscribe` field, Enable/Disable subscription lifecycle
- `internal/plugins/isis/circuits.go`, `redist_wiring.go` -- 5 address helpers -> `iface.Addresses`
- `internal/plugins/isis/adjacency_integration_linux_test.go`, `transport/transport_integration_linux_test.go`, `transport/transport_test.go` -- load iface backend; resolver-driven event test
- `docs/guide/configuration.md`, `docs/guide/isis.md` -- os-name selector + logical-name resolution
- `docs/DESIGN.md` -- pre-existing Shipped-Plugins + family-count drift (incidental)
- `test/isis/isis-logical-name.ci` -- os-name + IS-IS logical-name config surface
