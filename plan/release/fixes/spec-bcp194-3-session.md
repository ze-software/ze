# Spec: BCP 194 child 3 -- speaker and session hardening

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/templates/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | ready |
| Priority | P0 |
| Release reason | Repair existing GTSM listener protection and accepted CoPP behavior that fails open. |
| Scope | protocol |
| Depends | spec-bcp194-0-umbrella |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-27 |

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     The file's DIRECTORY carries the release bucket: plan/immediate/ for a defect
     an operator meets, plan/pre-release/ for work the release cannot go out
     without, plan/ for everything else (plan/README.md). -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

This spec is child 3 of the BCP 194 spec set. The umbrella is
`plan/release/validation/spec-bcp194-0-umbrella.md`.

Goal: harden the BGP speaker and its sessions per RFC 7454 S4 and S5. Two gaps
are verified.

Gap 1, GTSM before the connection is accepted. `RealListenerFactory` in
`internal/core/network/network.go` carries `MD5Peers` and an outgoing
`ListenTTL`. It sets no receive floor on the listen socket. The receive-side
GTSM check therefore runs only on the connection Ze already accepted, in
`tuneTCPConnectionForSettings`
(`internal/component/bgp/reactor/session_connection.go`). A SYN, and the rest
of the handshake, from a GTSM peer's address at a TTL below the peer's floor
still reaches the TCP stack and costs a request socket and a SYN-ACK. GTSM
stops protecting the control plane from off-path SYN packets, which is the
reason S5.2 recommends it.

Gap 2, the control plane ACL. `translatePolicy` in
`internal/plugins/copp/translate.go` emits accept terms for hand-typed
`trusted-source` entries, and one rate-limit term. It emits no final drop term,
so the ACL that S4 describes cannot be expressed, and it never learns the
configured BGP neighbors. The `over-limit-policy` leaf also defaults to
`accept`. An operator who turns CoPP on gets a count of the over-limit packets
and no drop.

The fix, in one paragraph each (design and its sources: Key Design Decisions):

- Gap 1: the `ze_gtsm` nftables input table that `internal/component/gtsm`
  already owns gains, for each GTSM peer, a drop term for TCP from the peer's
  address to the peer's Ze listen port whose TTL or hop limit is below the
  peer's floor. The term classifies by source, so a shared listener serving
  non-GTSM peers, multihop peers and unconfigured sources drops only the
  Dangerous packets RFC 5082 S3 names. The per-connection socket floor stays.
- Gap 2: with `control-plane-protection bgp` present, the `ze_copp` chain
  accepts new TCP to a protected port only from a configured BGP neighbor (a
  static peer's remote address or a dynamic group's range) or a
  `trusted-source` prefix, and drops the rest. `rate` becomes optional and
  limits the accepted neighbors only; `over-limit-policy` defaults to `drop`.

Out of scope, deliberately: TCP-AO (RFC 5925, S5.1) is absent from Ze. It is a
large separate feature and needs its own spec.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/traffic/cp-survival-2-copp-port179.md` - the CoPP design
  → Decision: CoPP is a system plugin that builds one `ze_copp` table through `firewall.RegisterTables` and `ApplyAll`
  → Constraint: the input chain policy stays `accept` under every setting, so SSH, ICMP and every service the table never names stay reachable
- [ ] `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` - several owners' tables at the input hook
  → Constraint: a drop in any base chain at a hook is final, an accept is not, so a `ze_gtsm` or `ze_copp` drop stands wherever the operator's own chains sit
- [ ] `ai/doors/system-plugins.md` - CoPP's door
  → Constraint: a plugin reaches the engine through the SDK, the bus or a command, never a Go call into a component's state. CoPP learns the neighbors from the `bgp` config section the SDK delivers
- [ ] `ai/doors/bgp-engine.md` - the reactor publishes per-peer GTSM state (`internal/component/bgp/reactor/gtsm.go`)
  → Decision: per-peer kernel state is published once per peer reconcile through `setGTSMPeers`, and the gtsm component owns the kernel
- [ ] `docs/architecture/core-design.md` - declared by `internal/component/firewall/model.go`, `validate.go`, `internal/plugins/firewall/nft/lower_linux.go` and `internal/component/bgp/reactor/config.go`
  → Constraint: Ze's datapath work runs through pluggable, registered backends (the interface section names `RegisterBackend`/`LoadBackend`); the IPv6 hop-limit match is added to the firewall model and lowered by the nft backend, never spelled as nft text by gtsm
- [ ] `docs/architecture/doctor-and-health-checks.md` - declared by `internal/component/gtsm/doctor.go`
  → Constraint: "Discovery beats enumeration": a check derives what it expects from the producer, with no second list, so AC-15's checks compare the kernel against `filterTables` and `translatePolicy` output, never a hand-written term list
- [ ] `ai/rules/platform-linux.md` - Linux socket options and nftables are in play
  → Constraint: the proof runs against Ze's own kernel, in a network namespace, on the agent's own architecture

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc7454.txt` S4, S5.1, S5.2 - the obligations this spec meets (the summary `rfc/short/rfc7454.md` is non-normative per `rfc/not-enrolled.txt`; the RFC text governs)
  → Constraint: S4 "This protection SHOULD be achieved by an Access Control List (ACL) that would discard all packets directed to TCP port 179 on the local device and sourced from an address not known or permitted to become a BGP neighbor."
  → Constraint: S4 "In addition to strict filtering, rate-limiting MAY be configured for accepted BGP traffic."
  → Constraint: S5.2 "Network administrators SHOULD implement TTL security on directly connected BGP peerings."
- [ ] `rfc/short/rfc5082.md` and `rfc/full/rfc5082.txt` S2.2, S3 - enrolled, four MUSTs
  → Constraint: S3 "MUST NOT drop (as part of GTSM processing) packets classified as Trusted or Unknown."
  → Constraint: S3 "MAY drop packets classified as Dangerous."
  → Constraint: S2.2 "For maximal protection, ingress filtering should be applied before the packet goes through the scarce resource."

**Key insights:** (minimal context to resume after compaction)
- Linux checks `min_ttl` at the `process:` label of `tcp_v4_rcv` against whatever socket the lookup found, a LISTEN socket included, so a floor on the listener is one value for every source. A shared listener cannot classify per peer; nftables can.
- FRR and BIRD both put TTL 255 on the listener's transmit side and the floor on the accepted or connected socket only. Ze already matches them; going further is allowed because the extra drop touches only Dangerous packets.
- RFC 7454 is not enrolled (non-normative, owner decision recorded in `rfc/not-enrolled.txt`), so this spec adds no `rfc/short/rfc7454.md` tags. RFC 5082 is enrolled: new GTSM tests carry `RFC requirement:` tags and discrimination records.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/core/network/network.go` - `RealListenerFactory.Listen` sets TCP MD5 per peer and the outgoing `ListenTTL` through `setListenIPTTL`; no receive floor
- [ ] `internal/core/network/ttl_linux.go` - `setListenIPTTL` sets IP_TTL and IPV6_UNICAST_HOPS on the listener; `setIPMinTTL` sets IP_MINTTL or IPV6_MINHOPCOUNT on one connection by the peer's family
- [ ] `internal/component/bgp/reactor/session_connection.go` - `connectionEstablished` calls `tuneTCPConnectionForSettings` on each connected or accepted socket: TCP_NODELAY, DSCP CS6, TTL and the floor
- [ ] `internal/component/bgp/reactor/gtsm.go` - `gtsmPeers` and `gtsmPeerOf` derive one `gtsm.Peer` (Addr, Port from `listenPortFor`, HopLimit, Floor) per peer with a `ttl` block; `publishGTSMKernelState` hands the set to `gtsm.SetPeers`; dynamic group members are not in the set
- [ ] `internal/component/gtsm/gtsm.go` - `filterTables` builds the `ze_gtsm` table (input hook, priority 0, policy accept); `peerTerms` emits ten IPv4 terms per peer that drop a related ICMP error quoting the peer's TCP session below its floor; no term reads the main TCP packets
- [ ] `internal/component/firewall/model.go` - `MatchIPv4TTLBelow` is daemon-only and IPv4-only; no IPv6 hop-limit match exists; `Table` carries `Sets`, `MatchInSet` and `Counter` exist
- [ ] `internal/plugins/copp/translate.go` - `translatePolicy` emits `established` accept, one accept per `trusted-source`, one `rate-limit` term on new TCP to the protected ports; no drop term
- [ ] `internal/plugins/copp/yang/ze-copp-conf.yang` - `rate` mandatory, `burst`, `protected-port` (179 when omitted), `trusted-source` (prefix list), `over-limit-policy` default `accept`
- [ ] `internal/plugins/copp/register.go` - `ConfigRoots` holds only `control-plane-protection`; verify stages, apply installs
- [ ] `internal/component/bgp/reactor/config.go` and `internal/component/bgp/config/resolve.go` - a static peer must carry `connection > remote > ip`; a dynamic group carries `connection > remote > range` with `ip dynamic`

**Behavior to preserve:** (unless the user explicitly said to change it)
- Per-connection IP_MINTTL / IPV6_MINHOPCOUNT and the listener's outgoing TTL 255, with their existing tests (`TestRealListenerFactoryListenTTLPassesPeerMinTTL` and the `rfc5082_*` tests)
- The `ze_gtsm` related-ICMP terms, the RTAX_HOPLIMIT host routes, and the table's withdrawal when the last GTSM peer goes
- `ze_copp` chain policy `accept`; the `established` term first; `trusted-source` accepted ahead of any rate limit; `protected-port` default 179; CoPP stays opt-in, so no deployment gains an implicit deny on upgrade (umbrella, "Behavior to preserve")
- The `over-limit-policy accept` spelling still counts and admits

**Behavior to change:** (only what the user asked for)
- A GTSM peer's TCP to its Ze listen port below the floor is dropped at the input hook, both families
- CoPP drops new TCP to a protected port from a source that is no configured neighbor and no `trusted-source`
- `rate` becomes optional; the limit applies to accepted neighbors only; `over-limit-policy` defaults to `drop`

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Configuration: a BGP peer's `connection { ttl { max N } }` block (GTSM), and the `control-plane-protection { bgp { ... } }` container (CoPP), entered through the config file, the CLI editor or a reload
- Wire: an IPv4 or IPv6 TCP segment addressed to Ze's BGP listen port, judged by the kernel's nftables input hook before the TCP stack sees it

### Transformation Path
1. GTSM: config apply → reactor peer reconcile → `gtsmPeers` derives `gtsm.Peer` per GTSM peer → `gtsm.SetPeers` → `filterTables` adds the listen-port terms beside the ICMP terms → `firewall.RegisterTables` + `ApplyAll` → nft backend lowers `MatchIPv4TTLBelow` and the new IPv6 hop-limit match → the kernel drops a Dangerous segment at the input hook
2. CoPP: config verify receives the `control-plane-protection` and `bgp` sections → the neighbor-source reader turns the `bgp` section into a prefix list (static peer addresses as host prefixes, dynamic group ranges) → `translatePolicy` builds the set, the accept, rate and drop terms → apply registers and applies the `ze_copp` table → the kernel drops new and stray TCP to a protected port from any other source
3. A reload that changes the peers re-runs both paths: the reactor reconcile republishes the GTSM set, and the `bgp` section's change reaches CoPP's verify and apply

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor ↔ gtsm component | `setGTSMPeers([]gtsm.Peer)`, existing in-process seam | Yes, `internal/component/bgp/reactor/gtsm.go` |
| Engine ↔ CoPP plugin | SDK config sections (`ConfigRoots`), JSON of the `bgp` and `control-plane-protection` roots | No (A-2) |
| Component/plugin ↔ kernel | `firewall.RegisterTables` + `ApplyAll`, nft backend lowering | Yes, both owners already use it |

### Integration Points
- `gtsm.Peer` and `peerTerms` - gain the listen-port drop terms; `Peer.Port` already carries `listenPortFor`
- `firewall.MatchIPv4TTLBelow` - reused; a sibling daemon-only IPv6 hop-limit match is added beside it, with its nft lowering and `validateMatch` case
- `firewall.Set` + `MatchInSet` - the CoPP neighbor set, one per address family
- `copp` `registry.Registration.ConfigRoots` - gains `bgp`
- The one reader of `connection > remote > ip` and `connection > remote > range` - shared by the BGP config parse and CoPP (A-3), never a second parser

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | GTSM terms ride the existing reactor → gtsm → firewall path; CoPP stays on `RegisterTables` |
| No unintended coupling (components stay isolated) | Yes, subject to A-3 | CoPP reads neighbors from its SDK config section, not from reactor state |
| No duplicated functionality (extends existing, does not recreate) | Yes | extends `ze_gtsm` and `ze_copp`; no third table, no listener-wide floor beside the per-connection one |
| Zero-copy preserved where applicable (refs, not copies) | N-A | config-time table building, no wire encoding |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | one new match type in the firewall model's existing match vocabulary; CoPP declares its roots in its own registration |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Linux applies a socket's `min_ttl` at the `process:` label of `tcp_v4_rcv` to whatever socket the lookup returned, a LISTEN socket included, so a listener floor is one value for all sources | Linux `net/ipv4/tcp_ipv4.c` `tcp_v4_rcv`, `if (unlikely(iph->ttl < READ_ONCE(inet_sk(sk)->min_ttl)))` after `process:`; same shape in `net/ipv6/tcp_ipv6.c` with `min_hopcount` (kernel.org master, read 2026-10-27) | A per-source listener floor would exist and the nftables term would be redundant | source read | validated |
| A-2 | The SDK delivers the `bgp` root to a non-BGP plugin that lists it in `ConfigRoots`, on boot and on every reload that changes it | `internal/component/bgp/plugins/rs/register.go` lists `bgp`; `rs/config_tx.go` reads `section.Root == configRootBGP` | CoPP cannot learn neighbors from config; it would need an engine command instead | Phase 1 wiring test `TestCoppDropTermWiring` driven through the plugin's verify and apply with both sections | unvalidated |
| A-3 | The neighbor addresses can be read from the raw `bgp` section by one function that the BGP config parse also calls, placed where CoPP may import it without pulling in the reactor | `reactor/config.go` reads `connection > remote > ip`; `config/resolve.go` reads `range`; `./le arch tier check` judges placement | A second reader would duplicate the leaf paths (`ai/rules/principles.md`, declare once) | Phase 1: place the function, move both existing reads onto it, run `./le arch tier check` scoped | unvalidated |
| A-4 | nftables `ip6 hoplimit` is lowerable by the nft backend the same way `ip ttl` is | `internal/plugins/firewall/nft/lower_linux.go` lowers `MatchIPv4TTLBelow` as a network-header payload compare | IPv6 GTSM listener terms cannot be built | Phase 2 Linux lowering test reading the installed rule back | unvalidated |
| A-5 | A peer with GTSM always has `Port` and `Floor` non-zero in `gtsm.Peer` | `gtsmPeerOf` copies `MinTTL` and `listenPortFor` | A term with port 0 or floor 0 would match nothing; `peerTerms` already skips such peers | existing `peerTerms` guard plus a unit row | validated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The listen-port term drops a Trusted or Unknown packet, which RFC 5082 S3 forbids | AC-4 or AC-5 red | the term matches source address AND destination port AND below-floor; boundary tests at floor and floor-1; Unknown-source test |
| R-2 | CoPP drops a real peer's SYN after a reload adds the peer, because the `bgp` change does not re-run CoPP | AC-9 red | `bgp` in `ConfigRoots`; reload test adds a peer and asserts the set |
| R-3 | A dynamic group member is dropped by CoPP | AC-8 red | the group's ranges are in the neighbor set |
| R-4 | An operator who wrote `rate` and relied on the `accept` default now drops over-limit connections from neighbors | `copp-bgp.ci` diff | pre-release: no deployment consumes `main`; the YANG description and `docs/guide/firewall.md` name the new default; the spelling `accept` keeps the old behaviour |
| R-5 | The rate limiter is starved by unknown sources | a flood from an unknown source drops neighbor SYNs | the drop of unknown sources sits before the rate term, so only accepted neighbors draw on the bucket (RFC 7454 S4, "rate-limiting MAY be configured for accepted BGP traffic") |
| R-6 | Firewall backend absent or failing: the drop terms are not installed and the system fails open silently | `ze doctor` | the existing `copp-input-chain` and gtsm kernel-state doctor checks are extended to name the missing term; apply errors stay surfaced (`publishGTSMKernelState` warns, CoPP apply refuses) |
| R-7 | Concurrent reconciles of the gtsm table | race detector | `SetPeers` already serializes on its package mutex; no new state |
| R-8 | Changing tagged RFC 5082 tests or `copp-*.ci` expectations reads as a weakening | review | every changed expectation goes to `ze-test-judge`; new tags get `./le rfc record` |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Live BGP sessions fail to establish: a wrong GTSM term drops a GTSM peer's handshake, a wrong CoPP set drops a neighbor's SYN. Established sessions are untouched (the `established` accept, and GTSM terms only drop below-floor packets) |
| How is it reverted? | Single commit revert per phase; no config migration (the YANG change only relaxes `rate` and changes a default) |
| Who else touches this path? | `plan/release/fixes/spec-firewall-existing-family-and-owner-correctness.md` (table owners, names `copp` and `gtsm`); `plan/post-release/features/spec-dataplane-seams-5-copp-non-tcp.md` (CoPP beyond TCP) |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| BGP peer with `ttl { max 1; }`, config apply | → | `Reactor.publishGTSMKernelState` → `gtsm.SetPeers` → `filterTables` listen-port terms | `TestListenerMinTTLWiring` (`internal/component/bgp/reactor/gtsm_test.go`): a config with one IPv4 and one IPv6 GTSM peer yields, through the seam `setGTSMPeers`, tables whose terms drop the peer's TCP to its listen port below the floor |
| `control-plane-protection bgp` plus BGP peers, config verify and apply | → | copp verify/apply → neighbor-source reader → `translatePolicy` drop term | `TestCoppDropTermWiring` (`internal/plugins/copp/register_test.go`): verify and apply with both SDK sections produce a `ze_copp` table whose neighbor set holds the static peer and the dynamic range, and whose last term drops TCP to the protected port |
| Same, on a real kernel | → | nft backend lowering of both | `test/firewall/gtsm-listener-floor.ci` and `test/firewall/copp-unknown-source-drop.ci` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | IPv4 GTSM peer (`ttl max 1`, floor 255); a SYN from the peer's address to Ze's listen port at TTL 254 | Ze sends no SYN-ACK and holds no request socket; the drop is counted in the `ze_gtsm` table |
| AC-2 | IPv6 GTSM peer (floor 255); a SYN from the peer's address at hop limit 254 | same as AC-1 for IPv6 |
| AC-3 | The same peers connecting at TTL / hop limit 255 | the session reaches Established |
| AC-4 | A SYN at TTL 1 to the listen port from a configured peer without GTSM, and from an address that is no peer | not dropped by `ze_gtsm` (RFC 5082 S3 Unknown); TCP answers as before |
| AC-5 | GTSM peer `ttl max 3` (floor 253) | TTL 253 accepted, TTL 252 dropped; the same at hop limits for IPv6 |
| AC-6 | A reload removes a peer's `ttl` block, then the last GTSM peer | its listen-port terms leave the table with its ICMP terms; the table is withdrawn with the last peer |
| AC-7 | `control-plane-protection bgp` present; new TCP to a protected port from a source that is no static peer address, no dynamic group range and no `trusted-source` | dropped and counted; Ze sends no SYN-ACK |
| AC-8 | Same container; new TCP from a static peer, from an address in a dynamic group range, and from a `trusted-source` prefix | accepted; the session reaches Established |
| AC-9 | A reload adds a peer (no CoPP change), then removes it | the new peer's SYN is accepted after the first reload and dropped after the second |
| AC-10 | `rate` set, `over-limit-policy` omitted, neighbor new-connection rate above the limit | the over-limit connections are dropped (default `drop`); with `over-limit-policy accept` they are admitted and counted |
| AC-11 | `bgp` container with no `rate` | config accepted; the table carries the ACL and no limiter |
| AC-12 | CoPP active; an established session, SSH to the router, ICMP echo | unaffected; the chain policy stays `accept` |
| AC-13 | `trusted-source` prefix with a `rate` configured | its connections bypass the limiter, as today |
| AC-14 | Interop: FRR peer with `ttl-security hops 1` under CoPP and GTSM; a probe from FRR's address at TTL 254, and a probe from an unconfigured address in FRR's container | the FRR session reaches and stays Established; neither probe receives a SYN-ACK |
| AC-15 | `ze doctor` with CoPP configured but its drop term absent from the kernel, and with a GTSM peer whose listen-port term is absent | each check names the missing term and the command that restores it |
| AC-16 | A dynamic group whose template carries `ttl { max 1; }` (`ParseDynamicGroupTemplate` shares the peer parser); a SYN from an address inside the group's range at TTL 254 | dropped as in AC-1, by a term whose source is the range; at TTL 255 the member session reaches Established |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator enables GTSM on an IXP peer; an off-path host spoofs the peer's address | SYN at TTL < 255 → nft input hook → `ze_gtsm` listen-port term drops it → no SYN-ACK | `test/firewall/gtsm-listener-floor.ci`, interop `bgp-session-protection-frr` |
| 2 | Operator turns on CoPP; a scanner floods port 179 | SYN from unknown source → `ze_copp` established/neighbor/rate terms miss → drop term | `test/firewall/copp-unknown-source-drop.ci`, interop `bgp-session-protection-frr` |
| 3 | Operator adds a peer while CoPP is on | reload → `bgp` section → CoPP verify/apply → set gains the address → SYN accepted | `TestCoppReloadAddsNeighbor`, `test/firewall/copp-unknown-source-drop.ci` (reload step) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestListenerMinTTLWiring` | `internal/component/bgp/reactor/gtsm_test.go` | wiring row 1; AC-1, AC-2 at table level | |
| `TestPeerTermsListenPortIPv4` | `internal/component/gtsm/gtsm_test.go` | one term per IPv4 peer: source = peer, TCP destination port = `Peer.Port`, TTL below `Floor`, drop + counter; AC-1, AC-5 | |
| `TestPeerTermsListenPortIPv6` | `internal/component/gtsm/gtsm_test.go` | the IPv6 term with the hop-limit match; AC-2 | |
| `TestListenTermsDynamicRange` | `internal/component/gtsm/gtsm_test.go` and `internal/component/bgp/reactor/gtsm_test.go` | a GTSM dynamic group's ranges yield listen-port terms with the range as source and the template's floor; AC-16 | |
| `TestPeerTermsSkipsUnsetFloorOrPort` | `internal/component/gtsm/gtsm_test.go` | a peer with floor 0 or port 0 gets no term (A-5) | |
| `TestFilterTablesWithdrawsWithLastPeer` | `internal/component/gtsm/gtsm_test.go` | existing withdrawal extended to the new terms; AC-6 | |
| `TestValidateMatchIPv6HopLimitBelow` | `internal/component/firewall/validate_test.go` | floor 0 refused, 1..255 accepted | |
| `TestLowerIPv6HopLimitBelow` | `internal/plugins/firewall/nft/lower_linux_test.go` | lowering reads IPv6 network-header offset 7 with a less-than compare; A-4 | |
| `TestNeighborSourcesFromBGPSection` | beside the shared reader chosen in Phase 1 (A-3) | static IPv4, IPv6 and link-local peers become host prefixes; dynamic group ranges pass through; a section with no peers yields an empty list, not an error | |
| `TestCoppDropTermWiring` | `internal/plugins/copp/register_test.go` | wiring row 2; AC-7, AC-8 at table level | |
| `TestTranslatePolicyTermOrder` | `internal/plugins/copp/translate_test.go` | order: established accept, trusted-source accept, rate term on neighbor set, neighbor accept, unknown-source drop; chain policy accept; AC-12, AC-13, R-5 | |
| `TestTranslatePolicyNoRate` | `internal/plugins/copp/translate_test.go` | no limiter term when `rate` is absent; AC-11 | |
| `TestParseCoppConfigOverLimitDefaultDrop` | `internal/plugins/copp/config_test.go` | omitted `over-limit-policy` parses as drop; explicit `accept` stays accept; AC-10 | |
| `TestCoppReloadAddsNeighbor` | `internal/plugins/copp/reload_test.go` | a `bgp` section change alone re-stages and re-applies the set; AC-9 | |
| `TestCoppDoctorNamesMissingDropTerm` | `internal/plugins/copp/doctor_test.go` | AC-15 for CoPP | |
| `TestGTSMDoctorNamesMissingListenTerm` | `internal/component/gtsm/doctor_test.go` | AC-15 for GTSM | |
| `TestGTSMListenerSYNBelowFloorDropped` | `internal/component/gtsm/rfc5082_gtsm_linux_test.go` | namespaces + Ze's kernel: SYN at TTL 254 from the peer address gets no SYN-ACK and the term counter rises; at 255 the handshake completes; `RFC requirement:` tag on RFC 5082 S3 MAY-drop / MUST-NOT-drop rows, discrimination record by `./le rfc record`; AC-1, AC-3 | |
| `TestGTSMListenerSYNBelowFloorDroppedIPv6` | `internal/component/gtsm/rfc5082_gtsm_linux_test.go` | AC-2, AC-3 for IPv6 | |
| `TestGTSMListenerUnknownNotDropped` | `internal/component/gtsm/rfc5082_gtsm_linux_test.go` | SYN at TTL 1 from a non-GTSM peer and from an unconfigured source is answered by TCP; tagged RFC 5082 S3 MUST NOT drop Unknown; AC-4 | |
| `TestCoppUnknownSourceDroppedLinux` | `internal/plugins/copp/copp_integration_linux_test.go` | namespaces: unknown source gets no SYN-ACK, neighbor and dynamic-range sources complete the handshake, SSH-port and ICMP unaffected; AC-7, AC-8, AC-12 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| GTSM floor vs received TTL (`ttl max N`, floor 256-N) | N 1-255 | TTL = floor accepted (N=1: 255; N=3: 253) | TTL = floor-1 dropped (254; 252) | N/A (TTL is a byte) |
| `MatchIPv6HopLimitBelow.Floor` | 1-255 | 1 and 255 | 0 refused by `validateMatch` | N/A |
| `protected-port` | 1-65535 | 1 and 65535 | 0 refused (existing YANG range) | 65536 refused |
| `rate` count | existing `rate-spec` pattern | unchanged | unchanged | unchanged |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `gtsm-listener-floor` | `test/firewall/gtsm-listener-floor.ci` | Operator configures a GTSM peer; `nft list table inet ze_gtsm` shows the listen-port drop term for both families; removing `ttl` removes it (AC-1, AC-2, AC-6) | |
| `copp-unknown-source-drop` | `test/firewall/copp-unknown-source-drop.ci` | Operator enables CoPP with two peers and a dynamic group; the table holds the neighbor sets and the final drop; a reload adding a peer extends the set (AC-7, AC-8, AC-9, AC-11) | |
| `copp-bgp` | `test/firewall/copp-bgp.ci` (existing) | expectation updated for the neighbor set, the drop term and the `drop` default; judged by `ze-test-judge` (AC-10) | |
| `copp-trusted` | `test/firewall/copp-trusted.ci` (existing) | trusted-source still precedes the limiter (AC-13); any expectation change judged by `ze-test-judge` | |
| `copp-over-limit-drop` | `test/firewall/copp-over-limit-drop.ci` (existing) | unchanged behaviour with the explicit spelling; a new step asserts the omitted leaf behaves as `drop` (AC-10) | |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-session-protection-frr` | `test/interop/scenarios/bgp-session-protection-frr/` | FRR (latest stable, `neighbor ... ttl-security hops 1`) | Ze configured with GTSM and CoPP establishes with FRR and keeps the session (AC-3, AC-8, AC-12); a probe from FRR's address at TTL 254 and a probe from a second, unconfigured address in FRR's container receive no SYN-ACK, read from Ze's `ze_gtsm` and `ze_copp` term counters and from the probe's own capture (AC-1, AC-7, AC-14). Red phase: rebuild Ze's image with the listen-port term and the drop term reverted; both probes must then receive a SYN-ACK | |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/component/gtsm/gtsm.go` - listen-port terms in `peerTerms` (both families); package and `peerTerms` comments that say IPv4-only and "no term reads the outer source" are rewritten for the new term (`ai/rules/stale-comments.md`)
- `internal/component/gtsm/doctor.go` - the kernel-state check names a missing listen-port term (AC-15)
- `internal/component/firewall/model.go`, `internal/component/firewall/validate.go` - daemon-only IPv6 hop-limit-below match; `MatchIPv4TTLBelow` comment no longer says no caller needs IPv6
- `internal/plugins/firewall/nft/lower_linux.go` - lowering of the IPv6 match
- `internal/component/bgp/reactor/gtsm.go` - `gtsmPeers` and `gtsmPeersFromResolvedTree` also publish each GTSM dynamic group's ranges with the template's floor and listen port (AC-16); file comment updated
- `internal/plugins/copp/register.go` - `ConfigRoots` gains `bgp`; verify and apply read both sections
- `internal/plugins/copp/config.go`, `internal/plugins/copp/model.go` - `rate` optional, `over-limit-policy` default drop, the neighbor prefix list in `coppPolicy`
- `internal/plugins/copp/translate.go` - neighbor sets, term order, final drop
- `internal/plugins/copp/doctor.go` - names a missing drop term (AC-15)
- `internal/plugins/copp/yang/ze-copp-conf.yang` - `rate` loses `mandatory`; `over-limit-policy` default `drop`; descriptions of the container, `rate`, `trusted-source` and `over-limit-policy` rewritten for the ACL
- `internal/component/bgp/reactor/config.go`, `internal/component/bgp/config/resolve.go` - their reads of `remote > ip` and `remote > range` move onto the shared reader (A-3)
- `test/firewall/copp-bgp.ci`, `test/firewall/copp-trusted.ci`, `test/firewall/copp-over-limit-drop.ci` - updated expectations (judge-approved)
- `rfc/short/rfc5082.md`, `rfc/requirements/rfc5082.md`, `rfc/enrolled.txt` (rfc5082 line), `docs/features/rfc-status.md` - the listen-port receive rule and its tests
- `docs/architecture/traffic/cp-survival-2-copp-port179.md` - ACL, term order, defaults
- `docs/guide/firewall.md`, `docs/guide/flowspec-protected-router.md` - CoPP examples and defaults
- `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` - the two tables' new terms, if it lists them
- `plan/release/validation/spec-bcp194-0-umbrella.md` - S1/S2 rows and the two wiring rows point at the mechanism this spec chose

## Files to Create
- `internal/component/gtsm/rfc5082_gtsm_linux_test.go` additions (file exists) - listener SYN proofs
- the shared neighbor-source reader and its test, placed in Phase 1 (A-3)
- `test/firewall/gtsm-listener-floor.ci` - functional test for the GTSM listen-port term
- `test/firewall/copp-unknown-source-drop.ci` - functional test for the CoPP ACL
- `test/interop/scenarios/bgp-session-protection-frr/` (`ze.conf`, `frr.conf`, scenario checks) - interop scenario, plus any checker in `internal/le/interoplab/bgp/` it needs for the two probes

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/copp/yang/ze-copp-conf.yang`: `rate` loses `mandatory`, `over-limit-policy` default `drop`; no new leaf. GTSM config unchanged |
| YANG validation constraints | Yes | existing `rate-spec` pattern, `protected-port` range and `trusted-source` prefix validator kept |
| YANG custom validators | No | no new leaf; `trusted-source` keeps `ipv4-prefix|ipv6-prefix` |
| CLI commands/flags | N-A | no command; the tables are read with `nft` and `ze doctor` |
| CLI grammar (keyword before value) | N-A | no CLI change |
| Editor autocomplete | Yes | automatic: the enum default changes, nothing to register |
| Functional test for new RPC/API | N-A | no RPC; config-to-kernel `.ci` tests listed under Functional Tests |
| Pipe completeness | N-A | no command output |
| Env var registration | N-A | no `environment/` leaf |
| Doctor check for runtime dependencies | Yes | no new dependency (nftables input hook already used by both owners); the existing `copp-input-chain` and gtsm kernel-state checks extend to the new terms (AC-15); diagnostic codes reused, a new code in `internal/core/diagnostic/codes.go` only if the existing code's text cannot name the term |
| Prometheus counters/metrics | No | the drop is counted by nft term counters (`firewall.Counter`), read by the tests and `nft list`; no Prometheus metric is added |
| BGP family surface (new SAFI / capability / attribute) | N-A | no family, capability or attribute |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | a repair of GTSM and CoPP, both already listed; the feature pages' wording is checked with `./le feature report` in the phase that changes them |
| 2 | Config syntax changed? | Yes | no syntax change; the `rate` optionality and `over-limit-policy` default are documented in `docs/guide/firewall.md` |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | Yes | `docs/architecture/traffic/cp-survival-2-copp-port179.md` (CoPP reads the `bgp` root) |
| 6 | Has a user guide page? | Yes | `docs/guide/firewall.md`, `docs/guide/flowspec-protected-router.md` |
| 7 | Wire format changed? | No | no BGP message changes; packets are filtered, not encoded |
| 8 | Plugin SDK/protocol changed? | No | CoPP uses the existing `ConfigRoots` mechanism |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc5082.md`, `rfc/requirements/rfc5082.md`, the `rfc5082` line of `rfc/enrolled.txt`, `docs/features/rfc-status.md`. RFC 7454 stays non-normative (`rfc/not-enrolled.txt`): its line is rewritten only if it names CoPP or GTSM behaviour that changed |
| 10 | Test infrastructure changed? | Yes, if a new interoplab checker is added | `docs/architecture/testing/interop.md` for the probe checker |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`, if it describes GTSM or CoPP: Ze now drops a below-floor handshake before the TCP stack, which FRR and BIRD leave to the socket |
| 12 | Internal architecture changed? | Yes | `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` if it lists the terms of `ze_gtsm` or `ze_copp` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | CoPP's registration gains a config root; `docs/features/plugins.md` if it lists roots |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` for `source: internal/plugins/copp/`, `source: internal/component/gtsm/`, `source: internal/component/firewall/model.go` in each phase |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/firewall.md` and `docs/guide/flowspec-protected-router.md` CoPP examples checked against the new YANG |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

Each phase is sized for one agent at about 60 tool calls, and lands its own
commit when its scoped package tests pass.

1. **Phase: Wiring (MANDATORY FIRST)** -- both entry points reach their producers; validate A-2 and A-3
   - Tests: `TestListenerMinTTLWiring`, `TestCoppDropTermWiring`, `TestNeighborSourcesFromBGPSection`
   - Files: `internal/plugins/copp/register.go` (`bgp` in `ConfigRoots`, both sections staged), the shared neighbor-source reader and the two existing reads moved onto it (A-3, `./le arch tier check` scoped), `internal/component/bgp/reactor/gtsm_test.go`
   - Verify: the two wiring tests fail on the missing terms, not on plumbing; the reader test passes; A-2 and A-3 marked validated or the design returns here
   - ACs: none closed; prepares AC-1, AC-2, AC-7, AC-8, AC-9
2. **Phase: GTSM listen-port floor** -- the `ze_gtsm` terms, both families
   - Tests: `TestPeerTermsListenPortIPv4`, `TestPeerTermsListenPortIPv6`, `TestPeerTermsSkipsUnsetFloorOrPort`, `TestFilterTablesWithdrawsWithLastPeer`, `TestValidateMatchIPv6HopLimitBelow`, `TestLowerIPv6HopLimitBelow`, `TestGTSMDoctorNamesMissingListenTerm`, the three `TestGTSMListener*` Linux proofs, `test/firewall/gtsm-listener-floor.ci`
   - Files: `internal/component/firewall/model.go`, `validate.go`, `internal/plugins/firewall/nft/lower_linux.go`, `internal/component/gtsm/gtsm.go`, `doctor.go`, RFC 5082 ledger files and docs rows 9 and 12
   - Verify: tests fail → implement → pass; `./le rfc record` for each new tag; `TestListenerMinTTLWiring` green
   - ACs: AC-1, AC-2, AC-3, AC-4, AC-5, AC-6, AC-15 (GTSM half), AC-16 (the reactor publishes each GTSM dynamic group's ranges beside its peers; `gtsm` gains a range form of the listen-port term)
3. **Phase: CoPP ACL** -- neighbor sets, term order, final drop, defaults
   - Tests: `TestTranslatePolicyTermOrder`, `TestTranslatePolicyNoRate`, `TestParseCoppConfigOverLimitDefaultDrop`, `TestCoppReloadAddsNeighbor`, `TestCoppDoctorNamesMissingDropTerm`, `TestCoppUnknownSourceDroppedLinux`, `test/firewall/copp-unknown-source-drop.ci`, updated `copp-bgp.ci`, `copp-trusted.ci`, `copp-over-limit-drop.ci` (each changed expectation through `ze-test-judge`)
   - Files: `internal/plugins/copp/` (config, model, translate, doctor, YANG), CoPP docs rows 2, 5, 6, 15, 17
   - Verify: tests fail → implement → pass; `TestCoppDropTermWiring` green
   - ACs: AC-7, AC-8, AC-9, AC-10, AC-11, AC-12, AC-13, AC-15 (CoPP half)
4. **Phase: Interop and goal validation** -- `bgp-session-protection-frr`
   - Tests: the scenario, with the red phase recorded (Ze image rebuilt with both terms reverted, both probes then answered)
   - Files: `test/interop/scenarios/bgp-session-protection-frr/`, any probe checker under `internal/le/interoplab/bgp/`, `docs/architecture/testing/interop.md`, `docs/comparison.md`, the umbrella's S1/S2 rows
   - Verify: green against FRR latest stable; red recorded
   - ACs: AC-14, and the end-to-end evidence for AC-1, AC-3, AC-7, AC-8

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | the GTSM term matches source address AND the peer's Ze listen port AND below-floor, never a bare TTL; the CoPP unknown-source drop sits after the established, trusted, rate and neighbor terms; the limiter only sees neighbor sources |
| Naming | term names follow `termName` style in `ze_gtsm`; CoPP set and term names kebab-case, stable across reloads so the doctor check can name them |
| Data flow | the reactor stays unaware of CoPP; CoPP learns neighbors only from its SDK `bgp` section through the shared reader |
| Rule: `ai/rules/rfc-compliance.md` | each new GTSM drop carries the RFC 5082 S3 quote above the statement that builds it; new tests tagged and recorded |
| Rule: `ai/rules/no-layering.md` | no listener-wide IP_MINTTL added beside the nft term; the two moved reads of `remote > ip` / `range` leave no old copy |
| Rule: `ai/rules/stale-comments.md` | `gtsm.go` package comment, `peerTerms` comment, `MatchIPv4TTLBelow` comment, CoPP YANG descriptions |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| `ze_gtsm` listen-port terms, both families | `TestPeerTermsListenPortIPv4`, `TestPeerTermsListenPortIPv6`; `test/firewall/gtsm-listener-floor.ci` |
| Below-floor SYN dropped on Ze's kernel | `TestGTSMListenerSYNBelowFloorDropped` and its IPv6 sibling under `./le job run` |
| CoPP final drop with derived neighbors | `TestCoppDropTermWiring`; `test/firewall/copp-unknown-source-drop.ci` |
| Defaults changed | `TestParseCoppConfigOverLimitDefaultDrop`; `grep -n 'default "drop"' internal/plugins/copp/yang/ze-copp-conf.yang` |
| Interop | `bgp-session-protection-frr` green, red phase recorded in the spec |
| RFC 5082 records | `./le rfc check` names no missing record for the new tags |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | the neighbor-source reader refuses nothing the BGP parse accepts and drops nothing it accepts; a malformed address is already refused by the BGP config |
| Fail-open | a failed apply of either table is reported and not silently swallowed; the doctor checks name a missing drop term |
| Resource exhaustion | term count bounded by configured peers (one GTSM listen term per peer per family; CoPP uses sets, so its term count is fixed) and never by anything a peer sends |
| Lock-out | chain policy stays `accept`; the CoPP drop names TCP to protected ports only; SSH and ICMP covered by AC-12 |
| Spoofing within the TTL radius | documented limit of GTSM (RFC 7454 S5.2): a directly connected attacker can still send at 255; not a defect of this spec |

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
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->

- Linux reads a socket's floor for whatever socket the lookup returned: `tcp_v4_rcv` checks `if (unlikely(iph->ttl < READ_ONCE(inet_sk(sk)->min_ttl)))` after its `process:` label, and `tcp_v6_rcv` checks `hdr->hop_limit < READ_ONCE(tcp_inet6_sk(sk)->min_hopcount)` (kernel.org master, `net/ipv4/tcp_ipv4.c`, `net/ipv6/tcp_ipv6.c`). On a LISTEN socket that is one floor for every source.
- FRR `bgp_network.c`, the listener setup: "if we intend to implement ttl-security, this socket needs ttl=255", followed by `sockopt_ttl(ainfo->ai_family, sock, MAXTTL)`; the floor is set only in `bgp_set_socket_ttl(connection)`, called on the accepted `incoming` connection in `bgp_accept` and on the dialing socket, as `sockopt_minttl(..., MAXTTL + 1 - peer->gtsm_hops)`.
- BIRD `proto/bgp/bgp.c`: `bgp_listen_open` sets `sk->ttl = 255` on the listener; `bgp_incoming_connection` sets `sk_set_min_ttl(sk, 256 - hops)` on the accepted socket only when `p->cf->ttl_security`; `bgp_connect` does the same after `sk_open(s)`.
- Ze already matches FRR and BIRD on the socket. The listen-port term goes further, and interoperates because it drops only what RFC 5082 S3 lets an implementation drop.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D-1: the below-floor drop before accept is a per-peer nftables term in `ze_gtsm` (source = peer, TCP destination port = the peer's Ze listen port, TTL or hop limit below the peer's floor) | (a) IP_MINTTL / IPV6_MINHOPCOUNT on the listen socket; (b) a listener per GTSM peer bound to the local address | (a) is one floor per listener (A-1), so it drops a non-GTSM or multihop peer's SYN and any unconfigured source's SYN, which RFC 5082 S3 forbids: "MUST NOT drop (as part of GTSM processing) packets classified as Trusted or Unknown." (b) cannot separate peers that share a local address. The term classifies per source, which S3 asks of "The GTSM packet identification step", and S2.2 places it where it belongs: "For maximal protection, ingress filtering should be applied before the packet goes through the scarce resource." The `ze_gtsm` table, its owner and its reconcile already exist |
| D-2: keep the per-connection floor | drop it as redundant with D-1 | FRR and BIRD both rely on it (Design Insights); it guards a session when the firewall backend is absent; it carries enrolled RFC 5082 tests |
| D-3: IPv6 gets a daemon-only hop-limit match beside `MatchIPv4TTLBelow` | leave IPv6 to the per-connection IPV6_MINHOPCOUNT | the SYN reaches the listener before any per-connection option exists, the same gap as IPv4; RFC 7454 S5.2 makes no family distinction |
| D-4: CoPP derives its permitted sources from the BGP config: each static peer's remote address as a host prefix, each dynamic group's ranges, plus `trusted-source` | (a) keep hand-typed `trusted-source` only; (b) the reactor publishes the set to CoPP | RFC 7454 S4: "This protection SHOULD be achieved by an Access Control List (ACL) that would discard all packets directed to TCP port 179 on the local device and sourced from an address not known or permitted to become a BGP neighbor", and "Some routers automatically program such an ACL upon BGP configuration." (a) leaves the ACL to drift from the peers; (b) is a Go call into a plugin that may run in another process (`ai/doors/system-plugins.md`) |
| D-5: the CoPP chain ends with a drop of TCP to a protected port from any other source, in any connection state, after the `established` accept | drop only `ct state new` | S4 says "discard all packets directed to TCP port 179"; an unknown source cannot own an established session, so the `established` accept still protects every live session |
| D-6: the rate limit applies to accepted neighbors only, after the unknown-source drop; `trusted-source` keeps bypassing it | limit everything to the port, as today | S4: "In addition to strict filtering, rate-limiting MAY be configured for accepted BGP traffic." Limiting before filtering lets an unknown-source flood spend the bucket that neighbors need (R-5) |
| D-7: `rate` becomes optional | keep it mandatory | the ACL is the SHOULD and the limit is the MAY (S4); requiring a rate to get the ACL forces a number the operator may not want |
| D-8: `over-limit-policy` defaults to `drop` | keep `accept` | the umbrella's S2 row and this spec's release reason name the `accept` default as failing open: a configured limit that stops nothing. `accept` remains available. Pre-release, no deployment consumes `main` (`ai/META-DECISIONS.md`, "Ze is pre-release") |
| D-9: CoPP stays opt-in; no implicit ACL when BGP alone is configured | program the ACL on every BGP config, as S4 says some routers do | the umbrella's "Behavior to preserve" forbids a child introducing an implicit deny that black-holes an existing deployment on upgrade; RFC 7454 is non-normative for Ze (`rfc/not-enrolled.txt`) and S4 itself allows manual configuration: "On other devices, this ACL should be configured and maintained manually or using scripts." |
| D-10: TCP-AO stays out | fold RFC 5925 in | unchanged from the umbrella: S5.1 reads "TCP-AO SHOULD be preferred when implemented", and Ze implements none |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     needs a row in the deferral shard named in the metadata table. -->
- GTSM cannot stop an attacker inside the TTL radius; RFC 7454 S5.2: "Nevertheless, it is not as effective because anyone inside the TTL diameter could spoof the TTL." That is the mechanism's limit, not work owed.
- The related-ICMP terms for a dynamic group's members stay as they are today (absent until a member exists); this spec adds the listen-port term for the group's ranges only (AC-16), because the SYN arrives before any member exists.
- TCP-AO (RFC 5925) remains absent; it needs its own spec (umbrella, Out of scope).
- CoPP beyond TCP is `plan/post-release/features/spec-dataplane-seams-5-copp-non-tcp.md`.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

For this spec:

| Code | Quote it carries |
|------|------------------|
| the statement in `peerTerms` that builds the listen-port drop | RFC 5082 Section 3, "MAY drop packets classified as Dangerous." and the MUST NOT on Trusted and Unknown, which the term's source-and-port match is what honours |
| the source-and-port match itself | RFC 5082 Section 3, the Dangerous classification sentence: "these are packets that have been identified as belonging to one of the GTSM-enabled sessions, but their TTL values are NOT within the expected range, and hence GTSM believes there is a risk that these packets have been spoofed." |
| the CoPP unknown-source drop in `translatePolicy` | RFC 7454 Section 4, the ACL sentence quoted in Key Design Decisions D-4 |
| the CoPP rate term's placement | RFC 7454 Section 4, "In addition to strict filtering, rate-limiting MAY be configured for accepted BGP traffic." |

Each new caller added on the path (the reactor's range publication, CoPP's
verify reading the `bgp` section) names the RFC and section at the call.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Disposition

| Field | Value |
|-------|-------|
| Date | 2026-10-27 |
| Checked at | e6474aa200 |
| Verdict | REWORK |
| Gap 1 still open | `RealListenerFactory.Listen` (`internal/core/network/network.go`) sets MD5 and `setListenIPTTL` only; the floor is set per connection by `tuneTCPConnectionForSettings` (`internal/component/bgp/reactor/session_connection.go`) |
| Gap 2 still open | `translatePolicy` (`internal/plugins/copp/translate.go`) emits established, trusted-source and rate-limit terms and no drop; `over-limit-policy` still `default "accept"` in `ze-copp-conf.yang`; `ConfigRoots` holds only `control-plane-protection` |
| Ground moved | `internal/component/gtsm` landed after the 2026-08-08 baseline (479e9d76a9, 2026-09-14) and owns a per-peer `ze_gtsm` nftables input table fed by `Reactor.publishGTSMKernelState` |
| Mechanism replaced | the Task's implied listen-socket IP_MINTTL would drop Unknown packets on a shared listener (A-1, RFC 5082 S3); FRR and BIRD set the floor per connection only |

The goal of both gaps holds at HEAD, so the spec is neither superseded nor
obsolete. The design it implied for Gap 1, a receive floor on the listen
socket, does not fit: one Linux socket floor applies to every source the
listener serves, and the gtsm component that landed in September already owns
the per-peer nftables table where a per-source drop belongs. The redesign above
moves Gap 1 onto that table and settles Gap 2 from RFC 7454 S4.
