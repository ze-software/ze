# Spec: Remove Version Numbers from Code

## Task

Remove ALL version number references from the codebase. No v6, v7, v2, v3, "legacy", "current" - nothing.

- API: Delete version constants and fields. ONE output format only.
- Migration: Semantic transform names only. No version terminology.

## Files to Modify

### API Version Removal

| File | Action | Status |
|------|--------|--------|
| `pkg/api/types.go:428-431` | Delete `APIVersionLegacy`, `APIVersionNLRI` constants | ✅ Done |
| `pkg/api/types.go` | Delete `Version int` from `ContentConfig` | ✅ Done |
| `pkg/api/text.go` | Remove all `content.Version == APIVersionLegacy` branches | ✅ Done |
| `pkg/api/text_test.go` | Remove v6/v7 test cases | ✅ Done |
| `pkg/api/json_test.go` | Remove version test cases | ✅ Done |
| `pkg/api/handler.go:187` | Remove `"api": "v6"` from response | ✅ Done |
| `pkg/reactor/peersettings.go:241` | Delete `Version int` field | ✅ Done |
| `pkg/config/bgp.go:517` | Delete `Version int` field | ✅ Done |
| `pkg/reactor/reactor.go` | Remove version default assignment | ✅ Done |
| `cmd/zebgp/run_test.go:83` | Remove `"api": "v6"` | ✅ Done |

### Migration Terminology Removal

| File | Action | Status |
|------|--------|--------|
| `pkg/config/migration/v2_to_v3_test.go` | Rename file | ✅ Done (now transformations_test.go) |
| `pkg/config/migration/detect_test.go` | Rename function names | ✅ Done |
| `pkg/config/loader.go` | Rename `detectV2SyntaxHint` | ✅ Done |
| `pkg/config/migration/detect_test.go` | Remove v2/v3 from comments (60 refs) | ❌ TODO |
| `pkg/config/bgp.go` | Remove v2/v3 from comments (20 refs) | ❌ TODO |
| `pkg/config/loader.go` | Remove v2/v3 from comments (6 refs) | ❌ TODO |
| `pkg/config/bgp_test.go` | Remove v2/v3 from comments (9 refs) | ❌ TODO |
| `pkg/config/migration/migrate_test.go` | Remove v2/v3 from comments (7 refs) | ❌ TODO |
| `pkg/config/migration/transformations_test.go` | Remove v2/v3 from comments (5 refs) | ❌ TODO |
| `cmd/zebgp/config_check.go` | Remove v2/v3 from comments (8 refs) | ❌ TODO |
| `cmd/zebgp/config_fmt.go` | Remove v2/v3 from comments (5 refs) | ❌ TODO |
| `cmd/zebgp/config_fmt_test.go` | Remove v2/v3 from comments (6 refs) | ❌ TODO |
| `cmd/zebgp/config_test.go` | Remove v2/v3 from comments (6 refs) | ❌ TODO |
| `.claude/zebgp/config/SYNTAX.md` | Remove v2/v3 (2 refs) | ❌ TODO |

## Summary

- **API version code:** ✅ Complete
- **Migration function names:** ✅ Complete
- **v2/v3 in comments:** ❌ ~113 occurrences remaining

## Checklist

- [x] Delete API version constants
- [x] Delete Version field from all config structs
- [x] Remove format branching in text.go (keep only one format)
- [x] Rename `v2_to_v3_test.go` → `transformations_test.go`
- [x] Rename `TestDetectV2*`/`TestDetectV3*` functions
- [ ] Remove v2/v3 from all comments (~113 remaining)
- [ ] Update documentation
- [ ] `make test && make lint && make functional`
