# Spec: RFC support derived from proof

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P2 |
| Release reason | Truthful release support claims: the RFC ledger's hand-written Support status mixes availability with proof, so it under- and over-claims. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Each `rfc/short/<stem>.md` Meta table carries a hand-written `Support status`
cell (Supported, Partial, Experimental, Future, Unsupported, and free-text
variants) that answers two different questions at once: can an operator use the
capability, and how well is it proven. RFC 1334 shows the defect: every gated
MUST is proven in both polarities (13/13), yet the row reads `Partial` because
no interop scenario exercises PAP. That interop gap is a property of the PPPoE
and L2TP features, not of the RFC.

Owner decision (2026-10-30): availability is a feature-level fact, already held
by `features/<id>.md` Scope; an operator turns on L2TP, not PPP. The RFC ledger
reports proof only, and that proof is derived, never authored.

This spec:

| Part | Change |
|------|--------|
| 1 | Every unmet obligation disclosed only in `Support remaining` prose becomes a `{gap}`, `{partial}` or `{linux-limit}` requirement row first, so no disclosure is lost |
| 2 | The `Support status`, `Support coverage` and `Support remaining` Meta rows are deleted from every summary, with their parser, checks and renderers |
| 3 | The ledger publishes derived proof per RFC: gated MUSTs proven over gated, open unmet rows, and the evidence kinds behind the proof (unit, functional, interop) |
| 4 | Feature maturity criterion S3 reads that derived proof instead of the deleted cell |
| 5 | `Support area` and `Use cases` stay, as classification with no status |

Out of scope, owned by a follow-up spec behind this one: the public status page
showing features (Scope, Level) with their RFC proof beneath.

Overlap: `plan/release/validation/spec-feature-maturity-declared.md` decided
(2026-10-07) that S3 reads the ledger's Support status. This spec replaces that
decision.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - "The public ledger's edges", "Profiles and aggregates", "Coverage figures", "The linux-limit annotation"
  → Decision: the per-RFC figure that counts is the verified-record figure (`profilecoverage.go::figuresByStem`, `StemFigures`): a row is proven only when a discrimination record still verifies; `{gap}`/`{partial}` never count. `./le rfc tree` prints it (rfc1334: mandatory 13 of 13; rfc7947: mandatory 2 of 2).
  → Constraint: an unwalked RFC publishes no figure, it renders `not walked` (`StemFigures.cell`). The derived cell must keep that.
  → Constraint: `| Support | <aggregate> |` row presence and `checkPublicRowMonotonic` are keyed on `HasRow()`, not on Status, and stay.
  → Constraint: the linux-limit section requires `Support status: Partial` beside a `{linux-limit}`; that clause goes with the cell (corpus holds zero `{linux-limit}` today).
- [ ] `docs/contributing/feature-maturity.md` - Scope (availability) and Level (proof) per feature; S2 interop; S3 RFC
  → Decision: availability and interop already live on the feature. S3 is the only feature criterion that reads the RFC Status cell.
  → Constraint: S3 today rejects only `{gap}` (`feature/check.go::openGaps`); `{partial}` and `{linux-limit}` are unmet too.
- [ ] `ai/rules/rfc-compliance.md` - no conformance number for an unwalked list; a gap and an exclusion stay distinct
  → Constraint: deleting the prose must not turn a disclosed unmet MUST into a silent 100%.

**Key insights:**
- The Status cell mixes availability and proof. Availability is a feature fact; proof is already computed per stem.
- Three "proven" counts exist and disagree: `CoverageRow.Both` (tags in both polarities, no record check; printed by `proofCell`), `ProvenShareOf` (Both plus `{single-polarity}`), and `StemFigures.Mandatory.Proven` (verified records, walked-aware).
- `rfc.Implements` (provenshare.go) uses `statusIsSupportClaim(meta.Status)` to choose the population of the published proof share (health, site-facts `rfc.proven`, rfc-compliance page). It needs a replacement.
- Evidence kind and tier per test are parsed (`carriers.go::CarrierFor` → `Carrier{Kind, Tier}`) but never reduced per RFC.
- The corpus audit (`tmp/support-split/remaining-audit.md`, 296 items over 200 summaries) found unmet MUSTs disclosed only in prose on enrolled stems, e.g. rfc7947 Section 2.1 "A route server MUST accept all UPDATE messages ..." has no requirement row, so the derived figure reads 2 of 2 while a MUST is known unmet.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/meta.go` - parses `Support status`/`Support coverage`/`Support remaining` into `Meta.Status/Coverage/Remaining`; `rowsFrom` builds `LedgerRow`; `checkRowCells` requires Area/Status/Coverage; brief aggregates hold Remaining at `-` and use Coverage as Note.
- [ ] `internal/le/rfc/check_status.go` - `checkStatusAgreement`, `rowDisclosesGap`, `statusPromisesSupport`, `checkUnprovenSupport`, `statusIsSupportClaim`, `checkGapCountAgreement`, `checkSupportedSignoff`, `checkOutOfScope` (Status arm), `checkPublicRowMonotonic` (keyed on HasRow).
- [ ] `internal/le/rfc/check_audit.go` - `checkAuditDisclosure`: a wrong/unimplemented/partial audit verdict must be disclosed in Status or Remaining.
- [ ] `internal/le/rfc/provenshare.go` - `Implements` = enrolled && HasRow && statusIsSupportClaim; `ProvenShareOf`.
- [ ] `internal/le/rfc/render_ledger.go` - `renderStatusPage` prints Area, Status, Proof (`proofCell`, tag-based), Coverage, Remaining; `statusPageIntro` defines the Status vocabulary.
- [ ] `internal/le/rfc/rfc.go` - AnnotationTraits `BarsSupported`, `RemainingRequired`, read only by `checkStatusAgreement`.
- [ ] `internal/le/rfc/selftest_core.go`, `fixture.go`, `internal/test/fixture/ui_fixture_le_rfc_answers.go` - fixtures and selftests carrying the cells.
- [ ] `internal/le/site/rfcledger.go`, `rfcdetail.go`, `rfccompliance.go`, `facts.go`, `docs.go` - emit `public-status`/`public-coverage`/`public-remaining`, `status-counts`, `supported-with-remaining`, `top-rfcs[].status`; render per-RFC and index pages. No gh-pages JS reads these fields.
- [ ] `internal/le/test/health/collect_rfc.go` - proof share via `ProvenShareOf`.
- [ ] `internal/le/feature/check.go` - `criterionRFC` requires Status prefix `Supported` and no `{gap}`; reads `rfc.Collected` (no carriers, records or sign-offs).
- [ ] `ai/skills/ze-rfc.md`, `ai/skills/ze-close.md`, `ai/skills/ze-rfc-audit.md` (and `.claude/skills` copies), `docs/contributing/rfc-implementation-guide.md` - instruct authors to write the cells.

**Behavior to preserve:**
- `| Support | <aggregate> |` / `| Support | - |` row presence, `Support area`, `Name`, `Use cases`, `Optional use cases`, aggregate rollups and profile figures.
- `checkPublicRowMonotonic` refusal of a newly enrolled stem with no row and of a vanished row.
- Every unmet-requirement disclosure that exists today, now carried by an annotation.

**Behavior to change:**
- Delete the three Meta rows from all summaries and their parser fields, checks, traits, fixtures and emitters.
- Ledger, site and health publish the verified-record figure plus open unmet rows and evidence kinds.
- Feature S3 judges the derived proof.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `rfc/short/<stem>.md` Meta table and requirement rows; test tags; `rfc/discrimination/`; `rfc/extraction/`.

### Transformation Path
1. `ParseMeta` and requirement parse → `Meta`, `[]Requirement`.
2. `CoverageRows` (tags) and `figuresByStem` (verified records, walked) → per-stem proof.
3. `renderStatusPage` → `docs/features/rfc-status.md`; `site/*` → `data/rfc-requirements.json`, `data/rfc-compliance.json`, `data/site-facts.json`.
4. `feature.Check` → `criterionRFC` reads `rfc.Collected`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| rfc package → site package | `RenderInput`, `StemFigures`, `TreeRow` | No |
| rfc package → feature package | `rfc.Collected` (lacks carriers, records, sign-offs) | No |
| repo → gh-pages | JSON field names | No |

### Integration Points
- `figuresByStem` / `StemFigures` - the single derived proof source.
- `carriers.go::CarrierFor` - evidence kind per proven row.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | design phase |
| No unintended coupling (components stay isolated) | No | design phase |
| No duplicated functionality (extends existing, does not recreate) | No | design phase |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A tooling |
| Registration over hardcoding, outbound | No | design phase |
| Registration over hardcoding, inbound | No | design phase |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Out-of-scope stems (Enrolment `out-of-scope`, Status Future) need no new annotations: they leave every count by enrolment, so deleting Status loses no counted disclosure | `meta.Enrolled()`; rfc2868, rfc6514, rfc1570 read `Enrolment out-of-scope` | eight Future summaries would publish unannotated MUSTs as owed | grep Enrolment of every Future/Unsupported stem | unvalidated |
| A-2 | No gh-pages page reads `public-status`, `status-counts`, `supported-with-remaining` or `top-rfcs[].status` | research agent grep of `../gh-pages` and `website/` | a live page breaks on the JSON change | re-grep at implementation | unvalidated |
| A-4 | Unannotated, untagged MUST rows already read as unproven in the verified-record figure, so only missing rows and tagged-but-partly-unmet rows publish false proof | `profilecoverage.go::figuresByStem` counts only verified records | the figure could over-claim on another shape | AC-6 and a fixture with an untagged row | unvalidated |
| A-3 | About 50 Partial/Experimental stems with zero unmet rows say "partial" for feature or evidence reasons, not for a missed obligation | audit class E | a stem silently reads 100% for an obligation nobody extracted | extraction sign-off state of each such stem | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A prose-only unmet MUST is dropped and the figure reads 100% (rfc7947 2.1) | audit class C list | Part 1 lands first: every class C item on an enrolled stem gets a row or annotation before any cell is deleted |
| R-2 | The published proof share population moves when `Implements` stops reading Status | `site-facts.json` `rfc.proven` changes | owner decision 2026-10-30: population is enrolled, with a row, Implementation `ze` or `mixed`; the drop is stated in the commit |
| R-3 | A class C or D item is annotated with the wrong kind or a paraphrased quote | `./le rfc check` row-quote refusal; audit re-read | each new row quotes the RFC sentence verbatim from `rfc/full/`; every item is listed in the Part 1 table with its source sentence |
| R-4 | Deleting the tests of retired checks is read as weakening | `ze-test-judge` refusal | each removed assertion is judged by an independent `ze-test-judge` agent before commit; the checks are retired by owner decision, the judge confirms nothing else is lost |
| R-5 | Another session writes a `Support status` row while this lands | near-miss refusal on their commit | the refusal message names the cell as retired and points at the derived figure |
| R-6 | Stricter S3 lowers a feature's ceiling | `./le feature check` | measured 2026-10-30: no feature at Level supported lists an RFC, so no ceiling drops today |

## Owner Decisions

| Date | Decision |
|------|----------|
| 2026-10-30 | Availability is a feature fact (`features/<id>.md` Scope); the RFC ledger reports derived proof only. `Support area` and `Use cases` stay as classification. |
| 2026-10-30 | Two specs: this one (ledger, checks, S3); a follow-up owns the public status page built from features. |
| 2026-10-30 | Proof share population: enrolled, with a public row, Implementation `ze` or `mixed`. Enrolled Future/Unsupported RFCs (rfc4739, rfc4577, rfc5998, rfc7166, rfc8212, rfc8522, rfc8893) count at their real figure. |
| 2026-10-30 | Order: prose-only unmet obligations become requirement rows first, then the cells go, then only the verified-record figure is published. |

## Design

### Alternatives

| Approach | How | Gains | Costs | Verdict |
|----------|-----|-------|-------|---------|
| A: one derived source | Extend `StemFigures` with per-kind counts of proven mandatory rows and the ids of unmet rows; every surface (status page, site JSON, health share, feature S3) reads it. Delete the tag-based `proofCell` count and the Status arm of `Implements` | One definition of proven, the one `ai/META-DECISIONS.md` states (a recorded break) | `rfc.Collected` must expose figures, so feature S3 loads carriers, records and sign-offs | chosen |
| B: generated status label | Keep a Status word but compute it from the figures and write it into the Meta table | Pages keep a familiar word | A second copy of a derived fact, written into an authored file; drifts the moment a record stales | rejected (a fact is declared once) |
| C: keep tag-based Proof | Delete the authored cells, keep `proofCell` | Smallest diff | Publishes "proven" for rows no break record proves, contradicting the release rule; disagrees with `./le rfc tree` | rejected |

### Proposed changes

| Area | Current | Proposed |
|------|---------|----------|
| Summaries (Part 1) | Unmet obligations on enrolled stems disclosed only in prose | Each item of the Part 1 table below gets its row or annotation. Class D items on enrolled stems need nothing: each is an optional-level row that already exists and reads unproven in the optional figure, or an optional sentence with no obligation. Out-of-scope and backlog stems need nothing (A-1) |
| Summaries (Part 2) | 200 carry `Support status`, `Support coverage`, `Support remaining` | All three rows deleted; class E prose is not moved (the feature's S2 and Scope gaps already say it) |
| Features | pppoe-access, l2tpv2-bng and others omit RFCs the audit maps to them | Each class E stem with a mapped feature is added to that feature's `RFCs` field |
| `meta.go` | Parses the three labels | Labels and fields removed; `refuseNearMiss` refuses a leftover row with a message naming the cell as retired and the derived figure as its replacement |
| Checks | `checkStatusAgreement` Status arms, `checkGapCountAgreement`, `checkUnprovenSupport`, `checkSupportedSignoff`, `checkOutOfScope` Status arm, `checkAuditDisclosure` Status arm, traits `BarsSupported`/`RemainingRequired` | Deleted. Kept: `checkPublicRowMonotonic` (message reworded), the no-row arms of `checkStatusAgreement` and `checkAuditDisclosure` |
| `StemFigures` | Proven/Total/Attainable/Excluded, Walked | Adds proven mandatory rows by strongest evidence kind (interop > functional > editor > unit, via `CarrierFor`) and the unmet requirement ids (`{gap}`, `{partial}`, `{linux-limit}`, audit `wrong`/`unimplemented`/`partial`) |
| Status page | Area, Status, Proof (tag count), Coverage, Remaining; brief: Area, Status, Note | Area, Proof (`not walked`, or `N of M MUST proven`), Evidence (`unit 13`), Unmet (ids or `-`); brief shape: Area, Proof. Intro rewritten with no Status vocabulary |
| `Implements` / `ProvenShareOf` | Status-filtered; tag-based Both plus single-polarity | Enrolled && HasRow && Implementation ze/mixed; share from verified-record figures |
| Site JSON | `public-status`, `public-coverage`, `public-remaining`, `status-counts`, `supported-with-remaining`, `top-rfcs[].status` | Removed; `stems[].figures` gains the evidence-kind counts and unmet ids; index and per-RFC pages print the derived cells |
| Feature S3 | Status prefix `Supported` and no `{gap}` | Each enrolled listed stem must be walked and have every attainable mandatory row proven; a `{linux-limit}` (excluded) does not block, per owner rule. Message names the stem and its figure |
| Skills and docs | Instruct authors to write the cells | `ai/skills/ze-rfc.md`, `ze-close.md`, `ze-rfc-audit.md` (regenerated `.claude/skills` copies), `docs/contributing/rfc-conformance-gates.md`, `rfc-implementation-guide.md`, `feature-maturity.md` rewritten |

### Part 1 items

Source: corpus audit of 2026-10-30 (classes C and F on enrolled stems). Every new row quotes its sentence verbatim from `rfc/full/<stem>.txt`, and the implementer reads that text before choosing the annotation.

| Stem | Requirement | Disclosure today | Action |
|------|-------------|------------------|--------|
| rfc7947 | none exists | Remaining: Section 2.1 unmet | New row RFC7947-2.1-1 [MUST] quoting "A route server MUST accept all UPDATE messages received from each of its clients for inclusion in its Adj-RIB-In." with `{gap}`; the fix is owned by `plan/release/fixes/spec-rfc7947-adj-rib-in-accepts-filtered-updates.md` |
| rfc9774 | none exists | Remaining: route aggregation (Sections 5, 5.2, 6.1, 6.2) not implemented | New row RFC9774-5.2-1 [MUST] quoting the Section 5.2 sentence, `{gap}`: aggregation is an absent feature of an implemented RFC, owed after release, and no owner decision declines it |
| rfc9915 | RFC9915-18.2.10.1-1 [MUST], tagged by Linux tests | Remaining: unmet on the VPP backend | `{partial}` with tested span (Linux) and gap span (VPP) |
| rfc1195 | RFC1195-3.2-3, RFC1195-3.10.2-1 [MUST], untagged | Remaining: manually configured summaries outside the boundary | `{gap}` each, unless the RFC text makes manual summaries optional, then `{feature-declined}` per the gates page |
| rfc2131 | RFC2131-4.1-7, RFC2131-4.1-15 [MUST], untagged | Remaining: client neither emits nor honours option overload | `{gap}` each; RFC2131-4.1-4 and -4.1-5 bind only a sender that extends options: annotate per the RFC's own condition |
| rfc2205 | RFC2205-2-1 [MUST], unannotated | Remaining names reverse-path state | Read the producer: tag if met, `{gap}` if not |
| rfc4754 | RFC4754-7-1 [SHALL], unannotated, conditional on ECDSA use | Remaining | Conditional obligation: annotate only if Ze uses ECDSA there; otherwise record the condition per the gates page |
| rfc9514 | RFC9514-8-1, tagged on the export side only | Remaining: offline decoder lacks the sum check | `{partial}` if the sentence binds the receiver; otherwise nothing |
| rfc8050 | RFC8050-4.1-1, tagged | Remaining: TABLE_DUMP_V2 exposes no Path Identifiers | `{partial}` if the omission breaks 4.1-1; otherwise nothing |

### Feature RFC lists (AC-12)

Stems whose prose was a feature-level statement (audit class E), added to the feature's `RFCs` field where absent. Stems no feature maps (rfc2385, rfc5082, rfc3623, rfc4302, rfc5282, rfc5635, rfc7474, rfc7752, rfc8414, rfc8666, rfc9086, sflow-v5) are reported, not forced onto a feature.

| Feature | Stems |
|---------|-------|
| `l2tpv2-bng` | rfc1334, rfc1661, rfc1877, rfc2661, rfc2865, rfc2866, rfc2869, rfc5072, rfc5176 |
| `pppoe-access` | rfc1334, rfc1661, rfc1877, rfc2516, rfc5072 |
| `radius-admin-aaa` | rfc2865, rfc2866, rfc3579 |
| `ospf` | rfc2328, rfc3630, rfc5187, rfc5250, rfc5340, rfc5709, rfc5838, rfc7684, rfc7770 |
| `is-is` | rfc1195, rfc2966, rfc5286, rfc5304, rfc5305, rfc5310 |
| `bfd-liveness-detection` | rfc5880, rfc5881, rfc5882, rfc5883, rfc9384 |
| `mpls-ldp-rsvp-te` | rfc2205, rfc3209 |
| `ipsec-eap-authentication` | rfc3748, rfc5216 |
| `vrrp` | rfc3768, rfc9568 |
| `flowspec-to-firewall-bridge` | rfc8955, rfc8956 |
| `aigp` | rfc7311 |
| `bmp-delivery` | rfc7854 |
| `ikev2-engine` | rfc4303 |
| `ipsec-mobike` | rfc4555 |
| `ipsec-nat-traversal` | rfc7296 |
| `ixp-route-server-dynamic-peers` | rfc7947 |
| `lan-dns-resolver` | rfc1035 (backlog: listed, bounded by S3 as unenrolled) |
| `rpki-aspa-policy-enforcement` | draft-ietf-sidrops-aspa-verification |
| `tacacs-aaa` | rfc8907 |

### Triple Challenge

| Challenge | Answer |
|-----------|--------|
| Simplicity | Net deletion: three cells, six checks, two traits, one proof count. The one addition is two fields on an existing figure |
| Uniformity | Same pattern as the earlier ledger move (summary Meta became the single declaration): delete the copy, derive the surface |
| Performance | Tooling only; the kind reduction is one pass over tags already grouped by `tagsByRID` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing in the shipped binary. The public RFC pages and JSON publish a wrong figure, or a feature ceiling is misjudged |
| How is it reverted? | Part 2 and Part 3 revert as code commits; the Part 1 rows are true statements and stay |
| Who else touches this path? | Every session editing `rfc/short/` (it writes or reads the deleted cells); `spec-feature-maturity-declared` (S3); `spec-rfc-verdict-fix-bgp` (owns the gap-count journal row this spec retires) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` over a summary carrying `Support status` | → | `meta.go` near-miss refusal | `test/ui/le-rfc-retired-support-cell.ci` |
| `./le rfc update` rendering the status page | → | `renderStatusPage` reading `StemFigures` | `test/ui/le-rfc-derived-proof-render.ci` |
| `./le feature check` over a feature listing a partly proven RFC | → | `criterionRFC` reading the derived figure | `TestCheckRefusesSupportedWithUnprovenRFC` |
| site build emitting `data/rfc-requirements.json` | → | `site/rfcledger.go::rfcLedgerStemOf` | `TestRFCLedgerPublishesDerivedProofOnly` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A summary carries a `Support status`, `Support coverage` or `Support remaining` row | `./le rfc check` refuses it, naming the summary and the label, saying the cell is retired and the proof is derived, with the fix (delete the row) |
| AC-2 | The corpus after this spec | No file under `rfc/short/` carries any of the three labels; `./le rfc check` reports no Meta refusal |
| AC-3 | rfc7947 after Part 1 | Its mandatory figure is below 100% and its row lists RFC7947-2.1-1 as unmet |
| AC-4 | rfc1334 | Status page row reads 13 of 13 MUST proven with evidence `unit 13` and no unmet ids |
| AC-5 | An enrolled RFC with no valid extraction sign-off | Its row reads `not walked`, with no number |
| AC-6 | A tagged requirement whose tags carry no verifying discrimination record | Not counted as proven anywhere: status page, site JSON, health share |
| AC-7 | `data/rfc-requirements.json` and `data/rfc-compliance.json` | Carry no `public-status`, `public-coverage`, `public-remaining`, `status-counts`, `supported-with-remaining` or `top-rfcs[].status`; each stem's figures carry evidence-kind counts and unmet ids |
| AC-8 | Proof share population | Exactly the stems that are enrolled, have a public row, and declare Implementation `ze` or `mixed`; rfc4739 is counted, rfc9582 (third-party) is not |
| AC-9 | A feature at Level supported listing an enrolled RFC that is unwalked, or has an attainable mandatory row unproven | `./le feature check` refuses with an S3 line naming the stem and its figure |
| AC-10 | Same feature where every attainable mandatory row is proven and the only unmet row is `{linux-limit}` | S3 is met |
| AC-11 | Every item of the Part 1 items table | Resolved as its Action says, and every new or changed row is carried by a requirement row or annotation whose text is a verbatim span of the cited RFC section (`./le rfc check` row-quote gate passes) |
| AC-12 | `features/pppoe-access.md` and `features/l2tpv2-bng.md` | List rfc1334 in `RFCs`; each other class E stem with a mapped feature is listed in that feature |
| AC-13 | `ai/skills/`, `.claude/skills/`, `docs/contributing/` | No instruction to write the three cells remains (grep for the labels returns only the retirement note) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseMetaRefusesRetiredSupportCells` | `internal/le/rfc/meta_test.go` | AC-1, each of the three labels | |
| `TestStemFiguresCountProvenByEvidenceKind` | `internal/le/rfc/profilecoverage_test.go` | strongest kind per proven row; interop beats unit | |
| `TestStemFiguresListUnmetIDs` | `internal/le/rfc/profilecoverage_test.go` | `{gap}`, `{partial}`, `{linux-limit}` and audit findings listed | |
| `TestStatusPageRowIsDerived` | `internal/le/rfc/render_ledger_test.go` | AC-4, AC-5, AC-6: Proof, Evidence, Unmet cells; `not walked` | |
| `TestImplementsPopulation` | `internal/le/rfc/provenshare_test.go` | AC-8 | |
| `TestProvenShareCountsVerifiedRecordsOnly` | `internal/le/rfc/provenshare_test.go` | AC-6 for the share | |
| `TestCheckRefusesSupportedWithUnprovenRFC` | `internal/le/feature/check_test.go` | AC-9 | |
| `TestCheckAcceptsSupportedWithLinuxLimitOnly` | `internal/le/feature/check_test.go` | AC-10 | |
| `TestRFCLedgerPublishesDerivedProofOnly` | `internal/le/site/rfcledger_test.go` | AC-7 | |
| `TestRealCorpusCarriesNoSupportCells` | `internal/le/rfc/meta_test.go` | AC-2 over the real tree | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Mandatory total | 0 to N | 0 renders `no rows`, never 100% | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-rfc-retired-support-cell` | `test/ui/le-rfc-retired-support-cell.ci` | an author writes `Support status`; `./le rfc check` refuses with the retirement message | |
| `le-rfc-derived-proof-render` | `test/ui/le-rfc-derived-proof-render.ci` | `./le rfc update` renders a fixture corpus: one walked fully proven stem, one unwalked, one with a gap | |

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer.

## Files to Modify
- `rfc/short/*.md` - Part 1 rows and annotations; Part 2 deletion of the three rows
- `features/*.md` - class E stems added to `RFCs`
- `internal/le/rfc/meta.go` - remove labels and fields; refusal message
- `internal/le/rfc/ledger.go` - `LedgerRow` loses Status, Coverage, Remaining
- `internal/le/rfc/check_status.go` - delete Status-only checks; keep no-row arm and `checkPublicRowMonotonic` (reworded)
- `internal/le/rfc/check_audit.go` - drop Status arm of `checkAuditDisclosure`
- `internal/le/rfc/check.go`, `checkgroups.go` - unwire deleted checks; reword `CheckPublicStatus`
- `internal/le/rfc/rfc.go` - remove `BarsSupported`, `RemainingRequired`
- `internal/le/rfc/profilecoverage.go` - evidence-kind counts and unmet ids on `StemFigures`
- `internal/le/rfc/provenshare.go` - `Implements` population; share from verified records; message
- `internal/le/rfc/render_ledger.go` - derived columns, brief shape, intro
- `internal/le/rfc/status.go` - expose figures to `Collected` for feature S3
- `internal/le/rfc/profile.go` - ShapeBrief comment
- `internal/le/rfc/selftest_core.go`, `fixture.go`, `internal/test/fixture/ui_fixture_le_rfc_answers.go` - fixtures
- `internal/le/site/rfcledger.go`, `rfcdetail.go`, `rfccompliance.go`, `facts.go` - derived fields and pages
- `internal/le/test/health/collect_rfc.go` - comment and share source
- `internal/le/feature/check.go` - `criterionRFC`, `openGaps`
- Tests named in the TDD plan, plus removal of assertions of retired checks (judged by `ze-test-judge`)
- `ai/skills/ze-rfc.md`, `ai/skills/ze-close.md`, `ai/skills/ze-rfc-audit.md` and generated `.claude/skills` copies
- `docs/contributing/rfc-conformance-gates.md`, `docs/contributing/rfc-implementation-guide.md`, `docs/contributing/feature-maturity.md`
- `plan/release/validation/spec-feature-maturity-declared.md` - append a note that S3's 2026-10-07 decision is replaced here

## Files to Create
- `test/ui/le-rfc-retired-support-cell.ci`
- `test/ui/le-rfc-derived-proof-render.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | tooling, no config |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | No | `./le rfc` and `./le feature` verbs unchanged; outputs change |
| CLI grammar (keyword before value) | N-A | no new command |
| Editor autocomplete | N-A | no YANG |
| Functional test for new RPC/API | Yes | `test/ui/le-rfc-*.ci` above |
| Pipe completeness | N-A | no new command output path |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/pppoe-access.md`, `features/l2tpv2-bng.md` and other class E targets (RFCs field) |
| 2 | Config syntax changed? | N-A | none |
| 3 | CLI command added/changed? | N-A | product CLI untouched |
| 4 | API/RPC added/changed? | N-A | none |
| 5 | Plugin added/changed? | N-A | none |
| 6 | Has a user guide page? | N-A | tooling |
| 7 | Wire format changed? | N-A | none |
| 8 | Plugin SDK/protocol changed? | N-A | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/*.md` Part 1 rows; `docs/features/rfc-status.md` regenerated |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/rfc-conformance-gates.md`, `docs/contributing/feature-maturity.md` |
| 11 | Affects daemon comparison? | N-A | none |
| 12 | Internal architecture changed? | Yes | `docs/contributing/rfc-implementation-guide.md` |
| 13 | Route metadata keys added/changed? | N-A | none |
| 14 | Prometheus counters added/changed? | N-A | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared by changed files: `docs/architecture/core-design.md` (rfc area: update if it names the Status cell), `docs/architecture/testing/test-health.md` (proof share source and population), `website/AI.md` (RFC data fields published). Re-run `./le spec citation anchors spec plan/release/validation/spec-rfc-support-derived-proof.md` in each phase |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | example Meta tables in `rfc-conformance-gates.md` and `ai/skills/ze-rfc.md` |

### Discovery (repo-maintenance Mechanical Checklist)
| Question | Answer |
|----------|--------|
| Where an agent looks first | `ai/INDEX.md` row for RFC conformance → `docs/contributing/rfc-conformance-gates.md` |
| Rule that prevents regression | near-miss refusal of the retired labels (AC-1) |
| Registry that prevents drift | `StemFigures` is the single proof source; no surface computes its own |
| Verification that proves it | `./le rfc check`, `./le feature check`, `./le rfc selftest`, the two `.ci` tests |

## Implementation Steps

1. **Phase: Wiring** -- write the two `.ci` tests and `TestParseMetaRefusesRetiredSupportCells` failing
   - Files: `test/ui/`, `internal/le/rfc/meta_test.go`
   - Verify: tests fail because the labels still parse
2. **Phase: Part 1, summary rows** -- resolve each row of the Part 1 items table, quoting `rfc/full/` verbatim; judgement items (rfc9514-8-1, rfc8050-4.1-1, rfc4754-7-1, rfc2205-2-1) read the RFC sentence for which side it binds, FRR and BIRD where silent
   - Verify: `./le rfc check` row-quote gate passes; `./le rfc tree` shows rfc7947 below 100% (AC-3, AC-11). Commit
3. **Phase: derived figure** -- `StemFigures` kind counts and unmet ids; `Implements`; `ProvenShareOf` on verified records
   - Tests: `TestStemFigures*`, `TestImplementsPopulation`, `TestProvenShareCountsVerifiedRecordsOnly`
4. **Phase: retire the cells** -- remove labels, fields, checks, traits, fixtures; render derived columns; site JSON and pages; delete the three rows from all summaries; judge removed assertions with `ze-test-judge`
   - Tests: `TestStatusPageRowIsDerived`, `TestRFCLedgerPublishesDerivedProofOnly`, `TestRealCorpusCarriesNoSupportCells`, both `.ci`
5. **Phase: feature S3 and feature RFCs** -- `criterionRFC` on the derived figure; class E stems added to features
   - Tests: `TestCheckRefusesSupportedWithUnprovenRFC`, `TestCheckAcceptsSupportedWithLinuxLimitOnly`
6. **Phase: docs and skills** -- rewrite the pages and skills named in Files to Modify; note on `spec-feature-maturity-declared`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No surface computes "proven" other than through `StemFigures`; grep for `CoverageRow.Both` consumers in renderers |
| Correctness | Every class C/D item from the audit file is accounted for: a row, an annotation, or a recorded reason it is not owed |
| Data flow | Feature S3 reads the rfc package's figure, never re-derives it |
| Rule: no-layering | No fallback that reads a leftover Status cell |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No Support status/coverage/remaining row in the corpus | `grep -l '^| Support \(status\|coverage\|remaining\) |' rfc/short/*.md` prints nothing |
| Derived status page | `./le rfc update`, then the rfc1334 and rfc7947 rows |
| Feature S3 on derived proof | `./le feature check` passes; unit tests above |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Summaries are authored input: a malformed annotation still refuses, never silently counts as proven |

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
| `plan/journal/gate-ignores-the-level-it-counts.md` | 2026-10-06 | `./le rfc check` gap count over RFC9256/RFC9552 Support remaining (retired with `checkGapCountAgreement`) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Proof is the verified-record figure only | tag-based Both; generated Status word | the release rule counts a requirement only when a recorded break proved it |
| Availability lives on features | per-RFC availability axis | an operator enables L2TP, not PPP (owner) |
| S3 requires every attainable mandatory row proven | today's Status prefix check | the old check passed on one proven row; no current Supported feature lists an RFC |

## Known Limitations
- The public status page built from features is the follow-up spec named in the Task section; it is written when this one is ready.
- Class F items that are not disclosures (stale prose, pending owner decisions such as RFC 5216 versus RFC 8996) leave with the cell; any still owed already has its own row or spec.

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

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted tests pass: `go test ./internal/le/rfc/... ./internal/le/feature/... ./internal/le/site/... ./internal/le/test/health/...` through `./le`, `./le test ui` for `le-rfc-retired-support-cell` and `le-rfc-derived-proof-render`, `./le rfc selftest`, `./le rfc check`, `./le feature check`, then `./le verify changed` over the closing commits
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: tooling)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + journal rows owed
- [ ] **Commit B:** `remove` the spec only
