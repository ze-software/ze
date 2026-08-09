# IS-IS Authentication

Per-PDU sign-on-send and verify-on-receive for all four PDU classes across both
levels, plus a key store built from YANG key chains. Three authentication types
are implemented: cleartext (type 1, sanity only), HMAC-MD5 (RFC 5304, type 54),
and the RFC 5310 generic-crypto family (type 3) covering HMAC-SHA-1, 224, 256,
384 and 512.

The **wire** contract, the TLV-first requirement, field zeroing, send and receive
order, and authenticated purges are documented in
[`../wire/isis.md`](../wire/isis.md) under "Authentication". This page carries the
structural decisions.

| Concern | File |
|---------|------|
| Algorithm types, errors, Apad, PDU classifier | `packet/auth_types.go` |
| Sign, layout, digest, checksum ordering, purge strip | `packet/auth_sign.go` |
| Verify and receive helpers | `packet/auth_verify.go` |
| Key store, lifetimes, chain resolution | `auth_keystore.go` |
| Engine glue | `auth_wiring.go` |

## Decision: the crypto backend is runtime-free

It lives in the `packet` subpackage, operates on raw PDU bytes only, and imports
no config, circuit or database layer. It is testable on bytes alone. The
three-file split is size, not behavior.

<!-- source: internal/plugins/isis/packet/auth_sign.go -- SignPDU, computeDigest, finalizeLSPChecksum, StripPurgeBody -->
<!-- source: internal/plugins/isis/packet/auth_verify.go -- VerifyPDU, verifyKey, authLayoutForReceived -->

## Decision: the pre-image differs per algorithm

The two RFCs disagree and an interop peer follows its RFC literally:

| Family | Authentication Data before the digest |
|--------|----------------------------------------|
| HMAC-MD5 (RFC 5304 section 2) | **zeroed** |
| HMAC-SHA (RFC 5310 sections 3.3 and 3.5) | filled with **Apad**, the value `0x878FE1F3` repeated |

Apad is applied on both sign and verify. Getting this wrong silently breaks every
HMAC-SHA digest against a conforming peer while HMAC-MD5 keeps working, so both
pre-images are pinned by name in the tests.

<!-- source: internal/plugins/isis/packet/auth_types.go -- the Apad fill, authTypeFor, digestLen, classOf -->

## Decision: one verify chokepoint, one sign hook per PDU class

The engine glue installs per-level signers on the originator and the flooder, a
per-interface hello signer on each circuit, and **one** verify hook on the PDU
dispatcher that runs before any handler routes the PDU to adjacency, database or
SNP processing.

No PDU path can therefore skip authentication when a chain is configured. A
wrong, missing, or not-first TLV 10 under configured authentication is rejected
before any state is touched, and the reject site increments the failure counter.

<!-- source: internal/plugins/isis/auth_wiring.go -- setKeyStore, installCircuitSigner, signLevelPDU, verifyFrame -->
<!-- source: internal/plugins/isis/server.go -- the dispatcher verify hook -->

Glue lives in `auth_wiring.go`, a sibling of `lsdb_wiring.go` and
`dis_wiring.go`, rather than being threaded through the circuit and database
packages.

## Decision: the key store selects one signing key and accepts many

The store resolves the YANG key chains into runtime keys, decoding the encoded
secrets into HMAC keys held only in memory. It selects a **single** active
signing key per chain, the first whose send lifetime contains the current time,
and accepts **all** keys whose accept lifetime contains it, which is what makes
rotation hitless.

A local level enum keeps the store free of a database or adjacency import.

<!-- source: internal/plugins/isis/auth_keystore.go -- the key store, lifetimes, decodeSecret -->

## Decision: fail open on a bad lifetime, fail closed on a bad key

An unparsable lifetime bound leaves that bound unset rather than disabling a
configured key. An unknown algorithm or an undecodable secret **drops** the key.
Chain size is capped to bound verify cost against a forged-PDU flood.

## Decision: constant-time compare everywhere

Every digest comparison uses `hmac.Equal`. There is no byte-equality or `==` on
any digest path. Cleartext also compares the password in constant time.

## Decision: one scratch buffer per verify, not one per candidate key

The verify path copies the received PDU into one scratch buffer for the whole key
chain, so a multi-key rotation chain does not allocate a copy per candidate, and
the caller's receive buffer is never mutated. The digest routine restores every
field it zeroes, so the scratch is reusable across candidate keys.

## Trap: a point-to-point hello is level-agnostic on the wire

RFC 5303 defines one PDU type with no level bit, so a receiver cannot tell from
the bytes which level negotiated. The sender signs with the **negotiated
adjacency level's** hello chain, and on a level-1-and-2 circuit the two chains
can differ.

Verify therefore tries keys from **both** per-interface hello chains for a
point-to-point hello. The per-key HMAC still rejects a forged or wrong-secret
PDU.

## Trap: the secret leaf is a sensitive leaf, not a pattern validator

The encoded secret uses the shared `ze:sensitive` machinery, the same as PPPoE
and WireGuard, rather than a bespoke pattern validator. The key store accepts
either an encoded value or a raw plaintext value, because an operator can type
one before commit encodes it.

## Coverage boundary

The functional test validates the key-chain **schema** through the real YANG: a
bad algorithm enum is rejected, and valid chains with an encoded secret and
per-level or per-interface references are accepted. Live sign, verify, wrong-key
and rotation need raw Layer-2 and are covered by the engine unit tests and the
interop scenario.

## Owned metric

`ze_isis_auth_failures_total{level,interface}`.
