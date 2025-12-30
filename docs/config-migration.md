# Configuration Migration

ZeBGP migrates configurations using named transformations. Each transformation has a specific purpose and can be previewed before applying.

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

## Available Transformations

View available transformations with:

```bash
$ zebgp config migrate --list

Available transformations (in order):
  neighbor->peer            Rename 'neighbor' blocks to 'peer'
  peer-glob->template.match Move glob patterns (10.0.0.0/8) to template.match
  template.neighbor->group  Rename template.neighbor to template.group
  static->announce          Convert 'static' route blocks to 'announce'
  api->new-format           Convert old api syntax (processes, format flags) to named blocks
```

## Migration Detection

ZeBGP detects what needs migration automatically:

**Deprecated patterns (triggers migration):**
- `neighbor <IP> { }` at root level
- `peer <glob>` at root level (e.g., `peer * { }`)
- `template { neighbor <name> { } }`
- `static { }` blocks (should use `announce { }`)
- Old-style `api { processes [...] }` syntax

**Current patterns (no migration needed):**
- `peer <IP> { }` at root level (non-glob)
- `template { group <name> { } }`
- `template { match <glob> { } }`
- `announce { ipv4 { unicast { } } }` blocks
- Named `api <name> { }` blocks

## Transformations

Migration performs these transformations in order:

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

Shows if migration is needed:

```bash
$ zebgp config check old.conf
⚠️  Config needs migration

Deprecated patterns found:
  • neighbor 192.0.2.1 → peer 192.0.2.1
  • template.neighbor defaults → template.group defaults

To migrate, run:
  zebgp config migrate <file> -o <output>
  zebgp config migrate <file> --in-place
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

Converts config to current format:

```bash
# List available transformations
$ zebgp config migrate --list

# Preview what would happen
$ zebgp config migrate --dry-run old.conf
Transformation analysis:
  ⏳ neighbor->peer (pending)
  ⏳ api->new-format (pending)
  ✅ peer-glob->template.match (done)
  ✅ template.neighbor->group (done)
  ✅ static->announce (done)

Result: 2 transformation(s) would apply. All would succeed.

# Migrate to stdout (config to stdout, progress to stderr)
$ zebgp config migrate old.conf
Transformations:
  ✅ neighbor->peer
  ⏭️  peer-glob->template.match (not needed)
  ...
2 applied, 3 skipped.

# Write to new file
$ zebgp config migrate old.conf -o new.conf

# Modify in place (creates .bak backup)
$ zebgp config migrate --in-place old.conf
```

**Flags:**
- `--list` - Show available transformations
- `--dry-run` - Show what would happen without applying
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
