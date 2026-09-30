# VRRP final author handoff (2026-09-29, after 5ccbd604ee)

Scope: the six work items the main thread named (R17, R18, 7.2-3 tags, 5798-7.2-1 through encodeLocked, 6.4.2-7 Solicited-Node join, 9568-7.1-4 v3 lookup).
No product code changed. Tests, rows, extraction, corrections, records.
Records: scratch/vrrp-final-records.sh, results in scratch/vrrp-final-records.txt, one log per record scratch/vrrp-final-rec-*.log.

| id | resolution | what now proves each clause (+/-) | records | expected verdict | notes |
|----|-----------|-----------------------------------|---------|------------------|-------|
| RFC9568-7.2-1 | row split (R17) + tests | field-fill row only now. + TestEncodeGoldenV3IPv4 (tag narrowed to field fill, checksum explicitly not claimed), + TestEncodeGoldenV3IPv6 (new tag), + transport TestSendAdvertFillsFieldsAndChecksum (new: IPv4 and IPv6 frames sent through encodeLocked, every field against literals). Row keeps {single-polarity: positive}, reason rewritten for fill only | + WriteTo on both goldens; + encodeLocked on the transport unit | enforced | D-15 approval packet.TestEncodeGoldenV3IPv4 |
| RFC9568-7.2-5 (new) | {gap} row | "* Compute the VRRP checksum" (§7.2), gap names the v3/IPv4 RFC 5798 pseudo-header tx form, keepalived interop, IPv6 unaffected (kernel IPV6_CHECKSUM) | none (gap) | gap | extraction 7.2 unsourced-ids; corrections paragraph |
| RFC9568-5.2.8-1 | row split (R18) | unchanged tests (verify both families, +/-). The deviating clause moved out | none new | enforced | corrections paragraph |
| RFC9568-5.2.8-2 (new) | {gap} row | §5.2.8 IPv4 message-only checksum sentence, verbatim; gap names tx (FillChecksum) and rx dual-accept (verifyReceived, checksum.go) | none (gap) | gap | level MUST on a keywordless definition sentence (D-3 format row); extraction 5.2.8 unsourced-ids. `./le rfc check` raised no row-quote, level or id refusal |
| RFC9568 Meta | Support remaining | "Four gaps gated", first sentence discloses tx pseudo-header form and rx dual-accept | - | - | docs/features/rfc-status.md is rendered from Meta: judge's index-update regenerates it |
| RFC5798-7.2-3 / RFC9568-7.2-3 | tests | + TestTxAdvertV4OnWire (guest): VRRPv3 source 192.0.2.251 primary over the lower secondary 192.0.2.5, tags added. IPv6 half and negatives unchanged | + resolveParentPrimaryV4 (guest, kernel tmp/kernel/build/vmlinuz) | enforced | D-15 approval transport.TestTxAdvertV4OnWire |
| RFC5798-7.2-1 | tests | + new transport TestSendAdvertFillsFieldsAndChecksum: the IPv4 frame from encodeLocked carries every field and a checksum equal to an RFC 1071 sum the test computes itself over the RFC 5798 pseudo-header; a transmit path skipping FillChecksum reddens it. Fill from instance state unchanged (TestTransmittedAdvertFilledFromInstanceState) | + encodeLocked | enforced | IPv6 checksum is the kernel's (IPV6_CHECKSUM), not claimed |
| RFC5798-6.4.2-7 / RFC9568-6.4.2-7 | tests | + new TestPromotionInstallsIPv6AddressesOnVirtualMACDevice: IPv6 non-owner installs nothing in Backup; on its own down timer installs both IPv6 VIPs on the vMAC device, never the parent. That address add is what makes Linux join the Solicited-Node group (Ze builds no MLD itself) | + doInstallVIPs | enforced (join clause at Ze's boundary) | membership itself (/proc/net/igmp6) not read in a guest: that asserts the kernel, not Ze. A judge wanting the kernel membership read would need a guest unit |
| RFC9568-7.1-4 | tests | + / - new TestRxV3VRIDCheckedOnTheReceivingInterface: VRRPv3 advert decoded through the production instance.lookup via engine dispatchRx; on eth1 (VRID 20) discarded with vrid, no FSM sees it; on eth0 reaches AdvertReceived | + dispatchRx, - lookup | enforced | |

## Results

- All 10 records exited 0 and each was OBSERVED red (scratch/vrrp-final-records.txt): 8 host, 2 guest. The guest units failed with `--- FAIL: TestTxAdvertV4OnWire` under the resolveParentPrimaryV4 break.
- Targeted mutant (not a record, go overlay scratch/vrrp-mut-overlay.json): encodeLocked with its FillChecksum call removed turns TestSendAdvertFillsFieldsAndChecksum/ipv4 red ("checksum = 0x0000, want 0x0504"). The old TestTransmittedAdvertFilledFromInstanceState stays green, which is the gap the judge named (log scratch/vrrp-mut-run.log).
- `go test -race ./internal/plugins/vrrp/...` green (scratch/vrrp-final-race.log). `go vet -tags integration` clean. gofmt clean.
- `./le rfc check` (scratch/vrrp-final-check1.log, run before the records): the only VRRP findings are STALE/SHIFTED audit verdicts. There is no row-quote, id-allocation, extraction or gap-count refusal. The other findings are producer-changed records in other stems.
- Weak spot left: the Solicited-Node join is proven at Ze's boundary, which is the address add. The kernel membership is not read.

## Files changed

- internal/plugins/vrrp/packet/packet_test.go (RFC9568-7.2-1 tag narrowed on TestEncodeGoldenV3IPv4; tag added to TestEncodeGoldenV3IPv6)
- internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go (two tags on TestTxAdvertV4OnWire)
- internal/plugins/vrrp/transport/advert_fill_checksum_test.go (new)
- internal/plugins/vrrp/promote_install_v6_test.go (new)
- internal/plugins/vrrp/rx_interface_v3_test.go (new)
- rfc/short/rfc9568.md (7.2-1 text and annotation, new 7.2-5 and 5.2.8-2 rows, Support remaining)
- rfc/extraction/rfc9568.json (unsourced-ids 7.2-5, 5.2.8-2)
- rfc/corrections/rfc9568.md (new, two Correction paragraphs)
- rfc/discrimination/rfc5798.json, rfc9568.json (records)
- RFC approvals: transport.TestTxAdvertV4OnWire, packet.TestEncodeGoldenV3IPv4
- scratch: vrrp-final-records.sh

## Gates owed (main thread / judge)

`./le go lint run` over internal/plugins/vrrp (three new test files unlinted), `./le rfc reseal` / audit-stamp / index-update under the ledger lock, `./le rfc check`. Stale verdicts expected in rfc3768/5798/9568 for every id whose tagged unit I touched (TestTxAdvertV4OnWire, TestEncodeGoldenV3IPv4 carry other ids' tags: RFC3768-7.2-2/3/4, RFC5798-7.2-2/4, RFC9568-7.2-2/4, RFC5798/9568-5.2.6-1): comment-only edits, bodies unchanged, need a re-read.
