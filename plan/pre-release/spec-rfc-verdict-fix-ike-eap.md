# Spec: rfc-verdict-fix-ike-eap

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` (the parent: its `audit-stamp` `mode rejudge` phase before any re-judge here, and its narrowing-audit output for the IKE/EAP group before any row edit, parent R-11) |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

This spec is child 5 of 8 of `plan/pre-release/spec-rfc-verdict-test-fix-pass.md`, the IKE/EAP group. The parent holds the method, the inventory and the cross-cutting work. This child carries the test fixes, row edits and D-8 code fixes for its packages and owned stems, and closes on its own.

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
| `internal/component/ike/engine/child_rekey_initiator_answer_test.go::TestChildRekeyAnswerWithoutTrafficSelectorsIsRefused`, and `rekey_test.go::TestRekeyWithoutTrafficSelectorsIsRefused` | RFC7296-2.9-1 | TS payload presence in a rekey answer, a neighbouring obligation; the verdict stays `enforced` on the other units | move the tags to the row that states the TS presence rule, or add it |
| `internal/component/ike/engine/rfc4301_spd_discard_test.go::TestSPDPolicyMirrorsTheInboundSelector` | RFC4301-4.4.1-4 | direction mirroring, not administrator ordering; the verdict stays `enforced` on the ordering units | move the tag |

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

- RFC7296-2.4-2 (weak, no tagged test) stays in the parent and is named in the acceptance criteria of `plan/immediate/spec-ike-dpd-demand-driven.md` (parent AC-10); this child does not re-judge it.
- RFC4303-2.1-1 tags units in IKE and OSPF files and stays in the parent (AC-9).
- The rfc9190 R-7 rows this child resolves: RFC9190-2.1.2-2, 2.1.3-1, 2.3-1 and 5.7-1.

## Required Reading

### Architecture Docs
- [ ] `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` - the Method table, the Required Reading constraints, D-2 to D-15, and the source tables above
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
- [ ] `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` - the parent: Split section, inventory tables, Method, Required Reading, AC-C1 to AC-C7
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
3. `./le rfc discriminate-record` records the observed red under a break for each added or changed cover
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
| A-1 | the parent's `mode rejudge` has landed before the first re-judge here | parent Implementation Steps 1 and 2 | a re-judge has no route but hand deletion, which P-2 bans | `./le rfc audit-stamp` help names `mode` | unvalidated |
| A-2 | the blocked-by list is complete for this scope | the parent's code-defect and overlap tables, each id checked in `rfc/audit/` or the R-7 table on 2026-09-28 | the child cannot reach AC-C1 | each package's author agent names any verdict it cannot move without a producer fix another spec owns | unvalidated |

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
| named when a D-8 fix is found | `test/interop/scenarios/` | IPsec interop (strongSwan, `internal/le/interoplab/ipsec`) for any D-8 fix; none is planned before a defect is found | the fixed behavior against another implementation | |

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

**Procedure P, for every package phase.** One author agent per package, never two on one test file. It reads the listing restricted to the package, records `./le rfc approve unit <pkg>.<Test> reason "D-15: ..."` before each tagged-unit edit (P-1), makes each clause of the quoted sentence asserted in both polarities or corrects the row, and fixes a verified defect under D-8. It runs the scoped package test under `./le job run`, then `./le rfc discriminate-record` for each added or changed cover. An independent judge that wrote none of the tests writes the verdicts in a pending file under session scratch and stamps them with `./le rfc audit-stamp stem <stem> from <pending> mode rejudge`. `./le rfc reseal` runs when the edit shifted sibling verdicts, serialized between children. Each package lands in one commit through `./le commit create` with the `RFC-approved:` trailer, author and judge together where they can. Verify: the listing filtered to the package holds only blocked ids, and `./le rfc check` adds no violation.

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
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** tests + rows + verdicts + records + D-8 code + edited spec
- [ ] **Commit B:** `remove plan/pre-release/spec-rfc-verdict-fix-ike-eap.md` only, in the same `./le commit create` script
