# Handover: spec-dhcpv6-client-lease-maintenance (closed 2026-10-19)

**Closed 2026-10-19 by the close6 reviewer; see the last section.** Nothing
here is open but the whole-tree gates named there.

Spec: spec-dhcpv6-client-lease-maintenance, removed at closure (status was
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

## Continuation 2026-10-18 (second phase agent): written, green, committed

Committed in the change that carries this line ("feat(dhcp): resolv.conf
tracks DNS servers per lease source; DUID pattern"). The earlier block,
`cmd/ze/hub/main.go` carrying another session's `loadResult.StampRelease`
hunk, lifted once `LoadResult.StampRelease` reached HEAD
(`internal/component/config/loader.go`); that hunk rode along.

- A-7 (owner, verbatim: "yes, track DNS servers per source"; spec Q8, Key
  Design Decisions, A-7 confirmed). `internal/component/config/system/resolv.go`
  (new): `Resolvers`, the only resolv.conf writer, `ProcessResolvers()`,
  `Configure(path, static)`, `SetLease(source, servers)`; static alone when set
  (RFC 3646 Section 6), else DHCPv4 then DHCPv6 sources by interface then unit,
  deduplicated; `written` guard leaves a file ze never wrote alone.
  `resolv_linux.go`/`resolv_other.go`: `writeResolvFile`; `WriteResolvConf`
  deleted. `internal/plugins/iface/dhcp/resolv_linux.go`: `setV4Resolvers`,
  `setV6Resolvers` (RFC 2132 3.8, RFC 3646 3, RFC 9915 18.2.10.5 quoted);
  `writeResolvConfTo`, `clearResolvConfAt`, `v6Worker.writeDNS`,
  `dHCPConfig.ResolvConfPath`/`HasStaticNameServers`,
  `iface.SetDHCPSystemConfig` and two factory params deleted. Withdrawals in
  `teardownV4`, `v6Worker.maintain` (lease empty), `v6Worker.shutdown`;
  `v6Lease.apply` now replaces DNS with what each Reply carries. Hub:
  `main.go`, `main_system.go` `applyResolvConf` (reload now also withdraws).
  `internal/le/arch/fspersistence/fspersistence.go`: stale dhcp rule removed.
  Docs: `docs/features/dns-resolver.md`, `docs/guide/configuration.md`,
  `docs/features/interfaces.md`. `rfc/full/rfc3646.txt` fetched.
  Tests: `internal/component/config/system/resolv_test.go` (5 tests),
  `internal/plugins/iface/dhcp/resolv_linux_test.go`
  (`TestRFC9915Section182105RenewWithoutDNSStopsUsingIt`,
  `TestDHCPv6LeaseEndWithdrawsItsDNS`, `TestDHCPv4TeardownWithdrawsItsDNS`).
  Forced red observed for all four behaviors (restore -> green).
- Phase 5: `duid` pattern `[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){2,129}` (type code
  + 1..128 octets, RFC 9915 Section 11.1; the old "2..130" note was wrong) in
  `internal/component/iface/yang/ze-iface-conf.yang`;
  `TestDHCPv6DUIDPatternRefusesMalformed` in `config_test.go`, red without the
  pattern. Docs paragraph in `docs/features/interfaces.md`.
- Journal: `registry-contamination.md` new open row (iface tests fail at
  `-count=2`); status updates in `reconciler-claims-state-another-owner-installed.md`
  and `silent-fall-through.md` (items 9 below done).
- Green: `go test -race -count=1` over `iface/dhcp`, `config/system`,
  `component/iface`, `component/config`, `cmd/ze/hub`; arch tier,
  fs-persistence, iface-resolution all exit 0.

Still owed from the list below: 1 (enrolment, tags, discrimination records,
`// RFC:` headers), 3 (veth test), 4 (Kea interop), 6 for the phase-2..4 unit
tests, 7 (gates), 8 (A-1, A-2, A-3, A-8, A-9).

## Continuation 2026-10-19 (third phase agent): two commits, then handoff

- `b9405b0b6d` dhcp: scoped lint cleared in `dhcp_v6_exchange_linux.go` and
  `dhcp_v6_lease_linux.go` (godot: closing sentence after each RFC quote;
  `v6EndUnspecified`/`v6KindUnspecified` keep a reasoned `//nolint:unused`;
  `v6ElapsedTime` uses `min`). `./le job run ... golangci-lint run
  ./internal/plugins/iface/dhcp/...` exits 0. (`./le go lint run scope ...`
  refused: "lint inputs changed during planning", other sessions editing.)
- `ec35ab9a2c` A-2, A-3 confirmed by `TestDHCPv6LinkOneDatagramPerExchange`
  (`dhcp_v6_link_linux_test.go`, recording `PacketConn` under
  `nclient6.NewWithConn`); `dialV6Link` and the test share
  `v6ClientOptions()`. Forced red recorded (WithRetry(3), 50 ms timeout ->
  second datagram). A-8 confirmed by reading (no `dhcp-expired` subscriber;
  `syncWorker.handleDHCPEvent` in `internal/plugins/ntp/ntp.go` takes
  `dhcp-acquired` only). Spec rows updated.
- Both commits used `structural-red-ok` after `./le --name dhcp6c arch tier
  check` and `arch iface-resolution` exited 0.

Findings for the next agent (read before starting):

- Item 3 has a design gap: `iface.Resolve` is the only way the worker finds
  the OS device (`v6Worker.openOnce`), and the logical-to-OS mapping is set
  only by the unexported `resolver.setMapping` via `setResolverConfig`
  (`internal/component/iface/resolve.go`). No exported seam exists for a test
  in `ifacedhcp` to bind a logical name to a veth (the v4 labs pass the veth
  name itself, so they never test a selector). Choose: an exported test seam
  in `iface` (a production API added for a test) or driving the iface config
  apply path. A third route needs no production API: an `export_test.go` in
  package `iface` that exposes `globalResolver.setMapping` and the installed
  `dhcpClientFactory` (unexported var, `register.go`), used by an
  external `package iface_test` integration test in
  `internal/component/iface/` (precedent: `netlink_integration_import_test.go`)
  that blank-imports `ifacedhcp` and starts the client through the factory
  `register.go` `SetDHCPClientFactory` installs (the real entry point). The
  test file then lives under `internal/component/iface/`, not the path the
  spec's Wiring row names: update the row. Template lab: `sfSetupLab` (`dhcp_shortframe_integration_linux_test.go`),
  `lmStart` (`rfc2131_dhcp_lease_maintenance_integration_linux_test.go`); a v6
  server can be a UDP socket on the peer joined to ff02::1:2 port 547. The
  test is `//go:build integration && linux` and runs in QEMU
  (`docs/architecture/testing/qemu-integration.md`, one-test recipe), never
  with sudo here.
- Item 4 is a package of its own: the DHCPv4 precedent `cc69746db8` (dnsmasq)
  added a Go checker under `internal/le/interoplab/bgp/` (`check_dhcp.go`,
  `check_dhcp_predicate.go` + test, `names.go`, `prepare.go`,
  `register_dhcp.go`, `run.go`), the Dockerfile, the scenario files and the
  `docs/architecture/testing/interop.md` rows: about 700 lines.
- Item 1 is a package of its own: `rfc/full/rfc9915.txt` is 6391 lines with
  353 normative lines; `rfc/short/rfc9915.md` does not exist. `./le rfc check`
  judges every row in the corpus, so do not land a half-written summary.
  Discrimination records (`./le rfc discriminate-record`) need the tags,
  which need the summary.
- A-1 and A-9 stay `unvalidated` (enrolment walk; Kea run).

## Remaining (nothing here is optional)

1. Phase 1: enrol RFC 9915 per `/ze-rfc` (whole document; server/relay
   `binds-another-role`; Confirm, Information-request, Reconfigure, IA_TA,
   auth, Server Unicast, configurable rate limit (Q6), DUID-LL SHOULD NOT (Q4)
   recorded as gaps/deviations), extraction sign-off, then add polarity tags
   to the tests and discrimination records. Add `// RFC: rfc/short/rfc9915.md`
   headers to the three v6 files (the hook warns they lack it).
2. Phase 5: YANG pattern on `duid` (colon hex, 2..130 octets) +
   `TestDHCPv6DUIDPatternRefusesMalformed` in `internal/component/iface/config_test.go`.
3. DONE 2026-10-19 (see the fourth-agent section below):
   `TestDHCPv6RenewsOnSelectorBoundInterface`, now in
   `internal/component/iface/dhcp_v6_selector_renew_integration_linux_test.go`.
4. Phase 6: `test/interop/Dockerfile.kea`, scenario `dhcpv6-client-renew-kea`,
   discrimination walk recorded; `docs/architecture/testing/interop.md`.
5. A-7: Section 18.2.10 "stop using" DNS a Renew Reply omits is NOT
   implemented; `writeResolvConfTo` ignores an empty list and the file is
   shared with v4. Needs a decision on how to remove only v6 servers.
6. Discrimination not yet forced for the new unit tests (revert -> red).
7. Gates owed: `./le go lint run`, `./le test unit all`,
   `./le test functional gating` (verification debt
   `plan/verification-debt/65b935c4.md`).
8. A-1, A-9 still `unvalidated` in the spec table (A-2, A-3, A-8 confirmed
   2026-10-19).
9. `plan/journal/reconciler-claims-state-another-owner-installed.md` row and
   `plan/journal/silent-fall-through.md` 2026-10-18 row: status updates owed.

## Continuation 2026-10-19 (fourth phase agent): item 3 done

- Item 3 landed by the test-only route (main thread's decision):
  `internal/component/iface/export_test.go` (`SetResolverMappingForTest`,
  `DHCPClientFactoryForTest`) and
  `internal/component/iface/dhcp_v6_selector_renew_integration_linux_test.go`
  (package `iface_test`, blank-imports `ifacedhcp`). The client starts
  through the installed factory under the logical name `uplinkv6`, which only
  the resolver mapping binds to the veth. The server end lives in its own
  netns, so replies cross the veth. The test asserts Rapid Commit Solicit,
  three Renews, a Rebind at T2 (the server ignores the 4th Renew), and a
  Release on Stop. It checks the client and server ids and the IA_NA address
  in each message, the kernel lifetimes left at each message, DAD (tentative,
  then clear), the address removed after Stop, and the acquired/renewed
  events naming the logical name. A-4 and A-5 are now confirmed in the spec.
- QEMU: `go test -c -tags integration` on the host (CGO_ENABLED=0), then
  `./le test qemu run kernel tmp/kernel/build/vmlinuz command "cd
  /workspace/internal/component/iface && /workspace/<bin> -test.v -test.run
  '^TestDHCPv6RenewsOnSelectorBoundInterface$'"`. Green 11.4 s. Forced red
  through a `-overlay` making `extend` dial `w.c.ifaceName`: "waiting for
  Renew 1: the client sent SOLICIT". Restored: green.
- Untagged: there is no RFC 9915 summary yet, so item 1 may tag it later (it
  then owes polarity and `./le rfc discriminate-record`).
- Gates still owed for this file pair: none beyond item 7. The scoped
  `golangci-lint run --build-tags integration ./internal/component/iface/`
  reports 0 issues.

## Continuation 2026-10-19 (RFC 9915 enrolment agent): item 1 done

- `e96a8131d2` enrols RFC 9915: `rfc/short/rfc9915.md` (126 client rows,
  verbatim quotes), `rfc/extraction/rfc9915.json` (218 sites: 126 mapped,
  76 binds-another-role, 16 duplicate-of), `rfc/discrimination/rfc9915.json`
  (70 records, route revert, each red observed). Rows: 70 proven (all
  single-polarity), 50 not-applicable, 3 lower-layer, 2 gap
  (RFC9915-18.2.10.1-1 no DAD outcome/Decline, RFC9915-18.2.12-2 no
  link-change Rebind). New `rfc9915_dhcp_v6_messages_linux_test.go` (ten
  tests). `// RFC:` headers on the three v6 source files.
- Next commit renames `dhcp_v6_{exchange,lease,link}_linux_test.go` to
  `rfc9915_dhcp_v6_*` (`./le rfc rename`, file-name rule); spec paths updated.
- Item 5 (A-7) is already met: `75cbddde58` empties the v6 DNS list on a
  Reply without the option; RFC9915-18.2.10.5-1 (NTP) is not-applicable.
- Not done: SHOULD/MAY rows are not in the summary (the gate needs MUST-level
  only); no `/ze-rfc-audit` verdicts yet; the selector-bound integration test
  is untagged; `dhcp_v6_pd_linux_test.go` has no `// RFC:` header (no tag).
- Owner decisions the summary records as not offered (disclosed in Support
  remaining, not gated): Confirm, Information-request, Reconfigure, Auth,
  DUID-LL default (Q4), fixed rate limit (Q6).

## Status of the Remaining list, 2026-10-19 (v6gap agent)

| Item | Status |
|------|--------|
| 1 RFC 9915 enrolment | DONE: `e96a8131d2` (summary, extraction, 70 records), `aad1449529` (test files renamed `rfc9915_*`) |
| 2 duid pattern | DONE (`75cbddde58`) |
| 3 selector-bound veth test | DONE: `7d9e1819aa` |
| 4 Kea interop | DONE: `8bd003c1ce`, scenario `dhcpv6-client-renew-kea`; A-9 confirmed in the spec |
| 5 A-7 stop using omitted DNS | DONE: already implemented by `75cbddde58` |
| 6 discrimination for unit tests | DONE for every tagged test (enrolment records + this agent's 13) |
| 7 gates | OWED: `./le go lint run`, `./le test unit all`, `./le test functional gating` (not run: owner said scoped only) |
| 8 A-1 | still `unvalidated` |

## The two RFC 9915 MUST gaps (owner ruling 2026-10-18), v6gap agent

Spec AC-26..AC-29 added. Code: `internal/plugins/iface/dhcp/dhcp_v6_attach_linux.go`
(new: `checkDAD`, `v6DADOf`, `decline`, `buildDecline`, `waitLease`,
`linkMoved`, `noteMove`, `rebindAfterMove`), `dhcp_v6_linux.go` (`maintain`
waits through `waitLease`; `extend` takes its retransmission parameters;
`releaseMatcher` renamed `completionMatcher`), `dhcp_linux.go`
(`DHCPClient.v6Links`, default `iface.Subscribe`), `iface.AddrInfo.DADFailed`
set from IFA_F_DADFAILED in `internal/plugins/iface/netlink/show_linux.go`.
Finding: the kernel DELETES a finite-lifetime address whose DAD failed
(`addrconf_dad_stop`, no DADFAILED flag in the RTM_DELADDR); only an infinite
one keeps `dadfailed`. So an installed address gone while the link is up
counts as a duplicate. Proven on a real kernel in QEMU by
`internal/component/iface/rfc9915_dhcp_v6_dad_integration_linux_test.go`
(peer holds the leased address nodad; the Decline arrives from fe80::).
Not done: a kernel-level test of the link-move Rebind (unit tests inject
`iface.LinkEvent`s; the monitor-to-resolver-to-Subscribe chain is not driven
in a test).

## Hand-off for verification, 2026-10-19 (v6fin agent)

Spec status is now `verification`. Nothing in the implementation list is open.

| Item | Result |
|------|--------|
| Kernel link-move test | `rfc9915_dhcp_v6_link_move_integration_linux_test.go`: the netlink monitor runs on a bus the resolver is bound to (`BindResolverEventsForTest`, `export_test.go`). A carrier flap of the peer end brings a Rebind with IA_NA and IA_PD; an MTU change (up, no down) sends nothing. Green in QEMU; red under a revert of `rebindAfterMove` and under the mutant `!w.linkDown` -> `false`, both recorded with `kernel` |
| 18.2.12-2 negative | The mutant record replaces the weak revert record on `TestRFC9915Section18212NoMoveNoRebind` |
| A-1 | Broken, no behavior effect: Table 1 and the constants match; 18.2.10.5 exists in 9915 only; four 8415-text quotes fixed, 52/52 now verbatim |
| Stale records | RFC9915-18.2.7-6/-7 on `shutdown` re-recorded; RFC2131-4.4.5 x4 in `6e2a3936bf` |

Owed by the verifier: `./le go lint run`, `./le test unit all`,
`./le test functional gating` (debt `65b935c4`, `38d8187a`). `./le rfc check`
has no finding on rfc9915 or iface/dhcp.

## Closure, 2026-10-19 (close6 reviewer)

Two ISSUEs found and fixed in commit A, then the spec was removed in commit B.

| Finding | Fix |
|---------|-----|
| 39 `{not-applicable}` rows in `rfc/short/rfc9915.md` excused MUSTs conditional on OPTIONAL features ze declined (Confirm, Information-request, Reconfigure, Authentication, User Class, Vendor-specific, NTP, DUID-LLT/EN, delegated-prefix use, MRC/MRD, Client-ID omission) | re-annotated `{feature-declined}` with the RFC's optionality sentence and the producer; Support remaining names NTP and delegated-prefix use; 5 vacuous unconditional rows stay `{not-applicable}` |
| After an interface was set down and up, the kernel had deleted the finite leased addresses, and the next DAD read found them gone with the link up and declined every one (red: `DECLINE@5s`) | `v6Worker.noteMove` installs every live address again with its remaining lifetimes; `checkDAD` reads queued link events first; `TestDHCPv6AddressFlushedByLinkDownReinstalledNotDeclined` |

Still owed (owner: not run here): `./le go lint run`, `./le test unit all`,
`./le test functional gating`, and the QEMU integration tests under
`internal/component/iface/` (`TestRFC9915LinkMoveRebindsThroughTheMonitor`
now also runs the reinstall on a carrier flap).

2026-10-19 review of 1cb92a54be (rv6): `noteMove` truncated a sub-second valid
lifetime left to 0, and 0/0 reaches netlink as no lifetime (permanent); it now
rounds the valid lifetime up (`v6SecondsUp`,
`TestDHCPv6MoveReinstallsAnAddressNearExpiryAsFinite`). The four QEMU tests
plus `TestDHCPv6CarrierFlapKeepsTheLeasedAddress` and
`TestDHCPv6AdminFlapReinstallsTheLeasedAddress` (new, untagged) pass on the
kernel. Two journal rows opened: the DAD/LinkDown race in `checkDAD`, and link
events unread during an exchange.

2026-10-19 (dadfix): the `checkDAD` race row is fixed (owner option (a): kernel link state read with the addresses, `goneDuplicates`); QEMU `TestDHCPv6AdminDownDuringDADBeforeItsEventNotDeclined` red under a revert, green with the fix; three rfc9915 records re-taken.
