# ike-eap child, OSPF ESP author (rfc4303 rows tested in internal/plugins/ospf)

Scope: RFC4303-2.1-2 (wrong), RFC4303-1-1, RFC4303-2-1, RFC4303-3.4.3-2 (weak).
Plan: 3.4.3-2 tests (OSPF refusal path, drop single-polarity marker); 2.1-2 new lookup-key unit + netns probe, old selector tags removed;
2-1 netns wire probe (+/-), old negative tag removed; 1-1 R3 split of the IKE-negotiable clause into {gap} RFC4303-1-2.

| id | resolution | what proves each clause | records | expected verdict | notes |
|----|------------|-------------------------|---------|------------------|-------|
| RFC4303-3.4.3-2 | tests | + TestRFC4303ESPReplayWindowCarriesIntegrity (esp replay-window 64, null and aes128 cipher: every installed SA carries sha256 + 32-octet key); - TestRFC4303ESPReplayWindowWithoutIntegrityIsRefused (replay-window 64 with no integrity, 3 cipher forms -> ErrIPsecAuthAlgo). IKE + kept (TestChildSAReplayRequiresIntegrity) | both, revert route (validateIPsecInterface / buildIPsecSA), observed red | enforced | {single-polarity} marker removed from the row (a refusal path now exists on the OSPF producer); Meta Enrolment reason count 2->1 single-polarity. Files: internal/plugins/ospf/rfc4303_esp_test.go (new), rfc/short/rfc4303.md, rfc/discrimination/rfc4303.json |
| RFC4303-1-1 | row (R3 split) + tests | row now "MUST be configurable via management interfaces." (§1). + TestIPsecESPRequiresIntegrity (kept: null cipher + sha256 accepted); - TestRFC4303IntegrityOnlyESPIsConfigurable (R1(b): every integrity algo x {no cipher leaf, explicit null} accepted and installed as ESP proto 50, null cipher, no cipher key). Old negative tag (null+no-integrity refused, proves 3.2-1) removed from TestIPsecESPRequiresIntegrity (approval D-15) | negative: revert validateESPConfidentiality, observed red | enforced | new {gap} row RFC4303-1-2 (IKEv2 negotiation clause; no ENCR_NULL in ike/ipsec EncryptionAlgo, matcher refuses ENCR_NULL); Correction paragraph in new rfc/corrections/rfc4303.md; extraction §1 unsourced-ids RFC4303-1-2; Support remaining text updated. Files: internal/plugins/ospf/config_ipsec_test.go, rfc4303_esp_test.go, rfc/short/rfc4303.md, rfc/corrections/rfc4303.md (new), rfc/extraction/rfc4303.json, rfc/discrimination/rfc4303.json |
| RFC4303-2-1 | tests | + TestIPsecSAProtocolNumber (kept: esp SAParams.Proto 50); + TestRFC4303ESPPacketOnTheWireCarriesProtocol50 (netns XFRM probe: aes128+sha256 esp, OSPF to ff02::5, AF_PACKET ETH_P_ALL capture, IPv6 Next Header == 50); - TestRFC4303IntegrityOnlyESPOnTheWireIsNotAH (R1(b): null-cipher esp, the AH-looking input, still 50 on the wire). Old negative (ah->51, proves RFC4302-2-1) removed from TestIPsecSAProtocolNumber (approval D-15) | both probes: revert ipsecProtoNumber, observed red | enforced for the OSPF producer | audit also names the IKE Child SA producer (engine/child.go installChildSA Proto protoESP) as untested: outside this package's files; a judge may still call it weak -> IKE test owed by the engine author. Files: internal/plugins/ospf/rfc4303_esp_probe_linux_test.go (new), internal/plugins/ospf/ipsec_install_test.go |
| RFC4303-2.1-2 | tests | + TestRFC4303InboundESPSAKeyedOnConfiguredDestination (unit: every installed state has the configured SPI, proto 50, a configured destination {fe80::1, ff02::5, ff02::6}, source unspecified); + TestRFC4303InboundESPMapsByConfiguredDestination (netns: ESP to fe80::1 from fe80::2 finds the SA: XfrmInNoStates +0, XfrmInStateProtoError +1); - TestRFC4303InboundESPToUnconfiguredDestinationMapsToNoSA (netns: same SPI to fe80::2, no state keyed there: XfrmInNoStates +1, nothing reaches OSPF). Both old tags removed from TestIPsecSAAddressMatchIndication (it asserts the traffic selector; comment rewritten, approval D-15) | all three: revert buildIPsecSA, observed red | enforced for the OSPF producer | same IKE-producer caveat as 2-1 (Support remaining now says so). Files: rfc4303_esp_test.go, rfc4303_esp_probe_linux_test.go, ipsec_install_test.go, rfc/short/rfc4303.md |

## Status (end)
All four ids done; nothing half-done. No code defect found (producers buildIPsecSA / validateIPsecInterface comply).
Verified: go test -race -run 'TestRFC4303|TestIPsecESPRequiresIntegrity|TestIPsecSAProtocolNumber|TestIPsecSAAddressMatchIndication|TestRFC4302' ./internal/plugins/ospf/ green (netns probes ran natively, not skipped).
Package lint (job ospf-esp-lint2): 2 findings, both in instance.go (helloHold/helloHeld unused) = the OSPF author's in-progress edit, not mine.
./le rfc check: rfc4303 lines are only STALE audit verdicts (1-1 text, 2-1, 2.1-2, 3.2-1, 3.4.3-2) + 2.1-1 SHIFTED (reseal). My unit edits also stale RFC4301-4.2-1 (TestIPsecESPRequiresIntegrity) and RFC4302-2-1 (TestIPsecSAProtocolNumber): comment-only tag removals, claims of those tags unchanged. New {gap} row RFC4303-1-2 needs a judge verdict.
OWED to main thread: full-package race run once the OSPF author's instance.go edit settles; lint; judge re-audit of rfc4303 (+ rfc4301 4.2-1, rfc4302 2-1 re-judge/reseal).
Open for another author: RFC4303-2-1 and 2.1-2 on the IKE Child SA producer (engine/child.go installChildSA) have no test; outside this package's files.

## Files changed
- internal/plugins/ospf/rfc4303_esp_test.go (new)
- internal/plugins/ospf/rfc4303_esp_probe_linux_test.go (new)
- internal/plugins/ospf/config_ipsec_test.go (RFC4303-1-1 negative tag removed from TestIPsecESPRequiresIntegrity)
- internal/plugins/ospf/ipsec_install_test.go (RFC4303-2-1 negative tag removed from TestIPsecSAProtocolNumber; RFC4303-2.1-2 tags removed from TestIPsecSAAddressMatchIndication, comment rewritten)
- rfc/short/rfc4303.md (1-1 narrowed, 1-2 gap row, 3.4.3-2 single-polarity marker removed, Meta Enrolment reason, Support remaining)
- rfc/corrections/rfc4303.md (new, Correction paragraph for 1-1)
- rfc/extraction/rfc4303.json (section 1 unsourced-ids RFC4303-1-2)
- rfc/discrimination/rfc4303.json (8 records)
- approvals: ospf.TestIPsecESPRequiresIntegrity, ospf.TestIPsecSAProtocolNumber, ospf.TestIPsecSAAddressMatchIndication
