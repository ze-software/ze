# Spec: logging-functional-tests

## Task

Add functional tests to verify:
1. Engine subsystem logging (`server`) via env vars → stderr/syslog
2. Plugin process logging (`gr` plugin) via `--log-level` flag → stderr

## Required Reading

### Architecture Docs
- [x] `docs/architecture/core-design.md` - [system components]
- [x] `docs/functional-tests.md` - [test infrastructure patterns]

### Source Files
- [x] `pkg/slogutil/slogutil.go` - [logging implementation]
- [x] `pkg/slogutil/syslog.go` - [syslog handler]
- [x] `pkg/testpeer/peer.go:956` - [.ci option parsing]
- [x] `test/functional/runner.go` - [test execution, env handling]
- [x] `pkg/plugin/server.go` - [server subsystem log calls]
- [x] `pkg/plugin/gr/gr.go` - [gr plugin log calls]

**Key insights:**
- Engine subsystems: `server`, `plugin`, `filter`, `coordinator`
- Plugin processes: `gr`, `rib` (use `--log-level` flag)
- Server logs: `"server: stageTransition START"`, `"server: deliverConfig"` (Debug)
- GR logs: `"parsed config"`, `"registered capability"` (Debug)
- `.ci` options parsed in testpeer/peer.go and runner.go

## 🧪 TDD Test Plan

### Unit Tests (already exist)
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBackendStderr` | `pkg/slogutil/slogutil_test.go` | Default stderr | ✅ |
| `TestBackendSyslog` | `pkg/slogutil/slogutil_test.go` | Syslog handler | ✅ |
| `TestLoggerEnabledDot` | `pkg/slogutil/slogutil_test.go` | Env var parsing | ✅ |

### Unit Tests (NEW - testsyslog)
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUDPServer` | `pkg/testsyslog/testsyslog_test.go` | UDP listen/receive | ✅ |
| `TestMessageCapture` | `pkg/testsyslog/testsyslog_test.go` | Message buffering | ✅ |
| `TestPatternMatch` | `pkg/testsyslog/testsyslog_test.go` | Regex matching | ✅ |
| `TestPatternMatchInvalid` | `pkg/testsyslog/testsyslog_test.go` | Invalid regex handling | ✅ |
| `TestServerClose` | `pkg/testsyslog/testsyslog_test.go` | Clean shutdown | ✅ |
| `TestContextCancellation` | `pkg/testsyslog/testsyslog_test.go` | Context respect | ✅ |

### Unit Tests (NEW - functional test framework)
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseCILoggingOptions` | `test/functional/record_test.go` | .ci option parsing (9 cases) | ✅ |
| `TestParseCILoggingOptionsNotAffectOthers` | `test/functional/record_test.go` | No regression | ✅ |
| `TestValidateLoggingExpectStderr` | `test/functional/runner_test.go` | Expect stderr (8 cases) | ✅ |
| `TestValidateLoggingRejectStderr` | `test/functional/runner_test.go` | Reject stderr (4 cases) | ✅ |
| `TestValidateLoggingExpectSyslog` | `test/functional/runner_test.go` | Expect syslog (5 cases) | ✅ |
| `TestValidateLoggingCombined` | `test/functional/runner_test.go` | Combined patterns | ✅ |

### Functional Tests (NEW - extend plugin tests)
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `logging-stderr` | `test/data/plugin/logging-stderr.ci` | `zebgp.log.server=debug` → stderr contains `subsystem=server` | ✅ |
| `logging-syslog` | `test/data/plugin/logging-syslog.ci` | `zebgp.log.backend=syslog` → test-syslog receives messages | ✅ |
| `logging-level-filter` | `test/data/plugin/logging-level-filter.ci` | `zebgp.log.server=info` → no DEBUG in stderr | ✅ |
| ~~`logging-plugin`~~ | ~~`test/data/plugin/logging-plugin.ci`~~ | ~~gr plugin `--log-level=debug`~~ | ⏸️ Design issue |

## Extended .ci Format

Add to existing format (parsed in testpeer and runner):

```
# Environment variables (set before zebgp starts)
option:env:zebgp.log.server=debug
option:env:zebgp.log.backend=syslog
option:env:zebgp.log.destination=localhost:1514

# Stderr pattern matching (regex, checked after test)
expect:stderr:subsystem=server
expect:stderr:level=DEBUG

# Negative patterns (must NOT appear)
reject:stderr:level=DEBUG

# Syslog pattern matching (requires test-syslog server)
expect:syslog:zebgp.*subsystem=server
```

## Files to Modify
- `pkg/testpeer/peer.go` - Parse `option:env:`, store in FileConfig
- `test/functional/runner.go` - Apply env vars, capture stderr, validate patterns
- `test/functional/record.go` - Add Env, ExpectStderr, RejectStderr, ExpectSyslog fields
- `Makefile` - No change needed (tests go in plugin category)
- `docs/functional-tests.md` - Document new .ci options + syslog architecture diagram

## Files to Create
- `pkg/testsyslog/testsyslog.go` - UDP syslog server library
- `pkg/testsyslog/testsyslog_test.go` - Unit tests (6 tests)
- `test/cmd/test-syslog/main.go` - CLI wrapper (for manual debugging)
- `test/functional/record_test.go` - Unit tests for .ci parsing (9 test cases)
- `test/functional/runner_test.go` - Unit tests for validateLogging (16 test cases)
- `test/data/plugin/logging-stderr.conf` - Minimal config with gr plugin
- `test/data/plugin/logging-stderr.ci` - Test engine stderr logging
- `test/data/plugin/logging-plugin.conf` - Config with gr plugin + log-level
- `test/data/plugin/logging-plugin.ci` - Test plugin stderr logging
- `test/data/plugin/logging-syslog.conf` - Config for syslog test
- `test/data/plugin/logging-syslog.ci` - Test syslog output
- `test/data/plugin/logging-level-filter.conf` - Config for level filter test
- `test/data/plugin/logging-level-filter.ci` - Test level filtering

## Implementation Steps

### Phase 1: Test Syslog Server (TDD)
1. **Write unit tests** - `pkg/testsyslog/testsyslog_test.go`
2. **Run tests** - Verify FAIL (paste output)
3. **Implement** - UDP server, message buffer, pattern matching
4. **Run tests** - Verify PASS (paste output)
5. **Create CLI** - `test/cmd/test-syslog/main.go`

### Phase 2: Extend .ci Format (TDD)
1. **Write tests** - Test option parsing in testpeer
2. **Run tests** - Verify FAIL
3. **Extend testpeer/peer.go** - Parse new options
4. **Extend runner.go** - Apply env, capture stderr, start test-syslog
5. **Extend record.go** - Add fields
6. **Run tests** - Verify PASS

### Phase 3: Functional Tests
1. **Create logging-stderr test** - Engine subsystem logging
2. **Run test** - Verify PASS
3. **Create logging-plugin test** - Plugin process logging
4. **Run test** - Verify PASS
5. **Create logging-syslog test** - Syslog backend
6. **Run test** - Verify PASS
7. **Create logging-level-filter test** - Level filtering
8. **Verify all** - `make lint && make test && make functional` (paste output)

### Phase 4: Documentation
1. **Update docs/functional-tests.md** - Document new .ci options
2. **Add syslog architecture diagram** - Flow diagram showing test runner → zebgp → testsyslog
3. **Add component table** - Key components with locations and purposes
4. **Document message format** - Syslog message format with slog.TextHandler

## Design Decisions

### Why extend .ci instead of new format?
- Reuses existing parser infrastructure
- Tests run as part of `make functional-plugin`
- No new test type or runner needed

### Test Syslog Server Design
```go
// pkg/testsyslog/testsyslog.go
type Server struct {
    addr     string
    messages []string
    mu       sync.Mutex
}

func (s *Server) Start(ctx context.Context) error  // Listen UDP
func (s *Server) Messages() []string               // Get captured messages
func (s *Server) Match(pattern string) bool        // Regex match any message
func (s *Server) Close() error
```

### Syslog Test Execution Flow
```
1. Parse .ci file, find option:env:zebgp.log.backend=syslog
2. Start test-syslog on dynamic port
3. Set zebgp.log.destination=localhost:<port>
4. Start zebgp-peer, start zebgp
5. Wait for test completion
6. Check expect:syslog: patterns against captured messages
7. Stop test-syslog
```

### What triggers server subsystem logs?
- Plugin registration: `"server: handleRegistrationLine parsed"`
- Config delivery: `"server: deliverConfig START"`
- Stage transitions: `"server: stageTransition START"`

A config with any plugin (like `gr`) will trigger these logs.

## Test Configs

### logging-stderr.conf
```
peer 127.0.0.1 {
    router-id 1.2.3.4;
    local-address 127.0.0.1;
    local-as 65001;
    peer-as 65001;

    process gr {
        run "zebgp plugin gr";
    }
}
```

### logging-plugin.conf
```
peer 127.0.0.1 {
    router-id 1.2.3.4;
    local-address 127.0.0.1;
    local-as 65001;
    peer-as 65001;

    graceful-restart { restart-time 120; }

    process gr {
        run "zebgp plugin gr --log-level=debug";
    }
}
```

## Implementation Summary

### What Was Implemented
- **testsyslog package** (`pkg/testsyslog/`): UDP syslog server for capturing log messages
  - `testsyslog.go`: Server with Start(), Port(), Messages(), Match(), Close()
  - `testsyslog_test.go`: 6 unit tests covering UDP, capture, pattern matching, close, context
- **test-syslog CLI** (`test/cmd/test-syslog/main.go`): Manual debugging tool
- **Extended .ci format** in `test/functional/record.go`:
  - `EnvVars []string` for `option:env:`
  - `ExpectStderr []string` for `expect:stderr:`
  - `RejectStderr []string` for `reject:stderr:`
  - `ExpectSyslog []string` for `expect:syslog:`
- **Extended runner** (`test/functional/runner.go`):
  - Start test-syslog server when `expect:syslog:` present
  - Auto-set `zebgp.log.backend=syslog` and `zebgp.log.destination`
  - `validateLogging()` function for pattern matching
- **Unit tests** (`test/functional/record_test.go`, `runner_test.go`):
  - 9 test cases for .ci option parsing
  - 16 test cases for validateLogging() (stderr expect/reject, syslog)
- **Functional tests**:
  - `logging-stderr.ci` - engine subsystem → stderr
  - `logging-syslog.ci` - syslog backend
  - `logging-level-filter.ci` - DEBUG filtered at INFO
- **Documentation**: Updated `docs/functional-tests.md` with new options and syslog architecture diagram

### Bugs Found/Fixed
- **Test configs missing `send { update; }`**: graceful-restart requires process with `send { update; }` block
- **Pattern not found initially**: syslog messages include Go slog TextHandler format, patterns work correctly

### Design Insights
- slogutil syslog backend uses `slog.NewTextHandler` writing to `syslog.Writer`
- Syslog messages format: `<priority>timestamp hostname zebgp: level=X subsystem=Y msg=Z key=value...`
- Patterns should match the TextHandler format (key=value pairs)

### Deviations from Plan
- **logging-plugin test removed**: `zebgp.log.plugin=enabled` has dual purpose - enables stderr relay but "enabled" is not a valid log level, causing discard logger. This is a design issue to address separately.
- **testpeer unchanged**: All parsing done in `test/functional/record.go` (not testpeer/peer.go)

### Future Work
- **zebgp.log.plugin design issue**: The env var serves dual purpose:
  1. `zebgp.log.plugin=enabled/disabled` - controls plugin stderr relay
  2. But "enabled" is not a valid log level for slogutil.Logger()
  - Recommendation: Split into separate vars or accept log levels for relay control

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (verified during development)
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (80+ tests including 3 new logging tests)

### Documentation (during implementation)
- [x] Required docs read
- [x] Spec file updated with Implementation Summary

### Completion (after tests pass)
- [x] Unit tests added for record.go parsing (9 test cases)
- [x] Unit tests added for validateLogging() (16 test cases)
- [x] Spec moved to `docs/plan/done/130-logging-functional-tests.md`
- [ ] All files committed together
