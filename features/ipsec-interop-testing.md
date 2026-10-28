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
| Doc review | 2026-10-28: reviewed the IPsec entry in docs/functional-tests.md and the public Description against the native strongSwan runner, scenario registrations and waitXFRM; reviewed the five-transform kernel subset against its QEMU, deployment and checker producers and accepted amd64/KVM and arm64/HVF receipts; no full-suite pass count or maturity promotion claimed |
| Defect review | 2026-10-07: journal balance-assertion-vacuous-without-a-loan.md row unfixed |
| Extra criteria | supported: a dated record of the suite's pass count = none yet |

## Description

Docker-based interop test infrastructure against strongSwan as the remote IKE peer, covering Ze as initiator (PSK, EAP-MSCHAPv2, EAP-TLS, EAP method negotiation by Nak, BGP redistribute, child rekey) and as responder (PSK, EAP-MSCHAPv2, IKE-SA rekey) via `test/interop-ipsec/scenarios/`. The dataplane checks require Linux XFRM: a scenario waits up to 30 seconds for the ESP state on each peer and fails when none appears. <!-- source: internal/le/interoplab/ipsec/ipsec.go -- native interop lab runner --> <!-- source: internal/le/interoplab/ipsec/helpers.go -- waitXFRM, xfrmWaitTimeout -->

The manual `./le test qemu vpn-kernel-test` additionally proves the kernel
facilities for AES-GCM, AES-CBC/SHA256, ChaCha20-Poly1305, 3DES/SHA1 and
NULL/SHA256 against strongSwan. It requires installed transforms and received
tunnel traffic with module loading disabled. Native amd64/KVM and arm64/HVF
receipts cover that five-scenario subset; they are not a pass count for the
whole IPsec suite and do not promote this feature's maturity.
<!-- source: internal/le/test/qemu/vpnproof.go -- runVPNKernelHere, vpnGuestPreparation -->
<!-- source: internal/le/test/deployment/vpnproof.go -- runVPNKernelAt -->
<!-- source: internal/le/interoplab/ipsec/vpnproof.go -- checkESPKernel -->
