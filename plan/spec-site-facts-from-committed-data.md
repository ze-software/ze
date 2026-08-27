# Spec: site-facts-from-committed-data

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | the `le` tooling rewrite (session `feature [42630f]`), in flight 2026-08-25 |
| Phase | 5/6 |
| Deferral shard | `plan/deferrals/site-facts-from-committed-data.md` (create on the first deferral) |
| Handoff | - |
| Updated | 2026-08-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Every number the website publishes about the repository comes from ONE committed
JSON in `main`. The site build reads that file. It does not walk `main`'s working
tree.

A published figure is a claim about the repository. Today some of those figures
are claims about whatever happened to be on disk when the build ran, in a
checkout four sessions share. The two are not the same thing, and the difference
is invisible in the output.

The rounding does not change. `fmt_int` floors a magnitude to one tenth of its
visible unit and marks it with a plus, `fmt_exact` leaves the gate-checked RFC
counts alone, `data-ze-stat` markers stay, and `update-site-stats.py` keeps
refreshing published HTML without a rebuild. This spec changes where the numbers
COME FROM, and nothing about how they are shown.

The pattern is named deliberately. Sixteen other `ze-*-update` targets derive
from the working tree and none of them warns
(`plan/journal/concurrent-session-corruption.md`), so this is the seventeenth
instance of a counted class. Fixing one instance without stating the pattern is
how a class survives being recorded.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/repo-maintenance.md` - generated files and the gates that read them
  → Constraint: a generated file and its source MUST move in the same commit
- [ ] `ai/rules/architecture.md` - where a new data file belongs
  → Decision: no new package; one committed data file and a moved read
- [ ] `ai/rules/evidence.md` - what a published claim owes
  → Constraint: a number derived from an uncommitted tree is an unverified claim about the repository

### Journal
- [ ] `plan/journal/concurrent-session-corruption.md` - the class this is an instance of
  → Constraint: a regeneration reads the WHOLE working tree, so it absorbs every session's in-flight state
  → Decision: sixteen `ze-*-update` targets have this shape and none warns; this spec fixes the seventeenth and states the pattern

**Key insights:** (minimal context to resume after compaction)
- The precedent is COMPLETE, not partial. `ze-test-health-update` regenerates `test/health/latest.json`, `ze-test-health-check` gates it for staleness, and the check is a prerequisite of `ze-generated-files-check`. Its Makefile comment states the property that makes this work: "Output is a pure function of committed state -- no wall-clock value".
- `sitefacts.inventory_counts` already READS that committed file and warns when it cannot, falling back to a tree walk only for a checkout without `../main`.
- Published facts fall into five categories with different truth sources. The design's job is to name them, not to force all five into one.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `website/tools/sitefacts.py` - builds the facts from five different kinds of source
- [ ] `website/tools/sitelib.py` - `number_token_specs`, `substitute_number_tokens`, `stat_span`
- [ ] `website/tools/check_site_stats.py` - `check_html_stat_markers`, `update_html_stat_markers`, `check_source_tokens`
- [ ] `website/tools/update-site-stats.py` - refreshes published HTML without a rebuild
- [ ] `Makefile` - `ze-test-health-update` / `ze-test-health-check`, the complete precedent
- [ ] `website/presentations/tools/loc_activity.py` - `display_magnitude`, the second copy of the rounding

**The five categories, and which are wrong:**

| Category | Producer | Truth source | Correct today? |
|----------|----------|--------------|----------------|
| Committed data | `count_go_tests` via `inventory_counts` | `test/health/latest.json` | Yes. This is the pattern |
| Working tree | `count_repo_annotations`, `count_go_packages`, `count_interop_targets`, `count_interop_scenarios` | every tracked `.go`; `go list ./...`; a scan of `test/interop/` | Was no, a dirty tree changed a published number. All four now read `website/data/repo-facts.json`, and their walks are the no-sibling fallback |
| Built binary | `count_cli_commands`, `count_config_sections` via `ze_json` | the compiled `ze` | Decided: they stay live and the file NAMES them (A-2). A value here would be a claim about a binary rather than about a commit |
| Network | `github_stars` | api.github.com | Cannot be committed truth. Already preserves the last published value on failure |
| Site-owned | `count_changes`, `count_blog_articles` | the build root's own `changes/`, `blog/` | Yes. Not about `main` at all |

**Behavior to preserve:**
- `fmt_int` rounding and its plus suffix, unchanged.
- `fmt_exact` staying exact for the RFC counts, which `scripts/dev/rfc_requirements.py` compares against the repository.
- `data-ze-stat` markers and `update-site-stats.py` refreshing published HTML with no rebuild.
- Building from a checkout with NO sibling `../main`: `write_facts` preserves the last published values rather than publishing zeros.
- `github_stars` keeping the last published value when the network fails.
- Every existing `{{ze:...}}` token name. A renamed token ships as literal text.

**Behavior to change:**
- `count_repo_annotations`, `count_go_packages`, `count_interop_targets` and `count_interop_scenarios` stop deriving from the working tree during a build. Those four are every published fact whose derivation walked a tree.
- The facts a build reads about `main` become committed data with a regeneration target and a staleness gate.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A person runs the regeneration target in `main`, in the tree they are about to commit.
- Output: one JSON file, exact values, committed with whatever changed the counts.

### Transformation Path
1. The regeneration target walks the tree ONCE, deliberately, and writes the committed JSON.
2. `sitefacts.build_facts` reads that JSON instead of re-deriving.
3. `fmt_int` floors each magnitude at render; `fmt_exact` leaves gate-checked counts alone.
4. `substitute_number_tokens` writes the rounded value into Markdown, and the same value plus a `data-ze-stat` marker into HTML.
5. `site.js` `initRepoStats` patches the marked spans at runtime from the published `data/site-facts.json`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| main ↔ website build | committed JSON read at build time, never a tree walk | No |
| build ↔ published site | `data/site-facts.json` plus `data-ze-stat` markers | No |
| published page ↔ browser | `initRepoStats` fetches the facts and rewrites marked spans | No |

### Integration Points
- `sitefacts.inventory_counts` - the existing committed-data reader this generalises
- `ze-test-health-update` / `ze-test-health-check` - the target-plus-gate pair to copy
- `check_site_stats.check_html_stat_markers` - already asserts every marker matches the facts

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A committed facts JSON is cheaper to review than prose churn across pages | Owner statement, 2026-08-25: "we should have the number in ONE file only on main as json with all the real data" | The churn moves rather than shrinks, and one file changes on every count change | Compare the diff of one count change before and after | confirmed: a count that moves without crossing a rounding boundary changes ONE line of `website/data/repo-facts.json` and no published page (`test_a_sub_boundary_move_changes_no_page`), and a count that crosses one changes that line and the page (`test_a_boundary_crossing_changes_the_page`). Both render through `sitelib.substitute_number_tokens`, so the comparison is of published bytes rather than of the display strings alone |
| A-2 | The binary-derived counts can join the committed JSON on the same cadence | `sitefacts.ze_json` runs the built `ze`, so they need a binary that matches the tree | The JSON is authoritative for some facts and live for others, a seam the design must name rather than discover | Read `ze_json` and its two callers; decide in step 4 | broken: they cannot, and the file names them instead. `ze_json` (`website/tools/sitefacts.py`) runs `zebinary.resolve(MAIN_REPO)`, so `count_cli_commands` and `count_config_sections` are claims about a binary somebody built rather than about a commit, and no regeneration from a tree can produce them. `repo-facts.json` records both under a `live` key with `category: built-binary` and the command that answers them (`internal/le/sitefacts/sitefacts.go`, `liveFacts`), so a reader can tell an uncommitted fact from a forgotten one. `TestTheFileRecordsTheFactsItCannotDerive` holds them to carrying no value |
| A-3 | Rounding makes a magnitude insensitive enough that a slightly stale JSON publishes the same string | `fmt_int` floors to one tenth of the visible unit | A stale JSON publishes a visibly wrong number | Test a count moved by 1 near a boundary and far from it | confirmed: 3852 renders `3,800+` and holds that string for a drift of 47; 687 renders `680+` and holds it for 6. A move of 1 changes the string only where it crosses the step, so 689 renders `680+` and 690 renders `690` |
| A-4 | No gate reads `data/site-facts.json` expecting live derivation | `check_site_stats.check_html_stat_markers` compares markers to the facts file | A gate reddens on a tree the person did not regenerate | Grep every reader of `site-facts.json` | confirmed: every reader goes through `sitefacts.write_facts` or `sitefacts.load_facts`, and none counts for itself. `render-site-facts.py` and `update-site-stats.py` write it; `check_site_stats.py` loads it in `check_html_stat_markers` and `update_html_stat_markers` and compares the markers against what the same build wrote; `assets/js/site.js` fetches the published file at runtime in `initRepoStats`. Moving the source under `build_facts` therefore reaches all of them at once |
| A-5 | Every published fact about `main` can be a pure function of committed state | `ze-test-health-update`'s Makefile comment claims exactly this for its own output | A fact that is not pure cannot be gated for staleness, and needs the live path | Attempt the regeneration twice on one commit and diff | broken, and the design says so rather than forcing it: EVERY fact is not, and two are named. `count_cli_commands` and `count_config_sections` run a built binary (A-2), so they are recorded as `live` and gated by nothing. Every OTHER published fact about `main` is pure, and two runs over one tree write byte-identical files (`TestUpdateWritesTheSameBytesTwice`). The purity that matters for the gate is over a COMMIT, not over a working tree: `le site-facts check` materializes HEAD before it derives, because `go list` reads a tree and a tree moves under a shared checkout |
| A-6 | The regeneration target and its staleness gate are ONE `internal/le/<name>/` package, registered through `internal/le/leroot.Register` plus one blank import in `internal/le/register.go` | `internal/le/vendorweb/actions.go` is that shape today: `leaction.New` holds a table where each action answers `(any, int)` and `Writes: true` marks the one that rewrites the tree. Corrected from an earlier guess of `scripts/le/`, which `feature [42630f]` reported as the wrong address on 2026-08-26 | The target lands in the tree the migration is emptying, and has to move again | Read `internal/le/vendorweb/actions.go` and `internal/le/leroot.Register` before step 2 | confirmed: `internal/le/sitefacts/register.go` registers `site-facts` through `leroot.Register`, `internal/le/register.go` blank-imports it, and `./bin/le site-facts` lists both actions |
| A-7 | An action answering `(any, int)` gets `\| json`, `\| yaml` and `\| table` with no per-tool rendering code | `feature [42630f]`, 2026-08-26, describing the `internal/le` contract | The staleness gate writes its own output formatting, and AC-6 costs more than a table | Run an existing `internal/le` action through each pipe before step 5 | confirmed: `./bin/le site-facts \| json` rendered the action table, with no rendering code in `internal/le/sitefacts` |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The committed JSON goes stale and the site publishes yesterday's repository | A number visibly behind the tree | A staleness gate on the `ze-test-health-check` model, wired as a prerequisite of `ze-generated-files-check` |
| R-2 | The staleness gate itself walks the working tree, reintroducing the defect it exists to catch | The gate disagrees with itself between two sessions in one checkout | It runs against a COMMIT, the way `make ze-verify-worktree` does |
| R-3 | The regeneration becomes a build step again for convenience, and the class returns | A site build that rewrites the committed JSON | The build reads and never writes it; only the target writes |
| R-4 | Someone regenerates in a dirty tree and publishes another session's work | The JSON diff carries counts nobody in this session changed | The target warns, naming the dirty paths. This is the class-wide fix the journal asks for |
| R-5 | Splitting facts across five categories confuses which is authoritative | Two numbers for one thing, or a stale number nobody can explain | The JSON records each fact's category, so a reader can tell what it is a claim about |
| R-6 | This spec is written against tooling the `le` rewrite is replacing, and the target lands in the wrong place or twice | A `Makefile` recipe here that is not the one-line shim every neighbouring target uses | Start no implementation until the `le` rewrite lands; re-read `internal/le/vendorweb/actions.go` at step 2 and correct A-6 |
| R-7 | The blank import in `internal/le/register.go` is committed without the package it names, so `cmd/ze` stops building for every session and for CI | `go build ./cmd/ze` fails at HEAD while it passes in the author's tree | The import and the package move in ONE commit. This is the rule the four generated-file incidents share (`plan/journal/concurrent-session-corruption.md`), and this checkout has paid it twice: `le.process.stream`, then `internal/le/weekly` on 2026-08-26 |
| R-8 | `leroot.Register` panics at init on a Meta missing Description, Mode or Section | `le` panics on startup for every command, not only the new one | Fill all three at registration. `internal/le/leroot.Register` documents the requirement and panics rather than rendering a blank help row |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Published numbers go stale or wrong. No daemon behavior, no wire behavior, no operator-facing surface |
| How is it reverted? | Single commit revert; the build derives from the tree again |
| Who else touches this path? | Every session whose commit changes a count, and the site build. `plan/journal/concurrent-session-corruption.md` records four sessions colliding on generated files in one night |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| the regeneration target run in `main` | → | the facts writer | `test_regeneration_writes_every_published_fact` |
| `sitefacts.build_facts` during a site build | → | the committed-JSON reader | `test_build_facts_reads_committed_data_not_the_tree` |
| a dirty working tree at regeneration time | → | the dirty-tree warning | `test_regeneration_warns_when_the_tree_is_dirty` |
| a published page carrying a marker | → | `check_site_stats.check_html_stat_markers` | `test_published_markers_match_the_committed_facts` |
| the staleness gate in `ze-generated-files-check` | → | the facts staleness check | `test_staleness_gate_names_the_target_that_fixes_it` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A site build runs with an edited but uncommitted `.go` file adding a `// Design:` line | The published count is unchanged from the committed facts |
| AC-2 | The regeneration target runs in a clean tree | The committed JSON holds the exact value for every published fact about `main` |
| AC-3 | The regeneration target runs in a dirty tree | It warns, naming the dirty paths, before writing |
| AC-4 | A count crosses a rounding boundary and the JSON is regenerated | The published Markdown and HTML both show the new rounded value |
| AC-5 | A count moves without crossing a boundary and the JSON is regenerated | The JSON changes, and no published page changes |
| AC-6 | The committed JSON is older than the tree | The staleness gate reports which facts are stale and names the target that fixes them |
| AC-7 | The site is built from a checkout with no sibling `../main` | The last published values are preserved, exactly as today |
| AC-8 | An RFC count is published | It is exact, unrounded, and agrees with `scripts/dev/rfc_requirements.py` |
| AC-9 | The regeneration runs twice on one commit | Both runs produce a byte-identical file |
| AC-10 | The GitHub star fetch fails | The last published value is kept, and the build does not fail |

AC-2's "every published fact about `main`" reads as Design Insights defines it:
every fact whose derivation WALKS. `count_direct_dependencies` and
`count_rfc_requirements` each read ONE committed artifact, so they are already
committed data and a copy would be a second record of one fact. The two
binary-derived counts cannot be committed at all (A-2), and the file records
them as `live` rather than leaving them unmentioned.

## 🧪 TDD Test Plan

### Unit Tests

The regeneration and its gate are Go, so their tests are Go. The rows below name
where each test LANDED: a test of `le site-facts` written in Python could only
fork the tool and read its output back.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_build_facts_reads_committed_data_not_the_tree` | `website/tools/test_sitefacts.py` | AC-1: an uncommitted edit does not move a published count | green |
| `test_interop_counts_come_from_the_committed_facts` | `website/tools/test_sitefacts.py` | AC-1 for the interop figures | green |
| `TestUpdateWritesTheDerivedFacts`, `TestUpdateCountsTheInteropSuiteGitHolds`, `TestTheFileRecordsTheFactsItCannotDerive` | `internal/le/sitefacts/sitefacts_test.go` | AC-2: every fact the tool owns is written, and the two it cannot derive are named | green |
| `TestUpdateNamesTheGoFilesNoCommitHolds` | `internal/le/sitefacts/sitefacts_test.go` | AC-3 | green |
| `test_a_sub_boundary_move_changes_no_page`, `test_a_boundary_crossing_changes_the_page` | `website/tools/test_sitefacts.py` | AC-4 and AC-5: the churn this spec exists to stop, and the move that must still reach a reader | green |
| `TestUpdateWritesTheSameBytesTwice` | `internal/le/sitefacts/sitefacts_test.go` | AC-9: pure function of committed state | green |
| `test_missing_sibling_repo_preserves_published_values` | `website/tools/test_sitefacts.py` | AC-7 | green |
| `test_rfc_counts_stay_exact` | `website/tools/test_sitefacts.py` | AC-8 | green |
| `test_star_fetch_failure_keeps_the_last_value` | `website/tools/test_sitefacts.py` | AC-10 | green |
| `TestCheckAgreesWithTheCommitItJudges`, `TestCheckJudgesTheCommitAndNotTheWorkingTree`, `TestCheckNamesTheStaleFactAndTheFix`, `TestCheckReportsAFileTheCommitDoesNotHold` | `internal/le/sitefacts/sitefacts_test.go` | AC-6 and R-2: the gate judges a commit, names what is stale, and names the action that fixes it | green |
| `TestRenderPublishesWhatTheSiteShows` | `internal/le/sitefacts/sitefacts_test.go` | the gate's idea of a published figure is the site's own, measured from `fmt_int` | green |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| a rounded magnitude near a boundary | 3600 - 3700 | 3,699 renders `3,600+` | N/A | 3,700 renders `3,700+` |
| the first rounding step | 99 - 100 | 99 renders `99` | N/A | 100 renders `100` |
| the second rounding step | 999 - 1000 | 999 renders `990+` | N/A | 1000 renders `1,000` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ze-site-facts-check`, run by `ze-generated-files-check` | `Makefile`, `scripts/le/application/repository.py` | A reader of the site sees a number the repository agrees with: the gate runs on every `ze-precommit-verify`, over the last commit | green, and OPEN for the main thread -- see below |

`test/website/*.ci` was the location this spec named before the tool landed in
`le`, and it is the wrong one. A `.ci` runs the `ze` daemon under the functional
runner (`test/` holds no `website` directory), while the subject here is a make
gate reading a git commit. Writing one would be a test of nothing.

What replaced it is the gate itself, wired as a prerequisite of
`ze-generated-files-check`, plus the four `internal/le/sitefacts` tests that drive it
over a fixture checkout with a real commit, a real worktree and a real
`go list`. `TestRegenCheckReadonlyCoversGenerators`
(`scripts/status/verify_run_test.go`) holds the wiring: it refuses an
undocumented prerequisite and refuses a generator with no read-only check.

## Files to Modify
- `website/tools/sitefacts.py` - read committed data; the tree walks survive only as the no-sibling fallback
- `website/tools/sitelib.py` - token specs, if a fact name changes
- `website/tools/check_site_stats.py` - UNCHANGED, and that is the finding. It loads the facts through `sitefacts.load_facts` and compares the published markers against them, so moving the source under `build_facts` reached it with no edit (A-4). The staleness check is `internal/le/sitefacts` `check`, in Go, because a second one in Python would be a second derivation
- `scripts/le/application/repository.py` - the two gate rows the Makefile shims reach
- `Makefile` - the two one-line shims, in the spelling every neighbour uses (`@$(CURDIR)/le repository <gate>`)
- `docs/functional-tests.md` - the staleness functional test

## Files to Create
- the committed facts JSON in `main` - exact values, each recording its category
- `website/tools/test_sitefacts.py` - the unit tests above
- ~~`test/website/*.ci` - the staleness functional test~~ NOT created: the runner it names launches the `ze` daemon and the subject is a make gate over a git commit. The Functional Tests table records what replaced it
- `internal/le/sitefacts/` - one package holding both actions, modelled on `internal/le/vendorweb`: a regenerate action with `Writes: true` and a check action without it
- `internal/le/register.go` - one blank import, committed in the SAME commit as the package it names
- `docs/architecture/site-facts.md` - the committed-source-of-truth pattern, written for the other sixteen targets

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Build tooling; no config surface |
| YANG validation constraints | N-A | No YANG leaf added |
| YANG custom validators | N-A | No YANG leaf added |
| CLI commands/flags | N-A | No `ze` command changes; the regeneration is a make target |
| CLI grammar (keyword before value) | N-A | No CLI surface |
| Editor autocomplete | N-A | No YANG leaf added |
| Functional test for new RPC/API | N-A | No RPC; the functional test covers staleness instead |
| Pipe completeness | N-A | No CLI output |
| Env var registration | N-A | No `environment/` leaf; the target takes no env var |
| Doctor check for runtime dependencies | N-A | No runtime dependency: the file is build-time data, read by the site build and by nothing the daemon runs |
| Prometheus counters/metrics | N-A | No runtime state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Build tooling; no user-facing feature |
| 2 | Config syntax changed? | No | No config surface |
| 3 | CLI command added/changed? | No | No `ze` command changes |
| 4 | API/RPC added/changed? | No | No RPC |
| 5 | Plugin added/changed? | No | No plugin |
| 6 | Has a user guide page? | No | Contributor tooling, not an operator topic |
| 7 | Wire format changed? | No | No protocol surface |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC counts are read, never changed; AC-8 keeps them exact |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` gains the staleness test |
| 11 | Affects daemon comparison? | No | No daemon behavior |
| 12 | Internal architecture changed? | Yes | `docs/architecture/site-facts.md`, naming the pattern for the other sixteen targets |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/core-design.md` -- declared by `internal/le/register.go` and by every `internal/le/*/register.go` as "le's composition, one import per tool". This spec adds one more tool to that composition and changes nothing about how composition works, so the doc is unaffected. Re-derive with `python3 scripts/dev/spec_doc_anchors.py plan/spec-site-facts-from-committed-data.md` after the code exists |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Any doc showing a site build command gains the regeneration target |

## Implementation Steps

Phase 1 is wiring. Each phase writes its test first.

1. **Wiring.** `sitefacts.build_facts` reads the committed JSON for ONE fact, end to end, proven by `test_build_facts_reads_committed_data_not_the_tree`. Nothing else moves until an uncommitted edit provably fails to change a published count.
2. **Regeneration target.** One target a person runs, beside `ze-test-health-update`, writing every published fact about `main` exactly. It warns on a dirty tree (AC-3), which is the half that stops this becoming instance eighteen.
3. **Move the working-tree facts.** `count_repo_annotations` and `count_go_packages` read committed data. Their tree walks survive only as the no-sibling fallback, exactly as `count_go_tests` already does.
4. **Decide the binary-derived seam.** `count_cli_commands` and `count_config_sections` run the `ze` binary. Either they join the committed JSON, or the JSON records them as live and says why. A-2 stays unvalidated until this phase closes it.
5. **Staleness gate.** Modelled on `ze-test-health-check` and wired as a prerequisite of `ze-generated-files-check`. It runs against a COMMIT, never the working tree (R-2), and names the target that fixes it (AC-6).
6. **Document the pattern.** `docs/architecture/site-facts.md`, written so the remaining sixteen targets have a template rather than a precedent buried in one tool.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:symbol |
| Correctness | The regenerate action and the check action derive a fact ONE way, not two. Two derivations over one tree drift by construction, which `sitefacts.inventory_counts` already records as the reason ONE counter exists |
| Correctness | The check judges a COMMIT, never the working tree (R-2). A check that reads the tree answers differently in two sessions of one checkout, and is the defect it exists to catch |
| Data flow | `build_facts` reads the committed JSON. No build path walks `main` for a fact the JSON holds; the tree walk survives ONLY as the no-sibling fallback |
| Naming | Each fact keeps its existing `{{ze:...}}` token name and its `site-facts.json` path. A renamed token ships as literal `{{ze:...}}` text, which `check_source_tokens` does not catch on a page it does not glob |
| Composition | `internal/le/register.go`'s blank import and `internal/le/sitefacts/` are in ONE commit (R-7). Verify with `go build ./cmd/ze` at the commit, not in the working tree |
| Registration | `leroot.Register` carries Description, Mode and Section. A Meta missing one panics at init, so `le` fails to start for EVERY command (R-8) |
| Rule: `ai/rules/repo-maintenance.md` | The committed JSON and whatever changed its counts move in the same commit |
| Rule: `ai/rules/evidence.md` | No published number is derived from a path this spec did not read |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `internal/le/sitefacts/` registers as an `le` command | `./le --help` lists it; `./le sitefacts` prints the action table |
| The regenerate action is marked as writing | `./le sitefacts` shows the writes marker on that row and not on the check |
| The committed facts JSON exists and is exact | `git ls-files` names it; its values match a fresh derivation on a clean tree |
| The staleness gate runs in the standard place | `grep ze-generated-files-check Makefile` names it as a prerequisite |
| `cmd/ze` builds at HEAD | `make ze-repository-tracked-build-check` |
| An uncommitted `.go` edit does not move a published count | `test_build_facts_reads_committed_data_not_the_tree` |

## Design Insights

**A fact read from ONE committed artifact is already committed data. Only a SCAN
has to move.** Phase 3 closed the two annotation counts and the package count,
and asked what to do with the four remaining build-time reads of `main`. They
are not one kind of thing, and treating them as one would either leave a scan in
place or duplicate a gated fact.

| Fact | Reads | Moves? | Why |
|------|-------|--------|-----|
| `count_interop_targets`, `count_interop_scenarios` | a directory scan of `test/interop/` | Yes | A scan counts whatever is on disk, so another session's in-flight scenario directory changes a published number. This is the defect the spec is about |
| `count_direct_dependencies` | `go.mod` | No | One committed file, authored not derived. Reading it is reading committed state |
| `count_rfc_requirements` | `ai/RFC-REQUIREMENTS.md`, `rfc/enrolled.txt` | No | Already generated AND staleness-gated. A copy in `repo-facts.json` would be a SECOND record of one fact, which is the drift `inventory_counts` exists to prevent, and AC-8 requires these exact and compared by `scripts/dev/rfc_requirements.py` |

The rule the table encodes, for the other sixteen targets: move a fact when its
derivation WALKS, leave it when it READS one artifact that is itself committed
and gated. `repo-facts.json` records the category either way, so a reader can
tell which they are looking at.

## Key Design Decisions

**The Makefile shim routes to the PYTHON `le`, and the Python gate runs the GO
one.** Phase 1 left this open: every neighbouring recipe spells
`@$(CURDIR)/le repository <gate>`, and `./le` is the Python entry point while
this tool lives in `cmd/ze`. Nothing in the Makefile names `cmd/ze` today, so a
recipe pointing there would have been the only one of its kind, and the shim
would have stopped matching its neighbours the day the migration flipped them
all at once.

So the shim is the ordinary one, and the two rows it reaches
(`scripts/le/application/repository.py`) carry
`argv=_go('./cmd/ze', 'site-facts', <verb>)`. One derivation stays behind both
entry points: `internal/le/sitefacts` `derive` is the only counter, and a Python
re-implementation would have been the second counter over one tree that
`inventory_counts` exists to prevent. `parity.Claim` names both gates, so the
census counts them as ported rather than as gates the Go `le` invented.

When the Makefile routes to the Go `le` directly, the two Python rows are
deleted and nothing else moves.

**Staleness is judged on the PUBLISHED figure, not the exact count.** `git
ls-files` answers from the index, so a regeneration run before `git add` cannot
count the files that same commit adds, and a gate demanding exact equality would
go red on the commit that fixed it. `render` (`internal/le/sitefacts/staleness.go`)
is `fmt_int` in Go, and its table was measured against the Python rather than
derived from it.

## Known Limitations

- The churn moves rather than vanishes. One JSON changes whenever a count changes. That is the trade the owner chose: a one-line diff instead of prose rewritten across pages.
- `github_stars` can never be a pure function of committed state. It stays live, with the existing last-published fallback.
- This fixes one of seventeen instances. Making every `ze-*-update` target warn on a dirty tree is the class-wide fix, and it is separate work.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
