# 1070 -- Forked Route Install via Loc-RIB RPC

## Context

OSPF and IS-IS do not program the FIB directly: their SPF installers insert
`locrib.Path` values into the process-wide Loc-RIB singleton (`locrib.Default()`),
which sysrib arbitrates and fib-kernel programs as `RTPROT_ZE`. In a forked
(external, out-of-process) plugin subprocess `locrib.Default()` returns nil (the
singleton lives in the engine's address space), so both installers short-circuited
every insert/remove: SPF still computed and snapshotted routes, but **not one route
reached the kernel** -- a silent black hole. This was the follow-up bug in
[[1068-digest-anchor-validator]]. The chosen direction (user call) was "make forking
work" via an RPC, not a fail-fast guard.

## Decisions

- **New `ze-plugin-engine:route-install` / `:route-remove` RPC** (batch payload). A
  forked plugin ships its route ops to the engine, which applies them to the REAL
  `locrib.Default()`; sysrib's `OnChange` carries them to the kernel unchanged
  (reuses [[639-rib-unified]]). Chose IPC-install over a fail-fast guard.
- **Wire carries the protocol NAME; the engine LOOKS IT UP** (`redistevents.ProtocolIDOf`)
  and REJECTS unknown -- never register-on-demand. ProtocolIDs are per-process
  (allocated by registration order), but the engine binary registers every in-tree
  protocol at package init (`ospfProtocolID = RegisterProtocol("ospf")`) regardless
  of config or fork, so a real forked-OSPF engine knows "ospf". Register-on-demand
  from wire input would pollute and, at ~65535 names, panic the global registry.
- **`RouteSink` interface per installer**: the local Loc-RIB in-process, a
  `routeinstall.Sink` (RPC) when forked. Chose over making `locrib.Default()` return
  a remote proxy (would mutate a core contract and violate module tiers). Wired via
  a package-level `routeInstallClient` set in `runOSPFEngine`/`runISISEngine`.
- **Sink buffers, Flush once per SPF Apply** (one route-install + one route-remove
  RPC per delta, R-1), with a bounded retry on transient error.
- **Disconnect cleanup (AC-8)**: the engine tracks each plugin's installed routes
  and withdraws them in `cleanupProcess`, so a crashed forked plugin leaves no stale
  kernel routes.

## Consequences

- Forked OSPF/IS-IS now install to the kernel. Static routes and the BGP RIB share
  the nil-Loc-RIB-when-forked property; adopting `RouteSink` there is follow-up.
- Reject-unknown means a forked route-installer's protocol MUST be registered in the
  engine (in-tree protocols are, via package init). A third-party external
  route-installer under a novel protocol would need an explicit registration
  mechanism -- not built.
- **Forked Graceful Restart route retention is NOT supported** (review ISSUE 3,
  accepted): a GR restart is a subprocess disconnect, and the engine-side AC-8
  withdrawal has no visibility into the plugin's GR suppress state
  (`ospf/spf/install.go` `RemoveAll` no-op). AC-8 (withdraw a crashed plugin) and GR
  (retain a restarting plugin) conflict on disconnect; resolving it needs a
  plugin->engine retain-across-restart signal. Follow-up: the OSPF/IS-IS GR specs.

## Gotchas

- **Wrong initial assumption -> a crash vector.** The first cut used
  `RegisterProtocol` (register-on-demand) believing "the engine may not have the
  name yet." False: all in-tree protocols register at package init. Adversarial
  review caught that a forked plugin could exhaust/panic the global ProtocolID
  registry. Fix: lookup + reject-unknown.
- **The QEMU test binary (`ze_core zetest ze_distro`) does NOT link ospf**, so the
  kernel test (`forked-route-install-kernel.ci`) installs under protocol "bgp" (core,
  always registered) as a protocol-agnostic stand-in; a real forked-OSPF engine
  links ospf. A generic Python external plugin drives the route-install RPC (no real
  adjacency needed) -- the RPC is the unit under test.
- **Disconnect-vs-install is TOCTOU-safe** only because `handleSingleProcessCommandsRPC`
  registers `defer cleanupProcess` before `defer wg.Wait()`; LIFO runs `wg.Wait()`
  first, draining in-flight installs before withdrawal.
- **A kernel route needs an on-link next-hop**: the needs-linux test adds a dummy
  interface so fib-kernel's programmed route is accepted.
- The route-install path is forked-only (in-process/bridge plugins have a non-nil
  local Loc-RIB and use the local sink), so no `dispatchPluginRPCDirect` twin.

## Files

- `pkg/plugin/rpc/types.go` (RouteInstall/RouteRemove envelope), `pkg/plugin/sdk/sdk_engine.go` (SDK methods)
- `internal/component/plugin/server/dispatch_route.go` (+ `dispatch.go` switch, `server.go` fields; `dispatch_cached.go` extracted to keep dispatch.go < 1000)
- `internal/core/rib/routeinstall/sink.go` (shared forked sink + retry)
- `internal/plugins/ospf/{spf/install.go,spf_wiring.go,register.go}`, `internal/plugins/isis/{spf/install.go,spf_wiring.go,register.go}`
- `test/plugin/forked-route-install.ci` (sysrib, darwin), `test/plugin/forked-route-install-kernel.ci` (kernel, QEMU)
- `docs/architecture/api/process-protocol.md`
