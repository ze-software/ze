# Handoff: rfc-priority-tree, Phase 1 onwards

**Spec:** `plan/release/delivery/spec-rfc-priority-tree.md` (Status in-progress, claimed by the 2026-10-25 session; the next session claims it again with `./le spec claim spec plan/release/delivery/spec-rfc-priority-tree.md`)
**Branch:** main
**Goal:** Order RFC coverage work by use-case profile and by a dependency tree derived from the RFC texts, report a mandatory and an optional coverage figure per RFC, per profile and per technology aggregate, name the next gap with `./le rfc next`, and publish one website page per profile and aggregate.

The owner rule behind it is in `ai/META-DECISIONS.md`, Goal and scope ("RFC work is ordered by use-case profile and by the RFC dependency tree"). Every design decision is in the spec's "Agreed shape", "Research gate decisions" and "Key Design Decisions" tables; do not reopen them.

## Phase 5 status (2026-10-26)

Landed (one commit, subject "spec status: RFC gaps Meta field"): `rfcGaps` and `rfcGapsWarn` (`internal/le/spec/status/specstatus.go`) split the `RFC gaps` cell, answer `[]` for `-` or an absent row, and warn on an empty or repeated item; `Spec.RFCGaps` (`report.go`) and `roadmap.Item.RFCGaps` (`roadmap.go`) carry it as `rfc-gaps`. Test `TestParseReadsTheRFCGapsField`; `TestInventoryIsStructuredDataWithKebabCaseKeys` now pins `rfc-gaps` among the record keys. Template row `| RFC gaps | - |` (`plan/templates/TEMPLATE.md`), `docs/contributing/spec-workflow.md` (metadata row and the paragraph after `Vocabulary`), `plan/README.md`, the `ai/INDEX.md` keyword. Backfilled: this spec (`-`) and `spec-ike-virtual-ip-assignment.md` (`RFC3948-5.1-1`). No other spec is backfilled: the spec does not ask for it, and an absent row declares none.

Left, and blocked on `internal/le/rfc` (Phase 3's package, not edited here):

| Item | Needs |
|------|-------|
| AC-16 refusal: an unknown requirement id or stem | An exported id-to-stem index of `rfc/short/` that reads the summaries without the `ScanTree` tag scan `rfc.Collect` runs. The id shape alone cannot name the stem (`SFLOW-V5-x-1` belongs to `sflow-v5`), so the lookup is the corpus's, never a second rule in `internal/le/spec` |
| AC-16 Profiles cell, `TestRoadmapProfilesColumnFromRFCGaps` | The exported profile closure Phase 3 writes (roots plus builds-on and updates reach). The roadmap reads only the objects of one committed revision (`roadmap.Collect`, `readBlobs`), while `rfc.LoadProfiles` and `graph.Load` read a directory: the closure needs an entry that a revision's `rfc/` tree can feed (texts through `graph.Parse` and `graph.Build`, profiles through a byte-level parser), or the decision that the roadmap reads the RFC corpus of the working tree |
| Where the refusal lands | `parseMetadata` can only warn. "Refused by the spec parser" is either the spec validation hook (`validateSpecText`, `internal/le/hookruntime/lifecycle.go`, which already refuses an undeclared Status) or a roadmap diagnostic; the continuation picks after the index exists |
| `internal/le/site/roadmap.go` | Renders `roadmap.Markdown`, so the column reaches the site when `Markdown` gains it; no separate edit expected |

## Phase 2 done (2026-10-26)

Phase 2 landed as ONE commit (subject "rfc graph: dependency graph from the RFC texts, header-checked Meta links"): `internal/le/rfc/graph/` (`Parse`: header `Updates:`/`Obsoletes:` lines with continuations and the front-matter prose form, normative references at column-0 headings, combined References fallback, indented headings of RFC 791/905/1071; `Build`: in-corpus edges, RFC 2119/8174 and self-edges dropped, builds-on redirected to the obsoleting successor; Tarjan components and levels), `rfc/graph.tsv` written by `IndexUpdate` and registered DERIVED (gitignored, like the ledger family since `8ae7424594`), `checkHeaderLinks` (`check_headerlinks.go`), `checkUpdatedBy` reading the updater's header from the graph, and the 24 Meta corrections plus two false `Updated by` links (rfc2784, rfc7011). Tests: `TestNormativeReferenceParse`, `TestHeaderLinksParse`, `TestRFCNumberBounds`, `TestEveryCorpusTextYieldsReferences`, `TestGraphRedirectsObsoleted`, `TestGraphCycleSharesLevel`, `TestIndexUpdateWritesGraph`, `TestCheckRefusesMetaLinkHeaderMismatch`, `TestCheckReportsMetaLinkHeaderMismatch`, `TestCorpusMetaLinksMatchHeaders`; the decisions are in the spec's Design Insights (Phase 2 bullets).

Phase 3 starts here: `./le rfc tree` and `./le rfc next` read `graph.Load` (levels: `Graph.Level`, cycles: `Graph.Component`, update edges: `Graph.Updaters`); the status page orders by aggregate, level, RFC number. No profile closure function exists yet: Phase 3 writes it over builds-on and updates edges.

## Phase 1 done (2026-10-25, third agent)

Phase 1 landed as ONE commit: the profile parser, the Support migration of all 204 summaries, the 36 profile files, the coverage function, the docs and this section. The third agent finished it: the fixture trees gained `FixtureAggregateRel` (`TestRatchetsFireWhenEnrolmentMovesToMeta`, `TestTheGeneratorRefusesToWriteWhenTheRenderProducesNoRowAtAll`), `TestStatusPageRowSetUnchanged` (AC-6, `render_ledger_test.go`) compares the public row cells of every summary across the migration commit, read from git (the commit that added `rfc/profiles/bgp-core.md` and its parent), and the doc wiring findings were cleared: the nine profile and coverage names no other package reads were unexported (`shapeUnspecified`, `provenRequirements`, `mandatoryCell`, `optionalCell`, `absentFigures`, `figuresByStem`, `rollupOf`, `Rollup.mandatoryComplete`; `aggregateNames` was deleted as unused), and `docs/contributing/rfc-implementation-guide.md` now states the `| Support | <aggregate> |` cell. `renderStatusPage` refuses an aggregate with no table shape rather than rendering it wide.

`docs/features/rfc-status.md` and `rfc/requirements/` are derived on demand and never committed (`8ae7424594`); the post-write hook removes them and a command that names one rebuilds it.

Phase 2 starts from this tree: `internal/le/rfc/graph/` (normative-reference and header parser, `rfc/graph.tsv` from `./le rfc index-update`, cycle components, levels), the header-vs-Meta link check with the 24 corrections, and `checkUpdatedBy` switched from the Meta `Updates` cell to the graph. A Phase 2 caller of the coverage figures re-exports the names it uses. AC-6's "ordered by level" part waits for the levels: Phase 1 orders rows by RFC number inside each aggregate.

### What Phase 1 holds (second agent's table, kept for the record)

| Path | State |
|------|-------|
| `rfc/profiles/*.md` (36, untracked) | 8 use-case profiles (spec table; the `vpn-concentrator` Title is the agent's wording) and 28 aggregates with Title, Heading, Intro, Shape, Release n/a. The committed `./le rfc check` ignores the directory (verified) |
| `internal/le/rfc/profile.go`, `profile_test.go` (new) | `LoadProfiles`, `parseProfile`, `parseUseCase` (AC-9 refusal), `parseAggregate`, `Shape`; `TestProfileUnknownRootRefused`, `TestProfileRefusals`, `TestCorpusProfilesParse` |
| `internal/le/rfc/profilecoverage.go`, `profilecoverage_test.go` (new) | `DiscriminationVerdict.Proven` (the one per-row predicate), `provenRequirements`, `figuresByStem`, `rowCounts` (`{gap}`/`{partial}` never proven), `absentFigures` (0%), `rollupOf`, `Rollup.mandatoryComplete`; the owner ruling on absent RFCs and the IKE pool is recorded in the spec under R-7 |
| `meta.go` | `Meta.Support`/`Rank` replaced by `Aggregate` + `Row`; `readSupport` reads `<aggregate>`, or `-` plus an `Aggregate` cell; `checkAggregate` runs in `summaryMetas(tree, stems, profiles)`; `rowsFrom(metas, aggregates)` |
| `render_ledger.go` | `statusSections` and the rank deleted; `placeRows(metas, aggregates)` refuses an undeclared aggregate; `statusRowCompare` orders by RFC number |
| `status.go`, `render.go`, `check.go` | `Collected.Profiles`, `RenderInput.Profiles`; profile problems join `ParseErrors` |
| `check_baseline.go` | `retiredSectionRank` reads a HEAD^ summary in the old shape, git history only, so the migration commit keeps its ratchets |
| `fixture.go`, `selftest.go`, `selftest_core.go` | `FixtureAggregate` (`bgp-core`), `FixtureAggregateProfile`, `FixtureAggregateRel` in `FixtureFiles`; `newSelftestTree` writes the profile |
| tests updated | `check_test.go`, `meta_test.go` (new: `TestSupportNamesAggregate`, `TestBaselineReadsTheRetiredSectionRank`), `render_test.go`, `render_ledger_test.go`, `partial_test.go`, `summary_test.go`, `meta_implementation_test.go`, `lowerlayer*_test.go`, `provenshare_test.go`, `featuredeclined_test.go`, `check_gap_test.go`, `site/facts_test.go`, `site/rfcdetail_test.go`, `test/health/collect_rfc_test.go`, `internal/test/fixture/ui_fixture_le_rfc_answers.go` |
| docs | `docs/contributing/rfc-conformance-gates.md` ("Profiles and aggregates", "Coverage figures", the brief-shape paragraph, the `checkPublicRowMonotonic` row); `ai/skills/ze-rfc.md` step 6b |
| spec | Phase 1/5, the R-7 owner decision, two Design Insights bullets |

The migration was prepared in the session scratch (`tmp/session/2026-10-25-f3ae7c00-09af-4420-914f-39f2f7169980/scratch/`: `prof/members.tsv` is the membership of all 204 summaries, `migrated/` the migrated copies).

## Status

### Done (committed)

| Commit | What |
|--------|------|
| `cebd86cd48`, `00b9273a2a` | Owner rule in `ai/META-DECISIONS.md`: release profiles need 100% mandatory coverage (MUST, MUST NOT, SHALL, SHALL NOT, REQUIRED), proven by a recorded break; optional (SHOULD, SHOULD NOT, RECOMMENDED, NOT RECOMMENDED, MAY, OPTIONAL) is a second figure, improved after |
| `d47d2705ba` | The spec, design approved |
| `1facce9b44` | `{updated-by: <ID>; why}` row marker: parser (`parseUpdatedBy`, `summary.go`), `Meta.Updates` from the Meta `Updates` cell (`updatesFrom`, `meta.go`), `checkUpdatedBy` (`check_core.go`), selftest `summary/updated-by`, tests in `internal/le/rfc/updatedby_test.go`, docs in `docs/contributing/rfc-conformance-gates.md` ("The updated-by marker") and `ai/skills/ze-rfc.md`. `Requirement` kept at 152 bytes (`RollupState` is `uint8`, `Derived` beside `Ticked`). `rfc/short/rfc2966.md` Updates cell corrected to None |
| `fb3027cc78` | The 13 rfc4271 rows RFC 7606 rewrites carry `{updated-by}` (list in the spec, section "First `{updated-by}` set") |

Evidence for `1facce9b44`: `go test -race ./internal/le/rfc` ok 287.844s; scoped lint leaves 7 findings, all pre-existing in `discriminate.go`, `discriminate_action.go`, `discriminate_observe.go`, `discriminate_progress_test.go`.

### In progress
- Nothing uncommitted from this work: Phase 1 is committed.

### Remaining (spec phases)

| Phase | Work |
|-------|------|
| 1 | Done: see "Phase 1 done" above |
| 2 | `internal/le/rfc/graph/`: normative-reference and header parser, `rfc/graph.tsv` via `./le rfc index-update`, cycle components, levels; header-vs-Meta link check and the 24 corrections; switch `checkUpdatedBy` from the Meta `Updates` cell to the graph |
| 3 | `./le rfc tree`, `./le rfc next` in `internal/le/rfc/actions.go`; status page ordered by aggregate, level, RFC number |
| 4 | Ledger JSON gains graph, profiles, figures; `rfc-profiles` producer (`internal/le/site/rfcprofiles.go`); `./le site build` and `./le site check` on `../gh-pages` |
| 5 | `RFC gaps` spec Meta field (`specstatus.go`, `parseMetadata`), roadmap Profiles column, template row |
| Closure | `/ze-review` to 0 BLOCKER/0 ISSUE, `plan/templates/TEMPLATE-CLOSURE.md`, `./le verify changed`, closing commits A and B |

### Not done, and who owns it

| Item | Owner |
|------|-------|
| `{updated-by}` markers for the other updater pairs (summaries declare about 127 Updates links) | This spec's goal needs them for the release profiles; do them per release profile after Phase 3, when `./le rfc tree profile <name>` shows which pairs the profile reaches. Each pair needs the row-by-row reading against `rfc/full/` that rfc4271/rfc7606 got |
| LAN DNS resolver, DHCPv6 server, NAT and NTP RFCs for fttx-edge | Feature specs not yet written; the owner decides whether they run |
| IKE virtual-IP pool has no non-test caller | `spec-ike-virtual-ip-assignment` (release fixes) |
| `./le verify changed` debt on this checkout | Owed before any push; the push itself needs the owner's order |

## Files Already Handled (don't re-read)

- `internal/le/rfc/meta.go`: `readSupport` parses `-` or `<section> <rank>`; `checkRowCells` picks the table shape from `Brief`; `updatesFrom` reads the Updates cell
- `internal/le/rfc/render_ledger.go`: `statusSections` (10 keys, Heading, Intro, `Brief`), `placeRows` (rank clash refusal, rank sort), `renderStatusPage`
- `internal/le/rfc/provenshare.go`: denominator rule of 2026-09-02 (not-applicable and lower-layer stay in; `{rollup}` out)
- `internal/le/rfc/check_core.go`: `checkSuperseded`, `checkUpdatedBy`
- `internal/le/rfc/rfc.go`: `gatedLevels` is the mandatory set; `Requirement` layout (152 bytes, rangeValCopy threshold is 160)
- `internal/le/site/rfcledger.go`: per-row proof in `rfcLedgerRequirement.Covers[].Proof` (`rfcProofsByCover`); lift that predicate into `internal/le/rfc` in Phase 1
- `internal/le/spec/roadmap/roadmap.go`, `internal/le/spec/status/specstatus.go`: roadmap rows from spec Meta; no RFC link today
- Research results are in the spec's "Research findings" table; the reference-parser traps are listed there

## Traps met in this session

- The shared `./le` follows HEAD and is rebuilt in the background from committed code. A summary annotation the committed parser does not know makes the pretool hook refuse Bash calls naming `rfc/`, `docs/` or `internal/le/rfc/` for every session. Commit a parser change first, confirm the rebuilt `./le rfc check` accepts the new syntax, then commit the data. For Phase 1 the Support migration and the parser change must land in ONE commit, because the old parser refuses the new cell and the new parser refuses the old one; prepare the migrated summaries in the session scratch directory and copy them in just before `./le commit create`.
- Lint caps findings at 10 per linter per run; a clean-looking list can hide more of the same kind.
- `go test` under `./le job run` needs the feature tags: `tags="ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')"`.
- `./le go lint run scope "./internal/le/rfc/..."` refuses while another session edits sources; retry.
