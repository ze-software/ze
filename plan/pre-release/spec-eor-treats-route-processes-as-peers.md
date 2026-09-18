# Spec: eor-treats-route-processes-as-peers

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | bgp |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**A process that creates routes is a PEER as far as the End-of-RIB marker is
concerned, and ze does not wait for a peer's marker before sending its own
(owner ruling, 2026-09-18).**

Ze sends its initial table to a new session and then the marker RFC 4724
Section 2 defines: everything before it was the initial routing update. Today
`sendInitialRoutes` (`internal/component/bgp/reactor/peer_initial_sync.go`)
drains its own queue, then WAITS for every process entitled to push routes into
that update, and writes the marker only when each has reported ready or the sync
timeout expires. That wait exists because of an earlier ruling, cited in the
same function, that a plugin-injected route belongs to the initial update
(owner, 2026-08-30).

This ruling supersedes that one for the marker's TIMING. A route-creating
process stands where a peer stands: ze never holds its own marker waiting for a
peer's, so it does not hold it waiting for a process either. The routes such a
process pushes are ordinary updates when they arrive, exactly as a peer's are.

**Why it matters.** The wait is a window, not a barrier. The queue gate closes
and the forwarding rail reopens at the START of it (`wakeForwardOverflow`),
while the marker is still owed, so a route forwarded from another peer can reach
the wire before the marker that claims to end the initial table. The peer is
then told that a live forwarded route was part of ze's initial table. Removing
the wait closes that window by making the marker follow ze's own table
immediately, which is what "treat the process like a peer" means on the wire.

**What it also settles.** Four functional tests assert routes-before-marker and
lose that race under load: `redistribute-export-reject`,
`ipv4-announce-withdraw`, `ipv6-announce-withdraw` and
`show-bgp-bare-runs-summary` (`plan/journal/false-synchronization-claim.md`,
2026-09-18). They are the symptom this ruling removes at the source; the spec
must decide what each of them asserts afterwards, because an ordering the
product no longer promises must not stay in a test.

**What this spec must NOT assume.** That deleting the wait is the whole change.
`initialSyncEOROwed` gates EOR suppression on the API rail
(`reactor_api_forward.go`), `apiSyncExpected` and `waitForAPISync` exist to serve
the 2026-08-30 ruling, and `forwardOrderHold` narrows `shouldQueue` twice for
reasons its own comment calls load-bearing. Each has to be read before anything
is removed, and the 2026-08-30 ruling has to be re-read in full: it may still
govern WHICH routes belong to the initial update even where it no longer governs
WHEN the marker goes out.

## Required Reading

### Architecture Docs
- [ ] The initial-sync and forwarding rails, page named by `ai/CODE-TO-DOCS.md` for `peer_initial_sync.go`
  → Decision: [fill during research]
  → Constraint: [fill during research]
- [ ] `rfc/full/rfc4724.txt` Section 2 - what the marker claims about everything before it
  → Constraint: [fill during research]

**Key insights:** (minimal context to resume after compaction)
- The ruling is about WHEN the marker goes out, not about which routes belong to the initial update.
- A route-creating process is a peer for this purpose; ze waits for no peer's marker before sending its own.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go` - `sendInitialRoutes` drains the queue, calls `wakeForwardOverflow`, waits for the route-pushing processes, then writes the marker and clears `initialSyncEOROwed`
- [ ] `internal/component/bgp/reactor/peer.go` - `forwardOrderHold` is `Established && sendingInitialRoutes != 0` and does not read `initialSyncEOROwed`
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - the EOR suppression path DOES read `initialSyncEOROwed`, beside `shouldQueue`

**Behavior to preserve:** (unless the user explicitly said to change it)
- One End-of-RIB per family per session (RFC 4724 Section 2), and the claim protocol that enforces it
- A forwarded UPDATE never overtakes route operations still queued for that peer

**Behavior to change:** (only what the user asked for)
- Ze stops holding its own End-of-RIB for the processes that push routes into the initial update

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `Peer.sendInitialRoutes` (`internal/component/bgp/reactor/peer_initial_sync.go`): every session that reaches Established.

### Transformation Path
1. [fill during design] The queue drains, the marker follows it, and the processes' routes arrive as ordinary updates afterwards.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor ↔ plugin rail | the API sync expectation set, if it survives at all | No |

### Integration Points
- The four tests named in the Task, and whatever each asserts once the ordering is the product's

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | [fill during design] | [fill during design] |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A process's routes arriving AFTER the marker is correct, because a peer's do | owner ruling, 2026-09-18 | the 2026-08-30 ruling still governs and only the window closes, by widening `forwardOrderHold` instead | owner confirmation at the gate | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A consumer depends on plugin routes preceding the marker | the four tests, and any `.ci` asserting it | name every one during research and put the list to the owner |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A graceful-restart helper folds fewer routes into its deferred batch, and a plugin's routes land after the marker instead of inside it |
| How is it reverted? | One commit |
| Who else touches this path? | the two forwarding rails, the route-server replay, the API `eor` command |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a session established with a route-pushing plugin configured | → | `sendInitialRoutes` writes the marker without waiting | [fill during design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a session establishes while a process that pushes routes is configured | ze's End-of-RIB follows its own table with no wait for that process |
| AC-2 | a route forwarded from another peer arrives during session start | it does not reach the wire before the marker |
| AC-3 | the four tests named in the Task | each asserts what the product now promises, and none asserts an ordering it does not |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | brings up a session on a router carrying a route-pushing plugin | initial table, then the marker, then the plugin's routes | [fill during design] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [fill during design] | `internal/component/bgp/reactor/` | AC-1 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| the sync timeout, if it survives at all | [fill during design] | [fill] | [fill] | [fill] |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [fill during design] | `test/plugin/*.ci` | AC-2 | |

## Files to Modify
- `internal/component/bgp/reactor/peer_initial_sync.go` - the wait, and the marker's timing. Design doc: [fill during research]

## Files to Create
- [fill during design]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no leaf |
| CLI commands/flags | N-A | none |
| Functional test for new RPC/API | Yes | the ordering test AC-2 names |
| Prometheus counters/metrics | No | `eorSent` keeps its contract, markers that reached the socket |
| BGP family surface (new SAFI / capability / attribute) | N-A | no family change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | none |
| 7 | Wire format changed? | No | the marker is unchanged; its timing is not the format |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | RFC 4724 Section 2, and the `rfc/short/` summary carrying the marker's requirements |
| 12 | Internal architecture changed? | Yes | the initial-sync page, once research names it |

## Implementation Steps

1. **Phase: Research** -- read the three producers, re-read the 2026-08-30 ruling in full, and name every consumer that depends on plugin routes preceding the marker
   - Verify: the list of dependents, and the owner's answer on A-1

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | the marker still goes out once per family per session, and no forwarded UPDATE precedes it |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The marker no longer waits for a route-creating process | AC-1's test |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | none: this adds no external input |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] No code snippets
- [ ] Current Behavior and Data Flow sections completed
- [ ] Every assumption has a Basis and a validation method

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
