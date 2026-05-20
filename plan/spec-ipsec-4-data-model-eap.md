# Spec: ipsec-4 -- IPsec Data Model EAP Extension

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | ipsec-3 |
| Phase | 1/4 |
| Updated | 2026-05-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `spec-ipsec-0-umbrella.md` -- umbrella design decisions
4. `plan/learned/734-ipsec-3-data-model.md` -- ipsec-3 learned summary
5. `internal/component/ipsec/config.go` -- existing IPsec config parser
6. `internal/component/ipsec/types.go` -- existing IPsec types

## Task

Extend the IPsec data model (ipsec-3, done) with EAP authentication configuration,
remote-access peer type, and virtual IP pool management. This enables road warrior
VPN from Windows, macOS, iOS, and Android clients using their built-in IKEv2 support.

The existing data model covers IKE groups, ESP groups, and site-to-site peers with
X.509 and PSK authentication. This spec adds EAP authentication methods (EAP-TLS
and EAP-MSCHAPv2), a remote-access peer type that accepts multiple clients, virtual
IP pool configuration for assigning addresses to road warrior clients via the IKEv2
Configuration Payload (RFC 7296 Section 2.19), and DNS/domain push.

## Required Reading

### Architecture Docs
- [ ] `plan/learned/734-ipsec-3-data-model.md` -- what ipsec-3 decided, constraints, gotchas
  -> Constraint: follow the same parser pattern (tree walker, typed structs, validation)
- [ ] `internal/component/ipsec/types.go` -- existing type definitions
  -> Constraint: extend, do not break existing SiteToSitePeer struct
- [ ] `internal/component/ipsec/config.go` -- existing config parser
  -> Constraint: add EAP parsing alongside existing x509/psk authentication parsing
- [ ] `internal/component/config/secret/secret.go` -- $9$ encoding
  -> Constraint: EAP passwords use $9$ encoding

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7296.md` -- IKEv2 Configuration Payload (Section 2.19) for virtual IP
- [ ] `rfc/short/rfc3748.md` -- EAP framework, method types
- [ ] `rfc/short/rfc5216.md` -- EAP-TLS
- [ ] `rfc/short/rfc2759.md` -- MS-CHAPv2

**Key insights:**
- EAP-MSCHAPv2 (type 26) is the Windows built-in IKEv2 default for password auth
- EAP-TLS (type 13) provides certificate-based client auth inside EAP
- IKEv2 Configuration Payload pushes virtual IP, DNS, and domain to clients
- $9$ encoding for EAP passwords follows existing pattern for wireguard keys and PPPoE passwords

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ipsec/types.go` -- IKEGroup, ESPGroup, SiteToSitePeer, AuthMode enum (x509, psk)
- [ ] `internal/component/ipsec/config.go` -- parseIKEGroup, parseESPGroup, parseSiteToSitePeer
- [ ] `internal/component/ipsec/config_test.go` -- existing parser tests
- [ ] `internal/component/ipsec/schema/` -- YANG schema for vpn ipsec

**Behavior to preserve:**
- All existing IKE/ESP group parsing unchanged
- Existing site-to-site peer parsing for x509/psk auth unchanged
- Existing validation rules unchanged

**Behavior to change:**
- AuthMode enum extended with EAPTls, EAPMSCHAPv2
- SiteToSitePeer gains optional EAP credential fields
- New RemoteAccessConfig struct for pool + EAP user database
- New VirtualIPPool struct with range, DNS, domain
- YANG schema extended with remote-access container and eap auth mode

## Data Flow (MANDATORY)

### Entry Point
- Config load/reload: YANG tree contains `vpn ipsec { remote-access { ... } }`

### Transformation Path
1. Config parser walks YANG tree, finds `remote-access` container
2. `parseRemoteAccess` extracts pool config and EAP user entries
3. EAP passwords decoded from $9$ by existing secret.Decode
4. Parsed entries stored in RemoteAccessConfig struct
5. Validation checks pool ranges, credential completeness

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG tree to Go struct | parseRemoteAccess tree walker | [ ] |
| $9$ encoded password to cleartext | secret.Decode (existing) | [ ] |

### Integration Points
- `internal/component/ipsec/types.go` -- extended with EAP types
- `internal/component/config/secret/` -- $9$ decoding for passwords
- `internal/component/ike/eap/` (ipsec-9) -- consumes EAP config at runtime

### Architectural Verification
- [ ] No bypassed layers (config parsing uses existing tree walker)
- [ ] No unintended coupling (data model defines types, engine consumes them)
- [ ] No duplicated functionality (extends existing ipsec config, does not recreate)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config load with `vpn ipsec { remote-access { pool ... } }` | -> | parseRemoteAccess produces RemoteAccessConfig | `test/parse/ipsec-remote-access.ci` |
| Config load with `authentication { mode eap-mschapv2 }` | -> | AuthMode parsed as EAPMSCHAPv2 | `test/parse/ipsec-eap-auth.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config has `remote-access { pool ... { range 10.10.0.0/24 } }` | Pool parsed with valid CIDR, stored in RemoteAccessConfig |
| AC-2 | Config has `authentication { mode eap-mschapv2 }` | AuthMode parsed as EAPMSCHAPv2 |
| AC-3 | Config has `eap-user thomas { password "$9$..." }` | User parsed, password decoded from $9$ |
| AC-4 | Config has `authentication { mode eap-tls }` | AuthMode parsed as EAPTls, requires certificate reference |
| AC-5 | Config has pool with `dns` and `domain` | DNS servers and search domain parsed |
| AC-6 | Pool range overlaps with existing interface address | Validation rejects with clear error |
| AC-7 | EAP-MSCHAPv2 user without password | Validation rejects (password mandatory for mschapv2) |
| AC-8 | Pool with both IPv4 range and IPv6 range6 | Both parsed, dual-stack pool supported |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseRemoteAccessPool` | `internal/component/ipsec/config_test.go` | Pool parsing with range, DNS, domain | |
| `TestParseEAPMSCHAPv2Auth` | `internal/component/ipsec/config_test.go` | EAP-MSCHAPv2 auth mode and user parsing | |
| `TestParseEAPTLSAuth` | `internal/component/ipsec/config_test.go` | EAP-TLS auth mode with certificate reference | |
| `TestValidatePoolRange` | `internal/component/ipsec/config_test.go` | Pool CIDR validation, overlap detection | |
| `TestValidateEAPUserPassword` | `internal/component/ipsec/config_test.go` | Missing password rejected for mschapv2 | |
| `TestDualStackPool` | `internal/component/ipsec/config_test.go` | IPv4 + IPv6 pool parsed correctly | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Pool prefix length (IPv4) | /8 - /30 | /30 | /31 (too small) | /7 (too large) |
| Pool prefix length (IPv6) | /48 - /126 | /126 | /127 (too small) | /47 (too large) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-remote-access` | `test/parse/ipsec-remote-access.ci` | Remote access config accepted and parsed | |
| `ipsec-eap-auth` | `test/parse/ipsec-eap-auth.ci` | EAP authentication config accepted | |

## Files to Modify
- `internal/component/ipsec/types.go` -- extend AuthMode enum, add EAP/pool types
- `internal/component/ipsec/config.go` -- add EAP and remote-access parsing
- `internal/component/ipsec/config_test.go` -- add EAP parsing tests
- `internal/component/ipsec/validate.go` -- add EAP and pool validation
- `internal/component/ipsec/schema/ze-ipsec-conf.yang` -- add remote-access container

## Files to Create
- `test/parse/ipsec-remote-access.ci` -- functional test for remote-access config
- `test/parse/ipsec-eap-auth.ci` -- functional test for EAP auth config

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

1. **Phase: Wiring (MANDATORY FIRST)** -- register remote-access parsing, write failing wiring tests
   - Tests: `test/parse/ipsec-remote-access.ci`
   - Files: `types.go` (new types), `config.go` (parser entry point)
   - Verify: entry point exists; wiring test fails because parser returns empty struct

2. **Phase: EAP Types and Pool Config** -- AuthMode extension, RemoteAccessConfig, VirtualIPPool
   - Tests: `TestParseRemoteAccessPool`, `TestParseEAPMSCHAPv2Auth`, `TestParseEAPTLSAuth`
   - Files: `types.go`, `config.go`, `ze-ipsec-conf.yang`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Validation** -- pool range validation, credential completeness
   - Tests: `TestValidatePoolRange`, `TestValidateEAPUserPassword`
   - Files: `validate.go`
   - Verify: invalid configs rejected with clear errors

4. **Functional tests** -- create after feature works
5. **Full verification** -- `make ze-verify`
6. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-8 has implementation with file:line |
| Correctness | $9$ decoding works for EAP passwords; pool overlap detection correct |
| Naming | AuthMode values follow existing naming convention |
| Data flow | Config parsing only; no runtime EAP logic in this spec |
| Rule: no-layering | EAP types extend existing AuthMode, not a separate enum |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| EAP auth modes exist | `grep -rn 'EAPTls\|EAPMSCHAPv2' internal/component/ipsec/types.go` |
| RemoteAccessConfig type exists | `grep -rn 'RemoteAccessConfig' internal/component/ipsec/types.go` |
| Pool parsing works | `go test -run TestParseRemoteAccessPool` |
| YANG schema updated | `grep -rn 'remote-access' internal/component/ipsec/schema/` |
| Functional tests exist | `ls test/parse/ipsec-remote-access.ci test/parse/ipsec-eap-auth.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Pool CIDR ranges validated; EAP usernames validated for length/characters |
| Secret handling | EAP passwords stored with $9$ encoding; never logged in cleartext |
| Resource exhaustion | Pool size bounded by prefix length; no unbounded allocation |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural then DESIGN phase |
| Functional test fails | Check AC; if AC wrong then DESIGN; if AC correct then IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
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

## Implementation Summary

### What Was Implemented
- (to be filled)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

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

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- (to be filled)

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (Review Gate section filled)
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ipsec-4-data-model-eap.md`
- [ ] Summary included in commit
