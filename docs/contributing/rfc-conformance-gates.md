# The RFC Conformance Gates

`./le rfc check` judges whether Ze's RFC obligations are still extracted,
implemented, proven and publicly declared. This page describes what it checks
and what each check refuses. What an author OWES is in
`ai/rules/rfc-compliance.md`; this page is how the machine measures it.

Every check named here is a function in `internal/le/rfc/`. The package was
ported from a Python tool, so older prose and older commit messages spell these
names in snake_case. The Go names below are the current ones.

## The artifacts

| Path | Holds |
|------|-------|
| `rfc/full/<stem>.txt`, `rfc/drafts/` | The RFC's own text. The source, and the only thing a conformance claim may quote |
| `rfc/short/<stem>.md` | The extracted summary: one checklist row per requirement, plus a Meta table |
| `rfc/enrolled.txt`, `rfc/not-enrolled.txt` | Which summaries are gated, and the recorded reason for each that is not |
| `rfc/extraction/<stem>.json` | The extraction sign-off: the walk of the RFC text, recorded so a machine can re-check it |
| `rfc/audit/<stem>.json` | A recorded `/ze-rfc-audit` verdict, and the fingerprints that keep it fresh |
| `rfc/discrimination/<stem>.json` | The recorded breaks under which a tagged unit goes red: one record per requirement, polarity and tagged unit |
| `rfc/drain-budget.txt` | The extraction drain schedule: a start date and a rate, and nothing else |
| `docs/features/rfc-status.md` | The PUBLIC support claim, one row per enrolled RFC |
| `ai/RFC-REQUIREMENTS.md` | The generated backlog, including the extraction sign-off counts |

## The ratchets

`./le rfc check` reads the WORKING TREE to judge coverage, and a tree cannot
tell "never proven" from "stopped being proven". Eight comparisons against git
HEAD supply that difference. Each fires only on a real downgrade, so a green run
means the evidence held rather than that nobody looked.

| Ratchet | Producer | Fires when |
|---------|----------|-----------|
| Enrolment is monotonic | `checkEnrolment` | an RFC whose MUSTs were gated stops being gated |
| Proof is monotonic | `checkCoverageRatchet` | a requirement loses a polarity it had at HEAD. A `{gap}` is NOT an escape: it is the move being blocked |
| Gating is monotonic | `checkLevelRatchet` | a requirement leaves the MUST-level population, because its level was gated at HEAD and is advisory now. That is the cheapest route from red to green, cheaper than `{gap}` and cheaper than deleting the row, because the id and the tests survive while every coverage obligation attached to the row disappears. The one escape is a `Correction <YYYY-MM-DD>:` paragraph in the same summary, naming the id and quoting at least 24 characters of the RFC verbatim. A row GAINING a gated level is never reported |
| Requirements do not vanish | `checkRetiredRequirements` | a requirement id of an enrolled RFC disappears from its summary. Without this, deleting the checklist line is cheaper than `{gap}`, which costs a public disclosure row, and the ratchet would pressure people to hide obligations rather than declare them. Correcting a misquote means editing the TEXT under the same id, which is allowed |
| Adding an RFC adds checking | `checkNewSummaries` | a summary NEW since HEAD declares gated MUSTs and is not in `rfc/enrolled.txt`, fails to parse, or captures zero requirements while `rfc/full/<stem>.txt` has MUST-level keywords. A document's own RFC 2119 key-words paragraph does not count, and neither does its reference-list entry for RFC 2119 or RFC 8174: both say where the words come from, and neither binds anybody |
| Non-unit evidence is monotonic, per tier | `checkEvidenceRatchet` | a requirement loses an evidence KIND it had at HEAD: its `.ci` becomes a unit test, or a verify-tier binding is swapped for a nightly-tier interop one. Keyed by `kind/tier`, so a substitution leaving the tag COUNT unchanged still fires. A unit test proves the algorithm; only a running functional or interop test proves the daemon or a peer. No annotation satisfies it |
| Extraction is monotonic | `checkExtractionRatchet` | a stem that carried a sign-off at HEAD carries none now, or a signed stem's exclusion count RISES without a `resign-reason` and a bumped `signed-off` date. The first stops the bound being un-bound by deleting a file; the second stops the exclusion list becoming a hatch where every unmapped site is excluded with a shrug |
| Public disclosure is monotonic | `checkStatusCompleteness` | an RFC enrolled since HEAD has no row in `docs/features/rfc-status.md`, or a row that existed at HEAD is gone while its RFC stays enrolled. Enrolment gates that RFC's MUSTs, so the public page must say the RFC exists |

`checkIDAllocation` and `checkAuditVerdictRatchet` (`internal/le/rfc/check_ratchets.go`,
`check_audit.go`) compare against HEAD on the same footing, for requirement id
allocation and recorded audit verdicts.

Summaries that predate HEAD are the existing backlog and are deliberately
grandfathered. A rule that reds the gate on unrelated work gets removed rather
than obeyed. Where git cannot answer, every ratchet judges nothing rather than
judging everything.

### The drain schedule

`checkDrainFloor` (`internal/le/rfc/check_extraction.go`) compares the derived
sign-off count against `rfc/drain-budget.txt`. It is a schedule rather than a
ratchet, and it ships INERT at rate 0. Only the owner arms it.

The rate is unset by RULING, not for want of a number. Four RFCs were walked end
to end on 2026-08-30 to measure what a sign-off costs, and the table is in
`rfc/drain-budget.txt` and in that spec. Thomas ruled on 2026-08-31 that the
schedule waits, because a quota over incomplete code buys a signature rather
than conformance.

**The trigger is the first RFC at 100% coverage**, in his words: "we need our
first 100% coverage before locking the gate for the RFC verification". Arming
waits on one enrolled RFC being taken all the way, not on a date and not on a
backlog count.

The reason the trigger is coverage rather than sign-offs is that the two measure
different things. A sign-off bounds what a summary MISSED: it is the walk of the
RFC's own text, recorded so a machine can re-check it, and that is what the four
walks costed. Coverage is every gated requirement actually PROVEN, in both
polarities, with no `{gap}` and no `{not-applicable}` standing. A corpus can be
fully signed off and prove nothing. Until one document has been carried to the
second state, nobody knows what a whole RFC costs, and a drain rate is a claim
about exactly that.

Read `rfc/drain-budget.txt`'s own comment before you propose a rate: it carries
the measurement and the trigger, and it says to reset `start` to the arming
date, because the floor is CUMULATIVE and an old date bills the tree for every
month the quota was inert.

The arithmetic is proven rather than assumed. `requiredFloor`, `parseDrainBudget`
and `checkDrainFloor` carry unit tests over the month count, the anniversary
clamp in a short month, the enrolled-set cap, the absent-file refusal and the
rate boundaries.

## The public ledger's edges

`docs/features/rfc-status.md` is the PUBLIC claim, and a `{gap}` annotation is
the private admission. `checkStatusAgreement` compares the two, but it reaches
for a row only when a `{gap}` exists. Three classes of defect sat outside it, so
each is a hard requirement rather than a HEAD comparison.

| Guard | Refuses |
|-------|---------|
| `checkSummaryDisposition` | a summary in `rfc/short/` that is in neither `rfc/enrolled.txt` nor `rfc/not-enrolled.txt`. Also a stem in BOTH, a disposition naming a summary that does not exist, and a disposition deleted while the stem never reaches `rfc/enrolled.txt`. Also a `non-normative` reason that judges what ZE owes rather than what the DOCUMENT states. Every summary needs a recorded disposition distinguishing "the RFC imposes nothing", "nobody extracted it", and "we do not have the text" |
| `checkUnprovenSupport` | a support claim over a summary that declares ZERO gated requirements. A claim is any Status other than `Unsupported` or `Future`, an empty cell included. Two ledgers agreeing on NOTHING is the cheapest way to look green. Two escapes exist, and both are evidence rather than assertion: a `non-normative` disposition whose reason states a property of the text, or a VALID `manual-walk` extraction sign-off carrying a `register-reason`. The second lets an Informational RFC that invokes RFC 2119 nowhere enrol on an honest zero, with no fabricated MUST |
| `checkGapCountAgreement` | a Remaining cell whose spelled number, sitting immediately before MUST or SHALL, disagrees with the real `{gap}` count. The COUNT is the only fact on that page a machine can own: it says how many annotations exist, never that their classifications are right |

Un-enrolment exempts only the MISSING-ROW branch. An un-enrolled RFC with no row
makes no public claim to contradict; one that HAS a row was contradicting its own
row in public.

## The superseded marker

`checkSuperseded` (`internal/le/rfc/check_core.go`) refuses a summary whose
forward Meta row names a successor unless every requirement line it declares
carries a `{superseded: ...}` marker.

`parseSuccessorStem` (`internal/le/rfc/ledger.go`) reads the label. The corpus
spells it four ways, and `obsoletedRowRE` matches `Obsoleted by` and
`Obsoleted-by` in either capitalisation. A qualifier after the label is kept,
which is how `rfc/short/rfc1334.md` writes `| Obsoleted-by (partial) |` for a
document whose CHAP half moved to RFC 1994 and whose PAP half did not. Any OTHER
Meta field whose name matches `obsolescenceRE` (`(?i)obsolet`) reds the gate
rather than being skipped: a reader that skips what it does not recognise cannot
be trusted to have found anything.

Widening the recognised word list is separate work, because the meta-field match
reads the first cell of every table row, so a looser word would collide with the
requirement tables themselves.

Four dispositions are accepted, and each has a precondition the gate checks:

| Disposition | Says | Precondition |
|-------------|------|--------------|
| `restated <ID>; why` | the successor states the same obligation, under that id | the successor's summary is in `rfc/short/` AND declares that id |
| `dropped; why` | the successor states no equivalent obligation | the successor's own text is in `rfc/full/` or `rfc/drafts/` |
| `unextracted <§section>; why` | the successor STATES it, at that section, and its summary declares no row | the successor's own text is in `rfc/full/` or `rfc/drafts/` |
| `unresolved; why` | the successor's text is not in this repository | that text is ABSENT |

The last two are DEBT, and the ledger publishes them as debt. An `unresolved`
line drains when somebody fetches and summarises the successor. An `unextracted`
line drains by an extraction pass over the successor's summary.

## The extraction sign-off

`./le rfc check` verifies that every requirement LISTED in a summary is covered.
It cannot know about an obligation nobody wrote down, so a green gate is bounded
by what was extracted. `rfc/extraction/<stem>.json` is the record of the walk
that fixes that bound, and it is a precondition of a new enrolment
(`checkEnrolment`).

| Step | Command or file |
|------|-----------------|
| Write the skeleton | `./le rfc extraction-create stem <stem>` |
| Classify every derived site and section by hand | the file the command names, under this session's scratch |
| Move the classified walk into the corpus | `mv <scratch>/rfc-extraction/<stem>.json rfc/extraction/<stem>.json` |
| Re-check the arithmetic | `./le rfc check` |
| Read the published backlog | `ai/RFC-REQUIREMENTS.md`, "Extraction sign-off" |
| Read the counts machine-readably | `./le rfc extraction-status` |

Before you enrol `rfc/short/<stem>.md` in `rfc/enrolled.txt`, walk the RFC's own
text section by section and confirm that every MUST, MUST NOT, SHALL, SHALL NOT
and REQUIRED has a checklist row. When `rfc/full/` lacks the source, fetch it
first, because "verified against the RFC" is not reproducible without it:

    curl -o rfc/full/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt

**The skeleton reaches `rfc/extraction/` only when every site and every section
already carries a disposition.** Anything less is written to this session's
scratch, and the command prints the `mv` that ends the walk. An unclassified
artifact under `rfc/extraction/` fails `./le rfc check` for the whole corpus, so
a generator that wrote one in place made its own output a gate failure, and a
batch of them a corpus-wide one. A refresh whose every decision carries forward
IS a sign-off, so that one is written in place as before.

**A sign-off counts when its stem is enrolled, and the rest is named rather than
hidden.** Credit and the backlog must describe one set, so a walk completed
before its RFC enrols raises no count. `./le rfc check` prints that set on its
own line so the walk is never silently uncounted, and it starts counting the day
its stem enters `rfc/enrolled.txt`.

Summaries enrolled before the gate existed are grandfathered and published as a
counted backlog. Grandfathering is implemented as SCOPE (new since HEAD), never
as an allowlist file, so nothing is added to a list of exceptions when an RFC
stops being one.

The contract is `rfc/extraction/README.md`. Five properties are worth knowing
before you meet one:

- **Only dispositions are authored.** Sites, sections, quotes, the register and
  every published count are DERIVED from the source text at check time. A
  hand-typed "sites seen" is a claim, and claims are what this removes.
- **A generated skeleton can never pass.** The writer emits only UNCLASSIFIED
  dispositions and an unclassified site fails the check, so mass-generating
  artifacts makes the gate redder rather than greener.
- **The register is derived, and a stronger claim is refused.** It is `rfc2119`,
  `prose`, or `manual-walk`. A keyword-only check can be vacuously green when an
  RFC declares gated obligations without a capitalised MUST-level keyword site.
- **The bound is over keyword-visible sites, not over obligations.** Recall can
  be near zero for an indicative-prose section. `unsourced-ids` records an
  obligation the extractor cannot see. This raises a floor from zero; it does
  not reach a ceiling.
- **A FIRST sign-off is reviewed, not ratcheted.** `checkExtractionRatchet`
  compares a stem against its own HEAD row, so a stem signing off for the first
  time has no baseline and could exclude every site. The published per-RFC
  exclusion ratio is the control; read it before you approve one.

### Two signals that an extraction is missing

| Signal | Why it matters |
|--------|----------------|
| A `{not-applicable}` whose reason is "ze has no X producer at all" | That admission is often the violation of a separate MUST requiring X to exist. RFC 4271 §5.1.4's "MUST implement a mechanism ... that allows MULTI_EXIT_DISC to be removed" was unextracted, and two requirements cited its absence as their exemption |
| A section whose siblings are enumerated but one clause is not | RFC 8666 §5's "MUST be ignored on reception" was omitted while §6, §7.1 and §7.2 each had it. An enumeration hole, not a style choice |

The requirement TEXT matters as much as its presence. A misquoted obligation
licenses a justification that never engages it: RFC 4271 §5.1.6 binds a speaker
THAT RECEIVES a route with ATOMIC_AGGREGATE, and recording it as an aggregator
rule let the readvertisement path be cited as evidence of non-applicability when
it is the bound path.

## The discrimination record

A tag is `RFC requirement: <ID> <polarity>` followed by prose that states what
the test demonstrates. `parseTagRest` (`internal/le/rfc/tags.go`) reads the
structured half. No gate reads the prose, because it is a sentence. A tag can
therefore advertise an assertion its body never makes.

`rfc/discrimination/<stem>.json` is what replaces reading that prose. One
record says that a named tagged unit was OBSERVED to fail under a named break
of the code the claim rests on. "The prose is true" is unfalsifiable by a
machine. "This break makes this unit red" is decidable and replayable.

| Field | Holds |
|-------|-------|
| `rid` | the requirement the record proves |
| `polarity` | `positive` or `negative`, the direction it proves |
| `unit` | the tagged unit key, `<path>::<FuncName>` for a Go function and a bare `<path>` when the whole file is the unit, which is the scope `UnitAt` (`internal/le/rfc/goscope.go`) answers for a `.ci`. `fingerprintKey` parses it, so the retired `<path>:<line>` form is refused |
| `unit-sha` | that unit's behavior hash when the red was observed |
| `route` | `mutant` for a generated break, `revert` for a producer disabled by hand, `no-break` for the escape |
| `producer` | the function the break was applied to, in the same key form. Required for a proof route, refused for the escape |
| `producer-sha` | that function's behavior hash when the break was applied |
| `break` | what was done to the producer. No gate parses it; a reviewer reads it |

The full artifact contract is `rfc/discrimination/README.md`.

`LoadDiscrimination` (`internal/le/rfc/discriminate.go`) reads the tree,
`verifyDiscrimination` re-checks each record's fingerprints against the working
tree, and `checkDiscriminationRatchet` (`internal/le/rfc/check_ratchets.go`)
judges what both answered. Six refusals exist today.

| Refuses | Why |
|---------|-----|
| A file that cannot be parsed, an unknown JSON key, or a filename that disagrees with its own `rfc` field | A corrupt record must never read as a corpus with nothing proven |
| A polarity, a route, a key or a fingerprint outside its closed form | A half-read record is the shape a false proof takes |
| A record naming a requirement no summary declares | A proof of an obligation nobody wrote down proves nothing |
| Two records claiming one requirement, polarity and carrier file | The proven count is published, and a duplicate inflates it |
| A record whose `unit` or `producer` no longer resolves in the tree | The record died with what it named |
| A record whose `unit-sha` or `producer-sha` no longer matches | Nothing observed the red over the code that is there now, so a hand-written record is refused by the same rule that catches a real drift |

The two fingerprints are minted by `sealDiscrimination`, the one place a hash is
computed, and they hash `behaviorBytes` rather than the raw text. A reworded
comment, a reflow, an inserted header and a blank line each leave a record
verified; a changed assertion or a rewritten producer voids it. That is the same
predicate `ChangedTags` uses, so a record goes stale exactly when the obligation
says its unit moved, and the re-stamp burden `rfc/audit/rfc7606.json` records
does not repeat here.

An ABSENT record is not refused. Most tags have never been proven, and that is
a backlog the summary line publishes:

    discrimination: 0 proven, 0 owed, 0 escaped

`proven` counts the records taking a proof route and `escaped` counts the
`no-break` records, which are debt rather than evidence. `owed` is
change-scoped: a tag present in the tree and absent at HEAD owes its record in
the change that adds it, and a tag older than HEAD is grandfathered, exactly as
the extraction backlog is. Where git cannot answer, `owed` is 0, because a
baseline that cannot be read accuses nobody.

`./le rfc discriminate stem <stem>` and `./le rfc discriminate id <ID>` answer
what one RFC or one requirement has proven, which of its records no longer
verify, and which of its tags carry no record. The gate itself runs no test, no
mutant and no scenario: it reads the recorded proof and compares its
fingerprints, which is what `checkAuditFreshness` already does for a verdict.

So a verified record says the red WAS observed, and that the code it was
observed over has not moved since. It does not say the red would happen again on
a machine that never ran it. Re-observing is `./le rfc discriminate`, which an
author runs deliberately.

## What the ratchets cannot see

None of the eight catches a tagged test whose assertions are weakened IN PLACE
while the shape stays the same. Three other mechanisms do:

- `writeWeakening` (`internal/le/hookruntime/writeedit.go`) refuses the edit at
  write time, through `testweakened.Proposed`
  (`internal/le/testweakened/proposed.go`). It blocks a behavior change to a
  test carrying an `RFC requirement:` tag, and separately blocks REMOVING the
  tag. Removal is checked first and on its own, because a tag is a comment and a
  behavior comparison would wave its deletion through. Scope is the enclosing
  test function: a tag sits on the doc comment, so a hunk-scoped guard would
  miss exactly the edit it exists to stop.
- `./le commit audit` checks the same at commit time.
- `checkAuditFreshness` (`internal/le/rfc/check_audit.go`) is the SHA ratchet,
  armed only for an RFC that has an `rfc/audit/<stem>.json`.

A `test/weakened.md` row is self-service, and it does NOT authorize weakening an
RFC-tagged test.
