# Testing

**When:** writing tests, or when a test fails and you are tempted to weaken it
**Severity:** blocking

## Directives

Rationale: `ai/rationale/testing.md`
Structural template: `ai/patterns/functional-test.md`

## Draft a Functional Test Before It Is Live (BLOCKING)

Never write or iterate on a `.ci` inside `test/<suite>/`, and never edit a live
one in place. That directory runs on every `make ze-verify` in the checkout,
including runs by OTHER sessions, who then have to work out whether your
half-written test is their regression.

| Step | Command |
|------|---------|
| Write it in the incubator | `test/draft/<suite>/<name>.ci` |
| Run only drafts | `ze-test <suite> --draft -a` |
| Prove it under load | `scripts/dev/stress-repro.py "<suite> --draft" --test <id> --any-failure` |
| Promote when green | `mv test/draft/<suite>/<name>.ci test/<suite>/` |

`test/draft/` is gitignored and skipped by every repo-wide gate, so a draft
cannot redden anything for anyone. Changing an existing test is the same move:
copy it into the incubator, work there, `mv` it back. Full workflow: the
`/ze-test` skill, `test/draft/README.md`, `docs/functional-tests.md`.

Nothing in the incubator is gated, so promote early: the accept-only check, the
`time.sleep(` ratchet, and frame-length validation only start applying once the
file is live.

## Fix Code, Not Tests

When a test fails, fix the code to make the test pass. NEVER weaken or simplify test expectations to match broken code. Tests are ground truth. Even if an underlying mechanism changed (e.g., Unix sockets replaced by SSH), the test expectations stay and the replacement mechanism must satisfy them.

NEVER modify test data (golden files, expected output, fixtures, `.ci` expectations) to make a failing test pass without explicit user authorization. When output changes, the default assumption is that the code is wrong, not the test data. Ask the user before updating any test data, even if the new output looks plausible.

## RFC-Tagged Tests (BLOCKING)

A test carrying an `RFC requirement: <id> <polarity>` tag is the proof behind a public
compliance claim in `docs/features/rfc-status.md`, and `make ze-rfc-check` counts it as
that proof. Editing it to match the code retires the evidence while the claim stays up.

| Situation | Do |
|-----------|-----|
| A tagged test fails after your change | Fix YOUR code. The test is the requirement |
| You believe the test is genuinely wrong | STOP. Show the user the RFC text beside the test and ask. Do not edit first and explain after |
| The summary misquotes the RFC | Fix `rfc/short/rfcNNNN.md` (keep the id), then re-run `/ze-rfc-audit` |
| Reformat / comment / re-tag | Allowed; behavior must be unchanged |
| You added, moved, deleted, or re-tagged a tagged test (or an edit shifted its line) | Run `make ze-rfc-index` and commit `ai/RFC-REQUIREMENTS.md` in the SAME commit. The ledger records each test's `file:line`, and `ze-rfc-check` (both verify modes) fails on a stale ledger, so a skipped regen lands on the next session as a cross-commit diff |

**Where a tag may live, and what it is worth: four carriers, declared once in `CARRIERS` (`scripts/dev/rfc_requirements.py`) and derived by the scanner, the HEAD baseline, the ledger and the ratchets. Evidence has two axes: KIND (which layer the test exercises) and TIER (whether anything executes it).**

| Carrier | Cell in the ledger | Executed by | Tier |
|---------|--------------------|-------------|------|
| `*_test.go` | `unit/verify` | `make ze-unit-test` | runs on every push |
| `*.ci` | `functional/verify` | `make ze-functional-test` | runs on every push, but ONLY from a suite that target actually runs: the tier is derived per-suite from `mk/test-functional.mk`'s own `all_suites=` line, so a `.ci` in a suite outside it (traffic, vrrp, ipsec, flow-export, static, vpp, chaos) earns no verify tier, and `test/draft/` is skipped entirely |
| `*.et` | `editor/verify` | `make ze-editor-test` | runs on every push, on the same earned-per-suite basis as `*.ci` |
| `test/interop/scenarios/*/check.py` | `interop/nightly` | `make ze-interop-test` | scheduled, ADVISORY |

- **Prefer a `.ci` over an interop binding** when a behavior is reachable from both: a `.ci` runs inside `ze-verify` on every push, interop does not (owner decision, umbrella D3).
- A requirement whose ONLY evidence is nightly-tier is marked `**nightly-only**` on its ledger row and counted in its own rollup column: it is not merge-gate-proven, and the rollup deliberately never sums the two.
- **A tag in `test/ipsec-interop/`, `test/l2tp-interop/`, `test/pppoe-interop/`, or any other `check.py` tree is REFUSED** with an error naming the file, because nothing runs those suites automatically and a tag nothing executes is an absence of evidence rather than weak evidence: wire the suite into a pipeline first, then give its carrier a tier in `CARRIERS`.
- **Non-unit evidence is monotonic, per requirement and per tier.** Replacing a `.ci` binding with a unit tag, or with a nightly interop tag, fails `make ze-rfc-check`, and no annotation satisfies it.
- A `check.py` is TOKENIZED, not line-scanned: a `#` inside a docstring or string literal is not a comment and is not a tag, and an untokenizable `check.py` fails the scan closed.

`// test-relax:` does **not** authorize changing a tagged test: it is your own
justification, not the user's approval. Enforced by the `rfc-tagged-test` hook, which runs
before `test-weakening` precisely so the relax token cannot pre-empt it
(`ai/rules/hook-mapping.md`). Once the USER approves, record what they approved:
`// rfc-test-change-approved: <date> <what and why>`.

Every gated requirement needs BOTH a positive and a negative test. A negative-only test
passes if the code rejects everything; a positive-only test passes if it accepts
everything. Only the pair pins behavior to the requirement. Assert the EXACT outcome, never
a floor: `GreaterOrEqual(TreatAsWithdraw)` is also satisfied by `SessionReset`, so it cannot
fail when the implementation over-reacts. See `ai/skills/ze-rfc.md`.

## Back-Fill New Test Types (BLOCKING)

When you introduce a new test type, technique, or infrastructure (fuzz target, property test, mutation gate, `-race` sweep, clock-injection audit, new `.ci`/`.et` category, QEMU harness), apply it to the existing code it covers, not only to the code added alongside it. Coverage that grows only forward from the introduction date is the trap (`plan/learned/RECURRING-PATTERNS.md`, "New test type added but not back-filled to existing code").

In the same work:

1. Name the applicable set: the package glob, symbol kind, or call-site pattern the new test type is meant to cover.
2. Back-fill that set, OR record the uncovered remainder as explicit, tracked backlog (spec, handoff, or deferral table). Never leave it implicit.
3. Prefer a grep- or registry-driven audit that enumerates every applicable site over per-file judgement. `/ze-hunt` enumerates sites for grep-detectable patterns.

## Test Sensitivity Ratchets (BLOCKING)

A test that cannot fail, and a test file no target builds, both read as coverage
while providing none. Neither is visible in any count of tests, which is how the
published totals grew for years without either being noticed.

`make ze-test-sensitivity-check` (stage 10 of `make ze-verify`, both modes) counts
them and enforces committed floors in `test/health/sensitivity-baseline.json`. The
counts may only go DOWN, following the `test/.ci-sleep-baseline` convention.

| Detector | Fires when | Fix |
|----------|-----------|-----|
| assert-nothing | A `Test*` function has no reachable `Error`/`Fatal`/`Fail` call, no assertion-library call, no compile-time `var _ T = ...` assertion, and no `panic` | Add a real assertion, or annotate: `// test-asserts-nothing: <why the oracle is implicit>` |
| tag-orphan | A `_test.go` build constraint needs a `ze_*` tag that no `go test -tags` in `Makefile` or `mk/*.mk` supplies | Add the tag to a `go test` invocation, or delete the file |

Benchmarks and fuzz targets are deliberately exempt: a benchmark measures, and a
fuzz target delegates its oracle to the engine. Raising a floor is forbidden --
`make ze-test-health` only lowers one, so a regression cannot be laundered into
the baseline by regenerating.

`docs/features/test-health.md` (generated, `make ze-test-health`) reports these
alongside RFC proof density, mutation kill rate, negative-test ratio, and
technique adoption by package age. Read it before claiming the suite is healthy:
it is the answer to "would a regression be caught", which no test count gives.
Details: `test/health/README.md`.

| Target | Enforces | Notes |
|--------|----------|-------|
| `make ze-test-sensitivity-check` | The two ratchets, read from the tree | Stage 10 of `ze-verify`, both modes. Independent of the report |
| `make ze-test-health-check` | STRUCTURAL facts only: orphaned test files, unproven RFCs, metric statuses | Inside `ze-regen-check-readonly`. Volume counters are published, not gated, so adding a test does not force a regeneration |

The split is deliberate. Byte-gating the whole report charged a
regenerate-and-commit to ~60% of commits, and a check that fires that often for
cosmetic reasons gets routed around instead of read: the same "advisory gate
permanently red" failure the report is built to expose.

## No Throw-Away Tests

Never write temporary test code. Add functional or unit tests that run in CI.

| Situation | Location | Format |
|-----------|----------|--------|
| Valid config parses | `test/parse/` | `.ci` with `expect=exit:code=0` |
| Invalid config fails | `test/parse/` | `.ci` with `expect=exit:code=1` + `expect=stderr:contains=` |
| BGP encoding | `test/encode/` | Config + expectations |
| Plugin behavior | `test/plugin/` | Config + expectations |
| Wire decoding | `test/decode/` | stdin + cmd + `expect=json:` |
| Editor/TUI behavior | `test/editor/` | `.et` with `input=`/`expect=` directives |
| Internal logic | `internal/<pkg>/<file>_test.go` | Go test file |

Each `test/<subdir>/` has its own runner and format — they are not interchangeable. `test/parse/` only accepts config-parse `.ci` files (config text + `expect=exit:code=`). Putting a BGP-plugin scenario there will be rejected; put it in `test/plugin/`. Pure-logic, reactor-free code (encoders, parsers, state machines exercised directly) belongs in Go unit tests (`internal/<pkg>/<file>_test.go`), not in any `.ci` directory — `.ci` tests exist to prove a user entry point works end-to-end through the daemon.

## Make Targets

### Component-Group Unit Tests

Test one logical area during development instead of all 349 packages:

| Target | Scope | Approx time |
|--------|-------|-------------|
| `make ze-test-bgp` | `./internal/component/bgp/...` (96 pkgs) | ~1:30 |
| `make ze-test-core` | `./internal/core/...` (26 pkgs) | ~30s |
| `make ze-test-plugins` | `./internal/plugins/...` (44 pkgs) | ~40s |
| `make ze-test-config` | `./internal/component/config/...` (13 pkgs) | ~20s |
| `make ze-test-cli` | `./internal/component/cli/...` (3 pkgs) | ~10s |
| `make ze-test-rest` | Everything not in a named group (~70 pkgs) | ~1:00 |

All groups run with `-race`. Use the group matching your change during iteration.

### Verification Targets

| Target | Purpose |
|--------|---------|
| `make ze-verify` | Pre-commit gate: lint, changed-file wiring/doc/inventory, vet evidence, two-pass unit, functional, and ExaBGP |
| `make ze-verify-changed` | Changed-package lint/test plus wiring/doc/inventory, functional, and ExaBGP |
| `make ze-verify-wiring-docs` | Changed-file-aware wiring, documentation, command, and inventory gate |
| `make ze-unit-test` | All unit tests with `-race` under default-on feature tags, plus bare `ze_core` compile-out checks (~5 min) |
| `make ze-functional-test` | All 13 functional test suites |
| `make ze-lint` | 26 linters |
| `make ze-ci` | lint + unit + build |
| `make ze-fuzz-test` | Fuzz tests (10s per target) |
| `make ze-exabgp-test` | ExaBGP compatibility via `ze-test exabgp --all` |
| `make ze-test` | All tests including fuzz |
| `make ze-editor-test` | Editor `.et` tests (headless TUI) |
| `make ze-chaos-test` | Chaos unit + functional + integration + web |
| `make ze-race-reactor` | Stress race-test reactor (`-race -count=20`) -- REQUIRED when touching reactor concurrency code |
| `make ze-mutation-test` | Mutation testing via gomu on all non-excluded packages (advisory, slow) |
| `make ze-mutation-changed` | Incremental mutation testing on changed files only (advisory, fast) |
| `make ze-mutation-report` | Mutation testing with HTML report to `tmp/mutation-report.html` |
| `make ze-test-sensitivity-check` | Assert-nothing and tag-orphan ratchets (in `ze-verify`, both modes) |
| `make ze-test-health` | Regenerate `docs/features/test-health.md` + `test/health/latest.json` |
| `make ze-test-health-record` | Append one KPI sample to `test/health/history.ndjson` |

### Contended Run Verdicts

When `make ze-verify` runs on a loaded machine, the failure index may show
`VERIFY FAILURE INDEX (CONTENDED RUN)` with host load details. This means the
system had load > CPU count with concurrent ze-test or go-test processes.

How to read contended failures:
- `near_timeout` kind: the test consumed >80% of its timeout but the context
  deadline did not fire. This is CPU starvation, not a bug. Rerun on a quiet
  machine.
- `host-load` field in failure group JSON: load average, CPU count, and
  concurrent process counts at run start.
- Timing baseline updates are suppressed during contended runs to prevent
  slow-run pollution of the EMA.
- The project rejects retry-on-failure masking. Contended verdicts are for
  classification, not automatic retry.
<!-- source: internal/test/runner/hostload.go -- HostLoad, Contended, IsNearTimeout -->

### Linux-Only Tests (QEMU)

**Full rule: `ai/rules/qemu-testing.md`** (build tags, virtual substitutes,
Makefile wiring, reference implementations). Read it before writing any
`//go:build linux` code.

| Target | What it runs | When required |
|--------|-------------|---------------|
| `make ze-qemu-integration-test` | iface, config/system, fib/kernel, firewall/nft, firewall/vpp, traffic/netlink in QEMU Alpine VM | Any change to `//go:build linux` code |

### Capability-Requiring `.ci` Tests (Linux host, per-test netns)

| Target | What it runs | When required |
|--------|-------------|---------------|
| `make ze-netns-test` | `firewall` `policy` `ospf` `ospfv3` suites under `ZE_TEST_NETNS=1` | Any change to nft/FIB/OSPF kernel programming |
| `make ze-netns-plugin-test` | `show-system-kernel-log`, which needs CAP_SYSLOG to read `/dev/kmsg` | Any change to `readKmsg` |

Both setcap a **throwaway** binary, run under `sudo` with a per-test network
namespace, assert the host's kernel state is byte-identical before and after,
and exit non-zero (never skip) when Linux, `sudo`, or `setcap` is missing.
Details: `docs/functional-tests.md` "Netns launch mode".

**Prefer a knob that skips the work over a target that supplies the privilege.**
Five L2TP `test/plugin` tests used to sit in the second target; they now set
`ze.l2tp.disable-kernel-dataplane=true`, build no kernel worker, and pass
unprivileged. That was right because each asserts on the CLI surface and never on
the kernel's view, so nothing was lost. It is the WRONG move whenever the
privileged behaviour is the behaviour under test -- `show system kernel-log`
cannot be freed this way, and neither can
`test/l2tp/session-stopccn-cascade.ci`, which sets `skip-kernel-probe` and still
needs the data plane. Note those are two DIFFERENT knobs:
`skip-kernel-probe` bypasses the modprobe only.
<!-- source: mk/test-integration.mk -- ze-netns-test, ze-netns-plugin-test -->

**fakeOps pattern:** VPP backends use a `vppOps` interface seam so the Apply
pipeline can be tested with a scripted fake without a running VPP daemon. The
`apply_test.go` files are `//go:build linux` (they import linux-only binapi
types) but do NOT need real VPP. They run in QEMU alongside the integration
tests. See `internal/plugins/traffic/vpp/apply_test.go` for the reference
pattern.

### VPP Backend Testing Is Mandatory (BLOCKING)

Every VPP backend must ship with functional tests. No exceptions, no deferrals.

| Requirement | How |
|-------------|-----|
| Apply/Undo pipeline | `fakeOps` scripted tests in `apply_test.go` covering create, update, delete, partial-failure undo, and reconciliation |
| Translate functions | Pure-function unit tests in `translate_test.go` for every supported config shape |
| Verify/reject logic | `verify_test.go` asserting accepted configs pass and unsupported configs return clear errors |
| Registration side-effects | `register_test.go` confirming `init()` wires the backend into the correct registry |

"VPP needs a real daemon" is not a valid reason to skip tests. The `vppOps`
interface seam exists precisely so Apply logic can be tested without VPP.
Translate and Verify are pure functions with no VPP dependency at all. If a
new backend cannot be tested with the fakeOps pattern, that is a design
problem to fix before merging, not a deferral to log.

### Two-Pass Verification (how `ze-verify` works)

`ze-verify` uses a two-pass strategy to avoid recompiling all 349 packages with
`-race` every time:

1. **Lint** (full or changed-only depending on target)
2. **Cached full pass** (`go test` without `-race`): Go caches results by source hash.
   The pass uses `ze_core` plus the default-on feature tags from `feature-gates.txt`,
   matching the shipped `make ze` feature set. It also runs the bare `ze_core`
   hub compile-out checks so absent-feature tests still execute.
   When nothing changed, this completes in under 1 second. Catches logic regressions
   across the entire codebase.
3. **Race pass on changed groups only** (`go test -race` on component groups containing
   modified `.go` files): catches data races in what you touched, without recompiling
   everything. Group detection uses `scripts/dev/changed-groups.sh`.
4. **Functional tests** (13 suites via `ze-test`)
5. **ExaBGP compatibility**

Recorded full passes (`tmp/.ze-verify-duration.txt`) run 4-10 minutes;
a one-group change sits at the low end. Budget 10 minutes, not 2.

## Iteration Workflow (BLOCKING)

**One change, one test, then scale.** Never bulk-modify test files or source files without validating the pattern on a single case first.

**Specific before generic.** For code changes, start with the narrowest test
that can fail because of the changed file: direct Go test, matching `.ci`/`.et`
case, file-level test, feature test, or suite-local command. Then move outward
only after the narrower test passes. Do not spend CPU on unaffected packages or
whole suites before proving the affected code path works.

If a changed file has an associated test file, feature test, or suite test, run
that first. After it passes, run the next broader relevant scope, then the
remaining gate. Order is: direct test -> file/feature test -> package ->
component group -> whole suite or `ze-verify`.

| Step | Action | Command |
|------|--------|---------|
| 1 | Make the change in ONE file | Edit a single `.ci` or `.go` file |
| 2 | Run just that test | `ze-test bgp plugin N` or `go test -run TestName` |
| 3 | Investigate if it fails | Read output, understand the format, fix |
| 4 | Only then apply to remaining files | Repeat the pattern that worked |

**Targeted test commands for development:**

| Scope | Command | Speed |
|-------|---------|-------|
| Single functional test | `ze-test bgp plugin N` or `ze-test ui N` | seconds |
| Resume functional suite | `ze-test bgp plugin --start N` or `ze-test ui --start N` | seconds to remaining suite |
| Single encode test | `ze-test bgp encode N` | seconds |
| Single editor test | `ze-test editor N` or `ze-test editor --pattern <name>` | seconds |
| Single ExaBGP compatibility test | `ze-test exabgp N` or `ze-test exabgp --start N` | seconds |
| Single Go test | `go test -race -run TestName ./pkg/...` | seconds |
| Single package | `go test -race ./internal/component/bgp/reactor/...` | seconds |
| Component group | `make ze-test-bgp` (or core, plugins, config, cli, rest) | 10s-1:30 |
| All unit tests | `make ze-unit-test` | ~5 min |
| All editor tests | `make ze-editor-test` | ~30s |
| Pre-commit gate | `make ze-verify` | 4-10 min (see `tmp/.ze-verify-duration.txt`) |

**A numeric id is a position, not an identity (BLOCKING for anything you keep).**
`ze-test <suite> N` resolves `N` as a one-based ordinal over the sorted `.ci` glob,
so adding, renaming, or deleting an EARLIER file silently renumbers every test
after it. Ids are fine while you iterate inside one turn. They are not fine in
anything that outlives the turn -- a verification script, a gate subset, a
handover, a commit message claiming "8/8 green". A concurrent session added `.ci`
files mid-session and id 373 moved from `resolve-ping` to
`remove-private-as-replace-peer` while an id-driven script reported green for
tests it never ran.

| Use | Form |
|-----|------|
| Iterating right now, from a failure index you just read | `ze-test bgp plugin 145` |
| A script, a gate subset, a handover, a claim of evidence | `ze-test bgp plugin --pattern <name>` |

A positional selector matches a record's Nick, Name, or CIFile EXACTLY
(`indexRecordSelector`, `internal/test/runner/selection.go`), so passing names as
positional ids is as stable as `--pattern` and, unlike a substring pattern,
cannot widen. `scripts/evidence/netns_qemu.py` selects all four of its subsets by
name for exactly this reason, and its `assert_named` guard refuses to run a
subset that still carries a numeric selector -- a nick had already drifted there,
with firewall `"17"` resolving to `command-owner-firewall-root.ci` rather than to
any `017-*.ci`.

**Always spell `--pattern` in full: `-p` is a DIFFERENT flag in most suites.** `-p` is `--parallel` (an int) for `ze-test bgp <type>`, `ze-test exabgp`, `ze-test vpp` and every `.ci` suite on the shared runner (`internal/test/cli/cmd_bgp.go:560`, `cmd_exabgp.go:201`, `cmd_vpp.go:148`, `ci_runner.go:47`), and `--pattern` (a string) only for `ze-test editor` and `ze-test web` (`cmd_editor.go:31`, `cmd_web.go:80`); `--pattern` itself has no short form anywhere. So `ze-test bgp plugin -p rfc7606-relay-one-field` is not a filtered run, it is a parse failure -- exit 2, no output, no tests -- and it reads as "nothing to report" rather than as an error.

**A `ze.log.<subsystem>` key in a `.ci` test must name a real slog subsystem.**
An internal plugin's logger name is `CanonicalSubsystemName` of its registry name
(`internal/component/plugin/inprocess.go`), which turns every hyphen into a dot,
and `getLogEnv` (`internal/core/slogutil/slogutil.go`) splits the subsystem on
`.` only. So a plugin registered `bgp-adj-rib-in` reads `ze.log.bgp.adj.rib.in`;
`ze.log.bgp.adj-rib-in` matches no lookup, sets nothing, and leaves the level at
the WARN default -- with no error, which is why it has recurred three times. A
hyphen in the key is legitimate ONLY when that exact subsystem is declared
literally in Go (`slogutil.LazyLogger("bgp.filter.aspath-length")`). Enforced by
`check_ci_log_subsystem_keys` in `make ze-verify-wiring-docs`.

**Escalation ladder:** direct test -> file/feature test -> single package -> component group -> whole suite or `ze-verify`. If any rung fails, fix from that evidence and rerun the failed rung or a narrower failing test, not a wider suite.

`make ze-verify` is the **final gate**, not a development tool. Use targeted commands and component groups during iteration.
On failure, `make ze-verify` writes the compact index `tmp/ze-verify-failures.log`.
Read that file first. The next run MUST be the listed `Rerun` command for the
failed stage, or an even narrower single test/package from the detail log. If
multiple failures are listed, clear each one with its focused rerun. Only after
all focused reruns pass may you rerun the whole suite or gate as final
confirmation. The combined log is `tmp/ze-verify.log`, and automation can read
`tmp/ze-verify-failures.json`.

**Overlapping runs:** If a test run is failing, kill it before starting another. Never run `make ze-verify` twice concurrently.

**Understand before modifying:** Before bulk-editing `.ci` files or test files, run one test and read its output to understand the format and expected behavior. Assumptions about test syntax cause cascading failures across every modified file.

## Individual Commands

```bash
go test -race ./internal/component/bgp/message/... -v  # Single package
go test -race ./... -run TestName -v          # Single test
go test -race -cover ./...                    # Coverage
make ze-fuzz-one FUZZ=FuzzName TIME=30s       # Single fuzz target
```

## Timing Baseline

`ze-test` saves per-test timing to `tmp/test-timings.json` (rolling EMA, alpha=0.3).
After 3 samples, the baseline is used for two things:

**Auto-timeout:** Per-test timeout = min(global, max(5s, 5x baseline avg)). A test that normally takes 500ms gets a 5s timeout instead of the default 15s. Catches hangs in seconds, not minutes. Explicit `.ci` `timeout=` overrides always win.

**Slow detection:** Tests exceeding 2x baseline are flagged in the summary output. Investigate before ignoring.

## Test Tools

- `ze-peer`: BGP test peer (`--sink`, `--echo`, `--port`, `--asn`)
- `ze-test`: Test runner. Common suite syntax is `--list`, `--all`, `--start N`, `--pattern TEXT`, or positional `N...`; `--list` prints `N/TOTAL id name` with one-based ids, and runs print one completion line per test plus periodic progress.

When adding a test runner, test format, make target, or verification gate, update
`ai/rules/discovery-updates.md` paths in the same change: `ai/INDEX.md` for the
tool, `ai/INDEX.md` (task navigation) if it changes task selection, this file for required
usage, and `docs/architecture/testing/` or `docs/contributing/` for detailed
operator documentation.

## Testing Python Tooling (scripts/)

There is no `pytest` and no `unittest discover` in this repo. A Python test that
nothing invokes never runs, and reads as coverage while providing none. Eight
`scripts/dev/*_test.py` files sat unexecuted this way until 2026-07-16. Use one of
the two wired conventions, never a bare test file plus hope:

| Your tool | Convention | Runs because |
|-----------|-----------|--------------|
| Has its own unit tests | Name them `<tool>_test.py` (unittest, with `unittest.main()`) and put them BESIDE the tool -- `scripts/dev/`, `test/scripts/`, or `test/perf/` | `TestPythonUnitTests` (`scripts/dev/python_tests_test.go`) globs `*_test.py` under EVERY root in `pythonTestRoots` and runs each. A new file in an existing root is picked up automatically; a Python tool in a NEW directory needs its root added there first, or its tests never run. Each root carries its own non-empty assertion so a root that stops contributing fails loudly rather than silently covering nothing |
| Wants fixture tests inside the script | Add a `--selftest` flag, then a small Go test that shells out to it | The pattern of `dep_audit.py`, `migrate_module.py`, `qemu-run.py`. See `scripts/dev/migrate_module_test.go` |

Both land inside `go test`, so `make ze-unit-test` covers them via `go list ./...`
and no make target is needed. `scripts/dev` and `scripts/evidence` are test-only Go
packages that exist for exactly this.

Do not add a `*_test.py` outside a directory covered by one of the above without
wiring it, and do not "fix" a discovery glob by replacing it with a hardcoded list:
the glob is what stops the next file from rotting.

## Temporary Files

Use project `tmp/` (gitignored) for scratch files — never `/tmp`.
Create a subfolder per debugging task (e.g., `tmp/watchdog-debug/`) to keep artifacts isolated.

**Prefer your session's own directory**: `dir=$(scripts/dev/session-scratch.sh)` gives
`tmp/s/<session-id>/`, which is removed at SessionEnd, so scratch cannot outlive its
owner or collide with a sibling session (`ai/rules/bash-output.md`).

The functional-test runner already writes there: its per-run and per-test working
directories (configs, sockets, daemon pid/ready files) root at
`sessionpath.DefaultScratchRoot()` / `EnsureScratchRoot(baseDir)` when a session is
active, instead of the unowned `$TMPDIR/ze-functional-*` they used before
(`internal/test/sessionpath`, `internal/test/runner/runner.go`). Off-session the
runner still uses the system temp dir, unchanged.

## Debugging Failures

Read the failure index before opening full logs or re-running.
After a suite or gate fails, the next test command MUST target
only the failing part: a single `.ci`/`.et` case, single Go test, single
package, or the stage-local `Rerun` command from the failure index. If there
are multiple failures, clear each one with its focused rerun. Only after all
focused reruns pass may you rerun the whole suite or gate as final
confirmation, except when the suite is the only available reproduction.

```bash
make ze-verify
# On failure, read:
tmp/ze-verify-failures.log
```

Use each group's `Rerun` command for the smallest useful scope. Open the
group's `Detail log` only after choosing the group. On success: one final pass
line plus fresh artifacts. Never `| tail`.

## Editor Tests (.et format)

`.et` files in `test/editor/` test the interactive TUI editor via headless simulation.
Infrastructure: `internal/component/cli/testing/` (parser, expect, headless, input, runner).
Run: `make ze-editor-test` or `bin/ze-test editor --all`; select by id/name with `bin/ze-test editor N`, and filter with `bin/ze-test editor --pattern <name>`.

### Directives

| Directive | Purpose | Example |
|-----------|---------|---------|
| `tmpfs=<path>:terminator=<TERM>` | Embedded config file | `tmpfs=test.conf:terminator=EOF` |
| `option=file:path=<name>` | Config file to load (required) | `option=file:path=test.conf` |
| `option=timeout:value=<dur>` | Test timeout (default 30s) | `option=timeout:value=10s` |
| `option=width:value=N` | Editor width (default 80) | `option=width:value=120` |
| `option=height:value=N` | Editor height (default 24) | `option=height:value=30` |
| `option=reload:mode=success\|fail` | Mock reload notifier | `option=reload:mode=success` |
| `option=monitor:ping=fake` | Deterministic ping monitor + fake PTR/origin resolvers (offline pipe-enrichment tests; see `internal/component/cli/testing/fake_monitor.go`) | `option=monitor:ping=fake` |
| `option=session:user=X:origin=Y` | Session identity | `option=session:user=alice:origin=ssh` |
| `session=<name>` | Switch to named session | `session=bob` |
| `input=type:text=<string>` | Type text | `input=type:text=show` |
| `input=<keyname>` | Press key | `input=enter`, `input=tab`, `input=up` |
| `input=ctrl:key=<char>` | Ctrl+key | `input=ctrl:key=c` |

**Named keys:** `tab`, `enter`, `esc`, `up`, `down`, `left`, `right`, `backspace`, `delete`, `home`, `end`, `pgup`, `pgdn`, `space`, `shift+tab`

### Expectations

| Type | Example | What it checks |
|------|---------|----------------|
| `expect=input:value=<text>` | `expect=input:value=show` | Text input buffer |
| `expect=input:empty` | | Input is empty |
| `expect=context:root` | | At root context |
| `expect=context:path=bgp.peer` | | At nested context |
| `expect=dirty:true\|false` | | Unsaved changes |
| `expect=error:none\|contains=<text>` | | Command error state |
| `expect=status:contains=<text>\|empty` | | Status message |
| `expect=mode:is=config\|operational` | | Editor mode |
| `expect=completion:contains=a,b` | | Tab completions include items |
| `expect=completion:empty\|count=N\|exact=a,b` | | Completion list state |
| `expect=ghost:text=<text>\|empty` | | Ghost text preview |
| `expect=content:contains=<text>` | | Config content |
| `expect=viewport:contains=<text>` | | Displayed output |
| `expect=dropdown:visible\|hidden` | | Dropdown shown |
| `expect=file:path=<rel>:contains=<text>` | | On-disk file content |
| `expect=file:path=<rel>:absent` | | File does not exist |
| `expect=timer:active\|inactive` | | Commit confirm timer |
| `expect=errors:count=N\|contains=<text>` | | Validation errors |
| `expect=warnings:count=N\|contains=<text>` | | Validation warnings |
| `expect=prompt:contains=<text>` | | Prompt text |

### When to use .et vs .ci vs Go tests

| Test need | Format | Why |
|-----------|--------|-----|
| TUI behavior (keystrokes, completions, history) | `.et` | Headless model simulates real TUI |
| BGP wire, config parsing, CLI commands | `.ci` | Process-level testing |
| Internal logic, persistence wiring | Go `_test.go` | Direct API access |

### Structure

Tests organized by concern in `test/editor/`: `commands/`, `completion/`, `lifecycle/`, `mode/`, `navigation/`, `pipe/`, `session/`, `validation/`, `workflow/`.

## Common Flaky Test Causes

| Symptom | Root Cause | Fix |
|---------|-----------|-----|
| Port reuse race in reactor tests | `Stop()` not waiting for cleanup | Ensure cleanup goroutines complete before returning |
| Completion test fails intermittently | Real bug, not flaky | Check `completeShowPath` includes YANG schema children |
| Inter-message timing in plugin tests | Sleep too tight under load | Increase inter-message delay or use synchronization |

Flake-shape catalogue (locked-write/unlocked-read, subscribe-before-broadcast,
gate-handler queue state, barrier FIFO, cleanup-drains-work, fixed-port
SO_REUSEPORT gate, test-fake pool IDs): `plan/learned/608-concurrent-test-patterns.md`.
Read it before investigating a new race or isolation flake.

## Reactor Concurrency Code (BLOCKING)

When touching `internal/component/bgp/reactor/session*.go`, `forward_pool*.go`,
`peer.go`, or any other reactor file that holds locks or shares state across
goroutines, the standard `-race -count=1` unit run is **not enough**. The
bufReader/bufWriter races (`d5843235`, `8dffd422`) lived 47 days because the
schedule that triggered them was rare. Run `make ze-race-reactor` (`-race
-count=20`) before claiming the change done.

| Touched | Required verification |
|---------|----------------------|
| `session*.go` lock acquire/release, field assign | `make ze-race-reactor` |
| `forward_pool*.go` worker drain or buffer release | `make ze-race-reactor` |
| New goroutine in reactor package | `make ze-race-reactor` |
| Any reactor field shared between Run loop and other goroutines | `make ze-race-reactor` |
| Reactor doc-only edits, log message changes | Not required |

A passing `ze-unit-test` is NOT proof that a reactor concurrency change is
race-free. Paste the `ze-race-reactor` output as evidence.

## Observer-Exit Antipattern in `.ci` Tests (BLOCKING)

Python observer plugins inside `tmpfs=*.run` blocks MUST NOT use the
`dispatch(api, 'daemon shutdown') ; sys.exit(1)` pattern to signal failure.
The runner only watches ze's exit code, and ze has already exited 0 from the
clean shutdown by the time the observer's `sys.exit(1)` runs. The test passes
silently. The cmd-4 fix (`1fc98747`) removed three such false-positives.

**Use `runtime_fail` instead.** `test/scripts/ze_api.py` provides
`runtime_fail(message)` which emits the `ZE-OBSERVER-FAIL` sentinel that the
runner detects via `validateLogging` (`internal/test/runner/runner_validate.go`).

| Bad | Good |
|-----|------|
| `print('FAIL: ...', file=sys.stderr); sys.exit(1)` | `from ze_api import runtime_fail; runtime_fail('reason')` |
| Relying on `expect=exit:code=0` to catch observer failures | Adding explicit `expect=stderr:pattern=` on production logs the plugin emits |
| `time.sleep(N)` then "INFO: filter not called" with no failure path | `runtime_fail` if the expected event did not arrive |

**Equivalent positive assertions also work.** The cmd-4 fix took the second
route: it asserted `expect=stderr:pattern=prefix-list accept` plus
`reject=stderr:pattern=prefix-list reject` on production log lines emitted by
`bgp-filter-prefix`. That is the strongest pattern because it verifies the
production code path, not the observer.

| Pattern | When to use |
|---------|------------|
| `expect=stderr:pattern=<production log line>` + `reject=stderr:pattern=<wrong outcome>` | Plugin emits a decision log on every iteration. **Preferred.** |
| `runtime_fail(...)` from observer when assertion fails | Observer must compute something the engine cannot log directly |
| Rely on `expect=exit:code=0` alone with a Python observer | Forbidden -- silent false positive |

Detection hook: `c_observer_sys_exit` in `.claude/hooks/pretool-writeedit.py`
(warns on Write/Edit of `.ci` files containing `tmpfs=*.run` Python with
`sys.exit(1)` and no `runtime_fail`).

**Sleep ratchet (BLOCKING):** the total `time.sleep(` count across
`test/**/*.ci` may only go down. The committed baseline lives in
`test/.ci-sleep-baseline`; `make ze-verify-wiring-docs` fails when the count
exceeds it. Use `ze_api` `wait_for_event` / `wait_for_shutdown` / `wait_until` /
`dispatch_until` (the payload-predicate waits, below) instead of sleeps (sleeps
hide real races). When your change removes sleeps, lower the baseline in the same
change. Known violations are tracked in `plan/known-failures/`
and must be migrated.

**Sleep justification (BLOCKING):** every `time.sleep(` that the ratchet tolerates
MUST carry a comment (on the line above, or trailing) explaining why it is there /
why it was not converted to a deterministic wait. `make ze-verify-wiring-docs`
fails on any unjustified sleep in a changed `.ci`. See
`ai/rules/ci-sleep-justification.md`.

## Python Observer API (`test/scripts/ze_api.py`)

Python plugins embedded in `.ci` tests via `tmpfs=*.py` can import `ze_api` for
the 5-stage plugin protocol and runtime assertions. Key functions:

| Function | Purpose |
|----------|---------|
| `ready()` | Complete all 5 stages, enter event loop (simple usage) |
| `send(cmd)` | Send a text command to the engine |
| `dispatch(api, cmd)` | Send command via API connection |
| `runtime_fail(msg)` | Signal assertion failure (replaces `sys.exit(1)`) |
| `wait_for_shutdown()` | Block until engine shuts down |
| `wait_for_event(timeout, predicate=None)` | Wait for the next event, or (with `predicate`) the first event whose decoded form satisfies it |
| `wait_until(predicate, attempts=20, delay=0.25)` | Poll an arbitrary `predicate()` (e.g. kernel FIB state) until true; returns bool |
| `dispatch_until(api, cmd, predicate, ...)` | Re-dispatch `cmd` until `predicate(result)` is true; returns the winning result dict (also `api.dispatch_until(cmd, predicate, ...)`) |
| `dispatch_until_done(cmd, ...)` | `dispatch_until` with the fixed `status=="done"` predicate |
| `run_rs_observer(expected_peers, forward_prefix=None)` | The standard route-server observer, one line: handshake, wait (event-driven) until every peer's EOR (and `forward_prefix`'s route, when given) is on the wire, then fire-and-forget shutdown. Load-robust successor to the `show bgp summary` `eor-sent` poll |
| `wait_rs_replayed(expected_peers, forward_prefix=None)` | The readiness half of `run_rs_observer`: block on the async event stream until N EORs (and optionally a route carrying `forward_prefix`) are sent. Returns bool |
| `shutdown_fire_and_forget()` | Send `request shutdown` without blocking on its RPC response (ze may close the connection before replying under load) |

**Prefer `run_rs_observer` for any route-server `.ci`.** The old copy-pasted
`all_peers_eor_sent` poll drove synchronous `show bgp summary` dispatch RPCs whose
30s TLS read could stall under load while the engine forwarded fine, stranding the
shutdown until the outer timeout killed ze. `run_rs_observer` waits on pushed events
instead (no request/response to stall on) and shuts down fire-and-forget.

`wait_until` / `dispatch_until` / `wait_for_event(predicate)` are the
payload-predicate waits: prefer them over `time.sleep` + a single-shot assert so
a test blocks exactly until the observed payload matches, not a guessed duration.

Full protocol usage: `API()` class with `declare_family()`, `declare_done()`,
`wait_for_config()`, `capability_done()`, `wait_for_registry()`, `ready()`.

Source: `test/scripts/ze_api.py` (docstring has examples).
<!-- source: test/scripts/ze_api.py -- wait_until, dispatch_until, dispatch_until_done, wait_for_event -->

First-class `.ci` engine steps have the symmetric declarative form
(`expect=output:matches=`/`absent=`/`json=`); see
`docs/architecture/testing/ci-format.md` "Engine Steps".

## Mutation Testing

Mutation testing uses [gomu](https://github.com/sivchari/gomu) to verify that
tests actually catch code changes. It modifies the AST (arithmetic, conditional,
logical, bitwise, branch, return value, error handling operators) and checks
whether the test suite detects each mutation. Advisory only, never gates
`ze-verify`.

gomu is vendored in `tools.go` and invoked via `go run`. No install needed.

| Target | Purpose |
|--------|---------|
| `make ze-mutation-test` | Full run on all non-excluded packages (slow) |
| `make ze-mutation-changed` | Incremental, changed files only (fast) |
| `make ze-mutation-report` | Full run with HTML report to `tmp/mutation-report.html` |

Tuning via environment: `GOMU_WORKERS` (default: `GO_TEST_PROCS`),
`GOMU_TIMEOUT` (default: 120s per test), `GOMU_THRESHOLD` (default: 0%).

gomu has no `--tags` support. Files with custom build tags (`ze_test`,
`ze_chaos`, `ze_perf`, `ze_analyze`) and `cmd/ze/` are excluded via
`.gomuignore`. Reports go to `tmp/` (gitignored).

## Pre-Commit

See `ai/rules/git-safety.md` for the full pre-commit workflow.

`make ze-verify` is the ONLY acceptable pre-commit verification. Not `go test`. Not any subset.
During development: `go test`, component groups (`make ze-test-bgp`), `make ze-unit-test` are fine for fast iteration.
