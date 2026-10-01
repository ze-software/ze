# rfc9552 author (BGP child, stem pass after BGP c19)

Status 2026-10-01 (BGP c19): NOT STARTED. BGP c19 derived the listing and stopped before call 80 (brief addendum: no new id after call 80). No file edited for this stem.

Derived listing (child spec "Derived listing" jq, scratch children/bgp/c19-listing.tsv, 136 lines over the child before "Blocked by" is subtracted; top stems by size: rfc9552 16, rfc4724 9, rfc9494 8, draft-ietf-idr-linklocal-capability 8, rfc8277 7, rfc9252 6, rfc8669 6, rfc2385 5).

rfc9552, 16 weak, none named under the child's "Blocked by" (the spec names RFC9552-5.1-2, 5.3.1.3-1, 5.3.2.2-3 elsewhere, not in this list):

| id | verdict | resolution | notes |
|----|---------|-----------|-------|
| RFC9552-5.1-3 | weak | not started | |
| RFC9552-5.1-4 | weak | not started | |
| RFC9552-5.1-5 | weak | not started | |
| RFC9552-5.2-6 | weak | not started | |
| RFC9552-5.2-7 | weak | not started | |
| RFC9552-5.2.1-1 | weak | not started | |
| RFC9552-5.2.1.4-1 | weak | not started | |
| RFC9552-5.2.2-1 | weak | not started | |
| RFC9552-5.2.2-4 | weak | not started | |
| RFC9552-5.2.2-5 | weak | not started | |
| RFC9552-5.2.3.1-1 | weak | not started | |
| RFC9552-5.3.2.1-1 | weak | not started | |
| RFC9552-8.2.2-1 | weak | not started | |
| RFC9552-8.2.2-2 | weak | not started | |
| RFC9552-8.2.2-3 | weak | not started | |
| RFC9552-8.2.2-9 | weak | not started | |

Read first: plan/handover/rfc-verdict-test-fix-pass/bgp/nlri-ls-author.md (earlier nlri/ls passes on this stem, "Ids left" and Continuations 2-3), then each id's audit note in rfc/audit/rfc9552.json. Tagged units live in internal/component/bgp/plugins/nlri/ls and internal/plugins/isis/bgpls_export_rfc9552_test.go.

# BGP c21 (2026-10-01, BGP c21 author) -- rfc9552 rows

No commit, no stamp. Logs scratch/c21-*. Records under flock ledger-rfc9552.lock. All new units in internal/component/bgp/reactor/rfc9552_nlri_test.go (receive path, enforceRFC7606), helpers lsNodeNLRIWithTrailing and lsReceive.

| id | resolution | + / - units | records | expected verdict | notes |
|----|-----------|-------------|---------|------------------|-------|
| RFC9552-5.1-4 | tests (NLRI half added on the session path) | + TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated (EXISTING, tag added under D-15 approval, body unchanged: unknown top-level TLV 1234 + unexpected sub-TLV 999 -> no action, NLRI byte-identical); - NEW TestRFC9552LinkStateNLRIUnknownTLVOverrunIsDiscarded (unknown TLV 1234 declaring 200 octets with 3 present -> NLRI discarded, survivor kept). Attribute half stays on nlri/ls rfc9552_test.go units | + and - on message/rfc7606_bgpls_nlri.go::bgplsNLRIWellFormed (revert, observed red, c21-rec-9552a.log) | weak -> enforced (both halves of "NLRI or the BGP-LS Attribute") | approval: `./le rfc approve unit reactor.TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated` (D-15) |
| RFC9552-5.1-5 | tests (same-type clauses added) | + NEW TestRFC9552LinkStateSameTypeTLVsOutOfOrderAreDiscarded (TLV 1234 twice, Lengths 3 then 2; and equal Lengths, Values bb bb then aa aa: each discarded, survivor kept); - NEW TestRFC9552LinkStateSameTypeTLVsInOrderArePropagated (Lengths 2 then 3; Values aa aa then bb bb: each survives byte-identical). Existing descending-type +/- units unchanged | + and - on message/rfc7606_bgpls_nlri.go::bgplsTLVOrdered (revert, observed red) | weak -> enforced: both clauses of "the above ordering rules" driven | c21-9552a.log green (-race, TestRFC9552*) |
| RFC9552-8.2.2-9 | tests (the two undriven bullets added) | + NEW TestRFC9552LinkStateUnreachLengthOverrunResetsSession (MP_UNREACH_NLRI LS NLRI with Total NLRI Length 255, 21 present -> RFC7606ActionSessionReset + error); + NEW TestRFC9552LinkStateRecognizedTLVSubTLVOverrunIsDiscarded (TLV 256 framed at 8, sub-TLV 512 declaring 8 with 4 present -> NLRI discarded, survivor kept). Existing +/- units unchanged (well-formed negative TestRFC9552LinkStateNLRIWithUnknownTLVsIsPropagated) | + on validateBGPLSNLRISyntax and + on bgplsNodeDescriptorWellFormed (message/rfc7606_bgpls_nlri.go, revert, observed red, c21-rec-9552b.log) | weak -> enforced: every bullet the audit note named is now driven on the receive path | c21-9552b.log green |

## BGP c21: files changed (rfc9552)
- internal/component/bgp/reactor/rfc9552_nlri_test.go (one tag added to an existing unit under D-15 approval; 6 NEW units; helpers lsNodeNLRIWithTrailing, lsReceive)
- rfc/discrimination/rfc9552.json (8 new records)

## BGP c21: rfc9552 rows NOT started (next author)
RFC9552-5.1-3 (reactor forward test: unknown attribute TLV reaches the forwarded UPDATE byte-identical; the llnhForward harness in reactor/rfc_draft_linklocal_reflect_test.go fits), 5.2-6, 5.2-7, 5.2.1-1, 5.2.1.4-1 (audit says the SENDER violates it: NodeDescriptor.WriteTo emits one sub-TLV 518 per SRv6SIDs entry -> D-8 candidate, failing test first), 5.2.2-1, 5.2.2-4, 5.2.2-5, 5.2.3.1-1, 5.3.2.1-1, 8.2.2-1, 8.2.2-2, 8.2.2-3 (8.2.2-1..3: drive enforceRFC7606 with semantically odd TLVs; the reactor units above show the shape).
OWED gates (main thread): scoped golangci-lint internal/component/bgp/reactor; `./le rfc check` (8 rfc9552 records, the D-15 approval).

## BGP c21: judge (2026-10-01, independent BGP c21 judge)

| id | old -> new | why |
|----|-----------|-----|
| RFC9552-5.1-4 | weak -> enforced | NLRI half now tagged on the receive path (unknown type tolerated byte-identical; unknown-type overrun discarded); attribute half unchanged; D-15 approval present |
| RFC9552-5.1-5 | weak -> enforced | type, same-type Length and same-type Value clauses each driven both polarities, isolated inputs, records |
| RFC9552-8.2.2-9 | weak -> enforced | MP_UNREACH length sum and recognized-TLV sub-TLV length bullets now driven; every concrete bullet of the 8.2.2 list covered |
| RFC9552-5.2.1.4-1 | weak -> weak | stale only by the added 5.1-4 tag line; sender half still violated (NodeDescriptor.WriteTo repeats sub-TLV 518) |
| RFC9552-8.2.2-4, 8.2.2-5 | enforced -> enforced | stale only by the added 5.1-4 tag line |
