# Spec: runtime-drain-and-undrain

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | spec-daemon-drain-before-stop |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

spec-daemon-drain-before-stop drains BGP, OSPF and IS-IS only as part of a
daemon stop: the drain RPC runs once, after the config fence, and the daemon
exits after it. An operator who wants traffic moved away for maintenance,
without stopping Ze, has no way to ask for it, and no way to bring the router
back into service afterwards.

Owner, 2026-10-13 (drain spec Q-2): "we want runtime but it need design". The
runtime drain, its cancel ("undrain") and its status are wanted, and are
designed here rather than in the stop drain spec.

Bucket: `plan/` (backlog). By `plan/README.md`'s test the release goes out
without it: an operator on the first release can still drain by stopping the
daemon, and meets no wrong answer, only a missing maintenance verb.

Known starting points, to be confirmed at design:

- The stop drain's RPC carries only kind, restart cause and deadline. Any
  field the runtime form needs (an operator reason, for one) is designed
  here, not inherited. The runtime form also needs the reverse RPC (undrain), which the stop drain
  never sends: the stop drain's state lives until the process exits.
- The stop drain refuses a second `drain` request on a connection (its AC-28)
  and runs the handler on a one-time lifecycle goroutine. A runtime drain that
  can be entered and left repeatedly needs a different lifecycle for that
  goroutine (`ai/rules/goroutine-lifecycle.md`).
- Form to choose at design: runtime verbs (`request drain`, `request drain
  cancel`), a config leaf that survives a restart (FRR `bgp graceful-shutdown`,
  EOS maintenance units), or both. `request quiesce` already exists in
  `ze-system-cmd.yang` with an unrelated meaning (an async-work barrier), so
  the name must not collide with it.
- RFC 8326 Section 4.3 asks for "configuration knobs that utilize the
  GRACEFUL_SHUTDOWN community to inform BGP neighbors in preparation for an
  impending neighbor shutdown". A per-neighbour form would close the partial
  Section 4.3 row the stop drain spec records.

## Required Reading

### Architecture Docs

- `plan/immediate/spec-daemon-drain-before-stop.md` (Q-2, the drain RPC, the per-protocol drain state)
- `docs/architecture/hub-architecture.md`
- `docs/architecture/plugin/plugin-system.md`

### RFC Summaries (Scope: protocol)

- `rfc/short/rfc8326.md` - BGP graceful shutdown (Section 4.3 knobs)
- `rfc/short/rfc6987.md` - OSPF stub router
- `rfc/short/rfc3787.md` - IS-IS overload bit

## Current Behavior (MANDATORY)

- [ ] `internal/core/ipc/yang/ze-system-cmd.yang` -> `request` holds `shutdown`, `reboot`, `halt`, `reload` and `quiesce`; none drains routing (2026-10-13). `quiesce` blocks until pending async work has drained, an unrelated meaning
- [ ] the drain RPC and per-protocol drain state -> designed in spec-daemon-drain-before-stop, not yet implemented; this spec starts after it lands

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
To be designed.

### Transformation Path
To be designed.

### Boundaries Crossed
To be designed.

### Integration Points
To be designed.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point -> Feature Code -> Test |
|-------------------------------------|
| To be designed -> to be designed -> to be designed |

## Acceptance Criteria

To be written at design.

## Risks & Assumptions

To be written at design.

## 🧪 TDD Test Plan

### Unit Tests

| Test | Asserts |
|------|---------|
| To be designed | To be designed |

### Interop Tests (Scope: protocol)

To be designed.

## Files to Modify

To be decided at design.

## Implementation Steps

To be decided at design.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before the change
- [ ] Tests PASS after the change
- [ ] `./le verify worktree`
