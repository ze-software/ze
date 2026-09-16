# Spec: ike-padded-path-probe

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 6/6 |
| Handoff | - |
| Updated | 2026-09-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`show mtu` (`docs/architecture/diagnostics/path-mtu.md`, closed as spec-path-mtu-diagnostic) measures a path with ICMP echo, and prints
the caveat that the figure is optimistic: a path can treat ICMP, UDP/4500 and
ESP differently, and the number that matters for a tunnel is the one the ESP
traffic actually meets. The VyOS tool it was ported from prints the same caveat
and cannot do better, because it is not the IKE daemon.

Ze is. An INFORMATIONAL exchange padded to a chosen size elicits an
authenticated reply from the peer over exactly the port, the encapsulation and
the path the ESP traffic rides. The reply coming back proves the request fit the
path. That removes the caveat for any peer with a live IKE SA rather than
printing it.

Scope: a padded INFORMATIONAL probe on the IKE engine, offered to the MTU module
as the prober used when an SA is up. ICMP stays the path for
`show mtu host <address>`, for the reference measurement, and for a tunnel that
is down, so this replaces no existing prober.

Open questions this spec answers before design:

- Which padding mechanism. RFC 7296 Section 3.14 pads the encrypted payload; a
  Notify payload carrying opaque data is another route. The choice is made
  against the RFC text and must not be one a peer can read as malformed.
- What a non-answer means. An ICMP probe separates "too big" from "lost" using
  the kernel error queue. A missing INFORMATIONAL reply has more causes, and
  RFC 4821 Section 7.6.4, which forbids moving the search bounds when loss may be
  congestion, binds here too.
- The retransmission interaction. IKE already retransmits an unanswered request,
  so a probe must not be retried by two mechanisms with two budgets.
- Whether a peer that is not Ze answers a padded INFORMATIONAL at all. That is an
  interop question against strongSwan and libreswan before it is a design
  question, and it decides whether this is general or Ze-to-Ze only.

Owner decision, 2026-09-11: ICMP ships first, and this is homed here so it is
scheduled rather than forgotten.

Answers, from RESEARCH and the owner's DESIGN decisions of 2026-09-16:

| Question | Answer |
|----------|--------|
| Padding mechanism | One status Notify of a private-use type whose Notification Data pads the datagram. The SK Pad Length is one octet (RFC 7296 Section 3.14), so SK padding caps at 255 octets and cannot reach 3000; an AEAD SK pads nothing (RFC 5282). A status Notify "MUST be ignored if not recognized" (RFC 7296 Section 3.10.1), and both strongSwan and libreswan answer it with an empty INFORMATIONAL response. Any payload TYPE the peer does not know is refused by libreswan with INVALID_SYNTAX and an SA delete, so the carrier is a Notify and never another payload type |
| Non-answer | Two copies of one message separate the causes. The first copy carries DF; the retransmission is the bitwise-identical IKE message with DF clear (RFC 7296 Section 2.1 lets the IP and UDP headers differ). An answer to the DF copy means the size fits. An answer only after the DF-clear copy means the size is too big. Silence through the ordinary retransmit budget, DF-clear copies included, deems the IKE SA failed, as it does for any unanswered request (RFC 7296 Section 2.1); the MTU module therefore probes only at or below the ICMP figure and stops at the first silent size. An ICMP too-big read off the IKE socket's error queue triggers the DF-clear copy at once |
| Retransmission | The probe is an ordinary IKE request under RFC 7296 Section 2.1: `serviceRequestRetransmit` repeats it on the ordinary schedule (`maxRequestRetransmits` 3, 500 ms doubling to a 60 s cap), every repeat with DF clear, and `serviceRequestWindow` fails the SA past the budget exactly as it does for a Delete. It gains NO probe-aware branch: no window is released without a response and no message id is rewound. The MTU module keeps a budget of exchanges rather than a budget of retries. Owner ruling (a), 2026-09-16 |
| Third-party peers | strongSwan `process_request` (`task_manager_v2.c`) answers a request carrying only an unknown status Notify with an empty response and keeps the SA; libreswan does the same for a Notify. Read from source (owner decision 2026-09-16: no libreswan lab peer; the strongSwan scenario is the live proof). The feature is general, not Ze-to-Ze only |

Owner decisions, 2026-09-16: the shared IKE socket with DF toggled per
datagram, one status Notify as the carrier, no RFC 7383 implementation first
(`plan/immediate/spec-ike-fragmentation-rfc7383.md` follows this spec), and no
libreswan lab peer.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-7-ikev2-engine.md` - the exchange machinery this extends
  → Constraint: the page is SILENT on INFORMATIONAL, DPD, the request window, retransmission and the SK builder. It gains the padded exchange's wire shape (phase 6) and a pointer to the established-loop section below
  → Decision: `maintainSA` (`established.go`) owns `sa.NextMsgID`, the keys and the window on one goroutine. Every probe is built and sent on that goroutine, reached through a request channel beside `stopCh` and `supersede`, never from the MTU module's goroutine
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - the page `established.go`, `inbound.go` and `dpd.go` DECLARE in their `// Design:` headers (established SA lifecycle, inbound classification, DPD)
  → Constraint: the page's "Inbound classification" section names `inbound.go` and the DPD source anchor names `dpd.go`, and nothing on it describes the request window, retransmission or the owner loop's INFORMATIONAL handling. The section this spec writes on INFORMATIONAL, DPD, the window, retransmission and the probe lands HERE, because this is the page the changed files declare; ipsec-7 points at it
- [ ] `docs/architecture/diagnostics/active-probes.md` - the page every `internal/core/probe` file declares
  → Constraint: "The Don't Fragment mode" and "The error queue" describe the option mapping and the drain this spec exports for the transport; the page gains the sentence that a foreign UDP socket (the IKE transport) reuses both, and its anchors are re-verified after the export
- [ ] `docs/architecture/wire/buffer-writer.md` - the page `wire/payload_notify.go` declares
  → Constraint: the Notify writes through `WriteTo(buf, off)` into the SK builder's buffer; the private-use constant changes no encoding rule, so the page is named as unaffected with that reason in row 16
- [ ] `docs/architecture/testing/qemu-integration.md` - the page `internal/le/qemu/alltests.go` declares
  → Constraint: "Any new Linux-only package" owes an entry in `integrationPackages`; `ike/transport` gains its first `integration && linux` test and joins the list, and the page's package table gains that row
- [ ] `docs/architecture/ike/ipsec-13-rekey-wire.md` - the message id and window rules the probe shares with rekey
  → Constraint: the sentence "Responses to Ze's own probes and Delete requests land in the owner loop and are dropped as out of window" is STALE: the `inboundInvalid` arm of `handleOwnedInbound` (`inbound.go`) decrypts the response and frees the window through `answerAuthenticatedResponse`. Repaired in phase 1, before any code edit touches that arm
- [ ] `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` - NAT-T and the port float
  → Constraint: the page is silent on which port post-establishment messages take. `SA.sendPath` (`sa.go`) answers: `localPort == NATTPort` selects the 4500 socket and the RFC 3948 marker; otherwise UDP/500. The page gains that sentence in phase 6
- [ ] `docs/architecture/diagnostics/path-mtu.md` - the consumer: the `prober` interface, the search, the payload
  → Decision: `pathSearch.confirm` (`search.go`) already confirms a reported figure (size passes, size+1 fails) before it descends the ladder. The IKE prober plugs into the same `prober` interface and the same confirm-first strategy; the MTU module never builds an IKE message
  → Constraint: every measurement row of "The payload" carries `method`; this spec adds `prober` (`ike` or `icmp`) beside it and keeps every existing key
- [ ] `docs/architecture/core-design.md` - where a core leaf sits and how a component registers into it
  → Constraint: `internal/core/ipsecinventory` is the model: `Register` once at `init` (panics on nil or a second call), a query that answers `ErrNotRegistered` on a build without the engine. `internal/core/ikeprobe` copies that shape exactly

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` and `rfc/full/rfc7296.txt` - IKEv2: sizes, retransmission, message ids, the window, NAT-T, Notify, padding
  → Constraint: Section 2: "All IKEv2 implementations MUST be able to send, receive, and process IKE messages that are up to 1280 octets long, and they SHOULD be able to send, receive, and process messages that are up to 3000 octets long." Ze's `MaxMsgSize` (`transport/udp.go`) is 3000, so the probe ceiling is min(interface MTU, 3000)
  → Constraint: Section 2.1: "the initiator MUST retransmit a request until it either receives a corresponding response or deems the IKE SA to have failed" and "A retransmission from the initiator MUST be bitwise identical to the original request. That is, everything starting from the IKE header (the IKE SA initiator's SPI onwards) must be bitwise identical; items before it (such as the IP and UDP headers) do not have to be identical." The DF-clear copy is the same IKE bytes; only the IP header changes
  → Constraint: Section 2.2: the Message ID is "incremented for each subsequent exchange", and each endpoint keeps "the next one to be used for a request it initiates". A probe consumes one id per size and advances `sa.NextMsgID` as `sendDPD` does; an id is never reused and never skipped
  → Constraint: Section 2.3: "An IKE endpoint MUST wait for a response to each of its messages before sending a subsequent message unless it has received a SET_WINDOW_SIZE Notify". Ze declares a window of one, so exactly one probe is outstanding at a time and no DPD, Delete or rekey goes out while it is
  → Constraint: Section 2.23: "The UDP payload of all packets containing IKE messages sent on port 4500 MUST begin with the prefix of four zeros". The four octets count toward the requested wire size
  → Constraint: Section 3.10.1: "Notify payloads with status types MAY be added to any message and MUST be ignored if not recognized." The carrier is a status Notify of a private-use type. The 40960-65535 private-use range is IANA "IKEv2 Notify Message Status Types" registry data, not RFC text
  → Constraint: Section 3.14: "Pad Length is the length of the Padding field", a one-octet field, so SK padding tops out at 255 octets and cannot reach the sizes a path probe needs
- [ ] `rfc/short/rfc5282.md` - AEAD SK: no padding at all
  → Constraint: an AEAD-keyed SA pads nothing, so SK padding is not a carrier on any suite; the Notify carrier is the same on every suite
- [ ] `rfc/full/rfc7383.txt` - IKEv2 fragmentation, the neighbor of a path probe
  → Constraint: Section 2.4: "IKE fragmentation MUST NOT be used unless both peers have indicated their support for it. After that, it is up to the initiator of each exchange to decide whether or not to use it." Ze negotiates none today (`NotifyFragmentationSupported` is a constant in `wire/payload_notify.go`, never sent), and the initiator of the probe exchange decides NOT to fragment it, ever
  → Constraint: Section 2.5.2: "implementing PMTU discovery in IKE is OPTIONAL" and its search "is performed downward" by refragmenting at smaller thresholds. A path probe that was IKE-fragmented would measure the fragment threshold rather than the path, so a future RFC 7383 implementation MUST never fragment a probe (`plan/immediate/spec-ike-fragmentation-rfc7383.md` carries the constraint)
- [ ] `rfc/full/rfc4821.txt` - Packetization Layer PMTUD
  → Constraint: Section 7.6.4: "the state variables eff_pmtu, search_low, and search_high SHOULD NOT be updated, and the same-sized probe SHOULD be attempted again". One silence moves no bound
- [ ] `rfc/full/rfc8899.txt` - DPLPMTUD for datagram protocols
  → Constraint: Section 4.1: "Protocols that permit exchange of control messages (without an application data block) can generate a probe packet by extending a control message with padding data. The total size of a probe packet includes all headers and padding". The size budget counts IP, UDP, the marker, the IKE header, the SK overhead and the Notify
  → Constraint: Section 5.1.2 recommends `MAX_PROBES` of 3, and Section 5.1.3 states "loss of a single probe is not an indication of a PMTU problem". Three probes per size before the size is declared too big
- [ ] `rfc/short/rfc3948.md` and `rfc/full/rfc3948.txt` - UDP encapsulation
  → Constraint: Section 2.2: "A Non-ESP Marker is 4 zero-valued bytes aligning with the SPI field". On 4500 the probe rides the same encapsulation ESP rides, marker included
- [ ] `rfc/short/rfc4555.md` and `rfc/full/rfc4555.txt` - MOBIKE path testing, the precedent for the idea
  → Decision: Section 3.10: "the initiator can use normal IKEv2 INFORMATIONAL request/response messages to test whether a certain path works". Ze implements no MOBIKE; the section is cited as the precedent that an INFORMATIONAL exchange is a legitimate path test, nothing more

**Key insights:** (minimal context to resume after compaction)
- Carrier: exactly one status Notify, private-use type, opaque data pads the DATAGRAM to the requested wire size (IP + UDP + marker on 4500 + IKE header + SK + Notify). Never another payload type.
- Two copies, one message id: DF copy first; on silence or on an error-queue too-big, the bitwise-identical IKE bytes again with DF clear. Answered on DF = fits; answered after DF-clear = too big; unanswered after the full budget = the SA is deemed failed (RFC 7296 Section 2.1), the outcome is `sa-failed`, and the MTU module tries no further size.
- The owner loop builds and sends; the MTU module asks through `internal/core/ikeprobe` and gets a typed outcome. Window one: one probe outstanding, an exchange budget of 16 per run bounds how long DPD waits.
- A probe never touches `dpdState`; a probe answered on either copy never sets `StateDead`; a probe unanswered after the full budget fails the SA as any request does (Section 2.1). Liveness stays DPD's job.
- The mitigation is the MTU module's: confirm-first (probe by IKE only at or below the ICMP figure), stop at the first silent size, and report that tunnel's figure as ICMP-measured and unconfirmed.
- Ceiling min(interface MTU, 3000); never IKE-fragmented; no second socket.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/ike/engine/established.go` - `maintainSA` runs the owner loop on a one-second ticker and owns `sa.NextMsgID`, the keys and the request window. `serviceRequestRetransmit` repeats `sa.requestMsg` on `shouldRetransmitRequest` (`maxRequestRetransmits` 3, 500 ms doubling to 60 s, `msgid.go`) unless a rekey is pending or DPD awaits a reply. `serviceRequestWindow` fails the SA (`StateDead`) when a non-DPD, non-rekey holder passes `requestWindowTimeout` (30 s), citing RFC 7296 Section 2.1's two exits. `sendRaw` writes through `sa.sendPath`
- [ ] `internal/component/ike/engine/inbound.go` - `handleOwnedInbound`: the `inboundInvalid` arm decrypts an INFORMATIONAL response that matches no pending rekey, frees the window with `answerAuthenticatedResponse`, and reports `dpdResp` plus `dpdRespMsgID`; `maintainSA` credits liveness only when `dpd.matchesProbe` agrees. `handleInformationalOwned` answers a request that carries no Delete with an empty response; unrecognized notifies go through `logIgnoredNotifies` (RFC 7296 Section 3.10.1)
- [ ] `internal/component/ike/engine/dpd.go` - `dpdState` (`probeMsgID`, `awaitReply`, `lastSent`, `retries`, `matchesProbe`); `sendDPD` builds on the owner goroutine, returns before `reserveRequestWindow` when the window is held, then `advanceMsgID`
- [ ] `internal/component/ike/engine/msgid.go` - `reserveRequestWindow`, `releaseRequestWindow`, `armRequestRetransmit`, `answerAuthenticatedResponse`, `requestWindowStale`, `advanceMsgID`; `classifyInbound` yields `inboundInvalid` for a response id that matches no rekey
- [ ] `internal/component/ike/engine/reconcile.go` - `PeerSession` carries `stopCh`, `supersede`, `pendingRekey`, `ikeRekeyHoldUntil`; `run` starts the loop; no request channel exists today
- [ ] `internal/component/ike/engine/sa.go` - `SA.sendPath` picks the 4500 socket (marker) when `localPort == NATTPort`, else the 500 socket; `SAState` with `StateDead`; `NextMsgID`, `requestMsg`, `requestMsgID`, `requestOutstanding`
- [ ] `internal/component/ike/engine/register.go` - `init` calls `ipsecinventory.Register(inventorySnapshot)`; `ActivePeers`, `lookupPeerSession`
- [ ] `internal/component/ike/engine/inventory.go` - `inventorySnapshot` fills `ipsecinventory.Tunnel` (`Up`, `Peer`, `UDPEncap`)
- [ ] `internal/component/ike/engine/auth.go` - `buildSKMessageCBCWithMsgID` pads to the cipher block only; the AEAD builder pads nothing
- [ ] `internal/component/ike/transport/udp.go` - `MaxMsgSize` 3000, `Send` holds `mu` only for the closed check and writes outside it, `Run` reads the socket into `inbound`; no `IP_MTU_DISCOVER`, no `IP_RECVERR`, no error-queue read. `nat.go` holds `NATTPort` 4500; `encap_linux.go` and `encap_other.go` are the existing platform split
- [ ] `internal/component/ike/wire/payload_notify.go` - `PayloadNotify` (`NotifyMsgType`, `NotificationData`, `WriteTo`, `Len`), `NotifyStatusFloor`, `NotifyTypeRecognized`, `NotifyFragmentationSupported`; no private-use constant
- [ ] `internal/core/ipsecinventory/registry.go` - `Register`, `Tunnels`, `ErrNotRegistered`, `Tunnel.Up`; the leaf shape `ikeprobe` copies
- [ ] `internal/core/probe/errqueue.go`, `errqueue_linux.go`, `socket_linux.go`, `df.go` - `QueuedError` (`MTU`, `Offender`, `Outcome`), `drainErrorQueue` (unexported), `dfOptionsOf` mapping `DFMode` to `IP_PMTUDISC_DO`/`PROBE`/`DONT` and `IP_RECVERR` (unexported), `DFHonorCache`, `DFBypassCache`
- [ ] `internal/component/mtu/cmd/search.go` - `prober` interface (`probe(ctx, payload) (probeAnswer, error)`), `probeOutcome` (`probeReplied`, `probeRefusedReported`, `probeRefusedUnreported`, `probeSilent`), `probesPerSizeMax`, `pathSearch.confirm` and `refine`, `wireProber` (ICMP)
- [ ] `internal/component/mtu/cmd/run.go` - `measure` opens one `targetProber` per target through `mtuDeps.openProber` and runs `searchPathMTU`; `icmpCaveat` is printed on every run; `measurementRow` emits `method`; `tunnelTarget` reads `ipsecinventory.Tunnel`
- [ ] `internal/le/interoplab/ipsec/checkers.go`, `helpers.go` - `checkMTUTunnelSizingStrongSwan` clamps the NAT box with `clampForwardedPath` and reads `show mtu`; the scenario map keys on the directory name
- [ ] `internal/le/qemu/alltests.go` - `integrationPackages` lists `ike/engine`, `core/probe`, `mtu/cmd`; `ike/transport` is not listed

**Behavior to preserve:**
- DPD: `dpdState`, `sendDPD`, `retransmitDPD`, `handleDPDResponse` and the `matchesProbe` correlation are unchanged; `TestDPD*` and `rfc7296_dpd_test.go` stay green.
- The rekey path: `pendingRekey`, `serviceRekeyRetransmit`, the rekey correlation in `handleOwnedInbound`, and `ipsec-13`'s wire rules.
- The request window: one outstanding request, `requestWindowTimeout` and `StateDead` for a Delete or an INVALID_MESSAGE_ID holder; a probe holder takes the same exit, and `serviceRequestWindow` is not edited.
- `show mtu`: the grammar, every existing payload key, the ICMP prober for `host`, for the reference address, and for a tunnel that is down; `show-mtu-*.ci` stay green.
- The transport's plain `Send` for every existing caller: same signature, same DF behavior (the kernel's default).
- `ipsecinventory`'s registration and the `Tunnel` fields.

**Behavior to change:**
- The owner loop accepts a probe request on a channel and runs the padded INFORMATIONAL exchange for it.
- `serviceRequestRetransmit` repeats a probe holder's `sa.requestMsg` with DF clear; `serviceRequestWindow` is unchanged and fails the SA past the budget for a probe exactly as for a Delete.
- The transport gains a per-datagram DF send and an error-queue drain; the IKE and NAT-T sockets get `IP_RECVERR`/`IPV6_RECVERR` at creation.
- `show mtu` selects the IKE prober for a tunnel whose inventory `Up` is true, confirms the ICMP figure with it, probes by IKE only at or below that figure, stops at the first size whose probe stays unanswered, and prints `prober` on every measurement row; the IKE-measured row drops the ICMP optimism caveat, and a tunnel whose probe failed the SA keeps its ICMP figure with an `unconfirmed` note naming the size.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `show mtu` (the existing `ze:command`, `internal/component/mtu/cmd`): `runMTU` resolves peer targets from `ipsecinventory.Tunnels`; a target whose `Tunnel.Up` is true asks `ikeprobe.Probe(ctx, peer name, wire size, mode)`.
- Format at entry: a peer name, a requested wire size in octets (the whole datagram), and a `probe.DFMode` (`DFHonorCache` for a normal run, `DFBypassCache` under `exhaustive`).

### Transformation Path
1. `ikeprobe.Probe` (`internal/core/ikeprobe/registry.go`) hands the request to the registered engine function or answers `ErrNotRegistered`.
2. The engine's registered function (`internal/component/ike/engine/probe.go`) finds the `PeerSession` by name (`lookupPeerSession`, `register.go`) and sends a probe request, carrying a reply channel, on the session's request channel (`reconcile.go`). A session with no established SA, a pending rekey, a held window or a rekey hold answers a refusal by name without touching the loop.
3. `maintainSA` (`established.go`) receives the request on its select, beside `stopCh` and `supersede`. It computes the Notification Data length from the requested wire size: size minus IP header (20 or 40), UDP header (8), the four-octet marker when `sa.sendPath` answers the 4500 socket, the IKE header (28), the SK payload overhead of the negotiated suite (IV, Pad Length, ICV or tag, block rounding), and the Notify's fixed header (8 with SPI size 0). A size the subtraction cannot reach below the smallest Notify or above min(interface MTU, 3000) is refused by name.
4. The loop builds an INFORMATIONAL request through the existing SK builder (`auth.go`) carrying exactly one `PayloadNotify` of the private-use type with that many octets of opaque data, reserves the window (`reserveRequestWindow`), records the probe's message id and its reply channel on the session, sends through the transport's DF send with DF set (DO, or PROBE under `DFBypassCache`), arms `armRequestRetransmit`, and advances `sa.NextMsgID`.
5. Three things can happen next. A response with the probe's id reaches the `inboundInvalid` arm of `handleOwnedInbound` (`inbound.go`), decrypts, frees the window, and the loop matches the id against the probe (not against `dpdState`): answered before any DF-clear copy went out is `fits`; answered after one is `too-big`. A too-big read off the error queue for this SA's peer address and port makes the loop retransmit the identical IKE bytes at once with DF clear. Silence makes `serviceRequestRetransmit` repeat the message on the ordinary schedule (3 repeats, 500 ms doubling to a 60 s cap), every repeat with DF clear. A probe unanswered after that full budget, DF-clear copies included, is an unanswered request: `serviceRequestWindow` fails the SA (`StateDead`) exactly as for a Delete, the owner loop tears it down on the next tick as it does for a DPD timeout (`cleanupChild`, then `errSADeletedByPeer` so `PeerSession.run` reconnects), and the probe outcome is `sa-failed`. `silent` is not a terminal outcome at the engine level; the MTU module's `probeSilent` belongs to the ICMP prober alone.
6. The outcome travels back on the reply channel to `ikeprobe.Probe`, then to the MTU module, which turns it into a `probeAnswer` for `searchPathMTU`: `fits` is `probeReplied`, `too-big` is `probeRefusedUnreported` (the peer did not report a figure), `sa-failed` ends the IKE attempt for that tunnel at once (no further size is tried) and the tunnel keeps its ICMP figure with a note naming the outcome and the size, `refused` and `not registered` end the IKE attempt and select ICMP for that tunnel with a note saying why.
7. `measurementRow` emits `prober: ike` and no optimism caveat for a figure the IKE prober confirmed; `prober: icmp` and the existing caveat otherwise, and `unconfirmed` with the reason when the IKE attempt ended on `sa-failed`. Without NAT-T the `ike` row keeps a "different path than ESP" caveat (UDP/500 against protocol 50).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| MTU module ↔ core leaf `ikeprobe` | a Go call with value types (peer name, size, mode) and a typed outcome; no engine import in `mtu/cmd` | No |
| Core leaf `ikeprobe` ↔ IKE engine | `Register` at `init` (`register.go`), same shape as `ipsecinventory.Register` | No |
| Engine caller goroutine ↔ owner loop | a request struct on the session's request channel, a reply channel inside it; the loop is the only writer of SA state | No |
| Owner loop ↔ transport | a DF-mode send and an error-queue event channel on `UDPTransport`; the loop reads the event and matches it by peer address and port | No |
| Transport ↔ kernel | `setsockopt IP_MTU_DISCOVER` (or `IPV6_MTU_DISCOVER`/`IPV6_DONTFRAG`) around one `sendto` under the send lock; `IP_RECVERR`/`IPV6_RECVERR` at socket creation; `MSG_ERRQUEUE` read through the exported `internal/core/probe` drain | No |
| Ze ↔ peer | one INFORMATIONAL request carrying one status Notify, private-use type, answered with an empty INFORMATIONAL response | No |

### Integration Points
- `maintainSA` select (`established.go`) - gains the request channel case; the probe's state (message id, reply channel, DF-clear sent flag) lives on the `PeerSession` beside `pendingRekey`.
- `handleOwnedInbound` `inboundInvalid` arm (`inbound.go`) - already decrypts and frees the window; the returned message id is matched against the probe id in the loop, in the same place `dpd.matchesProbe` is consulted, as a second correlation that never credits DPD.
- `serviceRequestRetransmit` (`established.go`) - repeats `sa.requestMsg` with DF clear when the holder is a probe, on the ordinary schedule. `serviceRequestWindow` is unchanged: a probe past the budget is an unanswered request and it fails the SA (Section 2.1), and the loop's `StateDead` arm answers `sa-failed` on the probe's reply channel before it tears the SA down.
- `sendDPD` (`dpd.go`) - the model for reserve, send, arm, advance; the probe copies the order and shares none of the state.
- `SA.sendPath` (`sa.go`) - selects the socket and answers whether the marker is present; the size budget reads that answer.
- `UDPTransport` (`transport/udp.go`) - gains the DF send and the error-queue event stream; `Send` keeps its signature and takes the same lock around its write.
- `wire.PayloadNotify` (`payload_notify.go`) - the carrier; one new constant in the private-use range.
- `ipsecinventory.Tunnel.Up` - the prober selection predicate in `mtu/cmd`.
- `pathSearch.confirm` and `refine` (`search.go`) - unchanged algorithm; the IKE prober is another `prober`, with an exchange budget enforced by the prober itself.
- `measurementRow` (`run.go`) - the `prober` field and the caveat rule.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | `ikeProber.probe` (`mtu/cmd/search.go`) calls `ikeprobe.Probe`; the engine's `probePeer` (`engine/probe.go`) hands the request to `maintainSA` over `PeerSession.probeRequests`; `answerProbeRequest` builds through the SK builder and sends through `UDPTransport.SendDF`; the only socket writes are `UDPTransport.write` (`Send`, `SendDF`) |
| No unintended coupling (components stay isolated) | Yes | `mtu/cmd` imports `internal/core/ikeprobe` and `internal/core/probe` (grep of its imports: no `ike/engine`); `ikeprobe/registry.go` imports `context`, `errors`, `sync` and `internal/core/probe` only; `./le tier check` (phase 6): "core import direction clean" |
| No duplicated functionality (extends existing, does not recreate) | Yes | the DF-clear copy rides `serviceRequestRetransmit` (`established.go`); the window is `reserveRequestWindow`/`armRequestRetransmit` (`msgid.go`); the DF option mapping and the error-queue parse are `probe.WithDFMode`, `probe.EnableErrorQueue`, `probe.DrainErrorQueue` (`internal/core/probe`, exported for the transport); the descent is `pathSearch.refine`, the same search the ICMP path runs |
| Zero-copy preserved where applicable (refs, not copies) | Yes | the Notify writes through `PayloadNotify.WriteTo` into the SK builder's buffer (`answerProbeRequest`); the DF-clear copy is `sa.requestMsg` re-sent by `sendProbeDFClear`, which changes the IP header only (`TestProbeRetransmitIsBitwiseIdenticalWithDFClear`) |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | `engine/register.go` `init` calls `ikeprobe.Register(probePeer)` beside `ipsecinventory.Register`; `mtu/cmd` reaches it through `ikeprobe.Probe` and no switch names the engine; `TestIKEProbeRegisteredByEngine`, `TestIKEProbeUnregisteredIsNotSilence` (`ErrNotRegistered` selects ICMP with a note) |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes, with the central lists named | Lists that learned a name: `notifyTypeNames` (`wire/payload_notify.go`) gained `ZE_PATH_PROBE_PADDING` (65280), Ze's own private-use type, so the wire registry names it and `logIgnoredNotifies` (`notify_error.go`, called from `inbound.go`) derives from that map through `wire.NotifyTypeRecognized` and needed no edit; `ipsecScenarios` (`test/interop-ipsec/parity_test.go`) and the checker map in `interoplab/ipsec/checkers.go` gained `ike-padded-probe-strongswan`, both keyed by the scenario directory name, which is the lab's registry; `integrationPackages` (`internal/le/qemu/alltests.go`) gained `./internal/component/ike/transport`, the closed list `TestEveryIntegrationPackageIsNamed` derives the population from; `internal/test/fixture/register_show_mtu.go` gained `plugin/show-mtu-ike-probe`, the fixture registry the runner resolves names in. Lists searched and untouched: the `show mtu` YANG help and the pipe renderers (`prober`, `exchanges`, `ike-confirmed`, `ike-declined` are payload keys the renderers walk generically; `show-mtu-json.ci` proves `\| json`), the engine's `Refusal` names are typed constants in `ikeprobe/registry.go` and `mtu/cmd` switches on the type, not on a string |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Per-datagram DF by toggling `IP_MTU_DISCOVER` around one send on the shared socket, under the send lock, does not disturb a concurrent send and takes effect for that datagram | Linux applies `IP_MTU_DISCOVER` at send time per socket; `internal/core/probe/socket_linux.go` already maps the three modes; the transport's lock serializes the toggle and the write | the DF copy of another SA's message leaves with DF set, or the probe leaves without DF. Fallback: PROBE mode on the whole socket for the duration of a run | the Linux integration test in `ike/transport` observes, on an AF_PACKET capture, the DF copy then the non-DF copy while a concurrent plain `Send` keeps the kernel default | confirmed (2026-09-16, phase 2): `TestDFSendThenPlainCopyOnTheWire` read the DF copy (flags 0x40) then the identical 1200-octet UDP payload with DF clear off the router's `rs0`; `TestPlainSendNeverLeavesUnderTheProbeOption` raced 800 plain sends against 200 DF-off sends and every plain datagram carried DF (the kernel default for a fitting datagram), every DF-off one did not, all 1000 captured. Root run under `sudo -n` natively, `job-ike-df-int3` log; discriminated: with `WithDFMode` cut to install nothing, the second copy left with DF set (`job-ike-df-mut2`) |
| A-2 | strongSwan answers an INFORMATIONAL request carrying one unknown status Notify with an empty response and keeps the SA | read at the producer, `task_manager_v2.c` `process_request` (RESEARCH, 2026-09-16); RFC 7296 Section 3.10.1 | the feature is Ze-to-Ze only, and the owner decides whether it ships | `ike-padded-probe-strongswan` asserts the answer and that the SA survives every size | confirmed (2026-09-16, phase 5): `ike-padded-probe-strongswan` step 1, charon 5.9 parsed every padded INFORMATIONAL request (`parsed INFORMATIONAL request N [ N(...) ]`, `requireCharonParsedProbes`: no parse failure, no INVALID_SYNTAX) and answered each with an empty INFORMATIONAL; 16 exchanges from 1500 down to the 1400 clamp, the SA established on both daemons afterwards and a lossless DF ping at 1278 through it (`requirePaddedProbeSAAlive`). Green run `scratch/interop-p5b-run6.log`; the same step went RED with the DF copy cut (`scratch/interop-p5b-mut.log`) |
| A-3 | The NAT box forwards the DF-clear retransmission after fragmenting it (conntrack defragments and refragments) | Linux netfilter defragments before conntrack and refragments on output; the scenario's `nat.conf` box is that kernel | a too-big size is read as silence across a NAT; the search still converges, one exchange slower per size | the scenario observes the DF-clear copy answered at a size above the clamp | confirmed (2026-09-16, phase 5): the box's `/proc/net/snmp` read mid-run showed `FragOKs 16 FragCreates 32` beside 17 Fragmentation Needed dropped in OUTPUT (`iptables -v -S`): every DF copy above 1400 was refused in silence and every DF-clear copy left the box in two fragments; charon answered them (too-big read on the DF-clear copy, the descent then found 1400), so the row carries `exchanges 16`, `path-mtu 1400`, `ike-confirmed 1400`, `prober: ike` (`requireIKEMeasured` needs at least two exchanges, which only an answered DF-clear copy produces) |
| A-4 | The kernel queues the ICMP too-big for the DF copy on the IKE socket's error queue with the peer's address as the offender, and the drain identifies which SA it belongs to | `internal/core/probe/errqueue_linux.go` `offenderAddr` reads it for the ICMP socket; the IKE socket is a UDP socket on the same kernel | the DF-clear copy waits for the retransmit timer instead of leaving at once; correctness holds, latency grows | the integration test queues an EMSGSIZE for the peer's address and the drain reports it with that offender | confirmed, with one correction (2026-09-16, phase 2): the offender is the ROUTER that answered (10.99.1.2), never the peer; the peer is the entry's own destination, which the kernel names on the `MSG_ERRQUEUE` read and `probe.QueuedError.Dest` now carries, delivered as `SizeRefusal.Peer` (10.99.2.2:500). `TestOversizedDFSendQueuesRefusalForThePeer` read both off `Refusals()` with MTU 1400, then the cache refusal of the second send with `Local` and port 0 (the kernel fills a LOCAL entry from the socket's connected port). Discriminated: with `EnableErrorQueue` cut, nothing reached `Refusals` (`job-ike-df-mut2`). Found on the way: `IP_RECVERR` makes the kernel hand an ICMP error about an earlier datagram to the NEXT send as its failure (`sock_alloc_send_pskb`), so `Send` gained the drain-then-retry in `write` and `TestSendSurvivesAnErrorQueuedForAnEarlierDatagram` |
| A-5 | The `inboundInvalid` arm can carry a second correlation (the probe id) without changing DPD's | the arm returns `dpdRespMsgID` and the loop decides; `matchesProbe` compares against `dpdState.probeMsgID` only | a probe response credits DPD, or a DPD response answers a probe | `TestProbeNeverTouchesDPDState`, and every existing `TestDPD*` and `rfc7296_dpd_test.go` staying green | confirmed (2026-09-16, phase 3): the arm is unchanged; `settleProbe` (`probe.go`) matches `PeerSession.pendingProbe.msgID` and `dpd.matchesProbe` its own id, in sequence in `maintainSA`. `TestProbeNeverTouchesDPDState` replays the earlier DPD answer at the probe and compares every `dpdState` field across the exchange; every `TestDPD*`, `TestDpd*`, `TestWin*` and `TestRtx*` green in the same run (`job-probe-p3c`). Discriminated: with `settleProbe` matching any response id, the replayed DPD answer ended the probe (`job-probe-p3-mut`) |
| A-6 | The mitigation keeps a fragment-dropping path from killing a live SA in the common case, because the first silent size ends the IKE probing: the module probes only at or below the ICMP figure, and one size that stays unanswered on its DF-clear copies is the last IKE probe of the run | the ICMP figure is already known before the first IKE probe (`pathSearch.confirm`, `search.go`, confirm-first), so a size the path carries unfragmented is answered on the DF copy and never reaches the DF-clear copy; only a path that drops IP fragments AND clamps below the ICMP figure sends a DF-clear copy that is lost | a tunnel goes down during a diagnostic: the probe fails the SA the way any IKE request larger than the path would, and the owner loop reconnects it | the `ike-padded-probe-strongswan` scenario's fragment-drop step: the NAT box drops the fragments of the DF-clear copy, `show mtu` runs once, the SA fails exactly once, the payload row says `sa-failed` and names the size, and no second size is tried (AC-13) | confirmed, with one correction (2026-09-16, phase 5): the fragments are dropped at strongSwan's REASSEMBLY, not on the box. Conntrack defragments before any netfilter chain runs on both containers, so an `iptables -f` rule matched nothing and run 4 measured 1400 in 16 exchanges as if the fragments had crossed; the step instead cuts strongSwan's `net.ipv4.ipfrag_low_thresh` then `ipfrag_high_thresh` to 0 (`dropFragmentsAtPeer`, low first because high may not go below low), so no fragmented datagram is ever reassembled. Step 3 of `ike-padded-probe-strongswan` then read `prober: icmp`, `ike-declined: sa-failed at 1500 octets`, `exchanges 1`, `path-mtu 1500` kept, the SA failed exactly once and was re-established (`waitZeIKEEstablished`), and a lossless DF ping crossed afterwards (`scratch/interop-p5b-run6.log`; phase 6 rerun from the pristine tree: `integration: 1 action(s) passed.`) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A peer reads the padded INFORMATIONAL as malformed and tears the SA down, so a diagnostic takes a tunnel down | the peer logs a parse failure, or `show vpn ipsec sa` shows the SA gone, in the first scenario run | the carrier is a status Notify only (RFC 7296 Section 3.10.1); the scenario runs in phase 5 before the feature is offered by default; a peer that reacts by tearing down stops the spec at that phase |
| R-2 | The DF toggle races another SA's send on the shared socket | the integration test capture shows a plain-send datagram with DF set, or a probe without it | the toggle and the write run under the transport's lock and `Send` takes the same lock; fallback is PROBE mode on the whole socket for the run |
| R-3 | A probe holding the window delays DPD past its own timeout | `dpd.shouldSend` is true while the window is held; the peer's DPD toward Ze is answered normally, so the peer stays up | the exchange budget (16 per run) and confirm-first bound the hold; `dpdState.lastSent` is untouched, so the next DPD leaves at the first free tick |
| R-4 | The error-queue drain reads another SA's too-big and retransmits the wrong probe | a DF-clear copy leaves for a peer whose probe drew no ICMP | the event is matched by peer address and port against the SA that holds the probe; an unmatched event is logged and dropped |
| R-5 | strongSwan in IKE_REKEYED drops the request (`reject_request`), so a probe racing a rekey loses its first copy | the DF copy draws no answer at a size that fitted a moment earlier | the ordinary retransmit schedule keeps the SA: the DF-clear copy 500 ms later is answered once the rekey completes, and that reads as `too-big`. One `too-big` moves no search bound (RFC 4821 Section 7.6.4): the module tries the same size once more, the retry is answered on the DF copy, and the figure stands (AC-3); the scenario forces the race once (AC-10) |
| R-6 | `exhaustive` in PROBE mode fails locally with EMSGSIZE when the interface MTU is smaller than the requested size | the send returns EMSGSIZE before anything leaves | a local refusal is a named outcome (`refused`, reason `local`) and the ceiling min(interface MTU, 3000) keeps it from being reached on a correct run |
| R-7 | A probe on a path that drops IP fragments fails the SA: the DF copy is too big, the DF-clear copies are fragmented and dropped, and after the full budget the SA is deemed failed the way any IKE request larger than the path would fail it | the `ike-padded-probe-strongswan` scenario's fragment-drop step: the SA fails once, the payload row says `sa-failed` and names the size | confirm-first (the module probes by IKE only at or below the ICMP figure), stop at the first silent size (`ikeProbeStopAtFirstSilence`, `search.go`), and the caveat stated plainly in the payload note and on `docs/architecture/diagnostics/path-mtu.md`. The deviation (a probe-aware `serviceRequestWindow` that releases the window and rewinds the id) was rejected. Owner ruling (a), 2026-09-16 |
| R-8 | A future RFC 7383 implementation fragments a probe and the figure becomes the fragment threshold | a probe leaves as more than one IKE message | AC-6 and the constraint recorded in `plan/immediate/spec-ike-fragmentation-rfc7383.md` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A live IKE SA. This sends traffic on a production tunnel's control channel, which is a sharper blast radius than the ICMP prober has: a stranded message id, a DPD credited by a probe response, or a window never released each cost the tunnel |
| How is it reverted? | Single commit revert; the MTU module falls back to the ICMP prober it already has |
| Who else touches this path? | `internal/component/mtu/cmd` (`docs/architecture/diagnostics/path-mtu.md`) is the consumer, `plan/immediate/spec-rfc4301-architecture-gaps.md` touches the same engine, `plan/immediate/spec-ike-fragmentation-rfc7383.md` will reuse the DF send and the error-queue drain |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show mtu` with a live SA to the peer | → | the padded INFORMATIONAL prober through `ikeprobe.Probe` | `TestShowMTUUsesIKEProbeWhenSAIsUp` |
| `show mtu` with the tunnel down | → | the ICMP prober, unchanged | `TestShowMTUFallsBackToICMPWhenSAIsDown` |
| the IKE component's `init` | → | `ikeprobe.Register` | `TestIKEProbeRegisteredByEngine` |
| a probe request on the session channel | → | `maintainSA` builds and sends the padded INFORMATIONAL | `TestPaddedInformationalReachesTheOwnerLoop` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A live IKE SA, a path clamped below the interface MTU, ICMP errors filtered on the path | the measured figure equals the clamp and the measurement row says `prober: ike` |
| AC-2 | A peer that refuses the padded exchange by name (`refused`, `not registered`), or a peer that answers no copy of the first size | the run keeps the ICMP figure for that tunnel, the row says `prober: icmp`, and a note says why the IKE prober was not used (the refusal by name, or `sa-failed` and the size) |
| AC-3 | A padded probe whose DF copy is lost once (a drop, or strongSwan's IKE_REKEYED race) | the ordinary retransmit schedule carries the DF-clear copy and the SA survives; the answer reads as `too-big`, and one `too-big` moves no search bound (RFC 4821 Section 7.6.4): the same size is tried once more before it is declared too big, within the exchange budget of 16 |
| AC-4 | A probe whose DF copy is black-holed but whose DF-clear copy is answered | the SA never fails, `dpdState` is untouched, the size reads as `too-big`, and the next DPD after the probe is answered without INVALID_MESSAGE_ID |
| AC-5 | Any probe, observed with an AF_PACKET capture rather than from Ze's own report | the datagram on the wire is exactly the requested size, the first copy carries DF, the retransmission carries no DF, and the IKE bytes of both copies are identical |
| AC-6 | Any run, any interface MTU | no probe exceeds min(interface MTU, 3000) octets and no probe is IKE-fragmented |
| AC-7 | The SA runs on 4500 (NAT-T) | the probe leaves from 4500 with the four-octet non-ESP marker and the row carries no path caveat. Without NAT-T the row carries the "different path than ESP" caveat |
| AC-8 | A probe requested while a rekey is pending, a rekey hold is in force, the window is held, or the SA is not established | the request is refused by name at once, never queued behind the condition |
| AC-9 | A build without the IKE component | `ikeprobe.Probe` answers `ErrNotRegistered` and `show mtu` uses ICMP for every target, as today |
| AC-10 | strongSwan in IKE_REKEYED drops the request (`reject_request`) | the run reads it as silence and the retry at the same size absorbs it; the figure is unchanged and the SA survives |
| AC-11 | An ICMP too-big for the DF copy arrives on the IKE socket's error queue | the DF-clear retransmission leaves at once rather than at the next retransmit timer, and the event is matched to the probe's peer (the socket is shared by every SA) |
| AC-12 | `show mtu exhaustive` | the DF copy is sent in PROBE mode so a poisoned kernel path cache is bypassed |
| AC-13 | A probe unanswered after the full retransmit budget, DF-clear copies included (a path that drops IP fragments) | the SA fails the way an unanswered DPD does (`StateDead`, `cleanupChild`, reconnect through `PeerSession.run`), the payload row says `sa-failed` and names the size, no second size is tried, and the run reports that tunnel's figure as ICMP-measured and unconfirmed |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs `show mtu` on a CPE whose peer path filters ICMP but carries ESP | CLI → inventory → `ikeprobe.Probe` → owner loop → padded INFORMATIONAL → measured figure, `prober: ike`, no optimism caveat | `show-mtu-ike-probe` |
| 2 | runs `show mtu` while the peer's IKE daemon does not answer the padded exchange | CLI → inventory → `ikeprobe.Probe` → the first size unanswered through the budget → `sa-failed` → the ICMP figure kept for that tunnel, `prober: icmp`, and a note naming `sa-failed` and the size | `TestShowMTUFallsBackToICMPWhenSAIsDown`, `TestIKEProbeSAFailedEndsTheRun`, `ike-padded-probe-strongswan` |
| 3 | runs `show mtu` across a NAT | CLI → inventory (`UDPEncap`) → probe on 4500 with the marker → figure for the path ESP-in-UDP rides | `ike-padded-probe-strongswan` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPaddedInformationalReachesRequestedSize` | `internal/component/ike/engine/probe_test.go` | the emitted datagram is exactly the size asked for, on both sockets, the marker counted on 4500 and not on 500 | green (phase 3, `job-probe-p3c`: `ok internal/component/ike/engine 26.3s`, 45 PASS) |
| `TestProbeCarriesOneStatusNotifyOnly` | `internal/component/ike/engine/probe_test.go` | the decrypted request holds one payload, a Notify of the private-use type with SPI size 0, and nothing else | green (phase 3, `job-probe-p3c`) |
| `TestProbeNeverTouchesDPDState` | `internal/component/ike/engine/probe_test.go` | a probe response with the probe's id changes no `dpdState` field and credits no liveness; a DPD response never answers a probe | green (phase 3, `job-probe-p3c`); RED under the cut that matched any response id (`job-probe-p3-mut`) |
| `TestProbeUnansweredFailsTheSALikeDPD` | `internal/component/ike/engine/probe_test.go` | a probe holder unanswered after every retransmit and `requestWindowTimeout` takes the teardown path DPD takes: `StateDead`, `cleanupChild`, `errSADeletedByPeer` from `maintainSA`; the reply channel carries `sa-failed`; no window is released without a response and `sa.NextMsgID` is not rewound | green (phase 3, `job-probe-p3c`) |
| `TestProbeAnsweredOnDFClearCopyDoesNotFailTheSA` | `internal/component/ike/engine/probe_test.go` | a probe whose DF copy draws nothing and whose DF-clear copy is answered frees the window, answers `too-big`, leaves the SA established and `dpdState` untouched, and the next DPD is answered | green (phase 3, `job-probe-p3c`) |
| `TestProbeRetransmitIsBitwiseIdenticalWithDFClear` | `internal/component/ike/engine/probe_test.go` | the second copy is `sa.requestMsg` byte for byte and leaves through the DF send with the clear mode | green (phase 3, `job-probe-p3c`) |
| `TestProbeSizeCeiling` | `internal/component/ike/engine/probe_test.go` | a size above min(interface MTU, 3000) is refused by name before anything is built | green (phase 3, `job-probe-p3c`) |
| `TestZeResponderIgnoresThePrivateStatusNotify` | `internal/component/ike/engine/probe_test.go` | `handleInformationalOwned` answers a request carrying only the private-use Notify with an empty response; the type is in `notifyTypeNames`, so `logIgnoredNotifies` writes no unknown-type line for it | green (phase 3, `job-probe-p3c`: `--- PASS: TestZeResponderIgnoresThePrivateStatusNotify`) |
| `TestProbeAdvancesMsgIDLikeDPD` | `internal/component/ike/engine/probe_test.go` | `sa.NextMsgID` advances by one per probe, in the order `sendDPD` uses (reserve, send, arm, advance) | green (phase 3, `job-probe-p3c`) |
| `TestProbeRefusedWhilstRekeyPending` | `internal/component/ike/engine/probe_test.go` | a pending rekey, a rekey hold, a held window, and an SA below `StateEstablished` each answer a distinct named refusal | green (phase 3, `job-probe-p3c`; phase 5, `job-p5b-engine-rekeyed` with `TestProbeRetiredByPeerRekeyIsAnsweredRekeyed`, the `rekeyed` refusal) |
| `TestProbeOutcomeZeroIsUnspecified` | `internal/core/ikeprobe/registry_test.go` (the leaf owns the type; the engine's completed outcomes are asserted by name in every `probe_test.go` case) | the zero `Outcome` prints as unspecified and is never returned by a completed probe | green (phase 5, `job-p5b-mtu`: `ok internal/core/ikeprobe 1.0s`) |
| `TestErrQueueTooBigTriggersDFClearAtOnce` | `internal/component/ike/engine/probe_test.go` | a too-big event for the probe's peer sends the DF-clear copy before the retransmit timer; one for another peer does not | green (phase 3, `job-probe-p3c`: `--- PASS: TestErrQueueTooBigTriggersDFClearAtOnce (10.04s)`) |
| `TestIKEProbeConfirmsThenDescends` | `internal/component/mtu/cmd/run_test.go` | the IKE prober confirms the ICMP figure (size passes, size+1 fails) and descends the ladder only when confirmation fails; the exchange budget of 16 ends the attempt | green (phase 4; phase 5, `job-p5b-mtu`: `ok internal/component/mtu/cmd 1.3s`, `ike-confirmed` 1300 asserted) |
| `TestIKEProbeUnregisteredIsNotSilence` | `internal/component/mtu/cmd/run_test.go` | `ErrNotRegistered` selects ICMP with a note; it is never counted as a silent probe | green (phase 4; phase 5, `job-p5b-mtu`) |
| `TestIKEProbeSAFailedEndsTheRun` | `internal/component/mtu/cmd/run_test.go` | an `sa-failed` outcome ends the IKE attempt for that tunnel at the first size (`ikeProbeStopAtFirstSilence`), no further size is asked, the row keeps the ICMP figure with `prober: icmp` and an `unconfirmed` note naming `sa-failed` and the size; the IKE prober is never asked above the ICMP figure | green (phase 4; phase 5, `job-p5b-mtu`) |
| `TestMeasurementRowNamesTheProber` | `internal/component/mtu/cmd/run_test.go` (beside the other run tests, not `mtu_test.go`) | every measurement row carries `prober`, the IKE row carries no optimism caveat, the non-NAT IKE row carries the different-path caveat | green (phase 4; phase 5, `job-p5b-mtu`) |
| `TestIKEProbeRegisterTwicePanics` | `internal/core/ikeprobe/registry_test.go` | the leaf refuses a nil and a second registration, as `ipsecinventory` does | green (phase 1; phase 5, `job-p5b-mtu`: `ok internal/core/ikeprobe 1.0s`) |
| `TestDFSendRestoresSocketMode` | `internal/component/ike/transport/udp_linux_test.go` (Linux build tag, beside `TestSendSurvivesAnErrorQueuedForAnEarlierDatagram`) | after a DF send the socket's `IP_MTU_DISCOVER` reads its prior value (Linux build tag) | green (phase 2: `ok internal/component/ike/transport 0.1s`; the three AF_PACKET tests in `udp_df_integration_linux_test.go` green under `sudo -n`, `job-ike-df-int3`, 3 PASS) |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| requested wire size, IPv4 | 68-3000 (RFC 791 minimum to `MaxMsgSize`) | 3000 | 67 | 3001 |
| requested wire size, IPv6 | 1280-3000 | 3000 | 1279 | 3001 |
| requested wire size against the interface | 68-interface MTU | interface MTU | N/A | interface MTU + 1 |
| exchanges per run | 1-16 | 16 | 0 | 17 |
| probes per size | 1-3 | 3 | 0 | 4 |
| private-use Notify type | 40960-65535 | 65535 | 40959 | N/A (65535 is the field's maximum) |
| Notification Data length | 0-2957 (3000 minus the smallest IPv4 datagram overhead) | fills the size exactly | N/A | a length the size cannot hold is refused by name |

Boundary confirmation (phase 6, read at the tests): `TestProbeSizeCeiling` (`engine/probe_test.go`) sends at `probeWireCeiling` (3000) and refuses `size` at 3001, refuses one below the smallest datagram the AEAD suite produces and sends at that smallest size, each refusal spending no message id and holding no window; `TestProbeRoundsDownToTheCipherGrid` covers the CBC grid between two reachable sizes; `TestIKEProbeNeverExceedsTheCeiling` (`mtu/cmd/run_test.go`) never offers an ICMP figure above `ikeWireMax`; `TestIKEExchangeBudgetBoundary` asks exactly `ikeExchangesPerRunMax` (16) times and the 17th answers `errIKEExchangeBudget` before the leaf is asked; the probes-per-size bound (3) is `TestIKEProbeRetriesTooBigBeforeBelievingIt` (the third `too-big` is believed, never the second); the private-use type is one constant (65280) inside the 40960-65535 range, asserted by `TestProbeCarriesOneStatusNotifyOnly`; IPv6 is refused `family` before any size arithmetic (the transport is `udp4`), so the IPv6 row has no reachable boundary in this build. The phase 6 GAP (no test asserted the `family` refusal) is closed: `TestProbeRefusesANonIPv4Peer` (`engine/probe_test.go`) drives `answerProbeRequest` with an authenticated IPv6 `peerEndpoint` and asserts `RefusalFamily` by name, no message id spent, no window held, nothing on the wire, then the same SA over IPv4 sends; red under a `MUTATION-APPLIED` cut of the `To4()` check (answered `send-failed`, restored), green after.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-mtu-ike-probe` | `test/plugin/show-mtu-ike-probe.ci` | Ze-to-Ze over the compiled fixture `plugin/clamped-path` (`internal/test/fixture`): the far responders run no dataplane, so an echo to a tunnel peer never returns and the ICMP search reads the 1400 clamp from the router's Fragmentation Needed alone; the padded exchange (aes256-cbc, sent at the 1388 grid point) confirms it, the row says `prober: ike`, keeps `path-mtu` 1400 and carries `ike-confirmed` 1388, and both SAs stay established; `option=needs-linux:caps=net-admin,net-raw`. Filtering the router's ICMP errors here leaves the ICMP search unmeasurable and the IKE prober never runs (confirm-first); the filtered-ICMP story is the interop scenario's (2026-09-16) | green, discriminated (phase 5: RED with `answerProbeRequest` cut to refuse, "prober icmp, want ike", `job-p5-ike-mut`); phase 6 against the pair rebuilt with the `rekeyed` fix, `job-p6-mtu-six` under `sudo -n`: `PASS show-mtu-ike-probe` (4.7s) and the five siblings `show-mtu-exhaustive`, `show-mtu-host`, `show-mtu-json`, `show-mtu-no-ipsec-component`, `show-mtu-oversized-tunnels` all PASS, 6/6 |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ike-padded-probe-strongswan` | `test/interop-ipsec/scenarios/` | strongSwan | a third-party responder answers the padded INFORMATIONAL at every size (A-2, R-1), the SA survives, the figure equals the NAT box clamp copied from `mtu-tunnel-sizing-strongswan` (`nat.conf`, `clampForwardedPath`), the DF-clear copy crosses the NAT (A-3), and one forced IKE_REKEYED race is absorbed (AC-10). A second step then makes the NAT box drop the fragments of the DF-clear copy and runs `show mtu` once: the SA fails exactly once, the payload row says `sa-failed` and names the size, no second size is tried, and the SA is re-established afterwards (A-6, R-7, AC-13). Discrimination per `ai/rules/interop-and-goal-validation.md`: the Notify carrier is reverted, the image rebuilt, the scenario goes red | green, discriminated (phase 5): three steps in `checkIKEPaddedProbeStrongSwan` (`interoplab/ipsec/checkers.go`). The box keeps the 1400 clamp for UDP and ESP, exempts ICMP from it (`exemptICMPFromClamp`) and drops every Fragmentation Needed toward Ze (`dropTooBigToward`), so the ICMP search is misled to 1500 and IKE refutes it: step 1 `prober: ike`, `path-mtu 1400`, `ike-confirmed 1400`, `exchanges 16`, NAT-T SA on 4500 with `caveats: []` (AC-1, AC-7, A-2, A-3); step 2 races `swanctl --rekey` and the probe survives as the IKE figure or a refusal by name (AC-10, after the `rekeyed` fix); step 3 cuts strongSwan's reassembly marks (`dropFragmentsAtPeer`), `sa-failed at 1500 octets`, `exchanges 1`, the SA re-established (AC-13, A-6). GREEN `scratch/interop-p5b-run6.log`; RED with the DF copy sent `probe.DFOff` (`scratch/interop-p5b-mut.log`: "path-mtu is 1500, want 1400 ... exchanges:1 ike-confirmed:1500"); restored, `MUTATION-APPLIED` grep 0. Phase 6 rerun from the pristine tree with the image rebuilt (`ze-ipsec-interop:latest` 19 minutes old at the read): `integration: 1 action(s) passed.` |

## Files to Modify
- `internal/component/ike/engine/established.go` - the request channel case in `maintainSA`; the DF-clear repeat in `serviceRequestRetransmit`; the probe correlation beside `dpd.matchesProbe`; the `sa-failed` answer on the probe's reply channel in the `StateDead` arm. `serviceRequestWindow` is not changed: no probe-aware branch, no release without a response, no message id rewound
- `internal/component/ike/engine/inbound.go` - the `inboundInvalid` arm's comment names the probe as a second correlated requester; no behavior change in the arm
- `internal/component/ike/engine/reconcile.go` - the request channel and the probe state on `PeerSession`
- `internal/component/ike/engine/register.go` - `ikeprobe.Register` at `init`, beside `ipsecinventory.Register`
- `internal/component/ike/wire/payload_notify.go` - the private-use status type constant and its name in `notifyTypeNames`
- `internal/component/ike/transport/udp.go` - the DF-mode send, the lock around every write, the error-queue event stream, `IP_RECVERR` at creation through the platform split
- `internal/core/probe/errqueue_linux.go`, `errqueue_other.go`, `socket_linux.go` - export the drain and the DF option setter for a foreign socket, so the transport reuses them rather than copying them
- `internal/component/mtu/cmd/search.go` - `confirm` takes the prober's outcomes; the exchange budget constant; the `ikeProbeStopAtFirstSilence` constant and the rule that no IKE probe is asked above the ICMP figure
- `internal/component/mtu/cmd/run.go` - prober selection on `Tunnel.Up`, the IKE prober adapter, the `prober` field, the caveat rules, the fallback note
- `internal/component/mtu/cmd/mtu_test.go`, `run_test.go` - the tests named above
- `internal/le/qemu/alltests.go` - `./internal/component/ike/transport` joins `integrationPackages`
- `internal/le/interoplab/ipsec/checkers.go`, `helpers.go` - the scenario's checker and the forced-rekey helper
- `test/interop-ipsec/parity_test.go` - the scenario name
- `rfc/short/rfc7296.md` - sender rows for Section 3.10.1 and Section 3.14; Support stays Partial
- `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, `ipsec-7-ikev2-engine.md`, `ipsec-13-rekey-wire.md`, `ipsec-9-ikev2-eap-nat.md`, `docs/architecture/diagnostics/path-mtu.md`, `docs/architecture/diagnostics/active-probes.md`, `docs/architecture/testing/qemu-integration.md`, `docs/architecture/core-design.md`, `docs/architecture/api/commands.md`, `docs/guide/command-reference.md`, `docs/features.md`, `docs/comparison.md`, `docs/functional-tests.md`, `docs/architecture/testing/interop.md` - per the Documentation Update Checklist; `docs/architecture/wire/buffer-writer.md` is declared by `payload_notify.go` and unaffected

## Files to Create
- `internal/core/ikeprobe/registry.go` - `Register`, `Probe`, `Outcome`, `ErrNotRegistered`; `registry_test.go`
- `internal/component/ike/engine/probe.go` - the request struct, the size budget, the build, the correlation, the outcomes; `probe_test.go`
- `internal/component/ike/transport/udp_linux.go` - the DF send and the error-queue drain; `udp_other.go` - the stub answering `probe.ErrDFUnsupported` and `probe.ErrErrQueueUnsupported`; `udp_df_integration_linux_test.go` - the two-namespace veth capture (`integration && linux`)
- `test/plugin/show-mtu-ike-probe.ci` - the functional test
- `test/interop-ipsec/scenarios/ike-padded-probe-strongswan/` - `nat.conf`, `swanctl.conf`, `ze.conf`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no new command; `show mtu` already exists and this changes only which prober it uses |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | no new leaf |
| CLI commands/flags | N-A | no new command |
| CLI grammar (keyword before value) | N-A | no new command |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | Yes | `test/plugin/show-mtu-ike-probe.ci`, PASS 6/6 in `job-p6-mtu-six` (phase 6) |
| Pipe completeness | Yes | the payload gains `prober`, `exchanges`, `ike-confirmed`, `ike-declined` as keys of the measurement row (`measurementRow`, `run.go`); `show-mtu-json.ci` renders `\| json` and PASSED beside the new `.ci`; the fixture `showMTUIKEProbeRow` reads the keys out of the JSON document (`ai/rules/cli.md`) |
| Env var registration | N-A | no leaf under `environment/` |
| Doctor check for runtime dependencies | N-A | no new runtime dependency: the IKE socket already exists and already has its checks (`engine/doctor.go`); `IP_RECVERR` is a socket option on that socket, not a new file, port or module |
| Prometheus counters/metrics | N-A | operator-invoked, no continuous state |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`: `show mtu` measures a live tunnel over IKE |
| 2 | Config syntax changed? | N-A | no config leaf |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`: the `prober` field and the caveat rules |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`: the payload names the prober |
| 5 | Plugin added/changed? | N-A | no plugin |
| 6 | Has a user guide page? | Yes | `docs/architecture/diagnostics/path-mtu.md`: the IKE prober, the `prober` field, the caveat rules, the exchange budget, confirm-first and stop-at-first-silence, and the plain statement that on a path that drops IP fragments a probe can take the tunnel down the way any IKE request larger than the path would |
| 7 | Wire format changed? | Yes | `docs/architecture/ike/ipsec-7-ikev2-engine.md`: the padded exchange's shape (the Notify, the size budget, the two copies). `docs/architecture/wire/` holds BGP, IS-IS, OSPF, L2TP and the `buffer-writer.md` codec page and no IKE page, and `ai/CODE-TO-DOCS.md` maps `ike/wire/` to `docs/features.md` only, so the IKE engine page is the right home |
| 8 | Plugin SDK/protocol changed? | N-A | no SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc7296.md`: sender rows for Section 3.10.1 (a status Notify Ze sends) and Section 3.14 (why SK padding is not the carrier); Support stays Partial |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` and `docs/architecture/testing/interop.md`: the new scenario and the transport integration test |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`: no other daemon measures the tunnel path from the IKE daemon |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` (the INFORMATIONAL, DPD, window, retransmission and probe section this spec writes: the page `established.go`, `inbound.go` and `dpd.go` declare), `ipsec-7-ikev2-engine.md` (the wire shape and a pointer), `ipsec-13-rekey-wire.md` (the stale sentence), `ipsec-9-ikev2-eap-nat.md` (which port post-establishment messages take), `docs/architecture/core-design.md` (the `ikeprobe` leaf), `docs/architecture/diagnostics/active-probes.md` (the exported drain and DF setter), `docs/architecture/testing/qemu-integration.md` (`ike/transport` in the package table) |
| 13 | Route metadata keys added/changed? | N-A | no route metadata |
| 14 | Prometheus counters added/changed? | N-A | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/architecture/core-design.md`: the `ikeprobe` registry beside `ipsecinventory`; `docs/guide/status.md` is unaffected because no doctor check is added (the IKE socket already carries its checks) |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/spec-ike-padded-path-probe.md` at implementation. Declared today by the files this spec names: `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` (`established.go`, `inbound.go`, `dpd.go`: edited, row 12), `docs/architecture/diagnostics/active-probes.md` (`internal/core/probe`: edited, row 12), `docs/architecture/testing/qemu-integration.md` (`alltests.go`: edited, row 12), `docs/architecture/wire/buffer-writer.md` (`payload_notify.go`: unaffected, a new constant changes no encoding rule) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/command-reference.md` holds the one `show mtu` JSON example (the oversized-tunnels run): its peer rows now read `prober: ike`, `ike-confirmed: 1388` (phase 6, the oversized SAs are aes256-cbc so the run IS IKE-confirmed). `docs/guide/ipsec.md` names `show mtu` in one sentence and holds no example; `docs/architecture/diagnostics/path-mtu.md` holds the grammar block only, and its payload table carries the keys (row 6) |

Evidence per row (phase 6, each page read against the code): 1 `docs/features.md` row "Path MTU diagnostic for IPsec tunnels" gained the IKE sentence and the prober names. 3 `docs/guide/command-reference.md`: the `prober`/`ike-confirmed`/`ike-declined` paragraph (phases 4-5) and the example (phase 6). 4 `docs/architecture/api/commands.md`: the `ze-show:mtu` row lists `prober`, `exchanges`, `ike-confirmed`, `ike-declined`. 6 `docs/architecture/diagnostics/path-mtu.md`: "The IKE prober" section (ceiling, confirm-first, the size sent, a rekey in flight, the descent, stop at the first silence, the caveat rules), anchored to `search.go` and `run.go`. 7 `docs/architecture/ike/ipsec-7-ikev2-engine.md`: "The padded path probe on the wire", the octet table and the grid rule, anchored to `probeNotifyOctets`. 9 `rfc/short/rfc7296.md`: the 65280 `ZE_PATH_PROBE_PADDING` row and the `[RFC7296-3.10.1-4] [MAY]` row; `./le rfc index-update` then `./le rfc check` (phase 6): 16 violations, none naming rfc7296, ike or ikeprobe (config vet failure, rfc3101 OSPF and rfc4301 `parseSPDPolicy` records, other sessions' surfaces). 10 `docs/functional-tests.md`: the six `show mtu` tests, `show-mtu-ike-probe.ci` and why it cannot filter ICMP; `docs/architecture/testing/interop.md`: the scenario paragraph with the ICMP exemption, the detached run and the reassembly-threshold cut as lab techniques, anchors naming `checkIKEPaddedProbeStrongSwan` and the four helpers. 11 `docs/comparison.md`: new row "Tunnel path measured over the live IKE SA" under Operations. 12 `ipsec-8-ikev2-child-xfrm.md`: "INFORMATIONAL requests, the window and retransmission" and "The padded path probe" with the eight-row refusal table (`rekeyed` included) and the ruling; `ipsec-13-rekey-wire.md`: the stale "dropped as out of window" sentence replaced by the `inboundInvalid` truth; `ipsec-9-ikev2-eap-nat.md`: "The two IKE sockets", `Send`/`SendDF`, the error queue and the `IP_RECVERR` side effect; `docs/architecture/core-design.md`: the `ikeprobe` leaf paragraph and the ike and mtu boundary rows; `docs/architecture/diagnostics/active-probes.md`: `Dest` and "A foreign socket"; `docs/architecture/testing/qemu-integration.md` needs no edit: it carries no package table, the population is derived from `integrationPackages` by `TestEveryIntegrationPackageIsNamed`. 15 `core-design.md` as in 12. 16 `./le spec citation anchors spec plan/spec-ike-padded-path-probe.md` (phase 6) names 7 documents mentioning this spec's code and not named here: `docs/DESIGN.md` (`register.go` "ike plugin"), `docs/architecture/api/process-protocol.md` (`OnAllPluginsReady`), `ipsec-10-cli-diag.md` (`ActiveTable`, `PeerInfo`), `ipsec-11-interop-eap.md` (the EAP checkers), `ipsec-14-responder.md` (`tryResponderSAInit`, `setPendingIKESwap`, the `runEstablished` teardown defer, `TerminateAllSAs`), `ipsec-3-data-model.md` and `ipsec-dataplane-inspection.md` (`PeerInfo`, `Info`, `setChildSA`); each anchor names a symbol this spec did not change, and the claims beside them were read and stand, so none is repaired. `./le doc check verify`: 2 CLAIM findings, both on pages this spec never touched (`docs/architecture/api/text-format.md`, `docs/features/formatting.md`); `./le docs-to-code index-check` reports the same two; `./le doc check links`: 8 broken references, none in this spec's files.

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the leaf, the channel, the stub prober, the page repair
   - Tests: `TestIKEProbeRegisteredByEngine`, `TestPaddedInformationalReachesTheOwnerLoop`, `TestShowMTUUsesIKEProbeWhenSAIsUp`, `TestShowMTUFallsBackToICMPWhenSAIsDown`, `TestIKEProbeRegisterTwicePanics`, `TestProbeOutcomeZeroIsUnspecified`
   - Files: `internal/core/ikeprobe/registry.go`, `engine/register.go`, `engine/reconcile.go` (the channel), `engine/probe.go` (a stub that answers `refused`), `mtu/cmd/run.go` (selection on `Tunnel.Up`), `docs/architecture/ike/ipsec-13-rekey-wire.md` (the stale sentence)
   - Verify: the engine registers, `show mtu` asks the leaf for an `Up` tunnel and falls back to ICMP on the stub's refusal, and the wiring tests fail because the loop builds nothing yet
2. **Phase: the transport** -- the DF send and the error queue
   - Tests: `TestDFSendRestoresSocketMode`, the integration test in `udp_df_integration_linux_test.go` (A-1, A-4)
   - Files: `transport/udp.go`, `udp_linux.go`, `udp_other.go`, `internal/core/probe` exports, `internal/le/qemu/alltests.go`
   - Verify: on the veth capture the DF copy then the non-DF copy of one payload; an EMSGSIZE queued for the peer's address reaches the event stream with that offender; every existing transport test green
3. **Phase: the exchange** -- build, send, retransmit, correlate; the unanswered probe fails the SA through the unchanged `serviceRequestWindow`
   - Tests: `TestPaddedInformationalReachesRequestedSize`, `TestProbeCarriesOneStatusNotifyOnly`, `TestProbeNeverTouchesDPDState`, `TestProbeUnansweredFailsTheSALikeDPD`, `TestProbeAnsweredOnDFClearCopyDoesNotFailTheSA`, `TestProbeRetransmitIsBitwiseIdenticalWithDFClear`, `TestProbeSizeCeiling`, `TestZeResponderIgnoresThePrivateStatusNotify`, `TestProbeAdvancesMsgIDLikeDPD`, `TestProbeRefusedWhilstRekeyPending`, `TestErrQueueTooBigTriggersDFClearAtOnce`, the boundary rows
   - Files: `engine/probe.go`, `established.go`, `inbound.go`, `wire/payload_notify.go`
   - Verify: every `TestDPD*`, `rfc7296_dpd_test.go`, `rfc7296_retransmit_test.go`, `rfc7296_window_test.go` and `rfc7296_msgid_test.go` still green; the wiring tests progress to a real exchange
4. **Phase: the MTU side** -- prober selection, confirm-first, the budget, stop at first silence, the payload
   - Tests: `TestIKEProbeConfirmsThenDescends`, `TestIKEProbeUnregisteredIsNotSilence`, `TestIKEProbeSAFailedEndsTheRun`, `TestMeasurementRowNamesTheProber`, the exchange and probes-per-size boundary rows
   - Files: `mtu/cmd/search.go` (the named constant `ikeProbeStopAtFirstSilence`, true, read by the IKE prober adapter and never by the ICMP path: an `sa-failed` outcome ends the IKE attempt at that size, and the comment above it carries the fragment-dropping-path caveat and the owner ruling), `run.go`, `mtu_test.go`, `run_test.go`
   - Verify: `show-mtu-*.ci` green unchanged; the `prober` key on every row; no IKE probe above the ICMP figure; an `sa-failed` outcome asks no second size
5. **Phase: interop and functional** -- the scenario and the `.ci`
   - Tests: `ike-padded-probe-strongswan`, `show-mtu-ike-probe`
   - Files: the scenario directory, `checkers.go`, `helpers.go`, `parity_test.go`, `test/plugin/show-mtu-ike-probe.ci`
   - Verify: A-2, A-3, A-6 driven to confirmed or broken (A-6 by the fragment-drop step: the SA fails exactly once and the payload names why); the discrimination walk recorded (revert the carrier, rebuild, red, restore, green)
6. **Phase: pages and ledger**
   - Tests: `./le doc check verify`, `./le spec citation anchors spec plan/spec-ike-padded-path-probe.md`
   - Files: every page in the Documentation Update Checklist, `rfc/short/rfc7296.md`
   - Verify: no stale anchor; row 16 answered from the command's output

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | A probe answered on either copy never drives IKE state; an unanswered probe never feeds `dpdState` and fails the SA only through the ordinary request timeout (`serviceRequestWindow`, unchanged) |
| Naming | The payload names which prober produced each figure, in the same key on every row |
| Data flow | The MTU module asks for a measurement and never constructs an IKE message itself |
| Rule: `ai/rules/rfc-compliance.md` | The padding mechanism is quoted from RFC 7296 above the code that builds it |
| Retransmission identity | The DF-clear copy is `sa.requestMsg` byte for byte; nothing rebuilds, re-encrypts or re-pads it |
| Death only by the budget | No probe path sets `StateDead` except the ordinary request timeout in `serviceRequestWindow`, which gains no probe-aware branch; no window is released without a response, no message id is rewound; and the MTU module stops at the first silent size (`ikeProbeStopAtFirstSilence`) and never probes above the ICMP figure |
| One socket | The probe leaves from the SA's own socket through `sendPath`; no second socket and no second port is opened |
| Window one | Exactly one probe is outstanding per SA; a second request is refused by name while the first holds the window |
| Size on the wire | The AF_PACKET capture, not Ze's report, is what the integration test and the scenario read for AC-5 |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/ike-padded-path-probe-e9e5e97a-9494-4074-a088-c3bd059b2fb3.md` (57 files, verdict clean, 2026-09-16) |
| `review check` | `review_gate: OK (36 code files, clean, hashes match)` |
| Rounds | 2 |
| Reviewer lenses used | round 1, the whole uncommitted diff: (1) goroutine and lock discipline of the owner loop and the transport (`maintainSA` probe/refusal arms, the deferred `failPendingProbe`, `retirePendingProbe` at the `out.newSA` swap, `probePeer` touching no SA state, `Send`/`SendDF` under `mu`, the bounded `refusals` channel, `WithDFMode` restore on every path); (2) RFC 7296 2.1/2.2/2.3 conformance of the retransmission path (bitwise-identical bytes from `sa.requestMsg`, `advanceMsgID` after the send, window one refusals by name, `serviceRequestWindow` unchanged) plus the eight always-in-scope classes (wiring, vacuous tests, AC without test, user-facing behavior without a `.ci`, Linux-only code without QEMU, removed guard, fail-open guard, RFC/interop non-conformance), the style pass over every changed Go file, the security checklist, and the documentation pages against the producers. Round 2, scope written before it ran: the two comment edits (`probe.go` doc-comment order, `established.go` citation), the three page edits (ipsec-7 never-fragment sentence, ipsec-8 and ipsec-9 shared-channel sentence) and the RFC 7383 skeleton repoint |

### Run 1
| Severity | Finding | File | Resolution |
|----------|---------|------|------------|
| NOTE | `probeRefusals` hands every established SA's owner loop the one `Refusals()` channel, so a router's refusal read by another SA's loop is dropped by `handleSizeRefusal` and the DF-clear copy leaves at the retransmit timer rather than at once; correct (the timer path is the RFC 7296 2.1 retransmission) and inside R-4's accepted fallback, but the pages were silent | `internal/component/ike/engine/probe.go`, `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, `ipsec-9-ikev2-eap-nat.md` | one sentence on each page and a Known Limitations bullet; no code change |
| NOTE | the doc comment of `failPendingProbe` sat above `retirePendingProbe`, so godoc attached both comments to the wrong function | `internal/component/ike/engine/probe.go` | the comment moved to its function |
| NOTE | three full-path citations of this spec survive its removal (`established.go` comment, ipsec-8, the RFC 7383 skeleton x4) | as named | restated as the bare stem, or repointed at the page that carries the fact (ipsec-7 gained the never-fragment sentence so the skeleton's row cites a page that holds it) |
| NOTE | `./le repository check`: `StopGraceful`, `PeerInfoMap` unwired exports, both in HEAD before this spec, `PeerInfoMap` called in-package; `./le commit audit`: `notify_error_test.go` (a name-map entry for the new constant, comment reworded) and `keepalive_test.go` (a setup error check removed with the `ResolveUDPAddr` it checked) flagged WEAKENED as RFC-tagged units changed; no assertion about the wire moved | foreign / test setup | the exports are journaled already (`plan/journal/invariant-enforced-by-an-absent-call-site.md`); the two tagged units are admitted with `rfc-change-ok` naming the edits |

Nothing at BLOCKER or ISSUE. Every new exported symbol has a non-test caller: `ikeprobe.Register` (`engine/register.go`), `ikeprobe.Probe` (`mtu/cmd/run.go` `liveDeps`), `UDPTransport.SendDF`/`Refusals` (`engine/probe.go`), `probe.EnableErrorQueue`/`WithDFMode`/`DrainErrorQueue` (`transport/udp_linux.go`, `probe/socket.go`), `QueuedError.Dest` (`transport/udp.go` `deliverQueuedError`), `wire.NotifyZePathProbePadding` (`engine/probe.go`). No peer can reach a `panic()`: the four new `BUG:` panics (`proberKind.String`, `measurement.caveats`, `ikeProber.probe` ceiling, `ikeProber.spend`) sit behind values the MTU module and the engine set, never the peer. Every loop, queue and retry states its bound (`refusalQueueDepth`, `ErrQueueDrainMax`, `ikeExchangesPerRunMax`, `probesPerSizeMax`, the ordinary retransmit budget). The guard audit: `answerProbeRequest` refuses closed on every miss (`sa-down` before anything else, then the window, then `family`, then `size`), and `TestProbeRefusedWhilstRekeyPending`, `TestProbeRefusesANonIPv4Peer` and `TestProbeSizeCeiling` drive each refusal from the request channel, the entry point.

### Run 2
| Severity | Finding | File | Resolution |
|----------|---------|------|------------|
| - | none: the two Go edits are comment-only (`gofmt` clean, the post-write lint clean), the engine, mtu and ikeprobe packages green under `-race` after them (`job-close-engine-race`: `ok ike/engine 93.6s`, `ok mtu/cmd 1.3s`, `ok ikeprobe 1.0s`); `./le doc check verify` reports only the two foreign CLAIMs (`text-format.md`, `formatting.md`); `./le spec citation` reports no reference to this spec | - | - |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | none above NOTE | - | - |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| A third-party peer answers | the `ike-padded-probe-strongswan` scenario passes |
| No SA is torn down by an answered probe | the same scenario asserts the SA survives every probe size and the forced rekey race |
| An unanswered probe fails the SA once and the run stops | the scenario's fragment-drop step asserts the SA fails exactly once, the payload row says `sa-failed` and names the size, and no second size is tried |
| The leaf exists and is registered | `ls internal/core/ikeprobe/registry.go`; `grep -n ikeprobe.Register internal/component/ike/engine/register.go` |
| The transport toggles DF per datagram | `./le job run` on the `ike/transport` integration package under QEMU: the capture shows both copies |
| Every measurement row names its prober | `grep -n '"prober"' test/plugin/show-mtu-ike-probe.ci test/plugin/show-mtu-json.ci` |
| The ledger rows exist | `grep -n 'RFC7296-3.10.1\|RFC7296-3.14' rfc/short/rfc7296.md` and `./le rfc check` |
| The stale sentence is gone | `grep -c 'dropped as out of window' docs/architecture/ike/ipsec-13-rekey-wire.md` prints 0 |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | A probe run must not exhaust the IKE message ID window or the retransmission queue: at most 16 exchanges per run, one outstanding, each on its own id |
| Authorization failing open | The probe is authenticated by the existing SA; it must not be reachable before authentication completes (`StateEstablished` only) |
| Input validation | The requested size is bounded by min(interface MTU, 3000) below and the family minimum above before any buffer is sized; the Notification Data length is derived, never taken from the caller |
| Error leakage | The refusal names a condition (`rekey-pending`, `window-held`, `sa-down`, `size`), never key material or a message id |
| Shared socket | The DF toggle is restored on every exit path, error included, so a later send by another SA never inherits DO or PROBE |
| Amplification | The response is empty, smaller than the request, so a probe cannot be used to make a peer amplify |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| A peer tears the SA down on a probe | STOP at phase 5. Report the peer's log line. The owner decides whether the feature ships |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The comment above `serviceRequestWindow` says freeing the window and carrying on was the third exit RFC 7296 Section 2.1 forbids. A probe is an ordinary request under that section, so it takes the same two exits as a Delete: a response on either copy, or the SA deemed failed after the full retransmit budget. The function gains no probe-aware branch, and the comment stays true. What keeps a diagnostic from taking a live tunnel down is not the engine but the MTU module: it probes only at or below the ICMP figure and stops at the first size that stays silent. Owner ruling (a), 2026-09-16.
- The transport's `Send` locks only the closed check. The DF send needs the write inside the lock, so `Send` gains the same lock around its write; that is the whole of the concurrency change.
- The CBC grid rule (phase 5, design point 0). A CBC suite reaches only datagram sizes on a 16-octet grid, so an ask between two grid points is sent at the grid point BELOW it, never above, and the answer names the size sent (`ikeprobe.Result.WireOctets`). A fit below the ask refutes nothing about the ask: the exchange tested nothing above the size it sent. So the ICMP figure stands as `path-mtu`, and the row's `ike-confirmed` carries the size the SA proved (`measureByIKE`, `run.go`; asked 1400, sent 1388, `path-mtu` 1400, `ike-confirmed` 1388). Without this rule the oversized-tunnels test's advice flipped from `circuit-clamped` to `two-clamps` only because its SA was CBC.
- The `.ci` cannot filter ICMP errors. The far daemons run the noop dataplane, so the daemon side encrypts an echo that never returns; the ICMP search's only information is the router's Fragmentation Needed, and dropping it leaves the search `unmeasurable`, after which the IKE prober never runs (confirm-first). So `show-mtu-ike-probe.ci` proves CONFIRMATION, and the filtered-ICMP story is proven by `ike-padded-probe-strongswan`, where the box exempts ICMP from the clamp so the ICMP figure reads 1500 and IKE refutes it to 1400. A `drop-icmp-errors` flag was added to the clamped-path fixture and removed again when this was understood.
- A product defect phase 5 found and fixed: a probe in flight when the PEER rekeyed the IKE SA was never answered. `maintainSA`'s `out.newSA` arm swapped `sa` and forgot the retired SA with its window, so no response could match and no timeout fired; the reply channel stayed empty and `show mtu` hung for good (scenario runs 1-3). `retirePendingProbe` now answers `refused: rekeyed` with the size sent at the swap, and the MTU side retries the size on the new SA (`ikeProber.probe` maps `RefusalRekeyed` to a silence). Journal: `plan/journal/silent-fall-through.md`.
- The `IP_RECVERR` side effect phase 2 found: with the option set, the kernel hands an ICMP error about an EARLIER datagram to the NEXT send as that send's failure and never sends it (`sock_alloc_send_pskb`), where an unconnected UDP socket without the option dropped the error. `UDPTransport.write` drains the queue on a failed write, delivers what it found on `Refusals()`, and retries once. Journal: `plan/journal/option-set-for-one-caller-changes-another.md`.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Homed as its own spec rather than built into the MTU diagnostic | build both probers at once | `show mtu host <address>` and the reference measurement need ICMP regardless, so this removes no prober; it adds accuracy to one half. Owner decision, 2026-09-11 |
| The shared IKE socket with DF toggled per datagram | a second socket bound to another port for probes | a second port changes the peer's view of the endpoint: RFC 7296 Section 2.23 and RFC 3948 Section 2.1 tie the ESP-in-UDP ports to the IKE ports, and a message from an unexpected port is what a NAT-T peer reads as a float. The measured path must be the path IKE and ESP use. Owner decision, 2026-09-16 |
| One status Notify of a private-use type as the carrier | SK padding (RFC 7296 Section 3.14) | the Pad Length is one octet, so SK padding caps at 255 and an AEAD suite (RFC 5282) pads nothing; a Notify's data field reaches any size. Section 3.10.1 makes an unrecognized status Notify ignorable by every conforming peer, and libreswan proves the alternative wrong: any unknown payload TYPE draws INVALID_SYNTAX and an SA delete |
| Retransmit the identical message with DF clear rather than abandon a silent probe | abandon the probe and free the window | abandoning strands the message id: the peer still expects it, the next DPD at id+1 draws INVALID_MESSAGE_ID, and DPD tears the SA down. Section 2.1 allows the IP header to differ between copies, so the DF-clear copy is a legal retransmission that reaches a peer behind any clamp |
| A probe is an ordinary IKE request: unanswered after the full retransmit budget, DF-clear copies included, it fails the SA through the unchanged `serviceRequestWindow`, exactly as an unanswered DPD fails it | a probe-aware branch of `serviceRequestWindow` that releases the window without `StateDead` and rewinds the message id, so a diagnostic can never deem a production SA failed | Section 2.1 offers two exits, a response or the SA deemed failed, and no third. The alternative is that third exit: a request forgotten while its SA keeps running, and a rewound id the peer may already have consumed. It is a deviation from the RFC, and the owner REJECTED it: conformance is not traded for a diagnostic's convenience. The mitigation moves to the MTU module, where it belongs: confirm-first, stop at the first silent size, report the figure as ICMP-measured and unconfirmed, and say plainly on the page that a fragment-dropping path can take the tunnel down the way any oversized IKE request would. Owner ruling (a), 2026-09-16 |
| Ze implements no RFC 7383 first | implement fragmentation, then probe through it | RFC 7383 Section 2.5.2 makes PMTU discovery OPTIONAL and searches fragmentation thresholds downward; a fragmented probe measures the threshold, not the path. A fragmentation implementation must simply never fragment a probe; `plan/immediate/spec-ike-fragmentation-rfc7383.md` follows this spec and inherits the DF send and the error-queue drain. Owner decision, 2026-09-16 |
| No libreswan lab peer | add a libreswan image (one `apk add` on Alpine 3.21, a Dockerfile, a keyed conf, a fourth peer name in `scenarioPlan` and `prepareScenario`, `whack` helpers) | libreswan's answer to a Notify-only INFORMATIONAL was read from source; the lab cost is a spec of its own. strongSwan is the live proof. Owner decision, 2026-09-16 |
| Confirm the ICMP figure first, descend the ladder only on failure | run the full IKE search from scratch | each IKE exchange holds the SA's window (Section 2.3), so exchanges are the scarce resource; confirming costs two, a full ladder costs many. `pathSearch.confirm` already exists for the reported-figure case |
| The MTU module keeps a budget of exchanges (16), the engine keeps the retransmit budget | one budget in one place | the two budgets bound different things: the engine bounds how long one id is outstanding, the module bounds how long DPD is held off. A single number cannot serve both |
| The error-queue too-big triggers the DF-clear copy at once | wait for the retransmit timer | the timer starts at 500 ms and doubles; a run of 16 exchanges at timer pace would hold the window for minutes. The kernel already tells the socket, and `internal/core/probe` already parses it |
| The mode parameter is `probe.DFMode` | a second enum in `ikeprobe` | one declaration (`ai/rules/principles.md`); the two leaves are both core, and `./le tier check` proves the import direction at implementation |

## Known Limitations

- Only measures a peer with a live IKE SA. A tunnel that is down is exactly the case an operator most wants measured, and ICMP remains the only answer there.
- Without NAT-T the probe rides UDP/500 while ESP rides protocol 50, so the row keeps a "different path than ESP" caveat; only a NAT-T SA measures the path ESP-in-UDP rides.
- Window one serializes everything: while a probe is out no DPD, Delete or rekey leaves, and a run of 16 exchanges holds the SA's request side for up to 16 round trips plus retransmissions. `show mtu` is operator-invoked and bounded, so this is accepted rather than worked around.
- Ze implements no RFC 7383, so an IKE_AUTH carrying certificates over a path that drops fragments still fails to establish. That is `plan/immediate/spec-ike-fragmentation-rfc7383.md`, which inherits the DF send, the error-queue drain, and the constraint that a probe is never fragmented.
- Only strongSwan is proven live; libreswan's behavior is read from source (A-2's sibling claim is unvalidated by a lab).
- On a path that drops IP fragments, a probe can take the tunnel down, the way any IKE request larger than the path would: the DF copy is too big, the DF-clear copies are fragmented and lost, and after the full retransmit budget the SA is deemed failed (RFC 7296 Section 2.1) and re-established by the owner loop. Confirm-first and stop-at-first-silence make this one failure per run at most, and the payload row names it (`sa-failed`, the size). `docs/architecture/diagnostics/path-mtu.md` states the caveat in those words. Owner ruling (a), 2026-09-16.
- The `.ci` (`show-mtu-ike-probe.ci`) proves confirmation only: the far dataplane is a noop, no echo returns, and ICMP cannot be filtered in that harness without leaving the search unmeasurable. Refutation (a `too-big` answer, the DF-clear copy, the fit) is proven by the interop scenario `ike-padded-probe-strongswan` alone.
- The fragment-dropping path in the interop scenario is emulated at strongSwan's reassembly (both `ipfrag` marks cut to 0), not at the box: conntrack defragments before any chain on the box, so no nft rule there can drop a fragment.
- The prober never runs above the ICMP figure: a wrong ICMP figure BELOW the true path MTU is confirmed at the smaller figure and never corrected upward. The IKE probe refutes an ICMP figure that is too large; it cannot raise one that is too small.
- The transport's `Refusals()` channel is one per socket and every established SA's owner loop reads it (`probeRefusals`, `engine/probe.go`), so a router's Fragmentation Needed reaches one loop: read by another SA's loop it is dropped there (`handleSizeRefusal`, R-4), and the probe's DF-clear copy leaves at the retransmit timer (500 ms) instead of at once. AC-11's at-once path is certain on a daemon with one established SA on the socket and probabilistic with several; correctness is the timer's either way. Found at closure (2026-09-16), recorded on `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` and `ipsec-9-ikev2-eap-nat.md`.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

| Site | RFC text to quote |
|------|-------------------|
| the Notify carrier in `engine/probe.go` | RFC 7296 Section 3.10.1: "Notify payloads with status types MAY be added to any message and MUST be ignored if not recognized." |
| the size ceiling in `engine/probe.go` | RFC 7296 Section 2: "MUST be able to send, receive, and process IKE messages that are up to 1280 octets long, and they SHOULD be able to send, receive, and process messages that are up to 3000 octets long." |
| the DF-clear repeat in `established.go` (`serviceRequestRetransmit`) | RFC 7296 Section 2.1: "the initiator MUST retransmit a request until it either receives a corresponding response or deems the IKE SA to have failed." and "A retransmission from the initiator MUST be bitwise identical to the original request. That is, everything starting from the IKE header (the IKE SA initiator's SPI onwards) must be bitwise identical; items before it (such as the IP and UDP headers) do not have to be identical." The probe is an ordinary request under this section: the existing quote above `serviceRequestWindow` already covers its failure, and no deviation row exists |
| the one-outstanding guard | RFC 7296 Section 2.3: "An IKE endpoint MUST wait for a response to each of its messages before sending a subsequent message" |
| the marker in the size budget | RFC 7296 Section 2.23: "The UDP payload of all packets containing IKE messages sent on port 4500 MUST begin with the prefix of four zeros" |
| the never-fragment constraint | RFC 7383 Section 2.4: "it is up to the initiator of each exchange to decide whether or not to use it." |
| the single-silence rule in `mtu/cmd` | RFC 4821 Section 7.6.4: "the state variables eff_pmtu, search_low, and search_high SHOULD NOT be updated, and the same-sized probe SHOULD be attempted again" |
| the three-probes rule in `mtu/cmd` | RFC 8899 Section 5.1.3: "loss of a single probe is not an indication of a PMTU problem" |

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
  - the 18 Unit Tests rows plus `TestProbeRoundsDownToTheCipherGrid`, `TestProbeRetiredByPeerRekeyIsAnsweredRekeyed` (engine), `TestIKEProbeReportsTheSizeThatFit`, `TestIKEFitBelowTheAskKeepsTheICMPFigure`, `TestIKEProbeRekeyedIsRetriedAtTheSameSize`, `TestIKEProbeNeverExceedsTheCeiling`, `TestIKEExchangeBudgetBoundary`, `TestIKEProbeRetriesTooBigBeforeBelievingIt` (mtu), `TestSendSurvivesAnErrorQueuedForAnEarlierDatagram` (transport), the three AF_PACKET tests in `udp_df_integration_linux_test.go`, `show-mtu-ike-probe.ci`, `ike-padded-probe-strongswan`
- [ ] Tests FAIL (paste output)
  - phase 2, `job-ike-df-mut2` (`WithDFMode` cut to install nothing): the second copy left with DF set; nothing reached `Refusals` with `EnableErrorQueue` cut
  - phase 3, `job-probe-p3-mut` (`settleProbe` matching any response id): the replayed DPD answer ended the probe, `TestProbeNeverTouchesDPDState` RED
  - phase 5, `job-p5-grid-mut` (round-DOWN cut to round-UP, `pathMTU=figure`): `TestProbeRoundsDownToTheCipherGrid` and `TestIKEProbeReportsTheSizeThatFit` RED; `job-p5b-mtu-mut` (the confirm rule cut): `TestIKEFitBelowTheAskKeepsTheICMPFigure` RED
  - phase 5, `job-p5-ike-mut` (`answerProbeRequest` cut to refuse): `show-mtu-ike-probe.ci` RED "prober icmp, want ike"
  - phase 5, `scratch/interop-p5b-mut.log` (the DF copy sent `DFOff`): "the measurement's path-mtu is 1500, want 1400: ... exchanges:1 ike-confirmed:1500 prober:ike"
- [ ] Tests PASS (paste output)
  - `ok internal/component/ike/engine 26.329s` (`job-probe-p3c`, 45 PASS); `ok internal/component/ike/transport 0.103s`; `ok internal/component/mtu/cmd 1.275s` and `ok internal/core/ikeprobe 1.034s` (`job-p5b-mtu`); `job-ike-df-int3` 3 PASS under `sudo -n`; `job-p6-mtu-six`: `PASS 721 show-mtu-exhaustive`, `PASS 722 show-mtu-host`, `PASS 723 show-mtu-ike-probe`, `PASS 724 show-mtu-json`, `PASS 725 show-mtu-no-ipsec-component`, `PASS 726 show-mtu-oversized-tunnels`; interop phase 6: `integration: 1 action(s) passed.`
- [ ] Boundary tests for all numeric inputs
  - the confirmation paragraph under the Boundary Tests table; the GAP named there in phase 6 (the `family` refusal had no test) is closed by `TestProbeRefusesANonIPv4Peer`
- [ ] Functional `.ci` tests for end-to-end behavior
  - `test/plugin/show-mtu-ike-probe.ci`, Functional Tests row
- [ ] Interop tests for protocol features (or N-A with a reason)
  - `ike-padded-probe-strongswan`, Interop Tests row

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- A core leaf, `internal/core/ikeprobe` (`Register`, `Probe`, `Request`, `Result`, `Outcome`, `Refusal`, `ErrNotRegistered`), the shape of `ipsecinventory`; the engine registers `probePeer` at `init()` (`engine/register.go`).
- The padded exchange on the owner loop (`engine/probe.go`: `probePeer`, `answerProbeRequest`, `probeNotifyOctets`, `settleProbe`, `handleSizeRefusal`, `sendProbeDFClear`, `retirePendingProbe`, `failPendingProbe`); `maintainSA` gains the request arm, the refusal arm, the deferred `failPendingProbe`, the `settleProbe` correlation beside `dpd.matchesProbe`, and `retirePendingProbe` at the `out.newSA` swap; `serviceRequestRetransmit` repeats a probe holder through `sendProbeDFClear`; `serviceRequestWindow` unchanged (owner ruling (a)).
- The transport: `Send` writes under `mu`, `SendDF` toggles `IP_MTU_DISCOVER` around one write through `probe.WithDFMode`, `IP_RECVERR` at creation through `probe.EnableErrorQueue`, `Run` and `write` drain the error queue through `probe.DrainErrorQueue` and deliver `SizeRefusal` on the bounded `Refusals()` channel; the NAT keepalive writes through `Send`; `udp_linux.go`/`udp_other.go` are the platform split.
- `internal/core/probe` exports the three (`EnableErrorQueue`, `WithDFMode`, `DrainErrorQueue`) and `QueuedError` gains `Dest`.
- `wire.NotifyZePathProbePadding` (65280) in the notify registry.
- The MTU side (`mtu/cmd/run.go`, `search.go`): `measureByIKE` confirm-first, the descent below the ICMP figure, `ikeProber` with the 16-exchange budget, `probeTooBig` retried like a silence, `ikeProbeStopAtFirstSilence`, the `rekeyed` retry, the CBC grid rule (design point 0), `prober`/`exchanges`/`ike-confirmed`/`ike-declined`/`caveats` on the row.
- Tests: 19 engine, 4 transport (3 AF_PACKET integration), 3 leaf, 11 mtu unit tests; `show-mtu-ike-probe.ci`; the interop scenario `ike-padded-probe-strongswan` with `checkIKEPaddedProbeStrongSwan` and four helpers; `./internal/component/ike/transport` in `integrationPackages`.

### Bugs Found/Fixed
- A probe in flight when the peer rekeyed the IKE SA was never answered (`maintainSA` swapped `sa` and forgot the retired window); `retirePendingProbe` answers `refused: rekeyed` and the MTU side retries the size. Covered by `TestProbeRetiredByPeerRekeyIsAnsweredRekeyed`, `TestIKEProbeRekeyedIsRetriedAtTheSameSize`. Journal: `plan/journal/silent-fall-through.md`.
- `IP_RECVERR` hands an ICMP error about an earlier datagram to the next send as its failure; `UDPTransport.write` drains and retries once. Covered by `TestSendSurvivesAnErrorQueuedForAnEarlierDatagram`. Journal: `plan/journal/option-set-for-one-caller-changes-another.md`.

### Documentation Updates
- Pages edited, each with source anchors naming the producers: `docs/architecture/ike/ipsec-7-ikev2-engine.md` ("The padded path probe on the wire", `probeNotifyOctets`, `NotifyZePathProbePadding`; closure added the never-fragment sentence), `ipsec-8-ikev2-child-xfrm.md` ("INFORMATIONAL requests, the window and retransmission", "The padded path probe": `probePeer`, `answerProbeRequest`, `settleProbe`, `handleSizeRefusal`, `PeerSession.probeRequests`; closure added the shared-channel sentence), `ipsec-9-ikev2-eap-nat.md` ("The two IKE sockets": `Send`, `SendDF`, `Run`, `SizeRefusal`, `Refusals`, the platform files; closure added the shared-channel sentence), `ipsec-13-rekey-wire.md` (the stale sentence replaced, `handleOwnedInbound`), `docs/architecture/diagnostics/path-mtu.md` ("The IKE prober": `ikeProber`, `measureByIKE`, `declineIKE`, the constants), `active-probes.md` (`Dest`, "A foreign socket": `EnableErrorQueue`, `WithDFMode`, `DrainErrorQueue`), `core-design.md` (the `ikeprobe` leaf: `Register`, `Probe`, `probePeer`), `docs/architecture/api/commands.md`, `docs/guide/command-reference.md`, `docs/features.md`, `docs/comparison.md`, `docs/functional-tests.md` (`showMTUIKEProbe`, `showMTUIKEProbeRow`), `docs/architecture/testing/interop.md` (`checkIKEPaddedProbeStrongSwan` and the four helpers), `rfc/short/rfc7296.md` (65280 row, `RFC7296-3.10.1-4`, the 3.10.1/3.14/2.1 coverage sentences).
- `./le doc check verify` at closure: 2 CLAIM findings, both foreign (`docs/architecture/api/text-format.md`, `FamilyIPv4Unicast`; `docs/features/formatting.md`, `validCLIFormats`); `./le docs-to-code index-update` regenerated nothing (the indexes were current); `./le spec citation`: no reference to this spec remains, 16 foreign dangling references.

### Deviations from Plan
- The "size passes, size+1 fails" confirm of the Unit Tests row became "the ICMP figure is asked once, never figure+1" (owner-confirmed 2026-09-16): a figure+1 exchange is the DF-clear copy a fragment-dropping path loses.
- A-4's offender is the router, never the peer; the peer is `QueuedError.Dest`, which did not exist in the plan.
- A-6's fragment drop is emulated at strongSwan's reassembly (`ipfrag_low_thresh`/`ipfrag_high_thresh` to 0), not on the box: conntrack defragments before any chain.
- Design point 0 (the CBC grid, round DOWN, `Result.WireOctets`, `ike-confirmed` below `path-mtu` on a fit below the ask) was not in the plan; the owner decided it in phase 5.
- The `.ci` proves confirmation only; refutation is the interop scenario's (Known Limitations).
- The `rekeyed` refusal and `retirePendingProbe` were added for a defect phase 5 found (Bugs Found/Fixed).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-4 named the peer as the error-queue offender | the offender is the router that answered; the refused datagram's destination is the name of the `MSG_ERRQUEUE` read, now `QueuedError.Dest` | `TestOversizedDFSendQueuesRefusalForThePeer` (phase 2) | `Dest` added, A-4 row corrected, `active-probes.md` documents it |
| assumption | A-6 planned to drop the DF-clear copy's fragments on the NAT box with an `iptables -f` rule | conntrack defragments before any netfilter chain on both containers, so the rule matched nothing and run 4 measured 1400 as if the fragments crossed | the scenario's step 3 read `exchanges 16`, `path-mtu 1400` where `sa-failed` was expected (phase 5) | `dropFragmentsAtPeer` cuts strongSwan's reassembly thresholds; A-6 row and `interop.md` corrected |
| approach | phase 5 planned to filter ICMP errors in the `.ci` so IKE would refute a wrong ICMP figure | the far daemons run the noop dataplane, an echo never returns, and without the router's Fragmentation Needed the ICMP search is `unmeasurable`, so the IKE prober never runs (confirm-first) | the `drop-icmp-errors` fixture flag produced `unmeasurable` (phase 5) | the flag was removed; the `.ci` proves confirmation, the interop scenario proves refutation; Known Limitations says so |
| approach | phase 4 read the spec's "(size passes, size+1 fails)" literally and asked figure+1 | every other sentence of the spec says at or below the ICMP figure, and figure+1 is the DF-clear copy a fragment-dropping path loses | phase 4's own reading, confirmed by the owner 2026-09-16 | `TestShowMTUUsesIKEProbeWhenSAIsUp` retargeted to one ask; the Unit Tests row's wording is superseded by the Deviations entry |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A padded INFORMATIONAL probe on the IKE engine, offered to the MTU module as the prober when an SA is up | Done | `engine/probe.go` `answerProbeRequest`; `mtu/cmd/run.go` `measureByIKE` | selected on `Tunnel.Up` (`resolvePeerTargets`) |
| ICMP stays the prober for `host`, the reference and a tunnel that is down | Done | `mtu/cmd/run.go` `measure` (`proberICMP` first, IKE only when `m.peer != ""`) | `TestShowMTUFallsBackToICMPWhenSAIsDown`, `referenceRow` carries `prober: icmp` |
| Padding mechanism decided against the RFC and never readable as malformed | Done | `wire/payload_notify.go` `NotifyZePathProbePadding`; `engine/probe.go` the Notify build | RFC 7296 3.10.1 quoted above the build; charon parsed every request (`requireCharonParsedProbes`) |
| What a non-answer means | Done | `engine/probe.go` `settleProbe` (fits / too-big), `failPendingProbe` (sa-failed) | AC-4, AC-13 tests |
| One retransmission mechanism, not two | Done | `established.go` `serviceRequestRetransmit` carries the DF-clear copy on the ordinary schedule; the MTU side counts exchanges (`ikeExchangesPerRunMax`) | `TestProbeRetransmitIsBitwiseIdenticalWithDFClear`, `TestProbeUnansweredFailsTheSALikeDPD` |
| A non-Ze peer answers | Done | `ike-padded-probe-strongswan` | A-2 confirmed against charon 5.9 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `ike-padded-probe-strongswan` step 1: `prober: ike`, `path-mtu 1400` with Fragmentation Needed dropped toward Ze (`checkIKEPaddedProbeStrongSwan`, `requireIKEMeasured`) | `v3-interop-ipsec.log` PASS |
| AC-2 | Done | `TestShowMTUFallsBackToICMPWhenSAIsDown`, `TestIKEProbeSAFailedEndsTheRun`, `TestIKEProbeUnregisteredIsNotSilence` (`declineIKE`, `run.go`) | row carries `ike-declined` |
| AC-3 | Done | `TestIKEProbeRetriesTooBigBeforeBelievingIt` (`pathSearch.attempt`, `probeTooBig`); scenario step 2 | RFC 4821 7.6.4 quoted above `attempt` |
| AC-4 | Done | `TestProbeAnsweredOnDFClearCopyDoesNotFailTheSA` (`settleProbe`) | `dpdState` compared field by field |
| AC-5 | Done | transport half `TestDFSendThenPlainCopyOnTheWire` (AF_PACKET), engine half `TestProbeRetransmitIsBitwiseIdenticalWithDFClear`, size `TestPaddedInformationalReachesRequestedSize` | QEMU `v3-qemu-ike-mtu-int.log`: `ike/transport` 22 PASS |
| AC-6 | Done | `TestProbeSizeCeiling` (`probeNotifyOctets`, 3001 refused), `TestIKEProbeNeverExceedsTheCeiling` (`ikeWireMax`) | never IKE-fragmented: one message is built, no fragmentation code exists (ipsec-7 says so) |
| AC-7 | Done | 4500 case of `TestPaddedInformationalReachesRequestedSize`; `TestMeasurementRowNamesTheProber` (`measurement.caveats`); scenario `caveats: []` on the NAT-T SA | |
| AC-8 | Done | `TestProbeRefusedWhilstRekeyPending`, `TestIKEProbeRegisteredByEngine` (`answerProbeRequest`, `probePeer`) | refused before the window is touched |
| AC-9 | Done | `TestIKEProbeUnregisteredIsNamed` (leaf), `TestIKEProbeUnregisteredIsNotSilence` (mtu) | `ErrNotRegistered` |
| AC-10 | Done | scenario step 2 (`requireProbeSurvivedRekey`); `TestProbeRetiredByPeerRekeyIsAnsweredRekeyed`, `TestIKEProbeRekeyedIsRetriedAtTheSameSize` | |
| AC-11 | Done | `TestErrQueueTooBigTriggersDFClearAtOnce` (`handleSizeRefusal`); `TestOversizedDFSendQueuesRefusalForThePeer` (`SizeRefusal.Peer`) | Known Limitations: several SAs on one socket share the channel |
| AC-12 | Done | `TestShowMTUUsesIKEProbeWhenSAIsUp` (the DF check under `exhaustive`, `dfMode`), `TestWithDFModeInstallsThenRestores` (`DFBypassCache` is `IP_PMTUDISC_PROBE`) | |
| AC-13 | Done | `TestProbeUnansweredFailsTheSALikeDPD` (engine), `TestIKEProbeSAFailedEndsTheRun` (mtu), scenario step 3 (`requireProbeFailedSA`, `waitZeIKEEstablished`) | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| the 18 Unit Tests rows | Done | as the table names, three file cells corrected in phase 6 | all green under `-race` at closure (`job-close-engine-race`) |
| `TestProbeRoundsDownToTheCipherGrid`, `TestProbeRetiredByPeerRekeyIsAnsweredRekeyed`, `TestProbeRefusesANonIPv4Peer` | Done | `engine/probe_test.go` | added in phases 5-6 |
| `TestIKEProbeReportsTheSizeThatFit`, `TestIKEFitBelowTheAskKeepsTheICMPFigure`, `TestIKEProbeRekeyedIsRetriedAtTheSameSize`, `TestIKEProbeNeverExceedsTheCeiling`, `TestIKEExchangeBudgetBoundary`, `TestIKEProbeRetriesTooBigBeforeBelievingIt` | Done | `mtu/cmd/run_test.go` | |
| `TestSendSurvivesAnErrorQueuedForAnEarlierDatagram`, `TestTransportInstallsErrorQueue`, `TestQueuedSizeRefusalReachesTheEventChannel`, the three AF_PACKET tests | Done | `transport/udp_linux_test.go`, `udp_test.go`, `udp_df_integration_linux_test.go` | |
| `show-mtu-ike-probe` | Done | `test/plugin/show-mtu-ike-probe.ci` | `v3-ci-show-mtu.log` 9/9 natively as root; QEMU `v3-qemu-all.log` PASS in the guest |
| `ike-padded-probe-strongswan` | Done | `test/interop-ipsec/scenarios/ike-padded-probe-strongswan/` | `v3-interop-ipsec.log` PASS; RED `interop-p5b-mut.log` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| every file in Files to Modify | Done | `docs/architecture/testing/qemu-integration.md` needed no edit (no package table; the population is derived) |
| every file in Files to Create | Done | `ls` in Pre-Commit Verification |

### Audit Summary
- **Total items:** 6 requirements, 13 ACs, 6 test groups, 2 file groups
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** 6, recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Remove the ICMP optimism caveat for a peer with a live IKE SA: measure the path the ESP traffic actually meets | interop | `ike-padded-probe-strongswan` step 1: the box clamps UDP and ESP at 1400 and carries ICMP at 1500; `show mtu` answers `path-mtu 1400`, `prober: ike`, `ike-confirmed 1400`, `caveats: []` on the NAT-T SA (`v3-interop-ipsec.log` PASS; RED with the DF copy sent `DFOff`, `interop-p5b-mut.log`: "path-mtu is 1500, want 1400") |
| The padded exchange is one a third-party peer answers and never reads as malformed | interop | the same scenario, `requireCharonParsedProbes` (charon 5.9 "parsed INFORMATIONAL request N" for every size, no INVALID_SYNTAX) and `requirePaddedProbeSAAlive` (a lossless DF ping through the tunnel after 16 exchanges) |
| A user reaches it through `show mtu` with a live SA | functional | `test/plugin/show-mtu-ike-probe.ci`: `prober: ike`, `path-mtu 1400`, `ike-confirmed 1388`, both SAs established afterwards (`v3-ci-show-mtu.log`; RED with `answerProbeRequest` cut to refuse, `job-p5-ike-mut`) |
| A non-answer is classified, and a probe never fails a live SA except through the RFC 7296 2.1 budget | unit + interop | `TestProbeAnsweredOnDFClearCopyDoesNotFailTheSA`, `TestProbeUnansweredFailsTheSALikeDPD` (`job-probe-p3c`, RED `job-probe-p3-mut`); scenario step 3 (`sa-failed at 1500 octets`, `exchanges 1`, the SA re-established) |
| One retransmission mechanism, bitwise identical | unit + AF_PACKET | `TestProbeRetransmitIsBitwiseIdenticalWithDFClear`; `TestDFSendThenPlainCopyOnTheWire` reads DF on the first copy and none on the identical second (`job-ike-df-int3`; RED `job-ike-df-mut2`) |
| ICMP stays for host, reference and a down tunnel | unit + functional | `TestShowMTUFallsBackToICMPWhenSAIsDown`, `TestMeasurementRowNamesTheProber`; the five sibling `show-mtu-*.ci` unchanged in meaning and green (`v3-ci-show-mtu.log`) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none: every in-scope item landed. IKEv2 fragmentation (RFC 7383) was never in this spec's scope; the owner opened it as its own spec on 2026-09-16 | - | `plan/immediate/spec-ike-fragmentation-rfc7383.md` (already committed, 33f280b6a4) |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/ikeprobe/registry.go`, `registry_test.go` | yes | `ls` at closure: `internal/core/ikeprobe/registry.go internal/core/ikeprobe/registry_test.go` |
| `internal/component/ike/engine/probe.go`, `probe_test.go` | yes | `ls`: `internal/component/ike/engine/probe.go internal/component/ike/engine/probe_test.go` |
| `internal/component/ike/transport/udp_linux.go`, `udp_other.go`, `udp_df_integration_linux_test.go` | yes | `ls`: all three listed |
| `test/plugin/show-mtu-ike-probe.ci` | yes | `ls`: `test/plugin/show-mtu-ike-probe.ci` |
| `test/interop-ipsec/scenarios/ike-padded-probe-strongswan/` | yes | `ls`: `nat.conf swanctl.conf ze.conf` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-13 | each row of the Implementation Audit | `job-close-engine-race` at closure: `ok internal/component/ike/engine 93.562s`, `ok internal/component/mtu/cmd 1.270s`, `ok internal/core/ikeprobe 1.038s` under `-race`; the main thread's `v3-*` logs for the functional, interop and QEMU runs (Deliverables) |
| AC-8 | the leaf is registered by the engine | `grep -c ikeprobe.Register internal/component/ike/engine/register.go` = 1 |
| AC-9 | the ledger rows exist | `grep -c 'RFC7296-3.10.1\|RFC7296-3.14' rfc/short/rfc7296.md` = 10 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `show mtu` with a live SA | `test/plugin/show-mtu-ike-probe.ci` (read: `run "ze-test fixture plugin/show-mtu-ike-probe"`, the fixture `showMTUIKEProbeRow` asserts `prober`, `ike-declined` absent, `ike-confirmed` 1388) | yes, `v3-ci-show-mtu.log` PASS |
| `show mtu` with the tunnel down | `TestShowMTUFallsBackToICMPWhenSAIsDown` (unit, the wiring row names it); `show-mtu-oversized-tunnels.ci` covers the ICMP rows | yes |
| the IKE component's `init` | `TestIKEProbeRegisteredByEngine` | yes |
| a probe request on the session channel | `TestPaddedInformationalReachesTheOwnerLoop` | yes |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestDFSendThenPlainCopyOnTheWire`, `TestPlainSendNeverLeavesUnderTheProbeOption` (Assumptions table row) |
| A-2 | confirmed | scenario step 1, `requireCharonParsedProbes` |
| A-3 | confirmed | the box's `FragOKs 16 FragCreates 32` beside 17 Fragmentation Needed dropped (Assumptions row) |
| A-4 | confirmed, corrected | `TestOversizedDFSendQueuesRefusalForThePeer`; the offender is the router, `Dest` is the peer (Mistake Log) |
| A-5 | confirmed | `TestProbeNeverTouchesDPDState`, RED `job-probe-p3-mut` |
| A-6 | confirmed, corrected | scenario step 3 with `dropFragmentsAtPeer` (Mistake Log) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| rows 1, 3, 4, 6, 7, 9, 10, 11, 12, 15, 16, 17 of the checklist (Yes) | each page read at closure against `probe.go`, `established.go`, `udp.go`, `udp_linux.go`, `run.go`, `search.go`, `payload_notify.go`, `checkers.go`; the ipsec-7 octet table matches `probeNotifyOctets`, the ipsec-8 refusal table matches `answerProbeRequest`'s order, the ipsec-9 write table matches `Send`/`SendDF`, path-mtu's key row matches `measurementRow` | yes; `./le doc check verify` at closure: only the two foreign CLAIMs |
| rows 2, 5, 8, 13, 14 (N-A) | no YANG leaf, plugin, SDK surface, route metadata or counter in the diff (`git diff --stat`: no file under `yang/`, `pkg/plugin/`, `internal/plugins/`) | yes |
| the never-fragment constraint the RFC 7383 skeleton cites | added to `ipsec-7-ikev2-engine.md` at closure so the repointed citation names a page that carries it | yes |
