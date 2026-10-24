# The forwarding door: interfaces, the system RIB, FIB writers and data planes

<!-- door-base: internal/component/iface internal/component/sysrib internal/component/vpp internal/component/firewall internal/component/traffic internal/component/trafficfeature internal/component/sysctl internal/component/mtu internal/plugins/iface internal/plugins/fib internal/plugins/firewall internal/plugins/traffic internal/plugins/policyroute -->

Read `ai/doors/system.md` first. This page covers the code that turns config and
selected routes into state in the kernel or in VPP. That state is shared with
programs Ze does not control, so most mistakes here are about ownership.

## Where it sits

- **Components with a pluggable backend.** `iface`, `firewall` and `traffic`
  each declare a `Backend` interface and a `RegisterBackend`/`LoadBackend` pair
  (`internal/component/{iface,firewall,traffic}/backend.go`). The backend
  plugins live under `internal/plugins/{iface,firewall,traffic}/`: netlink or
  nft for Linux, `vpp` for VPP. The YANG `backend` leaf selects one
  (`docs/architecture/core-design.md`, sections 14 and 14b).
- **The route path.** Protocols insert paths into the shared Loc-RIB.
  `sysrib` selects the system best and publishes `(system-rib, best-change)`.
  One FIB plugin writes it: `fib-kernel` for netlink, `fib-vpp` for VPP,
  `fib-p4` with no data plane yet (`ai/digests/fib-programming.md`,
  `docs/architecture/forked-route-install.md`). `rib { fib-withhold }` keeps a
  protocol out of the FIB and does not change selection
  (`internal/component/sysrib/fibimport.go`).
- **Supporting components.** `vpp` supervises the VPP process and its GoVPP
  connection (`ai/digests/vpp-dataplane.md`). `sysctl` owns every kernel
  tunable, with config over transient over plugin defaults
  (`internal/component/sysctl/sysctl.go`). `mtu` serves the read-only `show
  mtu` diagnostic (`docs/architecture/diagnostics/path-mtu.md`).
  `trafficfeature` derives traffic signals from the observation feed for
  detection plugins (`internal/component/trafficfeature/doc.go`). `policyroute`
  registers firewall tables and programs `ip rule` routes
  (`docs/architecture/policyroute/policy-routing.md`).
- **Compile-out.** `ze_vpp` removes `component/vpp` and every `vpp` backend,
  `ze_policyroute` removes policy routing (`feature-gates.txt`). The netlink
  and nft paths are always on.

## Who depends on it from outside

| Outside party | What they hold | The contract |
|---------------|----------------|--------------|
| The operator | `interface`, `firewall`, `traffic-control`, `fib`, `rib` and `sysctl` config | The YANG schema. A node a backend cannot implement carries `ze:backend`, and the commit refuses it with the YANG path (`core-design.md`, section 14a) |
| The Linux kernel | Links, addresses, routes, nft tables, qdiscs, AF_MPLS entries | Ze owns only what it marked. It owns routes by `rtm_protocol` (`internal/core/rtproto`, `FIBKernel` 250, `Static` 251) and nft tables by the `ze_` name prefix (`docs/architecture/firewall/table-ownership-and-shutdown-flush.md`, Rule 2). It also owns the tc qdisc it replaced, and restores it from a persisted snapshot (`docs/architecture/traffic/tc-original-qdisc-restore.md`) |
| Other routing daemons and the kernel itself | Routes, DHCP leases, SLAAC and link-local addresses on the same box | `fib-kernel` watches for external changes on a prefix Ze manages and reasserts only what it owns (`internal/plugins/fib/kernel/monitor.go`, `handleExternalChange`). A reconciler that reads "not in config" as "mine to delete" breaks them (Traps) |
| VPP | Its own FIB, interfaces, ACLs and policers | GoVPP binary API, stats and CLI sockets (`ai/digests/vpp-dataplane.md`). VPP semantics differ from Linux, so a backend that cannot match the config exactly refuses it (`ai/rules/architecture.md`, `ai/rationale/exact-or-reject.md`) |
| A third-party route producer | A plugin that wants its routes forwarded | The `ze-plugin-engine:route-install` RPC, which refuses a protocol name the engine has not registered (`internal/component/plugin/server/dispatch_route.go`, `resolveProtocol`), or BGP injection (`docs/guide/route-injection.md`) |

## Boundaries

- **One writer per data plane.** A FIB plugin declares `ProgramsFIB` and the
  `DataPlane` it reaches. A route producer declares `NeedsDataPlane`
  (`internal/component/plugin/registry/registry.go`, `Registration`).
  `Register` refuses a second claim on one data plane. The engine loads
  the writer for the configured backend, unless a `fib { }` block already chose
  one (`internal/component/plugin/server/startup_autoload.go`,
  `appendDataPlaneWriter`). Never add a list of FIB plugin names anywhere.
- **Protocols never write the kernel.** They insert `locrib.Path` values. A
  forked plugin ships them over the route-install RPC because `locrib.Default()`
  is nil in a subprocess (`docs/architecture/forked-route-install.md`). The one
  exception is a static route in a named table, which keeps its direct write
  (`docs/architecture/static-routes.md`). MPLS label entries reach the kernel
  only as `(mpls-fib, entry)` events that `fib-kernel` consumes
  (`internal/core/mplsfib/events.go`).
- **Exact or refuse, at verify.** Feature support is declared with `ze:backend`
  in YANG and checked by `config.validateBackendFeatures` from `OnConfigure` and
  `OnConfigVerify` (`core-design.md`, section 14a). A semantic limit the
  annotation cannot express goes in a backend verifier
  (`internal/component/traffic/backend.go`, `RegisterVerifier`). A runtime
  `errNotSupported` is defence in depth, not the gate.
- **Interface names go through the shared resolver**, never straight to the
  kernel (`docs/architecture/iface/logical-name-resolution.md`). Interface
  events reach consumers on the EventBus. BGP and `iface` do not import each
  other (`core-design.md`, section 14).
- **The firewall table registry is process-local.** Every nft table producer
  runs in process (`table-ownership-and-shutdown-flush.md`). A producer that
  can run forked needs an RPC, not a call to `RegisterTables`.
- **The report bus is process-local too.** `internal/core/report` is a package
  store with no RPC in `pkg/plugin/rpc`, so a warning a forked plugin raises
  never reaches `show warnings` (`core-design.md`, section 15).
- **Kernel tunables go through `sysctl`.** A plugin contributes a default over
  the EventBus (`internal/component/sysctl/events`). It does not write `/proc`.

## Read before code

| Read | Question it answers |
|------|---------------------|
| `docs/architecture/core-design.md`, sections 14, 14a, 14b, 15, 16 | Backend split, the commit-time capability gate, firewall and traffic backends, the report bus, the FIB pipeline |
| `docs/architecture/forked-route-install.md` | How a route from a forked producer reaches sysrib |
| `docs/architecture/static-routes.md` | Which static routes go through the Loc-RIB and which write the kernel |
| `docs/architecture/mpls/mpls-kernel.md` | How label push, swap and pop share the FIB, and the two MPLS test traps |
| `docs/architecture/iface/logical-name-resolution.md` | How a configured interface name becomes a device |
| `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` | Which nft tables Ze is allowed to reconcile and delete |
| `docs/architecture/traffic/fw-7-traffic-vpp.md`, "Exact or reject" | What a VPP approximation looks like and why it is refused |
| `docs/guide/vpp.md`, `docs/guide/firewall.md`, `docs/guide/traffic-control.md` | What the operator writes |
| `ai/digests/iface.md`, `fib-programming.md`, `vpp-dataplane.md`, `firewall.md` | The current code flow. `fib-programming.md` still says static routes bypass sysrib. `static-routes.md` is correct |
| `ai/patterns/plugin.md`, `ai/patterns/config-option.md` | The shape of a new backend plugin or YANG leaf |
| `ai/rules/platform-linux.md` | QEMU integration tests, `option=needs-linux`, `caps=` markers |
| `ai/rules/config.md` | YANG, env vars and config reads |

## Traps

- **A reconciler deleted state another owner installed.** `iface` removed DHCP
  leases, SLAAC addresses and the kernel link-local on every commit
  (`plan/journal/reconciler-claims-state-another-owner-installed.md`).
- **The kernel refused what the installer sent.** Stock Linux rejects
  `RTA_METRICS` on AF_MPLS, so no RSVP-TE transit LSP came up. The fix probes
  the kernel first (`plan/journal/kernel-refuses-what-the-installer-sends.md`).
- **VPP shipped five silent approximations**, including policers that run in
  series on the feature arc (`docs/architecture/traffic/fw-7-traffic-vpp.md`).
- **VPP reports addresses but no DAD outcome.** `ip_address_details` carries
  no DAD state, so the VPP backend answers `DADReported` false, and an address
  without `Tentative` or `DADFailed` there is not a unique one
  (`plan/journal/zero-value-as-valid-answer.md`, `detailsToInfo`).
- **Kernel FIB tests passed where the install is refused.** Six `.ci` tests
  had no capability marker and asserted only a log pattern
  (`plan/journal/green-that-could-not-have-been-red.md`).
- **A unit test that drives the FIB backend directly proves nothing about the
  daemon**, and an AF_MPLS entry that reads back proves nothing about
  forwarding (`docs/architecture/mpls/mpls-kernel.md`, both "Trap" sections).
- **A reconcile ran against a config the data plane never received** while a
  VPP apply was still owed (`plan/journal/reconciler-repairs-against-a-stale-snapshot.md`).

## Gates

| Command | What it judges |
|---------|----------------|
| `./le test unit plugins`, `./le test unit core` | Race-instrumented unit groups |
| `./le test integration iface`, `fib`, `firewall`, `traffic` | The netlink, nft and tc backends against a real kernel. It needs `CAP_NET_ADMIN` |
| `./le test qemu all-tests` | Every Linux integration package inside Ze's kernel |
| `./le test vpp`, `./le test firewall`, `./le test traffic`, `./le test policy`, `./le test static` | Functional `.ci` suites |
| `./le arch iface-resolution` | No code resolves an interface name against the kernel directly |
| `./le plugin boundary check` | No plain in-process call into engine state |
| `./le arch tier check` | Placement and import direction |
