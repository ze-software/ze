# 935 -- isis-10-auth

## Context
Spec `isis-10-auth` adds IS-IS authentication to the native engine: per-PDU
sign-on-send and verify-on-receive for all four PDU classes (IIH, LSP, CSNP,
PSNP) across both routing levels, plus a key store built from YANG key chains.
The structural TLV 10 (Authentication Information) codec already existed from
isis-2 (`packet/tlv_auth.go`, with the RFC-correct type bytes cleartext=1,
generic-crypto=3, HMAC-MD5=54); this spec is the crypto backend, the key store,
and the two shared hook points wired into the engine. Three auth types are
implemented: cleartext (type 1, sanity-only), HMAC-MD5 (RFC 5304, type 54), and
the RFC 5310 generic-crypto family (type 3) spanning HMAC-SHA-1/224/256/384/512.
The implementation is DONE: the tree builds on darwin and linux, all isis auth
unit tests pass under -race, and golangci-lint is clean (0 issues). On-the-wire
FRR interop validation is the only remaining item and is pending a Linux/QEMU
host (this work ran on darwin).

## Decisions
- The crypto backend lives PURE in the `packet` subpackage, runtime-free, split
  across three files (no behavior split, just size): `auth_types.go` (algorithm
  types, typed sentinel errors, per-algorithm helpers, Apad fill, PDU-class
  classifier), `auth_sign.go` (`SignPDU` + the shared encode/layout/digest/
  checksum machinery), `auth_verify.go` (`VerifyPDU` + receive helpers). The
  backend operates on raw PDU bytes only and imports no config/circuit/lsdb
  layer, so it is testable on bytes alone (spec Key Design Decision honored).
- The key store lives in the component root (`auth_keystore.go`): it resolves the
  isis-4-owned YANG key chains into runtime `packet.Key`s, decoding the `$9$`
  secrets to HMAC keys held only in memory, selecting a single ACTIVE signing key
  per chain (first key whose send-lifetime contains now) and accepting ALL keys
  whose accept-lifetime contains now on receive (hitless rotation, AC-4). A local
  `lsdbLevel` enum keeps the store free of an lsdb/adjacency import.
- Engine glue lives in a dedicated root file `auth_wiring.go` (matching the
  existing `*_wiring.go` split, e.g. dis_wiring.go/lsdb_wiring.go), NOT threaded
  through circuit/lsdb. It installs per-level signers on the Originator and
  Flooder (`SetSigner`), a per-interface IIH signer on each Circuit
  (`installCircuitSigner` from buildCircuit), and ONE verify chokepoint on the
  dispatcher (`dispatch.setVerify`) that runs BEFORE any handler routes the PDU
  to adjacency/LSDB/SNP processing (server.go dispatch: `if verify != nil &&
  !verify(rf) { return }`).
- The Authentication Data pre-image is per-algorithm because the two RFCs
  disagree and an interop peer follows the RFC literally: HMAC-MD5 (RFC 5304 sec
  2) ZEROES the value before the digest; the HMAC-SHA family (RFC 5310 sec 3.3
  step 1 / sec 3.5) fills the value with Apad (0x878FE1F3 repeated) before the
  digest, on BOTH sign and verify. Getting this wrong silently breaks every
  Ze<->FRR HMAC-SHA digest while HMAC-MD5 still works, so it is pinned by
  `TestISISAuthHMACSHAApadPreimage` and `TestISISAuthHMACMD5ZeroPreimage`.
- LSP signing zeroes three fields before the digest (RFC 5304 sec 2): the
  Authentication Value, the Checksum, and the Remaining Lifetime; the fields are
  saved and restored by `computeDigest`, and the Fletcher checksum is recomputed
  LAST (`finalizeLSPChecksum`) so the order is build -> sign -> checksum. On
  receive the checksum is accepted, then the same three fields are set to their
  pre-image, the digest recomputed, and the fields restored. IIH/CSNP/PSNP have
  no Checksum or Remaining Lifetime field, so only the Authentication Data
  pre-image is set.
- Authenticated purges (RFC 5304 sec 2): `StripPurgeBody` removes the LSP body
  and keeps only TLV 10, the originator re-signs it, and `VerifyPDU` rejects an
  unauthenticated purge (handled by the missing/failed digest) and a purge that
  carries any TLV other than TLV 10 (`ErrAuthPurgeExtraTLV`). This blocks the
  spoofed-purge attack (R-7).
- Every digest comparison uses `hmac.Equal` (constant time, AC-12/R-1); there is
  no `bytes.Equal`/`==` on any digest path. Cleartext also constant-time compares
  the password.

## Consequences
- One verify chokepoint + one sign hook per PDU class means no PDU path can skip
  authentication when a chain is configured: a wrong/missing/not-first TLV 10
  under configured auth is rejected before adjacency/LSDB/SNP processing, and the
  reject site increments `ze_isis_auth_failures_total{level,interface}` (this
  spec OWNS and registers that series; isis-13 only scrapes it).
- The verify path copies the received PDU into ONE scratch buffer for the whole
  key chain (not one copy per candidate key), so a multi-key rotation chain does
  not allocate N PDU copies, and the caller's receive buffer is never mutated
  (zero-copy preserved). `computeDigest` restores every field it zeroes so the
  scratch is reusable across candidate keys.
- Owned/registered metric: `ze_isis_auth_failures_total{level,interface}`
  (server.go), default NopRegistry until `RegisterMetrics`, documented in
  docs/plugin-development/metrics.md.

## Gotchas
- **A P2P Hello is level-agnostic on the wire** (RFC 5303: one PDU type, no level
  bit), so the receiver cannot tell from the bytes which level negotiated. The
  sender signs a P2P Hello with the NEGOTIATED adjacency level's IIH chain (which
  on an L1L2 circuit may be L1 or L2 and the two chains can differ). `verifyFrame`
  therefore tries keys from BOTH per-interface IIH chains (L1 and L2) for a P2P
  Hello; the per-key HMAC still rejects a forged or wrong-secret PDU. Pinned by
  `TestISISAuthP2PHelloBothChains`.
- **`TestISISAuthSignLSP` named in the spec Wiring Test does not exist as a
  literal test name.** LSP sign-on-send is covered instead by
  `TestISISAuthLSPChecksumAfterSign` (checksum-after-sign + 3-field zeroing
  round-trip), `TestISISAuthSignDeterministic`, and the engine-level
  `TestISISAuthEngineSignLevel`. This is a naming deviation, not a coverage gap.
- **The `$9$` secret leaf uses `ze:sensitive` in the YANG**, not a bespoke
  `$9$`-pattern custom validator. Commit-time masking/encoding is handled by the
  shared sensitive-leaf machinery (same as PPPoE/WireGuard), and the key store's
  `decodeSecret` accepts either a `$9$`-encoded value (decoded via
  `internal/component/config/secret`) or a raw plaintext value (operator typed it
  before commit auto-encoded it). A malformed/undecodable secret drops the key
  rather than silently weakening auth.
- **`test/isis/isis-auth.ci` is config-surface only by design.** It validates the
  key-chain schema (bad algorithm enum rejected, valid chains + `$9$` secret +
  per-level/per-interface refs accepted) through the real YANG. Live
  sign/verify/wrong-key/rotation need raw L2 (AF_PACKET on a Linux veth) and are
  covered by the engine unit tests and the FRR interop scenario, not by the .ci.
  `.ci` files are auto-discovered by glob (`internal/test/runner/decoding.go`),
  so no per-file pin is required.
- The keystore intentionally fails OPEN on an unparseable lifetime bound (a
  malformed RFC3339 timestamp leaves that bound unset rather than disabling a
  configured key) but fails CLOSED on an unknown algorithm or undecodable secret
  (the key is dropped). Chain size is capped at `maxKeysPerChain = 16` to bound
  verify cost against a forged-PDU flood (Security Review: resource use).

## Interop validation pending Linux execution
The FRR interop scenario `test/interop/scenarios/isis-auth-frr` is WRITTEN
(check.py + frr.conf with `isis password md5` / `area-password md5` /
`domain-password md5`, ze.conf with matching HMAC-MD5 key chains) and asserts an
HMAC-MD5-authenticated Ze<->FRR adjacency forms and stays Up with matched keys.
It runs under the Linux Docker/QEMU interop harness ONLY (raw L2 + FRR isisd) and
was NOT executed in this session (darwin host). Its check.py header tags it
spec-isis-13 AC-16 (the interop harness lives in isis-13); it directly validates
this spec's User Story 4. Status: scenario `isis-auth-frr` written; execution
pending Linux/QEMU. The wire/crypto contract it exercises is fully specified and
unit-proven here (the Apad/zero pre-image, type bytes, TLV-first, LSP 3-field
zeroing, checksum-after-sign).

## Files
- `internal/plugins/isis/packet/auth_types.go` (+`auth_types_test.go`):
  algorithm enum, typed errors, authTypeFor/digestLen/newHash/keyIDOctets,
  Apad fill, placeholderValue, pduClass/classOf.
- `internal/plugins/isis/packet/auth_sign.go` (+`auth_sign_test.go`): SignPDU,
  reencodeWithAuthFirst, the *AuthLayout locators, computeDigest,
  finalizeLSPChecksum, StripPurgeBody.
- `internal/plugins/isis/packet/auth_verify.go` (+`auth_verify_test.go`):
  VerifyPDU, verifyKey, authLayoutForReceived, pduTLVs (the bulk of the
  per-algorithm/per-PDU-class/boundary tests live in auth_verify_test.go).
- `internal/plugins/isis/auth_keystore.go` (+`auth_keystore_test.go`): the key
  store, lifetimes, per-PDU-class chain resolution, `$9$` decode.
- `internal/plugins/isis/auth_wiring.go` (+`auth_wiring_test.go`): engine glue
  -- setKeyStore, installCircuitSigner, signLevelPDU/signHelloPDU, verifyFrame.
- Modified: `server.go` (dispatcher verify hook + setVerify; authFailures
  CounterVec register), `circuits.go` (installCircuitSigner on buildCircuit),
  `config.go` (KeyConfig/KeyChainConfig + parseKeyChain + level/iface auth refs),
  `lsdb/origination.go` + `lsdb/flooding.go` + `circuit/circuit.go` (SetSigner),
  `yang/ze-isis-conf.yang` (key-chains list, algorithm enum, `$9$` sensitive
  secret, send/accept-lifetime, per-level/per-interface auth-key-chain refs).
- `test/isis/isis-auth.ci`: config-surface validation (bad algo rejected; valid
  chains + `$9$` secret + refs accepted).
- `test/interop/scenarios/isis-auth-frr/` (check.py, frr.conf, ze.conf): HMAC-MD5
  Ze<->FRR adjacency; Linux/QEMU-only, execution pending.
- `rfc/short/rfc5304.md`, `rfc/short/rfc5310.md`: RFC summaries (pre-existing).
- Docs: `docs/plugin-development/metrics.md` (auth-failures row + description),
  `docs/guide/isis.md` and `docs/architecture/wire/isis.md` (auth sections).
