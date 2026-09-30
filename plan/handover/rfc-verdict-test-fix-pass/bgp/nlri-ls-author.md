# nlri/ls author (BGP child, package internal/component/bgp/plugins/nlri/ls), first pass

Listing: 25 ids (column 4 = nlri/ls). None under "Blocked by". All re-checked weak/wrong in rfc/audit on 2026-09-30.

| id | resolution | what now proves each clause | records | expected verdict | notes |
|----|------------|-----------------------------|---------|------------------|-------|
| RFC9552-5.1-2 (wrong) | tests | ls_export TestRFC9552NativeLinkTLVsCanonicalOrder (+: two 259, two 261 ascending by Value; each repeated Type fixed Length so Length ties) / TestRFC9552NativeLinkTLVsReorderedSource (-: same-type addresses stored descending emitted ascending). Tags REMOVED from nlri/ls TestNodeDescriptorOrdersRepeatedSRv6SIDs / TestSRv6SIDOrderIsLengthBeforeValue (approved D-15; they assert repeated 518 in a Node Descriptor, which 5.2.1.4 forbids); units kept untagged as re-encode determinism checks; false "518 is the only sub-TLV a descriptor can repeat" comment in types_descriptor.go rewritten | rfc9552 +/- revert encodeNativeLink, observed red | enforced | Length key never differs within a Type on the only producer that repeats a Type; judge should say whether that is enough for the Length clause |
| RFC7752-3.1-3 (wrong) | tests | same two ls_export units (+/-): Value order compared leftmost octet first; with one Length per repeated Type the 7752 and 9552 tie-break rules order this output identically. Tags removed from the two 518 units (they asserted the 9552 Length-first rule against 7752's) | rfc7752 +/- revert encodeNativeLink, observed red | enforced | if the judge wants the Length-regardless clause shown with unequal lengths, no Ze producer emits that: retire under D-7 (successor RFC9552-5.1-2 replaces it) |
| RFC9552-5.1-1 | tests | + the two ls_export units on the native Link producer (ascending 256,257,259,260,261,262,263), beside the existing nlri/ls descriptor units | rfc9552 +/- revert encodeNativeLink | enforced | |
| RFC7752-3.1-2 | tests | same as RFC9552-5.1-1 | rfc7752 +/- revert encodeNativeLink | enforced | |
| RFC9514-3.1-3 | tests | receipt: existing nlri/ls units (+/-). originate: NEW ls_export TestRFC9514OriginatedReservedWordsZero (+) / TestRFC9514PoisonedSourceReservedNeverOriginated (-: source reserved 0xff, wire TLV 1038 Reserved 0) | rfc9514 +/- revert clearOriginatedReserved | enforced | |
| RFC9514-5.1-2 | tests | receipt: existing nlri/ls units. originate: same two ls_export units, TLV 1162 Reserved word 0, other fields intact | rfc9514 +/- revert clearOriginatedReserved | enforced | |
| RFC9514-4.1-2 | tests | receipt: existing nlri/ls TestRFC9514EndXSIDReservedIgnored. originate: NEW isis TestRFC9514ISISEndXOriginatedReservedZero (+) / TestRFC9514ISISEndXAllOnesNeverReachReserved (-: native flags+weight 0xff, TLV 1106 Reserved 0) | rfc9514 +/- revert isis adjacencySIDv6 | enforced | new file in isis package (no other agent holds it; R6) |
| RFC9514-4.2-2 | tests | same, TLV 1107 | rfc9514 +/- revert adjacencySIDv6 | enforced | |
| RFC9086-5-5 | tests + row | receipt: existing nlri/ls TestRFC9086PeerSIDIgnoresReservedFields (+). originate: NEW ls_export TestRFC9086OriginatedPeerSIDRsvdBitsZero (+: source 0xc0 -> wire 0xc0) / TestRFC9086SourceRsvdBitsNeverOriginated (-: source 0xcf -> wire 0xc0). {single-polarity: positive} marker removed from the row (a negative now exists) | rfc9086 +/- revert clearOriginatedReserved | enforced | same sentence as RFC9086-5-3 (duplicate rows); a merge is the judge's D-2 call |
| RFC7752-3.3.2.3-1 | tests + row | NEW isis bgpls_export_rfc7752_te_metric_test.go: TestRFC7752TEDefaultMetricWidenedWithZeroHighOrder (+: IS-IS sub-TLV 18 0x0a0b0c -> TLV 1092 000a0b0c) / TestRFC7752TEDefaultMetricAllOnesNeverReachesHighOrder (-: ffffff -> 00ffffff). Drives the real widening in isis linkAttribute (the path the note named). {single-polarity: positive} marker removed | rfc7752 +/- revert isis linkAttribute | enforced | old nlri/ls TestRFC7752TEDefaultMetricZeroPadded keeps its + tag |
| RFC7752-3.2-5 | tests | NEW ls_export export_rfc7752_family_test.go: TestRFC7752NativeNLRIAnnouncedAsSAFI71 (+: originated Node/Link/Prefix each announced "nlri bgp-ls/bgp-ls", registry (16388,71)) / TestRFC7752NativeNLRINeverAnnouncedAsVPN (-: none under bgp-ls-vpn or any other family) | rfc7752 +/- revert export_state.go send | enforced | stops at the announce command; family-name -> wire AFI/SAFI is the reactor's registry encode (not driven here) |
| RFC9085-2.1.1-1 | defect (D-8) + tests + row | DEFECT: isis srBlock copied the 3-octet SID/Label verbatim into TLV 1034/1036 sub-TLV 1161, so a source label with its 4 leftmost bits set reached the collector. Failing test first (observed red: 800000006404890003f03e80), fix `value[target+7] &= 0x0f` with the §2.1.1 quote above it. NEW isis bgpls_export_rfc9085_test.go TestRFC9085OriginatedSRGBLabelTwentyBits (+) / TestRFC9085OriginatedSRGBLabelHighBitsCleared (-). Receipt: existing nlri/ls TestRFC9085SIDLabelMasksLeftmostFourBits (+). {single-polarity} marker removed. Docs: nlri-bgpls.md paragraph on IS-IS SR Capabilities translation | rfc9085 +/- revert isis srBlock | enforced | ALSO: gap rows RFC9085-2.1.2-1, 2.1.2-2, 2.1.4-1, 2.1.4-2 say "no production path originates an SR Capabilities / SRLB TLV": FALSE, isis capabilities()+srBlock originate 1034/1036 (flags masked 0xc0 / 0, reserved 0). Those gaps are stale claims to correct and prove (not done: budget) |
| RFC9514-7.1-4 | tests | receipt: existing nlri/ls units. originate: NEW isis TestRFC9514ISISEndpointBehaviorOriginatedFlagsZero (+) / ...NativeFlagsNeverOriginated (-: native End flags 0xff, TLV 1250 Flags 0) | rfc9514 +/- revert isis endSID | enforced | |

## Files changed
- internal/component/bgp/plugins/ls_export/export_rfc9552_ordering_test.go (new)
- internal/component/bgp/plugins/ls_export/export_rfc9514_sentence_test.go (new)
- internal/component/bgp/plugins/ls_export/export_rfc9086_sentence_test.go (new)
- internal/component/bgp/plugins/ls_export/export_rfc7752_family_test.go (new)
- internal/plugins/isis/bgpls_export_rfc9514_sentence_test.go (new)
- internal/plugins/isis/bgpls_export_rfc7752_te_metric_test.go (new)
- internal/plugins/isis/bgpls_export_rfc9085_test.go (new)
- internal/plugins/isis/bgpls_export.go (srBlock fix, D-8)
- internal/component/bgp/plugins/nlri/ls/rfc9552_ordering_test.go (tags RFC9552-5.1-2/RFC7752-3.1-3 removed from two 518 units, comments; approved D-15; edited via a python script, not Edit: gofmt clean)
- internal/component/bgp/plugins/nlri/ls/types_descriptor.go (false "518 is the only sub-TLV a descriptor can repeat" comment rewritten; python edit, gofmt clean)
- docs/architecture/wire/nlri-bgpls.md (SR Capabilities translation paragraph)
- rfc/short/rfc9086.md (RFC9086-5-5 single-polarity marker removed), rfc/short/rfc7752.md (RFC7752-3.3.2.3-1 marker removed), rfc/short/rfc9085.md (RFC9085-2.1.1-1 marker removed)
- rfc/discrimination/rfc9552.json, rfc7752.json (new file), rfc9514.json, rfc9086.json, rfc9085.json (records, all observed red under revert)

Gates: go test -race green on ls_export, nlri/ls, isis. gofmt clean. golangci-lint NOT run (refused: parallel golangci-lint running) -> owed on the three packages. ./le rfc check: my stems show only STALE audit verdict lines (13 ids above, expected before re-judge), no other finding. Owed: independent judge + audit-stamp rejudge; reseal not needed for new units but check after judges.

## Ids left (not started or unresolved)
| id | state | recommendation |
|----|-------|----------------|
| RFC9514-7.2-5, 7.2-6 | unresolved (row, R3 state already) | originate clause is already split into {gap} rows RFC9514-7.2-2/7.2-3 (no SRv6 PeerNode SID TLV 1251 originated); quote cannot be narrowed below 24 chars. Judge under R3 on the receipt clause, whose +/- units exist (nlri/ls TestRFC9514PeerNodeSIDReservedIgnored) |
| RFC9085-2.1.2-3, 2.1.2-4 | not started | originate IS possible (see 2.1.1-1 note): add isis tests for 1034 reserved octet 0 and flags masked, then correct the stale gap rows 2.1.2-1/2.1.2-2 (and 2.1.4-x for SRLB); narrow the {single-polarity} reasons that cite attr_link/attr_prefix (other sections); split-needed rows from the child spec (§2.1.4, 2.2.1, 2.2.2, 2.3.1, 2.3.5 receipt) still owed |
| RFC7752-3.2-6 | not started | Ze originates no VPN BGP-LS (exporter hard-codes bgp-ls/bgp-ls). Transit framing under SAFI 72 is proven by message/rfc9552_bgpls_test.go (RFC9552-5.2-2 +): tag that unit with RFC7752-3.2-6 when the message author releases the file (R6), or R3-split origination as gap |
| RFC7752-3.1-1, RFC9552-5.1-3 | not started | needs a reactor-level test: unknown attribute TLV reaches the forwarded UPDATE byte-identical; negative = a different producer rule. reactor held by the message/reactor author: queue (R6) |
| RFC9552-5.1-4 | not started | tag reactor TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated for the NLRI half (judge's own pointer); queue behind reactor author |
| RFC9552-8.2.2-1, 8.2.2-2, 8.2.2-3 | not started | drive message/rfc7606_bgpls_nlri.go bgplsNLRIWellFormed and rfc7606_bgpls.go validateBGPLSAttr via enforceRFC7606 with semantically odd TLVs (+) and framing errors (-); and remove the 'fixed-length TLV refused' assertion from the negative, since 8.2.2 lists it as semantic. message package held by another author: queue |
| RFC7752-6.2.2-2 | not started | MP_REACH / MP_UNREACH TLV-sum and descriptor-sum checks live on the session path (message package): queue; successor is 'unextracted §8.2.2' |

# Continuation 2

Finding: BOTH IGP exporters originate TLV 1034/1036: ospf routerInformation (flags 0, reserved 0, from RI SRGB/SRLB) and isis capabilities()+srBlock (1034 flags &0xc0, 1036 flags 0, reserved 0). The four {gap} rows were false public claims.

| id | resolution | what now proves each clause | records | expected verdict | notes |
|----|------------|-----------------------------|---------|------------------|-------|
| RFC9085-2.1.2-1 | row (false gap removed) + tests | set-0 (OSPF flags): NEW ospf bgpls_export_rfc9085_test.go TestRFC9085OSPFOriginatedCapabilitiesFlagsReservedZero (+: exact 1034 00 00 001f40 0489 0003 003e80) / TestRFC9085OSPFSourceReservedNeverOriginated (-: native RI reserved ff -> Flags 0). ignored-on-receipt: NEW nlri/ls rfc9085_capabilities_receipt_test.go TestRFC9085CapabilitiesReceiptIgnoresFlagsAndReserved (+: Flags ff Reserved ff decode, range 1000/16000) | rfc9085 +/- revert ospf routerInformation; + revert nlri/ls decodeSRCapabilities; all observed red | enforced | IS-IS flags are RFC 8667's (not this OSPF clause), so no isis tag |
| RFC9085-2.1.2-2 | row + tests | set-0: the two ospf units (+/-) and NEW isis TestRFC9085ISISOriginatedCapabilitiesReservedZero (+) / TestRFC9085ISISAllOnesSourceNeverReachesReserved (-: flags ff, range ffffff -> Reserved 0); receipt: nlri/ls unit (+) | ospf revert routerInformation, isis revert srBlock, nlri/ls revert decodeSRCapabilities; observed red | enforced | |
| RFC9085-2.1.4-1 | row + tests | as 2.1.2-1 on TLV 1036 (same ospf units, same receipt unit) | ospf revert routerInformation; nlri/ls revert decodeSRLocalBlock | enforced | |
| RFC9085-2.1.4-2 | row + tests | as 2.1.2-2 on TLV 1036 (ospf + isis units, receipt unit) | ospf routerInformation, isis srBlock, nlri/ls decodeSRLocalBlock | enforced | |
| RFC9085-2.1.2-3 | tests + row (single-polarity marker removed) | set-0: ospf + isis units (+/-); receipt: existing nlri/ls TestRFC9085SRCapabilitiesIgnoresReservedAndUndefinedFlags (+) | ospf routerInformation, isis srBlock | enforced | same quote as 2.1.2-2 (duplicate row): D-2 merge is the judge's call. Old marker cited attr_link/attr_prefix (other sections): gone |
| RFC9085-2.1.2-4 | tests + row (marker removed) | set-0: ospf units (+/-); receipt: existing nlri/ls unit (+) | ospf routerInformation | enforced | same quote as 2.1.2-1 (duplicate): D-2 merge judge's call |
| RFC9514-7.2-2 | row text corrected, stays {gap} | none (no producer emits TLV 1251) | none | gap (unchanged) | re-judged: still a gap. "Flags written verbatim" was false for the origination path: ls_export clearOriginatedReserved masks 1251 Flags to 0xe0 and clears Reserved; but no producer hands it a 1251 because Ze assigns no SRv6 BGP EPE segments (7.2-1). Text now says so; the verbatim nlri/ls struct encoder (attr_srv6.go:170) is named as off-path. 7.2-3 text corrected the same way (stale line refs, exporter clear) |
| RFC9085-2.2.1-1 | row (FALSE gap removed) + tests | set-0: NEW isis bgpls_export_rfc9085_sid_test.go TestRFC9085ISISOriginatedSIDReservedZero (+: exact 1099 30 01 0000 003e81) / TestRFC9085ISISAllOnesSourceNeverReachesSIDReserved (-: all-ones native -> Reserved 0000); receipt: NEW nlri/ls TestRFC9085SIDReceiptIgnoresReserved (+: Reserved ffff decodes flags/weight/SID) | isis revert sidValue (+/-), nlri/ls revert decodeAdjacencySID; observed red | enforced | OSPF origination (bgplsAdjAttribute) implemented, untested |
| RFC9085-2.2.2-1 | row (FALSE gap: 1100 IS originated) + tests | set-0: same isis units, TLV 1100 exact bytes (+/-) | isis revert sidValue | weak-to-enforced | receipt clause: nlri/ls does not decode 1100 (carried unparsed); judge decides if that counts as "ignored" |
| RFC9085-2.3.1-1 | row (FALSE gap) + tests | set-0: isis units TLV 1158 (+/-); receipt: nlri/ls TestRFC9085SIDReceiptIgnoresReserved (+) | isis revert sidValue; nlri/ls revert decodePrefixSID | enforced | OSPF bgplsPrefixAttribute untested |
| RFC9085-2.3.5-1 | row (FALSE gap: 1159 IS originated from IS-IS TLV 149 and OSPF ranges) + tests | set-0: isis units TLV 1159 (+: exact; -: native TLV 149 reserved ff -> 0) | isis revert binding | weak-to-enforced | receipt: nlri/ls does not decode 1159 (unparsed) |
| RFC9085-2.1-1 | row (FALSE gap removed), UNTESTED | nothing yet | none | untested | both exporters attach SR TLVs to the originating node's Node NLRI; needs a +/- placement test (two nodes, only the advertiser carries 1034). Not done: budget |

rfc9085 Meta: Enrolment reason, Support coverage, Support remaining rewritten (no gaps left; Partial kept: 1100/1159 undecoded, OSPF SID origination untested, 2.1-1 untested). NOTE for judge: the rfc9085 edits to rows 619-622/624 were done with a python regex (not Edit), all others with Edit/Write.

### Files changed (continuation 2)
- internal/plugins/ospf/bgpls_export_rfc9085_test.go (new)
- internal/plugins/isis/bgpls_export_rfc9085_test.go (2 new functions, header)
- internal/plugins/isis/bgpls_export_rfc9085_sid_test.go (new)
- internal/component/bgp/plugins/nlri/ls/rfc9085_capabilities_receipt_test.go (new)
- rfc/short/rfc9085.md (9 rows de-annotated, Meta), rfc/short/rfc9514.md (7.2-2, 7.2-3 gap text)
- rfc/discrimination/rfc9085.json (records above; scripts c4-rec1.sh, c4-rec2.sh, logs c4-rec1.log, c4-rec2.log)

Gates (continuation 2): go test -race green on isis, ospf, nlri/ls (c4-race.log); golangci-lint 0 issues on the three packages (job-lint-pkg-0385d4f9.log); ./le rfc check: my stems show only STALE audit verdict lines for RFC9085-2.1.2-3/2.1.2-4 (expected before re-judge), every other finding belongs to other stems (linklocal draft, rfc1035, rfc2328, rfc9190). Owed: judge re-judge of the 11 rfc9085 ids + RFC9514-7.2-2/7.2-3, audit-stamp, index-update (rfc-status.md is generated from the Meta rows I changed).
Not done (item 3, as instructed): RFC9552-5.1-3, 5.1-4, 8.2.2-1/2/3, RFC7752-3.1-1, 6.2.2-2, 3.2-6 stay queued behind the message/reactor author. Also still open: RFC9514-7.2-5/7.2-6 (see first pass), RFC9085-2.1-1 placement test, OSPF-originated 1099/1100/1158/1159 tests.

# Continuation 3

| id | resolution | what now proves each clause | records | expected verdict | notes |
|----|------------|-----------------------------|---------|------------------|-------|
| RFC9085-2.2.1-1 | tests (OSPF half added) | set-0 OSPF: NEW ospf TestRFC9085OSPFOriginatedSIDReservedZero (+: exact 1099 60 05 0000 005dc1) / TestRFC9085OSPFSourceReservedNeverReachesSIDReserved (-: native Adj-SID Reserved ff -> 0000); isis units from cont 2 kept; receipt: nlri/ls TestRFC9085SIDReceiptIgnoresReserved (+) | ospf revert bgplsAdjAttribute +/-, observed red (c5-rec1b.log) | enforced | |
| RFC9085-2.3.1-1 | tests (OSPF half added) | same ospf units, TLV 1158 exact 40 01 0000 0000004d (+) / native Prefix-SID Reserved ff (-) | ospf revert bgplsPrefixAttribute +/- | enforced | |
| RFC9085-2.2.2-1 | tests (OSPF + receipt) | set-0: same ospf units TLV 1100 exact (+/-: LAN Adj-SID Reserved ff); receipt: NEW nlri/ls TestRFC9085UndecodedSIDReceiptKeepsReserved (+: TLV 1100 Reserved ffff kept whole as generic-lsid-1100, Node Name after it still decodes) | ospf revert bgplsAdjAttribute +/-; nlri/ls revert AttrTLVsToJSON (+) | enforced | receipt: this decoder carries 1100 undecoded; "ignored" = kept whole, not refused |
| RFC9085-2.3.5-1 | tests (OSPF + receipt) | set-0: same ospf units TLV 1159 exact 80 00 0010 0486 0008 40000000 00000064 (+/-: Range Reserved ff ff ff and inner Prefix-SID Reserved ff); receipt: same nlri/ls unit, generic-lsid-1159 kept whole | ospf revert prefixRange +/-; nlri/ls revert AttrTLVsToJSON (+) | enforced | |
| RFC9085-2.1-1 (SHOULD) | tests | NEW ospf TestRFC9085OSPFCapabilitiesOnOriginatorNode (+: two routers 2.2.2.2 and 3.3.3.3; only 2.2.2.2 has an RI LSA; its Node NLRI carries exact 1034 and 1036) / TestRFC9085OSPFCapabilitiesOnNoOtherNLRI (-: 3.3.3.3's Node NLRI and every Link/Prefix NLRI (which carry SIDs 1099/1100/1158/1159) carry neither 1034 nor 1036; asserts the other node, a link and a prefix exist) | ospf revert routerInformation +/- | enforced (SHOULD) | OSPF only; isis not driven. Revert is the tool's panic break, so the negative's red is "no snapshot", not a misplaced TLV: judge's call |
| RFC9085-2.1.2-3, 2.1.2-4 | row: D-2 merge (retired) | 2.1.2-3 folded into 2.1.2-2, 2.1.2-4 into 2.1.2-1. attr_test.go TestRFC9085SRCapabilitiesIgnoresReservedAndUndefinedFlags tags retagged (-4 -> 2.1.2-1 +, -3 -> 2.1.2-2 +), VALIDATES/doc comment corrected; ospf and isis duplicate -3/-4 tag lines removed (the units already carried identical -1/-2 claims). Approvals D-15 recorded for the 5 units | new records 2.1.2-1 +, 2.1.2-2 + on the attr_test unit (revert decodeSRCapabilities, observed red); the 6 records for the retired ids removed from rfc/discrimination/rfc9085.json (Edit) | 2.1.2-1/2.1.2-2 enforced | Retired paragraphs in rfc/corrections/rfc9085.md; §2.1.2 unsourced-ids removed from rfc/extraction/rfc9085.json; rfc/audit/rfc9085.json entries for the two retired ids REMOVED by me (./le rfc check refuses a verdict on an absent row; D-2 says the row leaves audit): judge please confirm. Meta: MUST count 9, Support remaining rewritten |

### Files changed (continuation 3)
- internal/plugins/ospf/bgpls_export_rfc9085_test.go (header; 4 new tests + scenario helpers; duplicate 2.1.2-3/-4 tag lines removed)
- internal/plugins/isis/bgpls_export_rfc9085_test.go (duplicate 2.1.2-3 tag lines removed)
- internal/component/bgp/plugins/nlri/ls/rfc9085_capabilities_receipt_test.go (header; new TestRFC9085UndecodedSIDReceiptKeepsReserved)
- internal/component/bgp/plugins/nlri/ls/attr_test.go (TestRFC9085SRCapabilitiesIgnoresReservedAndUndefinedFlags: tags retagged to 2.1.2-1/2.1.2-2, doc comment)
- rfc/short/rfc9085.md (rows 2.1.2-3/-4 removed; Enrolment reason, Support remaining)
- rfc/corrections/rfc9085.md (two Retired paragraphs)
- rfc/extraction/rfc9085.json (section 2.1.2 unsourced-ids removed)
- rfc/audit/rfc9085.json (entries for retired 2.1.2-3/-4 removed)
- rfc/discrimination/rfc9085.json (14 new records via discriminate-record: c5-rec1b.log, c5-rec2.log, c5-rec3.log; 6 retired-id records removed by Edit)

Gates (continuation 3): go test -race green on isis, nlri/ls, ls_export (job-c5-race-ac074ae3.log) and ospf targeted TestRFC9085|TestBGPLS (job-c5-ospf2); the lint job reported 0 issues on ospf, isis, nlri/ls, ls_export (job-c5-lint-f9448f17.log); gofmt clean. rfc check (c5-check2.log): rfc9085 shows only STALE verdict lines for 2.1.2-1, 2.1.2-2, 2.1.4-1, 2.1.4-2, 2.2.1-1, 2.2.2-1, 2.3.1-1, 2.3.5-1 and SHIFTED for 2.1.1-1 (expected before re-judge); nothing else for the stem. Owed to the judge: re-judge those 8 + RFC9085-2.1-1 (new tests), audit-stamp, reseal/index-update (rfc-status.md derives from the Meta I changed). Still open: IS-IS side of 2.1-1 untested (Support remaining says so); queued ids unchanged (RFC9552-5.1-3/5.1-4/8.2.2-x, RFC7752-3.1-1/6.2.2-2/3.2-6, RFC9514-7.2-5/7.2-6).
