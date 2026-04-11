# OSPF Implementation Guidance for Clean-Room Reimplementation

## Executive Summary

This document describes the OSPF (Open Shortest Path First) protocol and the architectural decisions made by FRRouting's `ospfd` (and `ospf6d`), for the purpose of writing a clean-room reimplementation in ze (Go). It cites RFC 2328 (OSPFv2), RFC 5340 (OSPFv3), and the extension RFCs extensively, avoiding any direct reproduction of FRR's C source. The goal is to equip a developer with enough understanding of the protocol and FRR's design pattern to build an independent, wire-compatible implementation without needing to study FRR's source line-by-line.

The guide deliberately mirrors the structure of the companion document `isis-implementation-guide.md` so that a reader already familiar with the IS-IS guide can navigate quickly. Where FRR makes a design choice that differs from ze's philosophy (e.g., eager parsing versus lazy parsing, multi-area sharing versus per-area isolation), this is called out explicitly.

### Reference Implementations Examined

This guide is a clean-room reading of two independent reference implementations. No source code, struct layouts, identifier names, comments, or algorithm implementations from either project have been reproduced here; only architectural decisions and protocol semantics are described, with all wire-level normative behaviour cited from the relevant RFCs.

| Project | Commit | Version / Date | Path | Role in this guide |
|---------|--------|----------------|------|--------------------|
| FRRouting `frr` | `cd39d029` | Release 10.5.3 (2026-03-13) | `frr/ospfd/` | Primary reference for OSPFv2 module layout, state machines, LSDB, extensions. OSPFv3 lives in `frr/ospf6d/` and is only referenced in §15. |
| BIRD | `2500b450` (tag `v3.2.1`) | 3.2.1 (2026-04-02) | `bird/proto/ospf/` on `stable-v3.2` | Primary reference for the per-packet-type file split, unified v2+v3 single-binary design, and single-LSDB domain-scoped architecture |
| BIRD (cross-check) | `f0f859c2` | 2.18 (2026-04-07) | `bird/proto/ospf/` on `master` | Used to confirm the v2/v3 unification predates the v3 threading rework |

FRR is GPLv2; BIRD is GPLv2. This guide references their architectural decisions and never their code.

---

## 1. OSPF in 300 Lines: Protocol Overview

### What OSPF Is

OSPF is a link-state interior gateway protocol standardized for IPv4 by RFC 2328 (version 2) and for IPv6 by RFC 5340 (version 3). Unlike IS-IS, which runs directly over the data link layer, OSPF runs over IP as protocol 89. OSPFv2 uses IPv4 directly (no UDP, no TCP), addressing packets to unicast peers or to the well-known multicast addresses `AllSPFRouters` (224.0.0.5) and `AllDRouters` (224.0.0.6). OSPFv3 uses IPv6 with the corresponding multicast addresses `ff02::5` and `ff02::6` (RFC 5340 §2.9).

OSPF is a **link-state** protocol: every router builds a complete map of the area by receiving Link State Advertisements (LSAs) from every other router in the area, then runs Dijkstra's algorithm locally to compute shortest paths. This is the same high-level approach as IS-IS, but with a very different wire format and a very different area model.

### Key Concepts

**Areas** (RFC 2328 §3.1): OSPF partitions the routing domain into areas, each identified by a 32-bit Area ID (written in dotted-quad form, e.g. `0.0.0.0`). Area `0.0.0.0` is the **backbone** and must be contiguous or stitched together by virtual links. Non-backbone areas connect to the backbone through one or more Area Border Routers (ABRs). Routers flood most LSAs only within their home area; summary LSAs cross area boundaries via ABRs. This design localises topology churn and bounds the size of the link-state database (LSDB) on any single router.

**Router ID** (RFC 2328 §3.2): Each OSPF router has a 32-bit Router ID, unique within the autonomous system, written in dotted-quad form. It identifies the router in LSA origination and in the Dijkstra tree. The Router ID is usually derived from a loopback interface address or set explicitly; it must not change during the life of the process without purging self-originated LSAs.

**Packet Types** (RFC 2328 §A.3): OSPF defines five packet types, carried directly over IP protocol 89:
- **Hello (type 1)**: Discovers neighbours and maintains adjacencies.
- **Database Description (DD, type 2)**: Summarises the LSDB during adjacency synchronisation.
- **Link State Request (LS Request, type 3)**: Requests specific LSAs that are missing or stale.
- **Link State Update (LS Update, type 4)**: Carries one or more LSAs, used both for initial flooding and for retransmission.
- **Link State Acknowledgment (LS Ack, type 5)**: Acknowledges received LSAs, direct or delayed.

All packets share a fixed 24-byte header in OSPFv2 (16 bytes in OSPFv3) that carries the version, type, length, originating Router ID, area ID, checksum, and authentication fields.

**LSA Types** (RFC 2328 §A.4, RFC 3101, RFC 5250): OSPF expresses topology and reachability through typed LSAs. The core set is:
- **Type 1 Router-LSA**: Emitted by every router. Describes its attached links and their metrics. Area-scoped.
- **Type 2 Network-LSA**: Emitted by the Designated Router (DR) on a broadcast or NBMA segment. Describes the transit network and lists the routers attached. Area-scoped.
- **Type 3 Summary-LSA (Network)**: Emitted by ABRs into neighbouring areas. Advertises an IP prefix reachable through the ABR. Area-scoped.
- **Type 4 Summary-LSA (ASBR)**: Emitted by ABRs. Advertises reachability to an AS Boundary Router (ASBR) in another area. Area-scoped.
- **Type 5 AS-External-LSA**: Emitted by ASBRs. Describes a route to a destination outside the OSPF domain (e.g. from BGP, static, kernel). Flooded to the entire AS except stub and NSSA areas.
- **Type 7 NSSA-External-LSA** (RFC 3101): Like Type 5, but originated within a Not-So-Stubby Area (NSSA) and scoped to that NSSA. Translated to Type 5 by the NSSA ABR for the rest of the AS.
- **Type 9, 10, 11 Opaque-LSAs** (RFC 5250): Application-specific LSAs scoped respectively to link-local, area, and AS. Carry TLV bodies used by extensions (TE, Router Information, Segment Routing, Graceful Restart grace LSA, Extended Link/Prefix).

Types 6 (Group-Membership) and 8 (External-Attributes) are historical and effectively unused in modern deployments.

**Designated Router / Backup Designated Router** (RFC 2328 §9.4): On broadcast and NBMA networks, one router is elected DR and another BDR. The DR is responsible for originating the Type 2 Network-LSA on behalf of the segment and for acting as the hub of flooding on the segment. Non-DR/BDR routers form full adjacencies only with the DR and BDR, reducing the number of adjacencies on a LAN from `N*(N-1)/2` to `2*(N-1)`. A two-phase election is used (first BDR, then DR), with priority and Router ID tiebreakers. The DR is **sticky**: a newly-joined router with higher priority does not automatically displace an existing DR.

**Adjacency** (RFC 2328 §10): Unlike IS-IS, where a neighbour either is or is not an adjacent peer, OSPF distinguishes "neighbours" (routers seen in Hellos) from "adjacencies" (full database synchronisation). On a point-to-point link, every neighbour becomes an adjacency. On a broadcast/NBMA link, adjacency is formed only with the DR and BDR. The NSM (Neighbor State Machine) drives the adjacency through nine states (Down, Attempt, Init, 2-Way, ExStart, Exchange, Loading, Full, plus a transient Deleted). The DD exchange in Exchange state synchronises LSDBs; the Loading state drains the LS Request list; Full state is the steady state.

**Flooding** (RFC 2328 §13): When a new or updated LSA arrives, the router compares it to its local copy (by LS sequence number, age, and checksum), installs it if it is newer, and floods it out the other interfaces subject to scope rules. Retransmission lists per neighbour track unacknowledged LSAs; unacknowledged LSAs are resent every `RxmtInterval` seconds (default 5) until they are acked or the neighbour goes down.

**SPF** (RFC 2328 §16): Each router runs Dijkstra's algorithm per area, with itself as the root, to build the area's shortest-path tree. Type 3/4 summaries are then folded in to compute inter-area routes. Type 5 external-LSAs are processed last to compute routes to AS-external destinations. The result is installed in the local RIB and passed to the FIB manager.

### Common Packet Header

All OSPFv2 packets share a fixed 24-byte header:
- **Version** (1 byte): Always 2 for OSPFv2.
- **Type** (1 byte): One of the five packet types (1–5).
- **Packet Length** (2 bytes): Total length including this header.
- **Router ID** (4 bytes): Originating router's 32-bit Router ID.
- **Area ID** (4 bytes): Area this packet belongs to, in 32-bit form.
- **Checksum** (2 bytes): Standard IP-style 16-bit one's complement of one's complement sum over the whole packet (including header, excluding the Authentication field), per RFC 1071. Zero means "not computed"; valid implementations always compute it.
- **AuType** (2 bytes): Authentication type: 0 null, 1 simple password, 2 MD5 (legacy cryptographic), plus the post-header keyed trailer for HMAC-SHA (RFC 5709, RFC 7474).
- **Authentication** (8 bytes): AuType-specific. For simple password, the 8 key bytes. For MD5, the key ID, auth data length, and crypto sequence number; the digest itself is appended **after** the packet body.

After this header, each packet type has its own body (see §3). OSPFv3 strips authentication from the header and moves it to an optional auth trailer (RFC 7166), making the header 16 bytes.

---

## 2. Wire Format and Packet Types

### Protocol and Addressing

OSPFv2 packets are sent in raw IPv4 datagrams with IP protocol number 89. The source address is the interface's primary address. Destination addresses depend on the packet type and network type:

- **Broadcast network, DR or BDR sending Updates or Acks to non-DR neighbours**: `AllDRouters` (224.0.0.6) — only the DR and BDR join this group.
- **Broadcast network, anything else**: `AllSPFRouters` (224.0.0.5) — all OSPF routers join this group.
- **Point-to-point**: `AllSPFRouters` (224.0.0.5), although the link has only one neighbour anyway.
- **NBMA and point-to-multipoint**: Unicast to each configured neighbour.
- **Virtual link**: Unicast to the far-end Router ID's transit address.

OSPFv3 uses the analogous IPv6 multicast addresses `ff02::5` and `ff02::6` (RFC 5340 §2.9). Virtual link packets are unicast globally-routable IPv6.

Packets are sent with IP TTL 1 on most network types, which means they cannot be routed off the link. Virtual links are the exception: they use normal routed IP so they can traverse a transit area.

### Packet Type Details

**Hello (type 1)** — RFC 2328 §A.3.2 / RFC 5340 §A.3.2. Body fields (OSPFv2):
- Network mask (4 bytes): Local interface mask. Must match on broadcast, point-to-multipoint, and NBMA; ignored on point-to-point unnumbered links.
- HelloInterval (2 bytes): Seconds between Hellos.
- Options (1 byte): Optional capability bits (E for AS-external support, N for NSSA, O for opaque, DC for demand circuits, etc.).
- Router Priority (1 byte): DR election weight (0 means "not eligible for DR").
- RouterDeadInterval (4 bytes): Seconds of silence before the neighbour is declared dead.
- Designated Router (4 bytes): The sender's current belief about the DR's interface address (or 0.0.0.0 if unknown).
- Backup Designated Router (4 bytes): Same for BDR.
- Neighbor list (variable, 4-byte entries): Router IDs of all neighbours heard on this link within the last RouterDeadInterval.

Hellos must agree on network mask (where applicable), HelloInterval, RouterDeadInterval, area ID, and the E-bit before adjacency can form (RFC 2328 §10.5). A mismatch is logged but silently discarded.

**Database Description (type 2)** — RFC 2328 §A.3.3 / RFC 5340 §A.3.3. Body fields (OSPFv2):
- Interface MTU (2 bytes): Local MTU. A mismatch can block adjacency (subject to MTU-Ignore override).
- Options (1 byte).
- Flags (1 byte): Init (I), More (M), Master (MS). These drive the master/slave negotiation and the multi-packet DD exchange.
- DD Sequence Number (4 bytes): Monotonically increasing during Exchange.
- LSA headers (variable, 20-byte entries): Headers (not full LSAs) of LSAs the sender has in its database.

DD packets are the primary means of synchronising LSDBs when a new adjacency is formed.

**Link State Request (type 3)** — RFC 2328 §A.3.4. Body is a list of triples (LS Type, LS ID, Advertising Router), each 12 bytes, naming LSAs the requester wants in full. Sent unicast during Loading state.

**Link State Update (type 4)** — RFC 2328 §A.3.5. Body is a 4-byte count followed by one or more full LSAs, each starting with the 20-byte LSA header and ending with the LSA-type-specific body. Used both for initial flooding and for retransmission to neighbours on the retransmit list.

**Link State Acknowledgment (type 5)** — RFC 2328 §A.3.6. Body is a list of 20-byte LSA headers, one per acknowledged LSA. Can be sent direct (immediately, unicast) or delayed (coalesced, typically multicast). RFC 2328 §13.5 Table 19 specifies the direct-versus-delayed decision.

### Sequence Numbers, Ages, and Checksums

**LS Sequence Number** (RFC 2328 §12.1.6): 32-bit signed integer starting at `0x80000001` (called `InitialSequenceNumber` in the RFC) and counting upward. The maximum usable value is `0x7FFFFFFF` (`MaxSequenceNumber`). When the originator reaches that maximum, it must flush the old LSA with age 3600 before re-originating with the initial sequence number. The sequence number comparison in RFC 2328 §13.1 uses signed comparison relative to the initial value; implementations must get this right or they will lose updates.

**LS Age** (RFC 2328 §12.1.1): 16-bit unsigned field, in seconds, with a maximum value of `MaxAge` (3600). Age increments as the LSA sits in the LSDB and, on receipt, is incremented by `InfTransDelay` to account for link latency. A received LSA with age equal to `MaxAge` is a **purge** — it must be retained long enough to be flooded, not silently dropped.

**Do-Not-Age bit** (RFC 1793 / RFC 4136): The high bit of the age field (`0x8000`) marks an LSA as frozen — used in demand-circuit networks and during graceful restart. The actual age is the low 15 bits.

**Checksum** (RFC 2328 §12.1.7, RFC 905 Annex C): The LSA header's Checksum field is a Fletcher-16 computed over the entire LSA **except** the age field. The age field is zeroed in the buffer used for the Fletcher calculation, and the position of the checksum field itself participates in the adjustment. The algorithm is identical to the one used by IS-IS (ISO 8473), and FRR shares a single implementation; a reimplementer should expect to test against RFC 905 Annex C test vectors and common Fletcher pitfalls (see §15).

The OSPF packet header (24 bytes) carries a separate checksum: a 16-bit one's complement over the whole packet including the header but excluding the authentication field, per RFC 1071. For cryptographic authentication (AuType 2 or the RFC 7474 trailer), the packet checksum field is set to zero and authentication replaces it.

---

## 3. LSA Registry and LSDB Organisation

### LSA Common Header

Every LSA, regardless of type, begins with a 20-byte header:
- **LS Age** (2 bytes).
- **Options** (1 byte): Capability bits describing what the originator supports.
- **LS Type** (1 byte): One of 1, 2, 3, 4, 5, 7, 9, 10, 11.
- **Link State ID** (4 bytes): Type-specific identifier within the (Type, Advertising Router) pair.
- **Advertising Router** (4 bytes): The originator's Router ID.
- **LS Sequence Number** (4 bytes).
- **Checksum** (2 bytes).
- **Length** (2 bytes): Total LSA length including this header.

The **database key** for an LSA is the triple `(LS Type, Link State ID, Advertising Router)`. The LSDB is typically organised as a per-area map keyed on this triple, with Type 5 LSAs kept in an AS-wide map (since they flood across area boundaries).

### Per-Type Body Layouts

**Type 1 Router-LSA** — RFC 2328 §A.4.2. Body:
- Flags (1 byte): V (virtual-link endpoint), E (ASBR), B (ABR), and a few less-common bits for multicast and NSSA.
- Reserved (1 byte).
- Number of links (2 bytes).
- A list of link descriptors, each 12 bytes: Link ID (4), Link Data (4), Type (1), Num TOS (1), Metric (2). The Link ID and Link Data fields are interpreted based on Type: point-to-point link to a router uses the neighbour's Router ID and the local interface address; transit link to a DR uses the DR's interface address and the local interface address; stub link uses the network's IP address and the subnet mask; virtual link uses the remote Router ID and the local interface address. FRR and every other implementation ignores the obsolete TOS (per-QoS metric) extension; the base metric is the only one used.

**Type 2 Network-LSA** — RFC 2328 §A.4.3. Body:
- Network Mask (4 bytes).
- Attached Routers (variable, 4-byte entries): Router IDs of every router fully adjacent on the segment, including the DR itself.

The Link State ID of a Network-LSA is the DR's interface address on the segment, not the network prefix. This is a frequent source of confusion — the LS ID of the Router-LSA is the originating Router ID, but the LS ID of the Network-LSA is the DR's **interface address**.

**Type 3 Summary-LSA (Network)** — RFC 2328 §A.4.4. Body:
- Network Mask (4 bytes).
- Reserved (1 byte).
- Metric (3 bytes, big-endian).

The Link State ID is the summarised network prefix itself.

**Type 4 Summary-LSA (ASBR)** — RFC 2328 §A.4.4. Same body layout as Type 3 but the Link State ID is the ASBR's Router ID and the network mask field is unused (set to zero).

**Type 5 AS-External-LSA** — RFC 2328 §A.4.5. Body:
- Network Mask (4 bytes).
- E-bit + Reserved + Metric (1 byte E/reserved, 3 bytes metric): E=0 means E1 (internal cost added), E=1 means E2 (external cost only).
- Forwarding Address (4 bytes): Usually 0.0.0.0 (meaning "send to the advertising ASBR"). Non-zero means traffic should be sent to that address directly if it is reachable via the OSPF topology.
- External Route Tag (4 bytes): Carried transparently, used by route-policy rules on other routers.

**Type 7 NSSA-External-LSA** — RFC 3101 §3.1. Identical body layout to Type 5 but the semantics are different: it is flooded only within its NSSA, and one of the area's ABRs (the "NSSA translator", elected per RFC 3101 §3.5) replicates it as a Type 5 on the backbone.

**Type 9 / 10 / 11 Opaque-LSAs** — RFC 5250 §3. The Link State ID's high byte is the opaque type and the low 24 bits are the opaque ID. The body is TLV-encoded (2-byte type, 2-byte length, value, padded to 4-byte boundary), with each TLV defined by the owning extension RFC. Known opaque types used in production today:

| Opaque Type | Name | RFC | Purpose |
|-------------|------|-----|---------|
| 1 | TE-LSA | RFC 3630 | Traffic-engineering link and node attributes |
| 4 | Router Information | RFC 7770 | Router capabilities, including SR algorithms |
| 7 | Extended Prefix Opaque | RFC 7684 | Prefix-SID and per-prefix attributes |
| 8 | Extended Link Opaque | RFC 7684 | Adjacency-SID and per-link attributes |
| 3 | Grace-LSA | RFC 3623 | Graceful-restart announcement |

Opaque-LSAs are not parsed by the core SPF machinery; they are flooded like any other LSA but dispatched to registered extension handlers.

### LSDB Layout

FRR keeps the LSDB as an array of per-type tables, per area, each table a radix/route-node tree keyed on `(LS ID, Advertising Router)`. Type 5 and Type 11 live in an AS-wide table on the instance. Each LSA is reference-counted; references come from the LSDB itself, per-neighbour retransmit lists, per-neighbour LS Request lists, SPF vertices under construction, and delayed-ack lists. Deletion happens when the refcount drops to zero and the LSA has been marked discardable.

**ze's lazy-over-eager philosophy.** A ze implementation should keep each LSA as its on-wire byte slice plus a thin header struct (type, LS ID, advertising router, sequence, checksum, length, age countdown). Body parsing happens only when SPF needs it or when the CLI asks for it. The benefit is obvious: flooding a Type 5 from neighbour A to neighbours B and C requires no parsing at all, and the checksum recomputation on purge does not need a parsed representation either. The cost is that extension handlers (TE, SR) need to parse on demand; since those extensions are out of scope for the first pass, this is a good trade.

### Maximum Age, Purge, and Refresh

RFC 2328 §14.1 defines three timers that drive LSA aging:

- **MaxAge** (3600 s): An LSA at this age is a purge and must be flooded like any other LSA, held in the LSDB long enough to acknowledge to all neighbours, and then deleted.
- **MaxAgeDiff** (900 s): Two LSAs are "functionally equivalent" for freshness comparison only if their ages differ by less than this much. This prevents slightly different ages from being treated as updates.
- **LSRefreshTime** (1800 s): The originator must re-flood its own LSAs at this cadence with a new sequence number. Without refresh, the LSAs would hit MaxAge and be purged.

Purge handling is subtle (see §15). A purged LSA must be kept in the LSDB until every neighbour has acknowledged it, otherwise routers that missed the purge will continue using the stale copy.

---

## 4. Domain Types and Constraints

The following types show up in any OSPF implementation. Each must support equality comparison, ordering (for LSDB indexing), and serialisation.

- **RouterID** (4 bytes, fixed): 32-bit identifier, displayed as dotted quad. Uniquely identifies a router in the AS. Should normally be derived from a stable loopback address. Changing the Router ID forces re-origination of every self-LSA and a full re-synchronisation with every neighbour.

- **AreaID** (4 bytes, fixed): 32-bit area identifier. `0.0.0.0` is reserved for the backbone. Areas are scalar identifiers — they do not have a structural hierarchy like IS-IS area addresses.

- **LSAKey**: The triple `(Type, LinkStateID, AdvertisingRouter)`. Within a given area (or AS for Type 5), this triple uniquely names an LSA. Equality and hashing are essential. Comparison does **not** include the sequence number — that is the "version" of the LSA, not part of its identity.

- **LSSequenceNumber** (4 bytes, signed): Monotonically increasing from `0x80000001` up to `0x7FFFFFFF`. Comparison under RFC 2328 §13.1 treats the sequence number as signed but with wraparound rules that favour freshness. See §15 for the trap.

- **LSAge** (2 bytes, unsigned, seconds): Counts upward from 0 at origination. The high bit is the DoNotAge flag (RFC 4136). Reaches MaxAge (3600) then purge kicks in.

- **Metric** (typically 16 bits for link cost, 24 bits for LSA metrics): Integer cost; lower is better. Default interface cost is derived from `ReferenceBandwidth / InterfaceBandwidth`, rounded to at least 1. OSPFv3 retains the same semantics.

- **HelloInterval / RouterDeadInterval** (2 bytes / 4 bytes, seconds): Per-interface negotiated in Hellos. Must match exactly between neighbours or adjacency is refused.

- **InterfaceID**: OSPFv3 adds a per-router 32-bit interface identifier (since link addresses are no longer globally unique for link-local IPv6 addresses). OSPFv2 identifies an interface by its IP address.

- **DDSequenceNumber** (4 bytes): Incremented during DD exchange. The master sets the initial value; the slave echoes it; both sides increment in lockstep. Used only within a single adjacency's Exchange phase.

All of these types are immutable keys or counters; they must be compared carefully for equality and for ordering. A common subtle bug is storing a Router ID as `net.IP` and accidentally comparing by slice identity rather than by 4-byte value.

---

## 5. State Machines

OSPF has two distinct state machines (not one, as in IS-IS): an Interface State Machine per OSPF-enabled interface and a Neighbor State Machine per neighbour. They interact through the DR/BDR election, the AdjOK? check, and shared LSDB operations.

### 5a. Interface State Machine (ISM)

**RFC 2328 §9.3** defines eight states:

| State | Meaning |
|-------|---------|
| Down | Interface is administratively or operationally down. No OSPF activity. |
| Loopback | Interface is a loopback. Advertised as a host stub link in the Router-LSA but sends no Hellos. |
| Waiting | Broadcast or NBMA interface is brought up and is delaying DR/BDR election until `RouterDeadInterval` seconds have passed or a BackupSeen event fires. |
| Point-to-Point | Interface is operational on a point-to-point, point-to-multipoint, or virtual link. No DR/BDR election. |
| DROther | Broadcast/NBMA interface is up, DR and BDR are elected, and this router is neither. |
| Backup | This router is the elected BDR on the segment. |
| DR | This router is the elected DR on the segment. |

**Events**: InterfaceUp, WaitTimer, BackupSeen, NeighborChange, LoopInd, UnloopInd, InterfaceDown, plus UnfinishedHello for startup delays.

**Key transitions**:
- On `InterfaceUp` with a point-to-point-style network type, go straight to Point-to-Point.
- On `InterfaceUp` with a broadcast or NBMA network type and non-zero priority, go to Waiting and arm the Wait timer for `RouterDeadInterval` seconds. With priority 0, skip DR election and go straight to DROther (eventually).
- On `WaitTimer` or `BackupSeen`, run the DR/BDR election and transition to DR, Backup, or DROther.
- On `NeighborChange` (a neighbour just entered or left 2-Way on this interface), re-run the DR election.
- On `InterfaceDown`, tear down every neighbour on the interface and go to Down.

The `BackupSeen` event fires when a Hello arrives declaring a DR or BDR. It exists to short-circuit the Wait timer when there is already a working election state.

### 5b. DR/BDR Election

**RFC 2328 §9.4** defines a two-phase algorithm:

**Phase 1 — BDR election.**
1. Consider every router on the segment (including self) with priority > 0 and NSM state ≥ 2-Way.
2. Exclude any router that currently advertises itself as the DR (that router is "taken").
3. Among the rest, the router that advertises itself as BDR wins. If no candidate advertises itself as BDR, the router with the highest priority wins. Router ID breaks ties.

**Phase 2 — DR election.**
1. Consider the same set.
2. If any router advertises itself as DR, that router stays DR (the **sticky** rule — this is how OSPF avoids churn on a working LAN).
3. Otherwise, the BDR elected in Phase 1 is promoted to DR.
4. If the DR has been elected (or re-elected) and the former BDR's role is now vacant, re-run Phase 1 to fill it.

The algorithm then compares the new (DR, BDR) to the old. If either role changed, the `NeighborChange` event propagates to every 2-Way neighbour, triggering an `AdjOK?` check on the NSM.

**Why sticky?** If a high-priority router joined a LAN with a working DR, naively running the election would always promote the new router, causing every other router to tear down and rebuild its adjacencies. The sticky rule means the new router sits as DROther until the DR fails.

### 5c. Neighbor State Machine (NSM)

**RFC 2328 §10.1** defines nine states (including the transient Deleted used only for cleanup):

| State | Meaning |
|-------|---------|
| Down | No Hello received recently. The initial state and the state after InactivityTimer. |
| Attempt | NBMA only. Poll interval is running; we expect to hear from this configured neighbour eventually. |
| Init | A Hello has been received but the sender did not list us in its neighbour list yet (one-way). |
| 2-Way | The neighbour is listing us, so we have confirmed bidirectional Hello reachability. On broadcast networks, many neighbours will stay here indefinitely — 2-Way is the steady state for neighbours with which we do not form a full adjacency. |
| ExStart | We have decided to form an adjacency and are negotiating master/slave for the DD exchange. |
| Exchange | DD packets with LSA header lists are flowing; both sides are describing their databases. |
| Loading | DD is done; we are sending LS Request packets for any LSAs we need and waiting for the neighbour to flood them to us. |
| Full | Synchronised. Participates in SPF, flooding, and route computation. |
| Deleted | Transient. Being torn down. |

**Events**: HelloReceived, Start, 2-WayReceived, NegotiationDone, ExchangeDone, BadLSReq, LoadingDone, AdjOK, SeqNumberMismatch, 1-WayReceived, KillNbr, InactivityTimer, LLDown.

**Key transitions**:
- `HelloReceived` restarts the InactivityTimer and, if we are in Down, moves to Init.
- `2-WayReceived` fires when we notice our own Router ID in the neighbour's Hello's neighbour list. Before this, we are in Init (one-way); after, we are in 2-Way. The ISM also records this as an interface-level event for DR election.
- On 2-Way, the router calls the `should_adj` predicate (see below). If adjacency is wanted, fire `Start` to move to ExStart; otherwise stay in 2-Way.
- `NegotiationDone` ends ExStart's master/slave election and moves to Exchange.
- `ExchangeDone` moves to Loading (or directly to Full if no LSAs need requesting).
- `LoadingDone` moves to Full.
- `AdjOK` is re-evaluated whenever the DR election changes, because the set of neighbours that should be adjacent depends on who is DR/BDR. If the predicate flips from "should adjacency" to "should not", the NSM drops back to 2-Way and clears the retransmit/request lists.
- `SeqNumberMismatch` and `BadLSReq` both restart the synchronisation from ExStart, which discards the partial exchange.
- `InactivityTimer` fires when no Hello has been seen for `RouterDeadInterval` seconds; the neighbour goes to Down.
- `KillNbr` and `LLDown` are external triggers (admin action, interface down) that unconditionally go to Down.

**The `should_adj` predicate** (RFC 2328 §10.4): On point-to-point, point-to-multipoint, and virtual links, always yes. On broadcast or NBMA, adjacency is formed only if (a) the local router is DR or BDR, or (b) the neighbour is DR or BDR. This is the architectural reason for the DR: it cuts adjacency count on a LAN from `N*(N-1)/2` to `2*(N-1)`.

### 5d. Database Synchronisation (ExStart → Exchange → Loading → Full)

**ExStart (RFC 2328 §10.8).** On entering ExStart, the router schedules DD packets with I=1, M=1, MS=1, an initial DD Sequence Number, and no LSA headers. On every DD received, compare Router IDs:
- If peer's Router ID > ours, peer is master; clear MS, adopt the peer's DD Sequence Number, fire `NegotiationDone`.
- If ours > peer's, we are master; keep MS set and increment the DD Sequence Number.

**Exchange (RFC 2328 §10.8, §10.3).** The master drives the exchange, incrementing the DD Sequence Number for each new packet and retransmitting on a timer if the slave does not respond. The slave echoes the master's sequence number. Each side walks its LSDB (filtered by area scope, stub/NSSA rules, and MaxAge exclusion) and sends LSA headers in batches. As each DD arrives, the receiver compares each LSA header to its own LSDB:
- If the header's `(Type, LS ID, Advertising Router)` is not in the local LSDB, or the local copy is older (sequence / age comparison per RFC 2328 §13.1), add the header to the **LS Request list** for this neighbour.
- Otherwise, skip it.

When both sides have sent a DD with M=0 and the slave has acknowledged, `ExchangeDone` fires.

**Loading (RFC 2328 §10.9).** Send LS Request packets for the LS Request list in chunks, wait for LS Updates containing the requested LSAs, and remove entries from the list as they arrive. When the list is empty, `LoadingDone` fires and the NSM moves to Full.

**Failures.** Mismatched DD Sequence Numbers, unexpected `I` or `MS` flag transitions, or an LS Request for an LSA that is not in the partner's LSDB all fire `SeqNumberMismatch` or `BadLSReq`, which returns to ExStart.

### 5e. Flooding, Retransmission, and Acknowledgement

Flooding is driven by RFC 2328 §13 and is the most intricate part of the protocol after the state machines.

**On receipt of an LSA in an LS Update (RFC 2328 §13, the "flooding procedure"):**
1. Validate the LSA checksum and that LS Type is known; drop otherwise.
2. If the LSA is a Type 5 AS-External and the receiving area is stub or NSSA, drop.
3. Look up the LSA by key in the appropriate LSDB.
4. Compare the received LSA to the local copy per §13.1:
   - Received is strictly newer: install it, set the retransmit list of every other neighbour on interfaces where the LSA should be flooded, arrange for an ack back to the sender (direct or delayed per Table 19), and flood out other interfaces.
   - Received is strictly older: the sender is behind; send it our copy directly back.
   - Received is identical: acknowledge (or do nothing if already on the ack list).
5. Handle the special cases: self-originated LSAs received from the network (purge or re-originate with higher sequence), MaxAge purges, and Flooded-From suppression on broadcast interfaces.

**Flooding out other interfaces.** For each interface `oi` (excluding the one we received on):
- Walk the neighbours of `oi`. For neighbours in Exchange or Loading, check the neighbour's database summary and LS Request lists to avoid redundant flooding. For neighbours in Full, queue the LSA on the retransmit list.
- On broadcast and NBMA interfaces where this router is DR or BDR, send the LS Update to `AllSPFRouters`. On broadcast interfaces where this router is DROther, send only to `AllDRouters` (i.e. DR and BDR). Point-to-point, point-to-multipoint, and NBMA use unicast (or multi-unicast) per RFC 2328 §13.3 Table 19.

**Retransmission.** Each neighbour has a **retransmit list** of LSAs that have been sent but not yet acknowledged. A per-neighbour timer fires every `RxmtInterval` seconds; on each tick, every LSA on the list is resent. Acknowledgements remove LSAs from the list. Neighbours transitioning out of Full empty the list.

**Acknowledgements.** RFC 2328 §13.5 Table 19 specifies when to send a direct ack (immediately, unicast) versus a delayed ack (coalesced onto the interface's delayed-ack list, flushed on a per-interface timer around once per second). The rule summary:
- LSA from the DR that we are the BDR for: delay (the DR will time out).
- LSA was a duplicate of one already on our retransmit list: direct ack (to stop the retransmission promptly).
- LSA is newer than our copy and we forwarded it back out the receiving interface as part of flooding: suppress (the flood itself acts as an implicit ack).
- Otherwise: delay.

**Delayed-ack pacing.** The delayed ack interval is roughly half the `RxmtInterval`, with a default implementation delay (FRR uses on the order of tens of milliseconds). The flush is always shorter than `RxmtInterval` so the sender sees the ack before resending.

**Max-age purge flooding.** A purge (LSA with age = MaxAge) is flooded the same way as any other LSA, but receivers keep the purged LSA in the LSDB (still age MaxAge) until every interface's retransmit list has been cleared, and only then delete it. If you delete too eagerly, a late neighbour re-advertises an older copy that you will then treat as newer, and the stale LSA re-enters the network.

---

## 6. SPF, Areas, and Route Computation

### 6a. Intra-area SPF (Dijkstra)

RFC 2328 §16.1 defines the SPF computation as a Dijkstra run per area with the local router as the root. FRR's implementation, like every other production implementation, uses a two-phase approach:

**Phase 1 — router and network vertices.** The candidate list is a priority queue ordered by distance from the root. Seed it with the root (self). For each vertex extracted:
1. Mark as in the tree (SPT).
2. Look up its LSA in the area's LSDB.
3. For each link described by the LSA:
   - Router-LSA link type 1 (point-to-point to router): target is a router vertex keyed by the neighbour's Router ID.
   - Router-LSA link type 2 (transit link to DR): target is a network vertex keyed by the DR's interface address (which is the Link State ID of the corresponding Network-LSA).
   - Router-LSA link type 3 (stub network): record as a stub for Phase 2.
   - Router-LSA link type 4 (virtual link): same as type 1 but through the transit area.
   - Network-LSA: attached routers list gives the router vertices, each at cost 0 from the pseudonode.
4. For each target vertex, compute the distance and add it to the candidate list, updating or merging nexthops on tie. Equal-cost parents are merged to support ECMP.

The two-way check (RFC 2328 §16.1 paragraph near the end) is the key correctness rule: when visiting a target vertex via a link, the target's own LSA must include a reciprocal link back. Without this check, you can walk one direction across a broken adjacency. FRR implements this check by consulting the target's LSA and looking for an outgoing link that references the current vertex.

**Phase 2 — stub networks.** For every router vertex already in the tree, re-examine its Router-LSA's stub links. For each stub, compute the distance to the stub's prefix (tree distance + stub cost), record it in the area routing table, and retain the nexthops from the router vertex.

**Nexthop computation** is the hairiest piece. When a vertex is first reached, the nexthop is inherited from its parent. When a vertex is reached via a network vertex (a transit segment), the nexthop is the local interface used to reach that network. RFC 2328 §16.1.1 describes the algorithm precisely; the key subtlety is that a parent's nexthops are copied on equal-cost tie-break, not replaced.

### 6b. SPF Throttling

RFC 2328 does not specify a throttle algorithm, but every production implementation debounces SPF runs to avoid thrashing during topology churn. FRR implements exponential backoff with three knobs:
- **Initial delay** (default 0 ms in FRR, commonly 50 ms): The first SPF run after a triggering event waits this long to allow further events to arrive and be batched.
- **Hold time** (default 50 ms): If another trigger arrives within the current hold window, the next run waits this long.
- **Max hold time** (default 5000 ms): The hold time doubles on every consecutive trigger within the window, capped at the max.

When the window passes without a trigger, the hold time resets to the initial value. The implementation is a simple timer with a "triggered" flag and a double-on-tick counter.

### 6c. Inter-area Route Computation

RFC 2328 §16.2 defines the IA (Inter-Area) computation. It runs after Phase 1+2 and consults Type 3 (Summary-LSA-Network) and Type 4 (Summary-LSA-ASBR) LSAs in the local areas:
- For each Type 3 in a non-backbone area, if this router is not an ABR, use the summary's prefix with cost `abr_cost + summary_metric`, installed as an IA route.
- If this router is an ABR, Type 3 LSAs from non-backbone areas are **ignored**: ABRs only honour backbone summaries (to prevent loops). ABRs compute IA routes from the backbone's Type 3 LSAs.
- Type 4 LSAs are used to build a route to the ASBR, so Type 5 / Type 7 LSAs can be resolved later.

The origination side (at an ABR) is symmetric: for every intra-area prefix in area X, originate a Type 3 into every other area (subject to stub/NSSA filters and configured ranges).

### 6d. AS-External Route Computation

RFC 2328 §16.4 defines the external computation. For each Type 5 LSA (plus Type 7 after translation):
1. Resolve the ASBR that originated it: look up the ASBR router vertex in the intra-area or inter-area route table. If unreachable, skip.
2. Compute the effective cost based on the E-bit (metric type):
   - E1 (metric type 1, E-bit = 0): cost = `asbr_cost + external_metric`. Comparable to internal OSPF routes and used in standard tie-breaking.
   - E2 (metric type 2, E-bit = 1): cost = `external_metric` only. Used only to compare among E2 routes; any E1 wins over any E2 regardless of cost.
3. If the LSA has a non-zero Forwarding Address, use it instead of the ASBR as the nexthop target (and re-resolve through the routing table).
4. Install the route with path type external-1 or external-2, carrying the route tag from the LSA.

RFC 3101 augments this for NSSA Type 7 LSAs: they are processed only within their own area; the translated Type 5 (originated by the NSSA ABR) is the one the rest of the AS sees.

### 6e. ABR and ASBR Behaviour

An **ABR** is any router with interfaces in more than one area (at least one of which is the backbone). ABRs are identified by the B flag in Type 1 Router-LSA. Their responsibilities:
- For every intra-area prefix reachable in area X, originate a Type 3 Summary-LSA into every other attached area (subject to area ranges and filters).
- For every known ASBR in area X, originate a Type 4 into every other attached area.
- Translate NSSA Type 7 into backbone Type 5 (only one ABR per NSSA does the translation, elected per RFC 3101 §3.5).
- Participate in the Area 0 flooding domain for Type 3/4; this is what keeps inter-area routing loop-free.

An **ASBR** is any router that redistributes routes from outside the OSPF AS (BGP, static, connected, kernel, other IGPs). ASBRs are identified by the E flag in Router-LSA. Their responsibilities:
- For every redistributed prefix, originate a Type 5 AS-External-LSA (or Type 7 if in an NSSA) with the configured metric type, metric, forwarding address, and route tag.
- Suppress redistribution per route-map and per-protocol filters.
- Originate the "default route" as Type 5 (0.0.0.0/0) when `default-information originate` is configured.

A single router can be both ABR and ASBR; the two flags are independent.

### 6f. Stub, Totally Stubby, and NSSA Areas

**Stub area** (RFC 2328 §3.6). Type 5 and Type 4 LSAs are not flooded into a stub area. External routes are unreachable via the stub, except via a default route originated by the ABR(s). The E-bit in the Hellos must be clear in stub areas; mismatching E-bit is a common misconfiguration that silently blocks adjacency.

**Totally stubby** (vendor extension, sometimes called "no-summary"). Additionally suppresses Type 3 except for the default. Useful for spoke sites where all non-local traffic heads to a single ABR.

**NSSA** (RFC 3101). Permits Type 7 origination inside the NSSA so local redistribution is possible, but still blocks Type 5 from the rest of the AS. One ABR is elected as the translator and rewrites Type 7 into Type 5 for backbone flooding. NSSA ABRs typically also originate a default route into the NSSA. RFC 3101 §3.5 defines the translator election: the NSSA ABR with the highest Router ID wins, with sticky semantics analogous to DR election.

**NSSA P-bit and priority tiers** (RFC 3101 §2.5). Each Type 7 LSA carries a **P (Propagate) bit** in its Options field. When set, the translator ABR must convert the Type 7 into a Type 5 for injection into the backbone. When clear, the Type 7 stays local to the NSSA and is never propagated. This lets an NSSA advertise a route internally without announcing it outside — useful when the NSSA has its own ASBR but the rest of the AS already learns the same prefix another way. When the same external prefix is known via multiple paths, RFC 3101 §2.5 mandates the following preference order during route selection:

1. **Type 7 with P-bit set** — preferred over Type 5. The rationale is that a Type 7 originator is closer (inside the NSSA) than any Type 5 originator can be from the NSSA's perspective, so routing through the NSSA's own ASBR is more specific.
2. **Type 5** — used when no Type 7 with P=1 exists for the prefix.
3. **Type 7 with P-bit clear** — used last; only matters within the NSSA itself since these are not propagated.

A clean-room implementation must honour this ordering during AS-external route computation (§6d). A naive implementation that treats Type 7 and Type 5 as equivalent will pick the wrong nexthop when both are present, which is exactly the scenario RFC 3101 was written to fix.

**Virtual link** (RFC 2328 §15). When a router is in a non-backbone area but has no direct backbone adjacency, a virtual link through a transit area can be configured. It is modelled as a point-to-point link with Area 0.0.0.0, but the OSPF packets traverse the transit area as normal routed IP (TTL > 1). Virtual links complicate the Type 1 Router-LSA (they appear as a special link type) and are rarely needed in modern deployments.

### 6g. Route Installation

After SPF, IA, and external computations complete, the router has a new routing table. Installation is the job of the FIB manager (zebra in FRR, `sysrib` in ze). Each installed route carries:
- The destination prefix.
- The metric (used for intra-protocol comparison).
- An administrative distance (called protocol preference in some systems): by convention 110 for OSPF intra-area/inter-area, 110 or 150 for OSPF external (vendor-dependent).
- A path type marker (intra-area, inter-area, external-1, external-2).
- An ECMP nexthop set. OSPF naturally produces ECMP when Dijkstra finds equal-cost parents.

Between SPF runs, the new routing table replaces the old; changes are computed as a diff and handed to the FIB. Removed routes are withdrawn; changed routes are modified; new routes are added. The FIB is not reloaded from scratch.

---

## 7. Network Types and Interface Model

OSPF's concept of a network type governs how Hellos are addressed, whether a DR is elected, how neighbours are discovered, and how LSAs are flooded on the segment. FRR supports six primary network types plus the loopback pseudo-type:

| Network type | DR? | Neighbour discovery | Hello addressing | Use case |
|--------------|-----|---------------------|------------------|----------|
| Broadcast | Yes | Automatic via multicast | `AllSPFRouters` | Ethernet LAN |
| Point-to-Point | No | Automatic via multicast | `AllSPFRouters` | Serial, GRE, MPLS LSPs |
| Point-to-Multipoint | No | Automatic via multicast (broadcast variant) or explicit (non-broadcast variant) | Multicast or unicast | Hub-and-spoke DMVPN, ATM PVCs |
| NBMA | Yes | Explicit static neighbour list | Unicast per neighbour | Frame Relay, X.25, large DMVPN |
| Virtual Link | No | Explicit static peer Router ID | Unicast, routed through transit area | Backbone repair |
| Loopback | No | None | — | Host routes for loopback addresses |

**Broadcast** is the default on Ethernet-like links. The DR and BDR election reduces adjacency count. Network-LSAs describe the segment.

**Point-to-point** is the default on PPP-like links. Every neighbour is fully adjacent; no DR, no Network-LSA, no need for multicast groups beyond discovery.

**Point-to-multipoint** is a hybrid: the underlying medium is multi-access but the semantics are that every pair of routers uses a direct adjacency (as if each pair were a point-to-point link), with no DR. Hellos are multicast on the "broadcast" variant and unicast on the "non-broadcast" variant. This is heavy in adjacencies but simpler than NBMA.

**NBMA** predates point-to-multipoint. The link is explicitly configured with a static neighbour list, and a DR is elected (useful when the medium has no multicast). Rarely used today outside legacy Frame Relay.

**Virtual links** are synthetic point-to-point links across a transit area. They are needed when an area has no direct attachment to Area 0. The OSPF packets traverse the transit area as normal routed IPv4 with a TTL large enough to reach the far end. Virtual links inherit their authentication from the transit area, not from the virtual area.

**Loopback** interfaces are advertised as stub host links in the Router-LSA (with mask 255.255.255.255) so other routers can reach the loopback address but no OSPF protocol traffic is ever sent on the loopback.

### Per-interface Configurables

Every OSPF-enabled interface carries a cluster of knobs:

- **Hello interval** (default 10 s on broadcast/P2P, 30 s on NBMA/P2MP).
- **Router dead interval** (default 4× HelloInterval).
- **Retransmit interval** `RxmtInterval` (default 5 s).
- **Transmit delay** `InfTransDelay` (default 1 s). Added to LS Age on transmission.
- **Cost** (derived from bandwidth by default; overridable).
- **Router priority** (default 1; 0 disqualifies from DR election).
- **Network type** (as above; overridable).
- **MTU** (learned from the interface; used in the DD MTU check).
- **MTU-ignore** (bypass the DD MTU check).
- **Passive** (suppress Hellos, still advertise as a stub link in the Router-LSA).
- **Authentication mode and key(s)**.
- **BFD enable** (delegate fast failure detection to BFD).
- **LDP sync enable** (delay cost convergence until LDP is ready on MPLS links).
- **Database filter** (rarely used; suppress flooding out this interface).

These align with RFC 2328 Appendix C's list of configurable architectural constants plus the per-link extensions from later RFCs.

---

## 8. Authentication

### AuType Values

RFC 2328 §D.3 defines three authentication types carried in the OSPFv2 common header:

- **AuType 0 — Null.** No authentication. The 8-byte authentication field is zero.
- **AuType 1 — Simple Password.** The 8-byte field holds a cleartext password. Provides no real security; only useful to stop accidental misconfiguration.
- **AuType 2 — Cryptographic (MD5-based).** The 8-byte field becomes a structure: `(KeyID: 1, AuthDataLen: 1, CryptoSeq: 4)`. The MD5 digest is appended **after** the OSPF packet body, not in the header. The digest is 16 bytes and includes the key material.

RFC 5709 and RFC 7474 add **keyed HMAC-SHA** trailer authentication, reusing AuType 2 but allowing SHA-1, SHA-256, SHA-384, and SHA-512 digests. The digest length is the algorithm's output length (20–64 bytes). RFC 7474 makes the algorithm selection explicit through key-management and adds protection against replay and rollback attacks.

OSPFv3 deprecates in-header authentication entirely (RFC 5340 §2.5). Originally it relied on IPsec AH/ESP (RFC 4552), which proved unwieldy. RFC 7166 introduces the **OSPFv3 Authentication Trailer**, similar in spirit to OSPFv2's trailer: an optional trailer TLV is appended to each OSPFv3 packet and carries a keyed cryptographic digest.

### What FRR Supports

FRR's ospfd supports AuType 0, 1, 2 (MD5 legacy), and the RFC 5709/7474 HMAC-SHA trailer. Keys are configured per interface or per area; per-interface takes precedence. FRR implements anti-replay via the `CryptoSeq` field: each sender maintains a monotonic sequence number (commonly derived from `time()` or a persistent counter), and each receiver tracks the last-seen sequence per (neighbour, key-id) and rejects any packet with a smaller number.

FRR's ospf6d supports RFC 7166 trailers with HMAC-SHA-1/256/384/512 (implemented in `ospf6_auth_trailer.c`).

### Implementation Notes

- The common-header checksum is **zeroed** for AuType 2. The digest is computed over the packet including the zeroed checksum and a pseudo-key-padded-to-length as the authentication data. The receiver repeats the computation with its stored key and compares.
- The OSPF packet length includes only the header + body, not the trailer. The receiver must read the full datagram and take the trailing bytes as the digest.
- **Key rollover.** RFC 5709 and RFC 7474 describe "key chains" where multiple keys have overlapping validity periods. During rollover, the sender uses the primary key but the receiver accepts all currently-valid keys. This avoids simultaneous-reconfiguration requirements.
- **Backwards compatibility.** Mixed AuType 2 and RFC 7474 deployments need careful key management: both use the same AuType value but different trailer lengths and algorithms.
- **Do not roll your own HMAC.** Use the language's well-tested implementation (Go's `crypto/hmac`, `crypto/md5`, `crypto/sha1`, `crypto/sha256`, etc.).

---

## 9. Concurrency and I/O Model

### FRR's Model

FRR is single-threaded and event-driven. All of ospfd runs on one OS thread with a libfrr event loop (`select`/`epoll`-based) that multiplexes:
- Raw socket reads for OSPF protocol 89 datagrams.
- Socket writes for outgoing packets (write-readiness callbacks).
- Per-neighbour and per-interface timers (hello send, dead interval, DD retransmit, LS Req retransmit, LS retransmit, delayed ack, SPF delay, LSA refresh, MaxAge walker, ABR re-origination, ASBR redistribute).
- zclient IPC with zebra for router-ID, interface status, address changes, and route install/withdraw.
- VTY (CLI) and SNMP (if compiled in).

The single-threaded model is the classic advantage-disadvantage pair: simple reasoning about state (no locks) but no multi-core scaling. For a small-to-medium OSPF deployment (a few hundred neighbours, tens of thousands of LSAs), a single thread is plenty. For very large deployments, lock-free partitioning by area is the usual answer.

### Recommended Model for ze

ze is Go-native, so goroutines are cheap and idiomatic. Two reasonable models:

**Model A — Mirror FRR.** One goroutine owns the OSPF instance. All events (packet in, timer fire, config change, zebra update) are channelled to it. The goroutine runs a select loop. This is easy to reason about and matches FRR's semantics. Inside the goroutine, no locking is needed. It scales as far as one core.

**Model B — Multi-goroutine with per-domain locks.** Split by concern:
- One goroutine per OSPF-enabled interface handles Hello send, Hello receive, and the ISM.
- One goroutine per neighbour handles the NSM, DD exchange, retransmission, and ack.
- One goroutine for the LSDB (channel-serialised, like an actor).
- One goroutine for SPF (triggered, debounced).
- One goroutine for zebra/sysrib integration.

Model B is closer to ze's BGP reactor pattern and interacts naturally with ze's plugin event bus. It has more surface area for races but with disciplined ownership (LSDB is only written from its own goroutine, neighbours own their own retransmit lists, etc.) the locking remains manageable.

**Recommendation:** Start with Model A for the first pass. Once the protocol is correct and tested, revisit Model B only if profiling shows a single goroutine cannot keep up. Most OSPF deployments will be saturated by the network, not by the CPU.

### Synchronisation Points

Regardless of the concurrency model, these are the choke points to design around:

- **LSDB writes.** Install/replace/delete of LSAs. Must be serialised; reads (for SPF, CLI, flooding) can be concurrent.
- **SPF.** Runs over a consistent LSDB snapshot. Under Model A this is automatic. Under Model B either snapshot the LSDB before running SPF (copy-on-write) or take a read lock for the duration of the run.
- **Retransmit lists.** Owned by a neighbour. Writes on LSA install, new neighbour, ack receipt, NSM transition.
- **Interface TX queue.** Ordered FIFO per interface; the ISM and NSM both feed packets onto it.

A practical tip: do not block zebra/sysrib callbacks on OSPF internal work. Accept the event into a channel and process asynchronously, so slow SPF runs do not back up the zebra socket.

---

## 10. Configuration Shape

ze uses YANG for configuration. The following schema is a starting point for a first-pass OSPFv2 module. It draws on RFC 9129 (the IETF OSPF YANG module) but is simplified for the subset described in this guide.

```yang
module ze-ospf-conf {
  namespace "http://ze.example/yang/ospf";
  prefix "ospf";

  container ospf {
    description "OSPFv2 configuration for ze.";

    leaf enabled { type boolean; default "false"; }

    leaf router-id {
      type string;  // dotted-quad
      description "Router ID. If omitted, derive from a loopback interface.";
    }

    leaf reference-bandwidth {
      type uint32;
      units "Mbps";
      default "100000";
      description "Auto-cost reference bandwidth.";
    }

    leaf maximum-paths {
      type uint8;
      default "8";
      description "Maximum ECMP paths per prefix.";
    }

    container timers {
      leaf spf-delay-ms       { type uint32; default "50"; }
      leaf spf-initial-hold-ms { type uint32; default "200"; }
      leaf spf-max-hold-ms     { type uint32; default "5000"; }
      leaf min-ls-interval-ms  { type uint32; default "5000"; }
      leaf min-ls-arrival-ms   { type uint32; default "1000"; }
    }

    leaf log-adjacency-changes {
      type enumeration {
        enum none;
        enum summary;
        enum detail;
      }
      default "summary";
    }

    list area {
      key "area-id";
      leaf area-id { type string; description "Dotted-quad area ID."; }

      leaf type {
        type enumeration {
          enum normal;
          enum stub;
          enum totally-stubby;
          enum nssa;
          enum totally-nssa;
        }
        default "normal";
      }

      leaf default-cost {
        type uint32;
        description "Cost of the default summary originated into stub/NSSA areas.";
      }

      list range {
        key "prefix";
        leaf prefix    { type string; description "CIDR, e.g. 10.0.0.0/16"; }
        leaf advertise { type boolean; default "true"; }
        leaf cost      { type uint32; }
      }

      container authentication {
        leaf type {
          type enumeration {
            enum none;
            enum simple;
            enum md5;
            enum hmac-sha1;
            enum hmac-sha256;
            enum hmac-sha512;
          }
          default "none";
        }
      }
    }

    list interface {
      key "name";
      leaf name    { type string; description "OS interface name."; }
      leaf area-id { type string; description "Area this interface belongs to."; }
      leaf enabled { type boolean; default "true"; }

      leaf network-type {
        type enumeration {
          enum broadcast;
          enum point-to-point;
          enum point-to-multipoint;
          enum nbma;
          enum loopback;
        }
      }

      leaf cost            { type uint16 { range "1..65535"; } }
      leaf priority        { type uint8  { range "0..255"; }  default "1"; }
      leaf hello-interval  { type uint16 { range "1..65535"; } default "10"; }
      leaf dead-interval   { type uint16 { range "1..65535"; } default "40"; }
      leaf retransmit-interval { type uint16 default "5"; }
      leaf transmit-delay  { type uint16 default "1"; }
      leaf passive         { type boolean default "false"; }
      leaf mtu-ignore      { type boolean default "false"; }

      container authentication {
        leaf type {
          type enumeration {
            enum inherit;
            enum none;
            enum simple;
            enum md5;
            enum hmac-sha1;
            enum hmac-sha256;
            enum hmac-sha512;
          }
          default "inherit";
        }
        leaf key-id  { type uint8; }
        leaf key     { type string; description "Key material, bound from keychain."; }
      }

      leaf bfd-enabled { type boolean default "false"; }

      list nbma-neighbor {
        key "address";
        leaf address  { type string; description "Static neighbour IP for NBMA."; }
        leaf priority { type uint8 default "0"; }
      }
    }

    list redistribute {
      key "source";
      leaf source {
        type enumeration {
          enum connected;
          enum static;
          enum kernel;
          enum bgp;
          enum isis;
          enum rip;
        }
      }
      leaf metric      { type uint32 default "20"; }
      leaf metric-type { type enumeration { enum type-1; enum type-2; } default "type-2"; }
      leaf tag         { type uint32 default "0"; }
      leaf route-map   { type string; }
      leaf always      { type boolean default "false"; }
    }

    container default-information {
      leaf originate  { type boolean default "false"; }
      leaf always     { type boolean default "false"; }
      leaf metric     { type uint32 default "1"; }
      leaf metric-type { type enumeration { enum type-1; enum type-2; } default "type-2"; }
      leaf route-map  { type string; }
    }
  }
}
```

### Configuration Decisions

- **Per-interface area binding** rather than FRR's legacy `network <prefix> area <id>` match. Per-interface is clearer, matches RFC 9129, and avoids a class of misconfigurations where a prefix inadvertently matches more interfaces than expected.
- **ECMP is enabled by default** with a cap of 8 paths. OSPF naturally produces ECMP; disabling it would be surprising.
- **Authentication is per-interface**, with an `inherit` option that picks up the area's setting. A first-pass implementation can skip area-level authentication and require explicit per-interface keys.
- **No TOS-based routing.** RFC 2328 §11 allows multiple metrics per link for different Types-of-Service, but this feature was deprecated decades ago and no modern implementation uses it.
- **No virtual-link configuration in the first pass.** Virtual links are an advanced feature that can be added in a follow-up once the backbone area is fully working.

---

## 11. Plugin Model and Code Organisation for ze

### File Organisation

```
internal/component/ospf/
├── ospf.go                 # Plugin entry point, ze integration
├── config.go               # YANG config parse and apply
├── server.go               # OSPF instance orchestration
├── instance.go             # Struct ospf: process, areas, timers
├── area.go                 # Area management, per-area LSDB, SPF trigger
├── interface/              # OSPF-enabled interface + ISM
│   ├── iface.go
│   ├── ism.go
│   ├── hello.go
│   └── election.go         # DR/BDR election
├── neighbor/               # Neighbour state + NSM
│   ├── neighbor.go
│   ├── nsm.go
│   └── adjacency.go        # DD exchange, LS Request drain
├── packet/                 # Wire codec
│   ├── header.go           # 24-byte common header
│   ├── hello.go
│   ├── dbdesc.go
│   ├── lsreq.go
│   ├── lsupdate.go
│   ├── lsack.go
│   ├── lsa.go              # 20-byte LSA header
│   ├── lsa_router.go       # Type 1
│   ├── lsa_network.go      # Type 2
│   ├── lsa_summary.go      # Type 3 / 4
│   ├── lsa_external.go     # Type 5 / 7
│   ├── lsa_opaque.go       # Type 9 / 10 / 11
│   ├── checksum.go         # Fletcher-16, IP checksum
│   └── auth.go             # AuType 0/1/2, HMAC-SHA trailer
├── types/                  # Domain types
│   ├── routerid.go
│   ├── areaid.go
│   ├── lsakey.go
│   └── metric.go
├── lsdb/                   # LSDB store and aging
│   ├── lsdb.go
│   ├── aging.go            # MaxAge walker, refresh timer
│   └── origination.go      # Self-LSA origination
├── flooding/
│   └── flood.go            # Flooding procedure, retransmit, ack
├── spf/
│   ├── spf.go              # Dijkstra
│   ├── ia.go               # Inter-area computation
│   ├── ase.go              # External computation
│   └── route.go            # Route representation
├── sysrib/
│   └── install.go          # Integration with ze sysrib plugin
├── cli/                    # show commands, debug toggles
│   ├── show_neighbor.go
│   ├── show_database.go
│   ├── show_interface.go
│   ├── show_route.go
│   └── show_spf.go
├── schema/
│   ├── ze-ospf-conf.yang
│   ├── embed.go
│   └── register.go
└── register.go             # Plugin registration
```

### Dependencies and Integration Points

- **`iface` component.** Subscribe to interface up/down, address add/remove, MTU change. On up, if configured, bring the ISM from Down to Waiting or Point-to-Point. On down, tear down every neighbour on the interface and flush its retransmit lists.
- **`sysrib` component.** On SPF completion, compute the diff from the previous routing table and push adds/deletes. OSPF routes carry path type (intra, inter, external-1, external-2), metric, and an ECMP nexthop set.
- **`config` component.** YANG schema registration, config parse, and config-change handler.
- **`cli` component.** Command registration for the operational `show ip ospf ...` family and debug toggles.
- **`bus` / event stream** (if used). Publish adjacency state changes and SPF events for other subsystems to consume; consume interface and address events.

### Plugin Registration

OSPF should register as a ze plugin in `register.go` via `init()` so it is discovered at startup. The registration declares:
- YANG schema path.
- Config-change handler (reconfigure the OSPF instance).
- CLI commands.
- Dependencies on `iface` and `sysrib`.
- Start/stop hooks for the OSPF process goroutine.

### Why not share code with IS-IS?

Both OSPF and IS-IS are link-state protocols with Dijkstra at the core, and it is tempting to share a generic SPF engine. **Do not do this in the first pass.** The two protocols have subtle differences in graph construction (OSPF has network vertices for transit LANs; IS-IS uses pseudo-node LSPs), in LSA/LSP lookup keys, and in metric semantics. A shared abstraction will leak details back into both callers and make each harder to reason about. Once both implementations are working independently, a shared SPF module can be refactored out if the duplication turns out to be genuinely mechanical.

### Reference Architecture: FRR ospfd

FRR's OSPFv2 lives in `ospfd/` with roughly 80 files and 2 MB of C. The split is much finer than for BFD because OSPF is a much larger protocol. The core module map:

| Area | Files (representative) | Role |
|------|------------------------|------|
| Instance and lifecycle | `ospfd.c/h`, `ospf_main.c` | Top-level OSPF instance, config apply, daemon init |
| Interfaces | `ospf_interface.c/h`, `ospf_network.c/h` | OSPF-enabled interface abstraction, socket setup, multicast joins |
| Neighbours | `ospf_neighbor.c/h` | Neighbour struct and table |
| State machines | `ospf_ism.c/h`, `ospf_nsm.c/h` | Interface and Neighbor state machines as independent engines |
| Packet codec | `ospf_packet.c/h` (117 kB, monolithic) | All five packet types encode/decode/dispatch in one file |
| LSA types | `ospf_lsa.c/h` (112 kB) | LSA common header plus every LSA-type body in one file |
| LSDB | `ospf_lsdb.c/h` | Per-area LSDB, refcounting |
| Flooding | `ospf_flood.c/h` | The §13 procedure, retransmit lists, delayed ack |
| SPF | `ospf_spf.c/h` | Dijkstra |
| Route computation | `ospf_route.c/h`, `ospf_abr.c/h`, `ospf_ia.c/h`, `ospf_asbr.c/h`, `ospf_ase.c/h` | Intra-area, inter-area, ASBR-summary, AS-external — each its own file |
| Authentication | `ospf_auth.c/h` | RFC 5709 / RFC 7474 trailer, separate from the packet codec |
| Zebra glue | `ospf_zebra.c/h` | Route install/withdraw, interface events, redistribution |
| Legacy CLI | `ospf_vty.c` (365 kB, the biggest single file) | Hand-written VTY commands |
| Extensions | `ospf_opaque.c`, `ospf_ri.c`, `ospf_te.c`, `ospf_ext.c`, `ospf_sr.c`, `ospf_gr.c`, `ospf_gr_helper.c`, `ospf_bfd.c`, `ospf_ldp_sync.c`, `ospf_ti_lfa.c` | One file per opaque-LSA consumer or side-feature |
| External API | `ospf_api.c/h`, `ospf_apiserver.c/h` | Optional `ospfclient` socket API |
| SNMP | `ospf_snmp.c` (67 kB) | OSPF MIB (RFC 4750) |

Architectural takeaways:

1. **Horizontal split by protocol subsystem.** Each orthogonal concern — ISM, NSM, LSDB, flooding, SPF, routing table, ABR, ASBR, external — gets its own file. Eighty files is a lot but each is focused.
2. **Monolithic packet codec.** `ospf_packet.c` handles all five packet types. BIRD goes the other way (see below). FRR's choice localises auth and checksum code but produces a 117 kB file.
3. **One file per LSA type family in core, one file per opaque-LSA consumer for extensions.** Stable LSA types are centralised in `ospf_lsa.c`; experimental opaque-LSA consumers each live in their own file (`ospf_te.c`, `ospf_ri.c`, `ospf_ext.c`, `ospf_sr.c`). Good compromise.
4. **External features are file-per-feature.** Graceful restart, BFD integration, LDP sync, TI-LFA are each their own `ospf_<feature>.c`. Cleanly dropped in and cleanly excluded at build time.
5. **`ospf_vty.c` at 365 kB** reflects the cost of a hand-written CLI grammar with bespoke command handlers for every operational query. ze's YANG-derived CLI should be far smaller.
6. **OSPFv2 and OSPFv3 are separate daemons** (`ospfd` and `ospf6d`). Almost no code sharing. The two LSA registries and wire formats are different enough that unification was judged not worth the integration cost.

### Reference Architecture: BIRD 3.2 proto/ospf

BIRD's `proto/ospf/` is **18 files** — a fifth of FRR's size — and does something unusual: a single source tree implements **both** OSPFv2 and OSPFv3.

| File | Responsibility |
|------|----------------|
| `ospf.c` / `ospf.h` | Protocol instance, configuration apply, top-level dispatcher |
| `iface.c` | Interface state machine (RFC 2328 §9.3), DR election, socket setup |
| `neighbor.c` | Neighbor state machine (RFC 2328 §10.3) |
| `hello.c` | Hello send/receive — one file just for type 1 |
| `dbdes.c` | Database Description send/receive — type 2 |
| `lsreq.c` | Link State Request send/receive — type 3 |
| `lsupd.c` | Link State Update send/receive, the flooding core — type 4 |
| `lsack.c` | Link State Acknowledgment send/receive — type 5 |
| `packet.c` | Common packet header, auth, checksum, multiplexing to the per-type handlers |
| `topology.c` / `topology.h` | LSDB management, LSA origination for every type |
| `lsalib.c` / `lsalib.h` | LSA codec, Fletcher-16, sequence comparison, opaque-LSA type mapping between v2 and v3 |
| `rt.c` / `rt.h` | Dijkstra, intra-area, inter-area, external, NSSA — all in one file |
| `config.Y` | YACC grammar for OSPF config |

Architectural takeaways:

1. **Per-packet-type split.** One file per OSPF packet type (`hello.c`, `dbdes.c`, `lsreq.c`, `lsupd.c`, `lsack.c`) is BIRD's single most distinctive OSPF decision. Each packet type's send path, receive path, validation, and state updates live together. FRR has the opposite philosophy (one big `ospf_packet.c` for all five). BIRD's split improves locality — when debugging a DBDES master/slave bug you read one file — at the cost of having auth and checksum logic live in a separate shared file (`packet.c`).
2. **OSPFv2 and OSPFv3 share one binary.** A runtime flag on the protocol instance and a family of inline predicates (`ospf_is_v2`, `ospf_is_v3`, `ospf_is_ip4`, `ospf_is_ip6`) select behaviour. Packet structures are unions of v2 and v3 layouts. LSA type numbers are mapped between the two registries by a lookup table in `lsalib.c`. **This is a major architectural divergence from FRR.** It saves code (one implementation of the FSM, flooding, SPF, LSDB) at the cost of per-function branches on the version flag. BIRD's maintainers consider the trade worth it; FRR's disagree.
3. **Single LSDB for all areas.** `topology.c` keeps one hash table keyed on `(domain, LS ID, advertising router, type)`, where the `domain` field carries the area ID (or interface ID for link-scoped LSAs, or 0 for Type 5). FRR keeps per-area LSDBs. BIRD's design is simpler but requires every flooding/SPF operation to include a domain filter in the lookup.
4. **SPF + inter-area + external in one file.** `rt.c` (about 2300 lines) holds the intra-area Dijkstra, the inter-area summary-LSA computation, the AS-external computation, NSSA translation, ABR/ASBR selection, and RFC 4576 VPN loop prevention. Reading it gives a complete picture of the routing-table derivation in one sitting. FRR spreads the same work across `ospf_spf.c`, `ospf_route.c`, `ospf_ia.c`, `ospf_asbr.c`, `ospf_ase.c`, and `ospf_abr.c`.
5. **Graceful Restart (RFC 3623 for v2, RFC 5187 for v3), NSSA (RFC 3101), Router Information (RFC 7770), and authentication trailers (RFC 5709, RFC 7166) are implemented.** Traffic Engineering (RFC 3630), Segment Routing (RFC 8665), and the Extended Link/Prefix opaque LSAs (RFC 7684) are **not** — BIRD's OSPF is focused on core routing, not MPLS or SR deployments.
6. **BFD integration is a small hook** in `neighbor.c`, calling BIRD's in-process BFD and receiving state via a callback. No IPC, no plugin-registration protocol, because it is all one binary.

### BIRD OSPF: Implementation Detail

The six points above cover the architecture at a glance. This subsection
goes a level deeper into the parts of BIRD that have no equivalent in FRR
or bio-rd and that a ze reimplementer will want to understand. The
citations point into the BIRD 3.2 tree at `proto/ospf/`, `nest/`,
`filter/`, `conf/`, and `lib/`.

#### The `nest/` protocol framework: how a protocol plugs in

BIRD's single most important architectural idea is not OSPF-specific: it
is the **protocol/channel/table** separation in `nest/`. Every routing
protocol, including OSPF, integrates through the same three-layer
contract, and the contract is cleaner than anything FRR has. ze should
study this before designing its IS-IS or OSPF integration points.

The layers:

1. **`struct proto`** (`nest/protocol.h`, around line 51) is the base
   struct for every running protocol instance. OSPF's `struct ospf_proto`
   embeds it at offset 0. BIRD calls protocol hooks through the base:
   `start()`, `shutdown()`, `reconfigure()`, `show_stats()`,
   `rt_notify()`, `preexport()`. The base also carries debug flags, a
   name, a dedicated memory pool, and an event pointer for async
   dispatch.
2. **`struct channel`** (`nest/protocol.h` around line 613) is the
   binding between a protocol instance and a specific routing table. A
   protocol can have zero, one, or many channels. OSPF has one main
   channel per instance. A channel carries its own import filter, export
   filter, rate limits, and per-route counters. When OSPF computes a
   route, it calls `rte_update(channel, net, rte)` and the channel
   decides what to do: apply the filter, insert into the table, or drop.
3. **`struct rtable`** is the actual routing table (a patricia FIB). A
   single BIRD daemon can have many rtables, and the same OSPF process
   can have channels into several of them. This is how multi-instance
   and VRF are modelled.

When an external protocol (say BGP) pushes a route into the master
table, the table's journal replays the update to every subscriber. OSPF,
via its channel's `rt_notify` hook (`proto/ospf/ospf.c:372` registers
it), receives the replayed update and decides whether to originate an
AS-external LSA for it. The import side works the same way in reverse:
OSPF's computed routes are injected via `rte_update()` and other
channels (kernel syncer, BGP redistribute-ospf) receive them through
their own journals.

**Why this matters for ze**: ze's current model is plugin-per-protocol
with ad-hoc glue between components. Adopting the proto/channel/table
pattern, even informally, gives you:

- One contract for every protocol integration. A new protocol drops in
  by implementing `Start/Stop/Reconfigure/RouteNotify/PreExport`.
- Natural reconfiguration semantics: channels can be added, removed, or
  re-filtered without restarting the protocol instance.
- Natural multi-table support: the same OSPF instance can feed multiple
  route tables (think per-VRF).
- Filters live outside the protocol, so OSPF never sees rejected routes;
  the channel and the filter engine handle them.

The Go translation is straightforward:
`type Protocol interface { Start(); Stop(); Reconfigure(newCfg); RouteNotify(ch, net, rte); PreExport(ch, rte) bool }`
plus a `Channel` struct with filter slots. This is significantly cleaner
than threading imports and exports through ad-hoc component boundaries.

#### The LSA lifecycle state machine (from `topology.h` top comment)

BIRD's `proto/ospf/topology.h:48-113` documents an LSA entry state
machine that is not in any RFC and is genuinely novel. The reason it
exists: LSA origination and flushing are **asynchronous** because
MinLSInterval (5 s per `ospf.h:68`) and sequence number wraparound
force deferral. An operator can call `ospf_originate_lsa()` at any
moment but the actual origination may happen milliseconds or seconds
later, after the throttle window closes.

The state set, paraphrasing the header comment:

- **R** Regular: `lsa.age < MaxAge` and `lsa_body != NULL`. The normal
  installed, active LSA.
- **F** Flushing: `lsa.age == MaxAge` and `lsa_body != NULL`. The LSA
  has been marked for flush and is being flooded with age 3600 so
  neighbours remove it, but the body is still present locally for
  retransmission.
- **E** Empty: `lsa.age == MaxAge` and `lsa_body == NULL`. Flush is
  acknowledged; body is freed but the entry is kept to preserve
  `inst_time` and `lsa.sn` for a possible re-origination.
- **X** Non-existent. The entry is removed from the hash table.

Each of R, F, E is **doubled** by whether a next-LSA origination is
scheduled (`next_lsa_body != NULL`), giving a six-state machine with
the suffix `n` for "next scheduled": R, Rn, F, Fn, E, En. The header
comment at `topology.h:87-113` lists the transitions:

- `X -> R` new origination, immediate
- `R -> Rn`, `F -> Fn`, `E -> En` origination requested but postponed
- `Rn/Fn/En -> R` postponed LSA finally originated
- `R -> Fn` refresh with sequence wrap: the old entry is flushed **and**
  a replacement is scheduled
- `R -> F`, `Rn -> Fn` LSA age timeout without flush request
- `R/Rn/Fn -> F`, `En -> E` explicit flush
- `F -> E`, `Fn -> En` flush acknowledged
- `E -> X` real-age timeout, entry removed

The machinery is driven by `ospf_update_lsadb()` at `topology.c:1846+`,
which runs once per `ospf_disp()` tick (1 second by default per
`ospf.h:75`). The callers `ospf_originate_lsa()` and `ospf_flush_lsa()`
only **request** a state change; the tick handler applies it.

**Why this matters for ze**: bio-rd's guide handwaves LSP purge as "mark
age 0, flood, eventually delete". RFC 10589 and RFC 2328 also do not
fully specify the asynchronous parts. A real implementation that tries
to do LSA origination synchronously on every trigger will violate
MinLSInterval, flap LSAs, and lose acknowledgement-grace semantics. The
"R/F/E + next-scheduled" state machine is the concrete solution. Adopt
it almost verbatim for both LSP (IS-IS) and LSA (OSPF) handling,
replacing the pseudo-RFC "decrement lifetime and delete" with an
explicit state field.

#### LSA modes: not all LSAs are originated the same way

`topology.h:116-154` defines five LSA modes:

- `LSA_M_BASIC` the classic model. Explicit `ospf_originate_lsa()` and
  `ospf_flush_lsa()` calls. When the LSA changes, SPF is rescheduled.
  Router-LSAs and Network-LSAs use this mode. Also the mode used for
  LSAs received from neighbours.
- `LSA_M_EXPORT` originated as a consequence of a route being exported
  to OSPF (AS-external-LSAs for redistributed routes). Must be
  reoriginated on every channel feed; if not touched in a feed cycle,
  automatically flushed. SPF is **not** rescheduled because the LSA is
  caused by route export, not topology.
- `LSA_M_RTCALC` originated during a routing-table calculation
  (summary-LSAs). Must be requested on every RTCALC run; otherwise
  flushed at the end. Again SPF is not rescheduled because the LSA is
  an output of RTCALC, not an input.
- `LSA_M_EXPORT_STALE` and `LSA_M_RTCALC_STALE` transitional states for
  the "not touched in the current feed/calc" case. If the next cycle
  touches them, they go back to the live state; if not, they are
  flushed.

This mode distinction is subtle but critical. It encodes **who owns the
LSA lifetime**: the protocol itself (BASIC), the route export system
(EXPORT), or the SPF calculator (RTCALC). A ze implementation that
treats all LSAs the same will either refresh Summary-LSAs too
aggressively (performance hit) or not at all (routes linger after
failure).

The stale trick is particularly elegant: during a feed or calc, mark all
EXPORT/RTCALC entries as stale; as the feed/calc revisits each, clear
the stale flag; at the end, any still-stale entries are flushed. This
gives a correct garbage collect without tracking explicit "which LSAs
are still needed" lists.

#### The `top_graph` LSDB

`topology.h:157-168` defines the LSDB container:

- `pool *pool` memory pool for entries and their bodies
- `slab *hash_slab` slab allocator specifically for `top_hash_entry`
- `struct top_hash_entry **hash_table` the table itself, with chained
  collisions via `entry->next`
- `uint ospf2` the OSPFv2/v3 flag, because the LSDB implementation
  handles both including the LSA type mapping between them
- `uint hash_size, hash_order, hash_mask, hash_entries,
  hash_entries_min, hash_entries_max` dynamic sizing state

The table grows and shrinks based on load: when `hash_entries` exceeds
`hash_entries_max`, the table doubles; when it falls below
`hash_entries_min`, it halves. The growth and shrink thresholds use
HI_MARK (x4) and LO_MARK (/5) to prevent thrashing.

The hash **key** is the four-tuple `(domain, lsa_id, advertising_rtr,
type)`. The `domain` field encodes scope: area ID for area-scoped LSAs
(Router, Network, Summary), interface ID for link-scoped LSAs (Link-LSA
in v3), and 0 for AS-scoped LSAs (AS-External). This is the unified
LSDB trick mentioned in point 3 of the architectural takeaways above,
and now you can see the mechanism: the scope is just another field in
the key, so one hash table serves all three LSA scopes without
per-scope partitioning.

**ze implication**: a Go `map[LSAKey]*LSA` with
`type LSAKey struct { Domain uint32; ID uint32; RTR uint32; Type uint16 }`
gives the same effect. Go maps resize automatically so the explicit
grow/shrink logic is not needed. Per-area partitioning can still be
cleaner for Go because range iteration is expensive on large maps and
many operations are area-scoped anyway.

#### The `ort` / `orta` routing table design

`proto/ospf/rt.h:18-89` defines two structures:

- **`orta`** is the routing information: path type (`RTS_OSPF_*`),
  router-LSA option bits, `metric1`, `metric2`, `tag`, advertising
  router ID, back-pointer to the OSPF area, computed nexthop list, and
  the LSA this route came from. This is the value.
- **`ort`** is the FIB node: embeds an `orta` under `n`, plus
  `old_metric1/2/tag/rid/ea` for diff-based export, `lsa_id` for
  OSPFv3 LSA ID allocation, flags for `external_rte`, `area_net`,
  and `keep`, plus the `fib_node` for membership in the routing-table
  patricia tree.

The separation matters for two reasons:

1. **Stable UIDs for OSPFv3 LSA IDs.** The `fib_node.uid` provides a
   stable 32-bit identifier for the prefix, reused as the LSA ID for
   Summary-LSAs and External-LSAs. Even if the route is withdrawn and
   later re-added, the `ort` entry is kept (`keep = 1`) so the UID is
   preserved, so the LSA ID stays stable, so neighbours do not see a
   churned Summary-LSA. This is documented in the comment at
   `rt.h:60-73`.
2. **Diff-based export.** The `old_*` fields cache the last-advertised
   state. When `rt_sync()` runs, it compares `old_*` to the new `orta`
   and only pushes `rte_update()` for rows that actually changed. This
   is the OSPF equivalent of the "dirty flag" optimization and it is
   what keeps rt_sync fast on large LSDBs.

`rt.h:113-117` documents four types of nexthops: **gateway** (non-NULL
iface, real gateway IP), **device** (non-NULL iface, no gateway, direct
ARP/ND lookup), **dummy vlink** (NULL iface, NULL gateway, used for
virtual-link route tracking), and **configured stubnet** (NULL nexthops
entirely). The comment at `rt.h:120-127` states that vlink and stubnet
nexthops cannot mix with gateway/device nexthops in the same `nhs`
list, which is an invariant the C code enforces but a Go sum type would
express more tightly.

**ze implication**: the dirty-diff pattern is worth copying verbatim.
It is the difference between "rt_sync pushes 50k updates per SPF run"
and "rt_sync pushes only what changed". Write your route table with
`oldMetric/oldTag/...` fields per entry and diff during sync.

#### SPF vertex coloring

`topology.h:37-41` defines three vertex colors:

- `OUTSPF = 0` not yet seen by the SPF walk
- `CANDIDATE = 1` in the candidate (TENT) list
- `INSPF = 2` finalized in the SPT

Each `top_hash_entry` carries a `u8 color` field that is mutated during
SPF, then reset to `OUTSPF` at the start of the next run. This is the
standard Dijkstra coloring but it is worth noting because BIRD stores it
**on the LSA**, not in a separate vertex structure. This saves an
allocation (no separate `struct vertex`) at the cost of making the LSA
entry a load-bearing data structure for SPF.

The tradeoff is reasonable for C where allocations are expensive. In Go
it is usually cleaner to have a separate
`type Vertex struct { LSA *LSA; Distance uint32; Color int; Parents []*Vertex }`
and throw the vertex graph away after the run. The FRR IS-IS
`isis_vertex` pattern (cited in `isis-frr-reference.md` section 10) is
the same idea.

#### The `rt.h` invariants block

`rt.h:96-127` documents the invariants that SPF and route construction
rely on. These are worth internalizing because they are the contract a
reimplementer must preserve or everything breaks:

- Finalized nodes (`color == INSPF`) have `lsa.age < MaxAge` and
  `dist < LSINFINITY`.
- `nhs` (computed nexthops) is non-NULL unless the node is the
  calculating router itself or a configured stubnet.
- `oa->rtr` never contains the calculating router.
- Metrics cannot overflow because they are bounded to "a small multiple
  of LSINFINITY" (about 2 x 0xFFFFFF), well below the uint32 cap.
- Vlink nexthops are replaced during inter-area processing and removed
  during ABR route calculation, so they are invisible to the ASBR
  pre-selection and external route processing phases.

A ze implementation should state these invariants in doc comments and
test them with assertions in debug mode. They are the kind of invariant
that works until someone adds a feature and quietly breaks it.

#### Per-interface socket model

FRR opens one raw socket per protocol and demuxes by interface at the
application layer. BIRD opens **one socket per OSPF interface**,
configured with per-interface rx/tx hooks, multicast group memberships,
and RX buffer size (`proto/ospf/iface.c` around `ospf_sk_open()`). The
socket is registered in the main loop's fd set and the hook is called
directly from `poll()` when the fd is readable.

This is different in three consequential ways:

1. **Per-interface RX buffer sizing.** An interface with a 9000-byte MTU
   can have a matching RX buffer without oversizing for normal
   interfaces. `struct ospf_iface_patt` at `ospf.h:183` has `u16
   rx_buffer` precisely for this.
2. **Per-interface multicast joins are explicit.** When an interface
   leaves the DR state, it leaves the AllDRouters group; it does not
   demux by "am I the DR right now" at the application layer.
3. **No application-level demux.** Each hook already knows which
   interface fired it because the socket is tied to the interface. No
   `if (packet.ifindex == ...)` check.

**ze implication**: Go's `net.ListenPacket` with `SO_BINDTODEVICE` (or
the equivalent raw-socket interface bound to one ifindex) gives you the
same pattern. One goroutine per interface, reading from a dedicated
socket, is both simpler to reason about and faster on multi-core systems
than one shared listener.

#### The filter language

`filter/` implements a bytecode virtual machine for route policy. The
shape, from what the code reveals:

- **Grammar**: `filter/config.Y` (YACC) parses filter source into an
  AST of `struct f_inst` instruction nodes. Each node has an opcode, a
  payload (literal, reference, nested tree), and line information for
  error reporting.
- **Compilation**: a pass over the `f_inst` tree linearizes it into a
  `struct f_line` array of bytecode operations. String constants are
  interned, function addresses are resolved, and the tree is freed.
- **Execution**: `filter_state` is a thread-local struct containing a
  value stack, an instruction stack, the current route being evaluated,
  accept/reject flags, and exception masks. `f_run()` is the entry
  point; it walks the bytecode, pushes and pops values, and returns
  `F_ACCEPT`, `F_REJECT`, or an error.

The language itself has primitives for:

- Prefix matching: `if net ~ [192.0.2.0/24+]` (set containment,
  prefix-length ranges)
- Community matching: `if bgp_community ~ (65001, *)`
- AS-path regex: `if bgp_path ~ [= 100 * 200 =]`
- Route attribute read and write: `bgp_local_pref = 200`
- Control flow: `if/then/else`, `accept`, `reject`, explicit return
- Local variables: `local int x = 10`
- Function calls: user-defined functions plus built-ins like `length()`,
  `mask()`, `format()`, `ipa_class()`

OSPF itself does not script filters. It exposes route attributes
(`ospf_metric1`, `ospf_metric2`, `ospf_tag`) and the **operator** writes
filters that read those attributes to decide import and export policy.
The channel applies the filter automatically when the route crosses the
protocol/channel boundary.

**ze implication**: ze has its own filter subsystem that is different
in shape. Do not translate BIRD's bytecode VM to Go literally. Do take
two ideas: (a) the filter is a separate subsystem invoked by the
protocol/channel boundary, not embedded in the protocol code; (b)
filters are data, not code, so they can be reconfigured live without
restarting the protocol. BIRD's filter grammar itself is worth reading
as prior art for any ze policy DSL ambitions.

#### Graceful reconfiguration

BIRD's ability to swap configurations without restarting the process is
its second-most-famous feature after filters. The mechanism lives in
`conf/conf.c`:

1. `config_alloc()` parses a new config file into a separate
   `struct config *new_config`, independent of the currently active
   `struct config *config`.
2. `config_commit(new_config, reconfigure_type, timeout)` starts the
   swap. For each protocol, it calls the protocol's `reconfigure(new,
   old)` hook. The protocol can either accept the new config (return
   true) or reject it (return false, forcing a restart).
3. OSPF's reconfigure hook (`proto/ospf/ospf.c:494-509`) walks the new
   area list, the new interface list, and the new filters, and decides
   on a per-element basis whether the change is hot-swappable. Area
   address change: restart required. Timer change: hot-swappable. New
   interface: hot-swappable.
4. Accepted changes are applied immediately. Rejected protocols are
   shut down and restarted with the new config.
5. The old and new configs coexist briefly. BIRD uses `OBSREF`
   (obstructable reference) so an old config stays alive as long as any
   running protocol holds a pointer into it, even after the new config
   is committed. Once all references drop, the old config is freed.

**ze implication**: ze already has YANG with transactions and rollback,
which is the modern equivalent at the configuration layer. But BIRD's
per-protocol `reconfigure()` hook that decides granularly what is
hot-swappable is worth adopting: rather than "YANG validates, then
everything restarts", have each component opt into hot-swap per config
field. A change to `hello-interval` should never bounce the adjacency.

#### Event loop and timer primitives

`sysdep/unix/io.c` around line 2611 contains BIRD's main event loop.
The shape (paraphrased):

```
main_birdloop:
  for ever {
    socket_prepare()     # compute pollfd array from registered sockets
    poll()               # wait for fds or timer expiry
    run_socket_hooks()   # rx/tx callbacks on ready fds
    run_timer_hooks()    # expired timers from the heap
    run_event_hooks()    # deferred/async events from the queue
  }
```

Timers are stored in a heap indexed by expiry time (`lib/timer.h` around
line 24). Adding a timer is O(log n); next-fire lookup is O(1). OSPF
uses timers for hello (`hello_timer_hook`), dead detection (`inactim`),
retransmit (`dbdes_timer`, `lsrq_timer`, `lsrt_timer`), and the
per-protocol dispatcher (`disp_timer` firing every `tick` seconds,
default 1).

Events are a separate queue (`lib/event.h` around line 19) for
**deferred** callbacks: "run this on the next loop iteration". OSPF
uses events for "wake up and recheck state" after a state change that
needs to propagate through multiple subsystems. Events are atomically
enqueued, so a timer callback can safely schedule more work without
re-entering the caller.

The critical property: **OSPF code cannot block**. No sleeps, no
waitgroups, no synchronous RPCs. Every operation that cannot complete
immediately must be encoded as a timer or an event. This is the same
discipline FRR requires (covered in `isis-frr-reference.md` section 3)
and the same discipline bio-rd's goroutine model sidesteps by having the
runtime schedule for you.

**ze implication**: if ze picks a goroutine-per-concern model for OSPF
or IS-IS, you get the non-blocking property for free, at the cost of
channel-synchronization overhead. If ze picks a
single-goroutine-per-protocol model with an event queue, you get BIRD's
semantics: no blocking, fast dispatch, easy reasoning, but you must
structure every state change as an event. The hybrid (one goroutine
per interface for I/O, one central actor for the LSDB) is probably the
right choice for ze and maps neatly onto BIRD's socket-per-iface plus
central protocol loop design.

#### Configurable constants worth noting

From `proto/ospf/ospf.h:65-79`:

| Name | Value | Meaning |
|------|------:|---------|
| `LSREFRESHTIME` | 1800 s | Re-originate self LSAs every 30 minutes |
| `MINLSINTERVAL` | 5 s | Minimum time between origination and reorigination |
| `MINLSARRIVAL` | 1 s | Minimum time between accepting two updates of the same LSA |
| `LSINFINITY` | 0xFFFFFF | Max link metric (24-bit) |
| `OSPF_DEFAULT_TICK` | 1 s | Dispatcher interval |
| `OSPF_DEFAULT_STUB_COST` | 1000 | Default cost for stub-area default route |
| `OSPF_DEFAULT_ECMP_LIMIT` | 16 | Max ECMP next hops |
| `OSPF_DEFAULT_GR_TIME` | 120 s | Graceful restart grace period |
| `OSPF_DEFAULT_TRANSINT` | 40 s | NSSA translator stability interval |

These are BIRD defaults. RFC 2328 appendix C specifies several of them
as SHOULD-values; BIRD uses the SHOULDs except `OSPF_DEFAULT_GR_TIME`
which is a BIRD choice (RFC 3623 leaves it to the operator). Worth
adopting verbatim unless you have a specific reason to diverge.

### Lessons for ze

- **From BIRD: per-packet-type file split.** ze's `packet/` subtree should have one file per packet type (`hello.go`, `dbdesc.go`, `lsreq.go`, `lsupdate.go`, `lsack.go`) plus a common `header.go` and `auth.go`. The §11 file layout already reflects this. BIRD confirms the choice.
- **From BIRD: the `nest/` protocol/channel/table pattern.** The single biggest reusable idea in BIRD. Every protocol plugs in through the same three-layer contract. See the "nest/ protocol framework" subsection above. ze should adopt the Go-ified version as the common integration shape for OSPF, IS-IS, and any future link-state protocol.
- **From BIRD: the R/F/E+next LSA lifecycle state machine.** Documented in `topology.h` top comment. Handles MinLSInterval throttle, sequence wrap, async flush, and refresh correctly. Replace the naive "decrement-and-delete" pattern with this state machine for both LSP and LSA handling.
- **From BIRD: LSA modes (BASIC / EXPORT / RTCALC) with stale-and-sweep GC.** The owner-of-lifetime distinction is subtle but correct. Adopt verbatim: Router-LSA is BASIC (protocol owns it), Summary-LSA is RTCALC (SPF owns it), AS-External-LSA is EXPORT (route feed owns it).
- **From BIRD: diff-based route export via `old_*` fields.** Keep `oldMetric/oldTag/...` on every route table entry. Skip `rte_update()` when the diff is empty. This is free performance.
- **From BIRD: per-interface socket with `SO_BINDTODEVICE`.** One goroutine per interface, one bound socket, no application-level ifindex demux. Cleaner and faster than the shared-listener model.
- **From BIRD: granular `reconfigure()` hook that opts into hot-swap per field.** Changing a hello interval should not reset the adjacency. ze's YANG transaction layer validates; each component decides what is hot-swappable.
- **From BIRD: one binary for OSPFv2 and OSPFv3? Probably not.** The appeal is real but the cost (branch density, union types, version flags threaded through every function) is high. Go is not great at tagged-union polymorphism. Stick with separate components and revisit only if duplication becomes painful.
- **From FRR: file-per-opaque-LSA-consumer.** When ze gets around to TE, SR, or Graceful Restart, each should live in its own file so they can be compiled out or selectively enabled.
- **From FRR: separate LSDBs per area.** BIRD's unified LSDB works for BIRD because C struct layout makes the domain field cheap to filter on. Go's more relational `map[Key]*LSA` per area is cleaner and gives free isolation for area-scoped operations.
- **From FRR: separate component for OSPFv3.** When ze adds v3, make it a sibling plugin, not an intrusive modification of the v2 plugin. Borrow patterns, not code.
- **From neither: the SPF monolith vs SPF scatter.** BIRD's single `rt.c` is readable but hard to test piecewise; FRR's scatter is testable but hard to grok holistically. ze's §11 layout (`spf/spf.go`, `spf/ia.go`, `spf/ase.go`, `spf/route.go`) splits the phases so each has its own unit tests — a third path that preserves both the readability and the testability.

---

## 12. Testing Strategy

OSPF is a correctness-sensitive protocol. Bugs in flooding, checksum, or SPF manifest as silent routing black-holes that are extremely hard to diagnose in production. Test relentlessly.

### Unit Tests

1. **Packet codec tests.** For each packet type and each LSA type, encode a known struct to bytes and compare to a hex fixture; decode the hex fixture and compare to a known struct; round-trip (encode → decode → encode) and compare bytes. Use real captures from Wireshark or `tcpdump` for the fixtures.
2. **Checksum tests.** Fletcher-16 (RFC 905 Annex C) has published test vectors; use them. IP checksum has a well-known algorithm but the zero-field-substitution rule is error-prone; test with a packet that contains the checksum position in a non-zero place.
3. **LSA comparison tests.** RFC 2328 §13.1 has a precise ordering (sequence, age, checksum). Write tests for every branch including sequence number wraparound, MaxAge, and age-within-MaxAgeDiff.
4. **ISM tests.** Mock interface, feed events, assert state transitions. Test DR election, including the sticky rule.
5. **NSM tests.** Mock neighbour, feed events, assert state transitions. Test each failure event (SeqNumberMismatch, BadLSReq, InactivityTimer).
6. **Flooding tests.** Mock LSDB and interfaces. Receive an LSA; verify retransmit lists are updated correctly on each interface. Receive an older LSA; verify the local copy is pushed back at the sender. Receive a duplicate; verify nothing happens.
7. **SPF tests.** Hand-build an LSDB with a known expected shortest-path tree; run Dijkstra; compare. Cover ECMP, stub networks, and the two-way check.
8. **IA and external tests.** Similarly, build LSDBs with Type 3/4/5/7 and verify the resulting routing table entries.
9. **Authentication tests.** Sign a packet; verify the signature; flip a bit; verify rejection.

### Fuzz Tests

Run the packet decoder against a fuzz corpus. Any decode of a malformed packet must fail gracefully (no panic, no out-of-bounds read). Seed the corpus with known-good captures and with deliberately corrupted versions.

### Integration Tests (`.ci`)

ze's functional test runner should spin up a pair of instances connected via an in-memory link abstraction (or via a Linux bridge/namespace for a more realistic test) and verify:
1. **Adjacency formation on a point-to-point link.** Hello exchange, NSM transition through ExStart, Exchange, Loading, Full. Time-bounded (e.g., 5× HelloInterval).
2. **Adjacency formation on a broadcast link with three routers.** DR election, BDR election, 2-Way among DROthers, Full between DROthers and DR/BDR.
3. **LSA flooding.** Change a link cost on router A; watch the Type 1 re-originate; watch B and C receive it; check their LSDBs converge.
4. **SPF convergence.** Line topology A-B-C. Install a prefix on A. Verify C's routing table has a route with nexthop B.
5. **ABR behaviour.** Two areas connected by an ABR. Originate a prefix in area 1; verify it appears as a Type 3 in area 2; verify routers in area 2 can reach it.
6. **ASBR / redistribution.** Redistribute a static route; verify the Type 5 LSA is originated and flooded; verify the external route is installed on other routers.
7. **Stub area.** Configure an area as stub; verify Type 5 LSAs are not flooded in; verify the default summary is originated by the ABR.
8. **NSSA.** Configure an area as NSSA; redistribute a route inside the NSSA; verify the Type 7 LSA is originated, not Type 5; verify the translator ABR converts it to Type 5 for the backbone.
9. **Failover.** Bring an interface down; verify adjacencies time out; verify SPF re-runs; verify the new routes converge.
10. **Authentication.** Configure MD5 and HMAC-SHA-256; verify adjacencies still form; verify a packet with a wrong key is rejected.

### Interop Tests

The gold standard is interop with FRR itself. Set up a Linux network namespace running FRR ospfd and a matching namespace running ze. Verify:
- Adjacency forms and goes Full with each of the major network types.
- LSDBs converge (every LSA in FRR's `show ip ospf database` is in ze's database and vice versa, with matching sequence numbers).
- Routing tables converge.
- Failures recover correctly on both sides.

Second-tier interop: Cisco IOS or Juniper Junos in a lab (or GNS3/EVE-NG). These validate against two independently-written implementations, catching bugs that a ze-vs-FRR test would share.

### Regression Tests

Every bug found in development, interop, or production gets its own `.ci` test. Keep a list of known limitations (features not yet implemented) and mark the corresponding tests as skipped with an RFC reference.

---

## 13. Known Hard Problems and Traps

OSPF has a set of well-known pitfalls. Most have caught every implementation at least once; address them early.

### 1. Fletcher-16 Checksum

RFC 1008 (via ISO 8473) defines the Fletcher-16 checksum used for LSA headers. The checksum is computed over the LSA **excluding the age field** (bytes 0–1) but **including the checksum position itself**. This creates a bootstrapping problem: the checksum depends on the bytes at the checksum field, which are the checksum. RFC 1008 Annex C specifies an adjustment algorithm: compute with the checksum field zeroed, then adjust the two halves so that the re-computed checksum matches.

**Test thoroughly.** Use RFC 905 or ISO 8473 test vectors. Verify both encode and decode. A common bug is encoding correctly but checking incorrectly on the receive side (or vice versa), so self-interop passes but cross-interop fails.

### 2. Sequence Number Comparison

RFC 2328 §13.1 specifies the freshness comparison for LSAs: compare sequence numbers as signed 32-bit integers, but with the special case that `MaxSequenceNumber` (0x7FFFFFFF) is considered newest unless another MaxSequenceNumber LSA with age MaxAge is also present. The comparison is intentional and subtle; a naive "higher is newer" implementation fails at wraparound.

**Test with sequence `0x80000001` vs `0x7FFFFFFF`, and with `0x7FFFFFFF` vs another `0x7FFFFFFF`.** Confirm the RFC's ordering is preserved.

### 3. Max-Age Purge Retention

When an LSA reaches age MaxAge (either because the originator sent a purge, or because the local age counter reached 3600), it must be retained in the LSDB and flooded as a purge **until every neighbour has acknowledged it or is no longer Full**. Deleting too early leaves a window in which a late neighbour re-floods the old copy and the stale LSA reappears.

**Test.** Force a purge, stop a neighbour's ack, verify the purge persists. Resume the neighbour, verify the purge is acked and then removed.

### 4. MTU Mismatch in DD

RFC 2328 §10.6 requires that the DD packet carry the interface MTU and that the receiver reject a DD with a larger MTU than its own. Failing to honour this causes packets to be fragmented or dropped silently during LSA flooding, which is very hard to diagnose. A mismatch is detected in the ExStart/Exchange transition and forces the NSM back to ExStart indefinitely.

FRR (and Cisco, and Juniper) offers `mtu-ignore` as an escape hatch for deployments where MTUs are known to differ benignly. The first-pass implementation should implement the strict check and the ignore override.

### 5. Network-LSA LS ID Confusion

The LS ID of a Router-LSA is the originating Router ID. Obvious. The LS ID of a Type 2 Network-LSA is **the DR's interface address** on the segment, **not** the network prefix. This is inconsistent with every other LSA type and has caught many an implementer. The network mask is carried in the LSA body, not in the LS ID.

When flooding a Type 2 into an LSDB keyed on `(Type, LS ID, Advertising Router)`, the DR is the advertising router and the LS ID is the DR's interface address. When SPF walks the graph, the Network-LSA is looked up by the interface address from the corresponding Router-LSA's transit link descriptor. Get this wrong and Dijkstra silently drops half the topology.

### 6. Two-Way Check in SPF

RFC 2328 §16.1 mandates that when Dijkstra visits a neighbour vertex via a link, the neighbour's own LSA must contain a reciprocal link back to the visiting router. Without this check, a stale or broken one-way adjacency causes SPF to walk in one direction across the link and compute routes that do not actually forward.

**Test.** Manufacture an LSDB where router A's LSA mentions B but B's LSA does not mention A. Verify SPF does not install routes through the broken adjacency.

### 7. External LSA E1 vs E2 Ordering

RFC 2328 §16.4 specifies that E1 (type-1 external) routes always win over E2 (type-2 external) routes, regardless of metric, because E1 accounts for the internal cost to the ASBR while E2 does not. A comparison that treats the metric as the primary key and the type as a tiebreaker is wrong.

### 8. ABR Acceptance Rule

RFC 2328 §16.2 says that at an ABR, only summaries received **through Area 0** are used to compute inter-area routes. Summaries received through non-backbone areas are ignored for route computation (though they are still stored in the LSDB for re-flooding if appropriate). This prevents loops in a scenario where a non-backbone area has multiple ABRs advertising the same prefix with different metrics.

### 9. NSSA Translator Election

RFC 3101 §3.5 specifies that one NSSA ABR is elected as the Type 7→Type 5 translator, by Router ID (highest wins). If you forget to implement the election and every NSSA ABR translates, you get duplicate Type 5 LSAs in the backbone.

### 10. Authentication with Zeroed Checksum

For AuType 2 and RFC 7474, the OSPF common-header checksum **must be zeroed** before the digest is computed and **must be zero** in the transmitted packet. If you compute the IP-style checksum and then compute the HMAC, the receiver will mismatch. Clear the checksum field before HMAC, then never write a checksum after.

### 11. Hello E-bit Mismatch in Stub Areas

In stub areas, the E-bit in the Hello's Options field must be clear (because stub areas do not carry external LSAs). Failing to clear it causes adjacency to never form with no obvious error. Similarly the N-bit must be set in NSSAs.

### 12. DR Stickiness on Join

A new router with higher priority that joins a LAN with an existing DR must **not** become the DR. The two-phase election's sticky rule (RFC 2328 §9.4) is easy to miss: the election prefers the router that already claims to be DR. Getting this wrong causes flapping every time a router reboots.

### 13. Checksum Refresh on Re-origination

When the originator increments the sequence number and re-originates an LSA, the checksum must be recomputed (since the LSA body did not change but the sequence number did). Do this in the origination path, not opportunistically.

### 14. Clock vs Monotonic Time

LS age and RxmtInterval are wall-clock seconds. If the system clock jumps (NTP, VM migration), LSA ages can skew. Use a monotonic clock for timer intervals internally, and compute ages as deltas from a monotonic origination timestamp, not from `time()`.

---

## 14. What FRR Implements and What It Does Not

FRR's ospfd is production-grade and implements most of OSPFv2 plus many extensions. The following is a coarse catalogue with scope notes for a first-pass clean-room implementation.

### Core Protocol (RFC 2328)

**Fully implemented.** All five packet types, all LSA types 1–5, all state machines, DR/BDR election, stub areas, virtual links, all four authentication types (including the RFC 5709/7474 HMAC-SHA trailer). No deviations from the base specification.

### NSSA (RFC 3101)

**Implemented**, including Type 7 origination, flooding within the NSSA, translator election, and Type 7→Type 5 translation at the elected ABR.

### Opaque LSA Framework (RFC 5250)

**Implemented.** Types 9, 10, and 11 are flooded per their scope rules; a registration API lets extension modules hook into origination and reception.

### Traffic Engineering (RFC 3630, RFC 5392)

**Implemented** as an opaque-LSA consumer in `ospf_te.c`. TE LSAs carry link bandwidth, admin groups, SRLGs, and TE metrics. First-pass ze implementations can safely skip this — no MPLS-TE, no TE-aware path computation.

### Router Information (RFC 7770)

**Implemented.** Advertises router capabilities via an opaque LSA. Feeds into Segment Routing and PCE discovery. Defer.

### Extended Link and Extended Prefix Opaque LSAs (RFC 7684)

**Implemented** as the foundation for Segment Routing. Defer until you need SR.

### Segment Routing (RFC 8665)

**Implemented.** Advertises prefix-SIDs and adjacency-SIDs, allocates SRGB/SRLB, integrates with the MPLS forwarding plane. Defer.

### Graceful Restart and Helper (RFC 3623)

**Implemented** for both the restarting side and the helper side. Grace-LSAs (Opaque Type 3) announce the restart window; helpers suppress LSDB churn and adjacency tear-downs for the grace period. A first-pass implementation should support helper mode (it is strictly receive-side) and defer the restarter side.

### BFD Integration (RFC 5880, RFC 5881)

**Implemented** as a thin wrapper around zebra's BFD subsystem. BFD failure triggers an NSM event that declares the neighbour down immediately. Defer if ze has no BFD subsystem yet; it can be bolted on later.

### LDP-IGP Synchronisation (RFC 5443, RFC 6138)

**Implemented.** Delays interface cost convergence until LDP has converged on the same link, avoiding transient black holes in MPLS networks. Defer unless ze has LDP.

### TI-LFA (Fast Reroute)

**Implemented** in `ospf_ti_lfa.c`. Computes loop-free backup paths for fast reroute. Defer.

### OSPF MIB (RFC 4750)

**Implemented** for SNMP monitoring. Defer.

### External LSA API (ospfclient)

**Implemented** as a Unix-domain socket API for external processes to inject Opaque LSAs. Useful for research and experimentation but not needed in production. Defer.

### SNMP and Operational Hooks

Beyond the MIB, FRR exposes statistics and `show` commands via VTY (the legacy CLI). ze should expose the equivalent via its own CLI and via the web/looking-glass components.

### Stub Router Advertisement (RFC 3137, RFC 6987)

**Implemented.** Lets a router advertise itself as a "stub" router for a bounded window by setting the metric of every non-stub link in its Router-LSA to `LSInfinity` (`0xFFFF`). Transit traffic drains off the router while the LSAs propagate, but the router itself remains reachable for its own directly-attached prefixes. The canonical uses are (a) startup — advertise stub while BGP is still converging, then withdraw after a hold-down, so transit traffic does not enter the router before it has a full view; (b) planned maintenance — announce stub ahead of a reboot so traffic drains gracefully. RFC 3137 is the original; **RFC 6987** updates it with clarifications and explicitly says `LSInfinity - 1` on stub links can be used to keep local prefixes reachable while forbidding transit. FRR's implementation tags this feature "max-metric router-lsa" in the CLI. BIRD's OSPF implements RFC 6987 as well. **SHOULD** implement in ze; it is cheap, safe, and valuable operationally.

### OSPFv2 Multi-Instance (RFC 6549)

**Implemented in BIRD, partially in FRR.** Borrows OSPFv3's Instance ID concept: an 8-bit field added to the OSPFv2 packet header (reusing the reserved byte in RFC 2328 §A.3.1) lets multiple OSPFv2 processes share a single interface without confusing each other's packets. Used most often for multi-topology deployments, for MPLS VPN PE boxes running multiple virtual OSPF instances on one interface, and for testbeds. The wire change is trivial (one byte); the hard part is plumbing the Instance ID through the interface binding, neighbor matching, and configuration surfaces. **DEFER** in ze; niche requirement, no impact on single-instance deployments.

### OSPF for MPLS/BGP L3VPN: the DN Bit (RFC 4576)

**Implemented in BIRD, implemented in FRR.** When a PE (Provider Edge) router runs OSPF with a CE (Customer Edge) as part of an MPLS L3VPN, there is a loop risk: a route learned from CE-A, carried across the MPLS backbone via BGP/VPNv4, and re-injected to CE-B as an OSPF Summary LSA, can be learned back by the originating PE and mistaken for an external route. RFC 4576 defines a **DN (Down) bit** (bit 0x8000 of the LSA header Options field, repurposed) that the PE sets when injecting a Summary or AS-External LSA learned from BGP. Any PE receiving an LSA with DN set ignores it for SPF, breaking the loop. RFC 4577 specifies the full OSPF-as-PE-CE procedure built on top of this. **DEFER** in ze; only relevant for L3VPN deployments, which depend on MPLS and VRF support that are separate undertakings.

### OSPF Flood Reduction (RFC 7715)

**Infrastructure present in FRR; enabled per-interface.** Normally OSPF refreshes every LSA every 30 minutes (`LSRefreshTime`) even if nothing has changed, to defend against silent LSDB corruption. In very stable topologies with thousands of LSAs, this refresh traffic becomes significant. RFC 7715 reuses the **DoNotAge (DNA)** bit from RFC 1793 (originally defined for demand-circuit support) to mark LSAs as exempt from periodic refresh: the originating router floods the LSA once with DNA set, receivers install it permanently, and the LSA is only re-flooded if its content actually changes. Participating routers advertise the DC (demand circuit) capability in the Hello Options field and the Router-LSA Options field. Failure modes are non-trivial (if a receiver missed the original DNA flood, it has no way to learn about the LSA until a real change). **DEFER** in ze; optimization for environments with huge LSDBs and stable topologies, not needed for a first-pass implementation.

### What FRR Does Not Implement (Or Does Minimally)

- **RFC 2676 QoS routing extensions.** Nobody implements these.
- **RFC 5185 multi-area adjacencies.** Allows a single interface to participate in multiple areas. Niche.
- **RFC 6845 simplified and flexible area types.** Some overlap with stub/NSSA; little real deployment.
- **Pre-RFC TOS routing.** Deprecated.

### Recommended scope for ze's first pass

**MUST:**
- Packet codec for all five types and LSA types 1–5.
- Common header including AuType 0 and AuType 2 (MD5 legacy).
- Full ISM, NSM, DR/BDR election.
- Full flooding procedure with retransmit and delayed ack.
- Per-area LSDB with refcounting, aging, refresh, and MaxAge purge.
- Full Dijkstra SPF with ECMP and the two-way check.
- Intra-area and inter-area route computation.
- AS-external route computation with E1/E2 semantics and forwarding address.
- Stub areas (RFC 2328 §3.6).
- Route installation into `sysrib`.
- YANG configuration and `show ip ospf` CLI.

**SHOULD:**
- NSSA (RFC 3101) with Type 7, translator election, and the P-bit priority ordering (§6f).
- HMAC-SHA trailer authentication (RFC 7474).
- Graceful Restart helper mode (RFC 3623).
- Stub Router Advertisement (RFC 3137, RFC 6987) — cheap operational win.
- Opaque-LSA framework plumbing (but no consumer modules).
- Virtual links (RFC 2328 §15), if the deployment needs discontiguous backbone repair.

**DEFER:**
- Segment Routing (RFC 8665) and RFC 7684 Extended Link/Prefix.
- Traffic Engineering LSAs (RFC 3630).
- Router Information LSA (RFC 7770).
- TI-LFA fast reroute.
- BFD integration.
- LDP-IGP sync.
- SNMP / MIB.
- External LSA API.
- OSPFv2 Multi-Instance (RFC 6549).
- OSPF L3VPN DN bit (RFC 4576) — only relevant with MPLS/VPN support.
- Flood Reduction (RFC 7715) — only relevant for very stable huge-LSDB networks.
- RFC 2676 QoS and RFC 5185 multi-area adjacencies.

---

## 15. OSPFv3 (RFC 5340) Considerations

OSPFv3 is a separate protocol on the wire but shares the high-level architecture (areas, LSAs, DR/BDR, SPF) with OSPFv2. FRR splits it into a second daemon (`ospf6d`) rather than cohabiting a single binary, and the split is sensible: the wire format and LSA registry are different enough that unifying them would produce more pain than gain.

The main differences:

**Transport over IPv6 (RFC 5340 §2.9).** OSPFv3 runs over IPv6 protocol 89 with multicast addresses `ff02::5` (AllSPFRouters) and `ff02::6` (AllDRouters). Source address is the link-local address of the originating interface. Destination is link-local or unicast (virtual links use global IPv6 addresses).

**Shorter common header (RFC 5340 §A.3.1).** 16 bytes instead of 24. Version, Type, Length, Router ID, Area ID, Checksum, Instance ID, and one reserved byte. **No authentication fields.** Authentication is done via an optional RFC 7166 trailer or via IPsec AH/ESP (RFC 4552).

**Instance ID (RFC 5340 §2.5).** 8-bit identifier allowing multiple OSPFv3 instances on the same link. Originally for multi-topology routing; now also used for multi-address-family support (RFC 5838).

**LSA type renumbering with scope in the type field (RFC 5340 §A.4.2.1).** OSPFv3 LSA types encode the flooding scope in the top two bits:
- `0x0000`–`0x1FFF`: link-local scope (flooded only on one link).
- `0x2000`–`0x3FFF`: area scope.
- `0x4000`–`0x5FFF`: AS scope.

The defined types are:
- `0x2001` Router-LSA (analogous to OSPFv2 Type 1 but with **no prefix information**).
- `0x2002` Network-LSA (analogous to Type 2, also without prefixes).
- `0x2003` Inter-Area-Prefix-LSA (analogous to Type 3).
- `0x2004` Inter-Area-Router-LSA (analogous to Type 4).
- `0x4005` AS-External-LSA (analogous to Type 5).
- `0x2007` NSSA-LSA (RFC 3101 analogue for v3).
- `0x0008` Link-LSA (new in v3, per-link, advertises the interface's link-local address and the list of IPv6 prefixes on the link).
- `0x2009` Intra-Area-Prefix-LSA (new in v3, carries prefixes for routers and transit networks, referencing the Router-LSA or Network-LSA by a 32-bit reference).
- `0x000B` Grace-LSA (for graceful restart, RFC 5187).

**Topology and prefix separation.** This is the architectural change that justifies OSPFv3's separate encoding: the Router-LSA and Network-LSA describe **only the graph**, not prefixes. Prefixes live in separate Intra-Area-Prefix-LSAs that reference the vertices. The benefits are:
- Prefix changes do not re-originate the topology.
- Multiple prefix families (IPv6, IPv4 via RFC 5838) can be layered on the same topology.
- Link-scope information (link-local addresses, per-link prefixes) is isolated in Link-LSAs.

**Interface ID (RFC 5340 §2.11).** Since IPv6 link-local addresses are not globally unique, OSPFv3 introduces a 32-bit Interface ID per router per interface to identify links in Router-LSAs.

**Multiple address families (RFC 5838).** A single OSPFv3 instance can carry multiple address families (IPv6 unicast, IPv4 unicast, IPv6 multicast) using the Instance ID to distinguish them. In practice FRR supports IPv6 unicast well and the others minimally.

**Authentication.** OSPFv3 originally relied on IPsec (RFC 4552), which is brittle. RFC 7166 adds an Authentication Trailer that mirrors OSPFv2's RFC 7474, with HMAC-SHA-1/256/384/512. FRR's ospf6d implements RFC 7166.

### Recommendation for ze

Start with OSPFv2 only. The wire format, state machines, and SPF are intricate enough on their own. Once ospfv2 is correct and deployed, add OSPFv3 as a separate component (`internal/component/ospfv3/` or similar). Most of the SPF, LSDB, and state machine code cannot be cleanly shared because the LSA types and encodings are different, but the patterns are identical and the second implementation will go faster than the first.

Do **not** design a grand unified "OSPF" component that tries to abstract over v2 and v3. FRR tried; it split into two daemons instead. Follow suit.

---

## 16. Recommended Implementation Order

Build OSPFv2 in phases. Each phase has a clear test gate before moving on.

### Phase 1 — Domain Types and Utilities

**Goal.** Foundational data structures without any networking.

**Deliverables.**
- RouterID, AreaID, LSAKey, LSSequenceNumber, LSAge, Metric types with parse/format/equality/order.
- Fletcher-16 checksum (shared with IS-IS).
- IP-style one's complement checksum.
- LS age counter utilities with MaxAge and DoNotAge handling.

**Test gate.** Unit tests round-trip every type. Fletcher passes RFC 905 vectors. IP checksum passes RFC 1071 vectors.

### Phase 2 — Packet Codec

**Goal.** Encode and decode all five packet types and LSA types 1–5.

**Deliverables.**
- Common header codec (24 bytes, AuType 0 and 2).
- Hello, DB Description, LS Request, LS Update, LS Ack.
- LSA common header (20 bytes) and body codecs for Router, Network, Summary (Network and ASBR), AS-External.
- Buffer-first encode path consistent with ze's philosophy (write into a pooled buffer, no `append`).

**Test gate.** Round-trip every packet and LSA. Decode real captures from Wireshark. Fuzz the decoder; no panics on random bytes.

### Phase 3 — Instance and Area Scaffolding

**Goal.** The bare skeleton of an OSPF instance: configuration apply, area creation, interface enrollment.

**Deliverables.**
- `struct instance` holding router-id, area map, timers, LSDB.
- `struct area` per area, with its own LSDB.
- `struct oi` per enrolled interface, with network type and ISM.
- Config parse from YANG and apply/re-apply.

**Test gate.** Load a YANG config; verify areas and interfaces are created correctly. Unit-test config diff logic.

### Phase 4 — ISM and Hello

**Goal.** Interfaces can come up, Hellos are sent and received, ISM moves through its states.

**Deliverables.**
- Hello sender (timer-driven).
- Hello receiver and header validation.
- ISM with all eight states and every documented event.
- DR/BDR election including the sticky rule.

**Test gate.** Point-to-point interface reaches Point-to-Point state within 1 HelloInterval. Broadcast interface reaches DR, Backup, or DROther within 1 RouterDeadInterval. Three routers on a LAN elect one DR, one BDR, and one DROther.

### Phase 5 — NSM and DD Exchange

**Goal.** Neighbours transition to Full.

**Deliverables.**
- NSM with all nine states.
- DD packet generation and reception.
- Master/slave negotiation.
- LS Request list population and drain.

**Test gate.** Two routers on a link reach Full state within 5 HelloIntervals. Kill one router; the other transitions to Down within RouterDeadInterval. Restart; re-sync.

### Phase 6 — LSDB, Origination, Flooding

**Goal.** LSAs flow between routers.

**Deliverables.**
- Per-area LSDB with refcount.
- Self-LSA origination (Type 1 Router-LSA) on interface events and refresh timer.
- Self-LSA origination (Type 2 Network-LSA) when elected DR.
- Flooding procedure with per-neighbour retransmit lists and delayed acks.
- MaxAge walker and LSRefresh.

**Test gate.** Three routers in a line; change a cost on one; verify the change propagates to the other two within 2 HelloIntervals. Verify LSAs at MaxAge are purged and removed.

### Phase 7 — SPF and Route Installation

**Goal.** Compute shortest paths and install routes.

**Deliverables.**
- Dijkstra with two phases (router/network vertices, then stub links).
- Two-way check.
- ECMP via equal-cost parent merge.
- SPF throttle with exponential back-off.
- Routing table with path type and admin distance.
- Integration with `sysrib`.

**Test gate.** Line topology with prefixes on every router; verify every router's RIB has routes to every prefix with correct next-hops. Kill a link; verify SPF re-runs and routes converge.

### Phase 8 — Inter-Area and ABR Origination

**Goal.** Two areas connected by an ABR exchange summaries.

**Deliverables.**
- Type 3 Summary-LSA origination at the ABR.
- Type 4 Summary-LSA origination at the ABR (when ASBRs exist).
- Inter-area route computation per RFC 2328 §16.2.
- Area range aggregation and `not-advertise`.

**Test gate.** Two areas, one ABR, prefixes in each; verify each area's routers see the other area's prefixes as inter-area routes with the ABR as nexthop.

### Phase 9 — AS-External and ASBR

**Goal.** Redistributed routes flood across the AS.

**Deliverables.**
- Type 5 AS-External-LSA origination on redistributed routes.
- External route computation per RFC 2328 §16.4 with E1/E2 semantics and forwarding address.
- `default-information originate`.

**Test gate.** Redistribute a static route at one router; verify every other router learns it as an external route with correct metric type.

### Phase 10 — Stub and NSSA

**Goal.** Stub and NSSA areas work correctly.

**Deliverables.**
- Stub area filtering (no Type 5 in stub; default summary from ABR).
- NSSA Type 7 origination.
- NSSA translator election (RFC 3101 §3.5).
- Type 7 → Type 5 translation at the elected ABR.

**Test gate.** Stub area: external routes from another area are not visible, default route is. NSSA: internal redistribution produces Type 7, translator converts to Type 5 on the backbone.

### Phase 11 — Authentication

**Goal.** Support MD5 and HMAC-SHA trailer.

**Deliverables.**
- AuType 2 with MD5 digest.
- RFC 7474 HMAC-SHA trailer.
- Key rotation support.

**Test gate.** Two routers configured with the same key form an adjacency. Wrong key fails. Key rotation succeeds hitlessly.

### Phase 12 — CLI, YANG, Diagnostics

**Goal.** Operators can configure, monitor, and debug.

**Deliverables.**
- YANG module complete.
- `show ip ospf`, `show ip ospf neighbor`, `show ip ospf interface`, `show ip ospf database`, `show ip ospf route`, `show ip ospf border-routers`, `show ip ospf spf`.
- Debug toggles and logging.
- Counters (adjacency count, LSA counts per type, packet RX/TX per type, SPF runs, auth failures).

**Test gate.** Each CLI command returns expected output. Configuration round-trips through YANG cleanly.

### Phase 13 — Interop

**Goal.** Verify wire compatibility with FRR.

**Deliverables.**
- `.ci` tests with a Linux namespace running FRR ospfd and a matching namespace running ze.
- Coverage: P2P, broadcast, multi-area, stub, NSSA, redistribution, authentication, failover.

**Test gate.** Every scenario converges with FRR as the peer. `tcpdump` captures match expected format. `show ip ospf database` on both sides agrees.

### Phase 14+ — Extensions (optional)

Once the core is stable and interop is green, add the SHOULD items (NSSA if not in Phase 10, virtual links, Graceful Restart helper, opaque-LSA framework) and then the DEFER items as driven by real needs.

---

## 17. Reference RFCs and Summary

### Core Normative References

1. **RFC 2328** — OSPF Version 2. The base specification. Indispensable.
2. **RFC 5340** — OSPF for IPv6 (OSPFv3). Required for v3.
3. **RFC 3101** — OSPF Not-So-Stubby Area (NSSA) Option.
4. **RFC 5250** — The OSPF Opaque LSA Option.
5. **RFC 1008** — Implementation Guide for the ISO Transport Protocol (Fletcher checksum, Annex C).
6. **RFC 1071** — Computing the Internet Checksum (OSPF packet header checksum).

### Authentication

7. **RFC 5709** — OSPFv2 HMAC-SHA Cryptographic Authentication.
8. **RFC 7474** — Security Extensions for OSPFv2.
9. **RFC 7166** — Supporting Authentication Trailer for OSPFv3.
10. **RFC 4552** — Authentication/Confidentiality for OSPFv3 (IPsec).

### Extensions (optional)

11. **RFC 3630** — Traffic Engineering (TE) Extensions to OSPF Version 2.
12. **RFC 5392** — OSPF Extensions in Support of Inter-AS MPLS and GMPLS TE.
13. **RFC 7770** — Advertising Router Information (Router Information LSA).
14. **RFC 7684** — OSPFv2 Prefix/Link Attribute Advertisement.
15. **RFC 8665** — OSPF Extensions for Segment Routing.
16. **RFC 8476** — Signaling Maximum SID Depth (MSD) Using OSPF.
17. **RFC 3623** — Graceful OSPF Restart.
18. **RFC 5187** — OSPFv3 Graceful Restart.
19. **RFC 5443** — LDP IGP Synchronization.
20. **RFC 6138** — LDP IGP Synchronization for Broadcast Networks.
21. **RFC 5880** — Bidirectional Forwarding Detection.
22. **RFC 5881** — BFD for IPv4 and IPv6 (Single Hop).
23. **RFC 4136** — OSPF Refresh and Flooding Reduction in Stable Topologies.
24. **RFC 4750** — OSPF Version 2 Management Information Base.
25. **RFC 9129** — YANG Data Model for the OSPF Protocol.

### Historical and Less Common

26. **RFC 5185** — OSPF Multi-Area Adjacency.
27. **RFC 5838** — Support of Address Families in OSPFv3.
28. **RFC 6845** — OSPF Hybrid Broadcast and Point-to-Multipoint Interface Type.
29. **RFC 1793** — Extending OSPF to Support Demand Circuits.
30. **RFC 5286** — Basic Specification for IP Fast Reroute (LFA).

### Implementation Priorities (Condensed)

- **Must have.** RFC 2328 end to end (packet codec, ISM, NSM, DR/BDR, flooding, LSDB, SPF, intra/inter-area, AS-external, stub), RFC 1008 Fletcher, RFC 1071 IP checksum, at least AuType 2 (MD5). Route install via `sysrib`. YANG config and operational CLI.
- **Should have.** RFC 3101 (NSSA), RFC 7474 (HMAC-SHA auth), RFC 3623 helper mode, RFC 5250 opaque framework.
- **Nice to have.** RFC 3630 TE, RFC 8665 SR, RFC 7684 Extended Link/Prefix, RFC 5880 BFD, RFC 5443 LDP sync, RFC 4750 MIB, RFC 5340 OSPFv3.

### Strategy

1. Implement Phases 1–7 for a minimum-viable OSPFv2 that can form adjacencies on point-to-point and broadcast networks, flood LSAs, and compute and install intra-area routes.
2. Add Phases 8–9 for full multi-area routing with redistribution.
3. Add Phases 10–11 for stub/NSSA and authentication.
4. Validate with Phases 12–13 (CLI, diagnostics, interop).
5. Treat extensions (Phase 14+) as demand-driven — do not write speculative code for features nobody has asked for yet.

The hardest parts are Fletcher-16 (§13.1), sequence number comparison (§13.2), Max-age purge retention (§13.3), and the SPF two-way check (§13.6). Test these early, test them hard, and test them against FRR's wire output. Interop with FRR is the best single check on correctness because FRR has been deployed for decades and its bugs are mostly known — if ze agrees with FRR on a complex LSDB, both are probably right.

Good luck with the implementation. OSPF is denser than IS-IS but the payoff is a protocol that is still dominant in enterprise and carrier networks, and having a native Go implementation in ze opens up the rest of the FRR-style multi-protocol story (IS-IS, BGP, OSPF, all on one stack).
