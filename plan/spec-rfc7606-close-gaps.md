# Spec: rfc7606-close-gaps

| Field | Value |
|-------|-------|
| Status | complete |
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
| unrecognized typed NLRI received | → | WITHDRAWN from scope (Section 5.4, owner decision) | n/a |
| ze re-chunks an UPDATE with two NLRI-bearing fields | → | `buildCombinedUpdates` (`wireu/split.go`) | `TestSplitWireUpdateOneNLRIFieldPerMessage` |

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
| AC-15 | ~~Unrecognized typed NLRI discarded~~ | **WITHDRAWN from scope** by owner decision (Section 5.4); ze retains and propagates, now documented as a reasoned divergence |
| AC-16 | ~~Recognized typed NLRI unchanged~~ | **WITHDRAWN with AC-15** |
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
| An ATTR_SET's inner attributes can be judged in the enclosing session's context | RFC 6368 Section 5 fixes the inner encoding "regardless of the capabilities advertised", and the attribute exists to carry the CUSTOMER's iBGP attributes | independent review | three bugs that would have withdrawn conforming routes. The clause was in RFC text I had already quoted into this spec: reading the RFC is not the same as reading it for the question being answered |
| Any non-None inner result means the ATTR_SET is malformed | RFC 7606 grades its actions; attribute-discard exists so the route SURVIVES | independent review | escalating inverted the document's central design decision |
| A test asserting "no log line appears" pins an Enabled() guard | a level-filtering handler drops the line either way | mutation testing after review | the audit note carried a false claim about what the test proved |
| Tagging the split tests as proof of RFC7606-5.1-2 | they prove the re-chunk path only; the MUST is not met | `make ze-rfc-check` caught the contradiction against the surviving `{gap}` | the overclaim never reached a commit |

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
- **Section 7.13, 7.15-1, 7.15-2, 7.16** — `internal/component/bgp/message/rfc7606_optional_attrs.go`:
  validators for attribute codes 24, 25 and 128, which had none. `attrValidators` is opt-in
  and `validateAttribute` returns nil for an unregistered code, so any length was accepted.
- **Section 6** — `Session.rfc7606Diagnostics` (`reactor/session_validation.go`) logs the
  NLRI involved and the entire UPDATE body at all four enforcement outcomes, gated on
  `Enabled()` so a malformed-UPDATE flood cannot make ze hex-encode for free.
- **Section 5.1-2 (narrowed, not closed)** — `buildCombinedUpdates` (`wireu/split.go`) now
  drains each NLRI-bearing component into its own message.
- Commits: `9eebcd30e` (the five gaps), `574e3c596` and `a6ad6e8d5` (three defects found
  along the way).

### Bugs Found/Fixed
- Three over-validation bugs in my own ATTR_SET walker, found by independent review, each of
  which would have WITHDRAWN CONFORMING ROUTES: inner attributes were judged with the
  session's `asn4` and `isIBGP`, and an inner attribute-discard was escalated to a
  whole-route withdraw. See Mistake Log.
- `Splitter.splitUpdateWithMP` silently dropped IPv4 WithdrawnRoutes and NLRI when an UPDATE
  also carried an MP attribute (pre-existing, fixed in `574e3c596`).
- `EVPNGeneric.WriteTo`/`Bytes` omitted the `[route-type][length]` header, so the encode path
  shipped two trailing zero octets and the JSON `raw` lost the first two body octets
  (pre-existing, fixed in `574e3c596`).
- `decodeBGPLSNLRI` abandoned every remaining NLRI after the first unparseable one
  (pre-existing, fixed in `a6ad6e8d5`).
- `docs/features/rfc-status.md` claimed seven gaps while the summary carried eight; the
  undisclosed one was 5.1-2.

### Documentation Updates
- `rfc/short/rfc7606.md`: five `{gap}` annotations removed, 5.1-2 narrowed and homed on
  `plan/spec-rfc7606-5-1-2-relay-shape.md`, 5.4 re-framed as a reasoned divergence with the
  mechanism corrected.
- `docs/features/rfc-status.md`: eight gaps → three, each described honestly.
- `rfc/audit/rfc7606.json`: five verdicts moved to `enforced` after implementation.
- `rfc/full/rfc5543.txt` and `rfc/full/rfc6368.txt` fetched, since 7.13 and 7.16 delegate
  their definition of "malformed" to them.

### Deviations from Plan
- **Section 5.4 dropped from scope** by owner decision after analysis: ze has no EVPN
  forwarding plane, so discarding unrecognized types would remove function from the PEs it
  relays between while improving nothing locally. Now owned as a documented divergence, and
  the survey behind it is recorded in the annotation.
- **Section 5.1-2 not closed**, carved into its own spec. The remaining half costs the
  zero-copy forward path, which deserves measuring before paying.
- `_git_cat_blobs`-style batching was not needed here; the TE check is minimal by design.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| RFC7606-7.13-1 | done | `rfc7606_optional_attrs.go` `validateTrafficEngineeringAttr` | positive + negative tagged |
| RFC7606-7.15-1 | done | `validateIPv6ExtCommunityAttr` | positive + negative tagged |
| RFC7606-7.15-2 | done | same validator, value ignored | now met by design, not omission; single-polarity positive |
| RFC7606-7.16-1 | done | `validateAttrSetAttr` / `validateAttrSetDepth` | positive + negative tagged |
| RFC7606-6-1 | done | `reactor/session_validation.go` `rfc7606Diagnostics` | positive + negative tagged |
| RFC7606-5.1-2 | partial, owner-approved | `wireu/split.go` `buildCombinedUpdates` | re-chunk path compliant; relay paths carved to `plan/spec-rfc7606-5-1-2-relay-shape.md` |
| RFC7606-5.4-1 | withdrawn from scope | - | owner decision; documented divergence |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRFC7606TrafficEngineering*` | done | `message/rfc7606_optional_attrs_test.go` | boundary at 35/36 |
| `TestRFC7606IPv6ExtCommunity*` | done | same | 0/19/20/21/30/40 plus unrecognized type |
| `TestRFC7606AttrSet*` | done | same + `rfc7606_attrset_context_test.go` + `rfc7606_attrset_discard_test.go` | the context and discard files were added after review found three over-validation bugs |
| `TestRFC7606Diagnostics*` | done | `reactor/session_rfc7606_diagnostics_test.go` + `session_rfc7606_diag_cost_test.go` | the cost file replaces a tautological negative |
| split one-field-per-message | done | `wireu/split_rfc7606_test.go` | untagged: proves the re-chunk path only, not the MUST |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `message/rfc7606.go` | not modified | already at the 1000-line limit; the new code went to its own file |
| `message/rfc7606_optional_attrs.go` | created | validators + registrations |
| `reactor/session_validation.go` | modified | Section 6 |
| `wireu/split.go` | modified | Section 5.1-2, partial |
| `evpn/`, `mvpn/` | not modified | Section 5.4 withdrawn from scope |
| `rfc/short/rfc7606.md`, `rfc/audit/rfc7606.json`, `docs/features/rfc-status.md` | modified | 8 gaps → 3 |

### Audit Summary
- **Total items:** 7 requirements, 20 ACs, 8 files
- **Done:** 5 requirements fully (7.13, 7.15-1, 7.15-2, 7.16, 6); 17 ACs
- **Partial:** 1 (RFC7606-5.1-2) — owner-approved, carved into its own spec
- **Skipped:** 1 (RFC7606-5.4-1) and its 2 ACs — owner decision, documented as a divergence
- **Changed:** 4 deviations, all in Deviations from Plan

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| §7.13 implemented | unit + mutation | `TestRFC7606TrafficEngineeringTooShort` / `...Valid`; removing the code-24 registration fails the first |
| §7.15-1 implemented | unit + mutation | `TestRFC7606IPv6ExtCommunityBadLength` / `...ValidLength`; removing the code-25 registration fails the first |
| §7.15-2 still met, now by design | unit | `TestRFC7606IPv6ExtCommunityUnrecognizedType`: Type 0x3f / Sub-Type 0xee at valid length 20 is accepted, because the validator takes the attribute value as `_` |
| §7.16 implemented | unit + mutation | `TestRFC7606AttrSetMalformed` / `...Valid` / `...InnerASPathAlwaysFourOctet` / `...InnerIBGPAttributesOnEBGPSession` / `...InnerDiscardDoesNotWithdraw` / `...NestingCapBoundary`; four mutants killed |
| §6 implemented | unit + mutation | `TestRFC7606DiagnosticsListNLRIAndUpdate` asserts the prefix and the raw pre-rewrite ORIGIN bytes; `TestRFC7606DiagnosticsCostsNothingWhenDisabled` asserts zero allocations and fails if the Enabled() guard is removed |
| §5.4 implemented | - | **WITHDRAWN from scope** by owner decision; now a documented divergence with the mechanism corrected |
| §5.1-2 implemented | wire test | **PARTIAL.** `wireu/split_rfc7606_test.go` proves the re-chunk path emits one field per message and fails if `split.go` is reverted to HEAD. The relay paths are NOT covered; the gap stands, owned by `plan/spec-rfc7606-5-1-2-relay-shape.md` |
| No regression | package tests | `internal/component/bgp/...` 81 packages ok; the single red is the known missing-build-tag artifact in `bgp/config`, green with the ze_core plus ze_ssh tags |
| Gate | ze-rfc-check | green, 2535 tags resolved, up from 2519 |
| Lint | golangci-lint | 0 issues across all five changed packages |

## Review Gate

One independent adversarial subagent over the whole diff, then author mutation testing.
Artifacts: `tmp/review/rfc7606-close-gaps-<sid>.md`,
`tmp/review/rfc7606-preexisting-defects-<sid>.md`, `tmp/review/rfc7606-bgpls-skip-<sid>.md`.

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | ATTR_SET judged its inner AS_PATH/AGGREGATOR with the SESSION's asn4; RFC 6368 Section 5 requires the 4-octet encoding "regardless of the capabilities advertised", so a conforming inner AS_PATH was read with a 2-octet AS size and the route withdrawn | `rfc7606_optional_attrs.go` | fixed: inner context forced to asn4=true |
| 2 | ISSUE | Same for isIBGP: an ATTR_SET preserving the customer's LOCAL_PREF was withdrawn on an eBGP session, which is the attribute's entire purpose | same | fixed: inner context forced to isIBGP=true |
| 3 | ISSUE | An inner attribute-discard was escalated to a whole-route withdraw, inverting RFC 7606's deliberate grading | same | fixed: gated on >= TreatAsWithdraw |
| 4 | ISSUE | The Section 6 negative test was tautological AND the audit note claimed it "pins the Enabled() guard" -- a false claim in a compliance artifact | `session_rfc7606_diagnostics_test.go`, `rfc/audit/rfc7606.json` | fixed: note corrected, allocation-based test added |
| 5 | ISSUE | The Section 5.1-2 rewrite had NO test; reverting it left everything green | `wireu/split.go` | fixed: three tests, verified to fail on revert |
| 6 | MINOR | `depth > cap` admitted cap+1 nesting levels while the message named cap | `validateAttrSetDepth` | fixed, boundary tested |
| 7 | MINOR | Section 6 missing on the NLRI-syntax enforcement path | `session_validation.go` | fixed: wired |
| 8 | MINOR | Vacuous EoR justification comment | `wireu/split.go` | fixed: reworded to say the path is unreachable |
| 9 | MINOR | Two of three Section 6 assertions non-discriminating (one IS load-bearing) | `session_rfc7606_diagnostics_test.go` | accepted; the load-bearing one proves the pre-rewrite dump |

### Fixes applied
- All nine, plus the overclaim the GATE caught after review: the split tests had been
  tagged as proof of RFC7606-5.1-2 while the requirement still carried a `{gap}`.
  Tags removed under a recorded owner approval; the tests remain as regression cover.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | none | Mutation testing after the fixes: 4 ATTR_SET mutants, the split revert, the Enabled()-guard removal, the EVPN header revert and the BGP-LS revert are ALL killed | - | - |

### Final status
- [ ] Review re-run shows 0 BLOCKER, 0 ISSUE -- every finding fixed and mutation-verified
- [ ] NOTEs recorded: Run 1 #9

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `message/rfc7606_optional_attrs.go` + 3 test files | yes | committed in `9eebcd30e` |
| `reactor/session_rfc7606_diagnostics_test.go`, `..._diag_cost_test.go` | yes | same |
| `wireu/split_rfc7606_test.go` | yes | same |
| `rfc/full/rfc5543.txt`, `rfc/full/rfc6368.txt` | yes | fetched 2026-07-20, same commit |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-3 | TE attribute bounds | `TestRFC7606TrafficEngineeringTooShort` (0/1/35) and `...Valid` (36/44/72) |
| AC-4..AC-7 | IPv6 ext-community length and type tolerance | `TestRFC7606IPv6ExtCommunityBadLength` (0/19/21/30), `...ValidLength` (20/40), `...UnrecognizedType` |
| AC-8..AC-13 | ATTR_SET malformed conditions and nesting | `TestRFC7606AttrSetMalformed` / `...Valid` / `...NestingCapBoundary` |
| AC-14 | Section 6 logs NLRI and the UPDATE | `TestRFC7606DiagnosticsListNLRIAndUpdate` asserts `10.0.0.0/8` and the raw `400102` |
| AC-15, AC-16 | withdrawn from scope | owner decision, recorded in Deviations |
| AC-17 | one NLRI-bearing field per re-chunked UPDATE | `TestSplitWireUpdateOneNLRIFieldPerMessage` |
| AC-18 | receive-side tolerance unchanged | no receive path was modified; the reactor and message suites pass |
| AC-19 | End-of-RIB unaffected | `SplitWireUpdate` returns early for any payload that fits, so an EoR never reaches the changed code |
| AC-20 | seven gaps gone | `grep -c '{gap' rfc/short/rfc7606.md` returns 3 (5.1-1 excluded, 5.1-2 partial, 5.4 withdrawn) |

### Wiring Verified (end-to-end)
No `.ci` was added. The planned functional tests are NOT present -- see Deviations.

| Entry Point | Test | Verified |
|-------------|------|----------|
| received UPDATE -> `ValidateUpdateRFC7606` -> each new validator | every unit test drives the entry point, not the validator directly | yes |
| RFC 7606 enforcement -> log | `TestRFC7606Diagnostics*` drive `enforceRFC7606` | yes |
| oversized UPDATE -> `SplitWireUpdate` | `wireu/split_rfc7606_test.go` | yes |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | nothing in ze emits code 24 or 128; `IPv6ExtendedCommunities.Len()` is `len(e)*20` (`core/bgp/attribute/community.go:486`), always a valid multiple, so ze cannot reject its own output |
| A-2 | confirmed | RFC 5543 Section 3 gives 4+8x4=36 fixed octets per descriptor; the reviewer independently confirmed L2SC and LSC carry no switching-capability-specific information, so 36 is exactly the smallest well-formed descriptor |
| A-3 | confirmed | depth cap tested on both sides; the reviewer ran 500,000 random inputs through the walker with no panic |
| A-4 | confirmed | origination is already compliant; only the two relay fits-branches remain, and they are carved into their own spec |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| rfc-status RFC 7606 row says three gaps | matches `grep -c '{gap' rfc/short/rfc7606.md` = 3 and the three non-enforced audit verdicts | yes |
| Section 5.4 divergence rationale | ze has no EVPN dataplane per the RFC 7432 row; RFC 9552 Section 5.1 override quoted from `rfc/short/rfc9552.md:28`; GoBGP/ExaBGP behaviour read from source in `~/Code` | yes |
| Section 5.1-2 gap names its owning spec | `plan/spec-rfc7606-5-1-2-relay-shape.md` exists | yes |

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
