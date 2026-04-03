# Spec: healthcheck-1-watchdog-med

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-04-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-healthcheck-0-umbrella.md` - umbrella design
4. `internal/component/bgp/plugins/watchdog/server.go` - command dispatch
5. `internal/component/bgp/plugins/watchdog/pool.go` - PoolEntry, dedup logic
6. `internal/component/bgp/plugins/watchdog/config.go` - Route building, PoolEntry creation
7. `internal/component/bgp/format.go` - FormatAnnounceCommand
8. `internal/component/bgp/route.go` - Route struct

## Task

Extend the watchdog `announce` command to accept an optional `med <N>` argument. When present, the watchdog clones the stored Route, overrides MED, calls FormatAnnounceCommand to produce a one-off command, and bypasses the per-peer `announced` dedup. This is a prerequisite for the healthcheck plugin (Phase 2+), which dispatches `watchdog announce <group> med <metric>` to control MED per health state.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component architecture
  -> Constraint: plugins communicate via DispatchCommand, never import siblings
- [ ] `.claude/patterns/plugin.md` - plugin structural template
  -> Constraint: tests use sendRoute hook injection, not real SDK

### RFC Summaries (MUST for protocol work)
N/A -- watchdog is operational tooling, not a BGP protocol feature.

**Key insights:**
- Watchdog handleCommand dispatches to handlePoolAction with announce=true/false
- handlePoolAction extracts args[0] as the pool name, dispatches to handlePoolActionSingle
- handlePoolActionSingle uses AnnouncePool/WithdrawPool which flip the per-peer `announced` boolean
- PoolEntry stores pre-computed AnnounceCmd/WithdrawCmd strings, no Route
- FormatAnnounceCommand accepts a *Route and produces "update text ... med N ..." when MED is set
- Route.MED is *uint32 -- clone-and-set is straightforward
- Config builds Route structs in buildRouteFromAttrs, then pre-computes commands in parseNLRIEntries

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/watchdog/server.go` - handleCommand dispatches "watchdog announce"/"watchdog withdraw" to handlePoolAction. handlePoolAction extracts args[0] as pool name, remaining args ignored. handlePoolActionSingle calls AnnouncePool (which flips announced boolean) then sends entry.AnnounceCmd for each entry.
  -> Constraint: handlePoolAction only reads args[0]. No MED parsing. No Route access.
- [ ] `internal/component/bgp/plugins/watchdog/pool.go` - PoolEntry has Key, AnnounceCmd, WithdrawCmd, initiallyAnnounced, announced map. No Route field. AnnouncePool calls announceForPeer which skips entries where announced[peer]==true (dedup).
  -> Decision: add Route field to PoolEntry. MED override path clones Route, bypasses dedup.
- [ ] `internal/component/bgp/plugins/watchdog/config.go` - parseNLRIEntries builds Route structs via buildRouteFromAttrs, then creates PoolEntry with pre-computed FormatAnnounceCommand/FormatWithdrawCommand strings. Route is discarded after command strings are built.
  -> Decision: store Route in PoolEntry during creation in parseNLRIEntries.
- [ ] `internal/component/bgp/format.go` - FormatAnnounceCommand produces "update text origin ... med N ... nhop ... nlri ... add ..." when route.MED is non-nil.
  -> Constraint: no changes needed to format.go. Clone Route + set MED + call FormatAnnounceCommand produces the correct command string.
- [ ] `internal/component/bgp/route.go` - Route struct with MED *uint32. Value type (struct copy = shallow clone). Slice fields (ASPath, Communities, etc.) share backing arrays on copy, but MED override only changes MED pointer -- no mutation of shared data.
  -> Constraint: struct copy is safe for MED-only override. No deep clone needed.

**Behavior to preserve:**
- `watchdog announce <name>` (no med arg) uses pre-computed AnnounceCmd with existing dedup -- zero behavioral change
- `watchdog withdraw <name>` unchanged
- Per-peer announced/withdrawn state tracking unchanged
- Peer reconnect resend (handleStateUp) sends AnnounceCmd for announced entries -- unchanged
- Wildcard `*` peer dispatch unchanged
- Config parsing and Route building unchanged

**Behavior to change:**
- `watchdog announce <name> med <N>` produces a one-off announce command with overridden MED
- MED override path bypasses the `announced` boolean dedup (always dispatches)
- MED override path still sets `announced[peer] = true` (so subsequent non-MED announce is deduped)
- PoolEntry gains a Route field (populated during config parsing)
- Group name `med` rejected at handlePoolAction with clear error (ambiguous with `med <N>` argument)

## Data Flow (MANDATORY)

### Entry Point
- Text command: `watchdog announce <name> med <N> [peer]`
- Dispatched via DispatchCommand from healthcheck plugin (or any external process)

### Transformation Path
1. handleCommand receives command="watchdog announce", args=["<name>", "med", "<N>"], peer="<addr>"
2. handlePoolAction parses: args[0]=name, detects args[1]=="med", parses args[2] as uint32, remaining=peer
3. handlePoolActionSingle (or handlePoolActionAll for wildcard) receives name + optional MED override
4. MED override path: for each entry, clone entry.Route, set clone.MED=&N, call FormatAnnounceCommand(&clone)
5. Bypass announced dedup: always send the one-off command, then set announced[peer]=true
6. sendRoute(peer, oneOffCmd) delivers to engine

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Caller -> Watchdog | DispatchCommand("watchdog announce <name> med <N>") | [ ] |
| Watchdog -> Engine | sendRoute(peer, "update text ... med N ...") | [ ] |

### Integration Points
- `handlePoolAction` -- extended to parse optional `med <N>` between name and peer args
- `handlePoolActionSingle` -- extended to accept optional MED, bypass dedup when set
- `PoolEntry` -- gains Route field
- `parseNLRIEntries` -- stores Route in PoolEntry alongside pre-computed commands

### Architectural Verification
- [ ] No bypassed layers -- watchdog command dispatch -> pool state -> sendRoute unchanged
- [ ] No unintended coupling -- only watchdog internals modified
- [ ] No duplicated functionality -- extends existing command handler
- [ ] Zero-copy preserved where applicable -- N/A (command path, not wire encoding)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `watchdog announce <name> med 500` command via .ci | -> | handlePoolAction MED parse + FormatAnnounceCommand clone | `test/plugin/watchdog-med-override.ci` |
| `watchdog announce <name> med 100` then `med 1000` | -> | Dedup bypassed, both produce UpdateRoute | `test/plugin/watchdog-med-override.ci` |
| `watchdog announce <name>` (no med, already announced) | -> | Dedup preserved, no UpdateRoute | `test/plugin/watchdog-med-override.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `watchdog announce X med 500` when route has MED=100 in config | One-off command sent with `med 500`, not config MED. Route pool's stored Route/AnnounceCmd unchanged. |
| AC-2 | `watchdog announce X med 100` then `watchdog announce X med 1000` | Both produce sendRoute calls (dedup bypassed for MED path) |
| AC-3 | `watchdog announce X` (no med) when already announced for peer | No sendRoute call (existing dedup preserved) |
| AC-4 | `watchdog announce X med 500` when route was previously withdrawn | sendRoute called with overridden MED. announced[peer] set to true. |
| AC-5 | `watchdog announce X med 500` with wildcard peer `*` | All peers with pool X receive the overridden command |
| AC-6 | `watchdog announce med` (pool name is literal "med") | Error returned: "med" rejected as group name |
| AC-7 | `watchdog announce X med abc` (non-numeric MED) | Error returned: invalid MED value |
| AC-8 | `watchdog announce X med` (missing MED value) | Error returned: missing MED value |
| AC-9 | `watchdog announce X med 500 10.0.0.1` (MED + explicit peer) | Command dispatched to peer 10.0.0.1 only with overridden MED |
| AC-10 | `watchdog announce X` after `watchdog announce X med 500` (same peer) | Dedup skips (already announced). No sendRoute call. |
| AC-11 | `watchdog withdraw X` after `watchdog announce X med 500` | Normal withdrawal via WithdrawCmd. announced[peer] set to false. |
| AC-12 | Peer reconnect after `watchdog announce X med 500` | handleStateUp resends using stored AnnounceCmd (config MED), not the overridden MED. The override is transient. |
| AC-13 | PoolEntry created from config with MED=100 in attributes | entry.Route.MED == &100, entry.AnnounceCmd contains "med 100" |
| AC-14 | PoolEntry created from config without MED in attributes | entry.Route.MED == nil, entry.Route stored regardless |
| AC-15 | `watchdog announce X med 4294967295` (max uint32) | Accepted, command contains "med 4294967295" |
| AC-16 | `watchdog announce X med 4294967296` (overflow uint32) | Error returned: invalid MED value |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMEDOverrideProducesOneOffCommand` | `server_test.go` | AC-1: MED override clones route, formats with new MED, sends via sendRoute |  |
| `TestMEDOverrideBypassesDedup` | `server_test.go` | AC-2: Two consecutive MED overrides both produce sendRoute calls |  |
| `TestNoMEDPreservesDedup` | `server_test.go` | AC-3: Non-MED announce skips when already announced |  |
| `TestMEDOverrideFromWithdrawn` | `server_test.go` | AC-4: MED override on withdrawn route sends and sets announced |  |
| `TestMEDOverrideWildcard` | `server_test.go` | AC-5: Wildcard dispatches MED override to all peers |  |
| `TestMEDGroupNameRejected` | `server_test.go` | AC-6: "med" as pool name returns error |  |
| `TestMEDInvalidValue` | `server_test.go` | AC-7, AC-8: Non-numeric or missing MED returns error |  |
| `TestMEDOverrideWithExplicitPeer` | `server_test.go` | AC-9: MED + peer arg dispatches to single peer |  |
| `TestNoMEDAfterMEDOverride` | `server_test.go` | AC-10: Non-MED announce deduped after MED override |  |
| `TestWithdrawAfterMEDOverride` | `server_test.go` | AC-11: Withdraw uses stored WithdrawCmd after MED override |  |
| `TestReconnectUsesStoredNotOverride` | `server_test.go` | AC-12: handleStateUp sends AnnounceCmd, not overridden MED |  |
| `TestPoolEntryStoresRoute` | `config_test.go` | AC-13, AC-14: Route stored in PoolEntry from config parsing |  |
| `TestMEDOverrideBoundary` | `server_test.go` | AC-15, AC-16: Max uint32 accepted, overflow rejected |  |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MED value | 0 - 4294967295 | 4294967295 | N/A (0 is valid) | 4294967296 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `watchdog-med-override` | `test/plugin/watchdog-med-override.ci` | External process sends `watchdog announce dnsr med 500`, peer receives UPDATE with MED=500. Then sends `med 1000`, peer receives second UPDATE with MED=1000. Then sends `watchdog announce dnsr` (no med, already announced), no new UPDATE. |  |

### Future
- None -- all tests implemented in this phase.

## Files to Modify
- `internal/component/bgp/plugins/watchdog/server.go` - Extend handlePoolAction to parse optional `med <N>`. Extend handlePoolActionSingle to accept MED override, clone Route, bypass dedup when set.
- `internal/component/bgp/plugins/watchdog/pool.go` - Add Route field to PoolEntry. Add method or function to produce MED-overridden announce command.
- `internal/component/bgp/plugins/watchdog/config.go` - Store Route in PoolEntry during parseNLRIEntries.
- `internal/component/bgp/plugins/watchdog/server_test.go` - Unit tests for MED override, dedup bypass, dedup preservation.
- `internal/component/bgp/plugins/watchdog/config_test.go` - Test Route stored in PoolEntry.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| CLI commands/flags | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | Yes | `test/plugin/watchdog-med-override.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` - add `watchdog announce <name> [med <N>] [peer]` syntax |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |

## Files to Create
- `test/plugin/watchdog-med-override.ci` - Functional test for MED override end-to-end

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: PoolEntry Route storage** -- Add Route field to PoolEntry. Update parseNLRIEntries to store Route in PoolEntry alongside pre-computed commands.
   - Tests: `TestPoolEntryStoresRoute`
   - Files: `pool.go`, `config.go`, `config_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: MED override command parsing** -- Extend handlePoolAction to parse optional `med <N>` from args. Reject "med" as pool name. Pass optional MED to handlePoolActionSingle/handlePoolActionAll.
   - Tests: `TestMEDGroupNameRejected`, `TestMEDInvalidValue`, `TestMEDOverrideBoundary`
   - Files: `server.go`, `server_test.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: MED override dispatch** -- In handlePoolActionSingle, when MED override is set: clone entry.Route, set MED, call FormatAnnounceCommand, bypass announced dedup, send one-off command.
   - Tests: `TestMEDOverrideProducesOneOffCommand`, `TestMEDOverrideBypassesDedup`, `TestNoMEDPreservesDedup`, `TestMEDOverrideFromWithdrawn`, `TestMEDOverrideWildcard`, `TestMEDOverrideWithExplicitPeer`, `TestNoMEDAfterMEDOverride`, `TestWithdrawAfterMEDOverride`, `TestReconnectUsesStoredNotOverride`
   - Files: `server.go`, `server_test.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Functional test** -- Create watchdog-med-override.ci: external process sends MED override commands, verify peer receives correct BGP UPDATEs.
   - Tests: `test/plugin/watchdog-med-override.ci`
   - Files: `test/plugin/watchdog-med-override.ci`
   - Verify: functional test passes

5. **Full verification** -- `make ze-verify`
6. **Complete spec** -- Fill audit tables, write learned summary.

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-16 has implementation with file:line |
| Correctness | MED override produces correct "update text ... med N ..." command string. Dedup bypassed only for MED path. Dedup preserved for non-MED path. |
| Naming | `med` literal keyword in parser, not magic number |
| Data flow | Route clone -> MED set -> FormatAnnounceCommand -> sendRoute. No mutation of stored Route. |
| Backward compat | All existing watchdog tests pass without modification. `watchdog announce <name>` without med arg behaves identically. |
| Rule: no-layering | No old code path left alongside new one -- single handlePoolAction handles both MED and non-MED |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| PoolEntry has Route field | `grep 'Route.*bgp.Route' internal/component/bgp/plugins/watchdog/pool.go` |
| parseNLRIEntries stores Route | `grep 'Route.*=' internal/component/bgp/plugins/watchdog/config.go` |
| handlePoolAction parses `med <N>` | `grep 'med' internal/component/bgp/plugins/watchdog/server.go` |
| MED override bypasses dedup | `grep -A5 'bypass\|override\|FormatAnnounce' internal/component/bgp/plugins/watchdog/server.go` |
| "med" rejected as group name | Unit test `TestMEDGroupNameRejected` passes |
| Functional test exists | `ls test/plugin/watchdog-med-override.ci` |
| All existing watchdog tests pass | `go test ./internal/component/bgp/plugins/watchdog/...` |
| Functional test passes | `make ze-functional-test` includes watchdog-med-override |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | MED value parsed as uint32 via strconv.ParseUint with bitSize=32. Rejects negative, overflow, non-numeric. |
| Integer overflow | MED is uint32, parsed with explicit bitSize=32. No overflow possible. |
| Command injection | Pool name from args, MED from parsed uint32. FormatAnnounceCommand uses fmt.Fprintf with %d for MED. No string interpolation of user input into commands. |
| Resource exhaustion | MED override creates one temporary Route clone per entry per dispatch. Bounded by pool size (config-limited). No accumulation. |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read server.go/pool.go from Current Behavior |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-16 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-healthcheck-1-watchdog-med.md`
- [ ] Summary included in commit
