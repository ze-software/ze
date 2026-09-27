# Spec: rfc-requirement-quote-hand-backfill

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 5/5 |
| Handoff | - |
| Updated | 2026-09-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Closed spec `spec-rfc-requirement-verbatim-quote` made every requirement
row in `rfc/short/<stem>.md` carry the RFC sentence it states, and
`./le rfc check` refuses a new or changed row whose quote is not in its section.
Its backfill (`./le rfc quote-backfill stem <stem> apply`) quoted 1821 rows
mechanically on 2026-09-26. This spec quotes every row the backfill could not.

The work ends when three things hold:

1. `./le rfc check` prints `unquoted: 0`, and no stem is named unjudged.
2. The row-quote rule applies to every row in the corpus, not only to a row a
   commit adds or changes. With the count at zero, the change scope has nothing
   left to protect.
3. Each row the backfill rewrote has had its tagged tests re-read against the
   new sentence (risk R-7 of the source spec). A test that proves less than the
   sentence now states is a finding: the test or the row is corrected, never
   the sentence.

### Rows left to quote

Figures from the backfill dry run over 201 stems (6354 rows), 2026-09-26. After
the apply, `./le rfc check` counted 3696 unquoted rows over 188 stems. The
difference from 3706 is the 10 `no-rfc-text` rows: rfc8326 (3) and rfc9129 (7)
have no text in `rfc/full/`, so the check names them unjudged and counts none
of their rows.

| Bucket | Kind | Rows | What a human does |
|--------|------|------|-------------------|
| review | partial | 256 | the sentence carries more obligations than the row; quote the whole sentence or split the row |
| review | number-absent | 217 | a number in the row is not in the sentence; find the sentence that carries it |
| review | qualified | 167 | the sentence qualifies the obligation; quote it with the qualifier |
| review | several-sites | 140 | the row maps to several sentences; pick one, or split the row |
| review | low-overlap | 108 | the paraphrase shares under half its content words with the sentence; check the mapping first |
| review | polarity-differs | 102 | NOT or MUST NOT differs between row and sentence; the row or the mapping is wrong |
| review | unresolved-anchor | 89 | the row's section does not resolve in the RFC text; correct the section |
| review | lead-in | 76 | the site is a lead-in to a list; quote the list item that states the obligation |
| review | outside-section | 69 | the sentence is not in the row's section; correct the section or the mapping |
| review | level-differs | 40 | the sentence's keyword differs from the row's level; correct the level or the mapping |
| review | short-sentence | 7 | the sentence is under the minimum length; quote the sentence with its context |
| human | unsourced | 1511 | the extraction declares the row has no source site; find the sentence, or retire the row |
| human | unmapped | 556 | no extraction site maps to the row; find the sentence |
| human | no-extraction | 358 | the stem has no extraction artifact; find each sentence by hand |
| human | no-rfc-text | 10 | fetch `rfc/full/rfc8326.txt` and `rfc/full/rfc9129.txt`, then quote |
| | **Total** | **3706** | review 1271, human 2435 |

### The review list

The per-stem list is not committed. It is derived: `./le rfc quote-backfill stem
<stem>` without `apply` writes nothing and lists every row it would not quote,
with its kind and reason. Run it for each stem in `ls rfc/short` to regenerate
the whole list over the tree in hand. A committed copy would be stale after the
first row quoted.

### Tagged tests to re-read (R-7)

Overlap cannot see a paraphrase whose words match and whose meaning differs.
The source spec measured about 1 in 15 applied rows as such. Three are known and
are read first:

| Row | What changed |
|-----|--------------|
| RFC7432-6.3-3 | the paraphrase named the normalized VID, the sentence names the originating VID |
| RFC9552-5.2-6 | the quoted sentence states a different obligation from the paraphrase |
| RFC5880-6.7.3-4 | the quoted sentence states a different obligation from the paraphrase |

Found by the rfc7606 re-audit (2026-09-26): four verdicts moved from enforced to
weak once the row states the full RFC sentence. Each note in
`rfc/audit/rfc7606.json` names the tag that moves it back; adding a tag owes a
discrimination record (`ai/rules/rfc-compliance.md`).

| Row | Clause no tagged unit drives |
|-----|------------------------------|
| RFC7606-3.g-2 | "whether recognized or unrecognized": both tagged units duplicate ORIGIN only |
| RFC7606-4-1 | the second §4 case, fewer than three octets remaining |
| RFC7606-7.10-2 | the zero-length half of "non-zero multiple of 4" |
| RFC7606-7.14-1 | the length-5 case; `TestRFC7606ExtendedCommunityLength` tests it but carries no tag |

Un-enrolled stems (AC-7, A-6), read 2026-09-27 under the STRICTNESS rule of the
strict re-audit brief. These 37 tagged rows changed text in or after 0bf0696576 in
stems that have no `rfc/audit/<stem>.json`, so this table is their only record.
The first 21 read changed after the backfill; the 16 added by job AC7-E
(rfc1035 3, rfc9190 13) changed in 0bf0696576 itself.
"Weak" means a clause of the quote has no assertion that goes red when that
clause is broken.

| ID | Verdict | Unit | What the test fails to prove |
|----|---------|------|------------------------------|
| DRAFT-IETF-SIDROPS-8210BIS-5.12-1 | weak | `TestParseASPAPDU`, `TestParseASPAPDUMalformed` (`internal/component/bgp/plugins/rpki/rtr_pdu_test.go`) | the second sentence: both units assert the parser sentinel `errASPAProviderList`. None asserts that the router returns an Error Report PDU with Error Code 9. The mapping in `RTRSession.readLoop` (`rtr_session.go`) is not driven by a tagged unit |
| DRAFT-IETF-SIDROPS-8210BIS-5.12-3 | weak | `TestParseASPAPDU`, `TestParseASPAPDUUnsorted` (`rtr_pdu_test.go`) | ascending order and uniqueness are proven in both polarities. The "zero or more" clause (a withdrawal carries no provider) is driven only by `TestParseASPAPDUWithdraw`, which carries no tag |
| DRAFT-IETF-SIDROPS-8210BIS-7-1 | enforced | `TestRTRSessionStartsAtV2` (`rtr_session_test.go`) | nothing missing: the first query on the wire is a Serial Query at version 2, and the row's `{single-polarity}` marker states why no negative exists |
| DRAFT-IETF-SIDROPS-8210BIS-7-2 | weak | `TestRTRUnknownNegotiationVersion` (`rtr_session_test.go`) | the exception "unless the received PDU is itself an Error Report PDU": no case feeds an Error Report carrying an unrecognized version and asserts that no Error Report goes back. `RTRSession.readLoop` guards it (`hdr.Type != pduErrorRpt`), untested |
| RFC1035-2.3.4-1 | weak | `TestRFC1035_ConfiguredTTLBoundedToASigned32BitPositive` (`internal/plugins/geodns/rfc1035_rr_test.go`) | the upper bound is proven both ways (2147483648 refused on three leaves, 2147483647 served). The lower bound of "positive" is driven by no tagged unit, and the sibling 4.1.3-1 negative serves TTL 0, which the word "positive" excludes. The row does not cite the RFC 2181 Section 8 reading (0 to 2^31-1) that would admit it |
| RFC1035-2.3.4-2 | enforced | `TestRFC1035_UDPReplyBoundedAndTruncated` (`internal/core/dnsserver/rfc1035_handler_test.go`) | nothing missing: an oversized reply packs to at most 512 octets, and a reply under the bound keeps all three answers |
| RFC1035-3.1-4 | enforced | `TestRFC1035_ConfiguredNameBoundedTo255WireOctets` (`internal/plugins/geodns/rfc1035_name_limits_test.go`), `test/parse/dns-name-too-long.ci` | nothing missing: 256 wire octets refused and 255 accepted, over the synthesized glue name and through `ze config validate` |
| RFC1035-4.1.1-1 | weak | `TestRFC1035_ReservedZFieldIsZero` (`internal/core/dnsserver/rfc1035_header_test.go`) | "in all queries": both polarities hold Z clear in responses only (an answer func that sets Z, a query that arrives with Z set). Ze also sends queries, from `internal/component/resolve/dns/resolver.go` (`SetQuestion`) and the as112 health probe (`internal/plugins/as112/health.go`), and no tagged unit asserts Z is zero in either |
| RFC1035-4.1.1-2 | enforced | the AA test in `internal/core/dnsserver/rfc1035_header_test.go`, `TestZoneAnswer_ResponseCodeByNamePosition` (`internal/plugins/as112/zones_test.go`), the negative-answer table test in `internal/plugins/geodns/rfc1035_negative_test.go` | nothing missing: AA set for names in a served zone, clear for names outside it. The dnsserver negative asserts the neighbouring RD bit, and the two plugin tables carry the AA negative |
| RFC1035-4.1.1-3 | enforced | `TestZoneAnswer_ResponseCodeByNamePosition` (as112), the negative-answer table test (geodns) | nothing missing: RCODE 3 for a name absent from a served zone, NOERROR for names that exist, REFUSED where Ze is no authority |
| RFC1035-4.1.3-1 | enforced | `TestRFC1035_RecordTTLIsA32BitUnsignedSecondCount` (`rfc1035_rr_test.go`) | nothing missing: the configured TTL (120, 2147483647, 0) reaches the wire unchanged. The test comment quotes the Section 3.2.1 wording ("before the source of the information should again be consulted"), not the Section 4.1.3 sentence the row quotes, so the comment is stale |
| RFC1035-4.1.3-2 | enforced | `TestRFC1035_RDLengthCountsTheRDataOctets` (`rfc1035_rr_test.go`) | nothing missing: RDLENGTH equals the RDATA octets for A (4), AAAA (16) and a variable-length SOA |
| RFC1035-4.1.4-1 | weak | `TestRFC1035_CompressionPointersInATruncatedDatagram` (`internal/plugins/geodns/rfc1035_compression_test.go`) | "the label must begin with two zero bits": `pointerTargets` treats any length octet without both top bits set as a label, so a label length octet of 0x40 to 0xBF (a label over 63 octets) passes both polarities. The negative asserts only that the stream reply holds no 0xC0 octet, not the "two zero bits" its comment claims |
| RFC1035-4.1.4-2 | enforced | the compression test in `internal/plugins/geodns/rfc1035_compression_test.go` | nothing missing: every pointer offset lands inside the message and past the header, and the names it expands to are the queried zone |
| RFC1035-4.1.4-4 | enforced | the compression test in `rfc1035_compression_test.go` | nothing missing: the NS RDLENGTH is below the expanded name length on the compressed datagram, and the uncompressed stream reply is longer |
| RFC1035-4.1.4-5 | weak | `TestRFC1035_InboundCompressionPointerUnderstood` (`rfc1035_compression_test.go`) | "understand arriving messages that contain pointers": the pointer sits in the additional TXT record's owner name and the question name is uncompressed, so the SOA answer does not depend on the pointer being expanded, and no assertion reads what it expands to. The client side, replies Ze reads in `resolve/dns/resolver.go` and the as112 health probe, is driven by no tagged unit |
| RFC1035-4.2.1-1 | enforced | `TestRFC1035_UDPReplyBoundedAndTruncated`, `TestRFC1035_UDPBoundFollowsAdvertisedEDNSSize` (dnsserver), the transport test in `internal/plugins/geodns/rfc1035_server_transport_test.go` | nothing missing: the datagram read off a real socket is at most 512 octets. The negative rests on the RFC 6891 advertised size, a neighbouring rule, but the positive holds the bound at the handler and at the socket |
| RFC1035-4.2.2-1 | weak | `TestRFC1035_TCPRepliesCarryATwoOctetLengthPrefix` (`rfc1035_compression_test.go`) | the first sentence, "use server port 53": the server listens on a `freePort` port (`serveCompressionZone`), and no tagged unit asserts that the TCP listener defaults to 53. The two-octet length prefix is proven both ways |
| RFC8362-2-1 | weak | `TestRFC8362ExtendedLSAsSetUBitOnTheWire`, `TestRFC8362BaseLSAsKeepUBitClearOnTheWire` (`internal/plugins/ospf/rfc8362_test.go`) | the U-bit is asserted on the E-Router-LSA (0xA021) and the E-Intra-Area-Prefix-LSA (0xA029) only. The third Extended LSA Ze originates, the E-Inter-Area-Prefix-LSA, is not checked, as the stem's enrolment reason already records |
| RFC9190-1-1 | weak | `TestEAPTLSCapsBothRolesAtTLS13`, `TestEAPTLSVersionCapLeavesTLS12Reachable` (`internal/core/eap/rfc9190_version_cap_test.go`) | the positive holds `MaxVersion` at TLS 1.3 on both roles and reads the peer's offer. The tagged negative proves a neighbouring rule (the cap is not a pin, TLS 1.2 still completes). No unit shows a version above 1.3 refused, and the row carries no `{single-polarity}` marker saying why none can |
| RFC9190-2.1.2-1 | enforced | `TestEAPTLS13IssuesASessionTicketTheNextExchangeRedeems`, `TestEAPTLS13ResumptionOffRunsAFullHandshakeAndStillIssuesATicket` (`internal/core/eap/rfc9190_resumption_test.go`), `TestEAPTLS13IssuesNoTicketToAClientThatOffersNoPSKMode` (`rfc9190_resumption_refusal_test.go`) | nothing missing: the initial authentication stores a non-empty ticket the next exchange redeems, a server with resumption off still issues a fresh one, and a client offering no PSK mode gets none |
| RFC9190-2.1.2-2 | weak | `TestEAPTLS13IssuesASessionTicketTheNextExchangeRedeems` (`rfc9190_resumption_test.go`) | the bound rests on crypto/tls's client refusing a lifetime over 604800: no assertion reads the lifetime the ticket declares, there is no negative, and no `{single-polarity}` marker |
| RFC9190-2.1.3-1 | weak | `TestEAPTLS13IssuesASessionTicketTheNextExchangeRedeems`, `TestEAPTLS13ResumptionOffRunsAFullHandshakeAndStillIssuesATicket` (`rfc9190_resumption_test.go`) | the positive reads `Resumed()` on a flight driven at TLS 1.3 and never the resumed session's version. The negative proves a neighbouring rule (declining resumption falls back to a full handshake); no unit refuses a mechanism that is not the TLS 1.3 one |
| RFC9190-2.1.8-2 | weak | `TestEAPTLS13PeerSendsAnAnonymousNAIAndKeepsTheRealm`, `TestEAPMSCHAPv2PeerSendsItsConfiguredIdentity` (`internal/core/eap/rfc9190_nai_test.go`), `TestEAPTLSPeerDropsTheUsernameTheCertificateCarries` (`rfc9190_cert_nai_test.go`) | "(or any other permanent identifiers)": the tagged units search the wire for the username only. The identity with no realm and the IP-address identity are driven by units tagged 2.1.8-5 and 2.1.8-3, not by a 2.1.8-2 unit |
| RFC9190-2.1.8-3 | weak | `TestEAPTLSPeerAnonymizesEveryConfiguredIdentity`, `TestNAIGrammarMatchesRFC7542Section22` (`rfc9190_nai_test.go`) | the grammar is proven both ways over `anonymousNAI`. The NAI the peer derives from a certificate (`certificateNAIs`, `realmNAI` in `nai.go`) is never run through `validNAI` by a tagged unit, so the positive's "every NAI the peer can emit" is not what it checks |
| RFC9190-2.1.8-4 | weak | `TestEAPTLS13AuthenticatorTreatsAnEmptyCertificateListAsTerminal` (`rfc9190_nai_test.go`) | the sentence binds both the peer and the server to TLS 1.3 processing. The one unit drives the server's handling of an empty certificate_list; nothing drives the peer's processing |
| RFC9190-2.1.8-5 | wrong | `TestEAPTLS13PeerSendsTheFixedUsernameWhenTheIdentityHasNoRealm` (`internal/core/eap/rfc9190_nai_test.go`) | the unit proves the fixed-username construction the next clause allows, for an identity with no realm. The recommended form, "@realm", is proven by `TestEAPTLS13PeerSendsAnAnonymousNAIAndKeepsTheRealm`, which is tagged only RFC9190-2.1.8-2 |
| RFC9190-2.1.9-2 | enforced | `TestEAPTLSAcceptsAnUnfragmentedMessageWithAndWithoutTheLengthBit`, `TestEAPTLSRefusesAnUnfragmentedMessageThatContradictsItself` (`internal/core/eap/rfc9190_fragmentation_test.go`) | nothing missing: both shapes are accepted and yield the same octets, and self-contradicting shapes are refused |
| RFC9190-2.3-1 | weak | `TestRFC9190MSKIsTheExportUnderTheRFCLabel` (`internal/core/eap/rfc5216_msk_label_test.go`) | Key_Material is proven (MSK is the first 64 octets of the export, and a wrong context changes it). Method-Id is not: no non-test code in `internal/core/eap` derives `EXPORTER_EAP_TLS_Method-Id` or a Session-Id, so that clause is an implementation gap to record under R-7 |
| RFC9190-2.5-1 | enforced | `TestEAPTLS13SendsProtectedSuccessIndication`, `TestEAPTLS13RefusedClientGetsNoSuccessIndication`, `TestEAPTLS12SendsNoProtectedSuccessIndication` (`internal/core/eap/rfc9190_test.go`), plus the resumption test and the interoplab checker | nothing missing: one application_data record decrypting to 0x00 sits in the Request before EAP-Success, EAP-Success is the last packet, and neither a refused client nor TLS 1.2 gets one |
| RFC9190-5.4-1 | weak | the eight tagged units in `internal/core/eap/rfc9190_revocation_test.go`, `TestEAPTLS13ResumptionStillNeedsARevocationSource` (`rfc9190_resumption_refusal_test.go`), `checkResponderEAPTLS13RevokedClient` (`internal/le/interoplab/ipsec/checkers.go`) | "all the certificates in the certificate chains": the authenticator's check is proven over the client leaf, a client intermediate, a missing list and a stale list, and the trust anchor exception holds. On the peer only the authenticator's leaf is driven. A revoked intermediate in the authenticator's chain is driven only by a 5.4-3 OCSP unit, and no unit shows the peer refusing when it holds no current list |
| RFC9190-5.4-2 | weak | `TestEAPTLS13StaplesTheConfiguredOCSPResponse`, `TestEAPTLS13StaplesNothingWhenTheCertificateCarriesNoResponse` (`internal/core/eap/rfc9190_ocsp_test.go`) | the configured staple reaches the peer on TLS 1.3 and nothing is stapled when none is configured. The RFC 8446 Section 4.4.2.1 half, no status sent to a client whose ClientHello carried no status_request, is driven by no unit, because every client in the suite is crypto/tls, which always offers it |
| RFC9190-5.4-3 | weak | the thirteen tagged units in `internal/core/eap/rfc9190_ocsp_test.go` | "abort the handshake with an appropriate alert": the units assert that no EAP-Success arrives and read the peer's error text. None reads the alert the authenticator receives. The comment's claim that crypto/tls turns the error into bad_certificate is not asserted |
| RFC9190-5.7-1 | weak | `TestEAPTLS13ResumptionRefusesARevokedClientChain` (`rfc9190_resumption_test.go`), `TestEAPTLS13TicketIsNotRedeemableUnderAnotherPeeringsKey` (`rfc9190_resumption_refusal_test.go`) | the positive asserts no EAP-Success, and a full handshake presenting the same revoked certificate fails the same way: it never asserts the exchange resumed or that no Certificate crossed. Neither tagged unit shows a resumption authorized on valid cached data; `TestEAPTLS13CompletesAResumptionWithAnUnrevokedChain` does, tagged 5.7-2 and 5.7-6 only |
| RFC9190-5.7-5 | weak | `TestEAPTLSPeerDropsAStoredTicketAtTheSection57Ceiling` (`rfc9190_resumption_test.go`), `TestEAPTLS13RefusesATicketPastTheSection57Lifetime` (`rfc9190_resumption_refusal_test.go`) | "regardless of the PSK or ticket lifetime": the stored ticket is crypto/tls's, whose declared lifetime is itself 604800 seconds, so a store that expired entries on the ticket's own lifetime passes both halves. No case stores a ticket declaring another lifetime |
| RFC9190-5.7-6 | enforced | `TestEAPTLS13ResumptionRefusesARevokedAuthenticatorChain`, `TestEAPTLS13CompletesAResumptionWithAnUnrevokedChain` (`rfc9190_resumption_test.go`), `TestEAPTLS13RefusesAResumptionWhoseCachedCertificateExpired` (`rfc9190_resumption_refusal_test.go`) | nothing missing: changed information (a revocation on the peer, an expiry on the authenticator) is reevaluated at resumption, and unchanged information resumes. `internal/core/eap` makes no accounting decision, so that alternative binds nothing |
| RFC9384-4-1 | enforced | `TestNotificationRefusedBySocketStillRecordsTheReason`, `TestNotificationDeliveredIsNotRecordedAsUnsent` (`internal/component/bgp/reactor/peer_last_error_test.go`), `TestBgpSummaryLastErrorSeparatesToldFromCouldNotTell` (`internal/component/bgp/plugins/cmd/peer/last_error_unsent_test.go`) | nothing missing: an unsent Cease/BFD Down stays in Stats and in `show bgp peer` as "(not sent)", and a delivered one does not |

Counts: 15 enforced, 21 weak, 1 wrong (the first 21: 13, 7, 1; AC7-E's 16:
2 enforced, 14 weak). D-15 homes weak and wrong verdicts in
`plan/pre-release/spec-rfc-verdict-test-fix-pass.md`. These 22 live only in this
table, so that spec names each of them in its "Un-enrolled R-7 rows" table and
its goal covers them. RFC9190-2.3-1 also records an implementation gap: Ze
derives no Method-Id.

The set to read is every row whose text changed in the backfill commit and whose
id a test tags (`RFC requirement:` tag). `git diff <backfill-commit>^ <backfill-commit> -- rfc/short`
names the rows.

### Owner decisions (2026-09-26)

| ID | Decision |
|----|----------|
| D-0 | One spec, run in phases: first the tooling, then the R-7 re-read, then the review bucket, then the human buckets stem by stem, then the rule flip. Each phase commits on its own |
| D-0b | Unsourced rows in prose-register stems are quoted here as well. The strength-4 blind walk later compares against the quoted rows |
| D-1 | In a stem with no numbered heading at all, the whole text is one citable section. A stem that has numbered sections still refuses a citation of the front matter |
| D-2 | A row that no RFC sentence states is retired through a dated paragraph in `rfc/corrections/<stem>.md`. That paragraph says why no sentence states the row. The tags move off it first, and the id is never reused |
| D-4 | Design approved. In every stem, a second agent that did not do the quoting re-derives a blind 10% sample, with at least one row. Any disagreement sends the whole stem back to be read again. No tool proposes a candidate sentence |
| D-5 | rfc905 has numbered headings indented five spaces, which the column-0 `sectionHeadingRE` misses, so D-1 would have made it a whole-text stem. The heading detector instead learns indented headings for a text with no column-0 heading, so rfc905 gets real sections. Its inventory site ids and extraction artifact change and are re-walked in phase 1 |
| D-5a | Main-thread call (2026-09-26), inside D-5. rfc905 rows that cite "(Annex B.N)" parse to section `x` (`sectionRE`, `summary.go`), so they are unresolved-anchor now that rfc905 has sections. They are rewritten to cite §B.N in the quoting phases, like the other 257 `x` rows. `sectionRE` does not learn "Annex", because that would change row parsing across the whole corpus for seven rows |
| D-6 | Main-thread call (2026-09-26), after the owner asked for every phase to be completed. Phases 2, 3 and 4 run as one pass per stem, so each stem is read and committed once: it quotes the rows and re-reads the tagged tests of every row whose text changed, in the backfill or in the pass. Test fixes for weak, wrong or unimplemented verdicts are not done in that pass. A follow-up pass groups them by package, so two stem agents never edit the same test file. New verdicts are stamped by `./le rfc audit-stamp stem <stem>`, which only fills a verdict that has no `requirement_sha` yet, because the skill forbids hand-computed hashes and no command existed |
| D-7 | Main-thread call (2026-09-26), from the rfc1877 pilot. Some rows state an obligation that another document states and already carries as its own row: RFC1877-x-3 to x-5 restate RFC 1661 §5.2 and §5.4. These are retired, and their tags move to the row that states the obligation, so nothing is lost. A row whose obligation has no row in its real document stays unquoted and is reported as a re-attribution case, because the gates refuse moving a row (`spec-rfc-requirement-reattribution`). The A-2 15% stop applies to stems of 20 or more rows. Below that, one row is already 5 to 20%, so each retirement is reported instead |
| D-8 | Owner decision (2026-09-26). A verified defect in an implemented capability that a re-read exposes is FIXED IN CODE under this spec: a failing test first, then the fix, with interop where it applies. It is not recorded as a `{gap}` instead. This came up because the gates refuse a `{gap}` on a row whose other clauses are tested. Where a fix proves impossible (a library limit, for example), it goes back to the owner |
| D-9 | Main-thread call (2026-09-26). The blind sample found about 28% of first-pass test verdicts lenient, `enforced` where the tests prove less. Two changes follow. Every verdict written before the STRICTNESS rule is re-audited strictly, stem by stem. The verdict sample rises from 10% to 20% of each stem's verdicts |
| D-10 | Owner decision (2026-09-27): some rows state an obligation their own RFC does not, but another document does. If that document has a summary, the missing row is added there and the tags move to it. If it has none, the row is retired with a correction naming the document, and a journal row is added to enrol that document later |
| D-11 | Owner decision (2026-09-27): the gate learns errata. Verified errata text is stored under `rfc/errata/<stem>/`, and a row that cites an erratum quotes that text verbatim. This covers RFC9568-7.1-5 (erratum 8301) |
| D-12 | Owner decision (2026-09-27), RFC 3101 §3.2 translator election: a translator is a router with the B-bit in the NSSA, and "equivalent" means the same mask, the same metric and the same non-zero forwarding address. Changing the tagged tests for this is approved under D-8 |
| D-13 | Owner decision (2026-09-27), revised the same day. The first ruling was to follow the draft with a bare UTF-8 Capability Value. That broke interop with FRR: FRR 10.3.1's bgp_capability_software_version reads the first octet as a length and tears the session down, and FRR and ExaBGP both send the length octet. Final ruling: support both forms. Ze decodes both: the length-prefixed form when the first octet equals the remaining length and the rest is valid UTF-8, otherwise the draft's bare string. The format Ze sends is a per-peer config choice, `draft` or `legacy`, defaulting to `draft` (owner's choice of default). A peer talking to FRR or ExaBGP must set `legacy`, and the docs and the config help say so. `legacy` is recorded as an owner-approved deviation from the draft |
| D-14 | Owner decision (2026-09-27), RFC5880-6.7.3-12: follow the RFC literally and seed bfd.RcvAuthSeq before the digest check. TestRFC5880ForgedFirstPacketDoesNotSeedReplayFloor is rewritten under this approval. This knowingly accepts that one forged first packet can pin the replay floor, since nothing clears it while 6.8.1-13 is a gap |
| D-15 | Owner decision (2026-09-27). With every row quoted (unquoted 0, from 3706), the strict re-read left 894 verdicts where the tests prove less than the sentence: 838 weak, 56 wrong, across 135 stems. Fixing them (test or row) moves to a separate spec, run by package. This spec closes after the phase 5 rule flip. The ledger already records each one as weak or wrong, so no claim outruns the evidence. Main-thread call inside D-15: the split-needed rows (about 80, from the stem reports) and the narrowing audit go to the same spec, because each split adds a row that owes tests |
| D-3 | A row backed only by prose without an RFC 2119 keyword, or by a lowercase "must", is quoted verbatim and keeps its level. The level changes only when the sentence carries a different 2119 keyword. A demotion from MUST carries its correction paragraph |

## Required Reading

- `spec-rfc-requirement-verbatim-quote`, closed: its closure commit holds the
  spec (the rule, the backfill kinds, R-7, A-6); the rule and the kinds live on
  the page below
- `docs/contributing/rfc-conformance-gates.md`, "The row quote"
- `ai/rules/rfc-compliance.md`

## Current Behavior (MANDATORY)

Source files read so far. The design phase completes this list.

- [ ] `internal/le/rfc/quote_backfill.go` - `quoteBackfill` judges each row and names the kind that kept it unquoted
  → Constraint: `judgeBackfillRow` returns an empty quote for unmapped, unsourced and no-extraction rows, so 2423 rows get no candidate sentence today. `sitesFor` (inventory.go) plus `backfillOverlap` could propose one, but only if it matches every sentence and not just `siteKeywordRE`: an unsourced row has no keyword sentence by definition
- [ ] `internal/le/rfc/check_quote.go` - `checkRowQuotes` judges only rows the tip commit adds or changes; `unquotedFigures` prints the tree figure
  → Constraint: `unquotedFigures` already runs `rowQuoteRefusal` over every row on every `./le rfc check`, so the whole-corpus rule costs no extra pass. Fuse the two passes so each stem's `newQuoteSource` is built once
  → Decision: end condition 2 deletes the change scope rather than widening it (no-layering). What goes: `quoteRevisions`, `readQuoteRevisions`, `quoteChangedStems`, `quoteRowsAt`, `quoteSourceAt`, `scopeChangedRows`, `checkUnquotedRatchet`, `CheckReport.QuoteHistoryUnread` and its printed line. `stemPath` and `gitCatBlobs` stay because they have other callers
  → Constraint: once every row is judged, the fixture RFC text in `check_test.go:checkFixtureTree`, `selftest_state.go` and `selftest_core.go` must hold every fixture row verbatim. Rows such as "A speaker SHOULD count widgets (§2)" would otherwise add a violation to every `Check()` test. The fixture text grows; the assertions are not weakened. Tests that pin the scope get rewritten: `TestCheckCountsUnchangedUnquotedRow`, `TestCheckRefusesUnquotedCountRise*`, `TestCheckRefusesNewRowNotVerbatimInSection` (its violation count), `TestCheckNamesStemWithoutRFCTextUnjudged`
- [ ] `internal/le/rfc/inventory.go` - `sectionHeadingRE`, `quoteSource.has`
  → Constraint: only numbered or lettered headings are sections, and front matter can never be cited. In seven stems, rfc792, rfc2347, rfc2348, rfc2349, rfc1997, rfc2782 and rfc905 (69 rows), all the text is front matter, so no hand quote can pass there until the tooling changes
- [ ] `internal/le/rfc/freshness.go` - `verdictFreshness` compares an audit verdict's `requirement_sha` against `RequirementSHA(req.Text)`
  → Constraint: rewording a row stales its audit verdict, which is a violation (`check_audit.go:checkAuditFreshness`). Only rfc7606 (65 verdicts) and draft-abraitis-idr-addpath-paths-limit (9) have audit files. A reworded row in either one is re-audited in the same commit
- [ ] `internal/le/rfc/signoff.go`, `discriminate.go:claimSHA`, `render_ledger.go`
  → Constraint: none of these reads row text. The extraction sign-off keys on RFC-text SHA and ids, discrimination records key on the test tag's prose, and the status page keys on counts. A hand reword stales none of them
- [ ] `internal/le/rfc/check_ratchets.go` - `checkRetiredRequirements`, `checkIDAllocation`, `checkLevelRatchet`
  → Constraint: deleting an id that `HEAD^` held in an enrolled stem is refused, and no documented route retires a fabricated row. A section fix under the same id is allowed. A level demotion from MUST needs a "Correction <date>:" paragraph in `rfc/corrections/<stem>.md`

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

Rows enter as lines of `rfc/short/<stem>.md`, parsed by the `reChecklist` grammar (`rfc.go`) into a `Requirement`. RFC text enters from `rfc/full/<stem>.txt` or `rfc/drafts/`, through `SourceText`.

### Transformation Path

1. `Requirement.Quote` (`summary.go`) peels the trailing `{...}` markers and the section parenthetical off the row, which leaves the quote.
2. `newQuoteSource` (`inventory.go`) strips the page furniture, splits the text into section bodies at `sectionHeadingRE` and squashes whitespace.
3. `rowQuoteRefusal` (`check_quote.go`) resolves the cited section to its nearest heading ancestor, then matches the quote in that section and its subsections.
4. `checkRowQuotes` reports the refusals as violations; `unquotedFigures` counts them as the backlog.

### Boundaries Crossed

Summary file, then RFC text, then `./le rfc check` violations and JSON (`unquoted`, `unquoted-total`, `unquoted-unjudged`). Audit verdicts (`rfc/audit/<stem>.json`) are keyed on the row-text SHA, so a row edit crosses into the audit.

### Integration Points

`./le verify worktree` runs `./le rfc check` on a detached checkout. Test tags (`RFC requirement: <ID>`) name row ids, never row text. `./le rfc quote-backfill` shares `quoteSource` and `rowQuoteRefusal` with the check.

### Measured 2026-09-26 (research phase)

| Measure | Value | How |
|---------|-------|-----|
| Rows still unquoted | 3672 (review 1249, human 2423), in 189 stems | `./le rfc quote-backfill stem <s>` dry run over all 201 stems. The spec's 3706 was 34 higher; the difference was not reconciled |
| Rows the backfill commit changed | 1855 in 153 stems (the commit message says 1821) | the rows of `0bf0696576^` and `0bf0696576`, parsed and compared by id |
| Changed rows a test tags | 986, over 1582 units (1559 Go funcs, 23 `.ci`) | `rfc.ScanTree` over the working tree |
| Rows citing section `x` | 257 | every one needs a section correction first |
| Rows in stems with no numbered heading | 69, in 7 stems | `sectionHeadingRE` |
| Stems with no extraction | rfc6514 (201 rows), rfc7454 (64), rfc8362 (50), rfc7627 (27), draft-ietf-sidrops-8210bis (13), rfc8195 (3) | backfill dry run |
| Sample of 21 hand-checked rows | 12 verbatim in the cited section, 4 in another section, 2 span consecutive sentences, 1 with no supporting sentence, 2 blocked by the tooling | read against `rfc/full` |
| Sample level mismatches | 5 of 21: the RFC has no 2119 keyword or lowercase "must" (5216, 8195, 7474, 792), or the row's level is wrong (RFC7950-7.6.4-1 is SHOULD, the RFC says MUST) | same |
| Sample effort | about 5 min a row on average, roughly 300 hours over 3672 rows | same |
| R-7 calibration | 2 of 3 known rows are only partly proven. RFC9552-5.2-6 does not drive adding or removing a TLV. RFC5880-6.7.3-4 does not drive the 32-bit wrap. RFC7432-6.3-3 is not-applicable, so there is no test | test bodies read |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | "unsourced" does not mean fabricated | in the sample, 2 of 3 unsourced rows had a sentence (RFC4271-4.3-4 shares the §4.3 sentence with -3) | retiring by kind would delete real obligations | every row is read against the RFC before any retire; the retire count is recorded per stem | unvalidated |
| A-2 | about 5% of rows have no supporting sentence | 1 of 21 sampled (RFC1877-x-5) | a larger retire volume means the ledger was inflated, and its public figures move | retire count per tranche; a tranche above 15% stops and is reported to the owner | broken (2026-09-27): rfc9582 retired 13 of 35 rows (37%); see the Mistake Log |
| A-3 | a verbatim span over consecutive sentences in one section replaces most row splits | the check matches any span inside one section body (`rowQuoteRefusal`) | rows split, and tags must follow the new ids | the sampled class (c) rows (RFC1661-2-1, RFC7474-5-2) pass as spans | unvalidated |
| A-4 | a hand reword stales only audit verdicts | `verdictFreshness` is the only reader of row text; sign-off (`signoff.go`), discrimination (`claimSHA`) and the render (`render_ledger.go`) do not read it | a mass reword triggers refusals across the ledger | the first stem commit of phase 3 runs `./le rfc check` with no new violation | unvalidated |
| A-5 | an audit file may carry verdicts for only some rows of a stem | `checkAuditSchema` and `checkAuditFreshness` iterate the verdicts present; no check demands a verdict per row (`check_audit.go`) | R-7 verdicts could not land stem by stem | the first R-7 stem commit passes `./le rfc check` with a partial audit file | unvalidated |
| A-6 | the audit check accepts a verdict on any enrolled stem | `checkAuditFiles` refuses only a file for an un-enrolled stem or a missing summary | R-7 rows of an un-enrolled stem have no durable record | list the R-7 stems that are not enrolled before phase 2 starts; their re-reads go in the per-stem table in this spec | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | a quoter picks a sentence that is in the RFC but states another obligation, which is the class the backfill's R-7 exposed | the blind sample disagrees with the quoter | owner decision D-4: in every stem, a second agent that did not quote re-derives a 10% sample blind, and any disagreement sends the whole stem back |
| R-2 | 69 rows in 7 stems cannot pass without a tooling change | unresolved-anchor refusals that name no numbered section | D-1, phase 1 |
| R-3 | the whole-corpus rule reddens every `Check()` fixture test | fixture rows that are not verbatim in `checkFixtureSource` | the fixture RFC text grows in the same phase-5 commit, and no assertion is weakened |
| R-4 | concurrent sessions edit `rfc/short`, `rfc/audit` or the tagged tests while stems land | a foreign hunk in a stem's summary or test file; SHIFTED verdicts (seen 2026-09-26 from another session's lint rewrite of the rfc7606 tests) | one stem per commit; a foreign hunk is judged against HEAD (`ai/rules/git-safety.md`), and a SHIFTED verdict from foreign work is resealed only after that work is committed |
| R-5 | a level demotion from MUST owes a correction paragraph and changes the ledger's MUST count | `checkLevelRatchet` refusal | the correction paragraph quotes the sentence and lands in the same commit; the per-stem table records the count of level changes |
| R-6 | a retired row drops tags, an extraction mapping, or an audit verdict that still names it | "unknown RFC requirement" (`check_core.go:evaluate`); `signoff.go` refusing a `mapped-to` or `unsourced-ids` entry that names a missing id; `checkAuditSchema` refusing a verdict on a missing id | the retire commit moves the tags to the row that states the obligation, or deletes a tag whose only claim was the fabricated row. It also drops the id from the extraction and the audit file, and D-2's correction paragraph names each move |
| R-7 | an R-7 re-read finds a test that proves less than its sentence, and the fix is a code change, not a test change | the audit verdict is `wrong` or `unimplemented` | the verdict is recorded and disclosed on the status page (`checkAuditDisclosure`). A defect in an implemented capability is fixed under `ai/rules/completion.md`; an absent feature is recorded as a gap (`ai/rules/rfc-compliance.md`). The spec stays open until each one has a home |
| R-9 | a retired id is remembered only by its `Retired` paragraph, so deleting that paragraph frees the id again, and no ratchet guards `rfc/corrections` | a `Retired` paragraph present at `HEAD^` and gone at HEAD | found in phase 1b (2026-09-26). The retirement records are reviewed in each stem commit; a ratchet on the paragraphs is recorded as a journal row, not built here |
| R-8 | the row-count figures shift under other sessions (3706 in the skeleton, 3672 measured; 1821 in the backfill commit message, 1855 measured) | the phase-start dry run disagrees with the figure in this spec | each phase starts from a fresh dry run; the spec's figures are a baseline, and AC-1 is the end condition, not a count |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing in the daemon. The public RFC ledger and `docs/features/rfc-status.md` counts move when rows retire or change level. A wrong quote misstates what Ze claims to conform to |
| How is it reverted? | per stem commit; the phase-1 and phase-5 tooling commits revert on their own |
| Who else touches this path? | strength-4 (the blind second walk of the prose stems, which reads the quoted rows afterwards); any session that edits `rfc/short`, `rfc/audit` or RFC-tagged tests |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` over a fixture commit whose unquoted row is unchanged from `HEAD^` | → | `checkRowQuotes` over every row | `TestCheckRefusesUnchangedUnquotedRow` (`internal/le/rfc/check_quote_test.go`), which replaces `TestCheckCountsUnchangedUnquotedRow` |
| `./le rfc check` over a fixture stem with no numbered heading | → | `quoteSource` resolving the whole-text section | `TestCheckAcceptsWholeTextQuoteInUnnumberedStem` (`check_quote_test.go`) |
| `./le rfc check` over a fixture commit that deletes an enrolled id | → | `checkRetiredRequirements` reading `rfc/corrections/<stem>.md` | `TestCheckAcceptsRetiredRowWithCorrection` (`check_ratchets_test.go`, or the file that holds the retire tests) |
| `./le rfc self-test` quote stage | → | `runQuoteSelftest` | `TestRFCSelftestQuoteStageRefusesFabricatedRow` stays green unchanged |

## Acceptance Criteria

| AC | Given / When | Then |
|----|--------------|------|
| AC-1 | `./le rfc check` on the tree at the end of phase 4 | its output carries no unquoted figure above zero and names no unjudged stem, and the JSON has `unquoted-total` 0 and an empty `unquoted-unjudged` |
| AC-2 | a fixture commit whose summary holds a row that is not verbatim and identical at `HEAD^` | `./le rfc check` exits non-zero and names that row, with the refusal reason (`rowQuoteRefusal`) |
| AC-3 | the phase-5 tree | none of `quoteRevisions`, `readQuoteRevisions`, `quoteChangedStems`, `quoteRowsAt`, `quoteSourceAt`, `scopeChangedRows`, `checkUnquotedRatchet` or `CheckReport.QuoteHistoryUnread` exists, and a grep for each returns nothing |
| AC-4 | a fixture stem with no numbered heading, and a row that cites the whole-text section with a verbatim quote | accepted. The same citation in a stem that has numbered headings is refused as an unresolved anchor |
| AC-5 | a fixture commit that deletes an enrolled id | refused with no correction paragraph; accepted with a dated retirement paragraph naming the id in `rfc/corrections/<stem>.md`; a later row reusing the id is refused (`checkIDAllocation`) |
| AC-6 | rfc8326 and rfc9129 | their text is in `rfc/full/`, and their 10 rows are quoted |
| AC-7 | each of the 986 tagged rows the backfill changed, in an enrolled stem | carries a fresh verdict in `rfc/audit/<stem>.json` (`verdictFreshness` answers fresh). Each row in an un-enrolled stem has a line in the R-7 table of this spec |
| AC-8 | a verdict `weak`, `wrong` or `unimplemented` from AC-7 | is resolved inside this spec: the test is fixed (verdict `enforced`, unit fingerprint moved), or the row is corrected, or the defect or gap is homed under R-7. Nothing sits as an open finding at closure without a named home |
| AC-9 | RFC7606-3.g-2, 4-1, 7.10-2, 7.14-1 | `enforced` in `rfc/audit/rfc7606.json`, and each new tag carries a discrimination record (`./le rfc discriminate-record`) |
| AC-10 | each quoted stem | its blind 10% sample (at least one row) is recorded in the per-stem table with agreement yes or no; every "no" has a re-read recorded |
| AC-11 | a row retired under D-2 | its correction paragraph quotes the search that found no sentence (sections read) and names where each of its tags moved |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| `TestCheckRefusesUnchangedUnquotedRow` | `internal/le/rfc/check_quote_test.go` | AC-2 |
| `TestCheckRefusesNewRowNotVerbatimInSection` (the violation count drops from 2 to 1 once the ratchet goes) | same | AC-2, AC-3 |
| `TestCheckNamesStemWithoutRFCTextRefused` (replaces `TestCheckNamesStemWithoutRFCTextUnjudged`) | same | a stem with no RFC text is refused row by row, not left unjudged |
| `TestCheckAcceptsWholeTextQuoteInUnnumberedStem` | same | AC-4 accept |
| `TestCheckRefusesFrontCitationInNumberedStem` | same | AC-4 refuse |
| `TestCheckRefusesRetiredRowWithoutCorrection` | the file that holds the `checkRetiredRequirements` tests | AC-5 refuse |
| `TestCheckAcceptsRetiredRowWithCorrection` | same | AC-5 accept |
| `TestCheckRefusesRetiredIDReuse` | same | AC-5 reuse |
| Deleted: `TestCheckRefusesUnquotedCountRise`, `TestCheckRefusesUnquotedCountRiseFromRFCTextChange` | `check_quote_test.go` | their subject, the ratchet, is deleted (no-layering) |

### Boundary Tests (numeric inputs)

N-A: the change takes no new numeric input. The 24-character minimum quote is already pinned by existing tests.

### Functional Tests

| Test | Validates |
|------|-----------|
| `./le rfc check` over the real tree at the end of phase 4 and of phase 5 | AC-1 (output pasted into the spec) |

AC-1 evidence, 2026-09-27, `./le --name l5 rfc check` (a fresh build) over HEAD
4ea190082d plus the uncommitted phase 5 flip, so every row is judged. The
unquoted figures no longer exist after the flip: an unquoted row is now a
violation, and the output names none. The 18 violations are all foreign
discrimination records whose producer changed; none is a row-quote refusal.
Each line is cut after "(producer-changed)"; the rest of every line is the
same explanation.

```
rfc-requirements: 18 violation(s)
  * rfc/discrimination/rfc4271.json: the revert record for RFC4271-6.3-15 positive at internal/component/bgp/reactor/session_update_error_rfc4271_test.go::TestRFC4271UpdateMalformedAttributeList no longer verifies (producer-changed) [...]
  * rfc/discrimination/rfc4271.json: the revert record for RFC4271-6.3-15 negative at internal/component/bgp/reactor/session_update_error_rfc4271_test.go::TestRFC4271UpdateMalformedAttributeList no longer verifies (producer-changed) [...]
  * rfc/discrimination/rfc7611.json: the revert record for RFC7611-2.1-1 negative at internal/component/bgp/reactor/rfc4271_rfc7611_ingress_test.go::TestRFC7611OwnRouteNeverReacceptedIntoSource no longer verifies (producer-changed) [...]
  * rfc/discrimination/rfc7611.json: the revert record for RFC7611-2.1-1 positive at internal/component/bgp/reactor/rfc4271_rfc7611_ingress_test.go::TestRFC7611OwnRouteNeverReacceptedIntoSource no longer verifies (producer-changed) [...]
  * rfc/discrimination/rfc7705.json: the revert record for RFC7705-3.3-1 positive at internal/component/bgp/config/peers_test.go::TestPeersFromConfigTree_LocalASOptionsPerNeighborGroup no longer verifies (producer-changed) [...]
  * rfc/discrimination/rfc7947.json: the mutant record for RFC7947-2.2.2.2-1 positive at internal/component/bgp/reactor/filter/loop_test.go::TestLoopIngressAcceptsNonAdjacentLeftmostAS no longer verifies (producer-changed) [...]
  * rfc/discrimination/rfc7947.json: the mutant record for RFC7947-2.2.2.2-1 negative at internal/component/bgp/reactor/filter/loop_test.go::TestLoopIngressRejectsLocalASFromRouteServer no longer verifies (producer-changed) [...]
  * rfc/discrimination/rfc8907.json: the revert record for RFC8907-10-1 positive at internal/component/tacacs/rfc8907_unencrypted_config_test.go::TestRFC8907UnencryptedModeIsNotReachableFromConfiguration no longer verifies (producer-changed) [...]
  * rfc/discrimination/rfc8907.json: the revert record for RFC8907-10-2 positive at internal/component/tacacs/rfc8907_obfuscation_test.go::TestRFC8907ClientNeverSendsUnobfuscatedBody no longer verifies (producer-changed) [...]
  * rfc/discrimination/rfc8907.json: the revert record for RFC8907-10.5.2-1 negative at internal/component/tacacs/rfc8907_obfuscation_test.go::TestRFC8907ClientRefusesUnobfuscatedReply no longer verifies (producer-changed) [...]
  * rfc/discrimination/rfc8907.json: the revert record for RFC8907-4.3-1 negative at internal/component/tacacs/rfc8907_connection_test.go::TestRFC8907NoSecondPacketWithoutSingleConnect no longer verifies (producer-changed) [...]
  * rfc/discrimination/rfc8907.json: the revert record for RFC8907-4.3-1 positive at internal/component/tacacs/rfc8907_connection_test.go::TestRFC8907SecondSessionReusesEstablishedConnection no longer verifies (producer-changed) [...]
  * rfc/discrimination/rfc8907.json: the revert record for RFC8907-4.3-2 negative at internal/component/tacacs/rfc8907_connection_test.go::TestRFC8907ClientDoesNotResignalSingleConnect no longer verifies (producer-changed) [...]
  * rfc/discrimination/rfc8907.json: the revert record for RFC8907-4.3-2 positive at internal/component/tacacs/rfc8907_connection_test.go::TestRFC8907FlagClearedOnLaterReplyIsIgnored no longer verifies (producer-changed) [...]
  * rfc/discrimination/rfc8907.json: the revert record for RFC8907-4.3-3 negative at internal/component/tacacs/rfc8907_connection_test.go::TestRFC8907ClosureOnFreshDialIsNotRetried no longer verifies (producer-changed) [...]
  * rfc/discrimination/rfc8907.json: the revert record for RFC8907-4.4-2 negative at internal/component/tacacs/rfc8907_connection_test.go::TestRFC8907HealthyPooledConnectionKeepsAcceptingSessions no longer verifies (producer-changed) [...]
  * rfc/discrimination/rfc8907.json: the revert record for RFC8907-4.4-3 negative at internal/component/tacacs/rfc8907_connection_test.go::TestRFC8907HealthyPooledConnectionStaysOpen no longer verifies (producer-changed) [...]
  * rfc/discrimination/rfc8907.json: the revert record for RFC8907-5.4.2.2-1 negative at internal/component/tacacs/rfc8907_connection_test.go::TestRFC8907PAPClientSendsNoContinue no longer verifies (producer-changed) [...]
```

### Interop Tests

N-A: tooling and ledger text; no protocol behaviour changes. An R-7 finding that turns out to be a protocol defect is handled under R-7 and carries its own interop obligation.

## Files to Modify

- `internal/le/rfc/check_quote.go` - judge every row; delete the change scope and the ratchet; fuse with `unquotedFigures`
- `internal/le/rfc/check.go` - drop the calls, `QuoteHistoryUnread`, and its printed line
- `internal/le/rfc/inventory.go` - `quoteSource`: a stem with no numbered heading exposes its whole text as one section (D-1)
- `internal/le/rfc/check_ratchets.go` - `checkRetiredRequirements` accepts a retirement named by a dated paragraph in `rfc/corrections/<stem>.md` (D-2)
- `internal/le/rfc/quote_backfill.go` - same whole-text section, so the dry run stops sending those rows to review
- `internal/le/rfc/check_quote_test.go`, `check_test.go`, `selftest_state.go`, `selftest_core.go` - the fixture RFC text grows to hold every fixture row verbatim
- `rfc/short/<stem>.md` - the rows, stem by stem
- `rfc/audit/<stem>.json` - R-7 verdicts, and re-judged verdicts for reworded rows in rfc7606 and draft-abraitis-idr-addpath-paths-limit
- `rfc/corrections/<stem>.md` and `rfc/corrections/README.md` - retirement and level-correction paragraphs, and the retirement form
- `rfc/extraction/<stem>.json` - drop the ids of retired rows from `mapped-to` and `unsourced-ids`
- tagged tests (`*_test.go`, `.ci`) - R-7 fixes and the rfc7606 tags; `rfc/discrimination/<stem>.json` for each new tag
- `docs/contributing/rfc-conformance-gates.md` - "The row quote" (every row judged, no ratchet, whole-text section) and "Requirements do not vanish" (retirement by correction)

## Files to Create

- `rfc/full/rfc8326.txt`, `rfc/full/rfc9129.txt`
- `rfc/audit/<stem>.json` for each enrolled R-7 stem that has none

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | no config |
| YANG validation constraints | N-A | no config |
| YANG custom validators | N-A | no config |
| CLI commands/flags | No | `./le rfc check` keeps its surface; only its verdicts widen. The JSON loses the field behind `QuoteHistoryUnread` if one is emitted |
| CLI grammar | N-A | no new verb |
| Editor autocomplete | N-A | no YANG |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | le tooling |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Prometheus counters/metrics | N-A | none |
| BGP family surface | N-A | no family |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | tooling only |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | `./le rfc check` verdicts widen; the gates page carries it (row 12) |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | none |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/<stem>.md` rows; `docs/features/rfc-status.md` regenerates from the summaries when a retirement or a level change moves a count |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/contributing/rfc-conformance-gates.md`, "The row quote" and "Requirements do not vanish", edited in the phase that changes each behaviour |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors` (2026-09-26) names two pages. The first is `docs/architecture/core-design.md`, declared by `check.go`, `inventory.go` and `check_ratchets.go`. It is unaffected: it describes the le artifact and hook model, not the row check, the section resolution or the retire rule. The second is `docs/architecture/testing/test-health.md`, which cites `check_ratchets.go` for `checkDiscriminationRatchet` only. It is unaffected because that ratchet does not change. The gates page is the one that changes (row 12). The anchors are re-run at phase 1 and phase 5 |
| 17 | Existing docs show examples for this area? | Yes | the gates page describes the ratchet and the change scope; phase 5 rewrites those paragraphs |

## Implementation Steps

Each phase runs in subagents through `/ze-implement`. The main thread supervises, verifies each report against source, and gates the next phase. Every stem tranche is one commit.

1. **Phase: Wiring and tooling (D-1, D-2, texts)**
   - Tests: `TestCheckAcceptsWholeTextQuoteInUnnumberedStem`, `TestCheckRefusesFrontCitationInNumberedStem`, `TestCheckRefusesRetiredRowWithoutCorrection`, `TestCheckAcceptsRetiredRowWithCorrection`, `TestCheckRefusesRetiredIDReuse`
   - Files: `inventory.go`, `quote_backfill.go`, `check_ratchets.go`, `rfc/corrections/README.md`, the gates page; fetch `rfc/full/rfc8326.txt` and `rfc/full/rfc9129.txt`
   - Verify: tests fail, then pass. The dry run no longer lists the 69 rows of the seven stems as unresolved-anchor
2. **Phase: R-7 re-read**
   - Input: the 986 ids (the backfill commit `0bf0696576`, rows parsed at both revisions, tags from `rfc.ScanTree`), grouped by stem. The three known rows and the rfc7606 four go first
   - Per stem: an agent applies `/ze-rfc-audit` to the changed tagged rows only and writes their verdicts. A second agent re-judges 10% blind (D-4). Findings go through AC-8. The rfc7606 four get their tags and discrimination records (AC-9)
   - Verify: `./le rfc check` shows no stale or missing verdict for the stem; the per-stem table below gains a row
3. **Phase: Review bucket (1249 rows)**
   - Per stem: an agent reads each row's cited section in `rfc/full` and writes the verbatim sentence or span. It fixes the section or the level (D-3, with a correction paragraph when it demotes a MUST), or retires the row (D-2). The tagged tests of each quoted row get their `/ze-rfc-audit` verdict (R-1). A second agent re-derives 10% blind
   - Verify: the stem's dry run lists nothing; `./le rfc check` shows no new violation
4. **Phase: Human buckets (2423 rows)**
   - Same procedure as phase 3, largest stems first (rfc6514, rfc9830, rfc4035, rfc2131, rfc4271, ...). The 257 rows citing `x` get their section first
   - Verify: at the end, `./le rfc check` meets AC-1 (output pasted)
5. **Phase: Rule over every row**
   - Tests: `TestCheckRefusesUnchangedUnquotedRow`, `TestCheckNamesStemWithoutRFCTextRefused`, the rewritten `TestCheckRefusesNewRowNotVerbatimInSection`; the ratchet tests are deleted
   - Files: `check_quote.go`, `check.go`, the fixtures, the gates page
   - Verify: AC-2, AC-3; `./le verify worktree`

### Deriving the R-7 set

Any session can rebuild the list; it is not committed, because it is derived. The steps:

1. Parse `rfc/short/<stem>.md` at `0bf0696576^` and at `0bf0696576` with the `reChecklist` grammar (`rfc.go`), for every stem that commit touched.
2. Pair the rows by id, and keep each id whose text differs. That gives 1855 on 2026-09-26, with no ids added or removed.
3. Scan tags with `rfc.ScanTree` (`carriers.go`), and keep the changed ids that at least one tag names. That gave 986.
4. Group by stem. For each stem, list the tagged units (`rfc.UnitAt`).

A `go run` of this lives only in a session's scratch. A session that repeats it states its own counts in the per-stem table and does not copy these ones.

### Stem brief (phases 2 to 4)

Every agent for a stem gets this brief, in this order. Phase 2 runs only steps 1, 6, 7, 8 and 9, on the stem's R-7 ids.

| Step | Action | Record |
|------|--------|--------|
| 1 | Run `./le rfc quote-backfill stem <stem>` (dry) and take the stem's review and human rows, with their kinds. For phase 2, take the stem's R-7 ids instead | the counts in the per-stem row |
| 2 | For each row, read the cited section of `rfc/full/<stem>.txt` (or `rfc/drafts/`). When the row cites `x` or an unresolved section, first find the section that states the obligation | - |
| 3 | Write the verbatim sentence, or a verbatim span of consecutive sentences in one section, as the row text. Keep the id, the markers and the parenthetical. Correct the section when the sentence is in another one | - |
| 4 | Level (D-3): keep the row's level unless the sentence carries a different RFC 2119 keyword. A demotion from MUST adds a dated "Correction" paragraph to `rfc/corrections/<stem>.md`, quoting the sentence | the level-change count |
| 5 | Retire (D-2), only after reading the whole RFC for the obligation, not just the cited section. Check first whether another document states it; a re-attribution then follows `plan/pre-release/spec-rfc-requirement-reattribution.md`, and is not a retirement. That spec is a skeleton, and the gates refuse a move today, so a re-attribution case is reported to the owner, not forced. Otherwise: move each tag to the row that does state the obligation, or delete a tag whose only claim was this row. Drop the id from `rfc/extraction/<stem>.json` and `rfc/audit/<stem>.json`. Delete the row and write the dated retirement paragraph naming the sections read and where each tag went | the retired count; stop and report if it passes 15% of the stem's rows (A-2) |
| 6 | For every row whose text changed in this step or in the backfill, and that a test tags, apply `/ze-rfc-audit` to that row only: read the RFC sentence and each tagged unit, and write the verdict in `rfc/audit/<stem>.json`. An un-enrolled stem gets a line in the per-stem table instead (A-6) | verdict counts |
| 7 | A `weak`, `wrong` or `unimplemented` verdict follows AC-8: fix the test (a new tag owes `./le rfc discriminate-record`), correct the row, or home the defect or gap under R-7 | the findings, named in the per-stem row |
| 8 | Run `./le rfc check`: no new violation for the stem, and the stem's dry run lists no row | - |
| 9 | Blind sample (D-4). A second agent gets the stem, the row ids and the RFC text, but not the new row text or the verdicts. It re-derives 10% of the changed rows, at least one, spread over the kinds. The main thread compares the two. Any disagreement returns the whole stem to step 2 | agreed yes/no |
| 10 | One commit per stem through `./le commit create`, naming the summary, the audit, the extraction, the corrections and any tests | the commit SHA |

### Per-stem progress

One row per stem, appended when the stem's commit lands.

| Stem | Phase | Rows quoted | Retired | Level changed | R-7 verdicts (enforced / weak / wrong) | Blind sample agreed | Commit |
|------|-------|-------------|---------|---------------|-----------------------------------------|---------------------|--------|
| draft-abraitis-bgp-version-capability | 2-4 | 9 | 0 | 0 | 11 / 0 / 0 | yes, 2/2; verdicts 1/2 | c7c098e642 90579b6a4f |
| draft-abraitis-idr-addpath-paths-limit | 2-4 | 5 | 0 | 0 | 6 / 1 / 0 | yes, 1/1; verdicts 1/1 | 9afaa8d466 |
| draft-ietf-bess-mup-safi | 2-4 | 39 | 0 | 0 | 0 / 9 / 0 | yes, 6/6; verdicts 1/2 | f15791393d e6028708e8 |
| draft-ietf-idr-bgp-bfd-strict-mode | 2-4 | 1 | 0 | 0 | 1 / 3 / 0 | yes, 1/1; verdicts 1/1 (BS-A, 2026-09-27) (note c) | 7757c3e955 |
| draft-ietf-idr-linklocal-capability | 2-4 | 15 | 0 | 0 | 4 / 8 / 0 | yes, 2/2; verdicts 2/2 | c88a3a858d a9beca2fc3 |
| draft-ietf-sidrops-8210bis | 2-4 | 8 | 5 | 0 | un-enrolled, R-7 table lines | yes, 1/1 | cbcade6f68 1dd66ea8f7 be0c565ad3 b6fc04265e |
| draft-ietf-sidrops-aspa-verification | 2-4 | 10 | 0 | 0 | 4 / 1 / 0 | yes, 1/1; verdicts 1/1 (BS-A, 2026-09-27) (note b) | 3e134ed06f |
| draft-walton-bgp-hostname-capability | 2-4 | 1 | 0 | 0 | no audit file | yes, 1/1 | 0c3b4ec246 |
| rfc1035 | 2-4 | 15 | 0 | 0 | un-enrolled, R-7 table lines | yes, 2/2 | f503e11de3 |
| rfc1071 | 2-4 | 9 | 2 | 0 | 1 / 4 / 1 | yes, 1/1; verdicts 1/1 (BS-A, 2026-09-27) (note c) | be0c565ad3 ad422fdf37 |
| rfc1195 | 2-4 | 21 | 0 | 0 | 12 / 14 / 2 | yes, 3/3 | 55a426abec 86206cffae 10edacdc5c |
| rfc1332 | 2-4 | 12 | 0 | 0 | 3 / 2 / 0 | yes, 1/1; verdicts 0/1 (BS-A, 2026-09-27); sent back, re-read 6f307a730b: now 1 / 4 / 0 (note b) | 98b5dd9379 |
| rfc1334 | 2-4 | 6 | 0 | 0 | 8 / 2 / 0 | yes, 1/1; verdicts 2/2 | 36e77ebc0a |
| rfc1350 | 2-4 | 10 | 0 | 0 | 2 / 5 / 1 | yes, 2/2; verdicts 2/2 | f4a4cea195 7ec6cda22b |
| rfc1661 | 2-4 | 45 | 0 | 7 | 9 / 3 / 2 | yes, 4/4; verdicts 3/3 | 0c00c9ade2 94bebd270b |
| rfc1877 | 2-4 | 0 | 5 | 0 | no audit file | none owed: no quote changed (retirements only) | b8cfb52f0b |
| rfc1994 | 2-4 | 12 | 0 | 0 | 6 / 6 / 2 | yes, 3/3; verdicts 1/3 | ec8e4ba924 024ae3edca |
| rfc1997 | 2-4 | 9 | 0 | 0 | 5 / 0 / 0 | yes, 1/1; verdicts 1/1 | 09812f450d 5cb3f11e8d |
| rfc2003 | 2-4 | 28 | 0 | 0 | no audit file | yes, 3/3 | 22ff9fffb0 |
| rfc2131 | 2-4 | 73 | 0 | 2 | 15 / 4 / 0 | yes, 10/10 | 0daedbc60e |
| rfc2132 | 2-4 | 39 | 0 | 0 | 7 / 0 / 1 | yes, 7/7; verdicts 1/1 | a6ecfeb969 |
| rfc2181 | 2-4 | 42 | 0 | 0 | 3 / 5 / 0 | yes, 6/6; verdicts 1/1 | 69758e2636 0ec5698c56 |
| rfc2205 | 2-4 | 26 | 0 (+1 added) | 0 | 5 / 11 / 0 | yes, 7/7; verdicts 3/4 | d69cfafda0 c78c87b136 76fcaae9c0 be0c565ad3 6887f57258 |
| rfc2328 | 2-4 | 59 | 0 (+1 added) | 0 | 20 / 30 / 1 | yes, 6/6; verdicts 8/10 (BS-A, 2026-09-27); sent back, re-read a2e975c045: now 19 / 31 / 1 (note b) | 4d04c8cc14 b12697744d be0c565ad3 |
| rfc2347 | 2-4 | 6 | 0 | 0 | 0 / 2 / 0 | yes, 1/1; verdicts 1/1 (BS-A, 2026-09-27) (note b) | c265e4cea0 |
| rfc2348 | 2-4 | 5 | 0 | 0 | 1 / 3 / 0 | yes, 1/1; verdicts 1/1 (BS-A, 2026-09-27) (note b) | 0a3b7577a6 |
| rfc2349 | 2-4 | 6 | 0 | 0 | 1 / 0 / 0 | yes, 1/1; verdicts 1/1 (BS-A, 2026-09-27) (note b) | eefd0832ef |
| rfc2385 | 2-4 | 4 | 0 | 0 | 4 / 5 / 0 | yes, 1/1; verdicts 2/2 | 97693d5cae |
| rfc2473 | 2-4 | 19 | 0 | 0 | no audit file | yes, 2/2 | 3329af6c31 |
| rfc2516 | 2-4 | 25 | 5 | 2 | 18 / 11 / 2 | yes, 4/4; verdicts 5/6 | 7b894364d9 9b29095b20 |
| rfc2545 | 2-4 | 2 | 0 | 0 | 4 / 0 / 0 | yes, 1/1; verdicts 1/1 (BS-A, 2026-09-27) (note b) | 0f1153d2a7 |
| rfc2661 | 2-4 | 25 | 0 | 2 | 5 / 17 / 0 | yes, 3/3; verdicts 4/4 | db6fb6b5a5 507c8aad1d |
| rfc2759 | 2-4 | 13 | 1 | 0 | 4 / 7 / 0 | yes, 1/1; verdicts 0/2 (BS-A, 2026-09-27); sent back, re-read ebb18b811a: now 3 / 8 / 0 (note b) | 6f3ab6c00b 76fcaae9c0 8b7d0ed7cf |
| rfc2782 | 2-4 | 11 | 0 | 0 | no audit file | yes, 1/1 | 299171c14d |
| rfc2784 | 2-4 | 9 | 0 | 0 | no audit file | yes, 1/1 | 716e4bc73a |
| rfc2865 | 2-4 | 12 | 3 | 0 | 13 / 7 / 0 | yes, 2/2; verdicts 4/4 | 67ece73a56 be0c565ad3 867e5564d3 |
| rfc2866 | 2-4 | 6 | 2 | 0 | 5 / 6 / 1 | yes, 1/1; verdicts 1/2 | 2d24464fbe be0c565ad3 |
| rfc2869 | 2-4 | 5 | 4 (+1 added) | 0 | 4 / 2 / 0 | yes, 1/1; verdicts 1/1 | 8e9c87b644 76fcaae9c0 dd7cec8ceb |
| rfc2890 | 2-4 | 4 | 0 | 0 | no audit file | yes, 1/1 | d2f3eb25c3 |
| rfc2918 | 2-4 | 9 | 0 | 0 | 3 / 4 / 0 | yes, 1/1; verdicts 1/1 (BS-A, 2026-09-27) (note b) | 8879c5c67f |
| rfc2966 | 2-4 | 6 | 0 | 0 | 3 / 2 / 0 | yes, 1/1; verdicts 1/1 (BS-A, 2026-09-27) (note b) | a80af96fcb |
| rfc3031 | 2-4 | 6 | 3 | 0 | no audit file | yes, 1/1 | fffeb765b3 |
| rfc3032 | 2-4 | 14 | 0 (+1 added) | 1 | 0 / 1 / 0 | yes, 2/2 | cdcf184e21 be0c565ad3 74205057fc |
| rfc3101 | 2-4 | 19 | 0 | 1 | 16 / 8 / 2 | yes, 3/3 | a677d1d9f6 b12697744d |
| rfc3209 | 2-4 | 13 | 3 | 0 | 8 / 8 / 1 | yes, 5/5; verdicts 3/3 | 1bf08dbad3 be0c565ad3 |
| rfc3579 | 2-4 | 29 | 0 | 0 | 13 / 4 / 0 | yes, 5/5; verdicts 2/2 | bb645d524e 8dda6261e4 |
| rfc3623 | 2-4 | 21 | 0 | 0 | 4 / 5 / 0 | yes, 3/3; verdicts 2/2 | 403271bbab aeb6e04996 b12697744d |
| rfc3630 | 2-4 | 8 | 0 | 0 | 1 / 0 / 0 | yes, 1/1; verdicts 1/1 (BS-A, 2026-09-27) (note b) | 09b16f1e1a |
| rfc3748 | 2-4 | 25 | 3 | 1 | 28 / 9 / 0 | yes, 5/5; verdicts 6/7 (BS-A, 2026-09-27); sent back, re-read 2cf1b04765: now 14 / 23 / 0 (note c) | be0c565ad3 41921aa760 |
| rfc3768 | 2-4 | 34 | 0 | 0 | 21 / 15 / 1 | yes, 5/5; verdicts 3/7 (BS-A, 2026-09-27); sent back, re-read e07e40de0f: now 9 / 26 / 2 (note c) | 850eb41b66 |
| rfc3786 | 2-4 | 7 | 0 | 0 | 1 / 1 / 0 | yes, 1/1; verdicts 1/1 (BS-A, 2026-09-27) (note b) | f26f531ea6 |
| rfc3787 | 2-4 | 6 | 0 | 0 | 0 / 2 / 0 | yes, 1/1; verdicts 0/1 (BS-A, 2026-09-27); sent back, re-read by SB-3 with no change and no commit: 0 / 2 / 0 (note b) | e1fa028050 |
| rfc3948 | 2-4 | 11 | 2 | 1 | 3 / 3 / 2 | yes, 2/2; verdicts 2/2 (BS-A, 2026-09-27) (note b) | 54d696b2c8 be0c565ad3 9372480ca8 |
| rfc3954 | 2-4 | 26 | 0 | 1 | 0 / 4 / 0 | yes, 3/3; verdicts 1/1 | 713d002114 cbb5a5576a |
| rfc4035 | 2-4 | 74 | 0 | 0 | 7 / 4 / 0 | yes, 12/12 | b43510554f |
| rfc4090 | 2-4 | 25 | 2 | 0 | 23 / 10 / 1 | yes, 4/4; verdicts 2/4 | a67df9731f 7e139a49e0 |
| rfc4213 | 2-4 | 29 | 0 | 1 | no audit file | yes, 4/4 | b35ce9e9d6 |
| rfc4271 | 2-4 | 77 | 0 | 1 | 22 / 55 / 3 | yes, 13/13; verdicts 15/16 | dae5429aec daa2adcccc |
| rfc4301 | 2-4 | 27 | 0 | 0 | 19 / 8 / 5 | no, 5/6; verdicts 3/6 (BS-A, 2026-09-27); sent back, re-read d240103498: now 15 / 12 / 5 (note b) | 9f61b44afa |
| rfc4302 | 2-4 | 18 | 0 | 0 | 4 / 3 / 2 | yes, 4/4; verdicts 1/2 | 134987b1eb c2bd7f0894 76fcaae9c0 |
| rfc4303 | 2-4 | 20 | 0 | 0 | 4 / 4 / 1 | yes, 2/2; verdicts 2/2 | 1054d49c4d |
| rfc4360 | 2-4 | 6 | 1 | 0 | 0 / 2 / 0 | yes, 1/1; verdicts 1/1 (BS-A, 2026-09-27) (note b) | 21cfb98092 |
| rfc4364 | 2-4 | 8 | 0 | 0 | no audit file | yes, 1/1 | 96ee1f5d0c |
| rfc4456 | 2-4 | 9 | 0 | 2 | 6 / 1 / 0 | yes, 1/1; verdicts 1/1 | 9129371a4b ef334fccad |
| rfc4552 | 2-4 | 10 | 0 | 0 | 3 / 3 / 0 | yes, 2/2; verdicts 1/1 | fc5a9d5772 |
| rfc4555 | 2-4 | 18 | 0 | 0 | 8 / 2 / 0 | yes, 2/2; verdicts 2/2 | d517a4a0cd |
| rfc4577 | 2-4 | 40 | 0 | 2 | 2 / 2 / 0 | yes, 6/6; verdicts 0/1 | b62d10fcbd 563081ef96 |
| rfc4578 | 2-4 | 5 | 0 | 0 | 1 / 0 / 0 | yes, 1/1; verdicts 0/1 (BS-B, 2026-09-27); sent back, re-read 5f65b0a6e8: now 0 / 1 / 0 (note b) | 8b4ee817ce |
| rfc4659 | 2-4 | 14 | 0 | 0 | 2 / 2 / 0 | yes, 2/2; verdicts 1/1 | 23149b447e |
| rfc4684 | 2-4 | 6 | 0 | 0 | 2 / 0 / 0 | yes, 1/1; verdicts 0/1 (BS-B, 2026-09-27); sent back, re-read 042a63f22e: now 1 / 1 / 0 (note b) | bf065296d4 |
| rfc4724 | 2-4 | 10 | 0 | 1 | 3 / 14 / 0 | yes, 3/3; verdicts 3/3 | 70647a8c13 |
| rfc4760 | 2-4 | 13 | 0 | 0 | 2 / 3 / 1 | yes, 2/2; verdicts 0/1 | 317e8e2884 |
| rfc4761 | 2-4 | 15 | 0 | 0 | no audit file | yes, 2/2 | 5a3420bd87 |
| rfc4862 | 2-4 | 15 | 0 | 0 | no audit file | yes, 2/2 | 4ff3b5641e |
| rfc5036 | 2-4 | 32 | 0 | 0 | 11 / 4 / 0 | yes, 7/7; verdicts 3/3 | 789e41cdd4 |
| rfc5072 | 2-4 | 17 | 0 | 1 | 8 / 3 / 0 | yes, 3/3; verdicts 2/2 | f2378be549 |
| rfc5082 | 2-4 | 7 | 0 | 0 | 3 / 0 / 0 | yes, 1/1; verdicts 0/1 (BS-B, 2026-09-27); sent back, re-read d25ba4d0d4: now 2 / 1 / 0 (note b) | ea9d36ee9f |
| rfc5176 | 2-4 | 19 | 0 | 0 | 12 / 9 / 0 | yes, 2/2; verdicts 4/4 | bbd439f6f7 |
| rfc5187 | 2-4 | 2 | 0 | 0 | 0 / 4 / 0 | yes, 1/1; verdicts 1/1 (BS-B, 2026-09-27) (note b) | 7d1b06cb05 |
| rfc5216 | 2-4 | 21 | 0 | 0 | 20 / 17 / 1 | yes, 5/5; verdicts 8/8 | c66f43decc |
| rfc5250 | 2-4 | 9 | 0 | 0 | 4 / 6 / 0 | yes, 1/1; verdicts 2/2 | 41b6a7e483 |
| rfc5282 | 2-4 | 14 | 0 | 0 | 10 / 8 / 0 | yes, 2/2; verdicts 4/4 | 3d64f5bee9 |
| rfc5286 | 2-4 | 30 | 0 | 0 | 2 / 1 / 0 | yes, 3/3; verdicts 1/1 | 483a9628b6 b12697744d |
| rfc5301 | 2-4 | 1 | 0 | 0 | 1 / 0 / 0 | yes, 1/1; verdicts 1/1 (BS-B, 2026-09-27) (note b) | 6d4e1fa55c |
| rfc5303 | 2-4 | 15 | 0 | 0 | 3 / 5 / 1 | yes, 2/2; verdicts 1/2 | 230651f0dc |
| rfc5304 | 2-4 | 3 | 0 | 0 | no audit file | yes, 1/1; no verdict sampled, no audit file (BS-B, 2026-09-27) (note b) | 8db8cc84c4 |
| rfc5305 | 2-4 | 13 | 0 | 0 | 2 / 2 / 1 | yes, 1/1; verdicts 0/1 | 588855afee 7a10313313 |
| rfc5308 | 2-4 | 8 | 0 | 0 | 3 / 5 / 0 | yes, 1/1; verdicts 2/2 (BS-B, 2026-09-27) (note b) | fc6e2b04af |
| rfc5310 | 2-4 | 7 | 0 | 0 | 1 / 5 / 1 | yes, 1/1; verdicts 1/1 | 3f8a691fd8 |
| rfc5340 | 2-4 | 31 | 0 | 0 | 9 / 4 / 0 | yes, 4/4; verdicts 1/3 | f6844cc678 b12697744d |
| rfc5392 | 2-4 | 27 | 0 | 1 | 6 / 2 / 0 | yes, 4/4; verdicts 2/2 | 898cabab50 b12697744d 38461a9435 |
| rfc5443 | 2-4 | 14 | 0 | 0 | 0 / 4 / 1 | yes, 1/1; verdicts 1/1 | a4a860c5fb |
| rfc5492 | 2-4 | 11 | 0 | 0 | 3 / 5 / 0 | yes, 2/2; verdicts 2/2 | 4f07a5e7a8 |
| rfc5549 | 2-4 | 4 | 0 | 1 | 2 / 3 / 0 | yes, 1/1; verdicts 1/1 (BS-B, 2026-09-27) (note b) | 423126ceb8 |
| rfc5561 | 2-4 | 6 | 0 | 0 | 1 / 0 / 0 | yes, 1/1; verdicts 1/1 (BS-B, 2026-09-27) (note b) | 3edb5848de |
| rfc5575 | 2-4 | 9 | 3 | 0 | 4 / 4 / 0 | yes, 2/2; verdicts 2/2 | 751be2b01f |
| rfc5701 | 2-4 | 6 | 0 | 0 | 3 / 0 / 0 | yes, 1/1; verdicts 1/1 (BS-B, 2026-09-27) (note b) | 3e01fca43a |
| rfc5709 | 2-4 | 17 | 0 | 0 | 10 / 4 / 0 | yes, 2/2; verdicts 3/3 | 78d54c9df7 b12697744d |
| rfc5798 | 2-4 | 52 | 0 | 0 | 22 / 21 / 0 | yes, 8/8 | 3780f7f19a 850eb41b66 |
| rfc5838 | 2-4 | 24 | 0 | 0 | 2 / 2 / 2 | yes, 3/3; verdicts 1/1 | eae19123cc |
| rfc5880 | 2-4 | 66 | 0 (+2 added) | 0 | 39 / 42 / 3 | no, 6/7; verdicts 15/17 (BS-B, 2026-09-27); sent back, re-read a6e945caa0: now 34 / 47 / 3 (note c) | 7757c3e955 |
| rfc5881 | 2-4 | 16 | 0 | 0 | 3 / 11 / 1 | yes, 3/3; verdicts 3/3 | a770d91783 7757c3e955 |
| rfc5882 | 2-4 | 26 | 0 | 0 | 0 / 2 / 0 | yes, 3/3; verdicts 1/1 | d98a845125 7757c3e955 |
| rfc5883 | 2-4 | 5 | 2 | 0 | 4 / 2 / 0 | yes, 1/1; verdicts 1/1 (BS-B, 2026-09-27) (note c) | 7757c3e955 |
| rfc6071 | 2-4 | 20 | 0 | 0 | no audit file | yes, 2/2 | 23a38246f5 |
| rfc6138 | 2-4 | 1 | 0 | 0 | 2 / 0 / 0 | yes, 1/1; verdicts 0/1 (BS-B, 2026-09-27); sent back, re-read 70d678a85f: now 1 / 1 / 0 (note b) | 306835a90b |
| rfc6286 | 2-4 | 5 | 0 | 0 | 3 / 2 / 0 | yes, 1/1; verdicts 1/1 (BS-B, 2026-09-27) (note b) | d3bbf55a6b |
| rfc6396 | 2-4 | 20 | 0 | 0 | 2 / 5 / 0 | yes, 2/2; verdicts 1/1 | d853474849 |
| rfc6397 | 2-4 | 1 | 0 | 0 | no audit file | yes, 1/1 | 7d865cf35e |
| rfc6482 | 2-4 | 3 | 0 | 0 | no audit file | yes, 1/1 | bb4655a759 |
| rfc6514 | 2-4 | 201 | 0 | 0 | no audit file | yes, 20/20 | 8140b9c44f |
| rfc6549 | 2-4 | 4 | 0 | 0 | 1 / 0 / 0 | yes, 1/1; verdicts 1/1 (BS-B, 2026-09-27) (note b) | cc8499c716 |
| rfc6608 | 2-4 | 3 | 0 | 0 | no audit file | yes, 1/1 | 65efc12ece |
| rfc6793 | 2-4 | 11 | 0 | 0 | 23 / 4 / 0 | yes, 3/3; verdicts 3/3 | 051abad24a a79270be1d |
| rfc6810 | 2-4 | 35 | 0 | 0 | 1 / 0 / 0 | yes, 6/6 | a24e6dd47c da5a5e8ad1 |
| rfc6811 | 2-4 | 3 | 0 | 0 | 0 / 1 / 0 | yes, 1/1; verdicts 1/1 (BS-B, 2026-09-27) (note b) | 087840eb88 |
| rfc6996 | 2-4 | 1 | 0 | 0 | 0 / 1 / 0 | yes, 1/1; verdicts 1/1 (BS-B, 2026-09-27) (note b) | 09b3fa29b0 |
| rfc7011 | 2-4 | 17 | 7 | 1 | 7 / 5 / 0 | yes, 4/4; verdicts 2/2 | e8d2ce4780 f4c6a6d18c |
| rfc7012 | 2-4 | 5 | 0 | 0 | 0 / 1 / 0 | yes, 1/1; verdicts 1/1 (BS-B, 2026-09-27) (note b) | e1f9b1dc12 |
| rfc7166 | 2-4 | 22 | 0 | 0 | no audit file | yes, 3/3 | b3bdc5f296 |
| rfc7296 | 2-4 | 57 | 2 (+2 added) | 0 | 62 / 23 / 1 | no, 5/6; verdicts 13/17 (BS-B, 2026-09-27); sent back, re-read d8dbe3eccb: now 50 / 34 / 2 (note c) | be0c565ad3 d5780d60dc |
| rfc7311 | 2-4 | 10 | 0 | 0 | 6 / 5 / 1 | yes, 2/2; verdicts 2/2 | 573079b6cf |
| rfc7313 | 2-4 | 7 | 0 | 0 | 1 / 4 / 0 | yes, 2/2; verdicts 1/1 | d4f5b91b4d |
| rfc7427 | 2-4 | 5 | 0 | 0 | 2 / 1 / 0 | yes, 1/1; verdicts 1/1 (BS-B, 2026-09-27) (note b) | 0023de112a |
| rfc7432 | 2-4 | 39 | 1 | 0 | 6 / 3 / 0 | yes, 11/11; verdicts 1/1 | 5ea6e7665e 07d832e388 |
| rfc7440 | 2-4 | 7 | 0 | 1 | 0 / 1 / 0 | yes, 1/1; verdicts 1/1 (BS-C, 2026-09-27) (note b) | d54d79acf7 |
| rfc7454 | 2-4 | 64 | 0 | 0 | no audit file | yes, 6/6 | 1d1239a281 |
| rfc7474 | 2-4 | 10 | 0 | 0 | 5 / 6 / 0 | yes, 1/1; verdicts 1/2 | 35cecf4b36 43f89ff8d5 b12697744d |
| rfc7534 | 2-4 | 16 | 0 | 0 | 1 / 2 / 0 | yes, 2/2; verdicts 1/1 | 65847dc1f9 |
| rfc7535 | 2-4 | 6 | 0 | 0 | no audit file | yes, 1/1 | f8d5c62d3a |
| rfc7606 | 2-4 | 0 | 0 | 0 | 46 / 4 / 0 (+1 unimplemented) | none owed: no quote changed (marker text only) | 546e3765b9 412d9774ec |
| rfc7611 | 2-4 | 3 | 0 | 0 | 1 / 1 / 0 | yes, 1/1; verdicts 0/1 (BS-C, 2026-09-27); on hand re-read the verdict at HEAD stands, the blind reader was lenient (note b) | 06eaa85f68 |
| rfc7627 | 2-4 | 27 | 0 | 0 | no audit file | yes, 3/3 | a89111926c |
| rfc7684 | 2-4 | 15 | 0 | 0 | 2 / 3 / 0 | yes, 2/2; verdicts 1/1 (BS-C, 2026-09-27) (note b) | 3b608c767a |
| rfc7705 | 2-4 | 13 | 0 | 0 | 6 / 3 / 0 | yes, 2/2; verdicts 2/2 | f14d9233bf |
| rfc7752 | 2-4 | 29 | 0 | 2 | 0 / 1 / 0 | yes, 5/5 | 56a8ab19aa b327acd569 |
| rfc7770 | 2-4 | 18 | 0 | 0 | 3 / 1 / 0 | yes, 2/2; verdicts 1/1 | afc1c13a98 |
| rfc7854 | 2-4 | 40 | 0 | 0 | 26 / 8 / 2 | yes, 4/4 | e19195ad69 |
| rfc7858 | 2-4 | 20 | 0 | 0 | 3 / 3 / 0 | yes, 3/3; verdicts 1/1 | 7ace94d47c |
| rfc7871 | 2-4 | 58 | 0 | 0 | 2 / 0 / 1 | yes, 6/6; verdicts 1/1 (BS-C, 2026-09-27) (note b); 36d18b7e6d also says a blind read of its verdicts disagreed and they were re-judged strictly (D-9), with no output on record | 36d18b7e6d |
| rfc7911 | 2-4 | 10 | 0 | 0 | 2 / 5 / 2 | yes, 1/1; verdicts 1/2 | d62d5aba89 5861ce1750 |
| rfc792 | 2-4 | 12 | 0 | 0 | 3 / 5 / 0 | yes, 1/1; verdicts 2/2 (BS-C, 2026-09-27) (note b) | 92cd7642e3 4ea190082d |
| rfc7947 | 2-4 | 8 | 0 | 2 | 4 / 2 / 0 | no, 0/1; verdicts 1/1 (BS-C, 2026-09-27); on hand re-read the row's §2.1 sentence is the better fit, since it carries the SHOULD the row is levelled at, and the row stands (note b) | 2ba3009b61 |
| rfc7950 | 2-4 | 66 | 1 | 1 | 8 / 13 / 2 | yes, 8/8; verdicts 5/5 | 1da03fcde0 |
| rfc7999 | 2-4 | 14 | 0 | 0 | 1 / 6 / 0 | yes, 2/2; verdicts 1/1 | 34e16a33fd |
| rfc8050 | 2-4 | 6 | 1 | 0 | 1 / 3 / 1 | yes, 1/1; verdicts 1/1 (BS-C, 2026-09-27) (note b) | 9b8d470be4 |
| rfc8092 | 2-4 | 5 | 0 | 0 | 1 / 6 / 0 | yes, 1/1; verdicts 0/1 (BS-C, 2026-09-27); on hand re-read the verdict at HEAD stands, the blind reader was lenient (note b) | 26657c25ce |
| rfc8097 | 2-4 | 7 | 0 | 0 | no audit file | yes, 1/1 | d404fe39d3 |
| rfc8195 | 2-4 | 3 | 0 | 0 | no audit file | yes, 1/1 | 019e8d84ea |
| rfc8203 | 2-4 | 4 | 0 | 0 | 2 / 3 / 0 | yes, 1/1; verdicts 1/1 | b7f7c8067a |
| rfc8210 | 2-4 | 49 | 0 | 5 | 20 / 10 / 0 | 7/8; RFC8210-7-8 differs: the blind reader took the section 7 sentence 'Routers, however, MUST handle such notifications (by ignoring them)', the row quotes the section 7 sentence 'The router MUST ignore any Serial Notify PDUs ... during this initial startup period'. Same obligation; f783d1af12 records agreement, no re-read recorded. Sent back on 2026-09-27 and re-read in 1db09b782e, verdicts still 20 / 10 / 0; the duplicate is a merge candidate in `spec-rfc-verdict-test-fix-pass.md` | f783d1af12 1db09b782e |
| rfc8277 | 2-4 | 25 | 0 | 0 | 1 / 8 / 0 | yes, 4/4; verdicts 0/2 | 063b7e0836 a5332075a3 |
| rfc8326 | 2-4 | 1 | 2 | 0 | no audit file | yes, 1/1 | 9a8e154780 06adc861b1 |
| rfc8362 | 2-4 | 50 | 0 | 0 | un-enrolled, R-7 table lines | yes, 5/5 | d3d4d965e3 |
| rfc8414 | 2-4 | 13 | 0 | 0 | 5 / 2 / 0 | yes, 2/2; verdicts 1/1 | f9fbda2842 be0c565ad3 |
| rfc8484 | 2-4 | 7 | 0 | 0 | 0 / 1 / 0 | yes, 1/1; verdicts 1/1 (BS-C, 2026-09-27) (note b) | 1734643634 |
| rfc8654 | 2-4 | 6 | 0 | 0 | 3 / 5 / 0 | yes, 1/1; verdicts 2/2 | 484fb4581f |
| rfc8665 | 2-4 | 41 | 0 | 2 | 19 / 9 / 0 | yes, 4/4; verdicts 6/6 (BS-C, 2026-09-27) (note c) | b12697744d |
| rfc8666 | 2-4 | 35 | 0 | 0 | 19 / 4 / 0 | yes, 4/4; verdicts 5/5 (BS-C, 2026-09-27) (note c) | b12697744d |
| rfc8669 | 2-4 | 25 | 0 | 0 | 2 / 10 / 0 | yes, 4/4; verdicts 2/2 | e926c85230 855b60b622 |
| rfc8671 | 2-4 | 6 | 1 | 0 | 6 / 1 / 0 | yes, 1/1; verdicts 1/1 (BS-C, 2026-09-27) (note b) | 84f57a537f |
| rfc8707 | 2-4 | 6 | 2 | 0 | no audit file | yes, 1/1; no verdict sampled, no audit file (BS-C, 2026-09-27) (note c) | be0c565ad3 |
| rfc8907 | 2-4 | 34 | 0 | 0 | 30 / 18 / 0 | yes, 6/6; verdicts 9/10 | 35ec40699a |
| rfc8950 | 2-4 | 3 | 0 | 0 | 1 / 3 / 0 | yes, 1/1; verdicts 1/1 (BS-C, 2026-09-27) (note b) | bb216425d0 |
| rfc8955 | 2-4 | 26 | 0 | 0 | 20 / 2 / 1 | yes, 4/4; verdicts 3/4 | 808e5178f4 be0c565ad3 5484c65fbe |
| rfc8956 | 2-4 | 10 | 0 | 0 | 6 / 1 / 0 | yes, 1/1; verdicts 1/1 | 2ea1e30ec7 |
| rfc9003 | 2-4 | 7 | 0 | 0 | 1 / 1 / 0 | yes, 1/1; verdicts 1/1 (BS-C, 2026-09-27) (note b) | 7851d49d92 |
| rfc9012 | 2-4 | 40 | 0 | 0 | 12 / 2 / 1 | yes, 4/4; verdicts 1/3 (BS-C, 2026-09-27); sent back, re-read 49a91fdf9a: now 0 / 14 / 1 (note b) | b1b796824c |
| rfc905 | 2-4 | 7 | 3 | 0 | 0 / 4 / 0 | yes, 1/1 | be0c565ad3 ed9dd6a089 2920e3e3cb |
| rfc9069 | 2-4 | 11 | 0 | 1 | 10 / 5 / 0 | yes, 2/2; verdicts 3/3 | aed25fc2f2 |
| rfc9072 | 2-4 | 7 | 0 | 0 | 1 / 0 / 0 | yes, 1/1; verdicts 1/1 (BS-C, 2026-09-27) (note b) | 04f374b49a 7c7c100d13 |
| rfc9085 | 2-4 | 13 | 0 | 0 | 0 / 3 / 0 | yes, 1/1; verdicts 0/1 | 87fe43d8de 145e52ee25 |
| rfc9086 | 2-4 | 15 | 0 | 0 | 3 / 4 / 0 | yes, 2/2; verdicts 1/1 | ce3064a1a2 |
| rfc9129 | 2-4 | 6 | 1 | 0 | no audit file | yes, 1/1 | 9a8e154780 15cb157202 |
| rfc9136 | 2-4 | 9 | 0 | 0 | 2 / 1 / 0 | yes, 2/2; verdicts 1/1 | 8708380a1d |
| rfc9190 | 2-4 | 36 | 0 | 0 | un-enrolled, R-7 table lines | yes, 4/4; no verdict sampled, un-enrolled (BS-C, 2026-09-27) (note b) | c45c3bae04 |
| rfc9234 | 2-4 | 11 | 0 | 0 | 8 / 9 / 0 | yes, 2/2; verdicts 3/3 | 78bc7a7651 |
| rfc9252 | 2-4 | 22 | 0 | 0 | 10 / 7 / 0 | yes, 4/4; verdicts 4/4 | b1e0fe39f8 faf7e61351 |
| rfc9256 | 2-4 | 38 | 0 | 0 | 2 / 0 / 0 | yes, 5/5; verdicts 1/1 | 258c244a3b |
| rfc9319 | 2-4 | 7 | 0 | 0 | no audit file | yes, 1/1 | 106e78a9bf |
| rfc9384 | 2-4 | 1 | 0 | 0 | un-enrolled, R-7 table lines | yes, 1/1; no verdict sampled, un-enrolled (BS-C, 2026-09-27) (note b) | 277719527b |
| rfc9494 | 2-4 | 15 | 0 | 0 | 3 / 2 / 0 | yes, 4/4 | f10d01a8f7 3b2fffd32b |
| rfc9514 | 2-4 | 22 | 0 | 0 | 10 / 7 / 0 | yes, 2/2; verdicts 3/3 | 4c77d6ca97 1fb067ed90 |
| rfc9552 | 2-4 | 39 | 0 | 3 | 23 / 17 / 1 | yes, 7/7; verdicts 8/8 | 376c12d49a 9d5dea7721 |
| rfc9568 | 2-4 | 55 | 0 (+1 added) | 0 | 25 / 21 / 1 | yes, 8/8; verdicts 8/9 | 3ad1eb9542 9e4ee8ed38 850eb41b66 69a54af2ac |
| rfc9582 | 2-4 | 9 | 13 | 0 | no audit file | yes, 2/2 | ea5b4f197d cbcade6f68 be0c565ad3 b6fc04265e |
| rfc9687 | 2-4 | 18 | 0 | 0 | 10 / 3 / 0 | yes, 2/2; verdicts 2/3 | ea0813c52f |
| rfc9728 | 2-4 | 12 | 0 | 0 | 5 / 5 / 2 | yes, 1/1; verdicts 2/2 (BS-C, 2026-09-27) (note b) | 4ccad9c749 |
| rfc9830 | 2-4 | 87 | 0 | 0 | 34 / 25 / 0 | yes, 11/11; verdicts 12/12 | 3c98aaa424 ab02812d23 |
| sflow-v5 | 2-4 | 41 | 1 | 2 | 12 / 12 / 0 | yes, 5/5; verdicts 4/5 | 95427dc548 be0c565ad3 87ffa88c22 f5291c50aa |

How the table was built (2026-09-27, AC-10 pass): the 190 stems whose
`rfc/short/<stem>.md` differs between 0bf0696576 and HEAD. The closer's draft
held 180, because it kept only commits whose body names the spec, which dropped
the ten stems whose rows landed in defect-fix or D-10 commits
(draft-ietf-idr-bgp-bfd-strict-mode, rfc3748, rfc3768, rfc5216, rfc5880,
rfc5883, rfc7296, rfc8665, rfc8666, rfc8707).

| Column | What it counts |
|--------|----------------|
| Rows quoted | rows whose text changed, trailing `{...}` markers ignored |
| Retired | ids at 0bf0696576 and absent at HEAD. "+N added" counts ids new at HEAD, the D-10 moves and splits |
| R-7 verdicts | every verdict in `rfc/audit/<stem>.json` at HEAD, because the D-9 strict re-audit judged the whole file. "un-enrolled" stems have their lines in the R-7 section |
| Blind sample agreed | the D-4 blind outputs in the session's `scratch/work/sample/*.out.json` against the rows at HEAD. "yes, k/n" is k of n sampled quotes agreeing (same sentence or span in the same section; a retired row agrees with a blind "no-sentence"). "verdicts a/b" is a of b D-9 blind verdicts equal to the verdict at HEAD. A mismatch is a verdict at HEAD that differs from the blind reader's, and this table does not judge which is right |
| Commit | every commit since 0bf0696576 touching the stem's summary, audit or corrections file |

Totals: 3627 rows quoted, 81 retired, 9 added, 48 level changes; verdicts
1114 enforced, 855 weak, 59 wrong.

Blind sample: 128 stems sampled. 127 agree on every sampled quote, and rfc8210
differs on one quote, recorded in its row with no re-read on record. Two stems
owe no sample: rfc1877 quoted nothing and retired five rows, and rfc7606 changed
only the `{gap}` text of RFC7606-5.1-1 in 546e3765b9. The remaining 60 stems
had no blind output on record until 2026-09-27, when the samples BS-A, BS-B and
BS-C ran over them (`scratch/work/sample/wave-bsa.*`, `wave-bsb.*`, `wave-bsc.*`):

- note b (50 stems): the commit states that "The blind 10% second read agreed
  (D-4)", but `commit_stems.sh` writes that sentence into every commit it makes,
  whatever happened, and no blind output named the stem when the commit landed.
  The statement is not evidence of a sample; the cell now records the 2026-09-27
  sample instead.
- note c (10 stems): the rows landed in a defect-fix or D-10 commit, and no commit
  and no blind output recorded a sample before 2026-09-27. For rfc1071 the quotes
  came from job J099 and rode in be0c565ad3. wave7 was cut before J099 ran, and
  wave-qf did not include rfc1071.

In the 2026-09-27 samples, 104 of 108 sampled quotes and 102 of 130 sampled
verdicts agreed. Every stem with a disagreement was re-read: 14 of the 60 were
sent back, with rfc8210 as the fifteenth for its earlier quote difference (the 14
re-read commits are listed in the Mistake Log, and rfc3787 was re-read with no
change), and for rfc7611, rfc7947 and rfc8092 the flagged line was re-read by hand
in BS-C and the reading at HEAD stands. AC-10 now holds for all 60 stems.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation, and the per-stem table covers every stem the dry run listed |
| Correctness | A quote states the row's obligation, not a neighbouring one: the blind sample agrees |
| Retire discipline | Every retired row has its correction paragraph, and no tag, mapping or verdict still names it |
| Rule: no-layering | Phase 5 deletes the change scope and the ratchet, and nothing keeps both paths |
| Rule: never weaken a test | An R-7 finding fixes the test or the row, and never removes a tag to clear a verdict |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Unquoted backlog at zero | `./le rfc check` output, pasted |
| Change scope gone | grep for each AC-3 symbol returns nothing |
| R-7 verdicts | `./le rfc check` with no stale verdict, and the per-stem table sums to 986 |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | `rfc/corrections` parsing: a malformed paragraph must not accept a retirement it does not name |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| No sentence proposer | the backfill proposes a best-overlap sentence for the human rows | overlap is the mechanism that let the backfill quote sentences stating other obligations (R-7). Search is not the cost; judgement is |
| R-7 verdicts in `rfc/audit/<stem>.json` | a per-row table in this spec | audit verdicts already exist, are keyed on the row text, and stale by themselves when a row changes again |
| Delete the change scope at the end | widen it to every row and keep the ratchet | no-layering: with the whole corpus judged, the ratchet protects nothing |
| Tooling first, rule flip last | flip first and quote under a red gate | a red `./le rfc check` on every commit would block every other session |

## Known Limitations

- The strength-4 blind second walk of the prose stems stays in `plan/pre-release/spec-rfc-evidence-strength-4-prose-second-walk.md`. This spec quotes the rows it will compare against.
- Tagged tests of rows the backfill did NOT change, and that phases 3 and 4 do not quote, are outside R-7: their row text did not move.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete
- [ ] `./le verify worktree` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** remove the spec

## Mistake Log

### Wrong Assumptions

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2 expected about 5% of rows to have no supporting sentence, with a stop at 15% of a stem's rows | rfc9582 lost 13 of its 35 rows (37%): 9 in cbcade6f68, re-attributed under D-7 to the draft-ietf-sidrops-8210bis rows that state them; RFC9582-5.12-2 and 5.12-7 in be0c565ad3 under owner decision D-10; RFC9582-5.12-6 and 5.12-9 in b6fc04265e under D-2, since no document states an ASPA AFI field. The summary had carried ASPA and RTR obligations that belong to 8210bis, not the ROA profile. Across the 190 changed stems, 81 of 6339 rows left their stem (1.3%), and rfc9582 is the only stem of 20 or more rows above 15% | the per-stem retire count at closure (AC-10 table) | the retirements stand on D-7, D-10 and D-2, the owner's rulings on rows another document states or no document states. A-2 is recorded broken for rfc9582, and no row is restored |

The per-stem commit bodies that the session's helper script `commit_stems.sh`
wrote claimed "The blind 10% second read agreed (D-4)" whether or not a sample had
run, because the script put that sentence into every commit it made. For 60 stems
no sample had run (notes b and c of the per-stem table). The samples then ran on
2026-09-27 as BS-A, BS-B and BS-C, and 15 stems were sent back and re-read under
the strict rule: rfc3768 e07e40de0f, rfc5880 a6e945caa0, rfc2759 ebb18b811a,
rfc3748 2cf1b04765, rfc4301 d240103498, rfc7296 d8dbe3eccb, rfc2328 a2e975c045,
rfc1332 6f307a730b, rfc9012 49a91fdf9a, rfc4578 5f65b0a6e8, rfc4684 042a63f22e,
rfc5082 d25ba4d0d4, rfc6138 70d678a85f and rfc8210 1db09b782e, with rfc3787
re-read and left unchanged, so it has no commit. The weak and wrong verdicts,
split-needed rows and row-quality findings the re-reads produced are in
`plan/pre-release/spec-rfc-verdict-test-fix-pass.md`. A commit message states what
happened only when the step that writes it checks that it happened.

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| AC-9: RFC7606-3.g-2, 4-1, 7.10-2 and 7.14-1 are still `weak` in `rfc/audit/rfc7606.json`, and no new tag or discrimination record exists for them | D-15 moved every weak and wrong verdict out of this spec | `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` |
| The 21 weak and 1 wrong R-7 lines of the un-enrolled stems (draft-ietf-sidrops-8210bis, rfc1035, rfc8362, rfc9190), and the RFC9190-2.3-1 Method-Id gap | same D-15 move; they live only in this spec's R-7 table, so the fix pass names each in its "Un-enrolled R-7 rows" table and its goal and AC-8 cover them | `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` |
| The weak and wrong verdicts of enrolled stems from the strict re-read (838 weak, 56 wrong at D-15), and the split-needed rows | owner decision D-15: fixing them (test or row) runs by package in a separate spec | `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` |
| RFC3948-5.1-1 (`unimplemented`, `{gap}`): ze assigns no inner address, so nothing prevents the tunnel-mode conflict | an absent feature, recorded as a gap under `ai/rules/rfc-compliance.md`; its gap text was corrected at closure to name `reloadPool` (`apply.go`) | `plan/immediate/spec-ike-virtual-ip-assignment.md` |

## Implementation Summary

### What Was Implemented
- Phase 1 (9a8e154780): a stem with no numbered heading cites its whole text as one section (D-1); indented headings are read where a text has no column-0 heading (D-5); `checkRetiredRequirements` accepts an id named first by a dated `Retired` paragraph with at least one section read, and `checkIDAllocation` refuses a retired id again (D-2). `rfc/full/rfc8326.txt` and `rfc/full/rfc9129.txt` fetched.
- `./le rfc audit-stamp` (b6ad657bf2, `auditStamp` in `internal/le/rfc/audit_stamp.go`) stamps new verdicts with computed fingerprints (D-6); errata are stored under `rfc/errata/<stem>/` and a row citing one is judged against the corrected text (76fcaae9c0, `erratumRowRefusal` in `errata.go`, D-11).
- Phases 2 to 4: every unquoted row of 201 stems quoted verbatim, retired (D-2, D-7, D-10) or its level corrected (D-3), one commit per stem, with the blind sample per stem (AC-10) and the strict R-7 verdicts in `rfc/audit/<stem>.json`.
- Phase 5 (eb63fc5813): `checkRowQuotes` judges every row; the change scope, the unquoted ratchet and the backlog figure are deleted.
- D-8 defect fixes found by the re-read landed in code (BFD, OSPF, softver D-13, RFC 3101 translator D-12), each in its own commit.

### Bugs Found/Fixed
- Closure: the RFC3948-5.1-1 `{gap}` text, its Meta "Support remaining" cell, its extraction reason and its audit note named `registerIKE` discarding the pool at `_ = ipPool` in `register.go`. That line is gone: `reloadPool` (`internal/component/ike/engine/apply.go`) stores the pool in `ikeEngineState.pool`, which no code reads. All four texts, and the source anchor in `docs/guide/ipsec.md`, now name `reloadPool`. The gap itself still holds.
- b77dfd0996: three real-corpus tests asserted counts this spec's stem commits moved; fixed to the new truth.

### Documentation Updates
- `docs/contributing/rfc-conformance-gates.md`: "The row quote" states that every row is judged and the ratchet is gone (eb63fc5813); "Requirements do not vanish" states the `Retired` paragraph (9a8e154780); the whole-text section (D-1) is described.
- `docs/guide/ipsec.md`: source anchor repointed from `register.go` to `apply.go` `reloadPool` (closure).
- `docs/features/rfc-status.md` regenerated by `./le rfc index-update` (closure run: no diff).
- `./le doc check verify` at closure: "Documentation tests PASSED" (scratch `close3-doccheck.out`).

### Deviations from Plan
- AC-9 not met here: D-15 moved every weak and wrong verdict, the rfc7606 four included, to the fix-pass spec (Work Not Done).
- A-2 broken for rfc9582 (Mistake Log).
- AC-6: 3 of the 10 no-rfc-text rows were retired under D-2 (`RFC8326-x-2`, `RFC8326-x-3`, `RFC9129-2.5-2`), with paragraphs in `rfc/corrections/`; the other 7 are quoted.
- RFC3948-5.1-1 and RFC7296-2.4-2 were counted by the closers' AC-7 script only because plan prose and an audit note contain the words `RFC requirement:`. No test tags them, so AC-7 does not reach them; they carry verdicts anyway (e70b768c49, bdaca99582).

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `./le rfc check` shows no unquoted row and no unjudged stem | Done | `checkRowQuotes`, `internal/le/rfc/check_quote.go` | closure run: 18 violations, all foreign producer-changed discrimination records, none a row-quote refusal |
| The row-quote rule applies to every row | Done | `checkRowQuotes` | `TestCheckRefusesUnchangedUnquotedRow` passes |
| Tagged tests of each rewritten row re-read (R-7) | Done | `rfc/audit/<stem>.json`, the R-7 table above | real-tag AC-7 derivation reports missing 0 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `./le rfc check` at closure | no unquoted figure exists after the flip; no quote violation |
| AC-2 | Done | `TestCheckRefusesUnchangedUnquotedRow` | pass at closure |
| AC-3 | Done | `git grep -w` over `internal` and `cmd` | 0 hits for all 8 symbols |
| AC-4 | Done | `TestCheckAcceptsWholeTextQuoteInUnnumberedStem`, `TestCheckRefusesFrontCitationInNumberedStem` | pass |
| AC-5 | Done | `TestCheckRefusesRetiredRowWithoutCorrection`, `TestCheckAcceptsRetiredRowWithCorrection`, `TestCheckRefusesRetiredIDReuse` | pass |
| AC-6 | Done | `rfc/full/rfc8326.txt`, `rfc/full/rfc9129.txt`; rows quoted or retired | see Deviations |
| AC-7 | Done | `close3-ac7-realtags.py`: 2139 changed tagged rows, 37 un-enrolled with R-7 lines, missing 0 | verdicts eed9f9c665 .. bdaca99582 |
| AC-8 | Done | every weak, wrong or unimplemented verdict is homed: fix-pass spec (D-15), `{gap}` rows in their own specs | Work Not Done |
| AC-9 | Changed | owner decision D-15 | homed in the fix-pass spec |
| AC-10 | Done | per-stem table; BS-A, BS-B, BS-C samples | 14 re-read commits listed in the Mistake Log |
| AC-11 | Done | `rfc/corrections/<stem>.md` `Retired` paragraphs name sections read and tag moves | `retires` refuses a paragraph with no section |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| the 8 `TestCheck*` tests of the Unit Tests table and `TestRFCSelftestQuoteStageRefusesFabricatedRow` | Done | `internal/le/rfc/check_quote_test.go`, `check_ratchets_test.go` | 9 PASS at closure (scratch `close3-tests.out`) |
| Deleted ratchet tests | Done | `check_quote_test.go` | `git grep` for `TestCheckRefusesUnquotedCountRise`: 0 hits |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `check_quote.go`, `check.go`, `inventory.go`, `check_ratchets.go`, `quote_backfill.go` | Done | 9a8e154780, eb63fc5813 |
| `rfc/short`, `rfc/audit`, `rfc/corrections`, `rfc/extraction` | Done | stem commits |
| `docs/contributing/rfc-conformance-gates.md` | Done | phase 1 and phase 5 commits |

### Audit Summary
- **Total items:** 11 AC, 3 task goals
- **Done:** 10 AC, 3 goals
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (AC-9, owner decision D-15, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| `./le rfc check` prints no unquoted figure and names no unjudged stem | functional run over the real tree | closure `./le rfc check`: "rfc-requirements: 18 violation(s)", every one a `producer-changed` discrimination record (rfc4271 2, rfc7611 2, rfc7705 1, rfc7947 2, rfc8907 11) whose producer last moved in 447c5c24da or b40379c07c; no quote refusal, no unjudged stem |
| The row-quote rule covers every row | unit test through `Check()` | `TestCheckRefusesUnchangedUnquotedRow`: an unquoted row identical at `HEAD^` is refused; AC-3 symbols absent |
| Every backfill-rewritten tagged row re-read | derivation over git history | rows at `0bf0696576^` vs HEAD, tags from `RFC requirement:` in code and `.ci` only: 2139 changed tagged rows, missing 0; the 37 un-enrolled rows are in the R-7 table |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/rfc-requirement-quote-hand-backfill-f7fd86a1-7a8c-495e-89c0-011b55afa13e.md` (19 files, verdict clean) |
| `./le spec review check` | clean, run after the last spec edit |
| Rounds | 1 |
| Reviewer lenses used | logic and wiring over the le/rfc tooling (retirement, errata, audit-stamp, heading detection), security of the corrections and errata readers, citation and documentation drift |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | the RFC3948-5.1-1 gap text, Meta cell, extraction reason, audit note and the ipsec guide anchor named a `register.go` line that no longer exists | `rfc/short/rfc3948.md`, `rfc/extraction/rfc3948.json`, `rfc/audit/rfc3948.json`, `docs/guide/ipsec.md` | repointed to `reloadPool` (`apply.go`); `./le rfc check` shows no new violation |

### Run 1
- 0 BLOCKER, 0 ISSUE after finding 1 was fixed.
- NOTE: `erratumCarries` (`errata.go`) accepts a quote that is a span of a cited erratum's corrected text without checking that the erratum's section is the row's section. A row cites the erratum by number in its own parenthetical, so the pairing is explicit.
- NOTE: `./le commit audit`: clean (0 test files weakened). `./le repo check`: one ISSUE, a stale `internal/le/repo/numberparse-allowlist.txt` line for `internal/component/bgp/config/loader_create.go`, from b40379c07c and 65e4117e5f (another session's router-id work), not this spec.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `rfc/full/rfc8326.txt` | yes | ls: 22K, 2026-09-26 |
| `rfc/full/rfc9129.txt` | yes | ls: 218K, 2026-09-26 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-2, AC-4, AC-5 | refusals and acceptances | `go test -race -run` of the 9 tests under `./le job run`: all PASS, `ok internal/le/rfc 7.649s` |
| AC-3 | symbols gone | `git grep -c -w` for each of the 8: 0 |
| AC-7 | verdict or R-7 line for every changed tagged row | `close3-ac7-realtags.py`: "changed tagged rows 2139 unenrolled 37 missing 0" |
| AC-1 | no quote violation | `./le rfc check`: 18 violations, `grep -vc producer-changed` = 0 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le rfc check` | none: the entry is driven through `Check()` by the named Go tests, and by the real-tree run above | yes |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | of 1511 unsourced rows, most were quoted; across the 190 changed stems 81 of 6339 rows left their stem (1.3%) |
| A-2 | broken | rfc9582 retired 13 of 35 (Mistake Log) |
| A-3 | confirmed | spans carried most rows; about 80 split-needed rows remain, homed by D-15 in the fix-pass spec |
| A-4 | confirmed | every stem commit ran `./le rfc check`; the closure run shows no sign-off, discrimination-claim or render refusal caused by a reword |
| A-5 | confirmed | `rfc/audit/rfc1661.json` held 14 verdicts before eed9f9c665 and passed the check |
| A-6 | confirmed | `auditStamp` refuses an un-enrolled stem; rfc9494's audit file was created in 3dbbfe8154 and passes; un-enrolled rows are in the R-7 table |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| gates page: every row judged, no ratchet | `checkRowQuotes`; AC-3 grep | yes |
| gates page: `Retired` paragraph | `retires`, `retiredIDs` in `check_ratchets.go` | yes |
| `docs/guide/ipsec.md` pool anchor | `reloadPool`, `apply.go`: "NOTHING READS s.pool YET" | yes |
| `./le doc check verify` | "Documentation tests PASSED" | yes |
