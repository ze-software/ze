# Spec: archive-credential-sanitization

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/2 |
| Updated | 2026-07-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/archive/archive.go` - credential leak sites
4. `internal/component/config/archive/cmd/archive.go` - success message leak site

## Task

`archive.go` includes raw URLs in error messages at lines 143, 188, 196, 206, and 216.
`cmd/archive.go` includes raw URL in a success message at line 131.
If the archive location contains embedded credentials (`https://user:pass@host/...`),
those credentials appear in error/log output. Fix by calling `url.URL.Redacted()` or
a helper wrapper before including URLs in any messages.

**Origin:** VyOS vyos-1x T8956 (2026-06 fix for TftpC.upload() leaking URL credentials).

## Required Reading

### Architecture Docs
- [ ] `internal/component/config/archive/archive.go` - archive upload/download with URL handling
  -> Constraint: `url` package already imported; `parsed` variable available in ValidateLocation and archiveToLocation
- [ ] `internal/component/config/archive/cmd/archive.go` - CLI trigger handler using ac.Location in success message
  -> Constraint: package `cmd` already imports `archive` package
- [ ] `internal/component/config/archive/archive_test.go` - existing test coverage for archive operations

### RFC Summaries (MUST for protocol work)
N/A - not protocol work.

**Key insights:**
- Go's `url.URL.Redacted()` replaces password with `xxxxx` in the string representation
- Six message sites use raw URL strings (5 error, 1 success)
- Other transfer paths (tftpserver, imageserver) are safe: they build URLs from separate fields
- Where `parsed` is already available, use `parsed.Redacted()` directly; where not, use a helper

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/archive/archive.go` - archive operations with URL handling
- [ ] `internal/component/config/archive/cmd/archive.go` - CLI trigger handler

**Behavior to preserve:**
- Archive upload/download functionality
- Error reporting (content, not URL format)
- All existing test assertions

**Behavior to change:**
- Error messages at archive.go lines 143, 188, 196, 206, 216 must sanitize URLs before inclusion
- Success message at cmd/archive.go line 131 must sanitize URL before inclusion

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- User-configured archive location (URL with possible embedded credentials)

### Transformation Path
1. `ArchiveConfig.Location` read from config
2. URL parsed for archive operations
3. On error, URL included in error message (LEAK POINT)
4. On success, URL included in response message (LEAK POINT)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> archive | Location string from config tree | [ ] |
| Archive -> error output | Error messages with raw URL | [ ] |
| Archive -> success output | Success message with raw URL | [ ] |

### Integration Points
- Config system provides `Location` field
- Error messages propagate to logs/CLI
- Success messages return to CLI via plugin response

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `Location` field can contain embedded credentials | URL format supports `user:pass@host` | No leak if not possible | Check YANG schema for location field | confirmed |
| A-2 | Six sites are the only leak points in archive package | grep of archive package for raw URL in error/message paths | Other sites could exist | Full grep for Location/rawURL/baseURL in fmt.Errorf and textbuf calls | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Other packages may also log URLs with credentials | Grep finds additional sites | Extend fix to all URL-logging code |

## Files to Modify

| File | What | Why |
|------|------|-----|
| `internal/component/config/archive/archive.go` | Add `RedactURL` helper; use `parsed.Redacted()` or helper in 5 error messages | Sanitize credentials before logging |
| `internal/component/config/archive/cmd/archive.go` | Use `archive.RedactURL` in success message | Sanitize credentials in response |
| `internal/component/config/archive/archive_test.go` | Add credential redaction tests | Verify AC-1, AC-2, AC-3 |

## TDD Test Plan

| Test | Validates | Phase |
|------|-----------|-------|
| `TestRedactURL_WithCredentials` | `RedactURL("https://user:pass@host/path")` returns `"https://user:xxxxx@host/path"` | 1 |
| `TestRedactURL_WithoutCredentials` | `RedactURL("https://host/path")` returns unchanged | 1 |
| `TestRedactURL_InvalidURL` | `RedactURL("://bad")` returns the raw string | 1 |
| `TestValidateLocation_RedactsCredentials` | `ValidateLocation("ftp://user:pass@host")` error does not contain "pass" | 1 |
| `TestToHTTP_RedactsCredentials` | `ToHTTP` error for credentialed URL does not contain password | 2 |

## Implementation Phases

### Phase 1: Helper + ValidateLocation + archiveToLocation
- Add `RedactURL` helper function
- Fix `ValidateLocation` (line 143): use `parsed.Redacted()`
- Fix `archiveToLocation` (lines 206, 216): use `RedactURL` / `parsed.Redacted()`
- Tests: RedactURL tests, ValidateLocation redaction test

### Phase 2: ToHTTP + cmd/archive.go
- Fix `ToHTTP` (lines 188, 196): use `RedactURL(baseURL)`
- Fix `cmd/archive.go` (line 131): use `archive.RedactURL(ac.Location)`
- Tests: ToHTTP redaction test

## Wiring Test (MANDATORY)
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Archive error with credentialed URL | -> | Redacted URL in error message | `TestValidateLocation_RedactsCredentials` |
| Archive HTTP error with credentialed URL | -> | Redacted URL in HTTP error message | `TestToHTTP_RedactsCredentials` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Archive location URL contains `user:pass@host` | Error messages show `user:xxxxx@host` |
| AC-2 | Archive location URL without credentials | Error messages unchanged |
| AC-3 | `ValidateLocation` with credentialed URL | Validation error shows redacted URL |

## End-to-End User Stories (MANDATORY for new features)
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Config archive with credentialed URL, operation fails | Error message contains redacted URL, not plaintext password | `TestValidateLocation_RedactsCredentials`, `TestToHTTP_RedactsCredentials` |

## Critical Review Checklist
| # | What to verify | Pass criteria |
|---|---------------|---------------|
| 1 | All 6 leak sites sanitized | grep for raw URL vars in fmt.Errorf/Str calls returns 0 matches |
| 2 | `RedactURL` handles error from url.Parse | Fallback to raw string, no panic |
| 3 | Existing tests still pass | `make ze-unit-test` green |
| 4 | No new allocations on hot paths | RedactURL only called in error/message paths |

## Deliverables Checklist
| # | Deliverable | Verification method |
|---|-------------|-------------------|
| 1 | `RedactURL` function in archive.go | `grep -n 'func RedactURL' internal/component/config/archive/archive.go` |
| 2 | All 6 leak sites use redacted URLs | `grep -n 'rawURL\|baseURL\|ac\.Location' archive.go cmd/archive.go` in error/message contexts |
| 3 | Tests for redaction behavior | `go test ./internal/component/config/archive/ -run Redact -v` |

## Security Review Checklist
| # | Concern | What to check |
|---|---------|--------------|
| 1 | Credential exposure in error messages | All URL-containing error messages use Redacted/RedactURL |
| 2 | Credential exposure in success messages | cmd/archive.go success path uses RedactURL |
| 3 | No new credential surfaces introduced | No new fmt.Errorf/log calls with raw URLs |

## Documentation Update Checklist
| # | Category | Applies? | File + what to update |
|---|----------|----------|----------------------|
| 1 | Feature list | No | No user-facing feature change |
| 2 | User guide | No | No usage change |
| 3 | Config syntax | No | No config change |
| 4 | CLI reference | No | No CLI change |
| 5 | API/RPC docs | No | No API change |
| 6 | Plugin SDK | No | No SDK change |
| 7 | Wire format | No | No wire change |
| 8 | RFC compliance | No | Not protocol work |
| 9 | Comparison table | No | N/A |
| 10 | Test infrastructure | No | Standard unit tests |
| 11 | Architecture design | No | No architectural change |

## Implementation Summary

**Changes:**
| File | What changed | Why |
|------|-------------|-----|
| `internal/component/config/archive/archive.go` | Added `RedactURL` helper; fixed 5 error messages to use `RedactURL()` or `parsed.Redacted()` | Sanitize embedded credentials before including URLs in error output |
| `internal/component/config/archive/cmd/archive.go` | Used `archive.RedactURL(ac.Location)` in success message | Sanitize credentials in CLI response |
| `internal/component/config/archive/archive_test.go` | Added 7 tests: 3 for `RedactURL`, 2 for `ValidateLocation` redaction, 1 for `ToHTTP` redaction | Verify AC-1, AC-2, AC-3 |

**Deviations:** Spec originally identified 4 leak sites; implementation found and fixed 6 (added archiveToLocation unsupported-scheme error and cmd/archive.go success message).

## Review Gate

Findings: 0 BLOCKER, 0 ISSUE. All 6 leak sites verified sanitized via grep. Lint clean (exit 0). All archive tests pass.

## Pre-Commit Verification

- Files Exist: `archive.go` (modified), `cmd/archive.go` (modified), `archive_test.go` (modified)
- AC-1 Verified: `TestRedactURL_WithCredentials` passes, `TestToHTTP_RedactsCredentials` passes
- AC-2 Verified: `TestRedactURL_WithoutCredentials` passes (3 subtests)
- AC-3 Verified: `TestValidateLocation_RedactsCredentials` and `TestValidateLocation_NoSchemeRedactsCredentials` pass
- Assumptions Resolved: A-1 confirmed (YANG `type string`, no constraint), A-2 confirmed (grep found 6 sites, all fixed)

## Known Limitations
- Low severity: archive locations come from local config, errors not sent to remote systems
