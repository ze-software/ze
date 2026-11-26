# Spec: RSVP-TE non-RSVP hop detection and traffic-control signaling (RFC 2205 Section 3.8)

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/templates/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1. -->

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Shipped RSVP-TE raw-IP transport already captures received IPv4 TTL, but the signaling engine never compares it against RSVP `Send_TTL` or notifies traffic control (`TC_Advertise` / ADSPEC Global Break bit) when a PATH traverses a non-RSVP IP router (`RFC2205-3-30`). |
| RFC gaps | RFC2205-3-30 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

| Requirements | Decision | Owning spec |
|--------------|----------|-------------|
| `RFC2205-3-30` (TTL vs `Send_TTL` non-RSVP hop detection and passing to traffic control) | Keep in `plan/release/fixes/` and implement for release 1. | `plan/release/fixes/spec-rsvpte-non-rsvp-neighbors.md` (this spec) |
| `RFC3209-4.2.5-1` (non-RSVP neighbor tracking and `LABEL_REQUEST` suppression) and `RFC2205-3-31` (routing-protocol IP-tunnel notification) | Split out and move both unstarted capabilities to `plan/post-release/features/`. | `plan/post-release/features/spec-rsvpte-non-rsvp-clouds.md` |

## Task

RFC 2205 Section 3.8 (`RFC2205-3-30`) requires:

> "RSVP must also test for the presence of non-RSVP hops in the path and pass this information to traffic control. From this flag bit that the RSVP process supplies and from its own local knowledge, traffic control can detect the presence of a hop in the path that is not capable of QoS control, and it passes this information to the receivers in Adspecs [RFC 2210]. With normal IP forwarding, RSVP can detect a non-RSVP hop by comparing the IP TTL with which a Path message is sent to the TTL with which it is received; for this purpose, the transmission TTL is placed in the common header."

RFC 2205 Section 3.11.2 defines the traffic-control OPWA call `TC_Advertise(Interface, Adspec, Non_RSVP_Hop_flag) -> New_Adspec`, and RFC 2210 Section 3.3.2 specifies that traffic control records the non-RSVP hop by setting the Global Break bit (bit 23 of word 1 of the Default General Parameters fragment, `0x80` at byte 9 of the ADSPEC object). RFC 2210 Section 2.3.1 specifies that when the Global Break bit is set, all ADSPEC parameters (including composed Path MTU) are unreliable.

Today, Ze's Linux raw transport (`internal/plugins/rsvpte/transport_linux.go::receivedIPv4Packet`) already extracts the arriving IPv4 header TTL into `Packet.TTL`, and `wire.go::DecodeHeader` extracts the RSVP Common Header `Send_TTL` into `ParsedMessage.Header.TTL`. However, `engine.go::handlePacket` discards `pkt.TTL` before calling `handlePath`, so:

1. Neither transit nor egress compares `Packet.TTL` against `msg.Header.TTL` (`Send_TTL`).
2. `mtu.go::updateAdspec` (Ze's `TC_Advertise` producer) has no `nonRSVPHop` parameter and never sets the ADSPEC Global Break bit when a PATH crossed a non-RSVP IP router.
3. A transit node relays the ADSPEC with the Global Break bit still clear, and checks/decrements only `msg.Header.TTL` (allowing non-RSVP hops that burned IP TTL down to 1 to be re-inflated on relay).
4. An egress node trusts `msg.PathMTU` from an ADSPEC whose immediately preceding hop crossed a non-RSVP router, echoing that unreliable Path MTU in its RESV FLOWSPEC and installing it on the egress pop.
5. FRR local repair (`frr.go::backupPath`) and merge-point ADSPEC recomputation (`frr.go::recomputeMergedAdspecLocked`) do not carry per-branch `NonRSVPHop` state, so a repair or merge would drop the Global Break bit.

**Goal:** Compare received IPv4 TTL (`Packet.TTL`) against RSVP `Send_TTL` (`Header.TTL`) on every received PATH, pass `Non_RSVP_Hop_flag` into Ze's traffic-control ADSPEC composition (`updateAdspec` at transit/FRR and `markAdspecNonRSVPHop` at egress/merge), prevent transit TTL re-inflation across non-RSVP hops, preserve exact direct-hop behavior when `Packet.TTL == Send_TTL`, and prove the end-to-end behavior in both unit/fabric tests and a real Linux network-namespace test with an intermediate non-RSVP IPv4 forwarding router.

| Requirement | RFC text | Producer or defect |
|-------------|----------|--------------------|
| RFC2205-3-30 | "RSVP must also test for the presence of non-RSVP hops in the path and pass this information to traffic control." (§3.8) | Defect: `transport_linux.go::receivedIPv4Packet` populates `Packet.TTL` and `wire.go::DecodeHeader` populates `Header.TTL`, but `engine.go::handlePacket` drops `pkt.TTL` before `handlePath`, and `mtu.go::updateAdspec` accepts no `nonRSVPHop` flag. |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform boundaries, cross-process plugin contracts, shared machinery
  → Decision: RSVP-TE is a self-contained edge plugin; no core/shared package changes are needed.
  → Constraint: Protocol peers rely on the RFC contract (`RFC 2205`, `RFC 2210`, `RFC 3209`).
- [ ] `ai/doors/routing-protocols.md` - RSVP-TE boundaries, RFC quoting and discrimination record gates, Linux namespace and freeRouter interop gates
  → Decision: Keep all non-RSVP hop detection and OPWA ADSPEC composition inside `internal/plugins/rsvpte/`; no netlink route writes or cross-plugin calls.
  → Constraint: Quote the RFC above enforcing code (`RFC 2205 Section 3.8`, `Section 3.11.2`, `RFC 2210 Section 3.3.2`) and land `RFC2205-3-30` positive/negative discrimination records with the implementation.
- [ ] `docs/architecture/rsvpte/mpls-rsvp-te.md` - RSVP-TE signaling engine, ADSPEC Path MTU floor decision, raw IP transport
  → Decision: A path MTU below `mplsfib.PathMTUMinimum` is refused only when a usable ADSPEC Path MTU exists (`received != 0`); an ADSPEC with the Global Break bit set reports `adspecPathMTU == 0` (unknown/unreliable) and installs no MTU metric.
  → Constraint: Preserve `ReceivedAdspec` unmodified for `PathErr` reporting upstream (`RFC 2205 Section 3.1.7`), and start every ADSPEC update from the received bytes so refresh ticks do not accumulate state.
- [ ] `docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md` - FRR PLR backup PATH construction and merge-point ADSPEC recomputation
  → Decision: `backupPath` rebuilds the backup ADSPEC from `ReceivedAdspec`, and `recomputeMergedAdspecLocked` recomputes the merged ADSPEC from all live branches on join, refresh, tear and expiry.
  → Constraint: Both paths must carry `NonRSVPHop` so local repair and merge points neither drop a live break bit nor retain a stale one after a non-RSVP branch departs.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2205.md` and `rfc/full/rfc2205.txt` (Sections 2.9, 3.1.1, 3.8, 3.11.2, 3.11.5) - `Send_TTL`, non-RSVP hop detection, `TC_Advertise(Interface, Adspec, Non_RSVP_Hop_flag)`
  → Constraint: On normal IPv4 forwarding, a direct RSVP hop arrives with `IP TTL == Send_TTL`; any intermediate non-RSVP IP router decrements IP TTL without altering `Send_TTL`, yielding `IP TTL < Send_TTL`.
- [ ] `rfc/full/rfc2210.txt` (Sections 2.3.1, 3.3.2) - ADSPEC Global Break bit (bit 23 of word 1 of the Default General Parameters fragment)
  → Constraint: Setting the Global Break bit (`0x80` at byte 9 of the ADSPEC object) notifies downstream RSVP nodes and the receiver that at least one hop did not support RSVP/IntServ and that all ADSPEC parameters are unreliable.

**Key insights:**
- On Linux, `rawTransport.openSockets` sets `IP_ROUTER_ALERT`, so `ip_forward()` delivers transit PATH packets to `ip_call_ra_chain()` BEFORE `ip_decrease_ttl()` (proven by `TestIntegrationRSVPPathCarriage` asserting `packet.TTL == msg.payload[4]`). An intermediate non-RSVP router has no `IP_ROUTER_ALERT` socket, so `ip_forward()` runs `ip_decrease_ttl()` and delivers `Packet.TTL == Send_TTL - k` (`k >= 1`) to the next RSVP node.
- Many existing unit tests construct synthetic `Packet{Src: ..., Payload: ...}` literals with `Packet.TTL == 0`. Because normal IPv4 forwarding never delivers a packet with IP TTL 0 (routers drop `TTL <= 1` before forwarding), treating `Packet.TTL == 0` as "IP TTL not provided by test transport" avoids false positives on existing synthetic unit tests while `fabricPort.SendPath` is updated to populate `Packet.TTL` from `msg[4]`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/rsvpte/transport.go` - defines `Packet{Src, Dst, IfIndex, TTL, Payload}` and `Transport`.
- [ ] `internal/plugins/rsvpte/transport_linux.go` - `SendPath` writes `header[8] = msg[4]` (`Send_TTL`); `receivedIPv4Packet` writes `Packet.TTL = data[8]`.
- [ ] `internal/plugins/rsvpte/wire.go` - `DecodeHeader` parses byte 4 into `Header.TTL`; `DecodeMessage` validates ADSPEC and computes `msg.PathMTU = adspecPathMTU(msg.AdspecRaw, msg.SenderTSpec.Service)` before `Packet.TTL` is seen.
- [ ] `internal/plugins/rsvpte/mtu.go` - `updateAdspec(raw, outgoingMTU)` sets `raw[9] |= intservBreak` only when `outgoingMTU == 0`; `adspecPathMTU` returns `0` whenever `raw[off+1]&intservBreak != 0`.
- [ ] `internal/plugins/rsvpte/engine.go` - `handlePacket` discards `pkt.TTL` and calls `handlePath(peer, msg)`; `handlePath` checks `onwardPathMTU(msg.PathMTU, selected.Route.MTU)` against the floor; `handlePathEgress` uses `msg.PathMTU` directly for `rsb.PathMTU` and `flow.MaxPacketSize`; `handlePathTransit` checks `msg.Header.TTL <= 1`, calls `updateAdspec(lsp.PSB.Adspec, selected.Route.MTU)`, and builds the relayed PATH with `msg.Header.TTL - 1`.
- [ ] `internal/plugins/rsvpte/fsm.go` - `pathStateBlock` and `mergedPath` store `Adspec` and `ReceivedAdspec` with no `NonRSVPHop` flag.
- [ ] `internal/plugins/rsvpte/frr.go` - `backupPath` rebuilds `backup.Adspec` from `psb.ReceivedAdspec` via `updateAdspec(backup.Adspec, bypassMTU)`; `mergeBackupPath` checks `msg.Header.TTL <= 1` and stores `mergedPath`; `recomputeMergedAdspecLocked` picks minimum `adspecPathMTU` across branches and calls `updateAdspec(psb.Adspec, psb.Route.MTU)` for transit.
- [ ] `internal/plugins/rsvpte/reservation.go` - `receivedPathState` builds a fresh PSB per PATH; `reservationPathMTU` reads `adspecPathMTU(lsp.PSB.Adspec, ...)`, which already returns `0` when the Global Break bit is set.
- [ ] `internal/plugins/rsvpte/build.go` - `buildPath` emits `psb.Adspec` as opaque bytes and writes `ttl` to byte 4 (`Send_TTL`); `buildPathErr` emits `ReceivedAdspec` unmodified.
- [ ] `internal/plugins/rsvpte/interop_test.go` - `fabricPort.SendPath` delivers `Packet` without populating `TTL`.

**Behavior to preserve:**
- Direct RSVP hops (`Packet.TTL == msg.Header.TTL`, and synthetic unit tests leaving `Packet.TTL == 0`): ADSPEC Global Break bit stays clear, composed Path MTU flows to egress and back in RESV/FIB, path-MTU floor refusal (`PathErr 21/05`) triggers on below-floor composed MTUs, and transit decrements `Send_TTL` by 1.
- `ReceivedAdspec` stays unmodified for upstream `PathErr` messages (`RFC 2205 Section 3.1.7`).
- PATH messages carrying no ADSPEC do not have a synthetic ADSPEC fabricated on transit or egress.
- Existing `show rsvp-te ...` JSON schemas, YANG config schema, and `buildPath` `LABEL_REQUEST` behavior remain unchanged.

**Behavior to change:**
- Pass `pkt.TTL` from `handlePacket` into `handlePath`, `handlePathEgress`, `handlePathTransit`, and `mergeBackupPath`.
- Detect non-RSVP hops via `nonRSVPHopDetected(receivedTTL, sendTTL)` (`receivedTTL != 0 && receivedTTL != sendTTL`) and bound hop budget via `effectivePathTTL(receivedTTL, sendTTL)` (`sendTTL` when `receivedTTL == 0`, else `min(receivedTTL, sendTTL)`).
- Pass `NonRSVPHop` into `updateAdspec` (setting byte 9 `intservBreak`), into `markAdspecNonRSVPHop` at egress and merged egress, into `pathStateBlock.NonRSVPHop`, `mergedPath.NonRSVPHop`, `backupPath`, and `recomputeMergedAdspecLocked`.
- When `nonRSVPHop` is true, treat received ADSPEC Path MTU as `0` (unreliable) in `handlePath`'s floor check and in `handlePathEgress`'s RESV/FIB MTU selection, and drop/decrement transit PATH using `effectivePathTTL`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Raw IPv4 TTL capture | `internal/plugins/rsvpte/transport_linux.go::receivedIPv4Packet` and `transport.go::Packet.TTL` | Reads IPv4 header byte 8 into `Packet.TTL` on every diverted or locally-delivered protocol-46 packet | Extends: consumes the already-captured `Packet.TTL` in `engine.go::handlePacket` and `handlePath` rather than adding another socket option or transport field |
| Outgoing IPv4 TTL / `Send_TTL` coupling | `internal/plugins/rsvpte/transport_linux.go::rawTransport.SendPath` and `wire.go::encodeHeader` | Copies RSVP Common Header byte 4 (`Send_TTL`) into IPv4 header byte 8 on every outgoing PATH | Extends: `handlePathTransit` feeds `effectivePathTTL - 1` into `buildPath`, reusing the existing `SendPath` IP-TTL coupling |
| ADSPEC OPWA composition (`TC_Advertise`) and Global Break bit | `internal/plugins/rsvpte/mtu.go::updateAdspec`, `adspecPathMTU`, `intservBreak` | Validates ADSPEC, updates general parameters (IS hops, BW, latency, MTU), sets `raw[9] \|= intservBreak` on missing link MTU, and returns `0` from `adspecPathMTU` when `intservBreak` is set | Extends: adds `nonRSVPHop bool` to `updateAdspec` to set `raw[9] \|= intservBreak` on non-RSVP hop detection and adds `markAdspecNonRSVPHop` for local egress/merge composition |
| Reservation Path MTU gating | `internal/plugins/rsvpte/reservation.go::reservationPathMTU` | Calls `adspecPathMTU(lsp.PSB.Adspec, ...)` and suppresses RESV/FIB Path MTU whenever the advertised ADSPEC returns `0` | Extends: relies directly on `psb.Adspec` carrying `intservBreak` so transit RESV processing automatically refuses unreliable MTUs without a second gating mechanism |
| FRR backup & merge-point ADSPEC recomputation | `internal/plugins/rsvpte/frr.go::backupPath`, `mergeBackupPath`, `recomputeMergedAdspecLocked` | Rebuilds backup and merged ADSPECs from `ReceivedAdspec` / per-branch `mergedPath.Adspec` | Extends: adds `NonRSVPHop` to `pathStateBlock` and `mergedPath` and passes it into `updateAdspec` / `markAdspecNonRSVPHop` |
| Multi-node in-memory RSVP fabric | `internal/plugins/rsvpte/interop_test.go::fabricPort` | Routes RSVP packets between in-process engines without Linux sockets | Extends: populates `Packet.TTL` from `msg[4]` in `SendPath` so multi-hop tests exercise real TTL comparison |
| freeRouter RSVP-TE interop lab | `internal/le/interoplab/rsvpte/rsvpte.go::scenarioCheckerMap` and `checkers.go::waitMessage` | Discovers declarative 4-role scenarios (`ingress`, `relay`, `transit`, `egress`) under `test/interop-rsvpte/scenarios/` and verifies wire packets via per-peer `tcpdump -vvv` captures | Extends: registers new scenario `transit-non-rsvp-hop-adspec-break` with a non-RSVP freeRouter `relay` (no `mpls rsvp4`) between Ze `ingress` and Ze `transit` toward freeRouter `egress` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads | "The RFC, then ExaBGP API compatibility, then ExaBGP's behavior, in that order (`ai/rules/rfc-compliance.md`)" | agrees | Implements RFC 2205 Section 3.8 / Section 3.11.2 and RFC 2210 Section 3.3.2 Global Break bit signaling for protocol peers without changing any `pkg/` or CLI/YANG contract. |
| `ai/doors/routing-protocols.md` | Boundaries | door covering `internal/plugins/rsvpte/` | "A protocol never writes a kernel route." | agrees | Changes stay strictly inside `internal/plugins/rsvpte/` signaling and ADSPEC composition; no netlink writes or cross-subsystem imports are added. |
| `ai/doors/routing-protocols.md` | RFC and interop obligations | door covering `internal/plugins/rsvpte/` | "RSVP-TE uses freeRouter in `test/interop-rsvpte/scenarios/`. Prove each one red with the fix reverted and the image rebuilt (`ai/rules/interop-and-goal-validation.md`)." | agrees | Adds the named freeRouter interop scenario `test/interop-rsvpte/scenarios/transit-non-rsvp-hop-adspec-break/` (freeRouter non-RSVP IPv4 `relay` + freeRouter RSVP-TE `egress`), registered in `internal/le/interoplab/rsvpte/`, and requires proving it red with the fix reverted and the lab image rebuilt. |
| `ai/doors/le-tooling.md` | Where it sits | door covering `internal/le/interoplab/rsvpte/` and `test/interop-rsvpte/` | "Interop scenarios live under `test/interop*/` and run through `internal/le/interoplab` (`docs/architecture/testing/interop.md`)." | agrees | Places declarative scenario files under `test/interop-rsvpte/scenarios/transit-non-rsvp-hop-adspec-break/` and its typed Go checker and catalog test under `internal/le/interoplab/rsvpte/`, with no shell scripts inside the scenario directory. |
| `docs/architecture/testing/interop.md` | The suites, one per protocol area | `// Design:` of `internal/le/interoplab/rsvpte/rsvpte.go` and `checkers.go` | "Every suite discovers its scenarios the same way. `Discover` (`internal/le/interoplab/discover.go`) reads the scenario directory, keeps the subdirectories, sorts the names lexically, and joins each one against the owning checker registry." | agrees | Registers `scenarioNonRSVPHopBreak` in `scenarioCheckerMap` alongside its declarative directory `test/interop-rsvpte/scenarios/transit-non-rsvp-hop-adspec-break/` and updates `TestScenarioCatalog` so discovery, catalog, and runner agree. |
| `docs/architecture/testing/interop.md` | Design decisions | `// Design:` of `internal/le/interoplab/rsvpte/rsvpte.go` and `checkers.go` | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | Reuses the existing `interop-rsvpte` suite's `KernelCapabilitiesRequired("mpls", "mpls-transit-mtu")` declaration without adding unrelated kernel capability requirements or bypassing preflight checks. |
| `docs/architecture/rsvpte/mpls-rsvp-te.md` | Decision: a path MTU under the labeled-route floor is refused, not clamped | `// Design:` of `engine.go`, `fsm.go`, `mtu.go`, `mtu_test.go`, `interop_test.go`, `rfc2205_transport_integration_linux_test.go`, and modified by this spec | "A received PATH whose composed ADSPEC MTU, bounded by the outgoing link, is below `mplsfib.PathMTUMinimum` draws a PathErr with Error Code 21 (Traffic Control Error), sub-code 05 (Bad Adspec value), and leaves no path state." | agrees | Preserves the floor check for every usable ADSPEC (`nonRSVPHop == false`); when a non-RSVP hop sets the Global Break bit, the ADSPEC MTU is unreliable (`0`, same as an ADSPEC that arrived with the break bit already set), so no MTU is echoed or installed. Also adds the RFC 2205 Section 3.8 non-RSVP-hop detection decision section to this page. |
| `docs/architecture/mpls/mpls-kernel.md` | Path MTU and label overhead | anchored to `internal/plugins/rsvpte/mtu.go` | "`mplsfib.Entry.PathMTU` carries the downstream frame payload budget, including labels. A zero value leaves the output device MTU as the bound." | agrees | Setting the ADSPEC Global Break bit on non-RSVP hop detection makes `adspecPathMTU` return `0`, so RSVP-TE passes `PathMTU = 0` to the MPLS FIB (leaving the output device MTU as the bound rather than installing an unreliable MTU), while preserving `mplsfib.PathMTUMinimum` floor refusal for unbroken ADSPECs. |
| `docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md` | Decision: local repair moves data and control traffic | `// Design:` of `frr.go` | "At the merge point, a backup sender joins the protected PATH when SESSION, LSP_ID, the remaining ERO and the sender traffic specification match." | agrees | Preserves merge-point matching and branch lifecycle while adding per-branch `NonRSVPHop` tracking so merged and backup PATH ADSPECs accurately reflect whether any active incoming branch crossed a non-RSVP hop. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- **Linux production path:** Raw IPv4 protocol-46 datagram received by `rawTransport.readLoop` -> `receive` -> `receivedIPv4Packet` (`internal/plugins/rsvpte/transport_linux.go`), which extracts IPv4 `data[8]` into `Packet.TTL` and payload `data[ihl:total]` into `Packet.Payload`, sending `Packet` over `Transport.Recv()`.
- **Format at entry:** `Packet{Src, Dst, IfIndex, TTL, Payload}` where `Payload` begins with the 8-byte RSVP Common Header (`Payload[4]` = `Send_TTL`) followed by RSVP objects (optional Class 13 C-Type 2 `ADSPEC`).

### Transformation Path
1. `engine.serve` reads `pkt` from `e.transport.Recv()` and calls `engine.handlePacket(pkt)`.
2. `handlePacket` decodes `pkt.Payload` with `DecodeMessage`, validating RSVP header (`msg.Header.TTL` = `Send_TTL`) and ADSPEC structure (`validateAdspec`, `adspecPathMTU`).
3. For `MsgTypePath`, `handlePacket` passes `peer`, `pkt.TTL`, and `msg` to `handlePath(src, receivedTTL, msg)`.
4. `handlePath` computes:
   - `nonRSVPHop := nonRSVPHopDetected(receivedTTL, msg.Header.TTL)` (`receivedTTL != 0 && receivedTTL != msg.Header.TTL`).
   - `effectiveTTL := effectivePathTTL(receivedTTL, msg.Header.TTL)` (`msg.Header.TTL` if `receivedTTL == 0`, else `min(receivedTTL, msg.Header.TTL)`).
   - `receivedMTU := msg.PathMTU` (zeroed to `0` when `nonRSVPHop` is true, because RFC 2210 Section 2.3.1/3.3.2 marks all ADSPEC parameters unreliable when the Global Break bit is set).
5. `handlePath` runs policy, ERO resolution, and the `onwardPathMTU(receivedMTU, selected.Route.MTU)` floor check, then dispatches:
   - **Merge point (`mergeBackupPath`)**: checks `!isLocalAddress && effectiveTTL <= 1`; stores `NonRSVPHop: nonRSVPHop` in `mergedPath`; calls `recomputeMergedAdspecLocked`, which ORs `NonRSVPHop` across live branches and invokes `updateAdspec(psb.Adspec, psb.Route.MTU, psb.NonRSVPHop)` (transit) or `markAdspecNonRSVPHop(psb.Adspec)` (egress).
   - **Egress (`handlePathEgress`)**: stores `psb.NonRSVPHop = nonRSVPHop`, clones `msg.AdspecRaw` into `psb.Adspec`, calls `markAdspecNonRSVPHop(psb.Adspec)` when `nonRSVPHop && len(psb.Adspec) > 0`, derives `pathMTU := adspecPathMTU(psb.Adspec, msg.SenderTSpec.Service)` (which is `0` whenever Global Break is set), and writes that `pathMTU` into `flow.MaxPacketSize`, `rsb.PathMTU`, and `fib.programPop`.
   - **Transit (`handlePathTransit`)**: drops the PATH when `effectiveTTL <= 1`; stores `lsp.PSB.NonRSVPHop = nonRSVPHop`; clones `msg.AdspecRaw` into `lsp.PSB.Adspec` and calls `updateAdspec(lsp.PSB.Adspec, selected.Route.MTU, nonRSVPHop)` (setting byte 9 `intservBreak` when `nonRSVPHop` is true); encodes the relayed PATH with `buildPath(lsp.PSB, routerID, effectiveTTL-1)` and transmits via `Transport.SendPath` (which writes IPv4 header TTL = `Send_TTL = effectiveTTL - 1`).
6. On subsequent RESV return at transit, `reservationPathMTU` calls `adspecPathMTU(lsp.PSB.Adspec, ...)` which returns `0` when `intservBreak` is set, preventing unreliable MTU installation.
7. On FRR local repair at a PLR, `backupPath` rebuilds `backup.Adspec` from `psb.ReceivedAdspec` via `updateAdspec(backup.Adspec, bypassMTU, psb.NonRSVPHop)`, preserving the Global Break bit across bypass signaling.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Linux kernel IPv4 stack ↔ `rawTransport` | `AF_INET SOCK_RAW` protocol-46 socket with `IP_ROUTER_ALERT` and `IP_HDRINCL`; IPv4 header byte 8 (`TTL`) read into `Packet.TTL` and written from `Send_TTL` (`msg[4]`) | No |
| `rawTransport` ↔ `engine` | In-process `Transport.Recv()` channel carrying `Packet` value (no shared mutable pointers) | No |
| `engine` ↔ traffic-control OPWA (`mtu.go`) | Direct package-internal calls `updateAdspec(raw, outgoingMTU, nonRSVPHop)` and `markAdspecNonRSVPHop(raw)` operating on owned byte slices | No |
| `engine` ↔ FIB writer (`fib-kernel` / `fib-vpp`) | Existing `(mpls-fib, entry)` EventBus message via `busFIB` carrying `PathMTU` (`0` when ADSPEC is broken) | No |

### Changed-Wire Census

**Search evidence:** Ran `git grep -n "ClassAdspec\|intservBreak\|updateAdspec\|adspecPathMTU\|Send_TTL" internal/ test/ pkg/` and `grep -n "\.TTL\|TTL:" internal/plugins/rsvpte/*.go`. Positive controls found: `internal/plugins/rsvpte/mtu.go::updateAdspec` (writer of ADSPEC break bit), `internal/plugins/rsvpte/mtu.go::adspecPathMTU` (reader of ADSPEC break bit), `internal/plugins/rsvpte/transport_linux.go::rawTransport.SendPath` (writer of IPv4 TTL from `Send_TTL`), `internal/plugins/rsvpte/transport_linux.go::receivedIPv4Packet` (reader of IPv4 TTL into `Packet.TTL`), `internal/plugins/rsvpte/wire.go::encodeHeader` / `DecodeHeader` (writer/reader of RSVP `Send_TTL`). Zero hits exist outside `internal/plugins/rsvpte/` in Go, shell, Python, or embedded scripts; outside-repository consumers are standard RFC 2205 / RFC 2210 / RFC 3209 RSVP-TE peers and intermediate IPv4 routers.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| RSVP PATH ADSPEC Global Break bit (Class 13, C-Type 2, byte 9 bit 7 `0x80`) | writer | `internal/plugins/rsvpte/mtu.go::encodeAdspec` | Positive control from grep: writes initial ADSPEC with byte 9 flags = 0 | Preserve: ingress originates with break bit clear |
| RSVP PATH ADSPEC Global Break bit (Class 13, C-Type 2, byte 9 bit 7 `0x80`) | writer | `internal/plugins/rsvpte/mtu.go::updateAdspec` (called by `engine.go::handlePathTransit`, `frr.go::backupPath`, `frr.go::recomputeMergedAdspecLocked`) | Positive control from grep: currently sets `raw[9] \|= intservBreak` only when `outgoingMTU == 0` | Change: add `nonRSVPHop bool` parameter and set `raw[9] \|= intservBreak` when `nonRSVPHop` is true, per RFC 2205 §3.11.2 and RFC 2210 §3.3.2 |
| RSVP PATH ADSPEC Global Break bit (Class 13, C-Type 2, byte 9 bit 7 `0x80`) | writer | `internal/plugins/rsvpte/mtu.go::markAdspecNonRSVPHop` (called by `engine.go::handlePathEgress`, `frr.go::recomputeMergedAdspecLocked`) | New helper beside `updateAdspec`; sets `raw[9] \|= intservBreak` on local egress/merged ADSPEC | Change: new local traffic-control writer ensuring egress and merged egress mark ADSPEC broken on non-RSVP hop detection |
| RSVP PATH ADSPEC Global Break bit (Class 13, C-Type 2, byte 9 bit 7 `0x80`) | writer | `internal/plugins/rsvpte/build.go::buildPath`, `buildPathErr` | Positive control from grep: encodes `psb.Adspec` (`buildPath`) and unmodified `ReceivedAdspec` (`buildPathErr`) via `encodeOpaqueObject` | Preserve: opaque byte relay carries the updated Global Break bit automatically on PATH while keeping `ReceivedAdspec` intact for PathErr |
| RSVP PATH ADSPEC Global Break bit (Class 13, C-Type 2, byte 9 bit 7 `0x80`) | reader | `internal/plugins/rsvpte/wire.go::DecodeMessage` + `mtu.go::adspecPathMTU` + `reservation.go::reservationPathMTU` | Positive control from grep: `adspecPathMTU` checks `raw[off+1]&intservBreak != 0` and returns `0` | Preserve: already honors the Global Break bit on receipt and during RESV MTU gating |
| RSVP PATH ADSPEC Global Break bit (Class 13, C-Type 2, byte 9 bit 7 `0x80`) | reader | `internal/plugins/rsvpte/engine.go::handlePathEgress` | Source read: currently reads pre-TTL-check `msg.PathMTU` instead of post-detection `psb.Adspec` | Change: read egress `pathMTU` from `adspecPathMTU(psb.Adspec, msg.SenderTSpec.Service)` after applying `markAdspecNonRSVPHop` |
| RSVP PATH ADSPEC Global Break bit (Class 13, C-Type 2, byte 9 bit 7 `0x80`) | reader | Outside-repository RFC 2205 / RFC 2210 / RFC 3209 RSVP-TE peers (freeRouter, FRR, vendor LSRs) | Protocol contract (`ai/doors/routing-protocols.md`, RFC 2210 Section 3.3.2) | Preserve: standard RFC 2210 wire bit; direct RSVP hops leave bit 0 unchanged |
| RSVP PATH Common Header `Send_TTL` (byte 4) and IPv4 Header TTL (byte 8) | writer | `internal/plugins/rsvpte/engine.go::sendPathLocked`, `build.go::encodeMessage`, `wire.go::encodeHeader`, `transport_linux.go::rawTransport.SendPath` | Positive control from grep: ingress writes `defaultIPTTL = 64` to `buf[4]`, `SendPath` copies `msg[4]` to IPv4 `header[8]` | Preserve: direct-hop origination and IP-header copy already match RFC 2205 Section 3.1.1 |
| RSVP PATH Common Header `Send_TTL` (byte 4) and IPv4 Header TTL (byte 8) | writer | `internal/plugins/rsvpte/engine.go::handlePathTransit` | Source read: currently relays with `msg.Header.TTL - 1` even if non-RSVP hops reduced `Packet.TTL` below `msg.Header.TTL` | Change: compute `effectiveTTL = effectivePathTTL(receivedTTL, msg.Header.TTL)`, drop if `effectiveTTL <= 1`, and relay with `effectiveTTL - 1` |
| RSVP PATH Common Header `Send_TTL` (byte 4) and IPv4 Header TTL (byte 8) | reader | `internal/plugins/rsvpte/transport_linux.go::receivedIPv4Packet`, `wire.go::DecodeHeader`, `engine.go::handlePacket` -> `handlePath` | Positive control from grep: `receivedIPv4Packet` reads IPv4 `data[8]` into `Packet.TTL`; `DecodeHeader` reads `buf[4]` into `Header.TTL`; `handlePacket` currently drops `pkt.TTL` | Change: pass `pkt.TTL` into `handlePath` and compare against `msg.Header.TTL` via `nonRSVPHopDetected` and `effectivePathTTL` |
| RSVP PATH Common Header `Send_TTL` (byte 4) and IPv4 Header TTL (byte 8) | reader | Outside-repository IPv4 routers and downstream RFC 2205 RSVP peers | RFC 791 / RFC 2205 Section 3.8 forwarding and TTL comparison | Preserve: outgoing IPv4 TTL equals outgoing `Send_TTL`, so the next RSVP hop sees equality on a direct link and decrement across a non-RSVP cloud |

### Integration Points
- `engine.go::handlePacket` -> `handlePath` -> `handlePathEgress` / `handlePathTransit` / `frr.go::mergeBackupPath` - threads `pkt.TTL` into path handling.
- `mtu.go::updateAdspec` and `markAdspecNonRSVPHop` - single traffic-control OPWA surface (`TC_Advertise`) that writes the Global Break bit.
- `fsm.go::pathStateBlock.NonRSVPHop` and `mergedPath.NonRSVPHop` - stores detected non-RSVP hop state for FRR backup PATH construction and merge-point branch recomputation.
- `interop_test.go::fabricPort.SendPath` - populates `Packet.TTL` from `msg[4]` so multi-node fabric tests propagate realistic TTLs.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | `rawTransport.receivedIPv4Packet` -> `Packet.TTL` -> `engine.handlePacket` -> `handlePath` -> `mtu.updateAdspec`/`markAdspecNonRSVPHop` -> `buildPath` / `reservationPathMTU` -> `busFIB`; no layer is skipped. |
| No unintended coupling (components stay isolated) | Yes | All code edits live strictly inside `internal/plugins/rsvpte/`; no imports from `internal/component/` or other plugins are added. |
| No duplicated functionality (extends existing, does not recreate) | Yes | Extends existing `Packet.TTL`, `mtu.go::updateAdspec`, `adspecPathMTU`, `reservationPathMTU`, `backupPath`, and `recomputeMergedAdspecLocked` instead of adding a parallel ADSPEC or TTL parser. |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `Packet.TTL` is a 1-byte value; `updateAdspec` / `markAdspecNonRSVPHop` mutate the already-cloned `psb.Adspec` slice in place with zero extra allocations. |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No new command, view, family, handler, or core/shared package change; existing `rsvp-te` plugin registration is unchanged. |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Introduces no new user-facing command, plugin, config leaf, or family name; `git grep` across `internal/` confirms no central switch, seed map, or completion table is touched. |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Linux `IP_ROUTER_ALERT` raw socket delivers direct-hop PATH packets with `Packet.TTL == Send_TTL` (before `ip_decrease_ttl`), while an intermediate non-RSVP Linux IPv4 router decrements IPv4 TTL by 1 (`Packet.TTL == Send_TTL - 1`). | `internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go::TestIntegrationRSVPPathCarriage` (line 454 asserts `packet.TTL == msg.payload[4]`); Linux `net/ipv4/ip_forward.c` runs `ip_call_ra_chain` before `ip_decrease_ttl`. | Direct hops would falsely trigger non-RSVP detection or intermediate routers would fail to trigger it. | `TestIntegrationRSVPNonRSVPHopDetection` in a 3-namespace Linux lab (sender -> non-RSVP forwarding router -> Ze transit/egress). | unvalidated |
| A-2 | Existing freeRouter interop scenarios (`test/interop-rsvpte/scenarios/`) send direct-hop PATH messages with IPv4 TTL equal to RSVP `Send_TTL`. | RFC 2205 Section 3.1.1 defines `Send_TTL` as the outgoing IP TTL; freeRouter implements RFC 2205/3209 on directly connected veth/UDP interfaces. | A freeRouter direct hop sending mismatched TTL/`Send_TTL` would unexpectedly set the Global Break bit on relayed PATHs. | Existing `./le test integration interop-rsvpte` suite passes without regression. | unvalidated |
| A-3 | Treating `Packet.TTL == 0` as "TTL unset by synthetic unit transport" cannot mask a real non-RSVP hop on Linux. | Under IPv4 forwarding (RFC 791 / Linux `ip_forward`), any router that receives a packet with `TTL <= 1` drops it with ICMP Time Exceeded rather than forwarding it with `TTL == 0`; `rawTransport.SendPath` sets IPv4 TTL = `Send_TTL >= 1`. | A packet with genuine IPv4 TTL 0 would skip non-RSVP detection. | Verified against RFC 791/Linux forwarding semantics and unit boundary tests for `Packet.TTL == 0`, `1`, `Send_TTL - 1`, `Send_TTL`. | unvalidated |
| A-4 | When `nonRSVPHop` is true, suppressing the received ADSPEC MTU in `handlePath`'s floor check (treating `receivedMTU` as `0`) matches RFC 2210 Section 2.3.1 ("all other parameters in the ADSPEC are unreliable"). | `rfc/full/rfc2210.txt` lines 399-402 and 1015-1018; `internal/plugins/rsvpte/mtu.go::adspecPathMTU` already returns `0` when `intservBreak` is set. | A PATH that crossed a non-RSVP hop with a stale/small MTU would either be falsely refused with `BadAdspec` or install a below-floor metric. | Unit test verifying a non-RSVP PATH with any ADSPEC MTU is neither falsely refused nor installed with a non-zero FIB PathMTU. | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Existing unit tests in `internal/plugins/rsvpte/` construct `Packet{Src: ..., Payload: ...}` with `TTL: 0` and assert non-zero `PathMTU`. | `TestTransitForwardsComposedAdspecMTU`, `pathmtu_floor_test.go`, or `interop_test.go` fail after enabling TTL comparison. | `nonRSVPHopDetected` returns `false` when `receivedTTL == 0`, and `fabricPort.SendPath` populates `Packet.TTL = msg[4]`. |
| R-2 | Non-RSVP hop break bit sticks permanently on an LSP after routing converges to an all-RSVP direct hop, or gets lost during FRR local repair / merge-point refresh. | Refresh recovery test or FRR backup/merge test observes wrong Global Break bit after topology change. | Rebuild `psb.Adspec` from `msg.AdspecRaw` on every PATH refresh, store `NonRSVPHop` per `mergedPath` branch, and recompute `psb.NonRSVPHop` from live branches only in `recomputeMergedAdspecLocked`. |
| R-3 | Non-RSVP router decrements IPv4 TTL to `1` while `Send_TTL` remains higher; transit checks only `msg.Header.TTL <= 1` and re-inflates TTL on relay. | Boundary test with `Send_TTL = 4`, `Packet.TTL = 1` relays instead of dropping. | Compute `effectivePathTTL(receivedTTL, sendTTL)` (`min(receivedTTL, sendTTL)` when `receivedTTL > 0`) and apply it to both the `<= 1` drop guard and the `effectiveTTL - 1` relayed `Send_TTL`. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Direct-hop RSVP-TE LSPs could falsely mark ADSPEC broken and lose Path MTU discovery (`PathMTU` drops to `0`), or transit could drop valid PATHs if `effectivePathTTL` mishandled `Packet.TTL == 0`. |
| How is it reverted? | Single commit revert inside `internal/plugins/rsvpte/` and `docs/architecture/rsvpte/mpls-rsvp-te.md`; no config migration, persisted state format change, or `pkg/` contract change. |
| Who else touches this path? | `plan/release/fixes/spec-rsvp-routing-and-interface-integration.md` also lists `internal/plugins/rsvpte/engine.go` (for incoming-interface and nonlocal-forwarding checks); its edits are orthogonal to `Packet.TTL` / `updateAdspec` and merge cleanly. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `engine.handlePacket` receiving a PATH `Packet` with `Packet.TTL < Header.TTL` | → | `handlePath` -> `nonRSVPHopDetected` -> `handlePathTransit` -> `updateAdspec(..., true)` -> `buildPath` -> `Transport.SendPath` | `TestRFC2205NonRSVPHopTransitSetsAdspecBreakBit` |
| `engine.handlePacket` receiving a PATH `Packet` with `Packet.TTL < Header.TTL` at egress | → | `handlePath` -> `nonRSVPHopDetected` -> `handlePathEgress` -> `markAdspecNonRSVPHop` -> `adspecPathMTU == 0` -> `buildResv` / `fib.programPop` | `TestRFC2205NonRSVPHopEgressSuppressesUnreliableMTU` |
| Real Linux `rawTransport` receiving an IPv4 PATH forwarded across a non-RSVP Linux IPv4 router namespace | → | `rawTransport.receivedIPv4Packet` (`Packet.TTL`) -> `engine.handlePacket` -> `updateAdspec(..., true)` -> `rawTransport.SendPath` wire capture | `TestIntegrationRSVPNonRSVPHopDetection` |
| freeRouter interop lab (`transit-non-rsvp-hop-adspec-break`) forwarding Ze `ingress` PATH through non-RSVP freeRouter `relay` to Ze `transit` and freeRouter `egress` | → | `rawTransport.receivedIPv4Packet` (`Packet.TTL = 63 < Send_TTL = 64`) -> `handlePathTransit` -> `updateAdspec(..., true)` -> `buildPath(..., 62)` -> freeRouter `egress` `tcpdump` capture + RESV + swap | `checkNonRSVPHopBreak` (`RSVPTE_INTEROP_SCENARIO=transit-non-rsvp-hop-adspec-break ./le test integration interop-rsvpte`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Transit engine receives a valid PATH carrying an ADSPEC (`PathMTU = 1500`, break bit clear) with `Send_TTL = 64` and `Packet.TTL = 63` (or `62`, multiple non-RSVP hops). | Transit records `PSB.NonRSVPHop = true`, sets the ADSPEC Global Break bit (`byte 9 & 0x80 != 0`) on the relayed PATH, sets relayed `Send_TTL = Packet.TTL - 1` (`62` or `61`), preserves unmodified `ReceivedAdspec` for upstream `PathErr`, and when the downstream RESV returns, installs `PathMTU = 0` on the transit swap rather than an unreliable ADSPEC MTU. | None; a relayed PATH after a detected non-RSVP hop with break bit clear or re-inflated `Send_TTL` violates RFC 2205 §3.8 and RFC 2210 §3.3.2. |
| AC-2 | Transit engine receives a valid PATH carrying an ADSPEC (`PathMTU = 1500`, break bit clear) on a direct RSVP hop (`Packet.TTL == Send_TTL == 64`, or synthetic unit `Packet.TTL == 0`). | Transit records `PSB.NonRSVPHop = false`, leaves the ADSPEC Global Break bit clear (`byte 9 & 0x80 == 0`), composes `min(1500, outgoingMTU)` into the relayed ADSPEC, relays with `Send_TTL = 63`, and installs the negotiated `PathMTU` when the downstream RESV returns. | Synthetic `Packet.TTL == 0` in unit tests is permitted to behave as direct-hop because real IPv4 forwarding drops `TTL <= 1` before forwarding and never delivers `TTL == 0`. |
| AC-3 | Egress engine receives a valid PATH carrying an ADSPEC (`PathMTU = 1500`, break bit clear) with `Send_TTL = 64` and `Packet.TTL = 63`. | Egress records `PSB.NonRSVPHop = true`, marks its local `PSB.Adspec` with the Global Break bit, establishes the LSP and sends a RESV whose `FLOWSPEC.MaxPacketSize` is `0` (not `1500`), and programs the egress pop with `PathMTU = 0`; on a direct hop (`Packet.TTL == 64`) egress preserves `FLOWSPEC.MaxPacketSize = 1500` and `PathMTU = 1500`. | LSP establishment and bandwidth admission still succeed because RFC 2205 §2.9/§3.8 specifies signaling continues through non-RSVP clouds while reporting the QoS break via traffic control. |
| AC-4 | Transit engine receives a PATH with `Send_TTL = 4` and `Packet.TTL = 1` (hop budget exhausted by non-RSVP routers). | Transit drops the PATH without installing state or relaying it downstream; with `Send_TTL = 4` and `Packet.TTL = 2`, transit relays with `Send_TTL = 1` and the Global Break bit set. | None; relaying a `Packet.TTL == 1` PATH would re-inflate IP TTL and violate hop-limit loop bounding. |
| AC-5 | A transit PLR whose incoming PATH crossed a non-RSVP hop (`PSB.NonRSVPHop = true`) executes RFC 4090 local repair (`backupPath`), OR a merge point receives one branch with `Packet.TTL < Send_TTL` and one direct branch (`Packet.TTL == Send_TTL`). | The PLR's backup PATH carries the ADSPEC Global Break bit (`byte 9 & 0x80 != 0`). At the merge point, while the non-RSVP branch is live, the merged PATH carries the Global Break bit (`PSB.NonRSVPHop = true`); once the non-RSVP branch tears down or expires leaving only the direct branch, `recomputeMergedAdspecLocked` clears `PSB.NonRSVPHop` and the Global Break bit. On ordinary transit, a later direct-hop PATH refresh (`Packet.TTL == Send_TTL`) also clears `PSB.NonRSVPHop` and the locally set break bit. | A PATH whose upstream ADSPEC already arrived with the Global Break bit set retains that upstream bit on refresh, because RFC 2210 §3.3.2 forbids clearing an upstream node's break bit. |
| AC-6 | In a 3-namespace Linux lab (`sender` -> non-RSVP `router` with `ip_forward=1` and no RSVP socket -> `ze-transit` running `rawTransport` + `engine` -> `egress` capture), `sender` transmits a PATH with ADSPEC (`PathMTU = 1500`) and `Send_TTL = 37`. | The non-RSVP Linux router decrements IPv4 TTL to `36`; `ze-transit`'s `rawTransport` delivers `Packet.TTL = 36` while `Header.TTL = 37`; `ze-transit` transmits a relayed PATH on the downstream link whose captured IPv4 TTL and RSVP `Send_TTL` are both `35` and whose ADSPEC byte 9 has `0x80` set. On a direct 2-namespace hop (`Packet.TTL == Send_TTL == 37`), `ze-transit` transmits `Send_TTL = 36` with ADSPEC byte 9 `0x80` clear. | Linux namespace test skips only when unprivileged user namespaces are unavailable (`!userns.Enter(t)`), matching existing `rfc2205_transport_integration_linux_test.go` convention. |
| AC-7 | In Docker freeRouter interop scenario `transit-non-rsvp-hop-adspec-break` (Ze `ingress` -> freeRouter non-RSVP IPv4 `relay` with no `mpls rsvp4` -> Ze `transit` -> freeRouter RSVP-TE `egress`), Ze `ingress` signals an LSP with no ERO toward `egress` loopback `198.51.100.4` via `relay`. | `relay` forwards the PATH natively with IPv4 `ttl 63` and RSVP `ttl: 64` (and ADSPEC Global Break bit clear); `egress` captures Ze `transit`'s relayed PATH (`hopFrom(addressZeTransit)`) carrying IPv4 `ttl 62`, RSVP `ttl: 62`, and the ADSPEC Global Break bit set; `egress` returns a `Resv Message` with a `Label Object` and Ze `transit` installs the MPLS swap (`waitSwap`). Reverting the Ze fix and rebuilding the lab image makes `transit-non-rsvp-hop-adspec-break` fail red (`ttl 63` and Global Break bit clear). | None; freeRouter interop proof is mandatory for wire-visible RSVP-TE changes (`ai/doors/routing-protocols.md`). |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Signals an RSVP-TE LSP whose PATH traverses a non-RSVP IPv4 router before reaching a Ze transit LSR | Non-RSVP IPv4 router decrements IP TTL -> `rawTransport.receivedIPv4Packet` (`Packet.TTL`) -> `engine.handlePacket` -> `handlePath` (`nonRSVPHopDetected`) -> `handlePathTransit` -> `mtu.updateAdspec(..., true)` -> `buildPath(..., effectiveTTL-1)` -> `rawTransport.SendPath` -> downstream peer | `transit-non-rsvp-hop-adspec-break` (`checkNonRSVPHopBreak`), `TestIntegrationRSVPNonRSVPHopDetection`, and `TestRFC2205NonRSVPHopTransitSetsAdspecBreakBit` |
| 2 | Terminates an RSVP-TE LSP on a Ze egress LSR after a non-RSVP IPv4 hop | `Packet.TTL < Send_TTL` -> `handlePath` -> `handlePathEgress` -> `markAdspecNonRSVPHop` -> `adspecPathMTU == 0` -> `buildResv` (`MaxPacketSize = 0`) + `fib.programPop(..., 0)` | `TestRFC2205NonRSVPHopEgressSuppressesUnreliableMTU` |
| 3 | Runs a direct RSVP-TE LSP (no non-RSVP hops) with ADSPEC Path MTU discovery | `Packet.TTL == Send_TTL` -> `handlePath` (`nonRSVPHop = false`) -> `updateAdspec(..., false)` -> composed MTU preserved in relayed PATH, RESV `FLOWSPEC`, and FIB entry | `TestRFC2205DirectHopLeavesAdspecBreakBitClear`, `TestIntegrationRSVPNonRSVPHopDetection`, and existing `transit-strict-hop-forwarded` / `transit-loose-ero-expansion` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC2205NonRSVPHopDetectionBoundaries` | `internal/plugins/rsvpte/rfc2205_non_rsvp_hop_test.go` | `nonRSVPHopDetected` and `effectivePathTTL` across all boundary inputs (`Packet.TTL` = `0`, `1`, `Send_TTL-1`, `Send_TTL`, `Send_TTL+1`, `255`; `Send_TTL` = `0`, `1`, `2`, `64`, `255`) | |
| `TestRFC2205AdspecNonRSVPHopFlag` | `internal/plugins/rsvpte/rfc2205_non_rsvp_hop_test.go` | `updateAdspec(..., true)` and `markAdspecNonRSVPHop` set byte 9 `0x80` (`intservBreak`), make `adspecPathMTU == 0`, while `updateAdspec(..., false)` preserves clean MTU composition (`RFC2205-3-30` positive/negative) | |
| `TestRFC2205NonRSVPHopTransitSetsAdspecBreakBit` | `internal/plugins/rsvpte/rfc2205_non_rsvp_hop_test.go` | Wiring + `RFC2205-3-30 positive`: transit receiving `Packet.TTL < Send_TTL` sets Global Break bit on relayed PATH, decrements from `effectivePathTTL`, preserves `ReceivedAdspec` on `PathErr`, and installs `PathMTU = 0` on returning RESV; also verifies PATH without ADSPEC records `PSB.NonRSVPHop = true` without fabricating ADSPEC | |
| `TestRFC2205DirectHopLeavesAdspecBreakBitClear` | `internal/plugins/rsvpte/rfc2205_non_rsvp_hop_test.go` | `RFC2205-3-30 negative`: direct hop (`Packet.TTL == Send_TTL`) leaves Global Break bit clear, preserves composed ADSPEC Path MTU through transit and egress, and programs negotiated `PathMTU` | |
| `TestRFC2205NonRSVPHopEgressSuppressesUnreliableMTU` | `internal/plugins/rsvpte/rfc2205_non_rsvp_hop_test.go` | Egress receiving `Packet.TTL < Send_TTL` marks `PSB.NonRSVPHop = true` and `PSB.Adspec` Global Break bit, sends RESV with `MaxPacketSize = 0`, programs pop with `PathMTU = 0`, and does not falsely trigger `BadAdspec` floor refusal | |
| `TestRFC2205NonRSVPHopTTLExhaustedDropped` | `internal/plugins/rsvpte/rfc2205_non_rsvp_hop_test.go` | Transit drops PATH when `Packet.TTL == 1` even if `Send_TTL > 1`, and relays with `Send_TTL = 1` + break bit when `Packet.TTL == 2` | |
| `TestRFC2205NonRSVPHopRefreshAndFRRLifecycle` | `internal/plugins/rsvpte/rfc2205_non_rsvp_hop_test.go` | Direct-hop refresh clears locally-set break bit (while preserving upstream break bit), PLR `backupPath` retains `NonRSVPHop` break bit, and merge point ORs `NonRSVPHop` across live branches and clears it once the non-RSVP branch tears/expires | |
| `TestAdspecWireFormat` / `TestAdspecUnknownAndOverride` / `TestAdspecOpaqueServicePreserved` | `internal/plugins/rsvpte/mtu_test.go` | Existing ADSPEC unit tests updated for new `updateAdspec` signature (`nonRSVPHop = false`) plus positive case with `nonRSVPHop = true` | |
| `TestScenarioCatalog` / `TestParseCaptureNonRSVPHopAdspecBreak` | `internal/le/interoplab/rsvpte/rsvpte_test.go` | Scenario catalog includes `transit-non-rsvp-hop-adspec-break`, and checker capture parsers match ADSPEC Global Break bit + IPv4/RSVP TTL (`63`/`64` incoming, `62`/`62` relayed) while rejecting clean ADSPEC or `ttl 63` | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `Packet.TTL` (`uint8`) | `0..255` (`0` = unset synthetic transport; `1..255` = IPv4 wire TTL) | `255` (and `1` for received hop-exhausted boundary) | N/A (`uint8` unsigned) | N/A (`uint8` max `255`) |
| `Header.TTL` / `Send_TTL` (`uint8`) | `0..255` (`<= 1` dropped at transit; `2..255` relayed) | `255` (min relayed `2`) | `0`, `1` (transit drops PATH) | N/A (`uint8` max `255`) |
| `effectivePathTTL` (`uint8`) | `0..255` (`min(Packet.TTL, Send_TTL)` when `Packet.TTL > 0`) | `255` (min relayed `2` -> outgoing `1`) | `0`, `1` (transit drops PATH) | N/A (`uint8` max `255`) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rsvpte-lsp-setup` (existing suite regression) | `test/rsvpte/rsvpte-lsp-setup.ci` | Single-daemon RSVP-TE config delivery and show commands continue passing unchanged (no new CLI/config/RPC surface is added; raw-IP non-RSVP forwarding is tested in the freeRouter interop scenario and Linux namespace integration below) | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `transit-non-rsvp-hop-adspec-break` | `test/interop-rsvpte/scenarios/transit-non-rsvp-hop-adspec-break/` | freeRouter (non-RSVP IPv4 `relay` without `mpls rsvp4` + RSVP-TE `egress` with `mpls rsvp4`) | Real non-RSVP IPv4 router (`relay`) decrements IPv4 TTL `64 -> 63` while leaving `Send_TTL = 64` and ADSPEC untouched; Ze `transit` detects the non-RSVP hop, sets the RFC 2210 ADSPEC Global Break bit, relays `Send_TTL = 62` / IPv4 `ttl 62` to freeRouter `egress`, and establishes the LSP (`waitSwap`); proved red when the Ze fix is reverted and the lab image is rebuilt | |
| `TestIntegrationRSVPNonRSVPHopDetection` (Linux kernel IPv4 router namespace) | `internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go` | Real Linux IPv4 forwarding stack (`ip_forward=1`, no RSVP socket) between sender and Ze transit | Real Linux IPv4 TTL decrement across a non-RSVP IP router triggers Ze's `Packet.TTL != Send_TTL` detection and sets the RFC 2210 ADSPEC Global Break bit + decremented `Send_TTL` on the wire, while a direct hop leaves the break bit clear; reverting `nonRSVPHopDetected` makes the test fail red | |
| `transit-strict-hop-forwarded`, `transit-loose-ero-expansion` (existing freeRouter suite) | `test/interop-rsvpte/scenarios/` | freeRouter | Regression proof that direct-hop freeRouter PATH/RESV signaling is unaffected (no spurious Global Break bit on direct hops) | |

## Files to Modify
- `internal/plugins/rsvpte/mtu.go` - extend `updateAdspec(raw []byte, outgoingMTU uint32, nonRSVPHop bool) error` to set `intservBreak` when `nonRSVPHop` is true, and add `markAdspecNonRSVPHop(raw []byte) error`.
- `internal/plugins/rsvpte/fsm.go` - add `NonRSVPHop bool` to `pathStateBlock` and `mergedPath`.
- `internal/plugins/rsvpte/engine.go` - add `nonRSVPHopDetected` and `effectivePathTTL`; thread `pkt.TTL` from `handlePacket` through `handlePath`, `handlePathEgress`, `handlePathTransit`, and `mergeBackupPath`; suppress unreliable ADSPEC Path MTU on non-RSVP hop detection; relay with `effectiveTTL - 1` and drop when `effectiveTTL <= 1`.
- `internal/plugins/rsvpte/frr.go` - pass `psb.NonRSVPHop` in `backupPath`; carry `NonRSVPHop` and `effectivePathTTL` in `mergeBackupPath`; recompute `psb.NonRSVPHop` and Global Break bit in `recomputeMergedAdspecLocked`.
- `internal/plugins/rsvpte/mtu_test.go` - update `updateAdspec` call sites for the new `nonRSVPHop` argument and add non-RSVP-hop coverage.
- `internal/plugins/rsvpte/interop_test.go` - populate `Packet.TTL` from `msg[4]` in `fabricPort.SendPath`.
- `internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go` - add `TestIntegrationRSVPNonRSVPHopDetection` exercising a 3-namespace Linux lab with an intermediate non-RSVP IPv4 forwarding router.
- `internal/le/interoplab/rsvpte/rsvpte.go` - register `scenarioNonRSVPHopBreak` in `scenarioCheckerMap`.
- `internal/le/interoplab/rsvpte/checkers.go` - add `scenarioNonRSVPHopBreak` and `checkNonRSVPHopBreak` verifying incoming `ttl 63` / `Send_TTL 64` from non-RSVP `relay`, outgoing `ttl 62` / `Send_TTL 62` + ADSPEC Global Break bit from Ze `transit` at freeRouter `egress`, egress RESV + LABEL, and `waitSwap`.
- `internal/le/interoplab/rsvpte/rsvpte_test.go` - add `scenarioNonRSVPHopBreak` to `TestScenarioCatalog` and add `TestParseCaptureNonRSVPHopAdspecBreak`.
- `docs/architecture/rsvpte/mpls-rsvp-te.md` - add decision section documenting RFC 2205 Section 3.8 TTL vs `Send_TTL` non-RSVP hop detection and ADSPEC Global Break bit signaling, with source anchors.
- `rfc/short/rfc2205.md` - update the `RFC2205-3-30` compliance checklist row (and repoint `RFC2205-3-31` to `plan/post-release/features/spec-rsvpte-non-rsvp-clouds.md`) during implementation closure.

## Files to Create
- `internal/plugins/rsvpte/rfc2205_non_rsvp_hop_test.go` - unit, boundary, wiring, FRR, and `RFC2205-3-30` positive/negative tagged tests.
- `test/interop-rsvpte/scenarios/transit-non-rsvp-hop-adspec-break/ingress.conf` - Ze ingress tunnel to `198.51.100.4` with no ERO (routes natively via `relay`).
- `test/interop-rsvpte/scenarios/transit-non-rsvp-hop-adspec-break/ingress-net.txt` - static route `198.51.100.4/32 via 172.29.81.15`.
- `test/interop-rsvpte/scenarios/transit-non-rsvp-hop-adspec-break/relay-hw.txt` - freeRouter hardware definition for non-RSVP `relay`.
- `test/interop-rsvpte/scenarios/transit-non-rsvp-hop-adspec-break/relay-sw.txt` - freeRouter plain IPv4 forwarding config (no `mpls rsvp4`) routing `198.51.100.4/32` via `172.29.81.3`.
- `test/interop-rsvpte/scenarios/transit-non-rsvp-hop-adspec-break/transit.conf` - Ze transit RSVP-TE config at `172.29.81.3`.
- `test/interop-rsvpte/scenarios/transit-non-rsvp-hop-adspec-break/transit-net.txt` - static route `198.51.100.4/32 via 172.29.81.14`.
- `test/interop-rsvpte/scenarios/transit-non-rsvp-hop-adspec-break/egress-hw.txt` - freeRouter hardware definition for RSVP-TE `egress`.
- `test/interop-rsvpte/scenarios/transit-non-rsvp-hop-adspec-break/egress-sw.txt` - freeRouter RSVP-TE `egress` (`172.29.81.14`, `loopback0 198.51.100.4/32`, `mpls rsvp4`).

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Pure RFC 2205 wire/engine signaling fix; no new config leaves or RPCs. |
| YANG validation constraints | N-A | No YANG schema additions. |
| YANG custom validators | N-A | No YANG schema additions. |
| CLI commands/flags | No | No new or changed CLI commands or flags. |
| CLI grammar (keyword before value) | N-A | No CLI commands added or changed. |
| Editor autocomplete | N-A | No YANG or CLI additions. |
| Functional test for new RPC/API | N-A | No new RPC or API; existing `test/rsvpte/*.ci` suite covers config/show regression and `TestIntegrationRSVPNonRSVPHopDetection` covers real-kernel wire behavior. |
| Pipe completeness | N-A | No new command output. |
| Env var registration | No | No new environment variables. |
| Doctor check for runtime dependencies | No | Uses existing raw IPv4 protocol-46 socket already covered by `internal/plugins/rsvpte/doctor.go`. |
| Doctor code per problem class | No | Non-RSVP hop detection is normal RFC 2205 protocol signaling (sets ADSPEC Global Break bit, does not refuse config or emit a local fault). |
| Prometheus counters/metrics | No | No new metrics required by RFC 2205 §3.8 or operator surface; existing `ze_rsvpte_path_recv_total` / `ze_rsvpte_path_sent_total` continue counting PATH messages. |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not a BGP family change. |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | `features/mpls-rsvpte.md` / `docs/features.md` stays Experimental (partial); non-RSVP hop detection is an RFC 2205 protocol correctness fix, not a new feature level. |
| 2 | Config syntax changed? | No | No YANG or config syntax changes. |
| 3 | CLI command added/changed? | No | No CLI command changes. |
| 4 | API/RPC added/changed? | No | No API or RPC changes. |
| 5 | Plugin added/changed? | No | Existing `rsvp-te` plugin registration and lifecycle are unchanged. |
| 6 | Has a user guide page? | No | `docs/guide/rsvp-te.md` covers operator configuration and CLI; no config or CLI syntax changes. |
| 7 | Wire format changed? | No | `docs/architecture/wire/` covers BGP wire formats; RSVP-TE wire behavior lives in `docs/architecture/rsvpte/mpls-rsvp-te.md` (updated under item 12/16). |
| 8 | Plugin SDK/protocol changed? | No | No changes to `pkg/plugin/` or process protocol. |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2205.md` (`RFC2205-3-30` compliance checklist row + discrimination record via `./le rfc record`, and `docs/features/rfc-status.md` if RFC 2205 met count updates). |
| 10 | Test infrastructure changed? | No | Uses existing `userns`/`rsvpWireLab` and `interop-rsvpte` 4-role lab runner without changing harness mechanics. |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` does not track per-bit ADSPEC break signaling. |
| 12 | Internal architecture changed? | Yes | `docs/architecture/rsvpte/mpls-rsvp-te.md` (new decision section for RFC 2205 Section 3.8 TTL vs `Send_TTL` non-RSVP hop detection and ADSPEC Global Break bit). |
| 13 | Route metadata keys added/changed? | No | No BGP route metadata keys touched. |
| 14 | Prometheus counters added/changed? | No | No metrics added or changed. |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registry entries added or changed. |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived via `./le spec citation anchors`: `docs/architecture/rsvpte/mpls-rsvp-te.md` (updated), `docs/architecture/mpls/mpls-kernel.md` (unaffected: `PathMTU = 0` and `mplsfib.PathMTUMinimum` floor semantics are preserved), `docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md` (unaffected: merge-point matching and backup control path behavior are preserved, only `NonRSVPHop` propagation added internally), `docs/architecture/testing/interop.md` (unaffected: `interop-rsvpte` suite discovery and kernel-capability declarations are unchanged), `docs/features.md` (unaffected: high-level RSVP-TE summary unchanged), `ai/digests/mpls-signaling.md` (unaffected: high-level PATH/RESV/FIB flow unchanged). |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No config, CLI, or API syntax changes. |

## Design presentation

### What changes

When a Ze RSVP-TE transit or egress node receives a PATH message whose IPv4 header TTL (`Packet.TTL`) differs from the RSVP Common Header `Send_TTL` (`Header.TTL`), Ze detects that the PATH traversed one or more non-RSVP IP routers (`RFC2205-3-30`) and passes that flag to its traffic-control OPWA ADSPEC path (`mtu.go::updateAdspec` / `markAdspecNonRSVPHop`). At transit, Ze sets the RFC 2210 Section 3.3.2 Global Break bit (`0x80` at byte 9 of ADSPEC) on the relayed PATH, bounds hop-limit checks and relayed `Send_TTL` by `min(Packet.TTL, Send_TTL)` so non-RSVP hops cannot be re-inflated, and suppresses unreliable ADSPEC Path MTU programming when the RESV returns. At egress, Ze sets the Global Break bit on its local PSB ADSPEC so it echoes `MaxPacketSize = 0` in RESV and installs `PathMTU = 0` on the egress pop. FRR backup PATHs and merge-point recomputation carry per-branch `NonRSVPHop` state so break bits are neither dropped during local repair nor stuck after a non-RSVP branch departs. Direct RSVP hops (`Packet.TTL == Send_TTL`) behave identically to today.

### Before and after

1. **Transit PATH across a non-RSVP IPv4 router (`Send_TTL = 64`, arriving `IPv4 TTL = 63`, ADSPEC `PathMTU = 1500`):**
   - Before: Ze ignores `Packet.TTL`, relays the PATH with `Send_TTL = 63` (from `64 - 1`), leaves ADSPEC byte 9 Global Break bit clear (`0x00`), and later installs the unreliable `PathMTU` on the swap entry.
   - After: Ze detects `63 != 64`, sets ADSPEC byte 9 Global Break bit (`0x80`), relays the PATH with `Send_TTL = 62` (`min(63, 64) - 1`), keeps `ReceivedAdspec` untouched for upstream `PathErr`, and installs `PathMTU = 0` when the RESV returns.
2. **Egress PATH across a non-RSVP IPv4 router (`Send_TTL = 64`, arriving `IPv4 TTL = 63`, ADSPEC `PathMTU = 1500`):**
   - Before: Ze echoes `MaxPacketSize = 1500` in the RESV FLOWSPEC and installs `PathMTU = 1500` on the egress pop.
   - After: Ze marks its local PSB ADSPEC with the Global Break bit, sends RESV with `MaxPacketSize = 0`, and installs `PathMTU = 0` on the egress pop while still establishing the LSP.
3. **Hop-budget exhaustion across non-RSVP routers (`Send_TTL = 4`, arriving `IPv4 TTL = 1`):**
   - Before: Transit sees `Send_TTL = 4 > 1`, relays the PATH with `Send_TTL = 3` and outgoing IPv4 TTL `3`, re-inflating the exhausted hop budget.
   - After: Transit computes `effectivePathTTL(1, 4) = 1 <= 1` and drops the PATH without installing state or relaying it.
4. **Direct RSVP hop (`Send_TTL = 64`, arriving `IPv4 TTL = 64`, ADSPEC `PathMTU = 1500`, outgoing link MTU `1400`):**
   - Before: Relayed PATH has `Send_TTL = 63`, ADSPEC Global Break bit clear (`0x00`), composed `PathMTU = 1400`.
   - After: Identical (`Send_TTL = 63`, Global Break bit clear `0x00`, composed `PathMTU = 1400`).

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Rejecting PATH messages with `PathErr` whenever `Packet.TTL != Send_TTL` | Violates RFC 2205 Sections 2.9 and 3.8, which require RSVP to operate across non-RSVP clouds and report the QoS gap to traffic control / receivers via the ADSPEC Global Break bit rather than dropping the reservation. |
| Synthesizing a new ADSPEC object on transit when the sender omitted ADSPEC | RFC 2205 Section 3.1.3 makes ADSPEC optional; fabricating an ADSPEC when a sender never requested OPWA alters wire object presence unexpectedly and is unnecessary because `adspecPathMTU(nil, ...)` is already `0`. |
| Changing `DecodeMessage` to take `Packet.TTL` directly | `DecodeMessage` is a pure RSVP-payload wire codec reused by decode tests without IP headers; comparing `Packet.TTL` with `msg.Header.TTL` belongs in `engine.go::handlePath`, which owns both the transport `Packet` and the decoded `ParsedMessage`. |
| Treating `Packet.TTL == 0` as a non-RSVP hop | Breaks dozens of existing unit tests that construct synthetic `Packet{Src, Payload}` literals without setting `TTL`, with zero security or protocol benefit because IPv4 forwarding drops `TTL <= 1` before forwarding and never delivers a real forwarded packet with `TTL == 0`. |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase 1: Wiring (MANDATORY FIRST)** -- thread `pkt.TTL` from `handlePacket` into `handlePath` / `handlePathEgress` / `handlePathTransit` / `mergeBackupPath`, add `NonRSVPHop` to `pathStateBlock` and `mergedPath`, extend `updateAdspec` with `nonRSVPHop bool`, add `markAdspecNonRSVPHop`, and write failing wiring tests (`TestRFC2205NonRSVPHopTransitSetsAdspecBreakBit`, `TestRFC2205NonRSVPHopEgressSuppressesUnreliableMTU`).
   - Tests: `TestRFC2205NonRSVPHopTransitSetsAdspecBreakBit`, `TestRFC2205NonRSVPHopEgressSuppressesUnreliableMTU`
   - Files: `internal/plugins/rsvpte/mtu.go`, `internal/plugins/rsvpte/fsm.go`, `internal/plugins/rsvpte/engine.go`, `internal/plugins/rsvpte/frr.go`, `internal/plugins/rsvpte/rfc2205_non_rsvp_hop_test.go`
   - Verify: entry point compiles and wiring tests fail against stubbed detection, then pass once detection and break-bit composition are active.
2. **Phase 2: Complete TTL detection, hop-limit bounding, egress suppression, and FRR/merge lifecycle** -- implement `nonRSVPHopDetected`, `effectivePathTTL`, `updateAdspec` break-bit logic, `markAdspecNonRSVPHop`, egress MTU suppression, `backupPath` and `recomputeMergedAdspecLocked` propagation, and `fabricPort.SendPath` TTL population; write all unit and boundary tests with `RFC requirement: RFC2205-3-30 positive` and `RFC requirement: RFC2205-3-30 negative` tags.
   - Tests: `TestRFC2205NonRSVPHopDetectionBoundaries`, `TestRFC2205AdspecNonRSVPHopFlag`, `TestRFC2205DirectHopLeavesAdspecBreakBitClear`, `TestRFC2205NonRSVPHopTTLExhaustedDropped`, `TestRFC2205NonRSVPHopRefreshAndFRRLifecycle`, plus updated `mtu_test.go`
   - Files: `internal/plugins/rsvpte/mtu.go`, `internal/plugins/rsvpte/fsm.go`, `internal/plugins/rsvpte/engine.go`, `internal/plugins/rsvpte/frr.go`, `internal/plugins/rsvpte/mtu_test.go`, `internal/plugins/rsvpte/interop_test.go`, `internal/plugins/rsvpte/rfc2205_non_rsvp_hop_test.go`
   - Verify: tests fail before implementation, pass with `-race` under `./le test unit pkg path ./internal/plugins/rsvpte/`.
3. **Phase 3: Real Linux namespace integration proof, freeRouter interop scenario, RFC records, and architecture documentation** -- add `TestIntegrationRSVPNonRSVPHopDetection` in `rfc2205_transport_integration_linux_test.go` (3-namespace lab with an intermediate non-RSVP IPv4 router), add the freeRouter interop scenario `test/interop-rsvpte/scenarios/transit-non-rsvp-hop-adspec-break/` and its checker/catalog tests in `internal/le/interoplab/rsvpte/` (proving the scenario red with the Ze fix reverted and the lab image rebuilt, then green with the fix restored), update `docs/architecture/rsvpte/mpls-rsvp-te.md` and `rfc/short/rfc2205.md`, and record `RFC2205-3-30` discrimination records via `./le rfc record` + `./le rfc check`.
   - Tests: `TestIntegrationRSVPNonRSVPHopDetection`, `./le test unit pkg path ./internal/le/interoplab/rsvpte/`, `RSVPTE_INTEROP_SCENARIO=transit-non-rsvp-hop-adspec-break ./le test integration interop-rsvpte`, `./le test integration interop-rsvpte`, `./le test rsvpte`, `./le rfc check`, `./le doc check all`
   - Files: `internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go`, `internal/le/interoplab/rsvpte/rsvpte.go`, `internal/le/interoplab/rsvpte/checkers.go`, `internal/le/interoplab/rsvpte/rsvpte_test.go`, `test/interop-rsvpte/scenarios/transit-non-rsvp-hop-adspec-break/*`, `docs/architecture/rsvpte/mpls-rsvp-te.md`, `rfc/short/rfc2205.md`
   - Verify: Linux integration test and freeRouter interop scenario both prove real IPv4 TTL decrement -> ADSPEC Global Break bit + `Send_TTL` bounding on the wire, and both fail red when detection is reverted; RFC check and doc check pass cleanly.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-7 has an implementation at `file:line` and passing test proof |
| Feature completeness | Every user story has a working path from `Packet.TTL` to wire ADSPEC / FIB `PathMTU` |
| Correctness | `Packet.TTL == 0` never causes false non-RSVP detection; `effectivePathTTL` never re-inflates TTL; `ReceivedAdspec` is never mutated in place |
| Concurrency | All `PSB.NonRSVPHop`, `PSB.Adspec`, and `MergedPaths` reads/writes stay under `lsp.mu` (`LSP.locked`) with no lock inversion |
| Data flow | Detection happens once in `handlePath` from `Packet.TTL` and `Header.TTL`, and passes `nonRSVPHop` through `updateAdspec` / `markAdspecNonRSVPHop` |
| Operator errors and doctor codes | Existing `PathErr 21/05` (`BadAdspec`) floor refusal continues to fire on unbroken below-floor ADSPECs; no new refusal or doctor code is introduced |
| Rule: `ai/rules/rfc-compliance.md` | `// RFC 2205 Section 3.8:` and `// RFC 2210 Section 3.3.2:` quotes sit above enforcing code; both `RFC2205-3-30 positive` and `RFC2205-3-30 negative` tags carry discrimination records |
| Rule: `ai/rules/interop-and-goal-validation.md` | `transit-non-rsvp-hop-adspec-break` is proved red with the Ze fix reverted and lab image rebuilt, then green with the fix restored |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| TTL vs `Send_TTL` non-RSVP hop detection and hop-limit bounding | `grep -n "nonRSVPHopDetected\|effectivePathTTL" internal/plugins/rsvpte/engine.go` and `./le test unit pkg path ./internal/plugins/rsvpte/ run TestRFC2205NonRSVPHop` |
| `TC_Advertise` Global Break bit in `mtu.go` | `grep -n "func updateAdspec\|func markAdspecNonRSVPHop" internal/plugins/rsvpte/mtu.go` and `./le test unit pkg path ./internal/plugins/rsvpte/ run TestRFC2205AdspecNonRSVPHopFlag` |
| FRR PLR and merge-point `NonRSVPHop` propagation | `grep -n "NonRSVPHop" internal/plugins/rsvpte/fsm.go internal/plugins/rsvpte/frr.go` and `./le test unit pkg path ./internal/plugins/rsvpte/ run TestRFC2205NonRSVPHopRefreshAndFRRLifecycle` |
| Linux 3-namespace non-RSVP router proof | `./le test unit pkg path ./internal/plugins/rsvpte/ run TestIntegrationRSVPNonRSVPHopDetection` (in Linux/QEMU environment with user namespaces) |
| freeRouter interop scenario `transit-non-rsvp-hop-adspec-break` | `./le test unit pkg path ./internal/le/interoplab/rsvpte/` and `RSVPTE_INTEROP_SCENARIO=transit-non-rsvp-hop-adspec-break ./le test integration interop-rsvpte` (plus revert-and-rebuild red proof) |
| RFC 2205 Section 3.8 architecture decision and RFC compliance records | `./le doc check all` and `./le rfc check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Untrusted `Send_TTL` / IPv4 TTL mismatch | Crafted packet with `Send_TTL = 0` or `Send_TTL > Packet.TTL` or `Packet.TTL = 1` cannot cause underflow (`uint8` wraparound to `255`), TTL re-inflation, or panic (`effectiveTTL <= 1` checked before `effectiveTTL - 1`). |
| Malformed ADSPEC with non-RSVP hop | `markAdspecNonRSVPHop` and `updateAdspec` both run `validateAdspec(raw)` before indexing `raw[9]`, preventing out-of-bounds panic on short/corrupted buffers. |
| Memory aliasing | `psb.Adspec` is cloned from `msg.AdspecRaw` before setting `raw[9] \|= intservBreak`, so `psb.ReceivedAdspec` and receive buffers are never mutated. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read `engine.go`, `mtu.go`, `frr.go`, and `transport_linux.go`; if RFC semantics differ, return to DESIGN |
| Lint failure | Fix inline; if architectural, return to DESIGN |
| Linux namespace test fails | Verify IPv4 forwarding (`ip_forward=1`) and routing in intermediate non-RSVP namespace; verify `Packet.TTL` decrement |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- On Linux, `IP_ROUTER_ALERT` diverts transit PATH packets in `ip_forward()` before `ip_decrease_ttl()`, whereas an intermediate non-RSVP IPv4 router runs `ip_decrease_ttl()`. That makes `Packet.TTL == Send_TTL` hold on direct RSVP hops and `Packet.TTL < Send_TTL` hold whenever one or more non-RSVP IPv4 routers intervene.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Detect non-RSVP hops via `Packet.TTL != 0 && Packet.TTL != Header.TTL` in `handlePath` | Putting TTL check in `DecodeMessage`; rejecting `Packet.TTL == 0` | Keeps `DecodeMessage` a pure wire codec, matches RFC 2205 §3.8 literally, and preserves synthetic unit tests whose `Packet.TTL` is unset while real IPv4 forwarding never delivers `TTL == 0`. |
| Pass `nonRSVPHop` into `mtu.go::updateAdspec` (and `markAdspecNonRSVPHop` for egress/merge) to set byte 9 `intservBreak` | Separate ADSPEC post-processor; rejecting PATH with `PathErr` | Extends Ze's existing `TC_Advertise` producer (`updateAdspec`) and matches RFC 2205 §2.9/§3.8/§3.11.2 and RFC 2210 §3.3.2 without duplicating ADSPEC logic. |
| Bound transit hop limit and relayed `Send_TTL` by `effectivePathTTL = min(Packet.TTL, Send_TTL)` (when `Packet.TTL > 0`) | Continuing to decrement `Header.TTL` alone | Prevents non-RSVP-burned IP TTL from being re-inflated on transit relay and keeps the downstream `Send_TTL` equal to the outgoing IPv4 TTL. |

## Known Limitations
- Non-RSVP neighbor tracking with `LABEL_REQUEST` suppression (`RFC3209-4.2.5-1`) and routing-protocol notification of IP-encapsulating tunnels (`RFC2205-3-31`) were split out by owner decision (2026-11-24) into `plan/post-release/features/spec-rsvpte-non-rsvp-clouds.md`.
- General non-RSVP-cloud routing/interface integration (`RFC2205-2-11`, `RFC2205-2-12`, `RFC2205-3-6`, `RFC2205-3-21`) is owned by `plan/release/fixes/spec-rsvp-routing-and-interface-integration.md`.

## RFC Documentation (Scope: protocol)

Add `// RFC 2205 Section 3.8:` and `// RFC 2210 Section 3.3.2:` quotes above the enforcing functions (`nonRSVPHopDetected`, `effectivePathTTL`, `updateAdspec`, `markAdspecNonRSVPHop`).

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 2205 Section 3.8 / Section 3.11.2 (`RFC2205-3-30`) and RFC 2210 Section 3.3.2 | Default `rsvp-te` configuration (no new option required): compare arriving IPv4 `Packet.TTL` against RSVP Common Header `Send_TTL` on every PATH; when `Packet.TTL > 0 && Packet.TTL != Send_TTL`, set the ADSPEC Global Break bit (`0x80` at byte 9) in traffic control (`updateAdspec` / `markAdspecNonRSVPHop`), suppress unreliable ADSPEC Path MTU on egress and returning RESV, and bound transit relay `Send_TTL` by `min(Packet.TTL, Send_TTL) - 1`. | Direct-hop ADSPEC Path MTU discovery, RFC 4090 facility-backup FRR local repair and merge-point ADSPEC recomputation, and `PathErr 21/05` floor refusal on unbroken ADSPECs remain preserved. | `TestRFC2205NonRSVPHopTransitSetsAdspecBreakBit`, `TestRFC2205DirectHopLeavesAdspecBreakBitClear`, `TestRFC2205NonRSVPHopEgressSuppressesUnreliableMTU`, `TestIntegrationRSVPNonRSVPHopDetection`, `transit-non-rsvp-hop-adspec-break` | `TestRFC2205NonRSVPHopRefreshAndFRRLifecycle`, `TestTransitForwardsComposedAdspecMTU`, existing `pathmtu_floor_test.go` and `interop-rsvpte` suite |

## Review Gate

Pending implementation: `/ze-review` and `/ze-review-spec` run after implementation completes, with `plan/templates/TEMPLATE-CLOSURE.md` appended by `/ze-close`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced (`rfc/short/rfc2205.md`)
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions with positive Expected Behavior and Permitted Survivors
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist answered Yes / No / N-A for every row

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: `./le test unit pkg path ./internal/plugins/rsvpte/`, `./le test unit pkg path ./internal/le/interoplab/rsvpte/`, `./le test rsvpte`, `RSVPTE_INTEROP_SCENARIO=transit-non-rsvp-hop-adspec-break ./le test integration interop-rsvpte`, `./le test integration interop-rsvpte`, `./le rfc check`, and `./le doc check all`. GitHub CI verifies the pushed commit.
- [ ] Feature code integrated (`internal/plugins/rsvpte/`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket (`plan/post-release/features/spec-rsvpte-non-rsvp-clouds.md`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` regression suite (`test/rsvpte/`) and Linux namespace integration test (`TestIntegrationRSVPNonRSVPHopDetection`)
- [ ] Interop tests for protocol features (Linux forwarding namespace + freeRouter regression suite)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-rsvpte-non-rsvp-neighbors.md` only, in the same `./le commit create` script
