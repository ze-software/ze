# dhcpserver + iface/dhcp author handoff (child: spec-rfc-verdict-fix-services, 2026-09-29)

STATUS: continuation 1 finished; every verdict in scope resolved. Nothing unresolved.

Scope: 6 weak/wrong verdicts tagging `internal/plugins/dhcpserver` (rfc2131 x4, rfc2132 x1 wrong, rfc4578 x1), none under "Blocked by", plus the narrowing row RFC2131-4.1-7. RFC2131-2-3 also tags `internal/plugins/iface/dhcp` (client clause).

| id | resolution | what now proves each clause (+/-) | records | expected verdict | notes |
|----|-----------|-----------------------------------|---------|------------------|-------|
| RFC2131-2-3 | defect (D-8, client) + tests | server clause: TestFlagsReservedBitsIgnored +/- (rfc2131_test.go, unchanged); client clause: TestRFC2131ClientSetsReservedFlagsBitsToZero +/- (iface/dhcp/dhcp_flags_integration_linux_test.go, guest unit) | client + and - (revert, guest) | enforced | Defect: vendored NewRequestFromOffer / NewRenewFromAck copy the server's flags word. Fix: clearReservedFlags modifier first in v4RequestModifiers and passed to client.Renew (dhcp_v4_linux.go), RFC quote above it |
| RFC2131-4.1-6 | defect (D-8, server) + tests | TestRFC2131EveryReplyOptionContainedInItsField +/- (rfc2131_identity_options_test.go); existing rfc2131_test.go tags unchanged | + and - (revert) | enforced | red-first: a 300-octet bootfile wrote option 67 with length octet 44 and overran the options field. Fix: safeAppendOption omits data over 255 octets; parsePXEConfig refuses bootfile-bios/uefi over 255 (config_test.go) |
| RFC2131-4.2-2 | tests | TestRFC2131ChaddrContentsIdentifyTheClient +/- | + and - | enforced | old hlen=0 negative tag removed from TestUnidentifiableClientRejected (approved D-15) |
| RFC2131-4.3.1-2 | tests + row | TestRFC2131ConfiguredParameterValuesReturned +/- | + and - | enforced | {single-polarity} removed from the row |
| RFC2132-2-1 | row (R2 retire) | its tag on TestEveryEmittedOptionHasLengthOctet moved to new row RFC2132-2-5 | n/a | retired | Retired paragraph in rfc/corrections/rfc2132.md; extraction site 2:3 excluded binds-another-role. The audit entry for 2-1 in rfc/audit/rfc2132.json is the judge's to handle |
| RFC2132-2-5 | row (new, D-3 lowercase must) | TestEveryEmittedOptionHasLengthOctet + ; row {single-polarity: positive} | + (revert) | enforced | id anchored: cites §2, above high-water 2-4; listed under unsourced-ids in the §2 extraction entry |
| RFC4578-2.1-2 | tests | TestRFC4578ClientArchLengthEvenAndPositive +/- (rfc4578_arch_test.go); handler_test.go tags unchanged | + and - | enforced | |
| RFC2131-4.1-7 (narrowing) | row split + R3 | new row RFC2131-4.1-15 {gap}; 4.1-7 changed from {not-applicable} to {gap} (continuation 1) | none | unimplemented (gap) | 4.1-15 id anchored: cites §4.1, above high-water 4.1-14. Summary count 19 -> 20 MUST gaps; Correction paragraph added |

## Files changed (whole package work, both authors)

- internal/plugins/dhcpserver/handler.go
- internal/plugins/dhcpserver/config.go
- internal/plugins/dhcpserver/handler_test.go
- internal/plugins/dhcpserver/config_test.go
- internal/plugins/dhcpserver/rfc2131_test.go
- internal/plugins/dhcpserver/rfc2131_identity_options_test.go (new)
- internal/plugins/dhcpserver/rfc4578_arch_test.go (new)
- internal/plugins/iface/dhcp/dhcp_v4_linux.go
- internal/plugins/iface/dhcp/dhcp_test.go
- internal/plugins/iface/dhcp/dhcp_flags_integration_linux_test.go (new)
- rfc/short/rfc2131.md, rfc/short/rfc2132.md
- rfc/corrections/rfc2131.md (new), rfc/corrections/rfc2132.md (new)
- rfc/extraction/rfc2131.json, rfc/extraction/rfc2132.json
- rfc/discrimination/rfc2131.json, rfc/discrimination/rfc2132.json (new), rfc/discrimination/rfc4578.json (new)
- docs/architecture/provisioning/dhcp-server.md, docs/features/interfaces.md

## Continuation 1 log (2026-09-29)

- iface/dhcp host build: green. `go test ./internal/plugins/iface/dhcp/` ok; `go vet -tags integration` ok; gofmt clean on both packages. msg.HostName() exists in the pinned dhcpv4, so no change was needed.
- RFC2131-4.1-7: {not-applicable} -> {gap} (R3, consistent with 4.1-15); Correction paragraph in rfc/corrections/rfc2131.md; Support remaining now "Twenty MUST gaps" and names 4.1-7. The row edit was made with a python replace, not the Edit tool.
- New ids checked: RFC2132-2-5 (§2, max was 2-4), RFC2131-4.1-15 (§4.1, max was 4.1-14). Both anchored.
- dhcpserver package: `go test ./internal/plugins/dhcpserver/` ok.
- RFC2131-2-3 client records WRITTEN, both polarities: revert of dhcp_v4_linux.go::clearReservedFlags, guest unit, kernel tmp/kernel/build/vmlinuz (logs scratch/rec-2131-2-3-{positive,negative}.log). Observed red: "waiting for DISCOVER: read packet ... i/o timeout" at test line 44. The revert panic is caught by the recover in dhcp_linux.go, so the client stops before it sends a DISCOVER, and the red fires before either tagged assertion.
- Extra evidence for the judge (not a record): unbroken guest run PASS (scratch/guest/baseline.log). Guest run with the body of clearReservedFlags made a no-op (Go overlay, `d.Flags &= flagsBroadcast` -> `_ = d`; scratch/guest/noop.log): the NEGATIVE assertion goes red at line 72, "REQUEST flags = 0xffff, reserved bits 0x7fff copied from the OFFER". So the negative tag discriminates the defect it names.
- Weak point for the judge: the POSITIVE tag (DISCOVER reserved bits zero) stays green under the no-op break, because nclient4 builds the DISCOVER with flags 0 whether or not ze clears them. Only the revert crash reddens it. If the judge wants a targeted positive, the candidate is a real gomu run over dhcp_v4_linux.go in the guest; none was run.

# RFC2131-2-3

Continuation 2 (2026-09-30). The audit note (weak) named one gap: the renewal path (renewV4 passes clearReservedFlags to client.Renew) had no test, so dropping that argument stayed green.

| id | resolution | what now proves each clause | records written | expected verdict | notes |
|----|-----------|-----------------------------|-----------------|------------------|-------|
| RFC2131-2-3 | tests | server clause unchanged (TestFlagsReservedBitsIgnored +/-); client acquisition unchanged (TestRFC2131ClientSetsReservedFlagsBitsToZero +/-); client RENEWAL: new TestRFC2131ClientRenewalSetsReservedFlagsBitsToZero (-): ACK flags 0xffff, 2 s lease, T1 renewal REQUEST read (told apart by ciaddr, because NewRenewFromAck copies the ACK's xid) and its bits 1-15 asserted zero | negative, revert of dhcp_v4_linux.go::renewV4, guest (scratch/rec-2131-2-3-renewal.log; red: renewal REQUEST read timeout, line 143) | enforced | Targeted evidence (not a record): Go overlay dropping the argument (`client.Renew(ctx, lease)`), guest run scratch/guest/droparg.log: the new assertion goes red at line 157, "renewing REQUEST flags = 0x7fff, reserved bits 0x7fff copied from the ACK". Unbroken: scratch/guest/baseline2.log PASS (3 units). Approval: dhcp.TestRFC2131ClientRenewalSetsReservedFlagsBitsToZero (D-15, own new unit). Gates: host race on iface/dhcp ok, vet -tags integration ok, gofmt clean |

Files changed (continuation 2):
- internal/plugins/iface/dhcp/dhcp_flags_integration_linux_test.go (header comment + new test function)
- rfc/discrimination/rfc2131.json (one negative record)
- plan/pre-release/spec-rfc-verdict-fix-services.md (Blocked-by cell RFC7950-7.19-1: weak -> wrong, matching rfc/audit/rfc7950.json)
