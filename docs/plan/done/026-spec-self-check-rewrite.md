# Spec: Rewrite self-check to match ExaBGP functional testing

## Task

Rewrite ZeBGP's `test/cmd/self-check` to align with ExaBGP's `qa/bin/functional` and `qa/bin/test_everything` architecture from the main branch.

---

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof
- Tests passing is NOT permission to commit - wait for user

### From ESSENTIAL_PROTOCOLS.md
- Check ExaBGP reference in `/Users/thomas/Code/github.com/exa-networks/exabgp/main/` before implementing
- Use agents for multi-file work to keep context low
- ONE function/type at a time during refactoring - no batching
- Post-completion self-review is MANDATORY

### From TDD_ENFORCEMENT.md
- Write test specifications before implementation
- Show test failure before showing implementation
- Document tests with VALIDATES/PREVENTS comments

---

## Current State Analysis

### ZeBGP Current (`test/cmd/self-check/`) - 705 lines
| Component | Lines | Purpose |
|-----------|-------|---------|
| main.go | 705 | All-in-one: discovery, runner, output |
| zebgp-peer | 144 | CLI wrapper for testpeer |
| internal/test/peer | 1,209 | Peer + checker + decoder |

**Limitations:**
- Single file, no separation of concerns
- No state machine for test lifecycle
- Crude process management (SIGKILL only)
- No timing/performance tracking
- No stress testing support
- No verbose/debug modes per test
- Fixed 30s timeout, no configurability
- Max 4 concurrent (hardcoded)

### ExaBGP Current (`qa/bin/functional`) - 2,902 lines
| Component | Lines | Purpose |
|-----------|-------|---------|
| State enum | ~50 | Test lifecycle (NONE→STARTING→RUNNING→SUCCESS/FAIL) |
| Record class | ~80 | Test metadata (nick, name, state, files) |
| Exec class | ~150 | Process wrapper (run, collect, terminate) |
| Tests container | ~200 | Test discovery, selection, iteration |
| EncodingTests | ~400 | Server-client model for encode tests |
| APITests | ~100 | Inherits EncodingTests with API specifics |
| DecodingTests | ~300 | Decode-only tests (no daemon) |
| Performance | ~150 | Timing cache, ETA calculation |
| Process mgmt | ~200 | psutil-based cleanup, graceful shutdown |
| CLI | ~300 | Rich argument parsing, modes |

**Advanced Features:**
- Per-test timing with 5-run rolling average
- Stress testing (`--stress N` runs test N times)
- Debug mode (`--debug test1 test2` verbose for specific tests)
- Edit mode (`--edit test` opens in $EDITOR)
- Dry run (`--dry` shows commands)
- Server/client separation (`--server`/`--client` for debugging)
- Automatic stale process cleanup
- File descriptor limits checking

---

## Architecture Design

### Directory Structure
```
test/
├── cmd/
│   ├── functional/         # Main test runner (Go, replaces self-check)
│   │   └── main.go
│   ├── test-everything/    # Full suite runner (Go)
│   │   └── main.go
│   └── zebgp-peer/         # Test peer (unchanged)
│       └── main.go
├── data/
│   ├── encode/             # Encode tests (unchanged)
│   │   ├── *.ci
│   │   └── *.conf
│   ├── api/                # API tests (unchanged)
│   │   ├── *.ci
│   │   ├── *.conf
│   │   └── *.run
│   └── decode/             # Future: decode-only tests
│       └── *.txt
└── internal/                    # Shared test infrastructure (new)
    ├── state.go            # State enum
    ├── record.go           # Test metadata
    ├── exec.go             # Process wrapper
    ├── tests.go            # Test container
    ├── timing.go           # Performance tracking
    ├── encoding.go         # Encoding test type
    ├── api.go              # API test type
    └── cli.go              # CLI parsing
```

### Core Types (test/internal/)

```go
// state.go
type State int
const (
    StateNone State = iota
    StateStarting
    StateRunning
    StateSuccess
    StateFail
    StateTimeout
    StateSkip
)

// record.go
type Record struct {
    Nick   string   // Single-char identifier (0-9, A-Z, a-z)
    Name   string   // Full test name
    State  State    // Current state
    Conf   *Config  // Parsed test configuration
    Files  []string // Source files (for --edit)
    Timing Timing   // Performance data
}

// exec.go
type Exec struct {
    Cmd     *exec.Cmd
    Stdout  bytes.Buffer
    Stderr  bytes.Buffer
    Started time.Time
    Timeout time.Duration
}

func (e *Exec) Run(ctx context.Context) error
func (e *Exec) Collect() (exitCode int, err error)
func (e *Exec) Terminate() error  // SIGTERM then SIGKILL

// tests.go
type Tests struct {
    byNick  map[string]*Record
    ordered []string
}

func (t *Tests) Discover(dir string, pattern string) error
func (t *Tests) Selected(nicks []string) []*Record
func (t *Tests) RunSelected(ctx context.Context, opts *RunOptions) bool

// timing.go
type Timing struct {
    Expected time.Duration
    Actual   time.Duration
    History  []time.Duration // Last 5 runs
}

type TimingCache struct {
    path  string
    times map[string]*Timing
}

func LoadTimingCache() (*TimingCache, error)
func (tc *TimingCache) Save() error
func (tc *TimingCache) Update(name string, duration time.Duration)
```

### CLI Interface (test/cmd/functional)

```
Usage: functional <type> [options] [tests...]

Types:
  encoding    Run encoding tests (static routes)
  api         Run API tests (dynamic routes via .run scripts)
  decoding    Run decoding tests (future)

Options:
  --list, -l          List available tests
  --all               Run all tests
  --verbose, -v       Show command output
  --debug, -d tests   Verbose only for specified tests
  --quiet, -q         Minimal output
  --timeout N         Set timeout in seconds (default: 30)
  --stress N          Run test N times (encoding only)
  --save DIR          Save logs to directory
  --edit test         Open test files in $EDITOR
  --dry               Show commands without running
  --server            Run server only (debugging)
  --client            Run client only (debugging)
  --parallel N        Max concurrent tests (default: 4)

Examples:
  go run ./test/cmd/functional encoding --list
  go run ./test/cmd/functional encoding 0 1 2
  go run ./test/cmd/functional encoding --all --verbose
  go run ./test/cmd/functional api --debug ae af
  go run ./test/cmd/functional encoding --stress 10 ebgp
```

### Test Execution Model

```
┌─ Discovery
│  1. Glob test/data/<type>/*.ci
│  2. Parse option:file: directives
│  3. Assign nicks (0-9, A-Z, a-z)
│  4. Create Record for each test
│
├─ Selection
│  1. Parse CLI args (nicks or --all)
│  2. Filter to selected tests
│  3. Load timing cache for ETA
│
├─ Execution (concurrent, limited by --parallel)
│  For each test:
│    1. State → Starting
│    2. Start zebgp-peer (test peer) on random port
│    3. Start zebgp daemon with config
│    4. State → Running
│    5. Wait for completion or timeout
│    6. Collect output, determine result
│    7. State → Success/Fail/Timeout
│    8. Update timing cache
│
└─ Reporting
   1. Show results (✓/✗ with color)
   2. On failure: show stdout/stderr
   3. Save logs if --save
   4. Exit 0 if all pass, 1 otherwise
```

---

## Implementation Steps

### Phase 1: Core Infrastructure (test/internal/)

1. **Create state.go**
   - State enum with String() method
   - State transition validation

2. **Create record.go**
   - Record struct
   - Config parsing from .ci files
   - Nick assignment algorithm

3. **Create exec.go**
   - Process wrapper with context support
   - Graceful termination (SIGTERM → SIGKILL)
   - Output capture

4. **Create tests.go**
   - Test container with discovery
   - Selection by nicks
   - Concurrent execution with semaphore

5. **Create timing.go**
   - Timing cache (~/.cache/zebgp/test_times.json)
   - Load/save/update operations
   - ETA calculation

### Phase 2: Test Types

6. **Create encoding.go**
   - EncodingTest struct (embeds Record, Exec)
   - Server-client execution model
   - Success detection ("successful" in output)

7. **Create api.go**
   - APITest struct (extends EncodingTest)
   - Socket path configuration
   - .run script execution

### Phase 3: CLI and Runner

8. **Create cli.go**
   - Argument parsing
   - Mode detection (list, run, edit, etc.)
   - Help text

9. **Create test/cmd/functional**
   - Main entry point
   - Wire up all components
   - Output formatting (colors, progress)

10. **Create test/cmd/test-everything**
    - Go program orchestrating all test types
    - Runs: lint, unit tests, functional tests
    - Reports overall status

### Phase 4: Migration

11. **Update Makefile**
    - Add `make functional` target
    - Add `make test-all` target
    - Keep `make self-check` as alias for backward compatibility

12. **Deprecate old code**
    - Mark `test/cmd/self-check/` as deprecated
    - Remove after verification period

### Phase 5: Documentation

13. **Update README**
    - Document new test runner usage
    - Document test file formats

14. **Update CLAUDE_CONTINUATION.md**
    - Document new test locations
    - Update test status table

---

## Verification Checklist

- [ ] `go run ./test/cmd/functional encoding --list` shows all tests
- [ ] `go run ./test/cmd/functional encoding --all` runs all tests
- [ ] `go run ./test/cmd/functional api --all` runs all API tests
- [ ] Same tests pass as before migration
- [ ] `--verbose` shows command output
- [ ] `--debug test` shows verbose for specific test only
- [ ] `--timeout` changes test timeout
- [ ] `--stress N` runs test N times
- [ ] `--edit` opens test files in editor
- [ ] Timing cache is updated after runs
- [ ] ETA is shown for --all runs
- [ ] `make test` still passes
- [ ] `make lint` still passes
- [ ] `make self-check` still works (backward compat)

---

## Key ExaBGP Files to Reference

| ZeBGP Component | ExaBGP Reference |
|-----------------|------------------|
| test/internal/state.go | qa/bin/functional:State enum |
| test/internal/record.go | qa/bin/functional:Record class |
| test/internal/exec.go | qa/bin/functional:Exec class |
| test/internal/tests.go | qa/bin/functional:Tests class |
| test/internal/timing.go | qa/bin/functional:timing functions |
| test/internal/encoding.go | qa/bin/functional:EncodingTests |
| test/internal/plugin.go | qa/bin/functional:APITests |
| test/cmd/functional | qa/bin/functional (main) |
| test/cmd/test-everything | qa/bin/test_everything |

---

## Effort Estimate

| Phase | Components | Estimated Lines | Priority |
|-------|------------|-----------------|----------|
| 1 | Core infrastructure | ~400 | P0 |
| 2 | Test types | ~200 | P0 |
| 3 | CLI and runner | ~300 | P0 |
| 4 | Migration | ~50 | P1 |
| 5 | Documentation | ~100 | P2 |
| **Total** | | **~1,050** | |

---

## Notes

- Keep testpeer (internal/test/peer) as-is - it's the BGP validation logic
- The new runner orchestrates tests, testpeer validates messages
- Use Go for test-everything (consistency with rest of codebase)
- Timing cache prevents rebuilding timing data on each run
- Nick assignment must be deterministic (sorted by name)
- Test data stays in test/data/ (no migration needed)

---

**Created:** 2025-12-27
**Status:** Ready for implementation
