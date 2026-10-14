# A paraphrase of a requirement becomes the authority

Somebody states a requirement. A spec, a comment or a design page keeps a summary
of it instead of the words. The words are then lost, and every later artifact is
derived from the summary: the code, the tests, the documentation and the next
spec.

What makes the class expensive is that nothing inside the tree can detect it.
Each derived artifact agrees with the others, so the suite is as green under the
wrong requirement as under the right one, and a reviewer who checks everything
finds nothing. Only the original text can arbitrate, and the class exists exactly
where that text was never written down.

The repair is always the same: quote the requirement verbatim on one durable
page, date it, and make every other artifact point at that page rather than
restate it.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-11 | config-apply-ordering-covers-every-root | `docs/architecture/config/apply-ordering.md` and the config transaction ordering it describes, `internal/component/config/transaction` | The owner stated the config apply order in May 2026. The spec kept a paraphrase and his words were not recorded. The paraphrase said make-before-break, add the new address and remove the old one last; he had asked for stop the binder, remove, add, restart. The subsystem, its unit tests, its two functional tests, its comments and its design page were all built from the paraphrase, so all five agreed and no test could tell the policies apart: nothing ever read the order a commit APPLIES. Four months. Found when the owner read the design page and quoted himself back | FIXED. The page gained a "The requirement" section carrying his words unedited with their date, and it is declared to outrank the spec wherever they disagree. Make-before-break is deleted rather than inverted: `Params.AllowDual`, `markDualPresence`, `tryRelaxCycle`, `isAddressOperation` and one constraint rule are gone, `operationPhase` (`solver.go`) sorts the five phases the owner named, and `test/reload/config-apply-ordering-address-swap.ci` reads the applied order off a kernel and reddens under a revert of each half. A rule requiring an owner requirement to be recorded verbatim is proposed to the owner in `plan/learned/020-quote-the-requirement-do-not-summarize-it.md` |
| 2026-09-27 | spec-rfc-requirement-quote-hand-backfill | ospf auth replay | RFC 7474 §2's strictly-greater sequence rule, defined for its AuType 3, was applied to AuType 2, where RFC 2328 D.4.3 accepts an equal sequence number | fixed (DF-OSPF-B): replayedSequence branches on AuType |
| 2026-10-13 | ldp-keepalive-expiry-notification (design) | `handleInit` and `NewSession` (`internal/plugins/ldp/session.go`), sizing `holdTime`; `docs/guide/mpls.md` "closes the session when no PDU arrives inside three times that value" | KeepAlive expiry fires at 3x the negotiated KeepAlive Time ("hold time is typically 3x keepalive", a BGP habit); RFC 5036 3.5.3 makes KeepAlive Time itself the maximum gap between PDUs. Guide and RFC5036-3.5.3-1 test pin the 3x | Fixed 2026-10-14 (spec phase 4): `handleInit` arms the negotiated time |
| 2026-10-13 | spec-bgp-rs-replacement-on-withdrawal (design) | `recoverySentNLRI` comment (`internal/component/bgp/plugins/rib/rib_recovery.go`) | Quotes RFC 8277 Section 2.4 as "The value of the Compatibility field SHOULD be set to 0x800000."; `rfc/full/rfc8277.txt` line 642 reads "Upon transmission, the Compatibility field SHOULD be set to 0x800000." A paraphrase inside quotation marks, which the verbatim-quote rule refuses | open: replace with the RFC sentence |
