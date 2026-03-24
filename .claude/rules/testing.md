---
paths:
  - "*"
---

# Testing

Rationale: `.claude/rationale/testing.md`

## Fix Code, Not Tests

**BLOCKING:** When a test fails, fix the code to make the test pass. NEVER weaken or simplify test expectations to match broken code. Tests are ground truth. Even if an underlying mechanism changed (e.g., Unix sockets replaced by SSH), the test expectations stay and the replacement mechanism must satisfy them.

## No Throw-Away Tests

**BLOCKING:** Never write temporary test code. Add functional or unit tests that run in CI.

| Situation | Location | Format |
|-----------|----------|--------|
| Valid config parses | `test/parse/` | `.ci` with `expect=exit:code=0` |
| Invalid config fails | `test/parse/` | `.ci` with `expect=exit:code=1` + `expect=stderr:contains=` |
| BGP encoding | `test/encode/` | Config + expectations |
| Plugin behavior | `test/plugin/` | Config + expectations |
| Wire decoding | `test/decode/` | stdin + cmd + `expect=json:` |
| Internal logic | `internal/<pkg>/<file>_test.go` | Go test file |

## Make Targets

| Target | Purpose |
|--------|---------|
| `make ze-unit-test` | Unit tests with race detector |
| `make ze-functional-test` | All functional tests |
| `make ze-lint` | 26 linters |
| `make ze-verify` | All tests except fuzz (before commits) |
| `make ze-ci` | lint + unit + build |
| `make ze-fuzz-test` | Fuzz tests (15s per target) |
| `make ze-exabgp-test` | ExaBGP compatibility |
| `make ze-test` | All tests including fuzz (use when specifically needed) |
| `make ze-chaos-test` | Chaos unit + functional + web |

## Iteration Workflow (BLOCKING)

**One change, one test, then scale.** Never bulk-modify test files or source files without validating the pattern on a single case first.

| Step | Action | Command |
|------|--------|---------|
| 1 | Make the change in ONE file | Edit a single `.ci` or `.go` file |
| 2 | Run just that test | `ze-test bgp plugin N` or `go test -run TestName` |
| 3 | Investigate if it fails | Read output, understand the format, fix |
| 4 | Only then apply to remaining files | Repeat the pattern that worked |

**Targeted test commands for development:**

| Scope | Command | Speed |
|-------|---------|-------|
| Single functional test | `ze-test bgp plugin N` | seconds |
| Single encode test | `ze-test bgp encode N` | seconds |
| Single Go test | `go test -race -run TestName ./pkg/...` | seconds |
| Single package | `go test -race ./internal/component/bgp/reactor/...` | seconds |
| All unit tests | `make ze-unit-test` | fast |
| Pre-commit gate | `make ze-verify` | ~2 min |

`make ze-verify` is the **final gate**, not a development tool. Use targeted commands during iteration.

**Overlapping runs:** If a test run is failing, kill it before starting another. Never run `make ze-verify` twice concurrently.

**Understand before modifying:** Before bulk-editing `.ci` files or test files, run one test and read its output to understand the format and expected behavior. Assumptions about test syntax cause cascading failures across every modified file.

## Individual Commands

```bash
go test -race ./internal/bgp/message/... -v  # Single package
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
- `ze-test`: Test runner (`ze-test bgp encode --list`, `--all`, by index)

## Temporary Files

Use project `tmp/` (gitignored) for scratch files — never `/tmp`.
Create a subfolder per debugging task (e.g., `tmp/watchdog-debug/`) to keep artifacts isolated.

## Debugging Failures

**BLOCKING:** Capture output. Search the log — don't re-run the suite.

```bash
make ze-verify > tmp/ze-test.log 2>&1 || grep -E "^--- FAIL|^FAIL|TEST FAILURE|✗|═══ FAIL" tmp/ze-test.log
```

On failure: search the log. On success: one line of exit status. Never `| tail`.

## Pre-Commit

See `rules/git-safety.md` for the full pre-commit workflow.

`make ze-verify` is the ONLY acceptable pre-commit verification. Not `go test`. Not any subset.
During development: `go test`, `make ze-unit-test` are fine for fast iteration.
