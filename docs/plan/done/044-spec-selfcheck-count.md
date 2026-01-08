# Spec: selfcheck --count

## SOURCE FILES (read before implementation)

```
┌─────────────────────────────────────────────────────────────────┐
│  Read these source files before implementing:                   │
│                                                                 │
│  1. test/cmd/selfcheck/main.go - CLI parsing, main flow        │
│  2. test/selfcheck/runner.go - Runner, Run(), runTest()        │
│  3. test/selfcheck/record.go - Record, State, Tests            │
│  4. test/selfcheck/display.go - Display, Summary()             │
│  5. docs/plan/spec-functional-test-diagnostics.md:516-545 - P2 spec │
│                                                                 │
│  NOTE: Protocol files should have been read at SESSION START.   │
│                                                                 │
│  ON COMPLETION: Update docs/plan/CLAUDE_CONTINUATION.md              │
└─────────────────────────────────────────────────────────────────┘
```

## Task

Add `--count N` flag to selfcheck for stress testing:
- Run each selected test N times regardless of pass/fail
- Report statistics: pass/fail count per test, min/avg/max time
- Useful for detecting flaky tests

## Current State

- Tests: 32 passed, 5 failed/timeout
- Git: clean

## Context Loaded

```
📖 Context loaded:
  ✅ test/cmd/selfcheck/main.go:1-287 - CLI parsing
  ✅ test/selfcheck/runner.go:1-391 - Runner implementation
  ✅ test/selfcheck/record.go:1-563 - Record/Tests types
  ✅ test/selfcheck/display.go:1-225 - Display logic

🔍 Patterns:
  - RunOptions struct for config (runner.go:16)
  - Record.Duration for timing (record.go:81)
  - Tests.Summary() for counts (record.go:326)
```

## Problem Analysis

**Goal:** Detect flaky tests by running tests multiple times.

**Current flow:**
1. main.go parses CLI → cliFlags struct
2. Creates Runner with tests
3. Calls runner.Run(ctx, opts) once
4. Display.Summary() prints results

**Changes needed:**
1. Add `count int` to cliFlags (main.go)
2. Add `Count int` to RunOptions (runner.go)
3. Add `RunWithCount()` method or modify Run() (runner.go)
4. Add `IterationStats` type for per-test stats (new file or record.go)
5. Add `StressSummary()` display method (display.go)

## Goal Achievement

```
🎯 User's goal: Run tests N times, report statistics

| Check | Status |
|-------|--------|
| --count N flag | ⏳ TODO |
| Multiple iterations | ⏳ TODO |
| Stats collection | ⏳ TODO |
| Stats display | ⏳ TODO |

Plan achieves goal: YES (pending implementation)
```

## Embedded Rules

- TDD: test must fail before impl
- Verify: make test && make lint before done
- No RFC relevance (internal tooling)

## Documentation Impact

- [ ] `docs/plan/CLAUDE_CONTINUATION.md` - update after impl

## Implementation Steps

### Phase 1: Tests

1. Create `test/selfcheck/stress_test.go`:
   - TestIterationStats_Add
   - TestIterationStats_Summary
   - TestRunWithCount_SingleIteration
   - TestRunWithCount_MultipleIterations

2. Run tests → must fail

### Phase 2: Types

1. Add to `test/selfcheck/stress.go`:
```go
// IterationStats tracks results across multiple runs.
type IterationStats struct {
    Nick      string
    Passed    int
    Failed    int
    TimedOut  int
    Durations []time.Duration
}

func (s *IterationStats) Add(state State, duration time.Duration)
func (s *IterationStats) Min() time.Duration
func (s *IterationStats) Max() time.Duration
func (s *IterationStats) Avg() time.Duration
func (s *IterationStats) PassRate() float64
```

2. Run tests → verify types compile

### Phase 3: Runner

1. Add `Count int` to `RunOptions` (runner.go:16)
2. Add `RunWithCount(ctx, opts) map[string]*IterationStats` to Runner
3. Implement iteration loop with state reset

### Phase 4: Display

1. Add `StressSummary(stats map[string]*IterationStats)` to Display
2. Format: Nick | Passed | Failed | Min | Avg | Max | Rate

### Phase 5: CLI

1. Add to cliFlags: `count int`
2. Add flag: `fs.IntVar(&cli.count, "count", 1, "run each test N times")`
3. If count > 1, call RunWithCount instead of Run

### Phase 6: Verification

```bash
make test && make lint
go run ./test/cmd/selfcheck encoding --count 3 0
```

## Checklist

- [ ] Tests fail first
- [ ] Tests pass after impl
- [ ] make test passes
- [ ] make lint passes
- [ ] Goal achieved
- [ ] docs/plan/CLAUDE_CONTINUATION.md updated
- [ ] Spec moved to docs/plan/done/
