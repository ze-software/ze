# ike-eap / internal/component/ike/engine -- author handoff (2026-09-28)

Listing: the child's derived jq restricted to units under `internal/component/ike/engine/`, minus
"Blocked by" and the parent's cross-group ids: 36 verdicts. 13 resolved by new tests (awaiting an
independent judge), 23 unresolved (continuation owed). No existing tagged unit was edited, so no
`./le rfc approve` was needed except one for a new unit edited before its first run
(`engine.TestRFC7296ResponderSPIIsZeroInTheFirstMessageAndItsCookieRepeat`). No product code changed.
No verdict stamped, nothing committed.

## Resolved by tests (judge owed)

All records: `route revert`, OBSERVED red, written by `./le rfc discriminate-record`; `./le rfc discriminate stem <s>` shows no unproven line for the new files.

| id | resolution | what now proves each clause (+/-) | records written (producer) | expected verdict | notes |
|----|-----------|-----------------------------------|---------------------------|------------------|-------|
| RFC3948-2.1-1 | tests | + `TestRFC3948GeneratedESPSPIIsNeverZero` (non-zero draw returned unchanged), - same unit (five scripted zero draws discarded, 6 reads); + `TestRFC3948OutboundESPSPIIsNeverZero` (SAr2 SPI 0x0a0b0c0d is the one outbound state), - same unit (SAr2 SPI 0: not established, no ESP state). File `rfc3948_spi_test.go` | 4: `child.go::generateESPSPI` (+/-), `fsm.go::handleAuthResponse` (+/-) | enforced | old TestGenerateESPSPI tags kept (ratchet) |
| RFC5282-4-1 | tests | encryptor half: + `TestRFC5282AEADSendSealsUnderSaltThenIV` (ze's initiator and responder messages open under salt\|\|IV, not under IV\|\|salt); decryptor half and - unchanged (`TestRFC5282AEADNonceIsSaltThenIV`, `TestRFC5282AEADRefusesAReversedNonce`) | 1: `crypto/aead.go::ikeAEADNonce` | enforced | file `rfc5282_aead_send_test.go` |
| RFC5282-5.1-1 | tests | sender half: + `TestRFC5282AEADSendAssociatedDataRunsThroughTheSKHeader` (DataOffset 32, opens over raw[:32], not over raw[:31] nor the fixed header alone); receiver +/- unchanged | 1: `auth.go::buildSKMessageAEADWithMsgID` | enforced | |
| RFC5282-5.1-2 | tests | sender half: + `TestRFC5282AEADSendAssociatedDataExcludesTheIVAndCiphertext` (does not open with the IV, or IV+ciphertext, in the AAD); receiver +/- unchanged | 1: `auth.go::buildSKMessageAEADWithMsgID` | enforced | |
| RFC5282-7.3-1 | tests | AES CCM: + `TestRFC5282AEADCCMProposalCarriesTheKeyLengthAttribute` (IDs 14/15/16 x 128/256: one Key Length attr of the key size); GCM + and crypto - unchanged | 1: `initiator.go::encAttrs` | enforced | IKE rail only for CCM (ESP CCM not offered by ze? unverified) |
| RFC5282-8-1 | tests | ESP SA: + `TestRFC5282AEADESPSelectionCarriesNoIntegrityAlgorithm` (NegotiateESP over AES-GCM offer selects AUTH_NONE though Hash configured), - `TestRFC5282AEADESPOfferWithIntegrityIsRefused` (AES-GCM + HMAC offer refused); IKE +/- unchanged | 2: `crypto/proposal.go::matchESP` | enforced | |
| RFC5282-8-2 | tests | ESP proposal: + `TestRFC5282AEADESPProposalCarriesNoIntegrityTransform`, - `TestRFC5282NonAEADESPProposalKeepsItsIntegrityTransform`; IKE +/- unchanged | 2: `initiator.go::espProposalToWire` | enforced | |
| RFC7296-3.1-4 | tests | cookie-repeat clause: + `TestRFC7296ResponderSPIIsZeroInTheFirstMessageAndItsCookieRepeat` (challenge carries SPIr 09..09; first request and COOKIE repeat both zero at octets 8-15); - unchanged (TestSPIZeroRules: response non-zero) | 1: `sa_init_retry.go::retrySAInit` | enforced | file `rfc7296_spi_cookie_test.go` |
| RFC7296-3.1-3 | tests | receive half: - `TestRFC7296ZeroInitiatorSPIIsDroppedOnReceipt` (real dispatchInbound over UDP; table holds an SA with zero iSPI that would match; zero-iSPI datagram not delivered, the following non-zero one is); + unchanged | 1: `register.go::dispatchInbound` | enforced | the revert break halts the whole dispatcher, so the record proves reach, not the guard line; the SA-with-zero-iSPI setup is what isolates the guard. File `rfc7296_zero_ispi_test.go` |
| RFC7296-3.1-13 | tests | every message kind: + `TestRFC7296InitiatorBitFollowsTheOriginalRole` (engineBuiltChains: I set on ini requests, clear on resp responses; plus original responder's Delete REQUEST clear and original initiator's INFORMATIONAL RESPONSE set); DPD +/- unchanged | 1: `rekey.go::initiatorFlag` | enforced | the Delete request is built with `initiatorFlag(resp)` as the product call sites do |
| RFC4555-4.2.5-1 | tests | unpredictable clause: + `TestRFC4555Cookie2IsTheRandomSourceOutput` (crypto/rand.Reader scripted per request, two seeds: COOKIE2 == first 32 octets of that stream, length 8..64); length +/- unchanged | 1: `mobike.go::startMobikeRequest` | enforced | file `rfc4555_cookie2_test.go` |
| RFC7427-3-4 | tests | sent half: + `TestRFC7427IKESAInitSendsTheSignatureHashAlgorithmsNotify` (real IKE_SA_INIT request and response: exactly one notify each, listing 2,3,4); received +/- unchanged | 1: `initiator.go::buildSignatureHashAlgosNotify` | enforced | file `rfc7427_notify_sent_test.go` |
| RFC7296-2.20-1 | tests | + `TestRFC7296ResponderAnswersAVersionRequestWithNoCP` (IKE_AUTH request re-encrypted with CP(CFG_REQUEST, empty APPLICATION_VERSION): responder establishes, response has no CP); - unchanged (sweep + round trip) | 1: `responder.go::handleAuthRequest` | enforced | file `rfc7296_cp_version_test.go` |

## Unresolved (continuation owed)

| id | resolution | why / recommendation |
|----|-----------|---------------------|
| RFC4301-4.4.2-1 | unresolved | read the XFRM STATE selector (not the SPD policy) and assert proto/ports of the SAD entry |
| RFC4301-5.1-2 (wrong) | unresolved (row/tag) | units prove bidirectional-bypass mirroring; the §5.1 step-4 nested-SA re-crossing is a forwarding role (kernel XFRM?). Move the tags to a mirroring row (add one if none) and decide whether Ze fills the role; judgement for the main thread |
| RFC4301-5.2-5 (wrong) | unresolved (row/tag) | same as 5.1-2 on SPD-O |
| RFC4303-3.4.3-2 | unresolved | the missing half is OSPFv3 manual SA (`buildIPsecSA`); test belongs in `internal/plugins/ospf` (shared-file rule) |
| RFC4555-x-1 | unresolved | needs an EAP (multiple IKE_AUTH) case asserting MOBIKE_SUPPORTED sits in the message carrying SA; and a negative that Ze does not use MOBIKE without its own offer |
| RFC7296-1.2-1 | unresolved (row) | descriptive overview span with no keyword; on the "level with no BCP 14 keyword" list, phase 12 |
| RFC7296-1.4.1-4 | unresolved | needs a crossed-deletes setup; unit comment stale (TestDelResponseCarriesThePairedDelete) |
| RFC7296-2.10-3 | unresolved | assert rekey nonces (rekey.go), read PRFs from prfRegistry, replace the tightness negative |
| RFC7296-2.11-2 | unresolved | read the reply datagram on the odd-port listener |
| RFC7296-2.21.2-1 | unresolved | needs an unsupported-critical-payload request drawing exactly UNSUPPORTED_CRITICAL_PAYLOAD (see 2.5-18) |
| RFC7296-2.21.4-7 | unresolved | SHOULD-send clause: feed a suspicious message, assert a protected Notify goes out |
| RFC7296-2.23-2 | unresolved | assert IKE traffic floats to 4500 after NAT detection |
| RFC7296-2.4-3 | unresolved | ICMP/routing clause: inject an ICMP unreachable and assert the SA survives, or show Ze reads no ICMP (then a claim, not a test) |
| RFC7296-3.1-9 | unresolved (decision) | sender-only rule; HEAD negative proves 3.1-12; `{single-polarity}` barred by the ratchet. Main thread decides: retag the negative to 3.1-12 and annotate, or row change |
| RFC7296-3.10.1-3 | unresolved | add other uncovered errors (bad payload length, invalid field) drawing INVALID_SYNTAX |
| RFC7296-3.11-2 | unresolved (row) | fragment row (row-quality list); also IKE Delete non-zero SPI Size / AH size on receipt untested |
| RFC7296-3.14-3 | unresolved (decision) | accept-any-IV has no refusal path; HEAD negative is integrity; same decision as 3.1-9; also a fragment row |
| RFC7296-3.15.1-2 | unresolved (decision) | sender MUST NOT; Ze sends no CFG_REQUEST; negative proves receive tolerance; same decision as 3.1-9 |
| RFC7296-3.3.2-1 | unresolved (row) | duplicate span with 3.3.6-1 (row-quality merge); add peer proposals missing ENCR/INTEG/D-H |
| RFC7296-3.3.6-1 (wrong) | unresolved (row) | merge into 3.3.2-1 moving its tags (row-quality table) |
| RFC7296-3.3.6-5 | unresolved | needs a transform ID Ze does not understand beside a sibling, and the attribute not-understood clause |
| RFC7296-3.5-5 | unresolved (row) | the quote runs past the ID_FQDN item into "ID_RFC822_ADDR 3 A fully-qua..." (crosses a list item); "All characters ... are ASCII" has no keyword. Row correction first |
| RFC7296-3.6-2 | unresolved | accept half: a received encoding 12/13 resolved and accepted when configured |

Mistagged units (child table): NOT done. rfc7296 has no row stating TS presence in a rekey answer
(grep of `rfc/short/rfc7296.md` for TSi/TSr), so moving the 2.9-1 tags of
`TestChildRekeyAnswerWithoutTrafficSelectorsIsRefused` / `TestRekeyWithoutTrafficSelectorsIsRefused`
needs a new row (id, extraction site, records). `TestSPDPolicyMirrorsTheInboundSelector` (4.4.1-4)
needs a target row too; the same new "mirroring" row could take the 5.1-2/5.2-5 bypass units. Phase 12 work.

## Verified

- `go test -race -count=1 ./internal/component/ike/engine/` under `./le job run`: ok (101.7s), after the last edit.
- gofmt clean on the package.
- OWED to the main thread: `./le go lint run` (post-write hook linted the package), `./le rfc check`, `./le rfc reseal` if shifted (no existing test file edited, so none expected), independent judge + `audit-stamp mode rejudge` for the 13 ids.

## Files changed

- internal/component/ike/engine/rfc5282_aead_send_test.go (new)
- internal/component/ike/engine/rfc3948_spi_test.go (new)
- internal/component/ike/engine/rfc7296_spi_cookie_test.go (new)
- internal/component/ike/engine/rfc4555_cookie2_test.go (new)
- internal/component/ike/engine/rfc7427_notify_sent_test.go (new)
- internal/component/ike/engine/rfc7296_cp_version_test.go (new)
- internal/component/ike/engine/rfc7296_initiator_bit_test.go (new)
- internal/component/ike/engine/rfc7296_zero_ispi_test.go (new)
- rfc/discrimination/rfc5282.json, rfc3948.json, rfc7296.json, rfc4555.json (new file if absent), rfc7427.json (records written by discriminate-record)
- the `./le rfc approve` store entry for engine.TestRFC7296ResponderSPIIsZeroInTheFirstMessageAndItsCookieRepeat
- plan/journal/concurrent-session-corruption.md (one row: discriminate-record overlay basename collision between subagents)

# Continuation 1 (2026-09-29): judge's 5 weak

| id | resolution | what now proves each clause (+/-) | records written (producer) | expected verdict | notes |
|----|-----------|-----------------------------------|---------------------------|------------------|-------|
| RFC5282-8-1 | tests | responder ESP: + `TestRFC5282ResponderSelectsNoIntegrityForAnAEADChildSA` (selectResponderESP accepts an AES-GCM-only SAi2 with Hash configured; the SAr2 proposal espProposalToWire builds from the selection carries AES GCM and 0 INTEG), - `TestRFC5282ResponderRefusesAnAEADChildSAOfferWithIntegrity` (AES-GCM + HMAC-SHA2-256 SAi2 -> ErrNoProposalChosen, nothing accepted). File `rfc5282_responder_esp_test.go` | 2: revert `responder.go::espProposalMatches` | enforced | gomu report in scratch covers child.go only (0 killed), so revert |
| RFC7296-3.1-3 | tests | NAT-T: - `TestRFC7296ZeroInitiatorSPIIsDroppedOnTheNATTSocket` (real dispatchNATTInbound, non-ESP-marked datagrams, table holds a zero-iSPI SA; zero-iSPI datagram not delivered, the following non-zero one delivered with the marker stripped). File `rfc7296_zero_ispi_natt_test.go` | 1: revert `register.go::dispatchNATTInbound` | enforced | reach record; the zero-iSPI SA isolates the guard |
| RFC7296-3.1-13 | tests | + `TestRFC7296InitiatorBitAtEveryRemainingCallSite`, both original roles, product-built only: writeDelete, sendIKESATeardown, startMobikeRequest (read off the peer socket), respondMobikeError, buildErrorNotifyResponse, initiateChildRekey, respondChildRekey accepted + INVALID_KE (pfs, no KEi), respondIKERekey accepted + INVALID_KE (KEi group 19). File `rfc7296_initiator_bit_sites_test.go` | 1: revert `rekey.go::initiatorFlag` | enforced | old unit's self-built Delete stays (not edited) |
| RFC7296-2.20-1 | tests | INFORMATIONAL (the §2.20 method): + `TestRFC7296InformationalVersionRequestDrawsNoCP` (post-auth INFORMATIONAL CP(CFG_REQUEST, empty APPLICATION_VERSION) -> response the peer decrypts has no CP), - `TestRFC7296InformationalVersionRequestReflectsNoVersionString` (R1(b): request carries "peer-ike 6.0.1"; answer has no CP and no non-empty version). File `rfc7296_cp_version_informational_test.go` | 2: revert `mobike.go::mobikeResponsePayloads` (the builder of the INFORMATIONAL answer's payloads) | enforced | old codec-round-trip negative in TestZeDeclinesApplicationVersion left in place (not edited) |
| RFC3948-2.1-1 | tests (tags) | added RFC3948-2.1-1 +/- tags to the six units in `rfc4303_peer_spi_test.go`: responder SAi2 (TestResponderRefusesPeerSPIZero/UsesPeerSPI), rekey request (TestChildRekeyRequestRefusesPeerSPIZero/UsesPeerSPI), rekey response (TestChildRekeyResponseRefusesPeerSPIZero/UsesPeerSPI) | 6: revert buildAuthResponse / respondChildRekey / applyChildRekeyResponse | enforced | 6 `./le rfc approve` D-15 entries; the RFC4303-2.1-1 records of those units may need `./le rfc reseal` (comment lines inserted) |

# Continuation 1 (2026-09-29): unresolved rows handled

| id | resolution | what now proves each clause (+/-) | records | expected verdict | notes |
|----|-----------|-----------------------------------|---------|------------------|-------|
| RFC7296-3.1-9 | tests (R1 a) | - NEW `TestRFC7296InitiatorIgnoresAResponseWithTheRBitClear` (`rfc7296_response_bit_receive_test.go`): the responder's real IKE_SA_INIT response with R cleared is not processed by the initiator's handleInbound (stays StateSAInitSent); with R set it is. + unchanged (TestResponseBitMatchesDirection sweep). Old negative MOVED to RFC7296-3.1-12 (the responder does not process or answer an R-set request) | new: revert fsm.go::handleInbound; moved 3.1-12 negative: revert responder.go::handleResponderInbound | enforced | D-15 approval engine.TestResponseBitMatchesDirection |
| RFC7296-3.14-3 | tests (R1 b) | - NEW `TestRFC7296RecipientAcceptsAnIVTheSenderShouldNotHaveChosen` (`rfc7296_iv_receive_test.go`): a repeated IV and the previous message's final ciphertext block as IV both decrypt; + unchanged. Old negative (flipped checksum refused) MOVED to RFC7296-3.14-6 negative | new: revert auth.go::decryptSKPayload; the moved 3.14-6 record is OWED (blocked, below) | enforced | D-15 approval engine.TestSKAcceptsAnyIVOnReceipt. Row is still the fragment "recipients MUST accept any value" |
| RFC7296-3.15.1-2 | unresolved -> OWNER-GATE (R1 fallback) | sender MUST NOT; Ze builds no CP, so no producer input can be pushed toward a non-empty netmask (R1 b impossible); the RFC has the receiver ignore, never refuse (R1 a impossible) | none | weak | add to RULINGS OWNER-GATE |
| RFC7296-3.3.6-1 into 3.3.2-1 | row (R5 merge) | 3.3.6-1 removed from rfc/short/rfc7296.md; `Retired 2026-09-29` paragraph in rfc/corrections/rfc7296.md; its unsourced-ids entry removed from rfc/extraction/rfc7296.json (§3.3.6 reason reworded); tags of TestResponderRequiresKEForDH retagged RFC7296-3.3.2-1 +/- | OWED (blocked): 3.3.2-1 positive and negative, producer responder.go::handleSAInitRequest | 3.3.2-1 re-judge | approvals engine.TestResponderRequiresKEForDH and engine.rfc7296_test (the hook keys a moved tag by file). rfc/audit/rfc7296.json still holds a 3.3.6-1 entry: judge removes it. 3.3.2-1's "peer proposals missing ENCR/INTEG/D-H" NOT added |

BLOCKED records (3): every `./le rfc discriminate-record` run, and every Bash call whose text names rfc paths, is refused: "internal/component/l2tp/reactor_sccrq_zero_tid_test.go:75: tag for RFC2661-10-2negative has invalid polarity" (another author's in-progress l2tp edit, a missing space after the id, likely the R5 10-2 = 24.10-1 merge). After it is fixed:
- discriminate-record id RFC7296-3.14-6 polarity negative unit internal/component/ike/engine/rfc7296_encrypt_test.go::TestSKAcceptsAnyIVOnReceipt route revert producer internal/component/ike/engine/auth.go::decryptSKPayload
- discriminate-record id RFC7296-3.3.2-1 polarity positive unit internal/component/ike/engine/rfc7296_test.go::TestResponderRequiresKEForDH route revert producer internal/component/ike/engine/responder.go::handleSAInitRequest
- the same with polarity negative

Still unresolved (budget): the mistagged units (a new TS-presence row for the RFC7296-2.9-1 rekey tags; a mirroring row for RFC4301-4.4.1-4 / 5.1-2 / 5.2-5), RFC3748-4-1 and RFC3748-4.1-3 engine tests, and the other rows of the first table (RFC4301-4.4.2-1, RFC4303-3.4.3-2, RFC4555-x-1, RFC7296-1.2-1, 1.4.1-4, 2.10-3, 2.11-2, 2.21.2-1, 2.21.4-7, 2.23-2, 2.4-3, 3.10.1-3, 3.11-2, 3.3.6-5, 3.5-5, 3.6-2). Recommendations unchanged from the first table.

Verified: the affected units (17 PASS) under `./le job run` with a private GOCACHE (scratch/gocache), because a concurrent `./le scratch cache-clean` (`go clean -cache`, 17+ min) was deleting ~/.cache/go-build entries under every build. The full-package `-race` run after this continuation's edits is OWED. Also OWED: `./le go lint run` (post-write lint failed on the vanishing cache), `./le rfc check`, `./le rfc reseal` (comment lines inserted above tagged units in rfc4303_peer_spi_test.go, rfc7296_header_test.go, rfc7296_encrypt_test.go, rfc7296_test.go), judge and audit-stamp for every id above. One edit to rfc7296_initiator_bit_sites_test.go went through a python rewrite, not the Edit tool, so no post-write hook saw it; gofmt is clean.

## Files changed (continuation 1)
- internal/component/ike/engine/rfc5282_responder_esp_test.go (new)
- internal/component/ike/engine/rfc7296_zero_ispi_natt_test.go (new)
- internal/component/ike/engine/rfc7296_initiator_bit_sites_test.go (new)
- internal/component/ike/engine/rfc7296_cp_version_informational_test.go (new)
- internal/component/ike/engine/rfc7296_response_bit_receive_test.go (new)
- internal/component/ike/engine/rfc7296_iv_receive_test.go (new)
- internal/component/ike/engine/rfc4303_peer_spi_test.go (6 RFC3948-2.1-1 tags)
- internal/component/ike/engine/rfc7296_header_test.go (3.1-9 negative tag moved to 3.1-12)
- internal/component/ike/engine/rfc7296_encrypt_test.go (3.14-3 negative tag moved to 3.14-6)
- internal/component/ike/engine/rfc7296_test.go (3.3.6-1 tags moved to 3.3.2-1)
- rfc/short/rfc7296.md, rfc/corrections/rfc7296.md, rfc/extraction/rfc7296.json
- rfc/discrimination/rfc5282.json, rfc7296.json, rfc3948.json (records)
- the `./le rfc approve` store: 10 entries (6 peer-SPI units, TestResponseBitMatchesDirection, TestSKAcceptsAnyIVOnReceipt, TestResponderRequiresKEForDH, engine.rfc7296_test)

# Continuation 2 (2026-09-29)

Blocked records now written (all `route revert`, OBSERVED red): RFC7296-3.14-6 negative (TestSKAcceptsAnyIVOnReceipt, auth.go::decryptSKPayload: a reach record, the break panics inside establishPSK; the integrity check is inline so no narrower revert exists, a gomu mutant would be the targeted route), RFC7296-3.3.2-1 positive and negative (TestResponderRequiresKEForDH, responder.go::handleSAInitRequest).

rfc7296_initiator_bit_sites_test.go: re-opened with Edit (one comment line reworded), hooks passed; the package vets clean, `parseAndDecrypt` is no longer referenced anywhere.

| id | resolution | what now proves each clause (+/-) | records (producer) | expected verdict | notes |
|----|-----------|-----------------------------------|--------------------|------------------|-------|
| RFC7296-1.3.3-3 (NEW row, §1.3.3 request sentence, MUST under D-3, no keyword) | row + tests | + NEW `TestRFC7296ChildRekeyRequestCarriesTheOfferNonceAndSelectors` (initiateChildRekey request decrypted by the peer: 1 SA, 1 Ni, 1 TSi, 1 TSr with selectors, KEi iff pfs enable); - NEW `TestRFC7296ChildRekeyRequestMissingAPayloadIsRefused` (peer request minus SA / Ni / TSi / TSr each refused errMalformedRequest + INVALID_SYNTAX, nothing installed; complete request answered); +/- MOVED from 2.9-1: `TestRekeyWithoutTrafficSelectorsIsRefused` | 4: rekey.go::initiateChildRekey, ::respondChildRekey (x3) | enforced | file `rfc7296_rekey_payloads_test.go` |
| RFC7296-1.3.3-4 (NEW row, §1.3.3 response sentence) | row + tests | + NEW `TestRFC7296ChildRekeyResponseCarriesTheSelectorsAndMayNarrow` (ze's response announces the installed scope; ze as initiator installs an answer narrowed to 10.1.0.0/25 from a /24 proposal); +/- MOVED from 2.9-1: `TestChildRekeyAnswerWithoutTrafficSelectorsIsRefused` | 3: rekey.go::applyChildRekeyResponse | enforced | |
| RFC4301-4.4.1-11 (NEW row, SPD-I bullet of §4.4.1) | row + tags | + `TestSPDPolicyMirrorsTheInboundSelector` (tag MOVED from 4.4.1-4), + `TestRFC4301BypassReturnTrafficHasAnSPDIEntry`, - `TestRFC4301BypassSPDIEntryIsNeverAnUnmirroredCopy` (tags ADDED beside 5.1-2) | 3: spd_policy.go::spdPolicyParams | enforced | |
| RFC4301-4.4.1-12 (NEW row, SPD-O bullet of §4.4.1) | row + tags | + `TestSPDPolicyMirrorsTheInboundSelector`, + `TestRFC4301InboundBypassReturnTrafficHasAnSPDOEntry`, - `TestRFC4301BypassSPDOEntryIsNeverAnUnmirroredCopy` (tags ADDED beside 5.2-5) | 3: spdPolicyParams | enforced | |
| RFC4301-5.1-2, 5.2-5 | unresolved (main thread) | their units keep their tags; they now also prove 4.4.1-11/12, which is what they really assert | none | wrong (unchanged) | both quotes are conditional on a packet re-crossing the IPsec boundary for nested-SA processing (§5.1 step 4, §5.2 last paragraph). Ze configures no nested SAs; per-packet forwarding is Linux XFRM. R2 route (retire "binds nested-SA processing ...") needs the main thread's call that neither Ze nor XFRM-on-Ze's-behalf fills that role (rfc-compliance: binds-another-role evidence); once decided, drop the 5.1-2/5.2-5 tags from the four units (coverage stays held by 4.4.1-11/12) |
| RFC7296-2.9-1 | tags moved | the two mistagged units no longer carry 2.9-1; the verdict's other units stay | | re-judge | |
| RFC3748-4.1-3 | tests (tag) | clause 1 (retransmitted Request keeps the Identifier): + `TestEapRtxResponderReplaysCachedResponseMidEAP` (the EAP Request re-sent on a retransmitted IKE_AUTH is the cached response byte for byte); clause 2 unchanged (core/eap) | 1: responder.go::replayCachedResponse | enforced | D-15 approvals for the unit and file |
| RFC3748-4-1 | DEFECT (D-8), stopped | no test written (budget) | none | weak | handleResponderEAP (responder_eap.go) and handleEAPResponse (fsm.go) set StateDead when the inner chain fails to parse; PayloadEAP.ReadFrom (wire/payload_eap.go) returns ErrTruncated when the EAP Length exceeds the payload, so an EAP message with Length > octets ends the IKE SA instead of being silently discarded. Recommendation: a distinct wire sentinel for the EAP length error, and both handlers drop the message and keep StateEAPInProgress (no notify, no state change, retransmit timer untouched). Design choice for the main thread: this also touches RFC 7296 §3.10.1 INVALID_SYNTAX on the responder side. Failing test fixture: copy eaprtxResponderMidExchange keeping `ini`, seal a raw inner chain with EAP Length 12 over 5 octets via buildSKMessage*WithMsgID, deliver through ps.handleResponderInbound, assert state stays EAPInProgress and nothing is sent. Same file: a decrypt failure also sets StateDead in both handlers (journal candidate) |

Stale ledger entries for the judge: rfc/discrimination/rfc7296.json still holds RFC7296-2.9-1 records for the two moved rekey units; rfc/discrimination/rfc4301.json holds the RFC4301-4.4.1-4 record for TestSPDPolicyMirrorsTheInboundSelector. The rfc7296 summary's Enrolment reason still says "227 rows, 222 of them gated"; the file now has 230 rows and 224 gated rows (the 3.3.6-1 retire of continuation 1 and the two new rows). Owed: `./le rfc check` (reseal for comment lines inserted above tagged units in rekey_test.go, child_rekey_initiator_answer_test.go, rfc4301_spd_discard_test.go, rfc4301_bypass_return_test.go, rfc7296_eap_retransmit_test.go), `./le go lint run`, full-package `-race` run, judge + audit-stamp for every id above.

Verified: 14 affected top-level units PASS under `./le job run` (`go test -count=1 -v -run ...`), gofmt clean.

Still unresolved (unchanged recommendations, first table): RFC4301-4.4.2-1, RFC4303-3.4.3-2, RFC4555-x-1, RFC7296-1.2-1, 1.4.1-4, 2.10-3, 2.11-2, 2.21.2-1, 2.21.4-7, 2.23-2, 2.4-3, 3.10.1-3, 3.11-2, 3.3.6-5, 3.5-5, 3.6-2; RFC7296-3.15.1-2 OWNER-GATE.

## Files changed (continuation 2)
- internal/component/ike/engine/rfc7296_rekey_payloads_test.go (new)
- internal/component/ike/engine/rekey_test.go, child_rekey_initiator_answer_test.go (2.9-1 tags moved to 1.3.3-3 / 1.3.3-4)
- internal/component/ike/engine/rfc4301_spd_discard_test.go (4.4.1-4 tag moved to 4.4.1-11/12)
- internal/component/ike/engine/rfc4301_bypass_return_test.go (4.4.1-11/12 tags added)
- internal/component/ike/engine/rfc7296_eap_retransmit_test.go (RFC3748-4.1-3 tag added)
- internal/component/ike/engine/rfc7296_initiator_bit_sites_test.go (comment reworded)
- rfc/short/rfc7296.md (rows 1.3.3-3, 1.3.3-4), rfc/extraction/rfc7296.json (§1.3.3 unsourced-ids + reason)
- rfc/short/rfc4301.md (rows 4.4.1-11, 4.4.1-12), rfc/extraction/rfc4301.json (§4.4.1 unsourced-ids)
- rfc/discrimination/rfc7296.json, rfc4301.json, rfc3748.json (records)
- `./le rfc approve` store: 13 entries (8 units: the two rekey units, TestSPDPolicyMirrorsTheInboundSelector, the four bypass units, TestEapRtxResponderReplaysCachedResponseMidEAP; and their 5 files)

# Continuation 3 (2026-09-29, relayed by main thread from the author's final report)
All records route revert, observed red.
| id | resolution | proof | records | expected | notes |
|----|-----------|-------|---------|----------|-------|
| RFC3748-4-1 | DEFECT fixed (D-8, R14) + tests | NEW rfc3748_eap_length_test.go: TestRFC3748AuthenticatorDiscardsAnOverlongEAPLength (- overlong EAP Response at round-2 msg id leaves StateEAPInProgress, no datagram; + real round-2 request then processed); TestRFC3748PeerDiscardsAnOverlongEAPLength (- overlong EAP Request on first IKE_AUTH response and round 2 via handleInbound leaves state/NextMsgID/LastSentMsg; + real message then processed). Both failed first (StateDead) | 4, producer engine/eap_auth.go::eapMessageDiscarded (panic break: reach only) | enforced | core/eap tags unchanged |
| RFC4301-5.1-2, 5.2-5 | RETIRED (R15) | tags deleted from 4 units in rfc4301_bypass_return_test.go (4.4.1-11/12 keep coverage) | 4 records dropped | n/a (audit entries dropped) | NEW rfc/corrections/rfc4301.md 2 Retired paragraphs; extraction 5.1:2, 5.2:4 excluded binds-another-role citing dataplane/xfrm_linux.go (one template per PROTECT, none for BYPASS); 5 D-15 approvals |
| RFC7296-3.3.2-1 | tests | NEW rfc7296_proposal_mandatory_test.go via wireProposalsToIKE + crypto.NegotiateIKE: + complete proposal chosen (and AES-GCM with no INTEG -> integrity NONE); - missing ENCR/INTEG (ErrProposalIncomplete), D-H (ErrDHGroupNone) | + crypto/proposal.go::negotiateIKE, - ikeProposalComplete | enforced | |
| RFC7296-3.14-6 | tests | NEW rfc7296_checksum_scope_test.go: + HMAC-SHA256-128 over header..ciphertext under SK_ai equals Ze's ICV and is accepted; - ICV over header+IV+PLAINTEXT refused (control accepted) | + engine/auth.go::buildSKMessageCBCWithMsgID, - crypto/cipher.go::VerifyIntegrity | enforced | CBC only |
R14 product change: wire/payload_eap.go ErrEAPLengthExceedsData (eapLen<4 still ErrTruncated); engine/eap_auth.go eapMessageDiscarded(err) with RFC 3748 §4 quote; callers responder_eap.go::handleResponderEAP, fsm.go::handleEAPResponse, fsm.go::handleAuthResponse (third site had the same defect) drop silently, no notify. Docs: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md. Journal: plan/journal/unprotected-message-changes-sa-state.md (initiator decrypt failure -> StateDead).
Verified: go test ike/engine, ike/wire, ike/crypto pass; gofmt clean.
OWED: lint; full -race; rfc check (+reseal: comment lines removed above tagged units in rfc4301_bypass_return_test.go); confirm 4301 retire passes checkRetiredRequirements/checkIDAllocation; judge + stamp RFC3748-4-1, RFC7296-3.3.2-1, 3.14-6; drop stale rfc7296 3.3.6-1 audit entry.
UNRESOLVED (cont 4): RFC4301-4.4.2-1, RFC4303-3.4.3-2, RFC4555-x-1, RFC7296-1.2-1, 1.4.1-4, 2.10-3, 2.11-2, 2.21.2-1, 2.21.4-7, 2.23-2, 2.4-3, 3.10.1-3, 3.11-2, 3.3.6-5, 3.5-5, 3.6-2. RFC7296-3.15.1-2 OWNER-GATE.
Files: ike/wire/payload_eap.go; ike/engine/{eap_auth.go,responder_eap.go,fsm.go,rfc3748_eap_length_test.go,rfc7296_proposal_mandatory_test.go,rfc7296_checksum_scope_test.go,rfc4301_bypass_return_test.go}; docs/architecture/ike/ipsec-9-ikev2-eap-nat.md; plan/journal/unprotected-message-changes-sa-state.md; rfc/short/rfc4301.md; rfc/corrections/rfc4301.md; rfc/extraction/rfc4301.json; rfc/audit/rfc4301.json; rfc/discrimination/{rfc3748,rfc7296,rfc4301}.json.

# Continuation 4 (2026-09-30)
All records route revert, OBSERVED red.
| id | resolution | proof (+/-) | records (producer) | expected | notes |
|----|-----------|-------------|--------------------|----------|-------|
| RFC7296-3.5-5 | row (Correction) | row now quotes the ID_FQDN entry through its ban; +/- TestWp2IDTerminatorRefused / TestWp2IDWithoutTerminatorAccepted (tag quotes made verbatim; the old tag quote was invented) | 2 (remote_id.go::refuseIDTerminators) | enforced | ASCII sentence has no keyword, dropped from the span; Correction paragraph in rfc/corrections/rfc7296.md; extraction 3.5:2 reason reworded (edited by python, not Edit) |
| RFC7296-3.5-6 (NEW, ID_RFC822_ADDR entry) | row + tags | + receive-path NUL mail refused; - clean mail accepted (receive + config remote-id) on the same two units | 2 (same producer) | enforced | extraction 3.5:3 duplicate-of -> mapped RFC7296-3.5-6; D-15 approvals for both units + file |
| RFC7296-3.11-2 | row (Correction: span widened to the SPI Size field entry) + tests | - NEW TestRFC7296DeleteWrongSPISizeForItsProtocolIsRefused (IKE size 4, AH 0/8, ESP 0: one INVALID_SYNTAX, no Delete, nothing removed, no reestablish); + NEW TestRFC7296DeleteSPISizeFixedForItsProtocolIsAccepted (IKE 0, AH 4: no notify); old ESP units/wire unit unchanged | 2 (delete.go::deleteMalformed; positive is a reach record) | enforced | file engine/rfc7296_delete_spi_size_test.go |
| RFC7296-2.11-2 | tests | + NEW TestRFC7296ReplyLandsOnTheRequestSourcePort (datagram read on the socket bound to the request's odd source port 34567: response flag, request's Message ID, decrypts under the IKE SA); - NEW TestRFC7296ReplyDoesNotGoToTheConfiguredEndpoint (configured remote receives nothing) | 2 (sa.go::adoptAuthenticatedEndpoint, panic break: reach) | enforced | file engine/rfc7296_reply_port_test.go. Old - tag on TestNattUnauthenticatedPacketDoesNotMoveTheEndpoint proves the §2.23 dynamic-update bound; rfc7296 has no such row, so it stays (judge: drop or home it) |
| RFC7296-2.23-2 | row (Correction) | row quoted the same sentence as 2.23-8 (IKE float, proven by 2.23-8's natt units); its extraction site 2.23:5 and both tagged units are the ESP sentence "However, if a NAT is detected, both devices MUST use UDP encapsulation for ESP." Row now quotes that. + TestChildSANATTEncapPorts, - TestChildSANoNATNoEncap (unchanged) | 2 (child.go::installChildSA, reach) | enforced (negative is an antecedent guard: without NAT §2.23 lets either side choose, judge may call it weak) | extraction 2.23:5 reason reworded |
| RFC7296-1.4.1-4 | tests (tags moved) | + TestDelCrossingDeleteAnswersWithoutAPairedDelete (own ESP Delete outstanding, peer's crossing Delete for the pair answered with no Delete payload); - TestDelWithoutOwnDeleteTheSameRequestIsPaired (same Delete without the crossing record IS paired). Both tags ADDED (units kept their 1.4.1-7 tags); the two 1.4.1-4 tags REMOVED from TestLcyInformationalResponseCarriesNoDeletePayload and its stale comment rewritten (premise false since TestDelResponseCarriesThePairedDelete) | 2 (delete.go::crossOwnDelete) | enforced | 5 D-15 approvals. Stale for judge: any 1.4.1-4 record on the lifecycle unit in rfc/discrimination/rfc7296.json; the {gap} on RFC7296-1.4-1 the old comment cited may now be stale too (ordinary pairing exists) |
| RFC7296-2.10-3 | tests (tags moved) | + NEW TestRFC7296EmittedNoncesMeetHalfOfEveryPRFKey (every PRF from crypto.SupportedPRFNames: IKE_SA_INIT Ni, Nr and a real CREATE_CHILD_SA rekey Ni decrypted off the wire each >= ceil(key/2)); - NEW TestRFC7296EmittedNoncesMeetTheLargestPRFBound (R1 b: the largest-key PRF, half > 16 so a wire-minimum nonce would fail; every emitted nonce meets it). Both tags REMOVED from TestNonceMeetsHalfPRFKeySize (comment now VALIDATES only) | 2 (sa.go::GenerateNonce, reach; nonceLen is a constant, no function producer) | enforced | file engine/rfc7296_nonce_size_test.go; responder rekey Nr (rekey.go:579/1097) not read; stale records for the old unit in rfc7296.json for the judge |

Verified (continuation 4): `go test -race -count=1 ./internal/component/ike/engine/` under `./le job run`: ok (103.7s) after the last edit; gofmt clean. Post-write lint hook passed on every edit after the one compile fix.
OWED to main thread: `./le go lint run` (package), `./le rfc check` (+ `./le rfc reseal`: comment lines changed above tagged units in rfc7296_wp2_test.go, rfc7296_delete_test.go, rfc7296_lifecycle_test.go, rfc7296_encrypt_test.go), judge + audit-stamp for RFC7296-3.5-5, 3.5-6 (new), 3.11-2, 2.11-2, 2.23-2, 1.4.1-4, 2.10-3. Stale for the judge: rfc/audit/rfc7296.json RFC7296-3.3.6-1 entry (row retired in continuation 1); discrimination records for RFC7296-1.4.1-4 on TestLcyInformationalResponseCarriesNoDeletePayload and RFC7296-2.10-3 on TestNonceMeetsHalfPRFKeySize, if any. The rfc7296 Enrolment reason row count moves by one (new row 3.5-6).
Approvals (`./le rfc approve`, D-15): engine.TestWp2IDTerminatorRefused, engine.TestWp2IDWithoutTerminatorAccepted, engine.rfc7296_wp2_test, engine.TestRFC7296ReplyLandsOnTheRequestSourcePort (own new unit, compile fix), engine.TestDelCrossingDeleteAnswersWithoutAPairedDelete, engine.TestDelWithoutOwnDeleteTheSameRequestIsPaired, engine.TestLcyInformationalResponseCarriesNoDeletePayload, engine.rfc7296_delete_test, engine.rfc7296_lifecycle_test, engine.TestNonceMeetsHalfPRFKeySize, engine.rfc7296_encrypt_test.

## Files changed (continuation 4)
- internal/component/ike/engine/rfc7296_delete_spi_size_test.go (new)
- internal/component/ike/engine/rfc7296_reply_port_test.go (new)
- internal/component/ike/engine/rfc7296_nonce_size_test.go (new)
- internal/component/ike/engine/rfc7296_wp2_test.go (3.5-5 tag quotes verbatim, 3.5-6 tags added)
- internal/component/ike/engine/rfc7296_delete_test.go (1.4.1-4 tags added)
- internal/component/ike/engine/rfc7296_lifecycle_test.go (1.4.1-4 tags removed, stale comment rewritten)
- internal/component/ike/engine/rfc7296_encrypt_test.go (2.10-3 tags removed)
- rfc/short/rfc7296.md (3.5-5 corrected, 3.5-6 new, 3.11-2 widened, 2.23-2 corrected)
- rfc/corrections/rfc7296.md (4 Correction paragraphs)
- rfc/extraction/rfc7296.json (3.5:2 reason, 3.5:3 -> mapped RFC7296-3.5-6, 2.23:5 reason)
- rfc/discrimination/rfc7296.json (14 records)

## Ids left (continuation 5), recommendations unchanged from the Unresolved table
RFC4301-4.4.2-1, RFC4303-3.4.3-2 (ospf package), RFC4555-x-1, RFC7296-1.2-1 (D-3 keep; needs a full-exchange test over each clause: algorithms, nonces, DH, AUTH, IDs/CERT, first Child SA), 2.21.2-1, 2.21.4-7, 2.4-3, 3.10.1-3, 3.3.6-5, 3.6-2. RFC7296-3.15.1-2 OWNER-GATE. Nothing half-done.

# Continuation 5 (2026-09-30)
All records route revert, OBSERVED red.
| id | resolution | proof (+/-) | records (producer) | expected | notes / files |
|----|-----------|-------------|--------------------|----------|---------------|
| RFC7296-3.5-5 | producer comments | invented quote "The ID_FQDN and ID_RFC822_ADDR strings MUST NOT contain..." replaced by the two verbatim §3.5 sentences in engine/remote_id.go (refuseIDTerminators doc) and ipsec/validate.go (ValidateIdentities); tags unchanged (+ TestWp2IDTerminatorRefused, - TestWp2IDWithoutTerminatorAccepted) | 2 re-recorded (remote_id.go::refuseIDTerminators) | enforced | ipsec/validate.go comment above no tagged unit's producer record; judge may want reseal |
| RFC7296-3.5-6 | row {feature-declined} (R19) | quote "implementations MUST be configurable to send at least one of ID_IPV4_ADDR, ID_FQDN, ID_RFC822_ADDR, or ID_KEY_ID"; producer engine/auth.go::encodeIKEID. Both 3.5-6 tags dropped from the wp2 units (D-15 approvals), their mail-address cases stay as supplementary coverage under 3.5-5; the 2 3.5-6 records deleted from rfc/discrimination/rfc7296.json; Support remaining row discloses the declined feature | 0 | met (declined) | extraction 3.5:3 still mapped-to RFC7296-3.5-6 (row kept, annotated), no change. Files: rfc/short/rfc7296.md, engine/rfc7296_wp2_test.go, rfc/discrimination/rfc7296.json |
| RFC7296-2.23-13 (NEW, R20) | row + tests | row quotes "When such a validated packet is found, ... (that is, they SHOULD dynamically update the address)." [SHOULD]; extraction section 2.23 unsourced-ids (no MUST-level site). + NEW TestRFC7296ValidatedPacketFromANewEndpointMovesTheSA (not behind NAT, no MOBIKE: second validated request from a new port moves peerEndpoint and remoteUDPAddr, response read on the new socket); - NEW TestRFC7296ValidatedPacketDoesNotMoveTheSAOutsideTheRow (behind NAT / MOBIKE: kept); - TestNattUnauthenticatedPacketDoesNotMoveTheEndpoint (tag MOVED from RFC7296-2.11-2 negative, comment now cites the validated-packet sentence; its old 2.11-2 record deleted; 2.11-2 negative still held by TestRFC7296ReplyDoesNotGoToTheConfiguredEndpoint) | 3 (sa.go::adoptAuthenticatedEndpoint) | enforced | new file engine/rfc7296_dynamic_update_test.go; engine/rfc7296_natt_test.go; rfc/extraction/rfc7296.json; approvals D-15 x3 |
| RFC7296-2.10-3 | tests | + TestRFC7296EmittedNoncesMeetHalfOfEveryPRFKey now also reads the CREATE_CHILD_SA rekey RESPONSE Nr (respondChildRekey, decrypted by the initiator) and the IKE SA rekey Ni and Nr (initiateIKERekey/respondIKERekey) off the wire; - TestRFC7296EmittedNoncesMeetTheLargestPRFBound is now R1(b), not a repeat: an IKE SA rekey that NEGOTIATES prf-hmac-sha2-512 (checked on the replacement SA's Proposal.PRF), Ni and Nr off the wire each >= half its key | 2 re-recorded (sa.go::GenerateNonce, reach) | enforced | file engine/rfc7296_nonce_size_test.go; approvals D-15 x2 |

Verified (continuation 5): `go test -race -count=1 -tags ze_ike ./internal/component/ike/engine/ ./internal/component/ike/ipsec/` under `./le job run`: both ok (engine 103.6s) after the last edit; gofmt clean.
OWED to main thread: `./le go lint run` on ike/engine + ike/ipsec; `./le rfc check`; reseal (comment lines changed above tagged units: rfc7296_natt_test.go, rfc7296_wp2_test.go; producer doc comments in remote_id.go and ipsec/validate.go); judge + audit-stamp for RFC7296-3.5-5, 3.5-6 (now {feature-declined}), 2.23-13 (new), 2.10-3, and 2.11-2 (negative now held only by TestRFC7296ReplyDoesNotGoToTheConfiguredEndpoint). The rfc7296 enrolment row count moves by one (2.23-13); the gated denominator keeps 3.5-6 as declined.
Approvals (D-15): engine.TestWp2IDTerminatorRefused, engine.TestWp2IDWithoutTerminatorAccepted, engine.TestRFC7296ValidatedPacketFromANewEndpointMovesTheSA and engine.TestRFC7296ValidatedPacketDoesNotMoveTheSAOutsideTheRow (own new units, compile fix), engine.TestNattUnauthenticatedPacketDoesNotMoveTheEndpoint, engine.TestRFC7296EmittedNoncesMeetHalfOfEveryPRFKey, engine.TestRFC7296EmittedNoncesMeetTheLargestPRFBound.

## Files changed (continuation 5)
- internal/component/ike/engine/remote_id.go (doc comment quote only)
- internal/component/ike/ipsec/validate.go (comment quote only)
- internal/component/ike/engine/rfc7296_wp2_test.go (3.5-6 tags dropped)
- internal/component/ike/engine/rfc7296_natt_test.go (tag 2.11-2 -> 2.23-13, comment)
- internal/component/ike/engine/rfc7296_dynamic_update_test.go (new)
- internal/component/ike/engine/rfc7296_nonce_size_test.go (rekey Nr + IKE rekey nonces, R1(b) negative)
- rfc/short/rfc7296.md (3.5-6 {feature-declined}, new 2.23-13, Support remaining discloses the declined ID_RFC822_ADDR send)
- rfc/extraction/rfc7296.json (section 2.23 unsourced-ids + RFC7296-2.23-13, reason)
- rfc/discrimination/rfc7296.json (7 records written; 3 deleted: 3.5-6 x2, 2.11-2 negative on the NAT-T unit)

## Ids left (continuation 6), not started, nothing half-done
RFC4301-4.4.2-1, RFC4555-x-1, RFC7296-1.2-1, 2.21.2-1, 2.21.4-7, 2.4-3, 3.10.1-3, 3.3.6-5, 3.6-2 (recommendations unchanged in the Unresolved table). RFC4303-3.4.3-2: producer is internal/plugins/ospf/ipsec_install.go::buildIPsecSA (OSPFv3 manual SA), not IKE; belongs to the ospf package's author. OWNER-GATE unchanged: RFC7296-3.15.1-2, 2.23-2.

# Continuation 6 (2026-09-30)
| id | resolution | proof (+/-) | records (producer) | expected | notes / files |
|----|-----------|-------------|--------------------|----------|---------------|
| RFC4301-4.4.2-1 | DEFECT (D-8), STOPPED with failing-first test | NEW untagged TestRFC4301InboundSADEntryCarriesTheNegotiatedSelectors (engine/rfc4301_sad_selector_test.go): RED, "inbound SAD entry has no selector". installChildSA (child.go) leaves dataplane.SAParams.Sel nil for every IKE Child SA, so the XFRM state selector is the zero/any value; only the SPD policy carries TSr/TSi/proto/ports. Tags unchanged. | 0 | stays weak until fixed | Not fixed here, design choice: (1) SASelector has no ports (dataplane.go, xfrm_linux.go:163, vpp.go:213 need SrcPort/DstPort); (2) one x->sel holds ONE pair, a multi-pair answer cannot be expressed ("value or values"); (3) transport mode + MOBIKE migrate / NAT-T transport would put outer addresses in x->sel that migrate must rewrite (xfrm_migrate_linux.go). Recommendation: set inbound.Sel from Selectors[0] (Src=TSRemote, Dst=TSLocal, UpperProto=selectorProto, + ports after extending SASelector) for tunnel mode first, prove with the netns XFRM integration (inner packet outside TS dropped by XfrmInStateMismatch), then decide transport mode; the multi-pair limit becomes a {gap} split (R3). Then retag the new test + negative and drop the row's {single-polarity}. |
| RFC4555-x-1 | tests | + NEW TestRFC4555EAPOfferRidesTheIKEAuthMessageCarryingSA (EAP initiator, MOBIKE live: first IKE_AUTH request has SA, no AUTH, MOBIKE_SUPPORTED; later buildEAPResponse request has no SA and no offer; peer answer enables MOBIKE); - NEW TestRFC4555NoOwnOfferNoMOBIKE (initiator that never built its offer, and one with no migrating dataplane whose request carries no offer off the wire: peer's MOBIKE_SUPPORTED does not enable MOBIKE). HEAD tags on TestMobikeAuthNegotiation kept (supplementary). | 2 (mobike.go::mobikeAuthOffer, mobike.go::acceptMobikeOffer; revert/panic) | enforced | file engine/rfc4555_mobike_offer_test.go (new). Responder-side multi-IKE_AUTH (buildAuthResponse fromEAP=true) not driven end to end; the offer append there is unconditional after the SA payload (responder.go:995). |
| RFC7296-3.6-2 | tests (accept half) | + NEW TestRFC7296ReceivedHashAndURLFormatsAreAcceptedWhenConfigured (hash-and-url ON: received encoding 12 (leaf) and encoding 13 (DER bundle leaf+intermediate), http URL on loopback, SHA-1 matched; storeRemoteCerts pending then cached; leaf stored as peer cert, bundle intermediate in chain). Send half + negatives unchanged (TestChuBothHashAndURLFormatsAreConfigurable, TestChuHashAndURLIsOffByDefault). | 1 (cert_payload.go::acceptedCertEncoding, revert) | enforced | file engine/rfc7296_hash_url_accept_test.go (new) |
| RFC7296-2.21.2-1 | tests | + NEW TestRFC7296UnsupportedCriticalPayloadDrawsOnlyThatNotify (protected INFORMATIONAL whose inner chain is private type 200 with C=1: not dispatched, answer = exactly one UNSUPPORTED_CRITICAL_PAYLOAD with data {200}, no other payload). Existing + (truncated chain -> INVALID_SYNTAX) and wire negative (TestCritChainReportsTruncationButNotBadContents) unchanged. | 1 (inbound.go::respondInnerParseError, revert) | enforced | file engine/rfc7296_inner_error_answers_test.go (new, shared with 3.10.1-3) |
| RFC7296-3.10.1-3 | tests | + NEW TestRFC7296UncoveredInnerErrorsDrawInvalidSyntax (Payload Length 0 and 3 in a protected request: not dispatched, exactly INVALID_SYNTAX). Existing +/- on TestErrInnerParseFailureDrawsInvalidSyntaxAndOuterDrawsNothing unchanged. | 1 (same producer, revert) | enforced (judge may still want an invalid-field-value case) | same file |
| RFC7296-2.21.4-7 | row (Correction) + R3 split | row narrowed to "The recipient MUST NOT change the state of any SAs as a result, but may wish to audit the event to aid in diagnosing malfunctions." [MUST NOT] (= extraction 2.21.4:5 quote); its tags (+/- TestErrProtectedInformationalNotifyChangesNoState) unchanged and prove exactly that. NEW row RFC7296-2.21.4-8 [SHOULD] = the send clause, {gap}: Ze starts no protected INFORMATIONAL over the SA on a suspicious message (inbound.go drops/counts). | 0 | enforced (2.21.4-7); 2.21.4-8 gap | rfc/short/rfc7296.md, rfc/corrections/rfc7296.md (Correction paragraph), rfc/extraction/rfc7296.json (2.21.4 unsourced-ids + RFC7296-2.21.4-8, reasons of 2.21.4 and 2.21.4:5). Enrolment row count +1. |

All records route revert (producer body -> panic), OBSERVED red. No existing tagged unit edited, so no D-15 approvals.
Verified (continuation 6): `go test -race -count=1 -tags ze_ike ./internal/component/ike/engine/` under `./le job run` (104.8s): every test passes except the deliberate failing-first TestRFC4301InboundSADEntryCarriesTheNegotiatedSelectors (the 4.4.2-1 defect). gofmt clean. One fixup edit on rfc7296_inner_error_answers_test.go was made with sed (type name errPair, comment unit name), not Edit.
OWED to main thread: `./le go lint run` on ike/engine; `./le rfc check` (+ enrolment count: new row RFC7296-2.21.4-8); judge + audit-stamp for RFC4555-x-1, RFC7296-3.6-2, 2.21.2-1, 3.10.1-3, 2.21.4-7 (corrected), 2.21.4-8 (new gap). Decision: RFC4301-4.4.2-1 defect fix (see row: SASelector ports, tunnel vs transport, multi-pair gap).

## Files changed (continuation 6)
- internal/component/ike/engine/rfc4301_sad_selector_test.go (new, failing-first, untagged)
- internal/component/ike/engine/rfc4555_mobike_offer_test.go (new)
- internal/component/ike/engine/rfc7296_hash_url_accept_test.go (new)
- internal/component/ike/engine/rfc7296_inner_error_answers_test.go (new)
- rfc/short/rfc7296.md (2.21.4-7 narrowed, new 2.21.4-8 {gap})
- rfc/corrections/rfc7296.md (1 Correction paragraph)
- rfc/extraction/rfc7296.json (section 2.21.4 unsourced-ids/reason, site 2.21.4:5 reason)
- rfc/discrimination/rfc4555.json (2 records), rfc/discrimination/rfc7296.json (3 records)

## Ids left (continuation 7), not started, nothing half-done
RFC7296-1.2-1 (D-3 keep; new test over establishPSK capturing all four messages: IKE_SA_INIT SA/KE/Nonce both ways, same Proposal, nonces crossed, same SK_d (DH); IKE_AUTH IDi/IDr/AUTH/SA/TS; negative: a tampered stored IKE_SA_INIT makes AUTH fail; CERT clause needs a cert-mode run, wpcRoleStates in rfc7296_cert_chain_test.go may serve), RFC7296-2.4-3 (ICMP clause: Ze reads no ICMP on the IKE socket -> R3 split or claim; check transport for IP_RECVERR), RFC7296-3.3.6-5 (unknown transform ID beside a known sibling of the same type, and an unknown Transform Attribute, both in crypto proposal selection + engine). RFC4301-4.4.2-1 awaits the fix decision. OWNER-GATE unchanged.

# Continuation 7 (2026-09-30)
| id | resolution | proof (+/-) | records (producer) | expected | notes / files |
|----|-----------|-------------|--------------------|----------|---------------|
| RFC4301-4.4.2-1 | DEFECT fixed (D-8, R21) + marker dropped | + retargeted TestRFC4301InboundSADEntryCarriesTheNegotiatedSelectors (rfc4301_sad_selector_test.go, now tagged): transport -> SAParams.Sel = TSr/TSi/proto/dport 179/any sport; tunnel -> inbound Dir-in tunnel protect policy with the same five, state Sel not asserted. +/- NEW kernel probes (rfc4301_sad_selector_linux_test.go, bare linux, self re-exec in user+net namespace, no root needed, lo disable_policy cleared): tunnel: inside delivered; outside by src addr / dport / proto -> XfrmInNoPols +1, not delivered. transport: inside delivered; dport 6000 and ICMP -> XfrmInStateMismatch +1, not delivered. | 5 (installChildSA x3: unit +, transport +/-; childPolicyParams x2: tunnel +/-; revert, OBSERVED red). Also manual red: with only the Sel block disabled, the transport probe delivered port 6000 and both counters stayed 0, the unit transport subtest failed; restored. | enforced | Fix: child.go installChildSA sets inbound.Sel in transport mode from childPolicyParams(child, SADirIn) with the §4.4.2 quote; dataplane.SASelector gains SrcPort/DstPort (PortMatch); xfrm_linux.go xfrmStateFromParams maps them via xfrmSelectorPort (OPAQUE refused, as for policies). VPP unchanged: it already refuses transport mode and any Sel. Migrate: xfrm_migrate_linux.go refuses a non-tunnel template and copies the old x->sel, so transport x->sel is never migrated. Probe re-exec passes -test.gocoverdir so discriminate-record sees coverage. |
| RFC4301-4.4.2-2 | NEW {gap} row (R3) | none (gap) | 0 | gap | multi-pair answer: only Selectors[0] reaches policy and state selector. rfc/short/rfc4301.md, rfc/extraction/rfc4301.json (4.4.2 reason + unsourced-ids), rfc/corrections/rfc4301.md (Correction paragraph). Enrolment row count +1. |

Files (4.4.2): internal/component/ike/engine/child.go, internal/component/ike/dataplane/dataplane.go, internal/component/ike/dataplane/xfrm_linux.go, internal/component/ike/engine/rfc4301_sad_selector_test.go (retargeted, tagged), internal/component/ike/engine/rfc4301_sad_selector_linux_test.go (new), docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md (new paragraph), rfc/short/rfc4301.md, rfc/extraction/rfc4301.json, rfc/corrections/rfc4301.md, rfc/discrimination/rfc4301.json (5 records), tmp/commit-rfc-approved-01a40e57.md (2 D-15 approvals for the new probe units, rename during authoring).
Verified: go test -race -count=1 -tags ze_ike engine (107.8s) + dataplane + plugins/ospf: all ok. golangci-lint engine+dataplane: 0 issues (before final lo sysctl/coverdir edits; hook lint clean after).

Item 3, stale records re-recorded (all route revert, OBSERVED red): RFC3948-2.1-1 +/- and RFC4303-2.1-1 +/- (producer fsm.go::handleAuthResponse; the 4303 pair were mutant records, now revert), RFC7296-2.23-2 +/- (installChildSA, staled by the 4.4.2 fix), RFC4301-3.2-1 + (installChildSA), RFC4301-4-1, 4.1-8, 4.1-9, 4.4.2.1-1, 4.4.2.1-3 each +/- (dataplane xfrm_linux.go::xfrmStateFromParams, staled by the SASelector port mapping), and the two HEAD-era 4.4.2-1 positive tags that had no record (TestChildSAInboundPolicyUsesNegotiatedTS, TestNarrowedSelectorsReachTheInstalledPolicy; producer childPolicyParams). `./le rfc discriminate stem` now shows 0 stale ike records for rfc3948, rfc4303, rfc7296, and 1 for rfc4301: RFC4301-4.4.1-4 + on rfc4301_spd_discard_test.go::TestSPDPolicyMirrorsTheInboundSelector, an orphan: the tag now sits on rfc4301_spd_order_test.go::TestSpdOperatorOrdersOverlappingPeers, so discriminate-record refuses the old unit; not caused here, left for the judge.
Files (item 3): rfc/discrimination/rfc3948.json, rfc4303.json, rfc7296.json, rfc4301.json.

## Ids left (continuation 8), not started, nothing half-done
RFC7296-1.2-1, RFC7296-2.4-3, RFC7296-3.3.6-5: next steps unchanged from the last paragraph of Continuation 6. Stopped at call 80 per the addendum.
OWED to main thread: `./le go lint run` on ike/engine + ike/dataplane (final state); `./le rfc check` (enrolment count: new row RFC4301-4.4.2-2); judge + audit-stamp RFC4301-4.4.2-1 (fixed, marker dropped) and 4.4.2-2 (new gap). The netns probes need no root (user namespace) and run in the plain `go test` of the engine package.

# Continuation 8 (2026-09-30)
| id | resolution | proof (+/-) | records (producer) | expected | notes / files |
|----|-----------|-------------|--------------------|----------|---------------|
| RFC7296-3.3.6-5 | DEFECT fixed (D-8) + tests | + NEW TestRFC7296UnacceptableTransformLeavesSiblingsOnOffer: IKE offer (wireProposalsToIKE -> crypto.NegotiateIKE, the responder path of responder.go:219 and rekey.go:1065) with ENCR {unknown ID 1024, policy ENCR} and {policy ENCR + unknown attr 99, policy ENCR}, both orders -> accepted with the understood sibling; ESP (matchOfferedESPProposal) the same for ENCR id, ENCR attr, INTEG attr -> matched. - NEW TestRFC7296UnacceptableTransformAloneRefusesTheProposal: unknown ENCR id alone, and each IKE type ENCR/PRF/INTEG/D-H carrying attr 99 alone -> ErrNoProposalChosen (controls without attr accepted); ESP unknown id, ENCR/INTEG/ESN carrying attr 99 alone -> no match (control matches). Failing-first observed: 4 IKE + 3 ESP attribute cases were ACCEPTED before the fix. HEAD tags (crypto TestPropUnknownTransformIDMakesTransformUnacceptable +/-, engine TestAltDHAlternativesAreBothOffered +, TestAltUnusableAlternativesAreStillRefused -) left unchanged as supplementary. | 2 new (appendIKECombinations +, espProposalMatches -; revert, OBSERVED red) + 3 re-recorded, staled by the fix (RFC5282-8-1 +/- espProposalMatches; RFC7296-3.3.2-1 - crypto ikeProposalComplete), OBSERVED red | enforced | Defect: wireEncryptionTransform / espProposalMatches read only Key Length and ignored every other attribute, so a transform carrying an attribute Ze does not understand was accepted, though wire/payload_sa.go keeps such attributes "so negotiation refuses the transform". Fix: wire Transform.AttrsUnderstood (attrFormats is the single source); appendIKECombinations and espProposalMatches drop such a transform (RFC quote above the statement) and refuse the proposal when an offered type is left empty (offered/usable type bitmasks) -- IKE via new crypto IKEProposal.UnacceptableTransformType + ErrTransformUnacceptable in ikeProposalComplete. Also reaches initiator VerifyAcceptedIKE (same reader): a response carrying an unknown attribute is refused. |
Files (3.3.6-5): internal/component/ike/engine/rfc7296_unknown_transform_test.go (new), internal/component/ike/engine/initiator.go, internal/component/ike/engine/responder.go, internal/component/ike/crypto/proposal.go, internal/component/ike/wire/payload_sa.go, docs/architecture/ike/ipsec-6-ikev2-crypto.md (new paragraph), rfc/discrimination/rfc7296.json, rfc/discrimination/rfc5282.json.
| RFC7296-2.4-3 | tests (ICMP clause; no split: Ze does read ICMP) | Ze sets IP_RECVERR on the IKE socket (transport/udp_linux.go installErrorQueue), so ICMP errors DO reach it: transport drops every non-EMSGSIZE entry (udp.go deliverQueuedError, Debug log), and the engine's only ICMP consumer, handleSizeRefusal (probe.go), resends DF-clear and never fails the SA. + NEW TestRFC7296ICMPUnreachableDoesNotFailTheSA (engine/rfc7296_icmp_linux_test.go, real loopback Port Unreachable queued on the SA's socket: SA Established, next request reaches peer, nothing on Refusals, peer's authenticated answer settles the path probe as fits). - NEW TestRFC7296ICMPFromThePeerWhileWaitingFailsNothing (probe outstanding; peer socket closed -> real Port Unreachable from the peer's own addr:port, drained by the next send, which succeeds; then a Fragmentation Needed for the peer into handleSizeRefusal: SA Established, pendingProbe kept, no sa-failed answer; peer's later answer settles it). HEAD tags (responder_test.go x2, rfc7296_routing_test.go +/-) unchanged: they prove the unprotected-message clause. | 2 (transport udp_linux.go::drainErrorQueue +, udp.go::deliverQueuedError -; revert, OBSERVED red with the panic inside the named producer, so the real ICMP path runs) | enforced | no code change, no doc change (behavior unchanged). File: internal/component/ike/engine/rfc7296_icmp_linux_test.go (new, linux build tag, no privilege), rfc/discrimination/rfc7296.json. |
| RFC7296-1.2-1 | tests (D-3 keep; CERT clause covered by an X.509 run, no split) | + NEW TestRFC7296InitialExchangesCarryWhatSection12Names (engine/rfc7296_initial_exchanges_test.go; autLoadPKI + autSAInitPair in X.509 mode, then the production handleAuthRequest/handleAuthResponse; all four messages captured): IKE_SA_INIT req/resp each SA+KE+Nonce; resp's single proposal is one the request offered, ini.Proposal == resp.Proposal; Ni/Nr held crosswise and distinct; both KE name the negotiated group, each side holds the other's public value, same SK_d; IKE_AUTH req IDi+CERT+AUTH+ESP SA+TSi+TSr, resp IDr+CERT+AUTH+one ESP proposal+TSi+TSr; each side records the other's ID; both Established; initiator holds NegotiatedTSi/TSr. - NEW TestRFC7296TamperedIKESAInitFailsIKEAuth (PSK and X.509: one flipped octet in the responder's stored IKE_SA_INIT request, or the initiator's stored response, keeps that side off Established; untouched control establishes both). HEAD +/- on TestInitialExchangeEncryptionBoundary unchanged (encryption clause). | 2 (responder.go::handleSAInitRequest +, auth.go::verifyRemoteAuth -; revert, OBSERVED red) | enforced | no code change. |

Verified (continuation 8): `go test -race -count=1 -tags ze_ike` on ike/engine (108.5s), ike/crypto, ike/wire, ike/transport under `./le job run`: all ok. golangci-lint (build tag ze_ike) engine+crypto+wire: 0 issues. gofmt clean. No existing tagged unit edited, so no D-15 approvals. No row changed, no new row.
Weak rfc7296 verdicts still in rfc/audit/rfc7296.json that tag ike/engine: 7 = 1.2-1, 2.4-3, 3.3.6-5 (this continuation, await judge), 3.3.2-1 (tests, earlier continuation, await judge), 3.5-6 (R19 row, await judge), 2.23-2 (Correction; also OWNER-GATE list), 3.15.1-2 (OWNER-GATE, R1 fallback). No ike/engine rfc7296 id is left unauthored except the two owner-gate ones.
OWED to main thread: `./le go lint run` over ike (final state); `./le rfc check`; judge + audit-stamp for RFC7296-1.2-1, 2.4-3, 3.3.6-5 (defect fixed) and the three re-recorded ones (RFC5282-8-1 +/-, RFC7296-3.3.2-1 -), whose producers changed.

## Files changed (continuation 8)
- internal/component/ike/engine/rfc7296_unknown_transform_test.go (new)
- internal/component/ike/engine/rfc7296_icmp_linux_test.go (new)
- internal/component/ike/engine/rfc7296_initial_exchanges_test.go (new)
- internal/component/ike/engine/initiator.go (appendIKECombinations: RFC 3.3.6 drop + emptied-type refusal; math/bits import)
- internal/component/ike/engine/responder.go (espProposalMatches: same)
- internal/component/ike/crypto/proposal.go (IKEProposal.UnacceptableTransformType, ErrTransformUnacceptable, check in ikeProposalComplete)
- internal/component/ike/wire/payload_sa.go (Transform.AttrsUnderstood)
- docs/architecture/ike/ipsec-6-ikev2-crypto.md (new paragraph)
- rfc/discrimination/rfc7296.json (6 new + 1 re-recorded), rfc/discrimination/rfc5282.json (2 re-recorded)
Nothing half-done.
