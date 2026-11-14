# Spec: rfc-figures-built-and-proven

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | Accurate support claims: the published RFC figures draw one "Ze correctness gap" segment over rows that are not built and rows that are tested but carry no recorded proof, so a reader cannot tell an absent feature from a pending proof |
| RFC gaps | - |
| Depends | - |
| Phase | 5/5 |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

WIP cap: Thomas gave a one-off exception on 2026-11-13 for this spec to be
claimed above the cap (19 in flight against a cap of 12). It covers this spec's
implementation only and raises no cap.

## Task

The published RFC figures answer "how much of what the RFCs require has Ze
proven". On the site data of 2026-11-10 that is 2,150 proven of 4,752 owed MUST
rows, 45.2%, over 196 implemented RFCs, all walked. The other 2,602 rows are
drawn as two segments: "OS testing gap" (239) and "Ze correctness gap" (2,363).

The second segment merges two populations that mean different things to a
reader. Counted from the generated requirement tables on 2026-11-13, about
1,460 of its rows are ones the summary itself declares unmet (`{gap}`,
`{partial}`), and about 890 are rows that carry tagged tests and no verifying
discrimination record. The first is a feature or behavior that is not built.
The second is built and tested, with its proof not yet recorded. Publishing
both as a "correctness gap" under-reports the software.

Thomas ruled three things on 2026-11-13, recorded in `ai/META-DECISIONS.md`
("Goal and scope"):

| Ruling | What this spec delivers |
|--------|-------------------------|
| A requirement that has tests but no recorded proof is published apart from one that is not built, never under one heading | The unproven Ze-owned MUST rows are split into four open sections (proof pending, exclusion unchecked, not assessed, not built) on every surface that draws or prints them. The design had two; the third and fourth came from Thomas's later decisions, recorded below |
| A requirement the operating system meets counts as done in every published figure once its test proves it, unless a rule names a reason to keep it out | Every published proven count and percentage counts OS verified rows with Ze proven rows |
| A public page says what is not built and says nothing about whether or when it will be | No public page or published data file carries an intent (never, not yet, undecided) for an unmet requirement. The open optional rows are split the way the MUST rows are, into the same four open sections |

Thomas's words on the third ruling: "we should effectively document what we
will not build, or what is not currently built and perhaps don't say anything
about if it will be done or not because that changed too easily." He also
ruled that the intent marker stays in the summaries as a private planning
fact: only the public surfaces go quiet.

Scope decisions Thomas made at the scope gate, 2026-11-13:

| Question | Decision |
|----------|----------|
| Order against `spec-site-rfc-recorded-data` | This spec lands first. Thomas first chose the other order, then reversed it at the design gate: that spec is in design with nothing built, so its recording takes the split shape as its first schema and this spec owes it nothing |
| Recording the missing proofs, correcting wrong `{gap}` marks | Not in this spec. This spec changes how rows are counted and shown, not which rows are proven or marked |
| Denominator | Unchanged. A row that is not built stays in the MUST rows owed, so the split alone moves no percentage |

Design decisions Thomas made at the design gate, 2026-11-13:

| Question | Decision |
|----------|----------|
| Name of the second MUST section | "Proof pending" |
| Open optional rows (SHOULD, MAY) | Split like the MUST rows: not built and proof pending. The alternative, one "not proven" count, was set aside by Thomas |
| Readers of the data field names outside the repository | None, so the fields are renamed directly |

## Independent review, 2026-11-13

An independent review on the development tier read this spec against the
source after Thomas approved it, and found two blockers and eleven issues. The
main thread confirmed each at the producer. The spec went back to `design`,
and the sections below carry the corrections. The one decision it left open,
where a not-owed claim with unchecked evidence is drawn, Thomas settled the
same day ("Which row lands where", row 6), and the spec returned to `ready`.
Where an acceptance criterion or a test below names "the two sections", the
third section of row 6 is counted with them: the sections sum to Kept, and the
register needs all of them empty.

| Finding | Correction in this spec |
|---------|-------------------------|
| The phases could not land in order: non-test code in health and the whole site package read the removed fields, so `le` would not build between phases | One change set, in the order of "Implementation Steps" |
| The site's Proof cell is rendered from a tree row read back from JSON, which holds no OS verified count, so it would disagree with the status page | The published proven count itself counts OS verified rows, so a row read back holds it |
| "Proof pending" would hold rows that can never carry a test (a not-owed claim whose evidence fails) and optional rows Thomas marked not ever | "Which row lands where" |
| The Evidence cell is documented to sum to the Proof cell's numerator, and the update report and a functional test read the same count | The numerator section and the census |
| The overview bar's segments were never designed | "The two bars" |
| Rollup lines print the sections' text, and a rollup holds no per-RFC figures, so the intent counts would vanish from them | "The intent in private text" |
| A not-built count larger than the unproven rows would pass the add-back check if proof pending were a remainder | Both sections are counted row by row, never derived by subtraction |

## Owner decisions during the page review, 2026-11-13 and 2026-11-14

Thomas reviewed the rendered pages from the working tree before phase 5. His
decisions changed the design below, and the sections after this one are
corrected to them. The acceptance criteria now name the four open sections
directly, so the note above about "the two sections" is history only.

| Question | Decision |
|----------|----------|
| An open row that no tagged test covers | A fourth open section, "Not assessed" (`not-assessed`), at the MUST and the optional level. Thomas: "Keep the number real." A row lands there when no non-gap tag covers it, so "Proof pending" never claims a test that does not exist |
| The label of the fourth section | Thomas dictated "Not Asserted". The pages were built as "Not assessed", and he confirmed "Not assessed" on 2026-11-14 |
| Order of the open sections | Proof pending, OS testing gap, Exclusion unchecked, Not assessed, Not built, on both bars, the tree text, the glossary and the profile tables |
| Card groups on the compliance pages | Positive, Negative, Neutral, Totals, in that order |
| The per-RFC total card | Last, under Totals. The 2026-09-01 decision not to lead with it stands. It is outlined with no coloured edge, labelled "Mandatory requirements (MUST level)", and its sibling denominator reads "of N mandatory requirements" |
| A colour for Neutral | Yellow rejected: amber is the warn tone, and a neutral number takes no colour |
| Card layout | Two cards per line |
| The card "No test at all" | Relabelled "Declared gap or no test". The at-a-glance row beside it reads "Mandatory requirements with no test and no annotation", because the two count different rows |
| The pages | Approved by Thomas on 2026-11-14 |

## Which row lands where

Every owed row that is not proven, not excluded and not performed by a lower
layer lands in exactly one section. The rule is the same at the MUST level and
the optional level, and it is applied in the row pass (`figuresByStem`), one
row at a time.

| # | Row | Section | Why |
|---|-----|---------|-----|
| 1 | Annotated `{gap}` or `{partial}` | Not built | The summary says the obligation is unmet. A partial row is built in part; the glossary sentence says the section counts rows not fully built |
| 2 | Unannotated, with an audit verdict of wrong, unimplemented or partial, and not proven | Not built | The audit says the obligation is unmet (A-6) |
| 3 | `{linux-limit}` whose register entry fails its check | Not built | Its trait says unmet, and without a verified limit entry the claim that Linux blocks it is unproven. None exists on 2026-11-13 |
| 4 | Optional row marked `{intent: not-ever}` | Not built | Thomas declined it, so its proof is not pending. The page says only that it is not built |
| 5 | Unannotated or `{single-polarity}`, not proven, that a non-gap tag covers: a tag with no record, a stale record, a `no-break` escape | Proof pending, at both levels | A test exists and no recorded break proves it yet. The tag test is `figureEvidence.tested`, the same at the MUST and the optional level |
| 6 | `{not-applicable}` or `{feature-declined}` whose evidence fails its check | Exclusion unchecked | Thomas's decision, 2026-11-13. The kind bars tests, so no proof can ever be recorded for it while it keeps the annotation. `rfc/exclusion-evidence-backlog.txt` counts 30 `not-applicable` rows in this state across the corpus |
| 7 | Unannotated with no tagged test at all | Not assessed, at both levels | Thomas's decision during the page review ("Keep the number real."): nothing yet says whether Ze meets the row, so it is neither proof pending nor not built. At the MUST level such a row is also a red of `./le rfc check` (`evaluate`, `internal/le/rfc/check_core.go`), and a red does not hold a commit. The count over the 196 implemented RFCs is 0. Across all walked documents it is 191, and 16 of those are rows of RFC 9582, which the route-server profile requires (Known Limitations) |

The rule is one function, `openSectionOf` in `internal/le/rfc/profilecoverage.go`,
asked in that order: not built (rows 1 to 4), exclusion unchecked (row 6),
proof pending (row 5), and not assessed for every other open row (row 7).

Row 6 is a third section, "Exclusion unchecked", by Thomas's decision of
2026-11-13. A row whose "not owed" claim has no checked evidence is neither
not built nor awaiting a proof. The section is counted at both levels, drawn
on the MUST bar and the overview bar, and defined in the glossary as a row
claimed not to bind Ze whose evidence is still to be checked. It stays inside
Kept, as such a row does today, and the register reads complete only when it
is empty. The alternative, counting it as not built, was set aside because it
overstates what is missing by up to 30 rows. "Not assessed" (row 7) is the
fourth open section, added during the page review, and the register needs it
empty too.

## The numerator

"Counts as done in every published figure" binds every place a proven count is
published, and they must agree with each other.

| Published figure | Today | After |
|------------------|-------|-------|
| Home card count and percentage (`rfc.proven`, `rfc.proven_percent`) | Ze proven over kept | Ze proven plus OS verified, over kept |
| Health proof density, its label, its detail sentence and its recorded series | Ze proven over kept | The same sum; the label and the sentence "never in ze proven" are rewritten |
| Tree row `mandatory-proven`, read by the status page Proof cell, the site's per-RFC Proof cell, the update report and `test/ui/le-rfc-derived-proof-render.ci` | Ze proven of all MUST rows | Ze proven plus OS verified, so every reader of the published count agrees without a change of its own |
| Evidence cell and `proven-by-kind` | Proven rows by strongest test kind, summing to the Proof numerator | Gains one kind, `os`, for the OS verified rows, so it still sums to the Proof numerator |
| `Sections.Must.ZeProven` and the bar's Ze proven segment | Ze proven | Unchanged: the bar keeps showing who performs a row |
| Profile rollup in `data/rfc-requirements.json` and `./le rfc tree` or `./le rfc next` JSON (`Rollup`, `ScopeRollup`) | `mandatory.proven`, Ze proven only | A flat `mandatory-proven` that counts OS verified rows, as the tree row does (`internal/le/rfc/rollup_published.go`). Reading it back takes the OS verified rows out again, and a published count smaller than its OS verified rows is refused. In process, `Rollup.Mandatory.Proven` stays Ze only |

The health floor in `test/health/quality-baseline.json` reads 66.2 for proof
density while the published figure is 45.2. What that floor measures against
was not established in this design, and the implementation reads the floor's
producer before touching the density (R-3). Answered in phase 2: R-3 in Risks.

## The two bars

| Bar | Segments, left to right |
|-----|-------------------------|
| MUST | Ze proven, OS verified, Proof pending, OS testing gap, Exclusion unchecked, Not assessed, Not built |
| Overview | Ze MUST proven, OS verified (MUST and optional), SHOULD proven, Not owed, Proof pending (MUST and optional), OS testing gap (MUST and optional), Exclusion unchecked (MUST and optional), Not assessed (MUST and optional), Not built (MUST and optional) |

This is the order as built, declared once in `rfcBars`
(`internal/le/site/rfcdata.go`). The overview bar sums the two levels in one
segment for proof pending, for exclusion unchecked, for not assessed and for
not built, as it already does for the OS segments. The three intent segments
and their style rules are deleted.

## The intent in private text

The intent counts stay private planning facts. A per-RFC row of `./le rfc
tree` prints them after its sections, from the figures it was computed from. A
rollup carries a private sum of them, never marshalled, so a rollup line of
the tree and of `./le rfc next` prints them too. A row read back from
published JSON holds no intent, and prints no intent clause rather than a
false zero.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md`, "Coverage figures" - the model of the two figures and the one split
  → Constraint: `SectionsOf` is the one split every surface reads, and it refuses figures whose sections do not add back to their totals. The new sections join that add-back check: Kept is the sum of every MUST section, Kept plus not owed is Total.
  → Constraint: a `{gap}` or `{partial}` row stays in the denominator and is never proven, whatever records its tags carry. This spec changes which section draws it, never whether it counts.
  → Constraint: "An enrolled RFC whose feature is still to come counts at its real figure (owner, 2026-10-30)." The population `Implements` selects is unchanged.
  → Constraint: "No ceiling, lowest ceiling or proven percentage is printed by the tree (owner information-preservation approval, 2026-10-31)." The tree keeps printing counts only.
  → Decision: the page states "a row the OS performs stays in the denominator and never in Ze proven, verified or not". The ruling of 2026-11-13 replaces the published-percentage half of that sentence: an OS verified row is in the numerator of every published proven figure. The MUST bar still draws Ze proven and OS verified as separate segments.
- [ ] `docs/contributing/rfc-conformance-gates.md`, "OS verified" - when a lower-layer row is verified
  → Constraint: OS verified needs a verifying, proving record that broke the producer the annotation names. A tag alone, a stale record and the `no-break` or `kernel-producer` escape leave an OS testing gap. This spec does not touch that predicate, so an escaped row never reaches the numerator.
- [ ] `docs/contributing/rfc-conformance-gates.md`, "The intent marker" - the three states of an open optional row
  → Constraint: the marker is legal only on optional rows, `not-ever` is set by Thomas only, and `./le rfc check` judges its quote. All of that stays: the marker remains an authored, gated summary fact.
  → Decision: the three counts (`NotYet`, `NotEver`, `NotDecided`) stay on `StemFigures` for private tooling and leave every published shape. The published shape splits the same open optional rows by the unmet predicate instead, as the MUST rows are split.
- [ ] `docs/architecture/site-facts.md`, "RFC completion data" - the published files and the marked elements
  → Constraint: every page value is a `data-ze-rfc` mark filled from `data/rfc/`, and the fill refuses a marker naming a field no file holds. A renamed field must change in the data writer, the bar declaration, the fallback fill and the browser script together.
  → Constraint: `register` reads `complete` only when no member is unwalked and the MUST gap sections are zero. With two Ze sections it needs both zero, and the OS testing gap zero.
- [ ] `docs/contributing/gh-pages.md`, "Quality pages" - which pages draw the bars and tables
  → Constraint: "every percentage left the index with the owner's information-preservation approval of 2026-10-31". The only published percentages are the home card and the health proof density.
- [ ] `ai/META-DECISIONS.md`, "Goal and scope" and "Website design"
  → Constraint: a feature Thomas left out of current work stays a gap owed later, and "no page calls them out of scope". The new section is named for the present state (not built), never for a plan.
  → Constraint: every website value that changes over time lives in one JSON file and pages render it. The split is a data field, never page text.
  → Constraint: website work is tested once, at closure, after Thomas approved the pages.
- [ ] `plan/post-release/engineering/spec-site-rfc-recorded-data.md` - the spec that lands after this one
  → Constraint: it will introduce a git-ignored recording under `website/recorded/` that embeds the section-shaped data under a schema. It is in design with nothing built, so this spec owes it no schema step: the recording's first schema is the split shape.
  → Constraint: it keeps the published files' format and bytes, so it moves when each published value is computed, not what it holds. This spec changes what they hold, and must land before that spec's goldens are cut.

**Key insights:**
- The mandatory gap count is one subtraction, attainable less proven, in `sectionsOf`. The row-level pass `figuresByStem` already holds, for each such row, whether the summary or the audit calls it unmet (`figureEvidence.unmet`), so the split needs one new count taken in that pass and no new predicate.
- The intent counts are published today through data, not only bar labels: the per-RFC files, the per-profile files and the requirement ledger (which embeds the tree row type) each carry `not-ever`, `not-yet` and `not-decided`.
- No summary carries an `{intent}` marker on 2026-11-13, so every open optional row is `not decided` and collapsing the three public counts loses no published information.
- Three public proven figures exist with different denominators: the home card and health proof density (proven over kept), the status page Proof cell (proven of all MUST rows), and the compliance buckets (every gated row, walked or not). The second ruling binds the first two; the buckets count polarity coverage, not proof.
- OS verified is 0 across the corpus on the 2026-11-10 data, so counting it moves no number at landing. It removes a ceiling: 239 lower-layer rows could never reach the numerator.
- The site derives almost everything from the bar declaration: the profile figure table, the list's Total columns and the glossary each loop over it. Three places still name a segment themselves: the SHOULD rows of the profile figure table, the register in the fallback fill, and the register in the browser script.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/sectioncounts.go` - `MustSections` (total, kept, ze-proven, os-verified, os-gap, ze-gap, not-owed), `ShouldSections` (proven, os-verified, os-gap, not-owed, not-ever, not-yet, not-decided), `SectionsOf` with its add-back refusals, `Sections.String` printing "Ze correctness gap" and the three intents
- [ ] `internal/le/rfc/profilecoverage.go` - `figuresByStem` counts each owed row once; `figureEvidence.unmet` (annotation trait Unmet, or an audit verdict of wrong, unimplemented or partial); `countIntent`; `countOS`; `rowCounts`
- [ ] `internal/le/rfc/implemented.go` - `ImplementedSectionsOf` sums walked implemented RFCs; `ProvenPercent` is Ze proven over kept
- [ ] `internal/le/rfc/render_ledger.go` - `proofCell` prints mandatory proven of mandatory total for the status page
- [ ] `internal/le/rfc/tree.go` - the tree row publishes its sections whole under `sections`; `sectionsCell` prints `Sections.String` as the row's text
- [ ] `internal/le/rfc/actions.go` - the tree action's help names "Ze correctness gap" and the three intent states
- [ ] `internal/le/site/rfcdata.go` - `rfcBars`, the one declaration of both bars' segments, labels and fields; `rfcStemData` embeds the model's section types
- [ ] `internal/le/site/rfcledger.go` - the published requirement ledger embeds the tree row type as each stem's figures
- [ ] `internal/le/site/rfcreader.go` - `rfcSegmentMeanings` holds one glossary sentence per segment, the three intents included, and a segment with no meaning panics
- [ ] `internal/le/site/rfcprofiles.go` - `rfcProfileFigures` loops the MUST bar's segments, then names the three intent rows itself; `rfcProfileFiguresProse`
- [ ] `internal/le/site/rfcfilter.go` - `rfcListTotalColumns` loops the MUST bar's segments; `rfcListCheckWords` says "the four bar sections"
- [ ] `internal/le/site/rfcdetail.go` - the detail page draws the same tier table as a profile
- [ ] `internal/le/site/rfccompliance.go` - `rfcEnrolledSectionsHTML` draws the enrolled bars and their Total row
- [ ] `internal/le/site/rfcfallback.go` - `rfcRegisterOf` names `must.ze-gap` and `must.os-gap`
- [ ] `internal/le/site/facts.go` - `factsFromRFCLedger` writes Ze proven, kept and `ProvenPercent` into the site facts
- [ ] `internal/le/site/homebody.go` - the home card prints the percentage and "proven of kept requirements for Ze or the OS"
- [ ] `internal/le/test/health/collect_rfc.go` - proof density divides Ze proven by kept itself, held to a floor
- [ ] `website/assets/js/rfc-figures.js` - `registerOf` names the two gap fields itself; every other field comes from the index

**Behavior to preserve:**
- The populations: which RFCs count (`Implements`), which rows are owed (`rowCounts`), which are proven (`DiscriminationVerdict.Proven`), which are not owed (`figureEvidence.excluded`), which are OS verified (`osVerified`).
- Kept and Total of every RFC and every rollup: no row enters or leaves the MUST bar.
- The intent marker's grammar, its gates, and what `./le rfc next` does with it.
- The unmet id list of each RFC, and the status page Unmet cell.
- No percentage on the tree, the RFC index, the profile pages or the detail pages.

**Behavior to change:**
- The mandatory "Ze correctness gap" section becomes four open sections (proof pending, exclusion unchecked, not assessed, not built), on every surface.
- Published proven counts and percentages count OS verified rows.
- Published shapes carry the open optional rows in the same four sections, in place of three intent counts.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The one section split | `internal/le/rfc/sectioncounts.go`, `SectionsOf` | Splits a walked RFC's figures into the sections both bars draw, and refuses a split that does not add back | Extends: the gap section becomes two sections inside the same split and the same add-back check. No surface gains a split of its own |
| The row-level unmet predicate | `internal/le/rfc/profilecoverage.go`, `figureEvidence.unmet` | Answers whether the summary or the audit says a row is not met; fills each RFC's unmet id list | Extends: the same predicate, asked of each unproven owed MUST row in the same pass, fills the new not-built count. A second definition of "not built" is not written |
| The annotation trait declaration | `internal/le/rfc/rfc.go`, `annotationKinds`, `AnnotationTraitsOf` | Declares once what each annotation kind means to every reader | Does not apply a change: the Unmet trait already says what the split needs, so no kind and no trait is added |
| The published percentage | `internal/le/rfc/implemented.go`, `ImplementedSections.ProvenPercent` | Ze proven over kept, read by the home card | Extends: the numerator becomes Ze proven plus OS verified, in that one function, and health reads it instead of dividing for itself |
| The bar declaration | `internal/le/site/rfcdata.go`, `rfcBars` | Declares each bar's segments, order, words and fields once; published in the index for the browser | Extends: the segment list changes there. The browser's drawing, the list columns, the MUST rows of the figure table and the glossary already derive from it |
| The marked-element fill | `internal/le/site/rfcfallback.go`, `rfcFigureSet.fillHTML`, `fillMirror` | Fills every `data-ze-rfc` mark from the data files and refuses a mark with no answer | Extends by use: renamed fields are refused wherever a page still names the old ones, which finds every stale mark |
| The intent marker | `internal/le/rfc/intent.go`, `parseIntent`, `evaluateIntent`; `countIntent` in `internal/le/rfc/profilecoverage.go` | Records and gates a planning intent per optional row, and counts open rows by it | Does not apply a change to the marker. Replaces its publication: the three counts leave the section types the site publishes |
| The recorded RFC data | `plan/post-release/engineering/spec-site-rfc-recorded-data.md` (not yet built) | Will record the site's RFC data in one git-ignored file with a schema | Does not apply: nothing of it exists yet. This spec lands first and that recording takes the split shape as its first schema |
| The le help functional tests | `test/ui/le-help-ai.ci` and its registered fixture | Build the `le` personality and read its help through the real binary | Extends: one more test of the same shape reads the tree action's help for the section names |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `rfc/short/<stem>.md` requirement rows with their annotations and markers, the `RFC requirement:` tags in the tests, the discrimination records under `rfc/discrimination/`, the audit verdicts under `rfc/audit/`, and the extraction sign-offs.
- Read by `./le rfc tree`, `./le rfc next`, `./le rfc update`, `./le site build` and `./le test health`.

### Transformation Path
1. `Collect` renders the corpus into one `RenderInput`.
2. `figuresByStem` counts each owed row into its figure, and in the same pass takes the OS, intent and unmet facts. The not-built count is taken here.
3. `SectionsOf` splits each walked RFC's figures into sections and checks they add back.
4. Rollups sum sections over a profile tier, an aggregate, or the implemented RFCs.
5. The site writes the sections into the data files and the bar declaration into the index; the fill and the browser script draw them. The tree, the status page and health print them as text.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Model to site data files | The section types marshalled as JSON into `data/rfc/` and the requirement ledger | No |
| Site data files to browser | `rfc-figures.js` reads the index and the per-RFC files | No |
| Site build to checked-out site | `./le site build` writes the data files and pages into the `../gh-pages` checkout | No |

### Changed-Wire Census

**Search evidence:** searched `internal/`, `website/`, `docs/`, `ai/`, `cmd/`, `test/`, `.github/` and scripts for `ZeGap`, `ze-gap`, `ze_gap`, "Ze correctness gap", `OSVerified`, `os-verified`, `NotEver`, `NotYet`, `NotDecided`, `not-ever`, `not-yet`, `not-decided`, `ProvenPercent`, `proven_percent`, `must_kept`, and the `sections` key. Positive controls: the writer `sectionsOf` and the reader `registerOf` in `website/assets/js/rfc-figures.js` were both found. No hit in `.github/`, `cmd/`, `ai/skills/`, `ai/RFC-REQUIREMENTS.md`, `rfc/requirements/*.md` or website page prose. One hit in `website/data/frontend-vocab.json` was not located and is checked in the wiring phase.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Section JSON | writer | `internal/le/rfc/sectioncounts.go`, `MustSections`, `ShouldSections` | Read: the struct tags are the published keys | Change: `ze-gap` is removed and replaced by two keys; the three intent keys are removed and replaced by the same two keys at the optional level |
| Section JSON | writer | `internal/le/rfc/tree.go`, `TreeRow.MarshalJSON` | Read: publishes `sections` whole | Change: follows the types; carries no intent count |
| Section JSON | writer | `internal/le/site/rfcdata.go`, `rfcStemDataOf`, `writeRFCData` | Read: embeds the model types | Change: follows the types |
| Section JSON | writer | `internal/le/site/rfcledger.go`, `publishRFCLedger` | Read: each stem's figures are the tree row type | Change: follows the types |
| Section JSON | reader | `internal/le/rfc/tree.go`, `TreeRow.UnmarshalJSON` | Read: holds the sections as published | Change: follows the types |
| Section JSON | reader | `internal/le/site/rfcfallback.go`, `rfcRegisterOf`, `fillHTML`, `fillMirror` | Read: `rfcRegisterOf` names the two gap fields | Change: `register` reads the new fields |
| Section JSON | reader | `internal/le/site/rfcprofiles.go`, `rfcProfileFigures` | Read: names the three intent fields itself | Change: two rows in their place, not built and proof pending |
| Section JSON | reader | `website/assets/js/rfc-figures.js`, `registerOf`, `sumStems` | Read: `registerOf` names `ze-gap` and `os-gap` | Change: reads the new fields |
| Section JSON | reader | outside the repository | Thomas, design gate 2026-11-13: nothing outside reads these fields | Preserve: no compatibility copy is owed |
| Bar declaration | writer | `internal/le/site/rfcdata.go`, `rfcBars` | Read | Change: segments replaced |
| Bar declaration | reader | `website/assets/js/rfc-figures.js`, `drawBar` | Census: draws what the index says | Preserve: holds no segment list of its own |
| Bar declaration | reader | `website/assets/css/30-components.css`, the `.rfc-seg-*` rules | Census: one rule per segment name | Change: rules for the removed names are deleted, rules for the new names added |
| Bar declaration | reader | `internal/le/site/rfcreader.go`, `rfcSegmentMeanings` | Read: panics on a segment with no meaning | Change: meanings for the new segments, the removed ones deleted |
| Bar declaration | reader | `internal/le/site/rfcfilter.go`, `rfcListTotalColumns`, `rfcListCheckWords` | Read: loops the MUST segments; the check words count them as four | Change: the columns follow the declaration; the count in words is corrected |
| Site facts | writer | `internal/le/site/facts.go`, `factsFromRFCLedger` | Read: writes Ze proven, kept and the percentage | Change: proven counts OS verified |
| Site facts | reader | `internal/le/site/homebody.go`, the home card | Read: "requirements for Ze or the OS" | Preserve: the sentence already names both, and the count beside it now matches it |
| Proven percentage | writer | `internal/le/rfc/implemented.go`, `ProvenPercent` | Read | Change: numerator gains OS verified |
| Proven percentage | reader | `internal/le/test/health/collect_rfc.go`, `collectRFC`, `densityMetric` | Read: divides Ze proven by kept itself | Change: reads the shared numerator; detail prose names the new sections |
| Proven count | writer | `internal/le/rfc/profilecoverage.go`, `figuresByStem` filling the mandatory figure; `internal/le/rfc/tree.go`, `TreeRow.MarshalJSON` publishing `mandatory-proven` | Read | Change: the published count is Ze proven plus OS verified; the evidence kinds gain `os` |
| Proven count | reader | `internal/le/rfc/render_ledger.go`, `proofCell`, `evidenceCell`, `statusPageIntro` | Read: the intro says the Evidence counts "sum to N" | Change: the intro names OS verified rows and the `os` kind; the cells follow the count |
| Proven count | reader | `internal/le/site/rfcdetail.go`, the per-RFC Proof cell from a tree row read back from JSON | Read: `entry.Figures.ProofCell()` over a row `TreeRow.UnmarshalJSON` rebuilt | Preserve: it reads the published count, which now holds the sum, so it needs no edit and agrees with the status page |
| Proven count | reader | `internal/le/rfc/weekly_source.go`, `mustFigure`, `weeklyTreeRowJSON` | Read: sums `mandatory-proven` over `mandatory-total` for the update report | Preserve: reads the published count; the update report's figure gains OS verified rows with no edit |
| Proven count | reader | `internal/test/fixture/ui_fixture_le_rfc_derived_proof.go`, the fixture of `test/ui/le-rfc-derived-proof-render.ci` | Read: holds the Proof cell equal to the tree's proven of total | Preserve: both sides read the same published count |
| Proven count | reader | `internal/le/site/facts.go`, the `rfc.proven` source sentence | Read: "requirements proven by Ze's tests" | Change: the sentence names the OS verified rows |
| Section text | writer | `internal/le/rfc/sectioncounts.go`, `Sections.String`; `internal/le/rfc/actions.go`, the tree action's help | Read | Change: names the two sections; the tree's text keeps the three intent counts, taken from the figures |
| Section text | reader | a person or agent reading `./le rfc tree` output or its help; no program parses it | Census: no parser found for the text form | Preserve: text only |

### Integration Points
- `figuresByStem` - takes the new count beside the unmet list it already fills.
- `SectionsOf` - the add-back check covers the new sections.
- `rfcBars` - the one place the segments are named for the site.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every MUST row in proof pending carries a tagged test | `evaluate` in `internal/le/rfc/check_core.go` reports an enrolled gated row with no test and no annotation as a red; the published buckets of 2026-11-10 count 0 such rows. A red holds no commit, so the gate does not make the count zero by construction | A few proof-pending rows have no test | A corpus test reports the count and fails above zero, naming each row; the fix is a tag or an annotation on that row, never a change to the section's words | superseded: once "Not assessed" exists, an untagged open row lands there and never in proof pending, so proof pending holds tagged rows by construction and the assumption can no longer fail. The question it guarded moved to the not assessed section: MUST not assessed is 0 over the 196 implemented RFCs (`TestNotAssessedMustRowsAreNamed` names each such row). `TestEveryProofPendingRowCarriesATaggedTest` stays, because the judge refused its deletion |
| A-2 | No summary carries an `{intent}` marker | `grep '{intent' rfc/short` answers 0 files on 2026-11-13 | Collapsing the public counts would remove published information, which needs the owner's information-preservation approval | The same grep at implementation | confirmed: 0 markers at implementation |
| A-3 | `spec-site-rfc-recorded-data` has built nothing when this spec's site phase starts | That spec's status on 2026-11-12: design, three open questions | A recording exists in the old shape, and this spec owes a regeneration step and a schema value | Check that spec's status and the tree for its recording file before the site phase | confirmed: that spec is still in design with nothing built, so its recording takes the four-section shape as its first schema |
| A-4 | Nothing outside this repository reads the field names of `data/rfc/` | Thomas, design gate 2026-11-13: "Nothing outside" | An outside reader breaks on the renamed fields | Confirmed by Thomas | confirmed by Thomas |
| A-7 | Most open optional rows carry neither an unmet mark nor a tagged test, so the optional "proof pending" count will hold rows with no test behind them | No gate obliges a test on an optional row; RFC 4271 publishes 31 open optional rows and 3 proven | The count is smaller than feared and the name is accurate | Count the open optional rows with no tag over the live corpus at implementation, and state that count in the glossary sentence's wording | confirmed, and worse than assumed: 1,811 of the open optional rows over the implemented RFCs had no test. That count is why Thomas added "Not assessed", so proof pending no longer holds them |
| A-5 | OS verified is 0 across the corpus, so the percentage does not move when this lands | Site data of 2026-11-10 | The home card and the health floor move at landing; the floor file needs its new value recorded | Read the count from the tree at implementation | confirmed: OS verified is 0 at implementation, so no published percentage moved and the floor file was not edited |
| A-6 | An unproven row whose only unmet mark is an audit verdict (wrong, unimplemented, partial) belongs under "not built" | The existing predicate treats summary and audit alike for the unmet list | Rows that are built but defective are named "not built" | Count them at implementation; above a handful, raise the name with Thomas | built as assumed: such a row lands in not built (`openSectionOf` over `figureEvidence.unmet`). The count of rows whose only unmet mark is an audit verdict was not measured, so it was not put to Thomas. It is owed at closure |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | "Not built" is drawn from `{gap}` marks, and some marks sit on code that exists (`spec-fttx-edge-unproven-row-review` found such rows) | A row in the not-built section with a passing tagged test | Out of scope by Thomas's decision; the section's glossary sentence says it counts what the summary declares, and the row review specs correct the marks |
| R-2 | The optional "proof pending" section overstates rows that have no test at all (A-7). Thomas chose the split knowing this | A reader takes an untested optional row for a tested one | The glossary sentence for the optional section says it counts every open optional row the summary does not declare unmet, tested or not. If A-7's count is large, the result goes back to Thomas with the number before the pages are approved |
| R-3 | The health proof density floor ratchets on a number whose definition changes, and the floor file reads 66.2 against a published 45.2 | The health run reports the density against its floor | Read the floor's producer first and establish what 66.2 is measured against. The change to the density is zero today (A-5); the floor's meaning is restated in `docs/architecture/testing/test-health.md` in the same change. Answered in phase 2: 66.2 is the best value the floor ever held under the definition used before 2026-10-31, which counted every MUST row. The metric already reads warn against it. The baseline file was not edited, and restating the floor is Thomas's call |
| R-4 | Goldens and browser assertions pin the old segment words, so every one changes | The site tests go red on label text | Each changed expectation goes to the `ze-test-judge` route with the ruling as the source that makes the old expectation wrong |
| R-5 | Another session holds uncommitted edits in `internal/le/site/` (`actions.go`, `build.go`, `producer.go` on 2026-11-13) | A foreign hunk in a file this spec edits | Commit this spec's hunks alone per `ai/rules/git-safety.md`; do not start the site phase while that session's work is in flight on the same files |
| R-6 | `./le rfc next` re-derives which rows owe Ze code per row (`rowOwes`), so its totals can disagree with the new sections | The next report's counts differ from the tree's | A test holds the next report's Ze-owed MUST count equal to the MUST sections that are not proven and not OS, for one corpus. The equality is MUST only: `rowOwes` skips an optional not-ever row, which the sections count as not built |
| R-8 | The intent marker stays readable in public: the repository is public, the ledger links each summary file, and a requirement marshalled by `le` carries its intent | A reader finds a not-ever marker in a summary | Accepted: the ruling is about what the pages and their data state, and the summary is the private planning record kept in a public repository. Named in Known Limitations |
| R-7 | A published file keeps an intent key through a type this spec did not reach | The guard test of AC-7 goes red | The guard reads every file the build publishes, not a list of known files. As built, the guard in `TestDataIndexAndFilesAgree` reads one published per-RFC file (Known Limitations) |

Risks met during implementation:

| ID | Risk met | What happened | Outcome |
|----|----------|---------------|---------|
| R-9 | Other sessions' uncommitted edits break the shared build | Four times another session's mid-edit stopped the build in the main checkout, once during the page review (a DDoS plugin file with an unused import and undefined names) | Each time the work waited for that session, and no copy of the tree was used |
| R-10 | The weakening guard refuses its own author's repair | The guard and its approval tool refused the rename of the helper `intentRow` to `splitIntentRow` in the untracked new file `sections_split_test.go` (the false positive of `plan/journal/guard-blocks-its-own-authors-repair.md`) | Thomas ran the rename by hand. The guard is not fixed |
| R-3 answered | The health floor's definition | See R-3 above | No floor change; Thomas decides whether to restate it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing in the shipped binary. A wrong split misstates the public RFC pages; a missed reader leaves a page refusing to build, which the fill reports by name |
| How is it reverted? | One revert of the model commit and one of the site commit, then `./le site build` |
| Who else touches this path? | `spec-site-rfc-recorded-data` (same data), `spec-rfc-optional-feature-ratio` (depends on the SHOULD states), the row review specs for fttx-edge, and a live session editing `internal/le/site/` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le help rfc tree` through the built binary | → | the tree action's declared help | `test/ui/le-help-rfc-tree-sections.ci` |
| `./le rfc tree` | → | `SectionsOf` with the four open sections | `TestTreeActionRendersLevelsAndFigures` |
| `./le site build`, RFC data step | → | `rfcBars`, `writeRFCData` | `TestDataIndexAndFilesAgree` |
| A reader's browser on a profile page | → | `rfc-figures.js` drawing the declared segments | `TestTheBrowserDrawsRFCFiguresFromTheirFiles` |
| `./le site build`, home card | → | `factsFromRFCLedger`, `ProvenPercent` | `TestTheHomeCardCountsOSVerifiedRows` |
| `./le test health` | → | `collectRFC` proof density | `TestProofDensityCountsOSVerifiedRows` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A walked RFC with unproven owed MUST rows, some declared unmet by the summary or the audit, some claimed not owed with failing evidence, some covered by a tagged test and some covered by none | Its four open sections hold, row by row: not built the declared unmet rows, exclusion unchecked the failing not-owed claims, proof pending the tagged rest, not assessed the untagged rest; the four with Ze proven, OS verified and OS testing gap sum to Kept | Kept, Total and not owed are unchanged for every RFC: the split moves no row across the bar's edge |
| AC-2 | A row the audit calls wrong that a verifying record proves, and a `{linux-limit}` row whose evidence passes | The first counts as Ze proven and the second as not owed; neither is in the not-built count, and both stay in the unmet id list | The unmet id list keeps every id it lists today, because it is the summary's disclosure and not a section |
| AC-3 | Any surface that draws or prints the MUST sections: the tree and its help, the per-RFC and profile data files, the requirement ledger, the bars, the list columns, the glossary, the health detail | Each names the four open sections in the same words and order (the open part reads proof pending, OS testing gap, exclusion unchecked, not assessed, not built), taken from the one split and the one bar declaration | The OS segments keep their names and order relative to Ze proven |
| AC-4 | A corpus with at least one OS verified MUST row | The home card count and percentage, the health proof density, the tree's published proven count, the status page Proof cell, the site's per-RFC Proof cell and the update report's figure each count that row as proven, and the Evidence cell lists it under an `os` kind so its counts still sum to the Proof numerator | The MUST bar still draws OS verified as its own segment, because who performs a row stays visible |
| AC-5 | A row carrying a `no-break` or `kernel-producer` escape, or a lower-layer row with a tag and no record | It is counted in no proven figure, and stays an OS testing gap or proof pending | No escaped row is permitted in a numerator: the owner rule publishes it as escaped, never as proven |
| AC-6 | A set of RFCs for the `register` value | It reads complete only when no member is unwalked and proof pending, OS testing gap, exclusion unchecked, not assessed and not built are all 0, and a sum that lacks one of those fields reads not complete | A set with no member still reads "no rows" |
| AC-7 | Every file `./le site build` publishes, and every page it renders | Open optional rows appear as proof pending, exclusion unchecked, not assessed and not built counts, and each published file and page holds those four where the three intent counts were | The summaries keep their markers, and `./le rfc tree` text and `./le rfc next` keep reading them, because the marker is a private planning fact |
| AC-8 | A summary with `{intent: not-ever}` and `{intent: not-yet}` rows | `./le rfc tree` prints the three intent counts in each row's text and in each rollup line, `./le rfc next` still skips a not-ever row, and the not-ever row is counted as not built | The tree's JSON carries the four open optional sections and no intent count, because the site publishes that type; a row read back from that JSON prints no intent clause |
| AC-9 | A walked RFC with open optional rows, some declared unmet by the summary or the audit or marked not ever, some tagged and some untagged | Its optional sections split those rows by the same rule as AC-1 into not built, exclusion unchecked, proof pending and not assessed, and with proven, OS verified, OS testing gap and not owed they sum to the optional Total | The optional Total and every other optional section are unchanged, because the split only renames how the open rows are divided |
| AC-10 | The docs that describe the figures | `rfc-conformance-gates.md`, `site-facts.md`, `gh-pages.md` and `testing/test-health.md` describe the four open sections, the numerator (the profile rollup included) and the private marker, with source anchors | Sections of those pages this spec does not reach stay as they are |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMustGapSplitsIntoNotBuiltAndProofPending` | `internal/le/rfc/sections_split_test.go` | AC-1 | pass (rfc package, whole) |
| `TestNotBuiltCountsOnlyUnprovenOwedRows` | `internal/le/rfc/sections_split_test.go` | AC-2 | pass (rfc package, whole) |
| `TestSectionsOfRefusesFiguresThatDoNotAddBack` | `internal/le/rfc/sectioncounts_test.go` | AC-1 add-back over the new sections (existing, extended) | pass (rfc package, whole) |
| `TestProvenPercentCountsOSVerifiedRows` | `internal/le/rfc/implemented_test.go` | AC-4, AC-5 | pass (rfc package, whole) |
| `TestProofCellCountsOSVerifiedRows` | `internal/le/rfc/render_ledger_test.go` | AC-4 | pass (rfc package, whole) |
| `TestEvidenceKindsSumToTheProofNumerator` | `internal/le/rfc/render_ledger_test.go` | AC-4, with an OS verified row | pass (rfc package, whole) |
| `TestAProofCellReadBackFromJSONCountsOSVerifiedRows` | `internal/le/rfc/tree_test.go` | AC-4 through the published row the site reads | pass (rfc package, whole) |
| `TestAPublishedRowWhoseProvenCountLacksItsOSRowsIsRefused` | `internal/le/rfc/tree_test.go` | AC-4: a published row whose proven count is smaller than its OS verified rows is refused on read-back | pass (rfc package, whole) |
| `TestANotOwedClaimWithoutEvidenceIsExclusionUnchecked` | `internal/le/rfc/sections_split_test.go` | "Which row lands where", row 6: the row is in exclusion unchecked at its level, in no other open section, and inside Kept | pass (rfc package, whole) |
| `TestAnOpenRowWithNoTestIsNotAssessed` | `internal/le/rfc/sections_split_test.go` | "Which row lands where", row 7, at both levels | pass (rfc package, whole) |
| `TestANotEverRowIsNotBuilt` | `internal/le/rfc/sections_split_test.go` | "Which row lands where", row 4 | pass (rfc package, whole) |
| `TestRollupLinesKeepTheIntentCounts` | `internal/le/rfc/sections_split_test.go` | AC-8 | pass (rfc package, whole) |
| `TestRFCFigureSetAnswersAsTheScriptDoes` | `internal/le/site/rfcprofiles_figures_test.go` | AC-6 in the fallback fill: each of the four open MUST sections alone keeps the register open (existing, extended; it took the place of the planned `TestTheFallbackRegisterNeedsEveryOpenMustSectionEmpty`) | pass (site package) |
| `TestTheBrowserDrawsAProfilePageFromItsFiles` | `internal/le/site/browsertest/rfcprofilepage_test.go` | AC-6 in the browser script's register (existing, extended) | pass (browsertest package, whole) |
| `TestRegisterCompleteNeedsNoOpenMustSection` | `internal/le/rfc/sections_split_test.go` | AC-6 over the five open MUST sections | pass (rfc package, whole) |
| `TestRegisterCompleteNeedsNoZeOrOSGap` | `internal/le/rfc/profilecoverage_test.go` | AC-6, the earlier test, kept under its name beside the new one and updated to the new fields | pass (rfc package, whole) |
| `TestOpenOptionalRowsSplitIntoNotBuiltAndProofPending` | `internal/le/rfc/sections_split_test.go` | AC-7, AC-9 | pass (rfc package, whole) |
| `TestOpenOptionalRowsWithNoTestAreCounted` | `internal/le/rfc/sections_split_corpus_test.go` | A-7 over the live corpus: tagged open optional rows are proof pending, untagged ones not assessed | pass (rfc package, whole) |
| `TestNotAssessedMustRowsAreNamed` | `internal/le/rfc/sections_split_corpus_test.go` | Row 7 over the live corpus: names every MUST row in not assessed | pass (rfc package, whole) |
| `TestEveryProofPendingRowCarriesATaggedTest` | `internal/le/rfc/sections_split_corpus_test.go` | A-1 over the live corpus. It now holds by construction; kept because the judge refused its deletion | pass (rfc package, whole) |
| `TestTreeJSONCarriesNoIntentCounts` | `internal/le/rfc/sections_split_test.go` | AC-8 | pass (rfc package, whole) |
| `TestTreeTextKeepsTheIntentCounts` | `internal/le/rfc/sections_split_test.go` | AC-8 | pass (rfc package, whole) |
| `TestNextOwesWhatTheMustSectionsCount` | `internal/le/rfc/sections_split_test.go` | R-6, MUST level only | pass (rfc package, whole) |
| `TestAPublishedRollupCountsOSVerifiedRows` | `internal/le/rfc/rollup_published_test.go` | The profile rollup row of "The numerator" | pass (rfc package, whole) |
| `TestAPublishedRollupWhoseProvenCountLacksItsOSRowsIsRefused` | `internal/le/rfc/rollup_published_test.go` | The rollup read-back refusal | pass (rfc package, whole) |
| `TestAPublishedScopeRollupKeepsItsOwnFields` | `internal/le/rfc/rollup_published_test.go` | A scope rollup's own fields survive its custom JSON | pass (rfc package, whole) |
| `TestTheBarLayoutKeepsTheAgreedOrder` | `internal/le/site/rfcdata_test.go` | AC-3: both bars' built order (existing, extended; it took the place of the planned `TestTheBarLayoutSeparatesNotBuiltFromProofPending`) | pass (site package) |
| `TestDataIndexAndFilesAgree` | `internal/le/site/rfcdata_test.go` | AC-7 and AC-9: a published per-RFC file holds the four open sections at both levels equal to the model, and none of the three intent keys, matched exactly (it took the place of the planned `TestNoPublishedRFCDataNamesAnIntent`, over one file only: Known Limitations) | pass (site package) |
| `TestTheHomeCardCountsOSVerifiedRows` | `internal/le/site/facts_test.go` | AC-4 | pass (site package) |
| `TestProofDensityCountsOSVerifiedRows` | `internal/le/test/health/collect_rfc_test.go` | AC-4 | pass (health package, whole) |
| `TestLeTreeSectionWordsAreReadFromTheTreeText`, `TestLeTreeSectionWordsRefuseLinesThatDisagree`, `TestLeTreeHelpMustNameEveryWordInOrder` | `internal/test/fixture/ui_fixture_le_help_rfc_tree_sections_test.go` | The wiring fixture's parser and order check | no run recorded in the state file |

The site package passes except five tests that were red before this work and
that it does not reach: the lab ISO size, the homepage proof strip, the
homepage hero, a lost-fact refusal message, and the shared header.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| not built | 0 to the unproven owed MUST rows | equal to that count | N/A | above it: cannot occur, because each section is counted row by row and none is a remainder; the add-back check refuses sections that do not sum to Kept |
| OS verified in the numerator | 0 to the lower-layer MUST rows | equal to that count | N/A | N/A: bounded by `checkMustSections` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-help-rfc-tree-sections` | `test/ui/le-help-rfc-tree-sections.ci` | An agent reads `./le help rfc tree` and `./le rfc tree` through the built binary; the fixture holds every section word the help names equal to the words the tree's own text prints, so the help cannot name a section the tree does not | pass, through the le built from 67932475cc |
| `TestTheBrowserDrawsRFCFiguresFromTheirFiles` | `internal/le/site/browsertest/rfcfigures_test.go` | A reader opens a profile page and sees the four open sections, each with its count, for the MUST rows and for the optional rows | pass (browsertest package, whole) |
| `TestRFCSegmentColoursAreReadableInBothThemes` | `internal/le/site/browsertest/rfcfigures_test.go` | The new segments are readable in both themes | pass (browsertest package, whole) |
| `le-rfc-derived-proof-render` | `test/ui/le-rfc-derived-proof-render.ci` | The status page Proof cell equals the tree's proven of total, with OS verified rows counted | pass after 5b56fc88cf |

## Files to Modify

As built, matched to `git show --stat` of the three commits. Commit
092738d86e (wiring fixture) modified no existing file.

Feature code:
- `internal/le/rfc/profilecoverage.go` - the four open counts at both levels in `figuresByStem` (`openSectionOf`, `countOpen`, `figureEvidence.tested`), the private intent sum, `StemFigures.MustProven`, the `os` evidence kind, `Rollup.mandatoryComplete`, and the rollup's JSON tags removed (5b56fc88cf, 67932475cc)
- `internal/le/rfc/sectioncounts.go` - the section types, `sectionsOf`, `open`, `add`, the add-back checks, `String`, `MustSections.Proven` (5b56fc88cf, 67932475cc)
- `internal/le/rfc/implemented.go` - `ProvenPercent` (5b56fc88cf)
- `internal/le/rfc/render_ledger.go` - `proofCell`, `evidenceCell` with the `os` kind, `statusPageIntro` (5b56fc88cf)
- `internal/le/rfc/tree.go` - the published proven count and its read-back, the text cell's intent clause (5b56fc88cf, 67932475cc)
- `internal/le/rfc/actions.go` - the tree help text (67932475cc)
- `internal/le/rfc/next.go` - the comments on `rowOwes` and the owes constants; `rowOwes` calls `declinedForever` (67932475cc)
- `internal/le/rfc/check_evidence.go` - the comment on `notOwedGap` (67932475cc)
- `internal/le/rfc/priority.go` - `ScopeRollup` publishes through its own JSON (67932475cc)
- `internal/le/rfc/weekly_source.go` - the `rfc-proven` note says the count holds OS performed rows (67932475cc)
- `internal/le/site/rfcdata.go` - the segment names and `rfcBars` (5b56fc88cf, 67932475cc)
- `internal/le/site/rfcreader.go` - `rfcSegmentMeanings` (67932475cc)
- `internal/le/site/rfcprofiles.go` - the SHOULD rows of `rfcProfileFigures`, and `rfcProfileFiguresProse` (67932475cc)
- `internal/le/site/rfcfilter.go` - `rfcListCheckWords` (67932475cc)
- `internal/le/site/rfcfallback.go` - `rfcRegisterOpenSections`, `rfcRegisterOf` (67932475cc)
- `internal/le/site/rfcdetail.go` - prose that called an unchecked exclusion a "Ze gap", the `rfcOutOfScopeValue` comment, and the detail cards of the page review (67932475cc)
- `internal/le/site/rfccompliance.go` - the bucket meanings and scope note that called an unchecked exclusion a "Ze gap", and the card groups, the total card and the relabelled card of the page review (67932475cc)
- `internal/le/site/facts.go` - the proven count, its source sentence and comments (5b56fc88cf, 67932475cc)
- `internal/le/test/health/collect_rfc.go` - density, label and detail prose (5b56fc88cf, 67932475cc)
- `website/assets/js/rfc-figures.js` - `registerOpenSections`, `registerOf` (67932475cc)
- `website/assets/css/30-components.css` - the segment rules and the two-per-line cards (67932475cc)

Tests, fixtures and goldens:
- `internal/le/rfc/`: `featuredeclined_test.go`, `implemented_test.go`, `linux_limit_test.go`, `lowerlayer_test.go`, `profilecoverage_test.go`, `render_ledger_test.go`, `sectioncounts_test.go`, `tree_test.go`, `weekly_source_test.go`
- `internal/le/site/`: `facts_test.go`, `rfccompliance_test.go`, `rfcdata_test.go`, `rfcdetail_figures_test.go`, `rfcdetail_test.go`, `rfcindex_figures_test.go`, `rfcoutofscope_test.go`, `rfcprofiles_figures_test.go`, `rfcprofiles_test.go`
- `internal/le/site/browsertest/`: `rfcdetailpage_test.go`, `rfcfigures_test.go`, `rfclistpage_test.go`, `rfcprofilepage_test.go`
- `internal/le/test/health/collect_rfc_test.go`
- `internal/test/fixture/ui_fixture_le_rfc_derived_proof.go` (5b56fc88cf) and `internal/test/fixture/ui_fixture_le_help_rfc_tree_sections.go` (gains "Not assessed", 67932475cc)
- `internal/le/site/testdata/rfc-bars.json`, `internal/le/site/testdata/rfc-compliance-pages.json`, `internal/le/site/testdata/rfc-profile-page.json` - goldens, each with a `Test-judge:` line in 67932475cc

Docs and plans:
- `docs/contributing/rfc-conformance-gates.md`, `docs/architecture/site-facts.md`, `docs/architecture/testing/test-health.md` (5b56fc88cf, 67932475cc)
- `docs/contributing/gh-pages.md` (67932475cc), committed by hunks because the file carries another session's uncommitted paragraph (the single-producer render paragraph), which stays in the working tree
- `plan/post-release/engineering/spec-rfc-optional-feature-ratio.md` - its Task placed its figure beside the per-row SHOULD states this spec stopped publishing; corrected to the four open optional sections in this spec's bookkeeping commit
- `plan/post-release/engineering/spec-site-rfc-recorded-data.md` - its Depends row names this spec, so its recording and goldens are cut from the four-section shape; edited in the same bookkeeping commit

The design said `internal/le/site/rfcdetail.go`, `internal/le/site/rfccompliance.go`
and `internal/le/site/homebody.go` needed no edit. That was wrong for the first
two: both carried prose calling an unchecked exclusion a "Ze gap", and both
took the page review's card decisions. `homebody.go` needed no edit: the home
card's sentence already says "for Ze or the OS".

## Files to Create
- `internal/le/rfc/rollup_published.go` - the published shape of `Rollup` and `ScopeRollup`, with the flat `mandatory-proven` and its read-back refusal (67932475cc)
- `internal/le/rfc/rollup_published_test.go` - the three rollup tests (67932475cc)
- `internal/le/rfc/sections_split_test.go` - the split's unit tests (67932475cc)
- `internal/le/rfc/sections_split_corpus_test.go` - the three corpus tests (67932475cc)
- `internal/test/fixture/register_le_help_rfc_tree_sections.go`, `internal/test/fixture/ui_fixture_le_help_rfc_tree_sections.go`, `internal/test/fixture/ui_fixture_le_help_rfc_tree_sections_test.go` - the wiring fixture and its unit tests (092738d86e)
- `test/ui/le-help-rfc-tree-sections.ci` - the help functional test, promoted from its draft (67932475cc)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config or RPC: development tooling and website |
| YANG validation constraints | N-A | No YANG leaf |
| YANG custom validators | N-A | No YANG leaf |
| CLI commands/flags | No | No `le` command or keyword is added; `./le rfc tree` keeps its grammar and changes its section words |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | N-A | No YANG leaf |
| Functional test for new RPC/API | Yes | `test/ui/le-help-rfc-tree-sections.ci` |
| Pipe completeness | No | The tree already answers through the shared renderer; its JSON changes shape with the section types |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No runtime dependency: nothing in the shipped binary changes |
| Doctor code per problem class | N-A | The refusals are `le` build-time refusals, not operator problems on a box |
| Prometheus counters/metrics | N-A | No daemon state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | No feature of `ze` changes |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | No `ze` command; the tree's help text is in `internal/le/rfc/actions.go` |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | No protocol wire; the site data shape is documented under row 12 |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/rfc-status.md` is regenerated by `./le rfc update` because the Proof cell counts OS verified rows; no summary row changes |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/test-health.md` (proof density) |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/contributing/rfc-conformance-gates.md` (Coverage figures, OS verified, The intent marker), `docs/architecture/site-facts.md` (RFC completion data), `docs/contributing/gh-pages.md` (Quality pages) |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived at implementation with `./le spec citation anchors`; the four pages of rows 10 and 12 are the ones that change. Two declared pages are unaffected: `docs/architecture/core-design.md` (declared by `render_ledger.go` and `actions.go` for the command engine, which this spec does not touch) and `website/AI.md` (declared by `facts.go` for the one-snapshot rule; it names no RFC section, intent or numerator, and the snapshot rule holds) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The tree JSON field list in `rfc-conformance-gates.md` and the data file table in `site-facts.md` |

## Implementation Steps

The section types are read by the rfc package, health and the site, in one
build of `le`. Removing a field breaks all three at once, and `le` is built
from committed HEAD for every session, so no commit may remove a field while a
reader still names it. The work is therefore two commits. The first changes
only a count's meaning and breaks no reader. The second changes the types and
every reader together, and because it changes what the website shows, it is
shown to Thomas before it is tested and committed.

1. **Phase: Wiring (MANDATORY FIRST)** -- the functional test of the tree's help and text, failing against today's words
   - Tests: `test/ui/le-help-rfc-tree-sections.ci`
   - Files: the `.ci` file and its fixture
   - Verify: it runs the built `le` and fails because the help still names the old section
2. **Phase: The numerator** -- OS verified in every published proven count; commit one
   - Tests: `TestProvenPercentCountsOSVerifiedRows`, `TestProofCellCountsOSVerifiedRows`, `TestEvidenceKindsSumToTheProofNumerator`, `TestAProofCellReadBackFromJSONCountsOSVerifiedRows`, `TestTheHomeCardCountsOSVerifiedRows`, `TestProofDensityCountsOSVerifiedRows`
   - Files: `profilecoverage.go`, `implemented.go`, `render_ledger.go`, `tree.go`, `facts.go`, `collect_rfc.go`
   - Docs in this phase: `rfc-conformance-gates.md` ("Coverage figures", "OS verified"), `test-health.md`
   - Verify: no field is renamed, so health and the site build unchanged; the floor question of R-3 is answered before the density is touched
3. **Phase: The split, in the working tree** -- the new sections at both levels, the private intent sum, every reader in the rfc package, health and the site, the bars, glossary, check words, fallback, script and styles. Not committed
   - Files: `sectioncounts.go`, `profilecoverage.go`, `tree.go`, `actions.go`, `next.go`, `check_evidence.go`, `collect_rfc.go`, the `internal/le/site/` files, `rfc-figures.js`, `30-components.css`
   - Verify: `le` builds from the working tree, and the RFC pages are rendered from it
4. **Phase: Thomas reviews the pages** -- the rendered profile, index, detail and home pages are shown with the counts of A-1, A-6 and A-7. Changes he asks for are made and shown again. Nothing is tested or committed in this phase
5. **Phase: Tests, goldens, docs and commit two** -- after his approval
   - Tests: every remaining test of the TDD plan, the two browser tests, the wiring test now passing; each changed expectation through the `ze-test-judge` route
   - Files: the tests, the three goldens, `rfc-conformance-gates.md` ("Coverage figures", "The intent marker"), `site-facts.md`, `gh-pages.md`, the two sibling specs
   - Verify: `./le site build` and `./le site check` over the `../gh-pages` checkout

**As built.** The work landed as three commits, not two: 092738d86e (the
wiring fixture, test-only Go, so the launcher's HEAD build could run it),
5b56fc88cf (the numerator) and 67932475cc (the split, the page-review cards
and the published rollup). The site tests and goldens of phase 5 were edited
before Thomas said the pages were approved, at his instruction to proceed;
he approved the pages on 2026-11-14. The `./le site build` and `./le site
check` run over `../gh-pages` had not run when this note was written.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | N-A: tooling scope |
| Correctness | Kept and Total are equal before and after for every RFC in the corpus; the four open sections sum to the old gap count (checked over 339 documents against a baseline taken before the change: 0 mismatches) |
| Naming | JSON keys kebab-case; the section words are identical on the tree, the bars, the glossary and health |
| Data flow | The not-built count is taken in `figuresByStem` only; no site file computes a section |
| Operator errors and doctor codes | N-A for doctor; each `le` refusal names the file, the cause and the command that fixes it |
| Rule: no-layering | `ze-gap` and the three intent keys are deleted, not kept beside the new keys |
| Rule: one declaration | No reader names a segment the bar declaration does not; the register in the script and in the fallback fill are the two exceptions, each covered by a test |
| Rule: test changes | Every changed golden and assertion carries a `ze-test-judge` approval |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Four open MUST sections on the tree | `./le rfc tree profile route-server` shows the four names with counts |
| No intent in published data | A search of the built site's data directory for the three intent keys answers nothing after `./le site build` |
| OS verified in the percentage | `TestProvenPercentCountsOSVerifiedRows` passes |
| Docs current | `./le doc check all` passes for the four pages |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The inputs are repository files the gates already parse; no new untrusted input |
| Published data | No private planning fact (an intent, a reason behind a not-ever) reaches a published file |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The published shape and the private shape were one type. Anything added to the section types is published three times over, which is how the intent counts reached the site. The public shape is now the default, and private facts are carried apart from it.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Take the not-built count in the row pass, with the existing unmet predicate | Derive it in the site from each RFC's unmet id list | The unmet list is not a subset of the gap rows (a proven row the audit calls wrong, a not-owed `{linux-limit}` row), so a count derived from it would be wrong, and the site would hold a second split |
| Remove the intent counts from the section types | Keep them in the types and have the site strip them before publishing | Stripping publishes by omission: the next type that embeds the sections publishes the intent again. A shape that cannot carry it cannot leak it |
| Keep OS verified as its own bar segment while counting it in the percentage | Merge it into Ze proven | Who performs a row is a fact the bar exists to show; the ruling is about what counts as done, not about hiding the layer |
| Delete `ze-gap` | Keep it as the sum beside the two new fields | `ai/rules/no-layering.md`: a kept sum is a second copy that can disagree |
| Split the open optional rows like the MUST rows | One "not proven" count for every open optional row | Thomas's decision at the design gate. It keeps the two levels uniform; its cost is that an optional row with no test reads as proof pending (R-2, A-7) |
| Land before `spec-site-rfc-recorded-data` | Land after it and bump its schema | Thomas's decision at the design gate: that spec has built nothing, so going first removes a schema step and corrects the public pages sooner |

## Known Limitations
- The not-built section is as accurate as the `{gap}` and `{partial}` marks. Correcting marks is `spec-fttx-edge-unproven-row-review` for that profile; the other profiles have no such spec yet.
- Recording the missing proofs is not in this spec.
- The intent marker stays readable in the public repository's summaries, which the ledger links. The ruling covers what the pages and their data state.
- Work landed as three commits, not one per phase: the section types cannot change without every reader, so the split and every reader went in one commit (67932475cc), after the wiring fixture and the numerator.
- RFC 9582, which the route-server profile requires, has 16 MUST rows with no tagged test and no annotation. They sit outside the 196 implemented RFCs' not assessed count (0), inside the 191 such rows across all walked documents. Found here, not fixed here: this spec changes how rows are counted, not which rows are tested.
- The guard of AC-7 (`TestDataIndexAndFilesAgree`) reads one published per-RFC file for the three intent keys, not every file the build publishes as R-7 planned. The other files carry no intent key because the section types that every published shape embeds cannot hold one.
- The changed-code lint (`./le go lint run scope` over the rfc, site and health packages) and `./le doc check all` had not run when this was written: the lint twice refused to plan while other sessions edited its inputs. The main thread runs both next.
- Five site tests were red before this work and it does not reach them: the lab ISO size, the homepage proof strip, the homepage hero, a lost-fact refusal message, and the shared header.
- The count of rows in not built whose only unmet mark is an audit verdict (A-6) was not measured.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
