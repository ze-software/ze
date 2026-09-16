# Spec: path-mtu-diagnostic

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | - |
| Phase | 6/6 |
| Handoff | - |
| Updated | 2026-09-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An IPsec tunnel sized larger than the path it rides on fragments or blackholes,
and neither symptom names its cause. The operator sees slow transfers, stalled
TLS handshakes and working pings, and has no way to ask the box what the path
actually carries.

`show mtu` answers that. It measures the real underlay path MTU to every
configured site-to-site peer and to an arbitrary host, derives the ESP ceiling
for each tunnel from the transform that tunnel actually negotiated, and reports
whether the interface is oversized, tight, under-utilised or correct, with the
configuration commands to fix it.

This ports a tool Exa runs in production on VyOS CPE (`faros_mtu.py`, version
2.2, from the FarOS package set), as part of migrating FarOS to Ze. The port is
not a translation. The Python shells out to `ping`, `ip`, `nstat` and `sysctl`
because VyOS operational mode is a shell, and it hardcodes AES-GCM-128 with an
IPv4 underlay and IPv6 inner because it cannot see what a tunnel negotiated. Ze
is the IKE daemon, so it derives the transform per child SA and refuses nothing.
The distilled arithmetic, probe budget, candidate ladder and verdict thresholds
are recorded in the session scratch note named under Required Reading.

The module is its own feature, removable by dropping a blank import (owner
requirement, 2026-09-11). It therefore takes IPsec state through a registered
snapshot rather than importing the IKE engine.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/plugin-system.md` - cross-boundary value types and the communication patterns
  → Constraint: a payload crossing a boundary carries no pointer fields, so the tunnel snapshot is a self-contained value type
  → Decision: the EventBus is asynchronous one-to-many with no replay, so an event subscription cannot answer "what tunnels exist right now"; a module started after a tunnel came up would report none
- [ ] `ai/patterns/registration.md` - every registry and its Register and Query symbols
  → Constraint: a feature registers itself and is discovered; no central list learns its name
  → Decision: `internal/core/diagnostic/doctor_registry.go` is the shape to copy for a registered synchronous query, and the IKE component is already a registrant there
- [ ] `ai/patterns/cli-command.md` and `ai/rules/cli.md` - the command contract
  → Constraint: keyword before value, and the handler returns structured data so `| json`, `| yaml` and `| table` each render one payload
  → Constraint: a severity marker is never glued to a name; the verdict is its own field
- [ ] `docs/architecture/diagnostics/active-probes.md` - the probe layer this depends on
  → Constraint: probes are in-daemon sockets; the DF mode and the error-queue read this module consumes landed with spec-probe-do-not-fragment (closed 2026-09-15) and are described on this page under "The Don't Fragment mode" and "The error queue"
- [ ] `docs/contributing/ze-go-style.md` - the working standard
  → Constraint: `netip.Addr` for an address, never a string; a typed enum whose zero means `Unspecified` for the verdict
  → Constraint: state the limit. The probe budget, the follow count and the candidate ladder are bounded and the bound is in the code

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc4301.txt` - Security Architecture for IP, Section 8
  → Constraint: Section 8.2.2, "In all IPsec implementations, the PMTU associated with an SA MUST be 'aged'". `plan/immediate/spec-rfc4301-architecture-gaps.md` phase 7 owns that value; this module READS it and never writes it (owner decision, 2026-09-11)
- [ ] `rfc/full/rfc1191.txt` - Path MTU Discovery
  → Constraint: Section 7 warns that a plateau table with too many entries wastes the search, and calls its own eleven values "an implementation suggestion, NOT a specification or requirement". The 31-value ladder is a deliberate departure, justified in Key Design Decisions (owner decision, 2026-09-11)
  → Constraint: Section 3, a reported MTU of zero is the old-router signal to start searching, not a value to discard. The ported Python discards it, which is a defect this port does not carry across
- [ ] `rfc/full/rfc4821.txt` - Packetization Layer Path MTU Discovery
  → Constraint: Section 7.6.4, "The presence of other losses near the loss of the probe may indicate that the probe was lost due to congestion rather than due to an MTU limitation. In this case, the state variables ... SHOULD NOT be updated". A bisection that moves its bounds on every silent probe walks the estimate down under congestion
- [ ] `rfc/full/rfc8899.txt` - Datagram PLPMTUD
  → Constraint: Section 5.1.3, "The default value of MAX_PROBES is 3". The ported Python retries once; three consecutive attempts of one size is the figure with an authority behind it
- [ ] `rfc/full/rfc8200.txt` - IPv6
  → Constraint: Section 5, the 1280 minimum link MTU is the floor below which no IPv6-carrying tunnel may be sized
- [ ] `rfc/full/rfc791.txt` - IPv4
  → Decision: 576 is a reassembly minimum, not a path-MTU floor, and is used here as a documented default rather than published as conformance

### Other
- [ ] `tmp/session/2026-09-15-e9e5e97a-9494-4074-a088-c3bd059b2fb3/scratch/faros_mtu.py` - `faros_mtu.py` 2.2 itself, supplied by the owner on 2026-09-16; the distillation below is what this spec pins, so the file is provenance rather than a dependency
  → Constraint: the ESP arithmetic, the verdict thresholds and the underlay advice matrix are reproduced from a tool in production; changing a threshold changes a verdict an operator already relies on
  → Decision: the colour-on-stderr handling, the subprocess prober and the refusal to advise on a non-standard proposal do not port

**Distillation of `faros_mtu.py` 2.2 (the figures this port pins):**

| Quantity | Value in the Python | Ze |
|----------|---------------------|-----|
| ESP overhead | outer IP header (20 v4, 40 v6) + 8 UDP when encapsulated + 8 ESP header + cipher IV + cipher ICV; AES-GCM-128 has IV 8, ICV 16, alignment block 4 | derived per negotiated transform, outer family from the installed endpoint |
| Ceiling (`max_mtu`) | `alignDown(underlay - overhead, block) - 2` (the 2 is the ESP trailer); 0 when nothing is left | same |
| Recommended | `alignDown(ceiling - 32 + 2, block) - 2`; absent when below the inner family's minimum link MTU (1280 inner v6, 576 inner v4) or when the ceiling is 0 | same; the margin of 32 is a named constant |
| MSS | `mtu - inner header (40 v6, 20 v4) - 20`; absent when not positive | same |
| Verdict order, first match wins | interface down; no usable value (`recommended` absent); oversized when `now > ceiling` (excess = now - ceiling); tight when `now > ceiling - 32` (spare = ceiling - now); under-utilised when `now != recommended` (gain = recommended - now); ok | the five verdicts, same thresholds, as a typed enum |
| Run verdict line | non-standard proposal: DO NOT APPLY; any oversized or no-usable: ACTION NEEDED; any tight or down: CHECK; any fault outside the table: ACTION NEEDED; only under-utilised: OK; no peers: NO TUNNELS; else OK | the same ladder, as a field |
| Remediation | one `set interfaces vti <if> mtu <recommended>` per oversized, tight or under-utilised tunnel; the underlay command only from the advice matrix; then a route-cache flush | Ze's own config syntax, as a list field |
| Probe budget | payload 68..65000; ICMP overhead 28 (v4) / 48 (v6); one retry on a silent loss, none on an explicit refusal; wait 2 s per probe | RFC 8899 MAX_PROBES 3 replaces the single retry (AC-10) |
| DF sanity gate | a 10000-octet payload MUST be refused before any figure is believed; a reply means DF is not honored and the run stops with exit 3 | kept: the probe layer's own DF proof makes it a cheap assertion, not a gate on a shell binary |
| Reported-MTU follow | confirm a reported wire MTU by `size passes` and `size + 1 fails`; follow a new reported value up to 6 times; the Python discards a reported value below 68 (this port does not: RFC 1191 zero is the search signal) | same, follows capped at 6 |
| Search | candidates inside the bracket first (31 values: 1500, 1492, 1480, 1476, 1472, 1468, 1462, 1460, 1458, 1454, 1452, 1450, 1442, 1438, 1436, 1430, 1428, 1420, 1412, 1400, 1398, 1380, 1358, 1350, 1300, 1280, 1260, 1200, 1100, 1024, 576), bisected by index; then a floor probe at 68; a contradicting bracket (`high <= low`) is re-tested and noted as a lossy path; then numeric bisection | same |
| How the answer was found | `via ICMP`, `via local iface MTU`, `ICMP under-reported`, `ICMP over-reported`, `ICMP filtered`, `ICMP kept changing`, `forced full search` | a typed method field with the same seven meanings |
| Force | discard the reported value and the bracket, search the full range with the cache bypassed | `DFBypassCache` from the probe layer |
| Reference targets | 1.1.1.1 then 8.8.8.8, first reachable; skipped for a single-host run | a config leaf, default 1.1.1.1 |
| Underlay advice | `current == min_measured`: capped by the interface (caution) unless current is 1500; `current > min_measured` and no reference: undecidable (caution); reference >= current: do NOT lower (info); reference == min_measured: whole circuit clamped, set underlay to reference (caution); otherwise: two clamps in series, set underlay to min(reference, min_measured) and confirm with a third address (caution) | AC-12, AC-13, AC-14 |
| Cache note | a cached PMTU that differs from the measurement is a caution naming `mtu_expires`; one that agrees is information unless forced | AC-11 |
| Local state | `tcp_mtu_probing` 0 is information; `IpFragOKs`/`Ip6FragOKs` non-zero is a caution; `IpFragFails` non-zero is a fault (a live PMTU blackhole); `IpReasmFails` non-zero is a caution; counters are absolute since boot | AC-20 |
| Peers | every remote address of a peer is measured and the tunnel is sized from the tightest; `any` is not probed; an unmeasured peer assumes the tightest measured path and is marked assumed; a peer with no bound interface is not sized | AC-7, AC-17, Known Limitations |
| ESP proposal | the Python reads the FIRST configured proposal and refuses to advise on a non-standard one; a non-standard proposal on an unused group is information only | Ze reads the negotiated transform (AC-15) and derives the overhead, so the refusal has no counterpart |
| Exit codes | 0 ok, 1 usage, 2 nothing measured, 3 DF gate failed | the command's status field carries the same four outcomes |

**Key insights:**
- The ESP ceiling is `alignDown(underlay - overhead, block) - trailer`, where the overhead is the outer IP header, 8 more octets when UDP encapsulation is in use, the 8-octet ESP header, the cipher IV and the cipher ICV. Every one of those except the outer header comes from the negotiated transform, which is why Ze can compute what the Python had to assume.
- The recommended MTU backs off 32 octets from the ceiling and lands on the cipher's alignment boundary, and is *absent* rather than small when that lands below the inner family's minimum link MTU.
- The reference measurement against an address unrelated to the tunnels is what makes the underlay advice decidable: a clamped access circuit is worth matching, a clamped peer path is not.
- A verdict is a field, not a prefix on a string, so the table renderer and `| json` agree by construction.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/reconcile.go` - `PeerInfo` and `(*PeerSession).Info()`; the snapshot taken under `ps.mu`, child fields filled only when the child SA exists
- [ ] `internal/component/ike/engine/child.go` - `ChildSA` holds `UDPEncap`, `Mode` and `RemoteAddr`; none of the three reaches `PeerInfo`
- [ ] `internal/component/ike/engine/responder.go` - `selectResponderESP` narrows `sa.ESPGroup.Proposals` to the single accepted proposal, and `matchOfferedESPProposal` is what selects it
- [ ] `internal/component/ike/engine/register.go` - `ActiveTable` and `ActivePeers` are process globals, reachable only from inside the IKE component
- [ ] `internal/component/ike/cmd/show_ipsec.go` - the existing read-only IPsec command and its row builder, the payload template to follow
- [ ] `internal/component/ping/cmd/register.go` and `internal/plugins/ping-cmd/yang/register.go` - the component-plus-YANG-plugin pair this module copies
- [ ] `internal/component/iface/dispatch.go` - `ListInterfaces`, `GetInterface` and `GetXFRMInfo`; the name-to-if_id mapping lives here, not in IKE
- [ ] `internal/component/iface/config.go` - the configured interface MTU, separate from the kernel one; no symbol reports both
- [ ] `internal/component/sysctl/backend_linux.go` - `read` is unexported and owns the key-to-path mapping including the interface-name case
- [ ] `internal/component/telemetry/collector/snmp_linux.go` - the vendored `procfs.ProcSnmp` is already read here, as rates rather than absolute values

**Behavior to preserve:**
- Every existing `show vpn ipsec sa` payload key. The negotiated-transform fix changes a value, not a key.
- The IKE engine's locking: the snapshot is taken the way `Info()` already takes it.
- `internal/component/sysctl`'s key-to-path mapping stays the single declaration of that mapping.

**Behavior to change:**
- `PeerInfo` gains the UDP-encapsulation flag, the mode, the installed remote address, and reports the negotiated transform rather than the configured one.
- The sysctl component gains an exported read, so no second copy of the key mapping is written.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `show mtu`, `show mtu host <address>`, with `exhaustive` and `detail` available under either. The Python's `force` was renamed on 2026-09-16 (owner decision): `exhaustive` names what the operator gets, a slower run whose figure came from probing every size on the wire rather than from anything the kernel remembered or a router reported, while `force` on a read-only diagnostic reads as a guard being overridden and `bypass-cache` needs the kernel's cache to be understood. `host <address>` is the only selector (owner decision, 2026-09-16): a pipe runs after the handler has answered, so `| match` cannot limit which peers are probed, and a `peer <name>` selector was considered and declined.
- No value at all in the default form: the peers come from the running IKE state.

### Transformation Path
1. The command handler asks the registered IPsec inventory for a snapshot of live tunnels.
2. For each distinct peer address, and for the reference address, the probe layer measures the path MTU.
3. Per tunnel, the ESP overhead is derived from that tunnel's negotiated transform, mode and encapsulation; the ceiling and the recommended value follow.
4. Each tunnel is classified against its interface's current MTU.
5. Local state is read: the kernel's cached PMTU, the fragmentation counters, and `tcp_mtu_probing`.
6. One payload is returned, carrying the measurements, the tunnel rows with their verdicts, the remediation commands and the notes. The pipe operators render it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| mtu ↔ ike | a registered snapshot query in a core leaf, value types only | Yes (2026-09-16): `liveDeps.tunnels = ipsecinventory.Tunnels` (`run.go`); the engine registers in `register.go` via `inventorySnapshot` (`ike/engine/inventory.go`); `TestIPsecInventoryRegisteredByIKE`; `internal/component/mtu/cmd` imports `ike/crypto` and `ike/ipsec` (untagged leaves, for the transform ids) and never `ike/engine`; `./le tier check` clean (phase 6b) |
| mtu ↔ iface | `GetXFRMInfo` and `GetInterface` for the if_id-to-name resolution and the interface MTU | Yes (2026-09-16): `liveDeps.getInterface = iface.GetInterface`, `xfrmInterfaces = listXFRMInterfaces` (`ListInterfaces` + `GetXFRMInfo.IfID`), `routeInterface = routeInterfaceOf` (`EnsureBackend` + `RouteLookup`), all in `run.go`; `show-mtu-oversized-tunnels.ci` resolves if_id 7 and 8 to `xa` and `xb` on a live xfrm backend |
| mtu ↔ sysctl | an exported read for `tcp_mtu_probing` | Yes (2026-09-16): `readTCPMTUProbing` (`state.go`) calls `sysctl.Read` (`backend.go`); `TestReadAnswersTheKernelValueOrSaysWhy` (`sysctl/backend_linux_test.go`), `TestLocalStateReadsTheRunningKernel` (`state_linux_test.go`) |
| mtu ↔ probe layer | the DF mode and the error-queue read of `internal/core/probe` (`OpenICMP`, `Socket.DrainErrors`, `KernelPathMTU`; `docs/architecture/diagnostics/active-probes.md`) | Yes (2026-09-16): `openLiveProber` and `liveDeps.kernelPathMTU = probe.KernelPathMTU` (`run.go`); `TestSearchClampedPathViaICMP`, `TestSearchExhaustiveBypassesPoisonedCache` (`search_integration_linux_test.go`) against a Linux router; `show-mtu-host.ci` measures 1400 through the sender/router/far namespaces |
| Component ↔ CLI | `ze-mtu-cmd.yang` and the registered RPC | Yes (2026-09-16): `pluginserver.RegisterRPCs` and `command.RegisterShape` (`mtu/cmd/register.go`); `TestShowMTUReachesTheHandler`, `TestShowMTUHostRefusedByTheGrammar` (`mtu_test.go`); `TestShowMTUArgDefsByName` (`config/yang/command_test.go`); the five `show-mtu-*.ci` |

### Integration Points
- `internal/core/ipsecinventory` is new: the IKE engine registers a snapshot function at `init()`, the mtu module calls it. Neither imports the other.
- `internal/component/mtu/cmd/` registers the RPC and the local meta, copying `internal/component/ping/cmd/register.go`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | `handleShowMTU` (`mtu.go`) -> `runMTU` (`run.go`) -> `mtuDeps` (`liveDeps`): tunnels through `ipsecinventory.Tunnels`, interfaces through `iface.GetInterface`/`ListInterfaces`/`GetXFRMInfo`, the sysctl through `sysctl.Read`, the probes through `probe.OpenICMP`/`DrainErrors`/`KernelPathMTU`, the counters through the vendored procfs. No `/proc/sys` path is opened by hand and no netlink socket is opened outside `iface` |
| No unintended coupling (components stay isolated) | Yes | `internal/component/mtu/cmd` imports `component/command`, `component/iface`, `component/plugin`, `component/sysctl`, `ike/crypto`, `ike/ipsec` (leaves), `core/diagnostic`, `core/env`, `core/ipsecinventory`, `core/probe`, `core/textbuf`; never `ike/engine` or `ike/cmd`. `ike/engine` imports `core/ipsecinventory` (stdlib-only leaf), never `mtu`. `./le tier check` clean (phase 6b, `scratch/`). `docs/architecture/core-design.md` section 19 carries the `mtu` row |
| No duplicated functionality (extends existing, does not recreate) | Yes | the prober is `internal/core/probe` (spec-probe-do-not-fragment), the ICMP socket doctor check is `checkICMPProbeSocket` (`probe/doctor.go`, not duplicated), the ICV lengths come from `crypto.AEADICVOctets` and the integrity registry's `TruncatedLength` (`overhead.go`, `espICVOctets`), the sysctl read is `sysctl.Read` over the existing backend, the interface facts come from `iface`, the pipes from `ApplyPipes`. The one new declaration is `espCipherWires` (IV and block per cipher), which nothing else in the tree held |
| Zero-copy preserved where applicable (refs, not copies) | Yes | not a wire path: one payload per operator-invoked run. `ipsecinventory.Tunnels` answers value-type records (a snapshot by design, so the engine's SA is never aliased); the wire prober allocates one `payload` buffer and one `readBuffer` per target (`openWireProber`, `search.go`) and slices them per size; `textbuf` builds the note strings |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | `mtu/cmd/register.go`: `pluginserver.RegisterRPCs` (the RPC), `command.RegisterShape` (the shape), `diagnostic.RegisterDoctorCheck` (`mtu-local-state`), `env.MustRegister` (`ze.mtu.reference-address`); `mtu/yang/register.go` and `plugins/mtu-cmd/yang/register.go` register the two YANG modules; `plugin/all/all.go` gained its three blank imports by `./le repository generate`. Derived from those registries, with no edit: the RPC dispatcher, the pipe catalog (`TestShowMTUEveryPipeRendersOnePayload`), the CLI completer and help (`TestShowMTUArgDefsByName`), the doctor runner (`TestDoctorMTULocalStateReportsEachUnreadableSource` through `DoctorChecksForPhase`), the published site catalog (`./le site build`, owed) |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Partial: five hand-kept lists learned a name, each named here | Searched for `mtu`, `show mtu`, `reference-address`, `doctor-mtu-local-state`, `ze-mtu-cmd`, `ze-mtu-conf`. Lists that had to learn the name: (1) `sectionMTU` (`config/graph.go`) and its entry in `extractSections` (`config/constants.go`): the environment sections `ApplyEnvConfig` extracts are a hand-kept list, as they are for `bgp`, `reactor` and `chaos`; (2) `envPlumbingTable` (`config/apply_env.go`): the YANG-leaf-to-env-key map is hand-kept for every section; (3) `builtinCodes` (`core/diagnostic/codes.go`): every diagnostic code is declared there, the pattern `checkICMPProbeSocket` also follows; (4) `TestArgDefsPopulated` (`config/yang/command_test.go`): its `cmdFiles` and `wantArgDefs` are hand-written because `yang.Modules()` is filled by the owning packages' `init()` and the test package cannot import them without a cycle; (5) `docs/guide/command-catalogue.md` row 322 and the regenerated `../wiki/command-catalog.md`. Each of the five is the SAME list every existing command feeds, none is a per-feature switch, and none was created by this spec. Lists checked and NOT edited: the RPC dispatcher, the pipe catalog, the completer, the doctor registry, the `ze explain` code index (reads `builtinCodes`), the site catalog: each derives from a registry |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | After negotiation, the single proposal left in the SA's ESP group is the one actually installed in the kernel | `selectResponderESP` narrows `sa.ESPGroup.Proposals` to the accepted proposal, and `installChildSA` reads index zero | the derived overhead is wrong for every tunnel and the tool repeats the failure it exists to remove | a QEMU test negotiating the SECOND of two configured proposals and asserting the reported transform and the derived overhead both follow it | confirmed (2026-09-16, phase 2): read at the producer on both roles and every rekey path: `selectResponderESP` (responder.go) narrows `sa.ESPGroup` before `createFirstChildSA` copies it onto the child; the initiator narrows at the IKE_AUTH accepted-offer check (fsm.go, `sa.ESPGroup.Proposals = []ipsec.ESPProposal{offer.ESPConfig}`); `handleChildRekeyResponse` sets `child.ESPGroup.Proposals = {prop}` (rekey.go); the responder rekey inherits `old.ESPGroup`, already narrowed, and matches against its index zero. The `mtu-negotiated-transform` scenario against strongSwan accepting the second of two proposals is green with the fix and red with `Info` reading the configured group |
| A-2 | The cipher's IV and ICV lengths, and its alignment block, are derivable from the negotiated transform for every transform Ze offers | the transform identifies the algorithm, and the arithmetic is a property of the algorithm | the overhead is wrong for some ciphers and right for others, which is worse than being wrong for all | a table-driven test over every ESP transform Ze can negotiate, asserting the derived overhead against hand-computed values | confirmed (2026-09-16, phase 3): `TestESPOverheadPerTransform` derives its population from `ipsec.SupportedESPEncryptionNames` and `crypto.SupportedIntegrityNames` (8 pairs: aes128gcm, aes256gcm, and aes128/aes256 with sha256/sha384/sha512), checks each over both endpoint families with and without UDP against hand-computed octets, and refuses to pass while a negotiable pair has no row. The IV and block are declared once in `espCipherWires` (overhead.go); the ICV is read from the crypto package (`AEADICVOctets`, the integrity registry's `TruncatedLength`). RED observed with the AES-GCM IV cut to 16 (`TestESPOverheadPerTransform` and `TestESPOverheadTransportMode` failed), GREEN restored |
| A-3 | `ChildSA.RemoteAddr` is the address the SA is installed on and differs from the configured address behind NAT | the peer endpoint is adopted after authentication | probes go to the configured address and measure a path the ESP traffic does not ride | a NAT interop scenario asserting the probed address equals the installed one | BROKEN as a statement about the engine, benign for the design (2026-09-16, phase 6): read at the producer, `createFirstChildSA` (`ike/engine/child.go`) takes the remote address from its caller, and both callers pass the CONFIGURED `peer.RemoteAddress` (`initiatorFirstChildSA` in `child.go`; `buildAuthResponse` in `responder.go`); `rekey.go` copies `old.RemoteAddr`; `adoptAuthenticatedEndpoint` (`sa.go`) moves only `sa.peerEndpoint`, which no Child SA reads. So `InstalledRemote` equals `ConfiguredRemote` in every reachable path today and the "differs behind NAT" premise is unreachable in Ze. The module's choice stands: `tunnelTarget` (`run.go`) probes `InstalledRemote`, the address the kernel SA carries ESP to, so the probe follows the engine the day it adopts the authenticated endpoint. `mtu-nat-installed-endpoint` proves across a real NAT that the probe goes where the ESP goes (172.28.0.7, the NAT box's face; `interop-nat-green1.log`), RED with the inventory cut `t.InstalledRemote = info.ChildLocalAddr` (`interop-nat-red.log`: `show mtu carries no peer measurement of 172.28.0.7`); `TestShowMTUProbesTheInstalledEndpoint` (`run_test.go`) covers the differing case over a fake inventory. The engine-side find is one row in `plan/journal/one-state-held-in-two-fields.md` |
| A-4 | A registered snapshot query is reachable synchronously from the mtu module in every deployment shape | the doctor registry works this way today and IKE is already a registrant | the module reports no tunnels on a box that has them, which is a silently-wrong zero | the snapshot returns an explicit "not registered" outcome, and a functional test asserts the message an operator sees when IKE is absent | confirmed (2026-09-16, phase 6): `ipsecinventory.Tunnels` (`registry.go`) answers `ErrNotRegistered` when no engine registered, `TestInventorySnapshotUnregisteredIsNotEmpty` (`registry_test.go`); the run maps it to `inventory: not-registered` beside `registered` with zero tunnels, `TestShowMTUNoInventoryIsNotZeroTunnels` (`run_test.go`), RED under the cut `r.inventory = inventoryNotRegistered` (`ci-mtu6-red.log`, "inventory not-registered, want registered"). The functional half is `show-mtu-no-ipsec-component.ci` (registered, no tunnels): the runner links one `ze` with every component, so the not-registered arm is the unit test (Known Limitations) |
| A-5 | The verdict thresholds carried over from `faros_mtu.py` are right for Ze's operators too | the tool is in production on Exa CPE | verdicts disagree with the tool operators already trust, during a migration where both run | the thresholds are stated in the spec and reviewed by the owner before implementation; a table-driven test pins each boundary | confirmed (2026-09-16): the owner supplied `faros_mtu.py` 2.2 itself on 2026-09-16 and the distillation table in Required Reading pins each figure to the script; `TestVerdictThresholds` (`verdict_test.go`) pins every boundary of the five verdicts, RED under the `>` to `>=` cut (phases 3+5), `TestRunVerdictLadder` pins the run verdict order |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `plan/immediate/spec-rfc4301-architecture-gaps.md` phase 7 introduces a per-SA PMTU while this module also holds one, and the two disagree | both specs edit the IKE engine's notion of a path MTU | this module READS and never writes (owner decision). Where phase 7 has landed, disagreement between the engine's value and the measured one is reported as a finding rather than silently resolved |
| R-2 | The measurement takes 30 to 60 seconds on a degraded path, and a CLI caller times out or an operator thinks the box has hung | a functional test with an unreachable peer | progress is emitted per peer as the run proceeds, and the probe budget is bounded so the worst case is computable rather than open |
| R-3 | Probing every configured peer from a production CPE emits traffic an operator did not expect | a customer asks why their firewall logged probes | the command is operator-invoked and never scheduled; the payload states what was sent and to where |
| R-4 | The reference measurement sends probes to an address outside the operator's control | a policy objection to pinging a public resolver by default | the reference target is a config leaf, defaulting to 1.1.1.1, and a run with it unset simply reports that the underlay advice is undecidable |
| R-5 | Deriving overhead per transform is more surface than hardcoding it, and a transform Ze can negotiate but whose arithmetic is unwritten produces a wrong number | a new ESP transform is added and nobody updates the overhead table | the overhead derivation is driven from the transform registry, so an unknown transform is an explicit refusal to advise rather than a default |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An operator applies an MTU derived from a wrong figure and clamps a working tunnel, or leaves an oversized one in place believing it checked. The command changes nothing itself, so the damage is through the advice it prints, which is why a wrong figure must be impossible rather than unlikely |
| How is it reverted? | Single commit revert for the module. The `PeerInfo` changes and the negotiated-transform fix are separable and worth keeping regardless |
| Who else touches this path? | `plan/immediate/spec-rfc4301-architecture-gaps.md` phase 7 (the per-SA PMTU), `internal/core/probe` (the probe layer this depends on, landed by spec-probe-do-not-fragment), `plan/spec-ike-padded-path-probe.md` (the later, more accurate prober) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show mtu` from the CLI | → | the registered RPC handler | `TestShowMTUReachesTheHandler` |
| `show mtu host <address>` | → | the same handler with one target | `TestShowMTUHostMeasuresOneAddress` |
| The IKE engine's `init()` | → | the registered inventory snapshot | `TestIPsecInventoryRegisteredByIKE` |
| `show mtu \| json` | → | the payload, rendered | `TestShowMTUPayloadRendersAsJSON` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show mtu host <address>` against a path clamped below the interface MTU | the measured path MTU equals the clamp, and the payload names how it was found |
| AC-2 | A peer whose child SA negotiated AES-GCM-128 without UDP encapsulation | the derived ESP overhead is the outer IP header plus 8 ESP plus 8 IV plus 16 ICV, and the ceiling aligns down to the 4-octet AEAD boundary less the 2-octet trailer |
| AC-3 | The same peer with UDP encapsulation in use | the overhead is 8 octets greater and the ceiling 8 lower |
| AC-4 | A tunnel whose interface MTU exceeds its ceiling | the verdict is oversized, the payload names the excess in octets, and a command setting the recommended value appears in the remediation list |
| AC-5 | A tunnel whose interface MTU is within the safety margin of the ceiling | the verdict distinguishes "tight" from "oversized", and names the spare octets |
| AC-6 | A tunnel whose ceiling leaves no value at or above the inner family's minimum link MTU | no MTU is recommended, the payload says so explicitly, and no command for that interface appears in the remediation list |
| AC-7 | A peer that answers no probe at any size | that peer is reported unmeasurable and every other peer is still measured |
| AC-8 | A path where a router reports an MTU | the reported value is confirmed on the wire, meaning the size passes and one octet more fails, before it is reported as the answer |
| AC-9 | A path where ICMP is filtered entirely | the candidate ladder and then a bisection find the value, and the payload says the answer came from a search rather than from a report |
| AC-10 | A probe is lost but the path is not limited at that size | the bounds are not moved on a single silence; the size is retried to the configured probe count before it is believed |
| AC-11 | `exhaustive` is given | the kernel's cached PMTU is bypassed, and a deliberately poisoned cache value does not appear in the answer |
| AC-12 | The reference address answers with the full interface MTU while the peers measure lower | the payload says the access circuit is not clamped and that lowering the underlay interface would cost octets on every other destination |
| AC-13 | The reference address measures the same as the peers | the payload says the whole circuit is clamped and recommends the underlay change, naming that it matters for traffic sent outside a tunnel |
| AC-14 | No reference address answers | the underlay advice is reported as undecidable rather than guessed |
| AC-15 | A peer negotiated the second of two configured ESP proposals | `show vpn ipsec sa` reports the negotiated transform, not the first configured one, and the MTU arithmetic uses the negotiated one |
| AC-16 | A child SA in transport mode rather than tunnel mode | the overhead reflects the mode, and a verdict is never computed with tunnel-mode arithmetic for a transport-mode SA |
| AC-17 | The peer is behind NAT, so the installed endpoint differs from the configured address | probes are sent to the installed address |
| AC-18 | The IKE component is not present in the build | `show mtu` reports that no IPsec inventory is registered, and `show mtu host <address>` still works |
| AC-19 | `show mtu \| json`, `\| yaml` and `\| table` | all three render the same payload, and no verdict is carried as a marker glued to another field's text |
| AC-20 | The box has non-zero outbound fragmentation counters | the payload reports them as a finding, with the absolute values, not as a rate |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs `show mtu` on a CPE with two oversized tunnels | CLI → inventory snapshot → probes → per-tunnel arithmetic → payload with verdicts and commands | `test-show-mtu-oversized-tunnels` |
| 2 | runs `show mtu host 1.1.1.1` to tell a clamped circuit from a clamped peer path | CLI → probe layer → single measurement | `test-show-mtu-host` |
| 3 | runs `show mtu exhaustive` after a stale cached PMTU misled them | CLI → bypass mode → wire measurement | `test-show-mtu-exhaustive` |
| 4 | pipes the output into a ticket as JSON | CLI → payload → pipe operators | `test-show-mtu-json` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestESPOverheadPerTransform` | `internal/component/mtu/cmd/overhead_test.go` | a table over every negotiable ESP transform, against hand-computed overheads (A-2) | GREEN (2026-09-16, phases 3+5); RED with the AES-GCM IV cut to 16 in `espCipherWires`; also `TestESPOverheadTransportMode` |
| `TestESPOverheadUnknownTransformRefuses` | `internal/component/mtu/cmd/overhead_test.go` | an unrecognised transform yields a refusal to advise, never a default | GREEN (2026-09-16, phases 3+5): `errOverheadRefused` for a down tunnel, an unspecified mode, an ENCR id outside `espCipherWires`, CBC with AUTH_NONE |
| `TestCeilingAlignsAndSubtractsTrailer` | `internal/component/mtu/cmd/arith_test.go` | the ceiling lands on the cipher boundary | GREEN (2026-09-16, phases 3+5): 1446 for aes128gcm/IPv4, 1438 with UDP; RED twice on wrong hand computations in the test, corrected in the test; also `TestFamilyOf`, `TestMSSAbsentWhenNoRoom` |
| `TestRecommendedAbsentBelowMinimumLinkMTU` | `internal/component/mtu/cmd/arith_test.go` | no value is recommended below 1280 inner IPv6 | GREEN (2026-09-16, phases 3+5): 1280/1279 inner IPv6 and 576/575 inner IPv4 pinned (Boundary Tests) |
| `TestVerdictThresholds` | `internal/component/mtu/cmd/verdict_test.go` | each of the five verdicts at its exact boundary (A-5) | GREEN (2026-09-16, phases 3+5); RED with `>` cut to `>=` in `classifyTunnel`; also `TestVerdictZeroValuesNeverRender`, `TestRunVerdictLadder`, `TestUnderlayAdviceMatrix` |
| `TestVerdictOrderNoUsableMTUBeforeOversized` | `internal/component/mtu/cmd/verdict_test.go` | a tunnel with no usable value is never classified as merely oversized | GREEN (2026-09-16, phases 3+5) |
| `TestShowMTU*` (15 tests) | `internal/component/mtu/cmd/run_test.go` | the run through the registered handler over `fakeDeps`: AC-1, 2, 4, 5, 6, 7, 11, 12, 13, 14, 16, 17, 18, 19, 20, the DF gate failure, the detail view, down and unbound tunnels | GREEN (2026-09-16, phase 6); RED on `commands` nil rendering as JSON `null` and on `probeOutcome.String` missing, then GREEN. `TestShowMTUExhaustiveBypassesTheCache` renamed with the `exhaustive` keyword (phase 6b) |
| `TestDoctorMTULocalStateReportsEachUnreadableSource`, `TestDoctorMTULocalStateSilentWhenBothRead` | `internal/component/mtu/cmd/doctor_test.go` | the `mtu-local-state` check through `diagnostic.DoctorChecksForPhase`: one warning per unreadable source, the code in `builtinCodes`, silence when both read | GREEN (2026-09-16, phase 6b, `job-mtu6-doctor-b780a1b4.log`) |
| `TestResolveIfIDReadsTheBoundInterface` | `internal/component/ike/engine/inventory_test.go` | the `vti { bind }` fix: the bound interface's if_id, 0 for no binding, an error for a dangling binding or if_id 0 | GREEN (2026-09-16, phase 6); before the fix `SiteToSitePeer.IfID` was never assigned, so every Child SA installed with if_id 0 |
| `TestShowMTUArgDefsByName` | `internal/component/config/yang/command_test.go` | the three leaves of `show mtu` as the grammar loads them: `host` typed, `search` = exhaustive, `view` = detail | GREEN (2026-09-16, phase 6b) |
| `TestSearchDoesNotMoveBoundsOnSingleSilence` | `internal/component/mtu/cmd/search_test.go` | RFC 4821 Section 7.6.4 (AC-10) | GREEN (2026-09-16, phase 4). RED under two applied cuts, each restored: `probesPerSizeMax = 2` fails `probesPerSizeMax = 2, RFC 8899 Section 5.1.2 says MAX_PROBES is 3`; `s.fail(payload)` on a silence fails `two lost probes at 1400 moved a bound and contradicted the retry` with 45 probes for 26 (scratch `job-mtu-mut-a`, `job-mtu-mut-b`) |
| `TestSearchLadderThenBisect` | `internal/component/mtu/cmd/search_test.go` | candidates inside the bracket are tried before the number line | GREEN (2026-09-16, phase 4); RED before `search.go` existed (undefined symbols); `TestSearchFilteredPathLadderThenBisect` proves it against a Linux router with the ICMP errors dropped |
| `TestReportedMTUConfirmedOnTheWire` | `internal/component/mtu/cmd/search_test.go` | a reported value is accepted only when the size passes and one more fails | GREEN (2026-09-16, phase 4): confirmed, over-reported, under-reported and the IPv6 overhead; `TestSearchClampedPathViaICMP` proves it against a Linux router. Also GREEN: `TestSearchUnmeasurable` (AC-7, 21 probes), `TestSearchForceDiscardsTheReport` (AC-11), `TestSearchZeroReportIsTheSearchSignal`, `TestSearchFollowIsCapped` (follows 6), `TestSearchContradictionIsRetestedAndNotedLossy`, `TestSearchBudgetIsTheComputedWorstCase` (108), `TestSearchDFGateFailed`, `TestSearchMethodNames`, and root-native `TestSearchForceBypassesPoisonedCache` |
| `TestInventorySnapshotUnregisteredIsNotEmpty` | `internal/core/ipsecinventory/registry_test.go` | an unregistered inventory is distinguishable from zero tunnels | GREEN (2026-09-16, phase 2, `go test -race -count=1 ./internal/core/ipsecinventory/...` ok) |
| `TestPeerInfoReportsNegotiatedTransform` | `internal/component/ike/engine/reconcile_test.go` | AC-15, the defect fix | GREEN (2026-09-16, phase 2); RED before the fix with `esp encryption = "aes256", want the negotiated "aes128gcm"`; `go test -race -count=1 ./internal/component/ike/engine/...` ok |
| `TestSAToMapChildCarriesInstalledFacts`, `TestSAToMapChildWithoutEndpointAnswersNull` | `internal/component/ike/cmd/show_ipsec_installed_test.go` | the `child-sa` object carries `mode`, `udp-encapsulation` and `remote-address` (null for no endpoint) beside the negotiated transform (AC-15, payload side) | GREEN (2026-09-16, phase 2); written after the code, so the discriminating red is the engine test and the interop cut |
| `TestIPsecInventoryRegisteredByIKE` | `internal/component/ike/engine/inventory_test.go` | the Wiring row: linking the engine registers the inventory, and the snapshot carries the negotiated transform and the installed facts | GREEN (2026-09-16, phase 2) |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| measured path MTU | 68-65535 | 68 | 67 | 65536 |
| recommended tunnel MTU, inner IPv6 | 1280-65535 | 1280 | 1279 | N/A |
| recommended tunnel MTU, inner IPv4 | 576-65535 | 576 | 575 | N/A |
| probes per size | 1-3 | 3 | 0 | 4 |
| reported-MTU follows | 0-6 | 6 | N/A | 7 |

Confirmed (2026-09-16): `payloadMin` 68 and `payloadMax` 65000 bound the search (`search.go`), `TestSearchBudgetIsTheComputedWorstCase` pins the 108-probe budget; the 1280 and 576 floors with their nearest producible neighbors are `TestRecommendedAbsentBelowMinimumLinkMTU` (a 2-octet block, since neither real cipher block lands on the floor); `probesPerSizeMax` 3 and `reportedFollowsMax` 6 are constants pinned by `TestSearchDoesNotMoveBoundsOnSingleSilence` (RED at 2) and `TestSearchFollowIsCapped`; `TestVerdictThresholds` pins each verdict boundary; a measured path above 65535 is a `BUG` panic in `run.go` because `payloadMax` bounds the search. An interface MTU outside 1..65535 (underlay and xfrm) is refused with a note in `run.go`, and NO unit test drives that arm: `iface` never reports such a value, and the arm is reported here rather than covered.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-mtu-oversized-tunnels` | `test/plugin/show-mtu-oversized-tunnels.ci` (fixture `plugin/show-mtu-oversized`, `internal/test/fixture/plugin_fixture_show_mtu.go`) | an operator is told which tunnels are oversized and by how much: two bound aes128gcm tunnels over a 1400 path, ceiling 1346, recommended 1314, excess 154, `set interface xfrm xa mtu 1314` and `xb`, `set interface ethernet sr0 mtu 1400`, verdict action-needed (user story 1, AC-2, 4, 13, 15, 17) | PASS (2026-09-16, phase 6, native root, `ci-mtu6-green.log`; re-run after the rename, `ci-mtu6c-run.log`, 4.3s). RED under the `searchUnmeasurable` cut: "both tunnels were not sized within the poll" (`ci-mtu6-red.log`) |
| `show-mtu-host` | `test/plugin/show-mtu-host.ci` (fixture `plugin/show-mtu-host`) | a single address is measured: 1400 via ICMP through sender 1600 / router / far 1400, four probes, underlay `sr0`, detail rows (AC-1) | PASS (2026-09-16, `ci-mtu6-green.log`, `ci-mtu6c-run.log`). RED under the cut: "status nothing-measured, want ok" |
| `show-mtu-exhaustive` | `test/plugin/show-mtu-exhaustive.ci` (fixture `plugin/show-mtu-exhaustive`; renamed from `show-mtu-force-bypasses-cache.ci` with a plain `mv` in phase 6b) | a poisoned cache (route mtu 1300) does not reach the answer: 1300 first, then `exhaustive` measures 1400 by "forced full search", `cached-path-mtu` 1300 and the stale note (AC-11, user story 3) | PASS (2026-09-16, `ci-mtu6-green.log` under the old name, `ci-mtu6c-run.log` under the new one). RED under the cut: "path-mtu 0, want 1300" |
| `show-mtu-json` | `test/plugin/show-mtu-json.ci` (fixture `plugin/show-mtu-json`) | the payload's document shape through `\| json`, kebab-case keys; the three renderings are `TestShowMTUEveryPipeRendersOnePayload` (AC-19) | PASS (2026-09-16, `ci-mtu6-green.log`, `ci-mtu6c-run.log`). RED under the `payload["Commands"]` cut: `key "Commands" is not kebab-case` (`ci-mtu6-red2.log`) |
| `show-mtu-no-ipsec-component` | `test/plugin/show-mtu-no-ipsec-component.ci` (fixture `plugin/show-mtu-no-tunnels`) | the registered-no-tunnels arm: `inventory registered`, verdict no-tunnels, never a zero (AC-18). The not-registered arm is `TestShowMTUNoInventoryIsNotZeroTunnels`: the runner's `ze` links every component | PASS (2026-09-16, `ci-mtu6-green.log`, `ci-mtu6c-run.log`). RED under the `inventoryNotRegistered` cut: "inventory not-registered, want registered" (`ci-mtu6-red.log`) |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `mtu-tunnel-sizing-strongswan` | `test/interop-ipsec/scenarios/` (checker `checkMTUTunnelSizingStrongSwan`, `internal/le/interoplab/ipsec/checkers.go`) | strongSwan | a tunnel to a real peer over a clamped path is measured, and the recommended MTU carries traffic where the current one fragments: the NAT box clamps the forwarded route to 1400, `show mtu \| json` reports the peer at 1400, the reference 172.28.0.5 at 1500, the tunnel oversized over aes256gcm behind UDP encapsulation (ceiling 1338, recommended 1306, excess 162, `set interface xfrm xa mtu 1306`), underlay not-clamped; `ping -M do` at inner 1400 loses 100%, at inner 1306 is lossless (AC-1, 3, 4, 12, user story 1; the derived-overhead half of `mtu-negotiated-transform`) | GREEN (2026-09-16, phase 6b): `IPSEC_INTEROP_SCENARIO=mtu-tunnel-sizing-strongswan ./le integration interop-ipsec` exit 0, `integration: 1 action(s) passed.` (`interop-sizing-green3.log`, `interop-sizing-green-final.log`). RED twice: inventory cut `t.InstalledRemote = info.ChildLocalAddr` -> `✗ FAIL: show mtu carries no peer measurement of 172.28.0.7` (`interop-sizing-red.log`); run cut `t.UDPEncap = false` -> `✗ FAIL: tunnel encapsulation false, want true across the NAT` with ceiling 1346 (`interop-sizing-red2.log`). The lab image gained `iputils` (`test/interop-ipsec/Dockerfile.ze`) for `ping -M do` |
| `mtu-negotiated-transform` | `test/interop-ipsec/scenarios/` | strongSwan | a peer that negotiates the second configured proposal is reported as running it, and the derived overhead follows (A-1, AC-15) | reported-transform half GREEN and discriminating (2026-09-16, phase 2): `IPSEC_INTEROP_SCENARIO=mtu-negotiated-transform ./le integration interop-ipsec` exit 0, `integration: 1 action(s) passed.`; with `Info` cut back to `ps.espGroup` the same run went RED: `✗ FAIL: ze reports the Child SA esp-encryption as "aes256gcm", want the accepted second proposal "aes128gcm"` / `FAIL 0 passed, 1 failed: mtu-negotiated-transform`. The derived-overhead half is owed by the phase that lands `show mtu` |
| `mtu-nat-installed-endpoint` | `test/interop-ipsec/scenarios/` (checker `checkMTUNATInstalledEndpoint`) | strongSwan behind NAT | probes go to the installed endpoint (A-3, AC-17): after `assertNATVerdict`, the `show vpn ipsec sa` child-sa `remote-address`, the peer measurement's target and the tunnel row's `remote` all equal 172.28.0.7, the NAT box's face where the ESP arrives, and never 172.28.0.3 where none does. It cannot discriminate installed from configured, because the two are equal in Ze today (A-3); it discriminates "the probe targets the installed endpoint at all" | GREEN (2026-09-16, phase 6b, `interop-nat-green1.log`, exit 0). RED with the inventory cut (`interop-nat-red.log`: `show mtu carries no peer measurement of 172.28.0.7`, target 172.28.0.2) |

## Files to Modify
- `internal/component/ike/engine/reconcile.go` - `PeerInfo` gains the encapsulation flag, the mode and the installed remote address, and reads the negotiated proposal rather than the configured group
- `internal/component/ike/engine/register.go` - registers the inventory snapshot at `init()`
- `internal/component/ike/cmd/show_ipsec.go` - the payload carries the corrected transform and the new fields
- `internal/component/sysctl/backend_linux.go` - an exported read, so the key-to-path mapping keeps one declaration
- `internal/component/plugin/all/all.go` - generated; regenerate with `./le repository generate`
- `docs/architecture/ike/ipsec-3-data-model.md` - the child SA's published state gains three fields
- `docs/architecture/ike/ipsec-7-ikev2-engine.md` - declared by the `// Design:` header of `internal/component/ike/engine/reconcile.go` and `register.go`. The engine gains a registered inventory, and the snapshot it publishes changes what it reports about a negotiated proposal
- `docs/architecture/ike/ipsec-10-cli-diag.md` - declared by the `// Design:` header of `internal/component/ike/cmd/show_ipsec.go`. The `show vpn ipsec sa` payload gains three fields and corrects the transform it reports
- `docs/guide/command-catalogue.md` - the `MTU discover / path MTU` row: fill the Ze cell, and drop the `shell` backend and the `ping -M` note, both of which this spec makes false
- `docs/guide/command-reference.md` - the new command
- `docs/features.md` - a new user-facing feature
- `plan/journal/constant-reported-as-measured-state.md` - the 2026-09-11 row is resolved by AC-15

## Files to Create
- `internal/core/ipsecinventory/registry.go` - `Register` and a snapshot query returning an explicit not-registered outcome
- `internal/component/mtu/cmd/register.go` - the RPC and local-meta registration, copying `internal/component/ping/cmd/register.go`
- `internal/component/mtu/cmd/mtu.go` - the handler and the run
- `internal/component/mtu/cmd/overhead.go` - the per-transform ESP overhead derivation
- `internal/component/mtu/cmd/arith.go` - the ceiling, the recommended value and the MSS
- `internal/component/mtu/cmd/search.go` - the confirm-follow-ladder-bisect search
- `internal/component/mtu/cmd/verdict.go` - the tunnel classification
- `internal/component/mtu/cmd/state.go` - the cached PMTU, the fragmentation counters and `tcp_mtu_probing`
- `internal/plugins/mtu-cmd/yang/ze-mtu-cmd.yang` - the command grammar
- `internal/plugins/mtu-cmd/yang/register.go` and `embed.go` - generated
- `docs/architecture/diagnostics/path-mtu.md` - the owning page; no page covers MTU, PMTU or fragmentation today
- `test/interop/scenarios/mtu-tunnel-sizing-strongswan/`, `mtu-negotiated-transform/`, `mtu-nat-installed-endpoint/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/mtu-cmd/yang/ze-mtu-cmd.yang` for the command; a config leaf for the reference address |
| YANG validation constraints | Yes | the host argument is an `inet:ip-address` from `ze-types.yang`, so a malformed address is refused by the grammar |
| YANG custom validators | N-A | native types cover the address and the two bare keywords |
| CLI commands/flags | Yes | `internal/component/mtu/cmd/mtu.go`, registered, not added to a central list |
| CLI grammar (keyword before value) | Yes | `show mtu host <address>`; `exhaustive` and `detail` are enum values typed bare, the `show system goroutines full` convention, under the leaves `search` and `view` |
| Editor autocomplete | Yes | automatic for the typed leaf; the host argument takes no dynamic completion because an arbitrary address is legal |
| Functional test for new RPC/API | Yes | five `.ci` scenarios listed above |
| Pipe completeness | Yes | one payload through `ApplyPipes`, proven by AC-19 |
| Env var registration | Yes | the reference-address leaf needs its `ze.mtu.<leaf>` registration via `env.MustRegister()` |
| Doctor check for runtime dependencies | Yes | DONE (2026-09-16, phase 6b): check `mtu-local-state` in `internal/component/mtu/cmd/doctor.go`, registered by `register.go` (`diagnostic.RegisterDoctorCheck`, PreConfig, order 770), code `doctor-mtu-local-state` in `builtinCodes` (`internal/core/diagnostic/codes.go`), tests `TestDoctorMTULocalStateReportsEachUnreadableSource` and `TestDoctorMTULocalStateSilentWhenBothRead`, row in `docs/guide/status.md`, section "The local-state dependency check" in `docs/architecture/diagnostics/path-mtu.md`. Design: this module reads `/proc/sys` and the SNMP counters, which are runtime dependencies: a check in the owning package plus a code in `internal/core/diagnostic/codes.go`. The ICMP socket dependency is covered by `checkICMPProbeSocket` (`internal/core/probe/doctor.go`, codes `doctor-icmp-probe` and `doctor-icmp-probe-unprivileged`) and is not duplicated here |
| Prometheus counters/metrics | N-A | the command is operator-invoked and holds no continuous state; the fragmentation counters it reports are already collected by telemetry |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP surface is touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`: DONE (phase 6), row "Path MTU diagnostic for IPsec tunnels" beside the Don't Fragment probes row, anchored to `run.go` and `search.go` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` for the reference-address leaf: DONE, the `environment { mtu { reference-address } }` example and the paragraph on the leaf and its `ze.mtu.reference-address` env override, anchored to `mtu.go` `referenceAddress` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`: DONE (phases 6, 6b), `### show mtu` with the grammar under `exhaustive` and a derived JSON example. `docs/guide/command-catalogue.md`: DONE (phase 6b), the "MTU discover / path MTU" row is `shipped`, `process`, in-daemon probes; the `shell` backend and the `ping -M` note are gone; `../wiki/command-catalog.md` regenerated |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`: DONE (phase 6), the `ze-show:mtu` row naming `handleShowMTU` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, `docs/plugin-overview.md`, `docs/features/plugins.md`: unaffected, read 2026-09-16: none of the three enumerates a `-cmd` YANG plugin (no `ping-cmd`, `traceroute-cmd`, `resolve-cmd` or `pki-cmd` row exists to add `mtu-cmd` beside); the plugin is discovered through `plugin/all/all.go`, which `docs/DESIGN.md` and `docs/architecture/command-ownership.md` describe generically |
| 6 | Has a user guide page? | Yes | `docs/architecture/diagnostics/path-mtu.md`: DONE, created in phase 1 and grown by every phase (the grammar, the run, the payload, the reference address, the search, the ESP overhead, the arithmetic, the verdicts, the underlay advice, the local state, "The local-state dependency check", the tests); read whole against the code on 2026-09-16 for the `exhaustive` rename and the payload keys, one stale `cache` leaf name corrected to `search`. `docs/guide/ipsec.md`: DONE (phase 6b), the `vti { bind }` paragraph after the transport-mode selectors links the page |
| 7 | Wire format changed? | N-A | no message format changes; the probes are ICMP echo built by the existing primitive |
| 8 | Plugin SDK/protocol changed? | N-A | no SDK surface changes |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | unaffected in the end: the phase 7 PMTU row of `rfc/short/rfc4301.md` did not move (phase 7 of `spec-rfc4301-architecture-gaps` has not landed, and this module only reads), and no RFC was enrolled, per the owner decision of 2026-09-11 recorded in `docs/architecture/diagnostics/active-probes.md`. The RFC 4821, 8899, 8200 and 791 quotations sit above the enforcing code in `search.go` and `arith.go` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`: DONE (phases 6, 6b), the paragraph on the five `show mtu` tests after the ping-df one. `docs/architecture/testing/interop.md`: DONE (2026-09-16, this phase), "The IPsec NAT box" gains the paragraph on the two `mtu-*` scenarios and the `iputils` ping, anchored to `checkers.go` and `Dockerfile.ze` |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`: DONE (phase 6), the row "IPsec tunnel MTU sizing from a live path measurement" |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`: DONE, the `ipsecinventory` leaf paragraph under section 19 (phase 2), the `sysctl.Read` paragraph under "Sysctl" (phases 3+5), the `ike` row's inventory clause (phase 2), and the `mtu` Component Boundaries row (2026-09-16, this phase) listing its imports as `internal/component/mtu/cmd` and the two YANG packages hold them |
| 13 | Route metadata keys added/changed? | N-A | no route metadata |
| 14 | Prometheus counters added/changed? | N-A | none added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/guide/status.md`: DONE (2026-09-16, this phase), the "MTU local-state check" row beside the ICMP probe socket one, anchored to `doctor.go`. `docs/plugin-overview.md`, `docs/features/plugins.md`: unaffected, see row 5 |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED, `./le spec citation anchors spec plan/spec-path-mtu-diagnostic.md` (2026-09-16, `scratch/anchors-p6e.log`) names eight pages, each read at its anchored paragraph: `docs/DESIGN.md` (all.go blank imports, `register.go` as the ike plugin: unaffected), `docs/architecture/api/process-protocol.md` (`OnAllPluginsReady` phase order: unaffected), `docs/architecture/command-ownership.md` (all.go wires `init()`: unaffected), `docs/architecture/ike/ipsec-13-rekey-wire.md` (`routeInbound`, `dispatchInbound`: unaffected), `docs/architecture/ike/ipsec-14-responder.md` (`matchResponderPeer`, `setSA`, `getSA`: unaffected), `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` (repaired in phase 6b for `resolveIfID`; its "createFirstChildSA takes RemoteAddr from the operator's remote-address" sentence agrees with A-3), `docs/architecture/ike/ipsec-dataplane-inspection.md` (`PeerSession.Info` derives the identity from the Child SA endpoints: still what `Info` does; unaffected), `docs/features/interfaces.md` (per-interface sysctl writes in `backend_linux.go`: the exported `Read` landed in `backend.go`, unaffected) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ipsec.md`: read 2026-09-16, its only MTU claims are the `vti { bind }` paragraph phase 6b wrote (the interface's MTU is the tunnel's, `show mtu` reports whether it fits the measured path); it carries no figure the command contradicts |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- `show mtu` reaches a handler
   - Tests: `TestShowMTUReachesTheHandler`, `TestShowMTUPayloadRendersAsJSON`, `TestIPsecInventoryRegisteredByIKE`
   - Files: `ze-mtu-cmd.yang`, `internal/component/mtu/cmd/register.go`, a stub handler, `internal/core/ipsecinventory/registry.go`, and the regenerated composition root
   - Verify: the command exists and returns a stub payload; the wiring test fails on content, not on reachability
2. **Phase: the IKE boundary and the negotiated-transform fix**
   - Tests: `TestPeerInfoReportsNegotiatedTransform`, `TestInventorySnapshotUnregisteredIsNotEmpty`, the `mtu-negotiated-transform` scenario
   - Files: `reconcile.go`, `register.go`, `show_ipsec.go`
   - Verify: A-1 is validated by negotiating the second of two proposals, not asserted. This phase stands alone and is committed on its own: it repairs a defect `show vpn ipsec sa` has today
3. **Phase: the arithmetic**
   - Tests: `TestESPOverheadPerTransform`, `TestESPOverheadUnknownTransformRefuses`, `TestCeilingAlignsAndSubtractsTrailer`, `TestRecommendedAbsentBelowMinimumLinkMTU`
   - Files: `overhead.go`, `arith.go`
   - Verify: A-2 is validated by the table over every negotiable transform
4. **Phase: the search**
   - Tests: `TestSearchLadderThenBisect`, `TestReportedMTUConfirmedOnTheWire`, `TestSearchDoesNotMoveBoundsOnSingleSilence`
   - Files: `search.go`
   - Verify: the RFC 4821 congestion rule and the RFC 8899 probe count are each pinned by a test that fails without them
5. **Phase: verdicts and local state**
   - Tests: `TestVerdictThresholds`, `TestVerdictOrderNoUsableMTUBeforeOversized`
   - Files: `verdict.go`, `state.go`, the exported sysctl read
   - Verify: each threshold is pinned at its exact boundary
6. **Phase: the payload, the output and the pages**
   - Tests: the five `.ci` scenarios, `mtu-tunnel-sizing-strongswan`, `mtu-nat-installed-endpoint`
   - Files: `mtu.go`, `docs/architecture/diagnostics/path-mtu.md`, and every row named at Documentation row 16
   - Verify: A-3 and A-4 are validated; the command-catalogue row no longer says `shell`

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The overhead is derived from the negotiated transform in every path, including the rekeyed one. No arithmetic reads a configured value where an installed one exists |
| Naming | Payload keys are kebab-case. The verdict is a field with a typed value, never a marker glued to a name |
| Data flow | The mtu module imports neither the IKE engine nor the iface backend. Every cross-component read goes through a registry or a dispatch call |
| Rule: `ai/rules/principles.md` | An unregistered inventory, a peer with no tunnels, and a peer whose tunnel is down are three outcomes with three names. No zero is an answer |
| Rule: `ai/rules/evidence.md` | Every figure the payload calls measured was measured. A value assumed from another peer's path is marked as assumed wherever it appears |
| Rule: `ai/rules/cli.md` | `| json`, `| yaml` and `| table` render the same payload, and the remediation commands are a field rather than pre-formatted text |

## Review Gate

Run inline by the closure context (2026-09-16), which wrote none of the code under review. Round scopes were written before each round ran.

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/path-mtu-diagnostic-e9e5e97a-9494-4074-a088-c3bd059b2fb3.md` (89 files, verdict=clean) |
| `review check` | `review_gate: OK (56 code files, clean, hashes match)` |
| Rounds | 2 |
| Reviewer lenses used | round 1: logic and wiring (every producer of the payload read: `runMTU`, `searchPathMTU`, `attempt`, `refine`, `confirm`, `deriveESPOverhead`, `ceiling`, `recommended`, `classifyTunnel`, `adviseUnderlay`, `resolveIfID`, `tunnelOf`, `ParseEchoReply`, `SizeRefusalOf`, `sysctl.Read`); security, allocation and the six-question style pass of `docs/contributing/ze-go-style.md` over every changed Go file. Round 2: the two fixes and their sibling call sites only |

### Run 1 (scope: the whole uncommitted diff, both lenses)
| Severity | Finding | File | Resolution |
|----------|---------|------|------------|
| ISSUE | `underlayCommand` spelled every underlay as `set interface ethernet <name>`, whatever list of the iface YANG the link belongs to. The functional tests' underlay `sr0` is a veth, and a bridge or a dummy underlay would have been handed a command that creates an `ethernet` block of that name instead of sizing the link. The advice is the product's only output (Blast Radius), so a wrong command is a wrong result | `internal/component/mtu/cmd/verdict.go` `underlayCommand`; `run.go` `readUnderlay` | fixed in round 1: `readUnderlay` reads the kind through `iface.CanonicalInterfaceType` (`discover.go`, the one mapping from a link type to a YANG list) into `underlayLink`, the underlay row carries `kind` when the schema holds a list for it, `underlayInput.kind` spells the command, and a kind the schema lacks (or the loopback) earns the value in the note and no command, the note saying why (`noCommandClause`, `kindConfigurable`). Tests: `TestUnderlayAdviceMatrix` (veth, bridge, unknown, loopback rows and the two note assertions), `TestShowMTUUnderlayCommandNamesTheLinkKind` through the handler, `TestShowMTUUnderlayAdvice` asserts `kind`; the fixture asserts `set interface veth sr0 mtu 1400`. RED with the fix reverted (`scratch/close-red.log`: `commands [set interface ethernet sr0 mtu 1400], want "set interface veth sr0 mtu 1400"`), GREEN restored (`close-green.log`); the five `.ci` rebuilt and 5/5 PASS natively as root with the veth spelling (`close-fx-run.log`) |
| ISSUE | `measure` dropped every error of `kernelPathMTU`, so a netlink failure read as "the kernel holds no entry" and the AC-11 cache note was silently absent (ze-go-style "Every error is handled") | `internal/component/mtu/cmd/run.go` `measure` | fixed in round 1: `probe.ErrPathMTUUnknown` (the no-entry answer, `errqueue_linux.go` `KernelPathMTU`) stays silent; any other error is a caution note naming the target and the error. Test: `TestShowMTUCacheReadFailureIsNoted`, RED under the inverted arm (`close-red.log`), GREEN restored |
| NOTE | `tunnelOf` leaves `TSLocal`/`TSRemote` invalid on any parse error, not only on the empty string; the text is the child's `*net.IPNet` String, canonical for a CIDR selector, so no reachable input takes the second path | `internal/component/ike/engine/inventory.go` `tunnelOf` | recorded, no change |
| NOTE | `wireProber.probe` honors a context deadline and not a bare cancellation; a caller that goes away without a deadline is served the bounded worst case (`runProbeBudget`, 216 s per target) | `internal/component/mtu/cmd/search.go` `probe` | recorded, no change: the bound is stated and computable (R-2) |
| NOTE | `plan/spec-ike-padded-path-probe.md` and `arith.go` cited this spec by path; commit B removes the file | `plan/spec-ike-padded-path-probe.md`, `internal/component/mtu/cmd/arith.go` | repointed at `docs/architecture/diagnostics/path-mtu.md` in commit A; `Depends` cleared |

Style pass: no `panic()` is reachable from a socket (every one is `BUG:` on a state only a Ze defect produces; the network's reported MTU is bounded in `confirm` and the prober's outcome enum is Ze's own). Every loop is bounded by a named constant (`runProbeBudget` is their sum) or by the interface count. Names carry the value, not the type. `openWireProber`/`close` state MUST on both sides. No duplicated fact: the ESP IV and block are declared once (`espCipherWires`), the ICV read from the crypto package, the kind read from `iface`. Return types are the narrowest used. Security: the host argument is a `netip.Addr` twice-checked (grammar and `parseMTUArgs`), a dash-leading value is refused by name, every allocation is fixed (`payloadMax`) or bounded by the tunnel and note counts, and no path from `handleShowMTU` reaches a configuration write (`grep` over `internal/component/mtu/cmd` finds no config, tx or apply call).

### Run 2 (scope: the two round-1 fixes, `verdict.go` `underlayCommand`/`kindConfigurable`/`noCommandClause`/`underlayInput.kind`, `run.go` `readUnderlay`/`underlayLink`/`adviseUnderlay`/`measure`, their tests, the fixture assertion, the four pages; sibling call sites `tunnelCommand` and `interfaceMTUCommand`)
| Severity | Finding | File | Resolution |
|----------|---------|------|------------|
| none | `kindConfigurable` is the one test both the command and the note apply; `iface.CanonicalInterfaceType` never answers `macvlan` (`infoToZeType`, `discover.go`), so no kind outside the YANG lists reaches `interfaceMTUCommand`; `tunnelCommand` keeps `xfrm` by construction (the tunnel is found by `Type == "xfrm"`); `errors.Is` reaches `ErrPathMTUUnknown` through the `%w` wrap in `KernelPathMTU`; the `kind` key is kebab-case and documented | - | 0 BLOCKER, 0 ISSUE, 0 NOTE inside the scope |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | the underlay command named `ethernet` for every link kind | `verdict.go` `underlayCommand` | `iface.CanonicalInterfaceType` read in `readUnderlay`, carried as `underlayInput.kind`, no command for a kind the schema lacks |
| 2 | ISSUE | a failed kernel cache read was silent | `run.go` `measure` | `ErrPathMTUUnknown` silent, any other error a caution note |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The module is removable | dropping its blank import from the composition root still builds and `show mtu` disappears |
| No import of the IKE engine | `grep -rn "ike/engine" internal/component/mtu/` returns nothing |
| The transform is negotiated, not configured | the `mtu-negotiated-transform` interop scenario goes red when the fix is reverted |
| The catalogue row is true | `grep -n "path MTU" docs/guide/command-catalogue.md` shows the Ze cell filled and no `shell` backend |
| Every verdict boundary is pinned | `go test -run TestVerdictThresholds ./internal/component/mtu/...` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | The host argument is operator-supplied and reaches a socket; it is a typed address from the grammar, never a string passed onward |
| Untrusted input | A reported next-hop MTU comes from the network and drives printed advice. It is bounded by family and confirmed on the wire before it reaches a recommendation |
| Resource exhaustion | The probe budget, the follow count and the ladder are each bounded, so a hostile path cannot extend a run indefinitely |
| Error leakage | The payload names peer addresses and interface names, which an operator already sees; it carries no key material and no SPI beyond what `show vpn ipsec sa` already publishes |
| Authorization failing open | The command is read-only by construction. It must hold no code path that writes configuration, and the review checks that no write helper is reachable from the handler |

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

**A-3 was written from the design intent and not from the producer (Mistake Log, wrong assumption).** The spec asserted that `ChildSA.RemoteAddr` differs from the configured address behind NAT because "the peer endpoint is adopted after authentication". Reading the producer in phase 6 showed the adoption moves `sa.peerEndpoint` only (`adoptAuthenticatedEndpoint`, `ike/engine/sa.go`), and both callers of `createFirstChildSA` hand it the configured `peer.RemoteAddress`; no Child SA reads the adopted endpoint. So the installed and the configured remote are one value in every reachable path today. The design survives because it was stated in terms of the SA the kernel holds, not in terms of the engine's internals: `tunnelTarget` (`run.go`) probes `InstalledRemote`, which is the address ESP is carried to, so the day the engine propagates the authenticated endpoint the probe follows without an edit here. What the wrong assumption cost: `mtu-nat-installed-endpoint` cannot discriminate installed from configured, only "the installed endpoint is what is probed", and the differing case is proven by a unit test over a fake inventory. The lesson is the one `ai/rules/evidence.md` states: an assumption's Basis cell names a producer, and the producer is read before the row is written, not at validation time. The engine find is one row in `plan/journal/one-state-held-in-two-fields.md`.

**The vti binding was never applied (design insight, found on the way).** `SiteToSitePeer.IfID` had no writer, so every `vti { bind }` peer installed its Child SA with if_id 0 and no `show mtu` could find an interface to size. The fix (`resolveIfID`, `established.go`) was in scope because user story 1 cannot run without it, and it changes engine behavior for every bound peer: a binding to no interface now fails the child rather than installing unbound. The pages and the journal row (`silent-fall-through.md`) carry it.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Its own component plus YANG plugin, taking IPsec state through a registered snapshot | extend `internal/component/ike/cmd/`, which is where the state already is | the owner requires a removable module (2026-09-11). The IKE placement was simpler and would have made the feature inseparable from the IKE component |
| A registered synchronous snapshot in a core leaf | subscribe to the existing `child-up` and `child-down` events | the bus has no replay, so a module started after a tunnel came up would report no tunnels: a silently-wrong zero, which `ai/rules/principles.md` bans outright |
| The overhead is derived from the negotiated transform | hardcode AES-GCM-128 as the Python does, and refuse to advise otherwise | Ze is the IKE daemon and knows what was negotiated. The refusal path exists in the Python only because it cannot see it, and removing that limitation is most of the value of porting rather than copying |
| The 31-value candidate ladder is kept | RFC 1191's eleven plateaus, or ladder-then-bisect | the ladder was tuned on real degraded customer paths, and RFC 1191 Section 7 calls its own table "an implementation suggestion, NOT a specification or requirement". The RFC's eleven values step from 1492 straight to 1006 and would land far below the truth on the clamped paths this tool exists for. Owner decision, 2026-09-11 |
| The module reads the engine's per-SA PMTU and never writes it | keep an independent value, or write measurements back | writing back breaks the read-only promise that makes the command safe on live customer equipment. Keeping a second value means the daemon and the diagnostic can disagree with nothing reconciling them. Reading it makes disagreement itself a reportable finding. Owner decision, 2026-09-11 |
| ICMP echo now, the IKE-padded probe later | build both, or build only the IKE-padded probe | `show mtu host <address>` and the reference measurement both need ICMP regardless, so the IKE probe removes no prober. It is homed in `plan/spec-ike-padded-path-probe.md` so it is scheduled rather than forgotten. Owner decision, 2026-09-11 |

## Known Limitations

- The measurement is ICMP, so a path that treats UDP or ESP differently is not measured and the figures are optimistic. The payload says so. `plan/spec-ike-padded-path-probe.md` removes this for a peer with a live IKE SA.
- The configured interface MTU and the kernel one are reported separately because no symbol today reports both sides; reconciling them is `internal/component/iface`'s work, not this module's.
- A policy-based SA with no bound interface has nothing to size and is reported as such rather than measured.
- A `vti { bind }` peer in transport mode is refused by the engine (`traffic_selector.go`), so a transport-mode tunnel is sized by the transport arithmetic (AC-16) and never earns an `xfrm` interface command.
- The run answers ONE payload when every target has been measured; it is not streamed. The RPC contract is one Response per command, and per-peer progress would be a `monitor mtu` verb this spec does not add (R-2: the probe budget per target is bounded by `runProbeBudget`, so the worst case is computable).
- AC-18 is proven in two halves: `show-mtu-no-ipsec-component.ci` proves the registered-no-tunnels arm, and `TestShowMTUNoInventoryIsNotZeroTunnels` the not-registered arm, because the functional runner links one `ze` with every component and cannot build a daemon without the IKE engine.
- An interface MTU outside 1..65535 is refused with a note by the run, and no test drives that arm: `iface` never reports such a value.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

The enforcing code here is the search and the floors: RFC 4821 Section 7.6.4 above
the rule that a single silent probe does not move the bounds, RFC 8899 Section 5.1.3
above the probe count, RFC 8200 Section 5 above the 1280 floor, and RFC 4301
Section 8.2.2 above the read of the engine's aged per-SA PMTU.

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
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
  Unit: `internal/component/mtu/cmd/{mtu,run,search,overhead,arith,verdict,state,doctor}_test.go`, `search_integration_linux_test.go`, `state_linux_test.go`; `internal/core/ipsecinventory/registry_test.go`; `internal/component/ike/engine/{inventory,reconcile}_test.go`; `internal/component/ike/cmd/show_ipsec_installed_test.go`; `internal/component/sysctl/backend_linux_test.go`; `internal/plugins/mtu-cmd/yang/cmd_schema_test.go`; `internal/component/config/yang/command_test.go`. Functional: the five `test/plugin/show-mtu-*.ci`. Interop: three checkers in `internal/le/interoplab/ipsec/checkers.go`.
- [ ] Tests FAIL (paste output)
  Wiring (phase 1, before the run existed): `TestShowMTUHostMeasuresOneAddress` red until phase 6. Engine (phase 2): `esp encryption = "aes256", want the negotiated "aes128gcm"`. Search (phase 4, `job-mtu-mut-a`): `probesPerSizeMax = 2, RFC 8899 Section 5.1.2 says MAX_PROBES is 3`. Run (phase 6, `ci-mtu6-red.log`): `path-mtu 0, want 1300`, `inventory not-registered, want registered`, `status nothing-measured, want ok`, `both tunnels were not sized within the poll`; `ci-mtu6-red2.log`: `key "Commands" is not kebab-case`. Interop (phase 6b, `interop-sizing-red.log`): `✗ FAIL: show mtu carries no peer measurement of 172.28.0.7`; `interop-sizing-red2.log`: `✗ FAIL: tunnel encapsulation false, want true across the NAT`.
- [ ] Tests PASS (paste output)
  `go test -race -count=1 ./internal/component/mtu/cmd/ ./internal/plugins/mtu-cmd/... ./internal/test/fixture/` exit 0 (`job-mtu6-rename-e9a9d7b1.log`); `./internal/component/ike/...` 8 ok, 0 FAIL (`job-mtu6-ike-*.log`). Functional (`ci-mtu6c-run.log`, 2026-09-16, native root after the rename): `PASS 721 show-mtu-exhaustive`, `PASS 722 show-mtu-host`, `PASS 723 show-mtu-json`, `PASS 724 show-mtu-no-ipsec-component`, `PASS 725 show-mtu-oversized-tunnels`, `pass 5/5 100.0% 8.8s`. Interop: `integration: 1 action(s) passed.` for each of `mtu-negotiated-transform`, `mtu-tunnel-sizing-strongswan` (`interop-sizing-green-final.log`), `mtu-nat-installed-endpoint` (`interop-nat-green1.log`).
- [ ] Boundary tests for all numeric inputs
  The Boundary Tests confirmation above: every row pinned except the interface-MTU-out-of-range note, which no unit test drives.
- [ ] Functional `.ci` tests for end-to-end behavior
  Five, the Functional Tests table; each went RED under an applied cut and GREEN with it restored.
- [ ] Interop tests for protocol features (or N-A with a reason)
  Three, the Interop Tests table, each against strongSwan and each with a recorded RED.

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- `show mtu`, `show mtu host <address>`, `exhaustive`, `detail`: a new removable module (`internal/component/mtu/cmd`, `mtu/yang`, `internal/plugins/mtu-cmd/yang`) reached through `plugin/all`. The run (`run.go` `runMTU`) measures every peer target and the reference address with an in-daemon DF prober (`search.go` `searchPathMTU`: sanity gate, reported figure confirmed on the wire, the 31-value ladder, the number line, RFC 4821 no-move on a single silence, RFC 8899 MAX_PROBES 3), derives the ESP overhead per negotiated transform (`overhead.go` `deriveESPOverhead`), computes the ceiling, the recommended value and the MSS (`arith.go`), classifies each tunnel and the run (`verdict.go`), applies the underlay advice matrix, reads the local state (`state.go`: absolute fragmentation counters, `tcp_mtu_probing`, the cached path MTU) and answers one document rendered by every pipe.
- The IKE engine registers an inventory snapshot (`ike/engine/inventory.go`, `core/ipsecinventory`) carrying the negotiated transform, the installed endpoint, the mode, the encapsulation and the traffic selectors; `show vpn ipsec sa` reports the negotiated transform (`dce0c5d0a8`).
- The `vti { bind }` fix: `resolveIfID` (`established.go`) reads the bound interface's if_id; before it every bound peer installed unbound.
- `sysctl.Read`, `probe.ParseEchoReply`, `probe.SizeRefusalOf` (shared with the ping session), the `mtu-local-state` doctor check, the `environment { mtu { reference-address } }` leaf, the runtime kernel's `CONFIG_XFRM_INTERFACE` (`38d2ac4340`), the compiled clamped-path fixture (`57470fdba8`).

### Bugs Found/Fixed
- `SiteToSitePeer.IfID` had no writer, so every `vti { bind }` Child SA installed with if_id 0: `TestResolveIfIDReadsTheBoundInterface`, `show-mtu-oversized-tunnels.ci`.
- `show vpn ipsec sa` reported the configured ESP group, not the negotiated proposal: `TestPeerInfoReportsNegotiatedTransform`, `mtu-negotiated-transform`.
- The runtime kernel profile lacked `CONFIG_XFRM_INTERFACE`, so no xfrm interface could be created in the guest: `show-mtu-oversized-tunnels.ci` under QEMU (`job-xfrm-guest-6c9cbb72.log`).
- `commands` rendered as JSON `null` on a run with none: `TestShowMTUOversizedTunnelGetsACommand` and the `.ci` kebab check.
- Closure review: the underlay command named `ethernet` for every link kind (`TestShowMTUUnderlayCommandNamesTheLinkKind`); a failed kernel cache read was silent (`TestShowMTUCacheReadFailureIsNoted`).

### Documentation Updates
- `docs/architecture/diagnostics/path-mtu.md` (new, the owning page; anchors to every producer), `docs/guide/command-reference.md` `### show mtu`, `docs/guide/command-catalogue.md` row 322, `docs/guide/configuration.md` (the reference-address leaf), `docs/architecture/api/commands.md` (`ze-show:mtu`), `docs/features.md`, `docs/comparison.md`, `docs/functional-tests.md`, `docs/architecture/testing/interop.md`, `docs/architecture/core-design.md` (the `mtu` row, `ipsecinventory`, `sysctl.Read`), `docs/architecture/diagnostics/active-probes.md`, `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` and `docs/guide/ipsec.md` (`vti { bind }` resolution), `docs/guide/status.md` (the doctor check), `ai/INSTRUCTIONS.md` (generated directory lists).
- `./le doc check verify` (`scratch/doc-verify-p6e.log`): red on one cause only, the `show mtu` command-equivalents surface that `./le site build` publishes into the sibling gh-pages checkout (outside this commit), plus the `../wiki/command-catalog.md` drift row read through the shared `bin/le`; both are named in the closure report.

### Deviations from Plan
- `force` became `exhaustive` (owner, 2026-09-16); `host <address>` is the only selector (owner).
- The interop scenarios live under `test/interop-ipsec/scenarios/`, not `test/interop/scenarios/`: that is the IPsec lab's directory.
- A-3 was wrong about the engine (Mistake Log); the design stands.
- The run answers one payload rather than per-peer progress (R-2): the RPC contract is one Response per command, and the budget is bounded instead.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-3 asserted `ChildSA.RemoteAddr` differs from the configured address behind NAT because the peer endpoint is adopted after authentication | `adoptAuthenticatedEndpoint` moves `sa.peerEndpoint` only, and both callers of `createFirstChildSA` pass the configured `peer.RemoteAddress`, so the two are one value in every reachable path | reading the producer in phase 6 while writing `mtu-nat-installed-endpoint` | the module probes `InstalledRemote` (the SA's own endpoint) so it follows the engine the day it changes; the engine find is one row in `plan/journal/one-state-held-in-two-fields.md`; the scenario's row says what it can and cannot discriminate |
| approach | the underlay remediation command was spelled `ethernet` for every link, copied from the VyOS tool where the underlay is always `interfaces ethernet` | Ze's iface YANG holds one list per link kind and `iface.CanonicalInterfaceType` already maps a link to its list | closure review, round 1 | the command is spelled with the kind and withheld for a kind the schema lacks |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Measure the underlay path MTU to every site-to-site peer and to a host | Done | `run.go` `runMTU`, `resolvePeerTargets`, `measure`; `search.go` `searchPathMTU` | `show-mtu-host.ci`, `mtu-tunnel-sizing-strongswan` |
| Derive the ESP ceiling per tunnel from the negotiated transform | Done | `overhead.go` `deriveESPOverhead`, `arith.go` `ceiling` | `TestESPOverheadPerTransform`, the interop scenario's ceiling 1338 over aes256gcm |
| Report oversized / tight / under-utilized / ok with the fixing commands | Done | `verdict.go` `classifyTunnel`, `tunnelCommand`; `run.go` `sizeTunnel` | `show-mtu-oversized-tunnels.ci` |
| Removable module taking IPsec state through a registered snapshot | Done | `core/ipsecinventory/registry.go`, `ike/engine/inventory.go`, `plugin/all/all.go` | `all.go` is the only importer; `grep ike/engine internal/component/mtu` is empty |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `show-mtu-host.ci`, `TestSearchClampedPathViaICMP`, `mtu-tunnel-sizing-strongswan` | `search.go` `searchPathMTU`, method in the payload |
| AC-2, AC-3 | Done | `TestESPOverheadPerTransform`, `TestCeilingAlignsAndSubtractsTrailer`, `TestShowMTUOversizedTunnelGetsACommand` | `overhead.go`, `arith.go` |
| AC-4, AC-5 | Done | `TestShowMTUOversizedTunnelGetsACommand`, `TestShowMTUTightAndOKTunnels`, `show-mtu-oversized-tunnels.ci` | `verdict.go` `classifyTunnel` |
| AC-6 | Done | `TestShowMTUNoUsableMTUHasNoCommand`, `TestRecommendedAbsentBelowMinimumLinkMTU` | `arith.go` `recommended` |
| AC-7 | Done | `TestSearchUnmeasurable`, `TestShowMTUUnmeasurablePeerIsAssumed` | `run.go` `measure` continues |
| AC-8 | Done | `TestReportedMTUConfirmedOnTheWire`, `TestSearchClampedPathViaICMP` | `search.go` `confirm` |
| AC-9 | Done | `TestSearchLadderThenBisect`, `TestSearchFilteredPathLadderThenBisect` | `search.go` `refine` |
| AC-10 | Done | `TestSearchDoesNotMoveBoundsOnSingleSilence` | `search.go` `attempt` |
| AC-11 | Done | `TestShowMTUExhaustiveBypassesTheCache`, `TestSearchExhaustiveDiscardsTheReport`, `show-mtu-exhaustive.ci` | `DFBypassCache` |
| AC-12, AC-13, AC-14 | Done | `TestShowMTUUnderlayAdvice`, `TestUnderlayAdviceMatrix`, the interop `not-clamped`, `show-mtu-oversized-tunnels.ci` `circuit-clamped` | `verdict.go` `adviseUnderlay` |
| AC-15 | Done | `TestPeerInfoReportsNegotiatedTransform`, `mtu-negotiated-transform` | `dce0c5d0a8` |
| AC-16 | Done | `TestShowMTUTransportModeUsesTransportArithmetic`, `TestESPOverheadTransportMode` | `arith.go` `ceiling` transport arm |
| AC-17 | Done | `TestShowMTUProbesTheInstalledEndpoint`, `mtu-nat-installed-endpoint` | `run.go` `tunnelTarget` |
| AC-18 | Done | `TestShowMTUNoInventoryIsNotZeroTunnels`, `show-mtu-no-ipsec-component.ci` | two halves, Known Limitations |
| AC-19 | Done | `TestShowMTUEveryPipeRendersOnePayload`, `show-mtu-json.ci` | `command.RegisterShape` |
| AC-20 | Done | `TestShowMTUFragmentationCountersAreAbsolute`, `TestFragmentationCountersAreAbsoluteWithSeverities` | `state.go` |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| every row of the Unit Tests table | Done | as named there | statuses filled per row |
| the five `.ci` | Done | `test/plugin/show-mtu-*.ci` | 5/5 PASS after the closure fix (`close-fx-run.log`) |
| the three interop scenarios | Done | `test/interop-ipsec/scenarios/mtu-*` | GREEN with recorded REDs |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| every file under Files to Modify and Files to Create | Done | `test/interop/scenarios/` became `test/interop-ipsec/scenarios/`; the `plan/journal/constant-reported-as-measured-state.md` row stands as a record |

### Audit Summary
- **Total items:** 4 requirements, 20 ACs, 3 test groups, the file lists
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** the scenario directory, the `exhaustive` word (Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The box measures what the path actually carries to each peer | interop | `mtu-tunnel-sizing-strongswan`: the NAT box clamps the forwarded route to 1400 and `show mtu` reports the strongSwan peer at 1400 and the reference at 1500; RED with the inventory cut (`interop-sizing-red.log`) |
| The ESP ceiling follows the transform the tunnel negotiated, not an assumption | interop, unit | the same scenario reports ceiling 1338 and recommended 1306 over the negotiated aes256gcm behind UDP, RED with `UDPEncap` cut (`interop-sizing-red2.log`); `mtu-negotiated-transform` RED with `Info` reading the configured group; `TestESPOverheadPerTransform` over every negotiable pair |
| The recommended value carries traffic where the current one fragments | interop | `mtu-tunnel-sizing-strongswan`: `ping -M do` at inner 1400 loses 100%, at inner 1306 is lossless |
| An operator gets the verdicts and the commands to apply | functional | `show-mtu-oversized-tunnels.ci`: two bound tunnels oversized by 154, `set interface xfrm xa mtu 1314` and `xb`, `set interface veth sr0 mtu 1400`, verdict action-needed (`close-fx-run.log`) |
| A stale cached PMTU cannot mislead the answer | functional | `show-mtu-exhaustive.ci`: a poisoned route MTU of 1300 is reported first, `exhaustive` measures 1400 and notes the stale cache |
| The module is removable and takes IPsec state through a registered snapshot | structural | `plugin/all/all.go` is the only importer of `component/mtu`; `TestIPsecInventoryRegisteredByIKE`; `TestShowMTUNoInventoryIsNotZeroTunnels` for the build without IKE |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none: every scoped item is implemented | the engine's child-SA endpoint (A-3) is a journal row in `plan/journal/one-state-held-in-two-fields.md`, not scope; the transport-mode `vti bind` refusal is a Known Limitation of the engine, not scope; the IKE-padded prober was homed before implementation in `plan/spec-ike-padded-path-probe.md` (Key Design Decisions) | - |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/mtu/cmd/{arith,doc,doctor,mtu,overhead,register,run,search,state,verdict}.go` and their tests | yes | `wc -l internal/component/mtu/cmd/*.go` at closure: 20 files; 6544 lines with the YANG packages, the fixture and the `.ci` |
| `internal/component/mtu/yang/ze-mtu-conf.yang`, `internal/plugins/mtu-cmd/yang/ze-mtu-cmd.yang` | yes | read at closure |
| `test/plugin/show-mtu-{exhaustive,host,json,no-ipsec-component,oversized-tunnels}.ci` | yes | `close-fx-run.log` names each |
| `test/interop-ipsec/scenarios/mtu-{tunnel-sizing-strongswan,nat-installed-endpoint}/{nat,ze,swanctl}.conf` | yes | `git status --untracked-files=all` at closure |
| `docs/architecture/diagnostics/path-mtu.md` | yes | untracked at closure, read whole |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-20 | the Acceptance Criteria table above, each by a named test | `./le job run label close-mtu-green command go test -count=1 ./internal/component/mtu/cmd/` ok (`close-green.log`); the `-race` run of the same package ok; `close-fx-run.log` 5/5 PASS; the interop logs named in the Interop table |
| AC-12/13 after the closure fix | the underlay command names the link's kind | `close-red.log` RED, `close-green.log` GREEN, `close-fx-run.log` oversized PASS with `set interface veth sr0 mtu 1400` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `show mtu` | `test/plugin/show-mtu-oversized-tunnels.ci`, `show-mtu-no-ipsec-component.ci` | yes: read; the fixture dispatches `show mtu` and asserts the tunnel rows, the commands and the verdict |
| `show mtu host <address>` | `test/plugin/show-mtu-host.ci`, `show-mtu-exhaustive.ci` | yes: read; 1400 via ICMP, then the poisoned cache and `exhaustive` |
| `show mtu \| json` | `test/plugin/show-mtu-json.ci` | yes: read; kebab-case keys asserted |
| the IKE engine's `init()` registers the inventory | `TestIPsecInventoryRegisteredByIKE` | yes: `inventory_test.go` read |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `mtu-negotiated-transform` red/green; the producers named in the Assumptions row |
| A-2 | confirmed | `TestESPOverheadPerTransform` over the registry population, red with the IV cut |
| A-3 | broken, benign | producers `createFirstChildSA`, `adoptAuthenticatedEndpoint`; Mistake Log; `plan/journal/one-state-held-in-two-fields.md` |
| A-4 | confirmed | `ErrNotRegistered`, `TestShowMTUNoInventoryIsNotZeroTunnels` red under the cut |
| A-5 | confirmed | the owner's `faros_mtu.py` 2.2 pinned in the distillation; `TestVerdictThresholds` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `path-mtu.md` payload keys, the run steps, the advice matrix (with `kind` and the withheld command) | `run.go` field constants, `verdict.go` `adviseUnderlay`, `underlayCommand` | yes, read against the producers at closure |
| `command-reference.md` `### show mtu` and its JSON example | `mtu.go` `parseMTUArgs`, the fixture's assertions (`set interface veth sr0 mtu 1400`) | yes |
| `ipsec-8-ikev2-child-xfrm.md`, `guide/ipsec.md` `vti { bind }` | `established.go` `resolveIfID` | yes |
| `status.md` doctor row, `core-design.md` `mtu` row, `api/commands.md`, `features.md`, `comparison.md`, `functional-tests.md`, `interop.md`, `configuration.md`, `active-probes.md`, `command-catalogue.md` | the registrations in `register.go`, the imports of `mtu/cmd`, the checker names in `checkers.go` | yes, each diff hunk read at closure |
| Categories 7, 8, 13, 14 (wire format, SDK, route metadata, counters) | no page under `docs/architecture/wire`, `docs/plugin-development` or `docs/architecture/meta` carries a claim this spec changes | N-A confirmed |
