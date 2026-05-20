# 739 -- ipsec-6-ikev2-crypto

## Context

Ze's native IKEv2 engine needs a pure-Go cryptographic primitives layer for DH key exchange, PRF-based key derivation, encryption/integrity, and proposal negotiation. This is a greenfield library package at `internal/component/ike/crypto/` with no network I/O or state, consumed by ipsec-7 (IKE engine). It maps the config algorithm name strings from `internal/component/ipsec/types.go` to IANA transform IDs and Go stdlib crypto implementations.

## Decisions

- Chose pure Go stdlib crypto (`crypto/ecdh`, `crypto/aes`, `crypto/hmac`, `math/big`) over CGo or external libraries because the umbrella spec mandates no CGo dependencies
- Chose flat map-based transform registry over a registration pattern because the algorithm set is small and fixed (4 encryption, 3 PRF, 3 integrity, 3 DH groups)
- Chose `crypto/ecdh` for ECP groups 19/20 over `crypto/elliptic` because ecdh provides the correct ECDH API directly (Go 1.20+)
- Chose `IKEProposal`/`ESPProposal` names in the crypto package (shadowing `ipsec.IKEProposal`) over prefixed names because the package qualifier disambiguates and the crypto proposals represent resolved transforms, not config references
- Chose constant-time PKCS#7 unpadding (`subtle.ConstantTimeByteEq`) over data-dependent branching, even though IKEv2 encrypt-then-MAC prevents padding oracles in practice

## Consequences

- ipsec-7 (IKE engine) can import this package directly for all crypto operations during IKE_SA_INIT, IKE_AUTH, and CREATE_CHILD_SA exchanges
- The transform registry maps config name strings ("aes128gcm", "sha256") to IANA IDs, key lengths, and AEAD flags, bridging the config layer (ipsec-3) and wire layer (ipsec-5)
- `prf+` is capped at 255 iterations per RFC 7296 Section 2.13; any key derivation exceeding this limit will fail explicitly
- DH private keys for MODP 2048 are generated in [2, p-2] to exclude degenerate values; public key validation rejects 0, 1, and p-1
- `SKKeys.Clear()` and `ChildSAKeys.Clear()` zero key material; callers must invoke these via defer

## Gotchas

- The MODP 2048 prime from RFC 3526 must be exact; any transcription error produces silent key agreement failures (verified by two-peer shared secret tests)
- `DHExchange.Clear()` can zero the Go big.Int but cannot guarantee the GC has not already copied it; Go does not support secure memory wiping

## Files

- `internal/component/ike/crypto/transform.go` -- transform type registry (IANA IDs, config name mapping)
- `internal/component/ike/crypto/dh.go` -- DH groups 14, 19, 20
- `internal/component/ike/crypto/prf.go` -- PRF and prf+ key expansion
- `internal/component/ike/crypto/cipher.go` -- AES-GCM, AES-CBC, HMAC integrity
- `internal/component/ike/crypto/keys.go` -- SKEYSEED, SK_* hierarchy, Child SA KEYMAT
- `internal/component/ike/crypto/proposal.go` -- IKE/ESP proposal negotiation
- `internal/component/ike/crypto/*_test.go` -- 35 unit tests covering all ACs
