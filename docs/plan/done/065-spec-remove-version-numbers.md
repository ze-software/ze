# Spec: Remove Version Numbers from Code

## Task

Remove ALL version number references from the codebase. No v6, v7, v2, v3, "legacy", "current" - nothing.

- API: Delete version constants and fields. ONE output format only.
- Migration: Semantic transform names only. No version terminology.

## Files Modified

### API Version Removal

| File | Action | Status |
|------|--------|--------|
| `internal/plugin/types.go` | Delete `APIVersionLegacy`, `APIVersionNLRI` constants | ✅ Done |
| `internal/plugin/types.go` | Delete `Version int` from `ContentConfig` | ✅ Done |
| `internal/plugin/text.go` | Remove all version comparison branches | ✅ Done |
| `internal/plugin/text_test.go` | Remove version test cases | ✅ Done |
| `internal/plugin/json_test.go` | Remove version test cases | ✅ Done |
| `internal/plugin/handler.go` | Remove `"api": "v6"` from response | ✅ Done |
| `internal/reactor/peersettings.go` | Delete `Version int` field | ✅ Done |
| `internal/config/bgp.go` | Delete `Version int` field | ✅ Done |
| `internal/reactor/reactor.go` | Remove version default assignment | ✅ Done |
| `cmd/ze/bgp/run_test.go` | Remove `"api": "v6"` | ✅ Done |

### Migration Terminology Removal

| File | Action | Status |
|------|--------|--------|
| `internal/config/migration/detect_test.go` | Remove v2/v3 from comments | ✅ Done |
| `internal/config/migration/migrate_test.go` | Remove v2/v3 from comments | ✅ Done |
| `internal/config/migration/transformations_test.go` | Remove v2/v3 from comments | ✅ Done |
| `internal/config/migration/api_test.go` | Remove v2/v3 from comments | ✅ Done |
| `internal/config/bgp.go` | Remove v2/v3 from comments | ✅ Done |
| `internal/config/bgp_test.go` | Remove v2/v3 from comments | ✅ Done |
| `internal/config/loader.go` | Remove v2/v3 from comments | ✅ Done |
| `internal/config/loader_test.go` | Remove v2/v3 from comments | ✅ Done |
| `internal/config/parser.go` | Remove v2/v3 from comments | ✅ Done |
| `internal/config/editor/completer_test.go` | Remove v3 from comments | ✅ Done |
| `cmd/ze/bgp/config_check.go` | Remove v2/v3 from comments | ✅ Done |
| `cmd/ze/bgp/config_fmt.go` | Rename `ErrV2Config` → `ErrOldConfig` | ✅ Done |
| `cmd/ze/bgp/config_fmt_test.go` | Remove v2/v3 from comments | ✅ Done |
| `cmd/ze/bgp/config_test.go` | Remove v2/v3 from comments | ✅ Done |
| `docs/architecture/config/SYNTAX.md` | Remove v2/v3 | ✅ Done |

## Remaining References (Legitimate)

| File | Reference | Reason |
|------|-----------|--------|
| `internal/store/attribute_test.go` | `v2`, `v3` | Variable names in tests |
| `cmd/ze/bgp/decode_test.go` | `OSPFv2/v3` | Protocol names |
| `docs/architecture/TEST_INVENTORY.md` | `conf-srv6-mup-v3` | MUP protocol version |

## Verification

```
✅ make test   - All tests pass
✅ make lint   - No issues
```

## Checklist

- [x] Delete API version constants
- [x] Delete Version field from all config structs
- [x] Remove format branching in text.go (keep only one format)
- [x] Remove v2/v3 from all function names
- [x] Remove v2/v3 from all comments
- [x] Update documentation
- [x] `make test && make lint` pass
