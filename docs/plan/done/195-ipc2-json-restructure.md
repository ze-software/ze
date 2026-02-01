# Spec: IPC Protocol JSON Restructure

## Task

Restructure JSON output format so that:
1. `peer` is at the `bgp` level (not nested inside event type)
2. State events use simple `"state": "up"` string (not a container)
3. Other events (update, notification, open, etc.) have data nested under `bgp.<event-type>`

## Implementation Summary

### What Was Implemented

**JSON Structure Changes:**

Before (incorrect):
```json
{"type":"bgp","bgp":{"type":"update","update":{"peer":{...},...}}}
{"type":"bgp","bgp":{"type":"state","state":{"peer":{...},"state":"up"}}}
```

After (correct):
```json
{"type":"bgp","bgp":{"type":"update","peer":{...},"update":{...}}}
{"type":"bgp","bgp":{"type":"state","peer":{...},"state":"up"}}
```

**Files Modified:**

| File | Changes |
|------|---------|
| `internal/plugin/json.go` | Updated `message()` to put peer at bgp level |
| `internal/plugin/json.go` | State functions use simple string value |
| `internal/plugin/text.go` | Updated `formatFilterResultJSON` and `formatStateChangeJSON` |
| `internal/plugin/json_test.go` | Updated test helpers and expectations |
| `internal/plugin/text_test.go` | Updated expected JSON format |
| `internal/plugin/rib/event.go` | Updated `parseEvent` to preserve peer from bgp level |
| `internal/exabgp/bridge.go` | Updated `ZebgpToExabgpJSON` for IPC Protocol format |
| `internal/exabgp/bridge_test.go` | Updated tests for IPC Protocol format |
| `test/plugin/check.ci` | Updated Python script for new format |
| `test/plugin/refresh.ci` | Updated Python script for state events |
| `docs/architecture/api/json-format.md` | Updated all JSON examples |
| `docs/architecture/api/ipc_protocol.md` | Updated format documentation |

### Key Design Decisions

1. **Peer at bgp level:** Consistent location for peer info across all event types
2. **State as simple string:** `"state": "up"` is more concise than `"state": {"state": "up"}`
3. **ExaBGP bridge updated:** Handles new IPC Protocol wrapper format

### Verification

```
make lint: 0 issues
make test: all pass
```

## Checklist

### TDD
- [x] Tests written
- [x] Tests FAIL (before implementation)
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes
- [x] `make test` passes

### Documentation
- [x] Architecture docs updated (json-format.md, ipc_protocol.md)
