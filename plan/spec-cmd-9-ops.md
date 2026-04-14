# Spec: cmd-9 -- Operational Commands

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 3/3 |
| Updated | 2026-04-14 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-cmd-0-umbrella.md` -- umbrella context
3. `internal/component/bgp/plugins/rib/rib_pipeline.go` -- RIB pipeline pattern
4. `internal/component/bgp/plugins/rib/bestpath.go` -- best-path selection
5. `internal/component/resolve/cmd/` -- existing resolve command tree
6. `internal/component/cmd/show/` -- existing show command pattern

## Task

Four operational command additions:

### 1. Best-Path Reason (`rib best <prefix> reason`)

New pipeline terminal showing best-path decision steps per RFC 4271 Section 9.1.2. For each
pair of competing paths, shows which decision step was decisive and why one was preferred.

### 2. Ping and Traceroute (`resolve ping`, `resolve traceroute`)

| Command | Purpose |
|---------|---------|
| `resolve ping <target> [source <ip>] [count <n>] [size <bytes>]` | ICMP echo probe |
| `resolve traceroute <target> [source <ip>]` | Hop-by-hop path trace |

Under the existing `resolve` command tree alongside DNS/Cymru/PeeringDB/IRR resolution tools.

### 3. Interface Counters (`show interface`)

| Command | Purpose |
|---------|---------|
| `show interface <name> counters` | RX/TX packets, bytes, errors, drops for one interface |
| `show interface brief` | One-line-per-interface summary |

### 4. Show Uptime (`show uptime`)

Returns daemon start time and running duration.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- CLI command dispatch, show command pattern
  -> Constraint: operational commands are read-only, dispatched via CLI handler registry
- [ ] `.claude/patterns/cli-command.md` -- how to add CLI commands
  -> Constraint: command registration, YANG-modeled, handler function

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` -- BGP-4 base: best-path decision process (Section 9.1.2)
  -> Constraint: decision steps in order: weight, local-pref, locally originated, AS-path length, origin, MED, eBGP over iBGP, IGP metric, router-id

**Key insights:**
- Best-path decision process has a fixed step order per RFC 4271 Section 9.1.2
- Ping/traceroute are OS-level operations; need raw socket or suid helper
- Interface counters come from OS network stack (netlink on Linux)
- Show uptime is trivial: store start time at daemon init, compute duration

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline.go` -- RIB show pipeline (source/filter/terminal)
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` -- best-path selection logic
- [ ] `internal/component/resolve/cmd/` -- existing resolve command implementations
- [ ] `internal/component/cmd/show/` -- existing show command implementations
- [ ] `internal/component/iface/` -- interface management

**Behavior to preserve:**
- Existing `rib best` output format for basic best-path query
- Existing resolve command tree (DNS, Cymru, PeeringDB, IRR)
- Existing show commands unchanged
- All existing config files parse and work identically

**Behavior to change:**
- `rib best <prefix> reason` shows decision steps instead of just the winner
- New `resolve ping` and `resolve traceroute` commands under existing resolve tree
- New `show interface counters` and `show interface brief` commands
- New `show uptime` command

## Data Flow (MANDATORY)

### Entry Point
- CLI: `rib best 10.0.0.0/24 reason` typed in CLI session
- CLI: `resolve ping 10.0.0.1` typed in CLI session
- CLI: `show interface eth0 counters` typed in CLI session
- CLI: `show uptime` typed in CLI session

### Transformation Path

**Best-path reason:**
1. Command parse: CLI dispatcher routes to rib pipeline with reason terminal
2. Prefix lookup: RIB queried for all paths to prefix
3. Decision replay: best-path selection replayed step-by-step, recording winner/loser at each step
4. Output: formatted text showing each decision step, values compared, and which path won

**Ping/traceroute:**
1. Command parse: CLI dispatcher routes to resolve handler
2. Argument extraction: target, source, count, size parsed
3. OS execution: ICMP echo request (ping) or TTL-incrementing probe (traceroute) executed
4. Output: RTT/status per probe (ping) or hop list (traceroute)

**Interface counters:**
1. Command parse: CLI dispatcher routes to show interface handler
2. OS query: netlink (Linux) queried for interface statistics
3. Output: formatted counters (brief: one line per interface; counters: detailed for one)

**Show uptime:**
1. Command parse: CLI dispatcher routes to show uptime handler
2. Computation: current time minus stored daemon start time
3. Output: start time and duration formatted

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> RIB Plugin | rib best reason dispatched to RIB pipeline | [ ] |
| CLI -> OS | ping/traceroute exec OS-level ICMP/UDP probes | [ ] |
| CLI -> OS | interface counters read from OS network stack | [ ] |

### Integration Points
- `rib_pipeline.go` -- add reason terminal to rib best pipeline
- `bestpath.go` -- expose decision step details for reason output
- `resolve/cmd/` -- add ping and traceroute handlers
- `cmd/show/` -- add interface counters, interface brief, and uptime handlers

### Architectural Verification
- [ ] No bypassed layers (CLI -> dispatcher -> handler -> data source)
- [ ] No unintended coupling (each command is independent)
- [ ] No duplicated functionality (extends existing command trees)
- [ ] Zero-copy not applicable (read-only queries, text output)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| CLI `rib best 10.0.0.0/24 reason` | → | RIB pipeline replays decision steps | `test/plugin/bestpath-reason.ci` |
| CLI `resolve ping 127.0.0.1` | → | ICMP probe returns RTT | `test/plugin/resolve-ping.ci` |
| CLI `show interface brief` | → | Interface list returned | `test/plugin/bestpath-reason.ci` |
| CLI `show uptime` | → | Daemon uptime returned | `test/plugin/bestpath-reason.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `rib best 10.0.0.0/24 reason` | Decision steps shown with winner/loser and which RFC step decided |
| AC-2 | `rib best` reason with two paths differing on local-pref | Local-pref step identified as deciding factor |
| AC-3 | `rib best` reason with two paths differing on AS-path length | AS-path length step identified as deciding factor |
| AC-4 | `resolve ping 127.0.0.1` | Returns RTT and status (success) |
| AC-5 | `resolve ping` with count and size | Specified number of probes with specified size sent |
| AC-6 | `resolve ping` with source | Probes sent from specified source address |
| AC-7 | `resolve traceroute 127.0.0.1` | Returns hop list with RTT per hop |
| AC-8 | `show interface eth0 counters` | RX/TX packets, bytes, errors, drops shown |
| AC-9 | `show interface brief` | One-line-per-interface with name, status, addresses |
| AC-10 | `show uptime` | Daemon start time and running duration shown |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBestPathReasonLocalPref` | `bestpath_reason_test.go` | Local-pref decision step reported | |
| `TestBestPathReasonASPathLen` | `bestpath_reason_test.go` | AS-path length decision step reported | |
| `TestBestPathReasonOrigin` | `bestpath_reason_test.go` | Origin decision step reported | |
| `TestBestPathReasonMED` | `bestpath_reason_test.go` | MED decision step reported | |
| `TestPingLoopback` | `resolve_ping_test.go` | Ping localhost returns success | |
| `TestPingWithCount` | `resolve_ping_test.go` | Count parameter respected | |
| `TestTracerouteLoopback` | `resolve_traceroute_test.go` | Traceroute localhost returns single hop | |
| `TestInterfaceCounters` | `show_interface_test.go` | Counters returned for named interface | |
| `TestInterfaceBrief` | `show_interface_test.go` | Brief shows all interfaces | |
| `TestShowUptime` | `show_uptime_test.go` | Uptime returns start time and duration | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ping count | 1-65535 | 65535 | 0 | 65536 |
| ping size | 1-65535 | 65535 | 0 | 65536 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bestpath-reason` | `test/plugin/bestpath-reason.ci` | rib best reason shows decision steps for a prefix | |
| `resolve-ping` | `test/plugin/resolve-ping.ci` | resolve ping localhost returns success | |

## Files to Modify

- `internal/component/bgp/plugins/rib/rib_pipeline.go` -- add reason terminal to rib best
- `internal/component/bgp/plugins/rib/bestpath.go` -- expose decision step details
- `internal/component/resolve/cmd/` -- add ping and traceroute handlers
- `internal/component/cmd/show/` -- add interface counters, interface brief, and uptime handlers

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| CLI command registration (reason) | [x] | `internal/component/bgp/plugins/rib/rib_pipeline.go` |
| CLI command registration (ping/traceroute) | [x] | `internal/component/resolve/cmd/` |
| CLI command registration (interface/uptime) | [x] | `internal/component/cmd/show/` |
| Functional test | [x] | `test/plugin/bestpath-reason.ci`, `test/plugin/resolve-ping.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- add best-path reason, ping/traceroute, counters, uptime |
| 2 | Config syntax changed? | [ ] | N/A (operational commands) |
| 3 | CLI command added/changed? | [x] | `docs/guide/commands.md` -- new operational commands |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [ ] | N/A |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc4271.md` -- Section 9.1.2 decision process |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- operational commands parity |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

- `test/plugin/bestpath-reason.ci` -- best-path reason functional test
- `test/plugin/resolve-ping.ci` -- ping functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Modify, TDD Plan |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-verify` |
| 5-12. | Standard flow |

### Implementation Phases

1. **Phase: Best-Path Reason** -- Add reason terminal to rib best pipeline, expose decision steps
   - Tests: `TestBestPathReason*`
   - Files: rib_pipeline.go, bestpath.go
2. **Phase: Ping/Traceroute** -- Add resolve ping and traceroute under existing resolve tree
   - Tests: `TestPing*`, `TestTraceroute*`
   - Files: resolve/cmd/
3. **Phase: Interface Counters** -- Add show interface counters and brief
   - Tests: `TestInterfaceCounters`, `TestInterfaceBrief`
   - Files: cmd/show/
4. **Phase: Show Uptime** -- Add show uptime command
   - Tests: `TestShowUptime`
   - Files: cmd/show/
5. **Functional tests** -- .ci tests proving end-to-end behavior
6. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 10 ACs demonstrated |
| RFC compliance | Best-path reason steps match RFC 4271 Section 9.1.2 order |
| OS interaction | Ping/traceroute handle permission errors gracefully |
| Interface portability | Interface counters work on Linux (netlink) |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Best-path reason terminal | `grep reason internal/component/bgp/plugins/rib/rib_pipeline.go` |
| Ping handler | `grep -r ping internal/component/resolve/cmd/` |
| Interface counters | `grep -r counters internal/component/cmd/show/` |
| .ci functional tests | `ls test/plugin/bestpath-reason.ci test/plugin/resolve-ping.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Privilege | Ping/traceroute may need raw socket or setuid; validate source address is local |
| Resource | Ping count/size bounded; traceroute max hops bounded |
| Input validation | Target address validated; interface name validated against OS |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Ping permission error | Document required capabilities or suid |
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

## RFC Documentation

Add `// RFC 4271 Section 9.1.2: "<decision step>"` above each best-path reason step.

## Implementation Summary

### What Was Implemented

**show uptime -- DONE:**
- `handleShowUptime` handler registered as `ze-show:uptime`
- Returns `start-time` (RFC 3339) and `uptime` (truncated to seconds)
- Nil-safety for both nil CommandContext and nil Reactor
- YANG node in `ze-cli-show-cmd.yang`
- Unit test: `TestHandleShowUptime_NilReactor`

**show interface brief -- DONE:**
- `showInterfaceBrief()` returns compact one-line-per-interface summary (name, state, MTU, first address)
- YANG child node `show/interface/brief` for tab-completion
- Unit test: `TestHandleShowInterfaceBrief`

**show interface counters -- DONE:**
- `handleShowInterface` with `<name> counters` arg returns stats-only JSON
- Existing `InterfaceStats` struct already has RX/TX packets/bytes/errors/drops

**resolve ping -- DONE:**
- `handlePing` runs system `ping` with source/count/size options
- 15s context timeout, registered as `ze-resolve:ping`
- YANG node in `ze-resolve-cmd.yang`

**resolve traceroute -- DONE:**
- `handleTraceroute` runs system `traceroute` with source option
- 30s context timeout, registered as `ze-resolve:traceroute`
- YANG node in `ze-resolve-cmd.yang`

### What Remains

~~`rib best <prefix> reason` terminal~~ -- DONE. Found already implemented during 2026-04-14 audit.

| Item | Effort | Design needed |
|------|--------|---------------|
| (none -- all features implemented) | - | - |

**rib best reason implementation (found 2026-04-14):**
- `BestStep` enum with `String()` in `bestpath.go` (10 named steps, RFC 4271 order)
- `BestPathExplanation` + `PairwiseStep` structs for narration
- `SelectBestExplain()`: slow-path variant recording per-step decisions
- `comparePairWithReason()`: core comparison with reason + step output
- `bestReasonTerminal` in `rib_pipeline_best.go`: drains upstream, re-runs explanation per prefix
- `parseBestPipelineArgs()` accepts "reason" keyword
- JSON shape: `{"best-path-reason": [{"family","prefix","winner-peer","candidates","steps":[{"step","incumbent","challenger","winner","reason"}]}]}`
- 7 unit tests (`TestSelectBestExplain_*`) + 4 pipeline tests (`TestBestPipelineReason_*`)
- Functional test: `test/plugin/bestpath-reason.ci` (added 2026-04-14)

### Bugs Found/Fixed
- show uptime panicked on nil CommandContext (fixed with nil guard)

### Documentation Updates
- `docs/guide/command-reference.md` not yet updated

### Deviations from Plan
- ~~`rib best reason` deferred pending design of instrumentation approach~~ -- found already implemented (2026-04-14 audit)

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
- **Partial:**
- **Skipped:**
- **Changed:**

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added for best-path reason
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-cmd-9-ops.md`
- [ ] Summary included in commit
