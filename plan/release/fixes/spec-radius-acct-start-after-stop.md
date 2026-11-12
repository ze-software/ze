# Spec: radius-acct-start-after-stop

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P1 |
| Release reason | Shipped-daemon defect an operator's RADIUS server meets: a subscriber session torn down just after it gets its address can send the Accounting-Stop before the Accounting-Start, and the server is left with an open accounting record for a session that ended. |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-l2tp-session-identity-and-publish-stall.md` (same file, `acct.go`; its open question 3 decides where the Accounting-Stop wait runs, so this spec is coordinated with it, or merged into it, at that spec's design pass) |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

One defect met on 2026-11-12 while
`spec-single-owner-lifecycle-5-session-end-locals` was implemented. That spec
orders the L2TP session events; this race is inside the RADIUS accounting
reader, so the event order does not close it. It has a journal row (Owned
journal rows). Nothing here is designed yet.

| Part | Producer | What the operator meets today | Why it is owed |
|------|----------|-------------------------------|----------------|
| Accounting-Start can follow the Stop | `onSessionIPAssigned`, `runAccounting`, `onSessionDown` (`internal/component/l2tp/plugins/authradius/acct.go`) | `onSessionIPAssigned` starts `runAccounting` with `go`, and it sends the Start without checking its context. `onSessionDown` cancels that context and sends the Stop at once, without waiting for the task. A teardown just after the address is assigned can put the Stop on the wire first | The RADIUS server keeps an open record for a session that ended, so the operator's billing and session counts are wrong |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the constraints every change keeps
  → Constraint: a goroutine is a long-lived worker on a channel, never one per event.
- [ ] `ai/doors/access-security.md` - L2TP and RADIUS accounting
  → Constraint: a RADIUS server is an outside party; the billing records it keeps are its contract with Ze.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2866.md` - not read for this skeleton. The design reads `rfc/full/rfc2866.txt` Section 2 and quotes what it says about the Start and Stop of one session.

**Key insights:**
- The Stop is the one accounting record sent from outside the session's own accounting task.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/plugins/authradius/acct.go` - `onSessionIPAssigned` stores the accounting session and starts `runAccounting` on a new goroutine. `runAccounting` calls `sendAcctStart`, then `interimLoop`. `onSessionDown` removes the session from the map, calls `sess.cancel()`, and then calls `sendAcctStop` without waiting for `runAccounting`.

**Behavior to preserve:**
- One Accounting-Stop for each Accounting-Start, with the terminate cause the teardown named.
- The Acct-Session-Id that Start and Stop share.

**Behavior to change:**
- No Accounting-Start reaches the server after its session's Stop.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| [mechanism] | [file + symbol, or page + section] | [what it does today] | [extends / replaces / does not apply: why] |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters]

### Transformation Path
1. [Stage 1]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| [boundary] | [how] | No |

### Integration Points
- [Existing function/type this connects to] - [how it integrates]

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [config/CLI/event that triggers it] | → | [function that actually runs] | [test name proving the chain] |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The Stop can reach the server before the Start | Read 2026-11-12 in `acct.go`; not reproduced | The row closes with the evidence | A unit test that ends the session before the accounting task sends its Start | unvalidated |

## Acceptance Criteria

Draft row. The design pass confirms or rewrites it.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A session ends right after its address is assigned, before its accounting task has sent the Start | The server receives no Start after that session's Stop | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [named in design] | `internal/component/l2tp/plugins/authradius/` | AC-1 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| [named in design] | `test/interop/scenarios/` | FreeRADIUS | [protocol behavior validated] | |

## Files to Modify
- [named in design]

## Files to Create
- [named in design]

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- [named in design]

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/late-write-lands-on-the-successor.md` | 2026-11-12 | authradius Accounting-Start sent by `runAccounting` can follow the Stop |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- [What was deliberately not done and why]

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] No code snippets
- [ ] AC-N rows carry testable assertions

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] The targeted tests for what changed pass, run in the main checkout: `./le test unit pkg path ./internal/component/l2tp/plugins/authradius/` and the functional tests the design names. No agent runs any `./le verify` command

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
