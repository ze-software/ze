# Deferrals: fixit-ike-dpd-cleartext

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-03 | spec-fixit-ike-dpd-cleartext TDD Test Plan | A `.ci` that configures a DPD `interval` and asserts the tunnel is STILL established after more than one interval has passed. The four unit tests in `internal/component/ike/engine/rfc7296_dpd_test.go` prove the probe is built encrypted; nothing proves a configured DPD does not tear a healthy tunnel down, which is the field symptom the cleartext defect produced | The ipsec `.ci` harness cannot yet drive a peer for longer than one DPD interval, which the spec states as the condition for adding the test | plan/spec-fixit-ike-test-discrimination.md | deferred |
| 2026-08-03 | closure review of spec-fixit-ike-dpd-cleartext, all three lenses | `sendDPD` (`internal/component/ike/engine/dpd.go`) with a nil transport reserves the request window, skips the build, and still sets `awaitReply` with a nil `probeMsg`. `serviceRequestWindow` (`established.go`) returns early on `dpd.awaitingReply()`, so nothing frees the window and only `timedOut` ends it, by declaring a live peer dead after zero attempts. RFC 7296 Section 2.4 asks for repeated unanswered attempts first | Graded NOTE: latent, because `maintainSA` always passes a real transport. The fix is an early return before `reserveRequestWindow`. It is the exact shape that spec's Task states: a resource acquired on the way in must be released on every way out | plan/spec-fixit-ike-resource-lifetime-leaks.md | deferred |
| 2026-08-03 | closure review of spec-fixit-ike-dpd-cleartext, RFC-conformance lens | Row `RFC7296-2.4-2` [SHOULD] "liveness checks are demand-driven, not periodic" is not met. `handleDPDResponse` (`internal/component/ike/engine/dpd.go`) clears `awaitReply` but never resets `dpd.lastSent`, so `shouldSend` keeps firing at `lastSent + interval` whatever authenticated traffic arrived. RFC 7296 Section 2.4: "Receipt of a fresh cryptographically protected message on an IKE SA or any of its Child SAs ensures liveness". Child SA data-plane traffic is not observed at all | Not this spec's requirement. It claims `RFC7296-1.4-5`, the probe's WIRE FORM, and this row governs WHEN to probe. The spec's goal holds with it open (`ai/rules/rule-precedence.md`: name it, home it, close, then fix it). The row carries no tagged test, so `ze-rfc-check` already reports it uncovered rather than green over a violation | **NEEDS A DESTINATION. No open spec covers DPD scheduling, so this needs a new one and the owner picks its shape.** Not `fixit-ike-resource-lifetime-leaks` (nothing leaks), not `fixit-ike-test-discrimination` (the behaviour is missing, not the test) | deferred |

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
