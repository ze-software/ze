# IPsec NAT Traversal

## Meta

| Field | Value |
|-------|-------|
| Name | IPsec NAT Traversal |
| Kind | protocol |
| Scope | partial |
| Scope gaps | RFC 3948 Section 5.1 ESP port behind NAT is an open defect, NAT transport mode is not proven on the appliance kernel |
| Level | experimental |
| Components | internal/component/ike/transport, internal/component/ike/engine/ts_nat_substitute.go |
| Interop | ipsec/real-nat-transport-ze-initiator, ipsec/real-nat-transport-ze-responder, ipsec/real-nat-tunnel-control, ipsec/natt-transport-inner-checksum, ipsec/natt-tunnel-inner-checksum |
| RFCs | rfc3948, rfc7296 |
| Docs | docs/guide/ipsec.md |
| Doc review | 2026-10-07: checked the 20s keepalive (DefaultKeepaliveInterval in internal/component/ike/transport/keepalive.go) and that ts_nat_substitute.go and nat.go hold the anchored behavior |
| Defect review | 2026-10-07: open: spec-ike-eap-rfc-defects (NAT port), spec-ipsec-nat-transport-runs-on-the-runtime-kernel; journal rows naming a Component, not each re-verified here: option-set-for-one-caller-changes-another.md:18, unwired-feature.md:37, unwired-feature.md:126 |
| Extra criteria | supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

NAT detection via SHA-1 hash notify payloads in IKE_SA_INIT (RFC 7296 Section 2.23). Port 4500 with non-ESP marker for IKE, UDP encapsulation for ESP (RFC 3948). NAT keepalive (0xFF byte, 20s interval). XFRM SA UDP encap attribute set when NAT detected. Transport mode works across an address-translating NAT on both roles: RFC 7296 Section 2.23.1's selector substitution replaces the addresses the peer named with the addresses this node observed, so the SA the kernel programs matches the packets it really sees. The SA records which side each translation is on, and `show vpn ipsec sa` reports it as `behind-nat` and `peer-behind-nat` beside `nat-detected`. <!-- source: internal/component/ike/transport/nat.go -- NAT detection and UDP encapsulation --> <!-- source: internal/component/ike/engine/ts_nat_substitute.go -- transport-mode selector substitution --> <!-- source: internal/component/ike/transport/keepalive.go -- NAT keepalive -->
