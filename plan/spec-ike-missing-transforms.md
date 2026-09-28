# Spec: ike-missing-transforms

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze's IPsec surface names IKEv2 transforms its crypto registry does not implement.
A doc sync on 2026-09-28 found `docs/guide/ipsec.md` claiming ChaCha20-Poly1305,
MODP 3072/4096/8192 (Diffie-Hellman groups 15, 16, 18) and ECP 521 (group 21).
The registry in `internal/component/ike/crypto/transform.go` holds DH groups 14,
19 and 20 only, and it holds no ChaCha20-Poly1305 entry, so the YANG enum
`chacha20poly1305` is refused at config parse. The guide has been corrected;
the "IKEv2 Cryptographic Primitives" row of `docs/features.md` still carries the
false claim (see Known Limitations).

Thomas asked for these transforms to be implemented:

| Transform | Wire identity | RFC | Where |
|-----------|---------------|-----|-------|
| ChaCha20-Poly1305 | ENCR 28, `ENCR_CHACHA20_POLY1305` | RFC 7634 (on RFC 8439) | IKE SA Encrypted payload and ESP Child SA |
| 3072-bit MODP | D-H 15 | RFC 3526 Section 4 | IKE SA and Child SA PFS |
| 4096-bit MODP | D-H 16 | RFC 3526 Section 5 | IKE SA and Child SA PFS |
| 8192-bit MODP | D-H 18 | RFC 3526 Section 7 | IKE SA and Child SA PFS |
| 521-bit random ECP | D-H 21 | RFC 5903 Section 3.3 | IKE SA and Child SA PFS |

3DES (`3des`, ENCR 3) is NOT in scope. There is no reason to add it: RFC 8221
Section 5 rates ENCR_3DES "SHOULD NOT" for ESP, RFC 8247 Section 2.1 downgrades it
to "MAY" for IKEv2 and says "there is no need to keep support for the much slower
ENCR_3DES", and no interop need asks for it. Its YANG enum stays named and
refused at commit, as today; removing it is not part of this spec (Key Design
Decisions, D-3, decided by Thomas 2026-09-28). MODP 6144 (group 17) was not asked for and stays out (Known
Limitations).

The goal: an operator configures any of the five transforms in an `ike-group` or
`esp-group`, the config loads, the IKE SA and Child SA negotiate it against
strongSwan, traffic flows through the installed SA, and `show vpn ipsec sa` names it.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-6-ikev2-crypto.md` - the crypto primitives layer this spec extends
  → Decision: "One table decides every AEAD property, keyed on the wire Transform ID." ChaCha20-Poly1305 joins `aeadTransforms` (`aead.go`); membership there IS `EncryptionID.IsAEAD`.
  → Decision: "Standard library crypto only, no CGo and no external library." ChaCha20-Poly1305 has no standard-library package, so this decision is amended to allow the already-vendored `golang.org/x/crypto/chacha20poly1305` (used today by `internal/appliance/crypto.go`). The doc paragraph is rewritten in the same phase.
  → Constraint: "MODP 2048 private keys are drawn from the range 2 to p-2. Public key validation rejects 0, 1 and p-1." The generalised MODP path keeps both rules for every MODP group.
  → Constraint: "A flat map registry, not the registration pattern ... three DH groups". The counts in that paragraph become wrong and are rewritten.
- [ ] `docs/architecture/ike/ipsec-3-data-model.md` - the data model the YANG enums belong to (it states nothing about the refused set; the gate is documented in the header comment of `ipsec/algorithm_support.go`)
  → Constraint: from `algorithm_support.go`: the YANG enum names every algorithm the data model can name; the build's set is decided by the crypto registry, and `ParseIPsecConfig` (`ipsec/config.go`) refuses the difference, naming `SupportedEncryptionNames()` / `SupportedDHGroupIDs()` in the error. No new gate is written: adding registry entries is what lifts the refusal.
- [ ] `docs/architecture/testing/interop.md` - "AES CCM on the strongSwan peer"
  → Decision: `ike-aes-ccm16` is the template: charon's `selected proposal:` log line (a fact about the octets Ze sent), then Ze's own `show vpn ipsec sa` field, then XFRM state on both sides, then `verifyTunnelTraffic`. New scenarios follow the same three-observation shape.
  → Constraint: a scenario is registered by name in `scenarioCheckers` (`internal/le/interoplab/ipsec/checkers.go`) and its directory name carries no numeric prefix.
- [ ] `ai/rules/interop-and-goal-validation.md`, `ai/rules/rfc-compliance.md`
  → Constraint: each new interop scenario owes a recorded red: revert the registry entry, rebuild the `ze-linux` image the lab drives, confirm red, restore, confirm green.
  → Constraint: functions added from 2026-09-24 quote the RFC sentence they implement; callers name the RFC section.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7634.md` - DOES NOT EXIST. Created in Phase 1 with `/ze-rfc` from `rfc/full/rfc7634.txt` (fetched 2026-09-28).
  → Constraint: RFC 7634 Section 2: "The KEYMAT requested for each ChaCha20-Poly1305 key is 36 octets. The first 32 octets are the 256-bit ChaCha20 key, and the remaining 4 octets are used as the Salt value in the nonce."
  → Constraint: RFC 7634 Section 2: "A 32-bit Salt is prepended to the 64-bit IV to form the 96-bit nonce." This is exactly `ikeAEADNonce` (salt then IV), so the nonce builder needs no change.
  → Constraint: RFC 7634 Section 3: "36 octets are requested for each of SK_ei and SK_er, of which the first 32 form the key and the last 4 form the salt. No octets are requested for SK_ai and SK_ar."
  → Constraint: RFC 7634 Section 3: "The sender SHOULD include no padding and set the Pad Length field to zero. The receiver MUST accept any length of padding." `buildSKMessageAEADWithMsgID` (`engine/auth.go`) already sends Pad Length 0 with no padding for every AEAD, and `TestRFC5282AEADReceiveAcceptsAnyPaddingTo255` (`engine/rfc5282_aead_sk_test.go`) covers the receive rule for the existing AEADs.
  → Constraint: RFC 7634 Section 4: "the value ENCR_CHACHA20_POLY1305 (28) should be used in the transform substructure of the SA payload as the ENCR (type 1) transform ID. As with other AEAD algorithms, INTEG (type 3) transform substructures MUST NOT be specified, or just one INTEG transform MAY be included with value NONE (0)."
  → Constraint: RFC 7634 Appendix B is a complete IKEv2 INFORMATIONAL known-answer vector (key 0x80..0x9f, salt a0a1a2a3, iSPI c0..c7, rSPI d0..d7, Message ID 9). It is the IKE-side unit oracle.
- [ ] `rfc/short/rfc3526.md` - DOES NOT EXIST. Created in Phase 1 with `/ze-rfc`.
  → Constraint: RFC 3526 Sections 4, 5 and 7: "This group is assigned id 15." / "id 16." / "id 18.", each with "The generator is: 2."
  → Constraint: RFC 3526 Section 8 gives strength estimates and exponent sizes (group 15: "130 | 260-" and "210 | 420-"; group 18: "190 | 380-" and "310 | 620-"). It states no MUST on exponent length; D-2 keeps full-length exponents.
- [ ] `rfc/short/rfc5903.md` - DOES NOT EXIST although `crypto/rfc5903_ecp_test.go` cites it. Created in Phase 1 with `/ze-rfc`.
  → Constraint: RFC 5903 Section 7: "Each component MUST have bit length as given in the following table" with "521-bit Random ECP Group 528", so a group 21 public value is 2 x 66 = 132 octets and carries no SEC 1 tag.
  → Constraint: RFC 5903 Section 7: "The Diffie-Hellman shared secret value consists of the x value of the Diffie-Hellman common value." For P-521 that is 66 octets; `crypto/ecdh` answers it.
  → Constraint: RFC 5903 Section 8.3 carries the 521-bit test vector.
- [ ] `rfc/short/rfc7296.md` - exists
  → Constraint: RFC 7296 Section 3.3.5: "The Key Length attribute MUST NOT be used with transforms that use a fixed-length key." ChaCha20-Poly1305 has a fixed 256-bit key (RFC 7634 Section 2: "The encryption key is 256 bits."), so Ze never sends the attribute on ENCR 28 and refuses an offer that carries one.
  → Constraint: RFC 7296 Section 3.3.2 table: "3072-bit MODP Group 15", "4096-bit MODP Group 16", "8192-bit MODP Group 18". Group 21 comes from the IANA registry the same section points to.
- [ ] `rfc/full/rfc8247.txt`, `rfc/full/rfc8221.txt` - algorithm requirement levels
  → Decision: RFC 8247 Section 2.1 rates `ENCR_CHACHA20_POLY1305` "SHOULD" for IKEv2; RFC 8221 Section 5 rates it "SHOULD" for ESP. Groups 15, 16, 18 and 21 are absent from the RFC 8247 Section 2.4 table, so they are MAY. No new MUST enters the conformance count from these two documents.

**Key insights:**
- The YANG already offers `chacha20poly1305`, and `dh-group` is `uint8 { range "1..31" }`: no new leaf. The refusal is `EncryptionImplemented` / `DHGroupImplemented` (`ipsec/algorithm_support.go`) reading the crypto registry, so a registry entry lifts it.
- Both dataplanes already name ChaCha20-Poly1305 for ESP: `xfrmAEADNames` maps `chacha20poly1305` to `rfc7539esp(chacha20,poly1305)` (`dataplane/xfrm_linux.go`), and `aeadSaltBytes` answers 4 and `vppCryptoAlg` maps `IPSEC_API_CRYPTO_ALG_CHACHA20_POLY1305` (`dataplane/vpp.go`). strongSwan's `kernel_netlink_ipsec.c` uses the same kernel name, and Linux `crypto/chacha20poly1305.c` refuses a key whose length is not `saltlen + CHACHA_KEY_SIZE`, so the XFRM key must be the 36 KEYMAT octets.
- `EncryptionTransform.KeyLength` does two jobs today: the wire Key Length attribute AND the key size `encKeyMaterialLen` derives KEYMAT from. For ChaCha20-Poly1305 the attribute is absent (0) and the key is 256 bits. Left as is, KEYMAT would be 0 + 4 = 4 octets. This is the central design change.
- The DH group is spread over seven lists today (see Current Behavior). Four new groups would be 28 edits. The design collapses them into one table.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/crypto/transform.go` (291L) - IANA ID constants; `EncryptionID.String` / `DHGroupID.String` switches; `encryptionRegistry` (10 names), `dhGroupRegistry` (14, 19, 20); `SupportedDHGroupIDs`, `LookupDHGroup`.
- [ ] `internal/component/ike/crypto/dh.go` - `modp2048Prime` / `modp2048Generator` / `modp2048Len`; `ecpCoordBits` (19, 20); `NewDHExchange` and `SharedSecret` each a switch over three groups; MODP length and `1 < y < p-1` checks; ECP SEC 1 / wire conversions.
- [ ] `internal/component/ike/crypto/aead.go` - `aeadTransforms` (GCM 16, CCM 8/12/16) with `saltOctets`, `icvOctets`, and a `mode` constructor that takes an AES `cipher.Block`; `newIKEAEAD` always calls `aes.NewCipher`; `SealIKEAEAD`, `OpenIKEAEAD`, `ikeAEADNonce`, `AEADICVOctets`.
- [ ] `internal/component/ike/crypto/keys.go` - `encKeyMaterialLen` answers `KeyLength / 8` plus the AEAD salt.
- [ ] `internal/component/ike/crypto/proposal.go` - `specifiedEncryption`, `specifiedDHGroup`, `keyLengthRequired` (its comment already says ChaCha20 carries no Key Length), `acceptEncryption` (copies `remote.KeyLength` into the chosen transform), `acceptDHGroup`.
- [ ] `internal/component/ike/ipsec/types.go` - `EncryptionChaCha20Poly` / `Encryption3DES` already in the enum and name map; `IsAEAD` already true for ChaCha20.
- [ ] `internal/component/ike/ipsec/algorithm_support.go` - `EncryptionImplemented`, `EncryptionImplementedESP` (refuses AES CCM only), `DHGroupImplemented`.
- [ ] `internal/component/ike/ipsec/config.go` - the ESP and IKE proposal parsers: the refusals that name the supported sets.
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - enum help "ChaCha20-Poly1305 AEAD (RFC 7634). No build implements it."; the `encryption` leaf descriptions in `esp-group` and `ike-group` say "No build carries a transform for chacha20poly1305 or 3des"; `dh-group` range 1..31.
- [ ] `internal/component/ike/engine/initiator.go` - `encAttrs` emits Key Length only when `KeyLength` is not 0; `wireEncryptionTransform` reads it off the wire.
- [ ] `internal/component/ike/engine/auth.go` - `buildSKMessageAEADWithMsgID`: IV 8, Pad Length 0, no padding; AAD is the first 32 octets.
- [ ] `internal/component/ike/engine/reconcile.go` - the Child SA info builder sets `ESPKeyBits` from the configured transform's `KeyLength` (would report 0 for ChaCha20).
- [ ] `internal/component/ike/cmd/show_ipsec.go` - `saToMap` renders `encryption` and `dh-group` from `ID.String()` (would render `unknown` for a new ID).
- [ ] `internal/component/ike/dataplane/xfrm_linux.go`, `dataplane/vpp.go` - ChaCha20 already mapped (see Key insights).
- [ ] `internal/le/interoplab/ipsec/checkers.go` - `scenarioCheckers` map; `checkIKEAESCCM16`.
- [ ] `test/interop-ipsec/scenarios/ike-aes-ccm16/` - template swanctl.conf / ze.conf.
- [ ] `test/ipsec/ipsec-sa-installed.ci` - two unprivileged ze daemons negotiate over loopback and assert `show vpn ipsec sa`.
- [ ] `gokrazy/kernel/runtime.config`, `internal/appliance/kernelreq.go` - `CONFIG_INET_ESP` is required; no ChaCha20-Poly1305 crypto option is named anywhere.

The DH group is declared in seven places today:

| # | Site | File |
|---|------|------|
| 1 | `DH_MODP_2048` / `DH_ECP_256` / `DH_ECP_384` constants | `crypto/transform.go` |
| 2 | `DHGroupID.String` switch | `crypto/transform.go` |
| 3 | `dhGroupRegistry` | `crypto/transform.go` |
| 4 | `specifiedDHGroup` | `crypto/proposal.go` |
| 5 | `ecpCoordBits` | `crypto/dh.go` |
| 6 | `NewDHExchange` switch | `crypto/dh.go` |
| 7 | `SharedSecret` switch | `crypto/dh.go` |

**Behavior to preserve:**
- Groups 14, 19, 20 and every existing cipher negotiate byte-identically: same KE lengths, same KEYMAT sizes, same wire attributes.
- The parse-time refusal for anything the registry lacks (3des, sha1, groups outside the registry) and its error text listing the implemented set.
- `EncryptionImplementedESP` still refuses AES CCM for ESP.
- MODP private exponent range 2..p-2 and public validation rejecting 0, 1, p-1, extended unchanged to groups 15, 16 and 18 (D-2).
- ECP: refusal of the SEC 1 tagged form and of a wrong-length value.
- `show vpn ipsec sa` JSON keys and existing values.

**Behavior to change:**
- `chacha20poly1305` in an `ike-group` or `esp-group` loads and negotiates instead of being refused.
- `dh-group` 15, 16, 18, 21 load and negotiate instead of being refused.
- YANG help and description text stops saying no build implements ChaCha20-Poly1305.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: an `ike-group` proposal with `encryption chacha20poly1305`, `hash sha256`, `dh-group 21`, and an `esp-group` proposal with `encryption chacha20poly1305`.
- Wire: a peer's IKE_SA_INIT / CREATE_CHILD_SA SA payload offering ENCR 28 or D-H 15/16/18/21, and its KE payload.

### Transformation Path
1. `ipsec.ParseIPsecConfig` resolves names; `EncryptionImplemented` / `EncryptionImplementedESP` / `DHGroupImplemented` consult `crypto.LookupEncryption` / `crypto.LookupDHGroup`.
2. Engine builds proposals (`encAttrs` in `initiator.go`: no Key Length for ENCR 28) and a `crypto.NewDHExchange(group)` KE value.
3. Responder / initiator negotiate: `crypto.acceptEncryption` (new: refuse a Key Length attribute on a fixed-key transform), `acceptDHGroup` (derived specified set).
4. `DHExchange.SharedSecret`, then `DeriveSKEYSEED`, then `DeriveSKKeys` (SK_e 36 octets each for ENCR 28, SK_a 0), then `SealIKEAEAD` / `OpenIKEAEAD` via the ChaCha20-Poly1305 mode.
5. Child SA: `DeriveChildSAKeys` / `DeriveChildSAKeysPFS` (36-octet encryption keys, no integrity keys), then `dataplane.SAParams` with `EncAlgo` `chacha20poly1305`, `IsAEAD` true and a 36-octet `EncKey`, then XFRM `rfc7539esp(chacha20,poly1305)` with ICV 128, or VPP `CHACHA20_POLY1305` with the salt split.
6. `show vpn ipsec sa` renders `EncryptionID.String` / `DHGroupID.String` and `ESPKeyBits`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to engine | typed `ipsec.IKEProposal` / `ESPProposal` | No |
| Engine to peer | IKEv2 SA / KE payloads, Encrypted payload | No |
| Engine to kernel | XFRM netlink `xfrm_algo_aead` name, key, ICV length | No |
| Engine to VPP | `ipsec_sad_entry_v3` crypto alg, key, salt | No |

### Integration Points
- `crypto.aeadTransforms` - gains ENCR 28, and its entry shape changes (Key Design Decisions D-A).
- `crypto.encKeyMaterialLen` - reads the fixed key size from the transform identity.
- `crypto.acceptEncryption` - gains the fixed-key refusal.
- New DH group table in `crypto` - the seven sites above derive from it.
- `engine/reconcile.go` - `ESPKeyBits` reads the effective key bits.
- `internal/le/interoplab/ipsec/checkers.go` - five new named scenarios: `chacha20poly1305-ike-esp` and one per DH group (D-1).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | New entries go through the existing registry, parse gate, negotiator and dataplane maps |
| No unintended coupling (components stay isolated) | Yes | All code stays in `internal/component/ike/crypto` plus one field read in `engine/reconcile.go` |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `ikeAEADNonce`, `SealIKEAEAD`, the ECP helpers and the MODP checks, generalised over a table |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Control-plane crypto only; allocation profile unchanged from the AES-GCM path |
| Registration over hardcoding, outbound | Yes | A DH group is one table row; no switch arm is added |
| Registration over hardcoding, inbound | Yes | Lists searched for the new names: the seven DH sites (collapsed to one table); `encryptionRegistry`, `specifiedEncryption`, `EncryptionID.String`, `aeadTransforms` (each gains one entry; bound by `aead_predicate_test.go` and `TestAcceptedAlgorithmsResolveToTransforms`); `xfrmAEADNames`, `aeadSaltBytes`, `vppCryptoAlg` (already hold ChaCha20); `ipsec.encryptionNames` (already holds it); `scenarioCheckers` (one row per scenario, the lab's existing pattern) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The strongSwan lab image (Alpine 3.21, strongSwan 5.9.14) registers `CHACHA20_POLY1305`, `MODP_3072`, `MODP_4096`, `MODP_8192` and `ECP_521` | the openssl plugin already provides AES CCM in that image (`Dockerfile.strongswan` comment, measured 2026-09-14) | A scenario cannot negotiate; the image needs another plugin | `swanctl --list-algs` in `ze-ipsec-strongswan`, recorded in `docs/architecture/testing/interop.md` | unvalidated |
| A-2 | The interop host kernel carries `rfc7539esp(chacha20,poly1305)` (CONFIG_CRYPTO_CHACHA20POLY1305) | Mainstream distro and Docker Desktop kernels build it | ESP install fails on both peers; the ESP half of the scenario is red for an environment reason | `grep rfc7539esp /proc/crypto` inside the lab container, or a first scenario run | unvalidated |
| A-3 | The appliance runtime kernel does not yet enable CONFIG_CRYPTO_CHACHA20POLY1305 | Absent from `gokrazy/kernel/runtime.config`; the gokrazy base config was not read | If already on, the added line is redundant but harmless; the require entry still pins it | Read the built kernel `.config`, or boot the QEMU image and read `/proc/crypto` | unvalidated |
| A-4 | `golang.org/x/crypto/chacha20poly1305.New` answers a 12-octet nonce and 16-octet overhead, matching RFC 7634 | Package contract; already vendored for `internal/appliance/crypto.go` | Constructor refuses; the mode constructor's size check catches it | the size check in the new mode constructor plus the RFC 7634 Appendix B vector | unvalidated |
| A-5 | Go `crypto/ecdh.P521` public key bytes are 133 octets (tag plus 2 x 66) and ECDH answers 66 octets | Go documentation; RFC 5903 Section 7 | ECP helpers refuse the value | RFC 5903 Section 8.3 vector test | unvalidated |
| A-6 | strongSwan uses full-length MODP exponents by default | `src/libstrongswan/crypto/key_exchange.c` on strongSwan master: the `dh_exponent_ansi_x9_42` setting defaults to TRUE and then sets `exp_len` to the prime length (read 2026-09-28) | D-2's rationale ("as strongSwan's default") is wrong; the decision itself stands on Ze's documented rule | Re-read at implementation against the lab's 5.9.14 | validated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A mistyped MODP prime digit gives a silent key-agreement failure against other peers while Ze-to-Ze works | Interop scenario for that group fails at IKE_AUTH decrypt; two-Ze tests stay green | `TestRFC3526MODPPrimesAreSafePrimes` (prime, safe prime, 64 one-bits top and bottom, exact bit length) plus a strongSwan scenario per group |
| R-2 | ChaCha20 KEYMAT sized from the wire Key Length (0) gives 4-octet keys | `TestRFC7634KeymatIs36Octets` red; XFRM install EINVAL; strongSwan AUTHENTICATION_FAILED | Key size taken from the transform identity (D-A), never from the attribute |
| R-3 | A peer offers ENCR 28 with a Key Length attribute and Ze echoes it back, violating RFC 7296 Section 3.3.5 | `TestRFC7296FixedKeyTransformRefusesKeyLength` red | `acceptEncryption` refuses the transform; `encAttrs` never emits it for a fixed-key transform |
| R-4 | MODP 8192 costs about 149 ms per modular exponentiation on the development machine (measured 2026-09-28, Go 1.27.1 darwin/arm64, full-length exponent; 9.5 ms with a 512-bit exponent; 3072 costs 8.6 ms, 4096 costs 18.9 ms, P-521 keygen twice plus ECDH 1.6 ms), so about 300 ms of CPU per responder IKE_SA_INIT, more on appliance hardware | `BenchmarkDHExchange/modp8192`; IKE_SA_INIT latency | The existing cookie threshold bounds half-open SAs. Full-length exponents are kept (D-2, decided 2026-09-28): the cost is accepted, not engineered away, and `BenchmarkDHExchange` records it |
| R-5 | `show vpn ipsec sa` renders `unknown` for a new ID, or ESP key bits 0 for ChaCha20 | Functional `.ci` asserting the rendered names | Names derive from the tables; `ESPKeyBits` reads the effective key bits |
| R-6 | Changing the `aeadTransform` mode constructor from an AES block to a key changes AES GCM / CCM keying | Existing RFC 5282 GCM/CCM tests red | The AES constructors build the block themselves; existing tests run unchanged |
| R-7 | Tests that pin today's refusal go red: `TestParseRejectsUnimplementedEncryption` iterates `chacha20poly1305`; `TestTransformRegistryUnknown` asserts group 21 absent | Red on first run | These assert behaviour this spec changes. The chacha row goes and 3des stays (still refused); the group-21 row moves to a group still absent (17). No refusal assertion is dropped |
| R-8 | The appliance lacks the kernel crypto for ESP ChaCha20, so the tunnel never installs on the product | XFRM newsa error in the log; QEMU evidence | Add CONFIG_CRYPTO_CHACHA20POLY1305 to `runtime.config`, `runtime.require` and `runtimeKernelRequirements` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | New transforms fail to negotiate or key wrongly (tunnel down; AEAD verification fails closed, so no silent exposure). A regression in the refactor could break groups 14/19/20 or AES GCM/CCM, which every existing tunnel uses |
| How is it reverted? | Single commit revert; no config migration (the YANG names already exist) |
| Who else touches this path? | `plan/spec-ike-post-quantum.md` (ML-KEM in the same DH registry; skeleton), `plan/spec-crypto-policy.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ike-group` and `esp-group` naming `chacha20poly1305` with `dh-group 21`, two ze daemons | → | `SealIKEAEAD` / `OpenIKEAEAD` ChaCha20 mode, `NewDHExchange` for group 21, Child SA keyed at 36 octets | `test/ipsec/ipsec-sa-chacha20poly1305.ci` |
| `ike-group` naming `dh-group 18`, two ze daemons | → | MODP table row 18 in `NewDHExchange` / `SharedSecret` | `test/ipsec/ipsec-sa-modp8192.ci` |
| Config naming each new transform | → | `ParseIPsecConfig` accepts; `EncryptionImplementedESP` true for ChaCha20 | `TestParseAcceptsNewTransforms` (`internal/component/ike/ipsec/algorithm_support_test.go`) |
| strongSwan peer, IKE and ESP ChaCha20-Poly1305 | → | Negotiation, Encrypted payload, XFRM install | interop `chacha20poly1305-ike-esp` |
| strongSwan peer, each new DH group | → | KE payload and shared secret | interop `ike-modp3072`, `ike-modp4096`, `ike-modp8192`, `ike-ecp521` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with an `ike-group` proposal `encryption chacha20poly1305`, `hash sha256`, `dh-group 19` | Config loads; no refusal |
| AC-2 | Config with an `esp-group` proposal `encryption chacha20poly1305` and no hash | Config loads; the ESP proposal offers ENCR 28 with no INTEG transform |
| AC-3 | Config with `dh-group` 15, 16, 18 or 21 | Config loads; the refusal for an absent group (for example 17) lists 14, 15, 16, 18, 19, 20, 21 |
| AC-4 | Ze offers ENCR 28 (IKE and ESP) | The ENCR transform substructure carries ID 28 and NO Key Length attribute |
| AC-5 | Peer offers ENCR 28 with a Key Length attribute | Ze does not select that transform (NO_PROPOSAL_CHOSEN if nothing else matches), under both the initiator and responder key-length rules |
| AC-6 | IKE SA negotiated with ENCR 28 | SK_ei and SK_er are 36 octets each, SK_ai and SK_ar are 0 octets; the Encrypted payload carries an 8-octet IV, Pad Length 0 and a 16-octet ICV |
| AC-7 | RFC 7634 Appendix B inputs | `OpenIKEAEAD` over the appendix's ciphertext and AAD answers the appendix's plaintext; one flipped ICV bit answers `ErrDecryptionFailed` |
| AC-8 | Child SA negotiated with ENCR 28 | Each direction's encryption key is 36 octets (32 key, 4 salt) with no integrity key; XFRM receives `rfc7539esp(chacha20,poly1305)`, ICV 128, key length 288 bits |
| AC-9 | Each MODP group 15, 16, 18 | KE public value is exactly 384 / 512 / 1024 octets; a shorter or longer value, and the values 0, 1 and p-1, are refused; two exchanges agree on the secret; the private exponent is drawn from 2..p-2, full length (D-2) |
| AC-10 | Group 21 | KE public value is 132 octets (X then Y, 66 each, no tag); a 133-octet SEC 1 value is refused; the RFC 5903 Section 8.3 vector reproduces both public values and the 66-octet shared secret |
| AC-11 | Each new transform against strongSwan | IKE SA ESTABLISHED, charon logs the selected proposal naming the transform, Child SA installed on both sides, ESP flows in both directions |
| AC-12 | `show vpn ipsec sa` for an SA using the new transforms | `encryption` names ChaCha20-Poly1305, `dh-group` names the group (never `unknown`), ESP key bits report 256 |
| AC-13 | Groups 14, 19, 20 and AES CBC/GCM/CCM | All existing tests pass unchanged; KE and KEYMAT lengths unchanged |
| AC-14 | Config naming `3des` | Still refused at load with the implemented set listed; the enum stays in the YANG (D-3) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures ChaCha20-Poly1305 for IKE and ESP with ECP 521 toward a peer | config, parse gate, proposals, KE, SK derivation, ChaCha20 SK payload, Child SA keys, XFRM, ESP traffic | interop `chacha20poly1305-ike-esp`, `test/ipsec/ipsec-sa-chacha20poly1305.ci` |
| 2 | Configures MODP 8192 for a site-to-site peer | config, parse gate, KE, shared secret, SA up | interop `ike-modp8192`, `test/ipsec/ipsec-sa-modp8192.ci` |
| 3 | Reads `show vpn ipsec sa` | `saToMap` renders table-derived names | assertions in both `.ci` files above |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDHGroupTableDerivesEveryList` | `internal/component/ike/crypto/dh_test.go` | every table row is in `SupportedDHGroupIDs`, specified for negotiation, has a name other than `unknown`, and `NewDHExchange` succeeds; no ID outside the table is accepted by any of them | |
| `TestRFC3526MODPPrimesAreSafePrimes` | `internal/component/ike/crypto/rfc3526_modp_test.go` | groups 14, 15, 16, 18: p has exactly N bits, top and bottom 64 bits all ones, p and (p-1)/2 pass `ProbablyPrime(64)`, generator 2 | |
| `TestDHGroupSharedSecretAgrees` | `internal/component/ike/crypto/dh_test.go` | table-driven over every group: two exchanges derive equal secrets of the group's length | |
| `TestRFC7296MODPPublicValueMatchesModulusLength` (extend) | `internal/component/ike/crypto/rfc7296_dh_test.go` | rows for 15, 16, 18 (384, 512, 1024 octets) | |
| `TestRFC7296MODPShortPublicValueIsRefusedOnReceipt` (extend) | `internal/component/ike/crypto/rfc7296_dh_test.go` | rows for 15, 16, 18 | |
| `TestDHInvalidPublicKey` (extend) | `internal/component/ike/crypto/dh_test.go` | 0, 1, p-1 refused for each MODP group | |
| `TestMODPPrivateExponentIsFullLength` | `internal/component/ike/crypto/dh_test.go` | D-2, AC-9: for each MODP group, every private exponent drawn over repeated exchanges lies in 2..p-2 and at least one has a bit length within 64 bits of p's, so a short-exponent regression goes red | |
| `TestECPPublicValueOctetLength` (extend) | `internal/component/ike/crypto/rfc5903_ecp_test.go` | group 21 answers 132 | |
| `TestECPKEPayloadMatchesRFC5903Vector` (extend) | `internal/component/ike/crypto/rfc5903_ecp_test.go` | Section 8.3 vector | |
| `TestECPRejectsNonConformingLength` (extend) | `internal/component/ike/crypto/rfc5903_ecp_test.go` | 131 and 133 octets refused for group 21 | |
| `TestRFC7634AppendixBIKEv2Vector` | `internal/component/ike/crypto/rfc7634_chacha_test.go` | AC-7 | |
| `TestRFC7634SealOpenRoundTrip` | `internal/component/ike/crypto/rfc7634_chacha_test.go` | seal then open answers the plaintext; output is the 8-octet IV, the ciphertext and 16 ICV octets | |
| `TestRFC7634KeymatIs36Octets` | `internal/component/ike/crypto/rfc7634_chacha_test.go` | `encKeyMaterialLen` of a wire-read ENCR 28 transform (KeyLength 0) is 36; `DeriveSKKeys` gives SK_e 36 and SK_a 0; `DeriveChildSAKeys` gives 36 and 0 | |
| `TestRFC7296FixedKeyTransformRefusesKeyLength` | `internal/component/ike/crypto/rfc7296_proposal_test.go` | AC-5 under both `keyLengthExact` and `keyLengthAtLeast` | |
| `TestRFC7634ProposalOffersNoKeyLength` | `internal/component/ike/engine/rfc7634_proposal_test.go` | AC-4: `encAttrs` of the registry's `chacha20poly1305` is empty for the IKE and the ESP offer | |
| `TestEncKeyMaterialLenSaltsEveryAEAD` (existing, gains a row by enumeration) | `internal/component/ike/crypto/aead_predicate_test.go` | ENCR 28 answers 36 | |
| `TestRFC5282AEADReceiveAcceptsAnyPaddingTo255` (extend) | `internal/component/ike/engine/rfc5282_aead_sk_test.go` | ENCR 28 row: RFC 7634 Section 3 "The receiver MUST accept any length of padding." | |
| `TestParseAcceptsNewTransforms` | `internal/component/ike/ipsec/algorithm_support_test.go` | AC-1, AC-2, AC-3 | |
| `TestParseRejectsUnimplementedEncryption` (change) | `internal/component/ike/ipsec/algorithm_support_test.go` | iterates `3des` only; AC-14 | |
| `TestTransformRegistryUnknown` (change) | `internal/component/ike/crypto/transform_test.go` | the group-21 row becomes group 17 | |
| `TestESPKeyBitsReportsFixedKey` | `internal/component/ike/engine/reconcile_test.go` | AC-12: an ENCR 28 Child SA reports 256 | |
| `TestXfrmChaCha20KeyIs36Octets` | `internal/component/ike/dataplane/xfrm_linux_test.go` | AC-8: state built from a ChaCha20 `SAParams` carries the kernel name, ICV 128 and 288 key bits | |
| `BenchmarkDHExchange` | `internal/component/ike/crypto/dh_test.go` | per-group keygen plus shared-secret cost (R-4 evidence) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `dh-group` (YANG) | 1..31; implemented 14, 15, 16, 18, 19, 20, 21 | 31 (YANG), 21 (implemented) | 0 | 32 |
| MODP public value length (group 15/16/18) | exactly 384 / 512 / 1024 | the exact length | length minus 1 | length plus 1 |
| MODP public value (integer) | 2..p-2 | 2 and p-2 | 1 | p-1 |
| ECP 521 public value length | exactly 132 | 132 | 131 | 133 |
| ENCR 28 Key Length attribute | absent only | absent | N/A | any present value |
| ChaCha20 KEYMAT per direction | exactly 36 | 36 | 35 | 37 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-sa-chacha20poly1305` | `test/ipsec/ipsec-sa-chacha20poly1305.ci` | two ze daemons, IKE and ESP ChaCha20-Poly1305, dh-group 21; `show vpn ipsec sa` names the transform and the group | |
| `ipsec-sa-modp8192` | `test/ipsec/ipsec-sa-modp8192.ci` | two ze daemons, dh-group 18; SA up; `show vpn ipsec sa` names `modp8192` | |
| `ipsec-psk-encoding-enum` (comment only) | `test/parse/ipsec-psk-encoding-enum.ci` | its comment cites `chacha20poly1305` as unimplemented; reword to `3des` (stale comment) | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `chacha20poly1305-ike-esp` | `test/interop-ipsec/scenarios/chacha20poly1305-ike-esp/` | strongSwan (charon) | charon logs `selected proposal: IKE:CHACHA20_POLY1305/` and `selected proposal: ESP:CHACHA20_POLY1305/`; Ze's IKE `encryption` and Child `esp-encryption` agree; XFRM `rfc7539esp` state on both; ESP both directions. DH group 19, so the cipher is isolated | |
| `ike-modp3072` | `test/interop-ipsec/scenarios/ike-modp3072/` | strongSwan | charon logs `MODP_3072`; Ze `dh-group` names `modp3072`; SA up; traffic | |
| `ike-modp4096` | `test/interop-ipsec/scenarios/ike-modp4096/` | strongSwan | same for `MODP_4096` | |
| `ike-modp8192` | `test/interop-ipsec/scenarios/ike-modp8192/` | strongSwan | same for `MODP_8192` | |
| `ike-ecp521` | `test/interop-ipsec/scenarios/ike-ecp521/` | strongSwan | same for `ECP_521` | |

Each scenario owes a recorded red: remove its registry row, rebuild `test/interop-ipsec/ze-linux`, confirm the scenario fails (charon NO_PROPOSAL_CHOSEN), restore, confirm green. The four DH groups each get their own scenario and their own recorded red (D-1): a red for one group proves nothing about another group's transcribed prime.

## Files to Modify
- `internal/component/ike/crypto/transform.go` - `ENCR_CHACHA20_POLY1305` (28); `DH_MODP_3072`, `DH_MODP_4096`, `DH_MODP_8192`, `DH_ECP_521` constants; `encryptionRegistry` gains `chacha20poly1305`; `EncryptionID.String` gains ENCR 28; `DHGroupID.String`, `dhGroupRegistry`, `SupportedDHGroupIDs`, `LookupDHGroup` derive from the DH table
- `internal/component/ike/crypto/dh.go` - replace `modp2048Prime`, `modp2048Generator`, `modp2048Len`, `ecpCoordBits` and both switches with the DH table (MODP rows: prime, generator 2, octet length derived from the prime; ECP rows: curve, component bits); `NewDHExchange` / `SharedSecret` dispatch on the row kind
- `internal/component/ike/crypto/aead.go` - `aeadTransform` gains the fixed key octets (0 for AES) and its mode constructor takes the key rather than an AES block; `newIKEAEAD` stops calling `aes.NewCipher` itself; new ChaCha20-Poly1305 entry (salt 4, ICV 16, key 32)
- `internal/component/ike/crypto/keys.go` - `encKeyMaterialLen` takes the key size from the transform identity when it is fixed
- `internal/component/ike/crypto/proposal.go` - `specifiedEncryption` gains ENCR 28; `specifiedDHGroup` derives from the table; `acceptEncryption` refuses a Key Length attribute on a fixed-key transform (RFC 7296 Section 3.3.5)
- `internal/component/ike/engine/reconcile.go` - `ESPKeyBits` reports effective key bits
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - enum help for `chacha20poly1305`; both `encryption` leaf descriptions name `3des` alone as refused; the `dh-group` description is unchanged (its refusal list is derived)
- `internal/component/ike/ipsec/algorithm_support.go` - the `DHGroupImplemented` comment says "the registry holds three groups"; reword
- `internal/le/interoplab/ipsec/checkers.go` - checker rows and functions for the new scenarios (one helper parameterised by the charon proposal fragment and Ze's names)
- `internal/le/site/testdata/published-configuration.md`, `internal/le/site/testdata/published-yang-config-tree.json` - YANG help and description mirrors
- `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require`, `internal/appliance/kernelreq.go` - CONFIG_CRYPTO_CHACHA20POLY1305
- `test/parse/ipsec-psk-encoding-enum.ci` - stale comment
- `docs/architecture/ike/ipsec-6-ikev2-crypto.md` - standard-library decision amended for x/crypto; registry counts; the DH table; the MODP private-key paragraph covers every MODP group
- `docs/guide/ipsec.md` - Proposals row lists the new transforms
- `docs/architecture/ike/ipsec-10-cli-diag.md` - `ESPKeyBits` is the effective key size
- `docs/guide/configuration.md` - IPsec encryption list gains `chacha20poly1305`; only `3des` stays unimplemented; the DH line names the implemented groups 14, 15, 16, 18, 19, 20, 21
- `docs/features.md` - "IKEv2 Cryptographic Primitives" row made true (Known Limitations covers the interim)
- `docs/architecture/testing/interop.md` - new scenarios and the `swanctl --list-algs` measurement (A-1)
- `test/interop-ipsec/Dockerfile.strongswan` - comment recording the algorithm providers (and the package line only if A-1 needs a plugin)

## Files to Create
- `internal/component/ike/crypto/rfc3526_modp_test.go`
- `internal/component/ike/crypto/rfc7634_chacha_test.go`
- `internal/component/ike/engine/rfc7634_proposal_test.go`
- `test/ipsec/ipsec-sa-chacha20poly1305.ci`, `test/ipsec/ipsec-sa-modp8192.ci`
- `swanctl.conf` and `ze.conf` under each new scenario directory in `test/interop-ipsec/scenarios/`
- `rfc/short/rfc7634.md`, `rfc/short/rfc3526.md`, `rfc/short/rfc5903.md` (via `/ze-rfc`)
- `rfc/discrimination/<stem>.json` records for every newly tagged test (via `./le rfc discriminate-record`)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new node: the enum and the `dh-group` range already exist. Only help and description text changes in `ze-ipsec-conf.yang` |
| YANG validation constraints | N-A | `dh-group` keeps `range "1..31"`; implementability stays a parse-time registry check by design (`ipsec-3-data-model.md`) |
| YANG custom validators | N-A | None added |
| CLI commands/flags | N-A | No command added |
| CLI grammar (keyword before value) | N-A | No grammar change |
| Editor autocomplete | N-A | Enum completion unchanged |
| Functional test for new RPC/API | Yes | `test/ipsec/ipsec-sa-chacha20poly1305.ci`, `test/ipsec/ipsec-sa-modp8192.ci` |
| Pipe completeness | N-A | `show vpn ipsec sa` output shape unchanged |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | No | The only new dependency is a kernel crypto option on the appliance, pinned at build time by `runtimeKernelRequirements` (`internal/appliance/kernelreq.go`), the existing mechanism for `CONFIG_INET_ESP`. No per-cipher `/proc/crypto` doctor probe: an absent cipher fails the XFRM install with an error rather than silently |
| Prometheus counters/metrics | N-A | No new observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`, "IKEv2 Cryptographic Primitives" row |
| 2 | Config syntax changed? | No | Syntax unchanged; accepted values widen |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md` Proposals row |
| 7 | Wire format changed? | Yes | `docs/architecture/ike/ipsec-6-ikev2-crypto.md` (new transforms, KE lengths, fixed-key rule) |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc7634.md`, `rfc/short/rfc3526.md`, `rfc/short/rfc5903.md` (new) and their `docs/features/rfc-status.md` rows (generated from `## Meta`) |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (new scenarios, A-1 measurement) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it lists IKE algorithms (check at implementation) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-6-ikev2-crypto.md` (DH table, x/crypto decision) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | From `./le spec citation anchors spec plan/spec-ike-missing-transforms.md` (run 2026-09-28). Declared by `// Design:`: `docs/architecture/ike/ipsec-6-ikev2-crypto.md` (transform.go, aead.go, keys.go, dh.go) UPDATED; `docs/architecture/ike/ipsec-3-data-model.md` (algorithm_support.go) UNAFFECTED, because it names neither `chacha20poly1305`, `3des` nor the refused set (grep 2026-09-28); `docs/architecture/ike/ipsec-7-ikev2-engine.md` (reconcile.go) UNAFFECTED, because its Encrypted payload sizing table already states "8 under an AEAD suite" for the IV and "inside the ciphertext under AEAD" for the ICV, and ChaCha20-Poly1305 has both. Mentioned only: `docs/architecture/ike/ipsec-10-cli-diag.md` UPDATED (its `PeerInfo` paragraph says `ESPKeyBits` is a transform id beside the names; it becomes the effective key size, 256 for ChaCha20 which sends no Key Length); `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, `docs/architecture/ike/ipsec-14-responder.md`, `docs/architecture/ike/ipsec-dataplane-inspection.md`, `docs/architecture/ike/ipsec-11-interop-eap.md` UNAFFECTED (they describe KEYMAT roles, responder selection, dataplane read-back and EAP scenarios, none of which names a transform set or a key size); `docs/guide/configuration.md` UPDATED (its IPsec section lists the encryption algorithms, says the schema names `chacha20poly1305` and `3des` with no transform, and gives "DH groups: 1-31", which names the YANG range rather than the implemented set) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ipsec.md` examples use `dh-group 14`; verify they still parse |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - RFC summaries and failing tests
   - `/ze-rfc` for RFC 7634, 3526, 5903 (texts in `rfc/full/`)
   - Tests: `TestParseAcceptsNewTransforms`, `TestDHGroupTableDerivesEveryList`, both `.ci` files
   - Verify: all red because the registry lacks the entries
2. **Phase: DH table** - replace the seven DH sites with one table; groups 14, 19, 20 first with no behaviour change (AC-13 green), then rows 15, 16, 18, 21
   - Tests: `TestRFC3526MODPPrimesAreSafePrimes`, the extended RFC 7296 / RFC 5903 tests, `TestDHGroupSharedSecretAgrees`, `BenchmarkDHExchange`
   - Primes are transcribed from `rfc/full/rfc3526.txt` Sections 4, 5 and 7, with the RFC quote above each row
3. **Phase: fixed-key AEAD** - the `aeadTransform` shape change with the AES constructors taking the key (AC-13 green), then the ChaCha20-Poly1305 row, `encKeyMaterialLen`, the `acceptEncryption` refusal, `specifiedEncryption`, `String`, the registry name
   - Tests: the `TestRFC7634` group, `TestRFC7296FixedKeyTransformRefusesKeyLength`, the extended padding test
4. **Phase: engine and reporting** - `ESPKeyBits`; `show` names
   - Tests: `TestESPKeyBitsReportsFixedKey`, `TestXfrmChaCha20KeyIs36Octets`, both `.ci` green
5. **Phase: YANG text, kernel config, docs** - every page in the checklist, in this phase
6. **Phase: interop** - A-1 and A-2 measured first; the five scenarios (`chacha20poly1305-ike-esp`, `ike-modp3072`, `ike-modp4096`, `ike-modp8192`, `ike-ecp521`) and their checkers; each with its own recorded red (D-1)
7. **Phase: RFC discrimination** - `./le rfc discriminate-record` for each tagged test

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Both user stories reach strongSwan and pass traffic |
| Correctness | ChaCha20 KEYMAT is 36 for IKE and ESP whether the transform came from config or from the wire; no Key Length attribute on ENCR 28 in either direction |
| Correctness | Every MODP prime equals the RFC 3526 text, proven by the safe-prime test AND a strongSwan scenario |
| Naming | Config word `chacha20poly1305`; `show` names from the tables; charon names in checkers match `swanctl --list-algs` |
| Data flow | No switch over a DH group ID remains in `crypto/`; no AES block is built outside the AES mode constructors |
| Rule: no-layering | The old DH switches and the `modp2048` variables are deleted, not kept beside the table |
| Rule: rfc-compliance | New functions quote RFC 7634 / 3526 / 5903 / 7296 text; callers name the section |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Registry holds the five transforms | `TestDHGroupTableDerivesEveryList`, `TestParseAcceptsNewTransforms` |
| No DH switch left | grep for `case DH_` in `internal/component/ike/crypto/` answers nothing |
| Interop green with recorded reds | `./le test integration` output for the new scenarios, plus the red runs |
| Docs true | the `docs/features.md` row and the `docs/guide/ipsec.md` Proposals row match the registry |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Peer KE value length and range for every MODP group; ECP point on curve (ecdh refuses) and exact length; Key Length attribute on ENCR 28 refused |
| Resource exhaustion | MODP 8192 CPU per IKE_SA_INIT (R-4); the cookie threshold still gates half-open SAs |
| Key handling | New exchange rows cleared by `DHExchange.Clear`; ChaCha20 key material cleared through `SKKeys.Clear` / `ChildSAKeys.Clear` like AES |
| Fail closed | AEAD open failure answers `ErrDecryptionFailed` only; an unknown group or cipher still answers an error, never a zero transform |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The Key Length attribute and the key size are different facts. AES carries its size on the wire; ChaCha20-Poly1305 carries it in its identity. Treating the attribute as the size is correct only while every cipher is AES.
- strongSwan's default MODP exponent is full length, the same as Ze's 2..p-2 range today (A-6).

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D-A: key size from the transform identity for fixed-key ciphers (a field in the AEAD table); the wire `KeyLength` stays the attribute | Put 256 in the registry's `KeyLength` | 256 in `KeyLength` would be emitted by `encAttrs`, violating RFC 7296 Section 3.3.5, and a wire-read transform would still carry 0 |
| D-B: one DH group table replaces seven lists | Add a case to each switch and map | Four groups times seven sites is the central-enumeration defect `ai/rules/principles.md` forbids; the table also serves `spec-ike-post-quantum` |
| D-C: the encryption lists each gain one entry rather than being collapsed | Collapse `String`, `specifiedEncryption` and `encryptionRegistry` into one table | Those lists are already bound by `aead_predicate_test.go`, `TestAcceptedAlgorithmsResolveToTransforms` and `TestVocabularyMatchesModel`; collapsing them is a refactor this feature does not need |
| D-D: `golang.org/x/crypto/chacha20poly1305` | Write ChaCha20-Poly1305 in `internal/core`, as was done for CCM | Already vendored and used by the appliance; the standard library has none; a hand-written AEAD adds risk and nothing else |
| D-1 (RESOLVED, Thomas 2026-09-28): one strongSwan interop scenario per new DH group: `ike-modp3072`, `ike-modp4096`, `ike-modp8192`, `ike-ecp521`, beside `chacha20poly1305-ike-esp`; each owes a recorded red | - | Only a foreign peer proves a transcribed prime matches everyone else's; each scenario is one strongSwan run |
| D-2 (RESOLVED, Thomas 2026-09-28): MODP private exponents stay full length (2..p-2) for groups 15, 16 and 18 | - | Ze's documented rule today and strongSwan's default (A-6). Measured cost about 149 ms per exponentiation for 8192 (R-4) |
| D-3 (RESOLVED, Thomas 2026-09-28): 3DES stays out of the build; the `3des` enum stays named in the schema and refused at commit as today | - | No reason found to implement it (RFC 8221 SHOULD NOT for ESP). Removing the enum is not part of this spec |

## Known Limitations

- MODP 6144 (group 17) is not added: not requested. It stays refused at parse, and `TestTransformRegistryUnknown` uses it as the absent-group row.
- 3DES is not implemented (see Task and D-3).
- The "IKEv2 Cryptographic Primitives" row of `docs/features.md` claims MODP 3072/4096/8192, ECP 521 and ChaCha20-Poly1305. Until this spec lands that claim is false, and `ai/rules/rfc-compliance.md` requires it corrected now on that surface, by the doc sync in progress rather than by this spec.
- VPP ESP ChaCha20 is covered by the existing `vpp_test.go` mapping tests only; no VPP interop scenario exists for any cipher.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT. For this spec: RFC 7634
Sections 2, 3 and 4; RFC 3526 Sections 4, 5 and 7; RFC 5903 Sections 3.3 and 7;
RFC 7296 Sections 3.3.5 and 3.4.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
