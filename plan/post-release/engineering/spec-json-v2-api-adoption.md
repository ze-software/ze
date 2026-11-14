# Spec: json-v2-api-adoption

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P3 |
| Release reason | An optimization and hardening study that changes no shipped behavior by itself: the faster JSON engine already runs under today's calls, so nothing here blocks release correctness or proof. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Go's second JSON API (`encoding/json/v2`, with `encoding/json/jsontext`) is in
the toolchain Ze builds with. Ze's code calls the first API almost everywhere.
The question from the owner was whether converting would make Ze faster or
better.

This spec is an investigation that ends in a verdict, not a conversion. It
produces, for each of three candidates, the evidence and a decision to convert
or to leave. A candidate the evidence supports becomes a child spec of its
own, named here. This spec converts nothing itself.

| Candidate | What converting could buy | What decides it |
|-----------|---------------------------|-----------------|
| C-1: the external plugin RPC path | Fewer allocations where JSON sits nearest route volume: each filter exchange with a forked plugin marshals one batch of routes and unmarshals one batch of verdicts | An allocation benchmark of that exchange under the first API and under the second |
| C-2: decoding input from outside | A duplicate key or a field name in the wrong case is refused where today it is silently accepted | A census of every site that decodes outside input, and what each would start refusing |
| C-3: types with their own marshal methods, and large answers | No temporary buffer that is built and then parsed again; a large answer written as a stream | An allocation benchmark on a representative type and a representative large answer |

A blanket conversion is out of scope by owner decision (2026-11-14): the
framing chosen was "investigate, then decide".

### What was established on 2026-11-14

Each row says how it was learned, because a search is weaker evidence than a
read of the producing function. The session that designs this spec re-derives
every count, since the tree and the toolchain will both have moved.

| Finding | How it was established |
|---------|------------------------|
| `go.mod` asks for Go 1.27.3, and the installed toolchain was 1.27.4 | Read from `go.mod` and `go version` |
| That toolchain turns the JSON v2 experiment on by default | Read in the toolchain's default-experiment table |
| With it on, the first API is a compatibility layer over the second implementation: the Go tool compiled `encoding/json` from its v2-backed source files | `go list` over `encoding/json` |
| The shipped daemon build clears the experiment setting, which resets it to the toolchain default and so keeps v2 on | Search of the daemon build environment; the build's own output was not inspected |
| 1715 tracked Go files outside `vendor/` import the first API: 918 tests, 221 development tooling, 161 test harness, 415 product | Search over tracked files |
| No file imports `encoding/json/v2`. One imports `encoding/json/jsontext`: the route view output of the system RIB | Search over tracked files |
| In the 415 product files: 303 marshal calls, 33 indented marshal calls, 468 unmarshal calls, 14 decoders, 43 encoders, 443 uses of the raw message type | Search; calls were counted, not read |
| 77 types in non-test code declare their own `MarshalJSON` or `UnmarshalJSON` | Search |
| The per-UPDATE route path does not go through the package: a compiled-in plugin receives a structured event with no JSON, and the JSON a forked plugin receives is written by hand into buffers | `docs/architecture/core-design.md` (event dispatch) and a search of the formatter; the formatter was not read in full |
| One filter exchange with a plugin carries up to the schema's batch maximum of routes. It costs one marshal of the batch input and one unmarshal of the batch reply through the first API | Read: `filterBatcher.exchangeOne` and `PluginConn.CallRPC` in `internal/component/plugin/ipc` |
| A second, per-route marshal happens only on the size-risk path, when the schema's length bound cannot prove the batch fits the frame. It is used for its length alone and the bytes are discarded | Read: `filterBatcher.exchange` and `frameFits` in `internal/component/plugin/ipc` |
| No third-party JSON library is used by Ze's own code. The experimental JSON module in `go.mod` is an indirect dependency of the website browser tests | `go mod why` |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the outside parties that read Ze's JSON
  → Constraint: a change under `pkg/` is a change to a published contract, and the JSON event format and answer shapes are read by third-party plugins and API clients. An output difference between the two APIs is a contract change, made on purpose with its page, never as a side effect of a conversion.
- [ ] `ai/doors/plugin-infra.md` - the plugin RPC contract
  → Constraint: a field name in `pkg/plugin/rpc` is wire, not Go. A decoder that starts matching names case-sensitively refuses a plugin in another language that relied on the lenient match.
  → Constraint: a request line is framed by its newline, so a payload must stay compact single-line output under whichever API writes it.
  → Constraint: every feature works over both transports. A change measured on the forked path must leave the in-process bridge path answering the same.
- [ ] `ai/doors/api-surfaces.md` - the API clients
  → Constraint: REST, MCP and birdwatcher clients break when response JSON changes. The birdwatcher contract in `docs/architecture/api/birdwatcher-compat.md` is normative and has no upstream schema to fall back on.
- [ ] `docs/architecture/api/json-format.md` - Ze's JSON output format
  → Constraint: keys are lowercase kebab-case and declared by tag. Not yet read for what it says about empty lists and absent values, which is the output difference this study must settle.
- [ ] `docs/plugin-development/protocol.md` - the filter-update batch, the design page the batch encoder declares
  → Constraint: not yet read. It is the page a C-1 child spec must keep true.
- [ ] `ai/rules/performance.md` - allocation rules
  → Constraint: JSON marshaling is one of the places where allocating a result is accepted, so C-1 and C-3 are optimizations to be justified by a measurement, not defects to be fixed on sight.
- [ ] `ai/META-DECISIONS.md` - which figures may be reported from which host
  → Constraint: allocation figures (bytes and allocations per operation) may be measured, reported and compared on any host, provided the same Go toolchain and architecture produced both sides. Time and throughput figures come only from a host whose performance is known.

**Key insights:**
- The speed of the second implementation is already in the shipped binary. What is left to gain is what only the second API exposes: marshal methods that write to a stream, streaming of large answers, and stricter decoding.
- The second API's defaults differ from the first in what it accepts and in what it writes, so a conversion is a behavior change with outside readers, not an import swap.
- The route hot path is outside this study. The control plane (plugin RPC framing, config, CLI and API answers) is where the package sits.
- The frame-size proof on the filter path depends on a schema-owned bound of each route's encoded length. Any change to how a route is encoded must keep that bound true, or the proof that a batch fits its frame fails.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/ipc/filter_batch.go` - one worker per plugin connection takes the queued routes up to the batch maximum, sends them as one exchange, and decodes one reply holding one verdict per route
- [ ] `internal/component/plugin/ipc/filter_batch_bytes.go` - proves a batch fits the frame from a per-route length bound, and marshals each route separately only when that bound cannot prove it
- [ ] `internal/component/plugin/ipc/rpc.go` (the call entry only) - `CallRPC` routes to the bridge, then the multiplexed connection, then the plain connection; the bridge branch marshals the parameters itself

**Still to read at design, before anything else:**
- [ ] the multiplexed and plain connection writers in `pkg/plugin/rpc` - where the batch is actually marshalled for a forked plugin
- [ ] the request decode sites in `internal/component/api` and `internal/component/mcp` - input from API and MCP clients
- [ ] the one file that already uses the token-level package (the system RIB route view output) - the pattern Ze has for it today

**Behavior to preserve:**
- Every JSON answer and event Ze writes, byte for byte, unless a child spec changes one on purpose with its page and its consumers.
- Compact single-line payloads on the plugin RPC connection.
- Acceptance of every request a conforming plugin or API client sends today.
- The per-route encoded length bound that the filter batch frame proof relies on.

**Behavior to change:** None in this spec. It measures and decides.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| DirectBridge structured events | `docs/architecture/core-design.md`, event dispatch | A compiled-in plugin receives a structured event with no JSON round trip | Does not apply: it already removes JSON from the in-process route path, which is why that path is outside this study |
| Hand-written JSON into buffers | `docs/architecture/api/json-format.md`; `ai/rules/performance.md` | The JSON a forked plugin receives for a route event is written into a pooled buffer without the package | Does not apply: no JSON API is involved. Named so that C-1 is not mistaken for the event path |
| Batched filter exchange | `internal/component/plugin/ipc/filter_batch.go`, `filterBatcher` | Groups queued routes into one exchange of up to the schema's batch maximum, so the JSON cost is per batch and not per route | Extends, if C-1 is supported: the batch stays, and only how it is encoded and decoded would change in a child spec |
| Typed filter slot on the bridge | `ai/doors/plugin-infra.md`, "Every feature works over both transports" | A compiled-in filter is called through a typed slot, with the same evaluator a forked plugin reaches over the batched message | Does not apply to the measurement, but binds its result: both transports must still answer the same |
| Token-level JSON output | the system RIB route view output | The one place Ze already uses the token-level package | Extends, if C-3 is supported: the pattern a streaming answer would follow. To be read at design |

## Data Flow

### Entry Point
- C-1: a route waiting for a filter verdict enters the per-connection queue of a plugin. Format at entry: a typed route input, not JSON.
- C-2: a request line from a forked plugin, or a request body from a REST or MCP client. Format at entry: JSON bytes written by an outside party.
- C-3: an answer value handed to a writer for a CLI, API or plugin reader.

### Transformation Path
1. Filter exchange: queued routes are grouped into one batch, the batch is proven to fit the frame, marshalled once, sent, and one reply of verdicts is unmarshalled and handed back route by route.
2. Outside input and answers: to be traced at design, per surface.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ forked plugin | Newline-framed request line carrying compact JSON, width-framed answer | Read for the filter exchange only |
| Engine ↔ API client | JSON request and response bodies | No |

### Changed-Wire Census
No wire changes in this spec: it measures and decides, and converts nothing.
The census of writers and readers is AC-3 and AC-4, which are this spec's
output, and each child spec owes its own census for the wire it changes.

### Integration Points
- `filterBatcher.exchangeOne` - where a C-1 benchmark attaches: it owns the marshal of the batch and the decode of the reply
- `rpc.FilterUpdateInput.JSONSizeBound` - the per-route length bound the frame proof relies on, which any other encoder must keep true
- The decode sites of outside input and the answer writers - to be named at design, per surface

### Architectural Verification
To be filled at design. This spec adds no feature, so the registration rows
are expected to read as not applicable, with the reason.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The second API is stable enough to build on | Unknown. The toolchain still gates it behind an experiment setting that is on by default, which suggests it can still change | Converting anything ties Ze to an API that may change under it; the study ends at "wait" | Reading the Go release notes for the toolchain in use, and quoting the sentence | unvalidated |
| A-2 | The filter exchange with a forked plugin is the place the package sits nearest route volume | The exchange was read; no other RPC was compared with it | C-1 measures a site that is not the busiest | Reading the other per-route or per-event RPCs in `internal/component/plugin/ipc` | unvalidated |
| A-3 | The second API can be called with options that keep the first API's output and acceptance rules | Recalled from the Go design, not checked against this toolchain | The stream-writing marshal methods cannot be adopted without also changing output | Reading the package documentation of the toolchain in use | unvalidated |
| A-4 | The second API writes an empty list where the first writes null for a nil slice, matches field names case-sensitively, and refuses duplicate keys and invalid UTF-8 | Recalled from the Go design, not checked against this toolchain | The census of changed output and changed acceptance is drawn against the wrong list of differences | Reading the package documentation, then a test over one representative type | unvalidated |
| A-5 | Forked plugins form a meaningful share of deployments, so their path is worth optimizing | Not established | C-1 optimizes a path few users run | Owner statement at the design gate | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A conversion changes an answer an outside client reads (null becomes an empty list) | A functional test expectation or a birdwatcher fixture changes | Census every writer before any conversion; where output must not move, call the second API with the first API's output rules (A-3) |
| R-2 | Stricter decoding refuses a request a working third-party plugin sends | A plugin in another language fails at startup or on its first RPC | Census what each decode site would start refusing; decide per site, and document the refusal with its fix |
| R-3 | The benchmark compares two toolchains or two architectures | The two figures were produced in different runs on different hosts | Both sides of every comparison come from one run on one host |
| R-4 | The study is read as licence for a blanket conversion | A child spec's file list grows past its candidate | Each child spec owns one candidate; this spec names them |
| R-5 | A different encoder writes a route longer than its schema-owned length bound, so a batch proven to fit overflows the frame | The oversized-batch tests of the filter path go red | Any C-1 child spec re-proves the bound against the encoder it introduces |

## Acceptance Criteria

Draft, to be settled at the design gate.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The Go release notes for the toolchain `go.mod` names | The spec records, with the quoted sentence, whether the second API is covered by Go's compatibility promise | None: the answer gates every candidate |
| AC-2 | One filter exchange of a full batch with a forked plugin, encoded and decoded under the first API and under the second | A committed benchmark reports bytes and allocations per exchange for both, from one run | The first API stays in place on this path unless the child spec for C-1 is opened |
| AC-3 | Every site that decodes input from a forked plugin, a REST client or an MCP client | The spec lists each site with what it would start refusing under the second API's defaults, and a verdict per site | Sites that decode Ze's own output stay lenient: no outside party writes that input |
| AC-4 | Every writer whose output differs between the two APIs | The spec lists each writer, the difference, and the outside reader it reaches | Writers whose output is identical under both are listed as unaffected, with the check that showed it |
| AC-5 | The evidence of AC-1 to AC-4 | Each candidate carries a verdict, convert or leave, with its reason; each "convert" names a child spec by path | A "leave" verdict keeps the first API there, which is correct when the measurement shows no gain |

## Wiring Test

This spec adds no feature a user reaches, so there is no entry point to wire.
What it must prove reachable is that each benchmark exercises the real
exchange and not a copy of it.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A filter verdict requested for a route on a plugin connection | → | The batched filter exchange and its reply decode | `BenchmarkFilterExchangeJSON`, driving the real exchange against a stub plugin end |

## 🧪 TDD Test Plan

Draft. The benchmarks are the deliverable of this spec; names are settled at
design.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `BenchmarkFilterExchangeJSON` | `internal/component/plugin/ipc/filter_batch_bench_test.go` | Bytes and allocations for one full-batch exchange, first API against second, in one run | |
| `TestJSONAPIDifferences` | `internal/component/plugin/ipc/json_api_differences_test.go` | Each difference A-4 lists is real on the toolchain in use, on one representative wire type | |

### Functional Tests
None: no user-reachable behavior changes. A child spec that converts a path
owes the functional tests for it.

## Files to Modify

None in product code. This spec adds benchmarks and records verdicts.

## Files to Create
- `internal/component/plugin/ipc/filter_batch_bench_test.go` - the C-1 allocation benchmark
- `internal/component/plugin/ipc/json_api_differences_test.go` - the proof of the differences the censuses are drawn against
- the C-3 benchmark, placed beside the representative type chosen at design

## Implementation Steps

Outline only; the design phase replaces it.

1. Settle A-1, A-3 and A-4 from the toolchain's release notes and package documentation. If the API is not stable, record the verdict "wait" with the evidence and close.
2. Read the producers named under Current Behavior and settle A-2.
3. Write the allocation benchmarks for C-1 and C-3 and record their figures.
4. Write the two censuses: decode sites of outside input, and writers whose output would change.
5. Record a verdict per candidate and open a child spec for each "convert".

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Investigate, then decide; conversions go to child specs | One spec that measures and converts; a spec limited to the C-1 measurement | Owner decision, 2026-11-14. The size of a conversion is unknown until the measurement is in, and C-1 (performance) and C-2 (input validation) are different kinds of change |
| Saved as a skeleton carrying the findings | Full design to `ready` now | Owner decision, 2026-11-14. Go and the RPC path will both have moved by the time post-release work starts, so the design is made against the code as it is then |

## Known Limitations

- The route hot path is not studied: it does not use the package.
- Tests and development tooling (1300 of the 1715 files) are not candidates. They follow whatever the product code they exercise does.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: `./le test unit pkg` over the packages that hold the new benchmarks and tests, named at design
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every candidate with a "convert" verdict is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior (N-A here: no user-reachable behavior changes)
- [ ] Interop tests for protocol features (N-A here: no wire changes)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean and recorded
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** benchmarks + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove` of this spec's path only, in the same `./le commit create` script

## Not yet written

This is a skeleton. The design phase adds the remaining sections of
`plan/templates/TEMPLATE.md`: Blast Radius, the Integration and Documentation
checklists, and the review checklists. It also completes Data Flow for C-2
and C-3, which this skeleton traces for the filter exchange only.
