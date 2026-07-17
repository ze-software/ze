# Ze Functional Test System

## Overview

Functional tests exercise release-gate behavior across BGP wire encoding and decoding, plugin behavior, config parsing, reloads, UI/editor flows, managed config, L2TP, firewall, policy routing, LDP, RSVP-TE, IS-IS, OSPF, OSPFv3, web UI, and install flows.

> For how the runner schedules and executes tests (the three execution engines, concurrency, reporting) and the web `.wb` test format, see [`architecture/testing/runner-architecture.md`](architecture/testing/runner-architecture.md).

```bash
# Quick start
make ze-functional-test   # Run all gating suites
make ze-encode-test       # Encoding tests only
make ze-plugin-test       # Plugin tests only
make ze-reload-test       # Reload tests only
```

## Release Gate Coverage

`make ze-verify` runs, in order: `ze-lint`, `ze-verify-wiring-docs`,
`ze-vet-evidence`, `ze-unit-test-cached`, `ze-unit-test-race-changed`,
`ze-functional-test`, and `ze-exabgp-test`. `make ze-verify-changed` uses the
changed-only lint and unit stages, then the same wiring, functional, and ExaBGP
stages. Both targets run under `scripts/dev/verify-lock.sh`, continue across
top-level stage failures, and write:

| Artifact | Purpose |
|----------|---------|
| `tmp/ze-verify.log` | Combined full verify log |
| `tmp/verify/<nn>-<stage>.log` | Full log for one stage |
| `tmp/ze-verify-failures.log` | Compact failure index to read first |
| `tmp/ze-verify-failures.json` | Machine-readable failure routing index |
| `tmp/ze-verify.status` | Freshness fingerprint for the last run |

The functional test target runs 22 suites: encode, plugin, parse, decode, reload,
ui, editor, managed, l2tp, firewall, policy, ldp, rsvpte, isis, ospf, ospfv3, web,
install, appliance, l2tp-wire, isis-wire, ospf-wire.

`make ze-validate` is a fast (~0.2s) post-verify check that catches recurring
implementation mistakes: stale source anchors, line-number anchors, unwired
exported symbols, and incomplete spec AC tables. Run it after `ze-verify` passes,
before presenting work as complete.
<!-- source: Makefile -- ze-verify, ze-verify-changed, ze-exabgp-test -->
<!-- source: scripts/dev/validate.py -- post-verify validation checks -->
<!-- source: scripts/status/verify_run.go -- artifact writing and grouped summaries -->
<!-- source: scripts/dev/verify-lock.sh -- verify-class lock -->
<!-- source: scripts/dev/verify-status.sh -- tmp/ze-verify.status -->
<!-- source: mk/test-functional.mk -- ze-functional-test suite list -->

The following shipped test suites are **not in the default release gate** and
must be run manually:

| Suite | Runner | Why not gated |
|-------|--------|---------------|
| Static routes | `bin/ze-test static --all` | Separate route-installation fixture |
| Traffic control | `bin/ze-test traffic --all` | Requires traffic-control platform support |
| Flow export | `bin/ze-test flow-export --all` | Requires Linux packet-sampling support for release evidence |
| VPP | `bin/ze-test vpp --all` | Requires Python VPP stub setup |
| L2TP wire | `bin/ze-test l2tp-wire --all` | Wire-level fixture separate from release-gate L2TP daemon scenarios |
| OSPFv2 wire | `bin/ze-test ospf-wire --all` | Wire-level codec fixture separate from release-gate OSPF runtime scenarios |
| IS-IS wire | `bin/ze-test isis-wire --all` | Wire-level codec fixture separate from release-gate IS-IS runtime scenarios |
| BGP interop | `python3 test/interop/run.py [scenario]` | Requires Docker peer daemons and image builds |
| IPsec interop | `make ze-ipsec-interop-test` or `python3 test/ipsec-interop/run.py [scenario]` | strongSwan peer via Docker (privileged). Ze as initiator (`01`-`05`) and responder (`07-responder-psk`, `08-responder-eap-mschapv2`, `09-responder-ike-rekey`). Dataplane checks gate on XFRM availability; control plane verified from strongSwan logs. |
| Chaos web | `bin/ze-test bgp chaos-web --all` | Chaos dashboard scenarios live under the BGP runner |

These suites also have `make` targets: `make ze-static-test`,
`make ze-traffic-test`, `make ze-flow-export-test`, `make ze-vpp-test`,
`make ze-l2tp-wire-test`, `make ze-isis-wire-test`, and `make ze-ospf-wire-test`.
Chaos web is available through
`make ze-chaos-web-test` and is also included in `make ze-chaos-test`.
<!-- source: mk/test-functional.mk -- non-gated functional targets and ze-functional-test suite list -->

Linux-tagged Go unit tests are separate from the functional suites. From a
non-Linux workstation, use `make ze-linux-test`; it runs the default Linux-only
unit package set in Docker and can be narrowed or expanded with
`ZE_LINUX_TEST_PACKAGES="./pkg/a ./pkg/b"`.

Clean release-candidate evidence can be run with `make ze-release-check`.
The target refuses a dirty worktree, clones the repository into an ephemeral
Docker container, mirrors the Woodpecker dependency setup, and runs
`make ze-verify` there.

`ze-deployment-*` targets are deployment-grade external evidence. Some use
Docker because the local runner needs a Linux system service; Docker-specific
targets say so in the target name.

Real VPP daemon evidence is separate from the stub-backed VPP functional suite:
`make ze-deployment-vpp-test` starts VPP in a privileged Docker container and
proves `fib-vpp` add and withdraw against VPP's real FIB.

External L2TP peer evidence is separate from the in-tree L2TP fixture suite:
`make ze-deployment-l2tp-test` starts Ze and a real `xl2tpd` LAC in a
privileged Docker container, then proves the control tunnel and incoming-call
session are established. Full PPP/NCP/kernel dataplane peer evidence is a
separate Linux-only target: `make ze-deployment-l2tp-ppp-test` creates Ze and
LAC network namespaces joined by a veth underlay, then requires `xl2tpd`,
`pppd`, `ping`, `/dev/ppp`, `iproute2`, and PPPoL2TP kernel support.
`make ze-deployment-l2tp-ppp-docker-test` runs a peer-isolated Docker lab
with Ze LNS, a real `xl2tpd`/`pppd` LAC, and FRR as a BGP peer in separate
containers on an isolated Docker bridge. It proves PPP LCP/IPCP, kernel
`pppN` interface creation, dataplane ping, and BGP route redistribution from
a live PPP session. The lab requires Docker and the host kernel (or Docker VM
kernel) to have PPPoL2TP support; it refuses to run if `/dev/ppp`, `ip l2tp`,
or the `l2tp_ppp`/`pppol2tp` module is missing. Run individual scenarios with
`python3 test/l2tp-interop/run.py <scenario-name>`.

`make ze-deployment-pppoe-accel-docker-test` runs the inverse-role PPPoE lab:
Ze as a PPPoE **client** (`pppoe-client` interface kind) against a real
[accel-ppp](https://accel-ppp.org/) access concentrator (the Alpine `accel-ppp`
package). It proves PADI/PADO/PADR/PADS discovery, LCP, CHAP-MD5 auth, IPCP
address assignment, the kernel `pppN` interface, dataplane ping to the AC
gateway, accel-ppp's session view, and teardown. It is the only test exercising
Ze as the PPPoE client (the `test/pppoe/*.ci` tests run Ze as the server). The
Docker form needs host-kernel `/dev/ppp` + `pppoe`; on macOS or any host without
it, `make ze-qemu-pppoe-accel-test` runs the same proof in a QEMU netns
(`scripts/evidence/effective-pppoe-accel.py`) using the runtime kernel built by
`make ze-kernel`. See `docs/labs/pppoe-interop.md`.
`make ze-qemu-vrrp-keepalived-test` runs the VRRP interop lab: Ze's VRRP against
a real [keepalived](https://keepalived.org/) (the Alpine `keepalived` package,
v2.3.1) on one L2 segment. Three network namespaces (Ze, keepalived, and a
passive observer) are bridged in a fourth, so the observer sees flooded
multicast and can prove the virtual IP moves at layer 2 and not merely in a log.
It proves RFC 9568 VRRPv3 election, failover on node death, the prio-0 graceful
stop, and that keepalived accepts Ze's advert format. Unlike the L2TP and PPPoE
labs it needs no `make ze-kernel`: the stock Alpine kernel already provides
macvlan, bridge and veth (probed 2026-07-15), so the target runs standalone.
Keepalived state is read from its `notify_*` markers and every timing assertion
is measured from tcpdump wire timestamps against the acceptance bands in
`plan/spec-vrrp-6-interop.md`, never from wall clock.

The Linux-only VRRP runtime `.ci` tests carry `option=needs-linux`: they SKIP on
darwin (the `interface` plugin cannot manage devices there, which cascades into
the vrrp plugin failing at stage Init) and run as root under
`make ze-qemu-needs-linux-test`, where `test/vrrp` is in the QEMU suite list.

<!-- source: internal/test/cli/register.go -- subcommand registry -->
<!-- source: internal/test/cli/cmd_bgp.go -- chaos-web suite -->
<!-- source: Makefile -- ze-linux-test -->
<!-- source: scripts/evidence/effective-verify.sh -- clean Docker ze-verify evidence -->
<!-- source: scripts/evidence/effective-l2tp-ppp.py -- full L2TP PPP/NCP peer evidence -->
<!-- source: scripts/evidence/effective-vrrp-keepalived.py -- VRRP vs keepalived interop evidence -->
<!-- source: mk/test-integration.mk -- ze-qemu-vrrp-keepalived-test -->
<!-- source: scripts/evidence/qemu-all-tests.sh -- vrrp suite in the QEMU needs-linux run -->

---

## Quick Start

```bash
# List available tests with run number, total, one-based id, and name
ze-test bgp encode --list
ze-test ui --list

# Run specific tests by id or exact name
ze-test bgp encode 4 5 6
ze-test ui 4

# Run all tests
ze-test bgp encode --all
ze-test ui --all

# Resume a suite after a timeout or interrupted run
ze-test bgp plugin --start 42
ze-test editor --start 42

# Stress test selected tests
ze-test bgp encode --count 10 1 2
```
<!-- source: internal/test/cli/cmd_bgp.go -- parseRunCLI, printRunUsage -->
<!-- source: internal/test/cli/ci_runner.go -- runCISubcommand common options -->
<!-- source: internal/test/cli/cmd_editor.go -- editorMain common options -->

---
## Functional Suite Inventory

Every `ze-test` functional suite uses the same selection contract after the suite
name: `--list`, `--all`, `--start ID`, `--pattern TEXT`, or positional
`ID_OR_NAME...`. `--list` prints each test as `N/TOTAL ID NAME`; `ID` is
one-based, so the full-suite progress number and runnable id match. Each run
prints one completion line per test with elapsed time, `N/TOTAL`, result, id,
and name, plus periodic progress while tests are still running.
<!-- source: internal/test/runner/selection.go -- Selection -->
<!-- source: internal/test/runner/display.go -- Status, TestFinished -->
<!-- source: internal/test/cli/cmd_web.go -- webMain parallel via ParallelRunner -->

| Suite | Command | Files | How it works |
|-------|---------|-------|--------------|
| Encode | `ze-test bgp encode` | `test/encode/*.ci` | Builds Ze and `ze-peer`, starts peers, then checks BGP wire output from configured routes. |
| Plugin | `ze-test bgp plugin` | `test/plugin/*.ci` | Runs Ze with embedded process/API fixtures, injects commands or plugin events, then checks BGP, stdout/stderr, syslog, HTTP, or file expectations. |
| Parse | `ze-test bgp parse` | `test/parse/*.ci` | Runs foreground config validation commands and checks exit code plus stdout/stderr expectations. |
| Decode | `ze-test bgp decode` | `test/decode/*.{ci,test}` | Feeds BGP message bytes to decode commands and compares JSON output with volatile fields normalized. |
| Reload | `ze-test bgp reload` | `test/reload/*.ci` | Starts Ze, rewrites config, sends SIGHUP, then checks post-reload behavior. |
| UI | `ze-test ui` | `test/ui/*.ci` | Runs foreground CLI commands and checks terminal output and exit status. |
| Editor | `ze-test editor` | `test/editor/**/*.et` | Runs headless editor keystroke scripts through the CLI testing harness. |
| Managed | `ze-test managed` | `test/managed/*.ci` | Exercises managed config, hub, auth, and fleet workflows through `.ci` process tests. |
| L2TP | `ze-test l2tp` | `test/l2tp/*.ci` | Runs L2TP control-plane scenarios over loopback UDP with fake test plugins where needed. |
| Firewall | `ze-test firewall` | `test/firewall/*.ci` | Exercises firewall configuration and daemon behavior through `.ci` process tests. |
| Policy | `ze-test policy` | `test/policy/*.ci` | Exercises policy-routing configuration and daemon behavior through `.ci` process tests. |
| Web | `ze-test web` | `test/web/*.wb` | Runs `.wb` browser scripts in parallel (cap 4) with per-test Ze daemon and isolated `agent-browser --session`. |
| Install | `ze-test install` | `test/install/*.ci` | Exercises offline install command and installer helper behavior. |
| Static | `ze-test static` | `test/static/*.ci` | Exercises static route installation and reload add/remove behavior. |
| Traffic | `ze-test traffic` | `test/traffic/*.ci` | Exercises traffic-control configuration and daemon behavior. |
| Flow export | `ze-test flow-export` | `test/flow-export/*.ci` | Exercises sFlow, NetFlow, and IPFIX export behavior. |
| VRRP | `ze-test vrrp` | `test/vrrp/*.ci` | Exercises the vrrp YANG augment under interface units, the plugin's cross-leaf verifier (mandatory vrid and its 1..255 range, duplicate vrid or virtual-address per unit+family, the operator-assigned priority 255 rejection, the version-dependent interval encodings, accept-mode as VRRPv3-only, the IPv6 first-address link-local rule, and the VPP backend rejection), and the doctor/explain surface. Each command asserts its own exit code via `cmd=...:exit=N`, because `expect=exit:code=` only ever reaches a file's last quick-exit `ze` command. Tests that boot a daemon carry `option=needs-linux` and run only under QEMU. |
| VPP | `ze-test vpp` | `test/vpp/*.ci` | Runs stub-backed VPP scenarios and checks the stub request log. |
| L2TP wire | `ze-test l2tp-wire` | `test/l2tp-wire/*.ci` | Exercises L2TP wire-level encode/decode and malformed-packet handling. |
| IS-IS wire | `ze-test isis-wire` | `test/isis-wire/*.ci` | Exercises IS-IS wire-level decode and malformed-PDU handling. |
| OSPFv2 wire | `ze-test ospf-wire` | `test/ospf-wire/*.ci` | Exercises OSPFv2 packet/LSA wire-level decode and malformed-packet handling. |
| OSPF | `ze-test ospf` | `test/ospf/*.ci` | Exercises release-gate OSPF config validation, interface ISM config leaves including passive and loopback records, NSM config leaves including `mtu-ignore`, LSDB flooding/retransmit/purge logic, SPF route installation via Loc-RIB/sysrib ECMP membership updates, inter-area ABR Type 3/4 summary origination, area ranges, summary withdraw, border-router snapshots, daemon route snapshot wiring, admin-distance arbitration, and raw-socket doctor diagnostics, plus the RFC 5250 opaque carrier, RFC 3630/5392 Traffic Engineering, the RFC 7770 Router Information LSA (`ospf-ri-*.ci`, `ospf6-ri-originate.ci`), and the RFC 7684 Extended Prefix/Link Opaque LSAs (`ospf-ext-register.ci`, `ospf-ext-prefix-originate.ci`, `ospf-ext-link-originate.ci`, `ospf-ext-prefix-receive.ci`, `ospf-ext-subtlv-hook.ci`, `ospf-ext-decode.ci`), with FRR interop scenarios (`ospf-ri-frr`, `ospf6-ri-frr`, `ospf-ext-prefix-link-frr`) run under QEMU. |
| Chaos | `ze-test bgp chaos` | `test/chaos/*.ci` | Runs Ze plus chaos peers end-to-end through the BGP `.ci` runner. |
| Chaos web | `ze-test bgp chaos-web` | `test/chaos-web/*.ci` | Runs chaos dashboard HTTP endpoint checks through the BGP `.ci` runner. |
| ExaBGP compatibility | `ze-test exabgp` | `test/exabgp-compat/encoding/*.ci` | Runs the ExaBGP compatibility fixtures through the Go `ze-test` runner, starts the mock BGP peer, runs the ExaBGP wrapper client, and checks the expected wire output. |
<!-- source: internal/test/cli/cmd_bgp.go -- BGP suite routing -->
<!-- source: internal/test/cli/ci_runner.go -- shared .ci suites -->
<!-- source: internal/test/cli/cmd_editor.go -- .et suite runner -->
<!-- source: internal/test/cli/cmd_web.go -- .wb suite runner -->
<!-- source: internal/test/cli/cmd_vpp.go -- VPP stub-backed suite runner -->
<!-- source: internal/test/cli/cmd_exabgp.go -- ExaBGP compatibility runner -->

---


## Unit Tests

Unit tests are standard Go `_test.go` files throughout the codebase. Running
all ~349 packages with `-race` takes ~5 minutes. During development, use
component-group targets to test only the area you changed:

| Target | Scope | Approx time |
|--------|-------|-------------|
| `make ze-test-bgp` | `./internal/component/bgp/...` (96 pkgs) | ~1:30 |
| `make ze-test-core` | `./internal/core/...` (26 pkgs) | ~30s |
| `make ze-test-plugins` | `./internal/plugins/...` (44 pkgs) | ~40s |
| `make ze-test-config` | `./internal/component/config/...` (13 pkgs) | ~20s |
| `make ze-test-cli` | `./internal/component/cli/...` (3 pkgs) | ~10s |
| `make ze-test-rest` | Everything not in a named group (~70 pkgs) | ~1:00 |
| `make ze-unit-test` | All packages with `-race` | ~5 min |

All groups run with `-race`.

### Property-based tests (stdlib `testing/quick`)

Invariant checks over generated inputs, using the standard library's
`testing/quick` (no third-party property engine). Each uses a fixed RNG seed and
a bounded iteration count so CI runs are deterministic and cannot time out. They
run in the ordinary unit passes (no build tag):

| Test | Package | Invariant |
|------|---------|-----------|
| `TestListenerConflictProperties` | `internal/component/config` | listener-conflict symmetry, port/family/protocol independence, wildcard dominance, `FindListenerConflict` ↔ pairwise equivalence (conflict is provably NOT transitive; `TestListenerConflictNotTransitive` pins the wildcard counterexample). |
| `TestMigrationRoundTripProperty` | `internal/exabgp/migration` | any valid ExaBGP config → migrate → serialize re-parses as valid Ze config with every neighbor preserved. |
| `TestForwardPoolOrderingProperty` | `internal/component/bgp/reactor` | withdrawals-first stable reorder, supersede-key determinism, malformed-body robustness, and exactly-once delivery under concurrent dispatch (`-race`). |
| `TestFilterChainRandomUpdatesProperty` | `internal/component/bgp/reactor` | `buildModifiedPayload` never panics and only emits well-formed UPDATE bodies over random payloads+ops; `LessOrder` is a strict total order. |
<!-- source: internal/component/config/listener_property_test.go -- TestListenerConflictProperties -->
<!-- source: internal/exabgp/migration/roundtrip_property_test.go -- TestMigrationRoundTripProperty -->
<!-- source: internal/component/bgp/reactor/forward_pool_property_test.go -- TestForwardPoolOrderingProperty -->
<!-- source: internal/component/bgp/reactor/filter_ordered_property_test.go -- TestFilterChainRandomUpdatesProperty -->

### Evidence / release-tier stress and perf tests (build-tagged, out of `ze-verify`)

Resource-heavy, host-dependent tests kept out of the pre-commit gate. Each is
gated by a build tag not in the default set and has a dedicated make target:

| Target | Test | Tag | What it drives |
|--------|------|-----|----------------|
| `make ze-stress-web-test` | `TestWebConcurrentEditStress` (`internal/component/web`) | `stress` | ≥64 concurrent editor sessions mutate+commit one config; asserts no race (`-race`), zero errors, no torn commit. |
| `make ze-stress-fleet-test` | `TestFleetManyClientsPerf` (`cmd/ze/hub`) | `fleetperf` | 128 concurrent managed clients auth + initial-sync against the real managed hub listener (TLS 1.3, `managedMaxConns` cap); records latency p50/p95/max, asserts zero error budget. |
<!-- source: internal/component/web/stress_test.go -- TestWebConcurrentEditStress -->
<!-- source: cmd/ze/hub/fleet_perf_test.go -- TestFleetManyClientsPerf -->
<!-- source: mk/test-integration.mk -- ze-stress-web-test, ze-stress-fleet-test -->

### Privileged kernel-state tests under QEMU (`option=needs-linux`)

`.ci` tests tagged `option=needs-linux` SKIP on non-Linux hosts (GOOS check) and
run as root inside the QEMU VM via `make ze-qemu-needs-linux-test`, where
CAP_NET_ADMIN and real interfaces are available. The `traffic` suite is enrolled
in `scripts/evidence/qemu-all-tests.sh`; `test/traffic/022-boot-qdisc-tc.ci` and
`023-reload-qdisc-tc.ci` assert real `tc qdisc show` kernel state after boot and
after a reload (the check `001`/`002` document as deferred). The chaos iface
fault family (`iface-link-flap`, `iface-addr-remove`) has a netns-scoped
integration test (`//go:build integration && linux`) run via
`make ze-integration-*`, plus the `test/chaos/iface-link-flap.ci` scenario.
<!-- source: scripts/evidence/qemu-all-tests.sh -- fsuite traffic -->
<!-- source: test/traffic/022-boot-qdisc-tc.ci -- needs-linux tc qdisc assertion -->
<!-- source: internal/chaos/peer/simulator_actions_iface_linux.go -- iface fault executor -->

### Netns launch mode for netlink `.ci` suites (host-safe firewall/policy/OSPF)

The netlink functional suites (`firewall`, `policy`, `ospf`, `ospfv3`, and any
suite whose `ze` daemon needs `CAP_NET_ADMIN` for nft/FIB) carry
`option=skip-os:value=darwin` and program real kernel state. Run unprivileged the
nft backend hard-fails; run with caps in the host namespace they reprogram the
operator's real firewall. An **opt-in per-test network-namespace launch mode**
makes them runnable host-safely on Linux:

```bash
make ze-netns-test                                   # firewall policy ospf ospfv3
make ze-netns-test ZE_NETNS_SUITES=firewall          # one suite
```

Requires Linux + `sudo` + `setcap` (libcap) + `nft` (nftables). The target
`setcap cap_net_admin,cap_net_raw,cap_net_bind_service+ep`s `bin/ze` and
`bin/ze-stripped`, then runs each suite under `sudo` with `ZE_TEST_NETNS=1
ZE_TEST_UID=$(id -u) ZE_TEST_GID=$(id -g)`, and asserts the host `nft list tables`
is byte-identical before and after (removing the caps afterward).

How the runner isolates each test (all gated on `ZE_TEST_NETNS`; the default path
is byte-identical for every suite that passes today):

1. Before spawning any child, the per-test goroutine `runtime.LockOSThread()`s and
   enters a fresh named network namespace (`netns.NewNamed`), bringing `lo` up in
   it. `ze`, `ze-peer`, and driver.py are all fork+exec'd from that locked thread,
   so they **inherit the same throwaway netns** (validated by
   `TestNetnsLaunchChildInheritsNamespace`) and reach each other over 127.0.0.1.
2. The runner (root, via `sudo`) creates the netns (`CAP_SYS_ADMIN`) and execs `ze`
   with `SysProcAttr.Credential` set to a normal user, so the **setcap'd `ze` runs
   non-root** with ambient `CAP_NET_ADMIN`. `ze` must not be root: its readiness
   file is written after `dropPrivileges`, so a root `ze` never writes it and the
   `daemon.ready` handshake times out. `ze-peer`/driver.py stay root so they can
   read nft state and signal the daemon.
3. The nft tables `ze` programs live in the per-test netns; on test end the runner
   restores the original netns and deletes the throwaway one. The host firewall is
   never touched.

**R-2 host-safety gate.** As belt-and-braces, a setcap'd `ze` that lands in the
host netns (isolation silently failed) refuses to program nft: the runner passes
the host netns inode via `ZE_TEST_NETNS_HOST`, and `firewallnft.Apply` aborts
before any kernel op if the current netns matches it (`refuseHostNetnsFirewall`).
The env is test-only, so production is unaffected.

**macOS / QEMU.** netns and nft do not exist on macOS; `make ze-netns-qemu-test`
exercises the launch path (child-inherits-netns + R-2 guard unit tests, and a
firewall subset under `ZE_TEST_NETNS`) inside the QEMU Alpine VM.

<!-- source: internal/test/runner/netns_linux.go -- enterTestNetns, testNetnsName -->
<!-- source: internal/test/runner/netns_linux_test.go -- TestNetnsLaunchChildInheritsNamespace (A-5) -->
<!-- source: internal/test/runner/runner_exec.go -- runOrchestrated netns entry + SysProcAttr credential drop -->
<!-- source: internal/plugins/firewall/nft/host_netns_guard_linux.go -- refuseHostNetnsFirewall (R-2) -->
<!-- source: mk/test-integration.mk -- ze-netns-test / ze-netns-qemu-test -->

### In-process integration tests (feeds that can't cross the plugin boundary)

Some chains are driven by `internal/core/observation`, a **process-local** feed
(a `.ci` cannot inject into it, because a config-loaded plugin runs isolated from
the engine's in-engine consumers). These are proven by in-process Go integration
tests that compose the real production types in one process and drive them with
synthetic observations. The anomaly facts→judgment→response chain
(`trafficfeature` → `anomaly/detect` → `anomaly/shape`) is proven this way by
`TestChainFactsToResponse`; it is `-short`-skippable (drives real 1s ticks).
<!-- source: internal/plugins/anomaly/detect/chain_integration_test.go -- TestChainFactsToResponse -->

### Development Workflow

Use the narrowest test scope that covers your change, then widen before committing:

```
single test  →  single package  →  component group  →  ze-verify
```

```bash
go test -race -run TestMyThing ./internal/component/config/system/...  # single test
make ze-test-config                                                     # component group
make ze-verify                                                          # pre-commit gate
```

`make ze-verify` is the pre-commit gate. It runs a two-pass strategy:
1. Cached full pass (all packages, no `-race`; completes quickly when nothing changed)
2. Race pass on changed groups only (detects data races in what you touched)
3. All functional test suites
4. ExaBGP compatibility

On failure, read `tmp/ze-verify-failures.log` first. It is a compact routing
index with one block per failing stage and group. Each group names its `Rerun`
command, `Detail log`, and `Parallel` value, so a reader can choose related
failures without opening the full combined log. Open the referenced
`tmp/verify/<nn>-<stage>.log` for full evidence for that group. Use
`tmp/ze-verify.log` only when the whole combined run is needed.
Automation should read `tmp/ze-verify-failures.json`.
<!-- source: Makefile -- ze-test-bgp, ze-test-core, ze-test-plugins, ze-test-config, ze-test-cli, ze-test-rest -->
<!-- source: scripts/status/verify_run.go -- stage logs, compact index, JSON index -->
<!-- source: mk/test-unit.mk -- ze-unit-test-cached, ze-unit-test-race-changed -->

---

## RFC Requirement Tags

A test that enforces an RFC 2119 MUST-level requirement tags itself, so the
coverage gate can bind the requirement to the proof that enforces it. The tag
names a stable requirement id (allocated in `rfc/short/*.md`, e.g.
`RFC7606-7.1-1` is §7.1, first requirement) and a mandatory polarity, `positive`
or `negative`. In Go tests the tag is a `//` comment; place it on the function
doc comment when the function tests one requirement, or inline at the table case
when one function covers many:

```go
// RFC requirement: RFC7606-7.1-1 positive -- valid ORIGIN length 1 is accepted
func TestRFC7606OriginValueIGP(t *testing.T) {

// RFC requirement: RFC7606-7.1-1 negative -- ORIGIN length 2 is treated as withdraw
func TestRFC7606MalformedOriginLength(t *testing.T) {
```

In a `.ci` file the tag is a line-start `#` comment with the same fields, and
must not sit inside a `terminator=` block (there `#` is raw file content, not a
comment):

```
# RFC requirement: RFC7606-7.1-1 negative -- malformed ORIGIN withdraws the route
```

Rules the gate enforces:

- **Polarity is mandatory and never inferred.** A tag with no `positive` or
  `negative` word fails to parse.
- **Every gated MUST needs BOTH polarities.** A negative-only test passes if the
  code rejects everything; a positive-only test passes if it accepts everything.
  Only the pair pins behavior to the requirement. A requirement that is genuinely
  testable only one way carries a `{single-polarity: positive|negative; why}`
  annotation on its summary line instead.
- **`make ze-rfc-check` is the gate.** For every MUST-level requirement of an
  enrolled RFC (`rfc/enrolled.txt`) it requires the positive/negative pair, or a
  reasoned `{gap}` / `{not-applicable}` / `{single-polarity}` annotation. It scans
  Go `_test.go` files and `.ci` files under `internal/`, `pkg/`, and `test/`, and
  `make ze-rfc-index` renders the requirement to test ledger into
  `ai/RFC-REQUIREMENTS.md`.
- **Do not edit a tagged test to make it pass.** Once a test carries an
  `RFC requirement:` tag its behavior may not change without explicit user
  approval, recorded as `// rfc-test-change-approved: <date> <what and why>`. Fix
  the code instead; the `rfc-tagged-test` hook blocks the edit otherwise.

<!-- source: scripts/dev/rfc_requirements.py -- scan_go_tags/scan_ci_tags -->

Full authoring guidance: `ai/skills/ze-rfc.md` (id allocation and annotations),
`ai/skills/ze-rfc-audit.md` (letter-and-spirit audit), and
`docs/contributing/rfc-implementation-guide.md`.

---

## Test Types

### 1. Encode Tests (`test/encode/`)

Static route tests - routes defined in config, sent at session establishment.

**Files:**
- `*.ci` - Expected messages and config reference
- `*.conf` - Ze configuration

### 2. Parse Tests (`test/parse/`)

Config parsing tests verify configurations parse correctly.

**Files:** All parse tests use `.ci` format with embedded config.

Parse coverage configs live in `test/parse/coverage-*.ci`. They are positive
parse/validate coverage for realistic multi-feature configurations, such as
IXP peering, large peer sets, RPKI policy, and redistribution. Run a specific
coverage case by name, for example `bin/ze-test bgp parse coverage-ixp-peering`.

BGP interop scenarios live under `test/interop/scenarios/NN-name/` and run with
`python3 test/interop/run.py NN-name`. Scenario files are written with the
default `172.30.0.x` lab prefix, then copied to `tmp/interop-rendered/` and
rewritten to an available `/24` before containers start. The default slot is
`172.30.0.0/24`; concurrent runs retry on `172.30.1.0/24`, `172.30.2.0/24`, and
so on. Use `ZE_INTEROP_SUBNET_INDEX=N` or `ZE_INTEROP_SUBNET_PREFIX=A.B.C.` to
force a specific lab prefix. A scenario that includes `bmp-collector.py` gets a
pre-started collector sidecar on the run's `.6` address so Ze's internal BMP
sender can connect before peer events are generated. A scenario that includes
`rpki-server` starts `ze-test rpki --bind 0.0.0.0` on the run's `.7` address.

**Positive tests** (expect success):
```
# test/parse/simple-v4.ci
stdin=config:terminator=EOF_CONF
bgp {
    peer test-peer {
        remote {
            ip 127.0.0.1;
            as 65533;
        }
        router-id 10.0.0.2;
        local-as 65533;
    }
}
EOF_CONF

cmd=foreground:seq=1:exec=ze bgp validate -:stdin=config
expect=exit:code=0
```

**Negative tests** (expect failure):
```
# test/parse/route-refresh-no-process.ci
stdin=config:terminator=EOF_CONF
bgp {
    peer test-peer {
        remote {
            ip 10.0.0.1;
            as 65002;
        }
        router-id 1.2.3.4;
        local-as 65001;
        capability { route-refresh; }
    }
}
EOF_CONF

cmd=foreground:seq=1:exec=ze bgp validate -:stdin=config
expect=exit:code=1
expect=stderr:contains=route-refresh requires process with send { update; }
```

### 3. API Tests (`test/api/`)

Dynamic route tests - routes injected via scripts using the process API.

**Files:**
- `*.ci` - Expected messages and config reference
- `*.conf` - Ze configuration (includes `process` block)
- `*.run` - Script that sends API commands

### 3b. MCP Tests (`test/plugin/mcp-*.ci`, `test/plugin/elicitation-*.ci`)

End-to-end scenarios for the MCP transport. The runner launches a ze
daemon with `--mcp <port>` in the background and `ze-test mcp` in the
foreground; assertions come from `expect=exit:code=...` and
`expect=stdout|stderr:contains=...`.

| File | What it covers |
|------|----------------|
| `mcp-announce.ci` | `ze_execute` dispatches a BGP UPDATE via the MCP endpoint, `ze-peer` verifies the wire bytes |
| `elicitation-accept.ci` | Client declares `capabilities.elicitation={}`, queues an accept reply, `ze_execute` with empty command triggers `elicitation/create` over SSE, accepted command is dispatched |
| `elicitation-decline.ci` | Same setup, queued decline surfaces as a tool error containing "declined" |
| `elicitation-no-capability.ci` | Client does NOT declare the capability; the server fails fast with "missing required argument" instead of hanging on an elicit |

The `ze-test mcp` client understands these extra stdin directives for
elicitation scenarios: `elicit-accept <json>`, `elicit-decline`,
`elicit-cancel`. Each queues one reply; the client auto-cancels when an
elicit frame arrives with nothing queued.

### 3c. Task Tests (`test/plugin/task-*.ci`)

Task-augmented `tools/call` scenarios using the `--tasks` flag on
`ze-test mcp`. The client declares `capabilities.tasks={}` at
initialize and uses task-specific directives.

| File | What it covers |
|------|----------------|
| `task-rib-routes.ci` | `show bgp rib` as a task: createTask -> poll -> tasks/result returns route data |
| `task-cancel.ci` | Create a long-running task, cancel it, verify state is cancelled |
| `task-forbidden.ci` | `clear bgp rib in` (forbidden) with `task:{}` is rejected with -32602 |
| `task-identity-scope.ci` | Two bearer-list identities each create tasks; `tasks/list` returns only own tasks |

Task directives: `task-call <tool> <args>`, `task-get <id>`,
`task-result <id>`, `task-cancel <id>`, `task-list`,
`task-wait <id> <state>`. `$LAST` substitutes the most recent directive
output (typically the taskId from `task-call`).

### 3d. GET-Stream Tests (`test/plugin/mcp-get-sse.ci`)

Server-initiated frames over the standalone `GET /mcp` SSE stream (MCP
2025-06-18 Streamable HTTP). `sse-listen` opens the GET stream
(`Accept: text/event-stream` + `Mcp-Session-Id`) on a background reader;
`sse-expect <method>` blocks until a server-initiated frame with that
JSON-RPC method arrives and prints it. `mcp-get-sse.ci` drives a
`task-call` whose completion notification (`notifications/tasks/status`)
is delivered on the GET stream, not the POST response.
<!-- source: internal/test/cli/cmd_mcp.go -- startSSE/sseExpect directives -->

### Forward-Path Claims

Claims about forwarding, per-destination egress filters, or wire-visible
re-advertisement must drive a real forward path. In practice that means the
`.ci` file must load a plugin that calls `ForwardUpdate()` or
`ForwardUpdatesDirect()` (for example `bgp-rs`) and then assert on the
destination peer with `expect=bgp:...` or another deterministic wire-visible
signal.

A two-peer setup by itself is not evidence. If no forwarding-capable plugin is
loaded, a destination `ze-peer` may establish while no egress filter ever runs.
Those files must be marked `partial` or `blocked` instead of claiming full wire
coverage.

There is also a known single-`ze-peer` multi-IP timing limitation for some
multi-destination scenarios. When a test needs one `ze-peer` process to keep
multiple local-IP sessions alive long enough for deterministic wire assertions
and that timing remains flaky, the file should name that exact blocker and stay
`partial`/`blocked` until the fixture support exists.

### Test-Only Internal Plugins (`internal/test/plugins/`)

Some `.ci` tests need a synthetic Go-side producer to drive features whose
real producer requires privileged or external infrastructure. `fakel2tp`, for
example, drives L2TP route-change events without requiring kernel L2TP, PPP,
or a peer daemon. These plugins live under `internal/test/plugins/<name>/`
and are loaded only by DUT builds compiled with the `zetest` tag. Normal
production builds do not import `internal/test/plugins/all` and do not expose
these registry entries.

First occupant: `internal/test/plugins/fakeredist/`. Pattern:

| File | Role |
|------|------|
| `fakeredist.go` | Package state, command parser, batch builder/emitter |
| `store.go` | Current-set tracking + `reemitAll` for redistribute late-join replay |
| `register.go` | Plugin registration + `OnExecuteCommand` dispatcher + `ReplayRequest` subscription |
| `fakeredist_test.go` | Unit tests for the command surface |

fakeredist tracks the routes it has emitted (`store.go`) and subscribes to
`redistevents.ReplayRequest`, re-emitting its current live set tagged with the
echoed `ReplayID` so the late-join `.ci` tests (`redistribute-late-join*.ci`) can
drive a peer-up replay in-process, the same way the real producers (static,
connected, l2tp) do.

The aggregator at `internal/test/plugins/all/all.go` blank-imports every
test-only internal plugin. `cmd/ze/plugins_zetest.go` imports that aggregator
only under the `zetest` build tag so functional tests can exercise fakeredist
and fakel2tp without polluting normal `cmd/ze` builds.

<!-- source: internal/test/plugins/fakeredist/register.go -- pattern reference -->
<!-- source: internal/test/plugins/all/all.go -- test-only aggregator -->
<!-- source: cmd/ze/plugins_zetest.go -- zetest-only import -->

### 4. Reload Tests (`test/reload/`)

Config reload tests - verify SIGHUP-triggered reload behavior end-to-end.

**Files:** All reload tests use `.ci` format with embedded config and tmpfs alternate configs.

**How they work:**
1. Daemon starts with initial config, establishes BGP session
2. Test peer verifies initial messages
3. `action=rewrite` replaces config file with alternate version in tmpfs
4. `action=sighup` sends SIGHUP to daemon PID (read from `daemon.pid` in tmpfs)
5. Daemon reloads config — peers restart if settings changed
6. Test peer verifies reconnection and new messages

**Example:**
```
# Initial config establishes session with one route
stdin=ze-bgp:terminator=EOF_CONF
bgp { peer loopback { remote { ip 127.0.0.1; } ... nlri { ipv4/unicast add 192.168.1.0/24; } } }
EOF_CONF

# Alternate config with two routes
tmpfs=config2.conf:terminator=EOF_CONF2
bgp { peer loopback { remote { ip 127.0.0.1; } ... nlri { ipv4/unicast add 192.168.1.0/24; ipv4/unicast add 10.0.0.0/24; } } }
EOF_CONF2

option=tcp_connections:value=2
expect=bgp:conn=1:seq=1:hex=...   # Initial route
action=rewrite:conn=1:seq=2:source=config2.conf:dest=ze-bgp.conf
action=sighup:conn=1:seq=2
expect=bgp:conn=2:seq=1:hex=...   # Both routes after reload
```

### 5. VPP Tests (`test/vpp/`)

> **Not in the default release gate.** VPP tests are not included in
> `make ze-verify` / `make ze-functional-test`. Run manually via
> `make ze-vpp-test` or `bin/ze-test vpp`.

Functional tests that exercise `fib-vpp` end-to-end against a Python
GoVPP-API stub. The stub replaces the real VPP process in CI: no DPDK,
no vfio, no root. Each test runs against a fresh per-test Unix socket.
<!-- source: test/scripts/vpp_stub.py -- stdlib-only GoVPP socket-client stub -->

Real-daemon evidence uses `make ze-deployment-vpp-test`. That target starts
`ligato/vpp-base` under Docker, runs Linux-built `ze` and `ze-test peer` inside
the same container, and checks that VPP's FIB contains then withdraws the test
route.

`make ze-deployment-vpp-iface-test` is the interface-feature counterpart
(`scripts/evidence/effective-vpp-iface.py`): it proves ze programs a GRE tunnel
and a wireguard interface on real VPP 25.10, and probes plugin presence for the
wireguard and linux-cp (LCP) plugins. Recorded image limit: `ligato/vpp-base`
ships `wireguard_plugin.so` but NOT `linux_cp_plugin.so`/`linux_nl_plugin.so`,
so the LCP-pair scenario records an evidence-backed SKIP; validating LCP TAP +
BGP-over-TAP needs a VPP image built with the linux-cp plugins.
<!-- source: scripts/evidence/effective-vpp-iface.py -- tunnel/wireguard/LCP real-VPP evidence -->

**Runner:** `ze-test vpp [flags] [tests...]`

| Flag | Purpose |
|------|---------|
| `-l`, `--list` | List available tests (discovered from `test/vpp/*.ci`) |
| `-a`, `--all` | Run every test under `test/vpp/` |
| `-t`, `--timeout` | Per-test timeout (default 30s) |
| `-p`, `--parallel` | Concurrent tests (default 1 -- each test binds its own Unix socket) |
| `-v`, `--verbose` | Show per-test output |
| `-s`, `--save DIR` | Save client/peer logs under `DIR/<id>-<name>/` for offline inspection |

**Dependencies:**
- Tests use the `vpp.external` YANG leaf (default `false`) so ze connects
  via GoVPP without execing the VPP binary. See `docs/guide/vpp.md`.
- The stub runs as `python3 -m vpp_stub --socket <path> --log <path>`;
  PYTHONPATH is set by the runner to `test/scripts/`.

**Example:**
```
bin/ze-test vpp -l
bin/ze-test vpp 001-boot
bin/ze-test vpp -a
```
<!-- source: internal/test/cli/cmd_vpp.go -- vppCmd wires EncodingTests to test/vpp/ -->

### 6. Backend Apply-Path Unit Tests (Go `_test.go`)

Backends that talk to an out-of-process IPC surface (VPP GoVPP, a netconf server,
any future RPC-backed kernel) expose a narrow unexported operation interface
(`vppOps`, `netconfOps`, ...) with only the methods the Apply path needs. The
production adapter wraps the live transport; unit tests substitute a scripted
fake that records calls and can fail on the Nth request.

`internal/plugins/traffic/vpp/` is the reference implementation:

- `ops.go` defines `vppOps` (6 methods: `dumpInterfaces`, `dumpPolicers`,
  `policerAddDel`, `policerDel`, `policerDeleteByName`, `policerOutput`).
- `backend_linux.go` exposes an internal `applyWithOps(ops, desired)` entry
  point and a `govppOps{ch api.Channel}` production adapter; `Apply`
  constructs a `govppOps` around the channel it opened and calls
  `applyWithOps`.
- `apply_test.go` defines `fakeOps` (records a `[]string` of labeled calls,
  supports `failOnNthAddDel` for deterministic partial-failure tests) and
  covers the create/update/undo/reconcile/orphan branches without a running
  VPP daemon.

Use this pattern when adding a new backend whose Apply path would otherwise
require full-stack integration tests to cover every undo / reconcile branch.

<!-- source: internal/plugins/traffic/vpp/ops.go -- vppOps interface -->
<!-- source: internal/plugins/traffic/vpp/backend_linux.go -- applyWithOps, govppOps adapter -->
<!-- source: internal/plugins/traffic/vpp/apply_test.go -- fakeOps + Apply-path tests -->

### 7. Decode Tests (`test/decode/`)

BGP message decoding tests verify that wire bytes decode to the expected JSON.
They use `.ci` files for command-based tests and still accept legacy `.test`
fixtures.
<!-- source: internal/test/runner/decoding.go -- DecodingTests.Discover, DecodingRunner.Run -->

**Format:**
```
stdin=payload:hex=<hex-encoded-bgp-message>
cmd=foreground:seq=1:exec=ze bgp decode --json --family <family> -:stdin=payload
expect=json:json=<expected-json>
```

JSON validation compares parsed objects field by field with key order ignored,
volatile fields removed, and `peer`/`neighbor` naming normalized.
<!-- source: internal/test/runner/decoding.go -- compareJSON, normalizeNeighborSection -->

### Install Tests (`test/install/`)

Install functional tests cover the offline `ze install` command surface and the
appliance kernel/initrd/ISO build commands. The QEMU evidence entries run Python drivers
that self-skip when external prerequisites are missing, printing either
`INSTALL-QEMU: SKIP` or `INSTALL-ISO-QEMU: SKIP` while exiting successfully.
Real failures exit non-zero.
<!-- source: internal/test/cli/register.go -- installCmd -->
<!-- source: test/install/qemu-full.ci -- PXE installer evidence entry -->
<!-- source: test/install/qemu-iso.ci -- ISO installer evidence entry -->

| File | What it verifies |
|------|------------------|
| `appliance-kernel-auto-docker.ci` | `ze appliance kernel` without `--builder` delegates to `run.py`, which prefers Docker, and writes the installer artifact into both `build/kernel/` and the XDG cache |
| `appliance-kernel-auto-qemu.ci` | `run.py`'s `select_builder` implements the docker-first / qemu-fallback auto-selection (moved out of Go) |
| `appliance-kernel-docker.ci` | `ze appliance kernel --builder docker` delegates to `run.py`, which drives Docker → `build.py` with the resolved `--fragment` list, writes `build/kernel/{Image,config,kernel.version}`, and stores the same artifact under the XDG cache |
| `appliance-kernel-qemu.ci` | `ze appliance kernel --builder qemu` delegates to `run.py`, which selects QEMU and invokes `qemu-build.py` with the resolved fragments (including the shared `efi-console` fragment for the hardware profile) |
| `appliance-kernel-runtime.ci` | `ze appliance kernel --target runtime` resolves the runtime registry, enforces the runtime floor, and caches the runtime TREE (vmlinuz + lib/modules) under a `target=runtime` cache dir |
| `appliance-push-image-escape.ci` | `ze appliance push` rejects `--image` candidates that escape the appliance directory before network or TLS work |
| `appliance-iso-default-paths.ci` | `ze appliance iso` succeeds with default kernel/initrd artifact paths and stages those files into the installer tree |
| `appliance-iso-arm64.ci` | `ze appliance iso` emits arm64 UEFI staging assets and arm64 kernel console settings when `image.arch=arm64` |
| `kernel-builder-single-driver.ci` | A single shared driver (`tools/kernel-builder/run.py`) replaces the docker/qemu invocation; no Makefile or Go file invokes docker/qemu/build.py directly (AC-1) |
| `kernel-arch-mapping-single.ci` | The arch → docker platform mapping appears exactly once, in `run.py` (AC-2) |
| `kernel-shared-fragment.ci` | The six shared console symbols are single-sourced in `common/efi-console.config` and pulled into runtime + hardware via `# ze-include`, absent from qemu; the python resolver expands it (AC-3/AC-5) |
| `kernel-compose.ci` | Runtime fragments (base + runtime + the shared `efi-console` fragment) keep required built-in options and exclude removed Kconfig symbols before any real build runs |
| `kernel-qemu-arch-alias.ci` | `tools/kernel-builder/qemu-build.py` accepts `aarch64` as an alias for `arm64` and continues to later path validation |
| `kernel-builder-packages.ci` | Shared runtime builder package lists include host tools needed for `CONFIG_KERNEL_ZSTD=y` images and `modules_install` output (`zstd`, `kmod`) in both Docker and QEMU backends |
| `kernel-builder-no-shell.ci` | `build.py`/`qemu-build.py`/`run.py`/`ksource.py` use no `shell=` subprocess argument; `enforce_required_symbols` + `embed_firmware` behave |
| `kernel-runtime-deps.ci` | `gokrazy/kernel/Makefile` treats the builder scripts (incl. `run.py`, `ksource.py`), the shared fragment, Dockerfile, and tracked patches as rebuild inputs |
| `kernel-tarball-dedup.ci` | The `cdn.kernel.org` URL + `vN.x` series construction lives only in `ksource.py`, imported by both `build.py` and `qemu-build.py` (AC-11) |
| `kernel-version-single-reader.ci` | No Makefile reads `kernel.version`; one variable name (`KERNEL_VERSION`) at the builder env boundary; `run.py` self-locates the version file (AC-14/AC-15) |
| `kernel-version-provenance.ci` | Every build emits a `build/kernel.version` provenance sidecar; a malformed or pre-7 version is rejected before any build (AC-16/AC-17) |
| `kernel-wiring.ci` | Installer and runtime Makefiles delegate the build to `tools/kernel-builder/run.py` (no inline docker/qemu) |
| `ze-kernel-no-modcache-mutation.ci` | `make ze-kernel` consumes the runtime kernel out-of-tree via a go.mod replace and never mutates the pinned modcache or creates `.ze-pinned-kernel` (AC-9/AC-10) |
| `qemu-full.ci` | PXE installer path writes the image, injects ZeFS, boots the written disk, and authenticates |
| `qemu-iso.ci` | Appliance ISO path writes the embedded image unchanged, skips PXE ZeFS injection, powers off safely, boots the written disk, and authenticates |
| `ze-kernel-overlay.ci` | `make ze-kernel` builds the runtime kernel via `run.py` and assembles it into an out-of-tree package (`tmp/kernel/pkg`) consumed via a go.mod replace, with the pinned modcache untouched |

Run the install suite with `bin/ze-test install --all`. For exhaustive QEMU
entry points:

- `make ze-install-qemu-test` — PXE HTTP install.
- `make ze-install-iso-qemu-test` — appliance ISO media (amd64 x86_64-UEFI or
  arm64 aarch64-UEFI; arch follows `ZE_INSTALL_ARCH` or the host).
- `make ze-install-scenarios-qemu-test` — failure-path / pin / rescue evidence
  (R-6 goroutine-panic recovery, `ze.mac` boot-NIC pin and flush recovery, and
  the three-branch rescue console).
- `make ze-install-ventoy-qemu-test` — Ventoy path: the appliance ISO carried as
  a file on a FAT data disk, located by `tryVentoyISO` and installed. Needs
  `grub-mkstandalone` + `xorriso` + `mtools`.

All self-skip with a single `SKIP` line when the operator-supplied installer
kernel (`ZE_INSTALL_KERNEL`) or a required host tool is unavailable.
<!-- source: test/install/appliance-kernel-auto-docker.ci -- ze appliance kernel default docker path -->
<!-- source: test/install/appliance-kernel-auto-qemu.ci -- ze appliance kernel qemu fallback path -->
<!-- source: test/install/appliance-kernel-docker.ci -- ze appliance kernel explicit docker path -->
<!-- source: test/install/appliance-kernel-qemu.ci -- ze appliance kernel explicit qemu path -->
<!-- source: test/install/kernel-builder-packages.ci -- shared builder package prerequisites -->
<!-- source: test/install/kernel-compose.ci -- runtime fragment contract -->
<!-- source: test/install/kernel-qemu-arch-alias.ci -- qemu arch alias validation -->
<!-- source: test/install/kernel-runtime-deps.ci -- runtime makefile dependency coverage -->
<!-- source: test/install/kernel-wiring.ci -- shared builder delegation -->
<!-- source: test/install/kernel-builder-single-driver.ci -- single run.py driver -->
<!-- source: test/install/kernel-shared-fragment.ci -- ze-include shared fragment -->
<!-- source: test/install/appliance-kernel-runtime.ci -- runtime verified path -->
<!-- source: test/install/kernel-version-provenance.ci -- provenance sidecar + version validation -->
<!-- source: test/install/ze-kernel-no-modcache-mutation.ci -- out-of-tree consumption -->
<!-- source: test/install/ze-kernel-overlay.ci -- ze-kernel out-of-tree package + go.mod replace -->
<!-- source: mk/test-integration.mk -- ze-install-qemu-test, ze-install-iso-qemu-test, ze-install-scenarios-qemu-test, ze-install-ventoy-qemu-test -->
<!-- source: scripts/evidence/effective-install-scenarios-qemu.py -- R-6 fault, ze.mac pin/flush, rescue console evidence -->
<!-- source: scripts/evidence/effective-install-ventoy-qemu.py -- Ventoy ISO-on-FAT evidence -->

### IS-IS Tests (`test/isis/`)

IS-IS functional tests cover the native IS-IS component (ISO/IEC 10589, RFC 1195
/ 5305 / 5301): config parse -> YANG schema -> NET/system-id validation ->
component startup. They exercise the config-wiring surface only; live adjacency,
LSDB, SPF, and FRR interop run as QEMU integration tests (raw L2 needs
`CAP_NET_RAW` on a Linux veth) and are owned by the runtime sibling specs.

| File | What it verifies |
|------|------------------|
| `isis-config.ci` | A valid `isis { net ... }` block validates through the real YANG schema and the `isis-net` custom validator; a structurally invalid NET is rejected |
| `isis-adjacency.ci` | The per-interface adjacency leaves (circuit-type, hello-interval, hold-multiplier, priority, level) validate on a point-to-point and a broadcast circuit; a hold-multiplier of 0 is rejected by the YANG range. Live adjacency (two nodes reaching Up) is proven by the `TestISISAdjacencyUp` unit test and the `adjacency_integration_linux_test.go` QEMU integration test |
| `isis-doctor-raw-socket.ci` | The `isis-raw-socket` doctor check reports the raw-socket / `CAP_NET_RAW` readiness of the transport |
| `isis-flooding.ci` | The CSNP and PSNP PDUs the flooding child builds for LSDB synchronisation (whole-range CSNP with a TLV 9 LSP-entry; PSNP ack/request TLV 9) decode through `ze isis decode`. Live reliable flooding (a three-node line converging, the SRM flood timer, CSNP/PSNP request/ack, the P2P initial CSNP, purge re-flood) is proven by the in-memory engine wiring tests (`TestISISLSDBSync`, `TestISISFloodSRMTimer`, `TestISISCSNPGapRequest`, `TestISISPSNPAck`) plus the QEMU integration tests and the FRR interop scenarios (isis-13) |
| `isis-auth.ci` | The authentication config surface validates through the real YANG schema: named `key-chains` with a `key-id`, an algorithm enum (cleartext / hmac-md5 / hmac-sha-256), a `$9$`-encoded secret, and per-level (area/domain) + per-interface (IIH) `auth-key-chain` references; an invalid algorithm enum is rejected. Live sign-on-send / verify-on-receive, wrong-key rejection, TLV-10-first ordering, the LSP checksum-after-sign interaction, authenticated purges, hitless rotation, and constant-time compare are proven by the unit tests (`TestISISAuthSignVerify*`, `TestISISAuthReject`, `TestISISAuthLSPChecksumAfterSign`, `TestISISAuthPurge`, `TestISISAuthRotation`, `TestISISAuthSecretEncoding`) plus the QEMU integration tests and the `isis-auth-frr` interop scenario (isis-13) |
| `isis-redist-bgp.ci` | The redistribution config surface validates through the real YANG schema: `redistribute { destination bgp { import isis } }` is accepted (the single source name `isis` is registered with the source registry, so the `redistribute-source` validator/completion accept it), and `redistribute { destination isis { import connected/static/bgp } }` is accepted (`destination isis` is a runtime-validated free-form list key); `destination isis { import isis }` validates but is a runtime no-op (loop prevention). Live route flow (an IS-IS SPF route in the BGP RIB; a connected/static/BGP prefix as a TLV 135 entry in an IS-IS LSP and in a peer's RIB) is proven by the unit tests in `internal/plugins/isis/redistribute` (`TestISISProducerRegistered`, `TestISISRegisterSource`, `TestISISRedistSourceToBGP`, `TestISISRedistSourceWithdrawToBGP`, `TestISISRedistConsumerConnected`/`Static`/`BGP`/`Withdraw`/`UpDownBit`/`Name`/`LogsFailure`, `TestISISConnectedAdvertise`, `TestISISRedistSelfImportRejected`, `TestISISRedistRegistrationOrder`, `TestISISRedistMetricBoundary`) plus the QEMU `isis-redist-frr` interop scenario |
| `isis-ipv6.ci` | A dual-stack `isis { ... }` config (per-interface `address-family ipv6-unicast`) validates through the real YANG schema, the IPv6 SPF + install pass is wired (a second `BuildRoutesV6` over the shared tree feeding an IPv6-family Loc-RIB Installer), and `show isis route ipv6` returns an empty list with no adjacency (no phantom IPv6 routes). Live dual-stack flow (TLV 232 link-local in the Hello, TLV 236 in the LSP, an fe80:: next-hop IPv6 route in the kernel, IPv6 redistribution both ways) is proven by the unit tests (`TestISISOriginateTLV236`, `TestISISOriginateTLV232Scope`, `TestISISProtocolsSupportedDualStack`, `TestISISIIHTLV232LinkLocal`, `TestISISIPv6SPFNextHop`, `TestISISIPv6LinkLocalNextHop`, `TestISISIPv6RouteLocRIBInsert`, `TestISISIPv6MetricAboveMaxIgnored`, `TestISISRedistConsumerIPv6`, `TestISISRedistSourceIPv6`) plus the QEMU `isis-dualstack-frr` interop scenario (isis-13) |
| `isis-show.ci` | The full `show isis <noun>` / `clear isis <action>` surface dispatches through the engine: with a NET and a passive interface (so the engine starts without `CAP_NET_RAW`), `show isis neighbor` returns an empty array, `show isis database` carries the own LSP, `show isis database detail` expands TLVs, `show isis route` is present, `show isis interface` reports the passive circuit, `show isis hostname` maps the local System ID to the configured name (TLV 137), `show isis spf-log` is present, and `clear isis adjacency` / `clear isis counters` return a status payload (no `unknown command`). Proxy arg-rejection and render shape are unit-tested in `cmd_show_test.go` / `show_test.go` |
| `isis-doctor.ci` | The IS-IS doctor codes are explainable (`ze explain doctor-isis-net-missing` / `doctor-isis-system-id-mismatch` / `doctor-isis-raw-socket`), and `ze doctor --json` against a config whose `system-id` disagrees with the NET emits `doctor-isis-system-id-mismatch`. The net-missing and raw-socket firing paths are unit-tested (`doctor_test.go`, isis-3 `transport/doctor_test.go`) |

The six FRR interop scenarios (`test/interop/scenarios/isis-{p2p,lan-dis,dualstack,auth,convergence,redist}-frr`)
exercise the protocol against a live FRR `isisd` over the shared Docker bridge: P2P
adjacency + convergence, LAN DIS election + pseudo-node LSP, IPv4+IPv6 reachability,
HMAC-MD5 authentication, link-down reconvergence, and IS-IS<->BGP redistribution.
They are the goal-validation evidence for the IS-IS umbrella and run under the
Linux Docker interop harness (`test/interop/daemons` has `isisd=yes`), not on darwin.

Run with `bin/ze-test isis --all` or `make ze-isis-test`. The offline wire-decode
suite is separate (`test/isis-wire/`, `make ze-isis-wire-test`).
<!-- source: internal/test/cli/register.go -- isis CI suite registration -->
<!-- source: test/isis/isis-config.ci -- config validation evidence -->
<!-- source: test/isis/isis-adjacency.ci -- adjacency config-surface evidence -->
<!-- source: test/isis/isis-flooding.ci -- flooding CSNP/PSNP wire-format evidence -->
<!-- source: test/isis/isis-auth.ci -- authentication config-surface evidence -->
<!-- source: test/isis/isis-ipv6.ci -- dual-stack IPv6 SPF/install wiring evidence -->
<!-- source: mk/test-functional.mk -- ze-isis-test -->

The OSPF FRR interop scenarios exercise the unified OSPF engine against a live FRR
`ospfd`/`ospf6d` over the shared Docker bridge. The OSPFv3 set proves a P2P and a broadcast
(DR/BDR + Network-LSA + Link-LSA) adjacency, ABR inter-area summary install
(`ospf-v6-multiarea-frr`: FRR installs Ze's area-1 prefix `2001:db8:a1::/64` as an inter-area
route), stub-area ABR default origination (`ospf-v6-stub-frr`: FRR installs the Ze-originated
`::/0`, no Type-5 leak), broadcast DR route install (`ospf-v6-broadcast-frr`), and ASBR
redistribution into AS-External / NSSA Type-7. Route assertions on the shared LAN use a
passive dummy interface carrying a unique global prefix that exists only on Ze, since a
segment prefix connected on both sides cannot be asserted as a route. They run under the
Linux Docker interop harness only (raw IPv6 proto 89 over `ff02::5`), not on darwin.
<!-- source: test/interop/scenarios/ospf-v6-multiarea-frr/check.py -- v6 inter-area route install -->
<!-- source: test/interop/scenarios/ospf-v6-stub-frr/check.py -- v6 stub ABR default + no Type-5 leak -->
<!-- source: test/interop/scenarios/ospf-v6-broadcast-frr/check.py -- v6 DR-advertised route install -->
<!-- source: test/interop/interop.py -- FRROSPF6 has_inter_area_prefix_lsa/has_as_external_lsa helpers -->
<!-- source: internal/plugins/ospf/origination_v6_stub.go -- v6 stub default origination (v6ApplyAreaTypePolicy) -->

---

## CLI Reference

Common suite form:

```
ze-test bgp <suite> [options] [ID_OR_NAME...]
ze-test <suite> [options] [ID_OR_NAME...]
```

Common selection options:

| Option | Meaning |
|--------|---------|
| `--list`, `-l` | List available tests as `N/TOTAL ID NAME` with one-based ids |
| `--all`, `-a` | Run all tests in the suite |
| `--start ID` | Run the test matching `ID` or exact name, then every later test in suite order |
| `--pattern TEXT` | Run tests whose id, name, or path contains `TEXT` |
| `ID_OR_NAME...` | Run only the listed tests |

Common run options:

| Option | Meaning |
|--------|---------|
| `-t`, `--timeout N` | Timeout per test for `.ci` BGP/VPP runners |
| `-p`, `--parallel N` | Max concurrent tests for `.ci` runners |
| `-v`, `--verbose` | Show verbose failure output |
| `-q`, `--quiet` | Minimal output |
| `-s`, `--save DIR` | Save logs for BGP/VPP `.ci` runners |
| `--port N` | Base port for BGP/VPP `.ci` runners |
| `-c`, `--count N` | BGP `.ci` stress mode, run each selected test N times |
| `--server ID`, `--client ID` | BGP `.ci` manual split-debug modes |
<!-- source: internal/test/cli/cmd_bgp.go -- parseRunCLI, printRunUsage -->
<!-- source: internal/test/cli/ci_runner.go -- runCISubcommand -->
<!-- source: internal/test/cli/cmd_vpp.go -- parseVPPCLI, printVPPUsage -->
<!-- source: internal/test/cli/cmd_editor.go -- editorMain -->
<!-- source: internal/test/cli/cmd_web.go -- cmdWebMain -->

---

## Test IDs

Each test has a one-based decimal id printed by `--list`. The id is the stable
selector to pass positionally or to `--start`; in a full-suite list or run it
matches the `N` in `N/TOTAL`. A line like `42/120  42  bgp-open` means "test id
42 is the 42nd test in a 120-test suite."
<!-- source: internal/test/runner/record.go -- GenerateNick -->
<!-- source: internal/test/runner/record_collection.go -- Tests.List -->

**Examples:**
```bash
# Run test id 4
ze-test bgp encode 4

# Run tests 1, 41, and 42
ze-test bgp encode 1 41 42

# Resume at id 42
ze-test bgp encode --start 42
```

---

## .ci File Format

The `.ci` file is the **source of truth** for bidirectional testing. Full format documentation: [`docs/architecture/testing/ci-format.md`](architecture/testing/ci-format.md)

```
# Tmpfs: embed config inline
tmpfs=test.conf:terminator=EOF_CONF
peer test-peer { remote { ip 127.0.0.1; } ... }
EOF_CONF

# Options
option=file:path=test.conf
option=asn:value=65000

# Commands and expectations
cmd=api:conn=1:seq=1:text=update text nhop set 10.0.1.1 nlri ipv4/unicast add 10.0.0.0/24
expect=bgp:conn=1:seq=1:hex=FFFF...
expect=json:conn=1:seq=1:json={...}
```

### Line Types

| Action | Example | Description |
|--------|---------|-------------|
| `tmpfs=` | `tmpfs=file.conf:terminator=EOF` | Embed file content inline |
| `option=` | `option=file:path=test.conf` | Test configuration |
| `cmd=` | `cmd=api:conn=1:seq=1:text=...` | API command |
| `expect=bgp:` | `expect=bgp:conn=1:seq=1:hex=...` | Expected wire bytes |
| `expect=json:` | `expect=json:conn=1:seq=1:json=...` | Expected JSON |
| `expect=stdout:` | `expect=stdout:contains=text` | Substring match in stdout |
| `expect=stderr:` | `expect=stderr:pattern=...` or `contains=...` | Regex or substring match in stderr |
| `expect=syslog:` | `expect=syslog:pattern=...` | Regex pattern in syslog |
| `reject=stderr:` | `reject=stderr:pattern=...` | Fail if stderr matches regex |
| `reject=syslog:` | `reject=syslog:pattern=...` | Fail if syslog matches regex |
| `action=notification:` | `action=notification:conn=1:seq=1:text=...` | Send NOTIFICATION |
| `action=rewrite:` | `action=rewrite:conn=1:seq=2:source=config2.conf:dest=ze-bgp.conf` | Rewrite config file |
| `action=sighup:` | `action=sighup:conn=1:seq=2` | Send SIGHUP to daemon |
| `action=sigterm:` | `action=sigterm:conn=1:seq=2` | Send SIGTERM to daemon |

### Waiting without sleep (the quiesce barrier)

Prefer a completion signal over a fixed `time.sleep` (the ci-sleep ratchet in
`scripts/dev/verify_wiring_docs.py` counts sleeps in `test/**/*.ci`). Existing
options: `ze_api.wait_for_event` / `wait_for_shutdown` / `wait_for_post_startup`
(block on a delivered event / bye / post-startup RPC), `expect=event`, and
`http=wait`.

For "I changed something, now assert its downstream effect", use the **quiesce
barrier**: `ze_api.quiesce()` (or the `request quiesce` command) sends
`ze-system:quiesce`, which blocks until every registered subsystem has drained
its pending async work (the BGP forward pool among them) and then replies. So
`send(route); quiesce()` guarantees the route is on the peer wire with no sleep.
See `docs/architecture/api/commands.md` "Quiesce Barrier".

<!-- source: test/scripts/ze_api.py -- quiesce (barrier helper) -->
<!-- source: internal/component/plugin/server/quiesce.go -- ze-system:quiesce handler -->

#### Payload-predicate waits

When the wait is "block until an *observed payload* matches a condition" (not a
one-shot barrier), use a payload-predicate wait so the test blocks exactly until
the condition holds instead of a guessed duration:

- `ze_api.wait_until(predicate, attempts=20, delay=0.25)` — poll an arbitrary
  `predicate()` (e.g. kernel FIB state via `ip route show`) until true.
- `api.dispatch_until(cmd, predicate)` — re-dispatch `cmd` until
  `predicate(result)` is true; returns the winning result dict.
  `dispatch_until_done(cmd)` is the `status=="done"` special case.
- `api.wait_for_event(timeout, predicate)` — return the first delivered event
  whose decoded (JSON) form satisfies `predicate` (`predicate=None` keeps the
  legacy "first event of any type" behavior).

The symmetric first-class form for `.ci` engine steps is the declarative
predicate grammar `expect=output:matches=<regexp>` / `absent=<substr>` /
`json=<path>=<value>` (and `expect=stream:matches=`); see
`docs/architecture/testing/ci-format.md` "Engine Steps".

<!-- source: test/scripts/ze_api.py -- wait_until, dispatch_until, wait_for_event -->
<!-- source: internal/test/runner/engine_steps.go -- parseEngineExpectContains, engineOutputSatisfied -->

### Tmpfs (Virtual File System)

Tmpfs allows embedding config files directly in `.ci` files:

```
tmpfs=peer.conf:terminator=EOF_CONF
peer test-peer {
    remote {
        ip 127.0.0.1;
        as 65533;
    }
    local-as 65533;
}
EOF_CONF

option=file:path=peer.conf
```

At runtime, Tmpfs files are written to a temp directory. This enables self-contained tests without separate `.conf` files.

### Directive Placement

Test directives belong to one of two scopes:

| Scope | Consumer | Placement |
|-------|----------|-----------|
| Test runner | The `ze-test` process itself (seeds `proc.Env`, drives orchestration) | File level, outside any `stdin=...` block |
| `ze-peer` stdin | The `ze-peer` subprocess reading its stdin at runtime | Inside the `stdin=peer:terminator=X` block |

Only `expect=bgp:...`, `expect=json:...`, `expect=exit:...`, `action=...`, `option=timeout:...`, `option=open:...`, `option=update:...`, `option=tcp_connections:...`, and `option=conn_map:...` are valid inside `stdin=peer:` blocks. The `option=timeout`, `option=open`, `option=update`, `option=tcp_connections`, and `option=conn_map` forms are consumed by `ze-peer` from its stdin and must stay in-block so the subprocess receives them.

`option=conn_map:value=router-id` sorts each accepted connection batch by the BGP router ID in OPEN. `option=conn_map:value=remote-ip` sorts each batch by the TCP source address, which stays stable when reload tests intentionally change router IDs. With `conn_map`, `option=tcp_connections:value=N` is the batch size; if expectations remain after one batch, `ze-peer` accepts another batch and continues with the next `conn=N` rules.

**`option=env:var=K:value=V` is consumed by the test runner (it appends to `proc.Env` when spawning `ze`/`ze-peer`/helper processes) and therefore MUST live at file level, outside any `stdin=peer:` block.** Placing it inside the block used to be silently dropped — the directive would be handed to `ze-peer`, which ignores it, and the target process would never see the variable. The parser now rejects this at `bin/ze-test <suite> -list` time with an error naming the exact directive and pointing at this section.

<!-- source: internal/test/runner/record_parse.go — parseAndAdd peer-block loop -->

**Correct placement:**

```
# option=env belongs ABOVE the stdin=peer block.
option=env:var=ze.log.bgp.server:value=debug

stdin=peer:terminator=EOF_PEER
option=timeout:value=15s
option=open:value=inspect-open-message
expect=bgp:conn=1:seq=1:hex=FFFF...
EOF_PEER
```

**Rejected at parse time:**

```
stdin=peer:terminator=EOF_PEER
option=timeout:value=15s
option=env:var=ze.log.bgp.server:value=debug   # <-- PARSE ERROR
expect=bgp:conn=1:seq=1:hex=FFFF...
EOF_PEER
```

### Logging Tests

Tests can verify logging behavior using `option=env:`, `expect=stderr:`, `reject=stderr:`, and `expect=syslog:`.

**Example: Verify server subsystem logs to stderr**
```
option=file:path=mytest.conf
option=env:var=ze.bgp.log.server:value=debug

expect=bgp:conn=1:seq=1:hex=FFFF...
expect=stderr:pattern=subsystem=server
```

**Example: Verify DEBUG messages are filtered at INFO level**
```
option=file:path=mytest.conf
option=env:var=ze.bgp.log.server:value=info

expect=bgp:conn=1:seq=1:hex=FFFF...
reject=stderr:pattern=level=DEBUG
```

**Example: Verify syslog backend**
```
option=file:path=mytest.conf
option=env:var=ze.bgp.log.server:value=debug

expect=bgp:conn=1:seq=1:hex=FFFF...
expect=syslog:pattern=subsystem=server
```

When `expect=syslog:` is present, the test runner automatically:
1. Starts a test-syslog UDP server on a dynamic port
2. Sets `ze.log.backend=syslog` and `ze.log.destination=127.0.0.1:<port>`
3. Validates patterns against captured syslog messages after test

#### Syslog Testing Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                      TEST RUNNER (runner.go)                      │
│                                                                   │
│  1. Parse .ci file                                                │
│     └── Found: expect:syslog:subsystem=server                     │
│                                                                   │
│  2. Start testsyslog server (UDP, dynamic port)                   │
│     └── syslog.New(0).Start(ctx) → port 54321                 │
│                                                                   │
│  3. Auto-inject env vars for ze-bgp:                               │
│     └── ze.bgp.log.backend=syslog                                  │
│     └── ze.bgp.log.destination=127.0.0.1:54321                     │
│     └── ze.bgp.log.server=debug  (from option:env:)                │
│                                                                   │
│  4. Start ze bgp with config                                       │
└──────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌──────────────────────────────────────────────────────────────────┐
│                           ZEBGP                                   │
│                                                                   │
│  slogutil.Logger("server") reads env vars:                        │
│    - ze.bgp.log.server=debug → enabled at DEBUG                    │
│    - ze.bgp.log.backend=syslog → use syslog handler                │
│    - ze.bgp.log.destination=127.0.0.1:54321 → UDP target           │
│                                                                   │
│  logger.Debug("msg", "subsystem", "server", ...)                  │
│         │                                                         │
│         ▼                                                         │
│  slog.TextHandler → syslog.Writer → UDP packet                    │
└──────────────────────────────────────────────────────────────────┘
                                │
                      UDP: "<14>... subsystem=server ..."
                                │
                                ▼
┌──────────────────────────────────────────────────────────────────┐
│                      TESTSYSLOG SERVER                            │
│                                                                   │
│  Receives: "<14>Jan 19 ... ze-bgp: level=DEBUG subsystem=server"   │
│  Stores in: srv.messages[]                                        │
└──────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌──────────────────────────────────────────────────────────────────┐
│                   VALIDATION (after test)                         │
│                                                                   │
│  validateLogging() checks each expect:syslog: pattern:            │
│    if !syslogSrv.Match("subsystem=server"):                       │
│        return error("pattern not found")                          │
│                                                                   │
│  Match() does regex search across all captured messages           │
└──────────────────────────────────────────────────────────────────┘
```

**Key components:**

| Component | Location | Purpose |
|-----------|----------|---------|
| `syslog.Server` | `internal/test/syslog/` | UDP server capturing syslog messages |
| `option:env:` | `.ci` file | Sets env vars (e.g., `ze.bgp.log.server=debug`) |
| `expect:syslog:` | `.ci` file | Regex pattern to match in captured messages |
| Auto-injection | `runner.go` | Adds `backend=syslog` + `destination=host:port` |
| `validateLogging()` | `runner.go` | Checks patterns after test completes |

<!-- source: internal/test/syslog/testsyslog.go -- syslog UDP server -->
<!-- source: internal/test/runner/runner.go -- auto-injection and validateLogging -->

**Message format:** Syslog messages use Go's `slog.TextHandler` format with syslog framing:
```
<priority>timestamp hostname ze-bgp: level=DEBUG subsystem=server msg="..." key=value
```

Patterns should match the key=value pairs (e.g., `subsystem=server`, `level=DEBUG`).

### Raw Message Format

```
MARKER:LENGTH:TYPE:PAYLOAD
```

- MARKER: 16 bytes (all FF)
- LENGTH: 2 bytes (total message length)
- TYPE: 1 byte (1=OPEN, 2=UPDATE, 3=NOTIFICATION, 4=KEEPALIVE)
- PAYLOAD: Variable

### JSON Validation Format

The `N:json:` lines use ZeBGP plugin format (not ExaBGP envelope format):

**Unicast:**
```json
{"meta":{"version":"1.0.0","format":"ze-bgp"},"message":{"type":"update"},"origin":"igp","ipv4/unicast":[{"next-hop":"10.0.1.254","action":"add","nlri":["10.0.0.0/24"]}]}
```

**FlowSpec:**
```json
{"meta":{"version":"1.0.0","format":"ze-bgp"},"message":{"type":"update"},"origin":"igp","ipv4/flowspec":[{"action":"add","nlri":{"next-hop":"1.2.3.4","destination":["192.168.0.1/32"],"string":"flow destination 192.168.0.1/32"}}]}
```

**Supported families:** `ipv4/unicast`, `ipv6/unicast`, `ipv4/flowspec`, `ipv6/flowspec`

**Key differences from ExaBGP envelope:**
- Flat structure (no `neighbor.message.update` nesting)
- `meta.format` = "ze-bgp" (not `exabgp` version)
- Family arrays at top level with `action` field
- FlowSpec: `nlri` is object with components; unicast: `nlri` is string array

**Context fields ignored:** `peer`, `direction` (test-environment dependent)

---

## Test Execution Flow

### Encode Tests

```
1. Runner builds ze + ze-peer to temp dir
2. Starts ze-peer on unique port with .ci expectations
3. Starts ze bgp with config
4. ze bgp connects, sends OPEN, receives OPEN
5. ze bgp sends UPDATE messages (from static routes)
6. ze-peer validates messages against expectations
7. ze-peer prints "successful" or error
```

### API Tests

```
1. Same as encode tests, plus:
5. ze bgp spawns .run script as subprocess
6. .run script sends commands via API
7. ze bgp processes commands, sends UPDATE messages
8. ze-peer validates messages
```

---

## Display Output

### Progress and per-test lines

During execution, TTY status updates include overall progress, longest-running
test timer, pass/running/fail/timeout counts, and pending ids. Non-TTY logs emit
periodic progress lines while tests are running.

```
progress 12/42 [5/20s] passed 12 running 4 [12, 13, 14, 15] pending 26
5.0s      12/42   4/8  running  12(5.0s), 13(5.0s), 14(5.0s), 15(5.0s)
```

Every completed test emits one line:

```
812ms    13/42  PASS  12  route-refresh-basic
```

| Field | Meaning |
|-------|---------|
| `progress N/TOTAL` | Completed tests, including skipped tests, out of selected tests |
| `[N/Ms]` | Longest running test: N seconds elapsed, M timeout |
| `running N [IDs]` | N tests currently executing, ids shown when at most five are running |
| `13/42` | One-based run number out of selected total for a completion line |
| `PASS  12  route-refresh-basic` | Result, decimal id, and test name |
<!-- source: internal/test/runner/display.go -- Status, TestFinished -->

### Section Header

Each test suite is framed by a section header, including top-level suites such
as `ui`, `managed`, `l2tp`, `firewall`, `policy`, `web`, and `install`:
```
═══════════════════════ encode ════════════════════════════════════════════════
```
<!-- source: internal/test/cli/cmd_bgp.go -- BGP suite headers -->
<!-- source: internal/test/cli/ci_runner.go -- top-level .ci suite headers -->

### Summary

On success:
```
pass  42/42  100.0%  3.2s
```

On failure:
```
fail  40/42  95.2%  3.2s  failed 2 [4, 9]  timeout 1 [12]
```

| Field | Format | Meaning |
|-------|--------|---------|
| Verdict | `pass` or `fail` | Green if all passed, red otherwise |
| Ratio | `N/M` | Passed / total completed tests |
| Rate | `N.N%` | Pass percentage |
| Time | `N.Ns` or `Nms` | Wall-clock elapsed |
| Failed | `failed N [ids]` | Only shown when greater than zero |
| Timeout | `timeout N [ids]` | Only shown when greater than zero |
<!-- source: internal/test/runner/display.go -- Summary -->

### Verify Failure Groups

In `ZE_VERIFY_MODE=1`, failed `.ci` suites emit native failure groups before
the full `TEST FAILURE` blocks. A group records the suite label, group id,
failure kind, related ids, compact summary, exact rerun command, detail log,
and parallelization hint. The top-level verify runner copies only bounded group
metadata into `tmp/ze-verify-failures.log`; the full evidence remains in the
stage log.
<!-- source: internal/test/runner/failure_group.go -- native functional groups -->
<!-- source: internal/test/runner/report.go -- detailed failure reports -->
<!-- source: internal/test/runner/display.go -- summary and debug hints -->

### Stress Test Mode

Use `--count N` (`-c N`) to run tests multiple times for benchmarking or detecting flaky tests:

```bash
# Run test C 10 times with timing
ze-test bgp plugin -c 10 C

# Run all encoding tests 5 times
ze-test bgp encode -c 5 -a
```

**Per-iteration timing** is shown during execution:
```
==> Iteration 1/10
==> Iteration 1: 5.2s

==> Iteration 2/10
==> Iteration 2: 4.8s
```

**Summary** shows per-test stats and overall timing:
```
STRESS TEST SUMMARY
═══════════════════════════════════════════════════════════════════════════════
Iterations: 10

ID       Pass   Fail    T/O        Min        Avg        Max    Rate
---------------------------------------------------------------------------
0          10      0      0      108ms      332ms      764ms  100.0%
1           8      2      0      115ms      400ms      900ms   80.0%
═══════════════════════════════════════════════════════════════════════════════
Iteration timing: min=4.8s avg=5.1s max=5.7s total=51.2s
Total: 20 iterations, 18 passed, 2 failed, 0 timed out (90.0% pass rate)
```

**Key metrics:**
- Per-test min/avg/max duration
- Per-test pass rate (color-coded: green=100%, yellow≥50%, red<50%)
- Iteration timing: min/avg/max/total wall-clock time

---

## Debugging Tests

### Run a single test

BGP suites use the `ze-test bgp <suite>` command shape:

```bash
ze-test bgp encode --timeout 60s --verbose 4
ze-test bgp plugin --server 4
ze-test bgp plugin --client 4
```

Top-level `.ci` suites use `ze-test <suite>` without the `bgp` prefix:

```bash
ze-test ui 4
ze-test managed 4
ze-test firewall 4
```

Resume from the last printed id after a timeout or interrupted run:

```bash
ze-test bgp plugin --start 42
ze-test ui --start 42
ze-test editor --start 42
```
<!-- source: internal/test/runner/selection.go -- Selection.Start -->

The compact verify index emits the smallest useful rerun command for each
failure group. Use that command before widening scope.

### Decode message bytes

```bash
# Decode UPDATE payload
ze bgp decode update 0000001540010100400200400304650165014005040000006400

# Decode full message
ze bgp decode raw FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D02...
```

---

## Adding New Tests

### Option 1: Tmpfs (Recommended)

Single self-contained `.ci` file with embedded config:

```
# test/encode/mytest.ci
tmpfs=mytest.conf:terminator=EOF_CONF
peer loopback {
    remote {
        ip 127.0.0.1;
        as 1;
    }
    router-id 1.2.3.4;
    local-address 127.0.0.1;
    local-as 1;

    family {
        ipv4/unicast;
    }
    announce {
        ipv4 {
            unicast 10.0.0.0/24 next-hop 1.2.3.4;
        }
    }
}
EOF_CONF

option=file:path=mytest.conf
cmd=api:conn=1:seq=1:text=update text nhop set 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D020000001540010100...
```

### Option 2: Separate Files

```
# test/encode/mytest.conf
peer loopback {
    remote {
        ip 127.0.0.1;
        as 1;
    }
    router-id 1.2.3.4;
    local-address 127.0.0.1;
    local-as 1;

    family {
        ipv4/unicast;
    }
    announce {
        ipv4 {
            unicast 10.0.0.0/24 next-hop 1.2.3.4;
        }
    }
}
```

```
# test/encode/mytest.ci
option=file:path=mytest.conf
cmd=api:conn=1:seq=1:text=update text nhop set 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D020000001540010100...
```

### Generate expected bytes

Run with ExaBGP first to capture correct bytes, or use `ze bgp decode` to verify.

### Adding Negative Parsing Tests

To test that invalid configs are rejected with specific errors, create a `.ci` file:

```
# test/parse/my-error.ci
stdin=config:terminator=EOF_CONF
bgp {
    peer test-peer {
        remote {
            ip 10.0.0.1;
            as 65002;
        }
        router-id 1.2.3.4;
        local-as 65001;
        # ... invalid configuration ...
    }
}
EOF_CONF

cmd=foreground:seq=1:exec=ze bgp validate -:stdin=config
expect=exit:code=1
expect=stderr:contains=specific error message substring
```

**Regex match** (for variable parts like IPs, line numbers):
```
expect=stderr:regex=peer \d+\.\d+\.\d+\.\d+: route-refresh requires
```

The test passes if:
- `ze bgp validate` exits with code 1
- Stderr contains the expected substring OR matches the regex pattern

### Tagging tests to an RFC requirement

A test that enforces an RFC MUST can bind itself to the requirement so the coverage
gate can see it. Add an `RFC requirement:` tag naming the requirement id and the
polarity it exercises. It works in both test styles:

```go
// RFC requirement: RFC7606-7.1-1 negative -- ORIGIN length 2 is treated as withdraw
func TestRFC7606MalformedOriginLength(t *testing.T) { ... }
```

```
# test/plugin/rfc7606-withdraw.ci  -- must be a '#' comment at the start of a line
# RFC requirement: RFC7606-7.1-1 negative
```

- `polarity` is mandatory and must be `positive` or `negative`; a gated MUST needs
  both. Text after `--` is an optional note.
- In `.ci` files the tag is only recognised as a line-start `#` comment; `#` inside a
  `terminator=` block is file content, not a tag, and is skipped.
- On a table-driven test, put the tag on the line of the enforcing case, not only on
  the function, so deleting that case re-opens the coverage.
- Once tagged, a test's behaviour must not be edited without user approval — the
  `rfc-tagged-test` hook blocks it. Fix the code, not the test.

`make ze-rfc-check` verifies every enrolled MUST has its pair of tags (or a reasoned
annotation) and that `ai/RFC-REQUIREMENTS.md` — the generated requirement → test map —
is fresh. See `docs/contributing/rfc-implementation-guide.md` §9.7 and
`ai/skills/ze-rfc.md`.

---

## Architecture

### Package: `internal/test/runner/`

| File | Purpose |
|------|---------|
| `color.go` | TTY-aware ANSI colors |
| `decode.go` | BGP message decoding for failure reports |
| `display.go` | Live progress display |
| `json.go` | JSON validation: transform envelope → plugin format |
| `limits.go` | ulimit check and auto-raise |
| `ports.go` | Dynamic port range allocation |
| `record.go` | Test record with state machine, Tmpfs file storage |
| `report.go` | AI-friendly failure reports |
| `runner.go` | Test execution engine, Tmpfs runtime support |
| `stress.go` | Iteration stats and timing for -c/--count |
| `timing.go` | Per-test timing baseline with auto-timeout |
| `timing_test.go` | Timing baseline tests |
| `parallel.go` | Parallel test execution |
| `tmpfs_test.go` | Tmpfs parsing integration tests |

<!-- source: internal/test/runner/color.go -- ANSI colors -->
<!-- source: internal/test/runner/decode.go -- BGP message decoding -->
<!-- source: internal/test/runner/display.go -- live progress display -->
<!-- source: internal/test/runner/json.go -- JSON validation -->
<!-- source: internal/test/runner/limits.go -- ulimit check -->
<!-- source: internal/test/runner/ports.go -- port allocation -->
<!-- source: internal/test/runner/record.go -- test record state machine -->
<!-- source: internal/test/runner/report.go -- failure reports -->
<!-- source: internal/test/runner/runner.go -- test execution engine -->
<!-- source: internal/test/runner/stress.go -- stress iteration stats -->
<!-- source: internal/test/runner/timing.go -- per-test timing baseline -->
<!-- source: internal/test/runner/parallel.go -- parallel test execution -->

### Package: `internal/tmpfs/`

| File | Purpose |
|------|---------|
| `tmpfs.go` | Tmpfs parser and writer |
| `limits.go` | Configurable limits from environment |
| `security.go` | Path validation (traversal, escape) |
| `cleanup.go` | Signal handling for temp cleanup |

### Entry Point: `internal/test/cli/*.go`

<!-- source: internal/test/cli/register.go -- test runner entry point -->
<!-- source: internal/test/cli/cmd_bgp.go -- bgp test subcommand -->
<!-- source: internal/test/cli/cmd_syslog.go -- syslog server subcommand -->
<!-- source: internal/test/mock/rpki/rpki.go -- RPKI mock RTR subcommand -->

Subcommand-based CLI with `bgp` for BGP test execution, `syslog` for syslog server, and `rpki` for deterministic RPKI mock RTR server.

### ze-test rpki

Deterministic RTR (RFC 8210) cache server for RPKI functional tests. Auto-generates VRPs for all /8 prefixes based on the first octet modulo 3:

| First octet % 3 | VRP | Result (AS 65001) |
|-----------------|-----|-------------------|
| 0 | ASN=65001, maxLen=/32 | Valid |
| 1 | ASN=65099, maxLen=/32 | Invalid |
| 2 | No VRP | NotFound |

Usage: `ze-test rpki --port 3323 [--valid-asn 65001] [--invalid-asn 65099]`

### Security

- Path traversal protection on `option:file:` and `.run` scripts
- Process isolation via `Setpgid`
- Context timeouts on all execution
- Dynamic port allocation prevents conflicts

### ExaBGP Compatibility Test Ports

ExaBGP compatibility tests (`make ze-exabgp-test` or `bin/ze-test exabgp --all`) use OS-assigned dynamic ports. The mock BGP server (`test/exabgp-compat/bin/bgp`) binds to port 0, receives an OS-assigned port, and prints `PORT <N>` to stdout. The Go runner reads this line from the server process output and passes the discovered port to the ExaBGP wrapper client. This eliminates port collisions when running concurrent test instances. Use `--server ID --port N` and `--client ID --port N` for split-terminal debugging.
<!-- source: test/exabgp-compat/bin/bgp -- dynamic port binding and PORT line output -->
<!-- source: internal/test/cli/cmd_exabgp.go -- waitExaBGPPort and split debug modes -->

### ExaBGP Verify Output

The ExaBGP compatibility runner is now integrated into `ze-test`, so it uses the
same `--list`, `--all`, `--start`, `--pattern`, per-test result lines, periodic
progress, and summary format as the other functional suites. `make
ze-exabgp-test` runs:

```bash
uv run --with paramiko bin/ze-test exabgp --all --timeout 180s
```
<!-- source: internal/test/cli/cmd_exabgp.go -- standard selection and progress output -->
<!-- source: Makefile -- ze-exabgp-test -->

---

## Per-Test Timing Baseline

`ze-test` maintains a rolling timing baseline in `tmp/test-timings.json` that enables two features:
<!-- source: internal/test/runner/timing.go -- TimingEntry, LoadTimings, SaveTimings -->

**Auto-timeout:** Each test's timeout is calculated as `min(global_timeout, max(5s, 5x baseline_avg))`. A test that normally takes 500ms gets a 5s timeout instead of the default 15s. This catches hangs in seconds rather than waiting for the global timeout. Explicit `option=timeout:value=` in the `.ci` file always takes precedence.

**Slow detection:** Tests exceeding 2x their baseline average are flagged in the summary output. Investigate performance regressions before ignoring these warnings.

The baseline uses an exponential moving average (EMA, alpha=0.3) and requires 3 samples before it is used for auto-timeout. The timings file is capped at 10 MB; if it grows beyond that, timing data is reset.

---

## Route Delivery Synchronization

Plugin test scripts use `wait_for_ack()` from `test/scripts/ze_api.py` to ensure routes have been delivered to peers before proceeding. It dispatches `request quiesce` (the `ze-system:quiesce` barrier), which drains BOTH the BGP forward pool (`bgp-forward-pool`) AND each peer's initial-sync opQueue (`bgp-peer-sync` / `DrainPeerSync`) before replying, so a route sent during a peer's establishment window is on the wire (past its EOR) when the call returns. It is a deterministic barrier with no `time.sleep`.
<!-- source: test/scripts/ze_api.py -- wait_for_ack() function -->
<!-- source: internal/component/plugin/server/quiesce.go -- request quiesce / quiesceAll / registerReactorQuiescer -->
<!-- source: internal/component/bgp/reactor/reactor_api.go -- DrainPeerSync (bgp-peer-sync quiescer) -->
<!-- source: internal/component/bgp/reactor/forward_pool_barrier.go -- forward pool flush barrier -->

**When writing new plugin tests:**

| Pattern | Use |
|---------|-----|
| After a batch of `send()` calls | Call `wait_for_ack()` before checking results or sending dependent commands |
| Between independent `send()` calls | No synchronization needed (FIFO ordering per peer is guaranteed) |
| Before `wait_for_shutdown()` | Call `wait_for_ack()` to ensure all routes hit the wire |

**Do NOT use `time.sleep()` for forward delivery synchronization.** The flush barrier is deterministic and does not depend on timing. Use `time.sleep()` only for non-forward-pool concerns (session establishment, RPKI cache, event propagation).

---

## Editor Tests (.et format)

Editor tests (`test/editor/`) verify the interactive TUI editor and CLI using headless keystroke simulation. Run all editor tests with `make ze-editor-test` or `bin/ze-test editor --all`; select one with `bin/ze-test editor ID_OR_NAME`.

<!-- source: internal/component/cli/testing/parser.go -- .et file parser -->
<!-- source: internal/test/cli/cmd_editor.go -- editorMain selection flags -->

### Key Directives

| Directive | Example | Purpose |
|-----------|---------|---------|
| `tmpfs=` | `tmpfs=test.conf:terminator=EOF` | Embed test files |
| `option=file:path=` | `option=file:path=test.conf` | Config file to load |
| `option=mode:value=` | `option=mode:value=command` | Command-only mode (no editor) |
| `option=history:store` | `option=history:store` | Enable zefs-backed history persistence |
| `input=type:text=` | `input=type:text=show` | Type text |
| `input=enter/up/down/tab` | `input=enter` | Press named key |
| `expect=input:value=` | `expect=input:value=show` | Assert input buffer content |
| `expect=mode:is=` | `expect=mode:is=command` | Assert editor mode |
| `restart=` | `restart=editor` | Simulate exit + relaunch (blob store persists) |

---

## Fuzz Testing

Ze includes 57 fuzz targets covering all wire parsers, NLRI codecs, config
parsing, cryptographic operations, protocol state machines, and
receiver/server-facing parsers (BMP, RADIUS, DHCP). Fuzz tests catch crashes,
panics, and memory corruption on malformed input.

```bash
make ze-fuzz-test                                    # All fuzz targets, 10s each
make ze-fuzz-one FUZZ=FuzzParseUpdate TIME=30s       # Single target, custom duration
```
<!-- source: mk/test-fuzz.mk -- ze-fuzz-test -->

Fuzz tests are not part of `make ze-verify` (they're time-bounded, not pass/fail
in the traditional sense). Run them periodically or before releases.

### Fuzz Target Areas

| Area | Targets | Examples |
|------|---------|---------|
| BGP messages | 6 | `FuzzParseHeader`, `FuzzUnpackOpen`, `FuzzUnpackUpdate`, `FuzzUnpackNotification` |
| BGP attributes | 8 | `FuzzParseOrigin`, `FuzzParseMED`, `FuzzParseASPath`, `FuzzParseCommunity` |
| NLRI codecs | 13 | `FuzzParseVPN`, `FuzzParseEVPN`, `FuzzParseFlowSpec`, `FuzzParseBGPLS`, `FuzzParseMUP` |
| Wire encoding | 4 | `FuzzParseIPv4Prefixes`, `FuzzParseIPv6Prefixes`, `FuzzRewriteASPath`, `FuzzParseNLRIs` |
| Config parser | 2 | `FuzzConfigParser`, `FuzzTokenizer` |
| L2TP | 4 | `FuzzParseMessageHeader`, `FuzzAVPIterator`, `FuzzHiddenDecrypt`, `FuzzOnReceiveSequence` |
| BFD | 3 | `FuzzParseControl`, `FuzzParseAuth`, `FuzzAuthDigest` |
| PPP | 6 | `FuzzParseLCPPacket`, `FuzzParseCHAPResponse`, `FuzzParsePAPRequest`, `FuzzParseFrame` |
| TACACS+ | 2 | `FuzzTacacsPacketUnmarshal`, `FuzzTacacsEncryptDecrypt` |
| Receiver/server parsers | 3 | `FuzzDecodeBMPTLV`, `FuzzDecodeRADIUSVSA`, `FuzzDHCPHandle` |
| Other | 6 | `FuzzHandleRoundTrip`, `FuzzEncodeDecode`, `FuzzScanner`, `FuzzParseRDString` |
<!-- source: internal/component/bgp/message/fuzz_test.go -- BGP message fuzz targets -->
<!-- source: internal/component/bgp/plugins/bmp/fuzz_test.go -- FuzzDecodeBMPTLV -->
<!-- source: internal/component/radius/fuzz_test.go -- FuzzDecodeRADIUSVSA -->
<!-- source: internal/plugins/dhcpserver/fuzz_test.go -- FuzzDHCPHandle -->
<!-- source: internal/core/bgp/attribute/builder_parse_fuzz_test.go -- attribute parser fuzz targets -->
<!-- source: internal/component/bgp/wireu/prefix_fuzz_test.go -- NLRI prefix fuzz targets -->
<!-- source: internal/component/config/fuzz_test.go -- config parser fuzz targets -->
<!-- source: internal/component/l2tp/ -- L2TP wire fuzz targets -->
<!-- source: internal/component/bfd/ -- BFD packet/auth fuzz targets -->
<!-- source: internal/component/l2tp/ppp/ -- PPP frame/protocol fuzz targets -->

### Writing a New Fuzz Target

```go
func FuzzParseMyProtocol(f *testing.F) {
    // Seed with known-good wire bytes
    f.Add([]byte{0x01, 0x02, 0x03})

    f.Fuzz(func(t *testing.T, data []byte) {
        // Must not panic on any input
        result, err := ParseMyProtocol(data)
        if err != nil {
            return  // Parse errors are expected
        }
        // Optionally: round-trip check
        encoded := result.Marshal()
        // ...
    })
}
```

---

## Live Tests (Docker + Internet)

Live tests run against real external infrastructure inside Docker containers.
They are **not** part of `make ze-verify` and require both Docker and internet access.

<!-- source: internal/component/bgp/plugins/rpki/rpki_live_test.go -- TestLiveRPKIValidation -->

```bash
make ze-live-test    # Run all live tests
```

### Build Tag

Live tests use `//go:build live`. They are excluded from all normal test targets
(`ze-unit-test`, `ze-functional-test`, `ze-verify`). The `ze-live-test` make target
passes `-tags live` to include them.

### RPKI Live Test

Starts a [stayrtr](https://github.com/bgp/stayrtr) container that fetches real-world
RPKI data, connects ze's RTR client, and validates known prefixes:

| Prefix | Origin AS | Expected | Owner |
|--------|-----------|----------|-------|
| `1.1.1.0/24` | 13335 | Valid | Cloudflare DNS |
| `8.8.8.0/24` | 15169 | Valid | Google DNS |
| `82.212.0.0/16` | 64496 | NotFound | No ROA coverage |
| `1.1.1.0/24` | 64496 | Invalid | Wrong origin for covered prefix |

**Requirements:** Docker, internet access (fetches ~5 MB rpki.json from Cloudflare).
**Timeout:** 180s (includes image pull, data fetch, RTR sync).
**Skip behavior:** Test skips gracefully if Docker is unavailable or image cannot be pulled.

---

## Integration Tests (Network Namespaces)

Integration tests exercise Linux dataplane packages against the real kernel inside
ephemeral network namespaces. They require `CAP_NET_ADMIN` (typically root) and are
excluded from all normal test targets.

<!-- source: internal/component/iface/integration_helpers_linux_test.go -- withNetNS, waitForEvent -->
<!-- source: internal/component/iface/config_integration_linux_test.go -- iface config ownership and rollback integration tests -->
<!-- source: internal/plugins/fib/kernel/integration_linux_test.go -- route ownership integration tests -->
<!-- source: internal/plugins/firewall/nft/integration_linux_test.go -- nft table ownership integration tests -->
<!-- source: internal/plugins/traffic/netlink/integration_linux_test.go -- tc qdisc restore integration tests -->

### Running on Linux

```bash
make ze-integration-test           # Run all integration tests
make ze-integration-iface-test     # Run iface integration tests only
make ze-integration-fib-test       # Run FIB kernel integration tests only
make ze-integration-firewall-test  # Run nft firewall integration tests only
make ze-integration-traffic-test   # Run traffic-control netlink integration tests only
```

### Running on macOS (QEMU)

macOS cannot run these tests natively. Use the QEMU Alpine VM:

```bash
make ze-qemu-integration-test     # Boots Alpine VM, runs all integration tests
```

This is the standard workflow for macOS developers. The QEMU runner boots an
Alpine Linux VM, mounts the repo via virtio-9p, and runs `go test -tags integration`
inside the VM. First run downloads the Alpine ISO and Go toolchain (~1 min);
subsequent runs reuse the cache (~30s boot + test time).

See [testing/qemu-integration.md](architecture/testing/qemu-integration.md) for
details on how to write QEMU integration tests and add new packages.

The `trafficusage` eBPF/TCX tests in `internal/plugins/trafficusage/`
(`//go:build integration && linux`) are auto-discovered by
`make ze-qemu-integration-test` on the Alpine kernel: `program_test.go` runs the
pure-Go eBPF programs through `BPF_PROG_TEST_RUN`, and
`attach_integration_linux_test.go` attaches them to a veth pair, injects packets
with AF_PACKET, and scrapes `/metrics`. A dedicated
`make ze-qemu-traffic-usage-test` runs the same tests against Ze's own runtime
kernel (`tmp/kernel/vmlinuz`, built by `make ze-kernel`), which carries the
required `CONFIG_BPF_SYSCALL`, `CONFIG_BPF_JIT`, and `CONFIG_VETH`.
<!-- source: internal/plugins/trafficusage/program_test.go -- BPF_PROG_TEST_RUN eBPF program tests -->
<!-- source: internal/plugins/trafficusage/attach_integration_linux_test.go -- veth + AF_PACKET + /metrics scrape -->

### Deployment Evidence

```bash
make ze-deployment-preflight       # Strict tool check for complete deployment evidence
make ze-release-check              # Run clean Docker ze-verify release evidence
make ze-deployment-vpp-test        # Run real VPP daemon FIB add/withdraw evidence
make ze-deployment-l2tp-test       # Run real xl2tpd LAC control/session evidence
make ze-deployment-l2tp-ppp-test   # Run real xl2tpd/pppd PPP/NCP evidence on Linux
make ze-install-qemu-test       # Run PXE installer QEMU evidence
make ze-install-iso-qemu-test   # Run appliance ISO installer QEMU evidence
make ze-deployment-l2tp-ppp-docker-test # Run L2TP PPP/NCP peer-isolated Docker lab (Ze LNS + LAC + FRR)
```

The `ze-deployment-*` prefix marks deployment-grade external evidence;
Docker-specific variants include `docker` in the target name.

`ze-deployment-preflight` is strict: Docker-backed substitutes are reported, but
the target fails until target-runner evidence and full PPP/NCP L2TP peer
requirements are available. For Ze's current LNS path, that means a LAC peer
such as `xl2tpd`, `pppd`, `/dev/ppp`, `iproute2`, and PPPoL2TP kernel support.

### Build Tag

Integration tests use `//go:build integration && linux`. They are excluded from
`ze-unit-test`, `ze-functional-test`, and `ze-verify`. The `ze-integration-*`
make targets pass `-tags integration` to include them.

### How They Work

Each test calls `withNetNS(t, func() { ... })` which:

1. Locks the goroutine to its OS thread (`runtime.LockOSThread`)
2. Creates a named network namespace (`netns.NewNamed`)
3. Switches into it (`netns.Set`)
4. Runs the test function (creating interfaces, addresses, etc.)
5. Restores the original namespace and deletes the test namespace in `t.Cleanup`

If namespace creation fails (missing `CAP_NET_ADMIN`), the test is skipped with `t.Skip`.

### Test Categories

| File | Covers | Tests |
|------|--------|-------|
| `manage_integration_linux_test.go` | Interface CRUD, addresses, MTU | 9 tests |
| `config_integration_linux_test.go` | Config apply ownership scope, reload deletion scope, rollback of created links | 3 tests |
| `monitor_integration_linux_test.go` | Netlink event monitoring | 5 tests |
| `sysctl_integration_linux_test.go` | Real /proc/sys writes | 2 tests |
| `mirror_integration_linux_test.go` | tc qdisc/filter setup | 5 tests |
| `dhcp_integration_linux_test.go` | DHCPv4 with in-process server | 2 tests |
| `migrate_integration_linux_test.go` | Make-before-break migration | 2 tests |

Additional dataplane integration packages:

| File | Covers | Tests |
|------|--------|-------|
| `internal/plugins/fib/kernel/integration_linux_test.go` | FIB route add/remove/replace, restart sweep, producer protocol isolation, flush-on-stop | 8 tests |
| `internal/plugins/firewall/nft/integration_linux_test.go` | nft table ownership, same-instance cleanup, restart reapply preservation | 2 tests |
| `internal/plugins/traffic/netlink/integration_linux_test.go` | real tc qdisc snapshot, persisted restore after backend restart | 1 test |
| `internal/plugins/iface/netlink/vlanqoslab_integration_linux_test.go` | VLAN QoS wire-level: egress PCP on wire, ingress PCP classification, DSCP-to-PCP full chain | 3 tests |
| `internal/plugins/trafficusage/program_test.go` | Pure-Go eBPF programs via BPF_PROG_TEST_RUN: per-IP and per-(port, protocol) byte accounting, ICMP port-0, non-IPv4/truncated pass-through, accumulation | 10 tests |
| `internal/plugins/trafficusage/attach_integration_linux_test.go` | Real TCX attach on a veth pair, AF_PACKET frame injection, clean Stop detach, `ze_traffic_usage_*` `/metrics` scrape | 2 tests |

### Shared Helpers

`integration_helpers_linux_test.go` provides:

| Helper | Purpose |
|--------|---------|
| `withNetNS(t, fn)` | Ephemeral namespace wrapper |
| `waitForEvent(t, bus, topic, timeout)` | Poll collectingBus for an event |
| `linkExists(name)` | Check if interface exists via netlink |
| `hasAddress(iface, cidr)` | Check if address is on interface |
| `requireLinkUp(t, name)` | Assert link is UP |
| `createDummyForTest(t, name)` | Create dummy with cleanup |
| `createVethForTest(t, name, peer)` | Create veth pair with cleanup |

---

## L2TP Tests

L2TP functional tests (`test/l2tp/`) verify tunnel lifecycle, session
negotiation, authentication, IP pool, and teardown over real loopback UDP.
Run with `bin/ze-test l2tp`.

> **In the default release gate.** The in-tree L2TP `.ci` tests are included in
> `make ze-verify` / `make ze-functional-test` and can be run directly with
> `make ze-l2tp-test` or `bin/ze-test l2tp --all`. External-peer and PPP
> dataplane evidence remain separate deployment targets.

```bash
ze-test l2tp --list    # List available tests
ze-test l2tp --all     # Run all tests
```

For external-peer evidence, run `make ze-deployment-l2tp-test`. It uses a
real `xl2tpd` LAC to establish the L2TP control tunnel and incoming-call session
against Ze. It intentionally does not claim full PPP/NCP dataplane proof when
the Docker host lacks the `l2tp_ppp` kernel module.

For full PPP/NCP peer evidence, run `make ze-deployment-l2tp-ppp-test` on a
Linux host or target runner with `xl2tpd`, `pppd`, `ping`, `/dev/ppp`,
`iproute2`, and PPPoL2TP kernel support. The target refuses skip-kernel-probe
mode, creates peer-isolated Ze and LAC network namespaces, starts Ze as LNS,
drives a real `xl2tpd`/`pppd` LAC across the veth underlay, waits for PPP
LCP/IPCP completion, verifies log field correctness (assigned address and pppN
interface name), checks the resulting Ze and LAC `pppN` address state, pings
the LNS through the PPP tunnel from the LAC namespace to prove dataplane
connectivity, observes subscriber route injection, and verifies teardown
returns both namespaces' kernel L2TP/PPP state to their initial snapshots.

On macOS, `make ze-deployment-l2tp-ppp-docker-test` runs that same proof in
privileged Linux containers. Docker is only a Linux userspace wrapper here: the
test still fails unless the Docker host kernel has `/dev/ppp`, Generic Netlink
L2TP, and PPPoL2TP support.

For appliance evidence, run `make ze-kernel` first, then
`make ze-deployment-gokrazy-l2tp-ppp-test` on a Linux host or target runner
with QEMU and the same PPPoL2TP LAC-side kernel support. `make ze-kernel`
defaults to `KERNEL_BUILDER=docker`; set `KERNEL_BUILDER=qemu` when you need to
force the shared QEMU backend. In both cases it uses the shared kernel builder
and tracked runtime fragments before copying vmlinuz/modules/DTBs into the
gokrazy module cache. The target builds a temporary gokrazy image with an L2TP
first-boot template and proof-only runtime environment
(`ze.l2tp.ncp.enable-ipv6cp=false`, because the static pool is IPv4-only),
boots it under QEMU with UDP 1701 forwarded into the appliance, drives a real
`xl2tpd`/`pppd` LAC from a Linux namespace, verifies PPP/IPCP and
LAC `pppN` address state, pings the Ze LNS address through PPP, and observes
appliance route inject/withdraw logs.
<!-- source: mk/gokrazy.mk -- ze-kernel -->
<!-- source: gokrazy/kernel/Makefile -- all -->

### Tunnel lifecycle

| Test | File | What it verifies |
|------|------|-----------------|
| SCCRQ handshake | `test/l2tp/handshake-sccrq.ci` | Python client sends SCCRQ hex, receives SCCRP |
| Full handshake | `test/l2tp/handshake-full.ci` | SCCRQ/SCCRP/SCCCN/ZLB exchange with challenge/response |
| Bad challenge | `test/l2tp/bad-challenge-response.ci` | Wrong challenge response triggers StopCCN RC=4 |

### Session lifecycle

| Test | File | What it verifies |
|------|------|-----------------|
| Incoming session (LNS) | `test/l2tp/session-incoming-lns.ci` | ICRQ/ICRP/ICCN exchange establishes session |
| CDN teardown | `test/l2tp/session-cdn-teardown.ci` | CDN tears down one session cleanly |
| StopCCN cascade | `test/l2tp/session-stopccn-cascade.ci` | StopCCN tears down tunnel and all its sessions |

### Authentication and IP pool

| Test | File | What it verifies |
|------|------|-----------------|
| Auth + pool | `test/l2tp/session-auth-pool.ci` | Full session with auth-local + pool allocation |
| Auth-local config | `test/l2tp/auth-local-config.ci` | Static user config parsed and auth works |
| RADIUS basic | `test/l2tp/auth-radius-basic.ci` | RADIUS Access-Request sent, session authenticated |
| RADIUS reject | `test/l2tp/auth-radius-reject.ci` | RADIUS Access-Reject fails the session |
| Pool basic | `test/l2tp/pool-basic.ci` | Pool allocates and releases addresses |
| Pool minimal range | `test/l2tp/pool-minimal-range.ci` | Single-address pool boundary case |
| Re-auth interval | `test/l2tp/reauth-interval-clamp.ci` | Safety floor clamps the re-auth interval |

### Config parsing

| Test | File | What it verifies |
|------|------|-----------------|
| Minimal config | `test/parse/l2tp-minimal.ci` | `l2tp { server main { port 1701 } }` parses |
| Bad port | `test/parse/l2tp-bad-port.ci` | `port 0` rejected |
| Unknown field | `test/parse/l2tp-unknown-field.ci` | Unknown key rejected with suggestion |
| Max sessions | `test/parse/l2tp-max-sessions.ci` | `max-sessions` value accepted |
| Auth policy | `test/parse/l2tp-auth-policy.ci` | `auth-method` and `allow-no-auth` values accepted |
| Hello retries | `test/parse/l2tp-hello-retries-parse.ci` | `hello-retries` (dead-peer threshold) accepted |
| Hello retries range | `test/parse/l2tp-hello-retries-range.ci` | `hello-retries 256` rejected (uint8 range) |

<!-- source: internal/test/cli/register.go -- l2tpCmd runner dispatch -->
<!-- source: internal/test/runner/record_parse.go -- .ci discovery and directive parsing -->

### L2TP scale tests

Scale tests (`test/l2tp-scale/`) validate Ze's L2TP control plane at
2000 concurrent sessions across 10 tunnels. They run on loopback (no
root, no Docker, no kernel modules) and measure session establishment
rate, RADIUS round-trip handling, pool allocation correctness, and
teardown completeness.

The test tooling lives in `ze-test l2tp-scale`, which bundles a Go LAC
simulator (speaking real L2TP wire protocol) and an embedded mock RADIUS
server. A Python harness (`test/l2tp-scale/harness.py`) orchestrates Ze
and the simulator.

```bash
python3 test/l2tp-scale/run.py                  # run all scenarios
python3 test/l2tp-scale/run.py 2k-sessions       # run specific scenario
ze-test l2tp-scale --help                        # simulator CLI help
```

| Scenario | Directory | What it validates |
|----------|-----------|-------------------|
| 2k sessions | `test/l2tp-scale/2k-sessions/` | All 2000 sessions established |
| Clean teardown | `test/l2tp-scale/clean-teardown/` | No resource leaks after teardown |
| Pool exhaustion | `test/l2tp-scale/pool-exhaustion/` | Sessions beyond pool size rejected |
| Slow RADIUS | `test/l2tp-scale/slow-radius/` | Sessions established under 500ms RADIUS delay |

<!-- source: internal/test/cli/cmd_l2tp_scale.go -- LAC simulator + mock RADIUS -->

---

**Updated:** 2026-05-08
