---
kind: directive
level: MUST
stage:
---
**Every protocol surface MUST be held to its own RFCs, and so MUST anything Ze speaks that has a standard behind it.** Not just BGP: IS-IS, OSPF, BFD, LDP, RSVP-TE, IKE and IPsec, L2TP, PPPoE, DHCP, NTP, RADIUS, TACACS+, gNMI, BMP, RPKI and VRRP.
**Every MUST and MUST NOT enforced in code MUST carry a comment directly above it naming the RFC section and quoting the requirement (`// RFC NNNN Section X.Y: "quoted requirement"`), covering whichever of the validation rules, error conditions, state transitions, timer constraints and message ordering the code enforces.** Protocol code MUST NOT be changed without documenting the wire format: an ASCII diagram with field offsets, byte offset annotations, and the RFC section reference.
**A MAY clause MUST be put to the user: implement it, skip it, or make it a config option.** You are not authorized to pick.
