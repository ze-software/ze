# Spec: followup-test-infra

| Field | Value |
|-------|-------|
| Status | done |
| Depends | - |
| Phase | 7/7 |
| Updated | 2026-07-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/listener.go` - conflict detector (property target)
4. `internal/test/runner/`, `mk/test-integration.mk`, `scripts/evidence/qemu-all-tests.sh` - runner + QEMU privilege path
5. `internal/chaos/engine/action.go` - chaos fault-action surface
6. `git log -p plan/deferrals.md` (pre-2026-07-06) - original deferral rows + evidence

## Task

Build the missing test infrastructure that blocks four classes of deferred tests. Each needs a framework/runner that does not exist yet in-tree; the individual tests are cheap once it lands.

This was a consolidation skeleton created from verified deferral survivors (backlog triage 2026-07-06). Designed 2026-07-09; all evidence re-verified at that date.

### Work items (migrated from the 2026-07-06 deferral triage; `L#` = row in the pre-triage `plan/deferrals.md`)

- **Property-test framework (L92,L93,L94,L96)** - no `testing/quick`/`rapid` harness in-tree ~~(only vendored)~~ (design correction: nothing property-related is vendored either). Adopt one, then: listener-conflict ~~symmetric+transitive~~ symmetric + wildcard-dominance properties (L92 - see correction below), round-trip migration (L93), overflow ordering under concurrency (L94), filter-chain random UPDATEs (L96).
- **Privileged CI runner (L200,L197)** - no CAP_NET_ADMIN/root+netns in the shared `.ci` runner. L200: `tc qdisc show` kernel-state assertion on `test/traffic/001-boot-apply.ci` (and `002-reload-apply.ci`, restored from the original row). L197: 1M-prefix pprof comparison run (benchmark+harness exist, only the privileged run is outstanding).
- **Two-peer wire-forwarding proof (L121,L80)** - conn->peer determinism already resolved (`internal/test/peer/peer_connmap.go`). Remaining: MP_REACH next-hop-self two-peer `.ci` (L121) and LLGR egress-suppress multi-peer test (L80, plus the route-reflection multi-peer test from the original row, restored).
- **Stress / chaos gaps (L95,L97,L98)** - iface chaos harness - ze-chaos is BGP-only (L95); web concurrent-edit stress (L97); fleet >100-concurrent-client perf (L98).

### Design-time corrections (2026-07-09, verified with file:line)

| Triage claim | Reality today |
|--------------|---------------|
| rapid "only vendored" | No property-test library exists in go.mod or vendor/ at all; the only matches are stdlib name manifests in x/tools |
| L92 "symmetric+transitive" | `conflicts()` (`config/listener.go:289-297`, verified firsthand) is symmetric by construction but **provably not transitive**: wildcard `0.0.0.0:80` conflicts with `1.1.1.1:80` and `2.2.2.2:80`, which do not conflict with each other (`ipsConflict` :302+). The property set is redefined below |
| `cmd/ze-chaos/` in Files to Modify | Does not exist - ze-chaos is `cmd/ze` under build tag `ze_chaos` (`Makefile:177`); chaos code is `internal/chaos/`; `mk/test-chaos.mk:13` `CHAOS_PACKAGES` points at the nonexistent path (fix in this spec) |
| "multi-peer test infrastructure not yet available" (llgr-readvertise.ci comment) | Stale: 10 two-ze-peer `.ci` tests ship today (e.g. `forward-two-tier-under-load.ci`, `redistribute-l2tp-multi-peer-nexthop.ci`); item 3 is test authoring, not infra |
| Filter chain (L96 target) | Rearchitected after the triage: commit `d7d925cc6` unified ingress/egress into one stage-ordered pass (`reactor/filter_ordered.go`); the property test targets the NEW pass |
| No privileged .ci path at all | The QEMU harness runs whole suites as root (`option=needs-linux` + `make ze-qemu-needs-linux-test`, `mk/test-integration.mk:198-208`, `scripts/evidence/qemu-run.py` ssh root@); the gap is that the `traffic` suite is absent from `scripts/evidence/qemu-all-tests.sh:124-136` |

## Required Reading

### Source files / docs

- [ ] `ai/rules/go-standards.md` (Dependencies section :26-28)
  → Constraint: "Never add new third-party imports (not already in go.mod) without asking the user first" - stdlib `testing/quick` needs no approval; `pgregory.net/rapid` does
- [ ] `internal/component/config/listener.go`
  → Constraint: properties that HOLD: symmetry (pairwise scan :258-273 + symmetric `conflicts()`), irreflexivity for distinct services on distinct ports, wildcard dominance (wildcard ~ every same-family IP, same proto+port), cross-family independence, protocol independence; transitivity does NOT hold - never assert it
- [ ] `internal/exabgp/migration/` (schema.go:46 ParseExaBGPConfig, migrate.go:58 MigrateFromExaBGP, migrate_serialize.go:17 SerializeTree)
  → Constraint: no ze→exabgp reverse converter exists; "round-trip" = random exabgp config → migrate → serialize → re-parse as ze tree → semantic equivalence assertions (peer count, families, policies), not byte equality
- [ ] `internal/component/bgp/reactor/forward_pool.go` (TryDispatch :515, DispatchOverflow :590, runWorker :1100-1120)
  → Constraint: end-to-end ordering already proven by `test/plugin/forward-two-tier-under-load.ci`; the property test adds randomized concurrent-dispatch coverage at unit level (withdrawals-first supersede rules per `forward_pool_supersede_test.go:247,273`)
- [ ] `internal/component/bgp/reactor/filter_ordered.go` + `forward_build.go` (buildModifiedPayload :58)
  → Constraint: L96 targets the unified stage-ordered pass (post-`d7d925cc6`); property: random UPDATE payloads through random filter-chain configs never panic, never emit malformed payloads, stage order invariants hold
- [ ] `internal/test/runner/` (parsing.go:214, record_parse.go:367-379 `option=needs-linux`), `mk/test-integration.mk` (:198-208 QEMU needs-linux, :57-60 ze-stress-profile), `scripts/evidence/qemu-all-tests.sh` (:124-136 suite list)
  → Decision: L200 is solved by adding the `traffic` suite to the QEMU suite list + `option=needs-linux`-gated variants asserting `tc qdisc show` - NOT by building a new privileged runner
- [ ] `internal/plugins/traffic/netlink/integration_linux_test.go` (withTrafficNetNS :19-56)
  → Constraint: the privileged Go-test pattern: `//go:build integration && linux`, netns.NewNamed, `t.Skipf` on missing CAP_NET_ADMIN
- [ ] `internal/test/peer/peer_connmap.go` (runConnMap :36, sortConnBatch :155), `internal/test/cli/cmd_peer.go` (:105-118 flags)
  → Constraint: deterministic conn→peer by router-id or remote-ip (`conn_map` option, expect.go:123-126); .ci runner auto-creates loopback aliases for `--bind` (runner_exec.go:406-410)
- [ ] `internal/component/bgp/plugins/gr/gr_egress.go` (LLGREgressFilter :57)
  → Constraint: behavior to prove multi-peer: stale route → non-LLGR EBGP peer withdrawn (:99), non-LLGR IBGP peer gets NO_EXPORT + LOCAL_PREF=0 (:89-91), LLGR-capable peer passes through (:76-80)
- [ ] `internal/chaos/engine/action.go` (:12-43)
  → Constraint: all 15 ActionTypes are BGP/TCP session faults + config-reload; iface faults (link flap, addr remove) are a new action family; scenario/orchestrator layers consume ActionType - extension is additive
- [ ] `internal/component/managed/`, `cmd/ze/hub/managed_server.go`, `pkg/fleet`
  → Constraint: fleet = managed-config system; dedicated hub listener since `643499bfc`; perf test = many concurrent fleet clients against one hub
- [ ] `ai/rules/qemu-testing.md`
  → Constraint: linux-only test code MUST have QEMU integration coverage; never skip for "needs hardware"

**Key insights:**
- Nothing here blocks on external decisions except the optional `rapid` adoption (user ask at implement time; stdlib default needs none).
- Two of the four "infrastructure" items are actually wiring/authoring on existing infrastructure (privileged runs via QEMU; two-peer tests via existing conn_map).
- The chaos and fleet items are the only genuinely new harness code.

## Current Behavior (MANDATORY)

**Source files read (2026-07-09):**

- [ ] `internal/component/config/listener.go` - pairwise detector, wildcard semantics (verified firsthand :258-314)
- [ ] `internal/test/runner/`, `mk/test-integration.mk`, `scripts/evidence/qemu-all-tests.sh` - needs-linux QEMU path exists; traffic suite not enrolled
- [ ] `test/traffic/001-boot-apply.ci` - asserts exit 0 + stderr message only; header documents the deferred qdisc assertion
- [ ] `test/stress/` harness + `mk/test-integration.mk:57-60` - 1M-prefix profile run fully scripted (`make ze-stress-profile`, sudo + ZE_PPROF)
- [ ] `internal/test/peer/peer_connmap.go` - deterministic multi-connection mapping
- [ ] `internal/chaos/engine/action.go` - BGP-only fault list
- [ ] `internal/component/bgp/reactor/filter_ordered.go`, `forward_pool.go` - property-test targets

**Behavior to preserve:**
- Runner semantics for existing `.ci` suites; `make ze-verify` stays unprivileged.
- Existing goldens/expectations of the 10 two-peer `.ci` tests and `llgr-readvertise.ci` (its stale comment gets corrected, not its assertions).
- Chaos scenario/orchestrator API for existing BGP actions.

**Behavior to change:**
- New property-test package(s); traffic suite enrolled in QEMU; new `.ci` tests; new chaos action family; new stress/perf tests; stale `mk/test-chaos.mk:13` pointer fixed.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Go `testing` harness (property + stress tests); `.ci` runner via `ze-test` (functional); QEMU harness for privileged suites; `internal/chaos` orchestrator for fault injection

### Transformation Path
1. Property engine generates random inputs → unit under test → invariant assertions
2. `.ci` directives → runner → daemon/peers → wire/kernel observation (`tc qdisc show` under QEMU root)
3. Chaos scenario → engine action (new: iface fault) → netlink manipulation → BGP/iface observers assert recovery
4. Fleet perf: N goroutine clients → managed hub listener → latency/error metrics recorded

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `.ci` runner → kernel | QEMU root execution (`option=needs-linux`) for real qdisc state | [ ] |
| Go test → property engine | generated inputs drive the unit under test | [ ] |
| chaos engine → kernel iface | netlink link/addr manipulation (privileged, integration-tagged) | [ ] |
| fleet clients → hub | managed listener (TLS) | [ ] |

### Integration Points
- `internal/test/runner/` (the .ci runner - no privilege changes; QEMU enrollment only)
- `internal/test/peer/` (existing multi-peer harness)
- `internal/chaos/engine/` (new action family)
- `scripts/evidence/qemu-all-tests.sh`, `mk/test-integration.mk`, `mk/test-chaos.mk`

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - new commands/views/families/handlers register and are core-discovered, not hardcoded into a core/shared package (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Corrected evidence holds at implement time | re-verified 2026-07-09 (firsthand: listener.go:258-314; agent research for the rest) | Re-scope item | grep/LSP at implement-audit | confirmed (one refinement: migration keys peers by derived name peer-N, IP lives at connection.remote.ip -- handled in the round-trip collector) |
| A-2 | stdlib `testing/quick` is expressive enough for the four property targets | properties are invariant checks over generated structs/payloads | Ask user to approve `pgregory.net/rapid` (go-standards.md:28) and switch | write L92 property first as pilot | confirmed -- all four property tests written and green on stdlib quick with custom Generate methods + fixed seeds; no `rapid` needed |
| A-3 | QEMU suite enrollment for traffic is mechanical (add to qemu-all-tests.sh list + needs-linux options) | plugin/l2tp/firewall suites already enrolled (:124-136) | Small runner fixes; fall back to integration-tagged Go test asserting qdisc | run `make ze-qemu-needs-linux-test` locally | confirmed (enrollment = one `fsuite traffic` line; needs-linux qdisc tests 022/023 authored + parse OK). QEMU EXECUTION env-blocked: `qemu-system-*` not installed in this sandbox; `option=needs-linux` gates on GOOS only, so natively it runs and fails (no eth0/CAP_NET_ADMIN). Runbook: `make ze-qemu-needs-linux-test`. |
| A-4 | `make ze-stress-profile` completes on this host with sudo and produces the pprof artifacts | harness + scenario complete (test/stress/run.py 05-profile-1m) | Fix harness bit-rot first; record in Mistake Log | execute the run (L197 deliverable) | ENV-BLOCKED (AC-3): no sudo/root, no netns/CAP_NET_ADMIN in this sandbox (per plan/known-failures.md). Run not executed; runbook recorded in Known Limitations. |
| A-5 | Chaos iface actions can reuse the engine's action plumbing without orchestrator schema changes beyond additive enum values | ActionType is a closed list consumed by scenario parser (action.go:12-43) | Extend scenario schema; still additive | phase 5 wiring test | confirmed -- two ActionTypes + names/maps/params/IsV2Action/guard/scheduler-weight all additive; no orchestrator schema change; unit tests green; direct `ze-chaos --chaos-actions iface-link-flap` exits 0. |
| A-6 | Fleet perf target ">100 concurrent clients" is a Go-test-level harness (no new binary) | managed client is a library (`internal/component/managed/client.go`) | Add a ze-test mode instead | phase 6 design review | confirmed -- 128-client Go test, no new binary. DEVIATION: test lives in `cmd/ze/hub/fleet_perf_test.go` (package hub), not `internal/component/managed/`, because the real hub+TLS+cap harness (`startManagedServer`) is there; `internal/component/managed` only has a net.Pipe mock. |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Property tests are flaky under CI time limits (random input explosion) | intermittent timeouts | Fixed seeds in CI mode + bounded iteration counts; quick.Config MaxCount tuned |
| R-2 | Transitivity myth resurfaces (someone "fixes" the property to assert it) | property test asserting transitivity | This spec + test comment document the wildcard counterexample |
| R-3 | QEMU traffic enrollment surfaces latent traffic .ci failures under root | qemu run red on unrelated assertions | Fix-forward per `ai/rules/no-workarounds-for-missing-behavior.md`; never weaken the tests |
| R-4 | 1M-prefix privileged run misses the L197 numeric targets (heap <12MB, GC <22%) | pprof table above targets | The run is the deliverable; misses become a new perf spec (record deferral with destination) |
| R-5 | Iface chaos in shared netns wrecks host networking when run unprivileged/wrong | link flap on a real interface | Actions operate only inside named netns/veth pairs created by the harness; integration-tagged |
| R-6 | Fleet 100-client test is resource-heavy and flaky in CI | timeouts in shared runners | Mark perf tests as evidence/release targets (like ze-traffic-test), not ze-verify |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Property harness generates random listener sets | → | `FindListenerConflict` invariants (symmetry, wildcard dominance, no cross-family) | `TestListenerConflictProperties` |
| Random exabgp config generated | → | migrate → serialize → re-parse semantic equivalence | `TestMigrationRoundTripProperty` |
| Random concurrent dispatch bursts | → | fwdPool ordering invariants (withdrawals-first, per-peer order) | `TestForwardPoolOrderingProperty` |
| Random UPDATE payloads + random filter chains | → | unified stage-ordered pass emits well-formed payloads | `TestFilterChainRandomUpdatesProperty` |
| QEMU root runs traffic suite | → | `tc qdisc show` reflects applied HTB/TBF config | `test/traffic/001-boot-apply.ci` + `002-reload-apply.ci` (needs-linux variants) |
| `make ze-stress-profile` (sudo) | → | 1M-prefix run captures cpu/heap/goroutine pprof | recorded pprof table in this spec (L197 evidence run) |
| Peer A announces MP_REACH IPv6 route; ze forwards to peer B with next-hop-self | → | `buildModifiedPayload` MP_REACH rewrite on the wire | `.ci` `forward-mpreach-nexthop-self-two-peer` |
| GR stale routes with one LLGR and one non-LLGR peer | → | `LLGREgressFilter` per-peer divergence on the wire | `.ci` `llgr-egress-suppress-multi-peer` |
| Stale routes toward a route-reflector client vs non-client | → | reflection path preserves LLGR egress rules | `.ci` `llgr-egress-rr-multi-peer` |
| Chaos scenario with `iface-link-flap` action | → | new iface action family executes + BGP session recovers | `TestChaosIfaceLinkFlap` (integration-tagged) + chaos scenario file |
| 50 concurrent web editor sessions mutate config | → | no data race, no lost commit, bounded latency | `TestWebConcurrentEditStress` |
| 128 fleet clients connect + fetch config | → | managed hub serves all within budget | `TestFleetManyClientsPerf` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Property package landed (stdlib `testing/quick` default; `rapid` only with recorded user approval) | Four property tests exist and pass with fixed-seed CI mode; L92 asserts symmetry/wildcard-dominance/irreflexivity/cross-family/protocol independence and explicitly documents the transitivity counterexample |
| AC-2 | `make ze-qemu-needs-linux-test` (or ze-qemu-all-test) | traffic suite enrolled; 001/002 assert real `tc qdisc show` output under root; native runs skip via `option=needs-linux` |
| AC-3 | Privileged 1M-prefix run executed once on a capable host | pprof comparison table (fringe heap vs <12MB target, GC CPU vs <22%) pasted into this spec + learned summary; run repeatable via documented `make ze-stress-profile` |
| AC-4 | Two-peer MP_REACH `.ci` | Peer B observes the IPv6 UPDATE with rewritten next-hop on the wire (not just RIB state) |
| AC-5 | LLGR multi-peer `.ci`s | Non-LLGR EBGP peer sees withdraw; non-LLGR IBGP peer sees NO_EXPORT + LOCAL_PREF=0; LLGR peer sees retained route; RR-client variant proves reflection path; `llgr-readvertise.ci` stale comment corrected |
| AC-6 | Chaos iface action family | At least link-flap + addr-remove actions, netns-scoped, scenario-file drivable; BGP-session recovery asserted; existing BGP actions untouched; `mk/test-chaos.mk:13` stale path fixed |
| AC-7 | Web concurrent-edit stress | N≥50 concurrent editor mutation sessions: no race detector hits, no lost/torn commit, error rate 0 |
| AC-8 | Fleet perf | N≥128 concurrent managed clients complete initial sync; latency/error budget recorded; test lives in evidence/release tier, not ze-verify |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator relies on config commit rejecting port clashes | random configs → CollectListeners → FindListenerConflict | `TestListenerConflictProperties` |
| 2 | Operator migrates an ExaBGP estate | arbitrary exabgp confs → `ze exabgp migrate` → valid ze config | `TestMigrationRoundTripProperty` |
| 3 | Operator applies traffic-control on an appliance | config → traffic backend → kernel qdisc visible | QEMU `001-boot-apply.ci` |
| 4 | ISP runs ze as transit with LLGR mixed peers | GR stale → per-peer egress divergence on wire | `llgr-egress-suppress-multi-peer.ci` |
| 5 | Operator fleet-manages 100+ routers from one hub | clients → managed listener → config sync | `TestFleetManyClientsPerf` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestListenerConflictProperties` (+ `TestListenerConflictNotTransitive`) | `internal/component/config/listener_property_test.go` | AC-1/L92 | PASS |
| `TestMigrationRoundTripProperty` | `internal/exabgp/migration/roundtrip_property_test.go` | AC-1/L93 | PASS |
| `TestForwardPoolOrderingProperty` | `internal/component/bgp/reactor/forward_pool_property_test.go` | AC-1/L94 | PASS (incl. -race) |
| `TestFilterChainRandomUpdatesProperty` | `internal/component/bgp/reactor/filter_ordered_property_test.go` | AC-1/L96 | PASS |
| `TestChaosIfaceLinkFlap`, `TestChaosIfaceAddrRemove` | `internal/chaos/engine/iface_integration_linux_test.go` | AC-6 | |
| `TestWebConcurrentEditStress` | `internal/component/web/stress_test.go` (or test-tagged) | AC-7 | |
| `TestFleetManyClientsPerf` | `internal/component/managed/perf_test.go` (evidence tag) | AC-8 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| listener port (property gen) | 1-65535 | 65535 | 0 excluded by type/config | N/A uint16 |
| property iteration count (CI) | fixed seed, bounded MaxCount | - | - | - |
| fleet client count | 128 target | - | - | resource-bound, document host requirements |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `022-boot-qdisc-tc.ci` + `023-reload-qdisc-tc.ci` (needs-linux); 001/002 scope-notes corrected | test/traffic | kernel state proof under QEMU root | AUTHORED; QEMU exec env-blocked (no qemu-system/CAP_NET_ADMIN) |
| `forward-mpreach-nexthop-self-two-peer.ci` | test/plugin | IPv6 MP_REACH forwarding proof | PASS |
| `llgr-egress-suppress-multi-peer.ci`, `llgr-egress-rr-multi-peer.ci` | test/plugin | LLGR per-peer egress divergence | NOT DONE (AC-5, see Known Limitations); WIP in tmp/scratch |
| `test/chaos/iface-link-flap.ci` scenario file | test/chaos | link-flap recovery | AUTHORED; `.ci` runner env-blocked (chaos build canceled in sandbox); direct ze-chaos exit 0 |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - no new wire behavior; LLGR/MP_REACH tests exercise existing implemented protocol paths against ze-test peers | - | - | - | |

## Files to Modify

- `scripts/evidence/qemu-all-tests.sh` - enroll traffic suite
- `test/traffic/001-boot-apply.ci`, `test/traffic/002-reload-apply.ci` - qdisc assertions (needs-linux gated)
- `test/plugin/llgr-readvertise.ci` - correct the stale "no multi-peer infra" comment
- `internal/chaos/engine/action.go` + scenario parser - additive iface action family
- `mk/test-chaos.mk` - fix stale `CHAOS_PACKAGES` path (:13)
- `mk/` - evidence-tier targets for stress/perf tests if missing
- `docs/functional-tests.md` - runner/QEMU enrollment + property-test documentation (Documentation Update Checklist row 10)

## Files to Create

- `internal/component/config/listener_property_test.go`
- `internal/exabgp/migration/roundtrip_property_test.go`
- `internal/component/bgp/reactor/forward_pool_property_test.go`
- `internal/component/bgp/reactor/filter_ordered_property_test.go`
- `internal/chaos/engine/iface_linux.go` (+ `iface_integration_linux_test.go`)
- `internal/component/web/stress_test.go`
- `internal/component/managed/perf_test.go`
- `test/plugin/forward-mpreach-nexthop-self-two-peer.ci`
- `test/plugin/llgr-egress-suppress-multi-peer.ci`, `test/plugin/llgr-egress-rr-multi-peer.ci`

## Implementation Steps

1. **Phase: Wiring (property pilot)** - L92 listener property on stdlib quick; failing first (deliberately wrong invariant) then green; A-2 verdict recorded (AC-1 pilot).
2. **Phase: remaining properties (TDD)** - L93/L94/L96 (AC-1).
3. **Phase: QEMU traffic enrollment** - suite list + qdisc assertions; run `ze-qemu-needs-linux-test` (AC-2).
4. **Phase: two-peer .ci authoring** - MP_REACH + LLGR variants (AC-4, AC-5).
5. **Phase: chaos iface actions (TDD, integration-tagged)** - netns-scoped link-flap/addr-remove + scenario (AC-6); fix mk/test-chaos.mk.
6. **Phase: stress/perf** - web concurrent-edit + fleet clients, evidence tier (AC-7, AC-8).
7. **Phase: L197 evidence run** - execute `make ze-stress-profile` privileged, paste pprof table (AC-3).
8. **Full verification** - `make ze-verify` (+ QEMU targets on capable host).
9. **Complete spec** - audit tables, `plan/learned/NNN-followup-test-infra.md`, two-commit closure.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (2026-07-10, closure session; surface = cad47fcc0 + a35f6c355 + e6c52a94f + working tree)

Pre-checks: `make ze-validate` PASS; `scripts/dev/audit-test-relaxation.py main` clean.
Wiring verified bottom-up: new ActionTypes reachable via `--chaos-actions` name lookup
(`engine/action.go` actionTypes map + direct ze-chaos run exit 0); iface executors called from
`executeChaos` (`peer/simulator_actions.go`); evidence-tier make targets' tags (`stress`,
`fleetperf`) match the test files; traffic suite enrolled (`qemu-all-tests.sh` fsuite line).

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | Comment claims the multi-peer egress divergence "is proven on the wire by llgr-egress-suppress-multi-peer.ci and llgr-egress-rr-multi-peer.ci" -- files never delivered (AC-5 deferred); false claim in a shipped test | test/plugin/llgr-readvertise.ci:13-18 | fixed: comment states the unwired rail + points at spec-rib-arch-7 |
| 2 | ISSUE | `--chaos-actions` flag help + usage text hardcode the v2 action list, omitting iface-link-flap / iface-addr-remove (undiscoverable from `ze-chaos chaos --help`) | internal/chaos/orchestrator/cli.go (flag desc + usage) | fixed at source: list derived from new `engine.V2ActionNames()`; regression test `TestV2ActionNamesComplete` |
| 3 | ISSUE | `chaos-web-dashboard.md` action tables (New Actions, scheduler weights) missing the iface fault family while the new sources carry `// Design:` anchors to that doc | docs/architecture/chaos-web-dashboard.md | fixed: rows + source anchors added; UI-exclusion note added (see #4) |
| 4 | NOTE | Dashboard manual-trigger UI deliberately excludes the iface actions (`chaosActionTypes()`, web/control.go:440-458): netns-harness-only per R-5 | internal/chaos/web/control.go:440 | acknowledged deliberate scope; now documented in the doc's trigger-forms section |
| 5 | NOTE | `flapLink`/`cycleAddr` sleep synchronously in executeChaos; `interval` param uncapped (cycles capped 20) | internal/chaos/peer/simulator_actions_iface_linux.go | acknowledged: operator-controlled chaos-tool input, cold path |
| 6 | NOTE | `NeedsReconnect()` exported with no production caller (same-package tests only); spec diff only added a map entry | internal/chaos/engine/action.go | fixed: unexported to `needsReconnect`, table + test kept |

### Fixes applied
- `test/plugin/llgr-readvertise.ci` -- comment rewritten (no multi-peer wire test exists;
  names the rail gap + owning spec). Re-run: PASS 1/1.
- `internal/chaos/engine/action.go` -- `V2ActionNames()` added (sorted, table-derived);
  `needsReconnect` unexported.
- `internal/chaos/engine/action_test.go` -- `TestV2ActionNamesComplete` (exact v2 set,
  sorted, iface pair present) so a future v2 action can never be invisible in help again.
- `internal/chaos/orchestrator/cli.go` -- flag description + usage text built from
  `engine.V2ActionNames()` (textbuf), eliminating the drift class.
- `docs/architecture/chaos-web-dashboard.md` -- iface rows in New Actions + weight tables,
  source anchors, explicit trigger-UI exclusion note (R-5 rationale).

### Run 2 (2026-07-10, after fixes)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE (pre-existing checker false positive) | `ze-validate` flags `CLIRun`: "no cross-package non-test caller". Factually wired: production entry is the registry closure `orchestrator/register.go:9` (root command "chaos", blank-imported by `cmd/ze/ze_chaos_run.go:7`); ze_chaos-tagged `cmd/ze/ze_chaos_main_test.go` drives it cross-package; same convention as `env.Run`. Export + callers untouched by this spec's diff; fires only while cli.go sits in the changed-file scan set | internal/chaos/orchestrator/cli.go:60 | acknowledged with evidence; unexport attempted and reverted (breaks tagged cmd/ze CLI tests, diverges from registry convention) |
| 2 | NOTE (pre-existing) | Every `-race` make target (incl. ze-unit-test and the new ze-stress-web-test) fails under go1.26 linux/amd64 with `Makefile:16 export CGO_ENABLED := 0` (10708d7dc, 2026-05-19): "go: -race requires cgo". Workaround: `make <target> CGO_ENABLED=1` (command-line beats `:=`). Global build-system condition, not introduced by this spec | Makefile:16 | acknowledged pre-existing; affects all -race targets equally; fresh AC-7 evidence run with the command-line override |

Evidence: `make ze-lint-changed` 0 issues; `go vet -tags ze_chaos ./cmd/ze/` clean;
`go test ./internal/chaos/...` 14/14 ok; `ze-chaos chaos --help` lists all 8 v2 actions;
`llgr-readvertise` PASS 1/1; `make ze-doc-test` PASSED; relaxation audit clean.

### Final status
- `/ze-review` Run 2: **0 BLOCKER, 0 ISSUE** (two acknowledged pre-existing NOTEs recorded
  above with file:line evidence).
- All NOTEs recorded above.

## Implementation Audit (2026-07-10 closure)

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | DONE | 4 property tests (files in TDD table); fresh re-run 2026-07-10 all green (L94 with `-race` via CGO_ENABLED=1) | stdlib quick, fixed seeds (A-2 confirmed) |
| AC-2 | DONE (code) / QEMU exec ENV-BLOCKED | 022/023 authored + parse; traffic suite in `qemu-all-tests.sh` (`fsuite traffic ... -p 1`); 001/002 scope notes corrected | Runbook: `make ze-qemu-needs-linux-test` (no qemu-system here); user-approved env-blocked closure 2026-07-10 |
| AC-3 | ENV-BLOCKED | Runbook recorded (Known Limitations): `make ze-stress-profile` (root+netns) | user-approved env-blocked closure 2026-07-10 |
| AC-4 | DONE | `forward-mpreach-nexthop-self-two-peer.ci` fresh re-run 2026-07-10: exit 0 | per-peer MP_REACH next-hop-self on the wire |
| AC-5 | DEFERRED (user-approved 2026-07-10) | Root cause with file:line chain in Known Limitations + `spec-rib-arch-7-llgr-multipeer-ci.md` (destination, updated) + `plan/deferrals.md` row 2026-07-10 | LLGR egress filter unwired on the readvertise/AnnounceNLRIBatch rail = large feature, not a fixture; explicit user decision relayed 2026-07-10 approves closing with AC-5 deferred |
| AC-6 | DONE (code+unit) / netns+scenario legs ENV-BLOCKED | `go test ./internal/chaos/...` 14/14 ok (fresh); direct `ze-chaos chaos --config-only --chaos-actions iface-link-flap` exit 0 (fresh); mk/test-chaos.mk:13 fixed | integration test SKIPs natively (CAP_NET_ADMIN gate); runbooks recorded |
| AC-7 | DONE | `TestWebConcurrentEditStress` fresh re-run 2026-07-10 via `make ze-stress-web-test CGO_ENABLED=1` (see Pre-Commit Verification) | evidence tier; CGO note in Review Gate Run 2 #2 |
| AC-8 | DONE | Fresh re-run 2026-07-10: 128 clients all synced in 86.3ms wall-clock; latency p50=64.0ms p95=76.3ms max=83.4ms; PASS (5.58s) | evidence tier (`make ze-stress-fleet-test`) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| 4 property test files | exist (commit cad47fcc0) | listener/migration/forward-pool/filter-ordered |
| `internal/chaos/engine/iface_linux.go` (planned name) | DEVIATION: shipped as `internal/chaos/peer/simulator_actions_iface_linux.go` + `_other.go` + engine params | executor lives with the peer simulator that owns conns; engine holds enum/params |
| `internal/component/web/stress_test.go` | exists | tag `stress` |
| `internal/component/managed/perf_test.go` (planned) | DEVIATION (A-6, recorded): `cmd/ze/hub/fleet_perf_test.go` | real hub+TLS harness lives there |
| `test/plugin/forward-mpreach-nexthop-self-two-peer.ci` | exists, green | |
| `test/plugin/llgr-egress-suppress-multi-peer.ci` + rr variant | NOT delivered -- AC-5 deferred (user-approved) | WIP preserved `tmp/scratch/` |
| `test/traffic/022/023` + 001/002 edits, `qemu-all-tests.sh`, `mk/test-chaos.mk`, `docs/functional-tests.md` | exist (commits cad47fcc0/a35f6c355) | |

### Audit Summary
- Total ACs: 8. Done: 5 (AC-1,4,6,7,8). Done-code/env-blocked-exec: 2 (AC-2,3). Deferred with
  user approval + destination: 1 (AC-5).
- Deviations: executor placement (chaos peer pkg), fleet-perf location (A-6) -- both recorded.

## Goal Validation

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Property-test framework unblocks L92-L96 class | unit tests (fresh green) | 4 property tests incl. transitivity counterexample doc (`TestListenerConflictNotTransitive`) |
| Privileged CI runner path for kernel-state assertions | QEMU enrollment + authored tests | `fsuite traffic` in qemu-all-tests.sh; 022/023 `option=needs-linux` gated; runbook `make ze-qemu-needs-linux-test` (exec env-blocked, user-approved) |
| Two-peer wire-forwarding proof | functional test (fresh green) | `forward-mpreach-nexthop-self-two-peer.ci` exit 0; LLGR variant deferred (AC-5, user-approved, destination spec updated with root cause) |
| Stress/chaos gaps closed | new harness + evidence-tier runs | iface action family (unit green, direct run exit 0); `TestWebConcurrentEditStress`; `TestFleetManyClientsPerf` 128 clients / 86ms |

## Pre-Commit Verification (2026-07-10, fresh evidence)

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| listener/migration/forward-pool/filter-ordered property tests | yes | committed in cad47fcc0 (git show --stat) |
| `test/plugin/forward-mpreach-nexthop-self-two-peer.ci` | yes | runner found + ran it (exit 0, 2026-07-10) |
| `test/traffic/022-boot-qdisc-tc.ci`, `023-reload-qdisc-tc.ci` | yes | grep shows `option=needs-linux` + `tc qdisc show dev eth0` lines |
| `test/chaos/iface-link-flap.ci` | yes | committed in cad47fcc0 |
| `internal/chaos/peer/simulator_actions_iface_{linux,other}.go` | yes | committed in cad47fcc0; `go test ./internal/chaos/...` compiles both |

### AC Verified (fresh, 2026-07-10)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | 4 property tests green | `go test` per package: L92 exit 0, L93 exit 0, L94 (-race, CGO_ENABLED=1) exit 0, L96 exit 0 |
| AC-2 | code complete, exec env-blocked | 022/023 exist + gated; `fsuite traffic` enrolled; no qemu-system binary on host |
| AC-3 | env-blocked, runbook recorded | Known Limitations paragraph; no sudo/CAP_NET_ADMIN here |
| AC-4 | MP_REACH two-peer on the wire | `bin/ze-test bgp plugin forward-mpreach-nexthop-self-two-peer` exit 0 |
| AC-5 | deferred with destination | `plan/deferrals.md` 2026-07-10 row; `spec-rib-arch-7-llgr-multipeer-ci.md` Root Cause Finding section; user approval relayed 2026-07-10 |
| AC-6 | additive iface family works | chaos pkg tests 14/14 ok; `ze-chaos chaos --config-only --chaos-actions iface-link-flap` exit 0; help lists both actions |
| AC-7 | web concurrent-edit stress green | `make ze-stress-web-test CGO_ENABLED=1` PASS 71.5s: concurrent_mutations_no_race 32.4s + concurrent_commit_storm 39.2s (64 users, 64 conflicts observed), -race clean |
| AC-8 | fleet 128-client perf green | fresh run: all synced 86.3ms; p50=64.0ms p95=76.3ms max=83.4ms; PASS |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | recorded in table (design-time re-verification + migration peer-name refinement) |
| A-2 | confirmed | 4 property tests green on stdlib quick; no `rapid` dependency added |
| A-3 | confirmed (code) | enrollment one-line; 022/023 parse; QEMU exec env-blocked with runbook |
| A-4 | ENV-BLOCKED (recorded) | no sudo/netns/CAP_NET_ADMIN; runbook in Known Limitations; user-approved |
| A-5 | confirmed | additive enum/params/guard/scheduler; unit tests green; direct run exit 0 |
| A-6 | confirmed (with recorded deviation) | fleet test in `cmd/ze/hub` (real hub harness); fresh PASS |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/functional-tests.md` new tiers (property, stress, needs-linux, iface chaos) | source anchors present (:230-233, :246, :260); `make ze-doc-test` PASSED 2026-07-10 | yes |
| `docs/architecture/chaos-web-dashboard.md` action tables | updated this session (Review Gate #3) + anchors to `simulator_actions_iface_linux.go`, `action_params.go`, `web/control.go`; `make ze-doc-test` PASSED after edit | yes |
| ai/INDEX.md discovery chain | row :344 "test, functional, .ci" -> `docs/functional-tests.md` -> new tiers | yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] Every chosen work item has feature code + test
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| stdlib `testing/quick` as default property engine | `pgregory.net/rapid` (better shrinking) | go-standards.md:28 requires user approval for new deps; quick needs none; A-2 pilot decides if rapid is worth the ask |
| L92 property set excludes transitivity | assert transitivity per the original row | Provably false (wildcard counterexample verified firsthand at listener.go:302+); asserting it would force wrong production changes |
| L200 via QEMU suite enrollment | new privileged .ci runner mode | Root .ci execution already exists (needs-linux/QEMU); enrollment is the minimum change |
| L197 as an evidence run, not new code | rebuild the harness | `make ze-stress-profile` + scenario 05-profile-1m are complete; only execution is outstanding |
| Chaos iface actions netns-scoped | operate on host interfaces | Host-iface manipulation from a test harness is destructive; netns keeps it hermetic (R-5) |
| Stress/perf tests in evidence/release tier | add to ze-verify | Resource-heavy and host-dependent (R-6); matches existing ze-traffic-test placement |

## Known Limitations

- Property tests document distribution assumptions (generated input shapes), not exhaustive proofs.
- The L197 numeric targets are goals from `plan/learned/618-rib-bestpath-pack.md`; a miss produces a follow-up perf spec, not a blocked closure of this one (record deferral with destination if missed).
- Fleet perf asserts scale on one host profile; cross-host scaling is not covered by this test.

### Environment-blocked (evidence recorded)

- **AC-3 (L197 privileged 1M-prefix pprof run):** not executed -- this sandbox has no sudo/root, no netns, no CAP_NET_ADMIN (`plan/known-failures.md`). Runbook: `make ze-stress-profile` (needs root + netns; sets `ZE_PPROF=1`, runs `test/stress/run.py 05-profile-1m`). Deliverable is the pprof comparison table (fringe heap vs <12MB target, GC CPU vs <22%); paste it here after the run.
- **AC-2 (QEMU tc-qdisc kernel-state run):** `qemu-system-*` is not installed here, and `option=needs-linux` gates on GOOS only, so natively the qdisc tests run and fail (no `eth0`, no CAP_NET_ADMIN). Code is complete and parses (`ze-test traffic --list` shows 022/023). Runbook: `make ze-qemu-needs-linux-test`.
- **AC-6 (chaos iface integration + scenario `.ci`):** the netns integration test compiles and SKIPs natively via the CAP_NET_ADMIN `t.Skipf` pattern (netlink precedent). The `test/chaos/iface-link-flap.ci` scenario is env-blocked by the chaos `.ci` runner in this sandbox: all four chaos `.ci` (incl. the three pre-existing) fail identically at ~30ms with "context canceled" (the on-demand `ze-chaos` build is canceled). Direct `bin/ze-chaos --chaos-actions iface-link-flap` exits 0. Runbooks: `make ze-integration-*` (netns test, needs CAP_NET_ADMIN); `make ze-chaos-integration-test` (scenario, needs a working chaos build env).

### Incomplete -- root cause found: LARGE FEATURE, deferred (NOT an environment block)

- **AC-5 (multi-peer LLGR egress divergence `.ci`):** NOT delivered. **Root cause investigated
  and confirmed 2026-07-10 (this closure session), with a producing `file:line` chain.**

  **Finding: the LLGR egress divergence never fires end-to-end in production. The two halves of
  the feature sit on different route-propagation rails and were never connected.**

  - *Consumer* -- `LLGREgressFilter` (`internal/component/bgp/plugins/gr/gr_egress.go:57`) reads
    `meta["stale"]` and stamps NO_EXPORT+LOCAL_PREF=0 (IBGP :89-91) / withdraw (EBGP :99). It is
    **only** invoked on the route-server / cache **ForwardUpdate** path: `safeEgressFilter` is
    called at `reactor/forward_rs.go:324` and `reactor/reactor_api_forward.go:490` and **nowhere
    else** (grep confirmed; `reactor_api_batch.go` has zero egress refs).
  - *Producer* -- the only code that sets `meta["stale"]` is the RIB readvertise/refresh path:
    `rib/rib_replay.go:299` (`resendRoutesWithCursor`, driven by `clear bgp rib out`) and
    `rib/rib_commands.go:616` (`sendRoutes`, RFC 7313). Both hand the meta to
    `rib.go:693 updateRouteWithMeta` -> `sdk_engine.go:44 UpdateRouteWithMeta` ->
    `MethodUpdateRoute` (`plugin/server/dispatch_registry.go:236`, sets `CommandContext.Meta`) ->
    `peer <sel> update cursor/text` -> `cmd/update/update_text.go:706 handleUpdate` ->
    `update_text.go:767 DispatchNLRIGroups` -> `reactor_api_batch.go:28 AnnounceNLRIBatch`.
  - **The gap:** `DispatchNLRIGroups` never forwards `ctx.Meta` into the batch, and
    `AnnounceNLRIBatch` builds each peer's UPDATE from scratch and **never calls any egress
    filter**. So `meta["stale"]`, carefully plumbed by the RIB, is dropped before it can reach
    the filter. The LLGR readvertise trigger `onLLGREntryDone` (`gr/gr.go:142`) issues exactly
    this `clear bgp rib out` -> RIB -> `AnnounceNLRIBatch` path, so stale routes re-advertised
    to non-LLGR peers arrive unmodified. This is a **known, documented** gap:
    `pkg/plugin/sdk/sdk_engine.go:42` -- "Plugin-originated routes currently go through
    AnnounceNLRIBatch (direct send) where CommandContext.Meta is not yet consumed by egress
    filters." The ForwardUpdate rail that *does* run the filter has **no** producer of
    `meta["stale"]` either (the `rs`/cache plugins set no stale meta -- grep confirmed), so the
    filter's stale branches are dead in production regardless of path.

  Why the existing coverage stayed green: the 8 `gr_egress_test.go` unit tests call
  `LLGREgressFilter` directly (bypassing the rail gap); `llgr-readvertise.ci` reconnects the
  **same** LLGR-capable peer and only needs the route delivered (no stamping), so it passes via
  plain ribOut replay.

  **Determination: this is a LARGE FEATURE, not a fix.** Closing the gap means wiring the egress
  filter pipeline (ModAccumulator application, incl. withdraw conversion) into the from-scratch
  `AnnounceNLRIBatch` direct-send path -- a hot path (buffer-first / memory-architecture rules)
  used by **all** plugin route injection (static, redistribute, RIB replay), so broad blast
  radius and non-trivial risk. It is not a bounded fix jammable into a closure session.

  **Deferred to `plan/spec-rib-arch-7-llgr-multipeer-ci.md`** (the pre-existing skeleton for the
  multi-peer LLGR `.ci`), updated with this root cause. Recorded in `plan/deferrals.md`
  (2026-07-10). The `llgr-egress-rr-multi-peer.ci` RR variant is folded into the same destination
  (it exercises the same unwired path). The WIP `.ci` is preserved at
  `tmp/scratch/llgr-egress-suppress-multi-peer.ci.wip`; note it also lacked any forwarding
  mechanism (no `rs`/`redistribute`), so it timed out with the target peers receiving nothing --
  the destination spec must first choose a forwarding path, then close the rail gap.

  The `llgr-readvertise.ci` stale multi-peer comment was already corrected (prior session)
  and re-corrected at closure (it briefly claimed the deferred `.ci`s existed; Review Gate #1).

  **Closure decision (2026-07-10):** the user explicitly approved closing this spec with AC-5
  deferred (destination `spec-rib-arch-7-llgr-multipeer-ci.md`, deferral row recorded) and the
  AC-2/AC-3 execution legs env-blocked with their runbooks. This is the explicit approval that
  `ai/rules/planning.md` requires for partial/deferred items.

## Implementation Status (2026-07-09)

| AC | Status | Evidence |
|----|--------|----------|
| AC-1 | DONE (green) | 4 property tests pass, incl. `-race` (Commit A cad47fcc0). A-2 confirmed: stdlib `testing/quick` sufficient, no `rapid`. |
| AC-2 | DONE (code) / QEMU exec ENV-BLOCKED | traffic suite enrolled; 022/023 needs-linux tc-qdisc tests authored + parse; 001/002 scope-notes corrected. QEMU run not runnable here (no qemu-system, no CAP_NET_ADMIN). |
| AC-3 | ENV-BLOCKED | privileged 1M-prefix pprof run needs root+netns; runbook `make ze-stress-profile`. |
| AC-4 | DONE (green) | `forward-mpreach-nexthop-self-two-peer.ci` passes: per-peer MP_REACH next-hop-self on the wire. |
| AC-5 | DEFERRED (root cause found) | multi-peer LLGR egress divergence `.ci` not green. Root cause 2026-07-10: LLGR egress filter (`gr_egress.go:57`) only runs on the ForwardUpdate rail (`forward_rs.go:324`, `reactor_api_forward.go:490`); the only `meta["stale"]` producer is the RIB readvertise path (`rib_replay.go:299`) which flows through `AnnounceNLRIBatch` (`reactor_api_batch.go:28`) -- a rail that drops `ctx.Meta` and never calls the filter (documented at `sdk_engine.go:42`). LARGE FEATURE (hot-path wiring, broad blast radius) -> deferred to `spec-rib-arch-7-llgr-multipeer-ci.md`. Logic covered by 8 unit tests; comment fix done. |
| AC-6 | DONE (code + unit) / netns+`.ci` legs ENV-BLOCKED | iface action family additive (enum/params/executor linux+stub/guard/scheduler); unit tests green; direct ze-chaos exit 0; `mk/test-chaos.mk:13` fixed. Integration test skips natively; scenario `.ci` env-blocked (chaos runner). |
| AC-7 | DONE (green) | `TestWebConcurrentEditStress` passes under `-race` (evidence tier). |
| AC-8 | DONE (green) | `TestFleetManyClientsPerf`: 128 clients synced ~80ms, zero errors (evidence tier). File-location deviation recorded (A-6). |

## Notes
- Designed 2026-07-09 from skeleton; user instruction 2026-07-09 authorized batch conversion to ready.
- Original L80/L200 sub-items dropped by the skeleton (route-reflection multi-peer; 002-reload-apply.ci) were restored during design (append-only correction).
