# Spec: bmp-sflow-export-rfc-defects

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

Two defects in what Ze reports to a monitoring collector, found by the strict
re-read of `spec-rfc-requirement-quote-hand-backfill`. A BMP
station is told Ze closed a session the peer dropped, and an sFlow collector's
sample pool stops advancing once it saturates. Both were read at the producer
in HEAD on 2026-09-27.

## Defects

| ID | Source | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record |
|----|--------|----------------|----------|--------------------|------------------------|
| D1 | RFC 7854 Section 4.9 | "Reason 4: The remote system closed the session without a notification message.  This includes any unexpected termination of the transport session, so in some cases both the local and remote systems might consider this to apply." | `internal/component/bgp/plugins/bmp/bmp_events.go::peerDownFor` | Every close without a NOTIFICATION is reported as reason 2 (local close, FSM event 0); reason 4 is never sent, including for a TCP reset or a connection the peer dropped | no row names it |
| D2 | sFlow v5 (`rfc/full/sflow-v5.txt`, Section 5, `flow_sample`), row SFLOW-V5-x-11. NO NORMATIVE SENTENCE: a Ze behaviour defect measured against the field definition | the definition: "unsigned int sample_pool;      /* Total number of packets that could have been sampled (i.e. packets skipped by sampling process + total number of samples) */", and the note on `sequence_number`: "If the agent resets the sample_pool then it must also reset the sequence_number." | `internal/plugins/flowexport/sflow/flow_adapter.go::EncodeFlowSample` | The pool is estimated as sequence times rate and saturates at 2^32-1. No sentence of sFlow v5 says whether this 32-bit counter wraps or saturates. The defect is that a saturated value stops being the count the definition names, and the lowercase note treats the pool as a cumulative counter reset only together with the sequence number, which saturation is not. Once saturated, a collector's per-interval delta reads zero | weak; lead, not checked against a collector. Row SFLOW-V5-x-11 itself quotes a field description tagged MUST: a row correction, listed with the other sFlow corrections in `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` |

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | D1: a peer-down after a TCP loss yields reason 4. D2: a pool past 2^32 wraps |
| Both polarities | D1: a local close without NOTIFICATION stays reason 2, a NOTIFICATION stays reason 1 or 3. D2: below the limit the value is unchanged |
| Discrimination | `./le rfc discriminate-record` for every tagged unit |
| Interop | D1 against a BMP collector (the existing BMP interop scenario); D2 against sflowtool or host-sflow, reading the collector's delta |

## Owner decisions

| Row | Question |
|-----|----------|
| D2 | No sentence decides it, so the owner picks the behaviour: wrap modulo 2^32 (what a collector's delta assumes), and whether the pool should count packets actually seen rather than sequence times rate |

## Required Reading

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7854.md`, `rfc/short/sflow-v5.md`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/bmp/bmp_events.go` - `peerDownFor`, `handleSenderState`
- [ ] `internal/plugins/flowexport/sflow/flow_adapter.go` - `EncodeFlowSample`

**Behavior to change:** the two rows.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- a BGP session close event; a sampled packet

### Transformation Path
1. close reason to the BMP plugin; Peer Down message to the station
2. sample to the sFlow encoder; datagram to the collector

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ BMP plugin | structured state event with a close reason | No |

### Integration Points
- the reactor close reasons (`peer_run.go`), which today do not tell a remote close from a local one

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| peer drops TCP | → | `peerDownFor` | [to fill in design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | monitored peer closes TCP with no NOTIFICATION | Peer Down reason 4 |
| AC-2 | sample pool crossing 2^32 | wraps modulo 2^32 |
| AC-3 | the `weak` verdict of SFLOW-V5-x-11, after this spec's D2 producer fix | the verdict reaches `enforced`: a tagged test proves the quoted sentence, and an agent that did not write that test re-judges it with `./le rfc audit-stamp ... mode rejudge`. The row correction stays with `plan/pre-release/spec-rfc-verdict-fix-services.md`, whose "Blocked by" table moved the verdict here (parent P-3, 2026-09-28) |
| AC-4 | the `weak` verdict of RFC7854-x-10 ("Reason indicates why the session was closed", Section 4.9), after this spec's D1 producer fix | the verdict reaches `enforced`: tagged tests tie each observed close cause to its reason in `peerDownFor` (a TCP loss or a peer close without NOTIFICATION gives reason 4, a local close without NOTIFICATION reason 2, a sent or received NOTIFICATION reason 1 or 3, a deconfigured peer reason 5), both polarities, re-judged by an agent that did not write them with `./le rfc audit-stamp ... mode rejudge`. `plan/pre-release/spec-rfc-verdict-fix-bgp.md` moved the verdict here (parent P-3, 2026-09-29) |
| AC-5 | the `weak` verdict of RFC7854-4.9-1 (Section 4.9, reason 2: "Following the reason code is a 2-byte field containing the code corresponding to the Finite State Machine (FSM) Event that caused the system to close the session"), after this spec's D1 producer fix | a local close without NOTIFICATION reaches the collector as reason 2 whose 2-octet data is the RFC 4271 Section 8.1 number of the FSM event that closed the session (ManualStop gives 00 02, AutomaticStop 00 08), and 00 00 only where no relevant event is defined. Today `peerDownFor` writes the constant `fsmEventNone` for every reason 2, because `rpc.StructuredEvent` carries only a Reason string, so the event must cross the reactor and the plugin boundary. Tagged tests cover both polarities and are re-judged by an agent that did not write them with `./le rfc audit-stamp ... mode rejudge`. `plan/pre-release/spec-rfc-verdict-fix-bgp.md` moved the verdict here (parent ruling R13, 2026-09-29) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| one failing-first test per row | beside each producer | D1, D2 | [to fill in design] |

## Files to Modify

- the two producers, the reactor close-reason source for D1, their tests

## Implementation Steps

1. Failing tests; 2. fixes; 3. discrimination and verdicts.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)

### Verification
- [ ] `./le verify worktree`
