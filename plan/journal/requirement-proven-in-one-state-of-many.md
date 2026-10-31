# A requirement proven in one state of many

An RFC obligation holds in every state of an automaton, but the tests that
carry its tag drive only the state where the feature is usually exercised. The
transition table gives the same action in the other states, and nothing would
go red if one of those cells lost it.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-31 | spec-ppp-l2tp-aaa-one-off-defects (found, not caused) | `LCPDoTransition` RTR and RUC cells (`internal/component/l2tp/ppp/ppp_fsm.go`) | Read, not run: RFC 1661 Section 5.5 owes sta in every state, but the RFC1661-5.5-1 tests drive Opened only; Stopped, Closing, Stopping, Ack-Rcvd and Ack-Sent are unasserted, as is scj in Closing and Stopping | open: a tagged table test through `handleLCPPacket` over every state, with discrimination records |
