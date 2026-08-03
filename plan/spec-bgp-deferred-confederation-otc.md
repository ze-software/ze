# Spec: bgp-deferred-confederation-otc

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/plugins/role/otc.go` - OTC ingress/egress processing
4. `internal/component/bgp/yang/ze-bgp-conf.yang` - BGP config surface (local AS)

## Task

Implement the RFC 9234 Section 5 AS Confederation rules for the Only-To-Customer
(OTC) attribute: on egress from an AS Confederation the OTC value MUST equal the
AS Confederation Identifier, and MUST NOT carry any Member-AS number other than
that identifier.

**Provenance:** deferred from `plan/spec-followup-bgp-feature.md` item 3. That
spec has since been closed and removed from disk (commit `7f60301d1`, "spec:
close followup-bgp-feature (all items done or re-deferred)"), so this file is the
only remaining home for the work. Re-deferred by user decision on 2026-07-08 and
re-verified on 2026-07-16.

**Why this is not reachable today.** ze is a single-AS speaker, so the Section 5
confederation rules are vacuously satisfied:

| Fact | Evidence |
|------|----------|
| Exactly one global local AS leaf exists, and it is mandatory | `internal/component/bgp/yang/ze-bgp-conf.yang` (`bgp/session/asn/local`) |
| The only other local AS is a per-peer override, not a member-AS | `internal/component/bgp/yang/ze-bgp-conf.yang` (`session/asn/local`, "Local AS (overrides global)") |
| No confederation identifier or member-AS leaf exists in any YANG | grep for `confed` across `internal/**/*.yang` matches only two filter descriptions |
| ze never originates confederation AS_PATH segments | `internal/core/bgp/attribute/as4.go` (`FilterConfedSegments` strips them), `internal/component/bgp/reactor/reactor_wire.go`, `internal/component/bgp/wireu/aspath_as4.go`; nothing constructs `ASConfedSequence` / `ASConfedSet` |

Real support therefore needs confederation-member configuration plus AS_CONFED
origination first. That is a large feature and is the true scope of this spec.
RFC 9234 also records that Role negotiation and OTC procedures are NOT RECOMMENDED
between autonomous systems in an AS Confederation, so the design must first settle
whether ze supports the combination at all, or rejects it at config validation.

~~**Prerequisite bug found while verifying (2026-07-16).** The OTC egress stamp is
inert today, independent of confederations: `extractLocalASN` reads the key
`local-as` from the BGP config subtree (`internal/component/bgp/plugins/role/config.go`),
but the config tree carries the global local AS at `bgp/session/asn/local`, as the
reactor's own reader shows (`internal/component/bgp/reactor/config.go`).
`getLocalASN` (`internal/component/bgp/plugins/role/role.go`) therefore returns
0 in production, and the stamp is skipped by the `localASN > 0` guard
(`internal/component/bgp/plugins/role/otc.go`). Fix that separately, before
any confederation work: this spec's whole subject is which ASN gets stamped.~~

(Superseded 2026-07-22 plan review: the prerequisite bug is FIXED --
`spec-fixit-local-asn-config-key` deleted the `extractLocalASN`/`local-as`
reader; the OTC stamp now reads `dest.LocalAS` (`otc.go`) and the
`localASN > 0` guard is live. The related A-3 assumption row and the
"Phase: Prerequisite" implementation step below are obsolete with it. The
spec's main premise -- ze is single-AS, so RFC 9234 Section 5 confederation
handling is a large open design question -- is unaffected.)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - BGP plugin registration and filter pipeline
  → Constraint: plugins register via `register.go`; the core never imports them directly

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9234.md` - BGP Roles and the OTC attribute
  → Constraint: [RFC9234-5-7] OTC added on egress from an AS Confederation MUST equal the AS Confederation Identifier (Section 5)
  → Constraint: [RFC9234-5-8] On egress from an AS Confederation, an UPDATE MUST NOT contain OTC with a Member-AS number other than the Confederation Identifier (Section 5)
  → Decision: RFC 9234 marks Role/OTC NOT RECOMMENDED between ASes in an AS Confederation (Section 5), so "reject at config" is a legitimate design answer
- [ ] `rfc/short/rfc4271.md` - base BGP, AS_PATH semantics
  → Constraint: AS_PATH segment types and loop detection define what confederation segments must not escape

**Key insights:**
- `role/otc.go`: egress stamping uses one flat local ASN, with no notion of a confederation boundary
- `role/otc.go`: `checkOTCEgress` suppresses on the destination's role only; a confederation boundary is not a role
- `rfc/short/rfc9234.md`: R012/R013 are the two unchecked confederation requirements

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/role/otc.go` - OTC ingress rules, egress suppression, and egress stamping via a single local ASN
- [ ] `internal/component/bgp/plugins/role/role.go` - plugin entry point; holds `filterLocalASN` and `getLocalASN` (line 66)
- [ ] `internal/component/bgp/plugins/role/config.go` - `extractLocalASN` (line 230) reads the `local-as` key from the BGP subtree
- [ ] `internal/component/bgp/reactor/config.go` - reads the global local AS from `bgp > session > asn > local` (lines 479-486)
- [ ] `internal/core/bgp/attribute/as4.go` - parses confederation AS_PATH segments and strips them on write; never originates them
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - config surface: one global local AS, one per-peer override, no confederation leaves

**Behavior to preserve:**
- Single-AS operation stays the default: a router with no confederation config behaves exactly as today
- RFC 9234 ingress rules stay non-overridable by the operator (`checkOTCIngress`, `otc.go`)
- OTC remains scoped to AFI 1/2 SAFI 1 (`isPayloadUnicast`, `otc.go`)
- "Once the OTC Attribute has been set, it MUST be preserved unchanged" (`otcAttrModHandler`, `otc.go`)
- Confederation AS_PATH segments continue to be stripped on egress, never leaked
- Malformed OTC continues to be treat-as-withdraw (`otc.go`)

**Behavior to change:**
- Add confederation identifier and member-AS configuration to the BGP config surface
- Originate AS_CONFED_SEQUENCE / AS_CONFED_SET within the confederation
- Stamp OTC on confederation egress with the Confederation Identifier, never a Member-AS
- Or, if the design rejects the combination: fail config validation with a clear message

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- BGP config tree delivered to the role plugin as a JSON section rooted at `bgp` (`RunRolePlugin` `OnConfigure`, `role.go`)
- Received UPDATE path attributes on the ingress filter (`OTCIngressFilter`, `role.go` registration, `otc.go`)
- Per-destination-peer forwarding on the egress filter (`OTCEgressFilter`, `otc.go`)

### Transformation Path
1. Config resolution builds the BGP tree; the plugin server extracts the `bgp` subtree and marshals it to JSON (`internal/component/plugin/server/reload.go`)
2. The role plugin parses that JSON and stores per-peer role config plus the local ASN in package state (`setFilterState`, `role.go`)
3. On ingress, `checkOTCIngress` applies the Section 5 rules and returns an accept / reject / treat-as-withdraw verdict plus an ASN to stamp (`otc.go`)
4. On egress, `OTCEgressFilter` suppresses to Provider/Peer/RS, then stamps OTC for Customer/Peer/RS-Client destinations using `getLocalASN` (`otc.go`)
5. The attribute mod handler writes the OTC bytes during the progressive attribute build, preserving any existing OTC (`otcAttrModHandler`, `otc.go`)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree → role plugin | JSON section rooted at `bgp` over the plugin RPC | [ ] |
| Role plugin → filter pipeline | Registered ingress/egress filter closures reading package state | [ ] |
| Filter → wire encoding | `ModAccumulator` ops resolved by `otcAttrModHandler` into attribute bytes | [ ] |
| Confederation boundary → eBGP egress | (does not exist today; this spec must create it) | [ ] |

### Integration Points
- `internal/component/bgp/plugins/role/` - OTC processing and role config
- `internal/core/bgp/attribute/as4.go` - confederation AS_PATH segment handling
- `internal/component/bgp/yang/ze-bgp-conf.yang` - config surface for confederation identity
- `internal/component/bgp/reactor/config.go` - the canonical reader of the global local AS

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding: confederation config and any new filter register through the existing plugin and filter registries; no per-feature switch added to a core package

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | ze has no confederation config surface today | grep for `confed` over `internal/**/*.yang` matches only filter descriptions | Scope shrinks to OTC value selection only | Re-grep YANG at pickup | unvalidated |
| A-2 | ze never originates AS_CONFED segments | `as4.go`, `reactor_wire.go`, `wireu/aspath_as4.go` all strip, none construct | Origination exists and only OTC selection is missing | Re-read the AS_PATH writers | unvalidated |
| A-3 | OTC egress stamping works before confederation work starts | Currently broken: `role/config.go` reads a `local-as` key the tree does not carry | This spec builds on an inert code path and cannot be tested | Fix the key, then assert a stamp fires end-to-end | broken |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | RFC 9234 marks Role/OTC NOT RECOMMENDED inside a confederation, so the feature may be unwanted | Design review questions the use case | Implement config-time rejection with a clear message instead of silent wrong behavior |
| R-2 | Confederation support is a large feature well beyond OTC | Scope creep into AS_PATH origination, best-path, and loop detection | Split: confederation core first, OTC value selection as a dependent spec |

## Wiring Test (MANDATORY)
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Confederation identity configured in the BGP tree | -> | Role plugin resolves confederation identifier | (fill during design) |
| UPDATE forwarded across the confederation boundary | -> | OTC stamped with the Confederation Identifier | (fill during design) |
| UPDATE forwarded to a member-AS inside the confederation | -> | No confederation-boundary OTC stamp applied | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Confederation identifier configured, route leaves the confederation to a Customer/Peer/RS-Client | OTC is stamped with the Confederation Identifier (RFC9234-5-7) |
| AC-2 | Route leaves the confederation carrying an OTC set to a Member-AS number | OTC value is corrected to, or rejected against, the Confederation Identifier (RFC9234-5-8) |
| AC-3 | Route moves between member-ASes inside the confederation | No confederation-boundary OTC processing is applied |
| AC-4 | No confederation configured | Behavior is byte-identical to today's single-AS path |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOTCEgressStampsConfederationIdentifier` | `internal/component/bgp/plugins/role/otc_test.go` | RFC9234-5-7: confederation egress stamps the identifier, not a member-AS | |
| `TestOTCEgressRejectsMemberASValue` | `internal/component/bgp/plugins/role/otc_test.go` | RFC9234-5-8: a member-AS OTC value never escapes the confederation | |
| `TestOTCNoConfederationUnchanged` | `internal/component/bgp/plugins/role/otc_test.go` | AC-4: the single-AS path is unaffected | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-otc-confederation-egress` | `test/bgp/*.ci` | Operator runs a confederation member and sees the Confederation Identifier in OTC on routes leaving the confederation | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-otc-confederation` | `test/interop/scenarios/` | FRR or BIRD | A third-party confederation member accepts ze's OTC value | |

## Files to Modify
- `internal/component/bgp/plugins/role/otc.go` - confederation-aware OTC value selection
- `internal/component/bgp/plugins/role/config.go` - confederation identity parsing (and the `local-as` key fix, if not already fixed)
- `internal/component/bgp/yang/ze-bgp-conf.yang` - confederation identifier and member-AS config surface
- `rfc/short/rfc9234.md` - tick R012/R013 once proven
- `docs/features/rfc-status.md` - status ledger row with source anchors

## Implementation Steps

1. **Phase: Prerequisite.** Fix the inert OTC egress stamp (`local-as` key mismatch) so the stamp path is testable at all
2. **Phase: Decision.** Settle whether ze supports Role/OTC inside a confederation or rejects it at config validation (RFC 9234 marks it NOT RECOMMENDED)
3. **Phase: Wiring (MANDATORY FIRST for the chosen path).** Add the config surface and a failing wiring test
4. **Phase: Confederation core.** Member-AS config plus AS_CONFED origination, if the decision is to support it
5. **Phase: OTC value selection.** Stamp the Confederation Identifier on confederation egress
6. **Functional and interop tests** → prove the boundary behavior against a third-party daemon
7. **Full verification** → `make ze-verify`

## RFC Documentation

Add `// RFC 9234 Section 5: "<quoted requirement>"` above the enforcing code for
R012 and R013.

## Known Limitations
- Large scope: real support needs confederation-member config and AS_CONFED origination before OTC value selection is meaningful
- RFC 9234 marks Role/OTC NOT RECOMMENDED inside a confederation, so the feature may resolve to a config-time rejection
- Blocked in practice by the inert OTC egress stamp recorded in the Task section, which must be fixed independently first

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete: every row has a concrete test name
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Registration over hardcoding verified

### Quality Gates (SHOULD pass, defer with user approval)
- [ ] RFC constraint comments added for R012 and R013
- [ ] `rfc/short/rfc9234.md` and `docs/features/rfc-status.md` updated

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
