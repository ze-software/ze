# Spec: ntp-client-kiss-of-death

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | RFC 5905 is optional in the fttx-edge profile, so its Kiss-o'-Death handling is post-release work; the NTP client still ships and still owes it |
| RFC gaps | RFC5905-7.4-1, RFC5905-7.4-2, RFC5905-7.4-3, RFC5905-7.4-4, RFC5905-7.4-5 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze's NTP client never reads a kiss code. `syncWorker.doSync`
(`internal/plugins/ntp/ntp.go`) reads `resp.Time`, derived from the
transmit timestamp, for its absurd-year check before `resp.Validate` refuses a
stratum-0 packet as a generic error. A server answering DENY or RSTR stays in
the list and is queried every round, and RATE never changes polling.

RFC 5905 text, `rfc/full/rfc5905.txt` Section 7.4:

- "Recipients of kiss codes MUST inspect them and, in the following cases, take these actions:"
- "For kiss codes DENY and RSTR, the client MUST demobilize any associations to that server and stop sending packets to that server;"
- "For kiss code RATE, the client MUST immediately reduce its polling interval to that server and continue to reduce it each time it receives a RATE kiss code."
- "Kiss codes beginning with the ASCII character "X" are for unregistered experimentation and development and MUST be ignored if not recognized."
- "The Receive Timestamp and the Transmit Timestamp (set by the server) are undefined when in a KoD packet and MUST NOT be relied upon to have valid values and MUST be discarded."

Section 7.4's RATE sentence says "reduce its polling interval"; design reads
chrony (`ntp_core.c`, the KoD RATE handling) and ntpd before choosing what Ze
does, and quotes the deciding function. The 2026-10-19 row in
`error-answered-with-another-errors-code.md` (unsynchronised chrony, no kiss
code) is a separate occurrence on the same function.

## Required Reading

- `rfc/short/rfc5905.md`, `rfc/full/rfc5905.txt` Section 7.4
- The NTP page named for `internal/plugins/ntp/ntp.go` in `ai/CODE-TO-DOCS.md`

## Current Behavior (MANDATORY)

**Source files read:** (2026-10-26; design re-reads before any decision)
- [ ] `internal/plugins/ntp/ntp.go` - `doSync` queries each server in turn, applies the absurd-year check on `resp.Time`, then `resp.Validate`; `run` uses the fixed `IntervalSec`

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- NTP response from a configured server

### Transformation Path
1. `ntp.Query` response
2. `doSync` checks and per-server state
3. Clock adjustment

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Network to NTP plugin | beevik/ntp `Response` | No |

### Integration Points
- Per-server state shown by the NTP status command

## Risks & Assumptions

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A demobilized server never returns after its operator lifts the restriction | Status shows the server stopped indefinitely | Re-admit on config reload and say so in status |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Configured server answering KoD DENY | → | `doSync` kiss handling | functional test with a stub NTP server (to write) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC5905Section74KissCodes` | `internal/plugins/ntp/` | AC-1 to AC-4 | |

Scoped evidence: `./le test unit` over `internal/plugins/ntp/`.

## Files to Modify
- `internal/plugins/ntp/ntp.go` - kiss-code inspection, demobilize, back-off
- `rfc/short/rfc5905.md` - remove the five `{gap}` annotations

## Implementation Steps

1. Read the kiss code before any timestamp
2. DENY and RSTR stop the server; RATE backs off; unknown X codes ignored

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `error-answered-with-another-errors-code.md` | 2026-10-26 | `doSync` never reads a kiss code |

## Acceptance Criteria

| AC | Criterion |
|----|-----------|
| AC-1 | A KoD packet is identified by its kiss code before any timestamp is read, and its timestamps are discarded |
| AC-2 | DENY or RSTR stops all queries to that server, and status says why |
| AC-3 | RATE lengthens the poll interval to that server on each receipt |
| AC-4 | An unrecognised kiss code starting with X is ignored |
