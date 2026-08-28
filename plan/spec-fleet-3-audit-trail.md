# Spec: fleet-3 -- Fleet Audit Trail

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-fleet-1-device-registry |
| Phase | - |
| Updated | 2026-05-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/component/plugin/server/managed.go` -- ManagedConfigService (hook points)
4. `spec-fleet-0-umbrella.md` -- umbrella design decisions
5. `spec-fleet-1-device-registry.md` -- device registry (dependency)

## Task

Add a centralized, append-only audit log for fleet operations. Every significant fleet
event is recorded with a timestamp, actor, device, and structured payload. The log is
stored in the hub's ZeFS blob and queryable through CLI and web.

Currently there is no record of who pushed what config to which device and when. The
per-device transaction protocol logs locally, but there is no hub-side view. An operator
investigating "who changed edge-42's config at 3am?" has no answer.

### Key Design Decisions (from umbrella)

| Decision | Detail |
|----------|--------|
| Storage | Append-only entries in ZeFS blob. Keyed by timestamp + sequence number |
| No external dependency | No syslog, no database. ZeFS is sufficient at fleet scale (hundreds of devices) |
| Structured entries | JSON-encoded event with type, timestamp, device, actor, payload |
| Retention | Configurable max entries (default 10000). Oldest pruned on overflow |
| Query | By device, by event type, by time range. CLI and web |
| Fleet-6/7 event types | Audit must also record the new fleet-control events: `fleet disable` and `fleet enable` (actor, device, time), reconnect `diverged` detected, `config-push` received, and divergence resolution (`adopt` / `revert`, with which config won). These are high-value security/forensic events. Add them to the event-type set when fleet-3 is designed |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/fleet-config.md` -- existing fleet config protocol
  -> Constraint: audit hooks into existing protocol flow, no protocol changes
- [ ] `internal/component/plugin/server/managed.go` -- RegisterClient, UnregisterClient, HandleConfigFetch
  -> Decision: audit calls added after each operation (non-blocking)

**Key insights:**
- Audit entries must not block the config-fetch path (async append)
- config-ack carries ok/error; both outcomes are audit-worthy
- Device connect/disconnect from fleet-1 registry are natural audit points

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/managed.go` -- no audit logging exists
  -> Constraint: audit is purely additive; all existing behavior unchanged
- [ ] `internal/component/config/archive/archive.go` -- config archive system (different purpose: archives config content, not operational events)
  -> Decision: audit log is separate from config archive (different granularity and purpose)

**Behavior to preserve:**
- All existing ManagedConfigService behavior unchanged
- Config archive system unchanged (complementary, not replaced)
- Config-fetch latency not impacted by audit (async writes)

**Behavior to change:**
- Audit entry appended on: device connect, device disconnect, config-fetch, config-ack (ok), config-ack (reject), config-changed sent, template change (fleet-2)
- New CLI commands: `show fleet audit [--device <name>] [--type <type>] [--since <time>]`
- New web view: audit log table on fleet dashboard

## Data Flow (MANDATORY)

### Entry Point
- Fleet operations trigger audit entries (connect, disconnect, config events)
- Operator queries audit via CLI or web

### Transformation Path
1. Fleet operation occurs (e.g., HandleConfigFetch completes)
2. Audit entry constructed: type, timestamp, device name, actor, payload
3. Entry appended to audit log in ZeFS blob (async, non-blocking)
4. CLI/web queries read audit log from blob with optional filters

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Fleet operation to Audit | Audit.Append() call after operation | [ ] |
| Audit to ZeFS | Blob append for persistence | [ ] |
| Audit to CLI/Web | Query API with filters | [ ] |

### Integration Points
- `ManagedConfigService` -- audit calls in RegisterClient, UnregisterClient, HandleConfigFetch
- `DeviceRegistry` from fleet-1 -- device context for audit entries
- CLI -- `show fleet audit` command family
- Web -- audit log table on fleet page

### Architectural Verification
- [ ] No bypassed layers (audit hooks into existing operation path)
- [ ] No unintended coupling (audit is append-only observer; removal has no functional impact)
- [ ] No duplicated functionality (separate from config archive)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Device connects to hub | -> | Audit entry: device.connect | `test/managed/fleet-audit-log.ci` |
| Device completes config-fetch | -> | Audit entry: config.fetch | `test/managed/fleet-audit-log.ci` |
| `show fleet audit` CLI | -> | Audit log query with entries | `test/managed/fleet-audit-log.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Device connects to hub | Audit entry recorded: type=device.connect, device=name, timestamp |
| AC-2 | Device disconnects | Audit entry: type=device.disconnect, device=name |
| AC-3 | Device completes config-fetch (new config) | Audit entry: type=config.fetch, device=name, version=hash |
| AC-4 | Device sends config-ack (ok) | Audit entry: type=config.ack, device=name, version=hash, ok=true |
| AC-5 | Device sends config-ack (reject) | Audit entry: type=config.reject, device=name, version=hash, error=message |
| AC-6 | Hub sends config-changed | Audit entry: type=config.push, device=name, version=hash |
| AC-7 | `show fleet audit` | All recent audit entries shown, newest first |
| AC-8 | `show fleet audit --device edge-01` | Only entries for edge-01 |
| AC-9 | `show fleet audit --type config.fetch` | Only config.fetch entries |
| AC-10 | `show fleet audit --since 2026-05-15T00:00:00Z` | Only entries after given time |
| AC-11 | Audit log exceeds max entries (default 10000) | Oldest entries pruned |
| AC-12 | Hub restarts | Audit log loaded from ZeFS blob; entries preserved |
| AC-13 | Config-fetch with audit append failure (blob write error) | Config-fetch still succeeds; audit failure logged but not propagated |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAuditLogAppend` | `internal/component/plugin/server/audit_test.go` | Entry appended and retrievable | |
| `TestAuditLogPersist` | `internal/component/plugin/server/audit_test.go` | Entries survive reload from blob | |
| `TestAuditLogFilterDevice` | `internal/component/plugin/server/audit_test.go` | Filter by device name | |
| `TestAuditLogFilterType` | `internal/component/plugin/server/audit_test.go` | Filter by event type | |
| `TestAuditLogFilterSince` | `internal/component/plugin/server/audit_test.go` | Filter by timestamp | |
| `TestAuditLogPrune` | `internal/component/plugin/server/audit_test.go` | Oldest entries pruned at max capacity | |
| `TestAuditLogAsyncNonBlocking` | `internal/component/plugin/server/audit_test.go` | Append does not block caller | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-audit-log` | `test/managed/fleet-audit-log.ci` | Device connects, fetches config, disconnects; all events in audit log | |

## Files to Modify
- `internal/component/plugin/server/managed.go` -- add audit calls to RegisterClient, UnregisterClient, HandleConfigFetch

## Files to Create
- `internal/component/plugin/server/audit.go` -- AuditLog type, append, query, persistence, pruning
- `internal/component/plugin/server/audit_test.go` -- unit tests
- `test/managed/fleet-audit-log.ci` -- functional test

## Implementation Steps

### Implementation Phases

1. **Phase: AuditLog type** -- append-only log with structured entries, persistence, query, pruning
   - Tests: `TestAuditLogAppend`, `TestAuditLogPersist`, `TestAuditLogFilterDevice`, `TestAuditLogFilterType`, `TestAuditLogFilterSince`, `TestAuditLogPrune`
   - Files: `audit.go`, `audit_test.go`
   - Verify: unit tests pass

2. **Phase: Wire into ManagedConfigService** -- audit calls after operations (non-blocking)
   - Tests: `TestAuditLogAsyncNonBlocking`, `fleet-audit-log.ci`
   - Files: `managed.go`, `audit.go`
   - Verify: audit entries created for all fleet operations

3. **Phase: CLI + Web** -- `show fleet audit` commands, web audit table
   - Tests: `fleet-audit-log.ci`
   - Files: CLI command registration, web page extension
   - Verify: audit queryable via CLI and web

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All AC-N have implementation |
| Correctness | Audit does not block config-fetch path; async append |
| Naming | Event types use dot-separated kebab-case (device.connect, config.fetch) |
| Data flow | Operation -> async Append -> blob persist -> CLI/web query |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Audit integrity | Append-only; no delete or modify API exposed |
| Information leakage | Audit entries do not contain config content or secrets; only version hashes |
| Resource exhaustion | Max entries bounded; pruning prevents unbounded growth |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete
- [ ] `./le verify current mode full` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
