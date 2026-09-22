---
kind: directive
level: MUST
stage:
---
**Implemented protocol capabilities MUST follow the applicable requirements of their own RFCs and standards.** This applies to every protocol Ze speaks, including BGP, IS-IS, OSPF, BFD, LDP, RSVP-TE, IKE and IPsec, L2TP, PPPoE, DHCP, NTP, RADIUS, TACACS+, gNMI, BMP, RPKI and VRRP. Implementation scope follows the baseline policy below.
**Every MUST and MUST NOT enforced in code MUST carry a comment directly above it naming the RFC section and quoting the requirement (`// RFC NNNN Section X.Y: "quoted requirement"`), covering whichever of the validation rules, error conditions, state transitions, timer constraints and message ordering the code enforces.** Protocol code MUST NOT be changed without documenting the wire format: an ASCII diagram with field offsets, byte offset annotations, and the RFC section reference.
