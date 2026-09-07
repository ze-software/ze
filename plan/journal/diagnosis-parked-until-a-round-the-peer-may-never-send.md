# A diagnosis is parked until a round the peer may never send

A protocol refuses a peer, computes a precise reason, and parks that reason on a
field read only by the NEXT round. The counterparty decides whether that round
happens. When it abandons the exchange instead, the reason is dropped and the
operator reads a timeout.

The two-round shape is usually correct, and often an RFC asks for it. The defect
is that the diagnosis rides on the second round rather than being published when
it is computed. Ask, of every parked cause: who decides whether the code that
reads this field runs, and is it the party the refusal is about?

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-05 | ipsec-rfc9190 | EAP-TLS authenticator refusal path | `tlsMethod.Process` (`internal/core/eap/eap_tls.go`) parks the certificate failure on `m.alertSent` and returns the fatal TLS alert as an EAP-Request, so the cause reaches `Session.err` only on the round that emits EAP-Failure. `handleResponderEAPRound` (`internal/component/ike/engine/responder_eap.go`) logs `sess.Err()` under `next.Code == eap.CodeFailure` alone, so nothing is logged until that round. RFC 5216 Section 2.1.3 makes the server "wait for the peer to reply", and a peer that abandons the exchange after the alert never replies. MEASURED in `test/interop-ipsec/scenarios/responder-eap-tls13-revoked-client`: ze refuses a revoked client certificate, charon logs `received fatal TLS alert 'bad certificate'` then `EAP_TLS method failed` and sends AUTH_FAILED, and ze's whole account of the refusal is one line, `ike: responder handshake timed out, tearing down`. The sentence naming the certificate, its serial number, the CA that withdrew it and RFC 9190 Section 5.4 is never written. Every EAP-TLS refusal reaches this, not a revocation one alone | FIXED 2026-09-07. `tlsMethod.Process` returns the alert as `MethodResult.FinalRequest` beside its `Err`, so `Session.finalRequest` writes the cause to `Session.err` as the alert goes out and `stateLastWord` still spends the second round RFC 5216 Section 2.1.3 asks for. `m.alertSent` and both of its read sites are gone. `handleResponderEAP` reads `sess.Err()` before and after `Process` and writes `ike: EAP authentication failed` on the round the cause first appears, which is one line per refusal whether or not the peer answers. Proven by `TestEAPTLS13RecordsTheRevocationWhenTheRefusedPeerWalksAway` (`internal/core/eap`), `TestEAPRefusalIsReportedBeforeTheEAPFailureRound` and `TestEAPRefusalIsReportedOnceWhenThePeerAnswersTheLastWord` (`internal/component/ike/engine`), each forced red against the unfixed code, and asserted against strongSwan by assertions 11 and 13 of `checkResponderEAPTLS13RevokedClient` |
