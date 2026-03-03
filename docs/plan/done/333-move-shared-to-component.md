# Spec: move-shared-to-component

## Task

Move `internal/plugin/bgp/shared/` to `internal/component/bgp/`. BGP is a subsystem/component, not a plugin. The shared route/event/format types are BGP domain concepts consumed by BGP plugins — they belong in the component layer alongside bus, config, engine, plugin. The name "shared" is a Go anti-pattern (describes relationship, not content).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` — shared BGP types location table
  → Constraint: table references shared types location, must update
- [ ] `docs/architecture/api/text-parser.md` — proposed shared parser location
  → Constraint: references `internal/plugin/bgp/shared/textparse.go`, must update

**Key insights:**
- BGP is a subsystem (CLAUDE.md: "Subsystem ≠ Plugin"), belongs in `internal/component/`
- `internal/component/` already has bus, config, engine, plugin — established pattern
- `internal/plugin/` is infrastructure (registry, process, hub) — not where domain types go

## Current Behavior

**Source files read:**
- [ ] `internal/plugin/bgp/shared/route.go` — Route struct, RouteKey func
- [ ] `internal/plugin/bgp/shared/event.go` — Event struct, ParseEvent, peer helpers, raw field extraction
- [ ] `internal/plugin/bgp/shared/format.go` — FormatAnnounceCommand, FormatWithdrawCommand, deprecated FormatRouteCommand wrapper
- [ ] `internal/plugin/bgp/shared/nlri.go` — ParseNLRIValue helper
- [ ] `internal/plugin/bgp/shared/event_test.go` — 7 test functions
- [ ] `internal/plugin/bgp/shared/format_test.go` — 8 test functions

**Consumers:**
- [ ] `internal/plugins/bgp-rib/event.go` — type aliases + function aliases to shared
- [ ] `internal/plugins/bgp-adj-rib-in/rib.go` — direct shared.ParseEvent, shared.RouteKey, shared.ParseNLRIValue
- [ ] `internal/plugins/bgp-adj-rib-in/rib_test.go` — direct shared.Event, shared.MessageInfo, shared.FamilyOperation
- [ ] `internal/plugins/bgp-watchdog/config.go` — direct shared.Route, shared.FormatAnnounceCommand, shared.FormatWithdrawCommand

**Behavior to preserve:**
- All type definitions, function signatures, and logic unchanged
- All existing tests continue to pass
- bgp-rib package-internal aliases (`formatRouteCommand`, `parseEvent`, etc.) still work

**Behavior to change:**
- Package name: `shared` → `bgp`
- Import path: `internal/plugin/bgp/shared` → `internal/component/bgp`
- Delete deprecated `FormatRouteCommand()` wrapper (ze never released, no compat needed per `rules/compatibility.md`)

## Data Flow

### Entry Point
- JSON events from engine → plugins parse via `bgp.ParseEvent()`
- Stored routes → plugins format via `bgp.FormatAnnounceCommand()` / `bgp.FormatWithdrawCommand()`

### Transformation Path
1. Engine sends JSON event over socket
2. Plugin calls `bgp.ParseEvent(data)` → `*bgp.Event`
3. Plugin stores routes as `bgp.Route` structs
4. Plugin formats commands via `bgp.FormatAnnounceCommand(&route)` → string

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine → Plugin | JSON events over unix socket | Not affected by move |
| Plugin → Engine | Text commands over unix socket | Not affected by move |

### Architectural Verification
- No bypassed layers — pure package rename/move
- No unintended coupling — dependency direction improved (component consumed by plugins)
- No duplicated functionality — moved, not copied
- Zero-copy not applicable — these are parsed JSON types, not wire types

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| JSON event string | → | `bgp.ParseEvent()` | `TestParseEvent_ZeBGPUpdateFormat` |
| Route struct | → | `bgp.FormatAnnounceCommand()` | `TestFormatAnnounceCommand_MinimalRoute` |
| Route struct | → | `bgp.FormatWithdrawCommand()` | `TestFormatWithdrawCommand` |
| NLRI value | → | `bgp.ParseNLRIValue()` | `TestParseNLRIValue` |
| Family+prefix+pathID | → | `bgp.RouteKey()` | `TestRouteKey` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Import `internal/component/bgp` | Package provides Route, Event, FormatAnnounceCommand, FormatWithdrawCommand, ParseEvent, ParseNLRIValue, RouteKey |
| AC-2 | `internal/plugin/bgp/shared/` deleted | No Go files reference old import path |
| AC-3 | bgp-rib, bgp-adj-rib-in, bgp-watchdog updated | All compile and tests pass with new import |
| AC-4 | `FormatRouteCommand` removed | Only `FormatAnnounceCommand` exists |
| AC-5 | Active architecture docs | Reference `internal/component/bgp/` not old path |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseEvent_ZeBGPUpdateFormat` | `internal/component/bgp/event_test.go` | ze-bgp JSON update parsing | ✅ Pass |
| `TestParseEvent_StateFormat` | `internal/component/bgp/event_test.go` | Peer state event parsing | ✅ Pass |
| `TestParseEvent_FormatFullRawFields` | `internal/component/bgp/event_test.go` | Raw hex field extraction | ✅ Pass |
| `TestParseEvent_PeerFormats` | `internal/component/bgp/event_test.go` | Flat and nested peer formats | ✅ Pass |
| `TestParseEvent_MultipleFamilies` | `internal/component/bgp/event_test.go` | Multi-family UPDATE parsing | ✅ Pass |
| `TestParseEvent_InvalidJSON` | `internal/component/bgp/event_test.go` | Error on malformed input | ✅ Pass |
| `TestParseNLRIValue` | `internal/component/bgp/event_test.go` | String and structured NLRI formats | ✅ Pass |
| `TestRouteKey` | `internal/component/bgp/event_test.go` | Unique key generation with path-id | ✅ Pass |
| `TestFormatAnnounceCommand_MinimalRoute` | `internal/component/bgp/format_test.go` | Minimal required fields | ✅ Pass |
| `TestFormatAnnounceCommand_FullAttributes` | `internal/component/bgp/format_test.go` | All attribute fields | ✅ Pass |
| `TestFormatAnnounceCommand_WithPathID` | `internal/component/bgp/format_test.go` | RFC 7911 path-id | ✅ Pass |
| `TestFormatAnnounceCommand_IPv6` | `internal/component/bgp/format_test.go` | IPv6 family and addresses | ✅ Pass |
| `TestFormatAnnounceCommand_ExtendedCommunities` | `internal/component/bgp/format_test.go` | Large and extended communities | ✅ Pass |
| `TestFormatAnnounceCommand_VPN` | `internal/component/bgp/format_test.go` | VPN with RD and labels | ✅ Pass |
| `TestFormatAnnounceCommand_NhopSelf` | `internal/component/bgp/format_test.go` | nhop self keyword | ✅ Pass |
| `TestFormatWithdrawCommand` | `internal/component/bgp/format_test.go` | Withdrawal command variants | ✅ Pass |

## Files to Modify

- `internal/plugins/bgp-rib/event.go` — update import and references
- `internal/plugins/bgp-adj-rib-in/rib.go` — update import and references
- `internal/plugins/bgp-adj-rib-in/rib_test.go` — update import and references
- `internal/plugins/bgp-watchdog/config.go` — update import and references
- `docs/architecture/api/architecture.md` — update shared types path
- `docs/architecture/api/text-parser.md` — update proposed parser path

## Files to Create

- `internal/component/bgp/route.go` — Route struct, RouteKey (from shared/route.go)
- `internal/component/bgp/event.go` — Event, ParseEvent, peer helpers (from shared/event.go)
- `internal/component/bgp/format.go` — FormatAnnounceCommand, FormatWithdrawCommand (from shared/format.go)
- `internal/component/bgp/nlri.go` — ParseNLRIValue (from shared/nlri.go)
- `internal/component/bgp/event_test.go` — event tests (from shared/event_test.go)
- `internal/component/bgp/format_test.go` — format tests (from shared/format_test.go)

## Files to Delete

- `internal/plugin/bgp/shared/route.go`
- `internal/plugin/bgp/shared/event.go`
- `internal/plugin/bgp/shared/format.go`
- `internal/plugin/bgp/shared/nlri.go`
- `internal/plugin/bgp/shared/event_test.go`
- `internal/plugin/bgp/shared/format_test.go`
- `internal/plugin/bgp/` directory (empty after removal)

## Implementation Summary

### What Was Implemented
- Moved 6 files from `internal/plugin/bgp/shared/` to `internal/component/bgp/`
- Changed package name from `shared` to `bgp`
- Updated 4 consumer files (bgp-rib, bgp-adj-rib-in, bgp-watchdog) to use new import
- Deleted deprecated `FormatRouteCommand()` wrapper
- Updated test names from `TestFormatRouteCommand_*` to `TestFormatAnnounceCommand_*`
- Updated 2 active architecture docs

### Bugs Found/Fixed
- None

### Documentation Updates
- `docs/architecture/api/architecture.md` — shared types path updated
- `docs/architecture/api/text-parser.md` — proposed parser path updated

### Deviations from Plan
- None

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Move shared types to component/bgp | ✅ Done | `internal/component/bgp/` | 6 files moved |
| Rename package shared → bgp | ✅ Done | All new files: `package bgp` | |
| Update consumers | ✅ Done | bgp-rib, bgp-adj-rib-in, bgp-watchdog | 4 files |
| Delete old directory | ✅ Done | `internal/plugin/bgp/` removed | |
| Delete deprecated FormatRouteCommand | ✅ Done | Removed from format.go | Per rules/compatibility.md |
| Update active docs | ✅ Done | architecture.md, text-parser.md | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `go vet ./internal/component/bgp/` passes | All types/funcs exported |
| AC-2 | ✅ Done | `grep 'internal/plugin/bgp/shared' **/*.go` = 0 matches | |
| AC-3 | ✅ Done | `go vet` passes for all 3 consumer packages | |
| AC-4 | ✅ Done | `grep FormatRouteCommand internal/component/bgp/format.go` = 0 | |
| AC-5 | ✅ Done | architecture.md:24, text-parser.md:176 | Both updated |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| All 16 tests | ✅ Pass | `internal/component/bgp/*_test.go` | Moved from shared, all pass |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/route.go` | ✅ Created | package bgp, Route + RouteKey |
| `internal/component/bgp/event.go` | ✅ Created | package bgp, Event + ParseEvent + helpers |
| `internal/component/bgp/format.go` | ✅ Created | package bgp, FormatAnnounceCommand + FormatWithdrawCommand |
| `internal/component/bgp/nlri.go` | ✅ Created | package bgp, ParseNLRIValue |
| `internal/component/bgp/event_test.go` | ✅ Created | 8 test functions |
| `internal/component/bgp/format_test.go` | ✅ Created | 8 test functions |
| `internal/plugins/bgp-rib/event.go` | ✅ Modified | shared → bgp |
| `internal/plugins/bgp-adj-rib-in/rib.go` | ✅ Modified | shared → bgp |
| `internal/plugins/bgp-adj-rib-in/rib_test.go` | ✅ Modified | shared → bgp |
| `internal/plugins/bgp-watchdog/config.go` | ✅ Modified | shared → bgp |
| `docs/architecture/api/architecture.md` | ✅ Modified | Path updated |
| `docs/architecture/api/text-parser.md` | ✅ Modified | Path updated |
| `internal/plugin/bgp/shared/*` | ✅ Deleted | 6 files + directory |

### Audit Summary
- **Total items:** 28
- **Done:** 28
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Design Insights

- BGP subsystem types used by plugins belong in `internal/component/bgp/`, not plugin infrastructure
- Go package naming: name for what it *provides* (bgp domain types), not its relationship to consumers (shared)
- `format.go` still imports `internal/plugins/bgp/attribute` for `FormatASPath()` — resolves when full BGP subsystem migrates to `internal/component/bgp/` in arch-0

## Verification

`make test-all` — lint, unit, functional pass. One pre-existing fuzz timeout in `bgp/attribute` (`FuzzParseExtCommunity` context deadline exceeded) — unrelated, that package was not touched.
