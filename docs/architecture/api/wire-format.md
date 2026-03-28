# Ze IPC Wire Format

## Transport

Messages are UTF-8 lines terminated by a newline byte (0x0A).
Compact JSON never contains unescaped newlines, making newline an unambiguous frame delimiter.

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

- `#<id>` is a decimal integer (monotonically increasing per connection).
- `<verb>` is a method name (requests) or `ok`/`error` (responses).
- `<json-payload>` is optional compact JSON.

Implementation: `pkg/plugin/rpc/framing.go`, `pkg/plugin/rpc/message.go`
<!-- source: pkg/plugin/rpc/framing.go -- newline-delimited framing -->
<!-- source: pkg/plugin/rpc/conn.go -- Conn -->

## Method Naming

Methods use `module:rpc-name` format. The module name comes from the YANG module that
defines the RPC. The RPC name uses kebab-case.

| Wire Method | YANG Module |
|-------------|-------------|
| `ze-bgp:peer-list` | ze-bgp-api |
| `ze-bgp:subscribe` | ze-bgp-api |
| `ze-system:daemon-status` | ze-system-api |
| `ze-system:version-software` | ze-system-api |
| `ze-system:command-list` | ze-system-api |
| `ze-rib:show-in` | ze-rib-api |
| `ze-plugin-api:session-ready` | ze-plugin-api |

Max method name length: 256 characters.
<!-- source: internal/component/config/yang/rpc.go -- WireModule -->

## Request

```
#42 ze-bgp:peer-list {"selector":"10.0.0.1"}
#43 ze-plugin-engine:declare-registration {"families":[{"name":"ipv4/unicast","mode":"both"}]}
#44 ze-bgp:subscribe {"events":["update"]}
```

| Component | Description |
|-----------|-------------|
| `#<id>` | Correlation ID (decimal integer) |
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
| ze-bgp-api | `internal/component/bgp/schema/ze-bgp-api.yang` | BGP RPCs + notifications |
| ze-system-api | `internal/ipc/schema/ze-system-api.yang` | System RPCs |
| ze-rib-api | `internal/component/bgp/plugins/rib/schema/ze-rib-api.yang` | RIB RPCs + notifications |
| ze-plugin-api | `internal/ipc/schema/ze-plugin-api.yang` | Plugin lifecycle RPCs |

Shared IPC types (typedefs, groupings) live in `ze-types` (`internal/component/config/yang/modules/ze-types.yang`).
<!-- source: internal/component/bgp/schema/ze-bgp-api.yang -- BGP RPCs -->
<!-- source: internal/core/ipc/schema/ze-system-api.yang -- system RPCs -->
<!-- source: internal/core/ipc/schema/ze-plugin-engine.yang -- plugin-engine RPCs -->

## JSON Conventions

All JSON follows Ze conventions (see `rules/json-format.md`):

- Keys use kebab-case (`"peer-count"`, not `"peerCount"`).
- Error identities use kebab-case (`"peer-not-found"`).
- Address families use `"afi/safi"` format (`"ipv4/unicast"`).
<!-- source: pkg/plugin/rpc/types.go -- DeclareRegistrationInput and other wire types -->
