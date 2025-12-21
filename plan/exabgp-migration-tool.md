# ExaBGP → ZeBGP Migration Tool

**Status:** Planning
**Created:** 2025-12-21
**Depends on:** `neighbor-to-peer-rename.md` (Phase 3 complete)

## Overview

CLI tool to convert ExaBGP configuration files to ZeBGP format. Since ExaBGP 5.0.0 config syntax ≈ ZeBGP initial format, this is primarily terminology and structural transforms plus warnings for unimplemented features.

---

## Commands

### `zebgp config check <file>`

Analyze config file, show version and required changes.

```bash
$ zebgp config check router.conf
Format: ExaBGP
Transforms needed:
  - 3 neighbor blocks → peer
  - 3 neighbor.static blocks → announce (5 routes: 4 IPv4, 1 IPv6)
  - 1 template.neighbor → template.group
Warnings:
  - line 45: flow routes not yet implemented
  - line 78: evpn not yet implemented
```

### `zebgp config import <file>`

Convert, validate, and output to stdout.

```bash
$ zebgp config import router.conf
# Outputs ZeBGP format to stdout (after validation)

$ zebgp config import router.conf > zebgp.conf
```

> **Note:** Output is always validated before being emitted. Invalid output = non-zero exit.

### `zebgp config import <file> -o <output>`

Convert and write to specified file.

```bash
$ zebgp config import router.conf -o zebgp.conf
Converted router.conf → zebgp.conf
Warnings:
  - flow routes not yet implemented (2 routes skipped)
```

### `zebgp config import <file> --in-place`

Convert file in-place, creating backup.

```bash
$ zebgp config import router.conf --in-place
Backup: router.conf.bak
Converted router.conf to ZeBGP format
```

---

## Transforms

### 1. `neighbor` → `peer`

```
# ExaBGP                          # ZeBGP
neighbor 192.0.2.1 {              peer 192.0.2.1 {
    local-as 65000;         →         local-as 65000;
    peer-as 65001;                    peer-as 65001;
}                                 }
```

**Implementation:** Already in `MigrateV2ToV3()` ✅

### 2. `template.neighbor` → `template.group`

```
# ExaBGP                          # ZeBGP
template {                        template {
    neighbor ibgp {         →         group ibgp {
        peer-as 65000;                    peer-as 65000;
    }                                 }
}                                 }
```

**Implementation:** Already in `MigrateV2ToV3()` ✅

### 3. Root `peer <glob>` → `template.match`

```
# ExaBGP                          # ZeBGP
peer * {                          template {
    hold-time 90;           →         match * {
}                                         hold-time 90;
                                      }
                                  }
```

**Implementation:** Already in `MigrateV2ToV3()` ✅

### 4. `static` → `announce` (NEW)

ExaBGP supports **both** `static { }` and `announce { }` blocks inside neighbor blocks. ZeBGP uses only `announce { }`. Migration converts `static` to `announce` with AFI/SAFI structure:

```
# ExaBGP (static block inside neighbor)
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    static {
        route 10.0.0.0/8 next-hop self;
        route 192.168.0.0/16 rd 65000:1 next-hop 1.2.3.4;
        route 2001:db8::/32 next-hop self;
    }
}

# ZeBGP (static → announce, stays inside peer block)
peer 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    announce {
        ipv4 {
            unicast 10.0.0.0/8 next-hop self;
            mpls-vpn 192.168.0.0/16 rd 65000:1 next-hop 1.2.3.4;
        }
        ipv6 {
            unicast 2001:db8::/32 next-hop self;
        }
    }
}
```

**Route Syntax Variants (both supported by ExaBGP and ZeBGP parser):**

```
# Flat syntax (single line)
route 10.0.0.0/24 next-hop 192.168.0.1 origin igp local-preference 100;

# Nested syntax (block format)
route 10.0.0.0/24 {
    next-hop 192.168.0.1;
    origin igp;
    local-preference 100;
}

# Mixed (both in same block) - also valid
```

> **Parser:** `pkg/config/parser.go:parseInlineList()` already handles all variants. Migration only transforms tree structure.

**Supported Route Attributes:**

| Category | Attributes |
|----------|------------|
| Required | `next-hop <ip>`, `next-hop self` |
| Path | `origin`, `as-path`, `med`, `local-preference`, `atomic-aggregate`, `aggregator` |
| Communities | `community`, `large-community`, `extended-community` |
| RR | `originator-id`, `cluster-list` |
| MPLS/VPN | `label`, `rd`/`route-distinguisher`, `bgp-prefix-sid` |
| Advanced | `aigp`, `path-information` (ADD-PATH) |

**AFI Detection (from prefix format):**

| Prefix Format | AFI |
|---------------|-----|
| Contains `.` but no `:` (e.g., `10.0.0.0/24`) | ipv4 |
| Contains `:` (e.g., `2001:db8::/32`, `::ffff:192.0.2.1/128`) | ipv6 |

> **Note:** IPv4-mapped IPv6 addresses (`::ffff:x.x.x.x`) are treated as IPv6.

**SAFI Detection (from prefix range + attributes):**

| Condition | → SAFI |
|-----------|--------|
| IPv4 prefix in 224.0.0.0/4 | multicast |
| IPv6 prefix in ff00::/8 | multicast |
| Has `rd` (route distinguisher) | mpls-vpn |
| Has `label` only (no rd) | mpls-vpn |
| None of above | unicast |

**Algorithm:**
1. For each `neighbor`/`peer` block in tree:
   - Find `static` child block (if present)
   - Create/get `announce` child block in same peer
   - For each `route` entry in static:
     - Parse prefix → determine AFI (ipv4/ipv6)
     - Check prefix range → multicast if in mcast range
     - Check attributes → mpls-vpn if has `rd` or `label`
     - Default → unicast
     - Move route to `peer.announce.<afi>.<safi>` (create path if needed)
   - Remove `static` block from peer
2. Merge with existing `peer.announce` block if present
3. Preserve route order within each AFI/SAFI

**Separate Sections (also inside neighbor blocks):**

| ExaBGP Section | Handling |
|----------------|----------|
| `neighbor.flow { route ... }` | Move to `peer.announce.<afi>.flow` |
| `neighbor.l2vpn { vpls { ... } }` | Move to `peer.announce.l2vpn.vpls` |
| `neighbor.l2vpn { evpn { ... } }` | Move to `peer.announce.l2vpn.evpn` |

**Edge cases:**
- Each peer keeps its own routes in its own `announce` block
- Preserve all attributes during transformation
- Handle both flat and nested route syntax

---

## Unsupported Feature Warnings

All ExaBGP address families are supported. Only structural/protocol features may be unsupported:

| Feature | Detection | Severity |
|---------|-----------|----------|
| `multi-session` | `multi-session true` | Error (not supported) |
| `operational` capability | `capability { operational }` | Warning |

**Behavior:**
- **Warning:** Include in output, print warning to stderr
- **Error:** Refuse to convert, explain why

**Supported Families (all):**
- IPv4/IPv6 unicast, multicast
- IPv4/IPv6 mpls-vpn (L3VPN)
- IPv4/IPv6 mcast-vpn
- IPv4/IPv6 flow (FlowSpec)
- IPv4/IPv6 mup
- L2VPN vpls, evpn

---

## Implementation Plan

> **Order:** Implement transforms first (4.1), then CLI (4.2), then feature detection (4.3).

### Phase 4.1: Static Block Extraction

| # | Task | Files |
|---|------|-------|
| 4.1.1 | Add `isIPv6Prefix(string) bool` helper | `pkg/config/migration/helpers.go` |
| 4.1.2 | Add `isMulticastPrefix(string) bool` helper | `pkg/config/migration/helpers.go` |
| 4.1.3 | Add `detectSAFI(route) string` helper | `pkg/config/migration/helpers.go` |
| 4.1.4 | Implement `extractStaticRoutes(tree)` | `pkg/config/migration/static.go` |
| 4.1.5 | Add to migration pipeline | `pkg/config/migration/v2_to_v3.go` |
| 4.1.6 | Tests for static extraction | `pkg/config/migration/static_test.go` |

### Phase 4.2: CLI Commands

| # | Task | Files |
|---|------|-------|
| 4.2.1 | Add `config check` command | `cmd/zebgp/config_check.go` |
| 4.2.2 | Add `config import` command | `cmd/zebgp/config_import.go` |
| 4.2.3 | Add output options (-o, --in-place) | `cmd/zebgp/config_import.go` |
| 4.2.4 | Wire up to main command tree | `cmd/zebgp/config.go` |

### Phase 4.3: Feature Detection

| # | Task | Files |
|---|------|-------|
| 4.3.1 | Define unsupported feature list | `pkg/config/migration/unsupported.go` |
| 4.3.2 | Implement `DetectUnsupported(tree)` | `pkg/config/migration/unsupported.go` |
| 4.3.3 | Integrate warnings into CLI output | `cmd/zebgp/config_import.go` |

## File Structure

```
cmd/zebgp/
├── config.go           # existing - add subcommands
├── config_check.go     # NEW: zebgp config check
├── config_import.go    # NEW: zebgp config import
└── ...

pkg/config/migration/
├── detect.go           # existing - version detection
├── v2_to_v3.go         # existing - main transforms
├── helpers.go          # NEW: prefix detection helpers
├── static.go           # NEW: static block extraction
├── static_test.go      # NEW: tests
├── unsupported.go      # NEW: feature detection
└── unsupported_test.go # NEW: tests
```

---

## Edge Cases

### Already ZeBGP Format

```bash
$ zebgp config check zebgp.conf
Format: ZeBGP (current)
No migration needed.
```

Detection: No `neighbor` blocks (uses `peer`), no `static` blocks (uses `announce`), no root `peer` globs, no `template.neighbor`

### Parse Errors

```bash
$ zebgp config import broken.conf
Error: parse error at line 23: unexpected token '}'
```

Exit with non-zero status, don't output partial config.

### Comments

**Limitation:** Comments are not preserved during migration. The parser doesn't capture them, and serialize outputs clean config.

```bash
$ zebgp config import router.conf
# Warning: comments from original file are not preserved
```

### Mixed Static and Existing Announce

If peer has both `static` and existing `announce` blocks, merge routes:

```
# Input (ExaBGP with both static and announce in neighbor)
neighbor 192.0.2.1 {
    static {
        route 10.0.0.0/8 next-hop self;
    }
    announce {
        ipv4 {
            unicast 192.168.0.0/16 next-hop self;
        }
    }
}

# Output (merged into single announce block)
peer 192.0.2.1 {
    announce {
        ipv4 {
            unicast 10.0.0.0/8 next-hop self;
            unicast 192.168.0.0/16 next-hop self;
        }
    }
}
```

---

## Success Criteria

1. ✅ `zebgp config check` correctly identifies ExaBGP format
2. ✅ `zebgp config import` produces valid ZeBGP config
3. ✅ `zebgp config import` auto-validates output before emitting
4. ✅ `neighbor.static` → `peer.announce.<afi>.<safi>`
5. ✅ IPv4 vs IPv6 correctly detected from prefix (`:` → IPv6)
6. ✅ Multicast detected from prefix range (224.0.0.0/4, ff00::/8)
7. ✅ mpls-vpn detected from `rd` or `label` attributes
8. ✅ Unsupported features generate warnings
9. ✅ `multi-session` rejected with clear error
10. ✅ `--in-place` creates backup before modifying
11. ✅ Parse errors handled gracefully
12. ✅ All existing tests still pass

---

## Testing Strategy

### Unit Tests

- `neighbor.static` → `peer.announce` with IPv4 routes
- `neighbor.static` → `peer.announce` with IPv6 routes
- `neighbor.static` → `peer.announce` with mixed routes
- `neighbor.static` merge with existing `neighbor.announce`
- Multiple neighbor blocks (each keeps own routes)
- Unsupported feature detection
- Already-migrated config detection

### Integration Tests

- Full ExaBGP config → ZeBGP config
- Round-trip: import then validate
- CLI output format
- Backup file creation

### Manual Testing

```bash
# Test with real ExaBGP configs
zebgp config check ../main/etc/exabgp/example.conf
zebgp config import ../main/etc/exabgp/example.conf | zebgp validate -
```

---

## Dependencies

- `pkg/config/migration/` - existing migration infrastructure
- `pkg/config/serialize.go` - tree serialization
- `pkg/config/parser.go` - config parsing

---

## Future Considerations

### ExaBGP API Compatibility

Separate from config migration. The ZeBGP API already uses `peer` terminology. ExaBGP scripts that send commands via API would need adaptation, but that's out of scope for this plan.

### Deprecation Timeline

After migration tool is stable:
1. Document migration process
2. Deprecate old syntax in parser (warnings)
3. Eventually remove old syntax support

---

## References

- ExaBGP config: `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/configuration/`
- ZeBGP migration: `pkg/config/migration/`
- Phase 3 complete: `plan/neighbor-to-peer-rename.md`
