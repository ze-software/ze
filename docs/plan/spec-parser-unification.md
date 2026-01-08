# Spec: Parser Unification

## Status: REVISED

**Original approach (tokenizer abstraction): Rejected**
**New approach: Extract value parsers to `pkg/parse/`**

See "Analysis" section for rationale.

---

## Goal

Reduce code duplication between API and config parsing for BGP attribute values.

## Analysis: Why Tokenizer Abstraction Was Rejected

### Original Proposal

```
Input (API or Config)
        ↓
Format-specific Tokenizer
        ↓
Common Token Stream: []Token
        ↓
Shared Parser: parseUpdate()
        ↓
UpdateCommand struct
```

### Problem: Wrong Abstraction Layer

| Layer | API | Config | Shareable? |
|-------|-----|--------|------------|
| Input | `[]string` | Token stream | ❌ Different |
| Grammar | Flat: `origin set igp nlri ...` | Nested: `{ origin igp; }` | ❌ Different |
| Value parsing | `igp`→0, `65000:100`→uint32 | Same | ✅ Same |

**The grammars are fundamentally different.** Config uses `{ }` and `;` delimiters, API uses flat keyword sequences. Forcing them through a common tokenizer interface adds coupling without benefit.

### Quantified Duplication

| Parser | API LOC | Config LOC | Shareable |
|--------|---------|------------|-----------|
| Community | → `parse.Community()` | → `parse.Community()` | ✅ Already shared |
| Large Community | → `parse.LargeCommunity()` | → `parse.LargeCommunity()` | ✅ Already shared |
| Origin | ~15 inline | ~15 `ParseOrigin()` | ❌ Duplicated |
| Extended Community | ~100 inline | ~150 `ParseExtendedCommunity()` | ❌ Duplicated |
| AS-Path | ~20 inline | ~30 `ParseASPath()` | ❌ Duplicated |
| Route Distinguisher | `nlri.ParseRDString()` | `ParseRouteDistinguisher()` | ❌ Two impls |

**Total duplicated: ~135 LOC** (value parsing, not structure)

### Cost Comparison

| Approach | Effort | Savings | Coupling |
|----------|--------|---------|----------|
| Tokenizer abstraction | ~500 LOC | ~135 LOC | High (grammar locked) |
| Extract value parsers | ~100 LOC new | ~135 LOC | Low (independent) |

## Current State

| | Config | API | Shared |
|---|---|---|---|
| Tokenizer | `pkg/config/tokenizer.go` | None (`[]string`) | - |
| Structure parser | recursive descent with `{ }` | linear keyword scan | ❌ Different |
| Community | `parseOneCommunity()` | `parseCommunity()` | → `parse.Community()` ✅ |
| Large Community | `parseOneLargeCommunity()` | `parseLargeCommunity()` | → `parse.LargeCommunity()` ✅ |
| Origin | `ParseOrigin()` | inline switch | ❌ Duplicate |
| Extended Community | `parseOneExtCommunity()` | inline | ❌ Duplicate |
| AS-Path | `ParseASPath()` | inline | ❌ Duplicate |
| Route Distinguisher | `ParseRouteDistinguisher()` | `nlri.ParseRDString()` | ❌ Two impls |

## Target Architecture

Extract RFC-compliant value parsers to `pkg/parse/`. Keep structure parsing separate.

```
pkg/parse/
├── community.go       # ✅ Already exists
├── origin.go          # NEW: Origin value parsing
├── extended.go        # NEW: Extended community parsing
├── aspath.go          # NEW: AS-Path parsing
├── rd.go              # NEW: Route Distinguisher (wire bytes)
└── *_test.go          # Tests for each
```

### Design Principle

```
┌─────────────────────────────────────────────────────┐
│   API (pkg/api/)       │   Config (pkg/config/)    │
│   Flat keyword parser  │   Nested block parser     │
├────────────────────────┴────────────────────────────┤
│           Shared Value Parsers (pkg/parse/)         │
│   Origin, ExtendedCommunity, ASPath, RD, etc.       │
└─────────────────────────────────────────────────────┘
```

## Implementation Steps

### Phase 1: Extract Origin (Priority: Low, ~15 LOC)

1. [ ] Create `pkg/parse/origin.go`
2. [ ] Write test → FAIL
3. [ ] Implement `parse.Origin(s string) (uint8, error)`
4. [ ] Test → PASS
5. [ ] Update `pkg/api/route.go` to use `parse.Origin()`
6. [ ] Update `pkg/config/routeattr.go` to use `parse.Origin()`
7. [ ] Delete duplicate code

```go
// pkg/parse/origin.go
func Origin(s string) (uint8, error) {
    switch strings.ToLower(s) {
    case "", "igp":
        return 0, nil
    case "egp":
        return 1, nil
    case "incomplete":
        return 2, nil
    default:
        return 0, fmt.Errorf("invalid origin %q: valid values are igp, egp, incomplete", s)
    }
}
```

### Phase 2: Extract Extended Community (Priority: High, ~100 LOC savings)

1. [ ] Create `pkg/parse/extended.go`
2. [ ] Write tests for all formats → FAIL
3. [ ] Implement `parse.ExtendedCommunity(s string) ([]byte, error)`
4. [ ] Test → PASS
5. [ ] Update API and config to use shared parser
6. [ ] Delete duplicate code

Formats to support:
- `target:ASN:NN` / `origin:ASN:NN`
- `target:IP:NN` / `origin:IP:NN`
- `target4:ASN:NN` / `origin4:ASN:NN`
- `redirect:ASN:NN`
- `0x0002fde800000001` (hex wire format)
- `ASN:NN` (generic)

### Phase 3: Extract AS-Path (Priority: Medium, ~20 LOC)

1. [ ] Create `pkg/parse/aspath.go`
2. [ ] Write test → FAIL
3. [ ] Implement `parse.ASPath(s string) ([]uint32, error)`
4. [ ] Test → PASS
5. [ ] Update API and config
6. [ ] Delete duplicate code

```go
// pkg/parse/aspath.go
// Parses "[ 65000 65001 65002 ]" or "65000 65001" format
func ASPath(s string) ([]uint32, error)
```

### Phase 4: Unify Route Distinguisher (Priority: Medium, type decision needed)

Current state:
- `nlri.ParseRDString()` → `nlri.RouteDistinguisher`
- `config.ParseRouteDistinguisher()` → `config.RouteDistinguisher{Bytes [8]byte}`

Options:
1. **Keep `nlri.RouteDistinguisher`** - config converts at load time
2. **New `parse.RouteDistinguisher`** - returns `[8]byte`, both convert

Decision: TBD based on usage patterns.

## File Changes

### New Files
- `pkg/parse/origin.go` + `origin_test.go`
- `pkg/parse/extended.go` + `extended_test.go`
- `pkg/parse/aspath.go` + `aspath_test.go`

### Modified Files
- `pkg/api/route.go` - use `parse.Origin()`, `parse.ExtendedCommunity()`, `parse.ASPath()`
- `pkg/config/routeattr.go` - use shared parsers, delete duplicate implementations

### Deleted Code
- `pkg/config/routeattr.go`: `ParseOrigin()` body (keep wrapper if needed)
- `pkg/config/routeattr.go`: `parseOneExtCommunity()` body
- `pkg/config/routeattr.go`: `ParseASPath()` body
- `pkg/api/route.go`: inline origin switch in `parseCommonAttribute()`
- `pkg/api/route.go`: inline ext-community parsing

## Checklist

### Phase 1: Origin
- [ ] `pkg/parse/origin.go` created
- [ ] `pkg/parse/origin_test.go` with TDD
- [ ] API updated to use `parse.Origin()`
- [ ] Config updated to use `parse.Origin()`

### Phase 2: Extended Community (HIGH PRIORITY)
- [ ] `pkg/parse/extended.go` created
- [ ] `pkg/parse/extended_test.go` with TDD
- [ ] All formats tested (target, origin, hex, generic)
- [ ] API updated
- [ ] Config updated

### Phase 3: AS-Path
- [ ] `pkg/parse/aspath.go` created
- [ ] `pkg/parse/aspath_test.go` with TDD
- [ ] API updated
- [ ] Config updated

### Phase 4: Route Distinguisher
- [ ] Type decision made
- [ ] Implementation complete
- [ ] Both consumers updated

### Verification
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] `make functional` passes
- [ ] No duplicate code remaining

---

**Created:** 2025-01-04
**Revised:** 2025-01-08 - Tokenizer approach rejected, switched to value parser extraction
**Depends on:** None (incremental, can proceed independently)
