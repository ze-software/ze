# Testing Guide

This guide covers how to run tests, what the different test types are, and how to
interpret their output. For the full technical reference (`.ci` format spec, `.et`
directives, fuzz target list), see `docs/functional-tests.md`.

## First time setup

```sh
make ze-setup    # install all dev tools: build deps, linters, appliance tools (one-time)
make ze-smoke    # verify everything works: lint + unit + build (~2 min)
```

If `ze-smoke` passes, your environment is ready.

## The escalation ladder

Use the narrowest test that covers your change. Escalate only when needed.

| Step | What you run | When | Time |
|------|-------------|------|------|
| 1 | `go test -race -run TestName ./pkg/...` | Iterating on one test | seconds |
| 2 | `go test -race ./internal/component/bgp/reactor/...` | Iterating on one package | seconds |
| 3 | `make ze-test-bgp` | Done with a component, want to check for regressions | 10s - 1:30 |
| 4 | `make ze-verify` | Ready to commit | ~2 min |

`make ze-verify` is the pre-commit gate. Everything below that is a development tool.

### Component groups for step 3

| Target | Scope | Time |
|--------|-------|------|
| `make ze-test-bgp` | BGP engine, wire, reactor | ~1:30 |
| `make ze-test-core` | Core libraries | ~30s |
| `make ze-test-plugins` | All plugins | ~40s |
| `make ze-test-config` | Config parsing, YANG | ~20s |
| `make ze-test-cli` | CLI component | ~10s |
| `make ze-test-rest` | Everything else | ~1:00 |

Pick the group matching your change.

## Test types

Ze has four levels of testing, each covering a different concern.

### Unit tests (`*_test.go`)

Standard Go tests. They validate logic in isolation: does the parser produce the
right output, does the state machine transition correctly, does the encoder
round-trip.

```sh
go test -race ./internal/component/bgp/message/...    # one package
go test -race -run TestParseOrigin ./internal/...      # one test
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
make ze-encode-test     # all encode tests
make ze-plugin-test     # all plugin tests
make ze-functional-test # all release-gate suites
make ze-exabgp-test     # ExaBGP compatibility through ze-test
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
make ze-mutation-changed                              # changed files only (fast)
make ze-mutation-pkg PKG=./internal/core/textbuf/     # one package
make ze-mutation-test                                 # all non-excluded packages (slow)
make ze-mutation-report                               # full run with HTML report
```

Mutation testing is advisory. It never gates `ze-verify` or CI. A surviving mutant
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

## How `ze-verify` works

`ze-verify` is the pre-commit gate. It uses a two-pass strategy to stay fast:

1. **Lint** (27 linters via golangci-lint)
2. **Vet evidence** (cross-compile evidence scripts for Linux)
3. **Cached full pass** (`go test` without `-race`): Go caches by source hash,
   so when nothing changed this is instant. Catches logic regressions everywhere.
4. **Race pass on changed groups** (`go test -race` only on packages with
   modified `.go` files): catches data races in what you touched.
5. **Functional tests** (the gating functional suites; see `mk/test-functional.mk`)
6. **ExaBGP compatibility**

Common case (one group changed): ~2 min total instead of 6+.

### The one stage that does not read your working tree

Every stage above compiles and runs the files on your disk, uncommitted ones
included. `ze-tracked-build-check` (`scripts/checks/tracked_build.go`) is the
exception: it extracts the commit with `git archive` and compiles the extracted
tree, so it sees only what git holds.

That is the population that breaks when a commit takes a consumer and leaves its
producer uncommitted. The build is green on your disk and red for everybody who
clones. Run it after the commit script when the commit carried Go:

```sh
make ze-tracked-build-check              # HEAD
make ze-tracked-build-check REV=7abe8a07e  # any commit
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
| Check my setup works | `make ze-smoke` |
| Run one Go test | `go test -race -run TestName ./pkg/...` |
| Run one functional test | `bin/ze-test bgp plugin 42` |
| Run tests for what I changed | `make ze-test-bgp` (pick your group) |
| Pre-commit check | `make ze-verify` |
| See all test targets | `make help-test` |
| List functional tests | `bin/ze-test bgp encode --list` |
| Run fuzz for one target | `make ze-fuzz-one FUZZ=FuzzName PKG=./path/... TIME=30s` |
| Check test coverage | `make ze-unit-test-cover` then open `coverage.html` |
| Mutation test changed files | `make ze-mutation-changed` |
| Mutation test one package | `make ze-mutation-pkg PKG=./internal/core/textbuf/` |
| Debug a verify failure | `grep FAIL tmp/ze-verify.log` |
| Check the commit I just made compiles | `make ze-tracked-build-check` |
| Prove a web or looking-glass template renders the same bytes | `make ze-web-golden-check` |
| Prove a web or looking-glass ROUTE answers the same bytes | `make ze-web-golden-check` (the handler capture runs in the same target) |
| Prove an HTML builder that no template holds renders the same bytes | `make ze-web-golden-check` (the markup capture runs in the same target) |
| Recapture those bytes after a deliberate markup change | `make ze-web-golden-update`, then read the diff |
