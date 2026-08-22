# Ze IPC Wire Format

## Transport

Messages are UTF-8 lines terminated by a newline byte (0x0A).
Compact JSON never contains unescaped newlines, making newline an unambiguous frame delimiter.

```
#1:1 ze-bgp:peer-list {"selector":"10.0.0.1"}
#1:1 ok {"peers":[{"address":"10.0.0.1","state":"established"}]}
```

### Framing

| Property | Value |
|----------|-------|
| Delimiter | Newline (0x0A) |
| Encoding | UTF-8 |
| Max message size | 16 MB (16,777,216 bytes) |
| Initial buffer | 64 KB |

Each line has the format: `#<len>:<id> <verb> [<json-payload>]`

- `#<len>:<id>` is a decimal integer (monotonically increasing per connection),
  preceded by one base-36 character stating how many digits it occupies. A
  reader takes that byte and reaches the verb by addition, never by searching
  the line for a space. `0` to `9` then `A` to `Z` spell a length up to 35, and
  a uint64 occupies 20 decimal digits at most, so `K` is the widest length any
  id can state.
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
#2:42 ze-bgp:peer-list {"selector":"10.0.0.1"}
#2:43 ze-plugin-engine:declare-registration {"families":[{"name":"ipv4/unicast","mode":"both"}]}
#2:44 ze-bgp:subscribe {"args":["bgp","event","update"]}
```

| Component | Description |
|-----------|-------------|
| `#<len>:<id>` | Correlation ID (decimal integer), preceded by its base-36 digit count |
| `<method>` | `module:rpc-name` |
| `<json>` | Optional JSON params |

## Successful Response

```
#2:42 ok {"peers":[{"address":"10.0.0.1","state":"established"}]}
#2:43 ok
```

| Component | Description |
|-----------|-------------|
| `#<len>:<id>` | Echoed from request |
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
#2:42 top map 1:5:peers 1:0:
#2:42 row 2:44:{"address":"10.0.0.1","state":"established"}
#2:42 end 1:1 1:0 1:0:
```

`top` opens the answer, `row` and `bad` carry a produced row and a rejected one,
`end` ends it, and `nay` is the whole answer to a command text naming no
command. Every token is three bytes, so a reader reaches it by arithmetic from
the id's length character rather than by searching the line for a space.

The head's item type says what each record IS: `doc`, `map` or `tab`. Neither
the head nor the terminator states an outcome. The verdict is DERIVED from the
terminator's two counts and its message, and a missing terminator means the
answer was truncated. Every variable-width field states its own byte count, so a
reader reaches each one by arithmetic. The frame is the same whatever the payload
is, so a handler that built one value answers with the same three lines and the
`doc` type. The grammar is in
[ipc_protocol.md](ipc_protocol.md), "Answer Protocol".
<!-- source: pkg/plugin/rpc/message.go -- AppendAnswerHead, AppendAnswerItem, AppendAnswerTerminator, ParseAnswerTail, Verdict, AnswerKindHead, AnswerKindTerminator -->
<!-- source: pkg/plugin/rpc/answer_write.go -- WriteRecordAnswer, WriteDocumentAnswer -->

## Error Response

```
#2:42 error {"code":"peer-not-found","message":"no peer at 10.0.0.99"}
#2:43 error {"message":"unknown method"}
```

| Component | Description |
|-----------|-------------|
| `#<len>:<id>` | Echoed from request |
| `error` | Error verb |
| `<json>` | Optional JSON with `code` and/or `message` fields |

## Batch Event Delivery

Events are delivered in batches for efficiency using a pooled buffer.

```
#1:7 ze-plugin-callback:deliver-batch {"events":["event1-json","event2-json"]}
#1:7 ok
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
