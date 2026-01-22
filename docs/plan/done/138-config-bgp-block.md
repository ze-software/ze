# Spec: config-bgp-block

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/config/syntax.md` - current config syntax
4. `internal/config/bgp.go:309-345` - `BGPSchema()` top-level schema
5. `internal/config/bgp.go:336-339` - template schema (`group`, `match`)
6. `internal/config/loader.go` - config loading logic
7. `internal/config/schema.go` - schema types (`ContainerNode`, `ListNode`)

## Task

Restructure configuration to wrap all BGP-specific config in `bgp {}` block, enabling future multi-protocol support. Update template syntax to use `peer <pattern>` with optional `inherit-name`.

**No backward compatibility required** - no users, no releases.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - current config syntax
- [ ] `docs/architecture/config/environment.md` - environment block structure

### Source Files
- [ ] `internal/config/bgp.go:309-345` - `BGPSchema()` current top-level schema
- [ ] `internal/config/bgp.go:336-339` - current template schema (`group`, `match`)
- [ ] `internal/config/loader.go` - config loading and template application
- [ ] `internal/config/schema.go` - schema types (`ContainerNode`, `ListNode`, etc.)

**Key insights:**
- Current config has BGP elements at top level (`router-id`, `local-as`, `peer`)
- Templates use `group` and `match` syntax
- Need to wrap BGP config in `bgp {}` block

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseBGPBlock` | `internal/config/loader_test.go` | Parses `bgp {}` wrapper | |
| `TestParseTemplateNewSyntax` | `internal/config/loader_test.go` | Parses `template { bgp { peer * { } } }` | |
| `TestInheritNameKeyword` | `internal/config/loader_test.go` | Parses `inherit-name` in templates | |
| `TestInheritPatternValidation` | `internal/config/loader_test.go` | Validates inherit against pattern | |

### Boundary Tests
N/A - No new numeric fields introduced. Pattern matching uses string globs.

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `bgp-block-simple` | `test/parse/bgp-block-simple.ci` | Basic `bgp {}` parsing | |
| `template-new-syntax` | `test/parse/template-new-syntax.ci` | New template syntax | |
| `inherit-pattern-match` | `test/parse/inherit-pattern-match.ci` | Inherit with pattern validation | |

### Negative Tests (Error Cases)
| Test | File | Error Case | Status |
|------|------|------------|--------|
| `TestInheritNonExistent` | `internal/config/loader_test.go` | `inherit foo` where `foo` not defined | |
| `TestInheritPatternMismatch` | `internal/config/loader_test.go` | `inherit foo` where `foo` is `peer 10.*` but peer is `192.168.1.1` | |
| `TestTopLevelBGPElementsRejected` | `internal/config/loader_test.go` | `router-id` at top level (should require `bgp {}`) | |

## Files to Modify
- `internal/config/bgp.go` - Add `bgp {}` block to schema, update template schema
- `internal/config/loader.go` - Parse new structure, apply templates with pattern matching
- `docs/architecture/config/syntax.md` - Update documentation
- `etc/ze/bgp/*.conf` - Update all example configs (~100 files)
- `test/parse/*.ci` - Update parsing tests
- `test/encode/*.ci` - Update encoding tests
- `test/plugin/*.ci` - Update plugin tests

## Files to Create
- `internal/config/template.go` - Template application with `peer <pattern>` matching
- `internal/config/validate.go` - Validate `inherit` against pattern constraints
- `test/parse/bgp-block-simple.ci` - Basic bgp block test
- `test/parse/template-new-syntax.ci` - New template syntax test
- `test/parse/inherit-pattern-match.ci` - Inherit validation test
- `test/parse/inherit-error-cases.ci` - Negative tests for inherit errors

## New Config Structure

### Before (Current)
```
environment { ... }
plugin { ... }
template {
    group backbone { ... }
    match "10.*" { ... }
}
router-id 10.0.0.1;
local-as 65000;
peer 10.1.1.1 { ... }
```

### After (New)
```
environment { ... }
plugin { ... }
template {
    bgp {
        peer * {
            inherit-name backbone;
            hold-time 90;
        }
        peer 10.* {
            local-as 65000;
        }
    }
}
bgp {
    listen 0.0.0.0:179;
    router-id 10.0.0.1;
    local-as 65000;
    peer 10.1.1.1 {
        inherit backbone;
        peer-as 65001;
    }
}
```

## Template Rules

| Pattern | `inherit-name` | Behavior |
|---------|----------------|----------|
| `peer *` | Yes | Named template, available for any peer to `inherit` |
| `peer *` | No | Auto-applied to ALL peers |
| `peer 10.*` | Yes | Named template, only peers matching `10.*` can `inherit` |
| `peer 10.*` | No | Auto-applied to peers matching `10.*` |

## Implementation Steps

1. **Write unit tests** - Create unit tests BEFORE implementation (strict TDD)
2. **Run tests** - Verify FAIL (paste output)
3. **Implement config schema** - Add `bgp {}` block, new template syntax
4. **Run tests** - Verify PASS (paste output)
5. **Update all config files** - Wrap BGP config in `bgp {}`
6. **Update all test files** - Update `.ci` files with new syntax
7. **Functional tests** - Create functional tests AFTER feature works
8. **Verify all** - `make lint && make test && make functional` (paste output)

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (output below)
- [x] Implementation complete
- [x] Tests PASS (output below)

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes (all packages)
- [x] `make functional` passes (96 tests: 42 encoding + 24 plugin + 12 parsing + 18 decoding)

### Documentation
- [x] Required docs read
- [x] Architecture docs updated with new syntax (`docs/architecture/config/syntax.md`)

## Implementation Summary

### What Was Implemented
- Added `bgp {}` container block to config schema
- Updated `template` to use `template { bgp { peer <pattern> { ... } } }` syntax
- Added `inherit-name` field for naming templates in `template.bgp.peer`
- `inherit` in `bgp.peer` references templates by name
- Pattern in `template.bgp.peer` limits which peers can inherit
- Migration framework updated with `wrap-bgp-block` and `template->new-format` transformations
- Migrated 169 files total:
  - 91 config files (`etc/ze/bgp/*.conf`)
  - 42 encoding tests (`test/encode/*.ci`)
  - 24 plugin tests (`test/plugin/*.ci`)
  - 12 parsing tests (`test/parse/*.ci`)

### Bugs Found/Fixed
- ExaBGP test input files (`test/exabgp/*/input.conf`) were accidentally migrated; restored to original ExaBGP format

### Design Insights
- `inherit-name <name>` in `template.bgp.peer <pattern>` defines a named template
- `inherit <name>` in `bgp.peer` uses that template
- Pattern `*` allows any peer to inherit; specific patterns (e.g., `10.0.0.*`) restrict inheritance
- Without `inherit-name`, pattern-based templates auto-apply to matching peers

### Deviations from Plan
- Did not create separate `internal/config/template.go` or `internal/config/validate.go` - functionality integrated into existing files
- Did not create new functional test files - existing tests updated instead
- Template design clarified: `peer <pattern>` with optional `inherit-name`, not separate `group`/`match` blocks
