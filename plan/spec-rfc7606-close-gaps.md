# Spec: rfc7606-close-gaps

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc7606.md` (the 8 `{gap}` annotations), `ai/rules/rfc-compliance.md`
4. `internal/component/bgp/message/rfc7606.go`, `internal/component/bgp/wireu/split.go`,
   `internal/component/bgp/reactor/session_validation.go`

## Task

Close seven of the eight RFC 7606 `{gap}` annotations in `rfc/short/rfc7606.md`, so ze
actually implements the obligation rather than disclosing that it does not.

**Explicitly EXCLUDED by the user: RFC7606-5.1-1** (MP_REACH/MP_UNREACH as the very first
path attribute). That divergence is deliberate, documented in
`docs/architecture/wire/mp-nlri-ordering.md`, and stays.

In scope:

| ID | Section | Obligation |
|----|---------|-----------|
| RFC7606-5.1-2 | §5.1 | An UPDATE MUST NOT contain more than one of: non-empty Withdrawn Routes, non-empty NLRI, MP_REACH_NLRI, MP_UNREACH_NLRI |
| RFC7606-5.4-1 | §5.4 | Routes with unrecognized NLRI types in a typed family MUST be discarded |
| RFC7606-6-1 | §6 | Debug logging must list the NLRI involved and contain the entire malformed UPDATE |
| RFC7606-7.13-1 | §7.13 | Malformed Traffic Engineering attribute (code 24) ⇒ treat-as-withdraw |
| RFC7606-7.15-1 | §7.15 | IPv6 Address Specific Extended Community (code 25) malformed if length is not a non-zero multiple of 20 ⇒ treat-as-withdraw |
| RFC7606-7.15-2 | §7.15 | An unrecognized IPv6 Ext-Community Type/Sub-Type MUST NOT be treated as an error |
| RFC7606-7.16-1 | §7.16 | Malformed ATTR_SET (code 128) ⇒ treat-as-withdraw |

Each closed gap must end with: implementing code, a positive AND a negative tagged test,
the `{gap}` annotation removed from the summary, and the
`docs/features/rfc-status.md` RFC 7606 row updated. `make ze-rfc-check` enforces all of
that: removing a `{gap}` without tests fails the gate.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/rfc-compliance.md` - the four ratchets and the tagging contract
  → Constraint: a met MUST needs BOTH polarities tagged, or a reasoned annotation.
  → Constraint: coverage is monotonic; these requirements currently have NO tags, so
    adding them can only improve, but the `{gap}` removal is checked against the tests.
- [ ] `ai/rules/buffer-first.md` - wire encoding must not allocate per-message
  → Constraint: RFC7606-5.1-2 touches the UPDATE encoder; any fix must keep the
    `WriteTo(buf, off) int` shape and must not add per-UPDATE allocation.
- [ ] `docs/architecture/wire/mp-nlri-ordering.md` - the excluded §5.1-1 decision
  → Constraint: MP_UNREACH-first / MP_REACH-last ordering must NOT change.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7606.md` - the eight gap annotations under audit
  → Constraint: §5.1's restrictions bind the SENDER. The same section says an
    implementation "MUST still be prepared to receive these fields in any position or
    combination", so receive-side behavior must not change.
- [ ] `rfc/full/rfc7606.txt` §5.1, §5.4, §6, §7.13, §7.15, §7.16 - read in full
  → Constraint: §7.13 says RFC 5543 "does not detail what constitutes malformation" and
    binds only "an implementation that determines (for whatever reason) that ... a
    malformed Traffic Engineering path attribute" — so the check may be minimal, and
    over-validating would blackhole valid routes.
  → Constraint: §6's "must" is LOWERCASE in the RFC. Under RFC 2119 the keywords are
    normative "when, and only when, they appear in all capitals", so §6 is arguably not a
    normative MUST at all. The summary records it as MUST; implementing it is still the
    right call (debuggability), but the classification is noted here rather than hidden.
- [ ] `rfc/full/rfc5543.txt` (fetched 2026-07-20) - Traffic Engineering attribute format
  → Constraint: each descriptor is Switching Cap(1) + Encoding(1) + Reserved(2) + eight
    4-octet Max LSP Bandwidth values = 36 fixed octets, then variable
    switching-capability-specific information whose length is defined by RFC 4203/5307
    per capability. "The attribute contains one or more of the following."
- [ ] `rfc/full/rfc6368.txt` (fetched 2026-07-20) §5 - ATTR_SET format and malformedness
  → Constraint: value is Origin AS (4 octets) then a path-attribute stream. Malformed if
    length < 4, OR the contained attributes include MP_REACH/MP_UNREACH, OR the included
    attributes are themselves malformed. RFC 7606 §7.16 replaces only the ACTION (now
    always treat-as-withdraw, dropping the old Partial/Neighbor-Complete branch).

**Key insights:**
- Four of the seven are a single well-trodden pattern: register an entry in
  `attrValidators` and return `RFC7606ActionTreatAsWithdraw`.
- §5.1-2 is the only one that changes what ze puts on the wire.
- §7.15-2 is currently satisfied *by omission* (nothing validates code 25, so nothing can
  error on an unknown type). Adding §7.15-1's length check must not break it: the check
  must look at LENGTH ONLY and never at Type/Sub-Type.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/message/rfc7606.go` - the RFC 7606 engine.
  → Constraint: `attrValidators` is a `[256]attrValidatorFn` array (`:412`) populated in
    `init()` (`:414-430`) with exactly 15 codes: 1,2,3,4,5,6,7,8,9,10,14,15,16,32,40.
    Codes 24, 25 and 128 are absent.
  → Constraint: `validateAttribute` (`:433-437`) returns `nil` when the entry is nil, so
    an unregistered code accepts ANY length.
  → Constraint: validator signature is
    `func(code uint8, length int, attrData []byte, isIBGP, asn4 bool) *RFC7606ValidationResult`.
    `validateExtCommunityAttr` (`:607-617`) is the closest template: `length == 0 ||
    length%8 != 0` ⇒ treat-as-withdraw, description built with `textbuf.Buffer`.
  → Constraint: attribute type-code constants live at `:57-73`; 24, 25 and 128 need adding.
  → Constraint: the main attribute walk is inline in `ValidateUpdateRFC7606` and already
    handles flags, duplicates and bounds; a validator sees only its own attribute's data.
- [ ] `internal/component/bgp/wireu/split.go` - `buildUpdatePayload` (`:440-452`) takes
  `ipv4Withdraws, baseAttrs, mpUnreach, mpReach, ipv4NLRI` and composes them into ONE
  body; `hasAnnounces` (`:442`) is `len(mpReach) > 0 || len(ipv4NLRI) > 0`.
  → Constraint: this is the producer behind RFC7606-5.1-2.
  → (fill from research: which callers can pass more than one non-empty component)
- [ ] `internal/component/bgp/reactor/session_validation.go` - the three RFC 7606 log
  sites (`:110-113`, `:148-150`, `:178`).
  → (fill from research: exactly what is logged and what is in scope)
- [ ] `internal/component/bgp/plugins/nlri/evpn/types.go` - `ParseEVPN` maps unknown route
  types to `EVPNGeneric` (`:237-243`).
  → (fill from research: storage and re-advertisement path)
- [ ] `internal/component/bgp/plugins/nlri/mvpn/types.go` - `ParseMVPN` (`:118-130`) does
  not branch on route type.
  → (fill from research)

**Behavior to preserve:**
- MP_UNREACH-first / MP_REACH-last ordering (§5.1-1, excluded).
- Receive-side tolerance: any position or combination of the four NLRI-bearing fields must
  still be accepted (§5.1 explicitly requires this).
- Unrecognized IPv6 Ext-Community Type/Sub-Type must not become an error (§7.15-2).
- The 44 `enforced` verdicts in `rfc/audit/rfc7606.json` and every existing tagged test.
- End-of-RIB (an UPDATE with no NLRI at all) must remain valid and unaffected.

**Behavior to change:**
- Codes 24, 25, 128 gain RFC 7606 validators.
- RFC 7606 error logs gain the NLRI list and the malformed UPDATE bytes.
- Unrecognized typed NLRI is discarded rather than retained.
- Locally re-encoded UPDATEs carry at most one NLRI-bearing field.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Codes 24/25/128 and §6: a received UPDATE on an established session →
  `ValidateUpdateRFC7606`.
- §5.4: a received typed-family NLRI → family parser → RIB install.
- §5.1-2: an UPDATE ze encodes → `buildUpdatePayload` → wire.

### Transformation Path
1. wire bytes → `ValidateUpdateRFC7606` → per-attribute `validateAttribute(code, ...)` →
   `RFC7606ValidationResult{Action: TreatAsWithdraw}` → synthesized withdrawal.
2. ATTR_SET additionally walks its own inner attribute stream and recurses into
   `validateAttribute` for each inner code, under a bounded depth.
3. §6: the same result → `session_validation.go` log site, now including NLRI + raw UPDATE.
4. §5.4: (fill from research)
5. §5.1-2: (fill from research)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire ↔ validation | attribute data slice, no copy | [ ] |
| Validation ↔ log | result + wire update already in scope | [ ] |
| Encoder ↔ wire | `buildUpdatePayload` byte composition | [ ] |

### Integration Points
- `attrValidators` init block — three new registrations.
- `session_validation.go` log sites — three call sites.

### Architectural Verification
- [ ] No bypassed layers (validators are reached through `validateAttribute`, not called directly)
- [ ] No unintended coupling (the message package does not learn about the reactor)
- [ ] No duplicated functionality (reuse `validateAttribute` recursively for ATTR_SET)
- [ ] Zero-copy preserved (validators take a slice; ATTR_SET's inner walk must not copy)
- [ ] Registration over hardcoding (`attrValidators` IS the registry; no switch added)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No code path today emits attribute 24, 25 or 128, so adding validators cannot reject ze's own output | `attrValidators` init (`rfc7606.go:414-430`) registers none of them | ze could withdraw its own routes | grep for the codes across encoders; functional tests | unvalidated |
| A-2 | A minimal TE check (length 0, or < 36) cannot false-positive on a valid RFC 5543 attribute | `rfc/full/rfc5543.txt` §3: 36 fixed octets per descriptor, "one or more" | valid TE routes blackholed | boundary test at 35/36 | unvalidated |
| A-3 | ATTR_SET nesting is bounded in practice, but a crafted UPDATE could nest deeply | RFC 6368 §5 permits "any BGP attribute" | stack exhaustion from a peer | explicit depth cap + test | unvalidated |
| A-4 | §5.1-2 affects only paths that RE-ENCODE, not paths that forward received bytes | RFC 7606 §5.1 binds the sender | either a missed obligation or a behavior change on forwarding | research + read every `buildUpdatePayload` caller | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | §7.15-1's length check breaks §7.15-2 (unknown type must not error) | a test asserting an unknown type is accepted turns red | the validator reads LENGTH ONLY; the negative test for 7.15-2 uses an unknown Type with a VALID length |
| R-2 | §5.4's discard drops routes an operator relies on | EVPN/MVPN functional tests turn red | scope the discard to genuinely unrecognized types; check for a "specification says otherwise" carve-out first |
| R-3 | §5.1-2 splitting changes message counts and breaks wire tests | `.ci` hex expectations diverge | identify every test that counts UPDATEs before changing the encoder |
| R-4 | §6 logging the entire UPDATE floods logs or leaks data | log volume on a malformed-attribute storm | bound the dump; follow the repo's existing logging convention |
| R-5 | ATTR_SET recursion into MP_REACH/MP_UNREACH validators mis-fires, since those expect to be top-level | inner validation rejects a valid ATTR_SET | RFC 6368 forbids MP_REACH/MP_UNREACH inside ATTR_SET, so their PRESENCE is itself the malformed condition — never validate them as inner attributes |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| received UPDATE with malformed TE attr | → | `validateTrafficEngineeringAttr` via `attrValidators[24]` | `TestRFC7606TrafficEngineeringMalformed` |
| received UPDATE with bad-length IPv6 ext-comm | → | `validateIPv6ExtCommunityAttr` via `attrValidators[25]` | `TestRFC7606IPv6ExtCommunityMalformed` |
| received UPDATE with malformed ATTR_SET | → | `validateAttrSetAttr` via `attrValidators[128]` | `TestRFC7606AttrSetMalformed` |
| RFC 7606 error on a real session | → | `session_validation.go` log sites | `TestRFC7606LogIncludesNLRIAndUpdate` |
| unrecognized typed NLRI received | → | (fill from research) | (fill) |
| ze encodes an UPDATE with two NLRI-bearing fields | → | `buildUpdatePayload` | (fill) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Attribute 24 with length 0 | treat-as-withdraw, AttrCode 24 |
| AC-2 | Attribute 24 with length 35 (one octet short of a descriptor) | treat-as-withdraw |
| AC-3 | Attribute 24 with length 36 (exactly one descriptor, no SC-specific info) | accepted, no error |
| AC-4 | Attribute 25 with length 0 | treat-as-withdraw, AttrCode 25 |
| AC-5 | Attribute 25 with length 19, 21 or 30 (not a multiple of 20) | treat-as-withdraw |
| AC-6 | Attribute 25 with length 20 and 40 | accepted |
| AC-7 | Attribute 25, valid length, UNRECOGNIZED Type and Sub-Type | accepted — §7.15-2 |
| AC-8 | ATTR_SET with length < 4 | treat-as-withdraw, AttrCode 128 |
| AC-9 | ATTR_SET containing an MP_REACH or MP_UNREACH attribute | treat-as-withdraw |
| AC-10 | ATTR_SET whose inner attribute is itself malformed (e.g. ORIGIN length 2) | treat-as-withdraw |
| AC-11 | ATTR_SET whose inner attribute stream is truncated | treat-as-withdraw |
| AC-12 | Well-formed ATTR_SET (Origin AS + valid ORIGIN/AS_PATH) | accepted |
| AC-13 | ATTR_SET nested beyond the depth cap | treat-as-withdraw, no stack growth |
| AC-14 | An UPDATE triggering any RFC 7606 error | the log entry contains the NLRI involved AND the malformed UPDATE bytes |
| AC-15 | Unrecognized typed NLRI (EVPN/MVPN) received | route is discarded: not stored, not re-advertised |
| AC-16 | Recognized typed NLRI | unchanged behavior |
| AC-17 | ze re-encodes an UPDATE that would carry two NLRI-bearing fields | at most one per UPDATE on the wire |
| AC-18 | ze receives an UPDATE with several NLRI-bearing fields | still accepted (§5.1 receive-side tolerance) |
| AC-19 | End-of-RIB | unchanged |
| AC-20 | `rfc/short/rfc7606.md` | the seven `{gap}` annotations are gone, each requirement carries a positive and a negative tag, and `make ze-rfc-check` is green |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | peer sends an UPDATE with a malformed ATTR_SET | wire → ValidateUpdateRFC7606 → validateAttrSetAttr → synthesized withdrawal → RIB | `TestRFC7606AttrSetMalformed` + a `.ci` |
| 2 | operator debugs a malformed-attribute incident | session_validation log site → log contains NLRI + full UPDATE | `TestRFC7606LogIncludesNLRIAndUpdate` |
| 3 | peer sends an EVPN route of an unrecognized type | wire → EVPN parse → discard | (fill from research) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC7606TrafficEngineeringMalformed` | `internal/component/bgp/message/rfc7606_test.go` | AC-1, AC-2 | |
| `TestRFC7606TrafficEngineeringValid` | same | AC-3 | |
| `TestRFC7606IPv6ExtCommunityMalformed` | same | AC-4, AC-5 | |
| `TestRFC7606IPv6ExtCommunityValid` | same | AC-6 | |
| `TestRFC7606IPv6ExtCommunityUnknownTypeAccepted` | same | AC-7 | |
| `TestRFC7606AttrSetMalformed` | same | AC-8..AC-11, AC-13 | |
| `TestRFC7606AttrSetValid` | same | AC-12 | |
| `TestRFC7606LogIncludesNLRIAndUpdate` | `internal/component/bgp/reactor/` | AC-14 | |
| (typed NLRI discard tests) | (fill from research) | AC-15, AC-16 | |
| (UPDATE composition tests) | `internal/component/bgp/wireu/split_test.go` | AC-17, AC-19 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| TE attribute length | ≥36 | 36 | 35 | N/A (any ≥36 accepted) |
| TE attribute length (zero) | >0 | 1 (structurally, still <36) | 0 | N/A |
| IPv6 ext-comm length | non-zero multiple of 20 | 20, 40 | 0, 19 | 21 |
| ATTR_SET length | ≥4 | 4 | 3 | N/A |
| ATTR_SET nesting depth | ≤ cap | cap | N/A | cap+1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc7606-attrset-withdraw.ci` | `test/decode/rfc7606-attrset-withdraw.ci` | peer sends an UPDATE whose ATTR_SET is malformed; the routes are withdrawn, the session survives | |
| `rfc7606-ipv6-extcomm-withdraw.ci` | `test/decode/rfc7606-ipv6-extcomm-withdraw.ci` | peer sends a bad-length IPv6 Address Specific Extended Community; treat-as-withdraw, session survives | |
| `rfc7606-te-attr-withdraw.ci` | `test/decode/rfc7606-te-attr-withdraw.ci` | peer sends a malformed Traffic Engineering attribute; treat-as-withdraw | |
| `rfc7606-ipv6-extcomm-unknown-type.ci` | `test/decode/rfc7606-ipv6-extcomm-unknown-type.ci` | an unrecognized IPv6 ext-community Type with a valid length is ACCEPTED (§7.15-2 negative) | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| existing RFC 7606 interop coverage | `test/interop/scenarios/` | FRR/BIRD | these are receive-side validations of attributes no common daemon emits malformed; interop value is low and the wire behavior is proven by `.ci` hex tests. Justified as N/A unless §5.1-2 changes emitted shape, which DOES warrant one | |

### Future (if deferring any tests)
- None.

## Files to Modify
- `internal/component/bgp/message/rfc7606.go` - three attribute code constants, three
  validators, three `attrValidators` registrations
- `internal/component/bgp/message/rfc7606_test.go` - unit tests + RFC requirement tags
- `internal/component/bgp/reactor/session_validation.go` - §6 log content
- `internal/component/bgp/wireu/split.go` - §5.1-2 (scope from research)
- `internal/component/bgp/plugins/nlri/evpn/`, `.../mvpn/` - §5.4 (scope from research)
- `rfc/short/rfc7606.md` - remove seven `{gap}` annotations
- `rfc/audit/rfc7606.json` - the seven verdicts move from `unimplemented`
- `docs/features/rfc-status.md` - RFC 7606 row: gaps drop from eight to one
- `ai/RFC-REQUIREMENTS.md` - regenerated by `make ze-rfc-index`

### BGP Family Checklist (if new SAFI / capability / attribute)
No new SAFI, capability or attribute is added: codes 24, 25 and 128 already exist on the
wire and are already parsed generically. This spec adds VALIDATION for them, so the family
checklist does not apply.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | no config surface |
| CLI commands/flags | No | - |
| Env var registration | No | - |
| Doctor check | No | no new runtime dependency |
| Prometheus counters | No | existing RFC 7606 counters already cover treat-as-withdraw |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | validation of already-parsed attributes |
| 7 | Wire format changed? | Yes (if §5.1-2 lands) | `docs/architecture/wire/` |
| 9 | RFC behavior implemented, changed, or newly proven? | **Yes** | `rfc/short/rfc7606.md` and `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | No | - |
| 12 | Internal architecture changed? | No | - |
| 16 | Changed source referenced by doc source anchors? | Yes | grep `docs/` for the changed files |
| others | | No | |

## Files to Create
- Possibly `test/decode/rfc7606-attrset-*.ci` and `test/decode/rfc7606-ipv6-extcomm-*.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-rfc-check`, `make ze-lint-changed`, targeted `go test` |
| 6. Critical review | Critical Review Checklist below |
| 7-9. Fix + re-verify | - |
| 10. Deliverables | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above |
| 13. /ze-review gate | Review Gate section |
| 14. Present + close | Executive Summary; two-commit closure |

### Implementation Phases

1. **Phase: attribute validators (§7.13, §7.15, §7.16)** — the three registrations, fully
   specified by the RFCs already read. Lowest risk, one file.
   - Tests: `TestRFC7606TrafficEngineering*`, `...IPv6ExtCommunity*`, `...AttrSet*`
   - Verify: tests fail → implement → pass
2. **Phase: §6 logging** — NLRI + raw UPDATE at the three log sites.
   - Tests: `TestRFC7606LogIncludesNLRIAndUpdate`
3. **Phase: §5.4 typed NLRI discard** — scope from research.
4. **Phase: §5.1-2 UPDATE composition** — scope from research; highest risk, done last so
   a failure here cannot strand the other six.
5. **Summary + disclosure** — remove the seven `{gap}` annotations, add the tags, re-audit
   the affected verdicts, update `docs/features/rfc-status.md`, `make ze-rfc-index`.
6. **Full verification** → `make ze-rfc-check` green with seven fewer gaps.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Each validator's malformed-definition matches the RFC text quoted in this spec, not a paraphrase |
| §7.15-2 preserved | The code-25 validator reads LENGTH ONLY and never Type/Sub-Type |
| §5.1-1 preserved | MP_UNREACH-first / MP_REACH-last ordering unchanged |
| Receive tolerance | §5.1's "MUST still be prepared to receive" is not violated by the §5.1-2 fix |
| Recursion bound | ATTR_SET depth is capped and the cap is tested |
| No over-validation | The TE check rejects only what the RFC's own text supports calling malformed |
| Rule: buffer-first | No new per-UPDATE allocation on the encode path |
| Tag honesty | Every new `RFC requirement:` tag names a test that would FAIL if the code stopped complying |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Codes 24/25/128 validated | `grep -n "attrValidators\[" internal/component/bgp/message/rfc7606.go` shows 18 registrations |
| Seven gaps closed | `grep -c "{gap" rfc/short/rfc7606.md` returns 1 |
| Gate green | `make ze-rfc-check` exit 0 |
| Disclosure updated | `docs/features/rfc-status.md` RFC 7606 row names one gap, not eight |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Unbounded recursion | ATTR_SET nesting from a hostile peer must not exhaust the stack |
| Unbounded allocation | The inner attribute walk must not allocate proportional to attacker input |
| Out-of-bounds read | Every inner length must be bounds-checked before slicing (the outer walk's bug class) |
| Log flooding / disclosure | §6 dumps attacker-controlled bytes into logs: bound the size, and confirm no secret material can appear |
| Fail-closed | A validator that cannot parse must return treat-as-withdraw, never nil |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Existing RFC 7606 test turns red | STOP: a regression in the 44 enforced verdicts is a blocker, not a test to update |
| Functional/interop test fails | Check AC; if AC wrong → DESIGN |
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

- Four of the eight gaps existed because `attrValidators` is an opt-in registry: an
  unregistered code silently accepts anything. The registry shape makes adding validation
  easy and makes NOT adding it invisible, which is why the gaps went unnoticed until the
  requirement list was extracted.

## Core Insight

A gap disclosed is not a gap fixed. The value of the `{gap}` annotations was that they
made this work enumerable and cheap to scope: each one already carried the producing
`file:line` for what was missing.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| TE (code 24) check is minimal: length 0 or < 36 | Full RFC 4203/5307 switching-capability parsing | §7.13 explicitly declines to define malformation and binds only implementations that DO determine it. Over-validating would blackhole valid routes, which is worse than under-validating an attribute ze does not act on |
| ATTR_SET inner walk reuses `validateAttribute` | A separate inner validator table | RFC 6368 says "the included attributes are malformed themselves" — the same definition, so the same code |
| ATTR_SET recursion is depth-capped | Unbounded recursion | A peer controls the nesting depth |

## Known Limitations
- §5.1-1 remains a deliberate divergence (user-excluded, documented).
- The TE check cannot detect a semantically malformed but length-plausible attribute.

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above each new validator.

## Implementation Summary

### What Was Implemented
- (fill during implementation)

### Bugs Found/Fixed
- (fill during implementation)

### Documentation Updates
- (fill during implementation)

### Deviations from Plan
- (fill during implementation)

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| §7.13 implemented | unit + functional | (fill) |
| §7.15-1 implemented | unit + functional | (fill) |
| §7.15-2 still met, now by design | unit | (fill) |
| §7.16 implemented | unit + functional | (fill) |
| §6 implemented | unit | (fill) |
| §5.4 implemented | unit + functional | (fill) |
| §5.1-2 implemented | wire test | (fill) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | (fill) | file:line | (fill) |

### Fixes applied
- (fill)

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
- [ ] AC-1..AC-20 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-rfc-check` green with seven fewer gaps
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
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
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only
