# Validated value discarded by its caller

A validator that answers "yes, and here is the thing that matched" is only as
good as what the caller does with the second half. Drop it and the caller
re-derives the value from somewhere else, usually the first element of the list
it just searched. The check passes, the wrong value is used one line later, and
the failure looks like success. Distrust a verifier whose only output is an
error: what it learned has nowhere to go.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-12 | fixit-ike-negotiation-conformance | ike engine | `verifyAcceptedOffer` (`engine/initiator.go`) matched the peer's accepted offer against every ESP proposal ze had sent, and returned the one it agreed with. Two of its four call sites wrote `if _, err := verifyAcceptedOffer(...)` and then keyed from `Proposals[0]`. A peer that accepted the second proposal therefore passed the check, and ze installed the first proposal's cipher and key length. The tunnel establishes and black-holes ESP, which is worse than a refused negotiation because it reads as success. The responder half had been narrowing correctly for months | `acceptedOffer.ESPConfig` carries the configured proposal, resolved by `espConfigForAccepted`. `applyChildRekeyResponse` keys from it and `handleAuthResponse` narrows `sa.ESPGroup.Proposals` to it, which is what `selectResponderESP` already did on the other side. `TestNegInitiatorInstallsAcceptedESPSuite` drives a peer that accepts the second of two proposals and asserts the algorithms of the states ze programs |
| 2026-08-12 | - | bgp rpki rtr session | `handlePDU` (`internal/component/bgp/plugins/rpki/rtr_session.go`) parses the End of Data PDU's Refresh Interval and stores it in `s.refreshInterval`, which is then read only by `Snapshot`. `Run` waits `s.retryInterval` after a SUCCESSFUL sync as well as after a failure, so RFC 8210 Section 6 ("The router SHOULD NOT poll the cache sooner than indicated by [Refresh Interval]") is not honoured. Measured against StayRTR, which sends refresh 3600 and retry 600: ze re-polls every 600 seconds | not fixed. Found by the StayRTR RTR interop scenario |
