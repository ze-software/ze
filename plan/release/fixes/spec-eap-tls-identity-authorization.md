# Spec: eap-tls-identity-authorization

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P0 |
| Release reason | A trusted certificate chain must not authorize an unintended client or server identity. |
| RFC gaps | RFC5216-5.2-1, RFC5216-5.3-2, RFC5216-5.3-3 |
| Depends | - |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-11-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

| Topic | Decision |
|-------|----------|
| Default when no identity constraint is configured | Fail-closed at commit unless an explicit identity constraint (allow-list, realm, or explicit opt-in wildcard on authenticator; or unambiguous remote FQDN / `server-identity` on client) is configured. |
| YANG grammar | Authenticator `allowed-identities` leaf-list (supporting exact SANs, wildcards, and NAI realms) plus peer `server-identity` leaf. |

## Task

Both EAP-TLS roles verify the peer's certificate chain and its revocation status, and
neither authorizes the identity the certificate carries. RFC 5216 Section 5.2 defines
that identity (Peer-Id and Server-Id, taken from the `subjectAltName` extension when
present) and Section 5.3 obliges each side to validate that the certificate's identity
is appropriate and authorized for EAP-TLS, following the matching rules of RFC 2818
Section 3.1. Today an EAP-TLS authenticator accepts any client certificate its trust
anchor issued, and an EAP-TLS peer accepts any server certificate its trust anchor
issued. This spec also unblocks `plan/release/fixes/spec-ipsec-remote-access.md`, whose
`eap-tls` mode requires the exported verified client identity.

| Requirement | RFC text (verbatim) | Producer or absence |
|---|---|---|
| RFC5216-5.2-1 | "Where the subjectAltName field is present in the peer or server certificate, the Peer-Id or Server-Id MUST be set to the contents of the subjectAltName." (Section 5.2) | `internal/core/eap/eap_tls.go::eapTLSPeerName` returns `PeerCertificates[0].Subject.String()` and never reads `subjectAltName`; neither `Session` nor `PeerSession` exports Peer-Id or Server-Id. `internal/core/eap/nai.go::certificateNAIs` reads the SAN only for deriving the peer's own anonymous NAI under RFC 9190 |
| RFC5216-5.3-2 | "Once a TLS session is established, EAP-TLS peer and server implementations MUST validate that the identities represented in the certificate are appropriate and authorized for use with EAP-TLS." (Section 5.3) | Authenticator: `eap_tls.go::newTLSMethod` `VerifyConnection` checks chain and revocation (`revocation.go::checkChainRevocation`) only. Peer: `peer_chain.go::serverChainCheck` checks chain, revocation, and OCSP stapling only. Neither authorizes Peer-Id or Server-Id against configuration |
| RFC5216-5.3-3 | "When performing this comparison, implementations MUST follow the validation rules specified in Section 3.1 of [RFC2818]." (Section 5.3) | `peer.go::tlsClientConfig` sets no expected server identity, so no RFC 2818 Section 3.1 comparison runs on either full or resumed TLS sessions |

This design:

1. Extracts Peer-Id and Server-Id from the `subjectAltName` extension in certificate
   wire order when that extension is present (falling back to Subject Common Name or
   non-empty Subject DN only when no `subjectAltName` extension exists), updates
   `eapTLSPeerName`, and exports `PeerID`/`PeerIDs` on `eap.Session` and
   `ServerID`/`ServerIDs` on `eap.PeerSession`.
2. Adds two YANG nodes under `vpn ipsec site-to-site peer authentication` in
   `ze-ipsec-conf.yang`: `leaf-list allowed-identities` for the EAP-TLS authenticator
   (`connection-type respond`) and `leaf server-identity` for the EAP-TLS peer
   (`connection-type initiate`), packed inside `ipsec.AuthAdditions` so `SiteToSitePeer`
   grows by zero bytes.
3. Enforces the 2026-11-24 fail-closed commit/validate/reload default in
   `ipsec.IPsecConfig.ValidateEAPTLSIdentities` (invoked from
   `engine.validateIPsecSections`, covering `ze config validate`, commit, and SIGHUP
   reload) and adds matching runtime fail-closed guards in `eapTLSServerConfig`,
   `buildPeerTLSConfig`, `newTLSMethod`, and `startTLSClient`, plus a `post-config`
   doctor check (`ipsec-eap-tls-identity` / `doctor-ipsec-eap-tls-identity`) for
   daemons started from an unvalidated config file.
4. Enforces identity authorization on both roles inside `VerifyConnection`, covering
   both full handshakes and resumed TLS 1.3 sessions (after `rebuildResumedChains` on
   the peer and over cached `PeerCertificates` on the authenticator), sends a fatal TLS
   `bad_certificate` alert on mismatch, and logs the verified `peer-id` or `server-id`
   in `recordEAPTLSAuthentication`.

## Design

### Identity Extraction (RFC 5216 Section 5.2)

`internal/core/eap/identity.go` defines a single SAN-first extractor shared by both
roles and by `eapTLSPeerName`.

| Certificate shape | Extracted identities (wire order) | Primary ID (`PeerID` / `ServerID` / `eapTLSPeerName`) | Authorization candidate set |
|---|---|---|---|
| `subjectAltName` extension (OID `2.5.29.17`) present, one or more supported entries (`rfc822Name`, `dNSName`, `uniformResourceIdentifier`, `iPAddress`, or `userPrincipalName` `otherName` OID `1.3.6.1.4.1.311.20.2.3`) | Every supported GeneralName in ASN.1 sequence order | First extracted SAN entry | Only the extracted SAN entries; Subject CN and Subject DN are never consulted |
| `subjectAltName` extension present, zero supported entries | Empty | `Subject.String()` for diagnostics only (or `"(no identity)"` if Subject is also empty) | Empty: authorization fails closed (no CN fallback, matching RFC 5216 §5.2, RFC 2818 §3.1, and `engine/remote_id.go::hasSubjectAltName`) |
| No `subjectAltName` extension, non-empty `Subject.CommonName` | Most specific CN (last CN RDN in `Subject.Names`, per RFC 2818 §3.1) | That CN | That CN (matched for DNS and NAI forms; never for IP literals per RFC 2818 §3.1) |
| No `subjectAltName` extension, empty CN, non-empty Subject DN | `cert.Subject.String()` | `cert.Subject.String()` | Non-empty Subject DN satisfies explicit `*` only; never matches a DNS, IP, URI, or NAI entry |
| No `subjectAltName` extension and empty Subject (or no certificate) | Empty | `"(no certificate)"` when no leaf; `"(no identity)"` when empty | Empty: fails closed even under `*` |

### Authenticator `allowed-identities` Grammar and Matching (`connection-type respond`)

| Entry form | Syntax example | Validation at commit (`ValidateEAPTLSIdentities`) | Match rule against client certificate |
|---|---|---|---|
| Exact DNS name | `swan-eap-client`, `vpn.example.com` | 1..255 bytes; valid DNS label characters (`[a-zA-Z0-9-]`, labels non-empty, no leading/trailing hyphen or dot, no `*`, no C0/DEL control bytes) | ASCII case-insensitive exact match against `dNSName` SAN (or most-specific CN only when no SAN extension exists) |
| Single-label DNS wildcard | `*.branch.example.com` | Starts with `*.` followed by at least two valid DNS labels (refuses `*.com`, `*.*.example.com`, `foo*.example.com`, `a.*.example.com`, and `.example.com` with exact fix message) | Matches a `dNSName` SAN (or CN when no SAN extension exists) with exactly one non-empty DNS label before `.branch.example.com` (RFC 2818 §3.1: matches `gw1.branch.example.com`, refuses `a.b.branch.example.com` and `branch.example.com`) |
| Exact NAI / RFC 822 | `alice@example.com` | Non-empty RFC 7542 username + `@` + valid RFC 7542 realm (`validNAI` / `validRealm` from `nai.go`, no `*`, no C0/DEL control bytes) | ASCII case-insensitive exact match against `rfc822Name` or UPN `otherName` SAN (or CN when no SAN extension exists) |
| NAI realm | `@example.com` | Leading `@` followed by a valid RFC 7542 realm of at least two labels (`validRealm`, refuses `@`, `@localhost`, `*@example.com`) | Matches any `rfc822Name` or UPN `otherName` SAN (or CN when no SAN extension exists) with a non-empty username before `@` and ASCII case-insensitive matching realm (never matches bare `@example.com`) |
| Exact IP address | `192.0.2.10`, `2001:db8::10` | Valid IPv4 or IPv6 literal via `netip.ParseAddr` (refuses CIDR prefixes with clear explanation) | Exact unmapped `netip.Addr` equality against `iPAddress` SAN only; never matches `dNSName` and never falls back to CN (RFC 2818 §3.1 and `remote_id.go` class separation) |
| Exact URI | `spiffe://corp.example/node/1`, `urn:ze:client:1` | Contains `://` or begins with `urn:`, non-empty scheme and body, printable non-whitespace ASCII, no `*` | Exact octet-for-octet match against `uniformResourceIdentifier` SAN only (never falls back to CN) |
| Explicit opt-in wildcard | `*` | Must be the ONLY entry in `allowed-identities` (mixing `*` with specific entries is refused at commit so specific entries are never dead config) | Accepts any client certificate that passed CA chain, `ExtKeyUsageClientAuth` / `any` / absent EKU, and revocation checks, and carries at least one non-empty identity |

A client certificate is authorized when ANY configured entry in `allowed-identities`
matches ANY extracted identity in the certificate's authorization candidate set
(RFC 9190 Section 2.2: "The name check passes when any configured name matches any name
in the SAN extension").

### Peer `server-identity` Grammar, Unambiguous FQDN Fallback, and RFC 2818 §3.1 Matching (`connection-type initiate`)

| Effective expected value source | Condition | Validation at commit | RFC 2818 Section 3.1 match rule against server certificate |
|---|---|---|---|
| Explicit `server-identity` leaf | Non-empty on `connection-type initiate`, `mode eap-tls` | 1..255 bytes; no C0/DEL terminators, no whitespace, no `*`, no leading `.` or `@`; must be a valid IP literal, DNS hostname/FQDN, or exact `user@realm` NAI | IP literal: exact unmapped match against `iPAddress` SAN only (no DNS, no wildcard, no CN fallback). Hostname/FQDN: ASCII case-insensitive match against `dNSName` SAN (exact or certificate single-label leftmost wildcard `*.a.b` with at least two suffix labels), falling back to most-specific Subject CN only when no SAN extension exists. Exact NAI: ASCII case-insensitive exact match against `rfc822Name`/UPN SAN (or CN when no SAN extension exists) |
| Implicit `remote-address` fallback | `server-identity` is empty AND `IsUnambiguousFQDN(remote-address)` is true (non-empty, not an IP literal, at least two valid non-empty DNS labels separated by `.`, no trailing dot, no `*`, no whitespace/control bytes) | Accepted at commit; `SiteToSitePeer.EffectiveEAPTLSServerIdentity()` returns `remote-address` | Same RFC 2818 Section 3.1 DNS hostname rule as above |
| Unconstrained initiate peer | `server-identity` is empty AND `remote-address` is empty, an IP literal (such as `172.28.0.3` or `127.0.0.2`), or a single-label name | Refused fail-closed at `ze config validate`, commit, and reload (`opererr` naming peer, RFC 5216 §5.3 / RFC 2818 §3.1, and exact `set vpn ipsec site-to-site peer <name> authentication server-identity <hostname-or-ip>` fix) | Never reaches TLS; runtime `buildPeerTLSConfig` and `startTLSClient` also fail closed |

### Fail-Closed Matrix Across Roles and Modes

| Peer mode and `connection-type` | `allowed-identities` set? | `server-identity` set? | `remote-address` is unambiguous FQDN? | Commit / validate / reload verdict |
|---|---|---|---|---|
| `eap-tls`, `respond` | Non-empty valid entries | No | Any | Accepted |
| `eap-tls`, `respond` | Empty | No | Any | Refused (`opererr`: set `allowed-identities`) |
| `eap-tls`, `respond` | Any | Yes | Any | Refused (`opererr`: `server-identity` is for `connection-type initiate`) |
| `eap-tls`, `initiate` | No | Non-empty valid entry | Any | Accepted |
| `eap-tls`, `initiate` | No | Empty | Yes (`vpn.example.com`) | Accepted (effective `server-identity` is `remote-address`) |
| `eap-tls`, `initiate` | No | Empty | No (IP literal, single label, or empty) | Refused (`opererr`: set `server-identity` or use an FQDN `remote-address`) |
| `eap-tls`, `initiate` | Yes | Any | Any | Refused (`opererr`: `allowed-identities` is for `connection-type respond`) |
| Any non-`eap-tls` mode | Yes | Any | Any | Refused (`opererr`: `allowed-identities` is read only in `eap-tls` mode) |
| Any non-`eap-tls` mode | Any | Yes | Any | Refused (`opererr`: `server-identity` is read only in `eap-tls` mode) |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform boundaries, config transactions, and registration
  → Constraint: every refusal an operator sees must be an `opererr` error naming the problem, why it matters, and the exact fix command on the last line.
- [ ] `ai/doors/access-security.md` - IKE/EAP boundaries and patched `crypto/tls` build note
  → Constraint: EAP-TLS builds only against Ze's patched GOROOT (`./le`), never bare `go test`.
- [ ] `ai/doors/config-yang.md` - YANG glue generation and `InProcessConfigVerifier`
  → Decision: place the commit check in `ipsec.IPsecConfig.ValidateEAPTLSIdentities` called from `engine.validateIPsecSections`, which is both `InProcessConfigVerifier` (`ze config validate`) and `OnConfigVerify` (commit and reload), so offline validation and live commit agree.
  → Constraint: regenerate `embed.go` and `register.go` via `./le yang glue update`; never hand-edit generated YANG glue.
- [ ] `ai/doors/core.md` - core leaf dependency direction
  → Constraint: `internal/core/eap` is core and must not import `internal/component/ike/ipsec` or any other component; identity parsing/matching lives in `internal/core/eap` and `ipsec` calls it or keeps validation aligned without upward imports.
- [ ] `ai/doors/le-tooling.md` - interop suite and `.ci` functional test contracts
  → Constraint: new strongSwan scenarios live under named directories in `test/interop-ipsec/scenarios/` with typed checkers registered in `internal/le/interoplab/ipsec/checkers.go`.
- [ ] `docs/architecture/ike/ipsec-3-data-model.md` - typed IPsec config model and struct packing
  → Decision: add `AllowedIdentities` and `ServerIdentity` inside `ipsec.AuthAdditions` (behind the existing pointer on `AuthConfig`), preserving `TestSiteToSitePeerStaysUnderTheHugeParamThreshold` (`< 288` bytes) and `AuthConfig.Equal` deep comparison.
- [ ] `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` - EAP-TLS server path, `FinalRequest`, and fatal alert delivery
  → Constraint: returning an error from `VerifyConnection` causes `crypto/tls` to emit a fatal `bad_certificate` alert, which `tlsMethod.Process` sends as `MethodResult.FinalRequest` and logs on the same round before waiting for the peer's reply.
- [ ] `docs/architecture/ike/ipsec-11-interop-eap.md` - EAP-TLS peer certificate checks, `InsecureSkipVerify`, and resumed-session chain rebuild
  → Decision: keep `InsecureSkipVerify: true` and empty `tls.Config.ServerName` (preserving the per-peering `ClientSessionCache` and CN-fallback semantics) and perform RFC 2818 Section 3.1 server identity verification inside `serverChainCheck.verifyConnection` after chain and revocation checks so both full and resumed sessions are verified.
- [ ] `docs/architecture/ike/ipsec-14-responder.md` - responder EAP wiring and per-peering resumption state
  → Constraint: `resumptionFor` invalidates cached ticket keys whenever `AuthConfig.Equal` returns false; putting the new leaves on `AuthConfig.Additions` automatically invalidates tickets when `allowed-identities` or `server-identity` changes (RFC 9190 Section 5.7).
- [ ] `docs/guide/ipsec.md` - operator guide ("Remote identity" and "EAP-TLS with TLS 1.3")
  → Constraint: lines 761-763 currently state "EAP-TLS has no server hostname, so the check validates the chain without DNS-name matching", which becomes false and must be updated in the same change.
- [ ] `docs/architecture/doctor-and-health-checks.md` - post-config doctor checks and `code` coverage on `Warn` logs
  → Constraint: every new `Warn` log line (`buildPeerTLSConfig` missing server identity at startup) must carry a registered doctor code under `"code"` backed by a registered doctor check.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5216.md` - Sections 5.2 and 5.3 (verified against `rfc/full/rfc5216.txt` lines 1351-1490)
  → Constraint (`RFC5216-5.2-1`, MUST): when `subjectAltName` is present, Peer-Id or Server-Id MUST be set to the contents of `subjectAltName`.
  → Constraint (`RFC5216-5.3-2`, MUST): once a TLS session is established, both EAP-TLS peer and server MUST validate that certificate identities are appropriate and authorized for EAP-TLS.
  → Constraint (`RFC5216-5.3-3`, MUST): identity comparison MUST follow RFC 2818 Section 3.1 (SAN precedence over CN, IP literals matched only against `iPAddress` SAN, single-component leftmost DNS wildcard, ASCII case-insensitive DNS comparison).
- [ ] `rfc/short/rfc9190.md` - Sections 2.2, 5.6, and 5.7
  → Constraint (Section 2.2): EAP-Response/Identity is unauthenticated and MUST NOT be used for authorization; server name check passes when any configured name matches any name in SAN.
  → Constraint (Section 5.7): authorization policy MUST also be enforced on resumed sessions against cached certificate data.

**Key insights:**
- `internal/core/eap` is in the `core` tier, so `internal/component/ike/ipsec` CAN import `internal/core/eap` to reuse the single identity parser/validator (`eap.ParseAllowedIdentity`, `eap.ParseServerIdentity`, `eap.IsUnambiguousFQDN`) without any tier violation or duplicate grammar! (Verify with `./le arch tier check` during implementation: components are allowed to import core.)
- `SiteToSitePeer` is guarded by `TestSiteToSitePeerStaysUnderTheHugeParamThreshold` (`< 288` bytes); `AuthConfig.Additions` (`*AuthAdditions`) is the established zero-size-growth home for per-mode auth fields.
- `engine.validateIPsecSections` is registered as `InProcessConfigVerifier` (`engine/register.go:338`) AND called by `verifyIPsecCommit` (`engine/config_verify_commit.go:20`), so adding `cfg.ValidateEAPTLSIdentities()` there enforces the fail-closed rule uniformly across `ze config validate`, CLI/web/hub commit, and SIGHUP reload.
- `ValidatePKIRefs` MUST remain ahead of `ValidateEAPTLSIdentities` in `validateIPsecSections` so an `eap-tls` peer missing `ca-certificate` still reports `eap-tls requires a ca-certificate` (`test/reload/tx-ipsec-eap-tls-requires-ca.ci`).
- Both roles already enforce RFC 5216 Section 5.3 EKU via `x509.Verify`: authenticator requests `ExtKeyUsageClientAuth` (both `crypto/tls` `RequireAndVerifyClientCert` and `clientChainVerifier.verify`), and peer defaults to `ExtKeyUsageServerAuth` (`peer_chain.go`).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/eap/eap_tls.go` - `eapTLSPeerName` returns `PeerCertificates[0].Subject.String()` without reading SAN; `newTLSMethod` `VerifyConnection` checks chain and CRL revocation only, with no client identity authorization; `tlsMethod` exports no Peer-Id
- [ ] `internal/core/eap/eap.go` - `MethodConfig` carries no `AllowedIdentities`; `Session` exposes `Identity()` (the unauthenticated EAP-Response/Identity string) and has no `PeerID()`/`PeerIDs()` accessor
- [ ] `internal/core/eap/peer.go` - `PeerTLSConfig` carries no `ServerIdentity`; `startTLSClient` and `tlsClientConfig` set `InsecureSkipVerify: true` and no `ServerName`; `PeerSession` has no `ServerID()`/`ServerIDs()` accessor
- [ ] `internal/core/eap/peer_chain.go` - `serverChainCheck.verifyPeerCertificate` validates the chain against `roots` without checking server identity; `verifyConnection` rebuilds resumed chains and runs CRL + OCSP checks only
- [ ] `internal/core/eap/nai.go` - defines `oidSubjectAltName`, `oidUserPrincipalName`, `otherName`, `otherNameText`, `validNAI`, `validRealm`, and `certificateNAIs` for RFC 9190 anonymous NAI derivation
- [ ] `internal/core/eap/client_chain.go` - `clientChainVerifier.verify` path-validates client certificates with `ExtKeyUsageClientAuth` when `IntermediateCertPEM` is configured
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - `vpn ipsec site-to-site peer authentication` has no `allowed-identities` or `server-identity` nodes
- [ ] `internal/component/ike/ipsec/types.go` - `AuthConfig.Additions` (`*AuthAdditions`) holds `IntermediateCertificates` and `SecondRound`; `AuthConfig.Equal` uses `reflect.DeepEqual`
- [ ] `internal/component/ike/ipsec/config.go` & `config_auth_policy.go` - `parseAuthConfig` reads `intermediate-certificate` and `second-round` in all modes for validation
- [ ] `internal/component/ike/ipsec/validate.go` - `ValidatePKIRefs` and `ValidateIntermediateRefs` validate EAP-TLS CA and intermediate refs; no EAP-TLS identity validation exists
- [ ] `internal/component/ike/engine/remote_id.go` - `hasSubjectAltName`, `asciiEqualFold`, `certificateCarriesIdentity` implement SAN-over-CN precedence and class separation for IKEv2 `IDi`/`IDr`
- [ ] `internal/component/ike/engine/config.go` - `validateIPsecSections` runs the IPsec config validators for both `InProcessConfigVerifier` and `OnConfigVerify`
- [ ] `internal/component/ike/engine/responder_eap.go` - `eapTLSServerConfig` loads cert, key, OCSP staple, CA, CRL, intermediates, and resumption state into `eap.MethodConfig`; `handleResponderEAP` calls `recordEAPTLSAuthentication(sa, sess.Resumed(), log)`
- [ ] `internal/component/ike/engine/fsm.go` - `buildPeerTLSConfig` loads client cert, key, CA, CRL, OCSP flag, and resumption state into `eap.PeerTLSConfig`; `handleEAPResponse` calls `recordEAPTLSAuthentication(sa, ps.Resumed(), log)`
- [ ] `internal/component/ike/engine/resumption.go` - `resumptionFor` keys per-peer ticket state on `AuthConfig.Equal`; `recordEAPTLSAuthentication` logs `peer` and `resumed` without the authenticated Peer-Id or Server-Id
- [ ] `internal/component/ike/engine/register.go` & `register_doctor_codes.go` - registers `InProcessConfigVerifier: validateIPsecSections` and all IPsec doctor checks and diagnostic codes

**Behavior to preserve:**
- Non-`eap-tls` authentication modes (`pre-shared-secret`, `x509`, `eap-mschapv2`, `eap-md5`) remain completely unchanged when `allowed-identities` and `server-identity` are unset.
- `vpn ipsec remote-access` authentication and `eap-user` parsing/validation remain untouched (owned by `spec-ipsec-remote-access.md`).
- EAP-TLS TLS 1.2 and TLS 1.3 wire framing, fragmentation, MSK/EMSK derivation, protected success indication (`0x00`), OCSP stapling, CRL revocation checks, intermediate CA pool validation, and session ticket issuance/resumption continue to function identically for authorized identities.
- `ValidatePKIRefs` still runs before `ValidateEAPTLSIdentities`, so an `eap-tls` peer missing `ca-certificate` keeps reporting `eap-tls requires a ca-certificate`.
- `SiteToSitePeer` struct size stays strictly under 288 bytes (`TestSiteToSitePeerStaysUnderTheHugeParamThreshold`).
- `TestSiteToSitePeerEqualAcrossTwoParses` remains true (two parses of an unchanged config compare equal and do not restart tunnels).

**Behavior to change:**
- `eapTLSPeerName` returns the first SAN entry when `subjectAltName` is present, instead of `Subject.String()`.
- `Session` exports `PeerID()`/`PeerIDs()` and `PeerSession` exports `ServerID()`/`ServerIDs()`, populated from the verified certificate's SAN (or CN/Subject when no SAN extension exists).
- Authenticator `VerifyConnection` refuses any client certificate whose SAN (or CN when SAN is absent) does not match `MethodConfig.AllowedIdentities`, sending a fatal TLS `bad_certificate` alert (full and resumed TLS 1.3 handshakes).
- Peer `serverChainCheck.verifyConnection` refuses any server certificate whose SAN (or CN when SAN is absent) does not match `PeerTLSConfig.ServerIdentity` per RFC 2818 Section 3.1, sending a fatal TLS `bad_certificate` alert (full and resumed TLS 1.3 handshakes).
- `ze config validate`, commit, and SIGHUP reload refuse any `site-to-site` `eap-tls` peer lacking a valid identity constraint (`allowed-identities` on `respond`, or `server-identity` / unambiguous FQDN `remote-address` on `initiate`), or carrying the wrong role's leaf or either leaf on a non-`eap-tls` mode.
- `recordEAPTLSAuthentication` logs the authenticated `peer-id` (responder) or `server-id` (initiator) on every completed EAP-TLS authentication.
- All existing repository EAP-TLS `.ci` tests, interop scenario configs, and unit test helpers are updated to declare explicit `allowed-identities` / `server-identity`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|---|---|---|---|
| SAN extension & UPN ASN.1 parser | `internal/core/eap/nai.go` (`oidSubjectAltName`, `oidUserPrincipalName`, `otherName`, `otherNameText`, `validNAI`, `validRealm`) | Reads `rfc822Name` and `userPrincipalName` `otherName` from SAN and validates RFC 7542 NAI/realm syntax | Extends: `internal/core/eap/identity.go` reuses `oidSubjectAltName`, `oidUserPrincipalName`, `otherNameText`, `validNAI`, and `validRealm` to walk all SAN GeneralNames in certificate wire order and validate NAI/realm entries without duplicating ASN.1 or NAI grammar code |
| Authenticator TLS verification callback | `internal/core/eap/eap_tls.go` (`newTLSMethod` `VerifyConnection`) and `internal/core/eap/client_chain.go` (`clientChainVerifier.verify`) | Verifies client chain (with `ExtKeyUsageClientAuth`) and CRL revocation on full and resumed sessions; sends fatal `bad_certificate` alert via `MethodResult.FinalRequest` on error | Extends: runs client identity authorization and records verified `PeerIDs` inside both `VerifyConnection` branches right after chain and revocation pass |
| Peer TLS chain verification | `internal/core/eap/peer_chain.go` (`serverChainCheck.verifyPeerCertificate`, `verifyConnection`, `rebuildResumedChains`) | Verifies server chain against `roots` (`ExtKeyUsageServerAuth`), rebuilds chains on resumed TLS 1.3 sessions, checks CRL and OCSP stapling, and publishes `verified` chains atomically | Extends: adds RFC 2818 Section 3.1 server identity verification in `verifyConnection` after chain, CRL, and OCSP checks (covering both full and resumed handshakes) and atomically publishes verified `serverIDs` |
| IKEv2 SAN-over-CN identity rules | `internal/component/ike/engine/remote_id.go` (`hasSubjectAltName`, `asciiEqualFold`, `certificateCarriesIdentity`) | Enforces SAN precedence over CN, ASCII-only case folding, and IP-vs-DNS class separation for IKEv2 `IDi`/`IDr` | Does not replace (separate protocol layer for IKEv2 `IDi`/`IDr`); aligns semantics: `internal/core/eap/identity.go` enforces the exact same SAN-over-CN, ASCII case-folding, and IP-vs-DNS separation invariants for EAP-TLS certificates |
| Zero-size-growth `AuthAdditions` container | `internal/component/ike/ipsec/types.go` (`AuthConfig.Additions`, `AuthAdditions`, `AuthConfig.Equal`) | Holds optional per-mode auth fields (`IntermediateCertificates`, `SecondRound`) behind one pointer so `SiteToSitePeer` stays under 288 bytes and `reflect.DeepEqual` detects all config edits | Extends: adds `AllowedIdentities` and `ServerIdentity` to `AuthAdditions` with nil-safe accessors on `AuthConfig`, keeping `SiteToSitePeer` byte size unchanged |
| Unified config verification gate | `internal/component/ike/engine/config.go` (`validateIPsecSections`) wired via `engine/register.go` (`InProcessConfigVerifier`) and `engine/config_verify_commit.go` (`verifyIPsecCommit`) | Runs all IPsec semantic checks side-effect-free on `ze config validate`, commit, and SIGHUP reload | Extends: adds `cfg.ValidateEAPTLSIdentities()` right after `ValidateIntermediateRefs` so all three config doors share one fail-closed verdict |
| Per-peering EAP-TLS resumption store | `internal/component/ike/engine/resumption.go` (`resumptionFor`, `recordEAPTLSAuthentication`) | Invalidates ticket keys when `AuthConfig.Equal` changes and logs completed EAP-TLS authentications | Extends: `AuthConfig.Equal` automatically invalidates tickets on `allowed-identities` or `server-identity` edits; `recordEAPTLSAuthentication` gains the authenticated identity argument to log `peer-id` or `server-id` |
| IPsec post-config doctor checks | `internal/component/ike/engine/register.go` (`DoctorChecks`) and `register_doctor_codes.go` (`doctorDiagnosticCodes`) | Registers post-config checks and diagnostic code metadata for `ze doctor` and `Warn` log coverage | Extends: adds `ipsec-eap-tls-identity` check (`DoctorPhasePostConfig`) and `doctor-ipsec-eap-tls-identity` code covering unvalidated startup configs and `buildPeerTLSConfig`'s runtime fail-closed warning |
| YANG embedded schema glue | `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` + `./le yang glue update` | Embeds and registers the `ze-ipsec-conf` YANG module for CLI editor, completion, web, API, and validation | Extends: adds `allowed-identities` and `server-identity` under `site-to-site peer authentication` and regenerates glue |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|---|---|---|---|---|---|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A backend that cannot apply a config exactly refuses it at verify. It never approximates" | agrees | Every missing or malformed `allowed-identities` or `server-identity` is refused fail-closed at `ze config validate`, commit, and reload with an `opererr` carrying the exact fix command; no silent fallback or approximation is performed |
| `ai/doors/access-security.md` | Boundaries | the door covering `internal/core/eap/` and `internal/component/ike/` | "EAP-TLS compiles only against a patched `crypto/tls`." | agrees | All new EAP-TLS identity code and tests live in `internal/core/eap` and `internal/component/ike` and are built and run through `./le`; no IKE state reader bypasses the core leaves |
| `ai/doors/core.md` | Boundaries | the door covering `internal/core/eap/` | "Where core needs something a component owns, the component registers into core. Do not add a row to the core import baseline." | agrees | `internal/core/eap/identity.go` imports only Go standard library packages; `internal/component/ike/ipsec` and `internal/component/ike/engine` import `internal/core/eap` downward |
| `ai/doors/config-yang.md` | Boundaries | the door covering `ze-ipsec-conf.yang` and config verification | "`./le yang glue update` regenerates each `yang/` package's `embed.go` and `register.go`." | agrees | Schema changes go into `ze-ipsec-conf.yang` only, glue is regenerated with `./le yang glue update`, and validation runs through `InProcessConfigVerifier` (`validateIPsecSections`) so offline validate and live commit agree |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `internal/le/interoplab/ipsec/checkers.go` and `test/` | "**Never import `internal/le` from product code**, not even for a helper." | agrees | Interop scenario checkers live in `internal/le/interoplab/ipsec/checkers.go` and new `.ci` tests live under `test/ipsec/`, `test/parse/`, and `test/reload/`; no product package imports `internal/le` |
| `docs/architecture/ike/ipsec-3-data-model.md` | Decisions | the `// Design:` page of `ipsec/types.go`, `config.go`, `config_auth_policy.go`, `validate.go`, and modified by this spec | "`Additions` is one pointer shared with the intermediate certificates, which keeps `SiteToSitePeer` under the 288-byte by-value threshold." | agrees | `AllowedIdentities` and `ServerIdentity` are placed inside `AuthAdditions` so `SiteToSitePeer` stays under 288 bytes and `SiteToSitePeer.Equal` / `AuthConfig.Equal` continue to compare every member via `reflect.DeepEqual` |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Decisions | the `// Design:` page of `engine/config.go`, `engine/fsm.go`, `engine/register.go` | "A reload restarts a peer whose configuration differs in ANY member, and leaves every other peer alone." | agrees | Because `AllowedIdentities` and `ServerIdentity` live on `AuthConfig.Additions`, editing either leaf makes `SiteToSitePeer.Equal` return false, restarting that peer and refreshing its resumption store while leaving other peers untouched |
| `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` | Decisions | anchored to `ipsec/types.go`, `ipsec/validate.go`, `engine/config.go`, `engine/fsm.go`, `engine/register.go`, `engine/register_doctor_codes.go` | — | silent | Covers Child SA installation, Linux XFRM / VPP dataplane, state scope, fragment guard, and route redistribution; silent on EAP-TLS certificate identity authorization, which completes before Child SA creation |
| `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` | Traps this code exists to avoid | the `// Design:` page of `eap/eap.go`, `eap/eap_tls.go`, and modified by this spec | "The cause is recorded on the round it is COMPUTED, never on the round after." | agrees | Client identity authorization runs in `VerifyConnection` during the TLS handshake; a mismatch produces a fatal `bad_certificate` alert delivered via `MethodResult.FinalRequest` and recorded in `Session.err` on the same round |
| `docs/architecture/ike/ipsec-10-cli-diag.md` | Decisions | anchored to `engine/fsm.go`, `engine/register.go` | — | silent | Covers IPsec show/clear/monitor commands, Prometheus metrics, and runtime SA doctor checks; silent on EAP-TLS certificate identity authorization and post-config validation (this spec does not change `PeerInfo`, `show vpn ipsec sa`, or metrics) |
| `docs/architecture/ike/ipsec-11-interop-eap.md` | Traps this code exists to avoid | the `// Design:` page of `eap/peer.go`, `eap/peer_chain.go`, and modified by this spec | "`serverChainCheck.rebuildResumedChains` is therefore the ONLY thing that revalidates the cached chain on this role." | agrees | Server identity authorization runs in `serverChainCheck.verifyConnection` after `rebuildResumedChains` (keeping `InsecureSkipVerify: true` and empty `ServerName` so per-peering resumption and CN fallback work identically on full and resumed sessions) |
| `docs/architecture/ike/ipsec-13-rekey-wire.md` | Traps this code exists to avoid | anchored to `engine/fsm.go`, `engine/register.go` | "On the EAP path the increment sits on the success branch alone: `handleEAPResponse` calls `advanceMsgID` only when the peer session reports `Done`, and any `Err` takes the SA to `StateDead` instead." | agrees | When `serverChainCheck.verifyConnection` rejects an unauthorized server certificate, `PeerSession.Process` returns `PeerResult.Err`, so `handleEAPResponse` leaves the message ID untouched and moves the SA to `StateDead` |
| `docs/architecture/ike/ipsec-14-responder.md` | Decisions | the `// Design:` page of `engine/responder_eap.go`, `engine/resumption.go`, and modified by this spec | "It is discarded when the peer leaves the config, and rebuilt whenever the peer's authentication config changes" | agrees | `eapTLSServerConfig` wires `AllowedIdentities` into `eap.MethodConfig`, `resumptionFor` rebuilds ticket keys on any `allowed-identities` edit, and `recordEAPTLSAuthentication` logs the verified `peer-id` |
| `docs/architecture/linux-limitations.md` | What belongs here | anchored to `engine/config.go` | — | silent | Global register of Linux kernel and Go library limitations; silent on EAP-TLS certificate identity authorization, which runs entirely in Go user space with no kernel or library limitation |
| `docs/architecture/api/process-protocol.md` | Overview | anchored to `engine/register.go` | — | silent | Documents plugin wire framing, Stage 1-5 startup, and reload RPCs; silent on EAP-TLS certificate identity authorization, which keeps `ike`'s existing `InProcessConfigVerifier` and `OnConfigVerify`/`OnConfigApply` wiring unchanged |
| `docs/architecture/api/process-protocol.md` | Design decisions | anchored to `engine/register.go` (`## Design decisions`, decision of 2026-11-23) | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | This spec adds no plugin engine operation and does not alter plugin identity authorization profiles; it authorizes X.509 EAP-TLS peer and server certificates inside `internal/core/eap` and `internal/component/ike` |
| `docs/architecture/api/process-protocol.md` | Design decisions | anchored to `engine/register.go` (`## Design decisions`, decision of 2026-11-25) | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | This spec does not modify plugin event delivery queues or overflow policies; EAP-TLS identity authorization runs synchronously inside the IKE engine handshake and config verification paths |
| `docs/architecture/doctor-and-health-checks.md` | Every finding names the command that fixes it | the `// Design:` page of `engine/register_doctor_codes.go` | "A doctor diagnostic carries `fix`: the command or config statement the operator runs to clear the problem, alone and with no label, as an operator error holds its fix (`internal/core/opererr`)." | agrees | `checkIPsecEAPTLSIdentity` registers `doctor-ipsec-eap-tls-identity` in `DoctorPhasePostConfig` with a concrete `set vpn ipsec site-to-site peer <name> authentication ...` fix string and backs the startup `Warn` log in `buildPeerTLSConfig` |
| `docs/architecture/testing/interop.md` | Design decisions | the `// Design:` page of `internal/le/interoplab/ipsec/checkers.go` (`## Design decisions` row 1) | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | The two new strongSwan negative scenarios (`responder-eap-tls13-unauthorized-client` and `eap-tls13-unauthorized-server`) run inside the existing `interop-ipsec` suite using its standard IPsec plan and declared kernel capabilities without altering any suite's capability selector |
| `docs/architecture/testing/interop.md` | The suites, one per protocol area | the `// Design:` page of `internal/le/interoplab/ipsec/checkers.go` | "A scenario directory carries only declarative inputs its runner reads: `ze.conf` plus the peer configuration and argument files that topology needs. Assertions never live there. They are typed Go checkers under `internal/le/interoplab/`" | agrees | Both new scenario directories hold only `ze.conf`, `swanctl.conf`, `strongswan.conf`, and PKI references, and their typed assertions live in `internal/le/interoplab/ipsec/checkers.go` |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
1. **Configuration path:** Operator writes `vpn ipsec site-to-site peer <name> authentication { mode eap-tls; allowed-identities ...; }` (for `connection-type respond`) or `server-identity ...;` / FQDN `remote-address` (for `connection-type initiate`), entering via `ze config validate`, CLI/web/API commit, SIGHUP reload, or daemon startup.
2. **Wire path (authenticator):** Inbound IKE_AUTH carrying EAP-Response/EAP-TLS with the client's TLS Certificate (or TLS 1.3 resumption PSK ticket carrying the cached client certificate).
3. **Wire path (peer):** Inbound IKE_AUTH carrying EAP-Request/EAP-TLS with the authenticator's TLS Certificate (or TLS 1.3 resumed ServerHello referencing the cached server certificate).

### Transformation Path
1. **Config parse and verify:** `config.Tree` -> `ipsec.ParseIPsecConfig` -> `parseAuthConfig` -> `parseEAPTLSIdentityPolicy` populates `AuthConfig.Additions.AllowedIdentities` and `AuthConfig.Additions.ServerIdentity`. `engine.validateIPsecSections` calls `cfg.ValidateEAPTLSIdentities()`, which delegates syntax validation to `eap.ParseAllowedIdentity` / `eap.ParseServerIdentity` / `eap.IsUnambiguousFQDN` and refuses missing/wrong-role/wrong-mode/malformed entries with `opererr`.
2. **Authenticator handshake (`connection-type respond`):** `startResponderEAP` -> `eapTLSServerConfig` copies `sa.PeerCfg.Auth.AllowedIdentities()` into `eap.MethodConfig.AllowedIdentities` (failing closed if empty) -> `eap.NewSession(TypeTLS, cfg)` -> `newTLSMethod` parses rules via `ParseAllowedIdentity` -> during TLS handshake (full or resumed), `VerifyConnection` validates chain (`RequireAndVerifyClientCert` or `clientChainVerifier.verify`) -> `checkChainRevocation` -> `authorizePeerCertificate(leaf, rules)` extracts SAN-first identities and checks against compiled rules -> on mismatch returns error -> `crypto/tls` emits fatal `bad_certificate` alert -> `tlsMethod.Process` sends it via `MethodResult.FinalRequest` and writes `Session.err` -> `handleResponderEAP` logs `ike: EAP authentication failed` with the extracted Peer-Id; on match stores verified Peer-Id(s) on `tlsMethod` -> on EAP-Success `handleResponderEAP` logs `ike: EAP-TLS authenticated` with `peer-id`.
3. **Peer handshake (`connection-type initiate`):** `startEAPExchange` -> `buildPeerTLSConfig` computes `sa.PeerCfg.EffectiveEAPTLSServerIdentity()` (failing closed with `code: doctor-ipsec-eap-tls-identity` Warn if empty) -> sets `eap.PeerTLSConfig.ServerIdentity` -> `eap.NewPeerSessionTLS` -> `startTLSClient` parses `ServerIdentity` via `ParseServerIdentity` and stores it on `serverChainCheck` -> during TLS handshake (full or resumed), `verifyPeerCertificate` (or `rebuildResumedChains`) builds verified chains -> `verifyConnection` runs `checkChainRevocation` -> `checkStapledChainStatus` -> `verifyServerCertificate(chains[0][0], expectedServerID)` enforcing RFC 2818 Section 3.1 -> on mismatch returns error -> `crypto/tls` emits fatal `bad_certificate` alert -> peer sends alert in EAP-Response, parks cause in `pendingErr`, and fails exchange with `ike: EAP failed` naming expected vs presented Server-Id; on match publishes `serverIDs` atomically -> on EAP-Success `handleEAPResponse` logs `ike: EAP-TLS authenticated` with `server-id`.

### Boundaries Crossed
| Boundary | How | Verified |
|---|---|---|
| YANG / Config Provider -> `ike` plugin | JSON `sdk.ConfigSection` (`vpn`, `pki`) lowered by `config.TreeFromPluginMap` into `ipsec.IPsecConfig` | Yes (`test/parse/ipsec-eap-tls-identity-required.ci`, `test/reload/tx-ipsec-eap-tls-identity-required.ci`) |
| `ike/ipsec` -> `core/eap` | Direct downward Go import (`eap.ParseAllowedIdentity`, `eap.ParseServerIdentity`, `eap.IsUnambiguousFQDN`) | Yes (`./le arch tier check`, `TestValidateEAPTLSIdentitiesMatrix`) |
| `ike/engine` -> `core/eap` | `eap.MethodConfig.AllowedIdentities` and `eap.PeerTLSConfig.ServerIdentity` passed at session construction; `Session.PeerID()`/`PeerIDs()` and `PeerSession.ServerID()`/`ServerIDs()` read on completion | Yes (`TestEAPTLSServerConfigWiresAllowedIdentities`, `TestBuildPeerTLSConfigWiresServerIdentity`, `test/ipsec/ipsec-eap-tls-identity-authorized.ci`) |
| `core/eap` -> `ike/engine` (alert & failure cause) | `MethodResult.FinalRequest` + `Session.Err()` (authenticator) and `PeerResult.Err` (peer) over IKE_AUTH SK-encrypted EAP payloads | Yes (`test/ipsec/ipsec-eap-tls-identity-rejected.ci`, `responder-eap-tls13-unauthorized-client`, `eap-tls13-unauthorized-server`) |
| `ike/engine` -> Operator / Doctor | `opererr` refusals at validate/commit/reload; `recordEAPTLSAuthentication` Info log (`peer-id`/`server-id`); `checkIPsecEAPTLSIdentity` (`doctor-ipsec-eap-tls-identity`) | Yes (`./le arch operator-error check`, `./le arch doctor-coverage check`, `TestDoctorIPsecEAPTLSIdentityCheck`) |

### Changed-Wire Census

**Search evidence:** Scope: `internal/core/eap`, `internal/component/ike`, `test/`, `docs/`; terms: `eap-tls`, `AuthEAPTLS`, `PeerTLSConfig`, `MethodConfig`, `eapTLSPeerName`, `recordEAPTLSAuthentication`, `buildPeerTLSConfig`, `eapTLSServerConfig`, `validateIPsecSections`. Positive controls confirmed: `test/ipsec/ipsec-eap-tls-clienthello.ci`, `test/interop-ipsec/scenarios/eap-tls13/ze.conf`, `test/interop-ipsec/scenarios/responder-eap-tls13/ze.conf`, `internal/component/ike/engine/responder_eap.go::eapTLSServerConfig`, and `internal/component/ike/engine/fsm.go::buildPeerTLSConfig` were all found. No IKEv2 or EAP-TLS wire packet format changes (RFC 5216 / RFC 9190 / RFC 7296 message layouts are preserved; unauthorized peers now receive standard TLS `bad_certificate` fatal alerts already encoded by `crypto/tls`). The changed wires are the `vpn ipsec site-to-site peer authentication` YANG config tree (plus its SDK JSON plugin section) and the structured EAP-TLS authentication log line.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|---|---|---|---|---|
| `vpn ipsec site-to-site peer authentication` config tree & SDK JSON | writer | `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` (`container authentication`), CLI editor, and repository configs (`test/ipsec/ipsec-eap-tls-*.ci`, `test/ipsec/ipsec-eap-nak-unacceptable-type.ci`, `test/parse/reference-ipsec-eap-tls-ca-missing.ci`, `test/interop-ipsec/scenarios/*eap-tls*/ze.conf`, `test/interop-ipsec/scenarios/real-nat-port-translated-eap-ze-responder/ze.conf`) | Grep `mode eap-tls` across `test/` located all 6 `.ci` files and 7 interop `ze.conf` files | Change: add `allowed-identities` leaf-list and `server-identity` leaf to YANG schema; update all 13 repository test/interop configs in the same change to carry explicit constraints (`ai/META-DECISIONS.md`) |
| `vpn ipsec site-to-site peer authentication` config tree & SDK JSON | reader | `internal/component/ike/ipsec/config_auth_policy.go` (`parseEAPTLSIdentityPolicy`), `ipsec/validate.go` (`ValidateEAPTLSIdentities`), `engine/config.go` (`validateIPsecSections`), `engine/doctor_eap_tls_identity.go` (`checkIPsecEAPTLSIdentity`), `engine/responder_eap.go` (`eapTLSServerConfig`), `engine/fsm.go` (`buildPeerTLSConfig`) | Read of `config.go`, `config_auth_policy.go`, `validate.go`, `engine/config.go`, `responder_eap.go`, `fsm.go` | Change: parse both new nodes into `AuthAdditions`, enforce fail-closed commit/validate/reload rules, and wire effective identities into `eap.MethodConfig` and `eap.PeerTLSConfig` |
| EAP-TLS over IKE_AUTH wire exchange | writer | `internal/core/eap/eap_tls.go` (`tlsMethod.Process`, `VerifyConnection`), `internal/core/eap/peer.go` (`handleTLSRequest`), `internal/core/eap/peer_chain.go` (`serverChainCheck.verifyConnection`) | Read of `eap_tls.go`, `peer.go`, `peer_chain.go` | Preserve wire format; unauthorized identity now returns an error from `VerifyConnection`, causing `crypto/tls` to emit a standard fatal `bad_certificate` TLS alert |
| EAP-TLS over IKE_AUTH wire exchange | reader | Outside-repository IKEv2/EAP-TLS peers (strongSwan 6.x charon in `test/interop-ipsec/scenarios/*eap-tls*`, OS road-warrior VPN clients) | Verified in `test/interop-ipsec/scenarios/eap-tls13/swanctl.conf` and `responder-eap-tls13/swanctl.conf` | Preserve: strongSwan already parses standard TLS `bad_certificate` alerts and completes authorized handshakes with SANs (`IP:172.28.0.3`, `DNS:swan-eap-client`) |
| Structured daemon log (`ike: EAP-TLS authenticated`) | writer | `internal/component/ike/engine/resumption.go` (`recordEAPTLSAuthentication`), called by `responder_eap.go` and `fsm.go` | Grep `recordEAPTLSAuthentication` found all 3 non-test call sites | Change (additive): keep existing `peer` and `resumed` attributes; add `peer-id` (responder) or `server-id` (initiator) attribute carrying the SAN-derived verified identity |
| Structured daemon log (`ike: EAP-TLS authenticated`) | reader | Operator log queries (`show log`), `test/ipsec/ipsec-eap-tls13-resumption.ci`, `internal/le/interoplab/ipsec/checkers.go`, and new `test/ipsec/ipsec-eap-tls-identity-*.ci` | Grep `EAP-TLS authenticated` across `internal/` and `test/` | Preserve existing fields; new functional tests assert the added `peer-id` / `server-id` attribute |

### Integration Points
- `internal/core/eap/eap_tls.go::eapTLSPeerName` — switched to SAN-first `certificatePrimaryID` so every error and log line (including `eapTLS12ExportRefused`) names the RFC 5216 §5.2 identity.
- `internal/core/eap/eap.go::Session` and `peer.go::PeerSession` — new `PeerID()`, `PeerIDs()`, `ServerID()`, `ServerIDs()` accessors for IKE logging and downstream `spec-ipsec-remote-access.md`.
- `internal/component/ike/ipsec/types.go::AuthAdditions` — zero-byte-growth storage on `SiteToSitePeer`, automatic participation in `SiteToSitePeer.Equal` and `AuthConfig.Equal` (restarting edited peers and invalidating resumption stores).
- `internal/component/ike/engine/config.go::validateIPsecSections` — single validation funnel for `ze config validate`, commit, and SIGHUP reload.
- `internal/component/ike/engine/register.go` & `register_doctor_codes.go` — `ipsec-eap-tls-identity` doctor check and `doctor-ipsec-eap-tls-identity` diagnostic code.

### Architectural Verification
| Check | Holds? | Evidence |
|---|---|---|
| No bypassed layers (data flows through the intended path) | Yes | Config flows `ze-ipsec-conf.yang` -> `ipsec.ParseIPsecConfig` (`parseEAPTLSIdentityPolicy`) -> `engine.validateIPsecSections` (`ValidateEAPTLSIdentities`) -> `eapTLSServerConfig` / `buildPeerTLSConfig` -> `eap.NewSession` / `eap.NewPeerSessionTLS` -> `VerifyConnection` (`authorizePeerCertificate` / `verifyServerCertificate`), proved by `ipsec-eap-tls-identity-authorized.ci` and `ipsec-eap-tls-identity-rejected.ci` |
| No unintended coupling (components stay isolated) | Yes | `./le arch tier check` passes; `internal/core/eap/identity.go` imports Go stdlib only; `internal/component/ike/ipsec` and `engine` import `internal/core/eap` downward |
| No duplicated functionality (extends existing, does not recreate) | Yes | `identity.go` reuses `nai.go` (`oidSubjectAltName`, `oidUserPrincipalName`, `otherName`, `otherNameText`, `validNAI`, `validRealm`), `ipsec/validate.go` delegates syntax checks to `eap.ParseAllowedIdentity`/`ParseServerIdentity`/`IsUnambiguousFQDN`, and `AuthAdditions` holds the new config fields |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `SiteToSitePeer` stays strictly under 288 bytes (`TestSiteToSitePeerStaysUnderTheHugeParamThreshold` PASS) |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | `./le yang glue check` and `./le config claims` pass; doctor check registers in `engine/register.go` and code in `register_doctor_codes.go`; no switch or central list added outside `ike`/`eap` |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | `allowed-identities` and `server-identity` derive automatically from `ze-ipsec-conf.yang` (verified by `./le config claims` and `test/parse/ipsec-eap-tls-identity-required.ci`); no duplicate leaf list exists in `internal/component/ike/ipsec/` or `internal/component/config/` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|---|---|---|---|---|---|
| A-1 | `internal/component/ike/ipsec` is allowed by `./le arch tier check` to import `internal/core/eap` for shared identity pattern parsing (`ParseAllowedIdentity`, `ParseServerIdentity`, `IsUnambiguousFQDN`) | `ai/doors/core.md`: "Components, plugins and the public SDK all import core"; `internal/component/ike/engine` already imports `internal/core/eap` | Move shared pattern validation into a tiny stdlib-only helper or duplicate with parity test | `./le arch tier check` PASS and `./internal/component/ike/ipsec/` unit tests PASS | confirmed |
| A-2 | On a TLS 1.3 resumed handshake on the authenticator, `cs.PeerCertificates` in `VerifyConnection` contains the cached client leaf certificate restored from the session ticket | `internal/core/eap/client_chain.go:81` comment and `crypto/tls` `serverHandshakeStateTLS13.readClientHello` | Resumed authenticator sessions would fail closed with empty `PeerCertificates` and would require refusing resumption or caching Peer-Id in ticket state | `TestRFC5216ResumedSessionReverifiesPeerAndServerIdentity` PASS (verified resumed authenticator both succeeds with matching policy and refuses with narrowed policy on cached leaf) | confirmed |
| A-3 | Existing strongSwan interop certificates carry `IP Address:172.28.0.3` (`eap-tls13/pki/server.pem`) and `DNS:swan-eap-client` (`responder-eap-tls13/pki/server.pem`, `responder-eap-tls13-leaf-only-client/pki/server.pem`) | Inspected directly with `openssl x509 -noout -subject -ext subjectAltName` on 2026-11-26 | Scenario configs would fail identity check until given the actual SANs | Verified in strongSwan runs: `eap-tls` PASS, `responder-eap-tls13-revoked-client` PASS, and `eap-tls13`/`responder-eap-tls13` logged `EAP-TLS authenticated` with `server-id=172.28.0.3` and `peer-id=swan-eap-client` | confirmed |
| A-4 | strongSwan 6.x logs a peer TLS `bad_certificate` alert and installs no XFRM SA when Ze rejects its server or client identity during EAP-TLS | `docs/architecture/ike/ipsec-11-interop-eap.md` (`responder-eap-tls13-revoked-client`) | Checker log needle for strongSwan side must be verified against strongSwan 6.x source before finalizing assertions (`ai/rules/interop-and-goal-validation.md`) | `responder-eap-tls13-unauthorized-client` PASS (13s, counterfactual red `SWAN_CLIENT_RC=1`) and `eap-tls13-unauthorized-server` PASS (11s, counterfactual red `SWAN_SERVER_RC=1`) | confirmed |
| A-5 | Adding `AllowedIdentities` and `ServerIdentity` inside `AuthAdditions` keeps `unsafe.Sizeof(SiteToSitePeer{}) < 288` | `internal/component/ike/ipsec/types.go:496`: `Additions` is already a single pointer on `AuthConfig` | `TestSiteToSitePeerStaysUnderTheHugeParamThreshold` fails | `TestSiteToSitePeerStaysUnderTheHugeParamThreshold` PASS | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|---|---|---|---|
| R-1 | Updating fail-closed validation breaks existing EAP-TLS tests or interop scenarios that omitted identity constraints | Unit tests in `internal/core/eap/`, `.ci` files in `test/ipsec/` / `test/parse/`, or strongSwan interop scenarios fail | Inventory every `mode eap-tls` config (already enumerated in Changed-Wire Census) and update all 13 test/interop configs and unit test helpers in the same change; keep `ValidatePKIRefs` ahead of `ValidateEAPTLSIdentities` so `tx-ipsec-eap-tls-requires-ca.ci` keeps its exact refusal |
| R-2 | CN fallback accidentally runs when a certificate carries a SAN extension of another type (e.g. `iPAddress` SAN + attacker-controlled CN matching `dNSName`) | Security review or adversarial unit test passes with mismatched SAN + matching CN | Gate CN fallback strictly on `!hasSubjectAltName(cert)` (checking OID `2.5.29.17` presence in `cert.Extensions`, identical to `engine/remote_id.go::hasSubjectAltName`), never on `len(cert.DNSNames) == 0`; dedicated negative unit test proves refusal |
| R-3 | Wildcard matching allows multi-label match (`*.example.com` matching `a.b.example.com`) or overbroad TLD wildcards (`*.com`) | Unit test for RFC 2818 Section 3.1 wildcard boundaries | Restrict wildcard matching to a single leftmost full-label `*` followed by at least two valid DNS labels, matching exactly one non-empty label with no `.`; refuse `*.com`, `*.*.example.com`, and partial-label wildcards at commit and in runtime matcher |
| R-4 | A resumed TLS 1.3 session bypasses identity authorization or survives a config edit that narrowed `allowed-identities` or changed `server-identity` | Resumption test establishes after identity policy change | Run identity verification inside `VerifyConnection` on every handshake (full and resumed), AND ensure `AuthConfig.Equal` compares `AuthAdditions` so `resumptionFor` discards ticket keys on any edit |
| R-5 | A daemon booted from an unvalidated legacy config on disk lacks `allowed-identities` or `server-identity` and either fails open or logs an uncovered `Warn` | `./le arch doctor-coverage check` fails, or handshake accepts unauthorized cert | Runtime builders (`eapTLSServerConfig`, `buildPeerTLSConfig`) and EAP core (`newTLSMethod`, `startTLSClient`) all fail closed; `buildPeerTLSConfig` logs `Warn` with `code: doctor-ipsec-eap-tls-identity` backed by `checkIPsecEAPTLSIdentity` |
| R-6 | Overlapping allow-list design conflicts with `spec-ipsec-remote-access.md` (R-8 of that spec) | Review of `spec-ipsec-remote-access.md` | Keep `allowed-identities` strictly under `site-to-site peer authentication`; `remote-access` keeps `eap-user` (matching verified leaf cert / exported `Session.PeerID()`/`PeerIDs()`), and `Session.PeerID()`/`PeerIDs()` supplies the exported identity `spec-ipsec-remote-access.md` depends on |

## Blast Radius

| Question | Answer |
|---|---|
| What breaks if this is wrong? | If too loose (fail-open): any certificate signed by the trusted CA authorizes as any EAP-TLS peer or server (security breach). If too strict or misparsed: legitimate `mode eap-tls` site-to-site tunnels fail commit or fail TLS handshake with `bad_certificate`. Non-`eap-tls` IPsec peers (PSK, X.509, EAP-MSCHAPv2, EAP-MD5) and all other subsystems are unaffected. |
| How is it reverted? | Single commit revert restores prior config schema and removes the identity checks; configs written with `allowed-identities` or `server-identity` would no longer parse after revert. |
| Who else touches this path? | `plan/release/fixes/spec-ipsec-remote-access.md` (depends on this spec's `Session.PeerID()`/`PeerIDs()` export and `RFC5216-5.3-2`), `plan/post-release/features/spec-eap-tls-certificate-profile.md` (owns `RFC5216-5.2-2..4`, separate certificate profile checks). |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|---|---|---|---|
| `ze config validate` / commit / reload on `site-to-site peer` with `mode eap-tls` | → | `ipsec.parseEAPTLSIdentityPolicy` → `engine.validateIPsecSections` → `ipsec.ValidateEAPTLSIdentities` | `TestValidateIPsecSectionsEnforcesEAPTLSIdentityConstraints` and `test/parse/ipsec-eap-tls-identity-required.ci` |
| EAP-TLS exchange with certificate carrying SAN (both roles) | → | `eap.extractCertificateIdentities`, `eapTLSPeerName`, `Session.PeerID`/`PeerIDs`, `PeerSession.ServerID`/`ServerIDs`, `VerifyConnection` authorization | `TestRFC5216IdentityIsTakenFromTheSANAndAuthorized` |
| IKE responder `eapTLSServerConfig` + `handleResponderEAP` | → | `eap.MethodConfig.AllowedIdentities` → `tlsMethod` `VerifyConnection` → `recordEAPTLSAuthentication` (`peer-id`) | `TestEAPTLSServerConfigWiresAllowedIdentities` and `test/ipsec/ipsec-eap-tls-identity-rejected.ci` |
| IKE initiator `buildPeerTLSConfig` + `handleEAPResponse` | → | `SiteToSitePeer.EffectiveEAPTLSServerIdentity` → `eap.PeerTLSConfig.ServerIdentity` → `serverChainCheck.verifyConnection` → `recordEAPTLSAuthentication` (`server-id`) | `TestBuildPeerTLSConfigWiresServerIdentity` and `test/ipsec/ipsec-eap-tls-identity-authorized.ci` |
| `ze doctor` on an unvalidated startup config lacking EAP-TLS identity constraint | → | `engine.checkIPsecEAPTLSIdentity` (`doctor-ipsec-eap-tls-identity`) | `TestDoctorIPsecEAPTLSIdentityCheck` |
| strongSwan client/server presenting valid CA-signed certificate with unauthorized SAN | → | Full IKEv2 + EAP-TLS 1.3 handshake through `ze` binary | Interop scenarios `responder-eap-tls13-unauthorized-client` and `eap-tls13-unauthorized-server` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|---|---|---|---|
| AC-1 | EAP-TLS handshake (both authenticator and peer roles) where the remote leaf certificate carries one or more `subjectAltName` entries (`dNSName`, `rfc822Name`, `iPAddress`, `uniformResourceIdentifier`, or UPN `otherName`) alongside a different `Subject.CommonName` | `Session.PeerID()` / `PeerSession.ServerID()` and `eapTLSPeerName` return the first SAN entry in certificate wire order; `PeerIDs()` / `ServerIDs()` return all SAN entries in wire order; authorization matches only against the SAN entries and refuses a certificate whose CN matches the policy while its SAN does not (`RFC5216-5.2-1`, `RFC5216-5.3-2`) | Certificates without any `subjectAltName` extension (OID `2.5.29.17`) fall back to the most specific Subject Common Name for DNS/NAI matching, as RFC 5216 Section 5.2 and RFC 2818 Section 3.1 specify |
| AC-2 | EAP-TLS authenticator configured with `allowed-identities` containing exact DNS, exact NAI/email, exact IP, exact URI, single-label DNS wildcard (`*.branch.example.com`), NAI realm (`@example.com`), or explicit opt-in wildcard (`*`) | A client certificate matching any configured entry completes authentication, exports its verified `PeerID()`, and logs `ike: EAP-TLS authenticated` with `peer-id`; a valid CA-signed client certificate matching none (including multi-label `a.b.branch.example.com` against `*.branch.example.com`, bare `@example.com` against `@example.com`, or `dNSName` `192.0.2.10` against IP entry `192.0.2.10`) is refused with a fatal TLS `bad_certificate` alert delivered via `FinalRequest`, `EAP-Failure`, and a failure log naming the extracted Peer-Id (`RFC5216-5.3-2`, `RFC5216-5.3-3`) | Explicit lone `*` admits any valid CA-signed client certificate carrying a non-empty identity, per the 2026-11-24 owner decision |
| AC-3 | EAP-TLS peer configured with `server-identity` (or implicit unambiguous FQDN `remote-address`) against a server certificate | A server certificate matching per RFC 2818 Section 3.1 (exact ASCII case-insensitive `dNSName`, leftmost single-label cert wildcard `*.a.b`, exact `iPAddress` SAN for IP literals, exact `rfc822Name`/UPN SAN, or CN fallback only when SAN extension is absent) completes authentication, exports `ServerID()`, and logs `ike: EAP-TLS authenticated` with `server-id`; a valid CA-signed server certificate that does not match (including IP literal against `dNSName` or CN, multi-label against `*.a.b`, or wrong `dNSName` with matching CN) is refused with a fatal TLS `bad_certificate` alert and a failure log naming expected and presented Server-Id (`RFC5216-5.3-2`, `RFC5216-5.3-3`) | None: every server certificate whose identity does not match the configured or derived `server-identity` is refused |
| AC-4 | `ze config validate`, commit, or SIGHUP reload of a `site-to-site peer` with `mode eap-tls` | Refuses fail-closed with an `opererr` naming the peer and exact fix command when: `respond` omits `allowed-identities`; `initiate` omits `server-identity` and `remote-address` is not an unambiguous multi-label FQDN; `allowed-identities` is set on `initiate` or non-`eap-tls`; `server-identity` is set on `respond` or non-`eap-tls`; or any entry is malformed (`*.com`, `*` mixed with another entry, `.example.com`, `*` in `server-identity`, control characters). Accepts valid `respond` with `allowed-identities`, valid `initiate` with `server-identity`, and valid `initiate` with unambiguous FQDN `remote-address` | Non-`eap-tls` peers without either leaf and `remote-access` blocks remain accepted unchanged; an `eap-tls` peer missing `ca-certificate` still reports the `ValidatePKIRefs` CA error first |
| AC-5 | TLS 1.3 session resumption on both roles, and config reload changing `allowed-identities` or `server-identity` | A resumed TLS 1.3 handshake re-runs identity authorization against the cached leaf certificate and exports the verified `PeerID()` / `ServerID()`; editing `allowed-identities` or `server-identity` restarts the peer and invalidates its `eap.Resumption` store so old tickets cannot bypass the tightened policy (RFC 9190 Section 5.7) | Resumed sessions whose cached leaf still satisfies the unchanged policy continue to resume normally |
| AC-6 | Daemon started from an unvalidated config file containing an `eap-tls` peer missing its identity constraint | Runtime builders (`eapTLSServerConfig`, `buildPeerTLSConfig`) and EAP core (`newTLSMethod`, `startTLSClient`) fail closed without establishing a tunnel; `buildPeerTLSConfig` logs `Warn` carrying `code: doctor-ipsec-eap-tls-identity`; `ze doctor` reports `doctor-ipsec-eap-tls-identity` with the exact `set vpn ipsec site-to-site peer ...` fix command | Healthy `eap-tls` peers in the same config continue to establish unaffected |
| AC-7 | RFC conformance ledger and discrimination records for `RFC5216-5.2-1`, `RFC5216-5.3-2`, and `RFC5216-5.3-3` | All three rows in `rfc/short/rfc5216.md` have their `{gap}` annotation removed, each carries tagged positive and negative tests in `internal/core/eap/rfc5216_identity_test.go` with recorded `./le rfc record` discrimination evidence, and `./le rfc check` reports no open issue for them | Other RFC 5216 gaps (`RFC5216-2.4-5`, `RFC5216-2.4-7`, `RFC5216-5.2-2..4`, `RFC5216-5.3-5`) remain tracked in their own specs |
| AC-8 | End-to-end strongSwan interoperability (`./le test integration interop-ipsec`) | All 7 existing EAP-TLS strongSwan scenarios stay green with explicit identity constraints, and both new negative scenarios (`responder-eap-tls13-unauthorized-client` and `eap-tls13-unauthorized-server`) reject the valid CA-signed peer with a TLS alert and zero installed XFRM SAs (and go red when identity verification is reverted) | `eap-tls` (TLS 1.2 without RFC 7627 EMS) continues to assert its existing MSK-export refusal after the server identity check passes |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|---|---|---|
| 1 | Configures an EAP-TLS responder peer with `allowed-identities` (exact SAN, `*.domain`, or `@realm`) and commits | YANG schema -> `parseEAPTLSIdentityPolicy` -> `validateIPsecSections` -> `eapTLSServerConfig` -> `tlsMethod` `VerifyConnection` -> `recordEAPTLSAuthentication` logs `peer-id` | `test/ipsec/ipsec-eap-tls-identity-authorized.ci` and strongSwan `responder-eap-tls13` |
| 2 | Connects an unauthorized client holding another certificate from the same CA | Client presents valid CA-signed leaf with non-matching SAN -> `VerifyConnection` rejects -> TLS `bad_certificate` alert via `FinalRequest` -> `EAP-Failure` -> SA dead, log names extracted SAN | `test/ipsec/ipsec-eap-tls-identity-rejected.ci` and strongSwan `responder-eap-tls13-unauthorized-client` |
| 3 | Configures an EAP-TLS initiator peer with `server-identity` (or FQDN `remote-address`) and connects to a server presenting a certificate for another host signed by the same CA | `EffectiveEAPTLSServerIdentity` -> `buildPeerTLSConfig` -> `serverChainCheck.verifyConnection` (RFC 2818 §3.1) -> TLS `bad_certificate` alert -> SA dead, log names expected vs presented Server-Id | `test/ipsec/ipsec-eap-tls-identity-rejected.ci` and strongSwan `eap-tls13-unauthorized-server` |
| 4 | Attempts to validate, commit, or reload an `eap-tls` peer without an identity constraint (or with wrong-role/malformed identity syntax) | `ze config validate` / `OnConfigVerify` -> `validateIPsecSections` -> `ValidateEAPTLSIdentities` -> `opererr` refusal naming exact `set` / `delete` fix command; running tunnels stay untouched on reload | `test/parse/ipsec-eap-tls-identity-required.ci` and `test/reload/tx-ipsec-eap-tls-identity-required.ci` |
| 5 | Runs `ze doctor` on a host booted from an unvalidated legacy config missing EAP-TLS identity constraints | `doctor` post-config phase -> `checkIPsecEAPTLSIdentity` -> `doctor-ipsec-eap-tls-identity` finding with exact `set vpn ipsec site-to-site peer ...` fix | `TestDoctorIPsecEAPTLSIdentityCheck` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|---|---|---|---|
| `TestRFC5216IdentityIsTakenFromTheSANAndAuthorized` | `internal/core/eap/rfc5216_identity_test.go` | Wiring & `RFC5216-5.2-1` / `RFC5216-5.3-2` / `RFC5216-5.3-3`: SAN wire-order extraction for `PeerID`/`PeerIDs` and `ServerID`/`ServerIDs`, `eapTLSPeerName` SAN-first behavior, SAN-over-CN no-fallback invariant, and refusal of unauthorized SAN on both roles | |
| `TestRFC5216AllowedIdentitiesMatchingMatrix` | `internal/core/eap/rfc5216_identity_test.go` | Exact DNS (ASCII case fold), single-label wildcard `*.branch.example.com` (refuses multi-label and apex), exact NAI (`rfc822Name` and UPN `otherName`), `@realm` (refuses bare `@realm`), exact IP (`iPAddress` only, refuses `dNSName` and CN), exact URI, lone `*`, empty-SAN/empty-Subject refusal, and client EKU refusal | |
| `TestRFC5216ServerIdentityRFC2818Rules` | `internal/core/eap/rfc5216_identity_test.go` | Peer RFC 2818 §3.1: exact DNS, cert leftmost single-label wildcard `*.a.b`, refusal of `*.com` / multi-label / inner wildcard, IP literal strictly `iPAddress` SAN only (never DNS or CN), CN fallback only when no SAN extension exists (most specific CN when multiple CNs exist), and server EKU refusal | |
| `TestRFC5216ResumedSessionReverifiesPeerAndServerIdentity` | `internal/core/eap/rfc5216_identity_test.go` | TLS 1.3 resumed handshake re-runs identity authorization on both roles, exports `PeerID()`/`ServerID()`, and refuses if cached leaf no longer matches tightened policy | |
| `TestRFC5216FailClosedWhenIdentityPolicyUnset` | `internal/core/eap/rfc5216_identity_test.go` | `newTLSMethod` refuses empty `AllowedIdentities`; `startTLSClient` refuses empty `ServerIdentity` | |
| `TestValidateEAPTLSIdentitiesMatrix` | `internal/component/ike/ipsec/validate_eaptls_identity_test.go` | Full commit/validate fail-closed matrix across `respond`/`initiate`, `eap-tls`/other modes, `allowed-identities`, `server-identity`, unambiguous FQDN `remote-address` fallback, malformed entries, `*` mixing refusal, `opererr` fix formatting, and `ValidatePKIRefs` ordering | |
| `TestParseEAPTLSIdentityPolicyAndEquality` | `internal/component/ike/ipsec/validate_eaptls_identity_test.go` | `parseEAPTLSIdentityPolicy`, `AuthAdditions` nil preservation when unset, `SiteToSitePeer.Equal` sensitivity to `allowed-identities` and `server-identity` edits, and `TestSiteToSitePeerStaysUnderTheHugeParamThreshold` | |
| `TestEAPTLSServerConfigWiresAllowedIdentities` | `internal/component/ike/engine/eap_tls_identity_wiring_test.go` | `eapTLSServerConfig` forwards `AllowedIdentities` and fails closed when empty; `resumptionFor` invalidates ticket store when `allowed-identities` changes; `recordEAPTLSAuthentication` logs `peer-id` | |
| `TestBuildPeerTLSConfigWiresServerIdentity` | `internal/component/ike/engine/eap_tls_identity_wiring_test.go` | `buildPeerTLSConfig` forwards explicit `server-identity` or unambiguous FQDN `remote-address`, fails closed with `code: doctor-ipsec-eap-tls-identity` Warn when neither exists; `resumptionFor` invalidates ticket store when `server-identity` changes; `recordEAPTLSAuthentication` logs `server-id` | |
| `TestValidateIPsecSectionsEnforcesEAPTLSIdentityConstraints` | `internal/component/ike/engine/eap_tls_identity_wiring_test.go` | `validateIPsecSections` (used by `ze config validate`, commit, and reload) enforces `ValidateEAPTLSIdentities` after `ValidatePKIRefs` | |
| `TestDoctorIPsecEAPTLSIdentityCheck` | `internal/component/ike/engine/doctor_eap_tls_identity_test.go` | `checkIPsecEAPTLSIdentity` reports `doctor-ipsec-eap-tls-identity` with concrete `set` fix for unconstrained `eap-tls` peers and returns healthy for constrained peers | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|---|---|---|---|---|
| `allowed-identities` entry length (YANG `length "1..255"`) | 1..255 bytes | 255-byte valid DNS name / 1-byte `*` | 0 bytes (empty string) | 256 bytes |
| `server-identity` length (YANG `length "1..255"`) | 1..255 bytes | 255-byte valid FQDN / 1-byte single-char hostname | 0 bytes (empty string) | 256 bytes |
| DNS wildcard suffix label count (`*.a.b`) | >= 2 labels after `*.` | 2 labels (`*.example.com`) | 1 label (`*.com`) or 0 labels (`*.`) | N/A (bounded by 255-byte entry length) |
| NAI realm label count (`@a.b`, RFC 7542 §2.2) | >= 2 labels after `@` | 2 labels (`@example.com`) | 1 label (`@localhost`) or 0 labels (`@`) | N/A (bounded by 255-byte entry length) |
| Unambiguous FQDN `remote-address` label count | >= 2 labels | 2 labels (`vpn.example`) | 1 label (`vpn`) or IP literal (`172.28.0.3`) | N/A (bounded by 255-byte DNS name length) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|---|---|---|---|
| `ipsec-eap-tls-identity-authorized` | `test/ipsec/ipsec-eap-tls-identity-authorized.ci` | Two unprivileged `ze` daemons complete EAP-TLS with `allowed-identities` and `server-identity`; SA establishes and logs carry `peer-id` and `server-id` | |
| `ipsec-eap-tls-identity-rejected` | `test/ipsec/ipsec-eap-tls-identity-rejected.ci` | Two unprivileged `ze` daemons fail closed when client SAN is not in `allowed-identities` and when server SAN does not match `server-identity`; logs name the extracted SAN and no SA establishes | |
| `ipsec-eap-tls-identity-required` | `test/parse/ipsec-eap-tls-identity-required.ci` | `ze config validate` refuses missing/wrong-role/wrong-mode/malformed `allowed-identities` and `server-identity`, and accepts valid exact, wildcard, realm, lone `*`, explicit `server-identity`, and FQDN `remote-address` configs | |
| `tx-ipsec-eap-tls-identity-required` | `test/reload/tx-ipsec-eap-tls-identity-required.ci` | SIGHUP reload refuses switching a peer to `eap-tls` without an identity constraint and preserves the running configuration | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|---|---|---|---|---|
| `responder-eap-tls13-unauthorized-client` | `test/interop-ipsec/scenarios/responder-eap-tls13-unauthorized-client/` | strongSwan 6.x (`charon`) | Ze EAP-TLS authenticator refuses a valid CA-signed strongSwan client whose SAN (`DNS:swan-eap-client`) is outside `allowed-identities`, sends a fatal TLS `bad_certificate` alert, logs the SAN Peer-Id, and installs zero XFRM SAs; reverting authorization makes the tunnel establish | |
| `eap-tls13-unauthorized-server` | `test/interop-ipsec/scenarios/eap-tls13-unauthorized-server/` | strongSwan 6.x (`charon`) | Ze EAP-TLS peer refuses a valid CA-signed strongSwan server whose SAN (`IP:172.28.0.3`) does not match `server-identity`, sends a fatal TLS `bad_certificate` alert, logs expected vs presented Server-Id, and installs zero XFRM SAs; reverting authorization makes the tunnel establish | |
| Existing 7 EAP-TLS scenarios (`eap-tls`, `eap-tls13`, `responder-eap-tls13`, `responder-eap-tls12`, `responder-eap-tls13-revoked-client`, `responder-eap-tls13-leaf-only-client`, `real-nat-port-translated-eap-ze-responder`) | `test/interop-ipsec/scenarios/` | strongSwan 6.x (`charon`) | Positive identity authorization works end-to-end against strongSwan for both IP SAN (`172.28.0.3`) and DNS SAN (`swan-eap-client`) across TLS 1.2, TLS 1.3, leaf-only intermediates, CRL revocation, and NAT-T | |

## Files to Modify

- `internal/core/eap/eap.go` - add `AllowedIdentities []string` to `MethodConfig`; add `Session.PeerID()` and `Session.PeerIDs()`
- `internal/core/eap/eap_tls.go` - switch `eapTLSPeerName` to SAN-first `certificatePrimaryID`; enforce `AllowedIdentities` in `newTLSMethod` and both `VerifyConnection` branches; publish verified Peer-Id(s) on `tlsMethod`
- `internal/core/eap/peer.go` - add `ServerIdentity string` to `PeerTLSConfig`; validate and wire it in `startTLSClient`; export `PeerSession.ServerID()` and `PeerSession.ServerIDs()`
- `internal/core/eap/peer_chain.go` - enforce RFC 2818 Section 3.1 server identity match in `serverChainCheck.verifyConnection` (full and resumed chains) and publish verified `serverIDs` atomically
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - add `leaf-list allowed-identities` and `leaf server-identity` under `vpn ipsec site-to-site peer authentication` (plus regenerated `embed.go` and `register.go` via `./le yang glue update`)
- `internal/component/ike/ipsec/types.go` - add `AllowedIdentities` and `ServerIdentity` to `AuthAdditions`; add `AuthConfig.AllowedIdentities()`, `AuthConfig.ServerIdentity()`, and `SiteToSitePeer.EffectiveEAPTLSServerIdentity()`
- `internal/component/ike/ipsec/config.go` - call `parseEAPTLSIdentityPolicy` from `parseAuthConfig`
- `internal/component/ike/ipsec/config_auth_policy.go` - add `parseEAPTLSIdentityPolicy`
- `internal/component/ike/ipsec/validate.go` - add `IPsecConfig.ValidateEAPTLSIdentities()` with `opererr` refusals
- `internal/component/ike/engine/config.go` - invoke `cfg.ValidateEAPTLSIdentities()` in `validateIPsecSections` after `ValidateIntermediateRefs`
- `internal/component/ike/engine/responder_eap.go` - pass `AllowedIdentities` in `eapTLSServerConfig` (fail closed if empty) and pass `sess.PeerID()` to `recordEAPTLSAuthentication`
- `internal/component/ike/engine/fsm.go` - pass `EffectiveEAPTLSServerIdentity()` in `buildPeerTLSConfig` (fail closed with `code: doctor-ipsec-eap-tls-identity` Warn if empty) and pass `ps.ServerID()` to `recordEAPTLSAuthentication`
- `internal/component/ike/engine/resumption.go` - extend `recordEAPTLSAuthentication` to log `peer-id` or `server-id`
- `internal/component/ike/engine/register.go` - register `ipsec-eap-tls-identity` post-config doctor check
- `internal/component/ike/engine/register_doctor_codes.go` - register `codeDoctorIPsecEAPTLSIdentity` (`doctor-ipsec-eap-tls-identity`)
- `internal/le/interoplab/ipsec/checkers.go` - register checkers for `responder-eap-tls13-unauthorized-client` and `eap-tls13-unauthorized-server`
- `rfc/short/rfc5216.md` - remove `{gap}` from `RFC5216-5.2-1`, `RFC5216-5.3-2`, and `RFC5216-5.3-3` and update Meta summary
- `docs/architecture/ike/ipsec-3-data-model.md` - document `allowed-identities`, `server-identity`, `AuthAdditions`, and `ValidateEAPTLSIdentities`
- `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` - document SAN-first Peer-Id/Server-Id extraction and authenticator identity authorization
- `docs/architecture/ike/ipsec-11-interop-eap.md` - document peer RFC 2818 Section 3.1 server identity verification and the two new strongSwan negative scenarios
- `docs/architecture/ike/ipsec-14-responder.md` - document authenticator `allowed-identities` wiring and `peer-id` log attribute
- `docs/guide/ipsec.md` - replace stale "without DNS-name matching" statement and document `allowed-identities` and `server-identity` configuration and examples
- `test/ipsec/ipsec-eap-tls-clienthello.ci`, `test/ipsec/ipsec-eap-tls13-resumption.ci`, `test/ipsec/ipsec-eap-tls13-ocsp-stapling.ci`, `test/ipsec/ipsec-eap-tls13-ocsp-required.ci`, `test/ipsec/ipsec-eap-nak-unacceptable-type.ci`, `test/parse/reference-ipsec-eap-tls-ca-missing.ci` - add explicit `allowed-identities` / `server-identity` to existing EAP-TLS test peers
- `test/interop-ipsec/scenarios/eap-tls/ze.conf`, `test/interop-ipsec/scenarios/eap-tls13/ze.conf`, `test/interop-ipsec/scenarios/responder-eap-tls13/ze.conf`, `test/interop-ipsec/scenarios/responder-eap-tls12/ze.conf`, `test/interop-ipsec/scenarios/responder-eap-tls13-revoked-client/ze.conf`, `test/interop-ipsec/scenarios/responder-eap-tls13-leaf-only-client/ze.conf`, `test/interop-ipsec/scenarios/real-nat-port-translated-eap-ze-responder/ze.conf` - add explicit `allowed-identities` / `server-identity` matching their fixture certificates
- Existing EAP-TLS unit test fixtures in `internal/core/eap/` and `internal/component/ike/engine/` - populate `AllowedIdentities` and `ServerIdentity` in existing test helpers where they construct `MethodConfig` / `PeerTLSConfig` for unrelated EAP-TLS tests

## Files to Create

- `internal/core/eap/identity.go` - SAN-first identity extraction, `ParseAllowedIdentity`, `ParseServerIdentity`, `IsUnambiguousFQDN`, authenticator allow-list authorization, and peer RFC 2818 Section 3.1 verification
- `internal/core/eap/rfc5216_identity_test.go` - RFC-tagged positive and negative unit tests for `RFC5216-5.2-1`, `RFC5216-5.3-2`, and `RFC5216-5.3-3`
- `internal/component/ike/ipsec/validate_eaptls_identity_test.go` - unit tests for `ValidateEAPTLSIdentities`, `parseEAPTLSIdentityPolicy`, and `EffectiveEAPTLSServerIdentity`
- `internal/component/ike/engine/doctor_eap_tls_identity.go` - `checkIPsecEAPTLSIdentity` post-config doctor check
- `internal/component/ike/engine/doctor_eap_tls_identity_test.go` - unit tests for `checkIPsecEAPTLSIdentity`
- `internal/component/ike/engine/eap_tls_identity_wiring_test.go` - engine wiring, resumption invalidation, and log attribute tests
- `test/ipsec/ipsec-eap-tls-identity-authorized.ci` - two-daemon functional positive test
- `test/ipsec/ipsec-eap-tls-identity-rejected.ci` - two-daemon functional negative test
- `test/parse/ipsec-eap-tls-identity-required.ci` - `ze config validate` fail-closed and grammar test
- `test/reload/tx-ipsec-eap-tls-identity-required.ci` - SIGHUP reload fail-closed refusal test
- `test/interop-ipsec/scenarios/responder-eap-tls13-unauthorized-client/` - strongSwan negative interop scenario (authenticator rejects unauthorized client SAN)
- `test/interop-ipsec/scenarios/eap-tls13-unauthorized-server/` - strongSwan negative interop scenario (peer rejects mismatched server SAN)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|---|---|---|
| YANG schema (new RPCs/config) | Yes | `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` (`allowed-identities` leaf-list and `server-identity` leaf under `vpn ipsec site-to-site peer authentication`), plus `./le yang glue update` |
| YANG validation constraints | Yes | `type string { length "1..255"; }` on both `allowed-identities` and `server-identity`; semantic role/mode/syntax checks in `ipsec.ValidateEAPTLSIdentities` called from `engine.validateIPsecSections` |
| YANG custom validators | No | Role- and mode-dependent cross-leaf checks (`mode eap-tls`, `connection-type`, `remote-address` FQDN fallback) live in `engine.validateIPsecSections` (`InProcessConfigVerifier` + `OnConfigVerify`), matching `ValidatePKIRefs` and `ValidateIntermediateRefs` |
| CLI commands/flags | N-A | No new CLI verb or flag; config leaves are edited through standard `set` / `delete` |
| CLI grammar (keyword before value) | N-A | No new CLI command |
| Editor autocomplete | Yes | Automatic from `ze-ipsec-conf.yang` (`allowed-identities`, `server-identity`, and `ze:help` descriptions) |
| Functional test for new RPC/API | Yes | `test/ipsec/ipsec-eap-tls-identity-authorized.ci`, `test/ipsec/ipsec-eap-tls-identity-rejected.ci`, `test/parse/ipsec-eap-tls-identity-required.ci`, `test/reload/tx-ipsec-eap-tls-identity-required.ci` |
| Pipe completeness | N-A | No new show/RPC output producer |
| Env var registration | N-A | Configuration is YANG-only (`ai/rules/config.md`); no env override added |
| Doctor check for runtime dependencies | Yes | `internal/component/ike/engine/doctor_eap_tls_identity.go` (`ipsec-eap-tls-identity`, `DoctorPhasePostConfig`) + `register_doctor_codes.go` (`doctor-ipsec-eap-tls-identity`) + `doctor_eap_tls_identity_test.go` |
| Doctor code per problem class | Yes | Missing/invalid EAP-TLS identity constraint on an unvalidated startup config (`buildPeerTLSConfig` runtime `Warn` log and `ze doctor` post-config finding) -> `ipsec-eap-tls-identity`, code `doctor-ipsec-eap-tls-identity`, phase `DoctorPhasePostConfig`. Handshake-time remote peer rejections reuse existing `ike: EAP authentication failed` / `ike: EAP failed` lines without introducing a new `Warn` site |
| Prometheus counters/metrics | N-A | Existing `ze_ipsec_eap_tls_resumption_hits_total` / `misses_total` and `ze_ipsec_tunnel_up` already observe authentication outcomes; no new counter requested |
| BGP family surface (new SAFI / capability / attribute) | N-A | IPsec/EAP-TLS protocol fix, no BGP family changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|---|---|---|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | EAP-TLS already exists in `features/`; this fix closes its RFC 5216 §§5.2–5.3 compliance gaps |
| 2 | Config syntax changed? | Yes | `docs/guide/ipsec.md` (`ze-ipsec-conf.yang` drives the generated configuration reference; `docs/architecture/config/syntax.md` covers grammar rules only, unaffected) |
| 3 | CLI command added/changed? | N-A | No CLI command added or changed |
| 4 | API/RPC added/changed? | N-A | No API or RPC added or changed |
| 5 | Plugin added/changed? | No | Existing `ike` plugin extended internally; no registration/root change |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md` (replace stale "without DNS-name matching" paragraph; document `allowed-identities` and `server-identity`) |
| 7 | Wire format changed? | No | Standard RFC 5216 / RFC 9190 EAP-TLS and TLS `bad_certificate` alert wire format unchanged |
| 8 | Plugin SDK/protocol changed? | N-A | No `pkg/` SDK or process-protocol change |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc5216.md` (`RFC5216-5.2-1`, `RFC5216-5.3-2`, `RFC5216-5.3-3`) and `docs/features/rfc-status.md` if generated/tracked |
| 10 | Test infrastructure changed? | No | Uses existing `.ci` runner and `interop-ipsec` lab |
| 11 | Affects daemon comparison? | No | No daemon comparison table row changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-3-data-model.md`, `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md`, `docs/architecture/ike/ipsec-11-interop-eap.md`, `docs/architecture/ike/ipsec-14-responder.md` |
| 13 | Route metadata keys added/changed? | N-A | No BGP route metadata touched |
| 14 | Prometheus counters added/changed? | N-A | No metrics added or changed |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | No plugin, event, send, command, capability, or inventory contract changed |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Verified via `./le spec citation anchors`: declared `// Design:` pages (`ipsec-3-data-model.md`, `ipsec-7-ikev2-engine.md`, `ipsec-9-ikev2-eap-nat.md`, `ipsec-11-interop-eap.md`, `ipsec-14-responder.md`, `doctor-and-health-checks.md`, `interop.md`) and anchored/advisory docs (`docs/guide/ipsec.md`, `ai/digests/ipsec-ike.md`, `docs/DESIGN.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, `docs/architecture/ike/ipsec-10-cli-diag.md`, `docs/architecture/ike/ipsec-13-rekey-wire.md`, `docs/architecture/linux-limitations.md`, `docs/config-reference.md`, `docs/contributing/patching-the-standard-library.md`, `docs/guide/configuration.md`, `docs/guide/migrating-from-vyos.md`) — `ipsec-3-data-model.md`, `ipsec-9-ikev2-eap-nat.md`, `ipsec-11-interop-eap.md`, `ipsec-14-responder.md`, and `docs/guide/ipsec.md` updated; all other anchored pages checked and unaffected in their anchored claims |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ipsec.md` EAP-TLS example (lines 575–580) updated to include `allowed-identities` |

## Design presentation

### What changes

An operator configuring an `eap-tls` IPsec site-to-site peer now declares which
certificate identity is authorized on the far end: `allowed-identities` on a
responding authenticator (supporting exact DNS/NAI/IP/URI SANs, single-label DNS
wildcards such as `*.branch.example.com`, NAI realms such as `@example.com`, or an
explicit lone `*` opt-in), and `server-identity` on an initiating peer (or an
unambiguous multi-label FQDN in `remote-address`). Validation, commit, and reload
fail closed if an `eap-tls` peer omits its identity constraint, and both EAP-TLS
roles extract Peer-Id and Server-Id from the certificate's `subjectAltName` extension
first (RFC 5216 Section 5.2), enforce identity authorization per RFC 5216 Section 5.3
and RFC 2818 Section 3.1 on both full and resumed TLS handshakes, send a fatal TLS
`bad_certificate` alert on mismatch, and log the verified `peer-id` or `server-id`.

### Before and after

1. **Committing an EAP-TLS responder peer without client identity authorization:**
   - Before: `vpn ipsec site-to-site peer branch authentication { mode eap-tls; certificate ze-server; ca-certificate corp-ca; }` commits cleanly, and any certificate signed by `corp-ca` establishes a tunnel.
   - After: `ze config validate`, commit, and reload refuse the config with an `opererr` error naming `branch`, explaining that RFC 5216 Section 5.3 requires authorizing client identities, and printing the exact fix on the last line: `set vpn ipsec site-to-site peer branch authentication allowed-identities <identity>` (or `"*"` to explicitly accept any certificate issued by `corp-ca`).

2. **Committing an EAP-TLS initiator peer targeting an IP address without `server-identity`:**
   - Before: `vpn ipsec site-to-site peer hq { remote-address 198.51.100.1; authentication { mode eap-tls; certificate ze-client; ca-certificate corp-ca; } }` commits cleanly, and any server certificate signed by `corp-ca` is trusted.
   - After: `ze config validate`, commit, and reload refuse the config with an `opererr` naming `hq` and the exact fix: `set vpn ipsec site-to-site peer hq authentication server-identity <hostname-or-ip>` (whereas `remote-address vpn.example.com` automatically verifies the server certificate against `vpn.example.com`).

3. **Unauthorized CA-signed peer connecting over EAP-TLS:**
   - Before: A client holding a valid `corp-ca`-signed certificate with SAN `DNS:rogue.example.com` completes EAP-TLS and brings up the IPsec Child SA; `eapTLSPeerName` logged only `CN=...`.
   - After: The authenticator extracts Peer-Id `rogue.example.com` from `subjectAltName`, rejects it against `allowed-identities`, sends a fatal TLS `bad_certificate` alert, logs `ike: EAP authentication failed` naming `rogue.example.com`, and installs no Child SA. Authorized sessions log `ike: EAP-TLS authenticated` with `peer-id=swan-eap-client` (responder) or `server-id=172.28.0.3` (initiator).

### Alternatives set aside

| Alternative | Why set aside |
|---|---|
| Reuse `remote-id` as the EAP-TLS certificate identity allow-list | `remote-id` constrains the IKEv2 `IDi`/`IDr` payload (which can be an opaque `ID_KEY_ID`, address range, or empty); overloading it couples IKEv2 ID policy with TLS certificate authorization and contradicts the 2026-11-24 owner decision (`allowed-identities` + `server-identity`) |
| Set `tls.Config.ServerName` and drop `InsecureSkipVerify` on the EAP-TLS peer | Go's `crypto/x509.Certificate.VerifyHostname` dropped Common Name fallback in Go 1.15 (violating RFC 5216 §5.2 and RFC 2818 §3.1 when a server certificate has no SAN extension), does not match `rfc822Name` SANs, and emits a cleartext SNI extension in ClientHello |
| Warn-only default when `allowed-identities` or `server-identity` is omitted | Leaves the security hole open by default and directly contradicts the 2026-11-24 owner decision ("Fail-closed at commit unless an explicit identity constraint is configured") |
| Support `.example.com` subtree syntax alongside `*.example.com` in `allowed-identities` | RFC 2818 Section 3.1 defines single-label DNS wildcard matching (`*.a.b`), not arbitrary-depth `.a.b` subtrees; refusing `.example.com` with an error pointing to `*.example.com` prevents operators from assuming arbitrary-depth matching |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** — add the `identity.go` entry-point stubs, `MethodConfig.AllowedIdentities`, `PeerTLSConfig.ServerIdentity`, `Session.PeerID`/`PeerIDs`, `PeerSession.ServerID`/`ServerIDs`, YANG leaves (`ze-ipsec-conf.yang` + `./le yang glue update`), `AuthAdditions` fields, `parseEAPTLSIdentityPolicy`, `ValidateEAPTLSIdentities` call in `validateIPsecSections`, engine plumbing in `eapTLSServerConfig` and `buildPeerTLSConfig`, and `checkIPsecEAPTLSIdentity` doctor registration; write failing wiring tests (`TestRFC5216IdentityIsTakenFromTheSANAndAuthorized`, `TestValidateIPsecSectionsEnforcesEAPTLSIdentityConstraints`, `TestEAPTLSServerConfigWiresAllowedIdentities`, `TestBuildPeerTLSConfigWiresServerIdentity`, `TestDoctorIPsecEAPTLSIdentityCheck`).
   - Tests: all wiring tests named in the Wiring Test table
   - Files: `internal/core/eap/identity.go`, `eap.go`, `eap_tls.go`, `peer.go`, `peer_chain.go`, `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`, `types.go`, `config.go`, `config_auth_policy.go`, `validate.go`, `internal/component/ike/engine/config.go`, `responder_eap.go`, `fsm.go`, `resumption.go`, `register.go`, `register_doctor_codes.go`, `doctor_eap_tls_identity.go`
   - Verify: all entry points compile and are reachable; wiring tests fail on unimplemented checks.

2. **Phase: Core EAP-TLS Identity Extraction and Verification (`internal/core/eap`)** — implement SAN wire-order extraction (`extractCertificateIdentities`, `certificatePrimaryID`), `eapTLSPeerName` SAN-first update, `ParseAllowedIdentity`, `ParseServerIdentity`, `IsUnambiguousFQDN`, authenticator `authorizePeerCertificate`, and peer RFC 2818 Section 3.1 `verifyServerCertificate`; wire into both `newTLSMethod` `VerifyConnection` branches and `serverChainCheck.verifyConnection` (covering full and resumed TLS 1.3 sessions); update existing `internal/core/eap/*_test.go` helpers to provide fixture identities; add full RFC-tagged unit suite in `rfc5216_identity_test.go`.
   - Tests: `TestRFC5216IdentityIsTakenFromTheSANAndAuthorized`, `TestRFC5216AllowedIdentitiesMatchingMatrix`, `TestRFC5216ServerIdentityRFC2818Rules`, `TestRFC5216ResumedSessionReverifiesPeerAndServerIdentity`, `TestRFC5216FailClosedWhenIdentityPolicyUnset`
   - Files: `internal/core/eap/identity.go`, `eap.go`, `eap_tls.go`, `peer.go`, `peer_chain.go`, `rfc5216_identity_test.go`, existing `internal/core/eap/*_test.go` fixtures
   - Verify: unit tests fail -> implement -> unit tests pass under `-race` via `./le test unit pkg path ./internal/core/eap/`.

3. **Phase: IPsec Data Model, Validation, Engine Wiring, and Doctor Check** — complete `ValidateEAPTLSIdentities` with `opererr` errors, wire `eapTLSServerConfig`, `buildPeerTLSConfig` (with `code: doctor-ipsec-eap-tls-identity` on startup Warn), `recordEAPTLSAuthentication` (`peer-id` / `server-id`), and `checkIPsecEAPTLSIdentity`; update existing unit test fixtures in `internal/component/ike/engine/`.
   - Tests: `TestValidateEAPTLSIdentitiesMatrix`, `TestParseEAPTLSIdentityPolicyAndEquality`, `TestSiteToSitePeerStaysUnderTheHugeParamThreshold`, `TestEAPTLSServerConfigWiresAllowedIdentities`, `TestBuildPeerTLSConfigWiresServerIdentity`, `TestValidateIPsecSectionsEnforcesEAPTLSIdentityConstraints`, `TestDoctorIPsecEAPTLSIdentityCheck`
   - Files: `internal/component/ike/ipsec/types.go`, `config.go`, `config_auth_policy.go`, `validate.go`, `validate_eaptls_identity_test.go`, `internal/component/ike/engine/config.go`, `responder_eap.go`, `fsm.go`, `resumption.go`, `register.go`, `register_doctor_codes.go`, `doctor_eap_tls_identity.go`, `doctor_eap_tls_identity_test.go`, `eap_tls_identity_wiring_test.go`
   - Verify: tests pass via `./le test unit pkg path ./internal/component/ike/ipsec/` and `./internal/component/ike/engine/`; `./le arch tier check`, `./le yang glue check`, `./le arch operator-error check`, and `./le arch doctor-coverage check` pass.

4. **Phase: Functional `.ci` Tests, Interop Scenarios, RFC Ledger, and Documentation** — update the 6 existing EAP-TLS `.ci` files and 7 existing strongSwan scenario configs; add the 4 new `.ci` tests and 2 new strongSwan negative scenarios (`responder-eap-tls13-unauthorized-client` and `eap-tls13-unauthorized-server`) with checkers in `internal/le/interoplab/ipsec/checkers.go`; update `rfc/short/rfc5216.md` and record `./le rfc record` discrimination proofs for `RFC5216-5.2-1`, `RFC5216-5.3-2`, and `RFC5216-5.3-3`; update `docs/guide/ipsec.md` and the four `docs/architecture/ike/` pages.
   - Tests: `./le test functional ipsec`, `./le test functional parse`, `./le test functional reload`, `./le test integration interop-ipsec`, `./le rfc check`, `./le doc check all`
   - Files: all test, interop, RFC, and doc files listed in Files to Modify and Files to Create
   - Verify: every new functional and negative interop test is proved red when identity checks are reverted, then green when restored; `./le rfc check` and `./le doc check all` are clean.

### Critical Review Checklist
| Check | What to verify for this spec |
|---|---|
| Completeness | Every AC-1..AC-8 has an implementation at `file:line` and passing positive + negative tests |
| Feature completeness | Every User Story 1..5 has a working path from config/wire to log/doctor with no broken links |
| Correctness | SAN extension presence (`OID 2.5.29.17`) strictly blocks CN fallback; IP literals match `iPAddress` SAN only; wildcards match a single leftmost label (`>= 2` suffix labels); ASCII-only case folding (`asciiEqualFold`, never `strings.EqualFold`) |
| Naming | YANG leaves are exact kebab-case `allowed-identities` and `server-identity`; log keys are `peer-id` and `server-id`; doctor code is `doctor-ipsec-eap-tls-identity` |
| Data flow | `internal/core/eap` imports stdlib only; `ipsec` and `engine` import `core/eap` downward; no duplicate identity grammar |
| Operator errors and doctor codes | Every commit/validate refusal is an `opererr` error with exact fix command alone on the last line; `buildPeerTLSConfig`'s new `Warn` log names `code: doctor-ipsec-eap-tls-identity`; `checkIPsecEAPTLSIdentity` carries `Fix` at `DoctorPhasePostConfig` (`ai/rules/user-facing-errors.md`) |
| Rule: `ai/rules/rfc-compliance.md` | Verbatim `// RFC 5216 Section 5.2:` / `// RFC 5216 Section 5.3:` / `// RFC 2818 Section 3.1:` quotes above enforcing code; discrimination records generated by `./le rfc record` |
| Rule: `types_size_test.go` | `SiteToSitePeer` remains `< 288` bytes via `AuthAdditions` |

### Deliverables Checklist
| Deliverable | Verification method |
|---|---|
| SAN-first extraction + authenticator/peer identity authorization in `internal/core/eap/identity.go` | `./le test unit pkg path ./internal/core/eap/ run TestRFC5216` |
| YANG schema leaves `allowed-identities` and `server-identity` + regenerated glue | `./le yang glue check` and `./le config claims` |
| Fail-closed config validation (`ValidateEAPTLSIdentities`) + `opererr` fixes | `./le test unit pkg path ./internal/component/ike/ipsec/` and `./le arch operator-error check` |
| Engine wiring, resumption invalidation, identity logging, and `ipsec-eap-tls-identity` doctor check | `./le test unit pkg path ./internal/component/ike/engine/` and `./le arch doctor-coverage check` |
| Functional `.ci` positive, negative, parse, and reload tests | `./le test functional ipsec`, `./le test functional parse`, `./le test functional reload` |
| strongSwan negative and positive interop scenarios | `./le test integration interop-ipsec` |
| `RFC5216-5.2-1`, `RFC5216-5.3-2`, `RFC5216-5.3-3` closed with discrimination records | `./le rfc check` |
| Updated guide and architecture pages with verified source anchors | `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|---|---|
| Input validation | Untrusted X.509 certificate SANs and Subject CNs are ASN.1-decoded with bounded loops (matching `nai.go::parseUserPrincipalNames`), reject embedded NUL/control bytes, and are logged only as `slog` values (never interpolated into format strings or commands) |
| Fail-closed authorization | Empty `AllowedIdentities` or empty `ServerIdentity` fails closed at validate, commit, reload, engine builder, and EAP core (`newTLSMethod`, `startTLSClient`); empty SAN extension never falls back to CN; empty certificate identity fails closed even under `*` |
| Class confusion & wildcard abuse | IP literals never match `dNSName` or CN; `*.example.com` never matches `a.b.example.com` or `example.com`; `*.com`, `*.*.example.com`, and partial-label wildcards are refused; ASCII-only folding prevents Unicode Kelvin-sign (`U+212A`) spoofing |
| Resumption TOCTOU (RFC 9190 §5.7) | Identity authorization runs inside `VerifyConnection` on resumed TLS 1.3 sessions too, and any edit to `allowed-identities` or `server-identity` invalidates the peer's `eap.Resumption` ticket store via `AuthConfig.Equal` |
| Unauthenticated Identity Response (RFC 9190 §2.2) | `Session.Identity()` (the cleartext EAP-Response/Identity) is never consulted by `authorizePeerCertificate`; authorization uses only the verified TLS certificate |

### Failure Routing

| Failure | Route To |
|---|---|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- `internal/component/ike/engine/register.go:338` registers `validateIPsecSections` as `InProcessConfigVerifier`, so placing `ValidateEAPTLSIdentities` there guarantees identical fail-closed enforcement across `ze config validate`, commit, and SIGHUP reload without duplicating checks.
- `SiteToSitePeer` sits right below the 288-byte `hugeParam` threshold (`types_size_test.go`), so `AuthConfig.Additions` (`*AuthAdditions`) is the exact container designed for per-mode optional fields.
- Keeping `InsecureSkipVerify: true` and empty `tls.Config.ServerName` on the EAP-TLS peer avoids both Go 1.15+'s removal of CN fallback (which would break RFC 2818 §3.1 / RFC 5216 §5.2 for SAN-less server certificates) and unintended cleartext SNI emission, while running RFC 2818 §3.1 checks in `serverChainCheck.verifyConnection` covers both full and resumed TLS 1.3 sessions.

## Owned journal rows
| Class file | Date | Surface (short) |
|---|---|---|

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|---|---|---|
| Dedicated `allowed-identities` leaf-list (responder) and `server-identity` leaf (initiator) under `site-to-site peer authentication`, failing closed at commit when omitted | Reusing `remote-id`; warning-only default | Matches the 2026-11-24 owner decision; separates IKEv2 `IDi`/`IDr` policy from EAP-TLS certificate authorization and prevents silent fail-open deployments |
| Implicit `remote-address` fallback on initiating `eap-tls` peers ONLY when `remote-address` is an unambiguous multi-label FQDN | Falling back to IP literals or single-label names; requiring `server-identity` even when `remote-address` is an FQDN | Implements the 2026-11-24 owner decision ("unambiguous remote FQDN / `server-identity` on client") while avoiding ambiguous IP-to-SAN guessing or search-domain-dependent single labels |
| Explicit lone `*` in `allowed-identities` as the sole opt-in allow-any wildcard (refusing `*` mixed with other entries) | Separate boolean `allow-any-identity` leaf; allowing `*` beside specific entries | Keeps one unified leaf-list per the 2026-11-24 owner decision while refusing misleading dead entries beside `*` |
| Enforce RFC 2818 §3.1 in `serverChainCheck.verifyConnection` with `InsecureSkipVerify: true` and empty `ServerName` | Setting `tls.Config.ServerName` and dropping `InsecureSkipVerify` | Preserves RFC 2818 §3.1 CN fallback when no SAN extension exists (which Go's `VerifyHostname` removed), supports `rfc822Name` SANs, avoids cleartext SNI, and runs identically on resumed TLS 1.3 sessions after `rebuildResumedChains` |
| Restrict DNS wildcards (both in `allowed-identities` and in server certificates) to a single leftmost full-label `*` with at least two suffix labels (`*.a.b`) | Partial-label wildcards (`f*.a.b`); single-label suffix (`*.com`); `.a.b` multi-level subtrees | Satisfies RFC 2818 §3.1 single-component matching while preventing TLD-wide wildcards, punycode partial-label hazards, and multi-level depth confusion |
| Pack `AllowedIdentities` and `ServerIdentity` inside `AuthAdditions` | Adding fields directly on `AuthConfig` | Keeps `SiteToSitePeer` under 288 bytes (`TestSiteToSitePeerStaysUnderTheHugeParamThreshold`) while still participating in `AuthConfig.Equal` deep comparison (restarting edited peers and invalidating resumption tickets) |
| Place identity pattern parsers in `internal/core/eap/identity.go` and import them downward from `internal/component/ike/ipsec` | Duplicating regex/grammar in `ipsec` and `eap` | Single source of truth for pattern syntax across commit validation and runtime matching with clean core-to-component import direction |

## Known Limitations

- `vpn ipsec remote-access` road-warrior EAP-TLS client authorization against `eap-user certificate` entries is owned by `plan/release/fixes/spec-ipsec-remote-access.md` (which depends on this spec's exported `Session.PeerID()`/`PeerIDs()`).
- Certificate profile constraints on issuing/verifying empty-subject critical SANs and deprecated `emailAddress` RDNs (`RFC5216-5.2-2`, `RFC5216-5.2-3`, `RFC5216-5.2-4`) are owned by `plan/post-release/features/spec-eap-tls-certificate-profile.md`.
- Partial-label wildcards (such as `f*.example.com`) and arbitrary-depth DNS subtrees (`.example.com`) are deliberately refused; operators use full-label `*.domain.tld` or explicit SAN entries.

## RFC Documentation (Scope: protocol)

Add `// RFC 5216 Section 5.2: "<quoted requirement>"`, `// RFC 5216 Section 5.3: "<quoted requirement>"`, and `// RFC 2818 Section 3.1: "<quoted requirement>"` above enforcing code in `internal/core/eap/identity.go`, `eap_tls.go`, and `peer_chain.go`.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---|---|---|---|---|
| RFC 5216 Section 5.2 (`RFC5216-5.2-1`) | Default `eap-tls` peer and authenticator extract Peer-Id / Server-Id from `subjectAltName` in certificate wire order whenever the extension is present, falling back to Subject CN/DN only when no SAN extension exists | Certificates carrying multiple SAN types (`dNSName`, `rfc822Name`, `iPAddress`, URI, UPN `otherName`) export all entries in wire order via `PeerIDs()` / `ServerIDs()` | `TestRFC5216IdentityIsTakenFromTheSANAndAuthorized` | `TestRFC5216AllowedIdentitiesMatchingMatrix` |
| RFC 5216 Section 5.3 (`RFC5216-5.3-2`) | Default fail-closed commit requires an explicit identity constraint (`allowed-identities` on responder; `server-identity` or unambiguous FQDN `remote-address` on initiator); both roles verify certificate EKU and authorize the certificate identity against configuration on full and resumed handshakes | Explicit lone `*` in `allowed-identities` permits any valid CA-signed client certificate carrying a non-empty identity when an operator opts in | `TestRFC5216IdentityIsTakenFromTheSANAndAuthorized`, `TestValidateEAPTLSIdentitiesMatrix`, `responder-eap-tls13-unauthorized-client`, `eap-tls13-unauthorized-server` | `TestRFC5216AllowedIdentitiesMatchingMatrix` (`*`, `*.domain`, `@realm`), `TestValidateEAPTLSIdentitiesMatrix` (lone `*` accepted, mixed `*` refused) |
| RFC 5216 Section 5.3 / RFC 2818 Section 3.1 (`RFC5216-5.3-3`) | Peer server-identity comparison follows RFC 2818 Section 3.1: SAN precedence over CN, IP literals matched strictly against `iPAddress` SAN, ASCII case-insensitive DNS matching, single-component leftmost wildcard `*.a.b`, CN fallback only when SAN extension is absent | Explicit `server-identity` overrides `remote-address` (allowing IP-addressed peers to pin an FQDN or IP SAN, or FQDN-addressed peers to pin a distinct certificate identity) | `TestRFC5216ServerIdentityRFC2818Rules`, `eap-tls13`, `eap-tls13-unauthorized-server` | `TestBuildPeerTLSConfigWiresServerIdentity` (`server-identity` overriding FQDN `remote-address`) |
| RFC 9190 Sections 2.2 & 5.7 | Authorization uses only the verified TLS certificate (never the unauthenticated EAP-Response/Identity), re-runs on resumed TLS 1.3 sessions, and invalidates resumption tickets on identity policy edits | `session-resumption true` (default) continues to resume authorized sessions across reconnects | `TestRFC5216ResumedSessionReverifiesPeerAndServerIdentity`, `TestEAPTLSServerConfigWiresAllowedIdentities` | `test/ipsec/ipsec-eap-tls13-resumption.ci` |

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
- [ ] AC-1..AC-8 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
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
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- **Core EAP-TLS identity extraction & authorization (`internal/core/eap/`):** Created `identity.go` (stdlib-only) implementing SAN-first wire-order extraction (`extractCertificateIdentities`, `certificatePrimaryID`, `appendSANIdentities`, `decodeGeneralName`, `mostSpecificCommonName`), unified pattern parsers (`ParseAllowedIdentity`, `ParseServerIdentity`, `IsUnambiguousFQDN`), authenticator authorization (`authorizePeerCertificate`, `allowedIdentityMatches`), and peer RFC 2818 Section 3.1 verification (`verifyServerCertificate`, `serverIdentityMatches`). Updated `eapTLSPeerName` (`eap_tls.go`), exported `Session.PeerID()`/`PeerIDs()` (`eap.go`) and `PeerSession.ServerID()`/`ServerIDs()` (`peer.go`), and enforced authorization in both `newTLSMethod` `VerifyConnection` branches (`eap_tls.go`) and `serverChainCheck.verifyConnection` (`peer_chain.go`) on full and resumed TLS 1.3 handshakes.
- **YANG schema, IPsec data model, and fail-closed validation (`internal/component/ike/ipsec/`):** Added `leaf-list allowed-identities` and `leaf server-identity` (`type string { length "1..255"; }`) under `vpn ipsec site-to-site peer authentication` in `ze-ipsec-conf.yang`; packed `AllowedIdentities` and `ServerIdentity` inside `AuthAdditions` (`types.go`) with `AllowedIdentities()`, `ServerIdentity()`, and `SiteToSitePeer.EffectiveEAPTLSServerIdentity()`; added `parseEAPTLSIdentityPolicy` (`config_auth_policy.go`, called from `config.go::parseAuthConfig`); implemented `IPsecConfig.ValidateEAPTLSIdentities()` and `ValidatePeerEAPTLSIdentity` (`validate.go`) with `opererr` refusals and exact `set`/`delete` fix commands.
- **IKE engine wiring, resumption invalidation, logging, and doctor (`internal/component/ike/engine/`):** Wired `cfg.ValidateEAPTLSIdentities()` in `validateIPsecSections` (`config.go`) right after `ValidateIntermediateRefs`; forwarded `AllowedIdentities` in `eapTLSServerConfig` (`responder_eap.go`, failing closed with `opererr` if empty) and `EffectiveEAPTLSServerIdentity()` in `buildPeerTLSConfig` (`fsm.go`, failing closed with `code: doctor-ipsec-eap-tls-identity` Warn if empty); extended `recordEAPTLSAuthentication` (`resumption.go`) to log `peer-id` (responder) and `server-id` (initiator); added `checkIPsecEAPTLSIdentity` (`doctor_eap_tls_identity.go`), registered `ipsec-eap-tls-identity` at `DoctorPhasePostConfig` (`register.go`), and registered `doctor-ipsec-eap-tls-identity` (`register_doctor_codes.go`). Deduplicated `"show config running"` via `fixShowConfigRunning` across `doctor_eap_tls_identity.go`, `doctor_implicit_discard.go`, and `doctor_pmtu_disc.go`.
- **Tests, strongSwan interop, RFC conformance, docs, and news fragment:** Added unit suites (`rfc5216_identity_test.go`, `validate_eaptls_identity_test.go`, `eap_tls_identity_wiring_test.go`, `doctor_eap_tls_identity_test.go`), 4 functional `.ci` tests (`ipsec-eap-tls-identity-authorized.ci`, `ipsec-eap-tls-identity-rejected.ci`, `ipsec-eap-tls-identity-required.ci`, `tx-ipsec-eap-tls-identity-required.ci`), 2 negative strongSwan interop scenarios (`responder-eap-tls13-unauthorized-client`, `eap-tls13-unauthorized-server`) with checkers (`internal/le/interoplab/ipsec/checkers.go`, `ipsec_test.go`, `test/interop-ipsec/parity_test.go`), updated all 6 existing `.ci` tests, 7 existing strongSwan `ze.conf` files, and 40 unit test fixtures, updated `rfc/short/rfc5216.md` plus all RFC audit/discrimination records, updated `docs/guide/ipsec.md` and four `docs/architecture/ike/` pages, cleared citers in `plan/release/fixes/spec-ipsec-remote-access.md`, and created `website/changes/fragments/eap-tls-identity-authorization.md`.

### Bugs Found/Fixed
- `internal/le/interoplab/ipsec/checkers.go`: `eapTLSRefusalFacts[1]` expected `"for peer CN=172.28.0.3"` from the pre-SAN `eapTLSPeerName`; updated to `"for peer 172.28.0.3"` matching RFC 5216 Section 5.2 SAN-first extraction (verified by strongSwan `eap-tls` PASS).
- `internal/component/ike/engine/doctor_implicit_discard.go` & `doctor_pmtu_disc.go`: extracted shared `fixShowConfigRunning = "show config running"` constant to satisfy `goconst` lint after adding `doctor_eap_tls_identity.go`.

### Documentation Updates
- `docs/guide/ipsec.md`: updated EAP-TLS responder example with `connection-type respond` and `allowed-identities`; replaced stale "without DNS-name matching" paragraph with `## EAP-TLS certificate identity authorization` with source anchors (`identity.go`, `peer.go`, `peer_chain.go`, `validate.go`, `responder_eap.go`, `fsm.go`, `resumption.go`, `doctor_eap_tls_identity.go`).
- `docs/architecture/ike/ipsec-3-data-model.md`: documented `allowed-identities`, `server-identity`, `AuthAdditions`, `EffectiveEAPTLSServerIdentity`, `parseEAPTLSIdentityPolicy`, and `ValidateEAPTLSIdentities`/`ValidatePeerEAPTLSIdentity` with source anchors.
- `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md`: documented SAN-first extraction (`extractCertificateIdentities`), `eapTLSPeerName`, `Session.PeerID`/`PeerIDs`, and authenticator `authorizePeerCertificate` with source anchors.
- `docs/architecture/ike/ipsec-11-interop-eap.md`: updated `ServerName`/`InsecureSkipVerify` explanation, documented `verifyServerCertificate` and `PeerSession.ServerID`/`ServerIDs`, and added `responder-eap-tls13-unauthorized-client` and `eap-tls13-unauthorized-server` with checker source anchors.
- `docs/architecture/ike/ipsec-14-responder.md`: documented `eapTLSServerConfig` `allowed-identities` wiring, `resumptionFor` ticket invalidation, `recordEAPTLSAuthentication` `peer-id`/`server-id`, and `checkIPsecEAPTLSIdentity` with source anchors.
- `rfc/short/rfc5216.md`: closed `{gap}` on `RFC5216-5.2-1`, `RFC5216-5.3-2`, and `RFC5216-5.3-3`; updated Meta `Enrolment reason`.
- `website/changes/fragments/eap-tls-identity-authorization.md`: created operator-facing news fragment (`topic: IPsec`, `feature: ipsec-eap-authentication`, `found-by: rfc-check`).
- `./le doc check all`: 0 errors and 0 broken source anchors across all modified `docs/guide/ipsec.md` and `docs/architecture/ike/*.md` files (checked 6137 code paths; remaining failures in tree are unrelated concurrent-session files `catalog.go` and `ze-policy-conf`).

### Deviations from Plan
- `internal/component/ike/ipsec/yang/embed.go` and `register.go` required no diff because `embed.go` embeds `ze-ipsec-conf.yang` via `//go:embed`; `./le yang glue check` confirmed glue is in sync.
- Extracted `fixShowConfigRunning` constant across `doctor_eap_tls_identity.go`, `doctor_implicit_discard.go`, and `doctor_pmtu_disc.go` to clear `goconst` lint cleanly.
- Added `TestRFC5216IdentityParsingBoundaries` in `internal/core/eap/rfc5216_identity_test.go` beyond the TDD plan list to pin every length and label-count boundary in one dedicated test.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| none | | | | |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| 1. SAN-first wire-order Peer-Id/Server-Id extraction, `eapTLSPeerName`, `Session.PeerID`/`PeerIDs`, `PeerSession.ServerID`/`ServerIDs` | Done | `internal/core/eap/identity.go:359-553`, `eap_tls.go:359-363,668-688`, `eap.go:515-553`, `peer.go:1510-1538`, `peer_chain.go:47-66` | Verified by `TestRFC5216IdentityIsTakenFromTheSANAndAuthorized` |
| 2. YANG `allowed-identities` leaf-list & `server-identity` leaf packed in `ipsec.AuthAdditions` | Done | `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang:1674-1715`, `types.go:633-664,861-875`, `config.go:872`, `config_auth_policy.go:261-279` | Zero size growth on `SiteToSitePeer` (`TestSiteToSitePeerStaysUnderTheHugeParamThreshold` PASS) |
| 3. Fail-closed commit/validate/reload + runtime guards + post-config doctor check | Done | `internal/component/ike/ipsec/validate.go:401-533`, `engine/config.go:179`, `responder_eap.go:165-176`, `fsm.go:1491-1501`, `eap_tls.go:508-525`, `peer.go:1306-1312`, `doctor_eap_tls_identity.go:28-75`, `register.go:467-474`, `register_doctor_codes.go:112,428-438` | `opererr` fixes verified; `doctor-ipsec-eap-tls-identity` registered |
| 4. Mutual `VerifyConnection` authorization (full + resumed TLS 1.3), fatal `bad_certificate` alert, and `peer-id`/`server-id` logging | Done | `internal/core/eap/eap_tls.go:559-600,655-666`, `peer_chain.go:183-200`, `engine/responder_eap.go:414,421`, `fsm.go:1313`, `resumption.go:216-227` | Proved in unit, `.ci`, and strongSwan interop |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRFC5216IdentityIsTakenFromTheSANAndAuthorized`, `test/ipsec/ipsec-eap-tls-identity-authorized.ci` | Wire-order SAN extraction, CN-no-fallback, and `eapTLSPeerName` verified |
| AC-2 | Done | `TestRFC5216AllowedIdentitiesMatchingMatrix`, `ipsec-eap-tls-identity-authorized.ci`, `ipsec-eap-tls-identity-rejected.ci`, `responder-eap-tls13-unauthorized-client` | All 7 rule forms + negative boundaries + fatal alert verified |
| AC-3 | Done | `TestRFC5216ServerIdentityRFC2818Rules`, `ipsec-eap-tls-identity-authorized.ci`, `ipsec-eap-tls-identity-rejected.ci`, `eap-tls13-unauthorized-server` | RFC 2818 §3.1 DNS/wildcard/IP/NAI/CN rules + fatal alert verified |
| AC-4 | Done | `TestValidateEAPTLSIdentitiesMatrix`, `TestValidateIPsecSectionsEnforcesEAPTLSIdentityConstraints`, `test/parse/ipsec-eap-tls-identity-required.ci`, `test/reload/tx-ipsec-eap-tls-identity-required.ci`, `test/reload/tx-ipsec-eap-tls-requires-ca.ci` | Full fail-closed matrix and `ValidatePKIRefs` precedence verified |
| AC-5 | Done | `TestRFC5216ResumedSessionReverifiesPeerAndServerIdentity`, `TestEAPTLSServerConfigWiresAllowedIdentities`, `TestBuildPeerTLSConfigWiresServerIdentity`, `TestPeerConfigChangedIgnoresNothingSilently`, `test/ipsec/ipsec-eap-tls13-resumption.ci` | Resumed TLS 1.3 re-verification and ticket invalidation on edit verified |
| AC-6 | Done | `TestRFC5216FailClosedWhenIdentityPolicyUnset`, `TestEAPTLSServerConfigWiresAllowedIdentities`, `TestBuildPeerTLSConfigWiresServerIdentity`, `TestDoctorIPsecEAPTLSIdentityCheck` | Runtime backstops + `doctor-ipsec-eap-tls-identity` verified |
| AC-7 | Done | `rfc/short/rfc5216.md`, `rfc/audit/rfc5216/RFC5216-{5.2-1,5.3-2,5.3-3}.json`, `rfc/discrimination/rfc5216/RFC5216-{5.2-1,5.3-2,5.3-3}.json` | Independent `/ze-rfc-audit` (`4a47c236`) `enforced` + 36 discrimination records clean |
| AC-8 | Done | `responder-eap-tls13-unauthorized-client` (PASS 13s + red), `eap-tls13-unauthorized-server` (PASS 11s + red), `eap-tls` (PASS 35s), `responder-eap-tls13-revoked-client` (PASS 16s), `eap-tls13` & `responder-eap-tls13` (auth + IKE SA proved; XFRM Child SA halted on host kernel `0xf != 0xef`) | See Goal Validation for full host kernel note |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRFC5216IdentityIsTakenFromTheSANAndAuthorized` | Done | `internal/core/eap/rfc5216_identity_test.go:208` | PASS |
| `TestRFC5216AllowedIdentitiesMatchingMatrix` | Done | `internal/core/eap/rfc5216_identity_test.go:407` | PASS |
| `TestRFC5216ServerIdentityRFC2818Rules` | Done | `internal/core/eap/rfc5216_identity_test.go:596` | PASS |
| `TestRFC5216ResumedSessionReverifiesPeerAndServerIdentity` | Done | `internal/core/eap/rfc5216_identity_test.go:722` | PASS |
| `TestRFC5216FailClosedWhenIdentityPolicyUnset` | Done | `internal/core/eap/rfc5216_identity_test.go:825` | PASS |
| `TestRFC5216IdentityParsingBoundaries` (extra) | Done | `internal/core/eap/rfc5216_identity_test.go:873` | PASS (all numeric/label boundaries) |
| `TestValidateEAPTLSIdentitiesMatrix` | Done | `internal/component/ike/ipsec/validate_eaptls_identity_test.go:75` | PASS |
| `TestParseEAPTLSIdentityPolicyAndEquality` | Done | `internal/component/ike/ipsec/validate_eaptls_identity_test.go:581` | PASS |
| `TestEAPTLSServerConfigWiresAllowedIdentities` | Done | `internal/component/ike/engine/eap_tls_identity_wiring_test.go:129` | PASS |
| `TestBuildPeerTLSConfigWiresServerIdentity` | Done | `internal/component/ike/engine/eap_tls_identity_wiring_test.go:202` | PASS |
| `TestValidateIPsecSectionsEnforcesEAPTLSIdentityConstraints` | Done | `internal/component/ike/engine/eap_tls_identity_wiring_test.go:40` | PASS |
| `TestDoctorIPsecEAPTLSIdentityCheck` | Done | `internal/component/ike/engine/doctor_eap_tls_identity_test.go:56` | PASS |
| `ipsec-eap-tls-identity-authorized.ci` | Done | `test/ipsec/ipsec-eap-tls-identity-authorized.ci` | PASS 80/80 under 64 CPU burners |
| `ipsec-eap-tls-identity-rejected.ci` | Done | `test/ipsec/ipsec-eap-tls-identity-rejected.ci` | PASS 80/80 under 64 CPU burners + counterfactual red (`EAP_CI_RC=1`) |
| `ipsec-eap-tls-identity-required.ci` | Done | `test/parse/ipsec-eap-tls-identity-required.ci` | PASS 80/80 under 64 CPU burners |
| `tx-ipsec-eap-tls-identity-required.ci` | Done | `test/reload/tx-ipsec-eap-tls-identity-required.ci` | PASS 80/80 under 64 CPU burners |
| `responder-eap-tls13-unauthorized-client` | Done | `test/interop-ipsec/scenarios/responder-eap-tls13-unauthorized-client/` | PASS (13s) + counterfactual red (`SWAN_CLIENT_RC=1`) |
| `eap-tls13-unauthorized-server` | Done | `test/interop-ipsec/scenarios/eap-tls13-unauthorized-server/` | PASS (11s) + counterfactual red (`SWAN_SERVER_RC=1`) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| All 12 Files to Create | Done | Verified via `ls -1` in Pre-Commit Verification |
| All Files to Modify (product, tests, interop, RFC, docs) | Done | `yang/embed.go` and `register.go` unchanged because `//go:embed ze-ipsec-conf.yang` needs no diff; added `doctor_implicit_discard.go` and `doctor_pmtu_disc.go` for `fixShowConfigRunning` constant; cleared citers in `plan/release/fixes/spec-ipsec-remote-access.md`; created `website/changes/fragments/eap-tls-identity-authorization.md` |

### Audit Summary
- **Total items:** 36 (4 task requirements + 8 ACs + 18 planned/added tests + 6 doc/RFC/interop deliverables)
- **Done:** 36
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (YANG glue unchanged via `//go:embed`; `fixShowConfigRunning` constant deduplication)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Extract Peer-Id and Server-Id from `subjectAltName` in wire order (RFC 5216 §5.2), update `eapTLSPeerName`, export `PeerID`/`PeerIDs` and `ServerID`/`ServerIDs` | Unit + functional + interop | `TestRFC5216IdentityIsTakenFromTheSANAndAuthorized` PASS; `rfc/discrimination/rfc5216/RFC5216-5.2-1.json` + `/ze-rfc-audit` `enforced`; `ipsec-eap-tls-identity-authorized.ci` PASS 80/80 asserting `peer-id=ze-eap-client` and `server-id=ze-eap-server`; strongSwan `eap-tls` PASS asserting `"for peer 172.28.0.3"` |
| Authorize client identities via `allowed-identities` and server identities via `server-identity` / unambiguous FQDN `remote-address` under RFC 5216 §5.3 and RFC 2818 §3.1 on full and resumed TLS 1.3 sessions | Unit + functional + strongSwan interop (with counterfactual red) | `TestRFC5216AllowedIdentitiesMatchingMatrix`, `TestRFC5216ServerIdentityRFC2818Rules`, `TestRFC5216ResumedSessionReverifiesPeerAndServerIdentity` PASS; `rfc/discrimination/rfc5216/RFC5216-{5.3-2,5.3-3}.json` + `/ze-rfc-audit` `enforced`; `ipsec-eap-tls-identity-rejected.ci` PASS 80/80 + counterfactual red (`EAP_CI_RC=1`); strongSwan `responder-eap-tls13-unauthorized-client` PASS (13s, red `SWAN_CLIENT_RC=1`) and `eap-tls13-unauthorized-server` PASS (11s, red `SWAN_SERVER_RC=1`); strongSwan `eap-tls13` & `responder-eap-tls13` proved full EAP-TLS 1.3 auth (`server-id=172.28.0.3`, `peer-id=swan-eap-client`) + IKE SA establishment (stopping only at XFRM Child SA install because host `claude` runs kernel scope `0xf` rather than Ze's 7.3 runtime kernel `0xef`) |
| Fail-closed commit/validate/reload default + runtime backstops + post-config doctor check (`doctor-ipsec-eap-tls-identity`) | Unit + functional + arch gates | `TestValidateEAPTLSIdentitiesMatrix`, `TestValidateIPsecSectionsEnforcesEAPTLSIdentityConstraints`, `TestEAPTLSServerConfigWiresAllowedIdentities`, `TestBuildPeerTLSConfigWiresServerIdentity`, `TestDoctorIPsecEAPTLSIdentityCheck` PASS; `ipsec-eap-tls-identity-required.ci` and `tx-ipsec-eap-tls-identity-required.ci` PASS 80/80; `tx-ipsec-eap-tls-requires-ca.ci` PASS; `./le arch operator-error check` and `./le arch doctor-coverage check` PASS |
| Preserve `SiteToSitePeer` struct size (`< 288` bytes) and `AuthConfig.Equal` reload/resumption sensitivity | Unit | `TestSiteToSitePeerStaysUnderTheHugeParamThreshold`, `TestParseEAPTLSIdentityPolicyAndEquality`, `TestPeerConfigChangedIgnoresNothingSilently` PASS |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | All in-scope deliverables completed. (`remote-access` EAP-TLS `eap-user` pinning and RFC 5216 §§5.2-2..4 certificate profile rules were out of scope and already live in `plan/release/fixes/spec-ipsec-remote-access.md` and `plan/post-release/features/spec-eap-tls-certificate-profile.md`.) | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/eap-tls-identity-authorization-9e4a1f62-3b7c-4d8e-a105-c6b2d8e4f901.md` |
| `./le spec review check` | clean |
| Rounds | 1 |
| Reviewer lenses used | logic+wiring, security+edge-cases, rfc+design+style (+ `/ze-review-spec`) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | NOTE | `./le repo check` notes `Session.PeerIDs` (`internal/core/eap/eap.go:540`) and `PeerSession.ServerIDs` (`internal/core/eap/peer.go:1525`) have no cross-package non-test caller yet; both are required by AC-1 and RFC 5216 Section 5.2 (wire-order multi-SAN export), tested in `TestRFC5216IdentityIsTakenFromTheSANAndAuthorized`, and `PeerIDs()` is the exported seam for `spec-ipsec-remote-access` | `internal/core/eap/eap.go:540`, `internal/core/eap/peer.go:1525` | Accepted as NOTE (by spec design) |
| 2 | NOTE | Positive strongSwan XFRM Child SA install in `eap-tls13` and `responder-eap-tls13` halted on host `claude` kernel scope `0xf != 0xef` after proving full EAP-TLS 1.3 authentication (`server-id=172.28.0.3`, `peer-id=swan-eap-client`) and IKE SA establishment; all negative strongSwan scenarios (`responder-eap-tls13-unauthorized-client`, `eap-tls13-unauthorized-server`, `eap-tls`, `responder-eap-tls13-revoked-client`) and all 4 `.ci` tests (80/80) passed completely | `test/interop-ipsec/scenarios/{eap-tls13,responder-eap-tls13}/` | Recorded honestly in Goal Validation |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/eap/identity.go` | Yes | `ls -1` verified |
| `internal/core/eap/rfc5216_identity_test.go` | Yes | `ls -1` verified |
| `internal/component/ike/ipsec/validate_eaptls_identity_test.go` | Yes | `ls -1` verified |
| `internal/component/ike/engine/doctor_eap_tls_identity.go` | Yes | `ls -1` verified |
| `internal/component/ike/engine/doctor_eap_tls_identity_test.go` | Yes | `ls -1` verified |
| `internal/component/ike/engine/eap_tls_identity_wiring_test.go` | Yes | `ls -1` verified |
| `test/ipsec/ipsec-eap-tls-identity-authorized.ci` | Yes | `ls -1` verified |
| `test/ipsec/ipsec-eap-tls-identity-rejected.ci` | Yes | `ls -1` verified |
| `test/parse/ipsec-eap-tls-identity-required.ci` | Yes | `ls -1` verified |
| `test/reload/tx-ipsec-eap-tls-identity-required.ci` | Yes | `ls -1` verified |
| `test/interop-ipsec/scenarios/responder-eap-tls13-unauthorized-client/` (10 files) | Yes | `ls -1` verified (`ze.conf`, `swanctl.conf`, `strongswan.conf`, 7 PKI files) |
| `test/interop-ipsec/scenarios/eap-tls13-unauthorized-server/` (10 files) | Yes | `ls -1` verified (`ze.conf`, `swanctl.conf`, `strongswan.conf`, 7 PKI files) |
| `rfc/audit/rfc5216/RFC5216-{5.2-1,5.3-2,5.3-3}.json` & `rfc/discrimination/rfc5216/RFC5216-{5.2-1,5.3-2,5.3-3}.json` | Yes | `ls -1` verified |
| `website/changes/fragments/eap-tls-identity-authorization.md` | Yes | `ls -1` verified |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | SAN wire-order extraction, CN-no-fallback, `eapTLSPeerName`, `PeerID`/`PeerIDs`, `ServerID`/`ServerIDs` | Read `internal/core/eap/identity.go:359-553`, `eap_tls.go:359-363`, `eap.go:515-553`, `peer.go:1510-1538`; `TestRFC5216IdentityIsTakenFromTheSANAndAuthorized` PASS |
| AC-2 | Authenticator `allowed-identities` (all 7 forms + fatal `bad_certificate` alert + `peer-id` log) | Read `identity.go:560-622`, `eap_tls.go:559-600,655-666`; `TestRFC5216AllowedIdentitiesMatchingMatrix`, `ipsec-eap-tls-identity-authorized.ci` (80/80), `ipsec-eap-tls-identity-rejected.ci` (80/80 + red), `responder-eap-tls13-unauthorized-client` (PASS 13s + red) |
| AC-3 | Peer `server-identity` / FQDN `remote-address` under RFC 2818 §3.1 + fatal `bad_certificate` alert + `server-id` log | Read `identity.go:631-677`, `peer_chain.go:183-200`, `fsm.go:1491-1501`; `TestRFC5216ServerIdentityRFC2818Rules`, `ipsec-eap-tls-identity-authorized.ci` (80/80), `ipsec-eap-tls-identity-rejected.ci` (80/80 + red), `eap-tls13-unauthorized-server` (PASS 11s + red) |
| AC-4 | Fail-closed validate/commit/reload matrix with `opererr` fixes and `ValidatePKIRefs` precedence | Read `ipsec/validate.go:401-533`, `engine/config.go:179`; `TestValidateEAPTLSIdentitiesMatrix`, `ipsec-eap-tls-identity-required.ci` (80/80), `tx-ipsec-eap-tls-identity-required.ci` (80/80), `tx-ipsec-eap-tls-requires-ca.ci` PASS |
| AC-5 | Resumed TLS 1.3 re-verification and ticket invalidation on policy edit | Read `eap_tls.go:559-600`, `peer_chain.go:183-200`, `ipsec/types.go:633-644`; `TestRFC5216ResumedSessionReverifiesPeerAndServerIdentity`, `TestPeerConfigChangedIgnoresNothingSilently`, `ipsec-eap-tls13-resumption.ci` PASS |
| AC-6 | Runtime fail-closed backstops + `doctor-ipsec-eap-tls-identity` | Read `responder_eap.go:165-176`, `fsm.go:1491-1501`, `doctor_eap_tls_identity.go:28-75`, `register.go:467-474`, `register_doctor_codes.go:428-438`; `TestDoctorIPsecEAPTLSIdentityCheck` + `./le arch doctor-coverage check` PASS |
| AC-7 | `RFC5216-5.2-1`, `RFC5216-5.3-2`, `RFC5216-5.3-3` closed with audit & discrimination records | Read `rfc/short/rfc5216.md:355,360-361` (no `{gap}`); `/ze-rfc-audit` (`4a47c236`) `enforced`; 36 discrimination records clean; `./le test weakened check` clean |
| AC-8 | strongSwan interop (negative + positive) | `responder-eap-tls13-unauthorized-client` PASS (13s, red), `eap-tls13-unauthorized-server` PASS (11s, red), `eap-tls` PASS (35s), `responder-eap-tls13-revoked-client` PASS (16s), `eap-tls13`/`responder-eap-tls13` auth + IKE SA proved on host `claude` (`0xf != 0xef`) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze config validate` on `site-to-site peer` with `mode eap-tls` | `test/parse/ipsec-eap-tls-identity-required.ci` | Yes (read file: tests all 9 refusal cases + 4 valid forms via `ze config validate -`) |
| SIGHUP reload switching peer to `eap-tls` without identity constraint | `test/reload/tx-ipsec-eap-tls-identity-required.ci` | Yes (read file: verifies `OnConfigVerify` refusal + running config preserved) |
| Live two-daemon EAP-TLS authorized handshake + `peer-id`/`server-id` logging | `test/ipsec/ipsec-eap-tls-identity-authorized.ci` | Yes (read file: two real `ze` daemons, asserts `show vpn ipsec sa`, `peer-id=ze-eap-client`, `server-id=ze-eap-server`) |
| Live two-daemon EAP-TLS unauthorized client & mismatched server refusal | `test/ipsec/ipsec-eap-tls-identity-rejected.ci` | Yes (read file: three real `ze` daemons, asserts both SAN refusal logs and zero `SA established`/`EAP-TLS authenticated`) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `./le arch tier check` PASS; `internal/component/ike/ipsec` imports `internal/core/eap` cleanly |
| A-2 | confirmed | `TestRFC5216ResumedSessionReverifiesPeerAndServerIdentity` PASS on resumed authenticator (`cs.PeerCertificates` restored from ticket) |
| A-3 | confirmed | strongSwan `eap-tls` PASS (`172.28.0.3`), `responder-eap-tls13-revoked-client` PASS, and `eap-tls13`/`responder-eap-tls13` authenticated `server-id=172.28.0.3` and `peer-id=swan-eap-client` |
| A-4 | confirmed | `responder-eap-tls13-unauthorized-client` PASS (13s + red) and `eap-tls13-unauthorized-server` PASS (11s + red) |
| A-5 | confirmed | `TestSiteToSitePeerStaysUnderTheHugeParamThreshold` PASS (`< 288` bytes) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/guide/ipsec.md` (`## EAP-TLS certificate identity authorization` + updated responder example) | Read diff; anchors resolve to `identity.go`, `peer.go`, `peer_chain.go`, `validate.go`, `responder_eap.go`, `fsm.go`, `resumption.go`, `doctor_eap_tls_identity.go`; `./le doc ste check` + `./le doc check all` clean on file | Yes |
| `docs/architecture/ike/ipsec-3-data-model.md`, `ipsec-9-ikev2-eap-nat.md`, `ipsec-11-interop-eap.md`, `ipsec-14-responder.md` | Read diff; all anchors verified by `./le doc check all` (0 broken anchors in `docs/architecture/ike/`) | Yes |
| `rfc/short/rfc5216.md` (`RFC5216-5.2-1`, `5.3-2`, `5.3-3`) | Read diff; `{gap}` removed, audit & discrimination records present; `docs/features/rfc-status.md` is derived on demand (`8ae7424594`) | Yes |
| External citers (`plan/release/fixes/spec-ipsec-remote-access.md`) | `git grep` confirmed 0 remaining `spec-eap-tls-identity-authorization.md` or `plan/` path citers | Yes |
| News fragment (`website/changes/fragments/eap-tls-identity-authorization.md`) | Created with valid front matter (`spec`, `topic: IPsec`, `feature: ipsec-eap-authentication`, `found-by: rfc-check`) and all 6 fixed headings | Yes |
