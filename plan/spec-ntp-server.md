# Spec: ntp-server

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/plugins/ntp/` - current client-only NTP
4. `ai/rules/planning.md` - skeleton; full DESIGN not yet done

## Task

**Large feature area — skeleton only. Full design not started.**

Ze's NTP support is **client-only**: it queries upstream servers (`beevik/ntp`) and
disciplines the local clock; it has no server/listener side. Consequences:

- Ze cannot serve time to downstream clients at all.
- There is no **local stratum / orphan mode**: when Ze loses every upstream, it
  cannot keep serving time from its own clock at a configured stratum. It simply
  reports unsynced and retries.

Add an NTP server capability, including a local-reference/local-stratum option so
Ze can continue serving clients through upstream loss. This is a substantial effort
(NTP server protocol, access control, packet handling) and must go through the full
`/ze-spec` RESEARCH/DESIGN workflow first.

This skeleton tracks the gap; it is NOT ready to implement.

## Required Reading

### Architecture Docs
- [ ] `internal/plugins/ntp/` - the existing client (query loop, clock discipline) to reuse timing/state.
  → Constraint: the server side listens and answers; keep it separate from the client discipline loop.
- [ ] `ai/rules/config.md` - server enable + access-control config surface.
  → Constraint: serving time to a network is a deliberate operator action; default off.

### RFC Summaries (MUST for protocol work)
- [ ] RFC 5905 (NTPv4) - server mode, stratum, local reference clock semantics.
  → Constraint: create the `rfc/short/` summary during DESIGN before coding.

**Key insights:**
- The client already tracks `Stratum` from upstream responses; a local-stratum server reuses that state to advertise an appropriate stratum, or a fixed local stratum when unsynced.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ntp/ntp.go` - client-only: imports `github.com/beevik/ntp` (ntp.go), queries upstream via `ntp.Query(addr)` (ntp.go); on no reachable upstream it publishes unsynced and retries. No listener.
- [ ] `internal/plugins/ntp/clock_linux.go` - disciplines the local clock (`Settimeofday`/`Adjtimex`); purely the consumer side.
- [ ] `internal/plugins/ntp/yang/ze-ntp-conf.yang` - config exposes upstream `server` list, `interval`, `max-step`, `slew-threshold`, `persist-path`; no `local`/`stratum`/`listen`/server-mode leaf.

**Behavior to preserve:**
- The existing client behaviour (upstream query + clock discipline) is unchanged; server mode is additive and off by default.

**Behavior to change:**
- Add an NTP server that answers client requests, with a local-stratum fallback.

## Data Flow (MANDATORY)

### Entry Point
- Config: new server-mode leaves in `ze-ntp-conf.yang` (e.g. `listen-address`, `allow` access rules, `local stratum <N>`).
- Wire: NTP client requests on UDP 123 arriving at the configured listen address.

### Transformation Path
1. Config parsed into server settings (listen addresses, access rules, local stratum).
2. Server binds UDP 123 and accepts client mode-3 requests.
3. For each request: build a mode-4 reply using the current synced state (stratum from upstream discipline) or, when unsynced and local-stratum is enabled, the configured local stratum.
4. Access rules filter which clients may query.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ ntp server | new YANG server leaves → server settings | [ ] |
| Client discipline ↔ server | shared sync state (stratum, offset) drives replies | [ ] |
| Wire ↔ server | UDP 123 request/response | [ ] |

### Integration Points
- `internal/plugins/ntp/state.go` - existing sync/stratum state feeds server replies.
- `internal/plugins/ntp/ntp.go` - server runs alongside the client query loop.

### Architectural Verification
- [ ] No bypassed layers (config via the standard path)
- [ ] No unintended coupling (server does not distort the client discipline loop)
- [ ] No duplicated functionality (server reuses the existing sync state)
- [ ] Registration over hardcoding — server mode is dhcp/ntp-plugin-local; no central NTP switch in core.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `beevik/ntp` or a small server can produce spec-compliant mode-4 replies | current client uses beevik | may need a dedicated server impl | design spike | unvalidated |
| A-2 | Existing sync state exposes enough (stratum, ref time, dispersion) for replies | `internal/plugins/ntp/state.go` | need richer state | read state.go during RESEARCH | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Serving inaccurate time when unsynced | clients drift | local-stratum default high (less preferred); clear "unsynced" signalling |
| R-2 | Open NTP server abused for amplification | traffic spikes | access rules + default-off + bind to specific interfaces |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `set service ntp local stratum 10` + client query | → | server replies with local stratum when unsynced | `test/plugin/ntp-server.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | server enabled, client mode-3 request | Ze answers with a mode-4 reply |
| AC-2 | upstream synced | reply carries the correct upstream-derived stratum |
| AC-3 | all upstream lost, `local stratum N` set | Ze keeps serving at stratum N |
| AC-4 | client not permitted by access rules | request dropped |
| AC-5 | server disabled (default) | no listener; client behaviour unchanged |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | enables NTP server with local stratum | config → server binds → replies to clients | `test/plugin/ntp-server.ci` |
| 2 | upstream fails, clients keep syncing to Ze | discipline loses upstream → server serves local stratum | `test/plugin/ntp-local-stratum.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNTPServerReply` | `internal/plugins/ntp/server_test.go` | mode-3 request → valid mode-4 reply | |
| `TestNTPLocalStratumFallback` | `internal/plugins/ntp/server_test.go` | serves local stratum when unsynced | |
| `TestNTPServerAccessRules` | `internal/plugins/ntp/server_test.go` | disallowed client dropped | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| local stratum | 1-15 | 15 | 0 | 16 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ntp-server` | `test/plugin/ntp-server.ci` | client obtains time from Ze | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-ntp-server` | `test/interop/scenarios/` | chrony/ntpd client | real NTP client syncs to Ze, incl. local-stratum | |

### Future (if deferring any tests)
- Design to define phasing (server first, then access control, then local stratum).

## Files to Modify
- `internal/plugins/ntp/ntp.go` - run a server alongside the client loop
- `internal/plugins/ntp/state.go` - expose sync state for replies
- `internal/plugins/ntp/yang/ze-ntp-conf.yang` - server + local-stratum leaves

## Files to Create
- `internal/plugins/ntp/server.go` - NTP server (listener + reply builder)
- `internal/plugins/ntp/server_test.go` - unit tests
- `test/plugin/ntp-server.ci` - functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file (skeleton — run `/ze-spec` RESEARCH/DESIGN first) |

### Implementation Phases
1. **RESEARCH/DESIGN (not started)** — full `/ze-spec` workflow: RFC 5905 summary, server-vs-client separation, access-control model, amplification safety. Not implementable as-is.

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Known Limitations
- Skeleton only: acceptance criteria and tests are provisional placeholders for DESIGN.

## Implementation Summary
### What Was Implemented
- Nothing yet (skeleton).

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
- [ ] Full `/ze-spec` DESIGN completed and approved before implementation
- [ ] `./le verify current mode full` passes (after implementation)
- [ ] Feature code integrated (`internal/*`)

### Quality Gates (SHOULD pass)
- [ ] RFC 5905 summary created

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
