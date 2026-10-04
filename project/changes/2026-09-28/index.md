# Week of 2026-09-28

A week of fixes found by checking Ze against the RFCs: BGP route selection for ADD-PATH and VPN routes, graceful restart, BFD, OSPF flooding and LDP sessions. Five changes need a look before upgrading, listed in their own section.

The release queue closed the week at 213 required work items and 221 nice-to-have, against 191 and 219 at the start. 24 items joined and none left. Eleven of the required additions are defects found this week, and seven more split the RFC fix work by area. The rest are update authenticity, an SRv6 forwarding path and two tooling items. This is an inventory preview. It reads two endpoints, so an item added and finished inside the same week never appears in it, and the counts measure work items rather than readiness: https://ze-software.net/project/roadmap/

## 🛰️ BGP

Fixed:

- With ADD-PATH, the best path is chosen once per prefix, and each path keeps its own MPLS label (RFC 7911, RFC 8277 Section 2.5).
- VPN and EVPN routes are compared without their labels, so the same prefix from two PEs meets in one election and a relabel replaces the old route.
- A labelled withdrawal carrying the recommended Compatibility value removed nothing (RFC 8277 Section 2.4).
- Stale routes were kept when a peer came back without Graceful Restart (RFC 4724 Section 4.2). LLGR now applies only to families both sides declared, and handles a zero restart time (RFC 9494).
- Static routes were sent for address families the session never negotiated.
- A route held back by a community, filter or next-hop check now goes out as a withdrawal, so the peer does not keep the earlier version.
- The extended OPEN length (RFC 9072) was one octet too long.

## 🔀 Other protocols

Fixed:

- BFD never sent AdminDown when a session was shut down. It now does, for three detection times, and OSPF no longer drops a neighbour on a peer's AdminDown (RFC 5880, RFC 5882).
- One bad LSA in an OSPF Link State Update discarded every LSA after it (RFC 2328 Section 13).
- LDP reported a session up on TCP connect. It now waits for the peer's KeepAlive (RFC 5036).
- L2TP with CQM enabled exited at start.
- RSVP-TE reservations never expired (RFC 2205).
- The TFTP server resent data on a duplicate ACK (RFC 1350).
- IKE now refuses a peer's ESP SPI of 0, and `vpn ipsec unmatched discard` refuses a commit on a backend that cannot enforce it.

## ⚠️ Changes to check

- sFlow: `agent-address` is now mandatory on an sflow collector, and every sflow collector must name the same one.
- PPP: with `auth-method pap`, Ze offers CHAP first and falls back to PAP after a Nak (RFC 1334 Section 2).
- BGP: `capability link-local-nexthop` needs `session link-local` on the peer or its group, or the commit is refused.
- MRT: `add-path` now governs table dumps only. Update records use the ADD-PATH format when the session negotiated it.
- REST API: `/api/v1/commands` returns kebab-case keys (`short-help`, `read-only`).

## 📚 Standards programme

Ze is being checked against every RFC it implements, one MUST at a time. The work continues.

175 RFCs are on the list. Of 6,430 requirements, 4,517 are MUST-level and 4,141 are checked. 248 still owe a test, the same as last week. Now that every requirement quotes the RFC sentence it comes from, each test is being judged again against that sentence. The BFD AdminDown and OSPF flooding fixes above came from that second reading.

Each test file that checks a single RFC is now named for it, so the tests for an RFC can be found by its number. Renaming them turned up 45 files whose name pointed at an RFC they did not test.

A green run proves everything on the list. It does not yet prove every test checks what its sentence asks: https://ze-software.net/quality/rfc-compliance/

## 🔭 Coming up

Fixing the remaining defects the second reading found, area by area. This work is under way.

The only-to-customer support for the ExaBGP compatibility bridge, mentioned last week, has still not landed.
