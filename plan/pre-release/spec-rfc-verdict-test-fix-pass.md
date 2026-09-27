# Spec: rfc-verdict-test-fix-pass

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | `plan/pre-release/spec-rfc-requirement-quote-hand-backfill.md` (closes first, after its phase 5 rule flip) |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner decision D-15 of `spec-rfc-requirement-quote-hand-backfill` (2026-09-27)
moved this work out of that spec. Once every requirement row in `rfc/short/`
quoted its RFC sentence, the strict re-read under D-6 and D-9 left verdicts in
`rfc/audit/<stem>.json` where the tagged tests prove less than the sentence says.
D-6 had already ruled that the quoting pass does not fix tests, and that a later
pass groups the fixes by package so that two agents never edit the same test file.
This is that pass.

**Goal.** Every `weak` and `wrong` verdict in `rfc/audit/` is resolved, in one of
two ways:

1. **A test proves the whole sentence.** Every clause of the quoted sentence is
   asserted in both polarities: the compliant input gives the RFC's outcome, and
   the non-compliant input gives the RFC's refusal. The tagged unit carries a
   discrimination record from `./le rfc discriminate-record`, and the verdict is
   re-judged `enforced` by an agent that did not write the test.
2. **The row is corrected.** The row claimed more than its RFC states, or the
   wrong sentence: it is corrected, retired or re-attributed under D-2, D-3, D-7
   and D-10 of the source spec, and its verdict is re-judged against the new text.

A verified defect that a test exposes is fixed in code under D-8: failing test
first, then the fix, with interop where the protocol has a peer. It is never
recorded as a `{gap}` in its place. Where a fix proves impossible, it goes back to
the owner.

**Also in scope, per D-15 (main-thread call inside it):**

- **The split-needed rows.** When a stem agent quoted a row, the verbatim span
  sometimes covered only part of what the old row claimed, because the rest sits
  in another section or is not consecutive. Each dropped obligation becomes its
  own row, and each new row owes tests like any other. The known set is in
  "Split-needed rows" below.
- **The narrowing audit.** Not every stem agent compared its quote against the
  old row text, and the mechanical backfill (1821 rows) never did. Every row whose
  text changed since the quoting began is compared against its pre-quote text for
  an obligation the quote dropped. Each one found joins the split-needed set.
- **The row-quality corrections.** The blind samples and send-back re-reads of
  2026-09-27 found rows that lose no obligation but quote a fragment, a list
  pointer or a bare pronoun, carry a level their sentence does not, duplicate
  another row, or are tagged by a unit that proves a neighbour. They are listed in
  "Row-quality corrections" below.

The ledger already records each finding as `weak` or `wrong`, so no public claim
outruns the evidence while this spec is open.

## Known Inventory

The verdict inventory is DERIVED. No list of ids is committed here, because the
audit files change as the work proceeds.

| Question | Derived by |
|----------|-----------|
| Every weak or wrong verdict (stem, id, verdict) | `jq -r 'input_filename as $f \| .requirements \| to_entries[] \| select(.value.verdict=="weak" or .value.verdict=="wrong") \| "\($f)\t\(.key)\t\(.value.verdict)"' rfc/audit/*.json` |
| The same, grouped by the package of each tagged test | the same filter, emitting the directory of every key of `.value.tests` (the part before `::`) |
| The narrowing set: rows whose text changed since the quoting began | every `rfc/short/<stem>.md` row whose text differs between `0bf0696576^` (the parent of the commit that landed the quote check and the backfill) and `HEAD`, compared by id |

### Measured 2026-09-27, after the blind samples and the send-back re-reads

920 `weak` and 61 `wrong`, 981 in all, across 144 stems of the 156 audit files,
counted from `rfc/audit/*.json` in the working tree after the 14 send-back
commits of 2026-09-27, which the source spec's Mistake Log lists. Every one names
at least one tagged test. The first count, before the samples, was 838 `weak` and
56 `wrong`, 894 in all, across 135 stems of 150 files: the strict re-reads moved 87
more verdicts out of `enforced` and added six audit files, which together raised the total by 87.

| Stem | Weak + wrong | Of which wrong |
|------|--------------|----------------|
| rfc4271 | 58 | 3 |
| rfc5880 | 50 | 3 |
| rfc7296 | 36 | 2 |
| rfc2328 | 32 | 1 |
| rfc3768 | 28 | 2 |
| rfc9830 | 25 | 0 |
| rfc3748 | 23 | 0 |
| rfc9568 | 22 | 1 |
| rfc5798 | 21 | 0 |
| rfc5216 | 18 | 1 |
| rfc8907 | 18 | 0 |
| rfc9552 | 18 | 1 |
| rfc2661 | 17 | 0 |
| rfc4301 | 17 | 5 |
| rfc1195 | 16 | 2 |

Top test packages, by verdicts that tag a test in them (a verdict tagging two
packages counts in both): `internal/component/bgp/reactor` 76,
`internal/plugins/ospf` 66, `internal/core/eap` 49,
`internal/core/bgp/attribute` 48, `internal/component/bgp/message` 40,
`internal/component/ike/engine` 38, `internal/plugins/vrrp` 37,
`internal/plugins/rsvpte` 32, `internal/component/bgp/plugins/rib` 24,
`internal/plugins/vrrp/packet` 20.

### Split-needed rows

Recorded by the stem agents of the source spec in their reports under
`tmp/session/2026-09-26-f7fd86a1-7a8c-495e-89c0-011b55afa13e/scratch/work/agents/`.
The rows from RFC5880-6.7.2-4 and from RFC2328-8.2-3 onwards, and the rfc8210 missing
rows, come from the blind samples and send-back re-reads of 2026-09-27 (BS-A, SB-1,
SB-3, RA-8210 in the same directory).
That directory is scratch and does not last, so the facts are copied here. Each
row names the id whose quote dropped an obligation, what was dropped, and where
the RFC states it. "Row correction" marks a claim no sentence states, which is a
correction of the row, not a new row.

| ID | Dropped obligation | Section |
|----|--------------------|---------|
| RFC7752-3.3.1.3-1 | Link Name: use of the FQDN or a subset of it is strongly RECOMMENDED | §3.3.2.7 |
| RFC9552-5.3.1.3-1 | Link Name: use of the FQDN or a substring of it is strongly RECOMMENDED | §5.3.2.7 |
| RFC9552-5.3.2.2-3 | the sentence before Table 9: reserved bits MUST be set to zero and SHOULD be ignored on receipt | §5.3.2.2 |
| DRAFT-IETF-BESS-MUP-SAFI-3.3.3-4 | DSD nexthop/locator mismatch is a malformed NLRI, treat-as-withdraw | §3.3.6 |
| RFC9568-7.1-4 | split candidate: the VRID-configured MUST versus the owner check that erratum 8298 makes a SHOULD verify-and-log | §7.1, erratum 8298 |
| RFC7854-x-3 | per-peer header follows the common header for Route Monitoring, Route Mirroring, Stats and Peer Up; Initiation and Termination carry the common header only | §4.3, §4.5, §4.6, §4.7, §4.8, §4.10 |
| RFC7854-x-2, x-4, x-10 | octet widths stated only by the figures (6-octet total, 4-octet field, 1-octet Reason): row correction | §4.1, §4.2, §4.9 |
| RFC9256-4-2 | SRv6 SID behavior and structure MAY be provided, for types I, J and K | §4 |
| RFC9256-4-3 | SR Algorithm MAY be provided, for types I, J and K | §4 |
| RFC5036-3.5.3-1 | unacceptable parameters: Session Rejected/Parameters Error Notification and close the TCP connection | §2.5.3, §3.9 |
| RFC5036-2.5.1-3 | active role: on an acceptable Initialization message, reply with a KeepAlive | §2.5.3 item 2.b |
| RFC1195-5.3.4-2 | TLV 130 must not appear in pseudonode LSPs; TLV 131 is not included in pseudonode LSPs | §5.3.5, §3.4 |
| RFC1195-4.5-1 | IP-only router forwarding to an OSI-only router: the packet must be discarded | §4.5, second paragraph |
| RFC5880-6.7.2-2 | key SHOULD also be allowed in hexadecimal form, for Keyed MD5 and SHA1 | §6.7.3, §6.7.4 |
| RFC5880-6.7.3-5 | the XmitAuthSeq SHOULD, for SHA1 | §6.7.4 |
| RFC5880-6.7.2-3 | Auth Key ID MUST be set to the ID of the current key, for MD5 and SHA1 (the SHA1 tags on this row belong there) | §6.7.3, §6.7.4 |
| RFC5880-6.7.3-8 | Sequence Number MUST be set to bfd.XmitAuthSeq, for SHA1 (the Simple Password claim was fabricated: row correction) | §6.7.4 |
| RFC5880-6.7.3-9 | Keyed SHA1 receive window | §6.7.4 |
| RFC5880-6.7.3-10 | Meticulous SHA1 receive window | §6.7.4 |
| RFC5880-6.7.2-4 | the Auth Type discard for Keyed MD5 and Keyed SHA1; the quote is the Simple Password case, while the row's parenthetical still cites §6.7.3 and §6.7.4 | §6.7.3, §6.7.4 |
| RFC5880-6.7.2-5 | the Auth Key ID discard for Keyed MD5 and Keyed SHA1; the quote says "configured password", §6.7.2 only, while the parenthetical still cites §6.7.3 and §6.7.4 | §6.7.3, §6.7.4 |
| RFC5880-6.7.2-6 | the Auth Len discard for Keyed MD5 (24) and Keyed SHA1 (28); the quote is the Simple Password rule, password length plus three | §6.7.3, §6.7.4 |
| RFC3101-2.2-1 | the aggregate's forwarding address is set to 0.0.0.0 | §3.2 step (3), §2.3 |
| RFC1994-2-1 | SHOULD take action to terminate the link | §4.2 |
| RFC2205-2-9 | must not prevent a different receiver from establishing a smaller reservation Q0 | §2 |
| RFC2205-3-13 | a SCOPE object in a ResvTear must be ignored | §3.1.6 |
| RFC2205-3.1-3 | an all-zero checksum means no checksum was transmitted (no sentence states a MUST to drop: row level to review) | §3.1.1 |
| RFC2661-6.1-1 | a message missing a required AVP is logged and the control connection cleared (lowercase should) | §7.1 |
| RFC2661-6.2-1 | SCCRP not acceptable: send StopCCN, clean up (state table, no keyword) | §7.2.1 |
| RFC2661-24.10-1 | refusing a peer's zero Tunnel or Session ID with StopCCN (lowercase should) | §7.1 |
| RFC2661-10-1 | CDN is valid in any non-idle session state (state tables only) | §7.4.1, §7.4.2, §7.5.1, §7.5.2 |
| RFC8277-2.2-3 | Rsrv MUST be ignored on reception, multi-label encoding | §2.3 |
| RFC8277-2.2-4 | Rsrv SHOULD be set to zero on transmission, multi-label encoding | §2.3 |
| RFC8277-3.2.1-4 | after a next-hop change, a route not propagated MUST be withdrawn from that peer | §3.2.2 |
| RFC8669-4-2 | SHOULD log an error when discarding an attribute: the malformed and the invalid case | §6, §4.1 |
| RFC2865-3-4 | Access-Challenge: the Response Authenticator MUST be correct; invalid packets silently discarded | §4.4 |
| RFC2865-5.11-1 | MUST NOT affect operation of the protocol: Reply-Message, Framed-Route, Vendor-Specific, Proxy-State | §5.18, §5.22, §5.26, §5.33 |
| RFC2865-5.25-1 | the client MUST NOT interpret the State attribute locally (two of the row's tags prove this half) | §5.24 |
| RFC5392-3.2.1-4 | the Remote AS Number sub-TLV is REQUIRED in a Link TLV advertising an inter-AS TE link | §3.3.1 |
| RFC5882-4.2.1-1 | SHOULD signal lack of connectivity in the control protocol, or emulate a timeout | §4.2.2.1, §4.2.2.2 |
| RFC5882-10.1.3-2 | BFD authentication SHOULD be used for EBGP | §10.2 |
| RFC5176-2.3-5 | a Disconnect-Request MUST contain only NAS and session identification attributes; otherwise Disconnect-NAK | §3 |
| RFC5176-2.3-7 | with multi-session support, a CoA-Request or Disconnect-Request MUST apply to all matching sessions | §3 |
| RFC5176-3.2-1 | an unsupported Service-Type value MUST get a CoA-NAK | §2.2 |
| RFC5176-3.3-2 | the Dynamic Authorization Server MUST NOT interpret State locally | §3.3, last paragraph |
| RFC5176-3.5-5 | Error-Cause code placement: 201 only in Disconnect-ACK, 202 never sent, 502 not by a NAS, 504 only in Disconnect-NAK | §3.5, after the code table |
| RFC5176-3.6-1 | the same attribute MUST NOT be used for identification and authorization at once (Note 7) | §3.6 |
| RFC5176-6.3-1 | the duplicate-detection window MUST equal the stale Event-Timestamp window | §6.3 |
| RFC5176-3.1-1 | "in the order they arrived" binds only a forwarding proxy, not the NAS: row correction | §3.1 |
| RFC4555-x-4 | address advertisement later in INFORMATIONAL requests; the initiator MAY put it in the UPDATE_SA_ADDRESSES request | §3.6 |
| RFC8665-3.1-7 | area-scoped flooding is REQUIRED for the SID/Label Range TLV and the SRLB TLV | §3.2, §3.3 |
| RFC5709-3.2-4 | key storage SHOULD persist across a warm or cold restart | §3.2 |
| RFC7950-9.1-1 | a type with no canonical form: the value MUST match its lexical representation | §9.1 |
| RFC7950-11-1 | changed semantics MUST get a new definition and identifier; data definition substatements MUST NOT be reordered | §11 |
| RFC7770-2.1-1 | OSPFv3 twin: Instance ID 0 SHOULD carry the Router Informational Capabilities TLV | §2.2 |
| RFC8414-3.3-1 | confidentiality protection MUST use TLS with a ciphersuite giving confidentiality and integrity | §6.1 |
| RFC9069-5.2-1 | each emulated peer MUST send a Peer Up whose OPEN indicates its address-family capabilities | §6.1.1 |
| RFC4659-8-3 | over IPv6 tunnelling, the BGP Next Hop SHALL contain an IPv6 address | §8 |
| RFC2869-x-2, x-5 | the Gigaword obligations for Acct-Output-Gigawords | §5.2 |
| RFC5305-3.2-1 | MUST NOT inject a /32 for the neighbor address, nor for the router ID | §3.3, §4.3 |
| RFC5305-3.2-2 | a TE router MUST include the sub-TLV on point-to-point adjacencies | §3.3 |
| RFC5305-3.1-1 | "SHOULD appear once at most", for the other sub-TLVs | §3.4, §3.5, §3.6, §3.7 |
| RFC9085-2.1-1 | the same sentence for the Link NLRI and the Prefix NLRI | §2.2, §2.3 |
| RFC9085-2.1.2-3 | Reserved ignored on receipt, in the other TLVs | §2.1.4, §2.2.1, §2.2.2, §2.3.1, §2.3.5 |
| RFC9085-2.1.2-4 | the flags sentence of the other TLV | §2.1.4 |
| RFC905-x-2 | set the checksum field to zero; initialize C0 and C1 to zero | §B.3.1, §B.3.2 |
| RFC905-x-3 | place X and Y in octets n and n+1 | §B.3.5 |
| RFC905-x-4 | re-sum over every octet including the checksum field | §B.4.2 |
| RFC3786-x-1 | the Partition Repair bit SHOULD be zero on all extended LSPs | §3.1.2 |
| RFC3786-x-2 | other neighbors MUST NOT be specified in an Extended LSP (row is `{not-applicable}`, untagged) | §3.2.1 |
| RFC3787-x-1 | TLV 133 is not used and MUST be ignored (the tags describe both halves) | §3.2 |
| RFC3787-x-2 | MUST include TLV 132 in IIH PDUs (TestISISIIHOriginationTLVs and TestISISHelloTLV132RequiresInterfaceAddr prove it) | §10 |
| RFC3787-4-1 | "set it in non-pseudonode LSP number Zero", descriptive only: row correction | §4 |
| RFC3032-2.1-1 | reserved label values 0 to 2 | §2.1 items i to iii |
| SFLOW-V5-x-7, x-9, x-10, x-12, x-15, x-20, x-24 | claims the document does not state ("wrapping", "2^30-1", "since boot", "[1, 2*N-1]", XDR alignment) or states in a sibling row: row correction | §3.1, §4.3, §5 |
| SFLOW-V5-x-11 | a `sample_pool` field description tagged MUST; no sentence states an obligation: row correction (the code defect is D2 of `plan/immediate/spec-bmp-sflow-export-rfc-defects.md`) | §5 |
| RFC1661-5.8-5 | the same Magic-Number sentence for Discard-Request, "Until the Magic-Number Configuration Option has been successfully negotiated, the Magic-Number MUST be transmitted as zero.": no row states it for §5.9 (RFC1661-5.9-1 says Ze sends no Discard-Request today) | §5.9 |
| RFC2328-8.2-3 | authenticate every received packet; accept a non-Hello packet only from an active neighbor. Neither is consecutive with the AuType sentence the row quotes | §D.4, §8.2 later paragraph |
| RFC2328-10.5-1 | declare bidirectional communication only when the router is listed in the neighbor's Hello | §10.5 later paragraph, §9.5 |
| RFC2328-10.6-1 | process DD packets in sequence and, as slave, reply to each; the quote is the §10.8 duplicate resend only | §10.6 |
| RFC2328-12.2-1 | a network-LSA is looked up on its Link State ID alone | §16.1 |
| RFC2328-12.4-1 | flush an AS-external-LSA for an unreachable destination, and an LSA no longer advertisable to an area; the quote is the §12.4.3 summary-LSA case | §12.4.4, §16.7 |
| RFC2328-12.4.3-1 | condense summaries as the configured area address ranges require | §12.4.3 |
| RFC2328-13-3 | drop an LS Acknowledgment from a neighbor below Exchange; the quote is the §13 LS Update case | §13.7 |
| RFC2328-13.3-3 | on non-broadcast networks, delayed LS Acknowledgments are sent as separate unicasts | §13.5 |
| RFC2328-13.4-1 | re-originate or flush by premature aging a received self-originated LSA; the quote is the detection only | §13.4 later paragraph, §14.1 |
| RFC2328-16.2-2 | the same skips for AS-external-LSAs; the quote is the §16.2 summary-LSA steps | §16.4 |
| RFC2328-D.3-2 | the sequence number is non-decreasing and reset to zero when the neighbor goes Down; Figure 18 separates it from the quoted sentence, so one span cannot carry both | §D.3 |
| RFC2347-x-2 | the client "must not use those options which were not acknowledged by the server"; RFC2347-x-3 does not carry it either (blind sample BS-A) | Negotiation Protocol |

Rows the stem agents found missing outright, which also owe tests:

| Stem | Missing obligation | Section |
|------|--------------------|---------|
| rfc5798 | a Master for non-owned addresses must determine which virtual router a packet was sent to, for the redirect source (lowercase must) | §8.1.1 |
| rfc5798 | the same for ICMPv6 redirects (lowercase must) | §8.2.1 |
| rfc5798 | advertisements to 224.0.0.18 should be encapsulated per RFC 1469 (lowercase should, Token Ring) | §A.2 |
| rfc9582 / 8210bis | the self-provider prohibition (RFC9582-5.12-2) and the multi-provider AS 0 MUST NOT (RFC9582-5.12-7) need their own 8210bis rows | 8210bis §5.12 |
| rfc8210 | SHOULD: "If the router has never issued a successful query against a particular cache, it SHOULD retry periodically using the default Retry Interval, above." | §6 |
| rfc8210 | SHOULD, cache side: the cache "SHOULD reject the connection if none of the iPAddress identities match the connection." (RFC8210-9.2-3 quotes only the MUST check before it) | §9.2 |
| rfc8210 | MAY: "host authentication MAY be supported. Implementations MAY support password authentication." (RFC8210-9.1-2 quotes only the user-authentication MUST before it) | §9.1 |

### Row-quality corrections

Found by the blind samples BS-A, BS-B and BS-C and the send-back re-reads SB-1,
SB-2, SB-3 and RA-8210 of 2026-09-27. Each row states an obligation, or the verdict
is judged against text the quote does not carry. None of them loses an obligation
outright, so none is in the split-needed table. Each is corrected, merged or
re-tagged in this pass, or a dated correction says why it stands.

| Kind | Rows | What is wrong | Correction owed |
|------|------|---------------|-----------------|
| list-pointer quote | RFC4301-4.4.1.1-1, 4.4.1-6, 4.4.2.1-1, 5.1-1, 5.2-3, 6.2-2 | each quote ends in "the following ...:" and states no obligation; the list that carries it is not quoted, and the verdicts were judged against the list | widen the span through the list, or split one row per list item |
| fragment quote, referent in a sibling row | RFC7296-3.1-8, 3.2-2, 3.2-4, 3.2-6, 2.10-2, 2.10-3, 2.21.4-4, 2.21.4-6, 2.5-7, 3.5-3, 3.14-3 | a sub-span of a sentence whose subject is in a sibling row's clause | widen to the list-item head |
| fragment quote, no referent | RFC7296-3.16-1, 3.16-2, 3.11-2, 3.1-9, 3.1-11 | the quote names no subject, and the span can be widened without a sibling | widen the span |
| bare-pronoun quote | RFC9012-3.1-3 ("it MUST be propagated unchanged"), 3.1-2 ("It MUST be disregarded"), 3.2.1-1 ("They MUST"), 3.7-2, 4.2-1, 4.3-2 ("the value") | the subject, such as the Reserved subfield, is in the sentence before | widen the span to the sentence that names the subject |
| level over a stronger keyword | RFC9012-11-7 | levelled SHOULD, but its quote also carries "MUST be able to filter the attribute from outgoing BGP UPDATE messages" | split the MUST into its own row, or re-level |
| level over a weaker keyword | RFC2759-x-8, x-9 | levelled MUST over a sentence whose only keyword is SHOULD | re-level, with a correction paragraph |
| level with no BCP 14 keyword | RFC7296-1.2-1, 2.6-1, 2.9-1, 2.23-3, 2.23-12, 2.4-1, 1.4-1, 2.8-2 (MUST NOT), 2.2-3; RFC4301-4.1-4, 7-1 (MUST NOT); RFC3748-4-2, 2-2, 4.2-1; RFC2759-x-3, x-10, x-12 (format and vector text); RFC3768-6.4.3-9 (pseudocode); RFC5301-3-8 (MUST NOT over "The string is not null-terminated."); RFC8050-4.2-1 (descriptive); RFC8050-x-4 (rationale) | D-3 permits a MUST level over a normative sentence without a keyword, so these are for review, not automatic demotion. SB-2 counted 17 of them in its four stems, the blind samples 4 more | review each; a demotion needs a correction paragraph per stem and an owner call on the format-definition rows |
| lowercase keyword | RFC8092-4-1 ("should") | levelled as a BCP 14 keyword | review the level |
| duplicate span | RFC7296-3.3.2-1 and RFC7296-3.3.6-1 | both now quote the same §3.3.3 span ("MUST understand all types" through the IKE line of the table); 3.3.6-1 is the D-H subset of 3.3.2-1 | merge, moving the tags of 3.3.6-1 |
| duplicate span | RFC8210-7-8 and RFC8210-5.2-1 | the §7 "The router MUST ignore any Serial Notify PDUs ... during this initial startup period" is the obligation 5.2-1 states, and 5.2-1 already cites §7 | merge |
| duplicate span | RFC3748-7.10-1 and 7.10-2 | 7.10-2 quotes a sub-span of 7.10-1's sentence; the tags of each cover a different half, so both are weak | merge, keeping every tag |
| duplicate span | RFC3748-4.1-11 against RFC3748-4.1-5 and RFC3748-2.1-3 | its first sentence duplicates 4.1-5, its Nak-after-non-Nak sentence duplicates 2.1-3 | merge into the two rows |
| id names another section | RFC5880-4.1-1 | the id names §4.1, but the quote and cite are the §6.8.7 transmit rule; no obligation is lost (the receive half is RFC5880-6.8.6-1), and an id is permanent | a dated correction saying why the id stands |
| text after the cite, or a wrapped hyphen | RFC3630-1-1 (prose after the section cite); DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-1 ("strict- mode"), RFC7684-5-1 ("sub- TLV") | the prose is not RFC text, and the hyphen split is the RFC's line wrap carried into the quote | move the prose out of the quote; join the hyphen |

Mistagged units, whose tag names a row the unit does not prove:

| Unit | Tagged under | What it proves instead | Correction owed |
|------|--------------|------------------------|-----------------|
| `internal/component/ike/engine/child_rekey_initiator_answer_test.go::TestChildRekeyAnswerWithoutTrafficSelectorsIsRefused`, and `rekey_test.go::TestRekeyWithoutTrafficSelectorsIsRefused` | RFC7296-2.9-1 | TS payload presence in a rekey answer, a neighbouring obligation; the verdict stays `enforced` on the other units | move the tags to the row that states the TS presence rule, or add it |
| `internal/component/ike/engine/rfc4301_spd_discard_test.go::TestSPDPolicyMirrorsTheInboundSelector` | RFC4301-4.4.1-4 | direction mirroring, not administrator ordering; the verdict stays `enforced` on the ordering units | move the tag |
| `internal/component/bgp/plugins/rpki/rtr_session_test.go::TestCacheResetTriggersResetQuery` | RFC8210-8.3-1 | the tag prose says "ze runs every configured cache in parallel", but `cacheGroup` is preference-ordered since 2026-09-20 | correct the tag prose, and re-judge the unit against the more-preferred-cache SHOULD of the quote |

### Code defects recorded by the stem agents

This spec declares no code defect. Each defect the stem agents recorded in their
`findings.tsv` files was confirmed at its producer in HEAD on 2026-09-27 and now
lives in exactly one spec, which carries its RFC quote, producer, verdict and
owner decisions. D-8 still holds: a verified defect is fixed in code, by that
spec.

| Spec | Rows it owns |
|------|--------------|
| `plan/immediate/spec-bgp-prefix-sid-rfc-defects.md` | RFC8669-6-1, RFC9252-3.4-1, RFC9252-3.2.1-3, RFC9252-5-1, RFC9252-7-1 |
| `plan/immediate/spec-bgp-graceful-restart-rfc-defects.md` | RFC4724-4-2, RFC9494-4.2-8, RFC9494-4.3-1, RFC9494-4.3-3, RFC9494-4.2-6 with 4.3-2 |
| `plan/immediate/spec-bgp-open-session-rfc-defects.md` | RFC9072-2-1, RFC6286-2.1-1, and two Ze behaviour defects with journal rows |
| `plan/immediate/spec-bgp-update-propagation-rfc-defects.md` | RFC4271-4.3-3 and 4.3-4, RFC8092-3-1, MUP 3.1.3.1-6/-7/-10, RFC9234-3.1-1, RFC7999-3.1-2, RFC 2545 Section 3 with LINKLOCAL-CAPABILITY-4-2 |
| `plan/immediate/spec-bgp-sr-policy-rfc-defects.md` | RFC 9830 Section 2.4.2 flags and BSID label, RFC9830-4.2.1-2 |
| `plan/immediate/spec-bmp-sflow-export-rfc-defects.md` | RFC 7854 Section 4.9 reason 4, SFLOW-V5-x-11 |
| `plan/immediate/spec-vrrp-owner-arp-rfc-defects.md` | RFC 9568 Section 8.1.2, and the vpp verifier parity journal row |
| `plan/immediate/spec-radius-rfc-defects.md` | RFC3579-3.3-2, RFC5176-2.3-2, RFC 2866 Section 5.5 |
| `plan/immediate/spec-ike-eap-rfc-defects.md` | RFC3948-2.1-2, RFC5216-2.1.1-4 |
| `plan/immediate/spec-ospf-nssa-translator-reachability.md` | RFC3101-3.2-2, the reachability clause |
| `plan/immediate/spec-rsvpte-frr-link-protection-fallback.md` | RFC 4090 Section 6 (no row), RFC4090-6-6 |
| `plan/immediate/spec-mcp-protected-resource-metadata-path.md` | RFC9728-3.1-3 |

Dropped as already fixed in HEAD, 26 rows. OSPF by `b12697744d`: RFC2328-D.4.3,
RFC3101-3.1-1, the equivalence part of RFC3101-3.2-2, RFC3623-5-2, RFC5286-x-2
and x-3, RFC5340-A.4.7-1, RFC8666-6-7, RFC8665-9-1, RFC5392-4. BFD by
`7757c3e955`: RFC5880-6.7.3-9, the SHA1 sequence seed (D-14), RFC5881-3-1, the
RFC5883-3-1 citation, the key length, RFC5880-6.8.1-5, 6.8.3-1, 6.8.6-8, 6.8.7-5,
and the passesTTLGate comment. VRRP by `850eb41b66`: RFC9568-7.1-4, RFC9568-7.1-12,
RFC3768-7.1-4, RFC3768-7.1-7, RFC3768-8.2-1, and the Section 6.4.3 tie-break on
the primary IP. softver by `90579b6a4f`: DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-3.

Dropped after reading, with the reason:

| Lead | Why it is not a code defect |
|------|-----------------------------|
| NodeDescriptor sub-TLV 518 repeated (RA12) | RFC 9552 Section 5.2.1.4 does forbid it: "At most, there MUST be one instance of each sub-TLV type present in any Node Descriptor." But no path emits it. The parser (`parseNodeDescriptorTLVsAt`, `nlri/ls/types.go`) is the only non-test writer of `NodeDescriptor.SRv6SIDs`, every received NLRI with a repeated sub-TLV is discarded first by `bgplsNodeDescriptorWellFormed` (`message/rfc7606_bgpls_nlri.go`), and `ls_export` never sets the field. What remains is test-side and stays in this spec: the RFC9552-5.1-2 units assert the forbidden encoding (verdict `wrong`), and the comment at `nlri/ls/types_descriptor.go` claiming 518 is "the only sub-TLV a descriptor can repeat" is false. RFC 9514 Section 6 puts 518 in the SRv6 SID Descriptors ("This field MUST contain a single SRv6 SID Information TLV") |
| DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2 | not dropped: confirmed, and owned by `spec-bgp-update-propagation-rfc-defects.md` D6 |
| RFC6286-2.1-1 per-peer router-id | not dropped: confirmed at `parsePeerSettings`, and owned by `spec-bgp-open-session-rfc-defects.md` D4 as an owner decision |

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - the discrimination record, the row quote, the refusals
  → Constraint: [to fill in design]
- [ ] `ai/skills/ze-rfc-audit.md` - the four judgement questions and the verdict vocabulary; STRICTNESS
  → Constraint: an `upgrade_reason` is owed for any weak or wrong to enforced move with no unit change
- [ ] `plan/pre-release/spec-rfc-requirement-quote-hand-backfill.md` - D-2, D-3, D-6, D-7, D-8, D-9, D-10, D-15 and the stem brief
  → Decision: [to fill in design]
- [ ] `plan/pre-release/spec-rfc-requirement-reattribution.md` - a re-attribution the gates refuse today
  → Constraint: [to fill in design]

**Key insights:**
- [to fill in design]

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/check.go` - [to fill in design: the gates this pass satisfies; confirm the file names with `gopls symbols`]
- [ ] `ai/skills/ze-rfc-audit.md` - the verdict rules this pass applies

**Behavior to preserve:**
- `./le rfc check` stays green on every commit of the pass: a weak or wrong verdict is never deleted, never upgraded without an `upgrade_reason`

**Behavior to change:**
- [to fill in design: test changes, row corrections and code fixes, per package]

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le rfc check`, reading `rfc/short/`, `rfc/audit/`, `rfc/discrimination/` and the tags in test files

### Transformation Path
1. The agent changes a tagged test, a row, or a producer
2. `./le rfc discriminate-record` records the observed red under a break
3. The independent auditor writes the verdict; `./le rfc audit-stamp` stamps it
4. `./le rfc check` compares verdicts, records and rows

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| test file to ledger | the RFC requirement tag | No |

### Integration Points
- [to fill in design]

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | |
| No duplicated functionality | No | |

## Method

| Rule | Why |
|------|-----|
| One agent per package, and never two agents on the same test file | the split D-6 made: stem agents shared test files, package agents do not |
| Strict judgement, per `ai/skills/ze-rfc-audit.md` | a floor assertion, a confounded buffer or a positive that proves only "no error" stays `weak` |
| Every clause of the quoted sentence, in both polarities | a one-sided test passes against a stub of the other side |
| A discrimination record for every tag added or changed | `ai/rules/rfc-compliance.md`: the proof is an observed red under a recorded break |
| An independent re-audit of every re-judged verdict | an author does not judge its own tests (`ai/rules/principles.md`) |
| Order by operator impact: BGP, BFD, OSPF, VRRP and IKE first, then the rest | these are the protocols an operator meets first on the release |
| A code defect: failing test first, then the fix, with interop where there is a peer | D-8 |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `0bf0696576^` holds the pre-quote text of every row the backfill and the hand pass changed | the commit message of `0bf0696576` names the check and the 1821-row backfill | the narrowing set misses rows quoted earlier | diff of `0bf0696576^` against its predecessor quote commits | unvalidated |
| A-2 | the split-needed table above is the whole set the stem agents recorded | grep of every report's split sections and every findings.tsv, 2026-09-27 | a split is lost when the scratch directory goes | the narrowing audit re-derives it | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | two package agents touch the same test file through a shared helper | a commit carries another agent's hunk | assign by test file, not by directory, where helpers are shared |
| R-2 | a fix to a test reveals a defect that grows past the package | a code change outside the package | D-8: fix it; where impossible, back to the owner |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | test-only changes break nothing an operator sees; D-8 code fixes change protocol behavior and carry their own tests and interop |
| How is it reverted? | one commit per package |
| Who else touches this path? | `spec-rfc-evidence-strength-*` specs working the same audit files |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` | → | the audit, discrimination and quote gates in `internal/le/rfc` | [to fill in design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | the derived weak-or-wrong listing over `rfc/audit/*.json` | lists nothing |
| AC-2 | every verdict moved to `enforced` in this pass | its tagged units carry a discrimination record, and an agent other than the test's author judged it |
| AC-3 | every row in the split-needed table and the missing-rows table | the dropped obligation is a row of its own, or a dated correction says why not, and the new row carries a verdict |
| AC-4 | the narrowing set | every row compared against its pre-quote text, and every dropped obligation handled as AC-3 |
| AC-5 | every spec in the code-defects pointer list | this spec declares none of their defects and does not fix them; a test this spec corrects whose producer one of them changes waits for, or lands with, that spec |
| AC-6 | `./le rfc check` | no violation, no stale verdict |
| AC-7 | every row and unit in the row-quality and mistagged-unit tables | corrected, merged or re-tagged, or a dated correction says why it stands; a changed row or tag is re-judged |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [per package, to fill in design] | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [to fill in design, for the D-8 fixes] | | | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| [to fill in design, for each D-8 fix with a wire-visible change] | | | | |

## Files to Modify
- `rfc/audit/<stem>.json`, `rfc/short/<stem>.md`, `rfc/corrections/<stem>.md`, `rfc/discrimination/<stem>.json` - verdicts, split rows, corrections, records
- tagged `*_test.go` files, per package - [to fill in design]
- the producers in the code-defects table - D-8 fixes

## Files to Create
- [to fill in design]

## Implementation Steps

1. **Phase: inventory** - run the derived listing, group by test package, confirm each code-defects row at its producer, run the narrowing audit and extend the split-needed set
2. **Phase: BGP, BFD, OSPF, VRRP, IKE packages** - one agent per package; independent re-audit per package
3. **Phase: the remaining packages** - same brief
4. **Phase: split rows and missing rows** - add the rows, tag and audit them
5. **Phase: row-quality corrections** - the row-quality and mistagged-unit tables, one stem at a time, re-judging each changed row

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Group by test package | group by stem, as the quoting pass did | stems share test files, so stem agents collided (D-6) |
| Derive the verdict inventory, commit only the split table | commit the 894 ids | the audit files are the source; a copy drifts from the first fix. The split facts live only in scratch |

## Known Limitations

- [to fill in design]

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
- [ ] AC-1..AC-N all demonstrated
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
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/pre-release/spec-rfc-verdict-test-fix-pass.md` only, in the same `./le commit create` script
