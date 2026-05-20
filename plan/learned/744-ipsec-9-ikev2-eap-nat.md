# 744 -- IKEv2 EAP Authentication and NAT Traversal

## Context

Ze's native IKEv2 engine (ipsec-7) supported only PSK and X.509 authentication for site-to-site peers. Road warrior VPN clients (Windows, macOS, iOS, Android) use their built-in IKEv2 client which defaults to EAP authentication. Without EAP support, Ze could not serve as a remote access VPN gateway. NAT traversal was also missing, blocking IPsec tunnels when either peer is behind a NAT device.

## Decisions

- Implemented MD4 from scratch over using an external dependency, because Go removed crypto/md4 from both stdlib and x/crypto. The implementation is minimal (70 lines) and verified against RFC 1320 test vectors.
- Chose to store EAP session state on the SA struct as `any` (interface) over importing the eap package directly in the engine package, to avoid import cycles between engine and eap.
- RFC 2759 magic constants (Magic1, Magic2) are used WITHOUT the trailing null byte, over including it. The RFC's C array declarations show [39] with 40 initializer values (a bug). Working implementations (hostapd, strongSwan) exclude the null. Verified against RFC 2759 Section 9 test vectors.
- EAP-TLS uses Go's crypto/tls with a custom net.Conn transport (eapTLSTransport) that pipes TLS records through EAP request/response packets, over implementing TLS from scratch.
- Virtual IP pool uses sequential allocation from CIDR base over random allocation, for simplicity and predictable debugging.
- NAT detection hashes are compared with constant-time equality over short-circuit, even though SHA-1 NAT detection is not a security-sensitive comparison, for consistency with the codebase pattern.

## Consequences

- Road warrior VPN is now possible: Windows clients can connect with EAP-MSCHAPv2 (username/password), any client can use EAP-TLS (certificate).
- The eap package is independent and testable without the engine, but the engine needs EAP wired into the responder IKE_AUTH path (currently initiator-only FSM).
- NAT-T UDP encapsulation attribute is set on XFRM SAs when NAT is detected, enabling ESP-in-UDP through NAT devices.
- The NATDetected flag on the SA propagates to child SA creation automatically.

## Gotchas

- MD4 is gone from Go entirely (stdlib and x/crypto). Any protocol requiring MD4 (MS-CHAPv2, NTLM) must bring its own implementation. Round-function rotation indexing must use the loop counter, not the word index from the permutation table.
- RFC 2759 magic constant sizes in the C pseudocode are off-by-one (array size excludes null, initializer includes it). This causes interop failures if the null is included. The RFC summary in the repo incorrectly stated "includes null" with the wrong byte count.
- The `DetectNAT` function initially took `[8]byte` for the received hash, but NAT detection hashes are 20-byte SHA-1. This caused the comparison to always report NAT present (length mismatch). Fixed to `[]byte`.
- EAP-TLS handshake is inherently asynchronous (TLS runs in a goroutine reading/writing through the transport), so the eapTLSTransport must use mutex-protected buffers and a notification channel. Non-blocking channel sends require a helper to avoid empty default cases (blocked by project hook).

## Files

- Created: `internal/component/ike/eap/eap.go`, `eap_mschapv2.go`, `eap_tls.go`, `mschapv2.go`, `md4.go`, `pool.go`
- Created: `internal/component/ike/eap/eap_test.go`, `mschapv2_test.go`, `pool_test.go`
- Created: `internal/component/ike/transport/nat.go`, `keepalive.go`, `nat_test.go`, `keepalive_test.go`
- Created: `internal/component/ike/engine/eap_auth.go`, `eap_auth_test.go`
- Modified: `internal/component/ike/engine/sa.go` (NAT/EAP fields), `child.go` (UDP encap), `initiator.go` (NAT detection payloads), `fsm.go` (NAT detection processing)
- Modified: `internal/component/ike/dataplane/dataplane.go` (UDP encap fields), `xfrm_linux.go` (XFRM encap)
- Modified: `docs/features.md` (EAP and NAT-T entries)
