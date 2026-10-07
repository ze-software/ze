# IKEv2 Engine

## Meta

| Field | Value |
|-------|-------|
| Name | IKEv2 Engine |
| Kind | protocol |
| Scope | complete |
| Level | experimental |
| Components | internal/component/ike/engine, internal/component/ike/dataplane |
| Real-path tests | test/ipsec/ipsec-child-rekey.ci, test/ipsec/ipsec-dpd-timeout.ci, test/ipsec/ipsec-clear-reestablish.ci, test/ipsec/ipsec-cookie-challenge.ci, test/ipsec/ipsec-peer-reload-applies-selectors.ci, test/ipsec/ipsec-teardown-leaves-nothing.ci |
| Interop | ipsec/psk-site-to-site, ipsec/responder-psk, ipsec/responder-ike-rekey, ipsec/child-rekey, ipsec/clear-reestablish |
| RFCs | rfc7296, rfc4301, rfc4303 |
| Docs | docs/guide/ipsec.md |
| Doc review | 2026-10-07: every source anchor resolves (engine/, engine/responder.go, feature-gates.txt ze_ike); initiator and responder roles, DPD and rekey are each driven by a listed test or interop scenario |
| Defect review | 2026-10-07: open: spec-ike-responder-eap-preserves-state-on-unauthenticated-input and spec-ike-eap-rfc-defects (ESP port behind NAT) name the engine; journal rows naming a Component, not each re-verified here: balance-assertion-vacuous-without-a-loan.md:8, check-cannot-see-the-change-it-looks-for.md:19, claim-outlives-the-evidence-it-cites.md:51, concurrent-session-corruption.md:12, constant-reported-as-measured-state.md:11, diagnosis-parked-until-a-round-the-peer-may-never-send.md:15, false-synchronization-claim.md:17, field-carries-two-meanings.md:22, field-carries-two-meanings.md:23, gate-verdict-depends-on-the-machine.md:119, guard-added-to-one-half-of-a-pair.md:29, guard-added-to-one-half-of-a-pair.md:39, guard-added-to-one-half-of-a-pair.md:46, guard-added-to-one-half-of-a-pair.md:55, guard-addition-drops-what-it-refuses.md:13, guard-enumerates-instead-of-subtracting.md:22, guard-enumerates-instead-of-subtracting.md:24, invariant-enforced-by-an-absent-call-site.md:6, one-state-held-in-two-fields.md:16, parallel-copies-collide-on-a-deterministic-port.md:7, parameter-no-caller-ever-fills.md:3, reference-checked-claim-unchecked.md:22, silent-fall-through.md:15, silent-fall-through.md:58, silent-fall-through.md:65, stale-artifact-reused.md:28, stale-artifact-reused.md:34, stale-artifact-reused.md:60, test-against-broken-path.md:60, test-against-broken-path.md:70, unprotected-message-changes-sa-state.md:3, unprotected-message-changes-sa-state.md:4, unwired-feature.md:13, unwired-feature.md:60, unwired-feature.md:64, unwired-feature.md:89, unwired-feature.md:99, unwired-feature.md:126, validated-value-discarded-by-its-caller.md:19, zero-value-as-valid-answer.md:27 |
| Extra criteria | supported: rekey collision proven against strongSwan = ipsec/rekey-collision; supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

Full IKE FSM for both **initiator and responder** roles (`connection-type initiate`/`respond`): IKE_SA_INIT, IKE_AUTH, CREATE_CHILD_SA, INFORMATIONAL exchanges. PSK and X.509 certificate authentication; as responder Ze also acts as the EAP authenticator (EAP-MSCHAPv2, EAP-TLS and MD5-Challenge server). Child SA creation with traffic selectors and ESP proposals. IKE SA and Child SA rekeying (initiate and respond) with collision handling. DPD (Dead Peer Detection) via INFORMATIONAL exchange with configurable interval and timeout. XFRM policy and state programming via netlink. Reconciliation on config reload. SK crypto, key derivation, AUTH octets, and ESP key roles are parameterized by SA role. <!-- source: internal/component/ike/engine/ -- IKE FSM and reconciliation --> <!-- source: internal/component/ike/engine/responder.go -- IKE responder handshake --> Compile-out-able with the `ze_ike` build tag (default-on in `ZE_FEATURES`): a stripped build drops the engine, IPsec config plumbing, EAP, wire codec, and command surface and rejects a `vpn { ipsec {} }` block. Two packages stay always-on: `ike/dataplane` (the XFRM programming seam OSPF's RFC 4552 authentication also uses) and `ike/crypto` (the IKEv2 transform registry `show mtu` reads to size the IPsec tunnels the kernel holds). <!-- source: feature-gates.txt -- ze_ike -->
