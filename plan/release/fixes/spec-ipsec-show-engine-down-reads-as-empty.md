# Spec: ipsec-show-engine-down-reads-as-empty

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P1 |
| Release reason | An operator command must not answer "no SAs" when the IKE engine that holds the SAs is not running. |
| Scope | cli |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the operator is promised.** `show vpn ipsec sa` lists the IKE SAs Ze
holds, and `clear vpn ipsec sa all` and `clear vpn ipsec sa peer <name>` tear
them down. An empty list means "Ze holds no SA". `terminated: 0` means "there
was nothing to clear". "peer not found" means "no such peer is configured or
active".

**What Ze does instead.** Each of the three commands gives that answer also
when no IKE engine is running, so "the engine is down" reads the same as "the
engine is up and holds nothing". The sibling `show vpn ipsec peer name <n>`
in the same file already refuses with "ipsec engine not running", and the
`show vpn ipsec dataplane` commands already refuse a missing backend through
`dataplaneReadError`, whose comment states the rule this spec applies: none of
these causes "may become an empty list". A second form of the same defect:
`runEngine` publishes the SA table and never clears it on exit, so after the
engine stops, `show vpn ipsec status` still reports `engine-running: true`, and
the three commands above answer from a table nobody maintains.

**Prior art.** VyOS T9542 (commit 2c510ff27) fixed the same class in VyOS:
its `show vpn ipsec sa` reported no SAs when the IKE daemon was not running,
and now says that the daemon is not running.

**What closing it means.** Every IPsec operator command that reads the engine
answers an error naming the stopped engine, why it matters and the command
that fixes it, whenever no engine is running, on every command surface. The
`engine-running` field of `show vpn ipsec status` is true only while an
engine runs.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `zero-value-as-valid-answer.md` | 2026-11-08 | `show vpn ipsec sa` and `clear vpn ipsec sa` read engine down as no SAs |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/access-security.md` - the IKE door
  → Constraint: readers of IKE state outside the component use the core leaves; the command package sits inside `ike` and reads the engine directly
- [ ] `docs/guide/ipsec.md` - the operator page for the IPsec commands
  → Decision: to be recorded at design

**Key insights:**
- `dataplaneReadError` already states the contract this spec applies to the engine: a missing producer is never an empty list.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/cmd/show_ipsec.go` - `handleShowVPNIPsecSA`: `engine.ActiveTable()` nil returns `StatusDone` with `peers: []`. `handleShowVPNIPsecPeer`: the same nil returns `StatusError` "ipsec engine not running", which names no fix. `handleShowVPNIPsecStatus`: `engine-running` is `ActiveTable() != nil`
- [ ] `internal/component/ike/cmd/ipsec.go` - `handleClearIPsecSAAll` returns `StatusDone` with `terminated: engine.TerminateAllSAs()`, which is 0 when `activePeersMap` is nil. `handleClearIPsecSAPeer` answers "peer not found" when `engine.TerminatePeerSA` returns false, which it also does for a nil `activePeersMap`
- [ ] `internal/component/ike/engine/register.go` - `ActiveTable`, `activeTablePtr`, `setActivePeers`: `runEngine` stores the table and the peer map once at start; no non-test code stores nil afterwards
- [ ] `internal/component/ike/engine/apply.go` - `ikeEngineState.shutdown` stops the worker and peers, closes listeners and releases the SPD; it does not clear the published table or peer map
- [ ] `internal/component/ike/cmd/show_dataplane.go` - `dataplaneReadError`, `activeDataplane`: a nil backend is answered as an error that says it is not the same answer as an empty dataplane
- [ ] `internal/component/ike/cmd/monitor_ipsec.go` - `streamIPsecMonitor` subscribes to engine events and streams nothing while no engine runs; whether it owes a first line that says so is a design question
- [ ] `internal/component/ike/cmd/show_ipsec_test.go` - `TestShowIPsecSA_NoEngine` asserts the empty-list answer for a nil table: the test encodes the defect and must change with it

**Behavior to preserve:**
- The `peers` envelope key and row shape of `show vpn ipsec sa` while an engine runs
- The expectations of `test/ipsec/ipsec-sa-show.ci`, `test/ipsec/ipsec-status-show.ci`, `test/ipsec/ipsec-clear-sa.ci` and `test/ipsec/ipsec-peer-show.ci`

**Behavior to change:**
- The no-engine answer of the five handlers above, and the published state after `runEngine` exits

## Existing Mechanisms

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| No-backend refusal | `dataplaneReadError`, `activeDataplane` (`ike/cmd/show_dataplane.go`) | Refuses a missing dataplane backend as an error, never an empty list | Extends: the same shape for a missing engine, one helper shared by the five handlers |
| Engine-not-running refusal | `handleShowVPNIPsecPeer` (`ike/cmd/show_ipsec.go`) | Refuses with "ipsec engine not running" | Extends: becomes the shared refusal, and gains the why and the fix command (`ai/rules/user-facing-errors.md`) |
| Published engine state | `activeTablePtr`, `setActivePeers` (`ike/engine/register.go`) | Set once by `runEngine`, never cleared | Extends: cleared on every exit of `runEngine`, so nil means "no engine" |
| Core-leaf no-engine answer | `ipsecinventory.Tunnels`, `ikeprobe.Probe` (`internal/core/ipsecinventory`, `internal/core/ikeprobe`) | Answer `ErrNotRegistered`, never an empty list, when no engine registered | Does not apply: the command package imports the engine directly; named as the precedent for the answer shape |
| `engine-running` field | `handleShowVPNIPsecStatus` | Derived from the published table | Extends: correct once the table is cleared on exit; no new field |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters: an operator command on any surface, dispatched by the daemon]
- [Format at entry]

### Transformation Path
1. The dispatcher routes the `ze-ike:` RPC to the registered handler
2. The handler reads the published engine state
3. The handler answers a response, or a refusal when no engine is published

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Command surface to daemon | The shared command registry | No |

### Integration Points
- `engine.ActiveTable`, `engine.ActivePeers`, `engine.TerminateAllSAs`, `engine.TerminatePeerSA` - the reads every handler shares

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A daemon built with IKE can run with no IKE engine started, so AC-1 is reachable through the daemon | `runEngine` publishes state only when it runs | AC-1 to AC-4 are reachable only after a stop (AC-5) | Read `runEngine` start conditions at design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Clearing the published table races a handler that loaded it | Race detector on the package test | Handlers already hold a loaded pointer; clear after peers stop |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show vpn ipsec sa` with no engine | → | `handleShowVPNIPsecSA` refusal | `.ci` under `test/ipsec/`, named at design |

## Acceptance Criteria

Each test runs the command through the daemon's dispatcher, the path every
command surface (CLI, SSH, web, REST, gRPC, MCP) shares.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Daemon running, no IKE engine running; operator runs `show vpn ipsec sa` | Error response that names the stopped IKE engine, says the list cannot be read, and gives the command that fixes or diagnoses it. No `peers` list |
| AC-2 | Same condition; `clear vpn ipsec sa all` | Error response naming the stopped engine; no `terminated: 0` |
| AC-3 | Same condition; `clear vpn ipsec sa peer <name>` | Error response naming the stopped engine, not "peer not found" |
| AC-4 | Same condition; `show vpn ipsec peer name <n>` | The same refusal text as AC-1, with the fix command |
| AC-5 | Engine ran, then stopped while the daemon stays up | `show vpn ipsec status` answers `engine-running: false`, and AC-1 to AC-4 hold |
| AC-6 | Engine running with no SA | `show vpn ipsec sa` still answers `peers: []` with `StatusDone` |

`TestShowIPsecSA_NoEngine` asserts the defect, so changing it is a test change
judged by an independent `ze-test-judge` agent (`ai/META-DECISIONS.md`, "Test
changes"); the old expectation is shown wrong against `dataplaneReadError`'s
stated contract and `ai/rules/principles.md`.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| replaces `TestShowIPsecSA_NoEngine` | `internal/component/ike/cmd/show_ipsec_test.go` | AC-1 refusal for a nil table (judged change) | |
| new, named at design | `internal/component/ike/cmd/ipsec_test.go` | AC-2, AC-3 refusals | |
| new, named at design | `internal/component/ike/engine/` | AC-5: published state is nil after `runEngine` exits | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| named at design | `test/ipsec/*.ci` | AC-1 to AC-4 through the daemon with no engine | |
| named at design | `test/ipsec/*.ci` | AC-5: engine stopped through the real path, then the commands | |
| `ipsec-sa-show` | `test/ipsec/ipsec-sa-show.ci` | AC-6 stays green | |

## Open design questions (answered at design, not by Thomas)

| # | Question | Where the answer comes from |
|---|----------|-----------------------------|
| 1 | How a functional test reaches "daemon up, engine not running" in a build that links IKE | `runEngine` start conditions in `ike/engine/register.go` and the IKE plugin's config roots |
| 2 | The exact fix command the refusal names | The IKE door, `docs/guide/ipsec.md`, and whether a `ze doctor` check for the engine exists |
| 3 | Whether `monitor vpn ipsec` owes a refusal or a first line when no engine runs | `streamIPsecMonitor` and the streaming handler contract |

## Files to Modify

- `internal/component/ike/cmd/show_ipsec.go` - shared no-engine refusal in `handleShowVPNIPsecSA` and `handleShowVPNIPsecPeer`
- `internal/component/ike/cmd/ipsec.go` - the same refusal in both clear handlers
- `internal/component/ike/engine/register.go`, `internal/component/ike/engine/apply.go` - clear the published table and peer map on every exit of `runEngine`
- `internal/component/ike/cmd/show_ipsec_test.go` - replace `TestShowIPsecSA_NoEngine` (judged change)
- `docs/guide/ipsec.md` - the operator answer when the engine is not running

## Files to Create

- `test/ipsec/<name>.ci` - AC-1 to AC-5 through the daemon

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| CLI commands/flags | No | Existing commands; only their no-engine answer changes |
| Functional test for new RPC/API | Yes | `test/ipsec/*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | Yes | `docs/guide/ipsec.md`, the no-engine answer |

## Implementation Steps

1. **Phase: Wiring** -- failing `.ci` for AC-1 through the daemon
2. **Phase: refusal** -- one shared no-engine refusal in the five handlers (AC-1 to AC-4)
3. **Phase: published state** -- clear on every exit of `runEngine` (AC-5)

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs: `./le test functional ipsec` for the new `.ci`, and the `ike/cmd` and `ike/engine` package tests under `./le job run`
