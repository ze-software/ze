# Spec: AS112 Anycast DNS Service (Umbrella)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-06-30 |

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
| `spec-as112-2-dns-server.md` | The `internal/plugins/as112/` DNS server plugin | spec-as112-1 |
| `spec-as112-3-bgp-integration.md` | BGP `update`-block wiring, healthcheck probe, worked config, end-to-end test | spec-as112-1, spec-as112-2 |

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
  → Constraint: 192.31.196.1 / 2001:4:112::1 serve only EMPTY.AS112.ARPA; the as112 node itself never needs to understand DNAME (§6).
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
  `config_apply.go:94-107`'s `desiredState()`, so an address added this way is
  reconciled away (removed as "stray") on the very next config-apply pass
  (`config_apply.go:778-813`). This is why child 1 must be a real registry
  consulted by `desiredState()`, not a startup-time RPC call.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/geodns/register.go` - plugin registration: `ConfigRoots: ["service"]`, `InProcessConfigVerifier`, `RunEngine`, `ConfigureEngineLogger`/`ConfigureMetrics` callbacks, `DoctorChecks` (lines 24-66); doctor check `geodns-listen-capability` (lines 47-55) warns when a configured listener can't bind (privileged port).
- [ ] `internal/plugins/geodns/server.go` - DNS server lifecycle: `bind()` opens one UDP + one TCP listener per endpoint via `net.ListenConfig` (lines 360-385); `apply()` reconciles listeners only on endpoint-set change, computed via `endpointSig()` (lines 322-358); single `dns.ServeMux` handler reads an atomically-published `resolverState` snapshot per query (lines 254-301); `stopAll()` drains in-flight handlers with a 5s timeout (lines 393-409).
- [ ] `internal/plugins/geodns/state.go` - `resolverState` (config + matcher + SOA serial) published via `sync/atomic.Pointer`; handler reads the snapshot lock-free (lines 7-32).
- [ ] `internal/component/iface/config.go` - `parseUnits()` merges a loopback unit's IPv4 and IPv6 leaf-list addresses into one `unitEntry.Addresses` slice (lines 875-954, merge at 908-914).
- [ ] `internal/component/iface/config_apply.go` - `desiredState()` builds the per-OS-interface-name desired-address map purely from parsed YANG config (lines 94-107); reconciliation calls `AddAddress`/`RemoveAddress` to converge kernel state to that map — any kernel address not present in `desiredState()` is treated as stray and removed (lines 778-813).
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
  `group` `update` blocks referencing the four AS112 prefixes with a `community`
  and `watchdog` block, plus a `healthcheck` probe definition (child 3).

### Transformation Path
1. as112 plugin config is parsed and validated (child 2); on `enabled: true` the
   plugin registers its four fixed addresses as an owned set against the new
   iface registry (child 1) instead of requiring operator-typed loopback config.
2. iface's `desiredState()` (`config_apply.go:94-107`) merges the plugin-owned
   addresses with YANG-config addresses; reconciliation adds them to the kernel
   loopback interface (`config_apply.go:778-793`, `manage_linux.go:204-220`).
3. The as112 DNS server (child 2) binds UDP+TCP listeners on the four addresses
   plus loopback, port 53, and answers the AS112 zones with static SOA/NS-only
   data (or NXDOMAIN out of zone), authoritative-only (no recursion).
4. A `healthcheck` probe (child 3) periodically queries the as112 DNS server.
   On reaching state UP it dispatches `request bgp watchdog announce <group>`;
   on DOWN it dispatches withdraw (existing healthcheck/watchdog mechanism,
   unmodified).
5. The watchdog plugin announces/withdraws the operator's `update`-block routes
   (carrying the chosen community and optional `asn.local 112 replace-as`
   override) to/from the peers in the targeted group(s) — existing watchdog
   mechanism, unmodified.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| as112 plugin ↔ iface (child 1) | new Go-level address-ownership registration API, consulted by `desiredState()` | [ ] |
| iface ↔ kernel | existing netlink `AddAddress`/`RemoveAddress` reconciliation, unchanged | [ ] |
| DNS client ↔ as112 plugin | UDP/TCP port 53 on the four anycast addresses, miekg/dns | [ ] |
| healthcheck probe ↔ as112 plugin | DNS query over loopback (or one of the anycast addresses), existing healthcheck probe mechanism | [ ] |
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
| 1 | rfc7534 §3.5 | MUST answer authoritatively for each delegated zone | Met | spec-as112-2 |
| 2 | rfc7534 §3.5 | MUST NOT include records beyond SOA/NS in Direct Delegation zones | Met | spec-as112-2 |
| 3 | rfc7534 §3.5 | MUST NOT host site's own RFC1918 records on the AS112 nameserver | Met by design (plugin only ever serves the fixed static empty-zone data, never site records) | spec-as112-2 |
| 4 | rfc7534 §3.3 | SHOULD support cloned loopback / multiple loopback addresses | Met (existing iface capability) | spec-as112-1 |
| 5 | rfc7534 §3.3 | SHOULD dedicate the host to AS112 purpose | **Not met — not software-enforceable.** Deployment recommendation only | Documented in `docs/guide/as112.md`; listed in Known Limitations |
| 6 | rfc7534 §3.3 | SHOULD order startup: loopback → DNS → routing | Met functionally, by a stronger mechanism — the watchdog stays withdrawn until the healthcheck probe actively confirms the DNS server is answering correctly, independent of process start order | spec-as112-3 |
| 7 | rfc7534 §3.3 | SHOULD NOT advertise the service prefix while addresses unconfigured or DNS not running | Met (healthcheck → watchdog) | spec-as112-3 |
| 8 | rfc7534 §3.4 | SHOULD restrict outbound advertisement to a prefix filter permitting only the service prefixes + an AS_PATH filter matching only locally-originated routes | **Conditionally met** — true if the operator dedicates the target peer-group to AS112 only (recommended, worked example in docs); not enforceable if the operator reuses a general-purpose/transit peer-group for other routes too (that is the operator's own BGP policy, outside this feature's scope) | spec-as112-3 (docs); flagged as R-1 below |
| 9 | rfc7534 §3.5 | SHOULD run authoritative-only (recursion disabled) | Met by design | spec-as112-2 |
| 10 | rfc7534 §3.5 | SHOULD keep HOSTNAME.AS112.{NET,ARPA} TXT answers within 512 octets without EDNS0 | Met — boundary-tested | spec-as112-2 |
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
| A-3 | `desiredState()` (`config_apply.go:94-107`) can be extended to merge a second, plugin-sourced map without changing its existing YANG-only call sites | Read of `config_apply.go:94-813`; no other caller relies on `desiredState()` being YANG-only | child 1 needs a larger refactor than anticipated | child 1 unit test exercising `desiredState()` with both a YANG address and a registered address present simultaneously | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Operator reuses a shared/transit peer-group for the AS112 `update` block, defeating RFC 7534 §3.4's "no transit" SHOULD (item #8 above) | `show bgp` on that group shows non-AS112 prefixes alongside the AS112 routes | `docs/guide/as112.md` recommends a dedicated peer-group with a worked example; doctor check (child 3, advisory severity) warns if a watchdog-gated AS112 route shares a group with non-AS112 `update` blocks |
| R-2 | A weak healthcheck probe (e.g. bare TCP connect) gives a false UP while the DNS server answers incorrectly or is serving stale/empty data, defeating item #7 | Functional test catches probe-content gap during review | child 3's worked example probe issues a real DNS query against a known AS112 zone, not just a port-open check |
| R-3 | A future plugin double-registers the same loopback address via child 1's registry, causing an ownership conflict | `go test -race` / unit test on the registry rejecting/erroring a conflicting registration | child 1 AC requires the registry to reject (not silently overwrite) a conflicting registration for an address already owned by a different registrant |

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
- `docs/guide/as112.md` - RFC compliance mapping, deployment guidance (dedicated peer-group recommendation, host-dedication and community-notification recommendations), worked config example

## Implementation Steps

The umbrella is closed by closing its children and writing the deployment doc.
Recommended order (foundational/independent first):

1. **spec-as112-1-iface-address-registry** — smallest, no dependencies, the foundation child 2 needs.
2. **spec-as112-2-dns-server** — the DNS plugin itself, depends on child 1's registry being available.
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

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Implementation Audit
(Filled at closure — roll-up of child audits.)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| AS112 node answers correctly and is reachable only where intended | child 2 + child 3 functional/interop tests | see children |
| Route never advertised while DNS is unhealthy | child 3 interop test | see child 3 |

## Review Gate
### Final status
- [ ] All three child specs closed
- [ ] Deployment doc passes `make ze-doc-test`

## Pre-Commit Verification
(Filled at closure — roll-up of child verification.)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated (each child closed + deployment doc)
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)

### Completion (BLOCKING — before ANY commit)
- [ ] All three child specs closed
- [ ] Implementation Summary filled (roll-up of child summaries)
- [ ] Write learned summary to `plan/learned/NNN-as112-0-umbrella.md`
