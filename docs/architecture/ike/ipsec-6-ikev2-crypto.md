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
<!-- source: internal/component/ike/crypto/aead.go -- aeadTransforms, AEADICVOctets, SealIKEAEAD, OpenIKEAEAD -->
<!-- source: internal/core/ccm/ccm.go -- New, the CCM mode of RFC 3610 -->
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
used are `crypto/ecdh`, `crypto/aes`, `crypto/cipher`, `crypto/hmac` and
`math/big`.

**AES CCM is written in `internal/core/ccm`, because the standard library has no
CCM.** RFC 5282 Section 3.2 obliges an IKEv2 implementation that negotiates AES
CCM to produce its ICV, and `crypto/cipher` carries GCM alone. The mode is CTR
encryption over a CBC-MAC, specified in RFC 3610 and NIST SP 800-38C, and it is
driven by the twenty-four packet vectors of RFC 3610 Section 8 in both
directions. It sits in `internal/core` rather than beside the IKE code because
it holds no config, registers nothing, and keeps no state.

**One table decides every AEAD property, keyed on the wire Transform ID.**
`aeadTransforms` (`aead.go`) carries the salt, the ICV and the mode constructor
for each of the four AEAD transforms Ze specifies. Membership in it IS the AEAD
property that `EncryptionID.IsAEAD` answers. The nonce length is derived rather
than stored: RFC 5282 Section 4 builds the nonce as the salt then the IV, so AES
GCM answers twelve octets and AES CCM eleven, and a stored length could disagree
with the salt beside it.

**AES CCM is offered for the IKE SA and refused for ESP.** RFC 5282 carries AES
CCM into the IKE SA's Encrypted payload, which Ze seals and opens in software.
An ESP SA is installed into a dataplane instead, and neither backend names an AES
CCM transform, so `EncryptionImplementedESP` (`ipsec/algorithm_support.go`)
refuses one at config parse rather than letting the Linux backend install it as
AES GCM. The `ike-aes-ccm16` interop scenario holds that split against strongSwan:
the IKE SA negotiates AES CCM and the Child SA negotiates AES GCM
(`docs/architecture/testing/interop.md`).

**A flat map registry, not the registration pattern.** The algorithm set is
small and fixed: ten encryption transforms, three PRFs, three integrity
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
