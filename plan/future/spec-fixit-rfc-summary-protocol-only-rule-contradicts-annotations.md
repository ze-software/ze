# Spec: fixit-rfc-summary-protocol-only-rule-contradicts-annotations

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Updated | 2026-08-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`ai/rules/rfc-compliance.md` states, blocking: "RFC summaries are protocol-only
reference documents: they MUST NOT contain Ze-specific information -- no Ze
implementation notes, no Ze file paths, no 'Ze does/does not' statements". A
reader should be able to use any `rfc/short/` file as a standalone protocol
reference with no knowledge of Ze.

**148 of the 178 summaries name a Ze source path.** That is 83% of the corpus,
and it is not drift: most of those paths sit INSIDE the gate's own annotations,
where naming the producing function is what makes the annotation valid.

Two examples of the shape, both from the current tree. `rfc/short/rfc1334.md`
carries a `{not-applicable}` reason saying ze never issues a second PAP
Authenticate-Request, and it cites `internal/component/l2tp/ppp/pap.go` as the
authenticator that only responds. `rfc/short/rfc1350.md` carries one saying ze is
a read-only TFTP server, citing the `handler.go` site in
`internal/plugins/tftpserver` that rejects WRQ.

An annotation's whole job is to say why this codebase owes less than the literal
requirement, and `ai/rules/evidence.md` requires that justification to name the
producing function. A `{not-applicable}` that names no code is an assertion. So
the annotation contract REQUIRES what the protocol-only rule FORBIDS, in the same
file, on the same line.

**The rule is therefore unenforceable as written, and an unenforceable blocking
rule is worse than none.** It cannot be given a check, because a check would red
on 83% of the corpus. It is cited in reviews, so agents spend effort stripping
paths from summaries and other agents put them back because the annotation needs
them. Two closures this session hit exactly that: one had its Corrections
rewritten to strip Ze paths, and a separate report recorded that `rfc/short/`
"still carries pre-existing Ze paths from earlier specs" and that "the rule or
the practice needs to give."

**This spec makes them give, in one direction, decided once.** The likely shape,
to be confirmed against the rule's intent rather than assumed: the protocol-only
constraint governs the summary's PROSE, which is the standalone reference a
reader uses, and the annotation braces are a separate register where naming the
producing function is mandatory. If that is the answer, the rule says so, and a
check enforces the boundary rather than the blanket.

**Not a licence to loosen.** `rfc/short/rfc1035.md` carries a prose bullet naming
`answerQuestions` in `internal/plugins/geodns/server.go`, outside any annotation.
That is what the rule was written for and it stays forbidden. The point is that
today nothing can tell the two cases apart.


## SETTLED 2026-08-18 -- the rule half is fixed; the corpus half moves here

The contradiction this spec was written for is RESOLVED in the rule, which is the
half that was actively costing sessions. `ai/rules/points/rfc-compliance/rfc-summaries-rfc-short/keep-ze-specifics-out-of-an-rfc-summary.md`
now scopes the protocol-only constraint to a summary's PROSE and states that the
`{...}` annotation on a requirement line is a separate register where naming the
producing function is MANDATORY, so the two registers can no longer be traded
against each other.

Measured while settling it, over `rfc/short/*.md`:

| Fact | Count |
|------|-------|
| Summaries naming a Ze source path | 148 of 178 |
| Lines naming `internal/` | 1583 |
| Annotation kinds carrying one (`{not-applicable:`, `{gap:`, `{single-polarity:`) | 740, 490, 293 |
| Candidate PROSE lines still naming a Ze path | 121 across 39 files, before false positives |

The candidate count is an upper bound: it counts the English words
"internal/external" and requirement ids whose shape the sizing heuristic missed.

**What remains is a corpus cleanup and a check, and neither blocks the release.**
No product behaviour depends on where a path sits inside a summary, so by
`plan/future/README.md` this is not a defect. It moves here rather than closing,
because the prose register is still unenforced and will drift again without a
check that reads the line as annotation or prose.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/rfc-compliance.md` -- the protocol-only rule in full, and the
      annotation and ratchet machinery in the same file that requires the paths
- [ ] `ai/rules/evidence.md` -- naming the producing function, which is what the
      annotations are complying with
- [ ] `rfc/extraction/README.md` -- the five properties of the sign-off contract,
      including which facts are derived and which are authored

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/rfc.go` -- the annotation parser, what a valid
      `{not-applicable}` / `{single-polarity}` / `{gap}` reason must contain, and
      `check_summary_disposition`, which already refuses a `non-normative` reason
      that judges what ZE owes rather than what the DOCUMENT states
  → Decision: that refusal is the closest existing precedent. It already draws a
    ze-versus-document line for one field. Read how before designing another.
- [ ] `rfc/short/rfc1035.md` -- a prose Ze reference outside any annotation, the
      case the rule was written for
- [ ] `rfc/short/rfc1334.md`, `rfc/short/rfc1350.md` -- annotations whose reasons
      name producing functions, the case the rule cannot mean

**Behavior to preserve:**
- Every existing gate and ratchet in `./le rfc check`.
- An annotation still has to name the producing function. Weakening that to
  satisfy the prose rule would trade a real guarantee for a stylistic one.
- A summary's prose stays usable as a standalone protocol reference.

**Behavior to change:**
- The rule states which register it governs, and a check enforces that boundary.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An agent writes or edits an `rfc/short/<stem>.md` and runs `./le rfc check`.
- A reviewer citing `ai/rules/rfc-compliance.md` against a summary.

### Transformation Path
1. A summary is authored: prose sections, then a requirement checklist whose rows
   may carry an annotation in braces.
2. `rfc_requirements.py` parses the rows and validates each annotation's shape.
3. Nothing parses the prose against the protocol-only rule.
4. A reviewer applies the rule by reading, and cannot tell which register a path
   sits in without judging each case.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| rule <-> corpus | a human or agent reading `ai/rules/rfc-compliance.md` against a file | Yes: 148 of 178 files would fail |
| annotation <-> evidence rule | the reason text naming a producing function | Yes, read in two summaries |
| gate <-> ze-versus-document distinction | `check_summary_disposition`'s `non-normative` refusal | Not read yet: it is the precedent |

### Integration Points
- `ai/rules/rfc-compliance.md` is generated from `ai/rules/points/rfc-compliance/`.
- `docs/features/rfc-status.md` is the public ledger and is a separate register
  where Ze statements belong by design.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the parser already reads these files |
| No unintended coupling (components stay isolated) | Yes | stays inside the RFC tooling |
| No duplicated functionality (extends existing, does not recreate) | Yes | `check_summary_disposition` already draws one ze-versus-document line; extend that idea |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Python tooling |
| Registration over hardcoding | N-A | no registration surface |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validation | Status |
|----|-----------|-------|----------|------------|--------|
| A-1 | Most of the 148 name paths only inside annotation braces | sampled three files | the corpus needs real cleaning and the spec is larger | classify all 148 by register with a script | unvalidated |
| A-2 | The rule's intent was the prose register | the stated purpose is a standalone protocol reference | the intent was the whole file, and the annotation contract must move instead | ask Thomas, quoting both texts | unvalidated |
| A-3 | The prose register can be delimited mechanically | requirement rows have a known shape | the check cannot be written and the rule stays a review-time judgement | attempt the delimiter on the corpus | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Relaxing the rule reads as permission to write Ze notes into summaries | prose Ze references start appearing | the check is the answer: a boundary that is enforced is stricter than a blanket that is not |
| R-2 | The cleanup of genuine prose violations is mistaken for the whole spec | the diff is 148 files | A-1 first: classify before editing anything |
| R-3 | A-2 resolves the other way and the annotation contract has to change | Thomas says the whole file | then the producing function moves to a sidecar, and that is a bigger spec that must be written, not improvised |

## Blast Radius

`rfc/short/` (178 files), `ai/rules/rfc-compliance.md`, and the review practice
that cites it. No daemon code. If A-2 resolves toward the annotation contract
moving, the blast radius includes every annotation in the corpus.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` over a summary with a Ze path in its PROSE | -> | the new register check | a case in `internal/le/` |
| the same with a Ze path only inside an annotation reason | -> | the same | a second case, accepted |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A summary naming a Ze source path in its prose | `./le rfc check` FAILS, naming the file and the line |
| AC-2 | A summary naming a Ze source path only inside an annotation reason | PASSES: the annotation contract requires it |
| AC-3 | The corpus as it stands | Every one of the 148 classified, and every prose violation either repaired or listed with a reason |
| AC-4 | `ai/rules/rfc-compliance.md` | States which register the protocol-only rule governs, and names the check that enforces it |
| AC-5 | An annotation whose reason names no producing function | Still refused, unchanged |
| AC-6 | `rfc/short/rfc1035.md` | Its prose Ze reference is gone, and the fact it carried lives where facts about Ze belong |

## End-to-End User Stories

- An agent writes a `{not-applicable}` annotation naming the function that makes
  it true, and the gate accepts it without a reviewer citing a rule against it.
- An agent adds a Ze implementation note to a summary's prose and the gate
  refuses it, so the standalone-reference property survives.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| a Ze path in prose is refused | `internal/le/` | AC-1 | |
| a Ze path in an annotation reason is accepted | `internal/le/` | AC-2 | |
| an annotation naming no producing function is still refused | `internal/le/` | AC-5 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | - | The RFC gate is a dev tool with no daemon surface; `./le rfc check` over the real corpus is the end-to-end test | |

## Files to Modify

- `internal/le/rfc/rfc.go` -- the new register check
- `internal/le/` -- the failing case first
- `ai/rules/points/rfc-compliance/` -- the point file behind the protocol-only
  directive, then `./le rules condensed-update` and `./le rules lint`
- `rfc/short/rfc1035.md` and whichever others AC-3 finds in prose

## Files to Create

- none expected

## Implementation Steps

1. **Phase: Classify** -- script the 148 into prose hits and annotation hits
   - Verify: A-1 confirmed or broken, with counts
2. **Phase: Ask** -- put both texts to Thomas and get A-2 settled. The rule and
   the annotation contract cannot both govern the same bytes, and which one gives
   is his call, not this spec's
   - Verify: A-2 confirmed or broken, recorded
3. **Phase: Delimit** -- prove the register boundary is mechanical
   - Verify: A-3, on the real corpus
4. **Phase: Check**
   - Verify: AC-1, AC-2, AC-5
5. **Phase: Clean the prose violations and correct the rule**
   - Verify: AC-3, AC-4, AC-6, and `./le rfc check` green

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify current mode full` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/specsession/review.go`
- [ ] **Commit A:** code + tests + spec
- [ ] **Commit B:** `git rm plan/<spec>` only

## Known Limitations

- A check on the prose register cannot judge whether a summary is a GOOD
  standalone reference. It can only refuse the one signal that is mechanical, a
  Ze source path. The rest stays a review judgement, which is honest.
