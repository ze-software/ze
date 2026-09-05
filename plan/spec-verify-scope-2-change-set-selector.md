# Spec: verify-scope-2-change-set-selector

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | `plan/spec-verify-scope-0-umbrella.md` |
| Phase | 1-5 done (the selector, its classification table, the narrowed fail-open, and the consumers), plus `test/runner/verify-scope-selector.ci` and the rules text. Closure re-landed phase 5's publication, which the le-personality port had dropped (`plan/journal/refactor-removes-feature.md`, 2026-09-02) |
| Handoff | - |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Build one selector that answers, for a changed file set: which Go packages must
be retested, which build-tag features the change can reach, and which it cannot.
Every later consumer reads that one answer.

Today the closest thing is `internal/le/`, and it is wrong in
both directions:

- **It over-selects.** It expands the changed set by the TRANSITIVE reverse
  dependency closure, taken from `go list -f '{{.Deps}}'`. 38 of 646 packages
  carry 200 or more transitive importers (`internal/core/textbuf` 545,
  `internal/core/stringsx` 458, `internal/core/env` 454). One edit under
  `internal/core/` selects a third of the tree, which is why
  `./le verify current mode changed` measured 4760s against the full run's 4418s.
- **It under-selects.** It builds that graph with `go list ./...` and no build
  tags, so every import inside a `//go:build ze_<feature>` file is invisible.
  `internal/component/ssh` reports ZERO importers, because `service_ssh.go` and
  `session_factory.go` (`cmd/ze/hub/`) are both compiled only under `ze_ssh`. An
  SSH change retests nothing in `cmd/ze/hub`.

The ingredients for a correct answer exist and nothing joins them:

| Ingredient | Producer | What it gives |
|---|---|---|
| package to build tag | `feature-gates.txt`, parsed by `loadFeatureTags` (`internal/le/plugin/imports/pluginimports.go`) and `load_feature_gates` (`internal/le/`) | 142 package rows over 36 tags |
| first-party reverse import graph | `collect_edges` (`internal/le/`) | built fresh on every run, never persisted, no tag awareness |
| per-file build constraint | `file_requires_tag` (`internal/le/`) | the dimension the graph lacks |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - the certificate and per-path manifest one verification run records
- [ ] `ai/rules/architecture.md` - tier rules and where a new tool belongs
  → Constraint: a config-driven engine belongs in `internal/component/`; a build tool belongs under `internal/le/`
- [ ] `ai/rules/evidence.md` - a guard must fail closed
  → Constraint: a zero value must never be a valid-looking answer. An empty selection must mean "select everything", never "select nothing"

**Key insights:**
- `dep_audit.py` already does the prefix match a selector needs, in `_same_feature_importer`.
- `tagFor` (`internal/le/plugin/imports/pluginimports.go`) resolves an import path to its tag by suffix-with-boundary match, and `loadFeatureTags` derives both `<pkg>` and `<pkg>/yang`.
- `all_ze_radius_ze_l2tp.go` proves tag membership is not one tag per file. The selector must handle a multi-tag combination.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/` - the transitive, untagged expansion, and the `PATHSPECS` / `PYTHON_TEST_PKG` mapping
- [ ] `internal/le/` - the coarse six-group mapping used by `ze-unit-test-race-changed`
- [ ] `internal/le/` - `collect_edges`, `load_feature_gates`, `_same_feature_importer`, `file_requires_tag`
- [ ] `internal/le/plugin/imports/pluginimports.go` - `loadFeatureTags`, `tagFor`
- [ ] `feature-gates.txt` - the manifest itself

**Behavior to preserve:**
- The non-Go inputs `changed-pkgs.sh` already handles: a `.py` or an `rfc/` change maps to `./scripts/dev`, because `python_tests_test.go` executes them. Dropping that reopens a hole the comment records.
- `vendor/` stays excluded.
- Output stays one `./`-prefixed package directory per line, sorted and unique. `_./le changed scope` and `_ze-unit-test-changed-impl` both consume it as a word list.

**Behavior to change:**
- Reverse dependencies become tag-aware, so a gated importer is visible.
- The expansion stops being the full transitive closure.
- The selector also answers, separately from the package list, which feature tags the change can reach.

**→ Decision (2026-08-19, main thread, from the measurements below): a tag-aware
graph at DEPTH 2. Not depth 1, not the transitive closure, and no
test-ownership map.**

Measured over 646 packages, and replayed over the last 20 commits that carried
non-vendor `.go` changes (mean selected packages):

| Bound | Mean selected | Recovers of closure |
|-------|---------------|---------------------|
| seed only | 2 | - |
| depth 1 | 20 | 34% |
| depth 2 | 50 | 85% |
| depth 3 | 57 | 97% |
| transitive | 59 | 100% |

Three facts decide it:

- **Nothing above depth 2 is worth having.** Depth 3 lands within 3% of the full
  closure, so the real choice is depth 1 against depth 2.
- **Depth 1 measurably loses coverage, and nothing cheap tells us where.** For
  `internal/core/family`, whose own tests reach 56.7%, the union of its 77 direct
  importers reaches 73.8% and the whole tree reaches 91.9%: a 17 point loss at
  depth 1. `coverage.out` (`internal/le/testunit/groups.go`) is produced with no `-coverpkg`,
  so no cross-package coverage data exists to separate a well-covered leaf from
  a weak one. `ai/rules/simplicity.md` allows cutting machinery, never
  correctness, and under-testing is a correctness cut.
- **Depth is not where the hour is.** `./le functional` (1472s) and
  `./le staticcheck-feature-matrix check` (874s) are 2346s of the 4418s run, and
  sub-spec 3 scopes both from the FEATURE-TAG answer, not from the package
  depth. Depth reaches the unit stages alone, so buying the last 15% of the
  closure back costs little and removes the argument entirely.

**→ Decision: the tag-aware graph is adopted unconditionally, because it is
free.** One `go list -tags '<ze_core + 36 feature tags>' -f '{{.Imports}}' ./...`
costs 2.60s against 2.92s for today's untagged `{{.Deps}}` run, and it turns
`internal/component/ssh`'s importer count from 0 into 1 (`cmd/ze/hub`), which is
AC-1. A per-tag loop is REFUSED: 37 sequential runs cost 94.6s, which is 315% of
AC-6's whole 30s budget. `ZE_FEATURES` is already derived from
`feature-gates.txt` at `internal/le/` native action tables, so the tag string needs no second source.

**→ Decision: a test-ownership map is REFUSED for this spec.** Nothing in the
repository relates a test to the package it exercises: `coverage.out` carries no
cross-package data and is gitignored, `test/health/latest.json` holds only
repo-wide aggregates, `ai/PACKAGE-MAP.md` skips `_test.go` by construction, and
no `.ci` file carries a package annotation. Building one means a new artifact
with its own staleness gate, which is R-2. The nearest precedent, `PREFIX_GROUP`
(`internal/le/`), is 11 hand-maintained prefixes, which is the
drift shape to avoid.

**→ Decision (2026-08-19, main thread, from a measurement on the live tree):
CLASSIFY the common non-Go kinds. Do not let them widen.**

Measured after phases 1-4 landed, on this checkout as it stands:

```
$ ./le changed scope ARGS="--print=both"
verify-scope: `internal/le/hookruntime/lifecycle.go` is a path kind the selector
  does not classify, so every package and every feature is selected
# packages
./...
```

One modified shell script widens the whole run, so the selector delivers no
scoping at all on a realistic tree. The fail-open is correct and stays; the
CLASSIFICATION is what is too narrow.

The principle, and it is already the repository's: a non-Go file is rarely
read by every package and rarely by none. It is an INPUT to a specific test,
and the map from input to reader already exists in one form -- `changed-pkgs.sh`
sends every `.py` and every `rfc/` path to `./scripts/dev`, because
`python_tests_test.go` is the only package that executes Python. The same
reasoning reaches the rest:

| Kind | Read by | Seeds |
|------|---------|-------|
| `.sh`, `.py` under `.claude/hooks/` | `hook-parity-check.py`, `hook-fixture-check.py`, run by `python_tests_test.go` | `./scripts/dev` |
| `.md` under `ai/`, `plan/` | the rules, journal and citation suites, same runner | `./scripts/dev` |
| `.md` under `docs/` | `docs_to_code.py` plus `doc_drift.go` | `./scripts/dev`, `./scripts/docvalid` |
| `internal/le/` native action tables, `internal/le/` | `functional_suite_test.py`, `doc_drift.go`, `github_workflows_test.go` | `./scripts/dev`, `./scripts/docvalid`, `./scripts/status` |
| `.yml` under `.github/` | `github_workflows_test.go` | `./scripts/dev` |
| `.ci`, `.et`, `.wb` under `test/` | `ci_fixture_test.go` and `TestCIPeerBlockCorpusParses` (`internal/test/runner`) walk the whole committed corpus; `walkFirstPartyFiles` (`internal/le/repository/`) reads their content too | `./internal/test/runner`, `./scripts/dev`, `./scripts/checks` |

**→ Correction (2026-08-19, main thread).** The row above first read "no Go
package compiles them and no unit test reads it, so this kind seeds nothing".
That was FALSE and it was mine: I asserted what reads the `.ci` corpus without
grepping for its readers, which is what `ai/rules/evidence.md` forbids and what
I had been requiring of every agent. Three Go tests read it. `ci_fixture_test.go`
walks every committed `.ci` and fails on a malformed BGP frame;
`TestCIPeerBlockCorpusParses` (`peer_block_directive_test.go`) parses the whole
corpus and is deliberately fatal rather than skipping when the tree moves;
`walkFirstPartyFiles` (`internal/le/repository/`) treats `.ci`, `.sh`,
`.mk` and `internal/le/` native action tables as relevant and reports on their content.

`toolingPackages` was wrong for the same reason: it named `internal/le/`,
`internal/le/` and `internal/le/` and omitted `./scripts/checks`, whose
tests read every one of those kinds. So the `internal/le/` native action tables, `internal/le/`, `.github/`,
`.claude/hooks/` and `docs/` rows all under-selected as well.

The consequence was measured, not theorised: a `.ci`-only change selected NO
package, and `_ze-unit-test-changed-impl` (`internal/le/` native action tables) prints "No changed Go
packages to test" and exits 0 on an empty list. That is the silent narrowing
this spec names as the one failure the selector must never have, introduced by
the very rule written to stop over-widening.

This is a hand-written table, and the earlier refusal of a test-ownership map
still holds: that map would have to relate every test to every package it
exercises, which nothing in the repository knows. This one relates a FILE KIND
to the two or three tooling packages that read it, it is the same shape as the
`PYTHON_TEST_PKG` constant it extends, and a kind it does not name still widens
rather than being silently dropped.

**→ Decision (2026-08-19, main thread, forced by a second measurement): the
fail-open answer stops being `./...` for an unclassified path. It becomes the
packages that could plausibly READ that path.**

After the classification table landed, the live tree still answered `./...`,
now blamed on `.claude/plan/ze-plan-config-completion`, with twelve other
unclassified paths behind it. Drop those thirteen and the SAME tree answers
**278 packages of 646** from 37 seeds. So the table is not what is failing.
Any tree a session dirties carries some path no table names, and one such path
widens everything, for ever. A rule that can never fire in practice is not a
safety property, it is a switched-off feature.

The replacement, in order:

| Path | Answer | Why |
|------|--------|-----|
| `go.mod`, `go.sum`, `vendor/` | widen to `./...` | a dependency moved, so every package that compiles against it is reachable. Unchanged |
| an unclassified path inside a directory that IS a Go package | that package | a fixture is read by the tests sitting beside it |
| any other unclassified path | the tooling packages, and LOG the path by name | see the argument below |
| `examples/plugin/go` | nothing | a separate module (`module example/acme-monitor`), so `go list ./...` cannot report it, and its `.go` files must not fall through to the `.go` branch and seed a directory no package owns. This rule must be tested BEFORE that branch |

**→ Correction (2026-08-19, main thread).** An earlier revision of the row above
said this path seeds `./internal/test/cli` "because `cmd_plugin_external.go` is
what builds it". That is FALSE and it was mine: `cmd_plugin_external.go`
(`internal/test/cli`) names the example only in a doc comment, and so does
`sdk.go` (`pkg/plugin/sdk`). Nothing in `internal/le/` native action tables, `internal/le/`, `.github/workflows/`
or any test compiles the module. Seeding `./internal/test/cli` would therefore
run tests that cannot detect a break in the example, which is a mapping that
looks like coverage and is not. The row now seeds nothing, for the same reason a
`.ci` body does.

The finding underneath it is recorded separately: a tracked Go module that no
gate compiles can rot silently, and this one has no gate at all.

**Why narrowing the fail-open is safe here, and it rests on one fact: the
package answer drives only `./le changed scope` and `ze-unit-test-changed`, and
both are Go-only stages.** A non-Go file cannot change what a Go package
compiles to. It can only change what a Go TEST does, and then only if that test
reads it. Every stage that judges non-Go content -- the doc gates, `./le rfc check`,
the functional suites -- is a SEPARATE stage that still runs unconditionally and
is not driven by this list. Narrowing this answer therefore skips no judgement
about the file itself.

The tooling packages are where the readers actually are, measured rather than
assumed: `test/health/latest.json`, one of the thirteen, is read by
`testing_health.py`, `testing_health_test.py`, `site_health_render_test.py` and
`verify_run_test.go` -- which is `./scripts/dev` and `./scripts/status`, and
nothing else.

**→ Constraint: every unclassified path MUST be named on stderr**, because the
residual risk is a non-Go file read by a Go test that sits neither beside it nor
in a tooling package. Naming the path is what makes that gap visible and gives
the next reader the evidence to add a rule. `ai/rules/repo-maintenance.md`
refuses a silent cap, and this is one.

**→ Constraint: the selector MUST log what depth 2 dropped.** That is the
measurement which would let a later spec take depth 1 safely, and
`ai/rules/repo-maintenance.md` refuses a silent cap.

## Data Flow (MANDATORY)

### Entry Point
- A verify run starts. The selector runs once, before the first stage.

### Transformation Path
1. Collect changed paths: unstaged, staged, untracked, and committed since the last green baseline.
2. Classify each path: Go package, Python test input, RFC corpus, or unclassified.
3. Map each Go package to its feature tag through `feature-gates.txt`, or to always-on.
4. Build the reverse import graph WITH build tags, so a gated importer is seen.
5. Emit two answers: the package set to retest, and the feature-tag set the change can reach.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Selector ↔ verify runner | a file under `tmp/`, written once per run | No |
| Selector ↔ make recipes | one `./pkg` per line on stdout, as today | No |
| Selector ↔ `feature-gates.txt` | read at run time, never copied | No |

### Integration Points
- `_./le changed scope` and `_ze-unit-test-changed-impl` (`internal/le/` native action tables) - today's consumers of `changed-pkgs.sh`.
- Sub-spec 3's consumers read the feature-tag answer.

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
| A-1 | `feature-gates.txt` covers every compile-out boundary | 142 rows, and `staticcheck_feature_matrix.go` derives its whole matrix from it | A feature exists that the manifest does not name, and the selector misses it | Compare the manifest's tags against the `//go:build ze_*` constraints in the tree | **CONFIRMED 2026-08-19** for feature gates, with the boundary named: the tree's `//go:build` lines also use ten `ze_*` tags the manifest does not hold, and none of them is a feature. Seven select a program flavor (`ze_test`, `ze_chaos`, `ze_perf`, `ze_analyze`, `ze_setup`, `ze_distro`, `ze_appliance`), two belong to the installer (`ze_installer`, `ze_installer_fault`), and `ze_core` is the base. `loadPackageGraph` (`internal/le/changed/selector.go`) uses `ze_core` plus the 36 manifest tags, which is the same set as `GO_TEST_TAGS` (the retired `Makefile` (current producers: `internal/le/` native action tables)), so a file the unit suite never compiles is a file the selector never needs to see |
| A-2 | A package not under any manifest prefix is always-on, so it reaches every tag | `plugin_imports.go` treats an unmatched path that way | An always-on classification hides a gated package | The selector's self-test asserts the always-on set against the build constraints | **CONFIRMED 2026-08-19 in its consequential direction, and false as a literal statement.** Eight packages sit under no manifest prefix and still hold a feature-gated file: `cmd/ze`, `cmd/ze/hub`, `internal/component/plugin/all`, `internal/component/aaa/all`, `internal/component/config/yang/cli`, `internal/component/config/infra`, `internal/plugins/diag/cmd`, `internal/plugins/static`. Six of them are composition roots. `reachedTags` (`internal/le/changed/selector.go`) calls every one of them always-on, which answers with EVERY tag, so the error is over-selection and never a hidden gate |
| A-3 | A tag-aware graph can be built without compiling every tag combination | `go list -tags` per tag set is cheaper than a build | Building the graph costs more than the scoping saves | Measure the selector's own runtime; it must stay under 30s | **CONFIRMED 2026-08-19**, with a correction: ONE all-tags run costs 2.60s (against 2.92s untagged today), and a per-tag loop costs 94.6s. The assumption holds only for the single-run shape, so the design fixes that shape rather than leaving it to the implementer |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Narrowing the transitive closure drops an importer whose test would have caught the change | CI red on a change the local gate passed | The design phase must justify the narrowing with measurement, and the selector logs every package it dropped |
| R-2 | The selector becomes a second source of truth beside `feature-gates.txt` | The two disagree after a manifest edit | The selector reads the manifest at run time and stores no copy |
| R-3 | An unclassified path selects nothing | A change to a new file kind runs no stage | Fail OPEN: an unclassified path selects everything, and the selector says so on stderr |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A change is under-tested and a defect lands. Nothing at runtime |
| How is it reverted? | Single commit revert. Consumers fall back to today's `changed-pkgs.sh` behavior |
| Who else touches this path? | `./le changed scope` and `ze-unit-test-changed` consume it today; sub-spec 3 adds two more consumers |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le changed scope` | → | the selector entry point | `TestSelectorEmitsPackagesAndTags` |
| an unclassified changed path | → | the named-and-narrowed branch | `TestSelectorNamesAKindNoRuleNames` |
| `go.mod`, `go.sum` or a `vendor/` path | → | the fail-open branch | `TestSelectorWidensWhenTheModuleGraphMoves` |
| a change under `internal/component/ssh` | → | the tag-aware reverse graph | `TestSelectorSeesGatedImporters` |
| `./le changed packages` | → | the selector's package list, or the answer this run published | `TestThePackagesVerbAnswersThePrecomputedFile`, `TestAScopeFileIsReadWhenNoArgumentAsksADifferentQuestion` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The changed set is `internal/component/ssh/ssh.go` | The package list includes `cmd/ze/hub`, which today's selector omits |
| AC-2 | The changed set is one file under `internal/core/env` | The package list is materially smaller than the 454 transitive importers, and the design records why the dropped ones are safe |
| AC-3 | The changed set is `internal/component/ssh/ssh.go` | The feature-tag answer names `ze_ssh` and does not name `ze_bgp` |
| AC-4 | A changed path matches no known kind | The selector names that path on stderr and answers with the packages that could read it, exit 0. Selecting EVERYTHING is what a module-graph move earns (`go.mod`, `go.sum`, `vendor/`). The third Decision in Current Behavior replaced the `./...` answer for an unclassified path, and this row follows it |
| AC-5 | `feature-gates.txt` gains a row | The selector's answer changes with it, with no second file to edit |
| AC-6 | The selector runs on the current tree | It completes in under 30 seconds |
| AC-7 | An `rfc/` corpus path changes | The package list includes the Go package that reads that corpus. The row said `./scripts/dev` when it was written; the le-personality port retired `scripts/` and the Python tooling with it, so the live answer is `./internal/le/rfc` (`rfcCorpusPrefix`, `internal/le/changed/selector.go`) |
| AC-8 | The selector runs on a working tree carrying the kinds a real session dirties: `.sh`, `.mk`, `internal/le/` native action tables, `.md` under `plan/`, `ai/` and `docs/`, `.ci` under `test/`, `.yml` under `.github/` | The package answer is materially smaller than `./...`. Each of those kinds maps to the tooling packages that READ it, not to the whole tree |
| AC-9 | A path of a kind the selector has no rule for | It is NAMED on stderr and seeds the packages that could read it: the package it sits in when that directory holds Go source, the tooling packages otherwise. It does not widen to `./...`. This row's original wording predates the third Decision in Current Behavior and said the opposite; the Decision governs |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSelectorEmitsPackagesAndTags` | `internal/le/changed/selector_test.go` | The two answers are produced and separable | PASS |
| `TestSelectorWidensWhenTheModuleGraphMoves` | `internal/le/changed/selector_test.go` | AC-4: `go.mod`, `go.sum` and `vendor/` select everything and name the path | PASS |
| `TestSelectorNamesAKindNoRuleNames` | `internal/le/changed/selector_test.go` | AC-9: six near-miss paths are named on stderr and seed their readers, not `./...` | PASS |
| `TestSelectorSeedsThePackageAnUnclassifiedPathSitsIn` | `internal/le/changed/selector_test.go` | AC-9: a fixture beside a package seeds that package and its two levels of importers | PASS |
| `TestSelectorMapsTheExternalPluginExample` | `internal/le/changed/selector_test.go` | `examples/plugin/go` is a second module nothing here compiles or reads, so it seeds nothing, matched before the `.go` branch | PASS |
| `TestSelectorMapsToolingInputKinds` | `internal/le/changed/selector_test.go` | AC-8: eight dirtied kinds each seed the packages that read them | PASS |
| `TestSelectorMapsTheFunctionalCorpusToItsWalkers` | `internal/le/changed/selector_test.go` | AC-8: a `.ci`, `.et` or `.wb` seeds the Go test packages that WALK the committed corpus, never an empty set | PASS |
| `TestSelectorMapsGoTreesTheUnitBuildNeverCompiles` | `internal/le/changed/selector_test.go` | `cmd/ze-installer`, the module root and `gokrazy/modcache` stop widening the whole run for no gain | PASS |
| `TestSelectorScopesARealisticDirtyTree` | `internal/le/changed/selector_test.go` | AC-8 end to end: thirteen dirtied paths, four of them unclassified, answer under 20 packages | PASS |
| `TestSelectorFailsOpenWhenTheSeedIsNoPackage` | `internal/le/changed/selector_test.go` | Security review: a seed the toolchain does not report as a package widens the answer and names the directory | PASS |
| `TestSelectorSeesGatedImporters` | `internal/le/changed/selector_test.go` | AC-1: a `//go:build ze_ssh` importer is visible | PASS |
| `TestSelectorSeesGatedImportersInFixture` | `internal/le/changed/selector_test.go` | AC-1 on a fixture whose only edge to the importer is the gated file | PASS |
| `TestSelectorTagAnswerNamesTheReachedFeature` | `internal/le/changed/selector_test.go` | AC-3: `ze_ssh` is named, `ze_bgp` is not | PASS |
| `TestSelectorMapsNativeRFCAndCorpusPaths` | `internal/le/changed/selector_test.go` | AC-7: an `rfc/` corpus path selects `./internal/le/rfc` | PASS |
| `TestSelectorBoundsCoreFanOut` | `internal/le/changed/selector_test.go` | AC-2: a core change stays well under the closure and says what it dropped | PASS |
| `TestSelectorRunsUnderBudget` | `internal/le/changed/selector_test.go` | AC-6: 2.43s measured against the 30s budget | PASS |
| `TestSelectorReadsManifestAtRunTime` | `internal/le/changed/selector_test.go` | AC-5: no copy of `feature-gates.txt` | PASS |
| `TestVerifyRunNamesTheFeatureScopeToEveryStage` | `internal/le/verify/engine/scope_test.go` | The run selects once and names both answers to every stage it starts | PASS |
| `TestVerifyRunPublishesTheChangeSetPerRun` | `internal/le/verify/engine/scope_test.go` | Two runs of one checkout publish at different paths | PASS |
| `TestVerifyRunPublishesTheScopedAnswerAGatedChangeProduces` | `internal/le/verify/engine/scope_test.go` | The published answer is the SCOPED one a gated edit earns, not the wide one. It goes red when `publishChangeScope` is not called | PASS |
| `TestVerifyRunRestoresTheChangeScopeItNamed` | `internal/le/verify/engine/scope_test.go` | A run does not leave its answer behind to scope whatever runs next in the process | PASS |
| `TestVerifyRunWidensWhenTheChangeSetCannotBeSelected` | `internal/le/verify/engine/scope_test.go` | A refused selection names neither answer, and unset is the widest reading of both | PASS |
| `TestThePackagesVerbAnswersThePrecomputedFile`, `TestAScopeFileIsReadWhenNoArgumentAsksADifferentQuestion`, `TestAScopeFileThatCannotBeReadWidensToEveryPackage`, `TestAMissingScopeFileWidensToEveryPackage`, `TestAnArgumentBypassesThePrecomputedAnswer` | `internal/le/changed/actions_test.go`, `internal/le/changed/scope_test.go` | The consumer's answer comes from the run's published file, and every route that cannot read it widens | PASS |
| `TestSelectorWidensWithNoTrustedGreenBaseline` | `internal/le/changed/selector_test.go` | No green commit widens to `./...`, on each of the three conditions that produce one | PASS |
| `TestSelectorReadsAnAbsoluteStatusFileOverride` | `internal/le/changed/selector_test.go` | `ZE_VERIFY_STATUS_FILE` naming an absolute path is read at that path | PASS |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| changed path count | 0-N | N | N/A | N/A |
| reverse-dependency depth | 1-N | 2 (the chosen bound) | 0 (selects only the changed package) | N/A |

| Boundary | Test | Result |
|----------|------|--------|
| depth 0 is refused | `TestSelectorRefusesDepthZero` | PASS: exit 2, and the message names `--depth` |
| depth 1 is reachable only by the explicit flag | `TestSelectorDepthOneOnlyByOverride` | PASS: the default answers the depth-2 set, `--depth=1` answers one level less |
| depth 3 is the closure on the fixture | `TestSelectorDepthThreeMatchesClosure` | PASS: depth 3 and depth 9 answer the same four packages |

<!-- Depth is the one number with a real failure mode at the low end: depth 0
     retests only the edited package and misses every importer. The value is
     fixed at 2 by the Decision in Current Behavior, and these tests pin it:
     depth 0 must be refused, depth 1 must be reachable only by an explicit
     override, and depth 3 must be indistinguishable from the closure on the
     fixture (it is within 3% on the real corpus). -->

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `verify-scope-selector` | `test/runner/verify-scope-selector.ci` | A developer edits one SSH file and asks which packages and features apply; the selector names them and names nothing else | PASS |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | Scope is tooling. No wire-visible behavior changes | |

## Files to Modify
- `internal/le/` - becomes a thin caller of the selector, or is deleted in favor of it
- `internal/le/` native action tables - `_./le changed scope`, `_ze-unit-test-changed-impl`, and the new `ze-verify-scope-selector` target
- `internal/le/verify/engine/run.go` - run the selector once before the first stage

## Files to Create
- `internal/le/changed/selector.go` - the selector
- `internal/le/changed/selector_test.go` - its self-test
- `test/runner/verify-scope-selector.ci` - end-to-end proof

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Build tooling, no runtime config |
| YANG validation constraints | N-A | as above |
| YANG custom validators | N-A | as above |
| CLI commands/flags | N-A | A make target, not a `ze` subcommand |
| CLI grammar (keyword before value) | N-A | as above |
| Editor autocomplete | N-A | as above |
| Functional test for new RPC/API | N-A | No RPC or API added |
| Pipe completeness | N-A | No `ze` CLI output added |
| Env var registration | N-A | No `ze.*` leaf added |
| Doctor check for runtime dependencies | N-A | No new runtime path, socket, port, module, or binary |
| Prometheus counters/metrics | N-A | No daemon-observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Developer tooling |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Sub-spec 3 owns the tier question |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/`: the selector's contract and its fail-open rule |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep for anchors naming `changed-pkgs.sh` and `dep_audit.py` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/rules/commands.md` describes the changed-mode targets |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the selector target exists and returns a refusal
   - Tests: `TestSelectorEmitsPackagesAndTags`
   - Files: `internal/le/changed/selector.go`, `internal/le/` native action tables
   - Verify: `./le changed scope` runs and the test fails on empty output
2. **Phase: Classification** -- path to package, path to feature tag, unclassified to fail-open
   - Tests: `TestSelectorNamesAKindNoRuleNames`, `TestSelectorReadsManifestAtRunTime`
   - Files: `internal/le/changed/selector.go`
   - Verify: AC-3, AC-4, AC-5, AC-7 hold
3. **Phase: Tag-aware reverse graph** -- a gated importer becomes visible
   - Tests: `TestSelectorSeesGatedImporters`
   - Files: `internal/le/changed/selector.go`
   - Verify: AC-1 holds
4. **Phase: Bounding the expansion** -- replace the transitive closure with the design's chosen rule, and record the justification
   - Tests: the depth boundary tests
   - Files: `internal/le/changed/selector.go`
   - Verify: AC-2 and AC-6 hold
5. **Phase: Consumers** -- the two existing make recipes read the selector
   - Tests: `TestChangedPkgsConsumersReadTheSelector`, `verify-scope-selector.ci`
   - Files: `internal/le/` native action tables, `internal/le/`, `internal/le/verify/engine/run.go`
   - Verify: `./le changed scope` and `./le verify deps unit-race-changed` still lint and test the right set

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Both answers, packages and tags, are reachable from a make recipe |
| Correctness | The fail-open branch is reached by a test, not only written |
| Naming | One name for the change set, used by every consumer |
| Data flow | `feature-gates.txt` is read, never copied. No second manifest |
| Rule: `ai/rules/evidence.md` | The guard fails closed: an empty selection means everything, and the selector says which paths it could not classify |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The selector exists and runs | `./le changed scope` |
| SSH sees its gated importers | `TestSelectorSeesGatedImporters` |
| The fan-out is bounded | Compare the selector's output against the 454-importer measurement for `internal/core/env` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Paths come from `git` output and can hold any byte a filename can. A path must not be able to inject a package pattern into the make recipes that consume the list |

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

- Three components already parse the same facts in three languages, and none answers "what can this change affect": `plugin_imports.go` reads the manifest to generate imports, `dep_audit.py` builds a reverse graph and discards it, `staticcheck_feature_matrix.go` reads the manifest to build a matrix. The selector is the join they all imply.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One selector process, two answers | A package selector and a separate feature selector | Both answers come from the same graph walk. Two tools would walk it twice and could disagree |
| Read `feature-gates.txt` at run time | Generate a selector input file | A generated copy is a second source of truth and needs its own staleness gate |
| Fail open on an unclassified path | Fail closed and refuse the run | Refusing blocks work on a new file kind. Selecting everything costs time and loses nothing |
| Depth 2 | Depth 1; depth 3; the full transitive closure | Depth 3 is within 3% of the closure, so the choice is 1 against 2. Depth 1 costs a measured 17 coverage points on `internal/core/family` and no cross-package coverage data exists to tell a weak leaf from a strong one. Depth reaches only the unit stages, and 2346s of the 4418s run sits in two stages sub-spec 3 scopes by feature tag instead, so the last 15% of the closure is cheap to keep |
| One all-tags `go list` | 37 per-tag `go list` runs | 2.60s against 94.6s. The per-tag loop alone is 315% of AC-6's whole 30s budget |
| No test-ownership map | A map from package to the tests exercising it | Nothing in the repo holds that relation today, so it is a new artifact needing its own staleness gate (R-2). Its nearest precedent is 11 hand-maintained prefixes in `changed-groups.sh` |

## Known Limitations
- The selector answers about first-party packages. A change to a vendored dependency selects everything, which is today's behavior.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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
- `Scope.resolveSelector` and its helpers (`internal/le/changed/selector.go`): one selector answering the packages to retest and the feature tags the change reaches, from one tag-aware `go list` bounded at depth 2.
- `nonGoPathRules`, `packageDirsFor`, `unclassifiedDirs`, `uncompiledTreeReaders` (same file): the classification table and the narrowed fail-open, so a dirtied tree answers its readers instead of `./...`.
- `ScopeFileKey`, `Scope.Resolve`, `Scope.fromFile`, `widen` (`internal/le/changed/scope.go`): the consumer route, which reads the answer a run published and widens on every route that cannot.
- `publishChangeScope`, `selectChangeSet`, `nameChangeScope` (`internal/le/verify/engine/scope.go`), called from `runMode` (`run.go`): the run selects once before its first stage, writes both answers into its own log directory, and names them in `ZE_VERIFY_SCOPE_PACKAGES` and `ZE_VERIFY_SCOPE_TAGS`.
- `test/runner/verify-scope-selector.ci` with `verifyScopeSelectorDriver` (`internal/test/fixture/misc_fixture_runner_scope.go`): the end-to-end proof over the real action.

### Bugs Found/Fixed
- **The run published nothing.** The le-personality port (`eae282592`) kept the selector and dropped `selectChangeSet` and the scope environment `execStage` exported, so no stage could read either answer: every verify judged all 38 staticcheck matrix rows, and `docs/contributing/running-commands.md` stated a producer the tree did not hold. Found by grepping for `ScopeFileKey`'s writers and confirmed against `runMode`, which set `ZE_VERIFY_MODE` alone. Fixed by `publishChangeScope`; covered by `TestVerifyRunPublishesTheScopedAnswerAGatedChangeProduces`, which goes red when the call is removed (observed). The 2026-09-02 row in `plan/journal/refactor-removes-feature.md` records it and now records the fix.
- **The green-baseline guard had lost its tests.** `greenBaseline` (`selector.go`) is what stops a clean unproven tree from selecting nothing, and the tests the spec named for it went with the shell they were written against. Restored as `TestSelectorWidensWithNoTrustedGreenBaseline` and `TestSelectorReadsAnAbsoluteStatusFileOverride`.
- **A `.ci` scenario claimed a naming it never read.** `verifyScopeSelectorDriver` printed `unclassified-path-is-named` while discarding the selector's stderr, so the whole guarantee for an unnamed kind could disappear with the scenario still green. It now asserts `no rule names <path>` and, for each module-graph move, `<path> changed, so a dependency moved`.
- **The tag key was declared away from its producer.** `ScopeTagsKey` lived in `staticcheckfeaturematrix`, its consumer, so the run could not name it without importing that package, and `staticcheckfeaturematrix_test.go` already imports the engine: an import cycle. The key now sits in `internal/le/changed/scope.go` beside `ScopeFileKey`, because the two are one contract published by one run from one walk of one graph, and the package that produces both answers names both files. The engine imports `changed` alone.

### Documentation Updates
- `docs/architecture/testing/verify-freshness-scope.md`: the publication paragraph now names its producer and the two file names, and states what an unpublished answer means. Anchor: `internal/le/verify/engine/scope.go -- publishChangeScope`.
- `docs/contributing/running-commands.md` needed no edit: its sentence "only a verify run publishes the feature-tag answer that `ZE_VERIFY_SCOPE_TAGS` (`ScopeTagsKey`) names" was the claim the missing producer falsified, and it is true again.
- `docs/functional-tests.md` already carries the scoped matrix and the six-way cut.

### Deviations from Plan
- **AC-7 named `./scripts/dev`, a package the tree no longer holds.** `scripts/` and the Python tooling were retired by the le-personality port. The row now reads the live answer, `./internal/le/rfc` for an `rfc/` corpus path, which is the same guarantee against the tree as it stands.
- **The TDD table named nine `TestChangedPkgs*` tests and four `TestVerifyRun*` tests that no longer existed under those names.** Corrected in place to the tests that exist, with the engine tests written by this closure.
- The retired `changed-pkgs.sh` is not "a thin caller or deleted": the whole shell layer is gone, and `internal/le/changed` is the native replacement.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The spec's TDD table was read as a record of tests that exist | Thirteen rows named tests retired with the shell they were written against, and four named a producer the port had removed | Grepping each name before pasting it as closure evidence | Every row now names a test this closure ran; the missing producer was rebuilt rather than re-recorded |
| approach | The publication looked like sub-spec 3's work, because sub-spec 3 is the consumers spec | This spec's own Files to Modify names `internal/le/verify/engine/run.go`, "run the selector once before the first stage" | Reading both specs' Files to Modify before deciding where the fix belonged | The producer landed here; sub-spec 3 verifies its consumer |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| One selector answering both questions from one walk | Done | `resolveSelector`, `internal/le/changed/selector.go` | `changeSet` carries `packages` and `tags` from the same graph |
| Tag-aware reverse dependencies, so a gated importer is visible | Done | `loadPackageGraph` | one `go list -tags ze_core,<36 manifest tags>` |
| The expansion stops being the transitive closure | Done | `reverseWalk`, `expandPackages` | `defaultDepth = 2`, and the drop is stated on stderr |
| Every later consumer reads that one answer | Done | `publishChangeScope` (`internal/le/verify/engine/scope.go`), `Scope.fromFile` (`scope.go`), `readChangeScope` (`internal/le/staticcheckfeaturematrix`) | one selection per run, named to every stage |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestSelectorSeesGatedImporters`, `TestSelectorSeesGatedImportersInFixture` | `./cmd/ze/hub` is in the answer for one SSH file |
| AC-2 | Done | `TestSelectorBoundsCoreFanOut` | bounded well under the closure, and `writeDropLog` records what was dropped |
| AC-3 | Done | `TestSelectorTagAnswerNamesTheReachedFeature` | `ze_ssh` named, `ze_bgp` not |
| AC-4 | Done | `TestSelectorWidensWhenTheModuleGraphMoves`, `TestSelectorNamesAKindNoRuleNames` | the module graph widens; an unclassified path is named and narrowed |
| AC-5 | Done | `TestSelectorReadsManifestAtRunTime` | `featureManifestPath` is read per run, never copied |
| AC-6 | Done | `TestSelectorRunsUnderBudget` | measured against the 30s budget |
| AC-7 | Done (restated) | `TestSelectorMapsNativeRFCAndCorpusPaths` | `./scripts/dev` no longer exists; the corpus seeds `./internal/le/rfc` |
| AC-8 | Done | `TestSelectorMapsToolingInputKinds`, `TestSelectorMapsTheFunctionalCorpusToItsWalkers`, `TestSelectorScopesARealisticDirtyTree` | thirteen dirtied paths answer under twenty packages |
| AC-9 | Done | `TestSelectorNamesAKindNoRuleNames`, `TestSelectorSeedsThePackageAnUnclassifiedPathSitsIn` | named on stderr, seeded to its readers |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Every selector case in the TDD table | PASS | `internal/le/changed/selector_test.go` | `go test ./internal/le/changed/...` ok 93.896s |
| The five publication cases | PASS | `internal/le/verify/engine/scope_test.go` | written by this closure; three go red without `publishChangeScope` |
| `TestTheAnswerARunPublishesScopesTheMatrixAndIsDealtWhole` | PASS | `internal/le/verify/engine/matrix_cut_test.go` | written by this closure: the join neither package's own tests can see |
| The two green-baseline cases | PASS | `internal/le/changed/selector_test.go` | written by this closure |
| `verify-scope-selector` | Exists, driver strengthened | `test/runner/verify-scope-selector.ci` | the unclassified and module-move scenarios now read stderr |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/changed/selector.go` | Done | 1155 lines |
| `internal/le/changed/selector_test.go` | Done | plus the two restored guard tests |
| `test/runner/verify-scope-selector.ci` | Done | |
| `internal/le/verify/engine/run.go` | Done | `runMode` calls `publishChangeScope` before the first stage |
| the retired `changed-pkgs.sh` | Changed | the shell layer is gone; `internal/le/changed` is the native replacement |

### Audit Summary
- **Total items:** 26
- **Done:** 25
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (the retired `changed-pkgs.sh`, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| One selector answers which packages must be retested | functional | `test/runner/verify-scope-selector.ci` pins `./cmd/ze`, `./cmd/ze/hub`, `./internal/component/ssh` exactly for one SSH file, and fails if the answer names anything more |
| It answers which build-tag features the change can reach | functional | the same scenario pins `ze_ssh` alone as the tag answer |
| It over-selects rather than under-selects | functional and unit | the same scenario drives `go.mod`, `go.sum` and a `vendor/` path to `./...` and asserts the stderr names the move; `TestSelectorWidensWithNoTrustedGreenBaseline` covers the three unproven-tree conditions |
| Every later consumer reads that one answer | unit, discriminated | `TestVerifyRunNamesTheFeatureScopeToEveryStage` proves every stage of a run reads the same two paths; `TestVerifyRunPublishesTheScopedAnswerAGatedChangeProduces` proves the published answer is the scoped one, and was observed RED with `publishChangeScope` uncalled (three failing cases) and GREEN with it restored |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Suite selection from the change set | No static signal attributes a `.ci` file to a Go package | `plan/spec-verify-scope-5-suite-coverage-map.md` |
| A gate that compiles `examples/plugin/go` | A tracked Go module no gate builds can rot silently; it is a separate module and outside the selector's subject | none yet; recorded here and in the comment on `packageDirsFor` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/verify-scope-2-change-set-selector-zeclose-vs2.md` |
| `review check` | `review_gate: OK (7 code files, clean, hashes match ...)`. It also NOTEs the model as `unknown`: this closure ran as a subagent with no transcript of its own, so `CurrentModel` could not read one |
| Rounds | 3. Round 1 found findings 1 to 4, round 2 found 5 and 6 in the fix for 1, round 3 was clean |
| Reviewer lenses used | wiring and producer reachability; guard-fails-closed; test vacuity and discrimination; Go style (`docs/contributing/ze-go-style.md`); documentation against producer |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | Nothing writes `ZE_VERIFY_SCOPE_PACKAGES` or `ZE_VERIFY_SCOPE_TAGS`, so the run publishes no answer and no consumer is scoped | `runMode`, `internal/le/verify/engine/run.go` | `publishChangeScope` (`internal/le/verify/engine/scope.go`), called before the first stage |
| 2 | ISSUE | The `.ci` driver prints `unclassified-path-is-named` while discarding stderr, so the naming guarantee is unasserted | `verifyScopeSelectorDriver`, `internal/test/fixture/misc_fixture_runner_scope.go` | the driver reads stderr and asserts `no rule names <path>` and the dependency-move line |
| 3 | ISSUE | `greenBaseline`'s three widening conditions and its absolute-override read have no test | `greenBaseline`, `internal/le/changed/selector.go` | `TestSelectorWidensWithNoTrustedGreenBaseline`, `TestSelectorReadsAnAbsoluteStatusFileOverride` |
| 4 | ISSUE | Thirteen TDD rows and AC-7 name tests and a package the tree no longer holds, so the spec's own record is false | this spec | every row rewritten to the test that exists; AC-7 restated against `./internal/le/rfc` |
| 5 | ISSUE | `ScopeTagsKey` is declared in its CONSUMER, so the producer cannot name it without an import cycle | `staticcheckfeaturematrix.ScopeTagsKey` | the key moved beside its twin in `internal/le/changed/scope.go`; the matrix now reads it from the package that produces the answer |
| 6 | NOTE | Nothing asserts that a SCOPED matrix is still dealt whole across the six pieces: the cut is proven over the unscoped 38 rows alone | `Matrix.Part`, `internal/le/staticcheckfeaturematrix` | `TestTheAnswerARunPublishesScopesTheMatrixAndIsDealtWhole` (`internal/le/verify/engine/matrix_cut_test.go`) drives the run's own published answer into `DeriveScoped` and deals the three surviving rows across the population's pieces |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/changed/selector.go` | Yes | `-rw-rw-r-- 44532 Sep 5 12:35` |
| `internal/le/changed/selector_test.go` | Yes | `-rw-rw-r-- 38844 Sep 5 14:21` |
| `test/runner/verify-scope-selector.ci` | Yes | `-rw-rw-r-- 1790 Aug 30 22:52` |
| `internal/le/verify/engine/scope.go` | Yes | `-rw-rw-r-- 5251 Sep 5 14:12` |
| `internal/le/verify/engine/scope_test.go` | Yes | `-rw-rw-r-- 8109 Sep 5 14:16` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | a gated importer is selected | `test/runner/verify-scope-selector.ci` asserts the answer is `./cmd/ze`, `./cmd/ze/hub`, `./internal/component/ssh` and nothing else |
| AC-3 | the tag answer is `ze_ssh` alone | the same scenario, `# tags` section |
| AC-6 | the selector answers under 30s | `TestSelectorRunsUnderBudget` PASS inside `ok github.com/ze-software/ze/internal/le/changed 93.896s` |
| AC-4, AC-9 | an unclassified path is named and narrowed; a module move widens | `TestSelectorNamesAKindNoRuleNames`, `TestSelectorWidensWhenTheModuleGraphMoves` PASS |
| every AC | the whole package is green | `go test ./internal/le/changed/... ./internal/le/verify/engine/ ./internal/le/staticcheckfeaturematrix/` exit 0 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le changed scope` | `test/runner/verify-scope-selector.ci` | Yes, read the driver: it runs `bin/le changed scope print <mode> paths-from <file>` for five path kinds and asserts both answers and the stderr |
| an unclassified changed path | the same | Yes, `demos/terminal/rpki/demo.cast`, stderr asserted |
| `go.mod`, `go.sum`, `vendor/` | the same | Yes, all three, `./...` and the reason asserted |
| a change under `internal/component/ssh` | the same | Yes, the exact three-package answer |
| `./le changed packages` | unit | `TestThePackagesVerbAnswersThePrecomputedFile`, `TestAScopeFileIsReadWhenNoArgumentAsksADifferentQuestion` |
| a verify run naming the answer to every stage | unit | `TestVerifyRunNamesTheFeatureScopeToEveryStage`, discriminated by the observed red |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed, with the boundary named | `loadPackageGraph` uses `ze_core` plus the 36 manifest tags; the ten non-feature `ze_*` tags select a program flavor and the unit suite never compiles those files |
| A-2 | confirmed in its consequential direction, false as a literal statement | `reachedTags` calls an unmatched package always-on, which answers with EVERY tag: the error is over-selection, never a hidden gate |
| A-3 | confirmed for the single-run shape | one all-tags `go list` measured at 2.60s against 94.6s for a per-tag loop; `TestSelectorRunsUnderBudget` holds it under 30s |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| #10 test infrastructure | `docs/functional-tests.md` carries the scoped matrix, the six-way cut and the unscoped suite selection | Yes |
| #12 internal architecture | `docs/architecture/testing/verify-freshness-scope.md` carries the selector contract, the per-kind table and now the publication producer | Yes, edited this closure |
| #16 source anchors on changed files | the per-kind table anchors `internal/le/changed/selector.go`; `internal/le/verify/engine/scope.go` gained one | Yes |
| #17 examples for this area | `docs/contributing/running-commands.md` describes the scoped targets and the `ZE_VERIFY_SCOPE_TAGS` producer, which the fix restored | Yes |
| every other row | No | The scope is build tooling: no YANG, no CLI verb, no RPC, no plugin, no wire format, no RFC tier |

## Core Insight

A port that keeps a CONSUMER and drops its PRODUCER leaves every gate correct and only slower, so nothing can go red and nobody can be told. The staticcheck matrix kept reading `ZE_VERIFY_SCOPE_TAGS`, kept widening on the empty answer it always got, and kept judging all 38 rows. The only way to see it was to ask who WRITES the variable. A registered env key with no writer is the shape to grep for.
