# Spec: API Command Restructure - Step 8: BGP Cache Commands

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/ipc_protocol.md` - BGP cache commands
4. `internal/plugin/msgid.go` - current handlers (to be deleted)
5. `internal/plugin/forward.go` - current handlers (to be deleted)

## Task

Migrate cache commands to BGP namespace with `bgp cache` prefix.

**Command Migration:**

| Old | New |
|-----|-----|
| `msg-id retain <id>` | `bgp cache <id> retain` |
| `msg-id release <id>` | `bgp cache <id> release` |
| `msg-id expire <id>` | `bgp cache <id> expire` |
| `msg-id list` | `bgp cache list` |
| `bgp peer <sel> forward update-id <id>` | `bgp cache <id> forward <sel>` |
| `bgp delete update-id <id>` | removed (use `bgp cache <id> expire`) |

**Why BGP-centric:**
- Cache stores BGP UPDATE messages
- Forward operation targets BGP peers
- Belongs in BGP subsystem namespace

## Required Reading

### Architecture Docs
- [x] `docs/architecture/api/ipc_protocol.md` - BGP cache section

### Source Files
- [x] `internal/plugin/msgid.go` - current msg-id handlers
- [x] `internal/plugin/forward.go` - current forward handlers

## 🧪 TDD Test Plan

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| cache id | 1-uint64 max | 18446744073709551615 | 0 | N/A (uint64) |
| cache id format | numeric | `12345` | `abc`, `-1`, empty | overflow string |

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBgpCacheRetain` | `internal/plugin/cache_test.go` | `bgp cache <id> retain` | |
| `TestBgpCacheRelease` | `internal/plugin/cache_test.go` | `bgp cache <id> release` | |
| `TestBgpCacheExpire` | `internal/plugin/cache_test.go` | `bgp cache <id> expire` | |
| `TestBgpCacheList` | `internal/plugin/cache_test.go` | `bgp cache list` | |
| `TestBgpCacheForward` | `internal/plugin/cache_test.go` | `bgp cache <id> forward <sel>` | |
| `TestBgpCacheInvalidId` | `internal/plugin/cache_test.go` | Invalid ID rejected | |
| `TestOldMsgIdRemoved` | `internal/plugin/handler_test.go` | `msg-id *` returns unknown | |
| `TestOldForwardRemoved` | `internal/plugin/handler_test.go` | `bgp peer forward update-id` returns unknown | |

## Files to Delete

| File | Reason |
|------|--------|
| `internal/plugin/msgid.go` | Replaced by cache.go |
| `internal/plugin/forward.go` | Replaced by cache.go |
| `internal/plugin/msgid_test.go` | Replaced by cache_test.go |
| `internal/plugin/forward_test.go` | Replaced by cache_test.go |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/plugin/cache.go` | BGP cache command handlers |
| `internal/plugin/cache_test.go` | Tests for cache handlers |

## Files to Modify

| File | Changes |
|------|---------|
| `internal/plugin/handler.go` | Remove RegisterMsgIDHandlers/RegisterForwardHandlers, add RegisterCacheHandlers |
| `internal/plugin/bgp.go` | Add "cache" to bgp subcommands |
| `internal/plugin/handler_test.go` | Update tests for removed commands |

## Implementation Steps

1. **Write tests** - Create cache_test.go with new command tests
2. **Run tests** - Verify FAIL
3. **Create cache.go** - Implement bgp cache handlers
4. **Update handler.go** - Register cache handlers, remove old registrations
5. **Update bgp.go** - Add cache to subcommands
6. **Delete old files** - Remove msgid.go, forward.go and their tests
7. **Update handler_test.go** - Change tests to verify old commands removed
8. **Run tests** - Verify PASS
9. **Verify all** - `make lint && make test && make functional`

## Command Parsing

The `bgp cache` command has special parsing:
- `bgp cache <id> retain` - id then action
- `bgp cache <id> forward <sel>` - id then action then selector
- `bgp cache list` - no id

Register as `bgp cache` and parse subcommand in handler.

## Implementation Summary

### What Was Implemented
- Created `internal/plugin/cache.go` with `bgp cache` handler
- Commands: `bgp cache <id> retain/release/expire`, `bgp cache <id> forward <sel>`, `bgp cache list`
- Deleted `msgid.go`, `forward.go` and their tests
- Updated `handler.go` to use RegisterCacheHandlers
- Updated `handler_test.go` to verify old commands removed, new command registered

### Files Created
- `internal/plugin/cache.go`
- `internal/plugin/cache_test.go`

### Files Deleted
- `internal/plugin/msgid.go`
- `internal/plugin/msgid_test.go`
- `internal/plugin/forward.go`
- `internal/plugin/forward_test.go`

### Files Modified
- `internal/plugin/handler.go` - replaced RegisterMsgIDHandlers/RegisterForwardHandlers with RegisterCacheHandlers
- `internal/plugin/handler_test.go` - tests for migration

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL initially
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes

### Completion
- [ ] All files committed together
- [ ] Spec moved to `docs/plan/done/`
