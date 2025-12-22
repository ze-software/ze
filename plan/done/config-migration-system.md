# Config Migration System Plan

**Status:** Complete (core functionality; fmt/auto-upgrade optional)
**Created:** 2025-12-21
**Updated:** 2025-12-21

---

## Overview

Database-style migration system for ExaBGP/ZeBGP configuration files. Handles deprecated option positions and format changes through versioned, sequential migrations applied in-memory before final config loading.

---

## Design Principles

1. **Strict by Default** - Refuse to start with deprecated config; require explicit upgrade
2. **Opt-in Optimistic Loading** - User must enable `--auto-upgrade` (accepts risk)
3. **Sequential Migrations** - v1→v2→v3→...→vN, never skip versions
4. **Heuristic Detection** - No version field; detect from config structure
5. **In-Memory First** - Transform Tree before typed conversion
6. **Backup Before Write** - Never modify original without backup
7. **Idempotent Migrations** - Safe to run multiple times

---

## Version Baseline

### Version 1 (v1) = ExaBGP main (current)

This is the ExaBGP main branch format as of 2025-12-21. RIB-related options are at neighbor level:

```
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    group-updates true;      # ← at neighbor level
    auto-flush true;         # ← at neighbor level
    adj-rib-in false;        # ← at neighbor level
    adj-rib-out false;       # ← at neighbor level
    manual-eor false;        # ← at neighbor level
}
```

### Version 2 (v2) = ZeBGP intermediate format

RIB-related options move to structured `rib { }` block:

```
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    rib {
        in {
            enable false;    # was adj-rib-in
        }
        out {
            enable false;    # was adj-rib-out
            group-updates true;
            auto-flush true;
        }
    }
    manual-eor false;        # stays at neighbor level (or moves?)
}
```

### Version 3 (v3) = ZeBGP target format

**See:** `plan/neighbor-to-peer-rename.md` for full details.

Renames `neighbor` to `peer`, restructures templates:

```
template {
    # Glob patterns - applied in config order to matching peers
    match * {
        rib { out { group-updates true; } }
    }
    match 192.168.*.* {
        rib { out { auto-commit-delay 50ms; } }
    }

    # Named templates - applied via explicit inherit
    group ibgp-rr {
        peer-as 65000;
        capability { route-refresh; }
    }
}

peer 192.0.2.1 {
    inherit ibgp-rr;
    local-as 65000;
}
```

**Key changes v2→v3:**
| v2 Syntax | v3 Syntax |
|-----------|-----------|
| `neighbor <IP> { }` | `peer <IP> { }` |
| `peer * { }` (root) | `template { match * { } }` |
| `template { neighbor <name> { } }` | `template { group <name> { } }` |

**Configuration Syntax Rules (v3):**
1. **Match order is config order** - `match` blocks apply in file order, not by specificity
2. **Multiple inheritance with last-wins** - Multiple `inherit` allowed, later overrides earlier
3. **Match only in template** - `match` blocks only valid inside `template { }`

---

## Architecture

### Core Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Config Loading Flow                          │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────────┐  │
│  │  Input   │───▶│ Tokenize │───▶│  Parse   │───▶│   Detect     │  │
│  │  String  │    │          │    │ (Lenient)│    │   Version    │  │
│  └──────────┘    └──────────┘    └──────────┘    └──────┬───────┘  │
│                                                          │          │
│                    ┌─────────────────────────────────────┤          │
│                    │                                     │          │
│                    ▼                                     ▼          │
│              ┌──────────┐                         ┌──────────┐      │
│              │ v1 (old) │                         │ vN (cur) │      │
│              └────┬─────┘                         └────┬─────┘      │
│                   │                                    │            │
│         ┌─────────┴─────────┐                          │            │
│         ▼                   │                          │            │
│  ┌────────────────┐         │                          │            │
│  │ --auto-upgrade │ NO      │                          │            │
│  │    enabled?    │────────▶│ ERROR: Run               │            │
│  └───────┬────────┘         │ `zebgp config upgrade`   │            │
│          │ YES              │                          │            │
│          ▼                  │                          │            │
│  ┌──────────────────────────┴──────────────────────┐   │            │
│  │               Migration Pipeline                 │   │            │
│  │  ┌────────┐   ┌────────┐   ┌────────┐           │   │            │
│  │  │ v1→v2  │──▶│ v2→v3  │──▶│  ...   │───────────┼───┘            │
│  │  └────────┘   └────────┘   └────────┘           │                │
│  └─────────────────────────────────────────────────┘                │
│                         │                                           │
│                         ▼                                           │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────────┐  │
│  │ Validate │───▶│ Convert  │───▶│ BGPConfig│───▶│   Reactor    │  │
│  │  Schema  │    │ To Types │    │  (typed) │    │   (runtime)  │  │
│  └──────────┘    └──────────┘    └──────────┘    └──────────────┘  │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Type Definitions

```go
// pkg/config/migration/version.go

// ConfigVersion represents a config schema version
type ConfigVersion int

const (
    VersionUnknown ConfigVersion = 0
    Version1       ConfigVersion = 1  // ExaBGP main (2025-12) - RIB opts at neighbor level
    Version2       ConfigVersion = 2  // ZeBGP intermediate - RIB opts in rib { } block
    Version3       ConfigVersion = 3  // ZeBGP target - neighbor→peer, template restructure
    VersionCurrent               = Version3
)

// VersionName returns human-readable version name
func (v ConfigVersion) String() string {
    switch v {
    case Version1:
        return "v1 (ExaBGP main 2025-12)"
    case Version2:
        return "v2 (ZeBGP intermediate)"
    case Version3:
        return "v3 (ZeBGP current)"
    default:
        return "unknown"
    }
}

// Migration transforms a Tree from one version to another
type Migration struct {
    From        ConfigVersion
    To          ConfigVersion
    Name        string                      // e.g., "rib-restructure"
    Description string                      // Human-readable explanation
    Migrate     func(*Tree) (*Tree, error)
}

// MigrationResult tracks what was changed
type MigrationResult struct {
    FromVersion ConfigVersion
    ToVersion   ConfigVersion
    Applied     []string   // Migration names applied
    Changes     []Change   // Specific changes made
    Warnings    []string   // Non-fatal issues
    BackupPath  string     // Path to backup file (if written)
}

// Change describes a single config modification
type Change struct {
    Type     string   // "move", "rename", "remove", "add"
    OldPath  string   // e.g., "neighbor.*.group-updates"
    NewPath  string   // e.g., "neighbor.*.rib.out.group-updates"
    OldValue string   // optional
    NewValue string   // optional
}
```

### Heuristic Version Detection

```go
// pkg/config/migration/detect.go

// DetectVersion examines a Tree to determine its schema version.
// Uses heuristic detection based on config structure - no version field needed.
func DetectVersion(tree *Tree) ConfigVersion {
    // Check newest to oldest

    // v3: Has peer (not glob) at root, template.group, template.match
    if hasV3Patterns(tree) {
        return Version3
    }

    // v2: Has neighbor at root, template.neighbor, or peer glob at root
    if hasV2Patterns(tree) {
        return Version2
    }

    // v1: Has group-updates/auto-flush/adj-rib-* at neighbor level
    if hasNeighborLevelRibOpts(tree) {
        return Version1
    }

    // No deprecated patterns found = assume current
    return VersionCurrent
}

// hasV3Patterns checks for v3 config structure
func hasV3Patterns(tree *Tree) bool {
    // v3: template.group or template.match exist
    if tmpl := tree.GetContainer("template"); tmpl != nil {
        if len(tmpl.FindAll("group")) > 0 || len(tmpl.FindAll("match")) > 0 {
            return true
        }
    }
    // v3: has peer at root that is NOT a glob pattern
    for _, p := range tree.FindAll("peer") {
        if !isGlobPattern(p.Key()) {
            return true
        }
    }
    return false
}

// hasV2Patterns checks for v2 config structure
func hasV2Patterns(tree *Tree) bool {
    // v2: has "neighbor" at root level
    if len(tree.FindAll("neighbor")) > 0 {
        return true
    }
    // v2: has peer glob at root level
    for _, p := range tree.FindAll("peer") {
        if isGlobPattern(p.Key()) {
            return true
        }
    }
    // v2: template.neighbor exists
    if tmpl := tree.GetContainer("template"); tmpl != nil {
        if len(tmpl.FindAll("neighbor")) > 0 {
            return true
        }
    }
    return false
}

// hasRibBlock checks if any neighbor has rib { } sub-block
func hasRibBlock(tree *Tree) bool {
    for _, neighbor := range tree.FindAll("neighbor") {
        if neighbor.Has("rib") {
            return true
        }
    }
    return false
}

// hasNeighborLevelRibOpts checks for v1-style RIB options at neighbor level
func hasNeighborLevelRibOpts(tree *Tree) bool {
    v1Fields := []string{"group-updates", "auto-flush", "adj-rib-in", "adj-rib-out"}
    for _, neighbor := range tree.FindAll("neighbor") {
        for _, field := range v1Fields {
            if neighbor.Has(field) {
                return true
            }
        }
    }
    return false
}

// DeprecatedField describes a field that needs migration
type DeprecatedField struct {
    Path        string        // e.g., "neighbor.192.0.2.1.group-updates"
    Field       string        // e.g., "group-updates"
    OldLocation string        // Human-readable old location
    NewLocation string        // Human-readable new location
    Version     ConfigVersion // Version this field belongs to
}

// FindDeprecated scans tree for deprecated fields and returns details
func FindDeprecated(tree *Tree) []DeprecatedField {
    var found []DeprecatedField

    v1Fields := map[string]string{
        "group-updates": "neighbor.*.rib.out.group-updates",
        "auto-flush":    "neighbor.*.rib.out.auto-flush",
        "adj-rib-in":    "neighbor.*.rib.in.enable",
        "adj-rib-out":   "neighbor.*.rib.out.enable",
    }

    for _, neighbor := range tree.FindAll("neighbor") {
        key := neighbor.Key() // e.g., "192.0.2.1"
        for field, newLoc := range v1Fields {
            if neighbor.Has(field) {
                found = append(found, DeprecatedField{
                    Path:        fmt.Sprintf("neighbor.%s.%s", key, field),
                    Field:       field,
                    OldLocation: fmt.Sprintf("neighbor { %s }", field),
                    NewLocation: newLoc,
                    Version:     Version1,
                })
            }
        }
    }

    return found
}
```

### Migration Registry

```go
// pkg/config/migration/registry.go

type Registry struct {
    migrations []Migration
}

func NewRegistry() *Registry {
    r := &Registry{}
    r.Register(migrateV1ToV2)
    r.Register(migrateV2ToV3)  // neighbor→peer, template restructure
    return r
}

func (r *Registry) Register(m Migration) {
    r.migrations = append(r.migrations, m)
}

// MigrateTo applies all migrations from detected version to target
func (r *Registry) MigrateTo(tree *Tree, from, to ConfigVersion) (*Tree, *MigrationResult, error) {
    result := &MigrationResult{
        FromVersion: from,
        ToVersion:   to,
    }
    current := tree.Clone()

    for _, m := range r.migrations {
        if m.From >= from && m.To <= to {
            var err error
            current, err = m.Migrate(current)
            if err != nil {
                return nil, result, fmt.Errorf("migration %s failed: %w", m.Name, err)
            }
            result.Applied = append(result.Applied, m.Name)
        }
    }
    return current, result, nil
}
```

### Loader Integration

```go
// pkg/config/loader.go (modified)

type LoadOptions struct {
    AutoUpgrade   bool   // Apply migrations automatically (opt-in, risky)
    BackupOnWrite bool   // Create backup before modifying file
    BackupDir     string // Where to store backups (default: same dir)
}

func LoadReactorFile(path string, opts LoadOptions) (*Reactor, *MigrationResult, error) {
    content, err := os.ReadFile(path)
    if err != nil {
        return nil, nil, err
    }

    // Phase 1: Parse to Tree (lenient - accepts deprecated fields)
    tree, err := ParseLenient(string(content))
    if err != nil {
        return nil, nil, err
    }

    // Phase 2: Detect version via heuristics
    version := DetectVersion(tree)
    result := &MigrationResult{FromVersion: version, ToVersion: VersionCurrent}

    // Phase 3: Handle deprecated config
    if version < VersionCurrent {
        deprecated := FindDeprecated(tree)

        if !opts.AutoUpgrade {
            // Default: refuse to start, require explicit upgrade
            return nil, result, &DeprecatedConfigError{
                ConfigPath:      path,
                DetectedVersion: version,
                CurrentVersion:  VersionCurrent,
                Deprecated:      deprecated,
            }
        }

        // Opt-in: auto-upgrade with backup
        if opts.BackupOnWrite {
            backupPath, err := createBackup(path, opts.BackupDir)
            if err != nil {
                return nil, result, fmt.Errorf("backup failed: %w", err)
            }
            result.BackupPath = backupPath
        }

        // Apply migrations
        registry := NewRegistry()
        tree, result, err = registry.MigrateTo(tree, version, VersionCurrent)
        if err != nil {
            return nil, result, err
        }
    }

    // Phase 4: Validate against current schema (strict)
    if err := ValidateTree(tree, BGPSchema()); err != nil {
        return nil, result, err
    }

    // Phase 5: Convert to typed config
    cfg, err := TreeToConfig(tree)
    if err != nil {
        return nil, result, err
    }

    // Phase 6: Create reactor
    reactor, err := CreateReactor(cfg)
    return reactor, result, err
}
```

---

## Error Handling

### DeprecatedConfigError

Returned when deprecated config detected and `AutoUpgrade=false` (the default):

```go
type DeprecatedConfigError struct {
    ConfigPath      string
    DetectedVersion ConfigVersion
    CurrentVersion  ConfigVersion
    Deprecated      []DeprecatedField
}

func (e *DeprecatedConfigError) Error() string {
    var b strings.Builder
    b.WriteString(fmt.Sprintf(
        "configuration file %q uses deprecated format (%s)\n",
        e.ConfigPath, e.DetectedVersion,
    ))
    b.WriteString(fmt.Sprintf("current format is %s\n\n", e.CurrentVersion))
    b.WriteString("Deprecated fields found:\n")
    for _, d := range e.Deprecated {
        b.WriteString(fmt.Sprintf("  - %s\n", d.Path))
        b.WriteString(fmt.Sprintf("    move to: %s\n", d.NewLocation))
    }
    b.WriteString("\nTo upgrade your configuration, run:\n")
    b.WriteString(fmt.Sprintf("  zebgp config upgrade %s\n\n", e.ConfigPath))
    b.WriteString("Or preview changes first:\n")
    b.WriteString(fmt.Sprintf("  zebgp config upgrade --dry-run %s\n", e.ConfigPath))
    return b.String()
}
```

---

## CLI Commands

### Command Structure

```
zebgp config
├── upgrade [file]     # Migrate config to current version
├── check [file]       # Show version and deprecated fields
├── dump               # Dump running/parsed config (was dump-config)
└── fmt [file]         # Format and normalize config (also applies migrations)
```

### Default Behavior (strict)

```bash
# Normal load - FAILS if deprecated config detected
$ zebgp --config exabgp.conf
Error: configuration file "exabgp.conf" uses deprecated format (v1)
current format is v2

Deprecated fields found:
  - neighbor.192.0.2.1.group-updates
    move to: neighbor.*.rib.out.group-updates
  - neighbor.192.0.2.1.adj-rib-out
    move to: neighbor.*.rib.out.enable

To upgrade your configuration, run:
  zebgp config upgrade exabgp.conf

Or preview changes first:
  zebgp config upgrade --dry-run exabgp.conf
```

### Explicit Upgrade Command

```bash
# Check config without modifying
$ zebgp config upgrade --dry-run exabgp.conf
Detected version: v1 (ExaBGP main 2025-12)
Target version: v2 (ZeBGP current)

Changes that would be applied:
  [move] neighbor.192.0.2.1.group-updates → neighbor.192.0.2.1.rib.out.group-updates
  [move] neighbor.192.0.2.1.auto-flush → neighbor.192.0.2.1.rib.out.auto-flush
  [move] neighbor.192.0.2.1.adj-rib-out → neighbor.192.0.2.1.rib.out.enable
  [move] neighbor.192.0.2.1.adj-rib-in → neighbor.192.0.2.1.rib.in.enable

No changes written (dry-run mode).

# Actually upgrade (creates backup)
$ zebgp config upgrade exabgp.conf
Backup created: exabgp.conf.20251221-143022.bak
Upgraded exabgp.conf from v1 to v2
Applied migrations:
  - rib-restructure

# Upgrade to different file
$ zebgp config upgrade exabgp.conf -o exabgp-v2.conf
Wrote upgraded config to exabgp-v2.conf

# Check current version
$ zebgp config check exabgp.conf
Config version: v2 (ZeBGP current)
No deprecated fields found.
```

### Opt-in Auto-Upgrade (risky)

```bash
# For users who accept the risk of automatic upgrades
$ zebgp --config exabgp.conf --auto-upgrade
Warning: Auto-upgrading config from v1 to v2
Backup created: exabgp.conf.20251221-143022.bak
Starting ZeBGP...
```

### Config Formatter (zebgp config fmt)

Formats and normalizes configuration files. Does **not** apply migrations - use `zebgp config upgrade` first if needed.

```bash
# Format config (normalizes style only)
$ zebgp config fmt exabgp.conf
Backup created: exabgp.conf.20251221-143022.bak
Formatted exabgp.conf

# Preview formatting changes
$ zebgp config fmt --dry-run exabgp.conf
--- exabgp.conf (original)
+++ exabgp.conf (formatted)
@@ -1,8 +1,8 @@
-neighbor 192.0.2.1{
-local-as 65000;
-    peer-as 65001;
-rib{out{group-updates true;}}}
+neighbor 192.0.2.1 {
+    local-as 65000;
+    peer-as 65001;
+    rib {
+        out {
+            group-updates true;
+        }
+    }
+}

No changes written (dry-run mode).

# Format to stdout (for piping)
$ zebgp config fmt --stdout exabgp.conf > exabgp-formatted.conf

# Format and write to different file
$ zebgp config fmt exabgp.conf -o exabgp-new.conf

# Upgrade + format in one pipeline
$ zebgp config upgrade exabgp.conf && zebgp config fmt exabgp.conf
```

**Formatting rules:**
1. Consistent indentation (4 spaces)
2. Alphabetical ordering of top-level blocks
3. Semicolons on leaf values
4. Consistent spacing around braces
5. Remove redundant defaults (optional, via `--compact`)
6. Preserve comments (where possible)

**Note:** `fmt` operates on valid configs only. Run `upgrade` first for deprecated configs.

---

## Known Migrations

### Migration: v1 → v2 (ExaBGP main → ZeBGP intermediate)

| Field | v1 Location | v2 Location |
|-------|-------------|-------------|
| `group-updates` | `neighbor { group-updates }` | `neighbor { rib { out { group-updates } } }` |
| `auto-flush` | `neighbor { auto-flush }` | `neighbor { rib { out { auto-flush } } }` |
| `adj-rib-out` | `neighbor { adj-rib-out }` | `neighbor { rib { out { enable } } }` |
| `adj-rib-in` | `neighbor { adj-rib-in }` | `neighbor { rib { in { enable } } }` |
| `manual-eor` | `neighbor { manual-eor }` | TBD (stays or moves?) |

**Detection heuristic:** If neighbor has `group-updates`, `auto-flush`, `adj-rib-in`, or `adj-rib-out` directly → v1

### Migration: v2 → v3 (ZeBGP intermediate → ZeBGP target)

**See:** `plan/neighbor-to-peer-rename.md` for full implementation details.

| v2 Syntax | v3 Syntax |
|-----------|-----------|
| `neighbor <IP> { }` | `peer <IP> { }` |
| `peer * { }` (root glob) | `template { match * { } }` |
| `peer 192.*.*.* { }` (root glob) | `template { match 192.*.*.* { } }` |
| `template { neighbor <name> { } }` | `template { group <name> { } }` |

**Detection heuristic:**
- Has `neighbor` at root level → v2
- Has `peer` glob at root level → v2
- Has `template { neighbor }` → v2

**Configuration Syntax Rules (v3):**
1. `match` blocks apply in config file order (not specificity)
2. Multiple `inherit` allowed with last-wins semantics
3. `match` only valid inside `template { }`

### Example Migration Implementation

```go
// pkg/config/migration/v1_to_v2.go

var migrateV1ToV2 = Migration{
    From:        Version1,
    To:          Version2,
    Name:        "rib-restructure",
    Description: "Move RIB options from neighbor level to rib { } block",
    Migrate:     doV1ToV2,
}

func doV1ToV2(tree *Tree) (*Tree, error) {
    result := tree.Clone()

    for _, neighbor := range result.FindAll("neighbor") {
        ribOut := make(map[string]interface{})
        ribIn := make(map[string]interface{})

        // Move group-updates → rib.out.group-updates
        if v := neighbor.Remove("group-updates"); v != nil {
            ribOut["group-updates"] = v
        }

        // Move auto-flush → rib.out.auto-flush
        if v := neighbor.Remove("auto-flush"); v != nil {
            ribOut["auto-flush"] = v
        }

        // Move adj-rib-out → rib.out.enable
        if v := neighbor.Remove("adj-rib-out"); v != nil {
            ribOut["enable"] = v
        }

        // Move adj-rib-in → rib.in.enable
        if v := neighbor.Remove("adj-rib-in"); v != nil {
            ribIn["enable"] = v
        }

        // Create rib block if we have anything
        if len(ribOut) > 0 || len(ribIn) > 0 {
            rib := neighbor.GetOrCreate("rib")
            if len(ribOut) > 0 {
                out := rib.GetOrCreate("out")
                for k, v := range ribOut {
                    out.Set(k, v)
                }
            }
            if len(ribIn) > 0 {
                in := rib.GetOrCreate("in")
                for k, v := range ribIn {
                    in.Set(k, v)
                }
            }
        }
    }

    return result, nil
}
```

### Example v2→v3 Migration Implementation

```go
// pkg/config/migration/v2_to_v3.go

var migrateV2ToV3 = Migration{
    From:        Version2,
    To:          Version3,
    Name:        "neighbor-to-peer",
    Description: "Rename neighbor→peer, move peer globs to template.match",
    Migrate:     doV2ToV3,
}

func doV2ToV3(tree *Tree) (*Tree, error) {
    result := tree.Clone()

    // 1. Move root "peer" globs → template.match (preserve order)
    template := result.GetOrCreate("template")
    for _, peerGlob := range result.RemoveAllOrdered("peer") {
        key := peerGlob.Key()
        if isGlobPattern(key) {
            // Glob pattern → template.match
            template.AddOrdered("match", key, peerGlob)
        }
    }

    // 2. Rename "neighbor" → "peer" at root level
    for _, neighbor := range result.RemoveAllOrdered("neighbor") {
        result.AddOrdered("peer", neighbor.Key(), neighbor)
    }

    // 3. Rename template.neighbor → template.group
    if tmpl := result.GetContainer("template"); tmpl != nil {
        for _, named := range tmpl.RemoveAllOrdered("neighbor") {
            tmpl.AddOrdered("group", named.Key(), named)
        }
    }

    return result, nil
}

// isGlobPattern returns true if s contains wildcard characters
func isGlobPattern(s string) bool {
    return strings.Contains(s, "*")
}
```

---

## Backup Strategy

### File Naming

```
{original-name}.{timestamp}.bak

Examples:
  exabgp.conf.20251221-143022.bak
  bgp.conf.20251221-143022.bak
```

### Backup Location

Priority order:
1. Same directory as original config (default)
2. `--backup-dir` flag if specified
3. `$XDG_STATE_HOME/zebgp/backups/` fallback

### Retention

Default: Keep last 5 backups per config file (configurable via `--backup-keep`).

---

## Implementation Plan

### Phase 1: Core Infrastructure

| # | Task | Files |
|---|------|-------|
| 1.1 | Create migration package | `pkg/config/migration/` |
| 1.2 | Define ConfigVersion type and constants | `migration/version.go` |
| 1.3 | Define Migration, MigrationResult, Change types | `migration/types.go` |
| 1.4 | Implement Tree.Clone() for safe mutations | `pkg/config/tree.go` |
| 1.5 | Implement Tree helpers (FindAll, Has, Remove, GetOrCreate, Set) | `pkg/config/tree.go` |
| 1.6 | Tests for Tree mutation methods | `pkg/config/tree_test.go` |

### Phase 2: Version Detection

| # | Task | Files |
|---|------|-------|
| 2.1 | Implement DetectVersion() with heuristics | `migration/detect.go` |
| 2.2 | Implement FindDeprecated() | `migration/detect.go` |
| 2.3 | Define v1 deprecated field patterns | `migration/v1_patterns.go` |
| 2.4 | Tests for version detection | `migration/detect_test.go` |

### Phase 3: Migration Engine

| # | Task | Files |
|---|------|-------|
| 3.1 | Implement Registry and MigrateTo() | `migration/registry.go` |
| 3.2 | Implement v1→v2 migration | `migration/v1_to_v2.go` |
| 3.3 | Tests for v1→v2 migration | `migration/v1_to_v2_test.go` |
| 3.4 | Implement v2→v3 migration (see `neighbor-to-peer-rename.md`) | `migration/v2_to_v3.go` |
| 3.5 | Tests for v2→v3 migration | `migration/v2_to_v3_test.go` |
| 3.6 | Implement DeprecatedConfigError | `migration/errors.go` |

### Phase 4: Loader Integration

| # | Task | Files |
|---|------|-------|
| 4.1 | Add ParseLenient() for deprecated field tolerance | `pkg/config/parser.go` |
| 4.2 | Add LoadOptions type | `pkg/config/loader.go` |
| 4.3 | Modify LoadReactorFile() with migration support | `pkg/config/loader.go` |
| 4.4 | Add backup creation logic | `pkg/config/backup.go` |
| 4.5 | Integration tests | `pkg/config/loader_test.go` |

### Phase 5: CLI Commands

All config-related commands live under `zebgp config`:

| # | Task | Files | Status |
|---|------|-------|--------|
| 5.1 | Add `--auto-upgrade` flag to main command | `cmd/zebgp/main.go` | |
| 5.2 | Add `zebgp config` parent command | `cmd/zebgp/config.go` | ✅ |
| 5.3 | Add `zebgp config migrate` subcommand | `cmd/zebgp/config_migrate.go` | ✅ |
| 5.4 | Add `zebgp config check` subcommand | `cmd/zebgp/config_check.go` | ✅ |
| 5.5 | Add `zebgp config dump` subcommand | `cmd/zebgp/configdump.go` | ✅ |
| 5.6 | Add `zebgp config fmt` subcommand | `cmd/zebgp/config_fmt.go` | ✅ |
| 5.7 | Add `--dry-run`, `-o`, `--in-place` flags | `cmd/zebgp/config_*.go` | ✅ |
| 5.8 | Detect unsupported features (multi-session, operational) | `cmd/zebgp/config_check.go` | ✅ |
| 5.9 | Show warnings in CLI output | `cmd/zebgp/config_*.go` | ✅ |

### Phase 5b: Config Formatter (formatting only, no migrations)

| # | Task | Files |
|---|------|-------|
| 5b.1 | Implement Tree serializer with formatting rules | `pkg/config/serialize.go` |
| 5b.2 | Add indentation normalization (4 spaces) | `pkg/config/serialize.go` |
| 5b.3 | Add block ordering (alphabetical) | `pkg/config/serialize.go` |
| 5b.4 | Diff output for `--dry-run` | `cmd/zebgp/config_fmt.go` |
| 5b.5 | Comment preservation (best-effort) | `pkg/config/serialize.go` |
| 5b.6 | Reject deprecated configs (require upgrade first) | `cmd/zebgp/config_fmt.go` |
| 5b.7 | Tests for formatter | `pkg/config/serialize_test.go` |

### Phase 6: Documentation

| # | Task | Files | Status |
|---|------|-------|--------|
| 6.1 | Document migration system | `docs/config-migration.md` | ✅ |
| 6.2 | Document deprecated options | `docs/deprecated-options.md` | ✅ |

---

## Testing Strategy

### Unit Tests

- Each migration function tested in isolation
- Tree manipulation methods tested
- Version detection tested against sample configs
- Heuristic detection edge cases

### Integration Tests

- Full load cycle with deprecated config → error
- Full load cycle with `--auto-upgrade` → success
- Backup creation verified
- Migrated config re-parseable
- Idempotency: migrate twice = same result

### Test Fixtures

```
testdata/configs/migration/
├── v1/
│   ├── basic.conf           # Simple v1 config
│   ├── all-rib-opts.conf    # All RIB options set
│   └── multiple-neighbors.conf
└── v2/
    └── current.conf         # Already current format
```

---

## Decisions Made

| Question | Decision |
|----------|----------|
| Explicit version field? | **No** - use heuristic detection |
| Default behavior? | **Refuse to start** - require explicit `zebgp config upgrade` |
| Optimistic loading? | **Opt-in** via `--auto-upgrade` flag |
| Write back migrated config? | **Yes** via `zebgp config upgrade` command |
| First version baseline? | **ExaBGP main (2025-12)** = v1 |

---

## Dependencies

- Requires Tree mutation methods (Clone, Remove, Set, GetOrCreate)
- Requires lenient parser mode (accept deprecated fields without error)
- No external dependencies

---

## Success Criteria

1. ✅ `zebgp --config old.conf` fails with clear upgrade instructions
2. ✅ `zebgp config upgrade old.conf` upgrades and creates backup
3. ✅ `zebgp config upgrade --dry-run old.conf` shows changes without modifying
4. ✅ `zebgp --config old.conf --auto-upgrade` works with warning
5. ✅ `zebgp config check old.conf` shows version and deprecated fields
6. ✅ All migrations are idempotent
7. ✅ Zero data loss through migration

---

## Unsupported Features

These ExaBGP features are detected and warned about, but not implemented in ZeBGP:

| Feature | Location | Reason |
|---------|----------|--------|
| `multi-session` | `capability { multi-session; }` | ExaBGP-specific, non-standard BGP extension |
| `operational` | `capability { operational; }` | ExaBGP-specific messaging capability |
| `operational` block | `peer { operational { ... } }` | ExaBGP-specific operational messages (ASM, ADM, queries) |

**CLI Behavior:**
- `zebgp config check` shows warnings for unsupported features
- `zebgp config migrate` shows warnings after migration
- Features are parsed but ignored at runtime

---

## Future Migrations

When adding new migrations:

1. Define new `VersionN` constant
2. Update `VersionCurrent`
3. Add detection heuristic to `DetectVersion()`
4. Add deprecated patterns to `FindDeprecated()`
5. Implement `migrateVN-1ToVN` migration
6. Register in `NewRegistry()`
7. Add test fixtures and tests

---

## References

- ExaBGP config: `../src/exabgp/configuration/neighbor/__init__.py`
- ZeBGP schema: `pkg/config/schema.go`
- Database migration patterns: Rails ActiveRecord, Alembic, Flyway
