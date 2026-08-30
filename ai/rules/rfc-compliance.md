# RFC Compliance (every protocol, not just BGP)

**When:** writing, changing, reviewing, or testing ANY protocol-implementing code, for ANY RFC Ze implements
**Severity:** blocking

## Directives

**Ze aims to be a model of RFC compliance, for EVERY RFC it implements.** Not
just BGP: IS-IS, OSPF, BFD, LDP, RSVP-TE, IKE/IPsec, L2TP, PPPoE, DHCP, NTP,
RADIUS, TACACS+, gNMI, BMP, RPKI, VRRP -- every protocol surface MUST be held to its
own RFCs, and so MUST anything Ze speaks that has a standard behind it.

**Conformance is a property of the code, checked against the RFC text. It MUST NOT be traded away for convenience, for a green test, or for expedience.** You cannot write an RFC-based application and not ensure RFC compliance.

**Conformance is not negotiable and nothing in the repo overrides the RFC: a deviation MUST NOT be made without an explicit instruction from Thomas, given in answer to the question that "Implement Full Compliance. Ask Thomas Only Before Doing LESS" (below) requires you to put to him. That question is owed only when you are about to do less than the RFC asks. Full compliance needs no question.**

**When Thomas authorises a deviation, it MUST be recorded as a row in `plan/journal/<class>.md` with the RFC section and the reason**, so the next reader finds a decision rather than a bug.

| Situation | What you MUST do |
|-----------|------------------|
| Full conformance and full proof of it are reachable | Implement it and prove it with a tagged test. Do not ask which subset Thomas wants |
| Anything short of full conformance or full proof of conformance looks like the answer | You are not authorized to pick it. STOP and ask Thomas -- see "Implement Full Compliance. Ask Thomas Only Before Doing LESS" below |
| You find code that does not do what the RFC requires, and the goal of the work in hand depends on that path | Fix it here. A known wire-visible violation the work depends on is a defect you are now the entry point for (`ai/rules/completion.md`) |
| You find code that does not do what the RFC requires, and the goal of the work in hand does not depend on it | Write the spec, close the work in hand, and put the violation to Thomas the moment it closes, quoting the requirement id and the RFC section text (`ai/rules/completion.md`). The ask is owed the same session; the fix waits for his answer. Leaving it as a comment, a report line, or an unhomed row is banned |
| A test pins the non-conformant behaviour | The TEST is wrong. A fixture, golden file, or assertion encoding a violation is not evidence the violation is intended -- it is the violation with a green bar on top. Fix the code, then correct the test and say so |
| A code comment calls the deviation deliberate | A comment is its author's belief, not a decision record (`ai/rules/evidence.md`). Check the RFC text, then `plan/learned/` for a real ruling. Absent one, the RFC wins |
| The RFC requirement is not in `rfc/short/<stem>.md` | An unextracted obligation is still an obligation. Add the checklist row (see Extraction Completeness) -- the gate's silence is not conformance |
| Conforming would change behaviour operators rely on | Say so plainly and ask which way to fix it. Never silently keep the violation, and never present "leave it non-conformant" as an option |
| An exemption genuinely applies (e.g. RFC 7947 route-server transparency) | Gate it on the exact condition the exempting RFC names. An exemption applied unconditionally is a violation for every case it was not written for |

**Before claiming a protocol behaviour is correct, the RFC text MUST be read**, not only the summary and not only the surrounding code. The section relied on MUST be cited.

**A claim that Ze violates an RFC MUST quote the RFC's own text before it is
recorded or acted on.** The quotation comes from `rfc/full/<stem>.txt` or
`rfc/drafts/`, is verbatim, is at least one whole sentence, and carries the
section number it was read at. A finding that cites only a requirement id, only
a `rfc/short/` line, or only its own paraphrase is UNVERIFIED and MUST be
labelled so, whatever else it carries.

**A `rfc/short/` summary is not the source.** It is a derived artifact and it is
the thing under audit, so it can be wrong in the two directions that matter: it
can state an obligation the RFC does not, and it can drop a clause that changes
the obligation's scope. A finding built on the summary alone inherits its error
and gives it the authority of a review.

This binds every place a violation is asserted: a review finding, a spec
premise, a journal row, an audit verdict, a `{gap}` or `{not-applicable}`
annotation, a report to the owner, and a message to another session. It binds
the reader too: a violation claim arriving without its quotation MUST be
re-derived from the RFC text before any work is commissioned on it.

**The failure this prevents is fabrication, not sloppiness.** A requirement id
and a section number are cheap to produce from memory and read as evidence, so
an id that names no such clause, a section number off by one, and a MUST
remembered as stricter than it is are all invisible at review time. Opening the
text costs one read and is the only thing that separates a finding from a
recollection.

**An RFC that the current one OBSOLETES MUST NOT be read as evidence about what Ze
owes.** The current document is the baseline, in full. Whether a clause was
inherited or newly written changes no obligation, and no implementation in service
began life against the obsoleted text, so the lineage predicts nothing about
interoperability either. A predecessor MUST NOT be downloaded into `rfc/full/`,
cited in a summary under `rfc/short/`, or named in Ze's own prose.

**The lineage that matters runs FORWARD: the documents that UPDATE the current one,
and its errata.** Those change the text in force. A clause with no updater and no
erratum stands as published, and saying so IS the answer to "has this MUST
changed since".

**A predecessor MAY be opened for one purpose alone: to see why a clause is worded
as it is, when the current text is genuinely ambiguous.** Even then it settles
nothing and MUST NOT appear in a conformance verdict.

**An obsoleted RFC quoted by a document Ze summarises stays as it is.** A
reference list transcribed from an external RFC is that RFC's text, and
`ai/rules/writing.md` never edits quoted external text.

## Implement Full Compliance. Ask Thomas Only Before Doing LESS (owner directive, 2026-07-27, clarified 2026-08-01)

**When "implement the RFC fully and prove it fully with tests" is one of the answers on the table, that IS the answer. It MUST be implemented and proven. Thomas has already chosen, so there is nothing to put to him.**

**Asking MUST happen only when you are about to do LESS.** Making Ze more conformant, or better proven, never needs permission: it MUST be done, then reported (`ai/rules/completion.md` still governs everything else). The gate exists in one direction only.

**Two readings, and the one that governs.** "Full compliance is on the table" MUST be treated as a trigger to IMPLEMENT. It MUST NOT be treated as a trigger to ask. The question is owed only when you are about to choose something NARROWER than full implementation plus a tagged test, and then it is "which way do I fix it", never "MAY I do less". **Full compliance MUST NOT be put beside a narrower option when asking Thomas to pick between them.**

| You are about to ... | Do instead |
|----------------------|------------|
| Classify a requirement `{gap}`, `{not-applicable}`, `partial`, or "does not apply to ze" | Ask. A classification that lowers what Ze owes is a decision about compliance, not bookkeeping |
| Leave a MUST implemented but unproven (no `RFC requirement:` tagged test) | Ask. "Implemented" is a claim; the tagged test is the evidence (`ai/rules/testing.md`, `ai/rules/testing.md`) |
| Leave a MUST unextracted, or scope a spec so an RFC obligation falls outside it | Ask. See Extraction Completeness -- the gate cannot see an obligation nobody wrote down |
| Defer an RFC requirement to a follow-up spec, a deferral row, or a known-failure shard | Ask. Recording is not fixing (`ai/rules/completion.md`), and the deferral machinery is not a compliance decision procedure. The spec-close-ask route in the conformance table above IS that ask, made the same session. The deferral row is never a substitute for it |
| Lower a requirement's level in `rfc/short/`, so the row stops being MUST-level | Read the RFC sentence first. It is a CORRECTION only when the document states the lower strength, and then it MUST be recorded as a `Correction <YYYY-MM-DD>:` paragraph quoting that sentence, which `check_level_ratchet` refuses to do without. Anything else lowers what Ze owes: ask |
| Close a spec, review, or audit whose RFC rows are anything other than implemented-and-proven | Ask before closing, not after |
| Answer "is this conformant enough" with anything but yes | Ask. "Enough" is Thomas's word to say, never yours |

**How to ask (never "MAY I skip it").** The requirement id and the RFC section text MUST be quoted verbatim, the producing function MUST be named (`ai/rules/evidence.md`), what full implementation plus a tagged test would actually cost MUST be stated, and then which way he wants it fixed MUST be asked. Offering "leave it non-conformant" as an option MUST NOT be done (`ai/rules/completion.md`).

**Every earlier answer that pointed away from full compliance or full proof MUST be treated as VOID.** Thomas voided them on 2026-07-27. A prior decision to skip, defer, partially implement, or leave a requirement untested is not authority, MUST NOT be cited as one, and does not survive being rediscovered.

**A void answer MUST be re-derived rather than relied on, wherever you meet it:**

| Where a void answer hides | What to do when you meet one |
|---------------------------|------------------------------|
| A `plan/learned/` deviation record, or a spec `Deviations` row | Do not rely on it. Raise the requirement with Thomas again, then correct the record with the new answer |
| A `{gap}`, `{not-applicable}` or `partial` in `rfc/short/*.md` or `docs/features/rfc-status.md` | Re-derive it from the RFC text. If it still reads as less than full compliance, ask |
| A deferral row marked `user-approved-drop`, or a `cancelled` status, covering an RFC obligation | Void. Re-raise it; the row is not a close |
| A code comment or `rfc/audit/*.json` verdict calling the deviation deliberate | A comment is a belief, not a ruling (`ai/rules/evidence.md`). Void by default; ask |

**Finding a void answer while doing something else is not permission to move on.** It MUST be raised, and the fresh answer MUST be recorded where the stale one lived, so the next reader inherits a decision rather than a rationalization.

## RFC Summaries (`rfc/short/`)

**A summary's PROSE is a protocol-only reference document: it MUST NOT contain Ze-specific information -- no Ze implementation notes, no Ze file paths, no "Ze does/does not" statements, no "for ze" sections.** Implementation decisions belong in specs (`plan/`), architecture docs (`docs/architecture/`), or code comments. A reader SHOULD be able to read any `rfc/short/` file's prose as a standalone protocol reference with no knowledge of Ze.

**The `{...}` annotation on a requirement line is a SEPARATE register, and the constraint above does not reach it.** An annotation exists to say why this codebase owes less than the literal requirement, and `ai/rules/evidence.md` requires that justification to name the producing function. So a `{gap: ...}`, `{not-applicable: ...}`, `{partial: ...}` or `{single-polarity: ...}` MUST name the code it judges, file and symbol, and a search it relied on MUST be recorded the same way. An annotation that names no code is an assertion, and the two registers MUST NOT be traded against each other: stripping a path out of an annotation to satisfy the prose rule destroys the evidence the annotation exists to carry.

**The boundary is the line, not the file.** A requirement line and its wrapped continuation are annotation; every other line is prose. A Ze path that has escaped into a bullet, a heading or a paragraph is what this rule was written for, and it stays forbidden.

**A summary whose forward Meta row names a successor MUST carry a `{superseded: ...}` marker on EVERY requirement line it declares.** `checkSuperseded` (`internal/le/rfc/check_core.go`) refuses the summary otherwise. The accepted labels, the four dispositions and the precondition each one carries are in `docs/contributing/rfc-conformance-gates.md`.
**The marker states where the obligation NOW LIVES. It MUST NOT be read, or written, as saying Ze owes less.** It is a fact about the DOCUMENT, so it composes with `{gap}`, `{not-applicable}` and `{single-polarity}` rather than replacing one. A marked requirement stays gated, stays counted, and stays judged by every ratchet.
**A `dropped` obligation is still owed for as long as Ze speaks the wire format the obsoleted document defines.** RFC 3768 is the VRRPv2 format keepalived speaks by default. RFC 9568 removing an obligation says what VRRPv3 requires, and nothing about what a VRRPv2 speaker owes on the wire.
**An `unresolved` or `unextracted` disposition is DEBT. Marking a line MUST NOT be treated as closing it.** Draining either is separable work with its own spec.

## Extraction Completeness (BLOCKING when enrolling a summary)

**Before enrolling `rfc/short/<stem>.md` in `rfc/enrolled.txt`, you MUST walk the RFC's own text section by section and confirm every MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED has a checklist row.** A green gate is bounded by what was extracted, so an obligation nobody wrote down is invisible to it and to any audit that only re-checks classifications.
**When `rfc/full/` lacks the source, you MUST fetch it first: `curl -o rfc/full/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`.** A claim of "verified against the RFC" is not reproducible without it.

**The walk MUST be RECORDED, not asserted.** Its record is `rfc/extraction/<stem>.json`, a sign-off artifact a machine re-checks, and it is a precondition of a new enrolment. The steps, the contract, and the five properties of the artifact are in `docs/contributing/rfc-conformance-gates.md`.

**Summaries enrolled before the gate existed are grandfathered and published as a counted backlog. Grandfathering MUST be implemented as SCOPE (new-since-HEAD); it MUST NOT be implemented as an allowlist file**, so nothing is added to a list of exceptions when an RFC stops being one.

**The requirement TEXT MUST be verified against the RFC, not only its presence.** A misquoted obligation licenses a justification that never engages it: RFC 4271 §5.1.6 binds a speaker THAT RECEIVES a route with ATOMIC_AGGREGATE, and recording it as an aggregator rule let the readvertisement path be cited as evidence of non-applicability when it is the bound path.

## What Keeps RFC Testing Valid (the eight ratchets)

**`./le rfc check` reads the WORKING TREE, and eight comparisons against HEAD supply what a tree cannot tell: "never proven" from "stopped being proven". Each fires only on a real downgrade, so you MUST treat a green run as evidence the proof held rather than as evidence nobody looked.**
**A red ratchet names a DOWNGRADE you made. You MUST restore the evidence it names; you MUST NOT reach for the annotation, the level change, or the deleted row that would make it green.** Each ratchet, what fires it, and the one documented escape are in `docs/contributing/rfc-conformance-gates.md`.

**A tagged test's assertions MUST NOT be weakened IN PLACE while the shape stays the same.** No ratchet catches that. The write-time guard, the commit audit and the audit-freshness SHA ratchet do, and `docs/contributing/rfc-conformance-gates.md` says which does what.
**A `test/weakened.md` row is self-service and MUST NOT be treated as authorizing the weakening of an RFC-tagged test.**

## Before Implementing BGP Features

1. Find RFC in `rfc/` — if missing: `curl -o rfc/full/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`
2. Read relevant sections, note MUST/SHOULD/MAY
3. Check ExaBGP reference

**Priority:** the RFC MUST outrank ExaBGP API compat, which MUST outrank the ExaBGP implementation.

## Wire Format Documentation (MANDATORY)

**Protocol code MUST NOT be modified without documenting the wire format: an ASCII diagram with field offsets, byte offset annotations, and the RFC section reference.**

## RFC MUST Comments (BLOCKING)

**Every MUST and MUST NOT enforced in code MUST carry a comment directly above it, naming the RFC section and quoting the requirement:**

```
// RFC NNNN Section X.Y: "quoted requirement"
<enforcing code>
```

**The comment MUST document whichever of these the code enforces: validation rules, error conditions, state transitions, timer constraints, and message ordering.**

## MAY Clauses

**A MAY clause MUST be put to the user: implement it, skip it, or make it a config option.** You are not authorized to pick.

## Common RFCs

**A change to one of these BGP features MUST be read against the RFC in its row, at the code its row names:**

| Feature | RFC | Location |
|---------|-----|----------|
| BGP-4 base | 4271 | `internal/component/bgp/message/`, `internal/component/bgp/reactor/` |
| MP-BGP | 4760 | `internal/component/bgp/reactor/received_update.go`, `internal/core/bgp/attribute/` |
| 4-byte ASN | 6793 | `internal/core/bgp/capability/capability.go` |
| Add-Path | 7911 | `internal/core/bgp/capability/capability.go` |
| GR | 4724 | `internal/core/bgp/capability/capability.go` |
| Revised error handling | 7606 | `internal/component/bgp/reactor/received_update.go` |
