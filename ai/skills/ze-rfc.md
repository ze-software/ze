---
name: ze-rfc
description: RFC Implementation Summary
---

# RFC Implementation Summary

Generate a structured implementation summary from an RFC text file.

## Instructions
1. Use ULTRATHINK for deep analysis
2. READ: `rfc/full/$ARGUMENTS.txt` (drafts: `rfc/drafts/$ARGUMENTS.txt`). If it is not
   there, FETCH it before writing a line of summary -- `curl -o rfc/full/$ARGUMENTS.txt
   https://www.rfc-editor.org/rfc/$ARGUMENTS.txt`, or the datatracker archive for a draft.
   Enrolment REQUIRES this file (`check_enrolment`, owner ruling 2026-07-27): a summary
   written without its source is validated only against itself, which permits both a
   requirement the RFC does not contain and, invisibly, one it does contain that was never
   extracted.
3. WRITE: `rfc/short/$ARGUMENTS.md`
4. CHECK errata: https://www.rfc-editor.org/errata/rfcNNNN
5. ALLOCATE requirement IDs (see "Requirement IDs" below) — every checklist line gets one
6. REGISTER: run `./le rfc index-update` to render the requirement into `rfc/requirements/<stem>.md`,
   then `./le rfc check`. **A summary that is new since HEAD and declares gated MUSTs
   must be enrolled in the same change** (`check_new_summaries`): writing obligations down
   and gating none of them is how a compliance claim rots. Enrolling does not mean every
   MUST is met — classify each honestly as tested, `{single-polarity}`, `{gap}` (with a
   non-"Supported" `Support status` row in the summary's own `## Meta` table) or
   `{not-applicable}`. The gate also fails a new summary that does not parse, or that
   captures zero requirements while
   `rfc/full/<stem>.txt` contains MUST-level keywords. **Enrolling a stem that was not
   enrolled at HEAD also requires an extraction sign-off** (`rfc/extraction/<stem>.json`);
   see "Extraction sign-off" below.

6b. DECLARE ENROLMENT AND THE PUBLIC ROW (BLOCKING). Both are declared in the summary's
   own `## Meta` table, and nowhere else. `rfc/enrolled.txt`, `rfc/not-enrolled.txt` and
   `docs/features/rfc-status.md` are GENERATED from those tables by `./le rfc index-update`,
   so an edit to one of the three is lost at the next run. The summary parser REFUSES an
   absent or unrecognized value and never defaults (`internal/le/rfc/meta.go` `ParseMeta`).
   A summary that declares nothing does not parse. An RFC leaves the gated population by a
   decision, never by an absence.

   | Meta row | Value |
   |----------|-------|
   | `Enrolment` | one of `enrolled`, `backlog`, `blocked`, `non-normative`, `out-of-scope`, `source-restricted` |
   | `Enrolment reason` | what makes that kind true. Never blank |
   | `Support` | `-` for no public row, or `<section-key> <rank>`. The keys are the sections `statusSections` declares (`internal/le/rfc/render_ledger.go`). The rank orders this RFC inside its section, because the page's reading order is authored |
   | `Support name` | the first cell, ONLY where an `rfc<number>` stem cannot derive it |
   | `Support area`, `Support status`, `Support coverage` | the authored cells, moved verbatim onto the page. Required whenever `Support` names a section |
   | `Support remaining` | what is not complete, or `-`. The four-column `drafts` section has no such column, so a row there writes `-` |

   Enrolling is ONE edit to ONE file: write `| Enrolment | enrolled |` with its reason,
   then run `./le rfc index-update`. Two states are unrepresentable now rather than
   refused: a stem in both disposition files, and a disposition naming a summary that does
   not exist. One field holds one value, and it dies with the summary that carries it.

   The five kinds that are not `enrolled`:

   - `non-normative` — the DOCUMENT imposes no MUST-level obligation on any speaker. Say
     what makes that true of the TEXT: its IETF category, the absence of an RFC 2119
     key-words section, a keyword scan. It must NOT say the obligation does not apply to Ze.
     That is a conformance judgement `ai/rules/rfc-compliance.md` reserves to the owner. The
     gate rejects a reason phrased that way.
   - `source-restricted` — DECISION, and the only permanent one. The standard's own text
     cannot be redistributed, so it can never sit under `rfc/full/` and `checkEnrolment`
     can never accept an enrolment. Name the publishing body (ISO, IEC, ITU, IEEE, ANSI, ETSI)
     or the license, copyright or paywall that stops the copy. Where the text IS fetchable
     the kind is `blocked` instead, and a fetch discharges it. It excuses NO public support
     claim: being unable to bound a claim is a reason to stop making it, so such a summary
     writes `Unsupported`, `Future`, or `| Support | - |`.
   - `out-of-scope` — DECISION. The extraction is DONE and the owner declined the feature
     for now, so the obligations are declared in full and none is gated. Its reason carries
     the date the decision was taken, and its public row may read only `Future` or
     `Unsupported`.
   - `backlog` — DEBT. The extraction is owed, or its obligations are not yet proven.
   - `blocked` — DEBT. Something outside the summary prevents enrolment, most often a
     missing `rfc/full/<stem>.txt`.

   `checkUnprovenSupport` reds separately when the public row claims support and the
   summary declares zero gated requirements. Extract the obligations. Or record the
   evidence that zero is a property of the document: a `non-normative` disposition, or a
   `manual-walk` sign-off whose `register-reason` says why.
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

**Extraction sign-off (BLOCKING before enrolling).** The old two-grep, eyeball-the-ratio
coverage check is superseded: it was an honour-system count that nothing recorded and
nothing re-checked. Record the walk instead, in an artifact a machine re-checks on every
`./le rfc check`:

```
./le rfc extraction-create stem $ARGUMENTS   # skeleton to session scratch, never to rfc/extraction/
                                             # then classify every site and section by hand,
                                             # then move the file in as the command says
./le rfc check                               # re-derives the inventory and judges it
```

The skeleton lists every normative site of the source (`<section>:<n>` plus the derived
sentence) and every section, with each disposition `null`. Classify each site `mapped` to
a requirement id or `excluded` with a kind and a reason, and each section `walked` or
`skipped`. **An unclassified site fails the check**, so generating the skeleton makes the
gate redder, never greener; only the walk makes it green. That is why the
skeleton lands in the scratch: an unclassified artifact under `rfc/extraction/`
reds the gate for the whole corpus, and generating a batch of them would red it
for everybody at once. The command writes in place only when every site and
section already carries a disposition, which is the refresh of a walk already
done.

Two arithmetics run: FORWARD (every derived site is accounted for) catches an obligation
you missed, and REVERSE (every gated requirement is some site's target, or is listed in a
section's `unsourced-ids`) catches one you invented. Contract:
`rfc/extraction/README.md`.

For a stem enrolled since HEAD the sign-off is a PRECONDITION of enrolment
(`check_enrolment`). RFCs enrolled before the gate existed are grandfathered and published
as a counted backlog in `ai/RFC-REQUIREMENTS.md`; `./le rfc extraction-status` emits the
same counts as JSON.

This replaces a check that had caught real failures and would have caught more: `rfc5303`,
`rfc5304` and `rfc5310` shipped summaries with 23, 13 and 12 normative keywords in the
source and **zero** captured. A ratio you eyeball says "roughly enough"; a classified site
list says which sentence, by name.

## Keep the ledger committed (BLOCKING)

`./le rfc index-update` writes two outputs from the summaries and the `RFC requirement:` tags.
One is `ai/RFC-REQUIREMENTS.md`, the index of counts, coverage rollup, audit coverage,
extraction sign-off and backlog. The other is one file per RFC under `rfc/requirements/`,
holding that RFC's requirement rows.

The per-RFC file records each enforcing test's `file:line`. It goes stale when you add or
retire a requirement. It also goes stale whenever a tagged test is added, moved, deleted or
re-tagged. An unrelated edit that shifts a tagged test's line stales it too.

Regenerate with `./le rfc index-update`. **Commit BOTH outputs in the SAME commit** as the
change that caused the drift: the index, and every changed file under `rfc/requirements/`.
Commit the index alone and the gate is red for the next session.
`./le rfc check` renders both and fails on any mismatch, and it
runs in both `./le verify current mode full` and `./le verify current mode changed` (`check_ledger_fresh`,
`internal/le/rfc/rfc.go`). A ledger left stale is not silently tolerated: it
surfaces later as a cross-commit diff that the next session inherits and the freshness gate
pins on them. This cuts both ways: it is also why you must not regenerate the ledger as a
side effect of unrelated work. If the diff is a pure line-number refresh with no change of
yours behind it, a prior commit skipped the regen, so commit that refresh on its own.

## Structure

### Meta
- RFC title, number, status, date
- Obsoletes / Updates / Obsoleted-by
- `Enrolment` and `Enrolment reason`, and the `Support` rows (step 6b). The parser
  refuses the summary without them, so a new file cannot omit them
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
  `rfc/requirements/<stem>.md` — never tick a box to claim a requirement is met.
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
| The id's section MUST match the line's `(§N)` | Checked by `./le rfc check` — the id and the citation can never drift apart |
| Ordinals are per-section, starting at 1 | Adding to §5.3 only has to clear §5.3's high-water, so it lands beside its siblings |
| **Never renumber. Never reuse.** A new requirement takes the next free ordinal IN ITS SECTION | A renumber silently re-points every test tag at the wrong requirement |
| When REMOVING a requirement, retire the ordinal — do not backfill the hole | Reuse makes an old tag point at a new, unrelated obligation |
| A line citing no section of its own anchors to `x` (`RFC1071-x-1`) | Conspicuous on purpose: a missing section reference is a defect to fix, not a resting state |
| A cite naming ANOTHER RFC (`(RFC 2328 §A.3.1)`) is NOT an anchor | That is RFC 2328's section; hanging this RFC's id off it names the wrong document |

If a requirement's TEXT needs correcting (a misquote), edit the text and KEEP the ID —
then re-run `/ze-rfc-audit <rfc>`, because changing requirement text re-stales every
verdict bound to it, by design. If the SECTION was wrong, the id changes with it; that is
safe only while no test tags it (a dangling tag fails the gate, which is the backstop).

## Correcting a LEVEL (BLOCKING)

Lowering a row's level out of the MUST-level set (`MUST`, `MUST NOT`, `SHALL`,
`SHALL NOT`, `REQUIRED`) removes every coverage obligation attached to it, and the row
keeps its id and its tests, so no other ratchet can see it happen. `check_level_ratchet`
refuses it unless the same summary carries the authorisation, as a paragraph:

```
Correction 2026-08-15: `RFC7296-2.8-1` was extracted at MUST strength. §2.8.1 states the
collision rule as a recommendation: the redundant SA "SHOULD be closed by the endpoint
that created it". Same requirement id, corrected text and level.
```

| Part | Requirement |
|------|-------------|
| Opener | `Correction <YYYY-MM-DD>:`, first line of the paragraph. A leading `>` is allowed |
| The row | The id in backticks. A paragraph naming a neighbour does not authorise this row |
| The proof | At least 24 characters in double quotes, appearing VERBATIM in `rfc/full/<stem>.txt` or `rfc/drafts/<stem>.txt`. Line wrapping is ignored; the words are compared |

A correction is for a row that MISQUOTED the RFC. When the document really does say MUST,
restore the level instead — and see `ai/rules/rfc-compliance.md`, "Implement Full
Compliance", before writing anything that lowers what Ze owes. Raising a level to a gated
one needs no record: the gate never charges for a conformance improvement.

## Annotations (when no test is expected)

A MUST-level requirement in an **enrolled** RFC must be covered by tests, or carry exactly
one annotation explaining why not. Every annotation needs a reason — a bare annotation is
rejected by `./le rfc check`.

```
- [ ] [RFC5303-3-1] [MUST] <text> (§3) {not-applicable: Ze does not implement IS-IS mesh groups}
- [ ] [RFC7606-5.1-1] [MUST] <text> (§5.1) {gap: Ze emits MP_UNREACH first, MP_REACH last; docs/architecture/wire/mp-nlri-ordering.md}
- [ ] [RFC7606-4-1] [MUST] <text> (§4) {single-polarity: negative; no conforming input exists to assert positively}
```

| Annotation | Meaning | Gate behavior |
|------------|---------|---------------|
| `{not-applicable: why}` | Not applicable to Ze (protocol not implemented, role not played) | Passes. MUST NOT coexist with any test tag |
| `{gap: why; ref}` | Known, deliberate divergence | Passes, but MUST be disclosed in that RFC's public row, which is declared by the `Support status` and `Support remaining` rows of its own `## Meta` table. Not a place to hide an accident |
| `{single-polarity: positive\|negative; why}` | Genuinely testable only one way | Requires tags of that polarity only, instead of the usual pair |

**These are not escape hatches.** If a requirement has no test because nobody wrote one
yet, that is not `not-applicable` — write the test, or leave the RFC un-enrolled. Both
`{not-applicable}` and `{gap}` FAIL the gate the moment a test tags the requirement
(a stale annotation is a lie the ratchet catches).

**Writing any of the three is Thomas's call, not yours** (owner directive 2026-07-27,
`ai/rules/rfc-compliance.md` "Implement Full Compliance. Ask Thomas Only Before Doing LESS").
Implementing the requirement fully and proving it with a tagged test is always an
available answer, and when it is reachable you take it WITHOUT asking. Choosing an
annotation instead is choosing less: stop, quote the requirement text and the producing
code `file:line`, and ask which way he wants it fixed.
An annotation you find already in place from an earlier session is VOID as authority —
re-derive it from the RFC text, and ask again if it still reads as less than full.

## Superseded Documents Carry Their Successor

When the summary's forward Meta row names a successor, EVERY requirement line needs a
second marker saying where that obligation now lives. `check_superseded` refuses the
summary otherwise, naming the id and the obsoleting RFC.

Write the Meta row as a chain, oldest first: the LAST RFC it names is the document
that states these obligations today. `rfc/short/rfc3768.md` reads
`RFC 5798, which was in turn obsoleted by RFC 9568 (both VRRPv3)`, and the successor
is RFC 9568. A row that says `None`, `-`, `n/a` or `(none)` creates no obligation at
all.

Write the label as `| Obsoleted by |` or `| Obsoleted-by |`, in either capitalisation,
with a qualifier after it if the document needs one: `rfc/short/rfc1334.md` writes
`| Obsoleted-by (partial) |` because RFC 1994 replaced its CHAP half and left PAP here.
`| Obsoletes |` is the backward direction and is allowed alongside it; 119 rows
carry one. Any OTHER field name containing `obsolet` stops the run with exit 2 and
names the field. That refusal exists because the reader used to know one spelling:
the corpus wrote four, and 93 requirements of three enrolled summaries were gated as
current documents until the label was widened.

It refuses on that WORD alone. A field named `Superseded by`, `Replaced by` or
`Successor` is skipped in silence, so do not reach for one: no summary writes such a
spelling today, and widening the word waits on a survey of what the corpus writes,
because the field-name pattern reads the first cell of every table row and a looser
word would collide with the requirement tables.

```
- [ ] [RFC3768-5.2.3-1] [MUST] <text> (§5.2.3) {superseded: restated RFC9568-5.1.1.3-1; RFC 9568 §5.1.1.3 keeps the IPv4 TTL of 255}
- [ ] [RFC3768-9.2-1] [MUST] <text> (§9.2) {superseded: dropped; RFC 9568 §1.2 change 6 removed the token ring appendix}
- [ ] [RFC7752-6.1.2-1] [SHOULD] <text> (§6.1.2) {superseded: unextracted §8.1.2; rfc/short/rfc9552.md declares no row for it}
- [ ] [RFC7627-5.2-1] [MUST] <text> (§5.2) {superseded: unresolved; rfc/full/rfc9846.txt is not in this repository}
```

| Disposition | Says | The gate checks |
|-------------|------|-----------------|
| `restated <ID>; why` | the successor states the same obligation, under that id | the successor's summary declares that id |
| `dropped; why` | the successor states no equivalent obligation | the successor's text is under `rfc/full/` or `rfc/drafts/` |
| `unextracted <§section>; why` | the successor states it there, and its summary has no row | the successor's text is under `rfc/full/` or `rfc/drafts/` |
| `unresolved; why` | the successor's text is not in this repository | that text is ABSENT |

**This marker is a fact about the DOCUMENT, never about coverage.** It composes with
`{not-applicable}`, `{gap}` and `{single-polarity}` instead of replacing one. It lowers
nothing. A marked requirement stays gated, stays counted and stays ratcheted.
Writing one is therefore NOT the owner-reserved judgement the three annotations above
are. It records where the IETF put the obligation, and it says nothing about what Ze
owes.

The last two dispositions are DEBT, and `ai/RFC-REQUIREMENTS.md` publishes them as
debt. Draining one is separate work: fetch and summarise the successor, or extract the
rows its summary is missing.

## Linking Requirements to Tests

The link is two-way, but only ONE side is authored: the test tags itself.
`rfc/requirements/<stem>.md` renders the reverse direction and is GENERATED — never hand-write
a test path into a summary (`ai/rules/evidence.md`). A hand-written back-link
survives deletion of the test it names; a tag dies with the test.

The rendered link names the file and the enclosing function, never a line, so an edit
above a tag does not churn the ledger. It still drifts when a tagged test is added, moved
between functions or deleted. Regenerate and commit it in the same change (see "Keep the
ledger committed").

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
- `.ci` tags must start at the line start (`internal/test/runner/parsing.go`) and must
  not sit inside a `terminator=` block, where `#` is raw file content, not a comment.

### The proof a new tag owes (BLOCKING)

The prose after the polarity is the CLAIM, and no gate can read a sentence. So a tag you
ADD owes, in the same change, a record of a break under which its own tagged unit goes
RED. `./le rfc check` refuses a tagged unit that is new against git HEAD and carries none.

| Step | Command | What it does |
|------|---------|--------------|
| 1 | `./le rfc discriminate stem <stem>` | says which of that RFC's tags carry a record, which records no longer verify, and which tags carry none |
| 2 | `./le rfc discriminate stem <stem> report <path>` | reads a gomu JSON report and proposes candidate breaks for the unit tags, best first |
| 3 | `./le rfc discriminate-record ...` | runs that tagged unit alone and requires the GREEN first, applies one break, requires the RED, and writes `rfc/discrimination/<stem>.json` |

The green comes BEFORE the break, not after it: a unit already failing proves nothing
about a break, and there is no post-restore run on any route. A unit or `.ci` break is a
Go overlay, so no file on disk is touched and there is nothing to restore. An interop
break edits the working tree, because the lab compiles ze inside Docker from the
repository as the build context, and that one is put back and re-read byte for byte.

`discriminate-record` refuses to write a red it did not observe, and refuses a red that
does not NAME the tagged unit: a build error and a failing sibling test each turn a run
red and neither proves the claim. A `.ci` or an interop tag takes the `revert` route and
cites the assertion it rests on, because no generated mutant reaches either carrier.

The `no-break` escape is the third route and it is not a way out. Its reason comes from a
closed vocabulary, the gate checks the fact each reason claims, and the record must also
tie that fact to THIS tag: `declaration-only` and `generated-producer` need the tag's own
claim to name something their producer file declares, and `foreign-producer` needs the
`fail(N, ...)` number its checker writes out. It is refused outright for a unit-tier tag
whose producer gomu can mutate.

Write the claim to match what the body checks. A claim wider than the assertion is what
this record exists to refuse, and rewording a claim stales its record and owes a fresh
one. Full contract: `docs/contributing/rfc-conformance-gates.md`, "The discrimination
record", and `rfc/discrimination/README.md`.

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

A row in `test/weakened.md` does **not** authorize changing a tagged test. It is
self-service: you would be writing your own approval. Only the user can approve.

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
- Never hand-write a test path into a summary — `rfc/requirements/<stem>.md` is generated
- Run `./le rfc index-update` and commit BOTH its outputs in the same change whenever a tagged
  test is added, moved, deleted, or re-tagged: `ai/RFC-REQUIREMENTS.md` and every changed
  file under `rfc/requirements/`. The per-RFC file records `file:line`, and `./le rfc check`
  fails on a stale index and on a stale per-RFC file
- Never annotate `{not-applicable}` / `{gap}` to reach green. Write the test, or leave
  the RFC un-enrolled and say so
- Enrolment (`| Enrolment | enrolled |` in the summary's `## Meta` table) means "every
  MUST here is CLASSIFIED": tested, or annotated with a reason that names what is
  missing. A NEW summary declaring gated MUSTs must be enrolled in the same change;
  only the pre-existing backlog is grandfathered
- Coverage is monotonic. A requirement that had a positive and a negative test cannot be
  demoted to `{gap}` later: `check_coverage_ratchet` compares against HEAD and an
  annotation does not substitute for proof that already existed
- A requirement id of an enrolled RFC cannot be deleted from its summary
  (`check_retired_requirements`). Fix a misquoted requirement by editing its TEXT under the
  same id; deleting the line retires the obligation silently

## Related

| Need | Use |
|------|-----|
| Check requirements are covered | `./le rfc check` |
| Regenerate the requirement ledger | `./le rfc index-update` → `ai/RFC-REQUIREMENTS.md` and `rfc/requirements/` |
| Read one RFC's requirement → test rows | Read `rfc/requirements/<stem>.md` after `./le rfc index-update` |
| Re-audit that tests still enforce letter and spirit | `/ze-rfc-audit <rfc>` |
| Public per-RFC support status | Declare it in the summary's `## Meta` `Support` rows. `./le rfc index-update` renders `docs/features/rfc-status.md` from them, and it must agree with the `{gap}` annotations |
