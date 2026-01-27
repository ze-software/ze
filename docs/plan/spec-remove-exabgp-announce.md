# Spec: remove-exabgp-announce

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. Parent: `docs/plan/spec-remove-exabgp-syntax.md` - umbrella spec
3. `.claude/rules/compatibility.md` - NO ExaBGP in engine
4. `docs/plan/done/180-native-update-syntax.md` - native syntax already implemented

## Task

Remove ExaBGP config syntax and the duplicate Go schema definitions. Per `.claude/rules/compatibility.md`:
- **Engine code:** No ExaBGP format awareness
- **Config migration:** External tool only, not runtime

## Required Reading

### Rules
- [ ] `.claude/rules/compatibility.md` - compatibility policy
- [ ] `.claude/rules/no-layering.md` - delete old, don't keep both

### Architecture Docs
- [ ] `docs/plan/done/166-yang-only-schema.md` - explains why Go schema exists
- [ ] `docs/plan/done/180-native-update-syntax.md` - replacement syntax

### Source Files
- [ ] `internal/config/bgp.go` - Go schema to delete (~210 Field() calls)
- [ ] `internal/plugin/bgp/schema/ze-bgp.yang` - YANG blocks to delete

**Key insights:**
- `LegacyBGPSchema()` exists only for migration tool parsing old ExaBGP syntax
- Migration tool moves to external `ze exabgp` command (already exists)
- All Go-based schema helpers can be deleted after YANG cleanup

## What to Remove

### YANG (ze-bgp.yang)

| Block | Lines | Description |
|-------|-------|-------------|
| `container announce` | ~150 | ExaBGP route announcements |
| `container static` | ~10 | Old ExaBGP syntax |
| `container flow` | ~20 | ExaBGP FlowSpec (not ipv4/flowspec) |
| `container l2vpn` | ~30 | ExaBGP L2VPN syntax |
| `container withdraw` | ~30 | ExaBGP withdraw |
| `container operational` | ~5 | ExaBGP-specific |
| `container api` | ~20 | Legacy API block |

### Go (bgp.go)

| Function | Lines | Description |
|----------|-------|-------------|
| `flowRouteAttributes()` | ~15 | Field definitions |
| `mcastVpnAttributes()` | ~15 | Field definitions |
| `vplsAttributes()` | ~15 | Field definitions |
| `routeAttributes()` | ~25 | Field definitions |
| `peerFields()` | ~130 | Field definitions |
| `templatePeerFields()` | ~80 | Field definitions |
| `LegacyBGPSchema()` | ~100 | Legacy schema builder |
| Related parsing code | ~200 | extractMVPNRoutes, etc. |

**Total: ~210 Field() calls, ~600 lines**

### Go (bgp.go extraction logic)

| Function | Action |
|----------|--------|
| `extractRoutesFromTree()` | Keep only `update` block parsing |
| `extractMVPNRoutes()` | Delete (ExaBGP-specific) |
| `extractVPLSRoutes()` | Delete (ExaBGP-specific) |
| `extractFlowSpecRoutes()` | Delete (ExaBGP-specific) |

## Migration Path

Users with old ExaBGP configs:
1. Run `ze bgp config migrate old.conf > new.conf`
2. Use new config with native `update { }` syntax

Migration tool already exists and handles conversion.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestYANGSchema_NoAnnounce` | `internal/config/yang_schema_test.go` | announce block rejected | |
| `TestYANGSchema_NoStatic` | `internal/config/yang_schema_test.go` | static block rejected | |
| `TestParseUpdateBlock_*` | `internal/config/bgp_test.go` | Native syntax still works | ✅ (existing) |

### Functional Tests to Convert
| Old Test | New Test | Change |
|----------|----------|--------|
| Tests using `announce { }` | Use `update { }` | Convert syntax |
| Tests using `static { }` | Use `update { }` | Convert syntax |

## Files to Modify

- `internal/plugin/bgp/schema/ze-bgp.yang` - Remove ExaBGP blocks
- `internal/config/bgp.go` - Remove Go schema, keep extraction logic for `update { }`
- `internal/config/bgp_test.go` - Convert tests, add rejection tests
- `cmd/ze/bgp/config_migrate.go` - Update to use external migration
- `cmd/ze/bgp/config_fmt.go` - Remove LegacyBGPSchema usage
- `cmd/ze/bgp/config_check.go` - Remove LegacyBGPSchema usage

## Files to Delete

- None (code removed from existing files)

## Implementation Steps

1. **Write rejection tests** - Tests that verify old syntax is rejected
   → **Review:** Tests fail initially (old syntax still accepted)?

2. **Run tests** - Verify FAIL

3. **Remove YANG blocks** - Delete announce, static, flow, l2vpn, withdraw, operational, api
   → **Review:** Only `update { }` remains for routes?

4. **Remove Go schema** - Delete LegacyBGPSchema and all helper functions
   → **Review:** ~600 lines removed?

5. **Update migration commands** - Remove LegacyBGPSchema usage
   → **Review:** Migration still works via external tool?

6. **Convert tests** - Update tests using old syntax to native syntax
   → **Review:** All tests use `update { }` syntax?

7. **Run tests** - Verify PASS

8. **Verify all** - `make lint && make test && make functional`

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Cleanup Verification
- [ ] No `announce { }` in YANG
- [ ] No `static { }` in YANG
- [ ] No `LegacyBGPSchema()` in Go
- [ ] No `Field("` definitions in bgp.go (except tests)
- [ ] All tests use native `update { }` syntax
