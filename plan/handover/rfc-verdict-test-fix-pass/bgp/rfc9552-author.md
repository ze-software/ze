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
