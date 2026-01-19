# Spec: PathAttributes Removal

## Task

Replace `plugin.PathAttributes` struct with `attribute.Builder` for wire-first attribute construction. This completes the buffer-first migration for INPUT paths (building routes from user commands).

## Required Reading

### Architecture Docs
- [x] `docs/architecture/buffer-architecture.md` - Target architecture
- [x] `docs/architecture/update-building.md` - Wire format construction

### Source Code
- [x] `internal/bgp/attribute/builder.go` - New Builder (already implemented)
- [x] `internal/plugin/types.go` - PathAttributes (to be replaced)
- [x] `internal/plugin/route.go` - Route parsing
- [x] `internal/plugin/update_text.go` - Text command parsing

**Key insights:**
- `PathAttributes` was intermediate representation: text → PathAttributes → wire bytes
- With `attribute.Builder`: text → Builder → wire bytes (direct)
- MUP routes need `spec.ExtCommunity` set directly, not just in builder

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestBuilderFromText` | `internal/bgp/attribute/builder_text_test.go` | Parse text → Builder |
| `TestRouteSpecWithBuilder` | `internal/plugin/route_test.go` | RouteSpec uses Builder |
| `TestParseOrigin` | `internal/plugin/route_test.go` | origin igp/egp/incomplete |
| `TestParseASPath` | `internal/plugin/route_test.go` | as-path [65001 65002] |
| `TestParseCommunity` | `internal/plugin/route_test.go` | community 65000:100 |
| `TestParseLargeCommunity` | `internal/plugin/route_test.go` | large-community 65000:1:2 |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| `mup4` | `test/data/plugin/mup4.ci` | MUP IPv4 with extended-community |
| `mup6` | `test/data/plugin/mup6.ci` | MUP IPv6 with extended-community |

## Files to Modify
- `internal/bgp/attribute/builder.go` - Add text parse methods
- `internal/plugin/types.go` - Replace PathAttributes with Builder
- `internal/plugin/route.go` - Update parsing to use Builder, fix MUP extended-community handling
- `internal/plugin/update_text.go` - Update parsing to use Builder
- `internal/reactor/reactor.go` - Remove buildBatchAttributes

## Implementation Steps
1. **Write tests** - Builder text parsing tests
2. **Run tests** - Verify FAIL
3. **Implement** - Phase 1-5 as documented below
4. **Run tests** - Verify PASS
5. **Verify all** - `make lint && make test && make functional`
6. **Bug fix** - MUP extended-community: handle BEFORE parseCommonAttributeBuilder

### Implementation Phases (completed)
- Phase 1: Add Builder text parsing methods
- Phase 2: Create RouteBuilder type
- Phase 3: Migrate parseCommonAttribute to parseCommonAttributeBuilder
- Phase 4: Migrate route spec types to use Builder
- Phase 5: Remove PathAttributes struct

### Bug Fix (2025-01-11)
`ParseMUPArgs` had dual handling for `extended-community`:
1. `parseCommonAttributeBuilder` parsed it first → stored in builder, returned consumed > 0
2. Switch case `extended-community` → never reached because of (1)

**Fix:** Handle MUP-specific keywords BEFORE `parseCommonAttributeBuilder` so `spec.ExtCommunity` is set.

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (output below)
- [x] Implementation complete
- [x] Tests PASS (output below)

### Verification
- [x] `make lint` passes (26 linters including `govet`, `staticcheck`, `gosec`, `gocritic`)
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] Required docs read
- [x] `docs/` updated if schema changed

### Completion
- [x] Spec moved to `docs/plan/done/NNN-pathattributes-removal.md`
