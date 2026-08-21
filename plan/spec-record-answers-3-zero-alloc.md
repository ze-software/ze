# Spec: record answers child 3 -- a zero-allocation, pooled record path

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | spec-record-answers-2-only-encoding |
| Phase | - |
| Deferral shard | `plan/deferrals/record-answers.md` |
| Handoff | - |
| Updated | 2026-08-21 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Child 3 of three. Child 1 gave the SDK a record producer and reader; child 2 made
the record sequence the only command-answer encoding and gave its lines a
three-byte kind token and a length-prefixed id. This child removes the
allocations behind that frame and converts the walks large enough for the
removal to matter.

## Task

The framing already allocates nothing. `AppendAnswerHead` and its siblings append
into a caller-owned buffer and write ids with `strconv.AppendUint`, and
`WriteAnswer` reuses one line buffer for every line of one answer. Every
allocation on a record answer is upstream of that, in how a row reaches the
encoder, and there are four.

A handler builds a `map[string]any` for each row through `plugin.Map`, 219 call
sites in the tree. The one existing generator, `commandRows` in
`internal/component/plugin/server/system.go`, then calls `json.Marshal` for each
row and a second `json.Marshal` over a fresh map for each fault. `rpc.Record`
holds two `json.RawMessage` fields, so the row type itself forces that
allocation and no producer can avoid it. `WriteAnswer` takes its line buffer
from a fresh 512-byte `make` for each answer rather than from
`internal/core/bufpool`, `answerFrame` does the same with 256 bytes on the exec
channel, and `ResponseJSON` copies its marshaled bytes into a string for every
buffered surface.

Two walks make this matter rather than being a tidy-up. `showPipeline` and
`bestPipeline` in `internal/component/bgp/plugins/rib/` build a `map[string]any`
for each route through `serializeRouteItem`, collect them into nested maps,
marshal the lot, and copy the result into a string. Both hold
`r.peerMu.RLock()` across the whole build. A full RIB is millions of routes.

`Records.Fields` also still has no producer, so the `tab` item type child 2
defines is unreachable and every streamed answer repeats its keys on every row.

The goal is a record path that allocates nothing for each row, takes its buffers
from a pool, holds no lock across a socket write, and can answer positionally.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/performance.md` - buffer ownership, pool strategy, the encoding rules
  → Constraint: the caller owns the buffer and the callee appends into it; a row API returning a fresh slice cannot be made allocation-free later
  → Constraint: every buffer in a pool has one maximum size, because variable-sized allocation defeats the pool
  → Decision: `fmt.Sprintf` and `.String()` concatenation are refused on this path; `textbuf.Buffer` or `strconv.Append*` into an owned buffer replaces them
- [ ] `docs/contributing/ze-style.md` - the memory table and the four reasons a copy is allowed
  → Constraint: a copy that fits none of the four named reasons is a defect until somebody names the fifth
  → Constraint: zero the padding, because a buffer written short and sent long leaks what the last user left in it
- [ ] `docs/architecture/api/ipc_protocol.md` - the answer grammar and what the head's column names mean
  → Constraint: the column names declare the SCHEMA, not the wire shape; the encoder still decides whether an answer streams at all
- [ ] `docs/architecture/core-design.md` - where the RIB read path sits and what it may hold
  → Constraint: a read that spans a socket write must not hold a lock the UPDATE write path needs
- [ ] `ai/rules/goroutine-lifecycle.md` - no goroutine for one event on a hot path
  → Constraint: a generator must not be drained by a goroutine started per answer

**Key insights:** (minimal context to resume after compaction)
- `rpc.Record` is the ceiling. While a row is two `json.RawMessage` fields, one allocation per row is forced whatever the handler does.
- The alloc gate exists and is fail-closed on absence, but `mk/alloc-gate.mk` benchmarks `./internal/component/bgp/reactor/...` only. A plugin-path benchmark is invisible to it until that glob widens.
- A benchmark opts into the gate by one entry in `perf.AllocCeilings`, never by editing the makefile. That is the registration point this spec uses.
- The closest buffer-first precedent for a ROW is the `AppendTo(buf []byte) []byte` family, not `WriteTo(buf, off) int`, whose users are fixed-layout packet encoders.
- The two column registries stay separate by owner directive: `RegisterColumns` orders columns for a person, `Records.Fields` declares a schema for a program.
- `bestPipeline` states that its drain dereferences `InEntry` pool handles that `handleReceived` may release, which is why its lock spans the drain.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `pkg/plugin/rpc/message.go` - `AppendAnswerHead`, `AppendAnswerItem`, `AppendAnswerFault`, `AppendAnswerTerminator` append into a caller buffer with no `fmt`; `AnswerRecordLineSize` measures a line before it is built; `ParseAnswerTail` refuses an unknown key
- [ ] `pkg/plugin/rpc/types.go` - `Record` is two `json.RawMessage` fields, which forces one allocation per row
- [ ] `internal/component/plugin/dispatch.go` - `answerLineCapacity` is a fresh 512-byte allocation per answer; `WriteAnswer`, `writeRecordAnswer`, `writeRecordLine`, `boundedRecord`, `checkRowArity`; `ResponseJSON` copies marshaled bytes into a string

**CHILD 1 MOVED MOST OF THIS SPEC'S TARGETS, 2026-08-21. Read before planning.**
`pkg/plugin/sdk` cannot import `internal/`, so making the SDK speak this
protocol moved the protocol's code into `pkg/plugin/rpc`. The line encoding is
there; the engine's `*Response` adapter stayed behind.

| This spec names | It is now at |
|-----------------|--------------|
| `answerLineCapacity`, the fresh 512-byte per-answer allocation (phase 2's target) | `pkg/plugin/rpc/answer_write.go` |
| `writeRecordAnswer`, `writeRecordLine`, `boundedRecord` (phase 3's targets) | `pkg/plugin/rpc/answer_write.go` as `WriteRecordAnswer`, `writeRecordLine`, `boundedRecord` |
| `internal/component/plugin/answer_row.go`, `zipRow`, `quoteFields` (phase 4's target) | `pkg/plugin/rpc/answer_row.go`. **The old file no longer exists** | <!-- doc-links: ignore (the left column names the path this machinery MOVED OUT OF; the row states that it no longer exists) -->
| `ResponseJSON`, `WriteAnswer`, `AnswerFor` | unmoved, `internal/component/plugin/dispatch.go` |

Two consequences for this spec's own goal, and neither is cosmetic:

- **The per-row allocation now has a NAMED producer to delete.** `Records.wire()`
  (`pkg/plugin/records.go:147`) appends each `Row` into a fresh slice, because
  `rpc.Record`'s two `json.RawMessage` fields force it and `rpc.Record`'s own doc
  forbids a reused scratch. Child 1 put the SDK's `Row` interface at
  `AppendTo(buf []byte) []byte` precisely so this spec can remove that hop. A-1
  is therefore already half-proven: the appender shape exists and has a producer.
- **~~The pool now serves two packages, and `pkg/plugin/rpc` cannot import
  `internal/core/bufpool`.~~ THAT WAS WRONG (corrected 2026-08-21, audit).**
  `pkg/` already imports `internal/` in this module and compiles:
  `pkg/plugin/rpc/bridge.go:15` takes `internal/core/selector`, and
  `pkg/plugin/sdk/sdk.go:40-41` takes `internal/component/plugin/ipc` and
  `internal/core/env`. Go's internal rule is rooted at the module, so the import
  is legal, and no gate forbids the pair: `ze-tier-check` (`Makefile:864`) judges
  `internal/component` against `internal/plugins` placement only, and
  `.golangci.yml` carries no depguard for it.

  **The real question is narrower, and it is about SIZING, not location.**
  `internal/core/bufpool.Pool` is fixed-size and its `Put` SILENTLY DROPS any
  buffer whose `cap != size`. The answer line grows by append, so a pool of that
  shape drains under exactly the wide rows this spec exists to make cheap.

  **The precedent is already in the same file as the writers.**
  `pkg/plugin/rpc/framing.go` runs `framePool` (`:36`, with `getFrameBuf:43` /
  `putFrameBuf:57`) for every request, response and event line: a `sync.Pool` of
  `*[]byte`, 4 KiB initial (`frameBufInitial:23`), dropped above 64 KiB
  (`frameBufMax:24`). `batch.go:batchBufPool:22` and
  `bridge.go:structuredEventPool:20` are two more in the same package. Grow and
  cap is the shape this repository already uses for a line that grows, and it is
  the one `ai/rules/performance.md`'s "one maximum size per pool" is satisfied by
  here, because the CAP is that size and `MaxMessageSize` (16 MiB,
  `framing.go:66`) is not a pool size anyone can seed for.

  What still has to be decided in phase 2, and it is one line of design rather
  than a blocker: reuse `framePool` for the answer line, or add a fourth pool
  beside it. Reuse shares one population with every other RPC line; a new pool
  keeps the answer's sizing independent. `internal/component/ssh/answer.go`
  keeps its own 256-byte frame either way (`newAnswerFrame:63`,
  `answerFrameCapacity:44`), so AC-2 spans two packages that must agree on one
  answer.
- [ ] `internal/component/plugin/types.go` - `Records` carries `Key`, `Fields` and `Rows iter.Seq[rpc.Record]`; `Fields` has no producer; `CollapseRecords` is the one collapse for buffered surfaces; the `errors` envelope is reserved
- [ ] `internal/component/plugin/server/system.go` - `commandRows` and `yieldCompletion` marshal each row, and marshal a fresh map for each fault
- [ ] `internal/component/ssh/answer.go` - `answerFrame` reuses one 256-byte line buffer per answer (`answerFrameCapacity`); `writeExecRecords` renders through `command.RenderRecords` and frames what the walk turned out to be
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline.go` - `showPipeline` holds `r.peerMu.RLock()` across construct and drain; `inboundSource.Next` refills a reused slice but builds one prefix string per route; `jsonTerminal.drain` and `serializeRouteItem` build a map per route, then nested maps, then marshal, then copy into a string
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline_best.go` - `bestPipeline` holds `RLock` across the drain because the drain dereferences `InEntry` pool handles; `newBestSource` builds a map, a key string and a byte-slice copy per route, appends into a slice, then sorts
- [ ] `internal/core/bufpool/bufpool.go` - `Pool` is a fixed-size byte-slice pool backed by `sync.Pool`, seeded for peak
- [ ] `internal/core/textbuf/textbuf.go` - `Buffer` with `Get()` and `Release()`, the string-building replacement for `fmt`
- [ ] `internal/perf/allocgate.go` - `AllocCeilings` maps a bare benchmark name to a maximum allocs/op; absence from the benchmark output is a violation, so the gate fails closed
- [ ] `mk/alloc-gate.mk` - `ze-alloc-check` benchmarks the reactor tree at `-benchtime=300x`, tees to `tmp/verify/alloc-gate-bench.txt`, then runs `TestAllocGateEnforce` in `internal/perf`
- [ ] `scripts/dev/spec_doc_anchors.py` - row 16 of the documentation checklist is derived from `// Design:` headers, which is why this spec names them rather than answering from memory
- [ ] `internal/component/command/column_order.go` - `RegisterColumns` orders columns for human rendering only, by owner directive

**Behavior to preserve:**
- The payload an operator sees. Every command's data is unchanged; only how it reaches the wire changes.
- The type decision stays in the encoder, taken from the record count, and the threshold stays a constant rather than a knob.
- `count` counts rows produced and `faults` counts rows rejected, and neither counts lines.
- A fault does not end a walk, and an over-wide row is rejected as a fault rather than costing the rows around it.
- Buffered surfaces keep reading one document through `CollapseRecords`.
- `RegisterColumns` keeps its human-rendering meaning and stays separate from `Records.Fields`.
- The alloc gate stays fail-closed: a registered benchmark absent from the output is a violation, not a pass.
- A benchmark still opts in through `perf.AllocCeilings`, not by editing `mk/alloc-gate.mk`.

**Behavior to change:**
- A row reaches the encoder as bytes appended into the encoder's buffer, not as a freshly marshaled slice.
- The line buffer comes from a pool, on both the plugin connection and the SSH exec channel.
- `showPipeline` and `bestPipeline` answer with generators, and hold no lock across a socket write.
- `Records.Fields` gains a producer, so the `tab` item type is reachable.
- The alloc gate's benchmark scope widens beyond the reactor tree to cover the record path.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A command whose handler answers with a row generator, reaching `WriteAnswer` over the plugin connection or `answerFrame` over the SSH exec channel.
- At entry the handler holds the collection it will walk: a RIB view, a peer set, a flow table.

### Transformation Path
1. The answer takes its line buffer from the pool rather than allocating one, and returns it when the answer ends.
2. The generator yields a row that appends its own bytes into that buffer, so no row-sized slice is created.
3. A handler that declares a column schema yields positional rows, and the encoder writes the names once on the head.
4. The appended row is measured in place and an over-wide one is rejected as a fault, without building the row twice.
5. For the RIB walks, the peer set is taken under the lock and the walk proceeds without it, so no socket write happens under `peerMu`.
6. The terminator returns the pooled buffer, whatever ended the walk.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Handler ↔ encoder | rows append into the encoder's buffer instead of returning slices | No |
| Encoder ↔ pool | line buffer acquired once per answer, released on every exit path | No |
| RIB read ↔ socket write | the walk spans the write, so the lock must not | No |
| Encoder ↔ buffered surfaces | `CollapseRecords` still produces one document from the same rows | No |

### Integration Points
- `bufpool.Pool` (`internal/core/bufpool`) - the line buffer's source, for `WriteAnswer` and for `answerFrame`.
- `textbuf.Buffer` (`internal/core/textbuf`) - the replacement for every `fmt` call and string concatenation found on this path.
- `AnswerRecordLineSize` (`pkg/plugin/rpc/message.go`) - the width check, which must not force a second build of the row.
- `AllocCeilings` (`internal/perf/allocgate.go`) - where each new benchmark's ceiling is registered.
- `serializeRouteItem` (`internal/component/bgp/plugins/rib/rib_pipeline.go`) - the per-route map builder the conversion replaces.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A row can append into the encoder's buffer without the encoder knowing the row's type | `AppendTo(buf []byte) []byte` is an established shape in the tree (`internal/core/family/family.go`, `pkg/plugin/rpc/enums.go`) | The row type keeps forcing one allocation, and zero allocation per row is unreachable | `AllocsPerRun` over a walk of 1000 rows returning zero | unvalidated |
| A-2 | The RIB walk can release `peerMu` before the socket write | `bestPipeline` states its drain dereferences `InEntry` pool handles `handleReceived` may release | The walk must copy what it needs under the lock, which reintroduces the per-route allocation this spec removes | Race detector over a walk concurrent with `handleReceived` | unvalidated |
| A-3 | One pool size fits every answer line | `ai/rules/performance.md` requires one maximum size per pool, and `MaxMessageSize` bounds a line | A row wider than the pooled buffer forces a per-line allocation, so the pool would cover only the common case | Benchmark over the widest row the RIB produces | unvalidated |
| A-4 | The alloc gate can cover a non-reactor package | `mk/alloc-gate.mk` hardcodes the reactor glob, and `TestAllocGateEnforce` enforces whatever the bench emits for registered names | The gate cannot see this path, and the zero-allocation claim has no enforcement | Widen the glob, then delete a pool release and watch the gate go red | unvalidated |
| A-5 | A command's column schema is known before its first row | `Records.Fields` is written on the head, which precedes every row | A schema discovered mid-walk cannot be declared, so those commands stay `ndjson` | Convert one command with a fixed schema and one without | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A pooled buffer is not released on an error path, so the pool drains under load | Answer latency rises over hours, and pool misses climb | Acquire and release in one function with the release deferred, and assert the pairing in a test that forces every error path |
| R-2 | A pooled buffer is retained by a row that outlives the answer | Corrupted answers under concurrency, or data from one answer appearing in another | The row appends and returns; nothing keeps a reference. State it as a MUST in the row contract's doc comment, on both sides of the pair |
| R-3 | Releasing `peerMu` before the walk lets a route be freed mid-answer | Race detector failure, or a row of zero values | Take a reference under the lock that survives the walk, or refcount the handle. The design phase names which |
| R-4 | The lock is held longer instead, spanning a slow client | A slow reader stalls the RIB write path | AC-5 forbids it: no socket write happens under `peerMu`, and a test asserts it |
| R-5 | Zero allocation is claimed but measured on a path the gate never runs | `make ze-alloc-check` green while the new benchmark is absent from the output | The gate is fail-closed on absence, and AC-3 requires the new benchmark to appear in `AllocCeilings` |
| R-6 | The positional row and the head's column names drift, so a consumer zips the wrong names | `checkRowArity` passes but values land under the wrong keys | Arity is already checked; add a test asserting names and values round-trip through `zipRow` |
| R-7 | Removing `plugin.Map` from a converted handler changes the payload | A `.ci` payload diff on a converted command | Child 2 proved the payload survives a reframe; here the same assertion runs before the build changes |
| R-8 | A pooled buffer carries the previous answer's bytes into this one | A line longer than what was written, with trailing data from another answer | Every write sets its own length, and no line is sent longer than it was written |
| R-9 | The conversion stops at the two RIB walks and the other large walks stay allocating | Peers, flows and traffic stats still build a map per row | Named as a Known Limitation with a deferral row, so it is a boundary rather than an oversight |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The RIB read path. A lock error can stall the BGP write path or read a freed route, which is the most severe failure in this family. A pool error degrades over hours rather than immediately, which makes it the harder one to see |
| How is it reverted? | Single commit revert per phase. No config migrates and nothing persists |
| Who else touches this path? | Any spec touching `rib_pipeline.go`, `rib_pipeline_best.go`, `dispatch.go`, `ssh/answer.go`, or the alloc gate |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A command answers with rows | → | `WriteAnswer` acquires and releases a pooled line buffer | `TestWriteAnswerUsesPooledBuffer` |
| A row is written | → | the row appends into the encoder's buffer | `TestRecordRowAppendsInPlace` |
| `show bgp rib` over a large table | → | `showPipeline` answers with a generator | `test-show-rib-streams` |
| `make ze-alloc-check` runs | → | the record-path benchmark appears in `AllocCeilings` | `TestAllocGateEnforce` |
| A handler declares a column schema | → | `Records.Fields` reaches the head's column names | `TestStreamAnswerDeclaresFields` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A walk of 1000 rows is written | The measured allocation count per row is zero |
| AC-2 | An answer is written and completed | Its line buffer came from the pool and was returned, on the success path and on every error path, over the plugin connection and over the exec channel |
| AC-3 | `make ze-alloc-check` runs | A benchmark covering the record path appears in the output with a ceiling registered in `perf.AllocCeilings`, and deleting the pool release turns the gate red |
| AC-4 | `show bgp rib` runs against a table larger than the buffer threshold | Rows stream, and the daemon never holds the whole table as one document |
| AC-5 | `show bgp rib` runs while UPDATEs are being processed | No socket write happens while `peerMu` is held, and the race detector is clean |
| AC-6 | A handler declares a column schema and its walk passes the threshold | The head's item type is `tab` and it carries the column names, and each row is a positional array whose arity matches |
| AC-7 | A handler declares no schema and its walk passes the threshold | The head's item type is `map` and rows stay self-describing |
| AC-8 | A converted command answers a bounded walk | The payload is byte-identical to the same command before this spec |
| AC-9 | A row wider than one wire message is produced | It is rejected as a fault, the walk continues, and the rejection costs no second build of the row |
| AC-10 | The record path is searched for `fmt` and string concatenation | Neither appears outside error construction; string building goes through `textbuf.Buffer` or `strconv.Append*` |
| AC-11 | A buffered surface reads a converted command | It receives the same document `CollapseRecords` produced before this spec |
| AC-12 | `show bgp rib best` runs | It streams under the same lock rule as AC-5, with its pool-handle dereference safe outside the lock |
| AC-13 | A short line is written into a pooled buffer that last held a long one | The line sent is exactly as long as it was written, and carries none of the previous answer's bytes |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Dumps a full RIB over SSH | CLI → `showPipeline` generator → `writeExecRecords` → exec channel → rendering | `test-show-rib-streams` |
| 2 | Dumps a full RIB and pipes it to `first 10` | `ApplyPipesRecords` stops the generator inside the buffering window | `test-show-rib-first-stops-walk` |
| 3 | Dumps the RIB while the daemon is taking UPDATEs | RIB read walk concurrent with `handleReceived` | `test-show-rib-under-load` |
| 4 | Renders a streamed answer as a table | `tab` head → `zipRow` against the column names → table | `test-stream-answer-renders-table` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRecordRowAppendsInPlace` | `internal/component/plugin/dispatch_test.go` | a row appends, it does not return a slice | |
| `TestWriteAnswerZeroAllocPerRow` | `internal/component/plugin/dispatch_test.go` | `AllocsPerRun` over 1000 rows is zero (AC-1) | |
| `TestWriteAnswerUsesPooledBuffer` | `internal/component/plugin/dispatch_test.go` | the buffer is acquired from and returned to the pool | |
| `TestWriteAnswerReleasesBufferOnError` | `internal/component/plugin/dispatch_test.go` | every error path releases (R-1) | |
| `TestAnswerFrameUsesPooledBuffer` | `internal/component/ssh/answer_test.go` | the exec channel's frame is pooled too (AC-2) | |
| `TestPooledLineCarriesNoResidue` | `internal/component/plugin/dispatch_test.go` | a short line after a long one leaks nothing (AC-13, R-8) | |
| `TestStreamAnswerDeclaresFields` | `internal/component/plugin/dispatch_test.go` | `Fields` reaches the head and rows are positional (AC-6) | |
| `TestStreamRowArityMismatchIsFault` | `internal/component/plugin/dispatch_test.go` | a short positional row is a fault, not a silent shift | |
| `TestZipRowRoundTrip` | `pkg/plugin/rpc/answer_row_test.go` | names and values survive the positional round trip (R-6) | |
| `TestShowPipelineStreams` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | the RIB walk answers with a generator | |
| `TestShowPipelineNoLockAcrossWrite` | `internal/component/bgp/plugins/rib/rib_pipeline_test.go` | no write happens under `peerMu` (AC-5) | |
| `TestBestPipelineHandleSafeOutsideLock` | `internal/component/bgp/plugins/rib/rib_pipeline_best_test.go` | the pool-handle dereference is safe (AC-12, R-3) | |
| `BenchmarkRecordAnswerRows` | `internal/component/plugin/dispatch_test.go` | the benchmark the alloc gate reads | |
| `TestAllocGateCoversRecordPath` | `internal/perf/allocgate_test.go` | the new benchmark has a registered ceiling (AC-3) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| rows in a walk | 0 to unbounded | `AnswerBufferThreshold` selects the document form | N/A | one past it selects a streamed form |
| pooled buffer size | one line to `MaxMessageSize` | `MaxMessageSize` | N/A | a row past it is a fault, never a growth |
| positional row arity | 0 to the declared column count | the declared column count | one short is a fault | one long is a fault |
| allocations per row | 0 | 0 | N/A | 1 fails the gate |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-show-rib-streams` | `test/plugin/show-rib-streams.ci` | an operator dumps a large RIB | | <!-- doc-links: ignore (functional test this spec will create; the work is not implemented) -->
| `test-show-rib-first-stops-walk` | `test/plugin/show-rib-first-stops-walk.ci` | `first 10` bounds a million-row walk | | <!-- doc-links: ignore (functional test this spec will create; the work is not implemented) -->
| `test-show-rib-under-load` | `test/plugin/show-rib-under-load.ci` | a dump runs while UPDATEs arrive | | <!-- doc-links: ignore (functional test this spec will create; the work is not implemented) -->
| `test-stream-answer-renders-table` | `test/plugin/stream-answer-renders-table.ci` | a positional answer renders as a table | | <!-- doc-links: ignore (functional test this spec will create; the work is not implemented) -->

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `show-rib-under-frr-load` | `test/interop/scenarios/` | FRR | The RIB dump streams correctly while a real peer feeds routes. That is the concurrency AC-5 asserts, and a real peer is the only way to produce genuine `handleReceived` traffic under it | |

## Files to Modify
- `pkg/plugin/rpc/types.go` - the row type stops forcing an allocation
- `pkg/plugin/rpc/message.go` - the width measurement works against an appended row
- `internal/component/plugin/dispatch.go` - pooled line buffer, row append contract, and `ResponseJSON` stops copying into a string where a caller can take bytes
- `internal/component/plugin/types.go` - `Records` carries a schema its producers set, and rows that append
- `pkg/plugin/rpc/answer_row.go` - `zipRow` and `quoteFields` against a produced schema
- `internal/component/ssh/answer.go` - `answerFrame` takes its buffer from the pool
- `internal/component/plugin/server/system.go` - `commandRows` converts to the append contract, and its fault path stops marshaling a fresh map
- `internal/component/bgp/plugins/rib/rib_pipeline.go` - `showPipeline` answers with a generator, `serializeRouteItem` appends instead of building a map, and the lock stops spanning the drain
- `internal/component/bgp/plugins/rib/rib_pipeline_best.go` - `bestPipeline` likewise, with its pool-handle dereference made safe outside the lock
- `mk/alloc-gate.mk` - the benchmark glob widens beyond the reactor tree
- `internal/perf/allocgate.go` - a ceiling for the record-path benchmark
- `docs/architecture/api/ipc_protocol.md` - the `tab` item type gains a producer, so the "no producer today" paragraph is wrong
- `docs/architecture/api/commands.md` - the declared design of `internal/component/plugin/dispatch.go`
- `docs/architecture/api/process-protocol.md` - the declared design of `internal/component/plugin/types.go` and `answer_row.go`
- `docs/functional-tests.md` - the declared design of `internal/perf/allocgate.go`
- `docs/architecture/plugin/rib-storage-design.md` - the declared design of `rib_pipeline.go` and `rib_pipeline_best.go`: the read path becomes a generator and stops holding `peerMu` across the drain
- `docs/architecture/core-design.md` - the RIB read path no longer holds `peerMu` across a write

## Files to Create
- `test/plugin/show-rib-streams.ci` - a large RIB dump <!-- doc-links: ignore (functional test this spec will create; the work is not implemented) -->
- `test/plugin/show-rib-first-stops-walk.ci` - the pipe bounds the walk <!-- doc-links: ignore (functional test this spec will create; the work is not implemented) -->
- `test/plugin/show-rib-under-load.ci` - a dump under concurrent UPDATEs <!-- doc-links: ignore (functional test this spec will create; the work is not implemented) -->
- `test/plugin/stream-answer-renders-table.ci` - positional rows render as a table <!-- doc-links: ignore (functional test this spec will create; the work is not implemented) -->
- `test/interop/scenarios/show-rib-under-frr-load/check.py` - the interop check named above <!-- doc-links: ignore (functional test this spec will create; the work is not implemented) -->

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config leaf and no new RPC; how existing answers are built changes |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | N-A | No verb or flag changes |
| CLI grammar (keyword before value) | N-A | No grammar change |
| Editor autocomplete | N-A | No new leaf or dynamic value |
| Functional test for new RPC/API | Yes | The four new `.ci` files above |
| Pipe completeness | Yes | `internal/component/command/pipe_records.go` and `render_records.go` must render every pipe over a positional answer, `table` included |
| Env var registration | N-A | No new env var; the pool is sized in code, not configured |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module, or binary |
| Prometheus counters/metrics | No | Pool statistics are not exported by this spec; the alloc gate is the enforcement surface |
| BGP family surface (new SAFI / capability / attribute) | N-A | The RIB read path changes, not any family, capability or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The operator sees the same data from the same commands |
| 2 | Config syntax changed? | No | No config surface touched |
| 3 | CLI command added/changed? | No | No verb, flag, or rendering changes |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` -- the `tab` item type becomes reachable |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` -- a plugin row appends rather than marshals |
| 6 | Has a user guide page? | No | No operator-facing workflow changes |
| 7 | Wire format changed? | No | Child 2 fixed the frame; this spec changes only how bytes reach it |
| 8 | Plugin SDK/protocol changed? | Yes | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` -- the row contract |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC governs this path; see the RFC Documentation section |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` -- four `.ci` files, one interop scenario, and a widened alloc gate |
| 11 | Affects daemon comparison? | No | No externally visible capability changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` -- the RIB read path's lock span |
| 13 | Route metadata keys added/changed? | No | The keys are unchanged; only how they are written changes |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED, verified with `python3 scripts/dev/spec_doc_anchors.py plan/spec-record-answers-3-zero-alloc.md`. The declaring `// Design:` headers are named in Files to Modify above: `dispatch.go` declares `commands.md`; `types.go` and `answer_row.go` declare `process-protocol.md`; `ssh/answer.go` declares `ipc_protocol.md`; `allocgate.go` declares `docs/functional-tests.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/api/ipc_protocol.md` states `Records.Fields` has no producer and shows no `tab` line; both are corrected |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the measurement exist before the optimization
   - Tests: `BenchmarkRecordAnswerRows`, `TestAllocGateCoversRecordPath`
   - Files: `mk/alloc-gate.mk`, `internal/perf/allocgate.go`, `internal/component/plugin/dispatch_test.go`
   - Verify: the benchmark runs under `make ze-alloc-check`, reports a non-zero count today, and the gate is red at a ceiling of zero
2. **Phase: Pooled line buffer** -- the per-answer allocation, on both channels
   - Tests: `TestWriteAnswerUsesPooledBuffer`, `TestWriteAnswerReleasesBufferOnError`, `TestAnswerFrameUsesPooledBuffer`, `TestPooledLineCarriesNoResidue`
   - Files: `internal/component/plugin/dispatch.go`, `internal/component/ssh/answer.go`
   - Verify: every exit path releases, including the width rejection and the failed write, and no residue crosses answers
3. **Phase: Row append contract** -- the per-row allocation
   - Tests: `TestRecordRowAppendsInPlace`, `TestWriteAnswerZeroAllocPerRow`, `TestStreamRowArityMismatchIsFault`
   - Files: `pkg/plugin/rpc/types.go`, `pkg/plugin/rpc/message.go`, `internal/component/plugin/types.go`, `internal/component/plugin/dispatch.go`, `internal/component/plugin/server/system.go`
   - Verify: the alloc gate goes green at zero, and AC-9 costs no second build of the row
4. **Phase: `tab` item-type producer** -- the schema that was never declared
   - Tests: `TestStreamAnswerDeclaresFields`, `TestZipRowRoundTrip`, `test-stream-answer-renders-table`
   - Files: `internal/component/plugin/types.go`, `pkg/plugin/rpc/answer_row.go`, `internal/component/command/render_records.go`
   - Verify: a schema-declaring command streams positionally and a schema-less one stays self-describing (AC-6, AC-7)
5. **Phase: RIB walk conversion** -- the walk that makes this matter
   - Tests: `TestShowPipelineStreams`, `TestShowPipelineNoLockAcrossWrite`, `TestBestPipelineHandleSafeOutsideLock`, `test-show-rib-streams`, `test-show-rib-first-stops-walk`
   - Files: `internal/component/bgp/plugins/rib/rib_pipeline.go`, `internal/component/bgp/plugins/rib/rib_pipeline_best.go`
   - Verify: payloads byte-identical for a bounded walk (AC-8), and no write under `peerMu` (AC-5)
6. **Phase: Concurrency proof** -- the risk that costs the most if it is wrong
   - Tests: `test-show-rib-under-load`, `show-rib-under-frr-load`
   - Files: the interop scenario and its check script
   - Verify: race detector clean, and the interop test fails when the lock change is reverted

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Feature completeness | Both RIB walks converted, not one; both channels pooled, not one; the schema producer exists, not only its consumer |
| Correctness | Payload byte-identical for bounded walks; a fault still leaves the walk running; an arity mismatch is a fault |
| Naming | The row contract is named for what it does, not for its Go type; the pool is named for what it holds |
| Data flow | Rows are walked once. A buffered surface does not consume the rows a record surface would write |
| Rule: `ai/rules/performance.md` | No `fmt` and no string concatenation on the record path; one pool, one maximum size; the caller owns the buffer |
| Rule: `ai/rules/goroutine-lifecycle.md` | No goroutine per answer and none per row |
| Rule: `ai/rules/evidence.md` | The zero-allocation claim rests on the gate, not on a reading of the code |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero allocations per row | `make ze-alloc-check` green with the record-path ceiling at zero |
| The gate can see this path | The benchmark name appears in the `ze-alloc-check` output and in `AllocCeilings` |
| The line buffer is pooled | `grep -n "answerLineCapacity\|answerFrameCapacity" internal/` returns nothing, and the pool is named instead |
| The `tab` item type has a producer | `grep -rn "Fields:" --include=*.go internal/ \| grep -v _test` returns a `Records` literal |
| The RIB walks stream | `TestShowPipelineStreams` and `test-show-rib-streams` pass |
| No lock across a write | `TestShowPipelineNoLockAcrossWrite` passes, and fails when the lock change is reverted |
| No residue across answers | `TestPooledLineCarriesNoResidue` passes |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A positional row's arity is checked before it reaches the wire, because a consumer reading by position cannot tell a short row from a missing value |
| Resource exhaustion | A pooled buffer that is never released drains the pool. Every acquisition has a paired release on every path, asserted by a test |
| Memory disclosure | A pooled buffer carries the previous answer's bytes. Every write sets its own length, and no line is sent longer than it was written (`docs/contributing/ze-style.md`, zero the padding) |
| Use after free | The RIB walk dereferences pool handles another goroutine may release. The lock change must not turn a safe read into a freed one |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| Race detector failure on the RIB walk | Back to DESIGN: A-2 is broken, and the lock strategy needs rechoosing |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A row appends into the encoder's buffer | Keep `rpc.Record` and pool the marshaled slices | Pooling a slice per row still costs a pool round trip per row and keeps the marshal. Appending removes both, and it is the shape `ai/rules/performance.md` already requires of every callee |
| `AppendTo(buf) []byte` rather than `WriteTo(buf, off) int` | The packet-encoder shape used by the ISIS and VRRP encoders | `WriteTo` fits a fixed-layout packet whose size is known before it is written. A row's size is not known until it is built, and the append form already exists in the tree for exactly that case |
| The alloc gate's scope widens rather than a second gate being added | A separate plugin-path alloc target | One fail-closed gate with one ceiling registry is the surface a future reader finds. A second gate is a second thing to forget to run |
| The measurement lands before the optimization | Convert first and measure after | A benchmark written after the change tends to measure the change rather than the goal. Red first is the same discipline as a failing test |
| The RIB walks are converted and the other large walks are not | Convert all 384 payload handlers | The two RIB walks are the unbounded ones. The rest are bounded by peer count or interface count, so they answer as one document whatever this spec does |
| `Records.Fields` is declared by the handler | Derive it from `RegisterColumns` | The two registries are separate by owner directive: `RegisterColumns` orders columns for a person, and a program reading positional rows needs a schema its producer guarantees, not a rendering preference |

## Known Limitations
- Handlers outside the two RIB walks keep building their payloads with `plugin.Map`. They walk bounded collections, so they answer as one document and the per-row cost does not compound. A deferral row records the boundary.
- `table` and `text` rendering still buffers every row before printing, because a column width needs them all. Recorded as a permanent limit in `plan/deferrals/streaming-answer-protocol.md`.
- REST, gRPC, web, MCP and the looking glass still collapse to one document through `CollapseRecords`. Their record-level streaming has its own deferral row and its own spec.
- The pool is sized in code rather than configured. An operator has no knob, which is deliberate: a pool size an operator can set is a way to make the daemon slower.

## RFC Documentation (Scope: protocol)

No RFC governs the Ze plugin connection, the SSH exec answer channel, or the RIB
read path this spec converts. The BGP RIB itself is governed by RFC 4271, and
this spec changes no route selection, no attribute handling, and no wire
message: it changes only how an already-selected route is rendered into a
command answer. The one RFC-adjacent obligation is that a read walk must not
delay UPDATE processing, which AC-5 asserts and the interop scenario proves
against a real peer.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] `make ze-alloc-check` passes with the record-path ceiling at zero
- [ ] Feature code integrated (`internal/*`, `pkg/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-record-answers-zero-alloc.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-record-answers-3-zero-alloc.md` only
