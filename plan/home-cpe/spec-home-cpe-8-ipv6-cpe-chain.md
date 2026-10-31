# Spec: home-cpe-8-ipv6-cpe-chain

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P3 |
| Release reason | Feature: a dual-stack home CPE on a PPPoE WAN. Owner timing: a missing RFC feature may wait until after the first release. The rows it closes are recorded gaps of enrolled RFCs (RFC 7084 WLL-2, WLL-3, L-2, L-3, L-4; RFC 5072 on the client side). |
| RFC gaps | RFC 5072 (IPv6CP, client side); RFC 7084 `RFC7084-4.2-9` (WLL-2), `RFC7084-4.2-10` (WLL-3), `RFC7084-4.3-5` (L-2), `RFC7084-4.3-6` (L-3), `RFC7084-4.3-7` (L-4); RFC 9915 Section 6.3 rows `RFC9915-6.3-1`, `RFC9915-6.3-2`, `RFC9915-6.3-3`, today `{feature-declined}` |
| Depends | Owner scope decision (below); `plan/release/validation/spec-ppp-ipv6-link-local-never-assigned.md` (link-local on `pppN`); `spec-home-cpe-1-logical-interface-name.md` (the LAN unit names the PPPoE client by its configured name) |
| Phase | 0/5 |
| Handoff | - |
| Updated | 2026-10-31 |

Parent: `plan/home-cpe/spec-home-cpe-0-umbrella.md` (child 8).

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decision needed before this spec moves to ready

Using a delegated prefix on the LAN lifts a feature the owner declined. The
three RFC 9915 Section 6.3 rows in `rfc/short/rfc9915.md` (lines 90 to 92,
`RFC9915-6.3-1`, `RFC9915-6.3-2`, `RFC9915-6.3-3`) carry `{feature-declined}`
with the reason "The owner kept delegated-prefix use declined on 2026-10-19
(plan/post-release/features/spec-dhcpv6-client-0-umbrella.md, Q5)". That is an
owner scope decision, so this spec cannot reverse it on its own.

| Decision | What each answer changes |
|----------|--------------------------|
| Lift the 2026-10-19 decline of delegated-prefix use for the home-CPE role | Yes: the three rows become `{gap}` now and are implemented and proven here (Phases 4 and 5). No: Phases 1 to 3 (IPv6CP, WAN RA/SLAAC, requesting PD on the PPPoE interface) still stand, the delegated prefix stays published and unused, and RFC 7084 L-2, L-3 and L-4 stay gaps |

## Task

Make Ze a dual-stack CPE on a PPPoE WAN:

1. **IPv6CP in the PPPoE client** (RFC 5072). Negotiate the Interface-Identifier
   alongside IPCP, independently of it (RFC 7084 WLL-3), and install the
   link-local address on `pppN` from the negotiated identifier.
2. **RA and SLAAC on the WAN.** Accept Router Advertisements on `pppN` with
   forwarding on (accept_ra 2), install the default route from Router
   Discovery (RFC 7084 W-3), autoconfigure a global address where the ISP
   offers one.
3. **DHCPv6 prefix delegation on the PPPoE interface** (RFC 9915), with the
   existing DHCPv6 client, which today runs only on an interface unit.
4. **A /64 from the delegated prefix on a named LAN interface** (RFC 7084 L-2),
   with the address on the LAN unit and the prefix advertised in its RAs, the
   advertised and installed lifetimes never longer than what remains of the
   delegated prefix (RFC 9915 Section 6.3), a Route Information Option for the
   delegated prefix (L-3), and Router Lifetime 0 while no prefix is configured
   or delegated (L-4).
5. **No Release while delegated** (`RFC9915-6.3-3`).

## Required Reading

### Architecture Docs
- [ ] `docs/guide/pppoe.md`, `docs/guide/configuration.md` (interface units, `ipv6 dhcpv6`, `router-advertisement`)
  → Constraint: `pppoe-client` uses `interface-common` only (`ze-iface-conf.yang` l.1812), so it has no `ipv6` container today.
- [ ] `plan/post-release/features/spec-dhcpv6-client-0-umbrella.md` Q5
  → Decision: owner, 2026-10-19: the 17 other declined rows stay declined; delegated-prefix use among them. This spec asks to lift it for the CPE role.
- [ ] `plan/post-release/features/spec-router-advertisement-options-out-of-scope.md`
  → Constraint: it holds RFC 4191 route information and router preference as deferred options. This spec implements the RIO only for a delegated prefix, as RFC 7084 L-3 requires; router preference and operator-configured routes stay there.
- [ ] `plan/release/validation/spec-ppp-ipv6-link-local-never-assigned.md`
  → Constraint: whether the kernel derives a link-local on a PPP device is unverified; the client installs the link-local from the negotiated identifier itself, as pppd does, and that spec's live proof covers the server side.
- [ ] `plan/post-release/features/spec-l2tp-ipv6-subscriber.md`
  → Constraint: the subscriber (server) IPv6 side is that spec's; this one is the client.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5072.md`
  → Constraint: RFC 5072 Section 2: "Before any IPv6 packets may be communicated, PPP MUST reach the network-layer protocol phase, and the IPv6 Control Protocol MUST reach the Opened state."
  → Constraint: RFC 5072 Section 4.1: "A Configure-Request MUST contain exactly one instance of the interface-identifier option [1]."
  → Constraint: RFC 5072 Section 5: "The negotiated interface identifier is used by the local end of the PPP link to autoconfigure an IPv6 link-local unicast address for the PPP interface."
- [ ] `rfc/short/rfc7084.md`
  → Constraint: RFC 7084 Section 4.3 L-2: "The IPv6 CE router MUST assign a separate /64 from its delegated prefix(es) (and ULA prefix if configured to provide ULA addressing) for each of its LAN interfaces."
  → Constraint: "Prefix delegation is two halves" (the summary's pitfall): requesting IA_PD is the client, carving and advertising is the CE glue.
- [ ] `rfc/short/rfc9915.md`
  → Constraint: RFC 9915 Section 6.3: "If the client uses a delegated prefix to configure addresses on interfaces on itself or other nodes behind it, the preferred and valid lifetimes of those addresses MUST be no longer than the remaining preferred and valid lifetimes, respectively, for the delegated prefix at any time."
- [ ] `rfc/short/rfc4861.md`, `rfc/short/rfc4862.md` - RA and SLAAC on the WAN and LAN.

**Key insights:**
- The server side already parses and writes IPv6CP options (`internal/component/l2tp/ppp/ipv6cp.go`: `parseIPv6CPOptions`, `writeIPv6CPOptions`, `isValidIPv6CPInterfaceID`, `generateIPv6CPInterfaceID`, `suggestIPv6CPInterfaceID`); the client reuses that codec rather than writing a second one.
- The DHCPv6 client already requests IA_PD and keeps the binding (`v6KindPrefix`, "published, not installed"); the missing half is the consumer.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/pppoeclient/session.go` - `negotiateIPCP` (l.618, called l.130): the client runs LCP, authentication, then IPCP only.
- [ ] `internal/component/l2tp/pppoeclient/network_phase.go` - `keepaliveLoop` (l.94): a unit frame whose protocol is not IPCP is dropped with `continue` (l.136); IPv6CP from the AC is never answered.
- [ ] `internal/component/l2tp/ppp/ipv6cp.go` - the server-side IPv6CP option codec and identifier rules.
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `list pppoe-client` (l.1708, `uses interface-common` l.1812); `interface-unit` (l.202) holds `ipv6` (l.432: `address`, `autoconf`, `accept-ra`, `forwarding`), `dhcpv6` (l.491: `enabled`, `pd length`, `duid`) and `router-advertisement` (l.537: lifetimes, flags, `prefix` list, `rdnss`).
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_lease_linux.go` - `v6KindPrefix`: "an IA Prefix inside the IA_PD, published, not installed".
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_linux.go` - `effect` installs IA_NA addresses and publishes each IA_PD prefix as a `dhcp-acquired` event; no subscriber configures an address from it (`rfc/short/rfc9915.md` row 6.3-1 reason).
- [ ] `internal/component/iface/config_ra.go` - `raUnitConfig` (`Prefixes` from the operator's `prefix` list only, `RouterLifetime` fixed by configuration).
- [ ] `internal/plugins/iface/ra/sender_linux.go`, `internal/core/ndp/ra.go` - `BuildRA` writes SLLA, PIO and RDNSS; no RIO; subscribes to no DHCP event.
- [ ] `test/interop-pppoe/scenarios/ipv6cp-missing-option`, `ipv6cp-zero-identifier` - IPv6CP scenarios on the AC side.

**Behavior to preserve:**
- IPv4-only PPPoE sessions against an AC that does not offer IPv6CP: the client's IPv6CP Configure-Request is rejected or times out and IPCP stays Opened (RFC 7084 WLL-3, independent NCPs).
- Operator-configured RA prefixes and lifetimes on units with no delegated prefix.
- The DHCPv6 client on interface units.

**Behavior to change:**
- The client opens IPv6CP; DHCPv6 and the IPv6 sysctls apply on `pppN`; a LAN unit can take a /64 from a named delegation.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `interface pppoe-client <name> ipv6 { accept-ra; autoconf; dhcpv6 { enabled; pd { length } } }` and, on the LAN unit, a delegated-prefix source naming the PPPoE client and a subnet id, with `router-advertisement enabled`.
- Wire: IPv6CP (PPP protocol 0x8057) on the PPPoE session; RA on `pppN`; DHCPv6 Reply with IA_PD.

### Transformation Path
1. LCP and authentication as today; then IPCP and IPv6CP negotiate independently on the unit.
2. On IPv6CP Opened, the client installs `fe80::<identifier>/64` on `pppN` and applies `accept_ra 2`, `autoconf`, `forwarding`.
3. The DHCPv6 client starts on `pppN`, requests IA_PD with the configured hint, and publishes the binding with its lifetimes and receive instant.
4. The LAN unit's delegated-prefix consumer derives the /64 at the configured subnet id, installs the address with lifetimes bounded by what remains of the delegation, and hands the prefix to the RA sender.
5. The RA sender advertises the /64 in a PIO and the delegated prefix in a RIO, lifetimes bounded the same way, Router Lifetime 0 while no prefix is configured or delegated.
6. On a Renew or Rebind with new lifetimes, or loss of the delegation, the address and the advertised lifetimes follow; on loss, the PIO is advertised with zero lifetimes and the address removed. No Release is sent while the delegation is in use.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| PPPoE client ↔ kernel | link-local and sysctls on `pppN` via netlink | No |
| DHCPv6 client ↔ iface | `dhcp-acquired` event carrying the IA_PD binding | No |
| iface ↔ RA sender | the unit's RA configuration with derived prefixes | No |

### Integration Points
- `internal/component/l2tp/ppp/ipv6cp.go` codec reused by the client.
- The DHCPv6 client's event bus publication, already present.
- `raUnitConfig.Prefixes`, extended with prefixes derived at run time.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | the LAN consumer reads the published event; the DHCP plugin configures no other interface |
| No unintended coupling | Yes | the PPPoE client does not call the DHCPv6 client; config wires them |
| No duplicated functionality | Yes | one IPv6CP codec; the `ipv6` container is a grouping shared by unit and pppoe-client |
| Zero-copy preserved where applicable | Yes | IPv6CP frames written into the session buffer, as IPCP |
| Registration over hardcoding, outbound | Yes | consumers subscribe to the event |
| Registration over hardcoding, inbound | Yes | the RA sender receives prefixes, it learns no DHCP name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | accel-ppp serves IPv6CP, RAs and DHCPv6 IA_PD on PPPoE | accel-ppp `ipv6pool` and `ipv6-dhcp` modules | interop peer must change | read accel-ppp's source and configure the existing `Dockerfile.accel` image | unvalidated |
| A-2 | The kernel does not derive a link-local on `pppN` by itself | `spec-ppp-ipv6-link-local-never-assigned` premise | the client's install is redundant but harmless | that spec's live proof; the client test reads the address either way | unvalidated |
| A-3 | The ipv6 grouping can be shared by `pppoe-client` without the unit's other leaves | YANG grouping structure | a separate container duplicates leaves | YANG refactor in Phase 1 | unvalidated |
| A-4 | The owner lifts the decline for the CPE role | open | Phases 4 and 5 do not run | owner answer | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | IPv6CP failure tears down IPv4 | IPCP closes when IPv6CP fails | WLL-3 test: IPv6CP rejected, IPv4 traffic continues |
| R-2 | Lifetimes drift past the delegation's | advertised valid lifetime above remaining | computed from the receive instant at each RA; unit test with a fake clock |
| R-3 | Journal row in `silent-fall-through.md` (2026-10-31, owned by `spec-ppp-l2tp-aaa-one-off-defects`) changes the same loop (Protocol-Reject for unsupported protocols) | both edit `keepaliveLoop` | coordinate: IPv6CP is no longer "unsupported" once this lands; the Protocol-Reject path stays theirs |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | PPPoE sessions against IPv4-only ACs if NCP independence fails; LAN hosts keep addresses past the delegation |
| How is it reverted? | single commit revert per phase; config leaves are new, so existing configs are unaffected |
| Who else touches this path? | children 12, 13, 17 (PPPoE client); `spec-ppp-l2tp-aaa-one-off-defects`; the DHCPv6 client umbrella; RA options spec |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `pppoe-client ... ipv6` config | → | client IPv6CP negotiation | `test/parse/pppoe-client-ipv6.ci` and interop `pppoe-ipv6cp-ze-client` |
| `pppoe-client ... ipv6 dhcpv6 pd` | → | DHCPv6 client on `pppN` | interop `pppoe-dhcpv6-pd-ze-client` |
| LAN unit delegated-prefix source | → | /64 install and RA PIO/RIO | interop `pppoe-dhcpv6-pd-ze-client` (LAN host SLAAC) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | AC offers IPv6CP; client configured with `ipv6` | the client's Configure-Request carries exactly one Interface-Identifier option, non-zero; IPv6CP reaches Opened; `pppN` holds `fe80::<negotiated id>/64` |
| AC-2 | AC Naks the identifier with a suggestion | the client's next Configure-Request carries the suggested identifier (RFC 5072 Section 4.1) |
| AC-3 | AC rejects IPv6CP (Protocol-Reject) or ignores it | IPCP stays Opened and IPv4 traffic continues; no LCP termination |
| AC-4 | IPv6CP closes while IPCP is Opened | IPv4 traffic continues |
| AC-5 | AC sends RAs on the session | `pppN` has accept_ra 2 with forwarding on; a default route via the AC's link-local; a SLAAC address when the RA carries an autonomous prefix |
| AC-6 | `dhcpv6 pd length 56` on the PPPoE client | Solicit on `pppN` carries IA_PD with a /56 hint; the binding is published with its lifetimes; a different delegated length is accepted (WPD-3) |
| AC-7 | LAN unit names the delegation with subnet id 1 (owner decision Yes) | the unit holds `<delegated>:1::1/64` (or the configured host part), preferred and valid lifetimes never above the delegation's remaining ones |
| AC-8 | same | the unit's RAs carry a PIO for the /64 with lifetimes bounded the same way, and a RIO for the delegated prefix |
| AC-9 | no prefix configured or delegated | the unit's RAs carry Router Lifetime 0 |
| AC-10 | Renew returns shorter lifetimes | the installed and advertised lifetimes shrink on the next RA |
| AC-11 | the delegation expires or the session ends | the PIO is advertised with zero lifetimes, the address removed, Router Lifetime 0 unless another prefix remains |
| AC-12 | client shutdown with a delegation in use | no Release for the delegated prefix while LAN addresses derived from it are outstanding |
| AC-13 | a LAN host (dhcpcd) on the LAN unit | it autoconfigures from the /64 and pings an address on the AC side over IPv6 through Ze |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `ipv6` on the PPPoE client | config → pppoe-client → IPv6CP → `pppN` link-local → RA → default route | `pppoe-ipv6cp-ze-client` |
| 2 | Configures PD and names the delegation on the LAN | DHCPv6 on `pppN` → event → LAN /64 → RA → LAN host SLAAC → IPv6 traffic | `pppoe-dhcpv6-pd-ze-client` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC5072ClientConfigureRequestOneIdentifier` | `internal/component/l2tp/pppoeclient/rfc5072_ipv6cp_client_test.go` | AC-1 | |
| `TestRFC5072ClientAdoptsNakSuggestion` | same | AC-2 | |
| `TestRFC7084WLL3NCPIndependence` | `internal/component/l2tp/pppoeclient/rfc7084_ncp_independence_test.go` | AC-3, AC-4 | |
| `TestRFC5072ClientLinkLocalFromIdentifier` | `rfc5072_ipv6cp_client_test.go` | AC-1 address | |
| `TestDelegatedSubnetDerivation` | `internal/component/iface/delegated_prefix_test.go` | subnet id inside the delegated length; refusal when out of range | |
| `TestRFC9915DelegatedLifetimeBound` | `internal/component/iface/rfc9915_delegated_lifetime_test.go` | AC-7, AC-10 with a fake clock | |
| `TestRFC7084L3RouteInformationOption` | `internal/core/ndp/rfc7084_rio_test.go` | AC-8 RIO encoding | |
| `TestRFC7084L4RouterLifetimeZero` | `internal/plugins/iface/ra/rfc7084_router_lifetime_test.go` | AC-9, AC-11 | |
| `TestRFC9915NoReleaseWhileDelegated` | `internal/plugins/iface/dhcp/rfc9915_dhcp_v6_release_linux_test.go` | AC-12 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| subnet id | 0 to 2^(64 - delegated length) - 1 | 255 for a /56 | N/A | 256 for a /56 |
| `pd length` | 48-64 per existing leaf | 64 | 47 | 65 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pppoe-client-ipv6` | `test/parse/pppoe-client-ipv6.ci` | the new leaves parse and validate | |
| `delegated-prefix-unknown-source` | `test/parse/` | a LAN unit naming an unknown PPPoE client is refused with the fix | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `pppoe-ipv6cp-ze-client` | `test/interop-pppoe/scenarios/` | accel-ppp | IPv6CP, link-local, RA default route; red walk: client drops IPv6CP | |
| `pppoe-ipv6cp-rejected-ze-client` | `test/interop-pppoe/scenarios/` | accel-ppp with IPv6 off | NCP independence (AC-3) | |
| `pppoe-dhcpv6-pd-ze-client` | `test/interop-topology/scenarios/` (child 18) | accel-ppp, dhcpcd | PD on `pppN`, LAN /64, RA PIO and RIO, LAN host traffic; red walk: lifetime bound removed, advertised valid lifetime above remaining | |

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - `ipv6` grouping shared by unit and `pppoe-client`; delegated-prefix source on the unit
- `internal/component/l2tp/pppoeclient/session.go`, `network_phase.go` - IPv6CP negotiation and Opened handling
- `internal/component/l2tp/ppp/ipv6cp.go` - codec exported to the client within the component
- `internal/plugins/iface/dhcp/dhcp_v6_linux.go` - DHCPv6 on `pppN`; no Release while delegated
- `internal/component/iface/config_ra.go`, `internal/plugins/iface/ra/sender_linux.go`, `internal/core/ndp/ra.go` - derived prefixes, RIO, Router Lifetime 0
- `rfc/short/rfc5072.md`, `rfc/short/rfc7084.md`, `rfc/short/rfc9915.md` - rows moved from gap or declined to tested
- `docs/guide/pppoe.md`, `docs/guide/configuration.md`
- `features/*.md` - PPPoE client and DHCPv6 entries

## Files to Create
- `internal/component/iface/delegated_prefix.go` and tests - the LAN consumer
- `internal/component/l2tp/pppoeclient/ipv6cp_client.go` and tests
- the interop scenario directories above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | Yes | `ze-iface-conf.yang` |
| YANG validation constraints | Yes | subnet id range, `pd length` reused |
| YANG custom validators | Yes | the delegated-prefix source must name a configured `pppoe-client` with `dhcpv6 pd`; completion of names |
| CLI commands/flags | Yes | `show interface` shows the delegated prefix and derived /64 |
| CLI grammar | Yes | keyword before value |
| Editor autocomplete | Yes | `CompleteFn` for the source name |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | Yes | show output through pipes |
| Env var registration | N-A | none |
| Doctor check | N-A | no new runtime dependency |
| Prometheus counters/metrics | Yes | IPv6CP state per session, delegated prefix present |
| BGP family surface | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `features/<pppoe-client>.md`, `features/<dhcpv6-client>.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/pppoe.md` |
| 7 | Wire format changed? | Yes | IPv6CP diagram in the client source with RFC 5072 offsets; RIO in `ra.go` |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc5072.md`, `rfc7084.md`, `rfc9915.md`, `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` IPv6 CPE row |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | Yes | subsystem telemetry doc |
| 15 | Registered inventory changed? | No | |
| 16 | Changed source referenced by doc anchors? | Yes | derived by `./le spec citation anchors` at implementation |
| 17 | Existing docs show examples for this area? | Yes | `pppoe.md` examples |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- YANG `ipv6` grouping on `pppoe-client`, parse test, interop scenario skeleton failing at IPv6CP
2. **Phase: IPv6CP client** -- AC-1 to AC-4; RFC 5072 quotes above each implementing function
3. **Phase: WAN RA, SLAAC and DHCPv6 on `pppN`** -- AC-5, AC-6
4. **Phase: LAN /64 and RA** (after the owner's Yes) -- AC-7 to AC-11
5. **Phase: No Release, rows and docs** -- AC-12, AC-13; RFC rows recorded with `./le rfc record`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | lifetimes computed from the Reply's receive instant, not the configured RA values |
| Data flow | the DHCP plugin configures no LAN address itself; iface consumes the event |
| Rule: no-layering | one IPv6CP codec; one `ipv6` grouping |
| Rule: rfc-compliance | every implementing function quotes its RFC sentence |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| IPv6CP client | unit tests and `pppoe-ipv6cp-ze-client` |
| PD to LAN | `pppoe-dhcpv6-pd-ze-client` green with red walk |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | IPv6CP option lengths; a delegated prefix shorter than /48 or longer than /64 refused for LAN derivation |
| Resource exhaustion | repeated Configure-Naks bounded by the existing max-configure counter |

## Owned journal rows

None owned. Related: `plan/journal/silent-fall-through.md` row 2026-10-31 (PPPoE client drops IPv6CP, CCP and unknown protocols), owned by `spec-ppp-l2tp-aaa-one-off-defects`; this spec removes IPv6CP from that row's set when it lands.

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Reuse the server's IPv6CP codec | a client-only codec | one implementation of RFC 5072 options |
| Share the unit's `ipv6` grouping on `pppoe-client` | give `pppoe-client` a unit | the PPPoE interface has no VLAN or unit semantics; a grouping avoids a second copy of the leaves |
| LAN consumer in iface, fed by the published event | the DHCP plugin installs LAN addresses | keeps the DHCP plugin from configuring other interfaces; matches the existing publication |
| RIO only for the delegated prefix | wait for the RA options spec | L-3 is a MUST of the CE role this spec implements |

## Known Limitations

- ULA generation (RFC 7084 ULA-1 to ULA-5) and a LAN DHCPv6 server (L-10) are not in this spec; they stay gaps in `rfc/short/rfc7084.md` for the RA options and DHCPv6 server specs.
- Delegation to further routers behind the CPE is out of scope.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

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

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] Interop red walks recorded

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A** and **Commit B** per `ai/rules/planning.md`
