---
name: ze-rfc
description: RFC Implementation Summary
---

# RFC Implementation Summary

Generate a structured implementation summary from an RFC text file.

## Instructions
1. Use ULTRATHINK for deep analysis
2. READ: `rfc/full/$ARGUMENTS.txt` (drafts: `rfc/drafts/$ARGUMENTS.txt`)
3. WRITE: `rfc/short/$ARGUMENTS.md`
4. CHECK errata: https://www.rfc-editor.org/errata/rfcNNNN
5. ALLOCATE requirement IDs (see "Requirement IDs" below) — every checklist line gets one
6. REGISTER: run `make ze-rfc-index` to render the requirement into `ai/RFC-REQUIREMENTS.md`,
   then `make ze-rfc-check`. **A summary that is new since HEAD and declares gated MUSTs
   must be enrolled in the same change** (`check_new_summaries`): writing obligations down
   and gating none of them is how a compliance claim rots. Enrolling does not mean every
   MUST is met — classify each honestly as tested, `{single-polarity}`, `{gap}` (with a
   non-"Supported" `docs/features/rfc-status.md` row) or `{not-applicable}`. The gate also
   fails a new summary that does not parse, or that captures zero requirements while
   `rfc/full/<stem>.txt` contains MUST-level keywords.
7. VERIFY: Re-read RFC and summary, check:
   - ALL wire formats captured with ASCII diagrams?
   - ALL MUST requirements listed?
   - ALL error conditions documented?
   - Key constants/type codes present?
   - Quoted requirements match RFC exactly?
   - Section references correct?
   - Field sizes/offsets accurate?
   - Compliance Checklist covers EVERY RFC 2119 keyword occurrence?
   - No duplicate checklist entries?
   - Checklist entries grouped by keyword level (MUST > SHOULD > MAY)?
   - EVERY line has a unique requirement ID, and no ID was renumbered or reused?

**Coverage self-check (BLOCKING).** Before finishing, count the normative keywords in the
source and compare against your checklist:

```
grep -oE '\b(MUST NOT|MUST|SHALL NOT|SHALL|SHOULD NOT|SHOULD|MAY)\b' rfc/full/$ARGUMENTS.txt | sort | uniq -c
grep -cE '^- \[ \] \[' rfc/short/$ARGUMENTS.md
```

The counts will not match exactly (one requirement can span several keyword occurrences, and
duplicates merge into one line). But an order-of-magnitude gap means you under-captured. This
check exists because it caught real failures: `rfc5303`, `rfc5304`, and `rfc5310` shipped
summaries with 23, 13, and 12 normative keywords in the source and **zero** captured.

## Keep the ledger committed (BLOCKING)

`ai/RFC-REQUIREMENTS.md` is generated from the summaries and the `RFC requirement:` tags,
and it records each enforcing test's `file:line`. So it goes stale not only when you add
or retire a requirement, but whenever a tagged test is added, moved, deleted, or re-tagged,
and even when an unrelated edit shifts a tagged test's line. Regenerate it with
`make ze-rfc-index` and **commit the regenerated ledger in the SAME commit** as the change
that caused the drift. `ze-rfc-check` renders the ledger and fails on any mismatch, and it
runs in both `ze-verify` and `ze-verify-changed` (`check_ledger_fresh`,
`scripts/dev/rfc_requirements.py`). A ledger left stale is not silently tolerated: it
surfaces later as a cross-commit diff that the next session inherits and the freshness gate
pins on them. This cuts both ways: it is also why you must not regenerate the ledger as a
side effect of unrelated work. If the diff is a pure line-number refresh with no change of
yours behind it, a prior commit skipped the regen, so commit that refresh on its own.

## Structure

### Meta
- RFC title, number, status, date
- Obsoletes / Updates / Obsoleted-by
- Purpose (1-2 sentences)
- Scope: AFI/SAFI if BGP extension

### Wire Formats
For EACH format defined (message/attribute/capability/NLRI):

```
#### <Name> Format
Type code: X (if applicable)
Flags: O=? T=? P=? E=? (for attributes)

<ASCII diagram verbatim>

| Field | Offset | Size | Type | Constraints |
|-------|--------|------|------|-------------|

Length calculation: <formula>
Parse order: <if non-obvious>
```

### Encoding Rules
- Byte ordering (exceptions to network order)
- Variable-length field encoding
- Optional field presence rules
- Padding requirements

### Decoding Rules
- Parse sequence
- How to determine field boundaries
- Unknown/unsupported value handling

### Validation
| Check | Valid | Invalid Action |
|-------|-------|----------------|

### MUST Requirements
Group by:
- **Tx**: What sender MUST do
- **Rx**: What receiver MUST do
- **Validation**: What MUST be checked
- **Errors**: How MUST respond
- **State**: State machine MUSTs
- **Timers**: Timing MUSTs

### SHOULD/MAY
- [SHOULD] requirement - <consequence if not>
- [MAY] option - <when to use>

## Compliance Checklist

**MANDATORY section.** Every RFC 2119 keyword (MUST, MUST NOT, SHOULD,
SHOULD NOT, MAY, SHALL, SHALL NOT, REQUIRED, RECOMMENDED, NOT
RECOMMENDED, OPTIONAL) extracted from the RFC becomes one checkbox
line. This section is the single place an implementer verifies full
coverage.

Format:
```
- [ ] [RFC7606-2-1] [MUST] <requirement quoted or tightly paraphrased> (§2)
- [ ] [RFC7606-2-2] [MUST NOT] <prohibition> (§2)
- [ ] [RFC7606-5.3-1] [MUST] <another obligation, different section> (§5.3)
- [ ] [RFC7606-5.3-2] [SHOULD] <recommendation> (§5.3)
- [ ] [RFC7606-7.1-1] [NOT RECOMMENDED] <discouragement> (§7.1)
- [ ] [RFC7606-7.1-2] [MAY] <option> (§7.1)
```

Rules:
- One line per distinct requirement, not per sentence. If a sentence
  contains two independent obligations, split them.
- Merge duplicates: if the same requirement appears in multiple sections,
  list it once with all section references.
- Group by keyword level: all MUST/MUST NOT first, then SHOULD/SHOULD NOT,
  then MAY.
- Within each group, order by section number.
- The checkbox is always `[ ]` (unchecked). It is a template marker, NOT
  coverage state. Coverage is DERIVED from test tags and rendered in
  `ai/RFC-REQUIREMENTS.md` — never tick a box to claim a requirement is met.
- Quote the RFC text when short enough; paraphrase only when the original
  is too long for a single line.
- Include the section reference so the implementer can find the normative
  text.

## Requirement IDs

Every checklist line carries a stable ID **`RFC<number>-<section>-<ordinal>`**:

```
RFC7606-5.3-4      §5.3, 4th requirement of that section
RFC7606-3.b-1      §3.b, 1st
RFC4271-9.1.2.2-3  §9.1.2.2, 3rd
```

Drafts use the uppercased filename stem (`DRAFT-IETF-BESS-MUP-SAFI-6-1`).
This ID is what tests reference, so it is a permanent contract.

**Why the section, not a counter.** RFCs are immutable: section numbers are frozen the
day the RFC publishes, which makes them the most stable name available. A bare counter
(`RFC7606-R055` — the RETIRED form; never allocate one, the parser rejects it) encodes
ALLOCATION ORDER — a fact about our editing history, not about
the RFC. It teaches a reader nothing, its neighbours are unrelated requirements, and the
moment you add a requirement (which must go at the end, since renumbering is forbidden)
the numbering stops tracking the document at all.

| Rule | Why |
|------|-----|
| Anchor to the FIRST section the line cites; ordinal counts within that section | Section is immutable; the id says where to read the normative text |
| The id's section MUST match the line's `(§N)` | Checked by `make ze-rfc-check` — the id and the citation can never drift apart |
| Ordinals are per-section, starting at 1 | Adding to §5.3 only has to clear §5.3's high-water, so it lands beside its siblings |
| **Never renumber. Never reuse.** A new requirement takes the next free ordinal IN ITS SECTION | A renumber silently re-points every test tag at the wrong requirement |
| When REMOVING a requirement, retire the ordinal — do not backfill the hole | Reuse makes an old tag point at a new, unrelated obligation |
| A line citing no section of its own anchors to `x` (`RFC1071-x-1`) | Conspicuous on purpose: a missing section reference is a defect to fix, not a resting state |
| A cite naming ANOTHER RFC (`(RFC 2328 §A.3.1)`) is NOT an anchor | That is RFC 2328's section; hanging this RFC's id off it names the wrong document |

If a requirement's TEXT needs correcting (a misquote), edit the text and KEEP the ID —
then re-run `/ze-rfc-audit <rfc>`, because changing requirement text re-stales every
verdict bound to it, by design. If the SECTION was wrong, the id changes with it; that is
safe only while no test tags it (a dangling tag fails the gate, which is the backstop).

## Annotations (when no test is expected)

A MUST-level requirement in an **enrolled** RFC must be covered by tests, or carry exactly
one annotation explaining why not. Every annotation needs a reason — a bare annotation is
rejected by `make ze-rfc-check`.

```
- [ ] [RFC5303-3-1] [MUST] <text> (§3) {not-applicable: Ze does not implement IS-IS mesh groups}
- [ ] [RFC7606-5.1-1] [MUST] <text> (§5.1) {gap: Ze emits MP_UNREACH first, MP_REACH last; docs/architecture/wire/mp-nlri-ordering.md}
- [ ] [RFC7606-4-1] [MUST] <text> (§4) {single-polarity: negative; no conforming input exists to assert positively}
```

| Annotation | Meaning | Gate behavior |
|------------|---------|---------------|
| `{not-applicable: why}` | Not applicable to Ze (protocol not implemented, role not played) | Passes. MUST NOT coexist with any test tag |
| `{gap: why; ref}` | Known, deliberate divergence | Passes, but MUST be disclosed in that RFC's `docs/features/rfc-status.md` row. Not a place to hide an accident |
| `{single-polarity: positive\|negative; why}` | Genuinely testable only one way | Requires tags of that polarity only, instead of the usual pair |

**These are not escape hatches.** If a requirement has no test because nobody wrote one
yet, that is not `not-applicable` — write the test, or leave the RFC un-enrolled. Both
`{not-applicable}` and `{gap}` FAIL the gate the moment a test tags the requirement
(a stale annotation is a lie the ratchet catches).

**Writing any of the three is Thomas's call, not yours** (owner directive 2026-07-27,
`ai/rules/rfc-compliance.md` "Ask Thomas Whenever Full Compliance Is On The Table").
Implementing the requirement fully and proving it with a tagged test is always an
available answer, so choosing an annotation instead is choosing less: stop, quote the
requirement text and the producing code `file:line`, and ask which way he wants it.
An annotation you find already in place from an earlier session is VOID as authority —
re-derive it from the RFC text, and ask again if it still reads as less than full.

## Linking Requirements to Tests

The link is two-way, but only ONE side is authored: the test tags itself.
`ai/RFC-REQUIREMENTS.md` renders the reverse direction and is GENERATED — never hand-write
a test path into a summary (`ai/rules/derive-not-hardcode.md`). A hand-written back-link
survives deletion of the test it names; a tag dies with the test.

The rendered link carries the test's `file:line`, so the ledger drifts whenever a tagged
test moves. Regenerate and commit it in the same change (see "Keep the ledger committed").

In the test that enforces a requirement:

```go
// RFC requirement: RFC7606-7.1-1 negative — ORIGIN length 2 is treated as withdraw
func TestRFC7606MalformedOriginLength(t *testing.T) {

// RFC requirement: RFC7606-7.1-1 positive — valid ORIGIN length 1 is accepted
func TestRFC7606OriginValueIGP(t *testing.T) {
```

```
# RFC requirement: RFC7606-7.1-1 negative — malformed ORIGIN withdraws the route
```

Rules:
- One ID per tag line, with a mandatory polarity. Multiple tags = multiple lines.
- **Every gated requirement needs BOTH a positive and a negative tag** unless it carries
  `{single-polarity: ...}`. A negative-only test passes if the code rejects everything; a
  positive-only test passes if it accepts everything. Only the pair pins behavior to the
  requirement.
- Place the tag on the test function's doc comment when the function tests exactly one
  requirement. Place it **inline at the table case** when one function covers many
  (e.g. `TestRFC7606SystematicLengthCorruption` spans ~100 cases across a dozen
  requirements) — a function-level tag there stays green after the one enforcing case
  is deleted, which is exactly the rot this system exists to catch.
- `.ci` tags must start at the line start (`internal/test/runner/parsing.go:248`) and must
  not sit inside a `terminator=` block, where `#` is raw file content, not a comment.

## Changing an RFC Test (BLOCKING)

**Once a test carries an `RFC requirement:` tag, do not change its behavior without
explicit user approval.** `ai/rules/testing.md` already says fix the code, not the test;
for RFC-tagged tests this is enforced, because a test that encodes a standards obligation
is the only thing standing between a regression and a shipped protocol violation.

| Situation | Do |
|-----------|-----|
| A tagged test fails after your change | Fix YOUR code. The test is the requirement |
| You believe the test is genuinely wrong | STOP. Show the user the RFC text and the test, and ask. Do not edit and explain afterwards |
| The RFC text itself was misquoted | Fix the summary line (keep the ID), then re-run `/ze-rfc-audit` |
| Refactor/rename/format only | Allowed — behavior must be unchanged |

`// test-relax:` does **not** authorize changing a tagged test. It is self-service: you
would be writing your own approval. Only the user can approve.

### Error Handling
| Condition | Detect How | Response | Code/Subcode |
|-----------|------------|----------|--------------|

### State Machine
| State | Event | Guard | Action | Next |
|-------|-------|-------|--------|------|

### Timers
| Name | Default | Range | Behavior |
|------|---------|-------|----------|

### Constants
| Name | Value | Usage |
|------|-------|-------|

### Algorithms
Step-by-step, pseudocode if RFC provides it.

### Pitfalls
- **Edge cases**: <specific scenarios>
- **Interop**: <known issues with other implementations>
- **Security**: <attack vectors, mitigations>

### Compatibility
- Behavior with non-supporting peers
- Feature negotiation fallback

## Rules
- MUST read file, never from memory
- ASCII diagrams: copy EXACTLY (spacing matters for field boundaries)
- Requirements: quote verbatim, cite section number
- Tables: prefer over prose for structured data
- Skip sections that don't apply (no empty sections)
- Skip: abstract, introduction (unless defines terms), acknowledgments, full IANA section (keep just the values)
- Errata: note any that affect implementation
- Every checklist line gets a unique, permanent ID. Never renumber, never reuse
- Never tick a checkbox — coverage is derived from test tags, not declared here
- Never hand-write a test path into a summary — `ai/RFC-REQUIREMENTS.md` is generated
- Regenerate `ai/RFC-REQUIREMENTS.md` (`make ze-rfc-index`) and commit it in the same change
  whenever a tagged test is added, moved, deleted, or re-tagged; it records `file:line`, and
  `ze-rfc-check` fails on a stale ledger
- Never annotate `{not-applicable}` / `{gap}` to reach green. Write the test, or leave
  the RFC un-enrolled and say so
- Enrollment (`rfc/enrolled.txt`) means "every MUST here is CLASSIFIED": tested, or
  annotated with a reason that names what is missing. A NEW summary declaring gated MUSTs
  must be enrolled in the same change; only the pre-existing backlog is grandfathered
- Coverage is monotonic. A requirement that had a positive and a negative test cannot be
  demoted to `{gap}` later: `check_coverage_ratchet` compares against HEAD and an
  annotation does not substitute for proof that already existed
- A requirement id of an enrolled RFC cannot be deleted from its summary
  (`check_retired_requirements`). Fix a misquoted requirement by editing its TEXT under the
  same id; deleting the line retires the obligation silently

## Related

| Need | Use |
|------|-----|
| Check requirements are covered | `make ze-rfc-check` |
| Regenerate the requirement ledger | `make ze-rfc-index` → `ai/RFC-REQUIREMENTS.md` |
| Re-audit that tests still enforce letter and spirit | `/ze-rfc-audit <rfc>` |
| Public per-RFC support status | `docs/features/rfc-status.md` (product ledger; must agree with `{gap}` annotations) |
