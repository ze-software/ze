# Spec: record answers child 1 -- the SDK produces and reads them

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/record-answers.md` |
| Handoff | - |
| Updated | 2026-08-21 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Child 1 of three. Child 2 (`spec-record-answers-2-only-encoding`) deletes the
single-line command-answer frame and fixes the line widths; child 3
(`spec-record-answers-3-zero-alloc`) makes the record path allocation-free.
Both depend on this one, because neither can delete a frame the SDK cannot
speak.

## Task

The record answer sequence exists on the engine side and no plugin can use it.

`Plugin.Run` builds its Stage 3 `DeclareCapabilitiesInput` without ever setting
`Protocol`, so no SDK plugin declares `record-answers` and every plugin answer
takes the single-line branch of `answerResult`. On the read side
`MuxConn.CallAnswer` exists with no non-test caller, and `callEngineWithResult`
returns one `json.RawMessage`, so an SDK plugin has no way to consume a record
answer even if one arrived.

The reverse direction is worse. A plugin's own command answer travels back as an
RPC result inside `ExecuteCommandOutput.Data`, a `json.RawMessage` the plugin
marshaled whole in `sdk_callbacks.go` and the engine re-wraps as
`plugin.RawJSON` in `routeToProcess`. Every large walk a plugin owns is
materialized twice, and no protocol name covers the engine-to-plugin direction:
`DeclareCapabilitiesInput` travels plugin-to-engine only, so the engine has no
way to say it reads answers.

The goal is that a plugin can produce a record answer and read one, in both
directions, so the frame stops being dead code.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/ipc_protocol.md` - the Answer Protocol grammar and its negotiation
  → Constraint: a later shape earns a NEW name in `protocol`; `ParseAnswerTail` refuses an unknown key, so a key added under an agreed name makes the line unreadable to the peer that agreed to the older spelling
  → Decision: only `dispatch-command` and `dispatch-command-args` take the answer path; every other engine op returns its own typed output and keeps the single-line frame
- [ ] `docs/architecture/api/process-protocol.md` - the five-stage startup and where `declare-capabilities` sits
  → Constraint: Stage 3 is the barrier; an answer written before it must use the shape a peer already understands, which is why Stage 3 itself is answered single-line whatever the peer declares
- [ ] `ai/rules/plugins.md` - plugin boundaries and command surface
  → Constraint: no plugin spelling in generic or central packages; a record producer must be a general SDK facility, not a per-plugin field
- [ ] `ai/rules/performance.md` - buffer ownership and pool lifecycle
  → Constraint: the caller owns the buffer and the callee writes into `buf[off:]`; a row API that returns a fresh `[]byte` for each row cannot be made allocation-free later, so child 3 is only reachable if this spec's row type does not force the allocation

**Key insights:** (minimal context to resume after compaction)
- The engine already branches on `Process.RecordAnswers()` inside `answerResult`; nothing engine-side is missing for the plugin-to-engine direction except a peer that declares the name.
- `rpc.Record` holds two `json.RawMessage` fields, so the type itself forces one allocation per row. A record API that keeps that shape caps child 3 before it starts.
- `Conn.AnswerWriter` serves an INBOUND request only. `execute-command` is engine-to-plugin, so the plugin's rows are an RPC result, and no answer writer exists for a result.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `pkg/plugin/sdk/sdk.go` - `Plugin.Run` sends Stage 3 `DeclareCapabilitiesInput` with `Capabilities` only; `callEngine`, `callEngineRaw` and `callEngineWithResult` each return one `json.RawMessage`
- [ ] `pkg/plugin/sdk/sdk_engine.go` - `DispatchCommand`, `DispatchCommandArgs` and `dispatchCommandResult` carry `(status string, data json.RawMessage, err error)`; `dispatchCommandRPC` unmarshals the whole result into `*rpc.DispatchCommandOutput`
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` - the plugin marshals its own command answer whole before it replies to `execute-command`
- [ ] `pkg/plugin/sdk/sdk_types.go` - `Registration` carries no field naming a protocol shape
- [ ] `pkg/plugin/plugin.go` - `CommandHandler` is `func(ctx *CommandContext) (any, error)`, so a handler has no way to answer with a generator
- [ ] `pkg/plugin/rpc/mux.go` - `MuxConn.CallAnswer` reads a full answer sequence and has no non-test caller; `interpretResponse` reads the payload after the verb, so a peer that does not know the shape takes a head line's tail as its result
- [ ] `pkg/plugin/rpc/types.go` - `ProtocolRecordAnswers` states no direction; `DeclareCapabilitiesInput` travels plugin to engine only; `ExecuteCommandOutput.Data` is a `json.RawMessage`; `Record` is two `json.RawMessage` fields
- [ ] `pkg/plugin/rpc/conn.go` - `Conn.AnswerWriter` exists for an inbound request, not for a result
- [ ] `internal/component/plugin/server/startup.go` - Stage 3 sets `Process.SetRecordAnswers` from `input.Understands`
- [ ] `internal/component/plugin/process/process.go` - `Process.RecordAnswers` is the single read of what the peer declared
- [ ] `internal/component/plugin/server/dispatch_registry.go` - `answerResult` selects the record sequence or the single-line result; `opDispatchCommand` and `opDispatchCommandArgs` are its only callers
- [ ] `internal/component/plugin/server/command.go` - `routeToProcess` wraps `rpcOut.Data` as `plugin.RawJSON`
- [ ] `internal/component/plugin/ipc/rpc.go` - `SendExecuteCommand` unmarshals a whole `ExecuteCommandOutput`
- [ ] `internal/component/plugin/server/system.go` - `handleSystemCommandList` and `commandRows` are the only row generator in the tree, and the shape a second producer copies
- [ ] `internal/component/plugin/types.go` - `Records` carries `Key`, `Fields` and `Rows iter.Seq[rpc.Record]`; `MarshalJSON` collapses through `CollapseRecords`

**Behavior to preserve:**
- The Stage 3 barrier: `declare-capabilities` is itself answered in the shape the peer already understands, whatever it declares in that same message.
- `answerResult`'s branch stays until child 2 removes it. This spec adds a peer that declares the name; it does not change what the engine does with the declaration.
- Buffered surfaces keep reading one document. `Records.MarshalJSON` and `CollapseRecords` are untouched.
- Every engine op other than `dispatch-command` and `dispatch-command-args` keeps its typed single-line output.
- `answerErrorsKey` stays reserved: a handler naming `errors` as its envelope is refused by both producers.

**Behavior to change:**
- The SDK declares `record-answers` at Stage 3 and reads record answers through `CallAnswer`.
- `pkg/plugin` gains a record producer so a plugin command handler can answer with rows.
- A plugin's own command answer reaches the engine as records rather than as one `ExecuteCommandOutput.Data` value.
- `ProtocolRecordAnswers` gains a stated direction, or a second name covers engine to plugin.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator command reaching `dispatch-command` or `dispatch-command-args` over the plugin connection, and an engine-to-plugin `execute-command` callback for a command the plugin owns.
- At entry the line is `#<id> <method> <json>`, newline framed, the params a JSON object.

### Transformation Path
1. Stage 3 `declare-capabilities` carries `protocol: ["record-answers"]` from the SDK; `startup.go` stores it through `Process.SetRecordAnswers`.
2. A plugin command handler returns a row generator instead of a built value; the SDK holds it as the response payload rather than marshaling it.
3. For a plugin-owned command the engine calls `execute-command`; the plugin writes head, records and terminator back as the result frame rather than one `Data` value.
4. `SendExecuteCommand` reads the sequence and hands the engine a generator, so `routeToProcess` builds a `Records` payload rather than a `RawJSON` one.
5. For an engine-owned command the plugin calls `dispatch-command`; `answerResult` sees `RecordAnswers()` true and `WriteAnswer` writes the sequence.
6. The SDK reads it with `CallAnswer` and yields rows to the plugin as they arrive, ending on the terminator's counts.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine → Plugin (`execute-command`) | result frame becomes a record sequence; needs a name covering this direction | No |
| Plugin → Engine (`dispatch-command`) | existing record sequence, now with a peer that declares it | No |
| SDK → plugin author | new record producer type in `pkg/plugin`, new answer-returning dispatch call | No |
| Direct bridge (in-process plugins) | `DirectBridge.DispatchCommand` returns a typed output and has no answer path | No |

### Integration Points
- `answerResult` (`internal/component/plugin/server/dispatch_registry.go`) - already branches; this spec supplies the peer that takes the record branch.
- `WriteAnswer` (`internal/component/plugin/dispatch.go`) - the single writer of the sequence, unchanged by this spec.
- `commandRows` (`internal/component/plugin/server/system.go`) - the row-generator shape a plugin-side producer mirrors.
- `bufpool.Pool` (`internal/core/bufpool`) - not used here; named so child 3 has a stated starting point.

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
| A-1 | No compatibility shim is owed to any out-of-tree plugin | `ai/rules/go-standards.md` states Ze has never been released and permits no compat code anywhere, the plugin API included | The single-line frame would have to survive beside the record one, which child 2 forbids | Owner confirmation, recorded here | unvalidated |
| A-2 | `ProtocolRecordAnswers` can be read as covering both directions | `pkg/plugin/rpc/types.go` states no direction on the constant | A second protocol name is needed for engine to plugin, and Stage 3 cannot carry it because `DeclareCapabilitiesInput` is one-way | Unit test asserting the engine writes an `execute-command` record result only for a peer that declared it | unvalidated |
| A-3 | `rpc.Record` can gain a byte-slice form without breaking its readers | `Record` is two `json.RawMessage` fields read by `writeRecordLine` and `AnswerRecordLineSize` | Child 3 cannot reach zero allocation, because the row type forces one allocation per row | Compile plus `AllocsPerRun` in child 3 | unvalidated |
| A-4 | `DirectBridge` can carry a record answer in process | `DirectBridge.DispatchCommand` returns a typed output today with no answer path | Internal plugins keep the buffered path while external ones stream, so one answer has two shapes by transport | Unit test over the bridge asserting the same row sequence as the socket path | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A partially converted SDK declares `record-answers` before it can read one, so every answer is misread as a head payload | `interpretResponse` returns a tail string where a JSON object was expected; plugin unmarshal errors at startup | Declare the name in the same phase that wires `CallAnswer`, never earlier; the wiring test asserts both in one run |
| R-2 | The engine-to-plugin direction needs a second name and Stage 3 cannot carry it | Design review finds no message in which the engine states what it reads | Carry the engine's declaration in the Stage 1 or Stage 2 message the engine already sends, or make the result frame self-describing so the reader needs no declaration |
| R-3 | A row generator that outlives its handler reads state the plugin has already released | Race detector failure, or rows carrying zero values under load | State the generator's lifetime in the SDK doc comment: it is walked before the handler's caller returns, and never stored |
| R-4 | Two answer shapes exist in the SDK during the conversion, and a plugin picks the wrong one | A plugin compiles against both the value-returning and the answer-returning dispatch call | The answer-returning call replaces the old one rather than joining it (`ai/rules/no-layering.md`) |
| R-5 | The bridge path and the socket path diverge, so an internal plugin answers differently from an external one | A `.ci` passing over the socket and failing in process, or the reverse | One test table drives both transports, as `wireBridgeDispatch` already does for dispatch |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every plugin command answer. A plugin that declares the shape and misreads it loses the answer to every command an operator runs against it, and a startup declaration error breaks the plugin at Stage 3, before it serves anything |
| How is it reverted? | Single commit revert. Nothing persists, no config migrates, and no peer outside the process tree sees the frame |
| Who else touches this path? | Child 2 and child 3 of this family, and any spec touching `dispatch_registry.go` or the SDK's engine calls |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Plugin startup Stage 3 | → | `Plugin.Run` sets `Protocol` on `DeclareCapabilitiesInput` | `TestPluginRunDeclaresRecordAnswers` |
| Plugin calls a streaming engine command | → | SDK answer-returning dispatch over `MuxConn.CallAnswer` | `TestDispatchCommandAnswerYieldsRows` |
| Operator runs a plugin-owned command with a long walk | → | plugin record producer → `SendExecuteCommand` → `routeToProcess` builds `Records` | `test-plugin-owned-command-streams` |
| Internal plugin over `DirectBridge` | → | bridge answer path | `TestDirectBridgeDispatchCommandAnswer` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An SDK plugin completes Stage 3 | Its `declare-capabilities` names `record-answers` in `protocol`, and the engine's `Process.RecordAnswers` reads true for that peer |
| AC-2 | An SDK plugin dispatches an engine command whose walk produces more rows than `AnswerBufferThreshold` | The plugin receives every row in walk order and the terminator's `count`, and at no point holds more than one row plus the read buffer |
| AC-3 | An SDK plugin dispatches an engine command whose walk ends at or under the threshold | The plugin receives one document, and the value it sees equals what the same command produced before this spec |
| AC-4 | A plugin command handler answers with a row generator and an operator runs that command | The rows reach the operator as records, and the engine never holds the whole collection |
| AC-5 | A plugin command handler answers with a built value | The answer is unchanged from today, byte for byte |
| AC-6 | A row generator yields a row that is wider than one wire message | That row is reported as a fault and the walk continues, and the terminator counts it under `faults` |
| AC-7 | The same plugin command is served over the socket and over `DirectBridge` | Both produce the same row sequence and the same terminator counts |
| AC-8 | A plugin declares `record-answers` and the engine sends `execute-command` | The engine reads the plugin's answer as a record sequence, and a plugin that declared nothing gets the value frame it gets today |
| AC-9 | A plugin names `errors` as its record envelope | The answer is refused with the reserved-envelope error, on both the socket and the bridge |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs a plugin-owned command that walks a large collection | CLI → dispatcher → `execute-command` → plugin row generator → record result → `routeToProcess` → operator | `test-plugin-owned-command-streams` |
| 2 | Runs an engine command from inside a plugin | plugin → `dispatch-command` → `answerResult` → `WriteAnswer` → SDK `CallAnswer` → plugin rows | `test-plugin-reads-engine-record-answer` |
| 3 | Runs a plugin command that fails half way through its walk | plugin generator faults one row → `bad` record → terminator faults count → operator sees applied and rejected side by side | `test-plugin-command-partial-fault` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPluginRunDeclaresRecordAnswers` | `pkg/plugin/sdk/sdk_test.go` | Stage 3 carries the protocol name | |
| `TestDispatchCommandAnswerYieldsRows` | `pkg/plugin/sdk/sdk_engine_test.go` | rows arrive in walk order, terminator ends the walk | |
| `TestDispatchCommandAnswerBoundedIsDocument` | `pkg/plugin/sdk/sdk_engine_test.go` | a short walk still reads as one document | |
| `TestCallAnswerStopsGeneratorOnConsumerStop` | `pkg/plugin/rpc/mux_test.go` | a consumer that stops reading stops the walk | |
| `TestExecuteCommandRecordResult` | `pkg/plugin/sdk/sdk_callbacks_test.go` | a plugin handler's generator becomes a record result | |
| `TestSendExecuteCommandReadsRecords` | `internal/component/plugin/ipc/rpc_test.go` | the engine reads the plugin's record result | |
| `TestRouteToProcessBuildsRecords` | `internal/component/plugin/server/command_test.go` | a streamed plugin answer becomes a `Records` payload, not `RawJSON` | |
| `TestDirectBridgeDispatchCommandAnswer` | `internal/component/plugin/bridge_test.go` | bridge and socket agree row for row | |
| `TestPluginRecordEnvelopeErrorsRefused` | `pkg/plugin/sdk/sdk_test.go` | the reserved envelope name is refused | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| rows produced before the type is decided | 0 to `AnswerBufferThreshold` | 256 | N/A | 257 selects a streamed type |
| record line width | 0 to `MaxMessageSize` | `MaxMessageSize` | N/A | one byte over is reported as a fault |
| `count` in the terminator | 0 to max uint64 | rows produced | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-plugin-owned-command-streams` | `test/plugin/plugin-owned-command-streams.ci` | an operator runs a plugin command with a long walk and sees every row | |
| `test-plugin-reads-engine-record-answer` | `test/plugin/plugin-reads-engine-answer.ci` | a plugin reads a streamed engine answer and acts on it | |
| `test-plugin-command-partial-fault` | `test/plugin/plugin-command-partial-fault.ci` | applied rows and rejected rows both reach the operator | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | The plugin connection is internal to Ze. No other implementation speaks it, so no peer daemon can validate it. The `.ci` tests above drive the real daemon and the real SDK, which is the strongest evidence this surface admits | N/A |

## Files to Modify
- `pkg/plugin/sdk/sdk.go` - declare `record-answers` at Stage 3; add the answer-returning engine call beside `callEngineWithResult`
- `pkg/plugin/sdk/sdk_engine.go` - `DispatchCommand` and `DispatchCommandArgs` return an answer rather than a whole payload
- `pkg/plugin/sdk/sdk_callbacks.go` - a command handler's row generator becomes a record result
- `pkg/plugin/sdk/sdk_types.go` - the registration surface for a record-producing handler
- `pkg/plugin/plugin.go` - the handler contract gains a record answer form
- `pkg/plugin/rpc/types.go` - state the direction `ProtocolRecordAnswers` covers; the record result shape for `execute-command`
- `pkg/plugin/rpc/mux.go` - `CallAnswer` gains its first non-test caller
- `pkg/plugin/rpc/conn.go` - an answer writer for a result, not only for an inbound request
- `internal/component/plugin/ipc/rpc.go` - `SendExecuteCommand` reads a record result
- `internal/component/plugin/server/command.go` - `routeToProcess` builds a `Records` payload from a streamed plugin answer
- `internal/component/plugin/bridge.go` - the direct bridge carries a record answer in process
- `docs/architecture/api/ipc_protocol.md` - the direction each protocol name covers, and the plugin-side producer
- `docs/architecture/api/process-protocol.md` - Stage 3 now declares the name from the SDK

## Files to Create
- `pkg/plugin/records.go` - the SDK record producer a plugin command handler answers with
- `test/plugin/plugin-owned-command-streams.ci` - operator runs a plugin command with a long walk
- `test/plugin/plugin-reads-engine-answer.ci` - plugin consumes a streamed engine answer
- `test/plugin/plugin-command-partial-fault.ci` - applied and rejected rows both arrive

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config leaf and no new YANG RPC: the change is the answer frame of two existing engine ops |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | N-A | No new verb. Existing commands answer the same data in a different frame |
| CLI grammar (keyword before value) | N-A | No new grammar |
| Editor autocomplete | N-A | No new leaf or dynamic value |
| Functional test for new RPC/API | Yes | `test/plugin/plugin-owned-command-streams.ci`, `test/plugin/plugin-reads-engine-answer.ci`, `test/plugin/plugin-command-partial-fault.ci` |
| Pipe completeness | Yes | Records already route through `ApplyPipesRecords` (`internal/component/command/pipe_records.go`); the new producer must not bypass it |
| Env var registration | N-A | `ZE_ANSWER_PROTOCOL` already exists (`AnswerProtocolEnv`) and is not a YANG-backed `ze.*` leaf |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module, or binary. The connection already exists |
| Prometheus counters/metrics | No | No new observable state; the terminator's counts already report what a walk produced |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP protocol work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The operator sees the same data; only the frame changes |
| 2 | Config syntax changed? | No | No config surface touched |
| 3 | CLI command added/changed? | No | No verb, flag, or output format changes |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` -- the `execute-command` result gains a record form |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` -- a plugin command handler can answer with rows |
| 6 | Has a user guide page? | No | The plugin author surface is the SDK doc, covered by row 5 |
| 7 | Wire format changed? | Yes | `docs/architecture/api/wire-format.md` -- the result frame of `execute-command` |
| 8 | Plugin SDK/protocol changed? | Yes | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC governs the plugin connection |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` -- three new `.ci` tests under `test/plugin/` |
| 11 | Affects daemon comparison? | No | No externally visible capability changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/ipc_protocol.md` -- the direction each protocol name covers |
| 13 | Route metadata keys added/changed? | No | No route metadata touched |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No new registration; the protocol name already exists in the inventory |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `python3 scripts/dev/spec_doc_anchors.py plan/spec-record-answers-1-sdk-path.md`. Known declaring headers: `pkg/plugin/rpc/types.go` and `pkg/plugin/rpc/mux.go` declare `ipc_protocol.md`; `internal/component/plugin/types.go` declares `process-protocol.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/api/ipc_protocol.md` carries the `declare-capabilities` example; it must show the SDK sending the name |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the shape reachable and prove it is not yet served
   - Tests: `TestPluginRunDeclaresRecordAnswers`, `TestDispatchCommandAnswerYieldsRows`, `TestDirectBridgeDispatchCommandAnswer`
   - Files: `pkg/plugin/sdk/sdk.go`, `pkg/plugin/records.go`, `internal/component/plugin/bridge.go`
   - Verify: the declaration reaches `Process.RecordAnswers` and the answer-returning call exists as a stub, so the wiring tests fail on behavior rather than on a missing symbol
2. **Phase: Read side** -- the SDK consumes a record answer
   - Tests: `TestDispatchCommandAnswerYieldsRows`, `TestDispatchCommandAnswerBoundedIsDocument`, `TestCallAnswerStopsGeneratorOnConsumerStop`
   - Files: `pkg/plugin/sdk/sdk_engine.go`, `pkg/plugin/rpc/mux.go`
   - Verify: declaring the name and reading the answer land in this phase together, so R-1 cannot occur between phases
3. **Phase: Produce side** -- a plugin command handler answers with rows
   - Tests: `TestExecuteCommandRecordResult`, `TestPluginRecordEnvelopeErrorsRefused`
   - Files: `pkg/plugin/plugin.go`, `pkg/plugin/records.go`, `pkg/plugin/sdk/sdk_callbacks.go`, `pkg/plugin/sdk/sdk_types.go`
   - Verify: a handler returning a built value is byte-identical to today (AC-5)
4. **Phase: Engine reads the plugin's rows** -- close the engine-to-plugin direction
   - Tests: `TestSendExecuteCommandReadsRecords`, `TestRouteToProcessBuildsRecords`
   - Files: `pkg/plugin/rpc/types.go`, `pkg/plugin/rpc/conn.go`, `internal/component/plugin/ipc/rpc.go`, `internal/component/plugin/server/command.go`
   - Verify: A-2 resolves here. Either the existing name is stated as symmetric or a second name exists, and the test names which
5. **Phase: Functional and docs** -- prove it from the operator's seat
   - Tests: `test-plugin-owned-command-streams`, `test-plugin-reads-engine-record-answer`, `test-plugin-command-partial-fault`
   - Files: the three `.ci` files, plus every doc row answered Yes above
   - Verify: `make ze-functional-plugin-test` passes, and each `.ci` fails when the producer is reverted

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Feature completeness | Both directions work: plugin reads an engine answer, and the engine reads a plugin answer |
| Correctness | A bounded walk is byte-identical to today's answer (AC-3, AC-5); a fault does not end a walk (AC-6) |
| Naming | The protocol name states its direction; the SDK record type is named for what it is, not for its Go type |
| Data flow | The row generator is walked once. A surface that flattens it does not also stream it |
| Rule: `ai/rules/no-layering.md` | The answer-returning dispatch call REPLACES the value-returning one. Both must not exist |
| Rule: `ai/rules/performance.md` | The SDK row type does not force one allocation per row, or child 3 cannot reach its goal |
| Rule: `ai/rules/goroutine-lifecycle.md` | No goroutine is started per answer or per row |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The SDK declares the protocol name | `grep -n "ProtocolRecordAnswers" pkg/plugin/sdk/` returns a non-test producer |
| `CallAnswer` has a non-test caller | `gopls references` on `CallAnswer` shows a caller outside `_test.go` |
| A plugin can answer with rows | `pkg/plugin/records.go` exists and a `.ci` exercises it |
| The engine reads a plugin's record result | `TestRouteToProcessBuildsRecords` passes and asserts a `Records` payload |
| Both transports agree | `TestDirectBridgeDispatchCommandAnswer` compares bridge and socket row for row |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A record line from a plugin is untrusted: its width is checked before it is written, and a row that is not valid JSON is refused rather than forwarded |
| Resource exhaustion | A plugin that never sends a terminator must not hold engine memory without bound; the read side carries a bounded queue and the answer ends on connection loss |
| Error leakage | A fault payload from a plugin reaches an operator; it must not carry an internal path or a raw Go error string |
| Authorization | Command dispatch authorization is unchanged. A record frame must not become a route around the existing check in the dispatcher |

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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The SDK gains a record producer before the single-line frame is deleted | Delete first and convert the SDK afterwards | A deletion that lands first leaves every plugin unable to answer. The order is forced by what breaks in between |
| The answer-returning dispatch call replaces the value-returning one | Keep both and let a plugin choose | Two shapes for one answer is the layering this repository forbids, and a plugin choosing between them is a decision nobody needs to make |
| A bounded walk keeps producing one document | Always stream, whatever the count | The encoder already decides from the record count, and changing that decision here would change every existing command's payload for no gain |

## Known Limitations
- Record-level streaming for REST, gRPC, web, MCP and the looking glass stays out of scope. Those surfaces collapse through `CollapseRecords` and are unaffected. The row is already recorded in `plan/deferrals/streaming-answer-protocol.md`.
- `table` and `text` rendering buffers whatever the wire does, because a column width needs every row. Recorded as a permanent limit in the same shard.
- The `tab` item type still has no producer after this spec. It belongs to child 3, with the column schema it needs.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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
- [ ] Learned summary written to `plan/learned/NNN-record-answers-sdk-path.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-record-answers-1-sdk-path.md` only
