# Spec: bgp-deferred-confederation-otc

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-09-05 |

## OWNER RULING 2026-08-05: implement confederation support

**Thomas answered the question this spec was waiting on. Ze IMPLEMENTS AS
Confederation support. The `{not-applicable}` classification is not the answer,
and the narrower config-validation option was not taken.**

He chose the full feature over two cheaper answers put beside it: keeping
`{not-applicable}` with corrected evidence, and refusing the ambiguous
role-plus-local-as-override combination at config validation.

The ruling authorises the full feature. This spec remains in `design` until
the confederation-core and OTC contracts below have an implementation design.

**What the ruling commits ze to**, from the option as put:

| Piece | Where |
|-------|-------|
| `bgp/confederation/identifier` and `bgp/confederation/member-as` config | `internal/component/bgp/yang/ze-bgp-conf.yang`, plus the config loader |
| `ASConfedSequence` and `ASConfedSet` ORIGINATION | Nothing constructs them today. `FilterConfedSegments` (`internal/core/bgp/attribute/as4.go`) only strips them on receive |
| The confederation egress boundary: strip member-AS segments, stamp OTC with the confederation identifier | `internal/component/bgp/reactor/`, and `OTCEgressFilter` (`internal/component/bgp/plugins/role/otc.go`), which today stamps `dest.LocalAS` unconditionally |
| RFC 9234 Section 5 conformance on top, both MUSTs proven by tagged tests | `rfc/short/rfc9234.md` rows RFC9234-5-7 and RFC9234-5-8 |
| An interop scenario, because this is wire-visible protocol behavior | `test/interop/scenarios/` (`ai/rules/interop-and-goal-validation.md`) |

**Do not treat the two `{not-applicable}` rows as settled.** They stay only while
the feature is absent. Once confederation config exists the condition IS
reachable, both MUSTs bind, and each needs an `RFC requirement:` tagged test.
Re-classifying them is part of this spec's closure, not a separate decision.

**RFC 9234 Section 5 also says Role negotiation and OTC procedures are NOT
RECOMMENDED between autonomous systems in an AS Confederation** (row
RFC9234-5-13). The design must say what ze does when an operator configures both,
because the ruling makes that combination reachable for the first time.

## What was re-derived, 2026-08-05

The two Section 5 rows were recorded `{not-applicable}`. Under the owner directive
of 2026-07-27 every classification pointing away from full compliance is VOID and
must be re-derived from the RFC text rather than cited, so it was.

The core of the old reasoning holds: no ze config can express confederation
membership, so "egress from the AS Confederation" never occurs. Two defects in the
record were found, and they are why this reached Thomas rather than being
confirmed quietly:

- **The evidence citation is stale.** Both reasons cite the egress stamp at a line
  number in `otc.go` that no longer holds it. The stamp is in `OTCEgressFilter`.
- **The premise is incomplete.** Both reasons argue that no per-peer AS exists
  which could act as a member-AS. `PeerSettings` carries `LocalAS` AND
  `GlobalLocalAS`, and `buildDynamicGroupSettings`
  (`internal/component/bgp/config/peers.go`) sets the per-peer one from a
  `local-as` override. `OTCEgressFilter` stamps `dest.LocalAS`, the per-peer
  effective value. So an operator using the override already makes ze advertise
  different OTC values on different sessions. That is structurally what Section 5
  exists to prevent, even though ze is not formally in a confederation.

**The prerequisite bug this spec recorded is FIXED, verified at source
2026-08-05.** The old text said `extractLocalASN` read a `local-as` key that the
config tree does not carry, so `getLocalASN` returned 0 and the `localASN > 0`
guard skipped the stamp in production. `extractLocalASN` no longer exists;
`OTCEgressFilter` reads `dest.LocalAS`, which the reactor fills from
`peer.settings.LocalAS`, the same field it builds AS_PATH from. The stamp is live,
and it fails closed with a log line when the value is 0.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/plugins/role/otc.go` - OTC ingress/egress processing
4. `internal/component/bgp/yang/ze-bgp-conf.yang` - BGP config surface (local AS)

## Task

Implement AS Confederation support under the 2026-08-05 owner ruling: the
identifier and member-AS configuration, session identity, AS_CONFED origination
and forwarding, loop prevention and external-boundary handling. This spec owns
the confederation core as well as RFC 9234 Section 5's OTC boundary rules.
On egress from the confederation, a newly added OTC must use the Confederation
Identifier and no OTC value naming another Member-AS may escape.

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
whether Role/OTC is allowed on internal member-to-member sessions. That choice
cannot replace confederation support with configuration refusal.

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
  → Decision: the internal member-to-member Role/OTC combination needs a design decision under RFC9234-5-13. Any refusal is confined to that combination; the owner already commissioned confederation core and external OTC handling.
- [ ] `rfc/short/rfc4271.md` - base BGP, AS_PATH semantics
  → Constraint: AS_PATH segment types and loop detection define what confederation segments must not escape
- [ ] `rfc/full/rfc5065.txt` and `rfc/short/rfc5065.md` - confederation core
  → Constraint: Sections 4 and 4.1 distinguish same-member, other-member and external peers for OPEN identity and AS_PATH origination/propagation. Section 5 governs malformed-boundary input, Section 5.2 covers MED/LOCAL_PREF and Section 5.3 covers path selection. The full-feature ruling requires this design and its evidence; this planning correction makes no compliance claim.

**Key insights:**
- `role/otc.go`: egress stamping uses one flat local ASN, with no notion of a confederation boundary
- `role/otc.go`: `checkOTCEgress` suppresses on the destination's role only; a confederation boundary is not a role
- `rfc/short/rfc9234.md`: R012/R013 are the two unchecked confederation requirements

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/role/otc.go` - OTC ingress rules, egress suppression, and egress stamping via a single local ASN
- [ ] `internal/component/bgp/plugins/role/role.go` - `setFilterState` stores peer-role configuration and name resolution; it no longer stores a separately parsed local ASN
- [ ] `internal/component/bgp/plugins/role/config.go` - role configuration; the retired `extractLocalASN` key reader is historical
- [ ] `internal/component/bgp/reactor/config.go` - reads the global local AS from `bgp > session > asn > local` (lines 479-486)
- [ ] `internal/core/bgp/attribute/as4.go` - parses confederation AS_PATH segments and strips them on write; never originates them
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - config surface: one global local AS, one per-peer override, no confederation leaves

**Behavior to preserve:**
- Single-AS operation stays the default: a router with no confederation config behaves exactly as today
- RFC 9234 ingress rules stay non-overridable by the operator (`checkOTCIngress`, `otc.go`)
- OTC remains scoped to AFI 1/2 SAFI 1 (`isPayloadUnicast`, `otc.go`)
- "Once the OTC Attribute has been set, it MUST be preserved unchanged" (`otcAttrModHandler`, `otc.go`)
- Confederation segments must not escape the external boundary. Internal member-to-member forwarding must retain and extend them under RFC 5065.
- Malformed OTC continues to be treat-as-withdraw (`otc.go`)

**Behavior to change:**
- Add confederation identifier and member-AS configuration to the BGP config surface
- Originate AS_CONFED_SEQUENCE / AS_CONFED_SET within the confederation
- Stamp OTC on confederation egress with the Confederation Identifier, never a Member-AS
- Specify the internal Role/OTC configuration outcome without making confederation origination or boundary processing conditional on that choice.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- BGP config tree delivered to the role plugin as a JSON section rooted at `bgp` (`RunRolePlugin` `OnConfigure`, `role.go`)
- Received UPDATE path attributes on the ingress filter (`OTCIngressFilter`, `role.go` registration, `otc.go`)
- Per-destination-peer forwarding on the egress filter (`OTCEgressFilter`, `otc.go`)

### Transformation Path
1. Config resolution builds the BGP tree; the plugin server extracts the `bgp` subtree and marshals it to JSON (`internal/component/plugin/server/reload.go`)
2. The role plugin stores per-peer role configuration through `setFilterState`; the reactor supplies the effective local ASN in `PeerFilterInfo`.
3. On ingress, `checkOTCIngress` applies the Section 5 rules and returns an accept / reject / treat-as-withdraw verdict plus an ASN to stamp (`otc.go`)
4. On egress, `OTCEgressFilter` applies the role rules and stamps absent OTC for eligible Customer/Peer/RS-Client advertisements using `dest.LocalAS`. The new confederation boundary must supply the identifier and prevent member-AS leakage.
5. The attribute mod handler writes the OTC bytes during the progressive attribute build, preserving any existing OTC (`otcAttrModHandler`, `otc.go`)
6. Confederation core resolves each destination as same-member, other-member or external. It chooses the OPEN identity and the AS_PATH transformation from that relationship before the wire is emitted. OTC processing consumes the same boundary fact.

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
| A-1 | Confederation configuration remains absent at implementation pickup | The recorded source investigation found no identifier or member-AS configuration | Reuse any subsequently added surface, but retain all core acceptance obligations | Re-read YANG and its producing config loader at pickup | unvalidated |
| A-2 | AS_CONFED origination remains absent at implementation pickup | The recorded writers strip confederation segments | Reuse any subsequently added producer and prove the complete core contract | Re-read the AS_PATH writers | unvalidated |
| A-3 | The old inert-stamp defect is no longer a prerequisite | `OTCEgressFilter` stamps from `dest.LocalAS`, and the old `extractLocalASN` reader is gone | A current stamping failure would need a new diagnosis | Source inspection of the egress stamp; end-to-end stamp proof remains required | source correction confirmed, 2026-09-19; no new test result |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Internal Role/OTC configuration is mistaken for permission to refuse the full feature | Design makes AS_CONFED origination conditional | Confine the remaining choice to internal member sessions; core and external-boundary ACs are mandatory |
| R-2 | OTC tests pass while the commissioned confederation core is absent | Tests inject a boundary fact without establishing member and external sessions | Keep core ownership here and require AC-5 through AC-10 from public configuration to peer wire |

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
| AC-5 | Operator configures the confederation identifier and Member-AS, with peers in the same member, another member and outside the confederation | Configuration reaches session classification; OPEN uses the Member-AS toward confederation members and the Confederation Identifier toward external peers (RFC 5065 Section 4) |
| AC-6 | Ze originates a route to each peer class | Same-member iBGP receives an empty AS_PATH; another member receives an AS_CONFED_SEQUENCE containing the local Member-AS; an external peer receives an AS_SEQUENCE containing the Confederation Identifier (Section 4.1) |
| AC-7 | Ze forwards a route inside the confederation | Same-member forwarding preserves AS_PATH; other-member forwarding prepends the Member-AS to AS_CONFED_SEQUENCE, with segment-boundary handling, while preserving existing confederation structure (Section 4.1) |
| AC-8 | A route containing AS_CONFED_SEQUENCE and AS_CONFED_SET crosses the external boundary | Both confederation segment types are removed and the Confederation Identifier is prepended to the remaining public AS_PATH; no member-AS segment reaches the peer (Sections 4.1 and 5) |
| AC-9 | A received path contains the local Confederation Identifier, or a confederation segment contains the local Member-AS | Normal own-AS loop handling rejects the loop; non-loop controls remain usable (Section 4) |
| AC-10 | Confederation segment origination, malformed input and route selection are exercised | AS_CONFED_SET origination is covered as commissioned. Derive and prove RFC 5065 Section 5's malformed-boundary handling, Section 5.2's member-to-member NEXT_HOP/MED/LOCAL_PREF rules and Section 5.3's neighbour-AS, path-length and internal-path classification; core acceptance cannot be replaced by OTC-only tests |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOTCEgressStampsConfederationIdentifier` | `internal/component/bgp/plugins/role/otc_test.go` | RFC9234-5-7: confederation egress stamps the identifier, not a member-AS | |
| `TestOTCEgressRejectsMemberASValue` | `internal/component/bgp/plugins/role/otc_test.go` | RFC9234-5-8: a member-AS OTC value never escapes the confederation | |
| `TestOTCNoConfederationUnchanged` | `internal/component/bgp/plugins/role/otc_test.go` | AC-4: the single-AS path is unaffected | |
| `TestConfederationSessionIdentity` | reactor/config test carriers selected during design | AC-5: config-derived member versus external OPEN identity, with no-confederation control | planned |
| `TestConfederationOriginationAndForwarding` | reactor AS_PATH test carriers selected during design | AC-6 through AC-8: same-member, other-member and external wire forms; both confederation segment types and segment boundaries | planned |
| `TestConfederationLoopAndSelectionRules` | reactor/RIB test carriers selected during design | AC-9 and AC-10: loop rejection, aggregation and Section 5 error/selection rules | planned |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-otc-confederation-egress` | `test/bgp/*.ci` | Operator runs a confederation member and sees the Confederation Identifier in OTC on routes leaving the confederation | |
| `confederation-core` | Functional plugin scenario selected during design | AC-5 through AC-10 from configured member/external sessions through origination, relay, loop rejection, withdrawal and external wire inspection | planned |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-otc-confederation` | `test/interop/scenarios/` | FRR or BIRD with the required OTC support | AC-1 through AC-4: a member-to-external topology observes the Confederation Identifier in OTC, excludes member-AS leakage and preserves the single-AS control | planned |
| `NN-confederation-core` | `test/interop/scenarios/` | FRR or BIRD confederation peers | AC-5 through AC-10: establish internal and external identities, originate and relay routes through members, reject loops, and inspect external AS_PATH with no AS_CONFED segments | planned |

## Files to Modify
- `internal/component/bgp/plugins/role/otc.go` - confederation-aware OTC value selection
- `internal/component/bgp/plugins/role/config.go` - confederation identity and boundary configuration; the old local-AS-key fix is already present
- `internal/component/bgp/yang/ze-bgp-conf.yang` - confederation identifier and member-AS config surface
- `internal/component/bgp/reactor/` - config-derived session identity, member/external classification, announce and forward AS_PATH handling
- `internal/core/bgp/attribute/` and the BGP RIB selection path - confederation segment, aggregation, loop and selection behaviour required by AC-6 through AC-10
- `rfc/short/rfc5065.md` - derive the core requirements and evidence before claiming support
- `rfc/short/rfc9234.md` - tick R012/R013 once proven
- `docs/features/rfc-status.md` - status ledger row with source anchors

## Implementation Steps

1. **Phase: Design the full contract.** Derive RFC 5065 Sections 4 and 5 against AC-5 through AC-10, including AS_CONFED_SET origination. Decide the internal Role/OTC configuration outcome separately. The old inert-stamp prerequisite is resolved.
2. **Phase: Wiring (MANDATORY FIRST).** Add the identifier/member-AS surface and destination classification, then prove configuration reaches the OPEN and AS_PATH producers.
3. **Phase: Confederation core.** Implement session identity, origination, member forwarding, external stripping, loop prevention, aggregation and selection/error handling. This phase is mandatory under the owner ruling.
4. **Phase: OTC boundary.** Consume the same confederation boundary for identifier stamping and member-AS exclusion, with both-polarity tests for RFC9234-5-7 and RFC9234-5-8.
5. **Phase: Functional and interop proof.** Exercise the complete core and OTC topology; injected boundary facts alone cannot prove the feature.
6. **Phase: Documentation and full verification.** Update configuration and architecture documentation and the RFC evidence, then run `./le verify worktree`.

## RFC Documentation

Add `// RFC 9234 Section 5: "<quoted requirement>"` above the enforcing code for
R012 and R013.

## Known Limitations
- Full confederation support remains in design, including the core acceptance contract and its concrete test carriers.
- RFC9234-5-13 leaves the internal Role/OTC combination to the design decision. It does not leave confederation support or external OTC handling optional.
- The former inert OTC stamp is fixed; it is retained only in the dated provenance above.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated, including the confederation core from public configuration through peer wire
- [ ] Wiring Test table complete: every row has a concrete test name
- [ ] `./le verify worktree` passes (lint + all ze tests)
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

## Work Inherited From a Deferral Row

<!-- The deferral directory was deleted on 2026-09-05. A row that named this spec as
     its destination is reproduced here, so the item and the reasoning behind it
     survive the directory. Each row is outstanding work this spec owns. -->

### From `followup-bgp-feature.md`, 2026-07-10

Deferred by spec-followup-bgp-feature item 3.

AS-Confederation OTC (RFC 9234 §5 confederation rules: OTC value = confed identifier, member-AS semantics)
