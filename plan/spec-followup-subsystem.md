# Spec: followup-subsystem

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `git log -p plan/deferrals.md` (pre-2026-07-06) - original deferral rows + evidence

## Task

Independent subsystem follow-ups: exabgp bridge, DNS, interface phase-4/platform, MCP, and port defaults. Grouped to preserve intent; each becomes its own design when picked up.

This is a consolidation skeleton created from verified deferral survivors (backlog triage 2026-07-06). Each item below was confirmed still-open against the codebase with a producing `file:line`. Split into phases when picked up; the sections after Task are lightweight scaffolding to be filled at design time.

### Work items (migrated from the 2026-07-06 deferral triage; `L#` = row in the pre-triage `plan/deferrals.md`)

- **exabgp-bridge-internal (L61,L62)** - bridge internal-plugin registration for `.ci`/production (SDK/TLS connect-back resolved the transport blocker) (L61); SetWriteDeadline degradation watchdog on non-TCP transports - writes may block indefinitely (L62).
- **dns-secure (L87)** - DNS-over-TLS, DNS-over-HTTPS, DNSSEC validation (core/dnsserver is plain DNS).
- **iface phase 4 + platform (L74,L75,L73)** - SLAAC (L74, DHCP/make-before-break/mirror already shipped); VM-level mirror/DHCPv6-PD/SLAAC tests (L75); macOS/BSD interface plugins `_darwin.go`/`_bsd.go` (L73).
- **mcp follow-ups (L224,L225)** - GET /mcp SSE `.ci` (L224, unit-only today); delete legacy `internal/component/mcp/handler.go` (L225 - now used by chaos orchestrator, so migrate those callers first).
- **port-defaults v2 (L79)** - range-vs-single port conflict detection + YANG-default lint check.

## Required Reading

### Source files / docs

- [ ] `internal/plugins/exabgp/main.go`, `pkg/plugin/rpc/conn.go` (write-deadline)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `internal/core/dnsserver/`, `internal/component/iface/`, `internal/plugins/iface/`
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `internal/component/mcp/handler.go`, `internal/component/mcp/streamable.go`, `internal/component/config/listener.go`
  -> Constraint: verify current behaviour against this source before designing.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; line numbers are pre-triage references)

- [ ] `internal/plugins/exabgp/main.go`
- [ ] `internal/core/dnsserver/`
- [ ] `internal/component/mcp/handler.go`
- [ ] `internal/component/config/listener.go`

**Behavior to preserve:**
- All existing behaviour of the listed files; this backlog work only adds the missing pieces named in the Task work items.

**Behavior to change:**
- Only the specific gaps enumerated in the Task work items.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- exabgp bridge stdin/stdout; DNS queries; iface config; MCP HTTP; port config

### Transformation Path
1. A request enters the relevant subsystem (bridge/DNS/iface/MCP/config)
2. The subsystem handles it (registration, secure transport, SLAAC, SSE, conflict detection)
3. Observable result reflects the added capability

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| bridge <-> engine | stdin/stdout vs internal registration | [ ] |
| MCP client -> server | Streamable HTTP / SSE | [ ] |

### Integration Points
- `internal/plugins/exabgp/`
- `internal/core/dnsserver/`
- `internal/component/mcp/`
- `internal/component/iface/`

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - new commands/views/families/handlers register and are core-discovered, not hardcoded into a core/shared package (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The verified `file:line` evidence in the Task items still holds at design time | 2026-07-06 backlog triage | Re-scope the item | grep/LSP at design time | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Scope drift when the umbrella is split into per-item specs | Item needs its own design doc | Split into a dedicated spec and re-point |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| exabgp bridge runs as an internal plugin | → | registration + `.ci` exercise it | (fill during design) |
| MCP GET /mcp opens an SSE stream | → | `.ci` observes server-initiated frames | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | (define per work item when this skeleton moves to `design`) | (define at design time) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (define at design time) | (define at design time) | per Task work item | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| exabgp-bridge-internal, dns-tls, mcp-get-sse (new) (`.ci`) | test/plugin, test/mcp | each subsystem's behaviour end-to-end | |

## Files to Modify

- `internal/plugins/exabgp/main.go` - see Task work items
- `internal/core/dnsserver/` - see Task work items
- `internal/component/mcp/handler.go` - see Task work items
- `internal/component/config/listener.go` - see Task work items

## Implementation Steps

1. **Phase: split** - if the umbrella covers unrelated items, split into per-item specs first.
2. **Phase: design** - for the chosen item, re-verify the `file:line` evidence and fill the Data Flow / Wiring / AC sections above.
3. **Phase: wiring** - register entry points, write the failing wiring test.
4. **Phase: implement (TDD)** - write test, fail, implement, pass, per work item.
5. **Full verification** - `make ze-verify`.
6. **Complete spec** - fill audit tables, write `plan/learned/NNN-<name>.md`, two-commit closure.

## Checklist

### Goal Gates (MUST pass)
- [ ] Every chosen work item has feature code + test
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Notes
- Skeleton = captured intent, not a designed spec (see `ai/rules/deferral-tracking.md`). Moves to `design` when someone picks it up.
