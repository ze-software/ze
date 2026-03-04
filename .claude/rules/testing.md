---
paths:
  - "*"
---

# Testing

Rationale: `.claude/rationale/testing.md`

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
| `make ze-verify` | All tests except fuzz (development) |
| `make ze-ci` | lint + unit + build |
| `make ze-fuzz-test` | Fuzz tests (15s per target) |
| `make ze-exabgp-test` | ExaBGP compatibility |
| `make ze-test` | All tests: lint + unit + functional + exabgp + chaos + fuzz (before commits) |
| `make ze-chaos-test` | Chaos unit + functional + web |

## Individual Commands

```bash
go test -race ./internal/bgp/message/... -v  # Single package
go test -race ./... -run TestName -v          # Single test
go test -race -cover ./...                    # Coverage
make ze-fuzz-one FUZZ=FuzzName TIME=30s       # Single fuzz target
```

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

**BLOCKING:** `make ze-test` is the ONLY acceptable pre-commit verification. Not `make ze-verify`. Not `go test`. Not any subset.

**BLOCKING:** Never ask to commit without reporting ALL test failures to the user first. If any test failed, list every failure explicitly before any commit discussion. Hiding, omitting, or glossing over failures is forbidden.

```
[ ] make ze-test — capture to tmp/ze-test.log (timeout 300s)
[ ] Report test result: pass/fail. If failures: list every one. No omissions.
[ ] Keep output concise — failures and summary only, not the full log.
[ ] User approval
```

During development: `go test`, `make ze-unit-test`, `make ze-verify` are fine for fast iteration.
Before committing or claiming done: `make ze-test`. No exceptions.
