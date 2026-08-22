# Ze IPC Protocol Specification

**Status:** Canonical reference for Ze inter-process communication

---

## Overview

Ze uses correlated, bidirectional RPC between the engine and plugins. Every
request carries a `uint64` ID and receives an `ok` or `error` response. The same
connection carries engine callbacks and plugin-to-engine requests.
<!-- source: pkg/plugin/rpc/conn.go -- Conn -->
<!-- source: pkg/plugin/rpc/mux.go -- MuxConn.readLoop -->

---

## Concepts

### Subsystems

A **subsystem** is a protocol component that follows the ZE API for plugin communication:

| Subsystem | Description |
|-----------|-------------|
| `bgp` | BGP protocol subsystem |
| `rib` | RIB subsystem (protocol-agnostic) |
| Future | `bmp`, `rpki`, etc. |

### Event Subscription

Plugins subscribe dynamically through `ze-plugin-engine:subscribe-events` or send
startup subscriptions in `ze-plugin-engine:ready`.
<!-- source: pkg/plugin/sdk/sdk_engine.go -- Plugin.SubscribeEvents -->
<!-- source: pkg/plugin/rpc/types.go -- SubscribeEventsInput -->
<!-- source: pkg/plugin/sdk/sdk.go -- Plugin.Run -->
<!-- source: pkg/plugin/rpc/types.go -- ReadyInput -->

### Plugin-Provided Commands

Plugins register command paths in `ze-plugin-engine:declare-registration`.
The engine command dispatcher invokes the registered callback.

Engine provides reactor methods, and plugins register commands that use them.

The same message carries a `pipes` list. Each `PipeDecl` names a CLI pipe alias
for one of that plugin's own commands, with the command path, the alias name,
its description and the operator chain the name stands for. The engine parses
the expansion once, at registration, and refuses the whole list when one entry
is wrong.
<!-- source: pkg/plugin/rpc/types.go -- PipeDecl, DeclareRegistrationInput -->
<!-- source: internal/component/plugin/server/startup.go -- validatePipeDecls, registerPluginPipes -->

### Message Cache

The engine exposes separate SDK RPCs for cached and stored UPDATEs:

- `ze-plugin-engine:forward-cached` forwards cached UPDATE IDs.
- `ze-plugin-engine:release-cached` releases cached UPDATE IDs.
- `ze-plugin-engine:relay-stored-route` relays routes that a plugin stores.
<!-- source: pkg/plugin/rpc/types.go -- MethodForwardCached, MethodReleaseCached, MethodRelayStoredRoute -->

---

## Plugin Transports

`rpc.Conn` supports one socket for both directions, separate read and write
connections, or standard input and standard output. All transports use the same
plugin RPC frames.
<!-- source: pkg/plugin/rpc/conn.go -- Conn, NewConn -->

---

## Wire Format

### Message Framing

All plugin RPC messages are UTF-8, newline-delimited lines:

```
#<len>:<id> <verb> [<json>]\n
```

Each line is a complete message. The `#` prefix and decimal `uint64` ID are
required, and the ID states its own width: `<len>` is one base-36 character,
`0` to `9` then `A` to `Z`, naming how many decimal digits follow the `:`. A
reader takes that one byte and reaches the verb by addition rather than by
searching the line for a space. A uint64 occupies 20 digits at most, so `K` is
the widest length an id can state. The optional payload is compact JSON.
<!-- source: pkg/plugin/rpc/framing.go -- FrameReader, FrameWriter -->
<!-- source: pkg/plugin/rpc/message.go -- ParseLine -->

### RPC Request (Either Direction)

```
#2:42 ze-plugin-engine:subscribe-events {"events":["update"]}
#2:43 ze-plugin-callback:deliver-batch {"events":[{"type":"bgp"}]}
```

For a request, the verb is a method name. A plugin-to-engine method uses the
`ze-plugin-engine:` prefix. An engine callback uses the
`ze-plugin-callback:` prefix.
<!-- source: pkg/plugin/rpc/message.go -- AppendRequest -->

### RPC Response (Either Direction)

```
#2:42 ok
#2:43 error {"code":"invalid-event","message":"unknown event"}
```

| Component | Requirement |
|-----------|-------------|
| `#<len>:<id>` | Echoes the request ID, with its base-36 digit count in front |
| `ok` | The only success response verb |
| `error` | The only error response verb |
| `<json>` | Optional result or error payload |

`done`, `warning`, `ack`, and partial response frames are not plugin RPC
response verbs. They can occur only inside a command-dispatch result payload.
<!-- source: pkg/plugin/rpc/message.go -- AppendResult, AppendError -->
<!-- source: pkg/plugin/rpc/mux.go -- MuxConn.readLoop, interpretResponse -->

### Event Format (Engine → Plugin)

JSON with `type` field indicating which key contains the payload. The `peer` field is at the `bgp` level; event type is in `bgp.message.type`; event-specific data is nested under the event type key:

```json
{
  "type": "bgp",
  "bgp": {
    "message": {"type": "update", "id": 123, "direction": "received"},
    "peer": {"address": "10.0.0.1", "group": "transit", "local": {"address": "10.0.0.2", "as": 65000}, "name": "upstream1", "remote": {"address": "10.0.0.1", "as": 65001}},
    "update": {
      "attr": {"origin": "igp", "as-path": [65001]},
      "nlri": {"ipv4/unicast": [{"action": "add", "next-hop": "10.0.0.1", "nlri": ["10.0.0.0/24"]}]}
    }
  }
}
```

**Exception:** State events use a simple string value for `state` instead of a container:

```json
{"type": "bgp", "bgp": {"message": {"type": "state"}, "peer": {...}, "state": "up"}}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Always | `"bgp"`, `"rib"` - indicates payload key |
| `bgp` | object | If type=bgp | BGP event payload |
| `rib` | object | If type=rib | RIB event payload |

**BGP event payload (`bgp` key):**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `message` | object | Always | `{"type":"<event>"}` with optional `"id"` and `"direction"` fields |
| `peer` | object | Always | `{"address":"<ip>", "local":{"address":"<local-ip>","as":<local-asn>}, "remote":{"address":"<ip>","as":<asn>}}` - at bgp level. Optional `"name"` and `"group"` when configured. |
| `<type>` | object/string | Usually | Event data nested under event type key (string for state events) |

**Message metadata fields (inside `bgp.message`):**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Always | `update`, `open`, `notification`, `keepalive`, `refresh`, `state`, `negotiated` |
| `id` | int | If > 0 | Message identifier |
| `direction` | string | If set | `"received"` or `"sent"` |

**BGP event data fields (inside `bgp.<type>` object, except state):**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `attr` | object | For UPDATE | Path attributes (origin, as-path, etc.) |
| `nlri` | object | For UPDATE | `{"<family>": [operations...]}` |
| `raw` | object | If format=full | Wire bytes (see Raw Format below) |

**State events:** Use simple `"state": "up"` string at bgp level (no container). Down events include `"reason": "..."` field.
<!-- source: internal/component/bgp/format/text_json.go -- appendStateChangeJSON -->

**RIB event payload (`rib` key):**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Always | `cache`, `route` |
| `action` | string | Always | `new`, `evict`, `add`, `remove` |
| `msg-id` | uint64 | For cache events | Message cache ID |
| `peer` | object | Always | `{"address":"<ip>", "local":{"address":"<local-ip>","as":<local-asn>}, "remote":{"address":"<ip>","as":<asn>}}`. Optional `"name"` and `"group"` when configured. |

**Message object fields (BGP wire messages only):**

| Field | Type | Description |
|-------|------|-------------|
| `id` | uint64 | Message cache ID (0 for locally-originated) |
| `direction` | string | `"received"` or `"sent"` |

**Raw object fields (format=full only):**

| Field | Type | Description |
|-------|------|-------------|
| `attributes` | string | Hex-encoded path attributes wire bytes |
| `nlri` | object | `{"<family>": "<hex>"}` - NLRI wire bytes per family |
| `withdrawn` | object | `{"<family>": "<hex>"}` - Withdrawn wire bytes per family |

Events are only sent to plugins that have subscribed.
<!-- source: internal/component/bgp/format/text_human.go -- appendFilterResultText -->
<!-- source: internal/component/bgp/format/text.go -- AppendOpen, AppendNotification -->

---

## Correlation and Command Dispatch

Every plugin RPC request has a `#<len>:<uint64>` correlation ID. The peer returns a
response with the same ID. Most requests receive exactly one response line.
Three carry a command answer, and each of those receives a sequence of lines:
`dispatch-command` and `dispatch-command-args` from the engine, and
`execute-command` from the plugin. See Answer Protocol below.
<!-- source: pkg/plugin/rpc/message.go -- ParseLine, AppendRequest, AppendResult, AppendError -->

The command dispatcher is a payload protocol inside plugin RPC:

```
#1:1 ze-plugin-engine:dispatch-command {"command":"show bgp peer * detail"}
```

The request line is the plugin RPC frame, and the `command` member inside it
belongs to `DispatchCommandInput`. The answer arrives as the sequence Answer
Protocol describes, and a caller that reads that sequence as one value gets the
`status` and `data` members of `DispatchCommandOutput`. The payload does not
change RPC framing.
<!-- source: pkg/plugin/rpc/types.go -- DispatchCommandInput, DispatchCommandOutput -->
<!-- source: pkg/plugin/sdk/sdk_engine.go -- Plugin.dispatchCommandValue, answerValue -->
<!-- source: internal/core/ipc/message.go -- MapResponse -->

---

## Answer Protocol

A command answer is a sequence of lines: a head, zero or more records, and a
terminator. One code path writes every answer, whatever its record count, so a
reader follows one path and nothing declares a shape the payload can contradict.
<!-- source: pkg/plugin/rpc/message.go -- AppendAnswerHead, AppendAnswerItem, AppendAnswerFault, AppendAnswerTerminator -->

Each line is `#<len>:<id> <kind> <fields>`, and its fields are positional, so a
reader decides how to take the answer without a JSON decoder and without reading
a key name. On the SSH exec channel there is no id field at all: one command owns
the channel, so nothing needs to be told apart.
<!-- source: pkg/plugin/rpc/message.go -- AnswerNoID, ParseAnswerLine -->

### One encoding, both directions

A command answer has one encoding on every connection, and nothing declares it.
The engine writes the head, the records and the terminator to every peer, and it
reads that same sequence back from every plugin. Stage 3
`declare-capabilities` carries the BGP capabilities a plugin injects into OPEN
and names no wire shape:

```
#1:2 ze-plugin-engine:declare-capabilities {"capabilities":[]}
#1:2 ok
```
<!-- source: pkg/plugin/rpc/types.go -- DeclareCapabilitiesInput, CapabilityDecl -->
<!-- source: pkg/plugin/rpc/answer_write.go -- WriteRecordAnswer, WriteDocumentAnswer -->
<!-- source: internal/component/plugin/server/dispatch_registry.go -- recordAnswer, opDispatchCommand, opDispatchCommandArgs -->
<!-- source: internal/component/plugin/ipc/rpc.go -- PluginConn.SendExecuteCommandAnswer -->

**The frame is the same whatever the payload is.** All three methods below
answer with a head, their records and a terminator, a built value included. A
reader knows which frame is arriving before it reads the first line, so the
payload cannot choose it. A payload built whole is the one document a bounded
walk collapses to, and the terminator counts one record.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- executeCommandAnswer -->

Three methods take this path:

| Method | Who asks | Who writes the answer |
|--------|----------|-----------------------|
| `ze-plugin-engine:dispatch-command` | the plugin | `WriteAnswer`, `internal/component/plugin/dispatch.go` |
| `ze-plugin-engine:dispatch-command-args` | the plugin | the same writer |
| `ze-plugin-callback:execute-command` | the engine | `Records.WriteAnswer`, `pkg/plugin/records.go` |
<!-- source: internal/component/plugin/server/dispatch_registry.go -- serveEngineOpJSON, recordAnswer.write -->
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- Plugin.answerExecuteCommand -->

Every other engine op returns its own typed output on one line, and so does
every startup RPC.

The in-process `DirectBridge` is the one transport with no line to carry a
record on. It replaces the pipe for an internal plugin, so its dispatch result
is the single marshaled `DispatchCommandOutput` value the answer collapses to.
An in-process caller that wants the records themselves takes the typed answer
slot instead.
<!-- source: internal/component/plugin/server/dispatch_registry.go -- serveEngineOpDirect -->
<!-- source: pkg/plugin/rpc/bridge.go -- DirectBridge.DispatchCommandAnswer -->

The grammar is closed. `ParseAnswerTail` refuses a kind it does not know and a
line carrying bytes past its last field, so anything added to a line makes that
whole line unreadable to a peer built against the grammar without it.
<!-- source: pkg/plugin/rpc/message.go -- ParseAnswerTail, parseAnswerHead, parseAnswerTerminator -->

### The kind says what the line is

The field after the id is a three-byte kind token, and it states what the line
IS. A reader knows that before it reads one byte of the tail, so no consumer
parses a payload to find out what it holds.

| Kind | The line |
|------|----------|
| `top` | opens the answer and states how its records are read |
| `row` | one row the command produced |
| `bad` | one row the command rejected. The walk goes on |
| `end` | ends the answer and carries its counts |
| `nay` | the whole answer to a command text naming no command |
<!-- source: pkg/plugin/rpc/message.go -- AnswerKindHead, AnswerKindRecord, AnswerKindFault, AnswerKindTerminator, AnswerKindNotUnderstood -->

Every kind is three bytes, so a reader reaches it by arithmetic and searches no
line for a separator. The offset differs per channel and is fixed within one:

| Channel | The kind sits at |
|---------|------------------|
| plugin connection | `4 + <digits>`, where `<digits>` is the count the id's base-36 length character states |
| SSH exec channel | offset zero, because that channel carries one answer and writes no `#<len>:<id>` |
<!-- source: pkg/plugin/rpc/message.go -- answerKindWidth, answerKindAt, ParseAnswerLine -->

Each kind is a whole word rather than a stump, because a person reads a captured
session by eye, and byte 0 is distinct inside the set, so a machine switches on
one load.

### Fields

No key name reaches the wire. Every field sits at a fixed position for its kind,
and every variable-width field states its own width, so a reader reaches each one
by arithmetic and searches no line for a separator.

| Field shape | Written as | Used by |
|-------------|-----------|---------|
| word | three bytes, no length | the kind, and the head's item type |
| counted number | `<len>:<digits>`, `<len>` one base-36 character | the id, and the terminator's two counts |
| counted text | `<len>:<n>:<bytes>`, where `<len>:<n>` is a counted number stating the byte count | the envelope name, the column names, every message, the error code, and the record payload |
<!-- source: pkg/plugin/rpc/message.go -- appendCountedNumber, appendCountedText, cutCountedNumber, cutCountedText -->

A counted field of length zero is present and empty, never omitted, so the field
count of a kind never varies. A uint64 occupies 20 decimal digits at most, so one
base-36 length character expresses every counted number the protocol writes, and
a counted text carries a value of any length.

### Lines

| Line | Kind | Fields, in order | Position |
|------|------|------------------|----------|
| head | `top` | item type, envelope name, column names | first line for this id |
| result record | `row` | the row the command produced, counted | between head and terminator |
| error record | `bad` | the row the command rejected, counted | between head and terminator |
| terminator | `end` | records produced, rows rejected, message | last line for this id |
| not understood | `nay` | error code, message | the only line for this id |
<!-- source: pkg/plugin/rpc/message.go -- AppendAnswerHead, AppendAnswerItem, AppendAnswerFault, AppendAnswerTerminator, AppendAnswerNotUnderstood -->

EVERY value is counted, the record payload included, so a JSON value that holds
`=` or a space needs no escaping and a reader never looks for the newline to
find where a payload stops. The count is the payload's own byte count and never
the whole line's: the prefix in front of it differs per channel, and a payload
count is the same number on both.

**The head states no outcome.** The terminator carries the whole outcome, and two
lines stating one outcome can disagree. The head is written AFTER the body on the
SSH exec channel, so a status there was never a fact a consumer could commit to
on the first line either.
<!-- source: pkg/plugin/rpc/message.go -- AppendAnswerHead, Verdict -->
<!-- source: internal/component/plugin/dispatch.go -- answerMessage -->

A `bad` record is a row the walk rejected, and it does not end the walk. The
records after it still arrive, and the terminator counts the two collections
separately:

```
#1:7 top map 1:5:peers 1:0:
#1:7 row 2:41:{"peer":"10.0.0.1","state":"established"}
#1:7 bad 2:60:{"path":"bgp/peer/10.0.0.2","message":"nexthop unreachable"}
#1:7 end 1:1 1:1 1:0:
```
<!-- source: pkg/plugin/rpc/message_test.go -- TestAnswerLineTableMatchesDoc -->

`TestAnswerLineTableMatchesDoc` builds each of those lines with the shipped
appenders and requires this file to carry it byte for byte, so the published
grammar cannot drift from the writer.

A record too wide for one line is rejected the same way. Every record is one
line, and a line holds at most 16 MB, so a wider record has no wire form at all.
The encoder writes a `bad` record in its place, which names the record by its
position in the walk and states the two sizes, and the walk continues to its
terminator. The rejected row quotes none of the record, because a row that
carried 16 MB would not fit the line either.

```
#1:7 bad 3:117:{"message":"answer record does not fit one wire message","record":12,"encoded-bytes":16777300,"limit-bytes":16777216}
```
<!-- source: pkg/plugin/rpc/answer_write.go -- boundedRecord, answerRecordTooLargeFault -->

### The terminator states no status

The verdict is DERIVED from the counts the terminator already carries. A stated
status would be a second source of truth for a fact the counts hold, and two
sources can disagree.

| Terminator | Verdict |
|------------|---------|
| N records, no rejected row, no message | `done` |
| N records and M rejected, N more than 0, no message | `partial` |
| no record and M rejected, no message | `error`, nothing succeeded |
| a message over no record and no rejected row | `error`, the command failed before it produced anything |
| a message over records or rejected rows | `aborted`, and the count states how far the walk got |
| no terminator | `truncated` |
<!-- source: pkg/plugin/rpc/message.go -- Verdict, VerdictDone, VerdictPartial, VerdictError, VerdictAborted, VerdictTruncated -->

The message makes an aborted walk expressible when no row faulted.
`end 3:417 1:0 2:20:rib snapshot expired` is neither done nor partial, and the
counts alone cannot say so. The count is also what tells a command that failed
outright from a walk that stopped part way: both state a message, and only the
second one produced rows.

A missing terminator makes truncation detectable. A connection that dies part way
leaves the records that arrived. No line then states how many there were.

### The item type says what each record IS

| Item type | Each record carries |
|-----------|---------------------|
| `doc` | the whole answer as one JSON document |
| `map` | one map of names to values |
| `tab` | one tabular row, read against the head's column names |
<!-- source: pkg/plugin/rpc/message.go -- AnswerTypeDocument, AnswerTypeMap, AnswerTypeTable, checkAnswerType -->

The column names carry the schema of a `tab` answer, in column order. A reader
zips each positional row back into an object against it.
<!-- source: pkg/plugin/rpc/answer_row.go -- zipRow, quoteFields -->
<!-- source: pkg/plugin/rpc/collapse.go -- CollapseRecords -->

Each token is a whole word, and each says what a record IS rather than naming a
serialization. That is what ends the collision this page used to apologize for:
`| json` and `| ndjson` are renderings an operator asked for, and no item type
shares a word with one.

A `| ndjson` chain over a bounded answer renders one line per record from a `doc`
body. A `| table` chain over a long answer renders one table from a `map` answer.
<!-- source: internal/component/command/render_records.go -- RenderRecords, streamsPerRecord -->

### The encoder decides the type, never the command

A handler returns a row generator and states nothing about the wire. The encoder
holds up to `AnswerBufferThreshold` records while it decides.

| The walk | Head | Body |
|----------|------|------|
| ends at or under 256 records | `doc`, no envelope name | one record holding the whole document, or none when the command reported nothing |
| passes 256, no columns declared | `map` under the envelope name | one record per row |
| passes 256, columns declared | `tab` under the envelope name and the column names | one positional record per row |
<!-- source: pkg/plugin/rpc/message.go -- AnswerBufferThreshold -->
<!-- source: internal/component/plugin/dispatch.go -- WriteAnswer -->
<!-- source: pkg/plugin/rpc/answer_write.go -- WriteRecordAnswer, WriteDocumentAnswer, AnswerStreamType -->

A walk that passes the threshold flushes the held records in walk order ahead of
the rest. A bounded answer therefore keeps the JSON its command answered with
before it produced records at all, and a long answer never materializes.

The terminator counts the records the walk produced, never the `row` lines, so it
means the same thing whichever type carried them. A payload that is not a
generator is one document, and the count is 1. A command that answered with no
data at all writes no `row` line and counts 0.

The head of a `doc` answer names no envelope. The document already holds the
envelope, and two statements of one fact can disagree.

The threshold is a constant, not a config knob. An operator who tuned it would
choose the wire shape of every command's answer for every consumer at once. The
two shapes carry the same data, so there is nothing to prefer.

`| first 10` never reaches the decision point. The chain runs before the
threshold is measured, so ten records is a bounded answer however long the walk
behind it is. The consumer stops the generator inside the buffering window.
<!-- source: internal/component/command/pipe_records.go -- ApplyPipesRecords, recordsFirst -->

### Not understood against understood-then-failed

| The daemon | Answer |
|-----------|--------|
| did not understand the command | a `nay` line carrying the reason, with an optional error code. The only line for this id |
| understood it and it failed | a `top` line, then an `end` line carrying the reason over a count of zero |
<!-- source: pkg/plugin/rpc/message.go -- AppendAnswerNotUnderstood, AnswerKindNotUnderstood -->
<!-- source: internal/component/ssh/answer.go -- writeExecFailure -->

```
#1:7 nay 1:0: 2:31:unknown command: shwo bgp peers
```

A client therefore offers completion for the first, and an operational message
for the second.

### Faults on the buffered path

The `CommandDispatcher.JSON` consumers (web, MCP, REST, gRPC, looking glass) read
the whole answer as one string. The collapse puts rejected rows under a SIBLING
key rather than dropping them.

| Answer | Buffered rendering |
|--------|--------------------|
| rows, no rejected row | `{"peers":[...]}`, or a bare `[...]` when the handler names no envelope. Unchanged |
| rows and rejected rows | `{"peers":[...],"errors":[...]}` |
| rejected rows, no envelope key | `{"data":[...],"errors":[...]}` |
| the handler names its envelope `errors` | refused on both paths |
<!-- source: internal/component/plugin/types.go -- Records.MarshalJSON -->
<!-- source: pkg/plugin/records.go -- Records.MarshalJSON -->
<!-- source: pkg/plugin/rpc/collapse.go -- CollapseRecords, AnswerErrorsKey, AnswerDefaultKey, ErrReservedEnvelopeKey -->

`errors` appears only when a row faulted, so an ordinary answer keeps the shape
it had. `data` is where the rows go when the handler names no envelope and a row
faulted: a bare array has nowhere to carry a sibling.

A commit that applied 97 leaves and rejected 3 renders both on a web page. The 97
are no longer lost with the error.

### SSH exec channel

The exec channel splits the answer across its two streams, because the daemon
renders. The format an operator sees comes from `ze.cli.format` in the daemon's
configuration, and four of the six renderings run to several lines. Rendered text
cannot travel inside a `row` record, which is one line by construction.

| Stream | Carries | When |
|--------|---------|------|
| stdout | the rendering, written as the records arrive | always |
| stderr | head, terminator, or the `nay` line | always |
<!-- source: internal/component/ssh/answer.go -- answerFrame, writeExecAnswer, writeExecRecords, writeExecDocument -->

Here the head's item type describes what the ANSWER turned out to be. It does
not describe the rendering on stdout, which the operator's chain decides.

Every session gets the frame. A plain `ssh <host> <command>` receives it on
stderr with no env request and no setup, and a client that reads only stdout
takes the rendering and ignores it.
<!-- source: internal/component/ssh/answer.go -- newAnswerFrame -->
<!-- source: internal/core/ssh/client/answer.go -- ExecCommandStream, readAnswerFrame, ErrAnswerTruncated -->

The head is written after the body, because the type is read from the walk. The
two streams are read independently, so no reader can order them anyway.

Exit codes are unchanged: 0 for success, 1 for failure. The verdict is on the
frame, never in the exit status.
<!-- source: internal/component/cli/client/main.go -- runBGP -->

### What produces records today

Two kinds of handler answer with a row generator. `system command list` is the
engine's. A plugin command handler is the SDK's: it returns a `plugin.Records`
and the SDK writes one line for each row of the walk. Every other command
returns a built payload, which the encoder answers as one `doc` record.
<!-- source: internal/component/plugin/server/system.go -- handleSystemCommandList, commandRows -->
<!-- source: pkg/plugin/records.go -- Records, Records.WriteAnswer -->

`Records.Fields` has no producer, so no command writes a `tab` answer today. The
column order a renderer applies still lives in the separate registry
`RegisterColumns` writes.
<!-- source: internal/component/plugin/types.go -- Records.Fields -->
<!-- source: internal/component/command/column_order.go -- RegisterColumns -->

---

## Command-Dispatch Namespaces

The names in this section are command dispatcher paths. A plugin sends a path
inside `ze-plugin-engine:dispatch-command`. These paths are not plugin RPC
methods or response verbs.
<!-- source: pkg/plugin/sdk/sdk_engine.go -- Plugin.DispatchCommand -->

### Plugin Namespace

Plugin lifecycle operations:

| Command | Description |
|---------|-------------|
| `plugin help` | List plugin subcommands |
| `plugin command list` | List plugin commands |
| `plugin command help "<cmd>"` | Command details |
| `plugin command complete "<partial>"` | Completion |
| `plugin session ready` | Signal plugin init complete |
| `plugin session ping` | Health check (returns PID) |
| `plugin session bye` | Disconnect |
<!-- source: internal/core/ipc/yang/ze-plugin-api.yang -- plugin lifecycle RPCs -->

### BGP Namespace

**Introspection:**

| Command | Description |
|---------|-------------|
| `bgp help` | List bgp subcommands |
| `bgp command list` | List bgp commands |
| `bgp command help "<cmd>"` | Command details |
| `bgp command complete "<partial>"` | Completion |
| `bgp event list` | List available BGP event types |

**Plugin Configuration:**

| Command | Description |
|---------|-------------|
| `bgp plugin encoding json\|text` | Set event encoding format |
| `bgp plugin format hex\|base64\|parsed\|full` | Set wire bytes format (JSON only) |
| `bgp plugin ack sync\|async` | Set ACK timing |
<!-- source: internal/component/bgp/yang/ze-bgp-api.yang -- plugin-encoding, plugin-format, plugin-ack -->

Format relationship:
- `encoding text` → always parsed (human readable)
- `encoding json` → format applies (hex, base64, parsed, or full)

| Format | Wire Bytes | Parsed Fields | Use Case |
|--------|------------|---------------|----------|
| `hex` | ✅ as hex string | ❌ | Wire-level debugging |
| `base64` | ✅ as base64 | ❌ | Binary transport |
| `parsed` | ❌ | ✅ | Most plugins (default) |
| `full` | ✅ as hex | ✅ | Debugging + analysis |

**Timing:** Encoding applies at event delivery time. Subscribe first, configure encoding later—events use current encoding when delivered.

**Peer Operations:**

Selector patterns: `*` (all), `<ip>` (specific), `!<ip>` (all except)

| Command | Description |
|---------|-------------|
| `bgp peer <sel> list` | List matching peers (brief) |
| `bgp peer <sel> show` | Show matching peers (detailed) |
| `bgp peer <sel> teardown [subcode]` | Graceful close (NOTIFICATION) |
| `bgp peer <sel> update text\|hex\|base64 ...` | Announce/withdraw routes |
| `bgp peer <sel> borr <family>` | Begin-of-Route-Refresh (RFC 7313) |
| `bgp peer <sel> eorr <family>` | End-of-Route-Refresh (RFC 7313) |
| `bgp peer <sel> ready` | Signal peer replay complete |
| `bgp peer <sel> tcp reset` | Force TCP RST |
| `bgp peer <sel> tcp ttl <num>` | Set TTL (multi-hop) |
<!-- source: internal/component/bgp/yang/ze-bgp-api.yang -- peer RPCs -->

**Watchdog:**

| Command | Description |
|---------|-------------|
| `request bgp watchdog announce <name>` | Send all routes in pool |
| `request bgp watchdog withdraw <name>` | Withdraw all routes in pool |

**Commits (Batching):**

| Command | Description |
|---------|-------------|
| `bgp request commit <name> start` | Begin batch |
| `bgp request commit <name> end` | Flush batch |
| `bgp request commit <name> eor` | Flush + send EOR |
| `bgp request commit <name> rollback` | Discard batch |
| `bgp request commit <name> show` | Show queued count |
| `bgp request commit list` | List active batches |
<!-- source: internal/component/bgp/transaction/commit_manager.go -- CommitManager -->
<!-- source: internal/component/bgp/plugins/cmd/commit/commit.go -- commit handlers -->

**Raw Passthrough:**

| Command | Description |
|---------|-------------|
| `bgp raw <type> <enc> <data>` | Send raw BGP message |
<!-- source: internal/component/bgp/plugins/cmd/raw/ -- raw passthrough handler -->

### System Namespace

| Command | Description |
|---------|-------------|
| `system help` | List system subcommands |
| `system command list` | List system commands |
| `system command help "<cmd>"` | Command details |
| `system command complete "<partial>"` | Completion |
| `system subsystem list` | List available subsystems |
| `system version software` | Ze version |
| `system version api` | IPC protocol version |
| `request shutdown` | Gracefully shutdown |
| `show status` | Show process status |
| `request reload` | Reload the configuration |
<!-- source: internal/core/ipc/yang/ze-system-api.yang -- system RPCs -->
<!-- source: internal/component/plugin/server/handler.go -- APIVersion -->

### RIB Namespace

**Built-in:**

| Command | Description |
|---------|-------------|
| `rib help` | List rib subcommands |
| `rib command list` | List rib commands |
| `rib command help "<cmd>"` | Command details |
| `rib command complete "<partial>"` | Completion |
| `rib event list` | List available RIB event types |
| `show bgp rib received [peer]` | Show Adj-RIB-In |
| `clear bgp rib in [peer]` | Clear Adj-RIB-In |

**Future (BGP cache in bgp subsystem):**

| Command | Description |
|---------|-------------|
| `bgp cache forward <id> <sel>` | Forward cached UPDATE to peers |
| `bgp cache forward <id1>,<id2>,...,<idN> <sel>` | Batch forward (comma-separated IDs) |
| `bgp cache retain <id>` | Keep in cache until released |
| `bgp cache release <id>` | Allow eviction (TTL-based) |
| `bgp cache release <id1>,<id2>,...,<idN>` | Batch release |
| `bgp cache expire <id>` | Remove immediately |
| `bgp cache list` | List cached IDs |
<!-- source: internal/component/bgp/reactor/reactor.go -- cache management -->

---

## Event Subscription

Plugins subscribe to events via commands (not config).

### Subscription Commands

```
request subscribe [peer <sel> | plugin <name>] <namespace> event <type> [direction received|sent|both]
request unsubscribe [peer <sel> | plugin <name>] <namespace> event <type> [direction received|sent|both]
```

Selector patterns:
- `peer <sel>` - filter by peer: `*` (all), `<ip>` (specific), `!<ip>` (all except)
- `plugin <name>` - filter by plugin name

Direction (for message events):
- `received` - messages received from peer
- `sent` - messages sent to peer
- `both` - both directions (default if omitted)

Examples:

```
request subscribe bgp event update                              # all peers, both directions
request subscribe bgp event update direction received           # all peers, received only
request subscribe peer upstream1 event update                   # specific peer, both directions
request subscribe peer * event state                            # explicit all peers
request subscribe peer !upstream1 event update direction sent   # all except one, sent only
request subscribe plugin rib-cache rib event cache              # events from specific plugin
request subscribe rib event route                               # RIB route events
```

### BGP Event Types

| Event | Has Direction | Description |
|-------|---------------|-------------|
| `update` | ✅ | UPDATE message |
| `open` | ✅ | OPEN message |
| `notification` | ✅ | NOTIFICATION message |
| `keepalive` | ✅ | KEEPALIVE message |
| `refresh` | ✅ | ROUTE-REFRESH message |
| `state` | ❌ | Peer state change (up/down) |
| `negotiated` | ❌ | Capability negotiation complete |

### RIB Event Types

| Event | Description |
|-------|-------------|
| `cache` | Msg-id cache event (new entry, eviction) |
| `route` | Route change (add/remove) |

RIB events include `peer` field indicating which peer caused the event.
<!-- source: internal/component/bgp/plugins/rib/yang/ze-rib-api.yang -- RIB event types -->

### Event Examples

**BGP UPDATE received from peer:**
```json
{
  "type": "bgp",
  "bgp": {
    "type": "update",
    "peer": {"address": "192.0.2.1", "local": {"address": "192.0.2.2", "as": 65000}, "remote": {"address": "192.0.2.1", "as": 65001}},
    "update": {
      "message": {"id": 123, "direction": "received"},
      "attr": {
        "origin": "igp",
        "as-path": [65001, 65002],
        "local-preference": 100
      },
      "nlri": {
        "ipv4/unicast": [
          {"action": "add", "next-hop": "192.0.2.1", "nlri": ["10.0.0.0/24"]}
        ]
      }
    }
  }
}
```

**BGP UPDATE sent to peer (locally-originated):**
```json
{
  "type": "bgp",
  "bgp": {
    "type": "update",
    "peer": {"address": "192.0.2.1", "local": {"address": "192.0.2.2", "as": 65000}, "remote": {"address": "192.0.2.1", "as": 65001}},
    "update": {
      "message": {"id": 0, "direction": "sent"},
      "attr": {
        "origin": "igp",
        "as-path": [65000]
      },
      "nlri": {
        "ipv4/unicast": [
          {"action": "add", "next-hop": "192.0.2.254", "nlri": ["172.16.0.0/16"]}
        ]
      }
    }
  }
}
```

**Note:** `bgp.update.message.id: 0` indicates locally-originated route (no cache entry for forwarding).

**BGP UPDATE with raw wire bytes (format=full):**
```json
{
  "type": "bgp",
  "bgp": {
    "type": "update",
    "peer": {"address": "192.0.2.1", "local": {"address": "192.0.2.2", "as": 65000}, "remote": {"address": "192.0.2.1", "as": 65001}},
    "update": {
      "message": {"id": 123, "direction": "received"},
      "attr": {
        "origin": "igp",
        "as-path": [65001]
      },
      "nlri": {
        "ipv4/unicast": [
          {"action": "add", "next-hop": "192.0.2.1", "nlri": ["10.0.0.0/24"]}
        ]
      },
      "raw": {
        "attr": "40010100400200040001fde8",
        "nlri": {"ipv4/unicast": "180a0000"},
        "withdrawn": {}
      }
    }
  }
}
```

**Peer state change (up):**
```json
{
  "type": "bgp",
  "bgp": {
    "type": "state",
    "peer": {"address": "192.0.2.1", "local": {"address": "192.0.2.2", "as": 65000}, "remote": {"address": "192.0.2.1", "as": 65001}},
    "state": "up"
  }
}
```

**Peer state change (down with reason):**
```json
{
  "type": "bgp",
  "bgp": {
    "type": "state",
    "peer": {"address": "192.0.2.1", "local": {"address": "192.0.2.2", "as": 65000}, "remote": {"address": "192.0.2.1", "as": 65001}},
    "state": "down",
    "reason": "hold timer expired"
  }
}
```

**RIB cache event:**
```json
{
  "type": "rib",
  "rib": {
    "type": "cache",
    "action": "new",
    "msg-id": 12345,
    "peer": {"address": "192.0.2.1", "local": {"address": "192.0.2.2", "as": 65000}, "remote": {"address": "192.0.2.1", "as": 65001}}
  }
}
```

---

## Plugin Startup Protocol

Five-stage synchronized startup with barriers:

```
Stage 1: REGISTRATION     Plugin → Engine: declare cmd/conf/receive, declare done
Stage 2: CONFIG           Engine → Plugin: config peer <addr> <key> <value>, config done
Stage 3: CAPABILITY       Plugin → Engine: capability hex <code> <value>, capability done
Stage 4: REGISTRY         Engine → Plugin: registry cmd <name>, registry done
Stage 5: READY            Plugin → Engine: plugin session ready
```

**Note:** During startup stages 1-4, plugins use minimal commands (`declare`, `capability`, `config`). After `plugin session ready`, the full command namespace is available.

**After ready:** Plugin configures itself and subscribes to events:

```
plugin session ready

bgp plugin encoding json
bgp plugin format full
bgp plugin ack async

request subscribe bgp event update direction received
request subscribe bgp event update direction sent
request subscribe peer * event state
request subscribe rib event cache
request subscribe rib event route
```

**Barrier semantics:** All plugins must complete each stage before any proceed to next.
<!-- source: internal/core/ipc/yang/ze-plugin-engine.yang -- startup RPCs -->
<!-- source: internal/core/ipc/yang/ze-plugin-callback.yang -- callback RPCs -->

**Timeout:** 5s per stage (configurable via `timeout` in plugin config).

**Subscription timing:** Subscriptions receive FUTURE events only. To get current state:
1. Subscribe to events first
2. Query `bgp peer * show` for existing peer states
3. Process both query results and incoming events

---

## Subsystem Discovery

Before subscribing to events from a subsystem, plugins should check availability:

```
system subsystem list
```

Response:
```json
{"type":"response","response":{"status":"done","data":{"subsystems":["bgp","rib"]}}}
```

Subscribing to unavailable subsystem returns error:
```json
{"type":"response","response":{"status":"error","data":"rib not available"}}
```

---

## Event Discovery

Each subsystem that supports events provides an `event list` command:

```
bgp event list
```

Response:
```json
{"type":"response","response":{"status":"done","data":{"events":["update","open","notification","keepalive","refresh","state","negotiated"]}}}
```

```
rib event list
```

Response:
```json
{"type":"response","response":{"status":"done","data":{"events":["cache","route"]}}}
```

---

## Error Codes

Errors are returned as strings in `data` field:

| Error Message | Cause |
|---------------|-------|
| `unknown command` | Unrecognized verb |
| `invalid peer address` | Malformed IP in selector |
| `no peers match selector` | No peers found for selector |
| `no peers have family negotiated` | No peers support requested AFI/SAFI |
| `stale re-advertise withheld the route: an egress filter panicked` | An LLGR (RFC 9494) stale re-advertise ran an egress filter that crashed, so nothing was decided for that peer |
| `stale re-advertise withheld the route: the modified UPDATE body could not be built` | The filter decided (Section 4.6 depreference), and the modified body could not be encoded |
| `invalid attribute` | Unrecognized attribute name |
| `missing required attribute` | Required field not provided |
| `parse error: <detail>` | Syntax error in command |
| `timeout` | Request timed out (default 30s) |
| `process not ready` | Plugin not yet at READY stage |
| `queue full` | Write queue backpressure triggered |
| `<subsystem> not available` | Subsystem not configured |

---

## Backpressure

| Constant | Value | Description |
|----------|-------|-------------|
| `WRITE_QUEUE_HIGH_WATER` | 1000 | Pause writes at this queue depth |
| `WRITE_QUEUE_LOW_WATER` | 100 | Resume when drained to this level |
| `PENDING_REQUEST_LIMIT` | 100 | Max pending requests per process |
| `DEFAULT_TIMEOUT` | 30s | Request timeout |
| `COMPLETION_TIMEOUT` | 500ms | Tab completion timeout |
| `RESPAWN_LIMIT` | 5 | Max respawns per 60s |
<!-- source: internal/component/plugin/process/process.go -- Process backpressure -->

When backpressure triggers:
1. Events dropped for affected process
2. Warning logged
3. Counter incremented
4. Resumes when queue drains

---

## Text Format (Alternative Encoding)

When `bgp plugin encoding text` is set, events use human-readable text format (not JSON wrapper):

```
peer <ip> remote as <asn> <direction> <type> <msg-id> <fields...>
```

State events use `peer <ip> remote as <asn> state <state> [reason <reason>]`. Text format intentionally stays flat for human readability. No JSON wrapping is applied.
<!-- source: internal/component/bgp/format/text_human.go -- appendStateChangeText, appendFilterResultText -->
<!-- source: internal/component/bgp/format/text.go -- AppendOpen, AppendNotification, AppendKeepalive, AppendRouteRefresh -->

Examples:

```
peer 192.0.2.1 remote as 65001 state up
peer 192.0.2.1 remote as 65001 received update 1 origin igp path 65001 next 192.0.2.1 nlri ipv4/unicast add 10.0.0.0/24
peer 192.0.2.1 remote as 65001 sent keepalive 42
```

---

## Plugin Wire Format

All plugins use the unified `#<len>:<id> <verb> [<json>]\n` wire format (see `wire-format.md`).
There is no separate text mode. The same newline-delimited framing is used for both
the 5-stage startup handshake and post-startup concurrent RPCs.
<!-- source: pkg/plugin/rpc/mux.go -- MuxConn -->

---

## Plugin Registration

Plugins can register custom commands:

| Command | Description |
|---------|-------------|
| `register command "<name>" description "<help>" [args "<usage>"] [completable] [timeout <dur>]` | Register command |
| `unregister command "<name>"` | Unregister command |
<!-- source: internal/component/plugin/server/command_registry.go -- CommandRegistry -->

---

## References

- `architecture.md` - Full API architecture
- `process-protocol.md` - Plugin startup details
- `commands.md` - Command syntax details
- `cli.md` - JSON output format details
- `capability-contract.md` - GR/RR capability handling
