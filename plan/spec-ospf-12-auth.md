# Spec: ospf-12-auth

| Field | Value |
|-------|-------|
| Status | done |
| Depends | spec-ospf-2-wire.md, spec-ospf-3-ip-transport.md, spec-ospf-4-component-config.md, spec-ospf-5-interface-ism.md, spec-ospf-6-neighbor-nsm.md, spec-ospf-7-lsdb-flooding.md |
| Phase | 12/12 |
| Updated | 2026-06-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-ospf-0-umbrella.md` (row ospf-12) - umbrella scope, dependency graph; read "Shared Contracts": "Two distinct checksums", "Authentication config model", "Packet receive dispatcher", "Metrics (canonical)", and the `packet/` Architecture row
4. `plan/spec-ospf-2-wire.md` - the common-header codec that exposes the 8-byte Authentication field and writes a zero-Checksum packet for AuType 2 (this spec is the crypto backend on top of it)
5. `plan/spec-ospf-5-interface-ism.md`, `plan/spec-ospf-6-neighbor-nsm.md` - the Hello (ISM) and DD/LS Request (NSM) packet paths this spec signs on send and verifies on receive
6. `docs/research/ospf-implementation-guide.md` sec 8 (Authentication, lines ~518-545) and sec 13 trap #10 (Authentication with Zeroed Checksum, lines ~1484-1487)
7. `internal/component/config/secret/secret.go` - Ze `$9$` reversible sensitive-value encoding
8. `internal/plugins/isis/auth_keystore.go`, `internal/plugins/isis/packet/auth_sign.go`, `auth_verify.go` - the IS-IS sibling pattern (key store + per-PDU verify/sign) OSPF mirrors

## Task

Add OSPFv2 authentication to Ze: a key store plus verify-on-receive and
sign-on-send for all five OSPF packet types (Hello, Database Description, LS
Request, LS Update, LS Ack). The 24-byte common-header codec, including the
structural framing of the 64-bit Authentication field for AuType 0/1/2 and the
ability to emit a packet with a zeroed Checksum field, already exists from
spec-ospf-2. This spec is the cryptographic backend, the key management, and the
per-packet verify/sign wiring built on top of that codec.

Four on-the-wire authentication schemes are in scope, selected by the AuType
field in the OSPFv2 common header (RFC 2328 §D.3, RFC 7474 §3):

- **AuType 0 (Null).** No authentication. The 8-byte Authentication field is
  zero. This is the default and existing behaviour; no key configured means no
  verify and no sign.
- **AuType 1 (Simple password).** The 8-byte Authentication field carries an
  8-byte cleartext password compared on receive. Provides no real security
  (it travels in clear and is trivially observed); useful only to stop accidental
  misconfiguration. The packet Checksum field is computed normally for AuType 1.
- **AuType 2 (Cryptographic, RFC 2328 Appendix D).** The 8-byte Authentication
  field becomes a structure: Reserved(2)=0, Key ID(1), Auth Data Length(1) =
  digest length, Cryptographic Sequence Number(4). The MD5 (16-byte) digest is
  **appended after** the OSPF packet body, NOT carried in the 8-byte
  Authentication field. The packet Checksum field is set to zero during
  cryptographic authentication (trap #10) and never backfilled. Anti-replay is
  provided by the non-decreasing 32-bit Cryptographic Sequence Number tracked per
  neighbour per receiving interface.

  Within AuType 2, RFC 5709 adds the HMAC-SHA algorithms (SHA-1, SHA-256,
  SHA-384, SHA-512) alongside Keyed-MD5: the 8-byte field is unchanged, the digest
  length (16/20/32/48/64 bytes) is carried in Auth Data Length, RFC 5709 specifies
  the Apad pad constant and key handling (the trailer fills the digest region with
  Apad during computation), and the 32-bit Cryptographic Sequence Number still
  rides in the 8-byte field. MD5 and the HMAC-SHA algorithms thus all share wire
  AuType value 2, differing only in algorithm and Auth Data Length.
- **AuType 3 (Cryptographic with Extended Sequence Numbers, RFC 7474 §3).** A
  DISTINCT AuType, not a variant of 2. The 8-byte field is restructured:
  Reserved(3 octets, was 2) = 0, Key ID(4 octets, was 1, in the former
  sequence-number position), Auth Data Length(1). A 64-bit Cryptographic Sequence
  Number (high-order boot count held in non-volatile storage + low-order
  monotonically-increasing counter) is appended BEFORE the digest; the digest
  (HMAC-SHA per RFC 5709) follows. The wider sequence number gives replay
  protection across reboots. As with AuType 2 the packet Checksum field is zero
  and Packet Length excludes the appended sequence number and digest.

Wherever the AuType-2 crypto handling below is described (zeroed checksum,
appended digest, per-neighbour sequence tracking, constant-time compare, key
chains), AuType 3 behaves identically EXCEPT for the restructured 8-byte field
(32-bit Key ID, no in-field sequence number) and the 64-bit appended sequence
number. See `rfc/short/rfc2328.md` (§D), `rfc/short/rfc5709.md`, and
`rfc/short/rfc7474.md`.

Keys are organised as key chains, not bare strings, matching the canonical
"Authentication config model" in spec-ospf-0-umbrella. Each key in a chain has a
key-id, an algorithm (enum `simple`/`md5`/`hmac-sha-1`/`hmac-sha-256`/
`hmac-sha-384`/`hmac-sha-512`), a secret (`$9$`-encoded), and optional send and
accept lifetimes for hitless rotation. Chains are configured per interface with an
area-level default selected by `inherit` (a per-interface `authentication`
setting may name a chain directly or `inherit` the chain bound to its area). A
chain holds multiple keys so an operator can rotate keys without dropping
adjacencies (an active key selected for signing plus standby keys accepted on
receive during overlap windows).

Keys are configured through YANG. The YANG key-chain leaves are OWNED by
spec-ospf-4 (config): ospf-4 defines the schema (chain, key-id, algorithm enum,
`$9$` secret, lifetimes, per-interface `authentication` with `inherit`) and this
spec owns the verify and sign semantics and the runtime key lookup built on top of
those leaves. The AuType codes and algorithm names used here MUST match the
ospf-4 schema enum and the canonical umbrella model. Sensitive key material uses
the Ze `$9$` reversible encoding (package `internal/component/config/secret`), the
same mechanism PPPoE passwords, WireGuard keys, and IS-IS keys already use, so
keys never appear as plaintext in `show configuration` or config backups.

Authentication is enforced on both transmit and receive. On send, the engine signs
every packet on an authenticated interface with the active key. On receive, the
instance dispatcher verifies before routing the packet to ISM/NSM/LSDB handlers;
verification first checks the AuType matches the configured scheme and then, per
AuType, compares the simple password (constant-time) or recomputes the digest over
the packet and compares it constant-time against the appended digest. A packet that
fails authentication is dropped before any protocol processing: it does not form or
sustain an adjacency, is not flooded, and is not installed in the LSDB. Each
rejection increments `ze_ospf_auth_failures_total{interface,reason}`, which this
spec OWNS and registers (per the umbrella "Metrics (canonical)" table); ospf-13
only scrapes and surfaces it.

Two framing constraints are load-bearing and tie back to the umbrella "Two
distinct checksums" contract. First, for AuType 2 (MD5) and the RFC 7474 HMAC-SHA
trailer, the OSPF common-header Checksum field MUST be zero before the digest is
computed and MUST be zero in the transmitted packet (trap #10): if the IP-style
checksum were computed and then the digest, the receiver would mismatch; the
encoder leaves Checksum zero and never backfills it for AuType 2. Second, the
OSPF Packet Length in the common header covers only the header plus body, NOT the
appended digest trailer; the receiver reads the full datagram and takes the
trailing Auth-Data-Length bytes after the body as the digest. All digest and
password comparisons use a constant-time compare to avoid timing side channels.

Ze has no OSPF authentication today beyond the structural AuType field codec from
ospf-2; there is no key store, no verify path, no sign path, no sequence-number
anti-replay state, and no failure accounting.

## Required Reading

### Architecture Docs
- [ ] `docs/research/ospf-implementation-guide.md` sec 8 (Authentication) - AuType values, FRR support, key rollover, implementation notes
  -> Constraint: AuType 0 Null, 1 Simple (8-byte cleartext), 2 Cryptographic (8-byte field = Key ID + Auth Data Length + 32-bit Crypto Seq; MD5 or RFC 5709 HMAC-SHA digest appended); 3 (RFC 7474) Cryptographic with Extended Sequence Numbers -- restructured field (32-bit Key ID), 64-bit sequence number appended before the digest
  -> Constraint: anti-replay via the monotonic Cryptographic Sequence Number; the receiver tracks the last-seen sequence per (neighbour, key-id) and rejects any smaller value
  -> Constraint: the common-header checksum is zeroed for AuType 2; the digest is computed over the packet including the zeroed checksum; key rollover uses overlapping-validity key chains (sender uses primary, receiver accepts all valid keys); do not roll your own HMAC -- use `crypto/hmac`, `crypto/md5`, `crypto/sha1`, `crypto/sha256`, ...
- [ ] `docs/research/ospf-implementation-guide.md` sec 13 trap #10 (Authentication with Zeroed Checksum) - clear the Checksum field before the digest and never write a checksum after
  -> Constraint: for AuType 2 and RFC 7474, zero the common-header Checksum before computing the digest and leave it zero on the wire; computing the IP-style checksum then the HMAC causes a receiver mismatch
- [ ] `plan/spec-ospf-2-wire.md` - the common-header codec (8-byte Authentication field framing, zero-Checksum-for-AuType-2 encode)
  -> Constraint: reuse the ospf-2 common-header codec; do not add a second header encoder; AuType 2 decomposes the 8-byte field into Key ID + Auth Data Length + Cryptographic Sequence Number (ospf-2 AC-4)
- [ ] `ai/rules/config-surface.md`, `ai/rules/config-naming.md` - YANG vs env var, kebab-case
  -> Constraint: keys are operator-facing YANG config (visible in commit/rollback), not env vars; kebab-case leaves; per-interface `authentication` with area `inherit`
- [ ] `internal/component/config/secret/secret.go` - `$9$` reversible encoding (Encode/Decode/IsEncoded)
  -> Constraint: store key material `$9$`-encoded like PPPoE/WireGuard/IS-IS; decode only in memory at sign/verify time
- [ ] `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md` - zero-copy, no-alloc hot path
  -> Constraint: verify reads the received raw bytes in place; sign writes the digest into the encode buffer after the packet body; no per-packet plaintext-key allocation churn
- [ ] `internal/plugins/isis/auth_keystore.go`, `internal/plugins/isis/packet/auth_sign.go`, `auth_verify.go` - the IS-IS sibling (key store + per-PDU verify/sign over raw bytes)
  -> Constraint: copy the structure (runtime-free codec helpers in `packet`, key store in the component, constant-time compare, `$9$` decode in memory); do NOT couple OSPF to IS-IS code -- different framing (header AuType vs TLV 10, appended digest vs in-TLV digest)

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2328.md` §D (Authentication) + Appendix D (Cryptographic authentication) - AuType 0/1/2, the 8-byte field structure, the appended MD5 digest, the zeroed checksum, the sequence number
  -> Constraint: AuType 1 password is 8 bytes cleartext in the Authentication field; AuType 2 field = Reserved(2)=0, Key ID(1), Auth Data Length(1), Cryptographic Sequence Number(4); the 16-byte MD5 digest is appended after the body; the Checksum field is zero; Packet Length excludes the digest
  -> Constraint: anti-replay: the Cryptographic Sequence Number is non-decreasing per neighbour; reject a packet whose sequence is less than the last accepted for that neighbour/key
- [ ] `rfc/short/rfc5709.md` - OSPFv2 HMAC-SHA Cryptographic Authentication
  -> Constraint: SHA-1/256/384/512; Auth Data Length is the algorithm output length (20/32/48/64); the Apad constant (0x878FE1F3 repeated) fills the digest region during the computation; First-Hash key derivation when the key is longer than the block size
- [ ] `rfc/short/rfc7474.md` - Security Extensions for OSPFv2 (auth trailer)
  -> Constraint: RFC 7474 defines AuType 3 (Cryptographic with Extended Sequence Numbers) -- a restructured 8-byte field (24-bit reserved + 32-bit Key ID, no in-field sequence number) with a 64-bit Cryptographic Sequence Number appended before the digest; HMAC-SHA algorithms come from RFC 5709; replay/rollback protection via the 64-bit sequence number (boot count + counter) and key-id; the packet Checksum stays zero

**Key insights:**
- The ospf-2 common-header codec already frames the 8-byte Authentication field for AuType 0/1/2 and can emit a zero-Checksum packet; this spec is the crypto backend (`crypto/hmac`, `crypto/md5`, `crypto/sha*`) plus a key store and exactly two hook points: verify on the ospf-4 receive dispatcher, sign on the ospf-3 transmit path
- AuType 1 keeps the normal IP checksum; AuType 2 (MD5) and the RFC 7474 HMAC-SHA trailer zero the Checksum and append the digest after the body (Packet Length excludes the digest); trap #10
- The 8-byte Authentication field is OUTSIDE the packet-checksum coverage by design (ospf-2), so for AuType 2 the digest-bearing fields do not perturb the (zero) checksum
- Anti-replay is the per-neighbour Cryptographic Sequence Number; constant-time compare on every digest/password verify; key chains give hitless rotation (active key signs, all valid keys accepted on receive)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ospf/packet/` (common-header codec from ospf-2) - encodes/decodes the 24-byte common header including AuType and the 8-byte Authentication field, and can emit a packet with a zeroed Checksum for AuType 2, but never computes or checks a digest or password
  -> Constraint: reuse the ospf-2 common-header codec and the zero-Checksum encode path; do not add a second header encoder
- [ ] `internal/component/config/secret/secret.go` - `$9$` Encode/Decode/IsEncoded for sensitive leaves
  -> Constraint: key store decodes `$9$` to plaintext only in memory when deriving HMAC/MD5 keys
- [ ] `plan/spec-ospf-5-interface-ism.md` (Hello path), `plan/spec-ospf-6-neighbor-nsm.md` (DD/LS Request path), the ospf-4 instance dispatcher and the ospf-3 transmit path - where packets are built (TX) and consumed (RX) today with no auth hook
  -> Constraint: add a single verify step on the ospf-4 receive dispatcher (before it routes by Type to ISM/NSM/LSDB) and a single sign step on the ospf-3 transmit path (the digest is in place before the frame is handed to the raw IP transport)
- [ ] `internal/plugins/isis/auth_keystore.go` + `internal/plugins/isis/packet/auth_sign.go`/`auth_verify.go` - the IS-IS key store + sign/verify backend this spec mirrors
  -> Constraint: same shape; OSPF differs in framing (header AuType, appended digest, sequence number) and has no purge concept

**Behavior to preserve:**
- The ospf-2 common-header byte layout is unchanged (this spec consumes it, does not alter the wire struct)
- The ospf-2 packet checksum (RFC 1071, excluding the 8-byte auth field) is unchanged; this spec adds only the AuType-2 rule that the Checksum stays zero and a digest is appended
- Unauthenticated operation (AuType 0, no key configured) remains the default and continues to work unchanged
- Packet build/parse paths from ospf-2/ospf-5/ospf-6 keep their shape; auth is an added hook, not a rewrite

**Behavior to change:**
- New `ospf` YANG auth leaves (coordinated with ospf-4): per-interface key chains with area `inherit`, key-id, algorithm enum, `$9$` secret, send/accept lifetimes
- RX dispatch (ospf-4) gains a verify step that can reject a packet before ISM/NSM/LSDB processing
- TX (ospf-3) gains a sign step that, for AuType 2, leaves the Checksum zero and appends the digest after the body; for AuType 1 writes the 8-byte password and keeps the normal checksum
- Per-neighbour Cryptographic Sequence Number state (send counter + last-accepted receive value) is maintained for anti-replay
- `ze_ospf_auth_failures_total{interface,reason}` is incremented on every rejected packet; this spec OWNS and registers the series (ospf-13 only scrapes/surfaces it)

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Config: the `ospf` auth subtree (per-interface key chains with area `inherit`, AuType-derived algorithm, key-id, `$9$`-encoded secret, send/accept lifetimes), arriving via the YANG-validated config tree (leaves owned by ospf-4)
- RX: a decoded OSPF packet on the ospf-4 receive dispatcher, still holding its raw bytes (and any appended digest trailer), tagged with the receiving interface and source address
- TX: an OSPF packet about to be handed to the ospf-3 transmit path

### Transformation Path
1. **Key config -> store:** config resolve builds per-interface key chains (resolving area `inherit`); `$9$` secrets are decoded to derive MD5/HMAC keys held only in memory; one active key is selected per interface for signing (send lifetime) and all currently valid keys are retained for verify (accept lifetime). The interface auth scheme (AuType) is derived from the active key's algorithm
2. **Packet send -> sign:** select the active key for the egress interface. For AuType 1, write the 8-byte cleartext password into the Authentication field and let ospf-2 compute the normal checksum. For AuType 2, write Reserved=0, Key ID, Auth Data Length = digest length, and the next 32-bit Cryptographic Sequence Number into the 8-byte field; leave the Checksum field zero; compute the digest over the common header + body (with the digest region filled with Apad for HMAC-SHA per RFC 5709, or the key appended for MD5 per RFC 2328 Appendix D); append the digest after the body; Packet Length stays header+body (excludes the digest). For AuType 3, write Reserved(24)=0, 32-bit Key ID, and Auth Data Length in the 8-byte field; append the 64-bit Cryptographic Sequence Number before the HMAC-SHA digest; leave the Checksum field zero and exclude the appended sequence number and digest from Packet Length.
3. **Packet receive -> verify:** on the ospf-4 dispatcher, before routing by Type, read the AuType. AuType 0 with auth configured is rejected. AuType 1 compares the 8-byte password constant-time. AuType 2 reads Key ID, Auth Data Length, and 32-bit Cryptographic Sequence Number; AuType 3 reads the 32-bit Key ID and Auth Data Length from the field and the 64-bit sequence number from the trailer. The verifier selects the matching key from the interface chain, checks the sequence number is not less than the last accepted for that (neighbour, key-id), recomputes the digest with the same MD5/RFC 5709 construction, and constant-time-compares against the appended Auth-Data-Length bytes.
4. **verify -> accept/reject:** a match accepts the packet (and, for AuType 2 or 3, updates the last-accepted sequence number for that neighbour/key) and lets it proceed to ISM/NSM/LSDB. A mismatch (wrong AuType, missing/short digest or sequence trailer, unknown key-id, replayed sequence, or digest/password mismatch) rejects and drops the packet and increments `ze_ospf_auth_failures_total{interface,reason}` with the corresponding `reason`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree <-> key store | resolved typed per-interface key chains (area `inherit` resolved; `$9$` decoded in memory) | [ ] |
| RX dispatch <-> verify | raw packet bytes + interface + source -> accept/reject | [ ] |
| TX <-> sign | encode buffer + active key -> 8-byte field + (AuType 2) appended digest, Checksum zero | [ ] |
| verify/sign <-> ospf-2 codec | existing common-header AuType-field encode/decode + zero-Checksum encode | [ ] |
| anti-replay <-> neighbour state | per-(neighbour, key-id) last-accepted Cryptographic Sequence Number (ospf-6 neighbour) | [ ] |
| reject <-> metrics | increment `ze_ospf_auth_failures_total{interface,reason}` (owned and registered here; surfaced in ospf-13) | [ ] |

### Integration Points
- `internal/plugins/ospf/packet/auth_verify.go` - verify/sign helpers over packet bytes, using the existing ospf-2 common-header codec; runtime-free
- Key store in the component (resolved from YANG, holds per-interface chains + active key per interface, area `inherit` resolved)
- RX dispatch hook (ospf-4 instance dispatcher) calls verify before routing by Type
- TX sign hook (ospf-3 transmit path) calls sign during/after encode, before the frame reaches the raw IP transport
- Per-neighbour Cryptographic Sequence Number state on the ospf-6 neighbour record (send counter and last-accepted receive value per key-id)
- `internal/component/config/secret` for `$9$` decode of key material
- `ze_ospf_auth_failures_total{interface,reason}` (owned and registered here per the umbrella canonical table; surfaced in ospf-13)

Consistent with the Shared Contracts in spec-ospf-0-umbrella: verify runs on the
ospf-4 packet receive dispatcher path (the dispatcher rejects a failed packet
before it routes by the common-header Type to ospf-5/ospf-6/ospf-7), and sign runs
on the ospf-3 transmit path (the digest is in place before the datagram is handed
to the raw IP socket); the ospf-5/ospf-6 references above are where the Hello and
DD/LS Request packets that this spec signs and verifies are built and consumed.

### Architectural Verification
- [ ] No bypassed layers (RX: bytes -> verify -> dispatch by Type; TX: encode -> sign (append digest / zero checksum) -> transport)
- [ ] No unintended coupling (verify/sign in `packet` depend on the ospf-2 common-header codec only, not on runtime state)
- [ ] No duplicated functionality (reuse the ospf-2 common-header codec and the ospf-2 RFC 1071 checksum; no second auth encoder; do not share IS-IS auth code)
- [ ] Zero-copy preserved (verify reads received bytes in place; sign writes the digest into the encode buffer after the body)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The AuType 2 8-byte field is Reserved(2)=0, Key ID(1), Auth Data Length(1)=digest length, Cryptographic Sequence Number(4); the digest is appended after the body and Packet Length excludes it | RFC 2328 Appendix D; guide sec 8 | Wrong field layout; FRR rejects every packet | `rfc/short/rfc2328.md` summary + ospf-auth-frr interop + ospf-2 AuType-field round-trip | unvalidated |
| A-2 | For AuType 2 (MD5 and HMAC-SHA) the common-header Checksum MUST be zero before the digest and zero on the wire (trap #10) | guide sec 13 #10; RFC 2328 Appendix D | Auth always fails against FRR | trap-#10 unit test (Checksum==0 on AuType-2 output) + ospf-auth-frr | unvalidated |
| A-3 | The MD5 digest construction per RFC 2328 Appendix D appends the key to the packet (with the digest region not yet present), while RFC 5709 HMAC-SHA fills the digest region with Apad during computation | RFC 2328 Appendix D; RFC 5709 | Cross-interop digest mismatch (self-interop can still pass) | per-algorithm sign/verify unit tests + ospf-auth-frr | unvalidated |
| A-4 | `internal/component/config/secret` `$9$` is the right sensitive-leaf mechanism for OSPF keys (as for PPPoE/WireGuard/IS-IS) | `secret.go`, config-surface rule; IS-IS sibling | Need a different secret store | config resolve test (encoded leaf decodes to key) | unvalidated |
| A-5 | A single active signing key per interface plus all currently valid keys accepted on receive is sufficient for hitless rotation | guide sec 8 key rollover; IS-IS A-5 | Need calendar-scheduled per-key selection on receive | rotation functional test (overlap window) | unvalidated |
| A-6 | Per-neighbour non-decreasing Cryptographic Sequence Number tracking is sufficient anti-replay (no persistence required across restart) | RFC 2328 Appendix D; guide sec 8 (FRR derives from time()/counter) | A restart with a reset counter could be rejected by peers; need a persistent or time-seeded counter | seed the send counter from a monotonic source; anti-replay unit test | unvalidated |
| A-7 | Per-interface key chains with area `inherit` (ospf-4 schema) resolve to a concrete chain per interface at config time | umbrella "Authentication config model"; guide sec 10 | `inherit` unresolved; interface signs with no key | config resolve test (interface with `inherit` picks up the area chain) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Non-constant-time compare leaks key material via timing | code review flags `bytes.Equal`/`==` on digests or passwords | mandatory `hmac.Equal` / `subtle.ConstantTimeCompare`; lint/grep check |
| R-2 | AuType 2 packet written with a non-zero Checksum breaks the digest (trap #10) | auth always fails against FRR | the encoder, when AuType==2, leaves Checksum zero and never backfills it; a test asserts Checksum==0 for AuType-2 output |
| R-3 | Packet Length includes the appended digest, so the receiver miscounts the body | round-trip/interop length or digest-boundary failures | Packet Length stays header+body; the receiver takes the trailing Auth-Data-Length bytes after the body as the digest; dedicated boundary test |
| R-4 | Key rotation drops adjacencies (no overlap window) | adjacency flap during key change | accept all currently valid keys on receive; rotation functional test |
| R-5 | Decoded plaintext keys linger or leak into logs/snapshots | key visible in `show`/logs | `$9$` at rest; never log decoded keys; redact in CLI/web |
| R-6 | Unauthenticated packet accepted when auth is configured (downgrade) | AuType 0 packet forms adjacency under configured auth | when a chain is configured, an AuType mismatch (including AuType 0) is a verify failure |
| R-7 | Replay: a captured AuType-2 packet is re-sent to disrupt or spoof | duplicate/old packets accepted | track the per-(neighbour, key-id) last-accepted Cryptographic Sequence Number and reject any smaller value; anti-replay unit test |
| R-8 | Sequence counter resets on restart and peers reject the lower value | adjacencies fail to re-form after a restart with auth | seed the send counter from a monotonic/time source so it never regresses across a restart; document the limitation |
| R-9 | RFC 5709 HMAC-SHA under AuType 2 and RFC 7474 AuType 3 share algorithms but not framing | one-way adjacency with no obvious error | the active key's mode selects AuType 2 versus AuType 3 explicitly; the receiver validates both the AuType and Auth Data Length before key lookup and increments the counter with `reason="autype-mismatch"`, `reason="algo-mismatch"`, or `reason="unknown-keyid"` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ospf` config with a per-interface key chain (or area `inherit`) | -> | key store built; active key + AuType selected per interface | `TestOSPFAuthKeyStore` |
| Packet received with wrong/no key under configured auth | -> | verify fails; adjacency does NOT reach Full; `ze_ospf_auth_failures_total` increments | `TestOSPFAuthWrongKeyRejected` |
| Packet received with the correct key | -> | verify passes; the packet proceeds to ISM/NSM | `TestOSPFAuthSignVerifyHMACSHA` (positive case) |
| AuType 2 packet signed on send | -> | 8-byte field = Key ID + Auth Data Length + Crypto Seq; Checksum zero; digest appended after body | `TestOSPFAuthCryptoFieldLayout`, `TestOSPFAuthDigestAppendedExcludedFromLength` |
| AuType 3 packet signed on send | -> | 8-byte field = 24-bit reserved + 32-bit Key ID + Auth Data Length; 64-bit sequence precedes digest; Checksum zero | `TestOSPFAuthType3FieldLayout`, `TestOSPFAuthType3SequenceTrailer` |
| Replayed AuType 2 packet (sequence <= last accepted) | -> | rejected as replay; not processed; counter increments | `TestOSPFAuthReplay` |
| `test/ospf/ospf-auth.ci` | -> | wrong key rejected, correct key forms adjacency, keys `$9$`-encoded, end to end | `ospf-auth` functional test |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Auth configured, peer sends an AuType 0 (or wrong-AuType) packet | Packet rejected; no adjacency progress; `ze_ospf_auth_failures_total{interface,reason="autype-mismatch"}` increments |
| AC-2 | Auth configured, peer sends a packet with the wrong key | Packet rejected; no adjacency; `ze_ospf_auth_failures_total{interface,reason="digest-mismatch"}` (or `reason="password-mismatch"`) increments |
| AC-3 | Auth configured, peer sends a packet with the correct key | Packet accepted; adjacency progresses; subsequent authenticated packets processed normally |
| AC-4 | Key rotation: operator adds a new active key while the old key is still valid | No adjacency drop; packets signed with either currently valid key are accepted during the overlap window |
| AC-5 | AuType 1 (Simple) configured | The 8-byte Authentication field carries the cleartext password; match accepted (constant-time), mismatch rejected (sanity only, not security); the packet Checksum is computed normally |
| AC-6 | AuType 2 MD5 (RFC 2328 Appendix D) configured | Sign and verify succeed for all 5 packet types; the 8-byte field = Reserved 0 + Key ID + Auth Data Length 16 + Crypto Seq; the 16-byte MD5 digest is appended after the body; the Checksum field is zero; wrong key rejected |
| AC-7 | AuType 3 RFC 7474 HMAC-SHA (SHA-1/256/384/512, RFC 5709) configured | Sign and verify succeed for all 5 packet types; 8-byte field carries 24-bit reserved + 32-bit Key ID + Auth Data Length; 64-bit sequence number is appended before the digest; Auth Data Length = 20/32/48/64; Apad construction per RFC 5709 is used; the Checksum field is zero; wrong key rejected |
| AC-8 | Any AuType 2 or AuType 3 cryptographic packet on the wire | The common-header Checksum is zero (never backfilled); Packet Length covers header+body only; the receiver reads the trailing digest for AuType 2 and the trailing 64-bit sequence plus digest for AuType 3 |
| AC-9 | AuType 2 or AuType 3 packet whose Cryptographic Sequence Number is less than the last accepted for that neighbour/key-id | Packet rejected as a replay; `ze_ospf_auth_failures_total{interface,reason="replay"}` increments; the last-accepted sequence is unchanged |
| AC-10 | AuType 2 or AuType 3 packet whose sequence is greater-or-equal to the last accepted, with a valid digest | Packet accepted; the last-accepted sequence for that neighbour/key-id is updated |
| AC-11 | Per-interface key vs area `inherit` | An interface with an explicit chain uses it; an interface set to `inherit` uses the area-level chain; the resolved chain signs and verifies |
| AC-12 | All 5 packet types (Hello, DD, LS Request, LS Update, LS Ack) under configured auth | Each type is signed on send and verified on receive; no type is left unauthenticated when a chain is configured |
| AC-13 | Key material in config | Stored `$9$`-encoded; never shown as plaintext in `show configuration`, logs, or web |
| AC-14 | Any digest or password comparison on the verify path | Uses constant-time compare (`hmac.Equal` / `subtle.ConstantTimeCompare`); no `bytes.Equal`/`==` on digests or passwords |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures an HMAC-SHA-256 key chain on an interface and the adjacency forms with an authenticated peer | config -> key store -> sign on TX / verify on RX dispatch -> adjacency Full | `TestOSPFAuthSignVerifyHMACSHA`, `test/ospf/ospf-auth.ci` |
| 2 | Configures a wrong key and the adjacency fails to form | config -> key store -> verify fails on RX dispatch -> adjacency stalls, counter increments | `TestOSPFAuthWrongKeyRejected`, `test/ospf/ospf-auth.ci` |
| 3 | Rotates the key with no outage by adding a new active key during an overlap window | config reload -> key store updates chain -> both keys accepted on RX -> no flap | `TestOSPFAuthRotation`, `test/ospf/ospf-auth.ci` |
| 4 | Sets an interface to `inherit` and it picks up the area-level key chain | config resolve -> area chain bound to the interface -> sign/verify with the inherited key | `TestOSPFAuthInherit`, `test/ospf/ospf-auth.ci` |
| 5 | Meshes with an FRR router using OSPF MD5 / HMAC-SHA authentication | full signed protocol over the wire | `test/interop/scenarios/ospf-auth-frr` (authored and run under spec-ospf-13) |
| 6 | Inspects the running config and sees keys obfuscated, not plaintext | config render -> `$9$` encoding | `TestOSPFAuthSecretEncoding`, `test/ospf/ospf-auth.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFAuthSignVerifySimple` | `internal/plugins/ospf/packet/auth_verify_test.go` | AuType 1 8-byte password sign/verify per packet type; normal checksum kept | |
| `TestOSPFAuthSignVerifyMD5` | `internal/plugins/ospf/packet/auth_verify_test.go` | AuType 2 MD5 (RFC 2328 Appendix D) sign/verify per packet type; 16-byte appended digest | |
| `TestOSPFAuthSignVerifyHMACSHA` | `internal/plugins/ospf/packet/auth_verify_test.go` | AuType 2 RFC 5709 HMAC-SHA-1/256/384/512 sign/verify per packet type; Apad construction | |
| `TestOSPFAuthType3FieldLayout` | `internal/plugins/ospf/packet/auth_verify_test.go` | AuType 3 RFC 7474 8-byte field: 24-bit reserved, 32-bit Key ID, Auth Data Length | |
| `TestOSPFAuthType3SequenceTrailer` | `internal/plugins/ospf/packet/auth_verify_test.go` | AuType 3 64-bit sequence number appended before digest; replay state uses the 64-bit value | |
| `TestOSPFAuthCryptoFieldLayout` | `internal/plugins/ospf/packet/auth_verify_test.go` | AuType 2 8-byte field = Reserved 0 + Key ID + Auth Data Length + Crypto Seq round-trips | |
| `TestOSPFAuthZeroedChecksum` | `internal/plugins/ospf/packet/auth_verify_test.go` | trap #10: AuType 2 output has Checksum == 0; digest computed over the zeroed-checksum packet | |
| `TestOSPFAuthDigestAppendedExcludedFromLength` | `internal/plugins/ospf/packet/auth_verify_test.go` | Packet Length covers header+body only; the digest is the trailing Auth-Data-Length bytes | |
| `TestOSPFAuthWrongKeyRejected` | `internal/plugins/ospf/packet/auth_verify_test.go` | mismatched digest/password rejected per packet type | |
| `TestOSPFAuthConstantTimeCompare` | `internal/plugins/ospf/packet/auth_verify_test.go` | verify uses constant-time compare (no `bytes.Equal`/`==` on digest or password) | |
| `TestOSPFAuthReplay` | `internal/plugins/ospf/auth_keystore_test.go` | sequence < last accepted rejected; >= accepted updates the last-accepted value | |
| `TestOSPFAuthKeyStore` | `internal/plugins/ospf/auth_keystore_test.go` | per-interface chains; active key + AuType selection | |
| `TestOSPFAuthInherit` | `internal/plugins/ospf/auth_keystore_test.go` | area `inherit` resolves to the area-level chain at config time | |
| `TestOSPFAuthRotation` | `internal/plugins/ospf/auth_keystore_test.go` | overlap window accepts old and new key | |
| `TestOSPFAuthSecretEncoding` | `internal/plugins/ospf/auth_keystore_test.go` | `$9$`-encoded leaf decodes to the derived key; plaintext never retained | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Simple password length | 1..8 bytes (8-byte Authentication field) | 8 | 0 (empty) | 9 |
| Key ID (AuType 2) | 0..255 (1 byte) | 255 | n/a | 256 |
| Key ID (AuType 3) | 0..4294967295 (32-bit) | 4294967295 | n/a | wraps to 0 |
| Auth Data Length (AuType 2) | 16 (MD5), 20/32/48/64 (HMAC-SHA) | 64 | 15 | 65 |
| Cryptographic Sequence Number | 0..0xFFFFFFFF (32-bit, non-decreasing) | 0xFFFFFFFF | n/a | wraps to 0 (replay edge) |
| Digest length MD5 | 16 bytes | 16 | <16 | >16 |
| Digest length HMAC-SHA-256 | 32 bytes | 32 | <32 | >32 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-auth` | `test/ospf/ospf-auth.ci` | wrong/no key rejected; correct key forms adjacency; area `inherit` resolves; rotation hitless; keys shown `$9$`-encoded | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-auth-frr` | `test/interop/scenarios/` | FRR ospfd | MD5 and HMAC-SHA OSPF authentication interop on the wire (defined and run in spec-ospf-13) | |

### Future (if deferring any tests)
- None planned. The interop scenario `ospf-auth-frr` is authored and run under spec-ospf-13 (the interop harness and FRR scenarios live there); the wire/crypto contract it exercises is fully specified here. Raw-IP / multicast on-wire auth tests are Linux-only and run as QEMU integration tests per `ai/rules/qemu-testing.md`.

## Files to Modify
- `internal/plugins/ospf/packet/` - reuse the existing ospf-2 common-header codec (no wire-struct change); add the verify/sign helpers alongside
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` - auth leaves (coordinated with spec-ospf-4; ospf-4 owns the schema, this spec specifies the auth subtree shape)
- `internal/plugins/ospf/iface/hello.go` - Hello TX/RX uses the shared verify/sign hooks
- `internal/plugins/ospf/neighbor/dd.go`, `internal/plugins/ospf/neighbor/lsreq.go` - DD and LS Request TX/RX use the shared verify/sign hooks
- `internal/plugins/ospf/lsdb/lsupdate.go`, `internal/plugins/ospf/lsdb/lsack.go` - LS Update and LS Ack TX/RX use the shared verify/sign hooks
- `internal/plugins/ospf/instance.go` (ospf-4 dispatcher) - verify chokepoint before Type routing
- `internal/plugins/ospf/config.go` (or `instance.go`) - build the key store from resolved config
- `internal/plugins/ospf/neighbor/` (ospf-6) - per-neighbour Cryptographic Sequence Number state for anti-replay

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | Yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` auth leaves (per-interface key chain, area `inherit`, key-id, algorithm enum, `$9$` secret, send/accept lifetimes) -- coordinated with spec-ospf-4 |
| YANG validation constraints | Yes | algorithm `enumeration` (simple/md5/hmac-sha-1/hmac-sha-256/hmac-sha-384/hmac-sha-512); key-id `range 0..4294967295` with AuType 2 rejecting values above 255 on send; secret `length` and `$9$` pattern; area-level key-chain leaf; `inherit` enum/boolean on the interface `authentication` leaf |
| YANG custom validators | Yes | secret leaf accepts `$9$`-encoded or plaintext (auto-encode on commit) via the shared `ze:sensitive` marker + `CompleteFn`; reuse the PPPoE/WireGuard/IS-IS secret-leaf pattern |
| CLI commands/flags | Yes | auth state surfaced via `show ip ospf interface` / `show ip ospf neighbor` (auth type, key-id, last-failure); `ze_ospf_auth_failures_total` is owned/registered HERE and only scraped/surfaced by ospf-13 |
| CLI grammar (action before identifier) | Yes | `ai/rules/cli-grammar.md` |
| Editor autocomplete | Yes | YANG enum (algorithm) driven; `CompleteFn` for algorithm values and chain names |
| Functional test for new RPC/API | Yes | `test/ospf/ospf-auth.ci` |
| Pipe completeness | Yes | any auth state in show output routes through `ApplyPipes`/`ProcessPipes` |
| Doctor check for runtime dependencies | No | none new (crypto is in-tree stdlib; no socket/file/cert material introduced by auth) |
| Prometheus counters/metrics | Yes | this spec OWNS and registers `ze_ospf_auth_failures_total{interface,reason}` (per the umbrella `## Shared Contracts` "Metrics (canonical)" table) and increments it on auth failure. Per-owner registration here, NOT in ospf-13 (ospf-13 only scrapes/asserts) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (OSPF authentication row) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` (auth leaves, `$9$` secret, area `inherit`) |
| 3 | CLI command added/changed? | No | (auth surfaced via existing `show ip ospf` commands; CLI additions tracked in ospf-13) |
| 4 | API/RPC added/changed? | No | (no new RPC; auth state rides existing show RPCs) |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/ospf.md` (authentication section) |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/ospf.md` (AuType 2 8-byte field, appended digest, zeroed checksum, Packet Length excludes digest, sequence-number anti-replay) |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc2328.md` (§D, Appendix D), `rfc/short/rfc5709.md`, `rfc/short/rfc7474.md` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (new `test/ospf/ospf-auth.ci`) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` (OSPF auth support) |
| 12 | Internal architecture changed? | No | (auth is a hook within the existing OSPF component; no new component) |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` (`ze_ospf_auth_failures_total`, owned and registered here per the umbrella canonical table; surfaced in ospf-13) |
| 15 | Registered plugin/event/command/capability changed? | No | |
| 16 | Changed files referenced by doc source anchors? | No | grep `docs/` for source anchors at completion |
| 17 | Existing docs show examples for this area? | No | verify `docs/guide/ospf.md` auth examples against YANG at completion |

## Files to Create
- `internal/plugins/ospf/packet/auth_verify.go` - verify-on-receive and sign-on-send helpers over packet bytes, using the existing ospf-2 common-header codec; AuType 1 (Simple), AuType 2 MD5/HMAC-SHA (RFC 2328 Appendix D and RFC 5709), and AuType 3 RFC 7474 extended-sequence HMAC-SHA; constant-time compare; zeroed-checksum + appended-digest framing; Packet-Length-excludes-digest handling
- `internal/plugins/ospf/packet/auth_verify_test.go` - per-algorithm per-packet-type sign/verify, AuType-2 field layout, AuType-3 field and sequence-trailer layout, zeroed-checksum, appended-digest boundary, wrong-key, constant-time, boundary tests
- `internal/plugins/ospf/auth_keystore.go` - key store: per-interface chains with area `inherit` resolution, active-key + AuType selection, `$9$` decode to derive MD5/HMAC keys, rotation overlap window, per-neighbour Cryptographic Sequence Number anti-replay state for AuType 2 and 64-bit AuType 3
- `internal/plugins/ospf/auth_keystore_test.go` - key store, `inherit`, rotation, replay, secret-encoding tests
- `internal/plugins/ospf/auth_wiring.go` - engine glue: build the store on config apply/reload, install the verify chokepoint on the ospf-4 dispatcher and the sign step on the ospf-3 transmit path, increment the failure counter at the reject site
- `test/ospf/ospf-auth.ci` - functional test (wrong/no key rejected, correct key forms adjacency, area `inherit`, hitless rotation, `$9$` rendering)
- `rfc/short/rfc5709.md` - OSPFv2 HMAC-SHA Cryptographic Authentication summary
- `rfc/short/rfc7474.md` - Security Extensions for OSPFv2 (auth trailer) summary
- YANG auth leaves: authored in `internal/plugins/ospf/yang/ze-ospf-conf.yang`, coordinated with spec-ospf-4 (ospf-4 owns the module; this spec contributes the auth subtree)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-14. | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- key store skeleton + verify/sign hook points; failing wiring tests
   - Tests: `TestOSPFAuthKeyStore`, `TestOSPFAuthWrongKeyRejected` (initially failing)
   - Files: `internal/plugins/ospf/auth_keystore.go`, `internal/plugins/ospf/packet/auth_verify.go` (stubs), `auth_wiring.go` hook points on the ospf-4 dispatcher and ospf-3 transmit path
   - Verify: hooks are reachable from config and the packet RX/TX paths; tests fail because verify/sign are stubs
2. **Phase: Verify/sign codec (AuType 1 Simple)** -- 8-byte cleartext password over the ospf-2 common-header field; normal checksum kept
   - Tests: `TestOSPFAuthSignVerifySimple`, `TestOSPFAuthWrongKeyRejected` (password case)
   - Files: `auth_verify.go`
   - Verify: round-trip sign/verify per packet type; mismatch rejected; checksum computed normally for AuType 1
3. **Phase: AuType 2 MD5 (RFC 2328 Appendix D)** -- 8-byte field layout, appended 16-byte MD5 digest, zeroed checksum, Packet-Length-excludes-digest; constant-time compare
   - Tests: `TestOSPFAuthSignVerifyMD5`, `TestOSPFAuthCryptoFieldLayout`, `TestOSPFAuthZeroedChecksum`, `TestOSPFAuthDigestAppendedExcludedFromLength`, `TestOSPFAuthConstantTimeCompare`
   - Files: `auth_verify.go`
   - Verify: per-packet-type sign/verify; Checksum==0 on output; digest appended and excluded from Packet Length; `hmac.Equal` used
4. **Phase: AuType 2 RFC 5709 HMAC-SHA** - SHA-1/256/384/512, Apad construction, Auth Data Length = digest length, same 8-byte AuType 2 field as MD5
   - Tests: `TestOSPFAuthSignVerifyHMACSHA`
   - Files: `auth_verify.go`
   - Verify: per-algorithm sign/verify; Apad fill on sign and verify; Auth Data Length matches the algorithm output length
5. **Phase: AuType 3 RFC 7474 extended sequence auth** - 24-bit reserved + 32-bit Key ID + Auth Data Length field, 64-bit sequence trailer before the HMAC-SHA digest
   - Tests: `TestOSPFAuthType3FieldLayout`, `TestOSPFAuthType3SequenceTrailer`, `TestOSPFAuthSignVerifyHMACSHA`
   - Files: `auth_verify.go`, `auth_keystore.go`
   - Verify: field layout is RFC 7474, the appended sequence number is covered by the digest, replay state uses the 64-bit value, and Packet Length excludes the sequence number and digest
6. **Phase: Anti-replay (Cryptographic Sequence Number)** - per-neighbour send counter (monotonic seed) + per-(neighbour, key-id) last-accepted receive value for 32-bit AuType 2 and 64-bit AuType 3
   - Tests: `TestOSPFAuthReplay`, `TestOSPFAuthType3SequenceTrailer`
   - Files: `auth_keystore.go`, `internal/plugins/ospf/neighbor/` state
   - Verify: sequence < last accepted rejected as replay; >= accepted updates the last-accepted value; the send counter never regresses
7. **Phase: Key store + `inherit` + rotation + secrets** -- per-interface chains, area `inherit` resolution, active-key/AuType selection, `$9$` decode, overlap window
   - Tests: `TestOSPFAuthKeyStore`, `TestOSPFAuthInherit`, `TestOSPFAuthRotation`, `TestOSPFAuthSecretEncoding`
   - Files: `auth_keystore.go`, config resolve, YANG auth leaves
   - Verify: `inherit` resolves to the area chain; rotation accepts both keys; keys stored `$9$`-encoded; plaintext not retained
8. **Phase: Wire hooks live** - verify on the ospf-4 RX dispatcher before Type routing, sign on the ospf-3 TX path for all 5 packet types; counter increment on reject with the right `reason`
   - Tests: `TestOSPFAuthWrongKeyRejected` passes (positive and negative); `TestOSPFAuthCryptoFieldLayout`, `TestOSPFAuthType3FieldLayout`
   - Files: `auth_wiring.go`, ospf-4 dispatcher, ospf-3 transmit path
   - Verify: wrong/no key fails adjacency; correct key forms it; rejects increment `ze_ospf_auth_failures_total` with the correct `reason`
9. **Functional test** -- `test/ospf/ospf-auth.ci`
10. **RFC refs** -- `// RFC 2328 Appendix D` / `// RFC 5709 Section X.Y` / `// RFC 7474 Section X.Y` comments at enforcing code
11. **Full verification** -- `make ze-verify`
12. **Complete spec** -- learned summary, delete spec

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-14 has implementation with file:line |
| Feature completeness | All five packet types (Hello/DD/LS Request/LS Update/LS Ack) and all schemes (Simple, MD5, AuType 2 HMAC-SHA family, AuType 3 RFC 7474 HMAC-SHA family) sign and verify; no packet type left unauthenticated when a chain is configured |
| Correctness | AuType 2 8-byte field = Reserved 0 + Key ID + Auth Data Length + Crypto Seq; MD5 digest 16 bytes appended; RFC 5709 HMAC-SHA digest 20/32/48/64 appended with Apad; AuType 3 8-byte field = 24-bit reserved + 32-bit Key ID + Auth Data Length plus 64-bit sequence trailer before the digest; Checksum zero for cryptographic auth; Packet Length excludes the digest and AuType 3 sequence trailer; sequence-number anti-replay; matches RFC 2328 Appendix D, RFC 5709, RFC 7474 |
| Naming | YANG kebab-case auth leaves; algorithm enum values match RFC names; `$9$` secret leaf like PPPoE/WireGuard/IS-IS |
| Data flow | RX bytes -> verify (ospf-4 dispatcher) -> Type routing; TX encode -> sign (ospf-3) -> transport; no bypass of verify when a chain is configured |
| CLI grammar | Any auth state surfaced through existing `show ip ospf` commands follows action-before-identifier |
| YANG validation | Algorithm `enumeration`; key-id `range 0..4294967295` with AuType 2 send rejection above 255; secret `length`/pattern; `inherit` constrained; no bare `type string` |
| Rule: security | Constant-time compare on every verify; no decoded key in logs or snapshots; downgrade (AuType mismatch under configured auth) rejected; replay rejected via the sequence number |
| Rule: no-duplication | Reuse the ospf-2 common-header codec and the ospf-2 checksum; no second auth encoder; do not share IS-IS auth code (different framing) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Verify/sign helpers | `ls internal/plugins/ospf/packet/auth_verify.go` |
| Key store | `ls internal/plugins/ospf/auth_keystore.go` |
| Engine glue | `ls internal/plugins/ospf/auth_wiring.go` |
| Functional test | `ls test/ospf/ospf-auth.ci` |
| RFC summaries | `ls rfc/short/rfc5709.md rfc/short/rfc7474.md` |
| Constant-time compare | `grep -n 'hmac.Equal\|subtle.ConstantTimeCompare' internal/plugins/ospf/packet/auth_verify.go` |
| Zeroed-checksum framing | `TestOSPFAuthZeroedChecksum`, `TestOSPFAuthDigestAppendedExcludedFromLength` pass |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | AuType, Key ID, and Auth Data Length validated before reading the appended digest; the digest length matches the algorithm; no slice past the datagram bound; Packet Length sanity vs datagram length |
| Constant-time compare | Every digest/password comparison uses `hmac.Equal` / `subtle.ConstantTimeCompare`; no `bytes.Equal`/`==` on digests or passwords |
| Key handling | Keys stored `$9$`-encoded at rest; decoded only in memory; never logged, never in `show`/web/snapshots; zeroed when feasible after a config change drops a key |
| Downgrade resistance | Under configured auth, an AuType 0 packet or an AuType that does not match the configured scheme is rejected (no silent unauthenticated acceptance) |
| Replay resistance | Per-(neighbour, key-id) non-decreasing Cryptographic Sequence Number enforced; the send counter is seeded so it never regresses across a restart; a replayed/old packet is rejected and counted |
| Zeroed-checksum trap | AuType 2 output always has Checksum == 0; the digest is computed over the zeroed-checksum packet; never backfill a checksum after signing (trap #10) |
| Resource use | Verify cost bounded by the number of valid keys in an interface chain; cap chain size to avoid CPU amplification on a flood of forged packets |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read RFC 2328 §D/Appendix D, RFC 5709/7474 summary / Current Behavior |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Interop mismatch | Capture with tcpdump, compare the 8-byte field / appended digest / checksum to FRR, fix the codec |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE, write IMMEDIATELY when you learn something -->
<!-- Route at completion: subsystem → arch doc, process → rules, knowledge → memory.md -->
- OSPF authentication is "a crypto backend plus a key store wired into two shared hook points" (verify on the ospf-4 RX dispatcher, sign on the ospf-3 TX path) -- the same shape as the IS-IS sibling, but with header-AuType framing, an appended digest, and a Cryptographic Sequence Number for anti-replay instead of TLV 10 and authenticated purges.
- The single highest interop risk is the AuType-2 framing: zeroed checksum (trap #10), the appended digest, and Packet Length excluding the digest. Pin each in a dedicated test and an RFC comment at the codec.

## Core Insight
<!-- Optional: the single most important design revelation from this work. -->
The load-bearing OSPF-specific detail is that for AuType 2 the digest lives
*outside* the packet (appended after the body, excluded from Packet Length) and
the common-header Checksum is forced to zero, while the 8-byte Authentication
field carries only the Key ID, Auth Data Length, and Cryptographic Sequence
Number. Get that framing and the sequence-number anti-replay right and the rest is
a standard HMAC backend over the existing ospf-2 codec.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Verify/sign helpers in `packet`, key store + replay state in the component | Put everything in the component | `packet` stays runtime-free and testable on raw bytes; the key store and per-neighbour sequence state need config/runtime context |
| Single active signing key per interface, all valid keys accepted on receive | Per-key-id calendar selection on receive | Simplest hitless rotation; matches FRR key rollover (guide sec 8) and the IS-IS sibling |
| `$9$` reversible encoding for keys | Plaintext leaf, or one-way hash | Consistent with PPPoE/WireGuard/IS-IS; keys must be reversible to derive the MD5/HMAC, but never shown plaintext |
| Reuse the ospf-2 common-header codec + zero-Checksum encode; no IS-IS code sharing | Share an auth backend with IS-IS | OSPF framing differs (header AuType + appended digest + sequence number vs TLV 10 + in-TLV digest + purges); a shared abstraction would leak detail into both (umbrella decision) |
| Per-neighbour Cryptographic Sequence Number for anti-replay | No replay protection | RFC 2328 Appendix D mandates the non-decreasing sequence; it is the only freshness mechanism OSPF auth provides |

## Known Limitations
<!-- Deliberate scope boundaries and constraints accepted. -->
- OSPF authentication proves key possession plus monotonic-sequence freshness; AuType 1 (Simple) is sanity-only and not a security mechanism (it travels in clear).
- Time-based per-key validity windows are simplified to an active signing key plus all currently valid keys accepted on receive; full calendar-scheduled key lifetimes are a future enhancement.
- The Cryptographic Sequence Number send counter is seeded from a monotonic source rather than persisted across restarts; a peer that strictly enforces non-decreasing sequence across our restart relies on that seed never regressing (R-8).
- `ze_ospf_auth_failures_total{interface,reason}` is OWNED and registered by this spec (per the umbrella canonical table) and incremented at the rejection site here; spec-ospf-13 only scrapes/surfaces it.

## RFC Documentation

Add `// RFC 2328 Appendix D: "<quoted requirement>"`, `// RFC 5709 Section X.Y: "<quoted requirement>"`, and `// RFC 7474 Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: the AuType 2 8-byte field structure (Reserved 0, Key ID, Auth Data Length, Cryptographic Sequence Number) and the appended MD5 digest (RFC 2328 Appendix D); the zeroed common-header Checksum for AuType 2 and the RFC 7474 trailer (trap #10); Packet Length excludes the appended digest; the non-decreasing Cryptographic Sequence Number anti-replay rule; the RFC 5709 Apad construction and digest lengths (20/32/48/64) for HMAC-SHA; the AuType codes (0 Null, 1 Simple, 2 Cryptographic) and algorithm names matching the ospf-4 schema enum; constant-time comparison on every verify; the engine signs the fully-constructed packet and the ospf-3 transport does not alter bytes after signing.

## Implementation Summary

### What Was Implemented
- `packet/auth_verify.go`: `Sign`/`Verify` for AuType 0/1/2/3 -- Simple password, RFC 2328 keyed-MD5, RFC 5709 HMAC-SHA-1/256/384/512 (Apad, Ko derivation), RFC 7474 AuType 3 (64-bit sequence trailer, 0x0001 protocol-id key suffix); `subtle.ConstantTimeCompare`/`hmac.Equal` on every verify; reuses the ospf-2 zero-checksum + auth-field-excluding framing. `packet/wire.go`: `readUint64`/`writeUint64`.
- `auth_keystore.go`: per-interface chain resolution with area `inherit`, `$9$` decode (`secret.Decode` + plaintext fallback), active-key signing, accept-any-chain-key on receive (rotation), per-(interface,neighbour,key-id) non-decreasing-sequence anti-replay, AuType selection (`extended-sequence` -> AuType 3).
- `auth_wiring.go`: `signPacket` (transport `SetSigner` hook -- rewrites AuType + checksum + auth field + digest), `verifyPacket` (dispatcher `authOK` chokepoint before handler routing), `ze_ospf_auth_failures_total{interface,reason}` increment.
- `transport/transport.go`: `SetSigner` + signer applied in `SendPacket`. `dispatcher.go`: `authOK` gate after checksum/area, before the handler. `instance.go`: `auth` store + metric, `installAuthHooks`, `setConfig` calls `auth.configure`.
- `ze-ospf-conf.yang` + `config.go`: `key-chains/extended-sequence` leaf (selects AuType 3).

### Bugs Found/Fixed
- (Review-gate findings recorded in the Review Gate section.)

### Documentation Updates
- `docs/guide/ospf.md` (authentication section), `docs/guide/configuration.md` (key-chain config + example), `docs/features.md` (auth row + anchor), `docs/comparison.md` (auth paragraph), `docs/plugin-development/metrics.md` (`ze_ospf_auth_failures_total`), `docs/comparison.html` regenerated. `make ze-doc-test` PASS.

### Deviations from Plan
- The verify/sign helpers expose `Sign(pkt, auType, key, seq)` / `Verify(wire, auType, key) (seq, ok)` operating on encoded packet bytes, rather than a per-packet-type API; the single transport signer + dispatcher verify chokepoints cover all five packet types without per-encoder changes.
- AuType 3 is selected by a per-chain `extended-sequence` boolean (added to the YANG) since AuType 2 and 3 share the HMAC-SHA algorithm enum.
- Boot-count NVRAM persistence is not implemented (the AuType 3 high-order word stays 0); intra-session replay is enforced, cross-reboot replay protection is best-effort (Known Limitation).

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

<!-- MANDATORY: Maps each stated goal to concrete proof it was achieved. -->
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Wrong/no key rejected | unit test | `TestOSPFAuthWrongKeyRejected` |
| Correct key forms adjacency, packets processed | unit + functional test | `TestOSPFAuthSignVerifyHMACSHA` (positive), `test/ospf/ospf-auth.ci` |
| Sign/verify all schemes + all 5 packet types | unit test | `TestOSPFAuthSignVerifySimple`, `TestOSPFAuthSignVerifyMD5`, `TestOSPFAuthSignVerifyHMACSHA`, `TestOSPFAuthType3FieldLayout`, `TestOSPFAuthType3SequenceTrailer` |
| AuType 2 and 3 framing (zeroed checksum, appended digest, sequence trailer, length) | unit test | `TestOSPFAuthZeroedChecksum`, `TestOSPFAuthDigestAppendedExcludedFromLength`, `TestOSPFAuthCryptoFieldLayout`, `TestOSPFAuthType3FieldLayout`, `TestOSPFAuthType3SequenceTrailer` |
| Anti-replay via Cryptographic Sequence Number | unit test | `TestOSPFAuthReplay`, `TestOSPFAuthType3SequenceTrailer` |
| Area `inherit` + hitless rotation | unit + functional test | `TestOSPFAuthInherit`, `TestOSPFAuthRotation`, `test/ospf/ospf-auth.ci` |
| Keys `$9$`-encoded, never plaintext | unit test | `TestOSPFAuthSecretEncoding` |
| FRR MD5/HMAC-SHA interop on the wire | interop test (Linux-pending) | `test/interop/scenarios/ospf-auth-frr` (authored and run under spec-ospf-13) |

## Review Gate

<!-- BLOCKING (rules/planning.md Completion Checklist step 7): -->
<!-- Run /ze-review BEFORE the final testing/verify step. Record the findings here. -->

### Run 1 (initial -- independent security review agent on the auth diff)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| B1 | BLOCKER | Replay check accepted an EQUAL cryptographic sequence (`seq < last`); RFC 7474 §2 + AC-9 require strictly greater, so a captured packet with `seq == last` replayed through to the handlers | `auth_keystore.go verify` | Changed to `seq <= last`. Test `TestOSPFAuthReplay` (equal-seq case) |
| I1 | ISSUE | Replay high-water mark was per-(interface,neighbor,key-id), not per packet type; RFC 7474 §2 requires per-type, else a reordered lower-sequence packet of another type is a false replay | `auth_keystore.go replayKey` | Added `pktType` to the slot. Test `TestOSPFAuthReplayPerType` |
| N1 | NOTE | `extended-sequence` (AuType 3) with `md5`/`simple` is an RFC-undefined combination, previously unvalidated | `config.go validateConfig` | Reject via `ErrESNRequiresHMAC`. Test `TestOSPFAuthExtendedSequenceRequiresHMAC` |
| - | clean | Crypto primitives (keyed-MD5 vs HMAC-SHA, Ko/Apad, AuType 3 seq+protocol-id, constant-time), bounds checks, downgrade resistance, the sign-hook checksum + buffer-aliasing, concurrency | -- | confirmed correct by the review |

### Fixes applied
- B1: `auth_keystore.go` replay comparison `seq <= last` (RFC 7474 §2 strictly-greater).
- I1: `replayKey` gains `pktType` (per-packet-type high-water mark).
- N1: `validateConfig` rejects `extended-sequence` with a non-HMAC algorithm (`isHMACSHA`).
- N3 (boot-count wrap / NVRAM) remains a documented Known Limitation (intra-session replay enforced).

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | clean | B1/I1/N1 fixed + tested | -- | `go test -race ./internal/plugins/ospf/...` EXIT 0; `make ze-lint-changed` 0; `make ze-ospf-test` 12/12; `make ze-doc-test` PASS |

### Run 3 (re-verification 2026-06-22 -- independent security agent audit of all 14 ACs + fail-open/swallow/replay/leak hunt)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | clean | NO fail-open, NO silent-swallow, NO replay weakness, NO key leakage. AuType 0/1/2/3 reject paths, strictly-greater per-(iface,neighbor,key-id,type) replay, constant-time compare, zeroed-checksum/appended-digest framing, and exact-length trailer all confirmed correct | `auth_verify.go`, `auth_keystore.go` | confirmed by the audit |
| 1 | ISSUE | the RX metric-increment + drop glue (`verifyPacket`) had no test -- the AC-1/2/9 "counter increments on reject" claim was unverified | `auth_wiring.go verifyPacket` | FIXED: `TestEngineVerifyPacketDropsAndCounts` drives the dispatcher chokepoint -- an unauthenticated packet on an authenticated interface is dropped AND `ze_ospf_auth_failures_total` increments; a signed packet passes and does not bump it |
| 2 | ISSUE | a simple-password (AuType 1) secret > 8 octets was silently truncated (RFC 2328 App D fixes the auth field at 8 octets) | `config.go validateConfig` | FIXED (RFC-correct): reject > 8-octet simple passwords with `ErrSimplePasswordLen`; boundary test `TestSimplePasswordLengthBoundary` (8 accepted, 9 rejected) |
| 3 | NOTE | `test/ospf/ospf-auth.ci` validates config surface only, not end-to-end adjacency/rotation/`$9$` rendering | `test/ospf/ospf-auth.ci` | RECORDED: behaviors are unit-covered; FRR `ospf-auth-frr` covers the e2e path (Linux/QEMU). |
| 4 | MINOR | no test for an explicit per-interface key-chain overriding area `inherit`; no OSPF-level `$9$` secret-encoding test; per-interface `authentication.mode` algorithm values are dead config (auth activates via a key-chain only) | `auth_keystore.go`, `yang` | RECORDED as follow-ups; the `inherit` path + the shared `ze:sensitive` framework are covered elsewhere. |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete, every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled, 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/ospf/`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`, no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass, defer with user approval)
- [ ] RFC constraint comments added (RFC 2328 Appendix D / RFC 5709 / RFC 7474)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (`ospf-auth-frr` under spec-ospf-13)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING, before ANY commit)
- [ ] Critical Review passes, all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-ospf-12-auth.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-12-auth.md` only (preserves edited spec in git history from commit A)
