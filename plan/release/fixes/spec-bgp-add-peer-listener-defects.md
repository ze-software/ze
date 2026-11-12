# Spec: bgp-add-peer-listener-defects

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P1 |
| Release reason | Shipped-daemon defects on the path that adds a BGP peer or a dynamic group and opens its listener: a passive dynamic group can be left with no listener, a refused add leaves state behind for a peer that does not exist, and the operator's listen error names no fix. |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-single-owner-lifecycle-2-bgp-session.md` (it adds `Reactor.listenersStarted` and changes the listener step of `addPeerLocked`, the code this spec reads; the order of the two is settled in design) |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Three defects on one path, met on 2026-11-12 while
`spec-single-owner-lifecycle-2-bgp-session` was implemented. That spec fixes a
configured passive peer added during the standalone start, and deliberately
leaves these three out. Each has a journal row (Owned journal rows). Nothing
here is designed yet: this skeleton states each defect, its producer, and why
it is a release fix.

| Part | Producer | What the operator meets today | Why it is a release fix |
|------|----------|-------------------------------|-------------------------|
| A dynamic group installed during the standalone start gets no listener | `installDynamicGroupsLocked` (`internal/component/bgp/reactor/reactor_dynamic.go`) | A passive dynamic group installed after the start's listener pass and before `running` is set has no socket, and nothing else opens one for a group, so no member can connect | A passive listener the config declares is missing |
| A refused add leaves state behind | `addPeerLocked` (`internal/component/bgp/reactor/reactor_peers.go`) | When the listener cannot start, the add is refused, but the peer's pool demand, its per-peer pool, its metrics and its stale-prefix alarm stay for a peer that does not exist | A refused change must leave the running state as it was |
| The listen error names no fix | `startListenerForAddressPort` (`internal/component/bgp/reactor/reactor.go`) | The error is `listen on <addr>: <err>`, a plain wrapped error with no fix command and no doctor code, returned through `AddPeer` to the reload, the API operation and the config loader | Every message an operator sees names the fix (`ai/rules/user-facing-errors.md`) |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the constraints every change keeps
  → Constraint: every message a user sees names the problem, why, and the exact fix, on every interface.
- [ ] `ai/doors/bgp-engine.md` - the reactor
  → Constraint: peers start only after every plugin phase finished. A listener opened earlier must not let a session reach Established before the plugins its policy names are connected.

### RFC Summaries (Scope: protocol)
- [ ] None read for this skeleton. The design names any RFC 4271 obligation on the passive side it relies on, and quotes it.

**Key insights:**
- A configured peer exists before its connection, so an early accept can wait in the peer's inbound slot until the barrier. A dynamic peer is built on accept, so that answer does not transfer to a group: the design decides what an accept before the barrier does for a group.
- The rollback in `addPeerLocked` runs after six writes it does not undo.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/reactor_dynamic.go` - `installDynamicGroupsLocked` opens a listener for each passive group only while `running` is set and `stopping` is not. Its comment says that before `running` the start's `startMultiListeners` opens every listener, and that nothing else opens a socket for a group.
- [ ] `internal/component/bgp/reactor/reactor_peers.go` - `addPeerLocked` stores the peer, advances `peerGeneration`, adds the peer to `fwdWeights`, registers its outgoing pool, sets the `peersConfigured` gauge and the `peersAddedTotal` counter, calls `setPrefixConfigMetrics` and `raisePrefixStale`, and only then opens the passive peer's listener. When that fails it deletes the peer from `r.peers` and returns the error.
- [ ] `internal/component/bgp/reactor/reactor.go` - `startListenerForAddressPort` returns a wrapped `fmt.Errorf` for a stopping reactor and for a listener that cannot start.
- [ ] `internal/component/bgp/config/loader_create.go` - the loader's peer loop wraps an `AddPeer` error as `add peer <address>: <err>` and returns it, then installs the dynamic groups through `SetDynamicGroups`.

Not read, and marked so wherever this spec relies on it: the other two callers
of `AddPeer` (the reload journal and the API operation) and how each one
renders the error, and whether any path installs a dynamic group inside the
standalone start window.

**Behavior to preserve:**
- An active-only peer gets no listener.
- A stop that has begun opens no listener.
- No session reaches Established before the plugin startup barrier lifts.

**Behavior to change:**
- A passive dynamic group installed at any point of the start has a listener.
- A refused add leaves no state the add wrote.
- The listen error an operator gets names the cause and the fix, with a doctor code.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| [mechanism] | [file + symbol, or page + section] | [what it does today] | [extends / replaces / does not apply: why] |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters: wire bytes, API command, config, plugin message]

### Transformation Path
1. [Stage 1]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| [boundary] | [how] | No |

### Integration Points
- [Existing function/type this connects to] - [how it integrates]

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A dynamic group can be installed inside the standalone start window | The gate in `installDynamicGroupsLocked`, read 2026-11-12. No path into the window was read | Part 1 is not reachable and its row closes with the evidence | Read every caller of `installDynamicGroupsLocked` against the start order | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | [what goes wrong] | [how we notice it] | [what we do about it] |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | [answered in design] |
| How is it reverted? | [answered in design] |
| Who else touches this path? | `plan/release/fixes/spec-single-owner-lifecycle-2-bgp-session.md` edits `addPeerLocked` and the start's listener pass |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [config/CLI/event that triggers it] | → | [function that actually runs] | [test name proving the chain] |

## Acceptance Criteria

Draft rows. The design pass confirms or rewrites each one.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A passive dynamic group is installed after the start's listener pass and before the reactor runs | A member's connection to the group's address is accepted once the barrier lifts, and no session reaches Established before it | None |
| AC-2 | `AddPeer` of a passive peer whose listener cannot start | The add is refused, and the peer's pool demand, per-peer pool, gauges, counter, prefix metrics and stale-prefix alarm are as they were before the call | None |
| AC-3 | `AddPeer` of a passive peer whose listener cannot start, through a reload | The operator's error names the cause and the command that fixes it, and carries a doctor code | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [named in design] | `internal/component/bgp/reactor/` | AC-1 to AC-3 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [named in design] | `test/.../*.ci` | [what the user expects to happen] | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| [named in design] | `test/interop/scenarios/` | [FRR/BIRD] | [protocol behavior validated] | |

## Files to Modify
- [named in design]

## Files to Create
- [named in design]

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- [named in design]

## Design Insights

- The add path writes six pieces of state before the one step that can fail, and the rollback undoes one of them.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/unwired-feature.md` | 2026-11-12 | `installDynamicGroupsLocked` opens no listener during the standalone start |
| `plan/journal/rollback-forgets-partial-apply.md` | 2026-11-12 | `addPeerLocked` rollback deletes only the peer entry |
| `plan/journal/error-names-no-fix-command.md` | 2026-11-12 | `startListenerForAddressPort` listen error with no fix and no doctor code |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- [What was deliberately not done and why]

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] No code snippets
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] The targeted tests for what changed pass, run in the main checkout: `./le test unit pkg path ./internal/component/bgp/reactor/` and the functional tests the design names. No agent runs any `./le verify` command

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
