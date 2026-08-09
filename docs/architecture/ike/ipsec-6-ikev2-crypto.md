# IKEv2 cryptographic primitives

A pure-Go primitives layer with no network input or output and no state:
Diffie-Hellman key exchange, PRF-based key derivation, encryption and integrity,
and proposal negotiation. It maps the config algorithm names to IANA transform
IDs and to Go standard library implementations.

<!-- source: internal/component/ike/crypto/transform.go -- LookupEncryption, EncryptionTransform, PRFTransform, IntegrityTransform, DHGroupTransform -->
<!-- source: internal/component/ike/crypto/dh.go -- NewDHExchange, DHExchange.SharedSecret -->
<!-- source: internal/component/ike/crypto/prf.go -- PRF, PRFPlus -->
<!-- source: internal/component/ike/crypto/keys.go -- DeriveSKEYSEED, DeriveSKKeys, DeriveChildSAKeys, DeriveChildSAKeysPFS -->
<!-- source: internal/component/ike/crypto/cipher.go -- AEAD and CBC sealers, HMAC integrity -->
<!-- source: internal/component/ike/crypto/proposal.go -- IKEProposal, ESPProposal, acceptEncryption, acceptPRF, acceptIntegrity -->

## RFC obligations carried by this code

- RFC 7296 Section 2.13 caps `prf+` at 255 iterations. `PRFPlus` fails
  explicitly when a derivation asks for more key material than that allows.
- The MODP 2048 prime is RFC 3526 Group 14. It must be transcribed exactly. A
  single wrong digit produces a silent key-agreement failure, which is why the
  two-peer shared-secret test exists.

<!-- source: internal/component/ike/crypto/dh.go -- modp2048Prime, modp2048Generator -->

## Decisions

**Standard library crypto only, no CGo and no external library.** The packages
used are `crypto/ecdh`, `crypto/aes`, `crypto/hmac` and `math/big`.

**A flat map registry, not the registration pattern.** The algorithm set is
small and fixed: four encryption transforms, three PRFs, three integrity
transforms and three DH groups. A registry with `init()` hooks buys nothing at
that size.

**`crypto/ecdh` for the ECP groups 19 and 20, not `crypto/elliptic`.** The ecdh
package gives the correct API directly.

**The crypto package names its own `IKEProposal` and `ESPProposal`.** These
shadow the config types of the same name on purpose: the package qualifier
disambiguates, and the crypto proposals are resolved transforms rather than
config references.

**PKCS#7 unpadding is constant time.** IKEv2 encrypts then MACs, so a padding
oracle is not reachable in practice. The constant-time form is still what ships,
because the cost is nil and the property does not depend on the caller.

## Traps this code exists to avoid

**Key material must be cleared through the whole chain.** `SKKeys.Clear` and
`ChildSAKeys.Clear` zero their buffers and callers invoke them through `defer`.
Any new path that derives keys inherits that obligation.

**`DHExchange.Clear` cannot guarantee erasure.** It zeroes the `big.Int`, but Go
gives no secure memory wipe and the garbage collector may already hold a copy.
Treat the clear as best effort, not as a guarantee.

**MODP 2048 private keys are drawn from the range 2 to p-2.** Public key
validation rejects 0, 1 and p-1. Degenerate values are the failure this excludes.

## Certificate payload handling

The CERT and CERTREQ surfaces sit in the engine package rather than in crypto,
because they need the SA and the PKI store. RFC 7296 Section 3.6 governs all
three: the bundle encoding, the hash-and-URL form, and the ordering rule that
puts the certificate holding the AUTH key first. The obligations are quoted
inline at each producer, and the fetcher's denied-prefix list names the RFC that
reserves each range. Read the code comments there; this document does not
restate them.

<!-- source: internal/component/ike/engine/certbundle.go -- encodeCertBundle, decodeCertBundle -->
<!-- source: internal/component/ike/engine/certurl.go -- splitHashAndURL, lookupHashAndURL, certURLDenied, certURLFetcher -->
<!-- source: internal/component/ike/engine/cert_payload.go -- CERT and CERTREQ payload construction -->
