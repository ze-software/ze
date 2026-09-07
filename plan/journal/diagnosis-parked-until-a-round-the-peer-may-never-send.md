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
| 2026-09-05 | ipsec-rfc9190 | EAP-TLS authenticator refusal path | `tlsMethod.Process` (`internal/core/eap/eap_tls.go`) parks the certificate failure on `m.alertSent` and returns the fatal TLS alert as an EAP-Request, so the cause reaches `Session.err` only on the round that emits EAP-Failure. `handleResponderEAPRound` (`internal/component/ike/engine/responder_eap.go`) logs `sess.Err()` under `next.Code == eap.CodeFailure` alone, so nothing is logged until that round. RFC 5216 Section 2.1.3 makes the server "wait for the peer to reply", and a peer that abandons the exchange after the alert never replies. MEASURED in `test/interop-ipsec/scenarios/responder-eap-tls13-revoked-client`: ze refuses a revoked client certificate, charon logs `received fatal TLS alert 'bad certificate'` then `EAP_TLS method failed` and sends AUTH_FAILED, and ze's whole account of the refusal is one line, `ike: responder handshake timed out, tearing down`. The sentence naming the certificate, its serial number, the CA that withdrew it and RFC 9190 Section 5.4 is never written. Every EAP-TLS refusal reaches this, not a revocation one alone | not fixed here. `MethodResult.FinalRequest` (`internal/core/eap/eap.go`) is the field built for exactly this, and its own doc comment names the EAP-TLS alert as the defect it exists for, but EAP-TLS was never moved onto it: `stateLastWord` already gives the RFC 5216 two-round behaviour while `finalRequest` sets `Session.err` on the FIRST round. The move touches the RFC-tagged refusal path in `eap_tls_alert_flight_test.go` and owes fresh discrimination records for every tagged unit it changes, which is its own package of work. The interop scenario asserts charon's account of ze's wire output instead, which is the stronger evidence anyway |
