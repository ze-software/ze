# Spec: record answers child 2 -- the only command-answer encoding, at fixed offsets

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | spec-record-answers-1-sdk-path |
| Phase | - |
| Deferral shard | `plan/deferrals/record-answers.md` |
| Handoff | - |
| Updated | 2026-08-21 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Child 2 of three. Child 1 gives the SDK a record producer and reader, which this
spec requires: deleting a frame no peer can replace would leave every plugin
unable to answer. Child 3 (`spec-record-answers-3-zero-alloc`) then removes the
per-row allocations behind the frame this spec fixes.

## Task

Two problems, and they are fixed in one pass because they rewrite the same lines
and the same fixtures.

**The frame is negotiated and nothing declares it.** `answerResult` branches on
`Process.RecordAnswers()`, and the SSH exec channel branches on
`declaresRecordAnswers` reading `ZE_ANSWER_PROTOCOL`. A negotiated shape means
two encodings of one answer live side by side, which `ai/rules/no-layering.md`
forbids and which `ai/rules/go-standards.md` already answers: Ze has never been
released, so no compat path is owed anywhere, the plugin API included. The
negotiation exists only because the record shape arrived second.

**The line cannot be read at a fixed offset.** Three fields vary in width, so a
reader scans for spaces before it knows anything. `appendAnswerID` writes `#`
plus one to twenty decimal digits plus a space, or nothing at all on the exec
channel. The verb is `ok` (two bytes) or `error` (five). The tail key runs from
`key` (three bytes) to `message` (seven). Worse, the line does NOT say what kind
of line it is: a reader learns whether it holds a head, a record, a fault or a
terminator only by parsing the tail and asking `AnswerTail.IsTerminator`, so the
hot line is scanned twice.

**The head states an outcome the terminator already owns.** `answerStatus`
returns `StatusError` only when the response itself is an error, which means
zero records, so `status=error` only ever appears on a two-line answer whose
terminator immediately follows. It is blind to a partial, to a walk aborted
part way, and to any failure after the head was written. `Verdict` derives the
outcome from the terminator alone and says why in its own comment: a terminator
carries no `status=`, so nothing can disagree with the counts. The head's
status is the one field that can. Its comment claims it lets a consumer commit
to a rendering on the first line, and on the SSH exec channel that is not even
possible, because `answerFrame.head` is written after the body.

The goal is one encoding with one field vocabulary. Every field on every line is
a three-byte word closed by a space, a counted number, or a counted text, so a
reader reaches every field by arithmetic and no line states a fact another line
can contradict.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/ipc_protocol.md` - the Answer Protocol grammar, its negotiation, and the line table
  → Decision: a later shape earns a NEW name in `protocol`; this spec removes the mechanism that made a name necessary, so the paragraph stating it must be rewritten rather than left standing
  → Constraint: only `dispatch-command` and `dispatch-command-args` take the answer path; the startup RPCs and every typed engine op keep the single-line frame, so deleting it wholesale is wrong
- [ ] `docs/architecture/api/wire-format.md` - the line grammar shared by requests and responses
  → Constraint: `#<id> <verb> [<json>]` is the shape of a REQUEST as well; a fixed-width id must apply to every line carrying one, or the protocol carries two id encodings
- [ ] `ai/rules/no-layering.md` - when replacing X with Y, delete X first
  → Constraint: the single-line command-answer path is deleted in the same commit that makes the record path unconditional, never left behind a flag
- [ ] `ai/rules/performance.md` - buffer ownership, and what a hot path may not do
  → Constraint: the record line is the line that repeats; every byte and every scan removed from it is multiplied by the walk length
- [ ] `docs/contributing/ze-style.md` - a limit on everything, types that cannot lie
  → Constraint: a fixed-width field states its own bound, so `answerRecordPrefixMax` stops being a maximum and becomes `answerRecordPrefixWidth`, the exact prefix size

**Key insights:** (minimal context to resume after compaction)
- Fixing the verb width alone buys nothing: the variable id in front of it moves every later offset.
- The kind token replaces both the verb AND the `item=` / `fault=` key, because a kind already says which payload follows. That is the whole win on the hot line.
- Two field shapes, and nothing else: a three-byte word closed by a space, and a counted value. A counted NUMBER is decimal digits closed by a space or by the end of the line. A counted TEXT is decimal digits, a colon, then that many bytes. Every line is built from those two, so one reader serves every line kind.
- Every token is a real word, never a truncation (`ai/rules/writing.md`, `docs/contributing/ze-style.md` on naming). Line kinds are `top`, `row`, `bad`, `end`, `nay`; item types are `doc`, `map`, `tab`. Byte 0 is distinct inside each field, so a machine switches on one load and a person reads a word.
- `doc` / `map` / `tab` say what an `item` IS -- one whole document, a map of names to values, a tabular row read against the head's columns -- rather than naming a serialization. That also ends the collision `ipc_protocol.md` apologises for, where `type=json` and the `| json` pipe operator share a word and mean different things.
- The head's `status=` is deleted, not shortened. `nay` covers the not-understood case, and the terminator's counts and message cover every other outcome.
- Every tail KEY leaves the wire with it. The head becomes positional, so `answerKeyStatus` through `answerKeyCode` and `ParseAnswerTail`'s key-dispatch loop all disappear.
- The id is `#42 `: the sigil, the decimal digits, and the one space that closes them. It is neither padded nor counted. A padded field would spend eighteen bytes on every line of a million-row walk, and a counted one measured slower than the fused loop that reads the plain form. The 2026-08-20 length prefix is REVERSED, and Key Design Decisions carries the measurement (owner, 2026-08-22).
- No field states an outer length. A digit run is closed by a byte no digit can be, a space for a number and a colon for a text, so a count in front of one buys a reader nothing. The base-36 length alphabet is deleted (owner, 2026-08-22).
- Offsets are computed per channel, not global: the mux channel carries the id and the exec channel writes `rpc.AnswerNoID` and no `#<id>` at all, so each has its own prefix rule and a reader knows which channel it is on at construction.
- `FormatResult` has no non-test caller. It is dead code that pins the old shape in a test.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `pkg/plugin/rpc/message.go` - `appendAnswerID` writes `#<id> ` in decimal or nothing for `AnswerNoID`; `appendAnswerPrefix` appends the literal `ok`; `appendAnswerKey` writes ` <name>=`; `answerRecordPrefixMax` bounds the prefix at 32 bytes; `ParseAnswerLine` and `ParseAnswerTail` read the tail as bare pairs; `AnswerTail.IsTerminator` derives the line kind; `FormatResult` has no non-test caller
- [ ] `pkg/plugin/rpc/types.go` - `ProtocolRecordAnswers`, `DeclareCapabilitiesInput.Understands`, `StatusOK`, `StatusDone`, `StatusError`
- [ ] `pkg/plugin/rpc/mux.go` - `MuxConn.readLoop` and `interpretResponse` split the line on the first space and take the rest as the payload
- [ ] `internal/component/plugin/dispatch.go` - `WriteAnswer`, `writeRecordAnswer`, `writeDocumentAnswer`, `writeRecordLine`, `answerStreamType`, `answerLineCapacity`

**CHILD 1 MOVED THESE, 2026-08-21. Read this before you plan any edit.** Child 1
made the Go SDK a first-class speaker of this protocol, and `pkg/plugin/sdk`
cannot import `internal/`. So the protocol's own code followed the protocol
package, in two forced moves, and the boundary that came out of it is clean: the
LINE ENCODING lives in `pkg/plugin/rpc`, and the engine's `*Response`-shaped
adapter stays in `internal/component/plugin`.

| Was | Is now |
|-----|--------|
| `internal/component/plugin/dispatch.go`: `writeRecordAnswer`, `writeDocumentAnswer`, `writeRecordLine`, `boundedRecord`, `answerStreamType`, `answerLineCapacity` | `pkg/plugin/rpc/answer_write.go`: `WriteRecordAnswer`, `WriteDocumentAnswer`, `writeDocumentLines`, `writeRecordLine`, `boundedRecord`, `answerRecordTooLargeFault`, `writeAnswerLine`, `marshalAnswerFields`, `AnswerStreamType` |
| `internal/component/plugin.CollapseRecords`, `internal/component/plugin/answer_row.go` | `pkg/plugin/rpc/collapse.go` (`CollapseRecords`, `AnswerErrorsKey`, `ErrEmptyAnswerRecord`, `ErrReservedEnvelopeKey`), `pkg/plugin/rpc/answer_row.go` | <!-- doc-links: ignore (the left column names the path this symbol MOVED OUT OF; it no longer exists, which is the row's point) -->
| still in `internal/component/plugin/dispatch.go` | `WriteAnswer`, `AnswerFor`, `ResponseJSON`, `RecordRows`, `answerRecords`, `answerStatus`, `answerMessage`, `documentAnswer`, `heldRecords`, `builtDocument` |

Two rows of this spec are affected in substance, not just in path. R-7's
"`writeDocumentAnswer` measures nothing" is now `WriteDocumentAnswer`
(`pkg/plugin/rpc/answer_write.go:149`), and the head's `status=` that phase 5
deletes is produced by `answerStatus`, which did NOT move.
- [ ] `internal/component/plugin/server/dispatch_registry.go` - `answerResult` is the branch between the record sequence and the single-line result; `opDispatchCommand` and `opDispatchCommandArgs` are its callers
- [ ] `internal/component/plugin/server/dispatch.go` - `responseToDispatchOutput` is the other encoding of a command answer
- [ ] `internal/component/plugin/process/process.go` - `RecordAnswers` and `SetRecordAnswers`
- [ ] `internal/component/plugin/server/startup.go` - Stage 3 reads `Understands`
- [ ] `internal/component/ssh/answer.go` - `declaresRecordAnswers`, `answerFrame`, `writeExecAnswer`, `writeExecRecords`, `writeExecDocument`, `writeExecFailure`
- [ ] `internal/core/ssh/client/answer.go` - `ExecCommandStream`, `readAnswerFrame`, `ErrAnswerTruncated` set `ZE_ANSWER_PROTOCOL`
- [ ] `internal/component/command/render_records.go` - `RenderRecords` and `streamsPerRecord` read the answer for the operator
- [ ] `test/plugin/answer-unconditional.ci` - asserts on the bytes a plugin declaring nothing receives; its premise disappears with the branch

**Behavior to preserve:**
- The data an operator sees. Every command's payload is unchanged; only the frame around it moves.
- The type decision: bounded walks stay one `json` document, and the threshold stays a constant, not a knob.
- `count` counts rows produced and `faults` counts rows rejected, and neither counts lines.
- A missing terminator still means truncation, and a fault still leaves the walk running.
- The single-line frame for the startup RPCs and for every typed engine op other than the two dispatch ops. Those never had a record shape and are out of scope.
- `Records.MarshalJSON` and `CollapseRecords` stay the one collapse for buffered surfaces.

**Behavior to change:**
- `ProtocolRecordAnswers`, `AnswerProtocolEnv`, `declaresRecordAnswers`, `Process.RecordAnswers`, `Process.SetRecordAnswers` and the `answerResult` branch are deleted. Every peer gets the record sequence.
- The id keeps its `#<id> ` spelling on every line that carries one, requests included: the sigil, the decimal digits, and the one space that closes them. Phase 3 put a base-36 length character in front of it and phase 6 deleted that character again, so the field ends this spec where it started.
- The verb on an answer line becomes a three-byte kind token: `top`, `row`, `bad`, `end`, `nay`.
- The record line drops `item=` and `fault=`; the kind states which, and the payload is a counted text that states its own byte count. It did run to the end of the line, until the owner's 2026-08-21 directive below made every field counted.
- Every other tail key goes too. Each line's fields become positional, in a fixed order per kind, and a variable-width field states its own byte count as a counted text: the digits, a colon, then that many bytes.
- The head's `status=` is deleted. `answerStatus` and `StatusOK` lose their only answer-side purpose, and the head-validity check in `awaitAnswerHead` moves from the status vocabulary to the kind token.
- `type=` becomes an item-type token: `doc`, `map`, `tab` in place of `json`, `ndjson`, `stream`.
- `answerRecordPrefixMax` becomes `answerRecordPrefixWidth` and it is derivable: the id field at its widest, the three-byte kind, the space that closes it, the twenty digits a uint64 occupies, and the colon that closes that count. 47 bytes, arithmetic rather than a guessed 32.
- `FormatResult` is deleted with its test.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A command answer leaving the daemon: over the multiplexed plugin connection, or over an SSH exec channel that carries one command.
- At entry the answer is a `*plugin.Response` holding either a built payload or a `Records` generator.

### Transformation Path
1. `WriteAnswer` decides the answer type from the record count, unchanged.
2. Each line is built by the append helpers into the reused line buffer, now writing a three-byte kind token where `ok` used to sit.
3. A record line writes id, kind, and the row payload as a counted text, with no key between them.
4. The head writes id, kind, item type, then the envelope key as a counted text, then the column names as a counted text. It states no status.
5. The terminator writes id, kind, then the two counts as counted numbers, then the message as a counted text.
6. On the mux channel the reader walks the id's digits to the space that closes them and takes the kind from the bytes after it; on the exec channel the id is absent and the kind sits at offset zero.
7. `RenderRecords` and `readAnswerFrame` read the kind directly rather than deriving it from the tail, and `awaitAnswerHead` validates the head by its kind rather than by its status.

### Line Grammar

Field order per kind, on the mux channel. The exec channel is identical with the
`#<id>` field absent, because one answer owns that channel.

**Owner directive, 2026-08-21: every field is counted, including the payload.**
The payload no longer runs to the end of the line. It states its byte count
first, in the same shape every other variable-width value uses, and the newline
stays as the line terminator.

**Owner directive, 2026-08-22: a counted field drops its own outer length, and
the rule is keyed to the field's TYPE.** A number is decimal digits and nothing
else, closed by a space or by the end of the line. A text is decimal digits, then
`:`, then exactly that many BYTES. The colon is ALWAYS present on a text, an
empty one included, which is `0:`.

| Kind | Fields, in order | Example |
|------|------------------|---------|
| `top` | id, item type, envelope key, column names | `#42 top map 5:peers 0:` |
| `row` | id, the row payload | `#42 row 32:{"peer":"10.0.0.1","state":"up"}` |
| `bad` | id, the fault payload | `#42 bad 33:{"message":"nexthop unreachable"}` |
| `end` | id, count, faults, message | `#42 end 417 3 0:` |
| `nay` | id, error code, message | `#42 nay 15:unknown-command 15:no such command` |

| Field shape | Written as | Used by |
|-------------|-----------|---------|
| word | three bytes, no count | kind, item type |
| counted number | decimal digits, closed by a space or by the end of the line | the id, under a `#` sigil; count; faults |
| counted text | `<n>:<value>`, where `<n>` is the value's BYTE count | envelope key, column names, error code, message, row payload, fault payload |

**A count is a BYTE count, never a count of characters.** A value MAY hold
multi-byte utf-8, so a reader slices the bytes that arrived rather than the text
they decode to. Phase 6 cost three red fixtures to a harness that counted
characters, and the grammar had never said which it was.

A counted text of zero bytes is present and empty, never omitted: it is written
`0:`, so the field count of a kind never varies. **There is no third shape.** Two
shapes, and a reader reaches every field of every kind by arithmetic with no scan
anywhere. Neither shape states an outer length of its own: a digit run is closed
by a byte no digit can be, so a count in front of one buys a reader nothing.

Why the payload is counted rather than trailing, and what it costs:

- **The length is already computed.** `AnswerRecordLineSize`
  (`pkg/plugin/rpc/message.go:310-317`) measures `len(value)` before the line is
  built, so a producer can refuse an over-wide record. Writing that number costs
  one `strconv.AppendUint` and no measurement.
- **`replaceNewlines` is DELETED** (`pkg/plugin/rpc/message.go:399-406`). It walks
  every payload byte and rewrites `\n` and `\r` to a space, which is a full pass
  over the hot path AND a silent corruption: the payload reaches the operator
  altered, and nothing records that it happened. A counted payload MAY contain a
  raw newline, so nothing has to be rewritten.
- **The reader stops scanning for the end of the line.** That was the last scan
  left on the record line, and removing the `item=` key without it would have
  traded one scan for another.
- **A stated length is a free integrity check.** A line whose payload length
  disagrees with the bytes before its newline is corrupt, and the reader can say
  so. The trailing form admits no such check.
- **The cost is about five bytes for a typical row**, `2:60 ` against a 60-byte
  payload, near 7 percent of the line. That is the trade: 7 percent of the wire
  for the removal of one write-side pass and one read-side scan per row.
- **It lifts this spec's own Known Limitation.** With one trailing field per kind,
  a later field could only be appended at the end. With every field counted, that
  constraint is gone.

The count is the PAYLOAD's length, never the whole line's. The prefix width
differs per channel, because the mux channel carries `#<id> ` and the exec
channel carries none, so a whole-line count would force per-channel arithmetic on
the reader. A payload count is the same number on both channels.

The newline stays. It costs one byte, it keeps a captured session readable by
eye, and it is what the integrity check compares against.

**A line MUST end with exactly one `\n`, and MUST NOT end with `\r\n` (owner
directive, 2026-08-21).** The byte after a counted payload is the line
terminator, and it is `\n`. Nothing else terminates a line, on either channel, in
either direction.

**This is not a style rule, and it breaks the current readers.** With
`replaceNewlines` deleted, `\r` becomes an ordinary payload byte that a producer
MAY write. Both readers use `bufio.Scanner` with the default split function, and
`pkg/plugin/rpc/framing.go:81` states what that does in its own comment:
"Default split func is bufio.ScanLines (splits on \n, strips \r\n)". So a payload
whose last byte is `\r` is silently truncated by the reader, and the stated
length then disagrees with the bytes that arrived.

Two sites carry it, and both change in this spec:

| Site | Today | Owed |
|------|-------|------|
| `pkg/plugin/rpc/framing.go:78` | `bufio.NewScanner(r)`, default `ScanLines` | a split function that splits on `\n` and strips NOTHING |
| `internal/core/ssh/client/answer.go:122` | `bufio.NewScanner(stderr)`, default `ScanLines` | the same split function |

A writer MUST NOT emit `\r\n`, and a reader MUST NOT strip a trailing `\r`. The
two obligations are a pair and neither is safe alone: a stripping reader
corrupts a payload a conforming writer sent, and a `\r\n` writer produces a
payload length no reader can verify.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin (mux connection) | `#<id> ` field, three-byte kind, keyless counted record payload | No |
| Daemon ↔ SSH exec client | no id on the channel, so the kind is at offset zero | No |
| Answer line ↔ request line | both carry `#<id>`, so the id width changes for both | No |
| Engine ↔ in-process plugin (`DirectBridge`) | no wire involved; the bridge must produce the same rows | No |

### Integration Points
- `appendAnswerPrefix` and `appendAnswerID` (`pkg/plugin/rpc/message.go`) - the two writers of the prefix this spec fixes.
- `AppendRequest`, `AppendResult`, `AppendOK`, `AppendError` (`pkg/plugin/rpc/message.go`) - the other writers of `#<id>`, changed for the id width only.
- `AnswerRecordLineSize` (`pkg/plugin/rpc/message.go`) - measures the line before it is built; its scratch becomes exact.
- `ParseAnswerLine`, `ParseAnswerTail` (`pkg/plugin/rpc/message.go`) - the readers, which stop deriving the kind.
- `RenderRecords` (`internal/component/command/render_records.go`) - the operator-facing reader.

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
| A-1 | No out-of-tree consumer speaks this wire | `ai/rules/go-standards.md` states Ze has never been released and permits no compat code anywhere, the plugin API included | The negotiation flag must survive, and the whole spec collapses to a rename | Owner confirmation, recorded here | unvalidated |
| A-2 | RETIRED with the mechanism it was about. The id carries no length character, so nothing has to cover its range | Phase 6 (`9313b7d5e`) deleted the length prefix. `cutID` walks the digits to the space that closes them, and refuses an id past the 20 digits a uint64 occupies and one past the range of a uint64 | N-A | `TestAnswerIDMaxUint64` and `TestAnswerIDFieldRejected` | **retired** |
| A-3 | The mux read loop can tell an answer line from a plain response line by its fixed-offset kind token | `MuxConn.readLoop` splits on the first space today and `interpretResponse` reads the verb | A separate discriminator is needed, most likely a distinct first byte for answer lines | Unit test feeding both line families through one reader | unvalidated |
| A-4 | Three bytes is enough for every line kind now and later | Five kinds exist: head, result record, rejected record, terminator, not understood | A sixth kind would need a longer token or a second alphabet | Owner directive: three bytes confirmed 2026-08-20 | unvalidated |
| A-5 | The head's `status=` carries nothing the terminator does not | `answerStatus` reads the response, never the walk, so it is blind to a partial and to a late failure; `Verdict` derives the outcome from the terminator alone and states that as its reason | A consumer loses a distinction it needs, and the status must return as a positional token | A test asserting a failed command, an aborted walk and a partial are told apart from the terminator alone | **broken** |
| A-6 | Positional fields are readable enough without their key names | Each kind has a fixed field order, and the tokens are words rather than bytes | A capture becomes hard to read by eye, and the key names must return | Review of the published line table against a real captured answer | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The deletion lands before child 1, so no plugin can answer | Plugin functional tests fail at Stage 3 or on the first command | `Depends` is enforced by review: this spec does not start until child 1 is closed |
| R-2 | RETIRED with the mechanism it was about: there is no length character left to disagree with the id. The risk under it survives, which is a reader that mis-slices the id and takes the kind from inside it | A reader reporting an unknown kind on a well-formed line | `cutID` is the one reader and `appendID` the one writer. The field MUST end at a space, every byte before it MUST be a decimal digit, an id past 20 digits is refused, and so is one past the range of a uint64. `TestAnswerIDFieldRejected` asserts the message each refusal earns, not only that one was earned |
| R-3 | The two channels get different offsets and a shared reader assumes one | A test passing over the plugin connection and failing over SSH exec | One reader takes the prefix length from its channel, set once at construction, never inferred per line |
| R-4 | Deleting the head's status loses a distinction some consumer relied on | A consumer that rendered a failure from the head now waits for the terminator | **The mitigation as written is now FALSE and the risk has landed.** `CallAnswer` has three non-test callers and `Answer.Status` has three production readers, all added by child 1 on 2026-08-21 and none present at HEAD: `answerValue` (`pkg/plugin/sdk/sdk_engine.go`), `ExecuteCommandValue` (`internal/component/plugin/ipc/rpc.go`) and `streamedPluginResponse` (`internal/component/plugin/server/command.go`, `if answer.Status == rpc.StatusError`). The surface is no longer provably unused. AC-11 still governs, and it now has to be MADE true rather than merely pinned |

**A-5 is BROKEN, and phase 5 does not start until this is answered (audit, 2026-08-21).**

The head's `status=` carries exactly one fact the terminator does not, and the tree
proves the state is live rather than theoretical. `answerStatus`
(`internal/component/plugin/dispatch.go:379-390`) returns `StatusError` whenever
`resp.Status` is `StatusError`, INCLUDING when `resp.Error` is empty.
`answerMessage` then returns `""`, `WriteRecordAnswer` puts that empty message on
the terminator, and `Verdict` (`pkg/plugin/rpc/message.go:637-651`) reads an empty
message with zero faults as `VerdictDone`. So a command that failed with no
message reaches a consumer as a SUCCESS once the head is gone.

The state is not hypothetical: `responseFailure`
(`internal/component/plugin/dispatch.go`) carries `errStatusErrorNoMessage` for
precisely it, which is a branch somebody wrote because the path is reachable.

**OWNER DIRECTIVE, 2026-08-21: fix the producer.** The decision below was put to
Thomas with its alternative and he chose it. It is settled: phase 5 repairs the
producer and does NOT keep the head's status as a positional token.

The reading, now owner-confirmed: **fix the producer, not the
frame.** A failure with no reason is a zero value wearing a valid answer's
clothes, which `ai/rules/evidence.md` refuses of any guard: fail closed or say
something. So a `StatusError` response MUST carry a message before the head's
status is deleted, and `errStatusErrorNoMessage` becomes unreachable rather than
handled. That keeps this spec's goal intact, because the terminator then carries
the whole outcome and nothing contradicts it.

The rejected alternative is keeping the status as a positional token on the head.
It costs three bytes on one line per answer, not per row, so the cost is not the
argument. The argument is that two lines would again state one outcome and could
disagree, which is the defect this spec exists to remove.

This changes phase 5's shape: it repairs `answerStatus`'s producer and proves
AC-11 against the terminator FIRST, and only then deletes the field.
| R-5 | Removing `item=` makes a record line indistinguishable from a malformed one | A truncated payload read as a valid empty row | The kind is checked before the payload is taken, and an unknown kind refuses the line rather than guessing |
| R-6 | Fixture churn hides a real behavior change | A `.ci` diff touching both the frame and the payload | Change the frame in one commit and assert payloads are byte-identical, so a payload diff is visible |
| R-7 | `writeDocumentAnswer` measures nothing, so a fat bounded answer still overflows one message | A bounded answer read as truncated; already recorded in `plan/journal/gate-excludes-part-of-its-population.md` | Bound the document line here, where the line writers are being rewritten anyway |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every command answer on both channels. A wrong offset makes the CLI, every plugin, and the SSH exec client read garbage. Nothing routes and no session drops, but the daemon becomes unusable from the operator's seat |
| How is it reverted? | Single commit revert. No config migrates and no persisted data carries the frame |
| Who else touches this path? | Child 1 and child 3 of this family, and any spec touching `dispatch.go`, `message.go`, `ssh/answer.go`, or the `test/plugin/answer-*.ci` fixtures |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Plugin runs a command over the mux connection | → | `answerResult` with the branch removed | `TestDispatchCommandAlwaysAnswersRecords` |
| Operator runs `ssh ze "<command>"` without setting any env var | → | `writeExecAnswer` with `declaresRecordAnswers` removed | `test-exec-answer-unconditional` |
| A reader takes a record line | → | `ParseAnswerLine` reaches the kind by arithmetic | `TestParseAnswerLineFixedOffsets` |
| A request line is written | → | `AppendRequest` opens with `appendID`, the one writer of the field | `TestAppendRequestSpellsTheSameIDField` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A plugin completes Stage 3 declaring no protocol name | It receives the record answer sequence for `dispatch-command` and `dispatch-command-args` |
| AC-2 | An SSH exec client connects without setting `ZE_ANSWER_PROTOCOL` | It receives head, records and terminator, and the env var no longer exists in the tree |
| AC-3 | Any answer line is written on the mux channel | Its kind token is reached from the id's length byte with one addition, identically for all five kinds, and never by searching for a space |
| AC-4 | Any answer line is written on the exec channel | Its kind token starts at offset zero |
| AC-5 | A result record line is written | Its payload's byte count follows the kind token, and the payload follows the count. No key, and no separator scanned for at either end: the reader slices the payload by arithmetic and never looks for the newline to find where it stops |
| AC-6 | Ids of one digit, ten digits and the maximum uint64 are written | Each reads back to the value written, and a reader reaches the kind token by walking the id's digits to the one space that closes them. An id past the twenty digits a uint64 occupies, and one past the range of a uint64, are each refused by name |
| AC-7 | A reader meets an unknown three-byte kind | It refuses the line with a named error rather than guessing the kind from the tail |
| AC-8 | A command answers with a bounded payload | The payload bytes are identical to those the same command produced before this spec |
| AC-9 | A bounded answer's document exceeds one wire message | It is refused with the same fault an over-wide record gets, rather than being written and read as truncated |
| AC-10 | The tree is searched for the deleted symbols | `ProtocolRecordAnswers`, `AnswerProtocolEnv`, `declaresRecordAnswers`, `RecordAnswers`, `SetRecordAnswers` and `FormatResult` are absent |
| AC-11 | A command fails outright, a walk is aborted part way, and a walk rejects some rows | A consumer tells the three apart from the terminator alone, and no line states an outcome that another line can contradict |
| AC-12 | A walk of one million rows is written and read | No line is scanned for a separator before its payload is taken |
| AC-13 | Any answer line is parsed | It carries no `key=value` pair at all, and every field is one of three shapes: a three-byte word closed by a space, a counted number of decimal digits closed by a space or by the end of the line, or a counted text of decimal digits, a colon, and that many bytes. No field runs to the end of the line uncounted |
| AC-14 | The tree is searched for the answer tail key names | `answerKeyStatus` through `answerKeyCode` are absent, and `ParseAnswerTail` no longer dispatches on a key name |
| AC-15 | A head is written | It states no status, and a reader that meets a line whose kind is not `top` where a head belongs refuses the answer |
| AC-16 | Each token is checked against a dictionary | Every one is a whole word, not a truncation: `top`, `row`, `bad`, `end`, `nay`, `doc`, `map`, `tab` |
| AC-17 | A head names an envelope | The name states its own BYTE count and then the colon every counted text carries, and an absent name writes `0:` rather than omitting the field, so a line's field count never varies |
| AC-18 | RETIRED by phase 6 (`0faf5e3a9`). It capped an envelope key and an error code at 35 bytes | The cap existed only because the spec assumed a one-character length form. A counted text states its byte count as a run of decimal digits, so it carries a value of any length and nothing is capped by its own encoding. The cap was never implemented: `grep -rn "envelopeKeyMax" --include=*.go .` returns nothing. What still bounds these values is the frame's own `MaxMessageSize` refusal |
| AC-19 | A row payload whose LAST byte is `\r`, and one containing a raw `\n`, are written and read | Both round-trip byte for byte. Neither is rewritten, neither is truncated, and no reader strips anything after the counted payload. This fails today: `bufio.ScanLines` strips a trailing `\r` at `pkg/plugin/rpc/framing.go:78` and `internal/core/ssh/client/answer.go:122` |
| AC-20 | A line arrives terminated with `\r\n`, and a line arrives whose stated payload length disagrees with the bytes before its `\n` | Each is refused with a named error rather than parsed. A line ends with exactly one `\n`, and the length is checked against what arrived |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs a command over SSH with no environment set up | ssh exec → `writeExecAnswer` → `readAnswerFrame` → rendering | `test-exec-answer-unconditional` |
| 2 | Runs a command that walks a million rows and pipes it to `first 10` | dispatcher → `ApplyPipesRecords` → `WriteAnswer` → `RenderRecords` | `test-answer-first-bounds-long-walk` |
| 3 | Runs a plugin command from a plugin that declares nothing | plugin → `dispatch-command` → record sequence → SDK reader | `test-plugin-answer-unconditional` |
| 4 | Reads a captured session to debug a plugin | wire bytes → the documented line table | `TestAnswerLineTableMatchesDoc` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseAnswerLineFixedOffsets` | `pkg/plugin/rpc/message_test.go` | every kind puts its token at the same offset | |
| `TestAppendAnswerRecordNoKey` | `pkg/plugin/rpc/message_test.go` | the record payload follows the prefix directly | |
| `TestAppendRequestSpellsTheSameIDField` | `pkg/plugin/rpc/message_test.go` | requests carry the same id encoding as answers | |
| `TestAnswerIDRoundTrip` | `pkg/plugin/rpc/message_test.go` | every digit count from 1 to 20 reads back to the value written | |
| `TestAnswerIDMaxUint64` | `pkg/plugin/rpc/message_test.go` | the maximum uint64 id round-trips | |
| `TestAnswerIDFieldRejected` | `pkg/plugin/rpc/message_test.go` | a malformed id field refuses the line, and each case earns its own message | |
| `TestParseAnswerLineUnknownKind` | `pkg/plugin/rpc/message_test.go` | an unknown kind is refused, not guessed | |
| `TestAnswerRecordLineSizeExact` | `pkg/plugin/rpc/message_test.go` | the measured size equals the written size | |
| `TestDispatchCommandAlwaysAnswersRecords` | `internal/component/plugin/server/dispatch_registry_test.go` | the negotiation branch is gone | |
| `TestWriteDocumentAnswerBounded` | `internal/component/plugin/dispatch_test.go` | an over-wide document is refused (R-7) | |
| `TestExecAnswerUnconditional` | `internal/component/ssh/answer_test.go` | no env var is consulted | |
| `TestMuxReadLoopSeparatesAnswerFromResponse` | `pkg/plugin/rpc/mux_test.go` | A-3 resolves: one reader, two line families | |
| `TestAnswerLineTableMatchesDoc` | `pkg/plugin/rpc/message_test.go` | the documented field order is the written field order | |
| `TestAnswerLineCarriesNoKeyNames` | `pkg/plugin/rpc/message_test.go` | no line carries a `key=value` pair (AC-13, AC-14) | |
| `TestHeadStatesNoStatus` | `pkg/plugin/rpc/message_test.go` | the head writes kind, id, type, key, columns, and nothing else (AC-15) | |
| `TestAwaitAnswerHeadValidatesByKind` | `pkg/plugin/rpc/mux_test.go` | the head guard moved from status to the kind token | |
| `TestVerdictTellsFailedFromAbortedFromPartial` | `pkg/plugin/rpc/message_test.go` | the terminator alone distinguishes the three outcomes (AC-11, A-5) | |
| `TestEnvelopeKeyLengthPrefixed` | `pkg/plugin/rpc/message_test.go` | an absent envelope writes `0:` rather than omitting the field (AC-17) | |
| ~~`TestEnvelopeKeyOverLimitRefused`~~ | - | RETIRED with AC-18 by phase 6 (`0faf5e3a9`). A counted text carries a value of any length, so there is no 35-byte cap to prove | retired |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| answer id digits | 1 to 20 | 20 | zero digits, `# `, refuses the line | 21 digits refuses the line, and so does a 20-digit run past the range of a uint64 |
| counted field digits | 1 to 20 | 20 | zero digits refuses the field | 21 digits refuses the field, because a uint64 occupies 20 |
| kind token length | 3 | 3 | 2 refuses the line | 4 refuses the line |
| item type token length | 3 | 3 | 2 refuses the head | 4 refuses the head |
| envelope key length | 0 to the frame's own limit | `MaxMessageSize`, like every other counted text | N/A, zero means no envelope and writes `0:` | N/A. The 35-byte cap was retired with AC-18 |
| record line width | prefix to `MaxMessageSize` | `MaxMessageSize` | N/A | one byte over becomes a fault |
| document line width | prefix to `MaxMessageSize` | `MaxMessageSize` | N/A | one byte over is refused (AC-9) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-exec-answer-unconditional` | `test/plugin/exec-answer-unconditional.ci` | an operator runs a command over SSH with nothing configured | | <!-- doc-links: ignore (functional test this spec will create; the work is not implemented) -->
| `test-plugin-answer-unconditional` | `test/plugin/answer-unconditional.ci` | a plugin that declares nothing still gets records; replaces `answer-not-negotiated.ci` | done, phase 1 |
| `test-answer-first-bounds-long-walk` | `test/plugin/answer-first-bounds-long-walk.ci` | `first 10` over a long walk still bounds the read | done, phase 6 |
| `test-plugin-command-document-too-wide` | `test/plugin/plugin-command-document-too-wide.ci` | a bounded answer whose document no line can carry is refused rather than truncated (AC-9, R-7) | done, phase 7 |
| `test-answer-payload-unchanged` | `test/plugin/answer-payload-unchanged.ci` | the payload of an existing command is byte-identical after the reframe | done, phase 8 |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | This wire is internal to Ze: the plugin connection and the SSH exec channel have no third-party speaker, so no peer daemon can validate it. The `.ci` tests drive the real daemon, the real SDK and the real SSH client, which is the strongest evidence this surface admits | N/A |

## Files to Modify
- `pkg/plugin/rpc/message.go` - kind tokens, positional counted fields, keyless record lines, the exact prefix width, delete `FormatResult`
- `pkg/plugin/rpc/types.go` - delete `ProtocolRecordAnswers` and `AnswerProtocolEnv`; `DeclareCapabilitiesInput` loses the protocol list if nothing else uses it
- `pkg/plugin/rpc/mux.go` - `readLoop` and `interpretResponse` take the id through `cutID`; `awaitAnswerHead` validates the head by its kind rather than by its status; `Answer` loses its `Status` field
- `pkg/plugin/rpc/conn.go` - the id writer shared with requests
- `internal/component/plugin/dispatch.go` - `WriteAnswer` unconditional; `writeDocumentAnswer` bounds its line
- `internal/component/plugin/server/dispatch_registry.go` - `answerResult` loses its branch
- `internal/component/plugin/server/dispatch.go` - `responseToDispatchOutput` stops being the second encoding of a command answer
- `internal/component/plugin/process/process.go` - delete `RecordAnswers` and `SetRecordAnswers`
- `internal/component/plugin/server/startup.go` - Stage 3 stops reading the protocol list
- `internal/component/ssh/answer.go` - delete `declaresRecordAnswers`; `writeExecAnswer` is unconditional
- `internal/core/ssh/client/answer.go` - stop setting the env var; read the fixed prefix
- `internal/component/command/render_records.go` - read the kind rather than deriving it
- `docs/architecture/api/ipc_protocol.md` - rewrite the Negotiation and Lines sections; publish the offset table
- `docs/architecture/api/process-protocol.md` - Stage 3 no longer negotiates an answer shape
- `docs/architecture/api/wire-format.md` - the id width applies to every line
- `test/plugin/answer-single-record.ci`, `test/plugin/answer-many-records.ci`, `test/plugin/answer-truncation-detected.ci`, `test/plugin/answer-unknown-command.ci` - reframed fixtures

## Files to Create
- `test/plugin/exec-answer-unconditional.ci` - SSH exec with nothing configured. NOT CREATED, and it is the one open item this spec leaves <!-- doc-links: ignore (functional test this spec planned and did not create) -->
- `test/plugin/answer-unconditional.ci` - replaces `answer-not-negotiated.ci`. Created by phase 1
- `test/plugin/answer-first-bounds-long-walk.ci` - the pipe still bounds the walk. Created by phase 6
- `test/plugin/plugin-command-document-too-wide.ci` - a bounded answer's document is measured before it is built. Created by phase 7
- `test/plugin/answer-payload-unchanged.ci` - payload bytes survive the reframe. Created by phase 8

## Files to Delete
- `test/plugin/answer-not-negotiated.ci` - DONE by phase 1 (`c08252e0a`), which renamed it to `answer-unconditional.ci`. Its premise was the branch this spec removes <!-- doc-links: ignore (the path names a file this spec DELETED; it no longer exists, which is the row's point) -->

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config leaf and no new RPC; the frame of two existing ops changes |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | N-A | No verb or flag changes |
| CLI grammar (keyword before value) | N-A | No grammar change |
| Editor autocomplete | N-A | No new leaf or dynamic value |
| Functional test for new RPC/API | Yes | The four new `.ci` files above |
| Pipe completeness | Yes | `internal/component/command/pipe_records.go` and `render_records.go` must render every pipe over the new frame |
| Env var registration | Yes | `ZE_ANSWER_PROTOCOL` is REMOVED. It was never a YANG-backed `ze.*` leaf, so nothing is deregistered, but the removal is named here so it is not reintroduced |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module, or binary |
| Prometheus counters/metrics | No | No new observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP protocol work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The operator sees the same data through the same commands |
| 2 | Config syntax changed? | No | No config surface touched |
| 3 | CLI command added/changed? | No | No verb, flag, or rendering changes |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` -- the answer frame of the two dispatch ops |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` -- a plugin no longer declares an answer shape |
| 6 | Has a user guide page? | No | No operator-facing workflow changes |
| 7 | Wire format changed? | Yes | `docs/architecture/api/wire-format.md` -- id width and answer line grammar |
| 8 | Plugin SDK/protocol changed? | Yes | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC governs this wire |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` -- four `.ci` files added, one deleted, four reframed |
| 11 | Affects daemon comparison? | No | No externally visible capability changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/ipc_protocol.md` -- Negotiation section deleted, offset table added |
| 13 | Route metadata keys added/changed? | No | No route metadata touched |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/features/plugins.md` -- the `record-answers` protocol name leaves the inventory |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `python3 scripts/dev/spec_doc_anchors.py plan/spec-record-answers-2-only-encoding.md`. Known declaring headers: `pkg/plugin/rpc/message.go`, `pkg/plugin/rpc/types.go` and `internal/component/ssh/answer.go` declare `ipc_protocol.md`; `internal/component/plugin/dispatch.go` declares `commands.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/api/ipc_protocol.md` carries worked answer lines; every one is rewritten with the new offsets |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the unconditional path reachable and prove the branch is gone
   - Tests: `TestDispatchCommandAlwaysAnswersRecords`, `TestExecAnswerUnconditional`
   - Files: `internal/component/plugin/server/dispatch_registry.go`, `internal/component/ssh/answer.go`
   - Verify: both tests fail while the branch stands, and the deleted-symbol search (AC-10) still finds the symbols
2. **Phase: Delete the negotiation** -- one encoding, no flag
   - Tests: `TestDispatchCommandAlwaysAnswersRecords`, `test-plugin-answer-unconditional`, `test-exec-answer-unconditional`
   - Files: `pkg/plugin/rpc/types.go`, `internal/component/plugin/process/process.go`, `internal/component/plugin/server/startup.go`, `internal/core/ssh/client/answer.go`, `internal/component/plugin/server/dispatch.go`
   - Verify: AC-10's search is empty, and `answer-unconditional.ci` is deleted rather than adjusted
3. **Phase: Length-prefixed id** -- every line carrying an id
   - Tests: `TestAppendRequestSpellsTheSameIDField`, `TestAnswerIDRoundTrip`, `TestAnswerIDMaxUint64`, `TestAnswerIDFieldRejected`
   - Files: `pkg/plugin/rpc/message.go`, `pkg/plugin/rpc/conn.go`, `pkg/plugin/rpc/mux.go`
   - Verify: requests and answers agree on the encoding, the whole uint64 range is expressible, and A-3 is answered by `TestMuxReadLoopSeparatesAnswerFromResponse`
4. **Phase: Kind tokens** -- the line states what it is
   - Tests: `TestParseAnswerLineFixedOffsets`, `TestParseAnswerLineUnknownKind`, `TestAnswerLineTableMatchesDoc`, `TestAwaitAnswerHeadValidatesByKind`
   - Files: `pkg/plugin/rpc/message.go`, `pkg/plugin/rpc/mux.go`, `internal/component/command/render_records.go`, `internal/core/ssh/client/answer.go`
   - Verify: no reader derives the kind from the tail, and the head-validity guard reads the kind rather than the status
5. **Phase: Positional fields** -- the key names leave the wire
   - Tests: `TestAnswerLineCarriesNoKeyNames`, `TestHeadStatesNoStatus`, `TestVerdictTellsFailedFromAbortedFromPartial`, `TestEnvelopeKeyLengthPrefixed`
   - Files: `pkg/plugin/rpc/message.go`, `internal/component/plugin/dispatch.go`, `internal/component/ssh/answer.go`
   - Verify: AC-11 passes BEFORE the status field is deleted, so the distinction is proven to live in the terminator first (R-4)
6. **Phase: Keyless record line and bounded prefix** -- the hot line
   - Tests: `TestAppendAnswerRecordNoKey`, `TestAnswerRecordLineSizeExact`, `test-answer-first-bounds-long-walk`
   - Files: `pkg/plugin/rpc/message.go`, `internal/component/plugin/dispatch.go`
   - Verify: the measured size equals the written size, and the prefix constant is exact rather than a maximum
7. **Phase: Bound the document line** -- close R-7
   - Tests: `TestWriteDocumentAnswerBounded`
   - Files: `internal/component/plugin/dispatch.go`
   - Verify: an over-wide bounded answer is refused, and the journal row is answered by code rather than by a record
8. **Phase: Fixtures and docs** -- one pass over everything that reads the wire
   - Tests: the reframed `.ci` files plus `test-answer-payload-unchanged`
   - Files: the `.ci` files and every doc row answered Yes above
   - Verify: `make ze-functional-plugin-test` passes, and the payload test proves only the frame moved
   - Owner directive, 2026-08-22: **the wire stopped explaining itself, so the document owes what the wire no longer says.** `#42 ok status=done type=ndjson key=peers` was legible with no documentation, because the key names WERE the explanation. `#42 top map 5:peers 0:` is not. So every wire example in an API document carries an IN-PLACE decode naming each field, adjacent to the bytes rather than in prose nearby; every count is stated as a BYTE count; every three-letter word is given its meaning where a reader first meets it; and no API page delegates its own subject to another page. A link stays for depth and MUST NOT be the only route to the meaning.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Feature completeness | Both channels reframed, and the request line's id changed with them |
| Correctness | Payload bytes unchanged (AC-8); an unknown kind refuses rather than guesses (AC-7); the document line is bounded (AC-9) |
| Naming | Every token is a whole word, never a truncation, and byte 0 is distinct inside each field: `top` `row` `bad` `end` `nay` for kinds, `doc` `map` `tab` for item types |
| Data flow | The kind is read once per line. Nothing derives it from the tail afterwards |
| Rule: `ai/rules/no-layering.md` | The single-line command-answer path is deleted, not disabled. No flag, no dead branch, no unused constant |
| Rule: `ai/rules/stale-comments.md` | Every comment describing the negotiation, the decimal id, or the `item=` key is rewritten in the same edit |
| Rule: `ai/rules/evidence.md` | The published offset table is asserted by a test, so the doc cannot drift from the writer |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The negotiation is gone | `grep -rn "ProtocolRecordAnswers\|AnswerProtocolEnv\|declaresRecordAnswers\|SetRecordAnswers\|FormatResult" --include=*.go .` returns nothing |
| The id is one digit run a space closes | `TestAnswerIDRoundTrip`, `TestAnswerIDMaxUint64` and `TestAnswerIDFieldRejected` pass |
| The kind is reached by arithmetic | `TestParseAnswerLineFixedOffsets` passes |
| No key name reaches the wire | `grep -n "answerKeyStatus\|answerKeyType\|answerKeyEnvelope\|answerKeyFields\|answerKeyItem\|answerKeyFault\|answerKeyCount\|answerKeyFaults\|answerKeyMessage\|answerKeyCode" pkg/plugin/rpc/message.go` returns nothing |
| The head states no status | `TestHeadStatesNoStatus` passes, and `grep -n "StatusOK" pkg/plugin/rpc/` returns no answer-side use |
| Every token is a word | `TestAnswerLineTableMatchesDoc` pins the vocabulary, and the review reads it against a dictionary |
| The record line has no key | `TestAppendAnswerRecordNoKey` passes |
| The prefix constant is exact | `TestAnswerRecordLineSizeExact` passes |
| The document line is bounded | `TestWriteDocumentAnswerBounded` passes |
| The doc matches the writer | `TestAnswerLineTableMatchesDoc` passes |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A fixed-offset reader must check the line is long enough BEFORE it slices, or a short line panics. Every kind read is bounds-checked |
| Injection | ANSWERED by the counted payload. A payload states its length, so a newline inside one cannot split the line and nothing has to be rewritten. `replaceNewlines` is DELETED rather than extended, which also ends the silent corruption it performed: it turned a `\n` or `\r` in operator data into a space and recorded nothing. The reader MUST compare the stated length against the bytes before the newline and refuse a line where they disagree, because that comparison is what replaces the delimiter's guarantee |
| Resource exhaustion | The document line gains the bound the record line already had, so no single answer can exceed one wire message |
| Error leakage | An unknown-kind error names the kind it saw; it must truncate that value rather than echoing an arbitrary-length line |

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
| A three-byte kind token replaces the verb | Keep `ok` and `error` and pad them; use a single discriminating byte | Padding hides the field boundary from a person reading a capture. One byte is unreadable. Three bytes gives a word a person reads and a first byte a machine switches on, with no trade between them (owner directive, 2026-08-20) |
| The kind replaces `item=` and `fault=` as well | Keep the keys and add a kind | Two statements of one fact can disagree, and the record line is the line that repeats. The key is the scan this spec exists to remove |
| **REVERSED 2026-08-22.** The id was to be length-prefixed, `#<len>:<id>` | 16 zero-padded hex digits; decimal padded to 20 | DECIDED 2026-08-20: a padded field pays its widest case on every line, which is eighteen wasted bytes per line of a million-row walk, and one length byte was to give the reader the same arithmetic for nothing. Phase 3 (`326ce6e96`) built it. **The owner measured it and reversed it in phase 6 (`9313b7d5e`): a counted id costs 8.1 to 9.2 ns against 3.2 to 3.5 ns for a fused loop over the plain form, zero allocations either way, and it is two bytes wider on every line.** The count bought nothing, because `cutID` still had to check the space that closes the field and still had to parse the digits, which IS the cost of the plain form. A count belongs on a field whose value can hold the delimiter. An id cannot: a digit is not a space. The id is `#<id> ` |
| **REVERSED 2026-08-22.** The length character was to be base 36, and every counted field was to state its own outer length | One decimal digit, capping the id at nine digits and wrapping the counter | DECIDED 2026-08-20: base 36 expresses 35 in one byte, so the whole 20-digit uint64 range fits with headroom and no counter has to wrap. Phase 5 (`46c4d0e1e`) extended the same form to every counted field. **The owner measured it and reversed it in `50468ee34`: reading an id with a fused accumulate loop costs 3.2 ns against 8.6 ns for decoding a base-36 length and then calling ParseUint on the slice it just measured, zero allocations either way (owner, Go 1.26.6).** The outer length bought nothing for the same reason, so `countedLengthAlphabet` is deleted. What closes a field is keyed to its TYPE instead: a number ends at a space or the line's end, a text ends after the bytes its count states. The one count that stays is the text's, because a text MAY hold the delimiter and nothing else can say where it ends |
| Every field is a three-byte word, a counted number, or a counted text | Keep `key=value` tails on the head and terminator, since they run once per answer | Two mechanisms in one protocol is the cost, not the byte count. One vocabulary means one reader, and it removes `ParseAnswerTail`'s key dispatch and its unknown-key branch outright |
| The head states no status | Shorten it to a three-byte token beside the type | `answerStatus` reads the response and never the walk, so it is blind to a partial and to a late failure; the terminator's counts and message already tell a failed command, an aborted walk and a partial apart. `Verdict` says as much in its own comment: a terminator carries no status, so nothing can disagree with the counts. The head's status was the one field that could |
| Item types are `doc`, `map`, `tab` | `jsn`, `ndj`, `str`, as three-byte spellings of the existing names | These say what an item IS rather than naming a serialization, and they end the collision `ipc_protocol.md` apologises for between `type=json` and the `\| json` pipe operator. They are also words rather than stumps |
| Every token is a whole word | Truncations such as `hed`, `don`, `obj`, `arr` | `docs/contributing/ze-style.md` asks for the noun, not the abbreviation, and a token a person must expand is the readability this design already spends. Byte 0 stays distinct inside each field, so nothing is lost mechanically (owner directive, 2026-08-21) |
| Fixed offsets are per channel | Always write the id, so one layout serves both | The exec channel carries one answer, so an id there states a fact with one possible value. A reader knows its channel at construction, so a per-channel constant is as static as a global one |
| The frame and the negotiation change in one spec | Two specs, one for each | They rewrite the same writers, the same readers and the same fixtures. Two passes would churn every fixture twice and make a payload change harder to see in the second diff |

## Known Limitations
- A line stops being self-describing to a person. `top map 5:peers` needs the published field order to read, where `status=done type=ndjson key=peers` did not. The tokens being whole words rather than bytes is what keeps that price payable, and A-6 tracks whether it was paid.
- ~~A new field can only be appended at the end of a line's field list.~~ REMOVED by the counted payload (owner directive, 2026-08-21). With no trailing field, every field states its own width, so a later field can sit anywhere a new protocol name agrees on.
- Record-level streaming for REST, gRPC, web, MCP and the looking glass stays out of scope; those surfaces collapse through `CollapseRecords`. Recorded in `plan/deferrals/streaming-answer-protocol.md`.
- `table` and `text` rendering still buffers, because a column width needs every row. Recorded as a permanent limit in the same shard.
- The per-row allocations behind the frame are untouched here. They belong to child 3.

## RFC Documentation (Scope: protocol)

No RFC governs the Ze plugin connection or the SSH exec answer channel. This
section is retained because the spec's Scope is `protocol`, and its answer is
that the protocol is Ze's own. The SSH transport beneath the exec channel is
governed by RFC 4254, and this spec changes nothing in it: the answer travels as
channel data, exactly as before.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-20 all demonstrated, or retired here with the commit that retired them (AC-18 only)
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
- [ ] Learned summary written to `plan/learned/NNN-record-answers-only-encoding.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-record-answers-2-only-encoding.md` only
