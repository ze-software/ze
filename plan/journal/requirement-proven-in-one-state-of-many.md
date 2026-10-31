# A requirement proven in one state of many

An RFC obligation holds in every state of an automaton, but the tests that
carry its tag drive only the state where the feature is usually exercised. The
transition table gives the same action in the other states, and nothing would
go red if one of those cells lost it.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-31 | spec-ppp-l2tp-aaa-one-off-defects (found, not caused) | `LCPDoTransition` RTR and RUC cells (`internal/component/l2tp/ppp/ppp_fsm.go`) | Read, not run: RFC 1661 Section 5.5 owes sta in every state, but the RFC1661-5.5-1 tests drive Opened only; Stopped, Closing, Stopping, Ack-Rcvd and Ack-Sent are unasserted, as is scj in Closing and Stopping | open: a tagged table test through `handleLCPPacket` over every state, with discrimination records |
| 2026-10-31 | - (RFC audit 245992d954) | eBGP MED on originated routes (`reactor/`) | The RFC4271-5.1.4-1 negative `med-locally-set-reaches-peer.ci` drives only `Peer.sendInitialRoutes` -> `buildRIBRouteUpdate`; MED strips in `writeAnnounceUpdate`, `BuildUnicast` and `planBatchAttrs` stayed green, so an announce to an Established peer is unproven | open: a tagged negative announcing after Established, with a discrimination record per builder |
