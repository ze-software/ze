# Deferrals: fixit-ike-dpd-cleartext

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-03 | spec-fixit-ike-dpd-cleartext TDD Test Plan | A `.ci` that configures a DPD `interval` and asserts the tunnel is STILL established after more than one interval has passed. The four unit tests in `internal/component/ike/engine/rfc7296_dpd_test.go` prove the probe is built encrypted; nothing proves a configured DPD does not tear a healthy tunnel down, which is the field symptom the cleartext defect produced | The ipsec `.ci` harness cannot yet drive a peer for longer than one DPD interval, which the spec states as the condition for adding the test | plan/spec-fixit-ike-test-discrimination.md | deferred |

Written on 2026-08-03 by a bookkeeping audit. The spec's metadata row named this file and
the file did not exist, so `/ze-close` had nothing to resolve. The row above is the work the
spec actually deferred, recovered from its TDD Test Plan. It is homed as item 5 and AC-8 of
`plan/spec-fixit-ike-test-discrimination.md`.

The spec's Known Limitations also asks "what Ze does with a bare-header probe from a peer in
the wild". That is NOT a deferral row: the behaviour is determined and conformant.
`decryptAndParse` (`internal/component/ike/engine/inbound.go`) returns "no SK payload" for a
message carrying no Encrypted payload, and that error is not `errInnerParse`, so RFC 7296
Section 3.10.1's precondition for an error notification is not met and Ze answers nothing.
Silence is what Section 1.4 asks for, because a probe with no Encrypted payload is not an
INFORMATIONAL request. No work is outstanding.
