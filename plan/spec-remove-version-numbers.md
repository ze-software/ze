# Spec: Remove Version Numbers from Code

## Task

Remove ALL version number references from the codebase:
- API: Remove `APIVersionLegacy=6`, `APIVersionNLRI=7`, `Version int` field - ONE output format
- Migration: Remove v2/v3 terminology - semantic transforms only

## Files to Modify

### API Version Removal

| File | Action |
|------|--------|
| `pkg/api/types.go:428-431` | Delete `APIVersionLegacy`, `APIVersionNLRI` constants |
| `pkg/api/types.go` | Delete `Version int` from `ContentConfig` |
| `pkg/api/text.go` | Remove all `content.Version == APIVersionLegacy` branches - ONE format |
| `pkg/api/text_test.go` | Remove v6/v7 test cases |
| `pkg/api/json_test.go` | Remove version test cases |
| `pkg/api/handler.go:187` | Remove `"api": "v6"` from response |
| `pkg/reactor/peersettings.go:241` | Delete `Version int` field |
| `pkg/config/bgp.go:517` | Delete `Version int` field |
| `pkg/reactor/reactor.go` | Remove version default assignment |
| `cmd/zebgp/run_test.go:83` | Remove `"api": "v6"` |

### Migration Terminology Removal

| File | Action |
|------|--------|
| `pkg/config/migration/v2_to_v3_test.go` | Rename file, remove v2/v3 from test names |
| `pkg/config/migration/detect_test.go` | Rename `TestDetectV2*`/`TestDetectV3*` |
| `pkg/config/loader.go:1791` | Rename `detectV2SyntaxHint` |
| `pkg/config/bgp.go` | Remove v2/v3 comments |
| `cmd/zebgp/config_check.go` | Remove v2/v3 from exit code docs |
| `cmd/zebgp/config_fmt.go` | Remove v2/v3 from error messages |
| `cmd/zebgp/config_fmt_test.go` | Remove v2/v3 from test names |
| `.claude/zebgp/EXABGP_COMPATIBILITY.md` | Remove "Version 6/7" |

## Implementation

### Phase 1: API - Single Format

Keep only the ZeBGP format (`peer X update announce nlri ...`). Remove ExaBGP-style format option entirely.

### Phase 2: Migration - Semantic Names

Transformations already have semantic names:
- `neighbor->peer`
- `static->announce`
- `api->new-format`

Remove "v2"/"v3" from file names, function names, and comments.

## Checklist

- [ ] Delete API version constants
- [ ] Delete Version field from configs
- [ ] Remove format branching in text.go
- [ ] Rename migration test files
- [ ] Remove v2/v3 from function names
- [ ] Remove v2/v3 from comments
- [ ] make test && make lint && make functional
