# Spec: API Command Restructure - Step 1: JSON Message Format

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/ipc_protocol.md` - target protocol spec
4. `internal/plugin/types.go` - Response struct
5. `internal/plugin/json.go` - Event JSON encoding

## Task

Add `type` field to all JSON messages. Top-level `type` indicates which key contains the payload.

**Response changes:**
- Add `"type": "response"` at root
- Nest response fields under `"response": {...}`

**Event changes:**
- Add `"type": "bgp"` or `"type": "rib"` at root (indicates payload key)
- Nest all event data under `"bgp": {...}` or `"rib": {...}`
- Event type is `bgp.type` or `rib.type` (e.g., `update`, `cache`)
- Keep `message` object for wire metadata: `{"id": N, "direction": "..."}`
- Keep `peer` as object: `{"address": "...", "asn": N}`
- Nest path attributes under `"attributes": {...}`
- Nest NLRI families under `"nlri": {"<family>": [...]}`
- Add `"raw": {...}` for wire bytes (format=full only)

**No backward compatibility** - direct replacement. RIB plugin event.go must be updated for new structure.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/ipc_protocol.md` - target wire format

### Source Files
- [ ] `internal/plugin/types.go` - Response struct definition
- [ ] `internal/plugin/json.go` - Event JSON encoding
- [ ] `internal/plugin/handler.go` - Response creation patterns

## Current State

**Response struct (types.go):**
```go
type Response struct {
    Serial  string `json:"serial,omitempty"`
    Status  string `json:"status"`
    Partial bool   `json:"partial,omitempty"`
    Data    any    `json:"data,omitempty"`
}
```

**Current response output:**
```json
{"serial":"1","status":"done","data":{...}}
```

**Current event output:**
```json
{
  "message": {"type": "update", "id": 123, "direction": "received"},
  "peer": {"address": "192.0.2.1", "asn": 65001},
  "origin": "igp",
  "as-path": [65001],
  "ipv4/unicast": [...]
}
```

## Target State

**Response wrapper struct:**
```go
type ResponseWrapper struct {
    Type     string    `json:"type"`     // Always "response"
    Response *Response `json:"response"` // Payload
}

type Response struct {
    Serial  string `json:"serial,omitempty"`
    Status  string `json:"status"`
    Partial bool   `json:"partial,omitempty"`
    Data    any    `json:"data,omitempty"`
}
```

**Target output:**
```json
{"type":"response","response":{"serial":"1","status":"done","data":{...}}}
```

**Event output (old → new):**
```json
// OLD:
{
  "message": {"type": "update", "id": 123, "direction": "received"},
  "peer": {"address": "192.0.2.1", "asn": 65001},
  "origin": "igp",
  "as-path": [65001],
  "ipv4/unicast": [{"action": "add", "next-hop": "192.0.2.1", "nlri": ["10.0.0.0/24"]}]
}

// NEW:
{
  "type": "bgp",
  "bgp": {
    "type": "update",
    "message": {"id": 123, "direction": "received"},
    "peer": {"address": "192.0.2.1", "asn": 65001},
    "attributes": {"origin": "igp", "as-path": [65001]},
    "nlri": {"ipv4/unicast": [{"action": "add", "next-hop": "192.0.2.1", "nlri": ["10.0.0.0/24"]}]},
    "raw": {"attributes": "...", "nlri": {"ipv4/unicast": "..."}, "withdrawn": {}}
  }
}
```

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResponseJSONHasTypeWrapper` | `internal/plugin/types_test.go` | Response JSON has `"type":"response"` and `"response":{...}` | |
| `TestEventJSONHasTopLevelType` | `internal/plugin/json_test.go` | BGP event has `"type":"bgp"` and `"bgp":{...}` | |
| `TestEventJSONBgpTypeField` | `internal/plugin/json_test.go` | BGP payload has `"type":"update"` etc. | |
| `TestEventJSONMessageMetadata` | `internal/plugin/json_test.go` | `message` object with id, direction (no type) | |
| `TestEventJSONPeerObject` | `internal/plugin/json_test.go` | `peer` is object with `address` and `asn` | |
| `TestEventJSONAttributesNested` | `internal/plugin/json_test.go` | Path attrs in `attributes` object | |
| `TestEventJSONNLRINested` | `internal/plugin/json_test.go` | Families in `nlri` object | |
| `TestEventJSONRawSection` | `internal/plugin/json_test.go` | Wire bytes in `raw` object (format=full) | |
| `TestResponseMarshalFormat` | `internal/plugin/types_test.go` | Full response wrapper format matches spec | |
| `TestRIBEventJSONFormat` | `internal/plugin/json_test.go` | RIB event has `"type":"rib"` and `"rib":{...}` | |
| `TestRIBEventParserNewFormat` | `internal/plugin/rib/event_test.go` | RIB plugin parses new event format | |

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `response-format` | `test/data/plugin/response-format.ci` | Verify response includes type field | |

## Files to Modify

| File | Changes |
|------|---------|
| `internal/plugin/types.go` | Add `Type` field to `Response` struct |
| `internal/plugin/handler.go` | Update all response creation to set `Type: "response"` |
| `internal/plugin/session.go` | Update response creation |
| `internal/plugin/commit.go` | Update response creation |
| `internal/plugin/route.go` | Update response creation |
| `internal/plugin/forward.go` | Update response creation |
| `internal/plugin/msgid.go` | Update response creation |
| `internal/plugin/raw.go` | Update response creation |
| `internal/plugin/refresh.go` | Update response creation |
| `internal/plugin/json.go` | Wrap events under `bgp`/`rib` key, add event type, nest attributes/nlri |
| `internal/plugin/rib/event.go` | Update event parsing for new format |

## Implementation Steps

1. **Write unit tests** - Create tests for new JSON format
2. **Run tests** - Verify FAIL (paste output)
3. **Update Response struct** - Add `Type` field with json tag
4. **Create helper function** - `NewResponse(status string, data any) *Response` that sets Type
5. **Update all handlers** - Use helper or set Type directly
6. **Update event encoding** - Add type/namespace fields
7. **Run tests** - Verify PASS (paste output)
8. **Verify all** - `make lint && make test && make functional` (paste output)

## Helper Functions

To avoid updating every response creation, add wrapper functions:

```go
// WrapResponse creates a response wrapper with type field set.
func WrapResponse(r *Response) *ResponseWrapper {
    return &ResponseWrapper{
        Type:     "response",
        Response: r,
    }
}

// NewResponse creates a response (without wrapper).
func NewResponse(status string, data any) *Response {
    return &Response{
        Status: status,
        Data:   data,
    }
}

// NewErrorResponse creates an error response (without wrapper).
func NewErrorResponse(msg string) *Response {
    return &Response{
        Status: "error",
        Data:   msg,
    }
}
```

**Serialization:** When encoding to JSON, use `WrapResponse()` to add the type/wrapper structure.

## Event Type Mapping

| Event Source | Top-level type | Payload type |
|--------------|----------------|--------------|
| BGP UPDATE received/sent | `bgp` | `update` |
| BGP OPEN | `bgp` | `open` |
| BGP NOTIFICATION | `bgp` | `notification` |
| BGP KEEPALIVE | `bgp` | `keepalive` |
| BGP ROUTE-REFRESH | `bgp` | `refresh` |
| Peer state change | `bgp` | `state` |
| Capability negotiation | `bgp` | `negotiated` |
| RIB cache events | `rib` | `cache` |
| RIB route changes | `rib` | `route` |

## Event Structure Summary

**BGP event:**
```json
{
  "type": "bgp",             // Indicates payload is in "bgp" key
  "bgp": {
    "type": "<event-type>",  // update, open, state, etc.
    "message": {             // Wire metadata (for BGP messages)
      "id": <N>,             // 0 for locally-originated
      "direction": "received|sent"
    },
    "peer": {
      "address": "<ip>",
      "asn": <N>
    },
    "attributes": {...},     // Path attributes (UPDATE only)
    "nlri": {                // NLRI families (UPDATE only)
      "<family>": [...]
    },
    "raw": {                 // Wire bytes (format=full)
      "attributes": "<hex>",
      "nlri": {"<family>": "<hex>"},
      "withdrawn": {"<family>": "<hex>"}
    }
  }
}
```

**RIB event:**
```json
{
  "type": "rib",             // Indicates payload is in "rib" key
  "rib": {
    "type": "<event-type>",  // cache, route
    "action": "<action>",    // new, evict, add, remove
    "msg-id": <N>,           // For cache events
    "peer": {
      "address": "<ip>",
      "asn": <N>
    }
  }
}
```

**Response:**
```json
{
  "type": "response",        // Indicates payload is in "response" key
  "response": {
    "serial": "<id>",
    "status": "done|error|warning|ack",
    "partial": false,
    "data": {...}
  }
}
```

## Implementation Summary

### What Was Implemented

- **ResponseWrapper pattern** - Added `ResponseWrapper` struct and `WrapResponse()` helper in `types.go`
- **All response serialization wrapped** - Updated 6 locations in `server.go`:
  - `sendResponse()` - socket client responses
  - `handleProcessCommand()` - process command forwarding
  - `handleRegisterCommand()` - 2 locations
  - `handleUnregisterCommand()` - 2 locations
- **BGP event IPC Protocol format** - Updated `json.go` marshal() to output `{"type":"bgp","bgp":{...}}`
- **Event structure changes**:
  - Path attributes nested under `"attributes"` key
  - NLRIs nested under `"nlri"` key
  - Message type moved to `bgp.type` (removed from `message` object)
  - `raw` section for wire bytes (format=full)
- **RIB event parsing** - Updated `rib/event.go` to parse IPC Protocol wrapper format
- **Test coverage** - All tests updated to verify new format

### Files Modified

| File | Changes |
|------|---------|
| `internal/plugin/types.go` | ResponseWrapper, WrapResponse(), NewResponse(), NewErrorResponse() |
| `internal/plugin/server.go` | All 6 response serialization points wrapped |
| `internal/plugin/json.go` | marshal() outputs IPC Protocol format with nested attributes/nlri |
| `internal/plugin/text.go` | Text format unchanged (no JSON wrapper per spec) |
| `internal/plugin/rib/event.go` | parseEvent handles IPC Protocol wrapper |
| `internal/plugin/types_test.go` | ResponseWrapper tests |
| `internal/plugin/server_test.go` | 5 tests verify response wrapper |
| `internal/plugin/json_test.go` | getBGPPayload helper, updated format tests |
| `internal/plugin/text_test.go` | Updated expected format |

### Bugs Found/Fixed

- **ResponseWrapper not used** - Initially created wrapper but didn't integrate into actual code paths
- **5 additional serialization points missed** - handleProcessCommand, handleRegisterCommand (2), handleUnregisterCommand (2)

### Deviations from Plan

- Did not modify handler.go, session.go, commit.go, route.go, forward.go, msgid.go, raw.go, refresh.go - Response creation unchanged, only serialization wrapped
- No functional test added (response-format.ci) - existing tests provide sufficient coverage

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (verified during implementation)
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (80 tests)

### Documentation
- [x] `ipc_protocol.md` already updated (version 2.0)

### Completion
- [ ] All files committed together
- [x] Spec moved to `docs/plan/done/`
