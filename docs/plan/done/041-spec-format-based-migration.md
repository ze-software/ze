# Spec: Format-Based Migration

## MANDATORY READING (BEFORE IMPLEMENTATION)

```
┌─────────────────────────────────────────────────────────────────┐
│  STOP. Read these files FIRST before ANY implementation:        │
│                                                                 │
│  1. .claude/ESSENTIAL_PROTOCOLS.md - Session rules, TDD         │
│  2. .claude/INDEX.md - Find what docs to load                   │
│  3. docs/plan/CLAUDE_CONTINUATION.md - Current state                 │
│  4. THIS SPEC FILE - Design requirements                        │
│  5. internal/config/migration/*.go - Current implementation          │
│  6. .claude/zebgp/config/SYNTAX.md - Config design docs         │
│                                                                 │
│  DO NOT PROCEED until all are read and understood.              │
│                                                                 │
│  ON COMPLETION: Update design docs listed in Documentation      │
│  Impact section to match any design changes made.               │
└─────────────────────────────────────────────────────────────────┘
```

## Documentation Impact

**MUST update these docs if design changes:**
- [ ] `.claude/zebgp/config/SYNTAX.md` - if migration syntax/behavior changes
- [ ] `docs/plan/CLAUDE_CONTINUATION.md` - always update after implementation
- [ ] `cmd/zebgp/README.md` - if CLI flags change (--dry-run, --list)

## Task

Refactor migration system from version-based (v2→v3) to format-based transformations with:
- Semantic naming (not arbitrary version numbers)
- Visibility into what was applied
- Dry-run to preview and validate
- Atomic: only commit if all transformations succeed

## Current State

- `make test`: PASS
- `make lint`: PASS (1 pre-existing issue in static_test.go)
- Last verified: 2025-12-30

## Design

### Core Principles

1. **Format-based, not version-based**: Each transformation has a semantic name describing what it does
2. **Atomic**: Changes are in-memory until ALL succeed; on any failure, original unchanged
3. **Detect-then-apply**: Each transformation has independent detection; skips if not needed
4. **Order matters**: Transformations run in dependency order (structural first, then content)
5. **Don't split tightly-coupled operations**: `migrateAPIBlocks` stays as one unit

### Types

```go
// Transformation defines a single migration step.
type Transformation struct {
    Name        string                                    // Semantic name for display
    Description string                                    // Human-readable explanation
    Detect      func(*config.Tree) bool                   // Does this issue exist?
    Apply       func(*config.Tree) (*config.Tree, error)  // Fix it
}

// MigrateResult holds the outcome of migration.
type MigrateResult struct {
    Tree    *config.Tree  // Transformed tree (only set on full success)
    Applied []string      // Transformations that ran
    Skipped []string      // Transformations not needed
}

// DryRunResult shows what would happen without applying changes.
type DryRunResult struct {
    AlreadyDone  []string  // Detect returned false - already migrated
    WouldApply   []string  // Detect returned true - would be applied
    WouldSucceed bool      // All transformations would succeed
    FailedAt     string    // Which transformation would fail (if any)
    Error        error     // Why it would fail
}
```

### Transformation Registry

```go
// transformations in dependency order (unexported).
// Phase 1: Structural renames (must run first - create peer/group blocks)
// Phase 2: Content transforms (operate on blocks created by phase 1)
var transformations = []Transformation{
    // Phase 1: Structural renames
    {
        Name:        "neighbor->peer",
        Description: "Rename 'neighbor' blocks to 'peer'",
        Detect:      hasNeighborAtRoot,
        Apply:       migrateNeighborToPeer,
    },
    {
        Name:        "peer-glob->template.match",
        Description: "Move glob patterns (10.0.0.0/8) to template.match",
        Detect:      hasPeerGlobPattern,
        Apply:       migratePeerGlobToMatch,
    },
    {
        Name:        "template.neighbor->group",
        Description: "Rename template.neighbor to template.group",
        Detect:      hasTemplateNeighbor,
        Apply:       migrateTemplateNeighborToGroup,
    },
    // Phase 2: Content transforms
    {
        Name:        "static->announce",
        Description: "Convert 'static' route blocks to 'announce'",
        Detect:      hasStaticBlocks,
        Apply:       extractStaticRoutes,
    },
    {
        Name:        "api->new-format",
        Description: "Convert old api syntax (processes, format flags) to named blocks",
        Detect:      hasOldAPIBlocks,
        Apply:       migrateAPIBlocks,
    },
}
```

### API

```go
// Migrate applies all needed transformations.
// Changes are in-memory until ALL succeed; on failure, original unchanged.
func Migrate(tree *config.Tree) (*MigrateResult, error)

// DryRun analyzes what would happen without applying changes.
// Validates transformations would succeed by running on a clone.
func DryRun(tree *config.Tree) (*DryRunResult, error)

// NeedsMigration returns true if any transformation would apply.
func NeedsMigration(tree *config.Tree) bool
```

### Implementation

```go
func Migrate(tree *config.Tree) (*MigrateResult, error) {
    if tree == nil {
        return nil, ErrNilTree
    }

    working := tree.Clone()  // Work on clone - original unchanged until success
    result := &MigrateResult{}

    for _, t := range transformations {
        if t.Detect(working) {
            migrated, err := t.Apply(working)
            if err != nil {
                // Failure: return error, original tree unchanged
                return nil, fmt.Errorf("%s: %w", t.Name, err)
            }
            working = migrated
            result.Applied = append(result.Applied, t.Name)
        } else {
            result.Skipped = append(result.Skipped, t.Name)
        }
    }

    // All succeeded: return transformed tree
    result.Tree = working
    return result, nil
}

func DryRun(tree *config.Tree) (*DryRunResult, error) {
    if tree == nil {
        return nil, ErrNilTree
    }

    result := &DryRunResult{WouldSucceed: true}
    working := tree.Clone()

    for _, t := range transformations {
        if t.Detect(working) {
            result.WouldApply = append(result.WouldApply, t.Name)
            // Run on clone to validate it would succeed
            migrated, err := t.Apply(working)
            if err != nil {
                result.WouldSucceed = false
                result.FailedAt = t.Name
                result.Error = err
                return result, nil  // Return analysis, not error
            }
            working = migrated
        } else {
            result.AlreadyDone = append(result.AlreadyDone, t.Name)
        }
    }

    return result, nil
}

func NeedsMigration(tree *config.Tree) bool {
    if tree == nil {
        return false
    }
    for _, t := range transformations {
        if t.Detect(tree) {
            return true
        }
    }
    return false
}
```

### CLI Output

**Migrate (replace mode):**
```
$ zebgp config migrate --replace legacy.conf

Applying transformations:
  ✅ neighbor->peer
  ⏭️  peer-glob->template.match (not needed)
  ⏭️  template.neighbor->group (not needed)
  ✅ static->announce
  ✅ api->new-format

3 transformations applied, 2 skipped.
Config updated: legacy.conf
```

**Migrate (stdout mode - existing behavior):**
```
$ zebgp config migrate legacy.conf

# Progress to stderr:
Applying transformations:
  ✅ neighbor->peer
  ✅ api->new-format

2 transformations applied.

# Config to stdout:
process foo { ... }
peer 10.0.0.1 { ... }
```

**Dry-run (partially migrated):**
```
$ zebgp config migrate --dry-run legacy.conf

Transformation analysis:
  ✅ neighbor->peer (done)
  ✅ peer-glob->template.match (done)
  ✅ template.neighbor->group (done)
  ⏳ static->announce (pending)
  ⏳ api->new-format (pending)

Result: 2 transformations would apply. All would succeed.
```

**Dry-run (would fail):**
```
$ zebgp config migrate --dry-run broken.conf

Transformation analysis:
  ✅ neighbor->peer (done)
  ⏳ static->announce (pending)
  ❌ api->new-format (would fail)

Error: api->new-format: duplicate process 'foo'

Result: Transformation would fail.
```

**Dry-run (nothing to do):**
```
$ zebgp config migrate --dry-run modern.conf

Transformation analysis:
  ✅ neighbor->peer (done)
  ✅ peer-glob->template.match (done)
  ✅ template.neighbor->group (done)
  ✅ static->announce (done)
  ✅ api->new-format (done)

Result: No transformation needed.
```

**List transformations:**
```
$ zebgp config migrate --list

Available transformations (in order):
  neighbor->peer           Rename 'neighbor' blocks to 'peer'
  peer-glob->template.match  Move glob patterns to template.match
  template.neighbor->group   Rename template.neighbor to template.group
  static->announce         Convert 'static' route blocks to 'announce'
  api->new-format          Convert old api syntax to named blocks
```

## Implementation Phases

### Phase 1: Create migrate.go with New API

**File:** `internal/config/migration/migrate.go` (new)

1. Define `Transformation` struct
2. Define `transformations` slice (unexported)
3. Define `MigrateResult` struct
4. Define `DryRunResult` struct
5. Create `Migrate()` function (atomic - only returns Tree on success)
6. Create `DryRun()` function
7. Create `NeedsMigration()` function

**Tests:** `internal/config/migration/migrate_test.go`
- `Migrate(nil)` returns `ErrNilTree`
- Empty tree returns empty Applied/Skipped
- Single transformation populates Applied
- Skipped transformation populates Skipped
- Error returns nil result (not partial)
- Original tree unchanged on error
- `DryRun` shows AlreadyDone for migrated configs
- `DryRun` shows WouldApply for unmigrated configs
- `DryRun` captures failure info without returning error

### Phase 2: Extract Detection Functions

**File:** `internal/config/migration/detect.go`

1. Extract `hasNeighborAtRoot()` from inline check in hasV2Patterns
2. Extract `hasPeerGlobPattern()` from inline loop
3. Extract `hasTemplateNeighbor()` from inline check
4. Keep existing `hasStaticBlocks()`, `hasOldAPIBlocks()`
5. Export functions needed by migrate.go

### Phase 3: Normalize Apply Function Signatures

**File:** `internal/config/migration/v2_to_v3.go`

Ensure all apply functions have signature: `func(tree *config.Tree) (*config.Tree, error)`

1. `migrateNeighborToPeer()` - check/update signature
2. `migratePeerGlobToMatch()` - check/update signature
3. `migrateTemplateNeighborToGroup()` - check/update signature
4. `extractStaticRoutes()` - already correct
5. `migrateAPIBlocks()` - already correct

### Phase 4: Update CLI

**File:** `cmd/zebgp/config_migrate.go`

1. Add `--dry-run` flag
2. Add `--list` flag
3. Use `migration.Migrate()` for actual migration
4. Use `migration.DryRun()` for dry-run mode
5. Display transformations with emoji status
6. Show summary counts
7. Stderr for progress, stdout for config (existing behavior)

### Phase 5: Cleanup Old API

1. Update tests to use new `Migrate()` API first
2. Remove `ConfigVersion` type
3. Remove `DetectVersion()` function
4. Remove `Version2`, `Version3`, `VersionCurrent`, `VersionUnknown` constants
5. Remove `MigrateV2ToV3()` (replaced by `Migrate()`)
6. Remove `hasV2Patterns()`, `hasV3Patterns()` (replaced by individual detect functions)
7. Update all imports/references

## What NOT To Do

- ❌ Don't split `migrateAPIBlocks()` into multiple transformations (tightly coupled)
- ❌ Don't return partial result on error (atomic: all or nothing)
- ❌ Don't create complex plugin/hook system

## Verification

### Phase 1
- [ ] `Transformation` struct defined with Name, Description, Detect, Apply
- [ ] `MigrateResult` struct defined
- [ ] `DryRunResult` struct defined
- [ ] `Migrate()` returns Tree only on full success
- [ ] `Migrate()` returns nil,error on failure (original unchanged)
- [ ] `DryRun()` returns analysis even on would-fail
- [ ] `NeedsMigration()` works
- [ ] All tests pass

### Phase 2
- [ ] `hasNeighborAtRoot()` extracted and exported
- [ ] `hasPeerGlobPattern()` extracted and exported
- [ ] `hasTemplateNeighbor()` extracted and exported
- [ ] Existing detection functions still work

### Phase 3
- [ ] All apply functions return `(*config.Tree, error)`
- [ ] Existing tests still pass

### Phase 4
- [ ] `--dry-run` shows analysis
- [ ] `--list` shows available transformations
- [ ] Progress to stderr, config to stdout
- [ ] Emoji status indicators work

### Phase 5
- [ ] No `ConfigVersion` references remain
- [ ] No `DetectVersion` references remain
- [ ] No `MigrateV2ToV3` references remain
- [ ] `make test && make lint` pass

## Benefits

| Aspect | Before | After |
|--------|--------|-------|
| Naming | `Version2`, `Version3` | `neighbor->peer`, `api->new-format` |
| Visibility | None | Applied/Skipped lists |
| Preview | None | `--dry-run` with validation |
| Atomicity | Implicit | Explicit (all or nothing) |
| Extensibility | Add new version | Add to transformations array |
| Partial migration | Works | Works (detect skips done items) |

---

**Created:** 2025-12-30
**Updated:** 2025-12-30
**Status:** Complete
