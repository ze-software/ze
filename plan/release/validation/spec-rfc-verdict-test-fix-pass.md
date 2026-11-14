# Spec: rfc-verdict-test-fix-pass

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P2 |
| Release reason | Required evidence for shipped behavior or truthful release support claims; historical work and proof debts remain distinct. |
| Scope | tooling |
| Depends | `spec-rfc-requirement-quote-hand-backfill` (closed 2026-09-27, `f265152e15`); the eight children, each closed and removed: spec-rfc-verdict-fix-bgp `3c417fdab2` (2026-10-24), spec-rfc-verdict-fix-bfd `22fb2639f0` (2026-10-12), spec-rfc-verdict-fix-ospf `3a490b812e` (2026-11-14), spec-rfc-verdict-fix-vrrp `3b30602d2e` (2026-09-30), spec-rfc-verdict-fix-ike-eap `44797abe31` (2026-11-11), spec-rfc-verdict-fix-access `a5ca6d22cd` (2026-11-12), spec-rfc-verdict-fix-routing `7523c321e8` (2026-10-12), spec-rfc-verdict-fix-services `22b8957163` (2026-11-14) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-14 |

Set aside 2026-10-09 (owner: "Set aside the 7 specs waiting on something else"): AC-11, parent closure, needed every child closed. The last child closed on 2026-11-14 (`22b8957163`), so the set-aside no longer holds. The spec leaves `blocked` for `ready`, and `./le spec claim` moves it to `in-progress` for its closure, since a direct edit cannot reserve a WIP slot.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner decision D-15 of `spec-rfc-requirement-quote-hand-backfill` (2026-09-27)
moved this work out of that spec. Once every requirement row in `rfc/short/`
quoted its RFC sentence, the strict re-read under D-6 and D-9 left verdicts in
`rfc/audit/<stem>.json` where the tagged tests prove less than the sentence says.
D-6 had already ruled that the quoting pass does not fix tests, and that a later
pass groups the fixes by package so that two agents never edit the same test file.
This is that pass.

**Goal.** Every `weak` and `wrong` verdict is resolved: each one in `rfc/audit/`,
and each one in "Un-enrolled R-7 rows" below, which belongs to a stem with no
audit file and so is recorded only in the source spec's R-7 table. Each is
resolved in one of two ways:

1. **A test proves the whole sentence.** Every clause of the quoted sentence is
   asserted in both polarities: the compliant input gives the RFC's outcome, and
   the non-compliant input gives the RFC's refusal. The tagged unit carries a
   discrimination record from `./le rfc record`, and the verdict is
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

## Split (owner decision, 2026-09-28)

This spec is the PARENT. It holds the method, the inventory and the cross-cutting
work. One child spec per protocol group carries the test fixes, and each child
closes on its own when its packages have no `weak` or `wrong` verdict.

| Order | Child | Test packages |
|-------|-------|---------------|
| 1 | BGP | `internal/component/bgp/...`, `internal/core/bgp/...`, BGP interop and MRT |
| 2 | BFD | `internal/component/bfd/...` |
| 3 | OSPF | `internal/plugins/ospf/...` |
| 4 | VRRP | `internal/plugins/vrrp/...` |
| 5 | IKE/EAP | `internal/component/ike/...`, `internal/core/eap` |
| 6 | access | L2TP, PPP, PPPoE, RADIUS, TACACS |
| 7 | routing | IS-IS, RSVP-TE, LDP and the other routing packages |
| 8 | services | DNS, TFTP, DHCP, flow export, MCP, config and YANG, the rest |

Progress, 2026-10-06: the BGP child's Linux continuation records the verified
chunks, outstanding evidence and owner-approved AIGP carrier replacement in
spec-rfc-verdict-fix-bgp, "Linux continuation".
The RR, initial-sync unwind, normalized OPEN, ModCopy carrier, OSPF oracle and
authored RFC records/corrections are committed; evidence limits remain explicit.
Neither child nor parent is closed. The owner requested a pause after the current
AIGP repair, scoped commits and a progress report, without dropping remaining
acceptance criteria.

Progress, 2026-10-07: FRR's same-domain prerequisite is committed in `01ad560278`;
the resulting route-server withdrawal-ownership repair is committed in
`b880c5a9a7`, with startup transport lifetime repaired separately in `6201025f8c`.
The unchanged native AIGP scenario now passes all 11/7/zero/7 transitions.
Complete reactor/RIB packages passed under race once, 61 selected roots each
passed 20 executions, and all five committed build flavors passed. Child
AC-W1–W5 and its "Ownership repair landing and proof" section retain the logs,
archive, approved fixture changes and coverage limits. Wiki `ad06d64` and site
`2258321885` publish the captured-session readiness contract locally; nothing
was pushed. The owner-requested pause follows this checkpoint, with remaining
forwarding, LLGR, OSPF and RFC acceptance still open. Neither spec is closed.

The owner subsequently resumed that remaining scope with "ok continue use agents"
and "use agents to do this in parrallel". Five independent slices now cover
forwarding, LLGR, inter-AS OSPF, readiness proof and RFC/closure inventory.
The child retains the checkpoint evidence and owns the resumed acceptance;
shared-image execution and canonical RFC writes stay serialized. No push is
authorized, and neither spec's closure criteria are reduced.

Progress, 2026-10-09: BGP source landed as `d3e5b4a770`, followed by the
independently completed attribute, wire, Loc-RIB ownership, interop-recorder and
BFD observer repairs recorded in the child. Both specs remain open.

The owner's 2026-10-08 "Use Docker" decision is active; no push is authorized.
Restored structured Docker proof `9631a78e` passes the named ADD-PATH collision
and IPv6 joint-subnet scenarios. The complete FRR capture fence passes in
`b69d6620`; replacement replay and joint matrix each completed eighty physical
runs. Their old endpoint/deadline/empty-capture limitations are historical.

The child's "Acceptance evidence, 2026-10-09" and "Remaining closure obligations,
2026-10-09" retain exact proof boundaries. All seven guest records, twenty IPv6
unit records and37 other writer records completed with observed reds. Actual
ordinary SDK and raw-export SDK old/final workflows passed their restored
wire assertions. Independent RFC2545 judgments now retain3-3 as partial,
3-4 enforced and the absent third-party address source as an explicit gap.
No address-discovery feature was commissioned.

Post-SRv6 immutable reactor race20 now passes2445 roots/48900 results with no
skips, failures or races (`033bf172`). Actual SRv6 SDK/FRR old/fixed proof and
configured VPNv6 wrong-global/nonzero-RD discrimination are complete. Ten family
and thirteen tunnel judgments are stamped; SRv6 allocation, ASBR service and VPN
pair origination remain explicit gaps, not new implementation scope.

The later framing review reproduced a parsed splitter fabricating IPv6 EOR
from mixed input. Producer and cached-worker old/fixed proof, full message
race20, and actual FRR old/fixed proof now complete (`96e6c367`/`db0bba59`);
the complete captures and bounded repair have independent acceptance.
All ten remaining native claim bindings recorded in `8381327b`. Seven framing
judgments are stamped in `8338b5fb`, and four residual judgments in `3f7574f9`.
Postcommit `4d6f376d` exposes sixteen committed BGP record drifts and one
mechanical RFC4659 shift, beside the seven foreign findings. The precommit
error text omitted uncommitted drift; it did not prove complete freshness.
Native `bdd77596` renews all sixteen and reseals RFC4659. Final review's
eight missing judgments and three narrowing defects are resolved through
twelve first judgments (`e71102d3`, `11f306b4`), including the restored
downstream ESI-label row. Conditional discard logging and per-session
tunnel filtering have race20 and selective semantic proof (`2bf9854a`).
Projection and wiring proof complete in `e50a6d98`; its check still reports
the seven foreign violations, not whole-corpus green. Source/evidence
landed as `fb3251e5b597`; all five committed build flavors pass `02da6c39`.
Final-source reactor race20 completes with48919 passes and one recursive
AIGP wire-readiness failure, no race report (`2a6f2ef4`). A controlled
stale-completion schedule reproduces it; exact-wire readiness and complete
transition assertions pass that schedule20 and both normal recursive roots20
(`5173b7f5`/`bfa85e8a`). Independent review, four renewed bindings and two
whole-row rejudgments accompany the separate correction `fa54c8e59d`.
Postcommit `c58edbb0` compiles all five flavors at descendant `1f796eee10e4`;
structured RFC checking has eight foreign findings, including concurrent
RFC7950 drift, with no uncommitted record drift or changed unproven units.
The held compatibility fixture and explicit setup/CI dependencies land as
`0b3c569cac`, with the already-observed real-kernel setup-consumer transitions,
both unchanged live wire assertions, native bindings and independent reviews.
Final canonical reconciliation and held documentation land as `907e16d30d`.
Its five committed-source build flavors pass. The complete structured
postcommit receipt reports seven foreign findings, no record drift, no changed
unproven units and no missing records; RFC7950 no longer appears.
Final native publication from that exact source revision completes22 producers
and994 pages; Chromium verifies the retired catalog entry/detail and replacement
replay contract. The copied artifact check passes, and165 generated files land
locally as gh-pages `f996e9b00e`, without pushing or staging scratch metadata.
The completed-input archive and both complete Git bundles are retained and
verified. Whole `verify worktree` remains running; its receipt and the final
checkpoint still owe the small completion supplement.
Both specs remain open; no push or whole-corpus green claim is made.

| Stays in the parent | Why |
|---------------------|-----|
| The narrowing audit | it runs over every stem, and its output feeds every child |
| A verdict whose tagged tests span two groups, and a verdict with no tagged test | two children in parallel sessions would edit the same test file (D-6) |

The split-needed rows, the missing rows, the row-quality corrections and the
mistagged units go to the child that owns the stem's protocol.

**Owner decisions at the research gate, 2026-09-28:**

| ID | Decision |
|----|----------|
| P-1 | D-15 is the owner's standing approval for every tagged-unit edit and tag move this pass makes. A child records `./le rfc approve unit <pkg>.<Test> reason "D-15: ..."` itself, and each commit carries the `RFC-approved:` trailer. A-3 is confirmed |
| P-2 | A recorded verdict is re-judged through a re-judge mode of `./le rfc audit-stamp`, built as the parent's first phase. Hand-deleting an audit entry and re-stamping it is not used |
| P-3 | A child closes when every verdict in its packages is `enforced`, except those it lists as blocked by a named spec (the 12 code-defect specs, `spec-ipsec-rfc9190`, `spec-fixit-dns-rfc1035-conformance`, `spec-ike-dpd-demand-driven`); each blocked verdict moves into that spec's acceptance criteria |

Measured 2026-09-28: 952 `weak` and 62 `wrong`, 1014 in all, across 104 test
packages.

**Package placement refined 2026-09-28** (each move removes a cross-group verdict
or a stem shared by two children): `test/plugin` and `test/reload` to BGP (rfc4271,
rfc6793, rfc7999, rfc9234); `test/parse` to IKE/EAP (rfc7296); `internal/core/network`
to BGP (rfc2385, rfc5082); `internal/plugins/flowspec-firewall` and
`internal/component/sysrib` to BGP (rfc8955, rfc7311); in `internal/plugins/fib/kernel`
the SRv6 nexthop test goes to BGP (rfc9252) and the on-link test stays in routing
(rfc1195); the IS-IS BGP-LS export test goes to BGP (rfc9552 only).

| Child | Weak | Wrong | Stems | Largest packages |
|-------|------|-------|-------|------------------|
| BGP | 344 | 14 | 61 | bgp/reactor 76, core/bgp/attribute 48, bgp/message 43, bgp/plugins/rib 27, nlri/ls 25 |
| BFD | 62 | 4 | 4 | bfd/session 19, bfd/engine 19, bfd/auth 17 |
| OSPF | 114 | 9 | 25 | ospf 66, ospf/lsdb 19, ospf/packet 16 |
| VRRP | 70 | 3 | 4 | vrrp 37, vrrp/packet 20, vrrp/fsm 15 |
| IKE/EAP | 109 | 10 | 10 | core/eap 49, ike/engine 38, ike/dataplane 11 |
| access | 104 | 7 | 13 | l2tp/ppp 31, tacacs 18, l2tp 17, radius 16 |
| routing | 77 | 7 | 16 | rsvpte 32, isis 16, isis/packet 13 |
| services | 77 | 8 | 22 | tftpserver 12, config 9, flowexport/sflow 9, mcp 8 |

The exclusive per-child counts are in each child spec. The children total 1008,
and with the parent's 5 cross-group verdicts and RFC7296-2.4-2 that is 1014.

A verdict that spans two children counts in each. Five remain after the moves,
and they stay in the parent: RFC1071-1-4 (OSPF, services), RFC4303-2.1-1 (IKE/EAP,
OSPF), RFC5882-4.4-1 (BFD, BGP, OSPF), RFC905-x-3 and RFC905-x-4 (routing, OSPF).
One verdict has no tagged test: RFC7296-2.4-2, whose note points at
`plan/release/fixes/spec-ike-dpd-demand-driven.md`.

**One writer per ledger file.** `rfc/audit/<stem>.json`, `rfc/discrimination/<stem>.json`,
`rfc/short/<stem>.md` and `rfc/corrections/<stem>.md` are per stem, so a stem
whose verdicts fall in two children has one owner:

| Stem | Owner | Other child with tagged tests |
|------|-------|-------------------------------|
| rfc1071 | OSPF | services (`core/probe/icmp_test.go`) |
| rfc905 | routing | OSPF (the two OSPF checksum test files) |
| rfc4301, rfc4303 | IKE/EAP | OSPF (`ospf/config_ipsec_test.go`, `ospf/ipsec_install_test.go`) |
| rfc5882 | BFD | BGP (`reactor/config_bfd_strict_test.go`), OSPF (`ospf/rfc5882_shared_key_test.go`) |
| rfc792 | services | VRRP (`vrrp/gateway_icmp_integration_linux_test.go`) |

Test files that carry tags of two owners, and so are edited by one child at a
time: `ospf/config_ipsec_test.go`, `ospf/ipsec_install_test.go`,
`ospf/packet/checksum_test.go`, `core/probe/icmp_test.go`,
`reactor/config_bfd_strict_test.go`, `vrrp/gateway_icmp_integration_linux_test.go`
(rfc792 is owned by services, and the tags are in VRRP).

## Known Inventory

The verdict inventory is DERIVED. No list of ids is committed here, because the
audit files change as the work proceeds.

| Question | Derived by |
|----------|-----------|
| Every weak or wrong verdict (stem, id, verdict) | `jq -r 'input_filename as $f \| .requirements \| to_entries[] \| select(.value.verdict=="weak" or .value.verdict=="wrong") \| "\($f)\t\(.key)\t\(.value.verdict)"' rfc/audit/*.json` |
| The same, grouped by the package of each tagged test | the same filter, emitting the directory of every key of `.value.tests` (the part before `::`) |
| The narrowing set: rows whose text changed since the quoting began | every `rfc/short/<stem>.md` row whose quote text (cut at the section marker, `(§` or the older `(S`) differs between its baseline and `HEAD`, compared by id. The baseline is `fa7ac196df` (2026-09-20) for a row already verbatim in `rfc/full/<stem>.txt` at `0bf0696576^`, and `0bf0696576^` otherwise (A-1). Rows present at the baseline and gone at `HEAD` join the set |

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

### Un-enrolled R-7 rows

The jq listing above cannot see these: their stems have no `rfc/audit/<stem>.json`,
so the R-7 table of `spec-rfc-requirement-quote-hand-backfill` (read 2026-09-27
under the STRICTNESS rule) is their only record. That table holds 37 lines; the
22 below are its `weak` and `wrong` ones, 7 weak and 1 wrong from the first 21
lines and 14 weak from the 16 added by job AC7-E. A row leaves this table when
its re-judged verdict is `enforced`, recorded here with the date, or in the
stem's audit file if the stem is enrolled first.

| ID | Verdict | Unit | What the test fails to prove |
|----|---------|------|------------------------------|
| DRAFT-IETF-SIDROPS-8210BIS-5.12-1 | weak | `TestParseASPAPDU`, `TestParseASPAPDUMalformed` (`internal/component/bgp/plugins/rpki/rtr_pdu_test.go`) | that the router answers with an Error Report PDU carrying Error Code 9; both units stop at the parser sentinel `errASPAProviderList` |
| DRAFT-IETF-SIDROPS-8210BIS-5.12-3 | weak | `TestParseASPAPDU`, `TestParseASPAPDUUnsorted` (`rtr_pdu_test.go`) | "zero or more" providers: only the untagged `TestParseASPAPDUWithdraw` drives a withdrawal with none |
| DRAFT-IETF-SIDROPS-8210BIS-7-2 | weak | `TestRTRUnknownNegotiationVersion` (`rtr_session_test.go`) | the exception: an Error Report with an unrecognized version draws no Error Report back |
| RFC1035-2.3.4-1 | weak | `TestRFC1035_ConfiguredTTLBoundedToASigned32BitPositive` (`internal/plugins/geodns/rfc1035_rr_test.go`) | the lower bound of "positive"; the sibling 4.1.3-1 negative serves TTL 0, and the row does not cite RFC 2181 Section 8 |
| RFC1035-4.1.1-1 | weak | `TestRFC1035_ReservedZFieldIsZero` (`internal/core/dnsserver/rfc1035_header_test.go`) | "in all queries": Z is held clear in responses only; the queries Ze sends (`resolve/dns/resolver.go`, `as112/health.go`) are not checked |
| RFC1035-4.1.4-1 | weak | `TestRFC1035_CompressionPointersInATruncatedDatagram` (`internal/plugins/geodns/rfc1035_compression_test.go`) | "the label must begin with two zero bits": a label length octet of 0x40 to 0xBF passes both polarities |
| RFC1035-4.1.4-5 | weak | `TestRFC1035_InboundCompressionPointerUnderstood` (`rfc1035_compression_test.go`) | the answer does not depend on the pointer being expanded, and no assertion reads its expansion; replies Ze reads as a client are not driven |
| RFC1035-4.2.2-1 | weak | `TestRFC1035_TCPRepliesCarryATwoOctetLengthPrefix` (`rfc1035_compression_test.go`) | "use server port 53": the listener runs on a free port and no unit asserts the TCP default |
| RFC8362-2-1 | weak | `TestRFC8362ExtendedLSAsSetUBitOnTheWire`, `TestRFC8362BaseLSAsKeepUBitClearOnTheWire` (`internal/plugins/ospf/rfc8362_test.go`) | the U-bit on the E-Inter-Area-Prefix-LSA, the third Extended LSA Ze originates |
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
| SFLOW-V5-x-11 | a `sample_pool` field description tagged MUST; no sentence states an obligation: row correction (the code defect is D2 of `plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md`) | §5 |
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

### Narrowing audit findings, 2026-09-28

The narrowing audit (Implementation Step 4, AC-7) compared every row's current
quote against its old claim. A first reader called each row, and an independent second reader
re-read every flagged row against the RFC text. The per-row outputs are
`out-NN.tsv` and `review/second-*.tsv` under
`tmp/session/2026-09-28-869df689-cc8f-4d78-9161-1d7c87434c8e/scratch/inventory/narrow/`,
which is scratch, so the result is copied here.

| Measure | Value |
|---------|-------|
| Rows read | 4816 substantive rows in 25 batches, plus 81 removed rows |
| Removed rows | all 81 accounted for by `Retired` paragraphs; 12 of them retired under D-10 to un-summarized documents, already journaled in `plan/journal/gate-excludes-part-of-its-population.md` |
| First pass flagged (dropped, retire or moved) | 161 |
| Second read of all 161 | 129 agree, 1 re-called (RFC7311-3.2-2 retire to dropped, covered by RFC7311-3.3-5), 31 covered by another row or by the row's own current text |
| Confirmed findings already in the tables above | 56 |
| Confirmed new findings | 81, in the table below |
| Blind 20% sample of kept calls | 931 rows re-read, 8 dropped found: 4 already in the tables above, 4 new (RFC4271-8.2.2-13, RFC4271-8.2.2-8, RFC3209-4.4.3-4, RFC8666-6-13, all in the table below) |
| Residual | new-miss rate about 0.4%, so about 20 undetected drops are expected among the 3724 kept rows not sampled (R-12) |

Each quoted sentence below was re-checked verbatim in `rfc/full/<stem>.txt` or
`rfc/drafts/` on 2026-09-28; an ellipsis marks an elision between verbatim spans.
"dropped" means the current quote lost the obligation, "retire" means no sentence
of the document states the row's old claim, and "moved" means another document
states it. Stems with no child before this audit are placed by protocol: rfc2473
and rfc4213 to services, rfc3031 to routing, rfc5701 to BGP, rfc7166 to OSPF.

| ID | Call | Obligation | Section | Child |
|----|------|------------|---------|-------|
| DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-4 | dropped | "Otherwise the NLRI is considered as a malformed. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]." | §3.1.3.1 | BGP |
| DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-2 | dropped | "Otherwise the NLRI is considered as a malformed. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]." | §3.1.4.1 | BGP |
| DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3 | dropped | "When a BGP speaker receives a MP_REACH_NLRI attribute update message with a Direct Segment Discovery route without a prefix SID attribute, than it MUST be treated as if it contained a malformed prefix SID attribute and the "Treat-as-withdraw procedure of [RFC7606] is applied." | §3.3.6 | BGP |
| RFC4271-6.1-3 | dropped | "The Data field MUST contain the erroneous Length field." | §6.1 | BGP |
| RFC4271-8.2.2-13 | dropped | "If the local system receives a TcpConnectionFails event (Event 18), the local system: ... - increments the ConnectRetryCounter by 1," | §8.2.2 (Active state) | BGP |
| RFC4271-8.2.2-15 | dropped | "In response to any other event (Events 9, 12-13, 20-22), the local system: - sends a NOTIFICATION message with the Error Code Finite State Machine Error, - deletes all routes associated with this connection, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1" | §8.2.2 | BGP |
| RFC4271-8.2.2-8 | dropped | "In response to a ManualStop event (Event 2), the local system: ... - sets ConnectRetryCounter to zero," | §8.2.2 (Connect, Active; also OpenConfirm, Established) | BGP |
| RFC4271-9.1.2.1-5 | retire | No RFC 4271 sentence says connection establishment failure SHOULD be logged; quote is a different obligation (mutual-recursion logging). | none | BGP |
| RFC4271-9.2.2.2-2 | dropped | "Otherwise, if at least one route among routes that are aggregated has ORIGIN with the value EGP, then the aggregated route MUST have the ORIGIN attribute with the value EGP." | §9.2.2.2 | BGP |
| RFC4360-x-1 | moved | stated by RFC7606 7.14, not by this document: "The Extended Community attribute SHALL be considered malformed if its length is not a non-zero multiple of 8.". RFC 4360 only states 8-octet encoding (quote); explicit length multiple-of-8 rule is RFC 7606. | RFC7606 7.14 | BGP |
| RFC4456-8-3 | retire | Old MUST NOT create unless originator in local AS has no RFC sentence; quote is different SHOULD NOT create if one exists. | none | BGP |
| RFC4659-3.2.1.1-1 | dropped | "When the IPv6 VPN traffic is to be transported to the BGP speaker using IPv6 tunneling (e.g., IPv6 MPLS LSPs, IPsec-protected IPv6 tunnels), the BGP speaker SHALL advertise a Next Hop Network Address field containing a VPN-IPv6 address - whose 8-octet RD is set to zero, and - whose 16-octet IPv6 address is set to the global IPv6 address of the advertising BGP speaker." | §3.2.1.1 | BGP |
| RFC4659-3.2.1.1-2 | dropped | "The link-local address shall be included in the Next Hop field if and only if the advertising BGP speaker shares a common subnet with the peer the route is being advertised to [BGP-IPv6]." | §3.2.1.1 | BGP |
| RFC5492-5-1 | dropped | "Each such capability is encoded in the same way as it would be encoded in the OPEN message." | §5 | BGP |
| RFC5701-2-4 | moved | stated by RFC4360 6, not by this document: "If a route has a non-transitivity extended community, then before advertising the route across the Autonomous System boundary the community SHOULD be removed from the route.". RFC 5701 only defines the 0x40 bit; the propagation rule is RFC 4360 Section 6 | RFC4360 6 | BGP |
| RFC7311-3-2 | retire | No RFC 7311 sentence requires attribute length consistent with TLVs; quote is a TLV-set definition. | none | BGP |
| RFC7432-11.2-7 | dropped | "The re-advertised routes MUST be the same as the original ones, except for the PMSI Tunnel attribute and the label carried in that attribute." | §11.2 | BGP |
| RFC7432-8.1.1-2 | retire | No RFC sentence requires ESI Label on the ES route; quote is the ES-Import RT obligation, a different one. | none | BGP |
| RFC7432-8.2.1-4 | dropped | "This label MUST be a downstream assigned MPLS label if the advertising PE is using ingress replication for receiving multicast, broadcast, or unknown unicast traffic from other PEs." | §8.2.1 | BGP |
| RFC7947-x-6 | retire | No RFC 7947 sentence permits ADD-PATH use; 2.3 is informative; quote carries no obligation. Nearest: ADD-PATH should enforce send-only mode (2.3.2.2.2). | none | BGP |
| RFC8955-4.2.2.4-1 | dropped | "Type 5 component values SHOULD be encoded as 1- or 2-octet quantities (numeric_op len=00 or len=01). \| Type 6 component values SHOULD be encoded as 1- or 2-octet quantities (numeric_op len=00 or len=01). \| Type 10 component values SHOULD be encoded as 1- or 2-octet quantities (numeric_op len=00 or len=01)." | §4.2.2.5; 4.2.2.6; 4.2.2.10 | BGP |
| RFC8955-4.2.2.7-1 | dropped | "Type 8 component values SHOULD be encoded as single octet (numeric_op len=00)." | §4.2.2.8 | BGP |
| RFC8955-6-2 | moved | stated by RFC 9117 Section 4.2, not by this document. Leftmost-ASN-matches-best-match-unicast rule is RFC 9117 Section 4.2 (text not in rfc/); quote is RFC 8955 neighbor-AS rule. | RFC 9117 Section 4.2 | BGP |
| RFC8955-7.1-4 | dropped | "A traffic-rate-packets of 0 should result in all traffic for the particular flow to be discarded." | §7.2 | BGP |
| RFC9012-15-1 | dropped | "This implies that the duty to filter external traffic extends to all routers participating in such tunnels." | §15 | BGP |
| RFC9012-3.2.4-1 | dropped | "Unless a key value is being advertised, the MPLS-in-GRE Encapsulation sub-TLV MUST NOT be present." | §3.2.5 | BGP |
| RFC9136-3.1-4 | dropped | "It MUST be all bytes zero otherwise." | §3.1 | BGP |
| RFC9494-4.2-1 | dropped | "The interval for which they are retained is limited by the sum of the Restart Time in the received Graceful Restart Capability and the Long-Lived Stale Time in the received Long-Lived Graceful Restart Capability." | §4.2 | BGP |
| RFC9830-5-1 | dropped | "This includes the validation of the length of each NLRI and the total length of the MP_REACH_NLRI and MP_UNREACH_NLRI attributes. It also includes the validation of the consistency of the NLRI length with the AFI and the endpoint address as specified in Section 2.1." | §5 | BGP |
| RFC5880-6.7.3-11 | dropped | "Otherwise (the hash does not match the Auth Key/Hash field), the received packet MUST be discarded." | §6.7.4 | BFD |
| RFC5883-5-2 | retire | No RFC sentence requires binding RX sockets per session type; quote duplicates RFC5883-5-1 port 4784 obligation. | none | BFD |
| RFC2328-13-6 | dropped | "The best route to the destination described by the summary-LSA must be recalculated (see Section 16.5)." | §13.2 | OSPF |
| RFC2328-9.5.1-1 | dropped | "The interface state must be at least Waiting for any Hello Packets to be sent out the NBMA interface." | §9.5.1 | OSPF |
| RFC3101-2.3-1 | dropped | "The Type field in the LSA header is 7." | §2.3 | OSPF |
| RFC3101-2.4-4 | dropped | "A Type-7 default LSA may be installed by NSSA border routers if and only if its P-bit is set." | §2.4 | OSPF |
| RFC3101-3.1-2 | dropped | "If there exists another border router in this list whose router-LSA has bit Nt set or who has a higher router ID, then its NSSATranslatorState is disabled." | §3.1 | OSPF |
| RFC5250-5-1 | dropped | "If no entries exist for the ASBR (i.e., the ASBR is unreachable), the router MUST do nothing with this LSA." | §5 | OSPF |
| RFC5709-3.3-1 | dropped | "Apad is the hexadecimal value 0x878FE1F3 repeated (L/4) times." | §3.3 | OSPF |
| RFC7166-4.6-4 | dropped | "If the two do not match, the packet MUST be discarded, and an error event SHOULD be logged." | §4.6 | OSPF |
| RFC8665-5-7 | dropped | "This MUST be done regardless of whether the next-hop router contributes to the best path to the prefix." | §5 | OSPF |
| RFC8666-6-13 | dropped | "If both the NP-Flag and E-Flag are set, then: Any upstream neighbor of the Prefix-SID originator MUST replace the Prefix-SID with an Explicit NULL label." | §6 | OSPF |
| RFC8666-6-8 | dropped | "This MUST be done regardless of whether the next-hop router contributes to the best path to the prefix." | §6 | OSPF |
| RFC3768-6.4.3-1 | dropped | "When a host sends an ARP request for one of the virtual router IP addresses, the Master virtual router MUST respond to the ARP request with the virtual MAC address for the virtual router." | §8.2 | VRRP |
| RFC9568-5.2.8-1 | dropped | "For the IPv6 address family, the checksum calculation also includes a prepended "pseudo-header", as defined in Section 8.1 of [RFC8200]." | §5.2.8 | VRRP |
| RFC9568-6.4.3-1 | dropped | "When a host sends an ARP request for one of the Virtual Router IPv4 addresses, the Active Router MUST respond to the ARP request with an ARP response that indicates the Virtual Router MAC address for the Virtual Router." | §8.1.2 | VRRP |
| RFC9568-6.4.3-3 | dropped | "When a host sends an ND Neighbor Solicitation message for a Virtual Router IPv6 address, the Active Router MUST respond to the ND Neighbor Solicitation message with the Virtual Router MAC address for the Virtual Router." | §8.2.2 | VRRP |
| RFC3748-2.3-1 | dropped | "Unless the authenticator implements one or more authentication methods locally which support the authenticator role, the EAP method layer header fields (Type, Type-Data) are not examined as part of the forwarding decision." | §2.3 | IKE/EAP |
| RFC3948-4-1 | retire | No RFC 3948 sentence says interval MUST be shorter than NAT binding timeout; quote is the SHOULD-send-after-M-seconds obligation. | none | IKE/EAP |
| RFC4301-4.4.1-6 | dropped | "- SPD-I: For inbound traffic that is to be bypassed or discarded ... - SPD-O: For outbound traffic ... - SPD-S: For traffic that is to be protected using IPsec" | §4.4.1 | IKE/EAP |
| RFC4303-3.4.4.1-1 | dropped | "If the default padding scheme (see Section 2.4) has been employed, the receiver SHOULD inspect the Padding field before removing the padding prior to passing the decrypted data to the next layer." | §3.4.4.1 | IKE/EAP |
| RFC4555-3.9-1 | dropped | "The notification data contains the IP addresses and ports from/to which the packet was sent." | §4.2.6 | IKE/EAP |
| RFC2865-1.1-2 | dropped | "A NAS is not required to implement all of these service types, and MUST treat unknown or unsupported Service-Types as though an Access-Reject had been received instead." | §5.6 | access |
| RFC2865-5-8 | dropped | "Strings of length zero (0) MUST NOT be sent; omit the entire attribute instead." | §5 | access |
| RFC2869-x-5 | retire | No RFC 2869 sentence requires computing from byte count; quote only defines the counter. | none | access |
| RFC3579-3.2-1 | dropped | "The Message-Authenticator is calculated and inserted in the packet before the Response Authenticator is calculated." | §3.2 | access |
| RFC8907-x-1 | dropped | "For example, a server MUST be configured to time out a Single Connection Mode TCP connection after a specific period of inactivity to preserve its resources." | §4.3 | access |
| RFC1195-1.4-1 | dropped | "In a dual area within a dual routing domain only dual routers may be used." | §1.4 | routing |
| RFC2205-3-12 | dropped | "Therefore, its IP destination address must be the session DestAddress, and its IP source address must be the sender address from the path state being torn down." | §3.1.5 | routing |
| RFC2205-3-34 | dropped | "When Path and PathTear messages are forwarded, path state marked "Local_Only" must be ignored." | §3.9 | routing |
| RFC2966-2-1 | dropped | "The bit must be set to zero for all other IP prefixes in L1 or L2 LSPs." | §2 | routing |
| RFC3031-3.14-1 | retire | No RFC 3031 sentence says merged labels map to one egress point; quote is the distinct same-label-two-FECs ban. | none | routing |
| RFC3209-4.4.3-4 | dropped | "A received Path message without an RRO indicates that the sender node no longer needs route recording." | §4.4.3 | routing |
| RFC4090-4.2-1 | dropped | "This PathErr SHOULD be generated as specified in [RSVP] for unknown objects with a Class-Num of the form "0bbbbbbb"." | §4.2 | routing |
| RFC4090-6.5-3 | dropped | "- Global revertive mode: The head-end LSR of each tunnel is responsible for reoptimizing the TE LSPs that used the failed resource." | §6.5.2 | routing |
| RFC1350-2-3 | moved | stated by RFC 1123 4.2.3.1, not by this document. Sorcerer's Apprentice fix (do not resend on duplicate ACK) is RFC 1123; RFC 1350 only cites it. rfc1123.txt not in rfc/full. | RFC 1123 4.2.3.1 | services |
| RFC2131-4.1-7 | dropped | "The 'file' field MUST be interpreted next (if the 'option overload' option indicates that the 'file' field contains DHCP options), followed by the 'sname' field." | §4.1 | services |
| RFC2181-10.3-1 | dropped | "It can also have other RRs, but never a CNAME RR." | §10.3 | services |
| RFC2473-4.1.1-2 | dropped | "The limit value in the encapsulating option is set to one less than the limit value found in the packet being encapsulated." | §4.1.1 | services |
| RFC4035-2.2-1 | dropped | "o The RRSIG Algorithm, Signer's Name, and Key Tag fields identify a zone key DNSKEY record at the zone apex." | §2.2 | services |
| RFC4035-3.1.1-4 | dropped | "If space does not permit inclusion of the DS or NSEC RRset and associated RRSIG RRs, the name server MUST set the TC bit (see Section 3.1.1)." | §3.1.4 | services |
| RFC4035-4.3-1 | dropped | "More precisely, a security-aware resolver must be able to distinguish between four cases:" | §4.3 | services |
| RFC4213-3.6-1 | dropped | "This is done by verifying that the source address is the IPv4 address of the encapsulator, as configured on the decapsulator." | §3.6 | services |
| RFC7011-8-3 | dropped | "Template Withdrawals (Section 8.1) MUST NOT be sent by Exporting Processes exporting via UDP and MUST be ignored by Collecting Processes collecting via UDP." | §8.4 | services |
| RFC7011-x-1 | retire | RFC never mandates short form for 0-254; Section 7 says length may also use 3 octets. Quote is descriptive, carries no obligation. | none | services |
| RFC7871-7.1.2-3 | dropped | "If an Intermediate Nameserver receives a query with SOURCE PREFIX-LENGTH set to 0, it MUST NOT include client address information in queries made to resolve that client's request (see Section 7.1.2)." | §7.5 | services |
| RFC7950-5.1-1 | dropped | "A submodule MUST only be included by either the module to which it belongs or another submodule that belongs to that module." | §7.2.2 | services |
| RFC7950-7.21.5-1 | dropped | "If the XPath expression references any node that also has associated "when" statements, those "when" expressions MUST be evaluated first. There MUST NOT be any circular dependencies among "when" expressions." | §7.21.5 | services |
| RFC7950-7.3-1 | dropped | "The "type" statement, which MUST be present, defines the base type from which this type is derived." | §7.3.2 | services |
| RFC7950-7.9.2-1 | dropped | "The case identifier MUST be unique within a choice." | §7.9.2 | services |
| RFC7950-9.2.4-1 | dropped | "If a length restriction is applied to a type that is already length-restricted, the new restriction MUST be equally limiting or more limiting, i.e., raising the lower bounds, reducing the upper bounds, removing explicit length values or ranges, or splitting ranges into multiple ranges with intermediate gaps." | §9.4.4 | services |
| SFLOW-V5-x-15 | retire | No spec sentence says counters are cumulative since boot; quote is descriptive text about lost counter samples. | none | services |

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
| level with no BCP 14 keyword | RFC7296-1.2-1, 2.6-1, 2.9-1, 2.23-3, 2.23-12, 2.4-1, 1.4-1, 2.8-2 (MUST NOT), 2.2-3; RFC4301-4.1-4, 7-1 (MUST NOT); RFC3748-4-2, 2-2, 4.2-1; RFC2759-x-3, x-10, x-12 (format and vector text); RFC3768-6.4.3-9 (pseudocode); RFC5301-3-8 (MUST NOT over "The string is not null-terminated."); RFC8050-4.2-1 (descriptive); RFC8050-x-4 (rationale) | the quote carries no BCP 14 keyword. SB-2 counted 17 of them in its four stems, the blind samples 4 more | none for the level: D-3 of the source spec fixes it, "A row backed only by prose without an RFC 2119 keyword, or by a lowercase "must", is quoted verbatim and keeps its level. The level changes only when the sentence carries a different 2119 keyword." D-3 itself raises no review and no question, and R5 of `plan/records/rfc-verdict-test-fix-pass-rulings.md` keeps the format and vector rows at MUST under D-3. Three rows moved anyway, each by an owner decision of 2026-10-11 with its correction paragraph: `RFC7296-1.4-1` MUST to SHOULD (`rfc/corrections/rfc7296.md`, "Correction 2026-10-11"), and `RFC2759-x-10` and `RFC2759-x-12` retired as example data (`rfc/corrections/rfc2759.md`, two "Retired 2026-10-11" paragraphs). Corrected 2026-11-14: this cell said D-3 only permitted the level and asked for a review for demotion |
| lowercase keyword | RFC8092-4-1 ("should") | the quote carries a lowercase "should" | D-3 names only a lowercase "must": "A row backed only by prose without an RFC 2119 keyword, or by a lowercase "must", is quoted verbatim and keeps its level." It says nothing of a lowercase "should", and R5 of `plan/records/rfc-verdict-test-fix-pass-rulings.md` leaves the comparable RFC5443-4-1 to the judge, so this row's level is the judge's call (corrected 2026-11-14 from "review the level") |
| duplicate span | RFC7296-3.3.2-1 and RFC7296-3.3.6-1 | both now quote the same §3.3.3 span ("MUST understand all types" through the IKE line of the table); 3.3.6-1 is the D-H subset of 3.3.2-1 | merge, moving the tags of 3.3.6-1 |
| duplicate span | RFC8210-7-8 and RFC8210-5.2-1 | the §7 "The router MUST ignore any Serial Notify PDUs ... during this initial startup period" is the obligation 5.2-1 states, and 5.2-1 already cites §7 | merge |
| duplicate span | RFC3748-7.10-1 and 7.10-2 | 7.10-2 quotes a sub-span of 7.10-1's sentence; the tags of each cover a different half, so both are weak | merge, keeping every tag |
| duplicate span | RFC3748-4.1-11 against RFC3748-4.1-5 and RFC3748-2.1-3 | its first sentence duplicates 4.1-5, its Nak-after-non-Nak sentence duplicates 2.1-3 | merge into the two rows |
| id names another section | RFC5880-4.1-1 | the id names §4.1, but the quote and cite are the §6.8.7 transmit rule; no obligation is lost (the receive half is RFC5880-6.8.6-1), and an id is permanent | a dated correction saying why the id stands |
| text after the cite, or a wrapped hyphen | RFC3630-1-1 (prose after the section cite); DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-1 ("strict- mode"), RFC7684-5-1 ("sub- TLV") | the prose is not RFC text, and the hyphen split is the RFC's line wrap carried into the quote | move the prose out of the quote; join the hyphen |

Mistagged units, whose tag names a row the unit does not prove:

| Unit | Tagged under | What it proves instead | Correction owed |
|------|--------------|------------------------|-----------------|
| `internal/component/ike/engine/rfc7296_child_rekey_initiator_answer_test.go::TestChildRekeyAnswerWithoutTrafficSelectorsIsRefused`, and `rekey_test.go::TestRekeyWithoutTrafficSelectorsIsRefused` | RFC7296-2.9-1 | TS payload presence in a rekey answer, a neighbouring obligation; the verdict stays `enforced` on the other units | move the tags to the row that states the TS presence rule, or add it |
| `internal/component/ike/engine/rfc4301_spd_discard_test.go::TestSPDPolicyMirrorsTheInboundSelector` | RFC4301-4.4.1-4 | direction mirroring, not administrator ordering; the verdict stays `enforced` on the ordering units | move the tag |
| `internal/component/bgp/plugins/rpki/rtr_session_test.go::TestCacheResetTriggersResetQuery` | RFC8210-8.3-1 | the tag prose says "ze runs every configured cache in parallel", but `cacheGroup` is preference-ordered since 2026-09-20 | correct the tag prose, and re-judge the unit against the more-preferred-cache SHOULD of the quote |

### Deferred items the tables above missed (found 2026-09-28)

Re-read of the closed source spec against this spec; each confirmed in HEAD.

| Item | What is owed | Group |
|------|--------------|-------|
| RFC7296-2.4-2 | `weak` with an empty `tests` map (the note names `newDPDState`): the one verdict with no tagged test, so the 2026-09-27 claim "every one names at least one tagged test" was false for it | parent |
| RFC9552-5.1-2 and `nlri/ls/types_descriptor.go` | the verdict is `wrong` (the units assert the forbidden repeated 518 encoding), and the comment claiming 518 is "the only sub-TLV a descriptor can repeat" is false per RFC 9514 Section 6 | BGP |
| `TestRFC1035_RecordTTLIsA32BitUnsignedSecondCount` (`internal/plugins/geodns/rfc1035_rr_test.go`) | the row RFC1035-4.1.3-1 is `enforced`, but the comment quotes §3.2.1, not the §4.1.3 sentence | services |
| R-7 detail lost in condensing | RFC1035-4.1.4-1: the negative's comment claims "two zero bits" but asserts only "no 0xC0 octet". RFC9190-5.4-3: the claim that crypto/tls turns the error into bad_certificate is not asserted. RFC9190-5.7-1: `TestEAPTLS13CompletesAResumptionWithAnUnrevokedChain` proves the valid-cache positive but carries only the 5.7-2 and 5.7-6 tags. DRAFT-8210BIS-7-2: the guard is `hdr.Type != pduErrorRpt` in `RTRSession.readLoop`. DRAFT-8210BIS-5.12-1: the Error Code 9 mapping is in `readLoop` | services, IKE/EAP, BGP |

### Narrowing set, measured 2026-09-28

5448 rows over 190 stems differ between their pre-quote text and HEAD (baseline
per A-1: `fa7ac196df` for the 16 early re-quotes, `0bf0696576^` for the rest).
458 differ only in punctuation, case or quote marks, and 251 belong to rfc6514
(201) and rfc8362 (50). 4816 rows are left to compare. The largest stems are
rfc4271 130, rfc4035 118, rfc5880 118, rfc9830 111, rfc7432 110, rfc1661 100 and
rfc2131 100. 81 baseline rows are gone from HEAD, most of them D-10 moves and
retirements (rfc9582 13, rfc7011 7), and each is checked for a lost obligation too.

### Other open specs that own or touch these verdicts

| Spec | Status | Overlap | Consequence for a child |
|------|--------|---------|-------------------------|
| the 12 code-defect specs below | immediate | they own weak or wrong verdicts in BGP, VRRP, access, IKE/EAP, OSPF, routing and services | a child cannot reach zero until the owning spec lands; AC-5 |
| `plan/release/validation/spec-ipsec-rfc9190.md` | in-progress | owns R-7 rows RFC9190-1-1, 2.1.8-2 to 2.1.8-5, 5.4-1 to 5.4-3, 5.7-5 | the IKE/EAP child leaves these rows to it |
| `plan/proposals/spec-fixit-dns-rfc1035-conformance.md` | blocked | owns R-7 rows RFC1035-2.3.4-1, 4.1.1-1, 4.1.4-5, 4.2.2-1 | the services child leaves these rows to it, or the owner unblocks it |
| `plan/release/fixes/spec-ike-dpd-demand-driven.md` | skeleton | RFC7296-2.4-2, the verdict with no test | the parent waits for it |
| `plan/release/validation/spec-rfc-evidence-strength-1-targeted-mutant-ratchet.md` | design | once it lands, a record on a mutatable unit carrier MUST take the `mutant` route | every record this pass writes after that date follows its route |
| `plan/release/validation/spec-rfc-evidence-strength-2-revert-upgrade-burndown.md` | skeleton | rewrites the same `rfc/discrimination/<stem>.json` files; its R-1 expects the upgrade to expose more weak tests | the two passes never run on one stem at once |
| `plan/release/validation/spec-rfc-requirement-reattribution.md` | skeleton | the rfc9582 to 8210bis moves overlap the missing-row item for 8210bis 5.12-2 and 5.12-7 | the BGP child adds those rows only through the route that spec settles |

### Code defects recorded by the stem agents

This spec declares no code defect. Each defect the stem agents recorded in their
`findings.tsv` files was confirmed at its producer in HEAD on 2026-09-27 and now
lives in exactly one spec, which carries its RFC quote, producer, verdict and
owner decisions. D-8 still holds: a verified defect is fixed in code, by that
spec.

| Spec | Rows it owns |
|------|--------------|
| `spec-bgp-prefix-sid-rfc-defects` (closed 2026-10-20) | RFC8669-6-1, RFC9252-3.4-1, RFC9252-3.2.1-3, RFC9252-5-1, RFC9252-7-1 |
| `plan/release/fixes/spec-bgp-graceful-restart-rfc-defects.md` | RFC4724-4-2, RFC9494-4.2-8, RFC9494-4.3-1, RFC9494-4.3-3, RFC9494-4.2-6 with 4.3-2 |
| spec-bgp-open-session-rfc-defects (closed 2026-10-21) | RFC9072-2-1, RFC6286-2.1-1, and two Ze behaviour defects with journal rows |
| `spec-bgp-update-propagation-rfc-defects` (closed 2026-10-23) | RFC4271-4.3-3 and 4.3-4, RFC8092-3-1, MUP 3.1.3.1-6/-7/-10, RFC9234-3.1-1, RFC7999-3.1-2, RFC 2545 Section 3 with LINKLOCAL-CAPABILITY-4-2 |
| `spec-bgp-sr-policy-rfc-defects` (closed 2026-10-21) | RFC 9830 Section 2.4.2 flags and BSID label, RFC9830-4.2.1-2 |
| `plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md` | RFC 7854 Section 4.9 reason 4, SFLOW-V5-x-11 |
| `plan/release/fixes/spec-vrrp-owner-arp-rfc-defects.md` | RFC 9568 Section 8.1.2, and the vpp verifier parity journal row |
| `spec-radius-rfc-defects` (closed 2026-10-20) | RFC3579-3.3-2, RFC5176-2.3-2, RFC 2866 Section 5.5 |
| `spec-ike-eap-rfc-defects` (closed 2026-10-21) | RFC3948-2.1-2, RFC5216-2.1.1-4 |
| `plan/release/fixes/spec-ospf-nssa-translator-reachability.md` | RFC3101-3.2-2, the reachability clause |
| `plan/release/fixes/spec-rsvpte-frr-link-protection-fallback.md` | RFC 4090 Section 6 (no row), RFC4090-6-6 |
| `plan/release/fixes/spec-mcp-protected-resource-metadata-path.md` | RFC9728-3.1-3 |

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
| RFC6286-2.1-1 per-peer router-id | not dropped: confirmed at `parsePeerSettings`, and owned by spec-bgp-open-session-rfc-defects (closed 2026-10-21) D4 as an owner decision |

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - the discrimination record, the row quote, the refusals
  → Constraint: every row stays a verbatim span of its cited section or its subsections, at least 24 characters, never across two sections (`checkRowQuotes`), so a widened span that reaches the next section is refused and the row must be split instead
  → Constraint: `requirement_sha` hashes the quote plus the section parenthetical with markers peeled, not the level: widening, narrowing or re-citing a row stales its verdict (`stale-requirement`) until it is re-judged in the same commit; re-levelling leaves it fresh but it is re-judged by hand when the meaning moved
  → Constraint: a new id is `<Prefix>-<section>-<n>` with n above the HEAD^ high-water mark for that section (`checkIDAllocation`), permanent, never reused; two sessions adding rows to one section of one stem collide, so one child owns each stem
  → Constraint: a new gated row on an enrolled stem lands with both polarity tags or an annotation (`{single-polarity}`, `{gap}` ...), a discrimination record for each new cover (a moved tag is a new cover), and a site in `rfc/extraction/<stem>.json` mapped to it or listed in `unsourced-ids`; a split row whose sentence site maps to the old row needs that site remapped. A verdict is not required at once
  → Constraint: an annotation never replaces a tag a row held at HEAD (`checkCoverageRatchet`), `{single-polarity}` is refused while the other polarity's tag exists, and `{lower-layer}`, `{rollup}` and `{not-applicable}` bar tags; so an implementation gap inside a tagged row (RFC9190-2.3-1 Method-Id) is SPLIT into a `{gap}` row, verdict `unimplemented`, and the old row keeps its tests
  → Constraint: a correction demoting out of MUST, MUST NOT, SHALL or REQUIRED is a `Correction <YYYY-MM-DD>:` paragraph naming the id in backticks and double-quoting at least 24 characters verbatim from the RFC; a retirement is `Retired <YYYY-MM-DD>:` with the id as the first backticked id and at least one `§<n>`; no gate reads any other paragraph
  → Constraint: a producer change (a D-8 fix, or another spec's) stales every record naming that producer at that commit, in any stem, and the committing child re-records them (`./le rfc discriminate stem <s>` lists them); a `no-break` escape is refused for a producer in a gomu-mutated file, and rewording a tag's prose changes `claim-sha` and owes a re-record
  → Constraint: `discriminate-record` picks host or QEMU guest by `go/build` over the file (`unitNeedsGuest`); a guest unit needs `kernel <vmlinuz>` from `ze appliance kernel`. A host-compilable `_linux_test.go` that skips without privileges stays green under the break and cannot be recorded; the interop `revert` route writes the break into the shared working tree, so it never runs while another child builds
- [ ] `ai/skills/ze-rfc-audit.md` - the four judgement questions and the verdict vocabulary; STRICTNESS
  → Constraint: an `upgrade_reason` is owed for any weak or wrong to enforced move with no unit change (`checkAuditFindings`); an author's test commit followed by the auditor's upgrade commit therefore needs it, so author and auditor share one commit where they can
  → Constraint: `enforced` needs a non-empty `tests` map, both polarities or `{single-polarity}`, and a note naming an identifier of 5 or more characters found in the tagged unit (`checkAuditNote`); a new `wrong` verdict needs the public row in `docs/features/rfc-status.md` to disclose non-support first
  → Constraint: `./le rfc audit-stamp` refuses an id that already carries a verdict (`stampRefusal`: "Re-judging a recorded verdict is /ze-rfc-audit's work, not a stamp") and refuses `upgrade_reason` in a pending file, and `reseal` re-stamps only `shifted` verdicts. No command re-judges a recorded verdict today, and every one of the 1014 targets has one
  → Constraint: `audit-stamp` refuses a stem that is not enrolled; draft-ietf-sidrops-8210bis, rfc1035 and rfc9190 are backlog and rfc8362 is out of scope, so the R-7 re-judgements are recorded in this spec's table (AC-8)
  → Constraint: editing a tagged unit stales its verdict (`stale-unit`), and editing any function in a test file moves the file sha of every verdict tagging that file (`shifted`, fixed by `./le rfc reseal`); `reseal` is corpus-wide and rewrites another child's audit files too
- [ ] `internal/le/hookruntime/writeedit.go` `writeWeakening`, `internal/le/commit/rfcchange.go` - the RFC test-change gate
  → Constraint: an edit that changes the behavior of an RFC-tagged unit, or removes or moves a tag, is blocked at edit time, and the commit needs an `RFC-approved:` trailer from `./le rfc approve unit <pkg>.<Test> reason "..."`, recorded per commit session; a brand-new test function needs none (`ChangedTags` returns nil when the old unit had no tag)
- [ ] `spec-rfc-requirement-quote-hand-backfill` - D-2, D-3, D-6, D-7, D-8, D-9, D-10, D-15 and the stem brief (closed at `f265152e15`; its full text is the deletion in `git show f265152e15 -- plan/pre-release/`, the only file that commit removed there; no learned summary exists, and the stem brief was never committed)
  → Constraint (D-2): a retired row loses its tags first (moved, or deleted where the row was its only claim), leaves extraction and audit, gets a dated `Retired` paragraph in `rfc/corrections/<stem>.md` naming the sections read and where each tag went, and its id is never reused
  → Constraint (D-3): a row over prose without a BCP 14 keyword, or a lowercase "must", keeps its level; the level changes only for a different keyword, and a demotion from MUST owes a dated correction paragraph. D-3 itself raises no review and no question over the "level with no BCP 14 keyword" rows (R5 of `plan/records/rfc-verdict-test-fix-pass-rulings.md`); the three that moved, RFC7296-1.4-1, RFC2759-x-10 and RFC2759-x-12, moved by owner decision of 2026-10-11, each with its correction paragraph (corrected 2026-11-14)
  → Decision (D-6): one agent per package, never two on one test file; a verdict enters only through `./le rfc audit-stamp stem <stem>` from a pending file outside `rfc/audit/`, never with a hand-computed sha
  → Constraint (D-7): a row whose obligation another document already rows is retired and its tags move there; a move the gates refuse goes to the owner; stop and report when more than 15% of a stem of 20 or more rows is retired, report each retirement in a smaller stem
  → Decision (D-8): a verified defect is fixed in code, failing test first, then interop where a peer exists; never a `{gap}` in its place; an impossible fix goes to the owner. Defects the 12 `plan/immediate/` specs own are fixed there (AC-5)
  → Constraint (D-9): judge strictly; every re-judged verdict has a reader other than its author, and the blind sample floor is 20% of a stem's verdicts
  → Decision (D-10): an obligation that belongs to another summarized document gets a row there and the tags move; with no summary the row is retired, the correction names the document, and a journal row asks for its enrolment
  → Constraint (D-14): RFC5880-6.7.3-12 seeds bfd.RcvAuthSeq before the digest check by owner decision; the BFD child MUST NOT reverse it, and RFC5880-6.8.1-13 stays a gap
  → Constraint (D-4, D-11, D-12, D-13, D-1/D-5a): the judge is never the author and no tool proposes a sentence; an erratum is quoted verbatim (RFC9568-7.1-4 under erratum 8298); RFC 3101 translator equivalence is same mask, metric and non-zero forwarding address; softver `legacy` is an owner-approved deviation, not a defect; rfc905 cites §B.N
  → Constraint (mistake log): a report or commit states a check ran only when the writing step verified it; a tag count is derived from code and `.ci` files, never from plan prose; a row edit can redden the real-corpus count tests in `internal/le/rfc`; the bgp/reactor link-local tests fail without `fd00::2` on loopback (journal row, not an RFC 2545 regression)
- [ ] `plan/release/validation/spec-rfc-requirement-reattribution.md` - a re-attribution the gates refuse today
  → Constraint: that spec (2026-09-01, skeleton) is stale in two places: `validateID` no longer checks the section, and `checkRetiredRequirements` now accepts `Retired`. The route rfc9582 used is today's route: move the tags to the destination row, drop the id from extraction, audit and discrimination, delete the row, add the `Retired` paragraph (`rfc/corrections/rfc9582.md`)
  → Constraint: still refused: an annotation over a row that had tags at HEAD, un-enrolment, and rewriting a row's text to another document's sentence under the old id (the quote is not in that RFC)

**Key insights:**
- The work is 1014 verdicts plus a 4816-row narrowing comparison plus about 120 split, missing and row-quality items, split into a parent and eight children (Split section)
- Ledger files are per stem, test files per package: a child owns stems, and five test files are edited by one child at a time
- Two gate facts shape every child: no command re-judges a recorded verdict, and every edit to a tagged unit needs an owner approval record
- A child's closure is bounded by other specs: 12 code-defect specs, `spec-ipsec-rfc9190`, `spec-fixit-dns-rfc1035-conformance`, `spec-ike-dpd-demand-driven`
- `./le rfc check` is red at HEAD from other sessions; a child answers only for the violations it adds

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/audit_stamp.go` (297L) - `auditStamp` loads the stem's audit file, runs `stampRefusal` on every pending entry (all or nothing), fills fingerprints with `stampFingerprints`, appends the entries and renames the file in through `replaceAudit`. `stampRefusal` refuses first when the id already has a verdict; `authoredVerdictKeys` admits only `verdict`, `note`, `code`, `no_code_path`
  → Constraint: the re-judge mode reuses `stampRefusal`'s vocabulary, tag and not-applicable refusals and `stampFingerprints` unchanged; only the "already judged" test inverts, and `upgrade_reason` joins the authored keys in that mode alone
  → Constraint: a re-judged entry keeps its position in `audit.Order` (replace in place, not append), so the diff of a re-judge shows one entry changed, never a deletion and an addition
- [ ] `internal/le/rfc/check_audit.go` `checkAuditFindings` - refuses a deleted weak or wrong verdict, and a weak or wrong to enforced move whose `units` equal HEAD^'s with no `upgrade_reason`
  → Constraint: the stamp's early refusal of an unchanged-units upgrade and the gate's check are one predicate, extracted from `checkAuditFindings` and called by both (`ai/rules/principles.md`: one declaration)
- [ ] `internal/le/rfc/actions.go` - the `audit-stamp` action declares `stem` and `from`, both required; `auditStampAnswer` calls `auditStamp`
  → Constraint: the mode is an optional parameter `mode new|rejudge`, default `new`, the same shape as the `mode full|changed` parameter of the verify actions (`internal/le/verify/actions.go`)
- [ ] `internal/le/rfc/audit_stamp_test.go` - `TestAuditStampFillsNewVerdictsAndTheyReadFresh`, `TestAuditStampLeavesARecordedVerdictUntouched`, `TestAuditStampRefusesAndWritesNothing`; `internal/le/rfc/actions_test.go` drives a verb through `Answer`
  → Constraint: `TestAuditStampLeavesARecordedVerdictUntouched` stays as the proof that the default mode is unchanged
- [ ] `ai/skills/ze-rfc-audit.md` - the verdict rules this pass applies, and the one place an author learns the stamp route

**Behavior to preserve:**
- a commit of this pass adds no `./le rfc check` violation of its own: a weak or wrong verdict is never deleted, never upgraded without a unit change or an `upgrade_reason`
- `./le rfc check` is red at HEAD on 2026-09-28 with 18 `producer-changed` records (rfc4271 6.3-15, rfc7611 2.1-1, rfc7705 3.3-1, rfc7947 2.2.2.2-1, rfc8907 x11) from other sessions, journaled in `plan/journal/concurrent-rfc-gate-stale.md`; a child judges its commit by the violations it adds, not by a green total

**Behavior to change (parent only; the children own the test, row and D-8 changes):**
- `./le rfc audit-stamp` gains `mode rejudge`: every pending id MUST already carry a verdict, the entry replaces it in place with fresh fingerprints, and `upgrade_reason` is accepted, required for a weak or wrong to enforced move whose units are unchanged, and refused on any other move
- `./le rfc audit-stamp` without `mode`, or with `mode new`, behaves as today

## Existing Mechanisms (MANDATORY)

A record of the mechanisms this spec's work extended or used, each read at its producer on 2026-11-14. It adds no design decision.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The stamp writer of `rfc/audit/` | `internal/le/rfc/audit_stamp.go`: `auditStamp`, `auditStampSource`, `stampRefusal`, `rejudgeRefusal`, `stampFingerprints`, `parseStampMode`, `rejudgeVerdictKeys` | `parseStampMode` reads `mode new` or `mode rejudge`, defaults to `new` and refuses any other word. `stampRefusal` refuses a judged id in `new` mode and an unjudged id in `rejudge` mode, and admits `upgrade_reason` only through `rejudgeVerdictKeys`. `rejudgeRefusal` accepts the reason on any `weak`, `wrong` or `partial` verdict raised to `enforced` (`findingUpgrade`), requires it there when every tagged unit is byte-identical (`upgradeOverUnchangedUnits`), and refuses it on any move that is not such an upgrade. With `source head`, `auditStampSource` stamps from immutable HEAD and refuses a `partial` verdict or a row annotated partial, because its quotation and discrimination checks need the working source. `stampFingerprints` recomputes every fingerprint | extends it: Behavior to change, "`./le rfc audit-stamp` gains `mode rejudge`: every pending id MUST already carry a verdict, the entry replaces it in place with fresh fingerprints, and `upgrade_reason` is accepted"; Current Behavior, "only the 'already judged' test inverts". Built in `2d078935e3` (2026-09-28) |
| The audit findings ratchet | `internal/le/rfc/check_audit.go`: `checkAuditFindings`, `findingUpgrade`, `upgradeOverUnchangedUnits`, `upgradeReasonGiven` | against the baseline, refuses a deleted `weak`, `wrong` or `partial` verdict and an upgrade to `enforced` over unchanged units with no `upgrade_reason`; `upgradeOverUnchangedUnits` is the one predicate, and `rejudgeRefusal` calls it too | extends it: Current Behavior, "the stamp's early refusal of an unchanged-units upgrade and the gate's check are one predicate, extracted from `checkAuditFindings` and called by both". Extracted in `2d078935e3` |
| The RFC test-change approval | `internal/le/rfc/approve.go`: `ApprovalPath`, `ApproveCommand`, `ApprovalTrailerLine`; the `approve` action in `internal/le/rfc/actions.go`; `internal/le/commit/rfcchange.go` | `./le rfc approve unit <pkg>.<Test> reason "<the judge's verdict>"` writes one row to `tmp/commit-rfc-approved-<session>.md`, which the edit hook and `./le commit create` read; the commit carries it as an `RFC-approved:` line. Since `e23a116783` (2026-10-26) the independent `ze-test-judge` agent runs it, and the agent that made the change never does | uses it: P-1, "A child records `./le rfc approve unit <pkg>.<Test> reason "D-15: ..."` itself". The owner's later rule (`ai/META-DECISIONS.md`, "Test changes") moved the recording to the judge, and the children's commits after 2026-10-26 follow it; the spec's design did not change the mechanism |
| An optional enum parameter of a `./le` action | `internal/le/rfc/actions.go`: the `audit-stamp` action, `keyMode` declared as `leaction.KindEnum`, `Requirement: leaction.Optional`, with `ValueHelp` per word | the action table publishes `mode` with its two words, so help and completion carry them | extends it: Current Behavior, "the mode is an optional parameter `mode new\|rejudge`, default `new`". That constraint compared it with a `mode full\|changed` parameter of `internal/le/verify/actions.go`; the verify actions now take `full` and `fast` switches instead (read 2026-11-14), so the comparison is historical |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le rfc audit-stamp stem <stem> from <pending> mode rejudge`, typed by a child's judging agent
- `./le rfc check`, reading `rfc/short/`, `rfc/audit/`, `rfc/discrimination/` and the tags in test files

### Transformation Path
1. The child's author agent changes a tagged test (approval recorded under P-1), a row, or a producer
2. `./le rfc reseal` re-stamps the sibling verdicts the edit only shifted
3. `./le rfc record` records the observed red under a break for each added or changed cover
4. A judging agent that did not author the test writes the new verdict in a pending file in session scratch
5. `./le rfc audit-stamp ... mode rejudge` refuses or replaces, computing `requirement_sha`, `tests`, `units` and `code`
6. `./le rfc check` compares verdicts, records and rows against HEAD^

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| test file to ledger | the `RFC requirement:` tag, fingerprinted by `stampFingerprints` | `TestAuditStampRejudgeReplacesInPlaceAndReadsFresh` |
| pending file to audit file | `readPendingVerdicts`, then `replaceAudit` validating before rename | `TestAuditStampRejudgeRefusesAndWritesNothing` |
| stamp to commit gate | the shared unchanged-units upgrade predicate | `TestAuditStampRejudgeUpgradeNeedsAReasonWhenUnitsAreUnchanged`, `TestCheckAuditRatchetSeesTipCommit` |

### Integration Points
- `auditStampAnswer` (`actions.go`) reads the new `mode` argument and passes it to `auditStamp`
- `checkAuditFindings` (`check_audit.go`) calls the extracted upgrade predicate instead of its inline test

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | the re-judge writes through `replaceAudit`, the one writer of `rfc/audit/` (`docs/architecture/core-design.md`) |
| No duplicated functionality | Yes | one fingerprint path, one refusal function, one upgrade predicate shared with the gate |

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
| One child owns each stem's ledger files; a test file carrying two owners' tags is edited by one child at a time | the ledger files are rewritten whole, with no lock |
| A tagged-unit edit records `./le rfc approve unit ... reason "D-15: ..."` before the edit | P-1; the edit hook and the commit gate refuse it otherwise |
| A verdict is replaced only through `./le rfc audit-stamp ... mode rejudge` | P-2; a hand-deleted entry reads as a deleted finding |
| Author and judge land in one commit where they can | an upgrade in a later commit than the test change needs an `upgrade_reason` |
| A verdict a child cannot move without another spec's producer fix is listed as blocked by that spec and moved into its acceptance criteria | P-3 |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `0bf0696576^` holds the pre-quote text of every row the backfill and the hand pass changed | the commit message of `0bf0696576` names the check and the 1821-row backfill | the narrowing set misses rows quoted earlier | diff of `0bf0696576^` against its predecessor quote commits | BROKEN 2026-09-28: feature commits between 2026-09-21 and 2026-09-24 (`3f8dcc5fb5`, `23a10f500a`, `3ff5a6f056`, `6a07404d5d`) re-quoted 16 rows first. For those the baseline is `fa7ac196df` (2026-09-20). 8 of them never changed again, so `0bf0696576^` misses them: RFC3031-3.10-1, 3.14-1, 3.16-1; RFC3209-4.1-1, 4.1-2; RFC7311-3.2-1, 3.2-2; RFC9552-5.2.2-6. RFC3209-4.1-2 went from "Upper 12 bits of the LABEL value MUST be zero" to "Labels MAY be carried in Resv messages". The 16 were found by a verbatim-substring heuristic, so they are a floor |
| A-2 | the split-needed table above is the whole set the stem agents recorded | grep of every report's split sections and every findings.tsv, 2026-09-27 | a split is lost when the scratch directory goes | the narrowing audit re-derives it | validated 2026-09-28: the narrowing audit re-derived the split set; 81 new rows found |
| A-3 | D-15 is the owner's standing approval for every tagged-unit edit this pass makes, so a child records `./le rfc approve unit ... reason` itself | D-15 moved the test fixes here | every changed tagged unit, about a thousand, waits for Thomas one by one | owner answer at the research gate | confirmed 2026-09-28: P-1 ("D-15 is the owner's standing approval for every tagged-unit edit and tag move this pass makes ... A-3 is confirmed"). From 2026-10-26 (`e23a116783`) the owner's rule moved the recording to an independent `ze-test-judge` agent, so later children recorded judge approvals; no tagged-unit edit waited for Thomas either way |
| A-4 | the verdicts owned by the 12 code-defect specs, `spec-ipsec-rfc9190` and `spec-fixit-dns-rfc1035-conformance` are the only ones a test-only fix cannot reach | the overlap table; `{gap}` is refused on a tagged row | a child cannot close at zero | each child lists its blocked-by verdicts at its own research | broken 2026-11-14: of the 19 `weak` verdicts left at the last child's closure, only RFC3101-3.2-2, RFC4090-6-6, RFC9494-4.2-6, RFC9728-3.3-1 and SFLOW-V5-x-11 are owned by specs in that set. Of the other 14, eleven are owned by specs the list did not name and three were owned by no spec until `937c0bb8fb` homed them (Implementation Audit, AC-11), and the services child found three more during its closure (RFC7950-7.9.4-1, RFC2131-4.1-7 and RFC2131-4.1-15, its A-2). P-3 absorbed the miss: each child moved its blocked verdicts into the owning spec's criteria, so no child closure depended on the list being complete |
| A-5 | the 16 early re-quotes are all the rows quoted before `0bf0696576` | a verbatim-substring heuristic against `rfc/full/`, back to 2026-09-10 | the narrowing audit misses a dropped obligation | the parent's narrowing phase re-runs the comparison for every row against its oldest text since 2026-09-01 | superseded 2026-09-28: the narrowing audit compared every row's quote with its old claim (4816 substantive, 81 removed), so no finding depends on the 16-row floor. The residual miss rate is R-12 |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | two package agents touch the same test file through a shared helper | a commit carries another agent's hunk | assign by test file, not by directory, where helpers are shared |
| R-2 | a fix to a test reveals a defect that grows past the package | a code change outside the package | D-8: fix it; where impossible, back to the owner |
| R-3 | two children commit while one holds uncommitted verdicts or records in the other's stem file, and the first commit carries a stale foreign hunk | a commit diff shows another child's ids | one owner per stem file (the Split table); `reseal` and interop `revert` runs are serialized between children |
| R-4 | a re-judge written by hand-deleting an audit entry and re-stamping reads, in the diff, as the deletion the findings ratchet exists to catch | a reviewer or a later gate treats it as a deleted finding | settled by P-2: the re-judge mode replaces in place, so no deletion appears |
| R-8 | the re-judge mode lets an agent re-stamp a verdict it did not re-read | a re-judged note identical to the old one, or a new note naming nothing in the unit | the same exposure a first stamp has; `checkAuditNote`, the independent-judge rule and the 20% blind sample (D-9) apply |
| R-9 | narrowing agents flag cosmetic rewording or an over-claiming paraphrase as a dropped obligation | a split-needed row whose "dropped obligation" no RFC sentence states | each proposed split names the RFC sentence that states it, quoted; a claim with no sentence is a D-2 retirement or nothing |
| R-10 | retirements from the narrowing audit pass the D-7 15% stop on a stem | the per-stem retirement count | stop and report to the owner, as rfc9582 was reported |
| R-11 | a child starts row edits on a stem the narrowing audit has not covered | the child's commit touches `rfc/short/<stem>.md` before the narrowing output for that stem is committed | the narrowing audit commits its output per child group, and a child's row phase waits for its group |
| R-5 | the evidence-strength-1 mutant ratchet lands mid-pass and refuses `revert` records written earlier | `./le rfc check` names a revert record on a mutatable carrier | a child re-records on the `mutant` route; evidence-strength-2 never runs on a stem a child holds |
| R-6 | a D-8 fix in one child stales records in a stem another child owns | `producer-changed` in a stem the committing child does not own | the committing child re-records them in the same commit, per the producer-change constraint |
| R-7 | a narrowed row is retired or re-levelled by one child while the parent's narrowing audit compares it | the parent's comparison meets a row that changed since its baseline read | the narrowing audit runs first, before the children start on row edits |
| R-12 | about 20 dropped obligations stay undetected among the 3724 kept rows the blind sample did not re-read (new-miss rate about 0.4%) | a child's test author finds a row whose old text claims more than its current quote | the child adds the split row under AC-C3, with the RFC sentence quoted |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | the parent changes a development tool: a wrong re-judge mode could replace a verdict with a fresh-looking one nobody re-made. No operator sees it. The children's test-only changes break nothing an operator sees; their D-8 fixes change protocol behavior and carry their own tests and interop |
| How is it reverted? | the re-judge mode is one commit; the children revert one commit per package |
| Who else touches this path? | `spec-rfc-evidence-strength-1` and `-2` (the same discrimination files and record route), every session that runs `./le rfc audit-stamp` or `reseal` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc audit-stamp stem <stem> from <pending> mode rejudge` | → | `auditStampAnswer` → `auditStamp` in re-judge mode | `TestRFCActionsAuditStampRejudgeMode` (`internal/le/rfc/actions_test.go`) |
| `./le rfc check` | → | `checkAuditFindings` calling the shared upgrade predicate | `TestCheckAuditRatchetSeesTipCommit` (`internal/le/rfc/check_audit_baseline_test.go`, existing: it already asserts the "stayed byte-identical" refusal through the tip commit, so it proves the extraction changed nothing) |

## Acceptance Criteria

The parent's own criteria:

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le rfc audit-stamp ... mode rejudge` over a pending entry whose id carries a verdict | the verdict is replaced at the same position in the file, its fingerprints are recomputed, and `./le rfc check` reads it fresh |
| AC-2 | `mode rejudge` over a pending entry whose id carries no verdict | refused, naming the id, nothing written |
| AC-3 | `mode rejudge`, weak or wrong to enforced, tagged units byte-identical to the recorded ones | refused without `upgrade_reason`; stamped with it, and the reason survives in the audit file |
| AC-4 | `mode rejudge` with `upgrade_reason` on any move other than weak or wrong to enforced | refused, naming the id |
| AC-5 | `./le rfc audit-stamp` with no `mode` or `mode new`, and any `mode` value other than `new` or `rejudge` | today's behavior, including the refusal of an id that carries a verdict; an unknown mode is refused |
| AC-6 | the docs that describe `audit-stamp` (`ai/skills/ze-rfc-audit.md`, `docs/functional-tests.md`, `docs/contributing/rfc-implementation-guide.md`, `docs/contributing/rfc-conformance-gates.md`) and the action's own help | each names the re-judge mode and when it is used; none still says a recorded verdict can only be refused |
| AC-7 | the narrowing set: 4816 substantive rows, the 81 removed rows and the 8 early re-quotes | every row compared against its pre-quote text; every dropped obligation, quoted from the RFC with its section, is added to "Split-needed rows" with its child; every removed row names where its obligation lives now or why it had none |
| AC-8 | the eight children | each child spec exists in `plan/pre-release/`, status `ready`, naming its packages, its owned stems, its rows from every table here, its blocked-by verdicts with the spec that blocks each, and the P-3 closure rule, and inherits AC-C1 to AC-C7 |
| AC-9 | the five cross-group verdicts (RFC1071-1-4, RFC4303-2.1-1, RFC5882-4.4-1, RFC905-x-3, RFC905-x-4) | each resolved as the goal's first or second way and re-judged by an agent that did not write the test. Transfer, 2026-10-11 (owner decision): RFC5882-4.4-1 is owned by AC-24 of `spec-bgp-live-receive-binding-and-bfd-keep-session` (closed 2026-10-13) (Phase 4, step 1), which carries the `internal/component/bfd/engine/engine.go` fix (`joined` widened again when a pinned client releases), its tests, its record and its independent re-judge. This pass does not touch `engine.go`. The verdict stays weak until that spec lands; it is named in another spec's acceptance criteria, which is what AC-11 asks, so AC-9 is owed here for the other four ids only |
| AC-10 | RFC7296-2.4-2 | named in the acceptance criteria of `plan/release/fixes/spec-ike-dpd-demand-driven.md` |
| AC-11 | parent closure | every child is closed, and the derived weak-or-wrong listing over `rfc/audit/*.json` holds only verdicts named in another spec's acceptance criteria. RFC5882-4.4-1 meets this through AC-24 of `spec-bgp-live-receive-binding-and-bfd-keep-session` (closed 2026-10-13) (transfer, 2026-10-11, see AC-9) |

The criteria every child inherits:

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-C1 | the derived weak-or-wrong listing restricted to the child's packages and owned stems | holds only verdicts the child lists as blocked by a named spec, each named in that spec's acceptance criteria (P-3) |
| AC-C2 | every verdict the child moves to `enforced` | its tagged units carry a discrimination record, and an agent other than the test's author judged it |
| AC-C3 | every row of the child's in the split-needed and missing-rows tables | the dropped obligation is a row of its own, or a dated correction says why not, and the new row carries a verdict |
| AC-C4 | every spec in the code-defects pointer list | the child declares none of their defects and does not fix them; a test whose producer one of them changes waits for, or lands with, that spec |
| AC-C5 | `./le rfc check` after each child commit | no violation the child's commit added, and no stale verdict in the child's stems |
| AC-C6 | every row and unit of the child's in the row-quality and mistagged-unit tables | corrected, merged or re-tagged, or a dated correction says why it stands; a changed row or tag is re-judged |
| AC-C7 | every row of the child's in "Un-enrolled R-7 rows" | resolved as the goal's first or second way and re-judged `enforced` by an agent that did not write the test, recorded in that table with the date or in the stem's audit file; RFC9190-2.3-1's Method-Id gap is split into a `{gap}` row under `ai/rules/rfc-compliance.md` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAuditStampRejudgeReplacesInPlaceAndReadsFresh` | `internal/le/rfc/audit_stamp_test.go` | AC-1: a judged id is replaced at its position with recomputed fingerprints, and `./le rfc check`'s freshness reads it fresh | |
| `TestAuditStampRejudgeRefusesAndWritesNothing` | `internal/le/rfc/audit_stamp_test.go` | AC-2: an unjudged id in a re-judge file refuses the whole file, and the audit file is byte-identical after | |
| `TestAuditStampRejudgeUpgradeNeedsAReasonWhenUnitsAreUnchanged` | `internal/le/rfc/audit_stamp_test.go` | AC-3: weak to enforced over unchanged units is refused without `upgrade_reason` and stamped with it; the same move after a unit change needs none | |
| `TestAuditStampRejudgeRefusesAReasonOffAnUpgrade` | `internal/le/rfc/audit_stamp_test.go` | AC-4: `upgrade_reason` on enforced to weak, weak to weak and weak to wrong is refused | |
| `TestAuditStampLeavesARecordedVerdictUntouched` (existing) | `internal/le/rfc/audit_stamp_test.go` | AC-5: the default mode still refuses a judged id | |
| `TestAuditStampRefusesAReasonInNewMode` | `internal/le/rfc/audit_stamp_test.go` | AC-5: `upgrade_reason` stays refused in the default mode | |
| `TestRFCActionsAuditStampRejudgeMode` | `internal/le/rfc/actions_test.go` | wiring and AC-5: `mode rejudge` reaches the re-judge path through `Answer`, `mode new` and no mode reach the default, an unknown mode is refused with exit 2 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | the change takes no numeric input | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A for the parent | `./le` actions are Go-tested through `Answer` (`internal/le/rfc/actions_test.go`); there is no `.ci` harness for `./le` verbs | the wiring row covers the typed command | |

The children's functional and interop tests are per D-8 fix and are planned in each child.

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A for the parent | | | the parent changes no protocol behavior; each child plans interop for its D-8 fixes | |

## Files to Modify
- `internal/le/rfc/audit_stamp.go` - the re-judge mode in `auditStamp` and `stampRefusal`, `upgrade_reason` admitted in that mode, replace-in-place
- `internal/le/rfc/check_audit.go` - extract the unchanged-units upgrade predicate from `checkAuditFindings`
- `internal/le/rfc/actions.go` - the optional `mode new|rejudge` parameter on `audit-stamp`, its help text, `auditStampAnswer`
- `internal/le/rfc/audit_stamp_test.go`, `internal/le/rfc/actions_test.go` - the tests above
- `ai/skills/ze-rfc-audit.md` - the re-judge route, and when `upgrade_reason` is written
- `docs/functional-tests.md`, `docs/contributing/rfc-implementation-guide.md`, `docs/contributing/rfc-conformance-gates.md` - the sentences that say a recorded verdict is only refused
- `ai/INDEX.md` - the RFC audit verdict row names the re-judge mode
- `plan/release/fixes/spec-ike-dpd-demand-driven.md` - AC-10
- `plan/release/validation/spec-rfc-verdict-test-fix-pass.md` - the narrowing output (AC-7)
- the children's files (tests, `rfc/short/`, `rfc/audit/`, `rfc/discrimination/`, `rfc/corrections/`, `rfc/extraction/`, D-8 producers) are named in each child

## Files to Create
- the eight children (AC-8) under `plan/pre-release/`: spec-rfc-verdict-fix-bgp and `spec-rfc-verdict-fix-` `-bfd.md`, `-ospf.md`, `-vrrp.md`, `-ike-eap.md`, `-access.md`, `-routing.md`, `-services.md`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | a `./le` development verb; no YANG |
| YANG validation constraints | N-A | no YANG leaf |
| YANG custom validators | N-A | no YANG leaf |
| CLI commands/flags | Yes | `internal/le/rfc/actions.go`, the `mode` parameter of `audit-stamp` |
| CLI grammar (keyword before value) | Yes | `mode rejudge`, keyword before value, the verify actions' shape |
| Editor autocomplete | N-A | `./le` completion comes from the action table, which carries the parameter |
| Functional test for new RPC/API | N-A | no RPC; the action is tested through `Answer` |
| Pipe completeness | N-A | the answer is the existing `AuditStampReport`, unchanged in shape |
| Env var registration | N-A | no env var |
| Doctor check for runtime dependencies | N-A | no new path, socket, port or binary |
| Prometheus counters/metrics | N-A | a development tool |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP family |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | a development verb; no operator sees it |
| 2 | Config syntax changed? | No | no config |
| 3 | CLI command added/changed? | No | `docs/guide/command-reference.md` covers `ze`, not `./le`; the `./le` verb is documented in the pages in row 10 and 12 |
| 4 | API/RPC added/changed? | No | no API |
| 5 | Plugin added/changed? | No | no plugin |
| 6 | Has a user guide page? | No | no guide page for the RFC ledger tools |
| 7 | Wire format changed? | No | the parent changes no wire format; each child answers for its D-8 fixes |
| 8 | Plugin SDK/protocol changed? | No | no SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | No for the parent | the children update `rfc/short/<stem>.md` and the `docs/features/rfc-status.md` row as they prove or correct rows |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (the audit-stamp paragraph), `docs/contributing/rfc-conformance-gates.md` |
| 11 | Affects daemon comparison? | No | no product behavior |
| 12 | Internal architecture changed? | Yes | `docs/contributing/rfc-implementation-guide.md` (the audit-stamp step), `ai/skills/ze-rfc-audit.md`; `docs/architecture/core-design.md` names `audit_stamp.go` as the one writer of `rfc/audit/`, which stays true |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | an `./le` action parameter, not a `ze` command |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/functional-tests.md` anchors `internal/le/rfc/audit_stamp.go -- auditStamp` (updated, row 10); `docs/architecture/core-design.md` is the `// Design:` doc of `audit_stamp.go` and stays accurate (row 12); `docs/features.md` anchors `internal/le/rfc/actions.go -- Answer` on the interoperability-testing row, which the new parameter leaves unaffected |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | every `./le rfc audit-stamp stem <stem> from <path>` example stays valid; the re-judge example is added beside it |

### Discovery

| Question | Answer |
|----------|--------|
| Where does an agent look first? | `ai/INDEX.md`, the "RFC audit verdict" row, which names the re-judge mode |
| What rule prevents regression? | `ai/skills/ze-rfc-audit.md` names the re-judge route; the action's help text states it |
| What registry prevents drift? | the `./le rfc` action table (`internal/le/rfc/actions.go`) publishes the parameter |
| What verification proves it? | the unit tests above, and `./le rfc check` reading a re-judged verdict fresh |

## Implementation Steps

1. **Phase: Wiring** - the `mode` parameter on `audit-stamp`, reaching a stub re-judge path
   - Tests: `TestRFCActionsAuditStampRejudgeMode`
   - Files: `internal/le/rfc/actions.go`, `internal/le/rfc/audit_stamp.go`
   - Verify: the test fails on the stub, then reaches the path
2. **Phase: re-judge mode** - inverted "already judged" refusal, `upgrade_reason` admitted in the mode, the shared upgrade predicate, replace in place
   - Tests: the four `TestAuditStampRejudge*` tests, `TestAuditStampRefusesAReasonInNewMode`, then `TestCheckAuditRatchetSeesTipCommit` and `TestAuditStampLeavesARecordedVerdictUntouched` unchanged and green
   - Files: `audit_stamp.go`, `check_audit.go`
   - Verify: tests fail, implement, tests pass; the docs of AC-6 edited in the same phase
3. **Phase: children** - write the eight child specs from the Split section, the inventory tables and P-1 to P-3; AC-10 edit to the DPD spec
   - Verify: each child carries every field AC-8 lists; the union of their owned stems and packages covers the derived listing, less the parent's five and RFC7296-2.4-2
4. **Phase: narrowing audit** - agents in batches of about 200 rows, whole stems per batch, each comparing pre-quote text against HEAD in `rfc/full/<stem>.txt`; output committed per child group
   - Per-row output, one line each: id, call (`kept`: the quote carries every obligation the old text claimed; `dropped`: an obligation the old text claimed is stated by an RFC sentence the quote lacks; `retire`: the old text claimed something no RFC sentence states, D-2; `moved`: another document states it, D-10), the quoted RFC sentence for `dropped`, its section, and the child group
   - Every `dropped`, `retire` and `moved` call is re-read by a second agent that did not make it, against the RFC text, before it enters "Split-needed rows"; a disagreement stays out of the table and is listed for the owner
   - A `kept` call needs no second read; the 20% blind sample (D-9) is drawn from the `kept` calls of each batch
   - Verify: AC-7; the sum of rows over all batch outputs equals the narrowing set count
5. **Phase: parent verdicts** - the five cross-group verdicts, author and judge separate
   - Verify: AC-9 through the re-judge mode; `./le rfc check` adds no violation

### Deliverables Checklist

Added 2026-11-14 at closure, from the deliverables the phases above name.

| Deliverable | Verification method |
|-------------|---------------------|
| `mode new\|rejudge` on `./le rfc audit-stamp` (AC-1 to AC-5) | `grep -n 'stampModeRejudge\|rejudgeRefusal\|parseStampMode' internal/le/rfc/audit_stamp.go`; `./le test unit pkg path ./internal/le/rfc/ run 'TestAuditStamp\|TestRFCActionsAuditStampRejudgeMode\|TestCheckAuditRatchetSeesTipCommit'` |
| one unchanged-units upgrade predicate shared by the stamp and the gate | `grep -n 'upgradeOverUnchangedUnits' internal/le/rfc/check_audit.go internal/le/rfc/audit_stamp.go` names a call in `checkAuditFindings` and in `rejudgeRefusal` |
| the re-judge route in every page AC-6 names | `grep -n 'mode rejudge' ai/skills/ze-rfc-audit.md docs/functional-tests.md docs/contributing/rfc-implementation-guide.md docs/contributing/rfc-conformance-gates.md ai/INDEX.md internal/le/rfc/actions.go` |
| the narrowing audit output (AC-7) | the "Narrowing audit findings, 2026-09-28" section; `git show --stat 4c7f52aa06` |
| the eight children, each closed (AC-8, AC-11) | `git log -1 --format=%s <sha>` for each closing commit in the Depends row |
| the five cross-group verdicts (AC-9) | `grep '"verdict"' rfc/audit/rfc1071/RFC1071-1-4.json rfc/audit/rfc4303/RFC4303-2.1-1.json rfc/audit/rfc905/RFC905-x-3.json rfc/audit/rfc905/RFC905-x-4.json rfc/audit/rfc5882/RFC5882-4.4-1.json` |
| every remaining weak or wrong verdict named in an owning spec's acceptance criteria (AC-11) | `grep -rlE '"verdict": *"(weak\|wrong)"' rfc/audit`, then each id read in its owner's AC table |

### Security Review Checklist

Added 2026-11-14 at closure. The parent's own code is a development verb; it reads a pending file the author wrote and writes `rfc/audit/`.

| Check | What to look for |
|-------|-----------------|
| Input validation | a pending file with an unknown mode word, an unjudged id in `rejudge` mode, a computed field written by hand, or an `upgrade_reason` off an upgrade is refused whole, naming the id, with the audit file byte-identical after (`parseStampMode`, `stampRefusal`, `rejudgeRefusal`; `TestAuditStampRejudgeRefusesAndWritesNothing`, `TestAuditStampRejudgeRefusesAReasonOffAnUpgrade`) |
| Authorization that could fail open | a re-judge cannot turn a finding into proof with nothing changed: `rejudgeRefusal` and `checkAuditFindings` apply the one predicate `upgradeOverUnchangedUnits`, so the stamp refuses what the commit gate refuses |
| Path handling | `from` names a file outside `rfc/audit/`, read by `readPendingVerdicts`; the write goes only through `replaceAudit`, the one writer of `rfc/audit/` |
| Resource exhaustion and error leakage | not applicable: a local development verb over a file the author holds, run by the author; its refusals name the id and the fix, and no secret passes through it |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Group by test package | group by stem, as the quoting pass did | stems share test files, so stem agents collided (D-6) |
| Derive the verdict inventory, commit only the split table | commit the 894 ids | the audit files are the source; a copy drifts from the first fix. The split facts live only in scratch |
| Parent plus eight children by protocol group | one phased spec; operator-first protocols only | owner decision, 2026-09-28: each child closes on its own and runs in its own session |
| Ledger ownership by stem, test work by package | package only | the ledger files are per stem and rewritten whole; two children on one stem lose or carry each other's hunks |
| `mode rejudge` on `audit-stamp` | a separate `audit-rejudge` verb; hand-delete and re-stamp | one writer with one inverted refusal; a second verb re-declares the fingerprint path, and a hand deletion reads as a deleted finding (P-2) |
| The upgrade predicate shared by the stamp and the gate | a copy in the stamp | one declaration; a copy drifts from the gate it anticipates |
| Children written `ready` by the parent | eight skeletons, each through `/ze-spec` | the method is fixed and the inventory derived; per-verdict reading happens in each child's implementation |
| Narrowing audit in the parent, before child row edits | narrowing inside each child | owner scope decision; one comparison per row, and no child edits a row while it is compared |

## Known Limitations

- The earlier `./le rfc check` report recorded18 `producer-changed` records attributed to concurrent work in `plan/journal/concurrent-rfc-gate-stale.md`. That historical result is not the current stale population. The final serialized check must establish current freshness and route actual refusals.
- The R-7 rows of un-enrolled stems (8210bis, rfc1035, rfc9190) are recorded in this spec's table, not in an audit file, because `audit-stamp` refuses an un-enrolled stem; enrolment is `spec-rfc-evidence-strength-3`'s
- Verdicts blocked by another spec's producer fix leave this pass in that spec's acceptance criteria (P-3), so the whole-corpus listing reaches empty only when those specs close
- Owner decision, 2026-10-08: `plan/post-release/features/spec-bgp-tunnel-encapsulation-consumption.md` owns the absent RFC 9012 attribute-driven consumer. RFC9012-13-16 is canonically unimplemented, with AC-2/AC-6 ownership, not a pending weak rejudge. Gap RFC9012-13-17, MAC-consumption RFC9012-4.2-1, Sections3.7/15 consumer work and first-occurrence consumption under13-21 remain there without conformance credit. Existing receive RFC9012-13-10/12/13/14 and RFC 9830 ownership do not move. RFC9012-13-10 is now canonical enforced for processing equivalence, not absent tunnel consumption.

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
- [ ] The scoped evidence passes: `./le test unit pkg path ./internal/le/rfc/ run 'TestAuditStamp|TestRFCActionsAuditStampRejudgeMode|TestCheckAuditRatchetSeesTipCommit'` and `./le rfc check`. No agent runs any `./le verify` command, and closing a spec never waits on a whole-tree check (`ai/META-DECISIONS.md`, "Ze is pre-release"); GitHub CI verifies the pushed commit (corrected 2026-11-14 from "`./le verify worktree` passes")
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
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/validation/spec-rfc-verdict-test-fix-pass.md` only, in the same `./le commit create` script

## Implementation Summary

Written 2026-11-14 from git history, the `rfc/audit/` ledger and the source, each claim read at HEAD that day. Six commits carry the trailer `Spec-work: rfc-verdict-test-fix-pass` (`git log --grep`, 2026-11-14), all closure work: `1f80b8f446` makes the re-judge reason rule read the same on every surface (the `audit-stamp` help text, the doc comments of `findingUpgrade` and `rejudgeRefusal`, `docs/functional-tests.md`, `ai/skills/ze-rfc-audit.md`), landed apart so the closing commits carry no code; `937c0bb8fb` homes three weak verdicts, `9c67f29b04` moves the verdict-pass records out of `plan/handover/` into `plan/records/`, `50b31b7804` names every remaining weak verdict in an AC row, `da39ab438f` removes the verdict pass's handover directory, and `1ae312a8f2` records RFC9234-5-6 re-judged `enforced` by an independent auditor after `028620f16e` had staled it (not one of this spec's ids; met in the closing ledger check). The parent's earlier commits carry none, and each child's work carries its child's stem.

### What Was Implemented
| Commit | Date | What it carries |
|--------|------|-----------------|
| `2d078935e3` | 2026-09-28 | Phases 1 and 2: `mode new\|rejudge` on `./le rfc audit-stamp` (`parseStampMode`, `stampRefusal`, `rejudgeRefusal`, `rejudgeVerdictKeys` in `internal/le/rfc/audit_stamp.go`; the `mode` parameter in `internal/le/rfc/actions.go`); `upgradeOverUnchangedUnits` extracted in `internal/le/rfc/check_audit.go` and called by both `checkAuditFindings` and `rejudgeRefusal`; the six tests of the TDD plan; the AC-6 pages (`ai/skills/ze-rfc-audit.md`, `docs/functional-tests.md`, `docs/contributing/rfc-implementation-guide.md`, `docs/contributing/rfc-conformance-gates.md`, `ai/INDEX.md`). Its message records the tests red on the stub and green after |
| `174862502f` | 2026-09-28 | Phase 3: the eight child specs, then under `plan/pre-release/` |
| `4c7f52aa06` | 2026-09-28 | Phase 4: the narrowing audit findings, into this spec and the children |
| `fefe38b68c` | 2026-09-28 | Phase 5: tests and records for four cross-group verdicts, re-judged in `mode rejudge` by an agent that wrote none of them: RFC1071-1-4, RFC905-x-3 and RFC905-x-4 weak to `enforced`; RFC4303-2.1-1 stayed weak |
| `cb4801b4cc` | 2026-09-28 | RFC4303-2.1-1, D-8 fix: the IKE initiator refuses a peer's reserved ESP SPI 0; `rfc4303_peer_spi_test.go` with records; an independent judge re-judged the verdict weak to `enforced` |
| the eight closing commits of the Depends row | 2026-09-30 to 2026-11-14 | each child's test fixes, records, row corrections and D-8 fixes, recorded in its closed spec (`git show <sha>^:<spec path>`) |

### Bugs Found/Fixed
- `cb4801b4cc`: RFC 4303 Section 2.1, the IKE initiator read an SAr2 ESP SPI of 0 as not chosen and drew an outbound SPI the peer never allocated; `rfc4303_peer_spi_test.go` covers all four peer-SPI producers in both polarities.
- The children's D-8 fixes are recorded in each child; `plan/learned/049-rfc-verdict-fix-ospf.md` and `plan/learned/050-rfc-verdict-fix-services.md` summarize two of them.

### Documentation Updates
- `2d078935e3`: the AC-6 pages above; `docs/functional-tests.md` anchors `internal/le/rfc/audit_stamp.go -- auditStamp` and `-- rejudgeRefusal`.
- At closure, 2026-11-14: `docs/functional-tests.md` (the audit-stamp paragraph), `ai/skills/ze-rfc-audit.md` ("Re-judging a recorded verdict") and the `audit-stamp` Why string in `internal/le/rfc/actions.go` say what `rejudgeRefusal` does: `upgrade_reason` is accepted on any `weak`, `wrong` or `partial` verdict raised to `enforced`, required there when every tagged unit is byte-identical, and refused on any move that is not such an upgrade. `docs/functional-tests.md` had also named `weak` or `wrong` only, and the skill had described the one-file-per-stem layout that `dcc219072a` replaced.

### Deviations from Plan
- AC-1 says "the same position in the file". `2d078935e3` met it over one audit file per stem. Since `dcc219072a` (2026-10-23) each verdict has its own file, `rfc/audit/<stem>/<id>.json`, so a re-judge rewrites that file; `TestAuditStampRejudgeReplacesInPlaceAndReadsFresh` asserts the id order (`auditIDOrder`) and that a verdict it was not given is unchanged.
- P-1: from `e23a116783` (2026-10-26) an independent `ze-test-judge` agent records each approval, not the child itself (Existing Mechanisms).
- A-4 broke (Mistake Log).
- RFC5882-4.4-1 left this pass by the AC-9 transfer and reads `partial`, not `enforced` (Work Not Done).

## Mistake Log
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-4 held the 12 code-defect specs, `spec-ipsec-rfc9190` and `spec-fixit-dns-rfc1035-conformance` the only owners of verdicts a test-only fix cannot reach | 11 of the 19 verdicts left on 2026-11-14 are owned by other specs, and three had no owner until `937c0bb8fb`; the services child's own re-run found three ids its list lacked (RFC7950-7.9.4-1, RFC2131-4.1-7 and RFC2131-4.1-15, its A-2 and Mistake Log) | the closure re-run of the derived listing, each id read in its owner's AC table, 2026-11-14 | P-3 had each child move its blocked verdicts into the owner's criteria; AC-11 lists the rest |
| claim without its producer read | the closure survey counted 16 of the 19 ids as owned because a live spec mentions each | RFC4271-5.1.2-2 and RFC4271-8.2.2-15 appear only in owned journal rows tables, and RFC2205-3-40 only in a Task table behind a catch-all AC-1 | reading each owner's AC table, 2026-11-14 | `50b31b7804` named each in an AC row: RFC4271-5.1.2-2 and RFC2205-3-40 AC-1 and AC-2 of `plan/release/validation/spec-rfc-weak-verdict-proof-gaps.md`, RFC4271-8.2.2-15 AC-1g of `plan/post-release/features/spec-bgp-fsm-delayopen-and-automatic-start.md` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Every `weak` and `wrong` verdict in `rfc/audit/` resolved, or named in an owning spec's criteria (P-3) | Done | AC-11 | 1014 on 2026-09-28; 19 `weak` and no `wrong` on 2026-11-14, each named in an owning spec's AC row (the AC-11 table) |
| Every un-enrolled R-7 row resolved | Done | the children's AC-C7 | 8210bis 5.12-1, 5.12-3 and 7-2 re-judged `enforced` 2026-09-29 (BGP child, `git show 3c417fdab2^:plan/release/validation/spec-rfc-verdict-fix-bgp.md`, R-7 table); RFC8362-2-1 `enforced` 2026-10-21 (OSPF child AC-C7); RFC1035-4.1.4-1 `enforced` in `dbdde9fb4f` and the other four RFC 1035 rows AC-28 of `plan/release/validation/spec-dns-existing-support-proof.md` (services child AC-C7); the thirteen RFC 9190 rows AC-9 and AC-10 of `plan/release/validation/spec-ipsec-rfc9190.md` (IKE/EAP child AC-C7) |
| Split-needed rows, the narrowing audit and the row-quality corrections | Done | AC-7 (`4c7f52aa06`); the children's AC-C3 and AC-C6 | each child closed with both |
| A verified defect fixed in code under D-8 | Done | Bugs Found/Fixed; each child | |
| P-2: verdicts replaced only through `mode rejudge` | Done | `stampRefusal` refuses a judged id in `new` mode, so no other route writes over a verdict | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestAuditStampRejudgeReplacesInPlaceAndReadsFresh` (`internal/le/rfc/audit_stamp_test.go`), added in `2d078935e3` | the scoped run (below) |
| AC-2 | Done | `TestAuditStampRejudgeRefusesAndWritesNothing` (`audit_stamp_test.go`); `stampRefusal`, "has no verdict ... to re-judge" | the scoped run |
| AC-3 | Done | `TestAuditStampRejudgeUpgradeNeedsAReasonWhenUnitsAreUnchanged` (`audit_stamp_test.go`); `rejudgeRefusal` | the scoped run |
| AC-4 | Done | `TestAuditStampRejudgeRefusesAReasonOffAnUpgrade` (`audit_stamp_test.go`); `rejudgeRefusal` | the scoped run |
| AC-5 | Done | `TestAuditStampLeavesARecordedVerdictUntouched` and `TestAuditStampRefusesAReasonInNewMode` (`audit_stamp_test.go`), `TestRFCActionsAuditStampRejudgeMode` (`internal/le/rfc/actions_test.go`); `parseStampMode` refuses an unknown word | the scoped run |
| AC-6 | Done | `mode rejudge` named in `ai/skills/ze-rfc-audit.md` ("Re-judging a recorded verdict"), `docs/functional-tests.md` (the audit-stamp paragraph), `docs/contributing/rfc-implementation-guide.md` (the audit-stamp step), `docs/contributing/rfc-conformance-gates.md` (the `rfc/audit/` row), `ai/INDEX.md` (the "RFC audit verdict" row) and the action's help (`actions.go`, the `audit-stamp` Why and the `mode` ValueHelp); no page says a recorded verdict can only be refused (grep, 2026-11-14) | `2d078935e3`; the `partial` wording of `docs/functional-tests.md` fixed at closure |
| AC-7 | Done | "Narrowing audit findings, 2026-09-28" (`4c7f52aa06`) | |
| AC-8 | Done | `174862502f` wrote the eight children; each closed in the commit the Depends row names | they were later moved out of `plan/pre-release/` and closed from there |
| AC-9 | Done for four ids | RFC1071-1-4, RFC905-x-3, RFC905-x-4 `enforced` in `fefe38b68c`; RFC4303-2.1-1 `enforced` in `cb4801b4cc` and re-judged since by independent auditors (2026-10-10, 2026-11-08, per its note); each re-judge by an agent that wrote none of the tests (both commit messages) | RFC5882-4.4-1 was transferred by owner decision of 2026-10-11 and reads `partial` today (Work Not Done) |
| AC-10 | Done | AC-2 of `plan/release/fixes/spec-ike-dpd-demand-driven.md` names RFC7296-2.4-2; the verdict reads `enforced` (independent re-judge of 2026-10-18 after `98975fb2d8`, per its note) | |
| AC-11 | Done 2026-11-14 | every child closed (Depends row); `grep -rlE '"verdict": *"(weak\|wrong)"' rfc/audit` lists 19 `weak` and 0 `wrong`, each named in an owning spec's AC row (table below) | RFC5882-4.4-1, `partial` and so outside the listing, is AC-1 of `plan/post-release/features/spec-bfd-shared-session-same-remote-system.md`. The pass's other ids that read `partial` on 2026-11-14 are outside the listing too, each named by its owner: RFC8050-4.1-1 AC-62 of `plan/release/fixes/spec-bgp-one-off-defects.md` (Work Not Done); RFC7950-7.6.5-1 AC-2 of `plan/release/fixes/spec-config-yang-when-unique-choice.md`, kept `partial` there as an owner-approved deviation (its OWNER RULING 6); RFC9086-7-1, RFC9514-7.2-5 and RFC9514-7.2-6 AC-1 to AC-4 of `plan/post-release/features/spec-bgp-epe-peer-segments-and-srv6-peernode.md` (AC-1 closes RFC9086-7-1's gap half, AC-2 and AC-3 the two RFC 9514 rows, AC-4 the ledger); RFC2131-4.1-7 and RFC2131-4.1-15 AC-31 of `plan/release/fixes/spec-network-services-one-off-defects.md`; RFC7950-7.9.4-1 AC-3 of `plan/release/fixes/spec-config-yang-when-unique-choice.md` (Work Not Done); RFC2545-3-3 AC-41 of `plan/release/fixes/spec-bgp-one-off-defects.md` and AC-5 to AC-7 of `plan/release/fixes/spec-rfc-proof-debt-outside-filter-wire.md`; RFC2545-3-6 AC-4 of `plan/release/fixes/spec-rfc2545-readvertise-on-condition-flip.md`; RFC4301-4.4.1-3 AC-7 of `plan/post-release/features/spec-ipsec-spd-final-discard-vpp-ospf.md`; RFC4301-5.2-2 AC-4 of `plan/release/fixes/spec-ipsec-dual-form-receive-spd-bypass.md`; RFC6286-2.1-1 an owner-approved deviation (Work Not Done) |

| Verdict | Owning spec | AC |
|---------|-------------|----|
| RFC2205-3-40 | `plan/release/validation/spec-rfc-weak-verdict-proof-gaps.md` | AC-2 |
| RFC3101-3.2-2 | `plan/release/fixes/spec-ospf-nssa-translator-reachability.md` | AC-3 |
| RFC4090-6-6 | `plan/release/fixes/spec-rsvpte-frr-link-protection-fallback.md` | AC-4 |
| RFC4271-5.1.2-2 | `plan/release/validation/spec-rfc-weak-verdict-proof-gaps.md` | AC-1 |
| RFC4271-8.2.2-15 | `plan/post-release/features/spec-bgp-fsm-delayopen-and-automatic-start.md` | AC-1g |
| RFC5036-3.4.4.1-1, RFC5036-3.4.4.1-2 | `plan/release/fixes/spec-ldp-label-request-path.md` | AC-HC-3 |
| RFC5798-7.2-1, RFC9568-7.2-5 | `plan/release/validation/spec-vrrp-deferred-keepalived-interop-scenarios.md` | AC-5 |
| RFC7296-3.13.1-1, RFC7296-3.13.1-2 | `plan/release/fixes/spec-rfc4301-architecture-gaps.md` | AC-13 |
| RFC7950-8.3.3-1 | `plan/release/fixes/spec-config-yang-when-unique-choice.md` | AC-2 |
| RFC7950-9.1-1 | `plan/release/fixes/spec-config-yang-when-unique-choice.md` (accepted forms); `plan/post-release/features/spec-config-yang-reference-types.md` (unsupported forms) | AC-2; AC-9 |
| RFC7950-9.6.4.2-1 | `plan/release/fixes/spec-config-yang-loader-structural-checks.md` | AC-2 |
| RFC9234-5-9 | `plan/release/fixes/spec-bgp-otc-internet-facing-edge.md` | AC-1 |
| RFC9252-5-3 | `plan/release/fixes/spec-vpp-srv6-service-policy.md` | AC-7 |
| RFC9494-4.2-6 | `plan/release/fixes/spec-bgp-graceful-restart-rfc-defects.md` | AC-4 |
| RFC9728-3.3-1 | `plan/release/fixes/spec-mcp-protected-resource-metadata-path.md` | AC-6 |
| SFLOW-V5-x-11 | `plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md` | AC-3 |

The scoped runs, by the main thread on 2026-11-14 under the race detector: `./le test unit pkg path ./internal/le/rfc/ run 'TestAuditStamp|TestRFCActionsAuditStampRejudgeMode|TestCheckAuditRatchetSeesTipCommit'`, exit 0, before the closure's edit of the `audit-stamp` help text; after that edit, `./le test unit pkg path ./internal/le/rfc/` over the whole package, exit 0; and after round 2's two doc-comment edits (`findingUpgrade` in `check_audit.go`, `rejudgeRefusal` in `audit_stamp.go`), `./le go lint run scope "./internal/le/rfc/"`, exit 0. The help text and both comments landed in `1f80b8f446`.

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestAuditStampRejudgeReplacesInPlaceAndReadsFresh` | Done | `internal/le/rfc/audit_stamp_test.go` | added in `2d078935e3`; green in the scoped run of 2026-11-14 (AC section) |
| `TestAuditStampRejudgeRefusesAndWritesNothing` | Done | `audit_stamp_test.go` | added in `2d078935e3`; green in the same run |
| `TestAuditStampRejudgeUpgradeNeedsAReasonWhenUnitsAreUnchanged` | Done | `audit_stamp_test.go` | added in `2d078935e3`; green in the same run |
| `TestAuditStampRejudgeRefusesAReasonOffAnUpgrade` | Done | `audit_stamp_test.go` | added in `2d078935e3`; green in the same run |
| `TestAuditStampLeavesARecordedVerdictUntouched` (existing) | Done | `audit_stamp_test.go` | kept as the default-mode proof; green in the same run |
| `TestAuditStampRefusesAReasonInNewMode` | Done | `audit_stamp_test.go` | added in `2d078935e3`; green in the same run |
| `TestRFCActionsAuditStampRejudgeMode` | Done | `internal/le/rfc/actions_test.go` | added in `2d078935e3`; green in the same run |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/rfc/audit_stamp.go`, `check_audit.go`, `actions.go`, `audit_stamp_test.go`, `actions_test.go` | Done | `2d078935e3`; `internal/le/rfc/audit.go` and `check_audit_baseline_test.go` changed too |
| `ai/skills/ze-rfc-audit.md`, `docs/functional-tests.md`, `docs/contributing/rfc-implementation-guide.md`, `docs/contributing/rfc-conformance-gates.md`, `ai/INDEX.md` | Done | `2d078935e3`; `docs/functional-tests.md` again at closure |
| `plan/release/fixes/spec-ike-dpd-demand-driven.md` | Done | AC-2 there |
| this spec (AC-7) | Done | `4c7f52aa06` |
| the eight children (Files to Create) | Done | `174862502f`, each removed at its closure |

### Audit Summary
- **Total items:** 28 (5 requirements, 11 ACs, 7 tests, 5 file rows)
- **Done:** 28; AC-9 is done for its four ids, and its fifth, transferred by owner decision, is AC-1 of `plan/post-release/features/spec-bfd-shared-session-same-remote-system.md`
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (AC-1's "position in the file", Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| every `weak` and `wrong` verdict in `rfc/audit/` resolved, by a test that proves the whole sentence or a corrected row | ledger query | 1014 on 2026-09-28; `grep -rlE '"verdict": *"(weak\|wrong)"' rfc/audit` on 2026-11-14 lists 19 `weak` and no `wrong`, each owned as AC-11 records |
| every un-enrolled R-7 row resolved | the children's recorded judgements | the 22 rows: three 8210bis and RFC8362-2-1 and RFC1035-4.1.4-1 re-judged `enforced` by non-authors; four RFC 1035 rows AC-28 of `plan/release/validation/spec-dns-existing-support-proof.md`; thirteen RFC 9190 rows AC-9 and AC-10 of `plan/release/validation/spec-ipsec-rfc9190.md` (Requirements from Task) |
| a resolved verdict carries a record and a judge who did not write the test | discrimination records and audit notes | `fefe38b68c` and `cb4801b4cc` for the parent's ids; each child's AC-C2 |
| the split-needed rows, the narrowing audit and the row-quality corrections done | spec record and the children's closures | AC-7 in `4c7f52aa06`; AC-C3 and AC-C6 of each child at its closing commit |
| a verified defect is fixed in code (D-8) | unit and interop tests in the children | `cb4801b4cc` here; the children's fixes, with interop where a peer exists (`plan/learned/049-rfc-verdict-fix-ospf.md`: three FRR scenarios red under revert; `plan/learned/050-rfc-verdict-fix-services.md`: `TestTFTPCurlDuplicateACKInterop`) |
| a recorded verdict is re-judged through `mode rejudge` (P-2) | unit tests through `Answer` | the seven TDD tests in `internal/le/rfc`, with `TestCheckAuditRatchetSeesTipCommit`: `./le test unit pkg path ./internal/le/rfc/ run 'TestAuditStamp\|TestRFCActionsAuditStampRejudgeMode\|TestCheckAuditRatchetSeesTipCommit'`, exit 0, race detector, run by the main thread on 2026-11-14 |

## Work Not Done
| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| RFC9568-7.2-5 and RFC5798-7.2-1 (`weak`) | a test gap: the wire test of the IPv6 advertisement checksum carries no tag for these rows (homed in `937c0bb8fb`) | `plan/release/validation/spec-vrrp-deferred-keepalived-interop-scenarios.md` AC-5 |
| RFC9234-5-9 (`weak`) | missing behaviour: Ze has no notion of an Internet-facing AS (homed in `937c0bb8fb`) | `plan/release/fixes/spec-bgp-otc-internet-facing-edge.md` AC-1 (moved from `plan/post-release/` in `50b31b7804`, since the route-server release profile lists RFC 9234) |
| RFC4271-5.1.2-2 (`weak`) | a proof gap: no tagged unit reaches the route-server rail AS_PATH guards (named in an AC row in `50b31b7804`) | `plan/release/validation/spec-rfc-weak-verdict-proof-gaps.md` AC-1 |
| RFC2205-3-40 (`weak`) | a proof gap on the diverted message's interface, source address and TTL (named in an AC row in `50b31b7804`) | `plan/release/validation/spec-rfc-weak-verdict-proof-gaps.md` AC-2 |
| RFC4271-8.2.2-15 (`weak`) | no tagged unit runs a DelayOpenTimer in Connect (named in an AC row in `50b31b7804`) | `plan/post-release/features/spec-bgp-fsm-delayopen-and-automatic-start.md` AC-1g |
| the other 13 `weak` verdicts | a producer fix or proof owned elsewhere (P-3) | each spec the AC-11 table names |
| RFC5882-4.4-1 reaching `enforced` | transferred 2026-10-11 to AC-24 of `spec-bgp-live-receive-binding-and-bfd-keep-session`, which closed 2026-10-13 with the verdict `partial` (re-judged 2026-10-30): sessions to one remote system are not always shared. The AC-11 listing counts `weak` and `wrong` only, so it does not show it | `plan/post-release/features/spec-bfd-shared-session-same-remote-system.md` AC-1 |
| four specs still cite this spec's path or the verdict pass's handover directory, which `da39ab438f` removed | their spec-edit gate refuses every edit, so the citation could not be repointed at closure: `plan/release/validation/spec-rfc-clause-scoped-proof.md`, `plan/release/fixes/spec-bgp-graceful-restart-rfc-defects.md`, `plan/post-release/features/spec-bgp-epe-peer-segments-and-srv6-peernode.md`, and the D2 row of `plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md` | `plan/handover/39-residuals-of-closed-handovers.md` |
| RFC8050-4.1-1 reaching `enforced` (`partial`, re-judged 2026-10-30) | missing behaviour: `writeTableDumpV2` (`internal/plugins/mrt/dump.go`) builds each `RIBEntry` with no Path Identifier and `RIBDumpVisitor.OnRoute` carries none, so an ADD-PATH TABLE_DUMP_V2 entry holds no route's identifier; RFC 8050 is listed by the route-collector profile, which is a release profile | `plan/release/fixes/spec-bgp-one-off-defects.md` AC-62 |
| RFC7950-7.9.4-1 reaching `enforced` (`partial`; found by the services child, its A-2) | a proof and producer gap: only "at least one node from" is tested, and `mandatoryPresent` (`internal/component/config/yang/validator_mandatory.go`) answers present at the first case holding a node, so nodes from two cases of one mandatory choice are accepted against "exactly one of the choice's case branches" (RFC 7950 Section 7.9.4) | `plan/release/fixes/spec-config-yang-when-unique-choice.md` AC-3 |
| RFC6286-2.1-1 reaching `enforced` (`partial`) | an owner-approved deviation, never counted conformant: D4 of the closed `spec-bgp-open-session-rfc-defects` (owner answer to its Q2, 2026-10-13) keeps the per-peer `session router-id` override, so one speaker's BGP Identifier can differ per peer (`rfc/short/rfc6286.md`, the RFC6286-2.1-1 row) | none: the deviation is the owner's decision |
| `./le verify worktree` | obsolete: no agent runs any `./le verify` command, and closing a spec never waits on a whole-tree check (`ai/META-DECISIONS.md`, "Ze is pre-release") | none: GitHub CI verifies the pushed commit |

## Review Gate

Six commits carry `Spec-work: rfc-verdict-test-fix-pass` (`git log --grep`, 2026-11-14): `937c0bb8fb`, `9c67f29b04`, `50b31b7804` and `da39ab438f`, closure work in `plan/` only, `1ae312a8f2`, the independent re-judge of RFC9234-5-6 in `rfc/audit/`, and `1f80b8f446`, the closure's help-text, comment, page and skill edits (Implementation Summary). The reviewed change set is this record, the closure's edits to `docs/functional-tests.md`, `ai/skills/ze-rfc-audit.md`, the `audit-stamp` help text in `internal/le/rfc/actions.go` and the doc comments of `findingUpgrade` (`check_audit.go`) and `rejudgeRefusal` (`audit_stamp.go`), AC-62 of `plan/release/fixes/spec-bgp-one-off-defects.md`, `plan/learned/051-rfc-verdict-test-fix-pass.md`, those six commits, and the removal of this spec.

| Field | Value |
|-------|-------|
| Artifact | recorded 2026-11-14 by `./le spec review record`: verdict clean, 3 rounds, over `internal/le/rfc/actions.go`, `audit_stamp.go` and `check_audit.go` |
| `./le spec review check` | OK, 2026-11-14 |
| Rounds | 3 |
| Verdict | clean: 0 BLOCKER, 0 open ISSUE across three rounds (R1-1 to R1-8, R2-5, R2-6 and R3-1 each carry a resolution below) |
| Reviewer lenses used | round 1 (independent reviewer, 2026-11-14, authored none of the change; scope: this record, the `docs/functional-tests.md` paragraph, learned 051, and the three `Spec-work` commits then landed, `937c0bb8fb`, `9c67f29b04` and `50b31b7804`): 1 BLOCKER, 7 ISSUE, 4 NOTE; round 2 (independent reviewer, 2026-11-14, authored none of the change; scope: round 1's resolutions, the `Spec-work` commits, the `partial` verdicts of the pass's 2026-09-28 inventory, and the code the help string describes): 0 BLOCKER, 2 ISSUE, 5 NOTE; round 3 (independent reviewer, 2026-11-14, authored none of the change; scope: round 2's resolutions, the five `Spec-work` commits, every `partial` verdict under `rfc/audit/` against the pass's ids and their AC owners, AC-62 against `rfc/full/rfc8050.txt` and `internal/plugins/mrt/dump.go`, and the services child's A-2 at `22b8957163^`): 0 BLOCKER, 1 ISSUE, 3 NOTE |

### Findings fixed
| # | Severity | Finding | Resolution |
|---|----------|---------|------------|
| R1-1 | BLOCKER | the D-3 correction said every no-keyword row keeps MUST with no demotion and no owner call; RFC7296-1.4-1 moved to SHOULD and RFC2759-x-10 and x-12 were retired, each by owner decision of 2026-10-11 | the "level with no BCP 14 keyword" cell and the D-3 constraint keep the D-3 quote, say D-3 itself raises no review and no question, and name the three exceptions with their correction paragraphs in `rfc/corrections/rfc7296.md` and `rfc/corrections/rfc2759.md` (both read) |
| R1-2 | ISSUE | Work Not Done, the Mistake Log action and the AC-11 note were stale after the owners landed | each owner AC row read in its spec; AC-11 now carries the listing command, its count (19 `weak`, 0 `wrong`, rerun 2026-11-14) and the id to spec to AC table; "owes an AC row" and "unowned" are gone |
| R1-3 | ISSUE | "one commit carries the trailer" was false | the Implementation Summary and this preamble name `937c0bb8fb`, `9c67f29b04` and `50b31b7804` |
| R1-4 | ISSUE | the closure's wording of `rejudgeRefusal` said the reason was refused on any move other than an upgrade over byte-identical units | `rejudgeRefusal`, `findingUpgrade` and `upgradeOverUnchangedUnits` read: the reason is accepted on any `weak`, `wrong` or `partial` raised to `enforced`, required there when every tagged unit is byte-identical, refused on any move that is not such an upgrade. Corrected in `docs/functional-tests.md`, Existing Mechanisms row 1, `ai/skills/ze-rfc-audit.md` and the `audit-stamp` Why string in `internal/le/rfc/actions.go` (only that string; no test, testdata or doc pins the old sentence, by grep) |
| R1-5 | ISSUE | `ai/skills/ze-rfc-audit.md` described the one-file-per-stem layout | it now says the entry rewrites that id's `rfc/audit/<stem>/<id>.json` with fresh fingerprints and no other verdict file changes; `./le ai sync update` owed |
| R1-6 | ISSUE | "RULINGS R5" named no path | each cite reads "R5 of `plan/records/rfc-verdict-test-fix-pass-rulings.md`"; no cite of the verdict pass's handover directory remains in this record or in learned 051 (grep, 2026-11-14) |
| R1-7 | ISSUE | learned 051 and Core Insight said one page restated the rule | both now say that of the surfaces that restate it (the skill, `docs/contributing/rfc-conformance-gates.md`, the action help, `docs/functional-tests.md`), only `docs/functional-tests.md` had dropped `partial`, and that three of them also said "refused on any other move", which R1-4 corrected |
| R1-8 | ISSUE | cells said a scoped run was owed after the main thread had run it | the AC rows, the TDD table, Goal Validation and Pre-Commit Verification record the run: exit 0, race detector, 2026-11-14, by the main thread |
| R1-N1 | NOTE | no stamp test drives `partial` to `enforced` over unchanged units | recorded: it holds by the shared predicate, since `findingUpgrade` names `VerdictPartial` and both the stamp and the gate call `upgradeOverUnchangedUnits` |
| R1-N2 | NOTE | Existing Mechanisms omitted the `source head` route and its refusal of a partial verdict | resolved: row 1 names it, read in `auditStampSource` |
| R1-N3 | NOTE | "lowercase keyword" widened D-3's lowercase "must" | resolved: the row quotes D-3's own words and leaves RFC8092-4-1's "should" to the judge, as R5 leaves RFC5443-4-1 |
| R1-N4 | NOTE | historical paths in the pre-closure sections | recorded: they describe the tree on the day each section was written |

Round 2:

| # | Severity | Finding | Resolution |
|---|----------|---------|------------|
| R2-5 | ISSUE | the doc comment of `findingUpgrade` named a finding as `weak` or `wrong`, while its body also tests `VerdictPartial` | the comment now names `weak`, `wrong` or `partial`; the doc comment of `rejudgeRefusal` (`audit_stamp.go`), which said a reason "on any other move is refused", now says a reason is accepted on any such upgrade, units changed or not, and refused on a move that is not one; learned 051 point 1 and Core Insight name both comments; both landed in `1f80b8f446`, and the main thread's scoped lint after them exited 0 |
| R2-6 | ISSUE | RFC8050-4.1-1 reads `partial` and no spec named it in an AC row | `rfc/audit/rfc8050/RFC8050-4.1-1.json` read (`partial`); the producer read at `writeTableDumpV2`, whose `RIBEntry` carries no Path Identifier, and `RIBDumpVisitor.OnRoute`, which passes none; RFC 8050 Section 4.1 quoted from `rfc/full/rfc8050.txt`; homed as AC-62 of `plan/release/fixes/spec-bgp-one-off-defects.md` (route-collector is a release profile); Work Not Done carries it and RFC6286-2.1-1 (an owner-approved deviation, D4 of the closed open-session spec); the AC-11 note names every `partial` id of the pass with its owner; learned 051 point 4 names it |
| R2-1 | NOTE | five commits carry the trailer, not three | the Implementation Summary, this preamble and the round 1 lens line name `937c0bb8fb`, `9c67f29b04`, `50b31b7804`, `da39ab438f` and `1ae312a8f2` (`git log --grep`, read 2026-11-14); `1f80b8f446`, landed after round 3, is the sixth, named in both lists |
| R2-2 | NOTE | the Work Not Done citers row said "which this closure removes" | it says `da39ab438f` removed the directory |
| R2-3 | NOTE | the scoped-run paragraph split the AC table after AC-5 and said a rerun was owed after the main thread had run it | moved below the AC-11 table, recording the three-test run before the help-text edit, and the whole-package run and the scoped lint after it, each exit 0 |
| R2-4 | NOTE | the AC-9 Pre-Commit row did not say whether `./le rfc check` named the four ids | the row records the main thread's run of 2026-11-14: no violation in the four ids, the others in other sessions' in-flight IPsec and reactor work |
| R2-N5 | NOTE | learned 051 point 2, the A-4 row and the Mistake Log said the services child found three more ids, and the finding read two (RFC2131-4.1-7 and 4.1-15) | not taken as worded: the services child's A-2 and Mistake Log (`git show 22b8957163^:plan/release/validation/spec-rfc-verdict-fix-services.md`) name three, RFC7950-7.9.4-1, RFC2131-4.1-7 and RFC2131-4.1-15; the three surfaces keep "three" and now name the ids |

Round 3:

| # | Severity | Finding | Resolution |
|---|----------|---------|------------|
| R3-1 | ISSUE | RFC7950-7.9.4-1 reads `partial` and no AC row named it; its only home was the Task table of `spec-config-yang-when-unique-choice.md` behind AC-1, whose `{gap}` wording does not fit a partial row | read the verdict file (`partial`), the `rfc/short/rfc7950.md` row (tested "at least one node from", gap "exactly one of the choice's case branches"), RFC 7950 Section 7.9.4 in `rfc/full/rfc7950.txt` and `mandatoryPresent`, which returns present at the first case holding a node; AC-3 of that spec now names it, with the quote, today's behaviour and the proof owed; the AC-11 note and a Work Not Done row name it |
| R3-2 | NOTE | AC-62 stated the target without today's behaviour | AC-62 now says today each entry's Path Identifier is 0, because `writeTableDumpV2` builds `RIBEntry` without `PathID` and `RIBDumpVisitor.OnRoute` carries none, then states the target |
| R3-3 | NOTE | the AC-11 list of `partial` ids omitted four the children routed | each AC read by grep to name its id: RFC2545-3-3 (AC-41 of `spec-bgp-one-off-defects.md`; AC-5 to AC-7 of `spec-rfc-proof-debt-outside-filter-wire.md`), RFC2545-3-6 (AC-4 of `spec-rfc2545-readvertise-on-condition-flip.md`), RFC4301-4.4.1-3 (AC-7 of `spec-ipsec-spd-final-discard-vpp-ospf.md`), RFC4301-5.2-2 (AC-4 of `spec-ipsec-dual-form-receive-spd-bypass.md`); all four added to the note |
| R3-4 | NOTE | over-long lines in learned 051 points 1 and 2 | rewrapped to the file's width |

## Pre-Commit Verification

Readers run 2026-11-14 by the closing agent; the scoped test run was made by the main thread the same day.

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/rfc/audit_stamp.go`, `internal/le/rfc/audit_stamp_test.go`, `internal/le/rfc/actions_test.go` | yes | `gopls symbols internal/le/rfc/audit_stamp.go` lists `parseStampMode`, `stampRefusal`, `rejudgeRefusal`, `stampFingerprints`; grep finds the seven TDD tests |
| the eight children (Files to Create) | removed, as closure owes | `git log -1 --format=%s` of each Depends commit reads a child's closure |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 to AC-5 | the re-judge mode refuses and replaces as specified | `grep -n 'func Test.*Rejudge\|func TestAuditStampLeavesARecordedVerdictUntouched\|func TestAuditStampRefusesAReasonInNewMode' internal/le/rfc/*_test.go` lists all seven; `rejudgeRefusal` calls `upgradeOverUnchangedUnits`, as `checkAuditFindings` does; `./le test unit pkg path ./internal/le/rfc/ run 'TestAuditStamp\|TestRFCActionsAuditStampRejudgeMode\|TestCheckAuditRatchetSeesTipCommit'`, exit 0, race detector, main thread, 2026-11-14 |
| AC-6 | every named page carries the re-judge route | the grep under Deliverables Checklist, 2026-11-14 |
| AC-9 | four ids `enforced` | `rfc/audit/rfc1071/RFC1071-1-4.json`, `rfc/audit/rfc4303/RFC4303-2.1-1.json`, `rfc/audit/rfc905/RFC905-x-3.json` and `RFC905-x-4.json` read `"verdict": "enforced"`; `RFC5882-4.4-1.json` reads `partial`; `./le rfc check` (main thread, 2026-11-14) names no violation in RFC1071-1-4, RFC4303-2.1-1, RFC905-x-3 or RFC905-x-4, and its other violations are in other sessions' in-flight IPsec and reactor work |
| AC-10 | RFC7296-2.4-2 named in an owner's AC | AC-2 of `plan/release/fixes/spec-ike-dpd-demand-driven.md`; `rfc/audit/rfc7296/RFC7296-2.4-2.json` reads `enforced` |
| AC-11 | every remaining weak or wrong verdict is named in an owning spec's AC row | `grep -rlE '"verdict": *"(weak\|wrong)"' rfc/audit \| wc -l` gives 19, and the same grep for `wrong` gives 0 (2026-11-14); each id read in the AC row the AC-11 table names |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le rfc audit-stamp stem <stem> from <pending> mode rejudge` | none: a Go unit through `Answer`, `TestRFCActionsAuditStampRejudgeMode` (`internal/le/rfc/actions_test.go`) | the `audit-stamp` action declares `keyMode` with `new` and `rejudge` and answers through `auditStampAnswer` (`actions.go`, read 2026-11-14); green in the scoped run of 2026-11-14 |
| `./le rfc check` | none: `TestCheckAuditRatchetSeesTipCommit` (`internal/le/rfc/check_audit_baseline_test.go`) | `checkAuditFindings` calls `upgradeOverUnchangedUnits`; green in the scoped run of 2026-11-14 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | its row: 16 rows re-quoted before `0bf0696576`, baseline `fa7ac196df` for them |
| A-2 | confirmed | the narrowing audit re-derived the split set (`4c7f52aa06`) |
| A-3 | confirmed | P-1, owner answer at the research gate, 2026-09-28 |
| A-4 | broken | Mistake Log, Deviations |
| A-5 | superseded | the narrowing audit compared every row, so no finding rests on the 16-row floor |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/functional-tests.md` (the audit-stamp paragraph), `ai/skills/ze-rfc-audit.md` (the re-judge section) and the `audit-stamp` Why string in `internal/le/rfc/actions.go` | `rejudgeRefusal`, `findingUpgrade` and `upgradeOverUnchangedUnits` read 2026-11-14: the reason is accepted on any `weak`, `wrong` or `partial` raised to `enforced`, required there over byte-identical units, refused on any move that is not such an upgrade; each surface now says so, and the skill names the per-verdict file | yes |
| `docs/contributing/rfc-conformance-gates.md` (the `rfc/audit/` row) | names `partial` and says only what the upgrade over unchanged units needs | yes, unchanged |
| `docs/architecture/core-design.md`, the one writer of `rfc/audit/` | `replaceAudit` (`internal/le/rfc/reseal.go`) stays the writer the stamp calls | yes |
| No operator-visible change | the parent changed a `./le` development verb only, so commit B owes `no-news` | yes |

## Core Insight

One predicate for one decision kept two surfaces in step. `upgradeOverUnchangedUnits` is called by the stamp and by the commit gate, so when a later spec added the `partial` verdict, `findingUpgrade` learned it once and both refuse the same move. The rule is also restated in prose by four surfaces (the skill, `docs/contributing/rfc-conformance-gates.md`, the action help, `docs/functional-tests.md`); only `docs/functional-tests.md` had dropped `partial`, as had the doc comment of `findingUpgrade` itself, which named `weak` or `wrong` above a body that also tests `partial`, and three of them said the reason is "refused on any other move", which misstated which moves accept one, as did the doc comment of `rejudgeRefusal`. The code's behaviour needed no change; the prose and those two comments did. Routed to `plan/learned/051-rfc-verdict-test-fix-pass.md`.
