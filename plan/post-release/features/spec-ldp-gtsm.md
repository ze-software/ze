# Spec: ldp-gtsm

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release: GTSM for LDP (RFC 6720), negotiated through the Hello G flag. No release profile lists RFC 6720, so no summary is written or enrolled until Thomas brings it into scope. RFC 7552 (LDP over IPv6) makes GTSM mandatory for IPv6 Link Hellos; Ze's LDP is IPv4-only today, so that obligation arrives with LDP IPv6. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

## Task

GTSM (RFC 5082) protects a directly connected session from off-link attackers:
both ends send with TTL 255 and drop anything that arrives below 255. Ze already
does this for BGP. LDP sessions between directly connected LSRs get the same
protection through RFC 6720: the G flag in the Common Hello Parameters TLV says
"I support GTSM", and when both Link Hellos carry it the TCP session runs under
GTSM. Ze's LDP neither sets nor reads the G flag, and its sessions run with
default TTLs.

An operator also needs to turn it off per neighbor, for an LSR that is not
directly connected over the session path.

### VyOS reference

VyOS `protocols mpls ldp neighbor <a> ttl-security disable|<hops>`; T9534
(vyos-1x `b4aed13fe`) fixed the rendering of `disable`. FRR's ldpd sets the
flag in `send_hello` from `F_LDPD_AF_NO_GTSM` and negotiates it in
`recv_hello` (`ldpd/hello.c`, "dynamic LDPv4 GTSM negotiation as per RFC
6720"). BIRD has no LDP, so FRR is the interop peer.

### RFC text read (`rfc/full/rfc6720.txt`, `rfc/full/rfc7552.txt`)

- RFC 6720 Section 2.1: the G flag is the third bit after T and R in the Common Hello Parameter TLV; "The G flag is meaningful only if the T flag is set to 0 (which must be the case for Basic Discovery); otherwise, the value of the G flag is ignored on receipt."
- RFC 6720 Section 2.2: "Unless configured otherwise, an LSR that supports GTSM procedures MUST set the G flag (for GTSM) to 1 in the Common Hello Parameter TLV in the LDP Link Hello message [RFC5036]."
- RFC 6720 Section 2.2: a received G=1 means the LSR "MUST enforce GTSM for LDP in the subsequent TCP/LDP peering session"; G absent or 0 means it "MUST NOT enforce GTSM".
- RFC 6720 Section 2.3: "This means that the LSR MUST check for the incoming unicast packets' TTL or Hop Count to be 255 for the particular LDP/TCP peering session and decide the further processing as per [RFC5082]."
- RFC 7552 Section 9: "LDP Link Hello packets MUST have their IPv6 Hop Limit set to 255 and be checked for the same upon receipt before any further processing, as per Section 3 of [RFC5082]." (owed when LDP gains IPv6)

## Required Reading

- [ ] `docs/architecture/ldp/mpls-ldp.md` - discovery and session setup
- [ ] `rfc/short/rfc5082.md` - GTSM as Ze implements it for BGP
- [ ] `rfc/short/rfc5036.md` - Section 3.5.2, the Common Hello Parameters TLV

## Current Behavior

- [ ] `internal/plugins/ldp/wire.go` - `EncodeHello` sets only the T (0x8000) and R (0x4000) flags; `DecodeHello` reads only those two, so a peer's G flag (0x2000) is dropped.
- [ ] `internal/plugins/ldp/register.go` - `listenDiscovery` opens an IPv4-only multicast socket (`udp4`); `ldpSessionDialer` sets no TTL and no minimum TTL on the session socket.
- [ ] `internal/component/gtsm/gtsm.go` - `SetPeers` takes the WHOLE GTSM peer set on every call and withdraws anything absent. BGP is its only caller.

## Existing Mechanisms

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| Socket TTL and minimum TTL | `internal/core/network` `SetIPTTL`, `SetIPMinTTL` | Set TTL 255 and `IP_MINTTL` / `IPV6_MINHOPCOUNT` on BGP sockets | Reused on the LDP session socket and its listener |
| GTSM kernel state | `internal/component/gtsm` `SetPeers` | Host routes with RTAX_HOPLIMIT and nft input rules per peer for related ICMP and pre-accept SYNs | Extends: the peer set must take contributions from more than one protocol. Today a second caller would overwrite BGP's set, so the design adds per-owner sets merged inside the package, and states how a forked LDP plugin reaches it (the package is in-process) |
| LDP Hello codec | `wire.go` `HelloMessage` | T and R flags | Extends: the G flag, encoded and decoded |
| LDP adjacency to session | `register.go` `sessionConfigForAdj` | Builds session parameters from the Hello | Extends: carries "both sent G=1" into the session, which then enforces GTSM |

## Data Flow

### Entry Point
LDP config (GTSM on by default, per-neighbor disable) and received Link Hellos.

### Transformation Path
1. Link Hellos carry G=1 unless disabled.
2. An adjacency records the neighbor's G flag.
3. When both sent G=1, the TCP session sets TTL 255 and the minimum TTL 255, and the GTSM component installs the neighbor's kernel state.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| LDP plugin to GTSM component | a registered, owner-scoped peer set (design decides the seam) | No |

### Integration Points
- Hello codec, adjacency state, session dialer and listener, GTSM component.

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| LDP config, neighbor sending G=1 | → | session socket with minimum TTL 255 | [Linux integration test reading the socket option, named at design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | LDP enabled, no GTSM config | Link Hellos carry G=1; hex asserted |
| AC-2 | Targeted Hello | G is not set, and a received G on a Targeted Hello is ignored (Section 2.1) |
| AC-3 | Both sides sent G=1 | The session socket sends with TTL 255 and drops TTL below 255 (Linux integration test injects a TTL 254 segment and sees it dropped) |
| AC-4 | Neighbor sent G=0 or no G | GTSM is not enforced for that session (MUST NOT) |
| AC-5 | Per-neighbor disable | Ze sends G=0 toward that neighbor's interface scope as the design defines, and never enforces GTSM for it |
| AC-6 | BGP GTSM peers and LDP GTSM neighbors at the same time | Both sets stay installed; removing one protocol's peer never withdraws the other's |
| AC-7 | `show ldp neighbor` | Prints whether GTSM was negotiated |
| AC-8 | Interop: Ze and FRR ldpd on one link | GTSM negotiated on both; the session survives; proven red with the flag reverted |

## Risks & Assumptions

| ID | Risk | Mitigation |
|----|------|------------|
| R-1 | A GTSM-enforcing session to a peer reached over several hops never comes up | Per-neighbor disable, and a doctor check naming the neighbor and the fix |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [named at design] | `internal/plugins/ldp/wire_test.go` | G flag encode and decode, T=1 handling | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [named at design] | `test/ldp/` | Config parses; Hellos carry G | |

## Files to Modify

- `internal/plugins/ldp/wire.go`, `register.go`, `session.go`, the LDP YANG
- `internal/component/gtsm/gtsm.go` (owner-scoped peer sets)
- `docs/guide/mpls.md`

## Implementation Steps

1. Owner-scoped GTSM peer sets, BGP behavior unchanged.
2. G flag codec, negotiation, session enforcement.
3. FRR interop scenario.

### Integration Checklist
- [ ] BGP GTSM tests unchanged and green

### Documentation Update Checklist
- [ ] `docs/guide/mpls.md` documents GTSM and the disable leaf

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before the change
- [ ] Tests PASS after the change
- [ ] `./le test ldp`, `./le test qemu all` for the socket test, `./le test integration interop`
