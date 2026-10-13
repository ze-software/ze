# Spec: rsvp-te-graceful-shutdown-rfc5817

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

When Ze stops, RSVP-TE tunnels through it are torn down without warning: the
drain added by spec-daemon-drain-before-stop moves BGP, OSPF and IS-IS traffic
away first, and LDP follows the IGP, but RSVP-TE has no graceful shutdown.
RFC 5817 defines one: before a node or link goes down, it signals the
head-ends so they re-route their LSPs first.

Owner, 2026-10-13 (drain spec Q-7): "make a note to eventually implement
RFC 5817 - not urgent". Hence this skeleton, in the backlog bucket: the release
goes out without it. Until then the drain spec records it as a conformance gap.

Enrolling RFC 5817 lifts the 2026-10-08 RFC freeze for this document, so the
design session asks the owner before writing `rfc/short/rfc5817.md`.

## Required Reading

### Architecture Docs

- `docs/architecture/rsvpte/mpls-rsvp-te.md`
- `docs/architecture/rsvpte/mpls-rsvp-te-fast-reroute.md`
- `plan/immediate/spec-daemon-drain-before-stop.md` (Q-7, and the drain this spec extends)

### RFC Summaries (Scope: protocol)

None yet: `rfc/full/rfc5817.txt` is not in the tree and the RFC freeze holds until the owner lifts it for this document (see Task).

## Current Behavior (MANDATORY)

- [ ] `internal/plugins/rsvpte/register.go` -> package registration; the package implements RSVP-TE (RFC 3209) over raw IP protocol 46. No file in `internal/plugins/rsvpte/` mentions RFC 5817 or graceful shutdown (grep, 2026-10-13)
- [ ] `internal/plugins/rsvpte/engine.go` -> engine run loop and stop state; its shutdown path is to be audited at design

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
