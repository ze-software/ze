# The RFC Conformance Gates

`./le rfc check` judges whether Ze's RFC obligations are still extracted,
implemented, proven and publicly declared. This page describes what it checks
and what each check refuses. What an author OWES is in
`ai/rules/rfc-compliance.md`; this page is how the machine measures it.

Every check named here is a function in `internal/le/rfc/`. The package was
ported from a Python tool, so older prose and older commit messages spell these
names in snake_case. The Go names below are the current ones.

The gate's own tests assert decisions over fixture trees: accepted and refused
coverage changes, published partial scope, and audit and discrimination records
that stay valid through a rename. `selftest_test.go` also checks that a failed
fixture produces a named failure and a nonzero result. A hash of the package's
Go source is not a behavioral test: a comment or equivalent rewrite changes it
without changing a verdict. It is not a substitute for those assertions.

`./le rfc selftest` runs its engine fixtures and makes a fresh public `Check`
call over the current checkout. Its unit tests pass owned fixture roots through
the same stage runner and assert a clean tree, a planted coverage violation and
an unreadable tag. Comparing repeated checks of the developer's whole checkout
would make those plumbing tests depend on its changing conformance state and
repeat the tagged-package compilation.

The action grammar and page-rendering tests use owned fixture roots too. The
real-checkout check, every in-scope tag and every recorded discrimination proof
remain corpus-wide assertions. Function boundaries are found in one line walk;
each operation's scope index builds its recorded-key name lookup once per
distinct source text. It retains every match, so an ambiguous name still refuses
a fingerprint.
These indexes hold source structure, not check verdicts, and do not survive the
operation.

The public check still type-checks every package carrying a Go requirement tag.
Its `go vet` child uses the checked tree's feature tags, Go version pin and cache
with the shared toolchain's process cap and isolated test environment. Module
fixtures link their cache storage to the checkout's content-addressed Go cache,
so each temporary root does not rebuild the same dependencies. Check verdicts
are never cached: a fixture changes gated source from compilable to a type error
and back, and asserts each result with the inherited Go settings poisoned.
Refused-rename checks fingerprint regular-file contents and symlink targets,
including their entry kinds; they do not traverse the shared cache.

Native unit observations remove the launching tool's checkout-root and
build-name environment aliases before starting the child. The child discovers
its own checkout from its working directory; the toolchain still supplies its
cache and process limit. The child-process regression in
`internal/le/rfc/native_fixture_test.go` checks the cache directory's identity,
including when the checkout reaches it through a symlink. Neither these
engine fixtures nor a green `rfc check` mean a protocol proof was executed:
the gate reads stored evidence, while `rfc record` runs the
claimed test with and without its break.

## The artifacts

| Path | Holds |
|------|-------|
| `rfc/full/<stem>.txt`, `rfc/drafts/` | The RFC's own text. The source, and the only thing a conformance claim may quote |
| `rfc/errata/<stem>/<number>.txt` | The verified text of one erratum a row cites: its original and corrected text as the RFC Editor publishes them. A row that cites it is quoted against it ("The row quote") |
| `rfc/short/<stem>.md` | The extracted summary, and the ONE place every fact about that RFC is declared. One checklist row per requirement, plus the `## Meta` table. That table states whether the RFC is gated, and what the public page claims for it |
| `rfc/enrolled.txt`, `rfc/not-enrolled.txt` | GENERATED from the Meta tables by `./le rfc update`: which summaries are gated, and the recorded reason for each that is not |
| `rfc/extraction/<stem>.json` | The extraction sign-off: the walk of the RFC text, recorded so a machine can re-check it |
| `rfc/audit/<stem>/<requirement-id>.json` | One requirement's recorded `/ze-rfc-audit` verdict, and the fingerprints that keep it fresh. `./le rfc audit-stamp stem <stem> from <path>` adds new verdicts from a pending file outside `rfc/audit/` and computes their fingerprints. With `mode rejudge` it replaces, in place, a recorded verdict a judge re-made, and accepts the `upgrade_reason` that a `weak`, `wrong` or `partial` verdict raised to `enforced` over unchanged units needs. `./le rfc reseal stem <stem>` re-stamps a recorded one whose unit only shifted, in the stems named and no other (the keyword repeats; with none, or with a stem that is not enrolled, it refuses and writes nothing), because a shift another session's edit caused in an RFC the caller did not touch is not the caller's to re-stamp; it cannot re-judge changed scope. `ResealWithProof`, the module rename's caller, alone walks every stem, bounded by its per-file rename proof |
| `rfc/discrimination/<stem>/<requirement-id>.json` | The recorded breaks under which a tagged unit goes red: one record per requirement, polarity and tagged unit, each requirement's records in its own file |
| `rfc/drain-budget.txt` | The extraction drain schedule: a start date and a rate, and nothing else |
| `docs/features/rfc-status.md` | GENERATED from the Meta tables by `./le rfc update`: the PUBLIC support claim, one row per summary that declares a section. Its `Proof` column is the exception to that: it is derived from the checklist and the tags, and states each stem's gated count partitioned into proven, annotated and untested |
| `ai/RFC-REQUIREMENTS.md` | The generated backlog: the coverage rollup, the audit coverage, the claim-discrimination counts and the extraction sign-off counts |
| `rfc/requirements/<stem>.md` | One RFC's requirement table: six cells per requirement, generated from the summary and the tags |
| `rfc/graph.tsv` | GENERATED from the RFC texts by `./le rfc update`: the dependency graph, one `from`, `kind`, `to` row per in-corpus edge ("The dependency graph") |

The audit verdicts and the discrimination records are stored one file per
requirement, never one per RFC. Several sessions work the same RFC at once, and
a per-RFC file carried every session's uncommitted entries, so a session could
not commit its own verdict without carrying another's or withholding its own.
With one file per requirement, two sessions working different requirements
never write the same file. A verdict's history is its file's git log, so the
audit carries no date and no re-stamp note. Every loader refuses a per-RFC file
of the retired layout, `rfc/audit/<stem>.json` or
`rfc/discrimination/<stem>.json`, and names `./le rfc evidence-split`, which
converts one into the per-requirement files without deleting it: the commit
that carries the converted files removes it (`remove <path>`). `from head`
converts what HEAD holds rather than the working tree, for a file that also
carries another session's uncommitted edits.
<!-- source: internal/le/rfc/evidence_layout.go -- listEvidence, evidenceAt, retiredLayoutErr -->
<!-- source: internal/le/rfc/evidence_split.go -- splitEvidence -->

Six of those paths are DERIVED and none of the six is tracked:
`ai/RFC-REQUIREMENTS.md`, `rfc/requirements/`, `rfc/enrolled.txt`,
`rfc/not-enrolled.txt`, `docs/features/rfc-status.md` and `rfc/graph.tsv`.
`IndexUpdate` (`internal/le/rfc/write.go`) writes all six in one run, and
`internal/le/rfc/register.go` registers each in `internal/le/derived`, so writing
a summary, an audit verdict, a discrimination record or any tag carrier REMOVES
them and a shell command naming one REBUILDS it first. A `git ls-files` over any
of the five answers nothing, and `./le commit create` refuses a commit that
carries one. `./le rfc check` reads the summaries, the tags and the audits
rather than any of the five, so no gate compares a re-render against a committed
copy.

`reference/` holds non-normative IETF documents (BCPs, Informational RFCs,
active drafts) and is never a requirement source. `./le rfc check` refuses a
summary or an extraction sign-off whose stem has a text under `reference/` and
none under `rfc/full/` or `rfc/drafts/`. It also refuses a summary that cites a
`reference/` path. Only the owner moves a document into `rfc/full/` or
`rfc/drafts/`, as `reference/README.md` states.
<!-- source: internal/le/rfc/check_reference.go -- checkReferenceSources -->

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
tell "never proven" from "stopped being proven". Nine comparisons against git
supply that difference, against HEAD or, where the change the commit under test
made is what is judged, against `HEAD^`. Each fires only on a real downgrade, so a green run
means the evidence held rather than that nobody looked.

| Ratchet | Producer | Fires when |
|---------|----------|-----------|
| Enrolment is monotonic | `checkEnrolment` | an RFC whose MUSTs were gated stops being gated |
| Proof is monotonic | `checkCoverageRatchet` | a requirement loses a polarity it had at HEAD. A `{gap}` an author writes is NOT an escape: it is the move being blocked. The one accepted loss is a loss the owner ruled, and it needs both halves citing one ruling: every unit that held the lost polarity at HEAD has an `./le rfc approve` row in this commit session whose reason names `owner ruling <id>`, and the row now carries an annotation citing the same `owner ruling <id>` that states what it keeps. For a tag move (OWNER RULING 6(b), 2026-10-02) that is `{single-polarity: <the polarity it keeps>; ... owner ruling <id> ...}`. For an absent feature the owner ruled a gap (OWNER RULING 8(g), 2026-10-02) it is `{gap: ... owner ruling <id> ...}`, and the row may then lose both polarities, since a `{gap}` row carries no tag. `<id>` is the ruling's date or number, matched case-blind (`ownerRuledMove`, `coverage_ruling.go`). Either half alone is refused, because each is an edit an author makes unaided. `./le rfc check` reads the rows of the session it runs in, so the move passes where the author runs it and in that session's verify; the approval reaches the commit as an `RFC-approved:` trailer and is dropped from the file once the commit lands, when HEAD no longer holds the lost tag |
| Gating is monotonic | `checkLevelRatchet` | a requirement leaves the MUST-level population, because its level was gated at `HEAD^` and is advisory now. The baseline is `HEAD^`, not HEAD, so the ratchet sees a demotion the commit under test made in the detached verify worktree. That is the cheapest route from red to green, cheaper than `{gap}` and cheaper than deleting the row, because the id and the tests survive while every coverage obligation attached to the row disappears. The one escape is a `Correction <YYYY-MM-DD>:` paragraph in `rfc/corrections/<stem>.md`, naming the id and quoting at least 24 characters of the RFC verbatim, matched against the same page-stripped haystack as a row quote. The record lives beside the summary rather than in it, because a summary carries what the RFC obliges and not a log of what this repository once got wrong. A row GAINING a gated level is never reported |
| Requirements do not vanish | `checkRetiredRequirements` | a requirement id of an enrolled RFC that `HEAD^` holds disappears from its summary. Without this, deleting the checklist line is cheaper than `{gap}`, which costs a public disclosure row, and the ratchet would pressure people to hide obligations rather than declare them. Correcting a misquote means editing the TEXT under the same id, which is allowed. The one accepted disappearance is a row no sentence of the RFC states: a `Retired <YYYY-MM-DD>:` paragraph in `rfc/corrections/<stem>.md` names the id as its first backticked id, and every section read as `§<n>`, after the tags have moved. It retires that first id only, so the row a tag moved to can be named after it (`retiredIDs`, format in `rfc/corrections/README.md`). A `Correction` paragraph does not retire a row, and `checkIDAllocation` refuses any row that carries a retired id again |
| Adding an RFC adds checking | `checkNewSummaries` | a summary NEW since `HEAD^` declares gated MUSTs and does not declare itself enrolled, fails to parse, or captures zero requirements while `rfc/full/<stem>.txt` has MUST-level keywords. A document's own RFC 2119 key-words paragraph does not count, and neither does its reference-list entry for RFC 2119 or RFC 8174: both say where the words come from, and neither binds anybody |
| Non-unit evidence is monotonic, per tier | `checkEvidenceRatchet` | a requirement loses an evidence KIND it had at HEAD: its `.ci` becomes a unit test, or a verify-tier binding is swapped for a nightly-tier interop one. Keyed by `kind/tier`, so a substitution leaving the tag COUNT unchanged still fires. A unit test proves the algorithm; only a running functional or interop test proves the daemon or a peer. No annotation satisfies it |
| Extraction is monotonic | `checkExtractionRatchet` | a stem that carried a sign-off at `HEAD^` carries none now, or a signed stem's exclusion count RISES over its `HEAD^` count without a `resign-reason` and a bumped `signed-off` date. The baseline is `HEAD^` (`baselineExtractions`), so the ratchet sees what the commit under test did in the detached verify worktree. The first stops the bound being un-bound by deleting a file; the second stops the exclusion list becoming a hatch where every unmapped site is excluded with a shrug |
| An exclusion carries its evidence | `checkExclusionEvidence` | a `{not-applicable}`, `{feature-declined}` or `{lower-layer}` row the commit added or changed against `HEAD^` lacks the evidence its kind owes, the count of untouched rows lacking it differs from `rfc/exclusion-evidence-backlog.txt`, a line of that file rose against `HEAD^` or the pushed `origin/main`, the working-tree copy of that file is malformed, or `HEAD^` cannot be read; `checkZeRolesInText` refuses a declared Ze role its RFC never names. Below "The exclusion evidence ratchet" |
| A claim keeps its proof, and a new claim owes one | `checkDiscriminationRatchet` | a tagged unit the tip commit added against `HEAD^` carries no discrimination record, a record committed at HEAD is deleted while its tag stands, or a recorded proof no longer verifies against the tree. This is the only ratchet that reads the PROSE half of a tag: `claim-sha` fires when the sentence is reworded, because a proof of the old claim is not a proof of the new one |

`checkIDAllocation` (`internal/le/rfc/check_ratchets.go`) compares against
`HEAD^`, for requirement id allocation. `checkAuditVerdictRatchet` and
`checkAuditFindings` (`check_audit.go`) compare against `HEAD^` too, for
recorded audit verdicts, through `baselineAudits`: a verdict the commit under
test deleted, or a finding it turned into `enforced`, is seen in the detached
verify worktree.

Requirement IDs are permanent. `parseChecklistLine` checks every row's ID form,
summary stem and positive ordinal, and `ParseSummaryText` rejects duplicate IDs.
`checkIDAllocation` requires an ID known to be absent from the `HEAD^` baseline
to match the row's cited section, using `x` when it has no citation. A readable
baseline with no requirements still enforces this rule. The baseline is
`HEAD^` and not HEAD because Thomas's release check, `./le verify worktree`, checks the commit under test
out detached, where the tree equals HEAD: a HEAD baseline already held every id
that commit added, so the guard judged nothing at the one gate that runs. The
id, level and retirement ratchets share that baseline, `baselineLevels`
(`internal/le/rfc/check_baseline.go`). When `HEAD^` does not resolve or the
baseline cannot be read, allocation comparisons judge nothing; when only one summary's
blob is missing or unparsable, only that summary's allocation history is unknown.
The baseline reader carries these states to the guard without another Git probe,
and structural validation still runs over every current row.

Once allocated, the ID stays unchanged when its citation is corrected:
`RFC1334-2.2-1` can correctly cite §2.2.1 without losing the checklist or its
existing test references. The correction grants no coverage or extraction
evidence. Retired IDs remain unavailable, and new ordinals must exceed the
high-water mark for the section encoded in the ID, even when an existing row's
citation has moved elsewhere.

Summaries that predate HEAD are the existing backlog and are deliberately
grandfathered. A rule that reds the gate on unrelated work gets removed rather
than obeyed. Where git cannot answer, every ratchet judges nothing rather than
judging everything.

The enrolled baseline is `baselineMetas` (`internal/le/rfc/check_baseline.go`).
It parses the `## Meta` table of every summary `HEAD^` holds, for the reason
`baselineLevels` reads `HEAD^`: in the detached verify worktree the tree equals
HEAD, so an un-enrolment, a new unenrolled summary or a dropped public row made
by the commit under test was already in a HEAD baseline. `baselineSummaryStems`,
which `checkNewSummaries` reads, is at `HEAD^` for the same reason. A summary
that does not parse there is skipped rather than emptying the baseline. One
unreadable file at `HEAD^` must not empty every ratchet's population.
`checkRetiredRequirements`, `checkLevelRatchet`, `checkCoverageRatchet` and
`checkEvidenceRatchet` run only where the current enrolled set intersects that
baseline. A baseline nobody can read disarms all four.

`baselineMetasBeforeMigration` reads the retired `rfc/enrolled.txt` and
`rfc/not-enrolled.txt` out of GIT HISTORY. It is reached only when no summary at
`HEAD^` declares an enrolment at all. That is the ability to compare against a
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

### The exclusion evidence ratchet

A row leaves Ze's own count only with checked evidence (owner ruling,
2026-10-29): the RFC sentence that settles it, read in `rfc/full/<stem>.txt`
at the section the row cites, and the role condition or the layer behind it.
The row's own text is already the RFC's requirement sentence ("The row
quote"); what each kind adds is the fact that removes the row from Ze's count.
`(*evidenceReader).gap` (`internal/le/rfc/check_evidence.go`) judges it:

| Kind | Evidence | Written as |
|------|----------|-----------|
| `{not-applicable}` | A role-condition quote from the cited section or its subsections and the specific role that sentence binds, which is none of the roles the summary declares Ze fills | `{not-applicable: "<the RFC's sentence scoping the obligation to a role>"; binds <the specific role the sentence names>; <why that role is not one ze fills>}`, with Ze's roles declared once per summary in its `## Meta` table as `\| Ze roles \| <role>, <role> \|`. Quote first, as in `{feature-declined}`. `parseNotApplicable` refuses a quoted payload with a quote under 24 characters, no `binds <role>` segment, or no reason. The quote is one of two shapes, and any other is refused naming both (`notApplicableShapes`). Shape (a) is the row's own obligation sentence: it holds a BCP 14 keyword in upper case (every level `rfc.go` declares, read from `gatedLevels` and `advisoryLevels`), and the row's quote, its text without the trailing `(§N)` (`Requirement.Quote`), holds it or it holds the row's quote, compared as a row quote is. The quote's first keyword must lie inside the span the row's quote occupies in it and must be the row's level, so a row stating the second clause of `"A route reflector MAY keep the widget, and a speaker MUST send the widget to every peer."` cannot bind a route reflector (`ownObligationRefusal`, in `(*evidenceReader).roleQuoteGap`). When the row's quote holds the quote, the row's first keyword of its level must also lie inside the span the quote occupies in the row, so a row `"A speaker MUST send the widget first. A route reflector MUST keep the widget it reflects."` at MUST cannot quote its second sentence to bind a route reflector (`levelKeywordAt`). Each refusal names the one that failed: the row's quote that neither holds, the keyword outside the row's clause, the keyword that is not the row's level, or the row's own obligation outside the quoted sentence. Its subject, the words before the first keyword, must be the bound role with only determiners or quantifiers (`roleDeterminers`) before it or between it and the keyword (`subjectRefusal`; `"A route reflector MUST NOT reflect ..."` binds a route reflector, and `binds the widget` or `binds reflector` against `"Every remote route reflector MUST ..."` is refused). A subject holding a comma or one of `subjectBreakers` (and, or, nor, that, which, who, of, from, to, by, with, not) is refused, since it names more than one party or qualifies the party by a clause: rfc9915's `"Clients and servers MUST discard any received Relay-reply messages."` cannot bind servers. A subject that is a pronoun (`It MUST ...`, `They all MUST ...`, `rolePronouns`) is refused. Shape (b) is a scoping sentence of the cited section with no keyword that carries one of `scopingMarkers` as whole words (apply, applies, applicable, only, solely, limited to, restricted to) and none of `scopingNegations` (not, no, except, unless, other than, neither; `"This section does not apply to route reflectors."` is refused), and the bound role need only be in it as whole words (`scopingRefusal`; `"This section applies only to relay agents."` binds relay agents, and `binds relay ag` is refused as not in the quoted sentence). Either shape must be a whole sentence of the cited section or a subsection: from a sentence start to its terminator, cut by the inventory's own splitter over the page-stripped paragraph, so a line wrap or a page break does not matter and a fragment that cuts the keyword away from its condition is refused (`(*quoteSource).wholeSentence`), and cut again where a full stop and a blank are followed by a lower-case word, unless the word before the full stop is one of `quoteAbbreviations` (e.g., i.e., etc., sec., cf., vs., ed., al., no., fig., viz., approx., resp., esp., min., max., inc., corp.), a single letter, a number, or holds another full stop; a leading list marker (`o`, `-`, `*`, `+`, `(a)`, `a)`, `1)`) is cut from both the sentence and the quote (`quoteSentences`), so RFC 1812's bullet `"A router MUST receive and process normally any packets with a broadcast destination address."` and a sentence opening `"iBGP speakers MUST ..."` after a full stop are whole. The cut is the quote check's alone: the inventory's site numbering still reads its own splitter. A bound role made only of words for any party (`genericRoleWords`: implementation, node, host, system, entity, device, party, peer, speaker, router, endpoint, sender, receiver), once the adjectives that say how a party behaves rather than which role it fills are set aside (`nonRoleAdjectives`: compliant, conforming, conformant, robust, receiving, sending, transmitting, originating, new, existing, standard), is refused before the quote is judged (rfc2784's `"Note that a compliant implementation MUST accept and process this field."` cannot bind a compliant implementation), since `binds an implementation` excludes Ze nearly anywhere; the refusal advises a role the RFC defines (a route reflector, a DHCP relay agent). A bound role of determiners or quantifiers alone (`binds all`) is generic too, and one made only of pronouns, determiners, scoping words or negations (`binds this only`, `binds them`) is refused as naming no party (`roleNamesNoParty`). `checkNotApplicableRoles`, in `ParseSummaryText`, reads the summary's `Meta.ZeRoles` once and refuses a summary holding a quoted row and no `Ze roles` row, naming the row to add; a bound role equal to, holding or held by a declared Ze role; and a bound role whose head noun is generic and whose qualifiers all appear in one declared role (`binds a BGP implementation` with Ze a `BGP speaker`, `qualifiedGenericRole`). Roles compare as whole words without case or the punctuation around a word (`binds a route reflector (RR)` is `route reflector rr`, as the subject is), leading determiners and quantifiers (`roleDeterminers`: a, an, the, this, that, these, those, each, every, any, all, other, some, local, remote) or a trailing plural, `-ies` read as `-y` (`binds BGP speakers` with Ze a `BGP speaker` is refused, and so are `binds each remote BGP speaker` and `binds a BGP speaker` with Ze a `speaker`, and `binds proxies` with Ze a `proxy`; `binds a server` with Ze an `observer` passes). `zeRolesFrom` refuses a `Ze roles` entry that is empty, has no letter (`-`) or is a placeholder (`zeRolePlaceholders`: none, nothing, n/a, na, tbd, todo, unknown), and `checkZeRolesInText` refuses, for a summary holding a quoted row, a declared role its `rfc/full/<stem>.txt` never names as whole words after the same normalisation, or an RFC text that is absent: a role the RFC never uses is held by no bound role, so every row would pass against it. A reason that opens with no quote is a row written before the kind carried evidence: it parses, needs no `Ze roles` row, and the ratchet owes it |
| `{feature-declined}` | Its quote, a whole sentence in the cited section or its subsections, or in the exact section the payload names, of one of three shapes: a sentence carrying an optionality marker, that sentence from a named section, or the row's own sentence as a conditional whose obligation a clause on the feature's use guards, with a never clause on that condition naming a producer the tree declares; or, in the heading form, feature words a title of the row's section or of a section above it names, with the same never clause | `{feature-declined: "<quote>"; <reason>}`, or `{feature-declined: §<section> "<quote>"; <reason>}` when the RFC makes the feature optional in another section, or `{feature-declined: heading "<feature words>"; ze never ...}`; see "The feature-declined annotation". Never counted: `parseFeatureDeclined` requires the quote, or in the heading form the feature words, so every row carries its evidence form; one whose quote is outside the section it is held to, or is not a whole sentence of it in one of the three shapes, and a heading form whose words no title above the row names, are refused by id |
| `{lower-layer}` | A layer component a reader can check, beside the producer | `linux:CONFIG_<option>`, `go:<package>` or `vendor/<module>` inside the head, see "The lower-layer annotation" |
| `{linux-limit}` | Its register entry | unchanged, `checkLinuxLimit` |

"Ze does not implement X" is never `{not-applicable}`: it is a `{gap}`, or
`{feature-declined}` for a feature Thomas named.

The two shapes are decided rules, not a grammar, so they refuse some honest
rows: an obligation in the passive voice ("The widget MUST be sent by relay
agents."), a keyword used as an adjective ("Relay agents are REQUIRED to ..."),
a subject with an interposed clause ("A relay agent, when it receives a
widget, MUST ..."), a scoping sentence that names its role through a
negation or an exception ("This section applies to every speaker except
route reflectors."), and any quote holding a `"`, which the annotation's own
quotation marks cannot carry. Such a row has two honest outlets: quote a
scoping sentence of the cited section, or record the row as `{gap: ...}` and
test it. A refused honest row is never forced into a different exclusion kind.

```
| Ze roles | BGP speaker, route server |
- [ ] [RFC9999-2-1] [MUST] A route reflector MUST send the widget before it sends any frame that depends on it (§2) {not-applicable: "A route reflector MUST send the widget before it sends any frame that depends on it."; binds a route reflector; ze reflects no route, so the reflector's order binds nothing it does}
```

The rows that predate the rule are a backlog, so the check is a ratchet over
`rfc/exclusion-evidence-backlog.txt`, which holds one `<kind> <count>` line for
each counted kind, `{lower-layer}` and `{not-applicable}`. `{feature-declined}`
has no line: its rows always carry their quote, so each is judged and refused
by id, and the legacy rows whose quote sits outside the cited section stay red
until they are backfilled:

- A row added, or whose cited section, kind or reason changed, since `HEAD^`
  or since the pushed `origin/main` (`evidenceFingerprint`, through
  `baselineLevels` and `pushedFingerprints`) is refused by its id when it lacks
  the evidence, and is not counted: it is the commit's to fix, not the
  backlog's. A row is counted only when both revisions record it unchanged, so
  one commit that backfills a row and adds a lacking one, keeping the line
  level, cannot let the next commit carry the added row as backlog
  (`evidenceRowIsNew`). A clone with no `origin/main` judges against `HEAD^`
  alone. The refusal names the summary line, the id, the evidence missing, and
  ends with the kind's format.
- A row of a summary `HEAD^` could not parse is judged against `origin/main`
  alone, and is new, so refused by id, when that branch records none either:
  a summary nobody could read never shelters a changed row.
- A row that carries its kind's evidence form (a quote, or a layer path) and
  whose evidence no longer passes its check is refused by its id, naming the
  failing check, and is never counted, even when the commit left it alone: its
  RFC text or the component it names changed under it. So a commit cannot
  break one row's evidence while it backfills another and keep the line level.
- Every other row lacking evidence is counted per kind, and the count must
  equal the committed line. A count below it is refused until the commit
  lowers the line to the new count. A count above it means an earlier commit
  set the line below the rows it counts: the refusal names the counted rows
  (up to `countedRowsNamedMax`) and says how many to backfill. It never advises
  raising the line, which the next rule refuses.
- The committed line is also read at `HEAD^` and at the pushed `origin/main`
  (`priorEvidence`, `committedBacklogAt`), and a line above either is refused,
  naming the kind, both counts and the commit read. So the committed number
  never rises against `HEAD^` nor against the pushed branch, and a rise one
  commit made is still refused in the next. Each is read from the newest
  commit at or before that revision whose copy of the file parses, so deleting
  the file in one commit and restoring it higher in the next is refused too,
  and so is breaking it in one commit and raising it in the next. A committed
  copy is read leniently: a line of a kind the ratchet no longer counts
  (`retiredEvidenceKinds`, `{feature-declined}`) is skipped, so every copy
  written before that kind left the file still parses, while the working-tree
  copy is refused for such a line (`parseHistoricalBacklog`). Only a history
  that never held a copy that parses compares nothing, and `./le rfc check`
  prints a line saying so for each half that compares nothing: `HEAD^`'s
  history, and `origin/main` when it is present but never held a copy that
  parses. A clone with no `origin/main` compares with `HEAD^` alone, and the
  check prints a line saying so too. Absence is `git show-ref --verify --quiet
  refs/remotes/origin/main` failing; a ref that exists and names no commit git
  can read is refused, telling the author to `git fetch origin`.
- A kind whose line reads 0 is refused row by row: full refusal, with no
  removal commit needed.
- A missing file, a missing kind, a kind on two lines, an unknown kind or a
  malformed count in the working tree's copy is refused; a ratchet with no
  committed count judges nothing. A malformed copy at `HEAD^` or `origin/main`
  is not refused again: the commit that wrote it was refused when checked, the
  commit repairing it must pass, and the counts compare with the newest copy
  that parses.
- A `HEAD^` git cannot read is refused (`priorRevisionState`): a broken reader
  never passes as an empty history. Absence is decided from
  `git rev-list --parents -n1 HEAD`, never from an exit status git also gives
  for a lost object: a HEAD with no parent listed, a HEAD that names no commit
  yet, or no `.git` above the tree is absent. A shallow clone, a listed parent
  whose commit git cannot read, or a git command that fails is unreadable, and
  the refusal ends with the fix for that reason: `git fetch --unshallow`,
  `git fetch` or `git fsck` for a lost object, or repairing the checkout until
  the named command answers. Only an absent `HEAD^` judges every row as
  untouched, and `./le rfc check` prints a line saying so.

<!-- source: internal/le/rfc/check_evidence.go -- checkExclusionEvidence, evidenceRowIsNew, evidenceReader -->
<!-- source: internal/le/rfc/check_evidence_prior.go -- priorEvidence, priorRevisionState, committedBacklogAt, pushedFingerprints, pushedBranchPresent -->
<!-- source: internal/le/rfc/summary.go -- checkNotApplicableRoles, genericRoleWords, roleKey -->
<!-- source: internal/le/rfc/notapplicable_roles.go -- scopingRefusal, scopingNegations, roleNamesNoParty -->
<!-- source: internal/le/rfc/check_evidence.go -- roleQuoteGap, ownObligationRefusal -->
<!-- source: internal/le/rfc/quote_sentences.go -- quoteSentences, quoteAbbreviations -->
<!-- source: internal/le/rfc/check_evidence.go -- parseEvidenceBacklog -->
<!-- source: internal/le/rfc/summary.go -- parseNotApplicable, parseLowerLayer -->
<!-- source: internal/le/rfc/check_core.go -- featureDeclinedQuote -->

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

A row whose aggregate declares `| Shape | brief |` (the `drafts` aggregate)
renders in a four-column table with no Remaining cell, and its
`Support remaining` must be `-`. Its Note, the `Support coverage` cell, is the
cell the public page prints there, so `rowsFrom` reads the Note as that row's
Remaining: a `{gap}` or `{partial}` on a draft is disclosed in the Note, and
`checkGapCountAgreement` counts a number spelled there.
<!-- source: internal/le/rfc/meta.go -- rowsFrom, checkRowCells -->

### Profiles and aggregates

Every summary names the technology aggregate it belongs to. A summary with a
public row writes it in the `Support` cell (`| Support | bgp-core |`); a summary
with no row writes `| Support | - |` and names it in an `Aggregate` cell
(`| Aggregate | tunnels |`), so it still joins its aggregate's rollup. The
retired `<section> <rank>` form is refused, and so is an `Aggregate` cell
beside a `Support` cell that already names one.

A summary counts in its aggregate's required figures unless it also writes
`| Aggregate tier | optional |`. That row moves it into the aggregate's
optional tier: its rows leave the aggregate's required figures and count in
the optional tier's own rollup, the way a use-case profile's optional members
do. The row qualifies the aggregate the `Support` or `Aggregate` cell already
names rather than naming one, so an aggregate is declared once. `optional` is
its one value: `required` is refused because an absent row says it, and any
other word is refused rather than read as required (`readAggregateTier`). The
owner lists an RFC as optional this way when it is not a priority for the
first release, and the summary's prose says why.
<!-- source: internal/le/rfc/meta.go -- readAggregateTier -->

The aggregates are files, `rfc/profiles/<name>.md`, one `| Field | Value |`
table each, opening with `# Profile: <name>`. `./le rfc check` loads them
before the summaries (`LoadProfiles`), and a summary naming an aggregate with
no file, or carrying a cell its aggregate's shape does not render, is refused
naming the file and the declared aggregates (`checkAggregate`). A profile that
does not parse is reported like a summary that does not parse.

| Kind | Fields |
|------|--------|
| `technology` | `Title`, `Heading` (the status page heading), `Intro` (prose under it, or `-`), `Shape` (`wide` for the six-column table, `brief` for the four-column one), `Release` (`n/a`) |
| `use-case` | `Title`, `Release` (`yes` marks a release profile) |

A use-case profile lists no member. Each summary declares its own use cases in
two mandatory Meta rows: `| Use cases | route-server, looking-glass |` names
the profiles that require it, counted in their figures, and
`| Optional use cases | lns |` names those that list it as optional, shown
with its own figures and never counted. Each is `-` when empty. Bringing an
optional RFC into scope is moving its name from one row to the other. A
release profile owes exactly the RFCs declared for it: a normative citation is
never an obligation (`ai/META-DECISIONS.md`, "Goal and scope").

`ParseMeta` refuses a missing row, an empty cell, `-` mixed with names, a token
that is not one lower-case hyphenated name, a name written twice in one row,
and one name in both rows (`readUseCases`), and a near-miss label such as
`Use-cases` or `Profiles` (`refuseNearMiss`). Once the profiles are loaded, a name with no profile file, or
one that is a technology aggregate, is refused naming the summary and listing
the use cases (`checkUseCases`). `./le rfc check` also refuses a use-case
profile no summary requires, an optional-only one included, because its
figure would read 0 of 0 (`checkUseCaseMembers`), and a profile file still
carrying the retired `Roots` or `Absent` row, naming the two summary rows that
replaced it (`parseUseCase`). Each refusal is collected, so one summary midway
through an edit never stops the check for the others.

The status page renders one section per aggregate, in name
order, with rows by dependency level (see "The dependency graph"), then RFC
number with the non-RFC stems after them by name; a summary whose text is not
in the repository has no level and sorts after the ones that do. No order is
authored, so no two summaries can claim one place.
<!-- source: internal/le/rfc/profile.go -- LoadProfiles, parseProfile, parseUseCase, parseAggregate -->
<!-- source: internal/le/rfc/meta.go -- readSupport, checkAggregate, readUseCases, refuseNearMiss, checkUseCases, checkUseCaseMembers -->
<!-- source: internal/le/rfc/render_ledger.go -- renderStatusPage, placeRows, statusRowCompare -->

### Coverage figures

Each RFC has two figures: MANDATORY over the gated levels (`IsGatedLevel`) and
OPTIONAL over every other level. Each is rows proven over rows owed. A row is
proven only when a tagged unit covering it carries a discrimination record
that still verifies and is not the `no-break` escape
(`DiscriminationVerdict.Proven`, the one per-row predicate). The figures use
the same row partition as `ProvenShareOf` (the 2026-09-02 ruling), proven only
by a verified record: `{rollup}` rows are out of both sides, and every other
annotated row stays in the denominator. `{single-polarity}` is the one
annotation a row can be proven under, and only by a verifying record on its
one side. A `{not-applicable}`,
`{lower-layer}`, `{feature-declined}` or `{linux-limit}` row is never proven by Ze,
whatever records its tags carry, so annotating a row cannot raise its figure; a
proving record on a gated `{lower-layer}` row counts it OS verified instead, a
count apart (see "OS verified"). A `{gap}`
or `{partial}` row stays in and is never proven either, because the summary
itself says the obligation is unmet: that is what keeps a requirement only
unreached code meets at 0%.

Each figure also carries a CEILING: attainable rows over the same total, or
`(Total - excluded) / Total`. The annotation declaration's `CeilingExcluded`
trait excludes `{linux-limit}`, `{lower-layer}`, `{feature-declined}` and
`{not-applicable}` from attainable rows, never from Total. Gaps and partials
remain attainable future work. This ceiling describes current classifications,
not an immutable platform limit: not-applicable and feature-declined decisions
can be revisited. Each excluded kind has its own count so Linux limits are
never hidden among classification decisions.

`Figure` JSON retains `proven` and `total`, adding `attainable` and `excluded`
(positive counts keyed by annotation kind). Tree rows expose
`mandatory-attainable`, `optional-attainable`, `mandatory-excluded` and
`optional-excluded` beside the existing proof fields. `TreeRow.CeilingCells`
renders these counts; `CeilingOf` sums walked rows and identifies the lowest
nonempty mandatory and optional RFC ceilings. Equal ceilings choose numbered
RFCs in numeric order before other stems, which sort by name. A zero total
renders `no rows`, not 100%.

An RFC with no valid extraction sign-off renders `not walked` rather than a
figure. Every RFC a profile lists has a summary (owner: "all RFC should have a
valid summary"), so there is no third, summary-less state. A rollup over one
tier of a use-case profile, or over an aggregate, sums its walked members and
states how many are not walked. It is never complete while that count is
above zero, nor while the set has no member: 0 of 0 measures nothing. A
use-case profile's percentage is the rollup of its required tier alone; its
optional tier carries a rollup of its own beside it.
The same walked-only rule applies to ceilings and per-kind exclusion counts.
Rollups sum counts before dividing, never average RFC percentages. An unwalked
RFC publishes neither proof nor ceiling measurements; its raw JSON counts are
retained under the existing `walked: false` convention.

Beside the two figures, each RFC carries two derived facts (`StemFigures`).
`MandatoryByKind` counts its proven mandatory rows by the strongest evidence
kind among the tags covering each row: interop, then functional, then editor,
then unit (`evidenceStrengthOrder`, `carriers.go`). Every proven mandatory row
lands in one kind, so the counts sum to the mandatory proven count, and a row
whose tags no carrier claims lands under `unknown` rather than leaving the sum
short. A gap demonstration tag is no evidence of proof and is skipped. `Unmet`
lists the ids of the owed rows, at every level, that the summary or the audit
says are unmet: an annotation whose declared traits say unmet (`{gap}`,
`{partial}`, `{linux-limit}`), or a recorded audit verdict of `wrong`,
`unimplemented` or `partial` (`auditVerdictUnmet`, the same set the ledger must
disclose). The list is the summary's own disclosure, so an unwalked RFC lists
it too. `StemFiguresOf` derives all of it from one rendered corpus, and every
surface that counts proof reads it.

The published proof share (`ProvenShareOf`, `internal/le/rfc/provenshare.go`)
sums these figures over the RFCs Ze implements (`Implements`): enrolled, which
already requires an Implementation of `ze` or `mixed`, and rendering a public
row. An enrolled RFC whose feature is still to come counts at its real figure
(owner, 2026-10-30). The numerator is the verified-record figure alone. The
share is a rollup over that set and follows the rollup rule above, through
`rollupOf` itself: only walked RFCs add to the numerator and the denominator,
and the share states how many RFCs it leaves out (`NotWalked`). An unwalked
list is unbounded, so a denominator holding its rows would measure the list,
not the software. Every surface that prints the share prints that count beside
it: the `/quality/health/` proof-density detail, the home page card
(`rfc.not_walked_display` in the site facts) and the headline card of the RFC
compliance page. The satisfaction buckets on that page still partition every
gated MUST of the RFCs Ze implements, walked or not, so their total is their
own sum, and it exceeds the share's denominator by the rows of the unwalked
RFCs.
<!-- source: internal/le/rfc/profilecoverage.go -- figuresByStem, rowCounts, rollupOf, Rollup.mandatoryComplete, StemFiguresOf, figureEvidence -->
<!-- source: internal/le/rfc/provenshare.go -- ProvenShareOf, Implements -->

### One declaration for annotation meaning

`annotationKinds` in `internal/le/rfc/rfc.go` declares each coverage kind's
parser and traits together. `AnnotationKinds` derives the accepted vocabulary;
`AnnotationTraitsOf` supplies the same meaning to coverage, rollup derivation,
public-status checks, audit legality, site buckets and health accounting. A new
kind does not add another list of exceptions in each reader.

The traits keep distinct questions separate: whether a row belongs in a
denominator, whether a verified record may prove it, whether polarity tags
contradict it, whether it is unmet, which audit verdicts agree with it, and
what it contributes to a rollup. Parser-specific payload checks still belong
to the registered parser and the gate that verifies that payload.

Unknown kinds are refused by the parser and the coverage and audit gates.
Render-only arithmetic keeps an unknown row in the denominator without proof
credit; the site records it as unmapped. It never inherits an exemption from
the zero value of a missing declaration. The existing annotation counts and
published shares are unchanged by this shared declaration.
<!-- source: internal/le/rfc/rfc.go -- annotationKinds, AnnotationKinds, AnnotationTraitsOf -->
<!-- source: internal/le/rfc/check_core.go -- evaluate, rollupTargetState -->
<!-- source: internal/le/rfc/check_status.go -- checkStatusAgreement, checkGapCountAgreement -->
<!-- source: internal/le/rfc/check_audit.go -- verdictClaims -->



| Guard | Refuses |
|-------|---------|
| `checkSummaryDisposition` | a `non-normative` reason that judges what ZE owes rather than what the DOCUMENT states, or that cites nothing a reviewer can check. `non-normative` is the one disposition that claims anything about conformance. Its reason rests on the document: the IETF category, an RFC 2119 / RFC 8174 / BCP 14 key-words paragraph, or a capitalized MUST/SHALL/REQUIRED scan of the source |
| `checkSourceRestricted` | a `source-restricted` reason that names neither the body publishing the standard (ISO, IEC, ITU, IEEE, ANSI, ETSI) nor the license, copyright or paywall that stops the text being copied, and the same kind written over a text that IS in the tree. It excuses no public support claim: being unable to bound a claim is a reason to stop making it, not a reason to be excused from proving it. It is the only PERMANENT disposition: where the text IS fetchable the kind is `blocked`, and a fetch discharges it |
| `checkUnprovenSupport` | two shapes of a claim nothing behind it can contradict. A support claim over a summary that declares ZERO gated requirements, where a claim is any Status other than `Unsupported` or `Future`, an empty cell included: a claim and a checklist that agree on NOTHING is the cheapest way to look green. Two escapes exist there, and each is evidence rather than assertion. They are a `non-normative` disposition whose reason states a property of the text, and a VALID `manual-walk` sign-off with a `register-reason`. The second one lets an Informational RFC that invokes RFC 2119 nowhere enrol on an honest zero. The other shape is a row PROMISING conformance -- `Supported`, alone or with a scope after it -- over gated requirements of which not one carries a both-polarity test. Neither escape reaches it, because both answer whether the DOCUMENT imposes a MUST rather than whether Ze meets one, and `Partial` is the row that states what is true |
| `checkPublicRowMonotonic` | a `Support` cell that named an aggregate at HEAD and reads `-` now, while the summary is still there, and a newly enrolled RFC that arrives with no row at all. It is keyed on the ROW, never on enrolment, because `checkSupportedSignoff` bills any row whose Status promises conformance. RFCs enrolled before it existed are grandfathered, so the count of enrolled RFCs with no row can only shrink |
| `checkLowerLayerProducer` | a `{lower-layer}` annotation whose producer this checkout cannot show: the file is absent, or it declares no function of that name. The kind rests on a fact a reader can open, and a producer that was renamed or deleted under the annotation is the event this catches |
| `checkFeatureDeclined` | a `{feature-declined}` annotation whose quote is not in the RFC's own text, whose RFC has no text in this repository, or whose producer this checkout cannot show. Two facts, because the kind makes two claims: the DOCUMENT makes the feature optional, and ZE built the narrower thing |
| `checkLinuxLimit` | a `{linux-limit}` annotation whose register is unreadable or malformed, whose entry is missing, is not a verified blocking kernel limitation, names another requirement, lacks evidence, or was checked at a kernel version other than the shipped one |
| `checkGapCountAgreement` | a Remaining cell whose spelled number (including zero), sitting immediately before MUST or SHALL, disagrees with the number of unmet rows (`{gap}`, `{partial}` and `{linux-limit}`). The COUNT is the only fact on that page a machine can own: it counts unmet requirement rows, never distinct protocol obligations or whether their classifications are right |

Un-enrolment exempts only the MISSING-ROW branch of `checkStatusAgreement`. An
un-enrolled RFC with no row makes no public claim to contradict; one that HAS a
row was contradicting its own row in public.

## Who implements the document (owner directive, 2026-09-21)

The ledger answers one question: does ZE's Go code do what the RFC says. A
document Ze does not write Go for leaves the count and names what implements it
instead, so a reader can tell Ze's work from a dependency's.

| Kind | What it says | Counted |
|------|--------------|---------|
| `ze` | Ze implements the document's obligations in Go | yes |
| `mixed` | Ze implements part in Go and another layer performs the rest | Ze's part only |
| `third-party` | a layer under or beside Ze performs it and Ze holds no Go code for it | no |
| `foundation` | the document defines, registers or describes, and obliges no implementer | no |

`third-party` and `mixed` name the implementer: the component and the mechanism
a reader can go and check, such as Linux XFRM for the ESP and AH datapath or the
Linux TCP stack for the transport. "The kernel" alone is refused, because an
unnamed implementer reads on the public page as work nobody owns.

**Leaving the count never means proving nothing.** Ze installs the state the
layer below acts on, so the boundary Ze owns stays testable where the packet
handling is not. Where Ze can observe the behavior it carries a test that
asserts what Ze produced: the selector, the security association, the socket
option, the kernel counter.

This is the DOCUMENT-level parent of the requirement-level annotation below.
The two do not compete. `{lower-layer}` governs one obligation whose role Ze
fills and which a layer under Ze meets on state Ze installs: that stays counted
and met, per the 2026-08-31 ruling. This section governs a document Ze does not
implement in Go at all. A document Ze merely configures is a document Ze
implements part of, which is `mixed`, and its requirements keep the
requirement-level rules.

The kind is the `| Implementation |` row of the summary's `## Meta` table, and
`readImplementation` (`internal/le/rfc/meta.go`) refuses four things: a summary
with no such row, a value outside the four kinds, a kind with no
`| Implementation reason |` beside it, and a `third-party` or `mixed` reason
that names no component a reader can go and check. `Meta.Enrolled` gates a
summary only where `implementationCounts` holds of its kind, so a `third-party`
or `foundation` document leaves the population `./le rfc check` counts, and the compliance page
publishes the split under "Who implements each document". A summary that leaves
the count on this fact carries the kind as its disposition on the declined
index, with the reason that names the implementer.

<!-- source: internal/le/rfc/meta.go -- readImplementation, Meta.Enrolled, implementationCounts -->

## Demonstrated gaps

A `{gap}` row is prose. It says Ze does not meet the requirement, and nothing
runs it, so nothing notices the day the behavior lands. A demonstrated gap is a
Go test that asserts the RFC-correct behavior and is tagged with the word `gap`
in the polarity's place:

```go
// RFC requirement: RFC7606-5.1-1 gap -- the MP_REACH_NLRI attribute is encoded first.
func TestMPReachEncodedFirst(t *testing.T) {
	rfcgap.Demonstrate(t, "RFC7606-5.1-1", func(tb testing.TB) {
		// assertions against tb, written for the correct behavior
	})
}
```

`rfcgap.Demonstrate` (`internal/test/rfcgap`) runs the body and inverts the
result. A body that records an assertion failure passes the test, because the
gap stands. A body that records none fails the test with the id, the summary
file and the two edits owed: remove `{gap}` from the row, and retag the test
`positive` or `negative`. A panic in the body fails the test as a panic, and is
never read as the gap standing. The row keeps its `{gap}` annotation: the
annotation stays the one summary fact, and the tag adds the evidence.
<!-- source: internal/test/rfcgap/rfcgap.go -- Demonstrate -->

`./le rfc check` never runs the test. It checks the tie, and refuses a gap tag
in three cases (`gapTagRefusal`, `internal/le/rfc/gaps.go`):

| Refusal | Why |
|---------|-----|
| The row is not annotated `{gap}` | The gap closed, or the annotation was lost. The message names the row and the retag owed |
| The function around the tag does not call `rfcgap.Demonstrate` with the tag's id as a string literal | A gap tag that nothing runs is a description, not a demonstration. The call is read with `go/ast` inside the unit `UnitAt` answers, so a commented-out call or another id does not count |
| The tag is in a `.ci` or `.et` file | Only a Go test can invert its own result |

<!-- source: internal/le/rfc/gaps.go -- gapTagRefusal, gapDemonstration -->

A `positive` or `negative` tag on a `{gap}` row stays refused as a stale
annotation. A gap tag proves no polarity, so `Collect` keeps it out of the proof
corpus (`splitGapTags`): it is not counted as coverage, no ratchet reads it, and
it owes no discrimination record. Its green half cannot be observed until the
gap closes, and the retag to `positive` or `negative` owes the normal record
then. Its package is type-checked with the tagged packages, because a gap test
is evidence only when it compiles.
<!-- source: internal/le/rfc/gaps.go -- splitGapTags -->
<!-- source: internal/le/rfc/check.go -- check -->

The gate publishes the split. `./le rfc check` prints a `gaps:` line with the
demonstrated and the described counts over every `{gap}` row, and names each
summary that holds a demonstrated one; the JSON report carries
`gaps-demonstrated`, `gaps-described` and `gaps-by-stem`. The per-RFC page names
the demonstrating test's unit on the gap's row in place of "no test", and counts
the demonstrated gaps beside the declared ones.
<!-- source: internal/le/rfc/gaps.go -- gapCounts -->
<!-- source: internal/le/site/rfcevidence.go -- rfcGapRows -->

## The lower-layer annotation

`{lower-layer}` says a layer UNDER Ze performs the behavior, on state Ze
installs into that layer. The owner ruling of 2026-08-31 counts such a
requirement MET and asks for a test at the boundary Ze owns. This kind is for
the requirements where that boundary carries nothing the behavior reads, so no
value of Ze's exists to assert. Sixteen RFC 4302 obligations are the case it was added
for: Linux XFRM builds every AH packet, and no field of the SA Ze installs
decides that the RESERVED field is zero. The owner rule still asks for a test
that checks the OS behaviour on the state Ze installs, and until one is proven
the row is an OS testing gap, never a correctness gap (below, "OS verified").

```
- [ ] [RFC4302-2.3-1] [MUST] The RESERVED field MUST be set to zero by the sender (§2.3) {lower-layer: Linux XFRM AH, linux:CONFIG_INET_AH; internal/plugins/ospf/ipsec_install.go::buildIPsecSA installs the AH SA and the kernel's AH output builds every header, so no value Ze writes decides this field}
```

The reason states three facts, and the gate checks each:

| Fact | Written as | Checked by |
|------|-----------|------------|
| The LAYER that performs the behavior | the head, before the `;` | `parseLowerLayer` refuses an empty head, and a reason with no `;` at all |
| The PRODUCER in Ze that installs into that layer | `<path>.go::<Symbol>` anywhere in the reason | `parseLowerLayer` refuses a reason naming none; `checkLowerLayerProducer` then refuses one the tree cannot show |
| The layer COMPONENT a reader checks | a token inside the head: `linux:CONFIG_<option>` (`linux:CONFIG_XFRM_USER`), `go:<standard library package>` (`go:crypto/x509`) or `vendor/<module path>` | `parseLowerLayer` reads it into `LayerPath`; the exclusion evidence ratchet (see "The ratchets") refuses a row without one; a `vendor/` path that is not exactly a module `vendor/modules.txt` lists as `# <module> <version>` (a package inside a module, the vendor directory or a file in it is refused), whose directory this checkout does not carry, or that no shipped flavor of `ze` links (most of `vendor/` serves the tooling, golangci-lint above all, and performs nothing for Ze); a `linux:` token that is no `CONFIG_` option, names one Ze's runtime kernel config does not build in with `y` (the fragments the builder's own resolution answers, `appliance.RuntimeKernelLayers` over `resolveKernelProfile`: `gokrazy/kernel/kernel.config`, a `# ze-base:` profile when `runtime.config` declares one, `gokrazy/kernel/runtime.config`, then every `# ze-include:` fragment under `tools/kernel-builder/common/`, after all of them; a later `# CONFIG_<name> is not set` disables it; `m` does not count, because the build accepts a required option only at `=y`), or names one no require manifest of those fragments lists (the `.require` beside each), since a fragment line is a request kconfig may answer otherwise and only a listed option is verified against the final `.config`; a `go:` path with a segment the go tool ignores (`testdata`, or one opening with `_` or `.`), that is under `cmd/` or `vendor/`, or that is not a standard library package the shipped `ze` links; and any path with a `.` or `..` segment (`(*evidenceReader).layerPathGap`). What `ze` links is `go list -deps` over `./cmd/ze` once per shipped flavor (the tags each `cmd/ze/build_tag_*_test.go` constraint requires, read by `buildflavor.Shipped` in `internal/test/buildflavor`, the one parser `TestNormalZeLinksNoInternalLe` shares, which refuses a constraint that is not a conjunction its own tags satisfy or a file with none; plus every feature gate, `GOOS=linux`, `CGO_ENABLED=0`), unioned and read at most once per check (`goListLinked`, `internal/le/rfc/check_evidence_linked.go`). A config file, require manifest or module list that cannot be read, or a `go list` that fails, refuses every path of that form |

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
  Ze owns, and the row carries no annotation: Ze's own test proves it. A
  tagged test on a `{lower-layer}` row is a different test, one of the layer
  below acting on the state Ze installs, and the gate accepts it (the
  annotation's `OSMet` trait, `evaluate` in `internal/le/rfc/check_core.go`),
  where a tag on a `{gap}` or a `{not-applicable}` row makes that annotation
  stale.
- **It is not a conformance rollup.** A requirement whose content is "implement
  all of this document" is met by the other rows and not by a layer. That row
  takes `{rollup}` (below), which derives its state from those rows and asserts
  nothing of its own.

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

### OS verified

A gated `{lower-layer}` row is OS VERIFIED when a tagged unit covering it
carries a discrimination record that still verifies and is a proof rather than
the `no-break` escape: the same `DiscriminationVerdict.Proven` predicate that
proves a row of Ze's own (`osVerified`, `internal/le/rfc/profilecoverage.go`).
A tag with no record, a stale record and the escape each leave it an OS
TESTING GAP. The record is made by `./le rfc record` like any other: the break
is to the state Ze installs, and the test that goes red is the one reading
what the layer below did with it.

The record must also break the producer the annotation names: its own
producer, or one of its paired breaks, is the annotation's `path::Symbol`
exactly. A record that broke any other function, in the same file or not,
proves a different fact, and the row stays an OS testing gap (`osProofsOf`,
`TestLowerLayerRecordBreakingAnotherProducerIsOSGap`). A break that removes
the installed state outright reddens any test that reads kernel output, so a
red alone does not show that Ze's state decides the outcome. Where Ze's state
truly decides nothing, the honest record is the `kernel-producer` escape,
which proves nothing, so the row stays an OS gap.
<!-- source: internal/le/rfc/profilecoverage.go -- osVerified, osProofsOf -->

`StemFigures` carries the two counts per RFC, `OSVerified` and `OSGap`, over
the gated rows only. They sum to the `lower-layer` count in
`Mandatory.Excluded`, and they change no other count: an OS-verified row is
still not proven by Ze. One shipped kernel (`internal/appliance/kernel.version`)
is the only OS a record speaks for, so neither count has a per-version split.

## Clause-scoped evidence

A whole sourced sentence can contain both tested behavior and an implementation
gap. Keep its permanent id, level, full quote and section citation, and add
`{partial: tested "<tested span>"; gap "<unmet span>"; <reason>}`. The reason
states what is absent and names the real inspected boundary as
`path/to/producer.go::Symbol`. It must resolve to non-test Go production code.

Selectors use JSON string escaping; semicolons inside a string are content.
Literal braces inside markers are unsupported and refused, including when
whitespace surrounds the `partial` kind. Collection records that parse error;
the shared render-input constructor refuses it rather than publishing a row
without its scope or silently dropping the requirement. Each selector must
be non-empty, match exactly once in the normalized parent quote at word
boundaries, and not overlap the other selector. Tested cannot select the whole
parent. A short selector is a locator, not a new requirement: the parent's
source, section and 24-character minimum remain unchanged. Only one coverage
annotation is allowed; a superseded marker remains independent.

Every positive/negative tag on the row refers only to Tested, and both
polarities are required. The marker declares scope, not proof. All obligations
outside Tested remain unmet or unproven, including any third obligation the
Gap selector does not name. An ordinary `{gap}` still forbids polarity tags.
A partial constituent makes a rollup a gap.

The independent `partial` audit verdict requires this valid annotation, both
polarities, non-empty tests and units maps, the gap-context Producer in its code
map, and a current verified mutant/revert record for every distinct
(requirement, polarity, unit) cover. Missing, stale and no-break records do not
qualify. Its note includes the tested selector, gap selector and producer,
explaining tested and missing behavior. Weak/wrong remain honest findings when
the scoped assertions are inadequate. Enforced on a partial row is refused by
both stamping modes and by the gate, and earns no audited proof on render-only
paths.

`AuditRequirementSHA` binds Text, kind, Tested, Gap, Producer and the complete
Reason as a length-delimited tuple whose fields have every whitespace run
collapsed to one space. Non-partial rows keep exactly `RequirementSHA(Text)`
with its existing normalization; extraction source hashes are unchanged. Adding,
removing or changing scope invalidates the audit as stale-requirement.
Whitespace-only reflow does not. Reseal cannot repair a semantic scope change;
an independent rejudgment must. Existing tag claims and observed-red identities
are not rewritten to adopt the annotation. Changed claim prose or behavior
still requires native re-recording.

Partial is a finding and stays on the audit worklist. Deletion and
partial-to-enforced upgrades use the existing finding ratchets, including
upgrade_reason over unchanged units. Removing a marker alone does not resolve
the finding. No owner-approval escape is added.

Each gated partial row counts once in Gated and Annotated and in the explicit
Partial subset, never in Both, One, Missing or any whole-proven numerator.
The primary partition remains Gated = Both + One + Annotated + Missing.
Ordinary gap and Demonstrate counters keep their meanings; partial rows have
separate counts and lists and are not labelled no-test. Public status must
disclose the remaining gap. A spelled unmet total counts ordinary gaps plus
partial requirement rows once per id, not distinct protocol obligations.
The six-cell shard, status page, CLI pipes, site JSON/HTML/mirror and health
show scope and zero whole credit. Verified record counts measure tag claims,
never whole requirements.
`rfc check` exposes the gated `partial` subset even at zero and a
`partial-scopes` list carrying each full requirement and its typed annotation.
The list includes advisory rows too; those do not enlarge the gated subset.
JSON, YAML and table pipes retain the same payload and exit status.

<!-- source: internal/le/rfc/summary.go -- parsePartial, partialScopeRefusal -->
<!-- source: internal/le/rfc/freshness.go -- AuditRequirementSHA -->
<!-- source: internal/le/rfc/check_audit.go -- verdictClaims, checkPartialProofs -->

## The row quote

A requirement row states the RFC's own sentence, copied verbatim. The row's
quote is its text before the trailing section parenthetical, after the trailing
`{...}` markers are peeled (`Requirement.Quote`, `internal/le/rfc/summary.go`).
Only the LAST parenthetical is cut, and only when it cites a section, so a quote
that carries `(in octets)` or a brace keeps it.

`checkRowQuotes` (`internal/le/rfc/check_quote.go`) refuses a row when one of
these is true:

| Refusal | Condition |
|---------|-----------|
| No source | the RFC's text is not in `rfc/full/` or `rfc/drafts/` |
| Too short | the quote is under 24 characters, which names no single sentence |
| Unresolved anchor | the RFC has headings, and the cited section is not one of them, and no heading ancestor of it is. A row that cites no section is refused the same way. An RFC with no heading is never refused this way (below) |
| Wrong section | the quote is verbatim in the RFC, but not in the cited section or its subsections. The refusal names the section that holds it |
| Not verbatim | the quote is in no section of the RFC |
| Erratum not stored | the row cites an erratum and `rfc/errata/<stem>/<number>.txt` is absent, not verified, or not that erratum. The refusal names the file |
| Replaced by an erratum | the quote is verbatim in the cited section as published, and an erratum the row cites replaced it |

The cited section resolves to its nearest heading ancestor: `3.b` resolves to
`3`, and `2.1.4` resolves to `2.1` when `2.1` is the deepest heading. The match
is then made in that section and in every subsection, one section body at a
time, so a span that joins two sections never matches. The lookup never falls
back to the whole document.

Headings are read one of two ways, decided once for each text, and the site
inventory and this check share the reading (`sectionBodies`). A text with a
heading at column 0 is read at column 0 only, and every heading-shaped line
indented below it is body text. A text with no column-0 heading is read at its
body margin, the indentation most of its lines share (owner decision D-5,
2026-09-26). There a heading is a clause number with no trailing dot, two
blanks and a title (`6.17  Checksum`), a dotted number alone on its line
(`1.3`), an annex (`ANNEX B - CHECKSUM ALGORITHMS`) or an annex clause
(`B.1  SYMBOLS`), and it must begin a paragraph that holds no contents line, a
line ending in dot leaders and a page number. The narrow shape is what keeps a
note numbered `1.`, a justified line that starts with a number, and the table
of contents out of the section list. In the corpus of 2026-09-26 only RFC 905 is
read at its margin and finds headings there: 329 sections, in the order of its
table of contents. Every other text derives the sections and site ids it did
before.

At column 0 a heading is a number (`3.1.  Title`, `4  Title`), a letter with a
dot (`A.  Title`), a letter and a number with no dot between (`A2.  Title`, RFC
4302), a letter and a dotted number with no trailing dot (`B.1 Level 1 Complete
Sequence Numbers PDU`, the annex subsections of RFC 1195), or `Appendix` or
`APPENDIX` before a letter that ends in a dot or a colon
(`Appendix D:  Configuration Parameters`, RFC 3101). The colon is read only
under the word, because a bare `S: 250 OK` is a line of a transcript. One line
of that shape is still not a heading: a number written with no dot at all that
is not the next top-level section, meaning one more than the highest top-level
number opened so far. Zero is never one. That is how the attribute tables of
RFC 2865, RFC 2869 and RFC 3579 open their rows (`0        0       0-1     0-1
101   Error-Cause`, `1        1       1       1           80
Message-Authenticator`), and how the RFC 2759 hash example opens its byte dumps
(`55 73 65 72`) and one label (`24 octet NT-Response:`). Read as headings, they
filed a table's notes under a section `0`, or back under section `1`, so a
verbatim quote of Note 1 was refused as outside the section it is in. A dotted
number keeps the plain reading, because a dotted heading can skip a number (`10.
Full Copyright Statement` after section 7, RFC 2548). On 2026-09-27 this moved
the sections or sites of RFC 1035, 1195, 1812, 2205, 2548, 2661, 2759, 2865,
2869, 3101, 3579, 3602, 4301, 4302, 4303, 4456, 4861, 5072, 6482, 791 and sFlow
v5, and every extraction artifact among them was re-walked with its decisions
carried forward by quote. Reading the undotted annex subsection, later the same
day, moved the sections of RFC 1195, 1812, 2328, 2473 and 3101 the same way; the
rows of RFC 1195 that had cited `§8` for annex text now cite the annex
subsection.
<!-- source: internal/le/rfc/inventory.go -- columnZeroHeadings -->

An RFC with no heading under either reading is the one exception (owner
decision D-1, 2026-09-26). Its whole text is one citable section, and every
citation resolves to it, whether the row cites an unnumbered title such as
`(§Echo)` or no section at all. There is no section for the citation to be wrong
about, so the check judges only whether the quote is verbatim, and a refusal
names that section `front`. This is not a fallback: an RFC with one heading or
more resolves exactly as above, and a citation of its front matter, the text
before the first heading, is refused as an unresolved anchor. In the corpus of
2026-09-26 the texts without a heading are RFC 792, 1997, 2347, 2348, 2349 and
2782.
<!-- source: internal/le/rfc/inventory.go -- quoteSource.wholeText, quoteSource.resolve -->
<!-- source: internal/le/rfc/inventory.go -- sectionHeadingRE -->
<!-- source: internal/le/rfc/inventory.go -- indentedHeadings -->
<!-- source: internal/le/rfc/inventory.go -- sectionBodies -->


One haystack builder serves every quote path: `quoteHaystack`
(`internal/le/rfc/inventory.go`) strips the page furniture (the footer, the
form feed, and the running header up to its first blank line, which RFC 792
writes on two lines), collapses the
whitespace and joins a word the RFC hyphenated across a line break. The row
check, `checkFeatureDeclined` and the level correction read by
`checkLevelRatchet` all match against it, so a sentence one of them finds, the
others find too. The strip is necessary because 163 of 6243 keyword sentences in
the corpus cross a page break. The join is there because the collapse reads
`close-` at the end of one line and `notify` on the next as `close- notify`:
the haystack and every quote drop the blank after a hyphen that ends a word, so
a row may write `close-notify` or `close- notify` and both match. A hyphen with
a blank on both sides is a dash and stays. The match is case-sensitive.
<!-- source: internal/le/rfc/inventory.go -- joinWrappedHyphens, quoteNeedle -->

### A row that cites an erratum

A row may state an obligation an erratum corrected or added. It then cites the
erratum in its section parenthetical, `(§7.1, erratum 8301)`, and its quote is
judged against the RFC as that erratum corrects it (owner decision D-11,
2026-09-27). `errata 543` and `Errata ID 7840` are read the same way. A number
written anywhere else in the row is not a citation.

The erratum's text lives in `rfc/errata/<stem>/<number>.txt`: a header of
`Key: value` lines, then an `Original Text:` line and the text the erratum
says the RFC holds, then a `Corrected Text:` line and the text it should hold.
Both blocks are copied from `https://www.rfc-editor.org/errata/eid<number>`
("says" and "It should say"). The check takes the file only when its `Errata
ID` is the number in its name, its `RFC` is the stem's, its `Status` is
`Verified`, and both blocks are present: a reported erratum is its reporter's
claim, and nobody verified it. `Location: Section X` names where the original
text is.

A quote that cites errata passes when it is one of these:

| Passes as | Condition |
|-----------|-----------|
| The corrected text | the quote is a verbatim span of the corrected text of one cited erratum |
| The corrected section | the quote is a verbatim span of the cited section once every cited erratum is applied: its original text replaced by its corrected text, in the section its `Location` names and that section's subsections |

The quote never passes as a span joining an erratum's text to the section around
it. Once the erratum is applied that text is in the section, so the second row
of the table covers it. A sentence the erratum left alone passes as before, so a
row citing an erratum only as context, such as `RFC9568-5.2.5-1`, keeps its
published sentence.

An erratum whose original text is not verbatim in the section it names cannot
be applied. That happens when one erratum corrects two sections at once, as
erratum 8301 corrects RFC 9568 sections 6.1 and 7.1, or when its `Location` names
no section. The check then cannot tell which published sentence the erratum
replaced, so it refuses every published sentence of the row and accepts only
the erratum's corrected text. Accepting the published text would pass a
sentence the erratum may have withdrawn.
<!-- source: internal/le/rfc/errata.go -- erratumRowRefusal, parseErratum, citedErrata -->

The rule judges every row in the corpus, against the summaries, the RFC texts
and the stored errata of the tree under check. A row the commit under test did
not touch is refused like a row it added, so a row that stops being verbatim
because its RFC text or a cited erratum's file changed is refused too, and the
refusal names that row. A stem with rows and no RFC text is refused row by row
(the "No source" refusal), never skipped. `checkRowQuotes`
(`internal/le/rfc/check_quote.go`) reads each stem's RFC text once.

Until 2026-09-27 the rule judged only the rows a commit added or edited, and a
ratchet refused a stem whose count of unquoted rows rose over `HEAD^`, while
`./le rfc check` printed the backlog as a measurement. The hand backfill
(`spec-rfc-requirement-quote-hand-backfill`) quoted every row the mechanical
backfill left, taking the backlog to zero, and the change scope, the ratchet and the backlog figure were
then deleted: with every row judged, an unquoted row is a violation, and a
count of violations beside the violations would state the same fact twice.
<!-- source: internal/le/rfc/check_quote.go -- checkRowQuotes, rowQuoteRefusal -->

`./le rfc quote-backfill stem <stem>` rewrites the rows whose quote can be taken
mechanically from the extraction walk. Without `apply` it writes nothing: it
prints the rows it would quote, the review list and the human list. With `apply`
it writes the summary. A row that already passes the row check is counted as
verbatim and left alone.
<!-- source: internal/le/rfc/quote_backfill.go -- quoteBackfill -->

The rewrite replaces only the text before the trailing section parenthetical with
the site sentence. The id, the level, the parenthetical and every trailing `{...}`
marker stay byte for byte. The tool parses the new line back and does not write
it unless the id, the level and the section are unchanged and the row check
accepts the quote.

A row is rewritten only when exactly one site maps it and every test below
passes. A failed test puts the row on the review list, with its kind:

| Kind | The row goes to review when |
|------|-----------------------------|
| `several-sites` | more than one site maps it, so a human chooses the sentence |
| `lead-in` | the site sentence ends in `:`, and the obligation is in the list after it |
| `short-sentence` | the sentence is shorter than the row check's minimum |
| `qualified` | the row quotes an RFC sentence and adds its own words, such as "the A half" |
| `unresolved-anchor` | the cited section names no heading of the RFC. An RFC with no heading never gives this kind, because its whole text is the cited section |
| `outside-section` | the sentence is not in the cited section or its subsections. The citation is never rewritten |
| `level-differs` | the sentence does not state the row's level. SHALL and REQUIRED count as MUST, and MUST NOT is its own level |
| `partial` | the sentence states more RFC 2119 keywords than the row, so the whole sentence widens what the row claims |
| `number-absent` | a number in the row is not in the sentence |
| `polarity-differs` | one of the row and the sentence says "not" or "never" and the other does not |
| `low-overlap` | the sentence carries less than half of the row's content words |
| `rewrite-refused` | the new line fails its parse-back or the row check |

The human list holds the rows that no site maps: `unsourced` (the extraction
declares them), `unmapped`, `no-extraction` (the stem has no artifact) and
`no-rfc-text`.

The review list catches a wrong MAPPING, not every wrong PARAPHRASE. A row whose
words match the sentence and whose meaning does not is still rewritten, and the
quote then states the RFC's obligation in place of the row's. That is the
purpose of the rewrite, but the tests tagged to the row still prove the old
claim. So read each rewritten row's tagged tests against its new text.

## The feature-declined annotation

`{feature-declined}` says the obligation is CONDITIONAL on a feature the RFC
makes optional, or conditional on the feature's use in the row's own sentence,
and Ze declined that feature, so the condition is false and nothing is owed. The owner approved it on 2026-09-03 for `RFC4302-2.5.1-1`: RFC
4302 §2.5.1 says an Extended Sequence Number "MUST be negotiated by an SA
management protocol", Ze negotiates no AH SA and uses no ESN, so `{gap}` would
accuse Ze of owing behavior it does not owe, and `{lower-layer}` would claim a
negotiation no layer performs.

```
- [ ] [RFC4302-2.5.1-1] [MUST] Use of an Extended Sequence Number MUST be negotiated by an SA management protocol (§2.5.1) {feature-declined: "To support high-speed IPsec implementations, a new option for sequence numbers SHOULD be offered, as an extension to the current, 32-bit sequence number field."; ze offers no ESN, so nothing ever uses one. internal/plugins/ospf/ipsec_install.go::buildIPsecSA is the only code that creates an AH SA, and it builds one manually keyed transport-mode state from static interface configuration with no ESN in it}
```

The reason states two facts, and the gate checks both:

| Fact | Written as | Checked by |
|------|-----------|------------|
| The RFC's own sentence making the feature OPTIONAL, or the row's own sentence making the obligation conditional on its use | a double-quoted span, first, before the `;`, optionally preceded by `§<section>` and a blank naming the section it comes from (see "A quote from another section" below) | `parseFeatureDeclined` refuses a body that opens with anything else and a quote under 24 characters; `checkFeatureDeclined` then refuses one that is in no section of `rfc/full/<stem>.txt`, matching against the same page-stripped section bodies as a row quote (see "The row quote"), so neither the RFC's line wrapping nor a page break matters. The exclusion evidence ratchet (see "The ratchets", `featureQuoteGap`) holds the quote to one section: the section the row cites, or the section `§` names. It must sit in that section or one of its subsections (`placeQuote`), and one the RFC carries in another section is refused, naming the section it is in and the section it is held to. It must be a whole sentence of that section (`(*quoteSource).wholeSentence`) and take the optionality shape or the conditional shape (`featureShapeGap`, below) |
| The PRODUCER in Ze that does the narrower thing | `<path>.go::<Symbol>` anywhere in the reason | `parseFeatureDeclined` refuses a reason naming none, and one naming a `_test.go` file; `checkFeatureDeclined` then refuses one the tree cannot show |

**A quote from another section.** An RFC often states that a feature is optional in one section and puts the
obligations conditional on it in another. RFC 9728 §2.2 says "Consumers of the
metadata MAY ignore the signed metadata if they do not support this feature.",
and the MUST it makes optional is in §3.3. A quote held to the row's own section
could never carry that evidence, so the payload names the section its quote
comes from:

```
- [ ] [RFC9728-3.3-4] [MUST] The recipient MUST validate that any signed metadata was signed by a key belonging to the issuer and that the signature is valid (§3.3) {feature-declined: §2.2 "Consumers of the metadata MAY ignore the signed metadata if they do not support this feature."; internal/component/mcp/oauth.go::writeResourceMetadata emits only plain JSON, and ze consumes no signed metadata}
```

`parseFeatureDeclined` reads the `§<section>` and the blank after it into
`Annotation.QuoteSection`; a `§` with no section, or a section with no blank
before the quote, is refused with the format. The section must have the shape
of a heading's number (`featureSectionIDRE`): digits joined by dots (`2`,
`2.2`, `4.1.3`), or an appendix letter and its numbers (`A`, `A.1`). A token of
any other shape, such as `§2.2;`, `§2.foo` or `§2.`, is refused at parse time,
naming the accepted shape. The exclusion evidence ratchet (`featureQuoteGap`,
in `(*evidenceReader).gap`) then holds the quote to the named section: the
section must be an exact heading of the RFC (`(*quoteSource).has`), with no
fallback to an ancestor, so `§2.99` in an RFC with no §2.99 is refused rather
than read as §2. The quote must be verbatim in that section or one of its
subsections (`placeQuote`). It refuses, naming the section and the quote, a
quote the named section does not carry (and the section that does, when one
does), a quote no section carries, and a named section the RFC has no heading
for. A named section is refused as redundant whenever the quote lies in the
row's own section or its subsections, whichever section it names (an ancestor
of the row's section included), so each case has one spelling. The form
without a section means the row's own section or its subsections, resolved to
its heading as a row's citation is.

**What the quote must say.** With or without a section named,
`featureShapeGap` judges the quote against the section it is held to. A
declined feature leaves the count only on the RFC's own words: a sentence that
makes the feature optional, or a sentence that makes the obligation conditional
on using the feature, with evidence that Ze never meets the condition
(`ai/META-DECISIONS.md`). So the quote takes one of three shapes, and a quote
that fits none is refused naming all three (`featureDeclinedShapes`):

| Shape | The quote | What else it owes |
|-------|-----------|-------------------|
| Optionality | A sentence holding an advisory level (`MAY`, `OPTIONAL`, `SHOULD`, `SHOULD NOT`, `RECOMMENDED`, `NOT RECOMMENDED`, read from `advisoryLevels`) in upper case, or no mandatory level and one of `optionalityWords` as whole words in any case: `optional`, `optionally`, or `if` followed by `support` or `supports` | It must not hold the row's own sentence (`Requirement.Quote`, compared as a row quote is): the obligation the row declines is no evidence that it is optional |
| A named section | Either other shape, from the section `§` names (above) | What that shape owes |
| Conditional | A sentence holding a mandatory level (`MUST`, `MUST NOT`, `SHALL`, `SHALL NOT`, `REQUIRED`, read from `gatedLevels`) whose obligation a clause on the feature's use guards (`conditionalGuard`). Before the first mandatory keyword, at the sentence's start or after a comma, a clause opens with one of `conditionalLeads` (`if`, `when`, `where`, `once`, `unless`), or is a subject held by a restrictive relative clause (`relativeSubjectRE`): an article and up to four words before `that` (`a client that`, `the client that holds a lease`), or, with no article, up to four words whose last is plural (`receivers that`). A word before `that` in `contentClauseHeads` (a copula such as `is` or `are`, or a verb such as `means`, `defines`, `requires` or `states`) opens a content clause and holds no subject; a preposition in `subjectPrepositions` (`of`, `to`, `for`, `in`, `on`, `from`, `with`) before `that` ties the clause to the preposition's object (`a receiver of the frame that arrived`); and a `that` directly before the keyword holds no condition (`A 1 octet field that MUST be set to 0`). The guard clause is the text after the lead word or after `that`, up to the keyword. The quote must be the row's own sentence, in the section the row cites, because the condition must guard the obligation the row declines. Another sentence, or one from a `§`-named section, is judged as the optionality shape only | Its reason, after the quote, carries a never clause, `ze never <condition>: <path>.go::<Symbol> <how>`, opening the reason or after a `;` (`neverClauseRE`). The condition shares at least one content word with the guard clause (`sharedContentWord`: compared singular, and not a word of `guardStopwords`, `roleDeterminers` or `genericRoleWords`). The producer it names must be a function this tree declares outside a `_test.go` file, resolved as every producer check resolves one (`resolveProducer`, in `neverClauseGap`). An optionality quote whose mandatory keyword a conditional clause guards owes the same never clause, so an advisory keyword beside a conditional `MUST` does not skip it |

RFC 9915 §6.3 is the conditional the owner's ruling names. The row is the
sentence itself, and the never clause carries the evidence:

```
- [ ] [RFC9915-6.3-2] [MUST] If the client uses a delegated prefix to configure addresses on interfaces on itself or other nodes behind it, the preferred and valid lifetimes of those addresses MUST be no longer than the remaining preferred and valid lifetimes, respectively, for the delegated prefix at any time. (§6.3) {feature-declined: "If the client uses a delegated prefix to configure addresses on interfaces on itself or other nodes behind it, the preferred and valid lifetimes of those addresses MUST be no longer than the remaining preferred and valid lifetimes, respectively, for the delegated prefix at any time."; ze never configures an address from a delegated prefix: <path>.go::<Symbol> <how the producer shows it>}
```

RFC 9915 §18.2.10.5's `"As a counterexample, a client that previously received
an NTP server address from the DHCP server and does not receive it anymore MUST
stop using the configured NTP server address."` is the relative-clause form:
the subject `a client that previously received ...` follows a comma.

| Refused | Why |
|---------|-----|
| A quote that is not a whole sentence of that section (`(*quoteSource).wholeSentence`, the same cut as a `{not-applicable}` quote) | A fragment can cut the optionality, or the condition, away from the words that scope it |
| A quote whose only BCP 14 keywords are mandatory levels and that no conditional clause guards (`"A receiver MUST NOT reorder ..."`) | It states an obligation, so it makes nothing optional or conditional |
| A quote with no optionality marker and no conditional obligation | The kind rests on the RFC saying the feature is optional, or the obligation conditional |
| An optionality quote holding the row's own sentence | The obligation the row declines is no evidence that it is optional |
| A conditional that is not the row's own sentence, or comes from a `§`-named section | Another obligation's condition says nothing of the one the row declines |
| A conditional whose reason carries no never clause | Nothing shows that Ze never meets the condition |
| A never clause whose condition shares no content word with the guard clause | It names another condition than the one the obligation rests on, and the refusal names the guard clause |
| A never clause naming a `_test.go` file, a file the checkout does not carry, or a function its file does not declare | The claim rests on code a reader can open, and a rename under it turns the gate red |

<!-- source: internal/le/rfc/check_evidence.go -- featureShapeGap -->
<!-- source: internal/le/rfc/check_conditional.go -- conditionalGuard, conditionalClause, neverClauseGap, sharedContentWord -->

The quote is the stronger of the two, and it is what a free-text reason cannot
be: a claim about the DOCUMENT, checkable against the document. A reason saying
"Ze does not do that" is a judgement, which is how `{not-applicable}` grew to
915 sites the owner ruling presumes are mostly wrong. Both refusals are the same
shape `checkLowerLayerProducer` already has, so a rename under the annotation
turns the gate red for either kind.

An RFC whose text this repository does not hold is REFUSED rather than skipped.
A quote nobody can check is exactly the assertion this kind exists not to be,
and enrolment already requires the text.

Four rules decide whether it is the right kind:

- **The obligation is CONDITIONAL.** The MUST is written as "if you do X, do it
  this way", and X is the feature. An unconditional MUST Ze does not meet is a
  `{gap}`, whatever the reason.
- **The RFC makes X optional, in words you can quote.** A feature the RFC
  requires is not declinable, so no quote exists and the check refuses the line.
- **Ze declined X, and code shows it.** The producer is the function that does
  the narrower thing: it is what a reader opens, and what a later change that
  ADDS the feature has to walk past.
- **The absent FEATURE is still disclosed.** It goes on the summary's own
  `Support status` and `Support remaining` rows, as an implementation gap a
  later scope decision can revisit, and never as a conformance gap
  (`ai/rules/rfc-compliance.md`, owner directive 2026-08-31). This kind is the
  coverage register's word for the decision the extraction register records as
  `feature-out-of-scope`. The two are spelled apart because the site's
  `out-of-scope` counter already means `{not-applicable}` there.

It stays INSIDE the gated denominator and OUT of the proven numerator, exactly
as `{lower-layer}` does, and
`TestNoAnnotationExceptSinglePolarityMovesThePublishedShare` holds that property
over this checkout's own corpus for every kind but `{single-polarity}`. On the
site it is its own bucket, `feature_declined`, labeled `Optional feature
declined`, and it is counted with `{not-applicable}` in the `Out of scope` card:
neither obligation binds Ze, and the card's own note names both kinds and what
each one says.

An audit verdict over a `{feature-declined}` line is `not-applicable`, with a
`no_code_path` reason saying why no Ze path reaches the declined feature.
`verdictClaims` accepts that verdict over `{not-applicable}` and
`{feature-declined}` alike, because both say nothing is owed, and
`./le rfc audit-stamp` applies the same check. Audit coverage is monotonic, so
a row judged before it was re-annotated keeps a verdict, and this is the one it
can hold. `unimplemented` stays refused there: it says Ze owes behavior it does
not deliver, which is the claim the annotation denies.
<!-- source: internal/le/rfc/check_audit.go -- verdictClaims -->

It cannot take a `{gap}`'s slot, for the reason recorded about `SupersededKind`:
one checklist line carries ONE disposition, so a line carrying both is refused
rather than silently relabeled.

### The heading form

A section heading that names the declined feature scopes every obligation under
it the way a conditional does. RFC 8907's `5.4.2.3.  CHAP Login` scopes its
START packet to CHAP, and RFC 9915's `11.2.  DUID Based on Link-Layer Address
Plus Time (DUID-LLT)` scopes its hardware type to a DUID-LLT, though neither
sentence says so. The heading form names the feature words in place of a quote,
with the keyword `heading` where the sectioned form writes `§<section>`, and
the summary's `## Meta` table names the owner decision that declined the
feature in a `Declined features` row, comma-separated entries of
`<feature words> (<owner-decision source>)`:

```
| Declined features | CHAP (owner 2026-09-21 tacacs_methods) |
- [ ] [RFC8907-5.4.2.3-1] [MUST] The START packet MUST contain the username in the user field, and the data field is a concatenation of the PPP id, the challenge, and the response. (§5.4.2.3) {feature-declined: heading "CHAP"; ze never uses CHAP: <path>.go::<Symbol> <how the producer shows it>}
```

| Fact | Checked by |
|------|------------|
| The feature words hold a word that is not a stopword, a determiner, a role word (`contentWords`) or a word nearly every RFC titles a section with (`genericHeadingWords`: introduction, overview, procedure, message, format, processing, security, consideration, session, timer, capability, notification and the rest) | `parseFeatureHeading`, at parse time, with the producer outside a `_test.go` file every `{feature-declined}` reason owes |
| The summary has a `Declined features` row, and the feature words equal, or hold as one run of whole words, the words of one of its entries, so `CHAP Login` is held by the entry `CHAP` and `CHAP` is not by `MS-CHAP`. A row is refused for an empty entry, an entry with no words, unbalanced parentheses, or a source that is missing, empty or a placeholder (none, n/a, tbd and the rest). A summary with a heading form and no row, or no matching entry, is refused naming the stem, the row and the exact Meta row to add | `checkDeclinedFeatureHeadings`, in `ParseSummaryText`; `declinedFeaturesFrom`, in `ParseMeta` |
| A feature word holds at least 3 characters (`minHeadingWords`), or is a two-letter word that is the whole of a matched entry's words, so the owner may name `PE` for RFC 4577's `PE Routers` and `ID` names nothing by accident | `checkDeclinedFeatureHeadings` |
| Each entry's words are in the RFC's text as one run of whole words, and an RFC whose text is absent is refused | `checkDeclinedFeaturesInText` |
| The words are one run of whole words, compared lower case and singular as a role is, in the title of the row's own section or of a section above it (`(*quoteSource).headingTitles`), so `CHAP` is in `CHAP Login` and not in `MS-CHAP v1 Login` | `featureHeadingGap`, through the exclusion evidence ratchet |
| The words are not in the RFC's own title, compared with hyphens read as blanks so `BGP` is in `(BGP-4)`, even where a heading repeats them, because they then name the whole document and the whole document is not one feature; and they hold a word that is neither one of the title's words nor generic, so `TACACS+ Packet` in RFC 8907 is refused. An RFC whose title cannot be read is refused | `featureHeadingGap` |
| The reason carries the never clause, `ze never <uses the feature>: <path>.go::<Symbol> <how>`, whose condition shares a content word with the feature words, naming a producer the tree declares outside a `_test.go` file | `neverClauseGap`, as for a conditional |

The RFC's own title (`documentTitle`, `documentTitleOf`) is the last front
matter paragraph before the first line reading `Abstract`, `Status of This
Memo` or, in the 1981 RFCs 791 and 792, `DARPA INTERNET PROGRAM`, compared
without case or a trailing colon (RFC 905 writes `Status of this Memo:`). A
front matter with none of them ends at the first heading, as RFC 1034 and
1035 number theirs `1. STATUS OF THIS MEMO`, and its last paragraph is the
title.

A heading wrapped over two lines is read from its first line only. The RFC's
text is owed as for a quote: `checkFeatureDeclined` refuses a heading form
whose RFC has no text here.

<!-- source: internal/le/rfc/check_heading.go -- featureHeadingGap, headingWordsGeneric, genericHeadingWords, phraseIn, checkDeclinedFeatureHeadings, checkDeclinedFeaturesInText -->
<!-- source: internal/le/rfc/meta.go -- declinedFeaturesFrom, metaDeclinedFeatures -->
<!-- source: internal/le/rfc/summary.go -- parseFeatureHeading, minHeadingWords -->
<!-- source: internal/le/rfc/inventory.go -- documentTitleOf, headingTitles -->

## The linux-limit annotation

`{linux-limit: LL-<ID>; <why no Ze code substitutes>}` says the obligation
binds, but Linux cannot provide the primitive it needs. The id names the entry
in `docs/architecture/linux-limitations.md`; the row does not copy its evidence.
A missing id or reason is refused, and a line cannot carry another disposition
beside this one.

`checkLinuxLimit`, in the annotations check group, requires all of these:

- The register parses without diagnostics and holds the named entry.
- Its `Kind` is `kernel`, `Effect` is `blocks`, and `Status` is `verified`.
  A library limit is fixed by a carried diff; `constrains design` means Ze can
  implement the behavior in another shape. Both remain `{gap}`. An unverified
  entry's `Re-check` names what must be settled first.
- `Blocks` names the exact requirement id, not a longer id sharing its prefix.
  `Evidence` is nonempty, and `Checked` names the kernel version in
  `internal/appliance/kernel.version`.

Every refusal identifies the summary row and the register source. Missing or
malformed inputs fail closed for annotations, even on unenrolled summaries.
A corpus without this annotation acquires no required register dependency.

The obligation stays counted and unproven. It is unmet in the public status
and Remaining count, and contributes a gap to a rollup. A tagged test makes
the annotation stale. The audit verdict `unimplemented` is legal (with its
usual code evidence); `not-applicable` is not, because the obligation binds.
The per-kind trait declaration supplies these decisions and the ceiling exclusion.
A summary carrying this annotation must never claim `Supported`, including
scoped `Supported ...` and legacy `Yes` claims, even when Remaining correctly
says `One MUST unmet`. Set `Support status` to `Partial` and retain the unmet
disclosure. This prohibition is declared only for `{linux-limit}`; ordinary
gap and partial annotations retain their existing disclosure rules.

Each added or reworded instance owes `Linux-limit-owed: <RID>: <LL-id>` in
the commit. It may land, but publication waits for the owner's
`Owner-attested:` answer through the existing commit approval machinery.
Design approval does not approve an instance.

`CheckReport.linux-limit-candidates` and the prose report list existing rows
named by eligible, current entries but not annotated with this kind. These
are evidence for an owner decision, not violations or approval: the report
never edits a summary, and an ineligible entry proposes nothing.
<!-- source: internal/le/rfc/linux_limit.go -- parseLinuxLimit, checkLinuxLimit, linuxLimitEntryRefusal -->
<!-- source: internal/le/rfc/check.go -- CheckReport, check -->

## The rollup annotation

`{rollup}` says the row asserts nothing of its own. The row is true exactly
when every row it names is true. RFC 4302 §5 is the case it was added for:
"Implementations that claim conformance or compliance with this specification
MUST fully implement the AH syntax and processing described here for unicast
traffic, and MUST comply with all requirements of the Security Architecture
document". No test can prove that sentence on its own row. A tagged test proves
one requirement, and a test on this row would claim more than its body checks.
Every other kind asserts something about the row that carries it, and none of
those assertions is true here. `{gap}` accuses Ze of owing behavior its
constituents already meet. `{lower-layer}` names no layer. `{not-applicable}`
denies a role Ze fills. `{feature-declined}` needs an optional feature.

```
- [ ] [RFC4302-5-2] [MUST] An implementation claiming to support multicast traffic MUST comply with the additional requirements specified for such traffic (§5) {rollup: RFC4302-2.4-2, RFC4302-2.4-3, RFC4302-2.4-4; Section 5 binds a multicast implementation to "the additional requirements specified for support of such traffic", which are the three multicast rows of Section 2.4}
```

The body is a target list, then `;`, then why those rows are the ones the
sentence binds. A target is a requirement id (`RFC4302-2.4-2`) or a summary
stem (`rfc4301`). A stem means every gated row of that enrolled summary except
its own rollups. `RFC4302-5-1` names the stem, because it binds all of RFC 4301.
A list of that summary's ids would go stale on the first row it gains.

The gate DERIVES the row's state from the rows it names, and never reads a
state the author wrote:

| Derived state | When | What the row publishes |
|---------------|------|------------------------|
| `met` | every target is proven both ways, or `{single-polarity}` proven its one way, or excused by `{lower-layer}`, `{feature-declined}` or `{not-applicable}`, or a met rollup | `derived: met` |
| `gap` | any target is `{gap}` or a gap rollup | `derived: gap: <target> is annotated {gap}`, naming the first target that decided it |
| `unproven` | any other target: no test, one polarity, a row with no state | `derived: unproven: <target> is not proven`, naming the first target that decided it |

The gate raises NO finding for a rollup, in any state (owner decision,
2026-09-15). A `{gap}` target is a declared state, disclosed under its own id
by the Remaining cell. An unproven target is already reported under its own id.
A finding on the rollup would report one fact twice, and the cause on the row
already names the constituent where the work is owed.

The rollup itself owes nothing. It changes state the day its constituents do,
with nobody editing it. A `{not-applicable}` target counts as met because the
exclusion is that row's own claim, presumed wrong and reviewed there. A stem
target with no gated row is unproven, because a rollup over nothing proves
nothing.

Five refusals hold the kind to the corpus. Each ends in the format sentence
`rollupFormat` (`internal/le/rfc/summary.go`):

| Refused | Where |
|---------|-------|
| No target, no `;`, or an empty reason | `parseRollup` |
| A target that is neither a requirement id nor a summary stem, or one named twice | `parseRollup` |
| The row's own id, an id no enrolled summary holds, or a stem that is not enrolled | `checkRollupTargets` (`internal/le/rfc/check_core.go`) |
| A rollup whose targets lead back to the row, at any depth | `checkRollupTargets`, a walk bounded by the number of rollups in the corpus |
| A tagged test on the row | `evaluate`, as a stale annotation, exactly as beside `{gap}` or `{not-applicable}` |

A target the corpus cannot show is refused rather than accepted as prose, for
the reason `{lower-layer}` refuses a producer the tree cannot show. The kind's
value is that a reader can check it, and an unchecked target is a judgement.
`RFC9190-2.4-1` and `RFC9190-5.6-4` point at RFC 8446 and RFC 7542, which have
no summary. They stay unannotated until those RFCs enroll.

It sits OUTSIDE the gated denominator and OUTSIDE the proven numerator, which is
the opposite of the two kinds above. A rollup carries no obligation of its own.
Every obligation it names is already counted once under that row's id, so
counting the rollup would bill a document twice. It would also hold a fully
conformant RFC's share one row short of 100%. `CoverageRows`
(`internal/le/rfc/coverage.go`) leaves it out of every count, and
`TestRollupMovesNeitherShareNorGatedCount` holds that over this checkout's
corpus in both directions.

The public ledger prints the derived state after the
reason in the Proof column (`{rollup} <targets>; <why>, derived: met`, or
`derived: gap: RFC4302-2.5-5 is annotated {gap}`). On the
site it is its own bucket, `rollup`, labeled `Derived from other rows`, in no
ratio card and no bucket table. The stem page lists it apart from the parts,
with the derived state as a mark on the row beside the target list.

It cannot take a `{gap}`'s slot, for the reason the two kinds above cannot: one
checklist line carries ONE disposition.

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

## The updated-by marker

`{superseded}` is for a document another RFC obsoletes. A document another RFC
only UPDATES keeps its obligations, and some of its rows are rewritten: RFC 7606
Section 3.d replaces RFC 4271's session reset for a missing well-known attribute
with treat-as-withdraw. Such a row carries one marker naming the updater's
requirement that rewrites it, with the reason:

```
- [ ] [RFC4271-6.3-7] [MUST] <text> (§6.3) {updated-by: RFC7606-3.d-1; RFC 7606 Section 3.d replaces the reset for a missing well-known mandatory attribute with treat-as-withdraw}
```

`parseUpdatedBy` (`internal/le/rfc/summary.go`) refuses a marker with an empty
body, no reason, or anything but exactly one requirement id. Where several rows
of the updater rewrite one row, the marker names the governing one, and the
others stay reachable through the updater's own summary. A row carries at most
one `{updated-by}`, and it composes with a coverage annotation and a
`{superseded}` marker on the same line, one of each kind (`stripMarkers`).

`checkUpdatedBy` (`internal/le/rfc/check_core.go`) holds the pointer against
the updater, and refuses, naming the fix, when:

| Refusal | Fix |
|---------|-----|
| The updater's summary is not in `rfc/short/` | Summarise the updater first, or remove the marker |
| The updater has no text under `rfc/full/` or `rfc/drafts/` | Fetch the text; the refusal prints the `curl` line |
| The updater's RFC header has no `Updates:` line naming this RFC | Remove the marker, or name the RFC whose header updates this one: only an updating RFC rewrites a row |
| The updater's summary does not declare the id | Name the row that rewrites this one, or extract it first |

The update edge is the updater's own RFC header, read from the dependency graph
(`graph.Graph.HeaderUpdates`), never a summary's Meta cell. The cell is held to
the header by `checkHeaderLinks` ("The dependency graph").
<!-- source: internal/le/rfc/check_core.go -- checkUpdatedBy, updatedByRefusal -->

The marker changes no count. The row stays gated, tagged and counted in its own
RFC exactly as before: it says where the row is ordered, never that Ze stops
owing it (`TestAnUpdatedByMarkerChangesNoCount`).

## The intent marker

An optional row (SHOULD, SHOULD NOT, RECOMMENDED, NOT RECOMMENDED, MAY,
OPTIONAL) that Ze has not proven is in one of three states, and the summary
declares two of them with a marker of its own, beside the coverage annotation:

| State | Marker | Who sets it |
|-------|--------|-------------|
| not yet | `{intent: not-yet}` | Anyone during RFC work: Ze means to support the row |
| not ever | `{intent: not-ever; "<the RFC sentence making the row optional>"; <why>}` | Thomas only: Ze will not support the row. An agent may propose it in a report, never write it |
| not decided | no marker | The default |

```
- [ ] [RFC9999-2-4] [SHOULD] <text> (§2) {intent: not-yet}
- [ ] [RFC9999-2-5] [MAY] <text> (§2) {intent: not-ever; "A speaker MAY log every widget it discards."; Ze keeps no discard log}
```

Proven is never declared. It is computed from the discrimination records, so a
proven row counts as proven whatever marker it still carries, a marker left on
it is refused as stale (below), and `{intent: proven}` is refused as an unknown
state.

`parseIntent` (`internal/le/rfc/intent.go`) refuses, naming the format:

| Refusal | Fix |
|---------|-----|
| An empty body, or a state other than `not-yet` and `not-ever` | Write one of the two states, or remove the marker: an unmarked row is not decided |
| `not-yet` with a reason | Remove the reason: not yet records only that the work is owed |
| `not-ever` with no reason, no quoted sentence, or no `; <why>` after the quote | Quote the RFC sentence that makes the row optional, then say why |
| `not-ever` whose quote states no optionality | Quote a sentence carrying an advisory keyword, or `optional`, `optionally` or `if ... supports` (the optionality shape of a `{feature-declined}` quote) |
| Two `{intent}` markers on one line | Keep one: a row has one intent |
| `{intent}` on a gated (MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT) row | Remove it. A mandatory obligation is owed whatever Ze intends; record an unmet one with `{gap}` (`TestIntentMarkerRefusedOnMust`) |

The marker composes with a coverage annotation, `{superseded}` and
`{updated-by}` on the same line, one of each kind (`stripMarkers`). No gate can
see who wrote a `not-ever`, so the rule that only Thomas sets one is enforced
in per-commit review, not by a check.

`./le rfc check` holds the quote to the RFC's own words, as it holds a
`{not-applicable}` role quote: a `not-ever` quote must be a whole sentence of
the section the row cites or one of its subsections, from the word that opens
it to the full stop that ends it (`intentQuoteGap`, run by the exclusion
evidence check on every row, new or not; `TestNotEverQuoteIsAWholeSentenceOfItsSection`).

A marker on a row it cannot describe is stale, and `./le rfc check` refuses it
with "remove the intent marker": on a row whose annotation leaves it out of the
rows Ze owes (`{not-applicable}`, `{lower-layer}`, `{feature-declined}`,
`{linux-limit}`), and on a row a proving record covers, since proven is
computed (`evaluateIntent`, `TestIntentMarkerIsStaleOnAProvenOrExcludedRow`).
A marker on a `{gap}` or `{partial}` row stays: that row is still owed.
<!-- source: internal/le/rfc/intent.go -- parseIntent, parseNotEver, intentOnGatedRow, evaluateIntent, intentQuoteGap -->

`StemFigures` carries three counts per RFC, `NotYet`, `NotEver` and
`NotDecided`, over the optional rows that are owed, not proven and not
excluded (the attainable optional rows less the proven ones). Each such row
lands in exactly one, by its marker, and no other count changes
(`countIntent`, `internal/le/rfc/profilecoverage.go`, `TestIntentCounts`).
<!-- source: internal/le/rfc/profilecoverage.go -- countIntent, ceilingExcluded -->

## The dependency graph

`internal/le/rfc/graph` reads every text under `rfc/full/` and `rfc/drafts/`
into a graph with one node per text. Three edge kinds come from the texts, and
none is authored:

| Kind | From |
|------|------|
| `builds-on` | A citation in the Normative References section, never in a Non-Normative References one (RFC 3526 carries both). A text with no separate one gives its combined References section instead, which over-reaches: the graph orders RFCs and suggests candidates, and no edge makes an RFC a profile member |
| `updates`, `obsoletes` | The `Updates:` and `Obsoletes:` header lines, continuation lines included, or the front-matter sentence (`This memo obsoletes RFC 1234`) an old RFC states them in |

`Parse` (`graph/parse.go`) reads a heading only at column 0, because the table
of contents repeats every heading indented; it skips page headers and footers,
reads `RFC` wrapped onto the next line and `RFC-NNNN` alike, bounds an RFC
number to five digits, and cuts a draft's version suffix so a citation of `-05`
and a corpus file at `-16` are one node. The oldest texts (RFC 791, 905, 1071)
indent their References heading, and an indented line that is only that heading
opens the section too. `TestEveryCorpusTextYieldsReferences` holds every corpus
text to yielding a section, or to the named `noReferences` list.

`Build` (`graph/graph.go`) keeps an edge only when its target is a corpus
text, drops RFC 2119 and RFC 8174 (they say how to read a document, not what it
builds on) and self-edges, and points a `builds-on` edge at an obsoleted RFC to
its successor, the corpus text whose header obsoletes it, followed down the
chain. Header edges keep the RFC the header names, because they ARE the header.
Citation cycles (RFC 4301, 4302 and 4303 cite each other) collapse into one
component; a component's level is 0 when it builds on nothing in the corpus,
else one above the highest component it builds on.

`./le rfc update` writes the edges to `rfc/graph.tsv`, a derived file
like the ledger family. `./le rfc check` parses the texts itself on every run
(`graph.Load`), so no check reads a stale copy.

`checkHeaderLinks` (`internal/le/rfc/check_headerlinks.go`) holds each
summary's Meta lineage cells to the headers, because the header is the
authority. It refuses, naming the summary, the cell, both values and the fix:

| Cell | Must |
|------|------|
| `Updates`, `Obsoletes` | Name exactly the RFCs the summary's own header names, parentheticals cut; `None` or `-` for none |
| `Updated by` | Name no corpus RFC whose header does not update this one. An RFC outside the corpus is not judged, and an updater the cell omits is not refused: the graph holds that reverse relation, so the cell need not copy it |

`Obsoleted by` is not judged against the headers: it declares the successor the
`{superseded}` markers resolve against, a reading that can name an RFC whose
header says Updates (RFC 8210 restates RFC 6810), and `checkSuperseded` holds
it. A summary whose text is not in the repository has no header to compare.
<!-- source: internal/le/rfc/graph/parse.go -- Parse, headerLinks, proseLinks, referenceSection -->
<!-- source: internal/le/rfc/graph/graph.go -- Load, Build, collapse -->
<!-- source: internal/le/rfc/check_headerlinks.go -- checkHeaderLinks -->

### The priority tree: `./le rfc tree` and `./le rfc next`

A use-case profile's members are the summaries that declare it, in two tiers:
the required tier from each `Use cases` row and the optional tier from each
`Optional use cases` row (`useCaseScope`, the one place membership is built).
No edge adds a member. A technology aggregate's members are the summaries
naming it, split by their `Aggregate tier` row: required unless it says
`optional` (`scope`). The graph still orders the rows, roots before leaves, and it
answers the candidates.

`./le rfc tree` prints one row per node, roots to leaves: level, cycle
members, aggregate, `builds-on` and `updated-by` edges, both figures (or `not
walked`, or `no summary` for a text no summary covers), the use-case profiles
requiring it and, under `optional-profiles`, those listing it as optional.
Without `profile` it covers every text and summary, with the rows a release
profile requires first. `profile <name>` limits it to that profile's required
members, each with the declaration that put it there (`route-server: required
by rfc/short/rfc7947.md`), under the rollup the profile percentage is. Then
comes the optional tier, `optional members, not counted`, with its own rollup
and rows (`route-server: optional in rfc/short/rfc2918.md`); an aggregate's
optional tier is its members whose `Aggregate tier` is `optional`
(`ospf: optional in rfc/short/rfc4577.md`). Last, for a use-case profile only,
come the candidates: each `builds-on` target or updater of a member, in either tier,
that neither tier lists, once, with the member and edge that name it
(`rfc7454 builds-on rfc5082`), by level and then number. A candidate is
advisory: it is never owed and never counted, and listing one is a summary
edit an author decides. A level is printed `L?` for a summary whose text is
not in the repository.

`./le rfc next` names the next gap the owner rule orders, across every release
profile or inside `profile <name>`, from the required members alone. Until
their mandatory figure is complete it judges the mandatory register: an RFC
owes there when a mandatory row is owed and not proven, or when it is not
walked. The answer is the lowest level holding such an RFC, the lowest RFC
number at that level, and every member of its citation cycle owing at the
same level. A row marked `{updated-by}` is owed at the updater's level,
counted in its own RFC's figure. Once the mandatory figure is complete it
judges the optional register the same way, and says so. The optional tier
follows under `optional, not counted` with its own rollup: each optional
member that is not walked or holds an uncovered row of either register, and
never the answered gap. Across several profiles, a stem one of them requires
is counted there and not listed as optional.

A `{updated-by}` marker on a member requires its updater listed for the same
profile in a tier at least as high: required when the member is required,
either row when it is optional. `./le rfc check` refuses the marker otherwise,
naming the member, the marker, the updater and the row to write
(`checkUpdatedByMembership`). An updater that changes a member's text but
carries no marker is a candidate, not a member.

Both answer through `| json`. The roadmap's Profiles column reads the same
required members through `requiredMembers`, called by `readRoadmapCorpus`,
which the rfc package registers with the roadmap. It reads one committed
revision's parsed inputs only: `ParseProfiles` over the profile files and
`ParseMeta` over the summaries, and no RFC text. The optional tier places
nothing in that column, so it is not read there. Requirement ids map to stems
through `RequirementStems` (parsed summaries) or `LoadRequirementStems` (a
checkout), which read only the summaries.

The website publishes the same trees. `PriorityOf` answers the whole-corpus
tree and one tree per profile file over a `RenderInput` the caller already
holds, and the site's requirement ledger carries them: its `graph` holds every
node with its level, cycle and figures and every edge with its kind, and its
`profiles` one tree per profile. The `rfc-profiles` producer renders one page
per profile from them (`docs/contributing/gh-pages.md`, "Quality pages"). A row
read back from that JSON renders its cells through `TreeRow.Cells`, the cells
`./le rfc tree` prints.
<!-- source: internal/le/rfc/priority.go -- useCaseScope, scope, path, loadPriorityCorpus, priorityCorpusOf -->
<!-- source: internal/le/rfc/tree.go -- tree, treeRow, candidates, Cells -->
<!-- source: internal/le/rfc/priority_export.go -- PriorityOf -->
<!-- source: internal/le/rfc/next.go -- next, gapCandidates, optionalGaps -->
<!-- source: internal/le/rfc/check_core.go -- checkUpdatedByMembership -->
<!-- source: internal/le/rfc/roadmapcorpus.go -- readRoadmapCorpus, requiredMembers -->
<!-- source: internal/le/rfc/priority_export.go -- RequirementStems -->
<!-- source: internal/le/rfc/profile.go -- ParseProfiles -->

## The extraction sign-off

`./le rfc check` verifies that every requirement LISTED in a summary is covered.
It cannot know about an obligation nobody wrote down, so a green gate is bounded
by what was extracted. `rfc/extraction/<stem>.json` is the record of the walk
that fixes that bound, and it is a precondition of a new enrolment
(`checkEnrolment`).

| Step | Command or file |
|------|-----------------|
| Write the skeleton | `./le rfc extraction create stem <stem>` |
| Classify every derived site and section by hand | the file the command names, under this session's scratch |
| Apply a whole walk's decisions at once | `./le rfc extraction classify decisions <path>` |
| Move a hand-classified walk into the corpus | `mv <scratch>/rfc-extraction/<stem>.json rfc/extraction/<stem>.json` |
| Re-check the arithmetic | `./le rfc check` |
| Read the published backlog | `ai/RFC-REQUIREMENTS.md`, "Extraction sign-off" |
| Read the counts machine-readably | `./le rfc extraction report` |

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

**`extraction classify` applies a walk; it does not perform one.** A long RFC
derives hundreds of sites, so the decisions live in one file the reviewer
authors and the command transcribes: no default, no locator pattern, no
disposition the file does not name, and a refusal for a decision naming a site
the source does not derive. The same placement rule governs its output, so a
walk that left a site undecided goes to the scratch, and the report names every
such site with the `residual` note the file recorded for it. The kinds that cost
more are the two the rule presumes against: `binds-another-role` needs a
`producer` the reason carries, refused when it names a path this tree does not
hold, and `feature-out-of-scope` needs the sentence that makes the feature
optional quoted verbatim, checked against the RFC's own text or against a
document the reason names by number. The field-by-field contract is
`rfc/extraction/README.md`, "Applying a walk".

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
  `rfc2119` needs at least as many keyword sites as the summary's gated rows,
  less the ids a section lists in `unsourced-ids`: an id the walk sanctions as
  unsourced is not billed against the keyword budget
  (`sourcedGatedCounts`, `DeriveRegister`). A `prose` sign-off over a source
  that supports `rfc2119` is judged against the prose site set it walked
  (`InventoryUnder`).
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

`rfc/discrimination/<stem>/<requirement-id>.json` is what replaces reading
that prose. The file is a JSON array of that requirement's records. One
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
| `route` | `mutant` for a one-line substitution from a gomu-format report entry, `revert` for a producer function replaced by a halt, `no-break` for the escape |
| `producer` | the code the break was applied to, in the same key form. Required for a proof route. An escape names it too, unless its reason is `foreign-producer`, because the reason is a claim ABOUT that code |
| `producer-sha` | that function's behavior hash when the break was applied |
| `break` | what was done to the producer, derived from what was applied. No gate parses it; a reviewer reads it |
| `paired` | the further breaks applied together with `producer`'s in the one broken run, each a `producer`, `producer-sha` and `break` of its own. Absent on a single-break record. A paired producer that is gone or changed stales the record exactly as `producer` does |
| `solo` | present exactly when `paired` is: one entry per break, `producer` first then each paired break, holding `producer`, `break`, `seconds` and `green`, the line of a run with that break ALONE that showed the unit passing. Missing, short, out of order or naming another break is refused |
| `citation` | the assertion a proof or a `foreign-producer` escape rests on: a numbered `fail(N, ...)` site for an interop checker, a directive line for a `.ci`. Required for a functional or interop proof and for the `foreign-producer` escape, refused for a unit record and for the two escapes that name a producer. On a `kernel-producer` escape it is a span of the tag's own claim instead, and required |
| `reason` | why no break exists, for the escape only, out of the closed vocabulary below |
| `kernel-functions` | the Linux functions a `kernel-producer` escape rests on, each as `<linux source path>::<function>`, at least one and none twice (`kernel-function`, repeatable, on `./le rfc record`). Required there, refused on every other record |
| `ze-state-excluded` | on a `kernel-producer` escape, one entry per Ze producer the unit reaches through an import: `producer` (a key in a package the unit's file imports) and `reason`, at least eight words naming the producer's symbol (or file), never the same text as another entry's. Given on `./le rfc record` as `ze-state-excluded "<producer> <reason>"`, repeatable. Refused on every other record |
| `later-route` | the route, not built yet, that could still prove a `kernel-producer` escape: `patched-kernel`, the only value. Required there, refused on every other record |

A key names a method as `<path>::<Type>.<Method>`, by its receiver type
alone, so a pointer receiver and a value receiver are spelled the same way:
`text_append.go::LargeCommunity.AppendText`, `blackhole.go::ROACache.invalidByLengthOnly`.
The Go method expression `<path>::(*Type).Method` is refused, naming the key to
write instead. A bare `<path>::<Name>` names the one function or method of that
name in the file. Where several methods share it, the bare name is refused as
ambiguous, and the refusal lists the `<Type>.<Method>` key of each. One
exception: where the file also declares a package-level function of that name,
the bare name names that function, the one a Go caller reaches by the bare name
(flowspec's `unmatchable.go::CheckMatchable`). Every reader of a key resolves it
the same way: the fingerprint, the revert break, the coverage check that a
revert producer is reached, the interop checker's scenario, and the producer an
annotation's reason names (`{lower-layer}`, `{feature-declined}`, a never
clause, `{partial}`).
<!-- source: internal/le/rfc/goscope.go -- keyTexts, keyText, candidateKeys -->
<!-- source: internal/le/rfc/audit.go -- fingerprintKey, receiverFormRE -->
<!-- source: internal/le/rfc/discriminate_observe.go -- producerLineSpan, requireKeyReached -->
<!-- source: internal/le/rfc/check_core.go -- resolveProducer -->

The full artifact contract is `rfc/discrimination/README.md`.

`loadDiscrimination` (`internal/le/rfc/discriminate.go`) reads the tree,
`verifyDiscrimination` re-checks each record's fingerprints against the working
tree, `baselineRecordBlobs` (`internal/le/rfc/check_baseline.go`) reads what HEAD
holds for the same files, and `checkDiscriminationRatchet`
(`internal/le/rfc/check_ratchets.go`) judges what all three answered. Eleven
refusals exist today.

| Refuses | Why |
|---------|-----|
| A file that cannot be parsed, an unknown JSON key, a record whose `rid` disagrees with its file name, or a per-RFC file of the retired layout | A corrupt record must never read as a corpus with nothing proven |
| A polarity, a route, a key or a fingerprint outside its closed form | A half-read record is the shape a false proof takes |
| A record naming a requirement no summary declares | A proof of an obligation nobody wrote down proves nothing |
| Two records claiming one requirement, polarity and tagged unit | The proven count is published, and a duplicate inflates it |
| A record whose `producer` no longer resolves in the tree | The break was applied to code that is gone |
| A record whose `unit-sha`, `claim-sha` or `producer-sha` no longer matches COMMITTED code | Nothing observed the red over the code that was committed, or the red was observed about a different sentence, so a hand-written record is refused by the same rule that catches a real drift. The drift is judged against HEAD, never against the working tree (owner decision, 2026-08-31): several sessions share this checkout, so one session's uncommitted edit to a producer would otherwise red the gate for all of them, and clearing an interop record costs a 576-second re-record. A record staled by an edit nobody has committed is REPORTED on a `discrimination:` line of its own, counted as proven by nothing, and becomes a violation at the commit that carries the edit. HEAD and the tree are compared at the granularity the record FINGERPRINTS, which is the producer or unit FUNCTION: comparing whole files let any unrelated uncommitted edit elsewhere in that file silence the author's own violation |
| A tagged unit the TIP COMMIT added against `HEAD^`, on an enrolled RFC's gated requirement, carrying no verified record | The obligation is what a CHANGE adds. A floor that starts at zero and only forbids going below zero proves nothing. Both sides are COMMITTED (owner decision, 2026-09-01): a tag sitting only in somebody's working tree bills nobody, and Thomas's release check, `./le verify worktree`, checks the commit under test out detached, where a tag that commit added IS the tip. A carrier file the tip RENAMED with its bytes unchanged added nothing: the baseline follows a rename whose blob id is identical, so its covers keep their proofs and owe none. A rename that changes even one byte is reviewed as an edit, and every unproven cover in it is owed |
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
the RFC 7606 audit verdicts recorded does not repeat here. Measured over this
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
bystander and in front of the author never, because Thomas's release check, `./le verify worktree`,
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

Records are written in the implementation phase, by the agent that adds or
changes the tagged unit, and they land in the same commit as that code
(`/ze-implement` step 5). Spec closure only checks them with `./le rfc check`
(`/ze-close` step 6), so a record stale at closure is a defect of the phase
that changed the unit. The rule is in `ai/rules/planning.md`, "Work Phases and
the Review Loop".

`./le rfc record` is the only writer of a record, and it writes one
only after it has SEEN the red. It applies the break, runs the tagged unit,
requires a failure that NAMES that unit, and refuses everything else. A run that
stayed green records nothing, and a run that went red without naming the unit
records nothing either: a build error, a sibling test and a flake each turn a
run red, and none of them says the claim's own test discriminated the break.

For a Go unit, the name is the unit's own `--- FAIL: <Func>` line, or a
subtest's `--- FAIL: <Func>/...` line. The match is on the whole name, so
`--- FAIL: TestWidgetSibling` does not name `TestWidget`. The unit runs under
`go test -v`. A revert halt can kill the binary from a goroutine the test does
not own, and then no FAIL line prints. That crash counts only when the output
carries the halt's own text and when `=== RUN   <Func>` comes before it. A halt
reached from package `init`, or from a package-level `var` initializer, fires
before any test starts. Every test in the package shows that red, so it is
refused as a red from initialization. Break a function the unit reaches, or
use the mutant route inside the value that the initializer returns
(`observationRunner.killedByTheBreak`).

| Route | The break | The runner |
|---|---|---|
| `mutant` | one report entry's `original` text replaced by its `mutated` text on its own line | `go test -v -run '^<Func>$'` over the tagged unit's package, under a Go `-overlay` |
| `revert` on a `.ci` | the producing function's body replaced by a halt | ONE `.ci`, against the isolated set `testfunctional.Prepare` builds under the same overlay: `le test <suite> <name>` for ordinary suites, `le test exabgp <suite> <absolute-ci-path>` for `test/exabgp-compat/` |
| `revert` on an interop checker | the same | the `Action` of the suite whose catalog `Package` holds the checker (`./le test integration interop-ipsec` for `internal/le/interoplab/ipsec/`), with that catalog's `SelectorVariable` (`IPSEC_INTEROP_SCENARIO`) set to the scenario the checker's own `const name` declares, confirmed under the catalog's `ScenarioRoot` |

The `mutant` route reads the report and does not check how it was made.
`loadGomuReport` decodes the `mutant` object of each `results[]` entry:
`filePath`, `line`, `column`, `original` and `mutated`. It reads `status` only
to mark a KILLED entry, and only the proposer below uses that mark.
`selectMutant` picks the one entry whose `<file>:<line>:<column>#<n>` selector
matches, whatever its status. `replaceOnLine` then requires the line to exist and
to carry `original` exactly once, and replaces that one occurrence. It
then requires the red to name the unit, as it does for every route. So the
evidence is the red the recorder observed, never the tool that wrote the entry.

A hand-written entry is how one statement is broken when a function holds more
than one check. `revert` replaces the whole function, so it proves that the unit
depends on the function. It does not prove that the unit's assertion catches the
one violation the claim names. Write the report in gomu's JSON shape, with one
`results[]` entry. Its `original` is a span that the line carries exactly once.
Its `mutated` text must still compile. A build failure is a red that does not
name the unit, so the recorder refuses it. Label the entry as hand-written in
three places, because the record stores only `route: mutant` and the derived
`break` text. Set `status` to `HAND_WRITTEN`, never `KILLED`, so that the
proposer never offers it. Write an `id` and a top-level `note` that say what the
break removes and why (the decoder ignores both). In the commit body, state that
the break was written by hand. The selector is `<filePath>:<line>:<column>#1` for
a single entry at its position. An independent judge reads the `break` text and
decides whether it is the violation the claim names (`ze-test-judge`).
<!-- source: internal/le/rfc/discriminate_propose.go -- loadGomuReport, reportMutant.selector, reportMutant.text -->
<!-- source: internal/le/rfc/discriminate_action.go -- mutantBreak, selectMutant, replaceOnLine -->

    {
      "note": "Hand-written single break: removes only the all-routers JoinGroup call in openRASocket.",
      "results": [{
        "mutant": {
          "id": "hand-written:sender_linux.go:joingroup-removed",
          "filePath": "internal/plugins/iface/ra/sender_linux.go",
          "line": 128, "column": 2,
          "original": "pc.JoinGroup(dev, &net.IPAddr{IP: net.ParseIP(allRoutersGroup)})",
          "mutated": "func() error { _ = dev; return nil }()"
        },
        "status": "HAND_WRITTEN"
      }]
    }

One record may rest on a SET of breaks applied together. Some properties are
guarded twice on the same path, by design ("Pair the check",
`docs/contributing/ze-go-style.md`): the RFC 4271 Section 5.1.2 rule that a
withdrawal carries no created AS_PATH is held both by the dispatch in
`ASPathEdit.Record` (`wireu/aspath_slot.go`) and by `planAttr`'s advertise gate
(`reactor/forward_build.go`), so breaking either alone leaves the scenario green
and neither alone is the red. The mutant route takes `mutant` more than once:
every named mutant is applied to the one broken run (two in one file land in that
file's single replacement), the clean run stays untouched, the first selector
becomes the record's `producer` and every further one a `paired` entry. A
selector named twice is refused.

The recorder then runs each break of the set ALONE, applied to the tree's own
copy of its file, and requires every one of those runs to leave the unit green.
It runs them on every carrier kind, and stores them in the record's `solo`
list, one entry per break in the order applied, `producer` first, each with its
duration and the line that showed the unit passing. Together with the red of
the whole set this shows that every break is needed: a break that reddens the
unit alone is refused, with the instruction to record it by itself, and a break
the unit never executes leaves the set behaving as the remaining breaks alone,
which their own solo runs show green. On a unit carrier, host or Linux guest,
the clean run also builds a coverage profile for a paired record, and each
paired producer must show as executed in it, exactly as a revert producer must.
A functional or interop carrier is a second process, so no profile reaches it,
and the solo runs are its evidence. `./le rfc check` refuses a paired record
whose `solo` list is short, out of order, names another break or holds an empty
green line, and a single-break or escape record that carries one, as an
unloadable record. A paired record with no `solo` list at all, taken before the
recorder ran solo runs, still loads, so the derived files every hook builds stay
readable, and its verdict is `solo-owed`. That verdict is a violation wherever
the record stands, committed or not, because no working-tree edit can explain a
proof that was never taken, and the violation names the repair: re-take the
record with `./le rfc record`, which runs each break alone. It
re-checks each paired producer's fingerprint against HEAD as it does
`producer`'s. It cannot re-run the solo runs: the record keeps what the recorder
observed, as it does for the red.
<!-- source: internal/le/rfc/discriminate_action.go -- mutantBreak, selectMutant -->
<!-- source: internal/le/rfc/discriminate_observe.go -- observeBreaks, requireSoloGreen, wantsProfile -->
<!-- source: internal/le/rfc/discriminate.go -- PairedBreak, SoloGreen, validateSoloRuns, ProofSoloOwed -->
<!-- source: internal/le/rfc/discriminate_validate.go -- validatePairedBreak -->
<!-- source: internal/le/rfc/check_ratchets.go -- checkDiscriminationRatchet, discriminationSoloOwedError -->

    ./le rfc record id <ID> polarity negative unit <file>::<Func> \
      route mutant report <report.json> mutant <file:line:column#n> \
      mutant <file:line:column#n> citation <N>

The suite comes from the interoplab catalog each lab registers in its
`register_catalog.go`, the same registry that maps an interop tag to its
carrier, so every lab with a catalog can carry a record and no suite table
exists in `internal/le/rfc`. A checker in a package no catalog declares is
refused before anything runs, naming the registration it lacks.
<!-- source: internal/le/rfc/carriers.go -- interopTrees, interopSuiteOf -->
<!-- source: internal/le/rfc/discriminate_observe.go -- carrierArgv, environment, interopScenarioOf -->

The compatibility runner's positional path selects exactly that file; `--pattern`
is a substring filter and can select siblings, so the recorder does not use it.
The suite comes from the tagged file's directory, not the ordinary functional
suite table: `functional-exabgp` has its own native action. Unknown ordinary
suites still refuse before building. Both routes keep the clean-before-break
check and build the broken binaries with the producer overlay in `GOFLAGS`.
Compatibility observations need the same explicit loopback setup as their native
runner, including inside a guest; see
[ExaBGP compatibility test ports](../functional-tests.md#exabgp-compatibility-test-ports).
<!-- source: internal/le/rfc/discriminate_observe.go -- functionalSelection and runFunctional -->

The scenario binding accepts both `const name = "<scenario>"` and a grouped
`const` declaration containing `name = "<scenario>"`. In either form the named
checker must resolve uniquely and the scenario must exist in the tree.

A Go unit runs where its own build constraints hold, and the file decides, not
the author. `unitNeedsGuest` (`internal/le/rfc/discriminate_guest.go`) asks
`go/build` of the tagged file, reading the `//go:build` line and the `_linux`
file-name suffix the way the go command reads them. The host comes first, under
the tag set the host run passes. A file the host cannot compile is asked of the
QEMU guest: `linux`, the guest's architecture, and the same tags plus
`integration`, which is how a test that touches the kernel is marked
(`ai/rules/platform-linux.md`). A file neither compiles is refused, because a
unit nothing ran cannot have gone red.

A guest unit is compiled on the HOST with `go test -c`, cross-built for the
guest and under the same overlay, so the break still reaches the compiler and no
file is modified. The binary then runs inside the guest through
`./le test qemu run`, selected to the one unit with `-test.run '^<Func>$' -test.v`
from its package directory, and the coverage profile is written through the
shared checkout where the host reads it. The guest script first copies the
binary to the guest's own `/tmp` and runs that copy, never the file on the
shared checkout (`guestScript`). The checkout reaches the guest over 9p with
`security_model=none`, so its files keep the host owner's uid and modes, and
the checkout root is commonly closed to other users. A unit that re-executes
itself in a new user namespace, as the SAD-selector probes do, holds no
capability over an inode whose owner that namespace does not map, so its exec
of a checkout path is refused with `EACCES` and the unit fails, observing
nothing. The guest-local copy is owned by the guest's root, which the child
namespace maps. A coverage run also names a guest-local counter directory with
`-test.gocoverdir=`, as `go test -coverprofile` does on the host: the binary
builds its profile from every counter file in that directory, and a unit that
re-executes itself passes the flag on, so a producer only the child reaches
still appears as executed. Without it the binary picks a private directory the
child never learns, and the recorder refuses the unit as never reaching its
producer.
<!-- source: internal/test/userns/userns_linux.go -- Enter fails on re-exec errors -->
<!-- source: internal/le/rfc/discriminate_guest.go -- guestScript, guestLocalDir --> Attribution is unchanged: the red must
name the unit. A guest proof MUST boot Ze's runtime kernel, so the recorder
takes `kernel <vmlinuz>` for a guest unit and refuses to run one without it, and
refuses the keyword for a unit that runs on the host (`requireGuestKernel`):

    ./le rfc record id <ID> polarity <p> unit <file>::<Func> \
      route revert producer <file>::<Func> kernel tmp/kernel/build/vmlinuz

A `.ci` the host's runner skips takes the same guest. The functional runner
decides, not the recorder: `testrunner.NeedsGuest` answers whether this host
skips the file (`option=needs-linux:caps=...` on a host that lacks the
capability), and a file it skips runs in the guest; a file this host runs keeps
the host run, and an ExaBGP compatibility file never asks. The daemons are
cross-built on the host into `tmp/qemu/rfc-discriminate/pid-<pid>/`, with the
overlay in `GOFLAGS` for the broken run, and the guest runs the one file through
`le test qemu all test <path>`, the narrowing `./le feature run`
already drives (`testqemu.OneCICommand`). The guest le that `le test qemu run`
rebuilds into the shared `tmp/qemu/` path is built after `GOFLAGS` is restored,
so it never carries the break. The same `kernel <vmlinuz>` rule applies:

    ./le rfc record id <ID> polarity <p> unit test/<dir>/<name>.ci \
      route revert producer <file>::<Func> citation '<directive>' \
      kernel tmp/kernel/build/vmlinuz

On either placement a `.ci` observation is judged by the runner's result line
for that file, never by the exit code alone. A skipped `.ci` exits 0 like one
that passed, so the clean run must print its `PASS` line and a run under the
break must print `FAIL` or `TIME`; a `SKIP`, or no line naming the file, is
refused as unobserved (`cleanRunUnobserved`, `brokenRunUnobserved`). Before
2026-10-20 a host without `CAP_NET_ADMIN` read a skip as a clean green and its
second skip as "stayed GREEN under the break".
<!-- source: internal/le/rfc/discriminate_guest.go -- functionalNeedsGuest, runFunctionalInGuest -->
<!-- source: internal/le/rfc/discriminate_observe.go -- cleanRunUnobserved, brokenRunUnobserved -->

Before 2026-09-27 the unit route always ran host `go test` with no extra tags,
so an `integration && linux` unit never compiled and its tags could not be
proven at all (journal row 155 of `gate-excludes-part-of-its-population.md`).

Adding `report <path>` to `./le rfc discriminate` turns it into a PROPOSER: it
prints the candidate breaks for each unproven unit tag, best first. Two filters
and one ranking. A candidate must be a mutant gomu recorded as KILLED, because
NOT_VIABLE does not compile and SURVIVED is noticed by no test in the package.
It must lie in code the tagged unit's own coverage profile executes, because a
mutant the unit never reaches cannot redden it. The rank is the count of symbols
the tag's own prose names that the break's text touches, which decides what is
offered first and nothing else: the gate never judges whether a break is a GOOD
break.

Every carrier applies the break through a Go overlay. `writeOverlay` stages the
replacement and overlay in a private directory for that observation, and
`requireRed` passes it to the carrier's runner. The shared producer is never
modified, even while the broken run is executing. Killing an observation cannot
leave that producer broken for another session.
<!-- source: internal/le/rfc/discriminate_observe.go -- writeOverlay, requireRed -->

For interop, the recorder re-executes the native action with the scenario
selector and `GOFLAGS=-overlay=<path>` in the child's environment. The native
lab's `StageBinaries` cross-compiles on the host with that environment, and the
Docker images copy in the resulting binaries. No source mutation or restoration
is needed.
<!-- source: internal/le/rfc/discriminate_observe.go -- carrierArgv, environment -->
<!-- source: internal/le/interoplab/zebuild.go -- StageBinaries, stageBinaries, stageBinary -->

Child stdout and stderr stream to the recorder's stderr as they arrive, while
the complete combined output is retained for attribution and the record's
excerpt. Structured result stdout stays separate. This exposes real progress to
the job log rather than hiding it until the child exits; it adds no heartbeat,
so a silent child stays silent.
<!-- source: internal/le/rfc/discriminate_observe.go -- exec, judgeRed, excerpt -->

An interop observation is judged by the scenario's own progress line, never by
the exit code or the `interop: N passed, M failed` summary. The clean run must
print `interop: [i/n] <scenario>: pass (`, and the run under the break must
print `interop: [i/n] <scenario>: FAIL (`, and the cited number is read where
the checker writes it, the anchor `scenario <scenario> assertion <citation>:`
(`checkerFailure`, `internal/le/interoplab/bgp/check_engine.go`).
`checkerFailure` answers a cause that is already its own report unchanged, so a
special checker's failure is no longer wrapped a second time under
`assertion 0`. A failure that still carries two links, as logs written before
that change do, `scenario S assertion 0: scenario S assertion 6: <cause>`, is
read at its innermost link, each link starting where the previous one's colon
ends.
A cause text that quotes another assertion's number is therefore never read as
the citation. A run that executed
no scenario is refused as unobserved, and a FAIL that names another assertion is
refused as not the record's. Before 2026-10-23 any interop red counted: a mutant whose ze
cross-compile failed ended the run with `0 passed, 0 failed` and was sealed as
a negative proof.
<!-- source: internal/le/rfc/discriminate_interop.go -- interopCleanUnobserved, interopBrokenUnobserved, interopAttribution -->

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
| `kernel-producer` | the Linux kernel produces the behavior from the packet alone, and no state Ze installs decides it, so no break of Ze's code can falsify the claim | the carrier kind is `unit` and go/build compiles its file for Linux and for no other system; the record names no producer; the RFC summary's `Implementation` is `mixed` or `third-party` and its `Implementation reason` names each of the record's `kernel-functions` verbatim, as a whole token; every Ze package the unit's file imports, `internal/test/` apart, is named by a `ze-state-excluded` entry whose producer the tree holds | the `citation` is a span of at least three words of the tag's own claim, whitespace collapsed as `claim-sha` collapses it |

`kernel-producer` is the one reason whose break exists, just not in this
repository: a kernel carrying one patch that breaks a `kernel-functions` entry would turn
the unit red. That is why every such record also names `later-route:
patched-kernel`, and why it publishes as escaped, never as proven. The route
that would replace it is designed in
`plan/post-release/kernel/spec-rfc-kernel-patch-discrimination-route.md`, and
its population is every record carrying this reason. The checks are
`escapeCheck.kernel` (`internal/le/rfc/discriminate_escape_kernel.go`) and
`validateKernelEscape` (`internal/le/rfc/discriminate_validate.go`). Where a state Ze installs
is the criterion the claim names, the operand the kernel's decision reads,
the row takes a proof route that breaks Ze's producer instead, as
`RFC792-Unreachable-1` does with `SetMTU`: the MTU is what the
fragmentation-needed decision compares the datagram against. A state that only
enables forwarding is not that criterion. A route that merely makes the
datagram reachable is a precondition held constant across both halves of the
claim, so breaking it turns the test red without falsifying the decision the
claim names, and such a row stays a `kernel-producer` escape with that route
named in a `ze-state-excluded` entry. The gate asks this of every Ze package the unit imports: each needs a
`ze-state-excluded` entry saying why its state does not decide the outcome,
so a unit that reaches Ze code cannot take the escape in silence. The gate
checks the entry's shape and that its producer exists; whether the reason is
true is a reviewer's question. Every imported Ze package is asked, not only
those gomu mutates, because that answer needs a walk of each package the gate
does not make, and one entry too many costs less than one too few.

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

## Moving a tagged test file

Every fingerprint in a record and in an audit verdict is independent of the
file's path, and every KEY names the path. A move therefore breaks nothing but
the keys, and `./le rfc move` rewrites the keys rather than re-stamping
anything.

The rename action removes its sources immediately; it does not prepare a
commit. For tracked-file relocation, use the commit-owned `file`/`remove`
workflow in [Committing](committing.md) instead. An untagged file with no
evidence keys can be copied byte-for-byte to its destination while its source
stays in place until the commit script removes it. Historical record citations
keep their original paths.
<!-- source: internal/le/rfc/rename.go -- applyRename -->

```
./le rfc move from <old> to <new>
./le rfc move plan <file>
./le rfc move propose <file> [under <dir>]
```

`from`/`to` moves one file and `plan` moves every `<old> <new>` line of a file.
The move is byte-pure, so the HEAD^ and origin/main baselines follow it (see
the obligation row above), the weakened-test audit and the RFC-change commit
gate read it as no change, and it needs no `RFC-approved:` trailer. After the
move the action rewrites every record's `unit` and `producer` naming the old
path in `rfc/discrimination/`, and every requirement's `tests`, `units` and
`code` key naming it in `rfc/audit/`. It finds those fields by walking the JSON
and edits their bytes in place, so a `break`, a note or any other string that
spells the old path keeps its bytes, and so do the formatting and another
session's hunks in those files. It rewrites every backtick or link citation of
the old path in a tracked file the link sweep polices, by the sweep's own
grammar (`Paths` in `internal/le/doc/citation`), and lists every other line of
such a file that mentions the file's base name, for a reader to judge. The
sweep's exemptions decide which files those are (`Policed` in the same
package): a record tree such as `plan/journal/`, `plan/learned/` or a spec
keeps the old path byte for byte and is not listed, because a
path in a record is a fact about the day it was written. The learned indexes
and the other live files under those trees stay policed and are rewritten. A citation the grammar expands from
braces holds no literal path to rewrite, so a line that still cites the old
path after the rewrite is listed as stale, to be edited by hand. Two such
citations go unlisted: one inside a moved file, because a moved file is never
searched for them, and one whose braces span a directory segment, because the
search looks for the old directory or the base name spelled out. A moved file
is never edited. When a write fails part-way, the action answers 2 with the
report of what it did write: the moves, the rewrites, and each target created
whose source is still in place.

It refuses the whole batch, and writes nothing, for a target that exists,
leaves its source's directory, changes the GOOS/GOARCH file-name suffix or is
not a `_test.go` file; a source that is untracked or differs from HEAD; two
pairs sharing a source or a target; a target the unit carrier holds that the
naming rule below refuses for the source's tags; an evidence file that is not
one JSON value, or whose path field spells the old path with an escape such as
`\/`; and an evidence or cited file another session changed while the rename
ran. The suffix is judged by
`go/build` itself: the rename is refused when the source and the target names
build on different sets of the platforms `go tool dist list` names, so a suffix
`go/build` knows and no port carries, such as `_sparc` or `_zos`, counts too.

`propose` plans a prefix addition only when a file's tags cite one RFC but its
name cites none. An untagged RFC-named file offers a choice of tag, marker or
rename, so it produces no automatic pair. Each proposed pair is
judged by `pairRefusals`, the predicate the rename applies to a pair, so a
source that is untracked or differs from HEAD, and a target that already
exists, moves the build suffix or fails the naming rule, is left out and named
with its reason, as is a target two findings share, and so is a file named for
another RFC, which has to be read before it moves. No pair in a proposed plan
draws a refusal from its paths, its source's state or its target. A refusal
judged on an evidence record, an escaped path spelling or evidence that is not
one JSON value, is judged by the batch's rewrite alone, so `propose` does not
see it, and the rename then refuses the whole batch rather than writing it. The output
file is created, never overwritten. Run `./le rfc update` after a rename.
<!-- source: internal/le/rfc/rename.go -- renameFiles, planRename, refusePair, pairRefusals, planCitations, proposeRenames, judgeRenameTarget -->
<!-- source: internal/le/doc/citation/policed.go -- Policed, Excluded -->
<!-- source: internal/le/rfc/check_baseline.go -- exactRenamesSince, coversAt -->

## Test file names

A unit test file's name and its `RFC requirement:` tags agree. A file whose
proof and gap tags all cite one stem is named for it, `rfcNNNN_<topic>_test.go`.
A file named for a stem carries at least one tag for it, or this marker with a
reason on the same line:

```
// RFC naming: untagged -- <reason>
```

The prefix of a stem is the stem with its hyphens turned into underscores, then
one underscore: `rfc792_`, `sflow_v5_`, `draft_ietf_sidrops_8210bis_`, with no
zero padding. A file name is read back to the longest stem prefix it opens with,
and a name opening `rfc<digits>_` is named for that RFC even when no summary
exists. A file whose tags cite two or more stems may carry any name that does
not claim a stem it never tags. Only `_test.go` files the unit carrier holds are
judged, so interop carriers, `.ci` files, `internal/le/`, `test/draft/`,
`testdata/` and `vendor/` are not. Each finding names the marker to write, or
a `./le rfc move` command: the exact one when the rename would take its
target, and one whose `<topic>` the reader chooses when it would not.

The repair keeps the old name as its topic, less every spelling of the target
stem it already carried: the stem itself (`gtsm_rfc5082_linux_test.go` becomes
`rfc5082_gtsm_linux_test.go`), the legacy `rfc_<stem>` (`rfc_sflow_v5`), and for
a draft the legacy `rfc_draft_<word>` and `rfc_<word>`, where the word is one of
the draft name's own after `draft` (`rfc_draft_abraitis`, and `rfc_mup` for
`draft-ietf-bess-mup-safi`). A draft word standing alone is kept, so
`rfc_mup_safi_test.go` repairs to `draft_ietf_bess_mup_safi_safi_test.go`. The
trailing elements `go/build` reads as a GOOS/GOARCH suffix are never removed. A
topic that held nothing else repairs to the bare `<stem>_test.go` whether or not
a file has that name.

`./le rfc check` runs `checkTestFileNames` over the whole tree on every run, not
only where the enrolled sets of HEAD and HEAD^ meet, because a name and its tags
disagree whatever the baseline holds. Two renames are named: the repair of a
file whose tags cite one other stem, and the rename offered to an untagged file
named for a stem, which drops the stem and keeps its topic. Either is named as a
command only when the rename would take that target. When the target exists,
when two files' renames take the same name, or when the new name changes which
platforms `go/build` compiles the file for (`linux_test.go` would become
`rfc5082_linux_test.go`, which builds on Linux alone, and `rfc5881_linux_test.go`
would become `linux_test.go`, which every platform builds), or when the naming
rule refuses the new name for the file's own tags and marker (an untagged
`rfc5881_rfc7311_test.go` would become `rfc7311_test.go`, named for an RFC it
never tags), the finding says so
and asks for a hand-chosen topic, `./le rfc move from <file> to
<dir>/<prefix><topic>_test.go`, with no prefix for the untagged file, rather
than a name that carries the stem twice or a command the rename refuses. An
untagged file named for its stem alone has no topic to keep, so its finding
names the `<topic>` form directly. `./le rfc move propose` leaves out of its plan
every pair the rename would refuse, and a shared target, for the same reason.
The finding, `propose` and `./le rfc move` judge a pair through one
predicate, `pairRefusals`. It answers a path outside the checkout, a pair that
leaves its package or names a file that is not a `_test.go` file, a source that
is untracked or differs from HEAD, and, through `judgeRenameTarget`, a taken
name, a moved build suffix and a name the naming rule refuses for the source's
tags and marker. A finding judges names, not the working tree, so it sets the
source's state aside: a name and its tags disagree whoever is editing the file.
A command a finding names therefore draws none of the refusals one pair draws
from its paths, its source's state and its target once its source matches HEAD,
and `propose`, whose pairs run now, leaves an edited source out. A refusal
judged on an evidence record, an escaped path spelling or evidence that is not
one JSON value, is judged by the batch's rewrite alone, so neither the finding
nor `propose` sees it, and the rename refuses the batch rather than writing it.
Two files sharing a target is the one refusal a single pair cannot see, and the
check counts it across its findings.
<!-- source: internal/le/rfc/names.go -- stemPrefix, stemOfFileName, judgeTestFileName, topicForStem, checkTestFileNames, verdictRename, repairBlocked, pairBlocked -->
<!-- source: internal/le/rfc/rename.go -- pairRefusals, judgeRenameTarget, refusePair -->
<!-- source: internal/le/rfc/check.go -- check -->

## What the ratchets cannot see

A tagged test whose assertions are weakened IN PLACE, while the shape stays the
same, is caught by `checkDiscriminationRatchet` once that test carries a record:
the weakened body changes `unit-sha`, the record stops verifying, and the gate
refuses it. Until a test carries one it is grandfathered, so three other
mechanisms carry the standing corpus:

- `writeWeakening` (`internal/le/hookruntime/writeedit.go`) refuses the edit at
  write time, through `testweakened.Proposed`
  (`internal/le/test/weakened/proposed.go`). It blocks a behavior change to a
  test carrying an `RFC requirement:` tag, and separately blocks REMOVING the
  tag. Removal is checked first and on its own, because a tag is a comment and a
  behavior comparison would wave its deletion through. Scope is the enclosing
  test function: a tag sits on the doc comment, so a hunk-scoped guard would
  miss exactly the edit it exists to stop.
- `./le test weakened check` checks the same at commit time.
- `checkAuditFreshness` (`internal/le/rfc/check_audit.go`) is the SHA ratchet,
  armed only for an RFC that has a verdict under `rfc/audit/<stem>/`.

The exclusion evidence ratchet cannot see a red commit once it is pushed. It
counts a lacking row only when `HEAD^` and `origin/main` both hold it
unchanged, so a commit that added a lacking row and was pushed while red
leaves the row in both baselines: from then on it is counted as backlog, not
refused by id, and only a committed line that would have to rise to carry it
still refuses. Pushing a red commit is what makes its lacking rows countable.

What none of them can see is the tag that OVER-CLAIMED from its first commit.
Each of the three is a CHANGE detector, so a test that never asserted what its
tag says has nothing for them to compare against. That is the hole
`rfc/discrimination/` closes, and it closes it only where a record exists: the
count of tags that carry one is published in `ai/RFC-REQUIREMENTS.md`, under
"Claim discrimination", beside the backlog that does not.

An RFC-tagged test changes only with the approval of an independent agent
running the `ze-test-judge` skill (`ai/skills/ze-test-judge.md`), never put to
Thomas. The judge records its verdict with
`./le rfc approve unit <package>.<TestName> reason "<the judge's verdict>"`,
and the commit carries it as an `RFC-approved:` trailer;
`docs/contributing/rfc-implementation-guide.md` says what the reason names.

RECORDING is the judge's job, so it runs the command whenever it approves. What
the command must never carry is an approval nobody gave, so the reason is the
judge's verdict sentence and never the author's own argument for why the change
is acceptable: the agent that made the change neither judges it nor records it.
