# IS-IS Implementation Guidance for Clean-Room Reimplementation

## Executive Summary

This document describes the IS-IS (Intermediate System to Intermediate System) protocol and bio-rd's design approach, for the purpose of writing a clean-room reimplementation in a different routing daemon (ze). This document cites RFC 10589 (the current standard for IS-IS) and related RFCs extensively, avoiding any direct reproduction of bio-rd's source code. The goal is to equip a developer with enough understanding of the protocol and architectural patterns to build an independent, compatible implementation without needing to study bio-rd's Go source line-by-line.

### Reference Implementations Examined

This guide is a clean-room reading of the reference implementations listed below. No source code, struct layouts, identifier names, comments, or algorithm implementations from any of these projects have been reproduced here; only architectural decisions and protocol semantics are described, with all wire-level normative behaviour cited from the relevant RFCs.

| Project | Commit | Version / Date | Path | Role in this guide |
|---------|--------|----------------|------|--------------------|
| bio-routing `bio-rd` | `14a8de96` | (2026-03-19) | `bio-rd/protocols/isis/` | Primary reference the guide was originally built around; per-PDU / per-TLV file split, Go-idiomatic layout, partial implementation (no SPF, L1 stubbed) |
| FRRouting `frr` | `cd39d029` | Release 10.5.3 (2026-03-13) | `frr/isisd/` | Secondary reference for extension coverage (SR, SRv6, TE, MT, flex-algo, LFA/TI-LFA, BFD, LDP-sync), raw-socket back ends, and the most complete YANG northbound in FRR |
| BIRD | `f0f859c2` / `2500b450` | 2.18 and 3.2.1 (April 2026) | n/a | **BIRD does not implement IS-IS.** A dead `isis` branch (last commit 2012-08-29) exists upstream but was never merged. See §15 for details. |

bio-rd is Apache-2.0; FRR is GPLv2; BIRD is GPLv2. This guide references their architectural decisions and never their code.

---

## 1. IS-IS in 300 Lines: Protocol Overview

### What IS-IS Is

IS-IS is a link-state interior gateway protocol standardized in ISO 10589 (and updated by IETF RFCs, with RFC 10589 being the current normative reference as of 2024). It is a **layer-2 aware** routing protocol—it runs directly over CLNS (Connectionless Network Service, ISO 8348) and can also carry IP routing information (RFC 5305 for IPv4, RFC 5308 for IPv6). Unlike OSPF, IS-IS uses **TLV (Type-Length-Value) encoding** for extensibility rather than fixed-format fields, making it simpler to add new capabilities without changing the wire format.

### Key Concepts

**System ID and Addressing** (RFC 10589 §1.4, §6.2): Each router has a 6-byte **System ID** uniquely identifying it within the routing domain. A **NET (Network Entity Title)** is the full ISO network address: `<AreaAddress | SystemID | NSEL>`. Area addresses are variable-length (1–13 bytes); the SEL byte at the end is traditionally 0 for routers. For example, a NET might be `49.0001.1234.5678.9abc.0000.00`, where `49.0001` is the area, `1234.5678.9abc` is the System ID, and `00` is the SEL. A router can have multiple NETs (one area address per NET, or multiple area addresses in one NET).

**Two Levels of Routing** (RFC 10589 §1.1, §8.2): IS-IS separates routing into two levels:
- **Level 1 (L1, intra-area)**: Routers within the same area. L1 routers form adjacencies only with other L1 routers in the same area (RFC 10589 §8.2.2 requires matching area addresses for L1 adjacency).
- **Level 2 (L2, inter-area)**: Routers that route across area boundaries. An L2 router can be standalone or also run L1 within its own area.
A single router can be L1-only, L2-only, or L1L2 (both levels).

**PDU Types** (RFC 10589 §9): IS-IS communicates via PDUs (Protocol Data Units):
- **IIH (IS-to-IS Hello)**: Three variants—L1 LAN Hello, L2 LAN Hello, and P2P Hello. Sent periodically to establish and maintain adjacencies. RFC 10589 §8.2, §8.3, §8.4.
- **LSP (Link State PDU)**: Describes a router's topology (its neighbours, IP prefixes it knows, etc.). RFC 10589 §7.3. Each LSP can be fragmented; multiple LSP numbers 1–255 per originator (RFC 10589 §7.3.3).
- **CSNP (Complete Sequence Number PDU)**: A summary of the local LSPDB (LSP database). Used for full synchronisation between neighbours. RFC 10589 §7.3.15.
- **PSNP (Partial Sequence Number PDU)**: Requests missing or newer LSPs. RFC 10589 §7.3.16.

Total: 8 PDU types (L1 and L2 versions of IIH, LSP, CSNP, PSNP, plus P2P Hello).

**DIS (Designated IS)** (RFC 10589 §8.4.5): On broadcast (multi-access) LAN circuits, one router is elected DIS via priority (and MAC address tiebreaker). The DIS creates a **pseudo-node LSP** with LAN ID `<systemid>.<pseudonodeid>`, declaring all routers on the segment as neighbours. This reduces full-mesh adjacencies to a star topology with the DIS at the center—a key simplification in large LANs.

**LSP Flooding** (RFC 10589 §7.3.14–17): When an LSP is received, it is checked for freshness (sequence number). If newer than what we have, it is flooded to all circuits except the incoming one. Flooding is managed via per-circuit, per-LSP flags:
- **SRM (Send Routing Message)**: Set on circuits that should forward this LSP.
- **SSN (Send Sequence Number)**: Set on the circuit that received it, to track whether a PSNP needs to acknowledge it.

**Adjacency State Machine** (RFC 10589 §8.2): Two-way hand-shake (for P2P) or broadcast neighbour detection (for LAN). States are typically Down, Initializing, and Up. Transitions driven by Hello reception and timer expiry.

**LSP Aging and Refresh** (RFC 10589 §7.3.11, §7.3.13): Each LSP carries a **Remaining Lifetime** counter (16-bit seconds). The counter decreases monotonically. When it reaches 0, the LSP is purged (re-originated with zero lifetime and flooded) and eventually removed. The originating router must refresh its own LSPs before they expire (typically every 15–20 minutes; RFC 10589 suggests 1200 seconds).

### Common PDU Header Structure

All IS-IS PDUs share a fixed 8-byte header after the CLNS header:
- **Proto Discriminator** (1 byte, 0x83): Identifies CLNS IS-IS.
- **Length Indicator** (1 byte): PDU length (variable by type, but used for validation).
- **Version/Protocol ID Extension** (1 byte, 0x01): IS-IS version identifier.
- **ID Length** (1 byte): Length of system/node IDs (typically 0 for auto-detect, 6 for fixed 6-byte System IDs).
- **PDU Type** (1 byte): Identifies which of the 9 PDU types (L1 LAN IIH, L2 LAN IIH, P2P Hello, L1 LSP, L2 LSP, L1 CSNP, L2 CSNP, L1 PSNP, L2 PSNP).
- **Version 2** (1 byte, 0x01): For RFC 10589 (called "Version" in old specs; always 1).
- **Reserved** (1 byte, 0x00): Must be zero.
- **Max Area Addresses** (1 byte): Number of area addresses supported (usually 0 or 1 for modern routers).

After this fixed header, each PDU type has its own body (e.g., IIH has circuit type, System ID, holding timer, and optionally DIS ID for LAN variants).

---

## 2. Wire Format and PDUs

### PDU Type Constants

From RFC 10589 §9:
- L1 LAN Hello: 0x0f
- L2 LAN Hello: 0x10
- P2P Hello: 0x11
- L1 LSP: 0x18
- L2 LSP: 0x14
- L1 CSNP: 0x24
- L2 CSNP: 0x19
- L1 PSNP: 0x26
- L2 PSNP: 0x1b

bio-rd supports P2P Hello, L2 LSP, L2 CSNP, and L2 PSNP in its main code paths; L1 support is stubbed with panic statements (indicating it was not prioritised).

### PDU-Specific Headers

**LAN IIH (L1/L2 Broadcast Hello):**
- Circuit Type (1 byte): 1 (L1), 2 (L2), or 3 (L1L2).
- System ID (6 bytes): Sender's System ID.
- Holding Timer (2 bytes): Time to hold this adjacency before timing out (in seconds).
- PDU Length (2 bytes): Total length of the IIH PDU.
- Priority (1 byte): for DIS election (0–127).
- Reserved (1 byte): 0x00.
- Designated IS (6 bytes): System ID of the current DIS (or 0x00... if none elected yet).
- TLVs (variable).

**P2P IIH (Point-to-Point Hello):**
- Circuit Type (1 byte): 1, 2, or 3.
- System ID (6 bytes): Sender's System ID.
- Holding Timer (2 bytes).
- PDU Length (2 bytes).
- Local Circuit ID (1 byte): Circuit ID on the sender's side (for link identification; used in P2P Adjacency State TLV).
- TLVs (variable).

**LSP (Link State PDU):**
- Length (2 bytes): PDU length (minimum 27 bytes without TLVs).
- Remaining Lifetime (2 bytes): Seconds until this LSP expires.
- LSPID (8 bytes): Source System ID (6) + pseudo-node ID (1) + LSP number (1).
- Sequence Number (4 bytes): Monotonically increasing; 0 is reserved for purge. Wraparound handling requires purging at 0xFFFFFFFF and waiting MaxAge + ZeroAgeLifetime before re-originating.
- Checksum (2 bytes): ISO 8473 Fletcher checksum, computed over the LSP starting after the Remaining Lifetime field (RFC 10589 §7.3.11).
- TypeBlock (1 byte): Usually 0x00 for routers.
- TLVs (variable).

**CSNP (Complete Sequence Number PDU):**
- PDU Length (2 bytes).
- Source ID (7 bytes): Sender's Source ID (System ID + pseudo-node ID, for LAN use).
- Start LSPID (8 bytes): Lowest LSPID in this CSNP's range.
- End LSPID (8 bytes): Highest LSPID in this CSNP's range.
- TLVs (variable, usually LSP Entries TLV 9).

**PSNP (Partial Sequence Number PDU):**
- PDU Length (2 bytes).
- Source ID (7 bytes).
- TLVs (variable, usually LSP Entries TLV 9).

### TLV Registry (RFC 10589 §9)

TLVs use the format: **Type (1 byte) | Length (1 byte) | Value (Length bytes)**. bio-rd organises TLVs into individual files; the packet directory lists approximately 40 files for different TLV types and tests.

**Core TLVs (MUST support):**

- **TLV 1: Area Addresses** (RFC 10589 §9.2): List of area addresses (1–13 bytes each). At least one on every LSP. Format: one or more variable-length area IDs.

- **TLV 2 & 6: IS Neighbours** (RFC 10589 §9.3–9.4): TLV 2 is the "narrow" metric version (obsolete per RFC 5305; uses 6-bit metric). TLV 6 is the general form. Each entry is 11 bytes: 7-byte neighbour Source ID + 4-byte metric + sub-TLVs. bio-rd supports TLV 22 (Extended IS Reachability) instead, which is the modern form (RFC 5305).

- **TLV 8: Padding** (RFC 10589 §9.10): Padding bytes to fill up to the MTU. Critical for MTU mismatch detection: if two routers have different MTUs and padding is not used, very large LSPs will silently fragment at the wrong size, breaking routing. RFC 10589 §8.2.3 describes using padded Hellos to detect MTU mismatches.

- **TLV 9: LSP Entries** (RFC 10589 §9.14): Used in CSNP/PSNP to list LSPs. Each entry: 8-byte LSPID + 4-byte sequence number + 2-byte checksum + 2-byte remaining lifetime = 16 bytes per entry.

- **TLV 10: Authentication** (RFC 10589 §9.8): Cleartext password (subtype 1, deprecated) or place-holder for HMAC-MD5 (RFC 5304, subtype 3) or generic cryptographic auth (RFC 5310, subtype 54). Must be **first** in the TLV sequence (RFC 5304 §1).

- **TLV 22: Extended IS Reachability** (RFC 5305 §2): Modern form of TLV 2/6. Each entry: 7-byte neighbour Source ID + 3-byte metric (24-bit, supporting wider range) + sub-TLVs. Supports sub-TLVs for link-local/remote IDs (subtype 4) and IPv4 neighbour addresses (subtype 8).

- **TLV 128 & 130: IP Internal/External Reachability** (RFC 10589 §9.8, §9.9): Old form, 6-bit metric. Rarely used; TLV 135 (Extended IP Reachability) is preferred.

- **TLV 135: Extended IP Reachability** (RFC 5305 §5): Modern IPv4 prefix advertisements. Each entry: 1-byte metric + 1-byte prefix length + variable-length prefix + sub-TLVs.

- **TLV 137: Dynamic Hostname** (RFC 5301): ASCII hostname for the originating router. Single entry per LSP.

- **TLV 232: IPv6 Interface Address** (RFC 5308 §4): IPv6 addresses on the interface.

- **TLV 236: IPv6 Reachability** (RFC 5308 §5): IPv6 prefixes reachable via this router.

- **TLV 229: Multi-Topology** (RFC 5120): Declares which topologies are supported. **bio-rd does not implement this**; it is out of scope.

- **TLV 242: Router Capability** (RFC 7981): Announces router capabilities (e.g., graceful restart, SR, segment routing flex-algo). **bio-rd does not implement this.**

**Important gotchas:**

- **TLV Ordering:** TLV 10 (authentication) must come first in the TLV sequence if present (RFC 5304 §1). Some implementations are strict about this.

- **Sub-TLVs:** TLVs 22 and 135 support sub-TLVs (themselves Type-Length-Value within the TLV). bio-rd includes a few (link-local IDs, IPv4 neighbour addresses); full TE sub-TLV support is not present.

- **Unknown TLVs:** A router must accept and propagate unknown TLVs when re-flooding LSPs (RFC 10589 §7.3.14). Implementations store them as opaque byte sequences.

### Encoding and Decoding

bio-rd separates the concerns:

**Decode Path (bytes → struct → validation):**
1. Receive raw bytes from the network.
2. Parse header; determine PDU type.
3. Dispatch to type-specific decoder (e.g., `DecodeLSPDU`, `DecodeP2PHello`).
4. Parse fixed fields into struct fields (System ID, holding timer, etc.).
5. Call generic `readTLVs(buf)` to parse TLVs; dispatch each TLV to its specific decoder (e.g., `readAreaAddressesTLV`, `readExtendedISReachabilityTLV`).
6. Return populated struct.

**Encode Path (struct → bytes):**
1. For each field, serialize to bytes.
2. For TLVs, call the TLV's `Serialize()` method.
3. Compute checksums (for LSPs, using the Fletcher algorithm).
4. Prepend the header.
5. Return bytes.

**bio-rd's approach vs. ze's philosophy:** bio-rd decodes eagerly on the receive path, storing parsed structs in memory (e.g., LSP objects with TLVs parsed). ze prefers lazy parsing—keeping bytes on the receive path and parsing on-demand. For IS-IS, this means:
- Store incoming LSPs as raw byte slices + parsed metadata (LSPID, sequence number, lifetime, checksum).
- Parse TLVs only when requested (e.g., for route computation or CLI display).
- This saves CPU and memory on the hot path but requires careful handling of TLV parsing errors.

**Checksums:** The ISO 8473 Fletcher checksum is notoriously error-prone (RFC 10589 §7.3.11). The checksum is computed over the entire LSP except the checksum field itself, **but the checksum position is included in the computation**. This creates a bootstrapping problem: the checksum position contains the checksum value, which affects the checksum. RFC 10589 specifies a two-step adjustment algorithm. Most implementations get this wrong on the first try; test extensively against known test vectors.

### Fragmentation

A single router's LSP is limited by the MTU of the network (usually 1492 bytes for Ethernet). If the router has many prefixes or neighbours, the LSP exceeds the MTU. RFC 10589 §7.3.3 allows up to 256 LSP fragments per originator (LSP number 1–255). When an LSP exceeds maxPDUSize, the router must split its state across multiple LSP numbers, each <= maxPDUSize. Fragmentation is the responsibility of the originating router; no special fragmentation is done in transit.

---

## 3. Domain Types and Constraints

The following types appear in the bio-rd implementation (by name only; no code reproduced). Each is essential for correctness:

- **SystemID**: 6-byte fixed array. Uniquely identifies a router. Encoded/transmitted as big-endian bytes. Printable format: `XXYY.XXYY.XXYY` (e.g., `0001.0002.0003`).

- **SourceID**: System ID (6 bytes) + pseudo-node ID (1 byte) = 7 bytes. Identifies a specific node (router or pseudo-node on a LAN). Pseudo-node ID is 0 for regular routers, non-zero for LAN pseudo-nodes.

- **LSPID**: Source ID (7 bytes) + LSP number (1 byte) = 8 bytes. Uniquely identifies an LSP fragment. Example: `0001.0002.0003.00-01` means System ID `0001.0002.0003`, pseudo-node 0, LSP number 1.

- **NET (Network Entity Title)**: Variable-length address: Area ID (1–13 bytes) + System ID (6 bytes) + SEL (1 byte). Example: `49.0001.1234.5678.9abc.0000.00`. The SEL is always 0x00 for routers. Total length 8–20 bytes. Parsing requires extracting the Area ID from the beginning, the System ID from the last 7 bytes (before SEL), and the SEL from the final byte.

- **AreaID**: 1–13 bytes, variable-length. Identifies a level-1 area. Two routers with different area addresses are at least two hops apart (they must use L2 routing to communicate). Format is arbitrary (hierarchical area numbers like `49.0001` are convention, not enforced).

- **SequenceNumber**: 32-bit unsigned integer. Monotonically increasing. 0 is reserved for purge operations. Wraparound: when reaching 0xFFFFFFFF, the router must purge the LSP (send it with remaining lifetime 0), wait for MaxAge + ZeroAgeLifetime (~5 minutes), and only then re-originate the next LSP. This prevents confusion between old and new LSPs with the same number.

- **RemainingLifetime**: 16-bit unsigned integer (seconds). When received, the router immediately decrements it once per second. When 0, the LSP is purged (not immediately deleted, but flooded with remaining lifetime 0, then deleted after a grace period). MaxAge is typically 1200 seconds.

- **Metric**: 6-bit narrow metric (0–63, RFC 10589) or 24-bit wide metric (0–16777215, RFC 5305). Default narrow metric is 10; wide metric is used for higher precision and is preferred in modern networks. The default wide metric for a link is 10.

- **HoldingTime**: 16-bit unsigned integer (seconds). Advertised in Hellos. If a Hello is not received within this time, the adjacency times out. Typically 3x the Hello interval.

- **LSPEntry**: Used in CSNP/PSNP: LSPID (8 bytes) + SequenceNumber (4 bytes) + Checksum (2 bytes) + RemainingLifetime (2 bytes) = 16 bytes.

All of these are immutable keys or values; they must be compared carefully for equality and ordering (especially LSPID and AreaID, which are variable-length or array-based).

---

## 4. State Machines

### 4a. Adjacency State Machine

**RFC 10589 §8.2** defines adjacency states and transitions. The machine has three states:

**States:**
- **Down**: No adjacency; the neighbour is unreachable or not yet heard from.
- **Initializing** (also called "Init"): One Hello has been received, but the neighbour has not yet acknowledged our System ID in its Hello's IS Neighbours TLV.
- **Up**: The neighbour has acknowledged us (two-way handshake complete).

**Events:**
- Hello received (Hello-triggered).
- Hello hold timer expired (timeout).
- Interface/circuit goes down.
- BFD state change (if BFD is enabled; RFC 5880 + RFC 7130, not implemented in bio-rd).

**Transitions:**

1. **Down → Initializing**: On receipt of a valid Hello from a new neighbour.
   - The Hello is parsed; the neighbour's System ID is extracted.
   - If the Hello is a P2P Hello with a P2P Adjacency State TLV (RFC 5303), check the state field. If it's DOWN or INITIALIZING, stay in Initializing (not yet 3-way). If it's UP, go to Up (3-way handshake).
   - If it's a LAN Hello (broadcast), mark the neighbour as Initializing.

2. **Initializing → Up**: On receipt of a Hello that includes **our System ID in its IS Neighbours TLV**.
   - For P2P (RFC 5303), this is indicated by the P2P Adjacency State TLV showing "UP" and echoing back our local circuit ID.
   - For LAN, this is implicit (the neighbour can hear our Hellos and will declare us in its own LSP once we're up; no explicit acknowledgement in Hellos, per classic RFC 10589).
   - Set the adjacency timeout to now + holding timer.

3. **Up → Down**: On timeout (no Hello received within holding timer) or circuit down.

4. **Initializing → Down**: On timeout or circuit down.

**Broadcast (LAN) vs Point-to-Point (P2P) differences:**

- **LAN IIH (RFC 10589 §8.2.4, §8.4.5)**: No explicit 3-way handshake in Hellos. Adjacency is established by the recipient detecting that the DIS (or another router) has declared it as a neighbour in the pseudo-node LSP. However, for simplicity, many implementations declare an adjacency as Up once they hear a Hello from the neighbour on a broadcast interface.

- **P2P IIH (RFC 10589 §8.4, RFC 5303)**: Explicit 3-way handshake. The P2PHello can carry a P2P Adjacency State TLV indicating the sender's view of the adjacency state (DOWN, INITIALIZING, or UP). This is the standard in modern point-to-point links (e.g., MPLS tunnels).

**bio-rd's approach:** Adjacency state is tracked per interface, per neighbour MAC address (for P2P) or neighbour System ID (for LAN). An adjacency checker goroutine (per neighbour) monitors the hold timer and transitions the state. When an adjacency transitions to Up, the neighbour's information (IP addresses, area addresses, protocols) is parsed from the Hello's TLVs and stored. When adjacency transitions to Down, the neighbour record is kept for a grace period (120 seconds in bio-rd) before deletion, to allow for transient failures.

### 4b. DIS Election (Designated IS) on Broadcast Circuits

**RFC 10589 §8.4.5** defines DIS election on broadcast (multi-access LAN) segments.

**Requirement:** A LAN with N routers creates a fully meshed N*(N-1)/2 adjacencies if all routers become adjacent to each other. This is inefficient. DIS election reduces this: the DIS acts as a hub, and non-DIS routers only become adjacent to the DIS. The DIS then generates a **pseudo-node LSP** with LAN ID `<system_id>.<pseudonodeid>` that declares all routers on the segment, effectively representing the LAN as a single intermediate node.

**Election Algorithm:**

1. All routers send IIHs with their priority (0–127, with 64 being a common default) and their MAC address.
2. Routers listen to others' Hellos.
3. If a router sees a Hello from a neighbour with higher priority, or equal priority but higher MAC address, it is **not** the DIS.
4. If a router's priority and MAC are the highest, it is the DIS.
5. If the current DIS is lost (no Hello for hold time), a new DIS is elected.
6. The DIS also sends a "DIS Hello" that includes the Designated IS field set to its own System ID.

**Pseudo-Node LSPs:**

When elected DIS, the router creates LSP(s) with a non-zero pseudo-node ID. These LSPs declare:
- All routers on the LAN (their System IDs).
- The segment's metric (typically 0, meaning the pseudo-node is a virtual intermediate system).
- Any area addresses advertised by the DIS.

Other routers do not advertise the LAN segment in their own LSPs; instead, they advertise the pseudo-node as a neighbour in their Extended IS Reachability TLVs. This centralises the LAN topology.

**DIS Timers:**

- DIS election happens periodically or on Hello receipt.
- If the current DIS is lost for the holding time, a new election is triggered.
- The election is typically very fast (within the next Hello period).

**bio-rd's approach:** DIS election is **not implemented** in bio-rd's main code paths (L1 is stubbed). For L2, broadcast circuits are supported, but DIS election is deferred. When implemented, it should trigger pseudo-node LSP generation.

### 4c. LSP Flooding (RFC 10589 §7.3.14–17)

Flooding is the core mechanism for disseminating LSPs across the network. It must ensure:
1. LSPs propagate to all routers.
2. Newer LSPs replace older ones.
3. Stale LSPs are eventually purged.

**Per-LSP State (SRM and SSN Flags):**

Each LSP in the LSPDB is associated with a set of interfaces, each with two flags:

- **SRM (Send Routing Message) Flag**: Set on interfaces that should **send** this LSP.
  - Set when a newer LSP is received (received with higher sequence number).
  - Cleared when the LSP is sent on that interface and acknowledged (via PSNP or by observing the neighbour in CSNP).

- **SSN (Send Sequence Number) Flag**: Set on the interface that received the LSP.
  - Indicates that a PSNP should be sent to acknowledge receipt.
  - Cleared when a PSNP is sent.

**Flooding Algorithm (on LSP receipt):**

When an LSP is received on interface I from a neighbour:

1. Parse the LSPID, sequence number, and remaining lifetime.
2. Check the local LSPDB for an existing entry with the same LSPID.
3. **Comparison (RFC 10589 §7.3.15):**
   - If the received LSP has a **higher sequence number**, it is newer: accept it, replace the local copy, set SRM on all interfaces except I, and set SSN on I.
   - If the sequence numbers are equal and the checksums differ (corruption detected), discard and request a PSNP.
   - If the sequence numbers are equal and checksums match, do nothing (duplicate).
   - If the received LSP has a **lower sequence number**, it is older: discard it (or send a PSNP back to the sender to inform them we have a newer version).
   - If the received LSP sequence is 0xFFFFFFFF (purge) and we have a higher sequence number, discard the purge (the originator will purge later).

4. **Zero-Lifetime Purge**: If the received LSP has remaining lifetime 0 and sequence number >= what we have, accept it and flood it (marking the LSP as "purged" locally but not deleting it yet).

**Periodic Flooding (SRM Timer):**

- A timer (typically 5 seconds, RFC 10589 recommends 5–30 seconds) periodically wakes up.
- For each LSP with SRM set on an interface, send the LSP on that interface (if not passive).
- After sending, clear the SRM flag on that interface.
- If SRM is not cleared after multiple rounds (e.g., no PSNP received), resend the LSP.

**Acknowledgement via PSNP (RFC 10589 §7.3.16–17):**

- When SSN is set on an interface, the router should send a PSNP listing that LSP entry.
- The PSNP tells the neighbour: "I have received your LSP with this sequence number and checksum."
- Receipt of a PSNP clears the SSN flag.

**CSNP Synchronisation:**

- On LAN circuits (broadcast), the DIS periodically sends a **Complete SNP (CSNP)** listing all LSPs in its range (usually the entire LSPDB in one CSNP, split across multiple if needed).
- Other routers listen to the CSNP and check: are there any LSPs listed in the CSNP that I don't have? If so, set SSN to request a PSNP.
- On P2P circuits, one router sends a single CSNP immediately after adjacency comes up (to synchronise the LSPDB), then periodically thereafter (though less often than on LAN).

**Purge Operation:**

When a router wants to remove an LSP it originated:
1. Set the remaining lifetime to 0.
2. Increment the sequence number.
3. Flood the LSP with remaining lifetime 0 (a "purge").
4. Wait for the propagation delay (typically MaxAge, ~1200 seconds) before removing it from the LSPDB.
5. The router must not originate a new LSP with the same LSPID until after the purge has expired.

**bio-rd's approach:** SRM and SSN flags are stored in the LSPDB entry, indexed by interface. Flooding is driven by a 5-second timer. CSNP is sent periodically on all interfaces; PSNP is sent in response to SSN flags. The `lsdb.go` file manages LSPDB updates and flooding; `net_ifa_tx.go` handles transmission; `net_ifa_rx.go` processes received PDUs.

---

## 5. LSP Database and SPF

### LSPDB Data Model

The LSPDB is a map keyed by LSPID, with each entry containing:

- The LSP bytes (or parsed LSP struct).
- Remaining lifetime (countdown counter).
- SRM bitmap (one bit per interface, indicating SRM flag state).
- SSN bitmap (one bit per interface, indicating SSN flag state).

bio-rd stores the parsed LSPDU struct alongside metadata. An alternative (closer to ze's philosophy) would be to store raw bytes + metadata and parse TLVs lazily.

### LSP Aging

Once per second (driven by a `decrementRemainingLifetimesRoutine`), all LSP entries' remaining lifetimes are decremented by 1. When an LSP reaches remaining lifetime 0, it is removed from the LSPDB (or marked as purged and kept for a grace period). When removing, check if this was an LSP we originated; if so, don't re-originate immediately (wait for sequence number to catch up).

### LSP Origination

A router originates LSPs to describe its own topology. For an L1 router, it creates L1 LSPs. For L2, L2 LSPs. For L1L2, both.

**Triggering Events (RFC 10589 §7.3.12):**

1. **Manual trigger**: Administrator initiates.
2. **Topology change**: Interface goes up/down, neighbour adjacency changes, metric changes, area address changes, etc.
3. **Refresh timer**: Typically every 900 seconds (15 minutes), re-originate with incremented sequence number to refresh the lifetime.
4. **Sequence number wraparound**: At sequence 0xFFFFFFFF, purge and wait.

**Contents of Originating LSP:**

- **Area Addresses TLV**: Area address(es) the router is part of.
- **Extended IS Reachability TLV (TLV 22)**: Neighbours (adjacent routers) and the metric to each.
- **Extended IP Reachability TLV (TLV 135)**: IPv4 prefixes the router knows (either configured, learned from other protocols, or connected subnets).
- **IPv6 Reachability TLV (TLV 236)**: IPv6 prefixes (RFC 5308, if IPv6 is enabled).
- **Dynamic Hostname TLV (TLV 137)**: Router's hostname (RFC 5301), optional.
- **Checksum**: Computed after all fields are assembled.

**Fragmentation**: If the LSP exceeds max PDU size (typically 1492 bytes), split across LSP numbers (LSP number 1, 2, 3, ..., up to 255). Each fragment is a separate LSP with its own sequence number and checksum.

**bio-rd's approach:** LSP origination is triggered by topology changes and a refresh timer. The `lsp.go` file handles LSP generation. The router builds the LSP by iterating over adjacent neighbours (from the neighbor manager) and connected subnets, creating the appropriate TLVs. When triggered, the entire L2 LSP set is regenerated (not incremental). Sequence numbers are incremented atomically (using `atomic.AddUint32` or a mutex).

### SPF (Shortest Path First)

IS-IS uses Dijkstra's algorithm over the directed graph formed by LSPs.

**Graph Construction:**

Each LSP declares edges (adjacencies):
- From the originating router (LSPID system ID) to each neighbour (listed in Extended IS Reachability TLV).
- For a pseudo-node LSP, from the pseudo-node to each router listed (metric 0 typically).

Vertices in the graph are System IDs (or LSPID prefixes). Edges are weighted by metric.

**SPF Algorithm:**

1. Start from the local router (root).
2. Maintain a priority queue of unvisited nodes, sorted by distance.
3. For each node in the queue:
   - Mark as visited.
   - For each edge from this node (from its LSP), if the neighbor is unvisited:
     - Calculate new distance = current distance + edge metric.
     - If this is better than the neighbor's known distance, update it and re-queue.
4. Continue until all reachable nodes are visited.

**Result:** A routing tree rooted at the local router, with distances and next-hops to all other routers.

**SPF Triggering:**

- On receipt of a new LSP (updated by a topology change).
- Periodically (e.g., every 30 seconds, even if no change).
- Debounced: if multiple LSPs arrive in quick succession, wait before re-running SPF (to avoid thrashing).

**Output:** Routes are installed in the FIB (Forwarding Information Base) via a Client interface (e.g., `device.Updater` in bio-rd, or a `sysrib` plugin in ze). Each route specifies:
- Destination prefix.
- Metric.
- Next-hops (multiple for ECMP).
- Interface(s) to use.

**bio-rd's SPF:** SPF is **not fully implemented** in bio-rd (as of the code examined). There is no Dijkstra implementation visible. The LSPDB is maintained and flooded correctly, but no SPF computation or route installation is present. This is a significant limitation; any real implementation must include SPF.

### Multi-Topology

RFC 5120 allows a single IS-IS instance to carry multiple "topologies" (e.g., unicast, multicast, specific traffic classes). Each topology has its own LSPDB and SPF computation, separated by TLV 229 (Multi-Topology).

**bio-rd does not implement Multi-Topology.** For a first implementation in ze, single-topology is acceptable; MT support is a future enhancement.

---

## 6. Circuit Types and Network Model

### Broadcast (LAN) Circuits

A broadcast circuit (Ethernet, 802.11, etc.) can have multiple routers. IS-IS communication on LANs uses multicast MAC addresses:

- **L1 ISs (AllL1ISs)**: `01:80:c2:00:00:14`
- **L2 ISs (AllL2ISs)**: `01:80:c2:00:00:15`
- **All ISs (AllISs)**: `09:00:2b:00:00:05`

IS-IS packets are sent to the appropriate multicast address based on the PDU type and level. Routers listen to the relevant multicast groups.

**Key properties:**
- DIS election (one router per LAN segment).
- Pseudo-node LSP generation by the DIS.
- Full mesh of adjacencies (or star if pseudo-node used).
- Hellos are sent as broadcast IIH (LAN IIH), not P2P IIH.

### Point-to-Point Circuits

A P2P circuit connects exactly two routers (e.g., a serial link, an MPLS tunnel, or a routed tunnel). IS-IS on P2P links uses:

- **Unicast MAC address** of the neighbour.
- **P2P Hello PDU type** (0x11).
- **RFC 5303 3-way handshake** in the P2P Adjacency State TLV (optional but recommended).
- No DIS election; no pseudo-node LSP.

**Key properties:**
- Simple 2-way adjacency establishment.
- No DIS election.
- Lower overhead (fewer adjacencies).

### Circuit Binding to Interfaces

bio-rd binds circuits to OS network interfaces via the `device.Updater` interface. When an interface goes up, IS-IS is enabled on it (if configured). When it goes down, adjacencies are torn down, and the interface is removed from SRM/SSN tracking.

In ze, this is handled by the `iface` plugin (already present), which notifies IS-IS via a subscription when interface status changes. IS-IS should subscribe to `device.DeviceUpdater` and react to interface up/down events.

### Multiple Circuits

A single router can have many circuits (interfaces). Each circuit maintains:
- Its own adjacency state machine (per neighbour).
- Separate SRM/SSN flags in the LSPDB.
- Independent timers (Hello, CSNP, etc.).

---

## 7. Authentication

### Mechanisms

**Cleartext Authentication (TLV 10, subtype 1):**
- Password is sent in the TLV as plaintext.
- Deprecated; do not use for security. Use only for basic sanity checks (e.g., "is this the right network?").

**HMAC-MD5 (RFC 5304):**
- TLV 10, subtype 3.
- Password is hashed with HMAC-MD5 and included in the TLV.
- Supported in bio-rd (file `tlv_authentication.go` is minimal; details in RFC 5304).
- Still widely used, but MD5 is cryptographically weak. Not recommended for new deployments.

**Generic Cryptographic Authentication (RFC 5310):**
- TLV 10, subtype 54 (or others for different algorithms).
- Supports HMAC-SHA-256, HMAC-SHA-512, etc.
- More robust than MD5. Recommended for security.

### Authentication Levels

- **Per-interface**: Each interface can have its own key.
- **Per-area**: All L1 routers in an area share a key.
- **Per-domain**: All L2 routers share a key.

### Key Chain Management

In practice, routers support **key chains** (multiple keys, each with a validity period). A router can have an active key and standby keys. During key rotation, both old and new keys are valid briefly, allowing hitless key updates.

Key chain management is a deployment detail, not part of the protocol itself. However, every implementation should support multiple keys and time-based activation/deactivation.

**bio-rd's approach:** Minimal; TLV 10 is parsed, and authentication type is known, but actual key derivation and verification are not implemented. For a production implementation in ze, add a crypto backend (using Go's `crypto/hmac` and `crypto/sha256` packages, plus a key storage/management layer).

---

## 8. Concurrency and I/O Model

### Goroutine Architecture

A scalable IS-IS implementation needs multiple goroutines:

1. **Per-Interface RX Goroutine**: Listens for incoming packets on the interface (blocks on `ethernetInterface.RecvPacket()`). Decodes PDUs and dispatches to LSPDB or neighbour manager.

2. **LSPDB Actor (Mutex-Guarded)**: Central store for LSPs. Protected by a `sync.RWMutex` to allow concurrent reads (for SPF, display) and exclusive writes (on new LSP receipt or aging). Alternatively, a single-threaded LSPDB channel-based goroutine can serialize all updates.

3. **Per-Neighbour Timeout Goroutine**: For each established adjacency, a goroutine monitors the hold timer (via ticker). On expiry, transitions adjacency to Down.

4. **Per-Interface Hello Sender**: Periodically sends Hellos (Hello interval is configurable, e.g., 10 seconds). Ticker-based.

5. **LSPDB Maintenance Goroutines**:
   - **Lifetime Decrement** (1 second ticker): Decrement remaining lifetimes, trigger purges.
   - **LSP Refresh** (typically 900 seconds): Re-originate own LSPs to refresh lifetime.
   - **CSNP Sender** (10 seconds): Send periodic CSNP on all interfaces.
   - **PSNP Sender** (5 seconds): Send PSNP for any LSPs with SSN flags.
   - **LSP Flood Timer** (5 seconds): Send LSPs with SRM flags.

6. **SPF Computation Goroutine** (event-driven, debounced): Wait for LSPDB changes; debounce for a few hundred milliseconds, then run SPF. Install routes via `sysrib`.

### Synchronization

- **LSPDB**: Protected by a reader-writer mutex. SPF and display queries take read locks; new LSP receipt and aging take write locks.
- **Neighbour state**: Each neighbour has its own state machine, protected by a mutex for the state field and timeout field.
- **Adjacency lists**: Protected by mutexes in the neighbour manager.

### Reactor vs Goroutines (Trade-Off)

**Goroutine approach (bio-rd's model):**
- Multiple independent goroutines, each handling one aspect (RX, TX, timers).
- Easy to understand (each concern has its own loop).
- Efficient (blocks on I/O naturally).
- Requires careful mutex discipline (deadlock risk if not done well).

**Reactor approach (some other implementations):**
- Single goroutine with an event loop, processing events from channels.
- Serializes all updates (no race conditions, easier to reason about).
- Higher context-switch overhead (all logic multiplexed into one loop).

**Recommendation for ze:** Follow bio-rd's multi-goroutine approach, but:
1. Use ze's existing reactor pattern (if it has one) for integration with other plugins.
2. Defer to the LSPDB actor pattern: use a channel-based queue for LSPDB updates to serialize write access.
3. Minimize lock contention; use atomic operations where possible.

---

## 9. Configuration Shape

### Recommended YANG Model

ze uses YANG for configuration. Here's a minimal schema:

```
module isis {
  namespace "http://example.com/isis";
  
  container isis {
    leaf enabled { type boolean; }
    
    leaf-list net {
      type string;
      description "Network Entity Title(s), e.g., 49.0001.0000.0000.0001.00";
    }
    
    leaf level {
      type enumeration {
        enum "l1" { value 1; }
        enum "l2" { value 2; }
        enum "l1-l2" { value 3; }
      }
      default "l1-l2";
    }
    
    leaf hostname {
      type string;
      description "Dynamic hostname to advertise (RFC 5301).";
    }
    
    leaf lsp-lifetime {
      type uint16;
      units "seconds";
      default "1200";
    }
    
    leaf lsp-refresh-interval {
      type uint16;
      units "seconds";
      default "900";
    }
    
    container authentication {
      leaf algorithm {
        type enumeration {
          enum "cleartext";
          enum "hmac-md5";
          enum "hmac-sha256";
        }
      }
      leaf key {
        type string;
        description "Authentication key.";
      }
    }
    
    container level-1 {
      leaf enabled { type boolean; default "true"; }
      leaf hello-interval { type uint16; units "seconds"; default "10"; }
      leaf hold-multiplier { type uint8; default "3"; }
      leaf area-authentication {
        type string;
        description "Key for L1 authentication.";
      }
    }
    
    container level-2 {
      leaf enabled { type boolean; default "true"; }
      leaf hello-interval { type uint16; units "seconds"; default "10"; }
      leaf hold-multiplier { type uint8; default "3"; }
      leaf domain-authentication {
        type string;
        description "Key for L2 authentication.";
      }
    }
    
    container interfaces {
      list interface {
        key "name";
        leaf name { type string; }
        leaf enabled { type boolean; default "true"; }
        leaf passive { type boolean; default "false"; description "No adjacencies on this interface."; }
        leaf type {
          type enumeration {
            enum "broadcast";
            enum "point-to-point";
          }
          default "broadcast";
        }
        
        container level-1 {
          leaf enabled { type boolean; }
          leaf metric { type uint32; default "10"; }
          leaf hello-interval { type uint16; units "seconds"; }
          leaf hold-multiplier { type uint8; default "3"; }
          leaf priority { type uint8; default "64"; }
          leaf authentication { type string; }
        }
        
        container level-2 {
          leaf enabled { type boolean; }
          leaf metric { type uint32; default "10"; }
          leaf hello-interval { type uint16; units "seconds"; }
          leaf hold-multiplier { type uint8; default "3"; }
          leaf priority { type uint8; default "64"; }
          leaf authentication { type string; }
        }
      }
    }
  }
}
```

### Key Parameters

- **NETs**: One or more network entity titles. At least one required.
- **Level**: Determines if router is L1, L2, or L1L2.
- **Hello interval** (per level, per interface): Frequency of Hellos. Default 10 seconds; P2P may be faster.
- **Hold multiplier**: Holding time = hello interval * hold multiplier. Default 3 (30 seconds if hello is 10s).
- **Metric**: Cost to reach neighbors via this interface. Default 10.
- **Passive interfaces**: Do not send Hellos, do not form adjacencies (e.g., for static routes or loopback interfaces).
- **DIS priority**: For broadcast circuits; higher wins DIS election.
- **Authentication**: Key and algorithm per level, per interface, or domain/area wide.

---

## 10. Plugin Model for ze

### File Organization

Assuming ze follows a plugin pattern, IS-IS should be located at:

```
internal/component/isis/
├── isis.go                 # Main plugin entry point
├── config.go               # Configuration parsing / YANG integration
├── server.go               # Core IS-IS server orchestration
├── lspdb.go                # LSPDB actor and maintenance
├── neighbor.go             # Neighbor state machine
├── circuit.go              # Circuit management, per-interface logic
├── spf.go                  # SPF computation
├── route_installer.go      # Integration with sysrib
├── cli.go                  # CLI commands (show isis neighbors, show isis database, etc.)
├── packet/                 # PDU encoding/decoding (similar to bio-rd's structure)
│   ├── header.go
│   ├── hello.go
│   ├── lsp.go
│   ├── csnp.go
│   ├── psnp.go
│   └── tlv_*.go            # TLV handlers
├── types/                  # Domain types (SystemID, LSPID, NET, etc.)
│   ├── types.go
│   ├── net.go
│   └── area.go
└── yang/
    └── isis.yang           # YANG schema (or link to external file)
```

### Dependencies

- **iface plugin**: For interface up/down notifications and TX/RX of packets.
- **sysrib plugin**: For route installation.
- **config plugin**: For YANG schema registration and config parsing.
- **metrics plugin**: For counters (if ze has one).

### Key Interfaces

The IS-IS plugin should expose:

1. **ISISServer interface**: Methods for starting/stopping, adding/removing interfaces, querying adjacencies and LSPDB.

2. **Config subscriber**: Listen for configuration changes (NETs, level, interface metrics, etc.). On change, reconfigure the server (may require restart if NETs change).

3. **CLI commands**:
   - `show isis neighbors`: List adjacencies and their status.
   - `show isis database`: List LSPs in the LSPDB.
   - `show isis spf`: Show SPF results (per level).
   - `show isis routes`: Show routes computed by SPF.
   - `isis clear counters`: Reset statistics.

4. **Metrics** (if ze supports them): Adjacency count, LSP count, SPF runs, authentication failures, etc.

### Integration Points

- **iface plugin**: Subscribe to `device.DeviceUpdater` for interface changes. Call `EthernetInterfaceFactory.GetInterface(name)` to get TX/RX handle.
- **sysrib plugin**: Call the route installation API to insert/delete routes when SPF completes. Each route includes destination, next-hops, and metric.

---

## 11. Testing Strategy

Testing is critical for correctness, especially given the complexity of state machines and checksums.

### Unit Tests

1. **TLV Codec Tests**: For each TLV type, verify:
   - Decode a known byte sequence; check parsed fields.
   - Encode a known struct; check output bytes.
   - Round-trip: encode → decode → encode; output should match input.

2. **PDU Codec Tests**: Similar for PDU headers and full PDUs.

3. **Checksum Tests**: Verify Fletcher checksum computation against test vectors from RFC 10589 or Cisco docs.

4. **Type Tests**: SystemID, LSPID, NET parsing and comparison.

5. **Adjacency FSM Tests**: Mock neighbors, send Hellos, verify state transitions. Test Up, Down, Initializing states.

6. **LSPDB Tests**: Insert LSPs, test aging (lifetime decrement), SRM/SSN flag management.

7. **SPF Tests**: Build a small LSP database manually; run SPF; verify shortest paths match expected output.

### Fuzz Tests

- **Packet Corpus**: Run the decoder against a fuzz corpus of valid and malformed IS-IS packets. Use `go-fuzz` or `Fuzz*` test functions (Go 1.18+).

### Integration Tests

1. **Two-Node Adjacency**: Spin up two IS-IS instances in separate goroutines, connected via an in-memory circuit (channel-based). Verify:
   - Hellos are exchanged.
   - Adjacency transitions to Up.
   - Timeout occurs if Hellos stop.

2. **LSP Flooding**: Three or more nodes connected in a line. Originate an LSP on node A; verify it floods to B and then to C.

3. **SPF Convergence**: Topology with known expected shortest paths. Run SPF; verify routes match.

4. **Adjacency Failover**: Bring up an adjacency, then simulate a failure (stop sending Hellos); verify adjacency times out and routes are recomputed.

### Interop Tests

- **Against FRR (Free Range Routing)**: Set up a lab with FRR routers and your IS-IS implementation. Verify:
  - Adjacencies form (may need to tune timers).
  - LSPs are exchanged correctly.
  - Routes are consistent across the network.
  - Failover and convergence work.

- **Packet Capture**: Compare packets from your implementation against FRR or Cisco IOS-XR captures to spot encoding differences.

### Regression Tests

- Add test cases for every bug found in production or during interop.
- Maintain a "known issues" list of RFCs your implementation doesn't support yet (e.g., Segment Routing, multi-topology, graceful restart).

---

## 12. Known Hard Problems and Traps

### 1. Fletcher Checksum Adjustment

**The problem:** The ISO 8473 Fletcher checksum is computed over the entire LSP, including the checksum field itself. This creates a circular dependency: the checksum depends on the value at the checksum position, which is the checksum being computed.

**The solution (RFC 10589 §7.3.11):** After computing the initial checksum over the PDU with the checksum field set to zeros, apply an adjustment:
- Set checksum field to the computed value.
- Recompute to verify: the checksum of the corrected PDU should be 0.

Most implementations either get this wrong the first time, or implement only one direction (encode or verify) correctly.

**Action:** Write extensive unit tests against known test vectors (e.g., from Cisco docs or RFC test suites). Do not assume your first implementation is correct.

### 2. Sequence Number Wraparound

**The problem:** Sequence numbers are 32-bit (0–4,294,967,295). After reaching 0xFFFFFFFF, the next sequence would wrap to 0. This can cause confusion: a purge LSP (sequence 0) looks like a very old LSP from a different epoch.

**The solution (RFC 10589 §7.3.3):** When sequence number approaches 0xFFFFFFFF:
1. Stop originating new LSPs with the current LSPID.
2. Send a purge (remaining lifetime 0, sequence 0xFFFFFFFF).
3. Wait for MaxAge + ZeroAgeLifetime (~1200 + 1200 = 2400 seconds) for the purge to propagate.
4. Only then re-originate with sequence number 1.

Failure to handle this correctly can cause route flapping or loops. Test this explicitly; do not assume it's rare enough to skip.

### 3. Zero-Lifetime Purge and the Zero Age Lifetime Timer

**The problem:** When an LSP is purged (remaining lifetime set to 0), it must still be flooded to all routers. If it's deleted immediately after flooding, a router that didn't receive the purge will keep using the old LSP. RFC 10589 specifies that a purged LSP must remain in the LSPDB for a grace period (ZeroAgeLifetime, typically 0–600 seconds) before deletion.

**Action:** Implement explicit purge handling: mark LSPs with remaining lifetime 0 separately, don't delete immediately, and clean up after the timer. bio-rd's approach: delete at remaining lifetime 0 (which may be too aggressive; verify against other implementations).

### 4. TLV Ordering Constraints

**The problem:** RFC 5304 (HMAC-MD5 Authentication) mandates that TLV 10 (Authentication) must be the **first** TLV in an LSP. Some implementations are strict about this; if authentication is not first, they reject the PDU.

**Action:** Ensure authentication TLVs are serialized first. Write tests to catch any LSPs with misplaced auth TLVs.

### 5. Area Address Mismatch (L1 Adjacency Requirement)

**The problem (RFC 10589 §8.2.2):** Two routers on the same LAN can only establish an L1 adjacency if they share at least one area address. If router A is in area `49.0001` and router B is in area `49.0002`, they cannot form an L1 adjacency (only L2 if both run L2).

**Action:** When processing P2P Hellos on an L1 interface, check the Area Addresses TLV. If no overlap, reject the adjacency (or log a warning). This is easy to miss and causes silent routing failures.

### 6. Padded Hellos and MTU Mismatch Detection

**The problem (RFC 10589 §8.2.3):** If router A uses a 1500-byte MTU and router B uses a 1492-byte MTU, and an LSP is 1495 bytes, A can send it (will fragment at the IP layer) but B cannot (will fragment at the IS-IS layer). This causes asymmetric routing or loops.

**Detection:** Pad IIH (Hello) packets to the MTU size. If a router receives a Hello with padding, it can infer the sender's MTU. If MTUs mismatch, log a warning and manually adjust MTU or link metrics to avoid the asymmetry.

**Action:** Implement padding in Hellos. Do not skip it to "save bandwidth"; MTU detection is critical for network stability.

### 7. Max PDU Size and LSP Fragmentation

**The problem:** If a router's LSPDB is large, a single LSP fragment may exceed maxPDUSize (typically 1492 bytes for Ethernet). The router must split into multiple LSP numbers (1, 2, 3, ..., up to 255).

**Action:** Implement LSP fragmentation. When originating an LSP, check the size; if it exceeds max, split the TLVs across multiple LSP numbers and re-originate.

### 8. Purge from Zero-Lifetime LSP vs Drop Due to Expiry

**The problem:** A router can receive two types of "dead" LSPs:
- A purge: remaining lifetime 0, sequence number from the originator.
- An expired LSP: received with a non-zero lifetime that decayed to 0 locally.

The difference: a purge must be re-flooded; an expiry is local garbage collection. Confusing the two can cause stale LSPs to linger or valid purges to be dropped.

**Action:** Track the distinction. When remaining lifetime reaches 0, mark as "purged" and re-flood; don't just delete.

### 9. Route Leaking Between L1 and L2

**The problem (RFC 2966, up/down bit):** In an L1L2 router (one that runs both levels), routes learned via L1 can be redistributed to L2 and vice versa. To prevent loops, RFC 2966 defines an "up/down" bit in the Extended IP Reachability TLV. Routes learned from L2 must have the up/down bit set when re-advertised in L1 (to prevent L1 routers from using them as shortcuts back to L2).

**Action:** If implementing L1L2, implement the up/down bit handling. If only L2 is implemented, this is not relevant.

### 10. RFC 5303 (3-Way P2P) vs Classic P2P Interop

**The problem:** RFC 5303 adds a P2P Adjacency State TLV to P2P Hellos for explicit 3-way handshake. Older implementations (pre-2010 or so) do not support it. If your implementation only does 3-way and you peer with a legacy router, adjacency may not form.

**Action:** Support both modes. If the neighbour doesn't send a P2P Adjacency State TLV, fall back to implicit (just-hearing-Hellos) adjacency formation. Prefer 3-way if both sides support it.

---

## 13. What bio-rd Does NOT Implement

This is critical for scoping a reimplementation. Here are features that are missing or stubbed in bio-rd:

### L1 (Level 1)

**Status:** Stubbed with panic statements. A router can be configured as L1-only or L1L2, but Hellos, CSNPs, LSPs for L1 are not sent or processed. If you try to use L1, the code panics.

**Recommendation:** Implement L1 support or clearly document that it's L2-only. For a first pass, L2-only is acceptable; L1 can be added later.

### Traffic Engineering (TE)

**Status:** Not implemented. RFC 5305 defines extended TLVs for TE (link bandwidth, reservable bandwidth, SRLG, admin group, etc.). bio-rd decodes the Extended IS Reachability TLV but not sub-TLVs for TE. Comment in the code notes: "TODO: Add length of sub TLVs. They will be added as soon as we support TE."

**Recommendation:** Store unknown/unsupported sub-TLVs as opaque byte sequences and propagate them when flooding. Don't attempt TE-based routing in a first implementation.

### Segment Routing (SR-ISIS)

**Status:** Not implemented. RFC 8667 defines SR-ISIS extensions (SRGB, SRLB, SID-to-prefix mappings, etc.). No TLV 242 (Router Capability) support.

**Recommendation:** Out of scope for initial implementation.

### IPv6 Routing

**Status:** Partially absent. The proto file shows IPv6 is recognized (IPv6 Protocol enum in LSPDU), but no TLV 236 (IPv6 Reachability) or TLV 232 (IPv6 Interface Address) decoders are visible in the packet directory. This is a significant gap.

**Recommendation:** Implement IPv6 reachability at the same time as IPv4, or document IPv4-only initially.

### Graceful Restart (RFC 5306)

**Status:** Not implemented. No graceful restart TLV (TLV 211) or state machine.

**Recommendation:** Out of scope for initial implementation. Add later if needed.

### BFD for IS-IS (RFC 5880, RFC 7130)

**Status:** Not implemented. No BFD integration. Adjacencies rely on Hello timeouts alone, which are slow (~30 seconds with default timers).

**Recommendation:** Out of scope for initial. If sub-second failover is needed, add BFD later.

### Multi-Topology (RFC 5120)

**Status:** Not implemented. No TLV 229 support, no separate SPF per topology.

**Recommendation:** Out of scope. Use single-topology routing.

### Overload Bit (RFC 3787)

**Status:** Appears to have a mention in comments (TODO: "The router is elected or superseded as the DIS"), but no implementation visible.

**Recommendation:** Implement the overload bit: when set in own LSPs, other routers treat the router as transit-capable but not suitable for destination. Useful during maintenance.

### Administrative Tags (RFC 5130)

**Status:** Not implemented. No tag TLVs in Extended IP Reachability.

**Recommendation:** Out of scope.

### Anycast Addresses

**Status:** Not implemented.

**Recommendation:** Out of scope; rarely used.

### Pseudo-Wire and L2VPN Handling

**Status:** Not implemented.

**Recommendation:** Out of scope.

### Fast Reroute / LFA / TI-LFA (RFC 5286, RFC 7490)

**Status:** Not implemented. No loop-free alternate computation.

**Recommendation:** Out of scope for initial. Implement basic SPF first; LFA is an enhancement.

### SPF itself

**Status:** The most critical missing piece. There is no Dijkstra algorithm or SPF computation visible in the code. The LSPDB is maintained and flooded, but routes are never computed or installed.

**Recommendation:** SPF is mandatory for any production implementation. Without it, IS-IS has no effect on routing.

---

## 14. Recommended Implementation Order

Build IS-IS in discrete phases. Each phase has defined test criteria.

### Phase 1: Domain Types and Utilities

**Goal:** Foundational data structures without any networking.

**Deliverables:**
- SystemID, SourceID, LSPID, NET, AreaID types with parsing, serialization, equality, and ordering.
- Utility functions: compare LSPIDs, parse NETs, format System IDs for display.
- Unit tests: round-trip serialization for each type, parsing valid/invalid inputs.

**Test gate:** All unit tests pass. Hand-test parsing a few real NETs (e.g., `49.0001.0000.0000.0001.00`).

### Phase 2: PDU Codec (Packet Serialization/Deserialization)

**Goal:** Encode and decode all 8 PDU types without business logic.

**Deliverables:**
- ISISHeader decoder/encoder.
- P2P Hello decoder/encoder.
- LAN Hello decoder/encoder (L1, L2).
- LSP decoder/encoder.
- CSNP decoder/encoder.
- PSNP decoder/encoder.
- TLV decoders/encoders for: Area Addresses, Extended IS Reachability, Extended IP Reachability, Dynamic Hostname, IP Interface Addresses, P2P Adjacency State, LSP Entries, Authentication, Padding.
- Checksum computation (Fletcher).

**Test gate:** 
- Round-trip tests: decode real packets from captures (use Wireshark to generate hex), re-encode, compare bytes.
- Fuzz tests: random bytes into decoder; must not crash.
- Checksum tests: known test vectors from RFC or vendor docs.

### Phase 3: Adjacency State Machine and Neighbor Management

**Goal:** Form and maintain adjacencies.

**Deliverables:**
- Adjacency FSM (Down, Initializing, Up).
- Neighbor tracking per interface.
- Hello sender (timer-based).
- Hello receiver and processing.
- Hold timer management.
- Adjacency state API (get all neighbors, get neighbors in state Up, etc.).

**Test gate:**
- Unit tests: mock interface, send/receive Hellos, verify state transitions.
- Integration: two instances in goroutines, connected via channel-based circuit, verify adjacency comes Up within ~2 Hellos.
- Timeout: stop sending Hellos, verify adjacency times out.

### Phase 4: LSPDB and LSP Origination

**Goal:** Store and originate LSPs.

**Deliverables:**
- LSPDB data structure (map by LSPID).
- Per-LSP SRM/SSN flag tracking.
- LSP origination: build LSPs from neighbor and route information.
- Lifetime decrement timer.
- LSP refresh timer.
- Sequence number management and wraparound handling.

**Test gate:**
- Insert LSPs, verify they're stored and retrieved.
- Decrement lifetime, verify LSPs are removed at 0.
- Originate an LSP, verify it has a valid sequence number and checksum.
- Refresh timer: originate, wait 900s, verify sequence incremented.

### Phase 5: LSP Flooding and CSNP/PSNP

**Goal:** Disseminate LSPs across the network.

**Deliverables:**
- Flooding logic: receive LSP, check freshness, set SRM/SSN on appropriate interfaces.
- CSNP periodic sender.
- PSNP sender (on SSN flags).
- CSNP receiver and processing.
- PSNP receiver and processing.

**Test gate:**
- Three-node line topology. Originate LSP on node A. Verify it floods to B, then to C.
- CSNP exchange: node A and B sync their LSPDBs via CSNP/PSNP.
- Newer LSP: send two versions of an LSP with different sequence numbers; verify the newer one replaces the older.

### Phase 6: SPF and Route Installation

**Goal:** Compute shortest paths and install routes.

**Deliverables:**
- Dijkstra algorithm over LSP graph.
- SPF trigger logic (on LSPDB change, debounced).
- Route computation per level (L1 and L2).
- Route installation API (call sysrib or equivalent).
- Metric handling (32-bit wide metrics).

**Test gate:**
- Unit test: manual LSPDB, run SPF, verify shortest paths match hand-computed results.
- Integration: three-node line, originate prefixes, run SPF, verify routes point in the right direction.
- Route installation: check that routes appear in the FIB after SPF completes.

### Phase 7: DIS Election (Broadcast Circuits)

**Goal:** Elect DIS and generate pseudo-node LSPs.

**Deliverables:**
- DIS election algorithm on broadcast interfaces.
- Pseudo-node LSP generation and origination.
- Pseudo-node declaration in own LSPs (no direct adjacencies, only to pseudo-node).
- DIS timeout and re-election.

**Test gate:**
- Three routers on a broadcast segment (simulated). Run election. Verify one router becomes DIS. Change DIS priority; verify new DIS is elected. Verify pseudo-node LSP is generated and reflects all routers.

### Phase 8: Authentication

**Goal:** Support HMAC-MD5 or other authentication.

**Deliverables:**
- TLV 10 auth decoding/encoding.
- Authentication type identification (cleartext, HMAC-MD5, HMAC-SHA-256, etc.).
- Key storage and retrieval.
- Auth verification on PDU receipt (check signature).
- Auth signature generation on PDU send.

**Test gate:**
- Send PDU with auth, receive with auth, verify signatures match.
- Change key; verify signature verification fails.
- Support key rotation (multiple valid keys).

### Phase 9: Configuration and Management

**Goal:** Expose CLI and configuration interfaces.

**Deliverables:**
- YANG schema for ISIS.
- Config parsing and application.
- CLI commands: `show isis neighbors`, `show isis database`, `show isis spf-results`.
- Counters and statistics.

**Test gate:**
- Configure ISIS via YANG; verify settings are applied.
- CLI commands return expected output.
- Statistics are populated.

### Phase 10: Interop Testing

**Goal:** Verify compatibility with other IS-IS implementations.

**Deliverables:**
- Lab setup with FRR (or another open-source IS-IS) and your implementation.
- Adjacency formation.
- LSP exchange and routing convergence.
- Failover and re-convergence.

**Test gate:**
- Adjacencies form with FRR neighbors.
- Routes converge correctly.
- No packet corruption or protocol violations (check Wireshark captures).

---

## 15. Code Organisation Suggestion for ze

### Package Layout

```
internal/component/isis/
├── isis.go                  # Plugin entry point, interface to ze
├── config.go                # Config parsing, YANG integration
├── types/
│   ├── types.go             # SystemID, SourceID, LSPID, NET, AreaID
│   └── types_test.go
├── packet/
│   ├── header.go            # ISISHeader codec
│   ├── hello.go             # IIH codec
│   ├── lsp.go               # LSP codec
│   ├── csnp.go              # CSNP codec
│   ├── psnp.go              # PSNP codec
│   ├── tlv.go               # TLV interface
│   ├── tlv_area.go          # TLV 1: Area Addresses
│   ├── tlv_extended_is.go   # TLV 22: Extended IS Reachability
│   ├── tlv_extended_ip.go   # TLV 135: Extended IP Reachability
│   ├── tlv_hostname.go      # TLV 137: Dynamic Hostname
│   ├── tlv_auth.go          # TLV 10: Authentication
│   ├── tlv_padding.go       # TLV 8: Padding
│   ├── tlv_lsp_entries.go   # TLV 9: LSP Entries
│   ├── tlv_p2p_adj.go       # TLV 240: P2P Adjacency State
│   ├── tlv_unknown.go       # TLV: Unknown/unsupported
│   └── packet_test.go       # Codec round-trip tests
├── neighbor/
│   ├── neighbor.go          # Neighbor/adjacency state machine
│   ├── manager.go           # Neighbor manager per interface
│   └── neighbor_test.go
├── circuit/
│   ├── circuit.go           # Circuit abstraction
│   ├── interface.go         # Per-interface logic (RX, TX, timers)
│   └── circuit_test.go
├── lspdb/
│   ├── lspdb.go             # LSPDB store and access
│   ├── entry.go             # LSPDB entry (LSP + SRM/SSN)
│   ├── aging.go             # Lifetime decrement, refresh logic
│   ├── flooding.go          # LSP flooding, CSNP/PSNP handling
│   └── lspdb_test.go
├── spf/
│   ├── spf.go               # Dijkstra algorithm
│   ├── graph.go             # Graph construction from LSPs
│   ├── route.go             # Route representation
│   └── spf_test.go
├── server.go                # Core server orchestration
├── config.yang              # YANG schema
├── cli.go                   # CLI command handlers
└── test/
    ├── testdata/            # Packet captures, test vectors
    └── fixtures.go          # Test helper functions
```

### Dependency Graph

```
isis (entry point)
  ├─> config              (parses YANG config)
  ├─> types               (domain types)
  ├─> packet              (PDU codec)
  ├─> neighbor            (uses packet, types)
  ├─> circuit             (uses neighbor, packet)
  ├─> lspdb               (uses packet, types)
  ├─> spf                 (uses lspdb, types)
  └─> cli                 (query-only, reads lspdb, neighbor, spf)

External dependencies:
  ├─> iface (interface management, TX/RX)
  ├─> sysrib (route installation)
  └─> config (YANG registration)
```

### Integration with ze

**Plugin Registration:**

In ze's plugin registry, IS-IS should register:
- Schema: YANG file for `/isis` path.
- Handlers: Config change, startup, shutdown.
- Dependencies: declare that it needs `iface` and `sysrib`.

**Config Application:**

When config is loaded or changed:
1. Parse the ISIS section.
2. Extract NETs, level, metrics, etc.
3. Call `server.Configure()` to apply settings.
4. Add/remove interfaces as needed.

**Startup:**

1. Create ISISServer instance with NETs and level.
2. Call `server.Start()` to spawn goroutines.
3. Register interface change subscriptions with the iface plugin.
4. Begin sending Hellos and accepting incoming PDUs.

**Shutdown:**

1. Call `server.Stop()` to gracefully tear down goroutines.
2. Clean up interface subscriptions.
3. Optionally send final purges to neighbors (graceful shutdown).

### Reference Architecture: bio-rd protocols/isis

bio-rd's `protocols/isis/` has a Go-idiomatic layout with four top-level directories:

| Path | Contents |
|------|----------|
| `api/` | gRPC API definition (`isis.proto`) and generated Go bindings |
| `packet/` | Wire codec: one file per PDU type (`hello.go`, `lsp.go`, `csnp.go`, `psnp.go`), a common `header.go`, and one file per TLV (`tlv_area_addresses.go`, `tlv_extended_is_reachability.go`, `tlv_extended_ip_reachability.go`, and roughly a dozen more). Every file has a matching `_test.go`. |
| `server/` | Runtime: `server.go` (top-level), `net_ifa.go` plus `net_ifa_rx.go`, `net_ifa_tx.go`, and `net_ifa_manager.go` (per-interface machinery), `neighbor.go` plus `neighbor_manager.go` (neighbour state), `lsdb.go` plus `lsdb_entry.go` and `lsp.go` (LSDB), `hello_sender.go` (periodic hello driver), `isis_api.go` (gRPC service glue) |
| `types/` | Domain types: `isis.go` (common constants), `net.go` (NET / System ID parsing), `area.go`, `circuit_type.go` |

Architectural takeaways:

1. **Per-PDU and per-TLV file split in `packet/`.** bio-rd takes "one file per packet type" further by adding one file per TLV. This matches IS-IS's TLV-based extensibility: new TLVs drop in as new files without touching existing ones. The cost is roughly 40 files in `packet/` for a moderately complete codec.
2. **Clear layering: `packet/` codec, `types/` domain types, `server/` runtime.** `packet/` never imports `server/`; `server/` imports `packet/` and `types/`; `types/` is a leaf. The idiomatic Go "package as layer" pattern.
3. **Per-interface machinery split into rx/tx/manager files.** `net_ifa_rx.go` owns the receive goroutine, `net_ifa_tx.go` the transmit goroutine, `net_ifa_manager.go` the per-interface lifecycle. This matches the way Go goroutines naturally partition per-interface work.
4. **gRPC as the operational API** rather than a CLI grammar. `api/isis.proto` defines the service; `isis_api.go` implements it. There is no VTY-style command parser.
5. **Tests live next to sources.** Every meaningful file has `_test.go`. Standard Go discipline, especially valuable for the packet codec round-trip tests.
6. **What is missing.** There is no `spf.go` in `server/` — bio-rd's IS-IS implements LSDB and flooding infrastructure but does not compute routes. L1 is stubbed. Authentication is minimal. These gaps are the ones already catalogued in §13 of this guide.

### Reference Architecture: FRR isisd

FRR's `isisd/` is roughly 70 files and 1.9 MB of C — comparable in size to ospfd — and is far more complete than bio-rd, covering essentially every IS-IS extension published:

| Area | Files (representative) | Role |
|------|------------------------|------|
| Top-level | `isisd.c` (103 kB), `isisd.h`, `isis_main.c` | Instance, config apply, daemon init |
| Circuit (interface) | `isis_circuit.c` (48 kB), `isis_circuit.h` | Per-interface circuit abstraction |
| Adjacency | `isis_adjacency.c` (25 kB), `isis_adjacency.h` | Neighbour state |
| State machines | `isis_csm.c`, `isis_events.c` | Circuit State Machine and event dispatch |
| DIS election | `isis_dr.c/h` | LAN DIS election |
| Wire codec | `isis_pdu.c` (75 kB), `isis_pdu.h`, `isis_tlvs.c` (239 kB), `isis_tlvs.h` | All PDU types and all TLVs in two monolithic files. `isis_tlvs.c` is the largest single source file in isisd. |
| LSP / LSDB | `isis_lsp.c` (67 kB), `isis_lsp.h` | LSP lifecycle, origination, fragmentation, aging |
| SPF | `isis_spf.c` (95 kB), `isis_spf.h`, `isis_spf_private.h` | Dijkstra plus multi-topology and flex-algo support |
| LFA / TI-LFA | `isis_lfa.c` (63 kB), `isis_lfa.h` | Fast reroute (RFC 5286-style plus TI-LFA) |
| Route install | `isis_route.c/h`, `isis_zebra.c` (44 kB) | Route computation and zebra integration |
| TX queue | `isis_tx_queue.c/h` | Per-circuit transmission queue |
| Redistribution | `isis_redist.c`, `isis_routemap.c` | External route import |
| Multi-topology | `isis_mt.c/h` | RFC 5120 |
| Flex-algo | `isis_flex_algo.c/h` | RFC 9350 |
| Segment Routing | `isis_sr.c` (37 kB), `isis_sr.h` | RFC 8667 |
| SRv6 | `isis_srv6.c` (23 kB), `isis_srv6.h` | RFC 9352 |
| Traffic Engineering | `isis_te.c` (61 kB), `isis_te.h` | TE extensions |
| Dynamic hostname | `isis_dynhn.c/h` | RFC 5301 |
| Affinity map | `isis_affinitymap.c/h` | Administrative-group naming |
| BFD integration | `isis_bfd.c/h` | Hook into bfdd |
| LDP sync | `isis_ldp_sync.c/h` | RFC 6138 |
| Checksum | `iso_checksum.c/h` | Fletcher-16 (shared with OSPF) |
| Raw-socket backends | `isis_bpf.c`, `isis_dlpi.c`, `isis_pfpacket.c` | BSD BPF, Solaris DLPI, Linux `AF_PACKET` |
| Fabricd | `fabricd.c/h`, `isis_vty_fabricd.c` | OpenFabric variant |
| YANG northbound | `isis_nb.c/h`, `isis_nb_config.c` (120 kB), `isis_nb_state.c`, `isis_nb_notifications.c` | Most advanced YANG migration in FRR |
| CLI | `isis_cli.c` (126 kB) | YANG-derived CLI |
| SNMP | `isis_snmp.c` (87 kB) | IS-IS MIB |

Architectural takeaways:

1. **Two monolithic codec files.** `isis_pdu.c` and `isis_tlvs.c` are each bigger than most entire protocol implementations. FRR went the opposite way from bio-rd and BIRD's per-type/per-TLV split. The single-file approach localises TLV registration macros but produces enormous source files.
2. **One file per major extension.** TE, SR, SRv6, flex-algo, MT, LFA, BFD, LDP-sync — each its own pair of files. This is the same house style as FRR ospfd.
3. **Three raw-socket back ends** — `isis_pfpacket.c`, `isis_bpf.c`, `isis_dlpi.c` — because IS-IS runs directly over L2 and each platform's raw-frame API is different. ze only needs Linux `AF_PACKET` for a first pass but should isolate it behind an interface.
4. **Fabricd is a sibling top-level** (`fabricd.c`, `fabricd.h`, `isis_vty_fabricd.c`) because OpenFabric is an IS-IS variant with different defaults and CLI verbs. It lives next to isisd but is a distinct protocol.
5. **YANG northbound is much more complete than for ospfd or bfdd.** `isis_nb_config.c` at 120 kB is evidence that isisd's YANG migration is the furthest along in FRR. ze can draw directly from the IETF `ietf-isis` YANG module.
6. **SPF is 95 kB but much of that is multi-topology and flex-algo bookkeeping.** The core Dijkstra is modest. TI-LFA adds another 63 kB in `isis_lfa.c`.

### BIRD Does Not Implement IS-IS

BIRD's stable releases (including 3.2.1, the latest at time of writing) do **not** ship an IS-IS protocol. BIRD upstream has an `isis` branch at `refs/heads/isis` whose latest commit is **"Some preliminary IS-IS commit", 2012-08-29**. The branch has been idle for more than a decade and was never merged. It predates BIRD's BFD support, and its protocol list reflects BIRD 1.3-era contents (bgp, isis, ospf, pipe, radv, rip, static — no BFD, no Babel).

Practically: **if you want an IS-IS reference implementation to study, your choices are FRR's `isisd/` (C, feature-complete) and bio-rd's `protocols/isis/` (Go, partial).** Do not spend time on the BIRD branch.

### Lessons for ze

- **From bio-rd: one file per TLV in `packet/`.** IS-IS TLVs are numerous and independent, and a per-TLV file layout makes each extension self-contained. ze's `packet/tlv_*.go` organisation in §15 follows this directly.
- **From bio-rd: per-interface goroutine split into rx/tx/manager.** The natural Go concurrency pattern for per-interface work.
- **From bio-rd: tests next to sources with `_test.go`.** Standard Go discipline, especially high-payoff for packet codecs.
- **From FRR: file-per-extension.** SR, TE, MT, flex-algo, LFA, BFD, LDP-sync each in their own file. Enables piecewise development and easy exclusion.
- **From FRR: raw-socket backend isolated behind an interface.** Even if ze only targets Linux `AF_PACKET` now, the abstraction keeps the door open for BSD or Solaris back ends later.
- **From neither: the monolith-vs-scatter codec decision.** bio-rd scatters aggressively (40 TLV files for a codec that is still incomplete); FRR monoliths aggressively (a 239 kB TLV file). ze's §15 layout splits by TLV family (core, opaque, TE, SR) rather than one file per individual TLV — a middle path that keeps file counts manageable while preserving modularity.
- **From the absence: BIRD's non-implementation is informative.** A mature, multi-protocol routing daemon deliberately decided IS-IS was not worth building, because the community maintaining BIRD has no production IS-IS users. This is a useful gut check: ze should ship IS-IS only if ze users actually need it, not for symmetry with FRR.

---

## 16. Summary and Next Steps

This document provides a complete architecture for a clean-room IS-IS implementation. It covers:

1. **Protocol fundamentals**: PDU types, addressing, two levels, DIS election, flooding, adjacency management.
2. **Wire format**: Exact field layouts, TLV registry, encoding/decoding strategies.
3. **Core algorithms**: Adjacency FSM, LSP flooding, SPF (Dijkstra).
4. **Implementation patterns**: Goroutine concurrency, LSPDB design, circuit management.
5. **Configuration and management**: YANG schema, CLI, plugin integration.
6. **Testing and debugging**: Unit, integration, interop, and fuzz testing.
7. **Known pitfalls**: Checksums, sequence number wraparound, MTU detection, etc.

### Reference RFCs (In Order of Importance)

1. **RFC 10589**: IS-IS (normative, 2024 update to ISO 10589).
2. **RFC 5305**: Extensions to IS-IS for Wide Metric support (IPv4).
3. **RFC 5308**: Extensions to IS-IS for IPv6 Routing.
4. **RFC 5304**: IS-IS Cryptographic Authentication (HMAC-MD5).
5. **RFC 5310**: IS-IS Generic Cryptographic Authentication (HMAC-SHA-256, etc.).
6. **RFC 5303**: Three-Way Adjacency for Point-to-Point IS-IS.
7. **RFC 5120**: M-ISIS: Multi Topology (MTR) Routing in IS-IS.
8. **RFC 5306**: IS-IS Graceful Restart.
9. **RFC 5301**: Dynamic Hostname Exchange Mechanism for IS-IS.
10. **RFC 7981**: IS-IS Extensions for Advertising Router Information.
11. **RFC 3787**: IS-IS Restart Signaling (Overload Bit).
12. **RFC 2966**: IS-IS extensions for routing IPv4 (up/down bit).
13. **RFC 5880, RFC 7130**: BFD for IS-IS.
14. **RFC 8667**: IS-IS Extensions in Support of Segment Routing.

### Implementation Priorities

- **Must have**: PDU codec, adjacency FSM, LSPDB, flooding, SPF, route installation.
- **Should have**: L1 support, DIS election, authentication, IPv6 routing.
- **Nice to have**: BFD, graceful restart, segment routing, TE.

Start with Phase 1–6 (types through SPF). Validate with two-node adjacency and routing. Add L1 and DIS (Phases 7–8) once L2 is stable. Leave advanced features (BFD, SR, MT) for later.

Good luck with the implementation. Test thoroughly, especially checksums, sequence number handling, and interop with FRR or IOS-XR.