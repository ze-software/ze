# Spec: dhcpv4-client-drops-lease-every-period

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance. -->

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The DHCPv4 client drops its address and default route once per lease period,
even when the server renews the lease every time it is asked. An operator with a
DHCP-addressed uplink sees the address vanish, the default route go with it, and
a fresh DORA exchange bring both back, on a cycle equal to the first lease time.

Owner instruction, 2026-10-18: "Fix the bugs identified". The scope is the
DHCPv4 client's lease maintenance in `internal/plugins/iface/dhcp/`: every defect
below, each CONFIRMED by reading the producer on 2026-10-18 (none reproduced at
runtime yet; the TDD plan reproduces each one red at HEAD).

| # | Defect | Producer (HEAD) | Source |
|---|--------|-----------------|--------|
| 1 | The lease loop renews at most once. After one successful RENEW it sleeps `t2 - t1`, skips REBIND because `renewed` is true, sleeps `leaseTime - t2`, then removes the address and publishes `dhcp-expired`. Measured from the renewal the address goes after half the new lease, and the outer loop restarts DORA from nothing | `runV4` in `dhcp_v4_linux.go` | spec skeleton 2026-10-03 |
| 2 | Renewal binds to the logical name. Acquisition opens `nclient4.New(binding.OsName)`; `renewV4` opens `nclient4.New(c.ifaceName)`, so on an `os-name` or `mac/match` entry every renewal fails or uses the wrong device | `renewV4` | skeleton; `plan/journal/identity-default-hides-a-mapping.md` 2026-10-18 |
| 3 | A failed default-route install is never retried: an `iface.AddRoute` error is logged at Warn and the lease is published as held | `handleV4Lease` | skeleton |
| 4 | A DHCPNAK to RENEW counts as one more failed renewal (`nclient4.ErrNak` is just an error to `renewV4`), so the client keeps the refused address until expiry | `renewV4` | `plan/journal/silent-fall-through.md` 2026-10-18 (VyOS T9108) |
| 5 | REBIND at T2 reuses `nclient4.Renew`, whose matcher is `IsCorrectServer(lease.Offer.ServerIdentifier())`, so an ACK from any other server is dropped: REBINDING contacts "any server" only on paper | `runV4` calling `renewV4` a second time; vendored `nclient4.(*Client).Renew` | same row |
| 6 | Each of RENEWING and REBINDING sends one exchange (the library's 3 tries over 35 s) and then sleeps to the next phase: no retransmission on the RFC 2131 Section 4.4.5 schedule | `runV4` | same row |
| 7 | An ACK with no IP address lease time option, or a lease time of 0, gets an invented one-hour lifetime | `v4LeaseTime` (`IPAddressLeaseTime(time.Hour)` plus a `<= 0` guard) | `plan/journal/zero-value-as-valid-answer.md` 2026-10-18 |

Found while designing, same worker, folded subject to the owner questions:

| # | Defect | Producer (HEAD) | Owner question |
|---|--------|-----------------|----------------|
| 8 | The RENEWING REQUEST is broadcast to 255.255.255.255:67 rather than unicast to the server identifier (RFC2131-4.1-10, a recorded MUST gap) | `renewV4` builds the client with no `WithServerAddr`/`WithUnicast` | Q3 |
| 9 | Option 61 (client-id) and 12 (hostname) are sent at acquisition and omitted on every renewal (RFC2131-2-2, a recorded MUST gap) | `renewV4` passes only `clearReservedFlags` to `Renew` | Q4 |
| 10 | The server's T1 (option 58) and T2 (option 59) are ignored; T1/T2 are always 0.5 and 0.875 of the lease | `v4RenewalTimes` | Q2 |
| 11 | A renewal ACK naming a different router leaves the old lease's default route installed | `handleV4Lease` adds the new route, nothing removes the old one | (inside defect 3's fix) |
| 12 | A lease whose address could not be installed (claim refused, netlink replace failed) is still timed and renewed as if held | `handleV4Lease` returns nothing; `runV4` continues to T1 | (inside defect 1's fix) |

The goal: a client that keeps renewing at T1 and rebinding at T2 for as long as
any server answers, retransmits on the RFC schedule, restarts on a DHCPNAK,
removes the address only when the lease really expires, holds the default route
it was given, and never invents a lease time.

Already landed, not redone here: 3c019f921f (infinite lease held, leased-address
registry, T1/T2 overflow fix), 0d6d873f2b (reconcile spares registered leases).

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` (the DHCP rows of the feature table, and the "config owns its addresses" bullet) - the page every DHCPv4 worker file declares (`// Design:`)
  → Constraint: the row "DHCPv4 address lifetime equals the lease, renewed with it" is false after the first renewal today; the page edit lands with Phase 2, not at closure
  → Constraint: the reconcile keeps a DHCPv4 lease only while its client registers it (`iface.RegisterLeasedAddresses`); every new removal path (NAK, expiry, refused install) MUST remove the address BEFORE unregistering, exactly as `removeV4Addr` does now, so a reconcile never sees an unregistered held address
- [ ] `docs/architecture/testing/interop.md` (suites table, Scenario Structure, Optional sidecars, Prove a scenario discriminates)
  → Decision: no DHCP interop suite exists; the `test/interop/scenarios/` suite already hosts a non-BGP peer selected by a file in the scenario directory (`keepalived.conf` starts keepalived: `internal/le/interoplab/bgp/prepare.go`, image declared in `run.go` as `peerKeepalived`), so a `dnsmasq.conf` sidecar follows that pattern (Q5)
  → Constraint: assertions live in typed Go checkers under `internal/le/interoplab/bgp/`; a bespoke checker splits lab I/O (`check_*.go`) from a pure predicate (`*_predicate.go`) and MUST NOT call `checkScenario`
  → Constraint: scenario directories are named, no numeric prefix; discrimination is proven by reverting the fix, rebuilding the ze image, observing red
- [ ] `plan/immediate/spec-dhcpv6-client-lease-maintenance.md` (status ready) - parallel spec on the same `DHCPClient`
  → Decision: owner answered its Q7: THIS spec adds the injected `clock.Clock` to `DHCPClient`; the v6 spec reuses it. No shared v4/v6 state machine
  → Constraint: its Files list expects `register.go` to pass `clock.RealClock{}` and `sleepOrStop` to read the clock; keep that shape so the v6 work lands without rework
- [ ] `internal/core/clock/clock.go` - `Clock` interface (`Now`, `Sleep`, `After`, `AfterFunc`, `NewTimer`, `NewTicker`), `RealClock{}`
  → Constraint: `internal/test/sim.FakeClock` fires only `AfterFunc` callbacks on `Add`/`Set`; its `NewTimer` and `After` channels are inert. `sleepOrStop` MUST therefore wait through `AfterFunc` (closing a channel), never `NewTimer`, or no fake-clock test can wake it
  → Decision: add `FakeClock.AwaitTimers(n, within)` beside the existing `AwaitTickers`, so a test advances the clock only after the worker has parked; without it a test races the worker's timer registration

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2131.md` - the enrolled summary (both roles); RFC scope is frozen to enrolled documents, and 2131 is enrolled
  → Constraint: rows this spec flips from gap to implemented, each needing a tagged test and a discrimination record: RFC2131-4.4.5-1 (expiry teardown on time), RFC2131-4.1-10 (unicast to server identifier, Q3), RFC2131-2-2 (same client-id in all messages, Q4); rows newly ze-authored and owed a tagged test: RFC2131-4.3.2-5 (REBIND broadcast), RFC2131-4.4.5-2 and -3 (T1 < T2 < expiry, Q2), RFC2131-4.4.5-8 (SHOULD: retransmit schedule)
  → Constraint: row RFC2131-4.4.5-5 says a renewal with a new yiaddr leaves the old address installed; HEAD's `handleV4Lease` now removes the old address when the CIDR differs, so the row's gap text is stale and is re-judged against the producer in Phase 5
  → Constraint: the `## Meta` "Support remaining" cell names the client gaps (2-2, 4.1-10, 4.4.5-1, 4.4.5-5); it changes, and `docs/features/rfc-status.md` is generated from it (`internal/le/rfc/render.go`)
- [ ] `rfc/full/rfc2131.txt` - read 2026-10-18, quoted below
  → Constraint: Section 4.4.5: "In both RENEWING and REBINDING states, if the client receives no response to its DHCPREQUEST message, the client SHOULD wait one-half of the remaining time until T2 (in RENEWING state) and one-half of the remaining lease time (in REBINDING state), down to a minimum of 60 seconds, before retransmitting the DHCPREQUEST message."
  → Constraint: Section 4.4.5: "When the client receives a DHCPACK from the server, the client computes the lease expiration time as the sum of the time at which the client sent the DHCPREQUEST message and the duration of the lease in the DHCPACK message." Timers start at the SEND time, not the ACK arrival
  → Constraint: Section 4.4.5: "If no DHCPACK arrives before time T2, the client moves to REBINDING state and sends (via broadcast) a DHCPREQUEST message to extend its lease." and Section 4.3.2 (REBINDING): "This message MUST be broadcast to the 0xffffffff IP broadcast address."
  → Constraint: Section 4.4.5: "T1 MUST be earlier than T2, which, in turn, MUST be earlier than the time at which the client's lease will expire." and "Times T1 and T2 are configurable by the server through options."
  → Constraint: Section 4.4.5: "If the lease expires before the client receives a DHCPACK, the client moves to INIT state, MUST immediately stop any other network processing"
  → Constraint: Section 3.1: "If the client receives a DHCPNAK message, the client restarts the configuration process." Section 3.2: "If the client receives a DHCPNAK message, it cannot reuse its remembered network address."
  → Constraint: Section 4.3.1 Table 3, row "IP address lease time": MUST in DHCPOFFER, MUST in DHCPACK (DHCPREQUEST), MUST NOT in DHCPNAK. A DHCPACK without it is malformed; a lease time of 0 grants nothing
  → Constraint: Section 4.1: "DHCP clients MUST use the IP address provided in the 'server identifier' option for any unicast requests to the DHCP server." Section 4.4.5: "At time T1 the client moves to RENEWING state and sends (via unicast) a DHCPREQUEST message"
  → Constraint: Section 2: "If the client uses a 'client identifier' in one message, it MUST use that same identifier in all subsequent messages"

**Key insights:** (minimal context to resume after compaction)
- The worker is a straight-line script, not a state machine; defects 1, 4, 5, 6 and 12 are each a missing state transition. The fix is an explicit INIT / BOUND / RENEWING / REBINDING loop (RFC 2131 Figure 5) driven by absolute deadlines on an injected clock.
- `nclient4.Renew` hard-pins the matcher to the original server, so REBIND cannot go through it; `Client.SendAndRead` with `NewRenewFromAck` and an `IsMessageType(ACK, NAK)` matcher is the library's own lower layer and does what REBIND needs.
- `nclient4` timeouts use real `time.After`; the worker therefore owns the schedule (absolute deadlines on its clock) and asks the library for exactly one transmission with a listen window equal to the computed wait.
- A unit test reaches a logical name that differs from its kernel device through a fake `iface.Backend` whose `GetInterface` returns a different `OsName` (the resolver builds the binding with `bindingFromInfo(info.OsName, info)`); no resolver export is needed.
- DHCP on the VPP backend is refused at parse (`test/parse/iface-vpp-rejects-dhcp.ci`), so the only route backend the retry meets is netlink, whose `AddRoute` is `RouteReplace` (idempotent).
- The package's integration tests run in the QEMU suite (`internal/le/test/qemu/alltests.go` lists `./internal/plugins/iface/dhcp`), not in `./le test integration iface`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/iface/dhcp/dhcp_v4_linux.go` (434L) - `runV4` loop (one DORA, sleep T1, one renew, sleeps, unconditional expiry), `renewV4` (logical-name socket, library `Renew`, NAK = error), `handleV4Lease` (claim, replace with lifetime, route add warn-only, resolv.conf, publish), `removeV4Addr`, `v4LeaseTime` (1 h default), `v4RenewalTimes`, `holdInfiniteV4`, `clearReservedFlags`, `v4RequestModifiers`
  → Constraint: `clearReservedFlags` MUST stay the last modifier on every message ze sends, including the new REBIND (RFC2131-2-3 tagged tests in `rfc2131_dhcp_flags_integration_linux_test.go`)
  → Constraint: `claimV4Address` before install, `settleV4Address` after, remove before unregister: the ordering contract with the reconcile is preserved on every path
- [ ] `internal/plugins/iface/dhcp/dhcp_linux.go` (225L) - `DHCPClient` struct, `newDHCPClient`, `Start`/`Stop`, `safeRunV4` (defer unregister), `stoppableContext`, `sleepOrStop` (`time.NewTimer`), `publishDHCP`
  → Constraint: `Stop` closes `c.stop`; every wait in the new loop selects on it, and every exchange's context is cancelled by it
- [ ] `internal/plugins/iface/dhcp/register.go` (93L) - `newDHCPClientFromFactory` builds `dHCPConfig` and calls `newDHCPClient`
- [ ] `vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go`, `lease.go` - `New(iface, opts)`, `WithUnicast`, `WithServerAddr`, `WithTimeout`, `WithRetry`, `SendAndRead(ctx, dest, msg, matcher)`, `IsMessageType`, `IsCorrectServer`, `ErrNak`, `ErrNoResponse`, `Renew` (pinned matcher, NAK as `*ErrNak`), defaults 5 s timeout, 3 tries, broadcast server address
  → Constraint: vendored code is not edited; ze composes `NewRenewFromAck` + `SendAndRead` itself
- [ ] `vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4packet.go` - `NewRenewFromAck` (ciaddr = yiaddr, no option 54, no option 50, `WithReply` copies opcode/HW type/xid/chaddr/flags only), `IPAddressLeaseTime(def)`, `IPAddressRenewalTime`, `IPAddressRebindingTime`
- [ ] `internal/component/iface/dispatch.go` - `ResolveDevice`; `AddRoute`/`RemoveRoute`/`ReplaceAddressWithLifetime` resolve the logical name themselves
- [ ] `internal/component/iface/resolve.go` - `Resolve` caches the binding; `OsName` comes from the backend's `InterfaceInfo.OsName`
- [ ] `internal/plugins/iface/netlink/manage_linux.go` - `netlinkBackend.AddRoute` is `netlink.RouteReplace`
- [ ] `internal/plugins/iface/dhcp/dhcp_lease_owner_integration_linux_test.go`, `dhcp_shortframe_integration_linux_test.go` - the veth lab: `sfSetupLab`, `sfReadDHCP`, `sfWrite`, `bindV4Lease`, `waitForV4Addr`; the peer side is a raw `packet.Conn` and holds no IP address
- [ ] `internal/plugins/iface/dhcp/dhcp_payload_linux_test.go` - `TestV4LeaseTime` asserts the one-hour default for an absent option: a test wrong about what it asserts, corrected here (Q1)

**Behavior to preserve:**
- DORA through `nclient4.Request` on `binding.OsName`, with `v4RequestModifiers()`; 5 s back-off on resolve/socket failure, 10 s back-off on a failed DORA
- Address installed with valid and preferred lifetime equal to the lease (`v4AddressLifetime`), infinite lease installed infinite and held until Stop (`holdInfiniteV4`)
- Leased-address registry ordering (claim, install, settle; remove, then unregister) and `safeRunV4`'s deferred unregister
- A renewal granting a different address removes the old one (`handleV4Lease`)
- Event topics `dhcp-acquired`, `dhcp-renewed`, `dhcp-expired` and the `DHCPPayload` JSON shape
- resolv.conf written on acquire/renew unless static name servers; never cleared on expiry
- Reserved flags bits cleared on every message (RFC2131-2-3)

**Behavior to change:** defects 1-12 above, as specified by the AC table.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `interface ethernet <name> unit <n> ipv4 dhcp { enabled true; ... }` → iface component config apply → `iface.SetDHCPClientFactory` hook → `newDHCPClientFromFactory` (`register.go`) → `DHCPClient.Start` → `safeRunV4` → `runV4`
- Wire: DHCPACK / DHCPNAK UDP frames received by the `nclient4` raw socket (DORA, REBIND) or a UDP socket on the leased address (RENEW, Q3)

### Transformation Path
1. INIT: resolve `c.ifaceName` to its kernel device; DORA through the exchanger on that device
2. Validate the ACK: address present, lease time present and non-zero (Q1); derive T1/T2 (options 58/59 under Q2, else defaults) and absolute deadlines from the REQUEST send instant
3. Install: claim, `iface.ReplaceAddressWithLifetime`, settle, route (retry on failure), resolv.conf, publish
4. BOUND: wait until T1 or the next route retry, whichever is first
5. RENEWING: one unicast REQUEST per attempt to the server identifier; ACK → steps 2 and 3 with `dhcp-renewed`; NAK → teardown, INIT; no answer → next attempt on the 4.4.5 schedule, or REBINDING at T2
6. REBINDING: one broadcast REQUEST per attempt, any server's ACK or NAK accepted; ACK → re-bound to that server; NAK → teardown, INIT; no answer → schedule, or expiry
7. Expiry: teardown (address, route, unregister), `dhcp-expired`, INIT at once

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface component → dhcp plugin | `iface.SetDHCPClientFactory` callback, in-process (the plugin refuses external mode) | No |
| dhcp plugin → iface backend | `iface.ReplaceAddressWithLifetime`, `RemoveAddress`, `AddRoute`, `RemoveRoute`, `ResolveDevice`, `RegisterLeasedAddresses` | No |
| dhcp plugin → event bus | `ze.EventBus.Emit` on the iface namespace, JSON `DHCPPayload` | No |
| ze ↔ DHCP server | RFC 2131 UDP 67/68 via vendored `nclient4` | No |

### Integration Points
- `nclient4.Client.SendAndRead` - used directly for RENEW and REBIND so ze owns destination, matcher and attempt count
- `internal/core/clock.Clock` - new `DHCPClient` field, the only time source of the v4 loop
- `iface.Backend` (test) - a fake backend registered under a test-only name proves device binding and route retry without a kernel

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | All address/route effects stay behind `iface.*` dispatch; no direct netlink from the plugin |
| No unintended coupling (components stay isolated) | Yes | Changes confined to `internal/plugins/iface/dhcp/` plus a test helper in `internal/test/sim` |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `nclient4` message builders and `SendAndRead`; reuses `clock.Clock` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Control-plane worker, one message per tens of seconds |
| Registration over hardcoding, outbound | Yes | No new command, family or handler; no core switch learns anything |
| Registration over hardcoding, inbound | Yes | Searched `dhcpTopicToEventType` (event topics, unchanged set) and `iface.SetDHCPClientFactory` (signature unchanged); no list learns a new name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A UDP socket bound to leased-address:68 can send the unicast RENEW and receive the unicast ACK while no other socket owns that address and port | `nclient4.WithUnicast` uses `net.ListenUDP("udp4", srcAddr)` | RENEW cannot be unicast; Q3 goes back to the owner | `TestDHCPv4RenewIsUnicastToServer` in the veth lab | unvalidated |
| A-2 | The veth lab can answer unicast frames if the peer side carries `sfServerIP` so the kernel resolves ARP for it | `sfSetupLab` assigns no address to the peer side today | the unicast test cannot run in the lab; it moves to the interop scenario only | lab extension in Phase 5, test passes | unvalidated |
| A-3 | dnsmasq NAKs a RENEW for an address that a SIGHUP-reloaded `dhcp-hostsfile` now reserves elsewhere, when `dhcp-authoritative` is set | dnsmasq documentation (read at implementation) | the NAK scenario needs a different trigger (restart with a new range) | `dhcpv4-renew-nak-dnsmasq` red/green walk | unvalidated |
| A-4 | A ze interop container can run `dhcp { enabled true }` on `eth0` beside the docker-assigned static address | `test/parse/dhcp-static-coexist.ci` accepts the config | the scenario needs a second docker network for the DHCP segment | scenario boots and acquires | unvalidated |
| A-5 | dnsmasq sends options 58/59 from `dhcp-option=option:T1,...` and `option:T2,...`, letting the interop run at T1 20 s under dnsmasq's 120 s minimum lease | dnsmasq option names | interop runtime grows to minutes (T1 60 s) | scenario timing | unvalidated |
| A-6 | DHCP is never configured on the VPP backend, so a route retry never meets the VPP backend's permanent `AddRoute` refusal | `test/parse/iface-vpp-rejects-dhcp.ci` | the retry would warn forever; add a permanent-error stop | read the parse refusal at implementation | unvalidated |
| A-7 | `sim.FakeClock.AfterFunc` callbacks run synchronously inside `Add`, so a callback that closes a channel wakes the worker deterministically | `internal/test/sim/sim.go` type doc comment | fake-clock tests are flaky | `TestSleepOrStopOnInjectedClock` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Teardown on NAK or expiry runs a second time (NAK then Stop) and removes an address a new lease just installed | address missing right after a re-acquire in tests | teardown clears `c.v4Address` and the held router; removal is keyed on what the worker holds, never on an old ACK |
| R-2 | A late ACK to a RENEW arrives after the attempt's listen window and is dropped | renew counted as no answer with a server that answered | listen window equals the whole wait before the next attempt, so only an ACK later than the next transmission is lost; that transmission draws the same answer |
| R-3 | Timers computed from ACK arrival instead of send instant make expiry late by the round trip | AC-14 red | the exchanger reports the send instant from the injected clock; deadlines derive from it |
| R-4 | Route retry floods the log on a persistent netlink failure | repeated Warn lines | Warn on the first failure only, Debug on each retry, Info once on recovery; back-off 5 s doubling to a 60 s cap |
| R-5 | Unicast RENEW fails on hosts where the leased address is not yet usable (tentative, policy routing) | every RENEW unanswered, every lease rebinding | RENEW failures fall to REBIND broadcast at T2 by design; the interop scenario asserts RENEW is answered |
| R-6 | Refusing an ACK without option 51 strands a host behind a non-conformant server | operator report, the refusal Warn | the Warn names the server identifier and the missing option, so the fix (server config) is stated; Q1 records the owner's choice |
| R-7 | The v6 spec lands first and adds its own clock field | merge conflict in `dhcp_linux.go` | owner answer Q7 (v6 spec) assigns the field to this spec; whichever lands second rebases onto the one field |
| R-8 | The new loop leaks goroutines (`stoppableContext` spawns one per exchange) | goroutine count grows per renewal in a long test | each context is cancelled on return (existing pattern); `TestDHCPv4LoopHoldsLeaseFiveLeasePeriods` asserts the goroutine count is flat |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | DHCP-addressed uplinks lose address and default route (the defect today), or keep an address the server refused |
| How is it reverted? | Single commit revert; no config or wire-format migration |
| Who else touches this path? | `spec-dhcpv6-client-lease-maintenance` (ready; reuses the clock field this spec adds), owned-address reconcile (`internal/component/iface`), DHCP server (`internal/plugins/dhcpserver`, same RFC summary file) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `newDHCPClient` + `Start` (what the config factory hook calls) | → | `runV4` lease loop on the injected clock | `TestDHCPv4LoopHoldsLeaseFiveLeasePeriods` |
| `iface.ResolveDevice` on a selector-bound logical name | → | exchanger opened on the kernel device for DORA, RENEW and REBIND | `TestDHCPv4ExchangesBindTheResolvedDevice` |
| Real kernel veth, ze client started through `newDHCPClient` + `Start` | → | RENEW repeated across several T1s, address never removed | `TestDHCPv4RenewsRepeatedlyOnVeth` |
| Config `dhcp { enabled true }` in a ze container against dnsmasq | → | RENEW, NAK restart | `dhcpv4-renew-nak-dnsmasq` interop checker |
| Same, two dnsmasq servers | → | REBIND accepted from a second server | `dhcpv4-rebind-two-servers-dnsmasq` interop checker |

## Acceptance Criteria

Times below are relative to the instant the client SENT the DHCPREQUEST that the ACK answers. "Teardown" means: leased address removed, the default route this lease installed removed, the lease unregistered, in that order.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Lease 120 s, no options 58/59, server ACKs every RENEW; clock advanced through 5 lease periods | RENEW sent at each T1 (60 s after each REQUEST send), no REBIND, no DISCOVER after the first, no address or route removal, no `dhcp-expired`; one `dhcp-renewed` per ACK; each ACK reinstalls the address with lifetime 120 s |
| AC-2 | Logical name `uplink`, backend reports kernel device `eth7` | DORA, every RENEW and every REBIND are opened on `eth7`; no exchange is opened on `uplink` |
| AC-3 | Route install fails at acquisition (first 2 `AddRoute` calls error), server gives router 192.0.2.1 | `dhcp-acquired` published once; `AddRoute` retried 5 s and 15 s after the first failure (back-off 5 s doubling, cap 60 s); third call succeeds; one Warn (first failure), one Info (installed); no retry after success |
| AC-4 | Route keeps failing | retries continue at the 60 s cap while BOUND, RENEWING or REBINDING; they stop at teardown or Stop |
| AC-5 | Renewal ACK names router 192.0.2.2 where the lease held 192.0.2.1 | route via 192.0.2.1 removed, route via 192.0.2.2 installed |
| AC-6 | Renewal ACK names no router where the lease held one | the old default route is removed |
| AC-7 | DHCPNAK answers a RENEW | teardown, `dhcp-expired` published, DISCOVER sent with no back-off wait; the NAK's server identifier and message (option 56, when present) logged at Info |
| AC-8 | DHCPNAK answers a REBIND (from any server) | same as AC-7 |
| AC-9 | Lease 1000 s, no server answers from T1 | RENEW transmissions at 500, 687.5, 781.25, 841.25 s; REBIND transmissions at 875, 937.5, 997.5 s; teardown and `dhcp-expired` at 1000 s; DISCOVER immediately after; no transmission in between (RFC 2131 Section 4.4.5: wait max(remaining/2, 60 s); a wait that reaches the phase end moves to that phase end) |
| AC-10 | REBIND message | destination 255.255.255.255:67, ciaddr = leased address, no option 54 (server identifier), no option 50 (requested IP), reserved flags bits zero |
| AC-11 | During REBINDING an ACK arrives from server B (server identifier differs from the leasing server A) | ACK accepted: lease re-bound, `dhcp-renewed` published; the next RENEW is addressed to B |
| AC-12 | RENEW message (Q3) | destination IP = leasing server identifier, port 67, source = leased address port 68, ciaddr = leased address, no option 54, no option 50 |
| AC-13 | During RENEWING an ACK arrives from a server other than the leasing one | ignored (RENEWING contacts "the server that originally issued the client's network address"); the attempt continues |
| AC-14 | ACK received 3 s after the REQUEST was sent, lease 100 s | T1 at 50 s, T2 at 87.5 s, expiry at 100 s, all from the send instant |
| AC-15 | DORA ACK with no option 51 (Q1) | no address installed, no `dhcp-acquired`; one Warn naming the interface, the server identifier, and that the ACK carries no IP address lease time (option 51), which RFC 2131 Table 3 requires, so the server's configuration must supply one; DORA retried after the 10 s back-off; repeat refusals from the same server log at Debug until a lease is accepted |
| AC-16 | DORA ACK with option 51 = 0 (Q1) | as AC-15, the Warn says the lease time is 0 |
| AC-17 | RENEW or REBIND answered by an ACK with no option 51 or with 0 (Q1) | treated as no answer: the held lease, address and route are kept; the retransmission schedule of AC-9 continues; one Warn per lease |
| AC-18 | Options 58 = 30 s and 59 = 70 s in a 100 s lease (Q2) | RENEW at 30 s, REBIND at 70 s |
| AC-19 | Options 58/59 violating T1 < T2 < lease (58 = 80, 59 = 40, lease 100), or either option alone breaking the order against the default for the other (Q2) | both fall back to the defaults 50 s and 87.5 s; one Debug line names the rejected values |
| AC-20 | Config sets `client-id` and `hostname` (Q4) | every RENEW and REBIND carries the same option 61 and option 12 bytes as the DORA REQUEST |
| AC-21 | Address install fails at acquisition (`ReplaceAddressWithLifetime` errors) | no BOUND state: no `dhcp-acquired`, no RENEW timer; DORA retried after the 10 s back-off |
| AC-22 | Stop in INIT, BOUND, RENEWING or REBINDING | worker returns; teardown when a lease is held; nothing published after Stop |
| AC-23 | Renewal ACK with lease time 0xffffffff | address reinstalled infinite; worker holds until Stop (unchanged) |
| AC-24 | Interop: ze against dnsmasq, T1 20 s, lease 120 s | ze holds one address across at least 3 RENEW/ACK exchanges with no DISCOVER; after dnsmasq is made to NAK, ze drops the address, sends DISCOVER, and acquires the newly reserved address |
| AC-25 | Interop: two dnsmasq servers, the leasing one stopped after bind | ze's RENEW goes unanswered, its REBIND is answered by the second server, the address is held throughout, no DISCOVER |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures DHCP on an uplink and leaves it running for hours | config → factory → `runV4` → RENEW every T1 → kernel address refreshed | `dhcpv4-renew-nak-dnsmasq` (renew phase), `TestDHCPv4RenewsRepeatedlyOnVeth` |
| 2 | Moves the server's reservation for the host | RENEW → NAK → teardown → DORA → new address | `dhcpv4-renew-nak-dnsmasq` (NAK phase), `TestDHCPv4NakOnRenewRestartsOnVeth` |
| 3 | Takes the primary DHCP server down for maintenance | RENEW unanswered → REBIND broadcast → secondary ACK | `dhcpv4-rebind-two-servers-dnsmasq`, `TestDHCPv4RebindAcceptsAnyServerOnVeth` |
| 4 | Binds the uplink by MAC (`mac/match`) or `os-name` | resolve → exchanges on the kernel device | `TestDHCPv4ExchangesBindTheResolvedDevice` |

## 🧪 TDD Test Plan

Every unit and integration test below MUST be red at HEAD for the reason its AC names (or, for a new seam, fail to compile against HEAD and then fail for that reason once the seam exists); record the red output in the closure sections.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSleepOrStopOnInjectedClock` | `internal/plugins/iface/dhcp/dhcp_test.go` | `sleepOrStop` wakes on `FakeClock.Add`, returns false on Stop (A-7) | |
| `TestV4RetransmitSchedule` | `internal/plugins/iface/dhcp/dhcp_v4_schedule_linux_test.go` | AC-9 transmission instants from the pure schedule function (both phases, 60 s floor, phase-end cap) | |
| `TestV4LeaseTimersFromSendTime` | same | AC-14 | |
| `TestV4ServerTimersHonoredWhenOrdered` | same | AC-18, AC-19 (all orderings, each option alone) | |
| `TestV4AckWithoutLeaseTimeRefused` | `internal/plugins/iface/dhcp/dhcp_payload_linux_test.go` (replaces the 1 h assertion in `TestV4LeaseTime`) | AC-15, AC-16: the lease-time reader answers "absent" and "zero" distinctly, never 1 h | |
| `TestDHCPv4LoopHoldsLeaseFiveLeasePeriods` | `internal/plugins/iface/dhcp/dhcp_v4_loop_linux_test.go` | AC-1, AC-23, R-8 (scripted exchanger, fake iface backend, `FakeClock`) | |
| `TestDHCPv4ExchangesBindTheResolvedDevice` | same | AC-2 | |
| `TestDHCPv4RouteInstallRetried` | same | AC-3, AC-4 | |
| `TestDHCPv4RouteFollowsRenewedRouter` | same | AC-5, AC-6 | |
| `TestDHCPv4NakRestartsConfiguration` | same | AC-7, AC-8 | |
| `TestDHCPv4SilentServersExpireOnSchedule` | same | AC-9 end to end through the loop, teardown order | |
| `TestDHCPv4RebindAcceptsAnyServer` | same | AC-11, AC-13 (matcher passed to the exchanger) | |
| `TestDHCPv4RenewalAckWithoutLeaseTimeIsNoAnswer` | same | AC-17 | |
| `TestDHCPv4RefusedAckNotInstalled` | same | AC-15, AC-16 through the loop, log once | |
| `TestDHCPv4InstallFailureIsNotBound` | same | AC-21 | |
| `TestDHCPv4StopInEveryState` | same | AC-22 | |
| `TestV4RenewalCarriesClientIdentity` | `internal/plugins/iface/dhcp/dhcp_test.go` | AC-20: renewal modifiers equal acquisition modifiers | |
| `TestFakeClockAwaitTimers` | `internal/test/sim/sim_test.go` | the new `AwaitTimers` helper counts pending AfterFunc timers and wakes on registration | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Option 51 lease time (s) | 1 to 0xfffffffe finite, 0xffffffff infinite | 1 and 0xfffffffe | 0 (refused, AC-16) | N/A (0xffffffff is the infinite value) |
| Option 58 T1 (s) | 1 to T2 - 1 | T2 - 1 | 0 (defaults) | ≥ T2 (defaults) |
| Option 59 T2 (s) | T1 + 1 to lease - 1 | lease - 1 | ≤ T1 (defaults) | ≥ lease (defaults) |
| Retransmit wait | 60 s floor, capped at phase end | remaining = 120 s gives 60 s | remaining < 60 s goes to phase end | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A (`.ci`) | - | A `.ci` cannot host a DHCP server on a link. The user path (config to kernel address over a lease lifetime) is proven by the veth integration tests, which run the real client against the real kernel, and by the interop scenarios, which start ze from config | |
| `TestDHCPv4RenewsRepeatedlyOnVeth` | `internal/plugins/iface/dhcp/rfc2131_dhcp_lease_maintenance_integration_linux_test.go` | lease 6 s, options 58 = 2, 59 = 4; lab ACKs 4 RENEWs; address present throughout, no DISCOVER (AC-1 on a real kernel). Tag RFC2131-4.4.5-1 positive | |
| `TestDHCPv4NakOnRenewRestartsOnVeth` | same | lab NAKs the RENEW; address removed, DISCOVER seen (AC-7) | |
| `TestDHCPv4RebindAcceptsAnyServerOnVeth` | same | lab ignores RENEW, answers the broadcast REBIND as server B; address kept; frame checks of AC-10. Tags RFC2131-4.3.2-5, RFC2131-4.4.5-6 | |
| `TestDHCPv4RenewIsUnicastToServer` | same | AC-12 frame checks (A-2 lab extension). Tag RFC2131-4.1-10 | |
| `TestDHCPv4RenewCarriesClientID` | same | AC-20 frame check. Tag RFC2131-2-2 | |
| `TestDHCPv4ExpiresOnTimeOnVeth` | same | lab silent after bind, lease 6 s: address and route gone within 1 s of expiry, DISCOVER follows. Tag RFC2131-4.4.5-1 negative | |

These run in the QEMU suite that already carries the package (`internal/le/test/qemu/alltests.go`).

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `dhcpv4-renew-nak-dnsmasq` | `test/interop/scenarios/dhcpv4-renew-nak-dnsmasq/` (`ze.conf`, `dnsmasq.conf`) | dnsmasq | AC-24: three or more RENEW/ACK pairs in dnsmasq's log for ze's MAC with no DISCOVER; after the checker moves ze's reservation and SIGHUPs dnsmasq (A-3), dnsmasq logs DHCPNAK, then a DHCPDISCOVER from ze and an ACK of the new address, and ze shows only the new address | |
| `dhcpv4-rebind-two-servers-dnsmasq` | `test/interop/scenarios/dhcpv4-rebind-two-servers-dnsmasq/` (`ze.conf`, `dnsmasq.conf`, `dnsmasq-b.conf`) | dnsmasq (two instances) | AC-25: the checker stops the server whose log holds ze's ACK; the other logs a DHCPREQUEST with ciaddr from ze and its DHCPACK; ze holds the address throughout; no DHCPDISCOVER | |

Discrimination (`ai/rules/interop-and-goal-validation.md`): for each scenario, revert the loop fix, rebuild the ze image, record red (renew scenario: DISCOVER appears at the first expiry; rebind scenario: the second server's ACK is ignored and the address is dropped), restore, record green.

## Files to Modify
- `internal/plugins/iface/dhcp/dhcp_v4_linux.go` - explicit INIT/BOUND/RENEWING/REBINDING loop replacing `runV4`'s straight line and `renewV4`; lease record with send instant and absolute deadlines; pure schedule and timer functions; lease-time reader that distinguishes absent and zero; install result from `handleV4Lease`; route-pending retry; teardown keyed on held address and held router; RFC 2131 quotes above each transition
- `internal/plugins/iface/dhcp/dhcp_linux.go` - `clock clock.Clock` and exchanger fields; `sleepOrStop` on `clock.AfterFunc`; a deadline wait; `newDHCPClient` takes the clock
- `internal/plugins/iface/dhcp/register.go` - pass `clock.RealClock{}` and the production exchanger
- `internal/plugins/iface/dhcp/dhcp_payload_linux_test.go` - `TestV4LeaseTime` 1 h assertion replaced (AC-15/16); `newTestClient` passes a clock
- `internal/plugins/iface/dhcp/dhcp_test.go` - `TestNewDHCPClientValidation` and `TestSleepOrStopWithClosedChannel` follow the constructor
- `internal/plugins/iface/dhcp/dhcp_shortframe_integration_linux_test.go` - the lab gives the peer side `sfServerIP` (A-2)
- `internal/test/sim/sim.go` - `FakeClock.AwaitTimers`
- `internal/le/interoplab/bgp/run.go`, `prepare.go`, `names.go`, `checkers.go` - dnsmasq peer image, `dnsmasq.conf`/`dnsmasq-b.conf` sidecar selection and mounts, container names and addresses, the two scenarios' checker registrations
- `docs/features/interfaces.md` - DHCP rows: renew at every T1, rebind to any server at T2, RFC retransmission schedule, NAK restarts, ACK without lease time refused, route retried, server T1/T2 honoured, renewal keeps client-id; correct the "renewed with it" row
- `docs/architecture/testing/interop.md` - dnsmasq sidecar row, scenario inventory rows
- `docs/features/interoperability-testing.md` - dnsmasq in the tested-peer list
- `rfc/short/rfc2131.md` - rows 2-2, 4.1-10, 4.3.2-5, 4.4.5-1, 4.4.5-2, 4.4.5-3, 4.4.5-5 (stale), 4.4.5-6, 4.4.5-7, 4.4.5-8 re-judged with tests; `## Meta` Support remaining
- `docs/features/rfc-status.md` - regenerated from the Meta row
- `rfc/discrimination/rfc2131.json` - records for each new tag (`./le rfc discriminate-record`)

## Files to Create
- `internal/plugins/iface/dhcp/dhcp_v4_exchange_linux.go` - the `v4Exchanger` seam (acquire, renew, rebind; each takes the kernel device name, the held lease, the listen window and the modifiers, and returns the reply, the send instant and a typed outcome: ACK, NAK, no answer) and its `nclient4` implementation
- `internal/plugins/iface/dhcp/dhcp_v4_schedule_linux_test.go`, `dhcp_v4_loop_linux_test.go` - unit tests above (scripted exchanger, fake `iface.Backend` registered under a test-only name)
- `internal/plugins/iface/dhcp/rfc2131_dhcp_lease_maintenance_integration_linux_test.go` - veth tests above
- `internal/le/interoplab/bgp/check_dhcp.go`, `check_dhcp_predicate.go` - bespoke checkers (lab I/O vs pure predicate over dnsmasq log text)
- `Dockerfile.dnsmasq` in the producer directory that holds `Dockerfile.keepalived`
- `test/interop/scenarios/dhcpv4-renew-nak-dnsmasq/ze.conf`, `dnsmasq.conf`
- `test/interop/scenarios/dhcpv4-rebind-two-servers-dnsmasq/ze.conf`, `dnsmasq.conf`, `dnsmasq-b.conf`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new leaf: timers and retransmission are protocol-defined |
| YANG validation constraints | N-A | No leaf added or changed |
| YANG custom validators | N-A | No leaf added or changed |
| CLI commands/flags | No | No command surface |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No leaf |
| Functional test for new RPC/API | N-A | No RPC or API |
| Pipe completeness | N-A | No command output |
| Env var registration | No | No env var |
| Doctor check for runtime dependencies | No | The unicast RENEW opens UDP 68 on the leased address; the client already depends on UDP 68 (raw socket) and has no doctor check today. Not a new dependency class |
| Prometheus counters/metrics | No | No metric added |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Defect fix; behaviour rows land in `docs/features/interfaces.md` (row 6) |
| 2 | Config syntax changed? | No | No leaf |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | Event topics and payload unchanged |
| 5 | Plugin added/changed? | No | `iface-dhcp` registration unchanged |
| 6 | Has a user guide page? | Yes | `docs/features/interfaces.md` DHCP rows and their source anchors |
| 7 | Wire format changed? | No | DHCP messages stay library-built; destinations change, documented in row 6 and the RFC summary |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2131.md` rows and Meta, `docs/features/rfc-status.md` (generated) |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (dnsmasq sidecar, inventory); `internal/test/sim` doc comment for `AwaitTimers` |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` checked by grep at implementation; edited if it lists DHCP client renewal |
| 12 | Internal architecture changed? | No | No subsystem doc describes the v4 loop beyond `docs/features/interfaces.md` |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived by `./le spec citation anchors spec plan/immediate/spec-dhcpv4-client-drops-lease-every-period.md`: `docs/features/interfaces.md` (declared by `dhcp_v4_linux.go` and `dhcp_linux.go`), `docs/architecture/testing/interop.md` (anchors on interoplab files); each page is updated in the phase that changes its file. `docs/architecture/chaos-web-dashboard.md` (declared by `internal/test/sim/sim.go`) is unaffected: it does not describe `FakeClock`'s methods (grep for `FakeClock` finds nothing), and `AwaitTimers` changes no simulation behaviour. Advisory mentions: `docs/architecture/plugin/plugin-relationships.md` and `docs/guide/plugins.md` (via `register.go`) are unaffected because the plugin's registration and factory signature do not change; `docs/features/interoperability-testing.md` (via `run.go`) gains dnsmasq in its peer list in Phase 6 |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No example changes; config is untouched |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- clock and exchanger seams
   - Tests: `TestFakeClockAwaitTimers`, `TestSleepOrStopOnInjectedClock`, `TestDHCPv4LoopHoldsLeaseFiveLeasePeriods` (red: renews once, then expires), `TestDHCPv4ExchangesBindTheResolvedDevice` (red: renewal opened on `uplink`)
   - Files: `internal/test/sim/sim.go`, `dhcp_linux.go`, `register.go`, `dhcp_v4_exchange_linux.go`, test helpers
   - Verify: the loop test reaches `runV4` through the injected exchanger and clock and fails for the defect-1 reason
2. **Phase: Lease state machine** -- INIT/BOUND/RENEWING/REBINDING with send-instant deadlines, schedule, NAK, expiry, any-server REBIND, install result
   - Tests: `TestV4RetransmitSchedule`, `TestV4LeaseTimersFromSendTime`, `TestDHCPv4NakRestartsConfiguration`, `TestDHCPv4SilentServersExpireOnSchedule`, `TestDHCPv4RebindAcceptsAnyServer`, `TestDHCPv4InstallFailureIsNotBound`, `TestDHCPv4StopInEveryState`
   - Files: `dhcp_v4_linux.go`, `dhcp_v4_exchange_linux.go`, `docs/features/interfaces.md` (rows edited in this phase)
3. **Phase: Lease-time refusal, server timers, client identity, unicast RENEW** (per Q1-Q4 answers)
   - Tests: `TestV4AckWithoutLeaseTimeRefused`, `TestDHCPv4RefusedAckNotInstalled`, `TestDHCPv4RenewalAckWithoutLeaseTimeIsNoAnswer`, `TestV4ServerTimersHonoredWhenOrdered`, `TestV4RenewalCarriesClientIdentity`
   - Files: `dhcp_v4_linux.go`, `dhcp_v4_exchange_linux.go`, `dhcp_payload_linux_test.go`, `docs/features/interfaces.md`
4. **Phase: Default route** -- held router, retry with back-off, router change on renewal
   - Tests: `TestDHCPv4RouteInstallRetried`, `TestDHCPv4RouteFollowsRenewedRouter`
   - Files: `dhcp_v4_linux.go`, `docs/features/interfaces.md`
5. **Phase: Kernel proof and RFC rows** -- veth integration tests (QEMU suite), lab peer address, RFC tags, discrimination records, summary rows and Meta, regenerate rfc-status
   - Tests: the six veth tests
   - Files: integration test files, `rfc/short/rfc2131.md`, `rfc/discrimination/rfc2131.json`, `docs/features/rfc-status.md`
6. **Phase: Interop** -- dnsmasq sidecar and two scenarios, red/green walk recorded
   - Files: interoplab files, scenario directories, `docs/architecture/testing/interop.md`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Deadlines derive from the send instant; teardown removes before unregistering; the NAK path never waits the 10 s back-off; a refused ACK never reaches `ReplaceAddressWithLifetime` |
| Correctness | No exchange opens on `c.ifaceName`; every one takes `ResolveDevice` output fetched for that exchange |
| Naming | Log lines say what is wrong, why, and the fix (`ai/rules/user-facing-errors.md`): the refusal names the server and the missing option 51 and says the server must send it |
| Data flow | All time reads go through `c.clock`; no `time.Now`/`time.NewTimer` left in the v4 path |
| Rule: rfc-compliance | Each transition carries the RFC 2131 quote above the statement; each caller names the section |
| Rule: no-layering | The old straight-line `runV4` and `renewV4` are deleted, not kept beside the new loop |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Loop holds a lease across renewals | `TestDHCPv4LoopHoldsLeaseFiveLeasePeriods` passes; red output at HEAD recorded |
| No logical-name socket | grep for `nclient4.New(c.ifaceName` returns nothing |
| No invented lease time | grep for `IPAddressLeaseTime(time.Hour` returns nothing |
| Interop green and discriminating | both scenarios pass; red walk recorded |
| RFC rows re-judged | `./le rfc check` passes |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | ACK with no yiaddr, no lease time, lease 0, absurd 58/59: each refused or defaulted per AC, none reaches the kernel |
| Resource exhaustion | A NAK restarts DORA at once, but a NAKed or failed DORA keeps the 10 s back-off, so a rogue server NAKing every REQUEST costs at most one DORA per 10 s |
| Spoofing | REBIND accepts any server by RFC design; RENEWING stays pinned to the leasing server (AC-13) |
| Error leakage | Logs carry server address and option names only |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- `nclient4.Renew` is unusable for REBIND by construction (pinned matcher); composing `NewRenewFromAck` + `SendAndRead` keeps the vendored builders and moves only the choices the RFC assigns the client (destination, accepted servers, attempt timing) into ze.
- Timing owned by ze on an injected clock, with the library asked for one transmission per attempt, is what makes the RFC schedule testable at all.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D-1 Explicit state loop with absolute deadlines (RFC 2131 Figure 5) | Wrap the straight-line loop in a `for` around renew/rebind | The straight line has no place for NAK, retransmission or a re-bound server; every patch adds a flag. A loop over states holds each transition in one place |
| D-2 Ze owns retransmission: one library transmission per attempt, listen window = wait to the next attempt | Library `WithRetry`/`WithTimeout` doubling | The library's doubling schedule is not the 4.4.5 schedule and runs on real time |
| D-3 REBIND via `SendAndRead` with `IsMessageType(ACK, NAK)` to broadcast | Fork or edit the vendored `Renew` | Vendored code stays untouched; the matcher and destination are the only differences |
| D-4 Exchanger interface (acquire, renew, rebind) as the test seam, production over `nclient4` | A fake DHCP server over a socket pair for unit tests | Socket fakes still run on real time; the seam is one small interface, and the veth tests cover the real library path |
| D-5 `clock.Clock` field passed by `newDHCPClient`, `RealClock{}` from `register.go`; `sleepOrStop` on `AfterFunc` | Package-level clock variable | Matches the v6 spec's plan (owner Q7 there); `AfterFunc` is the only `FakeClock` path that fires |
| D-6 NAK in RENEWING/REBINDING: teardown, `dhcp-expired`, DORA at once | Back off 10 s first | RFC 2131 Section 3.1 "restarts the configuration process"; the 10 s wait is only asked for after DECLINE |
| D-7 Route retry inside the worker, back-off 5 s doubling to 60 s, while a lease is held | Retry only at the next lease event (today), or a separate goroutine | The worker already wakes on deadlines; one more deadline costs nothing and needs no lock |
| D-8 ACK without option 51 or with 0: refused (DORA) / no answer (renewal) | A named, logged default (1 h) | RFC 2131 Table 3 makes option 51 a MUST in that ACK; a default is a lifetime no server granted (Q1) |
| D-9 dnsmasq sidecar in the existing `test/interop/scenarios/` suite | Kea; a new `test/interop-dhcp/` suite | dnsmasq is small, its authoritative mode NAKs, and options 58/59 are configurable; the keepalived sidecar is the precedent for a non-BGP peer in that suite (Q5) |

## Owner Questions

| ID | Question | Recommendation | Answer |
|----|----------|----------------|--------|
| Q1 | An ACK with no IP address lease time (option 51) or a lease time of 0: refuse it, or install it under a named, logged default? | Refuse. RFC 2131 Section 4.3.1 Table 3 makes the lease time a MUST in a DHCPACK to a DHCPREQUEST, and 0 grants nothing. In DORA the ACK is not installed and DORA retries after the 10 s back-off; in RENEWING/REBINDING it counts as no answer, so the held lease keeps its own deadlines. Log once with the server identifier and what is missing (AC-15 to AC-17); `TestV4LeaseTime`'s 1 h assertion is corrected | 2026-10-18, Thomas: "I approve of the specifications." Refuse, as recommended: DORA does not install and retries after the 10 s back-off; RENEW/REBIND treat it as no answer; log once naming the server identifier and what is missing; `TestV4LeaseTime` corrected |
| Q2 | Honour the server's T1 (option 58) and T2 (option 59)? | Yes, when T1 < T2 < lease holds (RFC 2131 Section 4.4.5 MUSTs), else both defaults; same function as the defect-1 fix (AC-18, AC-19) | 2026-10-18, Thomas: "I approve of the specifications." Yes, as recommended: options 58/59 when T1 < T2 < lease, else the defaults for both |
| Q3 | Send RENEW by unicast to the server identifier from the leased address (RFC2131-4.1-10, a recorded MUST gap; Section 4.4.5 "sends (via unicast)")? | Yes. The fix rebuilds the RENEW send path anyway; leaving it broadcast keeps a known MUST gap in the function being rewritten (AC-12, A-1) | 2026-10-18, Thomas: "I approve of the specifications." Yes, as recommended: unicast RENEW to the server identifier |
| Q4 | Carry option 61 (client-id) and 12 (hostname) on every RENEW and REBIND (RFC2131-2-2, a recorded MUST gap)? | Yes: pass the acquisition modifiers to every renewal (AC-20); a server keyed on client-id cannot otherwise match the renewal to the lease | 2026-10-18, Thomas: "I approve of the specifications." Yes, as recommended: options 61 and 12 on every RENEW and REBIND |
| Q5 | Host the DHCP interop scenarios as a dnsmasq sidecar in `test/interop/scenarios/`, or open a dedicated `test/interop-dhcp/` suite? | Sidecar in the existing suite, like keepalived; a new suite costs a runner, catalog and gate for two scenarios | 2026-10-18, Thomas: "I approve of the specifications." Sidecar in `test/interop/scenarios/`, as recommended |

## Known Limitations

- T1/T2 "fuzz" (RFC2131-4.4.5-4, SHOULD) is not added; the row stays as recorded. It is not among the defects the owner asked to fix.
- DORA retransmission keeps the library's non-randomized doubling (RFC2131-4.1-8, recorded MUST gap). Untouched here; the row stays a gap and is not presented as fixed.
- DHCPDECLINE and ARP probing (RFC2131-3.1-7, recorded gap) are outside this spec.

## RFC Documentation (Scope: protocol)

Add `// RFC 2131 Section X.Y: "<quoted requirement>"` above each statement that
performs a transition, timer or message choice (Section 4.4.5 for T1, T2,
retransmission, expiry and send-instant deadlines; Section 3.1 for the NAK restart;
Section 4.3.1 Table 3 for the lease-time refusal; Section 4.1 for the unicast
destination; Section 4.3.2 for the REBIND broadcast; Section 2 for the client
identifier). Callers name the section.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-25 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior (N-A with the reason above; veth integration and interop carry the path)
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + journal rows marked fixed (`silent-fall-through.md` T9108 v4 row, `identity-default-hides-a-mapping.md` renewV4 row, `zero-value-as-valid-answer.md` v4LeaseTime row)
- [ ] **Commit B:** `remove plan/immediate/spec-dhcpv4-client-drops-lease-every-period.md` only, in the same `./le commit create` script
