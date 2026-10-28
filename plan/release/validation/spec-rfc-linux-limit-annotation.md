# Spec: rfc-linux-limit-annotation

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P2 |
| Release reason | The public RFC compliance page and every release profile publish what share of an RFC Ze can reach. Today a fixed sentence says every RFC can reach 100%, which the figure arithmetic already contradicts, and the owner's one exception to full coverage (a primitive Linux cannot provide) has no state in the ledger. Accurate support claims are release validation |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas, 2026-10-28: "Add a 'Linux can't do this' for requirements".

`ai/META-DECISIONS.md` ("Goal and scope") says every RFC Ze implements gets
full coverage, and "The only exception is a basic primitive Linux cannot
provide, named with evidence". `ai/rules/rfc-compliance.md` adds that a Linux
implementation limit is one of the few claims put to the owner ("Ask only when
claiming a Linux implementation limit").

The ledger has no word for that exception. Every unmet requirement is `{gap}`,
which the site reads as "work Ze owes". So the RFC compliance index, the
filtered overview and the legend print a fixed sentence,
`rfcIndexCeiling` in `internal/le/site/rfcfilter.go`: "Every gap is work Ze
owes. No requirement is recorded as impossible on Linux, so every RFC here can
reach 100%."

Goal:

1. A requirement row can carry a new annotation saying Linux cannot provide the
   primitive the requirement needs. The annotation names its evidence, the gate
   checks the evidence exists and is current, and each instance carries the
   owner's approval.
2. Annotating such a row never raises a published figure.
3. Every RFC, every profile tier and every filtered overview publishes a
   computed CEILING: the share of its requirements that remains attainable. The
   fixed sentence is replaced by a computed statement.

Finding made while researching, which this spec also closes (it is the same
sentence): the fixed sentence is false TODAY. `figuresByStem` and `rowCounts`
(`internal/le/rfc/profilecoverage.go`) keep every `{not-applicable}`,
`{lower-layer}` and `{feature-declined}` row in the denominator and never prove
it. The corpus holds 874 `{not-applicable}`, 221 `{lower-layer}` and 68
`{feature-declined}` annotations (grep count over `rfc/short/*.md`, all levels),
so every RFC carrying one cannot reach 100% under its current annotations. RFC
4302's sixteen `{lower-layer}` rows are the documented case. The ceiling below
is defined so the page states this truthfully (Open decision D-3).

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - annotation kinds, how each is checked and counted
  → Decision: "Coverage figures": a `{not-applicable}`, `{lower-layer}` or `{feature-declined}` row "is never proven by Ze, whatever records its tags carry, so annotating a row cannot raise its figure". The new kind takes the same place in the arithmetic.
  → Constraint: "The lower-layer annotation" and "The feature-declined annotation" each make the reason claim a fact the gate checks (a producer, a quote). A reason that is only a judgement is how `{not-applicable}` grew to 915 sites. The new kind must rest on a checkable fact too.
  → Constraint: one checklist line carries ONE disposition; a line carrying `{gap}` and the new kind is refused, never relabeled.
  → Constraint: `checkGapCountAgreement` counts "unmet requirement rows" in the Remaining cell.
- [ ] `docs/architecture/linux-limitations.md` - the Linux Limitations Register
  → Decision: owner order 2026-10-20 makes this page "the one place where Ze records a Linux kernel behavior ... that stops Ze from implementing a feature or an RFC requirement"; "Every other surface (an `rfc/short/` annotation, ...) names the entry id and does not restate the limitation".
  → Constraint: each entry carries `Kind` (kernel or library), `Effect` (`blocks` or `constrains design`), `Status` (`verified` or `unverified`), `Checked`, `Limitation`, `Blocks` (with the RFC requirement ids), `Evidence` (each file and function read, with the line quoted), `Re-check`, `Ze meanwhile`.
  → Constraint: `TestLinuxLimitsCheckedAtShippedVersions` (`internal/appliance/linux_limits_test.go`) fails when a kernel entry's `Checked` differs from `internal/appliance/kernel.version` (7.2.9 today), so the register is current by construction.
- [ ] `docs/contributing/committing.md` - owed trailers and `attest`
  → Decision: `rfc-change-ok` writes `RFC-approval-owed:`; the commit lands, the push is held until `RFC-approved:` or the owner's `Owner-attested: <stable patch-id>: <owner words>` answers it. "Nothing outside the commits records either".
- [ ] `docs/contributing/gh-pages.md` - the site pages built from the ledger (`rfcfilter.go`, `rfcledger.go`)
  → Constraint: page edits ride in the same piece of work as the code (`ai/rules/documentation.md`).
- [ ] `ai/META-DECISIONS.md` ("Test changes") - approvals are recorded through an untracked route plus a commit trailer; "No tracked ledger of approvals is kept in the repository".
  → Decision: owner approval for a Linux limit follows the commit-trailer route, not a tracked approval file (D-2).

**Key insights:**
- The evidence already has a home: the register. The annotation names an `LL-` id and the gate holds the row against that entry, so the fact is declared once (`ai/rules/principles.md`).
- A `constrains design` entry means Ze CAN implement the requirement in another shape, so it is a `{gap}`, never a Linux limit. A `library` entry is fixable by a carried diff against upstream (`ai/META-DECISIONS.md`, "Code and dependencies"), so it is not "Linux cannot" either.
- Approval is a person act, and person acts already live in commit trailers with the push as the gate.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/rfc.go` - `annotationKinds`, the seven kind constants, `Annotation` (fields `Layer`, `Quote`, `Producer`, `Targets`, each read by one kind)
- [ ] `internal/le/rfc/summary.go` - `parseAnnotation` dispatches by kind; `parseFeatureDeclined` demands a double-quoted RFC sentence of at least 24 characters then a `<path>.go::<Symbol>` producer, refuses a `_test.go` producer; `featureDeclinedFormat` is the one format sentence
- [ ] `internal/le/rfc/check_core.go` - `annotationBarsATest` (a tagged test on such a row makes the annotation stale), `rollupTargetState` (per-kind rollup state), `checkFeatureDeclined` (quote found in `rfc/full/<stem>.txt`, producer found in the tree; an RFC with no text is refused)
- [ ] `internal/le/rfc/check.go` - `checkFeatureDeclined` runs in group `CheckAnnotations`
- [ ] `internal/le/rfc/profilecoverage.go` - `Figure{Proven, Total}`, `StemFigures`, `figuresByStem`, `rowCounts` (every annotated row except `{rollup}` is owed; only unannotated and `{single-polarity}` rows can be proven), `Rollup`, `rollupOf` (only walked members contribute)
- [ ] `internal/le/rfc/tree.go` - `TreeRow` JSON (`mandatory-proven`, `mandatory-total`, `optional-proven`, `optional-total`), `TreeRow.Cells`
- [ ] `internal/le/rfc/priority.go` - calls `figuresByStem` and `rollupOf`
- [ ] `internal/le/rfc/provenshare.go` - `ProvenShareOf`, `Implements`
- [ ] `internal/le/rfc/check_status.go` - `checkStatusAgreement` and `checkGapCountAgreement` read `{gap}` and `{partial}` as unmet
- [ ] `internal/le/rfc/check_audit.go` - `verdictClaims` (`not-applicable` verdict legal over `{not-applicable}` and `{feature-declined}`)
- [ ] `internal/le/site/rfcfilter.go` - `rfcIndexCeiling` constant, used in `rfcFilterHTML` (overview), `rfcIndexLegendHTML`, `rfcIndexLegendMirror`; `rfcFigureRowCells` writes `data-mp/mt/op/ot/walked`; the overview script sums them
- [ ] `internal/le/site/rfcledger.go` - coverage counters (`NotApplicable` `out-of-scope`, `LowerLayer` `met-below`, `FeatureDeclined` `optional-not-offered`, `Rollup`, `UnmappedAnnotations`)
- [ ] `internal/le/site/rfccompliance.go` - `rfcSatisfaction` bucket table (Key, Label, Short, Condition, Binds, Derived), `rfcAnnotationBucket` (the one kind to bucket mapping), the `rfcStanding` card table with Tone, Meaning, Rule, and `rfcLedgerCoverage.Bucket`
- [ ] `internal/le/site/rfcprofiles.go` - profile cards, `rfcProfilePercent`, `rfcProfileBarHTML`, `rfcProfileMirror`
- [ ] `internal/le/commit/owed_trailers.go` - `personActs.readLine` reads `Review-owed:`, `RFC-approval-owed:`, `Owner-attested:`
- [ ] `internal/le/commit/attest.go` - `attestationTrailer` refuses a commit carrying no owed trailer
- [ ] `internal/appliance/linux_limits_test.go` - `parseLinuxLimits`, `readLimitField`, `readLimitChecked` parse the register, in a test file

**Behavior to preserve:**
- Every existing figure, share and bucket count is unchanged for a corpus that carries no new annotation, except the ceiling sentence, which becomes computed.
- `TestNoAnnotationExceptSinglePolarityMovesThePublishedShare` (`internal/le/site/rfcdetail_test.go`) keeps holding over the new kind.
- The register stays the one home of every Linux limitation fact.
- Existing JSON keys keep their names and meaning; new keys are added beside them.

**Behavior to change:**
- A new annotation kind, its parser, its check, its owner-approval trailer.
- A ceiling figure per RFC, per profile tier, per filtered overview, in the JSON ledger and the Markdown mirrors.
- The fixed `rfcIndexCeiling` sentence is replaced by a computed statement.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An author writes a requirement row in `rfc/short/<stem>.md` carrying `{linux-limit: LL-<ID>; <reason>}`.
- The register entry `### LL-<ID>` in `docs/architecture/linux-limitations.md`.
- `./le commit create` over a commit that adds or rewords such an annotation.

### Transformation Path
1. Summary parse (`parseAnnotation`, `internal/le/rfc/summary.go`): the annotation becomes an `Annotation` with Kind `linux-limit`, a new field naming the register id, and the reason.
2. Register parse: a non-test parser of `docs/architecture/linux-limitations.md` (moved out of `internal/appliance/linux_limits_test.go`) answers each entry's fields.
3. Gate (`./le rfc check`, group `CheckAnnotations`): `checkLinuxLimit` holds each annotation against its entry (refusals in AC-2 to AC-7).
4. Figures (`figuresByStem`): the row stays owed and unproven; a new per-figure count of unattainable rows feeds the ceiling.
5. Tree and profiles (`TreeRow`, `Rollup`): ceiling counts carried in JSON and summed over walked members.
6. Site (`internal/le/site`): new bucket and counter, ceiling cells, computed ceiling sentence, overview script sums ceiling attributes.
7. Commit (`internal/le/commit`): `./le commit create` writes one `Linux-limit-owed: <RID>: <LL-id>` trailer per added or reworded annotation; the push derivation holds the push until `Owner-attested:` answers it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Summary file to gate | `rfc/short/*.md` row annotation text | No |
| Register page to gate | `docs/architecture/linux-limitations.md` entry fields | No |
| Gate to site | `rfc.Requirement`, `TreeRow`, `Rollup` structs and their JSON | No |
| Commit to push | commit message trailers read by `personActs` | No |

### Integration Points
- `annotationKinds` (`rfc.go`) - the new constant joins the set; `AnnotationKinds()` answers it.
- `rfcAnnotationBucket` (`rfccompliance.go`) - its default arm already counts an unknown kind as unmapped, so a missing arm is visible, not silent.
- `personActs` (`owed_trailers.go`) and `attestationTrailer` (`attest.go`) - one new owed key.
- `TestLinuxLimitsCheckedAtShippedVersions` - keeps its assertions, reads the moved parser.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Ceiling computed only in `figuresByStem`/`rollupOf`; the site reads `TreeRow`/`Rollup` and computes none, as `rfcStemRows` already does for the two figures |
| No unintended coupling (components stay isolated) | Yes | The register parser moves to a package both `internal/appliance` (its test) and `internal/le/rfc` import; `internal/le/rfc` does not import `internal/appliance` |
| No duplicated functionality (extends existing, does not recreate) | Yes | Evidence lives in the register (no second copy in the row); approval reuses owed trailers and `attest` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Tooling, no wire path |
| Registration over hardcoding, outbound | Yes | The kind is one constant in the closed set `annotationKinds`, which is the registry of kinds |
| Registration over hardcoding, inbound | No | Lists searched for the kind names: `annotationKinds` (rfc.go), `parseAnnotation` (summary.go), `annotationBarsATest` and `rollupTargetState` (check_core.go), `rowCounts` (profilecoverage.go), `checkStatusAgreement` and `checkGapCountAgreement` (check_status.go), `verdictClaims` (check_audit.go), `ruledAnnotation` (coverage_ruling.go), `annotatedGap` (gaps.go), the kind lists in the refusal messages of `unprovenChecklist` (check_status.go) and `checkNewSummaries` (check_ratchets.go), `rfcAnnotationBucket`, `rfcSatisfaction`, `rfcStanding` and `rfcLedgerCoverage.Bucket` (rfccompliance.go), the `rfcLedgerCoverage` counters (rfcledger.go), `rfcGlossary` (rfcreader.go), `densityMetric` (health collect_rfc.go). Each must learn the kind. The two refusal-message lists should derive from `AnnotationKinds()`; the per-kind behaviour switches are an existing pattern this spec extends, and D-6 asks whether to fold them into one per-kind trait table first |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The register is the right evidence source and its fields are stable enough to check | Owner order 2026-10-20 quoted on the page; `TestLinuxLimitsCheckedAtShippedVersions` already validates `Kind`, `Effect`, `Status`, `Checked` | Annotation would need its own source citation, a second copy of the fact | Thomas answers D-1 | unvalidated |
| A-2 | Only `Kind: kernel`, `Effect: blocks`, `Status: verified` entries prove "Linux cannot" | Register "What belongs here": `constrains design` means "Ze can implement the feature"; a library limit is fixable by a carried diff (`ai/META-DECISIONS.md`) | A library or unverified limit would excuse work Ze owes | Thomas answers D-1 | unvalidated |
| A-3 | The share-invariance test iterates every kind, so it covers the new one without edit | Test name and the gates page ("for every kind but `{single-polarity}`") | The new kind could move the share unseen | Read `rfcdetail_test.go` in phase 1 | unvalidated |
| A-4 | `attest` answers any owed trailer key once the key is registered in `personActs` | `committing.md`: `Owner-attested` "answers every person act the named commit owes"; `attestationTrailer` lists accepted keys in its refusal | A second edit in `attest.go` is owed | Read `attestationTrailer` in phase 4 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The kind becomes a new blanket exemption, as `{not-applicable}` did | Annotation count grows without register entries growing | The gate demands a verified `blocks` kernel entry whose `Blocks` names the row, and each instance owes the owner's attestation before push |
| R-2 | A kernel bump lifts a limit and the row keeps claiming it | Register re-check at the new version flips the entry | The gate refuses an annotation whose entry is gone, unverified or `constrains design`, so lifting the entry turns the row red in the same commit |
| R-3 | An agent forges `Owner-attested:` words | An attest commit Thomas did not order | The push itself needs Thomas's order (`./le commit push "<owner authorisation>"`), the existing control for every owed act; D-2 offers a stronger interactive-terminal route |
| R-4 | The ceiling under D-3 option B reads as an excuse, because `{not-applicable}` rows are presumed wrong | Reader quotes the ceiling as the target | The computed statement names each kind's share separately and says a `{not-applicable}` or `{feature-declined}` row is a classification that can be revisited |
| R-5 | The overview script and the Go rollup disagree on ceiling sums | Profile page and filtered overview show different ceilings for the same set | AC-12 compares the script's inputs with `rollupOf` output |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Published RFC figures and the compliance page; no shipped `ze` behavior |
| How is it reverted? | Single commit revert; no row carries the kind today, so nothing in `rfc/short/` depends on it |
| Who else touches this path? | Any session editing `internal/le/rfc`, `internal/le/site/rfc*.go`, `rfc/short/`, or the register |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` over a fixture tree with a `{linux-limit}` row and a register entry | → | `parseAnnotation`, `checkLinuxLimit` | `TestCheckLinuxLimitReadsTheRegisterAndTheRow` |
| `./le rfc tree` / priority report over the same fixture | → | `figuresByStem`, `rollupOf` ceiling | `TestALinuxLimitLowersTheCeilingAndNotTheFigure` |
| site build over the same fixture | → | computed ceiling statement, ledger counter, overview attributes | `TestTheIndexStatesTheComputedCeiling` |
| `./le commit create` over a staged summary adding the annotation | → | owed trailer writer, push derivation | `TestALinuxLimitCommitOwesTheOwnersAttestation` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Row carries `{linux-limit: LL-XFRM-EXAMPLE; reason}` with a matching verified kernel `blocks` entry naming the row id | `./le rfc check` raises no finding for it; the row parses with Kind `linux-limit` and the entry id |
| AC-2 | Annotation body names no `LL-` id, or nothing after the `;` | Parse refusal naming the row and the one format sentence |
| AC-3 | Annotation names an `LL-` id the register does not hold | Refusal naming the id and the register path |
| AC-4 | Entry `Effect` is `constrains design` | Refusal saying Ze can implement it in another shape, so the row is a `{gap}` |
| AC-5 | Entry `Status` is `unverified` | Refusal naming the entry's `Re-check` as what settles it |
| AC-6 | Entry `Kind` is `library` | Refusal saying a library limit is fixed by a carried diff, so the row is a `{gap}` |
| AC-7 | Entry `Blocks` field does not name the row's requirement id, or its `Evidence` field is empty | Refusal naming both sides, so the row and the entry cannot disagree |
| AC-8 | Row carries `{linux-limit}` and a tagged test | Stale-annotation refusal, as for `{gap}` |
| AC-9 | Row carries `{gap}` and `{linux-limit}` | Refused: one line, one disposition |
| AC-10 | A corpus, then the same corpus with one `{gap}` row re-annotated `{linux-limit}` | Mandatory and optional Proven and Total identical; proof share identical |
| AC-11 | RFC with T owed gated rows, U of them never attainable under D-3 | Its ceiling is (T - U) of T; JSON carries the attainable count beside proven and total; an unwalked RFC shows no ceiling |
| AC-12 | Profile tier and filtered overview over the same RFCs | Both show the sum of walked members' attainable counts over their totals, equal to `rollupOf` |
| AC-13 | Corpus with no unattainable row | Index, legend, overview and mirror state that every RFC shown can reach 100%, from the computed count, not a constant |
| AC-14 | Corpus with unattainable rows | The statement names how many requirements, split by reason (Linux cannot, and the D-3 kinds), and the lowest ceiling |
| AC-15 | Commit adds or rewords a `{linux-limit}` annotation | Message carries one `Linux-limit-owed: <RID>: <LL-id>` trailer per such row; the commit lands; the push is refused until an `Owner-attested:` for that commit lands |
| AC-16 | Commit touches the summary without changing an existing `{linux-limit}` annotation | No trailer owed |
| AC-17 | Public row says `Supported` over a summary holding a `{linux-limit}` row | `checkStatusAgreement` refuses, as for `{gap}`; the Remaining cell counts the row among unmet MUSTs (D-4) |
| AC-18 | Audit verdict over a `{linux-limit}` row | `unimplemented` accepted; `not-applicable` refused, because the obligation binds |
| AC-19 | Site build over a corpus with the kind | Ledger counter `linux-cannot` counts it; bucket `linux_limit` labelled "Linux cannot do this"; `UnmappedAnnotations` stays zero |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestALinuxLimitAnnotationCarriesItsRegisterID` | `internal/le/rfc/linux_limit_test.go` | AC-1 parse | |
| `TestALinuxLimitReasonIsRefusedWithoutBothFacts` | `internal/le/rfc/linux_limit_test.go` | AC-2 | |
| `TestCheckLinuxLimitReadsTheRegisterAndTheRow` | `internal/le/rfc/linux_limit_test.go` | AC-1, AC-3 to AC-7, table over entry shapes | |
| `TestALinuxLimitAnnotationIsStaleBesideATaggedTest` | `internal/le/rfc/linux_limit_test.go` | AC-8 | |
| `TestALinuxLimitAnnotationCannotDisplaceAGap` | `internal/le/rfc/linux_limit_test.go` | AC-9 | |
| `TestALinuxLimitRequirementIsAnnotatedAndNotProven` | `internal/le/rfc/linux_limit_test.go` | AC-10 at the rfc layer | |
| `TestALinuxLimitLowersTheCeilingAndNotTheFigure` | `internal/le/rfc/profilecoverage_test.go` | AC-10, AC-11 | |
| `TestTheRollupCeilingSumsWalkedMembersOnly` | `internal/le/rfc/profilecoverage_test.go` | AC-11, AC-12 | |
| `TestALinuxLimitRowCountsAsUnmetInTheStatusRow` | `internal/le/rfc/check_status_test.go` | AC-17 | |
| `TestUnimplementedVerdictStandsOverLinuxLimit` | `internal/le/rfc/check_audit_test.go` | AC-18 | |
| `TestParseLinuxLimitsForm` (moved with the parser) | new register package test | register parse unchanged by the move | |
| `TestTheIndexStatesTheComputedCeiling` | `internal/le/site/rfcfilter_test.go` | AC-13, AC-14, legend, overview, mirror | |
| `TestTheOverviewSumsTheCeilingTheRollupSums` | `internal/le/site/rfcfilter_test.go` | AC-12 data attributes | |
| `TestTheLedgerCountsLinuxLimitRows` | `internal/le/site/rfcledger_test.go` | AC-19 | |
| `TestNoAnnotationExceptSinglePolarityMovesThePublishedShare` (existing) | `internal/le/site/rfcdetail_test.go` | AC-10 over the real corpus | |
| `TestALinuxLimitCommitOwesTheOwnersAttestation` | `internal/le/commit/owed_test.go` | AC-15 | |
| `TestAnUnchangedLinuxLimitOwesNothing` | `internal/le/commit/owed_test.go` | AC-16 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| attainable count | 0 to Total | 0 and Total | N/A (a count) | N/A (cannot exceed Total by construction; a test asserts it) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | - | Tooling with no `ze` entry point; the wiring tests drive `./le rfc check`, the tree report, the site build and `./le commit create` through their own entry functions over fixture trees, as the `{feature-declined}` tests do | |

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer.

## Files to Modify
- `internal/le/rfc/rfc.go` - new kind constant in `annotationKinds`; new `Annotation` field for the register id; kind comment
- `internal/le/rfc/summary.go` - parser and its one format sentence
- `internal/le/rfc/check_core.go` - `checkLinuxLimit`; `annotationBarsATest`; `rollupTargetState` (a linux-limit target is a gap)
- `internal/le/rfc/check.go` - run `checkLinuxLimit` in `CheckAnnotations`
- `internal/le/rfc/profilecoverage.go` - attainable count on `Figure`, `StemFigures`, `Rollup`; `figuresByStem`, `rollupOf`, `String`
- `internal/le/rfc/tree.go` - `TreeRow` attainable JSON fields, `Cells`, `Text`
- `internal/le/rfc/check_status.go` - status agreement and Remaining count read the kind; kind list in the message derived from `AnnotationKinds()`
- `internal/le/rfc/check_ratchets.go` - kind list in the message derived from `AnnotationKinds()`
- `internal/le/rfc/check_audit.go` - `verdictClaims`
- `internal/le/rfc/coverage_ruling.go`, `internal/le/rfc/gaps.go` - read the kind where they read `{gap}` as unmet, per each function's contract
- `internal/le/site/rfccompliance.go` - bucket, card, count switch
- `internal/le/site/rfcledger.go` - `LinuxLimit` counter, JSON `linux-cannot`
- `internal/le/site/rfcfilter.go` - delete `rfcIndexCeiling`; computed statement in overview, legend, mirror; `data-ma`/`data-oa` attributes; overview script ceiling
- `internal/le/site/rfcprofiles.go` - ceiling on cards, rows and mirror
- `internal/le/site/rfcreader.go` - glossary entry
- `internal/le/test/health/collect_rfc.go` - kind count
- `internal/le/commit/owed_trailers.go`, `internal/le/commit/actions.go`, `internal/le/commit/attest.go`, `internal/le/commit/prepare.go` - `Linux-limit-owed:` trailer
- `internal/appliance/linux_limits_test.go` - use the moved parser; delete the local one (no layering)
- `docs/contributing/rfc-conformance-gates.md` - new section "The linux-limit annotation", "Coverage figures" ceiling, guard table row
- `docs/architecture/linux-limitations.md` - the annotation names an entry; which entries qualify
- `docs/contributing/committing.md` - the new owed trailer row
- `docs/architecture/testing/verify-freshness-scope.md` - the new owed act in its table
- `docs/architecture/testing/test-health.md` - the kind in the RFC annotation counts
- `docs/contributing/gh-pages.md` - ceiling on the compliance pages
- `ai/skills/ze-rfc.md` - the kind in the annotation vocabulary and when to use it

## Files to Create
- `internal/le/rfc/linux_limit_test.go` - unit tests above
- A non-test register parser package (name per D-5), with the moved `parseLinuxLimits` and its test

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Tooling, no config |
| YANG validation constraints | N-A | No YANG |
| YANG custom validators | N-A | No YANG |
| CLI commands/flags | No | No new command under D-2 option A; option B would add `./le rfc linux-limit approve` |
| CLI grammar (keyword before value) | N-A | No new command under the recommendation |
| Editor autocomplete | N-A | No YANG |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | N-A | No `ze` command output |
| Env var registration | N-A | None |
| Doctor check for runtime dependencies | N-A | No runtime dependency |
| Prometheus counters/metrics | N-A | None |
| BGP family surface | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Developer tooling and the public ledger page, not a `ze` feature |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/contributing/rfc-conformance-gates.md`; the rendered `docs/features/rfc-status.md` regenerates through `./le rfc update` |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/rfc-conformance-gates.md`, `docs/architecture/linux-limitations.md` (parser moves) |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/release/validation/spec-rfc-linux-limit-annotation.md` at implementation; known today: `rfc-conformance-gates.md` (profilecoverage.go, meta.go), `gh-pages.md` (rfcfilter.go, rfcledger.go), `linux-limitations.md` and `appliance/kernel-profiles.md` (linux_limits_test.go), `committing.md` (commit package). Declared by the changed files' `// Design:` headers: `docs/architecture/testing/verify-freshness-scope.md` (affected: the owed-act table gains `Linux-limit-owed:`), `docs/architecture/testing/test-health.md` (affected: the RFC kind counts in `collect_rfc.go` gain the kind), `website/AI.md` (affected if it lists the compliance page's buckets or the ceiling sentence; checked at implementation), `docs/architecture/core-design.md` (unaffected: no core design changes, tooling only) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The annotation examples in `rfc-conformance-gates.md` and `ai/skills/ze-rfc.md` |

## Implementation Steps

1. **Phase: Wiring** -- kind constant, parser stub, `checkLinuxLimit` stub registered in `CheckAnnotations`, register parser moved to its non-test package with `TestParseLinuxLimitsForm` and `TestLinuxLimitsCheckedAtShippedVersions` green on it
   - Tests: `TestCheckLinuxLimitReadsTheRegisterAndTheRow` (fails: stub)
   - Files: rfc.go, summary.go, check_core.go, check.go, register package, linux_limits_test.go
   - Verify: the check runs over the fixture and fails for the stub reason
2. **Phase: Gate** -- parse and check refusals AC-2 to AC-9; status, audit, rollup readers AC-17, AC-18
   - Tests: the `linux_limit_test.go` set, `TestALinuxLimitRowCountsAsUnmetInTheStatusRow`, `TestUnimplementedVerdictStandsOverLinuxLimit`
   - Files: summary.go, check_core.go, check_status.go, check_audit.go, coverage_ruling.go, gaps.go
   - Verify: tests fail, implement, tests pass
3. **Phase: Figures** -- attainable counts in `figuresByStem`, `rollupOf`, `TreeRow`, text output
   - Tests: `TestALinuxLimitLowersTheCeilingAndNotTheFigure`, `TestTheRollupCeilingSumsWalkedMembersOnly`
   - Files: profilecoverage.go, tree.go
   - Verify: tests fail, implement, tests pass
4. **Phase: Approval** -- `Linux-limit-owed:` trailer written by `./le commit create`, read by `personActs`, answered by `attest`
   - Tests: `TestALinuxLimitCommitOwesTheOwnersAttestation`, `TestAnUnchangedLinuxLimitOwesNothing`
   - Files: owed_trailers.go, actions.go, attest.go, prepare.go
   - Verify: tests fail, implement, tests pass
5. **Phase: Site** -- bucket, counter, card, ceiling cells, computed statement, overview script, mirrors
   - Tests: `TestTheIndexStatesTheComputedCeiling`, `TestTheOverviewSumsTheCeilingTheRollupSums`, `TestTheLedgerCountsLinuxLimitRows`, existing share-invariance test
   - Files: rfccompliance.go, rfcledger.go, rfcfilter.go, rfcprofiles.go, rfcreader.go, collect_rfc.go
   - Verify: tests fail, implement, tests pass
6. **Phase: Docs** -- each page listed in Files to Modify, in the same commit as the code it describes

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | Annotating a row never changes Proven or Total (AC-10); the ceiling never exceeds 100% nor falls below the figure |
| Correctness | An unwalked RFC shows no ceiling, as it shows no figure |
| Naming | JSON keys describe state and never repeat the kind word in `rfcledger.go` (`TestTheSiteReadsItsRFCVocabularyFromThePackage`) |
| Data flow | Ceiling computed once in `internal/le/rfc`; site and script only sum |
| Rule: principles (declared once) | The row names the register id and restates no kernel evidence |
| Rule: no-layering | The test-local register parser is deleted, not kept beside the shared one |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Kind in the vocabulary | an `AnnotationKinds()` test lists it |
| Fixed sentence gone | grep `rfcIndexCeiling` in `internal/le/site` returns nothing |
| Ceiling in JSON | `./le rfc tree` JSON rows carry the attainable fields |
| Trailer | owed tests above, and the `docs/contributing/committing.md` row |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Approval forgery | Only the owner-ordered push discharges the trailer; no gate accepts approval text inside the summary |
| Fail-open parse | A malformed register entry is a refusal of every annotation naming it, never a pass |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The register already has an `Effect` field that separates "Linux cannot" (`blocks`) from "Linux forces a shape" (`constrains design`). Every row that cites a register entry today cites a `constrains design` entry or an unverified one, so the register itself says none of today's Linux-blaming gaps is a Linux limit yet.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Evidence is a register id, checked against the entry | A kernel source path and commit inside the annotation, as `{feature-declined}` carries its quote | The owner order makes the register the one place; a citation in the row is a second copy that drifts, while the register is already pinned to the shipped kernel by test |
| Only verified kernel `blocks` entries qualify | Any entry | `constrains design` means Ze can implement it; a library is patched by a carried diff; unverified proves nothing |
| The row stays owed and unproven | Remove it from the denominator | Owner ruling 2026-09-02: annotating may not raise a figure |
| Ceiling computed in `internal/le/rfc` | Computed in the site | `rfcStemRows` already makes the site compute no figure of its own |
| Approval through an owed trailer and `Owner-attested:` | Approval text in the annotation; a tracked approval file | Text in the row is forgeable by any author; `ai/META-DECISIONS.md` keeps no tracked ledger of approvals |

## Open decisions for Thomas

| # | Decision | Options | Recommendation |
|---|----------|---------|----------------|
| D-1 | Name, syntax and evidence | (a) `{linux-limit: LL-<ID>; <why no Ze code substitutes>}`, evidence is the register entry, which must be `Kind: kernel`, `Effect: blocks`, `Status: verified`, name the row id in `Blocks` and carry a non-empty `Evidence`; (b) `{linux-cannot: "<kernel path>@v<version>: <quoted line>"; ...}` carrying its own citation; (c) (a) but also accept `library` entries | (a). The register is your declared single home for these facts, it is already tied to the shipped kernel (7.2.9) by test, and a library limit is fixed by a carried diff |
| D-2 | How each instance records your approval | (A) `./le commit create` writes `Linux-limit-owed: <RID>: <LL-id>` for each added or reworded annotation; the commit lands, the push is held until your `Owner-attested:` answers it; (B) a command `./le rfc linux-limit approve <RID>` that refuses without an interactive terminal (the `human.Check` route the full verification uses) and writes a record `./le rfc check` reads; (C) approval words inside the annotation | (A). It reuses the person-act machinery whole, and matches "no tracked ledger of approvals". (B) is stronger against an agent but adds a command and a tracked file; (C) is forgeable |
| D-3 | What the ceiling subtracts | (A) only `{linux-limit}` rows; (B) every row the figure can never prove under its current annotation: `{linux-limit}`, `{lower-layer}`, `{feature-declined}`, `{not-applicable}`, each shown as its own share | (B). Under `figuresByStem` those three kinds are never proven, so (A) would keep printing "can reach 100%" for RFC 4302 and every other RFC carrying them, which is false today. The statement names each kind's share so the Linux part stands alone |
| D-4 | How the status row treats the kind | (A) as unmet: a `Supported` row is refused and the Remaining cell counts it with gaps and partials; (B) its own phrase in the Remaining cell | (A). The Remaining count already means "unmet requirement rows", and a requirement Linux stops Ze meeting is unmet |
| D-5 | Where the register parser lives | (A) `internal/le/linuxlimit` (le-only); (B) a package under `internal/core/` | (A). Only le tooling and an appliance test read it |
| D-6 | The per-kind switches (about a dozen across `internal/le/rfc` and `internal/le/site`) | (A) add the kind to each, as `{feature-declined}` did; (B) first fold them into one per-kind trait table in `rfc.go` that every reader derives from | (A) in this spec, with (B) as its own engineering spec if you want it: (B) is the principled shape but restructures every existing kind |
| D-7 | Tone of the new bucket on the compliance cards | neutral (as `{lower-layer}`), or bad (as gaps) | neutral, with its own note: the obligation binds and is unmet, and you have accepted that Ze cannot meet it on Linux |

## Known Limitations
- No reverse check that a `blocks` entry's listed requirement ids carry the annotation: annotating needs your approval per instance, so the gate cannot force it. `./le rfc check` can list such rows as candidates in its report; that is in scope only if D-2 lands as A or B.
- No row is annotated by this spec. Candidates are evidence only (below).

### Candidates found in `rfc/short/` (evidence only, nothing annotated)

| Row | Register entry cited | Entry state | Qualifies under D-1 (a)? |
|-----|---------------------|-------------|---------------------------|
| RFC4301-6.2-3 | LL-XFRM-ICMP-RETURN-SA | kernel, blocks, unverified | No: unverified, and the entry says the kernel may already do it |
| RFC4301-4.4.1.1-2 | LL-NETLINK-XFRM-PORT-MASK | library, constrains design | No |
| RFC5798-8.2.2-2, RFC5798-8.2.2-3, RFC9568-8.2.2-2, RFC9568-8.2.2-3 | LL-NS-SOURCE-MAC | kernel, constrains design | No: Ze can install the device that makes it right |
| RFC3768-8.3-1 | LL-PROXY-ARP-MAC | kernel, constrains design | No |
| RFC3209-2.6-1 to 2.6-3 | none (the reason cites `net/mpls/af_mpls.c`) | LL-MPLS-FORWARD-NO-PMTU is constrains design | No |
| no row | LL-XFRM-STATE-ONE-SELECTOR | kernel, blocks, verified | Would qualify, but no requirement row names it: RFC 7296 Section 2.9 lets a peer propose several pairs, and no gated `RFC7296-2.9-*` row is a gap |

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
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits (`/ze-close` step 6f). The full verification is Thomas's release check: no agent runs it, and one that is owed is handed to him as its exact command. A red there gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior (N-A: tooling, reason in the Functional Tests table)
- [ ] Interop tests for protocol features (N-A: tooling)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/validation/spec-rfc-linux-limit-annotation.md` only, in the same `./le commit create` script
