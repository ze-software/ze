# Spec: cpe-5-firmware-update

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-05-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/system/schema/ze-system-conf.yang` - system config schema
4. `internal/component/gokrazy/` - gokrazy integration component

## Task

Add a firmware/image update-check mechanism to Ze. On gokrazy appliances, Ze should periodically fetch a version manifest from a configured URL, compare it against the running version, and report when a newer image is available. This does NOT perform the update itself (gokrazy handles that); it only checks and reports.

Modeled as a system config extension (alongside host, dns, tuning) with a background goroutine that polls periodically.

**Motivation:** VyOS home.conf uses `system { update-check { url http://archive.surfprotect.co.uk/public/iso/version.json } }`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component lifecycle, event bus
  → Constraint: update check emits events on the bus, does not side-effect
- [ ] `internal/component/config/system/schema/ze-system-conf.yang` - system config children
  → Decision: add update-check container alongside existing children
- [ ] `internal/component/gokrazy/gokrazy.go` - gokrazy integration, version detection
  → Decision: use gokrazy build info for running version comparison

**Key insights:**
- Version manifest is a JSON document with at minimum a `version` field
- Check interval defaults to daily (86400s) if not specified
- Result exposed via CLI (`show system update`) and web UI system page
- No auto-update: check and report only
- On non-gokrazy platforms, running version comes from build-time ldflags

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/system/schema/ze-system-conf.yang` - system config (host, dns, tuning, archive, peeringdb)
- [ ] `internal/component/gokrazy/gokrazy.go` - gokrazy platform integration
- [ ] `internal/component/web/page_system.go` - system status web page
- [ ] `cmd/ze/show/system.go` - CLI show system commands (if exists)

**Behavior to preserve:**
- Existing system config schema unchanged
- Gokrazy component lifecycle unchanged
- Web UI system page structure preserved

**Behavior to change:**
- Add `update-check` container to system YANG schema
- New background goroutine polls version URL
- Bus event emitted when new version detected
- CLI and web UI show update availability status

## Data Flow (MANDATORY)

### Entry Point
- Config commit with `system { update-check { url "https://..."; } }`

### Transformation Path
1. YANG schema parsed, URL validated as non-empty string
2. Config extracted by system config loader
3. Background goroutine started with configured interval (default 86400s)
4. HTTP GET to version URL, parse JSON response
5. Compare remote version against running version
6. If newer: emit `system.update.available` bus event with version details
7. Store last-check result in memory for CLI/web queries

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> system loader | config tree extract at startup/reload | [ ] |
| System -> HTTP | outbound HTTP GET to version URL | [ ] |
| System -> event bus | `system.update.available` event | [ ] |
| CLI/web -> update state | in-memory query | [ ] |

### Integration Points
- `internal/component/config/system/` - system config extraction
- `internal/component/web/page_system.go` - web UI display
- Event bus - update availability notifications

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling (check-only, no side effects)
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| config commit with update-check block | → | update checker start | `test/system/update-check.ci` |
| periodic timer fires | → | HTTP fetch + version compare | `TestUpdateCheckFetch` |
| newer version detected | → | bus event emitted | `TestUpdateCheckEvent` |
| `show system update` CLI | → | status display | `TestUpdateCheckCLI` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with `update-check { url "https://..."; }` | Config parses and validates |
| AC-2 | Timer fires at configured interval | HTTP GET to version URL executed |
| AC-3 | Remote version newer than running | `system.update.available` event emitted with version string |
| AC-4 | Remote version same or older | No event emitted, status shows "up to date" |
| AC-5 | HTTP fetch fails (timeout, DNS, 404) | Error logged, retry at next interval, no crash |
| AC-6 | `show system update` CLI command | Shows last check time, running version, available version (if any) |
| AC-7 | Config with `interval 3600` | Check runs hourly instead of daily |
| AC-8 | Config deleted (update-check removed) | Background goroutine stopped cleanly |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUpdateCheckConfigParse` | `internal/component/config/system/update_test.go` | Config extraction from tree | |
| `TestUpdateCheckFetch` | `internal/component/config/system/update_test.go` | HTTP fetch + JSON parse with httptest server | |
| `TestUpdateCheckEvent` | `internal/component/config/system/update_test.go` | Bus event emitted on newer version | |
| `TestUpdateCheckCLI` | `internal/component/config/system/update_test.go` | CLI output format | |
| `TestUpdateCheckFetchError` | `internal/component/config/system/update_test.go` | Graceful handling of HTTP errors | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-update-check` | `test/system/update-check.ci` | Configure update-check, verify periodic check runs, verify CLI output | |

## Files to Modify
- `internal/component/config/system/schema/ze-system-conf.yang` - add update-check container
- `internal/component/gokrazy/gokrazy.go` - wire update check goroutine (if gokrazy-specific)

## Files to Create
- `internal/component/config/system/update.go` - update check logic (fetch, compare, emit)
- `internal/component/config/system/update_test.go` - unit tests
- `test/system/update-check.ci` - functional test

## Implementation Steps

### Implementation Phases

1. **Phase: YANG schema** - Add update-check container to ze-system-conf.yang (url, interval)
2. **Phase: Config parsing** - Extract update-check config from tree
3. **Phase: Version fetch** - HTTP GET + JSON parse with httptest-based tests
4. **Phase: Version compare** - Semantic version comparison logic
5. **Phase: Bus event** - Emit `system.update.available` on newer version
6. **Phase: CLI integration** - `show system update` command
7. **Phase: Web UI** - Update status on system page
8. **Phase: Functional tests** - End-to-end with mock HTTP server

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All ACs have implementation |
| Correctness | Version comparison handles semver correctly |
| Naming | Bus event name follows project conventions |
| Data flow | Config -> goroutine -> HTTP -> compare -> event |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | URL must be HTTPS (or explicit opt-in for HTTP); JSON response bounded |
| Network safety | HTTP timeout configured, response size limited |
| No auto-update | Check only, never download or apply images |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Architecture docs updated

### Quality Gates (SHOULD pass)
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Implementation Audit complete

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
