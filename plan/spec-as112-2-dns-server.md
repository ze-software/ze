# Spec: AS112 DNS Server Plugin

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-as112-1-iface-address-registry |
| Phase | - |
| Updated | 2026-06-30 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-as112-0-umbrella.md` - RFC compliance mapping, cross-cutting decisions
4. `plan/spec-as112-1-iface-address-registry.md` - the address registry this plugin calls into
5. `rfc/short/rfc7534.md`, `rfc/short/rfc7535.md`, `rfc/short/rfc1035.md` - protocol requirements
6. `internal/plugins/geodns/` - architectural precedent

## Task

Build `internal/plugins/as112/`: a system plugin that, when enabled, registers
ownership of four fixed anycast addresses against the registry from
`spec-as112-1-iface-address-registry.md` and serves the AS112 sink zones
authoritatively on those addresses (port 53, UDP+TCP), per RFC 7534 §2.2/§3.5
and RFC 7535 §2. No operator-typed IP address anywhere in this plugin's config
— the four addresses are fixed Go constants, not configurable. The only
operator inputs are: enable the service, optionally restrict to one address
family, and optionally set a `hostname` identifier string surfaced in the
HOSTNAME.AS112.NET/ARPA TXT answers so operators can tell which anycast
instance answered a given query.

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
  inputs are an optional ipv4-only/ipv6-only restriction and the `hostname`
  string.

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
2. On `enabled: true`, `OnConfigure` calls `iface.RegisterOwnedAddresses("lo", "as112", <the fixed 4 addresses, filtered by address-family>)`. On `enabled: false` (or plugin shutdown), calls `iface.UnregisterOwnedAddresses("as112")`.
3. The DNS server binds UDP+TCP listeners on the four (or fewer, if family-restricted) addresses plus `127.0.0.1`/`::1` for local diagnostics, port 53.
4. Each query is answered from a precomputed, static zone table (no per-client logic): SOA+NS for the 19 reverse zones and EMPTY.AS112.ARPA; SOA+NS+TXT(hostname/facility) for HOSTNAME.AS112.NET/ARPA; NXDOMAIN outside all served zones; recursion always refused.
5. `show as112` and Prometheus metrics read the same atomic snapshot the server reads.

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
- [ ] No unintended coupling (this plugin does not import geodns; it independently mirrors the pattern)
- [ ] No duplicated functionality (reuses `miekg/dns`, already a vendored dependency via geodns and `internal/component/dns`)
- [ ] Zero-copy preserved where applicable (static zone answers can be precomputed once per config-reload, not rebuilt per query)
- [ ] Registration over hardcoding (plugin registers via the existing plugin registry like any other; no special-casing elsewhere)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Four fixed addresses (192.175.48.1, 192.31.196.1, 2620:4f:8000::1, 2001:4:112::1) are the complete address set — no BLACKHOLE-1/2 secondary addresses | User confirmed "4 addresses total" earlier in this conversation; matches umbrella A-2 | Real-world AS112 participation expects 3 addresses per family for the Direct Delegation service | re-confirm with user at WRITE-gate review (umbrella A-2 covers the same point) | unvalidated |
| A-2 | This plugin is deployed in-process (`internal` per `plan/learned/821-plugin-internal-keyword.md`), since it depends on spec-as112-1's Go-level registration API | spec-as112-1 R-3 restricts the registry to in-process callers | Operator tries to run as112 as an external/forked plugin and address registration silently fails or errors | doctor check fires / config verifier rejects external deployment mode for this plugin | unvalidated |
| A-3 | `hostname` and optional facility/location strings are short enough that, combined with the fixed SOA/NS overhead, the HOSTNAME.AS112.* TXT answer fits 512 octets without EDNS0 | RFC 7534 §3.5 requirement; geodns's own answers are typically well under this for comparable record counts | An operator sets an overly long `hostname` string and breaks the 512-octet requirement | YANG `length` constraint on the `hostname`/facility leaves sized so the worst case still fits, verified by a boundary test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Port 53 cannot be bound (privilege/capability missing) and the service silently fails to serve | doctor check `as112-listen-capability` fires (mirrors `geodns-listen-capability`) | doctor check + `show as112` reports listener-down state; documented in `docs/guide/as112.md` |
| R-2 | A static zone table is computed once at config-reload and never refreshed, causing the SOA serial to go stale across long uptimes | SOA serial mode mirrors geodns's existing `auto-epoch`/`auto-datetime`/`fixed` modes (`plan/learned/993-geodns-2-server.md`) — same gotchas apply, no new ones introduced | reuse geodns's already-solved serial-mode design verbatim |
| R-3 | Recursion-disabled enforcement regresses if a future change to the shared `miekg/dns` handler pattern (copied from geodns) accidentally enables recursive lookups | unit test explicitly asserting `RecursionAvailable=false` on every response and that no upstream resolver client exists in this plugin's dependency graph | `TestAS112NeverRecurses` in the TDD plan below |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `service/as112/enabled = true` committed | → | `OnConfigure` → `iface.RegisterOwnedAddresses` → DNS server `bind()` | `test/parse/as112-config.ci` (config accepted) + `test/plugin/as112-enable.ci` (server actually listening) |
| DNS query for `1.0.10.in-addr.arpa` sent to `192.175.48.1:53` | → | static zone answerer | `test/plugin/as112-dns-zones.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|--------------------|
| AC-1 | `enabled: true` committed | The four canonical addresses (filtered by any address-family restriction) are registered via spec-as112-1's API; no operator-typed IP required |
| AC-2 | Query for any name within a Direct-Delegation reverse zone (e.g. `1.0.10.in-addr.arpa`) | NOERROR, empty Answer, zone SOA in Authority |
| AC-3 | Query for any name within `empty.as112.arpa` | NOERROR, empty Answer, zone SOA in Authority |
| AC-4 | Query for `hostname.as112.net` TXT (and `.arpa`) | NOERROR; TXT records include the operator-configured `hostname` string as a distinct TXT string; total response ≤ 512 octets without EDNS0 |
| AC-5 | Query for a name outside every served zone | NXDOMAIN |
| AC-6 | Any query, any zone | `RecursionAvailable` is always false in the response header; the plugin never issues an upstream query |
| AC-7 | as112 enabled on a privileged port the process cannot bind | `ze doctor` reports `doctor-as112-listen-capability` |
| AC-8 | `show as112` invoked | Reports enabled state, zones served, listener addresses, registered-address-ownership state, sourced from the same atomic snapshot the DNS server reads |
| AC-9 | `enabled: false` committed after having been true | DNS server stops; `iface.UnregisterOwnedAddresses("as112")` called; addresses removed from loopback on next reconciliation (unless independently YANG-declared) |
| AC-10 | `address-family: ipv4-only` set | Only the two IPv4 addresses (192.175.48.1, 192.31.196.1) are registered/bound; IPv6 addresses are not |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|--------------------------|
| 1 | enables the as112 service with no other config | config → `OnConfigure` → registry (spec-as112-1) → loopback addresses appear → DNS server binds → answers correctly | `test/plugin/as112-enable.ci` + `test/plugin/as112-dns-zones.ci` |
| 2 | sets a `hostname` and queries `hostname.as112.net TXT` to identify the node | config → TXT answer construction → response | `test/plugin/as112-hostname.ci` |
| 3 | disables the service | config → server stop → address deregistration → loopback addresses removed | `test/plugin/as112-disable.ci` |

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

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|----------------|----------------|
| `hostname` string length | 0-63 octets (DNS label limit, also keeps 512-octet TXT budget) | 63 | n/a (empty allowed, omits the TXT string) | 64 (rejected by YANG `length` constraint) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|---------------------|--------|
| `as112-config` | `test/parse/as112-config.ci` | config with `enabled: true` and a `hostname` parses and validates | |
| `as112-enable` | `test/plugin/as112-enable.ci` | enabling the service makes it answer on the canonical addresses | |
| `as112-dns-zones` | `test/plugin/as112-dns-zones.ci` | queries against every served zone return the RFC-correct shape (AC-2,3,5,6) | |
| `as112-hostname` | `test/plugin/as112-hostname.ci` | HOSTNAME.AS112.NET/ARPA TXT carries the configured hostname, fits 512 octets | |
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
- `internal/plugins/as112/server.go` - DNS server lifecycle (binds, mux handler), mirrors geodns's server.go shape
- `internal/plugins/as112/zones.go` - static zone table: the 19 reverse zones + EMPTY.AS112.ARPA + HOSTNAME.AS112.{NET,ARPA}, SOA/NS/TXT synthesis
- `internal/plugins/as112/state.go` - atomic snapshot (mirrors geodns's state.go)
- `internal/plugins/as112/metrics.go` - Prometheus counters/histogram/gauges (mirrors geodns's metrics.go)
- `internal/plugins/as112/show.go` - `show as112` handler
- `internal/plugins/as112/doctor.go` - `as112-listen-capability` doctor check
- `internal/plugins/as112/yang/ze-as112-conf.yang` - config schema
- `internal/plugins/as112/yang/ze-as112-cmd.yang` - `show as112` command tree
- `internal/plugins/as112/yang/embed.go`, `internal/plugins/as112/yang/register.go` - YANG embedding/registration (mirrors geodns's yang/ layout)
- `internal/plugins/as112/register_test.go`, `config_test.go`, `zones_test.go`, `server_test.go`, `show_test.go`, `doctor_test.go`, `metrics_test.go` - unit tests
- `test/parse/as112-config.ci`, `test/plugin/as112-enable.ci`, `test/plugin/as112-dns-zones.ci`, `test/plugin/as112-hostname.ci`, `test/plugin/as112-disable.ci` - functional tests

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [x] Yes | `internal/plugins/as112/yang/ze-as112-conf.yang` |
| YANG validation constraints | [x] Yes | `hostname`/`facility`/`location` get `length` constraints; `address-family` is an `enumeration` |
| YANG custom validators | [ ] No — native constraints (length, enum) are sufficient | n/a |
| CLI commands/flags | [x] Yes | `show as112` via `internal/plugins/as112/yang/ze-as112-cmd.yang` |
| CLI grammar | [x] Yes | action-before-identifier per `ai/rules/cli-grammar.md`, mirrors `show geodns` |
| Editor autocomplete | [x] Yes | automatic for the `address-family` enum leaf |
| Functional test for new RPC/API | [x] Yes | `test/plugin/as112-enable.ci` exercises `show as112` |
| Pipe completeness | [x] Yes | `show as112` output routes through `ApplyPipes`/`ProcessPipes` per `ai/rules/pipe-completeness.md`, mirrors `show geodns` |
| Env var registration | [ ] No — this is operational policy config, not environment config (`ai/rules/config-surface.md`) | n/a |
| Doctor check for runtime dependencies | [x] Yes | `as112-listen-capability` (port 53 bind capability) registered in `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | [x] Yes | `ze_as112_dns_request_total`, `ze_as112_dns_response_total` (zone, qtype, rcode), `ze_as112_dns_request_latency_milliseconds`, `ze_as112_listener_up`, `ze_as112_config_reload_total` — mirrors geodns's metric shape, listed here per spec |

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
5. **Functional tests** → `test/plugin/as112-dns-zones.ci`, `as112-hostname.ci`, `as112-disable.ci`, `as112-enable.ci`.
6. **RFC refs** → `// RFC 7534 Section X.Y` / `// RFC 7535 Section X.Y` comments above zone-table construction and the recursion-refused logic.
7. **Full verification** → `make ze-verify`
8. **Complete spec** → audit tables, learned summary, two-commit close

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-10 each have file:line implementation |
| Feature completeness | Every End-to-End User Story has a working path; compare against geodns feature-for-feature where geodns's pattern applies (listener lifecycle, doctor check, metrics, show) |
| Correctness | Zone-apex vs. in-zone-name distinction is correct for every served zone; NXDOMAIN boundary is exactly at zone-set membership, no off-by-one |
| Naming | YANG leaves kebab-case; Go identifiers match `ai/rules/naming.md` |
| Data flow | Address registration happens only through spec-as112-1's API, never a direct netlink call from this plugin |
| CLI grammar | `show as112` follows action-before-identifier |
| Registration over hardcoding | Plugin registered via the standard registry; no special-casing in core packages |
| Doctor checks | `as112-listen-capability` registered per `ai/rules/doctor-checks.md` |
| YANG validation | `hostname`/`facility`/`location` have `length` constraints; `address-family` has `enumeration` |
| Prometheus counters | Counters match the list in the Integration Checklist, registered, names documented |
| Rule: no-layering | This plugin does not import geodns or any BGP package |

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
| Amplification | Confirm response sizes for the smallest queries are not disproportionately large (a known DNS-amplification-abuse vector) — SOA/NS-only answers are inherently small, document this explicitly as a deliberate mitigation |

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

## Known Limitations
- NSID (RFC 5001) is not implemented; the `hostname` TXT mechanism is the v1
  node-identification approach (see umbrella Known Limitations).
- Only one address per service per family (4 total); BLACKHOLE-1/2 secondary
  IANA addresses are out of scope.
- This plugin requires in-process deployment (depends on spec-as112-1's
  Go-level registry API); out-of-process/forked deployment is not supported.

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
- [ ] AC-1..AC-10 all demonstrated
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
