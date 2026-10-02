# Spec: Clause-scoped RFC proof without whole-requirement credit

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | - |
| Phase | implementation |
| Handoff | - |
| Updated | 2026-10-02 |

## Task

Represent a requirement whose implemented clause has real tests and recorded discrimination while another clause is an explicit implementation gap. Retain the whole verbatim RFC sentence, permanent requirement id, receive tests and records. Never publish partial evidence as enforcement of the whole requirement.

Owner decision 10 in `plan/handover/rfc-verdict-test-fix-pass/RULINGS.md` authorizes this scoped representation for RFC9514-7.2-5/6. The owner authorized design and implementation, not native SRv6 BGP EPE origination or deletion of receive proof. This document is the design proposal; the parent reviews it before authorizing its implementation phase. No interactive ze-spec gate, session claim change, implementation, RFC9514 ledger edit or verification is claimed by this design author.

Thomas approved the bounded annotation-plus-partial-verdict design on
2026-10-02 after the parent checked annotationBarsATest, stampFingerprints and
CoverageRows against the proposal. The gate explicitly chose unchanged
requirement/test identities and zero whole-requirement credit over a larger
clause-identity framework. Scope changes invalidate the independent audit;
unchanged historical discrimination observations are not rewritten.

The release bucket is `pre-release`: this is the RFC evidence owed outside the repository, not a change to the shipped packet path. Related work is `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` and its BGP child. `plan/pre-release/spec-excluded-requirement-owes-an-observable-test.md` concerns excluded obligations, not this still-counted obligation. The older `plan/pre-release/spec-bgp-ls-origination-and-the-scheduled-marker.md` concerns origination and scheduled work; neither a scheduled marker nor an exclusion represents proof of one clause.

## Required Reading

- [ ] `ai/rules/planning.md`, `ai/rules/spec-no-code.md`, `plan/README.md`, `plan/TEMPLATE.md`.
  → Constraint: preserve the parent claim/state; this proposal stays design until the parent carries the ze-spec gate. No code snippets or implementation evidence in a design.
- [ ] `ai/rules/architecture.md`, `ai/rules/simplicity.md`, `ai/rules/repo-maintenance.md`, `ai/INDEX.md` RFC discovery entries.
  → Decision: extend the existing annotation and verdict registers inside `internal/le/rfc`; introduce neither another ledger nor another command family.
- [ ] `docs/architecture/core-design.md`, command engine and gate/record separation.
  → Constraint: the gate reads evidence and never executes a proof; site and health consumers read the RFC package's vocabulary and derived counts.
- [ ] `docs/contributing/rfc-conformance-gates.md`, row quote, coverage, audit, extraction and discrimination contracts.
  → Constraint: the 24-character whole-row quote guard, permanent ids, recorded proof identity and every existing ratchet remain in force.
- [ ] `rfc/extraction/README.md`, `rfc/discrimination/README.md`, `ai/skills/ze-rfc-audit.md`.
  → Constraint: extraction records sentences, discrimination records an observed break of a tag claim, and only an independent judgment connects that evidence to the RFC's meaning. Partial annotation must not collapse those three questions.
- [ ] `website/AI.md` RFC ledger section and `docs/functional-tests.md` RFC Requirement Tags.
  → Constraint: public HTML, Markdown mirror, JSON and requirement shards must disclose the same scope. Retain existing tag grammar and six-cell shard shape.
- [ ] `plan/handover/rfc-verdict-test-fix-pass/{HANDOFF,AUTHOR-BRIEF,JUDGE-BRIEF,RULINGS}.md` and `bgp/c35-author.md` in that handover directory.
  → Constraint: latest owner ruling governs; no RFC9514 source/test/audit edits in this design phase; an independent judge, not a tool author, adjudicates the receive proof.
- [ ] `rfc/short/rfc9514.md`, `rfc/full/rfc9514.txt` §7.2, and corresponding audit, discrimination and extraction entries.
  → Constraint: both target rows retain their complete current Text and MUST level; absence of native SRv6 BGP EPE assignment/origination remains public.

**Key insights:** an annotation already keeps a row out of `CoverageRow.Both`, but other consumers and audit freshness need explicit treatment. `requirement_sha` currently hashes only `Requirement.Text`, which excludes annotations. Adding a scope without binding it to freshness would let a re-scoped claim retain an old judgment. No new discrimination key or extraction clause id is necessary when every tag on a partial row has one declared tested scope.

## Current Behavior

| Source read | Producing behavior and constraint |
|-------------|-----------------------------------|
| `internal/le/rfc/rfc.go`, `summary.go` | `Requirement` has one coverage `Annotation`; `annotationKinds` is the closed register. `parseAnnotation` parses kind-specific fields. `stripMarkers` removes annotations before storing Text. `Requirement.Quote` removes the trailing section citation only. |
| `internal/le/rfc/check_core.go` | `annotationBarsATest` forbids positive/negative tests beside gap, not-applicable, lower-layer, feature-declined and rollup; `evaluate` otherwise checks presence of polarities. There is no clause model. |
| `internal/le/rfc/coverage.go`, `provenshare.go`, `render_ledger.go` | `CoverageRows` counts every non-rollup gated row once; annotations are separate from Both/One/Missing. `ProvenShareOf` uses Both plus single-polarity. `proofCell` publishes a partition. |
| `internal/le/rfc/audit.go`, `check_audit.go`, `audit_stamp.go`, `freshness.go` | Five verdicts; enforced requires tests and polarity coverage. `stampFingerprints` and `auditFreshness` call `RequirementSHA(req.Text)`. Audit findings are published separately from polarity coverage. |
| `internal/le/rfc/artifact.go`, `inventory.go` | `ExtractionSite.MappedTo` is one requirement id. Sections sanction additional rows through UnsourcedIDs. Sites are sentences, not proof units. `RequirementSHA(string)` also hashes source text and must not acquire requirement-specific semantics. |
| `internal/le/rfc/render.go` | `RequirementRows` is the one producer of six cells. `requirementRow` already includes annotation reason and non-enforced audit marks. |
| `internal/le/rfc/check_status.go` | `checkStatusAgreement` and `checkGapCountAgreement` recognize only AnnotationGap. A new partial kind would otherwise evade gap disclosure. |
| `internal/le/site/rfcledger.go`, `rfccompliance.go`, `rfcevidence.go` | Site has explicit annotation buckets and an UnmappedAnnotations backstop; Gaps and GatedGaps currently denote full gap rows. Gap lists and NoTest arithmetic assume no positive/negative evidence. |
| `internal/le/test/health/collect_rfc.go` | Annotation counts feed a separately rendered coverage explanation; known gaps are described as genuinely untested. A partial row must not inherit that statement. |
| `internal/component/bgp/plugins/nlri/ls/reserved_rfc9514_test.go::TestRFC9514PeerNodeSIDReservedIgnored` | Tags assert receipt behavior, not origination: non-zero flag bits or Reserved word are accepted without changing relevant decoded values. Existing records name this unit and `decodeSRv6BGPPeerNodeSID`. The new type does not establish whether the assertions meet the independent audit standard. |

The full source sentence contains both origination and receipt obligations. Its receive tail, `ignored on receipt.`, is too short to serve as a standalone requirement under the existing quote guard. RFC9514-7.2-2/3 already disclose origination gaps, while -5/6 quote the whole sentence and retain receive tags. Extraction sites 7.2:2/3 map to -2/3; -5/6 are declared in §7.2 UnsourcedIDs. This proposal neither deletes nor allocates a requirement.

**Preserve:** all existing syntax and behavior for non-partial rows; native observed-red records; tag claim hashing; whole RFC quotes; ids, levels and section citations; audit independence; extraction denominators; all coverage/evidence/discrimination ratchets; no automatic proof runs.

**Change:** explicitly scope existing tags to a quoted part of their parent row, expose the remaining gap, and prevent any consumer from counting that row as whole-proven.

## Design and Interface Contract

### Alternatives

| Approach | Benefit | Cost / decision |
|----------|---------|-----------------|
| Registered partial annotation, one tested scope per row, existing tags and records | Preserves identities and uses existing single-disposition parsing/counting; no new ledger or tag syntax | Recommended. Adds one annotation kind, one audit verdict, scope-sensitive audit digest and consumer disclosure. |
| Child clauses with their own ids, tags, audits and records | Arbitrary per-clause proof composition | Rejected for this scope: new identity/migration/mapping machinery and record churn to express two existing receipt claims. |
| Allow ordinary gap beside tags, or split off a short receive-only requirement | Small local edit | Rejected: gap would contradict its established whole-row meaning, or the row quote would evade the truthfulness guard. Neither mechanically identifies what tests prove. |

### Authored annotation

Register `partial` in the existing annotation vocabulary. Its body is an ordered tested selector, gap selector and explanation, inside the ordinary trailing coverage marker. The textual shape is `{partial: tested "<tested span>"; gap "<gap span>"; <reason>}`. This is a format description, not a new tag grammar. Quoted strings use JSON string escaping; semicolons within them are content, not separators. Existing marker delimiter restrictions remain: unsupported literal braces must be rejected explicitly, not silently dropped or reinterpreted. No second coverage annotation may accompany it; the existing superseded marker remains independent.

| Parsed field | Type | Contract |
|--------------|------|----------|
| Kind | string | Existing Annotation.Kind, value partial, declared once in the register. |
| Tested | string | New kind-specific field, JSON `tested`; one non-empty exact contiguous span of the normalized parent Quote. Every positive/negative tag on this requirement refers only to this scope while the marker stands. |
| Gap | string | New kind-specific field, JSON `gap`; one non-empty exact contiguous span naming an unmet obligation in the parent Quote. |
| Reason | string | Existing field retains the whole annotation body so existing note rendering does not discard either selector. Explanation after the selectors is non-empty and states the missing behavior and a repo-relative non-test Go producer reference in existing path::Symbol form. |
| Producer | string | Existing field identifies the real production boundary inspected for the gap; must resolve, not name a fabricated absent function. Absence is still an audited conclusion, not something a function name proves. |

The parent Quote continues to pass its existing source/section/24-character check. Selectors are subordinate locators, not new requirements: use the same quote normalization; require exactly one match for each within that parent, distinct non-overlapping matches and word boundaries. Reject empty, absent, ambiguous, overlapping and whole-parent tested selectors. Do not impose the parent sentence's minimum length on a locator, or weaken that minimum for any requirement. Order in the RFC need not match selector order. The independent audit reads the full sentence: every obligation outside Tested remains unmet or unproven; Gap cannot imply that an omitted third obligation is met. No partial row ever earns whole credit, even if the marker is malformed or its audit is stale.

| Example row | Tested selector | Gap selector | Required disclosure |
|-------------|-----------------|--------------|---------------------|
| RFC9514-7.2-5 | ignored on receipt. | MUST be set to 0 when originated | Native SRv6 BGP EPE segment assignment and consequent TLV 1251 origination are absent; exporter masking of a supplied TLV is not native origination proof. |
| RFC9514-7.2-6 | ignored on receipt. | MUST be set to 0 when originated | Same absence, specifically the Reserved word. Retain the field-identifying prefix and full sentence in Text. |

### Gate, audit and freshness

| Surface | Required behavior |
|---------|-------------------|
| `evaluate` | Partial permits standing positive/negative tags only through its explicit branch; it requires both polarities for Tested. No tags or one polarity is a violation. Other annotationBarsATest cases are unchanged. A partial annotation alone declares scope, not proof. |
| Rollup derivation | A partial constituent derives gap, never met because it has both polarities. It remains a counted constituent; no new rollup exception. |
| Audit vocabulary | Add `partial`: the recorded tests enforce only the declared tested scope, and the whole requirement remains unmet. This is a finding, not an alias for enforced. Weak/wrong remain available if the scoped tests themselves are inadequate. |
| Partial verdict preconditions | Matching valid partial annotation, both polarities, non-empty tests/units maps, and a code map containing the annotation's real gap-context Producer. The note identifies tested behavior, missing behavior and the inspected production boundary. Require a current verified mutant/revert record for each distinct (rid, polarity, unit) cover before accepting partial; no-break, missing and stale records are not scoped proof. |
| Enforced verdict precondition | Explicitly reject enforced on any partial row, even if both tags and records exist. A stored invalid enforced verdict also contributes zero to published proven counts; rendering cannot rely on Check having run first. |
| Stamping | Default and rejudge modes apply the same partial/enforced cross-checks atomically before writing. Preserve existing fingerprint maps and keys, record order, and unrelated entries. A weak-to-partial transition is an independent rejudgment, not a mechanical reseal. |
| Scope-sensitive digest | Add one requirement-aware audit fingerprint producer. For non-partial rows, preserve exactly RequirementSHA(Text). For partial rows, hash an unambiguous canonical tuple of Text, kind, Tested, Gap, Producer and full Reason, with existing whitespace normalization. Use it in every audit stamping, freshness and reseal path. Do not change the generic RequirementSHA string function or extraction source hashes. |
| Freshness | Adding, changing or removing partial scope makes an existing audit stale-requirement unless independently rejudged. Reflow alone does not. Mechanical reseal must refuse semantic scope changes. Existing non-partial audit files do not need a corpus-wide reseal. |
| Ratchets | Partial becomes a finding for deletion and finding-to-enforced upgrade checks. Removing the marker alone does not resolve its recorded finding. Existing upgrade_reason safeguards apply to partial-to-enforced over unchanged units; acceptance still requires removal of the gap and whole-sentence independent judgment. Polarity, carrier/tier and record retention ratchets remain unchanged. No owner-approval bypass is added. |

The partial audit is the binding between the tested selector and the existing tag claims. A discrimination record still proves only its own claim paragraph under its observed break. Re-scoping the annotation leaves that historical observation intact but invalidates its audit binding; no surface may call the new scope proven until rejudged. Changed tag prose or test/producer behavior retains the existing native re-record obligation. No hashes or observed-red records are handwritten.

### Counts and publication

| Population or consumer | Contract |
|------------------------|----------|
| Requirement coverage | Each partial gated RID adds one to Gated and Annotated, zero to Both, One and Missing. Add `Partial` / JSON `partial` as an explicitly documented subset of Annotated, not an extra partition term. Existing Gated = Both + One + Annotated + Missing remains. |
| Whole-requirement proven share | Zero partial contribution to ProvenShareOf, audit Proven, supported-claim eligibility and every satisfied/proven bucket. Gated denominator unchanged. Never publish a fraction of a requirement as one proven requirement. |
| Audit worklist | Partial stays in the non-enforced worklist and finding total, at any level where a verdict is recorded. Do not make the parent weak/wrong listing look resolved by hiding partial findings elsewhere. |
| Gap disclosure | Partial rows require public gap disclosure under checkStatusAgreement/checkAuditDisclosure. A spelled total of unmet requirement rows counts ordinary gap plus partial, once per RID, irrespective of how many clauses are quoted. The explanation states this is a row count, not distinct protocol obligations. |
| Existing gap counters | Preserve ordinary gap/Demonstrate counter meanings. Add separate partial counts and lists; a partial receipt test is not a demonstrated origination gap. Combined displayed unmet totals may sum ordinary gaps and partial rows with labels naming both. |
| Site partition | Add a partial bucket inside Annotated, labelled `Partial proof; remaining gap`, never a green whole-satisfaction bucket. Keep it out of NoTest because tests exist. Show the unmet scope in gap detail without counting it again in ordinary GatedGaps or the bucket sum. |
| Requirement shard | Keep six columns. The Note includes tested selector, gap selector, explanation and partial audit/freshness. Positive/negative test links remain, labelled as scoped evidence, not whole-sentence enforcement. |
| Site JSON, HTML, Markdown | Carry parsed Tested/Gap/Producer through rfcLedgerAnnotation; retain full Text, tests, record status and partial verdict meaning. Publish scope beside each row and its record list. Build paths that use Collect/NewRenderInput without Check must still suppress whole credit. |
| CLI text/JSON/YAML/table, health, status page | Read the same derived partial subset, expose it even at zero, and qualify any surviving `proven` record count as tag-claim records, not requirements. The proof partition prints partial as a subset of annotated. Health no longer describes partial annotations as genuinely untested. |

### Extraction, corrections and migration

No extraction schema, site id, inventory algorithm or exclusion kind changes. A partial requirement still maps as one requirement or remains in an existing sanctioned UnsourcedIDs set; selector text creates neither a site nor a requirement. Mapping a source sentence is evidence of extraction, not full enforcement.

For RFC9514, retain sites 7.2:2/3 mapped to -2/3 and -5/6 in §7.2 UnsourcedIDs. The later ledger author corrects the section reason to say -5/6 retain the whole sentences with receipt-scoped evidence and an explicit origination gap; it must no longer imply they contain only the receipt half. Keep register, site counts, exclusions and all existing ids unchanged. The independent extraction review must not claim a new full walk merely because a reason changed.

The later ledger application records dated Correction paragraphs for -5/6 describing the scope annotation, quoting the whole RFC sentence and citing owner decision 10. It does not retire -2/3, deduplicate requirements, relevel rows or move tests. Those are separate semantic changes not authorized by this representation. Support status remains Partial; Support remaining names the absent native SRv6 BGP EPE originator. Preserve the four existing -5/6 discrimination records byte-for-byte when their native verification still establishes matching claims/units/producers; stale records are re-recorded natively, not deleted. No RFC9514 file changes belong to this design author.

## Data Flow

| Stage | Input → output | Boundary |
|-------|----------------|----------|
| Summary | Full checklist row → existing Requirement plus parsed partial fields | Authored prose to typed model; invalid syntax fails closed |
| Source validation | Parent Quote and normalized selectors → checked locators | RFC sentence remains sole authority |
| Tags / discrimination | Existing tags and records → scoped claim evidence | Historical observations retain identities |
| Audit | Requirement-aware digest, units, code and verified records → partial judgment / stale finding | Independent human meaning, mechanical freshness |
| Gate / counts | Typed requirements and evidence → unchanged primary partition plus partial subset | No clause credit enters whole numerator |
| Publication | RenderInput/RequirementRows/CoverageRows → CLI, shards, status, site and health | Shared counts, escaped at HTML/Markdown boundaries |

### Architectural Verification

| Check | Design answer | Evidence / implementation proof owed |
|-------|---------------|--------------------------------------|
| Intended layers | Yes | Existing Collect → evaluation/render input → consumers; no second collection service |
| No product coupling | Yes | All implementation inside repository tooling; no BGP runtime import added |
| No duplicate framework | Yes | Existing annotation, audit, extraction and discrimination artifacts |
| Runtime zero-copy | N-A | No packet/event path changes; normalize and parse once per loaded row, no repeated source scans in renderers |
| Outbound registration | Yes | Existing command entry points; new vocabulary in existing RFC registers, not commands |
| Inbound registration | Yes | AnnotationKinds and AuditVerdicts own vocabulary; update explicit site bucket mapping and vocabulary completeness tests. Search consumers of AnnotationGap, AnnotationRollup, VerdictEnforced, RequirementSHA and NoTest during implementation to ensure no hidden second meaning survives |

## Risks & Assumptions

| ID | Assumption | Basis | If wrong | Validation | Status |
|----|------------|-------|----------|------------|--------|
| A-1 | One tested scope per parent suffices | Both target ids carry receipt-only claims in TestRFC9514PeerNodeSIDReservedIgnored | Per-tag assignments would need a larger approved model | Independent judge reads every current tag before migration | confirmed for current tag prose; judgment owed |
| A-2 | Native SRv6 BGP EPE origination is absent | Owner decision 10; current Support remaining; c35-author producer investigation | Gap no longer describes current code | Ledger author/judge re-read native EPE producer at application | owner-established; current-tree recheck owed |
| A-3 | No additional schema is needed for extraction | ExtractionSite.MappedTo, UnsourcedIDs and existing RFC9514 mapping | A selector might be mistaken for a new requirement | Synthetic extraction equality test before/after annotation | source-confirmed; test owed |

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Scope laundering: trivial tested text masks the actual obligation | Locator passes but audit cannot link it to an assertion | Independent full-sentence judgment; all remainder unproven; zero whole credit regardless |
| R-2 | Annotation edit retains stale audit | Existing digest hashes Text only | Central requirement-aware fingerprint; add/change/remove/reseal negative cases |
| R-3 | Site or health counts tags as whole proof | Both rises or denominator shrinks on partial fixture | Exact one-row arithmetic across every consumer; invalid enforced fixture too |
| R-4 | Records copied/re-keyed or rerun merely to satisfy schema | Historic observation changes with unchanged tag claim | Preserve native keys/schema; audit binds scope, observations keep original meaning |
| R-5 | Shared checkout ledger writes race | Independent author/judge updates same stem | Existing per-stem locks and independent phase handoffs; design writes no ledger |
| R-6 | Quoted semicolons/braces break marker parsing | Row swallowed, selector truncated or unknown text accepted | Strict parser, explicit delimiter refusal, no silent fallback; adversarial parser fixtures |
| R-7 | Removing marker creates a whole-enforced appearance | New render without running Check shows Both | Partial audit finding stays visible and blocks whole audited proof; partial-to-enforced remains independently adjudicated and ratcheted |
| R-8 | A missing record is treated as scoped proof | Partial stamp succeeds with escaped/stale/absent record | Partial stamp and gate require verified proof routes; render names unverified scope instead of proof |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if wrong? | Corpus-wide compliance counts and public truthfulness, not packet handling. |
| Revert | Revert tooling and its later partial annotations/audits together; old tools intentionally refuse unknown vocabulary. Never deploy new ledger syntax before its reader. |
| Shared owners | Main integrates tooling; BGPHighAuthor owns RFC9514 application; independent judge stamps; no parent state or common handover edits by tooling author. |

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Existing rfc check command over isolated summary/tag fixture | → | Collect, parseAnnotation, evaluate, report | `TestPartialRequirementCheckReportsScope` and `test/ui/le-rfc-partial-proof.ci` |
| Existing audit-stamp command | → | auditStamp, verdictClaims, requirement-aware digest | `TestPartialAuditStampRefusesWholeEnforced` |
| Existing index-update and site publication | → | RequirementRows, CoverageRows, publishRFCLedger | `TestPartialProofPublicationsAgree` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Synthetic non-RFC9514 row with whole sourced sentence, partial annotation, uniquely matched receive tail under 24 chars | Parse retains exact RID/Text/level/section; both fields exposed. Full-row quote below 24 still refused; no RFC-number special case. |
| AC-2 | Empty/absent/ambiguous/overlapping selector, whole-parent tested span, malformed escapes, missing explanation/producer, nonexistent or test producer, duplicate coverage marker | Named refusal; row never disappears or gets whole credit. Test quoted semicolon and supported escaping positively. |
| AC-3 | Valid partial row with two, one or zero polarities | Two satisfies scoped coverage; one/zero yields named violation. Ordinary gap plus tags remains refused. Partial constituent makes rollup gap. |
| AC-4 | Partial audit with valid annotation, both tags, code fingerprint and verified records; controls missing each precondition | Valid independent judgment stamps partial atomically; each control refused. Enforced on partial refused in stamp and gate and never counted proven by render-only paths. |
| AC-5 | Only tested selector, gap selector, producer or reason changes; marker added/removed; whitespace-only reflow; unrelated non-partial row | Semantic changes yield stale-requirement and refuse reseal; reflow stays fresh; non-partial fingerprints remain unchanged. |
| AC-6 | One gated partial row, with two tags and two verified cover records in synthetic fixture | Gated=1, Annotated=1, Partial=1, Both=One=Missing=0; whole Proven=0; record proof count=2 claims, not two requirements. All primary partition sums remain exact. |
| AC-7 | Partial row published through check pipes, shard, status page, site JSON/HTML/mirror and health | Same whole text, scopes, gap reason and non-enforced status; links retained; partial subset visible; no `no test` label or whole-enforced badge on this row. |
| AC-8 | Partial row with missing public row or clean Supported/no-remaining claim; declared count omits partial | Gate refuses dishonest disclosure/count. Valid Partial status with named remaining behavior accepted; ordinary Demonstrate counts unchanged. |
| AC-9 | Annotation added with no tag/record/extraction edit | Existing record identities/hashes preserved; extraction sites/register/exclusion counts/ids unchanged; no new child requirements or unsourced ids. If tag claim changes instead, native discrimination freshness still rejects old record. |
| AC-10 | Partial finding deleted or changed to enforced over unchanged units without upgrade reason; polarity/carrier/record removed | Existing relevant ratchet refuses. Partial is retained in non-enforced worklist; no new approval exception. |
| AC-11 | Later RFC9514 -5/6 application after tool approval and independent judgment | Both whole sentences and ids survive, all real receive tests/records retained, public native EPE origination gap explicit, no whole enforcement. If receive assertions fail scoped audit they remain weak/wrong rather than being promoted by syntax. |

## TDD Test Plan

| Test | File | Validates |
|------|------|-----------|
| `TestPartialRequirementCheckReportsScope`, `TestPartialSelectorsRejectInvalidScope` | New `internal/le/rfc/partial_test.go` | AC-1..3, unrelated RFC fixture, quoted delimiters and word boundaries |
| `TestPartialAuditStampRefusesWholeEnforced`, `TestPartialAuditRequiresVerifiedClaims` | `internal/le/rfc/audit_stamp_test.go` | AC-4 atomic refusal and new/rejudge modes |
| `TestPartialScopeChangesInvalidateAudit` | `internal/le/rfc/audit_test.go` | AC-5 addition/removal/change, reflow and reseal |
| `TestPartialNeverMovesWholeProofNumerator`, `TestPartialDisclosureCountsRows` | New `internal/le/rfc/partial_test.go` | AC-6,8 including render-only invalid-audit input |
| `TestPartialKeepsExtractionAndClaimIdentity`, `TestPartialFindingCannotVanish` | New `internal/le/rfc/partial_test.go` | AC-9,10, baseline-backed fixture |
| `TestPartialProofPublicationsAgree` | `internal/le/site/rfcdetail_test.go` | AC-7 JSON/HTML/mirror, cards, gap list, record list |
| `TestPartialHealthExplainsScopedEvidence` | `internal/le/test/health/collect_rfc_test.go` | AC-6,7 health partition and terminology |
| `le-rfc-partial-proof` | New `test/ui/le-rfc-partial-proof.ci` | Real CLI check/index-update against isolated export; positive plus corrupted annotation/enforced-verdict controls |

Numeric boundaries: full quote length 23/24/25; selector occurrence count 0/1/2; overlapping versus touching spans; tag polarities 0/1/2; records missing/stale/escaped/verified. Interop N-A: no protocol behavior changes. Unit and functional results are owed by implementation, not run during design.

## Files to Modify

| Files | Responsibility |
|-------|----------------|
| `internal/le/rfc/rfc.go`, `summary.go`, `check_core.go`, `check_quote.go` | Annotation vocabulary/model, strict parsing, source-bound selectors, scoped coverage and rollup gap derivation |
| `internal/le/rfc/audit.go`, `audit_stamp.go`, `check_audit.go`, `freshness.go`, `reseal.go` | Partial verdict, matching declaration, scope-sensitive digest, verified-record preconditions and finding transitions |
| `internal/le/rfc/coverage.go`, `provenshare.go`, `check.go`, `check_status.go`, `render.go`, `render_ledger.go`, `sections.go` | Partial subset and honest reports/disclosure; no whole credit on render-only paths |
| `internal/le/rfc/discriminate_action.go` | Scope-qualified CLI presentation; no record schema/key/sealing change |
| `internal/le/site/rfcledger.go`, `rfccompliance.go`, `rfcdetail.go`, `rfcevidence.go` | Typed scopes, explicit bucket, record context, HTML/mirror publication and exact arithmetic |
| `internal/le/test/health/collect_rfc.go` | Partial annotation disclosure, no false untested/proven label |
| Test files named above; `internal/le/rfc/registry_test.go`, `summary_test.go`, `coverage_test.go`, `provenshare_test.go`, `render_ledger_test.go`, `check_audit_baseline_test.go`; site vocabulary/corpus tests | Existing closed-set and partition expectations plus regression controls |
| `docs/contributing/rfc-conformance-gates.md`, `docs/architecture/core-design.md`, `docs/functional-tests.md`, `website/AI.md` | Annotation/verdict contracts, public partitions, audit digest semantics and tag scope |
| `rfc/extraction/README.md`, `rfc/discrimination/README.md`, `ai/skills/ze-rfc-audit.md`, `ai/INDEX.md` | Extraction is not proof; unchanged observation identity; partial judging; discovery vocabulary |

`RequirementSHA`, extraction schema/algorithms, tag grammar, carrier registration and native discrimination sealing are intentionally unchanged; regression tests enforce these boundaries. No new package, command or generated artifact. Source Design headers naming core-design and website/AI are covered above. Implementation must resolve all affected fingerprint and verdict consumers, not only the named call sites.

## Files to Create

Only `internal/le/rfc/partial_test.go` and `test/ui/le-rfc-partial-proof.ci`. Prefer the existing implementation files; no new documentation page. Later corpus application belongs to BGPHighAuthor and the independent judge: `rfc/short/rfc9514.md`, `rfc/corrections/rfc9514.md`, extraction reason and audit entries. Records/tests change only if their own proof requires it, not to adopt the syntax.

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | Repository evidence, not product configuration |
| YANG validation | N-A | No leaf changes |
| YANG custom validators | N-A | No config surface |
| CLI commands/flags | Yes | Existing rfc reports and audit-stamp pending vocabulary; no new verb/flag |
| CLI grammar | N-A | Command keyword grammar unchanged |
| Editor autocomplete | N-A | No product editor input |
| Functional test for API | Yes | `test/ui/le-rfc-partial-proof.ci` reaches existing tooling entry points |
| Pipe completeness | Yes | CheckReport and published row models must expose partial in text/JSON/YAML/table |
| Env registration | N-A | No environment variable |
| Doctor dependencies | N-A | No new runtime path/service/binary; existing ledger files only |
| Prometheus metrics | N-A | Offline evidence counts, no daemon metrics |
| BGP family surface | N-A | No new SAFI/capability/attribute or BGP implementation |

### Documentation Update Checklist

| # | Question | Applies? | File / reason |
|---|----------|----------|---------------|
| 1 | User feature | N-A | No product capability; public evidence covered below |
| 2 | Config syntax | N-A | No config changes |
| 3 | CLI | Yes | `docs/contributing/rfc-conformance-gates.md`; le evidence output, not product command-reference |
| 4 | Product API/RPC | N-A | No product RPC; site JSON covered in website/AI |
| 5 | Plugin | N-A | No plugin changes |
| 6 | Guide | Yes | Conformance gates page is this tool's guide |
| 7 | Wire format | N-A | No wire changes |
| 8 | SDK/protocol | N-A | No SDK changes |
| 9 | RFC proof | Yes | Later -5/6 corpus application and source Meta disclosure; generated rfc-status never hand-edited |
| 10 | Test infrastructure | Yes | `docs/functional-tests.md`, `rfc/discrimination/README.md` scope interpretation |
| 11 | Daemon comparison | N-A | No new daemon capability |
| 12 | Architecture | Yes | `docs/architecture/core-design.md`, `website/AI.md` |
| 13 | Route metadata | N-A | No route state |
| 14 | Metrics | N-A | No Prometheus surface |
| 15 | Registered inventory | Yes | Annotation/verdict registries; gates guide and ai/INDEX discovery |
| 16 | Source anchors | Yes | Existing Design headers and gates/core-design/website anchors identified in source; implementation runs native citation inventory for the final changed set |
| 17 | Examples | Yes | Annotation/audit examples in gates guide, audit skill and functional-tests; existing command syntax retained |

### Discovery / Mechanical Checklist

| Question | Answer |
|----------|--------|
| Where an agent looks first | ai/INDEX RFC requirement coverage and audit rows gain partial/clause-scope keywords and link to the existing gates page |
| Rule preventing regression | Existing whole-sentence audit question plus explicit partial semantics in ze-rfc-audit; no undocumented exception to the 24-character quote guard |
| Registry preventing drift | AnnotationKinds and AuditVerdicts/AuditVerdictMeaning; site vocabulary completeness test and annotation partition tests |
| Proof of integration | CLI fixture plus publication/health arithmetic tests; no new verification stage, since existing rfc check owns these checks |

## Implementation Steps

1. **Wiring:** after parent approval, add isolated non-RFC9514 fixtures and the named check/stamp/publication wiring tests using existing commands. Their failure must identify unsupported partial representation, not unrelated corpus debt. No placeholder command or stub is needed because the entry points exist.
2. **Representation and enforcement:** add registered annotation parsing/validation, partial verdict preconditions, requirement-aware digest and finding transitions. Preserve generic hashes, tags and extraction schema. Update gates/audit docs beside these edits. Inputs are authored summary/audit plus existing evidence; output is validated scope with unchanged requirement identity.
3. **All consumers:** wire typed scope and subset counts through reports, shards, status, site and health. Prove numeric partitions and negative rendering cases, including invalid enforced data on a path not calling Check. Update source-anchored docs and discovery in the same phase.
4. **Corpus application and independent judgment:** parent assigns RFC9514 application to its ledger owner after tooling review. Apply Correction/Meta/extraction reason without changing ids or erasing proof. Independent judge confirms scoped assertions and records before stamping partial; inadequate assertions stay weak/wrong. Main executes consolidated verification and owns integration/closure.

### Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| No score inflation | Every partial row remains in denominator and outside whole-proven; site direct rendering cannot bypass it |
| Meaning versus observation | Native record is proof of a claim, not proof of origination; annotation cannot mechanically promote weak assertions |
| Freshness | Every add/change/remove scope edit invalidates audit, while unrelated non-partial records keep their fingerprints |
| No escape | Existing gap/test, quote, polarity, evidence-tier, extraction and discrimination guards unchanged |
| Public completeness | Scope + gap + full sentence visible in every format, worklist includes partial |
| Concurrency | No proof/audit write without existing ownership/lock; no generated ledger written by design author |

### Deliverables / Goal Validation

| Deliverable | Validation owed after implementation |
|-------------|--------------------------------------|
| Generic representation | Named parser/gate tests and native CLI fixture pass with an invented RFC number, proving no RFC9514 special case |
| No false whole proof | One-row counter matrix, enforced-on-partial refusal and direct publication controls pass |
| Durable meaning | Scope-edit freshness/reseal negatives and unchanged observation/extraction comparisons pass |
| Real target application | Independent row-by-row judgment of -5/6, retained receive evidence and visible missing native origination; no enforced whole sentence |
| Full integration | Main runs `./le verify worktree` over the integrated commit and records evidence; no run claimed in this design |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Authored input | Bounded scans over one parent row; invalid quote grammar cannot silently drop an obligation |
| File references | Reuse repo-contained producer-key validation; traversal/absolute/test producer refused |
| Output | Escape selector/reason at existing Markdown/HTML boundaries; no prose reparsing by consumers |
| False assurance | Malformed/stale/unsupported scopes and missing evidence never produce whole or scoped proof credit |

### Failure Routing

| Failure | Route |
|---------|-------|
| Selector cannot describe existing receive claim | Return to design; do not weaken whole-row quote guard |
| Independent audit finds inadequate receive assertion | BGP ledger/test owner repairs claim/test and records natively; verdict stays weak/wrong until then |
| Native origination becomes required scope | Separate owner capability decision; this tooling does not implement it |
| A consumer count disagrees | Fix shared producer or explicit partition consumer, never suppress the row |
| Shared file changes during implementation | Re-read and coordinate with owner; no stale overwrite |

## Key Design Decisions and Gate Challenges

| Challenge | Recommendation / answer |
|-----------|-------------------------|
| Scope | Do not widen this into universal per-clause test identities. One tested span plus explicit unmet span achieves the owner's example while keeping the whole row unproven. |
| Research | Existing annotation text is omitted from audit fingerprints. The scoped digest is required, not optional bookkeeping. |
| Simplicity | Registered annotation plus one verdict is smaller than child identity graphs and less ambiguous than permitting gap beside arbitrary tags. |
| Uniformity | Existing single coverage disposition, kind-specific parsed fields, independently stamped verdict and shared render producers are preserved. |
| Performance | Offline row parsing only; no packet/event allocations. Reuse normalized row and already loaded source/evidence; do not re-run tests at gate time. |
| Strongest design concern | A short locator cannot itself establish semantic completeness. Machine checks source membership; independent audit establishes meaning, and all unselected obligations conservatively stay unmet/unproven. |
| Genuine owner tradeoff | This proposal deliberately cannot compose several independently proved clauses into whole credit. Supporting that later would require a richer assignment model. No decision about implementing absent native EPE is implied or needed to approve this representation. |
| Independence test | Fields, transitions, consumers, migration constraints and ACs are specified here; no implementer needs the originating chat. The parent still owes interactive approval, and the independent judge still owes the actual evidence judgment. |

## Known Limitations

This changes evidence representation, not native SRv6 BGP EPE origination. That absence remains a public gap under the existing RFC9514 rows and the parent owner's scope. No guarantee of full semantic sentence partition can be obtained from substring matching, so partial never earns whole credit. Arbitrary per-tag clause composition is not part of this bounded contract.

## Review Gate

Reserved for the independent implementation review; no review has run in this design phase.

| Run | Scope | Reviewer lenses | Artifact | Outcome |
|-----|-------|-----------------|----------|---------|

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|

## Checklist

- [x] Parent reviews scope, research challenge, design alternatives and weakest point before implementation; Thomas approved on 2026-10-02.
- [ ] Every AC demonstrated by its named test or independent target-row evidence.
- [ ] Public and health partitions agree; extraction and observation identities preserved.
- [ ] Docs and discovery updated in the implementing phase, not postponed to closure.
- [ ] Independent review and consolidated `./le verify worktree` evidence recorded by main.
- [ ] No code, tests, formatters, builds or generated documents executed by this design phase.
