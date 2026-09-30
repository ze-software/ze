# vrrp tag-author handoff (2026-09-29)

| id | resolution | what now proves each clause | records written | expected verdict | notes |
|----|------------|-----------------------------|-----------------|------------------|-------|
| RFC9568-7.2-1 | tests | TestTransmittedAdvertFilledFromInstanceState (+), subtests ipv4 and ipv6: an Active VRRPv3 instance's doSendAdvert hands the transport version 3, the effective priority (200, then 150 after tracked eth1 fails), the configured 1000 ms and both VIPs in order; WriteTo octets of that state compared with literals (header 31 0a 96 02 00 64 plus address octets). Joins the existing packet goldens and transport frame test. | rfc9568 RFC9568-7.2-1 positive, route revert, producer instance.go::doSendAdvert, observed red (unit-sha 5e52bfe616aa357b) | strong | Claim states field fill only; checksum not claimed for 9568 (IPv4 deviation lives on RFC9568-7.2-5). |
| RFC5798-7.2-1 | tests (unit edited) | same unit; assertions unchanged, now also over IPv6 | rfc5798 RFC5798-7.2-1 positive re-recorded (unit-sha changed), route revert, observed red | unchanged (row is superseded) | |

Approval: `./le rfc approve unit vrrp.TestTransmittedAdvertFilledFromInstanceState reason "D-15: ..."` -> tmp/commit-rfc-approved-01a40e57.md.

Gates run: `go test -race ./internal/plugins/vrrp/...` green (ipv4 and ipv6 subtests pass); `golangci-lint run ./internal/plugins/vrrp/` 0 issues; gofmt clean.
Gates owed to main thread/judge: `./le rfc check`, reseal/index-update for rfc9568 and rfc5798 (not run: judges only).

Files changed:
- internal/plugins/vrrp/advert_fill_rfc5798_test.go
- rfc/discrimination/rfc9568.json
- rfc/discrimination/rfc5798.json
- tmp/commit-rfc-approved-01a40e57.md (approval)
