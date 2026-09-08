# Test weakenings this commit accepts

Six rows, and five of them are the same shape: a helper that gained a named
variant. `newCA`, `newLeaf`, `newCRL` and `driveEAPTLSFlight` each became a
one-line call into a new `*ValidFor` or `*Tuned` function that carries every
assertion the original held, so the counter reads 3 -> 0 in the wrapper and
sees nothing of the function now holding them. No assertion left the suite,
and each new variant is exercised by the resumption tests this commit adds.

The variants exist because RFC 9190 Section 5.7 bounds ticket storage at
604800 seconds, and proving that bound means moving a clock a week forward.
`tls.Config.Time` is what `checkForResumption` judges a ticket's age and its
carried certificate's expiry against, so the certificates have to outlive the
moved clock or the test would be measuring expiry instead of the ceiling.

The sixth row is the OWNER RULING of 2026-09-05, recorded in
`plan/spec-ipsec-rfc9190.md`: the deleted test pinned a non-conformance.

| Test | Reason |
|------|--------|
| newCA | Delegates to `newCAValidFor`, which holds all three assertions unchanged. The wrapper keeps the one-hour window every existing caller had, so no caller's behavior moves. |
| newLeaf | Delegates to `newLeafValidFor`, which holds all three assertions unchanged. Same one-hour default. |
| newCRL | Delegates to `newCRLValidFor`, which holds its assertion unchanged. The named window is the list's `nextUpdate`, so a list stays current for a test whose certificates outlive an hour. |
| driveEAPTLSFlight | Delegates to `driveTunedEAPTLSFlight`, which holds all three assertions unchanged and adds one hook into the authenticator's own `tls.Config` copy. The hook is the only way a test can reach that clock; no production default moves. |
| readPeerPlaintext | Two assertions to one, and the lost one is now unreachable rather than dropped. It read the peer's `tls.Conn` directly and failed on a decrypt error. The peer's own reader now consumes that record first (`consumePostHandshakeRecords`, `peer.go`), which is what lets crypto/tls process the RFC 9190 Section 2.1.2 NewSessionTicket in the same flight, so a second `Read` here would park forever on an empty transport. The function reads `PeerSession.indication` instead. A peer that failed to decrypt leaves that nil, and the remaining `t.Fatal` still fires, so an authenticator that sent no record or a wrong one still fails the caller's assertion. |
| TestEAPTLSIssuesNoUnredeemableSessionTicket | Deleted, and rewritten as `TestEAPTLS13TicketAndIndicationShareOneEAPRequest` in the same file. It asserted `SessionTicketsDisabled` was set, which the owner ruling of 2026-09-05 makes a pinned non-conformance against the RFC 9190 Section 2.1.2 MUST this commit implements. The spec says it is rewritten when resumption lands rather than defended. It carried no `RFC requirement:` tag, so no tagged proof leaves the ledger with it. The replacement asserts RFC 9190 Figure 2: the ticket and the 0x00 protected success indication leave in ONE EAP-Request. |
