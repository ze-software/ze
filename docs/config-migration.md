# Configuration Migration

ZeBGP uses a versioned configuration format. When the format changes, the `zebgp config` commands help migrate existing configurations to the current version.

## Quick Start

```bash
# Check config version and what needs migration
zebgp config check myconfig.conf

# Preview migration changes
zebgp config migrate --dry-run myconfig.conf

# Migrate in place (creates backup)
zebgp config migrate --in-place myconfig.conf

# Migrate to new file
zebgp config migrate myconfig.conf -o myconfig-v3.conf

# Format/normalize a v3 config
zebgp config fmt myconfig.conf

# Format in place
zebgp config fmt -w myconfig.conf

# Check if formatting needed (for CI)
zebgp config fmt --check myconfig.conf
```

## Config Versions

| Version | Name | Description |
|---------|------|-------------|
| v1 | ExaBGP main | RIB options at neighbor level (not yet supported) |
| v2 | ZeBGP intermediate | Uses `neighbor` keyword, `template { neighbor }` |
| v3 | ZeBGP current | Uses `peer` keyword, `template { group/match }` |

## Version Detection

ZeBGP detects config version automatically using heuristics (no version field required):

**v2 indicators:**
- `neighbor <IP> { }` at root level
- `peer <glob>` at root level (e.g., `peer * { }`)
- `template { neighbor <name> { } }`

**v3 indicators:**
- `peer <IP> { }` at root level (non-glob)
- `template { group <name> { } }`
- `template { match <glob> { } }`

## Migration: v2 to v3

The v2→v3 migration performs these transformations:

| v2 Syntax | v3 Syntax |
|-----------|-----------|
| `neighbor <IP> { }` | `peer <IP> { }` |
| `peer * { }` (root glob) | `template { match * { } }` |
| `peer 192.*.*.* { }` | `template { match 192.*.*.* { } }` |
| `template { neighbor <name> { } }` | `template { group <name> { } }` |

### Example

**Before (v2):**
```
template {
    neighbor defaults {
        hold-time 90;
    }
}

peer * {
    capability { route-refresh; }
}

neighbor 192.0.2.1 {
    inherit defaults;
    local-as 65000;
    peer-as 65001;
}
```

**After (v3):**
```
template {
    group defaults {
        hold-time 90;
    }
    match * {
        capability { route-refresh; }
    }
}

peer 192.0.2.1 {
    inherit defaults;
    local-as 65000;
    peer-as 65001;
}
```

## CLI Commands

### zebgp config check

Shows config version and any issues:

```bash
$ zebgp config check old.conf
Config version: v2 (ZeBGP intermediate)
Target version: v3 (ZeBGP current)

Migration required:
  neighbor 192.0.2.1 → peer 192.0.2.1
  template.neighbor.defaults → template.group.defaults
```

### zebgp config fmt

Formats and normalizes v3 config files:

```bash
# Print formatted config to stdout
$ zebgp config fmt config.conf

# Write back to file
$ zebgp config fmt -w config.conf

# Check if formatting needed (for CI)
$ zebgp config fmt --check config.conf
# Exit 0 = no changes needed
# Exit 1 = changes needed

# Show diff of changes
$ zebgp config fmt --diff config.conf
```

**Flags:**
- (none) - Print to stdout
- `-w` - Write result to source file
- `--check` - Exit 1 if changes needed (CI use)
- `--diff` - Show unified diff
- `-` - Read from stdin

**Note:** `fmt` only works on v3 configs. Run `migrate` first for v2 configs.

### zebgp config migrate

Converts config to current version:

```bash
# Preview changes (no modifications)
$ zebgp config migrate --dry-run old.conf

# Write to new file
$ zebgp config migrate old.conf -o new.conf

# Modify in place (creates .bak backup)
$ zebgp config migrate --in-place old.conf
```

**Flags:**
- `--dry-run` - Show changes without writing
- `-o <file>` - Write to specified file
- `--in-place` - Modify original file (creates backup)

## Unsupported Features

Some ExaBGP features are detected but not supported in ZeBGP:

| Feature | Location | Notes |
|---------|----------|-------|
| `multi-session` | `capability { }` | Non-standard extension |
| `operational` | `capability { }` | ExaBGP-specific |
| `operational` block | `peer { }` | ExaBGP-specific messaging |

These features generate warnings but don't block migration or loading.

## Backup Strategy

When using `--in-place`, a backup is created:

```
original.conf.bak
```

The backup contains the original file contents before migration.

## Error Handling

If migration fails:
1. Original file is unchanged
2. Error message describes the issue
3. Fix the issue and retry

Common errors:
- Invalid syntax in source config
- Conflicting definitions after migration
- File permission issues
