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

Engine provides reactor methods; plugins register commands that use them.

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
#<id> <verb> [<json>]\n
```

Each line is a complete message. The `#` prefix and decimal `uint64` ID are
required. The optional payload is compact JSON.
<!-- source: pkg/plugin/rpc/framing.go -- FrameReader, FrameWriter -->
<!-- source: pkg/plugin/rpc/message.go -- ParseLine -->

### RPC Request (Either Direction)

```
#42 ze-plugin-engine:subscribe-events {"events":["update"]}
#43 ze-plugin-callback:deliver-batch {"events":[{"type":"bgp"}]}
```

For a request, the verb is a method name. A plugin-to-engine method uses the
`ze-plugin-engine:` prefix. An engine callback uses the
`ze-plugin-callback:` prefix.
<!-- source: pkg/plugin/rpc/message.go -- AppendRequest -->

### RPC Response (Either Direction)

```
#42 ok
#43 error {"code":"invalid-event","message":"unknown event"}
```

| Component | Requirement |
|-----------|-------------|
| `#<id>` | Echoes the request ID |
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

Every plugin RPC request has a `#<uint64>` correlation ID. The peer returns a
response with the same ID. Every request except `dispatch-command` and
`dispatch-command-args` receives exactly one response line. Those two receive a
sequence of lines when the peer negotiated it: see Answer Protocol below.
<!-- source: pkg/plugin/rpc/message.go -- ParseLine, AppendRequest, AppendResult, AppendError -->

The command dispatcher is a payload protocol inside plugin RPC. For a peer that
negotiated nothing:

```
#1 ze-plugin-engine:dispatch-command {"command":"show bgp peer * detail"}
#1 ok {"status":"done","data":{"peers":[]}}
```

The outer `ok` is the plugin RPC response verb. The inner `status` and `data`
members belong to `DispatchCommandOutput`. They do not change RPC framing.
<!-- source: pkg/plugin/rpc/types.go -- DispatchCommandInput, DispatchCommandOutput -->
<!-- source: internal/core/ipc/message.go -- MapResponse -->

---

## Answer Protocol

A command answer is a sequence of lines: a head, zero or more records, and a
terminator. One code path writes every answer, whatever its record count, so a
reader follows one path and nothing declares a shape the payload can contradict.
<!-- source: pkg/plugin/rpc/message.go -- AppendAnswerHead, AppendAnswerItem, AppendAnswerFault, AppendAnswerTerminator -->

Each line is `#<id> ok <tail>`, and the tail is bare `key=value` pairs, so a
reader decides how to take the answer without a JSON decoder. On the SSH exec
channel there is no `#<id>`: one command owns the channel, so nothing needs to be
told apart.
<!-- source: pkg/plugin/rpc/message.go -- AnswerNoID, ParseAnswerLine -->

### Negotiation

The default is off. A peer receives the sequence only after it names
`record-answers` in the `protocol` list of its Stage 3 `declare-capabilities`:

```
#2 ze-plugin-engine:declare-capabilities {"capabilities":[],"protocol":["record-answers"]}
#2 ok
```

The guard fails closed. An absent list, an empty list, an unknown name, and a
peer whose Stage 3 has not run all read false. A plugin written before this
shape existed therefore receives `#<id> ok [<json>]` exactly as before.
<!-- source: pkg/plugin/rpc/types.go -- ProtocolRecordAnswers, DeclareCapabilitiesInput.Understands -->
<!-- source: internal/component/plugin/process/process.go -- Process.RecordAnswers -->

Only `dispatch-command` and `dispatch-command-args` take this path. Every other
engine op returns its own typed output, and the startup RPCs keep the
single-line frame whatever the peer declared. That is what lets Stage 3 itself
be answered before the shape is known.
<!-- source: internal/component/plugin/server/dispatch_registry.go -- answerResult, opDispatchCommand, opDispatchCommandArgs -->

A later shape earns a NEW name in `protocol`. `ParseAnswerTail` refuses a key it
does not know. A key added under an agreed name would make the whole line
unreadable to the peer that agreed to the older spelling.
<!-- source: pkg/plugin/rpc/message.go -- ParseAnswerTail -->

### Lines

| Line | Verb | Required keys | Optional keys | Position |
|------|------|---------------|---------------|----------|
| head | `ok` | `status=`, `type=` | `key=`, and `fields=` when `type=stream` | first line for this id |
| result record | `ok` | `item=` | - | between head and terminator |
| error record | `ok` | `fault=` | - | between head and terminator |
| terminator | `ok` | `count=` | `faults=`, `message=` | last line for this id |
| not understood | `error` | `message=` | `code=` | the only line for this id |
<!-- source: pkg/plugin/rpc/message.go -- AnswerTail, AnswerTail.IsTerminator, AppendAnswerNotUnderstood -->

`item=`, `fault=`, `message=` and `fields=` take the rest of the line verbatim,
and each is written last. A JSON value that holds `=` or a space therefore needs
no escaping. At most one open-ended value sits on a line.
<!-- source: pkg/plugin/rpc/message.go -- isOpenEndedKey, AppendAnswerHead -->

`status=` is `done` or `error`, and every head carries it. It states what the
daemon knows when the answer opens. A consumer that renders a failure differently
therefore commits to a rendering on the first line, rather than buffering the
whole answer to find out.
<!-- source: pkg/plugin/rpc/types.go -- StatusDone, StatusError -->
<!-- source: internal/component/plugin/dispatch.go -- answerStatus -->

A `fault=` record is a row the walk rejected, and it does not end the walk. The
records after it still arrive, and the terminator counts the two collections
separately:

```
#7 ok status=done type=ndjson key=peers
#7 ok item={"peer":"10.0.0.1","state":"established"}
#7 ok fault={"path":"bgp/peer/10.0.0.2","message":"nexthop unreachable"}
#7 ok count=1 faults=1
```

A record too wide for one line is rejected the same way. Every record is one
line, and a line holds at most 16 MB, so a wider record has no wire form at all.
The encoder writes a `fault=` in its place, which names the record by its
position in the walk and states the two sizes, and the walk continues to its
terminator. The rejected row quotes none of the record, because a row that
carried 16 MB would not fit the line either.

```
#7 ok fault={"message":"answer record does not fit one wire message","record":12,"encoded-bytes":16777300,"limit-bytes":16777216}
```
<!-- source: internal/component/plugin/dispatch.go -- boundedRecord, answerRecordTooLargeFault -->

### The terminator states no status

The verdict is DERIVED from the counts the terminator already carries. A stated
status would be a second source of truth for a fact the counts hold, and two
sources can disagree.

| Terminator | Verdict |
|------------|---------|
| `count=N faults=0` | `done` |
| `count=N faults=M`, N more than 0 | `partial` |
| `count=0 faults=M` | `error`, nothing succeeded |
| any `message=` | `aborted`. `count=` states how far the walk got |
| no terminator | `truncated` |
<!-- source: pkg/plugin/rpc/message.go -- Verdict, VerdictDone, VerdictPartial, VerdictError, VerdictAborted, VerdictTruncated -->

`message=` makes an aborted walk expressible when no row faulted.
`count=417 message=rib snapshot expired` is neither done nor partial, and the
counts alone cannot say so.

A missing terminator makes truncation detectable. A connection that dies part way
leaves the records that arrived. No line then states how many there were.

### `type=` says how to take each `item=`

| `type=` | Each `item=` carries |
|---------|----------------------|
| `json` | the whole answer as one JSON document |
| `ndjson` | one self-describing object |
| `stream` | a positional array, read against the head's `fields=` |
<!-- source: pkg/plugin/rpc/message.go -- AnswerTypeJSON, AnswerTypeNDJSON, AnswerTypeStream, checkAnswerType -->

`fields=` carries the column schema of a `type=stream` answer, in column order. A
reader zips each positional row back into an object against it.
<!-- source: internal/component/plugin/answer_row.go -- zipRow, quoteFields -->
<!-- source: internal/component/plugin/types.go -- CollapseRecords -->

**`type=` and the pipe operators are unrelated, and the shared words are an
unresolved naming collision.** `| json` and `| ndjson` are renderings an operator
asked for. `type=json` and `type=ndjson` say how the daemon serialized the body.
A reader MUST NOT infer one from the other.

A `| ndjson` chain over a bounded answer renders one line per record from a
`type=json` body. A `| table` chain over a long answer renders one table from a
`type=ndjson` answer.
<!-- source: internal/component/command/render_records.go -- RenderRecords, streamsPerRecord -->

### The encoder decides the type, never the command

A handler returns a row generator and states nothing about the wire. The encoder
holds up to `AnswerBufferThreshold` records while it decides.

| The walk | Head | Body |
|----------|------|------|
| ends at or under 256 records | `status=<s> type=json`, no `key=` | one `item=` holding the whole document, or none when the command reported nothing |
| passes 256, no `fields` declared | `status=<s> type=ndjson key=<k>` | one `item=` per record |
| passes 256, `fields` declared | `status=<s> type=stream key=<k> fields=[...]` | one positional `item=` per record |
<!-- source: pkg/plugin/rpc/message.go -- AnswerBufferThreshold -->
<!-- source: internal/component/plugin/dispatch.go -- WriteAnswer, writeRecordAnswer, writeDocumentAnswer, answerStreamType -->

A walk that passes the threshold flushes the held records in walk order ahead of
the rest. A bounded answer therefore keeps the JSON its command answered with
before it produced records at all, and a long answer never materializes.

`count=` counts the records the walk produced, never the `item=` lines, so the
terminator means the same thing whichever type carried them. A payload that is
not a generator is one document, and `count=1`. A command that answered with no
data at all writes no `item=` line and states `count=0`.

The head of a `type=json` answer carries no `key=`. The document already holds
the envelope, and two statements of one fact can disagree.

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
| did not understand the command | `#<id> error message=<text>`, with an optional `code=`. The only line for this id |
| understood it and it failed | a head stating `status=error`, then a terminator carrying `message=` |
<!-- source: pkg/plugin/rpc/message.go -- AppendAnswerNotUnderstood, AnswerVerbError -->
<!-- source: internal/component/ssh/answer.go -- writeExecFailure -->

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
<!-- source: internal/component/plugin/types.go -- CollapseRecords, Records.MarshalJSON, answerErrorsKey, answerDefaultKey -->
<!-- source: internal/component/plugin/dispatch.go -- errReservedEnvelopeKey -->

`errors` appears only when a row faulted, so an ordinary answer keeps the shape
it had. `data` is where the rows go when the handler names no envelope and a row
faulted: a bare array has nowhere to carry a sibling.

A commit that applied 97 leaves and rejected 3 renders both on a web page. The 97
are no longer lost with the error.

### SSH exec channel

The exec channel splits the answer across its two streams, because the daemon
renders. The format an operator sees comes from `ze.cli.format` in the daemon's
configuration, and four of the six renderings run to several lines. Rendered text
cannot travel inside an `item=`, which is one line by construction.

| Stream | Carries | When |
|--------|---------|------|
| stdout | the rendering, written as the records arrive | always |
| stderr | head, terminator, or the `error` verb line | only for a client that declared the shape |
<!-- source: internal/component/ssh/answer.go -- answerFrame, writeExecAnswer, writeExecRecords, writeExecDocument -->

Here the head's `type=` describes what the ANSWER turned out to be. It does not
describe the rendering on stdout, which the operator's chain decides.

A client declares the shape by setting `ZE_ANSWER_PROTOCOL=record-answers` in an
SSH env request. A plain `ssh <host> <command>` declares nothing and its bytes
are what they always were.
<!-- source: pkg/plugin/rpc/message.go -- AnswerProtocolEnv -->
<!-- source: internal/component/ssh/answer.go -- declaresRecordAnswers -->
<!-- source: internal/core/ssh/client/answer.go -- ExecCommandStream, readAnswerFrame, ErrAnswerTruncated -->

The head is written after the body, because the type is read from the walk. The
two streams are read independently, so no reader can order them anyway.

Exit codes are unchanged: 0 for success, 1 for failure. The verdict is on the
frame, never in the exit status.
<!-- source: internal/component/cli/client/main.go -- runBGP -->

### What produces records today

`system command list` is the only handler that answers with a row generator.
Every other command returns a built payload, which the encoder answers as one
`type=json` document.
<!-- source: internal/component/plugin/server/system.go -- handleSystemCommandList, commandRows -->

`Records.Fields` has no producer, so no command writes `type=stream` today. The
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

All plugins use the unified `#<id> <verb> [<json>]\n` wire format (see `wire-format.md`).
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
