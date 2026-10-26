# Spec: rfc-verdict-fix-ike-eap

| Field | Value |
|-------|-------|
| Status | verification |
| Priority | P2 |
| Release reason | Required evidence for shipped behavior or truthful release support claims; historical work and proof debts remain distinct. |
| Scope | tooling |
| Depends | `plan/release/validation/spec-rfc-verdict-test-fix-pass.md` (the parent: its `audit-stamp` `mode rejudge` phase before any re-judge here, and its narrowing-audit output for the IKE/EAP group before any row edit, parent R-11) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

This spec is child 5 of 8 of `plan/release/validation/spec-rfc-verdict-test-fix-pass.md`, the IKE/EAP group. The parent holds the method, the inventory and the cross-cutting work. This child carries the test fixes, row edits and D-8 code fixes for its packages and owned stems, and closes on its own.

**Goal (AC-C1).** The derived weak-or-wrong listing restricted to this child's packages and owned stems holds only the verdicts listed under "Blocked by" below, each named in the acceptance criteria of the spec that blocks it. Every other verdict is resolved as the parent's Goal states: a test proves the whole quoted sentence in both polarities and is re-judged `enforced` by an agent that did not write it, or the row is corrected, retired or re-attributed and re-judged against its new text.

| ID | Owner decision (parent, 2026-09-28) |
|----|------|
| P-1 | D-15 is standing approval for every tagged-unit edit and tag move: record `./le rfc approve unit <pkg>.<Test> reason "D-15: ..."` before the edit, and carry the `RFC-approved:` trailer in the commit |
| P-2 | a recorded verdict is replaced only through `./le rfc audit-stamp stem <stem> from <pending> mode rejudge`; an audit entry is never hand-deleted |
| P-3 | this child closes when every verdict in its scope is `enforced`, except those listed under "Blocked by"; each blocked verdict moves into the blocking spec's acceptance criteria |

Measured 2026-09-28 over `rfc/audit/*.json` with the filter below: **111 weak and 11 wrong, 122 in all, over 10 stems**. The parent's Split table counted a cross-group verdict in each child and predates the last two file moves, so some of its group figures differ. This count is exclusive (each verdict in exactly one child) and leaves out the parent's five cross-group verdicts and RFC7296-2.4-2.

### Packages it owns

Test packages: `internal/component/ike/...`, `internal/core/eap`, and `test/parse` (the refined move, rfc7296).

| Package | Weak + wrong (2026-09-28) | Of which wrong |
|---------|--------------|----------------|
| `internal/core/eap` | 49 | 1 |
| `internal/component/ike/engine` | 37 | 4 |
| `internal/component/ike/dataplane` | 11 | 3 |
| `internal/component/ike/crypto` | 8 | 1 |
| `internal/component/ike/wire` | 8 | 0 |
| `internal/component/ike/transport` | 5 | 0 |
| `internal/component/ike/ipsec` | 4 | 1 |
| `internal/plugins/ospf` | 4 | 1 |
| `test/parse` | 1 | 0 |

A verdict whose units span two of these packages counts in each.

### Derived listing

The verdict inventory is DERIVED. No id list is committed here, because the audit files change as the work proceeds.

| Question | Derived by |
|----------|-----------|
| Every weak or wrong verdict in this child's scope (stem, id, verdict) | `jq -r --arg pk '^(internal/component/ike/\|internal/core/eap/\|test/parse/)' --arg own '^(rfc4301\|rfc4303)$' --arg fo '^$' 'input_filename as $f \| ($f\|ltrimstr("rfc/audit/")\|rtrimstr(".json")) as $s \| .requirements \| to_entries[] \| select(.value.verdict=="weak" or .value.verdict=="wrong") \| select(.key\|IN("RFC1071-1-4","RFC4303-2.1-1","RFC5882-4.4-1","RFC905-x-3","RFC905-x-4")\|not) \| select((($s\|test($own)) or ([(.value.tests // {})\|keys[]\|test($pk)]\|any)) and ($s\|test($fo)\|not)) \| "\($s)\t\(.key)\t\(.value.verdict)"' rfc/audit/*.json` |
| What the arguments mean | `pk` matches a tagged test path in this child's packages; `own` adds this child's owned stems tagged from another child's packages; `fo` drops stems another child owns (`^$` matches none); the five ids are the parent's cross-group verdicts |

**Wiring, 2026-09-28, at `cb4801b4cc`:** the filter returns 111 weak and 11 wrong, 122 in all, over 10 stems. This equals the Task figure, so `fefe38b68c` (it re-judged only the five cross-group verdicts, which the filter drops) and `cb4801b4cc` (it re-stamped IKE audit entries) changed no count here. `./le rfc audit-stamp` accepts `mode rejudge` (A-1). The "Blocked by" verdicts are now acceptance criteria of their blocking specs: `spec-ike-eap-rfc-defects` AC-3 and `spec-ipsec-rfc9190` AC-9 (the nine R-7 ids, to be enrolled there).

### Owned stems

One writer per ledger file: this child alone writes `rfc/audit/<stem>.json`, `rfc/discrimination/<stem>.json`, `rfc/short/<stem>.md`, `rfc/corrections/<stem>.md` and `rfc/extraction/<stem>.json` for these stems.

| Stem | Weak + wrong (2026-09-28) | Of which wrong |
|------|--------------|----------------|
| rfc2759 | 8 | 0 |
| rfc3748 | 23 | 0 |
| rfc3948 | 5 | 2 |
| rfc4301 | 17 | 5 |
| rfc4303 | 4 | 1 |
| rfc4555 | 2 | 0 |
| rfc5216 | 18 | 1 |
| rfc5282 | 8 | 0 |
| rfc7296 | 36 | 2 |
| rfc7427 | 1 | 0 |
| rfc9190 | 0 in `rfc/audit/` | un-enrolled: its twelve R-7 rows; no audit file |

Test files that carry tags of two owners. A child edits one only while the other holds no uncommitted change in it:

| File | Shared by |
|------|-----------|
| `internal/plugins/ospf/config_ipsec_test.go`, `internal/plugins/ospf/ipsec_install_test.go` | OSPF (their package) and this child, which owns rfc4301 and rfc4303 (RFC4301-4.2-1, RFC4303-1-1, 2-1, 2.1-2) |

### Un-enrolled R-7 rows

Copied verbatim from the parent. A row leaves this table when its re-judged verdict is `enforced`, recorded here with the date, or in the stem's audit file if the stem is enrolled first.

| ID | Verdict | Unit | What the test fails to prove |
|----|---------|------|------------------------------|
| RFC9190-1-1 | weak | `TestEAPTLSCapsBothRolesAtTLS13`, `TestEAPTLSVersionCapLeavesTLS12Reachable` (`internal/core/eap/rfc9190_version_cap_test.go`) | a refusal above TLS 1.3: the negative proves the neighbouring not-a-pin rule, and no `{single-polarity}` marker says why none can exist |
| RFC9190-2.1.2-2 | weak | `TestEAPTLS13IssuesASessionTicketTheNextExchangeRedeems` (`rfc9190_resumption_test.go`) | the declared lifetime: no assertion reads it, the bound rests on crypto/tls's client, and there is no negative or marker |
| RFC9190-2.1.3-1 | weak | `TestEAPTLS13IssuesASessionTicketTheNextExchangeRedeems`, `TestEAPTLS13ResumptionOffRunsAFullHandshakeAndStillIssuesATicket` (`rfc9190_resumption_test.go`) | the resumed session's version is never read, and the negative proves the neighbouring full-handshake fallback |
| RFC9190-2.1.8-2 | weak | `TestEAPTLS13PeerSendsAnAnonymousNAIAndKeepsTheRealm`, `TestEAPMSCHAPv2PeerSendsItsConfiguredIdentity` (`rfc9190_nai_test.go`), `TestEAPTLSPeerDropsTheUsernameTheCertificateCarries` (`rfc9190_cert_nai_test.go`) | "(or any other permanent identifiers)": only the username is searched for; realm-less and IP-address identities sit in units tagged 2.1.8-5 and 2.1.8-3 |
| RFC9190-2.1.8-3 | weak | `TestEAPTLSPeerAnonymizesEveryConfiguredIdentity`, `TestNAIGrammarMatchesRFC7542Section22` (`rfc9190_nai_test.go`) | the certificate-derived NAI (`certificateNAIs`, `realmNAI` in `nai.go`) is never run through `validNAI` |
| RFC9190-2.1.8-4 | weak | `TestEAPTLS13AuthenticatorTreatsAnEmptyCertificateListAsTerminal` (`rfc9190_nai_test.go`) | the peer's TLS 1.3 processing; only the server's empty certificate_list is driven |
| RFC9190-2.1.8-5 | wrong | `TestEAPTLS13PeerSendsTheFixedUsernameWhenTheIdentityHasNoRealm` (`rfc9190_nai_test.go`) | the recommended "@realm" form; the unit proves the fixed-username construction the next clause allows, and the "@realm" test carries only the 2.1.8-2 tag |
| RFC9190-2.3-1 | weak | `TestRFC9190MSKIsTheExportUnderTheRFCLabel` (`internal/core/eap/rfc5216_msk_label_test.go`) | Method-Id: no non-test code derives `EXPORTER_EAP_TLS_Method-Id` or a Session-Id. An implementation gap, recorded under `ai/rules/rfc-compliance.md`; implementing it needs the owner's scope |
| RFC9190-5.4-1 | weak | the eight tagged units in `rfc9190_revocation_test.go`, `TestEAPTLS13ResumptionStillNeedsARevocationSource` (`rfc9190_resumption_refusal_test.go`), `checkResponderEAPTLS13RevokedClient` (`internal/le/interoplab/ipsec/checkers.go`) | on the peer: a revoked intermediate in the authenticator's chain (only a 5.4-3 OCSP unit), and a refusal when the peer holds no current list |
| RFC9190-5.4-2 | weak | `TestEAPTLS13StaplesTheConfiguredOCSPResponse`, `TestEAPTLS13StaplesNothingWhenTheCertificateCarriesNoResponse` (`rfc9190_ocsp_test.go`) | the RFC 8446 Section 4.4.2.1 half: no status to a client that sent no status_request; every client in the suite is crypto/tls, which always sends it |
| RFC9190-5.4-3 | weak | the thirteen tagged units in `rfc9190_ocsp_test.go` | "abort the handshake with an appropriate alert": no unit reads the alert the authenticator receives |
| RFC9190-5.7-1 | weak | `TestEAPTLS13ResumptionRefusesARevokedClientChain` (`rfc9190_resumption_test.go`), `TestEAPTLS13TicketIsNotRedeemableUnderAnotherPeeringsKey` (`rfc9190_resumption_refusal_test.go`) | that authorization rests on cached data: a full handshake with the same revoked certificate also passes the positive, and neither unit shows a resumption authorized on valid cached data |
| RFC9190-5.7-5 | weak | `TestEAPTLSPeerDropsAStoredTicketAtTheSection57Ceiling` (`rfc9190_resumption_test.go`), `TestEAPTLS13RefusesATicketPastTheSection57Lifetime` (`rfc9190_resumption_refusal_test.go`) | "regardless of the PSK or ticket lifetime": the ticket's own lifetime is 604800 seconds, so a store expiring on the ticket lifetime passes both halves |

### Split-needed rows

Copied verbatim from the parent. The parent's narrowing audit adds rows for this group, and this child takes them when that output is committed. "Row correction" marks a claim no sentence states.

| ID | Dropped obligation | Section |
|----|--------------------|---------|
| RFC4555-x-4 | address advertisement later in INFORMATIONAL requests; the initiator MAY put it in the UPDATE_SA_ADDRESSES request | §3.6 |

### Missing rows

None of this table's rows belong to this child.

### Row-quality corrections

| Kind | Rows | What is wrong | Correction owed |
|------|------|---------------|-----------------|
| list-pointer quote | RFC4301-4.4.1.1-1, 4.4.1-6, 4.4.2.1-1, 5.1-1, 5.2-3, 6.2-2 | each quote ends in "the following ...:" and states no obligation; the list that carries it is not quoted, and the verdicts were judged against the list | widen the span through the list, or split one row per list item |
| fragment quote, referent in a sibling row | RFC7296-3.1-8, 3.2-2, 3.2-4, 3.2-6, 2.10-2, 2.10-3, 2.21.4-4, 2.21.4-6, 2.5-7, 3.5-3, 3.14-3 | a sub-span of a sentence whose subject is in a sibling row's clause | widen to the list-item head |
| fragment quote, no referent | RFC7296-3.16-1, 3.16-2, 3.11-2, 3.1-9, 3.1-11 | the quote names no subject, and the span can be widened without a sibling | widen the span |
| level over a weaker keyword | RFC2759-x-8, x-9 | levelled MUST over a sentence whose only keyword is SHOULD | re-level, with a correction paragraph |
| level with no BCP 14 keyword | RFC7296-1.2-1, 2.6-1, 2.9-1, 2.23-3, 2.23-12, 2.4-1, 1.4-1, 2.8-2 (MUST NOT), 2.2-3; RFC4301-4.1-4, 7-1 (MUST NOT); RFC3748-4-2, 2-2, 4.2-1; RFC2759-x-3, x-10, x-12 (format and vector text); RFC3768-6.4.3-9 (pseudocode); RFC5301-3-8 (MUST NOT over "The string is not null-terminated."); RFC8050-4.2-1 (descriptive); RFC8050-x-4 (rationale) | D-3 permits a MUST level over a normative sentence without a keyword, so these are for review, not automatic demotion. SB-2 counted 17 of them in its four stems, the blind samples 4 more | review each; a demotion needs a correction paragraph per stem and an owner call on the format-definition rows |
| duplicate span | RFC7296-3.3.2-1 and RFC7296-3.3.6-1 | both now quote the same §3.3.3 span ("MUST understand all types" through the IKE line of the table); 3.3.6-1 is the D-H subset of 3.3.2-1 | merge, moving the tags of 3.3.6-1 |
| duplicate span | RFC3748-7.10-1 and 7.10-2 | 7.10-2 quotes a sub-span of 7.10-1's sentence; the tags of each cover a different half, so both are weak | merge, keeping every tag |
| duplicate span | RFC3748-4.1-11 against RFC3748-4.1-5 and RFC3748-2.1-3 | its first sentence duplicates 4.1-5, its Nak-after-non-Nak sentence duplicates 2.1-3 | merge into the two rows |

- The row starting `level with no BCP 14 keyword`: this child owns the RFC7296, RFC4301, RFC3748 and RFC2759 ids only; the other ids go to the child that owns their stem.

### Mistagged units

| Unit | Tagged under | What it proves instead | Correction owed |
|------|--------------|------------------------|-----------------|
| `internal/component/ike/engine/rfc7296_child_rekey_initiator_answer_test.go::TestChildRekeyAnswerWithoutTrafficSelectorsIsRefused`, and `rekey_test.go::TestRekeyWithoutTrafficSelectorsIsRefused` | RFC7296-2.9-1 | TS payload presence in a rekey answer, a neighbouring obligation; the verdict stays `enforced` on the other units | move the tags to the row that states the TS presence rule, or add it |
| `internal/component/ike/engine/rfc4301_spd_discard_test.go::TestSPDPolicyMirrorsTheInboundSelector` | RFC4301-4.4.1-4 | direction mirroring, not administrator ordering; the verdict stays `enforced` on the ordering units | move the tag |

### Row work outcomes, 2026-10-11 (Steps 11, 12 and 13, row side)

Written by the author of these row changes, who judged no verdict. Every changed or new row below waits for an independent judge (list under "Verdicts owed to an independent judge"). Each correction is a dated paragraph in `rfc/corrections/<stem>.md`.

| Row | Table | Outcome |
|-----|-------|---------|
| RFC4555-x-4 | split-needed | §3.6 half is now `RFC4555-3.6-2` [MAY] (INFORMATIONAL address update) and `RFC4555-3.6-3` [MAY] (in the same request as UPDATE_SA_ADDRESSES); positive tags on the new `TestMobikeUpdateCarriesItsAddressList`, revert records on `startMobikeRequest`. Ungated MAY rows: no negative, since Ze has one address and never sends ADDITIONAL_*_ADDRESS. Correction in `rfc/corrections/rfc4555.md` (new file) |
| RFC4555-3.9-1 | narrowing | §4.2.6 data format is now `RFC4555-4.2.6-1` [MUST under D-3]; both polarities on the new `TestMobikeNoNATsDataIsTheSendTuple`, revert records on `noNATsAllowedPayload` (positive) and `noNATsAllowedMatches` (negative); listed in `unsourced-ids` of section 4.2.6 in `rfc/extraction/rfc4555.json` |
| RFC3748-2.3-1 | narrowing | no row: dated correction in `rfc/corrections/rfc3748.md`. The "Unless ..." sentence has no keyword and binds a pass-through forwarding decision; Ze implements its methods locally and never forwards a remote peer's EAP |
| RFC4303-3.4.4.1-1 | narrowing | step 3 padding SHOULD is now `RFC4303-3.4.4.1-2` [SHOULD] `{gap}`: Ze's Go does no ESP processing and nothing here shows Linux XFRM inspects the Padding bytes. Correction in `rfc/corrections/rfc4303.md` |
| RFC4301-4.4.1-6 | narrowing, list-pointer | re-quoted from the "as follows:" lead-in to the SPD-S item, which its tagged units already assert; SPD-I and SPD-O stay -11 and -12. Correction in `rfc/corrections/rfc4301.md` |
| RFC4301-4.4.1.1-1, RFC4301-6.2-2 | list-pointer | stand, dated correction each: both are `{gap}` rows; the per-item split comes with the work that closes the gap |
| RFC4301-4.4.2.1-1, 5.1-1, 5.2-3 | list-pointer | resolved 2026-09-30 by earlier corrections (5.1-1 and 5.2-3 retired) |
| RFC7296-3.1-8, 3.2-2, 3.2-4, 3.2-6, 2.21.4-4, 2.21.4-6, 3.5-3, 3.14-3 | fragment, sibling referent | widened to start at the subject or list-item head, through the sibling clause, as `RFC7296-2.10-2` was on 2026-09-30; each correction names the clause the row keeps |
| RFC7296-3.16-1, 3.16-2, 3.1-9, 3.1-11 | fragment, no referent | widened to the list-item head; same id, level, tags |
| RFC7296-2.10-2, 2.10-3, 2.5-7, 3.11-2 | fragment | resolved 2026-09-30 by earlier corrections |
| RFC2759-x-8, x-9 | level over SHOULD | resolved 2026-09-28: both re-levelled SHOULD under the `RFC2759-x-8` correction |
| RFC7296-2.6-1, 2.9-1, 2.23-3, 2.23-12, 2.4-1, 2.2-3, 1.2-1, 2.8-2; RFC4301-4.1-4, 7-1; RFC3748-4-2, 2-2, 4.2-1 | level, no keyword | reviewed under D-3: each states a normative protocol fact Ze must hold (a header layout, an exchange shape, a narrowing rule, the no-negotiated-lifetime rule behind 2.8-2's MUST NOT), so each keeps its level; no correction |
| RFC7296-1.4-1 | level, no keyword | owner ruled 2026-10-11 ("SHOULD (Recommended)"): re-levelled SHOULD, same id, quote and tags; `Correction 2026-10-11` paragraph in `rfc/corrections/rfc7296.md`. `./le rfc check` does not report its audit verdict stale |
| RFC2759-x-3, x-10, x-12 | level, format definition | owner ruled 2026-10-11 ("x-3 MUST, retire x-10/x-12 (Recommended)"): x-3 stays MUST, unchanged. x-10 (§9.2 password octets) and x-12 (§9.1.3 example flow) retired by two `Retired 2026-10-11` paragraphs in `rfc/corrections/rfc2759.md`; their eight tags deleted (no row states the obligation), the five tests kept as vectors under `./le rfc approve` rows quoting the owner; their eight discrimination records, two audit verdicts and two `unsourced-ids` entries in `rfc/extraction/rfc2759.json` dropped |
| RFC9190-2.3-1 | R-7 | Method-Id half split into `RFC9190-2.3-4` [SHALL] `{gap}` (unsourced in the 2.3 section of `rfc/extraction/rfc9190.json`); `TestRFC9190MSKIsTheExportUnderTheRFCLabel` positive now carries a revert record on `exportEAPTLSKeys` |
| RFC9190-2.1.2-2, 2.1.3-1, 5.7-1 | R-7 | row side unchanged; every tagged unit already carries a record; verdict owed |

### Verdicts owed to an independent judge (2026-10-11)

None of these was judged by its author. rfc9190 is not enrolled; `./le rfc audit-stamp` creates `rfc/audit/rfc9190.json` from a pending file, and a skeleton with empty verdicts is at `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/pending-audit-rfc9190.json`.

| Stem | Ids | Route |
|------|-----|-------|
| rfc7296 | 3.1-8, 3.1-9, 3.1-11, 3.2-2, 3.2-4, 3.2-6, 2.21.4-4, 2.21.4-6, 3.5-3, 3.14-3, 3.16-1, 3.16-2 (STALE: text changed) | `audit-stamp mode rejudge` |
| rfc4301 | 4.4.1-6 (STALE: text changed) | `audit-stamp mode rejudge` |
| rfc4555 | 4.2.6-1, 3.6-2, 3.6-3 (new) | `audit-stamp` (mode new) |
| rfc4303 | 3.4.4.1-2 (new `{gap}`) | `audit-stamp` (mode new) |
| rfc9190 | 2.1.2-2, 2.1.3-1, 2.3-1, 5.7-1, 2.3-4 (new `{gap}`) | `audit-stamp` (mode new, creates the file) |

Judged 2026-10-11 by an independent judge (author of none of the rows or tests). Each note in `rfc/audit/<stem>.json` quotes the RFC and names the units read.

| Id | Verdict | Stamped | Reason |
|----|---------|---------|--------|
| RFC7296-3.1-8 | weak | yes | the X-bit positive calls `handleSAInitRequest` directly, skipping `handleResponderInbound` and the dispatch where the flags octet is judged, so a drop-on-X-bit break there stays green |
| RFC7296-3.1-9 | enforced | yes | exact R-bit equality on every built message; receive negative isolated by a one-bit difference |
| RFC7296-3.1-11 | weak | yes | send clause enforced; the receive clause has the same dispatcher bypass as 3.1-8 |
| RFC7296-3.2-2 | enforced | yes | critical Nonce parses identically at outer and inner chain |
| RFC7296-3.2-4 | enforced | yes | built-chain sweep plus engine source walk |
| RFC7296-3.2-6 | enforced | yes | sender choice reaches the wire as 0x80 and draws a whole-message rejection; clear otherwise |
| RFC7296-2.21.4-4 | enforced | yes | exactly one INVALID_IKE_SPI Notify asserted, unit and `.ci` |
| RFC7296-2.21.4-6 | weak | yes | the input is a payload-less request header, not an unprotected Notify; negative claims the same notify but sends an IKE Delete |
| RFC7296-3.5-3 | enforced | yes | all four types accepted with anti-vacuity; negative claim names ID_DER_ASN1_DN, body tests GN only (defect) |
| RFC7296-3.14-3 | enforced | yes | five IV shapes accepted across the pair; positive claim names a repeat its body does not send (defect) |
| RFC7296-3.16-1 | enforced | yes | identifier echoed exactly over 200, 7, 91 |
| RFC7296-3.16-2 | enforced | yes | eapLen == payloadLen-4 over five sizes |
| RFC7296-1.4-1 | enforced (re-judged at SHOULD) | yes | owner moved MUST to SHOULD in 970ea021d8; the paired-Delete response is asserted exactly, and the lower level asks less |
| RFC4301-4.4.1-6 | enforced | yes | the SPD-S pair reads back all three parts; the two disposition tests still tag this row with SPD-I/SPD-O claims (defect: retag to -11/-12) |
| RFC4555-4.2.6-1 | weak | yes | data clause enforced; the type 16402 the row quotes is never pinned (test and producer share `wire.NotifyNoNATsAllowed`) |
| RFC4555-3.6-2 | enforced in substance | no | positive discriminates, but a MAY row with no negative and no `{single-polarity}` cannot hold `enforced`; the row owes `{single-polarity: positive; ...}` |
| RFC4555-3.6-3 | enforced in substance | no | same as 3.6-2 |
| RFC4303-3.4.4.1-2 | unimplemented | yes | Ze's Go does no ESP decryption; `xfrmStateFromParams` asks for no padding check; the `{gap}` should also name the VPP backend |
| RFC9190-2.1.2-2 | enforced in substance | no | not enrolled; also positive only: crypto/tls fixes the lifetime at 7 days (`serverHandshakeStateTLS13.sendSessionTickets`) and the Go client refuses longer (`clientHandshakeStateTLS13` ticket handling, `maxSessionTicketLifetime`) |
| RFC9190-2.1.3-1 | enforced | no (not enrolled) | TLS 1.3 pinned, both ends resume on the second exchange |
| RFC9190-2.3-1 | weak | no (not enrolled) | Key_Material half exact; the Method-Id half the row still quotes has no producer and no `{partial}` |
| RFC9190-5.7-1 | weak | no (not enrolled) | resumed path not asserted; peer role untested; negative proves ticket-key isolation |
| RFC9190-2.3-4 | unimplemented | no (not enrolled) | no non-test code exports EXPORTER_EAP_TLS_Method-Id |

`./le rfc audit-stamp` refuses `rfc9190`: "'rfc9190' is not an enrolled stem", so the sentence above about it creating the file is wrong. The judged rfc9190 entries are in `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/new-rfc9190.json`, ready to stamp once the owner enrols the stem.

Mechanical, not a judgement: appending two tests to `mobike_test.go` SHIFTED ten rfc4555 verdicts (x-1, x-2, 3.9-1, 3.7-1, 3.8-1, 3.8-2, 3.7-4, 3.7-5, 4.2.1-1, 4.2.5-1) whose units are byte-identical; `./le rfc reseal` re-stamps them when no other child holds uncommitted audit changes.

Mechanical, not a judgement: deleting the x-10 and x-12 tags from `rfc2759_clause_test.go`, `rfc2759_failure_packet_test.go` and `rfc2759_mschapv2_test.go` SHIFTED seven rfc2759 verdicts (x-1, x-2, x-3, x-4, x-6, x-8, x-9) whose units are byte-identical; `./le rfc reseal` re-stamps them.

### Fix round for the judge's findings (owner decisions A and B, 2026-10-11)

Thomas, 2026-10-11: A "Enrol it (Recommended)" for RFC 9190; B "Approve the fixes (Recommended)" for the tagged tests the judge found weak or mis-claimed.

| # | Finding | Fix |
|---|---------|-----|
| 1 | RFC7296-3.1-8, 3.1-11 receive test bypassed the dispatch | `TestXBitsIgnoredOnReceipt` drives every case through `PeerSession.handleResponderInbound` |
| 2 | RFC7296-2.21.4-6 input carried no Notify; the negative sent a Delete | the input is the bytes `sendInvalidIKESPI` emits (one unprotected INVALID_IKE_SPI Notify, asserted), a sentinel shows no datagram answers it, and the negative sends the same Notify inside SK |
| 3 | RFC7296-3.5-3 negative claimed ID_DER_ASN1_DN | the claim names ID_DER_ASN1_GN only and says why DN is not a refused type (RFC7296-4-4) |
| 4 | RFC7296-3.14-3 positive claimed an unsent repeat IV | the claim names the three IVs it sends and points the repeat to the negative |
| 5 | RFC4301-4.4.1-6 disposition tests | retagged to RFC4301-4.4.1-11 and -12, both polarities each |
| 6 | RFC4555-4.2.6-1 type 16402 never pinned | literal 16402 asserted on the decoded notify; the IPv6-width negative is the packet's own tuple at 16 octets |
| 7 | RFC4555-3.6-2, 3.6-3 | `{single-polarity: positive; ...}` on both rows |
| 8 | RFC4303-3.4.4.1-2 `{gap}` named only XFRM | it names VPP (`vppBackend.InstallSA`, `vppSAFlags`) as well |
| 9 | RFC9190-2.3-1 Method-Id half; 5.7-1 resumed path and peer role | `{partial: tested "Key_Material"; gap "Method-Id"; ...}` plus the new negative `TestRFC9190MSKIsNotThePreTLS13Derivation`; both 5.7 refusal tests assert the second exchange resumed; the peer role is tagged on `TestEAPTLS13ResumptionRefusesARevokedAuthenticatorChain`; `TestEAPTLS13CompletesAResumptionWithAnUnrevokedChain` is tagged RFC9190-5.7-1 negative |
| 10 | rfc7296 Meta claimed all rows MET and proven | the reason defers to `./le rfc check` and the audit file, and claims no row is met |

Commits 2691e815b1 (IKE tests and records) and 0be0218ef1 (EAP tests, records, RFC9190-2.3-1 `{partial}`). Verdicts this round makes stale or newly owed, for an independent judge (the author judged none):

| Stem | Ids | Mode | Verdict (independent judge, 2026-10-11) |
|------|-----|------|-----------------------------------------|
| rfc7296 | 3.1-8, 3.1-11, 2.21.4-6, 3.5-3, 3.14-3, and 3.14-6 if its unit fingerprint moved | rejudge | 3.1-8 weak, 3.1-11 weak, 2.21.4-6 weak; 3.5-3, 3.14-3, 3.14-6 enforced |
| rfc4301 | 4.4.1-6 (lost two units), 4.4.1-11, 4.4.1-12 (gained them) | rejudge | all three enforced |
| rfc4555 | 4.2.6-1 | rejudge | enforced |
| rfc4555 | 3.6-2, 3.6-3 (now `{single-polarity}`) | new | both enforced |
| rfc4303 | 3.4.4.1-2 (gap text names VPP) | rejudge | unimplemented (gap stands; code map now names vpp.go InstallSA and vppSAFlags) |
| rfc9190 | 2.1.2-2, 2.1.3-1, 2.3-1 (now `{partial}`, so the prepared `weak` entry is stale), 5.7-1 (tests changed), 2.3-4 | new, once the stem is enrolled | not judged (not enrolled, owner decision pending) |

Still weak after the fix round, with the reason recorded in `rfc/audit/rfc7296.json`:

| Id | Why | Fix owed |
|----|-----|----------|
| RFC7296-3.1-8, 3.1-11 (receive clause) | The test enters at handleResponderInbound and never calls tryResponderSAInit (register.go), the first site that reads the flags octet of a fresh IKE_SA_INIT; no established-SA or initiator-side message carries X or V bits | Drive the noisy request through tryResponderSAInit, and add an established-SA message and an initiator-side response with the bits set |
| RFC7296-2.21.4-6 | The input is a response with no pending exchange, so classifyInbound sends it to inboundInvalid, where an authenticated copy also changes no state; the negative flips the R bit as well as the protection, so it cannot attribute the freeze to protection | Give the SA an outstanding request, send the INVALID_IKE_SPI response at that Message ID, and send the same response inside SK as the negative |

Second fix round (2026-10-11, owner approval "Approve the fixes (Recommended)"). TestXBitsIgnoredOnReceipt now drives the fresh request through tryResponderSAInit, an X/V-bit protected INFORMATIONAL through handleOwnedInbound, and an X/V-bit IKE_SA_INIT response through handleInbound. TestErrUnprotectedNotifyChangesNoState now holds a DPD probe outstanding and sends the unprotected and the SK-protected INVALID_IKE_SPI response at the probe's Message ID. Fresh mutant-route records break the flags check at register.go tryResponderSAInit (3.1-8 both polarities, 3.1-11 negative) and the decrypt condition in the inboundInvalid arm of handleOwnedInbound (2.21.4-6 both polarities). Verdicts owed to an independent judge (the author judged none):

| Stem | Ids | Mode |
|------|-----|------|
| rfc7296 | 3.1-8, 3.1-11, 2.21.4-6 (STALE) | rejudge: DONE 2026-10-11 by an independent judge, outcomes below |
| rfc7296 | 2.21.2-1, 2.21.2-2, 2.21.4-7, 3.10.1-1, 3.10.1-3, 3.1-3, 3.1-4, 3.1-7, 3.1-9, 3.1-12 (SHIFTED only) | `./le rfc reseal` |

Rejudge of the second fix round (2026-10-11, independent judge, stamped `mode rejudge` into `rfc/audit/rfc7296.json`):

| Id | Verdict | Reason | Fix owed |
|----|---------|--------|----------|
| RFC7296-3.1-8 | enforced | the X-bit request, established-SA request and initiator response reach the three dispatch entries with exact outcomes; the R-bit control and the tryResponderSAInit mutant records discriminate | none; four receive readers (AUTH and EAP response arms, established-SA response arm, sendInvalidIKESPI guard) stay undriven, all mask R alone |
| RFC7296-3.1-11 | weak | receive clause now sound; the send claim says "every message it sends" while the sample misses two of five header writers, sendSAInitNotifyRaw (COOKIE and IKE_SA_INIT error notifies) and sendInvalidIKESPI | sample those two emissions in TestBuiltMessagesClearXAndVBits, or scope the claim to the sample |
| RFC7296-2.21.4-6 | weak | the established route is now isolated and discriminating, but an unprotected IKE_AUTH response kills a half-open initiator SA: handleAuthResponse (fsm.go) sets StateDead when the SK payload is missing, as handleEAPResponse does; the VALIDATES line claims every SA | code defect, journal row in `plan/journal/guard-added-to-one-half-of-a-pair.md`; fix the handshake arms to drop silently, then add a StateAuthSent case to the test |

Mechanical: `./le rfc reseal` for any rfc4555 verdict whose `mobike_test.go` unit only shifted.

Third fix round (2026-10-11, judge outcome of 8b73eae876):

| Id | Done | Evidence | Still owed |
|----|------|----------|------------|
| RFC7296-2.21.4-6 | fixed in 87b04ed6cd: handleAuthResponse and handleEAPResponse drop an IKE_AUTH response with no SK payload instead of setting StateDead | new TestErrUnprotectedNotifyLeavesHalfOpenSA (tag 2.21.4-6 positive) feeds sendInvalidIKESPI bytes through routeInbound to StateAuthSent and StateEAPInProgress: red before the fix (state = dead in both subtests), green after; mutant record restoring StateDead in the handleAuthResponse drop | rejudge (STALE); RFC3748-4-5 rejudge too (STALE, it judged handleEAPResponse) |
| RFC7296-3.1-11 | NOT done: the fix edits engineBuiltMessages in rfc7296_header_test.go, which feeds the tagged tests of 3.1-5, 3.1-6, 3.1-7 and 3.1-11, so it needs fresh owner words | the five header-flags writers are writeAuthHeaderWithMsgID (auth.go), encodeSAInitRequest (initiator.go), buildSAInitResponse and sendSAInitNotifyRaw (responder.go), sendInvalidIKESPI (notify_error.go) | owner approval, then sample sendSAInitNotifyRaw and sendInvalidIKESPI |

Six revert records whose producer is handleAuthResponse were re-recorded after the fix: RFC3948-2.1-1, RFC4303-2.1-1 and RFC7296-2.21.2-2, both polarities each.

RFC7296-3.1-11 send claim, closed without touching the tagged sample (2026-10-11): a new file `internal/component/ike/engine/rfc7296_header_notify_writers_test.go` carries two units tagged `RFC7296-3.1-11 positive`. TestSAInitNotifiesClearXAndVBits drives sendSAInitNotifyRaw through sendCookieChallenge (COOKIE) and sendSAInitNotify (NO_PROPOSAL_CHOSEN, INVALID_KE_PAYLOAD); TestInvalidIKESPIClearsXAndVBits drives sendInvalidIKESPI through answerOutOfSA, once with a request carrying the V bit and every X bit. Each parses the datagram the peer read and asserts V and X clear. Mutant records: `wire.FlagResponse` -> `wire.FlagResponse | wire.FlagVersion` in the header literal of sendSAInitNotifyRaw (`responder.go`) and of sendInvalidIKESPI (`notify_error.go`), each observed red. `rfc7296_header_test.go` and engineBuiltMessages are unchanged. Verdict owed: RFC7296-3.1-11 rejudge (STALE), by an independent judge.

Rejudge of the third fix round (2026-10-11, independent judge, stamped `mode rejudge` into `rfc/audit/rfc7296.json` and `rfc/audit/rfc3748.json`; `./le rfc check` shows no violation in an IKE or EAP stem after it):

| Id | Verdict | Reason | Fix owed |
|----|---------|--------|----------|
| RFC7296-2.21.4-6 | enforced | the half-open defect is fixed at the producer (handleAuthResponse and handleEAPResponse return on no SK before touching the SA); TestErrUnprotectedNotifyLeavesHalfOpenSA drives sendInvalidIKESPI bytes through routeInbound to StateAuthSent and StateEAPInProgress with a mutant record on the AUTH arm; the established route and its protected negative stand; the responder drops any R-bit message before its state switch (pinned by TestNrsResponseNeverDrawsAResponse under 3.1-12) and StateSAInitSent falls under Section 2.21.1 | none for this row. Residual outside it: an SK payload that fails decryption still sets StateDead on a half-open initiator SA (forged SK, not an unprotected Notify) |
| RFC3748-4-5 | enforced | the only change to handleEAPResponse is the no-SK branch, which runs before decryption and Code admission; the three tagged units are byte-identical and the discard route (Process admission on both roles, the result.Discarded return) is unchanged | none |
| RFC7296-3.1-11 | enforced | all five header-flags writers are now sampled for the send clause (three by TestBuiltMessagesClearXAndVBits, sendSAInitNotifyRaw and sendInvalidIKESPI by the two new units, the latter against a V and X marked request), each with a mutant record; the receive clause stays covered by TestXBitsIgnoredOnReceipt | none |

Decision A is BLOCKED, not done: enrolling RFC 9190 gates its 33 MUST-level rows that carry no tag or one polarity, and the summary's Meta records the 2026-08-01 owner ruling that closed the annotation route. The handoff in the session state file names the choice.

### RFC 9190 enrolment: write the tests, then enrol (owner decision C, 2026-10-11)

Thomas, 2026-10-11: "Write the tests, then enrol (Recommended)". Before RFC 9190 is enrolled, every gated MUST row in `rfc/short/rfc9190.md` gets real tests in both polarities through the real entry point, each proven by `./le rfc record`. An annotation is used only where it is TRUE and the gate doc allows it: `{gap}` only where Ze genuinely does not implement the obligation, `{single-polarity}` only where no negative can exist. The 2026-08-01 owner ruling recorded in the summary's Meta closed annotations as a way to skip tests, and it stands. A row Ze does not implement is recorded as a gap and listed for the owner's scope decision, never built. Enrolment flips to `enrolled` in a final commit, and only once `./le rfc check` would show zero rfc9190 violations.

Progress by row (the gated MUST rows that lacked a test in one or both polarities on 2026-10-11: 32, plus the `{gap}` row 2.3-4):

| Row | State | Units |
|-----|-------|-------|
| 2.1.1-1 | both polarities, recorded | `TestEAPTLS13AuthenticatorAuthenticatesWithItsCertificate`, `TestEAPTLSAuthenticatorWithoutACertificateIsRefused` (`internal/core/eap/rfc9190_handshake_rules_test.go`) |
| 2.1.2-2 | both, recorded | negative added on `TestEAPTLS13RefusesATicketPastTheSection57Lifetime` |
| 2.1.3-2 | both, recorded | positive added on `TestEAPTLS13PeerClientHelloOffersNoEarlyDataOrPostHandshakeAuth` (wire psk_key_exchange_modes is psk_dhe_ke alone), negative on `TestEAPTLS13IssuesNoTicketToAClientThatOffersNoPSKMode` |
| 2.1-8 | both, recorded | tags added on the two record-version units of the next row |
| 2.1.4-1, 2.1.4-2 | both, recorded | `TestEAPTLS13EachEndAnswersWithRecordsOfTheNegotiatedVersion`, `TestEAPTLS12EachEndAnswersWithRecordsOfTheNegotiatedVersion` |
| 2.1.4-3 | both, recorded | `TestEAPTLS13FatalErrorSendsATLSAlert`, `TestEAPTLS13CompletedExchangeCarriesNoAlert` |
| 2.3-2, 2.3-3 | both, recorded | `TestRFC9190KeyMaterialIsOneFullLengthExport`, `TestRFC9190KeysAreNotAShorterExport` |
| 5.7-4 | both, recorded | positive on `TestEAPTLS13TicketIsNotRedeemableUnderAnotherPeeringsKey`, negative on `TestEAPTLS13CompletesAResumptionWithAnUnrevokedChain` |
| 5.6-1 | both, recorded | `TestEAPTLSAuthenticatorGrantsNoAccessToAPeerWithoutACertificate`, `TestEAPTLS13AuthenticatorGrantsAccessToTheCertificateItVerified` (`internal/core/eap/rfc9190_authorization_privacy_test.go`) |
| 5.6-2 | both, recorded | positive `TestEAPTLS13AuthenticatorGrantsAccessToTheCertificateItVerified`, negative added on `TestEAPTLS13ResumptionRefusesARevokedClientChain` |
| 5.8-1, 5.8-2, 5.8-3 | both, recorded | `TestEAPTLS13PeerIdentityCarriesNoPartOfTheUser`, `TestEAPTLS13PeerIdentityKeepsTheRealmApart`. The three sentences are conditional ("If anonymous NAIs are not used"); the tests show ze always sends an anonymous NAI, which the judge should weigh |
| 2.4-1 | both, recorded | `TestEAPTLS13BothRolesMeetTheRFC8446MandatoryToImplementSet`, `TestEAPTLS13AuthenticatorAbortsAClientHelloWithoutSignatureAlgorithms` (`internal/core/eap/rfc9190_cipher_suites_test.go`) |
| 2.4-2 | both, recorded | `TestEAPTLS13PeerOffersOnlySuitesWithConfidentiality`, `TestEAPTLSAuthenticatorRefusesSuitesWithoutConfidentiality` (replays ze's ClientHello offering only TLS_SHA256_SHA256 or TLS_RSA_WITH_NULL_SHA256) |
| 2.1-2 | both, recorded | positive as before; negative `TestEAPTLS13AuthenticatorRefusesAClientHelloOfferingEarlyData` (`internal/core/eap/rfc9190_tls_layer_test.go`) |
| 2.1-3, 2.1-5, 2.1-7, 2.1.2-3 | positive recorded, `{single-polarity: positive}` naming Go crypto/tls | `TestEAPTLS13AuthenticatorConfigSendsNoEarlyDataOrPostHandshakeRequest` asserts the tls.Config `newTLSMethod` builds and decrypts the authenticator's records with the test client's key log |
| 2.1-4, 2.1-6 | positive recorded, `{single-polarity: positive}` naming Go crypto/tls | `TestEAPTLS13PeerClientHelloOffersNoEarlyDataOrPostHandshakeAuth` |
| 2.1.2-4 | positive recorded, `{single-polarity: positive}` naming Go crypto/tls | `TestEAPTLS13PeerOffersNoEarlyDataFromATicketThatPermitsIt` (also asserts `tlsClientConfig` offers no ALPN, the config fact crypto/tls's 0-RTT offer turns on) |
| 2.1.1-2 | both, recorded | `TestEAPTLS13PSKIsUsedToResumeATicketTheAuthenticatorIssued`, `TestEAPTLS13PSKTheAuthenticatorDidNotIssueIsNotUsed` (the ServerHello pre_shared_key read off a replay) |
| 2.1.8-4 | both, recorded | negative `TestEAPTLS13PeerHoldingACertificateSendsIt` |
| 2.1.8-1 | both, recorded | negative `TestEAPTLS13AuthenticatorRefusesAnAnonymousPeerWithARevokedCertificate` (`internal/core/eap/rfc9190_cached_authorization_test.go`), break `checkChainRevocation` |
| 5.7-3 | both, recorded | negative `TestEAPTLS13ResumptionAuthorizesOnTheCachedChain` |
| 5.6-1, 5.6-2 | `{feature-declined}`, tags removed, records deleted | conditional on "When peer authentication is not used"; ze always requires and verifies a client certificate (`newTLSMethod`). The tests stay as evidence, untagged |
| 5.8-1, 5.8-2, 5.8-3 | `{feature-declined}`, tags removed, records deleted | conditional on "If anonymous NAIs are not used"; ze always sends an anonymous NAI (`anonymousNAI`). The tests stay as evidence, untagged |

Thomas, 2026-10-11, "Test Ze's TLS config + name Go TLS (Recommended)": for rows Go crypto/tls enforces with no Ze setting, the tests assert the configuration Ze hands to crypto/tls and each row names Go crypto/tls as the layer that performs it. The gate doc refuses `{lower-layer}` beside a tagged test (`check_core.go`, "the annotation is stale"), so these rows name Go crypto/tls in a `{single-polarity: positive; ...}` annotation, used only where no input to Ze can produce the violation a negative test would need. The positive is tagged and recorded. Discrimination for these rows is a revert of the producer that builds the config: the mutant route needs a gomu report (`./le rfc discriminate ... report <path>`), which no run in this session produced.

The rows still open, and what each needs, are in the per-spec session state handoff of 2026-10-11; rows move into the table above as they close.

### Deferred items the parent's tables missed

| Item | What is owed | Group |
|------|--------------|-------|
| R-7 detail lost in condensing | RFC1035-4.1.4-1: the negative's comment claims "two zero bits" but asserts only "no 0xC0 octet". RFC9190-5.4-3: the claim that crypto/tls turns the error into bad_certificate is not asserted. RFC9190-5.7-1: `TestEAPTLS13CompletesAResumptionWithAnUnrevokedChain` proves the valid-cache positive but carries only the 5.7-2 and 5.7-6 tags. DRAFT-8210BIS-7-2: the guard is `hdr.Type != pduErrorRpt` in `RTRSession.readLoop`. DRAFT-8210BIS-5.12-1: the Error Code 9 mapping is in `readLoop` | services, IKE/EAP, BGP |

- The row starting `R-7 detail lost in condensing`: this child owns RFC9190-5.4-3 and RFC9190-5.7-1 only; the other ids go to the child that owns their stem.

### Narrowing audit rows (parent, 2026-09-28)

Copied from the parent's "Narrowing audit findings, 2026-09-28", where the process and the call meanings are. AC-C3 includes these rows.

| ID | Call | Obligation | Section |
|----|------|------------|---------|
| RFC3748-2.3-1 | dropped | "Unless the authenticator implements one or more authentication methods locally which support the authenticator role, the EAP method layer header fields (Type, Type-Data) are not examined as part of the forwarding decision." | §2.3 |
| RFC3948-4-1 | retire | No RFC 3948 sentence says interval MUST be shorter than NAT binding timeout; quote is the SHOULD-send-after-M-seconds obligation. | none |
| RFC4301-4.4.1-6 | dropped | "- SPD-I: For inbound traffic that is to be bypassed or discarded ... - SPD-O: For outbound traffic ... - SPD-S: For traffic that is to be protected using IPsec" | §4.4.1 |
| RFC4303-3.4.4.1-1 | dropped | "If the default padding scheme (see Section 2.4) has been employed, the receiver SHOULD inspect the Padding field before removing the padding prior to passing the decrypted data to the next layer." | §3.4.4.1 |
| RFC4555-3.9-1 | dropped | "The notification data contains the IP addresses and ports from/to which the packet was sent." | §4.2.6 |

### Blocked by

Each id below was checked on 2026-09-28: a weak or wrong verdict in `rfc/audit/<stem>.json`, or a row of the R-7 table above. The blocking spec owns the producer fix (AC-C4), and the verdict moves into its acceptance criteria (P-3).

| ID | Verdict | Blocking spec |
|----|---------|---------------|
| RFC3948-2.1-2 | wrong | `spec-ike-eap-rfc-defects` |
| RFC5216-2.1.1-4 | wrong | `spec-ike-eap-rfc-defects` |
| RFC5216-2.1.2-1 | weak | `spec-ike-eap-rfc-defects` (AC-4, 2026-09-29) |
| RFC5216-2.1.1-5 | weak | `spec-ike-eap-rfc-defects` (AC-4 names it for re-proof: the fix removes the TLS 1.2 resumed conversation its test drives; 2026-09-30) |
| RFC4301-4.4.2.1-3 | wrong | `spec-ipsec-lifetime-volume` (AC-7, 2026-09-30): Ze keeps no byte-count lifetime, so there are no counters to fall out of synch until that spec lands |
| RFC9190-1-1 | weak (R-7) | `spec-ipsec-rfc9190` |
| RFC9190-2.1.8-2 | weak (R-7) | `spec-ipsec-rfc9190` |
| RFC9190-2.1.8-3 | weak (R-7) | `spec-ipsec-rfc9190` |
| RFC9190-2.1.8-4 | weak (R-7) | `spec-ipsec-rfc9190` |
| RFC9190-2.1.8-5 | wrong (R-7) | `spec-ipsec-rfc9190` |
| RFC9190-5.4-1 | weak (R-7) | `spec-ipsec-rfc9190` |
| RFC9190-5.4-2 | weak (R-7) | `spec-ipsec-rfc9190` |
| RFC9190-5.4-3 | weak (R-7) | `spec-ipsec-rfc9190` |
| RFC9190-5.7-5 | weak (R-7) | `spec-ipsec-rfc9190` |

- RFC7296-2.4-2 (weak, no tagged test) stays in the parent and is named in the acceptance criteria of `plan/release/fixes/spec-ike-dpd-demand-driven.md` (parent AC-10); this child does not re-judge it.
- RFC4303-2.1-1 tags units in IKE and OSPF files and stays in the parent (AC-9).
- The rfc9190 R-7 rows this child resolves: RFC9190-2.1.2-2, 2.1.3-1, 2.3-1 and 5.7-1.

## Required Reading

### Architecture Docs
- [ ] `plan/release/validation/spec-rfc-verdict-test-fix-pass.md` - the Method table, the Required Reading constraints, D-2 to D-15, and the source tables above
  → Constraint: every row stays a verbatim span of its cited section, at least 24 characters, never across two sections (`checkRowQuotes`); a span that must cross a section is split into two rows
  → Constraint: widening, narrowing or re-citing a row stales its verdict (`stale-requirement`) until it is re-judged in the same commit; a new id takes n above the HEAD^ high-water mark of its section (`checkIDAllocation`)
  → Constraint: a new gated row lands with both polarity tags or an annotation, a discrimination record for each new cover (a moved tag is a new cover), and an extraction site mapped to it or listed in `unsourced-ids`
  → Constraint: an upgrade from weak or wrong to `enforced` with unchanged units needs an `upgrade_reason` (`checkAuditFindings`), so author and judge land in one commit where they can
  → Constraint: editing any function in a test file shifts every verdict tagging that file; `./le rfc reseal` is corpus-wide and rewrites other children's audit files, so a reseal runs only when no other child holds uncommitted audit changes
  → Constraint: rfc4301 and rfc4303 are this child's ledger files, including the verdicts whose units sit in the two OSPF IPsec test files; those files are edited only while OSPF holds no uncommitted change in them
  → Constraint: rfc9190 is not enrolled, so `audit-stamp` refuses it: its R-7 re-judgements are recorded in this spec's R-7 table with the date
  → Constraint: RFC9190-2.3-1: the Method-Id gap is SPLIT into a `{gap}` row, verdict `unimplemented`, and the old row keeps its tests (`checkCoverageRatchet`); implementing Method-Id needs the owner's scope
  → Constraint: the interop `revert` route (strongSwan) writes the break into the shared working tree, so it never runs while another child builds
  → Constraint: the "level with no BCP 14 keyword" rows are reviewed under D-3, not demoted by rule; a demotion needs a correction paragraph per stem and an owner call on the format-definition rows
- [ ] `docs/contributing/rfc-conformance-gates.md` - the discrimination record, the row quote, the refusals
- [ ] `ai/skills/ze-rfc-audit.md` - the four judgement questions, the verdict vocabulary, STRICTNESS, and the re-judge route
  → Decision: judge strictly: a floor assertion, a confounded buffer or a positive that proves only "no error" stays `weak`

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/<stem>.md` for each owned stem above - the row text a verdict is judged against; the RFC text in `rfc/full/<stem>.txt` is the authority for every split and correction
  → Constraint: a claim that an obligation was dropped quotes the RFC sentence with its section (parent R-9)

**Key insights:**
- 122 verdicts over 9 packages and 11 stems, plus 25 rows copied from the parent's tables
- The listing is derived with the filter above, never copied; each stem's ledger files belong to one child
- A verdict whose producer another spec fixes is listed under "Blocked by" and leaves this child through P-3

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `plan/release/validation/spec-rfc-verdict-test-fix-pass.md` - the parent: Split section, inventory tables, Method, Required Reading, AC-C1 to AC-C7
- [ ] `rfc/audit/rfc7296.json` - weak and wrong verdicts with their `tests` maps and notes
- [ ] `rfc/audit/rfc3748.json` - weak and wrong verdicts with their `tests` maps and notes
- [ ] `rfc/audit/rfc5216.json` - weak and wrong verdicts with their `tests` maps and notes
- [ ] `internal/le/rfc/check_audit.go` - `checkAuditFindings` refuses a deleted weak or wrong verdict and an unchanged-units upgrade without `upgrade_reason`

**Behavior to preserve:**
- a commit of this child adds no `./le rfc check` violation of its own; a weak or wrong verdict is never deleted
- product behavior of every package in scope, except a D-8 fix of a verified defect, which carries its own failing-first test

**Behavior to change:**
- the tagged tests, rows, tags and verdicts named in this spec's tables and in the derived listing; D-8 producer fixes where a test exposes a verified defect

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le rfc check`, reading `rfc/short/`, `rfc/audit/`, `rfc/discrimination/` and the `RFC requirement:` tags in test files
- `./le rfc audit-stamp stem <stem> from <pending> mode rejudge`, typed by a judging agent

### Transformation Path
1. The author agent records `./le rfc approve unit` (P-1), then changes a tagged test, a row, a tag, or a producer (D-8)
2. `./le rfc reseal` re-stamps sibling verdicts the edit only shifted
3. `./le rfc record` records the observed red under a break for each added or changed cover
4. A judging agent that did not author the test writes the verdict in a pending file under session scratch
5. `./le rfc audit-stamp ... mode rejudge` replaces the verdict in place with fresh fingerprints
6. `./le rfc check` compares verdicts, records and rows against HEAD^

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| test file to ledger | the `RFC requirement:` tag, fingerprinted by `stampFingerprints` | `./le rfc check` reads the re-judged verdict fresh |
| pending file to audit file | `audit-stamp` `mode rejudge` | the parent's `TestAuditStampRejudgeReplacesInPlaceAndReadsFresh` |

### Integration Points
- the parent's `mode rejudge` of `./le rfc audit-stamp` (P-2)
- the blocking specs' acceptance criteria, which receive the blocked verdicts (P-3)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | every verdict enters through `audit-stamp`, every record through `discriminate-record` |
| No duplicated functionality | Yes | the listing is the parent's filter restricted to this scope; no id list is committed |
| Registration over hardcoding | N-A | no feature is added; this child changes tests, rows and D-8 producers |

## Method

The parent's Method table governs every phase and is not copied here. The rules that bite this group are the constraints under Required Reading above, plus: one agent per package and never two on one test file; strict judgement; every clause in both polarities; a discrimination record for every tag added or changed; an independent judge for every re-judged verdict; a D-8 defect fixed failing-test-first, with interop where a peer exists.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | the parent's `mode rejudge` has landed before the first re-judge here | parent Implementation Steps 1 and 2 | a re-judge has no route but hand deletion, which P-2 bans | `./le rfc audit-stamp` help names `mode` | confirmed 2026-10-11: the help prints `[mode <new\|rejudge>]` |
| A-2 | the blocked-by list is complete for this scope | the parent's code-defect and overlap tables, each id checked in `rfc/audit/` or the R-7 table on 2026-09-28 | the child cannot reach AC-C1 | each package's author agent names any verdict it cannot move without a producer fix another spec owns | confirmed 2026-10-11: the derived listing at HEAD returns exactly the five blocked ids, each in its blocking spec's acceptance criteria (review N4) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | a test fix exposes a defect that grows past the package | a code change outside the package | D-8: fix it; where impossible, back to the owner |
| R-2 | another child or spec commits while this child holds uncommitted hunks in a shared test file, or runs a reseal | a commit diff shows another owner's ids | the shared-file rule above; reseal and interop `revert` runs are serialized between children |
| R-C1 | the RFC7296 fragment and list-pointer widenings stale many verdicts at once (`stale-requirement`) | a widened row reads stale in `./le rfc check` | each widening and its re-judge land in one commit |
| R-C2 | `spec-ipsec-rfc9190` (in progress) edits the same `internal/core/eap` test files | a commit carries its hunks | edit an rfc9190 test file only while that spec holds no uncommitted change in it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | test-only changes break nothing an operator sees; a D-8 fix changes protocol behavior and carries its own tests and interop |
| How is it reverted? | one commit per package phase |
| Who else touches this path? | the parent, the children named in the shared-file table, the blocking specs, `spec-rfc-evidence-strength-1` and `-2` (the same discrimination files) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` | → | `checkAuditFindings` over this child's stems | the derived listing above holds only the "Blocked by" ids, and `TestCheckAuditRatchetSeesTipCommit` (`internal/le/rfc/check_audit_baseline_test.go`) stays green |

## Acceptance Criteria

Inherited from the parent (AC-C1 to AC-C7), restated for the IKE/EAP group.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-C1 | the derived listing above | holds only the "Blocked by" ids, each named in its blocking spec's acceptance criteria |
| AC-C2 | every verdict this child moves to `enforced` | its tagged units carry a discrimination record, and an agent other than the test's author judged it |
| AC-C3 | every row of this child in the split-needed and missing-rows tables, including the rows the parent's narrowing audit adds for this group (the "Narrowing audit rows (parent, 2026-09-28)" table) | the dropped obligation is a row of its own, or a dated correction says why not, and the new row carries a verdict |
| AC-C4 | every spec in the parent's code-defect list | this child declares none of their defects and does not fix them; a test whose producer one of them changes waits for, or lands with, that spec |
| AC-C5 | `./le rfc check` after each commit of this child | no violation the commit added, and no stale verdict in this child's stems |
| AC-C6 | every row and unit of this child in the row-quality and mistagged-unit tables | corrected, merged or re-tagged, or a dated correction says why it stands; a changed row or tag is re-judged |
| AC-C7 | every row of this child in "Un-enrolled R-7 rows" that no spec blocks | resolved as the parent's Goal states and re-judged `enforced` by an agent that did not write the test, recorded in that table with the date or in the stem's audit file; RFC9190-2.3-1's Method-Id gap is split into a `{gap}` row under `ai/rules/rfc-compliance.md` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| every unit tagged by a listed verdict | `internal/core/eap/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/ike/engine/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/ike/dataplane/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/ike/crypto/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/ike/wire/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/ike/transport/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/ike/ipsec/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/plugins/ospf/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `test/parse/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| each numeric field a quoted sentence bounds | the RFC's range | the RFC's last valid value | tested where the sentence states a lower bound | tested where the sentence states an upper bound |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| an existing tagged `.ci` test in scope, changed only where its verdict demands | `test/` | the operator-visible behavior the quoted sentence names | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `eap-tls`, `eap-tls13`, `eap-mschapv2`, `responder-eap-tls13`, `responder-eap-tls13-revoked-client`, `responder-eap-mschapv2` | `test/interop-ipsec/scenarios/` | strongSwan | EAP fixes `48591f88d5`, `b57b47161d` accepted by a conforming peer (non-regression) | pass, 2026-10-11 |
| `child-rekey`, `responder-raises-child-rekey`, `responder-ike-rekey`, `ike-rekey-retries-in-the-named-group` | `test/interop-ipsec/scenarios/` | strongSwan | rekey nonce fix `51bddc694e` (non-regression) | pass, 2026-10-11 |
| `dataplane-readback` | `test/interop-ipsec/scenarios/` | strongSwan | SAD selector and lifetime fixes `320f4582c8`, `c548bebf6f`, `d24fc20179` (non-regression) | pass, 2026-10-11, after harness fix `cc27032d56` |
| `natt-transport-inner-checksum`, `natt-tunnel-inner-checksum`, `real-nat-transport-ze-initiator`, `real-nat-transport-ze-responder`, `real-nat-tunnel-control` | `test/interop-ipsec/scenarios/` | strongSwan | NAT-T and RFC 3948 fixes `3b83ac7de1`, `e585f7d591` (non-regression) | pass, 2026-10-11 |
| `psk-site-to-site`, `responder-psk` | `test/interop-ipsec/scenarios/` | strongSwan | SPI and transform fixes `cb4801b4cc`, `2fed107f98`, `db64c5ca7b`, `c648880564`, `8f1ef0ade3` (non-regression) | pass, 2026-10-11 |

Logs: `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/interop-ipsec/<name>.log`. No scenario was run red with its fix reverted, so these prove acceptance and non-regression; the discrimination of each fix is carried by its unit records.

## Files to Modify

- the `_test.go` and `.ci` files of the packages above that carry the listed tags
- `rfc/short/<stem>.md`, `rfc/audit/<stem>.json`, `rfc/discrimination/<stem>.json`, `rfc/corrections/<stem>.md`, `rfc/extraction/<stem>.json` for the owned stems only
- a producer under the packages above when a test exposes a verified defect (D-8), named in the phase that finds it
- `docs/features/rfc-status.md`, the row of each owned stem whose support claim changes
- the acceptance criteria of each blocking spec, which receive its blocked verdicts (P-3)

## Files to Create
- none planned; a split or missing row is a new row in an existing `rfc/short/<stem>.md`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| YANG validation constraints | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| YANG custom validators | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| CLI commands/flags | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| CLI grammar (keyword before value) | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| Editor autocomplete | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| Functional test for new RPC/API | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| Pipe completeness | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| Env var registration | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| Doctor check for runtime dependencies | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| Prometheus counters/metrics | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |
| BGP family surface (new SAFI / capability / attribute) | N-A | tests, rows and verdicts only; a D-8 fix that needs this row answers it in the phase that finds the defect |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | this child changes tests, rows and verdicts, not this surface |
| 2 | Config syntax changed? | No | this child changes tests, rows and verdicts, not this surface |
| 3 | CLI command added/changed? | No | this child changes tests, rows and verdicts, not this surface |
| 4 | API/RPC added/changed? | No | this child changes tests, rows and verdicts, not this surface |
| 5 | Plugin added/changed? | No | this child changes tests, rows and verdicts, not this surface |
| 6 | Has a user guide page? | No | this child changes tests, rows and verdicts, not this surface |
| 7 | Wire format changed? | No | tests and rows only; a D-8 fix that changes the wire names `docs/architecture/wire/*.md` in its phase |
| 8 | Plugin SDK/protocol changed? | No | this child changes tests, rows and verdicts, not this surface |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/<stem>.md` for each owned stem a phase changes, and its `docs/features/rfc-status.md` row, with source anchors; a new `wrong` verdict discloses non-support there first |
| 10 | Test infrastructure changed? | No | this child changes tests, rows and verdicts, not this surface |
| 11 | Affects daemon comparison? | No | this child changes tests, rows and verdicts, not this surface |
| 12 | Internal architecture changed? | No | this child changes tests, rows and verdicts, not this surface |
| 13 | Route metadata keys added/changed? | No | this child changes tests, rows and verdicts, not this surface |
| 14 | Prometheus counters added/changed? | No | this child changes tests, rows and verdicts, not this surface |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | this child changes tests, rows and verdicts, not this surface |
| 16 | Any changed source file referenced by existing doc source anchors? | No | only test and ledger files change; a D-8 producer fix runs `./le spec citation anchors` in its phase |
| 17 | Existing docs show config/CLI/API examples for this area? | No | this child changes tests, rows and verdicts, not this surface |

## Implementation Steps

**Procedure P, for every package phase.** One author agent per package, never two on one test file. It reads the listing restricted to the package, records `./le rfc approve unit <pkg>.<Test> reason "D-15: ..."` before each tagged-unit edit (P-1), makes each clause of the quoted sentence asserted in both polarities or corrects the row, and fixes a verified defect under D-8. It runs the scoped package test under `./le job run`, then `./le rfc record` for each added or changed cover. An independent judge that wrote none of the tests writes the verdicts in a pending file under session scratch and stamps them with `./le rfc audit-stamp stem <stem> from <pending> mode rejudge`. `./le rfc reseal` runs when the edit shifted sibling verdicts, serialized between children. Each package lands in one commit through `./le commit create` with the `RFC-approved:` trailer, author and judge together where they can. Verify: the listing filtered to the package holds only blocked ids, and `./le rfc check` adds no violation.

1. **Phase: Wiring** - confirm the parent's `mode rejudge` has landed (A-1), reproduce the derived listing and its counts, and move each "Blocked by" verdict into its blocking spec's acceptance criteria (P-3)
   - Tests: the Wiring Test row
   - Files: the blocking specs' Acceptance Criteria tables
   - Verify: the listing count equals the Task figure, or the difference is explained by commits since 2026-09-28
2. **Phase: `internal/core/eap`** - 49 verdicts, 1 wrong; procedure P
3. **Phase: `internal/component/ike/engine`** - 37 verdicts, 4 wrong; procedure P
4. **Phase: `internal/component/ike/dataplane`** - 11 verdicts, 3 wrong; procedure P
5. **Phase: `internal/component/ike/crypto`** - 8 verdicts, 1 wrong; procedure P
6. **Phase: `internal/component/ike/wire`** - 8 verdicts, 0 wrong; procedure P
7. **Phase: `internal/component/ike/transport`** - 5 verdicts, 0 wrong; procedure P
8. **Phase: `internal/component/ike/ipsec`** - 4 verdicts, 1 wrong; procedure P
9. **Phase: `internal/plugins/ospf`** - 4 verdicts, 1 wrong; procedure P
10. **Phase: `test/parse`** - 1 verdicts, 0 wrong; procedure P
11. **Phase: split and missing rows** - after the parent commits its narrowing output for this group (parent R-11): each row of the split-needed and missing-rows tables becomes a row of its own with both polarity tags, a record and a verdict, or a dated correction says why not (AC-C3)
   - Verify: `./le rfc check` accepts the new ids, extraction sites and records
12. **Phase: row quality and mistagged units** - each row of those tables corrected, merged or re-tagged, or a dated correction says why it stands; each changed row or tag re-judged (AC-C6)
   - Verify: `./le rfc check` reads no stale verdict in the owned stems
13. **Phase: un-enrolled R-7 rows** - each row not under "Blocked by" resolved and re-judged by an agent that did not write the test, recorded in the R-7 table above with the date (AC-C7)
14. **Phase: closure check** - the derived listing holds only "Blocked by" ids (AC-C1), each named in its blocking spec's acceptance criteria

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-C row demonstrated; the listing holds only blocked ids |
| Correctness | every `enforced` verdict names an identifier in its unit, has both polarities or `{single-polarity}`, and a judge other than the author |
| Rule: no weakened test | no assertion removed or loosened to reach `enforced` (`ai/rules/completion.md`) |
| Rule: one writer per ledger file | no commit touches a stem another child owns |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| the listing reduced to blocked ids | the filter under "Derived listing" |
| a record for every changed cover | `./le rfc check` adds no missing-record violation |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | a negative test that feeds malformed input asserts the refusal, not only the absence of a crash |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | re-read the RFC sentence; a verified defect is a D-8 fix in this child unless a blocking spec owns it |
| A verdict cannot move without another spec's producer fix | add it to "Blocked by" and to that spec's acceptance criteria (P-3) |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the owner |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Scope by package, with each stem's ledger at one owner | scope by stem | the parent's split: test files are per package, ledger files per stem |
| A verdict of an owned stem whose units sit only in another child's package belongs to the stem owner | give it to the package owner | the ledger file has one writer; the shared test file is edited one child at a time |

## Known Limitations

- verdicts under "Blocked by" leave this child through P-3 and close with their blocking spec
- the parent's cross-group verdicts and RFC7296-2.4-2 are not in this child's scope

## RFC Documentation (Scope: protocol)

A D-8 fix adds `// RFC NNNN Section X.Y: "<quoted requirement>"` above the enforcing code, per `ai/rules/rfc-compliance.md`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-C1..AC-C7 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for the D-8 protocol fixes (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** tests + rows + verdicts + records + D-8 code + edited spec
- [ ] **Commit B:** `remove plan/release/validation/spec-rfc-verdict-fix-ike-eap.md` only, in the same `./le commit create` script

## Progress, 2026-10-11

Written from `plan/handover/rfc-verdict-test-fix-pass/` (QUEUE.md, HANDOFF.md) and git history, each claim re-checked against HEAD on 2026-10-11. Implementation is finished; the spec waits for its independent `/ze-review`. It is not closed.

## Implementation Summary

### What Was Implemented
- 44 commits since 2026-09-28 touch this child's eleven `rfc/audit/<stem>.json` files, including the AC-C2 record sweep `6b968f093b` (120 records), the tag fixes `1075b8992f`, the RFC9190 re-record `4074fd651a` and the Linux-host RFC4555-4.2.5-1 positive record `4baacedc8f`.
- 2026-10-11: `internal/core/eap/rfc5216_resumption_defect_test.go` (the tag-free red probe landed in `62ac09f39b`) carries `// RFC naming: untagged -- ...` naming `spec-ike-eap-rfc-defects` AC-4, which replaces it with a tagged proof of the fix. The test body is unchanged and still fails as the defect record intends (`TestRFC5216ResumingServerSendsNothingButChangeCipherSpecAndFinished`: handshake types `[2 4]`, a NewSessionTicket after server_hello).

### Bugs Found/Fixed (D-8, each with production code)
| Commit | Fix |
|--------|-----|
| `cb4801b4cc` | initiator refuses a peer's reserved ESP SPI 0 (RFC 4303 2.1) |
| `2d67f88f5f`, `51bddc694e`, `2fed107f98` | engine fixes: EAP auth, rekey nonces (2.10-3), unknown transform attribute (3.3.6-5) |
| `320f4582c8` | inbound SAD selector (RFC4301-4.4.2-1) |
| `3b83ac7de1`, `c548bebf6f`, `d24fc20179` | dataplane: RFC3948-5.2-1 transport conflict, SAD items, lifetimes |
| `db64c5ca7b` | SPI-pair lookup (RFC 7296) |
| `c648880564`, `8f1ef0ade3` | fail-closed catch-all; engine lookups return errors, no ENCR 0 offer |
| `59712974dd`, `e585f7d591` | transport: inventory TransformErr, NAT keepalive need |
| `48591f88d5`, `b57b47161d` | EAP peer certificate; EAP reassembly L-bit |

### Deviations from Plan
- None beyond the naming marker above.

## Mistake Log
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| false claim | An earlier draft of the D-8 Goal Validation row said no IKE or EAP interop scenario exists | `test/interop-ipsec/scenarios/` holds 40 strongSwan scenarios, 18 of them covering the D-8 fixes | Running the interop suite for the Goal row | Row corrected 2026-10-11, the 18 scenarios run and logged |
| over-claim | The D-8 interop row said each scenario "covers" a fix, though none was run red with the fix reverted | The runs prove acceptance and non-regression only; discrimination sits in the unit records | Review I1, 2026-10-11 | Row reworded 2026-10-11 |
| unevidenced AC | AC-C3, AC-C6 and AC-C7 were marked Done while Steps 11 to 13 had not run | Five dropped obligations, nine fragment rows, three list-pointer rows and the RFC9190 split had no outcome | Review B1 to B3, 2026-10-11 | Steps run 2026-10-11, "Row work outcomes" table |

## Implementation Audit

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-C1 | Done | the "Derived listing" filter at HEAD, 2026-10-11, returns RFC3948-2.1-2 (wrong), RFC4301-4.4.2.1-3 (wrong), RFC5216-2.1.1-4 (wrong), RFC5216-2.1.1-5 (weak), RFC5216-2.1.2-1 (weak); four are in AC rows of `spec-ike-eap-rfc-defects`, RFC4301-4.4.2.1-3 in AC-7 of `plan/release/fixes/spec-ipsec-lifetime-volume.md` | |
| AC-C2 | Done | 137 verdicts moved to `enforced` since `174862502f`; every tagged unit carries a record except RFC4303-2.1-1's four units, which is the parent's cross-group verdict (parent AC-9), not this child's | judge independence per commit is in QUEUE.md and is the reviewer's to confirm |
| AC-C3 | Rows done, verdicts owed | "Row work outcomes, 2026-10-11": the five dropped obligations of review B1 are rows `RFC4555-3.6-2`, `-3.6-3`, `-4.2.6-1`, `RFC4303-3.4.4.1-2` and re-quoted `RFC4301-4.4.1-6`, or a dated correction (`RFC3748-2.3-1`) | the new rows' verdicts wait for an independent judge ("Verdicts owed") |
| AC-C4 | Done | the five blocked verdicts sit in their owning specs' ACs | |
| AC-C5 | Done | `./le rfc check` 2026-10-11: the rfc5216 file-name violation is gone and no violation names an ike-eap stem | |
| AC-C6 | Rows done, verdicts owed | "Row work outcomes, 2026-10-11": every row of the row-quality table has an outcome; twelve RFC7296 rows and `RFC4301-4.4.1-6` widened or re-quoted with a dated correction each; mistagged units moved earlier (`1075b8992f` and the 2026-09-30 corrections) | thirteen STALE verdicts wait for an independent judge; four level rows wait for the owner (`RFC7296-1.4-1`, `RFC2759-x-3`, `x-10`, `x-12`) |
| AC-C7 | Rows done, verdicts owed | `RFC9190-2.3-4` `{gap}` split from 2.3-1; `TestRFC9190MSKIsTheExportUnderTheRFCLabel` recorded; pending skeleton for `rfc/audit/rfc9190.json` written | 2.1.2-2, 2.1.3-1, 2.3-1, 5.7-1 and 2.3-4 wait for an independent judge, who creates the audit file through `audit-stamp` |

## Goal Validation (BLOCKING)
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| AC-C1: the derived listing holds only "Blocked by" ids | ledger query | 5 rows, each in an owning spec's AC |
| every resolved verdict proven and judged by a non-author | discrimination records + audit | 0 unrecorded tagged units over 137 moved verdicts (RFC4303-2.1-1 is the parent's); `./le rfc check` clean for the ike-eap stems |
| D-8 fixes work against another implementation | interop | 18 of 18 strongSwan scenarios accept Ze with the D-8 fixes (acceptance and non-regression; reworded 2026-10-11 after review I1). No scenario was run red with its fix reverted, so the discrimination of each fix is carried by its unit records, not by these runs. The scenarios exercising each D-8 commit: EAP (`48591f88d5`, `b57b47161d`) `eap-tls`, `eap-tls13`, `eap-mschapv2`, `responder-eap-tls13`, `responder-eap-tls13-revoked-client`, `responder-eap-mschapv2`; rekey nonces (`51bddc694e`) `child-rekey`, `responder-raises-child-rekey`, `responder-ike-rekey`, `ike-rekey-retries-in-the-named-group`; SAD selector and lifetimes (`320f4582c8`, `c548bebf6f`, `d24fc20179`) `dataplane-readback`; NAT-T and RFC 3948 (`3b83ac7de1`, `e585f7d591`) `natt-transport-inner-checksum`, `natt-tunnel-inner-checksum`, `real-nat-transport-ze-initiator`, `real-nat-transport-ze-responder`, `real-nat-tunnel-control`; SPI and transform handling (`cb4801b4cc`, `2fed107f98`, `db64c5ca7b`, `c648880564`, `8f1ef0ade3`) `psk-site-to-site`, `responder-psk`. A conforming strongSwan never sends SPI 0 or an unknown transform attribute, so those two refusals stay proven by unit test only; the runs prove the fixes broke no conforming exchange. Second run, 2026-10-11 19:55 to 20:36, one QEMU guest at a time, over HEADs `0512529b57` to `f09ad6f51e` (the lab builds from the shared working tree, so the binary also carried other sessions' uncommitted BGP and CLI edits, none in IKE or IPsec code): the other 13 each print `── <name> ──`, `✓ PASS`, `PASS  1 scenario(s)`, `QEMU VM: PASS` in `<name>.log` under the same directory. Sixteen earlier attempts never reached strongSwan because another session's half-written edits did not compile at that moment (`<name>.build-break-<n>.log`, journaled in `plan/journal/concurrent-session-corruption.md`, 2026-10-11 row). `dataplane-readback` first failed on a harness defect, not a Ze one: its NOFILE probe read `/proc/1/task/1/children`, which exists only with `CONFIG_PROC_CHILDREN`, unset in Ze's runtime kernel that the QEMU guest boots (`FileNotFoundError`, `dataplane-readback.proc-children-1.log`). `unreadableDataplaneProbe` (`internal/le/interoplab/ipsec/helpers.go`) now finds tini's `ze` child through the parent pid in `/proc/<pid>/stat`, and the scenario passed with it. First run, 5 of 18 green on 2026-10-11 over HEAD `917da161f8`, each through `./le test qemu docker-lab lab 'test integration interop-ipsec' env 'IPSEC_INTEROP_SCENARIO=<name>'`, the darwin route (colima's daemon refused every scenario at the kernel probe: "unable to apply apparmor profile ... Load Ze's profile with: ./le setup docker kernel apparmor confirm ze-kernel-probe", logs in `colima-refused/`). Each log below prints `── <name> ──`, `✓ PASS`, `PASS  1 scenario(s)`, `QEMU VM: PASS`: `eap-tls`, `eap-mschapv2`, `child-rekey`, `responder-raises-child-rekey`, `responder-ike-rekey` (logs `<name>.log` under `tmp/session/2026-10-09-5620b26f-603e-4d57-826d-6ef92b7fcd64/scratch/interop-ipsec/`). `eap-mschapv2` passed on its second attempt; the first lost the guest SSH port race (`eap-mschapv2.port-race-flake-1.log`). No scenario failed on Ze's side in either run. The suite has 40 strongSwan scenarios under `test/interop-ipsec/scenarios/` (correction 2026-10-11: an earlier draft of this row said no IKE or EAP scenario exists, which was wrong) |

## Work Not Done
| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| RFC3948-2.1-2, RFC5216-2.1.1-4, RFC5216-2.1.1-5, RFC5216-2.1.2-1 | producer defects (P-3) | `spec-ike-eap-rfc-defects` (closed 2026-10-21, `plan/learned/032-ike-eap-rfc-defects.md`) |
| RFC4301-4.4.2.1-3 | Ze keeps no byte-count lifetime | `plan/release/fixes/spec-ipsec-lifetime-volume.md` AC-7 |
| `./le verify worktree` | needs a quiet machine | this spec, before closure |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | |
| `./le spec review check` | not run |
| Rounds | |
| Reviewer lenses used | |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
