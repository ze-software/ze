# Plan: Rename `neighbor` to `peer` in Config

## Rationale
- `peer` is shorter (4 chars vs 8 chars)
- Less prone to typos
- Consistent with API commands (`peer *`, `peer show`)

## Scope

### Schema Changes
| File | Change |
|------|--------|
| `pkg/config/bgp.go:282` | `schema.Define("neighbor", ...)` → `schema.Define("peer", ...)` |
| `pkg/config/bgp.go:292` | Template: `Field("neighbor", ...)` → `Field("peer", ...)` |
| `pkg/config/bgp.go:159` | Rename `neighborFields()` → `peerFields()` |

### Parser/TreeToConfig Changes
| File | Change |
|------|--------|
| `pkg/config/bgp.go:496` | `tmpl.GetList("neighbor")` → `tmpl.GetList("peer")` |
| `pkg/config/bgp.go:515` | `tree.GetList("neighbor")` → `tree.GetList("peer")` |
| `pkg/config/bgp.go:518` | Error msg: `"neighbor %s"` → `"peer %s"` |

### Struct Names (Keep as-is)
- `NeighborConfig` - internal Go name, no change needed
- `NeighborReactor` - internal Go name, no change needed

### Test Updates
| File | Count | Change |
|------|-------|--------|
| `bgp_test.go` | ~50 | `neighbor` → `peer` in config strings |
| `parser_test.go` | ~20 | `neighbor` → `peer` in config strings |
| `extended_test.go` | ~30 | `neighbor` → `peer` in config strings |
| `setparser_test.go` | ~25 | `set neighbor` → `set peer` |
| `serialize_test.go` | ~5 | `neighbor` → `peer` in config strings |
| `schema_test.go` | ~5 | `neighbor` → `peer` in schema tests |
| `tokenizer_test.go` | ~5 | `neighbor` → `peer` in tokenizer tests |
| `loader_test.go` | ~5 | `neighbor` → `peer` in config strings |

### SetParser Changes
| File | Change |
|------|--------|
| `pkg/config/setparser.go:19-21` | Update comments: `set neighbor` → `set peer` |

### Backward Compatibility (Optional)
Could support both syntaxes during transition:
```go
// Accept both "neighbor" and "peer" temporarily
schema.Define("neighbor", List(TypeIP, peerFields()...)) // deprecated
schema.Define("peer", List(TypeIP, peerFields()...))     // preferred
```

## Implementation Steps

1. **Schema**: Update `bgp.go` schema definitions
2. **Parser**: Update `GetList("neighbor")` calls
3. **Tests**: Bulk replace `neighbor` → `peer` in test configs
4. **SetParser**: Update comments and any hardcoded strings
5. **Verify**: `make test && make lint`

## Execution Strategy

Use `replace_all` for bulk changes:
```
# In test files - replace config keyword only
"neighbor " → "peer "
```

Careful with:
- `neighborFields()` function name
- `NeighborConfig` struct name (keep)
- Comments mentioning "neighbor"
- `neighborsByAddr` test helper (keep)

## Estimated Changes
- ~150 occurrences of `neighbor` in config strings
- ~10 occurrences in schema/parser code
- 0 struct renames needed
