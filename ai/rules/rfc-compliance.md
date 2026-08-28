# RFC Compliance (every protocol, not just BGP)

**When:** writing, changing, reviewing, or testing ANY protocol-implementing code, for ANY RFC Ze implements
**Severity:** blocking

## Directives

**Ze aims to be a model of RFC compliance, for EVERY RFC it implements.** Not
just BGP: IS-IS, OSPF, BFD, LDP, RSVP-TE, IKE/IPsec, L2TP, PPPoE, DHCP, NTP,
RADIUS, TACACS+, gNMI, BMP, RPKI, VRRP -- every protocol surface MUST be held to its
own RFCs, and so MUST anything Ze speaks that has a standard behind it.

You cannot write an RFC-based application and not ensure RFC compliance.
Conformance is a property of the code, checked against the RFC text, and it is
never traded away for convenience, for a green test, or for expedience.

**Conformance is not negotiable and nothing in the repo overrides the RFC: a deviation MUST NOT be made without an explicit instruction from Thomas, given in answer to the question that "Implement Full Compliance. Ask Thomas Only Before Doing LESS" (below) requires you to put to him. That question is owed only when you are about to do less than the RFC asks. Full compliance needs no question.**

When he does authorise one, record it as a row in `plan/journal/<class>.md` with
the RFC section and the reason, so the next reader finds a decision rather than a
bug.

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

Rationale: `ai/rationale/rfc-compliance.md`

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

| Where a void answer hides | What to do when you meet one |
|---------------------------|------------------------------|
| A `plan/learned/` deviation record, or a spec `Deviations` row | Do not rely on it. Raise the requirement with Thomas again, then correct the record with the new answer |
| A `{gap}` / `{not-applicable}` / `partial` in `rfc/short/*.md` or `docs/features/rfc-status.md` | Re-derive it from the RFC text. If it still reads as less than full compliance, ask |
| A deferral row marked `user-approved-drop`, or a `cancelled` status, covering an RFC obligation | Void. Re-raise it; the row is not a close |
| A code comment or `rfc/audit/*.json` verdict calling the deviation deliberate | A comment is a belief, not a ruling (`ai/rules/evidence.md`). Void by default; ask |

**Finding a void answer while doing something else is not permission to move on.** It MUST be raised, and the fresh answer MUST be recorded where the stale one lived, so the next reader inherits a decision rather than a rationalization.

## RFC Summaries (`rfc/short/`)

**A summary's PROSE is a protocol-only reference document: it MUST NOT contain Ze-specific information -- no Ze implementation notes, no Ze file paths, no "Ze does/does not" statements, no "for ze" sections.** Implementation decisions belong in specs (`plan/`), architecture docs (`docs/architecture/`), or code comments. A reader SHOULD be able to read any `rfc/short/` file's prose as a standalone protocol reference with no knowledge of Ze.

**The `{...}` annotation on a requirement line is a SEPARATE register, and the constraint above does not reach it.** An annotation exists to say why this codebase owes less than the literal requirement, and `ai/rules/evidence.md` requires that justification to name the producing function. So a `{gap: ...}`, `{not-applicable: ...}`, `{partial: ...}` or `{single-polarity: ...}` MUST name the code it judges, file and symbol, and a search it relied on MUST be recorded the same way. An annotation that names no code is an assertion, and the two registers MUST NOT be traded against each other: stripping a path out of an annotation to satisfy the prose rule destroys the evidence the annotation exists to carry.

**The boundary is the line, not the file.** A requirement line and its wrapped continuation are annotation; every other line is prose. A Ze path that has escaped into a bullet, a heading or a paragraph is what this rule was written for, and it stays forbidden.

**A summary whose forward Meta row names a successor MUST carry a
`{superseded: ...}` marker on EVERY requirement line it declares.**
`check_superseded` (`internal/le/rfc/rfc.go`) refuses the summary
otherwise. The row used to be prose nobody read. Seven summaries declare
themselves obsoleted, six of them enrolled, and the gate treated all seven as
current documents. A reader who opened one of their 230 requirement lines saw a
MUST with no sign that the document stating it had been replaced.

**The label MUST be `Obsoleted by` or `Obsoleted-by`, in either capitalisation,
and any OTHER Meta field whose name CONTAINS `obsolet` MUST red the gate rather
than be skipped.** That word is the whole of what the reader recognises: a field
named `Superseded by`, `Replaced by` or `Successor` is skipped in silence today,
and widening the word list is separate work because `_META_FIELD_RE` reads the
first cell of EVERY table row, so a looser word collides with the requirement
tables themselves. No summary uses such a spelling today, so the gap is
prospective; it is stated here rather than left for the next reader to discover
the way the hyphen was.
`parse_successor_stem` reads all four spellings and refuses a fifth,
because reading one of them is how this failed the first time: the corpus's
MAJORITY spelling is the hyphenated one, 28 rows against 18, and a reader that
knew only `Obsoleted by` gave 93 requirements of three enrolled summaries no
obligation at all. A qualifier after the label is kept, which is how
`rfc/short/rfc1334.md` writes `| Obsoleted-by (partial) |` for a document whose
CHAP half moved to RFC 1994 and whose PAP half did not. A reader that SKIPS what
it does not recognise cannot be trusted to have found anything.

**The marker states where the obligation NOW LIVES. It MUST NOT be read, or
written, as saying Ze owes less.** It is a fact about the DOCUMENT. So it
composes with `{gap}`, `{not-applicable}` and `{single-polarity}` rather than
replacing one. A marked requirement stays gated, stays counted in
`ai/RFC-REQUIREMENTS.md`, and stays judged by every ratchet.

| Disposition | Says | Precondition the gate checks |
|-------------|------|------------------------------|
| `restated <ID>; why` | the successor states the same obligation, under that id | the successor's summary is in `rfc/short/` AND declares that id |
| `dropped; why` | the successor states no equivalent obligation | the successor's own text is in `rfc/full/` or `rfc/drafts/` |
| `unextracted <§section>; why` | the successor STATES it, at that section, and its summary declares no row | the successor's own text is in `rfc/full/` or `rfc/drafts/` |
| `unresolved; why` | the successor's text is not in this repository | that text is ABSENT |

**A `dropped` obligation is still owed for as long as Ze speaks the wire format
the obsoleted document defines.** RFC 3768 is the VRRPv2 format keepalived speaks
by default. RFC 9568 removing an obligation says what VRRPv3 requires. It says
nothing about what a VRRPv2 speaker owes on the wire.

**The last two dispositions are DEBT, and the ledger publishes them as debt.**
Draining either one is separable work with its own spec. An `unresolved` line is
drained when somebody fetches and summarises the successor. An `unextracted` line
is drained by an extraction pass over the successor's summary. Marking a line
MUST NOT be treated as closing it.

## Extraction Completeness (BLOCKING when enrolling a summary)

`./le rfc check` verifies that every requirement **listed** in a summary is
covered. It cannot know about an obligation nobody wrote down. A green gate is
bounded by what was extracted, so a missing extraction is invisible to it and to
any audit that only re-checks classifications.

Before enrolling `rfc/short/<stem>.md` in `rfc/enrolled.txt`, walk the RFC's own
text section by section and confirm every MUST / MUST NOT / SHALL / SHALL NOT /
REQUIRED has a checklist row. Fetch the source first if it is absent:
`curl -o rfc/full/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`. A
claim of "verified against the RFC" is not reproducible when `rfc/full/` lacks
the file.

**The walk MUST be RECORDED, not asserted.** Its record is a sign-off artifact a machine re-checks, `rfc/extraction/<stem>.json`, and it is a **precondition of a new enrolment** (`check_enrolment`).

| Step | Command / file |
|------|----------------|
| Write the unclassified skeleton | `./le rfc extraction-create STEM=<stem>` |
| Classify every derived site and section by hand | `rfc/extraction/<stem>.json` |
| Re-check the arithmetic | `./le rfc check` |
| Read the published backlog | `ai/RFC-REQUIREMENTS.md`, "Extraction sign-off" |
| Read the counts machine-readably | `./le rfc extraction-status` |

**The contract is `rfc/extraction/README.md`.** Five properties SHOULD be known before you meet one.

- **Only dispositions are authored.** Sites, sections, quotes, the register and every published count are DERIVED from the source text at check time. A hand-typed "sites seen" is a claim, and claims are what this removes.
- **A generated skeleton can never pass.** The writer emits only UNCLASSIFIED dispositions and an unclassified site fails the check, so mass-generating artifacts makes the gate redder rather than greener.
- **The register is derived and a stronger claim is refused.** `rfc2119`, `prose`, or `manual-walk`. A keyword-only check can be vacuously green when an RFC declares gated obligations without a capitalised MUST-level keyword site.
- **The bound is over keyword-visible sites, not over obligations.** Recall can be near zero for an indicative-prose section (RFC 4271 §8.2.2: 35168 characters, one capitalised keyword). `unsourced-ids` records an obligation the extractor cannot see. This raises a floor from zero; it does not reach a ceiling.
- **A FIRST sign-off is reviewed, not ratcheted.** `check_extraction_ratchet` compares a stem against its own HEAD row, so a stem signing off for the first time has no baseline and could exclude every site. The published per-RFC exclusion ratio is the control; read it before you approve one.

**Summaries enrolled before the gate existed are grandfathered and published as a counted backlog.** Grandfathering MUST be implemented as SCOPE (new-since-HEAD); it MUST NOT be implemented as an allowlist file, so nothing is added to a list of exceptions when an RFC stops being one.

Two signals that an extraction is missing, both seen in practice:

| Signal | Why it matters |
|--------|----------------|
| A `{not-applicable}` whose reason is "ze has no X producer at all" | That admission is often the violation of a separate MUST requiring X to exist. RFC 4271 §5.1.4's "MUST implement a mechanism ... that allows MULTI_EXIT_DISC to be removed" was unextracted, and two requirements cited its absence as their exemption. |
| A section whose siblings are enumerated but one clause is not | RFC 8666 §5's "MUST be ignored on reception" was omitted while §6, §7.1 and §7.2 each had it. An enumeration hole, not a style choice. |

Also verify the requirement TEXT matches the RFC. A misquoted obligation licenses
a justification that never engages it: RFC 4271 §5.1.6 binds a speaker **that
receives** a route with ATOMIC_AGGREGATE, and recording it as an aggregator rule
let the readvertisement path be cited as evidence of non-applicability when it is
the bound path.

## What Keeps RFC Testing Valid (the eight ratchets)

`./le rfc check` reads the WORKING TREE to judge coverage, and a tree cannot tell
"never proven" from "stopped being proven". Eight comparisons against HEAD supply that
difference. Each fires only on a real downgrade, so a green run means the evidence held,
not that nobody looked.

| Ratchet | Producer | Fires when |
|---------|----------|-----------|
| Enrolment is monotonic | `check_enrolment` | an RFC whose MUSTs were gated stops being gated |
| **Proof is monotonic** | `check_coverage_ratchet` | a requirement loses a polarity it had at HEAD. `{gap}` is NOT an escape: it is the move being blocked |
| **Gating is monotonic** | `check_level_ratchet` | a requirement leaves the MUST-level population: its level was gated at HEAD and is advisory now. That is the CHEAPEST route from red to green in this gate, cheaper than `{gap}` and cheaper than deleting the row, because the id survives and the tests survive while every coverage obligation attached to the row disappears. The one escape is a `Correction <YYYY-MM-DD>:` paragraph in the same summary, naming the id and quoting at least 24 characters of the RFC verbatim: a quotation of the document, never a free-text reason. A row GAINING a gated level is never reported |
| **Requirements do not vanish** | `check_retired_requirements` | a requirement id of an enrolled RFC disappears from its summary. Without this, deleting the checklist line is the CHEAPEST route from red to green, cheaper than `{gap}` which costs a public disclosure row, and the ratchet would be pressuring people to hide obligations rather than declare them. Correcting a misquote means editing the TEXT under the same id, which is allowed |
| **Adding an RFC adds checking** | `check_new_summaries` | a summary that is NEW since HEAD declares gated MUSTs and is not in `rfc/enrolled.txt`, fails to parse, or captures zero requirements while `rfc/full/<stem>.txt` has MUST-level keywords. A document's own RFC 2119 key-words paragraph does not count, and neither does its reference-list entry for RFC 2119 or RFC 8174. Both say where the words come from, and neither binds anybody. Counting the paragraph refused RFC 7454, a BCP whose body states no MUST |
| **Non-unit evidence is monotonic, per tier** | `check_evidence_ratchet` | a requirement loses an evidence KIND it had at HEAD -- its `.ci` becomes a unit test, or its verify-tier binding is swapped for a nightly-tier interop one. Keyed by `kind/tier`, so a substitution that leaves the tag COUNT unchanged still fires: a unit test proves the algorithm, only a running functional or interop test proves the daemon or a peer. No annotation satisfies it |
| **Extraction is monotonic** | `check_extraction_ratchet` | a stem that carried a sign-off at HEAD carries none now, or a signed stem's exclusion count RISES without a `resign-reason` and a bumped `signed-off` date. The first stops the bound being un-bound by deleting a file; the second stops the exclusion list becoming an escape hatch where every unmapped site is excluded with a shrug |
| **Public disclosure is monotonic** | `check_status_completeness` | an RFC enrolled since HEAD has no row in `docs/features/rfc-status.md`, or a row that existed at HEAD is gone while its RFC stays enrolled. Enrolment gates that RFC's MUSTs, so the public page must say the RFC exists. A deleted row retires the public claim and leaves the obligation. It is also the one edit that makes `check_status_agreement`'s missing-row branch fire later, on somebody else's unrelated commit |

Summaries that predate HEAD are the existing backlog and are deliberately grandfathered:
a rule that reds the gate on unrelated work gets removed rather than obeyed. Where git
cannot answer, every ratchet judges nothing rather than judging everything.

**Beside the seven, `check_drain_floor` compares the derived sign-off count against the drain policy in `rfc/drain-budget.txt` (a start date and a rate, and nothing else).** It is a schedule rather than a ratchet, and it ships INERT at rate 0: only the owner MAY arm it, with a one-line commit.

### The public ledger's edges (not ratchets, hard requirements)

`docs/features/rfc-status.md` is the PUBLIC claim. A `{gap}` annotation is the private
admission. `check_status_agreement` has always compared the two. But it reaches for a row
only when a `{gap}` exists, so three classes of defect sat outside it. Each is now a hard
requirement rather than a HEAD comparison, because each is clean on the tree today.

| Guard | Refuses |
|-------|---------|
| `check_summary_disposition` | a summary in `rfc/short/` that is in neither `rfc/enrolled.txt` nor `rfc/not-enrolled.txt`. Also a stem in BOTH, a disposition naming a summary that does not exist, and a disposition deleted while the stem never reaches `rfc/enrolled.txt`. Also a `non-normative` reason that judges what ZE owes rather than what the DOCUMENT states. Every summary needs a recorded disposition that distinguishes "the RFC imposes nothing", "nobody extracted it", and "we do not have the text" |
| `check_unproven_support` | a support claim over a summary that declares ZERO gated requirements. A claim is any Status other than `Unsupported` or `Future`, an empty cell included. Two ledgers agreeing on NOTHING is not conformance. It is the cheapest way to look green. Two escapes exist and both are evidence rather than assertion. One is a `non-normative` disposition whose reason states a property of the text. The other is a VALID `manual-walk` extraction sign-off carrying a `register-reason`. The second lets an Informational RFC that invokes RFC 2119 nowhere enrol on an honest zero, and it needs no fabricated MUST |
| `check_gap_count_agreement` | a Remaining cell whose spelled number, sitting immediately before MUST or SHALL, disagrees with the real `{gap}` count. The COUNT is the only fact on that page a machine can own. It says how many annotations exist. It never says their classifications are right, which matters because the paragraph above VOIDS every annotation as authority. A Remaining PROSE derived from those classifications would launder a void judgement into generated text |

Un-enrolment no longer exempts a `{gap}` from disclosure either. It gates only the
MISSING-ROW branch. An un-enrolled RFC with no row makes no public claim to contradict. One
that HAS a row was contradicting its own row in public, with nothing to notice.

At edit time the `rfc-tagged-test` guard (`_rfc_tagged_change_err`) blocks a behavior
change to any test carrying an `RFC requirement:` tag, and separately blocks REMOVING the
tag. Removal is checked first and on its own: a tag is a comment, so a behavior comparison
waves its deletion through, after which the test is unguarded and a self-written row in
`test/weakened.md` alone buys any later weakening. Scope is the enclosing test function, not the edited hunk (a tag
sits on the doc comment, so a hunk-scoped guard misses exactly the edit it exists to stop)
and not the whole file (which blocked 331 of 3220 untagged helper functions).

**A tagged test's assertions MUST NOT be weakened *in place* while keeping the same shape.** None of the eight ratchets catches that: `c_test_weakening` and `./le commit audit`, plus the SHA ratchet (`check_audit_freshness`), catch it instead, wherever `/ze-rfc-audit` has recorded a verdict. The SHA ratchet is armed only for RFCs that have an `rfc/audit/<rfc>.json`.

## Before Implementing BGP Features

1. Find RFC in `rfc/` — if missing: `curl -o rfc/full/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`
2. Read relevant sections, note MUST/SHOULD/MAY
3. Check ExaBGP reference

**Priority:** the RFC MUST outrank ExaBGP API compat, which MUST outrank the ExaBGP implementation.

## Wire Format Documentation (MANDATORY)

Never modify protocol code without documenting wire format: ASCII diagram with field offsets, byte offset annotations, RFC section reference.

## RFC MUST Comments (BLOCKING)

Every MUST/MUST NOT enforced in code needs a comment directly above:
```
// RFC NNNN Section X.Y: "quoted requirement"
<enforcing code>
```

Document: validation rules, error conditions, state transitions, timer constraints, message ordering.

## MAY Clauses

ASK user: implement? skip? config option?

## Common RFCs

| Feature | RFC | Location |
|---------|-----|----------|
| BGP-4 base | 4271 | `internal/component/bgp/message/`, `internal/component/bgp/reactor/` |
| MP-BGP | 4760 | `internal/component/bgp/reactor/received_update.go`, `internal/core/bgp/attribute/` |
| 4-byte ASN | 6793 | `internal/core/bgp/capability/capability.go` |
| Add-Path | 7911 | `internal/core/bgp/capability/capability.go` |
| GR | 4724 | `internal/core/bgp/capability/capability.go` |
| Error handling | 7606 | revised error handling |

ExaBGP ref: `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/`
