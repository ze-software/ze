# Spec: fixit-ipsec-verify-siblings

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/6 |
| Updated | 2026-07-23 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/fail-closed-guards.md`, `ai/rules/exact-or-reject.md`, `ai/rules/doctor-checks.md`
4. `internal/component/ike/engine/responder_eap.go`, `internal/component/ike/ipsec/validate.go`, `internal/component/ike/engine/config.go`

## Task

`plan/learned/1255-fixit-codeql-security-triage.md` closed the CodeQL triage having fixed
the EAP-TLS trust-anchor fail-open on the **initiator** side, and recorded three siblings
of the same class that the spec's scope did not cover. This spec closes them.

The three, restated precisely against source (not as recorded, which was partly wrong):

1. **The responder silently discards a configured trust anchor.**
   `responder_eap.go:54-58` does `if ca := pki.GetCA(caName); ca != nil { ... }`. When the
   operator names a `ca-certificate` that is absent from the PKI store, the miss is
   swallowed and `cfg.CACertPEM` stays empty.
   -> Constraint: this is NOT a fail-open. `newTLSMethod` (`eap/eap_tls.go:156-168`) builds a
   non-nil empty `x509.CertPool` and sets `ClientAuth: tls.RequireAndVerifyClientCert`, and an
   empty non-nil pool REJECTS (measured: `x509: certificate signed by unknown authority`).
   The defect is that it fails closed *silently and late*: every client is refused at
   handshake with an opaque TLS error and nothing names the missing CA. That is an
   `exact-or-reject.md` violation and a `fail-closed-guards.md` "or say something" violation.

2. **Remote-access gateway certificate references are unvalidated.**
   `ValidateRemoteAccess` (`ipsec/validate.go:89-128`) checks pool ranges, group refs and
   per-user credentials, but never resolves `ra.Auth.CACertificate` / `ra.Auth.Certificate`
   against the PKI store, never applies the RFC 5216 Section 5.3 trust-anchor requirement that
   `ValidatePKIRefs` now applies to site-to-site peers, and never resolves
   `eap-user/*/certificate`. The YANG leaves are bare `type string`
   (`ze-ipsec-conf.yang:220-232`), so nothing else checks them either.

3. **`ValidateInterfaceRef` has no non-test caller.**
   `ipsec/validate.go:78` is called only from `ipsec/config_test.go:460`.
   `config.go:115` documents it as unwired.

Plus two bookkeeping obligations this work inherits:

4. Correct the mischaracterisation in `plan/learned/1255`, which calls item 1 a "fail-open".
   It is not one; leaving that wrong misdirects whoever picks it up.
5. Run `scripts/dev/stress-repro.py reload` against the load-sensitive reload tests recorded in
   `plan/known-failures/reload-transaction-tests-load-sensitive.md` and record the outcome.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/doctor-checks.md` - where a host-readiness dependency check belongs
  -> Decision: interface existence is a HOST fact, not a config-consistency fact, so it belongs
     in a doctor check, not in config verify. Confirmed by precedent: `checkDHCPInterfaces`
     (`internal/component/doctor/checks_listener.go:334-362`) does exactly this for
     `service/dhcp-server/listen-interface`, with the registered code `doctor-dhcp-iface`.
  -> Constraint: internal plugins declare checks via `Registration.DoctorChecks`, NOT by
     registering into the central doctor package (`plugin-self-containment.md`).
- [ ] `ai/rules/exact-or-reject.md` - silent approximation is banned
  -> Constraint: if the backend cannot deliver what the config asks for (a named CA that does
     not resolve), verify/commit must fail with a clear error rather than proceed degraded.
- [ ] `ai/rules/fail-closed-guards.md`
  -> Constraint: "a guard that genuinely cannot deny MUST log, error, or fail its gate. A guard
     that neither denies nor speaks does not exist." Item 1 denies but does not speak.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5216.md` - EAP-TLS
  -> Constraint: Section 5.3 "Both sides MUST perform certificate path validation." Already
     enforced for site-to-site peers by `ValidatePKIRefs`; item 2 extends the same requirement
     to the remote-access gateway, which is the same obligation on the same protocol.
- [ ] `rfc/short/rfc7296.md` - IKEv2 / EAP in IKE_AUTH (Section 2.16)

**Key insights:**
- The responder's empty-pool behaviour was measured, not assumed. An empty non-nil
  `x509.CertPool` rejects; only a `nil` Roots falls back to system roots, and this code never
  passes nil.
- `ConfigRoots` for the ike plugin is `{"vpn", "pki"}` (`engine/register.go:166`), so config
  verify cannot see the `interfaces` section. This is the second reason item 3 cannot be a
  config-verify check without widening the plugin's config delivery.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/responder_eap.go` - builds the responder's EAP method
  config; swallows a `pki.GetCA` miss at :54-58
- [ ] `internal/component/ike/eap/eap_tls.go` - `newTLSMethod` at :150-174; empty pool +
  `RequireAndVerifyClientCert`
- [ ] `internal/component/ike/ipsec/validate.go` - the four validators; `ValidateRemoteAccess`
  ignores `ra.Auth` certificate refs; `ValidateInterfaceRef` unwired
- [ ] `internal/component/ike/engine/config.go` - `ValidateIPsecSections` is the
  `OnConfigVerify` body; side-effect free via `candidatePKI`
- [ ] `internal/component/ike/ipsec/config.go` - `parseRemoteAccess` at :428; `parseEAPUser`
  reads `certificate` for `AuthEAPTLS` at :505-515
- [ ] `internal/component/doctor/checks_listener.go` - `checkDHCPInterfaces` template
- [ ] `internal/component/plugin/registry/doctor.go` - `DoctorCheckDef` / `DoctorCheckContext`
  (`Tree any` carries `*config.Tree`)
- [ ] `internal/plugins/ldp/register.go:211-219` - a real `DoctorChecks` declaration

**Behavior to preserve:**
- `ValidateIPsecSections` stays side-effect free; certificate names resolve against the
  CANDIDATE pki section via `candidatePKI`, never by installing it.
- Site-to-site peer validation messages and their tests are unchanged.
- A config with no `vpn` section, or no `remote-access` container, still verifies clean.
- `resolveInterfaceAddr`'s existing warn-and-continue at `register.go:267` stays: the doctor
  check reports readiness, it does not change engine behaviour.

**Behavior to change:**
- A remote-access gateway naming an unresolvable CA/certificate is REJECTED at verify.
- A remote-access gateway using `eap-tls` with no `ca-certificate` is REJECTED at verify.
- An `eap-user` naming an unresolvable certificate is REJECTED at verify.
- The responder returns an error naming the missing CA instead of proceeding with an empty
  trust pool.
- `ze doctor` reports a `vpn ipsec interface` that does not exist on the host.

## Data Flow (MANDATORY)

### Entry Point
- Config commit / SIGHUP reload -> plugin `OnConfigVerify` -> `ValidateIPsecSections`.
- `ze doctor [--json]` -> plugin doctor bridge -> `DoctorChecks[].Check(ctx)`.
- IKE_AUTH from a remote-access client -> `startResponderEAP` -> `eapMethodConfig` ->
  `eapTLSServerConfig`.

### Transformation Path
1. Config sections (`vpn`, `pki`) arrive as JSON strings on the verify RPC.
2. `parseVPNSections` -> `IPsecConfig`; `candidatePKI` -> `hasCA`/`hasCert`/`certCN` closures
   over a throwaway `pki.PKIConfig`.
3. `ValidateGroupRefs` -> `ValidatePKIRefs` -> `ValidateRemoteAccess` (now PKI-aware).
4. Doctor: `DoctorCheckContext.Tree` (`*config.Tree`) -> parse `vpn` subtree ->
   `IPsecConfig.Interface` -> `ValidateInterfaceRef(exists)` -> `[]rpc.DoctorCheckDiagnostic`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config system <-> ike plugin | `OnConfigVerify` over the reload transaction bridge | [ ] |
| Doctor runner <-> ike plugin | `Registration.DoctorChecks` bridge, `Tree any` | [ ] |
| ike engine <-> PKI store | `pki.GetCA` at session time (runtime, not verify) | [ ] |

### Integration Points
- `ValidateRemoteAccess` gains the same `hasCA, hasCert func(string) bool` parameters
  `ValidatePKIRefs` already takes, so `ValidateIPsecSections` supplies both from one
  `candidatePKI` call.
- The doctor check is declared in the ike `registry.Registration`, alongside the existing
  `RegisterHealthCheck()`.

### Architectural Verification
- [ ] No bypassed layers - verify stays in the plugin's own verifier; readiness stays in doctor
- [ ] No unintended coupling - no new import of `iface` into the verify path; the doctor check
      uses `net.InterfaceByName` behind a package var seam, as `checks_listener.go` does
- [ ] No duplicated functionality - reuses `ValidateInterfaceRef` and `candidatePKI`
- [ ] Zero-copy preserved where applicable - N/A (cold config path)
- [ ] Registration over hardcoding - doctor check registers via `DoctorChecks`; no ike spelling
      added to the central doctor package

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | An empty non-nil `x509.CertPool` rejects every client cert | measured in a scratch test | item 1 would be a real fail-open needing a different fix | scratch `TestEmptyPoolVerify`, to be re-expressed as a repo test | confirmed |
| A-2 | The ike plugin's doctor check receives a `*config.Tree` containing the `vpn` subtree | `registry.DoctorCheckContext.Tree any`; `checkDHCPInterfaces` reads `service/dhcp-server` from the same tree | the check cannot see its own config; would need a different context | unit test driving the registered check with a built tree | unvalidated |
| A-3 | No existing config in the repo's own tests configures remote-access eap-tls without a ca-certificate | the new rejection would break them | existing `.ci`/unit fixtures go red | grep `test/` + full ike suite | unvalidated |
| A-4 | `ValidateRemoteAccess` has no non-test caller other than `ValidateIPsecSections` | signature change is contained | wider blast radius than planned | `grep -rn ValidateRemoteAccess` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Requiring a CA for remote-access eap-tls rejects a config an operator is running today | a functional/interop test goes red | Ze is pre-release (`compatibility.md`); the same requirement already landed for peers in 1255. Reject, and say why in the error |
| R-2 | The doctor check fires on a box where the WAN interface legitimately does not exist yet (config-first provisioning) | false ERROR in `ze doctor` | severity choice: match `checkDHCPInterfaces` (Error) for consistency; doctor is advisory and does not block startup |
| R-3 | Changing `eapTLSServerConfig` to error makes an existing responder test red because it relied on the silent path | ike engine suite | that test was asserting the defect; fix the test with the reason recorded |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| reload transaction `OnConfigVerify` | -> | `ValidateRemoteAccess` PKI checks | `test/reload/test-tx-ipsec-remote-access-requires-ca.ci` |
| `ze doctor --json` | -> | ike doctor check -> `ValidateInterfaceRef` | `TestIPsecInterfaceDoctorCheckRegistered` + `TestIPsecInterfaceDoctorCheckReportsMissing` |
| IKE_AUTH from client | -> | `eapTLSServerConfig` | `TestEAPTLSServerConfigRefusesMissingCA` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `eapTLSServerConfig` with `CACertificate` naming a CA absent from the store | returns an error naming the missing CA; does not return a config with an empty `CACertPEM` |
| AC-2 | `eapTLSServerConfig` with a resolvable CA | unchanged: `CACertPEM` populated |
| AC-3 | an EAP-TLS authenticator built with no `CACertPEM` | refuses every client; pinned so the empty-pool semantics the fix relies on cannot silently change |
| AC-4 | ~~remote-access `x509 ca-certificate` naming a CA absent from the candidate pki~~ | ~~verify rejects naming the CA~~ -- moved to `plan/spec-ipsec-remote-access.md` (owner decision 2026-07-23: implement the feature rather than validate inert config). See `plan/deferrals/fixit-ipsec-verify-siblings.md` |
| AC-5 | ~~remote-access `x509 certificate` naming a cert absent from the candidate pki~~ | ~~verify rejects naming the certificate~~ -- moved, as AC-4 |
| AC-6 | ~~`eap-user` with `certificate` absent from the candidate pki~~ | ~~verify rejects~~ -- DROPPED, not moved: `EAPUser.Certificate` has no runtime consumer and a client certificate does not belong in the gateway's store. An invented requirement |
| AC-7 | ~~a valid remote-access config with resolvable refs~~ | ~~verify accepts~~ -- moved, as AC-4 |
| AC-8 | `vpn ipsec interface eth-nonexistent` + `ze doctor` | a diagnostic with code `doctor-ipsec-iface`, severity error, naming the interface |
| AC-9 | `vpn ipsec interface <a real interface>` + `ze doctor` | no `doctor-ipsec-iface` diagnostic |
| AC-10 | no `vpn` section at all + `ze doctor` | no `doctor-ipsec-iface` diagnostic, no panic |
| AC-11 | `ze explain doctor-ipsec-iface` | returns a registered explanation |
| AC-12 | `ValidateInterfaceRef` | has a non-test caller reachable from `ze doctor` |
| AC-13 | `plan/learned/1255` | no longer describes the responder as fail-open |
| AC-14 | `stress-repro.py reload` | outcome recorded in the known-failures entry |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | commits a remote-access eap-tls config with no CA | config commit -> tx bridge -> `OnConfigVerify` -> `ValidateRemoteAccess` -> rejection surfaced to the operator | `test/reload/test-tx-ipsec-remote-access-requires-ca.ci` |
| 2 | runs `ze doctor` with a bogus ipsec interface | doctor runner -> plugin doctor bridge -> ike check -> diagnostic | `TestIPsecInterfaceDoctorCheckReportsMissing` |
| 3 | an eap-tls client connects while the gateway CA is missing from the store | IKE_AUTH -> `startResponderEAP` -> `eapMethodConfig` -> error -> logged, SA dead | `TestEAPTLSServerConfigRefusesMissingCA` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEAPTLSServerConfigRefusesMissingCA` | `internal/component/ike/engine/responder_eap_test.go` | AC-1 | |
| `TestEAPTLSServerConfigLoadsResolvableCA` | `internal/component/ike/engine/responder_eap_test.go` | AC-2 | |
| `TestEmptyClientCAPoolRejects` | `internal/component/ike/eap/eap_tls_test.go` | A-1, pins the measured stdlib behaviour the fix relies on | |
| `TestValidateRemoteAccessRequiresCAForEAPTLS` | `internal/component/ike/ipsec/validate_test.go` | AC-3 | |
| `TestValidateRemoteAccessRejectsUnknownCA` | `internal/component/ike/ipsec/validate_test.go` | AC-4 | |
| `TestValidateRemoteAccessRejectsUnknownCertificate` | `internal/component/ike/ipsec/validate_test.go` | AC-5 | |
| `TestValidateRemoteAccessRejectsUnknownUserCertificate` | `internal/component/ike/ipsec/validate_test.go` | AC-6 | |
| `TestValidateRemoteAccessAcceptsResolvableRefs` | `internal/component/ike/ipsec/validate_test.go` | AC-7 | |
| `TestValidateIPsecSectionsRejectsRemoteAccessWithoutCA` | `internal/component/ike/engine/config_test.go` | AC-3 through the real verify entry point | |
| `TestIPsecInterfaceDoctorCheckReportsMissing` | `internal/component/ike/engine/doctor_test.go` | AC-8 | |
| `TestIPsecInterfaceDoctorCheckAcceptsPresent` | `internal/component/ike/engine/doctor_test.go` | AC-9 | |
| `TestIPsecInterfaceDoctorCheckNoVPNSection` | `internal/component/ike/engine/doctor_test.go` | AC-10 | |
| `TestIPsecInterfaceDoctorCheckRegistered` | `internal/component/ike/engine/doctor_test.go` | AC-12 wiring | |
| `TestDoctorIPsecIfaceCodeRegistered` | `internal/core/diagnostic/codes_test.go` | AC-11 | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A - no new numeric input | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-tx-ipsec-remote-access-requires-ca` | `test/reload/test-tx-ipsec-remote-access-requires-ca.ci` | commit of a remote-access eap-tls config with no CA is refused | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | | | No wire-format change. Items 1-3 are config-time and session-setup-time refusals; the bytes on the wire are unchanged. `interop-and-goal-validation.md` requires interop for wire-visible protocol change, which this is not. | |

## Files to Modify
- `internal/component/ike/engine/responder_eap.go` - error on unresolvable CA
- `internal/component/ike/ipsec/validate.go` - `ValidateRemoteAccess` PKI-aware
- `internal/component/ike/engine/config.go` - pass the candidate PKI closures through; drop the
  stale "remains unwired" doc comment
- `internal/component/ike/engine/register.go` - declare `DoctorChecks`
- `internal/core/diagnostic/codes.go` - register `doctor-ipsec-iface`
- `plan/learned/1255-fixit-codeql-security-triage.md` - correct the fail-open claim
- `plan/known-failures/reload-transaction-tests-load-sensitive.md` - record the stress outcome

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | no new leaf; the existing `x509` leaves gain a checker, not a schema change |
| YANG validation constraints | No | the refs are cross-references into the pki store, which YANG cannot express here |
| CLI commands/flags | No | |
| Functional test for new RPC/API | Yes | `test/reload/test-tx-ipsec-remote-access-requires-ca.ci` |
| Doctor check for runtime dependencies | Yes | `ike` `Registration.DoctorChecks` + `doctor-ipsec-iface` in `codes.go` + unit + functional |
| Prometheus counters | No | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | |
| 2 | Config syntax changed? | [ ] | |
| 3 | CLI command added/changed? | [ ] | |
| 4 | API/RPC added/changed? | [ ] | |
| 5 | Plugin added/changed? | [ ] | |
| 6 | Has a user guide page? | [ ] | `docs/guide/vpn/*` if it documents remote-access auth |
| 7 | Wire format changed? | [ ] | |
| 8 | Plugin SDK/protocol changed? | [ ] | |
| 9 | RFC behavior implemented/proven? | [ ] | `docs/features/rfc-status.md` RFC 5216 row |
| 10 | Test infrastructure changed? | [ ] | |
| 11 | Affects daemon comparison? | [ ] | |
| 12 | Internal architecture changed? | [ ] | |
| 13 | Route metadata keys? | [ ] | |
| 14 | Prometheus counters? | [ ] | |
| 15 | Registered plugin/command/inventory changed? | [ ] | doctor check inventory |
| 16 | Changed source referenced by doc source anchors? | [ ] | grep `docs/` for each changed file |
| 17 | Existing docs show examples for this area? | [ ] | verify remote-access examples still validate |

## Files to Create
- `internal/component/ike/engine/doctor.go` - the ipsec interface readiness check
- `internal/component/ike/engine/doctor_test.go`
- `test/reload/test-tx-ipsec-remote-access-requires-ca.ci`

## Implementation Steps

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** - register the doctor check and the widened validator
   signature; write failing wiring tests
   - Tests: `TestIPsecInterfaceDoctorCheckRegistered`, `TestValidateIPsecSectionsRejectsRemoteAccessWithoutCA`
   - Files: `engine/register.go`, `engine/doctor.go` (stub), `engine/config.go`
   - Verify: the check is present in `registry.PluginDoctorChecks()`; the verify test fails
     because the validator does not yet reject
2. **Phase: Responder trust anchor** - AC-1, AC-2
   - Tests: `TestEAPTLSServerConfigRefusesMissingCA`, `TestEAPTLSServerConfigLoadsResolvableCA`,
     `TestEmptyClientCAPoolRejects`
   - Files: `engine/responder_eap.go`, `eap/eap_tls_test.go`
3. **Phase: Remote-access PKI validation** - AC-3..AC-7
   - Tests: the five `TestValidateRemoteAccess*`
   - Files: `ipsec/validate.go`, `engine/config.go`
4. **Phase: Doctor check body** - AC-8..AC-12
   - Tests: the four `TestIPsecInterfaceDoctorCheck*`, `TestDoctorIPsecIfaceCodeRegistered`
   - Files: `engine/doctor.go`, `core/diagnostic/codes.go`
5. **Functional test** - `test/reload/test-tx-ipsec-remote-access-requires-ca.ci`
6. **Bookkeeping** - AC-13 (correct learned 1255), AC-14 (stress-repro reload)
7. **Full verification** -> changed-scope tests + lint
8. **Complete spec** -> audit, learned summary, two-commit closure

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Fail-closed | Every new guard denies on miss AND says which name failed to resolve |
| Mutation-verify | Disable each new rejection; its test must go red. A guard whose test passes with the guard removed does not gate |
| Side-effect freedom | The verify path still never calls `pki.Load`; `candidatePKI` remains the only resolver |
| Sibling call-site audit | `ValidateRemoteAccess` signature change: every caller updated (`before-writing-code.md`) |
| Error messages | Each rejection names the subject, the offending value, and what to do (`error-messages.md`) |
| Registration over hardcoding | No `ipsec` spelling added to the central doctor package |
| Rule: no-layering | No compatibility shim for the old `ValidateRemoteAccess` signature |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| doctor check registered | `grep -n DoctorChecks internal/component/ike/engine/register.go` |
| diagnostic code registered | `bin/ze explain doctor-ipsec-iface` |
| `ValidateInterfaceRef` wired | `grep -rn ValidateInterfaceRef --include=*.go` shows a non-test caller |
| learned 1255 corrected | `grep -n 'fail-open' plan/learned/1255-*.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | interface name from config reaching `net.InterfaceByName`: reject `/`, NUL and `..` as `checkDHCPInterfaces` does |
| Error leakage | rejection messages name config identifiers, never certificate material or key bytes |
| Fail direction | every new branch denies on error/miss; no new path returns a permissive zero value |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Existing ike test red | Determine whether it asserted the defect (fix the test, record why) or a real regression (fix the code) |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| `eapTLSServerConfig` was a fail-open (recorded in learned 1255) | it fails CLOSED, but silently and late: empty non-nil `x509.CertPool` + `RequireAndVerifyClientCert` rejects every client | measured `x509.Certificate.Verify` against an empty pool, then pinned end-to-end in `TestEAPTLSAuthenticatorWithoutCARejectsEveryClient` | changed the fix from "add validation" to "report the swallowed miss"; AC-13 corrects the record |
| remote-access certificate references were merely *unvalidated* | the whole `remote-access` block is **inert**: nothing in it reaches runtime behaviour at all | traced every consumer (below) | reframes item 2 entirely; validating references on dead config would polish an operator trap. Raised with the user rather than decided unilaterally |
| `EAPUser.Certificate` should resolve in the PKI store | it has **no runtime consumer**: parsed (`ipsec/config.go:514`), compared for config equality (`ipsec/types.go:513`), required non-empty (`ipsec/validate.go:122`), never read to make a decision. A client certificate would not normally live in the gateway's store anyway | `grep` for consumers | AC-6 dropped as an invented requirement |

### Finding: `vpn ipsec remote-access` is inert (raised 2026-07-23)

Traced end to end, no consumer applies any part of it:

| Field | Consumer | Effect |
|-------|----------|--------|
| `ra.Pool.*` | `engine/register.go:313-320` builds `eap.NewPool(...)` into `ipPool` | **discarded**: `register.go:372` is literally `_ = ipPool` |
| `ra.Auth.*` (mode, ca-certificate, certificate) | none | none |
| `ra.Users` (every `eap-user`) | none | none |

The responder only ever admits a source that `matchResponderPeer(pkt.RemoteAddr)`
resolves to a configured **site-to-site** peer (`engine/register.go:564-567`); anything
else is logged "unsolicited IKE_SA_INIT from unconfigured source" and dropped.
`PeerSession.peerCfg` is populated exclusively from `cfg.Peers`
(`engine/reconcile.go:366`). A road-warrior client, whose source address is by
definition not preconfigured, can therefore never establish.

So an operator can write a complete remote-access VPN block, have it accepted by
`ze config validate` AND by the reload transaction, and get a daemon that does
nothing with it. That is the exact operator trap `ai/rules/exact-or-reject.md`
exists to prevent, and it is a larger defect than the one this spec set out to fix.

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Wire `ValidateInterfaceRef` into `OnConfigVerify` | the ike plugin's `ConfigRoots` is `{"vpn","pki"}`, so verify cannot see `interfaces`; widening it would re-deliver ipsec config on every interface change, and interface existence is a host fact a config-first deployment legitimately fails | a plugin-owned doctor check, following `checkDHCPInterfaces` |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- The `if x != nil { use(x) }` shape around a lookup is the same defect whether the miss ends
  in "allow" or in "deny": the guard stops speaking. `fail-closed-guards.md` already says this
  ("or say something"), and item 1 is the clean example: it denies correctly and still is a bug.
- Where a check belongs is decided by what kind of fact it asserts. A config-consistency fact
  (does this name resolve inside the config I am judging?) belongs in the plugin verifier. A
  host-readiness fact (does this interface exist on this box?) belongs in `ze doctor`. Putting
  the second in the verifier makes config-first provisioning impossible.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Doctor check for the interface binding | config-verify with `interfaces` added to `ConfigRoots`; live `iface.Addresses` query inside verify | matches the established `checkDHCPInterfaces` precedent; avoids re-delivering ipsec config on every interface change; avoids rejecting a config that legitimately creates the interface it binds |
| Error, not warn, on an unresolvable responder CA | log a warning and continue | `exact-or-reject.md`: the operator asked for a trust anchor and would not get one. Continuing means every client is refused with an opaque TLS error |
| Extend `ValidateRemoteAccess` rather than add a parallel validator | new `ValidateRemoteAccessPKI` | one validator per config object; the caller already has the closures |

## Known Limitations
- The end-to-end QEMU rescue scenario from spec-fixit-codeql-security-triage still needs a
  purpose-built installer kernel. Not in scope here.
- The doctor check reports the interface named by `vpn ipsec interface` only. Per-peer
  `local-address` values are not probed.

## RFC Documentation

`// RFC 5216 Section 5.3: "Both sides MUST perform certificate path validation."` above the
remote-access trust-anchor requirement, matching the existing comment at `ipsec/validate.go:38-44`.

## Implementation Summary

### What Was Implemented
- `engine/responder_eap.go`: `eapTLSServerConfig` now refuses an EAP-TLS responder with no
  `ca-certificate`, and refuses one whose `ca-certificate` does not resolve in the PKI store,
  naming the CA. Previously both cases silently produced an empty trust pool.
- `eap/eap_tls_trust_anchor_test.go` (new): pins the empty-pool semantics the fix depends on,
  end-to-end through `newTLSMethod` and directly through `x509.Verify`.
- `engine/doctor.go` (new) + `Registration.DoctorChecks` in `engine/register.go`: a plugin-owned
  `ze doctor` check that reports a `vpn ipsec interface` absent from the host. This is what
  finally gives `ValidateInterfaceRef` a non-test caller.
- `core/diagnostic/codes.go`: registered `doctor-ipsec-iface`.
- `plan/learned/1255`: corrected the "fail-open" mischaracterisation (AC-13).
- `scripts/dev/stress-repro.py`: fixed two defects that blocked AC-14 (below).
- `test/reload/test-tx-ipsec-eap-tls-requires-ca.ci`: replaced a racy SIGTERM with an
  `await=stderr` fence.

### Bugs Found/Fixed
- **`stress-repro.py` crashed on every timeout.** `TimeoutExpired` carries undecoded bytes even
  under `text=True`, so the handler did `bytes + str` and raised `TypeError`, aborting the run.
  The tool could not report the failure class it exists to find. Hit on the first real
  invocation, because the 120s default guarantees a timeout for `bgp reload` under 64 burners.
- **`stress-repro.py` reported a usage error as a reproduction.** With `--any-failure`,
  `stress-repro.py reload` printed `*** REPRODUCED on invocation 1 ***` for
  `unknown command: reload`. Now exits 2 and names the likely cause.
- **`test-tx-ipsec-eap-tls-requires-ca` was racy, not load-sensitive-by-nature.** Its peer script
  sent SIGHUP and SIGTERM back-to-back, so under load the daemon was killed mid-reload and never
  logged the asserted rejection. This DISPROVES the hypothesis recorded in the known-failures
  entry (a closed plugin connection before verify dispatch): that signature appears nowhere in
  the capture.

### Documentation Updates
- `plan/known-failures/reload-transaction-tests-load-sensitive.md`: stress-run results, the
  disproved hypothesis, the test 34 fix, and the two reproducer defects.
- No `docs/` change: the responder refusal and the doctor check add no user-facing config or CLI
  surface, and `grep docs/ -e 'source: internal/component/ike'` returns no anchor on the changed
  files.

### Deviations from Plan
- **AC-4, AC-5, AC-7 moved** to `plan/spec-ipsec-remote-access.md`, and **AC-6 dropped**, after
  tracing showed the remote-access block is inert rather than merely unvalidated. Owner chose
  2026-07-23 to implement the feature. Recorded in `plan/deferrals/fixit-ipsec-verify-siblings.md`.
- **AC-3 repurposed** from a remote-access assertion to pinning the empty-pool semantics, which is
  the load-bearing fact behind the item-1 fix.
- **Item 5 grew a fix.** The spec said "run stress-repro and record the outcome". Running it
  required fixing the reproducer, and the outcome was a diagnosable, fixable race in a test this
  spec's predecessor wrote. Per `ai/rules/no-parking.md` both were fixed rather than recorded and
  left.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| 1. Responder no longer silently discards a configured trust anchor | Done | `ike/engine/responder_eap.go` | Errors naming the CA; RED then GREEN |
| 2. Remote-access gateway certificate references validated | Changed | `plan/deferrals/fixit-ipsec-verify-siblings.md` | Moved to `plan/spec-ipsec-remote-access.md`: the block is inert, so validating it would polish an operator trap. Owner chose to implement the feature |
| 3. `ValidateInterfaceRef` reachable from a user entry point | Done | `ike/engine/doctor.go`, `register.go` | Plugin-owned `ze doctor` check, proven both directions |
| 4. Correct learned 1255's mischaracterisation | Done | `plan/learned/1255-*.md` | Dated correction + two pinning tests |
| 5. Run `stress-repro.py` on reload and record the outcome | Done | `plan/known-failures/`, `RESOLVED.md` | Required fixing the reproducer first; outcome disproved the recorded hypothesis |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestEAPTLSServerConfigRefusesMissingCA` | RED: `CACertPEM=0 bytes`; GREEN after |
| AC-2 | Done | `TestEAPTLSServerConfigLoadsResolvableCA` | No-regression direction |
| AC-3 | Done | `TestEAPTLSAuthenticatorWithoutCARejectsEveryClient`, `TestEmptyClientCAPoolRejects` | Repurposed; pins the empty-pool semantics the fix rests on |
| AC-4 | Changed | -- | Moved to `plan/spec-ipsec-remote-access.md` |
| AC-5 | Changed | -- | Moved, as AC-4 |
| AC-6 | Skipped | -- | DROPPED as an invented requirement: `EAPUser.Certificate` has no runtime consumer |
| AC-7 | Changed | -- | Moved, as AC-4 |
| AC-8 | Done | `TestIPsecInterfaceDoctorCheckReportsMissing` + `ze doctor --json` exit 1 | Real entry point |
| AC-9 | Done | `TestIPsecInterfaceDoctorCheckAcceptsPresent` + `ze doctor --json` exit 0 | No false positive |
| AC-10 | Done | `TestIPsecInterfaceDoctorCheckQuietWithoutConfig` | 5 sub-cases incl. nil/wrong-type tree |
| AC-11 | Done | `./bin/ze explain doctor-ipsec-iface` | Resolves with title + description |
| AC-12 | Done | `TestIPsecInterfaceDoctorCheckRegistered` | Non-test caller via `registry.PluginDoctorChecks()` |
| AC-13 | Done | `plan/learned/1255-*.md` | Dated correction appended |
| AC-14 | Done | `tmp/stress-repro/bgp-reload-*.log` | Reproduced on invocation 1; after the fix `--test 34` survived 40/40 at 64 burners |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|-----------|-------|
| Responder trust anchor | Done | `ike/engine/responder_eap_test.go` | 3 tests |
| Empty-pool semantics | Done | `ike/eap/eap_tls_trust_anchor_test.go` | 2 tests, one end-to-end handshake |
| Doctor check | Done | `ike/engine/doctor_test.go` | 5 tests incl. malformed-name and registration |
| Remote-access validation | Changed | -- | Moved with AC-4/5/7 |
| Functional `.ci` | Changed | `test/reload/test-tx-ipsec-eap-tls-requires-ca.ci` | The planned new `.ci` moved with the deferral; the EXISTING reload `.ci` was made deterministic instead |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `ike/engine/doctor.go` + `_test.go` | Done | As planned |
| `ike/engine/responder_eap.go` | Done | As planned |
| `core/diagnostic/codes.go` | Done | `doctor-ipsec-iface` |
| `ipsec/validate.go`, `engine/config.go` | Changed | Untouched: their work moved to the remote-access spec |
| `test/reload/test-tx-ipsec-remote-access-requires-ca.ci` | Changed | Moved to the remote-access spec |
| Unplanned additions | Done | `cmd/ze/hub/main_reload.go`, `internal/test/runner/caps_*.go`, `scripts/dev/stress-repro.py`, `scripts/dev/commit_helper.py`, 9 `.ci` files -- all root-cause fixes for reds found while executing item 5 |

### Audit Summary
- **Total items:** 5 task requirements + 14 ACs
- **Done:** 4 requirements, 10 ACs
- **Partial:** none
- **Skipped:** AC-6 only, dropped as an invented requirement (no runtime consumer); recorded in the deferral shard
- **Changed:** requirement 2 and AC-4/5/7 moved to `plan/spec-ipsec-remote-access.md` under an owner decision; AC-3 repurposed. Item 5 grew root-cause fixes rather than a written record, per the owner directive of 2026-07-23

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| responder no longer swallows an unresolvable CA | unit test, RED then GREEN | `TestEAPTLSServerConfigRefusesMissingCA` failed with `CACertPEM=0 bytes` before the fix (`tmp/ipsec-a6027ea9/red1.txt`), passes after. `TestEAPTLSServerConfigLoadsResolvableCA` guards the no-regression direction |
| the responder's failure mode is understood, not assumed | end-to-end + stdlib pin | `TestEAPTLSAuthenticatorWithoutCARejectsEveryClient` drives a real handshake through `newTLSMethod`; `TestEmptyClientCAPoolRejects` pins `x509.Verify` against an empty non-nil pool |
| remote-access certificate refs validated at commit | -- | MOVED to `plan/spec-ipsec-remote-access.md` (see Deviations) |
| `ValidateInterfaceRef` reachable from a user entry point | `ze doctor` output, both directions | `./bin/ze explain doctor-ipsec-iface` resolves; `ze doctor --json` on a config naming `eth-does-not-exist` exits 1 emitting `doctor-ipsec-iface` with `vpn ipsec interface not found: eth-does-not-exist (peers without an explicit local-address will not establish)`; the same config naming a real interface exits 0 with no such diagnostic |
| learned 1255 no longer misstates the responder defect | grep | the "fail-open" bullet now carries a dated correction naming the empty-pool measurement and the two pinning tests |
| reload load-sensitivity characterised | `stress-repro.py` log, before and after | before: reproduced on invocation 1, 5 failures + 20 timeouts (`tmp/stress-repro/bgp-reload-20260723-005229.log`). After the fence: `--test 34` **not reproduced in 40 invocations** at 64 burners / 32 cores (`tmp/stress-repro/bgp-reload-34-20260723-010212.log`) |
| the whole affected surface still passes | unit + functional | 14/14 packages ok (`ike/...`, `core/diagnostic`, `component/doctor`, `test/runner`); `bgp reload` suite: test 34 passes, remaining failures are the documented pre-existing `[2, 18]` (`reload-commit-transactional-and-apply-ordering.md`) and `[27,28,30,31,32,33]` (`reload-iface-tunnel-wireguard-cap-net-admin.md`) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

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
| `internal/component/ike/engine/doctor.go` | yes | `git status` shows it untracked-new; compiles into `bin/ze` |
| `internal/component/ike/engine/doctor_test.go` | yes | 5 tests run and pass |
| `internal/component/ike/engine/responder_eap_test.go` | yes | 3 tests run and pass |
| `internal/component/ike/eap/eap_tls_trust_anchor_test.go` | yes | 2 tests run and pass |
| `internal/test/runner/caps_linux.go`, `caps_other.go`, `caps_option_test.go` | yes | `go vet` clean; 3 tests pass |
| `test/reload/test-tx-ipsec-eap-tls-requires-ca.ci` | yes | `ze-test bgp reload 34` PASS |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | unresolvable CA is an error | `tmp/ipsec-a6027ea9/red1.txt` shows the pre-fix failure text `CACertPEM=0 bytes`; post-fix run exits 0 |
| AC-3 | empty pool rejects | `TestEmptyClientCAPoolRejects` + measured `x509: certificate signed by unknown authority` |
| AC-8/AC-9 | doctor reports and stays quiet correctly | `./bin/ze doctor --json` exit 1 with `vpn ipsec interface not found: eth-does-not-exist`; same config with a real interface exits 0 with no `doctor-ipsec-iface` |
| AC-11 | code is explainable | `./bin/ze explain doctor-ipsec-iface` prints title + description |
| AC-12 | non-test caller exists | `grep -rn ValidateInterfaceRef` now shows `ike/engine/doctor.go`; chain `ze doctor` -> `runPluginRegistryChecks` (`component/doctor/checks_plugin_registry.go`) -> `checkIPsecInterface` |
| AC-14 | stress outcome recorded and acted on | before: reproduced on invocation 1; after: `--test 34` not reproduced in 40 invocations at 64 burners / 32 cores |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| reload transaction `OnConfigVerify` | `test/reload/test-tx-ipsec-eap-tls-requires-ca.ci` | yes -- read; asserts the rejection message and now fences on it with `await=stderr` instead of racing SIGTERM |
| `ze doctor --json` | (unit + live binary) | yes -- run against a real config both directions |
| `ze explain` | (live binary) | yes |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | empty non-nil `x509.CertPool` rejects; pinned by `TestEmptyClientCAPoolRejects` and an end-to-end handshake test |
| A-2 | confirmed | `DoctorCheckContext.Tree` carries a `*config.Tree`; the doctor tests build one and the live `ze doctor` run proves it end to end |
| A-3 | confirmed | `test/parse/ipsec-eap-auth.ci` configures remote-access eap-tls with no `pki` section but uses offline `ze config validate`, which does not invoke plugin verify; it stays green |
| A-4 | confirmed | `ValidateRemoteAccess` has one non-test caller (`engine/config.go:143`) plus 3 test call sites -- and was left untouched once the work moved |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No `docs/` change needed for the responder/doctor work | `grep docs/ -e 'source: internal/component/ike'` returns no anchor on the changed files | yes |
| `option=needs-linux` gained `caps=` | `docs/architecture/testing/ci-format.md` row updated | yes |
| Doc gates green | `make ze-doc-test` PASSED after regenerating `ai/DOCS-TO-CODE.md` | yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING - before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
