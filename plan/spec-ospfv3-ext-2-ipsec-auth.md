# Spec: ospfv3-ext-2 -- OSPFv3 IPsec AH/ESP Authentication (RFC 4552)

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-ospfv3-0-umbrella.md (delivered) |
| Phase | - |
| Updated | 2026-06-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `rfc/short/rfc4552.md` -- manual IPsec AH/ESP for OSPFv3: transport mode (§2), ESP-must/AH-may (§3), silent discard of unprotected/failed packets (§3/§4), interface-selected SPDs with src/dst/proto/dir/interface selectors (§6), manual keying mandatory because IKE does not fit the multicast group case (§7)
4. `rfc/short/rfc5340.md` -- OSPFv3 transport: IPv6 protocol 89, link-local source, `ff02::5`/`ff02::6` multicast (§2.9); IP version distinguishes v2/v3 for IPsec selectors (RFC 4552 §5)
5. `internal/component/ike/dataplane/dataplane.go` -- the kernel SA/SP install seam (`Dataplane` interface, `SAParams`, `SPParams`, `SADir`, `Register`/`Load`/`Get`); the reuse target for kernel IPsec plumbing
6. `internal/component/ike/dataplane/xfrm_linux.go` -- XFRM netlink backend (`InstallSA`/`InstallPolicy`/`RemoveSA`/`RemovePolicy`); transport mode (`Mode=1`) and IPv6 (`net.IP`) already supported; AH (`Proto=51`) and an upper-layer-protocol policy selector are the gaps
7. `internal/plugins/ospf/v3/transport/transport.go` -- the OSPFv3 raw IPv6 proto-89 transport orchestrator: `EnableInterface`, `OnInterfaceUp`/`OnInterfaceDown` callbacks, `SetSigner` (RFC 7166 only -- IPsec must NOT use it), `SendPacket`
8. `internal/plugins/ospf/v3/transport/backend_linux.go` -- per-interface raw IPv6 socket open (`OpenInterface`), `SO_BINDTODEVICE`, multicast join `ff02::5`/`ff02::6`; the kernel applies AH/ESP transparently once a policy+SA is installed
9. `internal/plugins/ospf/config.go` -- `ospfConfig`, `interfaceConfig`, `V6 *ospfConfig` (the `ospf { address-family ipv6 { ... } }` subtree); `interfaceConfig.Authentication` is the RFC 7166 key-chain seam (separate from IPsec)
10. `internal/plugins/ospf/register.go` -- `registerOSPF()` constructs `eng6` over `ospfv3transport.New(...)`; the v6 engine opens interfaces; the IPsec install/remove must hook the v6 engine's interface up/down, not the v2 family

## Task

Add RFC 4552 manual IPsec protection of OSPFv3 protocol packets to the OSPFv3
address family (`ospf { address-family ipv6 { ... } }`, the `eng6` instance in
`internal/plugins/ospf/`). When an operator configures a manual IPsec Security
Association on an OSPFv3 interface (SPI, AH-or-ESP protocol, algorithm, and
hex keys), Ze installs a transport-mode kernel IPsec security **policy** plus
inbound and outbound **SAs** scoped to IP protocol 89 traffic on that interface,
covering both the link-local multicast destinations (`ff02::5` AllSPFRouters,
`ff02::6` AllDRouters) and link-local unicast. The Linux kernel XFRM stack then
applies AH or ESP to every outgoing OSPFv3 packet and verifies every incoming
one transparently; OSPFv3 packet construction, the existing checksum path, and
the raw IPv6 proto-89 socket stay unchanged. Unprotected packets and packets
failing the integrity check are dropped by the kernel before they reach the
socket (RFC 4552 §3/§4).

This is the RFC 4552 authentication path. It is **distinct** from the RFC 7166
Authentication Trailer (spec-ospfv3-12-auth.md / `interfaceConfig.Authentication`
key chains): RFC 7166 is carried inside the OSPFv3 packet and signed by Ze in
the transport `SetSigner` hook; RFC 4552 is applied by the kernel below the
socket and Ze never touches the packet bytes. The two are mutually exclusive on
a given interface and the config validator must reject configuring both.

The feature reuses the existing IKE dataplane abstraction
(`internal/component/ike/dataplane`) for the actual kernel SA/policy install
rather than introducing a second XFRM client. Because that abstraction was built
for IKEv2-negotiated ESP child SAs (tunnel mode, ESP only, no upper-layer-protocol
selector), it is extended -- additively -- to also express AH (proto 51),
transport mode as a first-class policy mode, and an upper-layer-protocol selector
(89) so a policy can match only OSPFv3 traffic. The OSPFv3 plugin owns the
manual-SA config surface, the install/remove lifecycle keyed to interface
up/down, and the doctor/metrics; the dataplane owns the netlink mechanics.

### In scope (this spec)

| Item | Detail |
|------|--------|
| Manual SA config surface | Per-OSPFv3-interface IPsec block: `spi` (32-bit), `protocol` (`ah`/`esp`), `algorithm` (auth: sha1/sha256/sha384/sha512; for ESP also an encryption algorithm or `null`), `key` (hex auth key), and for ESP confidentiality an optional `encryption-key` (hex); all manual (no IKE) per RFC 4552 §7 |
| Kernel policy install | One transport-mode IPsec **security policy** per direction (out/in/fwd) matching IPv6 proto-89 traffic on the interface, installed when the OSPFv3 interface opens (RFC 4552 §2, §6) |
| Kernel SA install | One inbound and one outbound transport-mode SA (shared group keys for in/out per RFC 4552 §7), AH or ESP, with the configured SPI, algorithm, and key(s), src/dst covering link-local unicast and the `ff02::5`/`ff02::6` group destinations (§3, §4, §6) |
| Verify on receive (kernel) | Rely on the kernel XFRM inbound policy+SA to drop unprotected or failed-integrity OSPFv3 packets before delivery to the socket; surface the drop as a metric, never as a Ze-side parse (RFC 4552 §3, §4) |
| Lifecycle | Install on OSPFv3 interface up (`OnInterfaceUp`), remove on interface down (`OnInterfaceDown`) and on config removal; idempotent reconcile so a config change re-installs cleanly |
| AH and ESP, transport mode | ESP authentication mandatory, AH optional (§3); ESP confidentiality optional, ESP-only when used (§4); transport mode mandatory (§2) |
| Mutual exclusion with RFC 7166 | Config validator rejects an OSPFv3 interface that configures both IPsec (this spec) and a 7166 key chain |
| Dataplane extension | Extend `dataplane.SAParams`/`SPParams` for AH proto, transport-mode policy, and an upper-layer-protocol selector; keep the IKE ESP callers behavior-identical |
| Doctor + metrics | Doctor check for XFRM availability + manual-SA config sanity; metrics for installed SAs/policies and kernel-reported integrity drops |

### Out of scope (noted so it is not silently assumed done)

| Item | Where / why |
|------|-------------|
| IKEv2-negotiated keys for OSPFv3 | RFC 4552 §7 mandates manual keys for the multicast group case; IKE is explicitly not used here |
| RFC 7166 Authentication Trailer | spec-ospfv3-12-auth.md -- the in-packet auth path; this spec is the kernel-IPsec alternative |
| Tunnel-mode SAs / virtual-link dynamic policy (§2, §6 last para) | only transport mode on physical OSPFv3 interfaces; virtual links are a later spec |
| Automatic key rollover / key-chain lifetimes for IPsec | manual SAs are static per RFC 4552; rollover is operator-driven (config change) |
| OSPFv2 IPsec | RFC 4552 is OSPFv3-only; OSPFv2 uses RFC 2328 App D / RFC 7474 trailers |
| Confidentiality policy beyond ESP | RFC 4552 §4 forbids AH-for-confidentiality; only ESP encryption is offered |
| VPP dataplane parity | the XFRM (Linux) backend is the target; VPP IPsec for OSPFv3 is not required for this spec |

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -->
- [ ] `docs/research/ospf-implementation-guide.md` (lines ~530, ~1636, ~1665, ~1863: "OSPFv3 deprecates in-header authentication ... originally relied on IPsec AH/ESP (RFC 4552)", "RFC 4552 -- Authentication/Confidentiality for OSPFv3 (IPsec)") -- positions RFC 4552 vs RFC 7166 in Ze's OSPF roadmap
  -> Decision: RFC 4552 is the kernel-IPsec path; the guide confirms it is brittle and secondary to RFC 7166, so this spec is additive and never the default auth for base OSPFv3
  -> Constraint: OSPFv3 runs over IPv6 proto 89; the IPsec selector must match proto 89, and IP version (v6) distinguishes it from any OSPFv2 IPsec (none exists in Ze)
- [ ] `plan/spec-ospfv3-0-umbrella.md` ("Out of scope" row "RFC 4552 IPsec AH/ESP", "Authentication" row, §"Boundaries") -- the umbrella deferred RFC 4552 with the note "IPsec needs separate kernel policy work"; this spec is that work
  -> Constraint: base OSPFv3 adjacency MUST NOT require IPsec; the install path is opt-in per interface and a node with no IPsec block behaves exactly as today
  -> Decision: the config lives under the OSPFv3 address family (`ospf { address-family ipv6 { interfaces { interface { ipsec { ... } } } } }`), parsed into the `V6` sub-config, consumed by `eng6`, not the v2 family
- [ ] `ai/rules/plugin-self-containment.md` -- the OSPFv3 IPsec feature is OSPF-owned
  -> Constraint: the OSPFv3 plugin owns the config block, the install/remove lifecycle, the doctor check, and the metrics; the `dataplane` package owns only the generic netlink mechanics; no "ospf" spelling leaks into `internal/component/ike/dataplane`
- [ ] `ai/rules/config-surface.md` + `ai/rules/config-naming.md` -- YANG vs env var, kebab-case
  -> Constraint: SPI/protocol/algorithm/keys are operational YANG config (per-interface), not env vars; hex key leaves are `ze:sensitive` and never logged, mirroring `keyConfig.Secret`
- [ ] `ai/rules/module-tiers.md` -- where the install glue lives
  -> Decision: the OSPFv3 IPsec installer lives in `internal/plugins/ospf/` (plugin tier) and calls the `internal/component/ike/dataplane` seam (component tier); the dataplane stays config-agnostic and OSPF-agnostic, satisfying the dependency direction
- [ ] `ai/rules/qemu-testing.md` -- raw-socket + XFRM behavior is Linux-only
  -> Constraint: install/verify/drop behavior is exercised in QEMU integration tests (the kernel XFRM stack and proto-89 raw sockets cannot run in a unit test); interop with FRR `ospf6d` IPsec runs in the QEMU/namespace interop harness

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4552.md` -- manual IPsec AH/ESP for OSPFv3
  -> Constraint: §2 transport-mode SAs MUST be supported (tunnel mode is out of scope here)
  -> Constraint: §3 ESP authentication MUST be supported and AH MAY be supported; §4 if confidentiality is provided it MUST use ESP (never AH)
  -> Constraint: §3/§4 when auth or confidentiality is enabled, unprotected packets and packets failing the integrity/decryption check MUST be silently discarded -- in Ze this is the kernel XFRM inbound policy dropping them before the socket
  -> Constraint: §5 OSPFv2 and OSPFv3 both use protocol 89; the IP version (IPv6) plus the proto-89 selector distinguishes the OSPFv3 policy
  -> Constraint: §6 the SPD selectors MUST include source, destination, protocol, direction, and interface tagging; multiple interface-selected policies are required (one per OSPFv3 interface)
  -> Constraint: §7 IKE is not suitable for the one-to-many multicast group case, so manual keying MUST be available; inbound and outbound group traffic share SA parameters (a single shared key per interface, used for both the inbound and outbound SA)
- [ ] `rfc/short/rfc5340.md` -- OSPFv3 transport context
  -> Constraint: §2.9 OSPFv3 sends to link-local multicast `ff02::5`/`ff02::6` from a link-local source with hop limit 1; the IPsec SA/policy selectors must cover those group destinations and the link-local unicast used for DD/LSR/LSU/LSAck retransmission

**Key insights:**
- RFC 4552 is a **kernel-policy install** problem, not a packet-codec problem: once the XFRM transport-mode policy+SA for proto-89 is installed, the existing OSPFv3 transport sends/receives unchanged and the kernel does the AH/ESP work both ways.
- The reuse seam already exists (`internal/component/ike/dataplane`), but it was shaped for IKE ESP child SAs (tunnel mode, ESP only, no upper-layer selector). The minimum extension is: an AH proto value, transport mode in `SPParams.Mode`, and an `UpperProto` (89) policy selector. The IKE callers pass their current values and are unaffected.
- Manual group keying (§7) means **one shared auth key per interface used for both the inbound and outbound SA**, not a per-neighbour SA pair -- this is exactly why IKE is unusable and why the config surface is per-interface, not per-neighbour.
- RFC 4552 and RFC 7166 are mutually exclusive on an interface and use entirely separate code paths: 7166 uses the transport `SetSigner` hook (Ze signs the packet); 4552 installs kernel policy and leaves the packet bytes untouched. The validator must forbid both at once.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/component/ike/dataplane/dataplane.go` -- `Dataplane` interface (`InstallSA`/`RemoveSA`/`InstallPolicy`/`RemovePolicy`/`ListSAs`/`Close`); `SAParams{SPI,Src,Dst,IfID,Proto,Mode,ReqID,ReplayWin,EncAlgo,EncKey,AuthAlgo,AuthKey,IsAEAD,UDPEncap...}`; `SPParams{Src,Dst,Dir,Proto,Mode,IfID,ReqID}`; `SADir{In=1,Out=2,Fwd=3}`; package-global `Register`/`Load`/`Get`/`CloseBackend`
  -> Constraint: `Proto` is a `uint8` (50=ESP) -- AH is 51 and already representable; `Mode` is a `uint8` (1=transport, 2=tunnel) -- transport already representable for SAs; the gap is `SPParams` has NO upper-layer-protocol selector (so a policy cannot say "only proto 89") and the IKE callers always pass tunnel mode for policy
  -> Constraint: `Src`/`Dst` are `net.IP`/`*net.IPNet`, so IPv6 link-local + group destinations are representable without a type change
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` -- `InstallSA` builds a `netlink.XfrmState` honoring `Mode`/`Proto`/AEAD-vs-Crypt+Auth; `InstallPolicy` builds a `netlink.XfrmPolicy` with a single `XfrmPolicyTmpl{Proto,Mode,Reqid}` and `Dir`; `RemovePolicy`/`RemoveSA` by selector
  -> Constraint: `InstallSA` already supports AH-shaped state via `Crypt`+`Auth`, but AH has NO encryption -- for AH the install must set only `Auth` (the gap: today the non-AEAD branch always sets `Crypt`); `xfrmAuthName`/`xfrmAuthTruncLen` already map sha1/256/384/512
  -> Constraint: `InstallPolicy` does not set an upper-layer selector on the `XfrmPolicy` (`netlink.XfrmPolicy` has no proto field set today), so a proto-89-only match needs a new field threaded through `SPParams`
- [ ] `internal/component/ike/dataplane/register.go` -- `init()` registers `xfrm` and `vpp` backends; `internal/component/ike/engine/register.go:176` calls `dataplane.Load("xfrm")` -- the backend is loaded ONLY when the IKE engine runs
  -> Constraint: the OSPFv3 family must ensure the xfrm backend is loaded when OSPFv3 IPsec is configured even if IKE is not; the installer calls `dataplane.Load("xfrm")` (idempotent) or `dataplane.Get()` and surfaces a doctor warning when unavailable
- [ ] `internal/plugins/ospf/v3/transport/transport.go` -- `Transport.EnableInterface`/`DisableInterface`, `OnInterfaceUp(fn(ifindex,name))`/`OnInterfaceDown(fn(ifindex,name))`, `SetSigner(fn)` (RFC 7166), `SendPacket`/`Receive`; `HandleLinkUp` opens the socket then fires `onUp`; `HandleLinkDown` fires `onDown`
  -> Constraint: `OnInterfaceUp`/`OnInterfaceDown` are the exact lifecycle hooks for install/remove -- they fire with the resolved ifindex and name after the socket is open / before it closes; the installer registers these callbacks at engine construction (mirroring `installAuthHooks`)
  -> Constraint: `SetSigner` is the RFC 7166 path and MUST stay untouched by IPsec; an interface using IPsec leaves the signer unset (no in-packet trailer)
- [ ] `internal/plugins/ospf/v3/transport/backend_linux.go` -- opens `ip6:89` raw socket, `SO_BINDTODEVICE`, joins `ff02::5`/`ff02::6`, hop limit 1; `LinkLocalSource()` returns the bound link-local source used as the on-wire source
  -> Constraint: `LinkLocalSource()` gives the interface link-local that the outbound SA `Src` and inbound SA `Dst` selectors use for unicast; the group destinations `ff02::5`/`ff02::6` come from the transport's `AllSPFRouters`/`AllDRouters` constants
- [ ] `internal/plugins/ospf/config.go` -- `ospfConfig{Interfaces []interfaceConfig, V6 *ospfConfig, ...}`; `interfaceConfig{Name, AreaID, ..., Authentication authConfig}`; the v6 family is parsed from `ospf { address-family ipv6 { ... } }` into `cfg.V6`; `keyConfig.Secret` is `ze:sensitive`
  -> Constraint: the IPsec block is a NEW field on `interfaceConfig` (e.g. `IPsec *ipsecInterfaceConfig`) populated only under the v6 family; the parser must reject an IPsec block under the IPv4 family (RFC 4552 is OSPFv3-only) and reject both IPsec and `Authentication` (7166) on one interface
- [ ] `internal/plugins/ospf/register.go` -- `registerOSPF()` builds `eng6 = newEngineWithCodec(ospfv3transport.New(ospfv3transport.NewBackend()), v6Codec{})`; `eng6.setConfig(*cfg.V6)`; `openInterfaces`; `eng6.reconcile`
  -> Constraint: the IPsec installer is constructed and its `OnInterfaceUp`/`OnInterfaceDown` callbacks registered against the `eng6` transport only; it reads the per-interface IPsec config from `cfg.V6`
- [ ] `internal/plugins/ospf/auth_wiring.go` -- `installAuthHooks` wires `transport.SetSigner(e.signPacket)` and `dispatch.authOK = e.verifyPacket` (the RFC 7166 / OSPFv2 trailer path)
  -> Constraint: this is the precedent for "engine glue that hooks the transport"; the IPsec installer is a sibling (`installIPsecHooks`) but hooks `OnInterfaceUp`/`OnInterfaceDown` instead of the signer, and does NOT set `dispatch.authOK` (the kernel verifies, not Ze)
- [ ] `internal/plugins/ospf/v3/transport/doctor.go` + `doctor_linux.go` -- `checkOSPFv3RawSocket` warns when OSPFv3 is configured but a raw IPv6 proto-89 socket cannot open; `diagnostic` codes `doctor-ospfv3-raw-socket` registered in `internal/core/diagnostic/codes.go`
  -> Constraint: the new IPsec doctor check follows this exact shape -- a no-op unless OSPFv3 IPsec is configured, warning when XFRM/`dataplane` is unavailable; register a new diagnostic code alongside `doctor-ospfv3-raw-socket`

**Behavior to preserve:**
- The OSPFv3 raw IPv6 proto-89 transport, checksum finalization, multicast joins, and packet codecs -- IPsec changes none of them (the kernel operates below the socket).
- The RFC 7166 `SetSigner` path and `interfaceConfig.Authentication` key chains -- untouched and still usable on interfaces that do not use IPsec.
- All existing IKE dataplane callers (`engine/established.go`, `engine/reconcile.go`) -- the `SAParams`/`SPParams` extension is additive with zero-value defaults that reproduce today's tunnel-mode ESP behavior.
- All existing OSPFv3 functional/interop tests -- a node without an IPsec block is byte-for-byte unchanged on the wire.

**Behavior to change:** (all RFC-4552-required or enabling, not discretionary)
- `dataplane.SPParams`: add a `UpperProto uint8` selector (0 = any, today's behavior; 89 = OSPFv3) and make transport mode a valid `Mode`; `xfrm_linux.go` threads `UpperProto` onto the `XfrmPolicy` and supports a transport-mode policy template.
- `dataplane.InstallSA`/`SAParams`: support AH (`Proto=51`) by installing only an `Auth` algo (no `Crypt`) when the SA is AH; the encryption fields stay empty for AH.
- `interfaceConfig`: add the IPsec block (v6-only); `config.go` parses and validates it (reject under v4, reject alongside 7166).
- OSPFv3 engine: on `OnInterfaceUp` install the proto-89 transport-mode policy + in/out SAs; on `OnInterfaceDown` remove them; reconcile on config change.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Config:** `ospf { address-family ipv6 { interfaces { interface <name> { ipsec { spi ...; protocol esp; algorithm sha256; key <hex>; ... } } } } }` -> parsed into `cfg.V6.Interfaces[i].IPsec`.
- **Lifecycle:** the `eng6` transport opens the interface socket -> fires `OnInterfaceUp(ifindex, name)` -> the IPsec installer installs the kernel policy+SAs. Interface down / config removal -> `OnInterfaceDown` / reconcile -> removal.
- **Data plane (runtime):** outgoing OSPFv3 packet -> kernel XFRM outbound policy matches proto 89 on the interface -> kernel applies AH/ESP -> wire. Incoming -> kernel XFRM inbound policy requires AH/ESP -> verify/decrypt -> deliver to the raw socket (or silently drop on failure).

### Transformation Path
1. **Config parse (new):** `config.go` reads the `ipsec` container under each v6 interface into `ipsecInterfaceConfig{SPI, Protocol(ah/esp), AuthAlgo, AuthKey(hex->bytes), EncAlgo, EncKey(hex->bytes)}`; validates ranges, hex length vs algorithm, ESP-only-confidentiality, and the 7166 mutual-exclusion.
2. **Selector build (new):** for the interface, derive the XFRM selectors -- src = interface link-local (`transport.LinkLocalSource`), dsts = `ff02::5`, `ff02::6`, and link-local unicast (`fe80::/10`), proto = 89, direction = out/in/fwd, ifindex tag.
3. **Translate to dataplane params (new):** map `ipsecInterfaceConfig` -> `dataplane.SAParams` (Proto 50/51, Mode transport, the SPI, AuthAlgo/AuthKey, and for ESP EncAlgo/EncKey) and `dataplane.SPParams` (UpperProto 89, Mode transport, Dir, selectors, ifindex). One shared key drives both the inbound SA (Dst = our addrs) and the outbound SA (Src = our addrs) per §7.
4. **Kernel install (reused + extended):** `dataplane.Get().InstallPolicy(...)` (out/in/fwd) and `InstallSA(...)` (in + out) via the XFRM backend; the IKE-shaped backend gains AH and transport-policy + UpperProto support.
5. **Runtime (kernel, no Ze code):** the kernel applies/verifies AH/ESP on every proto-89 datagram on the interface; failures are dropped below the socket. Ze observes only that protected adjacencies form and reads XFRM error counters for the drop metric.
6. **Removal (new):** on interface down / config change, `RemovePolicy` + `RemoveSA` for the interface's SPIs; reconcile re-installs on a changed SPI/key/algorithm.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| OSPFv3 config <-> installer | `cfg.V6.Interfaces[i].IPsec` read by the installer at `OnInterfaceUp` | [ ] |
| Installer <-> dataplane | `dataplane.SAParams`/`SPParams` (value types, no cross-boundary pointers) via `InstallSA`/`InstallPolicy` | [ ] |
| Dataplane <-> kernel | `vishvananda/netlink` XFRM state/policy add/del (Linux) | [ ] |
| Transport <-> installer | `OnInterfaceUp`/`OnInterfaceDown` callbacks fire with ifindex+name; `LinkLocalSource()` supplies the unicast selector | [ ] |
| Kernel <-> raw socket | the kernel applies/verifies AH/ESP transparently; Ze's `SendPacket`/`Receive` see plaintext OSPFv3 | [ ] |
| Installer <-> doctor/metrics | XFRM availability + manual-SA sanity check; installed-SA gauge + XFRM drop counter | [ ] |

### Integration Points
- `internal/component/ike/dataplane` -- the SA/policy install seam (extended additively for AH, transport-mode policy, UpperProto); `Register`/`Load`/`Get`.
- `internal/plugins/ospf/v3/transport` -- `OnInterfaceUp`/`OnInterfaceDown` lifecycle, `LinkLocalSource`, the `AllSPFRouters`/`AllDRouters` constants.
- `internal/plugins/ospf` (`eng6`) -- the installer constructed and hooked in `registerOSPF`/engine construction; reads `cfg.V6`.
- `internal/plugins/ospf/config.go` + `yang/ze-ospf-conf.yang` -- the per-v6-interface `ipsec` block, parse + validate.
- `internal/core/diagnostic/codes.go` -- the new IPsec doctor diagnostic code.

### Architectural Verification
- [ ] No bypassed layers (config -> installer -> dataplane -> kernel; the data plane is the kernel, not a Ze fast path)
- [ ] No unintended coupling (the dataplane names no OSPF symbol; the installer depends on the dataplane, not vice-versa)
- [ ] No duplicated functionality (reuses the IKE XFRM client instead of a second netlink path; reuses the OSPFv3 transport lifecycle instead of a new socket)
- [ ] Zero-copy preserved where applicable (N/A on the packet path -- the kernel owns AH/ESP; key bytes are decoded once at config time)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The `internal/component/ike/dataplane` XFRM backend can install a transport-mode IPv6 SA + policy for proto-89 traffic with only additive `SAParams`/`SPParams` fields | `dataplane.go` (`Proto`/`Mode` already uint8, `Src`/`Dst` net.IP/IPNet), `xfrm_linux.go` (`Mode`/`Proto` honored) | a separate XFRM client or a larger rework of the dataplane is needed | `TestXFRMTransportPolicyUpperProto`, `TestXFRMInstallAHSA` (QEMU) | unvalidated |
| A-2 | `OnInterfaceUp`/`OnInterfaceDown` fire with a resolved ifindex+name at exactly the right time to install/remove the policy (socket open, link-local available) | `transport.go` `HandleLinkUp` fires `onUp` after open; `HandleLinkDown` fires `onDown` before/at close | install races the socket; packets sent before the policy exists are unprotected and dropped by the peer | `TestIPsecInstallOnInterfaceUp` (unit, fake dataplane) + QEMU adjacency | unvalidated |
| A-3 | A single shared manual key per interface, used for both the inbound and outbound SA, interoperates with FRR `ospf6d` IPsec (RFC 4552 §7 group keying) | RFC 4552 §7; FRR ospf6d IPsec model | per-neighbour SAs are required and the multicast case does not work | `ospfv3-ipsec-frr` interop (QEMU) | unvalidated |
| A-4 | The kernel XFRM inbound policy silently drops unprotected / failed-integrity proto-89 packets before the raw socket sees them, satisfying §3/§4 without Ze-side verify | RFC 4552 §3/§4; Linux XFRM `level required` semantics | Ze must verify in-band (it cannot -- AH/ESP is stripped by the kernel); the silent-discard requirement is unmet | `TestIPsecUnprotectedDropped` (QEMU: send plain proto-89, assert no adjacency + XFRM drop counter) | unvalidated |
| A-5 | The OSPFv3 packet path (checksum, multicast, link-local source) is unchanged by IPsec because the kernel operates below the socket | `backend_linux.go`, `transport.go` `SendPacket` | IPsec corrupts the checksum or the source binding | existing OSPFv3 functional tests still green with IPsec off; `ospfv3-ipsec-frr` adjacency reaches Full | unvalidated |
| A-6 | AH (proto 51) over the XFRM backend needs only an `Auth` algo and no `Crypt`/encryption | `xfrm_linux.go` non-AEAD branch sets both `Crypt`+`Auth`; AH has no encryption | the AH SA install fails or installs a malformed state | `TestXFRMInstallAHSA` (QEMU), `TestSAParamsAHNoEncryption` (unit) | unvalidated |
| A-7 | RFC 4552 and RFC 7166 are mutually exclusive and the validator can reject both on one interface at parse time | `config.go` parse order; `interfaceConfig.Authentication` and the new `IPsec` are sibling fields | a confusing double-auth config is accepted and one path silently wins | `TestIPsecAnd7166MutuallyExclusive` (unit) | unvalidated |
| A-8 | The xfrm dataplane backend is `Load`-able from the OSPFv3 family even when IKE is not configured (idempotent `Load`/`Get`) | `dataplane.Register` in `init()`; `Load("xfrm")` only called by IKE today | OSPFv3 IPsec silently no-ops because no backend is active | `TestIPsecLoadsXFRMBackend` (unit) + doctor check | unvalidated |
| A-9 | The IPsec block belongs only under the v6 family; placing it under v4 is a config error (no OSPFv2 IPsec) | RFC 4552 OSPFv3-only; `config.go` v4/v6 split | an OSPFv2 interface silently accepts an inert IPsec block | `TestIPsecRejectedUnderV4` (unit) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The policy is installed after the first Hello is sent, so the first packets are unprotected and dropped by the peer, delaying adjacency | adjacency forms slowly; peer logs auth-failed Hellos at startup | install the policy+SA on `OnInterfaceUp` BEFORE `EnableInterface`-driven Hello transmission; the engine sends its first Hello only after `onUp` returns; `TestIPsecInstallBeforeFirstHello` |
| R-2 | Shared-key (§7) modeling wrong -- two SAs with different in/out keys -- breaks multicast where every router uses the same group key | FRR rejects Ze's multicast OSPFv3; one-way adjacency | model one key per interface feeding both SAs; pin against FRR in `ospfv3-ipsec-frr`; `TestSAParamsSharedKey` |
| R-2b | SPI collision: the same SPI on inbound and outbound is required for the shared group SA, or the kernel rejects a duplicate | XFRM `EEXIST` on the second `InstallSA` | follow RFC 4552 §7: in and out share SA parameters but the kernel keys SAs by (dst, proto, SPI); use the configured SPI for both directions with the correct dst selector; `TestXFRMSharedSPIInOut` (QEMU) |
| R-3 | Extending `SAParams`/`SPParams` changes IKE ESP behavior (regression in the IPsec component) | IKE site-to-site SAs stop installing | additive fields with zero-value = today's behavior; run the existing IKE dataplane tests; `TestDataplaneIKEUnaffected` |
| R-4 | Key material logged or surfaced in `show`/diagnostics | a hex key appears in a log line or `show ipv6 ospf interface` | `ze:sensitive` on the YANG key leaves; the installer never logs key bytes; doctor reports presence not value; security-review check |
| R-5 | The kernel drops protected OSPFv3 packets due to a checksum/source mismatch under IPsec (the SA src selector must equal the on-wire link-local source) | adjacency never forms with IPsec on, forms with it off | derive the SA selector src from `transport.LinkLocalSource()` (the exact on-wire source); `TestIPsecSelectorMatchesLinkLocal` + QEMU |
| R-6 | Removal leaves stale SAs/policies after interface flap or config change, blocking re-adjacency | adjacency fails after a config change; `ip xfrm state` shows orphan SAs | reconcile diff installed-vs-desired; `RemoveSA`/`RemovePolicy` on down and on changed SPI; `TestIPsecReconcileReplacesSA` |
| R-7 | XFRM unavailable (no CAP_NET_ADMIN, no kernel IPsec) yields a silent no-op and an unprotected adjacency the operator believes is protected | OSPFv3 forms an adjacency despite IPsec configured but no SA in `ip xfrm` | doctor check warns; the installer logs an error and (configurable) refuses to bring the interface up when IPsec is required but uninstallable; `TestIPsecDoctorWarnsWhenXFRMUnavailable` |
| R-8 | AH transport mode breaks because AH authenticates immutable IPv6 header fields and the hop-limit-1 / source binding interacts badly | AH adjacency fails where ESP works | default and primary path is ESP (RFC 4552 §3 ESP must, AH may); AH is exercised separately in `ospfv3-ipsec-ah-frr`; document AH as best-effort if interop is fragile |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ospf { address-family ipv6 { interface X { ipsec { ... } } } }` config | -> | `config.go` parses `interfaceConfig.IPsec`; validator accepts a well-formed block | `TestParseOSPFv3IPsecConfig` (unit) + `test/ospfv3/ospfv3-ipsec-config.ci` |
| OSPFv3 interface opens (`eng6` `OnInterfaceUp`) | -> | `installIPsecHooks` -> installer builds selectors -> `dataplane.InstallPolicy` + `InstallSA` (in/out) | `TestIPsecInstallOnInterfaceUp` (unit, fake dataplane) + `test/ospfv3/ospfv3-ipsec-install.ci` |
| OSPFv3 interface closes / IPsec removed | -> | `OnInterfaceDown` / reconcile -> `RemovePolicy` + `RemoveSA` | `TestIPsecRemoveOnInterfaceDown` (unit) |
| `dataplane.InstallPolicy` with `UpperProto=89`, transport mode | -> | XFRM backend installs a proto-89 transport-mode policy | `TestXFRMTransportPolicyUpperProto` (QEMU) |
| `dataplane.InstallSA` with `Proto=51` (AH) | -> | XFRM backend installs an AH SA (Auth only, no Crypt) | `TestXFRMInstallAHSA` (QEMU) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A v6 interface with `ipsec { protocol esp; spi 256; algorithm sha256; key <32B-hex> }` | config parses into `interfaceConfig.IPsec`; on interface up a transport-mode ESP policy (proto 89) + inbound and outbound SAs with SPI 256 and the SHA-256 auth key are installed in the kernel |
| AC-2 | The same with `protocol ah` | an AH (proto 51) transport-mode SA is installed with the auth algo/key and NO encryption; policy template proto is AH (§3) |
| AC-3 | ESP with an `encryption esp; encryption-algorithm aes256; encryption-key <hex>` plus auth | the ESP SA carries both encryption and integrity (confidentiality via ESP only, §4) |
| AC-4 | `protocol ah` together with an `encryption-key` | config rejected: confidentiality MUST use ESP, never AH (§4) |
| AC-5 | An IPsec block on an OSPFv2 (IPv4) interface | config rejected: RFC 4552 is OSPFv3-only (§5, v6 family only) |
| AC-6 | An interface configuring both `ipsec` and a 7166 `authentication { key-chain }` | config rejected: the two auth paths are mutually exclusive |
| AC-7 | OSPFv3 interface up with IPsec configured | the kernel policy+SAs exist before the first Hello is transmitted; the adjacency forms protected (R-1) |
| AC-8 | A plain (unprotected) proto-89 packet arrives on an IPsec-enabled interface | the kernel inbound policy silently discards it; no adjacency from an unprotected peer; the XFRM drop metric increments (§3/§4) |
| AC-9 | A protected OSPFv3 packet with the wrong key arrives | the kernel integrity check fails and the packet is dropped before the socket; the drop metric increments (§3/§4) |
| AC-10 | The IPsec block is removed or the interface goes down | the installed policy + in/out SAs are removed; no orphan XFRM state remains |
| AC-11 | The SPI or key is changed in config | the installer removes the old SA/policy and installs the new one (idempotent reconcile); the adjacency re-forms |
| AC-12 | XFRM/dataplane is unavailable (no CAP_NET_ADMIN) but IPsec is configured | doctor reports a warning with code `doctor-ospfv3-ipsec`; the installer logs an error and does not silently pretend the interface is protected |
| AC-13 | Two Ze routers (or Ze + FRR `ospf6d`) on a link with matching manual SAs | adjacency reaches Full; OSPFv3 packets on the wire carry AH or ESP; `ip xfrm state`/`policy` show the installed transport-mode entries |
| AC-14 | An IKE site-to-site ESP tunnel configured alongside (unrelated) | IKE ESP SAs install exactly as before the dataplane extension (no regression) |
| AC-15 | Operator runs `show ipv6 ospf interface` and the IPsec doctor | the interface shows IPsec enabled with protocol/SPI (key value never shown); doctor confirms SAs installed |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures manual ESP IPsec on an OSPFv3 interface and brings it up | config -> `cfg.V6.Interfaces[i].IPsec` -> `OnInterfaceUp` -> installer -> `dataplane.InstallPolicy`+`InstallSA` -> kernel applies ESP -> Full adjacency | `test/ospfv3/ospfv3-ipsec-install.ci` + `ospfv3-ipsec-frr` interop |
| 2 | Peers Ze with FRR `ospf6d` using a matching manual AH SA | both install transport-mode AH for proto 89; kernel authenticates Hellos/DD/LSU both ways; adjacency reaches Full | `ospfv3-ipsec-ah-frr` interop (QEMU) |
| 3 | An attacker injects an unprotected OSPFv3 Hello on the link | kernel inbound policy drops it; no adjacency; XFRM drop metric increments | `test/ospfv3/ospfv3-ipsec-drop.ci` (QEMU: inject plain proto-89) |
| 4 | Rotates the IPsec key by editing config and committing | reconcile removes the old SA/policy and installs the new; adjacency re-forms | `test/ospfv3/ospfv3-ipsec-rekey.ci` |
| 5 | Runs `ze doctor` on a node with IPsec configured but no CAP_NET_ADMIN | doctor returns `doctor-ospfv3-ipsec` warning; the interface does not falsely claim protection | `test/ospfv3/ospfv3-ipsec-doctor.ci` |
| 6 | Removes the IPsec block | `OnInterfaceDown`/reconcile removes the kernel state; `ip xfrm` is clean; adjacency falls back to unprotected only if 7166 or nothing is configured | `test/ospfv3/ospfv3-ipsec-remove.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseOSPFv3IPsecConfig` | `internal/plugins/ospf/config_ipsec_test.go` | AC-1: SPI/protocol/algorithm/keys parse into `interfaceConfig.IPsec`; hex decode | |
| `TestIPsecRejectedUnderV4` | `internal/plugins/ospf/config_ipsec_test.go` | AC-5, A-9: IPsec block under the v4 family is a config error | |
| `TestIPsecAnd7166MutuallyExclusive` | `internal/plugins/ospf/config_ipsec_test.go` | AC-6, A-7: both auth paths on one interface rejected | |
| `TestIPsecAHWithEncryptionRejected` | `internal/plugins/ospf/config_ipsec_test.go` | AC-4: AH + confidentiality rejected (§4) | |
| `TestIPsecKeyLengthValidation` | `internal/plugins/ospf/config_ipsec_test.go` | hex key length must match the algorithm (sha256=32B etc.) | |
| `TestIPsecInstallOnInterfaceUp` | `internal/plugins/ospf/ipsec_install_test.go` | wiring: `OnInterfaceUp` -> `InstallPolicy`+two `InstallSA` (fake dataplane records calls) | |
| `TestIPsecInstallBeforeFirstHello` | `internal/plugins/ospf/ipsec_install_test.go` | R-1, AC-7: install completes before the engine sends the first Hello | |
| `TestIPsecRemoveOnInterfaceDown` | `internal/plugins/ospf/ipsec_install_test.go` | AC-10: down -> `RemovePolicy`+`RemoveSA` | |
| `TestIPsecReconcileReplacesSA` | `internal/plugins/ospf/ipsec_install_test.go` | AC-11, R-6: changed SPI/key removes old + installs new | |
| `TestIPsecSelectorMatchesLinkLocal` | `internal/plugins/ospf/ipsec_install_test.go` | R-5: SA src selector == `transport.LinkLocalSource()`; dsts include ff02::5/6 + fe80::/10 | |
| `TestSAParamsSharedKey` | `internal/plugins/ospf/ipsec_install_test.go` | R-2, A-3: one configured key feeds both in and out SAs (§7) | |
| `TestIPsecLoadsXFRMBackend` | `internal/plugins/ospf/ipsec_install_test.go` | A-8: installer Loads/Gets the xfrm backend when IKE is absent | |
| `TestSAParamsAHNoEncryption` | `internal/component/ike/dataplane/dataplane_test.go` | AC-2, A-6: AH `SAParams` carries Auth only, no Enc | |
| `TestSPParamsUpperProtoSelector` | `internal/component/ike/dataplane/dataplane_test.go` | the new `UpperProto` field defaults to 0 (any) and accepts 89 | |
| `TestDataplaneIKEUnaffected` | `internal/component/ike/dataplane/dataplane_test.go` | R-3, AC-14: zero-value new fields reproduce today's tunnel-mode ESP params | |
| `TestIPsecDoctorWarnsWhenXFRMUnavailable` | `internal/plugins/ospf/doctor_ipsec_test.go` | AC-12, R-7: doctor returns `doctor-ospfv3-ipsec` when dataplane unavailable + IPsec configured | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| SPI | 256-4294967295 (RFC 4303 §2.1 reserves 0-255) | 4294967295 | 255 (reserved) | N/A (uint32 max) |
| Protocol selector (IP proto) | {50 ESP, 51 AH} | 51 | N/A | a value other than 50/51 rejected |
| `UpperProto` policy selector | {0 any, 89 OSPF} | 89 | N/A | N/A (uint8) |
| SA Mode | {1 transport, 2 tunnel} | 1 (this spec) | N/A | tunnel not offered by the OSPFv3 surface |
| Auth key length (hex bytes) | sha1=20, sha256=32, sha384=48, sha512=64 | per-algo | shorter than algo | longer than algo |
| Encryption key length (hex bytes, ESP) | aes128=16, aes256=32 | per-algo | shorter | longer |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospfv3-ipsec-config` | `test/ospfv3/ospfv3-ipsec-config.ci` | a well-formed IPsec block parses; bad blocks (AH+enc, under-v4, +7166) are rejected with clear errors | |
| `ospfv3-ipsec-install` | `test/ospfv3/ospfv3-ipsec-install.ci` | bringing up an IPsec interface installs the kernel policy+SAs; `show ipv6 ospf interface` shows IPsec on | |
| `ospfv3-ipsec-drop` | `test/ospfv3/ospfv3-ipsec-drop.ci` | an injected unprotected proto-89 packet is dropped; the XFRM drop metric increments (QEMU) | |
| `ospfv3-ipsec-rekey` | `test/ospfv3/ospfv3-ipsec-rekey.ci` | changing SPI/key reconciles cleanly; adjacency re-forms | |
| `ospfv3-ipsec-remove` | `test/ospfv3/ospfv3-ipsec-remove.ci` | removing IPsec clears all kernel state | |
| `ospfv3-ipsec-doctor` | `test/ospfv3/ospfv3-ipsec-doctor.ci` | doctor warns when XFRM is unavailable and IPsec is configured | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospfv3-ipsec-frr` | `test/interop/scenarios/ospfv3-ipsec-frr/` | FRR `ospf6d` with matching manual ESP SA | Ze + FRR form a Full OSPFv3 adjacency with ESP transport-mode protection over proto-89 multicast; the wire carries ESP; an unprotected packet is dropped | |
| `ospfv3-ipsec-ah-frr` | `test/interop/scenarios/ospfv3-ipsec-ah-frr/` | FRR `ospf6d` with matching manual AH SA | Ze + FRR form a Full adjacency with AH transport-mode protection (§3 AH-may); validates the AH install path | |

> Interop is required: this changes the protected/dropped behavior of OSPFv3
> packets on the wire. The XFRM policy/SA install and proto-89 raw sockets are
> Linux-only and run as QEMU integration tests (`ai/rules/qemu-testing.md`),
> consistent with the rest of the OSPFv3 transport/interop set.

### Future (if deferring any tests)
- None. Every AC maps to a unit, functional, or interop test above. Tunnel mode, virtual-link IPsec, and VPP IPsec are out of scope (not deferred tests of this feature).

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*) -->
- `internal/component/ike/dataplane/dataplane.go` -- add `UpperProto uint8` to `SPParams`; document transport mode as a first-class `Mode`; clarify `Proto` 51 (AH) on `SAParams`; no change to the `Dataplane` interface signatures
- `internal/component/ike/dataplane/xfrm_linux.go` -- thread `UpperProto` onto the `netlink.XfrmPolicy`; support a transport-mode policy template; AH install path (set only `Auth`, no `Crypt`, when the SA is AH)
- `internal/plugins/ospf/config.go` -- add `IPsec *ipsecInterfaceConfig` to `interfaceConfig`; parse the v6 `ipsec` block; validate (ranges, hex-vs-algo, ESP-only confidentiality, v6-only, 7166 mutual-exclusion)
- `internal/plugins/ospf/register.go` -- construct the IPsec installer and register its `OnInterfaceUp`/`OnInterfaceDown` callbacks against the `eng6` transport; load the xfrm dataplane backend when v6 IPsec is configured
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the `ipsec` container under the v6 address-family interface (`spi`, `protocol`, `algorithm`, `key`, `encryption`, `encryption-algorithm`, `encryption-key`), key leaves `ze:sensitive`
- `internal/plugins/ospf/cmd_show.go` + `internal/plugins/ospf/show_summary.go` -- `show ipv6 ospf interface` reflects IPsec enabled + protocol/SPI (never the key)
- `internal/plugins/ospf/v3/transport/transport.go` -- (only if needed) expose the bound `LinkLocalSource` for an open interface to the installer; otherwise read it via the existing handle accessor
- `internal/core/diagnostic/codes.go` -- register the `doctor-ospfv3-ipsec` diagnostic code (title/description/examples) alongside `doctor-ospfv3-raw-socket`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` -- the v6 interface `ipsec` container; read `ai/rules/config-surface.md` + `ai/rules/config-naming.md` |
| YANG validation constraints | [ ] yes | `spi` `range "256..4294967295"`; `protocol`/`algorithm`/`encryption-algorithm` enumerations; `key`/`encryption-key` `pattern` for hex + `length`; `ze:sensitive` on key leaves |
| YANG custom validators | [ ] yes | a `ze:validate` validator enforces hex-length-vs-algorithm, ESP-only-confidentiality, and the 7166 mutual-exclusion (native YANG cannot express cross-leaf rules) |
| CLI commands/flags | [ ] yes | `show ipv6 ospf interface` IPsec status in `ze-ospf-cmd.yang` + `cmd_show.go` |
| CLI grammar (action before identifier) | [ ] yes | `ai/rules/cli-grammar.md` -- `show ipv6 ospf interface` |
| Editor autocomplete | [ ] yes | automatic for the YANG enums; `CompleteFn` not required (no dynamic values) |
| Functional test for new RPC/API | [ ] yes | `test/ospfv3/ospfv3-ipsec-*.ci` |
| Pipe completeness | [ ] yes | `show ipv6 ospf interface` already routes through `ApplyPipes`; the IPsec field inherits it |
| Env var registration | [ ] no | IPsec parameters are per-interface operational config, not `environment/` leaves |
| Doctor check for runtime dependencies | [ ] yes | XFRM availability + manual-SA sanity: owning-package doctor check, `doctor-ospfv3-ipsec` in `internal/core/diagnostic/codes.go`, unit test, functional test (`ai/rules/doctor-checks.md`) |
| Prometheus counters/metrics | [ ] yes | see the metrics rows below |

#### Metrics (new series owned by this spec)
| Metric | Type | Labels |
|--------|------|--------|
| `ze_ospfv3_ipsec_sas` | gauge | `interface`, `protocol` (ah/esp), `direction` (in/out) |
| `ze_ospfv3_ipsec_policies` | gauge | `interface`, `direction` |
| `ze_ospfv3_ipsec_install_failures_total` | counter | `interface`, `reason` |
| `ze_ospfv3_ipsec_kernel_drops_total` | counter | `interface`, `reason` (no-policy/auth-failed), read from XFRM error stats |

> These extend the umbrella's canonical OSPFv3 metric set; they use the
> `ze_ospfv3_ipsec_*` prefix and are registered by this spec's owner code. The
> umbrella metrics table must gain these rows when this spec lands.

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` -- OSPFv3 IPsec AH/ESP authentication |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` -- the v6 interface `ipsec` block |
| 3 | CLI command added/changed? | [ ] yes | `docs/guide/command-reference.md` -- `show ipv6 ospf interface` IPsec status |
| 4 | API/RPC added/changed? | [ ] no | the show RPC lives in the central `ze-show` namespace; document under the command reference |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` -- OSPF gains an OSPFv3 IPsec installer |
| 6 | Has a user guide page? | [ ] yes | `docs/guide/ospf.md` (or the OSPFv3 page) -- an IPsec section, distinguished from RFC 7166 |
| 7 | Wire format changed? | [ ] no | the OSPFv3 packet bytes are unchanged; AH/ESP is applied by the kernel below the socket (note this in the wire doc) |
| 8 | Plugin SDK/protocol changed? | [ ] no | no plugin-host protocol change |
| 9 | RFC behavior implemented? | [ ] yes | `rfc/short/rfc4552.md` -- flip the "Compliance Checklist for a Future RFC 4552 Spec" items to implemented |
| 10 | Test infrastructure changed? | [ ] yes (interop scenarios added) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` -- OSPFv3 IPsec parity with FRR ospf6d |
| 12 | Internal architecture changed? | [ ] yes | the OSPF subsystem doc + the IPsec/dataplane doc -- the dataplane reuse and the install lifecycle |
| 13 | Route metadata keys added/changed? | [ ] no | IPsec does not install routes |
| 14 | Prometheus counters added/changed? | [ ] yes | the OSPFv3 telemetry doc -- the four `ze_ospfv3_ipsec_*` series |
| 15 | Registered plugin/event/command/capability inventory changed? | [ ] yes | `docs/plugin-overview.md` + the umbrella metrics table + the new diagnostic code |
| 16 | Changed source referenced by doc source anchors? | [ ] check | grep `docs/` for anchors into the changed dataplane/transport/config files |
| 17 | Existing docs show examples for this area? | [ ] check | verify OSPFv3 config examples against the new `ipsec` block; verify IPsec/dataplane docs against the new `SPParams.UpperProto` |

## Files to Create
- `internal/plugins/ospf/ipsec_install.go` -- the OSPFv3 IPsec installer: build selectors from the interface config + `LinkLocalSource`, translate to `dataplane.SAParams`/`SPParams`, install/remove/reconcile, `installIPsecHooks` (registers the `OnInterfaceUp`/`OnInterfaceDown` callbacks), the metrics
- `internal/plugins/ospf/ipsec_install_test.go` -- unit tests over a fake `dataplane.Dataplane` (records install/remove calls)
- `internal/plugins/ospf/config_ipsec_test.go` -- parse + validation tests
- `internal/plugins/ospf/doctor_ipsec.go` + `internal/plugins/ospf/doctor_ipsec_test.go` -- the `doctor-ospfv3-ipsec` check
- `test/ospfv3/ospfv3-ipsec-config.ci`, `ospfv3-ipsec-install.ci`, `ospfv3-ipsec-drop.ci`, `ospfv3-ipsec-rekey.ci`, `ospfv3-ipsec-remove.ci`, `ospfv3-ipsec-doctor.ci`
- `test/interop/scenarios/ospfv3-ipsec-frr/` -- `ze.conf`, `frr.conf`, `check.py` (ESP)
- `test/interop/scenarios/ospfv3-ipsec-ah-frr/` -- `ze.conf`, `frr.conf`, `check.py` (AH)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan -- confirm the dataplane seam, the OSPFv3 transport lifecycle hooks, and the v6 config split exist |
| 3. Wiring phase | Wiring Test table -- installer hooked to `OnInterfaceUp`/`OnInterfaceDown` + failing wiring tests |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist |
| 8. Fix issues | from critical review |
| 9. Re-verify | re-run stage 6 |
| 10. Repeat 7-9 | until clean |
| 11. Deliverables review | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 13. Re-verify | re-run stage 6 |
| 14. Present summary | Executive Summary per `ai/rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- the installer skeleton hooked to the OSPFv3 interface lifecycle
   - Tests: `TestIPsecInstallOnInterfaceUp`, `TestIPsecRemoveOnInterfaceDown`, `test/ospfv3/ospfv3-ipsec-install.ci`
   - Files: `ipsec_install.go` (`installIPsecHooks`, a stub installer that records intent against a fake dataplane), `register.go` (construct + hook against `eng6`)
   - Verify: `OnInterfaceUp`/`OnInterfaceDown` call the installer; the deeper SA-shape tests still fail because translation is a stub
2. **Phase: Config surface + validation** -- the v6 interface `ipsec` block
   - Tests: `TestParseOSPFv3IPsecConfig`, `TestIPsecRejectedUnderV4`, `TestIPsecAnd7166MutuallyExclusive`, `TestIPsecAHWithEncryptionRejected`, `TestIPsecKeyLengthValidation`, `test/ospfv3/ospfv3-ipsec-config.ci`
   - Files: `config.go` (parse + validate), `yang/ze-ospf-conf.yang` (the container + constraints + custom validator), the validator registration
   - Verify: well-formed blocks parse; every invalid combination is rejected with a clear message
3. **Phase: Dataplane extension** -- AH, transport-mode policy, UpperProto selector
   - Tests: `TestSAParamsAHNoEncryption`, `TestSPParamsUpperProtoSelector`, `TestDataplaneIKEUnaffected`, `TestXFRMInstallAHSA` (QEMU), `TestXFRMTransportPolicyUpperProto` (QEMU)
   - Files: `dataplane.go` (`SPParams.UpperProto`), `xfrm_linux.go` (AH install, transport-mode policy, UpperProto threading)
   - Verify: AH/ESP transport-mode + proto-89 policy install in QEMU; IKE ESP params are byte-identical
4. **Phase: Selector build + install/remove/reconcile** -- the real translation
   - Tests: `TestIPsecSelectorMatchesLinkLocal`, `TestSAParamsSharedKey`, `TestIPsecReconcileReplacesSA`, `TestIPsecInstallBeforeFirstHello`, `TestIPsecLoadsXFRMBackend`
   - Files: `ipsec_install.go` (selectors from `LinkLocalSource` + ff02::5/6 + fe80::/10; one key -> in+out SA; reconcile diff; load the backend)
   - Verify: install precedes the first Hello; reconcile replaces cleanly; selectors match the on-wire source
5. **Phase: Doctor + metrics + show** -- the operator surface
   - Tests: `TestIPsecDoctorWarnsWhenXFRMUnavailable`, `test/ospfv3/ospfv3-ipsec-doctor.ci`, the four metric series asserted in install tests
   - Files: `doctor_ipsec.go`, `codes.go` (the diagnostic code), `cmd_show.go`/`show_summary.go` (IPsec status), metric registration in `ipsec_install.go`
   - Verify: doctor warns correctly; `show ipv6 ospf interface` shows IPsec on without the key; metrics register
6. **Functional tests** -> the six `.ci` cover config, install, drop, rekey, remove, doctor (drop/install are QEMU)
7. **RFC refs** -> add `// RFC 4552 Section X` comments on the transport-mode (§2), AH/ESP (§3), confidentiality-ESP-only (§4), proto-89/IP-version (§5), interface-selected-SPD (§6), and manual-shared-key (§7) enforcement points
8. **Interop** -> `ospfv3-ipsec-frr` (ESP) and `ospfv3-ipsec-ah-frr` (AH) QEMU scenarios
9. **Full verification** -> `make ze-verify`
10. **Complete spec** -> audit tables + learned summary; two commits (A: code+spec+learned, B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has a file:line implementation |
| Feature completeness | each user story has a working path; FRR ospf6d IPsec parity (ESP + AH, transport mode, manual keys) proven by interop |
| Correctness | transport mode only; ESP-must/AH-may; confidentiality ESP-only; proto-89 selector; shared key feeds both SAs (§7); install precedes first Hello; reconcile removes the old SA |
| Naming | `ze_ospfv3_ipsec_*` metrics; YANG `ipsec`/`spi`/`protocol`/`algorithm`/`key` kebab-case; `doctor-ospfv3-ipsec` |
| Data flow | config -> installer -> dataplane -> kernel; the dataplane names no OSPF symbol; the kernel verifies (Ze never sets `dispatch.authOK` for IPsec) |
| CLI grammar | `show ipv6 ospf interface` action-before-identifier |
| Doctor checks | `doctor-ospfv3-ipsec` registered, no-op unless IPsec configured, warns when XFRM unavailable |
| YANG validation | `spi` range, enums, hex pattern+length, `ze:sensitive` keys, custom cross-leaf validator |
| Prometheus counters | the four series defined, registered, listed; umbrella table updated |
| Rule: plugin-self-containment | the OSPFv3 plugin owns config/install/doctor/metrics; the dataplane stays OSPF-agnostic |
| Rule: module-tiers | the installer (plugin tier) calls the dataplane (component tier); no reverse dependency |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| IPsec installer hooked to interface lifecycle | `grep -rn 'OnInterfaceUp\|OnInterfaceDown' internal/plugins/ospf/ipsec_install.go` |
| Config parse + validation | `go test ./internal/plugins/ospf -run 'IPsec.*Config\|IPsecAnd7166\|IPsecRejected'` |
| Dataplane AH + UpperProto + transport policy | `go test ./internal/component/ike/dataplane -run 'AH\|UpperProto\|IKEUnaffected'` |
| `doctor-ospfv3-ipsec` registered | `grep -rn 'doctor-ospfv3-ipsec' internal/` |
| Four metric series | `grep -rn 'ze_ospfv3_ipsec_' internal/plugins/ospf` |
| ESP + AH interop scenarios present | `ls test/interop/scenarios/ospfv3-ipsec-frr/ test/interop/scenarios/ospfv3-ipsec-ah-frr/` |
| Functional tests present | `ls test/ospfv3/ospfv3-ipsec-*.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | SPI range (reject 0-255 reserved), hex key length vs algorithm, enum-only protocol/algorithm; malformed hex rejected at parse, never passed to the kernel |
| Key handling | `ze:sensitive` on key leaves; the installer never logs key bytes; `show`/doctor report presence/protocol/SPI, never the key (R-4) |
| Silent-protection failure | when XFRM is unavailable and IPsec is required, the operator is warned and the interface is not falsely reported protected (R-7, AC-12) |
| Drop semantics | unprotected/failed packets dropped by the kernel before the socket (§3/§4); confirm no Ze code path accepts a plaintext OSPFv3 packet on an IPsec interface (AC-8/AC-9) |
| Mutual exclusion | a 7166 + IPsec config cannot silently run both; the validator rejects it (AC-6) |
| Privilege | XFRM install needs CAP_NET_ADMIN; document the requirement and fail loud, not silent |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to the relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE -->

## Core Insight
RFC 4552 in Ze is a **kernel-policy install** feature, not a packet feature: the
existing OSPFv3 raw-socket transport and codecs stay untouched, and the only new
runtime work is installing a transport-mode XFRM policy+SA for proto-89 traffic
on each IPsec interface and letting the kernel apply/verify AH/ESP. The reuse
seam (`internal/component/ike/dataplane`) already does the netlink mechanics; the
gap is purely that it was shaped for IKE ESP child SAs (tunnel mode, ESP only, no
upper-layer selector), so a small additive extension (AH proto, transport-mode
policy, a proto-89 selector) makes it serve OSPFv3 manual SAs too. Manual
shared-per-interface group keying (§7) is what makes IKE inapplicable and the
config per-interface rather than per-neighbour.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Reuse `internal/component/ike/dataplane` for kernel SA/policy install | a dedicated OSPFv3 XFRM client | one netlink path, one place to test/secure; the abstraction already supports IPv6, transport mode for SAs, and ESP; the extension is small and benefits any future kernel-IPsec consumer |
| Install on `OnInterfaceUp`, remove on `OnInterfaceDown` | a separate IPsec reconcile loop independent of the transport | the transport already resolves ifindex + link-local and fires at exactly the right moments; reusing it guarantees the policy exists before the first Hello |
| Kernel verifies; Ze never sets `dispatch.authOK` for IPsec | mirror the 7166 verify hook | AH/ESP is stripped by the kernel below the socket; §3/§4 silent-discard is a kernel inbound-policy property, not a Ze parse |
| Per-interface shared key feeding both in and out SAs | per-neighbour SA pairs | RFC 4552 §7: multicast group traffic is one-to-many; per-neighbour SAs do not scale and IKE cannot key the group; shared manual keys are mandated |
| IPsec and RFC 7166 mutually exclusive per interface | allow both, let one win | two independent auth mechanisms on one interface is ambiguous and a security footgun; the validator forbids it |
| Config under the v6 address family only | a shared interface block | RFC 4552 is OSPFv3-only (§5); placing it under v4 would be inert and misleading |

## Known Limitations
- Manual SAs only (RFC 4552 §7); no automatic rekey -- key rotation is an operator config change.
- Transport mode only; tunnel mode and virtual-link dynamic IPsec policy are deferred.
- Linux XFRM backend only; VPP IPsec for OSPFv3 is not implemented here.
- AH is best-effort (RFC 4552 §3 "MAY"); ESP is the primary, fully-validated path. If AH transport-mode interop proves fragile against FRR, AH stays documented-but-secondary.
- The silent-discard guarantee (§3/§4) depends on the kernel XFRM inbound policy; Ze surfaces the drop as a metric but cannot itself inspect the dropped (already-discarded) packet.

## RFC Documentation

Add `// RFC 4552 Section X.Y: "<quoted requirement>"` above the enforcing code:
- §2 transport-mode SAs (the `Mode=transport` install)
- §3 ESP must / AH may (the protocol enum + AH install path)
- §4 confidentiality via ESP only (the AH+encryption rejection)
- §3/§4 silent discard of unprotected/failed packets (the kernel inbound policy + the drop metric)
- §5 protocol 89 / IP-version selector (the proto-89 + IPv6 selector)
- §6 interface-selected SPD with src/dst/proto/dir/interface selectors (the per-interface policy build)
- §7 manual keying, shared in/out group key (the single-key -> two-SA mapping)

## Implementation Summary

### What Was Implemented
- [filled at implementation time]

### Bugs Found/Fixed
- [filled at implementation time]

### Documentation Updates
- [filled at implementation time]

### Deviations from Plan
- [filled at implementation time]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|-----------|-------|

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
| Manual SA config (SPI/protocol/algorithm/keys) | unit + functional | `TestParseOSPFv3IPsecConfig`, `ospfv3-ipsec-config.ci` |
| Kernel transport-mode policy+SA for proto-89 installed on interface up | unit + interop | `TestIPsecInstallOnInterfaceUp`, `ospfv3-ipsec-frr` (`ip xfrm` shows transport SA) |
| AH and ESP supported (ESP must, AH may) | interop | `ospfv3-ipsec-frr` (ESP), `ospfv3-ipsec-ah-frr` (AH) |
| Verify-on-receive via kernel (silent discard of unprotected/failed) | functional (QEMU) | `ospfv3-ipsec-drop.ci`, `ze_ospfv3_ipsec_kernel_drops_total` |
| Reuse the IKE dataplane without regressing it | unit | `TestDataplaneIKEUnaffected` |
| RFC 7166 mutual exclusion + v6-only config | unit | `TestIPsecAnd7166MutuallyExclusive`, `TestIPsecRejectedUnderV4` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE |  | file:line |  |

### Fixes applied
-

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/ospf/*`, `internal/component/ike/dataplane/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass)
- [ ] RFC 4552 constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (the dataplane extension serves a real second consumer -- OSPFv3 -- not speculation)
- [ ] No speculative features (transport mode + AH/ESP only; tunnel/virtual-link/VPP deferred)
- [ ] Single responsibility per component (installer = lifecycle + translation; dataplane = netlink)
- [ ] Explicit > implicit behavior (install precedes first Hello; failure is loud)
- [ ] Minimal coupling (dataplane names no OSPF symbol)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospfv3-ipsec-frr`, `ospfv3-ipsec-ah-frr`)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ospfv3-ext-2-ipsec-auth.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospfv3-ext-2-ipsec-auth.md`
