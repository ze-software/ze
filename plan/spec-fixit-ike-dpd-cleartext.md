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
needs no change. One question stays open and is not scoped here: what Ze does with a
bare-header probe from a peer in the wild.
