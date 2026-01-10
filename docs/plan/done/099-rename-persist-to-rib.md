# Spec: rename-persist-to-rib

## Task

Rename the `persist` plugin to `rib` and enhance it to provide full RIB functionality:
- Adj-RIB-In: Routes received FROM peers
- Adj-RIB-Out: Routes sent TO peers (existing functionality)

In-memory only. No disk persistence. No best-path selection (Loc-RIB).

## Required Reading

### Architecture Docs
- [x] `docs/architecture/api/ARCHITECTURE.md` - Plugin protocol, event format, RIB ownership
- [x] `docs/architecture/api/CAPABILITY_CONTRACT.md` - API responsibility for RIB
- [x] `docs/architecture/rib-transition.md` - Engine vs API RIB ownership

### RFC Summaries (MUST for protocol work)
- N/A - This is plugin refactoring, not BGP protocol work

**Key insights:**
- API programs own ALL RIB data and logic (engine is minimal speaker)
- Event format uses array for prefixes: `{"nexthop": ["prefix1", "prefix2"]}`
- Both sent and received events use same format (only type field differs)
- Sent events: `type: "sent"`, Received events: `message.type: "update"`

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestParseEvent_SentFormat` | `pkg/plugin/rib/rib_test.go` | Sent events parsed correctly |
| `TestParseEvent_ReceivedFormat` | `pkg/plugin/rib/rib_test.go` | Received events with message wrapper parsed |
| `TestParseEvent_StateFormat` | `pkg/plugin/rib/rib_test.go` | State events parsed correctly |
| `TestParseEvent_RequestFormat` | `pkg/plugin/rib/rib_test.go` | CLI command requests parsed |
| `TestHandleSent_StoresRoutes` | `pkg/plugin/rib/rib_test.go` | Routes stored in Adj-RIB-Out |
| `TestHandleSent_Withdraw` | `pkg/plugin/rib/rib_test.go` | Withdrawals remove from Adj-RIB-Out |
| `TestHandleReceived_StoresRoutes` | `pkg/plugin/rib/rib_test.go` | Routes stored in Adj-RIB-In |
| `TestHandleReceived_Withdraw` | `pkg/plugin/rib/rib_test.go` | Withdrawals remove from Adj-RIB-In |
| `TestHandleState_PeerUp` | `pkg/plugin/rib/rib_test.go` | Replay on peer up |
| `TestHandleState_PeerDown` | `pkg/plugin/rib/rib_test.go` | Clear Adj-RIB-In on peer down |
| `TestStatusJSON` | `pkg/plugin/rib/rib_test.go` | Status shows both RIB counts |
| `TestRoutesJSON` | `pkg/plugin/rib/rib_test.go` | Routes output includes both RIBs |
| `TestDispatch_RoutesToCorrectHandler` | `pkg/plugin/rib/rib_test.go` | Events routed to correct handler |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| N/A | - | No new functional tests (plugin not exercised by encoding tests) |

## Files to Modify

### Renamed
- `pkg/plugin/persist/` → `pkg/plugin/rib/`
- `pkg/plugin/persist/persist.go` → `pkg/plugin/rib/rib.go`
- `cmd/zebgp/plugin_persist.go` → `cmd/zebgp/plugin_rib.go`

### Updated
- `pkg/plugin/rib/rib.go` - Package rename, type rename, add Adj-RIB-In, slog logging
- `pkg/plugin/rib/event.go` - Package rename, unified event parsing for both formats
- `cmd/zebgp/plugin_rib.go` - Import path, function name
- `cmd/zebgp/plugin.go` - Dispatch case, help text
- `pkg/source/registry_test.go` - Test API name
- `pkg/reactor/reactor.go` - Comment update
- `pkg/plugin/text.go` - Comment update
- `docs/architecture/api/ARCHITECTURE.md` - Persist → RIB
- `docs/architecture/api/CAPABILITY_CONTRACT.md` - Persist → RIB
- `docs/architecture/rib-transition.md` - Persist → RIB
- `.claude/rules/go-standards.md` - Added slog logging requirement

## Implementation Steps

1. **Write tests** - Created `pkg/plugin/rib/rib_test.go` with 13 tests
2. **Run tests** - Verified FAIL (package didn't exist)
3. **Implement Phase 1** - Rename persist → rib (git mv, package rename)
4. **Implement Phase 2** - Add Adj-RIB-In support (handleReceived)
5. **Implement Phase 3** - Enhanced commands (rib routes in/out)
6. **Critical Review** - Found format mismatch bug (expected nested map, actual is array)
7. **Fix Bug** - Updated handleReceived to use array format
8. **Add Logging** - Added slog logging for silent ignores
9. **Run tests** - Verified PASS
10. **Verify all** - `make lint && make test && make functional`

## Known Limitations

### ADD-PATH Not Supported
Route key uses `family:prefix` only. With ADD-PATH (RFC 7911), multiple paths to same prefix will overwrite. Full support requires plugin event format changes.

```go
// Current key (no path-id)
func routeKey(family, prefix string) string {
    return family + ":" + prefix
}
```

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (initial)
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (18 passed)

### Documentation
- [x] Required docs read
- [x] RFC summaries read (N/A - not protocol work)
- [x] RFC references added to code (N/A)
- [x] RFC constraint comments added (N/A)
- [x] `docs/` updated (ARCHITECTURE.md, CAPABILITY_CONTRACT.md, rib-transition.md)

### Completion
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
