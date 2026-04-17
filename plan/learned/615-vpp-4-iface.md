# 615 -- vpp-4-iface

## Context

ze's interface management component supports pluggable backends: `netlink`
for Linux and `vpp` for VPP dataplane. Before this spec set, only the
`netlink` backend was wired; `vpp` was either missing or incomplete.
`spec-vpp-4-iface` targets ~2000 LOC to implement `iface.Backend` for VPP
via GoVPP, so `ze config edit` becomes the single configuration authority
for VPP interfaces instead of vppcfg/vppctl.

Prior to this session, three commits (`b8bb4f4d2`, `52d3b3c72`,
`a8f7cb157`) shipped the skeleton, core GoVPP wiring (Create/Delete,
Set*, AddAddress, Bridge*, VLAN), and the bridge-domain ID separation.
This session closed out the remaining in-scope methods and the
functional test, and deferred the pieces blocked on third-party
vendoring to concrete follow-up specs.

## Decisions

- **Lazy channel acquisition over eager dial.** `newVPPBackend`
  originally called `connector.NewChannel()` eagerly, which failed with
  "govpp: not connected" because the iface component loads backends in
  its OnConfigure phase, before the vpp component finishes its GoVPP
  handshake. Replaced with `ensureChannel` + `sync.Once`: the backend
  load succeeds, channel dial happens on first method call. Chose this
  over adding an iface-component vpp-ready gate because the gate is an
  independent orchestration concern and the lazy pattern also benefits
  unit tests (they inject a mock channel before any method call).
- **`dumpAllRaw` skips `ensureChannel`.** `populateNameMap` is invoked
  inside `ensureChannel`'s `populate.Do`; routing it through
  `dumpAllInterfaces` (which calls `ensureChannel`) deadlocks on the
  second `sync.Once.Do`. Split into a channel-less inner
  (`dumpAllRaw`) that populate uses, keeping the public
  `dumpAllInterfaces` gated.
- **Defer to vendoring follow-ups, not to "best effort" partial
  implementations.** VXLAN/GRE/IPIP tunnels, VPP stats, LCP pair, mirror,
  and wireguard all need GoVPP binapi packages not currently in
  `vendor/` (or the separate stats library). `rules/go-standards.md`
  bars adding third-party imports without user approval. Chose to
  defer every blocked AC with a named destination spec over stubbing
  with "not supported" and claiming the AC done.
- **Monitor emits ifacenetlink's JSON shape.** `SwInterfaceEvent` is
  translated into `linkEventPayload` / `stateEventPayload` with the
  same keys that `ifacenetlink` uses, so downstream subscribers
  (bgp/reactor, web UI, logging) stay backend-agnostic. Chose this
  over a VPP-specific payload because every consumer today reads the
  netlink shape and diverging would force them to multiplex.
- **Minimal functional test over stub-extension.** `006-iface-create.ci`
  validates AC-1 (backend registers/loads) and AC-13 (populateNameMap
  runs clean) against `vpp_stub.py`. Full `.ci` coverage for AC-2..16
  would need the stub to implement `create_loopback`,
  `sw_interface_set_flags`, `sw_interface_add_del_address`,
  `sw_interface_dump`, and `sw_interface_event` -- deferred to
  spec-vpp-stub-iface-api.

## Consequences

- `ze config edit` with `interface { backend vpp; }` now loads the VPP
  backend cleanly at startup even when the vpp component is still
  shaking hands with the dataplane. Subsequent method calls succeed
  once the channel is live.
- External subscribers to `(interface, up/down/addr-*)` on the
  EventBus work identically whether the active backend is netlink or
  ifacevpp. No multiplexing required in bgp/reactor.
- Reconciliation during config-apply still races ahead of the vpp
  connection: `ListInterfaces` returns "VPP connector not available"
  on first config delivery, and the iface config-apply errors once.
  Tracked in spec-iface-vpp-ready-gate. Backend-load itself
  succeeds; the iface component degrades to additive-only mode rather
  than failing the whole config.
- Tunnel/LCP/stats/mirror/wireguard support now live in
  `plan/deferrals.md` with destination specs. Each needs `make
  vendor-pull` of the relevant GoVPP binapi package plus user
  approval.

## Gotchas

- `sync.Once` recursion is easy to write by accident when helper
  methods call back into a gated initializer. The `dumpAllRaw` split
  wasn't obvious until tests started hanging under `-race`.
- VPP's `SwInterfaceDump` `NameFilter` is substring-match, not exact
  match: `GetInterface("xe0")` would happily return `xe0.100` without
  the post-dump re-check in `query.go:GetInterface`.
- VPP emits fixed-length 64-byte string fields with embedded NUL
  padding; `trimCString` (via `strings.Cut` on `"\x00"`) is mandatory
  before returning a name to callers or using it as a map key.
- The first vpp-4 functional-test run failed with
  `ze.log.iface=info` instead of `ze.log.interface=info`; the iface
  component logs under subsystem "interface", matching the YANG
  container name, not the package name.
- `binapi/interface.SwInterfaceSetMacAddress` carries the MAC as
  `[6]byte` (EUI-48). `net.ParseMAC` accepts EUI-64 too; the
  `len(hw) != 6` guard in `SetMACAddress` is not redundant.

## Files

- `internal/plugins/ifacevpp/ifacevpp.go` -- lazy ensureChannel
  refactor, Close releases monitor + channel.
- `internal/plugins/ifacevpp/naming.go` -- cross-ref updates.
- `internal/plugins/ifacevpp/query.go` *(new)* -- ListInterfaces,
  GetInterface, Get/SetMACAddress, populateNameMap, detailsToInfo,
  trimCString, dumpAllRaw.
- `internal/plugins/ifacevpp/monitor.go` *(new)* -- StartMonitor,
  StopMonitor, SwInterfaceEvent -> iface-namespace dispatch loop.
- `internal/plugins/ifacevpp/query_test.go` *(new)* -- mock-channel
  tests for all new query-path methods.
- `internal/plugins/ifacevpp/monitor_test.go` *(new)* -- mock-channel
  tests for monitor lifecycle and event translation.
- `test/vpp/006-iface-create.ci` *(new)* -- functional test: ze
  loads vpp backend via config, populateNameMap runs clean.
- `plan/deferrals.md` -- VXLAN/GRE/IPIP/stats/LCP/mirror/wireguard,
  iface-vpp-ready-gate, and stub-iface-api deferrals added.
