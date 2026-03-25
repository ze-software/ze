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
| Editor/TUI behavior | `test/editor/` | `.et` with `input=`/`expect=` directives |
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
| `make ze-editor-test` | Editor `.et` tests (headless TUI) |
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
| Single editor test | `ze-test editor -p pattern` | seconds |
| Single Go test | `go test -race -run TestName ./pkg/...` | seconds |
| Single package | `go test -race ./internal/component/bgp/reactor/...` | seconds |
| All unit tests | `make ze-unit-test` | fast |
| All editor tests | `make ze-editor-test` | ~30s |
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

## Editor Tests (.et format)

`.et` files in `test/editor/` test the interactive TUI editor via headless simulation.
Infrastructure: `internal/component/cli/testing/` (parser, expect, headless, input, runner).
Run: `make ze-editor-test` or `bin/ze-test editor [-p pattern] [-v] [-l]`.

### Directives

| Directive | Purpose | Example |
|-----------|---------|---------|
| `tmpfs=<path>:terminator=<TERM>` | Embedded config file | `tmpfs=test.conf:terminator=EOF` |
| `option=file:path=<name>` | Config file to load (required) | `option=file:path=test.conf` |
| `option=timeout:value=<dur>` | Test timeout (default 30s) | `option=timeout:value=10s` |
| `option=width:value=N` | Editor width (default 80) | `option=width:value=120` |
| `option=height:value=N` | Editor height (default 24) | `option=height:value=30` |
| `option=reload:mode=success\|fail` | Mock reload notifier | `option=reload:mode=success` |
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
| `expect=mode:is=edit\|command` | | Editor mode |
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

## Bash Tool Timeouts

| Command | Timeout | Why |
|---------|---------|-----|
| Default | 15000ms | Bash tool default |
| `make ze-unit-test` | 120s | Longer than default |
| `make ze-verify` | 180s | Runs lint + unit + functional + exabgp; regularly takes over 1m50s |

## Common Flaky Test Causes

| Symptom | Root Cause | Fix |
|---------|-----------|-----|
| Port reuse race in reactor tests | `Stop()` not waiting for cleanup | Ensure cleanup goroutines complete before returning |
| Completion test fails intermittently | Real bug, not flaky | Check `completeShowPath` includes YANG schema children |
| Inter-message timing in plugin tests | Sleep too tight under load | Increase inter-message delay or use synchronization |

## Pre-Commit

See `rules/git-safety.md` for the full pre-commit workflow.

`make ze-verify` is the ONLY acceptable pre-commit verification. Not `go test`. Not any subset.
During development: `go test`, `make ze-unit-test` are fine for fast iteration.
