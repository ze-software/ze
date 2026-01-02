# Spec: API Attribute Filter

## Task

Add config option to limit which attributes are parsed and output in API messages, reducing CPU and memory for external processes that only need a subset.

## Required Reading (MUST complete before implementation)

- [x] `.claude/zebgp/api/ARCHITECTURE.md` - API binding model, update-id pattern
- [x] `.claude/zebgp/wire/ATTRIBUTES.md` - Attribute codes and wire format
- [x] `.claude/zebgp/config/SYNTAX.md` - API block syntax
- [x] `.claude/zebgp/EXABGP_COMPATIBILITY.md` - Compatibility philosophy
- [x] `pkg/bgp/attribute/wire.go` - AttributesWire implementation
- [x] `pkg/api/decode.go` - UPDATE parsing, attribute extraction pattern
- [x] `pkg/api/types.go` - ContentConfig, RawMessage structures

**Key insights from docs:**

1. API bindings support per-peer config in `content { }` block
2. Update-id assigned for ALL received UPDATEs at `reactor.go:2162`
3. AttributesWire has lazy parsing: `GetMultiple()` returns `map[Code]Attribute`
4. AttributesWire.All() returns `[]Attribute` (NOT map) - need conversion
5. AttributesWire has internal `sync.RWMutex` - no external mutex needed
6. Attribute bytes extraction pattern in `pkg/api/decode.go:43-49`
7. **Gap:** RawMessage needs AttrsWire field for lazy parsing
8. ZeBGP uses plural `communities`, ExaBGP uses singular (documented difference)

## Current State

- Tests: `make test` PASS, `make lint` 0 issues, `make functional` 37/37
- AttributesWire: `pkg/bgp/attribute/wire.go`
- Existing decode: `pkg/api/decode.go`

## Files to Modify

| File | Change |
|------|--------|
| `pkg/api/filter.go` | NEW: AttributeFilter type with FilterMode enum |
| `pkg/api/types.go` | Add `AttrsWire *AttributesWire` to RawMessage |
| `pkg/api/types.go` | Add `Attributes *AttributeFilter` to ContentConfig |
| `pkg/api/text.go` | Add `formatRoutesJSONv7WithAttrs()`, modify formatters |
| `pkg/api/decode.go` | Add `ExtractAttributeBytes()` helper |
| `pkg/config/api.go` | Add `parseAttributeFilter()` |
| `pkg/config/bgp.go` | Add `Attributes` to PeerContentConfig |
| `pkg/config/migration/attributes.go` | NEW: singular→plural migration |
| `pkg/reactor/reactor.go` | Create AttrsWire, pass in RawMessage |

## Problem

Current API outputs ALL parsed attributes, even if external process only needs a few.

## Solution

Config option in content block:

```
api foo {
    content {
        encoding json;
        attributes as-path next-hop communities;  # Only parse/output these
    }
    receive { update; }
}
```

## Limitations

1. **V7 format only:** Attribute filtering requires `version 7` (default). V6 format ignores filter and outputs all attributes.
2. **Received UPDATEs only:** Filter applies to UPDATEs received from peers. API-originated announcements are unaffected.

## Config Syntax

### Attribute Names

| Config Name | Code | Description |
|-------------|------|-------------|
| `origin` | 1 | ORIGIN |
| `as-path` | 2 | AS_PATH |
| `next-hop` | 3 | NEXT_HOP |
| `med` | 4 | MULTI_EXIT_DISC |
| `local-pref` | 5 | LOCAL_PREF |
| `atomic-aggregate` | 6 | ATOMIC_AGGREGATE |
| `aggregator` | 7 | AGGREGATOR |
| `community` | 8 | COMMUNITIES (singular for backward compat) |
| `communities` | 8 | COMMUNITIES (preferred, plural) |
| `originator-id` | 9 | ORIGINATOR_ID |
| `cluster-list` | 10 | CLUSTER_LIST |
| `extended-community` | 16 | EXTENDED_COMMUNITIES (singular compat) |
| `extended-communities` | 16 | EXTENDED_COMMUNITIES (preferred) |
| `large-community` | 32 | LARGE_COMMUNITIES (singular compat) |
| `large-communities` | 32 | LARGE_COMMUNITIES (preferred) |
| `attr-N` | N | Unknown/numeric attribute (e.g., `attr-99`) |
| `all` | - | All attributes (default) |
| `none` | - | No attributes (update-id only) |

**Note:** MP_REACH_NLRI (14) and MP_UNREACH_NLRI (15) are structural. NOT filterable.

**Backward Compatibility:** `community`, `extended-community`, `large-community` accepted in config. Migration tool converts to plural form.

## JSON Output Key Names

| Attribute | JSON Key | Notes |
|-----------|----------|-------|
| ORIGIN | `"origin"` | Singular |
| AS_PATH | `"as-path"` | Singular |
| NEXT_HOP | `"next-hop"` | Singular |
| MED | `"med"` | Singular |
| LOCAL_PREF | `"local-pref"` | Singular |
| COMMUNITIES | `"communities"` | **Plural** (differs from ExaBGP) |
| EXTENDED_COMMUNITIES | `"extended-communities"` | **Plural** |
| LARGE_COMMUNITIES | `"large-communities"` | **Plural** |

**ExaBGP difference:** ZeBGP uses plural for community types. See `.claude/zebgp/EXABGP_COMPATIBILITY.md`.

## Implementation

### AttributeFilter Type (No Invalid States, No Mutex)

```go
// pkg/api/filter.go

package api

import (
    "fmt"

    "github.com/exa-networks/zebgp/pkg/bgp/attribute"
)

// FilterMode defines how attributes are selected.
type FilterMode uint8

const (
    FilterModeAll       FilterMode = iota // Include all attributes (default)
    FilterModeNone                        // Include no attributes
    FilterModeSelective                   // Include only specified codes
)

// AttributeFilter specifies which attributes to include in API output.
// Thread-safe: AttributesWire has internal mutex; filter is read-only after construction.
type AttributeFilter struct {
    Mode  FilterMode
    Codes []attribute.AttributeCode // Only valid when Mode == FilterModeSelective
}

// NewFilterAll returns a filter that includes all attributes.
func NewFilterAll() AttributeFilter {
    return AttributeFilter{Mode: FilterModeAll}
}

// NewFilterNone returns a filter that excludes all attributes.
func NewFilterNone() AttributeFilter {
    return AttributeFilter{Mode: FilterModeNone}
}

// NewFilterSelective returns a filter for specific attribute codes.
func NewFilterSelective(codes []attribute.AttributeCode) AttributeFilter {
    return AttributeFilter{Mode: FilterModeSelective, Codes: codes}
}

// IsEmpty returns true if no attributes would be included.
func (f AttributeFilter) IsEmpty() bool {
    return f.Mode == FilterModeNone || (f.Mode == FilterModeSelective && len(f.Codes) == 0)
}

// Apply returns filtered attributes from AttributesWire.
// Returns nil map for FilterModeNone or nil wire.
// Thread-safe: AttributesWire handles its own locking.
func (f AttributeFilter) Apply(wire *attribute.AttributesWire) (map[attribute.AttributeCode]attribute.Attribute, error) {
    if wire == nil {
        return nil, nil
    }

    switch f.Mode {
    case FilterModeNone:
        return nil, nil

    case FilterModeAll:
        // All() returns []Attribute, convert to map
        attrs, err := wire.All()
        if err != nil {
            return nil, err
        }
        if len(attrs) == 0 {
            return nil, nil
        }
        result := make(map[attribute.AttributeCode]attribute.Attribute, len(attrs))
        for _, attr := range attrs {
            result[attr.Code()] = attr
        }
        return result, nil

    case FilterModeSelective:
        // GetMultiple() returns map directly
        result, err := wire.GetMultiple(f.Codes)
        if err != nil {
            return nil, err
        }
        if len(result) == 0 {
            return nil, nil
        }
        return result, nil

    default:
        return nil, fmt.Errorf("unknown filter mode: %d", f.Mode)
    }
}
```

### Attribute Bytes Extraction

```go
// pkg/api/decode.go - add helper function

// ExtractAttributeBytes extracts the path attributes section from UPDATE body.
// Returns nil if body is malformed or has no attributes.
// Pattern from existing DecodeUpdate at lines 43-49.
func ExtractAttributeBytes(body []byte) []byte {
    if len(body) < 4 {
        return nil
    }

    // Skip withdrawn routes
    withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
    offset := 2 + withdrawnLen

    if offset+2 > len(body) {
        return nil
    }

    // Read attribute length
    attrLen := int(binary.BigEndian.Uint16(body[offset : offset+2]))
    offset += 2

    if offset+attrLen > len(body) || attrLen == 0 {
        return nil
    }

    return body[offset : offset+attrLen]
}
```

### Config Parsing

```go
// pkg/config/api.go

var attributeNameToCode = map[string]attribute.AttributeCode{
    "origin":               attribute.AttrOrigin,
    "as-path":              attribute.AttrASPath,
    "next-hop":             attribute.AttrNextHop,
    "med":                  attribute.AttrMED,
    "local-pref":           attribute.AttrLocalPref,
    "atomic-aggregate":     attribute.AttrAtomicAggregate,
    "aggregator":           attribute.AttrAggregator,
    "community":            attribute.AttrCommunity,       // Singular (compat)
    "communities":          attribute.AttrCommunity,       // Plural (preferred)
    "originator-id":        attribute.AttrOriginatorID,
    "cluster-list":         attribute.AttrClusterList,
    "extended-community":   attribute.AttrExtCommunity,    // Singular (compat)
    "extended-communities": attribute.AttrExtCommunity,    // Plural (preferred)
    "large-community":      attribute.AttrLargeCommunity,  // Singular (compat)
    "large-communities":    attribute.AttrLargeCommunity,  // Plural (preferred)
}

func parseAttributeFilter(s string) (api.AttributeFilter, error) {
    s = strings.TrimSpace(s)
    if s == "all" || s == "" {
        return api.NewFilterAll(), nil
    }
    if s == "none" {
        return api.NewFilterNone(), nil
    }

    names := strings.Fields(s)
    seen := make(map[attribute.AttributeCode]bool, len(names))
    codes := make([]attribute.AttributeCode, 0, len(names))

    for _, name := range names {
        name = strings.ToLower(name)

        // Handle numeric attr-N syntax
        if strings.HasPrefix(name, "attr-") {
            numStr := strings.TrimPrefix(name, "attr-")
            num, err := strconv.Atoi(numStr)
            if err != nil || num < 0 || num > 255 {
                return api.AttributeFilter{}, fmt.Errorf("invalid attribute code: %s", name)
            }
            code := attribute.AttributeCode(num)
            if !seen[code] {
                seen[code] = true
                codes = append(codes, code)
            }
            continue
        }

        code, ok := attributeNameToCode[name]
        if !ok {
            return api.AttributeFilter{}, fmt.Errorf("unknown attribute %q, valid: %s, or attr-N for numeric",
                name, validAttributeNames())
        }
        if !seen[code] {
            seen[code] = true
            codes = append(codes, code)
        }
    }
    return api.NewFilterSelective(codes), nil
}

func validAttributeNames() string {
    // Return sorted list of valid names (excluding duplicates)
    unique := make(map[string]bool)
    for name := range attributeNameToCode {
        // Prefer plural forms in output
        if !strings.HasSuffix(name, "y") || strings.HasSuffix(name, "ies") {
            unique[name] = true
        }
    }
    names := make([]string, 0, len(unique))
    for name := range unique {
        names = append(names, name)
    }
    sort.Strings(names)
    return strings.Join(names, ", ")
}
```

### RawMessage Extension

```go
// pkg/api/types.go - add to RawMessage

type RawMessage struct {
    Type      message.Type
    RawBytes  []byte
    Timestamp time.Time
    UpdateID  uint64
    AttrsWire *attribute.AttributesWire  // NEW: for lazy attribute parsing
}
```

### ContentConfig Extension

```go
// pkg/api/types.go - add to ContentConfig

type ContentConfig struct {
    Encoding   string           // "json" | "text" (default: "text")
    Format     string           // "parsed" | "raw" | "full" (default: "parsed")
    Version    int              // 6 or 7 (default: 7)
    Attributes *AttributeFilter // NEW: nil means all (default)
}
```

### Reactor Integration

```go
// pkg/reactor/reactor.go - around line 2165

// Assign update-id for UPDATE messages (used for forwarding via API)
if msgType == message.TypeUPDATE && hasPeer {
    msg.UpdateID = nextUpdateID()

    // Create AttributesWire for lazy parsing (nil if extraction fails)
    if attrBytes := api.ExtractAttributeBytes(bytesCopy); attrBytes != nil {
        msg.AttrsWire = attribute.NewAttributesWire(attrBytes, peer.recvCtx.ID())
    }

    // Cache the update for forwarding
    r.recentUpdates.Add(&ReceivedUpdate{
        UpdateID:     msg.UpdateID,
        RawBytes:     bytesCopy,
        Attrs:        msg.AttrsWire,  // Share the same AttributesWire
        SourcePeerIP: peerAddr,
        SourceCtxID:  peer.recvCtx.ID(),
    })
}
```

### Message Formatting with Filter

```go
// pkg/api/text.go

// formatRoutesJSONv7WithAttrs formats UPDATE with pre-filtered attributes.
// attrs is nil for no attributes, empty map omits "attributes" key.
func formatRoutesJSONv7WithAttrs(
    peer PeerInfo,
    msg RawMessage,
    nlri []ReceivedRoute,  // NLRI decoded separately
    attrs map[attribute.AttributeCode]attribute.Attribute,
) string {
    var sb strings.Builder
    sb.WriteString(`{"type":"update"`)

    if msg.UpdateID != 0 {
        sb.WriteString(fmt.Sprintf(`,"update-id":%d`, msg.UpdateID))
    }

    sb.WriteString(fmt.Sprintf(`,"peer":{"address":"%s","asn":%d}`, peer.Address, peer.PeerAS))

    if len(nlri) > 0 {
        sb.WriteString(`,"announce":{"nlri":{`)
        // Format NLRI by family...
        formatNLRIByFamily(&sb, nlri)
        sb.WriteString(`}`)

        // Only include "attributes" key if attrs is non-nil and non-empty
        if len(attrs) > 0 {
            sb.WriteString(`,"attributes":{`)
            formatAttributesJSON(&sb, attrs)
            sb.WriteString(`}`)
        }
        sb.WriteString(`}`)
    }

    sb.WriteString(`}`)
    return sb.String()
}

// formatAttributesJSON writes attribute key-value pairs.
func formatAttributesJSON(sb *strings.Builder, attrs map[attribute.AttributeCode]attribute.Attribute) {
    first := true
    // Output in code order for deterministic output
    codes := make([]attribute.AttributeCode, 0, len(attrs))
    for code := range attrs {
        codes = append(codes, code)
    }
    sort.Slice(codes, func(i, j int) bool { return codes[i] < codes[j] })

    for _, code := range codes {
        attr := attrs[code]
        if !first {
            sb.WriteString(",")
        }
        first = false

        key := attributeCodeToJSONKey(code)
        sb.WriteString(fmt.Sprintf(`"%s":`, key))
        writeAttributeValue(sb, attr)
    }
}

func attributeCodeToJSONKey(code attribute.AttributeCode) string {
    switch code {
    case attribute.AttrOrigin:
        return "origin"
    case attribute.AttrASPath:
        return "as-path"
    case attribute.AttrNextHop:
        return "next-hop"
    case attribute.AttrMED:
        return "med"
    case attribute.AttrLocalPref:
        return "local-pref"
    case attribute.AttrAtomicAggregate:
        return "atomic-aggregate"
    case attribute.AttrAggregator:
        return "aggregator"
    case attribute.AttrCommunity:
        return "communities"  // Plural
    case attribute.AttrOriginatorID:
        return "originator-id"
    case attribute.AttrClusterList:
        return "cluster-list"
    case attribute.AttrExtCommunity:
        return "extended-communities"  // Plural
    case attribute.AttrLargeCommunity:
        return "large-communities"  // Plural
    default:
        return fmt.Sprintf("attr-%d", code)
    }
}
```

### FormatMessage Integration

```go
// pkg/api/text.go - modify FormatMessage

func FormatMessage(peer PeerInfo, msg RawMessage, content ContentConfig) string {
    content = content.WithDefaults()

    // V6 format ignores attribute filter
    if content.Version == APIVersionLegacy {
        return formatMessageV6(peer, msg, content)
    }

    // V7 with attribute filtering
    if content.Format == FormatParsed && msg.AttrsWire != nil && content.Attributes != nil {
        return formatParsedV7WithFilter(peer, msg, content)
    }

    // Default V7 path (no filtering)
    return formatMessageV7(peer, msg, content)
}

func formatParsedV7WithFilter(peer PeerInfo, msg RawMessage, content ContentConfig) string {
    // Decode NLRI (still need prefixes)
    decoded := DecodeUpdate(msg.RawBytes)

    // Apply attribute filter
    attrs, err := content.Attributes.Apply(msg.AttrsWire)
    if err != nil {
        // Log error, fall back to full decode
        return formatParsedV7(peer, msg, content)
    }

    return formatRoutesJSONv7WithAttrs(peer, msg, decoded.Announced, attrs)
}
```

## Config Migration

### Singular→Plural for Communities Only

```go
// pkg/config/migration/attributes.go

var communityAliases = map[string]string{
    "community":          "communities",
    "extended-community": "extended-communities",
    "large-community":    "large-communities",
}

// Add to transformations slice:
{
    Name:        "attributes->plural-communities",
    Description: "Convert singular community names to plural form",
    Detect:      hasSingularCommunityNames,
    Apply:       migrateCommunityNamesToPlural,
}

func hasSingularCommunityNames(tree *config.Tree) bool {
    for _, api := range tree.GetListOrdered("api") {
        if content := api.Value.GetContainer("content"); content != nil {
            if attrs := content.GetString("attributes"); attrs != "" {
                // Tokenize to avoid partial matches (word boundary)
                tokens := strings.Fields(attrs)
                for _, token := range tokens {
                    if _, ok := communityAliases[strings.ToLower(token)]; ok {
                        return true
                    }
                }
            }
        }
    }
    return false
}

func migrateCommunityNamesToPlural(tree *config.Tree) (*config.Tree, error) {
    result := tree.Clone()

    for _, api := range result.GetListOrdered("api") {
        if content := api.Value.GetContainer("content"); content != nil {
            if attrs := content.GetString("attributes"); attrs != "" {
                tokens := strings.Fields(attrs)
                changed := false
                for i, token := range tokens {
                    lower := strings.ToLower(token)
                    if plural, ok := communityAliases[lower]; ok {
                        tokens[i] = plural
                        changed = true
                    }
                }
                if changed {
                    content.SetString("attributes", strings.Join(tokens, " "))
                }
            }
        }
    }

    return result, nil
}
```

## API Output Examples

### With `attributes as-path next-hop communities;`

```json
{
    "type": "update",
    "update-id": 12345,
    "peer": { "address": "10.0.0.1", "asn": 65001 },
    "announce": {
        "nlri": { "ipv4 unicast": ["192.168.1.0/24"] },
        "attributes": {
            "as-path": [65001, 65002],
            "next-hop": "10.0.0.1",
            "communities": ["65001:100", "65001:200"]
        }
    }
}
```

### With `attributes none;`

```json
{
    "type": "update",
    "update-id": 12345,
    "peer": { "address": "10.0.0.1", "asn": 65001 },
    "announce": {
        "nlri": { "ipv4 unicast": ["192.168.1.0/24"] }
    }
}
```

No `"attributes"` key when filter is `none`. `update-id` always present.

### Empty Attribute Case

If `attributes as-path;` configured but UPDATE has no AS_PATH:

```json
{
    "type": "update",
    "update-id": 12345,
    "peer": { "address": "10.0.0.1", "asn": 65001 },
    "announce": {
        "nlri": { "ipv4 unicast": ["192.168.1.0/24"] }
    }
}
```

**No `"attributes"` key when empty.** Omit rather than include `"attributes": {}`.

## Testing Strategy

### Unit Tests (Primary)

The functional test framework tests ZeBGP→peer direction, not peer→ZeBGP→API.
Use unit tests for attribute filtering:

```go
// pkg/api/filter_test.go

// TestAttributeFilterModeAll verifies all attributes included.
// VALIDATES: Apply() with FilterModeAll returns all attrs as map.
// PREVENTS: Wrong mode, missing conversion from []Attribute to map.
func TestAttributeFilterModeAll(t *testing.T)

// TestAttributeFilterModeNone verifies no attributes included.
// VALIDATES: Apply() with FilterModeNone returns nil.
// PREVENTS: Accidental attribute leakage.
func TestAttributeFilterModeNone(t *testing.T)

// TestAttributeFilterModeSelective verifies specific codes included.
// VALIDATES: Only requested codes returned.
// PREVENTS: Extra or missing attributes.
func TestAttributeFilterModeSelective(t *testing.T)

// TestAttributeFilterAllReturnsMap verifies All() conversion.
// VALIDATES: []Attribute correctly converted to map[Code]Attribute.
// PREVENTS: Type mismatch with AttributesWire.All().
func TestAttributeFilterAllReturnsMap(t *testing.T)

// TestAttributeFilterNilWire verifies nil AttrsWire handling.
// VALIDATES: Apply() returns nil,nil when wire is nil.
// PREVENTS: Nil pointer panic.
func TestAttributeFilterNilWire(t *testing.T)

// TestAttributeFilterEmptyResult verifies empty result handling.
// VALIDATES: Requested attr not present -> nil map, not empty map.
// PREVENTS: Empty "attributes": {} in output.
func TestAttributeFilterEmptyResult(t *testing.T)

// TestAttributeFilterConcurrent verifies thread safety.
// VALIDATES: Multiple goroutines can call Apply() safely.
// PREVENTS: Race conditions (relies on AttributesWire internal mutex).
func TestAttributeFilterConcurrent(t *testing.T)
```

```go
// pkg/config/api_test.go

// TestParseAttributeFilterValid verifies valid parsing.
// VALIDATES: Known names map to correct codes.
// PREVENTS: Typos causing silent failures.
func TestParseAttributeFilterValid(t *testing.T)

// TestParseAttributeFilterNumeric verifies attr-N syntax.
// VALIDATES: attr-99 parses to code 99.
// PREVENTS: Unknown attributes inaccessible.
func TestParseAttributeFilterNumeric(t *testing.T)

// TestParseAttributeFilterInvalid verifies error handling.
// VALIDATES: Unknown names rejected with helpful error.
// PREVENTS: Silent acceptance of typos.
func TestParseAttributeFilterInvalid(t *testing.T)

// TestParseAttributeFilterCaseInsensitive verifies case handling.
// VALIDATES: "AS-PATH" == "as-path".
// PREVENTS: Case sensitivity surprises.
func TestParseAttributeFilterCaseInsensitive(t *testing.T)

// TestParseAttributeFilterDedupe verifies duplicate handling.
// VALIDATES: "as-path as-path" deduplicates silently.
// PREVENTS: Duplicate entries in codes slice.
func TestParseAttributeFilterDedupe(t *testing.T)

// TestParseAttributeFilterCommunityCompat verifies backward compat.
// VALIDATES: "community" accepted, maps to AttrCommunity.
// PREVENTS: Breaking old configs.
func TestParseAttributeFilterCommunityCompat(t *testing.T)
```

```go
// pkg/api/text_test.go

// TestFormatRoutesJSONv7WithAttrs verifies filtered output.
// VALIDATES: Only specified attributes appear in JSON.
// PREVENTS: Attribute leakage.
func TestFormatRoutesJSONv7WithAttrs(t *testing.T)

// TestFormatRoutesJSONv7WithAttrsEmpty verifies empty handling.
// VALIDATES: No "attributes" key when attrs is nil/empty.
// PREVENTS: Empty object in output.
func TestFormatRoutesJSONv7WithAttrsEmpty(t *testing.T)

// TestFormatRoutesJSONv7WithAttrsOrder verifies deterministic output.
// VALIDATES: Attributes output in code order.
// PREVENTS: Non-deterministic JSON.
func TestFormatRoutesJSONv7WithAttrsOrder(t *testing.T)
```

```go
// pkg/api/decode_test.go

// TestExtractAttributeBytes verifies extraction.
// VALIDATES: Correct byte range returned.
// PREVENTS: Off-by-one errors.
func TestExtractAttributeBytes(t *testing.T)

// TestExtractAttributeBytesEmpty verifies empty handling.
// VALIDATES: Returns nil for no attributes.
// PREVENTS: Empty slice vs nil confusion.
func TestExtractAttributeBytesEmpty(t *testing.T)

// TestExtractAttributeBytesMalformed verifies error handling.
// VALIDATES: Returns nil for malformed body.
// PREVENTS: Panic on bad input.
func TestExtractAttributeBytesMalformed(t *testing.T)
```

```go
// pkg/config/migration/attributes_test.go

// TestMigrateCommunityNamesToPlural verifies migration.
// VALIDATES: "community" -> "communities" in config.
// PREVENTS: Old configs breaking.
func TestMigrateCommunityNamesToPlural(t *testing.T)

// TestMigrationWordBoundary verifies no false matches.
// VALIDATES: "extended-community" doesn't match "community".
// PREVENTS: Double transformation corruption.
func TestMigrationWordBoundary(t *testing.T)
```

### Integration Test (Manual)

For end-to-end validation, manually test with real BGP session:

1. Start ZeBGP with attribute filter config
2. Connect BGP peer (e.g., GoBGP, ExaBGP)
3. Announce route with multiple attributes
4. Verify API output has only filtered attributes
5. Verify update-id present and forwarding works

## Implementation Steps

1. Write test for ExtractAttributeBytes (TDD)
2. See test FAIL
3. Implement `pkg/api/decode.go:ExtractAttributeBytes()`
4. See test PASS
5. Write test for AttributeFilter type (TDD)
6. See test FAIL
7. Implement `pkg/api/filter.go`
8. See test PASS
9. Write test for parseAttributeFilter (TDD)
10. See test FAIL
11. Implement config parsing
12. See test PASS
13. Write test for formatRoutesJSONv7WithAttrs (TDD)
14. See test FAIL
15. Implement text.go changes
16. See test PASS
17. Add ContentConfig.Attributes field
18. Add RawMessage.AttrsWire field
19. Modify reactor to create AttrsWire
20. Add migration transformation
21. Run `make test && make lint && make functional`

## Checklist

- [ ] Required docs read
- [ ] `ExtractAttributeBytes()` function
- [ ] `AttributeFilter` type with `FilterMode` enum (NO mutex)
- [ ] `NewFilterAll()`, `NewFilterNone()`, `NewFilterSelective()` constructors
- [ ] `AttributeFilter.Apply()` - converts All() slice to map
- [ ] `parseAttributeFilter()` function
- [ ] `attr-N` numeric syntax support
- [ ] Case-insensitive parsing
- [ ] Duplicate deduplication
- [ ] `community` → `communities` backward compat in config
- [ ] Error message with valid names list
- [ ] `RawMessage.AttrsWire` field
- [ ] `ContentConfig.Attributes` field
- [ ] Reactor creates AttrsWire from ExtractAttributeBytes
- [ ] `formatRoutesJSONv7WithAttrs()` function
- [ ] `attributeCodeToJSONKey()` mapping
- [ ] `FormatMessage` uses filter for V7 parsed format
- [ ] Empty attrs → no "attributes" key (not empty object)
- [ ] Migration: `attributes->plural-communities` (word boundary safe)
- [ ] Tests for ExtractAttributeBytes
- [ ] Tests for filter type (TDD)
- [ ] Tests for All() → map conversion
- [ ] Tests for parsing (TDD)
- [ ] Tests for numeric attr-N
- [ ] Tests for concurrent access
- [ ] Tests for migration word boundary
- [ ] Tests for nil wire handling
- [ ] Tests for empty result → nil
- [ ] Tests for JSON output formatting
- [ ] Tests for deterministic attribute order
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] `make functional` passes
- [ ] Update `.claude/zebgp/api/ARCHITECTURE.md` with attributes config

---

**Created:** 2026-01-01
**Revised:** 2026-01-02
