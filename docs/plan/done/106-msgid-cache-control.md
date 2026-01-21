# Spec: msgid-cache-control

## Task
Implement msg-id cache control commands for API programs to manage UPDATE cache lifetime.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - msg-id cache architecture, reactor design
- [ ] `docs/architecture/api/architecture.md` - API command patterns
- [ ] `docs/architecture/api/capability-contract.md` - documented msg-id commands spec

### RFC Summaries (MUST for protocol work)
N/A - internal API feature, no BGP protocol changes.

**Key insights:**
- Engine passes wire bytes to plugins, plugins control RIB
- msg-id cache enables zero-copy forwarding via `forward update-id`
- API needs cache control for graceful restart (retain routes for replay)
- Existing pattern: handlers in separate files, registered via `RegisterXxxHandlers(d)`

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestRecentUpdateCacheRetain` | `internal/reactor/recent_cache_test.go` | Retained entries survive lazy cleanup |
| `TestRecentUpdateCacheRetainNotFound` | `internal/reactor/recent_cache_test.go` | Retain returns false for missing entry |
| `TestRecentUpdateCacheRelease` | `internal/reactor/recent_cache_test.go` | Release clears flag and resets TTL |
| `TestRecentUpdateCacheReleaseNotFound` | `internal/reactor/recent_cache_test.go` | Release returns false for missing entry |
| `TestRecentUpdateCacheList` | `internal/reactor/recent_cache_test.go` | List returns all valid IDs |
| `TestRecentUpdateCacheListExcludesExpired` | `internal/reactor/recent_cache_test.go` | List excludes expired entries |
| `TestRecentUpdateCacheRetainedSurvivesExpiry` | `internal/reactor/recent_cache_test.go` | Retained entries in List after TTL |
| `TestRecentUpdateCacheRetainIdempotent` | `internal/reactor/recent_cache_test.go` | Double retain is safe |
| `TestRecentUpdateCacheReleaseNonRetained` | `internal/reactor/recent_cache_test.go` | Release on non-retained resets TTL |
| `TestRecentUpdateCacheTakeRetained` | `internal/reactor/recent_cache_test.go` | Take works on retained entries |
| `TestRecentUpdateCacheReleaseAfterTake` | `internal/reactor/recent_cache_test.go` | Release fails after Take |
| `TestMsgIDRetain` | `internal/plugin/msgid_test.go` | Handler parses and calls RetainUpdate |
| `TestMsgIDRelease` | `internal/plugin/msgid_test.go` | Handler parses and calls ReleaseUpdate |
| `TestMsgIDExpire` | `internal/plugin/msgid_test.go` | Handler parses and calls DeleteUpdate |
| `TestMsgIDList` | `internal/plugin/msgid_test.go` | Handler calls ListUpdates |
| `TestMsgIDRetainError` | `internal/plugin/msgid_test.go` | Error propagation on expired ID |
| `TestMsgIDUsageMessages` | `internal/plugin/msgid_test.go` | Error messages show correct syntax |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| N/A | - | Unit tests provide full coverage |

## Files to Modify
- `internal/reactor/recent_cache.go` - Add `retained` flag, `Retain()`, `Release()`, `List()`
- `internal/reactor/recent_cache_test.go` - Add 11 cache tests
- `internal/plugin/types.go` - Add `RetainUpdate`, `ReleaseUpdate`, `ListUpdates` to interface
- `internal/reactor/reactor.go` - Implement adapter methods on `reactorAPIAdapter`
- `internal/plugin/msgid.go` - New file: handler implementations
- `internal/plugin/msgid_test.go` - New file: handler tests
- `internal/plugin/handler.go` - Register `RegisterMsgIDHandlers(d)`
- `internal/plugin/forward_test.go` - Update mock with new interface methods
- `internal/plugin/handler_test.go` - Update mock with new interface methods
- `internal/plugin/update_text_test.go` - Update mock with new interface methods
- `docs/architecture/api/capability-contract.md` - Update status table

## Implementation Steps
1. **Write tests** - Create cache and handler tests
2. **Run tests** - Verify FAIL (paste output)
3. **Implement cache** - Add retained flag and methods to `recent_cache.go`
4. **Add interface** - Add methods to `ReactorInterface` in `types.go`
5. **Implement adapter** - Add methods to `reactorAPIAdapter` in `reactor.go`
6. **Update mocks** - Add stub methods to all mock reactors
7. **Implement handlers** - Create `msgid.go` with handlers
8. **Register handlers** - Add `RegisterMsgIDHandlers(d)` to `handler.go`
9. **Run tests** - Verify PASS (paste output)
10. **Verify all** - `make lint && make test && make functional` (paste output)

## RFC Documentation

### Reference Comments
N/A - internal API feature, no BGP protocol code.

### Constraint Comments
N/A - no RFC constraints apply.

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)

### Verification
- [ ] `make lint` passes (26 linters including `govet`, `staticcheck`, `gosec`, `gocritic`)
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] RFC summaries read (N/A - internal feature)
- [ ] RFC references added to code (N/A)
- [ ] RFC constraint comments added (N/A)
- [ ] `docs/` updated (capability-contract.md)

### Completion
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`

---

## Execution Log

### Tests FAIL
```
internal/reactor/recent_cache_test.go:321:12: cache.Retain undefined
internal/reactor/recent_cache_test.go:349:11: cache.Retain undefined
internal/plugin/msgid_test.go:158: unexpected error: unknown command
```

### Tests PASS
```
=== RUN   TestRecentUpdateCacheRetain
--- PASS: TestRecentUpdateCacheRetain (0.02s)
...
=== RUN   TestMsgIDRetain
--- PASS: TestMsgIDRetain (0.00s)
PASS
ok  	codeberg.org/thomas-mangin/ze/internal/reactor	1.667s
ok  	codeberg.org/thomas-mangin/ze/internal/plugin	1.448s
```

### Final Verification
```
make lint       → 0 issues
make test       → all pass
make functional → all pass
```
