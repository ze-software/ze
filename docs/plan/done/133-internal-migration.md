# Spec: internal-migration

## Task

Move all library code into `internal/` to make clear ZeBGP is a binary, not a library.

## Required Reading

### Architecture Docs
- [x] `docs/architecture/core-design.md` - [understand package dependencies]

### Source Files
- [x] `pkg/` - [16 packages to move]
- [x] `internal/` - [existing internal packages]
- [x] `test/functional/` - [library to move]
- [x] `test/ciformat/` - [library to move]

## Previous State

```
pkg/
├── bgp/            # BGP protocol (attribute/, capability/, context/, fsm/, message/, nlri/, wire/)
├── cbor/           # CBOR encoding
├── config/         # Configuration parsing
├── editor/         # Config editor
├── exabgp/         # ExaBGP compatibility
├── plugin/         # Plugin system (gr/, rib/, rr/)
├── pool/           # Deduplication pools
├── reactor/        # Engine orchestrator
├── rib/            # Routing tables
├── selector/       # Peer selection
├── slogutil/       # Logging
├── source/         # Source registry
├── testpeer/       # BGP test peer
├── testsyslog/     # Test syslog server
├── trace/          # Debug tracing
└── wire/           # Wire parsing utilities

test/
├── ciformat/       # CI file format parser (library)
├── functional/     # Functional test runner (library)
├── data/           # Test data (stays)
├── integration/    # Integration tests (stays)
└── internet/       # Scripts (stays)

internal/
├── pool/           # Renamed to delete-pool/ (unused, preserved for reference)
└── store/          # Storage primitives
```

## Current State (After Migration)

```
internal/
├── bgp/            # from pkg/bgp
├── cbor/           # from pkg/cbor
├── config/         # from pkg/config
│   └── editor/     # from pkg/editor (moved under config/)
├── exabgp/         # from pkg/exabgp
├── plugin/         # from pkg/plugin
├── pool/           # from pkg/pool
├── reactor/        # from pkg/reactor
├── rib/            # from pkg/rib
├── selector/       # from pkg/selector
├── slogutil/       # from pkg/slogutil
├── source/         # from pkg/source
├── store/          # already here
├── trace/          # from pkg/trace
├── wire/           # from pkg/wire
├── delete-pool/    # preserved for reference (was internal/pool/)
└── test/           # test infrastructure (grouped)
    ├── peer/       # from pkg/testpeer (package renamed)
    ├── syslog/     # from pkg/testsyslog (package renamed)
    ├── runner/     # from test/functional (package renamed)
    └── ci/         # from test/ciformat (package renamed)

test/
├── data/           # unchanged
├── integration/    # unchanged
└── internet/       # unchanged

pkg/                # deleted
```

## Pool Conflict Resolution

**Decision:** Rename `internal/pool/` → `internal/delete-pool/` (preserve for reference)

| Package | Status | Reason |
|---------|--------|--------|
| `pkg/pool/` | Keep, move to `internal/pool/` | Used by rib/storage, double-buffer design |
| `internal/pool/` | Renamed to `internal/delete-pool/` | Unused, single-buffer design with metrics |

The double-buffer design in `pkg/pool/` is better for BGP (non-blocking compaction).
The `delete-pool/` preserves scheduler/metrics ideas for potential future merge.

## 🧪 TDD Test Plan

### Unit Tests

No new unit tests required - this is a refactoring of import paths only.
All existing tests validate functionality is unchanged.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| All existing unit tests | `internal/**/*_test.go` | Functionality unchanged after move | ✅ |
| All existing integration tests | `test/integration/*_test.go` | End-to-end unchanged | ✅ |

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `make functional` | `Makefile` | All 90 tests pass after migration | ✅ |

## Files to Modify

- All `*.go` files with `pkg/` imports (~200+ files)
- All `*.go` files with `test/functional` or `test/ciformat` imports
- All `*.md` files with `pkg/` references
- `go.mod` - no changes needed (internal paths work automatically)

## Files to Create

None - only moving existing files.

## Files to Delete

- `pkg/` directory (after moving contents)

## Implementation Steps

1. **Create internal/test/** - `mkdir -p internal/test`
2. **Move pkg/* to internal/** - All 14 packages (excluding testpeer, testsyslog)
3. **Move pkg/testpeer to internal/test/peer** - Rename package to `peer`
4. **Move pkg/testsyslog to internal/test/syslog** - Rename package to `syslog`
5. **Move test/functional to internal/test/runner** - Rename package to `runner`
6. **Move test/ciformat to internal/test/ci** - Rename package to `ci`
7. **Update all imports** - sed across codebase
8. **Update package declarations** - Fix renamed packages
9. **Delete pkg/** - Remove empty directory
10. **Update documentation** - All .md files with pkg/ references
11. **Verify** - `make test && make lint && make functional`

## Import Changes

| Old Import | New Import |
|------------|------------|
| `pkg/bgp/...` | `internal/bgp/...` |
| `pkg/config/...` | `internal/config/...` |
| `pkg/editor` | `internal/config/editor` |
| `pkg/testpeer` | `internal/test/peer` |
| `pkg/testsyslog` | `internal/test/syslog` |
| `test/functional` | `internal/test/runner` |
| `test/ciformat` | `internal/test/ci` |

## Implementation Summary

### What Was Implemented
- Moved 14 packages from `pkg/` to `internal/`
- Moved `pkg/editor` → `internal/config/editor` (grouped under config)
- Moved `pkg/testpeer` → `internal/test/peer` (package renamed to `peer`)
- Moved `pkg/testsyslog` → `internal/test/syslog` (package renamed to `syslog`)
- Moved `test/functional` → `internal/test/runner` (package renamed to `runner`)
- Moved `test/ciformat` → `internal/test/ci` (package renamed to `ci`)
- Updated all imports across ~200+ Go files
- Updated all documentation (~100+ .md files)
- Deleted empty `pkg/` directory

### Bugs Found/Fixed
- Variable shadowing in `cmd/ze-test/run.go`: local `runner` shadowed package `runner`
- Variable shadowing in `test/integration/testpeer_test.go`: local `peer` shadowed package `peer`
- gofmt issue in `internal/rib/store.go`

### Design Insights
- Package renames (testpeer→peer, functional→runner) can cause variable shadowing issues
- Bulk sed replacements on .md files can corrupt spec files if they contain "before" state

## Checklist

### 🧪 TDD
- [x] Tests written (existing tests cover this)
- [x] Tests FAIL (N/A - refactoring only)
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (90 tests: 37 encoding, 23 plugin, 12 parsing, 18 decoding)
- [x] `make build` passes

### Completion
- [x] pkg/pool vs internal/pool resolved (renamed to delete-pool)
- [x] internal/test/ directory created
- [x] pkg/* moved to internal/* (14 packages)
- [x] pkg/testpeer → internal/test/peer
- [x] pkg/testsyslog → internal/test/syslog
- [x] test/functional → internal/test/runner
- [x] test/ciformat → internal/test/ci
- [x] All imports updated
- [x] All package declarations updated
- [x] pkg/ deleted
- [x] Documentation updated
- [ ] Spec moved to `docs/plan/done/`
