# Spec: ipsec-3 -- IPsec Data Model

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-05-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `spec-ipsec-0-umbrella.md` -- umbrella design decisions and deployment model
4. `internal/component/l2tp/config.go` -- config parser pattern (ExtractParameters from tree)
5. `internal/component/l2tp/schema/ze-l2tp-conf.yang` -- YANG schema pattern
6. `internal/component/iface/tunnel.go` -- TunnelKind enum pattern for algorithm enums

## Task

Define the YANG schema and Go types for the `vpn ipsec {}` configuration tree.
This covers IKE groups (proposals, DPD, key-exchange, lifetime, close-action),
ESP groups (proposals, PFS, lifetime), interface binding, and site-to-site peers
(authentication modes, connection type, local/remote address, IKE/ESP group
references, VTI bind). No runtime/strongSwan code here, just the data model,
config parser, and validation.

The reference config (from `../home.conf`) defines:
- ESP group `ESP-RW`: AES-128-GCM, SHA-256, PFS disabled, 86400s lifetime
- IKE group `IKE-RW`: IKEv2, DH-14, AES-128-GCM, SHA-256, DPD restart at 10/30s, close-action start, lifetime 0 (no reauth)
- Interface binding: pppoe0
- Site-to-site peer `management-bridge`: X.509 auth (ECDSA certs from `pki {}`), connection-type initiate, DNS remote, VTI bind

This spec produces typed Go structs that spec ipsec-4 (strongSwan integration)
consumes to generate swanctl.conf. The parser must validate cross-references
(IKE/ESP group names, PKI certificate names, interface names) at load time.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- component lifecycle, registration pattern
  -> Constraint: IPsec component registers via init() in register.go, YANG via schema/register.go
- [ ] `docs/features/interfaces.md` -- interface types, YANG schema conventions
  -> Constraint: interface binding leaf must reference a real interface kind
- [ ] `internal/component/l2tp/config.go` -- config parser pattern (tree walker, typed extraction)
  -> Decision: follow the same pattern: ExtractX functions that walk map[string]any subtrees
- [ ] `internal/component/l2tp/schema/ze-l2tp-conf.yang` -- YANG module structure for a subsystem
  -> Constraint: top-level container under root, lists with named keys, typed leaves
- [ ] `internal/component/iface/tunnel.go` -- TunnelKind enum pattern (iota + string map + parser)
  -> Decision: encryption, hash, and DH-group algorithms use the same enum pattern
- [ ] `internal/component/config/secret/secret.go` -- $9$ encoding for ze:sensitive leaves
  -> Constraint: PSK pre-shared-key leaf uses ze:sensitive, auto-decoded on load

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7296.md` -- IKEv2 (proposal format, transform types, DH groups)
  -> Constraint: algorithm enums and proposal structure must match IKEv2 transform types
- [ ] `rfc/short/rfc4301.md` -- IPsec Security Architecture (SA concept, SPD)
  -> Constraint: SA lifetime semantics (time-based, byte-based, 0 = no reauth)
- [ ] `rfc/short/rfc6071.md` -- IPsec/IKE Roadmap (algorithm requirements)
  -> Constraint: algorithm support matrix aligned with RFC recommendations

**Key insights:**
- L2TP config parser is the closest pattern: top-level container, nested lists, typed extraction
- TunnelKind enum pattern (iota + string map + ParseX) works for algorithm enums
- $9$ encoding already handles PSK; same ze:sensitive annotation as wireguard private-key
- YANG lists keyed by name (like `esp-group ESP-RW`) follow the same pattern as `tunnel gre0`
- Proposal lists keyed by number (like `proposal 10`) are YANG list with integer key
- Cross-reference validation (ike-group name, esp-group name, pki cert name) is Go-side, not YANG

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/config.go` -- ExtractParameters walks the config tree, produces Parameters struct. Validates ranges, cross-references. Returns detailed errors on invalid config
  -> Constraint: IPsec parser follows the same pattern: walk tree, produce typed struct, validate
- [ ] `internal/component/l2tp/schema/ze-l2tp-conf.yang` -- top-level `l2tp {}` container with nested lists. Uses `ze:sensitive`, `ze:required`, `ze:validate` annotations
  -> Constraint: IPsec YANG uses the same ze: extensions
- [ ] `internal/component/iface/tunnel.go` -- TunnelKind is iota enum with tunnelKindNames map and ParseTunnelKind. No exported String() beyond the map
  -> Decision: EncryptionAlgo, HashAlgo, DHGroup follow the same pattern
- [ ] `internal/component/iface/config.go` -- parseTunnelEntry reads from map[string]any, uses parseTunnelLeaves helper. Validates via Go-side checks (not YANG enforcement alone)
  -> Constraint: parseIKEGroup, parseESPGroup, parseSiteToSitePeer follow the same pattern
- [ ] `internal/component/config/secret/secret.go` -- Encode/Decode/IsEncoded for $9$ JunOS obfuscation
  -> Constraint: PSK leaf is ze:sensitive, decoded transparently by config parser
- [ ] `internal/component/iface/wireguard.go` -- WireguardSpec/WireguardPeerSpec with nested peer list
  -> Decision: SiteToSitePeer has nested AuthConfig struct, similar to WireguardPeerSpec nesting

**Behavior to preserve:**
- Existing config parser infrastructure (tree walkers, error wrapping) unchanged
- YANG schema registration pattern (schema/ package with embed.go + register.go) unchanged
- $9$ sensitive leaf handling unchanged
- All existing `vpn` or `ipsec` references in config (none exist today, so no conflict)

**Behavior to change:**
- New top-level `vpn { ipsec {} }` YANG container
- New Go types: IKEGroup, ESPGroup, IKEProposal, ESPProposal, DPDConfig, SiteToSitePeer, AuthConfig
- New algorithm enums: EncryptionAlgo, HashAlgo, DHGroup, PFSMode, AuthMode, ConnectionType, CloseAction, DPDAction
- New config parser functions: ParseIPsecConfig, parseIKEGroup, parseESPGroup, parseSiteToSitePeer
- New validation: cross-reference checking (group names, PKI names, interface names)

## Data Flow (MANDATORY)

### Entry Point
- Config load/reload: YANG tree produces `map[string]any` for the `vpn/ipsec` subtree
- The IPsec component's OnConfigure callback receives the subtree and calls ParseIPsecConfig

### Transformation Path
1. Config loader parses YANG, produces tree with `vpn/ipsec` subtree
2. ParseIPsecConfig walks the subtree, extracting esp-group, ike-group, site-to-site/peer maps
3. parseESPGroup builds ESPGroup struct from leaf values (lifetime, pfs, proposals)
4. parseIKEGroup builds IKEGroup struct from leaf values (key-exchange, lifetime, DPD, close-action, proposals)
5. parseSiteToSitePeer builds SiteToSitePeer struct (auth, connection-type, addresses, group refs, VTI bind)
6. Cross-reference validation: IKE/ESP group names resolve, PKI cert names resolve, interface exists
7. IPsecConfig struct returned to caller (spec ipsec-4 consumes it for swanctl.conf generation)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG tree to Go structs | Tree walker parses map[string]any leaves into typed fields | [ ] |
| IPsec component to PKI component | PKI store queried by name to validate cert references | [ ] |
| IPsec component to iface component | Interface list queried to validate interface binding leaf | [ ] |

### Integration Points
- `config.OnConfigure` callback -- receives config tree, calls ParseIPsecConfig
- `pki.Store` (from ipsec-1) -- ValidateCertRef checks ca-certificate and certificate names exist
- `iface` -- interface binding leaf validated against known interface names
- spec ipsec-4 consumes the typed structs to generate swanctl.conf

### Architectural Verification
- [ ] No bypassed layers (parser uses standard tree-walker, not raw file I/O)
- [ ] No unintended coupling (IPsec types in ipsec package, not leaked into iface or config)
- [ ] No duplicated functionality (algorithm enums are new, not duplicating existing enums)
- [ ] Zero-copy preserved where applicable (string values from tree, no extra copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config load with `vpn { ipsec { esp-group ... } }` | -> | parseESPGroup returns typed ESPGroup | `test/parse/ipsec-esp-group.ci` |
| Config load with `vpn { ipsec { ike-group ... } }` | -> | parseIKEGroup returns typed IKEGroup | `test/parse/ipsec-ike-group.ci` |
| Config load with `vpn { ipsec { site-to-site { peer ... } } }` | -> | parseSiteToSitePeer returns typed peer | `test/parse/ipsec-peer.ci` |
| Config load with invalid algorithm | -> | ParseIPsecConfig returns error | `test/parse/ipsec-invalid-algo.ci` |
| Config load with missing group reference | -> | ParseIPsecConfig returns error | `test/parse/ipsec-missing-group-ref.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with `esp-group` containing proposals with encryption and hash, lifetime, PFS mode | ESPGroup struct populated with all fields, proposals ordered by number |
| AC-2 | Config with `ike-group` containing proposals, DPD config, key-exchange, lifetime, close-action | IKEGroup struct populated with all fields including nested DPDConfig |
| AC-3 | Config with site-to-site peer with auth mode, connection-type, addresses, group refs, VTI bind | SiteToSitePeer struct populated with all fields including nested AuthConfig |
| AC-4 | X.509 auth with ca-certificate and certificate names | Names validated against PKI store; error if either name not found |
| AC-5 | PSK auth with $9$-encoded pre-shared key | Key decoded transparently; stored as plaintext in AuthConfig.PSK field |
| AC-6 | ESP proposal with unsupported encryption algorithm (e.g., `des`) | Config load returns descriptive error naming the invalid algorithm |
| AC-7 | IKE proposal with invalid DH group number (e.g., 0 or 99) | Config load returns descriptive error naming the invalid DH group |
| AC-8 | Peer references `ike-group NONEXISTENT` | Config load returns error: IKE group "NONEXISTENT" not defined |
| AC-9 | `interface` binding leaf references nonexistent interface | Config load returns error: interface "ethXX" not found |
| AC-10 | Config reload changes peer remote-address or auth config | Diff detects change; IPsecConfig.Changed() reports affected peer names |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseESPGroup` | `internal/component/ipsec/config_test.go` | ESP group with single and multiple proposals | |
| `TestParseESPGroupDefaults` | `internal/component/ipsec/config_test.go` | Default lifetime, default PFS mode | |
| `TestParseIKEGroup` | `internal/component/ipsec/config_test.go` | IKE group with proposals, DPD, close-action | |
| `TestParseIKEGroupDPD` | `internal/component/ipsec/config_test.go` | DPD action/interval/timeout extraction | |
| `TestParseSiteToSitePeerX509` | `internal/component/ipsec/config_test.go` | X.509 auth with local-id, remote-id, cert refs | |
| `TestParseSiteToSitePeerPSK` | `internal/component/ipsec/config_test.go` | PSK auth with decoded key | |
| `TestParseSiteToSitePeerVTI` | `internal/component/ipsec/config_test.go` | VTI bind extraction | |
| `TestParseInvalidEncryption` | `internal/component/ipsec/config_test.go` | Rejected: unknown encryption algorithm | |
| `TestParseInvalidDHGroup` | `internal/component/ipsec/config_test.go` | Rejected: DH group 0, 99 | |
| `TestParseMissingGroupRef` | `internal/component/ipsec/config_test.go` | Rejected: ike-group/esp-group name not found | |
| `TestParseInvalidInterfaceRef` | `internal/component/ipsec/config_test.go` | Rejected: interface binding not found | |
| `TestEncryptionAlgoString` | `internal/component/ipsec/types_test.go` | Enum String() and Parse round-trip | |
| `TestHashAlgoString` | `internal/component/ipsec/types_test.go` | Enum String() and Parse round-trip | |
| `TestDHGroupString` | `internal/component/ipsec/types_test.go` | Enum String() and Parse round-trip | |
| `TestIPsecConfigChanged` | `internal/component/ipsec/config_test.go` | Diff detection for changed peers | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ESP lifetime | 0-86400 | 86400 | N/A (0 = no expiry) | 86401 |
| IKE lifetime | 0-86400 | 86400 | N/A (0 = no reauth) | 86401 |
| DPD interval | 1-3600 | 3600 | 0 | 3601 |
| DPD timeout | 1-3600 | 3600 | 0 | 3601 |
| Proposal number | 1-65535 | 65535 | 0 | 65536 |
| DH group | 1-31 | 31 | 0 | 32 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-esp-group` | `test/parse/ipsec-esp-group.ci` | ESP group config accepted by parser | |
| `ipsec-ike-group` | `test/parse/ipsec-ike-group.ci` | IKE group config accepted by parser | |
| `ipsec-peer` | `test/parse/ipsec-peer.ci` | Site-to-site peer config accepted | |
| `ipsec-invalid-algo` | `test/parse/ipsec-invalid-algo.ci` | Invalid algorithm rejected with descriptive error | |
| `ipsec-missing-group-ref` | `test/parse/ipsec-missing-group-ref.ci` | Missing group reference rejected | |

## Files to Modify
- `cmd/ze/hub/main.go` -- import ipsec schema package to trigger init() registration

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new container) | [Yes] | `internal/component/ipsec/schema/ze-ipsec-conf.yang` |
| CLI commands/flags | [No] | CLI comes in ipsec-5 |
| Editor autocomplete | [Yes] | YANG-driven (automatic if YANG updated) |
| Functional test for config parsing | [Yes] | `test/parse/ipsec-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [Yes] | `docs/features.md` -- add IPsec data model row |
| 2 | Config syntax changed? | [Yes] | `docs/guide/configuration.md` -- add `vpn ipsec {}` section |
| 3 | CLI command added/changed? | [No] | |
| 4 | API/RPC added/changed? | [No] | |
| 5 | Plugin added/changed? | [No] | |
| 6 | Has a user guide page? | [Yes] | `docs/guide/ipsec.md` -- new page with config examples |
| 7 | Wire format changed? | [No] | |
| 8 | Plugin SDK/protocol changed? | [No] | |
| 9 | RFC behavior implemented? | [Yes] | `rfc/short/rfc7296.md` -- IKEv2 transform types |
| 10 | Test infrastructure changed? | [No] | |
| 11 | Affects daemon comparison? | [Yes] | `docs/comparison.md` -- IPsec support row |
| 12 | Internal architecture changed? | [No] | |

## Files to Create
- `internal/component/ipsec/types.go` -- IKEGroup, ESPGroup, IKEProposal, ESPProposal, DPDConfig, SiteToSitePeer, AuthConfig structs. EncryptionAlgo, HashAlgo, DHGroup, PFSMode, AuthMode, ConnectionType, CloseAction, DPDAction enums
- `internal/component/ipsec/config.go` -- ParseIPsecConfig, parseESPGroup, parseIKEGroup, parseSiteToSitePeer, parseAuthConfig. Tree-walker pattern
- `internal/component/ipsec/validate.go` -- ValidateGroupRefs, ValidatePKIRefs, ValidateInterfaceRef. Cross-reference validation
- `internal/component/ipsec/register.go` -- blank import of schema package for init() registration
- `internal/component/ipsec/schema/ze-ipsec-conf.yang` -- YANG module for `vpn { ipsec {} }` tree
- `internal/component/ipsec/schema/embed.go` -- go:embed for YANG file
- `internal/component/ipsec/schema/register.go` -- yang.RegisterModule in init()
- `internal/component/ipsec/config_test.go` -- unit tests for parser
- `internal/component/ipsec/types_test.go` -- unit tests for enum round-trips
- `test/parse/ipsec-esp-group.ci` -- functional test: ESP group accepted
- `test/parse/ipsec-ike-group.ci` -- functional test: IKE group accepted
- `test/parse/ipsec-peer.ci` -- functional test: peer accepted
- `test/parse/ipsec-invalid-algo.ci` -- functional test: invalid algorithm rejected
- `test/parse/ipsec-missing-group-ref.ci` -- functional test: missing ref rejected

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table -- register YANG, create parse entry point, write failing wiring tests |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- register YANG schema, create skeleton parser
   - Tests: `test/parse/ipsec-esp-group.ci` (failing: parser stub returns error)
   - Files: `register.go`, `schema/`, `config.go` skeleton
   - Verify: YANG schema registered, parse entry point reachable, wiring test fails because parser is a stub

2. **Phase: Algorithm Enums** -- define encryption, hash, DH group enums
   - Tests: `TestEncryptionAlgoString`, `TestHashAlgoString`, `TestDHGroupString`
   - Files: `types.go`, `types_test.go`
   - Verify: enum round-trip String/Parse works for all supported algorithms

3. **Phase: ESP Group Parser** -- parse esp-group list entries with proposals
   - Tests: `TestParseESPGroup`, `TestParseESPGroupDefaults`, `TestParseInvalidEncryption`
   - Files: `config.go` (parseESPGroup), `config_test.go`
   - Verify: ESP group with proposals parsed; invalid algorithm rejected

4. **Phase: IKE Group Parser** -- parse ike-group list entries with proposals and DPD
   - Tests: `TestParseIKEGroup`, `TestParseIKEGroupDPD`, `TestParseInvalidDHGroup`
   - Files: `config.go` (parseIKEGroup), `config_test.go`
   - Verify: IKE group with DPD and proposals parsed; invalid DH group rejected

5. **Phase: Site-to-Site Peer Parser** -- parse peer entries with auth, VTI, group refs
   - Tests: `TestParseSiteToSitePeerX509`, `TestParseSiteToSitePeerPSK`, `TestParseSiteToSitePeerVTI`
   - Files: `config.go` (parseSiteToSitePeer, parseAuthConfig), `config_test.go`
   - Verify: peers with X.509 and PSK auth parsed; VTI bind extracted

6. **Phase: Cross-Reference Validation** -- validate group refs, PKI refs, interface refs
   - Tests: `TestParseMissingGroupRef`, `TestParseInvalidInterfaceRef`
   - Files: `validate.go`, `config_test.go`
   - Verify: missing references produce descriptive errors

7. **Phase: Config Diff** -- detect changed peers on reload
   - Tests: `TestIPsecConfigChanged`
   - Files: `config.go` (IPsecConfig.Changed)
   - Verify: diff correctly identifies changed, added, and removed peers

8. **Functional tests** -- create .ci tests for end-user scenarios
9. **Full verification** -- `make ze-verify`
10. **Complete spec** -- audit, learned summary, delete spec

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Algorithm enum values match strongSwan swanctl.conf naming (spec ipsec-4 depends on this) |
| Naming | YANG leaves use kebab-case; Go types use CamelCase; enum strings match YANG values |
| Data flow | Parser only reads from tree, never writes back. No side effects during parsing |
| Rule: no-layering | No duplicate algorithm definitions; single source of truth in types.go |
| Rule: exact-or-reject | Unknown algorithm names rejected, not silently defaulted |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| YANG schema for `vpn { ipsec {} }` | `ls internal/component/ipsec/schema/ze-ipsec-conf.yang` |
| Algorithm enum types with String/Parse | `grep -n 'func.*EncryptionAlgo.*String' internal/component/ipsec/types.go` |
| ESP group parser | `grep -n 'func parseESPGroup' internal/component/ipsec/config.go` |
| IKE group parser | `grep -n 'func parseIKEGroup' internal/component/ipsec/config.go` |
| Site-to-site peer parser | `grep -n 'func parseSiteToSitePeer' internal/component/ipsec/config.go` |
| Cross-reference validation | `grep -n 'func Validate' internal/component/ipsec/validate.go` |
| Config diff detection | `grep -n 'func.*Changed' internal/component/ipsec/config.go` |
| 15 unit tests pass | `go test ./internal/component/ipsec/...` |
| 5 functional tests pass | `ls test/parse/ipsec-*.ci` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | All leaf values validated against allowed enums; no raw string passthrough to swanctl.conf |
| PSK handling | PSK stored only in memory after $9$ decode; never logged; never included in show output without redaction |
| Certificate references | Names validated against PKI store; no path traversal in certificate name strings |
| Integer overflow | Proposal numbers, lifetime, DPD interval/timeout validated against YANG ranges before casting |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior; RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural, DESIGN phase |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user |

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

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, algorithm requirements, proposal format constraints.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

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
-

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-ipsec-3-data-model.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary
