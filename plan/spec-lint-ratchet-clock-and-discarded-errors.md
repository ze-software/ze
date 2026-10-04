# Spec: lint-ratchet-clock-and-discarded-errors

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | - (the capture record in Phase 5 cross-references `plan/spec-journal-class-lifecycle.md`, see A-7) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Two journal classes keep collecting rows because nothing mechanical refuses a new
occurrence:

| Class | Journal | What it is | Why no tool refuses it today |
|-------|---------|------------|------------------------------|
| clock-bypass | `plan/journal/clock-bypass.md` | Product code reads the wall clock or waits on real time through package `time` (`Now`, `Since`, `Until`, `Sleep`, `After`, `AfterFunc`, `NewTimer`, `NewTicker`, `Tick`) instead of the injected `internal/core/clock` Clock, so simulation and tests cannot control it | Only `TestNoDirectTimeCalls` (`internal/core/clock/audit_test.go`) checks, and only over two directories (`bgp/reactor`, `bgp/fsm`) |
| discarded-error-becomes-destructive | `plan/journal/discarded-error-becomes-destructive.md` | An error is assigned to the blank identifier (`_ = f()`, `v, _ := f()`, `_, _ = f()`), safe on the day it was written, destructive once the operation gains the power to delete | `.golangci.yml` sets errcheck `check-blank: false`; `docs/contributing/ze-go-style.md` says "no linter refuses that form, so the reader is the check" |

Goal (owner-approved scope, 2026-10-05): a mechanical capture that refuses a NEW
occurrence of either class in shipped product code, while every existing
occurrence stays as the baseline. There is no repo-wide sweep: existing
occurrences are fixed only when a change touches their lines.

Out of scope: fixing the existing occurrences; dev tooling (anything the shipped
binaries do not link); designing how a journal class records its capture (owned
by `plan/spec-journal-class-lifecycle.md`).

The owner answered the three open questions on 2026-10-05 (Owner Decisions,
under Key Design Decisions): the clock class covers every `time` function the
Clock interface replaces, a discard is allowed only for a failure nothing can act
on and only through an explicit, reasoned list in `.golangci.yml`, and moved code
counts as new.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/ze-go-style-background.md` "How the gates choose what they judge" - the line-scoped gate this spec reuses
  → Decision: a style gate over a tree holding thousands of instances judges only CHANGED LINES since the merge base of HEAD and the upstream (or `origin/main`), read from refs only, so `./le verify worktree`'s detached worktree answers the same range
  → Constraint: no base resolves → exit 2 naming why; never "nothing changed"
- [ ] `docs/contributing/ze-go-style.md` "Every error is handled" and "The rules a tool enforces" - the page whose statements this spec makes false
  → Constraint: the sanctioned discard form is `//nolint:errcheck // <reason>` on the discarding line; the page's sentence "no linter refuses that form" and the table row "A blank discard `f, _ := open()` passes" must be rewritten in the same change
  → Constraint: the page states no clock rule at all; a "Time comes from the injected clock" statement must be added, since a gate that refuses something no page states is undocumented behavior. It names the refused `time` functions by pointing at the Clock interface and the three-row replacement table (Data Flow step 5), never by a copied list
  → Constraint: the page names the explicit allowed-discard list in `.golangci.yml` as the one place a discard needs no per-line reason, and states the test an entry must pass (a failure nothing can act on)
- [ ] `docs/architecture/testing/test-health.md` "The floors" and the weakening commit gate - the two other ratchet shapes in the repo
  → Decision: a tree-wide count floor (`test/health/sensitivity-baseline.json`, `test/.ci-sleep-baseline`) suits a total that is cheap to recount; it does not say WHICH occurrence is new, and a tree-wide count "moved under whoever reads it" with three sessions editing (751 to 755 in an hour, 2026-08-10)
  → Decision: the RFC discrimination ratchet is change-scoped: "the standing corpus is grandfathered", new units owe the proof. Same shape as chosen here
- [ ] `feature-gates.txt` header and `internal/le/repo/featuretags/daemontags.go` - where the shipped build tags are declared
  → Constraint: `feature-gates.txt` is the single source of the gate tags; `DaemonBase` is `ze_core ze_distro`; the appliance base (`ze_core ze_appliance`) is an unexported constant pair in `featuretags.go` (`coreTag`, `applianceTag`). The shipped scope MUST derive from these, never from a copied tag list
- [ ] `ai/rules/principles.md` (CORE) - silent zero, single declaration
  → Constraint: a changed Go file the gate cannot parse, type-check, or place in a build is exit 2, never a pass
- [ ] `ai/rules/simplicity.md`, `ai/rules/no-layering.md` - reuse before invent
  → Decision: reuse `repochanged.LinesSinceUpstream` and the `arch compound-guard` command shape; no second change-set reader, no new baseline file format

**Key insights:**
- The repo already has the exact ratchet mechanism this needs: `./le arch compound-guard check` judges changed lines in the unpushed range and runs as a verify stage. Two sibling gates in the same shape cover both classes with no baseline file.
- golangci `new-from-rev` / `new-from-merge-base` are global `issues:` settings; enabling either would also hide every other linter's backlog, and a separate golangci pass would need a second config (a second declaration of the errcheck settings) and the 16-flavor matrix.
- The shipped closure, computed from the three target binaries under the shipped tag sets, contains `internal/chaos` (imported by `internal/component/bgp/config/loader.go` and `loader_create.go`) and no `internal/le` package. A directory-name boundary would have been wrong in both directions.
- `//nolint:errcheck // <reason>` already appears on 2,212 non-test lines; `nolintlint` has `require-explanation: true` and `allow-unused: true`, so the marker is legal on a line errcheck does not currently report.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `.golangci.yml` - errcheck `check-type-assertions: true`, `check-blank: false`, `exclude-functions` lists the `strings.Builder` and `textbuf.Buffer` writers plus `fmt.Fprint`/`fmt.Fprintf` into `*textbuf.Buffer`; forbidigo `analyze-types: true` bans only legacy `log.*`; `issues.max-issues-per-linter: 50`; no `new-from-*` setting
  → Constraint: flipping `check-blank: true` globally would report about 1,500 sites truncated at 50 per linter and turn every full lint red; not an option
  → Decision: the errcheck exclude list in `.golangci.yml` is the one declaration of "a discard that needs no reason"; the new gate reads it from that file
  → Decision (owner, 2026-10-05, Q-2): `disable-default-exclusions: true` is set, so errcheck's implicit built-in list stops applying and the explicit `exclude-functions` list is the whole allowed set for errcheck and the gate alike. Every qualifying built-in entry is written into the list, each with its reason as a trailing YAML line comment (the Allowed Discards table under Key Design Decisions)
- [ ] `vendor/github.com/kisielk/errcheck/errcheck/excludes.go` (errcheck v1.20.0) - `DefaultExcludedSymbols`, the 32-entry built-in list golangci applies while `disable-default-exclusions` is false
  → Decision: each entry was judged against the owner's test (a failure nothing can act on) using the standard library's own documentation under Go 1.27; all 32 qualify, the four `strings.Builder` writers already sit in the repo list, and 28 are added
  → Constraint: a test compares the explicit list with this vendored variable, so an errcheck upgrade that adds a default turns red until the new entry is judged and either listed with a reason or recorded as refused
- [ ] `internal/le/arch/compoundguard/{actions.go,compoundguard.go,report.go,register.go,selftest.go}` - the gate to copy in shape
  → Constraint: area registered via `leroot.Register(area, leroot.GroupGate, ...)` in `register.go`, blank-imported from `internal/le/register.go`; actions `check` and `selftest`; `CheckReport{Base, Files, Findings}` with `exitCode` 0/1 and runner exit 2 on a git or parse error; `judged(path)` skips `_test.go`, `vendor/`, `testdata/`, dot directories; generated files skipped via `ast.IsGenerated`
- [ ] `internal/le/repo/changed/lines.go` - `LinesSinceUpstream(root) (ChangedLines, LineBase, error)`; `ChangedLines.Touches(path, first, last)`; untracked files count as wholly changed; renames detected with `--find-renames`
  → Constraint: reuse unchanged; a pure file rename carries no changed line, a moved block inside a split does
- [ ] `internal/le/verify/engine/stages.go` (stage list near line 113) and `verifyengine_test.go` (expected stage names near line 32) - `stage("arch compound-guard", "check")` sits after `arch enumeration`
  → Constraint: a new stage must be added to both the list and the test's expected names
- [ ] `internal/le/hookruntime/postwrite.go` `postFormatGo` - edit-time golangci run with `--new-from-rev=HEAD` over the edited package
  → Decision: this is the only existing use of golangci's new-from mode, and it is advisory feedback, not a gate. Unchanged by this spec
- [ ] `internal/le/commit/prepare.go` `checkSourceGates` and `internal/le/commit/rfcchange.go` - the commit-time gate keyed on named paths against HEAD
  → Decision: considered as the host and rejected (Key Design Decisions): `ai/rules/pre-release.md` makes the push, not the commit, where gates are owed, and compound-guard already established the verify-stage home for this shape
- [ ] `internal/core/clock/clock.go` and `audit_test.go` - `Clock` interface (`Now`, `Sleep`, `After`, `AfterFunc`, `NewTimer`, `NewTicker`), `RealClock`; `TestNoDirectTimeCalls` substring-greps two directories for `time.Now()`, `time.After(`, `time.Sleep(`, `time.AfterFunc(`, `time.NewTimer(`
  → Constraint: `internal/core/clock` is the one package allowed to call the refused `time` functions (it implements `RealClock`)
  → Decision (owner, 2026-10-05, Q-1): the refused set is every `time` function the Clock interface replaces. It is derived from the interface's method set (Now, Sleep, After, AfterFunc, NewTimer, NewTicker, each refusing the `time` function of the same name), plus three `time` functions with no same-named method, each naming the method that replaces it: `Since` and `Until` (replaced by `Now`), `Tick` (replaced by `NewTicker`, whose `Ticker` carries `C()` and the `Stop()` that `time.Tick` never offered). `Tick` needs no Clock method of its own
  → Decision: the audit test stays; it is a stricter whole-file ban (existing lines too) in two directories, while the gate judges changed lines across the shipped closure. It is not the mechanism this spec replaces
- [ ] `internal/le/go/lint/matrix.go` - `basePasses` and `flavorMatrix`: named flavors (host, linux-integration, capability, distro, appliance, setup, personalities, installer, compile-out, arm64 ...) each with GOOS/GOARCH/tags
  → Decision: the discard gate type-checks a changed file under a flavor from this matrix that admits it; the matrix is the one declaration of build contexts
- [ ] `internal/le/repo/featuretags/featuretags.go`, `daemontags.go` - `DaemonTags(root)`, `DaemonBuildTags(root, base)`, `DaemonBase`, `LEBase`
- [ ] `cmd/ze-installer/main.go` - `//go:build linux && ze_installer`

**Behavior to preserve:**
- `./le arch compound-guard check` output, exit codes and stage position.
- `./le go lint run` findings: `check-blank` stays false, forbidigo and `issues:` unchanged; the errcheck block changes only by turning the implicit default list into explicit entries, so errcheck excludes exactly the same calls as before.
- `TestNoDirectTimeCalls` unchanged.
- `postFormatGo` unchanged.

**Behavior to change:**
- `./le verify` gains two stages that refuse a new clock bypass or a new blank error discard on a changed line of shipped code.
- `.golangci.yml` errcheck: `disable-default-exclusions: true`, and `exclude-functions` carries every allowed discard explicitly, each entry with its reason as a trailing line comment.
- The style page states both rules and names the gates.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le arch clock-bypass check`, `./le arch discarded-error check`, each run directly or as a stage of `./le verify current` / `./le verify worktree`.
- Input: the git refs and working tree of the checkout (no arguments).

### Transformation Path
1. `repochanged.LinesSinceUpstream(root)` answers the changed line spans and the base commit.
2. The shipped-scope helper answers the set of shipped source files: the module packages in the dependency closure of `./cmd/ze`, `./cmd/ze-serial-shell` (under each shipped daemon personality) and `./cmd/ze-installer` (under its installer tags), with GOOS `linux` and the host GOOS, from `go list -deps`; a package's file counts when it is a non-test Go file of that package for at least one of those builds.
3. Changed paths are intersected with the shipped file set; `_test.go`, generated files, `vendor/`, `testdata/` and dot directories are dropped as compound-guard drops them. `internal/core/clock` is dropped for the clock class only.
4. For each remaining file, the gate picks a lint-matrix flavor whose build context admits the file and loads its package with type information under that flavor.
5. Detector:
   - clock-bypass: every identifier use that resolves to a refused function of package `time` (call or function value, any import alias, including a dot import). The refused set is built once per run: each method of the `Clock` interface in `internal/core/clock`, read through `go/types`, refuses the `time` function of the same name; a three-row table in the gate adds `Since` and `Until` (replaced by `Now`) and `Tick` (replaced by `NewTicker`). A Clock method with no same-named `time` function, or a table row whose replacement is not a Clock method, is exit 2. Each finding names the Clock method that replaces the call.
   - discarded-error: every assignment, short variable declaration or `var` spec whose LHS slot is the blank identifier and whose corresponding RHS value has type `error` (single value or one slot of a tuple), minus calls matching an entry of the errcheck `exclude-functions` list of `.golangci.yml` (including the first-argument form, such as `fmt.Fprintf(os.Stderr)`), minus a statement whose line carries `//nolint:errcheck` with a reason. The list is read with its YAML comments: an entry with no trailing line comment, or `disable-default-exclusions` not true, is exit 2, because the list is then not the whole, reasoned allowed set.
6. An occurrence is due when `ChangedLines.Touches(path, first, last)` holds over the statement span (clock: the line of the identifier; discard: the assignment statement from its first to last line).
7. Report: base, judged file count, findings (`file:line (Fn): <class text>`), the fix, and the page; exit 1 on any finding.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le ↔ git | `repochanged.LinesSinceUpstream` (existing) | No |
| le ↔ go toolchain | `go list -deps -json` for the shipped closure; `golang.org/x/tools/go/packages` for type-checked loads (already in `go.mod` at v0.49.0) | No |
| le ↔ `.golangci.yml` | YAML node read (`gopkg.in/yaml.v3`, already a direct dependency) of `linters.settings.errcheck.exclude-functions` with each entry's line comment, and of `disable-default-exclusions` | No |
| le ↔ `internal/core/clock` | `go/types` method set of the `Clock` interface, loaded with the changed packages | No |

### Integration Points
- `internal/le/repo/changed.LinesSinceUpstream`, `ChangedLines.Touches` - reused as-is.
- `internal/le/go/lint` flavor matrix - exported lookup of the flavor that admits a file (new exported accessor over existing data).
- `internal/le/repo/featuretags` - exported declaration of the shipped personalities (new, replaces the unexported literal pairs its `targets` uses, so the gokrazy and quickstart writers and the shipped scope read one declaration).
- `internal/le/verify/engine/stages.go` - two new stages beside `arch compound-guard`.
- `internal/core/clock.Clock` - its method set is the declaration of which `time` functions the clock gate refuses; a method added there widens the gate with no edit to it.
- `.golangci.yml` errcheck block - the declaration of allowed discards for errcheck and the gate; `vendor/github.com/kisielk/errcheck/errcheck.DefaultExcludedSymbols` is compared with it by test.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | change set via `repochanged`, verdict via the le action, stage via the verify engine |
| No unintended coupling (components stay isolated) | Yes | all new code under `internal/le/`, which never ships (absent from the shipped closure) |
| No duplicated functionality (extends existing, does not recreate) | Yes | reuses the change-set reader, the lint flavor matrix, the featuretags manifest reader and the errcheck exclude list; no baseline file |
| Zero-copy preserved where applicable (refs, not copies) | N-A | tooling, not a wire path |
| Registration over hardcoding, outbound | Yes | each area registers through `leroot.Register` in its own `register.go` |
| Registration over hardcoding, inbound | Yes | the shipped set derives from `feature-gates.txt` + the featuretags personality declaration + `go list`; the allowed discards derive from `.golangci.yml`; the refused clock functions derive from the `Clock` method set, with a three-row table only for the `time` functions no method shares a name with; searched: `featuretags.targets`, `lint.flavorMatrix`, `verify/engine/stages.go` (the stage list is the verify engine's own ordered declaration, which compound-guard is also in) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The shipped binaries are `cmd/ze` (distro and appliance personalities), `cmd/ze-serial-shell`, and `cmd/ze-installer`; the `ze appliance` host driver (`ze_setup`) and anything under `ze_le` are host tools | `AGENTS.md` Programs table ("Host binaries run on the build machine and are never cross-compiled"); `featuretags.DaemonBase` comment on the host driver | scope misses or over-includes a binary | `go list -deps` closure printed by the selftest; owner confirmation of the binary list | unvalidated |
| A-2 | Shipped personalities run on Linux, and `go install` (docs/guide/quickstart.md) also builds the distro personality on the host OS, so both GOOS values count | quickstart go install command; gokrazy config | a darwin-only file is judged or skipped wrongly | TestShippedScopeIncludesHostAndLinuxFiles | unvalidated |
| A-3 | Every changed shipped file is admitted by at least one lint-matrix flavor | `./le go lint run` proves tracked-file coverage across the matrix | the gate cannot type-check a file and must exit 2 | TestAFileNoFlavorAdmitsIsExitTwo; running check over the tree | unvalidated |
| A-4 | The allowed discards are exactly the explicit `exclude-functions` entries of `.golangci.yml` and `//nolint:errcheck // reason`; errcheck's implicit built-in list applies to neither tool | Owner decision Q-2 (2026-10-05); go-standards: a discarded error needs `//nolint:errcheck // <why>` | a built-in default left implicit would be a second, unreasoned declaration | owner answer to Q-2 | validated (owner, 2026-10-05) |
| A-5 | Approximate scale: within the shipped closure (3,860 non-test files outside `internal/core/clock`, linux and darwin, distro and appliance personalities with every gate tag, plus the installer), 649 non-test lines in 316 files name a refused `time` function: `Now` 400, `Since` 47, `Until` 13, `Sleep` 22, `After` 23, `AfterFunc` 17, `NewTimer` 42, `NewTicker` 77, `Tick` 0. The original three alone are 468 lines in 250 files (the earlier 446 in 229 was measured on a narrower build set). No shipped file imports `time` under an alias. About 1,488 lines match a textual blank-assignment pattern (not type-filtered, so an upper bound for the error class) | textual grep excluding comment lines over the `go list -deps -f GoFiles` union, 2026-10-05 | none: the gate does not depend on the number | `./le arch ... check` output at implementation | unvalidated |
| A-6 | A changed line in another session's uncommitted work reaching `./le verify current` is acceptable, as it is for compound-guard; `./le verify worktree` judges a commit and so sees only committed work | `ze-go-style-background.md`; compound-guard precedent | a session sees reds it did not write | compound-guard history | unvalidated |
| A-7 | `plan/spec-journal-class-lifecycle.md` (status ready on 2026-10-05) defines how a journal class records its capture: a `Captured:` line of kind `check` naming the gate function and a proof test. Under that spec a class captured by a check goes DUE again after one row dated after the capture | that spec's capture grammar and owner decision of 2026-10-05 | Phase 5 has no format to follow | read that spec when Phase 5 starts; use its format only once it has landed | unvalidated |
| A-8 | errcheck's arg-qualified entries (`fmt.Fprintf(os.Stderr)`, `fmt.Fprint(*bytes.Buffer)`) can be matched by the gate the way errcheck matches them: the first argument's type, or for `os.Stderr` the package-level variable itself | errcheck v1.20.0 exclusion syntax in `excludes.go` and its embedded-symbol matching | the gate allows or refuses a call errcheck treats the other way | selftest fixtures for `os.Stderr` allowed and another `*os.File` refused | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Moving code (a file split, a function move across packages) shows as added lines, so every bypass or discard in the moved block becomes due | a split commit's verify lists dozens of findings in code it did not author | Accepted by owner decision Q-3 (2026-10-05): moved code counts as new, as compound-guard treats it; the fix is mechanical. Recorded in the page. Not mitigated with `--color-moved` detection (a second diff reader) |
| R-2 | Type-checked loading is slow or fails on a package with a broken build under some flavor | stage duration; exit 2 on load error | load only the packages of changed shipped files; a load error is exit 2 naming package and flavor, never a pass |
| R-3 | `go list -deps` under the shipped tags fails (a gate tag that no longer builds) | exit 2 from the scope helper | exit 2 naming the build; this is a real defect the gate surfaces |
| R-4 | A `//nolint:errcheck` with no reason passes this gate | grep | `nolintlint require-explanation: true` in the full lint already refuses it; the gate also requires non-empty text after `//` on the marker and reports it otherwise |
| R-5 | `RealClock{}.Now()` written inline in a package with no injected clock passes the clock gate while defeating injection | review | Known limitation: the gate refuses the `time` call, not poor injection; stated on the page |
| R-6 | Two detector definitions drift from errcheck's notion of "error-typed blank" | a case errcheck `-blank` flags that the gate does not | selftest fixtures cover single, tuple, `var`, method value and interface-satisfying error types; types use `types.Implements` against the universe `error` |
| R-7 | The base is missing in a fresh clone or detached checkout with no `origin/main` | exit 2 "no pushed commit behind HEAD" | inherited from `LinesSinceUpstream`; same as compound-guard |
| R-8 | The explicit list misses a built-in default, so setting `disable-default-exclusions: true` makes the full lint report calls it excluded before (592 bare `fmt.Print*` lines alone) | `./le go lint run` gains errcheck findings | `TestExcludeFunctionsCoverErrcheckDefaults` compares the list with the vendored `DefaultExcludedSymbols` and fails on any default neither listed nor recorded as refused; the full lint is a gate owed by the main thread |
| R-9 | An allowed entry is used for a failure that can be acted on: `fmt.Print*` writes the command's real output to stdout, not only diagnostics, and errcheck matches by function, not by intent | review of a new `fmt.Print*` on an output path | Known limitation, accepted under Q-2: a command whose output must be complete writes through an `io.Writer` it holds and checks the error; stated on the style page |
| R-10 | A new Clock method has no same-named `time` function, or the replacement table names a method that was renamed | exit 2 from the refused-set builder | the builder refuses both shapes, so the drift stops the gate instead of narrowing it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible: `./le verify` reports a false red (blocks nothing under `ai/rules/pre-release.md`) or misses a new occurrence |
| How is it reverted? | Single commit revert; no data, config, or format migration |
| Who else touches this path? | `internal/le/verify/engine/stages.go` (every new stage), `internal/le/repo/featuretags` (gate manifest work), `docs/contributing/ze-go-style.md`, `.golangci.yml` (every lint setting change) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch clock-bypass check` in a git repo with an upstream | → | clock-bypass `runCheck` → `Check` | `TestClockBypassCheckJudgesTheUnpushedRange` (real temporary repository with an upstream, as `compoundguard/upstream_test.go`) |
| `./le arch discarded-error check` in a git repo with an upstream | → | discarded-error `runCheck` → `Check` | `TestDiscardedErrorCheckJudgesTheUnpushedRange` |
| `./le verify` stage list | → | `stage("arch clock-bypass", "check")`, `stage("arch discarded-error", "check")` | the expected stage names in `internal/le/verify/engine/verifyengine_test.go` |
| `./le arch clock-bypass selftest`, `./le arch discarded-error selftest` | → | fixture detection | `TestClockBypassSelftestPasses`, `TestDiscardedErrorSelftestPasses` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An unpushed change adds `time.Now()` to a non-test file of a package in the shipped closure | `./le arch clock-bypass check` exits 1 and names `file:line (Fn)`, the Clock method that replaces the call, and the style page |
| AC-2 | Same as AC-1 with `time.Since(x)`, `time.Until(x)`, `time.Sleep(d)`, `time.After(d)`, `time.AfterFunc(d, f)`, `time.NewTimer(d)`, `time.NewTicker(d)`, `time.Tick(d)`, an aliased import of `time`, a dot import, or the function value `time.Now` passed as an argument | exit 1, one finding each; `Since` and `Until` name `Now` as the replacement, `Tick` names `NewTicker` |
| AC-2b | A change uses `time.Duration`, `time.Time`, `time.Unix(...)`, `time.Parse(...)` or a method on a `time.Time` value | exit 0: only the refused functions are judged, never the package |
| AC-2c | The refused-set builder over a fixture `Clock` interface that gains a method `Foo` with no `time.Foo`, or a replacement table row naming a method the interface lacks | exit 2 naming the method; a fixture interface gaining a method whose name matches a `time` function widens the refused set with no gate edit |
| AC-3 | A shipped file holds an existing `time.Now()` on an unchanged line, and the change edits another line of the same file | exit 0 |
| AC-4 | A change adds `time.Now()` in `internal/core/clock`, in a `_test.go` file, in a generated file, or in a package outside the shipped closure (for example `internal/le/...`) | exit 0, file not judged |
| AC-5 | A change adds `time.Now()` to `internal/chaos` (root package, linked by `internal/component/bgp/config`) | exit 1: the boundary is the closure, not the directory name |
| AC-6 | An unpushed change adds `_ = f()` where `f` returns `error`, `v, _ := g()` where `g` returns `(T, error)`, `_, _ = h()` returning `(int, error)`, or `var _ = f()` | `./le arch discarded-error check` exits 1 with one finding per statement |
| AC-7 | A new blank assignment whose blank slot is not error-typed: `v, _ := m[k]`, `v, _ := x.(T)`, `_ = ctx`, `n, _ := utf8.DecodeRune(b)` | exit 0 |
| AC-8 | A new `_ = b.WriteString(s)` on a `*strings.Builder`, `_, _ = fmt.Fprintf(os.Stderr, ...)`, `_, _ = fmt.Println(...)`, `_, _ = rand.Read(b)` from `crypto/rand` (each an `exclude-functions` entry in `.golangci.yml`) | exit 0; removing the entry from a fixture config makes it exit 1; `_, _ = fmt.Fprintf(f, ...)` with `f` an `*os.File` other than `os.Stderr` exits 1 |
| AC-8b | A fixture `.golangci.yml` whose `exclude-functions` entry carries no trailing line comment, or whose errcheck block does not set `disable-default-exclusions: true` | exit 2 naming the entry or the setting; never a pass over an unreasoned or incomplete list |
| AC-8c | The real `.golangci.yml` after this change | `disable-default-exclusions: true`; `exclude-functions` holds the 11 entries it held before plus the 28 qualifying built-ins of the Allowed Discards table, every entry with a reason; every entry of the vendored `errcheck.DefaultExcludedSymbols` is listed |
| AC-9 | A new discard carrying `//nolint:errcheck // best-effort cleanup` on its line | exit 0 |
| AC-10 | A new discard carrying `//nolint:errcheck` with no reason | exit 1, the finding says the reason is missing |
| AC-11 | A multi-line assignment whose RHS call spans lines, and only a continuation line changed | exit 1 (the statement span is judged) |
| AC-12 | A changed shipped file that does not parse, does not type-check under its flavor, or that no lint-matrix flavor admits | exit 2 naming the file and the reason; never 0 |
| AC-13 | No upstream and no `origin/main` | exit 2 naming the missing base (inherited) |
| AC-14 | `./le verify` runs | both stages run, after `arch compound-guard`, and a finding fails the stage |
| AC-15 | `./le arch clock-bypass selftest` / `./le arch discarded-error selftest` | each proves detection against fixtures (flagged and allowed cases of AC-1..AC-11, including AC-2b and the AC-8 first-argument cases) and exits 0; a broken detector makes it exit 1 |
| AC-16 | `.golangci.yml`, `./le go lint run` | unchanged findings: `check-blank` stays false, no `new-from-*` added, and the explicit list excludes exactly the calls the implicit default list excluded |
| AC-17 | The style page | `docs/contributing/ze-go-style.md` states the clock rule (every `time` function the Clock interface replaces, pointing at the interface and the three-row table rather than copying the names) and the discard rule (the explicit reasoned list, the owner's test for an entry, the per-line marker otherwise), names both gates, and no longer says a blank discard passes every tool |
| AC-18 | A change that moves a block holding `time.Sleep(d)` or `_ = f()` to another file or package | exit 1 for each occurrence in the moved block (owner decision Q-3) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestClockBypassFlagsEveryReplacedTimeFunction` | `internal/le/arch/clockbypass/clockbypass_test.go` | AC-1, AC-2: one finding per refused function, each naming its replacing Clock method | |
| `TestClockBypassIgnoresTimeTypesAndOtherFunctions` | same | AC-2b | |
| `TestRefusedSetDerivesFromTheClockInterface` | same | AC-2c: fixture interface gaining a matching method widens the set; an unmatched method or a stale table row is exit 2 | |
| `TestClockBypassIgnoresUnchangedLines` | same | AC-3 | |
| `TestClockBypassSkipsClockPackageTestsGeneratedAndUnshipped` | same | AC-4 | |
| `TestClockBypassJudgesShippedChaosRoot` | same | AC-5 | |
| `TestDiscardFlagsErrorTypedBlankSlots` | `internal/le/arch/discardederror/discardederror_test.go` | AC-6 | |
| `TestDiscardIgnoresNonErrorBlankSlots` | same | AC-7 | |
| `TestDiscardHonoursGolangciExcludeFunctions` | same | AC-8, reads a fixture `.golangci.yml`, including the `os.Stderr` first-argument form against another `*os.File` | |
| `TestExcludeListWithoutReasonOrDefaultsDisabledIsExitTwo` | same | AC-8b | |
| `TestExcludeFunctionsCoverErrcheckDefaults` | same | AC-8c over the real `.golangci.yml` and the vendored `errcheck.DefaultExcludedSymbols`; R-8 | |
| `TestMovedBlockIsJudgedAsNew` | both detector tests | AC-18 | |
| `TestDiscardHonoursNolintErrcheckWithReason` | same | AC-9, AC-10 | |
| `TestDiscardJudgesTheWholeStatementSpan` | same | AC-11 | |
| `TestAFileNoFlavorAdmitsIsExitTwo` | shipped-scope helper test | AC-12 | |
| `TestAnUnparsableOrUntypedFileIsExitTwo` | both detector tests | AC-12 | |
| `TestShippedScopeIsTheTargetClosure` | shipped-scope helper test | closure contains `internal/component/bgp/reactor` and `internal/chaos`, contains no `internal/le/` package; derived from featuretags, not a literal | |
| `TestShippedScopeIncludesHostAndLinuxFiles` | shipped-scope helper test | A-2 | |
| `TestShippedPersonalitiesFeedTheDerivedTagFiles` | `internal/le/repo/featuretags/` | the gokrazy and quickstart lists read the new exported personality declaration | |
| verify stage names | `internal/le/verify/engine/verifyengine_test.go` | AC-14 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| changed-line span vs statement span | lines 1..N | a span touching only the last line of a statement | N-A | a span ending one line before the statement starts (not due) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestClockBypassCheckJudgesTheUnpushedRange` | `internal/le/arch/clockbypass/upstream_test.go` | a developer commits a new `time.Now()` locally and runs the check before a push: red; the same line on the upstream: green | |
| `TestDiscardedErrorCheckJudgesTheUnpushedRange` | `internal/le/arch/discardederror/upstream_test.go` | same for a new `_ = f()` | |

The le action over a real temporary repository is the user path for a tooling gate, as `compoundguard/upstream_test.go` established; there is no `.ci` surface.

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer.

Discrimination (owed before claiming the gates work, `ai/rules/interop-and-goal-validation.md`): break each detector (return no findings), confirm the selftest and both upstream tests go red, restore, confirm green; record the red output in the closure section.

## Files to Modify
- `.golangci.yml` - errcheck: `disable-default-exclusions: true`; the 28 qualifying built-ins added to `exclude-functions`; every entry, old and new, carries its reason as a trailing line comment (the existing group comment above the `textbuf` `fmt` entries stays as context)
- `internal/le/verify/engine/stages.go` - two stages after `arch compound-guard`, with the same scope comment
- `internal/le/verify/engine/verifyengine_test.go` - expected stage names
- `internal/le/register.go` - blank imports of the two new areas
- `internal/le/go/lint/matrix.go` - exported accessor answering the flavors (and their build contexts) so the gate can pick one that admits a file; no new flavor data
- `internal/le/repo/featuretags/featuretags.go` (+ `daemontags.go`) - export one declaration of the shipped personalities (distro, appliance, installer with their base tags); `targets` reads it instead of its local literals
- `docs/contributing/ze-go-style.md` - "Every error is handled" rewritten; new "Time comes from the injected clock" subsection; two rows in "The rules a tool enforces"; `<!-- source: -->` anchors
- `docs/contributing/ze-go-style-background.md` - "How the gates choose what they judge" covers the three line gates and the shipped-closure scope of the two new ones
- `ai/INDEX.md` - two command rows and keyword rows (clock bypass, time.Now, time.Sleep, time.NewTicker, discarded error, blank discard, check-blank, allowed discards)
- `ai/rules/points/go-standards/...` (the point carrying the banned-pattern list, per `ai/rules/rule-format.md`) - add the clock bypass to the banned patterns with a pointer to the gate; regenerate with `./le ai rules condensed-update`
- `plan/journal/clock-bypass.md`, `plan/journal/discarded-error-becomes-destructive.md` - capture record, Phase 5 only, in the lifecycle spec's format

## Files to Create
- `internal/le/arch/clockbypass/{actions.go,clockbypass.go,report.go,register.go,selftest.go}` + tests and `testdata/` fixtures
- `internal/le/arch/discardederror/{actions.go,discardederror.go,report.go,register.go,selftest.go}` + tests and `testdata/` fixtures
- `internal/le/arch/shipped/` - the shipped-scope helper: closure from `go list -deps`, per-file flavor choice, type-checked load. Shared by both gates (two users, both in this spec)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le tooling, no product config |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | Yes | `./le arch clock-bypass {check,selftest}`, `./le arch discarded-error {check,selftest}` via `leroot.Register` in each `register.go` (le, not `cmd/ze`) |
| CLI grammar (keyword before value) | Yes | no parameters; actions follow `leaction` (`ai/rules/cli.md`) |
| Editor autocomplete | N-A | le actions complete from the registry automatically |
| Functional test for new RPC/API | Yes | the two `upstream_test.go` tests |
| Pipe completeness | N-A | le output, not the ze CLI pipe system |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no runtime dependency of the shipped binary; the gate uses git and the go toolchain, which le already requires |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | developer gate, not an operator feature |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | the ze CLI is unchanged; le commands go in `ai/INDEX.md` |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | No | - |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/ze-go-style-background.md` (gate scope); `docs/contributing/testing.md` only if it lists verify stages (check) |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | the `// Design:` headers point at `ze-go-style.md` as compound-guard's do |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `ai/INDEX.md` command table (le area registry) |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derive with `./le spec citation anchors spec plan/spec-lint-ratchet-clock-and-discarded-errors.md`; expected: `matrix.go` → `docs/contributing/testing.md`; `featuretags` → `docs/architecture/plugin/feature-gates.md`; `stages.go` → verify docs. Declared by `// Design:` headers of changed files: `docs/architecture/core-design.md` (le composition, `register.go`, `actions.go`: unaffected, one more area registers the same way), `ai/rules/plugins.md` (featuretags: the manifest stays the single source; the personality export is checked against its "static consumers are generated" statement and updated if it names the consumers), `docs/architecture/testing/verify-freshness-scope.md` (`repochanged` reused unchanged: unaffected unless it lists the line-gate consumers, in which case the two new gates are added). Advisory source mentions of `stages.go`: `docs/DESIGN.md`, `docs/architecture/site-facts.md`, `docs/contributing/gh-pages.md`, `docs/functional-tests.md`; each is updated only if it enumerates verify stages, otherwise unaffected |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ze-go-style.md` discard example `f, _ := open()` stays correct; its "passes" claim changes |

## Implementation Steps

1. **Phase: Wiring** - register both areas with stub `check`/`selftest`, add the two verify stages and the expected names, write the two upstream tests (red: stub reports nothing).
   - Tests: wiring table rows
   - Files: `register.go` ×2, `internal/le/register.go`, `stages.go`, `verifyengine_test.go`
   - Verify: `./le arch clock-bypass check` and `./le arch discarded-error check` resolve; upstream tests fail for the expected reason
2. **Phase: Shipped scope** - export the shipped personalities from featuretags (and move `targets` onto them), export the flavor accessor from the lint matrix, write `internal/le/arch/shipped` (closure, flavor choice, typed load, exit-2 paths).
   - Tests: `TestShippedScopeIsTheTargetClosure`, `TestShippedScopeIncludesHostAndLinuxFiles`, `TestAFileNoFlavorAdmitsIsExitTwo`, `TestShippedPersonalitiesFeedTheDerivedTagFiles`; `./le repo feature-tags check` stays green
3. **Phase: clock-bypass detector** - the refused-set builder over the `Clock` method set and the three-row replacement table, AC-1..AC-5 (with AC-2b, AC-2c), AC-12, AC-18, selftest fixtures; page subsection written in this phase.
4. **Phase: discarded-error detector** - the `.golangci.yml` edit (defaults disabled, 28 built-ins listed, a reason on every entry) lands in this phase with the reader that refuses an unreasoned entry; AC-6..AC-11 (with AC-8b, AC-8c), AC-18, selftest fixtures; page section rewritten in this phase. The main thread's `./le go lint run` proves AC-16.
5. **Phase: Capture record and rules** - INDEX rows, go-standards point + condensed regeneration, background page; the two journal files record their capture in the format `plan/spec-journal-class-lifecycle.md` defines (cross-reference only; this spec does not design it). If that spec has not landed when this phase starts, ask the main thread which format to use rather than inventing one. Each class takes a `check` capture naming its gate's check function and the upstream test as proof; under that spec one row dated after the capture makes the class DUE again, and the pass that follows repairs the gate.
6. **Phase: Discrimination** - break each detector, record red, restore.

Gates owed by the implementer's main thread (not runnable inside a five-minute agent window): `./le go lint run`, `./le verify worktree`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | Clock detection is by `types.Func` identity in package `time`, not by text, so aliases and dot imports are caught and a local `Now` method is not |
| Correctness | A discard is error-typed by `go/types`, per slot; the exclude list is read from `.golangci.yml`, never copied |
| Single declaration | The refused clock functions come from the `Clock` method set plus the three-row table; no hand-written list of the six method names in the gate, the page or the rule point |
| Single declaration | The allowed discards are the `.golangci.yml` entries only; errcheck's implicit list is disabled, and the only other copy is the vendored default list, which a test compares |
| Silent pass | Every "cannot judge" path exits 2: no base, parse error, type error, no admitting flavor, `go list` failure, unreadable `.golangci.yml`, an entry without a reason, defaults not disabled, a Clock method with no `time` twin |
| Single declaration | Shipped tag sets come from featuretags; no tag literal in the new packages; no list of shipped directories anywhere |
| Rule: no-layering | No baseline file, no second change-set reader, no golangci `new-from-*` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Two gates registered | `./le arch clock-bypass selftest`, `./le arch discarded-error selftest` exit 0 |
| Two verify stages | `grep -n 'arch clock-bypass\|arch discarded-error' internal/le/verify/engine/stages.go` |
| Page updated | `grep -n 'injected clock\|discarded-error check' docs/contributing/ze-go-style.md` |
| golangci errcheck explicit | `git diff .golangci.yml` touches only the errcheck block; `TestExcludeFunctionsCoverErrcheckDefaults` passes; `./le go lint run` findings unchanged (main thread) |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | paths from git diff are repo-relative; refuse `..` and absolute paths as `repochanged` already does |
| Command execution | `go list` and git run with argv, never a shell string |

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

- The boundary question answered by `go list -deps` rather than directory names: `internal/chaos` ships (imported by `internal/component/bgp/config`), so "chaos is dev tooling" is false for its root package.
- `postFormatGo` already shows golangci's new-from mode is used only for advisory edit-time feedback; no gate in the repo uses it.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Change-scoped line gate on the unpushed range, as two `arch` areas beside `compound-guard`, run as verify stages | (a) golangci `new-from-rev`/`new-from-merge-base` with `check-blank: true` and a forbidigo time pattern; (b) committed baseline file (per-file counts or per-occurrence fingerprints); (c) commit-time gate in `./le commit create` keyed on named paths vs HEAD | (a) `issues.new-from-*` is global and would also hide every other linter's backlog in the full lint; a separate golangci pass needs a second config (second declaration of errcheck settings) and the 16-flavor matrix; revgrep has the same line semantics anyway. A rev pinned to HEAD judges only uncommitted work, so a committed occurrence escapes. (b) about 2,000 rows every concurrent session must edit, merge contention on rebase (the ci-sleep delta form solves this only for a total), and a second declaration of what HEAD already holds. (c) `ai/rules/pre-release.md` puts gates at the push, and compound-guard already established this home; the merge-base is read from refs, so rebases and concurrent commits need no baseline maintenance |
| Shipped scope = `go list -deps` closure of `cmd/ze`, `cmd/ze-serial-shell` (distro and appliance personalities) and `cmd/ze-installer`, Linux and host GOOS | directory list (internal/component, internal/plugins, ...) | derived from the build, as the owner asked; the closure includes `internal/core` (98 packages), `pkg/*`, `internal/exabgp`, `internal/chaos` root and excludes all of `internal/le`, `internal/test` |
| Allowed discards: the explicit `.golangci.yml` errcheck `exclude-functions` list, with `disable-default-exclusions: true` and a reason on every entry, plus `//nolint:errcheck // reason` on the line | leave errcheck's built-in list implicit; a new marker vocabulary | owner decision Q-2: an entry is allowed only when its failure is one nothing can act on, and the set must be explicit and reasoned. An implicit list is a second declaration nobody reviewed, and an errcheck upgrade would widen it silently; a new marker would be a second vocabulary |
| Refused clock functions derive from the `Clock` method set, plus a three-row table for `Since`, `Until`, `Tick` | a hand-written list of nine names in the gate | owner decision Q-1; the interface is the declaration of what the injected clock replaces, so a method added there widens the gate with no edit, and the table holds only the facts the interface cannot carry (which method replaces a function it has no twin for) |
| `time.Tick` is refused and replaced by `NewTicker`, with no new Clock method | add `Tick` to the interface | `Tick` is `NewTicker(d)` without the `Stop` the caller should own; the interface already carries the replacement, and shipped code holds no `time.Tick` today |
| No escape marker for the clock class; `internal/core/clock` is the only exempt package | `//nolint:forbidigo // reason` | `RealClock` gives real time, real sleeps and real timers wherever they are genuinely needed, so a real need is met by injection; a marker for a linter that does not report the line would mislead |
| Native detectors with `go/types` | AST-only detection | AST cannot tell an error slot from any other (`n, _ := utf8.DecodeRune(b)`), and a false refusal teaches people to ignore the gate |
| Keep `TestNoDirectTimeCalls` | delete it as superseded | it is stricter in its two directories: it bans existing lines too, where the gate judges only changed ones; the gate does not replace it |

### Owner Decisions (2026-10-05)

| # | Question | Decision | Effect on this spec |
|---|----------|----------|---------------------|
| Q-1 | Should the clock class also cover `time.Sleep`, `time.After`, `time.AfterFunc`, `time.NewTimer`, `time.NewTicker`, `time.Tick`? | Yes. `clock.Clock` offers `Now`, `Sleep`, `After`, `AfterFunc`, `NewTimer`, `NewTicker` (verified in `internal/core/clock/clock.go`); `time.Tick` has no Clock equivalent and is covered by `NewTicker` | Task, Data Flow step 5, AC-1, AC-2, AC-2b, AC-2c, A-5 re-measured (649 lines in 316 files), R-10, page and rule point wording |
| Q-2 | Should errcheck's built-in default exclusions count as allowed discards? | Only those whose failure nothing can act on, listed explicitly with a reason each in `.golangci.yml`, never the implicit list wholesale | `.golangci.yml` added to Files to Modify, A-4 validated, AC-8, AC-8b, AC-8c, AC-16, R-8, R-9, Allowed Discards table below |
| Q-3 | Is a moved block (file split) due, as compound-guard treats it? | Yes, moved code counts as new | R-1 accepted, AC-18 |

### Allowed Discards (Q-2)

Every entry of errcheck v1.20.0 `DefaultExcludedSymbols` judged against the owner's test. The reason column is what each entry's trailing comment in `.golangci.yml` says.

| Entry | Qualifies | Reason |
|-------|-----------|--------|
| `(*bytes.Buffer).Write`, `WriteByte`, `WriteRune`, `WriteString` | Yes | the error is always nil; the buffer panics with `ErrTooLarge` instead (`bytes` docs) |
| `fmt.Fprint`, `fmt.Fprintf`, `fmt.Fprintln` with `*bytes.Buffer` | Yes | the writer's error is always nil |
| `fmt.Fprint`, `fmt.Fprintf`, `fmt.Fprintln` with `*strings.Builder` | Yes | the writer's error is always nil |
| `(*strings.Builder).Write`, `WriteByte`, `WriteRune`, `WriteString` | Yes, already listed | the error is always nil |
| `crypto/rand.Read` | Yes | "It never returns an error, and always fills b entirely" (Go 1.24 onward; `go.mod` is 1.27) |
| `fmt.Print`, `fmt.Printf`, `fmt.Println` | Yes | a write to the process's stdout whose failure has no other channel to be reported on (owner's stdout example); R-9 records that the match is by function, not intent |
| `fmt.Fprint`, `fmt.Fprintf`, `fmt.Fprintln` with `os.Stderr` | Yes | stderr is the last diagnostic channel; a failure writing to it cannot be reported anywhere |
| `(*io.PipeReader).CloseWithError`, `(*io.PipeWriter).CloseWithError` | Yes | "always returns nil" (`io` docs) |
| `math/rand.Read`, `(*math/rand.Rand).Read` | Yes | "always returns len(p) and a nil error" (`math/rand` docs) |
| `(hash.Hash).Write` | Yes | "It never returns an error" (`hash` docs) |
| `(*crypto/sha3.SHA3).Write`, `(*crypto/sha3.SHAKE).Read`, `(*crypto/sha3.SHAKE).Write` | Yes | the error result is never set; misuse panics instead (Go 1.27 `crypto/internal/fips140/sha3`) |
| `(*hash/maphash.Hash).Write`, `WriteByte`, `WriteString` | Yes | "never fails; the count and error result are for implementing io.Writer" (`hash/maphash` docs) |

No built-in entry fails the test: all 32 are either nil on every path or a diagnostic stream with no other channel, so the refused set starts empty. A future default that fails it is recorded, with its reason, in the refused set `TestExcludeFunctionsCoverErrcheckDefaults` holds beside the comparison, never in this spec, which is deleted at closure.

## Known Limitations
- Existing occurrences stay until a change touches their lines; no sweep (owner scope).
- The clock gate refuses the `time` call, not weak injection (`clock.RealClock{}.Now()` inline passes).
- `fmt.Print*` discards pass by function name, including a stdout write that carries a command's real output (R-9).
- Dev tooling outside the shipped closure is not judged.

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
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
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
- [ ] Functional `.ci` tests for end-to-end behavior (N-A: tooling; the upstream tests drive the le entry point)
- [ ] Interop tests for protocol features (N-A: tooling)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/spec-lint-ratchet-clock-and-discarded-errors.md` only, in the same `./le commit create` script
