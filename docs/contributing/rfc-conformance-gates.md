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
| `rfc/short/<stem>.md` | The extracted summary, and the ONE place every fact about that RFC is declared. One checklist row per requirement, plus the `## Meta` table. That table states whether the RFC is gated, and what the public page claims for it |
| `rfc/enrolled.txt`, `rfc/not-enrolled.txt` | GENERATED from the Meta tables by `./le rfc index-update`: which summaries are gated, and the recorded reason for each that is not |
| `rfc/extraction/<stem>.json` | The extraction sign-off: the walk of the RFC text, recorded so a machine can re-check it |
| `rfc/audit/<stem>.json` | A recorded `/ze-rfc-audit` verdict, and the fingerprints that keep it fresh |
| `rfc/discrimination/<stem>.json` | The recorded breaks under which a tagged unit goes red: one record per requirement, polarity and tagged unit |
| `rfc/drain-budget.txt` | The extraction drain schedule: a start date and a rate, and nothing else |
| `docs/features/rfc-status.md` | GENERATED from the Meta tables by `./le rfc index-update`: the PUBLIC support claim, one row per summary that declares a section. Its `Proof` column is the exception to that: it is derived from the checklist and the tags, and states each stem's gated count partitioned into proven, annotated and untested |
| `ai/RFC-REQUIREMENTS.md` | The generated backlog: the coverage rollup, the audit coverage, the claim-discrimination counts and the extraction sign-off counts |
| `rfc/requirements/<stem>.md` | One RFC's requirement table: six cells per requirement, generated from the summary and the tags |

Everything in that table is also PUBLISHED, one page per summary stem at
`/quality/rfc-compliance/<stem>/`. The page carries the same six cells, the
recorded verdict and its freshness, and the state of every stored proof
re-verified against the tree. `internal/le/site/rfcledger.go` derives it, and
the disclosure is full by owner ruling of 2026-09-01: a `no-break` record is
named as the escape it is and never counted as a proof, a verdict that is not
`enforced` is named under its requirement id, and a gated MUST with no test is
listed rather than absorbed into a percentage.

## The gate's own answer

`./le rfc check` gives one of three answers, and its exit code says which.

| Exit | Answer | The page opens with |
|------|--------|---------------------|
| 0 | Clean: every gated MUST of every enrolled RFC is covered | `rfc-requirements OK:` |
| 2 | Violations, each named on a `  * ` line of its own | `rfc-requirements: <n> violation(s)` |
| 2 | The gate cannot READ the tree, so it judged nothing | `rfc-requirements: cannot run:` |

The third answer shares an exit code with the second on purpose. A gate that
never ran MUST NOT render as a pass (`ai/rules/principles.md`).

`CheckReport` (`internal/le/rfc/check.go`) is the ONE payload behind all three.
`| json`, `| yaml` and `| table` render that object. `CheckReport.Text` renders
the page a person reads. So `| json` answers an OBJECT and never a list of
violation strings. The shape does not change with the verdict, and the counts,
the evidence split and the audit figures stay reachable while the gate is red.
`violations` carries one string for each bullet the page lists, in the same
order.

The six `discrimination-*` keys render even at zero. They are published debt,
and an absent key would read as "this gate has no such stage".

`CheckReport.Text` takes a POINTER receiver, so `checkAnswer`
(`internal/le/rfc/actions.go`) returns a pointer. `leroot.Prose` is matched by a
type assertion, a value carries no pointer-receiver method, and the dispatcher
then falls through to the generic table renderer. That shipped for weeks: the
gate found every violation and still exited 2, and it showed the page to nobody.

`test/ui/le-rfc-answers.ci` holds the three answers to this contract. It seeds
ONE gated MUST that no test tags into an isolated export of HEAD. The gate MUST
name that requirement and exit 2. The same export without the seed MUST NOT
name it. A count read off the working checkout is met by whatever backlog is
standing, so the seed is what makes the case able to fail.

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
| Adding an RFC adds checking | `checkNewSummaries` | a summary NEW since HEAD declares gated MUSTs and does not declare itself enrolled, fails to parse, or captures zero requirements while `rfc/full/<stem>.txt` has MUST-level keywords. A document's own RFC 2119 key-words paragraph does not count, and neither does its reference-list entry for RFC 2119 or RFC 8174: both say where the words come from, and neither binds anybody |
| Non-unit evidence is monotonic, per tier | `checkEvidenceRatchet` | a requirement loses an evidence KIND it had at HEAD: its `.ci` becomes a unit test, or a verify-tier binding is swapped for a nightly-tier interop one. Keyed by `kind/tier`, so a substitution leaving the tag COUNT unchanged still fires. A unit test proves the algorithm; only a running functional or interop test proves the daemon or a peer. No annotation satisfies it |
| Extraction is monotonic | `checkExtractionRatchet` | a stem that carried a sign-off at HEAD carries none now, or a signed stem's exclusion count RISES without a `resign-reason` and a bumped `signed-off` date. The first stops the bound being un-bound by deleting a file; the second stops the exclusion list becoming a hatch where every unmapped site is excluded with a shrug |
| A claim keeps its proof, and a new claim owes one | `checkDiscriminationRatchet` | a tagged unit the tip commit added against `HEAD^` carries no discrimination record, a record committed at HEAD is deleted while its tag stands, or a recorded proof no longer verifies against the tree. This is the only ratchet that reads the PROSE half of a tag: `claim-sha` fires when the sentence is reworded, because a proof of the old claim is not a proof of the new one |

`checkIDAllocation` and `checkAuditVerdictRatchet` (`internal/le/rfc/check_ratchets.go`,
`check_audit.go`) compare against HEAD on the same footing, for requirement id
allocation and recorded audit verdicts.

Summaries that predate HEAD are the existing backlog and are deliberately
grandfathered. A rule that reds the gate on unrelated work gets removed rather
than obeyed. Where git cannot answer, every ratchet judges nothing rather than
judging everything.

The enrolled baseline is `baselineMetas` (`internal/le/rfc/check_baseline.go`).
It parses the `## Meta` table of every summary git HEAD holds. A summary that
does not parse there is skipped rather than emptying the baseline. One
unreadable file at HEAD must not empty every ratchet's population.
`checkRetiredRequirements`, `checkLevelRatchet`, `checkCoverageRatchet` and
`checkEvidenceRatchet` run only where the current enrolled set intersects that
baseline. A baseline nobody can read disarms all four.

`baselineMetasBeforeMigration` reads the retired `rfc/enrolled.txt` and
`rfc/not-enrolled.txt` out of GIT HISTORY. It is reached only when no summary at
HEAD declares an enrolment at all. That is the ability to compare against a
commit written before the declaration moved, and never a fallback in the live
path. Without it, the commit that moved the declaration is the one commit whose
baseline is unreadable, over exactly the change those four ratchets judge.

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
the private admission. Both are now written in one file: the summary declares
its own row in its `## Meta` table, and the page is rendered from it.

That retired four refusals, because each compared two copies of one fact. Each
of the four is now UNREPRESENTABLE rather than refused:

- a summary in neither disposition file
- a stem in both
- a disposition naming a summary that does not exist
- a row naming an RFC with no summary

A fifth, a newly enrolled RFC with no public row, is NOT unrepresentable: one
`Support` cell reading `-` states it. `checkStatusCompleteness` is gone, and
`checkPublicRowMonotonic` carries that refusal now, over the same two branches.
`checkSummaryDisposition` lost three of its branches, and `checkSupportedSignoff`
lost a population it can never judge. Nothing was weakened: deleting a copy is
the only free simplification, and the one refusal that was not a copy stayed.

`checkStatusAgreement` still compares the claim against the admission, and it
reaches for a row only when a `{gap}` exists. Five classes of defect sit outside
it, so each is a hard requirement rather than a HEAD comparison.

| Guard | Refuses |
|-------|---------|
| `checkSummaryDisposition` | a `non-normative` reason that judges what ZE owes rather than what the DOCUMENT states, or that cites nothing a reviewer can check. `non-normative` is the one disposition that claims anything about conformance. Its reason rests on the document: the IETF category, an RFC 2119 / RFC 8174 / BCP 14 key-words paragraph, or a capitalized MUST/SHALL/REQUIRED scan of the source |
| `checkSourceRestricted` | a `source-restricted` reason that names neither the body publishing the standard (ISO, IEC, ITU, IEEE, ANSI, ETSI) nor the license, copyright or paywall that stops the text being copied, and the same kind written over a text that IS in the tree. It excuses no public support claim: being unable to bound a claim is a reason to stop making it, not a reason to be excused from proving it. It is the only PERMANENT disposition: where the text IS fetchable the kind is `blocked`, and a fetch discharges it |
| `checkUnprovenSupport` | two shapes of a claim nothing behind it can contradict. A support claim over a summary that declares ZERO gated requirements, where a claim is any Status other than `Unsupported` or `Future`, an empty cell included: a claim and a checklist that agree on NOTHING is the cheapest way to look green. Two escapes exist there, and each is evidence rather than assertion. They are a `non-normative` disposition whose reason states a property of the text, and a VALID `manual-walk` sign-off with a `register-reason`. The second one lets an Informational RFC that invokes RFC 2119 nowhere enrol on an honest zero. The other shape is a row PROMISING conformance -- `Supported`, alone or with a scope after it -- over gated requirements of which not one carries a both-polarity test. Neither escape reaches it, because both answer whether the DOCUMENT imposes a MUST rather than whether Ze meets one, and `Partial` is the row that states what is true |
| `checkPublicRowMonotonic` | a `Support` cell that read a section at HEAD and reads `-` now, while the summary is still there, and a newly enrolled RFC that arrives with no row at all. It is keyed on the ROW, never on enrolment, because `checkSupportedSignoff` bills any row whose Status promises conformance. RFCs enrolled before it existed are grandfathered, so the count of enrolled RFCs with no row can only shrink |
| `checkLowerLayerProducer` | a `{lower-layer}` annotation whose producer this checkout cannot show: the file is absent, or it declares no function of that name. The kind rests on a fact a reader can open, and a producer that was renamed or deleted under the annotation is the event this catches |
| `checkGapCountAgreement` | a Remaining cell whose spelled number, sitting immediately before MUST or SHALL, disagrees with the real `{gap}` count. The COUNT is the only fact on that page a machine can own: it says how many annotations exist, never that their classifications are right |

Un-enrolment exempts only the MISSING-ROW branch of `checkStatusAgreement`. An
un-enrolled RFC with no row makes no public claim to contradict; one that HAS a
row was contradicting its own row in public.

## The lower-layer annotation

`{lower-layer}` says a layer UNDER Ze performs the behavior, on state Ze
installs into that layer. The owner ruling of 2026-08-31 counts such a
requirement MET and asks for a test at the boundary Ze owns. This kind is for
the requirements where that boundary carries nothing the behavior reads, so no
value exists to assert. Sixteen RFC 4302 obligations are the case it was added
for: Linux XFRM builds every AH packet, and no field of the SA Ze installs
decides that the RESERVED field is zero.

```
- [ ] [RFC4302-2.3-1] [MUST] The RESERVED field MUST be set to zero by the sender (§2.3) {lower-layer: Linux XFRM; internal/plugins/ospf/ipsec_install.go::buildIPsecSA installs the AH SA and the kernel's AH output builds every header, so no value Ze writes decides this field}
```

The reason states two facts, and the gate checks both:

| Fact | Written as | Checked by |
|------|-----------|------------|
| The LAYER that performs the behavior | the head, before the `;` | `parseLowerLayer` refuses an empty head, and a reason with no `;` at all |
| The PRODUCER in Ze that installs into that layer | `<path>.go::<Symbol>` anywhere in the reason | `parseLowerLayer` refuses a reason naming none; `checkLowerLayerProducer` then refuses one the tree cannot show |

That producer demand is the whole difference from `{not-applicable}`. That kind
asserts a judgement nothing in the tree can contradict, which is how it grew to
915 sites the owner ruling presumes are mostly wrong. This kind claims a fact,
and a rename or a deletion under it turns the gate red.

Four rules decide whether it is the right kind:

- **The obligation BINDS Ze.** A requirement addressed to a role Ze never fills
  is `{not-applicable}`, and that label is presumed wrong before it is written.
- **A layer under Ze performs it, on state Ze installs.** Where NO layer
  performs it, nothing is met and the honest kind is `{gap}`.
- **Ze's own boundary carries nothing to assert.** Where Ze installs a value the
  behavior reads, the requirement owes a TEST over that value, at the boundary
  Ze owns, and this annotation is refused beside it: a tagged test on a
  `{lower-layer}` row makes the annotation stale, exactly as it does on a
  `{gap}` or a `{not-applicable}` one.
- **It is not a conformance rollup.** A requirement whose content is "implement
  all of this document" is met by the other rows and not by a layer, so this
  kind would launder a whole document behind one claim.

It stays INSIDE the gated denominator and OUT of the proven numerator
(`ProvenShareOf`, `internal/le/rfc/provenshare.go`): the requirement is met, and
it is not proven BY ZE. Annotating a row may not move the published share by a
point. On the site it is its own bucket, `lower_layer`, labeled `Met below Ze`
and colored neutral, because an obligation met below Ze is neither a test Ze
wrote nor work Ze owes.

It cannot take a `{gap}`'s slot. It lives in the coverage register, where one
line carries ONE disposition, so a line carrying both is refused rather than
silently relabeled. That is the same reason `{superseded}` was kept out of the
register: a way out of the gated population must not be creatable by writing a
second marker beside the one already there.

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

Before you set the summary's `Enrolment` row to `enrolled`, walk the RFC's own
text section by section. Confirm that every MUST, MUST NOT, SHALL, SHALL NOT and
REQUIRED has a checklist row. When `rfc/full/` lacks the source, fetch it
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
its summary declares `enrolled`.

Summaries enrolled before the gate existed are grandfathered and published as a
counted backlog. Grandfathering is implemented as SCOPE (new since HEAD), never
as an allowlist file, so nothing is added to a list of exceptions when an RFC
stops being one.

The contract is `rfc/extraction/README.md`. Six properties are worth knowing
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
- **A gap is an ISSUE and an exclusion is a DECISION.** A `{gap}` says Ze owes
  the behavior and does not produce it, so it stays on the ledger until the
  behavior exists. An excluded site says the obligation never bound Ze, and the
  kind names which decision put it out of reach. `feature-out-of-scope` is the
  kind for an OPTIONAL feature Ze declined to offer: the absent FEATURE is
  disclosed on `docs/features/rfc-status.md`, through the summary's own
  `Support status` and `Support remaining` rows, as an implementation gap a
  later scope decision can revisit, and never as a conformance gap.

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
| `claim-sha` | the hash of what the TAG claims, which is a separate field because `behaviorBytes` strips comments and a claim IS a comment. Without it a sealed proof survives a reworded claim, and a widened sentence would be published as proven with no code edit at all |
| `route` | `mutant` for a generated break, `revert` for a producer disabled by hand, `no-break` for the escape |
| `producer` | the code the break was applied to, in the same key form. Required for a proof route. An escape names it too, unless its reason is `foreign-producer`, because the reason is a claim ABOUT that code |
| `producer-sha` | that function's behavior hash when the break was applied |
| `break` | what was done to the producer, derived from what was applied. No gate parses it; a reviewer reads it |
| `citation` | the assertion a proof or a `foreign-producer` escape rests on: a numbered `fail(N, ...)` site for an interop checker, a directive line for a `.ci`. Required for a functional or interop proof and for the `foreign-producer` escape, refused for a unit record and for the two escapes that name a producer |
| `reason` | why no break exists, for the escape only, out of the closed vocabulary below |

The full artifact contract is `rfc/discrimination/README.md`.

`loadDiscrimination` (`internal/le/rfc/discriminate.go`) reads the tree,
`verifyDiscrimination` re-checks each record's fingerprints against the working
tree, `baselineRecordBlobs` (`internal/le/rfc/check_baseline.go`) reads what HEAD
holds for the same files, and `checkDiscriminationRatchet`
(`internal/le/rfc/check_ratchets.go`) judges what all three answered. Eleven
refusals exist today.

| Refuses | Why |
|---------|-----|
| A file that cannot be parsed, an unknown JSON key, or a filename that disagrees with its own `rfc` field | A corrupt record must never read as a corpus with nothing proven |
| A polarity, a route, a key or a fingerprint outside its closed form | A half-read record is the shape a false proof takes |
| A record naming a requirement no summary declares | A proof of an obligation nobody wrote down proves nothing |
| Two records claiming one requirement, polarity and tagged unit | The proven count is published, and a duplicate inflates it |
| A record whose `producer` no longer resolves in the tree | The break was applied to code that is gone |
| A record whose `unit-sha`, `claim-sha` or `producer-sha` no longer matches COMMITTED code | Nothing observed the red over the code that was committed, or the red was observed about a different sentence, so a hand-written record is refused by the same rule that catches a real drift. The drift is judged against HEAD, never against the working tree (owner decision, 2026-08-31): several sessions share this checkout, so one session's uncommitted edit to a producer would otherwise red the gate for all of them, and clearing an interop record costs a 576-second re-record. A record staled by an edit nobody has committed is REPORTED on a `discrimination:` line of its own, counted as proven by nothing, and becomes a violation at the commit that carries the edit. HEAD and the tree are compared at the granularity the record FINGERPRINTS, which is the producer or unit FUNCTION: comparing whole files let any unrelated uncommitted edit elsewhere in that file silence the author's own violation |
| A tagged unit the TIP COMMIT added against `HEAD^`, on an enrolled RFC's gated requirement, carrying no verified record | The obligation is what a CHANGE adds. A floor that starts at zero and only forbids going below zero proves nothing. Both sides are COMMITTED (owner decision, 2026-09-01): a tag sitting only in somebody's working tree bills nobody, and `./le verify worktree` checks the commit under test out detached, where a tag that commit added IS the tip |
| A record committed at HEAD, deleted from the tree, while the tag it proved is still there | The proven set only goes up. Deleting a record beside a standing tag takes a proof off the published ledger and leaves the claim behind it |
| Nothing, when a record's TAG is gone | A record dies with the tag it proves, so an orphan has nothing left to be wrong about. It is REPORTED as removable, on a `discrimination:` line of its own, and counted as proven by nothing |
| A functional or interop record citing an assertion its carrier does not contain, or citing none | No generated break reaches either carrier, so the citation is what ties the recorded red to one assertion rather than to the whole suite. An interop citation is checked against the numbers the checker WRITES OUT, so an assertion numbered by expression -- `fail(index+2, err)` inside a loop -- cannot be cited until its checker writes the number |
| An escape whose reason is outside the closed vocabulary, whose precondition no longer holds, or that names code the tagged unit does not reach | An unconditioned reason is the blanket opt-out the escape exists to refuse, and a reason checked over any file an author picks is unconditioned in practice |

The three fingerprints are minted by `sealDiscrimination`, the one place a hash
is computed. `unit-sha` and `producer-sha` hash `behaviorBytes` rather than the
raw text. An unrelated comment, a reflow, an inserted header and a blank line
each leave a record verified; a changed assertion or a rewritten producer voids
it. That is the same predicate `ChangedTags` uses, so a record goes stale
exactly when the obligation says its unit moved, and the re-stamp burden
`rfc/audit/rfc7606.json` records does not repeat here. Measured over this
checkout's own records: a nine-line header prepended to every file they name,
which is the edit that cost that artifact two paragraphs of re-stamping, leaves
every one of them verified.

`claim-sha` is the exception, and it is why the claim is a field of its own. The
claim IS a comment, so `behaviorBytes` strips it, and a proof sealed against a
modest sentence would otherwise survive that sentence being widened with no code
edit at all. `claim-sha` hashes the comment PARAGRAPH the tag opens: the words
after the polarity on the tag's own line, plus every comment line under it, up
to the next tag, an empty comment line, or the first line that is not a comment.
2,701 of this checkout's 3,900 tags carry a claim that runs past the tag's own
line, so one line would leave two thirds of the corpus free to widen. Whitespace
runs collapse to one space, so re-wrapping a sentence changes nothing and
changing a word changes everything. The accepted cost is that rewording a claim,
a typo fix included, stales the record and owes a re-record.

An ABSENT record is not refused. Most tags have never been proven, and that is
a backlog the summary line publishes:

    discrimination: 0 proven, 0 owed, 0 escaped

`proven` counts the records taking a proof route and `escaped` counts the
`no-break` records, which are debt rather than evidence. `owed` is
change-scoped and keyed on the tagged UNIT: a unit the TIP COMMIT added against
`HEAD^` owes its record in that commit, and a unit the tip commit did not add is
grandfathered, exactly as the extraction backlog is. Only a MUST-level
requirement of an enrolled RFC obliges, because that is the population this gate
exists for. Where git cannot answer, `owed` is 0, because a baseline that cannot
be read accuses nobody, and every owed unit is also a violation, so a report
that renders at all renders `0 owed`.

Both sides of that comparison are COMMITTED (owner decision, 2026-09-01). A tag
that sits only in the working tree bills nobody: several sessions share this
checkout, and judging the tree instead put the violation in front of every
bystander and in front of the author never, because `./le verify worktree`
checks the commit under test out DETACHED and a tag that commit added is at HEAD
there. The tip commit is the one change whose author can still record a proof.

A second figure says how much sits BEHIND that obligation, and enforces nothing:

    discrimination: 0 tagged unit(s) carry a tag added since origin/main with no proof recorded.

The line renders only where `origin/main` resolves, because a count taken
against a baseline nobody read is worse than no count. It is the same predicate
as `owed` with the pushed branch as its baseline, so the measurement cannot
drift from the rule it measures. Billing it would be unclearable: the unpushed
set runs to hundreds of commits, its tags were added by sessions that have
finished, and nobody can clear it inside the change in hand.

A third figure sits beside those two and also enforces nothing:

    discrimination: 0 grandfathered tagged unit(s) changed behavior since HEAD with no proof recorded.

The spec answers "which tags owe a proof" twice. R-2 reads it wide -- a tag added
since HEAD, OR a tagged unit whose behavior changed -- and AC-3 reads it narrow,
because its violation names "the stale record", which only a unit that already
has one can have. The narrow reading is what the ratchet enforces, so a
grandfathered tagged test can be gutted today and nothing bills it. The owner's
decision of 2026-08-31 is to DETECT the wide set and PUBLISH it, and to enforce
nothing yet: the count is what says whether enforcing it is affordable, and a
ratchet that reds the tree over a backlog nobody has measured gets removed rather
than obeyed. `discriminationChangedUnits` consumes `ChangedTags`, so a
comment-only, whitespace-only or Go import-only edit counts nothing, and the
population is the narrow obligation's own: a gated requirement of an enrolled
RFC, on a unit HEAD already carried.

The unit rather than its file, since 2026-08-31. A file key bills nothing for a
second tag on a requirement the file already proves elsewhere, which is one of
the routes an over-claim takes.

One further line is published beside those figures, and it is a REPORT rather
than a refusal:

    unscanned: 10 'RFC requirement:' comment(s) sit in production Go on no carrier

Those are tag comments in non-test Go, where no carrier claims them: no gate
resolves the id, no gate demands the polarity, and no gate asks whether anything
runs them. They read as evidence to a person opening the file and are counted by
nothing. Eight of the ten in this checkout carry no polarity at all and would be
refused outright by `parseTagRest` if any scanner did read them. They are
published rather than refused because they predate the check, and a rule that
reds the tree over standing debt gets removed rather than obeyed.

`./le rfc discriminate stem <stem>` and `./le rfc discriminate id <ID>` answer
what one RFC or one requirement has proven, which of its records no longer
verify, and which of its tags carry no record. The gate itself runs no test, no
mutant and no scenario: it reads the recorded proof and compares its
fingerprints, which is what `checkAuditFreshness` already does for a verdict.

## Producing a record: the two proof routes

`./le rfc discriminate-record` is the only writer of a record, and it writes one
only after it has SEEN the red. It applies the break, runs the tagged unit,
requires a failure that NAMES that unit, and refuses everything else. A run that
stayed green records nothing, and a run that went red without naming the unit
records nothing either: a build error, a sibling test and a flake each turn a
run red, and none of them says the claim's own test discriminated the break.

| Route | The break | The runner |
|---|---|---|
| `mutant` | one gomu mutant, substituted into its own line | `go test -run '^<Func>$'` over the tagged unit's package, under a Go `-overlay` |
| `revert` on a `.ci` | the producing function's body replaced by a halt | `ze-test <suite> <name>`, ONE `.ci`, against the isolated set `functional.Prepare` builds under the same overlay |
| `revert` on an interop checker | the same | `./le integration interop` with `INTEROP_SCENARIO` set to the scenario the checker's own `const name` declares |

Adding `report <path>` to `./le rfc discriminate` turns it into a PROPOSER: it
prints the candidate breaks for each unproven unit tag, best first. Two filters
and one ranking. A candidate must be a mutant gomu recorded as KILLED, because
NOT_VIABLE does not compile and SURVIVED is noticed by no test in the package.
It must lie in code the tagged unit's own coverage profile executes, because a
mutant the unit never reaches cannot redden it. The rank is the count of symbols
the tag's own prose names that the break's text touches, which decides what is
offered first and nothing else: the gate never judges whether a break is a GOOD
break.

The break travels in a Go overlay, so no file on disk is modified and a
concurrent session in the same checkout sees nothing. The interop carrier is the
one exception, and it is a fact about that lab rather than a choice: the image
build compiles ze INSIDE Docker from the repository as its build context
(`internal/le/interoplab/docker.go`), where a host-side overlay is a file the
container never sees. There the break goes into the working tree and is put back
byte for byte, which is what `docs/contributing/testing.md` has always said to
do by hand.

## The escape, and the precondition behind each reason

`no-break` says no break exists. That is a claim about the tree, so the gate
goes and checks it, in the shape `checkSuperseded`'s four dispositions already
have. Without a checked precondition the escape would be cheaper than a proof,
and the escaped count would climb faster than the proven one.

Every reason also names what ties it to THIS claim. The fact each one states is
about a FILE or a CARRIER KIND, and neither is about one tag: a declaration-only
file exists in every package, and `interop` is a property of 37 tags at once, so
a reason checked on its own discharges every tag equally. That is the blanket
opt-out wearing a closed vocabulary, and both halves are checked.

| Reason | Claims | The gate CHECKS | The tie to the claim |
|---|---|---|---|
| `foreign-producer` | the behavior is produced by an implementation this repository does not build, so no edit here can falsify the claim | the carrier kind is `interop`, and the record names no producer | the `citation` names a `fail(N, ...)` number the tagged checker WRITES OUT, read by the same `interopCitationState` an interop proof passes |
| `declaration-only` | the code the claim rests on holds no function body: a table, an embed, a registration list | the named producer file declares no function | the tag's own claim names an identifier that file declares, matched whole-word and case-insensitively |
| `generated-producer` | the producer is generated, so a break is undone by the next generator run | the named producer's file carries the `// Code generated ... DO NOT EDIT.` line | the same: the tag's claim names something that file declares |

Both producer-naming reasons owe a third fact, about the FILE the record picked:
the producer must be code the tagged unit REACHES. A Go unit reaches its own
package and the packages its file imports; a `.ci` or an interop scenario runs
the whole daemon, so it reaches every file the Go tool compiles and nothing
under `testdata/`. Without that fact the two above are properties of a file
rather than of this test, and 605 of the 4,020 claims in the tree carry a whole
word that some function-free file somewhere declares (measured 2026-08-31), so
an author who could not prove a claim could go and find the file that fits the
words. A producer naming a function its own file does not declare is refused on
the same ground: every fact here reads the whole file, so an unresolved symbol
would sit in a published record read by nothing.

Coverage cannot supply the tie for the two producer-naming reasons, and that is
measured rather than assumed. A declaration-only file carries no statement, so
`go test -coverprofile` emits no block for it and no profile can ever show the
tagged unit reaching it. The claim is what is left, and `claim-sha` has already
pinned its wording.

One refusal comes before the reason is read: a `unit`-carrier tag whose producer
resolves and sits in a file gomu mutates is REFUSED the escape whatever reason it
offers, because a break can be generated for it and `mutant` is its route. The
`.gomuignore` patterns are read from that file rather than restated. It runs
inside `escapeCheck.verdict` (`internal/le/rfc/discriminate_escape.go`), which is
the GATE's own path: a guard that ran only where records are written would be
invisible to a record authored by hand, and to one whose producer became
mutatable after it was sealed.

So a verified record says the red WAS observed, and that the code it was
observed over has not moved since. It does not say the red would happen again on
a machine that never ran it. Re-observing is `./le rfc discriminate`, which an
author runs deliberately.

## What the ratchets cannot see

A tagged test whose assertions are weakened IN PLACE, while the shape stays the
same, is caught by `checkDiscriminationRatchet` once that test carries a record:
the weakened body changes `unit-sha`, the record stops verifying, and the gate
refuses it. Until a test carries one it is grandfathered, so three other
mechanisms carry the standing corpus:

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

What none of them can see is the tag that OVER-CLAIMED from its first commit.
Each of the three is a CHANGE detector, so a test that never asserted what its
tag says has nothing for them to compare against. That is the hole
`rfc/discrimination/` closes, and it closes it only where a record exists: the
count of tags that carry one is published in `ai/RFC-REQUIREMENTS.md`, under
"Claim discrimination", beside the backlog that does not.

A `test/weakened.md` row is self-service, and it does NOT authorize weakening an
RFC-tagged test.
