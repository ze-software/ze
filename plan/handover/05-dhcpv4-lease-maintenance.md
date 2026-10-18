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
