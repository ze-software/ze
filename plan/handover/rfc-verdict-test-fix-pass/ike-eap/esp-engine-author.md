# ike-eap esp-engine author (rfc4303, internal/component/ike/engine)

| id | resolution | what proves each clause | records | expected verdict | notes |
|----|------------|-------------------------|---------|------------------|-------|
| RFC4303-2-1 | tests | + TestRFC4301ChildSAIsInstalledAsESP (engine/rfc4301_esp_test.go, tag added, body unchanged, approval D-15): both IKE Child SAs installed with Proto 50. OSPF + wire probe and OSPF negative unchanged | positive, route revert, producer engine/child.go::installChildSA, OBSERVED red | strong | no new negative: the row's held negative (OSPF integrity-only-not-AH) stands; IKE has no AH/other-protocol input to force (no ENCR_NULL, protoESP constant) |
| RFC4303-2.1-2 | tests | + TestRFC4303InboundChildSAKeyedOnNegotiatedSPIAndLocalAddress (new, engine/rfc4303_inbound_key_test.go): the one inbound SA carries the negotiated SPI (ChildInboundSPI), Proto 50, Dst = negotiated local. - TestRFC4303InboundESPToAnotherLocalAddressMapsToNoSA (new, engine/rfc4303_inbound_key_linux_test.go, rootless netns): control to 10.250.0.1 found (XfrmInNoStates +0, delivered); same packet to 10.250.0.3 XfrmInNoStates +1, not delivered | positive + negative, route revert, producer installChildSA, OBSERVED red | strong | the netns unit ran its body in the child namespace (PASS in probe output) |

Verified: `go test -race -count=1 -tags ze_ike -run ...` under ./le job run: 6 PASS (log esp-engine-test.log). Records log: esp-engine-records.log.

Gates: gofmt clean; golangci-lint --build-tags ze_ike on engine: 0 issues (esp-engine-lint.log). `./le rfc check` (esp-engine-rfccheck.log): the only rfc4303 lines are the two expected STALE audit verdicts (RFC4303-2-1, RFC4303-2.1-2) awaiting the judge; no line for RFC4301-3.2-1. Owed to judge: re-judge + audit-stamp both ids; full-package -race run of engine if the judge wants it.

Files changed:
- internal/component/ike/engine/rfc4301_esp_test.go (tag line + RFC header line)
- internal/component/ike/engine/rfc4303_inbound_key_test.go (new)
- internal/component/ike/engine/rfc4303_inbound_key_linux_test.go (new)
- rfc/short/rfc4303.md (Support remaining line only: was stale once the IKE producer is proven; rows untouched)
- rfc/discrimination/rfc4303.json (3 records)
- tmp/commit-rfc-approved-01a40e57.md (approval for engine.TestRFC4301ChildSAIsInstalledAsESP)
