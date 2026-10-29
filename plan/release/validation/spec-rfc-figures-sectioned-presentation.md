# Spec: rfc-figures-sectioned-presentation

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P2 |
| Release reason | Accurate support claims: the published RFC coverage figure divides by rows Ze's code does not owe, so it misstates what Ze has done |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-29 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The RFC compliance pages publish one mandatory figure per RFC and per profile:
rows proven over every MUST row, with a "ceiling" beside it. Rows the operating
system performs, rows of an optional feature Ze declined, not-applicable rows
and Linux-limit rows all sit in the denominator. For the fttx-edge profile the
page reads 338 of 747 (45.2%) while Ze's own code owes 529 of those rows. The
figure therefore misreports Ze's state, and the ceiling hides the reason.

Replace the single figure and its ceiling with sections, following the owner
rules in `ai/META-DECISIONS.md` ("Goal and scope": an OS-met requirement
without a test is a testing gap; an OS-blocked requirement is reported in its
own section and never counts against Ze's mandatory coverage or holds a
release).

| Section | Counts | Status values |
|---------|--------|---------------|
| Ze MUST coverage | MUST rows Ze's code owes: total minus OS met, OS blocked, declined, not applicable | proven, correctness gap |
| OS met | MUST rows a lower layer performs, per OS and kernel version | verified (a test checks the OS behaviour on the state Ze installs), testing gap |
| OS blocked | MUST rows Linux cannot provide the primitive for, with a verified limit entry | blocked; never counted against Ze, never holds a release |
| Declined feature | MUST rows conditional on an optional feature Ze does not offer | declined, citing the RFC sentence that makes it optional |
| Not applicable | MUST rows that do not bind Ze's role | listed with evidence, not counted |
| Optional features | optional features offered over optional features defined | ratio; owned by `plan/post-release/engineering/spec-rfc-optional-feature-ratio.md`, not this spec |
| Optional requirements | SHOULD / MAY rows | proven, not yet, not ever, not decided (below) |

Decisions agreed with Thomas, 2026-10-29:

| Decision | Ruling |
|----------|--------|
| Leaving Ze's section raises the headline | Yes, but only with checked evidence: a row moves to OS met, OS blocked, declined or not applicable only when it cites the RFC sentence (section, read in `rfc/full/`) and the kernel behaviour or role condition that settles it. A check that refuses a moved row without that evidence lands before the new headline is published. This supersedes the 2026-10-25 ruling that annotating a row may not raise its figure |
| Gauges | Two bars per RFC set and per RFC row, same colours and same left-to-right order: a MUST bar and an overview bar (below) |
| MUST bar | The promise Ze must keep to be RFC compliant. Covers only the MUST rows the stack owes (Ze plus OS met); rows not owed (not applicable, declined, OS blocked) are left out of it |
| Overview bar | Every row of the RFC, MUST and SHOULD / MAY, so a reader sees how much of the optional part Ze supports as well. Counts on each segment, no single percentage |
| SHOULD / MAY states | Every row starts `not decided`. `not yet`: we intend to support it, not done; set during RFC work. `not ever`: we will not support it; only Thomas sets it (an agent may propose). `proven`: a tagged test with a verifying discrimination record |

Presentation, as agreed against the fttx-edge mockup (tables per section, the
gauges below):

| Surface | What it shows |
|---------|---------------|
| Profile summary | One line per section, plus an "All MUST rows" line equal to the sum of the five MUST sections |
| RFC list (`?profile=<name>&rfc=<stems>#rfc-list`) | One row per RFC: MUST total, then a column pair per section; a Total row at the bottom whose five MUST section columns add up to the MUST total, with a check line |
| RFC detail page | Each requirement with level, section, status and evidence, ending in a total that matches the RFC's row in the list |

Gauges: the current separate mandatory and optional bars are replaced by the
MUST bar and the overview bar above. Segments, one colour each, each also
carrying its name and count in text so the split stays readable without colour:

| Bar | Left to right |
|-----|---------------|
| MUST | Ze proven (green), OS verified (light green), OS testing gap (amber), Ze correctness gap (red) |
| Overview | Ze MUST proven (green), OS verified (light green), SHOULD proven (teal), not owed (grey), SHOULD not ever (light grey), SHOULD not yet, SHOULD not decided, OS testing gap (amber), Ze correctness gap (red) |

Mockup on fttx-edge counts from the live tree on 2026-10-29 (no OS-met row is
verified yet, and every unproven SHOULD row is still `not decided`):

```
MUST      |███████████████████████████████|▚▚▚▚▚▚▚▚▚▚▚|XXXXXXXXXXXXXXXXXX|  340/648 kept (52%)
           Ze proven 340                   OS untested 119  Ze gap 189

Overview  |███████████████████|▒|·····|░░░░░░░░░░░░░░░░░░|▚▚▚▚▚▚▚|XXXXXXXXXX|  all 1,082 rows
           MUST proven 340     S 7 not owed 99  SHOULD not decided 328  OS untested 119  Ze gap 189
```

| Where | Gauges |
|-------|--------|
| Top of the filtered RFC list (`quality/rfc-compliance/?profile=...`), before the tables | Both bars for the whole set of RFCs shown, from the same sums as the Total row |
| Each RFC row of the RFC table | Both bars per row, same segments, colours and order |

Segment widths are row counts over the bar's total, so the top bars and the
Total row show the same numbers.

Counts are summed across RFCs and divided once; percentages are never averaged.
The ceiling presentation and the old mandatory and optional bars are removed,
not kept beside the sections (`ai/rules/no-layering.md`). Published figures are corrected on the surfaces
that carried them, saying what the old figure counted.

Related: `plan/release/validation/spec-excluded-requirement-owes-an-observable-test.md`
adds the gate that enforces the OS-met test; this spec shows its absence as a
testing gap. Neither waits on the other.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - "Coverage figures", "The linux-limit annotation", the guard table
  → Constraint: "Coverage figures" states the 2026-10-25 rule (annotating cannot raise a figure) and the ceiling; both are rewritten with the code. Rollups sum walked members before dividing, an unwalked RFC publishes no figure, a zero total renders `no rows`: all three stay
- [ ] `docs/contributing/gh-pages.md` - "Quality pages"
  → Constraint: the page names every renderer of the bars and ceilings (filtered list, profile pages and index, detail pages, mirrors, JSON); each paragraph describing a ceiling or the two bars is rewritten in the same change

**Key insights:** under Current Behavior.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/profilecoverage.go` - the per-RFC and rollup figures
  → Constraint: `figuresByStem` keeps `{not-applicable}`, `{lower-layer}`, `{feature-declined}` and `{linux-limit}` rows in `Total` and never proves them "whatever records their tags carry"; an OS-met "verified" status needs a new proof predicate for those rows, separate from Ze's `Proven`
  → Constraint: `{gap}` and `{partial}` rows are never proven even with a verifying record; this stays for the Ze section
  → Decision: the code comment cites an owner ruling of 2026-10-25 that "annotating a row may not raise its figure"; the new owner rules supersede it for the headline, so the comment and `docs/contributing/rfc-conformance-gates.md` "Coverage figures" are rewritten with the code
  → Constraint: `rollupOf` sums only walked members and reports `NotWalked`; the sectioned rollup keeps that rule
  → Constraint: SHOULD / MAY rows (`!Gated()`) go through the same annotation partition into `Optional`; no field records intent, so `not yet`, `not ever` and `not decided` need a new per-row declaration in the summary, with `not decided` the default when absent
  → Constraint: a row holds ONE coverage annotation (`rowMarkers.add`, `summary.go`, refuses a second), so SHOULD intent cannot share that slot with `{gap}` or `{not-applicable}`; it needs its own marker slot, following `{superseded}` / `{updated-by}` (`stripMarkers`, `rowMarkers.add`)
  → Constraint: kind to section: none, `{gap}`, `{partial}`, `{single-polarity}` → Ze; `{lower-layer}` (215 rows in the corpus) → OS met; `{linux-limit}` (0) → OS blocked; `{feature-declined}` (63) → declined; `{not-applicable}` (833) → not applicable. `{rollup}` (15) is outside every denominator (`Counts` false) and fits no section, so "All MUST rows" counts non-rollup rows only
  → Constraint: nothing can prove a `{lower-layer}` row today: it never `Proves`, and `BarsTests` refuses any tagged test on it as a stale annotation (`check_core.go`). "OS verified" needs `BarsTests` off for lower-layer and a new predicate: lower-layer plus a tagged unit with a verifying discrimination record
- [ ] `internal/le/rfc/check_core.go`, `summary.go`, `linux_limit.go` - the exclusion evidence checks
  → Constraint: evidence today, against the owner rule (RFC sentence with section, plus the kernel behaviour or role condition): `{not-applicable}` has NO content check (reason must only be non-empty); `{lower-layer}` checks a producer `<path>.go::<Symbol>` exists (`checkLowerLayerProducer`), no RFC quote; `{feature-declined}` checks a quote is somewhere in the RFC text (`featureDeclinedQuote`, whole-text match, section not checked) and a producer; `{linux-limit}` checks the register entry (kernel side complete), no RFC sentence. The new check adds the section-scoped RFC quote to all four and a role condition to not-applicable; 833 not-applicable rows corpus-wide owe a backfill before it can refuse
- [ ] `internal/le/rfc/tree.go`, `priority.go`, `next.go`, `provenshare.go` - consumers of the figures
  → Constraint: `./le rfc tree` and `./le rfc next` print `Rollup.String()` and the ceiling; they change with the site (same model, `ai/rules/no-layering.md`)
  → Constraint: `ProvenShareOf` (`provenshare.go`) is a second figure used by the home page, facts, the compliance cards and test health; it keeps lower-layer and not-applicable in its denominator. DESIGN decides whether it follows the sections or is retired
  → Constraint: live fttx-edge (`./le rfc tree profile fttx-edge`, 2026-10-29): mandatory 340 of 747, optional 6 of 335, excluded feature-declined 18, lower-layer 119, not-applicable 81, linux-limit 0. Ze 529, MUST bar 648, Ze gap 189
- [ ] `internal/le/site/rfcfilter.go` - filtered RFC list, overview box, browser script
  → Constraint: `rfcFigureRowCells` writes `data-mp/mt/op/ot/ma/oa/me/oe` per `tr[data-rfc]`; `rfcFilterScript` `summarize` re-sums visible rows of BOTH the enrolled and the declined tables, walked only, and writes the two single-span overview bars and the ceiling text. It becomes one data attribute per segment and one sum feeding both top bars, a new Total row and its check line. No test runs the script; `TestTheOverviewSumsTheCeilingTheRollupSums` re-implements it in Go
  → Constraint: without scripts every row is shown, so the static HTML and the Markdown mirror need a server-side Total over all rows
  → Constraint: removed with the ceiling (`ai/rules/no-layering.md`): `rfcRowCeilingCells`, `rfcCeilingStatement`, `rfcCeilingFigure`, `rfcCeilingReasons`, `rfcExcludedJSON`, `rfcCeilingPolicy`, `rfcCeilingKinds`, `rfcIndexCeilingRows`, the ceiling legend, and in `internal/le/rfc` `CeilingString`, `CeilingSummary`, `CeilingOf`, `lowerCeiling`, `AnnotationTraits.CeilingExcluded` / `CeilingLabel`
- [ ] `internal/le/site/rfcprofiles.go`, `rfcdetail.go` - profile pages, profile index, RFC detail
  → Constraint: `rfcProfileBarHTML` / `rfcProfileRollupBarHTML` draw a one-segment `.rfc-bar`; `rfcProfileCards` carries mandatory, optional and two ceiling cards; `rfcDetailCeilingHTML` / `Mirror` is the detail "Attainable ceiling" section, replaced by a total matching the RFC's list row
- [ ] `internal/le/site/rfccompliance.go` - corpus index cards and "Requirement buckets" tape
  → Constraint: the existing `.rfc-tape` (`rfcSatisfactionHTML`) partitions gated MUST rows by TAG POLARITY and annotation (`rfcSatisfaction` buckets), not by recorded-break proof, and has no SHOULD rows. Its `.rfc-tape span{width:var(--w)}` markup can be reused; its partition cannot. Two partitions of the same rows on one site would publish two different answers, so DESIGN decides its fate
  → Constraint: `rfcToneRule`: a colour names what the measure means, never how well Ze scores; segment colours must be read that way. No light-green token exists (`website/assets/css/00-tokens.css`: `--mint-*` are greys, `--gold-*` purple); OS verified needs a new token
- [ ] `internal/le/site/rfcreader.go` - glossary and reader text
  → Constraint: becomes false and is rewritten with the code: the "Ceiling" glossary entry, the linux-limit entry ("still binds Ze and remains unmet ... stays in the denominator", which contradicts the OS-blocked owner rule), "every share is over the gated requirements", the `rfcIndexLegend` text, `rfcProfileTreeProse`, `rfcProfileCards` meanings, the `rfcStanding` / `rfcSatisfaction` linux_limit wording
- [ ] `internal/le/site/actions.go` - `./le site build`, `./le site check`
  → Constraint: no site check compares figures across surfaces; agreement between top bars, Total row, profile cards and detail totals is held only by unit tests, so this spec adds one

**Key insights:**
- Every figure comes from `figuresByStem` in `internal/le/rfc`; the site computes none, except the browser re-sum of the filtered list. The sections are a change to that one partition plus every renderer of it.
- OS verified cannot exist today: `{lower-layer}` refuses tests. Turning that off is part of this spec.
- Three exclusion kinds carry partial evidence checks; `{not-applicable}` (833 rows corpus-wide) carries none. The owner's evidence rule needs a section-scoped RFC quote on all four.
- A second partition (`ProvenShareOf`, the corpus tape) publishes a different answer over the same rows.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `rfc/short/<stem>.md` requirement rows (level, annotation, the new SHOULD intent marker), parsed by `parseSummaryFile` (`internal/le/rfc/summary.go`); test tags and `rfc/discrimination/<stem>.json` records give proof
- Read out by `./le rfc tree`, `./le rfc next`, and `./le site build` (quality pages)

### Transformation Path
1. Parse: `parseSummaryFile` → `Requirement` with one coverage annotation, `Gated()` level
2. Prove: `provenRequirements` over discrimination verdicts
3. Partition: `rowCounts` / `figuresByStem` → per-stem `StemFigures` (today Mandatory and Optional `Figure` with ceiling `Excluded`); becomes per-section counts
4. Roll up: `rollupOf` / `ScopeRollup` (walked members only) → profile and aggregate figures
5. Render: `TreeRow` and its JSON → CLI text, site HTML, Markdown mirrors, `data/rfc-compliance.json`, browser re-summing in the filtered list

### Boundaries Crossed
- `internal/le/rfc` (model) → `internal/le/site` (rendering) → `../gh-pages` (published pages and JSON) → browser script

### Integration Points
- `annotationKinds` / `AnnotationTraitsOf` (one declaration of what each kind means), `TreeRow` JSON, site consumers named under Current Behavior

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validation |
|----|------------|-------|----------|------------|
| A-1 | The excluded rows' existing reasons can be backfilled with evidence | `{feature-declined}` already carries a quote; `{lower-layer}` reasons name the kernel path | A row whose evidence cannot be found goes back to Ze's section | → Decision (Thomas, 2026-10-29, research gate): roll out ALL AT ONCE. No RFC switches to the sections until every excluded row in the corpus (about 833 not-applicable, 215 lower-layer, 63 feature-declined) carries the evidence and the check refuses a row without it. The backfill is the first implementation phase |
| A-2 | Rows of the not-enrolled table belong in the filtered-list sums | `summarize` includes them today, and a profile link must read that profile's rollup | Top bars disagree with the profile page | → Decision: the top bars, the Total row and the profile rollup read the same row set: every shown walked RFC, both tables. Answered by the existing rule that a profile link reads that profile's percentages |
| A-3 | `ProvenShareOf` and the corpus tape are replaced by the sections | Both partition the same gated rows a second way; `ai/rules/principles.md` (every fact declared once) and `ai/rules/no-layering.md` | Two answers over the same rows stay published | → Decision: both retired; every consumer (home page, facts, compliance cards, test health) reads the sections |
| A-4 | Many `{not-applicable}` reasons are misclassified | Sampled RFC 2131 reasons read "ze answers no DHCPINFORM at all" (an absent feature: a gap, or declined only if Thomas named it) and "hands construction to the vendored library" (a lower layer that still owes a test, `ai/rules/rfc-compliance.md`) | Ze's headline falls once they move back; that is the honest figure | Backfill classifies each row from the RFC text; the published correction says what the old figure counted |
| A-5 | The kernel Ze ships is the only OS an OS-verified record speaks for | One shipped kernel, `internal/appliance/kernel.version`; `{linux-limit}` already checks against it | "Per OS and kernel version" in the Task needs a dimension nothing records | → Decision: OS verified means verified on the shipped kernel; no per-version breakdown |

### Risks

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | The backfill is ~1,100 rows of reading RFC text; it stalls and the sections never publish | Ratchet count stops falling | Ratchet (existing drain pattern, `docs/contributing/rfc-conformance-gates.md` "The ratchets"): refuses a NEW row without evidence from phase 1, flips to full refusal at zero; backfill committed per RFC |
| R-2 | An agent marks a SHOULD row `not ever`, which only Thomas may set | A `not ever` marker added in a diff | The review gate (`/ze-review`) flags every added `not ever`; a `not ever` on a MUST row is refused outright |
| R-3 | The browser renderer sums wrongly and no Go test sees it | A Playwright test's rendered Total differs from the model's counts for the same RFCs | Playwright tests load fixture JSON and real published JSON, and compare rendered segment counts and the Total row with the model's section counts |
| R-6 | A per-RFC file and the index drift (stem renamed, file missing) | Renderer shows a missing RFC | The build writes all files in one pass from the model; a Go test checks every index entry has its file and every file is indexed; the renderer names a missing file instead of dropping it |
| R-4 | Turning `BarsTests` off for `{lower-layer}` lets a test with no record look like proof | An OS-verified count with no discrimination record | OS verified requires the same `DiscriminationVerdict.Proven` predicate as Ze proven |
| R-5 | Removing the ceiling and old bars changes tests that assert them | Ceiling tests red | Those tests are replaced, each judged by `ze-test-judge` (`ai/META-DECISIONS.md`, "Test changes") |

## Design

Alternatives considered:

| Approach | How | Verdict |
|----------|-----|---------|
| A: model owns the sections (chosen) | `internal/le/rfc` replaces `Figure` and the ceiling with per-section counts per RFC and per rollup; CLI, site, JSON and browser read them | Wins: one partition, the site computes nothing (`docs/contributing/gh-pages.md`), OS verified and SHOULD intent need model facts anyway |
| B: site derives sections | Site maps `Figure.Excluded` kinds to sections | Rejected: a second partition outside the model; cannot express OS verified or SHOULD intent |
| C: sections beside the ceiling | Keep ceiling, add sections | Rejected: `ai/rules/no-layering.md` |

Evidence per exclusion kind (phase 1). The row text itself is already the verbatim RFC span its section cites (`./le rfc check`, "The row quote"), so the requirement sentence is in hand; what each kind adds is the fact that removes the row from Ze's count:

| Kind | Evidence required | Checked by |
|------|-------------------|------------|
| `{not-applicable}` | A role-condition quote from the SAME RFC section the row cites, showing the obligation binds a role Ze does not fill (for example a sentence scoping it to hosts), plus the role Ze fills. "Ze does not implement X" is never not-applicable: it is a `{gap}`, or `{feature-declined}` for a feature Thomas named | New section-scoped quote check |
| `{lower-layer}` | Named layer component (kernel subsystem or vendored module path) and the Ze producer that installs the state it acts on | `checkLowerLayerProducer` (exists) plus a layer-path check |
| `{feature-declined}` | The optionality quote, now matched within the row's section | `featureDeclinedQuote` narrowed to the section |
| `{linux-limit}` | Register entry (exists) | `checkLinuxLimit` (unchanged) |

SHOULD / MAY intent (phase 3): a separate marker slot beside the coverage annotation, like `{superseded}`: `{intent: not-yet}` and `{intent: not-ever; <reason citing the RFC's optionality>}`. Absent means `not decided`. Refused on a gated (MUST-level) row. `proven` is never declared, it is computed.

OS verified (phase 2): `{lower-layer}` stops barring tests; a lower-layer row whose tagged unit carries a verifying discrimination record (`DiscriminationVerdict.Proven`) counts as OS verified, otherwise OS testing gap.

Section counts (phase 4), per RFC and per rollup, walked members only, summed before dividing:

| Bar | Counts |
|-----|--------|
| MUST | Ze proven, Ze gap, OS verified, OS gap; the bar's total is their sum. Not owed (not applicable, declined, OS blocked) are counted but not in the bar |
| Overview | MUST: Ze proven, OS verified, not owed, OS gap, Ze gap. SHOULD / MAY: proven, not owed (annotated), not ever, not yet, not decided |

Publication (Thomas, 2026-10-29, design gate): the RFC completion data is published as ONE JSON FILE PER RFC on the website, and the RFC pages read it in the browser. A data change republishes that RFC's file; no page changes. A JavaScript browser is required to see the figures, which Thomas accepts.

| Artifact | Holds |
|----------|-------|
| `data/rfc/<stem>.json` | Every requirement row (RID, level, section, status, evidence) and the RFC's section counts, from the model |
| `data/rfc/profiles/<name>.json` | Profile kind, title, release flag, required and optional members |
| `data/rfc/index.json` | Every stem with its file, title, walked flag and enrolment |
| RFC list, profile pages, profile index, detail pages | One browser renderer loads the files, sums counts once (walked only, counts before dividing), draws both bars, the Total row and the check line. The build writes the last build's values inside the marked elements as fallback text only |
| Markdown mirrors of those pages | The last build's values, plus links to the JSON files as the source |

Scope split (Thomas, 2026-10-29, agreed with session json-website and `spec-site-values-from-one-file`): site-wide values (header, home, health, compare, CLI reference, release and downloads) live in `data/site-facts.json` and belong to that spec; RFC completion data, the RFC pages and the RFC index aggregates belong to this one. `internal/le/site/facts.go` and `home.go` are shared: whichever lands first, the other rebases. Rule: `ai/META-DECISIONS.md`, "Website design".

The existing whole-ledger `data/rfc-requirements.json` and the per-page server rendering of figures are replaced, not kept beside it (`ai/rules/no-layering.md`). The renderer is tested in a real browser through Playwright, already used for the website demos.

A profile's mandatory register is complete when Ze gap and OS gap are both zero. `{rollup}` rows stay outside every count, so the check line sums non-rollup MUST rows.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc tree profile <name>` | → | section counts in `internal/le/rfc` | `TestTreeReportsSectionCounts` (`internal/le/rfc/tree_test.go`) |
| `./le rfc check` over a summary | → | evidence checks per exclusion kind, intent marker parse | `TestNotApplicableNeedsSectionRoleQuote`, `TestIntentMarkerRefusedOnMust` (`internal/le/rfc/check_core_test.go`) |
| `./le site build` | → | per-RFC, profile and index JSON | `TestPerRFCDataMatchesTree`, `TestDataIndexAndFilesAgree` (`internal/le/site/rfcdata_test.go`) |
| Page loaded in a browser | → | `rfc-figures.js` renderer over the JSON | Playwright: filtered list Total and check line, profile page bars, detail total, missing file named |

## Acceptance Criteria

| AC ID | Input / Trigger | Expected Behavior |
|-------|-----------------|-------------------|
| AC-1 | A `{not-applicable}` row whose reason carries no quote from its own cited section | `./le rfc check` refuses it, naming the RID and the missing role-condition quote (full refusal once the ratchet reaches zero; before then only for rows the commit adds or changes) |
| AC-2 | A `{feature-declined}` row whose quote is in the RFC but outside the row's section | Refused, naming the section searched |
| AC-3 | A `{lower-layer}` row with a tagged unit and a verifying discrimination record | Counted OS verified, not refused as stale |
| AC-4 | A `{lower-layer}` row with no verifying record | Counted OS testing gap |
| AC-5 | `{intent: not-yet}` / `{intent: not-ever; ...}` on a SHOULD row; no marker | Counted not yet / not ever / not decided |
| AC-6 | `{intent: ...}` on a MUST row | Refused |
| AC-7 | `./le rfc tree profile fttx-edge` | Prints the MUST bar counts (Ze proven, OS verified, OS gap, Ze gap, their sum), the not-owed counts per kind, and the SHOULD states; no ceiling |
| AC-8 | Rollup over members with one unwalked RFC | Unwalked member contributes nothing and is counted apart; zero rows render `no rows` |
| AC-9 | Profile mandatory register | Complete only when Ze gap and OS gap are zero |
| AC-10 | Filtered RFC list in a browser, with and without a profile filter | Two bars on top and on each row; a Total row whose five MUST section columns sum to the MUST total, with a check line; every number equals the model's section counts for the same RFCs |
| AC-11 | Profile page, profile index, RFC detail page in a browser | Two bars from the same per-RFC files; detail page ends in a total equal to its list row |
| AC-16 | `./le site build` | Writes one `data/rfc/<stem>.json` per summary, one per profile, and the index; each per-RFC file's counts equal `./le rfc tree` for that stem; the Markdown mirrors link to the files and carry no figures |
| AC-17 | Republishing one RFC's JSON with changed counts | The pages show the new counts with no page file changed |
| AC-18 | A per-RFC file listed in the index is missing | The page names the missing RFC; it is never silently dropped from a sum |
| AC-12 | Segment order and colour | MUST bar: Ze proven, OS verified, OS gap, Ze gap. Overview: Ze MUST proven, OS verified, SHOULD proven, not owed, not ever, not yet, not decided, OS gap, Ze gap. Each segment also carries name and count in text |
| AC-13 | Any published page | No ceiling text, no old mandatory/optional single bars, no corpus tape bucket partition, no `ProvenShareOf` share; home page, facts, compliance cards and test health read the sections |
| AC-14 | Glossary and reader text | Linux-limit text states OS blocked never counts against Ze; no "ceiling" entry; every term in the two bars defined once |
| AC-15 | First publication | The pages that carried the old figure say what it counted (all MUST rows, annotated rows included) beside the new one |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNotApplicableNeedsSectionRoleQuote` | `internal/le/rfc/check_core_test.go` | AC-1 | |
| `TestFeatureDeclinedQuoteIsSectionScoped` | `internal/le/rfc/check_core_test.go` | AC-2 | |
| `TestLowerLayerWithRecordIsOSVerified`, `TestLowerLayerWithoutRecordIsOSGap` | `internal/le/rfc/profilecoverage_test.go` | AC-3, AC-4 | |
| `TestIntentMarkerStates`, `TestIntentMarkerRefusedOnMust` | `internal/le/rfc/summary_test.go`, `check_core_test.go` | AC-5, AC-6 | |
| `TestTreeReportsSectionCounts` | `internal/le/rfc/tree_test.go` | AC-7 | |
| `TestSectionRollupSumsWalkedMembersOnly`, `TestRegisterCompleteNeedsNoZeOrOSGap` | `internal/le/rfc/profilecoverage_test.go` | AC-8, AC-9 | |
| `TestPerRFCDataMatchesTree`, `TestDataIndexAndFilesAgree`, `TestMirrorsLinkDataAndCarryNoFigures` | `internal/le/site/rfcdata_test.go` | AC-16 | |
| Playwright: list Total and check line, profile bars, detail total, segment order, republished file, missing file | renderer test beside the demo harness | AC-10, AC-11, AC-12, AC-17, AC-18 | |
| `TestNoCeilingOrSecondPartitionPublished` | `internal/le/site/rfccompliance_test.go` | AC-13 | |
| `TestGlossaryDefinesBarTerms` | `internal/le/site/rfcreader_test.go` | AC-14 | |

Replaced tests (judged by `ze-test-judge`): the ceiling tests named under Current Behavior and `TestTheOverviewSumsTheCeilingTheRollupSums`.

## Files to Modify

| File | Change |
|------|--------|
| `internal/le/rfc/rfc.go` | Traits: section membership replaces `CeilingExcluded` / `CeilingLabel`; lower-layer `BarsTests` off; intent marker kind |
| `internal/le/rfc/summary.go` | Parse not-applicable role quote, lower-layer layer path, intent marker slot |
| `internal/le/rfc/check_core.go`, `check.go`, `linux_limit.go` | Section-scoped quote checks, intent refusal on MUST, the evidence ratchet |
| `internal/le/rfc/profilecoverage.go`, `tree.go`, `priority.go`, `next.go` | Section counts replace `Figure` / ceiling; CLI text |
| `internal/le/rfc/provenshare.go` | Retired; consumers move to the sections |
| `internal/le/site/rfcdata.go` (new) | Writes `data/rfc/<stem>.json`, profile files and the index from the model |
| `website/assets/js/rfc-figures.js` (new) | The one browser renderer: loads the files, sums, draws both bars, Total row, check line |
| Playwright test for the renderer (new, beside the existing demo harness) | AC-10, AC-11, AC-17, AC-18 in a real browser |
| `internal/le/site/rfcfilter.go`, `rfcprofiles.go`, `rfcdetail.go`, `rfccompliance.go`, `rfcledger.go`, `rfcreader.go`, `home.go`, `facts.go` | Two bars, Total row, check line, removal of ceiling, tape and share |
| `internal/test/health/collect_rfc.go`, `internal/test/fixture/ui_fixture_le_rfc_answers.go` | Read sections |
| `website/assets/css/00-tokens.css` | OS-verified light-green token, light and dark |
| `rfc/short/*.md` | Evidence backfill and reclassification, per RFC |
| `docs/contributing/rfc-conformance-gates.md`, `docs/contributing/gh-pages.md` | "Coverage figures", guard table, "Quality pages" |

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Developer tooling and website; no Ze config |
| YANG validation constraints | N-A | No YANG |
| YANG custom validators | N-A | No YANG |
| CLI commands/flags | Yes | `./le rfc tree` / `./le rfc next` output changes; no new keyword |
| CLI grammar (keyword before value) | N-A | No new keyword |
| Editor autocomplete | N-A | No new keyword |
| Functional test for new RPC/API | N-A | No RPC; site build tests cover the path |
| Pipe completeness | Yes | `./le rfc tree` JSON / table pipes carry the section fields |
| Env var registration | N-A | None |
| Doctor check for runtime dependencies | N-A | No runtime dependency |
| Prometheus counters/metrics | N-A | None |
| BGP family surface | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or scope/evidence/level changed? | Yes | The published RFC figures change meaning; corrected on the quality pages themselves (AC-15) |
| 2 | Config syntax changed? | N-A | No config |
| 3 | CLI command added/changed? | Yes | `le help rfc` text for `tree` / `next` if it names the ceiling |
| 4 | API/RPC added/changed? | N-A | None |
| 5 | Plugin added/changed? | N-A | None |
| 6 | Has a user guide page? | N-A | Contributor tooling |
| 7 | Wire format changed? | N-A | None |
| 8 | Plugin SDK/protocol changed? | N-A | None |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/*.md` reclassifications in phase 1; `docs/features/rfc-status.md` regenerated |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/rfc-conformance-gates.md` (annotation evidence, intent marker, OS verified) |
| 11 | Affects daemon comparison? | N-A | No |
| 12 | Internal architecture changed? | Yes | `docs/contributing/gh-pages.md` "Quality pages" |
| 13 | Route metadata keys added/changed? | N-A | None |
| 14 | Prometheus counters added/changed? | N-A | None |
| 15 | Registered plugin, event type, ... changed? | N-A | None |
| 16 | Changed source files referenced by doc source anchors? | Yes | Derived at implementation by `./le spec citation anchors spec plan/release/validation/spec-rfc-figures-sectioned-presentation.md` |
| 17 | Existing docs show examples for this area? | Yes | Annotation examples in `docs/contributing/rfc-conformance-gates.md` |

## Implementation Steps

Each phase commits when it compiles and works (`ai/META-DECISIONS.md`).

| Phase | Work | Lands |
|-------|------|-------|
| 1 | Evidence checks for the four kinds, as a ratchet refusing new rows without evidence; then the corpus backfill per RFC, reclassifying misclassified rows (A-4) from the RFC text | Per RFC; full refusal when the ratchet reaches zero |
| 2 | OS verified: lower-layer stops barring tests; the verified predicate | With its tests |
| 3 | SHOULD intent marker: parse, refusal on MUST, counts | With its tests |
| 4 | Section counts in the model, CLI output, JSON; `ProvenShareOf` retired | With its tests and the conformance doc |
| 5 | Site: per-RFC, per-profile and index JSON; page shells; one browser renderer for both bars, Total row and check line; Playwright tests; colour token, glossary; ceiling, tape, share and server-rendered figures removed | Only after phase 1 reaches zero (Thomas's all-at-once decision), with the gh-pages doc and the published correction |

## Owned journal rows

| Row | Journal file |
|-----|--------------|
| - | - |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le verify changed` evidence for the change set
