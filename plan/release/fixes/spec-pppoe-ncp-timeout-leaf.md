# Spec: pppoe-ncp-timeout-leaf

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P1 |
| Release reason | Shipped PPPoE sessions wait a fixed 30 s for NCP, against RFC 1661 Section 3.6's SHOULD on the default path; L2TP already lets the operator set the bound, so this is shipped behaviour to correct, not an absent feature |
| RFC gaps | RFC1661-3.6-4 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-20 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

PPPoE has no `ncp timeout` leaf. The PPP backend already takes a per-session
NCP bound (`ppp.StartSession.IPTimeout`, read as `ipTimeout` by
`runNCPPhase` and `awaitIPDecision`), and L2TP sets it from its
`l2tp ncp timeout` leaf (`reactor_kernel.go`, `IPTimeout: policy.NCPTimeout`).
The PPPoE server builds its `ppp.StartSession` (`pppoe/server.go`) without
`IPTimeout`, and `pppoe/config.go` `Parameters` carries no NCP timeout, so
every PPPoE session waits the fixed `defaultIPTimeout` of 30 s (`ppp/ncp.go`).
RFC 1661 Section 3.6: "Because an implementation may initially use a
significant amount of time for link quality determination, implementations
SHOULD avoid fixed timeouts when waiting for their peers to configure a NCP."
The goal is an operator-configurable PPPoE NCP timeout that reaches
`IPTimeout` on every new PPPoE session, with the RFC1661-3.6-4 row of
`rfc/short/rfc1661.md` updated to the configuration each transport runs.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/l2tp/bng-5-pppoe.md` - PPPoE configuration, the `// Design:` page of `pppoe/config.go`
  → Decision: [to fill at design]
  → Constraint: [to fill at design]

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc1661.md` - row RFC1661-3.6-4 carries the Section 3.6 SHOULD
  → Constraint: [to fill at design]

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/ppp/ncp.go` - `runNCPPhase` and `awaitIPDecision` use `ipTimeout`, falling back to `defaultIPTimeout` (30 s) when zero
- [ ] `internal/component/l2tp/ppp/start_session.go` - `StartSession.IPTimeout`, zero means the package default
- [ ] `internal/component/l2tp/pppoe/config.go` - `ExtractParameters` reads no NCP timeout; `Parameters` has no field for one

**Behavior to preserve:**
- [to fill at design]

**Behavior to change:**
- [to fill at design]

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| L2TP `ncp` container | `internal/component/l2tp/yang/ze-l2tp-conf.yang`, `container ncp`, `leaf timeout` (uint16, range 1..3600, default 30) | Declares the L2TP NCP enables and the NCP timeout | Extends: the PPPoE leaf follows the same shape, so both transports read alike |
| L2TP NCP timeout parse path | `internal/component/l2tp/config.go` `ExtractParameters` (`ncpC.Get("timeout")` into `Parameters.NCPTimeout`), `subsystem_reload.go` hot-apply, `reactor_kernel.go` `IPTimeout: policy.NCPTimeout` | Carries the leaf to each new L2TP session's `ppp.StartSession` | Extends: PPPoE gets the equivalent path from `pppoe/config.go` `ExtractParameters` to `pppoe/server.go` |
| `ppp.StartSession.IPTimeout` | `internal/component/l2tp/ppp/start_session.go` `StartSession.IPTimeout`; `ppp/ncp.go` `runNCPPhase`, `awaitIPDecision`, `defaultIPTimeout` | Bounds the wait for NCP per session; zero falls back to 30 s | Extends: PPPoE sets it; the backend is unchanged |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters: wire bytes, API command, config, plugin message]
- [Format at entry]

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
| A-1 | [to fill at design] | [basis] | [impact] | [method] | unvalidated |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [config/CLI/event that triggers it] | → | [function that actually runs] | [test name proving the chain] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | [to fill at design] | [positive observable outcome] | [what remains permitted and why] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXxx` | `internal/.../xxx_test.go` | [to fill at design] | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-xxx` | `test/.../*.ci` | [to fill at design] | |

## Files to Modify
- `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` - [to fill at design]
- `internal/component/l2tp/pppoe/config.go` - [to fill at design]
- `internal/component/l2tp/pppoe/server.go` - [to fill at design]

## Files to Create
- [to fill at design]

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- [to fill at design]

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `schema-omits-a-leaf-the-backend-supports.md` | 2026-11-20 | PPPoE has no `ncp timeout` leaf; every session waits the fixed `defaultIPTimeout` |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted tests pass in the main checkout: `./le test unit pkg path ./internal/component/l2tp/pppoe/`, `./le test pppoe`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for the timeout range

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** remove the spec, in the same `./le commit create` script
