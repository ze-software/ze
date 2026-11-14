# Spec: bfd-shared-session-same-remote-system

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | `rfc/short/rfc5882.md` names no release profile in its Use cases row (it is `-`), so the unproven part of RFC5882-4.4-1 waits until after the first release |
| RFC gaps | RFC5882-4.4-1 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Provenance: `spec-rfc-verdict-test-fix-pass` (closed 2026-11-14,
`plan/learned/051-rfc-verdict-test-fix-pass.md`) needed every unresolved verdict
named in an owning spec's acceptance criteria. `RFC5882-4.4-1` is `partial`. Its
earlier owner, AC-24 of `spec-bgp-live-receive-binding-and-bfd-keep-session`,
closed on 2026-10-13, and no live spec names the row.

RFC 5882 Section 4.4: "If multiple control protocols wish to establish BFD
sessions with the same remote system for the same data protocol, all MUST share
a single BFD session."

What is proven (`rfc/audit/rfc5882/RFC5882-4.4-1.json`, tested selector "all
MUST share a single BFD session"): requests that reach one canonical key share
one session at refcount 2, and that session lives until the last release. A
request to a different remote system gets its own session.

What is not proven (gap selector "with the same remote system"), read at
`internal/component/bfd/engine/shared_session.go`:

- `sharesSession` matches an IPv6 link-local peer only when both keys name the
  same interface. A client that names no interface does not share with a
  session that names one. RFC 4007 Section 6 is the reason: fe80::1 with no link
  does not name one system.
- `sharedEntryLocked` returns no session when a request with an unset field
  matches two or more live sessions (`sharedMatch.only`). The request then gets
  a session of its own, because it could mean either one.
- `sharesSession` compares `Mode`, so two clients that ask for different hop
  modes to one peer address get two sessions. The audit leaves this case
  unjudged and unproven.

The open design question: in each of these cases, can Ze learn that two clients
name the same remote system (for example by resolving the link or the local
address from the client's own session), or does the client's config have to
name it? If config, the operator owes a message that names the field to set.
The design phase owes the FRR and BIRD source reading that `ai/META-DECISIONS.md`
requires before a protocol design is approved, with the deciding function
quoted.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/routing-protocols.md` - the BFD component and its clients
- [ ] `docs/architecture/bfd.md` - the BFD service and session sharing

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5882.md` - row RFC5882-4.4-1 and its `{partial}` marker
- [ ] `rfc/full/rfc5882.txt` Section 4.4; `rfc/full/rfc5883.txt` Section 4.1; `rfc/full/rfc5881.txt` Section 2

## Current Behavior (MANDATORY)

**Source files read:** (2026-11-14, skeleton survey only)
- [ ] `internal/component/bfd/engine/shared_session.go` - `sharedEntryLocked`, `sharedMatch.only`, `sharesSession`

**Behavior to preserve:** RFC 5883 Section 4.1, two named local addresses never share. RFC 5881 Section 2, two named links never share.

**Behavior to change:** the design phase decides this.

## Existing Mechanisms (MANDATORY)

Candidates only. The design phase confirms or rejects each one.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Shared session lookup | `internal/component/bfd/engine/shared_session.go` `sharedEntryLocked`, `sharesSession` | Joins a request to the one live session whose key it can name, and refuses a guess | Candidate to extend: the gap cases are decided here |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A client's `EnsureSession` (BGP, OSPF, static routes) through the BFD service. The design phase confirms this.

### Transformation Path
1. The design phase fills this.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Client protocol → BFD service | `api.GetService`, in process only | [ ] |

### Integration Points
- `internal/component/bfd/engine/` - shared session lookup (candidate)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Two clients ask for a session to one remote system in a gap case | → | shared session lookup | design phase names it |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Row `RFC5882-4.4-1`, audit verdict `partial`, gap "with the same remote system". Two control protocols ask for BFD sessions to the same remote system in each case the Task lists: an IPv6 link-local peer named with and without an interface, a request with an unset field while two or more live sessions to that peer differ in that field, and two hop modes to one peer address | Where the design shows the requests name one remote system, they share one session at refcount 2. Where Ze cannot know it, the client config is refused or warned about with the field to set. Proven when a tagged unit drives each case through `sharedEntryLocked` and asserts the session count and refcount, `./le rfc record` observes it red under a recorded break, the `{partial}` marker loses its gap, and an agent that wrote none of it re-judges the verdict `enforced` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| named at design | `internal/component/bfd/engine/` | AC-1, one subtest per gap case | planned |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| named at design | `test/bfd/` | BGP and OSPF to one link-local neighbour share one BFD session | planned |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| named at design | `test/interop/scenarios/` | FRR or BIRD | The peer sees one BFD session for two Ze clients | planned |

## Files to Modify
- `internal/component/bfd/engine/shared_session.go` - candidate, confirmed at design
- `rfc/short/rfc5882.md` - the `{partial}` marker, once AC-1 is met

## Implementation Steps
1. Design phase: read FRR's and BIRD's session-sharing source, then answer the design question in the Task.
2. Write the red unit, then the fix, then record the break with `./le rfc record`.
3. Independent re-judge of `rfc/audit/rfc5882/RFC5882-4.4-1.json`.

## Known Limitations
- Skeleton: no design yet.

## RFC Documentation (Scope: protocol)
The changed code carries `// RFC 5882 Section 4.4: "..."` above the decision, quoted byte-exact.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1 demonstrated
- [ ] `./le test unit pkg path ./internal/component/bfd/engine/` passes
- [ ] `./le test bfd` passes
- [ ] `./le rfc check` clean for `rfc5882`
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
