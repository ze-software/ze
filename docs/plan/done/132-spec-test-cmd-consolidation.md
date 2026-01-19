# Spec: test-cmd-consolidation

## Task

Consolidate test commands into cleaner structure:
- Move `zebgp-peer` to `cmd/`
- Create `zebgp-test` combining functional runner + syslog
- Delete obsolete tools (`migrate-ci`, `ci-fix-order`)

## Required Reading

### Architecture Docs
- [ ] `docs/functional-tests.md` - [current test structure]

### Source Files
- [ ] `test/cmd/functional/main.go` - [test runner to port]
- [ ] `test/cmd/test-syslog/main.go` - [syslog server to port]
- [ ] `test/cmd/zebgp-peer/main.go` - [peer to move]
- [ ] `Makefile` - [targets to update]

**Key insights:**
- functional imports `internal/test/runner` package (keep unchanged)
- test-syslog imports `internal/test/syslog` package (keep unchanged)
- zebgp-peer imports `internal/test/peer` package (keep unchanged)
- Only moving/reorganizing cmd entry points, not libraries

## Current State

```
test/cmd/
├── ci-fix-order/    # obsolete (parses old .ci format)
├── functional/      # test runner
├── migrate-ci/      # obsolete (migration done)
├── test-syslog/     # syslog server for tests
└── zebgp-peer/      # BGP test peer
```

## Target State

```
cmd/
├── zebgp/           # main binary (unchanged)
├── zebgp-peer/      # BGP test peer (moved from test/cmd/)
└── zebgp-test/      # test runner + syslog
    ├── main.go      # subcommand dispatch
    ├── run.go       # run subcommand (from functional)
    └── syslog.go    # syslog subcommand (from test-syslog)

test/cmd/            # deleted entirely
```

## Design

### zebgp-test subcommands

```
zebgp-test run [type] [flags]
    type: encoding, plugin, parsing, decoding, all (default: all)
    --list          list available tests
    --count N       repeat N times (stress test)
    -v              verbose output

zebgp-test syslog [flags]
    --port N        port to listen on (0 = dynamic)
    --pattern X     exit 0 when pattern matches
    --timeout D     timeout duration
```

### zebgp-peer (unchanged interface)

```
zebgp-peer [flags] [expect-file]
    --sink          accept any, reply keepalive
    --echo          echo messages back
    --port N        listen port
    --ipv6          bind IPv6
    --asn N         override ASN
```

## 🧪 TDD Test Plan

### Unit Tests

No new unit tests required - this is a reorganization of existing cmd entry points.
Libraries (`internal/test/runner`, `internal/test/syslog`, `internal/test/peer`) remain unchanged.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| Existing functional tests | `internal/test/runner/*_test.go` | Library unchanged | |
| Existing testpeer tests | `internal/test/peer/*_test.go` | Library unchanged | |
| Existing testsyslog tests | `internal/test/syslog/*_test.go` | Library unchanged | |

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `make functional` | `Makefile` | All functional tests pass with new structure | |

## Files to Delete

- `test/cmd/migrate-ci/` - obsolete
- `test/cmd/ci-fix-order/` - obsolete
- `test/cmd/functional/` - replaced by cmd/zebgp-test
- `test/cmd/test-syslog/` - folded into cmd/zebgp-test
- `test/cmd/zebgp-peer/` - moved to cmd/

## Files to Create

- `cmd/zebgp-test/main.go` - subcommand dispatch
- `cmd/zebgp-test/run.go` - run subcommand
- `cmd/zebgp-test/syslog.go` - syslog subcommand
- `cmd/zebgp-peer/main.go` - moved from test/cmd/

## Files to Modify

- `Makefile` - update functional target

## Implementation Steps

1. **Create cmd/zebgp-test/main.go** - subcommand dispatch
2. **Create cmd/zebgp-test/run.go** - port from test/cmd/functional/main.go
3. **Create cmd/zebgp-test/syslog.go** - port from test/cmd/test-syslog/main.go
4. **Move cmd/zebgp-peer/** - move from test/cmd/zebgp-peer/
5. **Update Makefile** - change functional target
6. **Verify** - `make test && make lint && make functional`
7. **Delete test/cmd/** - remove old directory
8. **Final verify** - `make test && make lint && make functional`

## Implementation Summary

### What Was Implemented
- Created `cmd/zebgp-test/` with subcommand dispatch (main.go, run.go, syslog.go)
- Moved `zebgp-peer` from `test/cmd/` to `cmd/`
- Updated Makefile functional targets to use `go run ./cmd/zebgp-test run`
- Updated `internal/test/runner/runner.go` build path for zebgp-peer
- Updated `internal/test/runner/report.go` debug commands
- Deleted `test/cmd/` directory entirely (including obsolete migrate-ci, ci-fix-order)
- Deleted stray `migrate-ci` directory at root
- Updated documentation: `docs/functional-tests.md`, `docs/debugging-tools.md`, `.claude/rules/testing.md`

### Design Insights
- Libraries unchanged: `internal/test/runner`, `internal/test/syslog`, `internal/test/peer` all work with new cmd structure
- Subcommand pattern (zebgp-test run, zebgp-test syslog) cleaner than separate binaries

## Checklist

### 🧪 TDD
- [x] Tests written (existing tests cover functionality)
- [x] Tests FAIL (N/A - refactoring, not new features)
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes (37 encoding + 23 plugin + 12 parsing + 18 decoding = 90 tests)

### Completion
- [x] cmd/zebgp-test created
- [x] cmd/zebgp-peer moved
- [x] test/cmd deleted
- [x] Makefile updated
- [ ] Spec moved to `docs/plan/done/`
