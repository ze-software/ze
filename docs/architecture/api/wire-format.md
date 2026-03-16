# Ze IPC Wire Format

## Transport

Messages are UTF-8 JSON objects terminated by a NUL byte (0x00).
NUL cannot appear in valid JSON, making it an unambiguous frame delimiter.

```
{"method":"ze-bgp:peer-list","id":1}\x00{"method":"ze-bgp:subscribe","more":true,"id":2}\x00
```

### Framing

| Property | Value |
|----------|-------|
| Delimiter | NUL byte (0x00) |
| Encoding | UTF-8 JSON |
| Max message size | 16 MB (16,777,216 bytes) |
| Initial buffer | 64 KB |

Implementation: `internal/ipc/framing.go`

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

Implementation: `internal/ipc/method.go`

## Request

```json
{
  "method": "ze-bgp:peer-list",
  "params": {"selector": "10.0.0.1"},
  "id": 42
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `method` | string | yes | `module:rpc-name` |
| `params` | object | no | RPC input parameters |
| `id` | string or number | no | Correlation ID, echoed in response |
| `more` | boolean | no | Request streaming responses |

## Successful Response

```json
{
  "result": {"peers": [{"address": "10.0.0.1", "state": "established"}]},
  "id": 42
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `result` | any | yes | RPC output data |
| `id` | string or number | no | Echoed from request |
| `continues` | boolean | no | More responses follow (streaming) |

## Error Response

```json
{
  "error": "peer-not-found",
  "params": {"address": "10.0.0.99"},
  "id": 42
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `error` | string | yes | Kebab-case error identity |
| `params` | object | no | Error detail parameters |
| `id` | string or number | no | Echoed from request |

Error identities are normalized to kebab-case (spaces become hyphens, lowercased).

## Streaming Protocol

For event subscriptions and streaming responses:

1. Client sends request with `"more": true`
2. Server sends responses with `"continues": true` for each event
3. Server sends final response without `"continues"` when stream ends

```
Client → {"method":"ze-bgp:subscribe","params":{"events":["update"]},"id":1,"more":true}\x00
Server → {"result":{"event":"update","peer":"10.0.0.1"},"id":1,"continues":true}\x00
Server → {"result":{"event":"update","peer":"10.0.0.2"},"id":1,"continues":true}\x00
Server → {"result":{"event":"update","peer":"10.0.0.3"},"id":1}\x00
```

The absence of `"continues"` (or `"continues": false`) signals the final response.

## Response Mapping

The `MapResponse()` function converts existing plugin Response fields to IPC wire format:

| Plugin Field | IPC Mapping |
|-------------|-------------|
| `status = "done"` | `RPCResult` with `result` |
| `status = "error"` | `RPCError` with `error` |
| `serial` | `id` (raw JSON number) |
| `partial = true` | `continues: true` |
| `data` | `result` (JSON-marshaled) or `error` (normalized) |

Implementation: `internal/ipc/message.go`

## YANG API Modules

RPC definitions live in YANG API modules, separate from config modules:

| Module | File | Contains |
|--------|------|----------|
| ze-bgp-api | `internal/component/bgp/schema/ze-bgp-api.yang` | BGP RPCs + notifications |
| ze-system-api | `internal/ipc/schema/ze-system-api.yang` | System RPCs |
| ze-rib-api | `internal/component/plugin/rib/schema/ze-rib-api.yang` | RIB RPCs + notifications |
| ze-plugin-api | `internal/ipc/schema/ze-plugin-api.yang` | Plugin lifecycle RPCs |

Shared IPC types (typedefs, groupings) live in `ze-types` (`internal/yang/modules/ze-types.yang`).

## JSON Conventions

All JSON follows Ze conventions (see `rules/json-format.md`):

- Keys use kebab-case (`"peer-count"`, not `"peerCount"`)
- Error identities use kebab-case (`"peer-not-found"`)
- Address families use `"afi/safi"` format (`"ipv4/unicast"`)
- Numeric IDs passed as raw JSON numbers (not quoted)
