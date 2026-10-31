# Spec: rfc-figures-sectioned-presentation

| Field | Value |
|-------|-------|
| Status | done |
| Scope | tooling |
| Priority | P2 |
| Release reason | Accurate support claims: the published RFC coverage figure divides by rows Ze's code does not owe, so it misstates what Ze has done |
| RFC gaps | - |
| Depends | - |
| Phase | 1/5 |
| Handoff | - |
| Updated | 2026-10-30 |

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

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Per-RFC figures and the tree row | `internal/le/rfc/profilecoverage.go` `StemFiguresOf`, `internal/le/rfc/tree.go` `TreeRow` | The one partition every published RFC figure comes from | extends: carries the OS split and judges exclusion evidence, so every renderer reads the sections from it |
| Proof share and the attainable ceiling | `ProvenShareOf` (retired in 8debe22e4b), the corpus tape | A second partition publishing a different answer over the same rows | replaces: deleted, the section counts are the only answer |
| Exclusion evidence ratchet | `internal/le/rfc/check_evidence.go` `evidenceReader.gap` | Refuses an exclusion row without its evidence at check time | extends: covers all four kinds, and the figures reuse its `gap()` so a row leaves the count on the same judgement |
| Linux-limit register check | `internal/le/rfc/linux_limit.go` `linuxLimitIssue` | Validates a `{linux-limit}` row against the register | extends: shared as `linuxLimitIssue` between the check and the figures |
| Site facts and the marker convention | `data/site-facts.json`, `internal/le/site/factmark.go` `markedFact`; `docs/architecture/site-facts.md` | One JSON source per changing value, rendered into marked elements | extends: to `data/rfc/<stem>.json` and profile files (`internal/le/site/rfcdata.go`), rendered by `website/assets/js/rfc-figures.js`, fallback text from `rfcfallback.go` |
| Plain-figure ratchet | `website/data/plain-figure-ratchet.json`, `internal/le/site/plaintext.go` | Lists pages still carrying an unmarked figure and refuses growth | extends: shrunk to `"pages": []` as the pages moved onto markers |
| Site build producers | `internal/le/site/rfccompliance.go`, `rfcprofiles.go`, `rfcdetail.go`, `rfcfilter.go`, `facts.go` | Render the quality pages with figures inline | extends: each emits markers and the last build's value instead of its own figure |

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

→ Decision (phase 1a review rounds, as implemented): not-applicable payload is `"<RFC sentence from the cited section>"; binds <role>; <why>`. Ze's roles are declared once per summary in a `## Meta` row `Ze roles`; the bound role must appear in the quote as a whole word (case, article and plural normalised), must not equal, contain or sit inside any declared Ze role, and must not be a bare generic word (implementation, node, router, speaker ...). The quote is a whole sentence of the cited section and takes one of two shapes: the row's own obligation sentence, whose plain subject (determiners only, no conjunction, clause or preposition) is the bound role; or a scoping sentence with no BCP 14 keyword that carries a scoping marker (applies, only, limited to ...) and names the bound role. Non-role adjectives (compliant, robust, receiving ...) are stripped before the generic check. A row that fits neither shape is recorded as `{gap}`, never forced into another exclusion. Each declared Ze role must itself occur in the RFC text. A mechanical check cannot prove the semantic claim, so each backfill commit's rows also get an independent review before landing. Lower-layer tokens are `go:<pkg>` and `vendor/<module>`, each linked into the shipped `ze` binary, or `linux:CONFIG_<opt>`, set `=y` in the kernel profile as the builder layers it and listed in a `.require` manifest the build enforces. `{feature-declined}` may name the section its optionality quote comes from (`§<section> "<sentence>"; <reason>`), because an RFC often declares a feature optional in one section and puts its conditional obligations in another (RFC 9728 §2.2 vs §3.3); without a section the quote comes from the row's own section. The quote is a whole sentence carrying an optionality marker (MAY, OPTIONAL, SHOULD, RECOMMENDED, "optional", "if ... supports"); or, for an owner-declined feature, the row's own obligation sentence when it makes the obligation conditional on the feature's use ("If the client uses a delegated prefix ... MUST"), whose reason names the Ze producer showing Ze never meets the condition (owner decision 2026-10-30, `ai/META-DECISIONS.md`). The heading form (`heading "<feature words>"`, owner decision 2026-10-30) must match an entry of the summary's `## Meta` row `Declined features`, each entry naming its owner-decision source, so the owner decision a decline rests on is declared once per summary and reviewed per commit. → Constraint (2026-10-30): spec-rfc-support-derived-proof (session done-vs-tested) retires the summary Meta cells `Support status`, `Support coverage` and `Support remaining` in favour of a derived Proof / Evidence / Unmet row. Phases 4 and 5 build the per-RFC JSON and the public pages on that derived row, never on the retired cells; unmet obligations are disclosed only as annotated requirement rows. The producer to build on is `rfc.StemFiguresOf(in *rfc.RenderInput) map[string]rfc.StemFigures` (internal/le/rfc/profilecoverage.go, commit 947a07ca7c): per stem Walked, Mandatory and Optional figures (Proven, Total, Attainable, Excluded by kind), MandatoryByKind (proven mandatory rows by strongest evidence: interop, functional, editor, unit, unknown) and Unmet (ids of gap/partial/linux-limit rows and of wrong/unimplemented/partial audit verdicts). done-vs-tested's Phase 4 (in flight 2026-10-30) makes TreeRow hold StemFigures whole, adds `mandatory-by-kind` and `unmet` to the tree JSON and the ProofCell / EvidenceCell / UnmetCell helpers, and empties LedgerRow; it does not retire ProvenShareOf, which stays this spec's phase 4 work, done after that commit lands. Phase 4's section counts and `data/rfc/<stem>.json` derive from StemFigures, extended with the OS-verified / OS-testing-gap split, rather than recomputing from the rows.
→ Decision (2026-10-31, owner): a kernel-met row is split the same way at every level. A SHOULD/MAY `{lower-layer}` row counts OS verified (tagged unit with a verifying record naming its producer) or OS testing gap, never "not owed"; the overview bar's OS verified and OS gap segments count MUST and SHOULD rows together, the MUST bar still counts MUST rows only; "not owed" means only "does not bind Ze" (not applicable, declined); an `{intent}` marker on a lower-layer row is refused, since the kernel already performs it.
→ Decision (2026-10-31, owner, information-preservation approval): the RFC pages (profile pages and index, detail pages, filtered list, compliance index) drop the proven and mandatory/optional percentages, every attainable-ceiling figure and statement (per RFC, per profile, the lowest-ceiling sentence, the ceiling policy text), the excluded share per kind as a percentage, and the rollup figure in each profile page's meta description. The counts behind them stay as bar segments and marked values. Tests of the retired ceiling feature are deleted with it, after an independent ze-test-judge.
→ Decision (2026-10-31, owner): RFC 8907 CHAP, MS-CHAP and ASCII interactive login are owed, outside the current work and done later; the rows citing an unrecorded 2026-09-21 "Keep PAP" decision are `{gap}` (commit 95d73c66f5).
→ Decision (2026-10-30, owner): a technology aggregate has an optional tier (`| Aggregate tier | optional |`, commits 51d86a3149, fea892fa0d); RFC 4577 and its DN-bit companion RFC 4576 sit there for `ospf`. Phase 4/5's `data/rfc/profiles/<name>.json` carries an aggregate's optional members as their own tier, and the aggregate page's lead and the `tree` help text say the main table holds only required members (review notes on 51d86a3149). A row whose feature no owner decision names is a `{gap}`, whatever an earlier judge wrote (phase 1b found 11 such rows in rfc7011, rfc9190 and rfc3101). `{feature-declined}` is judged row by row and never counted in the backlog, because its form always carries a quote. A lacking row counts against the backlog only when HEAD^ and origin/main both hold it unchanged, and the backlog line may never rise against either.

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

Marker (agreed with `spec-site-values-from-one-file`): every RFC value on a page sits inside an element carrying `data-ze-rfc="<stem>:<field>"`, read from `data/rfc/<stem>.json`; site-wide values use `data-ze-stat="<key>"`. That spec's plain-text guard accepts a value only inside one of the two markers. If the guard lands first, the RFC pages still carrying server-written figures sit in its committed ratchet list, which may only shrink; phase 5 empties it (AC-19).
→ Constraint (guard landed, 750348ea2c): the ratchet is `website/data/plain-figure-ratchet.json` and lists one page, `quality/rfc-compliance/index.html`, with 21 plain figures, all `rfc.*` aggregates written by `rfccompliance.go`; the per-RFC detail pages carry none the matcher recognises. Only pages under `quality/rfc-compliance/` may be listed, and a listed page with no plain figure left is refused as stale, so the phase 5 commit that marks the index's figures removes that entry in the same commit.
→ Constraint: the index's "240 summaries" and `rfc.summaries` in `data/site-facts.json` (229, RFCs holding at least one requirement) count different populations. Phase 5's producer decides which one the index shows and names it in `data/rfc/index.json`; the two must not appear as the same figure.

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
| AC-16 | `./le site build` | Writes one `data/rfc/<stem>.json` per summary, one per profile, and the index; each per-RFC file's counts equal `./le rfc tree` for that stem; the Markdown mirrors link to the files and carry only the last build's values, written from those files (corrected at closure: `ai/META-DECISIONS.md`, "Website design", has a page keep only the last build's value as fallback text inside the marked element, and a mirror, which runs no script, follows the same rule through `fillMirror`) |
| AC-17 | Republishing one RFC's JSON with changed counts | The pages show the new counts with no page file changed |
| AC-18 | A per-RFC file listed in the index is missing | The page names the missing RFC; it is never silently dropped from a sum |
| AC-19 | The plain-text guard of `spec-site-values-from-one-file` over the built RFC pages | Every RFC figure sits inside a `data-ze-rfc="<stem>:<field>"` element; no RFC page remains in the guard's ratchet list |
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

### Deliverables Checklist

Added at closure (ze-close run 2), filled from the spec's commits since 588dd0febf.

| Deliverable | Verification method |
|-------------|---------------------|
| Evidence checks per exclusion kind, as a ratchet | `rfc/exclusion-evidence-backlog.txt` (`lower-layer 0`, `not-applicable 30`); `go test -run 'TestNotApplicableNeedsSectionRoleQuote|TestFeatureDeclinedQuoteIsSectionScoped' ./internal/le/rfc/` |
| OS verified / OS testing gap split | `go test -run 'TestLowerLayerWithRecordIsOSVerified|TestLowerLayerWithoutRecordIsOSGap' ./internal/le/rfc/` |
| SHOULD intent marker, refused on MUST | `go test -run 'TestIntentMarkerStates|TestIntentMarkerRefusedOnMust' ./internal/le/rfc/` |
| Section counts in the model and `./le rfc tree` text and JSON, no ceiling | `./le rfc tree profile fttx-edge` (MUST 747 kept = 342 + 0 + 120 + 285, no "ceiling"); `TestTreeActionRendersLevelsAndFigures`, `TestALinuxLimitMovesARowToNotOwedAndNotTheFigure`, `TestSectionsTextSeparatesNotOwedKinds` |
| `ProvenShareOf` retired; home, facts and test health read the sections | `grep -rn ProvenShareOf internal/ --include='*.go'` finds no producer; `rfc.ImplementedSectionsOf` in `factsFromRFCLedger` (`internal/le/site/facts.go`) and `implementedSections` (`internal/le/test/health/collect_rfc.go`) |
| `data/rfc/<stem>.json`, profile files and index | `go test -run 'TestPerRFCDataCountsEverySegment|TestDataIndexAndFilesAgree|TestTheNativeRefreshPublishesTheRFCData' ./internal/le/site/` |
| One browser renderer, `website/assets/js/rfc-figures.js` | `internal/le/site/browsertest`: `TestTheBrowserDrawsRFCFiguresFromTheirFiles`, `TestTheBrowserSumsTheFilteredRFCListFromItsFiles`, `TestTheBrowserDrawsAProfilePageFromItsFiles`, `TestTheBrowserDrawsAnRFCDetailPageFromItsFile` |
| Pages carry markers and the last build's value only (`rfcfallback.go`) | `TestTheComplianceIndexMarksEveryFigure`, `TestRFCDetailPageShowsItsFiguresFromItsFile`, `TestTheCompliancePagesGoldenIsCurrent` |
| Plain-figure ratchet emptied (AC-19) | `website/data/plain-figure-ratchet.json` holds `"pages": []` since 039558c95a; `TestSiteCheckHonoursAndShrinksTheRFCRatchet` |
| Glossary defines every bar term (AC-14) | `go test -run 'TestGlossaryDefinesBarTerms|TestGlossaryStatesOSBlockedNeverCountsAgainstZe' ./internal/le/site/` |
| Published correction of the old figure (AC-15) | compliance index `rfcEnrolledScope` (`rfccompliance.go`); health detail 1ea81fd051; home card 2c982c66ad |
| OS-verified colour token | `TestRFCSegmentColoursAreReadableInBothThemes` (`internal/le/site/browsertest/rfcfigures_test.go`) |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| HTML injection on the pages | Every value the build writes into a page goes through `html.EscapeString` (`fillHTML`, `rfcValueHTML`, `rfcBarMarkHTML`, `rfcMarkedHTML`); `rfc-figures.js` writes only `textContent`, never `innerHTML` (grep finds no `innerHTML` or `insertAdjacentHTML`) |
| A data file that is missing, unreadable or malformed | `loadRFCFigureSet` / `readRFCDataFile` refuse by path with the command that republishes; `rfcFigureSet.sum` refuses a stem the index does not list or a file with no requirements count; `sumStems` in the browser names the file rather than dropping it from a sum (AC-18) |
| A marker the fill cannot reach | `rfcStrayMarker` refuses an element or mirror token not in the helpers' exact shape, so no page publishes an empty figure |
| Path traversal through the index | `readRFCDataFile` joins the artifact root with a file name the same build wrote into `data/rfc/index.json`; build-time tooling reading its own artifact, no outside input (NOTE, not a finding) |
| Resource exhaustion | The corpus is the repository's own `rfc/short/`; every loop is bounded by it |
| Panics | Each `panic("BUG: ...")` in `rfcprofiles.go`, `rfcfilter.go`, `rfcdetail.go` guards a registry invariant (`rfcBars` layout, `rfc.AnnotationTraitsOf`) at build time; no peer or socket reaches them |
| Fail-open on the published figure | A not-owed row whose evidence fails stays among the rows Ze owes (`(*evidenceReader).notOwedGap`, `TestNoPublishedNotOwedRowLacksItsEvidence`) |

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

## Implementation Audit

ACs 1-6, 9-12, 14, 17 and 18 are demonstrated by the tests the Deliverables
Checklist names; the rows below are the ones the closure walk had to settle.

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-7 | Done | `TestTreeActionRendersLevelsAndFigures` (MUST kept and the four segments in the text), `TestALinuxLimitMovesARowToNotOwedAndNotTheFigure` (not owed by kind in text and JSON; refuses "ceiling" and "%"), `TestSectionsTextSeparatesNotOwedKinds` (SHOULD states); live `./le rfc tree profile fttx-edge`: MUST 747 kept = 342 + 0 + 120 + 285, 0 lines name a ceiling | Producer `(*Sections).String`, `writeNotOwed` (`internal/le/rfc/sectioncounts.go`). TDD-plan name `TestTreeReportsSectionCounts` does not exist; these replace it |
| AC-8 | Done | `TestRollupNeverCompleteWithUnwalkedOrEmpty`, `TestTreeActionRendersLevelsAndFigures` (rollup 1 not walked), `TestAnUnwalkedRFCPublishesNoCounts` (site data), `rfcprofiles_figures_test.go` "an empty tier" and browsertest `rfcfigures_test.go` `empty-bar` (render `no rows`) | Producers `rollupOf`, `rfcFigureSet.sum`/`value`/`bar`, `sumStems`. TDD-plan name `TestSectionRollupSumsWalkedMembersOnly` does not exist |
| AC-13 | Done | `TestTheComplianceIndexMarksEveryFigure` (no ceiling, no share, no bucket table), `TestRFCDetailPageShowsItsFiguresFromItsFile` (no ceiling), `TestTheCompliancePagesGoldenIsCurrent`; `ProvenShareOf` has no producer left; home, facts and health read `rfc.ImplementedSectionsOf` | TDD-plan name `TestNoCeilingOrSecondPartitionPublished` does not exist; these replace it |
| AC-15 | Done (gap fixed) | Compliance index `rfcEnrolledScope` (`rfccompliance.go`); health detail 1ea81fd051 (`TestDensityDetailCountsAnExclusionNotOwedOnlyWithItsEvidence`); home RFC card 2c982c66ad (`TestTheHomepageRFCCardSaysWhatTheOldFigureCounted`) | Before run 2 only the index said what the old figure counted; the home card and the health page carried the old percentage with no statement. The profile and detail pages dropped their percentages under the owner's information-preservation approval and publish no replacement figure, so they owe no correction line; the index, home and health surfaces, which do publish one, carry it |
| AC-16 | Done | `TestPerRFCDataCountsEverySegment`, `TestDataIndexAndFilesAgree`, `TestTheNativeRefreshPublishesTheRFCData`; mirrors link the files (`rfcProfileDataLinks`, `rfcDetailFiguresMirror`) and carry the last build's values through `fillMirror` (`rfcfallback.go`) | AC text corrected at closure: "carry no figures" contradicted `ai/META-DECISIONS.md`, "Website design" (a page keeps the last build's value as fallback text inside the marked element), which the mirrors follow (`docs/architecture/site-facts.md`) |
| AC-19 | Done | `website/data/plain-figure-ratchet.json` is `"pages": []` since 039558c95a; `TestSiteCheckHonoursAndShrinksTheRFCRatchet`; figures reach a page only through `fillHTML` into markers, `rfcStrayMarker` refuses a marker it cannot fill | The guard matches `data/site-facts.json` values only, so an RFC figure outside a marker is prevented by construction (renderers emit markers, never values), not by the guard. `./le site check` over a fresh build is still owed (main thread) |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/rfc-figures-sectioned-presentation-1565110b-b077-440f-8648-0ceabeb0ceef.md` (12 files, verdict clean) |
| `./le spec review check` | review_gate: OK after the independent re-review of 1ea81fd051, 2c982c66ad, 945cfec3ad and 3de4ce3118 (9 files, clean); ./le rfc check refuses only two stale RFC 4271 audit verdicts, not this spec's |
| Rounds | 2 (run 1: sectioncounts.go, implemented.go, next.go rowOwes, rfc-figures.js sumStems; run 2: rfcfallback.go, rfcprofiles.go, rfcfilter.go, rfcdetail.go, rfccompliance.go, facts.go, collect_rfc.go, check_evidence.go, linux_limit.go, and run 1's fixes) |
| Reviewer lenses used | logic and population of every published count, published-claim accuracy, HTML injection and refusal paths, stale comments |

Not yet covered by the artifact: `internal/le/test/health/collect_rfc.go` and
`internal/le/site/homebody.go`, which carry run 2's own fixes; they owe a pass
by a reviewer who did not write them.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `./le rfc next` named an unproven `{intent: not-ever}` SHOULD row as owed | `rowOwes`, `internal/le/rfc/next.go` | d35c74defb |
| 2 | ISSUE | `sumStems` summed a walked RFC file with no must/should counts as zero rows | `website/assets/js/rfc-figures.js` | 32961c071d |
| 3 | ISSUE | The health detail said every `{not-applicable}`, `{feature-declined}` and `{linux-limit}` row is counted not owed; the sections keep a row whose evidence fails as a Ze gap, and the backlog shelters 30 not-applicable rows | `densityMetric`, `internal/le/test/health/collect_rfc.go` | 1ea81fd051 |
| 4 | ISSUE (AC-15) | The home RFC card and the health page changed their figure's population with no statement of what the old one counted | `homebody.go`, `densityMetric` | 2c982c66ad, 1ea81fd051 |
| 5 | NOTE | `isCounts` sat between `addCounts` and its doc comment | `rfc-figures.js` | 945cfec3ad |

## Implementation Summary

### What Was Implemented
- Phase 1: evidence checks per exclusion kind and the exclusion evidence ratchet (`check_evidence.go`, `rfc/exclusion-evidence-backlog.txt`), with the corpus backfill per RFC.
- Phase 2: `{lower-layer}` split into OS verified and OS testing gap at MUST and SHOULD level.
- Phase 3: the `{intent: not-yet | not-ever}` SHOULD marker, refused on a gated or lower-layer row (`intent.go`).
- Phase 4: section counts (`sectioncounts.go`, `ImplementedSectionsOf`), `./le rfc tree` text and JSON, `ProvenShareOf` retired (8debe22e4b).
- Phase 5: `data/rfc/<stem>.json`, profile files and index (`rfcdata.go`); the browser renderer `rfc-figures.js`; profile pages (2db85a86f9), detail pages (635e111e6f), filtered list (a992c571d8) and compliance index (039558c95a) on markers with the last build's value (`rfcfallback.go`); ceiling, tape and share removed; glossary (32961c071d); plain-figure ratchet emptied.

### Bugs Found/Fixed
- `rowOwes` named a not-ever SHOULD row as owed: d35c74defb, `TestNextNeverNamesANotEverRow`.
- `sumStems` drew an unsplit walked RFC file as zero rows: 32961c071d, browsertest case.
- Health detail overstated what leaves the count: 1ea81fd051, `TestDensityDetailCountsAnExclusionNotOwedOnlyWithItsEvidence`.
- Home card and health page lacked the AC-15 correction: 2c982c66ad, 1ea81fd051.
- `rfc.Requirement` grew past the rangeValCopy threshold with `Intent`: 8acf4b8d87 (back to 152 bytes).
- The inflation test's declined case had no corpus row left: 38f7c6f75e, judged.

### Documentation Updates
- `docs/contributing/rfc-conformance-gates.md` (annotation evidence, intent marker, OS verified, coverage figures), `docs/architecture/site-facts.md` ("RFC completion data", `rfcfallback.go` anchors), `docs/contributing/gh-pages.md` (quality pages), each landed with its phase. Closure result of `./le doc check all`: Pre-Commit Verification.

### Deviations from Plan
- AC-16 text corrected (see its row): mirrors carry the last build's values under the META "Website design" rule.
- Phase order: Implementation Steps put phase 5 after phase 1 reaches zero; phase 5 landed with `not-applicable 30` in the backlog. The order existed so that no excluded row raises a figure without evidence, and that is enforced mechanically: `(*evidenceReader).notOwedGap` keeps each of the 30 rows among the rows Ze owes as a Ze correctness gap, and `TestNoPublishedNotOwedRowLacksItsEvidence` checks the published not-owed counts equal the evidenced rows alone.
- Five TDD-plan test names were implemented under other names; the Implementation Audit maps each.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The health and home surfaces changed their figure's population without the AC-15 correction | AC-15 binds every surface that published the old figure | Closure AC walk | Fixed in 1ea81fd051, 2c982c66ad |
| assumption | AC-16 said mirrors carry no figures | The META "Website design" rule keeps the last build's value as fallback; a mirror runs no script | Closure review | AC text corrected |

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The published figure divides only by rows Ze and the OS owe | functional, real corpus | `TestNoPublishedNotOwedRowLacksItsEvidence` over this checkout's corpus; `./le rfc tree profile fttx-edge`: MUST 747 kept = 342 Ze proven + 0 OS verified + 120 OS testing gap + 285 Ze gap |
| Every page reads one set of data files and holds no figure of its own | browser (Playwright) | `TestTheBrowserDrawsRFCFiguresFromTheirFiles`, `TestTheBrowserSumsTheFilteredRFCListFromItsFiles`, `TestTheBrowserDrawsAProfilePageFromItsFiles`, `TestTheBrowserDrawsAnRFCDetailPageFromItsFile`; plain-figure ratchet `"pages": []` |
| A reader can tell what each segment means and what the old figure counted | unit over the rendered page | `TestGlossaryDefinesBarTerms`, `TestGlossaryStatesOSBlockedNeverCountsAgainstZe`, `TestTheHomepageRFCCardSaysWhatTheOldFigureCounted`, `TestDensityDetailCountsAnExclusionNotOwedOnlyWithItsEvidence` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Optional-feature ratio beside the SHOULD states | Owner split it out so it does not hold this spec up | `plan/post-release/engineering/spec-rfc-optional-feature-ratio.md` |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/site/rfcfallback.go`, `rfcdata.go`, `website/assets/js/rfc-figures.js`, `internal/le/rfc/sectioncounts.go`, `intent.go` | yes | reviewed at their current content (Review Gate artifact) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-7, 8, 13, 15, 16, 19 | as the Implementation Audit rows | scoped runs 2026-10-31: `./internal/le/test/health/` ok; `./internal/le/site/` red only on the two tests reading the untracked `docs/features/rfc-status.md`; live `./le rfc tree profile fttx-edge` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le rfc tree`, `./le site build`, page in a browser | none (tooling); the browsertest Playwright suite drives the built pages | `TestTheBrowserDrawsRFCFiguresFromTheirFiles` and siblings |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-N (Risks & Assumptions) | confirmed or broken per phase review | AC-16 assumption broken and recorded in the Mistake Log; phase-order deviation recorded in Deviations |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| site-facts.md "RFC completion data", rfc-conformance-gates.md, gh-pages.md | anchors to `rfcfallback.go`, `sectioncounts.go`; `./le doc check all` | three stale anchors this spec left fixed in 3de4ce3118; the check still fails on three anchor claims in interop.md and ipsec.md, not this spec's |

NOTEs, not fixed: `rfcFigureSet.value` answers "0" for a `not-owed-by-kind`
field of a walked file that lacks its level, where `sumStems` refuses (files
always carry both levels, `rfcDataOf`); the health detail renders `NaN%` over an
empty population, which `rfcMetrics`' guards make unreachable.
