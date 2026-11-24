# Spec: rfc-external-program-verification

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P2 |
| Release reason | Release validation and accurate RFC ledger claims: let obligations fulfilled by an external helper program Ze integrates with be demonstrated end-to-end and counted as External verified / External testing gap apart from Ze proven and OS verified, starting with RFC 9582 ROA validation via a real Relying Party validator feeding Ze's RPKI/RTR pipeline. |
| RFC gaps | RFC9582-3-1, RFC9582-4.1-1, RFC9582-4.3.1-1, RFC9582-4.3.1-2, RFC9582-4.3.1-3, RFC9582-4.3.1-4, RFC9582-4.3.2.2-1, RFC9582-5-1, RFC9582-5-2, RFC9582-5-3, RFC9582-5-4, RFC9582-5-5, RFC9582-5-6, RFC9582-5-7, RFC9582-6-1, RFC9582-6-2 |
| Depends | - |
| Phase | ready |
| Handoff | - |
| Updated | 2026-11-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Add a dedicated **external-program verification** lane to Ze's RFC conformance ledger (`internal/le/rfc/`), health collector (`internal/le/test/health/`), static site renderers (`internal/le/site/`, `website/`), and UI fixtures (`internal/test/fixture/`), parallel to the existing `{lower-layer}` → `OS verified` / `OS testing gap` lane used for the Linux kernel.

Today Ze has only two ways to represent an RFC whose core processing lives in an external user-space program (such as a Relying Party RPKI ROA validator like `rpki-client` or `Routinator`, optionally feeding StayRTR over RTR into `bgp-rpki`):
1. Mark the document `Implementation: third-party`, which removes it from Ze's conformance counts entirely and provides no structured proof tracking of the combined Ze + external-program behavior; or
2. Mark the document `Implementation: ze` with `{gap}` rows, which inflates `not built` even when Ze intentionally delegates that layer to an external program and can prove the combined system end-to-end.

This spec delivers four cohesive parts:
1. **Coverage annotation and gate checks:** Add `{external-program: <program>, <artifact-ref>; <path>.go::<Symbol> ...}` to `internal/le/rfc/`. The parser and `./le rfc check` verify both facts on every run: a pinned external-program artifact in the repository (`interop:<path>`, rejecting `..`, missing files, and `:latest` tags) and a real Ze integration producer (`<path>.go::<Symbol>`, rejecting `_test.go` and absent/renamed symbols). Documents using this lane declare `Implementation: mixed` (already included in `Implements(meta)` — no 5th `Implementation` kind).
2. **Dedicated sections and rendering:** Add `ExternalVerified` / `ExternalGap` (and `OptionalExternalVerified` / `OptionalExternalGap`) beside `OSVerified` / `OSGap` across `StemFigures`, `MustSections`, `ShouldSections`, `Rollup`, `TreeRow`, `ImplementedSections`, `data/rfc/*.json`, `data/rfc/index.json`, `docs/features/rfc-status.md`, `/quality/rfc-compliance/`, `/quality/rfc-map/`, and `/quality/health/`. Keep `External verified` strictly separate from `Ze proven` and `OS verified` on every bar and Evidence cell (`external N`), while counting it in `MustProven()` / `ProvenPercent()` and gating `Rollup.mandatoryComplete()` on `ExternalGap == 0`.
3. **Non-vacuous two-polarity proof rule:** A gated `{external-program}` row transitions from `External testing gap` to `External verified` only when tagged tests cover **both** `positive` and `negative` polarities, a verifying discrimination record breaks the annotation's named Ze integration producer (`req.Annotation.Producer`), and the negative polarity is proved non-vacuous (via an `external-bypass` break route or producer break showing the negative assertion fails when the external validator's enforcement is bypassed).
4. **First pilot — RFC 9582 (ROA Profile and Validation):** Add a pinned offline Docker interop image and BGP interop scenario using a real Relying Party ROA validator (`rpki-client` or `Routinator`, optionally feeding StayRTR) connected to Ze's `bgp-rpki` RTR client, with a locally generated synthetic trust anchor plus valid and deliberately malformed/forged `.roa` objects covering RFC 9582's 16 MUST rows, while keeping existing StayRTR RTR/ROV scenarios unchanged.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform and registration boundaries
  → Constraint: every new annotation kind, bar segment, and evidence kind must derive from one declaration (`annotationKinds`, `rfcBars`, `rfcSegmentMeanings`) rather than duplicating unvalidated parallel lists
- [ ] `ai/doors/le-tooling.md` - `le` command, hook, test, and interop boundaries
  → Constraint: `internal/le` is never imported by shipped `ze`; every `le` action stays callable as Go and renders through the shared renderer
- [ ] `ai/doors/bgp-plugins.md` - `bgp-rpki` plugin boundary for the RFC 9582 pilot
  → Constraint: `bgp-rpki` consumes validated VRP payloads over RTR (`internal/component/bgp/plugins/rpki/rtr_pdu.go::parsePrefixPDU`) and runs as an edge plugin
- [ ] `docs/contributing/rfc-conformance-gates.md` - existing `{lower-layer}`, `OS verified`, `SectionsOf`, ratchets, and discrimination records
  → Decision: `SectionsOf` (`internal/le/rfc/sectioncounts.go`) is the ONE split of per-RFC figures into bar sections read by `le rfc tree`, `le rfc next`, `data/rfc/`, homepage, and `/quality/health/`
  → Constraint: annotating a row must never raise a proven figure without verifying discrimination records matching the annotation's named producer; `MustSections.Proven()`, `StemFigures.MustProven()`, `TreeRow.UnmarshalJSON`, `Rollup.UnmarshalJSON`, and `Rollup.mandatoryComplete()` must stay consistent across every reader
- [ ] `docs/architecture/site-facts.md` - RFC completion data files (`data/rfc/*.json`, `data/rfc/index.json`)
  → Constraint: `rfcBars` (`internal/le/site/rfcdata.go`) is the single Go declaration published in `data/rfc/index.json` that `rfc-figures.js` reads; `rfcRegisterOpenSections` (`rfcfallback.go`) and `rfc-figures.js::registerOpenSections` must list the exact same open MUST fields
- [ ] `docs/architecture/testing/interop.md` - Docker interop suite catalog, image pinning, and vacuity rules
  → Decision: each Docker interop suite checks only its required `KernelCapabilities`; images are pinned by version/digest and never depend on live internet repositories during deterministic scenarios
  → Constraint: a scenario directory must be registered in its suite's checker catalog (`Discover` refuses an unregistered scenario)
- [ ] `website/AI.md` - website asset, mirror, glossary, and `rfc-map` rules
  → Constraint: every bar segment declared in `rfcBars` must have a glossary meaning in `rfcSegmentMeanings` (`rfcreader.go`), WCAG 4.5:1 CSS rules in `website/assets/css/30-components.css` (`TestRFCSegmentColoursAreReadableInBothThemes`), and matching field validation/rendering in `website/assets/js/rfc-map.js` and `website/assets/rfc-map.css`

### RFC Summaries
- [ ] `rfc/short/rfc9582.md` - pilot RFC (16 MUST rows currently `{gap}` under `Implementation: ze`)
  → Constraint: RFC 9582 obliges Relying Party validation of CMS-signed ROA objects (OID, version 0, AFI, no IPv4-mapped IPv6, maxLength, RFC 6488, RFC 3779 EE cert containment, no inherit, no AS ext); StayRTR alone only serves pre-validated JSON VRPs and cannot prove RFC 9582

**Key insights:**
- `internal/le/rfc/` already separates "who performs the row" (`OSMet` → `OS verified` / `OS testing gap` under `kindOS`) from "Ze's own Go proves the row" (`ZeProven` under interop/functional/editor/unit) through `AnnotationTraits` (`rfc.go`), `StemFigures` (`profilecoverage.go`), and `Sections` (`sectioncounts.go`).
- Every published JSON/CLI/website surface reads `Sections` (`sectioncounts.go`) and `rfcBars` (`internal/le/site/rfcdata.go`), EXCEPT four explicit lists that also enumerate segment/open keys and MUST move together: `rfcRegisterOpenSections` (`rfcfallback.go`), `registerOpenSections` (`website/assets/js/rfc-figures.js`), `proofSegments`/`fields` (`website/assets/js/rfc-map.js`), and `leTreeMustTarget`/`leRFCEvidenceKinds` (`internal/test/fixture/ui_fixture_le_help_rfc_tree_sections.go` and `ui_fixture_le_rfc_derived_proof.go`).
- For `{lower-layer}`, the evidence ratchet checks a real artifact in the checkout (`linux:CONFIG_*`, `go:<pkg>`, `vendor/<mod>`) plus a real Ze producer (`<path>.go::<Symbol>`). `{external-program}` uses the same two-fact defense (`interop:<Dockerfile>` + `<path>.go::<Symbol>`) with zero legacy backlog, plus two-polarity coverage and non-vacuous negative discrimination.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/rfc.go` - declares `annotationKinds` and `AnnotationTraits` (`OSMet`, `CeilingExcluded`, `Bucket`, `RollupState`). No external-program annotation kind or trait exists.
- [ ] `internal/le/rfc/summary.go` - `parseLowerLayer` parses `{lower-layer: <layer>; <reason>}`, extracts `<path>.go::<Symbol>` via `producerRE` and `LayerPath` via `layerPathRE`, and rejects `_test.go` producers.
- [ ] `internal/le/rfc/check_core.go` - `evaluate` skips stale-tag refusal for `traits.OSMet` rows; `checkLowerLayerProducer` verifies `<path>.go::<Symbol>` exists in the checkout.
- [ ] `internal/le/rfc/check_evidence.go` - `evidenceKinds` ratchets `{lower-layer}` and `{not-applicable}`; `layerPathGap` checks `linux:`, `go:`, `vendor/`; `notOwedGap` checks `{not-applicable}`, `{feature-declined}`, `{linux-limit}`.
- [ ] `internal/le/rfc/profilecoverage.go` - `StemFigures` tracks `OSVerified`/`OSGap`/`OptionalOSVerified`/`OptionalOSGap`, `kindOS = "os"`, `mustProven(zeProven, osVerified)`, `osVerified(req, osProofs)`, and `Rollup.mandatoryComplete()`.
- [ ] `internal/le/rfc/sectioncounts.go` - `MustSections` and `ShouldSections` hold `OSVerified`/`OSGap`; `SectionsOf` enforces arithmetic add-back; `Sections.String()` renders the exact CLI clause order.
- [ ] `internal/le/rfc/implemented.go` - `ImplementedSectionsOf` sums `SectionsOf` over walked `Implements(meta)` RFCs (`ze` and `mixed`); `ProvenPercent()` divides `Must.Proven()` by `Must.Kept`.
- [ ] `internal/le/rfc/tree.go` and `internal/le/rfc/rollup_published.go` - marshal/unmarshal `TreeRow` and `Rollup`, subtracting `OSVerified` from `mandatory-proven` to restore `ZeProven` on decode.
- [ ] `internal/le/rfc/intent.go` - `evaluateIntent` refuses `{intent}` markers on `osMet` rows.
- [ ] `internal/le/rfc/actions.go`, `internal/le/rfc/render_ledger.go`, `internal/le/rfc/next.go` - print section names in `le help rfc tree`, Evidence cell (`kindOS`), and `le rfc next` completion prose.
- [ ] `internal/le/rfc/discriminate_interop.go` - reads interop scenario pass/fail/attribution via `internal/le/test/overlay`.
- [ ] `internal/le/site/rfcdata.go`, `rfcfallback.go`, `rfcreader.go`, `rfccompliance.go`, `rfcledger.go`, `rfcprofiles.go` - declare `rfcBars`, `rfcRegisterOpenSections`, `rfcSegmentMeanings`, `rfcLowerLayerBucket`, and profile SHOULD figures.
- [ ] `internal/le/test/health/collect_rfc.go` - renders the `/quality/health/` proof-density detail line from `ImplementedSections` and `annotationCounts`.
- [ ] `website/assets/js/rfc-figures.js`, `website/assets/js/rfc-map.js`, `website/assets/css/30-components.css`, `website/assets/rfc-map.css` - client-side bar rendering, completeness check, RFC map validation, and segment colors.
- [ ] `internal/test/fixture/ui_fixture_le_help_rfc_tree_sections.go` and `ui_fixture_le_rfc_derived_proof.go` - UI wiring fixtures asserting the exact MUST bar segment order and Evidence cell vocabulary.
- [ ] `test/interop/Dockerfile.stayrtr` and `internal/le/interoplab/discover.go` - StayRTR v0.6.4 pinned image (serves static `vrps.json`, no CMS/X.509 ROA validation) and interop scenario discovery.

**Behavior to preserve:**
- Existing `{lower-layer}` rows, `OS verified` / `OS testing gap` counts, `linux:`/`go:`/`vendor/` checks, and `rfc/exclusion-evidence-backlog.txt` counts remain byte-for-byte unchanged.
- `Ze proven` counts ONLY requirements proven by Ze's own Go code; an `{external-program}` row is never counted in `Ze proven` or `OS verified`.
- `Implementation: third-party` remains available for RFCs Ze neither implements nor verifies with a pinned external helper program.
- Existing StayRTR scenarios (`rtr-stayrtr`, `rtr-stayrtr-preference`, `live-rpki`) remain unchanged.
- An `{external-program}` row lacking either polarity or lacking a verifying discrimination record stays an `External testing gap`, never raises `ProvenPercent()`, and keeps `Rollup.mandatoryComplete()` false.

**Behavior to change:**
- Add `{external-program}` coverage annotation, artifact/producer evidence checks, `ExternalVerified` / `ExternalGap` (MUST and SHOULD) counts, CLI/JSON/site/health/RFC-map rendering, and the RFC 9582 end-to-end ROA validator pilot.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Coverage annotation registry & traits | `internal/le/rfc/rfc.go` (`annotationKinds`, `AnnotationTraits`) | Declares every `{...}` coverage annotation once, with parser and behavioral traits (`OSMet`, `CeilingExcluded`, `Bucket`, `RollupState`) | Extends: add `AnnotationExternalProgram` with dedicated `ExternalMet` trait and parser, keeping `OSMet` untouched |
| Lower-layer producer & component checker | `internal/le/rfc/summary.go` (`parseLowerLayer`, `producerRE`), `check_core.go` (`checkLowerLayerProducer`), `check_evidence.go` (`layerPathGap`) | Parses `<layer>; <producer>`, verifies `<path>.go::<Symbol>` exists in checkout and checks component artifact | Extends: reuse `producerRE` and `resolveProducer` for `{external-program}` producer checking; add `externalArtifactGap` for pinned `interop:<path>` artifacts |
| Single per-RFC section split | `internal/le/rfc/profilecoverage.go` (`StemFigures`, `osVerified`, `mustProven`), `sectioncounts.go` (`SectionsOf`, `MustSections`, `ShouldSections`, `Sections.String`) | Splits every walked RFC into `ZeProven`, `OSVerified`, `OSGap`, open sections, and `NotOwed`, refusing any arithmetic mismatch | Extends: add `ExternalVerified` / `ExternalGap` (MUST & SHOULD), include `ExternalVerified` in `MustProven()` / `MustSections.Proven()`, and gate `Rollup.mandatoryComplete()` on `ExternalGap == 0` |
| Published JSON round-trip & site bar layout | `internal/le/rfc/tree.go`, `rollup_published.go`, `internal/le/site/rfcdata.go` (`rfcBars`), `rfcfallback.go`, `rfcreader.go` | Single Go declaration of bar segments (`rfcBars`) published in `data/rfc/index.json` and read by `rfc-figures.js`; glossary panics if any segment lacks a meaning | Extends: add `external-verified` and `external-gap` segments to `rfcBars`, `rfcSegmentMeanings`, `rfcRegisterOpenSections`, and JSON round-trip subtraction beside `OSVerified` |
| Interop lab & discrimination recorder | `internal/le/interoplab/` (`Discover`, `RegisterCatalog`), `internal/le/rfc/discriminate_interop.go` (`interopCleanUnobserved`, `interopBrokenUnobserved`, `interopAttribution`) | Runs Docker interop scenarios, attributes assertion failures, and records discrimination proofs breaking Ze producers | Extends: reuse the BGP interop suite catalog and interop discrimination route for the RFC 9582 ROA validator pilot scenario |
| Client-side RFC map & bar renderers | `website/assets/js/rfc-figures.js`, `website/assets/js/rfc-map.js`, `website/assets/css/30-components.css`, `website/assets/rfc-map.css` | Draws bars from `data/rfc/index.json` (`rfc-figures.js`) and node/inspector proof cards from `data/rfc/<stem>.json` (`rfc-map.js`) | Extends: add `external-verified` and `external-gap` to `registerOpenSections`, `rfc-map.js` field validation/segments/proven sum, and WCAG-checked CSS segment tokens |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | The core knows nothing, and everything registers | system door | "A feature registers from `init()` in its own `register.go` and is discovered." | agrees | The pilot interop scenario registers via `init()` in `internal/le/interoplab/bgp/register_rpki_roa_validator.go`; `AnnotationExternalProgram` is declared once in `annotationKinds` (`internal/le/rfc/rfc.go`). |
| `ai/doors/le-tooling.md` | Where it sits | door covering `internal/le/` and `internal/test/` | "A shipped `ze` never links an `internal/le` package." | agrees | Every change stays inside `internal/le/`, `internal/test/fixture/`, `test/interop/`, `website/`, `rfc/`, and `docs/`; no shipped `ze` package imports `internal/le`. |
| `docs/architecture/core-design.md` | Command engine and composition roots | `// Design:` page of `rfc.go`, `summary.go`, `check_core.go`, `render_ledger.go`, `actions.go` | "The closed vocabularies the gate reads are EXPORTED, and every publisher of a verdict reads them rather than spelling one." | agrees | `AnnotationExternalProgram` and `ExternalMet` are declared once in `internal/le/rfc/rfc.go` and exported via `AnnotationKinds` and `AnnotationTraitsOf`; `./le rfc check` reads stored records and never executes Docker interop. |
| `docs/contributing/rfc-conformance-gates.md` | Coverage figures | `// Design:` page of `check_evidence.go`, `intent.go`, `profilecoverage.go`, `sectioncounts.go`, `implemented.go`, `tree.go`, `rollup_published.go`, `next.go`, and UI fixtures; modified by this spec | "The MUST bar is, in order, Ze proven (the mandatory proven rows), OS verified, proof pending, OS testing gap, exclusion unchecked, not assessed and not built." | departs | Expands the 7-segment MUST bar to 9 segments (`Ze proven`, `OS verified`, `External verified`, `proof pending`, `OS testing gap`, `External testing gap`, `exclusion unchecked`, `not assessed`, `not built`), adds `ExternalVerified` to `MustProven()` and `MustSections.Proven()`, adds `external` to the Evidence cell, and gates `Rollup.mandatoryComplete()` on `ExternalGap == 0` (owner decisions, 2026-11-23). |
| `docs/architecture/site-facts.md` | RFC completion data | `// Design:` page of `rfcdata.go`, `rfcfallback.go`; anchored to `sectioncounts.go`; modified by this spec | "`must.kept` is the MUST bar: `ze-proven + os-verified + proof-pending + os-gap + exclusion-unchecked + not-assessed + not-built`, drawn in that order." | departs | Adds `external-verified` and `external-gap` to `data/rfc/<stem>.json` (`must` and `should`), `rfcBars`, `rfcRegisterOpenSections`, `registerOpenSections`, `proven-by-kind` (`external`), and the `complete` register check, while keeping `rfcBars` as the single Go declaration published in `data/rfc/index.json` (owner decisions, 2026-11-23). |
| `docs/architecture/testing/test-health.md` | The report | `// Design:` page of `collect_rfc.go`; anchored to `rfc.go`, `implemented.go`; modified by this spec | "The density is the proven MUST rows (`rfc.MustSections.Proven`: Ze proven plus OS verified, the numerator the home card divides) over the MUST rows Ze and the OS owe (Ze proven, OS verified, proof pending, OS testing gap, exclusion unchecked, not assessed and not built)." | departs | Includes `External verified` in `rfc.MustSections.Proven` (numerator) and both `External verified` and `External testing gap` in the owed denominator, while continuing to read `rfc.AnnotationKinds`, `rfc.AnnotationTraitsOf`, and `rfc.ImplementedSectionsOf` directly and reporting Ze, OS, and External verified separately in the detail line (owner decisions, 2026-11-23). |
| `docs/contributing/gh-pages.md` | Quality pages | `// Design:` page of `rfcreader.go`, `rfcprofiles.go`; modified by this spec | "The homepage RFC card states one, from `rfc.ImplementedSectionsOf` through the site facts: the proven MUST rows, Ze proven plus OS verified, over the MUST rows Ze and the OS owe (Ze proven, OS verified, proof pending, OS testing gap, exclusion unchecked, not assessed and not built)" | departs | Adds `External verified` and `External testing gap` to the MUST/SHOULD bars, `rfcSegmentMeanings` glossary, compliance bucket mapping via `rfc.AnnotationTraitsOf`, and homepage RFC card numerator/denominator, keeping `Ze proven`, `OS verified`, and `External verified` distinct (owner decisions, 2026-11-23). |
| `website/AI.md` | Website architecture | `// Design:` page of `rfccompliance.go`, `rfcledger.go`; modified by this spec | "Its script reads proof figures from the canonical per-RFC data files; the map never certifies a release. Connected RFC cards use a left-to-right background gradient with the same category colors, order and proportions as the recorded mandatory results bar, tinted for readable text." | agrees | `website/assets/js/rfc-map.js` and `website/assets/rfc-map.css` continue reading `data/rfc/<stem>.json` and mirror the updated 9-segment mandatory results bar order, colors, and proportions (`external-verified`, `external-gap`). |
| `docs/contributing/feature-maturity.md` | The ceiling | anchored to `profilecoverage.go`; modified by this spec | "A row the summary excludes from the ceiling (`{not-applicable}`, `{lower-layer}`, `{feature-declined}`, `{linux-limit}`) is not attainable, so it does not block" | departs | Adds `{external-program}` (`CeilingExcluded: true`, `ExternalMet: true`) beside `{lower-layer}` so external-program obligations stay outside Ze's own `Attainable` count in S3 while still gating `Rollup.mandatoryComplete()` on `ExternalGap == 0`. |
| `docs/contributing/rfc-implementation-guide.md` | 9.7 RFC Requirement Coverage Tags | anchored to `rfc.go`, `actions.go`; modified by this spec | "Each needs a reason (a bare annotation is rejected)." | agrees | Documents `{external-program: <program>, interop:<path>; <path>.go::<Symbol> ...}` alongside `{lower-layer}` and `{feature-declined}`, requiring a pinned artifact ref, a real Ze producer, and a reason on every row. |
| `docs/architecture/linux-limitations.md` | How an entry is written | anchored to `internal/le/rfc/profilecoverage.go` | - | silent | Governs the Linux Limitations Register and `{linux-limit: LL-<ID>}` entries; this spec leaves `{linux-limit}` parsing, verification, and counting unchanged. |
| `docs/architecture/testing/interop.md` | The suites, one per protocol area | modified by this spec | "A scenario with no checker, or a nil checker, is an ERROR rather than a skipped test, so a fixture and its registry cannot silently disagree." | agrees | The `bgp-rpki-roa-validator` pilot scenario lives under `test/interop/scenarios/bgp-rpki-roa-validator/` and registers its typed Go checker via `init()` in `internal/le/interoplab/bgp/register_rpki_roa_validator.go`. |
| `docs/architecture/testing/interop.md` | Design decisions | decision of 2026-11-23 | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | The `bgp-rpki-roa-validator` pilot scenario runs in the BGP Docker interop suite, requires no special kernel capabilities, and preserves scoped kernel-capability checks unchanged. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Checklist lines in `rfc/short/<stem>.md` carrying `{external-program: <program>, interop:<path>; <path>.go::<Symbol> ...}`.
- Discrimination records in `rfc/discrimination/<stem>/<rid>.json` produced by `./le rfc record` over tagged interop/integration units.

### Transformation Path
1. `ParseSummaryText` (`internal/le/rfc/summary.go`) parses `{external-program: ...}` into `Annotation{Kind: AnnotationExternalProgram, Layer: <program>, LayerPath: <interop:path>, Producer: <path::Symbol>, Reason: <full>}`, refusing empty program, missing `interop:<path>`, missing producer, or `_test.go` producer.
2. `./le rfc check` validates producer existence (`checkExternalProgramProducer` via `resolveProducer`), artifact existence and version-pinning (`externalArtifactGap`: tracked file under `test/interop*/`, no `..`, no `:latest` tag), non-stale intent (`evaluateIntent`), and discrimination record freshness/polarity/producer match (`externalVerified`).
3. `StemFiguresOf` (`profilecoverage.go`) and `SectionsOf` (`sectioncounts.go`) partition `{external-program}` rows into `ExternalVerified` (both positive and negative polarities covered by tagged tests with verifying proving records, at least one breaking `req.Annotation.Producer`) vs. `ExternalGap`, counting `ExternalVerified` under `kindExternal` (`"external"`) in `MandatoryByKind` and in `MustProven()`, while leaving `ZeProven` and `OSVerified` untouched.
4. `TreeRow`, `Rollup`, `ImplementedSections`, `internal/le/test/health/collect_rfc.go`, and `internal/le/site/rfcdata.go` serialize and render the updated `Sections` into CLI text, JSON (`data/rfc/*.json`, `data/rfc/index.json`, `data/rfc-requirements.json`), `docs/features/rfc-status.md`, `/quality/rfc-compliance/`, `/quality/rfc-map/`, and `/quality/health/`.
5. For the RFC 9582 pilot, the Docker interop scenario builds an offline synthetic RPKI Trust Anchor + CA + EE certs + valid and malformed/forged `.roa` objects, runs the pinned Relying Party validator (`rpki-client` or `Routinator`, optionally exporting validated VRPs to StayRTR) into Ze's `bgp-rpki` RTR client, and verifies that valid ROAs reach Ze's VRP table and validate BGP routes while every RFC 9582-violating ROA is rejected before reaching Ze.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `rfc/short/*.md` → `internal/le/rfc` | Checklist annotation parser and evidence checks | No |
| `internal/le/rfc` → `internal/le/site` & `internal/le/test/health` | Typed `Sections`, `StemFigures`, `ImplementedSections`, `AnnotationTraits` | No |
| `internal/le/site` → `website/assets/js/*` | Published JSON (`data/rfc/<stem>.json`, `data/rfc/index.json`, `data/rfc-requirements.json`) | No |
| External ROA validator container ↔ StayRTR / Ze `bgp-rpki` | Validated VRP export + RTR (RFC 8210) over Docker interop network | No |

### Changed-Wire Census

**Search evidence:** Searched `internal/le/`, `website/`, `internal/test/`, and `docs/contributing/` for `OSVerified|OSGap|os-verified|os-gap|OSMet|AnnotationLowerLayer|lower-layer|kindOS|registerOf` (positive controls: `internal/le/rfc/sectioncounts.go`, `internal/le/site/rfcdata.go`, `website/assets/js/rfc-figures.js`, `website/assets/js/rfc-map.js`, `internal/test/fixture/ui_fixture_le_help_rfc_tree_sections.go`).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `data/rfc/<stem>.json` | writer | `internal/le/site/rfcdata.go::rfcStemDataOf`, `internal/le/rfc/sectioncounts.go::MustSections`, `ShouldSections` | Adds `external-verified` and `external-gap` integer JSON fields and `"external"` key in `proven-by-kind` | Additive fields, always emitted (0 when unused); regenerate via `./le site build` |
| `data/rfc/<stem>.json` | reader | `website/assets/js/rfc-figures.js::sumStems`, `registerOf`, `drawBar` | `registerOpenSections` hardcodes open MUST fields; `drawBar` reads layout from `index.json` | Update `registerOpenSections` to include `"external-gap"` |
| `data/rfc/<stem>.json` | reader | `website/assets/js/rfc-map.js::ensureFigure`, `paintProof`, `proofSegments` | Hardcodes `fields`, `proven = must["ze-proven"] + must["os-verified"]`, and `proofSegments` | Update `fields`, `proven` sum, `proofSegments`, and metrics card to include `external-verified` and `external-gap` |
| `data/rfc/<stem>.json` | reader | `internal/le/site/rfcfallback.go::rfcRegisterOpenSections`, `sumRFCData` | Hardcodes `rfcRegisterOpenSections` matching `rfc-figures.js` | Add `rfcMustExternalGapField` to `rfcRegisterOpenSections` |
| `data/rfc/index.json` | writer | `internal/le/site/rfcdata.go::rfcBars` | Declares `Must` and `Overview` bar segment lists published in `data/rfc/index.json` | Add `external-verified` and `external-gap` segments to both bars |
| `data/rfc/index.json` | reader | `website/assets/js/rfc-figures.js::drawBar`, `internal/le/site/rfcfallback.go::loadRFCFigureSet`, `internal/le/site/browsertest/rfcfigures_test.go` | Reads `bars` segment layout from `data/rfc/index.json` (and `testdata/rfc-bars.json` golden) to render bars | Preserved dynamically via `index.json`; refresh `internal/le/site/testdata/rfc-bars.json` golden |
| `le rfc tree \| json` & `data/rfc-requirements.json` (`TreeRow`, `Rollup`) | writer / reader | `internal/le/rfc/tree.go::MarshalJSON`/`UnmarshalJSON`, `rollup_published.go::rollupOfPublished`, `internal/test/fixture/ui_fixture_le_rfc_derived_proof.go::leRFCTreeSections` | `UnmarshalJSON` subtracts `OSVerified` from `mandatory-proven` to restore `ZeProven`; fixture checks `Evidence` kinds and `OSVerified` | Subtract `OSVerified + ExternalVerified` on decode; add `"external"` to `leRFCEvidenceKinds` and check `ExternalVerified` |
| `le rfc tree` text & `le help rfc tree` | writer / reader | `internal/le/rfc/sectioncounts.go::Sections.String`, `actions.go`, `internal/test/fixture/ui_fixture_le_help_rfc_tree_sections.go::leTreeMustTarget` | Fixture parses `MUST N kept: ...` and compares segment names in order against `le help rfc tree` and `leTreeMustTarget` | Update `Sections.String`, `actions.go` help text, and `leTreeMustTarget` together |

### Integration Points
- `internal/le/rfc/rfc.go` (`annotationKinds`, `AnnotationTraits`)
- `internal/le/rfc/summary.go`, `check_core.go`, `check_evidence.go`, `intent.go`
- `internal/le/rfc/profilecoverage.go`, `sectioncounts.go`, `implemented.go`, `tree.go`, `rollup_published.go`, `render_ledger.go`, `actions.go`, `next.go`
- `internal/le/site/rfcdata.go`, `rfcfallback.go`, `rfcreader.go`, `rfccompliance.go`, `rfcledger.go`, `rfcprofiles.go`
- `internal/le/test/health/collect_rfc.go`
- `website/assets/js/rfc-figures.js`, `website/assets/js/rfc-map.js`, `website/assets/css/30-components.css`, `website/assets/rfc-map.css`
- `internal/test/fixture/ui_fixture_le_help_rfc_tree_sections.go`, `ui_fixture_le_rfc_derived_proof.go`
- `internal/le/interoplab/bgp/` and `test/interop/` (RFC 9582 ROA validator pilot)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Every count derives from `StemFiguresOf` → `SectionsOf` → `rfcBars` / `ImplementedSectionsOf` |
| No unintended coupling (components stay isolated) | Yes | Shipped `ze` imports nothing from `internal/le`; `bgp-rpki` unchanged |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `annotationKinds`, `producerRE`, `resolveProducer`, `SectionsOf`, `rfcBars`, and `interoplab` catalog |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Offline conformance tooling and Docker interop harness only |
| Registration over hardcoding, outbound | Yes | Pilot scenario registers via `init()` in `internal/le/interoplab/bgp/register_rpki_roa_validator.go`; annotation kind declared once in `annotationKinds` |
| Registration over hardcoding, inbound | Yes | `le rfc`, `interoplab.Discover`, and `rfcBarGlossary` discover registered scenarios and segments automatically; the four explicit JS/fixture lists are tracked in Changed-Wire Census |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A real Relying Party ROA validator (`rpki-client` or `Routinator`) can validate a locally generated offline trust anchor + synthetic `.roa` files inside Docker with zero internet access | `test/interop/Dockerfile.stayrtr` requires deterministic offline scenarios; `rpki-client` / `Routinator` support local TAL + local repository files | Pilot scenario would fail offline or depend on external internet state | Offline Docker build and run of the pilot scenario with network disabled to external hosts | unvalidated |
| A-2 | Breaking Ze's integration producer (`rtr_pdu.go::parsePrefixPDU`) turns the pilot positive check red, and bypassing external ROA validation (serving the invalid ROA's payload directly as a VRP) turns the negative checks red | `internal/le/rfc/profilecoverage.go::osVerified` requires a verifying discrimination record; negative proof requires showing invalid ROAs would reach Ze if validation were skipped | A break of Ze's RTR parser alone could leave negative ROA assertions vacuous | `./le rfc record` break run on `parsePrefixPDU` plus explicit bypass red observation for negative ROA cases | unvalidated |
| A-3 | Existing `rfc/exclusion-evidence-backlog.txt` needs no new backlog line for `{external-program}` because zero legacy `{external-program}` rows exist | `docs/contributing/rfc-conformance-gates.md` (`{feature-declined}` carries no backlog line because every row is checked immediately) | Adding a backlog line would complicate the ratchet unnecessarily | Unit test refusing any `{external-program}` row without valid `interop:<path>` and `<path>.go::<Symbol>` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `{external-program}` becomes a rubber stamp where a static mock (like StayRTR's `vrps.json`) earns `External verified` credit for RFC 9582 | A test passes when malformed `.roa` inputs are not actually parsed, or cites `Dockerfile.stayrtr` for CMS/X.509 ROA checks | Require a pinned real validator artifact (`interop:test/interop/Dockerfile.rpkiclient`), require both positive and negative polarity coverage, prove negative red when validator enforcement is bypassed, and require `/ze-rfc-audit` independent review |
| R-2 | Adding `external-verified` / `external-gap` breaks JSON/JS/fixture consumers that hardcode the 7-segment MUST bar or 5 open sections | `TestGlossaryDefinesBarTerms`, `TestRFCSegmentColoursAreReadableInBothThemes`, `leHelpRFCTreeSections`, `leRFCDerivedProofRender`, or `rfc-map.js` fail | Changed-Wire Census enumerates every writer and reader across Go, JS, CSS, and UI fixtures; update all in one phase and run all unit + UI fixture tests |
| R-3 | `ProvenPercent()` or `Rollup.UnmarshalJSON` miscounts `ZeProven` when `ZeProven`, `OSVerified`, and `ExternalVerified` are all non-zero | `Rollup.UnmarshalJSON` or `TreeRow.UnmarshalJSON` returns an error or wrong `ZeProven` | Add unit tests decoding mixed non-zero `ZeProven`, `OSVerified`, and `ExternalVerified` and refusing corrupted totals |
| R-4 | Negative-polarity discrimination on an external validator cannot break C/Rust code via Go AST mutation | `./le rfc record` rejects a negative tag on an `{external-program}` row if it demands a Go mutant that turns the negative check red | Define the negative discrimination contract cleanly (either `external-bypass` route in `./le rfc record`, or explicit Choice C-1 approved by Thomas before implementation) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | `./le rfc check`, `./le rfc tree`, `./le site build`, `/quality/rfc-compliance/`, `/quality/rfc-map/`, `/quality/health/`, and UI fixtures (`le-help-rfc-tree-sections`, `le-rfc-derived-proof-render`); zero impact on shipped `ze` binary |
| How is it reverted? | Single commit revert of tooling + site + pilot scenario; no config migration |
| Who else touches this path? | Active RFC proof-debt and kernel-discrimination specs (`spec-rfc-proof-debt-outside-filter-wire` (closed), `spec-rfc-kernel-patch-discrimination-route.md`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `rfc/short/<stem>.md` `{external-program: ...}` line | → | `ParseSummaryText` → `parseExternalProgram` → `checkExternalProgramProducer` → `externalArtifactGap` | `TestCheckExternalProgramAnnotationEndToEnd` (`internal/le/rfc/external_program_test.go`) |
| `le rfc tree` and `le help rfc tree` CLI | → | `StemFiguresOf` → `SectionsOf` → `Sections.String` → `actions.go` | `test/ui/le-help-rfc-tree-sections.ci` (`leHelpRFCTreeSections`) |
| `le rfc update` → `docs/features/rfc-status.md` | → | `StemFigures.MustProven` → `MandatoryByKind[kindExternal]` → `render_ledger.go` | `test/ui/le-rfc-derived-proof-render.ci` (`leRFCDerivedProofRender`) |
| `le site build` → `data/rfc/*.json`, `data/rfc/index.json`, HTML/CSS/JS | → | `rfcDataOf` → `rfcBars` → `rfcSegmentMeanings` → `rfc-figures.js` & `rfc-map.js` | `TestRFCExternalProgramSiteAndHealthRendering` (`internal/le/site/rfcdata_test.go`) and `TestRFCSegmentColoursAreReadableInBothThemes` |
| BGP interop runner (`./le test integration interop`) | → | `internal/le/interoplab/bgp/register_rpki_roa_validator.go` → `checkRPKIROAValidator` → Ze `bgp-rpki` RTR client | `bgp-rpki-roa-validator` (`test/interop/scenarios/bgp-rpki-roa-validator/`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A summary line carries `{external-program: <program>, interop:<path>; <path>.go::<Symbol> ...}` with an existing pinned `test/interop*/Dockerfile.*` and an existing non-test Go producer | `ParseSummaryText` parses `Layer`, `LayerPath`, `Producer`, and `Reason`; `./le rfc check` accepts the annotation and refuses missing/unpinned artifacts, `..` traversal, missing producers, or `_test.go` producers with messages naming the fix | Existing `{lower-layer}` rows with `linux:`, `go:`, `vendor/` remain parsed and validated by `parseLowerLayer` unchanged |
| AC-2 | A walked RFC with `Implementation: mixed` holds gated `{external-program}` rows: one with no tests, one with positive-only tag, and one with both positive and negative proving records (at least one breaking the named Ze producer) | `StemFiguresOf` and `SectionsOf` place the first two in `ExternalGap` and the third in `ExternalVerified`; `ZeProven` and `OSVerified` stay `0`; `MustProven()` includes `ExternalVerified`; `MandatoryByKind["external"]` equals `ExternalVerified`; `Rollup.mandatoryComplete()` is `false` while `ExternalGap > 0` and `true` once `ExternalGap == 0` and all other open sections are `0` | SHOULD/MAY `{external-program}` rows split identically into `OptionalExternalVerified` and `OptionalExternalGap`, never into `NotOwed` or intent counts |
| AC-3 | `TreeRow.MarshalJSON`/`UnmarshalJSON`, `Rollup.MarshalJSON`/`UnmarshalJSON`, `le rfc tree`, `le help rfc tree`, and `docs/features/rfc-status.md` render an RFC with non-zero `ZeProven`, `OSVerified`, and `ExternalVerified` | JSON round-trip preserves exact `ZeProven`, `OSVerified`, and `ExternalVerified` and refuses corrupted counts where `OSVerified + ExternalVerified > mandatory-proven`; `Sections.String` and `le help rfc tree` name `External verified` and `External testing gap` in exact bar order; Evidence cell renders `external N` after `os N` | Existing `le-help-rfc-tree-sections.ci` and `le-rfc-derived-proof-render.ci` updated to verify the 9-segment MUST bar and `external` evidence kind |
| AC-4 | Site build (`internal/le/site/`), `/quality/health/` (`internal/le/test/health/`), and client JS/CSS (`rfc-figures.js`, `rfc-map.js`, `30-components.css`, `rfc-map.css`) render the updated sections | `rfcBars` publishes `external-verified` and `external-gap` on both Must and Overview bars; `rfcSegmentMeanings` defines both; `rfcRegisterOpenSections` and `rfc-figures.js::registerOpenSections` both include `external-gap`; `rfc-map.js` validates and sums `ze-proven + os-verified + external-verified`; CSS classes pass `TestRFCSegmentColoursAreReadableInBothThemes`; health proof-density text reports `ExternalVerified` and `ExternalGap` accurately | Existing `ze-proven`, `os-verified`, `os-gap`, and `not-owed` segments keep their current labels, colors, and meanings |
| AC-5 | Pilot scenario `bgp-rpki-roa-validator` runs offline in Docker with a pinned Relying Party ROA validator (`rpki-client` or `Routinator`, optionally feeding StayRTR) connected to Ze `bgp-rpki` | Valid synthetic CMS-signed `.roa` produces a VRP in Ze and validates a matching BGP route `Valid`; deliberately malformed/forged `.roa` objects covering RFC 9582's 16 MUST requirements are rejected before reaching Ze so their prefixes remain `NotFound`; breaking Ze's `rtr_pdu.go::parsePrefixPDU` turns the positive check red; bypassing ROA validation turns the negative checks red | Existing StayRTR scenarios (`rtr-stayrtr`, `rtr-stayrtr-preference`, `live-rpki`) remain unchanged and continue proving RFC 8210 / RFC 6811 |
| AC-6 | `rfc/short/rfc9582.md` is updated to `Implementation: mixed` with `{external-program}` rows, tagged interop checks, recorded discrimination proofs, and independent `/ze-rfc-audit` verdicts | `./le rfc check` passes clean; `rfc9582` moves from 16 `not built` to `External verified` on proven rows (and `External testing gap` on any row still awaiting proof), with `Ze proven = 0` | `rfc/short/rfc6482.md` (obsoleted by RFC 9582) remains `Implementation: third-party` unchanged |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseExternalProgramAnnotation` | `internal/le/rfc/external_program_test.go` | AC-1: valid syntax, missing program, missing `interop:` ref, `..` traversal, unpinned `:latest`, missing producer, `_test.go` producer | |
| `TestCheckExternalProgramAnnotationEndToEnd` | `internal/le/rfc/external_program_test.go` | AC-1: `checkExternalProgramProducer` and `externalArtifactGap` accept real tree targets and refuse absent/renamed symbols or files | |
| `TestExternalProgramSectionsAndComplete` | `internal/le/rfc/external_program_test.go` | AC-2: 0-test, single-polarity, and both-polarity proving records split accurately into `ExternalVerified`/`ExternalGap` (MUST & SHOULD), `MustProven`, `MandatoryByKind["external"]`, `mandatoryComplete()`, and `evaluateIntent` refusal | |
| `TestExternalProgramJSONRoundTrip` | `internal/le/rfc/external_program_test.go` | AC-3: `TreeRow` and `Rollup` marshal/unmarshal preserve non-zero `ZeProven`, `OSVerified`, and `ExternalVerified` and reject corrupted sums | |
| `TestRFCExternalProgramSiteAndHealthRendering` | `internal/le/site/rfcdata_test.go`, `internal/le/test/health/collect_rfc_test.go` | AC-4: `rfcBars`, `rfcSegmentMeanings`, `rfcRegisterOpenSections`, `rfcLedgerCoverage`, and health proof-density prose | |
| `TestRFCSegmentColoursAreReadableInBothThemes` | `internal/le/site/` (existing contrast test) | AC-4: `.rfc-seg-external-verified` and `.rfc-seg-external-gap` meet WCAG 4.5:1 contrast in both light and dark themes | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `OSVerified + ExternalVerified` vs `MandatoryProven` on JSON decode | `0..MandatoryProven` | `MandatoryProven` | N-A | `MandatoryProven + 1` (refused) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-help-rfc-tree-sections` | `test/ui/le-help-rfc-tree-sections.ci` | `le rfc tree` and `le help rfc tree` print all 9 MUST segments in exact order | |
| `le-rfc-derived-proof-render` | `test/ui/le-rfc-derived-proof-render.ci` | `le rfc update` renders `external N` in Evidence cell matching `ExternalVerified` | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-rpki-roa-validator` | `test/interop/scenarios/bgp-rpki-roa-validator/` | Pinned `rpki-client` (or `Routinator`) + StayRTR + FRR | AC-5/AC-6: offline synthetic TA valid vs. malformed/forged ROAs validated end-to-end into Ze `bgp-rpki` | |

## Files to Modify
- `internal/le/rfc/rfc.go` - declare `AnnotationExternalProgram`, `ExternalMet` trait, and `annotationKinds` entry
- `internal/le/rfc/summary.go` - add `parseExternalProgram`, `externalArtifactRE`, and `externalProgramFormat`
- `internal/le/rfc/check_core.go` - handle `traits.ExternalMet` in `evaluate` and add `checkExternalProgramProducer`
- `internal/le/rfc/check_evidence.go` - add `externalArtifactGap` validating pinned `interop:<path>` artifacts on every `{external-program}` row
- `internal/le/rfc/intent.go` - refuse `{intent}` markers on `externalMet` rows
- `internal/le/rfc/profilecoverage.go` - add `ExternalVerified`, `ExternalGap`, `OptionalExternalVerified`, `OptionalExternalGap`, `kindExternal = "external"`, `externalVerified`, `mustProven`, and `mandatoryComplete()` check
- `internal/le/rfc/sectioncounts.go` - add `ExternalVerified`/`ExternalGap` fields, add-back checks, and `Sections.String` order
- `internal/le/rfc/implemented.go` - update comments/error text for Ze + OS + external-program owed rows
- `internal/le/rfc/tree.go` and `internal/le/rfc/rollup_published.go` - round-trip subtraction of `OSVerified + ExternalVerified`
- `internal/le/rfc/render_ledger.go`, `internal/le/rfc/actions.go`, `internal/le/rfc/next.go` - render `external` evidence kind, `le help rfc tree` prose, and `le rfc next` completion messages
- `internal/le/site/rfcdata.go`, `rfcfallback.go`, `rfcreader.go`, `rfccompliance.go`, `rfcledger.go`, `rfcprofiles.go` - add `external-verified`/`external-gap` bar segments, glossary meanings, open-section completeness list, compliance bucket, and profile SHOULD figures
- `internal/le/test/health/collect_rfc.go` - include `ExternalVerified`, `ExternalGap`, and `AnnotationExternalProgram` counts in proof-density metric
- `website/assets/js/rfc-figures.js`, `website/assets/js/rfc-map.js`, `website/assets/css/30-components.css`, `website/assets/rfc-map.css` - client-side completeness, RFC map validation/bars, and WCAG-checked CSS colors
- `internal/test/fixture/ui_fixture_le_help_rfc_tree_sections.go` and `internal/test/fixture/ui_fixture_le_rfc_derived_proof.go` - update expected 9-segment MUST bar order and `external` evidence kind
- `rfc/short/rfc9582.md` - move to `Implementation: mixed` with `{external-program}` rows once pilot scenario and proofs pass
- `docs/contributing/rfc-conformance-gates.md`, `docs/contributing/rfc-implementation-guide.md`, `docs/contributing/feature-maturity.md`, `docs/architecture/site-facts.md`, `docs/architecture/testing/test-health.md`, `docs/contributing/gh-pages.md`, `docs/architecture/testing/interop.md`, `website/AI.md`, `ai/rules/rfc-conformance.md` - document `{external-program}`, `External verified`, `External testing gap`, updated bar/density definitions, and the ROA validator pilot

## Files to Create
- `internal/le/rfc/external_program_test.go` - unit tests for parser, artifact/producer checks, two-polarity proof rule, section arithmetic, and JSON round-trip
- `test/interop/Dockerfile.rpkiclient` - pinned offline Relying Party ROA validator image (or `Dockerfile.routinator` per Choice C-2)
- `internal/le/interoplab/bgp/register_rpki_roa_validator.go` and `internal/le/interoplab/bgp/check_rpki_roa_validator.go` - registered BGP interop checker for `bgp-rpki-roa-validator`
- `test/interop/scenarios/bgp-rpki-roa-validator/` - offline synthetic TA/ROA fixture generator and scenario config

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Tooling-only spec (`internal/le/`); no shipped `ze` config or RPC |
| YANG validation constraints | N-A | No YANG leaves |
| YANG custom validators | N-A | No YANG validators |
| CLI commands/flags | Yes | `internal/le/rfc/actions.go` help prose for `le rfc tree` sections; no new verb |
| CLI grammar (keyword before value) | N-A | No new CLI verb or argument |
| Editor autocomplete | N-A | Tooling-only |
| Functional test for new RPC/API | Yes | `test/ui/le-help-rfc-tree-sections.ci`, `test/ui/le-rfc-derived-proof-render.ci` |
| Pipe completeness | Yes | `TreeReport`, `Rollup`, `CheckReport` already render through shared renderer |
| Env var registration | N-A | No new env var |
| Doctor check for runtime dependencies | N-A | Docker interop lab dependency is already checked by `interoplab` |
| Doctor code per problem class | N-A | Refusals are `./le rfc check` and `./le site build` diagnostics, not daemon doctor checks |
| Prometheus counters/metrics | N-A | Tooling-only |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP family/capability/attribute added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Tooling and RFC ledger presentation only; `features/rpki.md` scope unchanged |
| 2 | Config syntax changed? | No | No Ze config syntax change |
| 3 | CLI command added/changed? | No | `le rfc tree` help text updated in `internal/le/rfc/actions.go`; no `ze` CLI command changed |
| 4 | API/RPC added/changed? | No | No daemon API/RPC change |
| 5 | Plugin added/changed? | No | `bgp-rpki` unchanged |
| 6 | Has a user guide page? | No | `docs/guide/rpki.md` unchanged (external validator is interop proof, not new Ze config) |
| 7 | Wire format changed? | No | No protocol wire format change |
| 8 | Plugin SDK/protocol changed? | No | No SDK/protocol change |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9582.md` (and generated `docs/features/rfc-status.md`) |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (new ROA validator image and scenario) |
| 11 | Affects daemon comparison? | No | No daemon comparison change |
| 12 | Internal architecture changed? | Yes | `docs/contributing/rfc-conformance-gates.md`, `docs/contributing/rfc-implementation-guide.md`, `docs/contributing/feature-maturity.md`, `docs/architecture/site-facts.md`, `docs/architecture/testing/test-health.md`, `docs/contributing/gh-pages.md`, `website/AI.md`, `ai/rules/rfc-conformance.md` |
| 13 | Route metadata keys added/changed? | No | No metadata keys changed |
| 14 | Prometheus counters added/changed? | No | No metrics changed |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `website/data/repo-facts.json` (`interop.targets` / `interop.scenarios`) via `./le site facts update` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Updated in `## Files to Modify`: `docs/contributing/rfc-conformance-gates.md`, `docs/contributing/rfc-implementation-guide.md`, `docs/contributing/feature-maturity.md`, `docs/architecture/site-facts.md`, `docs/architecture/testing/test-health.md`, `docs/contributing/gh-pages.md`, `website/AI.md`. Unaffected declared/anchored pages: `docs/architecture/core-design.md` (states the exported-vocabulary and read-only-gate architecture without listing bar segments), `docs/architecture/linux-limitations.md` (governs `{linux-limit}` only, untouched), `docs/functional-tests.md` (documents `le rfc` command mechanics without enumerating annotation kinds or bar segments) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/contributing/rfc-conformance-gates.md` examples of sections and annotation syntax |

## Design presentation

### What changes

Ze's RFC conformance ledger gains a dedicated **External verified** / **External testing gap** lane alongside `Ze proven` and `OS verified`. When an RFC relies on an external user-space helper program rather than Ze Go code (starting with RFC 9582 ROA validation via a real Relying Party validator feeding Ze's `bgp-rpki` RTR client), the summary declares `Implementation: mixed` and annotates the delegated rows with `{external-program: <program>, interop:<Dockerfile>; <path>.go::<Symbol> ...}`. Those rows never enter `Ze proven` or `OS verified`: they sit in `External testing gap` until tagged tests cover both positive and negative polarities with non-vacuous discrimination proofs, at which point they become `External verified` and count toward `MustProven()` and `ProvenPercent()` with their own bar segment and `external N` evidence count.

### Before and after

1. **`./le rfc tree` MUST bar output**
   - Before: `MUST 16 kept: Ze proven 0, OS verified 0, proof pending 0, OS testing gap 0, exclusion unchecked 0, not assessed 0, not built 16`
   - After (before pilot proof): `MUST 16 kept: Ze proven 0, OS verified 0, External verified 0, proof pending 0, OS testing gap 0, External testing gap 16, exclusion unchecked 0, not assessed 0, not built 0`
   - After (once pilot proofs pass): `MUST 16 kept: Ze proven 0, OS verified 0, External verified 16, proof pending 0, OS testing gap 0, External testing gap 0, exclusion unchecked 0, not assessed 0, not built 0`
2. **`docs/features/rfc-status.md` Evidence cell for RFC 9582**
   - Before: `0 of 16 MUST proven` | `-` | `RFC9582-3-1, ...`
   - After: `16 of 16 MUST proven` | `external 16` | `-`
3. **Summary row in `rfc/short/rfc9582.md`**
   - Before: `{gap: ROA validation is wanted after the first release (owner ruling 2026-11-20) and is not built today...}`
   - After: `{external-program: rpki-client, interop:test/interop/Dockerfile.rpkiclient; internal/component/bgp/plugins/rpki/rtr_pdu.go::parsePrefixPDU consumes the validated VRP stream while rpki-client validates the RFC 9582 CMS/X.509 ROA object}`

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Generalize `OS verified` / `{lower-layer}` to hold both kernel and external user-space programs | Conflates Linux kernel obligations shipped with Ze OS against external user-space helpers; owner chose a separate `External verified` / `External testing gap` lane on 2026-11-23 |
| Mark RFC 9582 `Implementation: third-party` with no proof tracking | Removes RFC 9582 from Ze's conformance counts entirely and leaves end-to-end ROA validation behavior unverified in the ledger |
| Use StayRTR alone for RFC 9582 proof | StayRTR serves pre-validated JSON VRPs and never parses CMS/X.509 `.roa` objects; claiming RFC 9582 proof from StayRTR alone would be vacuous |
| Add a 5th `Implementation: external` kind | Unnecessary churn across `meta.go`, `Implements(meta)`, and tests; `Implementation: mixed` already models RFCs split between Ze's integration boundary and another layer |

### Documents departed from

| Document | Before | After |
|----------|--------|-------|
| `docs/contributing/rfc-conformance-gates.md` (`Coverage figures`) | 7-segment MUST bar (`Ze proven`, `OS verified`, `proof pending`, `OS testing gap`, `exclusion unchecked`, `not assessed`, `not built`); `MustProven()` and `MustSections.Proven()` sum `Ze proven + OS verified`; Evidence cell lists Ze test kinds plus `os` | 9-segment MUST bar adding `External verified` after `OS verified` and `External testing gap` after `OS testing gap`; `MustProven()` and `MustSections.Proven()` sum `Ze proven + OS verified + External verified`; Evidence cell lists `external N` after `os N`; `Rollup.mandatoryComplete()` requires `ExternalGap == 0` |
| `docs/architecture/site-facts.md` (`RFC completion data`) | `data/rfc/<stem>.json` `must`/`should` carry 7 MUST bar fields; `rfcRegisterOpenSections` and `registerOpenSections` list 5 open fields; `proven-by-kind` carries Ze kinds plus `os` | Adds `external-verified` and `external-gap` to `must`, `should`, and `rfcBars`; adds `external-gap` to `rfcRegisterOpenSections` and `registerOpenSections`; adds `external` to `proven-by-kind` |
| `docs/architecture/testing/test-health.md` (`The report`) | RFC proof-density numerator is `Ze proven + OS verified` over the 7 owed MUST sections | RFC proof-density numerator is `Ze proven + OS verified + External verified` over the 9 owed MUST sections, with Ze, OS, and External verified stated separately in the detail text |
| `docs/contributing/gh-pages.md` (`Quality pages`) | Homepage RFC card and compliance index describe the 7 owed MUST sections and `Ze proven + OS verified` numerator; `rfcSegmentMeanings` defines 7 MUST segments + `not-owed` | Adds `External verified` and `External testing gap` to the homepage card description, `rfcSegmentMeanings` glossary, and compliance bucket mapping (`external_program` via `rfc.AnnotationTraitsOf`) |
| `docs/contributing/feature-maturity.md` (`The ceiling`) | S3 excludes `{not-applicable}`, `{lower-layer}`, `{feature-declined}`, and `{linux-limit}` from Ze's `Attainable` count | Also excludes `{external-program}` (`CeilingExcluded: true`) from Ze's `Attainable` count beside `{lower-layer}`, while keeping `External testing gap` as an open mandatory section in `Rollup.mandatoryComplete()` |

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Negative-polarity discrimination route for `{external-program}` rows | (a) (Recommended) Support an `external-bypass` break mode in the interop scenario (skipping the external validator so the invalid ROA's VRP reaches StayRTR/Ze and proves the negative assertion goes RED) alongside breaking Ze's named integration producer on the positive test; (b) Require building a patched external validator binary with each check disabled (like `patched-kernel`) | (a) `external-bypass` break mode alongside breaking Ze's integration producer on the positive test (owner, 2026-11-23) |
| C-2 | Which Relying Party ROA validator image to pin for the RFC 9582 pilot | (a) (Recommended) `rpki-client` (lightweight OpenBSD/portable C validator, supports offline local TAL/repo mode and exports JSON VRP files directly consumable by StayRTR); (b) `Routinator` (Rust validator with built-in RTR server, heavier build/image) | (a) `rpki-client` pinned offline image feeding StayRTR (owner, 2026-11-23) |

## Implementation Steps

1. **Phase 1: Wiring & Core Annotation (`internal/le/rfc/`)** -- declare `AnnotationExternalProgram` + `ExternalMet` in `rfc.go`, parser in `summary.go`, producer check in `check_core.go`, pinned artifact check in `check_evidence.go`, intent refusal in `intent.go`, and failing wiring unit tests in `external_program_test.go`.
2. **Phase 2: Figures, Sections, Round-Trip & CLI (`internal/le/rfc/`)** -- extend `StemFigures`, `MustSections`, `ShouldSections`, `SectionsOf`, `Sections.String`, `ImplementedSections`, `TreeRow`, `Rollup`, `render_ledger.go`, `actions.go`, and `next.go`; update UI fixtures `ui_fixture_le_help_rfc_tree_sections.go` and `ui_fixture_le_rfc_derived_proof.go`.
3. **Phase 3: Site, Health, JS & CSS (`internal/le/site/`, `internal/le/test/health/`, `website/`)** -- extend `rfcBars`, `rfcSegmentMeanings`, `rfcRegisterOpenSections`, `rfccompliance.go`, `rfcledger.go`, `rfcprofiles.go`, `collect_rfc.go`, `rfc-figures.js`, `rfc-map.js`, `30-components.css`, and `rfc-map.css`; verify WCAG contrast and site build tests.
4. **Phase 4: RFC 9582 ROA Validator Pilot (`test/interop/`, `internal/le/interoplab/bgp/`, `rfc/short/rfc9582.md`)** -- build pinned offline validator Dockerfile + synthetic TA/ROA test fixtures + registered `bgp-rpki-roa-validator` interop scenario covering positive and negative cases for all 16 RFC 9582 MUST rows; record discrimination proofs and obtain independent `/ze-rfc-audit` verdicts.
5. **Phase 5: Documentation & Verification** -- update `docs/contributing/rfc-conformance-gates.md`, `docs/architecture/site-facts.md`, `docs/architecture/testing/interop.md`, `website/AI.md`, `ai/rules/rfc-conformance.md`, and run scoped Go + UI + interop checks.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-6 has an implementation at file:line |
| Correctness | `SectionsOf` arithmetic invariant holds for every combination of `ZeProven`, `OSVerified`, `ExternalVerified`, `OSGap`, `ExternalGap`, and open sections |
| Naming | JSON keys kebab-case (`external-verified`, `external-gap`); CLI labels match `le help rfc tree` and `rfcBars` |
| Data flow | Every published count derives from `StemFiguresOf` → `SectionsOf` with no duplicated arithmetic |
| Rule: rfc-conformance | `{external-program}` rows never enter `ZeProven`, never raise proven counts without verifying two-polarity proofs matching the named producer |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Core RFC annotation, checks, sections, round-trip | `./le test unit pkg path ./internal/le/rfc/` |
| Site data, glossary, contrast, health collector | `./le test unit pkg path ./internal/le/site/` and `./le test unit pkg path ./internal/le/test/health/` |
| CLI help and derived status page fixtures | `./le test functional ui` (scoped to `le-help-rfc-tree-sections.ci` and `le-rfc-derived-proof-render.ci`) |
| RFC 9582 offline ROA validator pilot | `INTEROP_SCENARIO=bgp-rpki-roa-validator ./le test integration interop` and `./le rfc check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Untrusted artifact / path inputs | `externalArtifactGap` rejects `..`, absolute paths, symlinks escaping `test/interop*/`, and unpinned `:latest` Docker images |

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

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Separate `External verified` / `External testing gap` lane alongside `OS verified` | Generalizing `OS verified` into a single delegated lane | Owner decision 2026-11-23: keeps Linux kernel obligations distinct from external user-space helper programs |
| Reuse `Implementation: mixed` for external-program RFCs | Adding a 5th `Implementation: external` kind, or keeping `ze` | Owner decision 2026-11-23: `mixed` is already counted by `Implements(meta)` and accurately describes Ze's integration + external program |
| Require both positive and negative polarity coverage + Ze producer break for `External verified` | Single-polarity record matching `{lower-layer}` | Owner decision 2026-11-23: prevents a permissive external mock from earning credit without proving invalid inputs are rejected |
| Count `External verified` in `MustProven()`, `ProvenPercent()`, and `mandatoryComplete()` | Keeping `External verified` out of `ProvenPercent()` | Owner decision 2026-11-23: matches `OS verified` while keeping all three proven segments distinct on bars and Evidence cells |
| Pilot RFC 9582 with a real Relying Party ROA validator (`rpki-client` or `Routinator`), keeping StayRTR for RTR/ROV interop | StayRTR-only pilot or tooling-only spec | Owner decision 2026-11-23: StayRTR serves pre-validated VRPs and does not parse CMS/X.509 ROAs; a real RP validator genuinely demonstrates RFC 9582 end-to-end |

## Known Limitations
- Offline synthetic ROA validation proves the pinned external validator version and its integration with Ze's RTR client; it does not certify live public RPKI trust anchor availability.

## Review Gate

| Run | Scope | Reviewer | Result | Fixes / rerun |
|-----|-------|----------|--------|---------------|

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
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread
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
- [ ] Functional `.ci` tests for end-user behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
