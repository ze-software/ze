# Handover: DHCPv4 lease maintenance (spec-dhcpv4-client-drops-lease-every-period)

Written 2026-10-18 by the ze-implement agent at its 100-call budget. Spec:
`plan/immediate/spec-dhcpv4-client-drops-lease-every-period.md` (in-progress,
Phase 5/6, claimed by session 20123ea5). Handoff field `-`: closure is `/ze-close`.

## Done (committed with this file)

Phases 1 to 4 of the spec, code and unit tests:

- `internal/test/sim/sim.go`: `FakeClock.AwaitTimers`, `FakeClock.NextTimer`
  (D-11), `TestFakeClockAwaitTimers`.
- `internal/plugins/iface/dhcp/dhcp_linux.go`: `DHCPClient.clock` and
  `v4Exchange`; `newDHCPClient(..., cfg, clk)` (clk required);
  `sleepOrStop` on `clock.AfterFunc`, `sleepUntilOrStop`. The DHCPv6 spec
  reuses this clock field unchanged.
- `dhcp_v4_exchange_linux.go` (new): `v4Exchanger` (`acquire`, `extend`),
  `nclient4Exchanger`, `v4Attempt`, `v4Reply`, outcome and delivery enums,
  `v4ExtendAccepts` (D-10).
- `dhcp_v4_linux.go`: `runV4` / `acquireV4` / `holdV4` / `extendV4` state loop;
  `v4NextAttempt`, `v4LeaseFromAck`, `v4LeaseTime` (refusals, no 1 h default),
  `v4LeaseTimers` (options 58/59), `installV4Lease`, route retry
  (`applyV4Route`, `installV4Route`, `removeV4Route`), `teardownV4`. Old
  `renewV4`, `handleV4Lease`, `removeV4Addr`, `holdInfiniteV4` deleted.
- Unit tests green (`go test -race -count=3` on the package and `internal/test/sim`):
  every test of the spec's Unit Tests table, in `dhcp_v4_loop_linux_test.go`,
  `dhcp_v4_schedule_linux_test.go`, `dhcp_v4_rig_linux_test.go` (rig),
  `dhcp_payload_linux_test.go` (`TestV4AckWithoutLeaseTimeRefused` replaces
  `TestV4LeaseTime`), `dhcp_test.go`. AC-1..AC-23 covered at unit level.
- Red walk: with `extendV4` returning the old lease on ACK and the route retry
  disabled, `TestDHCPv4LoopHoldsLeaseFiveLeasePeriods` failed with the defect-1
  shape (one RENEW, then REBIND, expiry, DORA every 10 s) and
  `TestDHCPv4RouteInstallRetried/recovers` failed; restored, green.
- `docs/features/interfaces.md` DHCP rows and anchors updated;
  `internal/component/iface/register.go` comment names `installV4Lease`.
- `sfSetupLab` adds a permanent neighbor for `sfServerIP` so the unicast RENEW
  crosses the veth (A-2 broken, replaced; see the spec's Assumptions).

## Remaining

| Item | Note |
|------|------|
| Phase 5: QEMU run of the package's integration tests | `./le test qemu run` (package listed in `internal/le/test/qemu/alltests.go`). `TestRFC2131ClientRenewalSetsReservedFlagsBitsToZero` now sees a unicast RENEW; it depends on the neighbor entry. Not run yet |
| Phase 5: the six veth tests in `rfc2131_dhcp_lease_maintenance_integration_linux_test.go` | not written |
| Phase 5: `rfc/short/rfc2131.md` rows, Meta, tags, `./le rfc discriminate-record`, regenerate `docs/features/rfc-status.md` | not started |
| Phase 6: dnsmasq sidecar, two interop scenarios, red/green walk | not started |
| A-1, A-3, A-4, A-5, A-6 | unvalidated |
| Gates owed | `./le go lint run`, `./le test unit all`, `./le test functional gating`, `./le rfc check` |
| RFC-tagged test bodies touched | `rfc2131_dhcp_flags_integration_linux_test.go` (constructor argument, one doc comment) and `rfc2131_dhcp_multiiface_integration_linux_test.go` (constructor argument): committed under `rfc-change-ok`; owner approval owed |

## Update 2026-10-18, second ze-implement agent (Phase 5 partly, at its budget)

Done, committed with this update:

- Defect found and fixed (spec D-16): the RENEW socket bound to the leased
  address never received a DHCPNAK the server broadcasts (RFC 2131 Section
  4.1, giaddr zero). `dhcp_v4_exchange_linux.go`: `newV4UnicastClient` and
  `v4UnicastConn` send through the bound UDP socket and read from the raw
  device socket. Red walk in QEMU with `WithUnicast` restored:
  `TestDHCPv4NakOnRenewRestartsOnVeth` FAIL ("address 192.0.2.50 present =
  true after 1s"), `TestDHCPv4RenewsRepeatedlyOnVeth` PASS; green with the fix.
- `rfc2131_dhcp_lease_maintenance_integration_linux_test.go`: the six veth
  tests of the spec plus `TestDHCPv4RenewalMovesToNewAddressOnVeth` (D-17) and
  `TestDHCPv4ExtendIgnoresServerEchoesOnVeth` (negative polarities). Whole
  package green in the QEMU guest (`./le test qemu run kernel
  tmp/kernel/build/vmlinuz`), `TestRFC2131ClientRenewalSetsReservedFlagsBitsToZero`
  included (A-2 replacement proven).
- `rfc/short/rfc2131.md`: annotations dropped on 2-2, 3.1-5, 4.1-10, 4.3.2-5,
  4.4.5-1, 4.4.5-5, 4.4.5-6, 4.4.5-7 (each now tagged both polarities);
  stale citations of deleted functions rewritten on 3.1-6, 3.1-8, 3.5-1,
  4.1-8, 4.1-9; Meta: fifteen MUST gaps, coverage text.
- `docs/features/interfaces.md` NAK row names the socket pair. Spec: A-1,
  A-2, A-6 judged; D-16, D-17 added.

How to run the package in QEMU while another session's DHCPv6 files do not
compile: build an overlay hiding them (untracked foreign files mapped to "",
modified foreign files mapped to their HEAD copy), then
`go test -c -tags integration -overlay <o> -o <bin> ./internal/plugins/iface/dhcp/`
and `GOFLAGS=-overlay=<o> ./le test qemu run kernel tmp/kernel/build/vmlinuz
command "cd /workspace/internal/plugins/iface/dhcp && /workspace/<bin> -test.v"`
(the guest `le` build compiles the package too).

Remaining:

| Item | Note |
|------|------|
| `./le rfc discriminate-record` for every new tag (16 tags, 9 requirements) plus the four stale records `./le rfc check` names (RFC2131-2-3 x3, RFC2131-3.6-1: their producers `renewV4`/`handleV4Lease` were deleted) | Guest units: `kernel tmp/kernel/build/vmlinuz`. BLOCKED while the DHCPv6 session's files in the package do not compile: the recorder sets its own GOFLAGS overlay, so a hiding overlay cannot be passed. Candidate producers: 4.4.5-1 `extendV4` / `teardownV4`; 4.3.2-5 `sendV4Extend`; 4.4.5-6, 4.4.5-7, 3.1-5 `v4ExtendRequest`; 4.1-10 `newV4UnicastClient`; 2-2 `v4RequestModifiers`; 4.4.5-5 `removeV4Address` |
| Stale audit verdict RFC2131-2-3 (and any other `./le rfc check` names for rfc2131) | re-judge via `./le rfc audit-stamp ... mode rejudge` |
| Regenerate `docs/features/rfc-status.md` | derived; `./le rfc index-update` |
| Phase 6 (dnsmasq sidecar, both scenarios, red/green walk) | not started. Precedent: `test/interop/Dockerfile.keepalived`, `internal/le/interoplab/bgp/run.go` (image list), `prepare.go` (peer config keyed on a scenario file), `names.go`, `checkers.go` |
| A-3, A-4, A-5 | unvalidated (interop) |
| Gates owed | `./le go lint run` (scoped to the dhcp package), `./le rfc check`, `./le test unit all`, `./le test functional gating` |
| COMMIT NOT MADE: this update and the Phase 5 work are uncommitted | `./le commit create` refused: `./le arch iface-resolution` flags `net.InterfaceByName(attempt.device)` in `newV4UnicastClient` (`dhcp_v4_exchange_linux.go`), needed for the hardware address `nclient4.NewWithConn` takes. `attempt.device` is already `iface.ResolveDevice` output (`sendV4Extend`), so the fix is an `allowlist` entry in `internal/le/arch/ifaceresolution/ifaceresolution.go` for that file with that reason, or reading the MAC through an iface op. The DHCPv6 session's `dhcp_v6_exchange_linux.go` draws the same finding. Then commit the six files: `dhcp_v4_exchange_linux.go`, `rfc2131_dhcp_lease_maintenance_integration_linux_test.go`, `rfc/short/rfc2131.md`, `docs/features/interfaces.md`, the spec, this handover |
