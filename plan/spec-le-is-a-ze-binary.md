# Spec: le is a ze binary

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 15 of 15 steps landed, the swap in `eae282592` |
| Handoff | - |
| Updated | 2026-08-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make `le` a `cmd/ze` personality, built on the same engine as `ze`: the same
registry, the same command grammar, the same pipe machinery. `ze` stays the
product. `le` becomes repo management and development. **The separation is USE,
and it is carried by the plugin sets and the binaries, never by the engine.**

Three goals, each standing on its own:

1. **One ENGINE, two plugin sets, two binaries (owner directive, 2026-08-26).**
   `ze` and `le` share the registry, the command grammar and the pipe
   machinery. They register no plugin in common, and **`ze` is never compiled
   with `le` code.** The architecture must not PRECLUDE a crossing, since that
   is the test of whether the engine is genuinely shared rather than merely
   similar, but no `ze` build ever performs one. Two registries kept in
   agreement by hand is the failure this exists to prevent; a shipped product
   carrying dev tooling is the failure the never-linked rule prevents. The rule
   is DIRECTIONAL rather than symmetric, for a measured reason set out below:
   several `le` tools MUST link the product in order to introspect it.
2. **All first-party development tooling is compiled Go.** Native packages are
   built by `go build ./...`, seen by `go vet` and the linter, and called as
   functions by their tests.
3. **`le` inherits the CLI contract** instead of reimplementing it: the
   keyword-before-value grammar, `| json`, `| yaml` and `| table` on every
   command, completion, `help`, and the exit-code conventions.

**What "shared engine" means mechanically.** Both binaries import the same
`internal/component/command/registry` and register through the same
`MustRegisterRootHandler`. `ze` never links a `le/` package, and that is provable
rather than asserted: `go list -deps` over each `ze` flavour must not name one.
Measured 2026-08-26, the mechanism already works this way for the existing
programs -- `internal/perf/cli` is absent from `ze`'s 630-package dependency list
with `ze_perf` off and present with it on.

**The rule is DIRECTIONAL, and the reason is measured rather than chosen
(2026-08-26).** Twelve tool files under the retired `scripts/` (current producer: `internal/le/`) blank-import
`internal/component/plugin/all`, the product composition root:
`inventory/commands.go`, `inventory/inventory.go`, `docvalid/commands.go`,
`docvalid/doc_drift.go`, `codegen/plugin_imports.go`, `checks/cli_grammar.go`,
`checks/ci_dispatch_commands.go`, `checks/config_claims.go`,
`checks/yang_leaf_mentions.go` and three test files. They must: their job is to
enumerate what `ze` REGISTERS, and a command inventory, a CLI-grammar check or a
YANG-leaf check cannot be computed without loading the registry it judges. So
`le` linking the product is inherent to what `le` is for, and forbidding it would
forbid the tools.

The owner's directive is preserved in the direction that carries the risk. `ze`
is what ships, so `ze` must never carry dev tooling. `le` is a build-host binary
that nobody deploys, so `le` linking the product costs nothing and enables the
introspection. What `le` must not do is REGISTER a product command as its own,
which is a different property and the one AC-3 now pins.

**Strategy completed: port everything, then swap (owner directive, 2026-08-26).**
The Go `le` was built alongside the Python implementation until it covered every
feature. The clean cutover then removed the Python implementation, `scripts/`,
the Makefile and `mk/`. Current development commands dispatch directly through
the native actions under `internal/le/`.

**Non-goal:** redesigning what the tools DO. Behaviour is preserved and proven
preserved. This spec moves and rewires; it does not rewrite checks.

**THE END STATE IS LARGER THAN THIS SPEC FIRST SAID (owner directives,
2026-08-26).** The swap is not the finish line. Three things follow it, each a
step of its own, each measured:

| # | End state | Surface |
|---|-----------|---------|
| A | **`scripts/` was deleted after its producers moved into compiled Go.** | Historical inventory: 281 code files plus fixtures, 11 directories |
| B | **The Makefile and `mk/` were deleted.** Native actions replace their behavior without preserving target identities as compatibility verbs | Historical inventory: 132 root targets, 279 across 19 `mk/` files, 1638 recipe lines |
| C | **Every document names the native action or Go producer that replaced them** | ~4500 path references, plus `ai/INSTRUCTIONS.md`, `ai/rules/`, `ai/INDEX.md`, the hooks, and every source anchor |

→ Historical constraint: **job admission was the hardest dependency in the
migration, and the Makefile could not be removed before it moved.** The retired
`scripts/dev/ze-run.sh` (current producer: `internal/le/job/job.go`) had been invoked by 79 recipe lines and had been the one
place that decided whether a heavy job could run. Its replacement is
`./le job run`, backed by `internal/le/job/`, which preserves fail-closed
registry handling and nested admission without Make-level concurrency.

→ Historical constraint: the retired Makefile also exported `GOCACHE`,
`GOLANGCI_LINT_CACHE` and `CGO_ENABLED`. The native replacement,
`internal/le/gotoolchain`, owns those settings. Its comments retain why each is
load-bearing: a cache under TMPDIR breaks Unix-socket tests when paths exceed the
kernel limit, and an ambient Go version newer than the version in `go.mod` can
make golangci-lint reject export data while reporting zero issues.

→ Decision: a file-structure reorganization sat between A and C. The former
top-level tool tree existed because `./le` was still the Python shim.

→ Decision (owner, amended after the cutover): **`./le` is the repository
launcher for the compiled `cmd/ze` personality.** The retired Python launcher
was used only during the migration and has no compatibility route in the
current tree. The launcher builds the native binary when needed, and every
development operation is selected by an area and that area's own action verb.

→ Decision (owner, 2026-08-26): **the launcher also carries job admission, and
the build it may trigger is itself an admitted job.** The two halves are one
mechanism rather than two features bolted together. Building `ze` compiles 630
packages and building `le` compiles the tool tree, so two sessions that both
find `bin/ze` missing and both start a build are exactly the oversubscription
the retired `scripts/dev/ze-run.sh` (current producer: `internal/le/job/job.go`) was written for after the 2026-08-17 freeze. A launcher
that ensured the binary existed WITHOUT admission would reintroduce the fault at
the one moment several sessions are most likely to collide: a fresh checkout, or
the first command after a `clean`.

So `./ze` and `./le` each: take a slot, build their binary if it is absent,
release, then exec. The registry, the fail-closed parse, the attach-on-identical
-work path and the `ZE_RUN_JOB` nesting rule all apply to that build the way
they apply to any other heavy job -- and attach is worth having here, because two
sessions racing on the same absent binary is precisely the case where one run
should answer both.

→ Historical constraint: admission was a precondition of removing the Makefile.
The current launcher owns admission through `internal/le/job/`, so `./le`
does not depend on a compatibility target.

→ Decision (owner, 2026-08-26): **`le` MUST NOT exec its own code. Where a Go
package in this tree holds the answer, the caller makes a FUNCTION CALL.**
`ReadGates` (`internal/le/parity/parity.go` <!-- doc-links: ignore (the parity census package is gone from the tree; this spec is still open on it) -->) is the live instance: it builds an
`exec.CommandContext` against `filepath.Join(root, "le")`, pipes stdout and
decodes JSON, and its own comment says the Python "stays the denominator until
the swap moves the declaration into Go". That is a TRANSITIONAL ARTIFACT, not a
design choice. The catalogue is already Go data for every ported area, in
`internal/le/leaction` and the `leroot` registry, so when `gates` lands the
subprocess, its timeout, its pipe and its decoder all lose their job together.

That dissolves the dark-versus-red hazard rather than managing it: a census
asking an in-process registry has no child that can fail to start. One honest
obligation survives -- while both implementations exist the census must still
report the PYTHON gate list as the denominator, so whoever converts the
numerator owes a statement of how that is still obtained.

The general form is the rule, and it reaches further than `le` calling `le`.
the retired `mk/check-docs.mk` (current producer: `internal/le/doc/check/actions.go`) runs seven `python3 scripts/dev/*.py (retired; current producer: `internal/le/`)` lines in sequence
(`rules_points.py` three times, `rules_index.py`, `rules_lint.py`,
`rules_condensed.py`, `code_to_docs.py`), every one of which a `internal/le`
package now implements or soon will. Each is a process start, a Python
interpreter and a re-parse of the same corpus. In one binary they are calls
sharing one parse.

Target layout:

| Path | Holds | Linked into |
|------|-------|-------------|
| `internal/component/command/`, `internal/core/` | the shared ENGINE: registry, grammar, pipes | both |
| `cmd/ze/` | the product's entry point and composition root, unchanged | `ze` |
| `internal/`, `internal/plugins/` (product trees) | `ze`'s plugins | `ze` only |
| `cmd/ze/` | shared entry point for the `ze` and `le` personalities | both |
| `internal/le/register.go` | composition root: blank imports say what `le` carries | `le` |
| `internal/le/<tool>/` | `le`'s plugins, one package per tool | `le` only |
| `internal/le/parity/` <!-- doc-links: ignore (the parity census package is gone from the tree; this spec is still open on it) --> | the census measuring how much is ported | `le` only |

→ Decision: `le`'s plugins sit in a top-level tree, NOT under `internal/`
alongside the product's. The directory is the statement that these are a
different program's plugins over the same engine, and it makes the never-linked
rule readable rather than merely enforced.

→ Constraint (measured 2026-08-26, step 1): the top-level tool tree could not
use the name `le/`. The repository root already held the executable Python shim
`./le`, and a directory could not share that name. Step 1 therefore used a
different top-level name. The owner's `le/` choice described a sibling of
`internal/` and `cmd/`; changing the spelling was a rename, not a reversal.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/core-design.md` - the small-core-plus-registration pattern this reuses
  → Decision: the core discovers features through registries and never imports them directly, which is exactly the property that lets one import move a tool between binaries
  → Constraint: no per-feature switch, field, or factory may be added to a shared package to make a tool reachable

- [ ] `ai/rules/cli.md` - the command contract every `le` command inherits
  → Constraint: keyword before value; every command supports all pipe operators; the response payload is structured data so `| json`, `| yaml` and `| table` each render it
  → Constraint: a ported tool that prints free text and returns no structured payload has NOT met the contract

- [ ] `ai/patterns/cli-command.md` - the structural template for a command
  → Constraint: use it per tool; do not invent a per-tool shape

- [ ] `ai/rules/architecture.md` - tier rules
  → Decision: the former top-level tool tree sat outside `internal/`. The tier check did not scan it. The binary dependency tests enforced the boundary instead.
  → Historical decision (step 1, 2026-08-26): the retired `make ze-tier-check` target classified only `internal/core`, `internal/component`, and `internal/plugins`; its current replacement is `./le tier check`. `TestZeLinksNoLePlugin` and `TestLeRegistersNoProductCommand` checked the program boundary.

- [ ] `ai/rules/no-layering.md` - "delete X first, then implement Y"
  → Decision: the owner has chosen duplicate-then-swap instead. The rule's concern, silent drift between two implementations, is answered by the parity gate (AC-9), not waived

- [ ] `ai/rules/simplicity.md` - the simplest fully correct answer
  → Decision: no per-tool `cmd/` directories and no `dev-gates.txt` build-tag manifest, because neither has a consumer

### RFC Summaries (Scope: protocol)

N/A - this spec touches no protocol surface.

**Key insights:** (minimal context to resume after compaction)

- `internal/perf/cli/register.go` is the whole precedent: `init()` calls `registry.MustRegisterRootHandler("perf", handler, Meta{...})`, and `cmd/ze/ze_perf_register.go` is six lines, a build tag plus one blank import.
- `rootHandlers` is package-level PER-PROCESS state, so two packages owning one name meet only when both are linked into one binary. Under the never-linked rule they never are, which is why `le perf` and `ze perf` can coexist. `MustRegisterRootHandler` panics on a duplicate, so if the rule were ever broken the failure is a startup crash rather than a silent shadow.
- `go list -deps` is how the never-linked rule is CHECKED. Measured: `internal/perf/cli` is absent from `ze`'s 630-package list with `ze_perf` off and present with it on.
- The 180 seconds of `go run` link time is a consequence, never the argument. The argument is the shared engine.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)

- [ ] `internal/perf/cli/register.go` - the per-program registration precedent, read in full
- [ ] `internal/component/command/registry/registry.go` - `RegisterRootHandler` at :246 refuses empty name, nil handler and duplicate name; `MustRegisterRootHandler` at :264 panics on that error; `Meta` at :148 is `{Description, Mode, Section, Subs, SubsFunc}`
- [ ] `cmd/ze/ze_perf_register.go` - six lines: `//go:build ze_perf` and one blank import
- [ ] `internal/le/leaction/leaction.go` - action dispatch, first-failure exit propagation, argument grammar and structured answers
- [ ] `internal/le/leroot/dispatch.go` - root ownership, dispatch and shared rendering
- [ ] `internal/le/contract_test.go` - deleted-producer and package-layout contracts
- [ ] `internal/le/functional/actions.go` and `internal/le/functional/suites.go` - the native suite table and area-local verbs
- [ ] `feature-gates.txt` - the manifest pattern, one line per gated package, every consumer deriving from it

**Historical baseline, measured before the clean cutover.** The Python
implementation under `scripts/le/` held 59 files, 8007 lines of code and 4011
lines of tests. Its 156 gates ran as imported Python, `go run`, shell, `go test`,
or `go build`. The Go half held 50 build-ignored files across seven `scripts/`
directories and was invisible to ordinary builds, vet and lint.

The current implementation is compiled under `internal/le/`. `./le` dispatches
the registered package actions in-process, and `TestNoPythonLeRemains` prevents
the retired producer tree from returning.

Measured on this machine, 2026-08-26: `go run` of a warm-cached script costs
2411 ms, a Python gate imported in-process 165 ms, and a no-op `go build` of an
unchanged package 270 ms. 36 `go run` gates in a sweep plus 40 in the test suites
is roughly 180 seconds of linker time per full run, buying nothing.

**Behavior to preserve:**

- The behavior behind each retired first-party entry point has one native action.
- The first failing action's own exit code propagates; `internal/le/commit/prepare.go` distinguishes 3 from 1.
- The `why` text attached to each action remains structured metadata.
- `ZE_REPO_ROOT` and its discovery contract remain unchanged.

**Behavior changed by the clean cutover:**

- Make target names are not compatibility identities. Each Go area declares its
  own concise verbs.
- In-process Go dispatch replaced forked `go run`, imported Python and shell
  producers.
- Compiled tooling is governed by ordinary builds, vet, lint and tier checks.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

- A developer types `./le <area> <verb>` or a native `ze <command>`.
- Format at entry: an argv vector; no wire bytes, no config tree.

### Transformation Path

1. Historically, an `mk/*.mk` shim forwarded into the Python `./le`, which
   resolved areas and either imported a Python producer or forked a Go or shell
   producer.
2. During the port, that route stayed unchanged so a partial Go implementation
   could not become the developer entry point.
3. Current state: the `cmd/ze` personality dispatches through
   `internal/component/command/registry`; the handler registered by
   `internal/le/<tool>/register.go` runs in-process and returns a structured
   payload for the pipe operators.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| `./le` ↔ native area action | argv, structured answer, exit code | `TestEveryMakeProducerHasAReachableNativeAction` |
| `le` ↔ registry | `MustRegisterRootHandler` at init, `LookupRoot` at dispatch | No |
| Dev tooling ↔ the compiled module | tool packages join `go build ./...`, `go vet`, the tier check | No |
| `le` ↔ `ze` | a blank import in either composition root | No |

### Integration Points

- `internal/component/command/registry` - the existing registry both binaries share; `le` adds root handlers to it exactly as `internal/perf/cli` does.
- `internal/component/command` - pipe filters, aliases, answer shapes and column order, which a ported tool registers rather than reimplements.

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
| A-1 | The 50 tool files compile once `//go:build ignore` is dropped | They type-check today under `go run` | A burst of latent errors on first port | Drop the tag on ONE file and build, in step 1 | **confirmed for the compiler, broken for the linter, 2026-08-26.** the retired `scripts/lint/consistency.go` (current producer: `internal/le/consistency/consistency.go`) with the tag removed: `go vet ./scripts/lint/` exits 0. The same file under `golangci-lint run --build-tags ze_core` reports SIX findings in 400 lines (errcheck 2, gosec 2, gocritic 1, nilerr 1), one of them a real defect: `:353` returns nil on a non-nil error. The tag was hiding the linter, not the compiler, so the port's cost is lint remediation rather than type errors. The tag was restored |
| A-2 | No tool's imports pull anything the module would rather not have | Inspected the retired `scripts/lint` (current producer: `internal/le/`) and the retired `scripts/docvalid` (current producer: `internal/le/`) only; the rest is assumption | Module dependency growth | `go list -deps` per tool package before porting it | **confirmed for the retired `scripts/lint` (current producer: `internal/le/`), 2026-08-26.** `go list -deps` over the `consistency` package in the former top-level tool tree named 281 packages and no third-party module. Its imports were `bufio`, `fmt`, `os`, `path/filepath`, `regexp`, `sort`, `strings`, `internal/core/textbuf`, and the `lepath` package in that tree. |
| A-3 | Behaviour is preserved by moving `main()` to `Run(args) int` | The signature is the only forced change | Silent behaviour drift | Per tool: run old and new over the real tree, diff exit code and output | **confirmed for the retired `scripts/lint` (current producer: `internal/le/`), 2026-08-26.** Over this checkout the script and the command report the same 1250 lines and both exit 1. The SEQUENCE differs and the script is what varies: two runs of the script disagree with each other, because `checkCrossRefs` iterates a map. `test/ui/le-consistency-answers.ci` re-runs both over the checkout in every ui suite. **Confirmed again for the retired `scripts/vendor` (current producer: `internal/le/`), 2026-08-26, including a WRITING tool.** Over 7 fixture trees each half of the pair answers the same exit code, the same stdout and the same stderr, and the sync leaves a byte-identical TREE behind -- which an output comparison alone would not prove. `test/ui/le-vendor-web-answers.ci` re-runs all three gates over the checkout, the sync over two copies of the real 3.6 MB asset trees. **Confirmed again for the retired `scripts/inventory` (current producer: `internal/le/`), 2026-08-26, including a REGISTRY-derived answer.** Both gates agree byte for byte over the checkout and over two fixture trees, in the page AND in the `--json` rendering, once the generation timestamp is normalized: both sides stamp the minute they ran in, and that is the only difference either way. `test/ui/le-inventory-answers.ci` builds all three binaries under the FULL feature tag set, which is what makes a registry-derived comparison mean anything -- a reduced set compiles modules out, and the two sides then disagree about the PRODUCT rather than about the port. **Confirmed again for the retired `scripts/codegen` (current producer: `internal/le/`), 2026-08-26, and a GENERATOR needs one thing more: the bytes it WRITES.** Output parity alone would pass a pair that agrees about "current" and emits different files, which silently invalidates `ze-generated-files-check` for everyone. the retired `scripts/codegen/parity_test.go` (current producer: `internal/le/repository/`) therefore compares the resulting TREE after each write, over 11 fixture trees across the five generators, and `test/ui/le-codegen-answers.ci` derives the same fact over the real checkout without writing: a check regenerates in memory and compares against what is committed, so a green check on both halves says both would emit exactly the committed bytes. The network generator has no check twin and is compared through a CONNECT proxy that terminates TLS itself, which is the only seam a script naming five fixed https URLs in a package variable leaves a test |
| A-4 | Most Python test CONTENT transfers as intent, not as code | Cases and reasoning are language-independent; the harness is not | Rewrite cost higher than planned | Port one area's tests first and measure | **confirmed, 2026-08-26, on the first Python port.** the retired `scripts/zeledon/post_weekly_test.py` (current producer: `internal/le/weekly/`) declares 11 cases in 276 lines. TEN transfer as INTENT and none as code: each `mock.patch.object` becomes a field the caller fills, so `subprocess.run` becomes `Poster.Send`, `time.sleep` becomes `Poster.Sleep`, `datetime.date.today()` becomes `Poster.Today` and `WEEKLY_DIR` becomes `Poster.ArchiveDir`. That substitution is the whole of the rewrite, and it is what lets the Go tests run with no channel, no clock and no home directory. The ELEVENTH does not transfer at all: `test_help_uses_canonical_weekly_post_paths` asserts that argparse's help names `python3 scripts/zeledon/post_weekly.py (retired; current producer: `internal/le/weekly/weekly.go`)`, and the port has no argparse and no such invocation -- its help is the registry Description. The 11 cases became 53 Go tests, plus 9 side-by-side ones in the retired `scripts/zeledon/parity_test.go` (current producer: `internal/le/weekly/`); the growth is boundary cases the Python never had, not a harder port |
| A-5 | A `le` command name that a `ze` root also uses is HARMLESS, because the two are never linked into one binary | `rootHandlers` (`internal/component/command/registry/registry.go`) is package-level per-process state, so two packages owning one name meet only when both are linked. The owner ruled on 2026-08-26 that they never are | If a build ever did link both, `MustRegisterRootHandler` panics at init -- loud, never a silent shadow | AC-2 and AC-3 prove the never-linked premise by `go list -deps`; the collision needs no separate guard | **confirmed by design, 2026-08-26.** Measured: today's 22 `le` area names collide with none of `ze`'s 34; the verb-first split collides on exactly one, `perf`, and under the never-linked rule that costs nothing |
| A-6 | A ported tool can answer structured data without redesigning it | `ai/rules/cli.md` requires it; unmeasured for tools that print prose reports | The port becomes a rewrite for those tools | Port the retired `scripts/lint` (current producer: `internal/le/`) first and see what its output costs | **confirmed, 2026-08-26, and the cost is 12 lines of shared code plus a payload declaration.** the retired `scripts/lint/consistency.go` (current producer: `internal/le/consistency/consistency.go`) is the hardest case the step table holds: a colored severity report with its own grouping. The port cost is set out under "What A-6 Measured" below. The engine renders rows as a table, which is right for an inventory and wrong for a report, so `leroot.Prose` lets a payload carry its own DEFAULT rendering; every pipe operator still goes to the engine. Nothing about the checks was redesigned, and no JSON, YAML or table code exists in the tool |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A ported test silently weakens: 40 subprocess tests become function calls | The weakened-test audit reports a regression | Each conversion states, per case, what the old assertion proved and where that proof now lives; a `test/weakened.md` row per carrier |
| R-2 | The two sides drifted during the port: a gate changed in Python and not in Go | Historical parity went red | The migration-time parity gate caught drift before the clean cutover. `TestNoPythonLeRemains` now prevents a second producer from returning |
| R-3 | `le` and `ze` diverge in CLI behaviour despite the shared registry | A pipe operator works on a `ze` command and not an `le` one | The wiring test drives a pipe operator through an `le` command |
| R-4 | Stale binary: `le` is built, so an old binary can run an old gate | A gate passes on code that should fail it | The repository launcher owns the build-before-dispatch decision; no Make prerequisite or compatibility route participates |
| R-5 | A producer is migrated but omitted from the native action surface | Its package exists without a reachable area/action pair | `TestEveryMakeProducerHasAReachableNativeAction` enumerates producers and native actions |
| R-6 | A retired Make identity survives as a Go compatibility verb | An action table contains a historical `ze-*` target spelling instead of its area-local verb | Action tables declare native verbs such as `./le test-unit bgp`; no Make target namespace is retained |
| R-7 | A tool's output is prose, and making it structured changes what operators read | A ported tool's output diff is large in step 2 | Treat output shape as behaviour: the parity test diffs it, and a deliberate change needs its own row here. **Step 2 measured the diff at THREE deliberate changes and nothing else** (2026-08-26): the palette, the walk error, and the ordering, each set out under "What A-6 Measured". **Step 4 measured the retired `scripts/vendor` (current producer: `internal/le/`) at TWO, and stdout is byte-identical on both halves** (2026-08-26): the failure PREFIX on stderr (`check_web: ` and `sync_web: ` name files the swap deletes, so the port writes `error: ` like every other ported tool), and the sync's fail-open, which is the defect row in `plan/journal/guard-added-to-one-half-of-a-pair.md` and is fixed in the direction of the fix. the retired `scripts/vendor/parity_test.go` (current producer: `internal/le/vendorweb/`) normalizes the first and pins the second. **Step 7 measured the retired `scripts/codegen` (current producer: `internal/le/`) at THREE, all of them normalized rather than argued** (2026-08-26): the failure prefix (four script names become `error: `), the PATH FORM (the scripts print an absolute path for one file and a bare base name for another; the commands name every file relative to the tree, because one payload cannot answer two path forms without leaving `\| json` and the default rendering disagreeing about what the value is), and the STREAM a verdict lands on (three of the four scripts print staleness to stderr because each models it as an error; a verdict is the payload in a command, so it reaches stdout where `\| json` can carry it, and only a genuine failure reaches stderr). **Step 10 measured the retired `scripts/evidence` (current producer: `internal/le/`) at ONE, and it is not about text at all** (2026-08-26): a proof that drives a real peer prints a progress LOG rather than a report, so what the port had to decide is which stream the log goes to. It goes to stderr, and the report is the payload on stdout, which is what lets `le deployment l2tp-test \| json` answer one document while the two daemons are still talking. Both ported scripts already wrote their progress to stderr, so the only visible change is the one line each printed on stdout at the end. **Step 13a measured the retired `scripts/dev/ze-run.sh` (current producer: `internal/le/job/job.go`) at TWO, and neither touches what a wrapped job prints** (2026-08-26): the banners carry ANSI sequences only when stderr is a terminal, because a follower REPLAYS a holder's log and the script's unconditional escapes would land inside it; and the command's payload renders as nothing by default, so a wrapped recipe's stdout stays the child's output alone while `\| json` still answers the report. The second is a `leroot.Prose` implementation returning the empty string, which is the one shape that keeps a structured payload and adds no line. **Step 10b measured the two VPP evidence gates at SIX, four of them fail-closed repairs and two of them cosmetic** (2026-08-26): a FAILED vppctl no longer answers "the plugin is absent" and no longer contributes evidence to a probe, both journal rows; a wanted kernel argument must be a whole argument of the command line rather than a substring of a longer one, also a journal row; a size that is not a whole number of bytes is refused, where the Python's `int()` answered a negative page count for `-1gb` and ended the run in a traceback for `1.5gb`; the LCP scenario's Linux link listing moves from stderr into the payload's evidence, because it is the half of that proof VPP's own command line cannot show and an operator could not pipe it where it was; and the hugepage run narrates its three long steps on stderr, where the Python was silent for up to an hour |
| R-8 | **The never-linked rule erodes and something quietly imports across the line.** It is the premise the shared engine rests on, and nothing about Go stops a developer adding one blank import. The day it happens, `ze` grows a dev-tool dependency, or `le` grows a product one, and the two plugin sets start sharing a binary | AC-2 or AC-3 goes red: a `le/` package appears in `ze`'s dependency list, or a product plugin in `le`'s | The invariant is CHECKED, not documented. `go list -deps` over each build flavour, in `ze-precommit-verify` from step 1. It discriminates: measured 2026-08-26, `internal/perf/cli` is absent from `ze`'s 630-package list with `ze_perf` off and present with it on, so the check can see the difference it exists to see |
| R-9 | The shared engine is shared in NAME only: `le` accretes its own grammar, its own pipe handling, its own help, and the two drift into similar-looking programs with nothing in common | A `le` package under `le/` starts declaring what `internal/component/command` already provides | AC-3b pins that both binaries link the one registry and neither declares its own. The engine is the thing this spec exists to share, so a second implementation of any part of it is the failure, not a convenience |

## Blast Radius

| Surface | Effect |
|---------|--------|
| retired `scripts/` tree | Historical producer inventory: 11 directories and 4507 references. Current producers live under `internal/le/`, `internal/test/`, or `internal/appliance/` |
| retired `mk/*.mk` and Makefile | Historical routing inventory only. Current commands use native area-local actions and keep no target-name compatibility layer |
| hook producers | Native hook dispatch and checks live under `internal/le/hookruntime/` and `internal/le/hookcheck/` |
| `ai/INSTRUCTIONS.md`, `ai/rules/`, `ai/INDEX.md` | Current instructions name native commands and Go producers |
| CI workflows | Dispatch native `./le <area> <verb>` actions |
| `go.mod` | Tool imports are ordinary module imports |

Getting out: until step 10 nothing routes to Go, so abandoning the work costs
only the unreferenced the former standalone composition tree. After step 10 the exit is a revert of one
changeover commit.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le <name>` typed by a developer | → | the root handler registered by `internal/le/<tool>/register.go` | `TestStandaloneLeAndZeLeHaveIdenticalSurface` |
| a `ze` build of any flavour | → | its dependency list, which must name no `le/` package | `TestNormalZeLinksNoInternalLe` |
| a `cmd/ze` build | → | its composition root, which registers only `le/` packages. Linking the product for introspection is allowed and required | `TestLeRegistersOneRootAndNoToolRoots` |
| either binary's dispatch | → | the one `internal/component/command/registry`, with no second registry declared | `TestStandaloneLeAndZeLeHaveIdenticalSurface` |
| `./le <name> \| json` | → | the tool's structured payload through `internal/component/command` pipe filters | `TestDispatchUsesSharedPipeRenderers` |
| tab after `./le ` | → | the command tree built from the registry | `TestStandaloneLeAndZeLeHaveIdenticalSurface` |
| every producer invoked as `./le <area> <verb>` | → | the named action in its `internal/le/<area>/actions.go` table | `TestEveryMakeProducerHasAReachableNativeAction` |
| `go build ./...` over a `git archive HEAD` export | → | every registered tool package | `TestTheCheckoutsOwnHeadIsClean` |
| binary init, importing both composition roots | → | `registry.RegisterRootHandler`'s duplicate rejection | `TestNoLeNameCollidesWithZe` |
| `./le <name>` from a shell, against the BUILT binary | → | the registered handler, end to end through the artifact a developer runs | `test/ui/le-binary-dispatches.ci` |

## Acceptance Criteria

| # | Criterion | Test |
|---|-----------|------|
| AC-1 | `cmd/ze/` produces a binary dispatching every ported tool through `internal/component/command/registry` | `TestStandaloneLeAndZeLeHaveIdenticalSurface` |
| AC-2 | A normal `ze` build links no `internal/le/` package | `TestNormalZeLinksNoInternalLe` |
| AC-3 | `le` MAY link the product composition root for introspection. It MUST register no product command as its own. `internal/le/register.go` imports only `internal/le/` packages | `TestLeRegistersOneRootAndNoToolRoots` |
| AC-3b | Both binaries dispatch through the SAME engine: each links `internal/component/command/registry`, and neither declares a registry of its own | `TestStandaloneLeAndZeLeHaveIdenticalSurface` |
| AC-4 | No ported tool file carries `//go:build ignore`; all are built by `go build ./...` and seen by `go vet` | `TestNoDevelopmentToolIsBuildIgnored` |
| AC-5 | Every ported tool's test calls it as a function; no test invokes it via `go run` | `TestNoDevelopmentToolTestShellsOutToGoRun` |
| AC-6 | Every first-party producer has one reachable native `./le <area> <verb>` action. No Go action preserves a retired Make target identity as a compatibility verb | `TestEveryMakeProducerHasAReachableNativeAction` |
| AC-7 | Each ported tool answers structured data, so `\| json`, `\| yaml` and `\| table` render it | `TestDispatchUsesSharedPipeRenderers` |
| AC-8 | A gate failure propagates its own exit code, never a flattened 1 | `TestFirstFailingGateExitCodeWins` |
| AC-9 | Native action completeness is derived from current producer and action tables. The final tree has no Python parity census, retired producer, or unported count | `TestEveryMakeProducerHasAReachableNativeAction` and `TestNoPythonLeRemains` |
| AC-10 | The committed tree builds and every registered tool loads from it | `TestTheCheckoutsOwnHeadIsClean` |
| AC-11 | Each migrated producer's observable contract is defended by native unit or functional tests; no test depends on a deleted implementation | Per-package tests and the affected functional suites |
| AC-12 | The final tree has no first-party Python, shell, or Make source, embedded interpreted helper, `//go:build ignore` tool, or retired tooling tree | `TestNoPythonLeRemains` |
| AC-13 | A duplicate root name is rejected at init rather than shadowing | `TestDuplicateLeRootIsRejected` |
| AC-14 | Every development-tool package lives under `internal/le/`; the final tree contains no former top-level tool path or import | `staleLeReferences` in `internal/le/contract_test.go`, called from the corpus walk in the same file |
| AC-15 | The standalone `le` binary is a `cmd/ze` personality, as `ze-test` is. A normal `ze` build links no `internal/le/` package | `TestStandaloneLeAndZeLeHaveIdenticalSurface` and `TestNormalZeLinksNoInternalLe` |
| AC-16 | A `ze_le` build exposes `ze le`. Standalone `le` and `ze le` list and dispatch the exact same command surface, including structured pipe output | `TestStandaloneLeAndZeLeHaveIdenticalSurface` |

## End-to-End User Stories

1. A developer runs `./le changed scope`, which dispatches directly to compiled Go.
2. A developer runs `./le doc check verify | json` and gets a machine-readable report through the shared renderer.
3. A developer adds a new check as one package under `internal/le/`, a `register.go`, and an action-table entry. It appears in help and completion with no compatibility target.
4. A developer builds the `le` personality from `cmd/ze` and runs `le <area> <verb>`.
5. A developer builds `ze` with `ze_le` and runs the same command as `ze le <area> <verb>`. Both forms list and dispatch the same tools.
6. A normal `ze` build contains no `internal/le/` package and no `le` command.
7. A reviewer checks native action coverage, and `TestEveryMakeProducerHasAReachableNativeAction` proves that every producer derived from the historical Make text has a reachable native action.

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRunReturnsToolExitCode` | `internal/le/<tool>/<tool>_test.go` | each ported tool's logic, called as a function | |
| `TestCompositionEqualsLiveRegisteringPackagePopulation` | `internal/le/register_test.go` | one handler per package, Meta carries Description, Mode and Section | |
| `TestDuplicateLeRootIsRejected` | `internal/le/register_test.go` | a duplicate root name panics at init rather than shadowing | |
| `TestEveryMakeProducerHasAReachableNativeAction` | `internal/le/completeness_test.go` | every current producer has one named native area/action route | |
| `TestFirstFailingGateExitCodeWins` | `internal/le/leroot/dispatch_test.go` | the failing gate's own code propagates, never a flattened 1 | |
| `TestNoDevelopmentToolIsBuildIgnored` | `internal/le/contract_test.go` | no ported file carries `//go:build ignore` | |
| `TestNoDevelopmentToolTestShellsOutToGoRun` | `internal/le/contract_test.go` | no test invokes a tool via `go run` | |
| `TestNoPythonLeRemains` | `internal/le/contract_test.go` | the retired Python producer tree and its references cannot return | |

### Boundary Tests (numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| gate exit code propagated | 0-125 | 125 | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestStandaloneLeAndZeLeHaveIdenticalSurface` | `internal/le/leroot/dispatch_test.go` | a developer runs `./le <name>` for every registered tool and reaches it | |
| `TestNormalZeLinksNoInternalLe` | `cmd/ze/ze_le_personality_test.go` | `go list -deps` over every `ze` flavour names no `le/` package | |
| `TestLeRegistersOneRootAndNoToolRoots` | `cmd/ze/ze_le_personality_test.go` | `internal/le/register.go` names no product command package; every `le` root handler comes from a `le/` package | |
| `TestStandaloneLeAndZeLeHaveIdenticalSurface` | `cmd/ze/ze_le_personality_test.go` | both link `internal/component/command/registry`, and neither declares a second one | |
| `TestDispatchUsesSharedPipeRenderers` | `internal/le/leroot/dispatch_test.go` | `./le <name> \| json` renders without per-tool JSON code | |
| `TestStandaloneLeAndZeLeHaveIdenticalSurface` | `cmd/ze/ze_le_personality_test.go` | tab completion offers the tool | |
| `TestEveryMakeProducerHasAReachableNativeAction` | `internal/le/completeness_test.go` | every current producer is reachable without a Make compatibility identity | |
| `TestTheCheckoutsOwnHeadIsClean` | `internal/le/tracked/tracked_test.go` | the committed tree builds and every tool loads from it | |
| per-area native behavior test | `internal/le/<area>/` | native exit codes, payloads, and side effects match the area contract |
| `le-binary-dispatches` | `test/ui/le-binary-dispatches.ci` | the BUILT `le` binary dispatches a registered command and renders `\| json` | |

The `.ci` is not a formality, and it is not a daemon test. Every Go test above
exercises a package; none exercises the artifact a developer actually runs.
Between them sit the build, the composition root and argv handling, which is
where a stale or mis-composed binary fails (R-4). The native functional runner
executes `./le ...` directly, so the fixture needs no Python or shell helper.

### Interop Tests (Scope: protocol)

N/A - no wire-visible behaviour changes.

**Discrimination requirement.** Every test above must be shown to FAIL when the
behaviour it pins is broken, by mutation, with the mutation recorded in the
commit (`ai/rules/interop-and-goal-validation.md`). A parity test that passes
because both sides are broken the same way proves nothing, and that is the
specific trap of a duplicate-then-swap migration.

## Files to Modify

- `internal/le/register.go` and each area's `actions.go` - native composition and action verbs
- `cmd/ze/register.go` - blank imports for any development tool wanted in a `ze_le` build <!-- doc-links: ignore (file this open spec plans and has not created yet) -->
- `ai/INDEX.md` - discovery rows for `le` command areas
- `ai/INSTRUCTIONS.md`, `ai/rules/commands.md` - native gate invocation
- `internal/le/hookruntime/`, `internal/le/hookcheck/` - native hook producers
- `docs/architecture/core-design.md` - the shared registry and two composition roots

## Files to Create

- `cmd/ze/` - entry point
- `internal/le/register.go` - composition root: blank imports
- `internal/le/<tool>/<tool>.go` - one package per tool, exposing `Answer(args []string) (any, int)`
- `internal/le/<tool>/register.go` - `init()` calling `leroot.Register`, which calls `registry.MustRegisterRootHandler`
- `internal/le/completeness_test.go` - producer-to-native-action completeness check

## Implementation Steps

The table below preserves the sequence and measurements used during the
duplicate-then-swap migration. It is not an implementation queue. Current
closure work is the native completeness and residue proof in AC-6, AC-9 and
AC-12.

The historical scope was 280 code files: 187 Python, 79 Go and 14 shell files,
counted on 2026-08-26.

| # | Step | PY | GO | SH | Refs |
|---|------|---:|---:|---:|-----:|
| 1 | the former standalone composition skeleton, registration contract, wiring tests, A-5 check, A-1 build probe, `le parity` at 0 of 156. NO tool ported. The contract tests must be red for the right reason before anything moves | - | - | - | - |
| 2 | the retired `scripts/lint` (current producer: `internal/le/`) -- first end-to-end port, its test, its parity proof, and the measurement A-6 needs | - | 2 | - | 8 |
| 3 | the retired `scripts/inventory` (current producer: `internal/le/`) | - | 3 | - | 24 |
| 4 | the retired `scripts/vendor` (current producer: `internal/le/`) | - | 4 | - | 26 |
| 5 | the retired `scripts/docvalid` (current producer: `internal/le/`) | - | 3 | - | 78 |
| 6 | the retired `scripts/status` (current producer: `internal/le/`) | - | 9 | - | 271 |
| 7 | the retired `scripts/codegen` (current producer: `internal/le/`) | - | 8 | - | 491 |
| 8 | the retired `scripts/checks` (current producer: `internal/le/`) | - | 30 | - | 369 |
| 9 | the retired `scripts/zeledon` (current producer: `internal/le/`) | 2 | - | - | 26 |
| 10 | the retired `scripts/evidence` (current producer: `internal/le/`) | 19 | 4 | 2 | 300 |
| 11 | The 22 Python `le` areas (the retired `scripts/le` (current producer: `internal/le/`)) | 59 | - | - | 157 |
| 12 | the retired `scripts/dev` (current producer: `internal/le/`), Python and Go halves | 107 | 16 | - | 2757 |
| 13 | The 12 shell scripts in the retired `scripts/dev` (current producer: `internal/le/`), last of the ports and the highest risk. `ze-run.sh` is the job-admission wrapper every heavy command routes through; `session-scratch.sh`, `spec-session.sh`, `verify-status.sh` and `verify-lock.sh` are named by `CLAUDE.md`, by the hooks, and by every session's habits. Converting these changes an interface every agent uses, not merely an implementation | - | - | 12 | - |
| 14 | THE SWAP. The original plan preserved every Make target, but the later owner amendment rejected that compatibility model. The completed cutover deleted the shims and proves every producer through a native area-local action instead | - | - | - | - |
| 15 | `ze le <command>`: the crossing, under a `ze_le` build tag. One root, sub-dispatched into le's own commands. Independent of the swap and of every port, so it can land at any point | - | - | - | - |

→ Decision (2026-08-26): step 15 makes goal 1's claim COMPILED. `ze` and `le`
share one engine, and the test of that is whether a crossing is POSSIBLE, so a
crossing nobody ever builds is a claim nobody has checked.
The `ze_le` companion in `cmd/ze` blank-imported the `zele` package from the
former top-level tool tree. That package registered root `le` and called
`leroot.Dispatch`, which had moved from the former standalone composition.
`TestZeLinksNoLePlugin` checked that a normal build linked no package from that
tree. `TestZeWithTheLeTagLinksLesTools` and
`TestZeWithTheLeTagRunsLesCommands` checked the tagged crossing.

→ Constraint: linking a tool package is what runs its `init()`, so in a `ze_le`
build each tool ALSO claims its own root name: `ze lint` resolves beside
`ze le lint`. Only a second dispatch table under the former top-level tool tree could prevent that,
and a second table is the failure AC-3b exists to refuse. A name later shared by
a `ze` root and an `le` tool therefore panics that build at init, loudly, which
is what A-5 already says such a build owes.

→ Constraint: `ze_le` is not in `feature-gates.txt` and MUST NOT be added. That
manifest declares compile-out-able product features and every consumer derives
default-on tag sets from it, so a row there would carry the crossing into
shipped builds. It does need a lint flavor, because a tag no pass compiles
leaves its file unlinted (the retired `scripts/dev/lint_flavors.py` (current producer: `internal/le/verify/lint/matrix.go`), the capability row).

→ Owner amendment (2026-08-26): the final package home is `internal/le/`, not
the former top-level tool tree. Every path and import from that tree above is transitional and is removed
before the swap. The move is one clean cutover with no alias package, forwarding
import, or compatibility path.

→ Owner amendment (2026-08-26): standalone `le` is a `cmd/ze` build personality,
the same architecture used by `ze-test`; the final tree has no separate
standalone composition. A normal `ze` build MUST NOT link `internal/le/`. A build with the
non-default `ze_le` tag exposes `ze le`, and its command inventory, dispatch,
exit codes, and structured answers MUST be identical to standalone `le`.

→ Constraint: a shell script is not automatically a `le` command. the retired `mk/test-fuzz.mk` (current producer: `internal/le/fuzz/actions.go`)
records the shape of the exception: the admission wrapper "STAYS HERE, and it
cannot move ... That is a Make-level concern about Make's own concurrency, and
`le` is not the right owner for it." Step 13 must say, per script, whether it
becomes a command, stays a thin wrapper the hooks keep calling, or does not move
-- and record the reason the way that header does.

→ Decision: steps 2 to 10 touch disjoint directories and may run in parallel
once step 1 exists. Steps 11 to 14 are sequential.

→ Decision: step 1 delivers no user-visible value and is not optional. A
migration that starts by porting a tool has no contract to port it into, and no
way to say how far along it is.

→ Constraint: the parity count must fall monotonically. A step that ports a tool
and leaves the count unchanged has not wired it.

→ Constraint: the swap is ONE changeover, not a trickle. A shim repointed early
puts a developer on a half-ported path, which is what this strategy avoids.

## What step 13a decided (the retired `scripts/dev/ze-run.sh` (current producer: `internal/le/job/job.go`))

The constraint above asks step 13 to say, per script, what becomes of it. For
the admission wrapper the answer is both a library and a command, and the
library is the half the rest of the migration depends on.

| Question | Answer |
|----------|--------|
| What it becomes | `internal/le/job`, plus `le job run label <label> command <argv...>` |
| What a GATE uses | `Admission.Run`, which is the whole wrapper: admission, the child, its log, the release |
| What a LAUNCHER uses | the same `Run`, for the build alone. It holds a slot only while the build runs, so the binary it then execs is outside the slot. `Admit` and `Ticket.Release` are the lower level, for a caller that does the work in-process |
| What the registry becomes | unchanged. Same directory, same file names, same fields, same work key, same tree hash. Both halves therefore admit each other's jobs while the migration runs, which is also what makes AC-11 provable at all |
| What the retired `mk/test-fuzz.mk` (current producer: `internal/le/fuzz/actions.go`) records | superseded, and left alone. Its header says the wrapper "cannot move ... a Make-level concern about Make's own concurrency". Removing make removes that premise, and the comment goes with the recipe it heads, at step 14 |

→ Constraint recorded for step 14: the launcher had to bootstrap its own
admission. `./ze` could use `bin/le`, while `./le` could not use that absent
binary to admit its build. The selected historical route used `go run` on the
former standalone composition to admit a `go build` of the same composition.
The first compile warmed the build cache without admission. The admitted build
then wrote `bin/le`. The measurement was 2411 ms for the tool tree. The
630-package `ze` build remained admitted in either route.

## What A-6 Measured (step 2, 2026-08-26)

A-6 asked whether a tool that prints a prose report can answer structured data
without the port becoming a rewrite. the retired `scripts/lint/consistency.go` (current producer: `internal/le/consistency/consistency.go`) was chosen
because it is the worst case in the step table: 452 lines that print a colored
severity report grouped by check, with a summary line.

**It is not a rewrite. The measured cost is below, and steps 6, 7 and 8 can be
scheduled as ports.**

| Cost | Size | Paid once or per tool |
|------|------|----------------------|
| The `leroot.Prose` seam: a payload may render itself, and does so only when the operator typed no pipe chain | 12 lines in `internal/le/leroot/leroot.go` plus its doc comment | ONCE, for every tool after this one |
| The payload declaration: `Finding` and `Report` in `internal/le/consistency/report.go` | 35 lines | per tool |
| `Report.Text`, which is the script's printing block with `fmt.Printf` replaced by a `textbuf` chain | 40 lines, against the script's 40 | per tool, and it is a transcription rather than a design |
| `command.RegisterShape` in `register.go` | 1 line | per tool |
| Lint remediation, which is what A-1 predicted | 4 findings, all mechanical | per tool |

**What made it cheap is that the report already HAD a record type.** The script
appended `finding{severity, category, file, line, message}` to a package-level
slice and rendered it at the end, so the payload was already written; it was
being thrown away at the last step. A tool that builds its text as it goes will
cost more, and the retired `scripts/status` (current producer: `internal/le/`) is where to expect that.

**That last sentence is wrong, and step 6 measured why (2026-08-26).** Both
tools in the retired `scripts/status` (current producer: `internal/le/`) were read, and neither cost what it predicted.

`spec_status.go` builds its text as it goes in the literal sense: `printTable`
and `printBucketSection` write to stdout with a format verb, and no record type
for the PAGE exists anywhere. The port was still cheap. Its `Text()` is 60 lines
against the script's 55, because the page derives entirely from the same
`[]spec` the `--json` mode already carried. `verify_run.go` is the opposite
shape and also not what was predicted: it already declares `verifyIndex`,
`stageResult` and `failureGroup`, and `writeFailureArtifacts` already marshals
them, so its payload is written too.

**The predictor is not "does it build text as it goes". It is "does the tool
already have to answer a machine".** A tool with a `--json` mode has a record
type by construction, whatever its printing code looks like, and the port then
costs a transcription. the retired `scripts/lint` (current producer: `internal/le/`), the retired `scripts/inventory` (current producer: `internal/le/`), the retired `scripts/vendor` (current producer: `internal/le/`) and
both halves of the retired `scripts/status` (current producer: `internal/le/`) all have one, which is five for five. The cost
to expect for the rest of the migration is therefore SIZE and lint remediation,
not payload design. For `verify_run.go` it is size alone: at 1849 lines it is
already an ERROR of `ze-consistency-check`'s 1000-line rule, so the port cannot
be one file.

**One more cost that appears only when a package is FOLDED.** `spec_status.go`
kept `specbucket` and `specmeta` as separate packages for one stated reason: a
`//go:build ignore` front end cannot be imported by a test. A compiled package
has no such problem, so the three became one, and `goconst` then saw three
copies of four status literals that had been one copy per file. Four findings,
mechanical, and the repair (naming the statuses) turns a typo in any of the
three enumerations into a compile error. Expect this wherever a tool kept a leaf
package to work around its own build tag.

**Three behaviour changes are deliberate, and each is asserted rather than
compared** (`test/ui/le-consistency-answers.ci`, the retired `scripts/lint/parity_test.go` (current producer: `internal/le/consistency/`)):

| Change | Why it is not optional |
|--------|------------------------|
| The palette. The script writes `\033[31m`; the command writes the semantic roles (`docs/architecture/cli/color-system.md`) | `c_raw_ansi` (`.claude/hooks/pretool-writeedit.py` (retired; now `internal/le/hookruntime/writeedit.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) -->) refuses a raw escape in any Go file that is not `textbuf.go`, `helpfmt.go` or a test. A compiled Ze package cannot spell the script's colors, so BYTE-identical output was never reachable. The findings are identical; both sides are stripped before comparison |
| A tree the tool cannot read is now REPORTED. The script's `walkGoFiles` returns nil on the walk error, so `consistency.go /nonexistent` prints "All consistency checks passed" and exits 0 | A gate that passes over a tree it never read is the failure this gate exists to prevent, applied to itself. `TestPortReportsWhatTheScriptFailsOpenOn` pins the difference in the direction of the fix |
| The cross-reference findings are SORTED. The script iterates a map, so two runs over one tree answer 1250 lines in different orders | An output that shuffles cannot be diffed, reviewed or ratcheted. `TestCheckIsDeterministic` pins it, and the parity comparison is over the SET of lines because the script's own order is not stable |

**One vacuity worth carrying forward.** A mutation of the `snake_case JSON tag`
message survived the `.ci` comparison over the real checkout, because this tree
draws zero `json-kebab-case` findings: the real tree exercises only the checks
that currently fire on it. The fixture table in the retired `scripts/lint/parity_test.go` (current producer: `internal/le/consistency/`)
kills that mutation, and a mutation of a message the tree DOES produce
(`hardcoded Status:`) kills the `.ci`. Both are needed, and a port that keeps
only the real-tree comparison is testing less than it looks.

## Design Insights

The measured 180 seconds is NOT the argument for this change and must not be
cited as one. It comes from `go run` relinking and could be recovered by
compiling the existing scripts into one multi-tool binary with `le` left in
Python. The argument is the shared REGISTRY: two registries kept in agreement by
hand can drift, and Python cannot join Ze's. Speed is a consequence.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| the former standalone composition as its own binary | a `ze_le` build tag inside `cmd/ze/`, as `ze-perf` and `ze-chaos` do | The separation is USE. A distinct binary says so; a tag hides it behind a build flag |
| One shared registry | a second registry for dev tools, with a drift check between them | The whole point: a feature crosses by adding an import. A drift check is a worse version of not being able to drift |
| `le/` as a top-level tree, NOT `internal/le/` | `internal/le/`, mirroring `internal/perf/`; or `tools/le/` <!-- doc-links: ignore (a layout this spec considered and rejected, so it was never created) --> | Historical owner directive, 2026-08-26: `le`'s plugins were treated as separate from `ze`'s. The retired `make ze-tier-check` target governed `internal/`; its current replacement is `./le tier check`, and the final package home is `internal/le/` |
| No per-tool `cmd/` directories | one small binary per tool, mirroring Python's module route | In Python the module route exists because there is no build step. In Go, `le <name>` IS that route. Per-tool binaries would be artifacts nobody consumes |
| No `dev-gates.txt` build-tag manifest | mirroring `feature-gates.txt` in full | Ze gates features because the appliance SHIPS and size matters. `le` never ships, so the tags have no consumer today. The structure leaves it a one-line addition later |
| Duplicate-then-swap | delete each Python original as its Go replacement lands, per `no-layering.md` | Owner directive, 2026-08-26. The migration-time parity gate made drift loud. The clean cutover replaced that census with native producer/action completeness |

## Known Limitations

- The 8007 lines of Python and 4011 lines of Python tests are sunk. Test CONTENT transfers as intent; the code does not.
- `le` gains a build step, so a stale binary can run an old gate. Python could not fail this way. R-4 mitigates it; it does not remove it.
- Build-tag gating of `le` features is deliberately not built. If a slim `le` is ever wanted, `dev-gates.txt` is the shape to add.
- Between step 1 and the swap the repository carries two implementations of the same tooling. That is the accepted cost of the chosen strategy.
- **The former top-level tool tree could not use the name `le`, because that name was already an executable file.** The repository root held the Python shim `./le`, and a directory could not share that name. The swap did not free it because `./le` stayed a file afterwards.
- **`le`'s engine footprint was 15 Ze packages when measured from the former standalone composition on 2026-08-26.** The 12 `internal/` packages this section names and `pkg/zefs`, `pkg/plugin/rpc`, and `pkg/ze` were reached through `internal/component/config/storage` and `internal/component/plugin/registry`. The binary linked 281 packages in total against `ze`'s 630, and none was a product plugin.
- **A tool directory holding SEVERAL gates becomes ONE command with several actions, not several commands (step 4, 2026-08-26).** the retired `scripts/vendor` (current producer: `internal/le/`) is the first of those: two programs, three gates. Three root commands was the first shape tried and `TestEveryPackageRegistersOneRootHandler` refused it, which is the contract working -- one package, one import, one root. The answer is the shape the Python `le` already had: an AREA holding GATES. `vendorweb` in the former top-level tool tree registers `vendor-web` and its three actions are `check`, `sync` and `update-report`, each verb derived from its Make target the way `Gate.short` derived it (the retired `scripts/le/devtools/gate.py` (current producer: `internal/le/`)). `Gate.writes` travels with them: the bare `le vendor-web` prints the listing `le <area> --list` printed, marker included, and `Meta.SubsFunc` derives help's one-line hint from the same table so the two cannot disagree.
- **The `--root DIR` flag is NOT ported, and `ZE_REPO_ROOT` is what replaces it (step 4, 2026-08-26).** Nothing but the two scripts' own tests ever passed it: the retired `scripts/le/application/generate.py` (current producer: `internal/le/`) invokes all three gates with no argument. `lepath.Root()` already honours `ZE_REPO_ROOT`, which covers the one operator case a flag covered, and a path positional would break the keyword-before-value grammar. The exported `Check(root, updates)` and `Sync(root)` still take the tree, so a test names a fixture by calling the function rather than by typing a flag.
- **`le` OWNS a command; it does not merely hold one in its registry, and dispatch reads ownership rather than the registry (step 3, 2026-08-26).** The first tool that had to link `internal/component/plugin/all` made AC-3's second half unreadable as written. Linking the product runs the product's `init()`s, and five of them register root commands of their own: `env`, `interface`, `plugin`, `schema` and `sysctl`, measured 2026-08-26 with and without the full feature tag set (the count is the same either way, because `plugin/all` imports the plugin packages and not the `cli` dispatch companions). So "every root handler in this process is a package from the former top-level tool tree" stopped being true the moment a tool did what AC-3 explicitly permits. The fix is one list: `leroot.Register` records the names it registered, `leroot.Owns` answers them, and `dispatch.go` in the former standalone composition asks it before it looks a name up, so `le interface` is an unknown command rather than `ze`'s interface editor. The registry stays the ONE owner of a name, so AC-13's duplicate panic is untouched. Three tests read the new line: `TestLeDispatchesNoProductCommand` drives every unowned root through dispatch, `TestLeOwnsWhatItRegisters` pins the list against the registry, and `TestParityCountsOnlyLeCommands` pins the census, which counted the five product roots as ported Go commands until it read the same list.
- **`internal/` conflates the engine with `ze`'s product code, and this spec does not fix it.** Measured 2026-08-26: the engine `le` needs is 12 packages (`component/command`, `component/command/registry`, `component/config/storage`, `component/plugin/registry`, and `core/env`, `core/envcatalog`, `core/helpfmt`, `core/metrics`, `core/selector`, `core/slogutil`, `core/stringsx`, `core/textbuf`). The rest is one program's product: 43 directories under `internal/component/` and 64 under `internal/plugins/`. So `internal/` is not "shared", it is "`ze`, plus a small engine nobody has named". Putting `le` at top level rather than at `internal/le/` was chosen partly for this reason: `internal/le/` would sit `le`'s plugins inside the tree they must never be linked with, while a top-level tree makes the rule readable at a glance. The symmetric alternative -- `internal/ze/` <!-- doc-links: ignore (a split this spec names and does not make, so the tree has no such directory) --> beside `internal/le/`, engine left in `internal/` -- was measured at **32,419 references** (24,037 for `internal/component/` and 8,382 for `internal/plugins/`), larger than this entire migration and touching every product file rather than the tooling. It is a real improvement and it needs its own spec, sequenced as: name and separate the 12-package engine first, then `internal/ze/` <!-- doc-links: ignore (a split this spec names and does not make, so the tree has no such directory) -->, then `le` could move symmetrically. The first of those three is useful on its own, because it would let `go list -deps` prove the ENGINE boundary the way AC-2 proves that `ze` links no dev tool (owner decision, 2026-08-26: option A, a top-level tree; step 1 spelled it the former top-level tool tree for the filename reason above).
- **A Python tool's seams are the process boundary, and that is what AC-11 is proven over (step 9, 2026-08-26).** A Go port and a Python script share no process, so no test can call both. They do share four things: the processes the tool execs, the files it writes, its stdout, and its exit code. the retired `scripts/zeledon/parity_test.go` (current producer: `internal/le/weekly/`) points BOTH implementations at one recording stand-in for `discord.sh` and at a temporary archive, so what is compared is the argv that would have reached a public channel rather than two reports written in each tool's own words. That is stronger than the stdout diff step 2 used: over the 37 real posts in `website/changes/posts/` the two send byte-identical messages in the same order, and a `charLen`-counts-bytes mutation is caught by that comparison alone, at `2026-07-20.md`, 9 messages against 10. The report text is deliberately NOT compared, because the command's plan names its own grammar and telling an `le` operator to pass `--yes` would be a defect rather than parity.
- **Four numbers cannot be seen from either tool's output, so they are compared directly (step 9, 2026-08-26).** The message limit, the stale window, the gap between posts, and the retry schedule. A limit one character lower splits every post in this corpus exactly the same way, so an output comparison is blind to it: that mutation SURVIVED the whole suite until `TestScriptAndCommandShareTheSameNumbers` read `LIMIT`, `STALE_AFTER_DAYS`, `SEND_DELAY` and `RATE_LIMIT_BACKOFF` out of the script's own module and set them against the package's. A later port whose behaviour is governed by a constant owes the same test.
- **A Python `--yes`-style confirmation becomes a KEYWORD, never a subcommand (step 9, 2026-08-26).** `le weekly` plans and `le weekly confirm` publishes, so the tool keeps ONE action and `switch args[0]` never appears, which is what `c_switch_dispatch` and `ai/rules/plugins.md` require. The alternative shape, `plan` and `post` as two subcommands, needs a dispatch table, and `internal/core/subdispatch` cannot serve one here: its handlers answer `int` and render themselves, which is what AC-7 forbids, and its help page spells `ze`. Step 4 met the same wall for a tool that genuinely has three actions, and answered it with a package-local action table (`vendorweb` in the former top-level tool tree, `actions`). Two tools, two shapes, and neither is shared yet: the THIRD tool that needs sub-actions should lift that table into `leroot` in the former top-level tool tree rather than write a third.
- **the retired `scripts/zeledon` (current producer: `internal/le/`) moves NO number in the census, and that is a property of the directory rather than of the port (step 9, 2026-08-26).** `the retired le parity census` declares 156 gates and none of them is this tool: it is an operator tool the `ze-weekly-update` skill runs by name, reached by no Make target. So `weekly` in the former top-level tool tree calls no `parity.Claim`, `unported` stays where it was, and `script-files` falls only when step 14 deletes the two `.py` files, which is true of every step because every ported script stays until the swap. The step table's monotonic-count constraint is about GATES, and it cannot bind a directory that declares none.
- **The published-week archive stays at the retired `scripts/zeledon/weekly/` (current producer: `internal/le/weekly/`), and step 14 owes it a new home (step 9, 2026-08-26).** That directory's contents are what mark a week as already published, so moving it while both implementations can run is what would publish a week twice. `weekly.ArchiveDirRel` in the former top-level tool tree names it and says so. Step 14 also owes `pythonTestRoots` (the retired `scripts/dev/python_tests_test.go` (current producer: `internal/le/`)) the removal of its the retired `scripts/zeledon` (current producer: `internal/le/`) entry: each root asserts that it contributes at least one test file, so deleting the script without it turns that assertion red.
- **`le` MUST be compiled with the full feature tag set, and step 14 owes that to whatever builds it (step 5, 2026-08-26).** Three tools now read the live registry, and a tag set short of the manifest compiles the address families and the BGP command handlers out. Measured: an untagged `le docvalid doc-drift-check` reports 11 findings against documentation that is correct, one per family the build no longer carries, because `registryFamilyNames` (`docvalid/drift.go` in the former top-level tool tree) answers 6 rather than 23. This is the trap `doc_drift_warnings` (the retired `scripts/dev/commit_helper.py` (current producer: `internal/le/commit/prepare.go`)) already documents for the script, met again on the binary. `test/ui/le-docvalid-answers.ci` derives the set from `feature-gates.txt` for all three binaries it builds.
- **Three gates over two scripts became ONE command with three actions, named after the DIRECTORY (step 5, 2026-08-26).** The three gates share no prefix -- `ze-command-contract-check`, `ze-doc-drift-check`, `ze-docs-pipe-operators-update` -- and they came from two different Python areas, so the vendor-web derivation (gate minus `ze-<area>-`) has nothing to strip. `le docvalid`'s verbs are the gate names minus `ze-`, which is as much as can be derived and still types nothing beside a gate name. `docvalid/actions.go` in the former top-level tool tree is the SECOND table of the vendor-web shape; the trigger to lift it into `leroot` in the former top-level tool tree stays where step 9 put it, at the third.
- **A command whose answers are not all one row set declares `ShapeDoc` for the whole root (step 5, 2026-08-26).** The contract answer carries seven lists, so `rowsInKeyed` refuses to choose between them. The shape is per ROOT because `leroot.Run` hands the engine the command NAME and never the action, so a per-action `RegisterShape` would never be looked up. The cost is that `| count` is refused for the drift action too, by name and before the tree is walked; `| json`, `| yaml` and `| table` render all three answers, and `| json` over the drift answer unwraps its single row set to the findings array.
- **Step 10 is PART DONE: 2 of the 7 the retired `scripts/evidence` (current producer: `internal/le/`) gates are ported, and the five that remain are the five biggest scripts (2026-08-26).** `evidence` in the former top-level tool tree carries `ze-evidence-release-candidate-check`, and `deployment` in the former top-level tool tree carries `ze-deployment-l2tp-test`; the shared build-tag derivation every other evidence script needs is `featuretags.DaemonBuildTags`. Unported: `ze-deployment-vpp-test` (`effective-vpp.py`, 1516L), `ze-deployment-gokrazy-l2tp-ppp-test` (1144L), `ze-deployment-l2tp-ppp-test` (741L), `ze-deployment-vpp-iface-test` (499L) and `ze-qemu-vpp-hugepages-test` (416L), the first four of which are rows in the existing `deployment` in the former top-level tool tree action table. Nine non-gate scripts remain beside them, `effective-vrrp-keepalived.py` (1734L) and `qemu-run.py` (870L) the largest. The directory is 11,724 lines, which is larger than any earlier step by a factor of four.

- **Step 10b ported 2 of those 5, and 3 remain (2026-08-26).** `deployment/vppiface.go` in the former top-level tool tree carries `ze-deployment-vpp-iface-test` and the new `qemu` in the former top-level tool tree carries `ze-qemu-vpp-hugepages-test`. Unported: `ze-deployment-vpp-test` (1516L), `ze-deployment-gokrazy-l2tp-ppp-test` (1144L) and `ze-deployment-l2tp-ppp-test` (741L), each still a `forked` row in `deployment/actions.go` in the former top-level tool tree. The nine non-gate scripts are untouched.

- **A `forked` row makes the census say PORTED while the driver is still Python, so the parity COUNT cannot see step 10b at all (2026-08-26).** `integration` in the former top-level tool tree and `deployment` in the former top-level tool tree claimed all seven evidence gates when their AREAS were ported (steps 10 and 11d), and each unported driver became a row that starts the same process the Make target started. `parity.Take` reads a claim whose command is reachable as served, so `unported` was already at its final value for these gates before any of them had a Go driver. Step 10b therefore leaves `unported` unchanged at 47 and `ported` unchanged, which does NOT mean the tools were not wired: `le deployment vpp-iface-test` and `le qemu vpp-hugepages-test` now run Go rather than `python3`. The number that will move is `script-files`, at step 14. Whether the census should distinguish "the area is ported" from "the driver is ported" is an owner question rather than something to fix by editing `parity.Take`.

- **A gate whose driver becomes Go LEAVES the area that forked it, because a verb is derived and not typed (step 10b, 2026-08-26).** `ze-qemu-vpp-hugepages-test` was a row of `integration` in the former top-level tool tree, where `leaction.Area.verbOf` strips `ze-integration-` and nothing else, so it was typed as its whole gate name. It is now `le qemu vpp-hugepages-test`, and `integration/gates.go` in the former top-level tool tree records that eight of the Python area's gates live in the areas that own their gate-name families rather than seven. Thirteen more `ze-qemu-` Make targets exist and none of them is an `le` gate today; each is a row in `qemu/actions.go` in the former top-level tool tree when it becomes one. Step 14 owes the move one repoint: the `ze-qemu-vpp-hugepages-test` recipe in the retired `mk/test-integration.mk` (current producer: `internal/le/integration/gates.go`) runs `$(CURDIR)/le integration ze-qemu-vpp-hugepages-test`, and `./le` is the Python shim whose `integration` area still declares that gate, so the target is unaffected today and must become `le qemu vpp-hugepages-test` in the commit that repoints `./le`.

- **The daemon cross-compile is now stated ONCE, in `deployment/daemonbuild.go` in the former top-level tool tree (step 10b, 2026-08-26).** `daemonRel`, `daemonBuildArgs` and `buildDaemon` were lifted out of `l2tp.go` when the second proof needed them, rather than copied. Both proofs build the same argv from the same manifest, which is what `TestBothHalvesBuildTheVPPDaemonWithEveryGate` and `TestTheDaemonIsBuiltWithEveryFeatureGate` each pin from their own side.

- **`qemu-run.py` BECOMES a command, and it is the precondition for every guest-side port (step 10b, 2026-08-26).** It is 870 lines and 16 recipe lines in the retired `mk/test-integration.mk` (current producer: `internal/le/integration/gates.go`) invoke it. Every one of its decisions is a HOST decision: which Alpine ISO to cache and boot, which packages to install in the guest, which 9p mount to share the checkout over, which accelerator to ask for, and how long to wait. That is the half `effective-verify.sh` gave to `le`, met a second time, so it is `le qemu run`. What runs INSIDE the guest is the string it is given, and `qemu-all-tests.sh` is one such string, so step 10's ruling that the guest entry point does not move is untouched by this: the two are opposite halves of one boundary rather than one decision. Two costs travel with the port. It imports `alpine_iso.py` and `homebrew.py`, so the three move together. And `--run` carries a command, which is a flag with a value: the `--root` precedent (step 4) says a flag becomes an environment setting, and `ze-qemu-debug` already exports `ZE_QEMU_DEBUG_RUN`, so `ZE_QEMU_RUN` is the shape that keeps keyword-before-value.

- **The four `effective-install-*-qemu.py` scripts BECOME commands, in `qemu` in the former top-level tool tree, and they are ONE package rather than four (step 10b, 2026-08-26).** Together they are 2066 lines. Each is a host-side driver: it cross-compiles `cmd/ze-installer`, builds an image with `ze appliance`, serves it over HTTP from the host, starts QEMU, and asserts on the host's view of the serial console and an SSH login. Nothing of any of them runs inside the guest -- the guest runs the INSTALLER, which is already Go and is the artifact under test -- so none of them meets the `qemu-all-tests.sh` exception. Their four Make targets already begin `ze-qemu-`, so each is a row in `qemu/actions.go` in the former top-level tool tree with a derived verb and no `Verb:` line. Three costs. They form an import CHAIN -- `-iso-` and `-scenarios-` load `effective-install-qemu.py` as a module, `-ventoy-` loads `-iso-`, and `-iso-` also loads `homebrew.py` -- so a half-port leaves a live script importing a deleted file, and all five move in one commit. `test/install/qemu-full.ci` and `test/install/qemu-iso.ci` exec two of them by path, so both `.ci` change in the same commit. And none of the four is an `le` gate today, so porting them moves no census number and adds four rows to a table the census reads.

- **A REPEATED QUERY is not an effect, and step 10b is where that rule had to be applied to a call SEQUENCE (2026-08-26).** The script asks `show plugins` six times, once for the report and once per gated scenario; the port asks each plugin once and remembers the answer. the retired `scripts/evidence/vpp_iface_parity_test.go` (current producer: `internal/le/deployment/`) therefore compares the sequence with those calls removed, asserts the remaining count on BOTH sides at 14, and asserts separately that each half asked the query at least once. Removing calls from a comparison is how a comparison becomes vacuous, so the count is what stops it: this is the step-10 L2TP lesson applied to a filter rather than to a delay.
- **A gate-name FAMILY became one area, and `ze-deployment-` is the first family whose members do not all live in the retired `scripts/` (current producer: `internal/le/`) (step 10, 2026-08-26).** Seven gates begin `ze-deployment-`, and two of them are `test/interop-l2tp/run.py` (retired; now `internal/le/interoplab/l2tp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> and `test/interop-pppoe/run.py` (retired; now `internal/le/interoplab/pppoe/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) -->, which this spec's scope does not reach. `deployment` in the former top-level tool tree is therefore an area that will still be growing after step 13, and each of those two is a row in the same table whenever `test/` is ported.
- **`effective-verify.sh` BECOMES a command; `qemu-all-tests.sh` DOES NOT MOVE (step 10, 2026-08-26).** The rule the retired `mk/test-fuzz.mk` (current producer: `internal/le/fuzz/actions.go`) records is that a script stays where the concern is. `effective-verify.sh` has two halves and only one of them is a container's: the host half decides (docker and git are present, the worktree is clean, this is the image and this is the mount) and that is `le`'s to own, so `evidence` in the former top-level tool tree owns it and carries the container's own bash program as `ContainerScript`, a constant a test can read. It stays a program rather than becoming Go steps because the container is a `golang` image carrying no `le`, and driving it from the host would mean cross-compiling a second `le` for whatever `--platform` names, out of the tree that has not been judged yet. `qemu-all-tests.sh` is the opposite: it runs INSIDE the guest, after `qemu-run.py` has 9p-mounted the checkout and installed a toolchain, and the guest holds no `le`. Making it a command makes `le` a guest-side runtime dependency of every QEMU target, which is a decision about the harness rather than about this script -- so it moves with `qemu-run.py` or not at all, and step 14 owes it a home outside the retired `scripts/` (current producer: `internal/le/`) either way. the retired `scripts/evidence/qemu_kernel_wiring_test.go` (current producer: `internal/le/deployment/`) also READS it as text to derive three checks.
- **Every `effective-*.py` that a QEMU target runs is executed INSIDE the VM, and step 14 owes those ports a guest-side binary (step 10, 2026-08-26).** the retired `mk/test-integration.mk` (current producer: `internal/le/integration/gates.go`) launches them as `python3 scripts/evidence/qemu-run.py (retired; current producer: `internal/le/qemu/run.go`) ... --run 'python3 scripts/evidence/effective-l2tp-ppp.py (retired; current producer: `internal/le/deployment/l2tp.go`)'`, and the same shape carries `effective-pppoe-accel.py` and `effective-vrrp-keepalived.py`. The guest has python3 and the 9p-mounted checkout; it has no `le`. The host already cross-compiles three binaries for the guest (`ze-qemu-crossbuild`), so a fourth is one line, but nothing in the port itself makes that true and the `--run` strings must change in the same commit.
- **AC-11 for a tool that drives Docker is proven over the argv, and the stand-in must PLAY the peer, not merely record it (step 10, 2026-08-26).** Both halves are pointed at one recording `docker` and one recording `go` on PATH, over a fixture checkout, and what is compared is every call's argv plus the files left on disk. The trap is timing: the first stand-in answered both daemon lines at once, both halves reached their verdict and stopped before `xl2tpd` had recorded its own argv, and the one call that proves another implementation was involved went missing from BOTH recordings -- so the comparison passed at five calls instead of six. The stand-in now delays the session line by a second, and the count is asserted rather than only compared. A parity test that compares two empty recordings proves nothing, and this is the shape that produces one.
- **A query is not an effect, so the two halves need not ask it the same way (step 10, 2026-08-26).** `effective-verify.sh` runs `git rev-parse --show-toplevel` and then two `git status` calls; the command runs one, because `lepath.Root()` already answers which checkout it is in and the porcelain listing already carries the dirty paths. The parity comparison is over the `docker run` argv, the exit status and the dirty verdict. This is the `--root` decision of step 4 reaching a second tool, and it is what makes the comparison about what the gate DOES rather than about how each half learned a fact.
- **`--strict` is not ported, and the drift gate exits 1 on drift (step 5, 2026-08-26).** Nothing invoked it: the retired `mk/check-docs.mk` (current producer: `internal/le/doc/check/actions.go`), the retired `scripts/le/application/check_docs.py` (current producer: `internal/le/`) and `doc_drift_warnings` (the retired `scripts/dev/commit_helper.py` (current producer: `internal/le/commit/prepare.go`)) all run the script bare, and the last reads exit 1 as its advisory signal. This is the `--root` decision of step 4 applied to the other flag the script declared.

## RFC Documentation (Scope: protocol)

N/A - this spec touches no protocol surface.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/le/*`, `cmd/ze/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] `TestEveryMakeProducerHasAReachableNativeAction` and `TestNoPythonLeRemains` pass

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)
- [ ] Every new test shown to FAIL under a recorded mutation of the behaviour it pins

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `./le spec session review record spec spec-le-is-a-ze-binary verdict clean rounds <N> file plan/spec-le-is-a-ze-binary.md`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

### Integration Checklist

| Row | Answer |
|-----|--------|
| YANG schema + validation | N/A - `le` commands are not operator config |
| CLI grammar | Yes - `ai/patterns/cli-command.md`, per tool |
| Completion | Yes - `TestStandaloneLeAndZeLeHaveIdenticalSurface` |
| Functional test | Yes - per-area parity tests plus `internal/le/leroot/dispatch_test.go` |
| Env var | N/A - no new env var; `ZE_REPO_ROOT` unchanged |
| Doctor check + diagnostic code | N/A - adds no runtime dependency to the daemon |
| Prometheus counters | N/A - build-host tool |
| BGP family surface | N/A - no protocol surface |

### Documentation Update Checklist (BLOCKING)

| Row | Answer |
|-----|--------|
| `ai/INDEX.md` keyword + task rows | Yes - one row per `le` command area |
| `CLAUDE.md` script paths | Yes - at the swap |
| `ai/rules/commands.md` | Yes - how to run a gate changes at the swap |
| `docs/architecture/core-design.md` | Yes - the shared registry and the two composition roots |
| `docs/contributing/` | Yes - how to add a new check |
| `docs/guide/command-reference.md` | N/A - `le` is not an operator surface |
| Source anchors (`<!-- source: -->`) | Yes - every anchor naming a moved file |
| `docs/features.md` | N/A - not a product feature |
| Plugin docs | N/A - no plugin change |
| API/RPC/event docs | N/A - no API change |
| Telemetry docs | N/A - no metric change |
| Runtime inventory docs | N/A - no registry inventory change |

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path |
| Correctness | The first failing gate's exit code propagates; `internal/le/commit/prepare.go` still distinguishes 3 from 1 |
| Naming | No `le` root name collides with a `ze` root name; every action uses its area-local native verb |
| Data flow | Registration only: no per-tool switch, field or factory added to a shared package |
| Rule: `ai/rules/cli.md` | Each ported tool answers structured data, so the pipe operators render it |
| Rule: `ai/rules/testing.md` | Each subprocess-to-function test conversion names what the old assertion proved and where that proof now lives |
| Rule: `ai/rules/no-layering.md` | The parity gate is live from step 1, so the two implementations cannot drift silently |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The `cmd/ze` le personality builds | `go build -o bin/le ./cmd/ze` |
| Every registered tool dispatches | `TestStandaloneLeAndZeLeHaveIdenticalSurface` |
| No ported tool is build-ignored | `TestNoDevelopmentToolIsBuildIgnored` |
| No test shells out to `go run` | `TestNoDevelopmentToolTestShellsOutToGoRun` |
| Every producer has a native action | `TestEveryMakeProducerHasAReachableNativeAction` |
| No retired Python producer remains | `TestNoPythonLeRemains` |
| No Make compatibility identity remains | inspect the native action tables; every verb is area-local |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | `le` takes developer argv, not untrusted input; a tool that reads repository files must still bound what it reads |
| Resource exhaustion | A tool walking the tree bounds its walk; no unbounded `make` over user-controlled counts |
| Error leakage | A gate's failure output names the file and the reason, and does not print environment or credentials |
| Authorization failing open | Not applicable to `le`, but any tool ported into `ze` becomes reachable on an appliance: check its Meta Mode and Section before adding that import |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Parity gate red after a port | The tool was not wired; do not lower the count by hand |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Review Gate

<!-- Filled by /ze-close via /ze-review. Do not fill in advance. -->

| Run | Verdict | Rounds | Lenses | Artifact |
|-----|---------|--------|--------|----------|
| | | | | |

## Pre-Commit Verification

<!-- BLOCKING. Do NOT trust the audit above: re-verify independently and paste
     the evidence. For each row run a command (ls, grep, go test -run) now.

     EVERY sub-table needs at least one data row: pre_commit_verification_gaps
     in scripts/dev/commit_helper.py (retired; current producer: `internal/le/commit/prepare.go`) checks them one by one and names the empty
     ones. A row in Files Exist is not evidence for AC Verified.
     Not acceptable: "already checked", "should work", a pointer to the audit. -->

### Files Exist (ls)
<!-- Every file in "Files to Create", and every .ci named in Wiring Test and
     Functional Tests. Paste the ls output. -->
| File | Exists | Evidence |
|------|--------|----------|
| `cmd/ze/` | Unresolved | Closure-time file evidence was not collected. |
| `internal/le/register.go` | Unresolved | Closure-time file evidence was not collected. |
| `internal/le/<tool>/<tool>.go` | Unresolved | This is the live file pattern; closure-time inventory was not collected. |
| `internal/le/<tool>/register.go` | Unresolved | This is the live file pattern; closure-time inventory was not collected. |
| `internal/le/parity/` <!-- doc-links: ignore (the parity census package is gone from the tree; this spec is still open on it) --> | Unresolved | Closure-time file evidence was not collected. |
| `test/ui/le-binary-dispatches.ci` | Unresolved | Closure-time `ls` was not run; earlier phase handoffs report this test, but that is not fresh file evidence. |

### AC Verified (grep/test)
<!-- Every AC-N, re-checked. Acceptable: test name + pass output, grep showing
     the call, ls showing the file. -->
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | The built `le` binary dispatches every ported tool through the shared registry. | None. Phase handoffs report `TestStandaloneLeAndZeLeHaveIdenticalSurface` and built-binary `.ci` coverage; closure-time verification was not run. |
| AC-2 | Every `ze` build flavour links no `internal/le/` plugin. | None. An earlier handoff reports `TestNormalZeLinksNoInternalLe` and eight `go list -deps` flavours; closure-time verification was not run. |
| AC-3 | `le` registers no product command as its own. | None. Phase handoffs report `TestLeRegistersOneRootAndNoToolRoots`; closure-time verification was not run. |
| AC-3b | Both binaries use one command registry and neither declares another. | None. A phase handoff reports `TestStandaloneLeAndZeLeHaveIdenticalSurface`; closure-time verification was not run. |
| AC-4 | No ported tool remains build-ignored. | None. Phase handoffs report `TestNoDevelopmentToolIsBuildIgnored`; the migration is incomplete and closure-time verification was not run. |
| AC-5 | Ported tool tests call functions and do not invoke `go run`. | None. Phase handoffs report `TestNoDevelopmentToolTestShellsOutToGoRun`; the migration is incomplete and closure-time verification was not run. |
| AC-6 | Every producer has a named native action and no action preserves a retired Make target identity. | Unresolved. Run `TestEveryMakeProducerHasAReachableNativeAction` for closure evidence. |
| AC-7 | Every ported tool returns structured data for the pipe operators. | None fresh. Phase handoffs name package and `.ci` checks for individual ports; closure-time verification was not run. |
| AC-8 | A gate preserves its first failing exit code. | None fresh. Phase handoffs record individual exit-code cases; the complete migration was not re-checked. |
| AC-9 | Native action completeness comes from current producer and action tables; no Python parity census remains. | Unresolved. Run `TestEveryMakeProducerHasAReachableNativeAction` and `TestNoPythonLeRemains` for closure evidence. |
| AC-10 | The committed tree builds and loads every registered tool. | Unresolved. `TestTheCheckoutsOwnHeadIsClean` was not run for closure and the current handover says nothing is committed. |
| AC-11 | Every old and new tool pair agrees on exit code and output. | Unresolved. Phase handoffs record per-area parity results, but the current handover lists unported work. |
| AC-12 | After the swap, no Python `le`, build-ignored tool, or reference remains. | Unresolved. The current handover lists the swap and the remaining Python tools as outstanding. |
| AC-13 | Duplicate root names are rejected rather than shadowed. | None fresh. Phase handoffs report `TestDuplicateLeRootIsRejected`; closure-time verification was not run. |

### Wiring Verified (end-to-end)
<!-- Every Wiring Test row: does the .ci exist AND exercise the claimed path?
     Read the file; do not infer it from its name. -->
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le <name>` typed by a developer | No `.ci` named for this row; plan names `TestStandaloneLeAndZeLeHaveIdenticalSurface` | Unresolved; closure-time test was not run. |
| A `ze` build of any flavour | No `.ci` named for this row; plan names `TestNormalZeLinksNoInternalLe` | Unresolved; closure-time test was not run. |
| A `cmd/ze` build | No `.ci` named for this row; plan names `TestLeRegistersOneRootAndNoToolRoots` | Unresolved; closure-time test was not run. |
| Either binary's dispatch | No `.ci` named for this row; plan names `TestStandaloneLeAndZeLeHaveIdenticalSurface` | Unresolved; closure-time test was not run. |
| `./le <name> \| json` | No `.ci` named for this row; plan names `TestDispatchUsesSharedPipeRenderers` | Unresolved; closure-time test was not run. |
| Tab after `./le ` | No `.ci` named for this row; plan names `TestStandaloneLeAndZeLeHaveIdenticalSurface` | Unresolved; closure-time test was not run. |
| `./le <area> <verb>` for every current producer | No `.ci` named for this row; plan names `TestEveryMakeProducerHasAReachableNativeAction` | Unresolved; closure-time test was not run. |
| `go build ./...` over a `git archive HEAD` export | No `.ci` named for this row; plan names `TestTheCheckoutsOwnHeadIsClean` | Unresolved; the current handover says nothing is committed. |
| Binary init importing both composition roots | No `.ci` named for this row; plan names `TestNoLeNameCollidesWithZe` | Unresolved; closure-time test was not run. |
| `./le <name>` against the built binary | `test/ui/le-binary-dispatches.ci` | Unresolved. Earlier handoffs report the `.ci`, but it was not read or run for closure. |

### Assumptions Resolved
<!-- Every A-N. `unvalidated` is not a valid final status. A broken assumption
     needs a Mistake Log row and a Deviations entry. -->
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | Broken for lint; compiler portion confirmed | The spec records that the first untagged tool passed `go vet` but produced six `golangci-lint` findings. This was not re-checked for closure. |
| A-2 | Unresolved | The spec confirms the dependency claim only for the retired `scripts/lint` (current producer: `internal/le/`); the migration-wide claim has no final evidence. |
| A-3 | Unresolved | Phase handoffs record parity for ported areas, but the current handover lists unported work. |
| A-4 | Confirmed in the spec; not closure-rechecked | The spec records that 10 of 11 first-port Python cases transferred as intent and none transferred as code. |
| A-5 | Confirmed in the spec; not closure-rechecked | The spec records the never-linked design and a measured current-name collision count. |
| A-6 | Confirmed in the spec; not closure-rechecked | The spec records the first structured-report port and its shared payload mechanism. |

### Documentation Verified
<!-- Every Yes in the Documentation checklist: verify the edited claim against
     source. Every No: paste the grep that proves no update was needed. -->
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `ai/INDEX.md` keyword and task rows | No closure-time source check was run; the checklist requires one row per command area. | No; unresolved. |
| `CLAUDE.md` script paths | No closure-time source check was run; the checklist schedules this update at the swap, which remains outstanding. | No; unresolved. |
| `ai/rules/commands.md` gate invocation | No closure-time source check was run; the checklist schedules this update at the swap, which remains outstanding. | No; unresolved. |
| `docs/architecture/core-design.md` shared registry and composition roots | No closure-time source check was run. | No; unresolved. |
| `docs/contributing/` instructions for adding a check | No closure-time source check was run. | No; unresolved. |
| `docs/guide/command-reference.md` | The checklist marks this N/A because `le` is not an operator surface; no independent closure check was run. | No; N/A claim not re-verified. |
| Source anchors naming moved files | No closure-time anchor check was run. | No; unresolved. |
| `docs/features.md` | The checklist marks this N/A because the migration is not a product feature; no independent closure check was run. | No; N/A claim not re-verified. |
| Plugin documentation | The checklist marks this N/A because no plugin surface changes; no independent closure check was run. | No; N/A claim not re-verified. |
| API, RPC, and event documentation | The checklist marks this N/A because no API surface changes; no independent closure check was run. | No; N/A claim not re-verified. |
| Telemetry documentation | The checklist marks this N/A because no metric changes; no independent closure check was run. | No; N/A claim not re-verified. |
| Runtime inventory documentation | The checklist marks this N/A because no registry inventory changes; no independent closure check was run. | No; N/A claim not re-verified. |
