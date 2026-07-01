# Spec: AS112 DNS Server Plugin

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-as112-1-iface-address-registry, spec-dns-server-harness |
| Phase | - |
| Updated | 2026-07-01 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-as112-0-umbrella.md` - RFC compliance mapping, cross-cutting decisions
4. `plan/spec-as112-1-iface-address-registry.md` - the address registry this plugin calls into
5. `plan/spec-dns-server-harness.md` - the `internal/core/dnsserver` harness this plugin CONSUMES (listener lifecycle, EDNS0/client-IP, authoritative wrapper, `IP_FREEBIND`, CIDR matcher)
6. `rfc/short/rfc7534.md`, `rfc/short/rfc7535.md`, `rfc/short/rfc1035.md` - protocol requirements
7. `internal/plugins/geodns/` - reference for zone/answer POLICY only (NOT imported; the shared server machinery now lives in `internal/core/dnsserver`)

## Task

Build `internal/plugins/as112/`: a system plugin that, when enabled, registers
ownership of four fixed anycast host addresses (**192.175.48.1/32,
192.31.196.1/32, 2620:4f:8000::1/128, 2001:4:112::1/128** — host prefix lengths,
not the /24,/48 covering prefixes that BGP announces; see umbrella finding H3)
against the registry from `spec-as112-1-iface-address-registry.md` and serves the
AS112 sink zones authoritatively on those addresses (port 53, UDP+TCP), per RFC
7534 §2.2/§3.5 and RFC 7535 §2. No operator-typed IP address anywhere in this
plugin's config — the four addresses are fixed Go constants, not configurable.
The only operator inputs are: enable the service, optionally restrict to one
address family, and optionally set a `hostname` identifier string surfaced in the
HOSTNAME.AS112.NET/ARPA TXT answers so operators can tell which anycast
instance answered a given query.

This plugin also owns three pieces of new behavior surfaced by the design review:
- **`IP_FREEBIND` on the listener sockets (finding B2).** as112 constructs the
  `internal/core/dnsserver` harness (from `spec-dns-server-harness`) with
  `Options{Freebind: true}`, so `bind()` does not fail with `EADDRNOTAVAIL` when
  the anycast address has not yet been applied to `lo` by iface reconciliation.
  The freebind `net.ListenConfig.Control` hook lives in the harness (default
  OFF); as112 opts in — no bespoke bind code in this plugin.
- **A shell-invokable health command for the healthcheck probe (finding M4).**
  A `ze` subcommand (e.g. `ze ... as112 health`, exit 0 iff a real authoritative
  query to an anycast service address returns the expected AS112 answer) that
  child 3's healthcheck probe calls. This is needed because `dig` is not on the
  gokrazy appliance and `ze resolve dns` uses the local recursive resolver and
  cannot target a specific server (`ze-resolve-cmd.yang:20-39`). Putting it here
  keeps child 3 docs+tests-only and keeps new code in the plugin that owns the
  service.
- **An optional in-plugin client-source access list (`allow-from`).** A
  `leaf-list allow-from { type zt:ip-prefix; }` (the same type geodns uses at
  `ze-geodns-conf.yang:220-221`). When **empty/unset**, the node answers every
  source (default AS112 public-sink behavior). When **non-empty**, only queries
  whose source IP is contained in one of the prefixes are answered; all others
  are silently **dropped** (no response — the firewall-equivalent, and the
  choice that gives zero amplification to spoofed out-of-range sources). This
  keeps access control *inside the plugin* instead of requiring the operator to
  hand-author firewall rules matching port 53 on four v4/v6 anycast addresses in
  the firewall section (harder to get right). Enforcement sits in the query
  handler after the client IP is resolved via the harness's `ClientIP`/
  `remoteAddr` (from `internal/core/dnsserver`) and reuses the harness's
  family-aware longest-prefix matcher (`netip.Prefix.Contains`) — both extracted
  from geodns by `spec-dns-server-harness`, not re-implemented here.
  - **CRITICAL interaction with H1/M4:** on-box/loopback sources are ALWAYS
    implicitly permitted regardless of `allow-from`, so the `ze … as112 health`
    probe (which queries an anycast address from the local host) is never
    blocked. Without this carve-out, setting `allow-from` would silently make the
    healthcheck fail and withdraw the route.
  - **Deployment note:** setting `allow-from` makes the node non-public — correct
    for a local-use mirror (umbrella A-1 default), wrong for a globally-reachable
    AS112 contributor (leave it empty there). It violates no RFC MUST: RFC 7534
    requires answering authoritatively for the *zones*, not for every *source*.

## Required Reading

### Architecture Docs
- [ ] `ai/patterns/plugin.md` - register.go / SDK protocol / doctor checks pattern
  → Constraint: same `RunEngine(net.Conn) int` entry point shape as geodns; same `ConfigureEngineLogger`/`ConfigureMetrics` callback wiring.
- [ ] `ai/rules/plugin-self-containment.md`
  → Constraint: this plugin owns its own YANG, doctor check, CLI/show command; it never reaches into BGP config (BGP wiring is spec-as112-3, documentation-only from this plugin's perspective).
- [ ] `ai/rules/buffer-first.md`
  → Constraint: DNS answers are built via the same `miekg/dns` message construction geodns already uses; no new allocation-heavy string building — reuse `internal/core/textbuf` where geodns does (per `plan/learned/992-geodns-1-config.md`'s "no `+` string concatenation" gotcha).

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7534.md` - zone list, address list, SOA/NS-only content rule, authoritative-only requirement, 512-octet TXT constraint
  → Constraint: exact zones — 10.in-addr.arpa; 16.172.in-addr.arpa .. 31.172.in-addr.arpa (16 zones); 168.192.in-addr.arpa; 254.169.in-addr.arpa; HOSTNAME.AS112.NET; HOSTNAME.AS112.ARPA.
  → Constraint: "There should be no other resource records included in this zone" (§3.5) — SOA + NS only for the reverse zones.
  → Constraint (finding M1): pin the RFC-mandated SOA parameters as Go constants and TEST them — refresh 1W, retry 1M, expire 1W, negative-cache/minimum TTL 1W, $TTL 1W (§3.5). The Direct-Delegation zones use MNAME/NS naming the canonical AS112 servers (PRISONER.IANA.ORG etc.); EMPTY.AS112.ARPA uses MNAME/NS `BLACKHOLE.AS112.ARPA` (RFC 7535 §8.2). `zones_test.go` must assert the SOA timers and MNAME/NS RDATA, not just the NODATA/NXDOMAIN shape — otherwise the plugin can pass every test while emitting non-conformant zone content.
  → Constraint: HOSTNAME.AS112.{NET,ARPA} TXT response must fit a 512-octet UDP datagram without requiring EDNS0 (§3.5).
- [ ] `rfc/short/rfc7535.md` - EMPTY.AS112.ARPA zone, the two DNAME-redirection addresses, DNAME support never required on the node
  → Constraint: EMPTY.AS112.ARPA is SOA+NS-only, identical shape to the Direct-Delegation zones; the plugin never parses or emits a DNAME record itself.
- [ ] `rfc/short/rfc1035.md` - message structure, negative-answer semantics (already the authority geodns itself documents and follows)
  → Constraint: in-zone name, no record of queried type → NOERROR + SOA-in-Authority (NODATA), not NXDOMAIN. Out-of-zone name → NXDOMAIN. Identical to geodns's existing `answerQuestions` behavior (`internal/plugins/geodns/server.go`).

**Key insights:**
- Unlike geodns, every query answered by this plugin gets the **same static
  answer** regardless of the querying client — there is no host-set/source-CIDR
  matching needed at all. The plugin is structurally simpler than geodns: no
  `source.go` equivalent, no per-client config.
- The four addresses (192.175.48.1, 192.31.196.1, 2620:4f:8000::1,
  2001:4:112::1) are Go constants, never operator-typed, per explicit user
  instruction. "Enabled: true" alone is sufficient; the only other operator
  inputs are an optional ipv4-only/ipv6-only restriction, the `hostname`
  string, and the optional `allow-from` client-source access list. Note the
  `allow-from` prefixes ARE operator-typed CIDRs — that is a client-source ACL,
  not a service *listen* address, so it does not reintroduce the "no operator IP
  entry" concern (which is about where the service binds, still fixed constants).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/geodns/register.go` - plugin registration shape to mirror: `Name`, `Description`, `Features: "yang"`, `YANG`, `ConfigRoots: ["service"]`, `InProcessConfigVerifier`, `RunEngine`, `ConfigureEngineLogger`/`ConfigureMetrics`, `DoctorChecks` (lines 24-66).
- [ ] `internal/plugins/geodns/server.go` - `bind()`/`apply()`/`stopAll()` listener lifecycle (lines 322-409); single `dns.ServeMux` handler reading an atomic state snapshot per query (lines 254-301); SOA/NS synthesis with glue (lines 159-240).
- [ ] `internal/plugins/geodns/state.go` - `resolverState` published via `sync/atomic.Pointer`, read lock-free by the query handler (lines 7-32).
- [ ] `internal/plugins/geodns/doctor.go` - `checkGeoDNSListenCapability` (lines 30-67, probe at 93-112): warns when a configured listener cannot bind (privileged port, missing capability).
- [ ] `internal/plugins/geodns/metrics.go` - Prometheus registration shape: request/response counters by zone+qtype(+rcode), latency histogram, listener-up gauge, config-reload counter (lines 15-69).
- [ ] `internal/plugins/geodns/show.go` - `show geodns` reads the same atomic snapshot the server reads, so status never drifts from what's actually served (lines 19-41).
- [ ] `plan/spec-as112-1-iface-address-registry.md` - the `RegisterOwnedAddresses`/`UnregisterOwnedAddresses` API this plugin's `OnConfigure` calls instead of requiring operator-typed loopback config.
- [ ] `internal/core/bgp/attribute/community.go`, `internal/component/bgp/yang/ze-bgp-conf.yang` - confirms this plugin has zero BGP-side responsibility; BGP wiring is documentation-only (spec-as112-3), not code in this plugin.

**Behavior to preserve:**
- geodns itself is completely unmodified by this spec — the as112 plugin is a
  new, independent plugin, not a change to geodns.
- The existing `desiredState()`/reconciliation behavior for interfaces with no
  as112 registration is unchanged (covered by spec-as112-1's preservation
  guarantee).

**Behavior to change:**
- None to existing code beyond the registry merge point already specified in
  spec-as112-1. This spec is purely additive: a new plugin directory.

## Data Flow (MANDATORY)

### Entry Point
- Config commit: `service/as112/enabled` (plus optional `address-family` and
  `hostname` leaves) under the as112 plugin's own YANG root.

### Transformation Path
1. Config parsed and validated (`InProcessConfigVerifier`, mirrors geodns's `verifyGeoDNSConfig`).
2. On `enabled: true`, `OnConfigure` calls `iface.RegisterOwnedAddresses("lo", "as112", <the fixed 4 /32,/128 host addresses, filtered by address-family>)` — which now also fires iface's reconcile-trigger (spec-as112-1, finding B1) so the addresses land on `lo` in this same op. On `enabled: false` (or plugin shutdown), calls `iface.UnregisterOwnedAddresses("as112")`.
3. The DNS server binds UDP+TCP listeners on the four (or fewer, if family-restricted) host addresses plus `127.0.0.1`/`::1` for local diagnostics, port 53. Sockets set `IP_FREEBIND` via a `net.ListenConfig.Control` hook (finding B2) so bind succeeds even if reconciliation has not yet applied the address to `lo`.
4. If `allow-from` is non-empty, the handler first checks the client source IP (from `remoteAddr(w)`) against the compiled allow-list (`netip.Prefix.Contains`, reusing `geodns/source.go:31-38`); on-box/loopback is always permitted; out-of-range queries are dropped with no response and `ze_as112_dns_denied_total` incremented. Otherwise (empty allow-from) every source proceeds. Each permitted query is then answered from a precomputed, static zone table (no per-client logic): SOA+NS for the 19 reverse zones and EMPTY.AS112.ARPA; SOA+NS+TXT for HOSTNAME.AS112.NET/ARPA; NXDOMAIN outside all served zones; recursion always refused. The same static table answers on every bound address (finding M2 — no per-address zone restriction). SOA parameters and MNAME/NS names are the RFC-mandated fixed values (finding M1).
5. A `ze` health command (finding M4) issues an authoritative query to an anycast service address and exits 0 iff the expected AS112 answer comes back; child 3's healthcheck probe calls it.
6. `show as112` and Prometheus metrics read the same atomic snapshot the server reads.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ plugin | YANG config tree → `parseConfig()`, mirrors geodns | [ ] |
| Plugin ↔ iface registry | direct Go call per spec-as112-1's API | [ ] |
| DNS client ↔ plugin | UDP/TCP port 53, `miekg/dns` | [ ] |
| Plugin ↔ engine (logging/metrics/doctor/show) | existing plugin SDK callbacks, mirrors geodns | [ ] |

### Integration Points
- `internal/plugins/geodns/server.go` lifecycle shape - structural precedent, not a code dependency
- `internal/component/iface` (spec-as112-1) `RegisterOwnedAddresses`/`UnregisterOwnedAddresses` - called from this plugin's `OnConfigure`

### Architectural Verification
- [ ] No bypassed layers (addresses flow through spec-as112-1's registry, not a direct netlink call from this plugin)
- [ ] No unintended coupling (this plugin does NOT import geodns — a sibling plugin, forbidden by `ai/rules/plugin-design.md:133`; the shared DNS-server machinery is consumed from the core `internal/core/dnsserver` harness instead)
- [ ] No duplicated functionality (server lifecycle, EDNS0/client-IP, authoritative wrapper, and CIDR matcher come from `internal/core/dnsserver`; `miekg/dns` is vendored via `go.mod` `github.com/miekg/dns v1.1.72`; there is no `internal/component/dns` package — that earlier reference was wrong)
- [ ] Zero-copy preserved where applicable (static zone answers can be precomputed once per config-reload, not rebuilt per query)
- [ ] Registration over hardcoding (plugin registers via the existing plugin registry like any other; no special-casing elsewhere)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Four fixed addresses (192.175.48.1, 192.31.196.1, 2620:4f:8000::1, 2001:4:112::1) are the complete address set — no BLACKHOLE-1/2 secondary addresses | User confirmed "4 addresses total" earlier in this conversation; matches umbrella A-2 | Real-world AS112 participation expects 3 addresses per family for the Direct Delegation service | re-confirm with user at WRITE-gate review (umbrella A-2 covers the same point) | unvalidated |
| A-2 | This plugin is deployed in-process (`internal` per `plan/learned/821-plugin-internal-keyword.md`), since it depends on spec-as112-1's Go-level registration API | spec-as112-1 R-3 restricts the registry to in-process callers | Operator tries to run as112 as an external/forked plugin and address registration silently fails or errors | doctor check fires / config verifier rejects external deployment mode for this plugin | unvalidated |
| A-3 | The **combined** `hostname`+`facility`+`location` TXT strings, plus the fixed SOA/NS overhead, fit a 512-octet UDP response without EDNS0 (finding M3 — the budget is on the assembled response, not any single field) | RFC 7534 §3.5 requirement | An operator sets all three fields near max and the total exceeds 512, forcing TC=1 truncation | the three YANG `length` constraints are jointly sized so the worst-case assembled response is ≤512, verified by `TestHostnameTXT_TotalResponseUnder512` (asserts size AND TC=0) — not a per-field bound | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Port 53 cannot be bound (privilege/capability missing) and the service silently fails to serve | doctor check `as112-listen-capability` fires (code `doctor-as112-port-unavailable`, mirrors `geodns-listen-capability`) | doctor check + `show as112` reports listener-down state; documented in `docs/guide/as112.md`. Note: `IP_FREEBIND` (finding B2) fixes the *address-not-yet-present* case but NOT a genuine privilege/capability failure — the doctor check still covers the latter |
| R-2 | A static zone table is computed once at config-reload and never refreshed, causing the SOA serial to go stale across long uptimes | SOA serial mode mirrors geodns's existing `auto-epoch`/`auto-datetime`/`fixed` modes (`plan/learned/993-geodns-2-server.md`) — same gotchas apply, no new ones introduced | reuse geodns's already-solved serial-mode design verbatim |
| R-3 | Recursion-disabled enforcement regresses if a future change to the shared `miekg/dns` handler pattern (copied from geodns) accidentally enables recursive lookups | unit test explicitly asserting `RecursionAvailable=false` on every response and that no upstream resolver client exists in this plugin's dependency graph | `TestAS112NeverRecurses` in the TDD plan below |
| R-4 | `IP_FREEBIND` lets the server bind an address that is never actually applied to `lo` (e.g. child 1's reconcile-trigger regresses), so it answers on loopback but the anycast address is unreachable — yet the route could still be announced | child 3's anycast-address healthcheck probe (finding H1) goes/stays DOWN because the query to the anycast address times out, so the route is withdrawn | freebind is paired with the anycast-address probe (H1), which is the real reachability gate; freebind only removes the startup ordering race, it is not a substitute for the probe |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `service/as112/enabled = true` committed | → | `OnConfigure` → `iface.RegisterOwnedAddresses` → DNS server `bind()` | `test/parse/as112-config.ci` (config accepted) + `test/plugin/as112-enable.ci` (server actually listening) |
| DNS query for `1.0.10.in-addr.arpa` sent to `192.175.48.1:53` | → | static zone answerer | `test/plugin/as112-dns-zones.ci` |
| `ze … as112 health` run against the running service | → | health command → authoritative query to an anycast address → exit code | `test/plugin/as112-health.ci` (finding M4) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|--------------------|
| AC-1 | `enabled: true` committed | The four canonical addresses (filtered by any address-family restriction) are registered via spec-as112-1's API; no operator-typed IP required |
| AC-2 | Query for any name within a Direct-Delegation reverse zone (e.g. `1.0.10.in-addr.arpa`) | NOERROR, empty Answer, zone SOA in Authority |
| AC-3 | Query for any name within `empty.as112.arpa` | NOERROR, empty Answer, zone SOA in Authority |
| AC-4 | Query for `hostname.as112.net` TXT (and `.arpa`) with `hostname`, `facility`, and `location` all set to their max-length values | NOERROR; TXT answer includes the `hostname` string as a distinct TXT string; the **assembled UDP response** (all TXT strings + NS + SOA) is ≤ 512 octets with TC=0 (not truncated), no EDNS0 (finding M3 — the budget is on the total response, not any single field) |
| AC-5 | Query for a name outside every served zone | NXDOMAIN |
| AC-6 | Any query, any zone | `RecursionAvailable` is always false in the response header; the plugin never issues an upstream query |
| AC-7 | as112 enabled on a privileged port the process cannot bind | `ze doctor` reports the check `as112-listen-capability` with diagnostic code `doctor-as112-port-unavailable` (matching geodns's naming: check name + `doctor-…-port-unavailable` code; finding L1) |
| AC-8 | `show as112` invoked | Reports enabled state, zones served, listener addresses, registered-address-ownership state, sourced from the same atomic snapshot the DNS server reads |
| AC-9 | `enabled: false` committed after having been true | DNS server stops; `iface.UnregisterOwnedAddresses("as112")` called; addresses removed from loopback within the same disable op (reconcile-trigger, spec-as112-1) unless independently YANG-declared |
| AC-10 | `address-family: ipv4-only` set | Only the two IPv4 addresses (192.175.48.1, 192.31.196.1) are registered/bound; IPv6 addresses are not |
| AC-11 | Server `bind()` runs before iface has applied the anycast address to `lo` | Bind succeeds (not `EADDRNOTAVAIL`) because the listener sockets set `IP_FREEBIND` via `net.ListenConfig.Control` (finding B2) |
| AC-12 | `ze … as112 health` run while the service answers correctly on an anycast address / while it does not | Exit 0 when a real authoritative query to an anycast service address returns the expected AS112 answer; non-zero otherwise. This is the command child 3's healthcheck probe calls (finding M4) |
| AC-13 | Any served zone's SOA / NS inspected | SOA carries the RFC 7534 §3.5 timers (refresh 1W, retry 1M, expire 1W, min-TTL 1W, $TTL 1W); Direct-Delegation MNAME/NS name the canonical AS112 servers; EMPTY.AS112.ARPA MNAME/NS = BLACKHOLE.AS112.ARPA (finding M1) |
| AC-14 | `allow-from` unset/empty, query from any source | Answered normally (default public-sink behavior — no access restriction) |
| AC-15 | `allow-from [ 10.0.0.0/8 ]` set; query from `10.1.2.3` vs from `203.0.113.5` | The in-range query is answered; the out-of-range query gets **no response** (silently dropped), and increments `ze_as112_dns_denied_total` |
| AC-16 | `allow-from` set to a range that does NOT include loopback; `ze … as112 health` (on-box source) run | The health query is still answered (loopback/on-box is always implicitly permitted), so the healthcheck is not broken by the access list — the critical H1/M4 interaction |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|--------------------------|
| 1 | enables the as112 service with no other config | config → `OnConfigure` → registry (spec-as112-1) → loopback addresses appear → DNS server binds → answers correctly | `test/plugin/as112-enable.ci` + `test/plugin/as112-dns-zones.ci` |
| 2 | sets a `hostname` and queries `hostname.as112.net TXT` to identify the node | config → TXT answer construction → response | `test/plugin/as112-hostname.ci` |
| 3 | disables the service | config → server stop → address deregistration → loopback addresses removed | `test/plugin/as112-disable.ci` |
| 4 | restricts the service to internal ranges by setting `allow-from` (no firewall-section rules needed) | config → compiled prefix matcher in snapshot → handler drops out-of-range sources, still serves on-box/loopback | `test/plugin/as112-allow-from.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseConfig_DefaultsEnabledFalse` | `internal/plugins/as112/config_test.go` | safe default (disabled) | |
| `TestParseConfig_AddressFamilyRestriction` | `internal/plugins/as112/config_test.go` | AC-10 | |
| `TestParseConfig_HostnameLengthBound` | `internal/plugins/as112/config_test.go` | A-3 boundary | |
| `TestZoneAnswer_ReverseZoneNoData` | `internal/plugins/as112/zones_test.go` | AC-2 | |
| `TestZoneAnswer_EmptyAS112Arpa` | `internal/plugins/as112/zones_test.go` | AC-3 | |
| `TestZoneAnswer_HostnameTXTIncludesHostname` | `internal/plugins/as112/zones_test.go` | AC-4 | |
| `TestZoneAnswer_OutOfZoneNXDOMAIN` | `internal/plugins/as112/zones_test.go` | AC-5 | |
| `TestAS112NeverRecurses` | `internal/plugins/as112/server_test.go` | AC-6 / R-3 | |
| `TestShowAS112_MatchesServerSnapshot` | `internal/plugins/as112/show_test.go` | AC-8 | |
| `TestOnConfigure_RegistersAddressesOnEnable` | `internal/plugins/as112/register_test.go` | AC-1, AC-9 (register/unregister symmetry) | |
| `TestSOA_RFCMandatedParameters` | `internal/plugins/as112/zones_test.go` | AC-13 / M1: SOA timers + MNAME/NS RDATA match RFC 7534 §3.5 / RFC 7535 §8.2 | |
| `TestHostnameTXT_TotalResponseUnder512` | `internal/plugins/as112/zones_test.go` | AC-4 / M3: max-length hostname+facility+location, assembled UDP response ≤512, TC=0 | |
| `TestListener_FreebindBindsWithoutAddress` | `internal/plugins/as112/server_test.go` | AC-11 / B2: bind succeeds with `IP_FREEBIND` when the address is absent | |
| `TestHealthCommand_ExitCodes` | `internal/plugins/as112/health_test.go` | AC-12 / M4: exit 0 when anycast answers, non-zero when down | |
| `TestAllowFrom_EmptyAnswersAll` | `internal/plugins/as112/server_test.go` | AC-14: unset allow-from answers every source | |
| `TestAllowFrom_DropsOutOfRange` | `internal/plugins/as112/server_test.go` | AC-15: in-range answered, out-of-range dropped (no WriteMsg) + denied counter | |
| `TestAllowFrom_LoopbackAlwaysPermitted` | `internal/plugins/as112/server_test.go` | AC-16: on-box/loopback source answered even when not in allow-from (H1/M4 carve-out) | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|----------------|----------------|
| `hostname` string length | 0-63 octets (DNS label limit) | 63 | n/a (empty allowed, omits the TXT string) | 64 (rejected by YANG `length` constraint) |
| assembled HOSTNAME.AS112.* UDP response (M3) | ≤ 512 octets with `hostname`+`facility`+`location` all at max length | 512 | - | > 512 must be prevented by the combined YANG `length` bounds on the three fields (tested by `TestHostnameTXT_TotalResponseUnder512`) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|---------------------|--------|
| `as112-config` | `test/parse/as112-config.ci` | config with `enabled: true` and a `hostname` parses and validates | |
| `as112-enable` | `test/plugin/as112-enable.ci` | enabling the service makes it answer on the canonical addresses | |
| `as112-dns-zones` | `test/plugin/as112-dns-zones.ci` | queries against every served zone return the RFC-correct shape (AC-2,3,5,6) | |
| `as112-hostname` | `test/plugin/as112-hostname.ci` | HOSTNAME.AS112.NET/ARPA TXT carries the configured hostname, assembled response ≤512 octets, TC=0 (AC-4) | |
| `as112-soa-content` | `test/plugin/as112-soa-content.ci` | SOA timers and MNAME/NS match the RFC-mandated values (AC-13) | |
| `as112-health` | `test/plugin/as112-health.ci` | `ze … as112 health` returns 0 when serving, non-zero when stopped (AC-12) | |
| `as112-allow-from` | `test/plugin/as112-allow-from.ci` | with `allow-from` set: in-range answered, out-of-range dropped, loopback/on-box always answered (AC-14/15/16) | |
| `as112-disable` | `test/plugin/as112-disable.ci` | disabling stops the server and deregisters addresses | |

### Interop Tests
N/A for this child — DNS wire compliance is exercised via the functional `.ci`
suite above using `miekg/dns` as the verifying client (mirrors geodns's own
test approach); cross-vendor BGP interop is owned by spec-as112-3.

## Files to Modify
- `internal/component/plugin/all/all.go` - regenerated via `make generate` to register the new as112 plugin

## Files to Create
- `internal/plugins/as112/register.go` - plugin registration, doctor check, RPC for `show as112`, `OnConfigure`
- `internal/plugins/as112/config.go` - YANG config parsing/validation (`enabled`, `address-family`, `hostname`, optional `facility`/`location` strings)
- `internal/plugins/as112/server.go` - constructs the `internal/core/dnsserver` harness with `Options{Freebind: true}` (finding B2) and an answer func over the static zone table; owns only as112's answer policy, not the listener/EDNS0 machinery (that is the harness, from `spec-dns-server-harness`)
- `internal/plugins/as112/zones.go` - static zone table: the 19 reverse zones + EMPTY.AS112.ARPA + HOSTNAME.AS112.{NET,ARPA}, SOA/NS/TXT synthesis with the RFC-mandated SOA timers and MNAME/NS names (finding M1)
- `internal/plugins/as112/state.go` - atomic snapshot (mirrors geodns's state.go)
- `internal/plugins/as112/metrics.go` - Prometheus counters/histogram/gauges (mirrors geodns's metrics.go)
- `internal/plugins/as112/show.go` - `show as112` handler
- `internal/plugins/as112/health.go` + `health_test.go` - the `ze … as112 health` command (finding M4): one-shot authoritative query to an anycast service address, shell-friendly exit code, used by child 3's healthcheck probe
- `internal/plugins/as112/doctor.go` - `as112-listen-capability` doctor check (diagnostic code `doctor-as112-port-unavailable`, matching geodns's naming; finding L1)
- `internal/plugins/as112/yang/ze-as112-conf.yang` - config schema
- `internal/plugins/as112/yang/ze-as112-cmd.yang` - `show as112` command tree + the `as112 health` command (finding M4)
- `internal/plugins/as112/yang/embed.go`, `internal/plugins/as112/yang/register.go` - YANG embedding/registration (mirrors geodns's yang/ layout)
- `internal/plugins/as112/register_test.go`, `config_test.go`, `zones_test.go`, `server_test.go`, `show_test.go`, `doctor_test.go`, `metrics_test.go`, `health_test.go` - unit tests
- `test/parse/as112-config.ci`, `test/plugin/as112-enable.ci`, `test/plugin/as112-dns-zones.ci`, `test/plugin/as112-hostname.ci`, `test/plugin/as112-soa-content.ci`, `test/plugin/as112-health.ci`, `test/plugin/as112-allow-from.ci`, `test/plugin/as112-disable.ci` - functional tests

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [x] Yes | `internal/plugins/as112/yang/ze-as112-conf.yang` — incl. `leaf-list allow-from { type zt:ip-prefix; }` (in-plugin client-source access list) |
| YANG validation constraints | [x] Yes | `hostname`/`facility`/`location` get `length` constraints; `address-family` is an `enumeration`; `allow-from` entries are `zt:ip-prefix` (same type as geodns `source/prefix`, `ze-geodns-conf.yang:220-221`) |
| YANG custom validators | [ ] No — native constraints (length, enum) are sufficient | n/a |
| CLI commands/flags | [x] Yes | `show as112` and `as112 health` (finding M4) via `internal/plugins/as112/yang/ze-as112-cmd.yang` |
| CLI grammar | [x] Yes | action-before-identifier per `ai/rules/cli-grammar.md`, mirrors `show geodns` |
| Editor autocomplete | [x] Yes | automatic for the `address-family` enum leaf |
| Functional test for new RPC/API | [x] Yes | `test/plugin/as112-enable.ci` exercises `show as112`; `test/plugin/as112-health.ci` exercises `as112 health` |
| Pipe completeness | [x] Yes | `show as112` output routes through `ApplyPipes`/`ProcessPipes` per `ai/rules/pipe-completeness.md`, mirrors `show geodns` |
| Env var registration | [ ] No — this is operational policy config, not environment config (`ai/rules/config-surface.md`) | n/a |
| Doctor check for runtime dependencies | [x] Yes | check `as112-listen-capability`; diagnostic code `doctor-as112-port-unavailable` registered in `internal/core/diagnostic/codes.go` (finding L1) |
| Listener conflict registration (finding L3) | [x] Yes | the `:53` binds mark themselves with the `ze:listener` extension (like geodns's listener list, `ze-geodns-conf.yang:32`) so parse-time cross-service port-conflict detection covers as112 vs geodns vs any other :53 listener |
| Prometheus counters/metrics | [x] Yes | `ze_as112_dns_request_total`, `ze_as112_dns_response_total` (zone, qtype, rcode), `ze_as112_dns_request_latency_milliseconds`, `ze_as112_listener_up`, `ze_as112_config_reload_total`, `ze_as112_dns_denied_total{reason="source-not-allowed"}` (allow-from drops) — mirrors geodns's metric shape plus the access-list counter |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|------------------|
| 1 | New user-facing feature? | [x] Yes | `docs/features.md` |
| 2 | Config syntax changed? | [x] Yes | `docs/guide/as112.md` (spec-as112-0) |
| 3 | CLI command added/changed? | [x] Yes | `docs/guide/command-reference.md` (`show as112`) |
| 4 | API/RPC added/changed? | [x] Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [x] Yes | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [x] Yes | `docs/guide/as112.md` |
| 7 | Wire format changed? | [ ] No — standard DNS wire format via `miekg/dns`, no new wire format | n/a |
| 8 | Plugin SDK/protocol changed? | [ ] No | n/a |
| 9 | RFC behavior implemented? | [x] Yes | `rfc/short/rfc7534.md`, `rfc/short/rfc7535.md` (already exist, no edit needed — confirmed both files present) |
| 10 | Test infrastructure changed? | [ ] No | n/a |
| 11 | Affects daemon comparison? | [x] Yes | `docs/comparison.md` (new feature row) |
| 12 | Internal architecture changed? | [ ] No — new plugin, no core architecture change beyond spec-as112-1 (covered there) | n/a |
| 13 | Route metadata keys added/changed? | [ ] No | n/a |
| 14 | Prometheus counters added/changed? | [x] Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | [x] Yes | `docs/plugin-overview.md`, `docs/features/plugins.md` |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] To verify — grep `docs/` for `geodns` to confirm no doc incorrectly implies as112 reuses geodns code | n/a unless grep hits |
| 17 | Existing docs show config/CLI/API examples for this area? | [ ] No — net-new area | n/a |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7-10. Critical review / fix / re-verify | Critical Review Checklist below |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 14. Present summary | Executive Summary per `ai/rules/planning.md` |

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** — `register.go` skeleton (plugin registers, `OnConfigure` stub), `yang/ze-as112-conf.yang` with `enabled` leaf only; failing wiring test `test/parse/as112-config.ci`.
   - Tests: `test/parse/as112-config.ci`
   - Files: `internal/plugins/as112/register.go`, `internal/plugins/as112/yang/`
   - Verify: config parses; plugin registers; server is a no-op stub
2. **Phase: Static zone data + answerer** — `zones.go` with the full zone table and SOA/NS/TXT synthesis; `server.go` binds and serves.
   - Tests: `TestZoneAnswer_*`, `TestAS112NeverRecurses`
   - Files: `internal/plugins/as112/zones.go`, `internal/plugins/as112/server.go`, `internal/plugins/as112/state.go`
   - Verify: unit tests fail → implement → pass
3. **Phase: Address registration** — wire `OnConfigure` to call spec-as112-1's `RegisterOwnedAddresses`/`UnregisterOwnedAddresses`.
   - Tests: `TestOnConfigure_RegistersAddressesOnEnable`
   - Files: `internal/plugins/as112/register.go`
   - Verify: test fails → implement → passes → wiring test (`as112-enable.ci`) passes
4. **Phase: Observability** — `metrics.go`, `show.go`, `doctor.go`.
   - Tests: `TestShowAS112_MatchesServerSnapshot`, doctor test
   - Files: `internal/plugins/as112/metrics.go`, `show.go`, `doctor.go`
   - Verify: tests fail → implement → pass
5. **Phase: Client-source access list (`allow-from`)** — parse the `allow-from` leaf-list into a compiled prefix matcher in the atomic snapshot; enforce in the query handler (drop out-of-range, always permit loopback/on-box); `ze_as112_dns_denied_total` counter; surface `allow-from` in `show as112`.
   - Tests: `TestAllowFrom_EmptyAnswersAll`, `TestAllowFrom_DropsOutOfRange`, `TestAllowFrom_LoopbackAlwaysPermitted`
   - Files: `internal/plugins/as112/config.go`, `server.go`, `state.go`, `metrics.go`, `yang/ze-as112-conf.yang`
   - Verify: tests fail → implement → pass (AC-14/15/16)
6. **Functional tests** → `test/plugin/as112-dns-zones.ci`, `as112-hostname.ci`, `as112-soa-content.ci`, `as112-health.ci`, `as112-allow-from.ci`, `as112-disable.ci`, `as112-enable.ci`.
7. **RFC refs** → `// RFC 7534 Section X.Y` / `// RFC 7535 Section X.Y` comments above zone-table construction and the recursion-refused logic.
8. **Full verification** → `make ze-verify`
9. **Complete spec** → audit tables, learned summary, two-commit close

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-16 each have file:line implementation |
| Feature completeness | Every End-to-End User Story has a working path; compare against geodns feature-for-feature where geodns's pattern applies (listener lifecycle, doctor check, metrics, show, client-source matching) |
| Access list | `allow-from` empty = answer all; non-empty = drop out-of-range; loopback/on-box always permitted (AC-14/15/16); reuses geodns's `netip.Prefix.Contains` matcher, no re-implementation |
| Correctness | Zone-apex vs. in-zone-name distinction is correct for every served zone; NXDOMAIN boundary is exactly at zone-set membership, no off-by-one |
| Naming | YANG leaves kebab-case; Go identifiers match `ai/rules/naming.md` |
| Data flow | Address registration happens only through spec-as112-1's API, never a direct netlink call from this plugin |
| CLI grammar | `show as112` follows action-before-identifier |
| Registration over hardcoding | Plugin registered via the standard registry; no special-casing in core packages |
| Doctor checks | `as112-listen-capability` registered per `ai/rules/doctor-checks.md` |
| YANG validation | `hostname`/`facility`/`location` have `length` constraints; `address-family` has `enumeration` |
| Prometheus counters | Counters match the list in the Integration Checklist, registered, names documented |
| Rule: no-layering | This plugin imports `internal/core/dnsserver` (plugin→core, allowed) but NOT geodns or any BGP/sibling plugin (`ai/rules/plugin-design.md:133`); verify with `dep_audit.py` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `internal/plugins/as112/` directory with all files listed above | `ls -la internal/plugins/as112/` |
| Plugin registered in composition root | `grep as112 internal/component/plugin/all/all.go` after `make generate` |
| All functional tests pass | `make ze-functional-test` filtered to `as112-*` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|--------------------|
| Input validation | `hostname`/`facility`/`location` length-bounded at YANG layer; no operator-controlled data reaches a code path that could overflow the 512-octet TXT budget |
| Resource exhaustion | DNS server has the same per-query bounds geodns already enforces (no unbounded response construction); recursion always refused, so this node cannot be used as an open recursive resolver / amplification vector |
| Amplification | Confirm response sizes for the smallest queries are not disproportionately large (a known DNS-amplification-abuse vector) — SOA/NS-only answers are inherently small, document this explicitly as a deliberate mitigation. When `allow-from` is set, out-of-range (incl. spoofed) sources get NO response (drop, not REFUSED), removing the reflection vector for those sources entirely |
| Access-list bypass | The loopback/on-box always-permit carve-out must be scoped to genuine on-box sources only (loopback addresses / the node's own addresses), not a spoofable range, so it cannot be used to bypass `allow-from` from off-box; verify the carve-out matches the actual local source, not a wildcard |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read geodns's `server.go`/RFC summaries from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong → escalate to umbrella; if AC correct → fix implementation |
| Audit finds missing AC | Back to relevant phase |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|---------------------------|-----------|
| No host-set/source-CIDR matching (unlike geodns) | Reuse geodns's full config model including host-sets | Every AS112 answer is identical regardless of client; host-set matching would be unused complexity |
| Fixed Go-constant addresses, no operator IP entry | Free-form listener IP list like geodns | RFC 7534/7535 addresses are protocol-fixed; typo-proofing and "no IP to configure" were explicit user requirements |
| Dedicated `hostname` config leaf, separate from `facility`/`location` | Fold node identity into the facility string | User explicitly asked for a way to identify which anycast instance answered — a dedicated, clearly-named field is more discoverable than overloading a free-text facility string |
| Own the healthcheck probe command (`ze … as112 health`) here, not in child 3 (finding M4) | (a) tell operators to install `dig`; (b) extend `ze resolve dns` to target a server | (a) `dig` is not on the gokrazy appliance; (b) `ze resolve dns` uses the local recursive resolver and cannot query a specific authoritative server (`ze-resolve-cmd.yang:20-39`). A purpose-built one-shot query in the plugin that owns the service is self-contained and always present with the `ze` binary |
| `IP_FREEBIND` on listeners (finding B2) | (a) bind-retry loop; (b) require iface to apply addresses before the plugin binds | Freebind is a single `setsockopt` in a `Control` hook, needs no ordering guarantee or retry state machine, and is the standard anycast-node pattern; the anycast-address healthcheck probe (H1) remains the real reachability gate |
| In-plugin `allow-from` client-source access list, drops out-of-range queries | (a) tell operators to write firewall-section rules; (b) REFUSED rcode instead of drop; (c) no access control at all | (a) hand-authoring firewall rules matching :53 on four v4/v6 anycast addresses is error-prone and splits the feature across two subsystems — plugin-self-containment favors the plugin owning its own access control; (b) drop gives zero amplification to spoofed out-of-range sources and is the firewall-equivalent (REFUSED still reflects a small packet to a possibly-spoofed victim) — REFUSED can be a future `deny-action` toggle; (c) a local-use mirror often must not answer arbitrary Internet sources. On-box/loopback is always permitted so the healthcheck (H1/M4) is never blocked |

## Known Limitations
- NSID (RFC 5001) is not implemented; the `hostname` TXT mechanism is the v1
  node-identification approach (see umbrella Known Limitations).
- Only one address per service per family (4 total); BLACKHOLE-1/2 secondary
  IANA addresses are out of scope.
- This plugin requires in-process deployment (depends on spec-as112-1's
  Go-level registry API); out-of-process/forked deployment is not supported.
- The `ze … as112 health` command (finding M4) is the probe tool for child 3;
  operators writing their own probes must use it (or another tool present on
  the target image), not `dig`, which the appliance does not ship.
- `allow-from` v1 drops out-of-range queries silently (no REFUSED); a
  `deny-action` toggle for REFUSED is a possible future addition. On-box/loopback
  is always permitted, so `allow-from` cannot be used to firewall the node from
  its own healthcheck. Setting `allow-from` makes the node non-public and is a
  local-use-only choice (leave it empty for a globally-reachable AS112 node).

## RFC Documentation

Add `// RFC 7534 Section 2.2: "<quoted requirement>"` and
`// RFC 7535 Section 2: "<quoted requirement>"` comments above the zone-table
construction in `zones.go`, and `// RFC 7534 Section 3.5: "...recursion no..."`
above the recursion-refusal logic in `server.go`.

## Implementation Summary
### What Was Implemented
- [Filled at closure]

### Bugs Found/Fixed
- [Filled at closure]

### Documentation Updates
- [Filled at closure]

### Deviations from Plan
- [Filled at closure]

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-------------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|---------------------------|----------------|----------------------|
| Serves AS112 zones correctly, no operator-typed IP, identifiable via hostname | functional test | `test/plugin/as112-dns-zones.ci`, `test/plugin/as112-hostname.ci` |

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [Filled during implementation]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|------------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|---------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|----------------------------------|------------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-16 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-as112-2-dns-server.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-as112-2-dns-server.md` only
