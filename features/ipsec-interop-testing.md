# IPsec Interop Testing

## Meta

| Field | Value |
|-------|-------|
| Name | IPsec Interop Testing |
| Kind | test-infra |
| Scope | complete |
| Level | experimental |
| Components | internal/le/interoplab/ipsec, test/interop-ipsec |
| Docs | docs/functional-tests.md |
| Doc review | 2026-10-07: waitXFRM in internal/le/interoplab/ipsec/helpers.go fails after xfrmWaitTimeout (30s) and no availability gate exists; scenarios under test/interop-ipsec/scenarios |
| Defect review | 2026-10-07: journal balance-assertion-vacuous-without-a-loan.md row unfixed |
| Extra criteria | supported: a dated record of the suite's pass count = none yet |

## Description

Docker-based interop test infrastructure against strongSwan as the remote IKE peer, covering Ze as initiator (PSK, EAP-MSCHAPv2, EAP-TLS, EAP method negotiation by Nak, BGP redistribute, child rekey) and as responder (PSK, EAP-MSCHAPv2, IKE-SA rekey) via `test/interop-ipsec/scenarios/`. The dataplane checks require Linux XFRM: a scenario waits up to 30 seconds for the ESP state on each peer and fails when none appears. <!-- source: internal/le/interoplab/ipsec/ipsec.go -- native interop lab runner --> <!-- source: internal/le/interoplab/ipsec/helpers.go -- waitXFRM, xfrmWaitTimeout -->
