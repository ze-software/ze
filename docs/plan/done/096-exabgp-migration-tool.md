# Spec: exabgp-migration-tool

## Task

CLI tool to convert ExaBGP configuration files to ZeBGP format. Transforms include:
- `neighbor` → `peer`
- `template.neighbor` → `template.group`
- `peer <glob>` → `template.match`
- `static` → `announce.<afi>.<safi>`
- API block syntax updates

## Required Reading

### Architecture Docs
- [x] `docs/architecture/config/SYNTAX.md` - ZeBGP config syntax reference
- [x] `docs/exabgp/EXABGP_COMPATIBILITY.md` - ExaBGP compatibility requirements

### RFC Summaries
- [x] `docs/rfc/rfc4271.md` - BGP-4 base (UPDATE message structure)
- [x] `docs/rfc/rfc4760.md` - Multiprotocol extensions (AFI/SAFI)
- [x] `docs/rfc/rfc8277.md` - Labeled unicast (SAFI 4)
- [x] `docs/rfc/rfc4364.md` - L3VPN (SAFI 128)

**Key insights:**
- RFC 8277: Label-only routes use SAFI 4 (labeled unicast), NOT SAFI 128
- RFC 4364: RD+label routes use SAFI 128 (MPLS-VPN)
- ZeBGP uses `nlri-mpls` for SAFI 4, `mpls-vpn` for SAFI 128

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestIsIPv6Prefix` | `internal/config/migration/helpers_test.go` | IPv6 prefix detection via `:` |
| `TestIsMulticastPrefix` | `internal/config/migration/helpers_test.go` | Multicast range detection |
| `TestDetectSAFI` | `internal/config/migration/helpers_test.go` | SAFI detection (unicast/multicast/nlri-mpls/mpls-vpn) |
| `TestExtractStaticRoutesIPv4` | `internal/config/migration/static_test.go` | IPv4 static→announce |
| `TestExtractStaticRoutesIPv6` | `internal/config/migration/static_test.go` | IPv6 static→announce |
| `TestExtractStaticRoutesMixed` | `internal/config/migration/static_test.go` | Mixed AFI routes |
| `TestExtractStaticRoutesMulticast` | `internal/config/migration/static_test.go` | Multicast SAFI detection |
| `TestExtractStaticRoutesMPLSVPN` | `internal/config/migration/static_test.go` | rd → mpls-vpn (SAFI 128) |
| `TestExtractStaticRoutesLabeledUnicast` | `internal/config/migration/static_test.go` | label-only → nlri-mpls (SAFI 4) |
| `TestExtractStaticRoutesMergeExisting` | `internal/config/migration/static_test.go` | Merge with existing announce |
| `TestMigrateAPIBlocks*` | `internal/config/migration/api_test.go` | API block transforms |
| `TestMigrate*` | `internal/config/migration/migrate_test.go` | Full pipeline |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| parsing tests | `qa/tests/parsing/` | Config parsing (10 tests) |

## Files to Modify
- `internal/config/migration/helpers.go` - SAFI detection (nlri-mpls fix)
- `internal/config/migration/helpers_test.go` - Updated test expectations
- `internal/config/migration/static.go` - Static block extraction
- `internal/config/migration/static_test.go` - Added labeled unicast test
- `internal/config/migration/api.go` - API block migration
- `internal/config/migration/migrate.go` - Migration pipeline
- `internal/config/migration/detect.go` - Legacy pattern detection
- `cmd/zebgp/config_check.go` - `zebgp config check` command
- `cmd/zebgp/config_migrate.go` - `zebgp config migrate` command

## Implementation Steps
1. **Write tests** - Created test cases for all transforms
2. **Run tests** - Verified FAIL before implementation
3. **Implement** - Migration functions
4. **Run tests** - Verified PASS
5. **Verify all** - `make lint && make test && make functional`
6. **RFC refs** - Added RFC 8277/4364 comments to detectSAFI

## RFC Documentation

### Reference Comments
```go
// RFC 8277: Labeled unicast uses SAFI 4 (prefix + label, no RD).
// RFC 4364: L3VPN uses SAFI 128 (RD:prefix + label).
func detectSAFI(prefix string, hasRD, hasLabel bool) string {
```

### Constraint Comments
```go
// RFC 4364: RD present = L3VPN (SAFI 128)
if hasRD {
    return "mpls-vpn"
}

// RFC 8277: Label only (no RD) = labeled unicast (SAFI 4)
if hasLabel {
    return "nlri-mpls"
}
```

## CLI Commands

### `zebgp config check <file>`
```bash
$ zebgp config check router.conf
⚠️  Config needs migration

Deprecated patterns found:
  • neighbor 192.0.2.1 → peer 192.0.2.1
  • neighbor.192.0.2.1.static → peer.192.0.2.1.announce.<afi>.<safi>

To migrate, run:
  zebgp config migrate <file> -o <output>
  zebgp config migrate <file> --in-place
```

### `zebgp config migrate <file>`
```bash
$ zebgp config migrate router.conf           # stdout
$ zebgp config migrate router.conf -o out    # to file
$ zebgp config migrate router.conf --in-place # in place + backup
$ zebgp config migrate --dry-run router.conf  # preview only
```

## Transforms

| # | Transform | Implementation |
|---|-----------|----------------|
| 1 | `neighbor` → `peer` | `migrate.go:migrateNeighborToPeer()` |
| 2 | `template.neighbor` → `template.group` | `migrate.go:migrateTemplateNeighborToGroup()` |
| 3 | `peer <glob>` → `template.match` | `migrate.go:migratePeerGlobToMatch()` |
| 4 | `static` → `announce.<afi>.<safi>` | `static.go:ExtractStaticRoutes()` |
| 5 | API block syntax | `api.go:MigrateAPIBlocks()` |

## SAFI Detection

| Condition | SAFI | ZeBGP Key |
|-----------|------|-----------|
| Prefix in 224.0.0.0/4 or ff00::/8 | 2 | `multicast` |
| Has `rd` attribute | 128 | `mpls-vpn` |
| Has `label` only (no rd) | 4 | `nlri-mpls` |
| Default | 1 | `unicast` |

## Out of Scope

**API Scripts:** ExaBGP Python scripts require manual migration. Use AI-assisted conversion with `docs/architecture/api/ARCHITECTURE.md` as reference.

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL verified
- [x] Implementation complete
- [x] Tests PASS verified

### Verification
- [x] `make lint` passes (migration package)
- [x] `make test` passes
- [x] `make functional` passes (53 tests)

### Documentation
- [x] Required docs read
- [x] RFC summaries read
- [x] RFC references added to code
- [x] RFC constraint comments added

### Completion
- [x] Spec moved to `docs/plan/done/096-exabgp-migration-tool.md`
