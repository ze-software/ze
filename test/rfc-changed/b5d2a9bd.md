# RFC-tagged test changes this session's commits carry

The owner's approval for changing a test that carries an `RFC requirement:` tag.
One row per test. An author MUST NOT write a row for their own change: the row
records Thomas's decision, and an author writing it is a forgery rather than a
shortcut (`docs/contributing/rfc-implementation-guide.md`).

## The approval

Thomas approved these six on 2026-09-08. He was shown the table of six test
names and files, the statement that every one of them adds the single line
`Resumption: NewResumption(time.Now, true)` to a `MethodConfig` literal because
the struct gained a required field, and the statement that no requirement id,
tag comment, assertion, byte offset or quoted requirement moves in any of them.
He answered: "ok".

The six share one cause. `MethodConfig` (`internal/core/eap/eap.go`) gained a
`Resumption` field in this commit, and an EAP-TLS authenticator is refused
without one, so every existing test that builds that struct had to name it. The
value each of them passes, `NewResumption(time.Now, true)`, is a live store on
the real clock, which is what these tests would get from the engine in
production (`resumptionFor`, `internal/component/ike/engine/resumption.go`).

Nothing else in any of the six moved. Each diff is one added line inside a
struct literal, plus the `time` import that line needs.

| Test | Reason |
|------|--------|
| TestRFC3748KeyDerivingMethodAuthenticatesBothEnds | `rfc3748_walk_test.go`. One line added to the rogue authenticator's `MethodConfig`. The test still runs a full EAP-TLS handshake against an untrusted chain and still asserts both ends refuse it. |
| TestRFC5216PeerRepliesBeforeItTerminates | `rfc5216_peer_wait_test.go`. One line added to the authenticator's `MethodConfig`. The RFC 5216 Section 2.1.3 reply-before-terminate assertion is untouched. |
| TestRFC5216ServerRepliesEAPFailureToPeerAlert | `rfc5216_termination_test.go`. One line added to the authenticator's `MethodConfig`. The EAP-Failure-for-peer-alert assertion is untouched. |
| TestEAPTLSPeerRejectsUntrustedServerChain | `eap_tls_handshake_test.go`. One line added to the authenticator's `MethodConfig`. The peer still rejects a chain outside its trust anchor. |
| TestEAPTLSPeerWithoutCARefusesToStart | `eap_tls_handshake_test.go`. One line added to the authenticator's `MethodConfig`. The peer still refuses to start with no configured trust anchor, which is the RFC 5216 Section 5.3 path-validation guard. |
| TestRFC5216PeerSendsItsAlertRatherThanTheNoDataResponse | `rfc5216_success_flight_test.go`. One line added to the authenticator's `MethodConfig`. The test still asserts the peer sends its TLS alert in place of the no-data response. |
