# Deferrals: fixit-ike-dpd-cleartext

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-03 | spec-fixit-ike-dpd-cleartext TDD Test Plan | A `.ci` that configures a DPD `interval` and asserts the tunnel is STILL established after more than one interval has passed. The four unit tests in `internal/component/ike/engine/rfc7296_dpd_test.go` prove the probe is built encrypted; nothing proves a configured DPD does not tear a healthy tunnel down, which is the field symptom the cleartext defect produced | ~~The ipsec `.ci` harness cannot yet drive a peer for longer than one DPD interval, which the spec states as the condition for adding the test~~ The premise was wrong, not merely stale: the harness never carried that limit. `runCISubcommandInner` (`internal/test/cli/ci_runner.go`) sets a 15s default budget that `resolveOrchestratedTimeout` (`internal/test/runner/runner_exec_util.go`) lets any `.ci` override with no cap, and `parseDPD` (`internal/component/ike/ipsec/config.go`) accepts an interval of 1 second. `ipsec-clear-reestablish.ci` had already declared 60s | plan/spec-fixit-ike-test-discrimination.md | done |
| 2026-08-03 | closure review of spec-fixit-ike-dpd-cleartext, all three lenses | `sendDPD` (`internal/component/ike/engine/dpd.go`) with a nil transport reserves the request window, skips the build, and still sets `awaitReply` with a nil `probeMsg`. `serviceRequestWindow` (`established.go`) returns early on `dpd.awaitingReply()`, so nothing frees the window and only `timedOut` ends it, by declaring a live peer dead after zero attempts. RFC 7296 Section 2.4 asks for repeated unanswered attempts first | Graded NOTE: latent, because `maintainSA` always passes a real transport. The fix is an early return before `reserveRequestWindow`. It is the exact shape that spec's Task states: a resource acquired on the way in must be released on every way out. DONE 2026-08-07: `sendDPD` returns before `reserveRequestWindow` when `sa.sendPath(tr)` answers nil, and the probe build moved out of the `tr != nil` branch, so the awaiting state is entered only for a datagram `sendRaw` was given. The predicate is the send path and not the `tr` argument, because `tr` is a fallback that a floated SA ignores in favor of `nattSocket`; `TestDPDFloatedSAProbesWithoutTheFallback` holds that line. `TestDPDNoTransportTakesNoWindow` (`internal/component/ike/engine/dpd_test.go`) asserts no window, no `awaitReply`, no stored probe, and no Message ID spent, with a real-transport control in the same test. Reverting the guard reddens four of its assertions plus the control, because the stranded window also made `reserveRequestWindow` refuse the next real probe. `TestSesPeerFailedOnlyAfterRepeatedSilence` (`rfc7296_session_test.go`, RFC7296-2.4-11) had the defect in its fixture and now raises a real probe through `dpdProbeLink`, approved by Thomas on 2026-08-07 and recorded with `rfc-test-change-approved`. `make ze-test-pkg PKG=./internal/component/ike/engine` is green | plan/spec-fixit-ike-resource-lifetime-leaks.md | done |
| 2026-08-03 | closure review of spec-fixit-ike-dpd-cleartext, RFC-conformance lens | Row `RFC7296-2.4-2` [SHOULD] "liveness checks are demand-driven, not periodic" is not met. `handleDPDResponse` (`internal/component/ike/engine/dpd.go`) clears `awaitReply` but never resets `dpd.lastSent`, so `shouldSend` keeps firing at `lastSent + interval` whatever authenticated traffic arrived. RFC 7296 Section 2.4: "Receipt of a fresh cryptographically protected message on an IKE SA or any of its Child SAs ensures liveness". Child SA data-plane traffic is not observed at all | Not this spec's requirement. It claims `RFC7296-1.4-5`, the probe's WIRE FORM, and this row governs WHEN to probe. The spec's goal holds with it open (`ai/rules/rule-precedence.md`: name it, home it, close, then fix it). The row carries no tagged test, so `ze-rfc-check` already reports it uncovered rather than green over a violation | plan/spec-ike-dpd-demand-driven.md | deferred |

Row 3 was homed on 2026-08-07, on Thomas's answer "create new spec". Its destination is
`plan/spec-ike-dpd-demand-driven.md`, Status `skeleton`. The row STAYS live: the work is
outstanding and homing is not completion. The two neighbours the row ruled out by name are
recorded in that spec's Task section, so the reason survives the Destination cell it used
to occupy: not `fixit-ike-resource-lifetime-leaks` (nothing leaks) and not
`fixit-ike-test-discrimination` (the behavior is missing, not the test).

Row 1 closed on 2026-08-07. `test/ipsec/ipsec-dpd-holds-tunnel.ci` configures a DPD
`interval` of 1 second on the initiator, pins the responder's at 3600 so probe traffic
has one direction, and asserts three things: the probe left (`dpd: sent probe`), the
peer answered it (`dpd: peer alive`), and ONE SA stayed established for more than 20
seconds. The third assertion reads `uptime-seconds` from the SA's own `EstablishedAt`
(`saToMap`, `internal/component/ike/cmd/show_ipsec.go`), so a DPD teardown resets it and
no amount of host load can make it pass. It measures state, never elapsed time.

The test discriminates, measured both ways. Blocking the sends in `sendDPD` AND
`retransmitDPD` (`internal/component/ike/engine/dpd.go`) turns it RED at the uptime
step; restoring them turns it GREEN. Blocking only `sendDPD`'s send leaves it GREEN,
correctly: `retransmitDPD` still delivers the probe and the tunnel really is healthy.
`make ze-ipsec-test` is 14/14 green three times over with the test in place.

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
