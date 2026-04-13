# L2TPv2 Clean-Room Implementation Guide

Clean-room protocol specification for implementing an L2TPv2 LNS/LAC in Go.
Derived exclusively from RFC 2661, RFC 3931 (for detection/rejection), and
the Linux kernel L2TP subsystem documentation.

Target: zero-bug day-one implementation. Every byte offset, every state
transition, every timer value, every edge case.

---

## Table of Contents

1. [Protocol Overview](#1-protocol-overview)
2. [Transport Layer](#2-transport-layer)
3. [L2TP Header Format](#3-l2tp-header-format)
4. [AVP Format](#4-avp-format)
5. [AVP Catalog](#5-avp-catalog)
6. [Control Message Types](#6-control-message-types)
7. [Required AVPs Per Message](#7-required-avps-per-message)
8. [Result Codes and Error Codes](#8-result-codes-and-error-codes)
9. [Tunnel State Machine](#9-tunnel-state-machine)
10. [Session State Machines](#10-session-state-machines)
11. [Reliable Delivery](#11-reliable-delivery)
12. [Slow Start and Congestion Control](#12-slow-start-and-congestion-control)
13. [Challenge/Response Authentication](#13-challengeresponse-authentication)
14. [Hidden AVP Encryption](#14-hidden-avp-encryption)
15. [Hello (Keepalive)](#15-hello-keepalive)
16. [Data Messages and PPP Encapsulation](#16-data-messages-and-ppp-encapsulation)
17. [Data Sequencing](#17-data-sequencing)
18. [Proxy LCP and Proxy Authentication](#18-proxy-lcp-and-proxy-authentication)
19. [WAN Error Notify and Set Link Info](#19-wan-error-notify-and-set-link-info)
20. [L2TPv3 Detection and Rejection](#20-l2tpv3-detection-and-rejection)
21. [Linux Kernel L2TP Subsystem](#21-linux-kernel-l2tp-subsystem)
22. [PPP-over-L2TP Operational Details](#22-ppp-over-l2tp-operational-details)
23. [Timer Values](#23-timer-values)
24. [Implementation Traps and Edge Cases](#24-implementation-traps-and-edge-cases)
25. [Security Considerations](#25-security-considerations)
26. [Go Implementation Notes](#26-go-implementation-notes)

---

## 1. Protocol Overview

L2TPv2 (Layer Two Tunneling Protocol, version 2) tunnels PPP sessions over
UDP/IP. It separates the concepts of **tunnel** (control connection between
two L2TP peers) and **session** (individual PPP link within a tunnel).

### 1.1 Roles

| Role | Name | Function |
|------|------|----------|
| LAC | L2TP Access Concentrator | Initiates tunnels/sessions. Forwards PPP frames from remote users into the tunnel. |
| LNS | L2TP Network Server | Terminates tunnels/sessions. Terminates PPP, assigns IP addresses, provides network access. |

A single implementation can act as both LAC and LNS simultaneously.

### 1.2 Protocol Layers

```
Remote User <-> [PPP] <-> LAC <-> [L2TP/UDP/IP] <-> LNS <-> [PPP termination] <-> Network
```

### 1.3 Relationship: Tunnels and Sessions

- One tunnel = one control connection between two peers.
- One tunnel can carry multiple sessions (multiplexed by Session ID).
- Each session = one PPP link.
- Tunnel teardown tears down all sessions within it.
- Session teardown does not affect the tunnel or other sessions.
- Multiple tunnels can exist between the same two peers.

### 1.4 Message Types

Two categories:
- **Control messages** (T=1): reliable delivery, sequenced, AVP-encoded payloads.
- **Data messages** (T=0): unreliable by default, carry PPP frames.

---

## 2. Transport Layer

### 2.1 UDP Encapsulation

- IANA registered port: **1701** (shared with L2F; distinguish by header Version field).
- The initiator sends from any source port to destination port 1701.
- The responder sends from any source port to the initiator's source port.
- Source and destination ports MUST remain constant for the lifetime of the tunnel.

### 2.2 UDP Checksums

- Control messages: UDP checksum MUST be enabled. Non-negotiable.
- Data messages: UDP checksum SHOULD be enabled by default. MAY provide a
  configuration option to disable (performance optimization).
- Receiver: MUST validate checksums when present. A checksum of 0 means
  "not computed" (UDP spec); accept the packet.

### 2.3 IP Fragmentation

L2TP does not handle fragmentation itself. The implementation should:
- Negotiate appropriate MRU via PPP LCP to avoid fragmentation.
- Set the DF (Don't Fragment) bit and handle ICMP "Fragmentation Needed"
  by adjusting MRU, or leave DF clear and let IP fragment.
- Account for overhead: UDP(8) + L2TP header(12 control, 6-14 data) +
  PPP protocol field(2) when computing effective MTU.

### 2.4 Overhead Calculation

```
Ethernet MTU:                          1500 bytes
- IP header:                             20 bytes
- UDP header:                             8 bytes
- L2TP data header (minimal):             6 bytes (flags+ver, tunnel ID, session ID)
- L2TP data header (with length):         8 bytes
- L2TP data header (with length+seq):    12 bytes
- PPP protocol field:                      2 bytes (1 if PFC negotiated)
= Available for PPP payload:    1464-1470 bytes (without IP options)

Typical PPP MRU for L2TP:             1460 bytes (conservative)
accel-ppp default ppp-max-mtu:         1420 bytes
```

---

## 3. L2TP Header Format

All multi-byte fields are big-endian (network byte order).

### 3.1 Full Header Layout

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|T|L|x|x|S|x|O|P|x|x|x|x|  Ver  |          Length (opt)        |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|           Tunnel ID           |           Session ID          |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|             Ns (opt)          |             Nr (opt)          |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|      Offset Size (opt)        |    Offset Pad... (opt)       |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

### 3.2 Flags and Version (bytes 0-1)

| Bit | Name | Meaning |
|-----|------|---------|
| 0 | T (Type) | 0 = data message, 1 = control message |
| 1 | L (Length) | 1 = Length field present. MUST be 1 for control messages. |
| 2 | x | Reserved. MUST be 0 on send. MUST be ignored on receive. |
| 3 | x | Reserved. MUST be 0 on send. MUST be ignored on receive. |
| 4 | S (Sequence) | 1 = Ns and Nr fields present. MUST be 1 for control messages. |
| 5 | x | Reserved. MUST be 0 on send. MUST be ignored on receive. |
| 6 | O (Offset) | 1 = Offset Size field present. MUST be 0 for control messages. |
| 7 | P (Priority) | 1 = preferential treatment for this data message. MUST be 0 for control. |
| 8-11 | x | Reserved. MUST be 0. |
| 12-15 | Ver | Protocol version. MUST be 2 for L2TPv2. Value 1 = L2F. Value 3 = L2TPv3. |

### 3.3 Control Message Header (Fixed Format)

Control messages always have T=1, L=1, S=1, O=0, P=0, Ver=2.

Bytes 0-1 are always `0xC802`:
```
Binary: 1100 1000 0000 0010
        T=1, L=1, x=0, x=0, S=1, x=0, O=0, P=0, x=0,0,0,0, Ver=0010
```

Fixed size: 12 bytes.

```
Byte  0-1:  0xC802 (flags + version)
Byte  2-3:  Length (total message length including this header, in octets)
Byte  4-5:  Tunnel ID (recipient's assigned tunnel ID)
Byte  6-7:  Session ID (recipient's assigned session ID, 0 for tunnel-scoped messages)
Byte  8-9:  Ns (sequence number of this message)
Byte 10-11: Nr (sequence number expected from peer)
```

### 3.4 Data Message Header (Variable Format)

Minimum 6 bytes (T=0, L=0, S=0, O=0):

```
Byte 0-1: Flags + Version (T=0, Ver=2; L/S/O/P optional)
Byte 2-3: Tunnel ID
Byte 4-5: Session ID
```

With all optional fields (L=1, S=1, O=1): up to 14 bytes + offset padding.

```
Byte  0-1:  Flags + Version
Byte  2-3:  Length (if L=1)
Byte  N:    Tunnel ID (offset depends on L)
Byte  N+2:  Session ID
Byte  N+4:  Ns (if S=1)
Byte  N+6:  Nr (if S=1) -- RESERVED in data messages, MUST be ignored
Byte  N+8:  Offset Size (if O=1)
Byte  N+10: Offset Pad (Offset Size bytes of undefined content)
```

### 3.5 Parsing Algorithm

```
1. Read bytes 0-1.
2. Extract T (bit 0), L (bit 1), S (bit 4), O (bit 6), P (bit 7), Ver (bits 12-15).
3. If Ver != 2: discard (or handle L2F/L2TPv3 detection).
4. offset = 2
5. If L=1: Length = bytes[offset:offset+2]; offset += 2
6. Tunnel ID = bytes[offset:offset+2]; offset += 2
7. Session ID = bytes[offset:offset+2]; offset += 2
8. If S=1: Ns = bytes[offset:offset+2]; Nr = bytes[offset+2:offset+4]; offset += 4
9. If O=1: OffsetSize = bytes[offset:offset+2]; offset += 2; offset += OffsetSize
10. Payload starts at offset.
```

### 3.6 ID Semantics

- **Tunnel ID** in the header is always the **recipient's** assigned tunnel ID.
  The sender uses the ID that the peer assigned to itself via the
  Assigned Tunnel ID AVP.
- **Session ID** follows the same convention: the recipient's assigned session ID.
- **ID 0** is reserved and never assigned. It is used:
  - In the first SCCRQ (before the peer has assigned a tunnel ID).
  - For tunnel-scoped messages where no session is relevant (HELLO, StopCCN).
  - In CDN when the sender has not yet received the peer's session ID.

---

## 4. AVP Format

### 4.1 Structure

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|M|H|rsvd |        Length       |           Vendor ID           |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|         Attribute Type        |        Attribute Value...     |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

| Field | Bits | Description |
|-------|------|-------------|
| M (Mandatory) | bit 0 | 1 = if AVP is unrecognized, terminate the session or tunnel |
| H (Hidden) | bit 1 | 1 = Attribute Value is encrypted (section 14) |
| Reserved | bits 2-5 | MUST be 0 on send. If non-zero on receive, treat as unrecognized AVP. |
| Length | bits 6-15 | Total AVP length in octets including this 6-byte header. Min=6, Max=1023. |
| Vendor ID | bytes 2-3 | 0 = IETF standard attributes. Non-zero = SMI Private Enterprise Number. |
| Attribute Type | bytes 4-5 | Attribute identifier, unique within a Vendor ID. |
| Attribute Value | bytes 6+ | (Length - 6) octets of data. |

### 4.2 Processing Rules

1. **Message Type AVP** (type 0) MUST be the first AVP in every control message.
2. If M=1 and AVP is unrecognized:
   - If the message is session-scoped: send CDN, tear down session.
   - If the message is tunnel-scoped: send StopCCN, tear down tunnel.
   - Use Result Code 2 (general error), Error Code 8 (unknown mandatory AVP).
3. If M=0 and AVP is unrecognized: silently ignore, continue processing remaining AVPs.
4. If reserved bits (2-5) are non-zero: treat the entire AVP as unrecognized and
   apply the M-bit rule above.
5. If Length < 6: malformed. Drop the message (cannot determine AVP boundaries).
6. If Length extends beyond the message: malformed. Drop the message.

### 4.3 Value Encoding

| Go type | Wire encoding |
|---------|---------------|
| uint8 | 1 byte |
| uint16 | 2 bytes, big-endian |
| uint32 | 4 bytes, big-endian |
| uint64 | 8 bytes, big-endian |
| string | UTF-8 bytes, no null terminator |
| octets | raw bytes |

---

## 5. AVP Catalog

All AVPs below have Vendor ID = 0 (IETF standard).

### 5.1 Message Type (Type 0)

- **Value**: uint16 identifying the control message type.
- **M**: MUST be 1.
- **H**: MUST be 0 (never hidden).
- **Length**: 8.
- **Present in**: every control message, as the first AVP.
- **Values**: see section 6.

### 5.2 Result Code (Type 1)

- **Value**: compound structure:
  ```
  Bytes 0-1:  Result Code (uint16)
  Bytes 2-3:  Error Code (uint16) -- present only if Result Code = 2 (General Error)
  Bytes 4+:   Error Message (UTF-8 string, optional, advisory)
  ```
- **M**: MUST be 1.
- **H**: MAY be hidden.
- **Length**: 8 (result only), 10 (result + error), 10+N (result + error + message).
- **Present in**: StopCCN, CDN.

### 5.3 Protocol Version (Type 2)

- **Value**: 2 bytes, encoding `[0x01, 0x00]`:
  ```
  Byte 0: Version = 1
  Byte 1: Revision = 0
  Wire bytes: 0x01 0x00
  ```
- **M**: MUST be 1.
- **H**: MUST be 0.
- **Length**: 8.
- **Present in**: SCCRQ, SCCRP (mandatory).

**IMPORTANT**: Two different "version" numbers coexist in L2TPv2. They are
not the same thing:
- **Header Version field** (bits 12-15 of byte 0-1) = **2**. Identifies the
  framing format (distinguishes L2TP from L2F which uses 1, and L2TPv3 which uses 3).
- **Protocol Version AVP value** = **1.0** (wire bytes `0x01 0x00`). Identifies
  the L2TP specification version (RFC 2661).

An implementation must send header Ver=2 and Protocol Version AVP=1.0. If a
peer sends Protocol Version AVP with Ver != 1 or Rev != 0, reject with
StopCCN Result Code 5.

### 5.4 Framing Capabilities (Type 3)

- **Value**: uint32 bitmask. RFC 2661 numbers bits from the LSB (bit 0 = LSB).
  ```
  Bit 0 (0x00000001): Asynchronous framing supported
  Bit 1 (0x00000002): Synchronous framing supported
  Bits 2-31: reserved, must be 0
  ```
- **M**: MUST be 1.
- **H**: MAY be hidden.
- **Length**: 10.
- **Present in**: SCCRQ, SCCRP (mandatory).

### 5.5 Bearer Capabilities (Type 4)

- **Value**: uint32 bitmask. Same bit numbering as Framing Capabilities.
  ```
  Bit 0 (0x00000001): Analog bearer supported
  Bit 1 (0x00000002): Digital bearer supported
  Bits 2-31: reserved, must be 0
  ```
- **M**: MUST be 1.
- **H**: MAY be hidden.
- **Length**: 10.
- **Present in**: SCCRQ, SCCRP (optional; mandatory if bearer is relevant).

### 5.6 Tie Breaker (Type 5)

- **Value**: uint64 (8 random bytes).
- **M**: SHOULD be 0.
- **H**: MUST be 0.
- **Length**: 14.
- **Present in**: SCCRQ only (optional).
- **Purpose**: resolve simultaneous SCCRQ. See section 9.3.

### 5.7 Firmware Revision (Type 6)

- **Value**: uint16 (vendor-specific encoding).
- **M**: MUST be 0.
- **H**: MAY be hidden.
- **Length**: 8.
- **Present in**: SCCRQ, SCCRP (optional, informational).

### 5.8 Host Name (Type 7)

- **Value**: string, minimum 1 byte. Identifies the sending peer.
- **M**: MUST be 1.
- **H**: MUST be 0 (used in challenge response computation).
- **Length**: 7+ (6-byte header + at least 1 byte of value).
- **Present in**: SCCRQ, SCCRP (mandatory).

### 5.9 Vendor Name (Type 8)

- **Value**: UTF-8 string (human-readable vendor identifier).
- **M**: MUST be 0.
- **H**: MAY be hidden.
- **Length**: 6+N.
- **Present in**: SCCRQ, SCCRP (optional, informational).

### 5.10 Assigned Tunnel ID (Type 9)

- **Value**: uint16, non-zero. The tunnel ID assigned by the sender for use
  in subsequent messages directed to this tunnel.
- **M**: MUST be 1.
- **H**: MAY be hidden.
- **Length**: 8.
- **Present in**: SCCRQ, SCCRP (mandatory), StopCCN (mandatory).
- **In StopCCN**: the sender includes its own previously-assigned tunnel ID
  so the recipient can identify the tunnel being closed, even if the tunnel
  was never fully established.

### 5.11 Receive Window Size (Type 10)

- **Value**: uint16, non-zero. Number of control messages the sender can
  buffer. Peer MUST NOT have more than this many unacknowledged messages
  outstanding.
- **M**: MUST be 1.
- **H**: MUST be 0.
- **Length**: 8.
- **Present in**: SCCRQ, SCCRP (optional; default = 4 if absent).
- **Constraint**: value MUST be non-zero. An implementation MUST support
  receiving a window of at least 4.

### 5.12 Challenge (Type 11)

- **Value**: arbitrary random bytes (length is variable, minimum 1 byte).
- **M**: MUST be 1.
- **H**: MAY be hidden.
- **Length**: 7+.
- **Present in**: SCCRQ, SCCRP (optional; presence triggers authentication).

### 5.13 Q.931 Cause Code (Type 12)

- **Value**: compound structure:
  ```
  Bytes 0-1: Cause Code (uint16, ITU-T Q.931 cause value)
  Byte  2:   Cause Msg (uint8, Q.931 message type)
  Bytes 3+:  Advisory Message (ASCII string, optional)
  ```
- **M**: MUST be 1.
- **H**: MUST be 0.
- **Length**: 9+N.
- **Present in**: CDN (optional, LAC only, for PSTN/ISDN calls).

### 5.14 Challenge Response (Type 13)

- **Value**: exactly 16 bytes (MD5 hash output).
- **M**: MUST be 1.
- **H**: MAY be hidden.
- **Length**: 22.
- **Present in**: SCCRP (if SCCRQ contained Challenge), SCCCN (if SCCRP contained Challenge).
- **Computation**: see section 13.

### 5.15 Assigned Session ID (Type 14)

- **Value**: uint16, non-zero. The session ID assigned by the sender.
- **M**: MUST be 1.
- **H**: MAY be hidden.
- **Length**: 8.
- **Present in**: ICRQ, ICRP, OCRQ, OCRP (mandatory), CDN (mandatory).
- **In CDN**: the sender includes its own assigned session ID so the
  recipient can identify the session, even before full ID exchange.

### 5.16 Call Serial Number (Type 15)

- **Value**: uint32. Globally unique identifier assigned by the LAC
  for this call. Used for correlation across tunnels.
- **M**: MUST be 1.
- **H**: MAY be hidden.
- **Length**: 10.
- **Present in**: ICRQ, OCRQ (mandatory).

### 5.17 Minimum BPS (Type 16)

- **Value**: uint32. Minimum line speed in bits per second acceptable for the call.
- **M**: MUST be 1.
- **H**: MAY be hidden.
- **Length**: 10.
- **Present in**: OCRQ (mandatory).

### 5.18 Maximum BPS (Type 17)

- **Value**: uint32. Maximum line speed in bits per second for the call.
- **M**: MUST be 1.
- **H**: MAY be hidden.
- **Length**: 10.
- **Present in**: OCRQ (mandatory).

### 5.19 Bearer Type (Type 18)

- **Value**: uint32 bitmask (same layout as Bearer Capabilities, type 4).
- **M**: MUST be 1.
- **H**: MAY be hidden.
- **Length**: 10.
- **Present in**: ICRQ (mandatory), OCRQ (mandatory).

### 5.20 Framing Type (Type 19)

- **Value**: uint32 bitmask (same layout as Framing Capabilities, type 3).
- **M**: MUST be 1.
- **H**: MAY be hidden.
- **Length**: 10.
- **Present in**: ICCN, OCCN (mandatory), OCRQ (mandatory).

### 5.21 Called Number (Type 21)

- **Value**: ASCII string. The number being called (DNIS).
- **M**: MUST be 1.
- **H**: MAY be hidden.
- **Length**: 6+N (may be empty, length=6).
- **Present in**: ICRQ (mandatory), OCRQ (mandatory).

### 5.22 Calling Number (Type 22)

- **Value**: ASCII string. The calling party's number (ANI/CLI).
- **M**: MUST be 1.
- **H**: MAY be hidden.
- **Length**: 6+N (may be empty).
- **Present in**: ICRQ (mandatory).

### 5.23 Sub-Address (Type 23)

- **Value**: ASCII string. Additional dialing information.
- **M**: MUST be 1.
- **H**: MAY be hidden.
- **Length**: 6+N.
- **Present in**: ICRQ (optional), OCRQ (optional).

### 5.24 Tx Connect Speed (Type 24)

- **Value**: uint32. Connected speed in BPS from LAC to remote system.
- **M**: MUST be 1.
- **H**: MAY be hidden.
- **Length**: 10.
- **Present in**: ICCN, OCCN (mandatory).

### 5.25 Physical Channel ID (Type 25)

- **Value**: uint32. Vendor-specific physical channel identifier at the LAC.
- **M**: MUST be 0.
- **H**: MAY be hidden.
- **Length**: 10.
- **Present in**: ICRQ (optional), OCRP (optional).

### 5.26 Initial Received LCP CONFREQ (Type 26)

- **Value**: raw LCP Configuration Request options (the body after the
  PPP LCP header, i.e., just the options, no Code/ID/Length fields).
- **M**: MUST be 0.
- **H**: MAY be hidden.
- **Length**: 6+N.
- **Present in**: ICCN (optional, proxy LCP).
- **Purpose**: first LCP CONFREQ received from the remote peer.

### 5.27 Last Sent LCP CONFREQ (Type 27)

- **Value**: raw LCP options (same format as type 26).
- **M**: MUST be 0.
- **H**: MAY be hidden.
- **Length**: 6+N.
- **Present in**: ICCN (optional, proxy LCP).
- **Purpose**: the last LCP CONFREQ the LAC sent to the remote peer.

### 5.28 Last Received LCP CONFREQ (Type 28)

- **Value**: raw LCP options (same format as type 26).
- **M**: MUST be 0.
- **H**: MAY be hidden.
- **Length**: 6+N.
- **Present in**: ICCN (optional, proxy LCP).
- **Purpose**: the last LCP CONFREQ received from the remote peer (which
  the LAC ACKed). This represents the negotiated LCP parameters.

### 5.29 Proxy Authen Type (Type 29)

- **Value**: uint16 identifying the authentication protocol used.
  ```
  0 = Reserved
  1 = Textual username/password exchange
  2 = PPP CHAP
  3 = PPP PAP
  4 = No Authentication
  5 = Microsoft CHAP Version 1 (MS-CHAPv1)
  ```
- **M**: MUST be 0.
- **H**: MAY be hidden.
- **Length**: 8.
- **Present in**: ICCN (optional, proxy authentication).

### 5.30 Proxy Authen Name (Type 30)

- **Value**: string. The username from the authentication exchange.
- **M**: MUST be 0.
- **H**: MAY be hidden.
- **Length**: 6+N.
- **Present in**: ICCN (optional, with proxy authen).

### 5.31 Proxy Authen Challenge (Type 31)

- **Value**: raw bytes. The CHAP challenge sent by the LAC.
- **M**: MUST be 0.
- **H**: MAY be hidden.
- **Length**: 6+N.
- **Present in**: ICCN (optional, with proxy authen type = CHAP).

### 5.32 Proxy Authen ID (Type 32)

- **Value**: 2 bytes.
  ```
  Byte 0: Reserved (0)
  Byte 1: CHAP ID used in the authentication exchange
  ```
- **M**: MUST be 0.
- **H**: MAY be hidden.
- **Length**: 8.
- **Present in**: ICCN (optional, with proxy authen).

### 5.33 Proxy Authen Response (Type 33)

- **Value**: raw bytes. The authentication response from the remote peer.
- **M**: MUST be 0.
- **H**: MAY be hidden.
- **Length**: 6+N.
- **Present in**: ICCN (optional, with proxy authen).

### 5.34 Call Errors (Type 34)

- **Value**: 26 bytes, fixed layout:
  ```
  Bytes  0-1:  Reserved (0)
  Bytes  2-5:  CRC Errors (uint32)
  Bytes  6-9:  Framing Errors (uint32)
  Bytes 10-13: Hardware Overruns (uint32)
  Bytes 14-17: Buffer Overruns (uint32)
  Bytes 18-21: Time-out Errors (uint32)
  Bytes 22-25: Alignment Errors (uint32)
  ```
- **M**: MUST be 1.
- **H**: MAY be hidden.
- **Length**: 32.
- **Present in**: WEN (mandatory).

### 5.35 ACCM (Type 35)

- **Value**: 10 bytes, fixed layout:
  ```
  Bytes 0-1: Reserved (0)
  Bytes 2-5: Send ACCM (uint32) -- the ACCM to use when sending to the remote
  Bytes 6-9: Receive ACCM (uint32) -- the ACCM expected from the remote
  ```
  Default ACCM (if SLI never sent): 0xFFFFFFFF for both.
- **M**: MUST be 1.
- **H**: MAY be hidden.
- **Length**: 16.
- **Present in**: SLI (mandatory).

### 5.36 Random Vector (Type 36)

- **Value**: arbitrary random bytes (recommended minimum 16 bytes).
- **M**: MUST be 1.
- **H**: MUST be 0 (never hidden; it is used to decrypt hidden AVPs).
- **Length**: 6+N.
- **Present in**: any control message that contains hidden AVPs. MUST precede
  the first hidden AVP in the message.
- **Purpose**: provides the Random Vector for the hidden AVP encryption algorithm.

### 5.37 Private Group ID (Type 37)

- **Value**: string. Identifies a private group for the session.
- **M**: MUST be 0.
- **H**: MAY be hidden.
- **Length**: 6+N.
- **Present in**: ICCN (optional).

### 5.38 Rx Connect Speed (Type 38)

- **Value**: uint32. Receive speed in BPS (remote to LAC direction).
  If absent, assume same as Tx Connect Speed.
- **M**: MUST be 0.
- **H**: MAY be hidden.
- **Length**: 10.
- **Present in**: ICCN, OCCN (optional).

### 5.39 Sequencing Required (Type 39)

- **Value**: none (presence alone signals the requirement).
- **M**: MUST be 1.
- **H**: MUST be 0.
- **Length**: 6 (header only, no value bytes).
- **Present in**: ICCN, OCCN (optional).
- **Semantics**: data messages for this session MUST always include
  sequence numbers (S=1 with Ns/Nr fields).

---

## 6. Control Message Types

| Value | Mnemonic | Full Name | Scope |
|-------|----------|-----------|-------|
| 1 | SCCRQ | Start-Control-Connection-Request | Tunnel |
| 2 | SCCRP | Start-Control-Connection-Reply | Tunnel |
| 3 | SCCCN | Start-Control-Connection-Connected | Tunnel |
| 4 | StopCCN | Stop-Control-Connection-Notification | Tunnel |
| 5 | (reserved) | | |
| 6 | HELLO | Hello (keepalive) | Tunnel |
| 7 | OCRQ | Outgoing-Call-Request | Session |
| 8 | OCRP | Outgoing-Call-Reply | Session |
| 9 | OCCN | Outgoing-Call-Connected | Session |
| 10 | ICRQ | Incoming-Call-Request | Session |
| 11 | ICRP | Incoming-Call-Reply | Session |
| 12 | ICCN | Incoming-Call-Connected | Session |
| 13 | (reserved) | | |
| 14 | CDN | Call-Disconnect-Notify | Session |
| 15 | WEN | WAN-Error-Notify | Session |
| 16 | SLI | Set-Link-Info | Session |

**Tunnel-scoped** messages use Session ID = 0 in the L2TP header.
**Session-scoped** messages use the recipient's Assigned Session ID.

### 6.1 Message Flow: Incoming Call (LAC-initiated)

```
LAC                                   LNS
 |                                     |
 |--- SCCRQ (TID=0) ----------------->|   Tunnel setup
 |<-- SCCRP (TID=LAC's assigned) -----|
 |--- SCCCN (TID=LNS's assigned) ---->|
 |                                     |
 |--- ICRQ (SID=0) ------------------>|   Session setup
 |<-- ICRP (SID=LAC's assigned) ------|
 |--- ICCN (SID=LNS's assigned) ----->|
 |                                     |
 |<========= PPP data ===============>|   Data flow
 |                                     |
 |--- CDN (SID=LNS's assigned) ------>|   Session teardown
 |  or                                 |
 |<-- CDN (SID=LAC's assigned) -------|
 |                                     |
 |--- StopCCN (TID=LNS's assigned) -->|   Tunnel teardown
 |  or                                 |
 |<-- StopCCN (TID=LAC's assigned) ---|
```

### 6.2 Message Flow: Outgoing Call (LNS-initiated)

```
LNS                                   LAC
 |                                     |
 |  (tunnel already established)       |
 |                                     |
 |--- OCRQ (SID=0) ------------------>|   Session setup
 |<-- OCRP (SID=LNS's assigned) ------|
 |<-- OCCN (SID=LNS's assigned) ------|
 |                                     |
 |<========= PPP data ===============>|   Data flow
```

---

## 7. Required AVPs Per Message

### 7.1 SCCRQ (type 1)

| AVP | Required | Notes |
|-----|----------|-------|
| Message Type (0) | MUST | Value = 1 |
| Protocol Version (2) | MUST | Value = 0x0100 |
| Host Name (7) | MUST | |
| Framing Capabilities (3) | MUST | |
| Assigned Tunnel ID (9) | MUST | |
| Bearer Capabilities (4) | MAY | |
| Receive Window Size (10) | MAY | Default 4 if absent |
| Challenge (11) | MAY | Triggers authentication |
| Tie Breaker (5) | MAY | For simultaneous open resolution |
| Firmware Revision (6) | MAY | |
| Vendor Name (8) | MAY | |

### 7.2 SCCRP (type 2)

| AVP | Required | Notes |
|-----|----------|-------|
| Message Type (0) | MUST | Value = 2 |
| Protocol Version (2) | MUST | Value = 0x0100 |
| Framing Capabilities (3) | MUST | |
| Host Name (7) | MUST | |
| Assigned Tunnel ID (9) | MUST | |
| Bearer Capabilities (4) | MAY | |
| Firmware Revision (6) | MAY | |
| Vendor Name (8) | MAY | |
| Receive Window Size (10) | MAY | Default 4 if absent |
| Challenge (11) | MAY | |
| Challenge Response (13) | Conditional | MUST if SCCRQ had Challenge |

### 7.3 SCCCN (type 3)

| AVP | Required | Notes |
|-----|----------|-------|
| Message Type (0) | MUST | Value = 3 |
| Challenge Response (13) | Conditional | MUST if SCCRP had Challenge |

### 7.4 StopCCN (type 4)

| AVP | Required | Notes |
|-----|----------|-------|
| Message Type (0) | MUST | Value = 4 |
| Assigned Tunnel ID (9) | MUST | Sender's own tunnel ID |
| Result Code (1) | MUST | |

### 7.5 HELLO (type 6)

| AVP | Required | Notes |
|-----|----------|-------|
| Message Type (0) | MUST | Value = 6 |

Session ID in header MUST be 0.

### 7.6 ICRQ (type 10)

| AVP | Required | Notes |
|-----|----------|-------|
| Message Type (0) | MUST | Value = 10 |
| Assigned Session ID (14) | MUST | |
| Call Serial Number (15) | MUST | |
| Bearer Type (18) | MAY | |
| Physical Channel ID (25) | MAY | |
| Calling Number (22) | MAY | |
| Called Number (21) | MAY | |
| Sub-Address (23) | MAY | |

### 7.7 ICRP (type 11)

| AVP | Required | Notes |
|-----|----------|-------|
| Message Type (0) | MUST | Value = 11 |
| Assigned Session ID (14) | MUST | |

### 7.8 ICCN (type 12)

| AVP | Required | Notes |
|-----|----------|-------|
| Message Type (0) | MUST | Value = 12 |
| Tx Connect Speed (24) | MUST | |
| Framing Type (19) | MUST | |
| Initial Received LCP CONFREQ (26) | MAY | Proxy LCP |
| Last Sent LCP CONFREQ (27) | MAY | Proxy LCP |
| Last Received LCP CONFREQ (28) | MAY | Proxy LCP |
| Proxy Authen Type (29) | MAY | Proxy auth |
| Proxy Authen Name (30) | MAY | With proxy auth |
| Proxy Authen Challenge (31) | MAY | With proxy auth type=CHAP |
| Proxy Authen ID (32) | MAY | With proxy auth |
| Proxy Authen Response (33) | MAY | With proxy auth |
| Private Group ID (37) | MAY | |
| Rx Connect Speed (38) | MAY | |
| Sequencing Required (39) | MAY | |

### 7.9 OCRQ (type 7)

| AVP | Required | Notes |
|-----|----------|-------|
| Message Type (0) | MUST | Value = 7 |
| Assigned Session ID (14) | MUST | |
| Call Serial Number (15) | MUST | |
| Minimum BPS (16) | MUST | |
| Maximum BPS (17) | MUST | |
| Bearer Type (18) | MUST | |
| Framing Type (19) | MUST | |
| Called Number (21) | MUST | |
| Sub-Address (23) | MAY | |

### 7.10 OCRP (type 8)

| AVP | Required | Notes |
|-----|----------|-------|
| Message Type (0) | MUST | Value = 8 |
| Assigned Session ID (14) | MUST | |
| Physical Channel ID (25) | MAY | |

### 7.11 OCCN (type 9)

| AVP | Required | Notes |
|-----|----------|-------|
| Message Type (0) | MUST | Value = 9 |
| Tx Connect Speed (24) | MUST | |
| Framing Type (19) | MUST | |
| Rx Connect Speed (38) | MAY | |
| Sequencing Required (39) | MAY | |

### 7.12 CDN (type 14)

| AVP | Required | Notes |
|-----|----------|-------|
| Message Type (0) | MUST | Value = 14 |
| Result Code (1) | MUST | CDN result codes |
| Assigned Session ID (14) | MUST | Sender's own session ID |
| Q.931 Cause Code (12) | MAY | |

### 7.13 WEN (type 15)

| AVP | Required | Notes |
|-----|----------|-------|
| Message Type (0) | MUST | Value = 15 |
| Call Errors (34) | MUST | |

### 7.14 SLI (type 16)

| AVP | Required | Notes |
|-----|----------|-------|
| Message Type (0) | MUST | Value = 16 |
| ACCM (35) | MUST | |

---

## 8. Result Codes and Error Codes

### 8.1 StopCCN Result Codes

| Value | Meaning |
|-------|---------|
| 0 | Reserved |
| 1 | General request to clear control connection |
| 2 | General error; Error Code indicates the problem |
| 3 | Control channel already exists |
| 4 | Requester is not authorized to establish a control channel |
| 5 | The protocol version of the requester is not supported; Error Code indicates the highest version supported |
| 6 | Requester is being shut down |
| 7 | Finite state machine error |

### 8.2 CDN Result Codes

| Value | Meaning |
|-------|---------|
| 0 | Reserved |
| 1 | Call disconnected due to loss of carrier |
| 2 | Call disconnected for the reason indicated in Error Code |
| 3 | Call disconnected for administrative reasons |
| 4 | Call failed due to lack of appropriate facilities being available (temporary condition) |
| 5 | Call failed due to lack of appropriate facilities being available (permanent condition) |
| 6 | Invalid destination |
| 7 | Call failed due to no carrier detected |
| 8 | Call failed due to detection of a busy signal |
| 9 | Call failed due to lack of a dial tone |
| 10 | Call was not established within time allotted by LAC |
| 11 | Call was connected but no appropriate framing was detected |

### 8.3 General Error Codes

Used when Result Code = 2 (General Error) in either StopCCN or CDN.

| Value | Meaning |
|-------|---------|
| 0 | No general error |
| 1 | No control connection exists yet for this LAC-LNS pair |
| 2 | Length is wrong |
| 3 | One of the field values was out of range or reserved field was non-zero |
| 4 | Insufficient resources to handle this operation now |
| 5 | The Session ID is invalid in this context |
| 6 | A generic vendor-specific error occurred in the LAC |
| 7 | Try another. If the requester is attempting to establish a tunnel with the remote, try a different IP address or hostname |
| 8 | Session or tunnel was shutdown due to receipt of an unknown AVP with the M-bit set |

---

## 9. Tunnel State Machine

### 9.1 States

| State | Description |
|-------|-------------|
| idle | No tunnel exists. Waiting for local open request or incoming SCCRQ. |
| wait-ctl-reply | Originator sent SCCRQ, waiting for SCCRP. |
| wait-ctl-conn | Responder sent SCCRP, waiting for SCCCN. |
| established | Tunnel is operational. Sessions can be created. |

### 9.2 Transition Table

```
Current State     | Event                          | Action                           | Next State
------------------|--------------------------------|----------------------------------|------------------
idle              | Local open request             | Send SCCRQ                       | wait-ctl-reply
idle              | Recv SCCRQ, acceptable         | Send SCCRP                       | wait-ctl-conn
idle              | Recv SCCRQ, not acceptable     | Send StopCCN, clean up           | idle
idle              | Recv SCCRP                     | Send StopCCN, clean up           | idle
idle              | Recv SCCCN                     | Clean up (no tunnel exists)      | idle
                  |                                |                                  |
wait-ctl-reply    | Recv SCCRP, acceptable         | Send SCCCN                       | established
wait-ctl-reply    | Recv SCCRP, not acceptable     | Send StopCCN, clean up           | idle
wait-ctl-reply    | Recv SCCRQ (simultaneous open) | Tie breaker resolution           | (see 9.3)
wait-ctl-reply    | Recv SCCCN                     | Send StopCCN, clean up           | idle
wait-ctl-reply    | Timeout / max retransmit       | Clean up                         | idle
                  |                                |                                  |
wait-ctl-conn     | Recv SCCCN, acceptable         | (tunnel ready)                   | established
wait-ctl-conn     | Recv SCCCN, not acceptable     | Send StopCCN, clean up           | idle
wait-ctl-conn     | Recv SCCRP                     | Send StopCCN, clean up           | idle
wait-ctl-conn     | Recv SCCRQ                     | Send StopCCN, clean up           | idle
wait-ctl-conn     | Timeout / max retransmit       | Clean up                         | idle
                  |                                |                                  |
established       | Recv StopCCN                   | Send ZLB ACK, clean up all sess  | idle
established       | Local close / admin shutdown   | Send StopCCN, clean up all sess  | idle
established       | Recv SCCRQ/SCCRP/SCCCN         | Send StopCCN, clean up           | idle
established       | Max retransmit exceeded        | Clean up all sessions            | idle
                  |                                |                                  |
ANY               | Recv StopCCN                   | Send ZLB ACK, clean up           | idle
```

### 9.3 Tie Breaker Resolution (Simultaneous Open)

When both peers send SCCRQ simultaneously:

1. Each peer receives a SCCRQ while in `wait-ctl-reply` state.
2. Compare Tie Breaker AVPs:
   - **Both have Tie Breaker**: the peer with the **lower** 8-byte value wins
     (treats the value as an unsigned 64-bit integer). The loser tears down
     its SCCRQ attempt (transitions to idle) and processes the received SCCRQ
     as if it were in idle state (becoming the responder).
     If values are **equal**: both peers tear down and wait a random delay before retrying.
   - **Only one has Tie Breaker**: the peer that included the Tie Breaker wins.
     The other peer becomes the responder.
   - **Neither has Tie Breaker**: two separate tunnels are established
     (both peers accept the other's SCCRQ and send SCCRP).

3. The "loser" MUST clean up its outgoing SCCRQ state, then re-process the
   received SCCRQ in idle state (send SCCRP, transition to wait-ctl-conn).

### 9.4 Acceptability Checks

An SCCRQ is "acceptable" if:
- Protocol Version is 1.0 (AVP value 0x0100).
- Framing Capabilities has at least one bit set.
- Host Name is present and non-empty.
- Assigned Tunnel ID is non-zero.
- If Challenge is present, shared secret is configured.
- No unrecognized mandatory AVPs (M=1).
- Resource limits not exceeded (max tunnels, max sessions, etc.).

An SCCRP is "acceptable" if:
- All SCCRQ acceptability checks, plus:
- If SCCRQ included Challenge, Challenge Response MUST be present and valid.

An SCCCN is "acceptable" if:
- If SCCRP included Challenge, Challenge Response MUST be present and valid.
- No unrecognized mandatory AVPs.

---

## 10. Session State Machines

### 10.1 Incoming Call (LAC Side)

| State | Event | Action | Next State |
|-------|-------|--------|------------|
| idle | Incoming call detected | Initiate tunnel (if needed) | wait-tunnel |
| wait-tunnel | Tunnel established | Send ICRQ | wait-reply |
| wait-tunnel | Call dropped / timeout | Clean up | idle |
| wait-reply | Recv ICRP, acceptable | Send ICCN | established |
| wait-reply | Recv ICRP, not acceptable | Send CDN, clean up | idle |
| wait-reply | Recv CDN | Clean up | idle |
| wait-reply | Call dropped | Send CDN, clean up | idle |
| wait-reply | Timeout | Send CDN, clean up | idle |
| established | Recv CDN | Clean up | idle |
| established | Call dropped | Send CDN, clean up | idle |
| established | Admin close | Send CDN, clean up | idle |

### 10.2 Incoming Call (LNS Side)

| State | Event | Action | Next State |
|-------|-------|--------|------------|
| idle | Recv ICRQ, acceptable | Send ICRP | wait-connect |
| idle | Recv ICRQ, not acceptable | Send CDN, clean up | idle |
| wait-connect | Recv ICCN, acceptable | Prepare for data (PPP) | established |
| wait-connect | Recv ICCN, not acceptable | Send CDN, clean up | idle |
| wait-connect | Recv CDN | Clean up | idle |
| wait-connect | Timeout | Send CDN, clean up | idle |
| established | Recv CDN | Clean up | idle |
| established | Local close / admin | Send CDN, clean up | idle |

### 10.3 Outgoing Call (LNS Side)

| State | Event | Action | Next State |
|-------|-------|--------|------------|
| idle | Local request to call | Initiate tunnel (if needed) | wait-tunnel |
| wait-tunnel | Tunnel established | Send OCRQ | wait-reply |
| wait-tunnel | Timeout | Clean up | idle |
| wait-reply | Recv OCRP, acceptable | (wait for call result) | wait-connect |
| wait-reply | Recv OCRP, not acceptable | Send CDN, clean up | idle |
| wait-reply | Recv CDN | Clean up | idle |
| wait-reply | Timeout | Send CDN, clean up | idle |
| wait-connect | Recv OCCN | Prepare for data | established |
| wait-connect | Recv CDN | Clean up | idle |
| wait-connect | Timeout | Send CDN, clean up | idle |
| established | Recv CDN | Clean up | idle |
| established | Local close | Send CDN, clean up | idle |

### 10.4 Outgoing Call (LAC Side)

| State | Event | Action | Next State |
|-------|-------|--------|------------|
| idle | Recv OCRQ, acceptable | Send OCRP, initiate call | wait-cs-answer |
| idle | Recv OCRQ, not acceptable | Send CDN, clean up | idle |
| wait-cs-answer | Bearer answers | Send OCCN | established |
| wait-cs-answer | Bearer fails | Send CDN, clean up | idle |
| wait-cs-answer | Recv CDN | Abort call, clean up | idle |
| wait-cs-answer | Timeout | Send CDN, clean up | idle |
| established | Recv CDN | Clean up | idle |
| established | Call drops | Send CDN, clean up | idle |

---

## 11. Reliable Delivery

The reliable delivery mechanism applies ONLY to control messages (T=1).
Data messages (T=0) are NOT reliably delivered.

### 11.1 Sequence Numbers

- **Ns**: the sequence number of the message being sent. Starts at 0 for
  each tunnel, increments by 1 (modulo 65536) for each control message sent.
- **Nr**: the sequence number the sender expects to receive next. Equals
  the Ns of the last in-order control message received, plus 1 (modulo 65536).
- ZLB (Zero-Length Body) messages carry valid Ns and Nr but do NOT increment
  the Ns counter. The next non-ZLB message reuses the same Ns value.

### 11.2 Modular Arithmetic

All sequence number comparisons use modulo-65536 arithmetic. To determine
if sequence number A is "less than" B:

```go
func seqBefore(a, b uint16) bool {
    return int16(a-b) < 0
}
```

This handles wraparound correctly. The valid window is 32767 in each direction.

### 11.3 Sending

1. Assign Ns = next_send_seq. Increment next_send_seq (mod 65536).
2. Set Nr = next_recv_seq (the Ns we expect from the peer).
3. If unacknowledged message count >= peer's Receive Window Size: queue
   the message, do not send yet.
4. Add message to retransmission queue (keyed by Ns).
5. Start retransmission timer if not already running.

### 11.4 Receiving

1. Parse header. Extract Ns and Nr.
2. **Process Nr**: acknowledge all messages in the retransmission queue
   with Ns < received Nr. Remove them from the queue.
3. **Check Ns**:
   - If Ns == next_recv_seq: process the message. Increment next_recv_seq.
     Check for queued out-of-order messages that are now in sequence.
   - If Ns < next_recv_seq (duplicate): do NOT process. But MUST send
     acknowledgment (ZLB or piggyback) to prevent the peer from retransmitting.
   - If Ns > next_recv_seq (out of order): implementation choice:
     - Queue for later processing when gap is filled, OR
     - Discard (peer will retransmit).
     Either way, do NOT update next_recv_seq.
4. Send acknowledgment: either piggyback Nr in the next outgoing control
   message, or send a ZLB if no control message is pending.

### 11.5 Retransmission

- **Initial retransmission timeout (RTO)**: 1 second (recommended, configurable).
- **Backoff**: exponential. Double the timeout on each retransmission.
  ```
  Attempt 1: 1s
  Attempt 2: 2s
  Attempt 3: 4s
  Attempt 4: 8s
  Attempt 5: 16s
  ```
- **Timeout cap**: MUST be at least 8 seconds. MAY be higher (e.g., 16s).
- **Maximum retransmissions**: 5 (recommended, SHOULD be configurable).
- **On max retransmissions exceeded**: tear down the tunnel and all its sessions.
- **Nr update on retransmission**: when retransmitting a message, the Ns stays
  the same, but Nr MUST be updated to the current value (reflecting any messages
  received since the original send).

### 11.6 ZLB (Zero-Length Body) Messages

- A ZLB is a control message header (12 bytes) with no AVPs.
- T=1, L=1, S=1, O=0, Ver=2. Length=12.
- Carries Ns (the sender's current sequence number, NOT incremented) and Nr
  (acknowledging the peer's messages).
- Used solely for acknowledgment when no control message is pending to piggyback on.
- ZLBs are NOT retransmitted. If a ZLB is lost, the peer will retransmit
  the unacknowledged message, and the receiver sends another ZLB.

### 11.7 Duplicate Acknowledgment

When receiving a duplicate (Ns < next_recv_seq), the receiver MUST still
acknowledge it. This is critical: the peer retransmitted because it did
not receive the previous ACK. Without re-acknowledging, the peer will
keep retransmitting until the tunnel times out.

### 11.8 Post-Teardown State Retention

After sending or receiving StopCCN:
- The sender maintains state for one full retransmission cycle (~31 seconds
  with default timers) to handle retransmissions of the StopCCN if the ZLB
  ACK from the receiver is lost.
- The receiver sends a ZLB ACK and also maintains state for the same period
  to handle duplicate StopCCN retransmissions.

---

## 12. Slow Start and Congestion Control

The control channel implements TCP-like congestion control to avoid
overwhelming the peer.

### 12.1 Variables

- **CWND** (congestion window): the number of messages that can be in
  flight (sent but unacknowledged). Starts at 1.
- **SSTHRESH** (slow start threshold): the boundary between slow start
  and congestion avoidance. Initialized to the peer's Receive Window Size.
- CWND is capped at the peer's Receive Window Size.

### 12.2 Slow Start Phase (CWND < SSTHRESH)

On each acknowledgment received (ZLB or piggybacked Nr that advances):
```
CWND = CWND + 1
```
Effect: CWND roughly doubles each round-trip time (exponential growth).

### 12.3 Congestion Avoidance Phase (CWND >= SSTHRESH)

On each acknowledgment received, CWND grows by 1/CWND. In integer arithmetic,
`1/CWND` is 0 for CWND > 1, so use a fractional counter:

```go
// Per-tunnel state:
//   cwnd          int  // congestion window (messages)
//   cwndCounter   int  // fractional ACK accumulator

cwndCounter++
if cwndCounter >= cwnd {
    cwnd++
    cwndCounter = 0
}
```

Effect: CWND increases by 1 per round-trip time (linear growth). The counter
tracks individual ACKs; CWND grows by 1 only after CWND ACKs are received.

### 12.4 On Retransmission (Congestion Detected)

```
SSTHRESH = max(CWND / 2, 1)
CWND = 1
```
Return to slow start phase.

### 12.5 Window Interaction

The effective send window is `min(CWND, peer's Receive Window Size)`.
Never send more than this many unacknowledged control messages.

---

## 13. Challenge/Response Authentication

### 13.1 Overview

Tunnel authentication is optional but strongly recommended. It uses a
CHAP-like mechanism (RFC 1994). Both peers can independently challenge
each other.

### 13.2 Protocol Flow

```
Peer A                                Peer B
  |                                     |
  |--- SCCRQ + Challenge(A) ---------->|  A challenges B
  |<-- SCCRP + Challenge(B)            |  B challenges A
  |         + ChallengeResponse(A) ----|  B responds to A's challenge
  |--- SCCCN + ChallengeResponse(B) -->|  A responds to B's challenge
  |                                     |
```

Either or both peers can include a Challenge AVP. If only one does,
only one direction is authenticated.

### 13.3 Computing the Challenge Response

```
ChallengeResponse = MD5(CHAP_ID || shared_secret || challenge_value)
```

Where:
- **CHAP_ID**: a single byte. The value is the **Message Type** of the
  message carrying the response:
  - For SCCRP (responding to challenge in SCCRQ): CHAP_ID = 2 (one byte: 0x02)
  - For SCCCN (responding to challenge in SCCRP): CHAP_ID = 3 (one byte: 0x03)
- **shared_secret**: the pre-configured shared secret between the two peers,
  as raw bytes.
- **challenge_value**: the raw bytes from the Challenge AVP.

The result is exactly 16 bytes (MD5 digest length).

### 13.4 Verification

The receiver computes the expected response using the same formula and
compares byte-by-byte with the received Challenge Response AVP value.
If they do not match:
- Send StopCCN with Result Code 4 (not authorized) or Result Code 2 with
  appropriate Error Code.
- Clean up the tunnel.

### 13.5 Challenge Generation

- Generate a random challenge of at least 16 bytes (recommended).
- Longer challenges provide no additional security (MD5 is the bottleneck)
  but do not cause interoperability issues.
- Use a cryptographically secure random source.

### 13.6 CHAP_ID Prevents Replay

The different CHAP_ID values (2 vs 3) for SCCRP and SCCCN ensure that
a captured response from one direction cannot be replayed in the other
direction, even with the same shared secret and challenge.

---

## 14. Hidden AVP Encryption

### 14.1 Overview

Hidden AVPs encrypt sensitive attribute values using MD5-based stream cipher
with a shared secret and a per-message random vector. This provides
confidentiality but NOT integrity or authentication (use tunnel authentication
for that).

### 14.2 Prerequisites

- A shared secret (S) known to both peers (same secret used for tunnel
  authentication).
- A Random Vector (RV): the value of the most recent Random Vector AVP
  (type 36) preceding this hidden AVP in the message. The Random Vector
  AVP MUST appear before any hidden AVPs.

### 14.3 Encoding Algorithm (Sender)

**Step 1: Build the Hidden Subformat**

```
+------+------+------+------+------+------+
| Original Length (2 bytes)  | Original Value (N bytes) | Padding (P bytes) |
+------+------+------+------+------+------+
```

- Original Length: uint16, big-endian. The length of the original Attribute
  Value (before hiding), in bytes.
- Original Value: the raw attribute value bytes.
- Padding: optional random bytes to obscure the true value length. The total
  (2 + N + P) SHOULD be a multiple of 16 for clean block alignment, but
  this is not required.

**Step 2: Compute the first cipher block**

```
b1 = MD5(type_bytes || S || RV)
```

Where:
- type_bytes = the 2-byte Attribute Type field of this AVP (big-endian).
- S = shared secret (raw bytes).
- RV = Random Vector value (raw bytes from the most recent Random Vector AVP).

**Step 3: XOR to produce ciphertext**

```
c1 = p1 XOR b1
```

Where p1 is the first 16 bytes of the hidden subformat (from Step 1).
If the subformat is shorter than 16 bytes, XOR only the available bytes.
The ciphertext length equals the subformat length; do not extend it.

**Step 4: Subsequent blocks (if subformat > 16 bytes)**

For i = 2, 3, ...:
```
b(i) = MD5(S || c(i-1))
c(i) = p(i) XOR b(i)
```

Where c(i-1) is the previous 16-byte ciphertext block, and p(i) is the
next 16 bytes of the subformat. The last block may be shorter than 16 bytes;
XOR only the available bytes.

**Step 5: Construct the AVP**

Set H=1 in the AVP flags. The Attribute Value in the AVP is the concatenation
of all ciphertext blocks. The AVP Length = 6 + total ciphertext length.

### 14.4 Decoding Algorithm (Receiver)

Reverse the encoding:

1. Get the Random Vector (RV) from the most recent Random Vector AVP
   preceding this hidden AVP.
2. Compute b1 = MD5(type_bytes || S || RV).
3. p1 = c1 XOR b1.
4. For subsequent blocks: b(i) = MD5(S || c(i-1)); p(i) = c(i) XOR b(i).
5. Concatenate all p(i) to get the hidden subformat.
6. Read Original Length (first 2 bytes). Extract Original Value (next
   Original Length bytes). Discard padding.

### 14.5 Multiple Hidden AVPs

- Multiple hidden AVPs in the same message share the same Random Vector
  (the most recently seen Random Vector AVP).
- A new Random Vector AVP can appear mid-message; it applies to all
  subsequent hidden AVPs.
- Each hidden AVP uses its own Attribute Type in the MD5 computation,
  so the same Random Vector produces different key streams for different
  AVP types.

### 14.6 Edge Cases

- If the hidden subformat is exactly 16 bytes, only one block is needed.
- If the hidden subformat is less than 16 bytes, the XOR is partial (only
  the available bytes are XORed).
- The padding is random and opaque. The decoder ignores everything after
  Original Length + Original Value.

---

## 15. Hello (Keepalive)

### 15.1 Purpose

Detect dead tunnels when no data or control traffic is flowing.

### 15.2 Mechanism

- When no messages (data or control) have been received from the peer for
  a configurable interval (recommended: 60 seconds), send a HELLO message.
- HELLO is a normal control message, so reliable delivery applies.
- If reliable delivery fails (retransmission exhausted), the tunnel is
  declared dead. Tear down the tunnel and all its sessions.

### 15.3 Message Format

- Control message header (12 bytes) + Message Type AVP (8 bytes) = 20 bytes total.
- Session ID in header = 0 (tunnel-scoped).
- The only AVP is Message Type (value = 6).

### 15.4 Response

The peer responds with a ZLB ACK (or piggybacks Nr in the next outgoing
control message). No explicit HELLO response message type exists.

### 15.5 Timer Reset

Reset the hello timer whenever ANY message (data or control) is received
from the peer. The hello interval measures silence, not time since last hello.

---

## 16. Data Messages and PPP Encapsulation

### 16.1 Data Message Structure

```
[IP Header (20 bytes)]
[UDP Header (8 bytes)]
[L2TP Data Header (6-14 bytes)]
[PPP Frame]
```

### 16.2 PPP Frame Format Within L2TP

The PPP frame carried in L2TP data messages is stripped of all HDLC framing:

**Removed** (by the LAC before encapsulation):
- HDLC Flag sequence (0x7E)
- HDLC Address field (0xFF)
- HDLC Control field (0x03)
- HDLC FCS (Frame Check Sequence)
- Byte stuffing / bit stuffing
- Any transparency escaping

**Retained**:
- PPP Protocol field (2 bytes, or 1 byte if Protocol Field Compression is
  negotiated in LCP)
- PPP Information field (the payload)

### 16.3 Common PPP Protocol Values

| Value | Protocol |
|-------|----------|
| 0x0021 | IPv4 |
| 0x0057 | IPv6 |
| 0x002B | IPX |
| 0xC021 | LCP (Link Control Protocol) |
| 0xC023 | PAP (Password Authentication Protocol) |
| 0xC223 | CHAP (Challenge Handshake Authentication Protocol) |
| 0x8021 | IPCP (IP Control Protocol) |
| 0x8057 | IPv6CP (IPv6 Control Protocol) |
| 0x80FD | CCP (Compression Control Protocol) |
| 0xC029 | CBCP (Callback Control Protocol) |

### 16.4 Minimum Data Header

When no optional fields are needed (L=0, S=0, O=0, P=0):

```
Byte 0-1: 0x0002 (T=0, L=0, S=0, O=0, P=0, Ver=2)
Byte 2-3: Tunnel ID
Byte 4-5: Session ID
Byte 6+:  PPP frame
```

Total L2TP overhead: 6 bytes.

### 16.5 Data Header with Length and Sequence

When L=1, S=1:

```
Byte  0-1:  0x4002 (T=0, L=1, S=0, O=0, P=0, Ver=2) -- if only L
  or: 0x4802 (T=0, L=1, S=1, O=0, P=0, Ver=2) -- if L+S
Byte  2-3:  Length (total L2TP message length)
Byte  4-5:  Tunnel ID
Byte  6-7:  Session ID
Byte  8-9:  Ns (if S=1)
Byte 10-11: Nr (if S=1, RESERVED -- MUST be set to 0 by sender, ignored by receiver)
Byte 12+:   PPP frame
```

---

## 17. Data Sequencing

### 17.1 Sequencing Required AVP

If the Sequencing Required AVP (type 39) is present in ICCN or OCCN:
- All data messages for this session MUST include sequence numbers (S=1).
- Neither side can disable sequencing for the session's lifetime.

### 17.2 Dynamic Sequencing (No Sequencing Required AVP)

The LNS controls sequencing behavior:
- If LNS sends data with S=1: LAC MUST also use S=1.
- If LNS sends data with S=0: LAC SHOULD also use S=0.
- LNS can change its mind at any time during the session.
- When LNS re-enables sequencing after disabling it, sequence numbers
  resume from where they left off (not reset to 0).

### 17.3 Data Sequence Numbers

- Ns starts at 0 for each session, increments by 1 (mod 65536) per data
  message sent with S=1.
- Nr in data messages is reserved and MUST be ignored on receipt. Set to 0.
- Data messages are NOT reliably delivered. Lost data messages are not
  retransmitted by L2TP (PPP/TCP above will handle retransmission).

### 17.4 Reordering

Optional. When enabled:
- Buffer out-of-order data messages for up to a configurable timeout.
- Deliver in order when the gap is filled or the timeout expires.
- Reduces out-of-order delivery to upper layers.
- Configured via PPPOL2TP_SO_REORDERTO socket option in the kernel.

---

## 18. Proxy LCP and Proxy Authentication

### 18.1 Purpose

When the LAC has already performed LCP negotiation and/or authentication
with the remote peer, it can forward the results to the LNS. The LNS
can then:
- Accept the proxy LCP parameters and skip its own LCP negotiation (faster).
- Accept the proxy authentication and skip re-authentication (single sign-on).
- Reject the proxy parameters and renegotiate from scratch.

### 18.2 Proxy LCP AVPs (in ICCN)

Three AVPs capture the LCP negotiation state:

1. **Initial Received LCP CONFREQ (type 26)**: the very first LCP CONFREQ
   received from the remote peer. Shows what the peer initially requested.

2. **Last Sent LCP CONFREQ (type 27)**: the last LCP CONFREQ the LAC sent
   to the remote peer. Shows what the LAC offered.

3. **Last Received LCP CONFREQ (type 28)**: the last LCP CONFREQ received
   from the remote peer (the one the LAC ACKed). Shows the negotiated result.

All three contain raw LCP option bytes (the body of the LCP Configuration
Request, after the PPP LCP header: no Code, ID, or Length fields).

### 18.3 Proxy Authentication AVPs (in ICCN)

| AVP | Content |
|-----|---------|
| Proxy Authen Type (29) | The authentication method used (PAP=3, CHAP=2, MS-CHAPv1=5, none=4) |
| Proxy Authen Name (30) | The username |
| Proxy Authen Challenge (31) | The CHAP challenge sent by the LAC (for CHAP/MS-CHAP) |
| Proxy Authen ID (32) | The CHAP ID used (byte 0 reserved, byte 1 = ID) |
| Proxy Authen Response (33) | The response from the remote peer |

### 18.4 LNS Behavior

The LNS has full discretion:
- Accept proxy LCP and move straight to NCP negotiation (IPCP, IPv6CP).
- Accept proxy auth and skip re-authentication.
- Reject proxy and renegotiate LCP from scratch (send new CONFREQ).
- Accept proxy LCP but reject proxy auth (re-authenticate the user).

---

## 19. WAN Error Notify and Set Link Info

### 19.1 WEN (WAN-Error-Notify, type 15)

Sent by the LAC to the LNS to report link errors on the physical connection.

- Contains the Call Errors AVP (type 34): 26 bytes of error counters.
- SHOULD be rate-limited to at most one WEN per 60 seconds per session.
- The LNS may use this information for monitoring, accounting, or deciding
  to terminate a degraded session.

### 19.2 SLI (Set-Link-Info, type 16)

Sent by the LNS to the LAC to update the ACCM (Async Control Character Map)
used on the physical link.

- Contains the ACCM AVP (type 35): send ACCM and receive ACCM.
- The LAC applies the new ACCM to the physical link (async PPP only).
- For synchronous links, ACCM is not applicable (set to 0xFFFFFFFF / ignore).

---

## 20. L2TPv3 Detection and Rejection

### 20.1 How to Detect L2TPv3

L2TPv3 (RFC 3931) uses the same UDP port (1701) but differs in the header:

1. **Version field** (bits 12-15): L2TPv3 uses Ver=3 (not 2).
2. **Protocol Version AVP**: L2TPv3 uses Ver=3, Rev=0 (not 1.0).
3. **32-bit IDs**: L2TPv3 uses 32-bit Control Connection ID (replacing the
   16-bit Tunnel ID) and 32-bit Session ID.
4. **New AVPs**: Assigned Connection ID (type not in L2TPv2 range), Router ID.

### 20.2 Rejection Procedure

When receiving a message with Ver=3 in the header, or an SCCRQ with
Protocol Version AVP value 3.0:

1. Send StopCCN with:
   - Result Code = 5 (protocol version not supported)
   - Error Code = highest supported version (encode as uint16: 0x0100 for L2TPv2)
2. Clean up any allocated state.

### 20.3 L2F Detection

If Ver=1 in the header, the message is L2F (Layer 2 Forwarding, Cisco).
Silently discard. Do not send any response (L2F is a different protocol).

---

## 21. Linux Kernel L2TP Subsystem

### 21.1 Architecture

The Linux kernel provides the data-plane acceleration for L2TP:

```
Userspace: L2TP control protocol (this implementation)
    |
    | Generic Netlink (tunnel/session management)
    | PPPoL2TP socket (PPP frame delivery)
    |
Kernel:    l2tp_core (tunnel/session state)
           l2tp_netlink (netlink command handlers)
           l2tp_ppp (PPP pseudowire, creates pppN interfaces)
           ppp_generic (PPP protocol engine)
```

### 21.2 Required Kernel Modules

- `l2tp_ppp` (or the older `pppol2tp`): PPP-over-L2TP pseudowire.
  Loading either one auto-loads dependencies (`l2tp_core`, `l2tp_netlink`, etc.).
- `ppp_generic`: kernel PPP framework.

Probe at startup:
```
modprobe l2tp_ppp || modprobe pppol2tp
```

### 21.3 Generic Netlink API

**Family**: `"l2tp"`, version 1.

#### 21.3.1 Tunnel Operations

**L2TP_CMD_TUNNEL_CREATE**: Create a kernel tunnel context.

Required attributes:
| Attribute | NLA Type | Value |
|-----------|----------|-------|
| L2TP_ATTR_CONN_ID | NLA_U32 | Local tunnel ID |
| L2TP_ATTR_PEER_CONN_ID | NLA_U32 | Peer's tunnel ID |
| L2TP_ATTR_PROTO_VERSION | NLA_U8 | 2 (for L2TPv2) |
| L2TP_ATTR_ENCAP_TYPE | NLA_U16 | 0 = UDP |
| L2TP_ATTR_FD | NLA_U32 | File descriptor of the connected UDP socket |

Optional attributes:
| Attribute | NLA Type | Purpose |
|-----------|----------|---------|
| L2TP_ATTR_DEBUG | NLA_U32 | Debug flags bitmask |
| L2TP_ATTR_UDP_CSUM | NLA_U8 | Enable IPv4 UDP checksums |
| L2TP_ATTR_UDP_ZERO_CSUM6_TX | NLA_FLAG | Zero UDP6 checksum on TX |
| L2TP_ATTR_UDP_ZERO_CSUM6_RX | NLA_FLAG | Accept zero UDP6 checksum on RX |

**L2TP_CMD_TUNNEL_DELETE**: Remove a tunnel and all its sessions.

Required: L2TP_ATTR_CONN_ID.

**L2TP_CMD_TUNNEL_GET**: Query tunnel information. Supports NLM_F_DUMP
for listing all tunnels.

#### 21.3.2 Session Operations

**L2TP_CMD_SESSION_CREATE**: Create a kernel session within a tunnel.

Required attributes:
| Attribute | NLA Type | Value |
|-----------|----------|-------|
| L2TP_ATTR_CONN_ID | NLA_U32 | Parent tunnel's local ID |
| L2TP_ATTR_SESSION_ID | NLA_U32 | Local session ID |
| L2TP_ATTR_PEER_SESSION_ID | NLA_U32 | Peer's session ID |
| L2TP_ATTR_PW_TYPE | NLA_U16 | 7 (L2TP_PWTYPE_PPP) |

Optional attributes:
| Attribute | NLA Type | Purpose |
|-----------|----------|---------|
| L2TP_ATTR_LNS_MODE | NLA_U8 | 1=LNS mode (auto-enables send/recv seq) |
| L2TP_ATTR_SEND_SEQ | NLA_U8 | 1=include Ns in data messages |
| L2TP_ATTR_RECV_SEQ | NLA_U8 | 1=require Ns in received data (drop if absent) |
| L2TP_ATTR_RECV_TIMEOUT | NLA_MSECS | Reorder timeout (0=disabled) |
| L2TP_ATTR_COOKIE | NLA_BINARY | Local cookie (L2TPv3 only) |
| L2TP_ATTR_PEER_COOKIE | NLA_BINARY | Peer cookie (L2TPv3 only) |
| L2TP_ATTR_L2SPEC_TYPE | NLA_U8 | L2-specific sublayer (L2TPv3 only) |

**L2TP_CMD_SESSION_DELETE**: Remove a session.

Required: L2TP_ATTR_CONN_ID + L2TP_ATTR_SESSION_ID.

**L2TP_CMD_SESSION_MODIFY**: Update session parameters.

Required: L2TP_ATTR_CONN_ID + L2TP_ATTR_SESSION_ID.
Modifiable: L2TP_ATTR_SEND_SEQ, L2TP_ATTR_RECV_SEQ, L2TP_ATTR_LNS_MODE,
L2TP_ATTR_RECV_TIMEOUT, L2TP_ATTR_DEBUG.

### 21.4 PPPoL2TP Socket API

#### 21.4.1 Socket Creation

```go
fd, err := syscall.Socket(syscall.AF_PPPOX, syscall.SOCK_DGRAM, PX_PROTO_OL2TP)
```

Constants:
```go
const (
    AF_PPPOX       = 24
    PX_PROTO_OL2TP = 1
)
```

#### 21.4.2 Connect Address Structure

For L2TPv2 over IPv4:

```go
type SockaddrPPPoL2TP struct {
    Family    uint16              // AF_PPPOX
    Protocol  uint32              // PX_PROTO_OL2TP
    PID       int32               // 0 = current process
    FD        int32               // tunnel UDP socket fd
    Addr      syscall.SockaddrInet4 // peer address
    STunnel   uint16              // local tunnel ID
    SSession  uint16              // local session ID
    DTunnel   uint16              // remote tunnel ID
    DSession  uint16              // remote session ID
}
```

For L2TPv2 over IPv6: use `SockaddrPPPoL2TPv6` with IPv6 address fields.

For L2TPv3: use `SockaddrPPPoL2TPv3` with 32-bit tunnel/session IDs.

#### 21.4.3 Socket Connect Sequence

```
1. Create UDP socket, bind to local addr, connect to peer addr.
2. Exchange L2TP control messages (SCCRQ/SCCRP/SCCCN, ICRQ/ICRP/ICCN).
3. Create kernel tunnel: L2TP_CMD_TUNNEL_CREATE with the UDP socket fd.
4. Create kernel session: L2TP_CMD_SESSION_CREATE.
5. Create PPPoL2TP socket: socket(AF_PPPOX, SOCK_DGRAM, PX_PROTO_OL2TP).
6. Connect: connect(pppox_fd, &sockaddr_pppol2tp, sizeof).
7. The PPPoL2TP socket now delivers PPP frames from the L2TP session.
8. Use the PPPoL2TP socket fd with the kernel PPP subsystem:
   - PPPIOCGCHAN: get the PPP channel index.
   - Open /dev/ppp, PPPIOCATTCHAN: attach the channel.
   - PPPIOCNEWUNIT: allocate a PPP unit (pppN interface).
   - PPPIOCCONNECT: connect the channel to the unit.
```

#### 21.4.4 Socket Options

Set via setsockopt with level = SOL_PPPOL2TP (defined per-platform, often 273):

| Option | Constant | Type | Description |
|--------|----------|------|-------------|
| Debug | 1 | uint32 | Debug bitmask |
| Recv Seq | 2 | uint32 (0/1) | Require sequence numbers in received data |
| Send Seq | 3 | uint32 (0/1) | Include sequence numbers in sent data |
| LNS Mode | 4 | uint32 (0/1) | 1=LNS (auto-enables send+recv seq) |
| Reorder Timeout | 5 | uint32 (ms) | 0=disable reordering |

### 21.5 Managed Tunnel Mode

For L2TPv2, always use managed mode:
1. Userspace creates and connects the UDP socket.
2. Userspace passes the fd to the kernel via L2TP_ATTR_FD.
3. Userspace continues to read/write control messages on the UDP socket.
4. The kernel reads L2TP data messages from the same UDP socket (the kernel
   "steals" data messages via the encap_recv callback; control messages
   pass through to userspace).

**Critical**: the kernel installs an encap_recv handler on the UDP socket.
After tunnel creation, the kernel intercepts UDP packets that look like
L2TP data messages (T=0) before they reach userspace. Control messages
(T=1) are passed through to userspace as normal UDP reads.

### 21.6 Data Flow Summary

```
Inbound (network -> PPP interface):
  Network -> UDP socket -> kernel l2tp_core (T=0?) -> l2tp_ppp -> ppp_generic -> pppN interface

  Control (T=1): passes through to userspace UDP read.
  Data (T=0): kernel strips L2TP header, delivers PPP frame to ppp_generic.

Outbound (PPP interface -> network):
  pppN interface -> ppp_generic -> l2tp_ppp -> l2tp_core -> UDP socket -> Network

  Kernel adds L2TP data header, sends via the UDP socket.
```

---

## 22. PPP-over-L2TP Operational Details

### 22.1 LNS PPP Termination

After session establishment (ICCN/OCCN received and accepted):

1. Create kernel tunnel and session (sections 21.3-21.4).
2. Create PPPoL2TP socket and connect.
3. Set PPPOL2TP_SO_LNSMODE = 1.
4. Attach to kernel PPP: PPPIOCGCHAN, open /dev/ppp, PPPIOCATTCHAN.
5. Create PPP unit: PPPIOCNEWUNIT (creates pppN interface).
6. Connect unit to channel: PPPIOCCONNECT.
7. Begin PPP negotiation:
   a. LCP: negotiate MRU, authentication method, echo interval.
   b. Authentication: PAP, CHAP, MS-CHAPv1/v2 (or accept proxy auth from LAC).
   c. IPCP: assign IPv4 address, DNS servers.
   d. IPv6CP: negotiate interface identifier.
8. Configure the pppN interface: set IP address, routes, MTU.
9. Session is now active. IP traffic flows through pppN.

### 22.2 LAC PPP Proxying

The LAC forwards PPP frames between the physical link and the L2TP tunnel:

1. Physical link -> LAC: strip HDLC framing (flags, address, control, FCS,
   byte stuffing). Keep PPP Protocol field + Information.
2. LAC -> L2TP tunnel: encapsulate in L2TP data message.
3. L2TP tunnel -> LAC: extract PPP frame from L2TP data message.
4. LAC -> Physical link: add HDLC framing (apply ACCM from SLI).

### 22.3 MTU Considerations

```
Physical link MTU (e.g., Ethernet): 1500
- IP header: 20
- UDP header: 8
- L2TP data header: 6 (minimal) to 12 (with length + sequence)
- PPP protocol field: 2 (or 1 with PFC)
= Maximum PPP payload: 1464-1470

Recommended PPP MRU: 1460 (conservative, accounts for all headers)
```

Configure ppp-max-mtu to enforce this limit during LCP negotiation.

### 22.4 Multiple Sessions Per Tunnel

- Each session has a unique Session ID (non-zero, unique within the tunnel).
- Each session creates its own pppN interface.
- Sessions are independent: one can be torn down without affecting others.
- Kernel multiplexes/demultiplexes by Session ID.

---

## 23. Timer Values

| Timer | Default | Range | Notes |
|-------|---------|-------|-------|
| Control retransmit initial (RTO) | 1 second | Configurable | |
| Retransmit backoff | Exponential (x2) | | RTO, 2*RTO, 4*RTO, ... |
| Retransmit timeout cap | 8 seconds minimum | >= 8s | MUST be at least 8s |
| Max retransmit count | 5 | Configurable | Tunnel torn down after |
| Full retransmit cycle | ~31 seconds | | 1+2+4+8+16 = 31s (5 retries) |
| Hello interval | 60 seconds | Configurable | Since last received message |
| Post-StopCCN state retention | ~31 seconds | Full retransmit cycle | |
| WEN rate limit | 60 seconds | | Max one per 60s per session |
| Session establishment timeout | 60 seconds | Configurable | Time to complete ICRQ->ICCN |
| Tunnel establishment timeout | 60 seconds | Configurable | Time to complete SCCRQ->SCCCN |

---

## 24. Implementation Traps and Edge Cases

### 24.1 ID Semantics

**Trap**: confusing "my ID" with "your ID" in headers.

- Header Tunnel ID = recipient's assigned tunnel ID (from their Assigned Tunnel ID AVP).
- Header Session ID = recipient's assigned session ID.
- In SCCRQ: Tunnel ID = 0 (peer hasn't assigned one yet).
- In StopCCN: Assigned Tunnel ID AVP = sender's own ID (for identification).
- In CDN: Assigned Session ID AVP = sender's own ID.

### 24.2 Protocol Version Confusion

**Trap**: the Protocol Version AVP says 1.0, but the header Version field says 2.

These are different things:
- Header Version (bits 12-15) = 2: identifies the L2TP framing format.
- Protocol Version AVP = 0x0100 (1.0): identifies the L2TP specification version.
- There is no inconsistency. This is how the RFC defines it.

### 24.3 ZLB Sequence Numbers

**Trap**: incrementing Ns for ZLB messages.

ZLB messages do NOT consume a sequence number. The Ns field in a ZLB is
the current next_send_seq value (not yet incremented). The next non-ZLB
message will use the same Ns.

### 24.4 Nr in Data Messages

**Trap**: processing Nr from data messages (S=1).

Nr in data messages is RESERVED and MUST be ignored. Only Nr from control
messages should update the acknowledgment state.

### 24.5 Duplicate Control Messages

**Trap**: not acknowledging duplicates.

Even though a duplicate control message should not be processed (its
effects were already applied), the receiver MUST acknowledge it. Otherwise,
the peer will keep retransmitting indefinitely, eventually timing out
and tearing down the tunnel.

### 24.6 Receive Window Size = 0

**Trap**: accepting a Receive Window Size of 0.

Zero is invalid. If the peer sends 0, treat it as 1 (or reject the tunnel).
The RFC says the value is "non-zero."

### 24.7 Hidden AVP Without Random Vector

**Trap**: receiving a hidden AVP when no Random Vector has been seen.

This is a protocol error. The receiver cannot decrypt the AVP. If M=1,
tear down the session/tunnel. If M=0, ignore the AVP.

### 24.8 Challenge Response Without Challenge

**Trap**: receiving a Challenge Response when no Challenge was sent.

Silently ignore the Challenge Response AVP. Do not validate it (there is
nothing to validate against). This is not an error condition.

### 24.9 Stale Timer Nr

**Trap**: not updating Nr when retransmitting a control message.

When a control message is retransmitted, the Ns stays the same (it is
the same message), but Nr MUST be updated to the current expected
sequence number. The peer may have sent messages since the original send,
and those need to be acknowledged.

### 24.10 Simultaneous Open with Equal Tie Breakers

**Trap**: establishing a tunnel when tie breaker values are equal.

If both peers send SCCRQ with identical Tie Breaker values (extremely
unlikely but possible), both peers MUST discard and wait a random delay
before retrying.

### 24.11 StopCCN After Partial Establishment

**Trap**: not sending Assigned Tunnel ID in StopCCN.

StopCCN MUST always include the sender's Assigned Tunnel ID AVP, even
if the tunnel was never fully established. The recipient needs this to
identify which tunnel is being torn down (since the header Tunnel ID
might be 0 if the peer never sent its assigned ID).

### 24.12 Session-Scoped vs Tunnel-Scoped Mandatory AVP

**Trap**: tearing down the tunnel when an unknown mandatory AVP is in a
session-scoped message.

If an unknown M=1 AVP appears in a session-scoped message (ICRQ, ICRP,
ICCN, etc.): tear down only the session (send CDN), not the tunnel.
If it appears in a tunnel-scoped message (SCCRQ, SCCRP, SCCCN): tear
down the tunnel (send StopCCN).

### 24.13 Message Type AVP Must Be First

**Trap**: accepting a control message where Message Type is not the first AVP.

The RFC requires Message Type to be the FIRST AVP. If it is not first,
the message is malformed. Discard the message (and consider sending StopCCN
if the tunnel is not established yet).

### 24.14 Length Field in Control Messages

**Trap**: not validating the Length field.

The L bit MUST be 1 for control messages. The Length field gives the total
message length. If Length is less than 12 (minimum control header), or
Length exceeds the UDP payload length, or Length does not match the actual
data received: discard the message.

### 24.15 Handling Tunnel Teardown Mid-Session-Setup

**Trap**: continuing session setup after receiving StopCCN.

When StopCCN is received, ALL sessions in the tunnel are immediately
terminated. Any in-progress session negotiations (waiting for ICCN, OCCN,
etc.) must be aborted. Send CDN for each session if possible, then clean up.

### 24.16 Ns Wraparound

**Trap**: integer overflow when comparing sequence numbers.

The comparison `int16(a - b) < 0` handles wraparound correctly in Go
(signed 16-bit subtraction). Test with values near 65535 and 0.

### 24.17 Multiple Tunnels Between Same Peers

**Trap**: assuming one tunnel per peer pair.

Multiple tunnels can exist between the same two IP addresses. Each tunnel
has a unique local Tunnel ID. Use (local IP, local port, remote IP,
remote port, Tunnel ID) as the lookup key, or just Tunnel ID if IDs are
globally unique.

### 24.18 Data Message Parsing: Variable Header

**Trap**: assuming a fixed data header size.

The data header size depends on which optional fields are present (L, S, O bits).
Parse the flags first, then extract fields in the correct order with correct
offsets.

### 24.19 UDP Source Port Variation

**Trap**: assuming the peer always uses port 1701.

The responder can use any source port. After the first message is received,
remember the peer's source address and port, and send all subsequent messages
to that address:port. Do not hardcode port 1701 for the peer.

### 24.20 Kernel Tunnel FD Sharing

**Trap**: reading data messages in userspace after kernel tunnel creation.

After creating the kernel tunnel with L2TP_CMD_TUNNEL_CREATE, the kernel
installs an encap_recv callback on the UDP socket. Data messages (T=0)
are intercepted by the kernel and never reach userspace reads. Only
control messages (T=1) pass through. Do not attempt to read data messages
from the UDP socket after kernel tunnel creation.

### 24.21 PPPoL2TP Session Socket Before Kernel Session

**Trap**: creating the PPPoL2TP socket before the kernel session.

The kernel session (L2TP_CMD_SESSION_CREATE) must exist before the
PPPoL2TP socket can connect to it. The connect() call on the PPPoL2TP
socket will fail if the kernel session does not exist.

### 24.22 Connected vs Unconnected UDP Socket

**Design decision**: for a multi-tunnel LNS, use a single unconnected UDP
socket for the listener. All tunnels share the socket. Send replies using
`sendto()` with the peer's address:port (remembered from the first received
message per tunnel).

Connected UDP (one socket per peer) is simpler for sending but requires one
fd per tunnel, hits fd limits at scale, and complicates the reactor (must
`select` across all sockets).

Unconnected UDP (one shared socket) scales to thousands of tunnels with a
single fd. The reactor reads from one socket and dispatches by Tunnel ID.
The tradeoff is needing `sendto()` per send instead of `write()`.

### 24.23 Kernel Module Probe Failure

**Trap**: silently continuing when `modprobe l2tp_ppp` and `modprobe pppol2tp`
both fail.

If neither kernel module can be loaded, all subsequent kernel operations
(L2TP_CMD_TUNNEL_CREATE, PPPoL2TP socket creation) will fail with confusing
errors. The L2TP subsystem MUST fail to start (return error from Start()) if
module probing fails. Log the specific error from modprobe.

### 24.24 SOL_PPPOL2TP Constant

**Trap**: hardcoding `SOL_PPPOL2TP = 273`.

This value is architecture-dependent and has changed across kernel versions.
Verify against the build target's kernel headers at compile time, or read
from `/usr/include/linux/if_pppol2tp.h`. On x86_64 Linux 5.x+, the value
is 273. On older kernels or other architectures, it may differ.

### 24.25 Cleanup Order

**Trap**: deleting the kernel tunnel before closing sessions.

Cleanup order must be:
1. Close PPPoL2TP sockets (disconnects PPP from sessions).
2. Delete kernel sessions (L2TP_CMD_SESSION_DELETE).
3. Delete kernel tunnel (L2TP_CMD_TUNNEL_DELETE).
4. Close the UDP socket.

Deleting the tunnel first may orphan session resources.

---

## 25. Security Considerations

### 25.1 Authentication

- Tunnel authentication (Challenge/Response) uses MD5, which is
  cryptographically weak. It prevents casual spoofing but not
  determined attackers.
- Shared secrets should be long (>= 16 characters) and random.
- Hidden AVPs use MD5-based encryption, which is also weak. They obscure
  values from casual observers but do not provide strong confidentiality.

### 25.2 IP Spoofing

- L2TP over UDP is vulnerable to IP spoofing. An attacker who can observe
  tunnel IDs and session IDs can inject data messages.
- Mitigations: IPsec (recommended by RFC 3193), firewalling, or L2TPv3
  with cookies.

### 25.3 Denial of Service

- Rate-limit incoming SCCRQ to prevent tunnel creation floods.
- Limit maximum simultaneous tunnels and sessions.
- Validate all fields before allocating resources (parse before alloc).

### 25.4 IPsec

RFC 3193 recommends running L2TP inside IPsec ESP for confidentiality
and integrity. This is transparent to the L2TP implementation (IPsec
operates below UDP).

---

## 26. Go Implementation Notes

### 26.1 Byte Order

Use `encoding/binary.BigEndian` for all multi-byte wire fields.

### 26.2 Sequence Number Comparison

```go
// seqBefore returns true if a comes before b in the 16-bit sequence space.
func seqBefore(a, b uint16) bool {
    return int16(a-b) < 0
}
```

### 26.3 Buffer Management

- Control messages are small (< 1024 bytes typically). Use a `sync.Pool`
  of 1500-byte buffers (matching UDP/Ethernet MTU). Get buffer from pool,
  write message with offset-based helpers, send, return to pool.
- For receiving, use a single read buffer for the reactor goroutine.
- Data messages are not handled in Go (kernel does encap/decap).
- See the ze integration document for the buffer-first encoding pattern
  (no `append`, no `make` in encoding helpers, skip-and-backfill for
  length fields).

### 26.4 Concurrency Model

**Note**: the ze integration document (section 11) specifies the concrete
concurrency model for ze's L2TP implementation. The guidance below is
generic; defer to the integration document where they differ.

Recommended for ze:
- Single reactor goroutine reads all tunnels' control messages from the
  shared UDP socket and dispatches to per-tunnel state machines synchronously.
- Single timer goroutine manages retransmission, hello, and timeout timers
  for all tunnels via a priority queue (min-heap by deadline).
- PPP negotiation (blocking `/dev/ppp` I/O) uses a fixed-size worker pool,
  not a goroutine per session.
- Session state is managed within the tunnel state machine (sessions share
  the tunnel's control channel).

### 26.5 Netlink

Use a Go netlink library (e.g., `vishvananda/netlink` or raw `syscall` with
`NETLINK_GENERIC`) for kernel tunnel/session management.

The Generic Netlink family "l2tp" must be resolved at runtime:
```go
familyID, err := netlinkFamily("l2tp")
```

### 26.6 PPPoL2TP Socket

Go's `syscall` package does not natively support AF_PPPOX. You will need
to use `syscall.RawSyscall` for socket creation and `unsafe.Pointer` for
the connect address structure.

```go
const (
    AF_PPPOX       = 24
    PX_PROTO_OL2TP = 1
    SOL_PPPOL2TP   = 273 // platform-specific, verify via kernel headers
)

fd, _, errno := syscall.RawSyscall(syscall.SYS_SOCKET, AF_PPPOX, syscall.SOCK_DGRAM, PX_PROTO_OL2TP)
```

### 26.7 /dev/ppp

Open `/dev/ppp` for PPP unit/channel management:
```go
pppFD, err := syscall.Open("/dev/ppp", syscall.O_RDWR, 0)
```

Key ioctls:
```go
const (
    PPPIOCGCHAN    = 0x800437B4 // get channel index
    PPPIOCATTCHAN  = 0x400437B8 // attach channel to /dev/ppp fd
    PPPIOCNEWUNIT  = 0xC004743E // allocate new PPP unit
    PPPIOCCONNECT  = 0x4004743A // connect channel to unit
    PPPIOCSMRU     = 0x40047452 // set MRU
    PPPIOCGUNIT    = 0x80047456 // get unit number
)
```

### 26.8 Testing Strategy

| Layer | Test Approach |
|-------|--------------|
| Header parsing/serialization | Unit tests with known byte sequences |
| AVP parsing/serialization | Unit tests, including hidden AVPs with known secrets |
| State machine transitions | Unit tests with mocked message dispatch |
| Reliable delivery | Unit tests: duplicate handling, out-of-order, retransmission |
| Challenge/Response | Unit tests with known challenge/secret/response triples |
| Hidden AVP encryption | Unit tests with known plaintext/ciphertext pairs |
| Kernel integration | Integration tests requiring root and kernel modules |
| Full tunnel/session lifecycle | Functional tests against accel-ppp or another L2TP peer |

### 26.9 Error Handling Priorities

1. Parse errors: log and discard the message. Do not crash.
2. Unknown mandatory AVP: send appropriate StopCCN/CDN and tear down.
3. Authentication failure: send StopCCN(Result=4) and tear down.
4. Retransmission exhausted: tear down tunnel and all sessions.
5. Resource exhaustion: send StopCCN/CDN(Result=2, Error=4) and reject.
6. Kernel errors (netlink, socket): log, send CDN/StopCCN, clean up.

### 26.10 Configuration Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| listen_addr | 0.0.0.0:1701 | UDP listen address and port |
| host_name | hostname | Host Name AVP value |
| shared_secret | (none) | Tunnel authentication secret |
| recv_window | 16 | Receive Window Size to advertise |
| hello_interval | 60s | Seconds of silence before sending HELLO |
| retransmit_initial | 1s | Initial retransmission timeout |
| retransmit_max | 5 | Maximum retransmission attempts |
| retransmit_cap | 16s | Maximum retransmission timeout |
| ppp_max_mtu | 1420 | Maximum PPP MRU to negotiate |
| max_tunnels | 0 (unlimited) | Maximum simultaneous tunnels |
| max_sessions | 0 (unlimited) | Maximum simultaneous sessions |
| hide_avps | false | Encrypt sensitive AVPs |
| data_sequencing | allow | allow, deny, prefer, require |
| reorder_timeout | 0 | Data reorder timeout in ms (0=disabled) |
| ephemeral_ports | false | Use ephemeral source ports |

---

## Appendix A: Packet Diagram Quick Reference

### A.1 SCCRQ (Initiator to Responder)

```
UDP(src=any, dst=1701)
L2TP Control Header:
  Flags: 0xC802 (T=1, L=1, S=1, Ver=2)
  Length: (total)
  Tunnel ID: 0 (peer has not assigned one yet)
  Session ID: 0
  Ns: 0 (first message)
  Nr: 0 (no messages received yet)
AVPs:
  Message Type: 1 (SCCRQ)
  Protocol Version: 0x01 0x00
  Host Name: "my-hostname"
  Framing Capabilities: 0x00000003 (async+sync)
  Assigned Tunnel ID: (our local TID, e.g., 1)
  [optional] Bearer Capabilities: 0x00000003
  [optional] Receive Window Size: 16
  [optional] Challenge: (random bytes)
  [optional] Tie Breaker: (8 random bytes)
  [optional] Vendor Name: "ze"
```

### A.2 SCCRP (Responder to Initiator)

```
L2TP Control Header:
  Tunnel ID: (initiator's Assigned Tunnel ID from SCCRQ)
  Session ID: 0
  Ns: 0
  Nr: 1 (acknowledges SCCRQ)
AVPs:
  Message Type: 2 (SCCRP)
  Protocol Version: 0x01 0x00
  Host Name: "peer-hostname"
  Framing Capabilities: 0x00000003
  Assigned Tunnel ID: (responder's local TID)
  [conditional] Challenge Response: MD5(0x02 || secret || SCCRQ_challenge)
  [optional] Challenge: (random bytes, challenges the initiator)
  [optional] Receive Window Size: 16
```

### A.3 SCCCN (Initiator to Responder)

```
L2TP Control Header:
  Tunnel ID: (responder's Assigned Tunnel ID from SCCRP)
  Session ID: 0
  Ns: 1
  Nr: 1
AVPs:
  Message Type: 3 (SCCCN)
  [conditional] Challenge Response: MD5(0x03 || secret || SCCRP_challenge)
```

### A.4 Data Message (PPP IPv4 Packet)

```
UDP
L2TP Data Header:
  Flags: 0x0002 (T=0, L=0, S=0, Ver=2)
  Tunnel ID: (recipient's TID)
  Session ID: (recipient's SID)
PPP Frame:
  Protocol: 0x0021 (IPv4)
  Information: [IPv4 packet bytes]
```

---

## Appendix B: State Machine Diagrams (ASCII)

### B.1 Tunnel State Machine

```
                    +--------+
         +--------->|  idle  |<---------+
         |          +--------+          |
         |           |      |           |
         |  local    |      | recv      |
         |  open     |      | SCCRQ     |
         |  request  |      | (accept)  |
         |           v      v           |
         |  +----------------+  +----------------+
         |  | wait-ctl-reply |  | wait-ctl-conn  |
         |  +----------------+  +----------------+
         |        |                    |
         |  recv  |                    | recv
         |  SCCRP |                    | SCCCN
         |  (ok)  |                    | (ok)
         |        v                    v
         |  +------------------------------+
         |  |         established          |
         |  +------------------------------+
         |        |              |
         |  recv  |              | local
         |  StopCCN              | close
         |        |              |
         +--------+--------------+
```

### B.2 Incoming Call Session (LNS Side)

```
                    +--------+
         +--------->|  idle  |
         |          +--------+
         |               |
         |          recv  |
         |          ICRQ  |
         |          (ok)  |
         |               v
         |     +--------------+
         |     | wait-connect |
         |     +--------------+
         |            |
         |      recv  |
         |      ICCN  |
         |      (ok)  |
         |            v
         |     +-------------+
         |     | established |
         |     +-------------+
         |            |
         |      recv CDN or
         |      local close
         |            |
         +------------+
```

---

## Appendix C: PPP Protocol Gap

This document covers L2TPv2 wire format, state machines, and kernel integration
in full detail. It does NOT provide implementation-level detail for the PPP
protocols that run inside L2TP sessions. A separate PPP implementation guide
is needed covering:

| Protocol | RFC | What it negotiates |
|----------|-----|--------------------|
| LCP | RFC 1661 | MRU, authentication method, magic number, echo. 10 states, ~30 transitions. |
| PAP | RFC 1334 | Username/password (cleartext). Simple request/response. |
| CHAP | RFC 1994 | Challenge/response (MD5). Periodic re-authentication. |
| MS-CHAPv1 | RFC 2433 | DES-based, legacy Windows. |
| MS-CHAPv2 | RFC 2759 | Mutual authentication, MPPE key derivation. |
| IPCP | RFC 1332 | IPv4 address, DNS servers (RFC 1877). Uses LCP FSM. |
| IPv6CP | RFC 5072 | Interface identifier (8 bytes). Does NOT assign addresses or prefixes. |
| CCP | RFC 1962 | MPPE encryption negotiation. |

**IPv6 addressing**: IPv6CP only negotiates the 64-bit interface identifier.
Actual IPv6 address assignment uses Router Advertisements (SLAAC, RFC 4862)
or DHCPv6 (RFC 8415) running over the established PPP link. Prefix Delegation
uses DHCPv6-PD (RFC 3633), which is a separate protocol from IPv6CP.

Until this companion document is written, use RFC 1661 (LCP FSM) and RFC 1332
(IPCP) as the primary references. The LCP FSM is the foundation: IPCP, IPv6CP,
and CCP all reuse the same state machine with different option types.
