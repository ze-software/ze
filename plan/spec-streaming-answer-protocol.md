# Spec: streaming-answer-protocol

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 6/8 |
| Deferral shard | `plan/deferrals/streaming-answer-protocol.md` |
| Handoff | - |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A command answer is materialized whole before any byte reaches the operator. `ResponseJSON` marshals `Response.Data` to one `[]byte`, copies it to a `string`, and every pipe operator that understands JSON re-parses that string into an `any` tree and re-marshals it. `show bgp rib | first 10` therefore builds the entire RIB, parses it, and discards all but ten rows.

The frame carries no way to say otherwise. `#<id> ok [<json>]` is one line, so the 16 MB `MaxMessageSize` is a ceiling on the whole answer rather than on one record, and a truncated answer is indistinguishable from a short one.

Three further things cannot be expressed today. A partial result, 97 rows applied and 3 rejected, collapses into one error string, so an operator cannot tell 97-of-100 from 0-of-100. A per-record error: the wire carries one `error` for the whole answer. And the difference between "I did not understand your command" and "I understood it, tried, and it failed", which share the `error` verb and the exit code, so a client cannot offer completion on the first and an operational message on the second.

Goal: make every answer a sequence of records produced by a generator, with the verdict on the first line, no count known up front, and a mandatory terminator whose absence detects truncation.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - the wire format this spec extends
  → Constraint: every message is one newline-terminated line, `#<id> <verb> [<json>]`; `FrameReader` is a `bufio.Scanner` with newline splitting and `MaxMessageSize` 16 MB
  → Decision: `MuxConn` routes verb `ok`/`error` to the waiting caller by id and every other verb to the `Requests()` channel, so a new verb would be read as an inbound request
- [ ] `docs/architecture/api/commands.md` - the dispatch contract and the pipe surface
  → Constraint: the daemon registers each handler under its YANG command path, and the path is the dispatch key
  → Decision: REST and gRPC are thin adapters over the shared API engine, so they inherit whatever the engine returns rather than framing their own answers
- [ ] `ai/rules/cli.md` - the response-payload contract
  → Constraint: a command's response payload MUST be structured data, so `| json`, `| yaml` and `| table` are three renderings of one payload; a streamed answer MUST keep that property
- [ ] `ai/rules/interop-and-goal-validation.md` - the vacuity traps
  → Constraint: a test asserting the ABSENCE of something passes when the mechanism is deleted; the reassembly-equivalence test must be shown to FAIL when streaming is reverted
- [ ] `ai/rules/no-layering.md` - replacement, not accumulation
  → Constraint: where the record path replaces the whole-string path, the whole-string path is deleted rather than kept beside it

**Key insights:**
- The frame layer already delivers the new tail unchanged: `ParseLine` cuts the verb at the first space and returns everything after it as one unsplit payload. Only the interpretation of that payload moves.
- Keeping the `ok`/`error` verb spellings means `readLoop`'s response-versus-request predicate (`verb == StatusOK || verb == StatusError`) needs no change, so no plugin's inbound dispatch is disturbed.
- `Response.Status` is already documented as `done`, `error`, or `partial`, and `Response.Partial` already exists described as "true for streaming chunks". The envelope anticipated this; nothing was built on it.
- The only per-record path that exists today is `monitor`, which proves the SSH exec channel streams.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/plugin/rpc/message.go` - `ParseLine` splits `#<id>`, cuts the verb at the first space, returns the remainder as one payload. `AppendResult` writes `#<id> ok <json>`, `AppendOK` writes `#<id> ok`, `AppendError` writes `#<id> error <json>`. `parseRPCError` falls back to using a non-JSON payload as the message text
- [ ] `pkg/plugin/rpc/mux.go` - `readLoop` reads a frame, requires the `#` prefix, cuts the id, and computes `isResponse` from the verb being `StatusOK` or `StatusError`. A response is routed with `pending.LoadAndDelete(idStr)` and sent on a `chan []byte`. A response with no waiting caller is logged as orphaned and increments `consecutiveBad`; past `maxConsecutiveBadLines` (100) the reader stores an error and closes the connection
- [ ] `pkg/plugin/rpc/framing.go` - `MaxMessageSize` is 16 MB, applied per line
- [ ] `internal/component/plugin/types.go` - `Response` carries `Serial`, `Status`, `Partial`, `Data`, `Error`. `ResponseData` is a sealed interface with `Map`, `Slice[T]` and `RawJSON` implementors. `RawJSON.MarshalJSON` returns an error for a non-JSON payload rather than quoting it
- [ ] `internal/component/plugin/dispatch.go` - `ResponseJSON` is the single flatten sequence: `json.Marshal(resp.Data)` then `string(b)`. `CommandDispatcher.JSON` wraps it in a `RenderedResponse` holding `Output string`
- [ ] `internal/component/ssh/ssh.go` - `execMiddleware` splits the pipe chain with `ProcessPipesDefaultFormatChecked`, calls the executor, and writes the whole rendering with one `Fprintln`. Lifecycle commands, the plugin protocol channel and streaming commands are answered before that split
- [ ] `internal/component/command/pipe.go` - `ApplyPipes` is `string` in, `string` out. `applyCount`, `applyFirst`, `applyLast`, `injectPipeMeta`, `ApplyJSON`, `applyNDJSON` and `applyYAML` each unmarshal the whole answer into `any` and re-marshal
- [ ] `internal/component/command/pipe_table.go` - `applyTableStyled` unmarshals the whole answer; column widths need every row
- [ ] `internal/core/ssh/client/client.go` - `ExecCommand` dials, runs `session.CombinedOutput`, and returns the whole answer as one trimmed string. `RawCommand` appends `| raw`
- [ ] `internal/component/cli/client/main.go` - `cliClient.Execute` sends the operator's text with pipes intact and prints what comes back; the client does not format. `StreamMonitor` is the one path that formats per line, via `ProcessPipesDefaultFunc`

**Behavior to preserve:**
- `#<id> ok [<json>]` and `#<id> error [<json>]` stay valid on the wire for any peer that has not negotiated the new answer shape
- `readLoop`'s response-versus-request discrimination, unchanged, because the verb spellings do not move
- The pipe chain keeps running in the daemon, so one implementation renders every surface
- `| raw` keeps answering the dispatcher's JSON byte for byte
- A command that returns nothing keeps printing `OK` when the chain names no format operator
- `RawJSON` keeps refusing a non-JSON payload

**Behavior to change:**
- The `ok` line gains a `key=value` tail carrying `status=` and one open-ended key
- Every answer becomes a head, zero or more records, and a terminator; a one-record answer is that shape with one record
- The `error` verb narrows to "the command was not understood"; execution failure moves to `status=error`
- An unbounded answer is produced by a generator rather than a built collection

## Data Flow (MANDATORY)

### Entry Point
- An operator's command text arrives on the SSH exec channel, or a plugin's RPC arrives on the multiplexed plugin connection
- Format at entry: unchanged, a newline-terminated line

### Transformation Path
1. `ParseLine` splits id, verb, and the remaining payload; unchanged
2. New: the payload is tokenized as `key=value` pairs rather than assumed to be JSON
3. The dispatcher resolves the command to a handler; unchanged
4. New: a handler answers with a row generator rather than a built collection
5. New: the encoder writes a head line, then one line per record, then a terminator
6. The pipe chain consumes records one at a time where the operator allows it, and buffers where it does not
7. The transport writes each line as it is produced rather than one line at the end

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator ↔ daemon | SSH exec channel, newline-framed lines | No |
| Daemon ↔ plugin | `MuxConn` over the plugin connection, newline-framed lines | No |
| Handler ↔ encoder | `ResponseData`, gaining a generator implementor | No |
| Encoder ↔ pipe chain | records pulled one at a time, or collapsed | No |

### Integration Points
- `ResponseData` - gains one implementor carrying a row generator and an envelope key
- `MuxConn.pending` - an id must survive many lines rather than one `LoadAndDelete`
- `ApplyPipes` - gains a record-at-a-time path replacing the whole-string path where it can

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding | No | |

## Wire Grammar

Every line is `#<id> <verb> <tail>`. On the SSH exec channel the `#<id>` prefix is absent, since one command owns the channel.

**Every answer opens with a meta line, then its records, then a terminator, and the code has one path** (owner decision, 2026-08-20). A single-element answer is that shape carrying one record. Nothing declares how many records follow, so nothing can be wrong about the count.

**`type=` on the head says how to read every `item=` that follows.** It is the one key a reader needs before the body, and it removes any first-byte or shape heuristic:

| `type=` | Each `item=` carries | Used for |
|---------|----------------------|----------|
| `json` | the whole answer as one JSON document | a bounded answer, the shape a command returns today |
| `ndjson` | one self-describing object | a sequence whose rows do not share a fixed schema |
| `stream` | a positional array, read against the head's `fields=` | a long sequence with a fixed schema, where repeating the keys on every row is most of the bytes |

`fields=` appears with `type=stream` and carries the column schema. It is not a declaration that exists to announce a shape: it is data the reader needs anyway, and it carries the column ORDER, which today lives in the separate `column_order.go` registry that a renderer joins by command name.

**Which type an answer uses is decided at run time from the OUTPUT, never by the command.** A handler returns a generator and decides nothing. The encoder pulls up to a threshold of records. A walk that ends at or under it is answered as one document. A walk that passes it streams, flushing the records already held. So a bounded answer keeps its current JSON and no consumer of an existing command breaks, while a long answer never materializes. The threshold is one named constant of the same order as the answer queue (256). It is not a config knob.

`| first 10` still cancels the walk, because 10 is under the threshold. The consumer stops during the buffering window and the decision point is never reached.

**Naming caution, unresolved.** `type=json` and `type=ndjson` share their words with the pipe operators `| json` and `| ndjson`. Those are RENDERINGS an operator asked for, not how the daemon serialized the body. The two are unrelated and a reader must not infer one from the other. Either the wire words change (`document`, `records`, `columns`) or the docs state the independence.

| Line | Verb | Required keys | Optional keys | Position |
|------|------|---------------|---------------|----------|
| head | `ok` | `status=`, `type=` | `key=`, and `fields=` when `type=stream` (last, open-ended) | first line for this id |
| result record | `ok` | `item=` | - | between head and terminator |
| error record | `ok` | `fault=` | - | between head and terminator |
| terminator | `ok` | `count=` | `faults=`, `message=` | last line for this id |
| not understood | `error` | `message=` | `code=` | the only line for this id |

| Example line | Meaning |
|--------------|---------|
| `#7 ok status=done type=json` | a bounded answer opens; one `item=` carries the whole document |
| `#7 ok status=done type=ndjson key=peers` | a sequence of self-describing objects follows, under the `peers` envelope |
| `#7 ok status=done type=stream fields=["peer","as","state"]` | a long fixed-schema sequence follows; each `item=` is a positional array against these fields |
| `#7 ok item=["10.0.0.1",65001,"established"]` | one row of a `type=stream` answer |
| `#7 ok item={"peer":"10.0.0.1","state":"established"}` | one result record |
| `#7 ok fault={"path":"bgp/peer/10.0.0.2","message":"nexthop unreachable"}` | one error record; the walk continues |
| `#7 ok count=2` | terminator; two records produced, none faulted |
| `#7 ok count=0` | the command succeeded and produced no records |
| `#7 ok count=0 message=peer 10.0.0.1 not configured` | understood, executed, failed before any record |
| `#7 ok count=97 faults=3` | 97 applied, 3 rejected |
| `#7 ok count=417 message=rib snapshot expired` | the walk aborted after 417 records with no faulted record |
| `#7 error message=unknown command: shwo bgp peers` | not understood; re-sending is pointless |

Four rules make the tail parseable without a JSON decoder, without quoting, and without lookahead:

| Rule | Consequence |
|------|-------------|
| the head is the first line for an id; the terminator is the line carrying `count=` | head and terminator are distinguished by a key, not by position alone, so a reader needs no lookahead and no state beyond "have I seen the head" |
| `item=`, `fault=` and `message=` take the rest of the line verbatim and are last | at most one open-ended value per line; a JSON value containing `=` or a space needs no escaping |
| the verdict is a key, never a payload field | no consumer parses JSON to decide how to read the answer |
| nothing declares the record count or the answer shape | there is no declaration for the payload to contradict |

### Status semantics

| Term | Answers | Values |
|------|---------|--------|
| verb | was the conversation valid, was the command understood | `ok`, `error` |
| `status=` on the head | what the daemon knows at open time | `done`, `error` |

**Head status is mandatory**, because a consumer that renders failures differently commits to a rendering on the first line. Without it, that consumer must buffer the whole answer to decide how to render the first record, which removes the benefit this work exists to deliver.

**The terminator states no status. The verdict is DERIVED from the counts it already carries** (owner decision, 2026-08-19). A stated status would be a second source of truth for a fact the counts already hold, and the two can disagree; deriving it makes disagreement unrepresentable rather than merely tested for.

| Terminator | Derived verdict |
|------------|-----------------|
| `count=N faults=0` | done |
| `count=N faults=M`, N greater than 0 | partial |
| `count=0 faults=M` | error, nothing succeeded |
| any `message=` present | the walk aborted; `count=` states how far it got |
| no terminator | truncated |

`message=` is what makes an aborted walk expressible when no record faulted: `count=417 message=rib snapshot expired` is neither done nor partial, and the counts alone cannot say so.

### How a fault renders on the buffered path

A `fault=` record has no place on the wire's item array, but the 24 `CommandDispatcher.JSON` consumers (web, MCP, REST, gRPC, looking glass) read only the buffered form, so refusing to render a faulted answer would leave them showing nothing at all for a partial result. The collapse therefore puts faults under a SIBLING key:

| Answer | Buffered rendering |
|--------|--------------------|
| rows only | `{"peers":[...]}` — unchanged, no new key appears |
| rows and faults | `{"peers":[...],"errors":[...]}` |
| faults, no envelope key | `{"data":[...],"errors":[...]}` |

`errors` is emitted ONLY when a row faulted, so an ordinary answer's shape never changes and no existing consumer meets a key it has not seen. `count=` still counts items alone and `faults=` counts faults, so the two collections stay separately countable on both paths and `| count` cannot disagree with the terminator. A handler naming its own envelope `errors` is refused on both paths, because two collections under one key means one overwrites the other.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The frame layer needs no change to carry a `key=value` tail | `ParseLine` (`pkg/plugin/rpc/message.go`) cuts the verb at the first space and returns the remainder unsplit | The migration grows to include the frame reader, and every plugin breaks at once rather than at the payload boundary | A unit test feeding a `key=value` tail through `ParseLine` and asserting the payload arrives whole | **confirmed, 2026-08-19.** `TestParseLineCarriesKeyValueTailWhole` passes against an unedited `ParseLine`, over a head, an `item=` holding `=` and spaces, a terminator, and a not-understood line |
| A-2 | Nothing today sets or reads `Response.Partial` | The field exists in `internal/component/plugin/types.go` described as "true for streaming chunks"; no producer was read | An existing streaming consumer exists and this spec collides with it | Grep for writes to `.Partial` across `internal/` and `pkg/`, and read every producer found | **broken, 2026-08-19** — see A-7 |
| A-7 | The server-side pending registry needs the same lifetime change as `MuxConn` | Assumed from `MuxConn.pending` using `LoadAndDelete`; the server registry was not read | Phase 4 is scoped wrongly and rebuilds a mechanism that already exists | Read `internal/component/plugin/server/pending.go` | **broken, 2026-08-19.** `PendingRequests.Partial` already keeps the entry, resets the timeout, and delivers on `RespChan`, and it has zero non-test callers. The FIELD `Response.Partial` is dead; the METHOD is a complete, unused mechanism. Phase 4 therefore narrows to `MuxConn.pending`, and the server side gains a PRODUCER rather than a lifetime change |
| A-3 | The 24 dispatcher consumers can each take the buffering path with a one-call edit | They call `CommandDispatcher.JSON` and consume `RenderedResponse.Output` as a string | Web, MCP and looking-glass need per-surface streaming work, multiplying the scope | Read each call site and confirm it consumes `Output` as a whole string | unvalidated |
| A-4 | A NETCONF `rpc-error` shape fits Ze's existing error modelling for the `fault=` payload | Ze is YANG-modelled throughout; the error modelling itself was not read | The `fault=` payload needs a Ze-specific shape and gains nothing from the alignment | Read the YANG error modelling and the existing error payload producers before fixing the shape | unvalidated |
| A-5 | An operator wants a distinct exit code for a derived partial verdict | Inferred from the goal, not stated by the owner | A script branching on the new code breaks when the owner picks a different mapping | Owner decision recorded in Key Design Decisions before implementation | unvalidated |
| A-6 | Three lines for a one-record answer is an acceptable wire cost on the plugin connection | Owner decision that one code path outranks line economy, 2026-08-19. Command answers are at operator rates; `deliver-event` and `deliver-batch` are requests, not answers, so they do not pay it | A high-rate answer path exists that this triples | Count answer lines per second on the plugin connection under the existing benchmark before and after | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `readLoop` routes a response with `pending.LoadAndDelete(idStr)`, so the id is gone after the head and every later line is orphaned. Every plugin RPC goes through that map | Any plugin RPC returns only the head, or hangs waiting for records | **closed 2026-08-19.** `routeResponse` (`pkg/plugin/rpc/mux.go`) keys on the entry type: a `chan []byte` is removed as its one line is routed, an `*answerCall` lives until `endAnswer`. `TestMuxConnDeliversEveryRecordToOneCaller` and `TestSingleResponseCallRPCPathUnchanged` hold the two halves |
| R-2 | An unmatched line takes `readLoop`'s orphaned-response branch, which increments `consecutiveBad`; past 100 the reader stores an error and closes a live plugin connection | A plugin connection drops shortly after a cancelled or abandoned answer | **closed 2026-08-19.** `discardOrphanResponse` (`pkg/plugin/rpc/mux.go`) counts an unmatched line only when it does not read as an answer tail. `TestOrphanRecordDoesNotCloseConnection` drives 150 orphaned records and the connection lives; `TestOrphanJunkStillClosesConnection` drives 150 orphaned JSON responses and it still closes |
| R-3 | Cancellation does not reach the generator, so `first 10` leaves the daemon walking a RIB nobody reads | CPU stays high after a short command returns | **CLOSED, phase 6.** `recordsFirst` (`internal/component/command/pipe_records.go`) returns out of its `for record := range records` loop, which makes the producing `yield` report false and stops the walk. No context is needed: range-over-func propagates the stop. `TestFirstNStopsTheGenerator` holds it, and reverting the return to a drain fails it with "generator produced 1000 rows, want 10" |
| R-4 | `parseRPCError` treats a non-JSON payload as the message text, so a `message=...` tail decodes as `Message: "message=..."` | An error message reaches the operator with a literal `message=` prefix | Update `parseRPCError` in the same change that introduces the tail, and pin the decoded text in a test |
| R-5 | `readLoop` sends the routed body on a `chan []byte`; a stream needs that channel to accept many sends without the reader goroutine blocking behind a slow consumer | The plugin reader goroutine stalls, and every other id's traffic stops with it | **closed 2026-08-19.** `routeAnswerLine` (`pkg/plugin/rpc/mux.go`) sends into a 256-line queue and never waits: a full queue ends that one answer with `ErrAnswerQueueFull`, which the consumer reads as a truncated verdict and a stated error. `TestSlowConsumerDoesNotStallReadLoop` fails under a blocking send, measured: the second caller times out at 20s |
| R-6 | `table` and `text` need every row for column widths, so they buffer and the memory win does not reach the default format | Memory profile unchanged for a default-format command | **CLOSED as an accepted limit, phase 6.** The buffering is paid once in `applyTableStyled` and declared in the `case pipeTable, pipeText:` arm of `applyRecordOp`, so it is a stated cost rather than an accident. `TestTableBuffersAndSaysSo` pins it. The win is MEASURED for the rest: over 4000 records of 8 KiB (32.7 MB), `\| count` held 0 bytes of heap and `\| last 8` held 46,112, against 38 MB for a collecting implementation |
| R-10 | `display` is credited with the streaming win, but `applyDisplaySelect` (`internal/component/command/pipe_columns.go`) cannot deliver it: it runs `json.NewDecoder(...).Decode(&data)` over the WHOLE payload, calls `selectFields`, and re-marshals — string in, string out. That is the exact pattern the Task section names as the problem, so the operator table would be promising a win the code does not produce | A memory profile for `\| display` that matches the whole-payload path rather than O(1) | **CLOSED, phase 6.** `applyDisplaySelect` now routes through `selectSequence`/`selectEnvelope`/`selectArray`/`selectElement`, and `selectFields`'s array arm calls the same `selectElement`, so one spelling serves the record path and the buffered path. `TestDisplaySelectsPerRecord` holds it; making `selectSequence` always answer not-a-sequence fails it with "was not read as a sequence, so the whole payload was decoded" |
| R-7 | Two ledgers of the same answer drift: the streamed form and the buffered form stop producing identical JSON | A consumer sees different data depending on which path it took | The reassembly-equivalence test is the control, and it must be shown to fail when streaming is reverted |
| R-8 | A one-record answer now costs three lines, tripling answer traffic on the plugin connection | Answer throughput regression in the existing benchmark | A-6 measures it before the change lands; if it bites, the head and terminator can carry the single record without changing the reader's one path |
| R-9 | `PendingRequests.Partial` and `PendingRequests.Complete` both deliver with a non-blocking send whose `default:` arm DROPS the response when `RespChan` is full. For a single final response that loses one answer; for a record sequence it loses rows silently, with no error, no counter and no log, which makes the stream wrong rather than slow | A record count that disagrees with the terminator's `count=`, reproducible under a slow consumer | **closed 2026-08-19.** `deliver` (`internal/component/plugin/server/pending.go`) waits up to the request's own timeout for room and returns `ErrPendingUndeliverable` when it expires. `Complete` and `Partial` return `error` rather than a found-bool, so a caller cannot ignore a row that did not arrive, and the three notice paths (`Add` at its limit, `timeout`, `CancelAll`) log what they could not deliver. `TestPartialWaitsForASlowConsumerRatherThanDropping` and `TestPartialReportsAResponseItCannotDeliver` hold both halves |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every plugin RPC on the shared `MuxConn`, and every command answer on every surface. A wrong `pending` lifetime hangs the daemon's plugin dispatch |
| How is it reverted? | Single commit revert while the shape is negotiated and off by default. Once a peer has negotiated it, a revert leaves that peer waiting for a terminator that never arrives |
| Who else touches this path? | Both neighbours on this pipe surface have since closed: `spec-cli-pipe-aliases` in `eda7ad83c`, and `spec-fixit-cli-format-default-everywhere` closing 2026-08-19. This spec now owns the pipe surface alone |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze cli -c "show bgp peer list"` over the SSH exec channel | → | the encoder writing head, records, terminator | `test/plugin/answer-many-records.ci` |
| `ze cli -c "show bgp peer list"` against a single peer | → | the same encoder path, one record | `test/plugin/answer-single-record.ci` |
| A handler answering with a row generator | → | the generator `ResponseData` implementor | `TestGeneratorAnswerReachesTheEncoder` |
| A plugin RPC answering with records | → | `MuxConn` routing many lines to one waiting caller | `TestMuxConnDeliversEveryRecordToOneCaller` |
| `show bgp peer list \| first 10` | → | the record-at-a-time pipe path and generator cancellation | `TestFirstNStopsTheGenerator` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any successful command, to a negotiated peer | A head carrying `status=` and `type=`, then its records, then a terminator carrying `count=`; the reader follows one path whatever the record count and whatever the type |
| AC-1b | A walk that ends at or under the threshold | `type=json`, one `item=` carrying the whole document, and that document is byte-identical to what the command answers today |
| AC-1c | A walk that passes the threshold with a fixed schema | `type=stream`, `fields=` on the head, one positional `item=` per row, and the records held during buffering are flushed in order before the rest |
| AC-2 | A command producing exactly one record | Head, one `item=` line, terminator with `count=1`; no special case in the reader. Proven by driving ONE command (`show bgp peer list`) against one peer and against two: a command that answers a fixed single record could not distinguish the shared path from a special case |
| AC-3 | A command that returns no data | Head, no records, terminator with `count=0`; the operator still sees `OK` when the chain names no format |
| AC-4 | A command text that is not a command | Verb `error` with `message=`, no `status=` key, and the answer is the only line |
| AC-5 | A command understood but failing before any record | Head `status=error`, terminator `count=0` carrying the operational `message=` |
| AC-6 | A generator of N rows | N `item=` lines and a terminator carrying `count=N`; no count appears before the terminator |
| AC-7 | A generator whose walk aborts after K rows with no faulted record | K `item=` lines already written, then a terminator carrying `count=K` and `message=`; the consumer reads it as aborted, not as done |
| AC-8 | A generator producing both results and per-record errors | `item=` and `fault=` lines interleaved, terminator carrying `count=` and `faults=` both non-zero, which derives to partial |
| AC-9 | An answer whose connection dies before the terminator | The consumer reports truncation rather than treating the records received as a complete answer |
| AC-10 | The same answer taken through the streamed path and the buffered path | Both marshal to identical JSON |
| AC-11 | A record whose JSON value contains `=` and spaces | The record round-trips with no escaping and no quoting |
| AC-12 | Every terminator shape in the derivation table | The verdict a consumer computes matches the table, and no `status=` key is written on a terminator |
| AC-13 | A peer that has not negotiated the new shape | Receives `#<id> ok [<json>]` exactly as today |
| AC-14 | `show bgp peer list \| first 10` against a large answer | The generator stops after ten rows; the remaining rows are never produced |
| AC-15 | A single record larger than `MaxMessageSize` | Refused with a fault naming the record, and the answer continues to its terminator |
| AC-16 | A record line arriving for an id with no waiting caller | Discarded without incrementing the counter that closes the connection at 100 |
| AC-17 | A consumer that stops reading mid-answer | `readLoop` does not block; other ids on the same connection keep flowing |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs a command whose answer is one record | exec channel → dispatcher → encoder → operator | `TestSingleRecordAnswerReachesOperator` |
| 2 | Runs a command over a large table with `\| first 10` | exec channel → generator → record pipe → cancellation | `TestFirstNStopsTheGenerator` |
| 3 | Commits a config where 3 of 100 leaves are invalid | exec channel → generator → `fault=` records → terminator deriving partial | `TestPartialApplyReportsBothCounts` |
| 4 | Mistypes a command | exec channel → dispatcher lookup miss → `error message=` | `TestUnknownCommandAnswersErrorVerb` |
| 5 | Loses the connection mid-answer | exec channel → records → no terminator | `TestMissingTerminatorReportsTruncation` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseLineCarriesKeyValueTailWhole` | `pkg/plugin/rpc/message_test.go` | A-1: the tail arrives unsplit | green |
| `TestTailTokenizerNeedsNoJSONDecoder` | `pkg/plugin/rpc/message_test.go` | control keys read without parsing a payload | green |
| `TestOpenEndedKeyRunsToEndOfLine` | `pkg/plugin/rpc/message_test.go` | AC-11 | green |
| `TestTerminatorIsTheLineCarryingCount` | `pkg/plugin/rpc/message_test.go` | head and terminator distinguished with no lookahead | green |
| `TestVerdictDerivesFromTheCounts` | `pkg/plugin/rpc/message_test.go` | AC-12, every row of the derivation table | green |
| `TestTerminatorCarriesNoStatusKey` | `pkg/plugin/rpc/message_test.go` | AC-12, the single source of truth holds | green |
| `TestVerbPredicateUnchangedForResponses` | `pkg/plugin/rpc/mux_test.go` | `readLoop`'s request-versus-response split is untouched | green |
| `TestMuxConnDeliversEveryRecordToOneCaller` | `pkg/plugin/rpc/mux_test.go` | R-1 | green |
| `TestOrphanRecordDoesNotCloseConnection` | `pkg/plugin/rpc/mux_test.go` | AC-16, R-2 | green |
| `TestOrphanJunkStillClosesConnection` | `pkg/plugin/rpc/mux_test.go` | the flood guard still fires for an orphan that is not an answer line | green |
| `TestSingleResponseCallRPCPathUnchanged` | `pkg/plugin/rpc/mux_test.go` | AC-13, the one-response path and its entry lifetime | green |
| `TestAnswerWithoutTerminatorReportsTruncation` | `pkg/plugin/rpc/mux_test.go` | AC-9 at the mux boundary | green |
| `TestNotUnderstoodAnswerReachesTheCaller` | `pkg/plugin/rpc/mux_test.go` | AC-4 at the mux boundary | green |
| `TestPartialWaitsForASlowConsumerRatherThanDropping` | `internal/component/plugin/server/pending_test.go` | R-9, backpressure rather than a drop | green |
| `TestPartialReportsAResponseItCannotDeliver` | `internal/component/plugin/server/pending_test.go` | R-9, an undeliverable row is reported | green |
| `TestSlowConsumerDoesNotStallReadLoop` | `pkg/plugin/rpc/mux_test.go` | AC-17, R-5 | green |
| `TestParseRPCErrorReadsMessageKey` | `pkg/plugin/rpc/message_test.go` | R-4 | green |
| `TestGeneratorAnswerReachesTheEncoder` | `internal/component/plugin/dispatch_test.go` | AC-6 | green |
| `TestSingleRecordUsesTheSameReaderPath` | `internal/component/plugin/dispatch_test.go` | AC-2, the one-path property | |
| `TestFaultDoesNotEndTheWalk` | `internal/component/plugin/dispatch_test.go` | AC-8 | |
| `TestTransportErrorEndsTheWalk` | `internal/component/plugin/dispatch_test.go` | the Go error slot is transport-only | |
| `TestStreamedAndBufferedAnswersAreIdentical` | `internal/component/plugin/dispatch_test.go` | AC-10, R-7 | |
| `TestFirstNStopsTheGenerator` | `internal/component/command/pipe_test.go` | AC-14, R-3 | |
| `TestCountConsumesWithoutBuffering` | `internal/component/command/pipe_test.go` | O(1) memory for `count` | |
| `TestLastNKeepsRingBufferOnly` | `internal/component/command/pipe_test.go` | O(N) memory for `last` | |
| `TestTableBuffersAndSaysSo` | `internal/component/command/pipe_table_test.go` | R-6, the limit is deliberate | |
| `TestRecordOverMaxMessageSizeFaults` | `pkg/plugin/rpc/framing_test.go` | AC-15 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| record size | 1 - 16 MB | 16777216 | 0 | 16777217 |
| `count=` in the terminator | 0 - max uint64 | max uint64 | N/A | overflow rejected |
| `first N` / `last N` | 1 - max int | max int | 0 | N/A |
| consecutive orphaned lines | 0 - 100 | 100 | N/A | 101 closes the connection |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `answer-single-record` | `test/plugin/*.ci` | a one-record command answers through the same path as a large one | |
| `answer-many-records` | `test/plugin/*.ci` | a large answer arrives record by record | |
| `answer-partial-apply` | `test/plugin/*.ci` | 97 applied, 3 rejected, both counts reported | |
| `answer-unknown-command` | `test/plugin/*.ci` | a mistyped command answers with the `error` verb | |
| `answer-truncation-detected` | `test/plugin/*.ci` | an answer cut before its terminator is reported as truncated | |
| `answer-not-negotiated` | `test/plugin/*.ci` | a peer that did not negotiate sees today's frame | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | This is Ze's own management wire, not a protocol another daemon speaks. No external implementation consumes it. The plugin SDK is the second implementation, and `answer-not-negotiated` is the compatibility proof | |

## Files to Modify
- `pkg/plugin/rpc/message.go` - tail tokenizer, `AppendResult` and `AppendError` writing the new keys, `parseRPCError` reading `message=`
- `pkg/plugin/rpc/mux.go` - pending entry lifetime, orphan-record handling, consumer backpressure
- `pkg/plugin/rpc/types.go` - the answer and record types the caller consumes
- `internal/component/plugin/types.go` - the generator `ResponseData` implementor
- `internal/component/plugin/dispatch.go` - `ResponseJSON` gaining the record path
- `internal/component/ssh/ssh.go` - `execMiddleware` writing lines as produced
- `internal/component/command/pipe.go` - the record-at-a-time path
- `internal/component/command/pipe_table.go` - the declared buffering path
- `internal/component/command/pipe_columns.go` - `applyDisplaySelect` rewritten record-by-record (R-10); it is the one operator credited with the streaming win whose current code is whole-payload
- `internal/component/command/pipe_records.go` - CREATED in phase 6: `ApplyPipesRecords` and `applyRecordOp`, the record-at-a-time half of the chain. The phase-1 stub was MOVED here out of `pipe.go`, not copied (`ai/rules/no-layering.md`)
- `internal/core/ssh/client/client.go` - a record-reading client call beside `ExecCommand`
- `internal/component/cli/client/main.go` - the exec path consuming records
- `docs/architecture/api/ipc_protocol.md` - the wire grammar's durable home; the `// Design:` target of both `message.go` and `mux.go`, and it shows an answer example that this replaces
- `docs/architecture/api/wire-format.md` - shows the `#1 ok {"peers":[...]}` answer shape being replaced
- `docs/architecture/api/process-protocol.md` - the Success row and the answer examples in the startup timeline
- `docs/architecture/api/commands.md` - the answer contract
- `docs/plugin-development/protocol.md` - the success-response row a plugin author reads
- `docs/architecture/system-architecture.md` - the declared `// Design:` document of both `internal/component/ssh/ssh.go` and `internal/core/ssh/client/client.go`, the two ends of the exec channel this changes

## Files to Create
- `test/plugin/answer-*.ci` - the six functional tests above
- `plan/deferrals/streaming-answer-protocol.md` - the deferral shard

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new command or config leaf; this changes how existing answers are framed |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | No | No new verb; every existing command gains the framing |
| CLI grammar (keyword before value) | N-A | No new command surface |
| Editor autocomplete | N-A | No new leaf or value |
| Functional test for new RPC/API | Yes | `test/plugin/answer-*.ci`, six scenarios |
| Pipe completeness | Yes | `internal/component/command/pipe.go`; every operator keeps working, and each declares whether it streams or buffers |
| Env var registration | Yes | A negotiation default needs `ze.*` registration if the owner wants it configurable rather than always negotiated |
| Doctor check for runtime dependencies | No | No new file path, socket, port, module, or binary |
| Prometheus counters/metrics | Yes | Records answered and answers truncated are observable state worth counting; names fixed at implementation |
| BGP family surface | N-A | No SAFI, capability, or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | No | No config leaf changes |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, for the exit-code mapping |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | No | No plugin gained or lost a command |
| 6 | Has a user guide page? | No | The framing is not an operator-facing topic of its own |
| 7 | Wire format changed? | Yes | `docs/architecture/api/process-protocol.md` is the wire doc for this surface |
| 8 | Plugin SDK/protocol changed? | Yes | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Ze's own management wire; no RFC governs it |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, for the record-reading `.ci` shape |
| 11 | Affects daemon comparison? | No | Not a comparison axis |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, command, capability, or inventory changed? | Yes | The negotiated capability is new inventory |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Ten documents carry the wire format. Five state the ANSWER shape and change: `ipc_protocol.md` (the `Design:` target of `message.go` and `mux.go`), `wire-format.md`, `process-protocol.md`, `commands.md`, `plugin-development/protocol.md`. Five state only the framing `#<id> <verb> [<json>]`, which stays true because requests are untouched: `plugin-development/commands.md`, `plugin-overview.md`, `DESIGN.md`, `why-ze.md`, `api/architecture.md`. Re-grep at implementation and confirm the split still holds |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/api/process-protocol.md` shows `#1 ok {...}` examples that become one of two valid shapes |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - prove head, record, terminator reaches an operator before anything is optimized
   - Tests: the four Wiring Test rows, all failing
   - Files: `pkg/plugin/rpc/message.go`, `internal/component/plugin/types.go`, `internal/component/ssh/ssh.go`
   - Verify: the entry point exists and the wiring tests fail because the encoder is a stub
2. **Phase: Tail grammar** - tokenizer, writers, and the four structural rules
   - Tests: `TestParseLineCarriesKeyValueTailWhole`, `TestOpenEndedKeyRunsToEndOfLine`, `TestTerminatorIsTheLineCarryingCount`, `TestVerdictDerivesFromTheCounts`, `TestTerminatorCarriesNoStatusKey`, `TestParseRPCErrorReadsMessageKey`
   - Files: `pkg/plugin/rpc/message.go`
   - Verify: A-1 confirmed or broken before any consumer is touched
3. **Phase: Negotiation** - the new shape is off until both sides agree, so every existing plugin keeps working
   - Tests: `answer-not-negotiated`, AC-13
   - Files: `pkg/plugin/rpc/types.go`, the startup capability declaration
   - Verify: an un-negotiated peer sees a byte-identical frame
4. **Phase: MuxConn lifetime and backpressure** - the highest-risk change, taken alone
   - Tests: `TestMuxConnDeliversEveryRecordToOneCaller`, `TestOrphanRecordDoesNotCloseConnection`, `TestSlowConsumerDoesNotStallReadLoop`, `TestVerbPredicateUnchangedForResponses`
   - Files: `pkg/plugin/rpc/mux.go`
   - Verify: R-1, R-2 and R-5 closed; the single-response path unchanged
5. **Phase: Generator answers** - the producer side
   - Tests: `TestGeneratorAnswerReachesTheEncoder`, `TestSingleRecordUsesTheSameReaderPath`, `TestFaultDoesNotEndTheWalk`, `TestTransportErrorEndsTheWalk`, `TestStreamedAndBufferedAnswersAreIdentical`
   - Files: `internal/component/plugin/types.go`, `internal/component/plugin/dispatch.go`
   - Verify: AC-10 holds, and it fails when the record path is reverted
6. **Phase: Pipe consumption and cancellation** - the memory win
   - Tests: `TestFirstNStopsTheGenerator`, `TestCountConsumesWithoutBuffering`, `TestLastNKeepsRingBufferOnly`, `TestTableBuffersAndSaysSo`, `TestDisplaySelectsPerRecord`
   - Files: `internal/component/command/pipe.go`, `pipe_table.go`, `pipe_columns.go`
   - Verify: R-3 closed; the walk stops when the reader stops. R-10 closed: `applyDisplaySelect` no longer decodes the whole payload, and selection happens on one record at a time
7. **Phase: Exec channel and client** - the operator-visible half
   - Tests: the six `.ci` scenarios, the five user stories
   - Files: `internal/component/ssh/ssh.go`, `internal/core/ssh/client/client.go`, `internal/component/cli/client/main.go`
   - Verify: exit codes distinguish the `error` verb, a head `status=error`, and a derived partial verdict
8. **Phase: Documentation** - the 12 answered rows above

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | All five user stories have a working path |
| Correctness | One reader path: no branch anywhere on how many records an answer carries |
| Correctness | The terminator carries no `status=` key, so the verdict has exactly one source: the counts |
| Correctness | A truncated answer is reported as truncated, never as a short answer |
| Naming | `status=` values match `Response.Status`'s existing `done`/`error`/`partial` vocabulary rather than introducing a second spelling |
| Data flow | The pipe chain still runs in the daemon; no formatting moves back to the client |
| Rule: `ai/rules/cli.md` | A record payload is still structured data, so `json`, `yaml` and `table` remain three renderings of one payload |
| Rule: `ai/rules/no-layering.md` | The whole-string path is deleted where the record path replaces it, not left beside it |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Tail tokenizer needing no JSON decoder | `TestTailTokenizerNeedsNoJSONDecoder` passes |
| One reader path for any record count | `TestSingleRecordUsesTheSameReaderPath` passes |
| Un-negotiated peers see today's frame | `answer-not-negotiated` passes |
| Streamed and buffered answers agree | `TestStreamedAndBufferedAnswersAreIdentical` passes, and fails when the record path is reverted |
| Generator cancellation | `TestFirstNStopsTheGenerator` passes |
| Wire grammar documented in its durable home | `grep -n "count=" docs/architecture/api/ipc_protocol.md` |
| No document still shows the replaced answer shape | `grep -rn "ok {" docs/` returns no answer example carrying a bare JSON payload |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A tail key repeated on one line, an unknown key, and a `status=` value outside `done`/`error`/`partial` each need a defined answer rather than a silent default |
| Frame injection | An open-ended value is the rest of the line, so a `\n` or `\r` inside one would end the frame early and let the remainder be read as a forged line — a `message=` could otherwise carry its own terminator and lie about `count=`. `replaceNewlines` substitutes a space before the value is written. `item=` and `fault=` carry `json.Marshal` output, which escapes newlines and cannot reach this, so the defense is load-bearing for `message=` and belt-and-braces for the other two |
| Resource exhaustion | An answer with no terminator holds a `pending` entry open; entries need a bound or a timeout, or a malicious plugin leaks them |
| Resource exhaustion | An unbounded generator with a slow reader needs backpressure, not an unbounded queue (R-5) |
| Error leakage | `fault=` payloads carry paths and messages to an operator; they must not carry internal state the command surface does not otherwise expose |
| Authorization | Authorization is decided once at dispatch; an answer must not outlive the authorizer generation accepted with the request |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A plugin RPC hangs after phase 4 | Stop. R-1 or R-5 is realized; revert phase 4 and re-derive the pending lifetime |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- Making every answer a record sequence removes a risk class rather than adding cases. With nothing declaring the shape or the count, there is no declaration for the payload to contradict, which is the failure recorded twice in `plan/journal/declared-format-contradicts-payload.md`.
- The envelope already anticipated this work and nothing was built on it: `Response.Status` carries `done`/`error`/`partial` and `Response.Partial` is documented as "true for streaming chunks". A-2 exists to find out whether that is vocabulary or a live mechanism.
- The frame layer is accidentally ready. `ParseLine` returning the tail unsplit means the migration is a payload-interpretation change, not a framing change, which is why negotiation can be a capability rather than a version bump.
- Keeping the `error` spelling is not conservatism: `readLoop` computes `isResponse` from the verb, so renaming it would move every plugin's inbound dispatch for no semantic gain.
- Separating the verb from `status=` pays for itself independently of streaming. It is the difference between "offer completion" and "show the operational error", which the CLI cannot express today at any answer size.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Every answer is a record sequence; a single element is one record | A `framing=line` / `framing=stream` declaration selecting between two shapes | One code path. Nothing declares the shape, so nothing can contradict it, and the reader never branches on record count. Owner decision, 2026-08-19 |
| Control fields as bare `key=value` | Control fields inside the JSON payload | A reader must not parse JSON to decide how to read the answer. Owner decision, 2026-08-19 |
| One verb plus keys | Six verbs: `ok`, `error`, `begin`, `item`, `fault`, `end` | One parse path instead of six, and `readLoop`'s response predicate stays untouched. Owner decision, 2026-08-19 |
| Head status mandatory | Deferring the verdict to the terminator | A consumer that renders failures differently would have to buffer the whole answer to render the first record. Owner decision, 2026-08-19 |
| `error` verb keeps its spelling, narrowed in meaning | Renaming it `err` | `error` is compared by `readLoop` to split responses from requests; renaming breaks every plugin for no gain once `status=` carries execution failure |
| Terminator identified by `count=` | Identified by position alone | A key-based test needs no reader state beyond "have I seen the head", and it survives a reordering bug rather than silently mis-reading |
| The terminator states no status; the verdict is derived from `count=`, `faults=` and `message=` | Repeating `status=` on the terminator, with a monotonicity rule forbidding it to improve on the head | A stated status is a second source of truth for what the counts already hold, and two ledgers of one fact drift. Deriving makes the disagreement unrepresentable instead of merely tested for, and deletes the monotonicity invariant entirely. Owner decision, 2026-08-19 |
| A record is one line, bounded by `MaxMessageSize` | Allowing a record to span lines | The bound moves from the whole answer to one record, which is the win; a record that cannot fit is a design error worth a fault |
| `table` and `text` buffer | Declared column widths, or a two-pass render | Column widths need every row. Declaring widths adds an option nobody asked for, and a human reading a table does not want a million rows |

## Known Limitations
- A one-record answer costs three lines instead of one. A-6 measures the cost and R-8 carries the fallback.
- `table` and `text` buffer the whole answer. The memory win applies to `json`, `ndjson`, `yaml`, `match`, `first`, `count`, and to `display` only after Phase 6 rewrites `applyDisplaySelect` record-by-record (R-10).
- `| fill` was REMOVED rather than carried (owner decision, 2026-08-19). `fill overall` ordered columns by measuring every cell in the result, a deeper buffering dependency than `table`'s own, and R-6 named only `table` and `text`. Widening the exception to keep a day-old operator was the worse trade.
- REST, gRPC, web, MCP and looking-glass take the buffering path in this spec. Their record-level streaming is separable future work and belongs in its own spec, not in the deferral shard.
- A record larger than `MaxMessageSize` is refused rather than split.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-17 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes
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
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
