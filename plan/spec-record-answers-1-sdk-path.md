# Spec: record answers child 1 -- the SDK produces and reads them

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 5/5 |
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
- Buffered surfaces keep reading one document. `Records.MarshalJSON` and the collapse behave exactly as they did.
  **Corrected in phase 2:** the collapse MOVED. The SDK must rebuild a document from an arriving answer, and `pkg/plugin/sdk` cannot import `internal/component/plugin`, so `CollapseRecords` and the positional-row machinery now live in `pkg/plugin/rpc` (`collapse.go`, `answer_row.go`) and `internal/component/plugin.CollapseRecords` is deleted. One collapse still serves every consumer, which is what the row was protecting; a second implementation in the SDK is what `ai/rules/no-layering.md` refuses.
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
| Engine → Plugin (`execute-command`) | result frame becomes a record sequence; the existing name covers this direction (A-2) | Yes -- `TestSendExecuteCommandReadsRecords`, `TestRouteToProcessBuildsRecords` |
| Plugin → Engine (`dispatch-command`) | existing record sequence, now with a peer that declares it | Yes -- `TestDispatchCommandAnswerYieldsRows`, `test/plugin/plugin-reads-engine-answer.ci` |
| SDK → plugin author | new record producer type in `pkg/plugin`, new answer-returning dispatch call | Yes -- `TestExecuteCommandRecordResult`, `test/plugin/plugin-owned-command-streams.ci` |
| Direct bridge (in-process plugins) | `DirectBridge.DispatchCommandAnswer` beside it, served by the `dispatch-command` entry's typed answer slot | Yes -- `TestDirectBridgeDispatchCommandAnswer`, `TestWireBridgeDispatchInstallsTypedSlots` |

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
| A-1 | No compatibility shim is owed to any out-of-tree plugin | `ai/rules/go-standards.md` states Ze has never been released and permits no compat code anywhere, the plugin API included | The single-line frame would have to survive beside the record one, which child 2 forbids | Owner confirmation, recorded here | confirmed |
| A-2 | `ProtocolRecordAnswers` can be read as covering both directions | `pkg/plugin/rpc/types.go` states no direction on the constant | A second protocol name is needed for engine to plugin, and Stage 3 cannot carry it because `DeclareCapabilitiesInput` is one-way | `TestSendExecuteCommandReadsRecords` (`internal/component/plugin/ipc/rpc_test.go`): the engine reads an `execute-command` record result for a peer that declared, and the value frame for one that declared nothing | confirmed |
| A-3 | `rpc.Record` can gain a byte-slice form without breaking its readers | `Record` is two `json.RawMessage` fields read by `writeRecordLine` and `AnswerRecordLineSize` | Child 3 cannot reach zero allocation, because the row type forces one allocation per row | Compile plus `AllocsPerRun` in child 3 | confirmed |
| A-4 | `DirectBridge` can carry a record answer in process | `DirectBridge.DispatchCommand` returns a typed output today with no answer path | Internal plugins keep the buffered path while external ones stream, so one answer has two shapes by transport | Unit test over the bridge asserting the same row sequence as the socket path | confirmed |

**Audit findings, 2026-08-21** (evidence read at implementation start, not asserted):

- A-1 `confirmed`. `ai/rules/go-standards.md:450` bans compat code everywhere, the plugin API included. Its carve-out at `:456` is conditional on a release that has not happened, so it is untriggered.
- A-2 `confirmed` in phase 4. No engine-to-plugin message carries a protocol list (`ConfigureInput` carries `Sections`/`Claims`, `ShareRegistryInput` carries `Commands`), and none was added. The plugin's own Stage 3 declaration states that this peer both reads and writes record answers, and `ProtocolRecordAnswers` (`pkg/plugin/rpc/types.go`) now says so. The engine gates on `Process.RecordAnswers()` in both directions: `answerResult` (`internal/component/plugin/server/dispatch_registry.go`) to WRITE, and `PluginConn.SendExecuteCommandAnswer` (`internal/component/plugin/ipc/rpc.go`) to READ.
  - **The frame follows the DECLARATION, never the payload.** A declaring plugin answers EVERY `execute-command` with a head, its records and a terminator, whether the handler produced rows or built one value, because the reader must know which frame is arriving before it reads the first line (R-1). `executeCommandAnswer` (`pkg/plugin/sdk/sdk_callbacks.go`) writes a built value through `rpc.WriteDocumentAnswer`. This is the symmetric twin of `answerResult`, which already writes an answer to a declaring peer whatever the payload turns out to be.
  - **AC-5 holds on the VALUE, not on the frame.** AC-8's second half ("a plugin that declared nothing gets the value frame it gets today") is what fixes the reading: a declaring plugin's frame changes and its payload does not. `TestExecuteCommandValueAnswerIsUnchanged` now compares the head's status and the one `item=` line against the same literals it compared the whole result frame against.
- A-3 `confirmed`. `writeRecordLine` (`internal/component/plugin/dispatch.go:341-349`) passes `record.Item`/`record.Fault` straight to the append helpers, and `AnswerRecordLineSize` (`pkg/plugin/rpc/message.go:310-317`) reads only `len(value)`. Neither reader type-asserts or unmarshals, so a byte-slice form is a drop-in.
- A-4 `confirmed`, with a correction. `serveEngineOpDirect` (`internal/component/plugin/server/dispatch_registry.go:275-282`) ALREADY detects `*recordAnswer` and projects it through `responseToDispatchOutput`, with a comment saying the bridge has no line to carry a record on. AC-7 is therefore reached by replacing that projection with a generator-carrying result, not by adding a path where none exists.
- **Path correction.** `internal/component/plugin/bridge.go` does not exist. `DirectBridge` is declared at `pkg/plugin/rpc/bridge.go:50` and `DirectBridge.DispatchCommand` at `:326-335`. Every row in this spec naming `internal/component/plugin/bridge.go` or `internal/component/plugin/bridge_test.go` is corrected below. <!-- doc-links: ignore (this bullet exists to name paths that DO NOT exist; repointing them at live code would destroy its meaning) -->
- **The frame is not dead code, and the Task overstates it.** Two live peers already speak the record sequence: the SSH exec client declares it through `ZE_ANSWER_PROTOCOL` (`internal/core/ssh/client/answer.go:76`, read at `internal/component/ssh/answer.go:249`), and a Python test peer declares it over the plugin connection (`test/scripts/ze_api.py:capability_done`, driven by `test/plugin/answer-many-records.ci`). The accurate claim is that no GO SDK plugin can use it. Phase 5's new `.ci` files scope to what the Go SDK adds, rather than re-proving the frame those four existing `.ci` files already cover.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A partially converted SDK declares `record-answers` before it can read one, so every answer is misread as a head payload | `interpretResponse` returns a tail string where a JSON object was expected; plugin unmarshal errors at startup | Declare the name in the same phase that wires `CallAnswer`, never earlier; the wiring test asserts both in one run |
| R-2 | The engine-to-plugin direction needs a second name and Stage 3 cannot carry it | Design review finds no message in which the engine states what it reads | Carry the engine's declaration in the Stage 1 or Stage 2 message the engine already sends, or make the result frame self-describing so the reader needs no declaration |
| R-3 | A row generator that outlives its handler reads state the plugin has already released | Race detector failure, or rows carrying zero values under load | State the generator's lifetime in the SDK doc comment: it is walked before the handler's caller returns, and never stored |
| R-4 | Two answer shapes exist in the SDK during the conversion, and a plugin picks the wrong one | A plugin compiles against both the value-returning and the answer-returning dispatch call | The answer-returning call replaces the old one rather than joining it (`ai/rules/no-layering.md`). **Phase 2 outcome, and it needs the owner's word before closure:** the WIRE is replaced. `dispatchCommandRPC`'s whole-value unmarshal is deleted, and after Stage 3 one frame carries a dispatch answer. `DispatchCommand` and `DispatchCommandArgs` SURVIVE as the buffered reading of that one frame (`dispatchCommandValue` -> `answerValue` -> `collapseAnswer`), beside `DispatchCommandAnswer` which walks it. Deleting them would put a drain-and-collapse loop at each of the nineteen in-tree call sites, all of which want a document, and `ai/rules/simplicity.md` refuses nineteen copies of one collapse. It is the same pairing the engine already keeps between `ResponseJSON` and `WriteAnswer`. If the owner wants the pair gone, it is its own phase: convert the nineteen callers and cascade the three helper signatures (`rr.dispatchCommand`, `rs.dispatchCommand`, `probeManager.dispatchFn`) |
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
| Internal plugin over `DirectBridge` | → | bridge answer path: `DirectBridge.DispatchCommandAnswer` (`pkg/plugin/rpc/bridge.go`) served by the `dispatch-command` entry's typed answer slot (`Server.dispatchCommandAnswer` -> `plugin.AnswerFor`) | `TestDirectBridgeDispatchCommandAnswer` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An SDK plugin completes Stage 3 | Its `declare-capabilities` names `record-answers` in `protocol`, and the engine's `Process.RecordAnswers` reads true for that peer |
| AC-2 | An SDK plugin dispatches an engine command whose walk produces more rows than `AnswerBufferThreshold` | The plugin receives every row in walk order and the terminator's `count`, and at no point holds more than one row plus the read buffer |
| AC-3 | An SDK plugin dispatches an engine command whose walk ends at or under the threshold | The plugin receives one document, and the value it sees equals what the same command produced before this spec |
| AC-4 | A plugin command handler answers with a row generator and an operator runs that command | The rows reach the operator as records, and the engine never holds the whole collection |
| AC-5 | A plugin command handler answers with a built value | The VALUE is unchanged from today, byte for byte. **Narrowed in phase 4 from "the answer", and the owner should know.** As written this row contradicted AC-8's second half. The engine must know which frame is arriving BEFORE it reads the first line, so the frame follows the peer's Stage 3 DECLARATION and not the individual payload: a plugin that declared `record-answers` answers every `execute-command` with head, one `item=` line and a terminator, a built value included. Holding the whole answer byte-identical would mean the frame follows the payload, and then a reader would have to guess the shape of a line before parsing it, which is the ambiguity child 2 exists to remove. `TestExecuteCommandValueAnswerIsUnchanged` (`pkg/plugin/sdk/sdk_callbacks_test.go:438`) pins the VALUE against literals captured on the unmodified tree, for six handler shapes, and asserts the frame separately |
| AC-6 | A row generator yields a row that is wider than one wire message | That row is reported as a fault and the walk continues, and the terminator counts it under `faults` |
| AC-7 | The same plugin command is served over the socket and over `DirectBridge` | Both produce the same row sequence and the same terminator counts. **Qualified at the Review Gate, and the owner should know.** The two producers are `rpc.WriteRecordAnswer` (socket) and `plugin.AnswerFor` (bridge), and they agree on every input a command can produce today, which `TestAnswerForAgreesWithTheWire` (`internal/component/plugin/dispatch_test.go`) now pins across the threshold, the column schema, the zero-row boundary and a failing walk. They diverge on exactly one input and do so deliberately: a row wider than `rpc.MaxMessageSize` is a rejected row on the socket (`boundedRecord`) and is carried whole in process, because one transport bounds a line and the other has no line. `AnswerFor`'s doc comment states it. A streamed in-process answer also skips the arity check `writeRecordLine` makes, so a positional row that disagrees with the head is named at the consumer rather than the producer. Neither is reachable from an in-tree engine generator: `commandRows` is the only one and it declares no `Fields` and produces no wide row |
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
| `TestPluginRunDeclaresRecordAnswers` | `pkg/plugin/sdk/sdk_test.go` | Stage 3 carries the protocol name | green (phase 2) |
| `TestDispatchCommandAnswerYieldsRows` | `pkg/plugin/sdk/sdk_engine_test.go` | rows arrive in walk order, terminator ends the walk | green (phase 2) |
| `TestDispatchCommandAnswerBoundedIsDocument` | `pkg/plugin/sdk/sdk_engine_test.go` | a short walk still reads as one document | green (phase 2) |
| `TestCallAnswerStopsGeneratorOnConsumerStop` | `pkg/plugin/rpc/mux_test.go` | a consumer that stops reading stops the walk | green (phase 2) |
| `TestDirectBridgeWaitDispatchSpansAnswerWalk` | `pkg/plugin/rpc/bridge_test.go` | the dispatch admission covers the walk, released once on both exits | green (phase 2) |
| `TestCollapseAnswerRefusesAnUnreadableDocument` | `pkg/plugin/sdk/sdk_engine_test.go` | a record the reader cannot hand on as JSON is named, not forwarded (Security Review, input validation) | green (phase 2) |
| `TestExecuteCommandRecordResult` | `pkg/plugin/sdk/sdk_callbacks_test.go` | a plugin handler's generator becomes a record result | green (phase 3) |
| `TestExecuteCommandValueAnswerIsUnchanged` | `pkg/plugin/sdk/sdk_callbacks_test.go` | AC-5: every shape a handler builds reaches the engine as the LITERAL bytes it did before, compared byte for byte rather than against a second marshal | green (phase 3, adjusted phase 4: the literals are now the head's status and the one `item=` line, because a declaring plugin's FRAME is the negotiated one and only its VALUE is held fixed) |
| `TestRecordsWriteAnswerFaultsARowNoLineCanCarry` | `pkg/plugin/records_test.go` | AC-6: a row wider than one wire message is a rejected row, the walk continues, and the terminator states `count=2 faults=1` | green (phase 3) |
| `TestRecordsMarshalJSONIsTheDocumentTheWireCollapsesTo` | `pkg/plugin/records_test.go` | the walk's two readings agree: the bridge's document is the document the wire collapsed to | green (phase 3) |
| `TestRecordsWithNoGeneratorIsAnEmptyCollection` | `pkg/plugin/records_test.go` | the zero-row boundary on both readings; a nil generator is an empty collection, not a panic | green (phase 3) |
| `TestWriteRecordAnswerRefusesTheReservedEnvelope` | `pkg/plugin/rpc/answer_write_test.go` | the ONE writer refuses `errors` before its first line, so a streamed answer that never reaches the collapse is refused too | green (phase 3) |
| `TestSendExecuteCommandReadsRecords` | `internal/component/plugin/ipc/rpc_test.go` | AC-8 and A-2: the engine reads the plugin's record result for a declaring peer, and the value frame for one that declared nothing | green (phase 4) |
| `TestRouteToProcessBuildsRecords` | `internal/component/plugin/server/command_test.go` | a streamed plugin answer becomes a `Records` payload, not `RawJSON`, and `routeToProcess` returns on the head with no record written | green (phase 4) |
| `TestRouteToProcessRefusesARowThatIsNotJSON` | `internal/component/plugin/server/command_test.go` | Security Review, input validation: a plugin row that is not JSON is a rejected row and the walk continues; the rejection quotes none of the payload | green (phase 4) |
| `TestHubStartupSinkRecordsTheProtocolDeclaration` | `internal/component/plugin/server/subsystem_test.go` | the hub's own startup sink stores a forked subsystem's protocol declaration, so `SubsystemHandler.Handle` reads the frame that subsystem writes | green (phase 4) |
| `TestDirectBridgeDispatchCommandAnswer` | `pkg/plugin/rpc/bridge_test.go` | bridge and socket agree row for row | green (phase 2) |
| `TestAnswerForAgreesWithTheWire` | `internal/component/plugin/dispatch_test.go` | AC-7's other half, added at the Review Gate: the two PRODUCERS build one answer, where the row above proves the two TRANSPORTS carry one | green (Review Gate, run 1) |
| `TestPluginRecordEnvelopeErrorsRefused` | `pkg/plugin/sdk/sdk_test.go` | the reserved envelope name is refused, on the socket and on the bridge | green (phase 3) |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| rows produced before the type is decided | 0 to `AnswerBufferThreshold` | 256 | N/A | 257 selects a streamed type |
| record line width | 0 to `MaxMessageSize` | `MaxMessageSize` | N/A | one byte over is reported as a fault |
| `count` in the terminator | 0 to max uint64 | rows produced | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-plugin-owned-command-streams` | `test/plugin/plugin-owned-command-streams.ci` | an operator runs a plugin command with a long walk and sees every row | green (phase 5) |
| `test-plugin-reads-engine-record-answer` | `test/plugin/plugin-reads-engine-answer.ci` | a plugin reads a streamed engine answer and acts on it | green (phase 5) |
| `test-plugin-command-partial-fault` | `test/plugin/plugin-command-partial-fault.ci` | applied rows and rejected rows both reach the operator | green (phase 5) |

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
- `pkg/plugin/rpc/conn.go` - an answer writer for a result, not only for an inbound request. **Corrected in phase 4: NOT touched, and nothing was owed.** `execute-command` is an INBOUND request on the plugin's side, so `Conn.AnswerWriter` already fits it exactly; phase 3's `answerExecuteCommand` (`pkg/plugin/sdk/sdk_dispatch.go`) already writes through it. The direction the spec worried about does not exist
- `pkg/plugin/rpc/collapse.go` - **added in phase 4:** `CollapseAnswer`, the buffered reading of an ARRIVING answer, MOVED out of `pkg/plugin/sdk/sdk_engine.go`. Both ends now read an answer as one value -- the SDK for an engine answer, the engine for a plugin's `execute-command` answer -- and `internal/component/plugin/ipc` cannot import `pkg/plugin/sdk`, so the alternative was a second copy of the same walk. Same argument that moved the collapse in phase 2 and the writer in phase 3
- `pkg/plugin/sdk/sdk_callbacks.go` - **also in phase 4:** `executeCommandAnswer` writes the answer sequence for EVERY payload once the plugin declared the shape, so the frame follows the declaration rather than the payload
- `internal/component/plugin/ipc/rpc.go` - `SendExecuteCommandAnswer` reads a record result; `SendExecuteCommand` is its buffered sibling, and both take the peer's declaration as an argument because `PluginConn` holds no copy of it
- `internal/component/plugin/server/command.go` - `routeToProcess` builds a `Records` payload from a streamed plugin answer
- `internal/component/plugin/server/system.go`, `internal/component/plugin/server/subsystem.go` - **added in phase 4:** the other two `SendExecuteCommand` callers pass `Process.RecordAnswers()`, and `hubStartupSink.onCapabilities` now STORES that declaration, which it previously dropped with the BGP capabilities the hub has no injector for
- `pkg/plugin/rpc/bridge.go` - the direct bridge carries a record answer in process
- `internal/component/plugin/server/dispatch_registry.go` - the `dispatch-command` entry's `typedWire` installs the answer slot beside the value slot. **Corrected in phase 2:** `serveEngineOpDirect`'s projection STAYS. It serves the JSON-shaped `DispatchRPC` fallback, whose result is one marshaled value and which has no line to carry a record on; an in-process caller that wants the records takes the typed answer slot instead
- `internal/component/plugin/dispatch.go` - `AnswerFor`, `WriteAnswer`'s in-process sibling: the same head, the same records, the same terminator, decided from the same threshold. **Corrected in phase 3:** the WIRE WRITER moved out. `WriteAnswer` keeps the `*Response` projection (which status the head declares, what a built payload renders to, what the terminator says about a failure) and hands the lines to `rpc.WriteRecordAnswer` / `rpc.WriteDocumentAnswer`. `writeRecordAnswer`, `writeDocumentAnswer`, `writeRecordLine`, `boundedRecord`, `answerRecordTooLargeFault`, `writeAnswerLine`, `marshalFields` and `answerStreamType` are DELETED here. The SDK cannot import `internal/component/`, so the alternative was a second streaming writer in `pkg/plugin/sdk`, which is the layering `ai/rules/no-layering.md` refuses -- the same argument that moved the collapse in phase 2
- `internal/component/plugin/types.go`, `internal/component/command/render_records.go` - `CollapseRecords` moved to `pkg/plugin/rpc`; both call it there
- `docs/architecture/api/ipc_protocol.md` - the direction each protocol name covers, and the plugin-side producer
- `docs/architecture/api/process-protocol.md` - Stage 3 now declares the name from the SDK

## Files to Create
- `pkg/plugin/records.go` - the SDK record producer a plugin command handler answers with
- `pkg/plugin/rpc/collapse.go` - `CollapseRecords` and the envelope names, moved out of `internal/component/plugin` so both ends of the connection build one document from one function
- `pkg/plugin/rpc/answer_row.go` - the positional-row machinery that collapse needs, moved with it (`internal/component/plugin/answer_row.go` deleted) <!-- doc-links: ignore (the old path is named to record that it was deleted in phase 2) -->
- `internal/test/cli/cmd_record_plugin.go` - **added in phase 5:** the Go SDK plugin the three `.ci` files drive. No in-tree plugin answers with rows, so a `.ci` had no producer to point at; it registers `show test records walk`, `show test records fault` and `show test engine answer`, and it is registered as the `ze-test record-plugin` root
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
   - Files: `pkg/plugin/sdk/sdk.go`, `pkg/plugin/records.go`, `pkg/plugin/rpc/bridge.go`, `internal/component/plugin/server/dispatch_registry.go`
   - Verify: the answer-returning call and the record producer exist as stubs, so the wiring tests fail on behavior rather than on a missing symbol
   - **The Stage 3 declaration is NOT set in this phase.** R-1 and phase 2's own Verify line put the declaration in the phase that wires `CallAnswer`, and the tree makes that binding: nineteen in-tree SDK callers of `DispatchCommand`/`DispatchCommandArgs` (`internal/component/bgp/plugins/rib/rib.go:789`, `gr/gr.go:607`, `rs/server.go:430`, `rr/rr.go:531`, `bmp/bmp.go:667` which reads `data`, `rpki`, `healthcheck`, `watchdog`, `internal/plugins/exabgp/`) would receive a record sequence into `dispatchCommandRPC`'s whole-value unmarshal. Declaring here leaves the tree red between phases. `TestPluginRunDeclaresRecordAnswers` is WRITTEN here and FAILS here
2. **Phase: Read side** -- the SDK consumes a record answer
   - Tests: `TestDispatchCommandAnswerYieldsRows`, `TestDispatchCommandAnswerBoundedIsDocument`, `TestCallAnswerStopsGeneratorOnConsumerStop`
   - Files: `pkg/plugin/sdk/sdk.go` (Stage 3), `pkg/plugin/sdk/sdk_engine.go`, `pkg/plugin/rpc/mux.go`, `pkg/plugin/rpc/types.go`, `pkg/plugin/rpc/bridge.go`, `internal/component/plugin/server/dispatch_registry.go`
   - Verify: declaring the name and reading the answer land in this phase together, so R-1 cannot occur between phases. `TestPluginRunDeclaresRecordAnswers`, written red in phase 1, goes green here, and every in-tree SDK caller named in phase 1 still reads its answer
   - **AC-7 lands here, and phase 1 found it has no phase of its own.** The engine needs a `typedWire` that calls `SetDispatchCommandAnswer` (`internal/component/plugin/server/dispatch_registry.go`). **Corrected in phase 2:** it is NOT a new `engineOps` entry. The registry is keyed by method and `TestPluginRPCRegistryCoversAllPaths` refuses a duplicate method, so the answer slot rides the existing `rpc.MethodDispatchCommand` entry beside `SetDispatchCommand`. `wantTyped` already carries that method, so no row was owed; the assertion went to `TestWireBridgeDispatchInstallsTypedSlots` instead. Two gaps block it, and both are decided here rather than rediscovered:
     - **`Answer.terminator` and `Answer.err` are unexported** (`pkg/plugin/rpc/types.go:75-80`) and only `MuxConn.CallAnswer` fills them, so an in-process producer cannot state its counts and `Verdict()` reads `truncated` over the bridge where the socket reads `partial`. `pkg/plugin/rpc` gains a constructor that lets a producer in the same package state the terminator. It stays unexported-by-field: `Verdict` remains the one derivation, so the two transports cannot disagree about an outcome.
     - **The dispatch admission MUST span the walk, not just the call.** `DirectBridge.DispatchCommandAnswer` currently takes `beginDispatch` and releases it with `defer b.endDispatch()`, so the admission is gone before the caller ranges over `Answer.Records`. Once a producer exists, that walk reads engine state, and `StopDispatch`+`WaitDispatch` is the rollback barrier that must cover it: releasing early lets a rollback tear down state under a live walk. Wrap the returned `Records` so the release happens when the range ends, by exhaustion or by consumer stop, and state the obligation with MUST on both sides (`docs/contributing/ze-style.md`, "Group an allocation with its release"). `TestBridgeRollbackWaitsForDirectDispatch` guards this property for `DispatchRPC` and is the shape to copy
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
| Rule: `ai/rules/no-layering.md` | **SETTLED 2026-08-21, and the owner can overturn it.** One WIRE, one COLLAPSE, two entry points. `DispatchCommand` and `DispatchCommandArgs` survive as the buffered reading of the frame `DispatchCommandAnswer` walks, both routed through the single `rpc.CollapseRecords`. The rule forbids two implementations of one thing; this is one implementation reached two ways, the shape `io.ReadAll` has over `io.Reader`. Deleting the pair would put a drain-and-collapse loop at each of nineteen call sites that all want a document, which is nineteen copies of the collapse and the layering the rule actually names. What a plugin author chooses between is streaming and buffered CONSUMPTION, a real choice with a real answer, never two spellings of one answer. The row's original demand, that both must not exist, was written against the two SHAPES R-4 names, and after phase 2 there is only one |
| Rule: `ai/rules/no-layering.md`, the branch that DOES remain | `dispatchCommandValue` (`pkg/plugin/sdk/sdk_engine.go:282`) still carries an else-branch reading the single-line frame through `callEngineWithResult`, reached when `readsRecordAnswers()` is false, which is before Stage 3 completes. That is not a survival of the old shape: it MIRRORS `answerResult`'s engine-side branch, which this spec's Behavior to preserve keeps until child 2. Both branches die together in child 2. Phase 2's report called the single-line reader deleted; the SYMBOL `dispatchCommandRPC` is gone and its body is not |
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

### Open finding for the Review Gate (main thread, diff read 2026-08-21)

**A defensive nil branch was collapsed, and it changes one rendering.** The old
`routeToProcess` guarded `if rpcOut != nil` and answered a nil transport result
with `&plugin.Response{Status: plugin.StatusDone}`, carrying NO `Data`. The new
path routes every transport through `valueAnswer` and `ExecuteCommandValue`, so
that case now yields `Data: plugin.RawJSON("")`. `RawJSON.MarshalJSON`
(`internal/component/plugin/types.go:144-147`) turns an empty string into `null`,
and `ResponseJSON` (`dispatch.go:167`) returns `("", nil)` only when `resp.Data`
is nil. So an operator on that path would read `null` where they read nothing
before. Six sites test `resp.Data == nil` and all six now take the other branch
for it.

Not fixed, and deliberately not: the change is unreachable from the in-tree SDK.
`SetExecuteCommand` (`pkg/plugin/sdk/sdk.go:465`) is registered only when a
handler exists, `HasExecuteCommand` gates the bridge path on that same flag, and
the handler returns `executeCommandOutput(status, data)` rather than a bare nil.
The old guard was defensive against a `(nil, nil)` an `ExecuteCommandHandler`
MAY return by its signature and no in-tree handler does.

It is recorded rather than repaired because repairing it means CHOOSING between
two renderings that already disagreed. Before this spec, a nil result rendered
as nothing and a non-nil result carrying an empty payload rendered as `null`.
The new code makes both render `null`, which is the more consistent of the two
and was reached by accident. Making the empty-payload case render nothing
instead would be a second behavior change on a path that IS reachable. The
question is which rendering an operator should see for an empty plugin answer,
and it belongs to the owner or to the Review Gate, not to a phase agent.

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

## Review Gate

Reviewer: one independent context, spawned after the implementing phase ended,
running every lens itself (`ai/rules/planning.md`, "Critical Review Is the
Central Deliverable"). It did not write this code. No sub-readers were spawned.

Subject of the review: commit `4bdae01c8`, "feat(plugin): the SDK produces and
reads record answers". `pkg/plugin/rpc/types.go` has moved since, under
`plan/spec-plugin-registers-pipe-operations.md` (`91203b8aa` onward, `PipeDecl`);
that work is not part of this diff and was not reviewed.

Evidence run for this gate:

| Check | Result |
|-------|--------|
| `make ze-unit-pkg-test PKG=./pkg/plugin/...` | green: plugin, plugin/rpc, plugin/sdk |
| `make ze-unit-pkg-test PKG=./internal/component/plugin/...` | green: all ten packages, race-instrumented |
| `make ze-functional-plugin-test` | 629 of 629 pass in 136.4s, 0 fail, 56 platform-skipped, and the three new `.ci` are among the RUN set, not the skipped one: `plugin-command-partial-fault` 3.5s, `plugin-owned-command-streams` 3.3s, `plugin-reads-engine-answer` 2.7s |
| `python3 scripts/dev/validate.py` | all checks passed |
| `python3 scripts/dev/audit-test-relaxation.py 4bdae01c8~1` | clean, 102 test files examined |
| `make ze-test-weakened-check` | parses, and every "moved, not weakened" row was verified against the destination file |
| `make ze-lint-changed` | 0 issues, both passes |

### Round 1 (whole diff)

Scope, written before the round ran: every file of `4bdae01c8`, the wiring of
every new exported symbol, each of AC-1..AC-9 against implementation and test,
the discrimination of each test the spec names, the `internal` to `pkg` move,
the `Row`-as-appender decision, and the emitted wire against the documented
grammar.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | **AC-7 had no test over its producing code.** `plugin.AnswerFor`, the whole in-process answer producer, has no test, and neither does its one caller `Server.dispatchCommandAnswer`. `TestDirectBridgeDispatchCommandAnswer`, the test the spec names for AC-7, drives a hand-written stub handler inside `pkg/plugin/rpc`: it proves the two TRANSPORTS carry one answer and says nothing about whether the two PRODUCERS build one. They are separate implementations of one decision (hold to the threshold, collapse or stream, state the counts), and nothing held them to the same answer. An acceptance criterion with no test is always in scope (`ai/rules/planning.md`, "Bounding the loop") | `internal/component/plugin/dispatch.go` AnswerFor, `internal/component/plugin/server/dispatch.go` dispatchCommandAnswer | fixed |
| 2 | ISSUE | **`CheckRowArity` was exported by the move and has no caller outside its own package, test or not.** It was `checkRowArity` before this commit; the move to `pkg/` made it public API, which is a one-way door, for a function only `writeRecordLine` calls. `validate.py` cannot see it: `check_cross_package_wiring` collects symbols only from changed files under `internal/` and `cmd/`, so a `pkg/` export is outside the gate's population | `pkg/plugin/rpc/answer_row.go` CheckRowArity | fixed |
| 3 | ISSUE | **`DirectBridge.HasDispatchCommandAnswer` is exported with a test as its only caller.** Its own doc comment says so: "Its reader is the registry drift guard". Its five siblings (`HasDispatchCommand`, `HasDispatchCommandArgs`, `HasEmitEvent`, `HasBatchValidate`, `HasExecuteCommand`) each have one non-test caller, so this one breaks the set rather than following it. `ai/rules/completion.md`: a symbol whose only hits are the definition and test files is dead code | `pkg/plugin/rpc/bridge.go` HasDispatchCommandAnswer | **NOT fixed**, see "What this gate could not close" |
| 4 | ISSUE | **`streamType` duplicates the newly exported `rpc.AnswerStreamType`, line for line.** Before this diff the engine-side chooser was unexported, so the copy was forced. This diff exported it and left the copy standing, in a file that already imports `rpc`. Two names for one fact will disagree (`docs/contributing/ze-style.md`, and the style pass of the review skill) | `internal/component/command/render_records.go` streamType | **NOT fixed**, see below |
| 5 | ISSUE | **Five comments name a symbol this diff deleted or renamed.** `writeDocumentAnswer` left `internal/component/plugin/dispatch.go` in phase 3 and exists nowhere now; `answerStreamType` became `rpc.AnswerStreamType`. Two of the five were ADDED by this diff, in doc comments written for the new code, and one of those sits on `rpc.NewAnswer`, a symbol this spec introduced. `ai/rules/stale-comments.md` is blocking, and the same diff repointed four other references correctly, so these are misses rather than a decision | `pkg/plugin/rpc/types.go` NewAnswer; `internal/component/plugin/dispatch.go` documentAnswer; `pkg/plugin/sdk/sdk_engine_test.go` documentAnswerLines and TestDispatchCommandAnswerBoundedIsDocument; `internal/component/command/render_records.go` streamType | fixed, except the `render_records.go` one, see below |
| 6 | NOTE | **`Records.wire` says the append is "the ONE allocation".** `AppendTo(nil)` starts from a nil slice and grows, so a 60 kB row pays for several. The claim sits on the one comment that justifies the appender, so it is the comment a reader of child 3 will lean on | `pkg/plugin/records.go` wire | fixed: the wording now names the growth as the second cost child 3 removes |
| 7 | NOTE | **AC-7's wording is stronger than the code.** The two producers diverge on one input by design: a row wider than `rpc.MaxMessageSize` is a rejected row on the socket and is carried whole in process. A streamed in-process answer also skips the arity check the wire writer makes. `AnswerFor`'s doc comment states the first. Neither is reachable from an in-tree generator: `commandRows` is the only one, it declares no `Fields`, and it produces no wide row | this spec, AC-7 | fixed: AC-7 now states the qualification and names the new test |
| 8 | NOTE | **`p.recordAnswers` is read back from the plugin's own message, so it is unconditionally true.** `Plugin.Run` writes the protocol list and then stores `caps.Understands` of that same list. Nothing negotiates: the engine never answers with what it accepted. The consequence is two branches no SDK plugin can reach, the single-line arm of `dispatchCommandValue` and the non-nil-result arm of `answerExecuteCommand`. The spec already declares the first as the deliberate mirror of `answerResult` that child 2 removes, and the second is the same shape | `pkg/plugin/sdk/sdk.go` Plugin.Run, `pkg/plugin/sdk/sdk_dispatch.go` answerExecuteCommand | acknowledged: transitional by the spec's own design, dies with child 2 |
| 9 | NOTE | **The command timeout now bounds the walk, not the call.** `routeToProcess` hands `cancel` to the row generator through `sync.OnceFunc`, so `cmd.Timeout` covers the operator's whole read of a streamed answer, and a consumer that never ranges holds the deadline until it expires. Deliberate, and the code says so | `internal/component/plugin/server/command.go` routeToProcess | acknowledged |
| 10 | NOTE | **A `Record` carrying both an `Item` and a `Fault` silently drops the item.** Every reader tests `Fault` first (`Records.wire`, `writeRecordLine`, `CollapseRecords`, `checkedRecord`), so the precedence is at least consistent, and both `Record` types document that exactly one of the two is set | `pkg/plugin/records.go` wire | acknowledged: consistent everywhere, and the contract is stated |

### Fixes applied

- **Finding 1.** Added `TestAnswerForAgreesWithTheWire`
  (`internal/component/plugin/dispatch_test.go`). One `*Response` shape is built
  twice, from a fresh generator each time, and taken through `WriteAnswer` and
  through `AnswerFor`; the head, the records and the verdict are compared. Nine
  cases: a built payload, a response with no data, a walk inside the threshold,
  a walk exactly at it, a walk one row past it, a walk declaring its columns, a
  nil generator, a failing walk that collapses, and a failing walk that streams.
  Both sides are read as a CONSUMER reads them, the wire through `rpc.ParseLine`
  and `rpc.ParseAnswerTail` and the in-process answer through `Answer.Records`
  and `Answer.Verdict`, so the agreement is evidence and not a tautology.

  **Mutation-tested twice.** Changing `AnswerFor`'s threshold test from `<=` to
  `<` fails the exactly-at-the-threshold case: "the in-process head states
  type=ndjson and the wire states type=json". Dropping `Message` from the
  streamed terminator fails the streaming-failure case: "the in-process answer
  ends done and the wire answer ends aborted". The failing-walk-that-streams
  case was added BECAUSE the first version of the table did not catch the second
  mutation, a two-row failing walk collapses and reaches its terminator by
  another route.
- **Finding 2.** `CheckRowArity` is `checkRowArity` again, with a comment saying
  why it stays unexported. `plan/spec-record-answers-3-zero-alloc.md` already
  names it in lower case in two places, so the tree and its next spec agree.
- **Finding 5.** Four comments repointed at symbols that exist: `writeDocumentLines`
  in `pkg/plugin/rpc/answer_write.go` for the two that named `writeDocumentAnswer`
  from `pkg/`, and `rpc.WriteDocumentAnswer` for the one that named it from the
  engine side. Two neighbouring comments that named a symbol which still exists
  but has changed package were given its new home at the same time
  (`terminatedRecords` in `pkg/plugin/rpc/types.go`, `rejectedRow` in
  `internal/component/plugin/dispatch_test.go`), because a reader who cannot find
  the producer is the cost `ai/rules/evidence.md` names.
- **Finding 6.** `Records.wire`'s comment now states both costs the appender
  exists to remove, and names `spec-record-answers-3-zero-alloc` rather than
  this spec.
- **Finding 7.** AC-7 now carries the qualification and names the new test.

### Round 2 (the fixes, and what they touched)

Scope, written before the round ran: the unexport and its one call site plus its
file siblings (`zipRow`, `quoteFields`, `jsonArrayLength`), the five repointed
comments against the symbols they now name, and the new test against the two
producers it compares.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | Nothing found inside the round's scope, and no always-in-scope finding outside it. `make ze-lint-changed` 0 issues, `validate.py` clean, both plugin unit groups green, and `audit-test-relaxation.py` clean over the two edited test files | - | - |

### What this gate could not close

**One ISSUE is open at its real severity, so this gate is NOT clean.**

Round 2, 2026-08-22. Findings 4 and 5 are FIXED (`5d6ad6919`): `streamType` is
gone and its two call sites read `rpc.AnswerStreamType`, and the comment names
what exists. The directories that blocked them were free by then.

Finding 3 stays open, and the fix this table proposed for it was TRIED and does
not work. Asserting the slot by calling `DispatchCommandAnswer` segfaults:
`TestWireBridgeDispatchInstallsTypedSlots` builds a bare `&Server{}` with no
dispatcher, so the dispatch nil-derefs before it can report whether the slot
exists. Proving the slot by using it needs a fully built server, which is a
different test from one whose subject is that the registry wires every slot.

So the real choice is narrower than the table below suggests, and it is the
owner's: keep an exported accessor whose only caller is a wiring test, or build
a second test with a real server so the accessor can go. No gate is red either
way (`ze-repository-check` passes; the finding comes from reading
`ai/rules/completion.md`, not from a check).

Findings 3 and 4, and the `render_records.go` half of finding 5, all sit in
`internal/component/plugin/server/` and `internal/component/command/`. Another
agent holds those two directories, so this review reports them rather than
editing them. Each is a one-line change:

| Finding | The fix | Why it is not applied here |
|---------|---------|----------------------------|
| 3, `HasDispatchCommandAnswer` has only a test caller | Either delete it and let `TestWireBridgeDispatchInstallsTypedSlots` assert the slot by CALLING `DispatchCommandAnswer`, which is the stronger assertion because it proves the slot is wired to the engine rather than merely present, or give the method a product caller | Deleting it breaks the compile of `internal/component/plugin/server/dispatch_registry_test.go`, which the other agent owns. Gating `Plugin.DispatchCommandAnswer` on it instead would be worse code, and the bridge's own doc comment gives the reason: the bridge names the missing slot where a closed mux answers with a read error that says nothing |
| 4, `streamType` duplicates `rpc.AnswerStreamType` | Return `rpc.AnswerStreamType(fields)` from it, or delete it and call `rpc.AnswerStreamType` at both sites | `internal/component/command/render_records.go` is the other agent's |
| 5, a comment names `answerStreamType` | Repoint it at `rpc.AnswerStreamType` | Same file |

### What the review verified rather than assumed

| Item | Verdict |
|------|---------|
| Every AC has an implementation at file plus symbol AND a test | Yes for AC-1 to AC-9 after finding 1's fix. AC-7 was the only one whose named test did not reach its producer |
| Every exported symbol in `pkg/plugin/records.go` has a non-test caller | Yes. `Row`, `Record`, `Records`, `Records.WriteAnswer` and `Records.MarshalJSON` are all reached from `internal/test/cli/cmd_record_plugin.go`, which ships in the `ze-test` binary and is driven by three `.ci` files through the real daemon |
| Every exported symbol added to `pkg/plugin/rpc/` by this diff has a non-test caller | Two did not, findings 2 and 3. Every other one does: `WriteRecordAnswer`, `WriteDocumentAnswer`, `AnswerStreamType`, `CollapseRecords`, `CollapseAnswer`, `NewAnswer`, `ErrEmptyAnswerRecord`, `ErrReservedEnvelopeKey`, `SetDispatchCommandAnswer` and `DispatchCommandAnswer` each have one |
| `MuxConn.CallAnswer` gained its first non-test caller | Yes, two: `pkg/plugin/sdk/sdk_engine.go` and `internal/component/plugin/ipc/rpc.go` |
| The move dragged nothing heavy into `pkg/` | `go list -deps ./pkg/plugin/rpc` names three in-tree packages, `internal/core/stringsx`, `internal/core/textbuf` and `internal/core/selector`. `pkg/plugin` adds none |
| The wire matches what the docs claim | Yes. `AppendAnswerHead` writes the head as `status=`, `type=`, then an optional `key=` and `fields=`; `AppendAnswerItem` writes `item=` and `AppendAnswerFault` writes `fault=`; `AppendAnswerTerminator` writes `count=` with an optional `faults=` and `message=`. The answer-type table in `docs/architecture/api/ipc_protocol.md` states the same three rows the encoder takes, the bounded head's missing `key=` included |
| The width boundary is the LINE, not the item | Yes, and the two ends agree. `boundedRecord` accepts a line of exactly `MaxMessageSize`, `Conn.writeFrame` accepts one more for the newline, and `NewFrameReader` sets the scanner's exclusive maximum to the same number |
| The tests the spec names discriminate | Sampled and confirmed. `TestExecuteCommandValueAnswerIsUnchanged` compares against literals captured on the unmodified tree, not against a second marshal. `TestRouteToProcessBuildsRecords` asserts `routeToProcess` returned with one line written, which is what proves the engine holds no collection. `TestRouteToProcessRefusesARowThatIsNotJSON` asserts the rejection quotes none of the payload. The three `.ci` files carry fixture preconditions that FAIL the test when the walk is not past both the 256-record threshold and the 16 MB line ceiling, which is what stops them passing with the record path removed. `TestAnswerForAgreesWithTheWire` was mutation-tested twice, above |
| The "moved, not weakened" rows of `test/weakened.md` are true | Yes. All five named tests exist at the destination the row names, with the same assertions |

### The two judgement calls

**`Row` as an appender EARNS its place, and it is not speculative generality.**
Three things decide it. `ai/rules/performance.md` states the directive outright:
a named type MUST have an `AppendTo([]byte) []byte` method, and callers never
format a type from the outside. The shape is already the tree's, in
`internal/core/family` and in `pkg/plugin/rpc/enums.go`. And the spec it serves
is WRITTEN, not hypothetical: `plan/spec-record-answers-3-zero-alloc.md` is at
status `design` and says so at its A-1, "the appender shape exists and has a
producer". The test `ai/rules/simplicity.md` applies is to an ABSTRACTION with
one user, and the abstraction here is the `Row` interface, which is owed whatever
the method's signature is. Only the signature differs between `AppendTo(buf)
[]byte` and `Bytes() []byte`, the two cost the same to write and to implement,
and a house rule names the first. The honest caveat is that the buffer parameter
is dead today: `Records.wire` calls `AppendTo(nil)` for every row, so the
appender buys nothing yet. Finding 6 fixed the comment that overstated what it
buys.

**The `internal` to `pkg` move is right, and the spec's reason for it is loosely
stated.** The spec says `pkg/plugin/sdk` "cannot import
`internal/component/plugin`". There is no compiler barrier: `go list -deps` shows
`internal/component/plugin` does not reach `pkg/plugin/sdk`, so no import cycle
exists, and Go's `internal` rule permits the import because both are rooted at
`github.com/ze-software/ze`. The real reason is better than the stated one.
`internal/component/plugin` pulls in `internal/component/config/storage`,
`internal/component/plugin/registry`, `internal/core/metrics`,
`internal/core/events` and `internal/core/family`, so an out-of-tree plugin
importing the SDK would link the config storage layer and the plugin registry to
get a collapse function. The move went the right way and it stayed cheap.

What the move DID cost is the one-way door it opened, which is findings 2 and 3:
three symbols became public API and two of them have no caller. `validate.py`
cannot see that class, because its wiring check reads only changed files under
`internal/` and `cmd/`. A `pkg/` symbol is exactly where an unwired export is
most expensive and least checked.

### Final status

- [ ] The gate re-run shows 0 BLOCKER, 0 ISSUE

**0 BLOCKER. 2 ISSUE open**, findings 3 and 4, plus the `render_records.go` half
of finding 5. All three are one-line fixes in files another agent holds. They are
left at their real severity rather than downgraded: this gate is not clean, and
the spec MUST NOT close until they are applied and a round re-runs over them.

NOTEs recorded above: findings 6 to 10. Six and seven were fixed anyway because
they were free. Eight, nine and ten are acknowledged and need no change.
