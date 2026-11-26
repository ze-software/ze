# Spec: ipsec-lifetime-volume

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Required simultaneous time and volume lifetime behavior belongs to already-shipped IPsec; the source records an unfed softBytes path, not optional new protocol support. |
| RFC gaps | RFC4301-4.4.2.1-2, RFC4301-4.4.2.1-3 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`, this spec file, `internal/component/ike/ipsec/types.go`, `internal/component/ike/ipsec/config.go`, `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`, `internal/component/ike/engine/rekey.go`, `internal/component/ike/engine/established.go`, and `internal/component/ike/dataplane/dataplane.go`.

## Task

This spec owes two RFC 4301 Section 4.4.2.1 requirements:

1. **RFC4301-4.4.2.1-2** (owner ruling, 2026-09-21): "A compliant implementation MUST support both types of lifetimes, and MUST support a simultaneous use of both."
2. **RFC4301-4.4.2.1-3** (moved here from `spec-rfc-verdict-fix-ike-eap` under P-3, 2026-09-30): "Note that implementations MUST be able to handle having the counters at the ends of an SA get out of synch, e.g., because of packet loss or because the implementations at each end of the SA aren't doing things the same way."

Today Ze rekeys ESP Child SAs only on a time-based lifetime (`vpn ipsec esp-group <name> lifetime`). It cannot rekey by processed volume in bytes (`life-bytes`) or packets (`life-packets`). High-throughput tunnels reach cryptographic volume limits long before a time lifetime expires; without volume-based rekey they either rekey too rarely (weakening confidentiality/integrity margins) or must use an artificially short time lifetime (needless churn on quiet tunnels).

Add ESP-group volume-based lifetimes (`life-bytes` and `life-packets`) so a Child SA rekeys via the existing `CREATE_CHILD_SA` wire exchange when its processed byte count, packet count, or time lifetime reaches its soft threshold (whichever comes first), and tears down unconditionally when any configured hard limit is reached (RFC 7296 Section 2.8). Wire the existing unfed `lifetimeState.softBytes` / `byteCount` scaffolding in `internal/component/ike/engine/rekey.go`, extend it with hard byte limits and soft/hard packet limits, feed per-SA counters from the existing `dataplane.Dataplane.ListSAs` read path on `maintainSA`'s 1-second tick, and refuse volume limits at commit verify on dataplane backends (such as VPP) that do not expose SA counters.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform boundaries, exact-or-reject at config verify, single schema.
  → Constraint: a backend that cannot enforce volume lifetimes (no SA counter readback) MUST refuse them at config verify rather than silently ignoring them.
- [ ] `ai/doors/access-security.md` - IKE/IPsec boundaries, reload total-equality rule, and test gates.
  → Constraint: `ESPGroup.Equal` uses `reflect.DeepEqual`; keep all config fields on `ESPGroup` so reload/reconcile covers them automatically without a hand-maintained field list.
- [ ] `ai/doors/le-tooling.md` - `.ci` kernel gating (`option=needs-linux`), compiled Go fixtures, and scoped test commands.
  → Constraint: real-XFRM `.ci` tests carry `option=needs-linux:caps=net-admin,bpf` and `option=exclusive:group=ipsec-xfrm`; compiled Go fixtures return an error on assertion failure.
- [ ] `docs/architecture/ike/ipsec-3-data-model.md` - typed model between `ze-ipsec-conf.yang` and the IKE engine.
  → Decision: tree-walker parser in `config.go` into `ESPGroup` in `types.go`; `Changed()` / `ESPGroup.Equal` compares the whole value.
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - Child SA lifecycle, 1-second ticker, XFRM/VPP `ListSAs`, and backend capability interfaces.
  → Constraint: reuse `maintainSA`'s single 1-second ticker and `dataplane.Dataplane.ListSAs`; do not add a per-SA timer goroutine or a separate netlink expire listener.
- [ ] `docs/architecture/ike/ipsec-13-rekey-wire.md` - `CREATE_CHILD_SA` rekey exchange, soft-before-hard lead, and unconditional hard expiry.
  → Constraint: volume-based rekey triggers the same `serviceChildRekeyTrigger` -> `startChildRekey` -> `initiateChildRekey` path as time-based rekey; `hardExpired` stays unconditional even when a rekey is in flight.
- [ ] `docs/architecture/testing/interop.md` - IPsec interop suite (`test/interop-ipsec/scenarios/`), named scenario directories with no numeric prefix, and scoped `KernelCapabilities`.
  → Constraint: register the strongSwan scenario by descriptive name in `internal/le/interoplab/ipsec/checkers.go` under `test/interop-ipsec/scenarios/`.
- [ ] `ai/rules/config.md` - YANG leaf naming, `ze:help` + `description` contract, string coercion, and native range constraints.
  → Constraint: `life-bytes` and `life-packets` MUST be 64-bit (`uint64`) in YANG and Go, declare native `range` constraints, and carry both `ze:help` (at most 96 chars / 25 words) and a distinct `description`.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4301.md` - Section 4.4.2.1 SAD lifetime data items (`RFC4301-4.4.2.1-2`, `RFC4301-4.4.2.1-3`, `RFC4301-4.4.2.1-11`).
  → Constraint: support both time and byte/volume lifetimes simultaneously; handle out-of-sync counters between the two ends of an SA cleanly.
- [ ] `rfc/short/rfc7296.md` - Section 2.8 and Section 2.8.1 SA lifetime local policy, jittered rekey trigger, make-before-break `CREATE_CHILD_SA`, collision tie-breaking, and unconditional hard expiry.
  → Constraint: lifetimes are local policy and never negotiated on the wire; randomize soft volume threshold before the hard ceiling to desynchronize peers and leave headroom for make-before-break rekey; tear down the SA unconditionally once any hard limit is reached.

**Key insights:**
- `lifetimeState` in `internal/component/ike/engine/rekey.go` already holds `softBytes uint64` and `byteCount uint64` and checks `ls.softBytes > 0 && ls.byteCount >= ls.softBytes` in `softExpired`, but `newLifetimeState(lifetimeSec uint32)` never sets `softBytes`, returns `nil` when `lifetimeSec == 0`, and nothing feeds `byteCount`.
- Per-SA 64-bit byte and packet readback already landed in `dataplane.Dataplane.ListSAs(ifID uint32)` (`SAInfo.BytesCurrent`, `SAInfo.PacketsCurrent`, verified beyond 32 bits in `xfrm_readback_linux_test.go` and on real XFRM in `test/ipsec/ipsec-show-sa-counters.ci`).
- Byte volumes easily exceed `uint32` max (~10^13 bytes on 10G+ links); accepted maximum for both `life-bytes` and `life-packets` is `18446744073709551615` (`uint64` max), with `0` disabling the respective limit and non-zero minimums of `65536` bytes (64 KiB) and `100` packets to prevent rekey storms while keeping functional and interop runs fast.
- Volume soft-threshold randomization MUST compute jitter with `big.Int.SetUint64` rather than casting `uint64` to `int64`, so values above `math.MaxInt64` (up to `uint64` max) never wrap negative.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/ipsec/types.go` - `ESPGroup` holds `Name string`, `Lifetime uint32`, `PFS PFSMode`, `Proposals []ESPProposal`; no byte or packet lifetime fields. `ESPGroup.Equal` compares the whole struct via `reflect.DeepEqual`.
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - `esp-group` declares `leaf lifetime` (`type uint32 { range "0..86400"; } default 3600;`) and no `life-bytes` or `life-packets` leaves.
- [ ] `internal/component/ike/ipsec/config.go` - `parseESPGroup` parses `lifetime` (uint32, capped at `maxLifetime = 86400`, default `3600`), `pfs`, and `proposal`; no volume leaves are parsed.
- [ ] `internal/component/ike/engine/rekey.go` - `lifetimeState` holds `softTime`, `hardTime`, `softBytes uint64`, `byteCount uint64`; `newLifetimeState(lifetimeSec uint32)` returns `nil` when `lifetimeSec == 0` and only populates `softTime`/`hardTime` via `rekeyLead`; `softExpired(now)` checks `softTime` and `softBytes > 0 && byteCount >= softBytes`; `hardExpired(now)` checks `hardTime` only.
- [ ] `internal/component/ike/engine/established.go` - `runEstablished` and `maintainSA` (`out.createdChild`, `out.newChild`) all construct `childLT` via `newLifetimeState(ps.getESPGroup().Lifetime)`. `maintainSA`'s 1-second ticker calls `serviceChildRekeyTrigger(sa, childLT, now, tr, log)` and checks `childLT.hardExpired(now)`, never reading SA counters from `dp`.
- [ ] `internal/component/ike/dataplane/dataplane.go` - `Dataplane.ListSAs(ifID uint32) ([]SAInfo, error)` already returns `SAInfo` carrying `BytesCurrent uint64`, `PacketsCurrent uint64`, `BytesHard uint64`, `PacketsHard uint64`, keyed by `SAIdentity` (`IdentityOf(spi, dst, proto, ifID)`). Backend capabilities use small optional interfaces (`CatchAllInstaller`, `SAOptionChecker`, `InboundStateSelectorInstaller`, `SAStateScopeInstaller`).
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` - `xfrmBackend.ListSAs` populates `SAInfo.BytesCurrent` and `SAInfo.PacketsCurrent` from `s.Statistics.Bytes` and `s.Statistics.Packets` (64-bit). `xfrmStateFromParams` sets no `Limits`, leaving all kernel limits at `XFRM_INF`.
- [ ] `internal/component/ike/dataplane/vpp.go`, `noop.go`, `xfrm_other.go`, `recording_zetest.go` - `ListSAs` returns `ErrNotSupported`; `SAOptionsSupported` returns `nil` on `noop`, `xfrm_other`, and `recording`, and enforces backend limits on `vpp` and Linux `xfrm`.
- [ ] `internal/component/ike/engine/config_verify_commit.go` - `verifyIPsecCommit` verifies candidate configs against the loaded dataplane (`verifyUnmatchedEnforceable`, `verifyPeerSAOptions`, `verifyForwardBypass`) before staging.

**Behavior to preserve:**
- Time-based rekey and hard-expiry behavior is unchanged when neither `life-bytes` nor `life-packets` is configured (or both are `0`), and `maintainSA` performs zero `ListSAs` calls per tick for time-only tunnels.
- `newLifetimeState(lifetimeSec uint32)` continues to work unchanged for IKE SAs (`ikeGroup.Lifetime`) and existing time-only unit tests.
- Existing `CREATE_CHILD_SA` make-before-break Child SA rekey, collision tie-breaking (RFC 7296 Section 2.8.1), `rebindRekeyOwed`, `TEMPORARY_FAILURE` / proposal-refusal backoffs, and `ze_ipsec_rekey_total{peer}` counting remain unchanged.
- Existing `esp-group` configurations without volume leaves remain valid with identical defaults (`lifetime 3600`, `life-bytes 0`, `life-packets 0`).
- `show vpn ipsec sa` and `show vpn ipsec dataplane` continue to read `SAInfo` counters unchanged.

**Behavior to change:**
- `esp-group` gains `life-bytes` (`uint64`, `0 | 65536..18446744073709551615`, default `0`) and `life-packets` (`uint64`, `0 | 100..18446744073709551615`, default `0`) leaves in YANG, `ESPGroup`, and `parseESPGroup`.
- `verifyIPsecCommit` refuses non-zero `life-bytes` or `life-packets` on a loaded dataplane backend that does not support SA counter readback (such as VPP), returning an `opererr` error naming the `esp-group`, backend, reason, and fix.
- `newChildLifetimeState(ps.getESPGroup())` builds `lifetimeState` carrying time, byte, and packet soft/hard thresholds simultaneously (or volume-only when `lifetime 0` is combined with non-zero volume limits).
- On each 1-second tick of `maintainSA`, when `childLT` has volume limits, `sampleChildVolume` reads the active Child SA's inbound and outbound `SAInfo` counters via `dp.ListSAs(child.IfID)` and feeds `max(outBytes, inBytes)` and `max(outPackets, inPackets)` into `childLT`.
- `softExpired` triggers `CREATE_CHILD_SA` rekey when any configured soft limit (time, bytes, or packets) is reached first; `hardExpired` tears down the Child SA unconditionally when any configured hard limit (time, bytes, or packets) is reached first.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| SA lifetime tracker | `internal/component/ike/engine/rekey.go` (`lifetimeState`, `newLifetimeState`, `softExpired`, `hardExpired`) | Tracks time `softTime`/`hardTime` with randomized lead (`rekeyLead`) and holds unfed `softBytes`/`byteCount` fields checked in `softExpired` | Extends: adds `hardBytes`, `softPackets`, `hardPackets`, `packetCount`, `newChildLifetimeState`, `hasVolumeLimits`, `recordVolume`, and hard volume checks in `hardExpired` while keeping `newLifetimeState` for IKE SAs |
| Established SA owner tick and rekey trigger | `internal/component/ike/engine/established.go` (`maintainSA`, `serviceChildRekeyTrigger`, `startChildRekey`) | Single 1-second ticker drives DPD, MOBIKE, Child/IKE soft rekey trigger, and unconditional hard expiry | Extends: calls `sampleChildVolume` on the existing 1-second tick only when `childLT.hasVolumeLimits()` and swaps Child SA lifetime construction to `newChildLifetimeState` |
| Dataplane SA counter readback | `internal/component/ike/dataplane/dataplane.go` (`Dataplane.ListSAs`, `SAInfo.BytesCurrent`, `SAInfo.PacketsCurrent`, `IdentityOf`) and `xfrm_linux.go` (`saInfoFromState`) | Dumps kernel XFRM SAD states with 64-bit `BytesCurrent`/`PacketsCurrent` per `(SPI, Dst, Proto, IfID)` identity (already used by `show vpn ipsec sa` and `health_drift.go`) | Extends (reuses as consumer): `sampleChildVolume` reads outbound and inbound `SAInfo` counters via `dp.ListSAs(child.IfID)` and `IdentityOf` with zero changes to netlink state installation |
| Commit-time backend capability verification | `internal/component/ike/engine/config_verify_commit.go` (`verifyIPsecCommit`), `sa_options.go` (`verifyPeerSAOptions`), `dataplane/dataplane.go` (`CatchAllInstaller`, `SAOptionChecker`) | Optional capability interfaces on `Dataplane` backends checked in `verifyIPsecCommit` to refuse unsupported options with `opererr` at commit time | Extends: adds `SACounterChecker` (`SACountersSupported() error`) on `dataplane` backends and `verifyVolumeLifetimes` in `verifyIPsecCommit` so backends without SA counter readback (such as VPP) fail closed at commit |
| IPsec data model and total-equality reload | `internal/component/ike/ipsec/types.go` (`ESPGroup`, `ESPGroup.Equal`), `config.go` (`parseESPGroup`), `yang/ze-ipsec-conf.yang` | Parses `vpn ipsec esp-group` into `ESPGroup`; `ESPGroup.Equal` uses `reflect.DeepEqual` so `reconcilePeers` / `adoptESPGroup` detects any field change | Extends: adds `LifeBytes uint64` and `LifePackets uint64` to `ESPGroup`, YANG schema, and `parseESPGroup`; reload detection and childless adoption work automatically |
| `CREATE_CHILD_SA` wire rekey and collision resolution | `internal/component/ike/engine/rekey.go` (`initiateChildRekey`, `applyChildRekeyResponse`, `respondChildRekey`, `localNonceIsLower`) | Performs make-before-break Child SA rekey, deletes superseded SA, and resolves simultaneous rekeys by lowest nonce (RFC 7296 §2.8.1) | Extends (reuses unchanged): volume soft-expiry enters the exact same `startChildRekey` path, and peer-initiated rekeys from out-of-sync counters enter `respondChildRekey` unchanged |
| Rekey metrics and monitor events | `internal/component/ike/engine/metrics.go` (`ze_ipsec_rekey_total`), `established.go` (`incRekeyCount`, `emitChildRekey`) | Increments `ze_ipsec_rekey_total{peer}` and emits `child-rekey` events whenever `out.newChild` installs | Does not replace (reuses unchanged): volume-triggered Child SA rekeys produce `out.newChild` and are automatically counted and streamed |
| Kernel XFRM netlink lifetime limits | `vendor/github.com/vishvananda/netlink/xfrm_state_linux.go` (`XfrmStateLimits`) | Kernel can enforce `ByteSoft`/`ByteHard`/`PacketSoft`/`PacketHard`, where `^uint64(0)` (`18446744073709551615`) is reserved as `XFRM_INF` (unlimited) and expiry fires async `XFRM_MSG_EXPIRE` | Does not apply: Ze has no netlink expire listener, kernel expiry would create dataplane drift before `maintainSA` ticks, and kernel `XFRM_INF` cannot represent `uint64` max as a finite limit (AC-4); engine-driven tracking via `ListSAs` matches time lifetimes |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A backend that cannot apply a config exactly refuses it at verify. It never approximates" | agrees | `life-bytes` and `life-packets` are validated in `ze-ipsec-conf.yang` and `parseESPGroup`, and `verifyVolumeLifetimes` checks `dataplane.SACounterChecker` in `verifyIPsecCommit` so a backend without SA counter readback (such as VPP) refuses volume lifetimes at verify rather than silently ignoring them. |
| `ai/doors/access-security.md` | Traps | the door covering `internal/component/ike/...` | "A field list over a config type misses the next field." | agrees | `LifeBytes` and `LifePackets` are added directly to `ipsec.ESPGroup`, whose `Equal` method uses `reflect.DeepEqual`, so reload reconciliation and childless `adoptESPGroup` pick up edits to both leaves automatically with no field list. |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `test/ipsec/ipsec-life-volume.ci`, `internal/test/fixture/...`, and `internal/le/interoplab/ipsec/...` | "A `.ci` that needs a Linux kernel feature carries `option=needs-linux`" | agrees | `test/ipsec/ipsec-life-volume.ci` carries `option=needs-linux:caps=net-admin,bpf` and `option=exclusive:group=ipsec-xfrm` and drives its real-XFRM assertions through a compiled Go fixture returning an error on assertion failure; unit tests run through `./le test unit pkg path ...`. |
| `docs/architecture/ike/ipsec-3-data-model.md` | Decisions | the `// Design:` page of `internal/component/ike/ipsec/types.go` and `config.go`; anchored to `ze-ipsec-conf.yang`; modified by this spec | "`reflect.DeepEqual` compares every member, so a member added to the type is compared on the day it is added." | agrees | `parseESPGroup` extends the existing tree-walker parser to populate `ESPGroup.LifeBytes` and `ESPGroup.LifePackets` as `uint64` (with `uint64` constant comparisons avoiding `gocritic` truncation warnings), and `ESPGroup.Equal` compares them automatically. This spec updates `ipsec-3-data-model.md` to document the two new `ESPGroup` fields and bounds. |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Decisions | anchored to `internal/component/ike/ipsec/types.go`, `rekey.go`, and `established.go` | "A reload restarts a peer whose configuration differs in ANY member, and leaves every other peer alone." | agrees | Because `peerConfigChanged` compares resolved `ipsec.ESPGroup.Equal`, editing `life-bytes` or `life-packets` restarts peers with installed Child SAs so the new lifetime takes effect and lets childless IKE SAs adopt the group in place via `adoptESPGroup`, leaving unrelated peers untouched. |
| `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` | Decisions | the `// Design:` page of `rekey.go`, `established.go`, `config_verify_commit.go`, and `dataplane/*.go`; modified by this spec | "One 1-second ticker drives dead peer detection and lifetimes, not a timer per SA." | agrees | `maintainSA` samples `dp.ListSAs(child.IfID)` on its existing 1-second ticker only when `childLT.hasVolumeLimits()` is true, adding no per-SA goroutine or parallel timer; VPP keeps `ListSAs` returning `ErrNotSupported` and refuses volume lifetimes via `SACounterChecker`. This spec updates `ipsec-8-ikev2-child-xfrm.md` to document volume sampling and `SACounterChecker`. |
| `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` | Decisions | anchored to `internal/component/ike/engine/established.go` and `internal/component/ike/dataplane/dataplane.go` | - | silent | Page covers EAP authentication, NAT traversal, NAT rebinding, and IKE UDP socket options. It makes no claim about Child SA byte or packet lifetimes, and `serviceChildRekeyTrigger` preserves `ps.rebindRekeyOwed` beside `childLT.softExpired(now)` unchanged. |
| `docs/architecture/ike/ipsec-11-interop-eap.md` | Decisions | anchored to `internal/le/interoplab/ipsec/checkers.go` | - | silent | Page covers EAP-MSCHAPv2 and EAP-TLS initiator authentication, session-ticket caching, anonymous NAI, revocation, and EAP interop scenarios. It makes no claim about Child SA volume lifetimes, and the new `child-rekey-life-bytes` scenario uses PSK authentication in `lifevolume.go`. |
| `docs/architecture/ike/ipsec-13-rekey-wire.md` | Traps this code exists to avoid | anchored to `internal/component/ike/engine/rekey.go` and `established.go`; modified by this spec | "hard expiry itself is NOT gated on a pending rekey." | agrees | Volume soft limits sit 5%–10% ahead of the hard byte/packet ceilings to leave make-before-break headroom and desynchronize peers, triggering `serviceChildRekeyTrigger` only when `pendingRekey == nil`, while `childLT.hardExpired(now)` remains unconditional on time, byte, and packet hard ceilings. This spec updates `ipsec-13-rekey-wire.md` to cover volume thresholds alongside time thresholds. |
| `docs/architecture/ike/ipsec-14-responder.md` | Decisions | anchored to `internal/component/ike/engine/rekey.go` and `established.go` | - | silent | Page covers IKEv2 responder role directionality, EAP server, virtual IP leasing, and parallel re-initiation. It states no rule about SA time or volume lifetimes; `runEstablished` adopts the responder's first Child SA and starts `newChildLifetimeState` on it without changing responder role or handshake semantics. |
| `docs/architecture/ike/ipsec-dataplane-inspection.md` | Decision: the counters come from the kernel | anchored to `internal/component/ike/dataplane/dataplane.go`, `xfrm_linux.go`, `vpp.go`, and `noop.go` | "The IKE engine never sees ESP payload, so a userspace count reports zero forever. `BytesCurrent` and `PacketsCurrent` are what the kernel recorded on the SA." | agrees | `sampleChildVolume` consumes `SAInfo.BytesCurrent` and `SAInfo.PacketsCurrent` from `dp.ListSAs(child.IfID)` keyed by `dataplane.IdentityOf` rather than counting in userspace; `vpp` and `noop` keep `ListSAs` returning `ErrNotSupported`, and `SACounterChecker` refuses volume lifetimes at commit verify on counterless backends. |
| `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` | Decisions | anchored to `internal/component/ike/engine/rekey.go` | - | silent | Page covers traffic-selector narrowing, transport-mode negotiation, NAT selector substitution, and EAP-TLS 1.2 export refusal. It states no rule about Child SA lifetimes or byte/packet rekey triggers, and this spec leaves narrowing and transport mode untouched. |
| `docs/architecture/linux-limitations.md` | What belongs here | anchored to `internal/component/ike/dataplane/xfrm_linux.go` | - | silent | Register records Linux kernel and library limitations that block or constrain Ze. Linux XFRM already exposes 64-bit `BytesCurrent` and `PacketsCurrent` through `ListSAs`, and engine-driven volume lifetimes introduce no new kernel limitation or workaround. |
| `docs/architecture/testing/interop.md` | The suites, one per protocol area | anchored to `internal/component/ike/engine/rekey.go`, `xfrm_linux.go`, and `internal/le/interoplab/ipsec/checkers.go` | "A scenario directory is NAMED and carries no numeric prefix." | agrees | The strongSwan volume-rekey scenario is placed at `test/interop-ipsec/scenarios/child-rekey-life-bytes/` (no numeric prefix, IPsec suite rather than BGP suite) with its typed Go checker registered in `scenarioCheckers` in `internal/le/interoplab/ipsec/checkers.go`. |
| `docs/architecture/testing/interop.md` | Design decisions | anchored to `internal/component/ike/engine/rekey.go`, `xfrm_linux.go`, and `internal/le/interoplab/ipsec/checkers.go` | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | `child-rekey-life-bytes` joins the existing IPsec interop suite (`internal/le/interoplab/ipsec/`) and inherits only the suite's existing IPsec/ESP `KernelCapabilities` without adding any extra capability requirement or affecting unrelated suites. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config leaves under `vpn ipsec esp-group <name>`: `life-bytes` (`uint64`, range `0 | 65536..18446744073709551615`, default `0`) and `life-packets` (`uint64`, range `0 | 100..18446744073709551615`, default `0`) in `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`.
- Runtime traffic volume counters from the dataplane SAD: `SAInfo.BytesCurrent` and `SAInfo.PacketsCurrent` returned by `dataplane.Dataplane.ListSAs(child.IfID)` on Linux XFRM.

### Transformation Path
1. **Schema validation (Layer 1/2):** `ze-ipsec-conf.yang` validates `life-bytes` (`0 | 65536..18446744073709551615`) and `life-packets` (`0 | 100..18446744073709551615`) at parse/validate/commit, rejecting negative values, sub-minimum non-zero values, and `uint64` overflow (`18446744073709551616`) without wrapping.
2. **Config delivery and parsing:** Config lowers `vpn` via `Tree.ToPluginMap` into JSON strings; `parseIPsecFromJSON` -> `config.TreeFromPluginMap` -> `ipsec.ParseIPsecConfig` -> `parseESPGroup` parses `life-bytes` and `life-packets` with `strconv.ParseUint(v, 10, 64)`, enforces the same non-zero minimums (`minLifeBytes = 65536`, `minLifePackets = 100`), and stores them on `ESPGroup.LifeBytes` and `ESPGroup.LifePackets`.
3. **Commit-time backend verification:** `verifyIPsecCommit` calls `verifyVolumeLifetimes(dp, dpName, cfg)`. If any `ESPGroup` sets `LifeBytes > 0` or `LifePackets > 0` and `dp != nil`, it queries `dataplane.SACounterChecker.SACountersSupported()` (failing closed if the backend does not implement the interface or returns an error such as VPP's `ErrNotSupported`), returning an `opererr` error naming the group, backend, reason, and fix.
4. **Reload reconciliation:** `reconcilePeers` compares `ESPGroup.Equal` (`reflect.DeepEqual`); a peer with an installed Child SA whose `esp-group` volume lifetime changed restarts, while a childless IKE SA adopts the updated `ESPGroup` via `adoptESPGroup`.
5. **Child SA lifetime initialization:** `runEstablished` (first Child SA), `maintainSA` (`out.createdChild` on a childless SA, and `out.newChild` after any Child SA rekey) call `newChildLifetimeState(ps.getESPGroup())`.
   - If `Lifetime == 0 && LifeBytes == 0 && LifePackets == 0`, returns `nil`.
   - If `Lifetime > 0`, computes `hardTime` and `softTime` with `rekeyLead` (unchanged).
   - If `LifeBytes > 0`, sets `hardBytes = LifeBytes` and `softBytes = volumeSoftThreshold(LifeBytes)` (randomized 5%–10% below `hardBytes` via `crypto/rand` and `big.Int.SetUint64`, strictly less than `hardBytes`).
   - If `LifePackets > 0`, sets `hardPackets = LifePackets` and `softPackets = volumeSoftThreshold(LifePackets)` (same 5%–10% randomized lead, strictly less than `hardPackets`).
6. **Per-tick counter sampling:** On each 1-second tick of `maintainSA`, if `childLT != nil && childLT.hasVolumeLimits()` and `child := ps.getChildSA(); child != nil`:
   - `sampleChildVolume` calls `dp.ListSAs(child.IfID)` and looks up both directional identities of the active Child SA: outbound `IdentityOf(child.OutboundSPI, child.RemoteAddr, ProtoESP, child.IfID)` and inbound `IdentityOf(child.InboundSPI, child.LocalAddr, ProtoESP, child.IfID)`.
   - Computes `bytes = max(outBytes, inBytes)` and `packets = max(outPackets, inPackets)` and calls `childLT.recordVolume(bytes, packets)` (monotonically updating `byteCount` and `packetCount` so a transient missing lookup cannot move counts backward).
   - If `dp == nil`, leaves counts unchanged. If `ListSAs` returns `ErrNotSupported`, logs once per Child SA at debug (`counterReadWarned`) and leaves counts unchanged. On a transient error, logs at debug and keeps the previous sample.
   - When `childLT == nil || !childLT.hasVolumeLimits()`, `ListSAs` is NOT called at all.
7. **Soft rekey trigger and hard teardown:**
   - `serviceChildRekeyTrigger` checks `childLT.softExpired(now)` (true if time OR `softBytes` OR `softPackets` reached) and initiates `CREATE_CHILD_SA` via `startChildRekey` -> `initiateChildRekey`.
   - When `out.newChild` completes (whether locally initiated or peer-initiated via `respondChildRekey`), `maintainSA` replaces `childLT` with a fresh `newChildLifetimeState(ps.getESPGroup())` whose `byteCount` and `packetCount` start at `0` for the new SPIs, and increments `ze_ipsec_rekey_total{peer}`.
   - If `childLT.hardExpired(now)` becomes true (time OR `hardBytes` OR `hardPackets` reached), `maintainSA` logs the expiry reason (`time`, `bytes`, or `packets`), calls `ps.cleanupChild(dp, bus, log)`, and returns `errTimeout` unconditionally, even if a rekey is in flight (RFC 7296 Section 2.8).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ `ike` plugin (`OnConfigVerify` / `OnConfigApply`) | `vpn` JSON section via `Tree.ToPluginMap` / `config.TreeFromPluginMap` -> `parseESPGroup` (`life-bytes`, `life-packets` as decimal strings parsed to `uint64`) | No |
| `ipsec.ESPGroup` ↔ `engine.lifetimeState` | `newChildLifetimeState(ps.getESPGroup())` populates `softTime`/`hardTime`, `softBytes`/`hardBytes`, `softPackets`/`hardPackets` | No |
| `dataplane.Dataplane` ↔ `engine.maintainSA` | `dp.ListSAs(child.IfID)` returns `[]dataplane.SAInfo` (`BytesCurrent`, `PacketsCurrent`) matched by `dataplane.IdentityOf` | No |
| `dataplane.SACounterChecker` ↔ `engine.verifyIPsecCommit` | Backend capability check refuses volume lifetimes at commit verify on backends without SA counter readback | No |

### Changed-Wire Census

**Search evidence:** Searched `internal/component/ike/`, `internal/plugins/`, `cmd/`, and `pkg/` for `"esp-group"`, `esp-group`, and `ESPGroup` (positive controls found: `internal/component/ike/ipsec/config.go::parseESPGroup`, `internal/component/ike/engine/config.go::parseIPsecFromJSON`, `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`). Checked IKEv2 wire builders in `internal/component/ike/engine/rekey.go` (`initiateChildRekey`, `respondChildRekey`): RFC 7296 Section 2.8 specifies that SA lifetimes are local policy and are NOT carried on the IKEv2 wire, so no IKEv2 or ESP wire message format changes; `CREATE_CHILD_SA` and `INFORMATIONAL` Delete bytes remain identical. The only changed wire is the internal plugin-config JSON section for `vpn ipsec esp-group`, which gains two optional decimal-string leaves (`life-bytes`, `life-packets`).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Plugin config JSON (`vpn` section, `esp-group` entry) | writer | `internal/component/config/plugin_map.go::Tree.ToPluginMap` (driven by `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`) | Generic YANG-driven serializer; emits every configured leaf under `esp-group` as a JSON string (`ai/rules/config.md`) | Preserve writer unchanged; add `life-bytes` and `life-packets` leaves to `ze-ipsec-conf.yang` |
| Plugin config JSON (`vpn` section, `esp-group` entry) | reader | `internal/component/ike/engine/config.go::parseIPsecFromJSON` -> `internal/component/ike/ipsec/config.go::parseESPGroup` | Only production reader of `esp-group` config entries (`config.go:205,242`); reads leaves as strings via `t.Get` | Change `parseESPGroup` to parse optional `life-bytes` and `life-packets` with `strconv.ParseUint(v, 10, 64)`; absent leaves default to `0` so existing configs preserve exact behavior |
| IKEv2 `CREATE_CHILD_SA` / `INFORMATIONAL` wire protocol | writer / reader | `internal/component/ike/engine/rekey.go::initiateChildRekey`, `applyChildRekeyResponse`, `respondChildRekey` and external IKEv2 peers (strongSwan) | RFC 7296 Section 2.8 carries no lifetime payload on the IKEv2 wire; volume rekey sends the exact `N(REKEY_SA), SA, Ni, [KEi,] TSi, TSr` exchange already implemented | Preserve unchanged; proven against strongSwan in `child-rekey-life-bytes` |

### Integration Points
- `ESPGroup` (`internal/component/ike/ipsec/types.go`) - add `LifeBytes uint64` and `LifePackets uint64`.
- `parseESPGroup` (`internal/component/ike/ipsec/config.go`) - parse and range-check `life-bytes` and `life-packets` as `uint64`.
- `ze-ipsec-conf.yang` (`internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`) - declare `life-bytes` and `life-packets` leaves under `esp-group`.
- `SACounterChecker` (`internal/component/ike/dataplane/dataplane.go`) - optional capability interface implemented on `xfrmBackend` (Linux and non-Linux), `vppBackend`, `noopDataplane`, and `recordingDataplane`, checked by `verifyVolumeLifetimes` in `internal/component/ike/engine/config_verify_commit.go`.
- `lifetimeState` / `newChildLifetimeState` (`internal/component/ike/engine/rekey.go`) - hold time, byte, and packet soft/hard thresholds and counters; evaluate `softExpired` and `hardExpired`.
- `runEstablished` / `maintainSA` / `sampleChildVolume` (`internal/component/ike/engine/established.go`) - initialize Child SA lifetimes from `ESPGroup` and sample `dp.ListSAs(child.IfID)` on the 1-second tick when volume limits are active.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Config flows `ze-ipsec-conf.yang` -> `Tree.ToPluginMap` -> `parseIPsecFromJSON` -> `parseESPGroup` -> `ESPGroup` -> `newChildLifetimeState` -> `maintainSA`; counters flow `dataplane.Dataplane.ListSAs` -> `sampleChildVolume` -> `lifetimeState`. |
| No unintended coupling (components stay isolated) | Yes | `ipsec` stays decoupled from `engine` and `dataplane`; `engine` reads counters only through the existing `dataplane.Dataplane` interface and checks backend capability through the new `dataplane.SACounterChecker` interface (no netlink or VPP imports in `engine`). |
| No duplicated functionality (extends existing, does not recreate) | Yes | Extends `lifetimeState`, `maintainSA`'s 1-second ticker, `dp.ListSAs`, `verifyIPsecCommit`, and `CREATE_CHILD_SA` rekey rather than adding a parallel timer, netlink expire listener, or second rekey path. |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `lifetimeState` is updated in place on the owner goroutine (`maintainSA`); `ListSAs` is only invoked when `childLT.hasVolumeLimits()` is true, adding zero allocation overhead to time-only tunnels. |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No core/shared package is touched; `ze-ipsec-conf.yang` is already embedded and registered by `internal/component/ike/ipsec/yang`, and `SACounterChecker` is an optional interface implemented by each dataplane backend beside `CatchAllInstaller` and `SAOptionChecker`. |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `internal/`, `cmd/`, and `pkg/` for `esp-group` and `ESPGroup`: CLI completion, help text, config tree validation, and `Tree.ToPluginMap` all derive dynamically from `ze-ipsec-conf.yang`; peer reload derives from `ESPGroup.Equal` (`reflect.DeepEqual`); only `parseESPGroup` (the owner of `esp-group` parsing) reads the new leaf names. |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `dataplane.Dataplane.ListSAs(ifID)` returns live 64-bit `BytesCurrent` and `PacketsCurrent` for both inbound and outbound Child SA states on Linux XFRM | `internal/component/ike/dataplane/xfrm_linux.go::saInfoFromState`, `xfrm_readback_linux_test.go::TestListSAsCountersBeyond32Bits`, `test/ipsec/ipsec-show-sa-counters.ci` | Volume counters would remain zero and rekey would never trigger | `test/ipsec/ipsec-life-volume.ci` and `child-rekey-life-bytes` interop scenario | unvalidated |
| A-2 | `uint64` end-to-end (YANG `uint64`, `strconv.ParseUint(..., 64)`, `ESPGroup`, `lifetimeState`, `SAInfo`) handles values up to `18446744073709551615` without truncation or signed wrap in jitter math | `ze-ipsec-conf.yang`, `rekey.go`, `big.Int.SetUint64` | Large byte/packet limits truncate or panic in `rand.Int` | `TestESPGroupVolumeParse` and `TestVolumeSoftThresholdNearUint64Max` | unvalidated |
| A-3 | Using `max(outBytes, inBytes)` and `max(outPackets, inPackets)` matches RFC 4301 per-unidirectional-SA volume limits and prevents overflow from summing two `uint64` counters | RFC 4301 §4.4.2.1 (each unidirectional SA carries its own key and lifetime); Linux XFRM counts per unidirectional state | Bidirectional sum would rekey at half the per-key limit and risk `uint64` wrap | `TestSampleChildVolumeUsesMaxDirection` and strongSwan interop | unvalidated |
| A-4 | On a single-host real-XFRM `.ci` fixture (`peer-a` real XFRM, `peer-b` noop), outbound packets matching `peer-a`'s XFRM policy increment `peer-a`'s outbound XFRM state `BytesCurrent`/`PacketsCurrent` enough to trigger a real `CREATE_CHILD_SA` rekey | Linux XFRM encrypts outbound packets matched by an installed OUT policy + state regardless of whether the remote drops ESP | `.ci` fixture would need network namespaces or veth to exercise real XFRM counters | Wiring phase proof of `test/ipsec/ipsec-life-volume.ci` in QEMU | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Casting a `uint64` byte or packet limit above `math.MaxInt64` to `int64` inside `rand.Int` wraps negative and panics | Panic in `volumeSoftThreshold` at `18446744073709551615` | Construct `big.Int` with `new(big.Int).SetUint64(span + 1)` and unit-test at `math.MaxUint64` |
| R-2 | Tiny configured `life-bytes` or `life-packets` values cause continuous rekey storms under normal traffic | Rapid `CREATE_CHILD_SA` churn | Enforce minimums (`65536` bytes and `100` packets) in both `ze-ipsec-conf.yang` and `parseESPGroup`; `0` disables |
| R-3 | Configuring volume lifetimes on VPP (where `ListSAs` returns `ErrNotSupported`) silently ignores the limit | Tunnel never rekeys by volume on VPP | Refuse non-zero `life-bytes`/`life-packets` at `verifyIPsecCommit` via `SACounterChecker` with an `opererr` naming the fix |
| R-4 | High throughput crosses both `softBytes` and `hardBytes` within one 1-second tick before a rekey finishes | Child SA hard-expires under bursty load instead of rekeying | Place `softBytes`/`softPackets` 5%–10% below the hard ceiling (`volumeSoftThreshold`) so sustained traffic triggers `CREATE_CHILD_SA` well ahead of the hard limit; if hard limit is still hit, RFC 7296 §2.8 requires unconditional teardown and reconnect |
| R-5 | Two peers with identical `life-bytes` or out-of-sync counters collide or error when one rekeys first | Peer rekey rejected or duplicate SAs left behind | Randomized 5%–10% soft margin desynchronizes peers; `respondChildRekey` accepts peer-initiated rekey at any local count and resets `childLT`; simultaneous collisions resolve via existing `localNonceIsLower` (RFC 7296 §2.8.1) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A Child SA with `life-bytes`/`life-packets` set could fail to rekey (staying past its volume limit), rekey prematurely/repeatedly, or tear down a live tunnel early on a miscomputed hard threshold. Time-only tunnels (the default) take an unchanged path with zero `ListSAs` calls. |
| How is it reverted? | Single commit revert of the Go, YANG, test, and doc changes; existing configs without `life-bytes`/`life-packets` are unaffected, and no wire protocol or persistent state format changes. |
| Who else touches this path? | `internal/component/ike/engine/rekey.go`, `established.go`, `config_verify_commit.go`, and `internal/component/ike/dataplane/*.go` are shared by all IPsec lifecycle specs; changes are additive to `ESPGroup`, `lifetimeState`, and `Dataplane` capability checks. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `vpn ipsec esp-group <name> life-bytes N` + outbound ESP traffic on real XFRM | → | `parseESPGroup` -> `newChildLifetimeState` -> `maintainSA` `sampleChildVolume` -> `softExpired` -> `startChildRekey` -> `CREATE_CHILD_SA` | `test/ipsec/ipsec-life-volume.ci` |
| `vpn ipsec esp-group <name> life-packets P` + outbound ESP traffic on real XFRM | → | `parseESPGroup` -> `newChildLifetimeState` -> `maintainSA` `sampleChildVolume` -> `softExpired` -> `startChildRekey` -> `CREATE_CHILD_SA` | `test/ipsec/ipsec-life-volume.ci` |
| `vpn ipsec esp-group <name> life-bytes N` against strongSwan peer with real bidirectional tunnel traffic | → | Real XFRM counter increment -> Ze `CREATE_CHILD_SA` rekey -> strongSwan accepts replacement SA and receives old-SA Delete | `test/interop-ipsec/scenarios/child-rekey-life-bytes` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `esp-group` sets `life-bytes N` (with long `lifetime 3600` or `lifetime 0`) and the Child SA processes at least `softBytes` bytes in either direction | `sampleChildVolume` reads the dataplane SA counter, `softExpired` triggers a `CREATE_CHILD_SA` Child SA rekey before time expiry, the replacement Child SA installs with fresh SPIs and reset volume counters, and traffic continues | None: any path that fails to rekey on byte volume or keeps stale counters violates RFC4301-4.4.2.1-2 |
| AC-2 | `esp-group` sets `life-packets P` (with long `lifetime 3600` or `lifetime 0`) and the Child SA processes at least `softPackets` packets in either direction | `sampleChildVolume` reads the dataplane SA packet counter, `softExpired` triggers a `CREATE_CHILD_SA` Child SA rekey before time expiry, and the replacement Child SA installs with fresh SPIs and reset packet counters | None: any path that ignores packet volume fails the configured `life-packets` leaf |
| AC-3 | `esp-group` sets time `lifetime` simultaneously with `life-bytes` and/or `life-packets` | Whichever enabled soft limit (time, bytes, or packets) is reached first triggers `CREATE_CHILD_SA` rekey, and whichever enabled hard limit is reached first tears down the Child SA unconditionally (RFC 7296 §2.8, RFC4301-4.4.2.1-2) | None: simultaneous time and volume enforcement is the core RFC4301-4.4.2.1-2 MUST |
| AC-4 | `esp-group` sets `life-bytes` or `life-packets` to `18446744073709551615` (`uint64` max), or to `18446744073709551616` (overflow) | `18446744073709551615` is accepted by YANG validation and `parseESPGroup`, stored without truncation, and `newChildLifetimeState` produces a valid `soft < hard` threshold without signed wrap; `18446744073709551616` is rejected without wrapping | None: truncation or signed wrap silently corrupts operator limits |
| AC-5 | `esp-group` omits `life-bytes` and `life-packets`, or sets both to `0` | Time-only rekey and hard-expiry behavior is preserved identically, `childLT.hasVolumeLimits()` is false, and `maintainSA` makes zero `ListSAs` calls per tick | Time-based rekey and expiry remain permitted and active per configured `lifetime` |
| AC-6 | `esp-group` sets non-zero `life-bytes` below `65536` (`1..65535`) or non-zero `life-packets` below `100` (`1..99`), or commits non-zero volume lifetimes against a backend whose `SACountersSupported()` fails (such as VPP) | YANG validation and `parseESPGroup` reject sub-minimum values naming the leaf and valid range; `verifyIPsecCommit` rejects volume lifetimes on a counterless backend with an `opererr` naming the group, backend, and fix | Value `0` remains permitted because `0` explicitly disables volume expiry, matching `esp-group lifetime 0` |
| AC-7 | The two ends of an SA have out-of-sync byte/packet counters (due to packet loss, asymmetric thresholds, or peer counting differences) | Each end rekeys independently when its own local counter reaches its soft limit; when the peer rekeys first while Ze's local counter is still below Ze's threshold, Ze accepts `CREATE_CHILD_SA` via `respondChildRekey`, installs the replacement SA, resets `childLT` to zero, and continues carrying traffic; simultaneous rekeys resolve via RFC 7296 §2.8.1 nonce comparison (`RFC4301-4.4.2.1-3`) | None: rejecting or stalling on a peer's earlier rekey violates RFC4301-4.4.2.1-3 |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `life-bytes` (alongside `lifetime 3600`) on a site-to-site peer's `esp-group` and sends traffic across the tunnel | `ze-ipsec-conf.yang` -> `parseESPGroup` -> `ESPGroup.LifeBytes` -> `newChildLifetimeState` -> `maintainSA` `sampleChildVolume` (`dp.ListSAs`) -> `softExpired` -> `CREATE_CHILD_SA` rekey -> replacement XFRM SA installed | `test/ipsec/ipsec-life-volume.ci` and `test/interop-ipsec/scenarios/child-rekey-life-bytes` |
| 2 | Configures `life-packets` on an `esp-group` and sends packets across the tunnel | `ze-ipsec-conf.yang` -> `parseESPGroup` -> `ESPGroup.LifePackets` -> `newChildLifetimeState` -> `maintainSA` `sampleChildVolume` -> `softExpired` -> `CREATE_CHILD_SA` rekey | `test/ipsec/ipsec-life-volume.ci` |
| 3 | Leaves `life-bytes` and `life-packets` unset (or `0`) on an existing config | `ESPGroup.LifeBytes == 0 && LifePackets == 0` -> time-only `lifetimeState`, zero `ListSAs` polling per tick | `TestMaintainSAZeroListSAsForTimeOnly` + existing `test/ipsec/ipsec-child-rekey-xfrm.ci` |
| 4 | Attempts to commit a sub-minimum volume limit or a volume limit on VPP | YANG / `parseESPGroup` rejects sub-minimum; `verifyIPsecCommit` (`verifyVolumeLifetimes`) returns `opererr` naming group, backend, and fix | `TestESPGroupVolumeParse` + `TestVerifyVolumeLifetimesRefusesUnsupportedBackend` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestESPGroupVolumeParse` | `internal/component/ike/ipsec/config_test.go` | `life-bytes` and `life-packets` parse to `uint64`, defaults (`0`), minimums (`65536`, `100`), `uint64` max (`18446744073709551615`), sub-minimum rejection, overflow (`18446744073709551616`) rejection, and `ESPGroup.Equal` detects edits | |
| `TestLifetimeStateSoftAndHardBytes` | `internal/component/ike/engine/rfc4301_volume_lifetime_test.go` | `newChildLifetimeState` sets `softBytes` in `[90%, 95%]` of `hardBytes` (including `math.MaxUint64` without signed wrap); `softExpired` fires at `softBytes`, `hardExpired` fires at `hardBytes`; works with `lifetime 0` (volume-only) | |
| `TestLifetimeStateSoftAndHardPackets` | `internal/component/ike/engine/rfc4301_volume_lifetime_test.go` | `newChildLifetimeState` sets `softPackets` in `[90%, 95%]` of `hardPackets`; `softExpired` fires at `softPackets`, `hardExpired` fires at `hardPackets` | |
| `TestLifetimeSimultaneousFirstLimitWins` | `internal/component/ike/engine/rfc4301_volume_lifetime_test.go` | RFC4301-4.4.2.1-2 positive/negative: time, byte, and packet limits active simultaneously; earliest soft limit triggers rekey and earliest hard limit triggers termination | |
| `TestSampleChildVolumeUsesMaxDirection` | `internal/component/ike/engine/rfc4301_volume_lifetime_test.go` | `sampleChildVolume` reads outbound and inbound `SAInfo` via `dp.ListSAs(child.IfID)` matched by `IdentityOf`, takes `max(out, in)` per counter, ignores unrelated SAs, handles `ErrNotSupported` without panic or backward drift, and time-only `maintainSA` calls `ListSAs` zero times | |
| `TestVolumeRekeyOutOfSyncCounters` | `internal/component/ike/engine/rfc4301_volume_lifetime_test.go` | RFC4301-4.4.2.1-3 positive/negative: two ends with disagreeing counters — peer A hits its soft limit while peer B is below its limit and both complete `CREATE_CHILD_SA` cleanly with reset `childLT`; peer B rekeying first while A is below threshold is accepted by A with reset `childLT`; simultaneous collision resolves by lowest nonce | |
| `TestVerifyVolumeLifetimesRefusesUnsupportedBackend` | `internal/component/ike/engine/rfc4301_volume_lifetime_test.go` | `verifyVolumeLifetimes` refuses non-zero `life-bytes` or `life-packets` on a backend whose `SACountersSupported()` returns `ErrNotSupported` (or does not implement `SACounterChecker`) with an `opererr` naming group, backend, and fix; accepts `0` and accepts Linux XFRM | |
| `TestVPPSACountersSupportedRefuses` | `internal/component/ike/dataplane/vpp_test.go` | `vppBackend.SACountersSupported()` returns `ErrNotSupported`; `xfrmBackend` (Linux), `noopDataplane`, and `recordingDataplane` return `nil` | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `life-bytes` | `0 \| 65536..18446744073709551615` (`uint64`) | `18446744073709551615` (and `0` disabled, `65536` minimum enabled) | `1..65535` (boundary `1`, `65535`) and negative `-1` | `18446744073709551616` (rejected without wrapping) |
| `life-packets` | `0 \| 100..18446744073709551615` (`uint64`) | `18446744073709551615` (and `0` disabled, `100` minimum enabled) | `1..99` (boundary `1`, `99`) and negative `-1` | `18446744073709551616` (rejected without wrapping) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-life-volume` | `test/ipsec/ipsec-life-volume.ci` | Real Linux XFRM (`option=needs-linux:caps=net-admin,bpf`, `option=exclusive:group=ipsec-xfrm`): peer-a configures `lifetime 3600` alongside `life-bytes` (case 1) and `life-packets` (case 2); compiled Go fixture (`le test fixture ipsec/life-volume`) sends real packets matching peer-a's outbound XFRM policy until kernel `BytesCurrent`/`PacketsCurrent` reach the threshold, asserting SPI rotation and `CREATE_CHILD_SA` rekey long before the 3600s time lifetime; also checks `ze config validate` accepts `18446744073709551615` and rejects `65535`, `99`, and `18446744073709551616` | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `child-rekey-life-bytes` | `test/interop-ipsec/scenarios/child-rekey-life-bytes/` | strongSwan 5.9.14 | Simultaneous time (`lifetime 3600`) and byte (`life-bytes 131072`) lifetimes on Ze against strongSwan configured with a much larger lifetime (out-of-sync thresholds/counters): real tunnel traffic triggers Ze's volume-based `CREATE_CHILD_SA` rekey before time expiry, strongSwan installs the replacement Child SA and receives the old-SA Delete, SPIs rotate on both sides, and post-rekey tunnel traffic succeeds (`RFC4301-4.4.2.1-2`, `RFC4301-4.4.2.1-3`) | |

## Files to Modify
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - add `life-bytes` and `life-packets` `uint64` leaves under `esp-group` with native `range`, default `0`, `ze:help`, and `description`
- `internal/component/ike/ipsec/types.go` - add `LifeBytes uint64` and `LifePackets uint64` to `ESPGroup`
- `internal/component/ike/ipsec/config.go` - parse and validate `life-bytes` and `life-packets` in `parseESPGroup` (`minLifeBytes = 65536`, `minLifePackets = 100`, `uint64`)
- `internal/component/ike/ipsec/config_test.go` - add `TestESPGroupVolumeParse` and boundary cases
- `internal/component/ike/engine/rekey.go` - extend `lifetimeState` (`hardBytes`, `softPackets`, `hardPackets`, `packetCount`, `counterReadWarned`), add `newChildLifetimeState`, `volumeSoftThreshold`, `hasVolumeLimits`, `recordVolume`, update `softExpired` and `hardExpired`
- `internal/component/ike/engine/established.go` - use `newChildLifetimeState(ps.getESPGroup())` at all three Child SA lifetime sites, add `sampleChildVolume` on `maintainSA`'s 1-second tick when `childLT.hasVolumeLimits()`, log volume hard-expiry reason
- `internal/component/ike/engine/config_verify_commit.go` - call `verifyVolumeLifetimes(dp, dpName, cfg)` during `verifyIPsecCommit`
- `internal/component/ike/dataplane/dataplane.go` - declare `SACounterChecker` (`SACountersSupported() error`) capability interface
- `internal/component/ike/dataplane/xfrm_linux.go` - implement `SACountersSupported() error` returning `nil`
- `internal/component/ike/dataplane/xfrm_other.go` - implement `SACountersSupported() error` returning `nil` (matching `SAOptionsSupported`)
- `internal/component/ike/dataplane/vpp.go` - implement `SACountersSupported() error` returning `ErrNotSupported`
- `internal/component/ike/dataplane/noop.go` - implement `SACountersSupported() error` returning `nil` (matching `SAOptionsSupported`)
- `internal/component/ike/dataplane/recording_zetest.go` - implement `SACountersSupported() error` returning `nil` (matching `SAOptionsSupported`)
- `internal/le/interoplab/ipsec/checkers.go` - register `child-rekey-life-bytes` in `scenarioCheckers`
- `docs/architecture/ike/ipsec-3-data-model.md` - document `ESPGroup.LifeBytes` and `LifePackets`
- `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - document `sampleChildVolume` via `ListSAs` on the 1-second tick and `SACounterChecker`
- `docs/architecture/ike/ipsec-13-rekey-wire.md` - document volume soft/hard triggers alongside time triggers in `lifetimeState`
- `docs/guide/ipsec.md` - update the `esp-group` config reference and lifetime table (replacing the "Ze carries no byte lifetime" note)
- `docs/guide/configuration.md` - document `life-bytes` and `life-packets` under `vpn ipsec esp-group`
- `rfc/short/rfc4301.md` - update `RFC4301-4.4.2.1-2` and `RFC4301-4.4.2.1-3` once enforced and recorded
- `rfc/short/rfc7296.md` - add source/test links for volume-triggered Child SA rekey under Section 2.8
- `features/ipsec-data-model.md` - note volume lifetime leaves under `esp-group`

## Files to Create
- `internal/component/ike/engine/rfc4301_volume_lifetime_test.go` - unit and RFC-tagged discrimination tests for byte/packet soft & hard expiry, simultaneous time+volume limits, `sampleChildVolume` max-direction feed, out-of-sync counters (`RFC4301-4.4.2.1-2`, `RFC4301-4.4.2.1-3`), and `verifyVolumeLifetimes`
- `internal/test/fixture/register_ipsec_life_volume.go` - register `ipsec/life-volume` functional test fixture
- `internal/test/fixture/tunnel_fixture_ipsec_life_volume.go` - compiled Go fixture driving real XFRM traffic and asserting SPI rotation on volume rekey
- `test/ipsec/ipsec-life-volume.ci` - real-XFRM functional wiring test for `life-bytes`, `life-packets`, and `ze config validate` boundary checks
- `internal/le/interoplab/ipsec/lifevolume.go` - strongSwan interop checker `checkChildRekeyLifeBytes`
- `test/interop-ipsec/scenarios/child-rekey-life-bytes/ze.conf` - Ze config with simultaneous `lifetime 3600` and `life-bytes 131072`
- `test/interop-ipsec/scenarios/child-rekey-life-bytes/swanctl.conf` - strongSwan peer config with longer lifetime/volume threshold (out-of-sync counter proof)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`: `life-bytes` and `life-packets` leaves under `esp-group`, both with `ze:help` (≤96 chars / ≤25 words) and distinct `description` (`ai/rules/config.md`) |
| YANG validation constraints | Yes | Native `uint64` ranges `"0 \| 65536..18446744073709551615"` and `"0 \| 100..18446744073709551615"` with default `0`, mirrored in `parseESPGroup` |
| YANG custom validators | No | Native YANG `range` and `uint64` type fully express the bounds; backend capability check runs in `verifyIPsecCommit` |
| CLI commands/flags | No | No new CLI verb; `show vpn ipsec sa` and `show vpn ipsec dataplane` already report SA byte/packet counters and SPIs |
| CLI grammar (keyword before value) | N-A | No new CLI command added |
| Editor autocomplete | Yes | Automatic from `ze-ipsec-conf.yang` leaf declarations and `ze:help` |
| Functional test for new RPC/API | Yes | `test/ipsec/ipsec-life-volume.ci` proves config validation, real XFRM counter feed, and runtime volume rekey |
| Pipe completeness | N-A | No new show/RPC output added |
| Env var registration | No | Pure YANG config leaves, no env var added (`ai/rules/config.md`) |
| Doctor check for runtime dependencies | No | Reuses existing `NETLINK_XFRM` dataplane (`dp.ListSAs`) already covered by existing IPsec doctor checks; no new file path, socket, kernel module, or binary |
| Doctor code per problem class | No | Commit-time refusal on counterless backends is an `opererr` returned directly to the operator at commit; runtime hard-expiry reuses existing `child-sa: hard lifetime expired` lifecycle exit |
| Prometheus counters/metrics | No | Existing `ze_ipsec_rekey_total{peer}` (`internal/component/ike/engine/metrics.go`) counts every Child SA rekey via `ps.incRekeyCount()` regardless of time/byte/packet trigger |
| BGP family surface (new SAFI / capability / attribute) | N-A | IPsec/IKE subsystem, not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/ipsec-data-model.md` (and re-render `docs/features.md` if needed) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` (new `life-bytes` and `life-packets` leaves under `vpn ipsec esp-group`); `docs/architecture/config/syntax.md` unaffected (generic grammar unchanged) |
| 3 | CLI command added/changed? | No | No CLI command added or changed |
| 4 | API/RPC added/changed? | No | No API/RPC signature changed |
| 5 | Plugin added/changed? | No | No new plugin added; existing `ike` plugin extended internally |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md` (update `esp-group` leaves and lifetime table line 1187) |
| 7 | Wire format changed? | No | RFC 7296 §2.8 carries no lifetime on the IKEv2 wire |
| 8 | Plugin SDK/protocol changed? | No | Uses existing `vpn` config section JSON delivery unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4301.md` (`RFC4301-4.4.2.1-2`, `RFC4301-4.4.2.1-3`), `rfc/short/rfc7296.md` (§2.8), and `docs/features/rfc-status.md` if status rows change |
| 10 | Test infrastructure changed? | No | Uses existing `.ci` runner and `interoplab/ipsec` registry unchanged |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` makes no claim about IPsec volume lifetimes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-3-data-model.md`, `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, `docs/architecture/ike/ipsec-13-rekey-wire.md` |
| 13 | Route metadata keys added/changed? | N-A | No BGP route metadata touched |
| 14 | Prometheus counters added/changed? | No | Existing `ze_ipsec_rekey_total{peer}` already covers volume rekeys |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No new plugin, event type, command, or inventory field registered |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Checked via `./le spec citation anchors`: `docs/architecture/ike/ipsec-3-data-model.md`, `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, `docs/architecture/ike/ipsec-13-rekey-wire.md`, and `docs/guide/ipsec.md` are updated; unaffected pages: `docs/architecture/ike/ipsec-7-ikev2-engine.md` (reload uses `ESPGroup.Equal` unchanged), `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` (EAP/NAT traversal and `rebindRekeyOwed` unchanged), `docs/architecture/ike/ipsec-11-interop-eap.md` (EAP initiator and EAP interop scenarios unchanged), `docs/architecture/ike/ipsec-14-responder.md` (responder role and handshake unchanged), `docs/architecture/ike/ipsec-dataplane-inspection.md` (`ListSAs` counter readback and `ErrNotSupported` contract unchanged), `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` (narrowing/transport mode unchanged), `docs/architecture/linux-limitations.md` (no new Linux kernel limitation introduced), `docs/architecture/testing/interop.md` (existing IPsec interop suite infrastructure unchanged), `docs/config-reference.md` (references `config.go` for general IPsec config, no `esp-group` lifetime table), `docs/guide/migrating-from-vyos.md` (covers VyOS `pfs`/encryption migration only) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verified `docs/guide/ipsec.md` and `docs/guide/configuration.md`; `docs/guide/ipsec.md` line 1187 explicitly states "Ze carries no byte lifetime" and will be updated in the same change |

## Design presentation

### What changes

Operators can configure `life-bytes` (`0 | 65536..18446744073709551615`, default `0`) and `life-packets` (`0 | 100..18446744073709551615`, default `0`) under `vpn ipsec esp-group <name>` alongside the existing time `lifetime`. When a Child SA has volume limits configured, `maintainSA` polls the active Child SA's inbound and outbound 64-bit counters from the existing `dataplane.Dataplane.ListSAs` interface on its 1-second tick, takes the per-direction maximum (`max(out, in)`), and feeds `lifetimeState`. Whichever soft limit (time, bytes, or packets) is reached first triggers the existing make-before-break `CREATE_CHILD_SA` rekey; whichever hard limit is reached first tears the Child SA down unconditionally (RFC 7296 Section 2.8). Tunnels with no volume limits pay zero `ListSAs` overhead, backends that do not expose SA counters (such as VPP) refuse volume limits at commit verify with an `opererr` naming the fix, and peers with out-of-sync counters interwork cleanly (satisfying `RFC4301-4.4.2.1-2` and `RFC4301-4.4.2.1-3`).

### Before and after

1. **Configuring simultaneous time and byte/packet lifetimes on an `esp-group`:**
   - Before: `set vpn ipsec esp-group site-b-esp life-bytes 1073741824` and `set vpn ipsec esp-group site-b-esp life-packets 1000000` are rejected as unknown leaves; `docs/guide/ipsec.md` reports that Ze carries no byte lifetime and cannot satisfy RFC 4301 Section 4.4.2.1's simultaneous lifetime requirement.
   - After: Both leaves validate as `uint64` (rejecting non-zero values below `65536` bytes or `100` packets and rejecting `18446744073709551616` without wrapping), load into `ESPGroup`, and trigger `CREATE_CHILD_SA` rekey when either time, bytes, or packets reaches its soft threshold first.
2. **High-throughput tunnel reaching its byte or packet ceiling before time expiry:**
   - Before: `lifetimeState.softBytes` is never assigned and `byteCount` is never fed, so a tunnel with `lifetime 3600` never rekeys until the 3600-second time soft threshold arrives regardless of traffic volume.
   - After: `maintainSA` reads `SAInfo.BytesCurrent` and `SAInfo.PacketsCurrent` from Linux XFRM via `dp.ListSAs(child.IfID)` on its 1-second tick; once `max(out, in)` reaches the randomized soft threshold (5%–10% ahead of the configured ceiling), Ze initiates `CREATE_CHILD_SA` rekey, rotates SPIs make-before-break, and resets the new Child SA's volume counters to zero.
3. **Committing volume lifetimes on a counterless dataplane backend (VPP):**
   - Before: No volume leaves exist; naively adding them without a backend capability check would silently accept `life-bytes` on VPP where `ListSAs` returns `ErrNotSupported`, so volume rekey would never fire.
   - After: `verifyIPsecCommit` calls `verifyVolumeLifetimes` -> `SACounterChecker.SACountersSupported()` and refuses the commit with an `opererr` naming the `esp-group`, the `vpp` backend, why volume lifetimes cannot be enforced there, and the exact delete command or XFRM backend alternative.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Program Linux XFRM kernel `XfrmStateLimits` (`ByteSoft`/`ByteHard`/`PacketSoft`/`PacketHard`) and add a netlink `XFRM_MSG_EXPIRE` listener | Kernel XFRM reserves `^uint64(0)` (`18446744073709551615`) as `XFRM_INF` (unlimited), which conflicts with AC-4's full `uint64` max; kernel hard expiry deletes states asynchronously outside `maintainSA`, causing dataplane drift; and `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` already standardizes on one 1-second ticker driving all SA lifetimes. |
| Sum outbound + inbound counters (`out + in`) instead of `max(out, in)` | Each direction of a Child SA pair has its own cryptographic key (`EncryptKeyI` vs `EncryptKeyR`) and its own RFC 4301 SAD entry; summing would halve per-key limits on symmetric traffic, diverge from strongSwan/XFRM per-SA semantics, and risk `uint64` addition overflow on large counters. |
| Use `[0, 10%)` jitter for volume soft threshold (mirroring `lifetimeJitter` without a floor) | Unlike time lifetimes, volume lifetimes have no retransmit-time budget floor in bytes/packets; a 0% draw would put `softBytes == hardBytes`, leaving zero headroom for make-before-break `CREATE_CHILD_SA` before hard expiry. A 5%–10% randomized margin guarantees headroom while still desynchronizing peers. |
| Put the functional test at `test/plugin/ipsec-life-bytes.ci` and interop scenario at `test/interop/scenarios/NN-ipsec-life-bytes-peer` | Both paths were from before IPsec's own test suites were organized: real-XFRM IPsec `.ci` tests live in `test/ipsec/` (`./le test functional ipsec`), and IPsec strongSwan scenarios live in `test/interop-ipsec/scenarios/` with descriptive names and no numeric prefix (`docs/architecture/testing/interop.md`). |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- add `life-bytes` and `life-packets` to `ze-ipsec-conf.yang`, `ESPGroup` (`types.go`), `parseESPGroup` (`config.go`), `SACounterChecker` (`dataplane/*.go`), and `verifyVolumeLifetimes` (`config_verify_commit.go`); wire `newChildLifetimeState` into `established.go`; create failing `test/ipsec/ipsec-life-volume.ci` and `internal/test/fixture/*ipsec_life_volume.go`.
   - Tests: `TestESPGroupVolumeParse`, `TestVerifyVolumeLifetimesRefusesUnsupportedBackend`, `TestVPPSACountersSupportedRefuses`, `test/ipsec/ipsec-life-volume.ci`
   - Files: `ze-ipsec-conf.yang`, `types.go`, `config.go`, `dataplane.go`, `xfrm_linux.go`, `xfrm_other.go`, `vpp.go`, `noop.go`, `recording_zetest.go`, `config_verify_commit.go`, `rekey.go`, `established.go`
   - Verify: config parsing, YANG validation, and backend capability checks pass; wiring `.ci` fails until counter sampling is wired.
2. **Phase: Volume lifetime logic and counter feed** -- implement `volumeSoftThreshold` (using `big.Int.SetUint64`), `newChildLifetimeState`, `hasVolumeLimits`, `recordVolume`, updated `softExpired`/`hardExpired` in `rekey.go`, and `sampleChildVolume` in `established.go` (`max(out, in)` from `dp.ListSAs(child.IfID)` on the 1-second tick only when `hasVolumeLimits()` is true).
   - Tests: `TestLifetimeStateSoftAndHardBytes`, `TestLifetimeStateSoftAndHardPackets`, `TestLifetimeSimultaneousFirstLimitWins`, `TestSampleChildVolumeUsesMaxDirection`, `TestVolumeRekeyOutOfSyncCounters`
   - Files: `internal/component/ike/engine/rekey.go`, `internal/component/ike/engine/established.go`, `internal/component/ike/engine/rfc4301_volume_lifetime_test.go`
   - Verify: all unit tests fail before implementation and pass after; `test/ipsec/ipsec-life-volume.ci` turns green in QEMU.
3. **Phase: Interop test and RFC records** -- add `test/interop-ipsec/scenarios/child-rekey-life-bytes/` (`ze.conf`, `swanctl.conf`) and `internal/le/interoplab/ipsec/lifevolume.go`, register in `checkers.go`, add `// RFC 4301 Section 4.4.2.1` and `// RFC 7296 Section 2.8` comments above enforcing code, and record RFC discrimination records for `RFC4301-4.4.2.1-2` and `RFC4301-4.4.2.1-3`.
   - Tests: `./le test integration interop-ipsec` (scenario `child-rekey-life-bytes`), `./le rfc record` + `./le rfc check`
   - Files: `internal/le/interoplab/ipsec/checkers.go`, `internal/le/interoplab/ipsec/lifevolume.go`, `test/interop-ipsec/scenarios/child-rekey-life-bytes/*`, `rfc/short/rfc4301.md`, `rfc/short/rfc7296.md`
4. **Phase: Documentation and scoped verification** -- update `ipsec-3-data-model.md`, `ipsec-8-ikev2-child-xfrm.md`, `ipsec-13-rekey-wire.md`, `docs/guide/ipsec.md`, `docs/guide/configuration.md`, and `features/ipsec-data-model.md`; run scoped main-thread verification commands:
   - `./le test unit pkg path ./internal/component/ike/ipsec/`
   - `./le test unit pkg path ./internal/component/ike/engine/`
   - `./le test unit pkg path ./internal/component/ike/dataplane/`
   - `./le doc yang-contract help-shape`
   - `./le config claims`
   - `./le config coercion check`
   - `./le test functional ipsec` (in QEMU for `needs-linux:caps=net-admin,bpf`)
   - `./le test integration interop-ipsec` (scenario `child-rekey-life-bytes`)
   - `./le rfc check`
   - `./le doc check all`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-7 implemented with file:line and proven by test |
| Feature completeness | Every user story has a working path from YANG config through `ListSAs` counter sampling to `CREATE_CHILD_SA` rekey |
| Correctness | `uint64` end-to-end with no 32-bit truncation and no `int64` wrap in `volumeSoftThreshold` at `math.MaxUint64`; earliest of time/byte/packet triggers; `max(out, in)` per-direction semantics; `childLT` resets cleanly on both local-initiated and peer-initiated Child SA rekeys |
| Naming | YANG leaves `life-bytes` and `life-packets` map directly to `ESPGroup.LifeBytes` and `ESPGroup.LifePackets`; `ze:help` fits 96 chars / 25 words with distinct `description` |
| Data flow | Time-only tunnels (`LifeBytes == 0 && LifePackets == 0`) execute zero `ListSAs` calls per tick; volume-enabled tunnels feed real `SAInfo` counters into `lifetimeState` |
| Operator errors and doctor codes | `verifyVolumeLifetimes` returns an `opererr` naming the `esp-group`, backend, reason, and fix command; `parseESPGroup` errors name group, leaf, offending value, and valid range |
| Rule: protocol / exact-or-reject | Counterless backends (VPP) refuse `life-bytes`/`life-packets` at `verifyIPsecCommit` rather than silently ignoring them |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| YANG + Go `uint64` volume config & boundaries | `./le test unit pkg path ./internal/component/ike/ipsec/ run TestESPGroupVolumeParse` and `./le doc yang-contract help-shape` |
| Volume lifetime engine + out-of-sync counters + backend refusal | `./le test unit pkg path ./internal/component/ike/engine/ run 'TestLifetime\|TestSampleChildVolume\|TestVolumeRekey\|TestVerifyVolumeLifetimes'` |
| VPP/XFRM `SACounterChecker` | `./le test unit pkg path ./internal/component/ike/dataplane/ run SACountersSupported` |
| Real-XFRM functional wiring test | `./le test functional ipsec` (`test/ipsec/ipsec-life-volume.ci` in QEMU) |
| strongSwan interop proof | `./le test integration interop-ipsec` (`child-rekey-life-bytes`) |
| RFC discrimination records and doc anchors | `./le rfc check` and `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Crypto lifetime enforcement | Volume soft trigger fires before hard ceiling; hard byte/packet expiry tears down the SA unconditionally even if rekey is in flight (RFC 7296 §2.8) |
| Input validation | `life-bytes` (`0 \| 65536..18446744073709551615`) and `life-packets` (`0 \| 100..18446744073709551615`) reject tiny values (preventing rekey-storm DoS) and reject `uint64` overflow without wrapping |
| Fail-closed backend check | Backends that cannot report SA counters (VPP or any backend not implementing `SACounterChecker`) refuse volume lifetimes at commit verify rather than silently leaving keys unrotated |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read source in Current Behavior; if misunderstood, return to RESEARCH |
| Lint failure | Fix inline; if architectural, return to DESIGN |
| Functional or interop test fails | Check the AC: wrong AC -> DESIGN, correct AC -> IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches and ask the user |

## Design Insights
<!-- LIVE: route lessons to their governing surface under ai/rules/planning.md. -->

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Feed `lifetimeState` from `dp.ListSAs(child.IfID)` on `maintainSA`'s existing 1-second ticker | Program Linux XFRM `XfrmStateLimits` and add a netlink `XFRM_MSG_EXPIRE` goroutine | Reuses already-shipped 64-bit `SAInfo` counter readback and single-ticker design; avoids `XFRM_INF == math.MaxUint64` collision (AC-4) and async kernel-expiry dataplane drift |
| Track `max(out, in)` across outbound and inbound SA states | Sum `out + in` | Matches RFC 4301 per-unidirectional-SA key lifetimes and strongSwan/XFRM per-state limits; immune to `uint64` addition overflow |
| Minimums `65536` bytes and `100` packets, `0` disabled, `uint64` max `18446744073709551615`, 5%–10% randomized soft lead | Fixed 10% lead, `[0, 10%)` jitter, or larger minimums like 1 MiB | Prevents tiny-value rekey storms while keeping real-XFRM functional and interop runs fast; 5% minimum lead guarantees make-before-break headroom and 5%–10% jitter desynchronizes peers |
| Commit-time `SACounterChecker` refusal on counterless backends (VPP) | Silently accept volume limits on VPP or probe `ListSAs(0)` during `OnConfigVerify` | Follows `CatchAllInstaller` / `SAOptionChecker` pattern: pure capability check with no netlink dump on verify, enforcing exact-or-reject (`ai/rules/protocol.md`) |

## Known Limitations
- VPP dataplane does not expose SA byte/packet counters through `ListSAs` today, so `life-bytes` and `life-packets` are refused at commit verify when the VPP dataplane is active (`docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`). Enabling volume lifetimes on VPP belongs to VPP SAD readback work once `vppBackend.ListSAs` is implemented.

## RFC Documentation (Scope: protocol)

Add `// RFC 4301 Section 4.4.2.1: "<quoted requirement>"` and `// RFC 7296 Section 2.8: "<quoted requirement>"` comments above enforcing code in `rekey.go`, `established.go`, and `config.go`.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 4301 Section 4.4.2.1 (`RFC4301-4.4.2.1-2`) | Default `esp-group` runs `lifetime 3600` (time-only); adding `life-bytes` and/or `life-packets` alongside `lifetime` enforces both time and volume lifetimes simultaneously, triggering rekey on whichever soft limit hits first and terminating on whichever hard limit hits first | `lifetime 0` with non-zero `life-bytes`/`life-packets` (volume-only), and `life-bytes 0` / `life-packets 0` (time-only) | `TestLifetimeSimultaneousFirstLimitWins`, `test/ipsec/ipsec-life-volume.ci`, `child-rekey-life-bytes` | `TestLifetimeStateSoftAndHardBytes` (`lifetime 0`), `TestSampleChildVolumeUsesMaxDirection` (zero `ListSAs` when time-only) |
| RFC 4301 Section 4.4.2.1 (`RFC4301-4.4.2.1-3`) | Each end tracks its own local SA counters; a peer that rekeys first (despite Ze's local counter being below threshold) is accepted cleanly via `respondChildRekey` with `childLT` reset, and simultaneous rekeys converge via RFC 7296 §2.8.1 nonce tie-breaking | Independent local `life-bytes` / `life-packets` configuration per peer (`RFC7296-2.8-2`) | `TestVolumeRekeyOutOfSyncCounters`, `child-rekey-life-bytes` | `TestVolumeRekeyOutOfSyncCounters` (both directions and simultaneous collision) |
| RFC 7296 Section 2.8 (`RFC7296-2.8-2`, `RFC7296-2.8-3`, `RFC7296-2.8-8`) | Lifetimes remain strictly local policy (not negotiated on wire); soft volume trigger is jittered 5%–10% ahead of hard limit; hard byte/packet expiry tears down the SA unconditionally | Existing `lifetime` time-based jitter and retransmit-budget floor (`rekeyLead`) | `TestLifetimeStateSoftAndHardBytes`, `TestLifetimeStateSoftAndHardPackets` | Existing `TestSALifetimeTime`, `TestRFC4301SADLifetimeIndicatesReplaceBeforeTerminate` |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate over the complete diff. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Scope, Priority, Release reason, RFC gaps, Depends, Phase, Handoff and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced (`rfc/short/rfc4301.md`, `rfc/short/rfc7296.md`)
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior, Existing Mechanisms, Design documents, Data Flow, Changed-Wire Census, and Design presentation sections completed
- [ ] AC-N rows carry testable positive behavior and Permitted Survivors
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist and Documentation Update Checklist answered Yes / No / N-A with evidence

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted tests pass in the main checkout run by the main thread (`./le test unit pkg path` for `ipsec`, `engine`, `dataplane`, `./le test functional ipsec` in QEMU, `./le test integration interop-ipsec` for `child-rekey-life-bytes`, `./le doc yang-contract help-shape`, `./le config claims`, `./le config coercion check`, `./le rfc check`, `./le doc check all`)
- [ ] Feature code integrated (`internal/component/ike/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes and `/ze-review` gate clean
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] RFC discrimination records written and `./le rfc check` green

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (`life-bytes`, `life-packets`)
- [ ] Functional `.ci` test (`test/ipsec/ipsec-life-volume.ci`) for end-to-end behavior
- [ ] Interop test (`child-rekey-life-bytes` against strongSwan) for protocol behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-ipsec-lifetime-volume.md` only, in the same `./le commit create` script
