# Spec: perf-next-0-umbrella -- Third Hot-Path Optimization Round

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/5 |
| Updated | 2026-10-08 |

## Remaining measurement and historical block

**The block is LIFTED as of 2026-09-05 and every child is closed.** Thomas
answered the Phase B question on 2026-09-05: implement it and meet AC-3. It
landed in `7d4fedbc6` at 6 allocs/op against a gate of 12, and
`spec-perf-next-2-filter-delta-alloc` closed the same day.
`spec-perf-next-1-ebgp-wire-lockfree` closed that day too, recording a removal:
the cache it optimized was deleted by `df44d8d27` on 2026-08-17.
`spec-perf-next-3-rib-show-alloc` closed on 2026-08-12.

What this umbrella still owes is its own Phase 5: re-measure end to end and
write the round up. The scheduling metadata now records that remainder; child
closure does not complete the parent's acceptance criteria.

The superseded text said: blocked on the same decision by Thomas that blocks
child 2, with children 1 and 3 complete (`ebgpWireSlot` in
`internal/component/bgp/reactor/received_update.go`, `Community.AppendText` in
`internal/core/bgp/attribute/text_append.go`). `ebgpWireSlot` no longer exists.

Two of this umbrella's own criteria need the same answer. AC-1 asks for a fresh
`le perf run PPROF=1` profile and AC-3 for a recorded re-run, but
`le perf run` exercises none of the three paths this round touched
(`docs/architecture/perf-round-3.md`), and `Dockerfile.ze` was recorded stale
when the round closed.
R-1 accepts per-child Go benchmarks as evidence for each optimisation. It does
not waive the umbrella's AC-1 or AC-3. Those obligations remain until the owner
explicitly accepts substitute evidence or the harness is repaired and run.

-> Decision (owner, 2026-10-08): substitute evidence is NOT accepted. Repair the stress harness so it exercises the three paths this round touched, then run AC-1 and AC-3 for real. The measurement runs only on a quiet machine over committed BGP code (another session was editing the reactor on 2026-10-08); a profile taken under foreign load or over uncommitted code is not evidence.
-> Constraint (owner, 2026-10-08): BGP perf measurements are made on the owner's Mac, not on the Linux development machine. The AC-1/AC-3 numbers therefore come from a run the owner makes on the Mac; agents prepare the harness and prove it reaches the three paths. The Linux netns stress harness (`./le test integration stress`, root) cannot run on macOS, so how AC-1/AC-3 are measured on the Mac is open (see the next decision).
-> Decision (owner, 2026-10-08): "we can only test perf on my mac as the hardware is known (vs here a VM which perf varies) even if we have to use qemu". AC-1/AC-3 run on the owner's Mac. The netns stress harness runs inside a Linux QEMU guest on the Mac through the repository's existing VM runner; one command boots the guest, runs the scenario, and copies back the report and the CPU profile. Agents prepare and prove the guest route on the Linux box (a smoke run proves completion and reach only; it is never a measurement); the owner runs the measurement.

### Harness repair (2026-10-08, done) and the measurement still owed

Diagnosis, read at the producer (`internal/le/test/integration/stress.go`):

| # | Defect | Effect |
|---|--------|--------|
| 1 | `preflight` started `bin/ze` whenever it existed and built only when it was missing | On 2026-10-08 `bin/ze` was `vcs.revision=8615151ee` of 2026-09-24: a profile would have measured a two-week-old daemon, and the report did not name the binary |
| 2 | Its fallback build used `-tags ze_core,ze_distro` alone | Every feature gate compiled out, BGP included (`internal/component/plugin/all/all_ze_bgp.go` is `//go:build ze_bgp`): the fallback DUT could not load the scenario |
| 3 | `05-profile-1m` was one injector into a DUT with `bgp-rib` and no second peer, no policy, no query | No UPDATE forwarded (former `ebgpWireSlot` path, now the eBGP prepend in the rebuild), no filter delta, no route rendered (`Community.AppendText`). Its `bgp-rib` was not even attached to the peer |
| 4 | `test/perf/configs-filter/ze.conf` (the `le perf` filter overlay) used `//` comments | `ze config validate` refused line 1, so the overlay that was meant to reach the filter delta through `le perf` never started |

`Dockerfile.ze` is not on this path: the stress harness runs in network
namespaces on the host, and the interop images' staleness is
`spec-interop-image-copies-a-prebuilt-ze`.

Repair: every run builds the DUT from the checkout into `tmp/stress/ze` with
`repofeaturetags.DaemonBuildTags` (report field `binary`); the profile scenario
carries `stressProfileReach` (eBGP sink 172.31.0.4 AS 65200 via `le test peer
--mode sink`, a best-table looking-glass fetch once the injector reports its
last byte, report field `queries`, empty table fails the run); its `ze.conf`
adds `STRESS-IMPORT`/`STRESS-EXPORT` modify policies, `bgp-rs`, attached
`bgp-rib`, and a plaintext looking glass on 127.0.0.1:8443;
`STRESS_PREFIXES` shortens a smoke run. Tests:
`TestStressHarnessBuildsZeFromCheckoutEveryRun`,
`TestStressProfileScenarioReachesRoundThreePaths`,
`TestStressProfileQueryRefusesAnEmptyTable`,
`TestStressProfileConfigMatchesTheHarness`,
`TestStressPrefixesShortensEveryRound`, `TestStressPrefixesRefusesANegativeCount`
(red against the pre-repair runner and config, green after).
`TestStressBuildFailureCarriesTheBuildOutput` and
`TestStressCleanupReportsOnlyANamespaceLeftBehind` cover the two defects the
first smoke attempt exposed: a failed DUT build reported `build Ze: ` with no
cause (now: exit status plus the last 20 lines of stderr, or of stdout), and
cleanup reported `delete namespace ...: exit 1` for namespaces the failed run
never created (now: an error only when `/run/netns/<name>` remains, naming it).
Both red before, green after.

**Guest smoke, 2026-10-08: REACH EVIDENCE ONLY, never a measurement.** Linux
amd64 development VM under KVM, `./le test qemu stress scenario 05-profile-1m
prefixes 20000 pprof output <scratch>/stress-smoke-5`. Two harness defects the
earlier attempts exposed are fixed first:

| Defect | Fix | Test (red before, green after) |
|--------|-----|--------------------------------|
| Ze (root) refused its config store: it opens the store beside its config, the scenario directory over 9p carries the host uid (1000 here, 501 on the Mac), and Ze refuses a store another user owns | `stageConfig` starts the DUT on a copy in `/tmp/ze-stress-config-<suffix>` (0700, removed before and after the run) | `TestStressDUTRunsOnARunPrivateConfigCopy` |
| The best-table query fired the moment the injector reported its last byte, before the RIB stored a route, and its guard refused only a zero-byte body: the smoke's query answered 122 bytes, an empty `routes` list, and passed | `awaitBestRoutes` probes `?limit=1` once a second until `pagination.total_results` is above zero (bounded by the round timeout), records it as `queries[].routes`, then fetches the whole table | `TestStressProfileQueryWaitsForAPopulatedTable`, `TestStressProfileQueryRefusesATableThatStaysEmpty` |

The smoke ran with the first fix and before the second. It completed: report
`passed: 1`, `binary` `/workspace/tmp/stress/ze`, the CPU, heap and goroutine
profiles and the DUT copied back, one round of 20000 prefixes in 21 UPDATEs.
Reach found in the profiles:

| Path | CPU profile (`-focus` on the four frames) | Heap profile (`alloc_space`) |
|------|------------------------------------------|------------------------------|
| eBGP forward with modify (`buildModifiedPayload`) | no samples | present: `forwardUpdateSelected` -> `buildForwardPayload` -> `buildModifiedPayload`, under `rs.flushWorkerBatch` |
| Filter delta (`textDeltaToModOps`, `parseFilterAttrsInto`) | no samples; `runIngressPolicyChain` and `rs.processForward` present | `runIngressPolicyChain`, `runEgressPolicyChainASN4`, `PolicyFilterChain` -> `applyFilterDelta`, `formatFilterAttrs` present; the two named frames absent |
| `Community.AppendText` | no samples | absent: the table it renders was empty (the second defect) |

The focused CPU profile matched no samples (90 s, 5.60 s sampled): 21 UPDATEs
put too little per-UPDATE work under the 100 Hz sampler. So the smoke proves the
guest route end to end and the forward-with-modify and policy-chain reach; it
does not show the three named frames in the CPU profile. Whether
`textDeltaToModOps`, `parseFilterAttrsInto` and `Community.AppendText` appear is
answered by the AC-1 profile on the Mac (1M prefixes, populated-table query),
whose `-focus` command is below; if any is absent there, that is the
Methodology scope gate, not a pass.

**Scenario fix and smoke 6, 2026-10-08: REACH EVIDENCE ONLY.** The paths run
once per UPDATE, so the injector gained `--inject-per-update N` and
`--inject-vary-attrs` (`InjectSpec.PerUpdate`, `InjectSpec.VaryAttrs`: one MED
and two COMMUNITIES per UPDATE derived from the message index) and the profile
round passes `1` and the vary flag: 1,000,000 UPDATEs plus End-of-RIB, 65
octets each, about 65 MB. Tests: `TestBuildUpdatesV4PerUpdateVaryExact`,
`TestBuildUpdatesV6PerUpdateVaryExact`, `TestBuildUpdatesPerUpdateCount`,
`TestPeerInjectPerUpdateAndVaryReachSpec`, and the profile reach test now
refuses a packed injector (red with the vary flag dropped, green restored).

Guest smoke 6 (`prefixes 100000 pprof`, `<scratch>/stress-smoke-6`) FAILED:
`the looking glass best table stayed empty for the whole round` (600 s), so no
profile came back. A loopback reproduction without root (`<scratch>/repro/`,
the guest-built `ze`, `ze.test.bgp.port`) shows the empty table is not this
change: the packed stream and a config without the import policy answer
`total_results: 0` too, on `routes/table/ipv4%2Funicast` and on
`routes/protocol/<injector>`, while the CPU profile shows
`rib.(*RIBManager).handleReceivedStructured` running. The same loopback run's
30 s CPU profile over 100,000 varied single-prefix UPDATEs (about 9,000
UPDATE/s, 296% CPU):

| Frame | CPU `-focus` |
|-------|--------------|
| `textDeltaToModOps` | 0.88 s of 88.93 s |
| `parseFilterAttrsInto` | 0.73 s |
| `buildModifiedPayload` | 0.18 s |
| `Community..AppendText` | absent: no route is ever rendered while the looking glass answers an empty table |

**Resolved 2026-10-09: the empty table was cdb9cfb751's defect, already fixed.**
The smoke-6 `ze` was built before cdb9cfb751 (`lg: fill the best-routes table`),
whose looking glass sent `show bgp rib best <family>`, which bgp-rib refuses, and
read the refusal as an empty table. The loopback reproduction above did not
isolate that: its injector named `127.0.0.1` as NEXT_HOP, which RFC 7606
treat-as-withdraw removes (`NEXT_HOP is not a unicast host address`, logged at
Debug only), so no route reached Adj-RIB-In on any binary, and its CPU figures
measured withdrawals, not stored routes. Rerun on loopback with next hop
`10.9.9.9` and the full profile config (1,000 varied UPDATEs): the smoke-6
binary answers `routes/table` `total_results: 0` beside `routes/protocol/127.0.0.1`
`1000`; a HEAD `c3adad6f66` build answers `1000` on both. The guest's next hop
`172.31.0.3` is the injector's own address on the shared /24, which the RFC 4271
Section 6.3 check accepts. The guest run is unblocked; it still has to be rerun.
Journal: `silent-fall-through.md` (Debug-only treat-as-withdraw) and
`zero-value-as-valid-answer.md` (`routes/protocol/{name}` by configured name
answers 0 routes).

**Measurement route (AC-1 then AC-3), run by the owner on the Mac.** Both runs
use the same guest size: set `ZE_QEMU_CPUS` and `ZE_QEMU_MEMORY` once (default
8 CPUs, 16384 MiB) and keep them for both, because the report's
`plan.qemu-argv` is what makes the two comparable. Run on a quiet Mac over a
committed BGP tree (`git status --short internal/component/bgp internal/core`
empty). The guest builds the DUT from the shared checkout, so the profile
measures that tree.

```bash
# AC-1: profile run; the output directory must be absent or empty
./le test qemu stress scenario 05-profile-1m pprof output tmp/perf-ac1
go tool pprof -top -nodecount=60 tmp/perf-ac1/ze tmp/perf-ac1/stress-profile-cpu.pb.gz
go tool pprof -sample_index=alloc_space -top -nodecount=60 tmp/perf-ac1/ze tmp/perf-ac1/stress-profile-heap.pb.gz
go tool pprof -top -focus='Community..AppendText|textDeltaToModOps|parseFilterAttrsInto|buildModifiedPayload|ReceivedUpdate' tmp/perf-ac1/ze tmp/perf-ac1/stress-profile-cpu.pb.gz
# AC-3: the same scenario without pprof; rounds[].elapsed-seconds and
# routes-per-second from tmp/perf-ac3/report.json are recorded here
./le test qemu stress scenario 05-profile-1m output tmp/perf-ac3
```

AC-1 pastes the baseline numbers and the frames each child targets; AC-3
records the re-run here and in `docs/performance.md` only if `le perf` numbers
change (that page is generated by `le perf report --doc`).

**Unverified on macOS, check on the first run.** The runner's darwin branch is
written and unit-tested but has never booted on a Mac. The order to check in
and each failure's signature are the table in
`docs/architecture/testing/qemu-integration.md`, "Running the BGP stress
harness in the guest".

| Check | Why it is open |
|-------|----------------|
| Homebrew QEMU carries 9p (`qemu-system-aarch64 -device help` lists `virtio-9p-pci`) | The checkout reaches the guest only over 9p |
| HVF accepts `-machine virt,highmem=on,accel=hvf` (highmem, IPA size) | Never booted under HVF |
| A 16 GiB guest fits (`ZE_QEMU_MEMORY`; 8192 on a 16 GB Mac) | The default guest memory is 16384 MiB |
| The guest `le` cross-builds for linux/arm64 | Only a linux/amd64 guest has been built and booted |

The Linux `sudo env ... ./le test integration stress` route stays a reach proof
on the development VM, never a measurement.

Historical position on 2026-07-22, superseded by the September child closures:
the review recorded all three children as shipped and the round's design record as
`docs/architecture/perf-round-3.md` (child 1 `ebgpWireSlot` lock-free slots
in `received_update.go,89`; child 2 `filterAttrs`/`filterAttrID` in
`filter_chain.go,79`, Phase B scratch-pool deliberately deferred there;
child 3 `Community.AppendText` in
`internal/core/bgp/attribute/text_append.go`). Remaining work is the
two-commit closure of the umbrella and its three children. Note:
the pol-4 `filter-delta-parse-once` follow-up is PRIOR work, not
child 2's completion signal.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. Child specs, all closed: `spec-perf-next-1-ebgp-wire-lockfree` and `spec-perf-next-2-filter-delta-alloc` (2026-09-05), `spec-perf-next-3-rib-show-alloc` (2026-08-12)

## Task

Coordinate the third performance optimization round (after campaigns 771 and 859).
A June 2026 audit (3 parallel code audits + 5 research dossiers, cross-verified
against source) identified three remaining evidence-backed hot-path improvements
and several candidates that turned out NOT to be worth doing. This umbrella:

1. Records the measurement baseline and the profiling-first methodology.
2. Lists the three child specs in execution order.
3. Records the **negative findings** so future sessions do not re-investigate them.

### Baseline (le perf, 2026-06-05, 100K IPv4/unicast routes, 4 GB VM, darwin/arm64 + Colima)

| DUT | Convergence | Throughput | p99 |
|-----|-------------|------------|-----|
| ze | 62ms +/- 10ms | 1,612,903 r/s | 43ms |
| bird | 65ms +/- 0ms | 1,538,461 r/s | 28ms |

History: 91ms (pre-771) -> 71ms (post-771) -> 62ms (post-859). Remaining gap to
BIRD's best recorded run (44ms) was attributed by the first campaign
to architecture (Go GC vs slab allocation, buffered vs in-place parsing,
socket-layer write coalescing), not to remaining low-hanging fruit.

### Historical child targets and current dispositions

| # | Spec | Original target | Disposition |
|---|------|--------|-----------------|
| 1 | `spec-perf-next-1-ebgp-wire-lockfree` | Mutex on every `EBGPWire` cache hit | Closed 2026-09-05; the entire cache was removed on 2026-08-17. Its benchmark figures are historical and cannot be rerun on the current tree |
| 2 | `spec-perf-next-2-filter-delta-alloc` | ~24 allocs per filter-modified UPDATE, re-measured at 20 | ACHIEVED, and better: 6 allocs/op, a 70% cut, held by `AllocCeilings["BenchmarkFilterModifyEgress"]` |
| 3 | `spec-perf-next-3-rib-show-alloc` | Per-route []string + String() in show/JSON enrichment | Closed 2026-08-12; the display-allocation change is recorded in `docs/architecture/perf-round-3.md` |

### Methodology (BLOCKING for every child)

1. **Profile before coding.** Run
   `STRESS_SCENARIO=05-profile-1m ZE_PPROF=1 ./le test integration stress` and
   capture its CPU, heap and goroutine profiles under `tmp/`.
   **Scope gate (not a formality):** the three children were designed from audit
   reasoning + arithmetic (15M lock ops/s; 24 allocs/op x fan-out), NOT from a
   fresh top-frame profile. AC-1's profile is therefore a real gate on scope —
   most acutely for child 2 (largest refactor, smallest win). For each child,
   locate its target frames in the captured profile before implementing it; if a
   child's frames are absent near the top, STOP and present the evidence to the
   user before doing that child. Child 1's RS-fan-out win is not expected to
   appear in this single-DUT 100K-route baseline at all — its parallel
   micro-benchmark is the proof, and that is acceptable per R-1/the child spec.
2. **Benchmark gate per child.** Each child defines a Go benchmark asserting the
   before/after allocs/op or ns/op. The benchmark is written FIRST and its
   "before" numbers are pasted into the child spec.
3. **Re-measure after.** Re-run `./le perf suggest` after each child
   lands; record convergence/throughput movement in the child's Implementation
   Summary. Movement within noise is acceptable for child 3 (its path is not the
   convergence path); the Go benchmark is its proof.

### Negative findings (do NOT re-investigate without new evidence)

| Candidate | Verdict | Evidence |
|-----------|---------|----------|
| Engine event dispatch slice copy (`internal/component/plugin/server/engine_event.go`) | NOT hot. No spec. | BGP events never reach engine subscribers; only config-transaction events do (~10-30 handler registrations per config reload, dispatch rate ~0.1/s operational). Verified via `deliverEvent` flow in `internal/component/plugin/server/dispatch.go`. |
| UPDATE builder pooling (the old spec-604 deferral) | ALREADY DONE. | Commit 233ff1726 (2026-04-16). All 14 make() sites eliminated; `GetUpdateBuilder`/`PutUpdateBuilder` pool exists; BuildUnicast measured at ~10 allocs/op. Reactor forward path never used builders. |
| `forward_build.go` pool-fallback make() (lines 278, 352, 376-378) | Deliberate design, keep. | Tiered escalation per-peer pool -> modBufPool -> make only for oversized payload on pool miss; commented `// pool-fallback` at each site. |
| RFC 7606 validation cache (`docs/research/optimisation-findings.md`) | Stale, unmeasured. | Document dated 2025-12-22 pre-dates both campaigns; explicitly requires measurement that was never done. Act only if a fresh profile shows validation frames at the top. |
| `prefixToWire` allocations (`internal/component/bgp/plugins/rib/rib_nlri.go,117`) | Cold path. No change. | Callers are CLI `inject`/`withdraw` one-shots (`rib_commands.go,383`) and tests; not per-route. |
| seqmap compaction (`internal/core/seqmap/seqmap.go`) | Sound design, infrequent. | O(n log n) only when dead > len/2 and len > 256; mutation-tested; not worth latency-quantile work without evidence. |
| Looking-glass error-path JSON (`internal/component/lg/server.go,550`) | Cold (error responses only). | Not worth touching. |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - the canonical architecture reference: the design principles all new code follows
- [ ] `docs/architecture/plugin/rib-storage-design.md` - the pool-based RIB storage design for API programs, which the engine does not implement
- [ ] `ai/rules/performance.md` - allocation strategy and pool inventory
  → Constraint: copies happen only at sanctioned boundaries (pool entry, ContextID mismatch, filter modify, JSON for external plugins)
- [ ] `docs/architecture/perf-round-3.md` - the third campaign, and the two before it in outline
  → Decision: profile-first; reject proposals that profiling shows are stack-allocated already
  → Decision: value-type struct keys over interned strings; one commit for bisection safety
- [ ] `internal/le/perfbench/actions.go` - le perf run / PPROF / report targets
  → Constraint: results land in `test/perf/results/`, profiles in `tmp/perf-run/pprof`

### RFC Summaries (MUST for protocol work)
- [ ] None at umbrella level (children carry their own; child 1 references `rfc/short/rfc4271.md`)

**Key insights:**
- Both prior campaigns delivered exactly what profiling showed and nothing speculative.
- The "sum of small wins" principle: 20ns x 200 peers x 100K UPDATE/s = 400ms CPU/s.

## Current Behavior (MANDATORY)

**Source files read:** (audit evidence behind the child selection)
- [ ] `internal/component/bgp/reactor/received_update.go` - EBGPWire mutex on every call including cache hits (child 1)
- [ ] `internal/component/bgp/reactor/filter_delta.go` - 14 make() sites + map[string]string parse per modified UPDATE (child 2)
- [ ] `internal/component/bgp/plugins/rib/rib_attr_format.go` - per-route []string + String() loops (child 3)
- [ ] `internal/component/bgp/reactor/forward_build.go` - verified pool fallbacks are deliberate (negative finding)
- [ ] `internal/component/plugin/server/engine_event.go` - verified dispatch is cold (negative finding)
- [ ] `internal/component/bgp/message/update_build.go` - verified builder pooling already done (negative finding)

**Behavior to preserve:**
- All wire formats, JSON output shapes, CLI output, and RFC semantics are unchanged by every child.
- `./le verify current mode full` green; `go test -race ./internal/component/bgp/reactor/...` green for reactor changes.

**Behavior to change:**
- None user-visible. Performance characteristics only.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Inbound BGP UPDATE wire bytes (children 1, 2); CLI/API `show bgp rib` request (child 3).

### Transformation Path
1. TCP read -> WireUpdate (lazy) -> ReceivedUpdate cached in RecentUpdateCache (child 1 touches the EBGP wire variant cache on this object).
2. Policy filter chain text round-trip -> filter delta parse -> wire attribute ops -> buildModifiedPayload (child 2 touches the parse + encode steps).
3. RIB entry -> route enrichment map -> json.Marshal -> pipe operators (child 3 touches the enrichment step).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire <-> reactor cache | WireUpdate referencing pool buffers, BufHandle ownership | [ ] |
| Engine <-> external filter plugin | text UPDATE serialization + RPC (sanctioned copy point) | [ ] |
| RIB <-> CLI/web | map[string]any -> JSON -> pipes | [ ] |

### Integration Points
- Children integrate into existing functions only; no new components, no new registries.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The 2026-06-05 le perf baseline is reproducible on this machine | `test/perf/results/` JSON files | Before/after deltas are noise | re-run `STRESS_SCENARIO=05-profile-1m ZE_PPROF=1 ./le test integration stress` before child 1 | broken until 2026-10-08 (the stress runner reused a stale `bin/ze` and its profile scenario reached none of the round's paths); harness repaired 2026-10-08, the run itself is the remaining phase |
| A-2 | No other session lands conflicting reactor changes mid-round, and the round starts from a clean committed base | git status at spec time | Rebase/benchmark churn; before/after deltas and `ze-unit-reactor-test-race` muddied by unrelated in-flight edits | Check `tmp/session/selected-spec` + git log before each child. NOTE at spec time the working tree had ~48 uncommitted files (cos/iface/l2tp/plugin-registry, none in reactor) — run this round on a branch off a committed base so benchmark deltas and the race gate are attributable to the child only | unvalidated |
| A-3 | The negative findings hold (no new callers appeared) | Dossiers dated 2026-06-11 | A "cold" path may have become hot | Fresh grep for callers during each child's audit step | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Micro-wins don't move le perf numbers (within noise) | Post-child re-measure shows no delta | Go benchmarks are the per-child proof; le perf movement is a bonus for children 1-2 and not expected for child 3 |
| R-2 | Optimization introduces a data race | `go test -race ./internal/component/bgp/reactor/...` failure | Race gate is BLOCKING in children touching reactor |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Umbrella has no feature code of its own; each child carries wiring rows | → | child specs 1-3 | existing test suite per child Wiring Test tables (e.g. TestReceivedUpdate_EBGPWireConcurrent) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Before child 1 starts | Fresh `STRESS_SCENARIO=05-profile-1m ZE_PPROF=1 ./le test integration stress` run captured, on the owner's Mac through `./le test qemu stress scenario 05-profile-1m pprof output tmp/perf-ac1` (owner decision 2026-10-08, "Measurement route"); baseline numbers pasted into this spec; each child's target frames located in the profile (or their absence noted and the child's scope reconsidered with the user per the Methodology scope gate) |
| AC-2 | Each child completes | Child's Go benchmark shows the asserted improvement; child's Review Gate clean |
| AC-3 | All children complete | `STRESS_SCENARIO=05-profile-1m ./le test integration stress` re-run, on the owner's Mac through `./le test qemu stress scenario 05-profile-1m output tmp/perf-ac3` with the AC-1 run's `ZE_QEMU_CPUS`/`ZE_QEMU_MEMORY`; final numbers recorded here and in `docs/performance.md` if changed |
| AC-4 | Umbrella closure | Negative-findings table copied into the learned summary so future sessions inherit it |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Per-child benchmarks and unit tests | see child specs | Child-level proof | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| None at umbrella level (no numeric inputs added) | - | - | - | - |

### Functional Tests
No user-facing behavior change at the umbrella level; existing test suite passes
(`./le verify current mode full`) is the umbrella-level functional gate. Children reference the
specific existing `.ci` suites that prove no regression on their paths.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing suite | `test/` (.ci, unchanged) | No regressions across BGP forward/show paths | |

### Interop Tests (MANDATORY for protocol features)
No wire protocol behavior changes in any child; interop not required (children
preserve RFC 4271 semantics byte-for-byte, asserted by existing unit tests).

## Files to Modify
- `internal/component/bgp/reactor/received_update.go` - via child 1
- `internal/component/bgp/reactor/filter_delta.go` - via child 2
- `internal/component/bgp/plugins/rib/rib_attr_format.go` - via child 3
- `docs/performance.md` - regenerate if final le perf numbers change

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] no | - |
| CLI commands/flags | [ ] no | - |
| Functional test for new RPC/API | [ ] no | - |
| Env var registration | [ ] no | - |
| Doctor check for runtime dependencies | [ ] no | - |
| Prometheus counters/metrics | [ ] no | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] no | - |
| 2 | Config syntax changed? | [ ] no | - |
| 3 | CLI command added/changed? | [ ] no | - |
| 4 | API/RPC added/changed? | [ ] no | - |
| 11 | Affects daemon comparison? | [ ] yes, if final numbers move | `docs/performance.md` (regenerated via `./le perf suggest`) |
| 12 | Internal architecture changed? | [ ] possibly (child 1 cache concurrency note) | `docs/architecture/buffer-architecture.md` per child 1 |

## Files to Create
- `spec-perf-next-1-ebgp-wire-lockfree` - child 1 (created with this umbrella, closed 2026-09-05; the cache it optimized was deleted by `df44d8d27` on 2026-08-17, so the round's record lives in `docs/architecture/perf-round-3.md` Section 1)
- `spec-perf-next-2-filter-delta-alloc` - child 2 (created with this umbrella, closed 2026-09-05 at 6 allocs/op; the round's record is `docs/architecture/perf-round-3.md` Section 4)
- `spec-perf-next-3-rib-show-alloc` - child 3 (created with this umbrella, closed 2026-08-12)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + the child being implemented |
| 2. Audit | Re-validate the child's assumptions (A-N) against current source |
| 3. Wiring phase | Child Wiring Test table |
| 4. Implement (TDD) | Child Implementation Phases |
| 5-14 | Per child spec |

### Implementation Phases
1. **Phase: Baseline (MANDATORY FIRST)** - run `./le perf suggest`; paste numbers + top pprof frames here
   - Tests: n/a (measurement)
   - Files: this spec (baseline section)
   - Verify: profile files exist under `tmp/perf-run/pprof`
2. **Phase: Child 1** - closed 2026-09-05; its optimised cache had already been removed on 2026-08-17
3. **Phase: Child 2** - closed 2026-09-05, including Phase B
4. **Phase: Child 3** - closed 2026-08-12
5. **Phase: Re-measure + close** - resolve the outstanding AC-1/AC-3 evidence decision, record the final measurements and round summary, then review this umbrella for closure. All children are already closed

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every child closed or explicitly deferred with user approval |
| Correctness | Final le perf re-run recorded; no regression vs 62ms baseline |
| Rule: no-speculative-features | Negative-findings table untouched (nothing from it implemented) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Baseline + final le perf numbers in spec | grep this file for the results table |
| Three children closed | `ls plan/spec-perf-next-*.md` shows which files remain open |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | No new inputs at umbrella level |

### Failure Routing
| Failure | Route To |
|---------|----------|
| le perf baseline not reproducible | STOP; report environment delta to user before children |
| Child benchmark shows no win | Mark child blocked, present evidence, ask user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| (research phase) update-builder pooling was open work | Done in commit 233ff1726 | Read the update-pool records during research | Child spec dropped before writing |
| (research phase) engine event dispatch was hot | Config-transaction-only, ~0.1/s | Traced deliverEvent callers | Candidate rejected |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
- An audit agent rating ("CRITICAL") is not evidence; tracing the actual caller
  chain reversed two of five candidates. Caller-chain verification is the gate.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Three small children over one mega-spec | Single combined spec | Matches 859 set pattern; independent files, independent bisection |
| Record negative findings in the umbrella | Drop them silently | Future sessions otherwise re-audit the same cold paths |

## Known Limitations
- This round does not attempt the architectural items (in-place parse, slab/arena
  RIB storage, socket-layer batching) that would close the remaining gap to
  BIRD's 44ms best run. Those need their own spec set with user buy-in on scope.

## Implementation Summary

### What Was Implemented
- [filled at completion]

### Bugs Found/Fixed
- [filled at completion]

### Documentation Updates
- [filled at completion]

### Deviations from Plan
- [filled at completion]

## Implementation Audit

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
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Reduce remaining hot-path overhead with evidence | benchmark + le perf run | [filled at completion] |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [filled during review]

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
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete (per child)
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `./le verify worktree` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### Quality Gates (SHOULD pass — defer with user approval)
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
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-perf-next-umbrella.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm` of spec only
