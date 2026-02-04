# Spec: Unify Test Tools into ze-test

## Task

Move testing commands out of production binaries into a unified `ze-test` binary:
- Move `ze config test` → `ze-test editor`
- Move `ze-peer` → `ze-test peer`

## Rationale

- Testing code shouldn't be in production binaries
- Single test tool binary is easier to maintain/distribute
- Eliminates subprocess build overhead in `--server` debug mode

## Implementation Summary

### What Was Implemented

**New files created:**
- `cmd/ze-test/editor.go` - Editor functional test runner (.et files)
- `cmd/ze-test/peer.go` - BGP test peer (sink/echo/check modes)

**Files modified:**
- `cmd/ze-test/main.go` - Added `editor` and `peer` command dispatch
- `cmd/ze-test/bgp.go` - Updated `--server` mode to use peer library directly (no subprocess)
- `cmd/ze/config/main.go` - Removed `test` subcommand
- `internal/test/runner/runner.go` - Builds `ze-test` instead of `ze-peer`, uses `ze-test peer`
- `Makefile` - Removed `ze-peer` target, updated `functional-editor`

**Files deleted:**
- `cmd/ze-peer/main.go` - Entire directory removed

### New Command Structure

```
ze-test <command>
├── bgp encode      # BGP encoding tests
├── bgp plugin      # BGP plugin tests
├── bgp decode      # BGP decoding tests
├── bgp parse       # Config parsing tests
├── editor          # Editor functional tests (.et files)
├── peer            # BGP test peer (sink/echo/check modes)
└── syslog          # Syslog server for testing
```

### Migration

| Before | After |
|--------|-------|
| `ze-peer --mode sink --port 1790` | `ze-test peer --mode sink --port 1790` |
| `ze-peer --port 1790 file.msg` | `ze-test peer --port 1790 file.msg` |
| `ze config test` | `ze-test editor` |
| `ze config test -v` | `ze-test editor -v` |

### Performance Improvement

The `--server` debug mode in `ze-test bgp` now uses the peer library directly instead of spawning a subprocess and building `ze-peer`. This eliminates the 2-3 second `go build` delay.

## Bugs Found/Fixed During Implementation

1. **Flag aliases broken** - Used `fs.String()` twice instead of `fs.StringVar()` for aliases
2. **Exit code wrong on file load error** - `parsePeerFlags()` returned nil for both success (help) and error
3. **Inconsistent error output** - Some errors went to stdout instead of stderr
4. **Signal handler leak** - Missing `signal.Stop()` cleanup

## Checklist

### TDD
- [x] Implementation complete
- [x] Feature code integrated into codebase
- [x] Functional tests verify end-user behavior

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] Doc comments updated in main.go
- [x] Makefile updated
