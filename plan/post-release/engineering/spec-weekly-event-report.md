# Spec: weekly-event-report

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P3 |
| Release reason | Development tooling for the weekly public update; no shipped behaviour changes |
| RFC gaps | - |
| Depends | spec-weekly-news-fragments (the `found-by` field and the fragment parser `parseNewsFragment`) |
| Phase | 4/6 |
| Handoff | - |
| Updated | 2026-11-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

WIP cap: Thomas approved claiming this spec and `spec-weekly-news-fragments` above the in-progress cap (2026-11-09 session), a one-off permission for this pair.

## Task

The weekly update's RFC section and its "what the checking found" stories need
the week's tooling events: requirements proven by a recorded break, interop
scenarios that gained a recorded red, documents whose requirement list was
signed off, test-judge approvals, and defects the checking surfaced. Today the
weekly writer rebuilds the week-on-week movement by hand from `git show` at
pinned revisions, and the published figure it needs (MUSTs proven) cannot be
read off the record files alone.

Thomas agreed (2026-11-09 session):

| Decision | Choice |
|----------|--------|
| Derived, not logged | A report derived from records the tooling already keeps, over the pinned `release-from`..`release-to` range; no new append-only log, because every fact is declared once and a shared log collides between sessions |
| Proven figure at a past revision | Export each endpoint revision into scratch and run the existing RFC producers on it, so the figure is the one the ledger publishes |
| Test-judge verdicts | Approvals counted from the trailers commits already carry; refusals get no record of their own, because the public update bans internal process and a refusal that led to an operator-visible fix reaches the report through that fix's fragment (`found-by: test-judge`) |
| Defects surfaced by a check | Marked by the optional `found-by` field on the fix's news fragment (`spec-weekly-news-fragments`), not by a journal column |

Pending Thomas's confirmation (asked 2026-11-10, option A or B), the main thread proceeds under option A: the RFC figures at each endpoint come from that endpoint's own le, built from its export and run as a subprocess (Key Design Decisions). Phase 2 feasibility found that today's producers refuse a past endpoint whose summary format changed (A-3), so "run the existing RFC producers on it" is met by running the producers that endpoint shipped. Option B, the previous post's figure, stays his to choose and would replace that row.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md` - the design every le change keeps
  → Constraint: a tool is a package under `internal/le/` that registers from `init()`; a new verb on an existing namespace is one more `leaction.Action` in that package's table, not a new package
  → Constraint: one CPU-intensive run per session; the two tree builds run one after the other, never in parallel
- [ ] `docs/contributing/rfc-conformance-gates.md` "The discrimination record" - one file per requirement, `rfc/discrimination/<stem>/<rid>.json`, an array of records
  → Constraint: a record carries no timestamp and no observed red; "when proven" comes only from git, and "proven" means the record still verifies against that revision's source (`DiscriminationVerdict.Proven`)
- [ ] `docs/architecture/testing/interop.md` - an interop red is an ordinary discrimination record whose unit is the scenario's checker function
  → Decision: scenarios that gained a recorded red are the newly proven records whose unit lies in an interop checker package, mapped to the scenario by the existing `interopScenarioOf`
- [ ] `ai/skills/ze-weekly-update.md` "The RFC MUST programme is a standing item" - the fact-to-source table the report replaces
  → Decision: that table's movement rows read the report; the "Defects the checking surfaced and fixed" row reads the report's fragment section
- [ ] `ai/skills/ze-test-judge.md` - approvals become `RFC-approved:` / `Weakening-approved:` trailers; a golden refresh carries a hand-written `Test-judge:` body line
  → Constraint: an approval no commit used leaves no trace, so the report counts approvals that landed, and says so

**Key insights:**
- Every RFC producer takes a directory root; none takes a revision. `internal/le/repo/bootstraps/tracked.go` `extract` already materialises a commit with `git archive` into scratch without touching `.git/`.
- `internal/le/spec/roadmap` is the precedent for a revision-pinned report and for registration across le packages: `roadmap.RegisterCorpusReader` is filled from `internal/le/rfc/register.go`, so `roadmap` imports none of `rfc`.
- `internal/le/weekly/answer.go` declares one action, `publish`, with `Default: true`; a second verb `report` slots into the same table.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/weekly/answer.go` - `actions` declares `le weekly` with the single default verb `publish`; `Answer` dispatches through the table
- [ ] `internal/le/rfc/tree.go` - `treeReport(tree, profile)` builds the RFC tree report of one checkout directory through `loadPriorityCorpus`
- [ ] `internal/le/rfc/profilecoverage.go` - `Proven` is the one per-row proof predicate; `provenRequirements` answers the proven requirement ids
- [ ] `internal/le/rfc/register.go` - registers the rfc namespace and fills `roadmap.RegisterCorpusReader` from `init()`
- [ ] `internal/le/commit/register.go` - registers the commit namespace from `init()`
- [ ] `internal/le/repo/bootstraps/tracked.go` - `resolveRev` and `extract`: resolve a revision to a SHA and materialise it with `git archive` into a scratch directory
- [ ] `internal/le/spec/roadmap/roadmap.go` - `Collect` resolves a revision once and reads only objects reachable from it
- [ ] `internal/le/spec/roadmap/profiles.go` - `RegisterCorpusReader`: the cross-package registration precedent, called once from `internal/le/rfc/register.go`
- [ ] `ai/skills/ze-weekly-update.md` - the RFC section's fact-to-source table, rebuilt by hand each week

**Behavior to preserve:**
- `le weekly` with no verb, and every `le weekly publish` line, behave exactly as today
- `./le rfc tree`, `./le rfc check` and `./le rfc extraction report` keep their answers; the report calls their producers, never a copy of their logic
- The published RFC figure and the report's figure are the same number for the same revision

**Behavior to change:**
- New `le weekly report from <rev> to <rev>` answers the week's tooling events through the shared renderer
- The weekly skill's RFC section and its "defects found" row read the report instead of `git show` by hand

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Weekly command table | `internal/le/weekly/answer.go` `actions` | Declares `le weekly publish` | Extends: one `report` action |
| Revision export | `internal/le/repo/bootstraps/tracked.go` `resolveRev`, `extract` | Materialises a commit with `git archive` into scratch | Extends: moved to a shared le helper package and called by both, so no second copy exists |
| Revision resolver | `internal/le/repo/objects/objects.go` `Resolve` | `rev-parse --verify --end-of-options <rev>^{commit}` to a full commit ID, refusing a leading `-` | Extends: the report resolves both endpoints with it, so no second resolver exists |
| le build from an export | `internal/le/repo/export/build.go` `BuildLe`, moved out of `internal/le/repo/bootstraps/tracked.go` `buildAndList` | Compiles an exported tree's own `cmd/ze` le personality with that tree's `DaemonBuildTags(tree, "ze_le")`, under the patched GOROOT le activated (inherited environment) | Extends: one helper, called by `./le repo bootstraps` and by the RFC source for an endpoint whose le the launcher has not built; no second copy |
| Launcher builds | `internal/le/go/toolchain/builds.go` `BuildsDir`, `ParseBuildName`, `RunnableBuild`; each build's `inputs` file (`git ls-tree <commit> -- <le inputs>`, the `le` script's `le_inputs`) | One le per commit under `bin/builds/<UTC commit time>-<short sha>/`, the newest two kept | Extends: an endpoint whose `inputs` equal a runnable build's `inputs` runs that build instead of compiling (identity of content, not of name); usually HEAD |
| RFC tree report | `internal/le/rfc/tree.go` `treeReport`, published as `le rfc tree \| json` | Mandatory proven and total over one directory | Extends: each endpoint's OWN le answers it as a subprocess on that endpoint's export (`ZE_REPO_ROOT=<export>`), parsed from `mandatory-proven` / `mandatory-total` / `walked`, keys present at HEAD and at 5064d6db1f |
| Proof predicate | `internal/le/rfc/profilecoverage.go` `Proven`, `provenRequirements` | The one definition of a proven requirement | Extends: the newly proven set is the difference of the two endpoint sets, each read from that endpoint's own le (`le rfc discriminate stem <stem> \| json`, `records` whose route proves) |
| Interop scenario mapping | `internal/le/rfc/discriminate_observe.go` `interopScenarioOf` | Maps a checker function to its scenario name | Extends: names the scenarios among newly proven records, today's function reading the checker source in the `to` export |
| Extraction sign-off | `internal/le/rfc` `extractionStatus` behind `./le rfc extraction report`; the tree row's `walked` | Which documents' requirement lists are signed off | Extends: read from each endpoint's own le; the report lists documents walked at `to` and not at `from` |
| Cross-package registration | `internal/le/spec/roadmap/profiles.go` `RegisterCorpusReader` | Lets `rfc` feed `roadmap` without `roadmap` importing it | Extends the pattern: `weekly.RegisterEventSource`, filled from each owner's `register.go`, so `weekly` imports no owner |
| Approval trailers | `internal/le/commit/actions.go` `declaredTrailers`; `rfc.ApprovalTrailer`, `testweakened.ApprovalTrailer` | Record each landed test-judge approval in the commit message | Extends: the commit package registers a source counting them over the range |
| News fragments | `spec-weekly-news-fragments`, `internal/le/commit/newsfragment.go` | Operator-facing fragment per change, optional `found-by` | Extends: a source lists fix fragments added in the range, grouped by `found-by` |
| Roadmap compare | `internal/le/spec/roadmap` `compare` | Compares the spec inventory between two revisions | Does not apply: it reports work items, not tooling events; the weekly skill keeps calling it |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le weekly report from <rev> to <rev>`, run by the weekly skill with its pinned `release-from` and `release-to`

### Transformation Path
1. Resolve both revisions to full SHAs; refuse an unresolvable one
2. List the first-parent commits in the range once
3. Export each endpoint into its own scratch directory, one after the other
4. Call every registered event source with the two directories, the two SHAs and the commit list; each answers one named section. The RFC source, for each endpoint in turn, runs that endpoint's own le (the launcher's build whose `inputs` match, else `export.BuildLe` into the run directory) as a subprocess with `ZE_REPO_ROOT` naming the export, and reads its `| json` answers; it never runs today's corpus readers on a past tree
5. Render the sections through the shared renderer; remove both scratch directories

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Git object store → scratch tree | `git archive` export | No |
| Today's le → an endpoint's own le | Subprocess (`<le> rfc ... \| json`, `ZE_REPO_ROOT=<export>`), its JSON answer parsed; the binary is a launcher build or `export.BuildLe` output | No |
| `weekly` ↔ owner packages | Registered event sources; `weekly` imports none of them | No |

### Changed-Wire Census

**Search evidence:** the report reads existing formats and writes only its rendered answer. Scope `internal/le/rfc`, `internal/le/commit`, `internal/le/test/weakened`, `internal/le/weekly`, `ai/skills`; terms `rfc/discrimination`, `RFC-approved`, `Weakening-approved`, `Test-judge:`, `found-by`. Control: `rfc/discrimination/<stem>/<rid>.json` has a known writer (`recordDiscrimination`) and reader (`loadDiscrimination`).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Discrimination records | writer | `internal/le/rfc` `recordDiscrimination` | Unchanged | Preserved |
| Discrimination records | reader | `internal/le/rfc` `loadDiscrimination`, called by the registered RFC source on each export | Existing reader | Preserved, read unchanged |
| Approval trailers | writer | `internal/le/commit/prepare.go` `Create` | Unchanged | Preserved |
| Approval trailers | reader | new commit-package event source over the range's messages | Same keys as `declaredTrailers` | Read only |
| `Test-judge:` body line | writer | agent, by `ai/skills/ze-test-judge.md` | Hand-written | Preserved; counted as a separate figure |
| `Test-judge:` body line | reader | the same commit-package event source | New reader | Read only |
| News fragment `found-by` | writer | agent (`spec-weekly-news-fragments`) | New with that spec | Owned by that spec |
| News fragment `found-by` | reader | fragment event source in `internal/le/commit`, using that spec's unexported parser `parseNewsFragment` (`newsFragment.Spec`, `Journal`, `Section`; `newsFoundBy`, `isNewsFragmentPath`) from the same package | New reader | Read only |
| Report answer | writer | `internal/le/weekly` `report` action | New | Shared renderer: text, json, yaml, table |
| Report answer | reader | `ai/skills/ze-weekly-update.md` (agent) | New | New |

### Integration Points
- `actions` in `internal/le/weekly/answer.go` - the `report` action
- `internal/le/rfc/register.go` and `internal/le/commit/register.go` - call `weekly.RegisterEventSource` from `init()`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to be filled at implementation |
| No unintended coupling (components stay isolated) | No | to be filled: `weekly` imports no owner package |
| No duplicated functionality (extends existing, does not recreate) | No | to be filled: one export helper, the existing RFC producers |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A, tooling |
| Registration over hardcoding, outbound | No | to be filled: each section is a registered source |
| Registration over hardcoding, inbound | No | to be filled: `weekly` holds no list of sources or trailer keys |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A `git archive` export of a past revision is enough for the RFC producers (no generated or export-ignored file they need is missing) | `bootstraps` builds le from such an export | The figure at a past revision errors or differs from the ledger | `TestReportProvenMatchesTreeReport` over a fixture repository, plus one run against two real revisions compared with `./le rfc tree` at the newer | confirmed for HEAD (main thread, 2026-11-10): `le rfc tree` on the export equals it on the checkout except for uncommitted and gitignored files, namely a dirty `rfc7296` record and the gitignored `iso-iec-10589` text, both absent from what git holds by design. `TestReportProvenMatchesTreeReport` still owed (phase 4b) |
| A-2 | Two endpoint builds run in acceptable time for a weekly task | Not measured | The weekly run becomes slow | Time one run at implementation and record it in the spec | confirmed for the export: the main thread measured, on HEAD c268fa58d4 (2026-11-10), a `git archive` export at 3s (373 MB) and `./le rfc tree` on it at 8s, so about 11s per endpoint and 22s for two run one after the other. A past endpoint's own le (option A) measured on 5064d6db1f: about 100s to compile its `cmd/ze` le personality from the export (patched GOROOT required: a plain toolchain fails on `tls.ZeServerChoosesSessionID`), then 11s for its `le rfc tree \| json` on its export. HEAD's le is usually already a launcher build, so a weekly run costs about one compile plus two tree answers; phase 6 times one real run |
| A-3 | The producers `treeReport`, `provenRequirements`, `interopScenarioOf` and `extractionStatus` can be called with an explicit directory root | Research: every producer takes a root | A producer reads `lepath.Root()` directly and needs a root parameter | Implementation reads each call path; a root parameter is added where missing | BROKEN (main thread, 2026-11-10): the producers take a root, but today's corpus readers refuse a past endpoint whose summary format changed in the range. On the 5064d6db1f export today's le answers "Meta field `Support status` is retired ... refusing to publish an incomplete requirement population". Each endpoint's figures therefore come from that endpoint's own le (Key Design Decisions); `interopScenarioOf` reads only checker source and stays in-process |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The report's figure differs from the published ledger figure | Weekly writer sees two numbers | The report reads each endpoint's figure from that endpoint's own le, which runs the producers that revision's ledger ran; a test compares it with `treeReport` on the same tree |
| R-2 | Scratch exports left behind after a failure fill the disk | Leftover directories under scratch | Exports live under one run directory removed on every exit path; a test fails a source midway |
| R-3 | A source failure silently drops its section | A section missing from the answer | A failed source answers its section as failed with the reason, never as empty |
| R-4 | Approval counts read as all judge work | A reader assumes refusals are included | The section states it counts approvals that landed in the range |
| R-5 | `report` changes how a bare `le weekly` line parses | A publish line breaks | `TestWeeklyBareLineStillPublishes` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The weekly writer gets a wrong or missing section; nothing user-visible in Ze until a post is approved |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `spec-weekly-news-fragments` (fragment parser, weekly skill Phase 1); `./le repo bootstraps` (shared export helper) |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `le weekly report from <a> to <b>` | → | `report` action, registered sources | `TestWeeklyReportAnswersEverySection` |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `le weekly report from <a> to <b>` on two valid revisions | Answers both full SHAs and one section per registered source, in a stable order, through the shared renderer (`| json`, `| yaml`, `| table`) | None |
| AC-2 | The RFC section | States mandatory proven over mandatory total at each endpoint, equal to that revision's own `le rfc tree` on that revision, and lists the requirement ids proven at `to` and not at `from` | A requirement proven at `from` and no longer at `to` is listed apart as lost, because a stale record is news too |
| AC-3 | The interop section | Names each scenario whose checker gained a proving record in the range, through `interopScenarioOf` at `to` | A checker that maps to no scenario is listed by its unit with the reason, rather than dropped |
| AC-4 | The extraction section | Lists documents signed off at `to` and not at `from` | None |
| AC-5 | The approvals section | Counts `RFC-approved:` and `Weakening-approved:` trailers and `Test-judge:` body lines in the range's commits, each count with the commits that carry it, and states that only approvals that landed are counted | None |
| AC-6 | The fragments section | Lists fix fragments added in the range grouped by `found-by`, with their spec or journal row | Fragments with no `found-by` are counted as one total, because they are not defects found by a check |
| AC-7 | An unresolvable revision | Refused before any export, naming the revision and the fix (a full SHA from the post's front matter) | None |
| AC-8 | A source that fails | Its section answers failed with the reason; the other sections still answer; the exit code says the report is incomplete | None: a silent empty section would read as no events |
| AC-9 | Any exit, success or failure | The run's scratch directory and both exports inside it are removed, and the answer names the run directory it used | None |
| AC-10 | `le weekly` with no verb, and every `le weekly publish` line | Parse and answer as today | None |
| AC-11 | `ai/skills/ze-weekly-update.md` | The RFC section's movement rows and the "defects found" row read `le weekly report`; the rest of its table is unchanged | None |
| AC-12 | `./le repo bootstraps` | Answers as today using the shared export helper | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWeeklyReportAnswersEverySection` | `internal/le/weekly/events_test.go` | AC-1, wiring, with stub sources | |
| `TestWeeklyReportRefusesUnresolvableRevision` | `internal/le/weekly/events_test.go` | AC-7 | |
| `TestWeeklyReportFailedSourceIsNamed` | `internal/le/weekly/events_test.go` | AC-8, R-3 | |
| `TestWeeklyReportRemovesExports` | `internal/le/weekly/events_test.go` | AC-9, R-2 | |
| `TestWeeklyBareLineStillPublishes` | `internal/le/weekly/answer_test.go` | AC-10, R-5 | |
| `TestReportProvenMatchesTreeReport` | `internal/le/rfc/weekly_source_test.go` | AC-2, A-1, R-1 | |
| `TestReportListsNewlyProvenAndLost` | `internal/le/rfc/weekly_source_test.go` | AC-2 | |
| `TestReportNamesInteropScenarios` | `internal/le/rfc/weekly_source_test.go` | AC-3 | |
| `TestReportListsNewSignOffs` | `internal/le/rfc/weekly_source_test.go` | AC-4 | |
| `TestReportCountsApprovalsInRange` | `internal/le/commit/weekly_source_test.go` | AC-5 | |
| `TestReportGroupsFragmentsByFoundBy` | `internal/le/commit/weekly_source_test.go` | AC-6 | |
| `TestExportHelperMaterialisesCommit` | `internal/le/repo/export/export_test.go` | AC-12 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | the command takes revisions, no numeric input | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | | The user is the weekly skill running an le command; `TestWeeklyReportAnswersEverySection` drives the action table, the same entry point | |

## Files to Modify
- `internal/le/weekly/answer.go` - the `report` action
- `internal/le/repo/bootstraps/tracked.go` - call the shared export helper instead of its own `extract`, and `export.BuildLe` instead of its own compile step in `buildAndList`
- `internal/le/rfc/register.go` - register the RFC event source
- `internal/le/commit/register.go` - register the approvals and fragments sources
- `ai/skills/ze-weekly-update.md` - the RFC section reads the report
- `docs/architecture/core-design.md` - declared by `internal/le/weekly/weekly.go`; the weekly tooling gains a report verb
- `docs/contributing/rfc-conformance-gates.md` - names the report as the way to see a range's movement
- `docs/architecture/testing/tracked-build-gate.md` - declared by `internal/le/repo/bootstraps/tracked.go`; names the shared export helper where it describes the `git archive` export

## Files to Create
- `internal/le/weekly/events.go` - the `report` action, `RegisterEventSource`, the run over exports
- `internal/le/weekly/events_test.go` - its tests
- `internal/le/repo/export/export.go` - the shared revision export helper
- `internal/le/repo/export/export_test.go` - its test
- `internal/le/repo/export/build.go` - `BuildLe`, the compile of an exported tree's own le personality, shared with `./le repo bootstraps` (covered by `TestTheCheckoutsOwnHeadIsClean` in `internal/le/repo/bootstraps/tracked_test.go`, which builds HEAD's le through it; a separate test would compile `cmd/ze` again)
- `internal/le/rfc/weekly_source.go` - the RFC, interop and extraction sections
- `internal/le/rfc/weekly_source_test.go` - their tests
- `internal/le/commit/weekly_source.go` - the approvals and fragments sections
- `internal/le/commit/weekly_source_test.go` - their tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | tooling only |
| YANG validation constraints | N-A | tooling only |
| YANG custom validators | N-A | tooling only |
| CLI commands/flags | Yes | `internal/le/weekly/answer.go` `report` |
| CLI grammar (keyword before value) | Yes | `report from <rev> to <rev>`, checked by `./le cli grammar` |
| Editor autocomplete | N-A | le completion reads the action table |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | Yes | answers through the shared renderer |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | `git` and `tar` are already required by le |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | N-A | tooling |
| 2 | Config syntax changed? | N-A | |
| 3 | CLI command added/changed? | N-A | le verb; `le help weekly` is generated from the action table |
| 4 | API/RPC added/changed? | N-A | |
| 5 | Plugin added/changed? | N-A | |
| 6 | Has a user guide page? | N-A | |
| 7 | Wire format changed? | N-A | |
| 8 | Plugin SDK/protocol changed? | N-A | |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | |
| 10 | Test infrastructure changed? | N-A | |
| 11 | Affects daemon comparison? | N-A | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, `docs/contributing/rfc-conformance-gates.md` |
| 13 | Route metadata keys added/changed? | N-A | |
| 14 | Prometheus counters added/changed? | N-A | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived by `./le spec citation anchors spec plan/post-release/engineering/spec-weekly-event-report.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | N-A | |

## Implementation Steps

1. **Phase: Wiring** -- `report` action, `RegisterEventSource`, stub source
   - Tests: `TestWeeklyReportAnswersEverySection`, `TestWeeklyBareLineStillPublishes`
   - Files: `answer.go`, `events.go`
2. **Phase: Export helper and feasibility** -- move `resolveRev` and `extract` to `internal/le/repo/export`; bootstraps calls it. Before any later phase, export one past revision, run `treeReport` on it, compare with `./le rfc tree` at that revision, and time two exports with their reports; record both results under A-1 and A-2. A failure returns the spec to DESIGN
   - Tests: `TestExportHelperMaterialisesCommit`, the existing bootstraps tests
3. **Phase: Run over exports** -- resolution, sequential exports, cleanup, failed-source answers
   - Tests: `TestWeeklyReportRefusesUnresolvableRevision`, `TestWeeklyReportFailedSourceIsNamed`, `TestWeeklyReportRemovesExports`
4. **Phase: RFC sources** -- 4a: `export.BuildLe` shared with bootstraps (done). 4b: for each endpoint, pick its le (the launcher build whose `inputs` file equals `git ls-tree <commit> -- <le inputs>`, else `export.BuildLe` into the run directory), run it as a subprocess with `ZE_REPO_ROOT=<export>`, and read the proven figure, the proven sets, the walked documents and the records' units from its `| json` answers; map units to scenarios with today's `interopScenarioOf` over the `to` export (A-3 broken: today's corpus readers are never run on a past tree)
   - Tests: the four `internal/le/rfc/weekly_source_test.go` tests
5. **Phase: Commit sources** -- approvals and fragments, the fragments source reading `parseNewsFragment` (`internal/le/commit/newsfragment.go`, landed with `spec-weekly-news-fragments`)
   - Tests: the two `internal/le/commit/weekly_source_test.go` tests
6. **Phase: Skill and docs** -- the weekly skill's RFC section; the two pages; `./le ai sync update`; record the timing of one real run (A-2)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | The proven figure comes from `Proven`, never from counting record files |
| Correctness | Exports run one at a time and are removed on every exit path |
| Data flow | `weekly` imports no owner package; owners register |
| Rule: user-facing errors | A refused revision names the revision and the fix |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Report command | `./le weekly report from <a> to <b> \| json` answers every section |
| Same figure as the ledger | `TestReportProvenMatchesTreeReport` passes |
| One export helper | `grep -rn 'git", "archive"' internal/le` names one site |
| Skill reads the report | `grep -n 'weekly report' ai/skills/ze-weekly-update.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Revisions resolved to full SHAs before use; never passed to a shell |
| Resource exhaustion | Scratch exports bounded to two, removed on exit |

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
| Derived report over existing records | New append-only event log | Every fact declared once; a shared log collides between sessions |
| Export each endpoint and run the existing producers | Diff record files only; blob-backed RFC readers | Only the producers give the published figure; blob readers are a large refactor |
| RFC figures at each endpoint come from that endpoint's own le (the launcher's build whose `inputs` match, else built from its export with `export.BuildLe`), run as a subprocess with `ZE_REPO_ROOT=<export>` and read through its `\| json` answer (option A, pending Thomas's confirmation) | Today's producers on the export (refuses a past summary format: A-3); the previous post's published figure (option B: approximate, no per-requirement movement, and Thomas may still choose it) | The figure is the one that revision's ledger published, whatever format its summaries used; the JSON keys read (`mandatory-proven`, `mandatory-total`, `walked`, `records`) exist at HEAD and at 5064d6db1f; the cost is one compile per past endpoint (A-2) |
| No record of test-judge refusals | One tracked file per refusal | The public post bans internal process; a refusal that matters shows through the fix's fragment (`found-by: test-judge`) |
| `found-by` on fragments marks defects found by a check | A journal column across every class file | The agent who fixed it knows; no migration of the journal |
| Sections are registered sources | One function in `weekly` calling each producer | `weekly` imports no owner, as `roadmap.RegisterCorpusReader` already does |

## Known Limitations
- Approvals that no commit used, and judge refusals, are not counted.
- Events that only exist outside the repository (a Discord thread, a lab run never recorded) are not in the report.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: `./le job run label unit-pkg command go test ./internal/le/weekly/... ./internal/le/rfc/ ./internal/le/commit/ ./internal/le/repo/export/ ./internal/le/repo/bootstraps/`, `./le cli grammar` (no `./le test unit` group covers `internal/le`)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove <the spec's path in its bucket>` with `no-news "tooling: no operator-visible change"`
