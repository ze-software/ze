# Spec: AS112 Anycast DNS Service (Umbrella)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-01 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. The three child specs: `spec-as112-1-iface-address-registry.md`, `spec-as112-2-dns-server.md`, `spec-as112-3-bgp-integration.md`
4. `rfc/short/rfc7534.md`, `rfc/short/rfc7535.md` - AS112 protocol requirements
5. `internal/plugins/geodns/` - architectural precedent for the new DNS plugin
6. `internal/component/bgp/plugins/watchdog/`, `internal/component/bgp/plugins/healthcheck/` - conditional route announcement mechanism this design reuses

## Task

Add an AS112 node to ze: a system plugin that answers DNS queries authoritatively
for the AS112 sink zones (RFC 1918 / link-local reverse zones, EMPTY.AS112.ARPA)
on the four well-known AS112 anycast addresses, automatically configured on
loopback, with those addresses originated into BGP and sent to operator-selected
peers/peer-groups, carrying an operator-chosen well-known BGP community
(no-export / no-advertise / nopeer) and an optional AS112-origin override, with
the BGP route automatically withdrawn if the DNS service itself becomes
unhealthy.

This umbrella owns the shared research, the cross-cutting design decisions, and
the RFC compliance mapping (every SHOULD in `rfc/short/rfc7534.md` and
`rfc/short/rfc7535.md`, with an explicit Met/Not-Met verdict for each). It ships
**no executable code itself**; the three child specs own implementation:

| Child | Scope | Depends |
|-------|-------|---------|
| `spec-as112-1-iface-address-registry.md` | Generic plugin-to-iface address-ownership registration API | - |
| `spec-as112-2-dns-server.md` | The `internal/plugins/as112/` DNS server plugin | spec-as112-1, **spec-dns-server-harness** |
| `spec-as112-3-bgp-integration.md` | BGP `update`-block wiring, healthcheck probe, worked config, end-to-end test | spec-as112-1, spec-as112-2 |

**Prerequisite (runs BEFORE this umbrella):** `plan/spec-dns-server-harness.md`
extracts geodns's generic DNS-server primitives (listener lifecycle, EDNS0/
client-IP resolution, authoritative-answer wrapper, `IP_FREEBIND` option, CIDR
matcher) into `internal/core/dnsserver` and migrates geodns onto it. child 2
CONSUMES that harness rather than mirroring geodns — a plugin MUST NOT import a
sibling plugin (`ai/rules/plugin-design.md:133`). Not a child of this umbrella
(it also benefits geodns), but child 2 is blocked on it.

## Required Reading

### Architecture Docs
- [ ] `ai/patterns/plugin.md` - plugin pattern (register.go, SDK protocol, doctor checks)
  → Constraint: the as112 plugin must be self-contained — its own YANG, doctor check, CLI/show command. Remove the plugin and the feature fully vanishes (`ai/rules/plugin-self-containment.md`).
- [ ] `ai/patterns/registration.md` - registration-over-hardcoding
  → Decision: the iface address-ownership API (child 1) is a generic registry, not a single `if pluginName == "as112"` special case — any future plugin needing anycast-style addressing can register against it.
- [ ] `ai/rules/plugin-self-containment.md`
  → Constraint: BGP origination is NOT owned by the as112 plugin. It is ordinary, already-generic BGP `update`-block config (community + watchdog), scoped to operator-chosen peer-groups. The as112 plugin never reaches into BGP config.

### RFC Summaries
- [ ] `rfc/short/rfc7534.md` - AS112 Nameserver Operations (this session; zones, addresses, host/routing/DNS-software requirements, operations, security)
  → Constraint: exact zone list (10.in-addr.arpa, 16-31.172.in-addr.arpa, 168.192.in-addr.arpa, 254.169.in-addr.arpa, HOSTNAME.AS112.NET/ARPA) and exact anycast addresses (192.175.48.1, 192.175.48.6, 192.175.48.42, 2620:4f:8000::{1,6,42}; this spec set uses only the four addresses below, not the secondary BLACKHOLE-1/2 addresses).
  → Constraint: zones contain SOA + NS only — "There should be no other resource records included in this zone" (§3.5).
- [ ] `rfc/short/rfc7535.md` - AS112 Redirection Using DNAME (this session; EMPTY.AS112.ARPA, the two DNAME-redirection addresses, DNAME-support-on-node-never-required)
  → Constraint: EMPTY.AS112.ARPA is reachable at 192.31.196.1 / 2001:4:112::1; the as112 node itself never needs to understand DNAME (§6). The plugin serves ONE static zone table on every bound address (child 2) — it does not restrict which zones answer on which address. This is harmless because the reverse zones are delegated only to the Direct-Delegation prefix (192.175.48.0/24, 2620:4f:8000::/48), so no reverse-zone query is ever routed to a DNAME address; and empty.as112.arpa answering on a Direct-Delegation address is likewise never reached in practice. (Earlier drafts said the DNAME addresses "serve only EMPTY.AS112.ARPA" — that per-address zone restriction is NOT implemented and is not required; this note supersedes it. Review finding M2.)
- [ ] `rfc/short/rfc1997.md` - BGP Communities Attribute (NO_EXPORT / NO_ADVERTISE / NO_EXPORT_SUBCONFED)
  → Constraint: well-known community semantics already implemented and named in `internal/core/bgp/attribute/community.go:36-53` (`CommunityNoExport=0xFFFFFF01`, `CommunityNoAdvertise=0xFFFFFF02`, `CommunityNoExportSubconfed=0xFFFFFF03`).
- [ ] `rfc/short/rfc3765.md` - NOPEER Community
  → Constraint: `CommunityNoPeer=0xFFFFFF04` (`community.go:53`), the community RFC 7534 specifically recommends for AS112 routes sent to bilateral peers.
- [ ] `rfc/short/rfc1035.md` - DNS message/RR structure, negative-answer semantics
  → Constraint: a name that exists in a served zone but has no record of the queried type is NOERROR with the zone SOA in Authority (NODATA), not NXDOMAIN; a name outside every served zone is NXDOMAIN. geodns already implements this pattern (`internal/plugins/geodns/server.go`).

**Key insights:**
- Almost everything this feature needs already exists in ze: BGP `update`-block
  communities (no new parsing), the `asn.local`/`local-options replace-as` peer
  override (no new origin-AS code), and the `healthcheck`→`watchdog` plugin pair
  (no new conditional-announcement code). The only genuinely new code is (a) the
  AS112 DNS server plugin itself and (b) a small, generic address-ownership
  registry in `iface` so enabling the service is sufficient — no second,
  manually-duplicated address configuration step (unlike geodns's existing
  precedent, which requires the operator to configure addresses twice).
- `internal/component/iface/cmd/manage.go:80-94` (`handleAddrAdd`, RPC
  `ze-iface:interface-addr-add`) looks like a shortcut but is a trap: it's an
  imperative, ephemeral kernel-only action. It does not touch
  `config_apply.go:18-110`'s `desiredState()`, so an address added this way is
  reconciled away (removed as "stray") on the very next config-apply pass
  (`config_apply.go:778-813`). This is why child 1 must be a real registry
  consulted by `desiredState()`, not a startup-time RPC call.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/geodns/register.go` - plugin registration: `ConfigRoots: ["service"]`, `InProcessConfigVerifier`, `RunEngine`, `ConfigureEngineLogger`/`ConfigureMetrics` callbacks, `DoctorChecks` (lines 24-66); doctor check `geodns-listen-capability` (lines 47-55) warns when a configured listener can't bind (privileged port).
- [ ] `internal/plugins/geodns/server.go` - DNS server lifecycle: `bind()` opens one UDP + one TCP listener per endpoint via `net.ListenConfig` (lines 360-385); `apply()` reconciles listeners only on endpoint-set change, computed via `endpointSig()` (lines 322-358); single `dns.ServeMux` handler reads an atomically-published `resolverState` snapshot per query (lines 254-301); `stopAll()` drains in-flight handlers with a 5s timeout (lines 393-409).
- [ ] `internal/plugins/geodns/state.go` - `resolverState` (config + matcher + SOA serial) published via `sync/atomic.Pointer`; handler reads the snapshot lock-free (lines 7-32).
- [ ] `internal/component/iface/config.go` - `parseUnits()` merges a loopback unit's IPv4 and IPv6 leaf-list addresses into one `unitEntry.Addresses` slice (lines 875-954, merge at 908-914).
- [ ] `internal/component/iface/config_apply.go` - `desiredState()` builds the per-OS-interface-name desired-address map purely from parsed YANG config (lines 18-110); reconciliation (`reconcileOnReadyWithJournal`, line 757) calls `AddAddress`/`RemoveAddress` to converge kernel state to that map — any kernel address not present in `desiredState()` is treated as stray and removed (lines 778-813).
- [ ] `internal/component/iface/cmd/manage.go` - `handleAddrAdd`/`handleAddrDel` (RPC `ze-iface:interface-addr-add`/`-del`, lines 80-110) call `iface.AddAddress`/`iface.RemoveAddress` directly — imperative only, bypasses `desiredState()` entirely.
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `container loopback` (lines 1114-1119): "Always present; ze manages its addresses and units," uses the shared `interface-unit` grouping (IPv4/IPv6 `leaf-list address`, lines 263-423).
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - `update` block container (lines 200-266): scoped to peer, group, or global; accumulates (BGP-level + group-level + peer-level routes all apply to a given peer); `community` leaf-list (line 237) accepts well-known names or `asn:value`; `watchdog` container (lines 260-264, `name`+`withdraw` leaves) ties a route to a named watchdog group; `asn` container (lines 445-468) has `local` (override local AS for this peer/group) and `local-options` leaf-list (`no-prepend`, `replace-as`).
- [ ] `internal/core/bgp/attribute/community.go` - well-known community constants: `CommunityNoExport=0xFFFFFF01` (line 39), `CommunityNoAdvertise=0xFFFFFF02` (line 43), `CommunityNoExportSubconfed=0xFFFFFF03` (line 49), `CommunityNoPeer=0xFFFFFF04` (line 53); display-name map (lines 107-110).
- [ ] `internal/core/bgp/attribute/text.go` - `wellKnownCommunityNames` (lines 49-58) parses `no-export`/`no-advertise`/`no-export-subconfed`/`nopeer` (and ASN:value, hex, bare-int forms) from config/update-block text.
- [ ] `internal/component/bgp/config/peers.go` - `ExportFilters` concatenate BGP-level + group-level + peer-level filters (lines 155-156); group-scoped `update` routes are announced only within that group's peer subtree.
- [ ] `internal/component/bgp/plugins/watchdog/config.go` - `collectWatchdogRoutes()` parses `update.watchdog{name;withdraw}` blocks into per-peer route pools keyed by watchdog-group name (lines 127-172).
- [ ] `internal/component/bgp/plugins/watchdog/pool.go` - `PoolSet`/`RoutePool` track per-peer announced/withdrawn state per watchdog group (lines 15-307).
- [ ] `internal/component/bgp/plugins/watchdog/server.go` - `request bgp watchdog announce|withdraw <group> [med <N>] [peer]` command dispatch (lines 44-82).
- [ ] `internal/component/bgp/plugins/healthcheck/fsm.go` - probe state machine: INIT → RISING/FALLING → UP/DOWN, transition after `rise`/`fall` consecutive successes/failures (lines 5-92).
- [ ] `internal/component/bgp/plugins/healthcheck/healthcheck.go` - `dispatchStateAction()`: UP dispatches `request bgp watchdog announce <group> med <up-metric>`; DOWN dispatches withdraw (if `withdraw-on-down=true`) or a re-announce with `down-metric` (lines 279-301).
- [ ] `internal/component/bgp/plugins/healthcheck/probe.go` - shell-command probe execution via `/bin/sh -c`, configurable timeout, process-group isolation (lines 17-51).
- [ ] `internal/component/bgp/plugins/healthcheck/config.go` - probe schema: `command`, `group`, `interval`, `fast-interval`, `timeout`, `rise`, `fall`, `withdraw-on-down`, `up-metric`/`down-metric`/`disabled-metric` (lines 14-43).
- [ ] `internal/plugins/static/yang/ze-static-conf.yang` - confirms static routes have no community/peer-targeting fields and only per-next-hop BFD tracking, not whole-route conditional advertisement (lines 110-159).

**Behavior to preserve:**
- geodns's listener-reconciliation pattern (rebind only on endpoint-set change) — the as112 plugin reuses this exact lifecycle shape.
- `desiredState()`'s existing semantics for operator-declared YANG addresses are unchanged; child 1 only adds a second, merged source of desired addresses (plugin-registered), it does not alter how YANG-declared addresses are computed or reconciled.
- Existing watchdog/healthcheck command surface and YANG schema are unchanged; this feature is a new *consumer* (via documented config), not a new code path in those plugins.

**Behavior to change:**
- `internal/component/iface` gains a new generic address-ownership registry (child 1) — purely additive; no existing iface behavior changes for interfaces/addresses that have no registered owner.

## Data Flow (MANDATORY)

### Entry Point
- Config commit: `service/as112/enabled` (child 2) plus operator-authored BGP
  `group` `update` blocks whose NLRI are the four **covering /24,/48 service
  prefixes** (NOT the /32,/128 host addresses — see Design Insights, finding H3),
  each with a `community` and a `watchdog` block that includes `withdraw`
  (finding H2), plus a `healthcheck` probe (child 3).

### Transformation Path
1. On `enabled: true`, the plugin registers its four /32,/128 host addresses
   against the iface registry (child 1) and publishes a reconcile-trigger event
   so iface re-reconciles immediately, not on the next unrelated commit (B1).
2. iface `desiredState()` (`config_apply.go:18-110`) merges plugin-owned and
   YANG addresses; `reconcileOnReadyWithJournal` (`config_apply.go:757-813`)
   adds them to `lo` and removes strays. The registration event drives the same
   trigger path iface uses for vpp `EventConnected` (`register.go:257`
   `subscribeReconcileOnReady`).
3. The DNS server (child 2) binds UDP+TCP on the four host addresses + loopback,
   port 53, answering static SOA/NS-only zones (NXDOMAIN out of zone),
   authoritative-only. Sockets set `IP_FREEBIND` via `net.ListenConfig.Control`
   so bind never races address presence (B2; geodns lacks this,
   `geodns/server.go:362`).
4. A `healthcheck` probe (child 3) queries an anycast service address (not just
   loopback) via a `ze` health command child 2 supplies (H1/M4). UP →
   `request bgp watchdog announce <group>`; DOWN → withdraw.
5. The watchdog announces/withdraws the operator's `update`-block routes
   (chosen community + optional `asn.local 112 replace-as`) to the targeted
   group(s) — unmodified.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| as112 plugin ↔ iface (child 1) | new Go-level address-ownership registration API, consulted by `desiredState()` | [ ] |
| iface ↔ kernel | existing netlink `AddAddress`/`RemoveAddress` reconciliation, unchanged | [ ] |
| DNS client ↔ as112 plugin | UDP/TCP port 53 on the four anycast addresses, miekg/dns | [ ] |
| healthcheck probe ↔ as112 plugin | authoritative DNS query against an **anycast service address** via child 2's `ze` health command (finding H1), existing healthcheck probe mechanism | [ ] |
| healthcheck ↔ watchdog | existing `DispatchCommandArgs` / `request bgp watchdog announce|withdraw`, unmodified | [ ] |
| watchdog ↔ BGP peers | existing wire UPDATE/WITHDRAW via the group-scoped `update` block, unmodified | [ ] |

### Integration Points
- `internal/component/iface/config_apply.go` `desiredState()` - child 1 hooks here
- `internal/plugins/geodns/server.go` lifecycle pattern - child 2 adapts this shape
- `internal/component/bgp/plugins/watchdog/config.go` `collectWatchdogRoutes()` - child 3's worked example routes through this, unmodified
- `internal/component/bgp/plugins/healthcheck/healthcheck.go` `dispatchStateAction()` - child 3's probe routes through this, unmodified

### Architectural Verification
- [ ] No bypassed layers (each child uses its subsystem's existing config→backend path; BGP origination never goes through the as112 plugin)
- [ ] No unintended coupling (children are independent Go packages; the as112 plugin never imports BGP packages, and vice versa)
- [ ] No duplicated functionality (child 3 reuses watchdog+healthcheck rather than inventing a new conditional-announcement mechanism; child 2 reuses the geodns server-lifecycle shape rather than a new DNS engine)
- [ ] Registration over hardcoding — child 1's registry is generic (no `as112`-specific spelling inside `internal/component/iface`); the as112 plugin self-registers like any other plugin

## RFC Compliance Mapping

Every SHOULD/MUST line from the `rfc/short/rfc7534.md` and `rfc/short/rfc7535.md`
Compliance Checklists, with an explicit verdict. (Re-read those two files for
exact wording before implementing — this table is the authoritative cross-check,
not a substitute for reading the checklists themselves.)

| # | Source | Requirement | Verdict | Closed by |
|---|--------|-------------|---------|-----------|
| 1 | rfc7534 §3.5 | MUST answer authoritatively for each delegated zone | Met — child 2 also pins the RFC-mandated SOA parameters (refresh 1W, retry 1M, expire 1W, min-TTL 1W, $TTL 1W) and canonical NS/MNAME names, with a test asserting them (finding M1) | spec-as112-2 |
| 2 | rfc7534 §3.5 | MUST NOT include records beyond SOA/NS in Direct Delegation zones | Met | spec-as112-2 |
| 3 | rfc7534 §3.5 | MUST NOT host site's own RFC1918 records on the AS112 nameserver | Met by design (plugin only ever serves the fixed static empty-zone data, never site records) | spec-as112-2 |
| 4 | rfc7534 §3.3 | SHOULD support cloned loopback / multiple loopback addresses | Met (existing iface capability) | spec-as112-1 |
| 5 | rfc7534 §3.3 | SHOULD dedicate the host to AS112 purpose | **Not met — not software-enforceable.** Deployment recommendation only | Documented in `docs/guide/as112.md`; listed in Known Limitations |
| 6 | rfc7534 §3.3 | SHOULD order startup: loopback → DNS → routing | Met functionally by a stronger mechanism, **conditional on two things (findings H1/H2):** (a) the `update` block includes the `watchdog` `withdraw` marker so the route starts withdrawn — its absence defaults to *announced* (`watchdog/config.go:145,292`); and (b) the healthcheck probe queries an *anycast service address*, not loopback, so UP means the advertised path actually answers. Both are enforced in child 3's worked example + tests | spec-as112-3 |
| 7 | rfc7534 §3.3 | SHOULD NOT advertise the service prefix while addresses unconfigured or DNS not running | Met (healthcheck → watchdog), under the same two conditions as #6 (H1/H2): `withdraw` marker present + anycast-address probe. Child 3 adds an advisory doctor check when an AS112 watchdog-gated `update` block omits `withdraw` | spec-as112-3 |
| 8 | rfc7534 §3.4 | SHOULD restrict outbound advertisement to a prefix filter permitting only the service prefixes + an AS_PATH filter matching only locally-originated routes | **Conditionally met** — true if the operator dedicates the target peer-group to AS112 only (recommended, worked example in docs); not enforceable if the operator reuses a general-purpose/transit peer-group for other routes too (that is the operator's own BGP policy, outside this feature's scope) | spec-as112-3 (docs); flagged as R-1 below |
| 9 | rfc7534 §3.5 | SHOULD run authoritative-only (recursion disabled) | Met by design | spec-as112-2 |
| 10 | rfc7534 §3.5 | SHOULD keep HOSTNAME.AS112.{NET,ARPA} TXT answers within 512 octets without EDNS0 | Met — child 2 boundary-tests the **assembled UDP response size** (hostname + facility + location TXT strings + NS + SOA) with TC=0, not any single field in isolation (finding M3) | spec-as112-2 |
| 11 | rfc7534 §4.1 | SHOULD monitor the node as a production service | Met (Prometheus metrics + `show as112`, mirrors geodns) | spec-as112-2 |
| 12 | rfc7534 §4.2 | SHOULD withdraw the service prefix before planned downtime | Met (manual `watchdog withdraw`, or automatic via healthcheck) | spec-as112-3 |
| 13 | rfc7534 §4.3 | SHOULD measure usage for trend/anomaly tracking | Met (Prometheus counters) | spec-as112-2 |
| 14 | rfc7534 §3.2/§5 | SHOULD notify the local community before installing; coordinate with other AS112 operators for globally-reachable nodes | **Not met — not software-enforceable.** Process/organizational step | Documented in `docs/guide/as112.md`; listed in Known Limitations |
| 15 | rfc7534 §3.4 | MAY configure only the relevant address family for single-stack nodes | Met — optional ipv4-only/ipv6-only toggle (no raw IP entry, per explicit user instruction) | spec-as112-2 |
| 16 | rfc7535 §6 | MUST NOT require DNAME support on the AS112 node itself | Met — the plugin only answers EMPTY.AS112.ARPA directly, never processes a DNAME | spec-as112-2 |
| 17 | rfc7535 §3.1 | SHOULD configure 192.31.196.1/2001:4:112::1 and announce covering routes, and host EMPTY.AS112.ARPA | Met | spec-as112-1, -2, -3 |
| 18 | rfc7535 §3.1 | SHOULD configure only the relevant address for single-stack nodes | Met — same toggle as #15 | spec-as112-2 |
| 19 | rfc7535 §4 | SHOULD leave existing Direct Delegation delegation/continuity unchanged | N/A for local-use deployment (routes never reach the global anycast cloud unless the operator explicitly applies the AS112-origin override on a publicly-peered group, which is then their own coordination responsibility) | Documented in `docs/guide/as112.md` |

**Items that cannot be met in software** (#5, #14): both are deployment/process
recommendations about how the operator runs and announces the service to the
human community, not something ze can verify or enforce. They are recorded here
per the explicit instruction to report any SHOULD the implementation cannot
meet, and are repeated in Known Limitations.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | This is deployed as a local-use AS112 mirror by default (operator's own ASN), with the global AS112 origin (AS 112) only on peer-groups where the operator explicitly applies `asn.local 112 replace-as` | User confirmed per-peer-group origin-AS choice should be a configuration knob, not a fixed mode | A peer-group accidentally leaks AS-112-origin routes to the wrong upstream | child 3 functional test asserting AS_PATH differs between an overridden and a non-overridden peer-group | unvalidated |
| A-2 | The four addresses (192.175.48.1, 192.31.196.1, 2620:4f:8000::1, 2001:4:112::1) are sufficient — the secondary IANA addresses (.48.6/.48.42 and IPv6 equivalents) are deliberately out of scope | User confirmed "4 addresses total" earlier in this conversation | Operator expects full 3-nameserver-per-family parity with real AS112.NET hosts | re-confirm with user at spec WRITE-gate review | unvalidated |
| A-3 | `desiredState()` (`config_apply.go:18-110`) can be extended to merge a second, plugin-sourced map without changing its existing YANG-only call sites | Read of `config_apply.go:18-110` (desiredState) and `757-813` (reconcile); no other caller relies on `desiredState()` being YANG-only | child 1 needs a larger refactor than anticipated | child 1 unit test exercising `desiredState()` with both a YANG address and a registered address present simultaneously | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Operator reuses a shared/transit peer-group for the AS112 `update` block, defeating RFC 7534 §3.4's "no transit" SHOULD (item #8 above) | `show bgp` on that group shows non-AS112 prefixes alongside the AS112 routes | `docs/guide/as112.md` recommends a dedicated peer-group with a worked example; doctor check (child 3, advisory severity) warns if a watchdog-gated AS112 route shares a group with non-AS112 `update` blocks |
| R-2 | A weak healthcheck probe (bare TCP connect, or a query to loopback only) gives a false UP while the anycast address is unreachable or the DNS content is wrong, defeating items #6/#7 (findings H1/R-2) | Functional test catches probe gap during review | child 3's worked-example probe issues a real authoritative query **against an anycast service address** (not loopback) for a known AS112 zone, via child 2's `ze` health command — verifies both address reachability and content |
| R-3 | A future plugin double-registers the same loopback address via child 1's registry, causing an ownership conflict | `go test -race` / unit test on the registry rejecting/erroring a conflicting registration | child 1 AC requires the registry to reject (not silently overwrite) a conflicting registration for an address already owned by a different registrant |
| R-4 | **(B1)** Addresses registered during the plugin's config handler are not picked up in the same commit because iface reconcile ran earlier (plugin handler order is non-deterministic — `plan/learned/821-plugin-internal-keyword.md`), so enabling as112 leaves `lo` without the addresses until an unrelated later commit | `test/parse/as112-address-registry.ci` shows addresses absent right after enable | child 1 makes `RegisterOwnedAddresses`/`Unregister` publish a reconcile-trigger event that re-runs iface reconcile (same path as `subscribeReconcileOnReady`, `iface/register.go:257`); child 1 AC + `.ci` assert the address reaches the kernel within the enable/disable op |
| R-5 | **(M4)** The worked-example probe uses a tool absent from the gokrazy appliance (`dig` is not shipped; `ze resolve dns` uses the local resolver and cannot target a specific server — `ze-resolve-cmd.yang:20-39`), so the probe never succeeds on the real target | Probe fails only on the appliance, not on the dev host `.ci` runs | child 2 supplies a `ze` health command (built into the always-present `ze` binary) that actively queries the anycast addresses; child 3's probe calls it |
| R-6 | **(B2)** The DNS server binds the anycast addresses before iface has applied them to `lo`, so `bind()` fails with `EADDRNOTAVAIL` | listener-down in `show as112` / doctor at startup | child 2 sets `IP_FREEBIND` on its listener sockets via `net.ListenConfig.Control` (geodns does not — `geodns/server.go:362`), so bind succeeds regardless of address-apply timing |

## Wiring Test (MANDATORY)

The umbrella has no executable feature of its own; its "wiring" is that each
child's wiring test passes and the deployment doc exists.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| spec-as112-1 closed | → | iface address registry | spec-as112-1 wiring test (see child) |
| spec-as112-2 closed | → | as112 DNS plugin | spec-as112-2 wiring test (see child) |
| spec-as112-3 closed | → | BGP worked-example + end-to-end test | spec-as112-3 wiring test (see child) |
| `docs/guide/as112.md` exists & is linked | → | deployment + RFC compliance guidance | `make ze-doc-test` (doc lint) + grep for the source anchor |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Child spec 1 (iface address registry) complete | `spec-as112-1-iface-address-registry.md` closed: generic registration API exists, tested, race-free |
| AC-2 | Child spec 2 (DNS server) complete | `spec-as112-2-dns-server.md` closed: as112 plugin serves all RFC 7534/7535 zones correctly on the four addresses, no operator-typed IP |
| AC-3 | Child spec 3 (BGP integration) complete | `spec-as112-3-bgp-integration.md` closed: worked config example + end-to-end interop test proving conditional announcement, community, and origin-AS override all work |
| AC-4 | Operator reads docs | `docs/guide/as112.md` documents the full RFC compliance mapping above, including the two not-software-enforceable SHOULDs, and a worked dedicated-peer-group example |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | enables `service/as112/enabled`, configures a dedicated peer-group with an AS112 `update` block (community + watchdog) and a healthcheck probe | child 1 (addresses appear) → child 2 (DNS answers correctly) → child 3 (healthcheck UP → watchdog announce, with chosen community) | child 3's end-to-end functional/interop test |
| 2 | the AS112 DNS server stops responding | healthcheck FSM → DOWN → watchdog withdraw → route disappears from the peer-group | child 3's failure-path functional test |
| 3 | operator wants AS112-origin routes on one peer-group only | sets `asn.local 112` + `local-options replace-as` on that group; another group is left at the real local AS | child 3's AS-PATH assertion test |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (none — umbrella owns no executable code) | n/a | unit coverage lives in the three child specs | |

### Functional Tests
No new user-facing feature in this file — functional coverage lives entirely in
the child specs (`spec-as112-1`, `spec-as112-2`, `spec-as112-3`); this row
covers only the doc.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `as112-doc` | `make ze-doc-test` | deployment guide builds and source anchors resolve | |

### Interop Tests
Owned by child 3 (the only child that touches the BGP wire). N/A for the umbrella itself.

## Files to Modify
- `docs/features.md` - add "AS112 anycast DNS" feature row (source-anchored)

## Files to Create
- `docs/guide/as112.md` - RFC compliance mapping, deployment guidance (dedicated peer-group recommendation, host-dedication and community-notification recommendations), worked config example. MUST also: (a) show the four covering /24,/48 prefixes as the announced NLRI and distinguish them from the /32,/128 host addresses (finding H3); (b) carry a hard warning that `asn.local 112 replace-as` on a publicly-peered group makes this an uncoordinated global AS112 node (finding M5); (c) show the `watchdog` `withdraw` marker in the worked `update` block and explain why omitting it announces before the DNS is healthy (finding H2); (d) document the optional in-plugin `allow-from` client-source access list (child 2) as the recommended way to restrict a local-use mirror to known ranges, instead of hand-authored firewall-section rules, noting it makes the node non-public

## Implementation Steps

The umbrella is closed by closing its children and writing the deployment doc.
Recommended order (foundational/independent first):

0. **spec-dns-server-harness** (PREREQUISITE, not a child) — extract geodns's DNS-server primitives to `internal/core/dnsserver` and migrate geodns; must land before child 2 so as112 consumes the harness instead of importing geodns.
1. **spec-as112-1-iface-address-registry** — smallest, no dependencies, the foundation child 2 needs. (May run in parallel with step 0; they don't touch the same code.)
2. **spec-as112-2-dns-server** — the DNS plugin itself, depends on child 1's registry AND the harness from step 0.
3. **spec-as112-3-bgp-integration** — worked config + end-to-end test, depends on both prior children existing and working.
4. **`docs/guide/as112.md`** — written last, once real examples from the closed children exist.

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + the relevant child |
| 2. Audit | Per-child Files/Tests |
| 14. Present summary | Roll up child summaries into one umbrella learned summary (`plan/learned/NNN-as112-0-umbrella.md`), following the `cp-survival-0-umbrella` precedent |

## Known Limitations
- Two RFC 7534 SHOULDs are not software-enforceable and are documented only,
  not coded: dedicating the host to AS112 purpose (§3.3), and community
  notification before installing a globally-reachable node (§3.2/§5).
- Only one address per service per family is configured (192.175.48.1,
  192.31.196.1, 2620:4f:8000::1, 2001:4:112::1) — the secondary IANA
  Direct-Delegation addresses (BLACKHOLE-1/BLACKHOLE-2, .48.6/.48.42 and IPv6
  equivalents) are out of scope per explicit user confirmation.
- NSID (RFC 5001 EDNS0 option) — the more general RFC-recommended mechanism for
  identifying which anycast instance answered a query ("Where software
  implementations support it, operational data should also be carried using
  NSID," rfc7534 §3.5) — is deliberately deferred. The HOSTNAME.AS112.NET/ARPA
  TXT zone with an operator-configured `hostname` leaf (spec-as112-2) is the v1
  mechanism for node identification; NSID could be a future addition.
- The BGP-side wiring is documentation + a worked example, not a new dedicated
  as112 BGP config surface — operators configure ordinary `update` blocks.
  This was an explicit design choice (see spec-as112-3 Alternatives) to avoid
  new cross-component config synthesis with no precedent in ze.

## Design Insights
- Nearly the entire feature is composition of existing, already-tested ze
  mechanisms (BGP communities, `local-as` override, watchdog/healthcheck). The
  only genuinely new code is the DNS plugin and a small generic iface registry
  — both narrowly scoped and each independently useful beyond this feature.
- **H3 — host addresses vs covering prefixes (do not conflate).** Two distinct
  object types:
  - **Host addresses** bound on `lo` (child 1/child 2), each a single-host
    prefix: 192.175.48.1/32, 192.31.196.1/32, 2620:4f:8000::1/128,
    2001:4:112::1/128 — the anycast catcher addresses the DNS server listens on.
  - **Covering service prefixes** announced by BGP (child 3, RFC 7534 §3.4):
    192.175.48.0/24 and 2620:4f:8000::/48 (Direct Delegation);
    192.31.196.0/24 and 2001:4:112::/48 (DNAME Redirection). These, NOT the
    /32,/128 host addresses, are the NLRI in the operator's `update` blocks.
    Announcing the /32,/128 host addresses would be wrong (widely filtered, not
    what §3.4 originates). Child 3's worked example spells out the four /24,/48
    prefixes explicitly; child 2 registers the four /32,/128 host addresses.
- **M5 — `asn.local 112 replace-as` is a global-routing foot-gun.** Applied to a
  publicly-peered group it injects this node into the *global* AS112 anycast
  system with no RFC 7534 §3.2/§5 coordination (item #14, not enforceable in
  software). Default is the operator's own ASN (local-use mirror, A-1); the
  override is per-group opt-in. `docs/guide/as112.md` MUST carry a hard warning,
  and child 3 adds an advisory doctor check when `replace-as 112` is set on a
  group holding eBGP sessions to non-private ASNs (see child 3 R-3).

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| `docs/features.md`, `docs/comparison.md`, `docs/guide/command-reference.md`, `docs/architecture/api/commands.md`, and `docs/plugin-development/metrics.md`/`docs/features/plugins.md` all need a new AS112 entry per the children's Documentation Update Checklist templates | Direct precedent (geodns, as112's own sibling plugin) has no entry in 4 of those 5 docs — they are not maintained per-plugin/per-command at that granularity in practice; only `docs/features.md` and `docs/guide/plugins.md`/`docs/plugin-overview.md` are | Verified by grepping each doc for "geodns" before forcing an artificial entry into docs that don't follow that granularity (spec-as112-2's Documentation Update Checklist) | Avoided adding inconsistent, out-of-pattern doc entries; documented the verified determination in each child's checklist instead of silently skipping |
| `docs/guide/as112.md` would naturally be written last, once all three children were independently closed (per this umbrella's own Implementation Steps ordering) | Neither spec-as112-2 nor spec-as112-3 had written it by the time spec-as112-3 reached its documentation phase, since spec-as112-2's own checklist explicitly deferred it here. spec-as112-3 wrote the whole file (config reference, worked BGP example, and this umbrella's RFC Compliance Mapping) in one pass rather than waiting for a separate final step | Discovered when checking `docs/guide/as112.md`'s existence while starting spec-as112-3's documentation work | No functional impact — the file's sections still trace to the spec that owns their content; this umbrella's closure work is now largely just verification, not first-authoring |

## Implementation Audit
Roll-up of the three child specs' own Implementation Audits (each filled in full in its own spec file):

| Child | Requirements | ACs | Tests | Files | Status |
|-------|-------------|-----|-------|-------|--------|
| spec-as112-1-iface-address-registry | Done | Done | Done | Done | Content-complete; git closure deferred (see below) |
| spec-as112-2-dns-server | Done | Done (16/16) | Done (25 unit/boundary + 8 functional) | Done (73 items, 0 skipped) | Content-complete; blocked only on a scratch-file cleanup pending user action (see spec's own Checklist note) |
| spec-as112-3-bgp-integration | Done | Done (9/11 fully; 2 partial, pre-authorized deferral) | Done (10 tests, all passing) | Done (37 items, 35 done + 2 partial) | Content-complete |

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| AS112 node answers correctly and is reachable only where intended | child 2 functional/unit tests + Linux/Docker integration tests | `internal/plugins/as112/integration_linux_test.go` (6 tests, real port-53 bind), `test/plugin/as112-{enable,health,disable}.ci` |
| Route never advertised while DNS is unhealthy | child 3 functional + interop tests | `test/plugin/as112-healthcheck-announce.ci`, `as112-probe-anycast-not-loopback.ci` |
| Correct community and origin-AS override, confirmed on the real wire | child 3 interop tests | `test/interop/scenarios/as112-origin-as-frr/`, `as112-community-frr/` |
| Zone-boundary and RFC-compliance correctness holds under adversarial review, not just the happy path | 2 independent adversarial review rounds (child 2) | 10 real bugs found and fixed, documented in spec-as112-2's Mistake Log and Review Gate |

## Review Gate
### Final status
- [x] All three child specs are content-complete (implementation, tests, docs all done and verified)
- [ ] All three child specs formally git-closed (two-commit close per `ai/rules/planning.md`) — commit script prepared; commits are user-triggered per this repo's git-safety rules, not something this session runs unilaterally
- [x] Deployment doc (`docs/guide/as112.md`) passes `make ze-doc-test` (confirmed: `Documentation tests PASSED`, tmp/lint/umbrella-doctest2.log)

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `docs/guide/as112.md` | Yes | Read directly; contains config reference, CLI commands, BGP worked example, full RFC Compliance Mapping (19 items), Known Limitations |
| `docs/features.md` (modified) | Yes | AS112 row added |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|------------------|
| AC-1 | spec-as112-1 closed (content) | `internal/component/iface/address_owner.go` + tests, 3 review rounds documented in spec-as112-1's own Review Gate |
| AC-2 | spec-as112-2 closed (content) | `internal/plugins/as112/`, 2 review rounds, 10 bugs found/fixed, documented in spec-as112-2's own Review Gate |
| AC-3 | spec-as112-3 closed (content) | Worked example + 10 tests all passing, 5 findings found/fixed, documented in spec-as112-3's own Review Gate |
| AC-4 | Deployment doc | `docs/guide/as112.md`, `make ze-doc-test` PASSED |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| See each child spec's own Wiring Verified table | — | All child wiring tests pass |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|----------------------------------|------------------|----------|
| `make ze-doc-test` (drift + anchors + YANG/handler contract) | Full run | PASSED (tmp/lint/umbrella-doctest2.log) — includes confirming `ze-as112:health`/`ze-show:as112` are correctly registered, source-anchored, and validated against the YANG/handler contract |
| Doc-drift: interop scenario count | `docs/DESIGN.md:792` | Fixed (66→68, reflecting the 2 new AS112 interop scenarios); re-verified clean |

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-4 all demonstrated (each child content-complete + deployment doc); git closure of the 3 children still pending (user-triggered commits)
- [x] End-to-End User Stories: every story has a working path and a passing test
- [x] Wiring Test table complete — every row has a concrete test name, none deferred
- [x] `make ze-test` passes (lint + all ze tests) — scoped verification passes (each child's own test suite green); full-repo run blocked by an unrelated concurrent session's in-progress ospf work, now logged in `plan/known-failures.md` and scoped per `ai/rules/git-safety.md`'s Known-Red Full Verify guidance (see spec-as112-2's Checklist note)
- [x] Documentation Update Checklist answered Yes/No with source evidence (each child's own checklist)

### TDD
- [x] Tests written (all three children)
- [x] Tests FAIL (paste output) — see each child's own Review Gate for regression-test-fails-before-fix evidence
- [x] Tests PASS (paste output) — see each child's own Pre-Commit Verification
- [x] Boundary tests for all numeric inputs — spec-as112-2 (hostname length, 512-octet response budget); N/A for spec-as112-1/3 (no new numeric surface)
- [x] Functional tests for end-to-end behavior — all three children
- [x] Interop tests for protocol features — spec-as112-3 (2 FRR-backed scenarios)

### Completion (BLOCKING — before ANY commit)
- [ ] All three child specs closed — content-complete; git closure (two-commit close) prepared in this pass but still user-triggered (the user runs the generated commit script); spec-as112-2 additionally has 2 (of 4) pending scratch-file cleanups blocking a fully clean final test run, blocked at the tool layer, user asked to run the `rm` themselves
- [x] Implementation Summary filled (roll-up of child summaries, above)
- [x] Write learned summary to `plan/learned/1035-as112-0-umbrella.md`
