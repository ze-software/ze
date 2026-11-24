# A requirement proven in one state of many

An RFC obligation holds in every state of an automaton, but the tests that
carry its tag drive only the state where the feature is usually exercised. The
transition table gives the same action in the other states, and nothing would
go red if one of those cells lost it.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-31 | spec-ppp-l2tp-aaa-one-off-defects (found, not caused) | `LCPDoTransition` RTR and RUC cells (`l2tp/ppp/ppp_fsm.go`) | RFC 1661 §5.5 owes sta in every state, but `RFC1661-5.5-1` tests drove Opened only, leaving Stopped, Closing, Stopping, Ack-Rcvd, Ack-Sent and Closing/Stopping scj unasserted | fixed: `TestRFC1661TerminateAckAcrossAllStates` and `TestRFC1661CodeRejectAcrossAllStates` (`rfc1661_rtr_ruc_states_test.go`) drive `handleLCPPacket` across all 8 states with `RFC1661-5.5-1`/`5.6-1` records |
| 2026-10-31 | spec-rfc4271-med-ibgp-readvertisement | eBGP MED on originated routes (`reactor/`) | `med-locally-set-reaches-peer.ci` drives only `sendInitialRoutes` -> `buildRIBRouteUpdate`; MED strips in `writeAnnounceUpdate`, `BuildUnicast` and `planBatchAttrs` stayed green | fixed: post-Established negatives in `rfc4271_originated_med_established_test.go` with per-builder discrimination records (`writeAnnounceUpdate`, `BuildUnicast`, `planBatchAttrs`) |
| 2026-11-15 | - | VRRP `backupAdvert` (`internal/plugins/vrrp/fsm/fsm.go`) | RFC 9568 Section 5.1: "The exception is that the router that owns the IPvX address associated with the Virtual Router always preempts, independent of the setting of this flag." Initialize honours `isOwner`; Backup takes the `!i.cfg.Preempt` branch without it, and `backupConfigUpdated` does not promote, so a reload that makes a Backup the owner leaves it Backup behind a lower-priority Active. Read, not run | open: the owner preempts from Backup whatever Preempt says, tested in each state |
