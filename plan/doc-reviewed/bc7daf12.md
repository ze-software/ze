# Reviewed claims: session bc7daf12

Each row records a docs claim a reader compared with a change that preserves
behavior. The rules and the check that reads this table are in
`docs/contributing/documentation-testing.md`.

| Doc | Source | Symbol | Commit | Reason |
|-----|--------|--------|--------|--------|
| docs/architecture/ike/ipsec-11-interop-eap.md | internal/component/ike/engine/auth.go | buildAuthRequest | 9e3ed47790 | RFC 4739 notifies are appended only for a peer with a second round, and the EAP-mode test now reads localAuth, which is round 1 in the first request; the EAP initiator flow the page describes is unchanged |
| docs/architecture/ike/ipsec-11-interop-eap.md | internal/component/ike/engine/responder_eap.go | eapTLSServerConfig | 9e3ed47790 | the intermediate list is read through the Intermediates accessor, which answers the same names; the CRL and trust-anchor behavior the claim states is unchanged |
| docs/architecture/ike/ipsec-13-rekey-wire.md | internal/component/ike/engine/fsm.go | handleAuthResponse | 9e3ed47790 | adds the RFC 4739 second-exchange branch; the EAP Done/Err handling of the Message ID counter the claim describes is untouched |
| docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md | internal/component/ike/engine/fsm.go | handleAuthResponse | 9e3ed47790 | the SAr2 SPI-0 refusal with INVALID_SYNTAX is unchanged; the change only adds the second-exchange branch before the Child SA tail |
| docs/architecture/ike/ipsec-9-ikev2-eap-nat.md | internal/component/ike/engine/fsm.go | handleAuthResponse | 9e3ed47790 | a response with no SK payload is still dropped with the SA kept; the change adds a message-id check and the second-exchange branch after that drop |
| docs/architecture/ike/ipsec-9-ikev2-eap-nat.md | internal/component/ike/engine/responder_eap.go | handleResponderEAP | 9e3ed47790 | only adds an ANOTHER_AUTH_FOLLOWS refusal after the AUTH-from-MSK verifies; EAP round handling, discard and endpoint adoption are unchanged |
