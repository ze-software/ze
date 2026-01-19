# Spec: Text Mode Per-Attribute Syntax

**Status: IMPLEMENTED**

## Task

Fix text mode parser to match spec: attribute names are keywords, not inside `attr set`.

## Grammar

```
<update-text> := <section>*
<section>     := <scalar-attr> | <list-attr> | <nlri-section> | <wire-attr>

<scalar-attr> := <scalar-name> (set <value> | del [<value>])
<scalar-name> := origin | med | local-preference | nhop | path-information | rd | label

<list-attr>   := <list-name> (set <list> | add <list> | del [<list>])
<list-name>   := as-path | community | large-community | extended-community

<nlri-section> := nlri <family> <nlri-op>+
<nlri-op>      := add <prefix>+ [watchdog set <name>] | del <prefix>+

<wire-attr>    := attr (set <bytes> | del [<bytes>])   # hex/b64 mode only
```

### Standalone Watchdog Commands

```
watchdog announce <name>   # send all routes in pool to peers
watchdog withdraw <name>   # withdraw all routes in pool from peers
```

### Attribute Types

| Type | Attributes |
|------|------------|
| **Scalar** | `origin`, `med`, `local-preference`, `nhop`, `path-information`, `rd`, `label` |
| **List** | `as-path`, `community`, `large-community`, `extended-community` |

### Scalar `del [<value>]` Semantics

- `<scalar> del` - remove attribute unconditionally
- `<scalar> del <value>` - remove only if current value matches, else error

Example:
```
origin set igp
origin del igp    # OK - matches current value
origin del egp    # ERROR - current value is igp, not egp
```

### Accumulator → Family Support

| Accumulator | Valid for | Error for |
|-------------|-----------|-----------|
| `rd` | `*-vpn` families | All others |
| `label` | `*-vpn`, `*-labeled` families | All others |
| `path-information` | Any (if ADD-PATH negotiated) | Ignored if not negotiated |

Note: `rd` and `label` not yet implemented.

## Required Reading

- [x] `docs/plan/new-syntax.md` - defines target syntax (`origin set igp`, not `attr set origin igp`)
- [x] `.claude/zebgp/api/ARCHITECTURE.md` - API command structure, UpdateText parser
- [x] `.claude/zebgp/wire/ATTRIBUTES.md` - attribute types (scalar vs list) for set/add/del validation
- [x] `.claude/zebgp/UPDATE_BUILDING.md` - Build path for API routes
- [x] `internal/plugin/update_text.go` - current parser implementation
- [x] `internal/plugin/update_text_test.go` - existing tests to update

**Key insights:**
- Spec defines text mode as per-attribute keywords: `origin set igp`, `med set 100`
- Wire mode (hex/b64) keeps `attr set <bytes>` unchanged
- Scalar attrs: `set` and `del [<value>]` (conditional delete)
- List attrs: `set`, `add`, `del [<list>]`
- AS-PATH: `set`, `add` (prepend), `del` (remove specific or all)
- Watchdog tags routes in nlri section: `add 1.0.0.0/24 watchdog set mypool`
- Standalone watchdog control: `watchdog announce/withdraw <name>`

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| `TestParseUpdateText_OriginSetIGP` | `internal/plugin/update_text_test.go` | `origin set igp` parses to Origin=0 |
| `TestParseUpdateText_MedSet` | `internal/plugin/update_text_test.go` | `med set 100` parses to MED=100 |
| `TestParseUpdateText_LocalPrefSet` | `internal/plugin/update_text_test.go` | `local-preference set 200` parses |
| `TestParseUpdateText_CommunitySet` | `internal/plugin/update_text_test.go` | `community set [65000:1]` replaces |
| `TestParseUpdateText_CommunityAdd` | `internal/plugin/update_text_test.go` | `community add [65000:2]` appends |
| `TestParseUpdateText_CommunityDel` | `internal/plugin/update_text_test.go` | `community del [65000:1]` removes |
| `TestParseUpdateText_MultipleAttrs` | `internal/plugin/update_text_test.go` | `origin set igp med set 100` both parsed |
| `TestParseUpdateText_OldAttrSetRejected` | `internal/plugin/update_text_test.go` | `attr set origin igp` returns error for text mode |

### Functional Tests

| Test | Location | Scenario |
|------|----------|----------|
| All .run files | `test/data/api/*.run` | Re-migrated to per-attribute syntax |

## Files to Modify

- `internal/plugin/update_text.go` - add attribute keywords as section starters
- `internal/plugin/update_text_test.go` - update 50+ tests to new syntax
- `scripts/migrate-api-syntax.py` - fix to generate per-attribute format
- `test/data/api/*.run` - re-migrate using fixed script

## Implementation Steps

1. **Write tests** - Add tests for `origin set igp`, `med set 100`, etc.
2. **Run tests** - Verify FAIL (paste output)
3. **Update parser** - Add attribute names to `isBoundaryKeyword()`, add `parseScalarAttr()`, `parseListAttr()`
4. **Run tests** - Verify PASS (paste output)
5. **Update existing tests** - Change `attr set origin igp` to `origin set igp` in all tests
6. **Run tests** - Verify PASS (paste output)
7. **Fix migration script** - Change `convert_attrs()` to generate per-attribute format
8. **Re-migrate .run files** - `git checkout -- test/data/ && python3 scripts/migrate-api-syntax.py`
9. **Verify all** - `make lint && make test && make functional` (paste output)

## RFC Documentation

- RFC 4271 Section 5 - Path Attributes (ORIGIN, AS_PATH, NEXT_HOP, MED, LOCAL_PREF)
- RFC 1997 - Communities
- RFC 8092 - Large Communities
- Add `// RFC NNNN` comments if modifying protocol-related code

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
- [ ] RFC references added (if applicable)
- [ ] `.claude/zebgp/api/ARCHITECTURE.md` updated if parser behavior changed

### Completion
- [ ] Spec moved to `docs/plan/done/NNN-text-mode-per-attribute.md`

---

**Created:** 2026-01-07
