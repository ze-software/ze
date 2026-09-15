# Spec: GTSM related ICMP messages carry and verify TTL 255

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | implementation |
| Handoff | - |
| Updated | 2026-09-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`./le rfc check` reports RFC5082-3-2 [MUST] with no test and no annotation. RFC
5082 Section 3 says: "The TTL field in all IP packets used for transmission of
messages associated with GTSM-enabled protocol sessions MUST be set to 255.
This also applies to the related ICMP error handling messages." Section 6.1
restates it as an obligation in both directions: "This specification mandates
setting and verifying TTL=255 of those as well as the main protocol packets."

The requirement has four cells, a transmit half and a receive half in each
family. Three of the four are produced by no layer today, so a GTSM peer of ze
receives ICMP errors from ze at the system default TTL, and ze accepts an
off-link ICMP error that claims to be about an IPv4 GTSM session. The second is
the attack RFC 5082 Section 6.1 names: "related messages provide a significant
attack vector to e.g., reset protocol sessions".

| Cell | Kernel behavior, read in Linux 7.2 | Status before this spec |
|------|-----------------------------------|-------------------------|
| Receive IPv6 | `tcp_v6_err` (`net/ipv6/tcp_ipv6.c`) compares the ICMPv6 error's OWN hop limit against the socket's `min_hopcount` | MET on state ze installs: `setIPMinTTL` sets IPV6_MINHOPCOUNT. Owes a tagged test |
| Receive IPv4 | `tcp_v4_err` (`net/ipv4/tcp_ipv4.c`) compares IP_MINTTL against the TTL of the header `skb->data` points at, which `icmp_rcv` (`net/ipv4/icmp.c`) has pulled to the QUOTED header of ze's own earlier packet | NOT met: the gated field sits inside the ICMP payload, so the sender of the error chooses it. Needs a filter ze installs |
| Transmit IPv4 | A locally generated ICMP error takes its TTL from `ip_select_ttl` (`net/ipv4/ip_output.c`), which reads the RTAX_HOPLIMIT route metric through `ip4_dst_hoplimit` before `net.ipv4.ip_default_ttl` | NOT met: ze installs no such metric |
| Transmit IPv6 | The same through `ip6_dst_hoplimit` | NOT met: ze installs no such metric |

The goal is all four cells produced by state ze installs, and each one proven in
both polarities by a tagged test with a discrimination record.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component boundaries and the registration pattern
  → Decision: the GTSM kernel state is published by a component the BGP reactor calls, not by new code inside the reactor
  → Constraint: a component reaches the kernel firewall through `firewall.RegisterTables`, never through its own nftables writer
- [ ] `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` - who owns a `ze_` table and when it is withdrawn
  → Constraint: a daemon-owned table carries the `ze_` prefix, and shutdown teardown is owned centrally by the firewall engine
- [ ] `docs/guide/firewall.md` - the operator-visible firewall surface
  → Constraint: the new match types are daemon-only, so no YANG leaf and no config syntax changes
- [ ] `docs/architecture/doctor-and-health-checks.md` - the three tiers of self-diagnosis and when each one's fact is produced
  → Constraint: `ze doctor` is offline readiness in the operator's own process, so a check that reads state the daemon installs can answer that question only inside the daemon; before a start it probes the environment instead

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5082.md` - GTSM, the four-cell fact and the kernel citations
  → Constraint: RFC5082-3-4 forbids dropping a packet no GTSM session claims, so the ICMP filter must associate the error with a BGP session of that peer before it drops anything
  → Constraint: the inbound floor is the peer's own `MinTTL` (255-N+1 for `ttl max N`), never a hard-wired 255, so the multi-hop form keeps working

**Key insights:**
- Ze meets GTSM through the kernel. Conformance is judged on the whole stack, and the proof asserts the state ze installs.
- The vendored `vishvananda/netlink` carries `Route.Hoplimit` (`route.go`) and encodes it as RTAX_HOPLIMIT in `routeHandle` (`route_linux.go`), which also decodes it, so the transmit half is a field on a route rather than a new netlink capability.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/network/ttl_linux.go` - sets IP_TTL / IPV6_UNICAST_HOPS and IP_MINTTL / IPV6_MINHOPCOUNT on a peer socket
- [ ] `internal/component/bgp/reactor/session_connection.go` - `tuneTCPConnectionForSettings` installs those options per connection
- [ ] `internal/component/bgp/reactor/config.go` - `parseTTLSettings` derives OutTTL 255 and MinTTL 255-N+1 from `ttl max N`
- [ ] `internal/component/bgp/reactor/reactor.go` - `listenTTLForListener` and `md5PeersForListener` iterate `r.peers` for kernel state that is not per connection
- [ ] `internal/plugins/vrrp/acceptfilter.go` - a protocol feature publishing a daemon-owned nftables table through `firewall.RegisterTables` plus `firewall.ApplyAll`
- [ ] `internal/plugins/firewall/nft/lower_linux.go` - `lowerMatch`, `lowerDSCPMatch` and `nfprotoGuard`: a raw network-header read is guarded by nfproto so it cannot reach the other family
- [ ] `internal/component/firewall/model.go` - the Match interface and its implementations
- [ ] `internal/component/firewall/validate.go` - `validateMatch` refuses a match in a family whose header the lowering cannot read

**Behavior to preserve:**
- The socket options and the listen-socket TTL are unchanged: RFC5082-3-1, RFC5082-3-3 and RFC5082-3-4 keep the producers and the tests they have.
- A peer with no `ttl` block installs nothing, so a deployment that does not use GTSM gets no route, no firewall table and no nftables backend load.
- `firewall.ApplyAll` keeps its current owners; this spec adds one more owner and no new writer of the kernel.

**Behavior to change:**
- A GTSM-enabled BGP peer gains a host route carrying hop limit 255, and an nftables input table that drops an ICMP error which claims to be about that peer's BGP session and arrives below the peer's TTL floor.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator config: `bgp { peer <name> { connection { ttl { max N } } } }`, parsed by `parseTTLSettings` into `PeerSettings.OutTTL` and `PeerSettings.MinTTL`.

### Transformation Path
1. Config apply reconciles the peer set in the reactor (`ReconcilePeersWithJournal`, `StartPeers`).
2. The reactor derives the GTSM peer set from `r.peers`: address, BGP port, outbound hop limit, inbound floor.
3. `gtsm.SetPeers` in the new component reconciles two kinds of kernel state from that set.
4. The route half installs, for each peer, a host route to the peer address carrying the hop limit as RTAX_HOPLIMIT, with the nexthop the kernel already resolves for that address.
5. The filter half publishes one `ze_gtsm` table through `firewall.RegisterTables` and `firewall.ApplyAll`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor ↔ GTSM component | A value-typed peer slice passed to one exported function | No |
| GTSM component ↔ firewall component | `firewall.RegisterTables` under owner `gtsm`, then `firewall.ApplyAll` | No |
| GTSM component ↔ kernel | netlink route replace and delete; the firewall backend owns the nftables writes | No |

### Integration Points
- `firewall.RegisterTables` / `firewall.ApplyAll` - the same publication path copp, vrrp, policyroute and flowspec-firewall already use.
- `network.SetIPMinTTL` - the socket option that already meets the IPv6 receive cell; this spec adds its test, not its code.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A locally generated ICMP error reads RTAX_HOPLIMIT from the route to its destination | `ip_select_ttl` reading `ip4_dst_hoplimit` before the sysctl, recorded in `rfc/short/rfc5082.md` | The transmit half needs another mechanism, and the owner's per-peer decision has to be put to him again | The transmit tagged tests observe the TTL of a captured ICMP error | confirmed: `TestGTSMTransmittedICMPErrorCarriesTTL255` reads 255 off the wire with the metric installed and the system default with it withdrawn, in both families |
| A-2 | The quoted IPv4 header inside an ICMP error about a ze packet is 20 bytes, so the quoted TCP ports sit at fixed offsets | Ze sets no IP options on a BGP socket, and a transit router adds none to a packet that carries none | The port offsets read the wrong bytes | The lowering compares the quoted version and header-length byte against 0x45 first, so a longer quoted header matches nothing | confirmed: the rules the kernel holds carry the 0x45 compare ahead of the port read, and `TestGTSMDeliversAnICMPErrorNoSessionClaims` shows a quoted port of another flow matching nothing |
| A-3 | The netlink library encodes RTAX_HOPLIMIT from `Route.Hoplimit` | `routeHandle` in `vendor/github.com/vishvananda/netlink/route_linux.go`, read directly | The route half cannot be expressed with the vendored library | The transmit tagged tests, which read the metric back off the kernel | confirmed: `routeHopLimit` reads the metric back off the kernel route in every transmit proof |
| A-4 | One BGP session of a GTSM peer always carries the configured BGP port as one of its two TCP ports | `peerListenPort` decides the port for the dial and the listen direction alike | The filter fails to claim a session, and a Dangerous error is delivered | The receive tagged tests inject both port polarities | confirmed for the derivation: `TestReactorPublishesAPeersOwnListenPort` shows the published port following `LocalPort`. The filter carries a term for each side, so either direction's quoted header is claimed |
| A-5 | An IPv4 route delete is matched on the destination alone | assumed while writing `withdrawHopLimitRoute`, and never checked | A withdrawn peer keeps a metric its configuration no longer asks for | `TestGTSMTransmittedICMPErrorWithoutTheRouteMetricIsNot255`, which reads the route back after the withdraw | BROKEN: the delete is matched on the scope as well, so a link-scoped route survived a delete naming the default scope. The withdraw now deletes the route the kernel holds. IPv6 did not show it, so only the IPv4 proof failed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A host route installed for a peer takes over the forwarding decision for that address and breaks the session | The session fails to establish after a config apply | The route copies the nexthop the kernel already resolves for the address, and carries a ze route protocol so it is identifiable and removable |
| R-2 | The ICMP filter drops an Unknown packet and breaks RFC5082-3-4, which is already proven | Ze's own ping or traceroute to the peer loses replies | Every drop term requires the ICMP error to quote a TCP header on that peer's BGP port, so an error about another flow is never claimed |
| R-3 | The filter's rule count grows with the peer count on the input path | A slow input path on a router with many GTSM peers | The rule count is ten per GTSM peer, bounded by operator config, and every term starts with the cheap ICMP type test |
| R-4 | The firewall backend is loaded on a box that configured no firewall | The nftables backend appears where the operator asked for nothing | The component publishes nothing and skips `ApplyAll` when the GTSM peer set is empty |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A wrong host route drops a GTSM peer's session; a wrong filter term drops ICMP the operator needs. Both are limited to peers that configured a `ttl` block |
| How is it reverted? | Single commit revert. The route and the table are both withdrawn by the reconcile that no longer names the peer |
| Who else touches this path? | The firewall backend owners (copp, vrrp, policyroute, flowspec-firewall, ddos) share `firewall.ApplyAll`; the reactor peer reconcile is shared with the MD5 and listen-TTL kernel state |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A peer config carrying `connection ttl max N` reaches the reactor peer reconcile | → | `Reactor.gtsmPeers` feeding `gtsm.SetPeers` | `TestReactorPublishesGTSMPeersFromConfig` |
| `gtsm.SetPeers` with one peer | → | `icmpFilterTables` publishing the `ze_gtsm` table | `TestGTSMPublishesTheICMPFilterTableForAPeer` |
| `gtsm.SetPeers` with no peer | → | the withdraw path | `TestGTSMWithdrawsEverythingWhenNoPeerEnablesIt` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A BGP peer configured with `connection ttl max N` | Ze installs a host route to the peer address carrying hop limit 255 as its RTAX_HOPLIMIT metric |
| AC-2 | An ICMP error the kernel generates toward that peer | It leaves with TTL 255 for IPv4, or hop limit 255 for IPv6 |
| AC-3 | An ICMPv4 error arriving from that peer, quoting a TCP header on the peer's BGP port, with an outer TTL below the peer's floor | It is dropped before the TCP stack processes it |
| AC-4 | The same ICMPv4 error arriving with outer TTL 255 | It is delivered |
| AC-5 | An ICMPv4 error arriving from that peer quoting a TCP header on any other port, with an outer TTL below the floor | It is delivered, because no GTSM session claims it (RFC5082-3-4) |
| AC-6 | An ICMPv6 error arriving with its own hop limit below the floor, about a socket ze gave IPV6_MINHOPCOUNT | The kernel drops it and counts it in TCPMinTTLDrop |
| AC-7 | A peer whose `ttl` block is removed, or a peer deleted from the config | The host route and that peer's filter terms are withdrawn |
| AC-8 | A configuration with no GTSM peer | No route, no `ze_gtsm` table, and no firewall apply |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `connection ttl max 1` on a peer and expects ze's ICMP errors to that peer to pass the peer's own GTSM check | config → parseTTLSettings → reactor peer reconcile → gtsm.SetPeers → netlink route with RTAX_HOPLIMIT | `TestGTSMTransmittedICMPErrorCarriesTTL255` |
| 2 | Configures the same peer and expects a spoofed off-link ICMP error not to disturb the session | config → gtsm.SetPeers → firewall.RegisterTables → nftables input drop | `TestGTSMDropsADangerousQuotedICMPError` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGTSMPublishesTheICMPFilterTableForAPeer` | `internal/component/gtsm/gtsm_test.go` | the table, chain and term shape built for one peer | |
| `TestGTSMWithdrawsEverythingWhenNoPeerEnablesIt` | `internal/component/gtsm/gtsm_test.go` | an empty peer set publishes no table | |
| `TestReactorPublishesGTSMPeersFromConfig` | `internal/component/bgp/reactor/gtsm_test.go` | the reactor derives the peer set from the config it parsed | |
| `TestLowerIPv4TTLBelowReadsTheTTLByteUnderAnNfprotoGuard` | `internal/plugins/firewall/nft/lower_linux_test.go` | the TTL match lowering | |
| `TestLowerICMPErrorQuotedTCPPortChecksTheQuotedHeader` | `internal/plugins/firewall/nft/lower_linux_test.go` | the quoted-header match lowering | |
| `TestGTSMTransmittedICMPErrorCarriesTTL255` | `internal/component/gtsm/gtsm_rfc5082_linux_test.go` | RFC5082-3-2 transmit IPv4, positive | |
| `TestGTSMTransmittedICMPErrorWithoutTheRouteMetricIsNot255` | `internal/component/gtsm/gtsm_rfc5082_linux_test.go` | RFC5082-3-2 transmit IPv4, negative | |
| `TestGTSMTransmittedICMPv6ErrorCarriesHopLimit255` | `internal/component/gtsm/gtsm_rfc5082_linux_test.go` | RFC5082-3-2 transmit IPv6, positive | |
| `TestGTSMTransmittedICMPv6ErrorWithoutTheRouteMetricIsNot255` | `internal/component/gtsm/gtsm_rfc5082_linux_test.go` | RFC5082-3-2 transmit IPv6, negative | |
| `TestGTSMDropsADangerousQuotedICMPError` | `internal/component/gtsm/gtsm_rfc5082_linux_test.go` | RFC5082-3-2 receive IPv4, positive | |
| `TestGTSMDeliversAQuotedICMPErrorAtTTL255` | `internal/component/gtsm/gtsm_rfc5082_linux_test.go` | RFC5082-3-2 receive IPv4, negative | |
| `TestGTSMDeliversAnICMPErrorNoSessionClaims` | `internal/component/gtsm/gtsm_rfc5082_linux_test.go` | the RFC5082-3-4 guard on the new filter | |
| `TestGTSMMinHopCountDropsALowHopLimitICMPv6Error` | `internal/core/network/ttl_gtsm_linux_test.go` | RFC5082-3-2 receive IPv6, positive | |
| `TestGTSMWithoutMinHopCountDeliversTheSameICMPv6Error` | `internal/core/network/ttl_gtsm_linux_test.go` | RFC5082-3-2 receive IPv6, negative | |
| `TestGTSMKernelStateDoctorCheckRegistered`, `TestGTSMKernelStateDoctorCheckIsSilentWithoutBGP`, `TestGTSMKernelStateDoctorCheckRefusesToGuessWithoutADerivation` | `internal/component/gtsm/doctor_test.go` | the doctor check is in the registry the runner reads, its code resolves for `ze explain`, and it is silent with no bgp{} block | pass |
| `TestGTSMDoctorReportsAPublishedPeerWhoseRouteIsMissing`, `...WhoseTableIsMissing`, `TestGTSMDoctorIsSilentWhenTheKernelHoldsWhatWasPublished`, `TestGTSMDoctorNeverReadsTheFirewallForAnIPv6OnlySet`, `TestGTSMDoctorReportsAnUnreadableFirewallOnce`, `TestGTSMDoctorReportsAConfiguredPeerTheKernelCannotRoute` | `internal/component/gtsm/doctor_linux_test.go` | both modes of the check, driven through `diagnostic.DoctorChecksForPhase` against a stood-in kernel | pass |
| `TestGTSMPeersFromResolvedTreeReadsTheConfigAlone` | `internal/component/bgp/reactor/gtsm_test.go` | the configuration-only derivation the doctor check reads through `gtsm.SetConfigPeers` | pass |
| `TestBespokeCheckerBranches/gtsm-related-icmp-ttl` | `internal/le/interoplab/bgp/bgp_test.go` | the interop predicates, both polarities: arrival, socket match, floor drop, FRR's ttl-security field | pass |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Route hop limit | 1-255 | 255 | 0 means "no metric", so it is refused as a GTSM value | N/A, the field is a uint8 |
| Inbound floor | 1-255 | 255 | 0 means "no GTSM", so no term is built | N/A, the field is a uint8 |
| Quoted TCP port | 1-65535 | 65535 | 0 is refused: no BGP session carries it | N/A, the field is a uint16 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `gtsm-related-icmp` | `test/firewall/gtsm-related-icmp.ci` | An operator's document with `connection ttl max 1` starts a daemon that installs the `ze_gtsm` table and the host route carrying hop limit 255; a reload without the `ttl` block withdraws both (AC-1, AC-7, AC-8). Driver: `firewall/gtsm-related-icmp` (`internal/test/fixture/netfilter_fixture_gtsm.go`), which reads `nft list table inet ze_gtsm` and `ip route show table main 192.0.2.2/32` before and after the reload | WRITTEN 2026-09-14. Green under `ZE_TEST_NETNS=1` as root (15 iterations under load, `./le stress-repro`). RED with the metric install removed from `installHopLimitRoute`: `no host route to 192.0.2.2 carrying hoplimit 255: 192.0.2.2 dev zegtsm0 proto 249 scope link`; green again with it restored |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `gtsm-related-icmp-ttl` | `test/interop/scenarios/gtsm-related-icmp-ttl/` | FRR 10.3.1 | An IPv6 GTSM session (`ttl max 1` on ze, `ttl-security hops 1` on FRR). The checker (`checkGTSMRelatedICMPTTL`, `internal/le/interoplab/bgp/check_rfc.go`) installs one nftables reject rule in ze's container that answers ONE TCP segment of the session from FRR with an ICMPv6 Destination Unreachable, generated by ze's kernel and so carrying the hop limit of ze's host route. It then reads FRR's kernel: `Icmp6InDestUnreachs` advanced (the error arrived), `Icmp6InErrors` unchanged (it was matched to the session's socket), `TCPMinTTLDrop` unchanged (it passed the IPV6_MINHOPCOUNT 255 floor), and the session stays Established. IPv6 on purpose: Linux verifies an ICMPv6 error by its OWN hop limit, while the IPv4 check reads the quoted TTL the sender chooses, so only the IPv6 receiver can tell the metric's presence from its absence | WRITTEN 2026-09-14. Green on image `sha256:008637f0…` and again on `sha256:9a31a16f…`. RED on the mutated image `sha256:be8c6d15…` (metric install removed from `installHopLimitRoute`): `assertion 5: FRR dropped ze's ICMPv6 error below its GTSM floor (TCPMinTTLDrop 0 before, 1 after): the error did not carry hop limit 255`. Predicates in `check_rfc_predicate.go` are driven in both polarities by `TestBespokeCheckerBranches` |

## Files to Modify
- `internal/component/firewall/model.go` - two daemon-only match types
- `internal/component/firewall/validate.go` - validation for the two new matches
- `internal/plugins/firewall/nft/lower_linux.go` - their lowering
- `internal/component/bgp/reactor/reactor.go` - derive the GTSM peer set and publish it
- `internal/core/rtproto/rtproto.go` - the route protocol that marks ze's GTSM host routes
- `internal/le/tier/testdata/tier_non_engine_categories.txt` - the new component's tier category
- `rfc/short/rfc5082.md` - the requirement's producers and tests
- `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` - the `ze_gtsm` owner row
- `docs/DESIGN.md` - what GTSM now covers for a session's related ICMP messages
- `docs/architecture/testing/qemu-integration.md` - the one case where a kernel test carries bare `linux`: a unit an `RFC requirement:` tag makes the discrimination recorder run with no tags at all
- `plan/journal/counter-counts-the-wrong-packets.md` - the counter placement met again

## Files to Create
- `internal/component/gtsm/gtsm.go` - the peer set, the filter table builder and the reconcile entry point
- `internal/component/gtsm/route_linux.go` - the RTAX_HOPLIMIT host route
- `internal/component/gtsm/route_other.go` - the unsupported stub
- `internal/component/gtsm/gtsm_test.go` - the portable unit tests
- `internal/component/gtsm/gtsm_rfc5082_linux_test.go` - the tagged Linux proofs
- `internal/component/gtsm/netns_linux_test.go` - the namespace and veth harness those proofs run in
- `internal/component/gtsm/packet_linux_test.go` - the frames they inject
- `internal/component/bgp/reactor/gtsm_test.go` - the wiring test
- `internal/component/gtsm/doctor.go`, `register.go`, `doctor_test.go`, `doctor_linux_test.go` - the doctor check, its registration and its tests (2026-09-14)
- `internal/test/fixture/netfilter_fixture_gtsm.go` - the driver of `test/firewall/gtsm-related-icmp.ci` (2026-09-14)
- `test/interop/scenarios/gtsm-related-icmp-ttl/` - the FRR scenario; its checker and predicates are in `internal/le/interoplab/bgp/check_rfc.go` and `check_rfc_predicate.go` (2026-09-14)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | The feature is derived from the existing `connection ttl` leaves; it adds no leaf |
| YANG validation constraints | No | No new leaf |
| YANG custom validators | No | No new leaf |
| CLI commands/flags | No | The table is visible through the existing `show firewall ruleset` |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | No | No new leaf |
| Functional test for new RPC/API | No | No RPC added |
| Pipe completeness | N-A | No command added |
| Env var registration | No | No environment leaf |
| Doctor check for runtime dependencies | Yes | `gtsm-kernel-state`, code `doctor-gtsm-kernel-state`, registered from `internal/component/gtsm/register.go` through `diagnostic.RegisterDoctorCheck` (`internal/component/gtsm/doctor.go`, `checkKernelState`). Inside the daemon (`show doctor`, the support bundle) it reads the set `SetPeers` published against the kernel and names each peer short of its host route with hop limit 255 or of the `ze_gtsm` table. In `ze doctor`, before a start, nothing is published and the kernel state is legitimately absent, so it derives the peer set from the configuration through the reactor's own parse (`gtsmPeersFromResolvedTree`, filled into `gtsm.SetConfigPeers`) and reports a peer the kernel resolves no route to, which is the install that fails at apply. Unit tests drive it through `diagnostic.DoctorChecksForPhase` (`doctor_test.go`, `doctor_linux_test.go`); the derivation is tested in `reactor/gtsm_test.go`. The code is in `internal/core/diagnostic/codes.go` and `docs/guide/health-checks.md` |
| Prometheus counters/metrics | No | The firewall component already counts rule hits |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | No | The existing `connection ttl` block drives it |
| 3 | CLI command added/changed? | No | No command added |
| 4 | API/RPC added/changed? | No | No RPC added |
| 5 | Plugin added/changed? | No | No plugin added |
| 6 | Has a user guide page? | Yes | `docs/guide/firewall.md` |
| 7 | Wire format changed? | No | No ze-encoded message changes |
| 8 | Plugin SDK/protocol changed? | No | No SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc5082.md`. `docs/features/rfc-status.md` is generated on demand and gitignored, so it carries no edit |
| 10 | Test infrastructure changed? | No | No runner change |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` GTSM row |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` where the component list is enumerated |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | No new counter |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration surface changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED from `./le spec citation anchors` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The GTSM examples in `docs/guide/` are re-read against the parser |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the reactor derives the GTSM peer set and calls the new component
   - Tests: `TestReactorPublishesGTSMPeersFromConfig`, `TestGTSMWithdrawsEverythingWhenNoPeerEnablesIt`
   - Files: `internal/component/gtsm/gtsm.go`, `internal/component/bgp/reactor/reactor.go`
   - Verify: the entry point exists and is reachable, and the wiring test fails while the component is a stub
2. **Phase: the two firewall matches** -- model, validation and lowering
   - Tests: `TestLowerIPv4TTLBelowReadsTheTTLByteUnderAnNfprotoGuard`, `TestLowerICMPErrorQuotedTCPPortChecksTheQuotedHeader`
   - Files: `internal/component/firewall/model.go`, `internal/component/firewall/validate.go`, `internal/plugins/firewall/nft/lower_linux.go`
   - Verify: the lowering emits the guard before the raw read, and an unreadable family refuses
3. **Phase: the filter table** -- build the terms for a peer and publish them
   - Tests: `TestGTSMPublishesTheICMPFilterTableForAPeer`
   - Files: `internal/component/gtsm/gtsm.go`
   - Verify: eight terms per peer, each requiring the quoted BGP port
4. **Phase: the route metric** -- install and withdraw the host route
   - Tests: the four transmit tagged tests
   - Files: `internal/component/gtsm/route_linux.go`, `internal/component/gtsm/route_other.go`
   - Verify: the metric is read back off the kernel, and the captured ICMP error carries 255
5. **Phase: the proofs** -- the eight tagged tests and their discrimination records
   - Tests: every row of the TDD plan
   - Files: `internal/component/gtsm/gtsm_rfc5082_linux_test.go`, `internal/core/network/ttl_gtsm_linux_test.go`
   - Verify: `./le rfc check` no longer names RFC5082-3-2

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The floor comes from the peer's MinTTL, not a constant; the quoted-header offsets are guarded by the version and header-length test |
| Naming | The match type names say which family's header byte they read |
| Data flow | The reactor holds no nftables or netlink knowledge, and the GTSM component holds no BGP knowledge beyond a port number |
| Rule: `ai/rules/rfc-compliance.md` | Each tagged claim states what the test body checks and no more |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| RFC5082-3-2 proven in both polarities | `./le rfc check` no longer names it |
| Every new tag carries a discrimination record | `./le rfc discriminate stem rfc5082` lists none unproven |
| The kernel state is reachable from config | The wiring tests named above |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The filter reads attacker-chosen bytes inside the ICMP payload; the quoted version and header-length test is what keeps the port offsets meaningful |
| Fail closed | A peer whose route or table cannot be installed is reported, and no partial state is left claiming to be a GTSM peer |
| Denial of service | The drop happens in the kernel before the TCP stack, so a flood of Dangerous errors never reaches the daemon |

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

- The two halves of RFC5082-3-2 need two different kernel mechanisms, because the kernel reads a different TTL in each direction: a route metric on transmit, and either a socket option (IPv6) or a packet filter (IPv4) on receive.
- The IPv4 receive cell is the one Linux cannot answer with a socket option, because `IP_MINTTL` is compared against the TTL inside the quoted header, which the sender of the error chooses. That is why a filter is the only mechanism, and why the filter has to read the quoted header itself to associate the error with a session.
- A firewall rule's counter is NOT a match count in ze. `applyChain` puts the counter at the front of the expression list, so it counts the packets that REACHED the rule. The first draft of the receive proofs read one term's counter as "this rule matched" and passed on a value every rule reported, matching or not. The proofs now read the kernel's own `IcmpMsg InType3` for delivery, and use the counters only for what they truthfully say: the last rule with a non-zero count is the rule that matched, because a verdict ends the evaluation. Recorded in `plan/journal/counter-counts-the-wrong-packets.md`, which already held the same finding from another feature.
- An IPv4 route delete is matched on the scope as well as the destination. A rebuilt route deleted nothing, and the peer kept its metric. The withdraw now deletes the route the kernel holds, and the IPv6 proof would never have shown it.
- `docs/features/rfc-status.md`, `ai/RFC-REQUIREMENTS.md` and `rfc/requirements/` are generated on demand and gitignored, so the RFC row of the documentation checklist is satisfied by `rfc/short/rfc5082.md` alone.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A per-peer route metric for the transmit half | A global `net.ipv4.ip_default_ttl` sysctl | The owner decided per-peer. A global sysctl would change the TTL of every locally generated packet on the box, including the non-GTSM ones |
| Ten filter terms per peer: five related ICMP types, each for both sides of the quoted TCP header | One term per peer, dropping every low-TTL ICMP error from the peer address | One term would drop errors about ze's other flows to that peer, which no GTSM session claims, and RFC5082-3-4 forbids dropping them. The five types are the four the TCP stack acts on plus the redirect, because acting on a spoofed redirect diverts the session's packets, which is the attack the TTL check exists to stop |
| The floor is the peer's MinTTL | A hard-wired 255 | The main packet path already uses the derived floor, so a multi-hop `ttl max N` keeps both paths consistent |
| A new component the reactor calls | A plugin subscribing to a BGP event | The state is derived from peer config the reactor already holds, and an event hop would add a second declaration of the peer set |

## Known Limitations

The three items an earlier session left outstanding, the `.ci` functional
test, the FRR interop scenario and the doctor check, were written on 2026-09-14.
The Integration and Test Plan rows above name the artifacts and their red and
green evidence.

- The drop policy for a Dangerous related message is not operator-configurable. RFC 5082 Section 3 expects the policy to be configurable; ze applies the same answer it already applies to a Dangerous main packet, which is to drop. A configurable policy is separate work and is not part of this spec.
- The filter covers BGP GTSM sessions. BFD and VRRP GTSM sessions carry no TCP quoted header, so their related messages are not claimed by these terms.

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
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- `internal/component/gtsm`: `SetPeers` reconciles the kernel to the GTSM peer set; `installHopLimitRoute` / `withdrawHopLimitRoute` (`route_linux.go`) carry RTAX_HOPLIMIT 255 on a host route to each peer; `filterTables` / `peerTerms` (`gtsm.go`) publish the `ze_gtsm` inet input table through `firewall.RegisterTables` + `firewall.ApplyAll`, ten drop terms per IPv4 peer.
- Two daemon-only firewall matches, `MatchIPv4TTLBelow` and `MatchICMPErrorQuotedTCPPort` (`internal/component/firewall/model.go`, `validate.go`), lowered under an nfproto guard by `lowerIPv4TTLBelowMatch` and `lowerICMPErrorQuotedTCPPortMatch` (`internal/plugins/firewall/nft/lower_linux.go`).
- Reactor wiring: `Reactor.gtsmPeers` derives the set from `PeerSettings.OutTTL` / `MinTTL`, `publishGTSMKernelState` calls `gtsm.SetPeers` at start (`reactor.go`) and at every peer reconcile (`reactor_api.go`); `gtsmPeersFromResolvedTree` feeds `gtsm.SetConfigPeers` for the offline doctor mode.
- Doctor check `gtsm-kernel-state` (`checkKernelState`, `doctor.go`), code `doctor-gtsm-kernel-state` (`internal/core/diagnostic/codes.go`), row in `docs/guide/health-checks.md`.
- Proofs: eight tagged RFC5082-3-2 units in `gtsm_rfc5082_linux_test.go` and `internal/core/network/ttl_gtsm_linux_test.go`; functional `test/firewall/gtsm-related-icmp.ci` with driver `netfilter_fixture_gtsm.go`; interop `test/interop/scenarios/gtsm-related-icmp-ttl/` with `checkGTSMRelatedICMPTTL`.

### Bugs Found/Fixed
- Closure (this session): `SetPeers` wrote `current = wanted` after a route install failed, and the unchanged-set short-circuit then never retried it, contradicting `applyHopLimitRoutes`, `publishGTSMKernelState`, `missingStateDiagnostic` and the `codes.go` description. Fixed: `applyHopLimitRoutes` returns the joined errors, `SetPeers` records `routeMissing` and re-runs an unchanged set while it is set. Test: `TestGTSMRetriesARouteTheKernelRefused`.
- Closure: `newTestReactor` (`reactor_dynamic_test.go`) built a reactor with no `config`, and `gtsmPeers` reading `r.config.Port` from `reconcilePeersJournaled` panicked `TestReloadRepublishesDeliveryGraph`. The fixture now carries `&Config{}`, as `New` always does.
- Closure: `./le integration gtsm` did not name `./internal/component/gtsm/...`, so the CAP_NET_ADMIN proofs had no registered runner. Added (`internal/le/integration/gates.go`).
- Closure: `//nolint:embedlit` named an unknown linter in `netns_linux_test.go`; the veth literal now uses the promoted `Name` field the tree already uses (`internal/plugins/iface/netlink/manage_linux.go`).
- Implementation (2026-09-14, recorded in Design Insights): IPv4 route delete matched on scope; counters placed before matches.

### Documentation Updates
- `docs/DESIGN.md` (anchors `internal/component/gtsm/gtsm.go -- SetPeers, peerTerms`, `route_linux.go -- installHopLimitRoute`); `docs/guide/firewall.md` (`gtsm.go -- filterTables, peerTerms`); `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` (`gtsm.go -- filterTableName, filterTables`); `docs/guide/health-checks.md` (`doctor.go -- checkKernelState`); `docs/architecture/testing/interop.md` names the scenario; `rfc/short/rfc5082.md` Support coverage names the producers; `ai/CODE-TO-DOCS.md` maps the package. All landed in `479e9d76a9`, `a45ba3bf45`, `7929e0f69d`, `130c8c54d7`.
- `./le doc check links`: 2 broken references, both in other sessions' files (`internal/plugins/ospf/yang_vocabulary_test.go`, `plan/handover-rfc-conformance-c9bdcd62.md`); none in this spec's files. `./le docs-to-code index-check`: 2 stale anchors in `docs/architecture/api/text-format.md` and `docs/features/formatting.md`, neither this spec's.

### Deviations from Plan
- Files to Modify named `internal/component/bgp/reactor/reactor.go` for the derivation; it lives in `reactor/gtsm.go`, with one call each in `reactor.go` and `reactor_api.go`.
- The interop checker is `internal/le/interoplab/bgp/check_gtsm.go`, not `check_rfc.go` as the Test Plan row says.
- Documentation checklist row 1 (`docs/features.md`) was answered Yes; the feature is the existing `connection ttl` block, and the user-visible additions live on `docs/guide/firewall.md` and `docs/guide/health-checks.md`, so no `docs/features.md` row was owed or written. Row 12 named `docs/architecture/core-design.md` "where the component list is enumerated"; that page enumerates no component list, and the generated list in `CLAUDE.md` carries `gtsm`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-5: an IPv4 route delete was assumed to match on the destination alone | The delete matches on the scope as well; a link-scoped route survived a delete naming the default scope | `TestGTSMTransmittedICMPErrorWithoutTheRouteMetricIsNot255` read the route back | `withdrawHopLimitRoute` deletes the route the kernel holds (`findHopLimitRoute`) |
| approach | The receive proofs read one nft term's counter as "this rule matched" | `applyChain` puts the counter before the matches, so it counts packets that reached the rule | Every rule reported the same count for one packet | The proofs read `IcmpMsg InType3`; row in `plan/journal/counter-counts-the-wrong-packets.md` |
| approach | `SetPeers` recorded the reconcile as done when a route install failed, and every comment promised a retry that the short-circuit prevented | A config apply that changes nothing is what happens when the interface comes up, so the same set must retry | Closure review of `SetPeers` against the comment in `applyHopLimitRoutes` | `routeMissing` re-runs an unchanged set; row in `plan/journal/record-written-before-the-operation-succeeds.md` |
| escalation | The tagged receive proofs read `/proc/net/snmp` from a locked thread that unshared a namespace | `/proc/net` answers for the thread-group leader's namespace | Three proofs red under `sudo` on this host with host-sized counters | Fixed in the continuation: both reads go through `/proc/thread-self/net/`, all nine proofs green here (`scratch/gtsm-proofs-v2.log`); row in `plan/journal/counter-counts-the-wrong-packets.md` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Transmit IPv4 and IPv6: a related ICMP error leaves at TTL 255 | Done | `installHopLimitRoute`, `internal/component/gtsm/route_linux.go` | RTAX_HOPLIMIT on a host route; `Route.Hoplimit` |
| Receive IPv4: an off-link error claiming the session is dropped | Done | `peerTerms`, `internal/component/gtsm/gtsm.go`; `lowerICMPErrorQuotedTCPPortMatch`, `internal/plugins/firewall/nft/lower_linux.go` | Ten terms per peer, quoted BGP port required |
| Receive IPv6: kernel drop on IPV6_MINHOPCOUNT, owed a tagged test | Done | `TestGTSMMinHopCountDropsALowHopLimitICMPv6Error`, `internal/core/network/ttl_gtsm_linux_test.go` | Producer unchanged: `setIPMinTTL` |
| Each cell proven in both polarities with a discrimination record | Done | `rfc/discrimination/rfc5082.json`; `./le rfc check` names no rfc5082 row | See Pre-Commit Verification for what this host could observe |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `installHopLimitRoute` (`route_linux.go`); `test/firewall/gtsm-related-icmp.ci` expects `hoplimit 255`; `TestGTSMTransmittedICMPErrorCarriesTTL255` | |
| AC-2 | Done | `TestGTSMTransmittedICMPErrorCarriesTTL255`, `TestGTSMTransmittedICMPv6ErrorCarriesHopLimit255` read the captured error's TTL; interop `gtsm-related-icmp-ttl` reads it from FRR's kernel | |
| AC-3 | Done | `peerTerms` (`gtsm.go`); `TestGTSMDropsADangerousQuotedICMPError` | |
| AC-4 | Done | `MatchIPv4TTLBelow{Floor}` strictly below; `TestGTSMDeliversAQuotedICMPErrorAtTTL255` | |
| AC-5 | Done | every term carries `MatchICMPErrorQuotedTCPPort`; `TestGTSMEveryDropTermRequiresTheQuotedBGPPort`, `TestGTSMDeliversAnICMPErrorNoSessionClaims` | |
| AC-6 | Done | `setIPMinTTL` (`internal/core/network/ttl_linux.go`); `TestGTSMMinHopCountDropsALowHopLimitICMPv6Error` | |
| AC-7 | Done | `applyHopLimitRoutes` withdraws peers absent from `wanted`; `filterTables` returns nil with no terms; `TestGTSMWithdrawsEverythingWhenNoPeerEnablesIt`; `.ci` RELOADED assertions | |
| AC-8 | Done | `filterTables` returns nil, `SetPeers` publishes nothing; `TestGTSMPublishesNothingWhenItHasNothingToSay` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Portable unit tests (`TestGTSMPublishesTheICMPFilterTableForAPeer`, `...WithdrawsEverything...`, `...PublishesNothing...`, `...BuildsNoFilterTerms...`, `...HandsTheRouteHalfEveryPeer`, `TestGTSMRetriesARouteTheKernelRefused`) | Done | `internal/component/gtsm/gtsm_test.go` | pass in `gtsm-network-pkg.log` |
| `TestReactorPublishesGTSMPeersFromConfig`, `TestGTSMPeersFromResolvedTreeReadsTheConfigAlone` | Done | `internal/component/bgp/reactor/gtsm_test.go` | pass in `integration-gtsm-2.log` before the package timeout |
| Lowering tests | Done | `internal/plugins/firewall/nft/lower_linux_test.go` | committed `479e9d76a9` |
| Transmit tagged tests (4) | Done | `gtsm_rfc5082_linux_test.go` | pass under sudo on this host |
| Receive tagged tests IPv4 (3) and IPv6 (2) | Done, RED here | `gtsm_rfc5082_linux_test.go`, `ttl_gtsm_linux_test.go` | `internal/core/network` pair passes; the three gtsm-package receive proofs read the leader thread's namespace (Mistake Log) |
| Doctor tests | Done | `doctor_test.go`, `doctor_linux_test.go` | pass |
| `TestBespokeCheckerBranches/gtsm-related-icmp-ttl` | Done | `internal/le/interoplab/bgp/bgp_test.go` | committed `a45ba3bf45` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/gtsm/{gtsm,route_linux,route_other,doctor,register}.go` and their tests | Done | ls below |
| `internal/component/firewall/model.go`, `validate.go`; `internal/plugins/firewall/nft/lower_linux.go` | Done | `479e9d76a9` |
| `internal/component/bgp/reactor/gtsm.go`, `gtsm_test.go` | Changed | derivation in its own file, not `reactor.go` |
| `internal/core/rtproto/rtproto.go` | Done | `rtproto.GTSM` |
| `internal/test/fixture/netfilter_fixture_gtsm.go`, `test/firewall/gtsm-related-icmp.ci` | Done | |
| `test/interop/scenarios/gtsm-related-icmp-ttl/`, `internal/le/interoplab/bgp/check_gtsm.go` | Changed | checker file named `check_gtsm.go` |
| `rfc/short/rfc5082.md`, `docs/DESIGN.md`, `docs/architecture/firewall/table-ownership-and-shutdown-flush.md`, `docs/guide/health-checks.md` | Done | |

### Audit Summary
- **Total items:** 23
- **Done:** 20
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Transmit IPv4 and IPv6 produced by state ze installs | tagged kernel test + interop | `TestGTSMTransmittedICMPErrorCarriesTTL255` and `TestGTSMTransmittedICMPv6ErrorCarriesHopLimit255` capture the error off a veth and read TTL 255, negatives read the system default with the metric withdrawn (pass, `scratch/gtsm-network-pkg.log`); FRR 10.3.1 reads hop limit 255 through its own IPV6_MINHOPCOUNT check, scenario `gtsm-related-icmp-ttl` passed (`scratch/interop-gtsm.log`, `integration: 1 action(s) passed`; RED on the mutated image recorded 2026-09-14) |
| Receive IPv4 produced by state ze installs, Unknown never dropped | tagged kernel test | `TestGTSMDropsADangerousQuotedICMPError`, `TestGTSMDeliversAQuotedICMPErrorAtTTL255` and `TestGTSMDeliversAnICMPErrorNoSessionClaims` all pass here once the counters are read from the testbed's own namespace (`scratch/gtsm-proofs-v2.log`); records in `rfc/discrimination/rfc5082.json` |
| Receive IPv6 proven on the state ze installs | tagged kernel test | `TestGTSMMinHopCountDropsALowHopLimitICMPv6Error` and `TestGTSMWithoutMinHopCountDeliversTheSameICMPv6Error` (`internal/component/gtsm/gtsm_rfc5082_linux_test.go`) pass under `sudo` in `scratch/gtsm-proofs-v2.log` |
| Reachable from an operator's configuration | functional | `test/firewall/gtsm-related-icmp.ci` (green 2026-09-14 under `ZE_TEST_NETNS=1`, RED with the metric install removed); not re-run here, see Pre-Commit Verification |
| `./le rfc check` no longer names RFC5082-3-2 | gate | `scratch/rfc-check.log`: no rfc5082 row; the one red is `rfc4301` (another session) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | every in-scope item is implemented; two owner questions are in the closure report (configurable Dangerous policy, RFC 5082 Section 3; IPv4 receive association by outer source address) | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | see `./le spec session review record` output in the closure report |
| `review check` | clean |
| Rounds | 2: round 1 found the retry defect, the fixture panic, the gate population and the lint finding; round 2 over the fixes found nothing above NOTE |
| Reviewer lenses used | logic+wiring, security+edge-cases, RFC 5082 conformance, style pass over every changed Go file |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | A failed route install was recorded as reconciled and never retried while the peer set stayed the same; four comments promised the retry | `SetPeers`, `applyHopLimitRoutes` | `routeMissing`, joined errors, `TestGTSMRetriesARouteTheKernelRefused` |
| 2 | ISSUE | `reconcilePeersJournaled` now reaches `gtsmPeers`, which reads `r.config.Port`; the reactor test fixture carried no config and `TestReloadRepublishesDeliveryGraph` panicked | `newTestReactor`, `reactor_dynamic_test.go` | `config: &Config{}` |
| 3 | ISSUE | The CAP_NET_ADMIN proofs had no registered runner: `./le integration gtsm` did not name the package | `internal/le/integration/gates.go` | package added |
| 4 | ISSUE | `//nolint:embedlit` names no linter; `modernize` reported the literal | `netns_linux_test.go` | promoted-field literal |

NOTEs: the `.ci` driver's withdrawal poll reads any `nft` error as absence (`gtsmRelatedICMP`). Fixed in the continuation, both with a journal row: the receive proofs read their counters from `/proc/thread-self/net/` (the testbed's own namespace), and `waitForIPv6` treats an interrupted address dump as a retry rather than a failure.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/gtsm/` | Yes | `doctor.go doctor_linux_test.go doctor_test.go gtsm.go gtsm_rfc5082_linux_test.go gtsm_test.go netns_linux_test.go packet_linux_test.go register.go route_linux.go route_other.go` |
| `internal/component/bgp/reactor/gtsm.go`, `gtsm_test.go` | Yes | 4068 and 5978 bytes |
| `test/firewall/gtsm-related-icmp.ci`, `internal/test/fixture/netfilter_fixture_gtsm.go` | Yes | 69 and 97 lines |
| `test/interop/scenarios/gtsm-related-icmp-ttl/{frr.conf,ze.conf}`, `internal/le/interoplab/bgp/check_gtsm.go` | Yes | 731, 1246 bytes; 136 lines |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2 | host route carries hop limit 255 and the captured error leaves at 255 | the four transmit tests pass under `sudo` (`scratch/gtsm-network-pkg.log`); interop passed |
| AC-3 | a Dangerous quoted error is dropped | `TestGTSMDropsADangerousQuotedICMPError` pass; `peerTerms` builds `MatchIPv4TTLBelow{Floor: p.Floor}` |
| AC-4, AC-5 | at-floor and unclaimed errors delivered | `TestGTSMEveryDropTermRequiresTheQuotedBGPPort` pass; `TestGTSMDeliversAQuotedICMPErrorAtTTL255` and `TestGTSMDeliversAnICMPErrorNoSessionClaims` pass under `sudo` (`scratch/gtsm-proofs-v2.log`, 27 of 27) |
| AC-6 | IPv6 kernel drop | `internal/core/network` ok, 5.456s |
| AC-7, AC-8 | withdrawal, and nothing for no peer | `TestGTSMWithdrawsEverythingWhenNoPeerEnablesIt`, `TestGTSMPublishesNothingWhenItHasNothingToSay` pass |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `connection ttl max N` → `Reactor.gtsmPeers` → `gtsm.SetPeers` | `test/firewall/gtsm-related-icmp.ci` reads `nft list table inet ze_gtsm` and `ip route show ... 192.0.2.2/32` before and after a reload | file read; not re-run here: `./le functional firewall` under `sudo ZE_TEST_NETNS=1` fails every test with `fork/exec .../bin/ze: permission denied` (journal row) |
| `gtsm.SetPeers` → `filterTables` / withdraw | unit seams | `TestGTSMPublishesTheICMPFilterTableForAPeer`, `TestGTSMWithdrawsEverythingWhenNoPeerEnablesIt` pass |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | transmit proofs read 255 with the metric and the default without |
| A-2 | confirmed | the 0x45 compare precedes the port read in `lowerICMPErrorQuotedTCPPortMatch` |
| A-3 | confirmed | `routeHopLimit` reads the metric back off the kernel |
| A-4 | confirmed | `TestReactorPublishesAPeersOwnListenPort`; both quoted sides carry a term |
| A-5 | broken | Mistake Log row 1; `withdrawHopLimitRoute` deletes the route the kernel holds |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/DESIGN.md` GTSM paragraph | `SetPeers`, `peerTerms`, `installHopLimitRoute` | Yes |
| `docs/guide/firewall.md` `ze_gtsm` paragraph | `filterTables`, `peerTerms` | Yes |
| `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` owner row | `filterTableName` | Yes |
| `docs/guide/health-checks.md` doctor row | `checkKernelState` | Yes |
| `rfc/short/rfc5082.md` Support coverage | `Reactor.gtsmPeers`, `internal/component/gtsm` | Yes |
| No new leaf, command, RPC, plugin, metric | `grep -rn gtsm internal/component/config/yang` finds no leaf; the table is read through the existing `show firewall ruleset` | Yes |

## Core Insight

A reconcile that records "done" before the kernel agrees turns every later reconcile into a no-op, and the comments that promise a retry are then the only place the retry exists.
