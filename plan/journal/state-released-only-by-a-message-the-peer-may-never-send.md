# A state is released only by a message the peer may never send

A protocol state is entered on a local event and left only when the peer sends
one named message. The RFC gives a second way out, a local timer, and the code
implements the message alone. A peer that never sends it holds the state, and
whatever the state retains, for as long as the session lasts.

Ask, of every state a peer's message ends: what ends it when that message never
comes, and does the RFC name a bound?

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-22 | - | `onEORReceived`, `handleLLSTExpired` (`internal/component/bgp/plugins/gr/gr_state.go`) | Read, not run: after a session re-establishes, a family leaves `staleFamilies` or `llstExpiredDuringSync` only on End-of-RIB. RFC 9494 Section 4.2 also ends it when "the Selection_Deferral_Timer discussed in [RFC4724] expires", so a peer that sends no End-of-RIB keeps its stale routes and its helper state with no bound | open: a local helper bound, designed in spec-gr-advanced (AC-7); FRR and BIRD differ, so its choice is that spec's Q-1 |
