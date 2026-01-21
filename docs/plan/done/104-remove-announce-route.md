# Spec: Remove "announce route" Syntax

## Task

Complete migration from `announce route` to `update text` syntax per DOCUMENTATION-ISSUES.md issue #2.

## Required Reading

### Architecture Docs
- [x] `docs/architecture/api/update-syntax.md` - New syntax reference
- [x] `docs/architecture/core-design.md` - Plugin output format

### RFC Summaries
- [x] `docs/rfc/rfc4271.md` - BGP UPDATE message format

**Key insights:**
- RIB/RR plugins output commands back to ZeBGP engine
- Output format must match registered command handlers
- `update text` is the canonical syntax for route announcements

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestHandleState_PeerUp` | `internal/plugin/rib/rib_test.go` | RIB outputs `update text` format |
| `TestReplayRoutesWithPathID` | `internal/plugin/rib/rib_test.go` | Path-ID uses `path-information set` |
| `TestServer_HandleStateDown` | `internal/plugin/rr/server_test.go` | RR outputs `update text nlri del` |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| encoding tests | `test/data/plugin/*.ci` | Route announcement/withdrawal |

## Files to Modify

- `internal/plugin/route.go` - Removed `handleAnnounceRoute`, kept `announceRouteImpl` (internal)
- `internal/plugin/commit.go` - Removed `commit announce route` handler
- `internal/plugin/rib/rib.go` - Changed output to `update text` syntax
- `internal/plugin/rr/server.go` - Changed output to `update text nlri del` syntax
- `internal/plugin/rib/rib_test.go` - Updated expected output strings
- `internal/plugin/rr/server_test.go` - Updated expected output strings
- `internal/plugin/commit_test.go` - Removed `commit announce` tests
- `internal/plugin/update_text_test.go` - Fixed Wire field usage
- `internal/reactor/reactor.go` - Added nil check for Origin extraction

## Implementation Steps

1. **Delete dead code** - Remove `handleAnnounceRoute` from route.go (paste output)
2. **Update commit.go** - Remove `announce` action from commit handler
3. **Update rib.go** - Change output format to `update text` syntax
4. **Update rr/server.go** - Change withdraw format to `update text nlri del`
5. **Update tests** - Fix expected strings in rib_test.go, server_test.go
6. **Verify all** - `make lint && make test && make functional`

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (output below)
- [x] Implementation complete
- [x] Tests PASS (output below)

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (37/37 encoding tests)

### Documentation
- [x] Required docs read
- [ ] `docs/` updated if schema changed

### Completion
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`

## Test Output

```
$ make test
ok  codeberg.org/thomas-mangin/ze/internal/plugin
ok  codeberg.org/thomas-mangin/ze/internal/plugin/rib
ok  codeberg.org/thomas-mangin/ze/internal/plugin/rr
ok  codeberg.org/thomas-mangin/ze/internal/reactor

$ make lint
0 issues.

$ go run ./test/cmd/functional encoding --all
passed 37
Total: 37 test(s) run, 100.0% passed
```

## Summary of Changes

| Change | Details |
|--------|---------|
| Removed `handleAnnounceRoute` | Dead code, was not registered |
| Kept `announceRouteImpl` | Used internally by `announce ipv4/ipv6` handlers |
| Removed `commit announce route` | Users should use `update text` directly |
| RIB output | `peer X update text nhop set Y nlri Z add P` |
| RR withdraw | `peer !X update text nlri Z del P` |
