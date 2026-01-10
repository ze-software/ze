# Spec: Buffer-First Migration

## Task

Migrate ZeBGP to buffer-first architecture where BGP messages are represented as byte buffers with iterators and partial parsers, eliminating duplication between wire and parsed representations.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/buffer-architecture.md` - Target architecture (MUST READ)
- [ ] `docs/architecture/encoding-context.md` - Context-dependent encoding
- [ ] `docs/architecture/update-building.md` - Wire format construction

### RFC Summaries
- [ ] `docs/rfc/rfc4271.md` - UPDATE message format
- [ ] `docs/rfc/rfc4760.md` - MP-BGP extensions
- [ ] `docs/rfc/rfc7911.md` - ADD-PATH

**Key insights:**
- Wire bytes are canonical - parsing is a view, not a transformation
- Context (ADD-PATH, ASN4) affects parsing, not storage
- Iterators enable zero-allocation traversal

## Problem Statement

Current state has 6+ layers of UPDATE/Message representations:

| Layer | Type | Issue |
|-------|------|-------|
| Wire | `message.Update` | Good - keeps raw bytes |
| Wire | `WireUpdate` | Good - adds metadata |
| Parsed | `PathAttributes` | Duplicates wire data |
| Parsed | `[]attribute.Attribute` | Allocates slice |
| JSON | `RouteUpdate` | Duplicates for serialization |
| Plugin | `rr.UpdateInfo` | Yet another representation |

**Problems:**
- Data duplicated in wire + parsed forms
- Slice allocations for lists (NLRI, AS-PATH, communities)
- Multiple representations must stay in sync
- Different code paths for wire vs API

## 🧪 TDD Test Plan

### Phase 1: Core Iterators

#### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestNLRIIterator` | `pkg/bgp/nlri/iterator_test.go` | Iterates prefixes without allocation |
| `TestNLRIIteratorAddPath` | `pkg/bgp/nlri/iterator_test.go` | Handles ADD-PATH path-id |
| `TestNLRIIteratorEmpty` | `pkg/bgp/nlri/iterator_test.go` | Empty buffer returns no items |
| `TestAttrIterator` | `pkg/bgp/attribute/iterator_test.go` | Iterates attributes |
| `TestAttrIteratorFind` | `pkg/bgp/attribute/iterator_test.go` | Finds specific type code |
| `TestASPathIterator` | `pkg/bgp/attribute/aspath_iter_test.go` | Iterates segments |
| `TestASPathIteratorASN4` | `pkg/bgp/attribute/aspath_iter_test.go` | 4-byte ASN parsing |

### Phase 2: Span Type

| Test | File | Validates |
|------|------|-----------|
| `TestSpanSlice` | `pkg/bgp/span_test.go` | Extracts subslice |
| `TestSpanEmpty` | `pkg/bgp/span_test.go` | Zero-length span |
| `TestParseUpdateOffsets` | `pkg/bgp/message/offsets_test.go` | Returns section spans |

### Phase 3: Direct Formatting

| Test | File | Validates |
|------|------|-----------|
| `TestFormatNLRIJSON` | `pkg/plugin/format_test.go` | Formats prefix from bytes |
| `TestFormatASPathJSON` | `pkg/plugin/format_test.go` | Formats AS-PATH from bytes |
| `TestFormatUpdateJSON` | `pkg/plugin/format_test.go` | Full UPDATE formatting |
| `TestFormatUpdateText` | `pkg/plugin/format_test.go` | Text format from bytes |

### Phase 4: RIB Integration

| Test | File | Validates |
|------|------|-----------|
| `TestRouteAttrIterator` | `pkg/rib/route_iter_test.go` | Route exposes iterator |
| `TestRouteASPathCaching` | `pkg/rib/route_iter_test.go` | Offset caching works |
| `TestRouteZeroCopy` | `pkg/rib/route_iter_test.go` | Direct forwarding |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| `iterator-nlri` | `qa/tests/iterator/` | NLRI iteration matches parsed |
| `iterator-attrs` | `qa/tests/iterator/` | Attr iteration matches parsed |
| `format-json` | `qa/tests/format/` | JSON output identical |
| `format-text` | `qa/tests/format/` | Text output identical |

## Implementation Phases

### Phase 1: Core Types (Non-Breaking)

Add iterator types without removing existing code:

```go
// pkg/bgp/span.go
type Span struct {
    Start int
    Len   int
}

// pkg/bgp/nlri/iterator.go
type NLRIIterator struct { ... }

// pkg/bgp/attribute/iterator.go
type AttrIterator struct { ... }

// pkg/bgp/attribute/aspath_iter.go
type ASPathIterator struct { ... }
```

**Files created:**
- `pkg/bgp/span.go` ✅
- `pkg/bgp/span_test.go` ✅
- `pkg/bgp/nlri/iterator.go` ✅
- `pkg/bgp/nlri/iterator_test.go` ✅
- `pkg/bgp/attribute/iterator.go` ✅
- `pkg/bgp/attribute/iterator_test.go` ✅
- `pkg/bgp/attribute/aspath_iter.go` ✅
- `pkg/bgp/attribute/aspath_iter_test.go` ✅

**Verification (2026-01-10):**
- [x] `make test` passes
- [x] `make lint` passes (0 issues)
- [x] `make functional` passes (80 tests)
- [x] Existing tests unchanged

### Phase 2: WireUpdate Integration

Add iterator methods to `WireUpdate`:

```go
// pkg/plugin/wire_update.go
func (u *WireUpdate) NLRIIterator(addPath bool) (*nlri.NLRIIterator, error)
func (u *WireUpdate) WithdrawnIterator(addPath bool) (*nlri.NLRIIterator, error)
func (u *WireUpdate) AttrIterator() (*attribute.AttrIterator, error)
```

**Files modified:**
- `pkg/plugin/wire_update.go` ✅
- `pkg/plugin/wire_update_test.go` ✅ (6 new tests)

**Verification (2026-01-10):**
- [x] `make test` passes
- [x] `make lint` passes (0 issues)
- [x] `make functional` passes (80 tests)

### Phase 3: Direct Formatting

Add format functions that write directly from buffer:

```go
// pkg/plugin/format_buffer.go
func FormatNLRIJSON(prefix []byte, pathID uint32, w io.Writer) error
func FormatASPathJSON(data []byte, asn4 bool, w io.Writer) error
func FormatAttributesJSON(data []byte, ctx ParseContext, w io.Writer) error
func FormatUpdateEventJSON(u *WireUpdate, ctx ParseContext, w io.Writer) error
```

**Files to create:**
- `pkg/plugin/format_buffer.go`

**Verification:**
- [ ] Output matches existing JSON encoder
- [ ] `make functional` passes

### Phase 4: RIB Migration

Update `Route` to use iterators:

```go
// pkg/rib/route.go
func (r *Route) AttrIterator() *attribute.AttrIterator
func (r *Route) ASPathIterator(asn4 bool) *attribute.ASPathIterator
```

Remove parsed attribute storage (if safe):
- [ ] Verify no code depends on `r.attributes` slice
- [ ] Keep wire bytes only

**Files to modify:**
- `pkg/rib/route.go`

### Phase 5: Deprecate Parsed Types

Mark for removal:
- `plugin.PathAttributes` struct
- `plugin.RouteUpdate` struct
- Slice-returning methods

Add deprecation comments:
```go
// Deprecated: Use AttrIterator instead. Will be removed.
func (u *Update) Attributes() []attribute.Attribute
```

### Phase 6: Remove Deprecated Code

After all consumers migrated:
- Remove `PathAttributes` struct
- Remove `RouteUpdate` struct
- Remove `rr.UpdateInfo` struct
- Simplify `RawMessage`

## Files Summary

### Create
| File | Purpose |
|------|---------|
| `pkg/bgp/span.go` | Span type for buffer sections |
| `pkg/bgp/nlri/iterator.go` | NLRI iterator |
| `pkg/bgp/attribute/iterator.go` | Attribute iterator |
| `pkg/bgp/attribute/aspath_iter.go` | AS-PATH iterator |
| `pkg/plugin/format_buffer.go` | Direct buffer formatting |

### Modify
| File | Changes |
|------|---------|
| `pkg/plugin/wire_update.go` | Add iterator methods |
| `pkg/rib/route.go` | Expose iterators, remove parsed storage |
| `pkg/plugin/json.go` | Use buffer formatting |

### Remove (Phase 6)
| File/Type | Replacement |
|-----------|-------------|
| `plugin.PathAttributes` | `AttrIterator` |
| `plugin.RouteUpdate` | `FormatUpdateEventJSON` |
| `rr.UpdateInfo` | `WireUpdate` |

## Compatibility Notes

### Wire Encoding Unchanged
- BGP wire format is not changing
- Only internal representation changes

### API Output Unchanged
- JSON format stays identical
- Text format stays identical
- Plugins see no difference

### Go Package API Changes
- New iterator types added
- Old methods deprecated then removed
- Per stability policy: Go API is unstable

## Checklist

### 🧪 TDD
- [x] Phase 1 tests written and FAIL
- [x] Phase 1 implementation complete, tests PASS
- [x] Phase 2 tests written and FAIL
- [x] Phase 2 implementation complete, tests PASS
- [ ] Phase 3 tests written and FAIL
- [ ] Phase 3 implementation complete, tests PASS
- [ ] Phase 4 tests written and FAIL
- [ ] Phase 4 implementation complete, tests PASS

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes
- [ ] JSON output unchanged (diff test)
- [ ] Text output unchanged (diff test)

### Documentation
- [x] `docs/architecture/buffer-architecture.md` exists
- [x] RFC references added to iterator code
- [ ] Deprecation comments added

### Completion
- [ ] Spec moved to `docs/plan/done/NNN-buffer-first-migration.md`
