# Handover: spec-dhcpv6-client-lease-maintenance (continuation needed)

Spec: `plan/immediate/spec-dhcpv6-client-lease-maintenance.md` (status
in-progress, Handoff verify). Code commit: `6b5c69ec11`. Written 2026-10-18 by
the phase agent at its 100-call budget.

## Done in 6b5c69ec11 (spec phases 2, 3, 4)

- `internal/plugins/iface/dhcp/dhcp_v6_linux.go`: `v6Worker` owned by the v6
  goroutine; `runV6` -> `identify` (DUID: `duid` leaf via `parseV6DUID`, else
  DUID-LL of the MAC; IAID = MAC last 4 octets, PD IAID first octet ^ 0xff) ->
  `acquire` (`solicit`, `request`, `takeLease`) -> `maintain` (`extend` Renew
  then Rebind, `applyExtension` with NoBinding -> Request) -> `shutdown`
  (remove addresses, then Release bounded by `v6ReleaseBound` 3 s). Leased
  addresses registered with `iface.RegisterLeasedAddresses` (`registerLeased`).
  `renewV6`, `v6Timers`, `handleV6Reply` deleted.
- `dhcp_v6_exchange_linux.go`: Table 1 constants, `v6Link` (window owned by the
  link on the injected clock), `runExchange` (Section 15 RT, MRC/MRD, Elapsed
  Time), rate limiter 20/20 s, `replyMatcher`, `v6AdvertiseCollector`,
  builders (`buildSolicit`, `buildRequest`, `buildFromLease`, `setLeaseIAs`,
  `emptyIAPD`).
- `dhcp_v6_lease_linux.go`: `v6Lease.apply` (18.2.10.1, 21.4/21.21 T1>T2,
  21.6/21.22 preferred>valid, valid 0 removes, 16-binding cap, omitted IA
  retry delay), `renewTimes`/`iaTimes` (earliest T1/T2, client-chosen 0.5/0.8,
  infinity), `lastExpiry`.
- `pdRequested` threaded through `iface.SetDHCPClientFactory`
  (`internal/component/iface/config.go`, `register.go`); YANG `pd`
  descriptions corrected; `docs/features/interfaces.md` DHCPv6 paragraph and
  rows updated; `ifaceresolution.go` allowlist entry for `dialV6Link`.
- Tests (green: `./le job run label dhcpv6-unit command go test -race
  -count=3 ./internal/plugins/iface/dhcp/` and `-count=1
  ./internal/component/iface/`): every `dhcp_v6_exchange_linux_test.go` and
  `dhcp_v6_lease_linux_test.go` row of the TDD plan except the two below, plus
  `TestDHCPv6LeaseBindingsCapped`. Names use `RFC9915`, carry NO
  `RFC requirement:` tags (RFC 9915 not enrolled yet).

## Remaining (nothing here is optional)

1. Phase 1: enrol RFC 9915 per `/ze-rfc` (whole document; server/relay
   `binds-another-role`; Confirm, Information-request, Reconfigure, IA_TA,
   auth, Server Unicast, configurable rate limit (Q6), DUID-LL SHOULD NOT (Q4)
   recorded as gaps/deviations), extraction sign-off, then add polarity tags
   to the tests and discrimination records. Add `// RFC: rfc/short/rfc9915.md`
   headers to the three v6 files (the hook warns they lack it).
2. Phase 5: YANG pattern on `duid` (colon hex, 2..130 octets) +
   `TestDHCPv6DUIDPatternRefusesMalformed` in `internal/component/iface/config_test.go`.
3. `TestDHCPv6RenewsOnSelectorBoundInterface` (integration, veth lab, Go
   server, `dhcp_v6_lease_integration_linux_test.go`) -- validates A-4, A-5.
4. Phase 6: `test/interop/Dockerfile.kea`, scenario `dhcpv6-client-renew-kea`,
   discrimination walk recorded; `docs/architecture/testing/interop.md`.
5. A-7: Section 18.2.10 "stop using" DNS a Renew Reply omits is NOT
   implemented; `writeResolvConfTo` ignores an empty list and the file is
   shared with v4. Needs a decision on how to remove only v6 servers.
6. Discrimination not yet forced for the new unit tests (revert -> red).
7. Gates owed: `./le go lint run`, `./le test unit all`,
   `./le test functional gating` (verification debt
   `plan/verification-debt/65b935c4.md`).
8. A-1, A-2, A-3, A-8, A-9 still `unvalidated` in the spec table.
9. `plan/journal/reconciler-claims-state-another-owner-installed.md` row and
   `plan/journal/silent-fall-through.md` 2026-10-18 row: status updates owed.
