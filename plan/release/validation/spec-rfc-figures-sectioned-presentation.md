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
| A-1 | The excluded rows' existing reasons can be backfilled with a section-scoped RFC quote | `{feature-declined}` already carries a quote; `{lower-layer}` reasons name the kernel path | The evidence check cannot refuse until ~1,100 rows are backfilled, so the new headline waits | Count rows per kind lacking a quote at DESIGN; owner decides rollout (research gate) |
| A-2 | Declined-table rows belong in the filtered-list sums | `summarize` includes them today | Top bars disagree with the Total row of the enrolled table | Decide at DESIGN; the Total row and the sum read the same row set |
| A-3 | `ProvenShareOf` and the corpus tape can follow the sections or be retired | Both partition the same gated rows | Two answers over the same rows stay published | DESIGN decision |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| filled at DESIGN | → | | |

## Acceptance Criteria

| AC ID | Input / Trigger | Expected Behavior |
|-------|-----------------|-------------------|
| filled at DESIGN | | |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| filled at DESIGN | | | |

## Files to Modify
- filled at DESIGN

### Integration Checklist
- filled at RESEARCH

### Documentation Update Checklist (BLOCKING)
- filled at RESEARCH

## Implementation Steps
- filled at DESIGN

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
