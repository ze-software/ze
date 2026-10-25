# Spec: rfc-priority-tree

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | Orders first-release RFC work by use-case profile and dependency tree, and publishes coverage per profile, so release planning closes the RFC gaps the release profiles reach before any leaf work |
| Depends | - |
| Phase | 0/5 |
| Handoff | - |
| Updated | 2026-10-25 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze implements over two hundred RFCs, and nothing says which gap to close first.
The public `Support` cell of each `rfc/short/<stem>.md` carries a hand-set section
and rank, which orders RFCs inside one area and says nothing about which RFC
another builds on, or which use case needs it.

The owner rule (`ai/META-DECISIONS.md`, Goal and scope): a release profile needs
every mandatory requirement (MUST, MUST NOT, SHALL, SHALL NOT, REQUIRED) of every
RFC it reaches covered before release; work moves from root RFCs to leaves,
except where a leaf updates a root's requirements; a requirement is covered only
when a recorded break proved its test goes red; optional requirements (SHOULD,
SHOULD NOT, RECOMMENDED, NOT RECOMMENDED, MAY, OPTIONAL) are a second figure,
improved after mandatory coverage reaches 100%.

The goal: a dependency DAG of the RFC corpus, profiles over it, a mandatory and
an optional coverage figure per RFC and per profile, a command that names the
next gap to close, and website pages that show each use case and each
technology aggregate at its percentages.

Agreed shape (owner, 2026-10-25):

| Element | Decision |
|---------|----------|
| DAG edges | Derived from the RFC texts, never hand-authored. "Builds on" = normative references restricted to RFCs in the corpus. "Updates" and "Obsoletes" = the RFC header lines |
| Invalidation exception | A row of an updated RFC that the updating RFC rewrites carries an `{updated-by: <updater requirement id>}` marker in the updated RFC's summary. It is ordered with the updater, not with the root. Applies to mandatory and optional rows alike |
| Profiles | The only authored input besides that marker, one small file each, in two kinds |
| Use-case profiles | List root RFCs and reach their closure over the DAG: route-server, route-collector, looking-glass, lns, vpn-concentrator, vpn-endpoint, ddos-flowspec (DDoS detection and mitigation by sending FlowSpec upstream), fttx-edge (an edge device connecting FTTx customers to the internet over L2TP, with local DNS resolution for the LAN) |
| Technology aggregates | Every RFC names its own aggregate in its summary; the aggregate rolls up one mandatory and one optional percentage over its members (MPLS, IPsec, IGP and the rest) |
| Release profiles | A profile file marks itself a release profile; the owner rule applies to those |
| Coverage | Numerator: rows whose covering tagged units carry a verified discrimination record. Denominator: the owner ruling of 2026-09-02 (`provenshare.go`): not-applicable and lower-layer rows stay in, `{rollup}` rows are out of both. An RFC with no valid extraction sign-off shows no number ("not walked") |
| Support cell | `<section> <rank>` becomes `<aggregate>`. The `-` declaration (no public row) stays. Support status, coverage and remaining stay editorial, with their existing gates. Order inside an aggregate: DAG level, then RFC number |
| Graph storage | Generated `rfc/graph.tsv` written by `./le rfc index-update`, under the same freshness check as `rfc/enrolled.txt` |
| Old combined References | 49 old RFCs have one References section; its informative citations become edges. Accepted: a profile may reach more RFCs than it needs, never fewer |
| Commands | `./le rfc tree` (DAG with both figures per node, optionally limited to one profile), `./le rfc next` (the next gap: lowest level with uncovered mandatory rows across the release profiles, or inside one named profile) |
| Website | One generated page per use-case profile and per aggregate under `/quality/rfc-compliance/profiles/`, showing both figures and the tree with a bar per RFC, built by `./le site build`. The existing RFC compliance index becomes the all-RFCs view |
| Roadmap | A new spec Meta field `RFC gaps` names the requirement ids or stems a spec closes; the roadmap derives a column naming the profiles they serve |

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - the gates every new check joins; "The superseded marker" is the model for `{updated-by}`
  → Constraint: a row marker is gated against a Meta link of the summary (`checkSuperseded` refuses a `{superseded}` marker on a summary with no `Obsoleted by`); `{updated-by}` is refused unless the updater is in the graph's Updated-by set for that stem
- [ ] `ai/skills/ze-rfc.md` step 6b - how a summary declares its public row today
  → Constraint: the skill text names `<section> <rank>`; it changes in the same phase as the parser
- [ ] `rfc/extraction/README.md` - the extraction sign-off
  → Constraint: only `signoff.go:evaluateExtractions` decides a VALID sign-off; `artifact.go:LoadExtractions` only parses. "Walked" is read from the former
- [ ] `docs/contributing/gh-pages.md`, `website/AI.md` - site generation
  → Constraint: `website/use-cases/` holds authored pages; no generator writes there. Generated RFC pages live under `/quality/rfc-compliance/`, and `removeRetiredRFCPages` deletes only pages carrying `rfcDetailMarker`, so a new page family carries its own marker
- [ ] `ai/doors/le-tooling.md` - the door for `internal/le`
  → Constraint: no private JSON mode; an action returns structured rows and `| json | yaml | table` come from the shared renderer

**Key insights:**
- The section+rank cell's only jobs are page order (`placeRows`) and table shape (`Brief`); everything else reads `HasRow()`
- No function counts rows proven by a discrimination record; the per-row proof exists in `rfcLedgerRequirement.Covers[].Proof`
- Mandatory = `gatedLevels` (`internal/le/rfc/rfc.go`) exactly; 4548 rows. Optional = the other six keywords; 1873 rows
- Normative references parse from 250 of 257 texts; 752 of 2009 references resolve inside the corpus; 10 citation cycles exist
- No marker exists today for a row an updating RFC rewrites

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/meta.go` (`readSupport`, `checkRowCells`, `Meta`, `rowsFrom`) - parses `Support` as `-` or `<section> <rank>`; `Area/Status/Coverage/Remaining` moved verbatim; `rowsFrom` makes Coverage stand in for Remaining on a `Brief` section
  → Constraint: `Support` is `-` or `<section> <rank>`; the section also picks the public table's shape (`checkRowCells`), so the aggregate must carry the `Brief` answer
- [ ] `internal/le/rfc/render_ledger.go` (`statusSections`, `statusSection`, `statusSectionKeys`, `placeRows`, `renderStatusPage`, `proofCell`) - 10 hard-coded section keys with Heading, Intro, `Brief`; `placeRows` refuses a shared rank and sorts by rank; `renderStatusPage` writes `docs/features/rfc-status.md`
  → Constraint: Rank's only enforcement is `placeRows`. The `statusSections` literal is a central enumeration and is deleted, not kept beside the aggregate files
- [ ] `internal/le/rfc/provenshare.go` (`ProvenShareOf`, `ProvenShare`) - corpus-wide proven share
  → Constraint: owner ruling 2026-09-02: not-applicable and lower-layer rows STAY in the denominator; only a whole RFC Ze does not implement leaves it; `{rollup}` is out of both. Profile coverage uses this denominator
- [ ] `internal/le/rfc/coverage.go` (`CoverageRows`) - per-RFC polarity buckets from tags only
  → Decision: not reused for the numerator; it counts tags, not discrimination proofs
- [ ] `internal/le/site/rfcledger.go` (`publishRFCLedger`, `rfcLedgerCoverageOf`, `rfcProofsByCover`) - writes `data/rfc-requirements.json`; per-row proofs in `Covers[].Proof` (`Verified`, `Proves`)
  → Decision: the new per-row "proven" predicate is lifted out of `rfcProofsByCover` into `internal/le/rfc` so the ledger, `tree`, `next` and the site share one answer
- [ ] `internal/le/rfc/check_core.go` (`checkSuperseded`) - `{superseded: ...}` markers exist only on a summary with `Obsoleted by` (325 markers, 8 summaries)
  → Constraint: `rfc/short/rfc7606.md` names no `RFC4271-` id; `{updated-by}` is new data authored in Phase 1
- [ ] `internal/le/rfc/signoff.go` (`evaluateExtractions`) - valid sign-offs by stem
- [ ] `internal/le/rfc/rfc.go` (`gatedLevels`) - MUST, MUST NOT, SHALL, SHALL NOT, REQUIRED
  → Decision: "mandatory" is `gatedLevels`, not a second list
- [ ] `internal/le/rfc/actions.go` (`actions`) - the `./le rfc` action table driving dispatch, help, grammar
  → Decision: `tree` and `next` are rows here
- [ ] `internal/le/site/build.go` (`refreshNativeSurfaces`), `site/rfccompliance.go` (`renderRFCCompliance`), `site/producer.go` (`registerProducer`) - stage 1 publishes the ledger JSON, stage 2 producers render pages
  → Decision: profiles and the graph join the ledger JSON; a new producer `rfc-profiles` renders the pages
- [ ] `internal/le/spec/roadmap/roadmap.go` (`Collect`), `internal/le/spec/status/specstatus.go` (`parseMetadata`) - roadmap rows from spec Meta: Priority, Release reason, Status, Depends, Phase, Updated
  → Constraint: no spec carries a machine-readable RFC link today; 23 summaries cite a spec path in prose, which is not parsed

**Behavior to preserve:**
- Every summary that has a public row today has one after migration, under the heading its aggregate declares, with the same Status, Coverage, Remaining and Proof cells
- The gap-count agreement, gap disclosure and unproven-support gates read the same cells as today
- `ProvenShareOf` and the published corpus-wide share are unchanged
- `{superseded}` markers and `checkSuperseded` unchanged
- `| json | yaml | table` behaviour of every existing `./le rfc` action

**Behavior to change:**
- `Support | <section> <rank>` becomes `Support | <aggregate>`; the `statusSections` literal and the rank are deleted
- The 10 section keys become aggregate files; `igp-mpls` splits into `isis`, `ospf`, `mpls`; `ipsec` keeps IKEv2/IPsec and gains `eap`; the 36 summaries without a row keep `-` and still name an aggregate for profile rollups (see AC-4)
- Summary Meta `Updates`/`Updated by`/`Obsoletes`/`Obsoleted by` cells are checked against the RFC headers; the 24 disagreements found are corrected to the header

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `rfc/full/*.txt` and `rfc/drafts/*.txt` (RFC texts: reference sections, header lines)
- `rfc/short/<stem>.md` (Meta `Support` aggregate, requirement rows, `{updated-by}` markers)
- `rfc/profiles/<name>.md` (profile Meta table: Kind, Title, Release, Roots or Heading/Intro/Shape)
- tags and `rfc/discrimination/`, `rfc/extraction/` (existing evidence)
- spec Meta `RFC gaps` field

### Transformation Path
1. `./le rfc index-update` parses every text into edges (builds-on, updates, obsoletes), redirects edges to obsoleted stems to their successor, drops RFC 2119/8174 and self-edges, and writes `rfc/graph.tsv`
2. `rfc.Collect` loads the graph, the profiles and the summaries; the graph collapses citation cycles into components and assigns each component a level (0 = no in-corpus dependency, else 1 + the highest dependency level)
3. The coverage function answers per stem: mandatory proven and denominator, optional proven and denominator, walked or not; rows carrying `{updated-by}` are attributed to the updater's level for ordering, and counted in their own RFC's figure
4. Profile closure = roots plus everything reachable by builds-on and updated-by edges; profile figures are sums over the closure
5. `tree` and `next` render rows from steps 2-4; `renderStatusPage` orders rows by aggregate then level then RFC number
6. `publishRFCLedger` adds graph, profiles and figures to `data/rfc-requirements.json`; the `rfc-profiles` producer renders pages from it
7. `./le spec roadmap` reads `RFC gaps`, maps each id to its stem, and lists the profiles whose closure holds it

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| repo files → `internal/le/rfc` | text parse at index-update, TSV read at Collect | No |
| `internal/le/rfc` → `internal/le/site` | ledger JSON in `data/rfc-requirements.json` | No |
| `internal/le/spec` → `internal/le/rfc` | roadmap calls the profile closure API | No |

### Integration Points
- `rfc.Collect` - gains graph and profiles
- `rfc/actions.go` table - gains `tree` and `next`
- `render_ledger.go` - order from aggregate files and levels
- `site/producer.go` - new `rfc-profiles` producer
- `spec/status/specstatus.go:parseMetadata` - new `RFC gaps` field

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | site reads only the ledger JSON, as today |
| No unintended coupling (components stay isolated) | Yes | roadmap calls one exported closure function of `internal/le/rfc` |
| No duplicated functionality (extends existing, does not recreate) | Yes | the proven predicate moves out of `site/rfcledger.go` into `internal/le/rfc`, one copy |
| Zero-copy preserved where applicable (refs, not copies) | N-A | build-time tooling |
| Registration over hardcoding, outbound | Yes | a new aggregate or profile is a new file under `rfc/profiles/`; no Go edit |
| Registration over hardcoding, inbound | Yes | `statusSections` (render_ledger.go) is deleted and derived from `rfc/profiles/`; searched: `statusSectionKeys`, `checkRowCells`, `placeRows`, `rowsFrom`, fixtures carrying `bgp-base 10` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Normative references are extractable from column-0 headings | research 2026-10-25: 201 separate, 49 combined, 7 special cases | Missing edges make a profile reach too few RFCs | parser test over the whole corpus asserting every text yields a section or is on a named no-references list | unvalidated |
| A-2 | Optional rows can carry tags and discrimination records like mandatory rows | `rfc/short` rows carry SHOULD/MAY levels; tags bind by requirement id | Optional figure always 0 | grep tags naming a SHOULD row id; one fixture | unvalidated |
| A-3 | The RFC header is the authority over summary Meta links | owner gate 2026-10-25; research found 24 disagreements | Wrong edges | the 24 rows reviewed one by one in Phase 2 | unvalidated |
| A-4 | Collapsing a citation cycle into one level is acceptable ordering | research: 10 components, mostly genuine pairs (4301/4303, 2865/2866) | `next` names a component member arbitrarily | `next` names the whole component, members by RFC number | unvalidated |
| A-5 | Drafts are nodes like RFCs | 15 drafts parse the same way; none cites another corpus draft | Draft edges missing | parser test on the drafts | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The Support migration touches 204 summaries that other sessions edit daily | commit refuses or carries foreign hunks | a one-shot migration action rewrites only the Support cell; commit per `hunks` where a summary carries foreign hunks |
| R-2 | Page order changes publicly | `docs/features/rfc-status.md` diff reorders rows | expected; the row SET is asserted unchanged (AC-6) |
| R-3 | Over-reach from combined References inflates profiles | a profile closure holds RFCs unrelated to its use | accepted by owner; `tree profile <name>` shows why each RFC is in (the edge path) |
| R-4 | `{updated-by}` markers are authored and can be wrong | a marker on a row the updater does not touch | gate against the header link; marker names the updater's requirement id, which must exist |
| R-5 | Stale `rfc/graph.tsv` after a new text lands | freshness check red | `./le rfc index-update` regenerates; check names the command |
| R-6 | Low published numbers read as regressions | readers compare with the status page | each page states the numerator definition and links the 2026-09-02 denominator rule |
| R-7 | A profile root missing from the corpus silently drops | closure smaller than intended | a root must exist in `rfc/short` or be listed as `absent` with its RFC number, rendered as an uncovered node |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Tooling and published pages only: wrong ordering or figures on the website and in `next`; no shipped binary changes |
| How is it reverted? | Single commit revert per phase; the Support migration commit is mechanical and reverts as one |
| Who else touches this path? | Every session enrolling or proving RFC rows edits `rfc/short/`; `internal/le/rfc` and `internal/le/site` are edited by the RFC gate and site work |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc index-update` | → | graph parser writes `rfc/graph.tsv` | `TestIndexUpdateWritesGraph` (`internal/le/rfc/write_test.go`) |
| `./le rfc tree` | → | tree answer over fixture corpus | `TestTreeActionRendersLevelsAndFigures` (`internal/le/rfc/tree_test.go`) |
| `./le rfc next profile lns` | → | next answer | `TestNextActionNamesLowestUncoveredLevel` (`internal/le/rfc/next_test.go`) |
| `./le rfc check` | → | `checkUpdatedBy`, `checkHeaderLinks`, profile parse | `TestCheckRefusesUpdatedByWithoutHeaderLink`, `TestCheckRefusesMetaLinkHeaderMismatch` (`internal/le/rfc/check_test.go`) |
| `./le site build` | → | `rfc-profiles` producer | `TestRFCProfilePagesRendered` (`internal/le/site/rfcprofiles_test.go`) |
| `./le spec roadmap` | → | `RFC gaps` column | `TestRoadmapProfilesColumnFromRFCGaps` (`internal/le/spec/roadmap/roadmap_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le rfc index-update` over the corpus | `rfc/graph.tsv` holds one row per in-corpus edge with kind builds-on, updates or obsoletes; no RFC 2119/8174 edge; no self-edge; an edge to an obsoleted stem points at its successor |
| AC-2 | Fixture texts with TOC duplicates, `RFC` wrapped to the next line, `RFC-NNNN`, combined References, header continuation lines, prose "This memo obsoletes" | each yields exactly the expected edges |
| AC-3 | Fixture with a citation cycle A↔B and C citing A | A and B share one level; C's level is one above |
| AC-4 | Summary `Support` cell naming an aggregate with no `rfc/profiles/<name>.md`, or `<section> <rank>` | `./le rfc check` refuses, naming the file and the valid aggregates; a `-` summary still names its aggregate in a separate `Aggregate` cell so it joins rollups |
| AC-5 | `{updated-by: RFC7606-3-1}` on a row of rfc4271 | accepted when rfc7606 is in rfc4271's header Updated-by set and RFC7606-3-1 exists; refused otherwise with the reason |
| AC-6 | Support migration applied | `docs/features/rfc-status.md` renders the same set of rows as before (same stems, same cells), grouped under aggregate headings, ordered by level then RFC number |
| AC-7 | Per-RFC coverage on a fixture with mandatory rows proven, tagged-unproven, not-applicable and rollup | mandatory figure = proven over (all gated rows minus rollup); not-applicable stays in the denominator; optional figure computed the same way over optional rows |
| AC-8 | RFC with no valid extraction sign-off | its figures render "not walked"; profile rollups state how many RFCs in the closure are not walked |
| AC-9 | Profile file with an unknown root stem, not marked `absent` | `./le rfc check` refuses, naming the profile and the stem |
| AC-10 | `./le rfc next profile lns` on a fixture | names the lowest-level RFC in the closure with an uncovered mandatory row, its uncovered requirement ids, and why it is in the closure (edge path); a row marked `{updated-by}` is reported at the updater's level |
| AC-11 | `./le rfc next` with no profile | the same answer across every release profile, naming the profiles each RFC serves |
| AC-12 | `./le rfc next profile <p>` when the closure's mandatory figure is 100% | names the next optional gap, saying mandatory is complete |
| AC-13 | `./le rfc tree profile route-server \| json` | structured rows: stem, title, level, aggregate, builds-on, updated-by, mandatory proven/total, optional proven/total, walked |
| AC-14 | Summary Meta `Updates` cell that disagrees with the RFC header | `./le rfc check` refuses, naming both values; the 24 disagreements found on 2026-10-25 are corrected |
| AC-15 | `./le site build` | one page per profile and per aggregate under `/quality/rfc-compliance/profiles/`, each with both figures, the not-walked count, and the tree with a bar per RFC; an index page lists all profiles with release profiles first; `./le site check` passes |
| AC-16 | Spec with `RFC gaps | RFC2661-5.4-1, rfc7947` | the roadmap row carries a Profiles cell naming every profile whose closure holds rfc2661 or rfc7947; an unknown id is refused by the spec parser |
| AC-17 | The eight use-case profiles and the aggregates committed | each parses; route-server, route-collector, looking-glass, lns, fttx-edge and vpn-endpoint are marked release; vpn-concentrator and ddos-flowspec are not (owner, 2026-10-25) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNormativeReferenceParse` | `internal/le/rfc/graph/parse_test.go` | AC-2 fixtures | |
| `TestHeaderLinksParse` | `internal/le/rfc/graph/parse_test.go` | header Updates/Obsoletes forms, AC-2 | |
| `TestEveryCorpusTextYieldsReferences` | `internal/le/rfc/graph/corpus_test.go` | A-1: every text has a section or is on the named no-references list | |
| `TestGraphRedirectsObsoleted` | `internal/le/rfc/graph/graph_test.go` | AC-1 | |
| `TestGraphCycleSharesLevel` | `internal/le/rfc/graph/graph_test.go` | AC-3 | |
| `TestIndexUpdateWritesGraph` | `internal/le/rfc/write_test.go` | AC-1 wiring and freshness | |
| `TestSupportNamesAggregate` | `internal/le/rfc/meta_test.go` | AC-4 | |
| `TestCheckUpdatedByHoldsThePointerAgainstTheUpdater`, `TestAMalformedUpdatedByMarkerIsRefused`, `TestAnUpdatedByMarkerParsesItsTargetAndItsReason`, `TestTheUpdatesCellNamesTheStemsItUpdates`, `TestAnUpdatedByMarkerChangesNoCount` | `internal/le/rfc/updatedby_test.go` | AC-5 | |
| `TestCheckRefusesMetaLinkHeaderMismatch` | `internal/le/rfc/check_test.go` | AC-14 | |
| `TestStatusPageRowSetUnchanged` | `internal/le/rfc/render_ledger_test.go` | AC-6 | |
| `TestCoverageMandatoryOptional` | `internal/le/rfc/profilecoverage_test.go` | AC-7, AC-8 | |
| `TestProfileUnknownRootRefused` | `internal/le/rfc/profile_test.go` | AC-9 | |
| `TestNextActionNamesLowestUncoveredLevel` | `internal/le/rfc/next_test.go` | AC-10, AC-11, AC-12 | |
| `TestTreeActionRendersLevelsAndFigures` | `internal/le/rfc/tree_test.go` | AC-13 | |
| `TestRFCProfilePagesRendered` | `internal/le/site/rfcprofiles_test.go` | AC-15 | |
| `TestRoadmapProfilesColumnFromRFCGaps` | `internal/le/spec/roadmap/roadmap_test.go` | AC-16 | |
| `TestCorpusProfilesParse` | `internal/le/rfc/profile_test.go` | AC-17 over the real `rfc/profiles/` | |

Existing tests updated in Phase 1 (fixture syntax `bgp-base 10` → aggregate): `render_test.go`, `site/rfcdetail_test.go`, `rfc/fixture.go`, `selftest_core.go`, `health/collect_rfc_test.go`, `check_test.go` `TestOneSummaryRendersExactlyOneStatusRow` and `TestSupportedRowScopeFixtures` (the shared-rank refusal is deleted with the rank; the omission, duplication, substitution and wrong-section refusals stay). This is a fixture syntax change, not a weakening: each assertion keeps its claim.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| RFC number in a reference | 1-99999 | 99999 | 0 | N/A (unbounded digits refused above 5) |
| Coverage denominator | 0-N | 1 | 0 renders "no mandatory rows", never 0% or 100% | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestRFCTreeOverRealCorpus` | `internal/le/rfc/corpus_test.go` | the real corpus builds a graph with no unknown aggregate, every profile resolves, `next` answers for every release profile | |
| `./le site check` | site pipeline | generated profile pages pass the site checks | |

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer.

## Files to Modify
- `internal/le/rfc/meta.go` - `Support` holds an aggregate; new `Aggregate` cell for `-` summaries
- `internal/le/rfc/render_ledger.go` - delete `statusSections` and rank ordering; order by aggregate, level, number
- `internal/le/rfc/check_core.go` - `checkUpdatedBy`, `checkHeaderLinks`
- `internal/le/rfc/check.go`, `internal/le/rfc/write.go`, `internal/le/rfc/register.go` - graph in index-update and Collect
- `internal/le/rfc/actions.go` - `tree`, `next`
- `internal/le/site/rfcledger.go` - lift the proven predicate; add graph, profiles, figures to the ledger JSON
- `internal/le/site/build.go` - register the producer path
- `internal/le/spec/status/specstatus.go` - `RFC gaps` field
- `internal/le/spec/roadmap/roadmap.go`, `internal/le/site/roadmap.go` - Profiles column
- `rfc/short/*.md` - Support cell migration; 24 Meta link corrections; `{updated-by}` markers (rfc4271 rows rewritten by rfc7606 as the first set)
- `ai/skills/ze-rfc.md` - step 6b
- `docs/contributing/rfc-conformance-gates.md` - graph, profiles, `{updated-by}`, coverage figures
- `rfc/README.md`, `ai/INDEX.md`, `plan/templates/TEMPLATE.md` (`RFC gaps` Meta row)
- `docs/architecture/core-design.md` - its section on the RFC artifacts `./le rfc index-update` writes ("the five RFC artifacts", `SessionStartDefer`) gains `rfc/graph.tsv` (Phase 2)
- `docs/contributing/spec-workflow.md` - the metadata table gains the `RFC gaps` field and the roadmap Profiles column (Phase 5)
- `docs/contributing/rfc-implementation-guide.md` - any `<section> <rank>` Support example (Phase 1)

## Files to Create
- `internal/le/rfc/graph/` - reference and header parser, graph, components, levels, closure
- `internal/le/rfc/profile.go` - profile file parser
- `internal/le/rfc/profilecoverage.go` - per-RFC and per-profile figures
- `internal/le/rfc/tree.go`, `internal/le/rfc/next.go` - the two answers
- `internal/le/site/rfcprofiles.go` - `rfc-profiles` producer
- `rfc/profiles/<name>.md` - eight use-case profiles and one file per aggregate
- `rfc/graph.tsv` - generated

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | tooling, no daemon config |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | Yes | `internal/le/rfc/actions.go` (`tree`, `next`) |
| CLI grammar (keyword before value) | Yes | `next profile <name>`, `tree profile <name>` per `ai/rules/cli.md` |
| Editor autocomplete | Yes | `profile` value completes from `rfc/profiles/` through the action parameter |
| Functional test for new RPC/API | N-A | no RPC; Go tests over the real corpus cover the commands |
| Pipe completeness | Yes | structured rows through the shared renderer |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no runtime dependency in the shipped binary |
| Prometheus counters/metrics | N-A | build-time tooling |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | developer tooling, not a Ze feature |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | Yes | `./le rfc` help is generated from the action table; `docs/contributing/rfc-conformance-gates.md` gains a section on `tree` and `next` |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | none |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/*.md` Support cells and Meta links; `docs/features/rfc-status.md` regenerated |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/contributing/rfc-conformance-gates.md`, `rfc/README.md`, `ai/skills/ze-rfc.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/release/delivery/spec-rfc-priority-tree.md` at each phase start and name every result |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | every `<section> <rank>` example in `ai/skills/ze-rfc.md` and `docs/contributing/rfc-conformance-gates.md` |

## Implementation Steps

Five phases, one commit each; each phase updates its pages before the next starts.

1. **Phase 1: model** -- profile files, aggregate files, `Support | <aggregate>` migration, `{updated-by}` marker and check, coverage function
   - Tests: `TestSupportNamesAggregate`, `TestCheckRefusesUpdatedByWithoutHeaderLink`, `TestStatusPageRowSetUnchanged`, `TestCoverageMandatoryOptional`, `TestProfileUnknownRootRefused`, `TestCorpusProfilesParse`
   - Files: `meta.go`, `render_ledger.go`, `check_core.go`, `profile.go`, `profilecoverage.go`, `rfc/profiles/`, `rfc/short/*.md` (Support cell only), `ai/skills/ze-rfc.md`
   - Wiring first: `./le rfc check` parses `rfc/profiles/` and refuses an unknown aggregate before the migration lands
   - The `{updated-by}` header check reads Meta links in this phase and switches to the graph in Phase 2
2. **Phase 2: graph extraction** -- parser, `rfc/graph.tsv`, header-vs-Meta check, 24 corrections
   - Tests: `TestNormativeReferenceParse`, `TestHeaderLinksParse`, `TestEveryCorpusTextYieldsReferences`, `TestGraphRedirectsObsoleted`, `TestGraphCycleSharesLevel`, `TestIndexUpdateWritesGraph`, `TestCheckRefusesMetaLinkHeaderMismatch`
3. **Phase 3: tree and next** -- the two actions, level ordering on the status page
   - Tests: `TestTreeActionRendersLevelsAndFigures`, `TestNextActionNamesLowestUncoveredLevel`, `TestRFCTreeOverRealCorpus`
4. **Phase 4: website** -- ledger JSON gains graph, profiles, figures; `rfc-profiles` producer; `./le site build` and `./le site check` on `../gh-pages`
   - Tests: `TestRFCProfilePagesRendered`
5. **Phase 5: roadmap** -- `RFC gaps` field, Profiles column, template row
   - Tests: `TestRoadmapProfilesColumnFromRFCGaps`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | Mandatory set is `gatedLevels` read once, never a second list; denominator matches `ProvenShareOf`'s rule |
| Correctness | An RFC with no sign-off never renders a percentage, including 0% |
| Data flow | Site renders only from the ledger JSON; `tree`, `next`, site and roadmap share one coverage function |
| Rule: no-layering | No path accepts `<section> <rank>` after Phase 1; `statusSections` is gone |
| Rule: principles (declare once) | Aggregate membership lives only in the summary; profile roots only in the profile file; edges only in the generated graph |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `rfc/graph.tsv` fresh | `./le rfc index-update` produces no diff |
| No `<section> <rank>` Support cell | `grep -E '^\| Support \| [a-z-]+ [0-9]+ \|' rfc/short/*.md` returns nothing |
| Profiles parse | `./le rfc check` green on profiles |
| `next` answers | `./le rfc next` names an RFC for each release profile |
| Website pages | `./le site check` passes; pages exist under `/quality/rfc-compliance/profiles/` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | RFC texts and drafts are fetched from the IETF; the parser bounds RFC numbers to 5 digits and refuses unknown stems rather than inventing nodes |
| Output escaping | profile titles and RFC titles are HTML-escaped by the page shell |

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

## Research gate decisions (owner, 2026-10-25)

| Question | Decision |
|----------|----------|
| Recording rows an updating RFC rewrites | A row marker on the updated RFC's summary, gated against the header Updated-by links. It applies to every requirement row a test covers, mandatory and optional |
| Combined References in 49 old RFCs | Accept the over-reach: a profile may reach more RFCs than it needs, never fewer. No exclusion list |
| Strict percentages | Intended: discrimination-proven numerator, 2026-09-02 denominator. Profile pages will read lower than today's status page |
| Graph storage | Generated `rfc/graph.tsv` |
| Membership | Each RFC names its aggregate in its summary |
| Levels reported | Mandatory and optional as two figures; mandatory first to 100%, then optional |

## Research findings (2026-10-25)

| Finding | Evidence | Consequence |
|---------|----------|-------------|
| Normative references parse from `rfc/full` | 201 of 257 texts have a separate Normative section; 49 have only a combined References; 7 special cases (rfc1035, rfc791, rfc905, rfc1071; rfc5301, rfc5396, rfc7457 have none) | Combined sections over-count edges (accepted) |
| Edge yield | 2009 references, 752 resolve to a corpus stem | Most edges leave the corpus and are dropped |
| Cycles | 10 components: 4301/4302/4303, 5881/5882/5883, 2205/2210/2211/2215, 2347/2348/2349, 3031/3032, 6810/6811, 2865/2866, 4576/4577, 7011/7012, 7534/7535 | Collapsed to one level each |
| Header Updates/Obsoletes | 65 texts carry Updates, 80 Obsoletes; continuation lines, `RFCs`/`IENs` prefixes, author column on the same line, 62 old texts state it in prose | Header is the authority; Meta disagrees in 8 pairs and omits in 16 |
| Obsoleted targets | edges to 1771, 5575, 4893 | Redirect to the successor |
| Parser traps | TOC lines repeat headings (column 0 only), `RFC` wrapped in 148 files, `RFC-NNNN` in 10 files, draft version suffixes | Each is a parser fixture |
| Requirement levels | 4548 mandatory rows (`gatedLevels`), 1873 optional | Two figures |
| Product gaps the profiles will show | IKE virtual-IP pool has no non-test caller (`ike/engine/apply.go`, owned by `spec-ike-virtual-ip-assignment`); no LAN DNS resolver (`internal/core/dnsserver` serves only as112 and geodns); `dhcpserver` IPv4 only; NAT and NTP have no enrolled RFC | Profiles show them as uncovered or absent nodes |

## First `{updated-by}` set: rfc4271 rows RFC 7606 rewrites

Mapped 2026-10-25 from `rfc/full/rfc7606.txt` against `rfc/full/rfc4271.txt` Section 6.3; quotes verified at the RFC text. The marker carries the updater requirement id only; the kind column is for the reader and is not marked.

| 4271 row | Marker names | RFC 7606 section | Kind |
|----------|--------------|------------------|------|
| RFC4271-6.3-1 | RFC7606-3.a-1 | 3.a | narrowed |
| RFC4271-6.3-5 | RFC7606-3.c-1 | 3.c | replaced |
| RFC4271-6.3-6 | RFC7606-3.e-1 | 3.e, 3.f, 7.7 | replaced for ORIGIN, AS_PATH, NEXT_HOP, MED, LOCAL_PREF, ATOMIC_AGGREGATE, AGGREGATOR |
| RFC4271-6.3-7 | RFC7606-3.d-1 | 3.d | replaced |
| RFC4271-6.3-9 | RFC7606-7.1-1 | 7.1 | replaced |
| RFC4271-6.3-10 | RFC7606-7.3-1 | 7.3 | replaced |
| RFC4271-6.3-12 | RFC7606-7.2-1 | 7.2 | replaced |
| RFC4271-6.3-13 | RFC7606-7.2-2 | 7.2 | relaxed (MUST reset becomes SHOULD withdraw, MAY reset) |
| RFC4271-6.3-14 | RFC7606-3.e-1 | 3.e, 3.f | narrowed (MED and AGGREGATOR leave the row) |
| RFC4271-6.3-15 | RFC7606-3.g-2 | 3.g | narrowed (MP_REACH/MP_UNREACH keep the reset) |
| RFC4271-6.3-16 | RFC7606-3.i-1 | 3.i, 5.3 | strengthened (Withdrawn Routes checked too; reset kept per 3.j) |
| RFC4271-5-7 | RFC7606-5.1-1 | 5.1 | narrowed (MP attribute first) |
| RFC4271-4.3-6 | RFC7606-5.1-2 | 5.1 | strengthened (sender side) |

→ Decision: the marker's syntax allows one updater id per row; where several 7606 rows rewrite one 4271 row (6.3-6, 6.3-14), it names the governing one (3.e-1), and the others stay reachable through the updater's own rows.
→ Decision: not marked: 6.3-4 (3.b "remains unchanged"), 6.3-8, 6.3-2/6.3-3/6.3-11 (already SHOULD ignore, outside 3.e's "cases that specify a session reset"), 6.3-17, 4.3-7, and 5.1.5-3 (same effect under 7.5; 7606 drops 4271's confederation exception, which is a difference in a lower-case reading, not a rewritten obligation).
→ Constraint: RFC 7606 Section 5.2 forces "session reset" for an UPDATE with attribute errors and no reachable NLRI, which limits every "replaced" row above; RFC7606-5.2-1 carries it, so the updater's rows, not the marker, hold that limit.

## Initial profiles (proposed roots, existing stems)

| Profile | Release | Roots | Absent (listed as `absent`) |
|---------|---------|-------|-----------------------------|
| route-server | yes | 4271, 4760, 6793, 7947, 9234, 1997, 8092, 7999, 6811, 8210, 9582, 7911, 7454 | 7948, 8212, 8893 |
| route-collector | yes | 4271, 4760, 6793, 7911, 7854, 8671, 9069, 6396, 8050, 6397, 7606 | - |
| looking-glass | yes | 4271, 7911, 1997, 8092, 6811 | 8522 |
| lns | yes | 2661, 1661, 1332, 1877, 5072, 1334, 1994, 2759, 2865, 2866, 2869, 5176 | 1570, 2867, 2868, 3162, 4818, 6911, 3145, 4861 |
| vpn-concentrator | no | 7296, 4301, 4303, 3948, 4555, 5282, 3748, 5216, 7427, 9190 | 8247, 8221, 7383, 4106, 7634, 8031, 4754, 5280, 9370, 5998, 4739, 5685, 9329 |
| vpn-endpoint | yes | 7296, 4301, 4302, 4303, 3948, 5282, 7427, 6071 | 8247, 8221, 7383, 4106, 7634, 8031, 4754, 5280, 9370 |
| ddos-flowspec | no | 4271, 4760, 8955, 8956, 9117, 7999, 4360, 7011, 7012, 3954, sflow-v5 | 5635, 3882 |
| fttx-edge | yes | 2661, 2516, 1661, 1332, 5072, 1877, 1994, 2131, 2132, 9915, 1035, 2181, 4035, 4862 | 7084, 3022, 4787, 5382, 5905, 8106, 4638, 4861 |

Aggregates (each RFC names one): bgp-core, bgp-session-ops, bgp-transport-security, bgp-communities-policy, bgp-vpn-evpn, bgp-flowspec, bgp-ls, bgp-sr-tunnel, bgp-monitoring, rpki, bfd, isis, ospf, mpls, ikev2-ipsec, eap, ppp-access, radius-tacacs, oauth-mgmt, dns, dhcp-provisioning, flow-telemetry, route-archival, fhrp, management, tunnels, primitives, drafts. Membership per research 2026-10-25; the 36 summaries without a row are assigned in Phase 1.

## Design Insights

- The marker is `{updated-by: <ID>; why}`: the reason is mandatory, as on `{superseded}`, because a pointer nobody explained is a pointer nobody checked. One `{updated-by}` per row, composing with one coverage annotation and one `{superseded}` (`stripMarkers`, `rowMarkers`).
- Until the graph lands, AC-5's "header Updated-by set" is read from the UPDATER's Meta `Updates` cell (`Meta.Updates`, `updatesFrom`), not from the updated summary's `Updated by` cell: the updater's own header carries the `Updates:` line, while the updated RFC's published text predates its updaters (`rfc/full/rfc4271.txt` names no RFC 7606). `updatesFrom` refuses a cell item that names no RFC, so `rfc/short/rfc2966.md`'s prose cell was corrected to its header (None).

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Edges derived from RFC texts | Hand-authored dependency lists | The references are a fact in the text; a hand list is a second declaration |
| Generated `rfc/graph.tsv` | Re-parse on every command | Reviewable diffs, fast commands, matches `enrolled.txt` |
| RFC names its aggregate | Aggregate lists its RFCs | A new RFC registers itself; no central list to edit |
| `{updated-by}` row marker | Whole-RFC ordering only | Owner: what matters are the statements tests cover; a root can reach 100% without its updaters |
| Mandatory = `gatedLevels` | A new keyword list | One declaration of what is mandatory |
| Denominator per 2026-09-02 | "Applicable rows only" | Owner ruling in force; annotations must not raise the score |
| Accept combined-References over-reach | Per-RFC informative exclusion list | Error in the safe direction, nothing to author |

## Known Limitations
- RFCs absent from the corpus appear as uncovered nodes; fetching and summarising them is per-RFC work under `ai/skills/ze-rfc.md`, ordered by `next`
- The product gaps the profiles expose (LAN DNS resolver, DHCPv6 server, NAT and NTP RFCs) are features with their own specs, not this one

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
