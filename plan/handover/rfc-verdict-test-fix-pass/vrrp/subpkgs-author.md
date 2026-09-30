# VRRP sub-packages author handoff (packet, fsm, transport) -- 2026-09-29

Scope: the derived listing restricted to internal/plugins/vrrp/{packet,fsm,transport}: 36 weak/wrong (rfc3768 15, rfc5798 9, rfc9568 12).
All new tests are in NEW files; no existing tagged unit was edited, so no D-15 approval was recorded.
Records: revert route, one `./le rfc discriminate-record` per tag, every one observed red (guest units with `kernel tmp/kernel/build/vmlinuz`, CGO_ENABLED=0).

| id | resolution | what now proves each clause (+/-) | records | expected verdict | notes |
|----|-----------|-----------------------------------|---------|------------------|-------|
| RFC3768-5.2.3-2 | tests | packet/rfc_rx_verdict_test.go TestDecodeV2DiscardsTTLNot255: + v2 TTL 255 accepted; - v2 TTL 0/1/64/254 ErrTTL | +,- Decode | enforced | the send-back asked for a v2 negative |
| RFC3768-5.3.2-1 | tests | TestDecodeV2DiscardsUnknownType: + v2 Type 1; - v2 Type 0/2/15 (checksum recomputed) ErrType | +,- Decode | enforced | |
| RFC3768-5.3.6-1 | tests | TestDecodeV2DiscardsUnknownOrUnconfiguredAuthType: + Auth 0; - Auth 1,2 (not local method) and 3,128,255 (unknown) ErrAuthType | +,- Decode | enforced | |
| RFC3768-7.1-2 | tests | TestDecodeV2DiscardsIncompletePacket: + 24-octet golden; - cut in fixed fields (ErrTruncated), cut in Auth Data under honest Count 16/20/23 octets (ErrLength), Count 3 over 2 addrs (ErrLength) | +,- Decode | enforced | |
| RFC5798-7.1-2 | tests | packet TestDecodeV3DiscardsIncompletePacket: + v4 and v6 goldens; - 7 octets ErrTruncated, v4 Count 2 w/ one addr, Count 3 over two, v6 1.5 addrs ErrLength | +,- Decode | enforced | |
| RFC9568-7.1-3 | tests | same test as RFC5798-7.1-2 | +,- Decode | enforced | |
| RFC5798-5.2.6-1 | tests | TestEncodeV3ReserveZeroOverDirtyBuffer: + WriteTo into an all-0xFF buffer writes rsvd 0, v4 and v6, bytes equal goldens; - held by TestDecodeV3ReserveIgnoredOnReceive (unchanged) | + WriteTo | enforced | |
| RFC5798-5.2.8-1 | tests | TestFillChecksumZeroesFieldAndCoversPseudoHeader: + FillChecksum over a stale 0xABCD field reproduces v4/v6 goldens, v6 golden equals an independent RFC 1071 sum with next header 112; - v6 advert summed with next header 58, or without pseudo-header, ErrChecksum | + FillChecksum, - verifyReceived | enforced | rx dual-accepts the RFC 9568 message-only v4 sum, which RFC 5798 would call wrong; documented interop deviation (docs/features/rfc-status.md RFC 5798 row), not claimed by the tag |
| RFC9568-5.2.8-1 | tests | TestDecodeVerifiesChecksumBothFamilies: + v4 message-only sum accepted and flagged, v6 accepted; - v4 sum verifying under no form, v6 flipped bit, ErrChecksum | +,- verifyReceived | enforced | same dual-accept note: a v4 advert whose RFC 9568 sum is wrong but whose RFC 5798 pseudo-header sum is right is accepted |
| RFC3768-6.4.2-4 | tests | fsm/rfc_verdict_test.go TestV2BackupShutdown: + v2 Backup Shutdown -> [StopTimers, EmitStateChange Initialize]; - the canceled timer's expiry does nothing | + handleBackup, - stopAllTimers | enforced | |
| RFC3768-6.4.2-5 | tests (FSM half) | TestPromotionSetsAdverTimerToOwnInterval: + v2 exact action list incl. StartAdvertTimer{2s}; - unarmed generation does not promote | + promoteToMaster, - matches | enforced | + transport/rfc_announce_verdict_test.go TestAnnounceMasterFramesPerAddress: one broadcast GARP per VIP, sha/tha = literal vMAC (+ buildGARP) |
| RFC5798-6.4.2-7 | tests (FSM half) | same test, v3 Backup that learned 4 s promotes with StartAdvertTimer{1s} | + promoteToMaster | likely enforced | + TestAnnounceMasterFramesPerAddress: GARP per IPv4 VIP with vMAC; NA per IPv6 VIP with R=1 S=0 O=1, Target, TLL = vMAC (+ BuildNA). Solicited-Node join is not asserted (kernel joins on address add); a judge can call that clause weak |
| RFC9568-6.4.2-7 | tests (FSM half) | same | + promoteToMaster | likely enforced | same as RFC5798-6.4.2-7 (+ buildGARP); same Solicited-Node caveat |
| RFC3768-6.4.2-6 | tests | TestV2BackupPriorityZeroSetsSkewTime: + literal 0.609375 s (RFC 3768 skew; v3 form gives 1.21875 s); - non-zero priority arms the full 6.609375 s | + skewTime, - backupAdvert | enforced | expectations are literals, not production formula calls |
| RFC3768-6.4.2-7 | tests | TestV2BackupResetsOrDiscards: + equal, greater, Preempt-off-lower each arm literal 6.609375 s (local interval, no adoption); - Preempt-on lower: no action, old timer still promotes | + masterDownInterval, - backupAdvert | enforced | |
| RFC3768-6.4.2-8 | tests | same test | + masterDownInterval, - backupAdvert | enforced | greater-priority case added |
| RFC3768-6.4.3-6 | tests | TestV2MasterAdverTimerFires: + [SendAdvert{100,2000}, StartAdvertTimer{2s}]; - unarmed generation acts not | + armAdvert, - matches | enforced | |
| RFC3768-6.4.3-7 | tests | TestV2MasterPriorityZero: + v2 Master Priority 0 -> SendAdvert + StartAdvertTimer{2s}; - non-zero lower gets nothing | +,- masterAdvert | enforced | |
| RFC3768-6.4.3-9 | tests | TestV2MasterDemotesOrDiscards: + higher priority and equal from 192.0.2.200 (> .10 only unsigned) demote with literal 6.609375 s; - equal from smaller, lower: nothing, Master | + demoteToBackup, - senderWinsTieBreak | enforced | |
| RFC5798-6.4.2-9 | tests | TestV3BackupAdoptsIntervalOrDiscards: + equal, greater, Preempt-off-lower adopt 4 s and arm literal 14.4375 s; - Preempt-on lower: nothing, interval not adopted | + adoptInterval, - backupAdvert | enforced | |
| RFC9568-5.1.1.3-2 | tests | vrrp/rx_discard_rfc9568_test.go TestInstanceRxDiscardsBadTTLTypeAndCountZero: + TTL 255 reaches FSM; - TTL 0/64/254 records ttl, no FSM event | +,- onPacket | enforced | new file in the top-level package (the discard is instance.onPacket) |
| RFC9568-5.1.2.3-2 | tests | same, IPv6 Hop Limit | +,- onPacket | enforced | |
| RFC9568-5.2.2-1 | tests | same, Type 0/2/15 | +,- onPacket | enforced | |
| RFC9568-5.2.5-1 | tests | same, exact-length Count 0 | +,- onPacket | enforced | |
| RFC3768-7.2-2 / RFC5798-7.2-2 / RFC9568-7.2-2 | tests | transport/rfc_tx_verdict_linux_test.go (guest) TestTxAdvertV4OnWire / TestTxAdvertV6OnWire: + captured frame source MAC equals literal 00-00-5e-00-01/02-0a | + openV4 / openV6 (guest) | enforced | rows keep {single-polarity: positive} |
| RFC3768-7.2-3 | tests | TestTxAdvertV4OnWire: + primary 192.0.2.251 chosen over a lower secondary 192.0.2.5/24 on the wire | + resolveParentPrimaryV4 (guest) | enforced | |
| RFC5798-7.2-3 / RFC9568-7.2-3 | tests | TestTxAdvertV6OnWire: + captured IPv6 source is the macvlan link-local; v4 half unchanged; - held (no-link-local / no-primary tests) | + macvlanLinkLocal (guest) | enforced | |
| RFC3768-7.2-4 / RFC5798-7.2-4 / RFC9568-7.2-4 | tests | TestTxAdvertV4OnWire (v2 and v3) / TestTxAdvertV6OnWire: + protocol/next header 112, dst 224.0.0.18 / ff02::12 compared with literals | + buildIPv4Header / SendAdvert (guest) | enforced | |
| RFC9568-7.1-4 | tests | transport/rfc_rx_iface_verdict_linux_test.go TestRxAdvertScopedToReceivingInterface (guest): + VRID 10 advert on A reaches A's instance and decodes; - on B (VRID 20 only) never reaches A, decodes ErrUnknownVRID at B | +,- onPacket | enforced | also tags RFC3768-7.1-4 (+/-, v2), the previous author's unresolved row |
| RFC5798-7.2-1 | tests | vrrp/advert_fill_rfc5798_test.go TestTransmittedAdvertFilledFromInstanceState: + params handed to the transport carry v3, effective priority 200 then 150 after a tracked interface fails, 1000 ms, both VIPs; the encoding carries those octets and a checksum that verifies; - held by TestBoundaryInterval (unchanged) | + doSendAdvert | enforced | |
| RFC9568-7.2-1 | needs ruling | not touched | none | wrong | see below |

## RFC9568-7.2-1 (wrong): needs a main-thread ruling

FillChecksum (packet/checksum.go) deliberately transmits the RFC 5798 pseudo-header checksum for v3/IPv4, contrary to RFC 9568 Section 5.2.8 ("For the IPv4 address family, the checksum calculation only includes the VRRP message ..."), for keepalived interop (proven by the keepalived QEMU lab). The row's "Compute the VRRP checksum" clause therefore is not met for IPv4 under RFC 9568, and no test can honestly say it is. Recommendation (R3 shape): split the row into the field-fill bullet (tagged by the v4/v6 goldens, IPv6 checksum compliant) and a "Compute the VRRP checksum" row carrying a {gap:} naming the IPv4 interop deviation (owner-approved deviations are never counted conformant, ai/rules/rfc-compliance.md). Move the RFC9568-7.2-1 tag off TestEncodeGoldenV3IPv4 (D-15). Owner call if the deviation is not yet owner-approved.

## Other changes

- internal/plugins/vrrp/transport/transport.go encodeLocked: a stale comment said v3/IPv4 FillChecksum is message-only; it now says the RFC 5798 pseudo-header form is sent (stale-comments rule). Its three records (RFC5798-5.2.9-1 +, RFC5798-7.2-3 -, RFC9568-7.2-3 -) were re-recorded.

## Gates owed (main thread)

`./le go lint run` over internal/plugins/vrrp/..., `./le rfc check`. Host tests: `go test ./internal/plugins/vrrp/...` passed. The guest tests TestTxAdvertV4OnWire, TestTxAdvertV6OnWire and TestRxAdvertScopedToReceivingInterface passed in QEMU (kernel tmp/kernel/build/vmlinuz).

## Files changed

- internal/plugins/vrrp/packet/rfc_rx_verdict_test.go (new)
- internal/plugins/vrrp/fsm/rfc_verdict_test.go (new)
- internal/plugins/vrrp/transport/rfc_tx_verdict_linux_test.go (new, integration && linux)
- internal/plugins/vrrp/transport/rfc_rx_iface_verdict_linux_test.go (new, integration && linux)
- internal/plugins/vrrp/transport/rfc_announce_verdict_test.go (new)
- internal/plugins/vrrp/rx_discard_rfc9568_test.go (new)
- internal/plugins/vrrp/advert_fill_rfc5798_test.go (new)
- internal/plugins/vrrp/transport/transport.go (comment only)
- rfc/discrimination/rfc3768.json, rfc5798.json, rfc9568.json (records written by discriminate-record)
