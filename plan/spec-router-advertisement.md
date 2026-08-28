# Spec: router-advertisement

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-10 |

Anchor refresh (2026-07-22 plan review, design unchanged and implementable;
citations below updated in-body): `reconcileDHCP` now `register.go`,
`DHCPStopper` `:812`, `SetDHCPClientFactory` `:824` (block `:809-826`),
shutdown stop-all `:764+`, `dhcpEntry`/`dhcpParams` `:841-856`. `BuildRA`
(`ra.go`), `startRASender`/`raSenderLoop` (`:29`/`:108`),
`Resolve`/`Subscribe` (`resolve.go`/`:80`) verified exact.

Status note: user instruction 2026-07-10 authorized conversion to ready (skeleton
filled from firsthand RESEARCH/DESIGN in this session; no separate approval round).

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/l2tp/ppp/ra.go` - the only existing RA builder (BNG, prefix-less)
4. `internal/component/l2tp/ppp/ra_linux.go` - the RA raw-socket send loop
5. `ai/rules/platform-linux.md` - RA is Linux-only; QEMU integration tests mandatory
6. `internal/component/iface/register.go` (reconcileDHCP at register.go, DHCPStopper/SetDHCPClientFactory at register.go) - the per-unit service reconcile + factory pattern this spec mirrors
7. `internal/component/iface/resolve.go` (Resolve :65, Addresses :71, Subscribe :80) - the mandatory interface-resolution surface
8. `internal/plugins/iface/dhcp/register.go` - the sibling plugin registration precedent (`iface-dhcp`)
9. `internal/component/iface/yang/ze-iface-conf.yang` (interface-unit grouping, ipv6 container at ze-iface-conf.yang) - where the new container sits

## Task

~~**Large feature area - skeleton only. Full design not started.**~~ (superseded 2026-07-10: design completed in this spec)

Ze cannot advertise IPv6 prefixes on a LAN interface. There is no radvd-equivalent
that periodically sends ICMPv6 Router Advertisements to drive SLAAC, advertise
on-link prefixes, DNS (RDNSS/DNSSL), MTU, or the managed/other-config flags. The
only RA emitter in the tree is the L2TP/PPP BNG subscriber path, and it deliberately
sends prefix-less RAs that steer the subscriber to DHCPv6; there is no prefix-option
type anywhere in the codebase.

Implement a LAN Router Advertisement sender:
- Per-interface RA config (managed/other flags, default lifetime, intervals, hop limit).
- One or more advertised prefixes with on-link/autonomous flags and lifetimes.
- ~~Optional RDNSS/DNSSL and MTU options.~~ (scoped 2026-07-10: RDNSS in scope; DNSSL and the MTU option are out of scope, see Known Limitations)
- Respond to Router Solicitations and send unsolicited RAs on a timer.

~~This is a new subsystem (most plausibly a new `internal/plugins/radvd/` plugin). It
must go through the full `/ze-spec` RESEARCH/DESIGN workflow. This skeleton tracks the
gap; it is NOT ready to implement.~~ (superseded 2026-07-10: placement decided as
`internal/plugins/iface/ra`, see Key Design Decisions D-1; RESEARCH/DESIGN done below)

### Design summary (2026-07-10)

| Aspect | Decision |
|--------|----------|
| Config surface | New `router-advertisement` container inside the per-unit `ipv6` container of the `interface-unit` grouping (`ze-iface-conf.yang`), sibling of `dhcpv6`, marked `ze:os "linux"` + `ze:backend "netlink"` |
| Sender placement | New edge plugin `internal/plugins/iface/ra` (plugin name `iface-ra`, package `ifacera`), factory-registered into the iface component exactly like `iface-dhcp` (`iface.SetDHCPClientFactory` precedent) |
| Packet encoding | New core leaf package `internal/core/ndp` (RFC 4861 RA header + Prefix Information + Source Link-Layer Address options, RFC 8106 RDNSS); `ppp.BuildRA` delegates to it with byte-identical output |
| Lifecycle | iface component reconciles desired vs active senders on config apply/reload (mirrors `reconcileDHCP`, register.go); per-sender goroutine follows `iface.Subscribe` link events; final zero-lifetime RA on removal/shutdown |
| Interface resolution | `iface.Resolve` only; the `*net.Interface` needed by multicast join is constructed from the resolver `Binding` (Ifindex/OsName), so `internal/le/ifaceresolution/ifaceresolution.go` needs no new allowlist entry |
| Tests | Unit golden-vector tests for encoding, iface config parse/reconcile unit tests, netns+veth QEMU integration tests proving a peer kernel autoconfigures, `.ci` parse accept/reject + a `needs-linux` functional test |

In scope: RA header + M/O flags, intervals, router lifetime, hop limit,
reachable/retrans timers, Prefix Information option (on-link/autonomous flags,
valid/preferred lifetimes), Source Link-Layer Address option, RDNSS option,
Router Solicitation responses, config reconcile + link up/down lifecycle,
metrics counters, doctor check.

Out of scope (recorded in Known Limitations): DNSSL, MTU option, RFC 4191 route
information/preference, unicast RAs, VPP backend, auto-derivation of prefixes
from configured unit addresses, `show interface` RA state surface.

## Required Reading

### Architecture Docs
- [ ] `internal/component/l2tp/ppp/ra_linux.go` - the existing RA raw-socket sender, RS listener, and send loop.
  → Constraint: reuse the `BuildRA` + raw-socket/RS-listener pattern, but generalise it to a LAN interface and add prefix options.
- [ ] `internal/core/sysctl/known_linux.go` - host-side RA acceptance sysctls (accept_ra, autoconf, forwarding).
  → Constraint: sending RAs is distinct from accepting them; a router sending RAs must have IPv6 forwarding on and typically does not accept RAs on the same interface.
- [ ] `ai/rules/platform-linux.md` - RA delivery is a kernel/link behaviour; QEMU integration tests are mandatory.
  → Constraint: never skip QEMU tests for "needs hardware".
- [ ] `internal/component/iface/register.go` (read 2026-07-10; re-anchored 2026-07-22) - reconcileDHCP (register.go+) + DHCPStopper/SetDHCPClientFactory (register.go) + shutdown stop-all (register.go+).
  → Decision: the RA sender copies this exact shape: RAStopper interface, SetRASenderFactory, reconcileRA on OnConfigure/OnConfigApply, stop-all on shutdown (D-4).
- [ ] `internal/plugins/iface/dhcp/register.go` (read 2026-07-10) - `iface-dhcp` registration: `registry.Registration{Dependencies: []string{"interface"}}` + factory handoff at init (register.go).
  → Decision: `iface-ra` registers identically; removing the plugin leaves the factory nil and reconcileRA a no-op (self-containment).
- [ ] `internal/component/iface/resolve.go` (read 2026-07-10) - Resolve (resolve.go), Addresses (resolve.go), Subscribe (resolve.go); Binding carries Ifindex/OsName/OperMAC.
  → Constraint: all interface-name handling goes through the resolver; the sender builds its `*net.Interface` from the Binding instead of `net.InterfaceByName` (D-3).
- [ ] `internal/le/ifaceresolution/ifaceresolution.go` (read 2026-07-10) - bans `net.InterfaceByName(` / `.LinkByName(` / SIOCGIFINDEX outside the allowlist (patterns at iface_resolution.go); `internal/component/l2tp/ppp/` is allowlisted because pppN names are kernel-assigned (iface_resolution.go).
  → Constraint: the new plugin must pass this gate with NO new allowlist entry (AC-11).
- [ ] `ai/rules/architecture.md` + `ai/rules/plugins.md` (read 2026-07-10).
  → Decision: engine nothing depends on -> edge plugin under `internal/plugins/`; nested under `internal/plugins/iface/` like `dhcp`/`netlink`/`vpp` (blank imports exist at `internal/component/plugin/all/all.go`); shared encoding primitive extracted to `internal/core/ndp` per the dedicated-feature-modules extraction rule.
- [ ] `ai/rules/config.md`, `ai/rules/config.md`, `ai/patterns/config-option.md` (read 2026-07-10).
  → Decision: operator-facing YANG config (no env vars); kebab-case full words with unit suffixes (`maximum-interval-seconds`); maximum native validation via range/default/description, cross-field checks at parse/verify time.
- [ ] `internal/plugins/iface/netlink/slaac_linux.go` (read 2026-07-10) - the receive-side complement: addrOrigin (slaac_linux.go) classifies kernel-autoconfigured addresses; ze runs no userspace RA client.
  → Constraint: receive-side tracking is untouched by this spec; the QEMU test can reuse its origin classification to assert the peer's SLAAC address.

### RFC Summaries (MUST for protocol work)
- [ ] RFC 4861 (Neighbor Discovery for IPv6): RA message and options - no short summary exists yet under `rfc/short/` (checked 2026-07-10); create it via `/ze-rfc` at implement time, before Phase 2 (todo row, deliberately not linked so no summary is fabricated).
- [ ] ~~RFC 4862 (IPv6 SLAAC) and RFC 8106 (RDNSS/DNSSL) inform the prefix and DNS options - add via `/ze-rfc` as needed.~~ (split 2026-07-10: 4862 exists, 8106 does not)
- [ ] RFC 4862 (IPv6 SLAAC): `rfc/short/rfc4862.md` EXISTS (added by the followup wave); documents the address lifecycle and the preferred <= valid lifetime rule (Section 5.5.3) this spec validates in config.
- [ ] RFC 8106 (RDNSS option): no short summary exists yet under `rfc/short/` (checked 2026-07-10); create it via `/ze-rfc` at implement time, before Phase 2 (todo row, deliberately not linked so no summary is fabricated).

**Key insights:**
- The RA message builder and raw-socket send loop already exist for BNG; the missing pieces are the LAN config surface and the Prefix Information option (RFC 4861 Section 4.6.2), which has no type in the codebase today.
- The iface component already owns a per-unit service lifecycle (DHCP clients) driven by a factory + reconcile pattern; RA senders are the same shape with the direction reversed (ze serves the LAN instead of consuming a lease).
- RFC 4861 Section 4.2 requires RAs to be sent with IPv6 Hop Limit 255; the Linux ndisc receive path discards RAs with any other hop limit. The existing ppp sender sets only `ControlMessage.IfIndex` (ra_linux.go) and never sets the multicast hop limit, whose kernel default is 1. The new sender sets it explicitly; A-4 verifies the claim in QEMU (and may reveal a latent ppp bug).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/ppp/ra.go` - `BuildRA` (ra.go) builds an ICMPv6 type-134 RA with header, flags, and RDNSS only; its comment (ra.go) states no Prefix Information is included because BNG uses DHCPv6-PD. There is no `PrefixInfo`/prefix-option type anywhere.
- [ ] `internal/component/l2tp/ppp/ra_linux.go` - ~~`raSenderLoop` (ra_linux.go)~~ `raSenderLoop` (ra_linux.go; corrected 2026-07-10 after re-read) sends RAs to `ff02::1` on a timer and on-demand to Router Solicitations, bound to the point-to-point `pppN` device; not a LAN broadcast sender.
- [ ] `internal/core/sysctl/known_linux.go` - only host-side RA behaviour is exposed (accept_ra at known_linux.go, autoconf at :69, forwarding at :77); nothing advertises prefixes.

Additional findings (2026-07-10 research pass, all read firsthand):
- `startRASender` (ra_linux.go) opens the raw socket via `net.ListenConfig` on `ip6:ipv6-icmp`, SO_BINDTODEVICE to the pppN device (ra_linux.go), joins ff02::2 (ra_linux.go), installs an ICMP6_FILTER accepting only Router Solicitations (ra_linux.go), and spawns `rsReaderLoop` (ra_linux.go) + `raSenderLoop`. Its only caller is `startIPv6Service` (ipv6_service_linux.go), invoked per PPP session. Timers are fixed (5 initial RAs at 3s, then a 600s ticker, ra_linux.go), not the RFC 4861 Section 6.2.4 randomised interval.
- The ppp send path sets only `ControlMessage.IfIndex` (ra_linux.go); it never sets the IPv6 multicast hop limit. See A-4.
- `internal/component/iface/register.go` - `reconcileDHCP` (register.go) builds a desired per-unit service set from parsed config and diffs it against running clients; `DHCPStopper` (register.go) + `SetDHCPClientFactory` (register.go) keep the component free of any import of the client plugin; shutdown stops all clients (register.go+). This is the lifecycle pattern the RA sender copies.
- `internal/component/iface/config.go` - per-unit ipv6 settings are parsed into `ipv6Settings` (config.go) with a nested `dhcpv6UnitConfig` (config.go) parsed by `parseDHCPv6Config` (config.go); parse errors propagate to OnConfigVerify (config.go comment), which is where the new cross-field RA checks reject invalid config.
- `internal/component/iface/resolve.go` - the shared resolver: `Resolve` (resolve.go) returns a `Binding` (Ifindex, OsName, OperMAC, ...), `Subscribe` (resolve.go) delivers per-logical-name link events. `internal/le/ifaceresolution/ifaceresolution.go` (patterns at :62-66) fails ./le verify current mode full for any direct `net.InterfaceByName`/`LinkByName` call outside its allowlist; `internal/component/l2tp/ppp/` is exempt only because pppN names are kernel-assigned (allowlist entry :55).
- `internal/plugins/iface/netlink/slaac_linux.go` - receive side: `addrOrigin` (slaac_linux.go) classifies kernel-autoconfigured addresses (static/slaac/temporary/dynamic) from IFA_F_* flags; ze deliberately runs no userspace RA client (file header comment). Untouched by this spec.
- `internal/component/iface/register.go` also carries receive-side RA state: `suppressRAForConfig`/`restoreAcceptRaDefrtr` (register.go) toggle `accept_ra_defrtr` for route-priority handling, and NTF_ROUTER neighbor events track discovered routers (register.go). This is about ACCEPTING RAs from other routers; it does not conflict with sending, but the reconcile code lives in the same file and the new code must not disturb it.
- `internal/component/iface/yang/ze-iface-conf.yang` - the per-unit `ipv6` container (ze-iface-conf.yang) already hosts `dhcpv6` (ze-iface-conf.yang, `ze:backend "netlink"`); `mirror` (ze-iface-conf.yang) shows the `ze:os "linux"` + `ze:backend` combination. The new `router-advertisement` container is a sibling of `dhcpv6`.
- `internal/core/probe/icmp.go` - core already hosts a low-level ICMPv6 primitive package precedent (`NetworkICMPv6` constant, icmp.go) extracted so feature modules do not depend on each other; the same rationale places the ND/RA encoder in `internal/core/ndp`.
- `internal/component/plugin/all/all.go` - blank imports for `internal/plugins/iface/dhcp` and `internal/plugins/iface/netlink` prove nested iface plugin discovery works; the generator adds `internal/plugins/iface/ra` the same way.
- QEMU precedents: `internal/component/iface/slaac_integration_linux_test.go` (integration && linux, netns + dummy, asserts origin=slaac) and `internal/plugins/iface/netlink/vlanqoslab_integration_linux_test.go` (netns + veth pair helpers). `internal/le/integration/gates.go` runs `$(ZE_QEMU_INTEGRATION_PKGS)` in the Alpine VM.

**Behavior to preserve:**
- The BNG PPP RA path is unchanged (it intentionally sends prefix-less M+O RAs); after the encoder extraction its wire output must stay byte-identical (AC-10).
- Host-side RA acceptance sysctls and the accept_ra_defrtr suppression machinery keep working as today.
- SLAAC receive-side origin tracking (slaac_linux.go) is untouched.

**Behavior to change:**
- A new per-interface RA sender advertises configured prefixes on a LAN, driven by config.

## Data Flow (MANDATORY)

### Entry Point
- Config: ~~per-interface RA settings plus one or more advertised prefixes (new config surface, most likely a `router-advert` service or an iface IPv6 sub-block).~~ (decided 2026-07-10) `interface <kind> <name> unit <unit> ipv6 router-advertisement { ... }` in `ze-iface-conf.yang`: per-unit flags, intervals, lifetimes, a `prefix` list, and an `rdnss` container (full leaf table under Key Design Decisions D-10).

### Transformation Path
1. Config parsed into per-interface RA state (flags, intervals, lifetimes, prefixes, DNS~~, MTU~~ (MTU option descoped 2026-07-10)): `parseRAConfig` fills a new `raUnitConfig` on `ipv6Settings` (config.go), parse/cross-field errors reject at OnConfigVerify.
2. An RA sender is started per configured interface via `reconcileRA` (mirrors reconcileDHCP, register.go) through the `SetRASenderFactory` seam; the sender resolves the logical name via `iface.Resolve`, joins all-routers (ff02::2) on the resolved OS device, and listens for Router Solicitations behind an ICMP6_FILTER.
3. RA messages are built by `internal/core/ndp`: header + flags + Prefix Information option(s) (new) + Source Link-Layer Address (from Binding.OperMAC) + optional RDNSS.
4. Unsolicited RAs are sent on a randomised timer (uniform in [minimum-interval, maximum-interval], RFC 4861 Section 6.2.4); solicited RAs answer Router Solicitations after a 0..500ms random delay, rate-limited to one multicast RA per 3s (RFC 4861 constants).
5. On link down (resolver Subscribe event) the sender pauses; on up/appeared it re-resolves, rejoins the group, and resumes with an initial burst.
6. On config change/removal, senders are reconfigured or stopped; a stopping sender emits a final RA with Router Lifetime 0 (RFC 4861 Section 6.2.5) before closing the socket. Shutdown stops all senders like the DHCP stop-all (register.go).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ RA sender | per-interface RA state built from config | [ ] |
| RA sender ↔ kernel | raw ICMPv6 socket sends type-134 to `ff02::1` | [ ] |
| RA options ↔ hosts | Prefix Information option drives SLAAC on receivers | [ ] |
| iface component ↔ ra plugin | factory seam (SetRASenderFactory), no direct import either way | [ ] |
| sender ↔ resolver | iface.Resolve for Binding, iface.Subscribe for link lifecycle | [ ] |

### Integration Points
- ~~New `internal/plugins/radvd/` (or an iface IPv6 RA sub-block) - config surface + sender lifecycle.~~ (decided 2026-07-10) New `internal/plugins/iface/ra` plugin (`iface-ra`) for the sender; config surface + reconcile live in the iface component (`ze-iface-conf.yang`, config.go, register.go), matching the iface-dhcp split.
- Reuse of the `BuildRA`/raw-socket/RS-listener pattern from `internal/component/l2tp/ppp/`, with the message encoding extracted to `internal/core/ndp` (ppp delegates, byte-identical).
- A new Prefix Information option type (absent today), plus Source Link-Layer Address, in `internal/core/ndp`.
- Generated composition root `internal/component/plugin/all/all.go` gains the blank import via `./le repository generate`.

### Architectural Verification
- [ ] No bypassed layers (RA sent through a raw ICMPv6 socket like the BNG path)
- [ ] No unintended coupling (RA sender self-contained in its plugin)
- [ ] No duplicated functionality (reuse the RA builder/socket pattern, extended with prefix options)
- [ ] Registration over hardcoding - the RA feature registers as a plugin; no per-feature field in a core struct.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The BNG RA builder/socket pattern generalises to a LAN sender | ra.go / ra_linux.go exist; 2026-07-10 read confirms the socket setup (ListenPacket + SO_BINDTODEVICE + JoinGroup + ICMP6_FILTER, ra_linux.go) has no ppp-specific step | need a fresh RA stack | integration test `TestRASenderPeerAutoconfigures` (netns veth) | unvalidated (design-level evidence only; settle in Phase 4) |
| A-2 | A Prefix Information option can be added cleanly to the RA builder | BuildRA is a linear offset-append encoder (ra.go); options append after the 16-byte header exactly like RDNSS (ra.go) | builder rework needed | unit test `TestBuildRAPrefixOption` golden vectors | unvalidated (design-level evidence only; settle in Phase 2) |
| A-3 | RA sending coexists with host-side accept_ra settings | sysctl model (known_linux.go); the userspace raw-socket send path is independent of accept_ra/forwarding sysctls | interface loops/conflicts | test forwarding-on + accept_ra-off in QEMU (part of `TestRASenderPeerAutoconfigures` setup) | unvalidated |
| A-4 | RAs must carry IPv6 Hop Limit 255 to be accepted by the Linux ndisc receive path, and the sender must set the multicast hop limit explicitly (kernel default is 1) | RFC 4861 Section 4.2 receiver rule; ppp sender sets only ControlMessage.IfIndex (ra_linux.go) and its subscribers may rely on different validation. UNVERIFIED against kernel source; treated as a requirement to prove | RAs silently dropped by every standard receiver | integration test asserts the peer autoconfigures AND a packet capture (raw socket read in the test) shows hop limit 255; if the ppp path is proven broken, file a followup | unvalidated |
| A-5 | The factory + reconcile seam (dhcpClientFactory shape) supports restart-on-param-change for RA senders | reconcileDHCP restarts clients whose params changed (register.go comment + dhcpEntry.params, register.go) | reconcile rework | unit test `TestReconcileRA` with a stub factory | unvalidated |
| A-6 | A `*net.Interface` constructed from the resolver Binding (Index + Name only) is sufficient for `ipv6.PacketConn.JoinGroup` and `ControlMessage.IfIndex` | JoinGroup consumes the interface index; ppp passes a full struct but uses only `iface.Index` in the send path (ra_linux.go). UNVERIFIED against x/net/ipv6 internals | fall back to post-resolution `net.InterfaceByName(binding.OsName)` plus an iface_resolution.go allowlist entry (ldp precedent, iface_resolution.go) | integration test joins the group and receives an RS through the filter | unvalidated |
| A-7 | The YANG loader accepts a new container inside the `interface-unit` grouping without touching other modules | dhcpv6/mirror/mpls containers already sit there (ze-iface-conf.yang,425,444) | schema surgery needed | `test/parse/iface-router-advertisement.ci` passes | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Misconfigured RA disrupts a LAN (wrong prefix/lifetime) | hosts get bad SLAAC addresses | validate prefix/lifetime; QEMU SLAAC test before ship |
| R-2 | RA sender and host RA-acceptance conflict on the same interface | routing loops / address churn | design guidance: forwarding on, accept_ra off on advertising interfaces; doctor check (D-6) warns when forwarding is off while advertising |
| R-3 | Extracting the encoder to `internal/core/ndp` changes ppp wire bytes and breaks BNG subscribers | parity unit test fails | ppp keeps its `RAConfig` API and delegates; `TestBuildRAParity` asserts byte-identical output for the ppp fixed config before any ppp caller changes |
| R-4 | Sender goroutine lifecycle leaks on rapid link flap or config churn | goroutine count grows in tests; stale sockets hold the group join | single owner goroutine per sender with context cancel (ppp cancel-func precedent, ra_linux.go); reconcile stops before starting a replacement; integration test flaps the link |
| R-5 | Randomised timers make tests flaky | intermittent CI failures on interval assertions | timer bounds injected via the sender config struct in tests (short intervals); assertions test bounds, not exact instants |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| ~~`set` RA on an interface with a prefix~~ | ~~→~~ | ~~RA sender emits type-134 with a Prefix Information option~~ | ~~`test/qemu/router-advertisement.ci`~~ (superseded 2026-07-10: no `test/qemu/` directory exists; needs-linux `.ci` tests live under `test/<area>/` and run in QEMU via retired `ze-qemu-needs-linux-test` (current: `./le qemu run command "./le qemu all-tests"`)) |
| config `interface veth ... unit default ipv6 router-advertisement { enabled true; prefix ... }` applied by the booted daemon | → | reconcileRA starts an `iface-ra` sender; peer veth end forms a SLAAC address | `test/plugin/iface-ra-slaac.ci` (`option=needs-linux`) |
| YANG schema accepts the new container | → | parse into `raUnitConfig` | `test/parse/iface-router-advertisement.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | RA enabled with prefix P on an interface | periodic RAs carry P with on-link/autonomous flags |
| AC-2 | a host on the link | forms a SLAAC address from P (proven in QEMU) |
| AC-3 | a Router Solicitation arrives | a solicited RA is sent promptly |
| AC-4 | RDNSS configured | RA carries the RDNSS option |
| AC-5 | RA config removed | sender stops (final zero-lifetime RA) |
| AC-6 | invalid prefix/lifetime | config verify rejects |
| AC-7 (added 2026-07-10) | `backend vpp` with a router-advertisement container | config verify rejects (netlink-only container, `ze:backend "netlink"`), mirroring `test/parse/iface-vpp-rejects-dhcp.ci` |
| AC-8 (added 2026-07-10) | any emitted RA on the wire | IPv6 Hop Limit 255, ICMPv6 type 134 code 0, Source Link-Layer Address option carrying the sender MAC (from resolver Binding.OperMAC) |
| AC-9 (added 2026-07-10) | link goes down, then up (resolver Subscribe events) | transmissions pause on down; on up the sender re-resolves, rejoins ff02::2, and resumes with an initial burst |
| AC-10 (added 2026-07-10) | ppp BuildRA after the encoder extraction | byte-identical output to the pre-extraction encoding for the BNG fixed config (parity unit test) |
| AC-11 (added 2026-07-10) | the retired `ze-iface-resolution-check` (current: `./le iface-resolution check`) over the new code | passes with zero new allowlist entries in `internal/le/ifaceresolution/ifaceresolution.go` |
| AC-12 (added 2026-07-10) | RAs sent (periodic or solicited) | `ze_iface_ra_sent_total` / `ze_iface_ra_solicited_total` counters increment |
| AC-13 (added 2026-08-01) | router lifetime configured as `0` | config verify ACCEPTS it and the sender emits RAs carrying Router Lifetime 0 (RFC 4861 section 4.2: the sender is not a default router, and the rest of the RA still applies) |
| AC-14 (added 2026-08-01) | RDNSS lifetime configured as `0` | config verify ACCEPTS it and the RDNSS option carries lifetime 0 (RFC 8106 section 5.1: the resolver address must no longer be used) |

AC-13 and AC-14 constrain AC-6: `0` is a meaningful protocol value for both
lifetimes, so it is NOT an "invalid lifetime" and the validator must not reject
it. Added from the VyOS July 2026 comparison, where T9084 fixed exactly this
defect: a CLI constraint of `1-7200` on `name-server-lifetime` banned the `0`
their own value help documented. Design the ranges as a union that admits `0`
alongside the live range, and cover `0` in the boundary tests.

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | advertises a /64 to a LAN so hosts autoconfigure | config → RA sender → Prefix Information option → host SLAAC | ~~`test/qemu/router-advertisement.ci`~~ `test/plugin/iface-ra-slaac.ci` + `TestRASenderPeerAutoconfigures` (path corrected 2026-07-10) |
| 2 (added 2026-07-10) | points LAN hosts at a DNS resolver via RDNSS without DHCPv6 | config rdnss → RA RDNSS option → host resolver list | `TestBuildRARDNSS` (encoding) + RDNSS bytes asserted in the integration capture |
| 3 (added 2026-07-10) | runs a DHCPv6-managed LAN (M/O flags, no autonomous prefix) | config managed/other-config → RA flags → hosts consult DHCPv6 | `TestBuildRAHeader` flag vectors + `test/parse/iface-router-advertisement.ci` variant |
| 4 (added 2026-07-10) | removes the RA block to retire ze as the LAN router | config reload → reconcileRA stop → final zero-lifetime RA | `TestRAFinalZeroLifetime` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| ~~`TestBuildRAPrefixOption`~~ | ~~`internal/plugins/radvd/ra_test.go`~~ (path superseded 2026-07-10, encoder moved to core) | | |
| ~~`TestRAConfigParse`~~ | ~~`internal/plugins/radvd/config_test.go`~~ (path superseded 2026-07-10, parse lives in the iface component) | | |
| `TestBuildRAHeader` | `internal/core/ndp/ra_test.go` | RFC 4861 Section 4.2 header layout golden bytes: type 134, hop limit, M/O flags, router lifetime, reachable/retrans | |
| `TestBuildRAPrefixOption` | `internal/core/ndp/ra_test.go` | Prefix Information option per RFC 4861 Section 4.6.2: type 3, length 4, prefix length, L/A flag bits, valid/preferred lifetimes, zero reserved, prefix bytes | |
| `TestBuildRASourceLinkLayer` | `internal/core/ndp/ra_test.go` | Source Link-Layer Address option per RFC 4861 Section 4.6.1: type 1, length 1, MAC bytes | |
| `TestBuildRARDNSS` | `internal/core/ndp/ra_test.go` | RDNSS option per RFC 8106 Section 5.1 (matches the existing ppp encoding) | |
| `TestBuildRAParity` | `internal/component/l2tp/ppp/ra_parity_test.go` | ppp BuildRA output byte-identical to pre-extraction bytes for the BNG fixed config (AC-10) | |
| `TestRAConfigParse` | `internal/component/iface/config_test.go` | router-advertisement container parsed into raUnitConfig with defaults | |
| `TestRAConfigCrossFieldReject` | `internal/component/iface/config_test.go` | preferred > valid, minimum > 0.75 x maximum, 0 < router-lifetime < maximum-interval all rejected at verify | |
| `TestReconcileRA` | `internal/component/iface/config_test.go` | stub factory: start on enable, stop on removal, restart on param change, no-op when factory nil (A-5) | |
| `TestRASenderMetrics` | `internal/plugins/iface/ra/sender_linux_test.go` | counters increment on send paths (AC-12) | |
| `TestRAIntervalBounds` | `internal/plugins/iface/ra/sender_linux_test.go` | computed unsolicited interval always within [minimum, maximum]; solicited delay within [0, 500ms] (R-5: bounds, not instants) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| prefix length | 0..128 | 128 | - | 129 |
| valid lifetime (s) | 0..4294967295 | 4294967295 | - | overflow rejected |
| maximum-interval-seconds | 4..1800 | 1800 | 3 | 1801 |
| minimum-interval-seconds | 3..1350 | 1350 | 2 | 1351 |
| minimum vs maximum (cross-field) | minimum <= 0.75 x maximum | 450 (at maximum 600) | - | 451 (at maximum 600) |
| router-lifetime-seconds | 0, or maximum-interval..9000 | 9000 | 1 (below maximum-interval, nonzero) | 9001 |
| hop-limit | 0..255 | 255 | - | 256 (uint8 type reject) |
| preferred vs valid lifetime (cross-field) | preferred <= valid | preferred == valid | - | preferred = valid + 1 |
| reachable-time-milliseconds | 0..3600000 | 3600000 | - | 3600001 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| ~~`router-advertisement`~~ | ~~`test/qemu/router-advertisement.ci`~~ (superseded 2026-07-10: no such directory; see Wiring Test) | | |
| `iface-router-advertisement` | `test/parse/iface-router-advertisement.ci` | full RA config (flags, intervals, prefixes, rdnss) accepted by `ze config validate` | |
| `iface-router-advertisement-invalid` | `test/parse/iface-router-advertisement-invalid.ci` | cross-field violations rejected with a clear error (AC-6) | |
| `iface-vpp-rejects-router-advertisement` | `test/parse/iface-vpp-rejects-router-advertisement.ci` | vpp backend rejects the netlink-only container (AC-7) | |
| `iface-ra-slaac` | `test/plugin/iface-ra-slaac.ci` (`option=needs-linux`) | daemon boots, applies RA config on a veth; peer veth end (accept_ra=2, autoconf=1) forms a SLAAC address from the advertised prefix; runs in QEMU via `./le qemu run command "./le qemu all-tests"` | |

QEMU Go integration tests (`//go:build integration && linux`, run by `./le qemu run command "./le qemu all-tests"`; package added to `ZE_QEMU_INTEGRATION_PKGS` in `internal/le/integration/gates.go`):
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRASenderPeerAutoconfigures` | `internal/plugins/iface/ra/ra_integration_linux_test.go` | netns + veth: sender on one end, peer end autoconfigures a global address from the advertised /64 (AC-1, AC-2); capture asserts hop limit 255 + SLLA (AC-8, A-4) | |
| `TestRASolicitedResponse` | `internal/plugins/iface/ra/ra_integration_linux_test.go` | RS sent from the peer end triggers a solicited RA within the RFC delay bounds (AC-3) | |
| `TestRAFinalZeroLifetime` | `internal/plugins/iface/ra/ra_integration_linux_test.go` | stopping the sender emits a final RA with Router Lifetime 0 (AC-5) | |
| `TestRALinkDownUp` | `internal/plugins/iface/ra/ra_integration_linux_test.go` | link flap pauses/resumes the sender via resolver Subscribe (AC-9); no goroutine leak (R-4) | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| a Linux host autoconfigures from Ze's RA | ~~-~~ `internal/plugins/iface/ra/ra_integration_linux_test.go` + `test/plugin/iface-ra-slaac.ci` (filled 2026-07-10) | Linux kernel host (QEMU netns) | RA + Prefix Information interops with a standard receiver | - |

### Future (if deferring any tests)
- Phasing: prefix advertisement + SLAAC first; ~~RDNSS/DNSSL, MTU, route information options in follow-up sub-specs.~~ (amended 2026-07-10) RDNSS is in scope now (AC-4, the ppp encoder already carries it); DNSSL, the MTU option, and RFC 4191 route information stay in follow-up sub-specs.

## Files to Modify
- ~~`internal/core/sysctl/known_linux.go` - ensure IPv6 forwarding is set on advertising interfaces (design decision)~~ (dropped 2026-07-10: D-6 decided doctor-check guidance instead of sysctl mutation; no new sysctl keys are needed, all referenced IPv6 keys already registered at known_linux.go)
- ~~iface IPv6 config surface - reference the new RA feature (design decision)~~ (made concrete 2026-07-10, rows below)
- `internal/component/iface/yang/ze-iface-conf.yang` - `router-advertisement` container inside the `interface-unit` grouping's `ipv6` container (sibling of `dhcpv6`, ze-iface-conf.yang), `ze:os "linux"` + `ze:backend "netlink"`, leaves per D-10
- `internal/component/iface/config.go` - `raUnitConfig` struct + `parseRAConfig` + `RouterAdvertisement` field on `ipv6Settings` (config.go) + cross-field verify errors
- `internal/component/iface/register.go` - `RAStopper` interface, `SetRASenderFactory`, `reconcileRA` called where `reconcileDHCP` is (OnConfigure/OnConfigApply), stop-all + final-RA on shutdown (near register.go)
- `internal/component/l2tp/ppp/ra.go` - delegate encoding to `internal/core/ndp` keeping the `RAConfig` API and byte-identical output (AC-10)
- `internal/component/plugin/all/all.go` - generated blank import of the new plugin (`./le repository generate`, never hand-edited)
- `internal/le/integration/gates.go` - add `./internal/plugins/iface/ra/...` to `ZE_QEMU_INTEGRATION_PKGS`
- `internal/core/diagnostic/codes.go` - diagnostic code for the D-6 doctor check
- `docs/features/interfaces.md` - document the per-unit `ipv6 router-advertisement` block
- `docs/features/rfc-status.md` - RFC 4861 (send side) + RFC 8106 rows with source anchors

## Files to Create
- ~~`internal/plugins/radvd/` - new plugin: config, RA sender lifecycle, prefix-option builder~~ (superseded 2026-07-10 by D-1/D-2 placement)
- ~~`internal/plugins/radvd/ra_test.go` - unit tests~~ (superseded 2026-07-10)
- ~~`test/qemu/router-advertisement.ci` - QEMU functional test~~ (superseded 2026-07-10: no `test/qemu/` directory exists)
- `internal/core/ndp/ra.go` - RFC 4861/8106 RA message + option encoder (buffer-first, pure, host-testable)
- `internal/core/ndp/ra_test.go` - golden-vector + boundary unit tests
- `internal/component/l2tp/ppp/ra_parity_test.go` - AC-10 parity test
- `internal/plugins/iface/ra/ifacera.go` - platform-independent config struct (ifacedhcp.go precedent)
- `internal/plugins/iface/ra/register.go` - `iface-ra` registration + `iface.SetRASenderFactory` handoff (linux-gated, register.go of iface-dhcp precedent)
- `internal/plugins/iface/ra/sender_linux.go` - socket setup, RS listener, randomised send loop, final zero-lifetime RA
- `internal/plugins/iface/ra/sender_linux_test.go` - interval-bounds + metrics unit tests
- `internal/plugins/iface/ra/ra_integration_linux_test.go` - netns + veth QEMU integration tests (4 tests, see TDD plan)
- `internal/plugins/iface/ra/doctor_linux.go` - D-6 doctor check + unit test (owner package per `ai/rules/plugins.md`)
- `test/parse/iface-router-advertisement.ci` - schema accept
- `test/parse/iface-router-advertisement-invalid.ci` - cross-field reject
- `test/parse/iface-vpp-rejects-router-advertisement.ci` - backend gating reject
- `test/plugin/iface-ra-slaac.ci` - `needs-linux` end-to-end functional test
- RFC short summaries for 4861 and 8106 under `rfc/short/` via `/ze-rfc` (implement time, before Phase 2)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | yes | `internal/component/iface/yang/ze-iface-conf.yang` (existing module extended; no new module registration needed) |
| YANG validation constraints | yes | every new leaf carries range/default/description (D-10 table); `prefix` list key uses `zt:prefix-ipv6`, rdnss servers `zt:ipv6-address` |
| YANG custom validators | no | native ranges cover single leaves; cross-field rules (preferred <= valid, minimum <= 0.75 x maximum, router-lifetime 0-or->=maximum) are parse/verify errors in `config.go`, the established iface pattern (e.g. tunnel invalid-combination tests in `test/parse/iface-tunnel-invalid-*.ci`) |
| CLI commands/flags | no | config-only feature; no new verbs |
| CLI grammar (action before identifier) | no | no CLI commands added |
| Editor autocomplete | yes (automatic) | derived from YANG types/enums; nothing custom |
| Functional test for new RPC/API | yes | `test/parse/iface-router-advertisement*.ci`, `test/plugin/iface-ra-slaac.ci` |
| Pipe completeness | no | no command output produced |
| Env var registration | no | operator-facing YANG config, not under `environment/` (`ai/rules/config.md` decision table: capacity/behavior knobs -> YANG) |
| Doctor check for runtime dependencies | yes | `internal/plugins/iface/ra/doctor_linux.go` + `internal/core/diagnostic/codes.go`: warn when RA is enabled on a unit whose device has `net.ipv6.conf.<dev>.forwarding=0` (D-6) |
| Prometheus counters/metrics | yes | `ze_iface_ra_sent_total{interface}`, `ze_iface_ra_solicited_total{interface}` via `internal/core/metrics` (85 plugin usages precedent); listed in docs row 14 |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | yes | `docs/features.md` (router-advertisement / SLAAC server row) |
| 2 | Config syntax changed? | yes | `docs/guide/configuration.md` (interface unit ipv6 router-advertisement block) |
| 3 | CLI command added/changed? | no | - |
| 4 | API/RPC added/changed? | no | - |
| 5 | Plugin added/changed? | yes | `docs/guide/plugins.md` (`iface-ra` entry) |
| 6 | Has a user guide page? | yes | `docs/features/interfaces.md` |
| 7 | Wire format changed? | no | BGP wire untouched; ND encoding documented via RFC anchors in `internal/core/ndp` |
| 8 | Plugin SDK/protocol changed? | no | - |
| 9 | RFC behavior implemented, changed, or newly proven? | yes | RFC 4861/8106 short summaries (via `/ze-rfc`) + `docs/features/rfc-status.md` rows |
| 10 | Test infrastructure changed? | no | existing runners; only package lists grow |
| 11 | Affects daemon comparison? | yes | `docs/comparison.md` (radvd-equivalent capability) |
| 12 | Internal architecture changed? | yes | note the `internal/core/ndp` extraction where the ppp RA design is described (`docs/research/l2tpv2-ze-integration.md` anchor) |
| 13 | Route metadata keys added/changed? | no | - |
| 14 | Prometheus counters added/changed? | yes | `docs/plugin-development/metrics.md` (two new counters) |
| 15 | Registered plugin/event/command inventory changed? | yes | `docs/plugin-overview.md`, `docs/features/plugins.md` (`iface-ra`) |
| 16 | Changed source files referenced by doc source anchors? | check at implement time | grep `docs/` for anchors on `ra.go`, `register.go`, `config.go` |
| 17 | Existing docs show config examples for this area? | yes | verify interface ipv6 examples still match the schema |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file ~~(skeleton - run `/ze-spec` RESEARCH/DESIGN first)~~ (design completed 2026-07-10) |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan - check what exists; validate cheap assumptions (A-2, A-5, A-7) |
| 3. Wiring phase | Wiring Test table - YANG container + parse + failing parse `.ci`, factory seam registered |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `./le verify-lint run && ./le test-unit  && ./le functional` (+ `./le iface-resolution check` for AC-11) |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | Executive Summary Report; two-commit closure per `ai/rules/planning.md` |

### Implementation Phases
~~1. **RESEARCH/DESIGN (not started)** - full `/ze-spec` workflow: config surface, Prefix Information option encoding, sender lifecycle, RS handling, forwarding/accept_ra interaction, QEMU SLAAC test design, phasing. Not implementable as-is.~~ (completed 2026-07-10; concrete phases below)

Each phase ends with a Self-Critical Review. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - YANG container + parse skeleton + factory seam
   - Tests: `test/parse/iface-router-advertisement.ci` (fails until schema lands), `TestRAConfigParse`, `TestReconcileRA` (fails against a stub)
   - Files: `ze-iface-conf.yang`, `config.go`, `register.go` (RAStopper/SetRASenderFactory/reconcileRA calling a nil factory), `internal/plugins/iface/ra/register.go` + `ifacera.go`, `./le repository generate` for `all.go`
   - Verify: config parses end to end; reconcile reaches the (stub) factory; parse `.ci` passes; RFC summaries for 4861/8106 created via `/ze-rfc` before Phase 2
2. **Phase: Core encoder** - `internal/core/ndp`
   - Tests: `TestBuildRAHeader`, `TestBuildRAPrefixOption`, `TestBuildRASourceLinkLayer`, `TestBuildRARDNSS` (golden vectors from the RFC layouts), boundary rows
   - Files: `internal/core/ndp/ra.go`, `ra_test.go`
   - Verify: tests fail -> implement -> pass; buffer-first API per `ai/rules/performance.md`
3. **Phase: ppp delegation (parity)** - AC-10
   - Tests: `TestBuildRAParity` (golden bytes captured from the CURRENT ppp encoder BEFORE the change)
   - Files: `internal/component/l2tp/ppp/ra.go`, `ra_parity_test.go`
   - Verify: byte-identical output; existing l2tp tests stay green
4. **Phase: Sender** - socket lifecycle, timers, RS handling
   - Tests: `TestRAIntervalBounds`, `TestRASenderMetrics`; integration tests `TestRASenderPeerAutoconfigures`, `TestRASolicitedResponse`, `TestRAFinalZeroLifetime`, `TestRALinkDownUp`
   - Files: `sender_linux.go`, `sender_linux_test.go`, `ra_integration_linux_test.go`, `internal/le/integration/gates.go`
   - Verify: the retired `ze-qemu-integration-test` (current: `./le qemu run command "./le qemu all-tests"`) green; A-1/A-3/A-4/A-6 settle here with evidence recorded in Assumptions
5. **Phase: Cross-field validation + reject tests**
   - Tests: `TestRAConfigCrossFieldReject`, `test/parse/iface-router-advertisement-invalid.ci`, `test/parse/iface-vpp-rejects-router-advertisement.ci`
   - Files: `config.go`
   - Verify: AC-6, AC-7
6. **Phase: Doctor check + metrics + functional test**
   - Tests: doctor unit test, `test/plugin/iface-ra-slaac.ci` (needs-linux, run via `./le qemu run command "./le qemu all-tests"`)
   - Files: `doctor_linux.go`, `internal/core/diagnostic/codes.go`, the `.ci`
   - Verify: AC-2 end to end in QEMU; AC-12
7. **Functional tests + RFC refs** - `// RFC 4861 Section X.Y` comments over validation, timer constants, and the zero-lifetime rule
8. **Full verification** - `./le verify current mode full`
9. **Complete spec** - audit tables, learned summary, two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-12 has implementation with file:line |
| Feature completeness | Every End-to-End User Story path works; radvd reference: flags, intervals, lifetimes, prefix flags, RDNSS all reachable from config |
| Correctness | RFC 4861 constants respected (MinRtrAdvInterval bounds, 3s multicast rate limit, 0..500ms solicited delay, hop limit 255, zero-lifetime final RA) |
| Naming | YANG kebab-case full words with unit suffixes; Go structs PascalCase of the leaves (`ai/rules/config.md`) |
| Data flow | parse in iface component only; sender receives a value config struct; no YANG knowledge inside the plugin |
| Registration over hardcoding | plugin registers via `registry.Register` + factory seam; no iface-ra spelling in any central package; removal test: nil factory -> reconcile no-op |
| Doctor checks | forwarding-off warning registered in the owner package with unit test |
| YANG validation | every new leaf has range/default/description; no bare `type string` |
| Prometheus counters | both counters registered and documented |
| Rule: iface-resolution | zero direct kernel resolution; `./le iface-resolution check` clean (AC-11) |
| Rule: buffer-first | encoder writes into caller buffer and returns length, no per-send allocations in the loop (`ai/rules/performance.md`, `ai/rules/performance.md`) |
| Rule: qemu-testing | all four integration tests present and wired into `ZE_QEMU_INTEGRATION_PKGS`; `.ci` marked `needs-linux` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `internal/core/ndp` package with RA encoder | `ls internal/core/ndp/` + `go test ./internal/core/ndp/` |
| ppp delegation with parity | `go test ./internal/component/l2tp/ppp/ -run TestBuildRAParity` |
| YANG container + parse | `grep -n "router-advertisement" internal/component/iface/yang/ze-iface-conf.yang internal/component/iface/config.go` |
| Factory seam + reconcile | `grep -n "SetRASenderFactory\|reconcileRA" internal/component/iface/register.go` |
| Plugin registered | `grep -n "iface/ra" internal/component/plugin/all/all.go` |
| Integration tests in QEMU list | `grep -n "iface/ra" internal/le/integration/gates.go` |
| Parse + functional `.ci` | `ls test/parse/iface-router-advertisement*.ci test/parse/iface-vpp-rejects-router-advertisement.ci test/plugin/iface-ra-slaac.ci` |
| RFC summaries | `ls rfc/short/ \| grep -E "4861\|8106"` (capture to tmp per bash-output rule) |
| Doctor check + diagnostic code | `grep -rn "ra" internal/core/diagnostic/codes.go` + doctor unit test run |
| Metrics | `grep -rn "ze_iface_ra" internal/plugins/iface/ra/ docs/plugin-development/metrics.md` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | RS packets are untrusted link traffic: length-check before parse; ICMP6_FILTER limits types but code must not trust it (filter set is best-effort, ppp logs and continues on failure) |
| Resource exhaustion | RS floods must not queue unbounded sends: capacity-1 coalescing channel (ppp precedent, ra_linux.go) + the 3s multicast rate limit |
| Privilege | raw ICMPv6 socket requires CAP_NET_RAW; the daemon already holds it (ping/traceroute); no new privilege surface, no setuid path |
| Config-driven amplification | router-lifetime/interval bounds enforced by YANG ranges prevent advertising storms (minimum-interval floor 3s) |
| Information leakage | RA reveals the router MAC and prefixes by design; RDNSS only advertises operator-configured addresses, never resolved system DNS |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior; RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural, back to DESIGN |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
| Kernel drops RAs in QEMU (A-4 fails) | Set hop limit via both SetMulticastHopLimit and ControlMessage; capture with the test's raw socket; if the ppp path is proven broken, record in Mistake Log + file a followup skeleton |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Skeleton assumed a top-level `internal/plugins/radvd/` plugin and a `test/qemu/` directory | placement follows the iface-dhcp nested-plugin + factory pattern; QEMU `.ci` tests live under `test/<area>/` with `option=needs-linux` | 2026-07-10 research pass (register.go, platform-linux.md) | spec paths corrected before any code |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
- The iface component already contains a receive-side RA subsystem (accept_ra_defrtr suppression, NTF_ROUTER tracking, register.go). Send and receive sides share a file but no state; keep it that way.
- The resolver Binding carries everything the raw socket needs (Ifindex, OsName, OperMAC), which is what makes the no-direct-resolution constraint (AC-11) achievable without an allowlist entry.

## Core Insight
The RA sender is not a new subsystem; it is the fourth instance of an existing
shape. The iface component already reconciles per-unit services through a
factory seam (DHCP), the tree already contains an RA raw-socket loop (ppp), and
core already hosts extracted ICMP primitives (probe). This spec composes those
three precedents and adds exactly one genuinely new artifact: the RFC 4861
option encoders.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D-1: sender in a new nested edge plugin `internal/plugins/iface/ra` (`iface-ra`, package `ifacera`), factory-registered into the iface component | (a) top-level `internal/plugins/radvd` with its own YANG module; (b) inside the `internal/plugins/iface/netlink` backend; (c) inside `internal/component/iface` itself | (a) splits interface config across two trees and duplicates the reconcile machinery; (b) the netlink backend owns kernel mutation ops, not service lifecycles (the DHCP client was deliberately split out); (c) would put a linux-only raw-socket service into the always-on component and break the delete-the-folder test. The iface-dhcp shape (register.go + internal/plugins/iface/dhcp/register.go) is the proven fit; module-tiers: an engine nothing depends on is an edge plugin |
| D-2: RA/ND message encoding in a new `internal/core/ndp` package; `ppp.BuildRA` delegates with byte-identical output | (a) import `internal/component/l2tp/ppp` from the new plugin; (b) duplicate the encoder in the plugin | (a) couples a LAN feature to the BNG domain library, and the ppp builder is deliberately prefix-less; (b) fails the spec's own no-duplication verification. `ai/rules/plugins.md` (dedicated feature modules) says shared low-level primitives extract to `internal/core/<x>`; `internal/core/probe` is the precedent |
| D-3: resolve logical names ONLY via `iface.Resolve`/`iface.Subscribe`; build the `*net.Interface` for group join from the Binding (Index/OsName); SO_BINDTODEVICE with Binding.OsName | post-resolution `net.InterfaceByName(binding.OsName)` + a new `internal/le/ifaceresolution/ifaceresolution.go` allowlist entry (ldp precedent) | keeps the ze-iface-resolution-check gate clean (AC-11) and honors os-name/mac-match selectors; fallback documented in A-6 if x/net needs a fuller struct |
| D-4: lifecycle owned by the iface component reconcile (`reconcileRA` mirroring `reconcileDHCP`), senders keyed by interface+unit, restart on param change, stop-all on shutdown, final zero-lifetime RA on stop | plugin subscribes to config events itself and manages its own set | the component already parses the interface tree and owns unit iteration; a second parser in the plugin would drift; the factory seam keeps the component import-free of the plugin |
| D-5: RFC 4861 Section 6.2 timers (uniform random interval in [minimum, maximum], initial burst of 3 at <= 16s, solicited delay 0..500ms, >= 3s between multicast RAs) | copy the ppp fixed timers (5 x 3s burst, 600s ticker) | ppp serves a single point-to-point subscriber; a LAN sender must follow the RFC constants for multi-host links; intervals injectable for tests (R-5) |
| D-6: forwarding interplay is guidance, not mutation: doctor check + apply-time log warning when `net.ipv6.conf.<dev>.forwarding=0` while advertising | (a) reject at config verify; (b) auto-set forwarding | (a) verify cannot see final kernel state (forwarding may come from a sysctl profile like `router` or global config) so rejection would be wrong; (b) silent kernel mutation outside declared config violates the sysctl model |
| D-7: set IPv6 multicast hop limit 255 explicitly on the socket AND assert it on the wire in the integration test | trust the kernel/library default like the ppp sender does | RFC 4861 Section 4.2 requires 255 and receivers discard otherwise; the kernel default multicast hop limit is 1; A-4 tracks the verification |
| D-8: include the Source Link-Layer Address option (RFC 4861 Section 4.6.1) from Binding.OperMAC | omit it like the ppp sender | on multi-access links the SLLA option saves receivers a neighbor solicitation round trip and is what radvd emits; the MAC is already in the Binding |
| D-9: observability = slog + two Prometheus counters (`ze_iface_ra_sent_total`, `ze_iface_ra_solicited_total`, label `interface`) + the D-6 doctor check | per-prefix gauges, `show interface` RA state | minimal first cut; state surfaces are follow-up scope |
| D-10: YANG shape per the leaf table below, all-native single-leaf validation, cross-field checks at parse/verify | custom `ze:validate` validators | native ranges are self-documenting and tab-complete; the iface component already rejects invalid combinations at verify (tunnel precedent) |

YANG leaf table (container `router-advertisement`, inside `unit`/`ipv6`, `ze:os "linux"`, `ze:backend "netlink"`; all names per `ai/rules/config.md`, full words + unit suffixes):

| Leaf / node | Type + native validation | Default | Meaning (RFC anchor) |
|-------------|--------------------------|---------|----------------------|
| `enabled` | boolean | false | master switch (dhcp/dhcpv6 `enabled` precedent) |
| `maximum-interval-seconds` | uint16, range 4..1800 | 600 | MaxRtrAdvInterval (RFC 4861 Section 6.2.1) |
| `minimum-interval-seconds` | uint16, range 3..1350 | 200 | MinRtrAdvInterval; cross-field <= 0.75 x maximum |
| `router-lifetime-seconds` | uint16, range 0..9000 | 1800 | AdvDefaultLifetime; 0 = not a default router; cross-field: 0 or >= maximum-interval |
| `hop-limit` | uint8, range 0..255 | 64 | AdvCurHopLimit; 0 = unspecified |
| `managed` | boolean | false | M flag (hosts use DHCPv6 for addresses) |
| `other-config` | boolean | false | O flag (hosts use DHCPv6 for other config) |
| `reachable-time-milliseconds` | uint32, range 0..3600000 | 0 | AdvReachableTime; 0 = unspecified |
| `retransmit-timer-milliseconds` | uint32 | 0 | AdvRetransTimer; 0 = unspecified |
| `prefix` (list, key `prefix`) | key type `zt:prefix-ipv6` | - | advertised Prefix Information options (RFC 4861 Section 4.6.2) |
| `prefix`/`on-link` | boolean | true | L flag |
| `prefix`/`autonomous` | boolean | true | A flag (drives SLAAC, RFC 4862) |
| `prefix`/`valid-lifetime-seconds` | uint32 | 2592000 | 30 days (radvd default); 4294967295 = infinity |
| `prefix`/`preferred-lifetime-seconds` | uint32 | 604800 | 7 days; cross-field <= valid (RFC 4862 Section 5.5.3) |
| `rdnss` (container) | - | - | RFC 8106 recursive DNS servers |
| `rdnss`/`server` | leaf-list `zt:ipv6-address`, `ze:syntax "bracket"`, max-elements 8 | - | advertised resolvers |
| `rdnss`/`lifetime-seconds` | uint32 | 0 | 0 = derive 3 x maximum-interval (RFC 8106 recommendation) |

## Known Limitations
- ~~Skeleton only: acceptance criteria and tests are provisional placeholders for DESIGN.~~ (resolved 2026-07-10: design completed)
- The Prefix Information option does not exist in the codebase yet; it is core to this feature and ~~must be designed~~ is specified in D-2/D-10 and Phase 2 (amended 2026-07-10).
- Out of scope, deliberate (2026-07-10): DNSSL and the MTU option (RFC 8106 / RFC 4861 Section 4.6.4), RFC 4191 route information and router preference, unicast RAs to individual hosts, VPP backend support (container is `ze:backend "netlink"`; a VPP RA path would use VPP's own ip6-ra feature and is a separate spec), auto-deriving advertised prefixes from configured unit addresses, `show interface` RA state, RFC 6275 mobile-IP fields.
- The ppp sender keeps its fixed timers and prefix-less policy; only its encoding is delegated. If A-4 proves the ppp hop-limit handling broken, the fix is a followup, not silent scope growth here.

## RFC Documentation

Add `// RFC NNNN Section X.Y` comments above enforcing code. MUST document:
hop limit 255 (4861 Section 4.2), option layouts (4861 Sections 4.6.1/4.6.2, 8106 Section 5.1),
timer constants and bounds (4861 Section 6.2), the zero-lifetime final RA (4861 Section 6.2.5),
the solicited-RA delay and rate limit (4861 Section 6.2.6), preferred <= valid (4862 Section 5.5.3).

## Implementation Summary
### What Was Implemented
(2026-08-03 implementation session; spec Status stays `ready` because
`internal/le/speclifecycle/session.go claim` refused the claim on the WIP cap, and
hand-editing Status to route around that check is banned.)

- `internal/core/ndp` (`ra.go`): the RFC 4861 / RFC 8106 encoder. `BuildRA`
  writes the header, the Source Link-layer Address option, any number of Prefix
  Information options, and the RDNSS option; `RALen` sizes the buffer. Nothing
  allocates, and a short buffer writes nothing and returns 0.
- `internal/component/l2tp/ppp/ra.go`: `BuildRA` now delegates to `ndp` and
  keeps its `RAConfig` API. Byte parity proven by `TestBuildRAParity`.
- `internal/component/iface/yang/ze-iface-conf.yang`: the
  `router-advertisement` container inside the per-unit `ipv6` container.
- `internal/component/iface/config_ra.go`: `raUnitConfig`, `parseRAConfig`,
  `raValidate`, and `EffectiveRDNSSLifetime`.
- `internal/component/iface/reconcile_ra.go`: `RAStopper`, `RASenderSpec`,
  `SetRASenderFactory`, `reconcileRA`, `stopAllRASenders`, and the shared
  `forEachConfiguredUnit` iterator. Called from `register.go` at both
  `reconcileDHCP` sites and on shutdown.
- `internal/plugins/iface/ra`: the `iface-ra` plugin. `ifacera.go` holds the
  RFC 4861 Section 10 timers and the two counters; `sender_linux.go` holds the
  socket, the Router Solicitation reader, the send loop, the link-flap
  handling, and the final zero-lifetime advertisements; `doctor.go` holds the
  D-6 forwarding warning.
- `internal/core/diagnostic/codes.go`: `doctor-iface-ra-forwarding`.

### Bugs Found/Fixed
- None in existing code. The ppp sender's missing multicast hop limit (A-4) is
  unchanged and still unverified; see Assumptions.

### Documentation Updates
- (delegated in the same session; see the session report)

### Deviations from Plan
| Planned | Done | Why |
|---------|------|-----|
| YANG leaves named `maximum-interval-seconds`, `router-lifetime-seconds`, ... (D-10) | `maximum-interval`, `router-lifetime`, ... with a YANG `units seconds;` statement | `ai/rules/config.md` now lists a unit suffix in the name as the anti-pattern and requires the `units` statement. The rule postdates the D-10 table. |
| `rdnss/lifetime-seconds` uint32 with `default 0`, where 0 derives 3 x maximum-interval (D-10) | `rdnss/lifetime` with NO default; unset derives 3 x maximum-interval, an explicit 0 reaches the wire | AC-14 (added 2026-08-01) requires an explicit 0 to be advertised. A default of 0 makes "unset" and "retire these resolvers" the same value, which is the VyOS T9084 defect the AC exists to prevent. |
| Add `./internal/plugins/iface/ra/...` to `ZE_QEMU_INTEGRATION_PKGS` in `internal/le/integration/gates.go` | No the native action tables under `internal/le/` edit | The variable is derived by `grep -rl '^//go:build integration && linux'`, so the build tag alone enrolls the package. |
| Parse and reconcile tests in `internal/component/iface/config_test.go` | `config_ra_test.go` and `reconcile_ra_test.go` | `config_test.go` is already past the 1000-line modularity limit, and `config.go` and `register.go` are too. The new code went into new files for the same reason. |
| Factory signature mirroring `SetDHCPClientFactory`'s twelve positional parameters | `SetRASenderFactory(func(RASenderSpec) (RAStopper, error))` | One value struct carries the advertisement, so a new leaf does not change the seam. `RASenderSpec.Equal` drives restart-on-change, because the struct holds slices and `==` is unavailable. |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
(2026-08-03. All 14 met. "proven" means a green test asserts the AC's stated behavior and a
mutation of the producing code turned that test red.)

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | proven | `ndp.BuildRA` prefix options (`TestBuildRAPrefixOption`), `ifacera.Sender.run`; QEMU `TestRASenderPeerAutoconfigures`, `TestRASenderWireFormat` | |
| AC-2 | proven | QEMU `TestRASenderPeerAutoconfigures`: a peer kernel forms a SLAAC address from the advertised /64 | |
| AC-3 | proven | `solicitedSendTime`, `solicitedDelay` (`TestRARateLimit`, `TestRASolicitedDelayBounds`); QEMU `TestRASolicitedResponse` | |
| AC-4 | proven | `ndp.writeRDNSS` (`TestBuildRARDNSS`), `raParseRDNSS` (`TestRAConfigParse`) | |
| AC-5 | proven | `Sender.sendFinal` (`TestRAFinalZeroLifetime`), `reconcileRA` (`TestReconcileRA`); QEMU `TestRAFinalZeroLifetimeOnWire` | |
| AC-6 | proven | `raValidate`, `raParsePrefixEntry` (`TestRAConfigCrossFieldReject`, `test/parse/iface-router-advertisement-invalid.ci`) | |
| AC-7 | proven | `ze:backend "netlink"` on the container (`test/parse/iface-vpp-rejects-router-advertisement.ci`) | |
| AC-8 | proven | `openRASocket` sets `SetMulticastHopLimit(255)` and `SetHopLimit(255)`; QEMU `TestRASenderWireFormat` captures hop limit 255, type 134 code 0, and the SLLA option | settles assumption A-4 |
| AC-9 | proven | `Sender.onLinkEvent`; QEMU `TestRALinkDownUp` | |
| AC-10 | proven | `ppp.BuildRA` delegating to `ndp.BuildRA` (`TestBuildRAParity`, hand-derived goldens, green before and after the extraction) | |
| AC-11 | proven | `./le iface-resolution check` OK, zero new allowlist entries; `NewSender` uses `iface.Resolve` only | |
| AC-12 | proven | `incSent`, `incSolicited` (`TestRASenderMetrics`) | |
| AC-13 | proven | `raValidate` router-lifetime branch (`TestRAConfigZeroLifetimesAccepted`, `test/parse/iface-router-advertisement.ci`) | |
| AC-14 | proven | `raParseRDNSS` keeps an explicit 0 (`TestRAConfigZeroLifetimesAccepted`, `TestRASenderConfigFromUnit`) | the leaf carries no YANG default, which is what keeps 0 distinguishable from unset |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| a standard Linux host autoconfigures from ze's RA | QEMU interop (kernel receiver) | `TestRASenderPeerAutoconfigures` output + `test/plugin/iface-ra-slaac.ci` run in `./le qemu run command "./le qemu all-tests"` |
| RA wire format is RFC 4861 conformant | unit golden vectors + on-wire capture | `internal/core/ndp/ra_test.go` vectors; integration capture asserts hop limit 255 + option bytes |
| config lifecycle is safe (apply/reload/remove) | functional + integration | `TestReconcileRA`, `TestRAFinalZeroLifetime`, `TestRALinkDownUp` |
| BNG path unharmed | parity test | `TestBuildRAParity` |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

<!-- Filled at /implement completion; see ai/rules/planning.md step 5. -->

### Files Exist (ls)
| File | Exists |
|------|--------|

### AC Verified (grep/test)
| AC ID | Evidence |
|-------|----------|

### Wiring Verified (end-to-end)
| Wiring row | Evidence |
|------------|----------|

### Assumptions Resolved
| ID | Final status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Doc row | Evidence |
|---------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] Full `/ze-spec` DESIGN completed ~~and approved~~ before implementation (design filled 2026-07-10; user instruction 2026-07-10 authorized conversion to ready)
- [ ] QEMU SLAAC test passes (`TestRASenderPeerAutoconfigures` + `test/plugin/iface-ra-slaac.ci`)
- [ ] `./le verify current mode full` passes (after implementation)
- [ ] Feature code integrated (`internal/*`)
- [ ] `./le iface-resolution check` passes with no new allowlist entries (AC-11)

### Quality Gates (SHOULD pass)
- [ ] Registration over hardcoding reviewed (RA registers as a plugin)
- [ ] Interface/IPv6 docs updated
- [ ] ppp parity test green (AC-10)
- [ ] Doctor check + metrics present (D-6, D-9)

### Design
- [ ] Key Design Decisions D-1..D-10 still hold at implement time (re-check D-3/A-6 first)
- [ ] RFC 4861 + 8106 short summaries created via `/ze-rfc` before Phase 2

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior (QEMU)

### Completion (BLOCKING - before ANY commit)
- [ ] Implementation Audit tables filled
- [ ] Goal Validation evidence recorded
- [ ] Pre-Commit Verification filled
- [ ] Learned summary written; two-commit closure per `ai/rules/planning.md`
