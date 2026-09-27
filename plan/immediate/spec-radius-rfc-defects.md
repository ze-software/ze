# Spec: radius-rfc-defects

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Three defects in Ze's RADIUS client and its L2TP dynamic-authorization and
accounting use, found by the strict re-read of
`spec-rfc-requirement-quote-hand-backfill`. Each was read at
its producer in HEAD on 2026-09-27.

## Defects

| ID | RFC section | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record |
|----|-------------|----------------|----------|--------------------|------------------------|
| D1 | RFC 3579 Section 3.3 [Note 1] (RFC3579-3.3-2) | "An Access-Request that contains either a User-Password or CHAP-Password or ARAP-Password or one or more EAP-Message attributes MUST NOT contain more than one type of those four attributes." | `internal/component/radius/client.go::oneCredentialType` | The credential set holds three of the four types; ARAP-Password (70) is missing, so EAP-Message beside ARAP-Password encodes. Ze builds no ARAP today, so only a caller-supplied attribute reaches it | weak |
| D2 | RFC 5176 Section 2.3 (RFC5176-2.3-2) | "A Dynamic Authorization Server implementing this specification MUST be capable of detecting a duplicate request if it has the same source IP address, source UDP port, and Identifier within a short span of time." | `internal/component/l2tp/plugins/authradius/coa.go::replayKey` | The key is source IP, Code, Identifier and Request Authenticator: the source UDP port is ignored, so two clients behind one address collide, and the Authenticator makes a same-port same-Identifier retransmission with a changed Authenticator look new | weak; lead |
| D3 | RFC 2866 Section 5.5 | "This attribute is a unique Accounting ID to make it easy to match start and stop records in a log file." | `internal/component/l2tp/plugins/authradius/acct.go::genSessionID` | The id is tunnel, session and a process-local counter, so after a restart ids repeat in the server's log | suspected; the sentence is descriptive, not a MUST |

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | one per row, red against HEAD |
| Both polarities | D1: each single credential type encodes, each pair refuses. D2: same IP, port and Identifier is a duplicate; a different port is not. D3: ids differ across a simulated restart |
| Discrimination | `./le rfc discriminate-record` for every tagged unit |
| Interop | D2 against FreeRADIUS `radclient` sending CoA from two source ports; D3 against a FreeRADIUS accounting log |

## Owner decisions

| Row | Question |
|-----|----------|
| D3 | Which restart-unique scheme makes the Acct-Session-Id unique across restarts: a boot counter kept in storage, or a time component? |

## Required Reading

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc3579.md`, `rfc/short/rfc5176.md`, `rfc/short/rfc2866.md`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/radius/client.go` - `oneCredentialType`
- [ ] `internal/component/l2tp/plugins/authradius/coa.go` - `replayKey`
- [ ] `internal/component/l2tp/plugins/authradius/acct.go` - `genSessionID`

**Behavior to change:** the three rows.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- an Access-Request built by Ze; a CoA or Disconnect-Request received on UDP 3799; an L2TP session start

### Transformation Path
1. request build and validation in the RADIUS client
2. DAS duplicate detection before acting on a CoA
3. accounting id generation at session start

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| L2TP plugin ↔ RADIUS component | client API | No |

### Integration Points
- the authradius plugin's UDP listener, which has the source port

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| CoA on UDP 3799 | → | `replayKey` | [to fill in design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Access-Request with EAP-Message and ARAP-Password | refused |
| AC-2 | CoA retransmitted from the same IP and port with the same Identifier | answered once |
| AC-3 | accounting after a restart | no repeated Acct-Session-Id |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| one failing-first test per row | beside each producer | D1 to D3 | [to fill in design] |

## Files to Modify

- the three producers and their tests

## Implementation Steps

1. Failing tests; 2. fixes; 3. discrimination and verdicts.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)

### Verification
- [ ] `./le verify worktree`
