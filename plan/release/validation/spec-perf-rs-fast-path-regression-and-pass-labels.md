# Spec: perf-rs-fast-path-regression-and-pass-labels

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P2 |
| Release reason | Release validation: the route-server fast path measures about 5x slower than it did on 2026-09-21, and the comparison report shows Ze's sender-first pass under the bare DUT name, so the published convergence figure for Ze is the wrong pass |
| RFC gaps | - |
| Depends | - |
| Phase | 4/6 |
| Handoff | - |
| Updated | 2026-11-01 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas's words: "write the spec with both". This spec owns two defects found
in the same perf lab runs on 2026-10-31 and 2026-11-01.

**Item 1: fast-path convergence regression.** The perf lab DUT
(`test/perf/configs/ze.conf`: two EBGP peers, `bgp-rs` and `adj-rib-in`
attached, `rs-fast-path enable`, global `final accept` import and export)
forwards 100,000 ipv4/unicast routes about 5x slower than it did on
2026-09-21. Every figure below is the main pass (`ze.json`), repeat 4, quiet
host, 0 routes lost, read by the spec author from the result files of the
investigation session (that directory is not durable, so the numbers live
here):

| Build | Run | Convergence (median) | Stddev |
|-------|-----|----------------------|--------|
| HEAD edfc1e2d13 | `head-3` | 89 ms | 6 ms |
| 515de727d4 (2026-09-21) | `res-0921-2` | 17 ms | 0 ms |
| HEAD edfc1e2d13 | `head-4` | 92 ms | 4 ms |
| HEAD (Thomas's run, repeat 3) | `thomas-2026-10-31` | 86 ms | 31 ms |

A 30 s CPU profile (`PPROF=1`) of each build, read by the spec author with
`go tool pprof -top -cum`:

| Function (cumulative) | HEAD | 2026-09-21 |
|-----------------------|------|------------|
| total samples | 3.07 s | 2.23 s |
| `(*Peer).run`, the source read goroutine | 1.29 s | 0.14 s |
| `reactorForwardRSSection` | 1.21 s | n/a |
| `tryDirectWriteNoFlush` and `(*Session).writeUpdateBody` | 1.18 s | n/a |
| `nlrisplit.splitCIDR` (the per-NLRI walk, callbacks included) | 1.14 s | n/a |
| `(*adjOutWrite).record` / `recordSection` | 0.69 s | n/a |
| `runtime.mapassign` | 0.54 s | 0.23 s |
| `(*adjOutWrite).filter` / `section` | 0.45 s | n/a |
| `(*adjOutWrite).allow` | 0.40 s | n/a |
| `(*adjOutWrite).owesWithdrawal` / `(*Session).recoveryOwedFor` | 0.17 s | n/a |
| `(*adjOutWrite).recordDeletion` | 0.14 s | n/a |
| `(*adjRIBOut).takeReleased` | 0.05 s | n/a |
| `bufio.(*Writer).Write` | 0.02 s | n/a |

So the fast path is taken, the wire write itself costs nothing, and the time
is the Adj-RIB-Out ownership bookkeeping in
`internal/component/bgp/reactor/session_ownership.go`, added by b880c5a9a7
(2026-10-07) and changed by 22 commits since. A bisect could not narrow it:
the c9e5b8b340 and b880c5a9a7 builds lost every route in both passes, so the
commit-level attribution rests on the profile. The `final accept` policy is not
the cause: `reactorForwardRSSection` (`forward_rs.go`) checks
`pf.exportPolicy.AcceptsAtEnd` and `hasActiveFilter`, and the global
`final accept` leaves no active filter to run.

The goal is the main pass back near the 2026-09-21 figure with every ownership,
withdrawal, election and recovery behaviour unchanged, and a Go-level guard
that catches a return of the regression without the Docker lab.

**Item 2: the comparison report hides a pass.** Thomas's run printed one `ze`
row reading 334 ms convergence. That was `ze-propagation.json`, the
sender-first pass; the main pass `ze.json` read 86 ms ±31 and was not shown.
`run-head-3.log` shows the same: one `ze` row at 291 ms while `ze.json` held
89 ms. When a DUT has both a main and a propagation result, `le perf compare`
(markdown, HTML and the doc snapshot) must show both, each labelled with its
pass, with the pass named in one declaration.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md`, `ai/doors/le-tooling.md` - the design every change here keeps
  → Constraint: received bytes are borrowed; encoding is buffer-first; a per-route function allocates nothing (`ai/rules/performance.md`)
  → Constraint: a forwarder keeps per-source FIFO order and never drops a cached UPDATE (bgp-plugins door, Traps)
- [ ] `ai/rules/performance.md` - the reactor is a named hot path
  → Constraint: "A per-UPDATE, per-route or per-NLRI function MUST NOT allocate"; `recordSection` calls `new(adjRIBOutRoute)` per new key today
- [ ] `docs/architecture/update-building.md`, "One final ownership and duplicate boundary" - the design `session_ownership.go`, `adj_rib_out.go` and `recovery_owed.go` declare
  → Constraint: one current owner per native route and outgoing ADD-PATH presence and ID; an original or synthesized withdrawal requires an existing matching owner; an owner withdrawal at a destination without ADD-PATH send is owed to the recovery owner, not emitted
  → Decision: the page describes semantics, not allocation; the implementer adds one sentence on the entry store and the once-per-write recovery answer in the commit that lands each
- [ ] `docs/guide/benchmarking.md` - the perf runner, its two passes, and the comparison
  → Decision: a pass that lost routes is left out of every rendering; a complete pass of the same DUT is still rendered. This spec keeps that and adds that both complete passes render
- [ ] `docs/architecture/perf-round-3.md` - how earlier perf rounds proved their changes
  → Constraint: each change is proven by its own Go benchmark, not only by the end-to-end `le perf send` number, and preserves wire bytes byte for byte
- [ ] `plan/handover/12-rs-replacement-phase-3-remaining.md`, `plan/handover/30-bgp-rfc8212-phase3.md` - why the ownership bookkeeping exists
  → Constraint: the owed-withdrawal, announcement election, tombstone and released-receipt records are the closed route-server replacement and RFC 8212 work; every behaviour they prove stays
- [ ] `plan/learned/DESIGN-HISTORY.md` - "UPDATE delivery uses a bounded channel plus a goroutine PER PEER" and the engine invariants
  → Decision: moving the bookkeeping to another goroutine is rejected (Key Design Decisions)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - Section 9.2, quoted at `owesWithdrawal`
  → Constraint: "All newly installed routes and all newly unfeasible routes for which there is no replacement route SHALL be advertised to its peers by means of an UPDATE message." The owed-withdrawal decision stays per key
- [ ] `rfc/short/rfc7947.md` - Section 2.1, quoted at `oweAnnouncement`
  → Constraint: the announcement election stays the RIB's
- [ ] `rfc/short/rfc8212.md` - Section 3, quoted in `allow` and `recordSection`
  → Constraint: a receipt admitted under a superseded import policy stays refused; a released receipt keeps its slot
- [ ] `rfc/short/rfc7911.md` - Sections 3 and 5, quoted at `key` and `allow`
  → Constraint: the path identifier is part of the key; an unseen withdrawn identifier is ignored

**Key insights:**
- No wire byte changes in item 1: it is a cost change on a path whose output is already right. The ownership tests are the oracle.
- Item 2 is a report key that omits a fact (which pass) the producer used.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/session_write.go` - `writeUpdateBody`: under `writeMu`, `beginAdjOut`, then `ownership.filter(nil, body)`; when `removed != 0` it clears `owners`, `withdrawn` and the staged sets and runs `filter` a second time into a pooled buffer; PATHS-LIMIT may rewrite the body again; then `applyStaged`, `record(body)` over the final body, and the wire write
- [ ] `internal/component/bgp/reactor/session_ownership.go` - `adjOutWrite`, pooled (`adjOutWritePool`) with maps `owners`, `withdrawn`, `owes`, `movers`, `refused`, `announces`, `contested`, `ordinals`. `section` splits each NLRI section (`nlrisplit.Get`) and calls `allow` per NLRI; `allow` derives the key (`key` then `fwdPathKeyFor`), reads `table.routes[key]`, and for a withdrawal calls `owesWithdrawal`. `record` walks the final body again, re-derives every key, reads `owners[key]`, and per new key does `new(adjRIBOutRoute)`, a `table.routes` assign and `takeReleased`; per withdrawn key `recordDeletion` deletes and tombstones
- [ ] `internal/component/bgp/reactor/recovery_owed.go` - `recoveryOwedFor(source)`: loads the recovery worker, its owner command, and asks `ownerReceives`, which reads `DeliveryGraph().Receivers(...)` for the source address on every call: once per withdrawn NLRI today
- [ ] `internal/component/bgp/reactor/adj_rib_out.go` - `adjRIBOut` (`routes map[adjOutKey]*adjRIBOutRoute`, `owed`, `tombstones`, `released`, `sourcePaths`) and `adjRIBOutRoute` (owner, revision, signature, exactNLRI, contested)
- [ ] `internal/component/bgp/reactor/forward_rs.go` - `tryDirectWriteNoFlush` and `reactorForwardRSSection`: the fast path writes from the source's read goroutine
- [ ] `internal/perf/result.go` - `perf.Result` carries `dut-name`, `family`, `force-mp` and no field naming the pass
- [ ] `internal/perf/benchmark.go` - `BenchConfig.SenderFirst` decides the pass; the `Result` literal copies `DUTName`, `Family`, `ForceMP` and drops `SenderFirst`
- [ ] `internal/perf/cli/cmd_run.go` - `-sender-first` flag, "measures RIB propagation instead of live forwarding"
- [ ] `internal/perf/report/markdown.go` - `latestComparisonResults` keys on `comparisonKey{DUTName, Family, ForceMP}` and keeps the newest timestamp; the propagation pass runs second, so it always displaces the main pass. `groupResults` (markdown and HTML) and `PerformanceDoc` (doc snapshot) both go through it
- [ ] `internal/perf/report/html.go`, `internal/perf/report/perfdoc.go` - rows and chart bars are labelled by `DUTName` alone
- [ ] `internal/test/perfrunner/run.go` - `runPerf` names the file `<dut>-propagation.json` from `senderFirst`; `measureDUT` appends both result paths; `render` hands every path to the reporter; `passMeasurement` and `passPropagation` spell the passes a second time for the summary

**Behavior to preserve:**
- Every byte a destination receives, every owed withdrawal, election, tombstone, released receipt, recovery decision, PATHS-LIMIT outcome and send counter, as the tests named in AC-4 prove
- The fast path stays on the source read goroutine
- `le perf compare` still keeps the newest result per DUT, family, force-mp and pass, and still leaves out a pass that lost routes
- The runner's summary text for an incomplete pass

**Behavior to change:**
- Item 1: the per-NLRI cost of the ownership bookkeeping
- Item 2: a result records its pass; the report keys and labels on it

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Pooled per-write state | `session_ownership.go` `adjOutWritePool`, `adjOutWrite.release` | Reuses the write's maps, cleared on release | Extends: the per-write recovery answer and any per-write scratch live here and are cleared in `release` |
| Owners map filled by `allow` | `session_ownership.go` `adjOutWrite.owners` | Maps each admitted key to its owner; `recordSection` reads it back after re-deriving the key | Extends: phase 3 may carry the derived keys from the filter walk to the record walk instead of deriving them twice, if the benchmark shows the derivation still dominates |
| Recovery owner lookup | `recovery_owed.go` `Session.recoveryOwedFor`, `recoveryWorker.ownerReceives` | Reads the worker, its owner and the delivery-graph snapshot per withdrawn NLRI | Extends: answered once per source per write, under `writeMu`, from the same reads |
| Adj-RIB-Out table | `adj_rib_out.go` `adjRIBOut.routes`, `adjRIBOutRoute` | One heap object per installed key | Extends: entries come from a per-table chunked store with a free list, so installing a key does not allocate |
| Writer benchmark | `rfc4271_attr_flags_egress_test.go` `BenchmarkWriteUpdateBodyEgressWalk` | Drives `writeUpdateBody`, prepared by agents and run by the owner on the Mac | Extends: the new forwarded-write benchmark follows the same shape, driving the writer rather than a helper |
| Fast path | `forward_rs.go` `tryDirectWriteNoFlush` | Writes from the source read goroutine | Does not apply: kept as is; moving the work elsewhere is rejected (Key Design Decisions) |
| Perf result | `internal/perf/result.go` `perf.Result` | JSON record of one `le perf send` | Extends: gains the pass |
| Pass selection | `internal/perf/benchmark.go` `BenchConfig.SenderFirst` | Decides which pass runs | Extends: the one declaration of the passes and their names sits beside it, and the result copies the pass from the config |
| Report dedup | `internal/perf/report/markdown.go` `latestComparisonResults`, `comparisonKey` | Newest result per DUT, family, force-mp | Extends: the key gains the pass |
| Runner pass phrases and file name | `internal/test/perfrunner/run.go` `passMeasurement`, `passPropagation`, `runPerf` | Spell the passes for the summary and the file name | Replaces: derived from the perf declaration, so the pass is spelled once |
| Lab profile capture | `run.go` `Runner.captureProfiles` (`PPROF=1`) | Captures CPU profiles during the main pass | Does not apply: the evidence tool used to verify AC-1, unchanged |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Item 1: a forwarded UPDATE read by the source session, reaching `tryDirectWriteNoFlush` with `rs-fast-path enable`, as wire bytes in a borrowed read buffer
- Item 2: `le perf send` writes a result JSON; `le perf compare` and the runner's `render` read result JSON files

### Transformation Path
1. Item 1: `Peer.run` reads the UPDATE; `reactorForwardRSSection` picks the destinations; `tryDirectWriteNoFlush` calls `writeUpdateBody` per destination
2. `beginAdjOut` binds the write to the table under its mutex
3. `filter` walks every NLRI section once (twice when an NLRI is removed); `allow` derives the key, reads the table, stages owes, movers and elections
4. `applyStaged`, then `record` walks the final body, installs or deletes each key
5. The bytes go to the session's `bufio.Writer`
6. Item 2: `perf.Run` builds `Result` from `BenchConfig`; the report dedups, groups by family and renders rows

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Source read goroutine to destination session | `writeMu` and the table mutex, unchanged | Yes, by the tests in AC-4 |
| Engine to `bgp-rs` recovery owner | The recovery worker and the delivery graph, read, not called across a process | Yes: `ownerReceives` reads the immutable delivery-graph snapshot |
| `le perf send` to `le perf compare` | The result JSON file | Yes: the new field is in the JSON, kebab-case |

### Integration Points
- `writeUpdateBody` - unchanged call order; the bookkeeping it calls gets cheaper
- `perf.Run` - fills the pass from `BenchConfig.SenderFirst`
- `report.Markdown`, `report.HTML`, `report.PerformanceDoc` - key and label on the pass

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The writer's call order and the table mutex stay; only the per-NLRI work inside them changes |
| No unintended coupling (components stay isolated) | Yes | No new import; the report reads the result file only |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing Mechanisms: every change extends a named mechanism |
| Zero-copy preserved where applicable (refs, not copies) | Yes | The fast path still writes the received bytes; no new copy of a body |
| Registration over hardcoding, outbound: no per-feature field, switch case, or factory added to a core/shared package | Yes | The pass declaration is a value in `internal/perf` beside the flag that selects it |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name | Yes | Searched `internal/perf/` and `internal/test/perfrunner/` for `propagation` and `sender-first`: the runner's file name and summary phrases are the only other spellings, and both derive from the perf declaration after this spec |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The regression is the ownership bookkeeping, not another change since 2026-09-21 | The profile table above; `bufio.Write` 0.02 s | The fixes here do not reach AC-1 | AC-1 run with a fresh profile: `record`, `filter` and `allow` cumulative fall and convergence follows | unvalidated |
| A-2 | Within one write, the recovery owner answer for a source cannot change in a way the write must see | `recoveryOwedFor` reads an atomic worker pointer and an immutable delivery-graph snapshot; the write runs under `writeMu` | A concurrent owner change mid-write would split one UPDATE's keys between owed and written | A unit test that changes the owner between two writes sees the new answer on the second write; per-NLRI reads today can already split one write, so one read per write is never less consistent | confirmed (phase 3, `TestOwesWithdrawalReadsRecoveryOwnerOncePerWrite`): a write that detaches the owner after its first key still owes all 8 keys (per-key reads at HEAD owed 1 of 8), and the next write reads the detached owner and emits its withdrawal; the answer lives in `adjOutWrite.recoveryAnswers`, taken by `adjOutWrite.recoveryOwedFor` under `writeMu` after `beginAdjOut`, shared by both filter passes, cleared in `release` |
| A-3 | No `*adjRIBOutRoute` is held past its key's deletion or the table's reset | `recordDeletion`, `keepReleased`, `takeReleased`, `setOwner` | A reused slot would carry another key's owner: a wrong withdrawal or election | Grep every place that stores an entry pointer outside `table.routes` before phase 2; the released-receipt and tombstone tests stay green | confirmed (phase 1, `gopls references` on `adjRIBOutRoute`): the only field of that type is `adjRIBOut.routes`; every other reference is a call-scoped parameter under the table mutex (`keepReleased`, `oweReleasedRestore`, `bindSourcePath`, `owesWithdrawal`, `oweAnnouncement`, `adjOutContests`, `adjOutExactNLRI`, `adjOutHolds`, `departedOwner`, `adjOutWithdrawalNLRI`, `setOwner`, `releasePath`); no entry field address is taken; `adjOutReleased` copies owner and revision by value; `adjOutWithdrawalNLRI` copies `exactNLRI` out. Two sites the entry store must respect: `recordDeletion` reads `entry.owner` after `delete(w.table.routes, key)`, so the slot is freed after that read; `adj_rib_out_released.go` passes a temporary `&adjRIBOutRoute{owner: owner}` to `releasePath`, which is not a table slot and never enters the free list |
| A-4 | The 2026-09-21 build is a fair baseline although it had no ownership semantics | Thomas asked for the main pass "back near" that figure | AC-1's bound is unreachable with the semantics kept | Phase 4 measures; if the floor with every AC-4 test green stays above the bound, the implementer stops with the profile and asks Thomas which way to proceed | unvalidated |
| A-5 | No committed result file depends on the old JSON shape | `git ls-files test/perf/results` is empty | Old files would render as an unrecorded pass | Same command at implementation | confirmed 2026-11-01: `git ls-files test/perf/results` printed nothing; a pre-field file renders `not recorded` (`TestComparisonUnrecordedPassIsNamed`) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A cheaper bookkeeping path changes an election, tombstone or owed-withdrawal outcome | A test named in AC-4 goes red | Fix the code; no test in AC-4 is edited except through `ze-test-judge` |
| R-2 | The chunked entry store retains memory after a mass withdrawal | Heap in a 1M route soak stays at peak | Chunks are freed on `clearRoutes`; the free list reuses slots; retention is bounded by the session's peak table, the same bound the map already holds. Phase 4 evidence: `adjRIBOutEntries.clear` runs in `clearRoutes`, `free` zeroes a slot so a free slot keeps no Peer or signature alive, and `TestAdjOutEntryStoreReusesFreedSlots` shows the reuse and the empty store after a reset; no soak run |
| R-3 | Host load makes AC-1 noise (294 ms ±355 was observed at load average about 97) | Stddev above 10 ms | AC-1 is run by Thomas on the Mac on a quiet host, never by an agent |
| R-4 | The Go benchmark measures a path the lab does not take | Benchmark improves, lab does not | The benchmark drives `writeUpdateBody` with a source-bound forwarded item, the same authority and table state as the fast path |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Item 1: a route-server destination keeps a route it should withdraw, misses a replacement, or elects wrongly. Item 2: a perf report labels a pass wrongly |
| How is it reverted? | Each phase is one commit and reverts alone; the JSON field is additive |
| Who else touches this path? | Any session on the reactor's route-server replacement, RFC 8212 or PATHS-LIMIT work; check `git log` on `session_ownership.go` before each phase |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Forwarded UPDATE at an `rs-fast-path` destination | → | `writeUpdateBody` → `adjOutWrite.filter` / `record` | `BenchmarkAdjOutForwardedWrite` and `TestAdjOutForwardedWriteAllocations` in `internal/component/bgp/reactor/session_ownership_bench_test.go` |
| Forwarded withdrawal through the live route server | → | `owesWithdrawal` → `recoveryOwedFor` | `test/plugin/bgp-rs-withdrawal-replacement-fastpath.ci` |
| `le perf send --sender-first` | → | `perf.Run` fills the pass | `TestRunResultRecordsPass` in `internal/perf/benchmark_test.go` |
| `le perf compare` over two passes of one DUT | → | `report.Markdown`, `report.HTML`, `report.PerformanceDoc` | `TestComparisonRendersBothPassesOfOneDUT` in `internal/perf/report/report_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le perf run dut ze` on Thomas's Mac, quiet host, 100,000 ipv4/unicast routes, repeat 4 | The main pass (`ze.json`) reports a median convergence of at most 26 ms (1.5x the 17 ms of 2026-09-21), stddev at most 5 ms, 0 routes lost; the pasted `./le perf` output and a fresh `PPROF=1` profile of `(*Peer).run` are recorded in this spec |
| AC-2 | `BenchmarkAdjOutForwardedWrite`: a source-bound forwarded UPDATE of about 1,000 IPv4 /24 NLRIs (the `le perf send` auto batch within 4096 bytes) written through `writeUpdateBody` to a fresh table, then withdrawn | Reports ns/op and allocs/op per case; Thomas runs it on the Mac at HEAD before phase 1 and after phase 3, and both outputs are pasted in this spec |
| AC-3 | `TestAdjOutForwardedWriteAllocations`: the same forwarded write of N fresh keys for N = 10 and N = 1,000, and a same-owner re-announcement of held keys | Allocations per write do not grow with N beyond one chunk per chunk-size keys, and the re-announcement allocates nothing; the test goes red at HEAD (red recorded in the commit body) |
| AC-4 | The reactor package with `-tags ze_bgp`, the `bgp-rs` plugin package, and the functional tests `bgp-rs-withdrawal-replacement.ci`, `bgp-rs-withdrawal-replacement-fastpath.ci`, `bgp-rs-withdrawal-replacement-gr.ci`, `bgp-rs-withdrawal-replacement-many.ci`, `bgp-rr-withdrawal-replacement.ci`, `addpath-departure-rr-replacement.ci` | All pass with no assertion weakened; the ownership oracle files are `session_ownership_test.go`, `session_ownership_writer_test.go`, `rfc7947_rs_announcement_election_test.go`, `rfc7947_forward_rs_test.go`, `recovery_owed_test.go`, `recovery_owed_fence_test.go`, `recovery_owed_worker_test.go`, `recovery_owed_staging_test.go`, `recovery_owed_no_export_test.go`, `relay_recovery_test.go`, `rfc8212_released_receipt_test.go`, `rfc8212_released_two_source_test.go`, `rfc8212_recovery_epoch_test.go`, `session_owed_work_test.go`, `session_paths_limit_recovery_test.go`, `forward_aggregator_tombstone_test.go`, `rfc8654_rib_recovery_test.go` |
| AC-5 | A forwarded withdrawal of N keys owned by one source, at a destination whose recovery owner receives that source, then the owner detached and a second withdrawal written | The first write owes all N keys and emits none; the second write emits its withdrawals; the recovery owner's answer is read once per source per write |
| AC-6 | `le perf send` with `-sender-first`, and without it | The result JSON carries `"pass": "rib-propagation"`, and `"pass": "live-forwarding"`, respectively |
| AC-7 | `le perf compare` (markdown) over a `live-forwarding` and a `rib-propagation` result of DUT `ze`, same family, the propagation one newer | Two rows for `ze`, each with its own convergence, and a Pass column naming `live-forwarding` and `rib-propagation` |
| AC-8 | The same two results with `--html` | Two table rows and two chart bars, each labelled with DUT and pass |
| AC-9 | The same two results with `--doc` | The results table shows both rows with their pass; the DUT setup section lists `ze` once |
| AC-10 | A result file with no `pass` field beside one with a pass, same DUT | The unrecorded one renders with the pass cell reading `not recorded`, never as either pass, and does not displace the other |
| AC-11 | The perf runner over a ze run whose propagation pass lost routes | The summary line names the pass with the same words as today (`the propagation pass`), derived from the one declaration; the file is still `ze-propagation.json` |
| AC-12 | `docs/guide/benchmarking.md` and `docs/guide/command-reference.md` (`le perf compare`, `le perf send`) | Describe the `pass` field, the Pass column, and that both passes of a DUT render |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAdjOutForwardedWriteAllocations` | `internal/component/bgp/reactor/session_ownership_bench_test.go` (built `!race`; `session_ownership_bench_race_test.go` skips the name under `-race`, whose `sync.Pool` drops make counts the detector's) | AC-3 | written, red at HEAD (cycle 10 keys 26.0, 1000 keys 1016.0 allocs; re-announce 8.0), green after phase 4 (0.0, 0.0; 0.0) |
| `BenchmarkAdjOutForwardedWrite` | `internal/component/bgp/reactor/session_ownership_bench_test.go` | AC-2, owner-run on the Mac | written, smoke-run `-benchtime=1x` only |
| `TestOwesWithdrawalReadsRecoveryOwnerOncePerWrite` | `internal/component/bgp/reactor/recovery_owed_test.go` | AC-5 | written, red at HEAD (owed 1 of 8), green after phase 3 |
| `TestAdjOutEntryStoreReusesFreedSlots` | `internal/component/bgp/reactor/adj_rib_out_test.go` | A freed slot is reused for a new key and carries none of the old key's owner, revision, signature or contested bit; `clearRoutes` drops every chunk (A-3, R-2) | written, red before the store was wired (slot not reused), green after phase 4 |
| `TestRunResultRecordsPass` | `internal/perf/benchmark_test.go` | AC-6 | |
| `TestComparisonRendersBothPassesOfOneDUT` | `internal/perf/report/report_test.go` | AC-7, AC-8, AC-9 | |
| `TestComparisonUnrecordedPassIsNamed` | `internal/perf/report/report_test.go` | AC-10 | |
| `TestRunnerNamesPassFromDeclaration` | `internal/test/perfrunner/run_test.go` | AC-11 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| NLRIs per forwarded write (benchmark and alloc test) | 1 to the 4096-byte message bound, 65535 with RFC 8654 | 1 and the bound | N/A | N/A: the split refuses a truncated body today |
| Entry chunk size | one fixed constant | the constant | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-rs-withdrawal-replacement-fastpath` | `test/plugin/bgp-rs-withdrawal-replacement-fastpath.ci` | A route server on the fast path replaces a withdrawn route with the RIB's next best | existing, must stay green |
| `bgp-rs-withdrawal-replacement-many` | `test/plugin/bgp-rs-withdrawal-replacement-many.ci` | Many owed keys at once | existing, must stay green |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-relay-withdraw-shape-frr` | `test/interop/scenarios/` | FRR | Relayed withdrawals keep their shape against FRR. Item 1 changes no wire byte, so this is regression evidence the main thread runs, not a new scenario | existing |

## Files to Modify
- `internal/component/bgp/reactor/session_ownership.go` - per-write recovery answer; record walk without a per-key heap object; optional key carry (phase 3)
- `internal/component/bgp/reactor/adj_rib_out.go` - chunked entry store and free list for `adjRIBOutRoute`, reset in `clearRoutes`
- `internal/component/bgp/reactor/recovery_owed.go` - `recoveryOwedFor` comment if its call contract changes
- `internal/perf/result.go` - `pass` field
- `internal/perf/benchmark.go` - the pass declaration (values and names) and filling `Result`
- `internal/perf/report/markdown.go`, `internal/perf/report/html.go`, `internal/perf/report/perfdoc.go` - key and label on the pass
- `internal/test/perfrunner/run.go` - file name and summary phrases derived from the declaration
- `docs/guide/benchmarking.md`, `docs/guide/command-reference.md` - AC-12
- `docs/architecture/update-building.md` - the entry store and the per-write recovery answer (doc checklist row 12)

## Files to Create
- `internal/component/bgp/reactor/session_ownership_bench_test.go` - AC-2, AC-3
- `internal/component/bgp/reactor/session_ownership_bench_race_test.go` - the AC-3 test name under `-race`, skipped with its reason

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config |
| YANG validation constraints | N-A | No config |
| YANG custom validators | N-A | No config |
| CLI commands/flags | No | `le perf send` and `compare` keep their flags; only the JSON and the rendering change |
| CLI grammar (keyword before value) | N-A | No new command |
| Editor autocomplete | N-A | No YANG |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | N-A | No `ze` command output |
| Env var registration | N-A | None |
| Doctor check for runtime dependencies | N-A | No runtime dependency |
| Prometheus counters/metrics | No | The send counters stay as they are |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Performance only |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`le perf compare` output, `le perf send` JSON) |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/benchmarking.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Behaviour unchanged; the RFC-tagged tests in AC-4 stay as they are |
| 10 | Test infrastructure changed? | No | The runner's output changes, documented in `benchmarking.md` |
| 11 | Affects daemon comparison? | Yes | `docs/performance.md` is hand-written; its next refresh shows both passes. No edit owed by this spec unless Thomas's AC-1 run is published there |
| 12 | Internal architecture changed? | Yes | `docs/architecture/update-building.md`, "One final ownership and duplicate boundary": the per-table entry store and the once-per-write recovery answer |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Derived at implementation | `./le spec citation anchors spec plan/release/validation/spec-perf-rs-fast-path-regression-and-pass-labels.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The `le perf compare` examples in `benchmarking.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the benchmark, the allocation test and the report test, all failing
   - Tests: `TestAdjOutForwardedWriteAllocations`, `BenchmarkAdjOutForwardedWrite`, `TestComparisonRendersBothPassesOfOneDUT`
   - Files: `session_ownership_bench_test.go`, `report_test.go`
   - Verify: the alloc test is red at HEAD; the report test shows one `ze` row; Thomas runs the benchmark at HEAD for the before figure
2. **Phase: Item 2, the pass** -- declaration, `Result` field, report key and label, runner derivation, docs
   - Tests: `TestRunResultRecordsPass`, `TestComparisonRendersBothPassesOfOneDUT`, `TestComparisonUnrecordedPassIsNamed`, `TestRunnerNamesPassFromDeclaration`
   - Files: `internal/perf/`, `internal/perf/report/`, `internal/test/perfrunner/run.go`, the two guide pages
   - Verify: tests fail, implement, pass; lands as its own commit
3. **Phase: Item 1a, recovery answer once per write**
   - Tests: `TestOwesWithdrawalReadsRecoveryOwnerOncePerWrite`, AC-4 set
   - Files: `session_ownership.go`, `recovery_owed.go`
   - Verify: AC-4 green; own commit
4. **Phase: Item 1b, entry store**
   - Tests: `TestAdjOutEntryStoreReusesFreedSlots`, `TestAdjOutForwardedWriteAllocations`, AC-4 set
   - Files: `adj_rib_out.go`, `session_ownership.go`
   - Verify: A-3 grep done first and recorded; alloc test green; own commit
5. **Phase: Item 1c, key carry, only if measured** -- Thomas runs the benchmark; if `fwdPathKeyFor` and the `owners` lookups in `recordSection` still hold more than a quarter of the write's time, carry the keys the filter walk derived, in final-body order, to `record`, for the case where the final body is the filter's output; otherwise this phase records the measurement and does nothing
   - Tests: AC-4 set, `TestAdjOutForwardedWriteAllocations`
   - Files: `session_ownership.go`
6. **Phase: lab proof** -- Thomas runs AC-1 on the Mac with `PPROF=1`; the output and profile figures are pasted here. If AC-1's bound is not met with AC-4 green, stop with the profile and ask Thomas which way to proceed (A-4)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No AC-4 file is edited; the recovery answer is read under `writeMu` after `beginAdjOut`; a reused entry slot is zeroed before use |
| Correctness | `record` still installs exactly the final body's NLRIs after a second filter pass and after a PATHS-LIMIT rewrite |
| Naming | JSON key `pass`, values kebab-case; the pass is spelled in one place |
| Data flow | The report reads the pass from the result, never from a file name |
| Rule: `ai/rules/performance.md` | No per-NLRI allocation left in `recordSection` for a key installed into a non-full chunk |
| Rule: `ai/rules/no-layering.md` | The per-key `new(adjRIBOutRoute)` is deleted, not kept as a fallback |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Allocation guard | `./le job run label unit-pkg command go test -tags ze_bgp -run TestAdjOutForwardedWriteAllocations ./internal/component/bgp/reactor/` |
| Ownership oracle green | `./le job run label unit-pkg command go test -tags ze_bgp ./internal/component/bgp/reactor/ ./internal/component/bgp/plugins/rs/` |
| Report shows both passes | `./le job run label unit-pkg command go test ./internal/perf/... ./internal/test/perfrunner/` |
| Lab figure | AC-1 output pasted in this spec |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | The entry store's chunks are bounded by the session's peak table and freed on reset; a peer that announces and withdraws in a loop reuses slots |
| Input validation | Result JSON with an unknown `pass` value is refused by the report with the file name, the value and the accepted values |

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

- The propagation pass always displaces the main one because it runs second and `latestComparisonResults` keeps the newest timestamp per `comparisonKey`, which omits the pass. Found at the producer, `internal/perf/report/markdown.go`.
- The 2026-09-21 propagation result (`res-0921-2/ze-propagation.json`) lost 84,463 of 100,000 routes, so only the main pass has a valid 2026-09-21 baseline.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `same-bytes-decoded-once-per-accessor.md` | 2026-11-01 | `adjOutWrite.filter` and `record` derive every NLRI key twice, with per-NLRI recovery lookup and allocation |
| `key-omits-a-fact-the-builder-uses.md` | 2026-11-01 | `latestComparisonResults` keys omit the perf pass |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Answer the recovery owner once per source per write | Per NLRI, as today | Under `writeMu` one answer is at least as consistent as N answers that can differ mid-UPDATE; the reads are an atomic load and an immutable snapshot (A-2) |
| Chunked entry store with a free list, pointers kept | Store `adjRIBOutRoute` by value in the map | Every caller (`setOwner`, `owesWithdrawal`, `keepReleased`) takes the entry by pointer; by-value storage would turn each into a read-modify-write and widen the change across the ownership code |
| No map pre-size | Size `table.routes` from the prefix maximum | A guessed size is wrong in both directions: the perf config's maximum is 1,000,000, so pre-sizing would hold memory for routes that never come; growth is amortized |
| Key carry only after measurement | Merge the filter and record walks now | `record` must reflect the final body, which a second filter pass or PATHS-LIMIT can change; the carry adds a second shape of the record walk, worth it only if the derivation still dominates after phases 3 and 4 |
| Keep the bookkeeping on the source read goroutine | Move it to a destination worker | The fast path exists to write from the read goroutine; moving the write moves the cost to another core and adds a handoff copy of borrowed bytes, without removing work. Per-peer delivery ordering (`DESIGN-HISTORY`) would also need a new ordering guarantee |
| The pass is a field of the result, named once in `internal/perf` | Read the pass from the file name `-propagation.json` | A file name is a second declaration and breaks for any manual `--output` path |
| An unrecorded pass is named as such | Treat a missing field as the live-forwarding pass | A silent default would label an old propagation result as the main pass, the defect this spec fixes (`ai/rules/principles.md`) |

## Known Limitations

- `le perf track` compares results in file order with no DUT or pass key (`CheckHistory`, `internal/perf/regression.go`). A history file is one DUT's series by the command's contract, and the runner never writes one, so this spec does not change it.

## RFC Documentation (Scope: protocol)

No RFC behaviour changes. The existing quotes at `owesWithdrawal`, `oweAnnouncement`, `allow`, `key` and `recordSection` stay above the code they describe; a moved statement carries its quote with it.

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, and `./le verify changed` over the closing commits
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/validation/spec-perf-rs-fast-path-regression-and-pass-labels.md` only, in the same `./le commit create` script
