# Spec: ipsec-1 -- PKI Certificate Store

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/7 |
| Updated | 2026-05-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `spec-ipsec-0-umbrella.md` -- umbrella design decisions
4. `internal/component/config/secret/secret.go` -- $9$ encoding pattern
5. `internal/component/iface/wireguard.go` -- WireguardSpec pattern (sensitive keys)
6. `internal/component/l2tp/register.go` -- component registration pattern
7. `internal/component/l2tp/schema/` -- YANG schema embed + register pattern

## Task

Build a PKI certificate store component that parses certificates and private keys
from Ze's YANG config tree and holds them in memory for consumers (IPsec, TLS,
managed device auth). The store is shared infrastructure, not IPsec-specific.

The reference config (`../home.conf`) defines a `pki {}` top-level container with
two entry types: `ca <name>` (CA certificates) and `certificate <name>` (device
certificates with private keys). CA certificates contain a single **raw base64-encoded
DER** X.509 certificate (no PEM headers; this is the VyOS convention). Device
certificates contain a base64-DER certificate plus a `$9$`-encoded private key
under `private { key ... }`.

The certificates are issued by the SurfProtect CA infrastructure
(`certauth` package at `git.exa.net.uk/surfprotect/lachesis/lachesis/certauth`).
That package handles PEM file I/O, multi-format key detection (PKCS8, PKCS1, SEC1),
certificate pool creation, and ECDSA/RSA sign/verify.
Ze will use `x509.ParseCertificate` for DER, `x509.CertPool` for chain validation,
`x509.ParsePKCS8PrivateKey` with fallback to `ParseECPrivateKey` and
`ParsePKCS1PrivateKey` for key type detection.

Ze currently has no PKI handling at all. The `$9$` sensitive encoding infrastructure
exists (`internal/component/config/secret/`) and is used by wireguard keys, PPPoE
passwords, and API tokens. The PKI store extends this pattern to X.509 certificates
and associated private keys.

Consumers will access the store through a query interface: lookup by name, list all,
get CA pool for chain validation. The IPsec component (spec ipsec-4) will use this
to **export PEM files** for strongSwan's swanctl.conf (strongSwan expects PEM on disk).
The web/TLS component could use this for serving HTTPS with operator-provided
certificates.

### Certificate Format Details

The YANG `certificate` leaf holds a **base64-encoded DER** bytestring (not PEM).
The parsing pipeline is: base64 decode -> raw DER bytes -> `x509.ParseCertificate`.
This differs from PEM, which wraps DER in `-----BEGIN CERTIFICATE-----` headers.

The private key leaf is `$9$`-encoded (ze:sensitive auto-decodes). The decoded value
is also base64-encoded DER of the raw key. The key type must be detected by trying
parsers in order: `x509.ParsePKCS8PrivateKey` (generic), then `x509.ParseECPrivateKey`
(SEC1), then `x509.ParsePKCS1PrivateKey` (RSA). 

For strongSwan, the store must **export to PEM files** on disk:
- `/tmp/ze-ipsec/ca-<name>.pem` (CA cert)
- `/tmp/ze-ipsec/cert-<name>.pem` (device cert)
- `/tmp/ze-ipsec/key-<name>.pem` (private key, 0600 permissions)

These files are written before charon starts and cleaned up on shutdown.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- component lifecycle, registration pattern
  -> Constraint: PKI component follows init()-based registration via register.go
  -> Constraint: component registers YANG schema via schema/register.go init()
- [ ] `internal/component/config/secret/secret.go` -- $9$ sensitive encoding
  -> Constraint: private keys use ze:sensitive YANG extension, auto-decoded on load
  -> Decision: store holds decoded private key bytes, never exposes $9$ form to consumers
- [ ] `internal/component/iface/wireguard.go` -- WireguardSpec with WireguardKey alias
  -> Decision: PKI store wraps crypto/x509.Certificate behind a CertificateEntry type
- [ ] `internal/component/l2tp/register.go` -- component registration pattern
  -> Constraint: PKI register.go blank-imports schema package to trigger schema init()
- [ ] `internal/component/l2tp/schema/` -- YANG schema embed + register pattern
  -> Constraint: embed.go exports the YANG string, register.go calls yang.RegisterModule
- [ ] `internal/component/iface/config.go` -- config tree-walker pattern (parseWireguardEntry)
  -> Decision: PKI config parser follows same map[string]any tree-walking pattern

**Key insights:**
- $9$ encoding is automatic via ze:sensitive; the config parser delivers plaintext to the component
- Components discover each other through registries, not direct imports
- YANG schemas are embedded Go strings registered at init() time
- Config reload must update the in-memory store atomically (swap pointer, not mutate)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/secret/secret.go` -- Encode/Decode/IsEncoded for $9$ obfuscation. JunOS-compatible. Used for WireGuard keys, PPPoE passwords, API tokens
  -> Constraint: private-key leaf must use same ze:sensitive mechanism
- [ ] `internal/component/iface/wireguard.go` -- WireguardKey = wgtypes.Key (32-byte alias). WireguardSpec holds parsed key material. WireguardPeerSpec.HasPresharedKey for optional field
  -> Decision: PKI uses crypto/x509.Certificate and crypto.PrivateKey (interface), not byte aliases
- [ ] `internal/component/iface/config.go` -- parseWireguardEntry reads private-key from tree, base64 decodes. applyWireguards diffs old vs new spec for reconciliation
  -> Decision: PKI config parser decodes base64 PEM from YANG leaf, parses DER
- [ ] `internal/component/l2tp/config.go` -- ExtractParameters reads from config tree, returns typed struct. Validation inline
  -> Constraint: PKI follows same pattern: extract from tree, validate, return typed struct
- [ ] `internal/component/l2tp/schema/ze-l2tp-conf.yang` -- module with namespace, containers, lists, leaves
  -> Constraint: PKI YANG module follows same structure
- [ ] `internal/component/l2tp/schema/embed.go` -- go:embed directive, exported string var
  -> Constraint: PKI schema embed follows same pattern
- [ ] `internal/component/l2tp/schema/register.go` -- init() calls yang.RegisterModule
  -> Constraint: PKI schema register follows same pattern

**Behavior to preserve:**
- $9$ encoding/decoding mechanism unchanged
- Config transaction and rollback mechanics unchanged
- Existing component registration pattern unchanged
- No changes to any existing component

**Behavior to change:**
- New `pki {}` top-level YANG container (new YANG module)
- New PKI component at `internal/component/pki/`
- New `show pki certificates` and `show pki certificate <name>` CLI commands
- Hub startup wires PKI component

## Data Flow (MANDATORY)

### Entry Point
- Config load/reload: YANG tree contains `pki {}` block with CA and certificate entries
- CLI: `show pki certificates` and `show pki certificate <name>` query the store

### Transformation Path
1. YANG parser reads `pki {}` container from config file
2. Config tree contains `pki/ca/<name>/certificate` (base64-encoded DER) and `pki/certificate/<name>/certificate` (base64-encoded DER) + `pki/certificate/<name>/private/key` ($9$-encoded, auto-decoded to base64-encoded DER)
3. PKI config parser walks the tree: for each entry, base64-decodes to raw DER bytes, then `x509.ParseCertificate` for certs and multi-format key detection for private keys
4. Parser validates: DER well-formed, certificate parseable, private key matches certificate public key, chain validates against known CAs
5. Parser returns a PKIConfig struct containing CACerts and Certificates maps
6. PKI store atomically swaps its contents (pointer swap, not mutation)
7. Consumers (IPsec, TLS) query the store by name or request the CA pool
8. IPsec consumer calls ExportPEM to write PEM files to /tmp/ze-ipsec/ for strongSwan

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree to PKI parser | Tree-walker reads map[string]any leaves | [ ] |
| $9$ encoding to plaintext | Automatic via ze:sensitive YANG extension | [ ] |
| PKI parser to store | Atomic pointer swap of PKIConfig | [ ] |
| Store to consumers (IPsec, TLS) | Query methods: GetCA, GetCertificate, CAPool, IntermediatePool, CertCN | [ ] |
| Store to CLI | show pki commands query store, format as JSON | [ ] |

### Integration Points
- `config/yang` -- YANG schema registration via yang.RegisterModule
- `config/secret` -- $9$ auto-decoding of private-key leaves
- `cmd/ze/hub/main.go` -- blank-import of PKI package triggers registration
- CLI show command tree -- new `show pki` subtree
- Future: IPsec component (ipsec-4) queries store for cert/key paths
- Future: web/TLS component queries store for HTTPS serving cert

### Architectural Verification
- [ ] No bypassed layers (PKI uses config tree, not raw file reads)
- [ ] No unintended coupling (store is query-only; consumers pull, store does not push)
- [ ] No duplicated functionality (reuses $9$ encoding, YANG registration, tree-walker)
- [ ] Zero-copy preserved where applicable (certificates are parsed once, referenced by pointer)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config load with `pki { ca test-ca { certificate ... } }` | -> | PKI parser returns CACerts with parsed X.509 | `test/parse/pki-ca-certificate.ci` |
| Config load with `pki { certificate test-dev { certificate ... private { key ... } } }` | -> | PKI parser returns Certificates with cert + private key | `test/parse/pki-device-certificate.ci` |
| `show pki certificates` CLI command | -> | Store query returns list, formatted as JSON | `test/plugin/pki-show-certificates.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config has `pki { ca test-ca { certificate <base64-DER> } }` | CA certificate base64-decoded, DER-parsed via x509.ParseCertificate, stored in memory with subject, issuer, expiry, key type |
| AC-2 | Config has `pki { certificate dev-1 { certificate <base64-DER> private { key <$9$> } } }` | Device cert parsed, chain validated against known CAs, private key decoded |
| AC-3 | Private key leaf contains $9$-encoded value | Key auto-decoded via ze:sensitive, base64-decoded to DER, parsed via PKCS8/SEC1/PKCS1 detection |
| AC-4 | CA cert and device cert loaded | Chain validation succeeds (device cert signed by CA). Expiry checked against current time. Key usage checked if present |
| AC-5 | `show pki certificates` | Table listing all certificates: name, type (ca/device), subject CN, issuer CN, expiry date, key algorithm, valid (yes/no) |
| AC-6 | `show pki certificate <name>` | Full details: subject, issuer, serial, not-before, not-after, key algorithm, key size, SANs, key usage, chain status |
| AC-7 | Config has expired certificate or cert not signed by any loaded CA | Config load rejects with descriptive error naming the certificate and the reason |
| AC-8 | Config reload changes a certificate | Store atomically updated. New consumers see new cert. No daemon restart |
| AC-9 | Consumer calls ExportPEM for a device certificate | PEM files written to /tmp/ze-ipsec/: ca-<name>.pem (cert), cert-<name>.pem (cert), key-<name>.pem (private key with 0600 permissions). Correct PEM headers (BEGIN CERTIFICATE, BEGIN PRIVATE KEY) |
| AC-10 | Store queried for certificate issued by SurfProtect CPE Management CA | ECDSA P-256 certificate parsed correctly, subject CN matches device name, issuer CN matches CA |
| AC-11 | Config has `pki { certificate dev-1 { certificate <DER> intermediate <DER> private { key ... } } }` | Intermediate certificate(s) parsed, stored alongside device cert. Chain validation uses intermediates pool (KeyVault three-tier pattern: root + intermediate + device) |
| AC-12 | ExportPEM called for cert with intermediates | Intermediate certs written alongside device cert for strongSwan (separate PEM file or appended to cert chain file) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseCACertificate` | `internal/component/pki/config_test.go` | Base64-DER decoded, x509.ParseCertificate succeeds, fields extracted | |
| `TestParseDeviceCertificate` | `internal/component/pki/config_test.go` | Cert (base64-DER) + private key parsed, key matches cert public key | |
| `TestParsePrivateKeyECDSA` | `internal/component/pki/config_test.go` | ECDSA private key parsed from decoded $9$ base64-DER via PKCS8/SEC1 detection | |
| `TestParsePrivateKeyRSA` | `internal/component/pki/config_test.go` | RSA private key parsed (for completeness, even if home.conf uses ECDSA) | |
| `TestParsePrivateKeyEd25519` | `internal/component/pki/config_test.go` | Ed25519 private key parsed | |
| `TestChainValidation` | `internal/component/pki/store_test.go` | Device cert validates against CA pool | |
| `TestChainValidationFails` | `internal/component/pki/store_test.go` | Device cert signed by unknown CA returns error | |
| `TestExpiredCertRejected` | `internal/component/pki/store_test.go` | Expired cert produces descriptive error | |
| `TestKeyMismatch` | `internal/component/pki/store_test.go` | Private key not matching cert public key returns error | |
| `TestStoreAtomicSwap` | `internal/component/pki/store_test.go` | Reload swaps store contents; old queries see old data, new queries see new | |
| `TestStoreGetCA` | `internal/component/pki/store_test.go` | GetCA returns named CA cert or nil | |
| `TestStoreGetCertificate` | `internal/component/pki/store_test.go` | GetCertificate returns named device cert or nil | |
| `TestStoreCAPool` | `internal/component/pki/store_test.go` | CAPool returns x509.CertPool containing all loaded CAs | |
| `TestShowPKICertificates` | `internal/component/pki/show_test.go` | JSON output includes all loaded certs with expected fields | |
| `TestShowPKICertificateByName` | `internal/component/pki/show_test.go` | JSON output for named cert includes full details | |
| `TestExportPEM` | `internal/component/pki/store_test.go` | ExportPEM writes correct PEM files to target directory, key file has 0600 permissions | |
| `TestExportPEMCleanup` | `internal/component/pki/store_test.go` | CleanupPEM removes exported files | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Certificate PEM length | 1 to ~64KB | ~64KB (typical chain) | 0 (empty) | N/A (no hard upper) |
| CA list count | 0 to unbounded | N/A | N/A | N/A (no hard limit) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pki-ca-certificate` | `test/parse/pki-ca-certificate.ci` | Config with CA cert accepted, parseable | |
| `pki-device-certificate` | `test/parse/pki-device-certificate.ci` | Config with device cert + key accepted | |
| `pki-show-certificates` | `test/plugin/pki-show-certificates.ci` | `show pki certificates` returns cert list | |
| `pki-invalid-expired` | `test/parse/pki-invalid-expired.ci` | Expired cert rejected at config load | |

## Files to Modify
- `cmd/ze/hub/main.go` -- blank-import `internal/component/pki` to wire registration
- `docs/features.md` -- add PKI row to feature table (if a PKI/security section exists)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config container) | [Yes] | `internal/component/pki/schema/ze-pki-conf.yang` |
| CLI commands/flags | [Yes] | `show pki certificates`, `show pki certificate <name>` |
| Editor autocomplete | [Yes] | YANG-driven (automatic if YANG schema registered) |
| Functional test for new RPC/API | [Yes] | `test/plugin/pki-show-certificates.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [Yes] | `docs/features.md` -- PKI certificate management |
| 2 | Config syntax changed? | [Yes] | `docs/guide/configuration.md` -- pki {} container |
| 3 | CLI command added/changed? | [Yes] | `docs/guide/command-reference.md` -- show pki |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [Yes] | `docs/guide/pki.md` -- PKI guide |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A (X.509 is ITU, not IETF RFC for parsing) |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create
- `internal/component/pki/config.go` -- config parser: walks pki tree, parses PEM, decodes keys
- `internal/component/pki/store.go` -- in-memory certificate store: atomic swap, query methods (GetCA, GetCertificate, CAPool, List)
- `internal/component/pki/types.go` -- CACertEntry, CertificateEntry, PKIConfig types
- `internal/component/pki/show.go` -- show pki CLI command handlers (JSON output)
- `internal/component/pki/register.go` -- component registration (blank-imports schema)
- `internal/component/pki/schema/ze-pki-conf.yang` -- YANG module for pki {} container
- `internal/component/pki/schema/ze-pki-api.yang` -- YANG module for show pki commands
- `internal/component/pki/schema/embed.go` -- go:embed directives for YANG files
- `internal/component/pki/schema/register.go` -- init() calls yang.RegisterModule
- `internal/component/pki/config_test.go` -- unit tests for parsing
- `internal/component/pki/store_test.go` -- unit tests for store + validation
- `internal/component/pki/show_test.go` -- unit tests for show output
- `test/parse/pki-ca-certificate.ci` -- parse test for CA cert config
- `test/parse/pki-device-certificate.ci` -- parse test for device cert config
- `test/parse/pki-invalid-expired.ci` -- parse test for expired cert rejection
- `test/plugin/pki-show-certificates.ci` -- functional test for show pki

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- YANG schema, registration, skeleton parser
   - Tests: wiring tests from Wiring Test table (initially failing)
   - Files: `schema/ze-pki-conf.yang`, `schema/embed.go`, `schema/register.go`, `register.go`, `config.go` (skeleton)
   - Verify: YANG schema loads, config block accepted, parser called but returns stub

2. **Phase: Certificate Parsing** -- PEM decode, x509 parse, private key decode
   - Tests: `TestParseCACertificate`, `TestParseDeviceCertificate`, `TestParsePrivateKeyECDSA`, `TestParsePrivateKeyRSA`, `TestParsePrivateKeyEd25519`, `TestKeyMismatch`
   - Files: `config.go`, `types.go`
   - Verify: certificates and keys parsed from config tree, validation errors on bad input

3. **Phase: Store** -- in-memory store, atomic swap, query methods, chain validation
   - Tests: `TestChainValidation`, `TestChainValidationFails`, `TestExpiredCertRejected`, `TestStoreAtomicSwap`, `TestStoreGetCA`, `TestStoreGetCertificate`, `TestStoreCAPool`
   - Files: `store.go`
   - Verify: store holds parsed certs, validates chains, swaps atomically on reload

4. **Phase: CLI** -- show pki commands, JSON output
   - Tests: `TestShowPKICertificates`, `TestShowPKICertificateByName`
   - Files: `show.go`, `schema/ze-pki-api.yang`
   - Verify: show commands return formatted cert info

5. **Functional tests** -- .ci tests for end-to-end behavior
6. **Full verification** -- `make ze-verify`
7. **Complete spec** -- audit, learned summary, spec closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N (1-8) has implementation with file:line |
| Correctness | PEM parsing handles single cert and cert chain. Private key type detection (ECDSA, RSA, Ed25519) is correct |
| Naming | YANG uses kebab-case (`ca-certificate`, `private-key`). Go types use CamelCase. JSON keys use kebab-case |
| Data flow | Config tree to parser to store. Store is read-only after swap. No mutation |
| Rule: no-layering | No duplicate certificate parsing anywhere else in the codebase |
| Rule: exact-or-reject | Invalid PEM, invalid cert, expired cert, key mismatch all rejected with descriptive error |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| YANG schema loads | `ze config validate` with pki block does not error |
| CA cert parsing | `TestParseCACertificate` passes |
| Device cert parsing | `TestParseDeviceCertificate` passes |
| Chain validation | `TestChainValidation` and `TestChainValidationFails` pass |
| Expired cert rejected | `TestExpiredCertRejected` passes |
| Show CLI works | `test/plugin/pki-show-certificates.ci` passes |
| Store query API | `TestStoreGetCA`, `TestStoreGetCertificate`, `TestStoreCAPool` pass |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | PEM data from config could be malformed: verify x509.ParseCertificate handles it safely |
| Private key exposure | Private keys must never appear in logs, error messages, JSON output, or bus events. Verify slog fields are redacted |
| Memory handling | Private key bytes should not linger in unused buffers. Consider zeroing after parse (best effort in Go) |
| Certificate validation | Verify expiry check uses time.Now() not a cached time. Verify chain validation uses the correct CA pool |
| $9$ passthrough | Verify private-key leaf is ze:sensitive and the store never re-encodes or exposes the $9$ form |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior, RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural, DESIGN phase |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete, every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled, 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks, no failures)

### Quality Gates (SHOULD pass, defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling
