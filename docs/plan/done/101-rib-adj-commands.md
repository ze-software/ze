# Spec: RIB Adjacent Commands with Full Attributes

## Task

Add Adj-RIB inspection and manipulation commands to the RIB plugin with:
1. Peer selector filtering (*, IP, !IP, ip,ip,ip)
2. Full attribute storage and resend capability
3. Proper package organization for shared code

## Commands Implemented

| Command | Description |
|---------|-------------|
| `rib adjacent status` | Show RIB plugin status |
| `rib adjacent inbound show` | Show inbound routes (with peer selector) |
| `rib adjacent inbound empty` | Empty inbound RIB for peers |
| `rib adjacent outbound show` | Show outbound routes (with peer selector) |
| `rib adjacent outbound resend` | Resend stored routes to peers |

## Package Changes (79ab85e)

- ✅ `internal/selector/` - Extracted from plugin (reusable peer selectors)
- ✅ `internal/bgp/attribute/text.go` - Text formatting functions
- ✅ `internal/parse/` - DELETED (consolidated into attribute/text.go)
- ✅ `internal/plugin/rib/` - Full attribute storage and commands

## Files Modified

- `internal/selector/selector.go` - Selector type, Parse(), Matches()
- `internal/selector/selector_test.go` - Tests
- `internal/bgp/attribute/text.go` - FormatASPath, FormatCommunities, etc.
- `internal/bgp/attribute/text_test.go` - Tests
- `internal/plugin/rib/rib.go` - Commands and attribute storage
- `internal/plugin/rib/rib_test.go` - Tests
- `internal/plugin/rib/event.go` - Attribute fields for JSON

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests PASS

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] `docs/architecture/ROUTE_TYPES.md` created
