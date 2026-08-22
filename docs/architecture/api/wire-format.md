# Ze IPC Wire Format

## Transport

Messages are UTF-8 lines terminated by exactly one newline byte (0x0A). A line
MUST NOT be terminated by `\r\n`, and a reader MUST NOT strip a trailing `\r`.

A line that states its own width is taken by that width, and the byte after it
MUST be the newline. Every variable-width field of an answer line states its own
width, and a text field states it as a BYTE count, so a value inside one MAY hold
a raw newline or a carriage return: neither is a frame boundary and neither is
rewritten on the way to the wire. Every other line ends at its newline.
<!-- source: pkg/plugin/rpc/framing.go -- ScanAnswerLines, scanStatedLine -->

```
#1 ze-bgp:peer-list {"selector":"10.0.0.1"}
#1 ok {"peers":[{"address":"10.0.0.1","state":"established"}]}
```

### Framing

| Property | Value |
|----------|-------|
| Delimiter | Newline (0x0A) |
| Encoding | UTF-8 |
| Max message size | 16 MB (16,777,216 bytes) |
| Initial buffer | 64 KB |

Each line has the format: `#<id> <verb> [<json-payload>]`

- `#<id>` is a decimal integer (monotonically increasing per connection),
  closed by one space. A digit run a space terminates is unambiguous, so the id
  needs no count in front of it, and one fused loop reads the value while it
  walks to that space. A uint64 occupies 20 decimal digits at most, and an id
  wider than that is refused.
- `<verb>` is a method name (requests) or `ok`/`error` (responses).
- `<json-payload>` is optional compact JSON.

Implementation: `pkg/plugin/rpc/framing.go`, `pkg/plugin/rpc/message.go`
<!-- source: pkg/plugin/rpc/framing.go -- newline-delimited framing -->
<!-- source: pkg/plugin/rpc/conn.go -- Conn -->

## Method Naming

YANG RPC methods use `module:rpc-name` format. `WireModule` derives the method
prefix from the declaring module. RPC names use kebab-case.

| Wire Method | Declaration | Kind |
|-------------|-------------|------|
| `ze-bgp:peer-list` | `ze-bgp-api` | YANG RPC |
| `ze-bgp:subscribe` | `ze-cli-subscribe-cmd` | Command-dispatch method for `request subscribe` |
| `ze-system:daemon-status` | `ze-system-api` | YANG RPC |
| `ze-system:version-software` | `ze-system-api` | YANG RPC |
| `ze-system:command-list` | `ze-system-api` | YANG RPC |
| `ze-rib:show` | `ze-rib-api` | YANG RPC |
| `ze-plugin:session-ready` | `ze-plugin-api` | YANG RPC |
| `ze-plugin-engine:subscribe-events` | `ze-plugin-engine` | Plugin SDK RPC |

`WireModule` removes the `-api` suffix from a YANG RPC module. A `ze:command`
declaration can set a different wire method for command dispatch.
<!-- source: internal/component/config/yang/rpc.go -- WireModule -->
<!-- source: internal/component/bgp/yang/ze-bgp-api.yang -- peer-list -->
<!-- source: internal/component/cmd/subscribe/yang/ze-cli-subscribe-cmd.yang -- request subscribe -->
<!-- source: internal/core/ipc/yang/ze-system-api.yang -- daemon-status, version-software, command-list -->
<!-- source: internal/component/bgp/plugins/rib/yang/ze-rib-api.yang -- show -->
<!-- source: internal/core/ipc/yang/ze-plugin-api.yang -- session-ready -->
<!-- source: internal/core/ipc/yang/ze-plugin-engine.yang -- subscribe-events -->

## Request

```
#42 ze-bgp:peer-list {"selector":"10.0.0.1"}
#43 ze-plugin-engine:declare-registration {"families":[{"name":"ipv4/unicast","mode":"both"}]}
#44 ze-bgp:subscribe {"args":["bgp","event","update"]}
```

| Component | Description |
|-----------|-------------|
| `#<id>` | Correlation ID (decimal integer), closed by one space |
| `<method>` | `module:rpc-name` |
| `<json>` | Optional JSON params |

## Successful Response

```
#42 ok {"peers":[{"address":"10.0.0.1","state":"established"}]}
#43 ok
```

| Component | Description |
|-----------|-------------|
| `#<id>` | Echoed from request |
| `ok` | Success verb |
| `<json>` | Optional JSON result (absent for void responses) |

## Record Answer

Three methods have a second success form, and every peer uses it. The engine
writes it for `dispatch-command` and `dispatch-command-args`, and the plugin
writes it for `execute-command`: one encoding covers both directions. The answer
is a head, zero or more records, and a terminator. The field after the id is a
three-byte kind token stating what the line IS, and the fields after that are
positional rather than JSON and carry no key name:

```
#42 top map 5:peers 0:
|   |   |   | |     |
|   |   |   | |     +----- column names, 0 BYTES, so the records are not positional
|   |   |   | +----------- those 5 bytes: peers, the key the records go under
|   |   |   +------------- 5, the envelope name's BYTE count, then its colon
|   |   +----------------- item type map: each record is one map of names to values
|   +--------------------- kind top: the head, always the first line for this id
+------------------------- correlation id 42: the sigil, the digits, and the space

#42 row 44:{"address":"10.0.0.1","state":"established"}
|   |   |  |
|   |   |  +----- those 44 bytes, the row byte for byte
|   |   +-------- 44, the payload's BYTE count, then its colon
|   +------------ kind row: one record the walk produced
+---------------- correlation id 42

#42 end 1 0 0:
|   |   | | |
|   |   | | +----- message, 0 BYTES, so the walk stated none
|   |   | +------- 0 rows rejected
|   |   +--------- 1 record produced
|   +------------- kind end: the terminator, always the last line for this id
+----------------- correlation id 42
```

Five words say what a line IS, and each is three bytes closed by a space, so a
reader takes the four bytes as one load rather than searching for a separator.

| Word | The line is |
|------|-------------|
| `top` | the head, which opens the answer |
| `row` | one record the command produced |
| `bad` | one record the command rejected. The walk goes on |
| `end` | the terminator, which ends the answer |
| `nay` | the whole answer to a command text naming no command |

Three more words say how the records read, and the head carries one of them:

| Word | The records are |
|------|-----------------|
| `doc` | one document. The whole answer is that one value |
| `map` | one map of names to values for each record |
| `tab` | one positional row for each record, read against the head's column names |

Two field shapes carry every value. A NUMBER is decimal digits closed by a space
or by the end of the line, and it never carries a colon. A TEXT is decimal
digits, a colon, then that many BYTES. **The count is a BYTE count, never a count
of characters**, so a value holding multi-byte utf-8 is sliced by the bytes that
arrived. A text of zero bytes is written `0:`, present and empty, so a line's
field count never varies.

Neither the head nor the terminator states an outcome. The verdict is DERIVED
from the terminator's two counts and its message: no rejected row and no message
is `done`, some of each is `partial`, a message over no record is `error`, a
message over records is `aborted`, and a missing terminator is `truncated`. The
frame is the same whatever the payload is, so a handler that built one value
answers with the same three lines and the `doc` type.

[ipc_protocol.md](ipc_protocol.md), "Answer Protocol", carries the same grammar
with the buffering threshold, the over-wide record, and the failure cases beside
it.
<!-- source: pkg/plugin/rpc/message.go -- AppendAnswerHead, AppendAnswerItem, AppendAnswerTerminator, ParseAnswerTail, Verdict, AnswerKindHead, AnswerKindTerminator -->
<!-- source: pkg/plugin/rpc/answer_write.go -- WriteRecordAnswer, WriteDocumentAnswer -->

## Error Response

```
#42 error {"code":"peer-not-found","message":"no peer at 10.0.0.99"}
#43 error {"message":"unknown method"}
```

| Component | Description |
|-----------|-------------|
| `#<id>` | Echoed from request |
| `error` | Error verb |
| `<json>` | Optional JSON with `code` and/or `message` fields |

## Batch Event Delivery

Events are delivered in batches for efficiency using a pooled buffer.

```
#7 ze-plugin-callback:deliver-batch {"events":["event1-json","event2-json"]}
#7 ok
```

Implementation: `pkg/plugin/rpc/batch.go`
<!-- source: pkg/plugin/rpc/batch.go -- batch event delivery -->

## Response Mapping

The `MapResponse()` function converts plugin Response fields to wire format.

Implementation: `internal/core/ipc/message.go`
<!-- source: internal/core/ipc/message.go -- MapResponse -->

## YANG API Modules

RPC definitions live in YANG API modules, separate from config modules:

| Module | File | Contains |
|--------|------|----------|
| ze-bgp-api | `internal/component/bgp/yang/ze-bgp-api.yang` | BGP RPCs and notifications |
| ze-system-api | `internal/core/ipc/yang/ze-system-api.yang` | System RPCs |
| ze-rib-api | `internal/component/bgp/plugins/rib/yang/ze-rib-api.yang` | RIB RPCs and notifications |
| ze-plugin-api | `internal/core/ipc/yang/ze-plugin-api.yang` | Plugin lifecycle RPCs |
| ze-plugin-engine | `internal/core/ipc/yang/ze-plugin-engine.yang` | Plugin-to-engine SDK RPCs |
| ze-cli-subscribe-cmd | `internal/component/cmd/subscribe/yang/ze-cli-subscribe-cmd.yang` | Event subscription command paths |

Shared IPC types (typedefs, groupings) live in `ze-types` (`internal/component/config/yang/modules/ze-types.yang`).
<!-- source: internal/component/bgp/yang/ze-bgp-api.yang -- BGP RPCs -->
<!-- source: internal/core/ipc/yang/ze-system-api.yang -- system RPCs -->
<!-- source: internal/core/ipc/yang/ze-plugin-engine.yang -- plugin-engine RPCs -->

## JSON Conventions

All JSON follows Ze [JSON format conventions](../../../ai/rules/cli.md#json-format):

- Keys use kebab-case (`"peer-count"`, not `"peerCount"`).
- Error identities use kebab-case (`"peer-not-found"`).
- Address families use `"afi/safi"` format (`"ipv4/unicast"`).
<!-- source: pkg/plugin/rpc/types.go -- DeclareRegistrationInput and other wire types -->
