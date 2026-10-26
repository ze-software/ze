# Spec: rfc-profile-declared-membership

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P2 |
| Release reason | The release profiles' coverage figures are what the first release is judged against. `./le rfc tree profile route-server` reports mandatory 909 of 2246 (40.5%) over 133 RFCs because membership follows normative citations into PPP, CHAP, DHCP, TFTP, IS-IS, DNSSEC, GRE, IKEv2, ESP, AH, MPLS, LDP and OSPF. The published figure is false, and the release gate it feeds is unreachable for a reason that is not a Ze defect |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`ai/META-DECISIONS.md`, "Goal and scope" (commits 1e7a9dbc34 and
38a3281ae1), now reads: "A release profile owes exactly the RFCs declared for
it, the ones its role implements, the way an established implementation of
that role (BIRD for a route server) does. A normative citation is never an
obligation: the RFC dependency graph only suggests candidates. An RFC for an
optional transport or feature is listed for the profile as optional, and it
counts toward the profile's percentage only once Thomas brings it into scope.
Every RFC a profile lists, required or optional, has a valid summary."

Today a use-case profile's members are its `Roots` plus everything reachable
over `builds-on` and `updates` edges (`priorityCorpus.closure`,
`internal/le/rfc/priority.go`), plus `Absent` RFCs that have no summary and
render at 0%. Three chains make the route-server figure false: RFC 7454 to
RFC 5082 to RFC 2003 to RFC 1812 (PPP, CHAP, DHCP, TFTP, IS-IS, DNSSEC, GRE);
RFC 8210 Section 9 ("Caches and routers MAY use TCP over IPsec transport")
into IKEv2, ESP and AH; RFC 7606 to RFC 6368 to RFC 4364 into MPLS, LDP and
OSPF. BIRD runs as an established IXP route server with none of these.

Goal:

1. Every RFC a profile lists has a valid summary. The 36 RFCs that today sit
   under `Absent`, or that this spec lists newly (RFC 4033, RFC 4034), get one
   through the `ze-rfc` skill, each passing `./le rfc check` (verbatim row
   quotes, extraction walk). This happens first, so the tooling switch never
   sees a member without a summary.
2. Profile membership becomes a declared fact in two tiers. Each
   `rfc/short/<stem>.md` declares in its `## Meta` table the use-case profiles
   that require it (counted) and those that list it as optional (shown with
   its own figures, never counted in the profile's percentage). Bringing an
   optional RFC into scope is moving one name from one row to the other.
3. The citation closure as a source of obligation is deleted, and so are the
   profile files' `Roots` and `Absent` fields (`ai/rules/no-layering.md`). The
   graph survives for two jobs: the level order (roots before leaves) and an
   advisory list of one-hop candidates, which no figure counts.

Owner direction on where the declaration lives: "have them listed in the
summary RFC we create and have the tooling able to parse it quickly".

## Required Reading

### Architecture Docs
- [ ] `ai/META-DECISIONS.md` "Goal and scope" - the rule this spec implements
  → Decision: two tiers. Required members count toward the profile's figures. Optional members are listed with their own figures and never enter the profile percentage. There is no third state.
  → Decision: every listed RFC, in either tier, has a summary. Nothing renders "absent at 0%" any more.
  → Decision: "work moves from root RFCs to leaves, except where a leaf updates a root's requirements" stays an ORDER rule. The graph level keeps ordering `tree` and `next`; it no longer decides membership.
- [ ] `docs/contributing/rfc-conformance-gates.md` "Profiles and aggregates" - the profile file format
  → Constraint: a use-case profile today requires `Kind`, `Title`, `Release`, `Roots`, `Absent` (`parseUseCase`, `internal/le/rfc/profile.go`).
  → Decision: `Roots` and `Absent` are both deleted from the format, and a profile file carrying either is refused. A use-case profile file holds `Kind`, `Title`, `Release` only.
- [ ] `docs/contributing/rfc-conformance-gates.md` "Coverage figures" - rollups
  → Constraint: a rollup is never complete while a member is not walked (`Rollup.mandatoryComplete`, `profilecoverage.go`). This predicate stays for each tier. The absent half (`Rollup.Absent`, `absentFigures`, `StemFigures.Absent`, "absent at 0%") is deleted with the `Absent` field.
- [ ] `docs/contributing/rfc-conformance-gates.md` "The dependency graph" - edges
  → Constraint: `builds-on` comes from the Normative References section and over-reaches "on purpose: a profile may reach more RFCs than it needs, never fewer". That sentence is the rationale being reversed and must be rewritten.
  → Decision: `graph.Load`, `graph.Build`, levels, cycles and `rfc/graph.tsv` are unchanged. Only profile membership stops reading the edges.
- [ ] `docs/contributing/rfc-conformance-gates.md` "The updated-by marker" - `{updated-by: <ID>; why}`
  → Constraint: `checkUpdatedBy` (`check_core.go`) holds the marker to the updater's own header and summary, and the marker changes no count. Today there are 13 markers, all in `rfc/short/rfc4271.md`, all naming RFC 7606.
  → Decision: the marker is the authored, checked statement that an updater rewrites a requirement of the marked RFC. It decides the `updates` question (Key Design Decisions).
- [ ] `docs/contributing/rfc-conformance-gates.md` "The extraction sign-off", "The row quote" - what a new summary must pass
  → Constraint: a summary with no valid extraction sign-off renders `not walked` and keeps its tier's rollup incomplete. Every row quote must be a verbatim span of the cited section. Phase 1 summaries must clear both, or the required figure stays incomplete by construction.
- [ ] `docs/contributing/rfc-conformance-gates.md` "The priority tree" - `tree`, `next`, `ProfileClosures`, `PriorityOf`
  → Constraint: `ProfileClosures` and `./le rfc tree profile <name>` answer the same members ("so the two cannot disagree"). That property must hold under the new name, per tier.
- [ ] `docs/contributing/gh-pages.md` (the `rfc-profiles` paragraph near line 620) - public profile pages
  → Constraint: today the page shows absent RFCs as 0% bars under "Absent from the corpus" and says why each RFC is in the closure. Both change.
- [ ] `docs/contributing/spec-workflow.md` lines 129-142 and `docs/architecture/site-facts.md` lines 118-121 - the roadmap Profiles column
  → Constraint: the column reads `rfc/short/`, `rfc/profiles/`, `rfc/full/` and `rfc/drafts/` blobs of the same revision (`rfcDirs`, `internal/le/spec/roadmap/profiles.go`). With declared membership it needs only `rfc/short/` and `rfc/profiles/`.
- [ ] `ai/doors/le-tooling.md` - the door for `internal/le`
  → Constraint: a shipped `ze` never links `internal/le`. The roadmap reaches the RFC parser only through `roadmap.RegisterCorpusReader` (`internal/le/rfc/register.go`), and that inversion stays.
- [ ] `.claude/skills/ze-rfc/SKILL.md` - how a summary is written
  → Constraint: Phase 1 runs this skill once per new RFC. The skill's text gains the two Meta rows in Phase 5.
- [ ] `plan/release/delivery/spec-rfc-updated-by-markers.md` (skeleton) - adds `{updated-by}` markers to updater pairs the release profiles reach
  → Constraint: under AC-6, every marker that spec adds onto a member requires the updater to be listed in a compatible tier. The two specs compose, and neither has to land first.
- [ ] `ai/rules/principles.md` - registration, declare once, no silent zero
  → Decision: each summary declares itself, and no list in `rfc/profiles/` duplicates it. A profile with no required member must not render "mandatory complete" (AC-5).

### RFC Summaries
- [ ] `rfc/short/rfc4271.md` - carries the 13 `{updated-by: RFC7606-…}` markers
  → Constraint: every profile that requires rfc4271 also requires rfc7606 (AC-6).
- [ ] `rfc/short/rfc7454.md` - `Implementation: foundation`
  → Constraint: a foundation summary contributes no figure, so requiring it for route-server adds a row and nothing to the count.

**Key insights:**
- Membership moves from the profile file (`Roots`, `Absent`) plus the edge closure to two mandatory Meta rows in every summary: `| Use cases | … |` (required, counted) and `| Optional use cases | … |` (listed, not counted), each `-` when empty.
- Every listed RFC has a summary. Phase 1 writes the 36 missing ones before any tooling changes.
- `updates` edges no longer pull an updater in. A `{updated-by}` marker does: the check refuses a member whose marker names an updater that is not listed in a compatible tier.
- `tree`, `next` and the site show the optional tier in its own section with its own rollup. The profile percentage is the required tier alone.
- The roadmap stops reading `rfc/full/` and `rfc/drafts/` (owner: "able to parse it quickly").

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/profile.go` (342L) - `UseCase{Name, Title, Release, Roots, Absent}`; `parseUseCase` requires `Roots` and `Absent`; `absentNumber` refuses an Absent item that has a summary; `LoadProfiles`/`ParseProfiles` collect problems rather than fail.
- [ ] `internal/le/rfc/meta.go` (966L) - `ParseMeta` reads Title, lineage, Enrolment, Implementation, Support/Aggregate; `knownMetaLabels` plus `nearMissRE` refuse misspelled labels; `summaryMetas` runs `checkAggregate` once profiles are loaded and collects problems per stem.
- [ ] `internal/le/rfc/priority.go` (304L) - `newPriorityCorpus` indexes `buildsOn` and `updaters`; `closure(roots)` is a BFS over both; `useCaseScope` builds `profileScope{reached, absent}`; `profileScope.path` renders "route-server: rfc4271 builds-on rfc791"; `absentStems`, `absentStemSet`, `absentNumbers`; `rollupOfScopes` sums the union.
- [ ] `internal/le/rfc/profilecoverage.go` (244L) - `StemFigures.Absent`, `absentFigures(number)`, `Rollup.Absent` (JSON `absent`), "absent at 0%" in `Rollup.String`, `mandatoryComplete` false while absent or unwalked members exist.
- [ ] `internal/le/rfc/priority_export.go` (107L) - `ProfileClosures(g, profiles)` answers each use case's members for the roadmap; `PriorityOf` answers the site's trees.
- [ ] `internal/le/rfc/tree.go` (249L) - `tree(profile)`: whole-corpus mode puts "nodes a release profile reaches" first; profile mode lists `scope.reached` and then unreached Absent items; `treeRow.Profiles` names every use case whose `reached` holds the stem.
- [ ] `internal/le/rfc/next.go` (334L) - `next` judges `useCaseScopes(true)`; `gapCandidates` iterates `scope.reached`; absent RFCs are gaps with `absentPath`; `firstPath` gives the edge path.
- [ ] `internal/le/rfc/roadmapcorpus.go` - `readRoadmapCorpus` parses texts with `graph.Parse`, builds the graph, calls `ProfileClosures`, and fills `roadmap.Corpus.ProfilesOf`.
- [ ] `internal/le/spec/roadmap/profiles.go` - `rfcDirs` lists the four corpus directories read from git blobs; `Corpus.ProfilesOf` is "every use-case profile whose closure holds it".
- [ ] `internal/le/site/rfcprofiles.go` - renders one page per profile from the ledger's `profiles`, with an "Absent" figure card (line 204) and "Absent from the corpus" rows; prose at lines 207, 299-341 and 480-496 speaks of what a profile "reaches".
- [ ] `internal/le/rfc/actions.go` lines 171-186 - the `le help` text of `rfc tree` and `rfc next` describes the closure.
- [ ] `internal/component/bgp/plugins/rpki/rtr_pdu.go` line 16 - `rtrVersionMin uint8 = 1`. The RTR client downgrades no lower than version 1, so Ze does not ship RTR version 0 (RFC 6810).
- [ ] `internal/component/ike/engine/`, `internal/component/ike/wire/payload_sa.go`, `internal/plugins/ospf/config.go` - AH (RFC 4302) is configurable only for OSPFv3 IPsec (`ipsecProtoAH`, `ze-ospf-conf.yang` `enum ah`). The IKE engine only parses AH protocol ids and deletes AH SAs. It offers no AH proposal, so Ze does not ship AH for the vpn-endpoint role.
- [ ] `rfc/profiles/*.md` - eight use-case profiles: six release (route-server, route-collector, looking-glass, vpn-endpoint, lns, fttx-edge) and two not (vpn-concentrator, ddos-flowspec). There are 28 technology aggregates.

**Behavior to preserve:**
- Technology aggregates: membership through the `Support`/`Aggregate` cell, rendering, `checkAggregate`, status page order. All untouched.
- Graph parsing, levels, cycles, `rfc/graph.tsv`, `checkHeaderLinks`, `checkUpdatedBy`.
- Ordering in `tree` and `next`: level, then RFC number, cycle members together; a `{updated-by}` row is owed at the updater's level.
- Both figures (mandatory and optional keyword registers), `ScopeRollup.MandatoryComplete`, and the not-walked count.
- Problem collection: one unparsable summary or profile never stops `./le rfc check` for other sessions.

**Behavior to change:**
- Use-case membership: declared by summaries in two tiers, never reached over edges.
- `Roots` and `Absent` are retired from the profile format, and refused when present. The absent state is deleted from figures, rollups, `tree`, `next`, JSON and the site.
- `tree`, `next` and the site gain a separate optional-tier section with its own rollup. `tree profile <name>` gains a candidates list. The path text, help text, site prose and roadmap inputs change.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `rfc/short/<stem>.md` `## Meta` table, two new mandatory rows: `| Use cases | route-server, looking-glass |` and `| Optional use cases | lns |`, each `-` when empty.
- `rfc/profiles/<name>.md`, use-case kind, now `Kind`, `Title`, `Release`.

### Transformation Path
1. `ParseMeta` reads both rows into two `Meta` fields (sorted names, empty for `-`). Each is a parse error naming the summary: a missing row, an empty cell, a token that is not one profile name, a duplicate within a row, `-` mixed with names, and one name in both rows.
2. `summaryMetas`, beside `checkAggregate`, refuses a name with no use-case profile file and a name that is a technology aggregate, listing the valid use cases. The stem is collected as a problem, not a fatal error.
3. `priorityCorpus` builds two member sets per `profileScope`, required and optional, from the metas. No edge is followed.
4. `rollupOfScopes` sums the required set for the profile figure, and separately the optional set for the optional-tier figure. `tree`, `next` and `PriorityOf` carry both.
5. Candidates pass, `tree profile <name>` only: the `buildsOn` targets and `updaters` of each member (either tier) that are listed in neither tier, each printed once with the first member and edge that names it, sorted by level and then by number.
6. `ProfileClosures` is replaced by a members function over `Profiles` and parsed metas that answers both tiers. `readRoadmapCorpus` parses each summary's Meta and reads no texts. The roadmap Profiles column names required membership only, because an optional listing places no spec in a release profile.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `internal/le/rfc` ↔ `internal/le/spec/roadmap` | `roadmap.RegisterCorpusReader(readRoadmapCorpus)`; `Corpus.ProfilesOf` | No |
| `internal/le/rfc` ↔ `internal/le/site` | `rfc.PriorityOf` → requirement ledger JSON `profiles` → `rfc-profiles` producer | No |

### Integration Points
- `summaryMetas` (`meta.go`) - the name checks run where `checkAggregate` runs, with the same problem collection.
- `Collect`/`check.go` - the marker-consistency check (AC-6) and the empty-profile check (AC-5) report as findings through the existing check driver.
- `useCaseScope` (`priority.go`) - the single place membership is built, which every consumer already goes through.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | membership enters through `ParseMeta`, as the aggregate does |
| No unintended coupling (components stay isolated) | Yes | the roadmap still reaches `internal/le/rfc` only through its registered reader |
| No duplicated functionality (extends existing, does not recreate) | Yes | reuses Meta parsing, problem collection, `profileScope` and the rollup |
| Zero-copy preserved where applicable (refs, not copies) | N-A | developer tooling, no wire path |
| Registration over hardcoding, outbound | Yes | each summary declares its own profiles; no list in Go names an RFC |
| Registration over hardcoding, inbound | Yes | searched: `Roots` and `Absent` (profile.go, priority.go, priority_export.go, profilecoverage.go, tree.go, next.go, site/rfcprofiles.go and their tests), `ProfileClosures` (roadmapcorpus.go), `closure(` (priority.go). Profile names are validated against `Profiles.UseCases`, loaded from `rfc/profiles/`, never a literal |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Owner wants the declaration in each summary, not in the profile file | Owner direction quoted in Task | Roots would stay and summaries would not change | Owner statement | validated |
| A-2 | `{updated-by}` markers are the right evidence that an updater changes a member's obligations | `checkUpdatedBy` holds each marker to the updater's header and to a declared requirement id | An updater that rewrites a member with no marker is missed. It still shows in the candidates list, and `spec-rfc-updated-by-markers` adds the missing markers | AC-6 test plus the candidates output for route-server | unvalidated |
| A-3 | Two mandatory rows on every summary (240 after Phase 1) is acceptable churn | `Support` is already mandatory with `-`, and summaries are machine-edited | An optional row would let a forgotten declaration read as "not owed" with no warning | Owner review of this spec | unvalidated |
| A-4 | The required-tier lists in Population match what BIRD, accel-ppp and strongSwan implement for each role | Agent judgement from the role, the BIRD feature set and each summary's `Support status`; Thomas's answers on the optional tier, RFC 6810, RFC 4302 and DNSSEC | Wrong figures until corrected; a correction is moving one name between two rows | Owner review of the Population tables | unvalidated |
| A-5 | No consumer reads profile members other than those listed | grep of `ProfileClosures`, `useCaseScope`, `useCaseScopes`, `.Roots`, `.Absent`, `LoadProfiles`, `ParseProfiles` across `internal` and `cmd` | A missed consumer keeps reading a deleted field | `Roots` and `Absent` are deleted from `UseCase`, so any reader fails to compile | unvalidated |
| A-6 | Each of the 36 new RFCs can be extracted and signed off within this spec | The texts of 12 are in `rfc/full/`; the other 24 are fetchable from rfc-editor.org | A summary that cannot be walked leaves its tier `not walked` and incomplete, which is truthful. The spec stays open until it is walked | `./le rfc check` per summary in Phase 1 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A summary another session is editing loses or conflicts on the new rows | `./le rfc check` reports the stem's missing row | Problem collection keeps the check running, and the error names the row and the fix |
| R-2 | A profile with no required member reads "mandatory complete" (0 of 0) | the rollup prints `mandatory 0 of 0` with complete | AC-5 refuses a use-case profile with no required member |
| R-3 | An author lists an RFC because the graph shows it as a candidate, re-creating the over-reach by hand | the candidates list shrinks while membership grows fast | the candidates heading states they are not owed and quotes the owner rule |
| R-4 | Site pages keep stale "closure" or "absent" prose | `TestRFCProfilePagesRendered` asserts the old wording | update prose and test together, and grep the site package for "reach", "closure" and "Absent" |
| R-5 | The roadmap Profiles column changes for existing specs (fewer profiles per stem) | roadmap snapshot diff | intended; `TestRoadmapProfilesOverRealCorpus` logs the new placement and the reviewer reads it |
| R-6 | A stale profile file from another session still carries `Roots` or `Absent` | parse problem on that profile | the refusal names the retired field and the summary rows that replace it |
| R-7 | A summary names a technology aggregate (for example `bgp-core`) in a use-case row | parse problem | the refusal says it is an aggregate and lists the use cases |
| R-8 | Phase 1 summaries are paraphrased or partly walked, and their figures look proven | `./le rfc check` row-quote refusals; extraction sign-off missing | each summary passes `./le rfc check` before Phase 2 starts; a summary without sign-off renders `not walked` |
| R-9 | Removing the `absent` JSON key breaks a reader of the published ledger | site build fails on the key | the only reader is `internal/le/site/rfcprofiles.go`, which changes in the same phase |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Developer-facing figures and the public RFC profile pages show wrong membership. No shipped binary changes |
| How is it reverted? | Phase 1 summaries stand alone. Phases 2-5 revert as one commit (code, docs, summary rows, profiles) |
| Who else touches this path? | Every session editing `rfc/short/*.md`; `spec-rfc-updated-by-markers` (markers in summaries); site generation sessions |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc tree profile <name>` over a fixture tree whose summaries declare both rows | → | `ParseMeta` → `useCaseScope` → `tree` | `TestTreeProfileMembersAreDeclared` |
| `./le rfc next profile <name>` over the same fixture | → | `gapCandidates` over required members, optional section after it | `TestNextIgnoresUndeclaredCitations` |
| `./le rfc check` over a fixture with an unknown use-case name | → | `summaryMetas` name check | `TestUseCasesNamingUnknownProfileRefused` |
| roadmap Profiles column over committed blobs | → | `readRoadmapCorpus` → members function | `TestRoadmapProfilesColumnFromRFCGaps` (updated) |
| site `rfc-profiles` producer | → | `PriorityOf` → `renderRFCProfiles` | `TestRFCProfilePagesRendered` (updated) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A summary whose Meta has `Use cases` = `route-server` and `Optional use cases` = `looking-glass` | `ParseMeta` answers one required and one optional name. `./le rfc tree profile route-server` lists the stem among the required rows with the path `route-server: required by rfc/short/<stem>.md`. `tree profile looking-glass` lists it in the optional section with the path `looking-glass: optional in rfc/short/<stem>.md` |
| AC-2 | A summary missing either row; an empty cell; `-, route-server`; a duplicate name in one row; one name in both rows; a token that is not one lower-case hyphenated name; a misspelled label such as `Use-cases`, `Optional usecases` or `Profiles` | Each is a parse problem naming the summary, the cell and the fix, and `./le rfc check` still runs for every other summary |
| AC-3 | A summary naming, in either row, a profile with no `rfc/profiles/` file or a technology aggregate | Refused, naming the summary and listing the use-case profiles by name |
| AC-4 | Profile P requires rfc A; A's text cites rfc B normatively and B lists P in neither row; rfc C's header updates A and C carries no marker on A's rows | Neither B nor C is a member of P in either tier: they are absent from P's rows, both rollups, `next`, `ProfilesOf` and the site page. Both appear in `tree profile P` under the candidates heading with the edge that names them |
| AC-5 | A use-case profile that no summary lists as required (optional-only included) | `./le rfc check` refuses it, naming the profile and the row a summary writes to join it. Its rollup is never reported complete |
| AC-6 | Summary A lists P and carries a `{updated-by: X-…}` marker. X is listed for P in neither row, or A is required and X only optional | `./le rfc check` refuses, naming A, the marker, X, and the fix: list P in X's row of the same tier as A (required when A is required) |
| AC-7 | A use-case profile file carrying a `Roots` or an `Absent` row | Parse problem naming the file, saying the field is retired and that membership is each summary's two use-case rows |
| AC-8 | `./le rfc tree profile <P>` for a profile with both tiers | The first line's figures, RFC count and completeness cover the required tier only. A separate optional section prints its own rollup line and rows. The required figure is the same with or without optional members present (fixture with and without them) |
| AC-9 | `./le rfc next` and `next profile <P>` | The answered gap comes from required members only. Optional members owing rows appear in a separately labelled "optional, not counted" section, also under its own JSON key |
| AC-10 | The site profile page for P | Figure cards and the level table cover the required tier. A separate "Optional (not counted)" table shows each optional member with its own bars, plus that tier's rollup. No "Absent" card or row exists |
| AC-11 | The committed corpus after Phases 1-4 | `./le rfc tree profile route-server` reports exactly the route-server required list in Population, and the optional section exactly its optional list. No PPP, CHAP, DHCP, TFTP, IS-IS, DNSSEC, GRE, IKEv2, ESP, AH, MPLS, LDP or OSPF summary is a member of either tier. Every member of every profile has a summary (no "no summary" member row anywhere) |
| AC-12 | Roadmap Profiles column at a revision | Computed from `rfc/short/` and `rfc/profiles/` only; `rfcDirs` no longer lists `rfc/full` or `rfc/drafts`. A spec whose `RFC gaps` names a stem gets exactly the profiles that stem's summary requires |
| AC-13 | Whole-corpus `./le rfc tree` | Rows required by a release profile come first. Each row's `Profiles` names the profiles requiring it, and a new `optional-profiles` field names those listing it as optional |
| AC-14 | `le help rfc tree`, `le help rfc next` | The text describes the two tiers, says the percentage counts the required tier only, and names the candidates list. The word closure no longer describes membership |
| AC-15 | Each Phase 1 summary | `./le rfc check` reports no problem for it: Meta parses, every row quote is verbatim in its cited section, and the extraction sign-off is valid |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseMetaUseCases` | `internal/le/rfc/meta_test.go` | AC-1 parse and sort of both rows | |
| `TestParseMetaUseCasesRefusals` | `internal/le/rfc/meta_test.go` | AC-2 table: missing, empty, mixed `-`, duplicate, name in both rows, bad token, near-miss labels | |
| `TestUseCasesNamingUnknownProfileRefused` | `internal/le/rfc/meta_test.go` | AC-3 unknown name and aggregate name, in each row | |
| `TestProfileRetiredFieldsRefused` | `internal/le/rfc/profile_test.go` | AC-7 for `Roots` and `Absent`; replaces `TestProfileUnknownRootRefused` and the Absent cases of `TestProfileRefusals` | |
| `TestTreeProfileMembersAreDeclared` | `internal/le/rfc/tree_test.go` | AC-1, AC-4, AC-13 over a fixture graph with builds-on and updates edges outside the declared set | |
| `TestTreeOptionalTierNotCounted` | `internal/le/rfc/tree_test.go` | AC-8: the required figure is identical with and without optional members; the optional section has its own rollup | |
| `TestTreeProfileListsCandidates` | `internal/le/rfc/tree_test.go` | AC-4 candidates: one hop, not counted, edge named | |
| `TestNextIgnoresUndeclaredCitations` | `internal/le/rfc/next_test.go` | AC-4 for `next` | |
| `TestNextOptionalSectionSeparate` | `internal/le/rfc/next_test.go` | AC-9 | |
| `TestEmptyUseCaseProfileRefused` | `internal/le/rfc/profile_test.go` | AC-5, including an optional-only profile | |
| `TestUpdatedByMarkerRequiresUpdaterListed` | `internal/le/rfc/check_core_test.go` | AC-6, both refusal shapes | |
| `TestProfileMembersAndRequirementStems` | `internal/le/rfc/priority_export_test.go` | the members function equals `tree profile` members per tier; replaces `TestProfileClosuresAndRequirementStems` | |
| `TestRollupHasNoAbsentState` | `internal/le/rfc/profilecoverage_test.go` | the absent state is gone: no `absent` JSON key, no "absent at 0%" text | |
| `TestCorpusProfilesParse` (existing) | `internal/le/rfc/profile_test.go` | the real `rfc/profiles/` parse without `Roots` or `Absent` | |
| `TestRoadmapProfilesColumnFromRFCGaps` (existing, updated) | `internal/le/rfc/roadmapcorpus_test.go` | AC-12 with no text files in the input | |
| `TestRoadmapDigestCoversTheRFCCorpus` (existing, updated) | `internal/le/rfc/roadmapcorpus_test.go` | AC-12: the digest covers only the two directories | |
| `TestRFCProfilePagesRendered` (existing, updated) | `internal/le/site/rfcprofiles_test.go` | AC-10 | |
| `TestEveryProfileMemberHasASummary` | `internal/le/rfc/profile_test.go` | AC-11 over the real checkout: every name in either row resolves to a summary, by construction; the test proves no profile reports a "no summary" member | |
| `TestRouteServerProfileExcludesCitationClosure` | `internal/le/rfc/profile_test.go` | AC-11 over the real checkout: no member in either tier belongs to the ppp-access, dhcp-provisioning, isis, dns, tunnels, ikev2-ipsec, mpls or ospf aggregates | |
| `TestHelpDescribesDeclaredMembership` | `internal/le/rfc/actions_test.go` | AC-14 | |

Existing tests whose fixtures build `UseCase{Roots: …, Absent: …}` (`tree_test.go`
including `TestAbsentRFCTheClosureReachesHoldsOneState`, `priority_export_test.go`,
`next_test.go`, `profilecoverage_test.go`, `roadmapcorpus_test.go`) are rewritten
to declare membership through fixture metas. What each asserted about ordering,
walked state and figures is kept, and only the membership source changes. Two
kinds of assertion change their expectation by design: a citation-reached stem
being a member (AC-4), and the absent state (deleted by owner rule). Both go to
an independent `ze-test-judge` agent under `ai/META-DECISIONS.md` "Test
changes", which cites the owner rule as the source showing the old expectation
is wrong.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | the feature takes no numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `./le rfc tree profile route-server` on the checkout | output pasted in closure | the profile figure covers the required list; the optional section is separate | |
| `./le rfc check` on the checkout | output pasted in closure | no problem in any Phase 1 summary and no use-case row problem | |

### Interop Tests
N-A: tooling with no protocol peer (`ai/rules/interop-and-goal-validation.md` exemption).

## Population

**Required** is written into `Use cases`, **Optional** into `Optional use
cases`. Thomas moves an optional RFC into scope by moving its name to the other
row. Stems marked `*` have no summary today; Phase 1 writes them (full list
below). Every other summary writes `-` in both rows.

### route-server (BIRD as IXP route server)
| Tier | Stems |
|------|-------|
| Required | rfc4271, rfc4760, rfc2545, rfc5492, rfc6793, rfc6286, rfc6608, rfc7606, rfc7607, rfc9687, rfc9774, rfc7911, rfc7947, rfc9234, rfc1997, rfc8092, rfc7999, rfc6811, rfc8210, rfc9582, rfc7454, rfc7948*, rfc8212*, rfc8893* |
| Optional | rfc2918, rfc7313, rfc4486, rfc8203, rfc9003, rfc4360, rfc8654, rfc9072, rfc5082, rfc2385, rfc4724, rfc8950 |
| Dropped | rfc6810: replaced by RFC 8210, and Ze does not ship RTR version 0 (`rtrVersionMin uint8 = 1`, `internal/component/bgp/plugins/rpki/rtr_pdu.go`). rfc7705: AS migration is a router feature, not a route-server one. Everything reached only by citation |

### route-collector (RIPE RIS or RouteViews style: BGP in, MRT and BMP)
| Tier | Stems |
|------|-------|
| Required | rfc4271, rfc4760, rfc2545, rfc5492, rfc6793, rfc6286, rfc6608, rfc7606, rfc7607, rfc9687, rfc9774, rfc7911, rfc7854, rfc8671, rfc9069, rfc6396, rfc6397, rfc8050 |
| Optional | rfc8654, rfc9072, rfc2918 |
| Dropped | rfc1997, rfc8092: a collector archives attribute bytes and does not act on communities |

### looking-glass
| Tier | Stems |
|------|-------|
| Required | rfc4271, rfc4760, rfc2545, rfc5492, rfc6793, rfc6286, rfc6608, rfc7606, rfc7607, rfc9687, rfc9774, rfc7911, rfc1997, rfc8092, rfc6811, rfc8522* |
| Optional | rfc8210, rfc4360 |

### vpn-endpoint (strongSwan site-to-site)
| Tier | Stems |
|------|-------|
| Required | rfc7296, rfc4301, rfc4303, rfc3948, rfc5282, rfc7427, rfc6071, rfc8247*, rfc8221*, rfc4106*, rfc5280* |
| Optional | rfc7383*, rfc7634*, rfc8031*, rfc4754*, rfc9370* |
| Dropped | rfc4302 AH: the IKE engine offers no AH proposal; AH is configurable only for OSPFv3 IPsec (`ipsecProtoAH`, `internal/plugins/ospf/config.go`), which is not this role. rfc4555, rfc3748, rfc5216, rfc9190: concentrator role |

RFC 5280 is required because rfc7427 (signature authentication) is required and
its certificates are X.509. Its summary declares `Implementation: third-party`
(Go's `crypto/x509`), so it counts nothing and still owes tests at Ze's boundary
(`ai/rules/rfc-compliance.md`). RFC 4106 is required because RFC 8221 makes
AES-GCM a MUST-level ESP algorithm.

### lns (accel-ppp as L2TP network server)
| Tier | Stems |
|------|-------|
| Required | rfc2661, rfc1661, rfc1332, rfc1877, rfc5072, rfc1334, rfc1994, rfc2759, rfc2865, rfc2866, rfc2869, rfc5176, rfc3162*, rfc4861* |
| Optional | rfc3579, rfc1570*, rfc2867*, rfc2868*, rfc4818*, rfc6911*, rfc3145* |

RFC 3162 (RADIUS IPv6 attributes) and RFC 4861 (Neighbor Discovery, the router
advertisements on the PPP link) are required because rfc5072 (IPv6 over PPP)
is required. The rest are optional RADIUS or L2TP extensions.

### fttx-edge
| Tier | Stems |
|------|-------|
| Required | rfc2661, rfc2516, rfc1661, rfc1332, rfc5072, rfc1877, rfc1994, rfc2131, rfc2132, rfc9915, rfc1035, rfc2181, rfc4862, rfc7084*, rfc3022*, rfc4787*, rfc5382*, rfc4861* |
| Optional | rfc4035, rfc4033*, rfc4034* (DNSSEC, optional only and never required: owner, "DNSSEC is not urgent at all"), rfc5905*, rfc8106*, rfc4638* |

### Non-release use cases
| Profile | Required | Optional |
|---------|----------|----------|
| vpn-concentrator | rfc7296, rfc4301, rfc4303, rfc3948, rfc4555, rfc5282, rfc3748, rfc5216, rfc7427, rfc9190, rfc8247*, rfc8221*, rfc4106*, rfc5280*, rfc5998* | rfc7383*, rfc7634*, rfc8031*, rfc4754*, rfc9370*, rfc4739*, rfc5685*, rfc9329* |
| ddos-flowspec | rfc4271, rfc7606, rfc4760, rfc8955, rfc8956, rfc9117, rfc7999, rfc4360, rfc7011, rfc7012, rfc3954, sflow-v5, rfc5635* | rfc3882* |

Every profile that requires rfc4271 also requires rfc7606, as AC-6 demands (13
markers in `rfc/short/rfc4271.md`). ddos-flowspec gains rfc7606 for that reason.

### New summaries owed (Phase 1)
36 RFCs. Of these, 12 have a text in `rfc/full/` (3022, 4106, 4787, 4861, 5280,
5382, 7383, 7634, 8106, 8212, 8221, 8247). The other 24 are fetched first with
`curl -o rfc/full/rfcNNNN.txt https://www.rfc-editor.org/rfc/rfcNNNN.txt`.
The aggregate column is a proposal that `ze-rfc` confirms.

| Stem | Profiles (tier) | Proposed aggregate |
|------|-----------------|--------------------|
| rfc7948 | route-server (required) | bgp-session-ops |
| rfc8212 | route-server (required) | bgp-core |
| rfc8893 | route-server (required) | rpki |
| rfc8522 | looking-glass (required) | bgp-monitoring |
| rfc8247 | vpn-endpoint, vpn-concentrator (required) | ikev2-ipsec |
| rfc8221 | vpn-endpoint, vpn-concentrator (required) | ikev2-ipsec |
| rfc4106 | vpn-endpoint, vpn-concentrator (required) | ikev2-ipsec |
| rfc5280 | vpn-endpoint, vpn-concentrator (required) | ikev2-ipsec |
| rfc7383 | vpn-endpoint, vpn-concentrator (optional) | ikev2-ipsec |
| rfc7634 | vpn-endpoint, vpn-concentrator (optional) | ikev2-ipsec |
| rfc8031 | vpn-endpoint, vpn-concentrator (optional) | ikev2-ipsec |
| rfc4754 | vpn-endpoint, vpn-concentrator (optional) | ikev2-ipsec |
| rfc9370 | vpn-endpoint, vpn-concentrator (optional) | ikev2-ipsec |
| rfc5998 | vpn-concentrator (required) | eap |
| rfc4739 | vpn-concentrator (optional) | ikev2-ipsec |
| rfc5685 | vpn-concentrator (optional) | ikev2-ipsec |
| rfc9329 | vpn-concentrator (optional) | ikev2-ipsec |
| rfc3162 | lns (required) | radius-tacacs |
| rfc4861 | lns, fttx-edge (required) | dhcp-provisioning |
| rfc1570 | lns (optional) | ppp-access |
| rfc2867 | lns (optional) | radius-tacacs |
| rfc2868 | lns (optional) | radius-tacacs |
| rfc4818 | lns (optional) | radius-tacacs |
| rfc6911 | lns (optional) | radius-tacacs |
| rfc3145 | lns (optional) | ppp-access |
| rfc7084 | fttx-edge (required) | dhcp-provisioning |
| rfc3022 | fttx-edge (required) | firewall or a new nat aggregate, `ze-rfc` decides |
| rfc4787 | fttx-edge (required) | same as rfc3022 |
| rfc5382 | fttx-edge (required) | same as rfc3022 |
| rfc5905 | fttx-edge (optional) | management |
| rfc8106 | fttx-edge (optional) | dhcp-provisioning |
| rfc4638 | fttx-edge (optional) | ppp-access |
| rfc4033 | fttx-edge (optional) | dns |
| rfc4034 | fttx-edge (optional) | dns |
| rfc5635 | ddos-flowspec (required) | bgp-communities-policy |
| rfc3882 | ddos-flowspec (optional) | bgp-communities-policy |

## Files to Modify
- `internal/le/rfc/meta.go` - the `Use cases` and `Optional use cases` labels and `Meta` fields, their parse and refusals, `knownMetaLabels`, `nearMissRE` widened to catch `use case` and `profile` spellings, and the name check in `summaryMetas`
- `internal/le/rfc/profile.go` - delete `Roots` and `Absent` from `UseCase` and `parseUseCase`, delete `absentNumber` and `absentRE`, refuse both retired rows, doc comments
- `internal/le/rfc/priority.go` - delete `closure` and the absent helpers; `useCaseScope` builds both tiers from metas; `path` renders the declaration; candidates pass; `rollupOfScopes` per tier
- `internal/le/rfc/profilecoverage.go` - delete `StemFigures.Absent`, `absentFigures`, `Rollup.Absent` and "absent at 0%"
- `internal/le/rfc/priority_export.go` - `ProfileClosures` replaced by a two-tier members function
- `internal/le/rfc/tree.go` - optional section and its rollup, candidates, `optional-profiles` on rows, release-first ordering by required membership; absent rows deleted
- `internal/le/rfc/next.go` - required tier answers, optional section, absent gaps deleted
- `internal/le/rfc/check.go`, `internal/le/rfc/check_core.go` - AC-5 and AC-6 findings
- `internal/le/rfc/roadmapcorpus.go` - parse Meta, drop texts and the graph
- `internal/le/spec/roadmap/profiles.go` - `rfcDirs` down to two directories; `Corpus` comments
- `internal/le/rfc/actions.go` - help text of `tree` and `next`
- `internal/le/site/rfcprofiles.go` - optional table, Absent card and rows deleted, prose
- the tests named in the TDD plan
- `rfc/short/*.md` (204 existing plus 36 new) - the two use-case rows in each
- `rfc/profiles/{route-server,route-collector,looking-glass,vpn-endpoint,lns,fttx-edge,vpn-concentrator,ddos-flowspec}.md` - delete `Roots` and `Absent`
- `docs/contributing/rfc-conformance-gates.md` - "Profiles and aggregates", "Coverage figures" (absent sentences), "The dependency graph" (the over-reach sentence), "The priority tree"
- `docs/contributing/rfc-implementation-guide.md` - the Meta row description near line 657
- `docs/contributing/gh-pages.md` - the `rfc-profiles` paragraph
- `docs/contributing/spec-workflow.md` - the roadmap Profiles column paragraph
- `docs/architecture/site-facts.md` lines 118-121 - the directories the Profiles column reads
- `.claude/skills/ze-rfc/SKILL.md` lines 12-16 - "closure" wording; a summary writes both rows
- ledger and site outputs - regenerated by `./le rfc update`, never hand-edited

## Files to Create
- `rfc/short/<stem>.md` for each of the 36 stems in "New summaries owed", with its `rfc/extraction/<stem>.json` sign-off
- `rfc/full/rfcNNNN.txt` for the 24 texts not yet in the tree

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | tooling, no product config |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | No | `tree` and `next` keep their grammar; only output and help text change (`actions.go`) |
| CLI grammar (keyword before value) | N-A | no new keyword |
| Editor autocomplete | N-A | profile-name completion already derives from `LoadProfiles` (`actions.go` 525-542) |
| Functional test for new RPC/API | N-A | no RPC; command output is covered by the le package tests |
| Pipe completeness | No | `| json` gains `optional`, `candidates` and `optional-profiles` keys and loses `absent` |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Prometheus counters/metrics | N-A | none |
| BGP family surface | N-A | no family |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | developer tooling; no `features/<id>.md` covers the priority tree |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | the `ze` CLI is untouched; `le help` text lives in `actions.go` |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | 36 new `rfc/short/*.md` with their Support or Aggregate rows, which `docs/features/rfc-status.md` renders through `./le rfc update`; the two use-case rows in every summary |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/rfc-conformance-gates.md` (four sections), `docs/contributing/rfc-implementation-guide.md` |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/contributing/spec-workflow.md` (roadmap inputs), `docs/contributing/gh-pages.md` (profile pages). `docs/architecture/core-design.md`, declared by the `// Design:` header of `meta.go`, `check.go`, `check_core.go` and `actions.go` ("the rfc area, as one command"), is unaffected: no command is added or removed, and the area's shape does not change. Profile membership is described only in `docs/contributing/rfc-conformance-gates.md` |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/release/validation/spec-rfc-profile-declared-membership.md`. Known: the gates-doc sections anchor `profile.go`, `priority.go`, `priority_export.go`, `profilecoverage.go`, `tree.go`, `next.go` and `meta.go`; `spec-workflow.md` anchors `roadmap/profiles.go`; `docs/architecture/site-facts.md` lines 118-121 list the four directories the Profiles column reads, which drop to two (Phase 3); `docs/functional-tests.md` anchors only `actions.go` `Answer`, which this spec does not change |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the `route-server: rfc7947 builds-on rfc4271` example in "The priority tree" becomes the declared form |

## Implementation Steps

1. **Phase: Summaries (before any tooling change)** - for each of the 36 stems in "New summaries owed", fetch the text if missing, run the `ze-rfc` skill, and write the extraction sign-off. Each one passes `./le rfc check` (AC-15) before the next phase starts. They are written with today's Meta format and gain the use-case rows in Phase 4, so `./le rfc check` stays green on today's tooling. Once each summary exists, its RFC is removed from the `Absent` row of each profile listing it, because `absentNumber` refuses the duplicate, and it is added to that profile's `Roots`. That keeps today's closure correct throughout Phase 1. These summaries can land as their own commits.
2. **Phase: Wiring** - add both Meta rows with their parse and refusals; delete `Roots`, `Absent` and the absent state; make `useCaseScope` build the two tiers from metas. Write `TestTreeProfileMembersAreDeclared`, `TestTreeOptionalTierNotCounted` and `TestParseMetaUseCases` first; they fail on the old closure.
   - Files: `meta.go`, `profile.go`, `priority.go`, `profilecoverage.go`
   - Verify: package `internal/le/rfc` compiles only once every `Roots` and `Absent` reader is gone (A-5)
3. **Phase: Consumers** - optional section and candidates in `tree.go`; `next.go`; the members function in `priority_export.go`; `roadmapcorpus.go` and `rfcDirs`; the site page; help text.
   - Tests: `TestTreeProfileListsCandidates`, `TestNextIgnoresUndeclaredCitations`, `TestNextOptionalSectionSeparate`, `TestRollupHasNoAbsentState`, the roadmap and site tests, `TestHelpDescribesDeclaredMembership`
4. **Phase: Checks and population** - AC-3 (in `summaryMetas`), AC-5 and AC-6 findings. Write both rows into all 240 summaries from the Population tables (`-` elsewhere), and delete `Roots` from the eight profiles. Phases 2 to 4 land as one commit, because a summary without the rows does not parse under the new reader. The edit runs only on `rfc/short/*.md` and those eight files.
   - Tests: `TestUseCasesNamingUnknownProfileRefused`, `TestEmptyUseCaseProfileRefused`, `TestUpdatedByMarkerRequiresUpdaterListed`, `TestEveryProfileMemberHasASummary`, `TestRouteServerProfileExcludesCitationClosure`, `TestCorpusProfilesParse`; `./le rfc check` shows no use-case row problem
5. **Phase: Docs** - each page is edited in the phase whose code makes it wrong (`ai/rules/documentation.md`): the gates doc with phases 2-4, spec-workflow, site-facts and gh-pages with phase 3, the skill and the implementation guide with phase 4. `./le rfc update` regenerates the derived files.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No path from `buildsOn` or `updaters` into either member set remains |
| Correctness | The required figure is independent of the optional tier; an optional-only or empty profile never renders complete |
| Correctness | No absent state survives anywhere (grep `Absent`, `absent at 0%` in `internal/le/rfc` and `internal/le/site`) |
| Naming | JSON keys kebab-case (`optional`, `candidates`, `optional-profiles`); Meta labels exactly `Use cases` and `Optional use cases` |
| Data flow | Membership built in exactly one place (`useCaseScope`) |
| Rule: no-layering | `closure`, `Roots` and `Absent` deleted, not kept as a fallback |
| Rule: principles, declare once | No Go literal and no profile file lists a member |
| Rule: rfc-compliance | Every Phase 1 row is a verbatim quote; a third-party summary (RFC 5280) names its implementer |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Every summary carries both rows | `grep -L '^| Use cases |' rfc/short/*.md` and `grep -L '^| Optional use cases |' rfc/short/*.md` print nothing |
| No profile carries a retired field | `grep -lE '^\| (Roots|Absent) \|' rfc/profiles/*.md` prints nothing |
| 36 new summaries pass | `./le rfc check` shows no problem naming any of them |
| Route-server figure over the required tier | `./le rfc tree profile route-server`: the first line's RFC count equals the required list (24); the optional section lists 12 |
| Roadmap reads two directories | `rfcDirs` in `internal/le/spec/roadmap/profiles.go` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Both rows are parsed against `profileNameRE` and the loaded use-case set; no path is built from a name outside the existing profile loader |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| A Phase 1 summary cannot be walked | Keep the spec open, record the blocker with evidence; never list the RFC without a summary |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Two mandatory rows, `Use cases` (required) and `Optional use cases` (listed, not counted), both `-` when empty | (a) a per-name marker in one row, such as `route-server, lns (optional)`; (b) optional rows | Two rows keep one plain grammar per cell, the grammar `Support` and `Aggregate` already use. Bringing an RFC into scope is moving a name between rows, and `grep '^| Use cases |.*route-server'` answers the counted set with no parser (owner: "parse it quickly"). A marker adds a second grammar inside a cell and a misspelled marker that could silently be counted. Mandatory rows mean a forgotten declaration is an error, never a silent "not owed" (`ai/rules/principles.md`); `Support` is already mandatory with `-` |
| One name in both rows is refused | the required row wins | A name in both rows is two declarations of one fact, and the reader cannot tell which was meant |
| `Roots` deleted, a `Roots` row refused | derive `Roots` from the declarations | Nothing reads a root any more: order comes from the graph level, and every member is declared. A derived copy is a second declaration with no reader |
| `Absent` deleted with the whole absent state; every listed RFC gets a summary first | keep `Absent` as the declaration for summary-less RFCs | Owner: "all RFC should have a valid summary !" (`ai/META-DECISIONS.md`, 38a3281ae1). No-layering: the field and its 0% rendering go, with no fallback. Phase 1 runs first, so no member is ever without a summary |
| `updates` edges do not pull an updater in; a `{updated-by}` marker on a member does, enforced by a check, and the updater must be in the same tier or higher | follow `updates` automatically; ignore updaters entirely | Following them automatically would pull rfc7705, rfc8654, rfc9072 and the BFD strict-mode draft into route-server as required. Ignoring them would drop rfc7606, which rewrites 13 rfc4271 rows. The marker is the checked statement that an updater changes a member's requirement. Header-only updaters appear as candidates |
| The optional tier has its own rollup in `tree`, `next` and the site, and the profile percentage is the required tier alone | count optional members at a weight; hide them | Owner: "when we count the percentage we should only count the RFC for which we have decided to bring the optional in scope." Showing them keeps the listed work visible |
| The roadmap Profiles column names required membership only | name both tiers | An optional listing places nothing in a release profile, and the column answers which release a spec serves |
| Candidates are one hop, listed only by `tree profile <name>` | the transitive closure as advisory; candidates on the site and in `next` | Transitive reach is the over-reach being removed. One hop answers "what does a member lean on that nobody decided" |
| A profile with no required member is refused | render 0 of 0 | 0 of 0 reads complete, a silent wrong value |
| RFC 6810 and RFC 4302 dropped | list them as optional | Owner: drop unless the code shows Ze ships them for the role. `rtrVersionMin = 1` means no RTR version 0. The IKE engine proposes no AH, which is configurable only for OSPFv3 IPsec |
| DNSSEC RFCs (4033, 4034, 4035) optional for fttx-edge only, never required | required | Owner: "DNSSEC is not urgent at all". RFC 4033 and RFC 4034 are listed beside 4035 because 4035 rests on them, which owes them summaries in Phase 1 |

## Known Limitations
- Phase 1 is RFC extraction work across 36 documents and dominates the spec's cost. A summary that cannot be walked keeps its tier incomplete and the spec open. It is never listed without a summary.

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
- [ ] Every user story has a working path and a passing test (N-A: Scope is tooling)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the `internal/le/rfc`, `internal/le/spec/roadmap` and `internal/le/site` package tests, `./le rfc check`, and `./le verify changed` over the closing commits (`/ze-close` step 6f). The full verification is Thomas's release check
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
- [ ] Boundary tests for all numeric inputs (N-A: none)
- [ ] Functional `.ci` tests for end-to-end behavior (N-A: le tooling, covered by package tests over fixture trees)
- [ ] Interop tests for protocol features (N-A: tooling)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/validation/spec-rfc-profile-declared-membership.md` only, in the same `./le commit create` script
