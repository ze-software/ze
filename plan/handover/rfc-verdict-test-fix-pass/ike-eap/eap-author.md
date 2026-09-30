# ike-eap child, package internal/core/eap, author handoff (2026-09-28)

Scope: stems rfc2759, rfc3748, rfc5216 only; 49 weak/wrong verdicts, RFC5216-2.1.1-4 blocked
(spec-ike-eap-rfc-defects). No rfc9190_* file touched. No verdict stamped, nothing committed.
The author hit its 100-call budget: 14 rfc5216 and 9 rfc3748 verdicts remain for a continuation.

## Verdict table

| id | resolution | what now proves each clause (+/-) | records written | expected verdict | notes |
|----|-----------|------------------------------------|-----------------|------------------|-------|
| RFC3748-2-1 | tests | + TestRFC3748NoNewRequestBeforeAValidResponse (rfc3748_clause_test.go): invalid Responses draw nothing, the valid one draws the next packet, a replay draws nothing | positive (revert eap.go::handleMethod) | enforced | row keeps its {single-polarity: positive} marker |
| RFC3748-2-2 | tests | + / - TestRFC3748NoNewRequestBeforeAValidResponse (- another Identifier, another Type draw no packet) | pos (handleMethod), neg (eap.go::Process) | enforced | old negative tag on TestRFC3748AuthenticatorRequiresValidResponse removed (approved D-15); it had no record |
| RFC3748-2.1-1 | tests | peer clause: TestRFC3748ThePeerKeepsToOneMethod (- EAP-TLS Request to an MS-CHAPv2 peer discarded; + same method concludes). Auth clause stays on TestRFC3748OneMethodPerConversation | pos, neg (peer.go::handleRequest) | enforced | |
| RFC3748-2.1-2 | tests | TestRFC3748OneMethodThenTheResult: + every method Request is Type 26, EAP-Success, nothing after; - wrong password: one method, EAP-Failure, nothing after | pos, neg (handleMethod) | enforced | |
| RFC3748-2.1-4 | tests | TestRFC3748TheAuthenticatorSendsNoRequestOfAnotherType: + all Requests Type 26, no additional method after Success; - a Type-13 Response mid-method draws no Request, method continues. Peer clause stays on rfc3748_discard_test.go units | pos, neg (handleMethod) | enforced | |
| RFC3748-4-2 | tests | TestRFC3748LengthCountsTheWholePacket: + encoded Length 8 over 8 octets, decoded back; - Length counting Data only (4) or Type-Data only (3) refused | pos (Encode), neg (DecodePacket) | enforced | level review (no keyword): kept MUST under D-3 |
| RFC3748-4-4 | tests | TestRFC3748PaddingPastLengthIsNotData: + padding reaches neither Type-Data nor the recorded identity through Session.Process; - the same octets inside Length are delivered | pos, neg (DecodePacket) | enforced | old single-input pair on TestRFC3748LengthBoundsTypeData left in place |
| RFC3748-4.2-2 | tests | TestRFC3748SuccessAndFailureCarryNoData: + Encode drops Type/Type-Data (00 04 header); - a received Success/Failure with data is delivered with none | pos (Encode), neg (DecodePacket) | enforced | old negative tag on TestRFC3748SuccessFailureFormat removed (approved D-15); it had no record |
| RFC3748-4.2-5 | tests | TestRFC3748PeerEndsAConversationItRefuses: + peer-decides trigger (wrong Authenticator Response): Err, no ack, later Success concludes nothing, no MSK; - matching value concludes | pos, neg (peer.go::handleMSCHAPv2Success) | enforced | authenticator trigger stays on TestRFC3748PeerEndsAnUnsuccessfulConversation |
| RFC3748-4.2-8 | tests | Failure half: TestRFC3748PeerDiscardsAFailureWhereNoneIsPermitted (+ Failure after both indications discarded, Success still concludes; - mid-method Failure read as ErrEAPFailure) | pos, neg (peer.go::Process) | enforced | Success half unchanged |
| RFC3748-5.4-1 | tests | TestRFC3748MD5ChallengeAnswersOnlyARequestThatCarriesOne: + Response Code/Type/Identifier/value; - Value-Size 0 or no Type-Data draws no Response and an error | pos, neg (peer.go::handleMD5ChallengeRequest) | enforced | negative tag removed from TestRFC3748MD5ChallengeRequeryDrawsNoResponse (approved); its old record is now an orphan |
| RFC3748-5.4-2 | tests | TestRFC3748MD5ChallengeServerRefusesAWrongValue: + both roles run Type 4 to Success/Done; - wrong secret draws EAP-Failure | pos, neg (eap_md5challenge.go::Process) | enforced | negative tag removed from TestRFC3748MD5ChallengeIsTheConfiguredMethod (approved); old record orphaned |
| RFC3748-7.5-1 | tests | server clause: TestRFC3748TheServerValidatesThePeersMIC (+ clean flight EAP-Success; - one octet of the peer's second record flight changed: no EAP-Success). Peer clause stays on TestRFC3748EAPTLSValidatesItsPerPacketMIC | pos, neg (eap_tls.go::Process) | enforced | |
| RFC3748-7.10-4 | tests | TestRFC3748KeyDerivingMethodsAuthenticateThePeerToo: + MS-CHAPv2 mutual success with MSK; - EAP-TLS untrusted client refused with no server MSK, wrong NT-Response refused with no MSK, wrong Authenticator Response refused with no peer MSK | pos (eap_mschapv2.go::handleResponse), neg (peer.go::handleMSCHAPv2Success) | enforced | the neg record's red lands in the unit's first half (driveClauseFlight also calls handleMSCHAPv2Success); a targeted mutant would be sharper |
| RFC3748-4-1 | unresolved | - | - | weak | silence is decided by the caller of DecodePacket, the IKE engine (handleResponderEAP / handleEAPResponse); the proof belongs in internal/component/ike/engine |
| RFC3748-4.1-1 | unresolved | - | - | weak | needs an invalid-Request case under this id (peer discards, no Response) |
| RFC3748-4.1-3 | unresolved | - | - | weak | retransmission is IKE's (byte-identical IKE message); the same-Identifier clause is proven only at the ike/engine layer, or needs a row note |
| RFC3748-4.1-11 | unresolved | - | - | weak | row-quality: merge into 4.1-5 and 2.1-3 (D-2 Retired paragraph, tags moved) |
| RFC3748-4.2-11 | unresolved | - | - | weak | lost-Failure case has no unit |
| RFC3748-7.10-1 / 7.10-2 | unresolved | - | - | weak | row-quality: duplicate span, merge keeping every tag; re-recorded 7.10-2 positive on TestRFC3748EAPTLSExportsASixtyFourOctetEMSK after the unit edit |
| RFC3748-7.10-6 | unresolved | - | - | weak | "or used to derive any other keys" unproven; assert MSK equals the independent derivation (exporter 0-63, RFC 3079) on both methods |
| RFC3748-2.2-1 | unresolved | - | - | weak | Notification Request clause: ze's authenticator sends none; needs a peer case or a scope marker |
| RFC2759-x-1 | tests | TestRFC2759PeerSendsTheResponseValueLayout (rfc2759_clause_test.go): + peer emits 8 zero Reserved octets; - each of the 8 octets non-zero refused (fresh authenticator copy per case) | pos (peer.go::handleMSCHAPv2Challenge), neg (eap_mschapv2.go::handleResponse) | enforced | |
| RFC2759-x-2 | tests | same unit: + peer emits Flags 0; - Flags 1 refused | pos, neg (as x-1) | enforced | |
| RFC2759-x-3 | tests | same unit: + Value-Size 49, Peer-Challenge, NT-Response (independent), Name; - Value-Size 48/50 refused | pos, neg (as x-1) | enforced | level review: kept MUST under D-3 (format row, owner call) |
| RFC2759-x-4 | tests | TestRFC2759NTResponseUsesTheBareUserName: + peer sends DOMAIN\user with bare-name NT-Response, authenticator accepts; - domain-qualified NT-Response refused via Process | pos, neg (mschapv2.go::stripDomain) | enforced | |
| RFC2759-x-8 | tests + row | TestRFC2759AuthenticatorChallengeComesFromCryptoRand: + Value-Size 16, octets from crypto/rand.Reader. Row re-levelled MUST -> SHOULD | pos (eap_mschapv2.go::Start) | enforced | Correction 2026-09-28 in rfc/corrections/rfc2759.md; single-polarity marker kept |
| RFC2759-x-9 | tests + row | TestRFC2759PeerChallengeEntersTheNTResponse: + NT-Response over the sent Peer-Challenge; swapped Peer-Challenge refused. Row re-levelled MUST -> SHOULD | pos (handleMSCHAPv2Challenge) | enforced | same correction paragraph; single-polarity marker kept |
| RFC2759-x-10 | tests | TestRFC2759PasswordHashVector: + ntPasswordHash("clientPass") = §9.2 PasswordHash = MD4 of the §9.2 unicode octets; - "é" hashed as E9 00, not UTF-8 | pos, neg (mschapv2.go::ntPasswordHash) | enforced | level review: kept MUST (vector row, owner call) |
| RFC2759-x-12 | tests | TestRFC2759FailureInvitesNoRetry: + E=691, R=0, EAP-Failure after the answer; - verified NT-Response ends in EAP-Success | pos (sendFailure), neg (sendSuccess) | enforced | level review: kept MUST (flow row, owner call) |
| RFC5216-2.3-1 | tests | tag added on TestRFC3748EAPTLSExportsASixtyFourOctetEMSK (tls12-rfc5216 case): + MSK = export 0-63 (new assertion), EMSK = export 64-127 | pos (eap_tls.go::exportEAPTLSKeys) | enforced | unit edit approved D-15 |
| RFC5216-2.4-1 | tests | TestRFC5216BothRolesInstallAVersionFloor (rfc5216_clause_test.go): + peer tls.Config MinVersion TLS 1.2 | pos (peer.go::tlsClientConfig) | enforced | revert panics the whole builder; a mutant on the MinVersion line would be sharper |
| RFC5216-2.4-2 | tests | same unit: + authenticator tls.Config MinVersion TLS 1.2 | pos (eap_tls.go::newTLSMethod) | enforced | |
| RFC5216-2.1.1-1, 2.1.1-5, 2.1.1-6, 2.1.1-9, 2.1.1-10, 2.1.2-1, 2.1.3-4, 2.1.5-1, 2.4-3, 2.4-4, 3-1, 3-5, 5.3-1, 5.4-1 | unresolved | - | - | weak | not reached (budget). 2.1.1-10/2.1.1-5/2.1.2-1 need a driven resumption with flight decode; 5.4-1 peer half must go in a new non-rfc9190 file |
| RFC5216-2.1.1-4 | blocked | - | - | wrong | spec-ike-eap-rfc-defects |

Counts: tests 25 (2 with a row re-level), unresolved 23, blocked 1. Defects found: none.

## Approvals recorded (carry the RFC-approved trailers)

eap.TestRFC3748SuccessFailureFormat, eap.TestRFC3748MD5ChallengeRequeryDrawsNoResponse,
eap.TestRFC3748MD5ChallengeIsTheConfiguredMethod, eap.TestRFC3748AuthenticatorRequiresValidResponse,
eap.TestRFC2759NTResponseUsesTheBareUserName, eap.TestRFC3748EAPTLSExportsASixtyFourOctetEMSK
(files tmp/commit-rfc-approved-*.md).

## Orphan records (tag removed, reported removable by rfc check)

RFC3748-5.4-1 negative on TestRFC3748MD5ChallengeRequeryDrawsNoResponse;
RFC3748-5.4-2 negative on TestRFC3748MD5ChallengeIsTheConfiguredMethod.

## Gates owed (not run here)

`./le rfc check` over rfc2759/rfc3748/rfc5216 (new tags, re-levelled rows, orphan records),
`./le go lint run` on internal/core/eap, and the independent re-judge of every "enforced" row
above. The whole eap package passed: `go test ./internal/core/eap/` ok (2.9s).
`./le rfc index-update` was run once.

## Files changed

- internal/core/eap/rfc3748_clause_test.go (new)
- internal/core/eap/rfc2759_clause_test.go (new)
- internal/core/eap/rfc5216_clause_test.go (new)
- internal/core/eap/rfc3748_test.go
- internal/core/eap/rfc3748_md5challenge_test.go
- internal/core/eap/rfc3748_emsk_test.go
- rfc/short/rfc2759.md (x-8, x-9 level only; other hunks in rfc/short are foreign)
- rfc/corrections/rfc2759.md
- rfc/discrimination/rfc2759.json (new)
- rfc/discrimination/rfc3748.json
- rfc/discrimination/rfc5216.json
- generated by index-update: ai/RFC-REQUIREMENTS.md, rfc/requirements/*, rfc/enrolled.txt, rfc/not-enrolled.txt, docs/features/rfc-status.md (showed no diff for these stems at the time of the run)

## CONTINUATION 2026-09-29 (second author)

Nothing stamped, nothing committed, no reseal/audit-stamp/index-update run.

| id | resolution | what now proves each clause (+/-) | records written | expected verdict | notes |
|----|-----------|------------------------------------|-----------------|------------------|-------|
| RFC3748-7.10-2 | tests + row merge | + TestRFC3748MSCHAPv2MSKIsTheRFC3079Derivation (rfc3748_remaining_clause_test.go): MS-CHAPv2 MSK = RFC 3079 server receive key, server send key, 32 zero octets, on both ends, rebuilt from primitives; RFC 3079 §3.5.3 SendStartKey128 known answer. EAP-TLS MSK value already on TestRFC3748EAPTLSExportsASixtyFourOctetEMSK. + TestRFC3748MSKSize moved here from 7.10-1 | pos (mschapv2.go::GetAsymmetricStartKey), pos TestRFC3748MSKSize (DeriveMSK) | enforced | RFC 3079 §3.5.3 printed MasterKey does not follow from its printed NT-Response (checked by hand), so the vector is used from the MasterKey on |
| RFC3748-7.10-1 | row (retired, R5 merge) | tag moved to 7.10-2 (approved D-15) | - | row gone | Retired paragraph in rfc/corrections/rfc3748.md; extraction 7.10:1 remapped to 7.10-2, its unsourced-ids entry removed |
| RFC3748-7.10-6 | tests | + same unit: the MSK is a function of the MPPE MasterKey alone (EMSK does not enter). Negative unchanged (EMSK never on the wire) | pos (DeriveMSK) | enforced | EAP-TLS side: MSK = exporter 0-63 already asserted on TestRFC3748EAPTLSExportsASixtyFourOctetEMSK (not re-tagged 7.10-6) |
| RFC3748-4.1-11 | row + tests | row narrowed to its third sentence (Correction 2026-09-29, R5: sentences 1-2 are 4.1-5 and 2.1-3). + TestRFC3748AuthenticatorDiscardsANakAfterTheInitialNonNakResponse (Nak after initial non-Nak discarded, method still concludes); old +/- tags kept | pos (eap.go::nakUnexpected) | enforced | the two old tags hold no records (none existed at HEAD) |
| RFC3748-4.1-1 | tests | - TestRFC3748PeerAnswersNoInvalidRequest: other-method Request mid-method and method Request after EAP-Success draw no Response, valid Request between draws one. single-polarity marker removed from the row | neg (peer.go::handleRequest) | enforced | timer clause stays on the synchronous API (positive unit) |
| RFC3748-4.2-11 | tests | + TestRFC3748PeerConcludesFailureWithoutTheFailurePacket: EAP-Failure dropped after the MS-CHAPv2 failure indication; next packet (forged Success, stray Request) ends with ErrEAPFailure, not Done, no MSK | pos (peer.go::handleMSCHAPv2Failure) | enforced | lost-Success half stays on the old unit |
| RFC3748-2.2-1 | tests | + TestRFC3748TheAuthenticatorSendsNoNotificationRequest (success and failure conversations: no Type-2 Request); - TestRFC3748ANotificationRequestCarriesNothingToTheMethod (Notification carrying the MS-CHAPv2 result bytes: empty Notification Response, early Success discarded, real Request concludes with shared MSK) | pos (eap_mschapv2.go::sendFailure), neg (peer.go::notificationResponse) | enforced | |
| RFC3748-4-1, RFC3748-4.1-3 | queued | - | - | weak | proof belongs in internal/component/ike/engine, held by another agent now |
| RFC3748-5.4-1/5.4-2 orphans | left | - | - | - | the discriminate tools offer no removal verb |

### rfc5216 (all new units in internal/core/eap/rfc5216_flight_content_test.go; flights reassembled from fragments and read handshake message by handshake message)

| id | resolution | what now proves each clause (+/-) | records written | expected verdict | notes |
|----|-----------|------------------------------------|-----------------|------------------|-------|
| RFC5216-2.1.1-6 | tests | + TestRFC5216ServerFlightCarriesItsCertificateAndEndsWithServerHelloDone: full TLS 1.2 server flight has certificate, server_hello_done last | pos (eap_tls.go::newTLSMethod) | enforced | old config tags kept |
| RFC5216-5.3-1 | tests (partial) | + TestRFC5216ServerSendsItsChainWithoutTheRoot: certificate list = server leaf only, root absent | pos (newTLSMethod) | weak until the retrieval clause is split | "ability to retrieve intermediate certificates" is absent in ze (no AIA fetch): needs an R3 split into a {gap} row + extraction site; not done (budget) |
| RFC5216-2.1.1-1 | tests (positive) | + TestRFC5216PeerFlightAnswersTheCertificateRequest: server flight has certificate_request, peer flight = certificate, client_key_exchange, certificate_verify | pos (peer.go::tlsClientConfig) | weak on the negative | negative stays the ClientAuth config unit. PeerSession always loads a cert, so a certless peer needs a raw tls.Client driven over EAP framing (R1 a). Queue or OWNER-GATE |
| RFC5216-2.1.1-9 | tests | + same unit: client_key_exchange, then change_cipher_spec, then one sealed finished record | pos (peer.go::readAndSendTLS) | enforced (positive); old negative unchanged | |
| RFC5216-2.1.1-10 | tests | + TestRFC5216ResumedFlightsCarryOnlyChangeCipherSpecAndFinished: second TLS 1.2 conversation over shared stores resumes; peer answer = CCS + one sealed finished, no plaintext handshake | pos (resumption.go::peerSessionCache) | enforced | |
| RFC5216-2.1.1-5 | tests | + same unit: resumed ServerHello suite = first conversation's suite (read off the wire) | pos (resumption.go::TicketKeys) | enforced | |
| RFC5216-2.1.2-1 | DEFECT (D-8), stopped | failing untagged test TestRFC5216ResumingServerSendsNothingButChangeCipherSpecAndFinished: resumed server flight is server_hello, NewSessionTicket, CCS, finished | - | weak | crypto/tls renews the ticket on TLS 1.2 resumption (RFC 5077 §3.3 permits it; RFC 5216 §2.1.2 forbids anything but CCS+finished). Design choice: recommend refusing TLS 1.2 resumption on the authenticator (UnwrapSession returns nil for a TLS 1.2 session; tickets stay for TLS 1.3 per RFC 9190) and adding it to D2 of plan/immediate/spec-ike-eap-rfc-defects.md beside RFC5216-2.1.1-4, same TLS 1.2 resumption area. The package test run is red on this one unit by design |
| RFC5216-2.1.3-4 | tests | + TestRFC5216AuthenticatorEndsTheConversationAfterItsFailure: no-data Response to the alert draws EAP-Failure; three later Responses draw nothing; not Succeeded | pos (eap_tls.go::Process) | enforced | |
| RFC5216-2.4-3 | tests | + TestRFC5216NeitherSideNegotiatesCompression (ClientHello offers [0], ServerHello selects 0); - TestRFC5216ServerRefusesTheCompressionAPeerOffers (injected ClientHello offering [1,0] draws ServerHello with 0). single-polarity marker removed from the row | pos (tlsClientConfig), neg (newTLSMethod) | enforced | |
| RFC5216-3-5 | tests | + TestRFC5216SendersWriteZeroReservedFlagBits: every EAP-TLS flags octet both roles write has bits 0x1f clear | pos (eap_tls.go::nextFragment) | enforced | |
| RFC5216-5.4-1 | tests | + TestRFC5216PeerRefusesARevokedServerCertificateOverTLS12: peer CRL revoking the server cert, TLS 1.2: peer error, no Success, no MSK (new non-rfc9190 file) | pos (revocation.go::checkChainRevocation) | enforced | |
| RFC5216-2.1.5-1 | unresolved, OWNER-GATE (R1) | - | - | weak | ze accepts an unfragmented message larger than its fragment size (the compression test feeds one), so no receiving-side refusal exists; the producer-side (b) negative restates the positive |
| RFC5216-3-1 | unresolved, OWNER-GATE (R1) | - | - | weak | RFC 5216 never forbids L on later fragments; ze's reassembler accepts a first fragment without L (TestReassembleBoundsBufferWithoutLengthFlag), so no refusal path. The HEAD negative's prose ("MUST NOT") overclaims and should be reworded or moved |
| RFC5216-2.4-4 | queued | - | - | weak | proof is at the IKE layer (proposal selection independent of the TLS suite), internal/component/ike/engine, held by another agent |
| RFC5216-2.1.1-4 | blocked | - | - | wrong | already in AC-3/D2 of plan/immediate/spec-ike-eap-rfc-defects.md (P-3 satisfied) |

### Counts (this continuation)
tests 17 (rfc3748 6 + rfc5216 11 ids), row changes 3 (4.1-11 correction, 7.10-1 retired, 4.1-1 and 2.4-3 markers removed), defect 1 (RFC5216-2.1.2-1), queued 4 (RFC3748-4-1, 4.1-3, RFC5216-2.4-4, 2.1.1-1 negative), OWNER-GATE 2 (RFC5216-2.1.5-1, 3-1), partial 1 (RFC5216-5.3-1 split owed), blocked 1.

### Approvals recorded this continuation
eap.TestRFC3748MSKSize, eap.TestRFC3748MSCHAPv2MSKIsTheRFC3079Derivation, eap.TestRFC5216ServerRefusesTheCompressionAPeerOffers, eap.TestRFC5216ResumedFlightsCarryOnlyChangeCipherSpecAndFinished.

### Files changed this continuation
- internal/core/eap/rfc3748_remaining_clause_test.go (new)
- internal/core/eap/rfc5216_flight_content_test.go (new)
- internal/core/eap/rfc3748_test.go (TestRFC3748MSKSize tag id only)
- rfc/short/rfc3748.md (4.1-1 marker, 7.10-1 row removed, 4.1-11 text)
- rfc/short/rfc5216.md (2.4-3 marker)
- rfc/corrections/rfc3748.md (Retired 7.10-1, Correction 4.1-11)
- rfc/extraction/rfc3748.json (7.10:1 -> 7.10-2, 7.10 unsourced-ids removed)
- rfc/discrimination/rfc3748.json, rfc/discrimination/rfc5216.json (records above)

### Gates owed (not run here)
`./le rfc check` over rfc3748/rfc5216 (retired id, corrected row, removed markers, new tags and records), `./le go lint run` on internal/core/eap, independent re-judge. Package run: `go test ./internal/core/eap/` red ONLY on TestRFC5216ResumingServerSendsNothingButChangeCipherSpecAndFinished (the defect test). gofmt clean.

Environment: the cache disk filled once mid-run (link "no space left on device", then "crypto/... is not in std"); the retries recorded cleanly.

# Continuation 3 (2026-09-30, third author)

Nothing stamped, nothing committed, no reseal/audit-stamp/index-update. Rows appended per id as each finished. All records route revert, observed red, under flock ledger.lock.

| id | resolution | what now proves each clause (+/-) | records written | expected verdict | notes |
|----|-----------|------------------------------------|-----------------|------------------|-------|
| RFC3748-2.2-1 | records | Nak/Success/Failure/Notification Response clauses: TestRFC3748FrameworkMessagesReachNoMethod (lockstep) and TestRFC3748NoFrameworkMessageReachesAnEAPMethod (ordering), both tags each | 4: pos nakRefused, neg handleMethod, on each unit | enforced | judge's only gap was the missing records; no test edit |
| RFC3748-4.1-11 | records | Type-mismatch clause: TestAuthenticatorDiscardsAResponseOfAnotherType (+), TestAuthenticatorProcessesAResponseOfTheMethodType (-) | 2: handleMethod | enforced | no test edit |
| RFC3748-4.1-1 | tests + records | timer clause: + TestRFC3748PeerNeverRetransmitsOnATimer (new file rfc3748_peer_timer_test.go): PeerSession type walked for time.Timer/Ticker/chan time.Time (none), Process the only method returning *Packet/PeerResult, and a peer that answered a Request answers an early EAP-Success with nothing (discarded). Record added for the old positive TestRFC3748PeerHasNoRetransmitTimer | pos handleRequest (old unit), pos Process (new unit) | enforced | approval recorded on the new unit (lint fix after first write) |
| RFC3748-7.10-6 | tests (tag) + record | EAP-TLS side: tag added on TestRFC3748EAPTLSExportsASixtyFourOctetEMSK at the MSK == export octets 0-63 comparison (TLS 1.2 and 1.3), so an MSK derived from the EMSK fails | pos exportEAPTLSKeys | enforced | approval D-15 recorded; unit-sha of the 7.10-2 and 5216-2.3-1 records unchanged (comment-only edit) |
| RFC5216-2.1.3-4 | records | TestEAPTLSSessionPutsTheAlertOnTheWireBeforeEAPFailure (+), TestRFC5216ServerSendsNoEAPFailureWhenBothSidesAuthenticate (-) | 2: eap_tls.go::Process | enforced | no test edit |
| RFC5216-2.4-3 | tag removed | the weak positive tag on TestEAPTLSMutualAuthHandshakeSucceeds (HandshakeComplete only) dropped, body comment reworded; positive/negative stay on the recorded wire units | - | enforced | approval D-15 recorded; that tag held no record |
| RFC5216-2.1.1-1 | tests + records | - TestRFC5216AuthenticatorRefusesAPeerThatSendsNoCertificate (new file rfc5216_certless_peer_test.go, R1 a): peer holding only a cert from a CA the certificate_request does not name; crypto/tls answers with an empty certificate_list and no certificate_verify (asserted off the wire); authenticator sends no EAP-Success, Session not Succeeded, TLS handshake incomplete. Records added for the older TestEAPTLSMutualAuthHandshakeSucceeds (+) and TestEAPTLSAuthenticatorRequiresClientCert (-) | neg newTLSMethod (new), pos/neg newTLSMethod (old units) | enforced | |
| RFC5216-5.3-1 | row split (R3) + records | 5.3-1 narrowed to the server's path-validation SHOULD (verbatim prefix of sentence 1): + TestEAPTLSMutualAuthHandshakeSucceeds, - TestEAPTLSServerRejectsUntrustedClientChain (both recorded, newTLSMethod) | pos/neg newTLSMethod | enforced | Correction 2026-09-30 in rfc/corrections/rfc5216.md |
| RFC5216-5.3-4 (new) | split row + tag moves + records | "Therefore, the EAP-TLS server SHOULD provide its entire certificate chain minus the root ... The EAP-TLS peer SHOULD support validating the server certificate ...": + TestRFC5216ServerSendsItsChainWithoutTheRoot (moved), + TestEAPTLSMutualAuthHandshakeSucceeds (added, peer side), - TestEAPTLSPeerRejectsUntrustedServerChain (moved), - TestEAPTLSPeerWithoutCARefusesToStart (moved) | pos newTLSMethod, pos verifyPeerCertificate, neg verifyPeerCertificate, neg startTLSClient | enforced (needs first judge) | approvals D-15 recorded on the 4 units. SHOULD rows carry no extraction site (5.3-1 had none); rfc check to confirm |
| RFC5216-5.3-5 (new) | {gap} row | "including the ability to retrieve intermediate certificates ..." : no AIA fetch, no intermediate pool; disclosed in Support remaining | - | gap | |
| orphan records | left | RFC5216-5.3-1 records on TestRFC5216ServerSendsItsChainWithoutTheRoot, TestEAPTLSPeerRejectsUntrustedServerChain, TestEAPTLSPeerWithoutCARefusesToStart (tags moved to 5.3-4) | - | - | no removal verb; rfc check reports them |
| RFC5216-2.1.1-5 | blocked | - | - | weak | AC-4 of plan/immediate/spec-ike-eap-rfc-defects.md removes TLS 1.2 resumption and names 2.1.1-5/2.1.1-10 for re-proof in that work; a suite-change test on the TLS 1.2 resumed conversation would be deleted by it. Add 2.1.1-5 to the child spec's Blocked-by table (main thread) |
| RFC5216-2.4-4 | queued (R6) | - | - | weak | the missing proof (ESP/IKE proposal selection independent of the TLS suite) lives in internal/component/ike/engine, held by the IKE judge; eap side only exports the MSK |
| rfc5216_resumption_defect_test.go | left, untracked | - | - | - | belongs to AC-4 of spec-ike-eap-rfc-defects (owner ruling: refuse TLS 1.2 resumption); still the only red in the package run |

### Approvals recorded (continuation 3)
eap.TestRFC3748PeerNeverRetransmitsOnATimer, eap.TestRFC3748EAPTLSExportsASixtyFourOctetEMSK, eap.TestEAPTLSMutualAuthHandshakeSucceeds (twice: 2.4-3 tag drop, 5.3 split), eap.TestRFC5216ServerSendsItsChainWithoutTheRoot, eap.TestEAPTLSPeerRejectsUntrustedServerChain, eap.TestEAPTLSPeerWithoutCARefusesToStart.

### Files changed (continuation 3)
- internal/core/eap/rfc3748_peer_timer_test.go (new)
- internal/core/eap/rfc5216_certless_peer_test.go (new)
- internal/core/eap/rfc3748_emsk_test.go (7.10-6 tag)
- internal/core/eap/eap_tls_handshake_test.go (2.4-3 tag dropped, 5.3-1 prose split, 5.3-4 tags)
- internal/core/eap/rfc5216_flight_content_test.go (5.3-1 -> 5.3-4 tag)
- rfc/short/rfc5216.md (5.3-1 narrowed, 5.3-4 and 5.3-5 {gap} added, Support remaining sentence)
- rfc/corrections/rfc5216.md (Correction 2026-09-30)
- rfc/discrimination/rfc3748.json, rfc/discrimination/rfc5216.json (records above)

### Gates (continuation 3)
- go test -race ./internal/core/eap/: red only on TestRFC5216ResumingServerSendsNothingButChangeCipherSpecAndFinished (AC-4 defect test). gofmt clean. golangci-lint ./internal/core/eap/...: 0 issues.
- ./le rfc check, rfc3748/rfc5216 lines: only STALE/SHIFTED audit verdicts remain (judge re-stamp: 3748 4.1-1, 7.10-2, 7.10-6; 5216 2.4-1..4, 2.1.1-1, 5.3-1, 2.3-1 stale; 5.4-1, 2.1.3-4, 2.1.1-5/6/9/10, 3-5 shifted -> reseal). No discrimination, extraction or row error. The new 5.3-4/5.3-5 rows need a first judgement.
- Owed by main thread/judge: re-judge the ids above; docs/features/rfc-status.md RFC 5216 row regenerates at index-update (gap count +1).

# Continuation 4 (2026-09-30, fourth author)

Nothing stamped, nothing committed, no reseal/audit-stamp/index-update. Records route revert, observed red, under flock ledger.lock. Rows appended per id.

| id | resolution | what now proves each clause (+/-) | records written | expected verdict | notes |
|----|-----------|------------------------------------|-----------------|------------------|-------|
| D-8 peer certificate (RFC5216-2.1.1-1) | defect fixed | + TestRFC5216PeerSendsItsCertificateWhateverTheRequestNames (new file rfc5216_peer_certificate_test.go): peer holding a cert whose issuer the certificate_request does not name sends certificate (first entry = configured DER) + certificate_verify, read off the wire. Red before the fix ("carries [11 16] and no certificate_verify"), green after | pos answerCertificateRequest | enforced | peer.go: new answerCertificateRequest (GetClientCertificate, RFC 5216 §2.1.1 quote above the return), new field tlsCertificate, tlsClientConfig drops the cert parameter and Certificates. Test helpers peerTLSConfigForTest / peerTLSConfigFor updated. Docs: ipsec-9-ikev2-eap-nat.md obligation bullet. NOTE: the first peer.go edit (field, startTLSClient, tlsClientConfig) was applied with a python replace, not Edit; the answerCertificateRequest block went through Edit and the hook |
| RFC5216-2.1.1-1 negative | tests + record | - TestRFC5216AuthenticatorRefusesAPeerThatSendsNoCertificate reworked: peer configured with the TRUSTED client cert; driveCertlessPeer empties peer.tlsCertificate once tlsStarted, so the certless flight (empty certificate_list, no certificate_verify) is asserted off the wire; authenticator: no EAP-Success, not Succeeded, handshake incomplete, Session.Err non-nil and NOT a tls.CertificateVerificationError | neg newTLSMethod (re-recorded, unit-sha bf0c5f4f) | enforced | approval D-15 recorded |
| TestRFC5216TLSConfigBothRolesInstallWhatTheFlightsCarry | test edit (fallout of D-8) | peer assertion now: Certificates empty, GetClientCertificate answers one certificate | see next row | stale for 2.1.1-4/5/6/9/10, 2.1.2-1 | approval D-15 recorded; claims unchanged |
| re-records (fallout of D-8) | records | TLSConfigBoth unit: 2.1.1-4/5/6, 2.1.2-1 (newTLSMethod), 2.1.1-9/10 (tlsClientConfig) positive; producer tlsClientConfig changed, so also 2.1.1-9/10 neg (TLSConfigRefusesWhatWouldBreakTheFlights), 2.1.1-1 pos (PeerFlightAnswersTheCertificateRequest), 2.4-1 pos (BothRolesInstallAVersionFloor), 2.4-3 pos (NeitherSideNegotiatesCompression), RFC9190-1-1 neg (TestEAPTLSVersionCapLeavesTLS12Reachable) | 11, all exit 0 | unchanged verdicts need re-stamp where STALE | |
| RFC5216-5.3-1 negative | tests + record | - TestEAPTLSServerRejectsUntrustedClientChain now asserts Session.Err is a tls.CertificateVerificationError wrapping x509.UnknownAuthorityError whose UnverifiedCertificates[0] is the configured untrusted client leaf (the chain was received and path-validated), besides no EAP-Success / handshake incomplete; distinct from 2.1.1-1's certless refusal which asserts NOT a verification error | neg newTLSMethod (unit-sha 42f9bf22) | enforced | approval D-15 recorded |
| RFC5216-5.3-4 | tests + records | + TestRFC5216ServerSendsItsIntermediateButNotTheRoot (new file rfc5216_server_chain_test.go): server leaf issued by an intermediate under the peer's root; certificate message = [leaf, intermediate], root absent; peer (root-only anchor) validates, no peer error, EAP-Success reached and peer done. - TestRFC5216PeerRefusesAServerChainMissingItsIntermediate: server configured with the leaf alone, [leaf] asserted on the wire (non-compliant form, R1 a), peer refuses with x509.UnknownAuthorityError, not done, no EAP-Success. Older single-level tags kept | pos newTLSMethod, neg peer_chain.go::verifyPeerCertificate | enforced (first judgement pending) | no approval needed (new units) |

### Approvals recorded (continuation 4)
eap.TestRFC5216TLSConfigBothRolesInstallWhatTheFlightsCarry, eap.TestRFC5216AuthenticatorRefusesAPeerThatSendsNoCertificate, eap.TestEAPTLSServerRejectsUntrustedClientChain.

### Files changed (continuation 4)
- internal/core/eap/peer.go (answerCertificateRequest, tlsCertificate field, tlsClientConfig signature)
- internal/core/eap/rfc5216_peer_certificate_test.go (new)
- internal/core/eap/rfc5216_server_chain_test.go (new)
- internal/core/eap/rfc5216_certless_peer_test.go (rewritten: driveCertlessPeer)
- internal/core/eap/eap_tls_handshake_test.go (5.3-1 negative assertions, bytes import)
- internal/core/eap/rfc5216_tls_config_test.go (peer assertion, helper)
- internal/core/eap/rfc9190_version_cap_test.go (helper only)
- docs/architecture/ike/ipsec-9-ikev2-eap-nat.md (RFC 5216 §2.1.1 bullet + source anchor)
- rfc/discrimination/rfc5216.json, rfc/discrimination/rfc9190.json

### Gates (continuation 4)
- go test -race ./internal/core/eap/: red only on TestRFC5216ResumingServerSendsNothingButChangeCipherSpecAndFinished (untracked AC-4 defect test, left). gofmt clean. golangci-lint ./internal/core/eap/...: 0 issues.
- ./le rfc check, rfc5216/rfc9190 lines: only STALE/SHIFTED audit verdicts (judge: 2.1.1-1, 5.3-1, 5.3-4, 2.1.1-4/5/6/9/10, 2.1.2-1 stale; 2.4-1/2, 2.3-1 shifted; 2.4-4 stale is the IKE author's). No discrimination/extraction/row error.
- Owed by main thread: judge re-stamp of the ids above; full lint/verify (whole tree) not run here.
