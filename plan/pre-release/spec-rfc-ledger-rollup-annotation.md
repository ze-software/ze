# Spec: a conformance rollup row derives its status from the rows it names

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-09-15 |

Scope is tooling: the change is in `internal/le/rfc` and `internal/le/site`,
so the Interop and End-to-End User Stories sections do not apply.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Some RFC requirements say nothing on their own. RFC 4302 Section 5 reads
"Implementations that claim conformance or compliance with this specification
MUST fully implement the AH syntax and processing described here for unicast
traffic, and MUST comply with all requirements of the Security Architecture
document", and its next sentence binds a multicast implementation to "the
additional requirements specified for support of such traffic", which are the
Section 2.4 rows already on the ledger. Such a row is true exactly when every
row it names is true. No test can carry it, because a tagged test proves one
requirement and a test on the rollup would claim more than its body checks
(`ai/rules/rfc-compliance.md`). None of the five annotation kinds fits it
either: `{gap}` on `RFC4302-5-2` accuses ze of owing behavior its constituents
already meet, `{lower-layer}` is refused by name for a rollup
(`docs/contributing/rfc-conformance-gates.md`, "It is not a conformance
rollup"), `{not-applicable}` denies a role ze fills, `{feature-declined}` needs
an optional feature, and `{single-polarity}` needs a test.

So `RFC4302-5-1` and `RFC4302-5-2` are the last two rows `./le rfc check`
reports as "has no test and no annotation" in that RFC, and nothing an author
can write today is both accepted and true.

The goal: a sixth kind, `{rollup: <target>, <target>, ...; why}`, whose status
the gate DERIVES from the rows it names and never asserts. The rollup reads as
met when every target is met, as a gap while any target is a gap, and as
unproven while any target is unproven; a target the corpus cannot show is
refused at parse time, exactly as a `{lower-layer}` producer the tree cannot
show is. The row then changes state the day its constituents do, with nobody
editing it: the "declare once, derive everything" principle applied to the
ledger (`ai/rules/principles.md`).

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - the annotation kinds, the ratchets, the public ledger's edges
  → Decision: `{lower-layer}` sits inside the gated denominator and outside the proven numerator, in its own bucket. A rollup does the opposite on the denominator: it carries no obligation of its own, so it sits OUTSIDE both, and its bucket shows the derived state
  → Constraint: a coverage-register line carries ONE disposition; a second marker beside an existing one is refused, never relabeled (`stripMarkers`)
  → Constraint: `checkGapCountAgreement` counts `{gap}` annotations only; a derived gap is not a `{gap}` and does not move the Remaining cell, because the constituent gap already did
- [ ] `docs/contributing/rfc-implementation-guide.md` - §9.7, the kind vocabulary an author reads
  → Constraint: every kind needs a reason; a bare annotation is refused
- [ ] `docs/contributing/writing-style.md` - the refusal sentences are operator-facing text
  → Constraint: one format sentence per kind, ending every refusal, as `lowerLayerFormat` does
- [ ] `docs/architecture/core-design.md` - the design page every `internal/le/rfc` file declares
  → Constraint: native tooling is a Go package behind a registered `./le` action; the kind reaches the operator through `./le rfc check` and no script
- [ ] `docs/architecture/testing/test-health.md` - the page `internal/le/testhealth/collect_rfc.go` declares
  → Constraint: the test-health split lists every annotation kind from `rfc.AnnotationKinds()`, so the page's kind list gains the rollup with its meaning
- [ ] `website/AI.md` - the page the site renderers `rfccompliance.go` and `rfcledger.go` declare
  → Constraint: every bucket on the compliance page has a key, a label and a tone in `rfcSatisfaction`; a kind with no bucket is red by test

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4302.md` - the two rows this spec exists for
  → Constraint: `RFC4302-5-1` names RFC 4301 as a whole, which is its own enrolled summary, so a target must be able to name another summary
- [ ] `rfc/short/rfc9190.md` - `RFC9190-2.4-1` and `RFC9190-5.6-4` point at RFC 8446 and RFC 7542, which have no summary
  → Decision: a target the corpus cannot show is refused. Those two rows stay unannotated until their RFC enrols; this spec does not annotate them

**Key insights:** (minimal context to resume after compaction)
- The parser dispatch is `parseAnnotation` (`internal/le/rfc/summary.go`); kinds live in `annotationKinds` (`rfc.go`); the gate arm is "has no test and no annotation" in `evaluate` (`check_core.go`)
- Every kind must have a site bucket or `TestEveryAnnotationKindHasABucket` goes red (`internal/le/site/rfccompliance.go`)
- The template commit is `a3ef6d27d9` (`{lower-layer}`), with `lowerlayer_test.go` and `lowerlayer_corpus_test.go` as the test shape

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/rfc/rfc.go` - `Annotation{Kind, Polarity, Layer, Quote, Producer, Reason}`, the kind constants, `annotationKinds`, `AnnotationKinds()`, `idRE` for a requirement id
- [ ] `internal/le/rfc/summary.go` - `parseAnnotation` cuts `kind: rest`, refuses an unknown kind and an empty reason, and branches to `parseLowerLayer` and `parseFeatureDeclined`; `stripMarkers` refuses two coverage annotations on one line; `parseSummaryText` holds one file's requirements
- [ ] `internal/le/rfc/check_core.go` - `evaluate` names a gated row with no tag and no annotation; `annotationBarsATest` lists the kinds a tag beside them makes stale; `checkLowerLayerProducer` and `checkFeatureDeclined` are the validator shape (loop, skip other kinds, `requirementWhere`, one refusal string per state)
- [ ] `internal/le/rfc/check.go` - the `notes(...)` wiring of the two validators
- [ ] `internal/le/rfc/check_status.go` - `checkGapCountAgreement` and `checkStatusAgreement` count `Kind == AnnotationGap` only
- [ ] `internal/le/rfc/check_ratchets.go` - `checkCoverageRatchet`, `checkLevelRatchet`, `checkRetiredRequirements` read tags, levels and ids, never annotations
- [ ] `internal/le/rfc/coverage.go`, `internal/le/rfc/provenshare.go` - `CoverageRows` counts any annotated row as `Annotated`, in `Gated` and out of `Proven`; `ProvenShareOf` adds `singlePolarityCounts` to the numerator
- [ ] `internal/le/site/rfccompliance.go` - `rfcAnnotationBucket` (kind to bucket), the `rfcSatisfaction` table, `rfcStanding`, `rfcNonBindingOf`
- [ ] `internal/le/site/rfcledger.go` - `rfcLedgerCoverage` with one field per kind
- [ ] `internal/le/testhealth/collect_rfc.go` - `annotationKinds` derived from `rfc.AnnotationKinds()`
- [ ] `internal/le/rfc/render.go`, `render_ledger.go` - the Proof column prints `{kind} reason` generically

**Behavior to preserve:** (unless the user explicitly said to change it)
- Every existing kind parses, validates and renders exactly as today; `lowerlayer_test.go`, `featuredeclined_test.go` and `lowerlayer_corpus_test.go` stay green untouched
- The published share moves by no annotation but `{single-polarity}` (`TestNoAnnotationExceptSinglePolarityMovesThePublishedShare`)
- The Remaining cell counts `{gap}` annotations and nothing else

**Behavior to change:** (only what the user asked for)
- `{rollup: ...}` is a sixth kind, parsed, validated against the corpus, derived at check time, rendered in its own bucket, and excluded from the gated denominator
- `RFC4302-5-1` and `RFC4302-5-2` carry it

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A `{rollup: <targets>; <why>}` trailing group on a Compliance Checklist line in `rfc/short/<stem>.md`, read by `./le rfc check`, `./le rfc index-update` and `./le site`
- A target is a requirement id (`RFC4302-2.4-2`) or a summary stem (`rfc4301`), the latter meaning every gated row of that enrolled summary except its own rollups

### Transformation Path
1. `parseAnnotation` (`summary.go`) cuts the kind, then a new `parseRollup` splits the targets on commas, holds each against `idRE` or the stem form, refuses an empty list, an empty reason, a malformed target and a duplicate, and returns `Annotation{Kind: "rollup", Targets: [...], Reason: why}`. It never sees the row's own id, so self-reference is step 2's refusal
2. `checkRollupTargets` (`check_core.go`, beside `checkLowerLayerProducer`) holds every target against the whole corpus: an id must exist in an enrolled summary, a stem must be an enrolled summary, a target must not be the row itself, and a target that is itself a rollup must not lead back to the row (a bounded walk over the target graph, depth capped at the number of rollups in the corpus)
3. `rollupDeriver.derive` (`check_core.go`, built by `newRollupDeriver` from one pass over the enrolled rows) answers one of three states from the targets' own state as `evaluate` already computes it: `met` when every target is proven in both polarities, or `{single-polarity}` proven in its one, or `{lower-layer}`, `{feature-declined}`, `{not-applicable}`, or a met rollup; `gap` when any target is `{gap}` or a gap rollup; `unproven` otherwise. A `{not-applicable}` target counts as met because an obligation excluded from ze is not one the rollup can owe
4. `rollupDeriver.fill` writes the derived state and, for a gap or an unproven rollup, the sentence naming the first target that made it so onto the row (`Requirement.Derived`, `Requirement.DerivedCause`). `evaluate` raises NO finding for a rollup in any state: a `{gap}` target is a declared state already disclosed under its own id, and an unproven target is already reported under its own id, so a rollup finding would be a second report of one fact (owner decision, 2026-09-15). The renderers print the state and the cause
5. `CoverageRows` leaves a rollup out of `Gated`, `Proven` and `Annotated`; `ProvenShareOf` therefore never sees it
6. `rfcAnnotationBucket` maps the kind to a new `rfcRollupBucket`, labeled "Derived from other rows", neutral tone; `rfcLedgerCoverage` gains a `Rollup` field and its stem-page counter shows the derived state per row; `render.go` prints `{rollup: ...} why` through the generic path with the derived state appended

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Summary text ↔ `Annotation` struct | `parseAnnotation` | No |
| One summary ↔ the corpus | `checkRollupTargets` reads every enrolled summary, which `check.go` already loads for `checkCoverageRatchet` | No |
| `internal/le/rfc` ↔ `internal/le/site` | the kind constant and `AnnotationKinds()`; the site reads the derived state through a new `Requirement.Derived` field `evaluate` fills | No |

### Integration Points
- `annotationKinds` (`rfc.go`) - the registry every kind list derives from; `collect_rfc.go` and `render_ledger.go` pick the new kind up without an edit
- `notes(checkLowerLayerProducer(...))` in `check.go` - `checkRollupTargets` is wired on the next line
- `TestEveryAnnotationKindHasABucket` (`rfccompliance_test.go`) - forces the bucket

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the kind enters through `parseAnnotation` and is judged in `evaluate`, where every other kind is |
| No unintended coupling (components stay isolated) | Yes | the site reads a field on `Requirement`, not the rfc package's internals |
| No duplicated functionality (extends existing, does not recreate) | Yes | derivation reuses the per-id state `evaluate` computes; no second reader of tags |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling over a few hundred rows |
| Registration over hardcoding, outbound | Yes | the kind registers in `annotationKinds`; the bucket registers in `rfcSatisfaction` |
| Registration over hardcoding, inbound | Partly | `rfcAnnotationBucket` is a switch over kinds and gains one arm; `TestEveryAnnotationKindHasABucket` is the guard that makes a forgotten arm red. `annotationBarsATest` gains the kind too, because a tag beside a rollup is the vacuity the kind exists to prevent. The hand-written kind prose in `checkNewSummaries`, `checkUnprovenSupport`, `collect_rfc.go` line 425 and the two contributing pages is edited; each is named in Files to Modify |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `evaluate` already holds per-id proven state (`byRID`, per-id polarity) that `deriveRollup` can read without re-walking the tags | research report over `check_core.go` | derivation needs its own tag walk, a second reader of the tag format | reading `evaluate` before Phase 2 | confirmed with one adjustment: `byRID` is per-id, but the polarity set was computed inside the loop per row, so `newRollupDeriver` takes `byRID` and makes one pass over the rows before the loop (`rollupTargetState`); no second reader of the tags (Phase 3) |
| A-2 | Every summary is loaded before `evaluate` runs, so a cross-summary target can be resolved in one pass | `check.go` loads all summaries for `checkCoverageRatchet` | `checkRollupTargets` needs a loader of its own | reading `check.go` before Phase 2 | confirmed: `Collect` fills `Collected.Requirements` with every summary's rows, enrolled or not, and `checkRollupTargets` takes that slice and `Collected.Enrolled` (Phase 2) |
| A-3 | The public share must not move when a rollup is annotated or when it turns met | owner ruling behind `TestNoAnnotationExceptSinglePolarityMovesThePublishedShare` and this spec's Task | the rollup would count a whole document twice | the corpus test in the TDD plan | confirmed: `TestRollupMovesNeitherShareNorGatedCount` holds `ProvenShareOf` and every `CoverageRow` equal with the rollup lines removed and with one synthetic rollup added, and went red when `CoverageRows` counted the row (Phase 3) |
| A-4 | `RFC4302-5-1`'s target set is: every gated `RFC4302-*` row except `5-1`, `5-2` and the three multicast rows `2.4-2`, `2.4-3`, `2.4-4`, plus the stem `rfc4301` | the RFC's own sentence: "for unicast traffic, and MUST comply with all requirements of the Security Architecture document" | the row derives from the wrong set | owner reads the list in the summary edit | confirmed by derivation, owner read pending: the row carries the 29 gated unicast ids in summary order plus `rfc4301`, derives gap, and its first cause is `RFC4302-2.5-5` (`TestRollupRowsInThisCorpusDeriveAsStated`, Phase 4); the owner still reads the list |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A rollup that names a rollup that names it back loops the derivation | `checkRollupTargets` walk exceeds the rollup count | the walk is bounded by the count of rollups in the corpus and refuses on the first repeat |
| R-2 | A stem target silently widens when rows are added to that summary later | none visible; the rollup just gets stricter | that is the intended behavior and the page says so; the reverse (a row removed) is caught by `checkRetiredRequirements` on the target summary |
| R-3 | A met rollup reads as "proven by ze" on the public page | the row lands in the proven bucket | it has its own bucket and never enters `Proven`; the corpus test pins it |
| R-4 | An author writes `{rollup}` to launder a `{gap}` | a rollup whose targets are all met while the row's own sentence names behavior nothing tests | the validator refuses a rollup with no target, and the review reads the row's sentence against its targets; the site shows the target list beside the row |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing in the daemon: this is the ledger tooling. A wrong derivation misreports two rows on `docs/features/rfc-status.md` and the site |
| How is it reverted? | One commit revert; the two summary lines go back to unannotated |
| Who else touches this path? | `spec-rfc-ledger-gap-count-reader`, `spec-rfcgate-5-should-level` and `spec-rfc-requirement-reattribution` (`plan/pre-release/`) work the same package; none touches the annotation parser |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A summary line carrying `{rollup: ...}` read by `./le rfc check` | → | `parseAnnotation` then `parseRollup` | `TestRollupParsesTargetsAndReason` |
| `./le rfc check` over a corpus whose rollup names an id no summary holds | → | `checkRollupTargets` | `TestRollupRefusesATargetTheCorpusCannotShow` |
| `./le rfc check` over a corpus where one target is `{gap}` | → | `rollupDeriver.fill` inside `evaluate` | `TestRollupIsAGapWhileOneTargetIs` |
| `./le site` rendering a summary with a rollup | → | `rfcAnnotationBucket` | `TestEveryAnnotationKindHasABucket` (existing, goes red the moment the kind exists) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `{rollup: RFC4302-2.4-2, RFC4302-2.4-3; why}` on a gated row | Parses to kind `rollup`, two targets, the reason; `./le rfc check` no longer reports the row as "has no test and no annotation" |
| AC-2 | `{rollup: ; why}`, `{rollup: RFC4302-5-2; why}` on `RFC4302-5-2` itself, or a duplicated target | Refused by `./le rfc check` with a sentence naming the row and ending in the rollup format sentence: the empty list, the empty reason, a malformed target and the duplicate at parse time, the self-reference by `checkRollupTargets`, which is the first reader that holds the row's own id beside its targets |
| AC-3 | A target id no enrolled summary holds, or a stem that is not an enrolled summary | `./le rfc check` refuses it, naming the row and the target |
| AC-4 | Every target proven, lower-layer, feature-declined, not-applicable, single-polarity proven, or a met rollup | The rollup is met: no finding, bucket "Derived from other rows", state met |
| AC-5 | One target carries `{gap}` | The rollup derives gap, renders that state with the cause naming that target (`derived: gap: <target> is annotated {gap}`), and raises no finding; the Remaining cell count is unchanged |
| AC-6 | One target has no test and no annotation | The rollup derives unproven, renders that state with the cause naming that target, and raises no finding; the target's own finding stands |
| AC-7 | Two rollups naming each other | Refused by `checkRollupTargets` before derivation |
| AC-8 | A tag placed on a rollup row | Refused as stale, the way a tag beside `{lower-layer}` is |
| AC-9 | Any corpus | `ProvenShareOf` and `CoverageRows` return the same numbers with every rollup line removed |
| AC-10 | `rfc/short/rfc4302.md` after this spec | `RFC4302-5-2` carries `{rollup: RFC4302-2.4-2, RFC4302-2.4-3, RFC4302-2.4-4; ...}` and is met; `RFC4302-5-1` carries its unicast set plus `rfc4301` and is a gap, its cause `RFC4302-2.5-5` (the first of its two `{gap}` targets in author order; `RFC4302-4-1` is the second); `./le rfc check` names neither row |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRollupParsesTargetsAndReason` | `internal/le/rfc/rollup_test.go` | AC-1: ids and a stem parse, reason kept | |
| `TestRollupRefusesEmptyMalformedAndDuplicateTargets` | `internal/le/rfc/rollup_test.go` | AC-2, the parse-time refusals | |
| `TestRollupRefusesATargetTheCorpusCannotShow` | `internal/le/rfc/rollup_test.go` | AC-3 | |
| `TestRollupIsMetWhenEveryTargetIs` | `internal/le/rfc/rollup_test.go` | AC-4, every met kind in the table | |
| `TestRollupIsAGapWhileOneTargetIs` | `internal/le/rfc/rollup_test.go` | AC-5: the derived state and cause on the row, no finding, and the Remaining count unchanged | |
| `TestRollupIsUnprovenWhileOneTargetIs` | `internal/le/rfc/rollup_test.go` | AC-6: the derived state and cause on the row, no finding | |
| `TestRollupRefusesACycle` | `internal/le/rfc/rollup_test.go` | AC-7, and the self-reference half of AC-2 | |
| `TestRollupCannotStandBesideATag` | `internal/le/rfc/rollup_test.go` | AC-8 | |
| `TestRollupMovesNeitherShareNorGatedCount` | `internal/le/rfc/rollup_corpus_test.go` | AC-9 over this checkout's corpus | |
| `TestRollupRowsInThisCorpusDeriveAsStated` | `internal/le/rfc/rollup_corpus_test.go` | AC-10: 5-2 met, 5-1 gap with cause 2.5-5, neither named by the gate | |
| `TestEveryAnnotationKindHasABucket` | `internal/le/site/rfccompliance_test.go` | existing guard, extended by the new arm | |
| `TestRollupRendersInItsOwnBucket` | `internal/le/site/rfccompliance_test.go` | the bucket label and the target list on the page | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Target count | 1 to the corpus size | every gated id in the corpus | 0 (refused: a rollup over nothing) | N/A, bounded by the corpus |
| Cycle walk depth | 0 to the number of rollups | the rollup count | N/A | one past the count is a cycle, refused |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `./le rfc selftest` | `internal/le/rfc/selftest_core.go` | The in-code corpus the self-test builds gains one rollup row, so the registered action proves the derivation without a checkout | |

### Interop Tests (Scope: protocol)
N-A: tooling, no wire behavior.

## Files to Modify
- `internal/le/rfc/rfc.go` - `AnnotationRollup` constant, `annotationKinds` entry, `Annotation.Targets`, `Requirement.Derived`
- `internal/le/rfc/summary.go` - `parseRollup` and its dispatch in `parseAnnotation`; `rollupFormat` sentence
- `internal/le/rfc/check_core.go` - `checkRollupTargets`, `rollupDeriver` and its `fill`, `deriveRollups` for the renderers (package-private: `NewRenderInput` is its one caller), `annotationBarsATest`
- `internal/le/rfc/check.go` - wire `checkRollupTargets` beside `checkLowerLayerProducer`
- `internal/le/rfc/coverage.go` - `CoverageRows` leaves a rollup out of every count
- `internal/le/rfc/check_ratchets.go` - the kind prose in `checkNewSummaries`
- `internal/le/rfc/check_status.go` - the kind prose in `checkUnprovenSupport`
- `internal/le/rfc/render.go` - the derived state after the reason in the Proof column
- `internal/le/rfc/selftest_core.go` - one rollup row in the self-test corpus
- `internal/le/site/rfccompliance.go` - `rfcRollupBucket`, `rfcRollupLabel`, the `rfcSatisfaction` row, the `rfcAnnotationBucket` arm, the `.rfc-tape-rollup` style
- `internal/le/site/rfcledger.go` - `rfcLedgerCoverage.Rollup` and its switch arm
- `internal/le/testhealth/collect_rfc.go` - the hand-written kind prose
- `rfc/short/rfc4302.md` - the two rows
- `docs/contributing/rfc-conformance-gates.md` - "The rollup annotation" section beside the lower-layer and feature-declined ones, and the rollup sentence removed from the lower-layer rules list is replaced by a pointer
- `docs/contributing/rfc-implementation-guide.md` - §9.7 kind vocabulary
- `ai/skills/ze-rfc.md` - the kind an author may write
- `docs/features/test-health.md` - the kind list

## Files to Create
- `internal/le/rfc/rollup_test.go` - the unit tests above
- `internal/le/rfc/rollup_corpus_test.go` - the two corpus tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No config surface |
| YANG validation constraints | No | No leaf |
| YANG custom validators | No | No leaf |
| CLI commands/flags | No | `./le rfc check`, `index-update` and `./le site` learn the kind through the registry; no new verb |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | No | No leaf |
| Functional test for new RPC/API | No | No RPC |
| Pipe completeness | N-A | No command added |
| Env var registration | No | No environment leaf |
| Doctor check for runtime dependencies | No | No runtime dependency; the tooling reads files in the checkout |
| Prometheus counters/metrics | No | Tooling |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Contributor tooling |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | The existing verbs accept one more kind; `docs/contributing/rfc-conformance-gates.md` is their page |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/contributing/rfc-conformance-gates.md`, `docs/contributing/rfc-implementation-guide.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4302.md`; `docs/features/rfc-status.md` is rendered from it |
| 10 | Test infrastructure changed? | Yes | `docs/features/test-health.md` kind list |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | One more kind in an existing registry |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: `./le spec citation anchors spec plan/pre-release/spec-rfc-ledger-rollup-annotation.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The annotation examples in `rfc-conformance-gates.md` and `rfc-implementation-guide.md` §9.7 are re-read against `parseAnnotation` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the kind exists and reaches the gate
   - Tests: `TestRollupParsesTargetsAndReason`, `TestEveryAnnotationKindHasABucket`
   - Files: `rfc.go`, `summary.go`, `rfccompliance.go`
   - Verify: a summary line with `{rollup: ...}` parses; the site test is red until the bucket arm exists, then green
2. **Phase: validation** -- targets held against the corpus
   - Tests: `TestRollupRefusesEmptyMalformedAndDuplicateTargets`, `TestRollupRefusesATargetTheCorpusCannotShow`, `TestRollupRefusesACycle`, `TestRollupCannotStandBesideATag`
   - Files: `summary.go`, `check_core.go`, `check.go`
   - Verify: each refusal names the row and the target and ends in the format sentence
3. **Phase: derivation** -- the three states, computed in `evaluate`
   - Tests: `TestRollupIsMetWhenEveryTargetIs`, `TestRollupIsAGapWhileOneTargetIs`, `TestRollupIsUnprovenWhileOneTargetIs`, `TestRollupMovesNeitherShareNorGatedCount`
   - Files: `check_core.go`, `coverage.go`, `check_status.go`, `check_ratchets.go`
   - Verify: the Remaining cell and `ProvenShareOf` are unchanged by every rollup in the corpus
4. **Phase: rendering and the two rows** -- the bucket, the ledger page, the self-test, the summaries, the pages
   - Tests: `TestRollupRendersInItsOwnBucket`, `TestRollupRowsInThisCorpusDeriveAsStated`, `./le rfc selftest`, `./le rfc check`
   - Files: `render.go`, `rfcledger.go`, `collect_rfc.go`, `selftest_core.go`, `rfc/short/rfc4302.md`, the four pages
   - Verify: `./le rfc check` names neither `RFC4302-5-1` nor `RFC4302-5-2`; `RFC4302-5-1` renders as a derived gap whose cause names `RFC4302-2.5-5`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every wiring row has a working path |
| Correctness | A `{not-applicable}` target counts as met and a `{gap}` target as gap; a stem target expands to the summary's gated rows minus its rollups; the cycle bound is the rollup count, not a constant |
| Naming | The kind is spelled `rollup` in the summary, the constant, the bucket key and the CSS class; the label is "Derived from other rows" everywhere it is shown |
| Data flow | Derivation reads the state `evaluate` computed; no second tag walk; the site reads `Requirement.Derived` and never re-derives |
| Rule: `ai/rules/rfc-compliance.md` | The rollup enters no numerator and hides no gap: a derived gap still names the constituent row in its finding |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The kind is registered | `grep -n AnnotationRollup internal/le/rfc/rfc.go` shows the constant in `annotationKinds` |
| The two rows are annotated and derive as stated | `./le rfc check` exit 0 for rfc4302, and `TestRollupRowsInThisCorpusDeriveAsStated` green |
| The share is untouched | `TestRollupMovesNeitherShareNorGatedCount` green |
| The pages describe the kind | `grep -n rollup docs/contributing/rfc-conformance-gates.md docs/contributing/rfc-implementation-guide.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The target list is authored text in the checkout; `idRE` and the stem form bound what a target can be, and an unknown target is refused rather than skipped |
| Resource exhaustion | The cycle walk is bounded by the corpus's rollup count |

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
- The five existing kinds each ASSERT something about one row. A rollup asserts nothing; it names other rows. That is why it cannot share their bookkeeping: an asserted kind sits in the gated denominator because the obligation is ze's, and a rollup sits outside it because every obligation it carries is already counted once under its own id.
- Two of the four rollup-shaped rows in the corpus (`RFC9190-2.4-1`, `RFC9190-5.6-4`) point at RFCs with no summary. Refusing an unresolvable target, rather than accepting it as text, is what keeps the kind from becoming the next `{not-applicable}`: 915 judgements nobody can check.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Derived state, computed at check time | An asserted `{met-by: ...}` the author keeps current | An asserted status is a second copy of the constituents' status, and the two drift; the derived one cannot |
| Outside the gated denominator | Inside it and outside the numerator, like `{lower-layer}` | A rollup carries no obligation of its own; counting it would bill a document twice and hold the share below 100% for a fully conformant RFC |
| A stem target for a whole enrolled summary | Listing every id of the other summary | `RFC4302-5-1` binds all of RFC 4301; a list of 90 ids goes stale on the first row that summary gains, and the stem cannot |
| Refuse an unresolvable target | Accept it as prose | The kind's value is that a reader can check it; an unchecked target is a judgement |
| A `{not-applicable}` target counts as met | Counts as gap, or as unproven | The target's exclusion is its own row's claim, presumed wrong and reviewed there; the rollup must not second-guess it |
| Both `RFC9190` rows stay unannotated | Annotate them against RFC 8446 and 7542 by prose | Their targets have no summary; the day one enrols, the rollup can name it |

## Known Limitations
- A rollup cannot name a requirement in a summary that is not enrolled. `RFC9190-2.4-1` and `RFC9190-5.6-4` therefore stay unannotated; their RFCs enrol under `plan/pre-release/spec-followup-rfc-enrollment.md`.

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
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- The sixth kind: `AnnotationRollup`, `Annotation.Targets`, `Requirement.Derived`, `Requirement.DerivedCause`, `Requirement.Rollup()`, `Requirement.DerivedMark()`, `RollupState` (`internal/le/rfc/rfc.go`); `parseRollup`, `isRollupTargetForm`, `rollupFormat` (`summary.go`); `checkRollupTargets`, `leadsBackTo`, `rollupDeriver` with `newRollupDeriver`, `rollupTargetState`, `fill`, `derive`, `answer`, `verdictsOf`, `rollupRowVerdict`, `deriveRollups`, and the `annotationBarsATest` arm (`check_core.go`); wired in `check` (`check.go`) and `NewRenderInput` (`render.go`)
- The denominator: `CoverageRows` skips a rollup (`coverage.go`); the `ProvenShare.Gated` comment names the exception (`provenshare.go`)
- The renderers: `requirementRow` appends `, derived: <mark>` (`render.go`); `rfcRollupBucket`, `rfcRollupLabel`, the `Derived` column of `rfcSatisfaction`, `rfcBucketDerived`, and the tape, key, table, mirror and binding split leaving a Derived bucket out (`internal/le/site/rfccompliance.go`); `rfcLedgerCoverage.Rollup`, `rfcLedgerAnnotation.Targets` and `.Derived` (`rfcledger.go`); the `Derived from other rows` bucket, its role note and the `derived` mark (`rfcdetail.go`); `annotationCounts.total` and `densityMetric` (`internal/le/testhealth/collect_rfc.go`)
- The self-test: `selftestRollupSummary`, `selftestDerivedRollup`, results `coverage/derived-rollup` and `coverage/derived-rollup-unproven` (`selftest_core.go`)
- The two rows in `rfc/short/rfc4302.md`: `RFC4302-5-1` derives gap (cause `RFC4302-2.5-5`), `RFC4302-5-2` derives met
- The kind prose in `checkNewSummaries` (`check_ratchets.go`) and `unprovenChecklist` (`check_status.go`)

### Bugs Found/Fixed
- A `{rollup}` row in a summary nobody enrolled reached `requirementRow` with `Derived == RollupNone`, and `DerivedMark` panics there: `fill` derived enrolled rows only while `RequirementRows` prints every summary. Fixed at closure by recording every rollup's targets and filling every rollup row (`newRollupDeriver`, `fill`); covered by `TestRollupInAnUnenrolledSummaryStillDerives` (`rollup_test.go`)
- `DeriveRollups` was exported with one caller, `NewRenderInput`, in its own package (`./le repository check` ISSUE); unexported to `deriveRollups`

### Documentation Updates
- `docs/contributing/rfc-conformance-gates.md`: "The rollup annotation" section (example, body, the three-state table, the no-finding decision, the five refusals, the denominator, the ledger and the site); the lower-layer rule "It is not a conformance rollup" points at it
- `docs/contributing/rfc-implementation-guide.md` §9.7: the kind and "the last three need the facts"
- `docs/contributing/gh-pages.md`: the Derived bucket, `rollupDeriver.fill` and `rfc.NewRenderInput`, the stem page's bucket and mark
- `ai/skills/ze-rfc.md`: the `{rollup}` paragraph and "any of the six"
- `docs/features/test-health.md`, `test/health/latest.json`, `test/health/quality-baseline.json`: regenerated by `./le test-health update` (tracked generated files)
- `./le doc check verify`: 1 drift, `internal/le/wikicatalog/catalog.go` (the wiki catalog against the live command catalog), not this spec's; `./le doc check links`: the 2 reds journaled in `plan/journal/gate-red-where-nothing-blocks-on-it.md`, neither this spec's

### Deviations from Plan
- A rollup raises no finding (owner decision, 2026-09-15): the spec's Data Flow step 4, AC-5, AC-6 and AC-10 were amended in Phase 4 and `rollupFindings` deleted
- `TestRollupRefusesEmptySelfAndDuplicateTargets` became `TestRollupRefusesEmptyMalformedAndDuplicateTargets`; the self case lives in `TestRollupRefusesACycle` because only `checkRollupTargets` holds the row's own id
- `deriveRollups` is package-private, not exported
- Every rollup row is derived, enrolled or not (the bug above); the spec said "every enrolled {rollup} row"

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Phases 3 and 4 derived the enrolled rollup rows only, the population the gate reads | The renderer prints every summary, and `DerivedMark` panics on an underived rollup | Closure review, tracing `requirementRow` callers (`RequirementRows` walks `ShardStems`, not the enrolled set) | Fill every rollup row; regression test; journal row in `gate-excludes-part-of-its-population.md` |
| approach | Phase 3 wrote a finding for a derived gap or unproven rollup as the spec said | A `{gap}` target is disclosed under its own id and an unproven target reported there, so the finding repeated one fact | Phase 4 saw AC-5/AC-6 and AC-10 could not both hold while two targets are `{gap}`; owner decided | `rollupFindings` deleted, spec amended, page column renamed "What the row publishes" |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A sixth kind whose status the gate derives and never asserts | Done | `rollupDeriver.fill`, `internal/le/rfc/check_core.go` | met / gap / unproven with the first deciding cause |
| A target the corpus cannot show is refused | Done | `checkRollupTargets`, `check_core.go` | id, stem, self, unenrolled, cycle |
| The row changes state the day its constituents do | Done | `deriveRollups` in `NewRenderInput`, `render.go` | derived at every render, nothing authored |
| `RFC4302-5-1` and `-2` no longer "has no test and no annotation" | Done | the `RFC4302-5-1` and `RFC4302-5-2` lines of `rfc/short/rfc4302.md`; `evaluate` arm | `./le rfc check` names neither (grep count 0) |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRollupParsesTargetsAndReason`; `./le rfc check` silent on rfc4302 | |
| AC-2 | Done | `TestRollupRefusesEmptyMalformedAndDuplicateTargets`, `TestRollupRefusesACycle` (self) | every refusal ends in `rollupFormat` |
| AC-3 | Done | `TestRollupRefusesATargetTheCorpusCannotShow` | |
| AC-4 | Done | `TestRollupIsMetWhenEveryTargetIs` | every met kind, a met rollup, a stem |
| AC-5 | Done | `TestRollupIsAGapWhileOneTargetIs` | cause, no finding, Remaining unchanged via `checkGapCountAgreement` |
| AC-6 | Done | `TestRollupIsUnprovenWhileOneTargetIs` | six unproven shapes, target's own finding stands |
| AC-7 | Done | `TestRollupRefusesACycle` | `leadsBackTo` bounded by the rollup count |
| AC-8 | Done | `TestRollupCannotStandBesideATag` | stale, as beside `{lower-layer}` |
| AC-9 | Done | `TestRollupMovesNeitherShareNorGatedCount` | both directions over this corpus |
| AC-10 | Done | `TestRollupRowsInThisCorpusDeriveAsStated` | 5-2 met, 5-1 gap cause `RFC4302-2.5-5`, neither named |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRollupParsesTargetsAndReason` | Done | `internal/le/rfc/rollup_test.go` | |
| `TestRollupRefusesEmptyMalformedAndDuplicateTargets` | Done | `rollup_test.go` | renamed from the spec's first draft |
| `TestRollupRefusesATargetTheCorpusCannotShow` | Done | `rollup_test.go` | |
| `TestRollupIsMetWhenEveryTargetIs` | Done | `rollup_test.go` | |
| `TestRollupIsAGapWhileOneTargetIs` | Done | `rollup_test.go` | |
| `TestRollupIsUnprovenWhileOneTargetIs` | Done | `rollup_test.go` | |
| `TestRollupRefusesACycle` | Done | `rollup_test.go` | |
| `TestRollupCannotStandBesideATag` | Done | `rollup_test.go` | |
| `TestRollupCannotDisplaceAGap` | Done | `rollup_test.go` | added: one line, one disposition |
| `TestRollupInAnUnenrolledSummaryStillDerives` | Done | `rollup_test.go` | added at closure for the bug above |
| `TestRollupMovesNeitherShareNorGatedCount` | Done | `rollup_corpus_test.go` | |
| `TestRollupRowsInThisCorpusDeriveAsStated` | Done | `rollup_corpus_test.go` | |
| `TestEveryAnnotationKindHasABucket` | Done | `internal/le/site/rfccompliance_test.go` | existing, green with the arm |
| `TestRollupRendersInItsOwnBucket` | Done | `rfccompliance_test.go` | |
| `TestADerivedBucketIsOutsideEveryPartitionSum` | Done | `rfccompliance_test.go` | added: the site arithmetic |
| `TestARollupRowIsCountedApartFromTheSplitSum` | Done | `internal/le/testhealth/collect_rfc_rollup_test.go` | added: the test-health split |
| `./le rfc selftest` | Done | `selftest_core.go` | `coverage/derived-rollup`, `coverage/derived-rollup-unproven` pass; the one red is `real-tree/public-check` on other sessions' 15 stale discrimination records |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/rfc/rfc.go`, `summary.go`, `check_core.go`, `check.go`, `coverage.go`, `check_ratchets.go`, `check_status.go`, `render.go`, `selftest_core.go` | Done | as listed |
| `internal/le/rfc/provenshare.go`, `lowerlayer_corpus_test.go` | Changed | not in the plan: the `Gated` comment, and the rollup counterfactual is the line removed (AC-9) |
| `internal/le/site/rfccompliance.go`, `rfcledger.go` | Done | plus `rfcdetail.go` and `rfcdetail_test.go` for the stem page, not in the plan |
| `internal/le/testhealth/collect_rfc.go` | Done | plus `annotationCounts.total`, which would have refused the first real rollup row |
| `rfc/short/rfc4302.md` | Done | the two rows |
| the four pages | Done | plus `docs/contributing/gh-pages.md` |
| `internal/le/rfc/rollup_test.go`, `rollup_corpus_test.go` | Done | created |

### Audit Summary
- **Total items:** 4 requirements, 10 ACs, 17 tests, 7 file groups
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** 4, recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The gate DERIVES the row's status and never asserts it | unit + corpus test | `TestRollupIsMetWhenEveryTargetIs` turns met into unproven by removing RFC2-1-1's tags with no edit to the row; `TestRollupRowsInThisCorpusDeriveAsStated` reads 5-1 gap / 5-2 met through `Collect` + `evaluate` |
| A target the corpus cannot show is refused at check time | unit test | `TestRollupRefusesATargetTheCorpusCannotShow`, `TestRollupRefusesACycle`; each refusal ends in `rollupFormat` |
| The two rfc4302 rows are no longer "has no test and no annotation" | registered action | `./le rfc check`: 15 violations, `grep -c rfc4302` = 0 (log `scratch/close-rfc-check.log`); `rfc/requirements/rfc4302.md` 5-1 Proof ends `derived: gap: RFC4302-2.5-5 is annotated {gap}`, 5-2 `derived: met` |
| The share and the Remaining cell do not move | corpus test | `TestRollupMovesNeitherShareNorGatedCount` (both directions); `TestRollupIsAGapWhileOneTargetIs` holds `checkGapCountAgreement` at one gap and refuses three |
| The site publishes the derived state outside every partition | site test + build | `TestRollupRendersInItsOwnBucket`, `TestADerivedBucketIsOutsideEveryPartitionSum`; `./le site build` exit 0, `../gh-pages/quality/rfc-compliance/rfc4302/index.md` shows `Derived from other rows | 2` and `**derived:** gap: RFC4302-2.5-5 is annotated {gap}` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | every in-scope item is done | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/rfc-ledger-rollup-annotation-b2d741a4-c6a6-4265-8342-fb6ceed4dfd9.md` |
| `review_gate.py check` | clean (`./le spec session review check`: OK, 20 code files, hashes match) |
| Rounds | 2: round 1 over the whole diff found the two ISSUEs below and four NOTEs; round 2 over the fixes found nothing |
| Reviewer lenses used | wiring + removed-behavior audit (evaluate's `byRID`, `rollupFindings`), logic + edge cases (cycle bound, stem expansion, unenrolled rows), security (authored input, bounded walks), style pass over every changed Go file (the one `panic()` is `BUG:` and reachable from no socket), docs drift, `./le repository check`, `./le commit audit` |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | A `{rollup}` row in an unenrolled summary is rendered by `RequirementRows` with no derived state, so `DerivedMark` panics and `./le rfc index-update` stops | `rollupDeriver.fill`, `newRollupDeriver` (`internal/le/rfc/check_core.go`) | record every rollup's targets and fill every rollup row; `TestRollupInAnUnenrolledSummaryStillDerives` |
| 2 | ISSUE | `DeriveRollups` exported with no cross-package non-test caller (`./le repository check`) | `internal/le/rfc/check_core.go` | unexported to `deriveRollups` |
| - | NOTE | `RollupState` comment claimed the states are "ordered by what they cost" while nothing compares them | `rfc.go` | comment names `answer`'s author-order rule |
| - | NOTE | two test doc comments still said a rollup raises "one finding" | `rollup_test.go` | comments say cause on the row, no finding |
| - | NOTE | STE: 2 `may`, 2 `-ing` frozen verbs, 1 "below" for a limit in new comments | `check_core.go`, `rfc.go`, `coverage.go` | rewritten; the run-on count (about 90 sentences over 25 words across the new comments) is left, per the guideline's own "never rewrite a sentence only to satisfy a word count" |
| - | NOTE | `./le commit audit` WEAKENED `internal/le/docvalid/helpshape_schema_test.go` | not this spec's file | reported, not touched |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/rfc/rollup_test.go` | Yes | `git status --porcelain --untracked-files=all` lists `?? internal/le/rfc/rollup_test.go` |
| `internal/le/rfc/rollup_corpus_test.go` | Yes | same listing |
| `internal/le/testhealth/collect_rfc_rollup_test.go` | Yes | same listing |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-9 | the unit tests named above pass | `./le job run label close-final-rfc command go test -count=1 ./internal/le/rfc/ ./internal/le/testhealth/`: only `TestSupportedRowsHaveDerivableScope` and `TestNativeImplementationFixture` red, both journaled before this spec (`plan/journal/hardcoded-count-in-test.md`, HEAD digest seal) |
| AC-10 | 5-2 met, 5-1 gap with cause 2.5-5, neither named | `TestRollupRowsInThisCorpusDeriveAsStated` green in that run; `./le rfc check` grep count 0 for rfc4302 |
| site | bucket and marks | `./le job run label close-final-site command go test -count=1 -run 'Rollup\|Bucket\|Partition\|StemPage\|Vocabulary\|Derived' ./internal/le/site/`: ok; the full site run's only reds are the three `TestTheChanges*` journaled in `hardcoded-count-in-test.md` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le rfc check` over `rfc/short/rfc4302.md` | none (tooling; the registered action is the entry) | Yes: `check` (`check.go`) calls `checkRollupTargets` then `evaluate`, whose `newRollupDeriver(...).fill` writes the state; the run above names neither row |
| `./le rfc index-update` / `./le site build` | none | Yes: `NewRenderInput` (`render.go`) calls `deriveRollups`; `rfc/requirements/rfc4302.md` and the built stem page carry the derived marks |
| `./le rfc selftest` | `selftest_core.go` | Yes: `coverage/derived-rollup` and `coverage/derived-rollup-unproven` pass |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `newRollupDeriver` reads `byRID` once (`check_core.go`); no second tag walk |
| A-2 | confirmed | `check` passes `collected.Requirements` and `collected.Enrolled` to `checkRollupTargets` (`check.go`) |
| A-3 | confirmed | `TestRollupMovesNeitherShareNorGatedCount` |
| A-4 | confirmed by derivation, owner read pending | the `RFC4302-5-1` line of `rfc/short/rfc4302.md` carries the 29 unicast ids plus `rfc4301`; `TestRollupRowsInThisCorpusDeriveAsStated`. The owner reads the target list (question carried in the closure report) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| #6 user guide: the three-state table and five refusals in `rfc-conformance-gates.md` | `rollupTargetState`, `answer`, `rollupRowVerdict`; `parseRollup`, `checkRollupTargets`, `annotationBarsATest` | Yes, each row read against its producer |
| #9 RFC row: `rfc/short/rfc4302.md`; `docs/features/rfc-status.md` derived on demand (untracked, `8ae7424594`) | `./le rfc index-update` output: RFC 4302 `32 gated: 9 proven, 23 annotated, 0 untested` | Yes |
| #10 test infrastructure: `docs/features/test-health.md` | regenerated by `./le test-health update`; `densityMetric` sentence | Yes |
| #16 anchors: `./le repository check` | no stale anchor in this spec's files | Yes |
| #17 examples: the `{rollup}` example lines on both pages | `parseRollup` accepts them; the gates page example IS the `RFC4302-5-2` line of `rfc4302.md` | Yes |
| #3 CLI: no verb added | `grep -n rollup internal/le/rfc/register.go` finds nothing | Yes |
