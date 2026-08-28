# Testing Guide

This guide covers how to run tests, what the different test types are, and how to
interpret their output. For the full technical reference (`.ci` format spec, `.et`
directives, fuzz target list), see `docs/functional-tests.md`.

## First-time setup

```sh
./le setup
./le verify current mode full
```

`./le` is the compiled repository development entry point. Each area exposes
its own verbs, for example `./le functional plugin`,
`./le qemu install-iso-test`, and `./le test-unit bgp`. Legacy command names
have no compatibility aliases.

Action words describe the contract: `check` is a read-only verdict, `verify` is
a composite policy gate, `report` is advisory output, and `write` or `update`
changes tracked state.

## The escalation ladder

Use the narrowest test that covers your change. Escalate only when needed.

| Step | What you run | When | Time |
|------|--------------|------|------|
| 1 | `./le job run label one-test command go test ./pkg/... -run TestName` | Iterating on one test | seconds |
| 2 | `./le test-unit bgp` | Checking one component group | 10s to 1:30 |
| 3 | `./le functional plugin` | Checking the user-visible path | varies |
| 4 | `./le verify current mode full` | Ready to commit | about 2 minutes |

`./le verify current mode full` is the pre-commit gate. The narrower commands
are development tools. `./le job run` admits an individual heavy command
through `internal/le/lejob`, so concurrent sessions do not oversubscribe the
machine.

### Component groups for step 3

| Command | Scope | Time |
|---------|-------|------|
| `./le test-unit bgp` | BGP engine, wire, reactor | about 1:30 |
| `./le test-unit core` | Core libraries | about 30 seconds |
| `./le test-unit plugins` | All plugins | about 40 seconds |
| `./le test-unit config` | Config parsing and YANG | about 20 seconds |
| `./le test-unit cli` | CLI component | about 10 seconds |

Pick the group matching your change.

## Test types

Ze has four levels of testing, each covering a different concern.

### Unit tests (`*_test.go`)

Standard Go tests. They validate logic in isolation: does the parser produce the
right output, does the state machine transition correctly, does the encoder
round-trip.

```sh
./le job run label bgp-message command go test ./internal/component/bgp/message/...
./le job run label parse-origin command go test ./internal/... -run TestParseOrigin
./le verify current mode full
```

### Functional tests (`.ci` files)

These spin up real Ze processes and test behavior end-to-end: does the config
parse, does the BGP session establish, does the plugin produce the right output.

Each `test/` subdirectory has its own runner and format:

| Directory | What it tests | Runner |
|-----------|---------------|--------|
| `test/encode/` | BGP wire encoding | `ze-test bgp encode` |
| `test/decode/` | Wire decoding | `ze-test bgp decode` |
| `test/parse/` | Config parsing (valid/invalid) | `ze-test bgp parse` |
| `test/plugin/` | Plugin behavior | `ze-test bgp plugin` |
| `test/reload/` | Config reload | `ze-test bgp reload` |
| `test/ui/` | CLI completion | `ze-test ui` |
| `test/editor/` | TUI editor (`.et` files) | `ze-test editor` |
| `test/managed/` | Managed config | `ze-test managed` |
| `test/web/` | Web UI | `ze-test web` |
| `test/l2tp/` | L2TP daemon | `ze-test l2tp` |
| `test/firewall/` | Firewall | `ze-test firewall` |
| `test/policy/` | Policy routing | `ze-test policy` |
| `test/exabgp-compat/` | ExaBGP compatibility | `ze-test exabgp` |

Run a single test by one-based ID or exact name, list the available IDs, or
resume from the last printed ID after an interrupted run. Queue the runner
through `./le job run label <label> command <argv...>` to use the same admission
as a full suite.

```sh
./le job run label plugin-42 command bin/ze-test bgp plugin 42          # test id 42
./le job run label encode-list command bin/ze-test bgp encode --list    # list N/TOTAL, id, and name
./le job run label plugin-from-42 command bin/ze-test bgp plugin --start 42  # id 42 and every later test
./le job run label editor-7 command bin/ze-test editor 7                # editor test id 7
./le job run label editor-nav command bin/ze-test editor -p nav         # editor tests matching "nav"
./le job run label exabgp-from-20 command bin/ze-test exabgp --start 20 # resume ExaBGP compatibility
```
<!-- source: internal/test/runner/selection.go -- Selection -->
<!-- source: internal/test/runner/display.go -- TestFinished -->
<!-- source: internal/test/cli/cmd_exabgp.go -- ExaBGP compatibility selection -->

Run a full suite:

```sh
./le functional encode
./le functional plugin
./le functional
./le functional exabgp-test
```

### Mutation tests (gomu)

Coverage tells you which lines ran. Mutation testing tells you whether the tests
would notice if those lines did something different.
[gomu](https://github.com/sivchari/gomu) rewrites the AST (arithmetic, conditional,
logical, bitwise, branch, return value, and error handling operators), runs the test
suite against each mutation, and reports which mutations survived.

gomu uses overlay-based execution, so it never modifies source files on disk.
It is a Go tool recorded in `tools.go`. The native `./le mutation combine`
command combines completed reports, and `./le mutation record-history` appends
their per-package scores to the committed history.

Mutation testing is advisory. It never gates `./le verify current mode full` or CI. A surviving mutant
is a signal that a test could be stronger, not a blocking failure.

Files with custom build tags and `cmd/ze/` are excluded via `.gomuignore` because
gomu has no `--tags` support. Reports land in `tmp/` (gitignored). Mutation score
history is tracked in `test/mutation/history.ndjson`.

Tuning: `GOMU_WORKERS` (default: half CPU cores), `GOMU_TIMEOUT` (default: 120s),
`GOMU_THRESHOLD` (default: 0%).

### Interop and integration tests

These require external infrastructure (Docker, root/CAP_NET_ADMIN, QEMU, or internet).
They are not part of the normal development cycle.

```sh
./le integration interop
./le integration iface
./le qemu all-tests
./le integration live-rpki
```


## When a test must be weakened

A red test means the CODE is wrong by default. Fix the code. When the coverage is
genuinely gone, because the feature it proved was removed or another test now
proves it, the removal is recorded rather than silent.

The record is `test/weakened.md`. It holds one row per weakened test, `| Test |
Reason |`, and it is REPLACED per commit. Delete the rows of the last commit,
write the rows of this one, and commit the file with the change. Git history
holds every past row beside the change it accepted, so
`git log -p -- test/weakened.md` is how you read them.

The route, in order:

1. Write the row first. The native write-edit hook reads the file from disk, so
   a row added after the edit takes effect only after the edit is retried.
2. Make the edit.
3. Name `test/weakened.md` in the commit. `internal/le/commit.Answer` refuses
   a commit that weakens a test and leaves the row in the working tree.

The test name is the enclosing top-level `func TestXxx` for Go, and the file stem
for a `.ci` or a `.et`. Write `package.TestName` when the bare name matches two
weakened tests in one commit.

An edit that only lowers a COUNT lands with a notice and no row: consolidating
three cases into one table lowers a count exactly as deleting a check does. The
COMMIT still needs a row for it, and that row is where you say which of the two
happened.

`./le test-weakened check` runs the checker over the file and is a stage of
`./le verify current mode full` in both modes. The rule is `ai/rules/testing.md`, and the design is
`docs/architecture/testing/test-health.md`.

## How `./le verify current mode full` works

`./le verify current mode full` is the pre-commit gate. It uses a two-pass strategy to stay fast:

1. **Lint** (27 linters via golangci-lint)
2. **Vet evidence** (cross-compile the Go evidence packages for Linux)
3. **Cached full pass** (`go test` without `-race`): Go caches by source hash,
   so when nothing changed this is instant. Catches logic regressions everywhere.
4. **Changed-group pass**: uses the test-only `CGO_ENABLED=1 go test -race`
   path on Linux and Darwin. Its test binaries are never release/build evidence.
5. **Functional tests** from `internal/le/functional/catalog.go`
6. **ExaBGP compatibility**

Common case (one group changed): ~2 min total instead of 6+.

### The builds the linter reads

<!-- source: internal/le/lintgate/actions.go -- Answer -->

golangci-lint analyzes ONE build for each run: one GOOS, one GOARCH, one tag
set. `./le verify-lint run` therefore runs more than one.

| Pass | Build | What only it reads |
|------|-------|--------------------|
| 1 | the host GOOS, `.golangci.yml` tags | the shipped daemon |
| 2 | `GOOS=linux`, plus `integration` | every kernel-facing `//go:build integration` test |
| 3..N | one for each row of `FLAVORS` (`internal/le/lintgate.Answer`) | `ze_installer`, `ze_distro`, `ze_appliance`, `ze_setup`, `tinygo`, and the capability tags (`debug`, `race`, `live`, ...). Also the GOOS and GOARCH targets no other pass compiles: `darwin`, `freebsd`, `openbsd`, `dragonfly`, `wasip1`, `linux/arm64` and `linux/riscv64`. Also the `compile-out` build, which drops every feature gate and keeps `ze_core` alone |

Each flavor pass lints only the packages holding a file the two passes above do
not load. That package set is DERIVED from the tree with `go list` on every run.
A hand-written list drifts the moment somebody adds a `//go:build debug`
file in a new package, and the drift is silent.

The driver then asserts coverage. Every tracked Go file must be loaded by some
pass. The exceptions are `vendor/`, `gokrazy/modcache/`, and the `//go:build
ignore` files that belong to no build. `./le verify-lint run` executes the
native plan and returns its flavor rows as structured output; `./le verify-lint run`
retains the established target interface.

Two files are still outside it, and the driver names both on every run.
`examples/plugin/go/main.go`, which is a separate Go module. And `tools.go`,
whose imports are programs rather than packages.

The `//go:build !ze_<feature>` compile-out stubs -- the code an operator reaches
when a feature is OFF -- were a third population until 2026-08-24. No pass could
select one, because `--build-tags` only ADDS to the config's list. The
`compile-out` row reaches every stub: it runs against a derived copy of
`.golangci.yml` carrying no build tags at all (`tagless_config`), so the command
line is the whole tag set and that set is `ze_core`. One row is enough because
the gates are independent, so a build with none of them on satisfies every
negated term at once. A feature-only helper must therefore carry its consumer's
build constraint. Without it, the bare-core build reports the helper as
`unused`, which is what it is in a binary that compiles its only caller out.

### Feature-tag structural type check

<!-- source: internal/le/staticcheckmatrix/actions.go -- Answer -->

`./le staticcheck-feature-matrix check` type-checks the working tree in N+2
configurations derived from the N unique features in `feature-gates.txt`: one
distro all-on row, one bare-core row, and one row that omits each feature.
Staticcheck includes selected `_test.go` files. This stage type-checks those
files without running their tests.

The matrix guarantees these direct single-feature omissions. It makes no
guarantee for arbitrary combinations with two or more omitted features.

Inside a verify run the stage judges only the rows the change set can move: the
distro all-on and bare-core rows, plus one row per feature tag the change
reached. Typing the target yourself judges every row, because only a verify run
publishes the feature-tag answer it scopes by. What widens the scope back to
every row is `../architecture/testing/verify-freshness-scope.md`. Rerun the stage
directly:

```sh
./le staticcheck-feature-matrix check
```

The matrix checks package and test variants in the working tree.
`./le repository-tracked-build check` remains the committed-tree final-link check for shipped
build flavors.

### The one stage that does not read your working tree

Every stage above compiles and runs the files on your disk, uncommitted ones
included. `./le repository-tracked-build check` (`internal/le/trackedbuild.Answer`) is the
exception: it extracts the commit with `git archive` and compiles the extracted
tree, so it sees only what git holds.

That is the population that breaks when a commit takes a consumer and leaves its
producer uncommitted. The build is green on your disk and red for everybody who
clones. Run it after the commit script when the commit carried Go:

```sh
./le repository-tracked-build check
REV=7abe8a07e ./le repository-tracked-build check
```

The action builds every flavor in `internal/le/trackedbuild/matrix.go` over
`./...`. Each row pins its tags, operating system where required, and a
tag-gated anchor file that proves the flavor selected code. Naming the package
alone is insufficient because `go build ./...` can skip every constrained file
and still exit zero.
About 45 seconds warm. It does not compile `_test.go` files, because `go build`
never does.

Output is captured to `tmp/ze-verify.log`. On failure:

```sh
grep -E "^--- FAIL|^FAIL|TEST FAILURE" tmp/ze-verify.log
```

## Interpreting output

### Unit test failures

Standard Go test output. Look for `--- FAIL: TestName` lines:

```
--- FAIL: TestParseOrigin (0.00s)
    origin_test.go:42: expected 0, got 1
FAIL	github.com/ze-software/ze/internal/core/bgp/attribute	0.003s
```

### Functional test failures

`ze-test` prints a summary at the end of each suite. Look for the test name
and the expectation that failed:

```
FAIL  encode/addpath.ci
  expect=bgp:conn=1:seq=1:hex=...
  got: FFFF...0038...
```

### Fuzz failures

Go places the failing input in `testdata/fuzz/<TestName>/` under the package.
The file contains the input that triggered the crash. Fix the code, then the
fuzz corpus entry becomes a regression test automatically.

## Cheat sheet

| I want to... | Run |
|--------------|-----|
| Check my setup | `./le verify current mode full` |
| Run one Go test | `./le job run label one-test command go test ./pkg/... -run TestName` |
| Run one functional test | `./le job run label plugin-42 command bin/ze-test bgp plugin 42` |
| Run a component group | `./le test-unit bgp` |
| Run the pre-commit check | `./le verify current mode full` |
| Type-check every supported feature combination | `./le staticcheck-feature-matrix check` |
| List native test actions | `./le help` |
| List functional tests | `./le job run label encode-list command bin/ze-test bgp encode --list` |
| Run one fuzz target | `FUZZ=FuzzName PKG=./path/... TIME=30s ./le fuzz run` |
| Run all fuzz targets | `./le fuzz run` |
| Check the commit compiles | `./le repository-tracked-build check` |
| Check web behavior | `./le functional web` |
| Check that every `*_templ.go` matches its `.templ` source | `./le doc-check templ-output`, and `./le repository generate` to bring it back in step. Both walk `internal/` only. Run neither templ command by hand, and switch off an editor's on-save templ integration. A bare `templ generate` walks from the repo root. It writes that root into every generated file, and it reds the gate |
