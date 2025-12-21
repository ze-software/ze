# Two-Level Route Grouping for UPDATE Generation

**Status:** Proposed
**Created:** 2025-12-21

---

## Problem Statement

Current `GroupByAttributes()` groups routes by:
- Family + NextHop + Attributes (excluding AS_PATH)

Routes with different `route.asPath` values end up in the same group. When building UPDATEs, `buildASPath()` looks for AS_PATH in `group.Attributes` (won't find it), then creates a fresh AS_PATH. This loses the original AS_PATH stored in `route.asPath`.

**RFC 4271 Section 4.3:** Path attributes apply to ALL NLRIs in an UPDATE. Routes with different AS_PATHs cannot share an UPDATE.

---

## Design Goal

Two-level grouping that:
1. **Maximizes memory sharing** - Routes share `[]Attribute` slices at level 1
2. **Produces valid UPDATEs** - Each level-2 group has one AS_PATH
3. **Maximizes NLRI packing** - Routes with same attrs AND same AS_PATH grouped

```
Level 1: AttributeGroup (Family + NextHop + Attributes)
    └── Level 2: ASPathGroup (AS_PATH)
            └── Leaf: Routes/NLRIs
```

---

## Data Structures

### New Types (grouping.go)

```go
// AttributeGroup represents routes sharing the same non-AS_PATH attributes.
// This is level 1 of the two-level grouping hierarchy.
type AttributeGroup struct {
    Key        string                  // Family + NextHop + Attributes hash
    Family     nlri.Family
    NextHop    []byte
    Attributes []attribute.Attribute   // Shared reference (memory efficient)
    ByASPath   []ASPathGroup           // Level 2 sub-groups
}

// ASPathGroup represents routes sharing the same AS_PATH within an AttributeGroup.
// Each ASPathGroup produces exactly one UPDATE message.
type ASPathGroup struct {
    ASPath *attribute.ASPath          // nil = no AS_PATH (locally originated)
    Routes []*Route                   // Routes/NLRIs with this AS_PATH
}
```

### Existing Types (unchanged)

```go
// RouteGroup - keep for backward compatibility or simple use cases
type RouteGroup struct { ... }
```

---

## Implementation Phases

### Phase 1: Add Data Structures

**File:** `pkg/rib/grouping.go`

1. Add `AttributeGroup` struct
2. Add `ASPathGroup` struct
3. Add helper: `hashASPath(*attribute.ASPath) string` for grouping key

**Tests:** `pkg/rib/grouping_test.go`
- Test struct creation
- Test ASPath hashing (nil, empty, with segments)

---

### Phase 2: Implement Two-Level Grouping

**File:** `pkg/rib/grouping.go`

```go
// GroupByAttributesTwoLevel groups routes first by attributes, then by AS_PATH.
// Returns attribute groups, each containing AS_PATH sub-groups.
// Each ASPathGroup can be sent as a single UPDATE message.
func GroupByAttributesTwoLevel(routes []*Route) []AttributeGroup
```

**Algorithm:**
1. First pass: Group by `buildAttributeKey()` (existing, excludes AS_PATH)
2. Second pass: Within each group, sub-group by `route.ASPath()`
3. Sort both levels for deterministic ordering

**Tests:**
- Routes with same attrs, same AS_PATH → 1 AttributeGroup, 1 ASPathGroup
- Routes with same attrs, different AS_PATH → 1 AttributeGroup, N ASPathGroups
- Routes with different attrs → N AttributeGroups
- Mixed nil/non-nil AS_PATH → separate ASPathGroups
- Empty input → empty output

---

### Phase 3: Update CommitService

**File:** `pkg/rib/commit.go`

1. Add `buildGroupedUpdateTwoLevel(attrGroup, aspGroup)` method
2. Modify `Commit()` to use two-level iteration when `groupUpdates=true`
3. Pass `aspGroup.ASPath` to AS_PATH building logic

```go
func (c *CommitService) Commit(routes []*Route, opts CommitOptions) {
    if c.groupUpdates {
        attrGroups := GroupByAttributesTwoLevel(routes)
        for _, attrGroup := range attrGroups {
            for _, aspGroup := range attrGroup.ByASPath {
                update := c.buildGroupedUpdateTwoLevel(&attrGroup, &aspGroup)
                c.sender.SendUpdate(update)
            }
        }
    }
}
```

**Tests:**
- Verify correct number of UPDATEs generated
- Verify each UPDATE has correct AS_PATH
- Verify NLRIs correctly distributed

---

### Phase 4: Update AS_PATH Building

**File:** `pkg/rib/commit.go`

1. Add `packAttributesWithASPath(attrs, asPath, nextHop, family, nlri)` or modify existing
2. Use explicit AS_PATH from `ASPathGroup`, not searched from attributes
3. Apply eBGP/iBGP rules to the explicit AS_PATH

```go
func (c *CommitService) buildASPathFromExplicit(asPath *attribute.ASPath) []byte {
    if c.isIBGP() {
        // Preserve as-is (or empty if nil)
        return packASPath(asPath)
    }
    // eBGP: prepend local AS
    return packASPathWithPrepend(asPath, c.negotiated.LocalAS)
}
```

**Tests:**
- eBGP with existing AS_PATH → local AS prepended
- eBGP with nil AS_PATH → [LocalAS]
- iBGP with existing AS_PATH → preserved unchanged
- iBGP with nil AS_PATH → empty

---

### Phase 5: Wire Format Verification

**File:** `pkg/rib/commit_wire_test.go`

Add tests that verify exact wire format:
1. Two routes, same attrs, different AS_PATHs → 2 UPDATEs with correct AS_PATHs
2. Two routes, same attrs, same AS_PATH → 1 UPDATE with both NLRIs
3. Verify AS_PATH attribute bytes in packed UPDATE

---

### Phase 6: Integration & Cleanup

1. Update any code that uses old `GroupByAttributes` if needed
2. Consider deprecating `RouteGroup` or keeping for simple cases
3. Update `CLAUDE_CONTINUATION.md`

---

## Edge Cases

| Case | Behavior |
|------|----------|
| All routes have nil AS_PATH | Single ASPathGroup with nil → fresh AS_PATH created |
| Mixed nil and non-nil | Separate ASPathGroups |
| AS_PATH in both `route.asPath` and `route.attributes` | `route.asPath` takes precedence |
| Empty routes slice | Return empty slice |
| Single route | 1 AttributeGroup, 1 ASPathGroup |

---

## Example

**Input routes:**
```
Route A: 10.0.0.0/8,  NH=1.1.1.1, attrs=[ORIGIN=IGP], asPath=[65001]
Route B: 10.0.1.0/24, NH=1.1.1.1, attrs=[ORIGIN=IGP], asPath=[65001]
Route C: 10.0.2.0/24, NH=1.1.1.1, attrs=[ORIGIN=IGP], asPath=[65001,65002]
Route D: 10.0.3.0/24, NH=1.1.1.1, attrs=[ORIGIN=IGP, MED=100], asPath=[65001]
```

**Two-level grouping:**
```
AttributeGroup 1: Key=[IPv4-unicast, 1.1.1.1, ORIGIN=IGP]
    ├── ASPathGroup: [65001]
    │       ├── Route A (10.0.0.0/8)
    │       └── Route B (10.0.1.0/24)
    │
    └── ASPathGroup: [65001, 65002]
            └── Route C (10.0.2.0/24)

AttributeGroup 2: Key=[IPv4-unicast, 1.1.1.1, ORIGIN=IGP, MED=100]
    └── ASPathGroup: [65001]
            └── Route D (10.0.3.0/24)
```

**UPDATEs generated:** 3
1. `attrs=[ORIGIN, AS_PATH=[65001]], NLRIs=[10.0.0.0/8, 10.0.1.0/24]`
2. `attrs=[ORIGIN, AS_PATH=[65001,65002]], NLRIs=[10.0.2.0/24]`
3. `attrs=[ORIGIN, MED=100, AS_PATH=[65001]], NLRIs=[10.0.3.0/24]`

---

## Memory Model

```
┌─────────────────────────────────────────────────────────────────┐
│ AttributeGroup 1                                                │
│   Attributes: ──────────────────┐                               │
│                                 ▼                               │
│                         [ORIGIN=IGP] (shared slice)             │
│                                 ▲                               │
│   ASPathGroup 1:                │                               │
│     ASPath: [65001]             │                               │
│     Routes: [A, B] ─────────────┘                               │
│                                                                 │
│   ASPathGroup 2:                                                │
│     ASPath: [65001, 65002]                                      │
│     Routes: [C] ────────────────┘                               │
└─────────────────────────────────────────────────────────────────┘
```

Routes A, B, C all reference the same `[ORIGIN=IGP]` slice.

---

## Test Plan

### Unit Tests (TDD - write first)

| Test | Validates |
|------|-----------|
| `TestGroupByAttributesTwoLevel_SameAttrsSameASPath` | Routes grouped together |
| `TestGroupByAttributesTwoLevel_SameAttrsDiffASPath` | Separate ASPathGroups |
| `TestGroupByAttributesTwoLevel_DiffAttrs` | Separate AttributeGroups |
| `TestGroupByAttributesTwoLevel_NilASPath` | Nil AS_PATH handled |
| `TestGroupByAttributesTwoLevel_MixedNilASPath` | Nil and non-nil separated |
| `TestGroupByAttributesTwoLevel_Empty` | Empty input → empty output |
| `TestGroupByAttributesTwoLevel_Deterministic` | Same input → same order |

### Integration Tests

| Test | Validates |
|------|-----------|
| `TestCommitService_TwoLevelGrouping_CorrectUpdateCount` | Right number of UPDATEs |
| `TestCommitService_TwoLevelGrouping_PreservesASPath` | AS_PATH in UPDATE matches route |
| `TestCommitService_TwoLevelGrouping_eBGPPrepends` | Local AS prepended for eBGP |
| `TestCommitService_TwoLevelGrouping_iBGPPreserves` | AS_PATH unchanged for iBGP |

### Wire Format Tests

| Test | Validates |
|------|-----------|
| `TestCommitService_TwoLevel_WireFormat` | Exact bytes of generated UPDATEs |

---

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Performance regression from two-pass grouping | Benchmark before/after; single-pass possible if needed |
| Breaking existing tests | Run full test suite after each phase |
| Memory overhead from new structures | Measure; structures are small (pointers) |

---

## Success Criteria

1. All existing tests pass
2. New tests for two-level grouping pass
3. Routes with different `route.asPath` produce separate UPDATEs
4. Routes with same `route.asPath` share UPDATEs
5. Wire format tests verify correct AS_PATH encoding
6. `make test && make lint` clean

---

## Files Modified

| File | Changes |
|------|---------|
| `pkg/rib/grouping.go` | Add `AttributeGroup`, `ASPathGroup`, `GroupByAttributesTwoLevel` |
| `pkg/rib/grouping_test.go` | Add two-level grouping tests |
| `pkg/rib/commit.go` | Update `Commit()` to use two-level, add `buildGroupedUpdateTwoLevel` |
| `pkg/rib/commit_test.go` | Add integration tests |
| `pkg/rib/commit_wire_test.go` | Add wire format tests |
| `plan/CLAUDE_CONTINUATION.md` | Update status |
