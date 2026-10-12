# Spec: bsd-family-interface-backend

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Depends | Phase 0 compile fixes for freebsd/openbsd and the openbsd `vet-platforms` row (in flight in another session on 2026-10-12, no spec file: done elsewhere, not specced here); `plan/spec-bsd-qemu-guest-kind.md` for every FreeBSD/OpenBSD runtime test |
| Phase | 0/4 (Do not start until the owner approves; the five Open Owner Questions below are unanswered) |
| Handoff | - |
| Updated | 2026-10-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The owner asked for macOS, FreeBSD and OpenBSD support "as much as possible". Today
Ze builds on those systems but manages nothing on them: the iface `netlink` package
compiles a stub on every non-Linux GOOS whose only working methods are the two
listing calls, `fib-kernel` rejects every route, and OSPF, IS-IS and single-hop BFD
fail closed. This spec designs one shared BSD-family backend built on the routing
socket (`AF_ROUTE`), the interface ioctls and `sysctl(NET_RT_*)`, the mechanisms FRR
zebra and BIRD use on the same three systems, and lands it in four phases:

| Phase | Delivers |
|-------|----------|
| P1 | Shared routing-socket backend: interface listing, link monitor, IPv4/IPv6 addresses, admin up/down, MTU, MAC, routes, route lookup, statistics, media (speed/duplex), cloned-interface create/destroy; the `fib-kernel` backend on the same message encoder; a per-OS `default_*.go`; every other Backend method an explicit, named unsupported error |
| P2 | VLAN, bridge with STP, veth equivalents (FreeBSD `epair(4)`, OpenBSD `pair(4)`, macOS `feth`), gif/gre/vxlan tunnels, WireGuard, neighbor listing |
| P3 | Protocol transports: OSPF raw IP proto 89, BFD single-hop receive TTL and interface binding, IS-IS over BPF, TCP-MD5 through a PF_KEY SA writer on FreeBSD and OpenBSD |
| P4 | VRF: FreeBSD FIBs and OpenBSD routing tables/rdomains; MPLS on OpenBSD; sysctl writes for FreeBSD and OpenBSD |

Out of this spec, each in its own later spec (skeletons written with this one): pf
firewall backend (`plan/spec-bsd-pf-firewall-backend.md`), PF_KEY IPsec dataplane
(`plan/spec-bsd-pfkey-ipsec.md`), L2TP/PPPoE (`plan/spec-bsd-l2tp-pppoe.md`), traffic
shaping (`plan/spec-bsd-traffic-shaping.md`), VRRP against carp(4)
(`plan/spec-bsd-vrrp-carp.md`), and the QEMU BSD guest kind the tests need
(`plan/spec-bsd-qemu-guest-kind.md`). Owner question 5 asks whether that split holds.

## Open Owner Questions (UNANSWERED: the spec stays in `design` until each is answered)

| # | Question | Why it matters | Main-thread recommendation |
|---|----------|----------------|----------------------------|
| Q1 | Is macOS a deployment target or a developer platform only? | Deployment means macOS owes P2 (feth, userspace WireGuard), P3 transports, and a support claim on `docs/features/interfaces.md`; dev-only means P1 on darwin is enough and the darwin rows say "development" | Dev-only. macOS has no MPLS, no VRF, no TCP-MD5 and no kernel WireGuard |
| Q2 | FreeBSD or OpenBSD as the router target, or both? | Decides which guest carries the interop scenarios and which OS P4 targets first | Both for P1 and P2; OpenBSD leads P4 because it alone has kernel MPLS and its rtables/rdomains carry VRF more completely than FreeBSD FIBs |
| Q3 | Route ownership and metric on BSD | Linux identifies Ze routes by `rtm_protocol` (`internal/core/rtproto/rtproto.go`) and orders them by metric; BSD has only `RTF_PROTO1`..`RTF_PROTO3` and no metric except OpenBSD `rtm_priority`. `Backend.AddRoute(..., metric, proto)` needs a per-OS meaning | Mark Ze routes `RTF_PROTO1` and keep a Ze-side ownership record (prefix, gateway, owner `rtproto.Proto`) so the owner a Linux `rtm_protocol` carries is not lost; use `rtm_priority` for metric on OpenBSD; refuse a non-default metric on FreeBSD and macOS with a named error |
| Q4 | Minimum OS versions | Fixes which struct layouts, `RTM_VERSION`, sysctl MIBs and ioctls the golden-byte tests pin | FreeBSD 14, OpenBSD 7.5, macOS 14 |
| Q5 | Are pf, PF_KEY IPsec, L2TP/PPPoE and shaping separate later specs? | Keeps this spec to interface, route and transport work that one shared encoder serves | Yes, as the skeletons named in the Task |

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` - the Platform rows and the "Stub (non-Linux)" section describe today's non-Linux behavior
  → Constraint: the Platform table rows "macOS / Darwin" and "FreeBSD / OpenBSD" read `missing`; each phase that lands moves its row, and the per-method coverage table (columns netlink / VPP / stub) gains a BSD column, in the same commit as the code
  → Constraint: the stub section promises that config-driven reconciliation which creates, addresses or changes an interface "sees the explicit error and rejects under exact-or-reject"; every method P1 does not implement keeps that promise with a named error
- [ ] `docs/architecture/iface/netlink-monitor.md` - how the Linux monitor feeds the event bus
  → Constraint: the BSD monitor publishes the same event types on the same bus (`StartMonitor(eb ze.EventBus)`), so consumers stay platform-blind
- [ ] `docs/architecture/iface/management.md` - interface ownership and reconciliation
  → Constraint: device and address ownership (`address_owner.go`, `device_owner.go`) are platform-neutral and must not grow per-OS branches
- [ ] `docs/architecture/core-design.md` - FIB backend, sysctl backend and TCP-MD5 sections (named by the `// Design:` headers of `fib/kernel/backend.go`, `sysctl/backend_darwin.go`, `network/md5_freebsd.go`)
  → Constraint: FIB backends are chosen at build time through `newBackend()` behind build tags; the BSD fib backend is one more build-tagged file, not a registry entry
- [ ] `docs/architecture/bfd.md` - BFD transport socket options
  → Constraint: single-hop BFD fails closed on non-Linux today because the receive TTL reads zero; P3 must read the real TTL, never relax the check
- [ ] `docs/architecture/testing/qemu-integration.md` - guest kinds, `option=needs-linux`, namespace isolation
  → Constraint: the QEMU harness boots Alpine Linux guests only; FreeBSD/OpenBSD runtime tests depend on `plan/spec-bsd-qemu-guest-kind.md`
- [ ] `ai/rules/platform-linux.md` - Linux-only tests run through QEMU, never natively
  → Constraint: no rule says how BSD runtime tests run; the guest-kind spec owes `ai/rules/platform-bsd.md` (authored under `ai/rules/rule-format.md`) before P1 can claim a FreeBSD/OpenBSD runtime proof

### RFC Summaries (Scope: protocol, P3 only)
- [ ] `rfc/short/rfc2385.md` - TCP MD5 signature option
  → Constraint: a configured password that does not reach the kernel must be reported (today `tcpMD5Supported()` answers false on FreeBSD for exactly this reason); P3 makes it true only when Ze itself writes the SA
- [ ] `rfc/short/rfc5881.md` - BFD single-hop, TTL 255 on send and check on receive
  → Constraint: the receive check needs the received TTL (`IP_RECVTTL` on BSD) and the receiving interface (`IP_RECVIF`); without both the session stays fail-closed
- [ ] `rfc/short/rfc2328.md` - OSPFv2 runs directly over IP protocol 89 with multicast 224.0.0.5/6
  → Constraint: the BSD raw socket must learn the receiving interface per packet (`IP_RECVIF`) and the destination (`IP_RECVDSTADDR`), which BIRD uses on BSD in place of `IP_PKTINFO`

### Reference implementations (read in `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/upstream-src/`)
- [ ] FRR `frr-kernel_socket.c` `rtm_write` - sets `RTF_UP`, `RTF_GATEWAY`, `RTF_HOST`, `RTF_MPLS`, and always `RTF_PROTO1`; on read, `RTF_PROTO1` means `ZEBRA_FLAG_SELFROUTE`
  → Decision: Ze marks its routes `RTF_PROTO1`, the same convention as FRR and BIRD
  → Constraint: FRR and BIRD use the same bit, so `RTF_PROTO1` alone cannot tell Ze's routes from another daemon's on a shared host; the Ze-side ownership record (Q3) is what disambiguates
- [ ] FRR `frr-ioctl.c` `if_set_prefix_ctx` and `if_set_prefix6_ctx` - IPv4 addresses via `SIOCAIFADDR`/`SIOCDIFADDR`, IPv6 via `SIOCAIFADDR_IN6` with `struct in6_aliasreq`
  → Constraint: `in6_aliasreq` and `SIOCAIFADDR_IN6` are not in `golang.org/x/sys/unix`; Ze defines them per GOOS in hand-written files, each pinned by a golden-byte test against the struct size of the minimum version (Q4)
- [ ] BIRD `bird-krt-sock.c` `krt_send_route` - `RTF_UP | RTF_PROTO1`, `rtm_tableid` (OpenBSD), `rtm_priority` as metric (OpenBSD, `KRT_USE_METRIC`); FreeBSD multi-FIB via `SO_SETFIB` per socket and `setfib()` around sysctl scans
  → Decision: P4 follows BIRD's split: one routing socket per FreeBSD FIB with `SO_SETFIB`, OpenBSD `rtm_tableid` on a single socket
- [ ] BIRD `bird-sysio.h` - `IP_RECVDSTADDR`/`IP_RECVIF` for raw protocol sockets; `sk_set_md5_auth` writes the SAD through PF_KEY on FreeBSD (`USE_MD5SIG_SETKEY`) and only sets `TCP_MD5SIG` to `TCP_SIG_SPI` elsewhere
  → Decision: P3 writes the TCP-MD5 SA through PF_KEY on both FreeBSD and OpenBSD, then sets `TCP_MD5SIG`, so a configured password takes effect without setkey(8)

**Key insights:**
- One message encoder (`rt_msghdr` plus sockaddrs, padded per OS) serves iface routes, `fib-kernel`, `RouteLookup` (`RTM_GET`) and P4 table selection.
- Listing and dumps read `sysctl(NET_RT_IFLIST / NET_RT_DUMP)`; `golang.org/x/net/route` parses those but is not vendored (`vendor/golang.org/x/net/` has bpf, ipv4, ipv6, no `route/`), so P1 starts with `go mod vendor` of that package. Writing messages stays Ze's own encoder: the encoder must be buffer-first and cover every `RTM_CHANGE` address slot.
- WireGuard: `vendor/golang.zx2c4.com/wireguard/wgctrl/internal/` already carries `wgfreebsd` and `wgopenbsd` (kernel) and `wguser` (userspace, the only option on macOS).
- TCP-MD5 already has `internal/core/network/md5_freebsd.go` (flag only, `tcpMD5Supported()` false), `md5_darwin.go` (refuses), and OpenBSD falls to `md5_other.go`.
- sysctl already has `internal/component/sysctl/backend_darwin.go` (forwarding keys, read path); FreeBSD/OpenBSD use `backend_other.go`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/backend.go` - `Backend` interface, 43 methods, `RegisterBackend`, `LoadBackend`, `DefaultBackendName()`, `ErrCountersNotResettable`
- [ ] `internal/component/iface/default_other.go` - `//go:build !linux`, `defaultBackendName = ""`: no backend by default on any non-Linux OS
- [ ] `internal/component/iface/default_linux.go` - `defaultBackendName = "netlink"`
- [ ] `internal/component/iface/register.go` - falls back to `defaultBackendName` when the config names no backend
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `leaf backend` with `type string` and `default "netlink"`
- [ ] `internal/plugins/iface/netlink/register.go` - registers the name `netlink` on every GOOS, including the non-Linux stub
- [ ] `internal/plugins/iface/netlink/backend_other.go` - `//go:build !linux`, `stubBackend`, every method but listing returns `interface management not supported on <GOOS>`
- [ ] `internal/plugins/iface/netlink/show_other.go` - `ListInterfaces`/`GetInterface` through `net.Interfaces`
- [ ] `internal/plugins/fib/kernel/backend_other.go` - `//go:build !linux`, `unsupportedBackend`, every method returns `fib-kernel: unsupported platform <GOOS>`; `monitor_other.go` likewise
- [ ] `internal/core/rtproto/rtproto.go` - `Proto` is the Linux `rtm_protocol` value: `GTSM` 249, `FIBKernel` 250, `Static` 251, `PolicyRoute` 252, `Iface` 253
- [ ] `internal/plugins/ospf/transport/backend_other.go` - `ErrUnsupportedPlatform`, raw IPv4 transport Linux-only
- [ ] `internal/plugins/isis/transport/backend_other.go` - Linux-only stub
- [ ] `internal/component/bfd/transport/udp_other.go` - sets `IP_TTL=255` on send, rejects device binding, returns receive TTL 0 so single-hop fails closed
- [ ] `internal/core/network/md5_freebsd.go`, `md5_darwin.go` - FreeBSD sets the flag only and reports unsupported; darwin refuses
- [ ] `internal/component/sysctl/backend_darwin.go` - forwarding keys only, read through `unix.SysctlUint32`
- [ ] `.github/workflows/*.yml` - every job `runs-on: ubuntu-latest`, no macOS or BSD runner

**Behavior to preserve:**
- Linux behavior is untouched: `netlink` stays the Linux default and every Linux test stays green.
- The listing that `ze init` relies on keeps working on every non-Linux OS (today through `show_other.go`).
- A non-Linux OS this spec does not cover (Windows, NetBSD, DragonFly) keeps today's stub and its error text.
- Exact-or-reject: a config that asks for something the backend cannot do is rejected with a named error, never silently skipped.
- `tcpMD5Supported()` answers false on any OS where Ze does not itself install the key.

**Behavior to change:**
- darwin, freebsd and openbsd get a real backend (`routesock`) and it is their default.
- The netlink stub stops compiling on those three GOOS values (no-layering: replaced, not kept beside).
- `fib-kernel` installs, replaces, deletes and lists routes on those three.
- P3/P4 as in the Task table.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config tree `interface { ... }` and `backend` leaf, resolved by `internal/component/iface/register.go`.
- `fib-kernel` route events from the RIB/sysrib.
- Kernel routing-socket messages (`RTM_IFINFO`, `RTM_NEWADDR`, `RTM_DELADDR`, `RTM_ADD`, `RTM_DELETE`, `RTM_CHANGE`, `RTM_IFANNOUNCE` where present) read from an `AF_ROUTE` socket.

### Transformation Path
1. Config apply calls `Backend` methods (`AddAddress`, `SetMTU`, `AddRoute`, ...).
2. The `routesock` backend encodes an ioctl request (addresses, flags, MTU, MAC, clone create/destroy) or a routing-socket message into a pooled buffer.
3. The kernel answers: ioctl errno, or the echoed routing message carrying `rtm_errno`, matched by `rtm_seq` and `rtm_pid`.
4. The monitor goroutine reads the same socket family, drops its own echoes (`rtm_pid` equals Ze's pid), decodes the rest with the shared decoder and publishes the existing iface events on the event bus.
5. An overrun (`ENOBUFS` on read; on OpenBSD `SO_RERROR` reports it) triggers a full resync: `NET_RT_IFLIST` plus `NET_RT_DUMP`, diffed against the last snapshot.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to kernel (routes) | `AF_ROUTE` socket, `rt_msghdr` + sockaddrs, `RTM_VERSION` per OS | No |
| Ze to kernel (interfaces) | `ioctl` on an `AF_INET`/`AF_INET6` datagram socket (`SIOCSIFFLAGS`, `SIOCSIFMTU`, `SIOCSIFLLADDR`, `SIOCAIFADDR`, `SIOCDIFADDR`, `SIOCAIFADDR_IN6`, `SIOCDIFADDR_IN6`, `SIOCIFCREATE2`, `SIOCIFDESTROY`) | No |
| Kernel to Ze (state) | `sysctl` `NET_RT_IFLIST` (darwin `NET_RT_IFLIST2` for 64-bit counters), `NET_RT_DUMP` | No |
| Engine to iface plugin | existing `Backend` interface, unchanged signatures | No |

### Integration Points
- `iface.RegisterBackend("routesock", ...)` - new registration, discovered by `LoadBackend`.
- `iface/default_darwin.go`, `default_freebsd.go`, `default_openbsd.go` - per-OS default name `routesock`; `default_other.go` narrows to the remaining GOOS values.
- `fibkernel.newBackend()` - a `backend_bsd.go` behind `darwin || freebsd || openbsd` builds on the shared encoder package.
- `internal/core/network/md5_freebsd.go` and a new `md5_openbsd.go` - P3 PF_KEY SA writer.
- `internal/component/sysctl/backend_freebsd.go`, `backend_openbsd.go` - P4.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Config apply still calls `Backend`; only the implementation behind it is new |
| No unintended coupling (components stay isolated) | Yes | The encoder lives in one shared internal package imported by the iface backend and `fib-kernel`, both plugins; no component imports a plugin |
| No duplicated functionality (extends existing, does not recreate) | Yes | One encoder for iface routes, `fib-kernel` and P4 tables; listing reuses `x/net/route` parsing rather than a second parser |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Messages are encoded into pooled buffers with `WriteTo(buf, off) int`; decode walks the read buffer in place |
| Registration over hardcoding, outbound | Yes | `routesock` registers through `iface.RegisterBackend` in its own `register.go`; the per-OS default is a build-tagged constant, the existing pattern |
| Registration over hardcoding, inbound | No | The YANG `backend` leaf carries `default "netlink"` and its description names `netlink` and `vpp` by hand; `register.go` falls back to `defaultBackendName`. Resolved in P1 by removing the YANG default so the per-OS Go default applies, and by making the description and `ze:help` name the registry rather than a list (A-2) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Phase 0 (freebsd/openbsd compile fixes, openbsd `vet-platforms`) lands before P1 starts | Main-thread brief, 2026-10-12; `internal/le/go/vetplatforms/` exists | P1 starts on a tree that does not build for the target and absorbs compile fixes it does not own | `GOOS=freebsd` and `GOOS=openbsd` `go vet ./...` clean at P1 start | unvalidated |
| A-2 | Removing the YANG `default "netlink"` lets `register.go` apply the per-OS default without changing Linux behavior | `ze-iface-conf.yang` leaf `backend`; the `defaultBackendName` fallback in `register.go` | Linux configs without a `backend` leaf could resolve differently | Unit test over a tree without the leaf on linux and on darwin | unvalidated |
| A-3 | `golang.org/x/net/route` parses `NET_RT_IFLIST` and `NET_RT_DUMP` correctly on all three OS at the minimum versions | Widely used parser; not yet vendored | Ze needs its own decoder for listing too | Golden-byte decode tests over captured sysctl output from each OS | unvalidated |
| A-4 | Struct layouts (`rt_msghdr`, `if_msghdr`, `ifa_msghdr`, `in6_aliasreq`) are stable across the minimum versions in Q4 | FRR and BIRD compile one layout per OS | Per-version layouts, more golden files | Golden-byte tests on each guest at the minimum and current release | unvalidated |
| A-5 | GitHub macOS runners allow `sudo` for routing-socket writes on a `lo0` alias | Main-thread research | macOS runtime tests limited to read-only paths | One CI job that adds and removes a route on `lo0` | unvalidated |
| A-6 | The QEMU BSD guest kind exists before any FreeBSD/OpenBSD runtime AC is claimed | `plan/spec-bsd-qemu-guest-kind.md` | Runtime ACs cannot be proven; only golden-byte tests run | That spec closed | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `RTF_PROTO1` is shared with FRR and BIRD, so a sweep of "our" routes deletes another daemon's | Ownership test with a foreign `RTF_PROTO1` route present | Sweep only routes present in the Ze-side ownership record (Q3); never sweep by flag alone |
| R-2 | Monitor socket overrun loses events silently and Ze's view drifts | `ENOBUFS` on read, `SO_RERROR` on OpenBSD, a resync diff that is not empty | Large `SO_RCVBUF`; any overrun triggers a full resync and a counter increment; a test injects an overrun |
| R-3 | Own echoes processed as external changes, causing apply loops | Monitor test counts events after a Ze-originated add | Drop messages whose `rtm_pid` is Ze's pid and whose `rtm_seq` is outstanding |
| R-4 | darwin lacks `RTM_IFANNOUNCE`, so interface arrival and departure are missed | darwin monitor test creating a `feth` | On darwin detect add/remove from `RTM_IFINFO` index changes and a resync |
| R-5 | `ResetCounters` cannot reset kernel counters on BSD | Counter test | Return `ErrCountersNotResettable` (already defined) rather than fake a reset with a baseline offset; the CLI already names this error |
| R-6 | A non-default metric silently ignored on FreeBSD/macOS | AddRoute test with metric 100 | Named refusal error, per Q3 recommendation; OpenBSD maps to `rtm_priority` |
| R-7 | `in6_aliasreq` hand-defined with the wrong size corrupts the ioctl | Golden-byte size test fails, or `EINVAL` from the kernel | Size and field-offset tests per GOOS against the minimum version headers |
| R-8 | IPv6 address lifetimes (`ReplaceAddressWithLifetime`) differ from Linux semantics | Lifetime test reads back `ia6t_vltime`/`ia6t_pltime` | Map to `in6_addrlifetime`; refuse what cannot be expressed |
| R-9 | Tests that need root run unattended on a developer Mac | A darwin test that writes routes runs outside CI | Root-needing darwin tests gated by an explicit tag and the CI job only; `.claude/rules/sudo.md` forbids agent `sudo` outside the stress harness |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | On BSD hosts: wrong routes or addresses installed, or a foreign daemon's routes swept (R-1). Linux is not on the path except through the YANG default change (A-2) |
| How is it reverted? | Single commit per phase; reverting restores the stub on the three GOOS values |
| Who else touches this path? | Phase 0 compile-fix session; `plan/spec-vrf.md` (P4 VRF overlaps its model); `plan/immediate/spec-kernel-capability-gate.md` (Linux kernel capability doctor checks, which a BSD backend must not trip) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config with no `backend` leaf on darwin/freebsd/openbsd | → | `iface.LoadBackend` picks `routesock` | `TestDefaultBackendIsRoutesockOnBSD` (`internal/component/iface/default_bsd_test.go`) |
| Config `interface ethernet <name> address 192.0.2.1/24` | → | `routesock.AddAddress` encodes `SIOCAIFADDR` | `TestRoutesockAddAddressV4Golden` (`internal/plugins/iface/routesock/addr_test.go`) |
| Config IPv6 address | → | `routesock.AddAddress` encodes `SIOCAIFADDR_IN6` | `TestRoutesockAddAddressV6Golden` |
| `fib-kernel` route install | → | `fibkernel` BSD backend encodes `RTM_ADD` with `RTF_PROTO1` | `TestFibKernelBSDAddRouteGolden` (`internal/plugins/fib/kernel/backend_bsd_test.go`) |
| Kernel `RTM_IFINFO` on the routing socket | → | monitor publishes the link event | `TestRoutesockMonitorLinkEvent` (`internal/plugins/iface/routesock/monitor_test.go`) |
| `ze` daemon on a FreeBSD guest with an interface config | → | full apply through `routesock` | `test/bsd/iface-address-apply.ci` (runs under the BSD guest kind) |
| `ze` daemon on a macOS runner with a `lo0` alias config | → | full apply through `routesock` | `test/bsd/darwin-lo0-alias.ci` (macOS CI job) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | P1: config with no `backend` leaf, built for darwin, freebsd or openbsd | Active backend is `routesock`; `show interface` lists every kernel interface with index, name, flags, MTU, MAC |
| AC-2 | P1: config `backend netlink` on darwin/freebsd/openbsd | Config rejected with `unknown backend "netlink"` and the registered names, because the netlink package no longer registers there |
| AC-3 | P1: Linux config with no `backend` leaf | Active backend stays `netlink` (A-2) |
| AC-4 | P1: add then remove an IPv4 address | Address appears in `NET_RT_IFLIST` and in `show interface`, then disappears; no other address on the interface changes |
| AC-5 | P1: add then remove an IPv6 address, with and without lifetimes | Address appears with the requested valid/preferred lifetimes, then disappears |
| AC-6 | P1: admin down then up; set MTU 1400; set MAC | Each change reads back from the kernel; a refused change (MTU above the driver limit) surfaces the errno with the interface name |
| AC-7 | P1: `AddRoute` IPv4 and IPv6 with gateway and with interface only | Route present in `NET_RT_DUMP` with `RTF_PROTO1`; `ListRoutes` returns it; `RemoveRoute` removes it |
| AC-8 | P1: `AddRoute` with a non-default metric on FreeBSD or macOS | Refused with an error naming the platform and the metric (Q3); on OpenBSD the metric is `rtm_priority` and reads back |
| AC-9 | P1: a foreign route carrying `RTF_PROTO1` that Ze never installed | Ze's sweep and `fib-kernel` stale cleanup leave it in place (R-1) |
| AC-10 | P1: `RouteLookup` and `AddressIsLocal` for a destination | Answer from `RTM_GET` names the outgoing interface and gateway; a local address answers true |
| AC-11 | P1: `GetStats` | Byte and packet counters match `netstat -I` for the interface; on darwin 64-bit counters come from `NET_RT_IFLIST2` |
| AC-12 | P1: `ResetCounters` | Returns `ErrCountersNotResettable`, and the CLI says counters cannot be reset on this platform and why |
| AC-13 | P1: `LinkSpeedDuplex` on an interface with media | Speed and duplex from `SIOCGIFMEDIA`; an interface without media answers unknown, not zero |
| AC-14 | P1: create then delete a cloned loopback (`lo1` on FreeBSD/OpenBSD, `feth` on darwin) | Interface exists then is gone; `CreateDummy` on an OS without a suitable cloner returns a named error |
| AC-15 | P1: external `route add` and `ifconfig alias` while the monitor runs | Matching iface events published within one second; Ze's own changes publish no external-change event (R-3) |
| AC-16 | P1: forced monitor overrun | A full resync runs, the overrun counter increments, and the post-resync state equals the kernel state (R-2) |
| AC-17 | P1: any Backend method P1 does not implement on that OS (VLAN, bridge, tunnels, WireGuard, macvlan, XFRM, mirror, LCP) | Returns an error naming the method, the GOOS, and the spec phase that will add it; config apply rejects under exact-or-reject |
| AC-18 | P1: `fib-kernel` install, replace, delete, list on each OS | Kernel table reflects each; `listZeRoutes` returns only owned routes |
| AC-19 | P2: VLAN on FreeBSD/OpenBSD (`vlan(4)`), bridge with STP (FreeBSD `if_bridge`, OpenBSD `bridge(4)` / `veb(4)`), veth equivalents (`epair`, `pair`, `feth`) | Created, configured and deleted through the backend; VLAN QoS map accepts what the OS can express and refuses the rest by name |
| AC-20 | P2: gif, gre, vxlan tunnels | Created with local/remote endpoints and deleted; unsupported tunnel kinds on an OS are refused by name |
| AC-21 | P2: WireGuard on FreeBSD and OpenBSD (kernel, via vendored `wgfreebsd`/`wgopenbsd`) and macOS (userspace `wguser`, only if Q1 says deployment) | Device configured, peer handshake completes against a Linux WireGuard peer |
| AC-22 | P2: `ListNeighbors` IPv4 and IPv6 | Entries match `arp -an` and `ndp -an` |
| AC-23 | P2: `CreateMacvlanDevice` | Refused with a named error: no macvlan on BSD |
| AC-24 | P3: OSPFv2 on FreeBSD/OpenBSD | Adjacency reaches Full against FRR and BIRD; packets carry the receiving interface from `IP_RECVIF` |
| AC-25 | P3: single-hop BFD on BSD | Session reaches Up against FRR; a packet with TTL below 255 is dropped (RFC 5881 Section 5) |
| AC-26 | P3: IS-IS over BPF on FreeBSD/OpenBSD | Adjacency Up against FRR |
| AC-27 | P3: BGP with TCP-MD5 password on FreeBSD/OpenBSD | Ze writes the SA through PF_KEY, the session establishes against FRR/OpenBGPD with the same password and fails with a wrong one; `tcpMD5Supported()` answers true; Ze removes the SA on peer deletion |
| AC-28 | P3: BGP with TCP-MD5 on macOS | Config rejected with the existing named error (no kernel support) |
| AC-29 | P4: VRF on FreeBSD (FIBs, `net.fibs`) and OpenBSD (rtables/rdomains) | Routes installed into the VRF's table only; route lookup in that VRF answers from that table; a VRF on macOS is refused by name |
| AC-30 | P4: MPLS label routes on OpenBSD | Label push/swap/pop entries installed with `RTF_MPLS` and the `mpe(4)` interface; refused by name on FreeBSD and macOS |
| AC-31 | P4: sysctl writes on FreeBSD/OpenBSD (forwarding, and the keys the sysctl component declares) | Value written and read back; an unknown key is refused by name |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `ze` on FreeBSD with an interface address and a static route | config, iface apply, `routesock` ioctl and `RTM_ADD`, kernel | `test/bsd/iface-address-apply.ci`, `test/bsd/static-route-apply.ci` |
| 2 | Peers BGP from OpenBSD and installs learned routes | BGP, RIB, sysrib, `fib-kernel` BSD backend, kernel | interop `openbsd-fib-openbgpd` |
| 3 | Runs `ze` on a Mac for development and sees interfaces and routes | `show interface`, `show ip route` through `routesock` | `test/bsd/darwin-lo0-alias.ci` |
| 4 | Runs OSPF on FreeBSD against FRR | OSPF transport BSD raw socket, adjacency, SPF, `fib-kernel` | interop `freebsd-ospf-frr` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDefaultBackendIsRoutesockOnBSD` | `internal/component/iface/default_bsd_test.go` | AC-1, AC-3 | |
| `TestRoutesockAddAddressV4Golden` | `internal/plugins/iface/routesock/addr_test.go` | AC-4 encoding, bytes compared to a golden file per GOOS | |
| `TestRoutesockAddAddressV6Golden` | same | AC-5 encoding, `in6_aliasreq` size and offsets | |
| `TestRtMsgEncodeAddGolden` | `internal/core/routesock/encode_test.go` | `RTM_ADD`/`RTM_DELETE`/`RTM_CHANGE`/`RTM_GET` bytes per GOOS, sockaddr padding rule per OS | |
| `TestRtMsgDecodeCaptured` | `internal/core/routesock/decode_test.go` | Decode of captured messages from each OS (A-3, A-4) | |
| `FuzzRtMsgDecode` | same | Decoder never reads past `rtm_msglen` or a sockaddr length | |
| `TestRoutesockMonitorLinkEvent` | `internal/plugins/iface/routesock/monitor_test.go` | AC-15 against a fake socket | |
| `TestRoutesockMonitorOverrunResync` | same | AC-16 | |
| `TestRoutesockDropsOwnEcho` | same | R-3 | |
| `TestRoutesockUnsupportedNamesPhase` | `internal/plugins/iface/routesock/unsupported_test.go` | AC-17 error text | |
| `TestFibKernelBSDAddRouteGolden` | `internal/plugins/fib/kernel/backend_bsd_test.go` | AC-18 encoding | |
| `TestFibKernelBSDSweepSkipsForeign` | same | AC-9 | |
| `TestMetricRefusedWithoutPriority` | `internal/plugins/iface/routesock/route_test.go` | AC-8 | |
| `TestPFKeyMD5SAGolden` | `internal/core/network/pfkey_test.go` | AC-27 `SADB_ADD` bytes | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MTU | 68-65535 (IPv4 minimum; driver may lower the top) | 65535 | 67 | 65536 |
| Prefix length v4 | 0-32 | 32 | N/A | 33 |
| Prefix length v6 | 0-128 | 128 | N/A | 129 |
| OpenBSD `rtm_priority` | 1-63 (`RTP_MAX`) | 63 | 0 | 64 |
| FreeBSD FIB number | 0 to `net.fibs` minus 1 | `net.fibs` minus 1 | N/A | `net.fibs` |
| OpenBSD rtable id | 0-255 (`RT_TABLEID_MAX`) | 255 | N/A | 256 |
| VLAN id | 1-4094 | 4094 | 0 | 4095 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-address-apply` | `test/bsd/iface-address-apply.ci` | FreeBSD and OpenBSD guest: addresses v4/v6 applied and removed | |
| `static-route-apply` | `test/bsd/static-route-apply.ci` | Static route installed with `RTF_PROTO1`, removed on config delete | |
| `monitor-external-change` | `test/bsd/monitor-external-change.ci` | `ifconfig alias` by hand appears in `show interface` | |
| `darwin-lo0-alias` | `test/bsd/darwin-lo0-alias.ci` | macOS runner: alias on `lo0`, route via `lo0`, both removed | |
| `unsupported-refusal` | `test/bsd/unsupported-refusal.ci` | P1 config with a VLAN is rejected with the named error | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `freebsd-fib-bird` | `test/interop/scenarios/` (BSD guest kind) | BIRD | Routes learned over BGP are installed in the FreeBSD kernel and BIRD's `RTF_PROTO1` routes in the same table are left alone | |
| `openbsd-fib-openbgpd` | same | OpenBGPD | Same on OpenBSD, plus `rtm_priority` coexistence | |
| `freebsd-ospf-frr` | same | FRR | P3 OSPF adjacency and route install | |
| `openbsd-bfd-frr` | same | FRR | P3 single-hop BFD with TTL check | |
| `freebsd-md5-frr` | same | FRR | P3 TCP-MD5 through a Ze-written SA | |
| `openbsd-mpls-frr` | same | FRR | P4 MPLS label switching on OpenBSD | |

Each scenario owes the revert-and-red walk of `ai/rules/interop-and-goal-validation.md`.

## Files to Modify
- `internal/component/iface/default_other.go` - narrow to `!linux && !darwin && !freebsd && !openbsd`
- `internal/component/iface/yang/ze-iface-conf.yang` - drop `default "netlink"`; description and `ze:help` stop listing backends by hand
- `internal/plugins/iface/netlink/backend_other.go`, `show_other.go`, `doctor_other.go`, `register.go` - stub narrowed to the GOOS values this spec does not cover
- `internal/plugins/fib/kernel/backend_other.go`, `monitor_other.go` - narrowed likewise
- `internal/plugins/ospf/transport/backend_other.go`, `internal/plugins/isis/transport/backend_other.go`, `internal/component/bfd/transport/udp_other.go` - P3 narrows each
- `internal/core/network/md5_freebsd.go` - P3: write the SA, `tcpMD5Supported()` true
- `internal/component/sysctl/backend_other.go` - P4 narrows
- `internal/plugins/fib/kernel/mpls.go` - P4 OpenBSD MPLS entry point
- `go.mod`, `vendor/modules.txt` - vendor `golang.org/x/net/route`
- `.github/workflows/verify.yml` - macOS job (A-5)
- `docs/features/interfaces.md` - Platform rows, coverage table BSD column, "Stub (non-Linux)" section
- `docs/architecture/iface/netlink-monitor.md` - BSD monitor section, or a sibling page linked from it
- `docs/architecture/core-design.md` - FIB backend and TCP-MD5 sections
- `docs/architecture/bfd.md` - BSD receive TTL
- `docs/architecture/ospf/ospf-3-ip-transport.md` - P3: the BSD raw socket backend beside the Linux one (declared by `ospf/transport/backend_other.go`)
- `docs/architecture/mpls/mpls-kernel.md` - P4: OpenBSD `AF_MPLS` / `RTF_MPLS` / `mpe(4)` beside the Linux MPLS path (declared by `fib/kernel/mpls.go`)
- `docs/comparison.md` - platform rows

## Files to Create
- `internal/core/routesock/` - shared encoder/decoder, per-GOOS constants and struct definitions (`*_darwin.go`, `*_freebsd.go`, `*_openbsd.go`)
- `internal/plugins/iface/routesock/` - `register.go`, backend, monitor, per-OS clone and media files, `doctor.go`
- `internal/component/iface/default_darwin.go`, `default_freebsd.go`, `default_openbsd.go`
- `internal/plugins/fib/kernel/backend_bsd.go`, `monitor_bsd.go`
- `internal/core/network/md5_openbsd.go`, `pfkey_bsd.go` (P3)
- `internal/plugins/ospf/transport/backend_bsd.go`, `internal/plugins/isis/transport/backend_bsd.go`, `internal/component/bfd/transport/udp_bsd.go` (P3)
- `internal/component/sysctl/backend_freebsd.go`, `backend_openbsd.go` (P4)
- `test/bsd/*.ci` - functional tests listed above
- golden files under each package's `testdata/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-iface-conf.yang` `backend` leaf default removed; P4 VRF table id leaves come from `plan/spec-vrf.md` |
| YANG validation constraints | Yes | P4 table id ranges (Boundary Tests) |
| YANG custom validators | No | Backend name already validated by `LoadBackend` against the registry |
| CLI commands/flags | No | No new command; existing `show interface`, `show ip route` answer on BSD |
| CLI grammar (keyword before value) | N-A | No new command |
| Editor autocomplete | N-A | No new leaf with dynamic values |
| Functional test for new RPC/API | N-A | No new RPC; `test/bsd/*.ci` cover the behavior |
| Pipe completeness | N-A | No new output |
| Env var registration | No | None planned |
| Doctor check for runtime dependencies | Yes | `routesock/doctor.go`: routing socket opens, `net.fibs` (FreeBSD P4), PF_KEY socket (P3); codes in `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | Yes | `ze_iface_routesock_overruns_total`, `ze_iface_routesock_resyncs_total` (no labels) |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP family change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/` entry for interfaces (level read from `./le feature report`) gains BSD evidence per phase |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` `backend` default per OS |
| 3 | CLI command added/changed? | No | No command change |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` names the `routesock` backend |
| 6 | Has a user guide page? | No | No `docs/guide/` page covers interfaces today; the per-platform support table lives in `docs/features/interfaces.md` (row 17) |
| 7 | Wire format changed? | No | No protocol wire change; routing-socket layouts documented in the architecture page |
| 8 | Plugin SDK/protocol changed? | No | None |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | P3: `rfc/short/rfc2385.md`, `rfc/short/rfc5881.md`, `rfc/short/rfc2328.md` support rows for BSD |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (`test/bsd/`), via `plan/spec-bsd-qemu-guest-kind.md` |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` platform rows |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, `docs/architecture/iface/netlink-monitor.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md` backend list |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Run `./le spec citation anchors spec plan/spec-bsd-family-interface-backend.md` at the start of each phase; known: `docs/features/interfaces.md` anchors `backend_other.go` and `show_other.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/features/interfaces.md` stub section text |

## Implementation Steps

1. **Phase P1a: Wiring** -- vendor `x/net/route`; create `internal/core/routesock` skeleton and `internal/plugins/iface/routesock` registering `routesock`; per-OS defaults; narrow the netlink stub; remove the YANG default
   - Tests: `TestDefaultBackendIsRoutesockOnBSD`, `TestRoutesockUnsupportedNamesPhase`
   - Files: `register.go`, `default_*.go`, `backend_other.go` build lines, YANG
   - Verify: wiring tests fail on the stub, then pass
2. **Phase P1b: Encoder and listing** -- routing message encode/decode with golden files per GOOS; listing and stats via sysctl
   - Tests: `TestRtMsgEncodeAddGolden`, `TestRtMsgDecodeCaptured`, `FuzzRtMsgDecode`
   - Files: `internal/core/routesock/`
   - Verify: tests fail, implement, tests pass
3. **Phase P1c: Interface operations** -- addresses v4/v6, flags, MTU, MAC, media, clone create/destroy
   - Tests: `TestRoutesockAddAddressV4Golden`, `TestRoutesockAddAddressV6Golden`, `iface-address-apply`
   - Files: `internal/plugins/iface/routesock/`
   - Verify: tests fail, implement, tests pass
4. **Phase P1d: Routes, lookup, ownership record, fib-kernel** -- Q3 answer applied
   - Tests: `TestFibKernelBSDAddRouteGolden`, `TestFibKernelBSDSweepSkipsForeign`, `TestMetricRefusedWithoutPriority`, `static-route-apply`, interop `freebsd-fib-bird`, `openbsd-fib-openbgpd`
   - Files: `internal/plugins/fib/kernel/backend_bsd.go`, routesock route files
   - Verify: tests fail, implement, tests pass
5. **Phase P1e: Monitor** -- one long-lived reader goroutine, echo filter, overrun resync, darwin interface arrival without `RTM_IFANNOUNCE`, OpenBSD `ROUTE_MSGFILTER`
   - Tests: monitor tests, `monitor-external-change`
   - Files: routesock monitor files, `fib/kernel/monitor_bsd.go`
   - Verify: tests fail, implement, tests pass
6. **Phase P1f: macOS CI job and docs** -- `darwin-lo0-alias`; doc rows moved
   - Tests: `darwin-lo0-alias`
   - Files: `.github/workflows/verify.yml`, docs listed above
   - Verify: CI job green on a branch run
7. **Phase P2** -- VLAN, bridge+STP, veth equivalents, tunnels, WireGuard, neighbors (AC-19 to AC-23)
8. **Phase P3** -- OSPF, BFD, IS-IS transports; PF_KEY MD5 (AC-24 to AC-28), with interop scenarios
9. **Phase P4** -- FIBs/rtables VRF, OpenBSD MPLS, sysctl writes (AC-29 to AC-31), coordinated with `plan/spec-vrf.md`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, per OS where it applies |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Sweeps delete only routes in the ownership record; every refusal names method, GOOS and phase |
| Naming | Backend name `routesock` in YANG description, docs and errors alike |
| Data flow | Encoder used by both iface and `fib-kernel`; no second encoder |
| Rule: no-layering | The netlink stub no longer builds on darwin/freebsd/openbsd |
| Rule: performance | Encoding into pooled buffers, `WriteTo(buf, off) int`; no per-message allocation on the monitor path |
| Rule: goroutine-lifecycle | One monitor reader per socket, stopped by `StopMonitor` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `routesock` backend registered on the three GOOS | `GOOS=freebsd go list -deps ./cmd/ze` includes the package; `TestDefaultBackendIsRoutesockOnBSD` |
| Golden-byte encoder tests per GOOS | Package tests under `./le job run` on darwin; cross-compiled test binaries on the BSD guests |
| macOS CI job | `.github/workflows/verify.yml` job runs `test/bsd/darwin-lo0-alias.ci` |
| Docs rows moved | `grep -n 'FreeBSD / OpenBSD' docs/features/interfaces.md` shows the phase status |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Interface names bounded to `IFNAMSIZ` before copying into ioctl structs; prefix lengths and table ids range-checked before encoding |
| Kernel message parsing | Decoder bounds-checks `rtm_msglen` and every sockaddr length against the buffer; fuzz test over the decoder |
| PF_KEY secrets | MD5 keys never logged; SA removed on peer delete and on shutdown |
| Fail-open | A failed SA write leaves `tcpMD5Supported()` false for that peer and the session is not started unsigned |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- FRR and BIRD both write `RTF_PROTO1`; the bit is a convention, not an owner identity.
- darwin has no `RTM_IFANNOUNCE`; OpenBSD alone offers `ROUTE_MSGFILTER` and `SO_RERROR`.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One shared `routesock` backend for all three OS with per-GOOS constant files | One backend per OS | The message format, ioctls and sysctl MIBs are one family; FRR and BIRD each keep one BSD implementation with per-OS conditionals |
| New backend name `routesock` | Keep the name `netlink` on BSD | `netlink` names a Linux mechanism; a BSD config that says `netlink` should be refused, not silently mean something else |
| Own encoder, `x/net/route` for parsing only | `x/net/route` for both | Encoding must be buffer-first into pools and cover every `RTM_CHANGE` slot |
| `ErrCountersNotResettable` on BSD | Baseline offset to fake a reset | A faked reset reports numbers the kernel does not hold; the error already exists and the CLI names it |

## Known Limitations
- macOS gets P1 only unless Q1 says deployment.
- Windows, NetBSD and DragonFly keep the stub; no spec covers them.
- pf, PF_KEY IPsec dataplane, L2TP/PPPoE, shaping and VRRP/carp are the separate specs named in the Task.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT. Applies to P3 (RFC 2385,
RFC 5881, RFC 2328 transport code).

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/spec-bsd-family-interface-backend.md` only, in the same `./le commit create` script
