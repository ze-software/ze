# Spec: RIB Adjacent Commands with Full Attributes

## Task

Add Adj-RIB inspection and manipulation commands to the RIB plugin with:
1. Peer selector filtering (*, IP, !IP, ip,ip,ip)
2. Full attribute storage and resend capability
3. Proper package organization for shared code

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/ARCHITECTURE.md` - RIB plugin design, API protocol
- [ ] `docs/architecture/wire/ATTRIBUTES.md` - Attribute formats

### RFC Summaries
- [ ] `docs/rfc/rfc4271.md` - BGP-4 path attributes

**Key insights:**
- RIB plugin tracks routes as shadow copy, not actual router RIB
- Resend must include full attributes for correct BGP semantics
- Selector syntax is generic ZeBGP convention, not plugin-specific

## Design Decisions

### Package Reorganization

**Problem:** `pkg/plugin/selector.go` contains generic selector syntax used beyond plugins.

**Solution:** New package structure:
```
pkg/selector/           # Peer selector syntax (*, IP, !IP, ip,ip,ip)
  selector.go           # Selector type, Parse(), Matches(), String()
  selector_test.go

pkg/bgp/attribute/      # Wire format + text format
  text.go               # Text parse/format functions (NEW)
  community.go          # Existing wire format
  ...

pkg/parse/              # DELETE - consolidate into attribute/text.go

pkg/plugin/             # Plugin system only
  route.go              # Remove parse functions, import from attribute
  ...
```

### Attribute Text Format

**Rule:** `[]` brackets if more than one value.

| Attribute | Single | Multiple |
|-----------|--------|----------|
| origin | `origin igp` | N/A (scalar) |
| as-path | `as-path 65001` | `as-path [65001 65002]` |
| med | `med 100` | N/A (scalar) |
| local-preference | `local-preference 100` | N/A (scalar) |
| community | `community 65000:100` | `community [65000:100 65000:200]` |
| large-community | `large-community 65000:1:2` | `large-community [65000:1:2 65000:1:3]` |

### Command Naming

| Old | New | Reason |
|-----|-----|--------|
| `rib status` | `rib adjacent status` | Clarity: these are Adj-RIBs |
| `rib routes` | `rib adjacent inbound show` | Explicit direction |
| `rib routes in` | `rib adjacent inbound show` | Consistent naming |
| `rib routes out` | `rib adjacent outbound show` | Consistent naming |
| N/A | `rib adjacent inbound empty` | Clear semantics (not "clear") |
| N/A | `rib adjacent outbound resend` | Re-announce stored routes |

**Note:** `inbound empty` (not `clear`) because it only empties local plugin state, doesn't trigger BGP route-refresh.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestParse*` | `pkg/selector/selector_test.go` | Selector parsing |
| `TestSelectorMatches*` | `pkg/selector/selector_test.go` | Selector matching |
| `TestFormatASPath` | `pkg/bgp/attribute/text_test.go` | AS-path formatting |
| `TestFormatCommunities` | `pkg/bgp/attribute/text_test.go` | Community formatting |
| `TestParseCommunities` | `pkg/bgp/attribute/text_test.go` | Community parsing |
| `TestHandleRequest_RIBAdjacentInboundEmpty` | `pkg/plugin/rib/rib_test.go` | Empty command |
| `TestHandleRequest_RIBAdjacentOutboundResend` | `pkg/plugin/rib/rib_test.go` | Resend with attrs |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| Existing | `qa/tests/` | Existing tests must pass |

## Files to Modify

### New Files
- `pkg/selector/selector.go` - Selector type (moved from plugin)
- `pkg/selector/selector_test.go` - Tests (moved from plugin)
- `pkg/bgp/attribute/text.go` - Text parse/format functions
- `pkg/bgp/attribute/text_test.go` - Tests

### Modified Files
- `pkg/plugin/selector.go` - DELETE (moved to pkg/selector/)
- `pkg/plugin/selector_test.go` - DELETE (moved to pkg/selector/)
- `pkg/plugin/route.go` - Remove parse functions, import from attribute
- `pkg/plugin/types.go` - Import selector from new location
- `pkg/plugin/forward.go` - Import selector from new location
- `pkg/plugin/command.go` - Import selector from new location
- `pkg/reactor/reactor.go` - Import selector from new location
- `pkg/parse/community.go` - DELETE (moved to attribute/text.go)
- `pkg/parse/community_test.go` - DELETE (moved to attribute/text_test.go)
- `pkg/plugin/rib/rib.go` - Rename command, store attrs, format on resend
- `pkg/plugin/rib/rib_test.go` - Update tests
- `pkg/plugin/rib/event.go` - Add attribute fields for JSON parsing

## Implementation Steps

1. **Create pkg/selector/** - Move selector code
2. **Update selector imports** - All files using Selector
3. **Create pkg/bgp/attribute/text.go** - Parse/format functions
4. **Move from pkg/parse/** - Community parsing
5. **Move from pkg/plugin/route.go** - Attribute parsing
6. **Delete pkg/parse/** - After consolidation
7. **Update pkg/plugin/route.go** - Import from attribute
8. **Update RIB Event struct** - Add attribute fields
9. **Update RIB Route struct** - Store attributes
10. **Rename inbound clear → empty** - Command rename
11. **Update sendRoutes** - Format full attributes using attribute.Format*
12. **Run tests** - `make test && make lint && make functional`

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
- [ ] Code follows existing patterns

### Completion
- [ ] Spec moved to `docs/plan/done/NNN-rib-adj-commands.md`
