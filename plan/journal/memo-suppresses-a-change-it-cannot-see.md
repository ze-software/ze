# A suppression memo with no invalidation hides the change it compresses

A producer keeps a memo of what it has already sent and drops a later message
that matches. The compression is correct while every message restates the same
state. It stops being correct the moment a message CONTRADICTS an earlier one,
because the memo has no way to learn that the state moved: the next message that
restores the earlier state is identical to the one already in the memo, so the
consumer never hears about it.

The tell is a memo keyed on the message rather than on the state, cleared only
by a lifecycle event, and a message set that carries both an assertion and its
retraction.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-30 | rfcgate-6-supported-extraction-signoff | bmp sender route monitoring, `handleSenderUpdate` in `internal/component/bgp/plugins/bmp/bmp_events.go` | The handler hashes each UPDATE body with FNV-1a and keeps the sum in `bp.dedupState` for that peer, then drops any later UPDATE whose body hashes the same. The memo is cleared on one event only, the BGP session going down (`handleStructuredEvent`, `rpc.SessionStateDown`), and never by a message that contradicts an earlier one. So announce P, withdraw P, re-announce P with identical bytes leaves the collector holding the withdrawal, because the third UPDATE carries a state change and is suppressed as a duplicate. RFC 7854 Section 4.6 permits the compression, "Route monitoring messages are state-compressed", and Section 5 says ongoing monitoring is accomplished by propagating route changes, so a compressor whose memory survives a contradicting withdraw reports a route as gone while ze advertises it. Both directions carry the same memo, Adj-RIB-In and the RFC 8671 Adj-RIB-Out alike, so the monitoring station's picture of a correct network is wrong in either. Found while walking RFC 8671 Section 5.1 for its extraction sign-off | not fixed. It blocks no obligation of the sign-off that found it: Section 5.1's MUST binds the FIDELITY of what is conveyed, and every message ze sends carries the transmitted PDU verbatim, proven in `TestRFC8671AdjRIBOutConveysTheTransmittedUpdate`. Two repairs fit and the choice is not this walk's to make. Clear the peer's memo whenever an UPDATE carries a withdrawal, in the withdrawn-routes field or in MP_UNREACH_NLRI, which keeps the compression and costs an attribute walk on the sender path. Or drop the body-hash memo, which is the deliberate AC-7 behaviour another spec added and `TestBMPRiboutDedupSameUpdate` pins |
