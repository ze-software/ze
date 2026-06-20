# Spec: ospf-13-cli-diag-interop

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-ospf-1-types.md, spec-ospf-2-wire.md, spec-ospf-3-ip-transport.md, spec-ospf-4-component-config.md, spec-ospf-5-interface-ism.md, spec-ospf-6-neighbor-nsm.md, spec-ospf-7-lsdb-flooding.md, spec-ospf-8-spf-rib.md, spec-ospf-9-inter-area-abr.md, spec-ospf-10-as-external-asbr.md, spec-ospf-11-stub-nssa.md, spec-ospf-12-auth.md |
| Phase | - |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-ospf-0-umbrella.md` - the OSPF umbrella; this child supplies the goal-validation interop evidence and asserts the full canonical `ze_ospf_*` metric set. Read "Shared Contracts": "Command + API YANG", "Metrics (canonical)", "Test + interop wiring", and the ospf-13 Child Specs / Dependency rows
4. `internal/plugins/isis/cmd_show.go` and `internal/plugins/ldp/cmd_show.go` - `pluginserver.RegisterRPCs` + `ForwardToPlugin` proxy pattern (the exact model for `show ip ospf ...`)
5. `plan/learned/937-isis-13-cli-diag-interop.md` - the sibling IS-IS presentation/diagnostic/interop child; OSPF copies its structure (show proxy, metrics-assert, doctor surface, FRR interop, web SSE) with the OSPF noun set
6. `ai/rules/cli-grammar.md`, `ai/rules/pipe-completeness.md`, `ai/rules/doctor-checks.md`, `ai/rules/interop-and-goal-validation.md`
7. Sibling engine specs: `spec-ospf-5-interface-ism.md` (interface + DR/BDR snapshot), `spec-ospf-7-lsdb-flooding.md` (LSDB snapshot, max-metric origination), `spec-ospf-8-spf-rib.md` (SPF/route snapshot, SPF log)

## Task

Add the presentation, observability and verification layer over the working
OSPFv2 engine built by the sibling specs (ospf-1 through ospf-12). The engine
already maintains interface, neighbour, LSDB, SPF and route state and exposes
read-only snapshot APIs; this child makes that state observable (CLI, web,
metrics), surfaces readiness checks (doctor), exposes the max-metric router-LSA
(stub-router) config and reflects it in `show`, and proves the protocol works
against a reference implementation (FRR `ospfd`). Nothing here originates or
changes protocol state: it renders snapshots, exports metrics, surfaces checks,
and drives end-to-end interop.

Concretely, this child delivers:

- **CLI show/clear commands** registered the LDP/IS-IS way:
  `pluginserver.RegisterRPCs` with `ze-show:ospf-*` / `ze-clear:ospf-*` wire
  methods whose handlers proxy fixed plugin commands to the engine via the
  dispatcher (model `internal/plugins/isis/cmd_show.go`). Commands:
  `show ip ospf` (process summary: router-id, areas, ABR/ASBR status, SPF
  stats), `show ip ospf neighbor`, `show ip ospf interface`,
  `show ip ospf database` (plus per-LSA-type subviews
  `router`/`network`/`summary`/`asbr-summary`/`external`/`nssa-external`),
  `show ip ospf route`, `show ip ospf border-routers`, `show ip ospf spf` (SPF
  log); `clear ip ospf process`, `clear ip ospf neighbor`, `clear ip ospf
  counters`. CLI grammar is action-before-identifier; every command that emits
  output routes through the pipe machinery (`ApplyPipes`/`ProcessPipes`).
- **max-metric router-lsa (stub-router) config + CLI reflection.** Add the
  `max-metric router-lsa` config leaves (RFC 6987 stub-router: always /
  on-startup with a duration / on-shutdown with a duration) under the `ospf`
  container and reflect the active stub-router state in `show ip ospf` (process
  summary). ORIGINATION of the max-metric Router-LSA is owned by ospf-7; this
  child owns only the config leaves and the `show` reflection.
- **Web views**: an OSPF neighbour page and an OSPF database page with SSE live
  updates, following the existing web component patterns (model the IS-IS web
  surface).
- **Prometheus metrics**: SCRAPE and ASSERT the complete canonical `ze_ospf_*`
  series from the umbrella "Metrics (canonical)" table. ospf-13 registers NO
  metric series itself; each series is owned and registered by its producing
  spec (ospf-3/5/6/7/8/9/10/11/12). This child only verifies the full set is
  exposed with the exact names and labels.
- **Doctor checks**: surface the `doctor-ospf-raw-socket` /`CAP_NET_RAW`
  readiness check defined and registered in ospf-3
  (`transport/doctor_linux.go`), plus config-sanity checks (router-id present
  and a valid dotted-quad; every enabled interface bound to a declared area).
  This child registers ONLY the two config-sanity `doctor-ospf-*` codes; the
  raw-socket code is owned by ospf-3 and only surfaced here.
- **FRR `ospfd` interop scenarios** under `test/interop/scenarios/`:
  `ospf-p2p-frr`, `ospf-broadcast-frr`, `ospf-multiarea-frr`,
  `ospf-stub-nssa-frr`, `ospf-auth-frr`, and `ospf-convergence-frr`. These are
  MANDATORY (per `ai/rules/interop-and-goal-validation.md` and the umbrella
  "Test + interop wiring" row) and are the GOAL-VALIDATION evidence for the
  umbrella.
- **Documentation**: a user guide `docs/guide/ospf.md`, a wire/protocol doc
  `docs/architecture/wire/ospf.md`, the comparison/features/command-reference
  rows, the `ze_ospf_*` metrics listing, and the functional-test/interop docs.

This child depends on the engine snapshots from the siblings (ospf-5/7/8); it
pulls data from the interface/neighbour/LSDB/SPF/route snapshot APIs those specs
define and must not reach around them into engine internals.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] - checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
- [ ] `plan/spec-ospf-0-umbrella.md` "Shared Contracts" - "Command + API YANG", "Metrics (canonical)", "Test + interop wiring"; the ospf-13 Child Specs / Dependency rows
  → Decision: ship ONE command YANG `ze-ospf-cmd.yang` binding the CENTRAL `ze-show:ospf-*` / `ze-clear:ospf-*` namespaces (no `ze-ospf-api.yang`); register the RPCs in Go via `pluginserver.RegisterRPCs`, LDP/IS-IS style
  → Constraint: ospf-13 registers NO metric series; it scrapes/asserts the FULL canonical `ze_ospf_*` set from the umbrella Metrics table (each series owned by ospf-3/5/6/7/8/9/10/11/12). Never coin a bare `ospf_*` name
- [ ] `internal/plugins/isis/cmd_show.go`, `internal/plugins/ldp/cmd_show.go` - `pluginserver.RegisterRPCs` + `ForwardToPlugin` proxy; the show command is a thin proxy to a fixed engine command
  → Decision: implement `internal/plugins/ospf/cmd_show.go` identically: one `RPCRegistration` per `ze-show:ospf-<noun>` / `ze-clear:ospf-<action>` wire method, each forwarding a fixed `show ip ospf <noun>` / `clear ip ospf <action>` command through `Dispatcher().ForwardToPlugin` (NOT `Dispatch`, which re-matches the builtin and recurses)
  → Constraint: the show handlers carry no protocol logic; the engine `OnExecuteCommand` produces the JSON, the proxy relays it unchanged so removing the component removes the command, its schema, and the handlers together (plugin-self-containment)
- [ ] `plan/learned/937-isis-13-cli-diag-interop.md` - the IS-IS sibling presentation/diagnostic/interop child
  → Constraint: copy its structure (show proxy, metrics-assert-only test, config-sanity doctor + surfaced raw-socket check, web neighbour/database SSE, six FRR scenarios, two-commit closure); substitute the OSPF noun set and the `ze_ospf_*` / `doctor-ospf-*` names
- [ ] `ai/rules/cli-grammar.md` - keywords before values; action before identifier
  → Constraint: `show ip ospf database router` (action keyword, a per-LSA-type subview, not a free value); `clear ip ospf process|neighbor|counters` are runtime actions (verb form allowed, not YANG-tree mutations); any selector (e.g. a neighbour router-id filter) must be typed, never a bare positional
- [ ] `ai/rules/pipe-completeness.md` - every output command supports all pipe operators
  → Constraint: each `show ip ospf ...` routes JSON through `ApplyPipes`/`ProcessPipes`; `| resolve` and `| origin` apply on the data; `| json`, `| table`, `| text`, `| count`, `| match` all work
- [ ] `ai/rules/doctor-checks.md` - readiness checks own their registration, code, and tests
  → Constraint: this child owns ONLY the config-sanity checks (`doctor-ospf-router-id-missing`, `doctor-ospf-interface-area-unbound`) and registers ONLY those two codes. The `CAP_NET_RAW`/raw-socket check AND its `doctor-ospf-raw-socket` code are owned and registered by ospf-3 (`transport/doctor_linux.go`); this child only SURFACES that result, never re-registers it. Provide a unit test and a `ze doctor --json` functional test for the config-sanity checks
- [ ] `ai/rules/interop-and-goal-validation.md` - protocol features MUST have interop tests; goal validation needs concrete evidence
  → Constraint: each FRR scenario has `ze.conf`, `frr.conf`, `check.py`; `check.py` waits for adjacency to Full, asserts the specific behaviour (route present, DR elected, inter-area route, stub/NSSA route, auth succeeds, reconvergence), and verifies stability
- [ ] `docs/research/ospf-implementation-guide.md` §11 (CLI / plugin registration) + §12 (Testing Strategy: unit/fuzz/integration/interop/regression)
  → Decision: CLI noun set follows the guide (`show ip ospf neighbor/interface/database/route/spf`), extended with `border-routers` (Type 4 / ABR-ASBR view) and the process summary `show ip ospf`, normalised to Ze grammar
  → Constraint: interop validates adjacency formation across network types, LSDB convergence (every LSA present both ways with matching sequence numbers), routing-table convergence, and failover; compare captures against FRR to catch encoding differences

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc6987.md` - OSPF Stub Router Advertisement (max-metric router-lsa)
  → Constraint: the `max-metric router-lsa` config leaves expose stub-router mode (always / on-startup / on-shutdown with durations); origination of the max-metric Router-LSA is ospf-7; this child only adds the config leaves and reflects the active state in `show ip ospf`. The protocol RFC summaries (RFC 2328 / 3101 / 5709 / 7474) live in the owning siblings; this child references them

**Key insights:** (minimal context to resume after compaction)
- The show layer is a proxy: wire method → `ForwardToPlugin` → engine `show ip ospf <noun>` command → JSON snapshot → pipes. No protocol logic here.
- ospf-13 registers NO metric series; it asserts the full canonical `ze_ospf_*` set is exposed by its owners (umbrella Metrics table).
- The only new doctor codes are the two config-sanity `doctor-ospf-*`; the raw-socket check is surfaced from ospf-3, not re-registered.
- The six FRR scenarios are the umbrella's goal-validation evidence; they map to the umbrella ACs.
- max-metric router-lsa: config leaves + `show ip ospf` reflection here; origination is ospf-7.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `internal/plugins/isis/cmd_show.go` - existing protocol component proxies `show isis <noun>` to the engine via `ForwardToPlugin`
  → Constraint: OSPF mirrors this file structure exactly under `internal/plugins/ospf/cmd_show.go`, substituting the `show ip ospf` noun set and the `ze-show:ospf-*` / `ze-clear:ospf-*` tokens
- [ ] `internal/plugins/isis/codes.go` + `register.go` - doctor codes owned in the component (the IS-IS deviation that became the pattern), prefix `doctor-<component>-<condition>`, explainable via `ze explain`
  → Constraint: this child registers ONLY `doctor-ospf-router-id-missing` and `doctor-ospf-interface-area-unbound` (config-sanity), owned in the component. `doctor-ospf-raw-socket` (code + check) is OWNED and registered by ospf-3 (`transport/doctor_linux.go`); ospf-13 must NOT re-register it (double registration), it only surfaces all `doctor-ospf-*` results in `ze doctor` / `ze explain`
- [ ] sibling engine specs (ospf-5/7/8) define interface/neighbour/LSDB/SPF/route snapshot APIs consumed here
  → Constraint: this child reads those snapshots; it does not add engine state. The max-metric Router-LSA origination is ospf-7; ospf-13 reads the active stub-router state for the `show ip ospf` summary

**Behavior to preserve:** (unless user explicitly said to change)
- The OSPF engine (interface ISM, neighbour NSM, LSDB, flooding, DR/BDR, SPF, route install, inter-area, external, stub/NSSA, auth) behaves exactly as the siblings built it; this child is read-only over engine state plus three runtime clear actions
- Existing `show ldp ...` / `show isis ...` / `show bgp ...` grammar and the generic `show` entry point are unchanged; `show ip ospf ...` is an additional noun branch
- The SPF → Loc-RIB insertion (`locrib.Path`, AdminDistance 110) → sysrib `OnChange` → fibkernel install path (ospf-8) is untouched (this is NOT `redistevents`; redistribution is the separate ospf-10 path); metrics observe it, they do not alter it
- Existing doctor checks and `ze explain` output are unchanged; OSPF adds new codes only

**Behavior to change:** (only what this child adds)
- New `ze-show:ospf-*` / `ze-clear:ospf-*` wire methods and `show ip ospf <noun>` / `clear ip ospf <action>` commands
- New `max-metric router-lsa` config leaves (RFC 6987) under the `ospf` container + their reflection in `show ip ospf`
- New OSPF web neighbour and database pages with SSE
- New Prometheus assertion (scrape) over the canonical `ze_ospf_*` set (no new series registered here)
- New `doctor-ospf-*` config-sanity codes and checks

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Operator types `show ip ospf <noun>` (or `clear ip ospf <action>`) in the CLI, or hits the `ze-show:ospf-*` wire method via API/web
- Prometheus scraper hits the metrics endpoint
- `ze doctor` runs the registered OSPF checks against the config tree and platform
- The FRR interop harness drives real OSPF packets over a Linux link

### Transformation Path
1. **CLI show:** wire method `ze-show:ospf-neighbor` → proxy handler → `Dispatcher().ForwardToPlugin(ctx, "show ip ospf neighbor")` → engine `OnExecuteCommand` → reads the neighbour snapshot → returns JSON `plugin.Response`
2. **Pipes:** the CLI/dispatcher routes the JSON through `ApplyPipes`/`ProcessPipes` for `| json/table/text/count/match/resolve/origin`
3. **Per-LSA-type database:** `show ip ospf database router|network|summary|asbr-summary|external|nssa-external` → proxy → engine LSDB snapshot filtered by LS Type → JSON
4. **CLI clear:** `clear ip ospf process|neighbor|counters` → engine command → runtime action (full SPF/LSDB reset / neighbour teardown-reform / counter reset); returns a status response
5. **max-metric reflection:** `show ip ospf` reads the active stub-router state (set by ospf-7 origination from the config leaves added here) and reports it in the process summary
6. **Web:** the OSPF web page subscribes to an SSE stream backed by the same neighbour/LSDB snapshots; updates push on engine change events
7. **Metrics:** the OSPF component publishes counters/gauges to the metrics registry (owners ospf-3/5/6/7/8/9/10/11/12); the scraper reads the current values; ospf-13's test asserts the full set
8. **Doctor:** `ze doctor` invokes the registered OSPF checks; the raw-socket check (from ospf-3) and the config-sanity checks emit `rpc.DoctorCheckDiagnostic` with `doctor-ospf-*` codes
9. **Interop:** FRR `ospfd` and Ze exchange Hello/DD/LS Request/LS Update/LS Ack over a Linux link; `check.py` asserts adjacency Full / route / DR / inter-area / stub-NSSA / auth / reconvergence

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI ↔ engine | `ze-show:ospf-*` wire method → dispatcher → `ForwardToPlugin` → `OnExecuteCommand` JSON | [ ] |
| Engine snapshot ↔ renderer | read-only interface/neighbour/LSDB/SPF/route snapshot (siblings ospf-5/7/8) | [ ] |
| Output ↔ pipes | JSON through `ApplyPipes`/`ProcessPipes` | [ ] |
| Config ↔ engine (max-metric) | `max-metric router-lsa` YANG leaves → engine stub-router state (origination ospf-7); `show ip ospf` reflects | [ ] |
| Engine ↔ web | SSE stream from snapshot + change events | [ ] |
| Engine ↔ metrics | counters/gauges published by the owning specs to the telemetry registry | [ ] |
| Config/platform ↔ doctor | `registry.DoctorCheckContext` → `rpc.DoctorCheckDiagnostic` | [ ] |
| Ze ↔ FRR | real OSPF packets over the interop Linux link | [ ] |

### Integration Points
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` - the ONE owner command YANG for the show/clear surface (show binds `ze-show:ospf-*`, clear binds `ze-clear:ospf-*`; no `ze-ospf-api.yang`), enforced by `scripts/checks/command_ownership.go`
- `internal/plugins/ospf/cmd_show.go` - new RPC registrations (model: isis/ldp)
- `internal/plugins/ospf/` engine `OnExecuteCommand` - handles `show ip ospf <noun>` / `clear ip ospf <action>` (snapshot rendering lives near the engine, siblings)
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` (owned by ospf-4) - extended here with the `max-metric router-lsa` leaves (RFC 6987)
- `internal/component/web` - OSPF neighbour/database pages + SSE (existing web patterns)
- `internal/core/metrics` (telemetry) - assertion only; the series are registered by the owning specs
- `internal/plugins/ospf/codes.go` + OSPF doctor check function - readiness checks
- `test/interop/scenarios/ospf-*-frr/` - interop harness

### Architectural Verification
- [ ] No bypassed layers (show → dispatcher → engine; no direct reach into engine internals from the proxy)
- [ ] No unintended coupling (rendering reads sibling snapshot APIs only; web/metrics/doctor are independent surfaces; no IS-IS coupling)
- [ ] No duplicated functionality (reuse `ApplyPipes`, existing web SSE infra, existing metrics registry, existing doctor runner bridge)
- [ ] Zero-copy preserved where applicable (snapshots are read-only; no LSDB byte copies for display beyond what the snapshot API hands out)

## Risks & Assumptions

<!-- LIVE -- written during RESEARCH/DESIGN, statuses updated during implementation. -->

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The engine exposes read-only interface/neighbour/LSDB/SPF/route snapshot APIs for the renderer | umbrella Child Specs ospf-5/7/8; LDP/IS-IS precedent | Renderer must be co-designed with the engine spec; add snapshot APIs there | `show ip ospf neighbor` returns engine data (Wiring Test) | unvalidated |
| A-2 | The `ForwardToPlugin` proxy pattern fits OSPF unchanged (engine produces JSON via `OnExecuteCommand`) | `internal/plugins/isis/cmd_show.go`, `ldp/cmd_show.go` | Need a custom handler that queries snapshots directly | `ospf-show.ci` exercises the proxy path | unvalidated |
| A-3 | Existing web SSE infra can host two new OSPF pages without engine changes | `internal/component/web` IS-IS/LDP view precedent | Need new web plumbing in this child | web functional/`.et` test renders live neighbour page | unvalidated |
| A-4 | Every canonical `ze_ospf_*` series is registered by its owning spec, so ospf-13 only asserts the scrape | umbrella "Metrics (canonical)" table; IS-IS metrics-assert precedent | A series is missing/misowned → route the fix to the owning spec, not register here | `TestOSPFMetricsRegistered` lists every series with exact labels | unvalidated |
| A-5 | The ospf-3 raw-socket doctor check is already registered by its owner and can be surfaced by `ze doctor --json` here without re-registration | ospf-3 doctor ownership; decision table below | If missing, fix ownership in ospf-3, not by duplicating the code here | `ze doctor --json` emits `doctor-ospf-raw-socket` | unvalidated |
| A-6 | The interop harness supports the network link OSPF needs (raw IP proto 89 + multicast `224.0.0.5`/`224.0.0.6` over a Linux bridge/veth) | `test/interop/scenarios/` BGP/IS-IS pattern; ospf-3 multicast transport | Extend the runner for multicast-capable connectivity | `ospf-p2p-frr` forms adjacency to Full | unvalidated |
| A-7 | The `max-metric router-lsa` config leaves belong in `ze-ospf-conf.yang` (owned by ospf-4), extended here, with origination wired in ospf-7 | umbrella config model; RFC 6987 | If ospf-4/ospf-7 own the leaf, drop the leaf addition here and only reflect | `show ip ospf` reflects stub-router state from the configured leaves | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | FRR timer/encoding differences block adjacency in interop | adjacency never reaches Full in `ospf-p2p-frr` | Capture with tcpdump, compare to FRR `show ip ospf database`, fix codec/FSM in the owning sibling (guide §12) |
| R-2 | DR/BDR election mismatch with FRR on the broadcast scenario | broadcast adjacency stuck in 2-Way with FRR | Cover the sticky-DR rule and priority; guide §13 trap #12; fix election in ospf-5 |
| R-3 | Pipe completeness missed on one show command (esp. the per-LSA-type database subviews) | `\| json` or `\| resolve` fails on `show ip ospf database external` | Mechanical grep for the dispatch path per command; functional pipe test |
| R-4 | Doctor check fires when OSPF is not configured | false positive on a BGP-only node | Check gates on the `ospf` config block being present (doctor-checks.md unit-test rule) |
| R-5 | Web SSE leaks goroutines on page close | rising goroutine count under repeated page loads | Reuse the existing SSE lifecycle; close on disconnect |
| R-6 | Interop scenario naming clashes with the numbered `NN-*` convention the runner expects | runner does not discover `ospf-*-frr` | Confirm runner discovery; adopt the numbered prefix only if required (umbrella uses the unnumbered names) |
| R-7 | `show ip ospf` summary reads max-metric state the engine has not yet exposed | summary shows blank stub-router state with the leaves set | Co-design the active-stub-router snapshot field with ospf-7 origination; the config leaves added here only feed it |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze-show:ospf` wire method | → | proxy → engine process-summary snapshot JSON (router-id, areas, ABR/ASBR, SPF stats, stub-router state) | `test/ospf/ospf-show.ci` |
| `ze-show:ospf-neighbor` wire method | → | proxy → engine neighbour snapshot JSON | `test/ospf/ospf-show.ci` (asserts `show ip ospf neighbor` returns engine neighbour data) |
| `show ip ospf database external` | → | proxy → engine LSDB snapshot filtered to Type 5 | `test/ospf/ospf-show.ci` |
| `show ip ospf route` piped `\| json` | → | engine route snapshot through `ApplyPipes` | `test/ospf/ospf-show.ci` |
| `clear ip ospf counters` | → | engine counter-reset runtime action | `test/ospf/ospf-show.ci` |
| `max-metric router-lsa always` in config | → | engine stub-router state (origination ospf-7) reflected in `show ip ospf` | `test/ospf/ospf-show.ci` |
| `ze doctor --json` with `ospf { ... }` config | → | `doctor-ospf-raw-socket` / config-sanity check fires | `test/ospf/ospf-doctor.ci` |
| FRR `ospfd` peer over a Linux link | → | full OSPF protocol over the wire | `test/interop/scenarios/ospf-p2p-frr/check.py` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show ip ospf` (process summary) | Renders router-id, configured areas, ABR/ASBR status, SPF run count / timers, and active stub-router (max-metric) state |
| AC-2 | `show ip ospf neighbor` with adjacencies Full | Renders each neighbour with router-id, interface, state, DR/BDR, priority, dead time, address |
| AC-3 | `show ip ospf interface` | Lists OSPF-enabled interfaces with area, network-type, cost, state, DR/BDR, hello/dead, priority, passive |
| AC-4 | `show ip ospf database` and the per-LSA-type subviews (`router`/`network`/`summary`/`asbr-summary`/`external`/`nssa-external`) | Base lists all LSAs (LS Type, LS ID, Adv Router, sequence, age, checksum); each subview filters to its LS Type (1/2/3/4/5/7) |
| AC-5 | `show ip ospf route` | Lists OSPF-computed routes with prefix, path type (intra/inter/E1/E2), cost, next-hop, area |
| AC-6 | `show ip ospf border-routers` | Lists routes to ABRs/ASBRs (router-id, cost, next-hop, area, ABR/ASBR flag) |
| AC-7 | `show ip ospf spf` | Renders recent SPF runs (timestamp, area, trigger, duration, node count) |
| AC-8 | Any `show ip ospf ...` piped through `\| json/table/text/count/match/resolve/origin` | Each pipe operator works on the command output |
| AC-9 | `clear ip ospf process` / `clear ip ospf neighbor` / `clear ip ospf counters` | Full reset (re-run SPF, re-form adjacencies) / neighbours torn down and re-formed / counters reset; status response returned |
| AC-10 | `max-metric router-lsa always` (and on-startup / on-shutdown with durations) configured | Config validates; engine enters stub-router mode (origination ospf-7); `show ip ospf` reports the active stub-router state and remaining time |
| AC-11 | Web OSPF neighbour and database pages open | Pages render current state and update live over SSE as neighbours/LSDB change |
| AC-12 | Prometheus scrape | Every exact series and label in the umbrella "Metrics (canonical)" table is exposed; all series are `ze_ospf_*`, none bare `ospf_*`; ospf-13 only asserts the scrape and registers no metric series itself |
| AC-13 | `ze doctor --json` without `CAP_NET_RAW` and `ospf` configured | `doctor-ospf-raw-socket` (from ospf-3) reported as failing with a clear remedy |
| AC-14 | `ze doctor --json` with `ospf` configured but no/invalid router-id, or an enabled interface bound to an undeclared area | `doctor-ospf-router-id-missing` / `doctor-ospf-interface-area-unbound` reported |
| AC-15 | `ospf-p2p-frr` scenario | Point-to-point adjacency reaches Full with FRR; routes converge both ways |
| AC-16 | `ospf-broadcast-frr` scenario | DR/BDR elected on a broadcast LAN with FRR; Network-LSA present; routes converge |
| AC-17 | `ospf-multiarea-frr` scenario | A prefix in area 1 appears as a Type 3 Summary in area 0 across the ABR; routers in area 0 reach it |
| AC-18 | `ospf-stub-nssa-frr` scenario | Stub area suppresses Type 5 and gets the ABR default; NSSA originates Type 7 and the translator converts it to Type 5 on the backbone; both reachable with FRR |
| AC-19 | `ospf-auth-frr` scenario | MD5 / HMAC-SHA-authenticated adjacency forms; a wrong key is rejected |
| AC-20 | `ospf-convergence-frr` scenario | After a link goes down, both sides reconverge, SPF re-runs, and stale routes are withdrawn |
| AC-21 | Any `ospf-*-frr` scenario started by the interop runner | The runner launches a live FRR `ospfd` (because `test/interop/daemons` has `ospfd=yes` and `interop.py` mounts the daemons file plus the scenario `frr.conf`); the FRR ospfd process is running and reaches OSPF adjacency Full with Ze, not merely that the scenario directory exists |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `show ip ospf` | CLI → `ze-show:ospf` → dispatcher → engine process-summary snapshot → JSON → pipes | `test/ospf/ospf-show.ci` |
| 2 | Runs `show ip ospf neighbor` | CLI → proxy → engine neighbour snapshot → render | `test/ospf/ospf-show.ci` |
| 3 | Runs `show ip ospf database external` | CLI → proxy → engine LSDB snapshot filtered to Type 5 → render | `test/ospf/ospf-show.ci` |
| 4 | Runs `show ip ospf route \| json` | CLI → proxy → engine route snapshot → `ApplyPipes` json | `test/ospf/ospf-show.ci` |
| 5 | Runs `show ip ospf interface` / `border-routers` / `spf` | CLI → proxy → respective engine snapshot → render | `test/ospf/ospf-show.ci` |
| 6 | Runs `clear ip ospf process` | CLI → engine runtime action → SPF re-runs, adjacencies re-form | `test/ospf/ospf-show.ci` |
| 7 | Configures `max-metric router-lsa always` and checks `show ip ospf` | config → engine stub-router state (ospf-7) → `show ip ospf` reflection | `test/ospf/ospf-show.ci` |
| 8 | Opens the OSPF web neighbour/database page | web route → SSE → snapshot stream → live update | `test/web/ospf-neighbor-database.wb` |
| 9 | Scrapes Prometheus | scraper → telemetry registry → `ze_ospf_*` series | `test/ospf/ospf-metrics.ci` |
| 10 | Runs `ze doctor` on a node missing `CAP_NET_RAW` | doctor runner → OSPF check → `doctor-ospf-raw-socket` | `test/ospf/ospf-doctor.ci` |
| 11 | Meshes a Ze node with an FRR router (P2P/broadcast/multi-area/stub-NSSA/auth/convergence) | full protocol over the wire | `test/interop/scenarios/ospf-*-frr/check.py` |

<!-- If a path has a broken link (no implementation at some step), that is a spec gap. -->

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFShowSummaryRender` | `internal/plugins/ospf/cmd_show_test.go` | process-summary snapshot → JSON/table render (router-id, areas, ABR/ASBR, SPF, stub-router state) | |
| `TestOSPFShowNeighborRender` | `internal/plugins/ospf/cmd_show_test.go` | neighbour snapshot → render (router-id, state, DR/BDR, priority, dead time) | |
| `TestOSPFShowInterfaceRender` | `internal/plugins/ospf/cmd_show_test.go` | interface snapshot → render (area, network-type, cost, state, DR/BDR) | |
| `TestOSPFShowDatabaseRender` | `internal/plugins/ospf/cmd_show_test.go` | LSDB snapshot → base render + per-LS-type filter (1/2/3/4/5/7) | |
| `TestOSPFShowRouteRender` | `internal/plugins/ospf/cmd_show_test.go` | route snapshot → render with path type / cost / nexthop / area | |
| `TestOSPFShowBorderRoutersRender` | `internal/plugins/ospf/cmd_show_test.go` | ABR/ASBR route snapshot → render | |
| `TestOSPFShowSPFRender` | `internal/plugins/ospf/cmd_show_test.go` | SPF-log entries render | |
| `TestOSPFShowProxyArgsRejected` | `internal/plugins/ospf/cmd_show_test.go` | extra/unknown args rejected (matches the ldp/isis proxy contract) | |
| `TestOSPFMaxMetricReflected` | `internal/plugins/ospf/cmd_show_test.go` | configured `max-metric router-lsa` is reflected in the process summary render | |
| `TestOSPFMetricsRegistered` | `internal/plugins/ospf/metrics_test.go` | every series in the umbrella "Metrics (canonical)" table is registered with its exact name and labels; none bare `ospf_*`; ospf-13 registers none itself (two-way guard) | |
| `TestOSPFDoctorRawSocketCheck` | `internal/plugins/ospf/doctor_test.go` | the ospf-3 raw-socket check / `doctor-ospf-raw-socket` code is surfaced here (not re-registered) | |
| `TestOSPFDoctorConfigSanity` | `internal/plugins/ospf/doctor_test.go` | router-id-missing / interface-area-unbound emit their codes; clean config emits none; fires only when `ospf` configured | |
| `TestOSPFCmdSchemaOwnsShowOSPF` | `internal/plugins/ospf/yang/cmd_schema_test.go` | owner-presence: `ze-ospf-cmd.yang` declares the `ze:command "ze-show:ospf-*"` show tokens (plugin-self-containment both-halves invariant) | |
| `TestOSPFCmdSchemaOwnsClearOSPF` | `internal/plugins/ospf/yang/cmd_schema_test.go` | owner-presence: `ze-ospf-cmd.yang` declares the `ze:command "ze-clear:ospf-*"` clear tokens | |
| `TestShowSchemaHasNoMigratedOwnerCommands` (extend) | `internal/component/cmd/show/yang/self_containment_test.go` | central-guard: the `ze-show:ospf-*` tokens are absent from the central show schema | |
| `TestClearOwnerRemovalLeavesNoResidue` (extend) | `internal/component/cmd/clear/yang/self_containment_test.go` | central-guard: the `ze-clear:ospf-*` tokens are absent from the central clear schema (the ACTUAL test name in that file) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `show ip ospf database <lsa-type>` arg count | 0..1 keyword | one LSA-type keyword | N/A | second positional rejected |
| LSA-type subview keyword | closed set | `nssa-external` | N/A | unknown keyword rejected (not a free value) |
| `max-metric router-lsa` on-startup / on-shutdown duration (seconds) | 5..86400 (RFC 6987 range) | 86400 | 4 rejected | 86401 rejected |
| SPF-log entries shown | 0..N retained | N | N/A | request beyond retained returns all retained |
| route/summary cost column (display, 24-bit metric) | 0..16777215 | 16777215 | N/A | values are read from the snapshot, never originated here; display the snapshot value, do not cap |

### Functional Tests
<!-- REQUIRED: Verify feature works from end-user perspective -->
The `test/ospf` suite is registered in `internal/test/cli/register.go` and
`mk/test-functional.mk`; ospf-4 establishes the suite (per the umbrella "Test +
interop wiring" row), and this spec adds the show and doctor `.ci` cases plus the
interop cases below to it. Raw-IP / multicast paths are Linux-only and run as
QEMU integration tests (`ai/rules/qemu-testing.md`), not plain `.ci`.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-show` | `test/ospf/ospf-show.ci` | each `show ip ospf <noun>` renders engine data; per-LSA-type database subviews filter; pipes work; `clear ip ospf` acts; `max-metric` reflected | |
| `ospf-doctor` | `test/ospf/ospf-doctor.ci` | `ze doctor --json` flags missing `CAP_NET_RAW` and bad config (router-id / interface-area) | |
| `ospf-metrics` | `test/ospf/ospf-metrics.ci` | metrics scrape exposes every canonical `ze_ospf_*` series with exact labels and no bare `ospf_*` | |
| `ospf-web` | `test/web/ospf-neighbor-database.wb` | web neighbour/database pages render and update from the OSPF snapshot/SSE path | |

### Interop Tests (MANDATORY for protocol features)
<!-- This table is the umbrella's goal-validation evidence; it is fully populated. -->
Interop against FRR `ospfd` is MANDATORY per `ai/rules/interop-and-goal-validation.md`
and the umbrella "Test + interop wiring" row: it is not optional and not deferrable.
The seven scenarios below are the full mandatory set. All seven DEPEND on FRR ospfd
runner support: `test/interop/daemons` must be set to `ospfd=yes` and `interop.py`
must mount that shared daemons file plus the scenario `frr.conf` so a live FRR
ospfd actually starts. Creating the scenario directories alone does NOT launch
ospfd; see Files to Modify (AC-21).

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-p2p-frr` | `test/interop/scenarios/ospf-p2p-frr/` | FRR ospfd | Point-to-point adjacency to Full + route convergence both ways | |
| `ospf-broadcast-frr` | `test/interop/scenarios/ospf-broadcast-frr/` | FRR ospfd | Broadcast DR/BDR election + Network-LSA + route convergence | |
| `ospf-multiarea-frr` | `test/interop/scenarios/ospf-multiarea-frr/` | FRR ospfd | Inter-area: area-1 prefix appears as Type 3 in area 0 across the ABR; reachable | |
| `ospf-stub-nssa-frr` | `test/interop/scenarios/ospf-stub-nssa-frr/` | FRR ospfd | Stub default injection (no Type 5) + NSSA Type 7 origination + Type 7→5 translation | |
| `ospf-redist-frr` | `test/interop/scenarios/ospf-redist-frr/` | FRR ospfd | OSPF route redistribution to BGP and connected/static/BGP import as Type 5 AS-External-LSAs | |
| `ospf-auth-frr` | `test/interop/scenarios/ospf-auth-frr/` | FRR ospfd | MD5 / HMAC auth: correct key forms adjacency, wrong key rejected | |
| `ospf-convergence-frr` | `test/interop/scenarios/ospf-convergence-frr/` | FRR ospfd | Link down → reconverge; SPF re-runs; stale routes withdrawn | |

### Future (if deferring any tests)
- Opaque-LSA / SR / TE / GR / BFD-for-OSPF interop are deferred with the corresponding out-of-scope features (umbrella out-of-scope table). Requires explicit user approval to defer.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` - add the `max-metric router-lsa` leaves (RFC 6987): a container with `administrative` (always) boolean, `on-startup` (boolean + `duration` 5..86400 s) and `on-shutdown` (boolean + `duration` 5..86400 s). The schema module itself is created by ospf-4; this child adds these leaves. Origination is wired in ospf-7; this child only adds the leaves and reflects them in `show ip ospf`
- `internal/component/cmd/show/yang/self_containment_test.go` - add the OSPF show tokens to the banned-token map in `TestShowSchemaHasNoMigratedOwnerCommands` (`ze-show:ospf`, `ze-show:ospf-neighbor`, `ze-show:ospf-interface`, `ze-show:ospf-database`, `ze-show:ospf-route`, `ze-show:ospf-border-routers`, `ze-show:ospf-spf`), so a `show ip ospf ...` command can never drift back into the central show schema (the central-guard half of `ai/rules/plugin-self-containment.md`, both-halves invariant)
- `internal/component/cmd/clear/yang/self_containment_test.go` - add the OSPF clear tokens to the banned-token map in `TestClearOwnerRemovalLeavesNoResidue` (the ACTUAL test name in that file): add `"ze-clear:ospf-process"`, `"ze-clear:ospf-neighbor"`, `"ze-clear:ospf-counters"`, so `clear ip ospf ...` can never drift into the central clear schema
- `internal/plugins/ospf/instance.go` - `OnExecuteCommand` handles `show ip ospf <noun>` / `clear ip ospf <action>` against sibling snapshots (per-LSA-type database filter, process summary incl. stub-router state, clear actions)
- `internal/plugins/ospf/cmd_show.go` - proxy RPC registrations for fixed `ze-show:ospf-*` and `ze-clear:ospf-*` methods
- `internal/component/web/handler_ospf.go`, `internal/component/web/page_ospf.go`, `internal/component/web/handler_ospf_test.go` - register the OSPF neighbour/database routes and SSE handlers (existing web component patterns)
- `test/interop/daemons` - set `ospfd=yes`. This shared daemons file is mounted read-only into the FRR container at `/etc/frr/daemons` by `interop.py`; FRR will NOT start `ospfd` while this is `no`, so merely creating the scenario dirs is insufficient
- `test/interop/interop.py` - ensure the interop runner actually launches FRR `ospfd` for the OSPF scenarios: confirm the per-scenario `frr.conf` (which carries the `router ospf` config) is mounted alongside the now-`ospfd=yes` daemons file; confirm the FRR container has the capabilities OSPF needs (`NET_ADMIN`/`NET_RAW` over the link); and if a multicast-capable bridge/veth link is required for OSPF frames (proto 89, `224.0.0.5`/`224.0.0.6`), extend the runner to provide it (links to A-6). The deliverable is that running an `ospf-*-frr` scenario starts a live FRR ospfd that reaches adjacency Full with Ze, not just that the scenario files exist
- `docs/features.md`, `docs/comparison.md`, `docs/guide/command-reference.md`, `docs/functional-tests.md`, `docs/plugin-development/metrics.md` - OSPF CLI / feature / comparison / functional-test / metrics rows
- `docs/guide/configuration.md`, `docs/architecture/api/commands.md`, `docs/plugin-overview.md`, `docs/guide/status.md` - OSPF config, central show/clear RPC surface, edge-plugin overview, and status/diagnostic docs

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Command/CLI YANG (show/clear command tree) | NEEDED | `internal/plugins/ospf/yang/ze-ospf-cmd.yang`: TWO separate augment statements because show and clear are distinct command-tree roots. (1) SHOW augment of `/clishowcmd:show` (import `ze-cli-show-cmd { prefix clishowcmd; }`) for `show ip ospf` (summary) + `neighbor/interface/database[ router\|network\|summary\|asbr-summary\|external\|nssa-external]/route/border-routers/spf`, each node `config false` with `ze:command "ze-show:ospf-<noun>"` (central SHOW namespace, model `ze-isis-cmd.yang` / `ze-ldp-cmd.yang`). (2) CLEAR augment of `/cliclearcmd:clear` (import `ze-cli-clear-cmd { prefix cliclearcmd; }`) for `clear ip ospf process/neighbor/counters`, each node `config false` with `ze:command "ze-clear:ospf-<action>"` (central CLEAR namespace, model the iface/ike/isis clears). Both verbs use a CENTRAL command namespace; there is NO per-component `ze-ospf-api.yang`. `clear` is its own root, NOT a child of `clishowcmd:show`. `scripts/checks/command_ownership.go` enforces that the owner ships this command YANG; `cmd_show.go` registration alone is insufficient |
| API/RPC YANG (RPC/query schema) | No | NO `ze-ospf-api.yang`. Both the show methods (`ze-show:ospf-*`) and the clear actions (`ze-clear:ospf-*`) are RPCs in CENTRAL namespaces, registered in Go via `pluginserver.RegisterRPCs` in `cmd_show.go` (model: IS-IS/LDP, which ship no api YANG for `ze-show:*`) |
| YANG config schema (new config) | Yes (small) | `internal/plugins/ospf/yang/ze-ospf-conf.yang` (module owned by ospf-4): add the `max-metric router-lsa` leaves (RFC 6987). No other config leaves are added by this child |
| YANG validation constraints | Yes | the `on-startup`/`on-shutdown` duration leaves carry `range "5..86400"` (RFC 6987); the `administrative` flag is boolean; the LSA-type and clear-action tokens are closed keyword sets in the command YANG |
| YANG custom validators | No | the new config leaves are native-typed (boolean + ranged uint); no custom validator needed here |
| CLI commands/flags | Yes | `internal/plugins/ospf/cmd_show.go`: `show ip ospf [neighbor\|interface\|database[ <lsa-type>]\|route\|border-routers\|spf]`, `clear ip ospf process/neighbor/counters` |
| CLI grammar (action before identifier) | Yes | `ai/rules/cli-grammar.md`: the LSA-type subview keyword and `database` are action keywords; `clear ip ospf <action>` is verb-form runtime action; any filter selector is typed |
| Editor autocomplete | Yes | closed noun/LSA-type/action set surfaced through the command registration so completion offers them |
| Functional test for new RPC/API | Yes | `test/ospf/ospf-show.ci`, `test/ospf/ospf-doctor.ci` |
| Pipe completeness | Yes | every `show ip ospf ...` routes JSON through `ApplyPipes`/`ProcessPipes`; `\| resolve`/`\| origin` apply on data; `ai/rules/pipe-completeness.md` |
| Env var registration | No | N/A (no env-only settings) |
| Doctor check for runtime dependencies | Yes | config-sanity checks (`doctor-ospf-router-id-missing`, `doctor-ospf-interface-area-unbound`) registered here, owned in the component (`codes.go` + `register.go`, the IS-IS pattern). The `CAP_NET_RAW`/raw-socket check and its `doctor-ospf-raw-socket` code are OWNED by ospf-3 (transport) and only SURFACED here; ospf-13 does not re-register them. `ai/rules/doctor-checks.md` |
| Prometheus counters/metrics | Yes (assert only) | the exact `ze_ospf_*` series + labels in the umbrella "Metrics (canonical)" table (the single source of truth); each owning spec (ospf-3/5/6/7/8/9/10/11/12) registers its rows, ospf-13 scrapes/asserts the full set and registers NONE |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (OSPF CLI/diag surface, max-metric) |
| 2 | Config syntax changed? | Yes (small) | `docs/guide/configuration.md` (the `max-metric router-lsa` leaves only; the core OSPF config is ospf-4) |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`show ip ospf ...`, `clear ip ospf ...`) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (new `ze-show:ospf-*` / `ze-clear:ospf-*` wire methods) |
| 5 | Plugin added/changed? | No | OSPF is a component (ospf-4), not a plugin dir |
| 6 | Has a user guide page? | Yes | `docs/guide/ospf.md` |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/ospf.md` (consolidated wire doc; interop is the proof) |
| 8 | Plugin SDK/protocol changed? | No | no SDK change |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc6987.md` (max-metric router-lsa); protocol RFC summaries are siblings |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (new `test/ospf/`), interop docs (new `ospf-*-frr` scenarios) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` |
| 12 | Internal architecture changed? | No | component itself is ospf-4; this child adds surfaces, not new internal architecture |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` (list the `ze_ospf_*` series + labels) |
| 15 | Registered plugin/event/command/capability changed? | Yes | `docs/plugin-overview.md`, `docs/guide/status.md` (new `show ip ospf ...`/`clear ip ospf ...` commands) |
| 16 | Changed files referenced by doc source anchors? | No | grep `docs/` for source anchors at completion |
| 17 | Existing docs show examples for this area? | No | grep at completion; update any stale OSPF examples |

## Files to Create
- `internal/plugins/ospf/yang/ze-ospf-cmd.yang` - CLI command tree with TWO separate augment statements, one per command-tree root. (1) SHOW augment: `augment "/clishowcmd:show"` (import `ze-cli-show-cmd { prefix clishowcmd; }`, model `internal/plugins/isis/yang/ze-isis-cmd.yang`) for `show ip ospf` (summary) + `neighbor/interface/database[ <lsa-type>]/route/border-routers/spf`, each node `config false` with `ze:command "ze-show:ospf-<noun>"`. (2) CLEAR augment: `augment "/cliclearcmd:clear"` (import `ze-cli-clear-cmd { prefix cliclearcmd; }`) for `clear ip ospf process/neighbor/counters`, each node `config false` with `ze:command "ze-clear:ospf-<action>"`. `clear` is a SEPARATE command-tree root, distinct from `clishowcmd:show`; clear nodes MUST NOT hang off the show root. Both verbs bind into CENTRAL command namespaces; there is NO per-component api YANG module (no ze-ospf-api). Required because `scripts/checks/command_ownership.go` enforces owner command YANG for these show/clear verbs; `internal/plugins/ospf/cmd_show.go` RPC registration alone does not satisfy the gate
- `internal/plugins/ospf/cmd_show.go` - `pluginserver.RegisterRPCs` for the show methods `ze-show:ospf-*` AND the clear actions `ze-clear:ospf-process/neighbor/counters`; proxy handlers forwarding via `Dispatcher().ForwardToPlugin` (model: IS-IS/LDP; never re-`Dispatch`). No per-component api YANG module is created
- `internal/plugins/ospf/yang/cmd_schema_test.go` - the OWNER-presence half of the `ai/rules/plugin-self-containment.md` both-halves invariant: `TestOSPFCmdSchemaOwnsShowOSPF` asserts `ze-ospf-cmd.yang` declares the `ze:command "ze-show:ospf-*"` show tokens, and `TestOSPFCmdSchemaOwnsClearOSPF` asserts it declares the `ze:command "ze-clear:ospf-*"` clear tokens (mirrors `internal/plugins/isis/yang/cmd_schema_test.go`). The matching central-guard halves are added to the show/clear self-containment test files (Files to Modify)
- `internal/plugins/ospf/cmd_show_test.go` - render/format + proxy-arg unit tests (`TestOSPFShow*Render`, `TestOSPFShowProxyArgsRejected`, `TestOSPFMaxMetricReflected`)
- `internal/plugins/ospf/metrics_test.go` - `TestOSPFMetricsRegistered`: ospf-13 owns NO metric series (per the umbrella "Metrics (canonical)" table, every `ze_ospf_*` series is owned and registered by its producing spec: ospf-3/5/6/7/8/9/10/11/12). This child does NOT centrally register metrics; it only ASSERTS (via a scrape) that the full canonical set is present with correct labels, in both directions (every canonical series present + no unexpected `ze_ospf_*`). Do NOT add a central metrics registry source file here (no component-root metrics.go, by design)
- `internal/plugins/ospf/doctor.go` - config-sanity checks ONLY (`doctor-ospf-router-id-missing`, `doctor-ospf-interface-area-unbound`); the raw-socket/`CAP_NET_RAW` check lives in ospf-3's `internal/plugins/ospf/transport/doctor_linux.go` and is surfaced, not duplicated, here
- `internal/plugins/ospf/codes.go` - code metadata for the two config-sanity codes, owned in the component (the IS-IS pattern; core `diagnostic/codes.go` carries only a guard comment, no registration)
- `internal/plugins/ospf/doctor_test.go` - doctor check unit tests (`TestOSPFDoctorConfigSanity`, `TestOSPFDoctorRawSocketCheck` surface assertion)
- `internal/component/web/handler_ospf.go`, `internal/component/web/page_ospf.go`, `internal/component/web/handler_ospf_test.go` - OSPF neighbour + database views (templates/handlers/SSE per the existing web layout, model `handler_isis.go`/`page_isis.go`)
- `test/ospf/ospf-show.ci` - show/clear functional test (all nouns, per-LSA-type subviews, pipes, clear actions, max-metric reflection)
- `test/ospf/ospf-doctor.ci` - doctor functional test (`ze doctor --json` raw-socket explain + config-sanity)
- The seven interop scenario dirs below DEPEND on the FRR ospfd runner support in Files to Modify (`test/interop/daemons` set to `ospfd=yes`, `interop.py` launching FRR ospfd); the dirs alone do not start ospfd
- `test/interop/scenarios/ospf-p2p-frr/{ze.conf,frr.conf,check.py}`
- `test/interop/scenarios/ospf-broadcast-frr/{ze.conf,frr.conf,check.py}`
- `test/interop/scenarios/ospf-multiarea-frr/{ze.conf,frr.conf,check.py}`
- `test/interop/scenarios/ospf-stub-nssa-frr/{ze.conf,frr.conf,check.py}`
- `test/interop/scenarios/ospf-redist-frr/{ze.conf,frr.conf,check.py}`
- `test/interop/scenarios/ospf-auth-frr/{ze.conf,frr.conf,check.py}`
- `test/interop/scenarios/ospf-convergence-frr/{ze.conf,frr.conf,check.py}`
- `docs/guide/ospf.md` - OSPF user guide (config + show/clear + troubleshooting)
- `docs/architecture/wire/ospf.md` - OSPF wire/protocol overview doc
- `rfc/short/rfc6987.md` - OSPF Stub Router Advertisement summary (if not created by ospf-7)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + the umbrella row ospf-13 |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan - check what siblings already provide |
| 3. Wiring phase | Wiring Test table - register `ze-show:ospf-*`, write failing `ospf-show.ci` |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-14. | Standard flow (incl. interop run) |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - register `ze-show:ospf-*` / `ze-clear:ospf-*` RPCs and a failing functional test
   - Tests: `test/ospf/ospf-show.ci` (fails: handlers return stubbed empty data)
   - Files: `internal/plugins/ospf/cmd_show.go`, `internal/plugins/ospf/yang/ze-ospf-cmd.yang`
   - Verify: `show ip ospf neighbor` reaches the engine command but returns empty until snapshots wired; command-ownership check passes
2. **Phase: Show renderers** - render each snapshot (summary, neighbor, interface, database[+per-LSA-type], route, border-routers, spf)
   - Tests: `TestOSPFShow*Render`, `TestOSPFShowProxyArgsRejected`
   - Files: engine `OnExecuteCommand` handlers + `cmd_show.go`
   - Verify: each command renders sibling snapshot data; the per-LSA-type filter narrows the database; pipes pass
3. **Phase: max-metric config + reflection** - add the `max-metric router-lsa` leaves to `ze-ospf-conf.yang`; reflect active stub-router state in `show ip ospf`
   - Tests: `TestOSPFMaxMetricReflected`, boundary test on the duration leaves
   - Files: `internal/plugins/ospf/yang/ze-ospf-conf.yang`, summary renderer (origination is ospf-7)
   - Verify: config validates with the duration range; `show ip ospf` reports the active state
4. **Phase: Clear actions** - `clear ip ospf process/neighbor/counters`
   - Tests: `ospf-show.ci` clear assertions
   - Files: engine command dispatch
   - Verify: SPF re-runs / adjacencies re-form / counters reset; grammar is verb-form runtime action
5. **Phase: Metrics (assert only)** - confirm the full canonical `ze_ospf_*` set is registered by its per-owner specs (ospf-3/5/6/7/8/9/10/11/12 per the umbrella table); ospf-13 registers NONE itself
   - Tests: `TestOSPFMetricsRegistered` (two-way guard), metrics scrape assertion in the functional test
   - Files: `internal/plugins/ospf/metrics_test.go` (assertion only; no `metrics.go` registry)
   - Verify: scrape lists every canonical series with its labels; no series registered centrally here
6. **Phase: Doctor** - config-sanity checks here (register `doctor-ospf-router-id-missing` / `doctor-ospf-interface-area-unbound`); the raw-socket check + `doctor-ospf-raw-socket` code come from ospf-3 and are only surfaced
   - Tests: `TestOSPFDoctorConfigSanity`, `TestOSPFDoctorRawSocketCheck`, `test/ospf/ospf-doctor.ci`
   - Files: `internal/plugins/ospf/doctor.go`, `internal/plugins/ospf/codes.go`
   - Verify: config-sanity checks fire only when `ospf` configured; raw-socket code (from ospf-3) and the two new codes are explainable via `ze explain`; no double registration
7. **Phase: Web** - OSPF neighbour + database pages with SSE
   - Tests: `test/web/ospf-neighbor-database.wb`
   - Files: `internal/component/web/handler_ospf.go`, `page_ospf.go`, `handler_ospf_test.go`
   - Verify: pages render and update live; no goroutine leak on disconnect
8. **Phase: Interop** - FRR ospfd runner support, then the seven FRR scenarios
   - Tests: `test/interop/scenarios/ospf-*-frr/check.py`
   - Files: `test/interop/daemons` (set `ospfd=yes`), `test/interop/interop.py` (launch FRR ospfd; provide a multicast-capable link if needed, A-6), then `ze.conf`, `frr.conf`, `check.py` per scenario
   - Verify: a live FRR ospfd starts and reaches adjacency Full (AC-21); each scenario asserts its specific behaviour and stability
9. **Documentation** - guide, wire doc, comparison/features/command-reference/functional-tests/metrics rows
10. **Full verification** - `make ze-verify` + `make ze-interop-test` for the OSPF scenarios
11. **Complete spec** - audit tables, learned summary, two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line; all six interop scenarios exist and pass |
| Feature completeness | Every End-to-End User Story has a working path; show noun set matches the guide §11 list (normalised to Ze grammar) plus `border-routers` and the per-LSA-type database subviews |
| Correctness | Renders match sibling snapshot fields; per-LSA-type filter narrows to the right LS Type; max-metric reflection matches the active engine state; metrics values track engine state |
| Naming | CLI `show ip ospf <noun>` / `clear ip ospf <action>`; metric series `ze_ospf_*`; doctor codes `doctor-ospf-*` |
| Data flow | show → dispatcher → `ForwardToPlugin` → engine snapshot → pipes; no reach into engine internals; metrics observe, do not mutate |
| CLI grammar | action before identifier; `database <lsa-type>` keyword; typed selectors; clear is verb-form runtime action (`ai/rules/cli-grammar.md`) |
| Pipe completeness | every show command routes through `ApplyPipes`; `\| resolve`/`\| origin` apply on data (`ai/rules/pipe-completeness.md`) |
| Doctor checks | check gates on `ospf` config present; codes registered + explainable; unit + functional tests; raw-socket surfaced not re-registered (`ai/rules/doctor-checks.md`) |
| Prometheus counters | full canonical `ze_ospf_*` set asserted present with names + labels; none bare `ospf_*`; ospf-13 registers none |
| Rule: plugin-self-containment | all OSPF show/clear/metrics-assert/doctor code under `internal/plugins/ospf/`; removing the component removes them |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Command YANG (single, no api yang) | `ls internal/plugins/ospf/yang/ze-ospf-cmd.yang`; `grep -L ze-ospf-api internal/plugins/ospf/yang/*.yang` (no api module); `make ze-command-ownership-check` passes |
| `cmd_show.go` with all show/clear RPCs | `ls internal/plugins/ospf/cmd_show.go`; grep `ze-show:ospf-` and `ze-clear:ospf-` |
| max-metric config + reflection | grep `max-metric` in `ze-ospf-conf.yang`; `TestOSPFMaxMetricReflected` PASS; `ospf-show.ci` asserts the summary field |
| Show functional test | `ls test/ospf/ospf-show.ci` |
| Doctor functional test + codes | `ls test/ospf/ospf-doctor.ci`; grep `doctor-ospf-` in the component `codes.go` |
| Metrics series | scrape assertion lists `ze_ospf_*`; `TestOSPFMetricsRegistered` PASS |
| Web pages | `ls` the OSPF web view files; live-update test passes |
| FRR ospfd runner support | `grep '^ospfd=yes' test/interop/daemons`; an `ospf-*-frr` scenario run shows a live FRR ospfd process (the runner launches it), not just the scenario dir |
| Six FRR interop scenarios | `ls test/interop/scenarios/ospf-*-frr/` (6 dirs, each with ze.conf/frr.conf/check.py), each running against the live FRR ospfd |
| Docs | `ls docs/guide/ospf.md docs/architecture/wire/ospf.md`; comparison/features/command-reference/metrics rows present |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | show/clear handlers reject extra/unknown args (proxy contract); the LSA-type and clear-action keyword sets are closed; snapshot rendering bounds-checks slices it does not own |
| Information disclosure | show output does not leak auth keys; `show ip ospf interface` shows auth-configured (type), not key material |
| Resource exhaustion | spf-log and LSDB renders are bounded; SSE streams close on disconnect; metrics cardinality bounded (no per-LSA / per-neighbour labels) |
| Privilege | doctor surfaces the `CAP_NET_RAW` requirement; no privileged action performed by the show/clear layer |
| Negative interop | `ospf-auth-frr` proves a wrong key is rejected (security behaviour proven against FRR) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read sibling snapshot API; if engine data is missing, route to the owning sibling spec (ospf-5/7/8) |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Interop mismatch | Capture with tcpdump, compare to FRR `show ip ospf database`, fix codec/FSM in the owning sibling |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE - write IMMEDIATELY when you learn something -->
- This child is the umbrella's goal-validation surface: the six FRR scenarios are where the umbrella ACs are proven against a real implementation, not just asserted by unit tests.
- The show layer carries no protocol logic; it is a dispatcher proxy over engine snapshots (the LDP/IS-IS model), which keeps the component self-contained and the engine the single source of truth.
- ospf-13 owns NO metric series: it asserts the full canonical `ze_ospf_*` set is exposed by its per-owner specs. The only registrations this child owns are the two config-sanity doctor codes and the small `max-metric router-lsa` config leaves.

## Core Insight
The presentation/diagnostic/interop layer is "thin glue over snapshots plus a
real peer." Its value is not new logic but proof: it makes the engine observable
(CLI/web/metrics), checks readiness (doctor), and demonstrates correctness over
the wire against FRR `ospfd`. The risk concentrates in interop (DR/BDR election,
timer/encoding differences, multi-area/stub-NSSA), which routes fixes back to the
owning engine siblings, not here.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Show commands proxy to engine commands via `ForwardToPlugin` (IS-IS/LDP model) | Renderer queries engine snapshots directly from `cmd_show.go`; or `Dispatch` (recurses) | Keeps protocol logic in the engine, the command self-contained, matches the established pattern, and avoids the builtin re-match recursion trap |
| ospf-13 asserts the canonical metric set, registers none | Register a central OSPF metrics.go here | One series, one owner (the producing spec); a central registry would split ownership and risk silent rename/leak |
| Doctor owns ONLY config-sanity checks here; ospf-3 owns the raw-socket check + code | Register the raw-socket code here too | One code, one owner: ospf-3 (transport) owns `doctor-ospf-raw-socket`; this child registers only the two config-sanity codes and surfaces the raw-socket result. Double registration of one code is a bug |
| max-metric: config leaves + `show` reflection here; origination in ospf-7 | Own the whole RFC 6987 feature here | Origination is engine-state mutation (LSDB), which is the ospf-7 concern; ospf-13 owns only the operator-facing surface (config + reflection) |
| Unnumbered interop dir names (`ospf-p2p-frr`) | Numbered `NN-ospf-*` to match an existing convention | The umbrella references the unnumbered names; confirm runner discovery and adopt numbering only if required (R-6) |

## Known Limitations
- Display-only for OSPF state: this child renders, reflects and clears; it does not originate or modify protocol state (that is the siblings' job, incl. the max-metric Router-LSA origination in ospf-7).
- Opaque-LSA / SR / TE / GR / BFD-for-OSPF interop are out of scope (umbrella out-of-scope table); only base P2P/broadcast/multi-area/stub-NSSA/auth/convergence are validated against FRR.
- Web views cover neighbour and database only; interface/route/border-routers/spf remain CLI-first in v1.

## RFC Documentation

Add `// RFC 6987 Section X.Y: "<quoted requirement>"` above the max-metric config
handling and its reflection in `show ip ospf`. Protocol RFC constraint comments
(RFC 2328 / 3101 / 5709 / 7474) live in the owning siblings (ospf-2/5/6/7/8/9/10/11/12);
this child references them, it does not re-implement them.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered — add test for each]

### Documentation Updates
- [Docs updated, with source anchors named, or "None" with grep evidence]
- [If docs were changed: `make ze-doc-test` result]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

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

<!-- MANDATORY: Maps each stated goal to concrete proof it was achieved. -->
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Command surface observable (all `show ip ospf` nouns + per-LSA-type subviews + clear actions) | Functional test | `test/ospf/ospf-show.ci` dispatches every show noun, the six database subviews and the three clear actions; render unit tests `TestOSPFShow*Render` PASS |
| max-metric router-lsa config + reflection | Functional + unit | `ospf-show.ci` asserts the summary stub-router field; `TestOSPFMaxMetricReflected` PASS; duration boundary test PASS |
| Diagnostics observable (doctor codes + explain) | Functional + unit | `test/ospf/ospf-doctor.ci`; `TestOSPFDoctorConfigSanity`, `TestOSPFDoctorRawSocketCheck` PASS |
| Metrics canonical set exposed | Unit test | `TestOSPFMetricsRegistered` PASS (exact `ze_ospf_*` names + labels, none bare, two-way guard) |
| Mesh over OSPF P2P (adjacency Full, routes both ways) | Interop against FRR ospfd | `ospf-p2p-frr` `check.py` waits for Full + asserts convergence |
| Broadcast DR/BDR election | Interop against FRR ospfd | `ospf-broadcast-frr` `check.py` asserts DR/BDR + Network-LSA + convergence |
| Inter-area (Type 3 across an ABR) | Interop against FRR ospfd | `ospf-multiarea-frr` `check.py` asserts the area-1 prefix reachable in area 0 |
| Stub + NSSA (default injection, Type 7→5 translation) | Interop against FRR ospfd | `ospf-stub-nssa-frr` `check.py` asserts stub default + NSSA translation |
| Authentication (auth adjacency, wrong key rejected) | Interop against FRR ospfd | `ospf-auth-frr` `check.py` asserts correct-key Full / wrong-key reject |
| Convergence (link-down reconvergence, stale withdraw) | Interop against FRR ospfd | `ospf-convergence-frr` `check.py` asserts reconvergence + withdraw |

## Review Gate

<!-- BLOCKING (rules/planning.md Completion Checklist step 7): -->
<!-- Run /ze-review BEFORE the final testing/verify step. Record the findings here. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | pending | `/ze-review` not run yet for this design spec | this spec | run during implementation; record concrete findings here |

### Fixes applied
- Pending: record concrete fixes after `/ze-review` reports BLOCKER or ISSUE findings.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-21 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/ospf/`, web, metrics-assert, doctor)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added (RFC 6987 max-metric)
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
- [ ] Interop tests for protocol features (six FRR scenarios)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-ospf-13-cli-diag-interop.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-13-cli-diag-interop.md` only (preserves edited spec in git history from commit A)
