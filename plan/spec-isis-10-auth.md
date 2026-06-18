# Spec: isis-10-auth

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-isis-2-wire.md, spec-isis-4-component-config.md, spec-isis-5-adjacency.md, spec-isis-6-lsdb.md |
| Phase | - |
| Updated | 2026-06-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-isis-0-umbrella.md` (row isis-10) - umbrella scope, dependency graph
4. `plan/spec-isis-2-wire.md` - TLV 10 codec (already exists from isis-2)
5. `plan/spec-isis-5-adjacency.md`, `plan/spec-isis-6-lsdb.md` - the PDU paths this spec signs/verifies
6. `docs/research/isis-implementation-guide.md` sec 7 (Authentication), sec 12 item 4 (TLV ordering)
7. `internal/component/config/secret/secret.go` - Ze `$9$` reversible sensitive-value encoding

## Task

Add IS-IS authentication to Ze: a key store plus per-PDU verify-on-receive and
sign-on-send for all four PDU classes (IIH, LSP, CSNP, PSNP), across both
routing levels. The TLV 10 (Authentication Information) wire codec already exists
from spec-isis-2; this spec is verification, signing, and key management built on
top of it.

Three authentication types are in scope. Cleartext (TLV 10 authentication type
1) is included for basic sanity checking only, not security. HMAC-MD5 (RFC 5304,
authentication type 54) and generic cryptographic authentication (RFC 5310,
authentication type 3, carrying HMAC-SHA family digests, SHA-256 first) provide
real integrity protection.

Keys are organised as key chains, not bare strings, matching the canonical
"Authentication config model" in spec-isis-0-umbrella. Each key in a chain has a
key-id, an algorithm (enum cleartext/hmac-md5/hmac-sha-256/...), a secret
(`$9$`-encoded), and optional send and accept lifetimes for hitless rotation.
Per-interface chains authenticate IIH; per-level chains authenticate LSP/SNP (the
area key for L1, the domain key for L2). A chain holds multiple keys so an
operator can rotate keys without dropping adjacencies (an active key for signing
plus standby keys accepted on receive).

Keys are configured through YANG. The YANG key-chain leaves are OWNED by
spec-isis-4 (config): isis-4 defines the schema (chain, key-id, algorithm enum,
`$9$` secret, lifetimes) and this spec owns the verify and sign semantics and the
runtime key lookup built on top of those leaves. The authentication type codes
and algorithm names used here MUST match the isis-4 schema enum and the canonical
umbrella model. Sensitive key material uses the Ze `$9$` reversible encoding
(package `internal/component/config/secret`), the same mechanism PPPoE passwords
and WireGuard keys already use, so keys never appear as plaintext in
`show configuration` or config backups.

Authentication is enforced on both transmit and receive. On send, the engine
signs every PDU of an authenticated PDU class with the active key. On receive, it
verifies against every currently valid key in the relevant chain and rejects any
PDU that fails. Rejected PDUs do not form or sustain adjacencies, are not stored
in the LSDB, and do not satisfy synchronisation; each rejection increments
`ze_isis_auth_failures_total{level,interface}`, which this spec OWNS and
registers (per the umbrella "Metrics (canonical)" table); spec-isis-13 only
scrapes/surfaces it.

Two ordering and integrity constraints are load-bearing. RFC 5304 section 1
requires the Authentication TLV to be the first TLV in the PDU; Ze enforces this
on encode and validates position on decode. The fields zeroed before the digest
depend on the PDU class. For LSPs, RFC 5304 section 2 requires three fields to be
set to zero before the HMAC-MD5 digest is computed: the Authentication Value
inside TLV 10, the LSP Checksum, and the Remaining Lifetime. The ISO Fletcher
checksum (spec-isis-2) is computed after the authentication value is in place, so
the order of operations is signing then checksum on send, and checksum acceptance
then digest verification on receive; on receive a system saves the Authentication
Value, Checksum, and Remaining Lifetime, zeroes them, computes the digest, then
restores them (RFC 5304 section 2). For IIH, CSNP, and PSNP there is no Checksum
or Remaining Lifetime field in the PDU header, so only the Authentication Value is
zeroed (RFC 5304/5310); per RFC 5304 section 2 an implementation MUST NOT include
the optional Checksum TLV in Sequence Number PDUs and Hellos when authentication
is in use. All digest comparisons use a constant-time compare to avoid timing
side channels.

Ze has no IS-IS authentication today beyond the inert TLV 10 codec from isis-2;
there is no key store, no verify path, no sign path, and no failure accounting.

## Required Reading

### Architecture Docs
- [ ] `docs/research/isis-implementation-guide.md` sec 7 (Authentication) - mechanisms, levels, key chains
  -> Constraint: cleartext (type 1) is sanity-only; HMAC-MD5 and generic crypto are the real options
  -> Constraint: support multiple keys with time-based activation for hitless rotation
  -> Constraint: RFC-correct authentication type codes are cleartext = 1, HMAC-MD5 = 54 (RFC 5304), generic crypto = 3 (RFC 5310); the research guide sec 7 has them swapped (it lists MD5=3, generic=54), so trust the RFCs.
- [ ] `docs/research/isis-implementation-guide.md` sec 12 item 4 (TLV Ordering) - auth TLV must be first
  -> Constraint: TLV 10 MUST be the first TLV; some peers reject otherwise. Enforce on encode, validate on decode.
- [ ] `ai/rules/config-surface.md`, `ai/rules/config-naming.md` - YANG vs env var, kebab-case
  -> Constraint: keys are operator-facing YANG config (visible in commit/rollback), not env vars; kebab-case leaves
- [ ] `internal/component/config/secret/secret.go` - `$9$` reversible encoding (Encode/Decode/IsEncoded)
  -> Constraint: store key material `$9$`-encoded like PPPoE/WireGuard; decode only in memory at sign/verify time
- [ ] `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md` - zero-copy, no-alloc hot path
  -> Constraint: verify reads the received raw bytes in place; sign writes the digest into the encode buffer; no per-PDU plaintext-key allocation churn

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5304.md` - IS-IS Cryptographic Authentication, HMAC-MD5 (CREATED)
  -> Constraint: Authentication TLV first in the PDU (sec 1); HMAC-MD5 authentication type 54
  -> Constraint: digest computed with the Authentication Value field zeroed; for LSPs the Checksum AND Remaining Lifetime fields are ALSO set to zero before the digest (RFC 5304 sec 2); IIH/CSNP/PSNP have no checksum or lifetime field, so only the Authentication Value is zeroed
  -> Constraint: a purge (zero Remaining Lifetime) MUST still carry valid authentication; an IS that initiates a purge removes the LSP body and adds the authentication TLV, MUST NOT accept unauthenticated purges, and MUST NOT accept purges that contain TLVs other than the authentication TLV (RFC 5304 sec 2)
- [ ] `rfc/short/rfc5310.md` - IS-IS Generic Cryptographic Authentication (CREATED)
  -> Constraint: authentication type 3, Key-ID + HMAC value; algorithm agility (HMAC-SHA-1/224/256/384/512)
  -> Constraint: SHA-256 first; digest covers the PDU with the digest octets zeroed

**Key insights:**
- TLV 10 codec exists (isis-2); this spec is the crypto backend (`crypto/hmac`, `crypto/md5`, `crypto/sha256`) plus a key store and the per-PDU verify/sign wiring
- Auth applies per PDU class: IIH is per-interface (circuit key), LSP/CSNP/PSNP are per-level (area key for L1, domain key for L2)
- RFC type codes: cleartext = 1, HMAC-MD5 (RFC 5304) = 54, generic crypto (RFC 5310) = 3
- TLV 10 must be first; for LSPs the Authentication Value, Checksum, and Remaining Lifetime are all zeroed before the digest (RFC 5304 sec 2), LSP signing happens before the Fletcher checksum; constant-time compare on every verify
- Purges are authenticated: a zero-lifetime purge still carries valid auth, the originator regenerates the auth TLV on the stripped LSP, and an authenticated IS rejects unauthenticated purges and purges carrying non-auth TLVs (RFC 5304 sec 2)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/isis/packet/` (TLV 10 codec from isis-2) - encodes/decodes the Authentication TLV bytes (type, value) but never computes or checks a digest
  -> Constraint: reuse the existing TLV 10 codec; do not add a second auth TLV encoder
- [ ] `internal/component/config/secret/secret.go` - `$9$` Encode/Decode/IsEncoded for sensitive leaves
  -> Constraint: key store decodes `$9$` to plaintext only in memory when deriving HMAC keys
- [ ] `plan/spec-isis-5-adjacency.md` IIH path, `plan/spec-isis-6-lsdb.md` LSP/SNP paths - where PDUs are built (TX) and consumed (RX) today with no auth hook
  -> Constraint: add a single verify hook on the RX dispatch and a single sign hook on the TX encode, per PDU class

**Behavior to preserve:**
- TLV 10 codec byte layout from isis-2 is unchanged (this spec consumes it, does not alter the wire struct)
- Fletcher checksum computation from isis-2 is unchanged; only the order relative to signing is specified here
- Unauthenticated operation (no key configured) remains the default and continues to work unchanged
- PDU build/parse paths from isis-5/isis-6 keep their shape; auth is an added hook, not a rewrite

**Behavior to change:**
- New `isis` YANG auth leaves (coordinated with isis-4) and a key store in the component
- RX dispatch gains a verify step that can reject a PDU before adjacency/LSDB processing
- TX encode gains a sign step that inserts TLV 10 first and (for LSPs) runs before checksum
- `ze_isis_auth_failures_total` is incremented on every rejected PDU; this spec OWNS and registers the series (isis-13 only scrapes/surfaces it)

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Config: the `isis` auth subtree (per-level area/domain keys, per-interface circuit keys, auth type, key-id, `$9$`-encoded secret), arriving via the YANG-validated config tree (leaves owned by isis-4)
- RX: a decoded PDU (IIH/LSP/CSNP/PSNP) on the receive dispatch, still holding its raw bytes
- TX: a PDU about to be encoded into the send buffer

### Transformation Path
1. **Key config -> store:** config resolve builds per-level and per-interface key chains; `$9$` secrets are decoded to derive HMAC keys held only in memory; an active key is selected for signing and all unexpired keys are retained for verify
2. **PDU receive -> verify:** locate TLV 10 (must be first); select the key chain by PDU class (circuit for IIH, area for L1 LSP/SNP, domain for L2 LSP/SNP); for each valid key recompute the digest over the PDU with the Authentication Value zeroed, and for LSPs also with the Checksum and Remaining Lifetime fields zeroed (saving and restoring all three per RFC 5304 sec 2, after checksum acceptance); constant-time compare against the received digest
3. **verify -> accept/reject:** match against any valid key accepts and the PDU proceeds to adjacency/LSDB/SNP handling; no match rejects, drops the PDU, and increments the auth-failure counter; an LSP with zero Remaining Lifetime (a purge) is verified the same way, and per RFC 5304 sec 2 an unauthenticated purge or a purge carrying TLVs other than TLV 10 is rejected
4. **PDU send -> sign:** reserve TLV 10 as the first TLV, encode the body (including the Padding TLV 8 for IIH), compute the digest over the buffer with the Authentication Value zeroed, and for LSPs with the Checksum and Remaining Lifetime fields zeroed (RFC 5304 sec 2), write the digest into TLV 10; for LSPs compute the Fletcher checksum after the digest is in place. When the originator purges an LSP it removes the body, keeps only TLV 10, and regenerates the authentication over the purge (RFC 5304 sec 2)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree <-> key store | resolved typed key chains (`$9$` decoded in memory) | [ ] |
| RX dispatch <-> verify | raw PDU bytes + PDU class -> accept/reject | [ ] |
| TX encode <-> sign | encode buffer + active key -> TLV 10 first, then checksum | [ ] |
| verify/sign <-> TLV 10 codec | existing isis-2 Authentication TLV encode/decode | [ ] |
| reject <-> metrics | increment `ze_isis_auth_failures_total` (owned and registered here; surfaced in isis-13) | [ ] |

### Integration Points
- `internal/component/isis/packet/auth_verify.go` - verify/sign helpers over PDU bytes, using the existing TLV 10 codec
- Key store in the component (resolved from YANG, holds chains + active key)
- RX dispatch hook (isis-5 IIH, isis-6 LSP/SNP) calls verify before processing
- TX encode hook (isis-5 IIH, isis-6 LSP/SNP) calls sign during encode
- `internal/component/config/secret` for `$9$` decode of key material
- `ze_isis_auth_failures_total` (owned and registered here per the umbrella canonical table; surfaced in isis-13)

Consistent with the Shared Contracts in spec-isis-0-umbrella: verify runs on the
isis-4 PDU receive dispatcher path (the dispatcher rejects a failed PDU before it
routes by PDU class to isis-5/isis-6/isis-7), and sign runs on the isis-3 transmit
path (the digest is in place before the frame is handed to the raw L2 transport);
the isis-5/isis-6 references above select which chain applies per PDU class.

### Architectural Verification
- [ ] No bypassed layers (RX: bytes -> verify -> dispatch; TX: encode -> sign -> checksum -> wire)
- [ ] No unintended coupling (verify/sign in `packet` depend on TLV 10 codec only, not on runtime state)
- [ ] No duplicated functionality (reuse isis-2 TLV 10 codec and isis-2 checksum; no second auth encoder)
- [ ] Zero-copy preserved (verify reads received bytes in place; sign writes into the encode buffer)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | RFC-correct auth type codes are cleartext 1, HMAC-MD5 54 (RFC 5304), generic crypto 3 (RFC 5310); the research guide sec 7 has them swapped | RFC 5304/5310, guide sec 7 | Wrong wire type byte; FRR rejects | `rfc/short/rfc5304.md` + `rfc5310.md` summaries; isis-auth-frr interop | unvalidated |
| A-2 | TLV 10 codec from isis-2 exposes the value bytes so a digest can be written in place after the rest of the PDU is encoded | isis-2 codec | Need a placeholder/back-patch path in the encoder | unit sign test round-trip | unvalidated |
| A-3 | The LSP digest is computed with the Authentication Value, Checksum, and Remaining Lifetime all zeroed (RFC 5304 sec 2) and the Fletcher checksum is applied after signing | RFC 5304 sec 2, guide sec 12.1 | Checksum/auth interaction wrong; interop failures; FRR rejects the digest | checksum-after-sign unit test (zeroes all three fields) + isis-auth-frr | unvalidated |
| A-4 | `internal/component/config/secret` `$9$` is the right sensitive-leaf mechanism for IS-IS keys (as for PPPoE/WireGuard) | `secret.go`, config-surface rule | Need a different secret store | config resolve test (encoded leaf decodes to key) | unvalidated |
| A-5 | A single active signing key plus all unexpired keys accepted on receive is sufficient for hitless rotation | guide sec 7 key chains | Need per-key-id selection on receive too | rotation functional test (overlap window) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Non-constant-time compare leaks key material via timing | code review flags `bytes.Equal` on digests | mandatory `hmac.Equal` / `subtle.ConstantTimeCompare`; lint/grep check |
| R-2 | TLV 10 not first causes silent peer rejection | one-way adjacency with a strict peer | enforce first-TLV on encode; decode validates position; interop with FRR |
| R-3 | LSP checksum computed before signing corrupts either field | round-trip or interop checksum/auth failures | explicit checksum-after-sign ordering + dedicated test |
| R-4 | Key rotation drops adjacencies (no overlap window) | adjacency flap during key change | accept all unexpired keys on receive; rotation functional test |
| R-5 | Decoded plaintext keys linger or leak into logs/snapshots | key visible in `show`/logs | `$9$` at rest; never log decoded keys; redact in CLI/web |
| R-6 | Unauthenticated PDU accepted when auth is configured (downgrade) | PDU with no TLV 10 forms adjacency under auth | when a chain is configured, a missing/!first TLV 10 is a verify failure |
| R-7 | Spoofed purge: an IS without the key zeroes the Remaining Lifetime of a captured LSP and re-floods it to purge a route | LSDB entry disappears after an unauthenticated zero-lifetime LSP arrives | per RFC 5304 sec 2 reject unauthenticated purges and purges carrying any TLV other than TLV 10; originator regenerates auth on the stripped purge; `TestISISAuthPurge` + functional test |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `isis` config with per-level/interface auth key | -> | key store built; active key selected | `TestISISAuthKeyStore` |
| IIH received with wrong/no key under configured auth | -> | verify fails; adjacency does NOT reach Up; counter increments | `TestISISAuthReject` |
| IIH received with correct key | -> | verify passes; adjacency reaches Up | `TestISISAuthReject` (positive case) |
| LSP signed on send | -> | TLV 10 first, digest valid (auth value, checksum, remaining lifetime zeroed), checksum after sign | `TestISISAuthSignLSP` |
| Purge (zero Remaining Lifetime) received under auth | -> | unauthenticated purge or purge with non-auth TLVs rejected; LSDB entry retained; counter increments | `TestISISAuthPurge` |
| `test/isis/isis-auth.ci` | -> | wrong key rejected, correct key forms adjacency and accepts LSPs end to end | `isis-auth` functional test |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Auth configured, peer sends PDU with no TLV 10 | PDU rejected; no adjacency; auth-failure counter increments |
| AC-2 | Auth configured, peer sends PDU with wrong key | PDU rejected; no adjacency; auth-failure counter increments |
| AC-3 | Auth configured, peer sends PDU with correct key | Adjacency reaches Up; subsequent authenticated LSPs accepted into the LSDB |
| AC-4 | Key rotation: operator adds a new active key while the old key is still valid | No adjacency drop; PDUs signed with either valid key are accepted during the overlap window |
| AC-5 | Cleartext (type 1) configured | TLV 10 carries the plaintext password; match accepted, mismatch rejected (sanity only, not security) |
| AC-6 | HMAC-MD5 (RFC 5304, type 54) configured | Sign and verify succeed for IIH/LSP/CSNP/PSNP; wrong key rejected |
| AC-7 | Generic crypto HMAC-SHA-256 (RFC 5310, type 3) configured | Sign and verify succeed for IIH/LSP/CSNP/PSNP; wrong key rejected |
| AC-8 | Any signed PDU on the wire | TLV 10 is the first TLV; decode rejects a PDU where TLV 10 is present but not first |
| AC-9 | Signed LSP | Auth digest computed over the PDU with the Authentication Value, Checksum, and Remaining Lifetime all zeroed (RFC 5304 sec 2); Fletcher checksum computed after signing; round-trips and verifies |
| AC-10 | Per-interface key on IIH, per-level key on LSP/CSNP/PSNP | Correct chain selected by PDU class and level; cross-use is rejected |
| AC-11 | Key material in config | Stored `$9$`-encoded; never shown as plaintext in `show configuration`, logs, or web |
| AC-12 | Any digest comparison on the verify path | Uses constant-time compare (`hmac.Equal` / `subtle.ConstantTimeCompare`) |
| AC-13 | Auth configured, originator purges an LSP (sets Remaining Lifetime to zero) | The purge removes the LSP body, keeps only TLV 10, and is signed: the digest is regenerated over the purge with Authentication Value, Checksum, and Remaining Lifetime zeroed (RFC 5304 sec 2); the purge round-trips and verifies |
| AC-14 | Auth configured, peer sends a purge (zero Remaining Lifetime) that is unauthenticated, or that carries any TLV other than TLV 10 | The purge is rejected; the LSP is NOT purged from the LSDB; auth-failure counter increments (RFC 5304 sec 2: MUST NOT accept unauthenticated purges or purges with non-auth TLVs) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures an HMAC-SHA-256 key on a circuit and the adjacency forms with an authenticated peer | config -> key store -> IIH sign on TX / verify on RX -> adjacency Up | `TestISISAuthReject` (positive), `test/isis/isis-auth.ci` |
| 2 | Configures a wrong key and the adjacency fails to form | config -> key store -> IIH verify fails -> adjacency stays Down, counter increments | `TestISISAuthReject`, `test/isis/isis-auth.ci` |
| 3 | Rotates the key with no outage by adding a new active key during an overlap window | config reload -> key store updates chain -> both keys accepted on RX -> no flap | `TestISISAuthRotation`, `test/isis/isis-auth.ci` |
| 4 | Meshes with an FRR router using IS-IS authentication | full signed protocol over the wire | `test/interop/scenarios/isis-auth-frr` (noted in spec-isis-13) |
| 5 | Inspects the running config and sees keys obfuscated, not plaintext | config render -> `$9$` encoding | `TestISISAuthSecretEncoding`, `test/isis/isis-auth.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestISISAuthSignVerifyCleartext` | `internal/component/isis/packet/auth_verify_test.go` | type 1 sign/verify per PDU class | |
| `TestISISAuthSignVerifyHMACMD5` | `internal/component/isis/packet/auth_verify_test.go` | type 54 HMAC-MD5 sign/verify per PDU class | |
| `TestISISAuthSignVerifyHMACSHA256` | `internal/component/isis/packet/auth_verify_test.go` | type 3 HMAC-SHA-256 sign/verify per PDU class | |
| `TestISISAuthTLVFirstOnEncode` | `internal/component/isis/packet/auth_verify_test.go` | TLV 10 emitted as the first TLV on sign | |
| `TestISISAuthTLVNotFirstRejected` | `internal/component/isis/packet/auth_verify_test.go` | decode rejects TLV 10 present but not first | |
| `TestISISAuthLSPChecksumAfterSign` | `internal/component/isis/packet/auth_verify_test.go` | LSP digest over zeroed Authentication Value, Checksum, and Remaining Lifetime (RFC 5304 sec 2); checksum applied after signing; round-trips | |
| `TestISISAuthPurge` | `internal/component/isis/packet/auth_verify_test.go` | signed purge (body stripped, only TLV 10, auth regenerated over zeroed lifetime) round-trips; unauthenticated purge and purge carrying non-auth TLVs rejected (RFC 5304 sec 2) | |
| `TestISISAuthConstantTimeCompare` | `internal/component/isis/packet/auth_verify_test.go` | verify uses constant-time compare (no `bytes.Equal` on digest) | |
| `TestISISAuthWrongKeyRejected` | `internal/component/isis/packet/auth_verify_test.go` | mismatched digest rejected per PDU class | |
| `TestISISAuthKeyStore` | `internal/component/isis/auth_keystore_test.go` | per-level/per-interface chains; active key selection | |
| `TestISISAuthRotation` | `internal/component/isis/auth_keystore_test.go` | overlap window accepts old and new key | |
| `TestISISAuthSecretEncoding` | `internal/component/isis/auth_keystore_test.go` | `$9$`-encoded leaf decodes to the derived key; plaintext never retained | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Key length (cleartext password) | 1..254 bytes (TLV 10 value fits in a 255-byte TLV minus type byte) | 254 | 0 (empty) | 255 |
| Key length (HMAC-MD5 secret) | 1..255 bytes | 255 | 0 (empty) | 256 |
| Key length (HMAC-SHA secret) | 1..255 bytes | 255 | 0 (empty) | 256 |
| Key-ID (RFC 5310 generic) | 0..65535 | 65535 | n/a | 65536 |
| Digest length HMAC-MD5 | 16 bytes | 16 | <16 | >16 |
| Digest length HMAC-SHA-256 | 32 bytes | 32 | <32 | >32 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `isis-auth` | `test/isis/isis-auth.ci` | wrong/no key rejected; correct key forms adjacency and accepts LSPs; rotation hitless; keys shown `$9$`-encoded | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `isis-auth-frr` | `test/interop/scenarios/` | FRR isisd | HMAC authentication interop (defined and run in spec-isis-13) | |

### Future (if deferring any tests)
- None planned. Interop scenario `isis-auth-frr` is authored and run under spec-isis-13 (interop harness lives there); the wire/crypto contract it exercises is fully specified here.

## Files to Modify
- `internal/component/isis/packet/` - reuse the existing TLV 10 codec from isis-2 (no wire-struct change); add the verify/sign helpers alongside
- `internal/component/isis/yang/ze-isis-conf.yang` - auth leaves (coordinated with spec-isis-4; isis-4 owns the schema, this spec specifies the auth subtree shape)
- IIH TX/RX path (spec-isis-5) - add the sign-on-encode and verify-on-receive hooks
- LSP/CSNP/PSNP TX/RX path (spec-isis-6) - add the sign-on-encode and verify-on-receive hooks
- `internal/component/isis/server.go` (or config.go) - build the key store from resolved config

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | Yes | `internal/component/isis/yang/ze-isis-conf.yang` auth leaves (per-level `area`/`domain` key, per-interface circuit key, type enum, key-id, `$9$` secret) -- coordinated with spec-isis-4 |
| YANG validation constraints | Yes | type `enumeration` (cleartext/hmac-md5/hmac-sha-256/...); key-id `range 0..65535`; secret `length` and `$9$` pattern; level enum |
| YANG custom validators | Yes | secret leaf accepts `$9$`-encoded or plaintext (auto-encode on commit) via `ze:validate` + `CompleteFn`; reuse the PPPoE/WireGuard secret-leaf pattern |
| CLI commands/flags | Yes | auth state surfaced via `show isis interface` / `show isis neighbor` (auth type, last-failure); `ze_isis_auth_failures_total` is owned/registered HERE and only scraped/surfaced by isis-13 |
| CLI grammar (action before identifier) | Yes | `ai/rules/cli-grammar.md` |
| Editor autocomplete | Yes | YANG enum (auth type) driven; `CompleteFn` for type values |
| Functional test for new RPC/API | Yes | `test/isis/isis-auth.ci` |
| Pipe completeness | Yes | any auth state in show output routes through `ApplyPipes`/`ProcessPipes` |
| Doctor check for runtime dependencies | No | none new (crypto is in-tree stdlib; no socket/file/cert material introduced by auth) |
| Prometheus counters/metrics | Yes | this spec OWNS and registers `ze_isis_auth_failures_total{level,interface}` (per the umbrella `## Shared Contracts` "Metrics (canonical)" table) and increments it on auth failure. Per-owner registration here, NOT in isis-13 (isis-13 only scrapes/asserts) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (IS-IS authentication row) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` (auth leaves, `$9$` secret) |
| 3 | CLI command added/changed? | No | (auth surfaced via existing `show isis` commands; CLI additions tracked in isis-13) |
| 4 | API/RPC added/changed? | No | (no new RPC; auth state rides existing show RPCs) |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/isis.md` (authentication section) |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/isis.md` (TLV 10 first-TLV rule, LSP digest over zeroed Authentication Value + Checksum + Remaining Lifetime, checksum-after-sign, authenticated purges) |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc5304.md`, `rfc/short/rfc5310.md` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (new `test/isis/isis-auth.ci`) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` (IS-IS auth support) |
| 12 | Internal architecture changed? | No | (auth is a hook within the existing IS-IS component; no new component) |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` (`ze_isis_auth_failures_total`, owned and registered here per the umbrella canonical table; surfaced in isis-13) |
| 15 | Registered plugin/event/command/capability changed? | No | |
| 16 | Changed files referenced by doc source anchors? | No | grep `docs/` for source anchors at completion |
| 17 | Existing docs show examples for this area? | No | verify `docs/guide/isis.md` auth examples against YANG at completion |

## Files to Create
- `internal/component/isis/packet/auth_verify.go` - verify-on-receive and sign-on-send helpers over PDU bytes, using the existing TLV 10 codec; cleartext, HMAC-MD5 (RFC 5304), generic crypto HMAC-SHA (RFC 5310); constant-time compare; TLV-first enforcement; LSP digest over zeroed Authentication Value, Checksum, and Remaining Lifetime (RFC 5304 sec 2) with checksum-after-sign; IIH/CSNP/PSNP zero only the Authentication Value; authenticated-purge sign/verify (strip body, keep TLV 10, reject unauthenticated or non-auth-TLV purges)
- `internal/component/isis/packet/auth_verify_test.go` - per-algorithm per-PDU-class sign/verify, TLV-first, checksum ordering, constant-time, wrong-key, boundary tests
- `internal/component/isis/auth_keystore.go` - key store: per-level (area/domain) and per-interface (circuit) chains, active-key selection, `$9$` decode to derive HMAC keys, rotation overlap window
- `internal/component/isis/auth_keystore_test.go` - key store, rotation, secret-encoding tests
- `test/isis/isis-auth.ci` - functional test (wrong/no key rejected, correct key forms adjacency and accepts LSPs, hitless rotation, `$9$` rendering)
- `rfc/short/rfc5304.md` - IS-IS Cryptographic Authentication (HMAC-MD5) summary
- `rfc/short/rfc5310.md` - IS-IS Generic Cryptographic Authentication (HMAC-SHA) summary
- YANG auth leaves: authored in `internal/component/isis/yang/ze-isis-conf.yang`, coordinated with spec-isis-4 (isis-4 owns the module; this spec contributes the auth subtree)

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
   - Tests: `TestISISAuthKeyStore`, `TestISISAuthReject` (initially failing)
   - Files: `internal/component/isis/auth_keystore.go`, `internal/component/isis/packet/auth_verify.go` (stubs), RX/TX hook points in isis-5/isis-6 paths
   - Verify: hooks are reachable from config and PDU paths; tests fail because verify/sign are stubs
2. **Phase: Verify/sign codec (cleartext)** -- TLV 10 type 1 over the existing codec, TLV-first enforcement
   - Tests: `TestISISAuthSignVerifyCleartext`, `TestISISAuthTLVFirstOnEncode`, `TestISISAuthTLVNotFirstRejected`
   - Files: `auth_verify.go`
   - Verify: round-trip sign/verify; TLV-first enforced on encode and validated on decode
3. **Phase: HMAC-MD5 (RFC 5304, type 54)** -- `crypto/hmac` + `crypto/md5`; IIH/CSNP/PSNP digest over zeroed Authentication Value only; constant-time compare
   - Tests: `TestISISAuthSignVerifyHMACMD5`, `TestISISAuthConstantTimeCompare`, `TestISISAuthWrongKeyRejected`
   - Files: `auth_verify.go`
   - Verify: per-PDU-class sign/verify; type byte 54; wrong key rejected; `hmac.Equal` used
4. **Phase: Generic crypto HMAC-SHA (RFC 5310, type 3)** -- HMAC-SHA-256 first, algorithm-agile; Key-ID handling
   - Tests: `TestISISAuthSignVerifyHMACSHA256`
   - Files: `auth_verify.go`
   - Verify: SHA-256 sign/verify; type byte 3 (RFC 5310); Key-ID round-trips
5. **Phase: LSP checksum/auth interaction + authenticated purge** -- LSP digest over zeroed Authentication Value, Checksum, and Remaining Lifetime (RFC 5304 sec 2); digest before checksum on send; checksum then digest on receive; signed purges and purge acceptance rules
   - Tests: `TestISISAuthLSPChecksumAfterSign`, `TestISISAuthPurge`
   - Files: `auth_verify.go`, LSP encode/decode path (isis-6)
   - Verify: signed LSP round-trips with valid checksum and valid digest; signed purge round-trips; unauthenticated purge and purge with non-auth TLVs rejected
6. **Phase: Key store + rotation + secrets** -- per-level/per-interface chains, active-key selection, `$9$` decode, overlap window
   - Tests: `TestISISAuthRotation`, `TestISISAuthSecretEncoding`
   - Files: `auth_keystore.go`, config resolve, YANG auth leaves
   - Verify: rotation accepts both keys; keys stored `$9$`-encoded; plaintext not retained
7. **Phase: Wire hooks live** -- IIH (per-interface) and LSP/CSNP/PSNP (per-level) verify on RX, sign on TX; counter increment on reject
   - Tests: `TestISISAuthReject` passes (positive and negative); `TestISISAuthSignLSP`
   - Files: isis-5 IIH path, isis-6 LSP/SNP path
   - Verify: wrong key fails adjacency; correct key forms it; rejects increment the counter
8. **Functional test** -- `test/isis/isis-auth.ci`
9. **RFC refs** -- `// RFC 5304 Section X.Y` / `// RFC 5310 Section X.Y` comments at enforcing code
10. **Full verification** -- `make ze-verify`
11. **Complete spec** -- learned summary, delete spec

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-14 has implementation with file:line |
| Feature completeness | All four PDU classes (IIH/LSP/CSNP/PSNP) and all three auth types sign and verify; no class left unauthenticated when a chain is configured |
| Correctness | TLV 10 first; HMAC-MD5 = type 54, generic crypto = type 3; LSP digest over zeroed Authentication Value, Checksum, and Remaining Lifetime (RFC 5304 sec 2); IIH/CSNP/PSNP zero only the Authentication Value; LSP Fletcher checksum after signing; authenticated purges enforced; matches RFC 5304/5310 |
| Naming | YANG kebab-case auth leaves; auth type enum values match RFC names; `$9$` secret leaf like PPPoE/WireGuard |
| Data flow | RX bytes -> verify -> dispatch; TX encode -> sign -> checksum -> wire; no bypass of verify when a chain is configured |
| CLI grammar | Any auth state surfaced through existing `show isis` commands follows action-before-identifier |
| YANG validation | Auth type `enumeration`; key-id `range`; secret `length`/pattern; no bare `type string` |
| Rule: security | Constant-time compare on every verify; no decoded key in logs or snapshots; downgrade (missing TLV 10 under configured auth) rejected |
| Rule: no-duplication | Reuse isis-2 TLV 10 codec and isis-2 checksum; no second auth encoder or checksum routine |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Verify/sign helpers | `ls internal/component/isis/packet/auth_verify.go` |
| Key store | `ls internal/component/isis/auth_keystore.go` |
| Functional test | `ls test/isis/isis-auth.ci` |
| RFC summaries | `ls rfc/short/rfc5304.md rfc/short/rfc5310.md` |
| Constant-time compare | `grep -n 'hmac.Equal\|subtle.ConstantTimeCompare' internal/component/isis/packet/auth_verify.go` |
| TLV-first enforcement | `TestISISAuthTLVFirstOnEncode`, `TestISISAuthTLVNotFirstRejected` pass |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | TLV 10 length and position validated before reading the digest; digest length matches the algorithm; no slice past the PDU bound |
| Constant-time compare | Every digest comparison uses `hmac.Equal` / `subtle.ConstantTimeCompare`; no `bytes.Equal`/`==` on digests |
| Key handling | Keys stored `$9$`-encoded at rest; decoded only in memory; never logged, never in `show`/web/snapshots; zeroed when feasible after a config change drops a key |
| Downgrade resistance | Under configured auth, a PDU with no TLV 10 or with TLV 10 not first is rejected (no silent unauthenticated acceptance) |
| Authenticated purges | Per RFC 5304 sec 2, under configured auth an unauthenticated purge (zero Remaining Lifetime) MUST be rejected and a purge carrying any TLV other than TLV 10 MUST be rejected, so an IS without the key cannot spoof a purge by zeroing the lifetime of a captured LSP and re-flooding it; the originator regenerates valid authentication when it strips the LSP body and purges (MUST remove the body and add the authentication TLV) |
| Replay/spoofing | Auth only proves the sender holds the key; note that IS-IS auth does not prevent replay, and sequence/lifetime sanity (isis-6) still applies |
| Resource use | Verify cost bounded by the number of valid keys in a chain; cap chain size to avoid CPU amplification on a flood of forged PDUs |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read RFC 5304/5310 summary / Current Behavior |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Interop mismatch | Capture with tcpdump, compare digest/type byte to FRR, fix codec |
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
- RFC authentication type codes are HMAC-MD5 = 54 (RFC 5304) and generic crypto = 3 (RFC 5310); the research guide sec 7 has them swapped. Pin them in a constant test and leave an RFC comment at the codec.
- TLV 10 codec already exists from isis-2; the real work is a crypto backend plus exactly two hook points (verify on RX dispatch, sign on TX encode) shared by all four PDU classes.

## Core Insight
IS-IS authentication in Ze is "an HMAC backend plus a key store wired into two
shared hook points." The wire TLV already exists; correctness concentrates in
three places: TLV-10-first ordering, the LSP digest computed with the
Authentication Value, Checksum, and Remaining Lifetime all zeroed (RFC 5304 sec
2) plus checksum-after-sign interaction and authenticated-purge handling, and
constant-time comparison.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Verify/sign helpers in `packet`, key store in the component | Put everything in the component | `packet` stays runtime-free and testable on raw bytes; key store needs config/runtime state |
| Single active signing key, all unexpired keys accepted on receive | Per-key-id selection on receive | Simplest hitless rotation; matches common key-chain behavior (guide sec 7) |
| `$9$` reversible encoding for keys | Plaintext leaf, or one-way hash | Consistent with PPPoE/WireGuard; keys must be reversible to derive HMAC, but never shown plaintext |
| RFC-correct type codes (54 / 3), comment the guide error | Follow the guide verbatim | Interop with FRR/Cisco/Juniper requires the wire-correct type byte |

## Known Limitations
- IS-IS authentication proves key possession, not freshness; it does not prevent replay (sequence/lifetime sanity in isis-6 still applies)
- Cleartext (type 1) is supported for sanity checks only and is not a security mechanism
- Time-based per-key validity windows are simplified to an active key plus accepted unexpired keys; full calendar-scheduled key lifetimes are a future enhancement
- `ze_isis_auth_failures_total` is OWNED and registered by this spec (per the umbrella canonical table) and incremented at the rejection site here; spec-isis-13 only scrapes/surfaces it

## RFC Documentation

Add `// RFC 5304 Section X.Y: "<quoted requirement>"` and `// RFC 5310 Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: TLV-10-first ordering (RFC 5304 sec 1); LSP digest computed with the Authentication Value, Checksum, and Remaining Lifetime all zeroed (RFC 5304 sec 2); IIH/CSNP/PSNP zero only the Authentication Value; LSP checksum-after-sign interaction; authenticated-purge rules (RFC 5304 sec 2: MUST remove the body and add the authentication TLV, MUST NOT accept unauthenticated purges or purges with non-auth TLVs); authentication type codes (cleartext 1, HMAC-MD5 54, generic crypto 3); the engine signs the fully-constructed PDU including the Padding TLV 8 and the transport does not alter bytes after signing (umbrella "Final PDU bytes" contract; RFC 5304 sec 2 signs padded Hellos); constant-time comparison requirement.

## Implementation Summary

### What Was Implemented
- [To be filled]

### Bugs Found/Fixed
- [To be filled]

### Documentation Updates
- [To be filled]

### Deviations from Plan
- [To be filled]

## Implementation Audit

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
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Wrong/no key rejected | functional test | `test/isis/isis-auth.ci` |
| Correct key forms adjacency and accepts LSPs | functional test | `test/isis/isis-auth.ci`, `TestISISAuthReject` |
| Hitless key rotation | functional test | `TestISISAuthRotation`, `test/isis/isis-auth.ci` |
| FRR auth interop | interop test | `test/interop/scenarios/isis-auth-frr` (run under isis-13) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [To be filled]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

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
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/component/isis/`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered with source evidence
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added (RFC 5304 / RFC 5310)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (isis-auth-frr under isis-13)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-isis-10-auth.md`
- [ ] Summary included in commit
