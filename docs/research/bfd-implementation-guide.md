# BFD Implementation Guidance for Clean-Room Reimplementation

## Executive Summary

This document describes the BFD (Bidirectional Forwarding Detection) protocol and FRRouting's `bfdd` architectural choices, for the purpose of writing a clean-room reimplementation in ze (Go). It cites RFC 5880 (base), RFC 5881 (single-hop), RFC 5883 (multi-hop), RFC 5882 (generic application), and the related authentication, seamless, and data-plane RFCs extensively, avoiding any direct reproduction of FRR's C source. The goal is to equip a developer with enough understanding of the protocol and FRR's design pattern to build an independent, wire-compatible implementation without reading FRR source line-by-line.

The guide mirrors the structure of the companion `ospf-implementation-guide.md` and `isis-implementation-guide.md`. BFD is by far the smallest of the three protocols — it has one packet type, one state machine, no database, no SPF — so this guide is correspondingly shorter. The hard parts are not the protocol itself but the client API (how OSPF, BGP, IS-IS, and static routes register sessions and receive down events) and the timing discipline (detecting failure within milliseconds without false positives under load).

### Reference Implementations Examined

This guide is a clean-room reading of two independent reference implementations. No source code, struct layouts, identifier names, comments, or algorithm implementations from either project have been reproduced here; only architectural decisions and protocol semantics are described, with all wire-level normative behaviour cited from the relevant RFCs.

| Project | Commit | Version / Date | Path | Role in this guide |
|---------|--------|----------------|------|--------------------|
| FRRouting `frr` | `cd39d029` | Release 10.5.3 (2026-03-13) | `frr/bfdd/` | Primary reference for module layout, client IPC via zebra, data-plane offload interface, authentication |
| BIRD | `2500b450` (tag `v3.2.1`) | 3.2.1 (2026-04-02) | `bird/proto/bfd/` on `stable-v3.2` | Primary reference for the threaded model, express-loop pattern, direct-call client API |
| BIRD (cross-check) | `f0f859c2` | 2.18 (2026-04-07) | `bird/proto/bfd/` on `master` | Single-threaded 2.x line, used to document what is 3.x-specific |

FRR is GPLv2; BIRD is GPLv2. This guide references their architectural decisions and never their code.

---

## 1. BFD in 200 Lines: Protocol Overview

### What BFD Is

BFD is a lightweight, bidirectional, path-monitoring protocol defined in RFC 5880. Its purpose is simple: detect that a forwarding path between two systems has failed, as fast as possible, without depending on the failure detection built into higher-level protocols (OSPF hello timers, BGP keepalives, LACP timeouts, etc.). Typical BFD sessions detect loss of connectivity in **50–300 ms**, one or two orders of magnitude faster than the protocols that consume BFD's output.

BFD does not carry routing information. It is a pure liveness protocol. When a BFD session comes up or goes down, it notifies its **clients** — the routing daemons and management layers that registered interest in that path. BFD is therefore a service, not a stand-alone protocol: a BFD daemon provides session liveness to client protocols that want faster convergence than their own timers allow.

### Key Concepts

**Session** (RFC 5880 §1): A BFD session is an ordered pair of endpoints monitoring bidirectional reachability between themselves. Each session is identified locally by a 32-bit **discriminator**, assigned freely by its owner, and carries parameters (desired TX interval, required RX interval, detection multiplier) that govern how fast the session runs.

**Control Packets** (RFC 5880 §4.1): BFD endpoints exchange fixed-format 24-byte control packets over UDP. Each packet advertises the sender's current state, its discriminators, its timer parameters, and a handful of flags. Control packets are the primary failure-detection mechanism: if none arrive within a **Detection Time** window, the session is declared Down and the far end is considered unreachable.

**Single-Hop vs Multi-Hop** (RFC 5881, RFC 5883): Single-hop BFD monitors directly-attached links; it runs on UDP port **3784**, enforces TTL 255 (Generalized TTL Security Mechanism), and binds to an outgoing interface. Multi-hop BFD monitors a path across multiple routed hops; it runs on UDP port **4784**, uses normal TTL handling, and is keyed on the source/destination address pair alone. Single-hop and multi-hop sessions can coexist on the same system.

**Echo Mode** (RFC 5880 §6.4, RFC 5881 §5): An optional optimisation. The local end sends UDP packets addressed to itself, routed through the far end. The far end simply loops them back at IP (no BFD processing). Fast round-trip detection is achieved without the far end running a fast BFD engine. Echo packets use UDP port **3785**.

**Three-Way Handshake via Discriminators** (RFC 5880 §6.3): Each session has a local **My Discriminator** and expects to receive the far end's equivalent in **Your Discriminator**. On the first packet, Your Discriminator is unknown and is sent as zero; the receiver associates the packet with its session by source/destination address (and interface, for single-hop) and learns the remote discriminator. From that point on, both sides echo each other's discriminators and lookup is keyed on the local discriminator received in Your Discriminator.

**State Machine** (RFC 5880 §6.2): BFD has only four states: **AdminDown, Down, Init, Up**. Transitions are driven by the state field in received packets combined with the local state. The full FSM fits in a 4x4 table. There are no database exchanges, no Dijkstra, no LSAs — just "is the far end sending Up-state packets?".

**Clients** (RFC 5882): Routing protocols (OSPF, BGP, IS-IS), static routes, and LDP register with the local BFD implementation to ask "tell me when session X goes Up or Down". On a Down event with a diagnostic code, the client tears down the corresponding routing adjacency or next-hop. On an Up event, the client re-forms.

### Design Philosophy

RFC 5880 is intentionally minimal. The protocol has one packet type, one state machine, and one timer discipline. The hard engineering is not in the protocol but in:

1. **Making it go fast** — running 50-ms BFD on hundreds of sessions means tens of thousands of packets per second, and missed Hellos during a GC pause look exactly like a real failure.
2. **Making it go slow when the link is fine** — starting at a conservative 1-second interval and speeding up only after the session is stable prevents congestion collapse during startup.
3. **Integrating with clients cleanly** — multiple clients may share a session, session parameters may change while the session is up, and authenticity matters for security.

A first-pass implementation can skip authentication, demand mode, echo mode, data-plane offload, and RFC 7880 seamless BFD and still provide meaningful value.

---

## 2. Wire Format

### The Mandatory 24-Byte Control Packet

RFC 5880 §4.1 defines a single control-packet format, 24 bytes mandatory plus optional authentication. The fields, in order:

| Offset | Size | Field | Meaning |
|--------|------|-------|---------|
| 0 | 3 bits | Version | Always 1. |
| 0 | 5 bits | Diagnostic | Reason for last Down transition (see table below). |
| 1 | 2 bits | State (Sta) | AdminDown (0), Down (1), Init (2), Up (3). |
| 1 | 1 bit | P (Poll) | Sender wants a parameter change acknowledged. |
| 1 | 1 bit | F (Final) | Sender acknowledges a Poll. |
| 1 | 1 bit | C (CPI) | Control Plane Independent — session survives routing-plane restart. |
| 1 | 1 bit | A (Auth) | Authentication section present. |
| 1 | 1 bit | D (Demand) | Sender wants demand mode. |
| 1 | 1 bit | M (Multipoint) | Reserved for RFC 8562 (multipoint BFD); zero in point-to-point. |
| 2 | 1 byte | Detect Mult | Multiplier applied to detect time. Typical values 3–5. |
| 3 | 1 byte | Length | Total packet length including any auth section. |
| 4 | 4 bytes | My Discriminator | Sender's local session ID. |
| 8 | 4 bytes | Your Discriminator | Echo of the receiver's discriminator, or zero if unknown. |
| 12 | 4 bytes | Desired Min TX Interval | Microseconds. Rate at which the sender wants to transmit. |
| 16 | 4 bytes | Required Min RX Interval | Microseconds. Fastest rate at which the sender can receive. |
| 20 | 4 bytes | Required Min Echo RX Interval | Microseconds. Zero means "I do not support echo". |

All multi-byte fields are network byte order. The packet is followed by an optional variable-length authentication section if the A bit is set.

### Diagnostic Codes

RFC 5880 §4.1 defines nine diagnostic codes (5-bit field, values 0–8 standardised; the rest reserved):

| Code | Name | Meaning |
|------|------|---------|
| 0 | No Diagnostic | Never transitioned to Down, or cleared. |
| 1 | Control Detection Time Expired | Timed out waiting for a control packet. The most common case. |
| 2 | Echo Function Failed | Echo packet was not returned in time. |
| 3 | Neighbor Signaled Session Down | Peer's state field was Down or AdminDown. |
| 4 | Forwarding Plane Reset | Local forwarding plane restarted (e.g., line card reset). |
| 5 | Path Down | Some non-BFD mechanism reports the path down. |
| 6 | Concatenated Path Down | A lower-layer or upstream segment is down. |
| 7 | Administratively Down | Local administrator shut the session down. |
| 8 | Reverse Concatenated Path Down | Reverse path has a segment down (asymmetric failure). |

The diagnostic is set when the local state transitions to Down and is included in every transmitted packet from then on, so the far end can report why its peer went away.

### Authentication Section

RFC 5880 §4.2–4.4 defines five authentication types carried in a trailing TLV-like section. The first byte is the **Auth Type**, the second is the **Auth Length** (including the 2-byte header), and the remainder is type-specific.

| Auth Type | Name | RFC 5880 § |
|-----------|------|------------|
| 1 | Simple Password | §4.2 |
| 2 | Keyed MD5 | §4.3 |
| 3 | Meticulous Keyed MD5 | §4.3 |
| 4 | Keyed SHA1 | §4.4 |
| 5 | Meticulous Keyed SHA1 | §4.4 |

All four keyed variants carry a 1-byte Auth Key ID, a 1-byte Reserved field, a 32-bit Sequence Number, and a fixed-length digest (16 bytes for MD5, 20 bytes for SHA1). The **meticulous** variants require the sequence number to strictly increase with every packet, providing strong replay protection; the non-meticulous variants allow the sequence to remain the same across multiple packets (the sender increments it periodically) and are slightly more tolerant of reordering.

**Scope note.** A first-pass ze implementation should skip authentication altogether and add keyed-SHA1 later if deployments demand it. Simple Password is cleartext and useful only for misconfiguration detection; the MD5 variants are cryptographically weak and on the path to deprecation.

---

## 3. Transport and Encapsulation

### UDP Ports

BFD uses three UDP ports, each with a distinct role:

| Port | Role | RFC |
|------|------|-----|
| 3784 | Single-hop control | RFC 5881 |
| 4784 | Multi-hop control | RFC 5883 |
| 3785 | Echo | RFC 5881 §5 |

The source port is chosen from the ephemeral range (typically 49152–65535). Multiple single-hop sessions to the same destination use distinct source ports so the receiver can demultiplex without relying on the interface alone.

### Single-Hop (RFC 5881)

Single-hop BFD monitors a directly-attached link. The key constraints:

- **TTL/Hop-Limit = 255** on transmit and **TTL/Hop-Limit must equal 255** on receive. This is the Generalized TTL Security Mechanism (GTSM, RFC 5082): any packet received with a lower TTL has been routed and therefore is not from the direct neighbour. Drop it.
- **Destination address** is the peer's unicast address on the link.
- **Source address** is the local interface's address used to reach the peer.
- **Bound to outgoing interface**: on Linux, the sending socket is bound via `SO_BINDTODEVICE` (or the equivalent Go `net.Dialer.Control` hook) so the session is tied to a specific interface.
- **IPv4 and IPv6** both supported with identical semantics.

### Multi-Hop (RFC 5883)

Multi-hop BFD monitors a path through multiple routed hops. Used for:
- iBGP sessions between loopback addresses.
- VPN peers reached via a tunnel or overlay.
- Remote static routes.

The constraints relax:
- **TTL**: no GTSM check. Instead, a configurable **minimum TTL** is enforced: received packets with TTL below the configured minimum are dropped. This is a weaker replacement for GTSM.
- **Destination address**: the peer's routable address.
- **Source address**: any local address that reaches the peer.
- **No interface binding**: the key does not include interface/ifindex.
- **Port 4784** (not 3784), so single-hop and multi-hop can coexist without ambiguity.

### Echo (RFC 5880 §6.4, RFC 5881 §5)

Echo packets are sent on UDP port 3785 with the source and destination IP addresses both set to the **local** router's address. The packet is routed through the far end, which forwards it back at the IP layer because the destination matches the sender's address. The sender measures round-trip time and the far end never runs BFD processing on the packet.

Echo mode is supported only on **single-hop** sessions (RFC 5883 explicitly prohibits it on multi-hop). It requires that the far end advertise a non-zero **Required Min Echo RX Interval** in its control packets — that is the peer's "yes, I can loop echo packets this fast" signal. When echo is active, the control-packet transmission rate slows down (to the larger of the two `RequiredMinRxInterval` values) and echo packets take over fast failure detection.

### IPv6 Specifics

IPv6 BFD sessions work identically to IPv4 but with the IPv6 header's hop limit replacing TTL. Single-hop IPv6 sessions use link-local addresses and bind to the interface via the scope ID. Multi-hop IPv6 sessions use global addresses. The Traffic Class and Flow Label fields are not used by BFD; set them to zero.

---

## 4. Domain Types and Constraints

These are the types any BFD implementation must handle. Each must support equality, hashing, and serialisation.

- **Discriminator** (32-bit unsigned, nonzero): Local session identifier. Must be unique within the local BFD implementation, ideally across reboots to avoid stale-state confusion. FRR uses a counter; a ze implementation can do the same or use a random value from a CSPRNG.

- **SessionKey**: The tuple `(peer_address, local_address, interface_or_vrf, single_or_multi_hop, address_family)`. Used when Your Discriminator is zero (first packet) to associate an incoming packet with a known session. Multi-hop keys omit interface; VRF is always part of the key.

- **Interval** (32-bit unsigned, microseconds): Used for DesiredMinTX, RequiredMinRX, RequiredMinEchoRX. The RFC uses microseconds (not milliseconds or nanoseconds) to allow sub-millisecond timing. Valid range: 0 (for RequiredMinEchoRX meaning "no echo") up to 4 294 seconds (4.3 billion microseconds). Practical values: 50 000 (50 ms) to 1 000 000 (1 s).

- **DetectMult** (8-bit unsigned, 1–255): Multiplier applied to compute detection time. Typical value 3 (so 3 consecutive missed packets triggers Down). A value of 1 is permitted but provides no tolerance for reordering.

- **State** (2 bits): AdminDown, Down, Init, Up.

- **Diagnostic** (5 bits): One of the nine defined codes.

- **Flags** (6 bits in the state byte): P, F, C, A, D, M.

- **SequenceNumber** (32-bit unsigned, auth only): Monotonically increasing in meticulous modes; monotonically non-decreasing in non-meticulous modes. Replay protection.

All of these are fixed-width and straightforward to serialise. The most common bug is treating Interval as milliseconds instead of microseconds, producing sessions that run 1000× slower than intended.

---

## 5. Session State Machine

RFC 5880 §6.2 defines four states and a deterministic transition table. The state machine is small enough to memorise.

### States

| State | Meaning |
|-------|---------|
| AdminDown (0) | Session is administratively disabled. No control packets sent except perhaps a final Down notification. |
| Down (1) | No valid control packet has been received recently, or the peer has signalled Down. |
| Init (2) | We have received a valid control packet from the peer with the peer's state = Down. The peer has heard from us (indirectly) but has not yet confirmed our Down→Init transition. |
| Up (3) | Both sides have acknowledged each other. The session is active. |

The three-way handshake progresses Down → Init → Up. AdminDown is a terminal state that can be entered or left only by administrative action.

### Transition Table

The local state transition depends on the **received state** (what the peer says it is). RFC 5880 §6.8.6 tabulates it:

| Local → \ Received ↓ | AdminDown | Down | Init | Up |
|----------------------|-----------|------|------|----|
| **Down** | stay Down | → Init | → Up | (ignore, stay Down until Init received) |
| **Init** | → Down (diag 3) | stay Init | → Up | → Up |
| **Up** | → Down (diag 3) | → Down (diag 3) | stay Up | stay Up |
| **AdminDown** | — | — | — | — |

Plus the timer-driven transitions:
- **Detection timer expires** in any non-AdminDown state: → Down (diag 1, "Control Detection Time Expired").
- **Echo detection timer expires** when echo is active: → Down (diag 2, "Echo Function Failed").
- **Administrative action**: → AdminDown (diag 7) or from AdminDown → Down on re-enable.

### Notes on the Transitions

- The transition from Down to Up when the received state is Up happens because a peer in Up state can only reach Up if it previously saw our Init, which means our Hellos are getting through. The handshake can therefore collapse from three steps to two on the second round.
- On Init → Up, the local end starts the faster timer negotiation via Poll (see §7).
- On Up → Down, the diagnostic is "Neighbor Signaled" when the peer's state field made it happen, or "Control Detection Time Expired" when local timing made it happen. Two distinct reasons, same state transition.
- AdminDown is not entered by protocol events; only administrative action. Receipt of an AdminDown from the peer causes the local session to drop to Down (with diagnostic 3), not to mirror AdminDown.

### State Change Notification

Every state transition must notify registered clients (see §10). The notification includes the new state, the diagnostic code (for Down transitions), and a timestamp. Clients typically only care about Up ↔ Down transitions; AdminDown is usually equivalent to Down for their purposes.

---

## 6. Discriminator Association and Session Lookup

### The Problem

When a control packet arrives, the receiver must find the right session to feed it into. There are two cases:

**Case 1: Your Discriminator is non-zero.** The packet echoes back the local session's My Discriminator. Look up the session directly in a hash table keyed on local discriminator. O(1), trivially fast.

**Case 2: Your Discriminator is zero.** The peer does not yet know our discriminator (first packet or after a reset). Look up the session by key: `(src_addr, dst_addr, interface, vrf, is_multi_hop, address_family)`. O(1) via a hash table on the composite key. Not as fast as Case 1 but still cheap.

### First-Packet Handling

On the **very first packet** of a session, the local end has not yet learned the peer's My Discriminator, so its transmitted Your Discriminator is zero. When the peer receives this packet, it knows "this is for me" only via the address-tuple lookup. The peer then stores `remote_discriminator = packet.my_discriminator` in its session, and from the next packet onward, Your Discriminator is non-zero and direct lookup works.

RFC 5880 §6.8.6 imposes an important constraint: if a packet arrives with `Your Discriminator == 0` and the local session is **not** in Down or AdminDown, the packet is rejected. This prevents a malicious or confused peer from resetting a working session by sending a zero-discriminator packet.

### Multiple Sessions to the Same Peer

It is legal and common to have several BFD sessions between the same pair of systems — for example, one single-hop session per link, plus one multi-hop session for the iBGP peering. The sessions are distinguished by the key tuple: single-hop includes the interface, multi-hop is keyed on addresses alone, and the UDP ports differ.

### Discriminator Uniqueness

The local discriminator must be unique within the local implementation. It does **not** need to be globally unique. A simple strategy is a local counter starting at 1, incremented on each new session; this yields a compact value and trivial allocation. More paranoid implementations seed the counter with a random 32-bit value at boot so stale-state confusion across reboots is unlikely.

RFC 5880 says discriminators are 32-bit unsigned and zero is reserved for "unknown". Any other value is valid.

---

## 7. Timer Negotiation and Poll / Final

### The Parameters

Each end of a BFD session advertises three timer parameters in every control packet:

- **Desired Min TX Interval**: "I want to transmit at this rate." Local parameter.
- **Required Min RX Interval**: "Do not send to me faster than this." Local parameter.
- **Required Min Echo RX Interval**: "I can loop echo packets this fast." Zero means "I cannot do echo".

### Derived Rates

From these advertisements, each end computes:

- **Actual TX Interval** = `max(local_DesiredMinTx, remote_RequiredMinRx)`. We transmit at our own desired rate unless the peer has told us it cannot keep up, in which case we slow down.
- **Detection Time** = `remote_DetectMult * max(local_RequiredMinRx, remote_DesiredMinTx)`. We declare the session Down if no valid control packet has been received within this window. Note that the **local** DetectMult is not used in the local detection calculation — the remote's multiplier is used because the RFC models the detection from the remote's point of view.

The interesting consequence: if the two ends disagree on intervals (one wants 50 ms, the other 300 ms), the session runs at the slower rate, and the detection time follows. This is by design: BFD never runs faster than either end can handle.

### Slow Start

RFC 5880 §6.8.3 mandates that a session starts with conservative timers: both ends begin advertising an Interval of 1 second (or larger) until the session reaches Up. This prevents a session from hammering the control plane during the handshake, when packets are most likely to be dropped anyway. Once the session is Up, the configured fast timers are negotiated in via the Poll/Final sequence.

### Poll / Final Sequence

RFC 5880 §6.5 and §6.8.2 define how timer parameters change on a session that is already Up. The problem is that changing timers unilaterally would disagree with the peer's detection time, potentially causing a spurious Down transition. The solution is a two-step handshake:

1. Local end sets the P bit in its outgoing control packets **and** changes its advertised intervals to the new values.
2. Remote end receives a packet with P=1 and replies with F=1 (echoing the new intervals it has already picked up from the packet).
3. Local end sees F=1 and knows the remote has adopted the new intervals. Local end clears P, the cycle is complete.

Until the F bit arrives, the local end keeps sending with P=1 and continues using the **old** intervals for its own detection time, so a brief inconsistency does not cause a tear-down.

A Poll sequence applies to any parameter change: a new DesiredMinTx, a new RequiredMinRx, a new DetectMult. Only one Poll can be in flight at a time; if a new change arrives while a Poll is outstanding, queue it until F arrives, then start a second Poll.

### Starting Out

When a session first goes Up (Init → Up or Down → Up), the local end typically **immediately initiates a Poll sequence** to transition from slow-start intervals to configured fast intervals. This is the most common Poll in practice: the ones triggered by administrative reconfiguration are rare.

---

## 8. Echo Mode and Demand Mode

### Echo Mode (RFC 5880 §6.4)

**Prerequisites.** Echo mode is available only if both ends support it and the far end advertises a non-zero `RequiredMinEchoRX`. It is further restricted to single-hop sessions (RFC 5883 §4 prohibits multi-hop echo).

**Operation.** When echo is active on a session:

1. The local end sends echo packets on UDP port 3785, with source and destination IP both set to the local end's own address. Routing forwards these packets to the far end; the far end's forwarding plane loops them back because the destination matches a route back to the originator.
2. The local end measures RTT for each echo packet. If no echo is received for `DetectMult * EchoTxInterval` microseconds, the session goes Down with diagnostic 2.
3. Control packets continue but at a slower rate — the remote's `RequiredMinRxInterval` becomes the actual control-packet TX interval, typically 1 second. This saves CPU on both ends while still providing BFD state maintenance.

**Why echo?** Echo probes the forwarding plane directly. Control packets are handled by the control plane (a BFD daemon), which may experience CPU contention, scheduling jitter, or GC pauses. Echo packets are looped at IP forwarding speed by the far end's hardware or kernel, bypassing the far end's BFD daemon entirely. This makes echo more responsive to real forwarding-plane failures and less susceptible to false positives from control-plane scheduling.

**Implementation note.** The far end need not know the session is in echo mode; it just forwards packets normally. All the state is on the sender side. The cost is that the sender must loop its own packet through the far end's forwarding plane, which requires that (a) the far end has a route back to the sender, and (b) the far end is willing to forward packets destined to the sender's own address.

### Demand Mode (RFC 5880 §6.6)

**Purpose.** Demand mode eliminates periodic control-packet exchange once a session has been established and some other mechanism guarantees path liveness (for example, an underlying hardware that reports link state reliably). Either end can request demand mode by setting the D bit.

**Operation.** When both ends agree on demand mode, control-packet transmission stops. The session remains Up as long as no timer or external event triggers a poll. If either end needs to verify the session, it sends a control packet with P=1; the far end responds with F=1. As long as the far end responds, the session is known to be alive.

**Scope note.** Demand mode is rarely deployed. FRR implements the D-bit field but the full detection-timer-suppression logic is a documented TODO. For a first-pass ze implementation, skip demand mode entirely; the code path is simpler without it, and no real deployment will miss it.

---

## 9. Authentication

BFD authentication (RFC 5880 §6.7) is per-packet and replay-protected. All five defined types share the same 2-byte header (Auth Type, Auth Length); the body is type-specific.

### Simple Password (RFC 5880 §6.7.2)

- Auth Length: 3 to 19 bytes (1 + 1 + 1 + 1–16 password bytes).
- Body: Auth Key ID (1 byte), Password (1–16 bytes).
- Computation: none. The password is sent in cleartext.
- Replay: none.

Use only for sanity checks; not a security mechanism.

### Keyed MD5 (RFC 5880 §6.7.3)

- Auth Length: 24 bytes.
- Body: Auth Key ID (1), Reserved (1), Sequence Number (4), Digest (16).
- Computation: MD5 over the BFD packet with the Digest field replaced by the key material (padded/truncated to 16 bytes).
- Replay: sender increments Sequence Number periodically; receiver rejects packets with a sequence number strictly less than the last accepted.

### Meticulous Keyed MD5 (RFC 5880 §6.7.3)

Same as Keyed MD5 but the receiver rejects any packet whose sequence number is **not strictly greater** than the last accepted. The sender must therefore increment the sequence number on every transmitted packet. Stronger replay protection; less tolerant of packet reordering, which should not happen on single-hop BFD anyway.

### Keyed SHA1 (RFC 5880 §6.7.4)

Same structure as Keyed MD5 but the digest is 20 bytes (SHA-1 output). Auth Length: 28 bytes. Less cryptographically weak than MD5 though SHA-1 itself is also deprecated for cryptographic purposes.

### Meticulous Keyed SHA1

Same as Keyed SHA1 with the strict-increase rule.

### Implementation Notes

- **Key material** is shared out-of-band and never transmitted on the wire.
- **Sequence number persistence** matters: if a router reboots and its sequence number resets to zero, the remote will reject all packets until the local sequence number catches up. Real deployments either persist the sequence number across reboots or use the local system clock as a seed.
- **Key rollover** is not defined by RFC 5880. Real deployments use multiple key IDs with overlapping validity windows, analogous to OSPF and IS-IS keychains.
- **Authentication adds cost.** Every packet is HMACed on transmit and verify-HMACed on receive. At 50 ms intervals with 1000 sessions that is 20 000 HMAC operations per second per side. Budget for it.

**First-pass recommendation for ze**: implement the auth section parser (so authenticated packets are not mistaken for malformed) and reject any packet that carries an auth section (returning a clear error log). Add actual auth support as a follow-up when deployments ask for it.

---

## 10. Client API and Integration

BFD is a service. Other components ask it to monitor paths and subscribe to state-change notifications. The API shape is the hard part of integrating BFD into a larger system.

### Session Request

A client asks for a session by supplying:

- **Peer address** (IPv4 or IPv6).
- **Local address** (optional; derived from the routing table if omitted).
- **Interface** (single-hop only) or **multi-hop flag**.
- **VRF** (or default).
- **Desired timers**: DesiredMinTX, RequiredMinRX, DetectMult. These may be taken from a **profile** rather than given explicitly.
- **Echo-mode preference** (optional).
- **Authentication** configuration (optional).
- **Callback** for state-change notifications.

The BFD implementation returns a handle (a session ID or a Go `*Session`) that the client retains.

### Shared Sessions and Refcounting

Multiple clients can register interest in the same session. For example, an OSPF adjacency and a BGP peering both want to know when the path to `192.0.2.1` goes down. The BFD implementation must **coalesce** the two requests into a single session with a refcount:

1. First client asks for a session: refcount = 1. Session is created and started.
2. Second client asks for the same session (same key, compatible timers): refcount = 2. No new session; the existing one serves both clients.
3. First client deregisters: refcount = 1. Session continues.
4. Second client deregisters: refcount = 0. Session is torn down.

**Compatibility of timers.** If two clients want different DesiredMinTX values, pick the faster of the two (smaller value). Changing timers on a live session uses the Poll sequence. If compatibility is impossible (for example, one wants echo and the other does not), either raise an error or create two independent sessions; the practical answer is almost always to use the most aggressive values.

**Administrative pinning.** If a session was created by explicit configuration (CLI or YANG), it persists at refcount zero so administrative intent is preserved. FRR marks such sessions with a flag; a restart does not remove them.

### State Change Notification

When a session transitions, every registered client is notified. The notification carries:

- New state.
- Diagnostic code (for transitions to Down).
- Timestamp of the transition.
- Optionally, the negotiated timers (clients typically ignore).

Notification delivery is asynchronous. The BFD implementation should not block on slow clients; it should queue notifications and deliver them without holding the BFD event loop's locks. For ze, this maps naturally to the plugin event bus: publish a typed `BfdStateChange` event and let subscribers consume at their own pace.

### Integration With ze's Plugin Model

In ze, the BFD component publishes session state events; OSPF, BGP, IS-IS, static routes, and any other interested subsystem subscribe. The subscriber contract is:

1. Subscribe on startup with a predicate (interested in all sessions, or only sessions matching a peer address, etc.).
2. On notification, update the consumer's own state (tear down an OSPF adjacency, withdraw a BGP route, deactivate a static next-hop).
3. Unsubscribe on shutdown.

A BGP plugin registering BFD monitoring for a peer should do it in two steps: call `bfd.EnsureSession(...)` with the peer's parameters, then subscribe to events for that session. On peer shutdown, unsubscribe and call `bfd.ReleaseSession(...)` so the refcount drops.

---

## 11. Multi-Hop and VRF Specifics

### Multi-Hop Details

The multi-hop flavour (RFC 5883) differs from single-hop in three ways beyond the UDP port change:

1. **No interface binding.** The session is keyed on `(peer, local, vrf, address_family)`. The outgoing interface is determined by the local routing table and may change if the route changes.
2. **No GTSM.** TTL is not forced to 255. A configurable minimum TTL (default typically 254 for "at most 1 hop" or 1 for "any path length") is applied on receive.
3. **Source selection.** The client can let BFD pick the source address automatically via a routing lookup, or can specify it explicitly. Auto-selection is preferable because the source may change if the route changes.

**Route-change sensitivity.** If the route to the peer changes (for example, after an OSPF convergence), the outgoing interface or source address changes. A conservative multi-hop BFD implementation will notice this and restart the session with the new parameters. FRR does this via a nexthop-tracking subscription to zebra.

### VRF and Network Namespaces

Each BFD session belongs to a **VRF**. On Linux, a VRF is typically a network namespace or a VRF device. The session key must include VRF identity because two sessions to the same peer IP in two different VRFs are distinct.

Socket creation binds to the VRF: on Linux, this is done by setting the socket's mark or binding to the VRF device. The specifics are platform-dependent but the architectural rule is clear — the VRF is part of the session's identity.

**ze considerations.** ze already has a VRF concept in its iface component (if not, BFD is a good driver for adding one). The BFD component must accept a VRF name or ID in every session request and carry it through to socket creation.

### ECMP

BFD sessions are **per-nexthop**. If a peer is reachable via multiple equal-cost paths (ECMP), a naive implementation monitors only the active path; a failure that triggers a different path selection is handled by whatever monitors the new path. More sophisticated systems run one BFD session per ECMP member link; this is the domain of RFC 7130 (BFD for LAG) and related work.

For a first-pass implementation, pick one path and monitor it. Sophisticated multi-path BFD is a future enhancement.

---

## 12. Concurrency and I/O Model

### FRR's Model

FRR's bfdd is single-threaded, built on the libfrr event loop. All sockets (one per VRF and address family, for single-hop, multi-hop, and echo) feed the same event loop. Timers are scheduled with microsecond granularity but the loop's practical resolution is limited by the OS scheduler (typically 1 ms on Linux). Session state is kept in hash tables keyed on discriminator and session key.

At small scale (tens of sessions, 300-ms intervals) the single-threaded model is easy and correct. At large scale (thousands of sessions, 50-ms intervals) it becomes a CPU-bound loop doing nothing but packet I/O and HMAC, and hardware offload (see §13) starts to look attractive.

### Model Options for ze

**Model A — One goroutine per session.** Each session runs its own loop with its own ticker. Arithmetic: 1000 sessions × 50 ms interval = 20 000 packets/s TX and RX. A goroutine per session is ~2 MB of stack total — cheap. The OS socket can be shared across sessions (per VRF, per address family, per port) with a dispatcher goroutine reading from it and forwarding to the right session by discriminator lookup.

**Model B — Shared event loop per VRF.** One goroutine owns a hash table of sessions for its VRF and runs all timers off a shared ticker. This is closer to FRR's model and more efficient at large scale but loses some of Go's natural simplicity.

**Recommendation.** Start with **Model A** (goroutine per session) plus a shared RX dispatcher. It is the idiomatic Go approach, easy to reason about, and scales well up to several thousand sessions on modest hardware. Revisit Model B only if profiling shows the per-session overhead dominates the HMAC and packet-encoding costs.

### Socket Layout

For a typical deployment in ze:
- One **RX socket** per VRF per UDP port (3784, 4784, 3785) per address family (IPv4, IPv6). So for the default VRF that is 6 sockets; plus echo if enabled.
- One **TX socket** per session (single-hop) so each session gets its own ephemeral source port. Multi-hop sessions can share a TX socket per VRF.

The per-session TX socket pattern is what FRR does and it eliminates the need for source-port bookkeeping and de-duplication on the receive side.

### Timing Precision

BFD at 50 ms intervals demands reasonable timing precision. Go's `time.Ticker` is not guaranteed to fire exactly on schedule; under GC pressure or scheduler contention, ticks can be late by tens of milliseconds. Two mitigations:

1. **Use monotonic clocks.** Go's `time.Now()` includes a monotonic component by default; do not use `time.Unix()` for interval computations.
2. **Compare deadlines, not tick counts.** When the ticker fires, compute how many packet intervals have elapsed since the last TX and send that many packets. If you simply send one per tick, a late tick makes the session underrun.
3. **Keep the hot path allocation-free.** Reuse buffers. BFD packets are 24–48 bytes; a ringbuffer of preallocated packets is trivial and eliminates per-packet allocation.

---

## 13. Data-Plane Offload (Optional)

### The Idea

FRR includes `dplane.c` and `bfddp_packet.h`, defining a binary protocol between `bfdd` (the control plane) and an external data-plane engine (DPDK, a kernel module, or hardware). The split is:

- **Control plane (bfdd)** owns session policy, timer negotiation, the state machine, and client notification.
- **Data plane** handles packet I/O at line rate and reports state transitions back to the control plane.

Message types flow bidirectionally: `AddSession`, `DeleteSession`, `StateChange`, `SessionCounters`, `EchoRequest`, `EchoReply`, etc.

### Why Offload Matters

At scale (tens of thousands of sessions at aggressive intervals), the control-plane CPU becomes the bottleneck. Hardware or DPDK can run BFD at line rate across thousands of sessions without burning a CPU core.

### Recommendation for ze

**Defer.** The first pass should run BFD entirely in user space. The data-plane protocol is interesting future work and maps naturally to ze's plugin model (BFD control plane as one plugin, data-plane offload as another), but it is not required for the initial implementation.

---

## 14. Configuration Shape

ze uses YANG for configuration. A first-pass BFD module can closely follow RFC 9314 (YANG Data Model for BFD). The essentials:

```yang
module ze-bfd-conf {
  namespace "http://ze.example/yang/bfd";
  prefix "bfd";

  container bfd {
    description "BFD control configuration for ze.";

    leaf enabled { type boolean; default "true"; }

    list profile {
      key "name";
      description "Reusable timer and feature profile.";

      leaf name { type string; }
      leaf detect-multiplier { type uint8 { range "1..255"; } default "3"; }
      leaf desired-min-tx-us { type uint32; default "300000"; }
      leaf required-min-rx-us { type uint32; default "300000"; }
      leaf required-min-echo-rx-us { type uint32; default "0"; }
      leaf echo-mode { type boolean; default "false"; }
      leaf passive-mode { type boolean; default "false"; }

      container authentication {
        leaf type {
          type enumeration {
            enum none;
            enum simple;
            enum keyed-md5;
            enum meticulous-keyed-md5;
            enum keyed-sha1;
            enum meticulous-keyed-sha1;
          }
          default "none";
        }
        leaf key-id { type uint8; }
        leaf key    { type string; }
      }
    }

    list single-hop-session {
      key "peer vrf interface";
      description "Per-link BFD session.";

      leaf peer      { type string; description "Peer IPv4 or IPv6 address."; }
      leaf local     { type string; description "Optional local source address."; }
      leaf interface { type string; description "OS interface name."; }
      leaf vrf       { type string; default "default"; }
      leaf profile   { type string; description "Named profile or inline overrides."; }
      leaf shutdown  { type boolean; default "false"; }
    }

    list multi-hop-session {
      key "peer local vrf";
      description "Multi-hop BFD session between routed endpoints.";

      leaf peer     { type string; }
      leaf local    { type string; description "Local source address (or auto)."; }
      leaf vrf      { type string; default "default"; }
      leaf min-ttl  { type uint8; default "254"; description "Minimum acceptable TTL on receive."; }
      leaf profile  { type string; }
      leaf shutdown { type boolean; default "false"; }
    }
  }
}
```

### Notes

- **Profiles** are reusable bundles of timer and feature parameters. A session references a profile by name; inline overrides are allowed for per-session adjustments.
- **Sessions created by clients** (OSPF, BGP) do **not** live in the configuration tree. They are created at runtime through the client API and live only as long as their requester is interested.
- **Interactive operation**: the typical operator does not configure individual BFD sessions. They enable BFD on an OSPF interface or on a BGP peer, and the relevant daemon asks bfdd to create the session with the operator's timer choices. Configuration-level sessions are for monitoring paths that no protocol watches — for example, liveness of a gateway for a static route.

---

## 15. Plugin Model and Code Organisation for ze

### File Organisation

```
internal/component/bfd/
├── bfd.go                  # Plugin entry point, ze integration
├── config.go               # YANG config parse and apply
├── server.go               # BFD engine lifecycle
├── session/
│   ├── session.go          # Session struct and lifecycle
│   ├── fsm.go              # State machine (Down/Init/Up/AdminDown)
│   ├── timers.go           # Poll/Final, detect, TX timers
│   ├── echo.go             # Echo-mode support (optional)
│   └── session_test.go
├── packet/
│   ├── control.go          # 24-byte control packet codec
│   ├── auth.go             # Authentication section codec
│   ├── diag.go             # Diagnostic code constants
│   └── packet_test.go
├── transport/
│   ├── singlehop.go        # Port 3784, GTSM, interface binding
│   ├── multihop.go         # Port 4784, min-TTL check
│   ├── echo.go             # Port 3785
│   └── socket.go           # Per-VRF per-AF socket management
├── api/
│   ├── register.go         # Client-facing EnsureSession / ReleaseSession
│   ├── events.go           # State-change event definitions
│   └── profile.go          # Named profiles
├── schema/
│   ├── ze-bfd-conf.yang
│   ├── embed.go
│   └── register.go
└── register.go             # Plugin registration
```

### Dependencies

- **iface** component: interface up/down, address changes, VRF binding.
- **sysrib** component: (optional) for nexthop tracking on multi-hop sessions.
- **event stream / bus**: publish state-change events to subscribers.
- **config** component: YANG registration and parse.

### API Shape for Consumers

Clients (OSPF, BGP, static) consume BFD via a small Go interface:

```go
type Service interface {
    // EnsureSession returns an existing session matching the request or
    // creates a new one. Refcount is incremented. Thread-safe.
    EnsureSession(req SessionRequest) (Session, error)

    // ReleaseSession decrements the session's refcount. The session is
    // torn down when refcount reaches zero (unless administratively pinned).
    ReleaseSession(s Session) error
}

type Session interface {
    ID() SessionID
    State() State
    Subscribe() <-chan StateChange
    Unsubscribe(<-chan StateChange)
}
```

Subscribers consume from `Subscribe()`'s channel and handle state changes in their own goroutine. The BFD implementation guarantees that the channel is never closed while subscribers hold a reference, and that unsubscribe cleanly stops the goroutine feeding it.

### Plugin Registration

BFD registers via `init()` in `register.go` like every other ze plugin. The registration declares YANG schema, config handlers, dependencies, and start/stop hooks. CLI commands for `show bfd sessions`, `show bfd profile`, `debug bfd`, etc., are registered through the cli component.

### Reference Architecture: FRR bfdd

FRR splits BFD across 17 files in `bfdd/` totalling roughly 400 kB of C source. The split encodes several deliberate architectural decisions.

| File | Size | Responsibility |
|------|------|----------------|
| `bfd.c` / `bfd.h` | 65 k / 25 k | Session runtime: FSM, timer wheel, session lookup by key and by discriminator, refcounting |
| `bfd_packet.c` | 67 k | Wire codec, socket I/O (all three UDP ports), GTSM enforcement, echo-mode TX/RX |
| `bfdd.c` | 8 k | Daemon process init, libfrr wiring, signal handlers |
| `event.c` | 5 k | Thin event-loop adapter |
| `ptm_adapter.c` | 23 k | Client IPC boundary: zebra ZAPI protocol serving bgpd, ospfd, staticd, etc. |
| `dplane.c` | 30 k | Data-plane offload IPC loop and message dispatch |
| `bfddp_packet.h` | 9 k | BFD Data Plane Protocol wire format — the control/data-plane contract |
| `bfdd_nb.c` + `bfdd_nb_config.c` + `bfdd_nb_state.c` + `bfdd_nb.h` | ~83 k | YANG northbound: config apply, state queries, notifications |
| `bfdd_cli.c` | 38 k | YANG-derived CLI glue |
| `bfdd_vty.c` | 37 k | Legacy VTY command surface (pre-YANG) |

Architectural takeaways:

1. **Session policy is strictly separated from packet I/O.** `bfd.c` owns the state machine and timers; `bfd_packet.c` owns the sockets. This is precisely what makes data-plane offload possible — you can move packet I/O to hardware while keeping policy in software without untangling the two.
2. **Client IPC is pluggable behind `ptm_adapter.c`.** The adapter is the seam where alternative transports could replace the zebra client. In ze the equivalent role is played by the plugin event bus or by a direct Go interface call.
3. **Data-plane offload has a hard interface** (`bfddp_packet.h`). A well-defined wire protocol lets bfdd drive DPDK, a kernel module, or a hardware engine without leaking implementation details into the control plane.
4. **Configuration is served three ways** — legacy VTY, YANG northbound, and YANG-derived CLI — which is why the config surface is larger than the runtime. This reflects FRR's mid-migration state. ze can go straight to a single YANG path and generate CLI from it.
5. **bfdd is a separate daemon**, not a library. Other FRR daemons link to a thin client library (`lib/bfd.c`) and talk to bfdd via zebra. The separation gives process isolation and independent lifetime at the cost of IPC round trips.

### Reference Architecture: BIRD 3.2 proto/bfd

BIRD takes the opposite approach. Its `proto/bfd/` is **six files** — about one tenth of FRR's count:

| File | Responsibility |
|------|----------------|
| `bfd.c` | Session lifecycle, FSM, daemon coordination, cross-loop synchronisation via callbacks |
| `bfd.h` | Data model (proto, session, request, iface), public API declarations |
| `packets.c` | Wire codec, socket I/O, RX demultiplexer by discriminator |
| `config.Y` | YACC grammar for BFD sections in BIRD's unified config language |
| `Makefile` / `Doc` | Build plumbing and user documentation |

**BIRD release lines.** BIRD upstream maintains two parallel stable lines. The `master` branch is version **2.18** (single-threaded, one cooperative I/O loop for the whole daemon, no per-protocol pthreads). The `stable-v3.2` branch is version **3.2.1** (threaded, per-protocol `birdloop` objects, a low-latency pthread "express loop" for BFD). Both ship the same six-file layout above — the partitioning of code into `bfd.c`, `bfd.h`, `packets.c`, and `config.Y` is unchanged across the major-version line — but the **threading model and cross-loop state publication are fundamentally different**. This guide describes v3.2.1 semantics except where explicitly noted; in 2.18, the express loop does not exist and BFD runs on the single main loop alongside BGP, OSPF, and everything else, which means BFD's timing discipline in 2.18 is subject to the same scheduling jitter that motivated the v3 rewrite.

Architectural takeaways:

1. **No separate daemon.** BIRD is a single process; BFD is a protocol plugin sharing address space with BGP, OSPF, RIP, and the routing core. Client registration is a direct function call, not an IPC round trip. When OSPF wants a session, it calls into BFD and receives a request handle; state changes arrive via a C callback.
2. **No client-IPC adapter.** There is nothing analogous to FRR's `ptm_adapter.c` because there is no inter-process boundary. The seam is a function pointer.
3. **No northbound / YANG / VTY split.** BIRD uses its own unified configuration grammar (`config.Y`) and its own `show protocols` CLI, both shared with every other protocol. Configuration is parsed once at daemon start; there is no running-config / candidate-config separation.
4. **BIRD 3 introduces a per-protocol "express loop" (v3 only, not in 2.x).** Every BFD protocol instance creates two `birdloop` objects: the normal protocol loop (shared with other protocols for reconfiguration and CLI) and a dedicated low-latency worker pthread for packet RX, TX timers, and FSM transitions. State is published across the two loops via atomic state pairs (local and remote state packed into a single 64-bit word) and callback-based deferred notifications. This is the single most important architectural idea in BIRD 3's BFD: lock-free, sub-10 ms jitter because the express loop never runs configuration or CLI work. **In BIRD 2.x this is absent** — BFD shares the main loop with every other protocol and pays the full scheduling cost of any slow operation elsewhere in the daemon.
5. **Shared RX sockets, per-interface TX sockets.** One shared RX socket per (address-family, single-hop-or-multi-hop) — four sockets total — with packet dispatch by discriminator hash on receive. TX sockets are per-BFD-interface (refcounted) so the kernel picks the right source address and interface via socket binding. FRR uses a per-session TX socket; BIRD shares across sessions on the same interface.
6. **Wire-layer authentication is implemented** (simple, keyed MD5, keyed SHA1, and both meticulous variants). **Echo mode and multipoint BFD are not present** as of v3.2.1 — FRR implements echo, BIRD does not.

### Lessons for ze

Both projects encode real wisdom, and ze should borrow the best of each:

- **From BIRD: run BFD on its own loop/goroutine.** Go makes this trivial. A dedicated goroutine for timers and packet I/O, separate from the configuration-apply path and the CLI handlers, avoids scheduling jitter under load. Model A from §12 (goroutine-per-session) aligns with this philosophy.
- **From BIRD: direct function-call client API.** Since ze is a single binary with plugin components (not separate processes), OSPF and BGP should talk to BFD through a typed Go interface (§10's `Service` / `Session`), not through a plugin event round-trip. Events are for state notifications, not for session establishment.
- **From FRR: keep packet I/O strictly separated from session policy.** ze's `packet/` and `session/` sub-packages (§15) already reflect this. Do not let the wire-format code know about timer negotiation or state machines.
- **From FRR: the data-plane offload seam is worth designing for even if unused.** A clean `packet/` ↔ `transport/` boundary (where `transport/` could in principle be replaced by an XDP/eBPF back end later) keeps the door open without forcing implementation now.
- **From neither: the triple configuration split.** FRR has three overlapping config surfaces because it is mid-migration; BIRD has one but it is a bespoke grammar. ze has YANG from the start — use one schema and generate CLI from it, do not create parallel paths.

---

## 16. Testing Strategy

### Unit Tests

1. **Packet codec.** Round-trip every control-packet field, every flag combination, every auth type. Use RFC 5880 Appendix A test vectors where available.
2. **State machine.** Assert every cell of the RFC 5880 §6.8.6 transition table. Also test the timer-driven transitions (detection timer fires, echo detection timer fires, administrative toggles).
3. **Discriminator lookup.** Test both fast-path (Your Discriminator non-zero) and slow-path (zero) lookup. Test first-packet handling, including the rule that non-Down sessions reject zero-discriminator packets.
4. **Timer negotiation.** Drive a Poll/Final exchange, verify the new intervals are committed after F arrives, and verify the detect time updates correctly.
5. **Slow start.** Verify that sessions start at 1 second and transition to configured intervals only after Up.
6. **Refcount.** Assert that multiple `EnsureSession` calls on the same key coalesce into one session with a correct refcount.

### Fuzz Tests

Decode random byte sequences as BFD packets. The decoder must never panic. Start with a corpus of valid packets plus deliberately malformed variants (short length, oversized auth section, invalid version, etc.).

### Integration Tests (`.ci`)

1. **Two-session up.** Two instances on a simulated link reach Up within ~2 seconds (slow-start period) and then accelerate to configured intervals.
2. **Failure detection.** Kill one side mid-session, verify the other side transitions to Down within `DetectMult * interval` and reports diagnostic 1.
3. **Admin toggle.** Administratively shut a session; verify the peer reports Down with diagnostic 3. Unshut; verify recovery.
4. **Timer change.** On a live session, change DesiredMinTX via configuration; verify the Poll/Final sequence and the actual TX rate change after F arrives, not before.
5. **Echo mode.** Enable echo; verify control-packet rate slows and echo packets flow; kill the return path; verify diagnostic 2.
6. **Multi-hop.** Set up a multi-hop session across a two-hop namespace topology; verify adjacency and failover.
7. **VRF isolation.** Two sessions with the same peer IP in different VRFs must be independent.
8. **Refcount.** Ask OSPF and BGP to monitor the same peer; verify one session is created; verify deregistration from one does not tear down the other.

### Interop Tests

Run against FRR's bfdd in a namespace. Verify every session state transition agrees with FRR's `show bfd peer` output. Verify packet traces captured by `tcpdump` match expected byte layouts.

---

## 17. Known Hard Problems and Traps

### 1. Microseconds, Not Milliseconds

The RFC uses microseconds for all interval fields. A DesiredMinTX of `300` means 300 µs — three hundred microseconds — not 300 milliseconds. Confusing the two produces sessions that either refuse to come up (too fast for the peer) or run 1000× slower than intended. Always double-check the unit on every field that carries a time.

### 2. Who Is in Charge of the Detection Timer

RFC 5880 §6.8.4 says the local detection time is computed from the **remote** multiplier and the **negotiated** interval:

    detection_time = remote_DetectMult × max(local_RequiredMinRX, remote_DesiredMinTX)

A common mistake is to use the local multiplier. This produces sessions that fail or recover at the wrong rate and gives spurious diagnostic-1 down events.

### 3. First-Packet Zero Discriminator

RFC 5880 §6.8.6 says to reject a packet with Your Discriminator zero if the local session is not in Down or AdminDown. Ignoring this rule creates a trivial DoS: an attacker (or a confused peer) can reset any session by sending a single zero-discriminator packet.

### 4. Poll/Final Race

If the local end changes parameters while a Poll is already in flight, the naive approach sends a second packet with P=1 before the first F arrives. The peer gets confused. Correct handling: queue the change until F arrives, then start a second Poll.

### 5. Detection During Slow Start

During slow start, intervals are 1 second. If the configured operating interval is 50 ms, sessions take up to `DetectMult × 1 s` to reach Up — potentially 3 s or more. Clients that expect subsecond up times must account for this; BFD does not skip slow-start to accelerate initial convergence.

### 6. Sequence Number Persistence Across Restart

Authenticated sessions use monotonically increasing sequence numbers. If a bfd daemon restarts and resets its sequence counter, the peer rejects all packets until the new sequence catches up, which is forever because it never will. Persist the sequence number across restarts, or seed it with a value larger than any plausible previous value (e.g., current Unix time in microseconds).

### 7. GC Pause Looks Like a Failure

At 50 ms intervals with `DetectMult=3`, a 150 ms stall looks exactly like a failure. Go's GC is usually sub-10 ms but under pressure can spike. Keep allocations off the hot path; pre-allocate packet buffers; profile under load to catch surprises. If STW pauses cannot be eliminated, slow BFD down or use hardware offload.

### 8. Clock Source Mismatch

Interval arithmetic must use monotonic clocks. Wall-clock jumps (NTP, VM migration) produce spurious up/down events. Go's `time.Now()` includes a monotonic component; use it. Never use `time.Unix()` for interval math.

### 9. Binding Sockets to the Right Interface

On Linux, single-hop BFD must bind its transmitting socket to the outgoing interface (`SO_BINDTODEVICE`) so the OS does not pick a different interface when multiple paths exist. This is easy to forget and the bug manifests only on routers with multi-homed interfaces.

### 10. TTL on Receive (GTSM)

RFC 5082 GTSM requires the receiver to check that the incoming packet's TTL is 255 for single-hop BFD. Failing to enforce this makes the single-hop session trivially spoofable from anywhere in the network. Linux `IP_RECVTTL` socket option exposes the TTL on the ancillary data of every received packet; use it.

### 11. Interleaved Echo and Control Packets

Echo mode receives packets on UDP 3785 that are BFD but from the forwarding plane, not the peer's BFD engine. Do not feed them into the session state machine; they are only for RTT measurement. A common bug is running echo packets through the full packet parser and being surprised when "state Up" messages arrive at wrong times.

### 12. Multi-Hop Source Address Changes

Multi-hop sessions pick a source address from the routing table. If routes change, the source may change; a BFD session whose source changed mid-stream will be rejected by the peer because the peer's session is keyed on the old source. Subscribe to routing-table changes and restart affected sessions.

### 13. Do Not Clear Your Discriminator Once You Know It

Once a session has learned the peer's My Discriminator, keep it until the session is torn down. A naive implementation that re-learns it from every packet will be tricked by a confused peer that sends zero.

### 14. Authentication Type Mismatch

If one side runs authenticated BFD and the other does not, packets are silently rejected and the session never comes up. The operator sees a session flap that appears to have no cause. Detect and log this loudly: a packet received without the A bit on a session that expects authentication, or vice versa, should log a clear error.

---

## 18. What FRR Implements and What It Does Not

### Fully Implemented (RFC 5880 Core)

- All four states with the full transition table.
- Single-hop (RFC 5881) and multi-hop (RFC 5883).
- Echo mode, single-hop only.
- Slow-start and Poll/Final timer negotiation.
- Diagnostic codes 0–8.
- GTSM on single-hop, min-TTL check on multi-hop.
- IPv4 and IPv6.
- VRF binding.

### Partial or Deferred

- **Authentication**: the packet-parsing side is present, but full keyed-MD5/SHA1 production handling has outstanding TODOs. For a first-pass ze implementation this is fine; authentication can be added later.
- **Demand mode (RFC 5880 §6.6)**: the D bit is recognised but detection-timer suppression is not fully wired.
- **S-BFD (RFC 7880, RFC 7881)**: seamless BFD reflector and initiator modes exist in the codebase but are not the main deployment path. Defer.
- **Micro-BFD on LAG (RFC 7130)**: per-member-link sessions on LAG bundles. Defer.
- **Multipoint BFD (RFC 8562)**: for multicast distribution-tree monitoring. Niche; defer indefinitely.

### Supported Integrations

FRR's bfdd integrates with ospfd, ospf6d, bgpd, isisd, pimd, staticd, pbrd, eigrpd, and others via the zebra-mediated client API. The protocol daemons each include a thin `*_bfd.c` file that translates protocol-specific events into BFD session lifecycle calls.

### Recommended scope for ze's first pass

**MUST:**
- Control-packet codec.
- Single-hop transport on UDP 3784 with GTSM.
- Multi-hop transport on UDP 4784 with min-TTL.
- State machine (Down, Init, Up, AdminDown) with the full transition table.
- Discriminator-based session lookup plus address-tuple lookup on first packet.
- Slow start and Poll/Final timer negotiation.
- Session refcounting for shared use by multiple clients.
- Client API with subscribe/unsubscribe and a `Service` interface.
- YANG configuration (profiles and explicitly-pinned sessions).
- Integration with ze's iface component for interface/VRF events.

**SHOULD:**
- Echo mode.
- IPv6 support.
- Nexthop tracking for multi-hop sessions.
- CLI `show bfd` commands.

**DEFER:**
- Authentication (RFC 5880 §6.7).
- Demand mode.
- S-BFD (RFC 7880, RFC 7881).
- Micro-BFD on LAG (RFC 7130).
- Data-plane offload (FRR's bfddp).
- Multipoint BFD (RFC 8562).

---

## 19. Recommended Implementation Order

### Phase 1 — Packet Codec

**Goal.** Encode and decode the 24-byte control packet and the optional authentication header shell (parsing auth type and length only, not validating digests).

**Deliverables.** Codec, diagnostic-code constants, flag constants, round-trip tests.

**Test gate.** Unit tests for every field, including all nine diagnostic codes and every flag combination. Round-trip a few packets captured from FRR with `tcpdump` and verify byte-for-byte equality.

### Phase 2 — State Machine

**Goal.** Implement the four-state FSM with the full RFC 5880 §6.8.6 transition table, plus the detection-timer-driven transitions.

**Deliverables.** `Session` struct, `fsm.go` with the transition table, timer stubs.

**Test gate.** Unit tests asserting every cell of the transition table. Tests for detection-timer expiry, administrative toggle, and the first-packet zero-discriminator rule.

### Phase 3 — Transport and I/O

**Goal.** Real UDP sockets, real transmission, real reception, single-hop only.

**Deliverables.** Per-session TX socket bound to interface, per-VRF RX socket, GTSM enforcement, socket-to-session dispatch via discriminator lookup.

**Test gate.** Two in-process sessions communicate via real localhost sockets (on different interfaces or with separate namespaces). Adjacency reaches Up within the slow-start window. Packet captures look right.

### Phase 4 — Slow Start and Poll/Final

**Goal.** Implement the slow-start 1-second interval and the Poll/Final timer negotiation.

**Deliverables.** Slow-start logic, Poll initiation, Final detection, committed-interval tracking.

**Test gate.** A session reaches Up at slow-start rate, then transitions to configured fast timers via Poll/Final. Verify the peer does not time out during the transition.

### Phase 5 — Multi-Hop

**Goal.** Add multi-hop transport on UDP 4784.

**Deliverables.** Multi-hop socket, min-TTL check, session-key rules without interface binding.

**Test gate.** Multi-hop session across a two-hop namespace topology works end to end.

### Phase 6 — Client API and Refcounting

**Goal.** Let OSPF, BGP, and static clients register sessions and subscribe to state changes.

**Deliverables.** `Service` interface, `EnsureSession` / `ReleaseSession`, event publishing to subscribers.

**Test gate.** Two independent clients share one session via refcount. State changes are delivered to every subscriber. Unsubscribing cleanly stops delivery.

### Phase 7 — Configuration and CLI

**Goal.** YANG module, profile management, explicit-session pinning, `show bfd` commands.

**Deliverables.** YANG schema, config-apply handler, operational CLI.

**Test gate.** Configure a profile, pin a session to it, observe it running in `show bfd sessions`.

### Phase 8 — Echo Mode

**Goal.** Single-hop echo mode per RFC 5880 §6.4.

**Deliverables.** UDP 3785 socket, echo transmission, RTT tracking, detection timer from echo packets.

**Test gate.** Echo-enabled session rides over slowed-down control traffic; killing the return path triggers diagnostic 2.

### Phase 9 — Interop

**Goal.** Verify wire compatibility with FRR's bfdd.

**Deliverables.** `.ci` tests in a namespace topology with FRR bfdd and ze bfd cooperating across a link (or across several links for multi-hop).

**Test gate.** Every combination of transport (single-hop, multi-hop) and address family (IPv4, IPv6) forms an adjacency with FRR as the peer.

### Phase 10+ — Optional Extras

Authentication, demand mode, S-BFD, LAG BFD, data-plane offload. Add when real deployments need them.

---

## 20. Reference RFCs and Summary

### Core

1. **RFC 5880** — Bidirectional Forwarding Detection (BFD). The base specification. Required.
2. **RFC 5881** — BFD for IPv4 and IPv6 (Single Hop). UDP 3784, GTSM, echo port 3785.
3. **RFC 5883** — BFD for Multihop Paths. UDP 4784, no GTSM, min-TTL.
4. **RFC 5882** — Generic Application of BFD. How client protocols should use BFD.

### Authentication and Security

5. **RFC 5082** — Generalized TTL Security Mechanism. Background for the single-hop TTL=255 rule.

### Extensions (Optional / Deferred)

6. **RFC 7130** — BFD on Link Aggregation Group Interfaces (Micro-BFD).
7. **RFC 7880** — Seamless BFD.
8. **RFC 7881** — Seamless BFD for IPv4, IPv6, and MPLS.
9. **RFC 8562** — BFD for Multipoint Networks.
10. **RFC 5884** — BFD for MPLS LSPs.
11. **RFC 5885** — BFD for the Pseudowire Virtual Circuit Connectivity Verification.
12. **RFC 9127** — YANG Data Model for BFD (base).
13. **RFC 9314** — YANG Data Model for BFD with Enhancements.

### Implementation Priorities (Condensed)

- **Must have.** RFC 5880, RFC 5881, RFC 5883, RFC 5882 (client semantics), RFC 5082 GTSM. Single-hop and multi-hop over IPv4 and IPv6. Full state machine. Client API with refcounting. YANG config.
- **Should have.** Echo mode. Operational CLI. Interop with FRR in a namespace.
- **Defer.** Authentication, demand mode, S-BFD, micro-BFD for LAG, BFD for MPLS LSPs, data-plane offload.

### Strategy

1. Implement Phases 1–6 for a minimum-viable BFD service: packet codec, state machine, single-hop and multi-hop transport, slow-start, client API with refcounting. This is enough for OSPF and BGP to start using BFD for subsecond failure detection.
2. Add Phases 7–8 for configuration, CLI, and echo mode.
3. Validate with Phase 9 (interop with FRR).
4. Treat extensions as demand-driven — do not write authentication or S-BFD until a deployment actually needs it.

BFD is the smallest of the three link-state / liveness protocols covered in this research series (BFD, IS-IS, OSPF) but has the largest multiplier: once BFD is working, every routing protocol in ze gains subsecond failure detection for free. Invest in correctness, especially the state machine and the timer arithmetic, because a buggy BFD implementation is far worse than no BFD at all — it produces false-positive failure events that tear down the routing tables it is supposed to protect.

Good luck with the implementation. The protocol is small, the state machine is four lines, the packet is 24 bytes, but the engineering discipline to make it go fast without going wrong is the same as for any other high-rate networking code: preallocate, avoid GC on the hot path, use monotonic clocks, test the transitions exhaustively, and interop against a known-good peer.
