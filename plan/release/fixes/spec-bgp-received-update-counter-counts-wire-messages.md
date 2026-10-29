# Spec: bgp-received-update-counter-counts-wire-messages

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P1 |
| Release reason | A peer's received UPDATE counter, shown by `show` and exported as a metric, reports fewer UPDATEs than the peer sent whenever the read path coalesces them, so an operator comparing it with the peer's sent counter sees a loss that did not happen |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-29 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Found by the RFC 8212 spec (`spec-bgp-rfc8212-ebgp-default-reject.md`). With
`ze.bgp.reactor.coalesce` on (the default), `Session.readAndProcessCoalesced`
(`internal/component/bgp/reactor/session_coalesce.go`) appends the NLRI of an
IPv4 unicast UPDATE whose path attributes equal the held one, and
`flushCoalesce` dispatches the batch as ONE synthetic UPDATE through
`processValidatedMessage`. `Reactor.notifyMessageReceiver`
(`reactor_notify.go`) then calls `incrUpdatesReceived` once per dispatched
message, so two UPDATEs on the wire count as one. The byte metric and the
capture tee already run per wire message, before coalescing.

To be designed: the received UPDATE counter counts each UPDATE as it arrives on
the wire, on both the standard and the coalescing read paths, while the EOR
counter and the dispatch stay as they are.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - IPv4 unicast UPDATE coalescing
  → Constraint: to be filled at design

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - UPDATE message handling
  → Constraint: to be filled at design

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/session_coalesce.go` - merges UPDATEs with identical attributes; read at design
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - `notifyMessageReceiver` increments `updatesReceived`; read at design
- [ ] `internal/component/bgp/reactor/peer_stats.go` - the per-peer counters; read at design

**Behavior to preserve:**
- Coalescing itself, the EOR counter, and what the RIB receives

**Behavior to change:**
- `updates-received` equals the number of UPDATE messages the peer sent

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An UPDATE read from the peer's TCP connection

### Transformation Path
1. to be filled at design

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session ↔ Reactor | `onMessageReceived` callback | No |

### Integration Points
- to be filled at design

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | to be filled at design |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | to be filled at design | - | - | - | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | to be filled at design | - | - |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | to be filled at design |
| How is it reverted? | to be filled at design |
| Who else touches this path? | `spec-bgp-rfc8212-ebgp-default-reject` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Two UPDATEs with identical attributes from a peer | → | to be filled at design | to be named at design |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A peer sends two consecutive IPv4 unicast UPDATEs with identical path attributes, coalescing on | The peer's `updates-received` counter rises by 2 |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Reads a peer's received UPDATE count after the peer sent N UPDATEs | to be filled at design | to be named at design |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| to be filled at design | - | AC-1 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| to be named at design | `test/` | AC-1 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| to be named at design | `test/interop/scenarios/` | FRR | to be filled at design | |

## Files to Modify
- to be filled at design

## Files to Create
- to be filled at design

## Implementation Steps

1. to be filled at design

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- to be filled at design

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Current Behavior and Data Flow sections completed

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify changed` passes on the closing commit

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-bgp-received-update-counter-counts-wire-messages.md` only

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `counter-counts-the-wrong-packets.md` | 2026-10-29 | Coalesced UPDATEs counted once in `updates-received` |
