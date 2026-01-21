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
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] Architecture docs updated with new syntax

## Implementation Summary

<!-- Fill after implementation, before moving to done/ -->

### What Was Implemented
- (pending)

### Bugs Found/Fixed
- (pending)

### Design Insights
- (pending)

### Deviations from Plan
- (pending)
