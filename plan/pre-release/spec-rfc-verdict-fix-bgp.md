# Spec: rfc-verdict-fix-bgp

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` (the parent: its `audit-stamp` `mode rejudge` phase before any re-judge here, and its narrowing-audit output for the BGP group before any row edit, parent R-11) |
| Phase | 2/48 |
| Handoff | - |
| Updated | 2026-10-06 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

This spec is child 1 of 8 of `plan/pre-release/spec-rfc-verdict-test-fix-pass.md`, the BGP group. The parent holds the method, the inventory and the cross-cutting work. This child carries the test fixes, row edits and D-8 code fixes for its packages and owned stems, and closes on its own.

**Goal (AC-C1).** The derived weak-or-wrong listing restricted to this child's packages and owned stems holds only the verdicts listed under "Blocked by" below, each named in the acceptance criteria of the spec that blocks it. Every other verdict is resolved as the parent's Goal states: a test proves the whole quoted sentence in both polarities and is re-judged `enforced` by an agent that did not write it, or the row is corrected, retired or re-attributed and re-judged against its new text.

| ID | Owner decision (parent, 2026-09-28) |
|----|------|
| P-1 | D-15 is standing approval for every tagged-unit edit and tag move: record `./le rfc approve unit <pkg>.<Test> reason "D-15: ..."` before the edit, and carry the `RFC-approved:` trailer in the commit |
| P-2 | a recorded verdict is replaced only through `./le rfc audit-stamp stem <stem> from <pending> mode rejudge`; an audit entry is never hand-deleted |
| P-3 | this child closes when every verdict in its scope is `enforced`, except those listed under "Blocked by"; each blocked verdict moves into the blocking spec's acceptance criteria |

Measured 2026-09-28 over `rfc/audit/*.json` with the filter below: **346 weak and 14 wrong, 360 in all, over 60 stems**. The parent's Split table counted a cross-group verdict in each child and predates the last two file moves, so some of its group figures differ. This count is exclusive (each verdict in exactly one child) and leaves out the parent's five cross-group verdicts and RFC7296-2.4-2.

### Packages it owns

Test packages: `internal/component/bgp/...`, `internal/core/bgp/...`, BGP interop (`internal/le/interoplab/bgp`) and MRT (`internal/mrt`, `internal/plugins/mrt`), plus the refined moves: `test/plugin`, `test/reload`, `internal/core/network`, `internal/plugins/flowspec-firewall`, `internal/component/sysrib`, the SRv6 nexthop test file `internal/plugins/fib/kernel/rfc9252_nexthop_srv6_linux_test.go`, and the IS-IS BGP-LS export test file `internal/plugins/isis/bgpls_export_rfc9552_test.go`.

The refinement paths above record the original assignment. Ownership follows those carriers through renames. The derived query below uses their current paths and includes the new OSPF router-ID proof.

| Package | Weak + wrong (2026-09-28) | Of which wrong |
|---------|--------------|----------------|
| `internal/component/bgp/reactor` | 75 | 0 |
| `internal/core/bgp/attribute` | 48 | 2 |
| `internal/component/bgp/message` | 43 | 0 |
| `internal/component/bgp/plugins/rib` | 27 | 2 |
| `internal/component/bgp/plugins/nlri/ls` | 25 | 2 |
| `internal/core/bgp/capability` | 18 | 2 |
| `internal/component/bgp/plugins/bmp` | 16 | 2 |
| `internal/component/bgp/plugins/rpki` | 16 | 0 |
| `internal/component/bgp/fsm` | 12 | 0 |
| `internal/component/bgp/plugins/gr` | 10 | 0 |
| `internal/component/bgp/plugins/nlri/mup` | 9 | 0 |
| `internal/component/bgp/plugins/role` | 9 | 0 |
| `internal/component/bgp/plugins/ls_export` | 8 | 0 |
| `internal/component/bgp/plugins/rib/pool` | 6 | 0 |
| `internal/mrt` | 6 | 1 |
| `internal/component/bgp/plugins/rib/storage` | 5 | 3 |
| `internal/le/interoplab/bgp` | 5 | 0 |
| `internal/component/bgp/plugins/nlri/flowspec` | 4 | 0 |
| `internal/component/bgp/plugins/nlri/labeled` | 4 | 0 |
| `internal/component/bgp/plugins/nlri/srpolicy` | 4 | 0 |
| `internal/core/network` | 4 | 0 |
| `internal/component/bgp/config` | 3 | 0 |
| `internal/component/bgp/plugins/nlri/evpn` | 3 | 0 |
| `internal/component/bgp/wireu` | 3 | 0 |
| `internal/plugins/mrt` | 3 | 0 |
| `test/plugin` | 3 | 0 |
| `internal/component/bgp/grmarker` | 2 | 0 |
| `internal/component/bgp/plugins/filter_community` | 2 | 0 |
| `internal/component/bgp/plugins/route_refresh/handler` | 2 | 0 |
| `internal/core/bgp/context` | 2 | 0 |
| `internal/plugins/isis` | 2 | 0 |
| `internal/component/bgp/plugins/adj_rib_in` | 1 | 0 |
| `internal/component/bgp/plugins/cmd/announce` | 1 | 0 |
| `internal/component/bgp/plugins/filter_modify` | 1 | 0 |
| `internal/component/bgp/plugins/filter_prefix` | 1 | 0 |
| `internal/component/bgp/plugins/filter_remove_private_as` | 1 | 0 |
| `internal/component/bgp/plugins/nlri/vpn` | 1 | 0 |
| `internal/component/bgp/server` | 1 | 0 |
| `internal/component/sysrib` | 1 | 0 |
| `internal/core/bgp/nlri/nlrisplit` | 1 | 0 |
| `internal/plugins/fib/kernel` | 1 | 0 |
| `internal/plugins/flowspec-firewall` | 1 | 0 |
| `test/reload` | 1 | 0 |

A verdict whose units span two of these packages counts in each.

### Derived listing

The verdict inventory is DERIVED. No id list is committed here, because the audit files change as the work proceeds.

| Question | Derived by |
|----------|-----------|
| Every weak or wrong verdict in this child's scope (stem, id, verdict) | `jq -r --arg pk '^(internal/component/bgp/\|internal/core/bgp/\|internal/le/interoplab/bgp/\|internal/mrt/\|internal/plugins/mrt/\|test/plugin/\|test/reload/\|internal/core/network/\|internal/plugins/flowspec-firewall/\|internal/component/sysrib/\|internal/plugins/fib/kernel/rfc9252_nexthop_srv6_linux_test.go\|internal/plugins/isis/rfc9552_router_id_test.go\|internal/plugins/ospf/rfc9552_router_id_test.go)' --arg own '^$' --arg fo '^(rfc5882)$' 'input_filename as $f \| ($f\|ltrimstr("rfc/audit/")\|rtrimstr(".json")) as $s \| .requirements \| to_entries[] \| select(.value.verdict=="weak" or .value.verdict=="wrong") \| select(.key\|IN("RFC1071-1-4","RFC4303-2.1-1","RFC5882-4.4-1","RFC905-x-3","RFC905-x-4")\|not) \| select((($s\|test($own)) or ([(.value.tests // {})\|keys[]\|test($pk)]\|any)) and ($s\|test($fo)\|not)) \| "\($s)\t\(.key)\t\(.value.verdict)"' rfc/audit/*.json` |
| What the arguments mean | `pk` matches a tagged test path in this child's packages; `own` adds this child's owned stems tagged from another child's packages; `fo` drops stems another child owns (`^$` matches none); the five ids are the parent's cross-group verdicts |

**Wiring, 2026-09-28, at `cb4801b4cc`:** the filter returns 346 weak and 14 wrong, 360 in all, over 60 stems. This equals the Task figure, so `fefe38b68c` (it re-judged only the five cross-group verdicts, which the filter drops) and `cb4801b4cc` changed no count here. `./le rfc audit-stamp` accepts `mode rejudge` (A-1). The "Blocked by" verdicts are now acceptance criteria of their blocking specs: `spec-bgp-prefix-sid-rfc-defects` AC-6, `spec-bgp-graceful-restart-rfc-defects` AC-4, `spec-bgp-open-session-rfc-defects` AC-4, `spec-bgp-update-propagation-rfc-defects` AC-5, `spec-bgp-sr-policy-rfc-defects` AC-4.

### Owned stems

One writer per ledger file: this child alone writes `rfc/audit/<stem>.json`, `rfc/discrimination/<stem>.json`, `rfc/short/<stem>.md`, `rfc/corrections/<stem>.md` and `rfc/extraction/<stem>.json` for these stems.

| Stem | Weak + wrong (2026-09-28) | Of which wrong |
|------|--------------|----------------|
| draft-abraitis-idr-addpath-paths-limit | 1 | 0 |
| draft-ietf-bess-mup-safi | 9 | 0 |
| draft-ietf-idr-bgp-bfd-strict-mode | 3 | 0 |
| draft-ietf-idr-linklocal-capability | 8 | 0 |
| draft-ietf-sidrops-aspa-verification | 1 | 0 |
| rfc2385 | 5 | 0 |
| rfc2918 | 4 | 0 |
| rfc4271 | 58 | 3 |
| rfc4360 | 2 | 0 |
| rfc4456 | 1 | 0 |
| rfc4659 | 2 | 0 |
| rfc4684 | 1 | 0 |
| rfc4724 | 14 | 0 |
| rfc4760 | 4 | 1 |
| rfc5082 | 1 | 0 |
| rfc5492 | 5 | 0 |
| rfc5549 | 3 | 0 |
| rfc5575 | 4 | 0 |
| rfc6286 | 2 | 0 |
| rfc6396 | 5 | 0 |
| rfc6793 | 4 | 0 |
| rfc6810 | 4 | 0 |
| rfc6811 | 1 | 0 |
| rfc6996 | 1 | 0 |
| rfc7311 | 6 | 1 |
| rfc7313 | 4 | 0 |
| rfc7432 | 3 | 0 |
| rfc7606 | 4 | 0 |
| rfc7611 | 1 | 0 |
| rfc7705 | 3 | 0 |
| rfc7752 | 8 | 1 |
| rfc7854 | 10 | 2 |
| rfc7911 | 7 | 2 |
| rfc7947 | 2 | 0 |
| rfc7999 | 6 | 0 |
| rfc8050 | 4 | 1 |
| rfc8092 | 6 | 0 |
| rfc8203 | 3 | 0 |
| rfc8210 | 10 | 0 |
| rfc8277 | 8 | 0 |
| rfc8654 | 5 | 0 |
| rfc8669 | 10 | 0 |
| rfc8671 | 1 | 0 |
| rfc8950 | 3 | 0 |
| rfc8955 | 3 | 1 |
| rfc8956 | 1 | 0 |
| rfc9003 | 1 | 0 |
| rfc9012 | 15 | 1 |
| rfc9069 | 5 | 0 |
| rfc9072 | 3 | 0 |
| rfc9085 | 3 | 0 |
| rfc9086 | 4 | 0 |
| rfc9136 | 1 | 0 |
| rfc9234 | 9 | 0 |
| rfc9252 | 7 | 0 |
| rfc9494 | 8 | 0 |
| rfc9514 | 7 | 0 |
| rfc9552 | 18 | 1 |
| rfc9687 | 3 | 0 |
| rfc9830 | 25 | 0 |
| draft-ietf-sidrops-8210bis | 0 in `rfc/audit/` | un-enrolled: its three R-7 rows and the two missing 5.12 rows; no audit file |
| rfc9256 | 0 in `rfc/audit/` | split rows 4-2 and 4-3; no weak or wrong verdict |
| rfc9582 | 0 in `rfc/audit/` | the source of the 5.12-2 and 5.12-7 moves to 8210bis, through `spec-rfc-requirement-reattribution` |
| rfc5701 | 0 | narrowing audit row RFC5701-2-4; no weak or wrong verdict |

Stems tagged in this child's packages that another child owns:

| Stem | Owner | Tags here |
|------|-------|-----------|
| rfc5882 | BFD | the tag of `TestStrictPeerRequestReachesTheSharedKey` in `reactor/config_bfd_strict_test.go` belongs to RFC5882-4.4-1, a cross-group verdict the parent holds |

Test files that carry tags of two owners. A child edits one only while the other holds no uncommitted change in it:

| File | Shared by |
|------|-----------|
| `internal/component/bgp/reactor/config_bfd_strict_test.go` | this child (DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE tags) and BFD, which owns rfc5882 (the RFC5882-4.4-1 tag, a parent verdict) |

### Un-enrolled R-7 rows

Copied verbatim from the parent. A row leaves this table when its re-judged verdict is `enforced`, recorded here with the date, or in the stem's audit file if the stem is enrolled first.

| ID | Verdict | Unit | What the test fails to prove |
|----|---------|------|------------------------------|
| DRAFT-IETF-SIDROPS-8210BIS-5.12-1 | enforced (independent re-judge, 2026-09-29) | `TestASPAEmptyAnnouncementDrawsErrorReport9` (`internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go`), beside `TestParseASPAPDU`, `TestParseASPAPDUMalformed` | was: the Error Report with Error Code 9 was not asserted. Now over TCP: a one-provider announcement syncs with no byte sent back and is attested; a zero-provider announcement draws an Error Report, code 9, encapsulating the PDU, and publishes nothing. Observed-red records in `rfc/discrimination/draft-ietf-sidrops-8210bis.json` |
| DRAFT-IETF-SIDROPS-8210BIS-5.12-3 | enforced (independent re-judge, 2026-09-29) | `TestASPAProviderListOfZeroOrMoreUniqueAscending` (`rtr_session_rfc8210_test.go`), beside `TestParseASPAPDU`, `TestParseASPAPDUUnsorted` | was: "zero or more" had no tagged assertion. Now a zero-provider withdrawal is accepted and applied, a unique ascending list is accepted, and a repeated or descending list fails with `errASPAProviderList` and publishes nothing |
| DRAFT-IETF-SIDROPS-8210BIS-7-2 | enforced (independent re-judge, 2026-09-29) | `TestRTRUnknownVersionErrorReportIsNotAnswered` (`rtr_session_rfc8210_test.go`), beside `TestRTRUnknownNegotiationVersion` | was: the Error Report exception was not asserted. Now a version 99 Error Report with code 0 or 4 fails the sync and the cache reads EOF with no byte before it |

### Split-needed rows

Copied verbatim from the parent. The parent's narrowing audit adds rows for this group, and this child takes them when that output is committed. "Row correction" marks a claim no sentence states.

| ID | Dropped obligation | Section |
|----|--------------------|---------|
| RFC7752-3.3.1.3-1 | Link Name: use of the FQDN or a subset of it is strongly RECOMMENDED | §3.3.2.7 |
| RFC9552-5.3.1.3-1 | Link Name: use of the FQDN or a substring of it is strongly RECOMMENDED | §5.3.2.7 |
| RFC9552-5.3.2.2-3 | the sentence before Table 9: reserved bits MUST be set to zero and SHOULD be ignored on receipt | §5.3.2.2 |
| DRAFT-IETF-BESS-MUP-SAFI-3.3.3-4 | DSD nexthop/locator mismatch is a malformed NLRI, treat-as-withdraw | §3.3.6 |
| RFC7854-x-3 | per-peer header follows the common header for Route Monitoring, Route Mirroring, Stats and Peer Up; Initiation and Termination carry the common header only | §4.3, §4.5, §4.6, §4.7, §4.8, §4.10 |
| RFC7854-x-2, x-4, x-10 | octet widths stated only by the figures (6-octet total, 4-octet field, 1-octet Reason): row correction | §4.1, §4.2, §4.9 |
| RFC9256-4-2 | SRv6 SID behavior and structure MAY be provided, for types I, J and K | §4 |
| RFC9256-4-3 | SR Algorithm MAY be provided, for types I, J and K | §4 |
| RFC8277-2.2-3 | Rsrv MUST be ignored on reception, multi-label encoding | §2.3 |
| RFC8277-2.2-4 | Rsrv SHOULD be set to zero on transmission, multi-label encoding | §2.3 |
| RFC8277-3.2.1-4 | after a next-hop change, a route not propagated MUST be withdrawn from that peer | §3.2.2 |
| RFC8669-4-2 | SHOULD log an error when discarding an attribute: the malformed and the invalid case | §6, §4.1 |
| RFC9069-5.2-1 | each emulated peer MUST send a Peer Up whose OPEN indicates its address-family capabilities | §6.1.1 |
| RFC4659-8-3 | over IPv6 tunnelling, the BGP Next Hop SHALL contain an IPv6 address | §8 |
| RFC9085-2.1-1 | the same sentence for the Link NLRI and the Prefix NLRI | §2.2, §2.3 |
| RFC9085-2.1.2-3 | Reserved ignored on receipt, in the other TLVs | §2.1.4, §2.2.1, §2.2.2, §2.3.1, §2.3.5 |
| RFC9085-2.1.2-4 | the flags sentence of the other TLV | §2.1.4 |

### Missing rows

| Stem | Missing obligation | Section |
|------|--------------------|---------|
| rfc9582 / 8210bis | the self-provider prohibition (RFC9582-5.12-2) and the multi-provider AS 0 MUST NOT (RFC9582-5.12-7) need their own 8210bis rows | 8210bis §5.12 |
| rfc8210 | SHOULD: "If the router has never issued a successful query against a particular cache, it SHOULD retry periodically using the default Retry Interval, above." | §6 |
| rfc8210 | SHOULD, cache side: the cache "SHOULD reject the connection if none of the iPAddress identities match the connection." (RFC8210-9.2-3 quotes only the MUST check before it) | §9.2 |
| rfc8210 | MAY: "host authentication MAY be supported. Implementations MAY support password authentication." (RFC8210-9.1-2 quotes only the user-authentication MUST before it) | §9.1 |

### Row-quality corrections

| Kind | Rows | What is wrong | Correction owed |
|------|------|---------------|-----------------|
| bare-pronoun quote | RFC9012-3.1-3 ("it MUST be propagated unchanged"), 3.1-2 ("It MUST be disregarded"), 3.2.1-1 ("They MUST"), 3.7-2, 4.2-1, 4.3-2 ("the value") | the subject, such as the Reserved subfield, is in the sentence before | widen the span to the sentence that names the subject |
| level over a stronger keyword | RFC9012-11-7 | levelled SHOULD, but its quote also carries "MUST be able to filter the attribute from outgoing BGP UPDATE messages" | split the MUST into its own row, or re-level |
| level with no BCP 14 keyword | RFC7296-1.2-1, 2.6-1, 2.9-1, 2.23-3, 2.23-12, 2.4-1, 1.4-1, 2.8-2 (MUST NOT), 2.2-3; RFC4301-4.1-4, 7-1 (MUST NOT); RFC3748-4-2, 2-2, 4.2-1; RFC2759-x-3, x-10, x-12 (format and vector text); RFC3768-6.4.3-9 (pseudocode); RFC5301-3-8 (MUST NOT over "The string is not null-terminated."); RFC8050-4.2-1 (descriptive); RFC8050-x-4 (rationale) | D-3 permits a MUST level over a normative sentence without a keyword, so these are for review, not automatic demotion. SB-2 counted 17 of them in its four stems, the blind samples 4 more | review each; a demotion needs a correction paragraph per stem and an owner call on the format-definition rows |
| lowercase keyword | RFC8092-4-1 ("should") | levelled as a BCP 14 keyword | review the level |
| duplicate span | RFC8210-7-8 and RFC8210-5.2-1 | the §7 "The router MUST ignore any Serial Notify PDUs ... during this initial startup period" is the obligation 5.2-1 states, and 5.2-1 already cites §7 | merge |
| text after the cite, or a wrapped hyphen | RFC3630-1-1 (prose after the section cite); DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-1 ("strict- mode"), RFC7684-5-1 ("sub- TLV") | the prose is not RFC text, and the hyphen split is the RFC's line wrap carried into the quote | move the prose out of the quote; join the hyphen |

- The row starting `level with no BCP 14 keyword`: this child owns RFC8050-4.2-1 and RFC8050-x-4 only; the other ids go to the child that owns their stem.
- The row starting `text after the cite, or a wrapped hyphen`: this child owns DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-1 only; the other ids go to the child that owns their stem.

### Mistagged units

| Unit | Tagged under | What it proves instead | Correction owed |
|------|--------------|------------------------|-----------------|
| `internal/component/bgp/plugins/rpki/rtr_session_test.go::TestCacheResetTriggersResetQuery` | RFC8210-8.3-1 | the tag prose says "ze runs every configured cache in parallel", but `cacheGroup` is preference-ordered since 2026-09-20 | correct the tag prose, and re-judge the unit against the more-preferred-cache SHOULD of the quote |

### Deferred items the parent's tables missed

| Item | What is owed | Group |
|------|--------------|-------|
| RFC9552-5.1-2 and `nlri/ls/types_descriptor.go` | the verdict is `wrong` (the units assert the forbidden repeated 518 encoding), and the comment claiming 518 is "the only sub-TLV a descriptor can repeat" is false per RFC 9514 Section 6 | BGP |
| R-7 detail lost in condensing | RFC1035-4.1.4-1: the negative's comment claims "two zero bits" but asserts only "no 0xC0 octet". RFC9190-5.4-3: the claim that crypto/tls turns the error into bad_certificate is not asserted. RFC9190-5.7-1: `TestEAPTLS13CompletesAResumptionWithAnUnrevokedChain` proves the valid-cache positive but carries only the 5.7-2 and 5.7-6 tags. DRAFT-8210BIS-7-2: the guard is `hdr.Type != pduErrorRpt` in `RTRSession.readLoop`. DRAFT-8210BIS-5.12-1: the Error Code 9 mapping is in `readLoop` | services, IKE/EAP, BGP |

- The row starting `R-7 detail lost in condensing`: this child owns DRAFT-8210BIS-7-2 and DRAFT-8210BIS-5.12-1 only; the other ids go to the child that owns their stem.

### Narrowing audit rows (parent, 2026-09-28)

Copied from the parent's "Narrowing audit findings, 2026-09-28", where the process and the call meanings are. AC-C3 includes these rows.

| ID | Call | Obligation | Section |
|----|------|------------|---------|
| DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-4 | dropped | "Otherwise the NLRI is considered as a malformed. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]." | §3.1.3.1 |
| DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-2 | dropped | "Otherwise the NLRI is considered as a malformed. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]." | §3.1.4.1 |
| DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3 | dropped | "When a BGP speaker receives a MP_REACH_NLRI attribute update message with a Direct Segment Discovery route without a prefix SID attribute, than it MUST be treated as if it contained a malformed prefix SID attribute and the "Treat-as-withdraw procedure of [RFC7606] is applied." | §3.3.6 |
| RFC4271-6.1-3 | dropped | "The Data field MUST contain the erroneous Length field." | §6.1 |
| RFC4271-8.2.2-13 | dropped | "If the local system receives a TcpConnectionFails event (Event 18), the local system: ... - increments the ConnectRetryCounter by 1," | §8.2.2 (Active state) |
| RFC4271-8.2.2-15 | dropped | "In response to any other event (Events 9, 12-13, 20-22), the local system: - sends a NOTIFICATION message with the Error Code Finite State Machine Error, - deletes all routes associated with this connection, - sets the ConnectRetryTimer to zero, - releases all BGP resources, - drops the TCP connection, - increments the ConnectRetryCounter by 1" | §8.2.2 |
| RFC4271-8.2.2-8 | dropped | "In response to a ManualStop event (Event 2), the local system: ... - sets ConnectRetryCounter to zero," | §8.2.2 (Connect, Active; also OpenConfirm, Established) |
| RFC4271-9.1.2.1-5 | retire | No RFC 4271 sentence says connection establishment failure SHOULD be logged; quote is a different obligation (mutual-recursion logging). | none |
| RFC4271-9.2.2.2-2 | dropped | "Otherwise, if at least one route among routes that are aggregated has ORIGIN with the value EGP, then the aggregated route MUST have the ORIGIN attribute with the value EGP." | §9.2.2.2 |
| RFC4360-x-1 | moved | stated by RFC7606 7.14, not by this document: "The Extended Community attribute SHALL be considered malformed if its length is not a non-zero multiple of 8.". RFC 4360 only states 8-octet encoding (quote); explicit length multiple-of-8 rule is RFC 7606. | RFC7606 7.14 |
| RFC4456-8-3 | retire | Old MUST NOT create unless originator in local AS has no RFC sentence; quote is different SHOULD NOT create if one exists. | none |
| RFC4659-3.2.1.1-1 | dropped | "When the IPv6 VPN traffic is to be transported to the BGP speaker using IPv6 tunneling (e.g., IPv6 MPLS LSPs, IPsec-protected IPv6 tunnels), the BGP speaker SHALL advertise a Next Hop Network Address field containing a VPN-IPv6 address - whose 8-octet RD is set to zero, and - whose 16-octet IPv6 address is set to the global IPv6 address of the advertising BGP speaker." | §3.2.1.1 |
| RFC4659-3.2.1.1-2 | dropped | "The link-local address shall be included in the Next Hop field if and only if the advertising BGP speaker shares a common subnet with the peer the route is being advertised to [BGP-IPv6]." | §3.2.1.1 |
| RFC5492-5-1 | dropped | "Each such capability is encoded in the same way as it would be encoded in the OPEN message." | §5 |
| RFC5701-2-4 | moved | stated by RFC4360 6, not by this document: "If a route has a non-transitivity extended community, then before advertising the route across the Autonomous System boundary the community SHOULD be removed from the route.". RFC 5701 only defines the 0x40 bit; the propagation rule is RFC 4360 Section 6 | RFC4360 6 |
| RFC7311-3-2 | retire | No RFC 7311 sentence requires attribute length consistent with TLVs; quote is a TLV-set definition. | none |
| RFC7432-11.2-7 | dropped | "The re-advertised routes MUST be the same as the original ones, except for the PMSI Tunnel attribute and the label carried in that attribute." | §11.2 |
| RFC7432-8.1.1-2 | retire | No RFC sentence requires ESI Label on the ES route; quote is the ES-Import RT obligation, a different one. | none |
| RFC7432-8.2.1-4 | dropped | "This label MUST be a downstream assigned MPLS label if the advertising PE is using ingress replication for receiving multicast, broadcast, or unknown unicast traffic from other PEs." | §8.2.1 |
| RFC7947-x-6 | retire | No RFC 7947 sentence permits ADD-PATH use; 2.3 is informative; quote carries no obligation. Nearest: ADD-PATH should enforce send-only mode (2.3.2.2.2). | none |
| RFC8955-4.2.2.4-1 | dropped | "Type 5 component values SHOULD be encoded as 1- or 2-octet quantities (numeric_op len=00 or len=01). \| Type 6 component values SHOULD be encoded as 1- or 2-octet quantities (numeric_op len=00 or len=01). \| Type 10 component values SHOULD be encoded as 1- or 2-octet quantities (numeric_op len=00 or len=01)." | §4.2.2.5; 4.2.2.6; 4.2.2.10 |
| RFC8955-4.2.2.7-1 | dropped | "Type 8 component values SHOULD be encoded as single octet (numeric_op len=00)." | §4.2.2.8 |
| RFC8955-6-2 | moved | stated by RFC 9117 Section 4.2, not by this document. Leftmost-ASN-matches-best-match-unicast rule is RFC 9117 Section 4.2 (text not in rfc/); quote is RFC 8955 neighbor-AS rule. | RFC 9117 Section 4.2 |
| RFC8955-7.1-4 | dropped | "A traffic-rate-packets of 0 should result in all traffic for the particular flow to be discarded." | §7.2 |
| RFC9012-15-1 | dropped | "This implies that the duty to filter external traffic extends to all routers participating in such tunnels." | §15 |
| RFC9012-3.2.4-1 | dropped | "Unless a key value is being advertised, the MPLS-in-GRE Encapsulation sub-TLV MUST NOT be present." | §3.2.5 |
| RFC9136-3.1-4 | dropped | "It MUST be all bytes zero otherwise." | §3.1 |
| RFC9494-4.2-1 | dropped | "The interval for which they are retained is limited by the sum of the Restart Time in the received Graceful Restart Capability and the Long-Lived Stale Time in the received Long-Lived Graceful Restart Capability." | §4.2 |
| RFC9830-5-1 | dropped | "This includes the validation of the length of each NLRI and the total length of the MP_REACH_NLRI and MP_UNREACH_NLRI attributes. It also includes the validation of the consistency of the NLRI length with the AFI and the endpoint address as specified in Section 2.1." | §5 |

### Blocked by

Each id below was checked on 2026-09-28: a weak or wrong verdict in `rfc/audit/<stem>.json`, or a row of the R-7 table above. The blocking spec owns the producer fix (AC-C4), and the verdict moves into its acceptance criteria (P-3).

| ID | Verdict | Blocking spec |
|----|---------|---------------|
| RFC8669-6-1 | weak | `spec-bgp-prefix-sid-rfc-defects` |
| RFC9252-7-2 | weak | `spec-bgp-prefix-sid-rfc-defects` (AC-7, 2026-09-30) |
| RFC8669-3.1-2 | weak | `spec-bgp-prefix-sid-rfc-defects` (AC-7, 2026-09-30) |
| RFC8669-3.2-4 | weak | `spec-bgp-prefix-sid-rfc-defects` (AC-7, 2026-09-30) |
| RFC9252-3.4-1 | weak | `spec-bgp-prefix-sid-rfc-defects` |
| RFC9252-3.2.1-3 | weak | `spec-bgp-prefix-sid-rfc-defects` |
| RFC9252-5-1 | weak | `spec-bgp-prefix-sid-rfc-defects` |
| RFC9252-7-1 | weak | `spec-bgp-prefix-sid-rfc-defects` |
| RFC4724-4-2 | weak | `spec-bgp-graceful-restart-rfc-defects` |
| RFC9494-4.2-6 | weak | `spec-bgp-graceful-restart-rfc-defects` |
| RFC9494-4.2-8 | weak | `spec-bgp-graceful-restart-rfc-defects` |
| RFC9494-4.3-1 | weak | `spec-bgp-graceful-restart-rfc-defects` |
| RFC6286-2.1-1 | weak | `spec-bgp-open-session-rfc-defects` |
| RFC9494-4.3-3 | weak | `spec-bgp-graceful-restart-rfc-defects` (the current audit retains this finding; the prior prose below incorrectly called it enforced) |
| RFC9072-2-1 | weak | `spec-bgp-open-session-rfc-defects` |
| RFC4271-4.3-3 | weak | `spec-bgp-update-propagation-rfc-defects` |
| RFC4271-4.3-4 | weak | `spec-bgp-update-propagation-rfc-defects` |
| RFC8092-3-1 | weak | `spec-bgp-update-propagation-rfc-defects` |
| DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-6 | weak | `spec-bgp-update-propagation-rfc-defects` |
| DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-7 | weak | `spec-bgp-update-propagation-rfc-defects` |
| DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-10 | weak | `spec-bgp-update-propagation-rfc-defects` |
| RFC9234-3.1-1 | weak | `spec-bgp-update-propagation-rfc-defects` |
| RFC7999-3.1-2 | weak | `spec-bgp-update-propagation-rfc-defects` |
| DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-1 | weak | `spec-bgp-update-propagation-rfc-defects` D6 / AC-5 (2026-10-02: received link-local-only next hop leaks across multihop egress; existing-generation withdrawal also owed) |
| DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-9 | weak | `spec-bgp-update-propagation-rfc-defects` D6 / AC-5 (same red probe, external multihop case; global-address control passes) |
| RFC9830-4.2.1-2 | weak | `spec-bgp-sr-policy-rfc-defects` |
| RFC9830-2.4.2-6 | weak | `spec-bgp-sr-policy-rfc-defects` (D1: its quote is the row text verbatim, though the spec says "no row names it") |
| RFC9830-4.2.1-7 | weak | `spec-bgp-sr-policy-rfc-defects` AC-5 (§4.2.1 receive validation: attribute 23 has no validator) |
| RFC7854-x-10 | weak | `spec-bmp-sflow-export-rfc-defects` AC-4 (§4.9 cause-to-reason tie in `peerDownFor`; reason 4 is its D1) |
| RFC7854-4.9-1 | weak | `spec-bmp-sflow-export-rfc-defects` AC-5 (§4.9 reason 2 FSM event code: the event must reach `peerDownFor` across `rpc.StructuredEvent`; lands with D1) |
| RFC4271-9.2-7 | weak | `spec-bgp-rs-replacement-on-withdrawal` AC-3 (R57(b), 2026-10-01: bgp-rs relays a source's withdrawal while another source's route for the prefix remains; the fix needs a per-source path store) |
| DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT-3-7 | weak | `spec-add-path-limit-send-receive` AC-21 (BGP c31, 2026-10-02: the unmet half is the ingress ceiling, "the maximum number of paths to accept from a sender"; no code drops a received path above PathsLimitRecv, and that spec builds the ceiling) |
| RFC8669-6-3 | weak | `spec-bgp-prefix-sid-rfc-defects` AC-8/AC-9 (owner decision, 2026-10-02: discard duplicate recognized single-occurrence TLVs on relay, including types 5 and 6; preserve the existing red probe) |
| RFC9252-5-3 | weak | `spec-vpp-srv6-service-policy` AC-1..AC-7 (owner decision, 2026-10-02: VPP needs an encapsulating SR policy and locally allocated binding SID before prefix steering; preserve the existing red probe) |

- RFC9494-5-2 is a `{gap}` with no verdict in `rfc/audit/rfc9494.json`, so it is not in the derived listing and not a blocked verdict; owner ruling 8 (e) homes it in `spec-bgp-llgr-per-family-config` AC-6.
- RFC8277-2.5-3 is a `{gap}` with no weak or wrong verdict; `spec-bgp-addpath-best-path-per-prefix` AC-6 owns it.
- RFC9494-4.3-2 has no verdict in `rfc/audit/rfc9494.json`. RFC9494-4.3-3 remains `weak` and is listed above. The graceful-restart spec owns both producer defects.
- RFC9252-5-2 is `unimplemented`, not weak or wrong.
- RFC 2545 Section 3 (update-propagation D6) and RFC 7854 Section 4.9 reason 4 (bmp-sflow D1) have no row. RFC7854-4.9-1 is explicitly blocked by the BMP spec's AC-5 for its reason 2 FSM event code. The committing spec also renews records changed by the shared `peerDownFor` repair (R-6 of the parent).
- RFC 9830 Section 2.4.2 D2 (the BSID label width) has no row; its producer change stales the srpolicy records the same way.

## Required Reading

### Architecture Docs
- [ ] `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` - the Method table, the Required Reading constraints, D-2 to D-15, and the source tables above
  → Constraint: every row stays a verbatim span of its cited section, at least 24 characters, never across two sections (`checkRowQuotes`); a span that must cross a section is split into two rows
  → Constraint: widening, narrowing or re-citing a row stales its verdict (`stale-requirement`) until it is re-judged in the same commit; a new id takes n above the HEAD^ high-water mark of its section (`checkIDAllocation`)
  → Constraint: a new gated row lands with both polarity tags or an annotation, a discrimination record for each new cover (a moved tag is a new cover), and an extraction site mapped to it or listed in `unsourced-ids`
  → Constraint: an upgrade from weak or wrong to `enforced` with unchanged units needs an `upgrade_reason` (`checkAuditFindings`), so author and judge land in one commit where they can
  → Constraint: editing any function in a test file shifts every verdict tagging that file; `./le rfc reseal` is corpus-wide and rewrites other children's audit files, so a reseal runs only when no other child holds uncommitted audit changes
  → Constraint: the bgp/reactor link-local tests fail without `fd00::2` on loopback: a journal row, not an RFC 2545 regression (parent mistake log)
  → Constraint: the rfc9582 to 8210bis moves and the two missing 5.12 rows go only through the route `spec-rfc-requirement-reattribution` settles (parent overlap table)
  → Constraint: draft-ietf-sidrops-8210bis is not enrolled, so `audit-stamp` refuses it: its R-7 re-judgements are recorded in this spec's R-7 table with the date
  → Constraint: RFC9552-5.1-2 is `wrong` because its units assert the repeated 518 encoding RFC 9552 Section 5.2.1.4 forbids; the comment in `nlri/ls/types_descriptor.go` claiming 518 is "the only sub-TLV a descriptor can repeat" is false per RFC 9514 Section 6 and is corrected in the same phase (parent "Dropped after reading")
  → Constraint: `./le rfc check` carries 18 `producer-changed` records from other sessions (rfc4271 6.3-15, rfc7611 2.1-1, rfc7705 3.3-1, rfc7947 2.2.2.2-1, and rfc8907): this child answers only for the violations its commits add
  → Constraint: D-13: softver `legacy` is an owner-approved deviation, not a defect
- [ ] `docs/contributing/rfc-conformance-gates.md` - the discrimination record, the row quote, the refusals
- [ ] `ai/skills/ze-rfc-audit.md` - the four judgement questions, the verdict vocabulary, STRICTNESS, and the re-judge route
  → Decision: judge strictly: a floor assertion, a confounded buffer or a positive that proves only "no error" stays `weak`

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/<stem>.md` for each owned stem above - the row text a verdict is judged against; the RFC text in `rfc/full/<stem>.txt` is the authority for every split and correction
  → Constraint: a claim that an obligation was dropped quotes the RFC sentence with its section (parent R-9)

**Key insights:**
- 360 verdicts over 43 packages and 63 stems, plus 33 rows copied from the parent's tables
- The listing is derived with the filter above, never copied; each stem's ledger files belong to one child
- A verdict whose producer another spec fixes is listed under "Blocked by" and leaves this child through P-3

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `plan/pre-release/spec-rfc-verdict-test-fix-pass.md` - the parent: Split section, inventory tables, Method, Required Reading, AC-C1 to AC-C7
- [ ] `rfc/audit/rfc4271.json` - weak and wrong verdicts with their `tests` maps and notes
- [ ] `rfc/audit/rfc9830.json` - weak and wrong verdicts with their `tests` maps and notes
- [ ] `rfc/audit/rfc9552.json` - weak and wrong verdicts with their `tests` maps and notes
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

### MRT D-8 amendment (owner decision, 2026-10-02)

The owner selected context-aware parsing now. RFC 8050 Sections 2, 3 and 5.1
signal ADD-PATH with one whole-message subtype; they do not encode a per-family
map. RFC 7911 Section 5 negotiates each direction and family using both OPENs.
This repair preserves the captured message, not a reconstruction of its routes.

Data flow: the Session's complete inbound wire boundary, before semantic
validation/coalescing, and successful outbound transport writes feed a separate
synchronous raw observer. The existing semantic MessageCallback remains the
route-delivery boundary, including synthesized withdrawals. MRT writes one
unchanged complete BGP message per record and native LOCAL subtypes for sent
messages. Directional encoding contexts are immutable; neither observation nor
subtype selection builds a per-message capability map. Buffered outbound writes
are observed after transport acceptance, without forcing per-message flushes;
only split frames need bounded temporary storage, owned by that connection.

Each offline file/read invocation owns bounded context keyed by peer/local
endpoints and interface, with separate actual OPENs per direction. Capability
parsing/negotiation uses the existing core capability/family definitions. Actual
OPEN/AS4 identities must match subsequent record identities. A new OPEN epoch,
NOTIFICATION or teardown clears context; the recorder's post-handshake
Idle-to-Established notification preserves the just-observed handshake. Missing
OPENs, independently rotated files, mid-session starts and identity reuse never
inherit guessed state. Overflow is an explicit error, not silent eviction.

`ParseBGPMessage(record.BGPMessage)` remains the context-bearing API, borrowing
the original bytes. Ordinary records and ADD-PATH records with one unambiguous
family remain readable without OPENs. Multiple-family ADD-PATH records without
both OPENs return an explicit unavailable/ambiguous-context error, never a
heuristic decode. Semantic CLI consumers carry context and errors through their
real entry points; raw/header-only transforms may retain opaque bytes.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-MRT1 | Actual received/sent messages, including directional OPENs, consecutive/coalesced messages, rejected/dropped UPDATEs and treat-as-withdraw synthesis | Exactly one byte-identical complete original message per MRT record; no synthesized receive messages; sent records use LOCAL subtypes. Partial transport writes publish only complete accepted frames, with connection-specific context and no callback lock inversion. |
| AC-MRT2 | Both actual OPENs, one-sided/repeated OPEN, teardown/reconnect, peer/ASN reuse, opposite direction, another file, or bounded context exhaustion | Only the valid epoch's direction/family intersection supplies decoding context; no configured-capability substitute, cross-peer/file leakage, silent eviction or stale negotiation. |
| AC-MRT3 | Classic ordinary plus MP ADD-PATH, and the reverse, using distinct announcement/withdrawal prefixes | With OPEN context, decode exact prefixes, Path Identifiers and counts in all four NLRI locations. The independent mixed-distinct judge fixture without OPENs reports unavailable context. Same-mode legacy fixtures cannot imply recovery of arbitrary mixed captures. |
| AC-MRT4 | show/density/filter/statistics and other semantic consumers; inject/replay/serve | Every caller preserves context and reports undecodable content as an error/nonzero exit or an explicit incomplete contract, never a silent filter nonmatch or complete-looking partial count. Replay refuses unsupported Path-ID-bearing or ambiguous UPDATEs before transmission; ordinary UPDATEs and harmless empty EOR remain supported without a new replay negotiation feature. |
| AC-MRT5 | RFC6396-1-1 and RFC6396-4.4.2-2 proof carriers | Independent literal bytes pin ET microseconds, old/new FSM states, RIB_GENERIC AFI and legacy TABLE_DUMP numeric fields; consecutive differently sized BGP messages assert exact per-record framing and total record count. Existing raw-data/UTF-8 coverage remains. |
| AC-MRT6 | RFC8050-x-3 roundtrip carrier and every added/changed RFC claim or producer | Correct the Path-Identifier claim/fixture mismatch without losing assertions or claim coverage; record native discrimination for changed/new claims and stale affected producers, never handwritten packet evidence or fingerprints. |
| AC-MRT7 | Public API/architecture/support surfaces and independent rereview | Documentation accurately distinguishes recoverable and ambiguous captures and replay refusal; every caller is migrated without raw-byte compatibility wrappers. Main consolidates formatting/tests/native aggregate checks and assigns an independent judge before closure. |


## Method

The parent's Method table governs every phase and is not copied here. The rules that bite this group are the constraints under Required Reading above, plus: one agent per package and never two on one test file; strict judgement; every clause in both polarities; a discrimination record for every tag added or changed; an independent judge for every re-judged verdict; a D-8 defect fixed failing-test-first, with interop where a peer exists.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | the parent's `mode rejudge` has landed before the first re-judge here | parent Implementation Steps 1 and 2 | a re-judge has no route but hand deletion, which P-2 bans | native `audit-stamp mode rejudge` stamped both RFC8050 findings in `14698ed9c4` | confirmed 2026-10-05 |
| A-2 | the blocked-by list is complete for this scope | the parent's code-defect and overlap tables, each id checked in `rfc/audit/` or the R-7 table on 2026-09-28 | the child cannot reach AC-C1 | each package's author agent names any verdict it cannot move without a producer fix another spec owns | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | a test fix exposes a defect that grows past the package | a code change outside the package | D-8: fix it; where impossible, back to the owner |
| R-2 | another child or spec commits while this child holds uncommitted hunks in a shared test file, or runs a reseal | a commit diff shows another owner's ids | the shared-file rule above; reseal and interop `revert` runs are serialized between children |
| R-C1 | six BGP code-defect specs change producers this child's tests tag (reactor, rib, message, srpolicy, bmp) | `producer-changed` in a BGP stem after another spec's commit | the committing spec re-records (parent R-6); this child re-judges after the fix lands (AC-C4) |
| R-C2 | `internal/component/bgp/reactor` (75 verdicts) is too large for one agent context | an author agent reaches its call budget mid-package | split the package phase by test file, never two agents on one file (parent R-1) |

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

Inherited from the parent (AC-C1 to AC-C7), restated for the BGP group.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-C1 | the derived listing above | holds only the "Blocked by" ids, each named in its blocking spec's acceptance criteria |
| AC-C2 | every verdict this child moves to `enforced` | its tagged units carry a discrimination record, and an agent other than the test's author judged it |
| AC-C3 | every row of this child in the split-needed and missing-rows tables, including the rows the parent's narrowing audit adds for this group (the "Narrowing audit rows (parent, 2026-09-28)" table) | the dropped obligation is a row of its own, or a dated correction says why not, and the new row carries a verdict |
| AC-C4 | every spec in the parent's code-defect list | this child declares none of their defects and does not fix them; a test whose producer one of them changes waits for, or lands with, that spec |
| AC-C5 | `./le rfc check` after each commit of this child | no violation the commit added, and no stale verdict in this child's stems |
| AC-C6 | every row and unit of this child in the row-quality and mistagged-unit tables | corrected, merged or re-tagged, or a dated correction says why it stands; a changed row or tag is re-judged |
| AC-C7 | every row of this child in "Un-enrolled R-7 rows" that no spec blocks | resolved as the parent's Goal states and re-judged `enforced` by an agent that did not write the test, recorded in that table with the date or in the stem's audit file |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| every unit tagged by a listed verdict | `internal/component/bgp/reactor/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/core/bgp/attribute/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/message/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/plugins/rib/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/plugins/nlri/ls/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/core/bgp/capability/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/plugins/bmp/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/plugins/rpki/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/fsm/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/plugins/gr/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/plugins/nlri/mup/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/plugins/role/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/plugins/ls_export/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/plugins/rib/pool/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/mrt/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/plugins/rib/storage/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/le/interoplab/bgp/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/plugins/nlri/flowspec/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/plugins/nlri/labeled/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/plugins/nlri/srpolicy/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/core/network/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/config/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/plugins/nlri/evpn/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/wireu/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/plugins/mrt/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `test/plugin/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/grmarker/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/plugins/filter_community/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/plugins/route_refresh/handler/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/core/bgp/context/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/plugins/isis/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/plugins/adj_rib_in/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/plugins/cmd/announce/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/plugins/filter_modify/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/plugins/filter_prefix/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/plugins/filter_remove_private_as/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/plugins/nlri/vpn/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/bgp/server/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/component/sysrib/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/core/bgp/nlri/nlrisplit/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/plugins/fib/kernel/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `internal/plugins/flowspec-firewall/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |
| every unit tagged by a listed verdict | `test/reload/*_test.go` | each clause of the quoted sentence in both polarities, with a record from `discriminate-record` | |

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
| named when a D-8 fix is found | `test/interop/scenarios/` | BGP interop scenarios under `test/interop/scenarios/` for any D-8 fix with a peer (FRR, BIRD, GoBGP); none is planned before a defect is found | the fixed behavior against another implementation | |

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
2. **Phase: `internal/component/bgp/reactor`** - 75 verdicts, 0 wrong; procedure P
3. **Phase: `internal/core/bgp/attribute`** - 48 verdicts, 2 wrong; procedure P
4. **Phase: `internal/component/bgp/message`** - 43 verdicts, 0 wrong; procedure P
5. **Phase: `internal/component/bgp/plugins/rib`** - 27 verdicts, 2 wrong; procedure P
6. **Phase: `internal/component/bgp/plugins/nlri/ls`** - 25 verdicts, 2 wrong; procedure P
7. **Phase: `internal/core/bgp/capability`** - 18 verdicts, 2 wrong; procedure P
8. **Phase: `internal/component/bgp/plugins/bmp`** - 16 verdicts, 2 wrong; procedure P
9. **Phase: `internal/component/bgp/plugins/rpki`** - 16 verdicts, 0 wrong; procedure P
10. **Phase: `internal/component/bgp/fsm`** - 12 verdicts, 0 wrong; procedure P
11. **Phase: `internal/component/bgp/plugins/gr`** - 10 verdicts, 0 wrong; procedure P
12. **Phase: `internal/component/bgp/plugins/nlri/mup`** - 9 verdicts, 0 wrong; procedure P
13. **Phase: `internal/component/bgp/plugins/role`** - 9 verdicts, 0 wrong; procedure P
14. **Phase: `internal/component/bgp/plugins/ls_export`** - 8 verdicts, 0 wrong; procedure P
15. **Phase: `internal/component/bgp/plugins/rib/pool`** - 6 verdicts, 0 wrong; procedure P
16. **Phase: `internal/mrt`** - 6 verdicts, 1 wrong; procedure P
17. **Phase: `internal/component/bgp/plugins/rib/storage`** - 5 verdicts, 3 wrong; procedure P
18. **Phase: `internal/le/interoplab/bgp`** - 5 verdicts, 0 wrong; procedure P
19. **Phase: `internal/component/bgp/plugins/nlri/flowspec`** - 4 verdicts, 0 wrong; procedure P
20. **Phase: `internal/component/bgp/plugins/nlri/labeled`** - 4 verdicts, 0 wrong; procedure P
21. **Phase: `internal/component/bgp/plugins/nlri/srpolicy`** - 4 verdicts, 0 wrong; procedure P
22. **Phase: `internal/core/network`** - 4 verdicts, 0 wrong; procedure P
23. **Phase: `internal/component/bgp/config`** - 3 verdicts, 0 wrong; procedure P
24. **Phase: `internal/component/bgp/plugins/nlri/evpn`** - 3 verdicts, 0 wrong; procedure P
25. **Phase: `internal/component/bgp/wireu`** - 3 verdicts, 0 wrong; procedure P
26. **Phase: `internal/plugins/mrt`** - 3 verdicts, 0 wrong; procedure P
27. **Phase: `test/plugin`** - 3 verdicts, 0 wrong; procedure P
28. **Phase: `internal/component/bgp/grmarker`** - 2 verdicts, 0 wrong; procedure P
29. **Phase: `internal/component/bgp/plugins/filter_community`** - 2 verdicts, 0 wrong; procedure P
30. **Phase: `internal/component/bgp/plugins/route_refresh/handler`** - 2 verdicts, 0 wrong; procedure P
31. **Phase: `internal/core/bgp/context`** - 2 verdicts, 0 wrong; procedure P
32. **Phase: `internal/plugins/isis`** - 2 verdicts, 0 wrong; procedure P
33. **Phase: `internal/component/bgp/plugins/adj_rib_in`** - 1 verdicts, 0 wrong; procedure P
34. **Phase: `internal/component/bgp/plugins/cmd/announce`** - 1 verdicts, 0 wrong; procedure P
35. **Phase: `internal/component/bgp/plugins/filter_modify`** - 1 verdicts, 0 wrong; procedure P
36. **Phase: `internal/component/bgp/plugins/filter_prefix`** - 1 verdicts, 0 wrong; procedure P
37. **Phase: `internal/component/bgp/plugins/filter_remove_private_as`** - 1 verdicts, 0 wrong; procedure P
38. **Phase: `internal/component/bgp/plugins/nlri/vpn`** - 1 verdicts, 0 wrong; procedure P
39. **Phase: `internal/component/bgp/server`** - 1 verdicts, 0 wrong; procedure P
40. **Phase: `internal/component/sysrib`** - 1 verdicts, 0 wrong; procedure P
41. **Phase: `internal/core/bgp/nlri/nlrisplit`** - 1 verdicts, 0 wrong; procedure P
42. **Phase: `internal/plugins/fib/kernel`** - 1 verdicts, 0 wrong; procedure P
43. **Phase: `internal/plugins/flowspec-firewall`** - 1 verdicts, 0 wrong; procedure P
44. **Phase: `test/reload`** - 1 verdicts, 0 wrong; procedure P
45. **Phase: split and missing rows** - after the parent commits its narrowing output for this group (parent R-11): each row of the split-needed and missing-rows tables becomes a row of its own with both polarity tags, a record and a verdict, or a dated correction says why not (AC-C3)
   - Verify: `./le rfc check` accepts the new ids, extraction sites and records
46. **Phase: row quality and mistagged units** - each row of those tables corrected, merged or re-tagged, or a dated correction says why it stands; each changed row or tag re-judged (AC-C6)
   - Verify: `./le rfc check` reads no stale verdict in the owned stems
47. **Phase: un-enrolled R-7 rows** - each row not under "Blocked by" resolved and re-judged by an agent that did not write the test, recorded in the R-7 table above with the date (AC-C7)
48. **Phase: closure check** - the derived listing holds only "Blocked by" ids (AC-C1), each named in its blocking spec's acceptance criteria

### Linux continuation, 2026-10-06

The child and parent remain open. The checkpoint and rulings under
`plan/handover/rfc-verdict-test-fix-pass/` still define scope and ownership.
Completed Linux logs and the session state are preserved outside temporary
storage in `/home/thomas/ze-recovery/bgp-20261005T230453160053Z/linux-evidence-20261006-resumed.tar.gz`.

This table preserves the earlier pause checkpoint. Subsequent source and
documentation landing, and the completed AIGP red, are recorded in
`plan/spec-enum-switch-exhaustiveness.md` and its final-proof archive index.
Pending runs and commit boundaries below describe that earlier checkpoint.

| Work | Observed result | Next action |
|------|-----------------|-------------|
| Normalized OPEN capability values | Saved regressions failed before the producer fix; format/GR/RR race tests passed afterward. Independent review accepted the fix. Commit `a02ab8ba399f` passed all five committed-tree compilation flavors. | Preserve the evidence; the LLGR transition workflow also passed80 physical invocations. |
| Initial-sync panic unwind | Bounded regressions exposed retained write and peer locks. The affected forwarding/initial-sync race run passed at count20. Native requests19–27 now record named reds instead of stalled teardown. Commit `847eb69d18ad` passed all five committed-tree compilation flavors. | Preserve the bounded red/green and native discrimination evidence. |
| RR cached forwarding | Both VPN RR workflows passed. Commit `a18e18a6829c` landed the command repair; all five committed-tree compilation flavors passed. | Preserve this evidence; no repeat run owed for this unchanged repair. |
| Forwarding workflows | Six required workflows passed across the recorded runs. The former ModCopy result was vacuous; independently reviewed replacement `852265267893` has exact two-recipient frames and passed all five compilation flavors. | The replacement workflow and its semantic controls remain unexecuted; do not claim the seventh workflow. |
| LLGR physical acceptance | Disk-backed runs stopped at1,5,1 and33 completed invocations. The rebuilt memory-backed `llgr-transition` completed80 physical invocations with unchanged deadlines, MAY_ATTACH=0, parallel2, burners2 and any-failure. The wrapper stopped on stress's exit1, meaning "not reproduced", before the other three workflows. | Accept only the named transition80 result. The other three memory-backed80 runs remain owed at the requested pause. |
| AIGP source cost | The loopback fixture's NEXT_HOP was rejected before AIGP. The container replacement's oracle units and independent review passed. Native execution exposed invalid connect placement, then no passive listener under local ip auto; the wire peer now has a concrete bind address. | Read the current native run after the listener fix. Exact107/withdrawal/111 and original controls must pass before claiming repair. Owner authorized retirement of the obsolete draft and fixture on2026-10-06. |
| OSPF inter-AS oracle | Fresh positive FRR interop and oracle units passed. Commit `96f653df70` carries the independently reviewed oracle. The remote-ID mutation failed during durable boot initialization, before its semantic assertion. | Keep the mutation unproven. Shared interop documentation still needs a safe commit alongside externally owned hunks. |
| Canonical RFC work | Corrected FlowSpec request15, requests19–27 and three MED records landed in `613020ee39`. AC-C3 corrections and four native-stamped first unimplemented judgments landed in `70644a181f`. The full RFC check reported241 violations; two cached summary-count complaints were addressed afterward. | Do not claim a green RFC gate. Pending rejudgments and quality corrections remain open; no absent feature was implemented. |

Owner's pause boundary: finish the current AIGP carrier repair, commit all work
authored in this session, update this plan and write a progress report, then pause.
Do not start another repair lane. This is not spec closure or scope reduction.

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
- [ ] **Commit B:** `remove plan/pre-release/spec-rfc-verdict-fix-bgp.md` only, in the same `./le commit create` script
