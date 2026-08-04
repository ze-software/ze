# Spec: fixit-ike-dpd-cleartext

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/fixit-ike-dpd-cleartext.md` |
| Updated | 2026-07-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Found on 2026-07-30 during design work on an unrelated fix for
`plan/learned/1313-rfcgate-1b-rfc7296-pilot.md`. Owner ruling the same day: "Fix it now, and
extract the missing row."

## Task

**Ze sends its Dead Peer Detection probe in cleartext, so configuring DPD breaks the tunnel
it is meant to protect.**

`sendDPD` (`internal/component/ike/engine/dpd.go`) builds a `wire.Message` that
carries a `Header` and NOTHING else. No payload list, and no Encrypted payload. It writes
those bytes with `msg.CheckedWriteTo(buf, 0)` and sends them. It never calls
`buildEncryptedMessageEx`, which every other INFORMATIONAL request in the engine uses.

RFC 7296 section 1.4 describes the liveness check:

<!-- ste: ignore -->
> An INFORMATIONAL request with no payloads (other than the empty Encrypted payload required by the syntax) is commonly used as a check for liveness.

Section 1.4 requires the Encrypted payload. Ze's probe has none, so it is not an
INFORMATIONAL request. A conforming peer drops it. Ze would drop it too, because
`decryptAndParse` (`internal/component/ike/engine/inbound.go`) needs an SK payload.

**The user-visible consequence.** DPD is a configuration surface
(`internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`, with `interval`, `timeout`
and `action`). Any non-zero `interval` builds a live `dpdState`
(`internal/component/ike/engine/dpd.go`), which `established.go` wires into the
maintain loop. The probe then draws no answer, `timedOut` fires, and Ze declares a healthy
peer dead every DPD timeout.

It is latent today only because `newDPDState` returns nil at `Interval == 0`, so an
operator who never configures DPD never meets it.

**Why nothing caught it.** There is NO extracted requirement row for this section 1.4
obligation, so `make ze-rfc-check` cannot see it. The row that covers DPD is
`RFC7296-2.4-1`. It governs the answer to an empty INFORMATIONAL request, not the
well-formedness of the one Ze sends. `ai/rules/rfc-compliance.md` states the principle this breaks.
An unextracted obligation is still an obligation, and the gate's silence is not conformance.

## Required Reading

| Document | Why |
|----------|-----|
| `rfc/full/rfc7296.txt` sections 1.4 and 2.4 | The liveness check and the DPD rules, verbatim |
| `ai/rules/rfc-compliance.md` | An unextracted obligation is still an obligation |
| `internal/component/ike/engine/dpd.go` | The probe builder and the state machine |
| `internal/component/ike/engine/auth.go` | `buildEncryptedMessageEx`, which the fix must use |

## Current Behavior (MANDATORY)

Source files read on 2026-07-30:

- [ ] `internal/component/ike/engine/dpd.go`
- [ ] `internal/component/ike/engine/established.go`
- [ ] `internal/component/ike/engine/inbound.go`
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`
- [ ] `rfc/short/rfc7296.md`

`sendDPD` reserves the shared request window, builds the bare header, increments
`sa.NextMsgID`, and sends. The response path already expects a real message.
`handleOwnedInbound` authenticates an INFORMATIONAL response first, then credits liveness
(`inbound.go`). `matchesProbe` (`dpd.go`) correlates it by message id.

So the RECEIVE half is already correct. Only the SEND half is malformed.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

The `maintainSA` tick, `internal/component/ike/engine/established.go`, when
`dpd.shouldSend(now)` is true.

### Transformation Path

`sendDPD` reserves the request window, builds a message, and hands bytes to
`transport.UDPTransport`. The fix replaces the hand-built header with
`buildEncryptedMessageEx(sa, nil, sa.NextMsgID, wire.ExchangeInformational, initiatorFlag(sa))`,
which is what an empty INFORMATIONAL request is: an SK payload wrapping an empty chain.

The response returns through `classifyInbound`, is authenticated in `handleOwnedInbound`,
and credits liveness only when `matchesProbe` agrees on the message id.

### Boundaries Crossed

| Boundary | Direction | Carries |
|----------|-----------|---------|
| engine -> transport | out | The probe, which must now be encrypted and integrity-protected |
| transport -> engine | in | The peer's authenticated response |

### Integration Points

`rfc/short/rfc7296.md` is the checklist for RFC 7296. This spec ADDS a
row to `rfc/short/rfc7296.md`, so the pilot's row count changes and its Appendix A gains an
entry.

## Key Design Decisions

| Decision | Over | Because |
|----------|------|---------|
| Build the probe with `buildEncryptedMessageEx` and a nil inner chain | Hand-building an SK payload in `sendDPD` | One producer for every encrypted INFORMATIONAL. `handleInformationalOwned` already answers with a nil chain the same way |
| Extract the missing requirement row in the same spec | Fixing the code alone | The gate's blindness is why this survived. A fix with no row leaves the next regression equally invisible |
| Keep the request-window reservation exactly where it is | Moving it after the build | It landed in commit `b1c53ee3f` and is already proven. A build failure must release it, which the current code does |

## Blast Radius

`internal/component/ike/engine/dpd.go` only, plus one new row and its tagged tests. The
change is wire-visible and it moves DPD from broken to working. No operator can be relying
on the current behaviour, because the current behaviour is a peer that is always declared
dead.

## Risks & Assumptions

| Id | Statement | Basis | Validation |
|----|-----------|-------|------------|
| A-1 | `buildEncryptedMessageEx` accepts a nil inner chain | `handleInformationalOwned` passes nil at `inbound.go` | Read it, then assert the built probe decrypts to an empty chain |
| A-2 | The receive half already works | `inbound.go` authenticates first, then credits liveness | A round-trip test: probe out, response in, liveness credited |
| R-1 | An encrypted probe changes the message size, and some test asserts the old 28 bytes | The old probe is a bare header | Grep the suite for a 28-byte assertion first, then change the builder |

## Acceptance Criteria

| Id | Criterion |
|----|-----------|
| AC-1 | The DPD probe carries an Encrypted payload and authenticates under the SA's keys |
| AC-2 | The probe decrypts to an EMPTY inner payload chain, which is what section 1.4 describes |
| AC-3 | A peer SA can decrypt and parse the probe Ze sends |
| AC-4 | A round trip credits liveness: probe out, authenticated response in, `awaitReply` cleared |
| AC-5 | A new requirement row for that section 1.4 obligation exists in `rfc/short/rfc7296.md` |
| AC-6 | That row carries a positive AND a negative tagged test, and both are mutation-verified |
| AC-7 | The request window is still reserved before the send and released on a build failure |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | | Feature Code | Test |
|-------------|---|--------------|------|
| `maintainSA` tick with DPD due (`established.go`) | -> | the encrypted probe builder | `TestDpdProbeIsEncrypted` |
| The peer decrypting what Ze sent | -> | `decryptAndParse` | `TestDpdProbeDecryptsToEmptyChain` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | Proves |
|------|--------|
| `TestDpdProbeIsEncrypted` | AC-1, AC-6 |
| `TestDpdProbeDecryptsToEmptyChain` | AC-2, AC-3 |
| `TestDpdRoundTripCreditsLiveness` | AC-4 |
| `TestDpdBuildFailureReleasesWindow` | AC-7 |

### Functional Tests

| Test | Role |
|------|------|
| `test/ipsec/ipsec-sa-installed.ci` | Proves the SA still establishes with the changed probe path |
| `test/ipsec/ipsec-clear-reestablish.ci` | Proves teardown and re-establishment still work |

The ipsec suite runs on every push now that `ipsec` is in `all_suites`
(`mk/test-functional.mk`). A `.ci` that configures DPD and asserts the tunnel STAYS up
is the strongest daemon-level proof. Add one when the harness can drive a peer for longer
than one DPD interval.

## Files to Modify

| File | Change |
|------|--------|
| `internal/component/ike/engine/dpd.go` | Build the probe through `buildEncryptedMessageEx` |
| `rfc/short/rfc7296.md` | The new section 1.4 row |
| `plan/learned/1313-rfcgate-1b-rfc7296-pilot.md` | Appendix A gains the row, and the count changes |

## Files to Create

| File | Holds |
|------|-------|
| `internal/component/ike/engine/rfc7296_dpd_test.go` | The tagged pair and the round-trip tests |

## Implementation Steps

1. Write the failing tests, and record the red output.
2. Replace the hand-built header with `buildEncryptedMessageEx`.
3. Add the requirement row, with an id that does not collide with the existing section 1.4 ordinals.
4. Confirm green, then mutation-verify the tagged pair.
5. Run the ipsec `.ci` suite.

## Test Evidence

Tests FAIL, before the fix (`go test -run TestDpd ./internal/component/ike/engine/ -v`):

```
=== RUN   TestDpdProbeIsEncrypted
    rfc7296_dpd_test.go:97: the probe carries no Encrypted payload, so it is not protected
--- FAIL: TestDpdProbeIsEncrypted (0.05s)
=== RUN   TestDpdProbeDecryptsToEmptyChain
    rfc7296_dpd_test.go:124: the message did not authenticate under the expected IKE SA: no SK payload
--- FAIL: TestDpdProbeDecryptsToEmptyChain (0.05s)
=== RUN   TestDpdRoundTripCreditsLiveness
    rfc7296_dpd_test.go:143: the message did not authenticate under the expected IKE SA: no SK payload
--- FAIL: TestDpdRoundTripCreditsLiveness (0.06s)
=== RUN   TestDpdBuildFailureReleasesWindow
    rfc7296_dpd_test.go:184: a probe whose build failed: ze wrote 28 unexpected bytes before the sentinel
--- FAIL: TestDpdBuildFailureReleasesWindow (0.12s)
FAIL
```

The last line names the defect exactly: 28 bytes is the bare IKE header.

Tests PASS, after the fix:

```
=== RUN   TestDpdProbeIsEncrypted
--- PASS: TestDpdProbeIsEncrypted (0.06s)
=== RUN   TestDpdProbeDecryptsToEmptyChain
--- PASS: TestDpdProbeDecryptsToEmptyChain (0.03s)
=== RUN   TestDpdRoundTripCreditsLiveness
--- PASS: TestDpdRoundTripCreditsLiveness (0.03s)
=== RUN   TestDpdBuildFailureReleasesWindow
--- PASS: TestDpdBuildFailureReleasesWindow (0.03s)
PASS
ok  	github.com/ze-software/ze/internal/component/ike/engine	0.279s
```

Mutation. `sendDPD` was reverted to the bare-header build. `TestDpdProbeIsEncrypted`, which
carries both polarities of `RFC7296-1.4-5`, went red at the same assertion as the first red
run. The three sibling tests went red with it. The mutation was reverted and all four are
green again.

## Goal Gates

`make ze-verify`.

## Quality Gates

`make ze-lint-changed`, `make ze-rfc-check`, `python3 scripts/dev/ste_check.py --check`.

## RFC Documentation (Scope: protocol)

`rfc/short/rfc7296.md` gains one row. It states that an INFORMATIONAL request carries the
Encrypted payload that section 1.4 requires.

**The id is `RFC7296-1.4-5`.** Section 1.4 holds `1.4-1`, `1.4-3` and `1.4-4` in the
summary, so its high-water mark at HEAD is 4. `check_id_allocation`
(`scripts/dev/rfc_requirements.py`) refuses a new row at or below the mark, because a
returning id is indistinguishable from a text correction. The next free ordinal above the
mark is 5.

`plan/learned/1313-rfcgate-1b-rfc7296-pilot.md` planned this obligation as `RFC7296-1.4-2`, classed
**NOT IMPL**. That id sits below the mark, so the gate refuses it. Appendix A now carries
`RFC7296-1.4-5` and the class `impl-testable`, so the plan and the summary agree.

The row text is section 1.4's sentence verbatim: "INFORMATIONAL exchanges MUST ONLY occur
after the initial exchanges and are cryptographically protected with the negotiated keys."

## Checklist

- [ ] Tests written first, run, and Tests FAIL output pasted into the spec
- [ ] The probe is built through `buildEncryptedMessageEx`
- [ ] The new row exists and its id does not collide
- [ ] Tests PASS output pasted into the spec
- [ ] The tagged pair mutation-verified
- [ ] `make ze-verify` green

## Known Limitations

This fixes the probe Ze SENDS. The probe Ze RECEIVES is already handled correctly, so it
needs no change. ~~One question stays open and is not scoped here: what Ze does with a
bare-header probe from a peer in the wild.~~

**Answered 2026-08-03 (bookkeeping audit), so nothing is left open here.** `decryptAndParse`
(`internal/component/ike/engine/inbound.go`) returns "no SK payload" when a message carries
no Encrypted payload. That error is not `errInnerParse`, and `errInnerParse` is the marker
RFC 7296 Section 3.10.1's error-notification precondition rides on, so Ze answers a
bare-header probe with silence. That is the conformant outcome: Section 1.4 requires the
Encrypted payload, so a bare header is not an INFORMATIONAL request and there is nothing to
respond to. No code change and no deferral row follow from it.

**Deferral shard, created 2026-08-03.** The metadata row at the top named
`plan/deferrals/fixit-ike-dpd-cleartext.md` and that file did not exist, so this spec's one
real deferral was unrecorded and `/ze-close` had nothing to resolve. The shard now holds it:
the DPD hold-open `.ci` this spec's TDD Test Plan defers ("Add one when the harness can drive
a peer for longer than one DPD interval"), homed as item 5 and AC-8 of
`plan/spec-fixit-ike-test-discrimination.md`.

---

## Implementation Summary

### What Was Implemented
- `sendDPD` (`internal/component/ike/engine/dpd.go`) builds the liveness probe through
  `buildEncryptedMessageEx(sa, nil, msgID, wire.ExchangeInformational, initiatorFlag(sa))`.
  The nil inner chain IS the empty payload list of RFC 7296 Section 1.4, and the SK payload
  it produces is the protection that section requires.
- A build failure releases the window it reserved (`sa.releaseRequestWindow()`), writes no
  datagram, and leaves no outstanding wait. The reservation itself did not move.
- `rfc/short/rfc7296.md` gained row `RFC7296-1.4-5`, carrying Section 1.4's sentence.
- `internal/component/ike/engine/rfc7296_dpd_test.go` holds the tagged pair and three
  sibling tests.

### Bugs Found/Fixed
- The probe was a bare 28-byte IKE header. A conforming peer drops it, so every probe went
  unanswered and DPD declared a healthy peer dead at each timeout. Now covered by
  `TestDpdProbeIsEncrypted`, whose negative half feeds that exact bare header to
  `decryptAndParse` under the peer SA and requires the refusal.

### Documentation Updates
- No doc change was needed. `grep -rn "source: internal/component/ike/engine/dpd.go" docs/`
  returns nothing, so no anchored claim points at the changed producer.
  `docs/features/rfc-status.md`'s RFC 7296 row already discloses the behaviour: "Section 1.4
  liveness probes carry the Encrypted payload and authenticate under the negotiated keys."
- `make ze-doc-test` result: recorded in Pre-Commit Verification below.

### Deviations from Plan
- The requirement id is `RFC7296-1.4-5`, not the `RFC7296-1.4-2` that
  `plan/learned/1313-rfcgate-1b-rfc7296-pilot.md` planned. `check_id_allocation`
  (`scripts/dev/rfc_requirements.py`) refuses an id at or below a section's high-water mark,
  which was 4, because a returning id cannot be told from a text correction.
- The spec's Known Limitations left one question open ("what Ze does with a bare-header probe
  from a peer in the wild"). It was answered on 2026-08-03 rather than deferred: Ze answers
  with silence, and that is conformant. The paragraph in this spec carries the trace.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The spec's metadata named a deferral shard that did not exist, so the one real deferral was invisible to `/ze-close` | The deferred work was real and stated only in the TDD Test Plan prose | The 2026-08-03 bookkeeping audit | The shard was created and the row homed at `plan/spec-fixit-ike-test-discrimination.md` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The DPD probe must carry the Encrypted payload Section 1.4 requires | Done | `internal/component/ike/engine/dpd.go` `sendDPD` | Built by `buildEncryptedMessageEx` with a nil inner chain |
| Extract the missing Section 1.4 requirement row | Done | `rfc/short/rfc7296.md` row `RFC7296-1.4-5` | Id allocated above the section high-water mark |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `sendDPD` (`dpd.go`) calls `buildEncryptedMessageEx`; `TestDpdProbeIsEncrypted` asserts a `*wire.PayloadSK` is present and that the peer SA authenticates it | |
| AC-2 | Done | `TestDpdProbeDecryptsToEmptyChain` asserts the decrypted inner chain has zero payloads | The nil argument to `buildEncryptedMessageEx` is what produces it |
| AC-3 | Done | `TestDpdProbeIsEncrypted` runs `decryptAndParse(peer, ...)` over the datagram `sendDPD` wrote | A real peer SA, not a re-decrypt under our own |
| AC-4 | Done | `TestDpdRoundTripCreditsLiveness`: probe out, `handleOwnedInbound` reads the answer, `matchesProbe` agrees on the id, `handleDPDResponse` clears `awaitReply` | `handleDPDResponse` and `matchesProbe` (`dpd.go`) are the producers |
| AC-5 | Done | `rfc/short/rfc7296.md` carries `[RFC7296-1.4-5] [MUST]` | |
| AC-6 | Done | `rfc7296_dpd_test.go` carries `RFC7296-1.4-5 positive` and `RFC7296-1.4-5 negative` | Mutation recorded under Test Evidence: reverting `sendDPD` to the bare header reddens both polarities |
| AC-7 | Done | `sendDPD` calls `sa.reserveRequestWindow()` before the build and `sa.releaseRequestWindow()` on `err != nil`; `TestDpdBuildFailureReleasesWindow` drives it with a 7-octet SK_ei | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestDpdProbeIsEncrypted` | Done | `internal/component/ike/engine/rfc7296_dpd_test.go` | Carries both polarities of `RFC7296-1.4-5` |
| `TestDpdProbeDecryptsToEmptyChain` | Done | same file | |
| `TestDpdRoundTripCreditsLiveness` | Done | same file | |
| `TestDpdBuildFailureReleasesWindow` | Done | same file | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/ike/engine/dpd.go` | Done | `sendDPD` rebuilt on `buildEncryptedMessageEx` |
| `rfc/short/rfc7296.md` | Done | Row `RFC7296-1.4-5` added |
| `plan/learned/1313-rfcgate-1b-rfc7296-pilot.md` | Done | Appendix A carries the row and its class |
| `internal/component/ike/engine/rfc7296_dpd_test.go` | Done | Created |

### Audit Summary
- **Total items:** 13 (2 requirements, 7 ACs, 4 files)
- **Done:** 13
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0 (the id change is recorded in Deviations, and it is an allocation, not a scope change)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Configuring DPD must stop breaking the tunnel it protects: the probe must be a conforming INFORMATIONAL request | functional (Go, peer-SA round trip) | `TestDpdProbeIsEncrypted` decrypts the datagram `sendDPD` wrote under a SEPARATE peer SA. Its negative half proves the old bare header is refused by that same peer, so the test discriminates |
| The probe must decrypt to the empty chain Section 1.4 describes | functional | `TestDpdProbeDecryptsToEmptyChain`: `len(inner) == 0` |
| A liveness round trip must credit liveness | functional | `TestDpdRoundTripCreditsLiveness`: `out.dpdResp && out.dpdRespMsgID == probeID`, then `awaitingReply()` false and `requestOutstanding` false |
| The gate must be able to see this obligation in future | gate row | `make ze-rfc-check` covers `RFC7296-1.4-5`; the row did not exist before this spec |
| The daemon still establishes and tears down with the changed probe path | functional (`.ci`) | `test/ipsec/ipsec-sa-installed.ci` and `test/ipsec/ipsec-clear-reestablish.ci`, both in the `ipsec` suite that `mk/test-functional.mk` runs on every push |

**Known gap in this table, stated rather than hidden.** No test holds a configured DPD
`interval` open past one interval at daemon level. That is the deferral row below, and it is
homed at `plan/spec-fixit-ike-test-discrimination.md` AC-8.

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| A `.ci` that configures a DPD `interval` and asserts the tunnel is still established after more than one interval | deferred (LIVE) | `plan/spec-fixit-ike-test-discrimination.md`, item 5 and AC-8. That spec exists on disk and is open |

**The shard is NOT removed by this closure.** `plan/deferrals/fixit-ike-dpd-cleartext.md`
still holds one live row, so it outlives this spec (`ai/rules/planning.md`, and
`deferral_shard_removal_problems` in `scripts/dev/commit_helper.py` would block the removal).
No FOREIGN shard was emptied by this closure, so none is removed either.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-ike-dpd-cleartext-c4c78ddb-c47b-4f1a-a85d-5911d7c65455.md` |
| `review_gate.py check` | clean (`review_gate: OK (0 code files, clean, hashes match ...)`) |
| Reviewer lenses used | Three independent subagents through `/ze-review`: logic+wiring+removed-behaviour; security+edge-cases+test-vacuity; RFC 7296 conformance+interop |

**Round 1 scope, written before the round ran:** the whole five-spec IKE changeset at HEAD
(`git log --oneline -14 -- internal/component/ike/`), including the sibling call sites of
every changed function. Three lenses, because the ask was "re-verify before closing".

### Findings fixed
<!-- Only BLOCKER and ISSUE. Every finding was reproduced against source by the
     closing agent before it was graded; a reviewer can be wrong. -->
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | NOTE | A probe raised with a nil transport reserves the request window, sets `awaitReply` with a nil `probeMsg`, and nothing frees it: `serviceRequestWindow` returns early on `dpd.awaitingReply()`. Only `timedOut` ends it, by declaring a live peer dead after zero attempts | `internal/component/ike/engine/dpd.go` `sendDPD`; `internal/component/ike/engine/established.go` `serviceRequestWindow` | Not fixed. Reproduced by reading both producers, and latent: `maintainSA` always passes a real transport, so no production path reaches it. NOTEs do not block. Recorded in `plan/learned/1332-fixit-ike-dpd-cleartext.md` and homed as a row in this spec's surviving shard |
| 2 | ISSUE, outside this spec's scope | Row `RFC7296-2.4-2` [SHOULD] "liveness checks are demand-driven, not periodic" is not met: `handleDPDResponse` clears `awaitReply` but never resets `dpd.lastSent`, so `shouldSend` keeps firing on a fixed period whatever authenticated traffic arrived | `internal/component/ike/engine/dpd.go` `handleDPDResponse`, `shouldSend` | Not fixed here. Verified that the row carries NO tagged test, so `ze-rfc-check` reports it uncovered rather than green over a violation: it is a disclosed gap, not a hidden one. This spec claims `RFC7296-1.4-5` (the probe's wire form), not `RFC7296-2.4-2` (when to probe), and its goal holds with this open (`ai/rules/rule-precedence.md`: name it, home it, close, then fix it). Homed as a row in this spec's surviving shard |

No BLOCKER and no in-scope ISSUE was raised against the seven acceptance criteria of this
spec, so the loop ended at round 1.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/ike/engine/rfc7296_dpd_test.go` | Yes | `ls -la` on 2026-08-03: 7382 bytes |
| `internal/component/ike/engine/dpd.go` | Yes | Read in full on 2026-08-03; `sendDPD` at the `buildEncryptedMessageEx` call |
| `test/ipsec/ipsec-sa-installed.ci` | Yes | `ls test/ipsec/` |
| `test/ipsec/ipsec-clear-reestablish.ci` | Yes | `ls test/ipsec/` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2 | The probe is built encrypted with an empty inner chain | Read `sendDPD`: `buildEncryptedMessageEx(sa, nil, msgID, wire.ExchangeInformational, initiatorFlag(sa))`. `--- PASS: TestDpdProbeIsEncrypted (0.07s)` |
| AC-3 | A peer SA decrypts it | `--- PASS: TestDpdProbeDecryptsToEmptyChain (0.04s)` |
| AC-4 | A round trip credits liveness | `--- PASS: TestDpdRoundTripCreditsLiveness (0.03s)` |
| AC-5 | The row exists | `grep -n "RFC7296-1.4-5" rfc/short/rfc7296.md` -> line 526 |
| AC-6 | Both polarities are tagged | `grep -rn "RFC7296-1.4-5"` over `*.go` -> `rfc7296_dpd_test.go` positive and negative |
| AC-7 | The window is released on a build failure | Read `sendDPD`: `sa.releaseRequestWindow()` in the `err != nil` arm. `--- PASS: TestDpdBuildFailureReleasesWindow (0.03s)` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `maintainSA` tick with DPD due (`established.go`) -> the encrypted probe builder | `test/ipsec/ipsec-sa-installed.ci` | Partially. The `.ci` proves the SA establishes and the dataplane carries it with the changed probe path in the binary. It does NOT hold a DPD interval open; that is the live deferral row. The tick-to-builder link itself is proven in Go by `TestDpdProbeIsEncrypted`, which calls `sendDPD` and reads the datagram off the peer transport |
| The peer decrypting what Ze sent -> `decryptAndParse` | none | Proven in Go: `TestDpdProbeIsEncrypted` calls `decryptAndParse(peer, msg, probe)` on a SEPARATE SA |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `buildEncryptedMessageEx` accepts a nil inner chain; `TestDpdProbeDecryptsToEmptyChain` shows the built probe decrypts to zero payloads |
| A-2 | confirmed | The receive half needed no change: `handleOwnedInbound` authenticates before `matchesProbe` is consulted. `TestDpdRoundTripCreditsLiveness` drives it |
| R-1 | did not materialise | No test asserted a 28-byte probe. The only 28-byte reference in the suite is `TestDpdBuildFailureReleasesWindow`'s failure message, which asserts the ABSENCE of those bytes |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No `docs/` claim is anchored to the changed producer | `grep -rln "source: internal/component/ike/engine/dpd.go" docs/` returns nothing | Yes |
| RFC compliance disclosure | `docs/features/rfc-status.md` RFC 7296 row already states "Section 1.4 liveness probes carry the Encrypted payload and authenticate under the negotiated keys" | Yes |
| Config syntax, CLI, API, plugin SDK, wire format, comparison table, test infrastructure, architecture | No change. The DPD config surface (`interval`, `timeout`, `action`) is untouched; the wire form of the probe changed but no doc states it | Yes |

## Core Insight

The gate could not see this defect because nobody had written the obligation down. `make
ze-rfc-check` verifies that every requirement LISTED in a summary is covered; it cannot
verify that the summary lists every requirement. The row that looked like it covered DPD,
`RFC7296-2.4-1`, governs the ANSWER to an empty INFORMATIONAL request, not the
well-formedness of the one Ze sends. A checklist's silence is not conformance, and the
cheapest way to keep a violation invisible is to leave its obligation unextracted.
