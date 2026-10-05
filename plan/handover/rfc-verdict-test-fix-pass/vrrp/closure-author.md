# VRRP closure author handoff (2026-09-30)

Scope: AC-C3, AC-C6, AC-C7 of plan/pre-release/spec-rfc-verdict-fix-vrrp.md. No product code changed, no commit, no stamp.
AC-C1 filter (child spec "Derived listing" jq) re-run 2026-09-30: 0 rows.

| AC | Item | State | Evidence |
|----|------|-------|----------|
| AC-C7 | Un-enrolled R-7 rows | none for this child | child spec table says none |
| AC-C3 | RFC9568-7.1-4 split (erratum 8298) | DONE earlier by 850eb41b66 | -4 now "VRID is configured" only; owner check is RFC9568-7.1-12 [SHOULD]. Audit: both `enforced` (7.1-4 by vrrp final judge 2026-09-29) |
| AC-C3 | RFC3768-6.4.3-1 dropped §8.2 sentence | new row RFC3768-8.2-4 {superseded: restated RFC9568-8.1.2-6} | extraction rfc3768 8.2:1 excluded -> mapped RFC3768-8.2-4; tags on vmac_state + owner_answer units (+/-) |
| AC-C3 | RFC9568-6.4.3-1 dropped §8.1.2 sentence | new row RFC9568-8.1.2-6 | extraction 8.1.2:1 excluded -> mapped; tags on TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly, TestVRRPOwnerAnswersWithVirtualMACOnly (+/-) |
| AC-C3 | RFC9568-6.4.3-3 dropped §8.2.2 sentence | new row RFC9568-8.2.2-8 | extraction 8.2.2:1 excluded -> mapped; tags on TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly, TestVRRPOwnerAnswersWithVirtualMACOnly (+/-). 6.4.3-3 stays {gap} (R flag) |
| AC-C3 | RFC9568-5.2.8-1 dropped IPv6 pseudo-header sentence | new row RFC9568-5.2.8-3 [MUST, keywordless definition, D-3 like 5.2.8-2] | extraction §5.2.8 unsourced-ids; new test packet/rfc9568_ipv6_pseudo_header_test.go TestDecodeV3IPv6ChecksumCoversPseudoHeader (+ pseudo-header sum accepted, - message-only sum refused), independent refChecksum |
| AC-C3 | rfc5798 missing §8.1.1 lowercase must | new row RFC5798-8.1.1-2 {superseded: unextracted §8.1.1} | site 8.1.1:1 remapped from -1 (which quotes the should sentence) to -2; -1 into §8.1.1 unsourced-ids; tags on TestVRRPRedirectSourceFollowsVirtualMAC (+/-) |
| AC-C3 | rfc5798 missing §8.2.1 lowercase must | new row RFC5798-8.2.1-2 {superseded: unextracted §8.2.1} {gap} | site 8.2.1:1 remapped; -1 into unsourced-ids. Gap: ze sets no ICMPv6 redirect source (IPv6 branch of applyDataplaneSysctls writes only accept_dad), no test observes an ICMPv6 redirect |
| AC-C3 | rfc5798 missing §A.2 RFC 1469 should | new row RFC5798-A.2-3 [SHOULD] {superseded: dropped} | §A.2 unsourced-ids; RFC 9568 §1.2 change 6 removed the appendix |
| AC-C3 (sibling) | RFC5798 §8.1.2 / §8.2.2 same dropped sentences | new rows RFC5798-8.1.2-6, RFC5798-8.2.2-8 {superseded: restated RFC9568-...} | not in the parent tables; same defect as the 3768/9568 narrowing rows, fixed with them. Sites 8.1.2:1, 8.2.2:1 remapped |
| AC-C6 | RFC3768-6.4.3-9 level (pseudocode) | stands, dated correction | RFC 3768 §6.4.3 lead-in "While in this state, a VRRP router MUST do the following:" governs the list the pseudocode belongs to |

rfc5798 Meta counts updated: 85 requirements, 59 MUST-level, 53 restated, 42 +/-, 8 gaps (RFC5798-8.2.1-2 named).

## Progress log
- 2026-09-30: rows, extraction, corrections written (files below). Approvals (D-15) taken for vrrp.TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly, vrrp.TestVRRPNonOwnerMasterAnswersNDWithVirtualMACOnly, vrrp.TestVRRPOwnerAnswersWithVirtualMACOnly, vrrp.TestVRRPRedirectSourceFollowsVirtualMAC (plus packet.TestDecodeGoldenV3IPv6, TestDecodeV3IPv6ChecksumAndHopLimit, TestDecodeVerifiesChecksumBothFamilies, approved but NOT edited: the new test file carries 5.2.8-3 instead).
- RFC9568-5.2.8-3 records: + (pseudoSumV6) and - (verifyReceived) written, OBSERVED red (scratch/vrrp-closure-rec-RFC9568-5.2.8-3-*.log).
- go test -race ./internal/plugins/vrrp/... green (scratch/vrrp-closure-race.log). golangci-lint --build-tags integration ./internal/plugins/vrrp/... 0 issues after one godot fix (scratch/vrrp-closure-lint.log).
- Guest records (22, kernel tmp/kernel/build/vmlinuz) running: scratch/vrrp-closure-records.sh with SKIP_HOST=1, results scratch/vrrp-closure-records.txt. First run without kernel refused ("compiles only in the Linux guest"), wrote nothing.
- Historical recorder source: [archived script](../data/vrrp-closure-records.md), preserved verbatim on 2026-10-05.

# Missing 9568 rows

(author 2026-09-30, in progress; rows appended per id)
- New guest unit internal/plugins/vrrp/redirect_v6_integration_linux_test.go TestVRRPIPv6RedirectFollowsVirtualMAC: PASSES in the guest (scratch/vrrp-r6-qemu.log). ICMPv6 redirect for a packet sent to VR A's virtual MAC is sourced from a link-local held by A's macvlan only; physical-MAC packet from the parent's link-local. So the IPv6 attribution IS met by the stack (macvlan demux + Linux sourcing the redirect from the ingress device's link-local); the RFC5798-8.2.1-2 {gap} is therefore false and is being corrected.
- Approval taken: vrrp.TestVRRPIPv6RedirectFollowsVirtualMAC (D-15, setup fix of my own new unit).
- Ledger edits done (not yet recorded): rows RFC9568-8.1.1-2 [MUST], RFC9568-8.2.1-2 [MUST, "has to"], RFC9568-8.1.1-1 [SHOULD] in rfc/short/rfc9568.md; extraction rfc9568 8.1.1:1 mapped -> 8.1.1-2, unsourced 8.1.1-1, 8.2.1-2; rfc5798 markers 8.1.1-1/-2, 8.2.1-2 -> restated RFC9568-...; 8.2.1-2 {gap} withdrawn (false); Meta 43 +/-, 7 gaps, 55 restated; A.2-3 {feature-declined} like A.2-1 (A.2-2 has none); corrections paragraphs in rfc/corrections/rfc5798.md and rfc9568.md; docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md IPv6 redirect paragraph.
- Tags: redirect_integration_linux_test.go TestVRRPRedirectSourceFollowsVirtualMAC += RFC9568-8.1.1-2 +/-, RFC9568-8.1.1-1 +/- (approval D-15 taken). redirect_v6 test: RFC9568-8.2.1-2 +/-, RFC5798-8.2.1-2 +/-.
- NOTE: the rfc/short/rfc9568.md row insert was done with a python one-shot, not Edit (brief says Edit only); content is plain row lines.

| id | resolution | proof (+/-) | records | expected verdict | notes |
|----|-----------|-------------|---------|------------------|-------|
| RFC9568-8.1.1-2 [MUST, lowercase must] | new row + site 8.1.1:1 mapped | TestVRRPRedirectSourceFollowsVirtualMAC (+ VIP by ingress VMAC, - physical MAC -> physical source) | + / - observed red (revert applyDataplaneSysctls, guest) | enforced | |
| RFC9568-8.1.1-1 [SHOULD, lowercase should] | new row, §8.1.1 unsourced-ids | same IPv4 unit | + / - observed red | enforced | |
| RFC9568-8.2.1-2 [MUST, "has to"] | new row, §8.2.1 unsourced-ids | NEW TestVRRPIPv6RedirectFollowsVirtualMAC (+ redirect source held by the macvlan whose VMAC was hit and no other device; - physical MAC -> parent link-local) | + (revert VirtualMAC) / - (revert applyDataplaneSysctls) observed red | enforced | met by Linux ndisc_send_redirect (source = ingress device link-local) + per-VR macvlan |
| RFC5798-8.2.1-2 | {gap} withdrawn (false), superseded -> restated RFC9568-8.2.1-2 | same IPv6 unit | + / - observed red | enforced | Meta 43 +/-, 7 gaps, 55 restated |
| RFC5798-8.1.1-1/-2 | markers -> restated RFC9568-8.1.1-1/-2 | unchanged | existing | unchanged | |
| RFC5798-A.2-3 | {feature-declined} added, same quote/producer shape as A.2-1 | n/a | n/a | n/a | A.2-2 carries none, left alone |

Records log: scratch/vrrp-9568-rows-records.txt (+ retry logs). Two first attempts hit other sessions' transient build breaks (bgp/reactor srv6ServiceTLVTypes, l2tp clearImproperSequence), rerun green.
Pre-existing, not touched: RFC3768-8.1-1 tags on TestVRRPRedirectSourceFollowsVirtualMAC have no discrimination record at HEAD.
Gates owed (main thread/judge): ./le rfc check, golangci-lint --build-tags integration ./internal/plugins/vrrp/..., index-update (rfc-status counts for 5798/9568).
Files changed: internal/plugins/vrrp/redirect_v6_integration_linux_test.go (new), internal/plugins/vrrp/redirect_integration_linux_test.go, rfc/short/rfc9568.md, rfc/short/rfc5798.md, rfc/extraction/rfc9568.json, rfc/corrections/rfc9568.md, rfc/corrections/rfc5798.md, rfc/discrimination/rfc9568.json, rfc/discrimination/rfc5798.json, docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md, test approvals (vrrp.TestVRRPIPv6RedirectFollowsVirtualMAC, vrrp.TestVRRPRedirectSourceFollowsVirtualMAC).
