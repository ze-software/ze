# Testing Guide

This guide covers how to run tests, what the different test types are, and how to
interpret their output. For the full technical reference (`.ci` format spec, `.et`
directives, fuzz target list), see `docs/functional-tests.md`.

## First time setup

```sh
make ze-dev-setup    # install all dev tools: build deps, linters, appliance tools (one-time)
make ze-smoke-verify    # verify everything works: lint + unit + build (~2 min)
```

If `ze-smoke-verify` passes, your environment is ready.

## Make target naming

Public project targets use:

```text
ze-<family>[-<scope>...][-<subject>]-<action>[-<mode>][-<format>]
```

The family and scope come first, then the subject, then the action. For
example, `ze-unit-bgp-test`, `ze-functional-plugin-test`, and
`ze-qemu-install-iso-test` identify progressively more specific test families
before saying that they run tests. Modes follow the action
(`ze-unit-test-cached`, `ze-fuzz-test-one`, `ze-qemu-test-all`), and output
formats come last (`ze-inventory-json`).

Action words describe the contract: `check` is a read-only verdict, `verify` is
a composite policy gate, `report` is advisory output, `update` rewrites tracked
state, `reconcile` rewrites generated state before checking whether it was
current, `record` appends durable evidence, `sync` copies canonical state, and
`build` produces an artifact. Build targets include the action rather than
reusing a bare binary basename: `ze-build` produces `bin/ze`,
`ze-appliance-build` produces `bin/ze-appliance`, and `ze-test-build` produces
`bin/ze-test`.

The conventional unprefixed entry points (`build`, `check`, `clean`, `fmt`,
`generate`, `help`, `test`, `tidy`, and `vet`) remain short. Retired names have
no compatibility aliases, so scripts and documentation must use the current
spelling. `make help`, `make help-test`, `make help-deploy`, and `make help-dev`
show the common entry points, not an exhaustive target inventory.

## The escalation ladder

Use the narrowest test that covers your change. Escalate only when needed.

| Step | What you run | When | Time |
|------|-------------|------|------|
| 1 | `CGO_ENABLED=0 go test -run TestName ./pkg/...` | Iterating on one test | seconds |
| 2 | `CGO_ENABLED=0 go test ./internal/component/bgp/reactor/...` | Iterating on one package | seconds |
| 3 | `make ze-unit-bgp-test` | Done with a component, want to check for regressions | 10s - 1:30 |
| 4 | `make ze-precommit-verify` | Ready to commit | ~2 min |

`make ze-precommit-verify` is the pre-commit gate. Everything below that is a development tool.

### The cadence targets

The ladder above is keyed to a change you are making. Three targets are keyed to
the calendar instead, and they exist for one reason: `make ze-precommit-verify` runs 27
checks and the nightly workflows run a dozen more, which leaves a set that is in
NEITHER and is therefore run by nobody.

| Target | Time | What it is for |
|--------|------|----------------|
| `make ze-cadence-daily-run` | seconds | Run it every morning. No Docker, no network, and it never takes the verify lock, so it cannot block |
| `make ze-cadence-weekly-run` | minutes | Takes the same repo-wide lock as `ze-precommit-verify`. Do not start it beside one: it blocks rather than fails, which reads as a hang |
| `make ze-cadence-monthly-run` | long | Needs Docker, QEMU or root. Its preflight probe runs first and says what this machine can do |

Each member is one of two kinds. A `gate` has a verdict, and a non-zero exit
fails the run. A `note` is a census or a report that exits 0 whatever it finds,
so it is printed and never fails the run. Mixing the two under one exit code is
what makes an aggregate meaningless: the censuses would drag it red every day
until it was ignored. The summary table is the product; the exit code covers the
gates.

`make ze-cadence-daily-run` is where `ze-repository-check` finally runs. `ze-precommit-verify` runs
`ze-repository-tree-check`, which passes `--changed-file ''`, and two of validate's
checks return empty before reading anything when that list is empty.

### Component groups for step 3

| Target | Scope | Time |
|--------|-------|------|
| `make ze-unit-bgp-test` | BGP engine, wire, reactor | ~1:30 |
| `make ze-unit-core-test` | Core libraries | ~30s |
| `make ze-unit-plugins-test` | All plugins | ~40s |
| `make ze-unit-config-test` | Config parsing, YANG | ~20s |
| `make ze-unit-cli-test` | CLI component | ~10s |
| `make ze-unit-rest-test` | Everything else | ~1:00 |

Pick the group matching your change.

## Test types

Ze has four levels of testing, each covering a different concern.

### Unit tests (`*_test.go`)

Standard Go tests. They validate logic in isolation: does the parser produce the
right output, does the state machine transition correctly, does the encoder
round-trip.

```sh
CGO_ENABLED=0 go test ./internal/component/bgp/message/...                  # one package
CGO_ENABLED=0 go test -run TestParseOrigin ./internal/...                  # one test
make ze-unit-test                                      # all packages (~5 min)
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

Run a single test by one-based id or exact name, list the available ids, or
resume from the last printed id after an interrupted run:

```sh
bin/ze-test bgp plugin 42          # test id 42
bin/ze-test bgp encode --list      # list N/TOTAL, id, and name
bin/ze-test bgp plugin --start 42  # run id 42 and every later test
bin/ze-test editor 7               # editor test id 7
bin/ze-test editor -p nav          # editor tests matching "nav"
bin/ze-test exabgp --start 20      # resume ExaBGP compatibility
```
<!-- source: internal/test/runner/selection.go -- Selection -->
<!-- source: internal/test/runner/display.go -- TestFinished -->
<!-- source: internal/test/cli/cmd_exabgp.go -- ExaBGP compatibility selection -->

Run a full suite:

```sh
make ze-functional-encode-test     # all encode tests
make ze-functional-plugin-test     # all plugin tests
make ze-functional-test # all release-gate suites
make ze-functional-exabgp-test     # ExaBGP compatibility through ze-test
```

### Mutation tests (gomu)

Coverage tells you which lines ran. Mutation testing tells you whether the tests
would notice if those lines did something different.
[gomu](https://github.com/sivchari/gomu) rewrites the AST (arithmetic, conditional,
logical, bitwise, branch, return value, and error handling operators), runs the test
suite against each mutation, and reports which mutations survived.

gomu uses overlay-based execution, so it never modifies source files on disk. It is
vendored in `tools.go` and runs via `go run`; no separate install is needed.

```sh
make ze-mutation-test-changed                              # changed files only (fast)
make ze-mutation-pkg-test PKG=./internal/core/textbuf/     # one package
make ze-mutation-test                                 # all non-excluded packages (slow)
make ze-mutation-report                               # full run with HTML report
```

Mutation testing is advisory. It never gates `ze-precommit-verify` or CI. A surviving mutant
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
make ze-interop-test              # FRR/BIRD in Docker
make ze-integration-test          # netns tests (needs root)
make ze-qemu-integration-test     # same tests in QEMU (macOS-friendly)
make ze-live-rpki-test            # real RPKI data (needs internet)
```

See `make help-test` for the full list.

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

1. Write the row FIRST. `c_test_weakening` (`.claude/hooks/pretool-writeedit.py`)
   reads the file from disk, so a row written after the edit buys nothing until
   you retry. The refusal message prints the exact row to write.
2. Make the edit.
3. Name `test/weakened.md` in the commit. `scripts/dev/commit_helper.py` refuses
   a commit that weakens a test and leaves the row in the working tree.

The test name is the enclosing top-level `func TestXxx` for Go, and the file stem
for a `.ci` or a `.et`. Write `package.TestName` when the bare name matches two
weakened tests in one commit.

An edit that only lowers a COUNT lands with a notice and no row: consolidating
three cases into one table lowers a count exactly as deleting a check does. The
COMMIT still needs a row for it, and that row is where you say which of the two
happened.

`make ze-test-weakened-check` runs the checker over the file and is a stage of
`ze-precommit-verify` in both modes. The rule is `ai/rules/testing.md`, and the design is
`docs/architecture/testing/test-health.md`.

## How `ze-precommit-verify` works

`ze-precommit-verify` is the pre-commit gate. It uses a two-pass strategy to stay fast:

1. **Lint** (27 linters via golangci-lint)
2. **Vet evidence** (cross-compile evidence scripts for Linux)
3. **Cached full pass** (`go test` without `-race`): Go caches by source hash,
   so when nothing changed this is instant. Catches logic regressions everywhere.
4. **Changed-group pass**: uses the test-only `CGO_ENABLED=1 go test -race`
   path on Linux and Darwin. Its test binaries are never release/build evidence.
5. **Functional tests** (the gating functional suites; see `mk/test-functional.mk`)
6. **ExaBGP compatibility**

Common case (one group changed): ~2 min total instead of 6+.

### Feature-tag structural type check

<!-- source: scripts/checks/staticcheck_feature_matrix.go -- buildFeatureMatrix, runStaticcheckFeatureMatrix -->

`make ze-staticcheck-feature-matrix-check` type-checks the working tree in N+2
configurations derived from the N unique features in `feature-gates.txt`: one
distro all-on row, one bare-core row, and one row that omits each feature.
Staticcheck includes selected `_test.go` files. This stage type-checks those
files without running their tests.

The matrix guarantees these direct single-feature omissions. It makes no
guarantee for arbitrary combinations with two or more omitted features.
Rerun the stage directly:

```sh
make ze-staticcheck-feature-matrix-check
```

The matrix checks package and test variants in the working tree.
`ze-repository-tracked-build-check` remains the committed-tree final-link check for shipped
build flavors.

### The one stage that does not read your working tree

Every stage above compiles and runs the files on your disk, uncommitted ones
included. `ze-repository-tracked-build-check` (`scripts/checks/tracked_build.go`) is the
exception: it extracts the commit with `git archive` and compiles the extracted
tree, so it sees only what git holds.

That is the population that breaks when a commit takes a consumer and leaves its
producer uncommitted. The build is green on your disk and red for everybody who
clones. Run it after the commit script when the commit carried Go:

```sh
make ze-repository-tracked-build-check              # HEAD
make ze-repository-tracked-build-check REV=7abe8a07e  # any commit
```

It builds six flavors over `./...`: `ze_core ze_distro`, `ze_test` and
`ze_core ze_appliance` each carry the feature tags that commit declared in
`feature-gates.txt`; `ze_setup`, `ze_core ze_setup` and `ze_installer` carry
none, matching the Makefile targets that build them. The installer flavor pins
`GOOS=linux`, and every flavor must select the tag-gated FILES its own tags own
(`ze_core_dispatch.go` for the core flavors, `setup_dispatch.go` for `ze_setup`,
and so on). Naming the package is not enough: `cmd/ze/main.go` carries no build
constraint, so the package resolves under any tag set at all, and `go build
./...` skips every file its constraints exclude while still exiting 0.
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
| Check my setup works | `make ze-smoke-verify` |
| Run one Go test | `CGO_ENABLED=0 go test -run TestName ./pkg/...` |
| Run one functional test | `bin/ze-test bgp plugin 42` |
| Run tests for what I changed | `make ze-unit-bgp-test` (pick your group) |
| Pre-commit check | `make ze-precommit-verify` |
| Type-check every supported direct feature omission | `make ze-staticcheck-feature-matrix-check` |
| See all test targets | `make help-test` |
| List functional tests | `bin/ze-test bgp encode --list` |
| Run fuzz for one target | `make ze-fuzz-test-one FUZZ=FuzzName PKG=./path/... TIME=30s` |
| Check test coverage | `make ze-unit-test-coverage` then open `coverage.html` |
| Mutation test changed files | `make ze-mutation-test-changed` |
| Mutation test one package | `make ze-mutation-pkg-test PKG=./internal/core/textbuf/` |
| Debug a verify failure | `grep FAIL tmp/ze-verify.log` |
| Check the commit I just made compiles | `make ze-repository-tracked-build-check` |
| Prove a web or looking-glass template renders the same bytes | `make ze-web-golden-check` |
| Prove a web or looking-glass ROUTE answers the same bytes | `make ze-web-golden-check` (the handler capture runs in the same target) |
| Prove an HTML builder that no template holds renders the same bytes | `make ze-web-golden-check` (the markup capture runs in the same target) |
| Recapture those bytes after a deliberate markup change | `make ze-web-golden-update`, then read the diff |
| Prove a rendering-engine port changed no page | `make ze-templ-port-check REF=<sha>`. It compares every fixture against the bytes it held at REF, under `golden.AssertPortFidelity`. Whitespace layout, doctype case, the attribute delimiter and the character-reference spelling fold. Nothing else does. Run it BEFORE you recapture. After a recapture, `ze-web-golden-check` compares the port against itself |
| Check that every `*_templ.go` matches its `.templ` source | `make ze-templ-output-check`, and `make generate` to bring it back in step. Both walk `internal/` only. Run neither templ command by hand, and switch off an editor's on-save templ integration. A bare `templ generate` walks from the repo root. It writes that root into every generated file, and it reds the gate |
