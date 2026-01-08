# Spec: Functional Test AI-Friendly Diagnostics

## SOURCE FILES (read before implementation)

```
┌─────────────────────────────────────────────────────────────────┐
│  Read these source files before implementing:                   │
│                                                                 │
│  1. test/pkg/encoding.go - Current runner, runTest method       │
│  2. test/pkg/tests.go - Display method                          │
│  3. test/pkg/cli.go - PrintSummary                              │
│  4. test/cmd/zebgp-peer/main.go - Peer output format            │
│  5. pkg/bgp/message/*.go - Message decoding library             │
│  6. pkg/testpeer/decode.go - Decoding functions to copy         │
│  7. pkg/testpeer/peer.go - Checker, LoadExpectFile              │
│                                                                 │
│  NOTE: Protocol files (.claude/ESSENTIAL_PROTOCOLS.md,          │
│  .claude/INDEX.md, docs/plan/CLAUDE_CONTINUATION.md) should have     │
│  been read at SESSION START, before /prep was invoked.          │
│                                                                 │
│  ON COMPLETION: Update design docs listed in Documentation      │
│  Impact section to match any design changes made.               │
└─────────────────────────────────────────────────────────────────┘
```

## Task

Create a **NEW** functional test program with AI-friendly debugging output.

**IMPORTANT:** Do NOT modify the existing `test/cmd/functional` or `test/pkg/*`. Create a new program that can eventually replace it.

When a test fails, an AI assistant should be able to understand:
1. What message was expected (hex + decoded)
2. What message was received (hex + decoded)
3. Exactly where they differ
4. Context to fix the bug (config, test setup)

## New Program Location

```
test/cmd/selfcheck/          # NEW program
  main.go                    # Entry point

test/selfcheck/              # NEW package (not test/pkg)
  color.go                   # TTY-aware colors
  decode.go                  # ZeBGP library integration
  ports.go                   # Dynamic port finder
  limits.go                  # ulimit check
  record.go                  # Test record with MessageExpect
  runner.go                  # Test execution
  report.go                  # AI-friendly failure output
```

The existing `test/cmd/functional` and `test/pkg/*` remain untouched until the new program is proven.

## Files to Create

### DO NOT MODIFY (in active use)

```
pkg/testpeer/          # DO NOT TOUCH - currently in use
  decode.go            # Use as-is: DecodeMessage, Diff
  peer.go              # Use as-is: Checker, LoadExpectFile
```

### New Program (wraps pkg/testpeer)

```
test/cmd/selfcheck/
  main.go              # Entry point, CLI parsing, --port/--count flags

test/selfcheck/
  color.go             # TTY-aware color helpers (uses golang.org/x/term)
  decode.go            # Wrapper: adds ColoredFormat(), EnhancedDiff() around testpeer
  display.go           # Live test status display
  limits.go            # ulimit check and raise
  ports.go             # Dynamic port range finder
  report.go            # AI-friendly failure report
  runner.go            # Test execution

  ports_test.go        # Unit tests for port finder
  decode_test.go       # Unit tests for wrapper
```

## Current State

| Metric | Value |
|--------|-------|
| Tests | 32 passed, 5 failed (feature gaps - SRv6 MUP, watchdog) |
| Failure output | ✅ AI-friendly: cmd + raw + decoded |
| Message decoding | ✅ Shown in failure output |
| AI debuggability | ✅ Structured reports with diff and debug commands |

**P0 Implementation Complete** (commit 5b92814)

## Problem Statement

Current failure output:
```
V srv6-mup failed:
test timed out
peer: listening on 127.0.0.1:1821
...
```

**Problems:**
1. No decoded message content
2. No expected vs received comparison
3. No structured format for AI parsing
4. Display has artifacts (escape codes visible when piped)
5. No cmd ↔ raw ↔ json correlation (ExaBGP .ci format supports all three)
6. Not colorful - hard to visually scan

## Goal Achievement

```
🎯 User's actual goal: AI-friendly failure diagnostics

| Check | Status |
|-------|--------|
| Expected msg shown? | ✅ cmd + raw + decoded |
| Received msg shown? | ✅ raw + decoded |
| Messages decoded? | ✅ Automatic in failure report |
| Diff shown? | ✅ Attribute + byte-level diff |
| cmd ↔ raw ↔ json? | ✅ All formats parsed from .ci |
| Colorful output? | ✅ TTY-aware colors |
| AI can debug? | ✅ Structured reports with context |
```

## Implementation Steps

### Phase 1: AI-Friendly Failure Output (P0)

#### 1.1 Structured Colorful Failure Report

When a test fails, output a structured, colorful report showing **cmd + raw + decoded**:

```
[CYAN]═══════════════════════════════════════════════════════════════════════════════[RESET]
[RED]TEST FAILURE[RESET]: [CYAN]V[RESET] srv6-mup
[CYAN]═══════════════════════════════════════════════════════════════════════════════[RESET]

[YELLOW]CONFIG:[RESET]  test/data/encode/srv6-mup.conf
[YELLOW]CI FILE:[RESET] test/data/encode/srv6-mup.ci
[YELLOW]TYPE:[RESET]    [RED]mismatch[RESET] (message 1 of 2)

[CYAN]───────────────────────────────────────────────────────────────────────────────[RESET]
[CYAN]EXPECTED MESSAGE 1:[RESET]
[CYAN]───────────────────────────────────────────────────────────────────────────────[RESET]
[YELLOW]cmd:[RESET]     announce route 10.0.0.0/24 next-hop 1.2.3.4 as-path [65000]
[YELLOW]raw:[RESET]     FFFF...002D020000001540010100400200[GREEN]0A000001[RESET]...
[YELLOW]decoded:[RESET]
  [GRAY]type:[RESET]      UPDATE
  [GRAY]withdrawn:[RESET] (none)
  [GRAY]attributes:[RESET]
    ORIGIN:   IGP
    AS_PATH:  [65000]
    NEXT_HOP: [GREEN]10.0.0.1[RESET]
  [GRAY]nlri:[RESET]       10.0.0.0/24

[CYAN]───────────────────────────────────────────────────────────────────────────────[RESET]
[CYAN]RECEIVED MESSAGE 1:[RESET]
[CYAN]───────────────────────────────────────────────────────────────────────────────[RESET]
[YELLOW]raw:[RESET]     FFFF...002D020000001540010100400200[RED]0A000002[RESET]...
[YELLOW]decoded:[RESET]
  [GRAY]type:[RESET]      UPDATE
  [GRAY]withdrawn:[RESET] (none)
  [GRAY]attributes:[RESET]
    ORIGIN:   IGP
    AS_PATH:  [65000]
    NEXT_HOP: [RED]10.0.0.2[RESET]
  [GRAY]nlri:[RESET]       10.0.0.0/24

[CYAN]───────────────────────────────────────────────────────────────────────────────[RESET]
[YELLOW]DIFF:[RESET]
[CYAN]───────────────────────────────────────────────────────────────────────────────[RESET]
  NEXT_HOP: [RED]-10.0.0.1[RESET] [GREEN]+10.0.0.2[RESET]
  [GRAY]raw byte 37-40:[RESET] [RED]0A000001[RESET] → [GREEN]0A000002[RESET]

[CYAN]───────────────────────────────────────────────────────────────────────────────[RESET]
[YELLOW]DEBUG:[RESET]
[CYAN]───────────────────────────────────────────────────────────────────────────────[RESET]
[GRAY]# Decode expected:[RESET]
zebgp decode update FFFF...

[GRAY]# Decode received:[RESET]
zebgp decode update FFFF...

[GRAY]# Run test manually:[RESET]
go run ./test/cmd/functional encoding --server V
go run ./test/cmd/functional encoding --client V

[CYAN]═══════════════════════════════════════════════════════════════════════════════[RESET]
```

**Color Legend:**
- [CYAN] - Section headers, labels
- [YELLOW] - Field names
- [GREEN] - Expected/correct values
- [RED] - Received/incorrect values, errors
- [GRAY] - De-emphasized info

**TTY Detection:** Colors only when stdout is a terminal. Plain text when piped.

#### 1.1b Timeout Failure Output

Different failure types need different output:

```
[CYAN]═══════════════════════════════════════════════════════════════════════════════[RESET]
[RED]TEST FAILURE[RESET]: [CYAN]V[RESET] srv6-mup
[CYAN]═══════════════════════════════════════════════════════════════════════════════[RESET]

[YELLOW]CONFIG:[RESET]  test/data/encode/srv6-mup.conf
[YELLOW]TYPE:[RESET]    [RED]timeout[RESET] (30s)

[CYAN]───────────────────────────────────────────────────────────────────────────────[RESET]
[YELLOW]PROGRESS:[RESET]
[CYAN]───────────────────────────────────────────────────────────────────────────────[RESET]
  [GRAY]expected messages:[RESET] 3
  [GRAY]received messages:[RESET] 1
  [GRAY]status:[RESET]            [RED]waiting for message 2[RESET]

[CYAN]───────────────────────────────────────────────────────────────────────────────[RESET]
[YELLOW]LAST RECEIVED (message 1):[RESET]
[CYAN]───────────────────────────────────────────────────────────────────────────────[RESET]
[YELLOW]raw:[RESET]     FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:0013:04:
[YELLOW]decoded:[RESET] KEEPALIVE

[CYAN]───────────────────────────────────────────────────────────────────────────────[RESET]
[YELLOW]EXPECTED NEXT (message 2):[RESET]
[CYAN]───────────────────────────────────────────────────────────────────────────────[RESET]
[YELLOW]cmd:[RESET]     announce route 10.0.0.0/24 next-hop 1.2.3.4
[YELLOW]raw:[RESET]     FFFF...002D02...
[YELLOW]decoded:[RESET]
  [GRAY]type:[RESET] UPDATE
  [GRAY]nlri:[RESET] 10.0.0.0/24

[CYAN]───────────────────────────────────────────────────────────────────────────────[RESET]
[YELLOW]CLIENT OUTPUT:[RESET]
[CYAN]───────────────────────────────────────────────────────────────────────────────[RESET]
Starting ZeBGP with config: test/data/encode/srv6-mup.conf
ZeBGP running. Press Ctrl+C to stop.
[GRAY](no further output - likely stuck or missing feature)[RESET]

[CYAN]═══════════════════════════════════════════════════════════════════════════════[RESET]
```

#### 1.2 Copy and Evolve Decoder (DO NOT MODIFY pkg/testpeer)

**Starting Point:** Copy `pkg/testpeer/decode.go` to `test/selfcheck/decode.go`.

**CONSTRAINT:** `pkg/testpeer` is in active use - DO NOT MODIFY IT.

```bash
# Step 1: Copy as starting point
cp pkg/testpeer/decode.go test/selfcheck/decode.go

# Step 2: Change package name
# package testpeer → package selfcheck

# Step 3: Evolve freely
```

**Then enhance the copy:**

```go
// test/selfcheck/decode.go - OUR OWN COPY

package selfcheck

// Add color support
func (m *DecodedMessage) ColoredString(useColors bool) string {
    // Enhanced with colors
}

// Add byte offset diff
func EnhancedDiff(expected, received string, useColors bool) string {
    // Attribute comparison + byte offsets
}
```

**Why copy, not wrap:**
- No import dependency on pkg/testpeer
- Free to modify/extend without affecting existing code
- Cleaner separation
- Can diverge as needed for AI-friendly output

**Files:**
- NEW: `test/selfcheck/decode.go` - Copy of pkg/testpeer/decode.go, then enhanced
- NEW: `test/selfcheck/report.go` - Uses local decode for AI-friendly output

#### 1.3 Parse cmd + raw + json from .ci File

The .ci file format supports three representations:
```
1:cmd:announce route 10.0.0.0/24 next-hop 1.2.3.4
1:raw:FFFF...:002D:02:...
1:json:{"type":"update","nlri":["10.0.0.0/24"]}
```

Enhance Record to store all three:

```go
// MessageExpect holds expected message in multiple formats
type MessageExpect struct {
    Index   int    // Message number (1, 2, 3...)
    Cmd     string // Human-readable API command (if present)
    Raw     []byte // Wire format bytes
    JSON    string // JSON representation (if present)
    Decoded string // Human-readable decoded (generated from Raw)
}

// In Record struct
type Record struct {
    // ... existing fields
    Messages []MessageExpect  // Expected messages with all formats
}
```

**Parsing in parseAndAdd:**
```go
case strings.Contains(line, ":cmd:"):
    // Parse: "1:cmd:announce route..."
    parts := strings.SplitN(line, ":", 3)
    idx, _ := strconv.Atoi(parts[0])
    r.getOrCreateMessage(idx).Cmd = parts[2]

case strings.Contains(line, ":raw:"):
    // Parse: "1:raw:FFFF..."
    parts := strings.SplitN(line, ":", 3)
    idx, _ := strconv.Atoi(parts[0])
    r.getOrCreateMessage(idx).Raw = hexToBytes(parts[2])

case strings.Contains(line, ":json:"):
    // Parse: "1:json:{...}"
    parts := strings.SplitN(line, ":", 3)
    idx, _ := strconv.Atoi(parts[0])
    r.getOrCreateMessage(idx).JSON = parts[2]
```

**Files:**
- NEW: `test/selfcheck/record.go` - Record with MessageExpect struct

#### 1.4 Capture and Decode Received Messages

The peer (zebgp-peer) already outputs received messages. Parse and decode:

```go
// extractReceivedMessages parses peer output for received raw messages
func extractReceivedMessages(peerOutput string) [][]byte {
    // Parse "msg recv FFFF..." lines
}
```

**Files:**
- NEW: `test/selfcheck/runner.go` - Extract and decode received messages

### Phase 2: Port Management & ulimit (P0)

#### 2.1 Dynamic Port Range Selection

When struggling with a test, users often run multiple instances. Instead of warning, **find free ports automatically**.

```go
// FindFreePortRange finds N consecutive free ports starting from base
func FindFreePortRange(base, count int) (int, error) {
    for startPort := base; startPort < 65000; startPort += count {
        if isPortRangeFree(startPort, count) {
            return startPort, nil
        }
    }
    return 0, fmt.Errorf("no free port range of %d ports found", count)
}

// isPortRangeFree checks if ports [start, start+count) are all available
func isPortRangeFree(start, count int) bool {
    for port := start; port < start+count; port++ {
        ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
        if err != nil {
            return false  // Port in use
        }
        ln.Close()
    }
    return true
}
```

**Behavior:**
1. Try `--port` value (default 1790)
2. If range is occupied, scan for next free range
3. Print which range is being used:
   ```
   [CYAN]ports:[RESET] 1790-1826 (37 tests)
   ```
   or if shifted:
   ```
   [YELLOW]ports:[RESET] 1827-1863 (base 1790 in use, shifted)
   ```

**Files:**
- NEW: `test/selfcheck/ports.go` - Port range finder
- NEW: `test/cmd/selfcheck/main.go` - CLI with --port flag, reports range

#### 2.2 Check File Descriptor Limit

Before running tests, verify sufficient file descriptors:

```go
// CheckUlimit ensures sufficient file descriptors for parallel tests
// Each test spawns: zebgp (may fork) + zebgp-peer = ~20 FDs per concurrent test
// With parallel=4: need 4 × 20 = 80 FDs minimum, recommend 256+
func CheckUlimit(parallel int) error {
    var limit syscall.Rlimit
    if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
        return err
    }

    fdsPerTest := uint64(20)  // Conservative: sockets, pipes, files
    needed := uint64(parallel) * fdsPerTest
    recommended := uint64(256)
    if needed < recommended {
        needed = recommended
    }

    if limit.Cur < needed {
        // Try to raise soft limit
        newLimit := min(needed, limit.Max)
        limit.Cur = newLimit
        if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
            return fmt.Errorf("ulimit too low: have %d, need %d (run: ulimit -n %d)",
                limit.Cur, needed, needed)
        }
        fmt.Printf("[YELLOW]ulimit:[RESET] raised to %d\n", newLimit)
    }
    return nil
}
```

**Files:**
- NEW: `test/selfcheck/limits.go`

#### 2.3 TTY Detection for Colors

Add proper terminal detection:

```go
// test/pkg/color.go

import "golang.org/x/term"

var useColors = term.IsTerminal(int(os.Stdout.Fd()))

func red(s string) string {
    if !useColors { return s }
    return "\033[91m" + s + "\033[0m"
}

func green(s string) string {
    if !useColors { return s }
    return "\033[92m" + s + "\033[0m"
}

// ... cyan, yellow, gray
```

**Files:**
- NEW: `test/selfcheck/color.go` - Color helpers with TTY detection
- ADD: `golang.org/x/term` dependency

### Phase 3: Display & Report (P1)

#### 3.1 Fix Display Artifacts

Current display shows escape code artifacts. Fix terminal handling:

```go
// Display shows test status with proper terminal clearing
func (ts *Tests) Display() {
    // Clear line properly before writing
    fmt.Print("\r\033[K")  // Return + clear to end of line

    // ... build status string

    // Flush without newline
    fmt.Print(status)
}
```

**Files:**
- NEW: `test/selfcheck/display.go` - Test status display with proper clearing

#### 3.2 AI-Friendly Failure Report

```go
// Report generates structured failure output
type Report struct {
    TestNick    string
    TestName    string
    ConfigFile  string
    CIFile      string
    FailureType string  // "mismatch", "timeout", "connection_refused"
    Expected    []MessageExpect
    Received    []ReceivedMessage
}

func (r *Report) Print() {
    // Uses color.go for TTY-aware output
    // Structured sections: CONFIG, EXPECTED, RECEIVED, DIFF, DEBUG
}
```

**Files:**
- NEW: `test/selfcheck/report.go` - AI-friendly failure report generator

### Phase 4: Stress Testing (P2)

#### 4.1 Count-Based Stress Mode

Go-style stress testing: run each selected test N times.

```
functional encoding --count 10 ebgp
```

**Behavior:**
- Run test N times regardless of pass/fail
- Report statistics: passed/failed count, min/avg/max time
- Useful for detecting flaky tests

```go
// RunWithCount runs each test count times
func (r *Runner) RunWithCount(ctx context.Context, opts *RunOptions, count int) bool {
    for i := 0; i < count; i++ {
        // Reset test states
        for _, rec := range r.tests.Selected() {
            rec.State = StateNone
        }
        // Run iteration
        r.Run(ctx, opts)
        // Collect stats
    }
    // Print statistics
}
```

**Files:**
- NEW: `test/selfcheck/runner.go` - RunWithCount method
- NEW: `test/cmd/selfcheck/main.go` - --count flag

### Phase 5: Optional Improvements (P3)

#### 5.1 --save Implementation

Save test output to files for later analysis:

```
functional encoding --save /tmp/test-run/ ebgp
```

Creates:
```
/tmp/test-run/
  ebgp/
    peer-stdout.log
    peer-stderr.log
    client-stdout.log
    client-stderr.log
    expected.txt
    received.txt
```

**Files:**
- NEW: `test/selfcheck/runner.go` - SaveDir implementation

### Phase 6: Migration & Cleanup (P4)

#### 6.1 Validate New Program

Before removing old code, verify:
- [ ] All existing tests pass with new program
- [ ] Output is more useful for debugging
- [ ] No regressions in functionality
- [ ] Makefile targets work

#### 6.2 Remove Old Program

Once validated:
```bash
rm -rf test/cmd/functional/
rm -rf test/pkg/
rm test/cmd/self-check/  # if exists
```

#### 6.3 Rename selfcheck to functional

```bash
git mv test/cmd/selfcheck test/cmd/functional
git mv test/selfcheck test/functional
```

Update imports in all files.

#### 6.4 Update Documentation

- [ ] `.claude/zebgp/FUNCTIONAL_TESTS.md` - Rewrite for new architecture
- [ ] `README.md` - Update test instructions if mentioned
- [ ] `Makefile` - Update targets
- [ ] `docs/plan/CLAUDE_CONTINUATION.md` - Mark migration complete

#### 6.5 Update Makefile

```makefile
# Old targets removed, new targets:
functional: functional-encoding functional-api

functional-encoding:
	go run ./test/cmd/functional encoding --all

functional-api:
	go run ./test/cmd/functional api --all
```

## Priority Order

1. **P0 (Do Now):** AI-friendly failure output + dynamic ports + ulimit ✅
2. **P1 (Soon):** Display fixes ✅ (done as part of P0)
3. **P2 (Later):** Stress testing (--count) ✅ (commit 4ce5b99)
4. **P3 (Optional):** --save implementation ✅ (commit 511b423)
5. **P4 (Final):** Remove old program, rename, update docs ✅

## Dropped from Original Spec

| Feature | Reason |
|---------|--------|
| Signal handlers | Go context cancellation is sufficient |
| Concurrent run **warning** | Replaced with dynamic port allocation (solves the problem) |
| Stale process cleanup | Process groups handle this |
| Retry-on-fail | Masks bugs; use --count for flaky detection |
| Decoding tests | Different spec if needed |
| Parsing tests | Different spec if needed |
| CLI tests | Different spec if needed |

## Embedded Rules

- TDD: Write tests for new code in test/selfcheck/
- Verify: `make test && make lint` before done
- **Copy, don't import pkg/testpeer**: Copy files as starting point, evolve independently
- Colors: TTY detection, plain text when piped

## Documentation Impact

- [ ] `.claude/zebgp/FUNCTIONAL_TESTS.md` - Update with new output format
- [ ] `docs/plan/CLAUDE_CONTINUATION.md` - Update after impl

## Checklist

### Phase 0-3: New Program (P0 COMPLETE)
- [x] Copy pkg/testpeer/decode.go → test/selfcheck/decode.go
- [x] Add ColoredString() to copied decode.go
- [x] Add ColoredDiff() with byte offsets
- [x] Copy .ci parsing logic, add cmd + raw + json extraction
- [x] pkg/testpeer NOT modified (in active use)
- [x] Failure output shows all three forms
- [x] Colors work on TTY, plain when piped
- [x] Dynamic port range finder works
- [x] Reports port range at startup
- [x] ulimit check uses parallel count (not testCount)
- [x] ExaBGP-style progress: `timeout [N/M] running N passed N failed N [IDs]`
- [x] `make test` passes
- [x] `make lint` passes

### Phase 4: Stress Testing (P2 COMPLETE)
- [x] `--count N` flag added to CLI
- [x] `RunWithCount()` method in runner.go
- [x] `IterationStats` type for per-test statistics
- [x] `StressSummary()` display with pass/fail/timeout counts
- [x] Min/avg/max timing per test
- [x] Context cancellation handled between iterations
- [x] Per-iteration failure reports suppressed (quiet mode)
- [x] 9 unit tests for stats logic

### Phase 5: Save Output (P3 COMPLETE)
- [x] `--save DIR` flag (already in CLI)
- [x] `saveTestOutput()` method in runner.go
- [x] Separate stdout/stderr files for peer and client
- [x] expected.txt from .ci file expects
- [x] received.txt from parsed peer output
- [x] Directory naming: `nick-testname` for easy identification
- [x] Filename sanitization (path separators, special chars)
- [x] Warning on save failure (not silent)
- [x] Permissions: 0700/0600 (gosec compliant)

### Phase 6: Migration (P4 COMPLETE)
- [x] All tests pass with new program
- [x] Old `test/cmd/functional` removed
- [x] Old `test/cmd/self-check` removed
- [x] Old `test/pkg` removed (14 files)
- [x] `test/cmd/selfcheck` → `test/cmd/functional`
- [x] `test/selfcheck` → `test/functional`
- [x] Package renamed from `selfcheck` to `functional`
- [x] Imports updated
- [x] Makefile updated (removed self-check target)
- [x] `make test` passes
- [x] `make lint` passes
- [x] `.claude/zebgp/FUNCTIONAL_TESTS.md` rewritten
- [x] `docs/plan/CLAUDE_CONTINUATION.md` updated
- [x] Spec moved to `docs/plan/done/`

## Summary of Key Design Decisions

1. **Copy, don't wrap**: Copy pkg/testpeer/decode.go as starting point, evolve freely
2. **pkg/testpeer untouched**: In active use, no imports from it
3. **Three Forms**: Show cmd (human intent) + raw (wire format) + decoded
4. **Colorful but Pipe-Safe**: Use `golang.org/x/term` for TTY detection
5. **AI-Friendly**: Structured, labeled sections that Claude can parse and act on
6. **Dynamic Ports**: Auto-find free port range, don't just warn about conflicts
7. **ulimit by Parallelism**: `parallel × 20 FDs`, not `testCount × 10`
