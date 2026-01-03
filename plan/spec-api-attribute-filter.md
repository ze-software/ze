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
| `pkg/api/types.go` | Add `AttrsWire` to RawMessage, `Attributes` to ContentConfig |
| `pkg/api/types.go` | REMOVE: `ContentConfig.Version`, `PeerAPIBinding.Version`, `APIVersionLegacy`, `APIVersionNLRI` |
| `pkg/api/text.go` | REMOVE: `formatMessageV6()`, `formatMessageV7()` and related |
| `pkg/api/text.go` | ADD: `formatGroupedJSON()`, `formatParsedText()`, etc. |
| `pkg/api/decode.go` | Add `ExtractAttributeBytes()` helper |
| `pkg/bgp/attribute/wire.go` | Add `GetMPReachPrefixes()`, `GetMPUnreachPrefixes()` if not present |
| `pkg/config/api.go` | Add `parseAttributeFilter()` |
| `pkg/config/bgp.go` | Add `Attributes` to PeerContentConfig |
| `pkg/reactor/reactor.go` | Create AttrsWire, pass in RawMessage |
| `pkg/api/*_test.go` | Update tests to remove Version references |

## Problem

Current API outputs ALL parsed attributes, even if external process only needs a few.

## Solution

Config option in content block:

```
api foo {
    content {
        encoding json;
        attribute as-path next-hop communities;  # Only parse/output these
        nlri ipv4 unicast;                       # Only include IPv4 unicast
        nlri ipv6 unicast;                       # Also include IPv6 unicast
    }
    receive { update; }
}
```

## Limitations

1. **Received UPDATEs only:** Filter applies to UPDATEs received from peers. API-originated announcements are unaffected.
2. **Unicast only:** Grouped format currently handles IPv4/IPv6 unicast. Other SAFIs (mpls-vpn, flowspec, evpn, etc.) deferred to future work.

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
| `community` / `communities` | 8 | COMMUNITIES |
| `originator-id` | 9 | ORIGINATOR_ID |
| `cluster-list` | 10 | CLUSTER_LIST |
| `extended-community` / `extended-communities` | 16 | EXTENDED_COMMUNITIES |
| `large-community` / `large-communities` | 32 | LARGE_COMMUNITIES |
| `attr-N` | N | Unknown/numeric attribute (e.g., `attr-99`) |
| `all` | - | All attributes (default) |
| `none` | - | No attributes (update-id only) |

**Rejected codes:** `attr-14` (MP_REACH_NLRI) and `attr-15` (MP_UNREACH_NLRI) are structural and MUST be rejected in config parsing with error: `"attr-14 (MP_REACH_NLRI) is structural and cannot be filtered"`.

**Both forms accepted:** `community`/`communities`, `extended-community`/`extended-communities`, `large-community`/`large-communities` all accepted. Internally normalized to attribute codes.

### NLRI Family Names

| Config Syntax | Canonical Name |
|---------------|----------------|
| `ipv4 unicast` | ipv4 unicast |
| `ipv6 unicast` | ipv6 unicast |
| `ipv4 multicast` | ipv4 multicast |
| `ipv6 multicast` | ipv6 multicast |
| `ipv4 mpls` | ipv4 mpls |
| `ipv6 mpls` | ipv6 mpls |
| `ipv4 mpls-vpn` | ipv4 mpls-vpn |
| `ipv6 mpls-vpn` | ipv6 mpls-vpn |
| `ipv4 flowspec` | ipv4 flowspec |
| `ipv6 flowspec` | ipv6 flowspec |
| `l2vpn evpn` | l2vpn evpn |
| `l2vpn vpls` | l2vpn vpls |
| `all` | All families (default) |
| `none` | No families |

Multiple `nlri` statements can be used to select multiple families:
```
nlri ipv4 unicast;
nlri ipv6 unicast;
```

## JSON Output Format

### Grouped Format (ZeBGP Extension)

This format groups NLRI by address family with shared attributes, matching BGP UPDATE wire semantics. **Not ExaBGP compatible.**

```json
{
    "type": "update",
    "update-id": 12345,
    "peer": {"address": "10.0.0.1", "asn": 65001},
    "announce": {
        "ipv4 unicast": ["192.168.1.0/24", "192.168.2.0/24"],
        "attributes": {
            "next-hop": "10.0.0.1",
            "origin": "igp",
            "as-path": [65001, 65002],
            "communities": ["65001:100"]
        }
    },
    "withdraw": {
        "ipv4 unicast": ["192.168.3.0/24"]
    }
}
```

**Structure:**
- `update-id`: Always present for received UPDATEs (for forwarding)
- `announce.<family>`: Array of prefix strings (not objects)
- `announce.attributes`: Shared attributes for all announced NLRI
- `withdraw.<family>`: Array of prefix strings (no attributes)

### JSON Output Key Names

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

// FilterResult contains filtered attributes and NLRI from a single Apply call.
// No caching - computed fresh each call.
type FilterResult struct {
    Attributes map[attribute.AttributeCode]attribute.Attribute
    Announced  []netip.Prefix  // IPv4 from body + IPv6 from MP_REACH_NLRI
    Withdrawn  []netip.Prefix  // IPv4 from body + IPv6 from MP_UNREACH_NLRI
}

// IsEOR returns true if this is an End-of-RIB marker (no NLRI).
func (r FilterResult) IsEOR() bool {
    return len(r.Announced) == 0 && len(r.Withdrawn) == 0
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

// Apply returns filtered attributes AND NLRI in one call.
// Extracts NLRI from body (IPv4) and MP_REACH/MP_UNREACH attributes (IPv6).
// No duplicate parsing - attributes parsed once via AttrsWire.
func (f AttributeFilter) Apply(body []byte, wire *attribute.AttributesWire) (FilterResult, error) {
    result := FilterResult{}

    // Extract NLRI (IPv4 from body structure, IPv6 from MP attributes)
    result.Announced, result.Withdrawn = extractNLRI(body, wire)

    if wire == nil {
        return result, nil
    }

    switch f.Mode {
    case FilterModeNone:
        return result, nil

    case FilterModeAll:
        // All() returns []Attribute, convert to map
        attrs, err := wire.All()
        if err != nil {
            return result, err
        }
        if len(attrs) > 0 {
            result.Attributes = make(map[attribute.AttributeCode]attribute.Attribute, len(attrs))
            for _, attr := range attrs {
                result.Attributes[attr.Code()] = attr
            }
        }
        return result, nil

    case FilterModeSelective:
        // GetMultiple() returns map directly
        attrs, err := wire.GetMultiple(f.Codes)
        if err != nil {
            return result, err
        }
        if len(attrs) > 0 {
            result.Attributes = attrs
        }
        return result, nil

    default:
        return result, fmt.Errorf("unknown filter mode: %d", f.Mode)
    }
}

// extractNLRI extracts prefixes from UPDATE body and MP attributes.
// IPv4 NLRI/withdrawn from body structure, IPv6 from MP_REACH/MP_UNREACH.
func extractNLRI(body []byte, wire *attribute.AttributesWire) (announced, withdrawn []netip.Prefix) {
    if len(body) < 4 {
        return nil, nil
    }

    // Parse UPDATE structure for IPv4
    withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
    offset := 2

    // IPv4 withdrawn
    if withdrawnLen > 0 && offset+withdrawnLen <= len(body) {
        withdrawn = parseIPv4Prefixes(body[offset : offset+withdrawnLen])
    }
    offset += withdrawnLen

    if offset+2 > len(body) {
        return announced, withdrawn
    }

    attrLen := int(binary.BigEndian.Uint16(body[offset : offset+2]))
    offset += 2
    nlriOffset := offset + attrLen

    // IPv4 NLRI (after attributes)
    if nlriOffset < len(body) {
        announced = parseIPv4Prefixes(body[nlriOffset:])
    }

    // IPv6 from MP attributes (if wire available)
    if wire != nil {
        if mpReach := wire.GetMPReachPrefixes(); len(mpReach) > 0 {
            announced = append(announced, mpReach...)
        }
        if mpUnreach := wire.GetMPUnreachPrefixes(); len(mpUnreach) > 0 {
            withdrawn = append(withdrawn, mpUnreach...)
        }
    }

    return announced, withdrawn
}
```

### Required AttributesWire Methods

The `extractNLRI` function requires these methods on `AttributesWire`:

```go
// pkg/bgp/attribute/wire.go - add if not present

// GetMPReachPrefixes returns IPv6 prefixes from MP_REACH_NLRI attribute.
// Returns nil if attribute not present or not IPv6 unicast.
func (w *AttributesWire) GetMPReachPrefixes() []netip.Prefix

// GetMPUnreachPrefixes returns IPv6 prefixes from MP_UNREACH_NLRI attribute.
// Returns nil if attribute not present or not IPv6 unicast.
func (w *AttributesWire) GetMPUnreachPrefixes() []netip.Prefix
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

// structuralAttributes cannot be filtered (MP_REACH/UNREACH).
var structuralAttributes = map[attribute.AttributeCode]string{
    attribute.AttrMPReachNLRI:   "MP_REACH_NLRI",
    attribute.AttrMPUnreachNLRI: "MP_UNREACH_NLRI",
}

var attributeNameToCode = map[string]attribute.AttributeCode{
    "origin":               attribute.AttrOrigin,
    "as-path":              attribute.AttrASPath,
    "next-hop":             attribute.AttrNextHop,
    "med":                  attribute.AttrMED,
    "local-pref":           attribute.AttrLocalPref,
    "atomic-aggregate":     attribute.AttrAtomicAggregate,
    "aggregator":           attribute.AttrAggregator,
    "community":            attribute.AttrCommunity,       // Singular
    "communities":          attribute.AttrCommunity,       // Plural (both accepted)
    "originator-id":        attribute.AttrOriginatorID,
    "cluster-list":         attribute.AttrClusterList,
    "extended-community":   attribute.AttrExtCommunity,    // Singular
    "extended-communities": attribute.AttrExtCommunity,    // Plural (both accepted)
    "large-community":      attribute.AttrLargeCommunity,  // Singular
    "large-communities":    attribute.AttrLargeCommunity,  // Plural (both accepted)
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

            // Reject structural attributes
            if structName, ok := structuralAttributes[code]; ok {
                return api.AttributeFilter{}, fmt.Errorf("attr-%d (%s) is structural and cannot be filtered", num, structName)
            }

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
    Type      message.MessageType
    RawBytes  []byte
    Timestamp time.Time
    UpdateID  uint64
    AttrsWire *attribute.AttributesWire  // NEW: for lazy attribute parsing
}
```

### ContentConfig Cleanup

```go
// pkg/api/types.go - modify ContentConfig
// REMOVE: Version field, APIVersionLegacy, APIVersionNLRI constants

type ContentConfig struct {
    Encoding   string           // "json" | "text" (default: "text")
    Format     string           // "parsed" | "raw" | "full" (default: "parsed")
    Attributes *AttributeFilter // NEW: nil means all (default)
}

// REMOVE these constants:
// APIVersionLegacy = 6
// APIVersionNLRI   = 7
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

### Grouped JSON Formatter

```go
// pkg/api/text.go

// formatGroupedJSON formats UPDATE in grouped format (ZeBGP extension).
// NLRI grouped by family as string arrays, attributes shared at announce level.
// Empty UPDATE (no announce, no withdraw) is formatted as EOR.
func formatGroupedJSON(peer PeerInfo, msg RawMessage, fr FilterResult) string {
    var sb strings.Builder

    // Empty UPDATE = End-of-RIB marker
    if fr.IsEOR() {
        sb.WriteString(`{"type":"eor"`)
        if msg.UpdateID != 0 {
            sb.WriteString(fmt.Sprintf(`,"update-id":%d`, msg.UpdateID))
        }
        sb.WriteString(fmt.Sprintf(`,"peer":{"address":"%s","asn":%d}`, peer.Address, peer.PeerAS))
        sb.WriteString("}\n")
        return sb.String()
    }

    sb.WriteString(`{"type":"update"`)

    if msg.UpdateID != 0 {
        sb.WriteString(fmt.Sprintf(`,"update-id":%d`, msg.UpdateID))
    }

    sb.WriteString(fmt.Sprintf(`,"peer":{"address":"%s","asn":%d}`, peer.Address, peer.PeerAS))

    // Announce section
    if len(fr.Announced) > 0 {
        sb.WriteString(`,"announce":{`)
        formatGroupedPrefixes(&sb, fr.Announced)

        // Shared attributes
        if len(fr.Attributes) > 0 {
            sb.WriteString(`,"attributes":{`)
            formatAttributesJSON(&sb, fr.Attributes)
            sb.WriteString(`}`)
        }
        sb.WriteString(`}`)
    }

    // Withdraw section
    if len(fr.Withdrawn) > 0 {
        sb.WriteString(`,"withdraw":{`)
        formatGroupedPrefixes(&sb, fr.Withdrawn)
        sb.WriteString(`}`)
    }

    sb.WriteString("}\n")
    return sb.String()
}

// formatGroupedPrefixes writes prefixes grouped by address family as string arrays.
// Used for both announced and withdrawn.
func formatGroupedPrefixes(sb *strings.Builder, prefixes []netip.Prefix) {
    ipv4 := make([]string, 0)
    ipv6 := make([]string, 0)
    for _, p := range prefixes {
        if p.Addr().Is6() {
            ipv6 = append(ipv6, p.String())
        } else {
            ipv4 = append(ipv4, p.String())
        }
    }

    needComma := false
    if len(ipv4) > 0 {
        sb.WriteString(`"ipv4 unicast":`)
        writeStringArray(sb, ipv4)
        needComma = true
    }
    if len(ipv6) > 0 {
        if needComma {
            sb.WriteString(",")
        }
        sb.WriteString(`"ipv6 unicast":`)
        writeStringArray(sb, ipv6)
    }
}

// writeStringArray writes a JSON array of strings.
func writeStringArray(sb *strings.Builder, items []string) {
    sb.WriteString("[")
    for i, item := range items {
        if i > 0 {
            sb.WriteString(",")
        }
        sb.WriteString(fmt.Sprintf(`"%s"`, item))
    }
    sb.WriteString("]")
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

// writeAttributeValue writes the JSON value for an attribute.
func writeAttributeValue(sb *strings.Builder, attr attribute.Attribute) {
    switch a := attr.(type) {
    case *attribute.Origin:
        sb.WriteString(fmt.Sprintf(`"%s"`, strings.ToLower(a.String())))

    case *attribute.ASPath:
        sb.WriteString("[")
        first := true
        for _, seg := range a.Segments {
            for _, asn := range seg.ASNs {
                if !first {
                    sb.WriteString(",")
                }
                first = false
                sb.WriteString(fmt.Sprintf("%d", asn))
            }
        }
        sb.WriteString("]")

    case *attribute.NextHop:
        sb.WriteString(fmt.Sprintf(`"%s"`, a.Addr))

    case *attribute.MED:
        sb.WriteString(fmt.Sprintf("%d", uint32(*a)))

    case *attribute.LocalPref:
        sb.WriteString(fmt.Sprintf("%d", uint32(*a)))

    case *attribute.AtomicAggregate:
        sb.WriteString("true")

    case *attribute.Aggregator:
        sb.WriteString(fmt.Sprintf(`{"asn":%d,"address":"%s"}`, a.ASN, a.Address))

    case *attribute.Communities:
        sb.WriteString("[")
        for i, c := range *a {
            if i > 0 {
                sb.WriteString(",")
            }
            // Format as "asn:value" or well-known name
            sb.WriteString(fmt.Sprintf(`"%s"`, c.String()))
        }
        sb.WriteString("]")

    case *attribute.OriginatorID:
        sb.WriteString(fmt.Sprintf(`"%s"`, a.String()))

    case *attribute.ClusterList:
        sb.WriteString("[")
        for i, id := range *a {
            if i > 0 {
                sb.WriteString(",")
            }
            sb.WriteString(fmt.Sprintf(`"%s"`, id.String()))
        }
        sb.WriteString("]")

    case *attribute.ExtendedCommunities:
        sb.WriteString("[")
        for i, ec := range *a {
            if i > 0 {
                sb.WriteString(",")
            }
            sb.WriteString(fmt.Sprintf(`"%s"`, ec.String()))
        }
        sb.WriteString("]")

    case *attribute.LargeCommunities:
        sb.WriteString("[")
        for i, lc := range *a {
            if i > 0 {
                sb.WriteString(",")
            }
            sb.WriteString(fmt.Sprintf(`"%s"`, lc.String()))
        }
        sb.WriteString("]")

    case *attribute.OpaqueAttribute:
        // Unknown attribute: output as hex
        sb.WriteString(fmt.Sprintf(`"%x"`, a.Value()))

    default:
        // Fallback: try String() method
        if s, ok := attr.(fmt.Stringer); ok {
            sb.WriteString(fmt.Sprintf(`"%s"`, s.String()))
        } else {
            sb.WriteString(`null`)
        }
    }
}
```

### FormatMessage Integration

```go
// pkg/api/text.go - modify FormatMessage

func FormatMessage(peer PeerInfo, msg RawMessage, content ContentConfig) string {
    content = content.WithDefaults()

    // Apply filter - returns both attributes AND NLRI in one call
    filter := content.Attributes
    if filter == nil {
        filter = &AttributeFilter{Mode: FilterModeAll}
    }

    fr, err := filter.Apply(msg.RawBytes, msg.AttrsWire)
    if err != nil {
        // Log error, continue with empty result
        fr = FilterResult{}
    }

    // JSON encoding uses grouped format
    if content.Encoding == EncodingJSON {
        switch content.Format {
        case FormatRaw:
            return formatRawJSON(peer, msg)
        case FormatFull:
            return formatFullJSON(peer, msg, fr)
        default: // FormatParsed
            return formatGroupedJSON(peer, msg, fr)
        }
    }

    // Text encoding
    switch content.Format {
    case FormatRaw:
        return formatRawText(peer, msg)
    case FormatFull:
        return formatFullText(peer, msg, fr)
    default: // FormatParsed
        return formatParsedText(peer, msg, fr)
    }
}
```

### Raw and Full JSON Formats

```go
// pkg/api/text.go

// formatRawJSON outputs UPDATE as hex bytes in JSON wrapper.
func formatRawJSON(peer PeerInfo, msg RawMessage) string {
    var sb strings.Builder
    sb.WriteString(`{"type":"update"`)

    if msg.UpdateID != 0 {
        sb.WriteString(fmt.Sprintf(`,"update-id":%d`, msg.UpdateID))
    }

    sb.WriteString(fmt.Sprintf(`,"peer":{"address":"%s","asn":%d}`, peer.Address, peer.PeerAS))
    sb.WriteString(fmt.Sprintf(`,"raw":"%x"`, msg.RawBytes))
    sb.WriteString("}\n")
    return sb.String()
}

// formatFullJSON combines parsed grouped format with raw bytes.
func formatFullJSON(peer PeerInfo, msg RawMessage, fr FilterResult) string {
    var sb strings.Builder

    // EOR with raw
    if fr.IsEOR() {
        sb.WriteString(`{"type":"eor"`)
        if msg.UpdateID != 0 {
            sb.WriteString(fmt.Sprintf(`,"update-id":%d`, msg.UpdateID))
        }
        sb.WriteString(fmt.Sprintf(`,"peer":{"address":"%s","asn":%d}`, peer.Address, peer.PeerAS))
        sb.WriteString(fmt.Sprintf(`,"raw":"%x"`, msg.RawBytes))
        sb.WriteString("}\n")
        return sb.String()
    }

    sb.WriteString(`{"type":"update"`)

    if msg.UpdateID != 0 {
        sb.WriteString(fmt.Sprintf(`,"update-id":%d`, msg.UpdateID))
    }

    sb.WriteString(fmt.Sprintf(`,"peer":{"address":"%s","asn":%d}`, peer.Address, peer.PeerAS))

    // Announce section
    if len(fr.Announced) > 0 {
        sb.WriteString(`,"announce":{`)
        formatGroupedPrefixes(&sb, fr.Announced)
        if len(fr.Attributes) > 0 {
            sb.WriteString(`,"attributes":{`)
            formatAttributesJSON(&sb, fr.Attributes)
            sb.WriteString(`}`)
        }
        sb.WriteString(`}`)
    }

    // Withdraw section
    if len(fr.Withdrawn) > 0 {
        sb.WriteString(`,"withdraw":{`)
        formatGroupedPrefixes(&sb, fr.Withdrawn)
        sb.WriteString(`}`)
    }

    // Add raw bytes
    sb.WriteString(fmt.Sprintf(`,"raw":"%x"`, msg.RawBytes))

    sb.WriteString("}\n")
    return sb.String()
}
```

### Text Format Definitions

```go
// pkg/api/text.go

// formatParsedText formats UPDATE as human-readable text.
// Format: peer <ip> update announce <family> <prefix> [<prefix>...] attributes <attr>=<value> ...
func formatParsedText(peer PeerInfo, msg RawMessage, fr FilterResult) string {
    var sb strings.Builder

    // EOR
    if fr.IsEOR() {
        return fmt.Sprintf("peer %s eor\n", peer.Address)
    }

    // Announce
    if len(fr.Announced) > 0 {
        sb.WriteString(fmt.Sprintf("peer %s update announce", peer.Address))

        // Group by family
        ipv4, ipv6 := groupByFamily(fr.Announced)
        if len(ipv4) > 0 {
            sb.WriteString(" ipv4-unicast")
            for _, p := range ipv4 {
                sb.WriteString(fmt.Sprintf(" %s", p))
            }
        }
        if len(ipv6) > 0 {
            sb.WriteString(" ipv6-unicast")
            for _, p := range ipv6 {
                sb.WriteString(fmt.Sprintf(" %s", p))
            }
        }

        // Attributes
        if len(fr.Attributes) > 0 {
            sb.WriteString(" attributes")
            writeAttributesText(&sb, fr.Attributes)
        }
        sb.WriteString("\n")
    }

    // Withdraw
    if len(fr.Withdrawn) > 0 {
        sb.WriteString(fmt.Sprintf("peer %s update withdraw", peer.Address))
        ipv4, ipv6 := groupByFamily(fr.Withdrawn)
        if len(ipv4) > 0 {
            sb.WriteString(" ipv4-unicast")
            for _, p := range ipv4 {
                sb.WriteString(fmt.Sprintf(" %s", p))
            }
        }
        if len(ipv6) > 0 {
            sb.WriteString(" ipv6-unicast")
            for _, p := range ipv6 {
                sb.WriteString(fmt.Sprintf(" %s", p))
            }
        }
        sb.WriteString("\n")
    }

    return sb.String()
}

// groupByFamily splits prefixes into IPv4 and IPv6 strings.
func groupByFamily(prefixes []netip.Prefix) (ipv4, ipv6 []string) {
    for _, p := range prefixes {
        if p.Addr().Is6() {
            ipv6 = append(ipv6, p.String())
        } else {
            ipv4 = append(ipv4, p.String())
        }
    }
    return ipv4, ipv6
}

// writeAttributesText writes attributes in text format: key=value key=value ...
func writeAttributesText(sb *strings.Builder, attrs map[attribute.AttributeCode]attribute.Attribute) {
    codes := sortedCodes(attrs)
    for _, code := range codes {
        attr := attrs[code]
        key := attributeCodeToTextKey(code)
        value := attributeToTextValue(attr)
        sb.WriteString(fmt.Sprintf(" %s=%s", key, value))
    }
}

// attributeToTextValue formats attribute value for text output.
func attributeToTextValue(attr attribute.Attribute) string {
    switch a := attr.(type) {
    case *attribute.Origin:
        return strings.ToLower(a.String())
    case *attribute.ASPath:
        var parts []string
        for _, seg := range a.Segments {
            for _, asn := range seg.ASNs {
                parts = append(parts, fmt.Sprintf("%d", asn))
            }
        }
        return strings.Join(parts, ",")
    case *attribute.NextHop:
        return a.Addr.String()
    case *attribute.MED:
        return fmt.Sprintf("%d", uint32(*a))
    case *attribute.LocalPref:
        return fmt.Sprintf("%d", uint32(*a))
    case *attribute.Communities:
        var parts []string
        for _, c := range *a {
            parts = append(parts, c.String())
        }
        return strings.Join(parts, ",")
    default:
        return attr.(fmt.Stringer).String()
    }
}

// formatRawText formats UPDATE as hex dump.
func formatRawText(peer PeerInfo, msg RawMessage) string {
    return fmt.Sprintf("peer %s update raw %x\n", peer.Address, msg.RawBytes)
}

// formatFullText combines parsed and raw.
func formatFullText(peer PeerInfo, msg RawMessage, fr FilterResult) string {
    parsed := formatParsedText(peer, msg, fr)
    raw := formatRawText(peer, msg)
    return parsed + raw
}
```

**Text Output Examples:**

```
peer 10.0.0.1 update announce ipv4-unicast 192.168.1.0/24 192.168.2.0/24 attributes origin=igp as-path=65001,65002 next-hop=10.0.0.1
peer 10.0.0.1 update withdraw ipv4-unicast 192.168.3.0/24
peer 10.0.0.1 eor
```

## API Output Examples

### Default (all attributes)

```json
{
    "type": "update",
    "update-id": 12345,
    "peer": {"address": "10.0.0.1", "asn": 65001},
    "announce": {
        "ipv4 unicast": ["192.168.1.0/24", "192.168.2.0/24"],
        "attributes": {
            "origin": "igp",
            "as-path": [65001, 65002],
            "next-hop": "10.0.0.1",
            "local-pref": 100,
            "communities": ["65001:100", "65001:200"]
        }
    }
}
```

### With `attributes as-path next-hop communities;`

```json
{
    "type": "update",
    "update-id": 12345,
    "peer": {"address": "10.0.0.1", "asn": 65001},
    "announce": {
        "ipv4 unicast": ["192.168.1.0/24"],
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
    "peer": {"address": "10.0.0.1", "asn": 65001},
    "announce": {
        "ipv4 unicast": ["192.168.1.0/24"]
    }
}
```

No `"attributes"` key when filter is `none`. `update-id` always present.

### With withdrawals

```json
{
    "type": "update",
    "update-id": 12346,
    "peer": {"address": "10.0.0.1", "asn": 65001},
    "announce": {
        "ipv4 unicast": ["192.168.1.0/24"],
        "attributes": {
            "next-hop": "10.0.0.1",
            "origin": "igp"
        }
    },
    "withdraw": {
        "ipv4 unicast": ["192.168.3.0/24"]
    }
}
```

### Withdraw only

```json
{
    "type": "update",
    "update-id": 12347,
    "peer": {"address": "10.0.0.1", "asn": 65001},
    "withdraw": {
        "ipv4 unicast": ["192.168.2.0/24", "192.168.3.0/24"]
    }
}
```

### Empty Attribute Case

If `attributes as-path;` configured but UPDATE has no AS_PATH:

```json
{
    "type": "update",
    "update-id": 12345,
    "peer": {"address": "10.0.0.1", "asn": 65001},
    "announce": {
        "ipv4 unicast": ["192.168.1.0/24"]
    }
}
```

**No `"attributes"` key when empty.** Omit rather than include `"attributes": {}`.

### End-of-RIB (EOR)

Empty UPDATE (no announcements, no withdrawals) indicates End-of-RIB:

```json
{
    "type": "eor",
    "update-id": 12348,
    "peer": {"address": "10.0.0.1", "asn": 65001}
}
```

Type is `"eor"` not `"update"` for clarity.

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

// TestParseAttributeFilterStructuralRejected verifies MP_REACH/UNREACH rejected.
// VALIDATES: attr-14 and attr-15 fail config parsing.
// PREVENTS: Structural attributes being filtered.
func TestParseAttributeFilterStructuralRejected(t *testing.T)

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

// TestFormatGroupedJSON verifies grouped output format.
// VALIDATES: NLRI as string arrays, attributes at announce level.
// PREVENTS: Wrong structure.
func TestFormatGroupedJSON(t *testing.T)

// TestFormatGroupedJSONWithFilter verifies filtered output.
// VALIDATES: Only specified attributes appear in JSON.
// PREVENTS: Attribute leakage.
func TestFormatGroupedJSONWithFilter(t *testing.T)

// TestFormatGroupedJSONEmpty verifies empty handling.
// VALIDATES: No "attributes" key when attrs is nil/empty.
// PREVENTS: Empty object in output.
func TestFormatGroupedJSONEmpty(t *testing.T)

// TestFormatGroupedJSONOrder verifies deterministic output.
// VALIDATES: Attributes output in code order.
// PREVENTS: Non-deterministic JSON.
func TestFormatGroupedJSONOrder(t *testing.T)

// TestFormatGroupedJSONWithdraw verifies withdraw format.
// VALIDATES: Withdrawals grouped by family, no attributes.
// PREVENTS: Wrong withdraw structure.
func TestFormatGroupedJSONWithdraw(t *testing.T)
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
// pkg/api/text_test.go

// TestWriteAttributeValue verifies JSON attribute serialization.
// VALIDATES: Each attribute type serializes correctly.
// PREVENTS: Malformed JSON output.
func TestWriteAttributeValue(t *testing.T)

// TestAttributeToTextValue verifies text attribute formatting.
// VALIDATES: Text output is parseable and concise.
// PREVENTS: Broken text format.
func TestAttributeToTextValue(t *testing.T)
```

```go
// pkg/config/api_test.go

// TestParseAttributeFilterBothForms verifies singular/plural accepted.
// VALIDATES: "community" and "communities" both map to AttrCommunity.
// PREVENTS: Unnecessarily strict config parsing.
func TestParseAttributeFilterBothForms(t *testing.T)
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
11. Implement config parsing (including attr-14/15 rejection)
12. See test PASS
13. Write test for formatGroupedJSON (TDD)
14. See test FAIL
15. Implement text.go changes
16. See test PASS
17. Add ContentConfig.Attributes field
18. Add RawMessage.AttrsWire field
19. Modify reactor to create AttrsWire
20. Run `make test && make lint && make functional`

## Checklist

- [ ] Required docs read
- [ ] `ExtractAttributeBytes()` function
- [ ] `AttributeFilter` type with `FilterMode` enum (NO mutex)
- [ ] `NewFilterAll()`, `NewFilterNone()`, `NewFilterSelective()` constructors
- [ ] `FilterResult` type with Attributes, Announced, Withdrawn
- [ ] `FilterResult.IsEOR()` method
- [ ] `AttributeFilter.Apply(body, wire)` - returns FilterResult (attrs + NLRI)
- [ ] `extractNLRI()` - IPv4 from body, IPv6 from MP attrs
- [ ] `AttributesWire.GetMPReachPrefixes()` method
- [ ] `AttributesWire.GetMPUnreachPrefixes()` method
- [ ] `parseAttributeFilter()` function
- [ ] `attr-N` numeric syntax support
- [ ] `attr-14`/`attr-15` rejected with clear error
- [ ] Case-insensitive parsing
- [ ] Duplicate deduplication
- [ ] Both singular/plural community forms accepted
- [ ] Error message with valid names list
- [ ] `RawMessage.AttrsWire` field
- [ ] `ContentConfig.Attributes` field
- [ ] REMOVE: `ContentConfig.Version` field
- [ ] REMOVE: `PeerAPIBinding.Version` field
- [ ] REMOVE: `APIVersionLegacy`, `APIVersionNLRI` constants
- [ ] REMOVE: `formatMessageV6()`, `formatMessageV7()` and related functions
- [ ] Reactor creates AttrsWire from ExtractAttributeBytes
- [ ] `formatGroupedJSON()` function (grouped format)
- [ ] `formatGroupedPrefixes()` - prefixes as string arrays by family
- [ ] `groupByFamily()` - split prefixes into IPv4/IPv6
- [ ] `formatRawJSON()`, `formatFullJSON()` - new format variants
- [ ] `formatParsedText()`, `formatRawText()`, `formatFullText()` - text variants
- [ ] `writeAttributeValue()` - JSON attribute serialization
- [ ] `attributeToTextValue()` - text attribute serialization
- [ ] `attributeCodeToJSONKey()` mapping
- [ ] `FormatMessage` uses grouped format for JSON
- [ ] Empty attrs → no "attributes" key (not empty object)
- [ ] Tests for ExtractAttributeBytes
- [ ] Tests for filter type (TDD)
- [ ] Tests for FilterResult (attrs + NLRI in one call)
- [ ] Tests for extractNLRI (IPv4 + IPv6)
- [ ] Tests for parsing (TDD)
- [ ] Tests for numeric attr-N
- [ ] Tests for structural rejection (attr-14, attr-15)
- [ ] Tests for concurrent access
- [ ] Tests for singular/plural forms both accepted
- [ ] Tests for nil wire handling
- [ ] Tests for empty result → nil
- [ ] Tests for grouped JSON format
- [ ] Tests for withdraw format
- [ ] Tests for EOR (empty UPDATE) format
- [ ] Tests for deterministic attribute order
- [ ] Tests for writeAttributeValue (each attribute type)
- [ ] Tests for text format output
- [ ] Update/remove tests referencing Version
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] `make functional` passes
- [ ] Update `.claude/zebgp/api/ARCHITECTURE.md` with attributes config

---

## Implementation Status (2026-01-02)

### Completed

- [x] `ExtractAttributeBytes()` in decode.go
- [x] `AttributeFilter` type with `FilterMode` enum (All/None/Selective)
- [x] `ParseAttributeFilter()` config parser
- [x] `FilterResult` struct with Attributes map + NLRI lists
- [x] `ApplyToUpdate()` for lazy parsing with filter
- [x] O(1) `Includes()` via codeSet map
- [x] `RawMessage.AttrsWire` field added
- [x] `ContentConfig.Attributes` field added
- [x] AttrsWire creation in reactor.go
- [x] New FilterResult-based formatters (JSON + text, v6 + v7)
- [x] FormatMessage uses AttrsWire when available

### Issues Found - MUST FIX

#### 🔴 Issue 1: String comparison for message type

**Location:** `pkg/api/text.go:169`

```go
if msg.Type.String() == "UPDATE" && msg.AttrsWire != nil {
```

**Problem:** Fragile string comparison instead of type constant.

**Fix:** Import `message` package, use `msg.Type == message.TypeUPDATE`

#### 🔴 Issue 2: formatFullFromResult JSON incomplete

**Location:** `pkg/api/text.go:228-231`

```go
if content.Encoding == EncodingJSON {
    return parsed // TODO: merge raw into JSON
}
```

**Problem:** For `format=full encoding=json`, raw bytes are NOT included in output. Regression from old behavior.

**Fix:** Merge raw hex into JSON output, e.g.:
```go
// Insert ,"raw":"hexbytes" before final }
return strings.TrimSuffix(parsed, "}\n") + fmt.Sprintf(`,"raw":"%s"}`+"\n", rawHex)
```

#### 🟡 Issue 3: Single NextHop for all prefixes

**Location:** `pkg/api/filter.go` FilterResult struct

```go
NextHop netip.Addr  // Single next-hop for all prefixes
```

**Problem:** If UPDATE has both IPv4 (NEXT_HOP attr) and IPv6 (MP_REACH next-hop) with *different* next-hops, only one is used for all prefixes in output.

**Fix:** Change FilterResult to track per-family next-hops:
```go
type FilterResult struct {
    Attributes  map[attribute.AttributeCode]attribute.Attribute
    Announced   []netip.Prefix
    Withdrawn   []netip.Prefix
    NextHopIPv4 netip.Addr  // From NEXT_HOP attribute
    NextHopIPv6 netip.Addr  // From MP_REACH_NLRI
}
```

Then in formatters, use appropriate next-hop based on prefix family.

#### 🟡 Issue 4: No integration test for new FormatMessage path

**Problem:** Tests cover `ApplyToUpdate` with constructed bytes, but no test exercises the full path:
```
RawMessage with AttrsWire → FormatMessage → verify formatted output
```

**Fix:** Add test in `pkg/api/text_test.go`:
```go
func TestFormatMessageWithAttrsWire(t *testing.T) {
    // Build UPDATE with known attributes
    // Create RawMessage with AttrsWire set
    // Call FormatMessage
    // Verify output contains expected formatted content
}
```

#### 🟡 Issue 5: Legacy code duplication

**Problem:** Both old formatters (`formatMessageV6`, `formatParsedV7`, etc.) AND new formatters (`formatFilterResultTextV6`, etc.) exist. Doubles maintenance burden.

**Fix:** Remove legacy formatters. When AttrsWire is nil, create it on-demand:
```go
if msg.AttrsWire == nil && msg.Type == message.TypeUPDATE {
    if attrBytes := ExtractAttributeBytes(msg.RawBytes); attrBytes != nil {
        // Use zero context ID for legacy path
        msg.AttrsWire = attribute.NewAttributesWire(attrBytes, 0)
    }
}
```

Then always use the FilterResult path.

### Not Started

- [ ] Remove legacy Version code (APIVersionLegacy, APIVersionNLRI)
- [ ] Grouped JSON format per spec (NLRI as string arrays)
- [ ] Config integration (parseAttributeFilter in config/bgp.go)

---

## New API Text Format (2026-01-02 Revision)

### Design Decisions

1. **Attributes first** - All NLRI in an announce share the same attributes, so attributes come before families
2. **Split announce/withdraw** - Separate lines for clarity
3. **Per-family next-hop** - RFC 4760 defines next-hop per MP_REACH_NLRI (per AFI/SAFI), not per individual prefix
4. **Update-ID uses increment** - Already implemented using `atomic.Uint64.Add(1)`, avoids clock issues (DST, NTP jumps)

### Text Format

**Announce:**
```
peer <ip> update announce <attributes> <afi> <safi> next-hop <ip> nlri <prefix> [<prefix>...] [<afi> <safi> next-hop <ip> nlri <prefix>...]
```

**Withdraw:**
```
peer <ip> update withdraw <afi> <safi> nlri <prefix> [<prefix>...] [<afi> <safi> nlri <prefix>...]
```

**EOR:**
```
peer <ip> update eor <afi> <safi>
```

### Examples

**Single family announce:**
```
peer 10.0.0.1 update announce origin igp as-path 65001 65002 local-preference 100 ipv4 unicast next-hop 10.0.0.1 nlri 192.168.1.0/24 192.168.2.0/24
```

**Multi-family announce (same UPDATE):**
```
peer 10.0.0.1 update announce origin igp as-path 65001 ipv4 unicast next-hop 10.0.0.1 nlri 10.0.0.0/8 ipv6 unicast next-hop 2001:db8::1 nlri 2001:db8::/32 2001:db8:1::/48
```

**Withdraw:**
```
peer 10.0.0.1 update withdraw ipv4 unicast nlri 192.168.3.0/24 192.168.4.0/24
```

**Mixed announce + withdraw (same UPDATE):**
```
peer 10.0.0.1 update announce origin igp ipv4 unicast next-hop 10.0.0.1 nlri 192.168.1.0/24
peer 10.0.0.1 update withdraw ipv4 unicast nlri 192.168.2.0/24
```

### JSON Format

```json
{
    "type": "update",
    "update-id": 12345,
    "peer": {"address": "10.0.0.1", "asn": 65001},
    "announce": {
        "attributes": {
            "origin": "igp",
            "as-path": [65001, 65002],
            "local-preference": 100
        },
        "ipv4 unicast": {
            "next-hop": "10.0.0.1",
            "nlri": ["192.168.1.0/24", "192.168.2.0/24"]
        },
        "ipv6 unicast": {
            "next-hop": "2001:db8::1",
            "nlri": ["2001:db8::/32"]
        }
    },
    "withdraw": {
        "ipv4 unicast": {
            "nlri": ["192.168.3.0/24"]
        }
    }
}
```

### Implementation Notes

1. **FilterResult changes needed:**
   - Group announced/withdrawn by AFI/SAFI
   - Track next-hop per family, not globally

2. **Legacy path disabled:**
   - Lazy parsing via AttrsWire temporarily disabled
   - Bug: `getOriginFromResult()` returns empty despite `result.Attributes` having 4 entries
   - Root cause: attribute code type mismatch between map key and lookup

---

## Action Items

### ✅ COMPLETED: Lazy Parsing Path & Legacy Removal

- [x] Fix type assertion bug (value vs pointer types for Origin, LocalPref, MED)
- [x] Re-enable AttrsWire path in `FormatMessage()`
- [x] Remove legacy formatters (~500 lines: formatMessageV6, formatMessageV7, etc.)
- [x] Add `buildFilterResultFromDecode()` fallback when AttrsWire is nil
- [x] Add `formatNonUpdate()` for OPEN/NOTIFICATION/KEEPALIVE raw output
- [x] Fix `update-id` inclusion in JSON output
- [x] Update tests to set `msg.Type = message.TypeUPDATE`
- [x] Run full test suite: `make test && make lint && make functional` ✅

### Phase 1: Fix Remaining Bugs

| Priority | Issue | Location | Fix |
|----------|-------|----------|-----|
| 🔴 High | Next-hop extraction only uses first route | `buildFilterResultFromDecode:209-215` | Iterate all routes for IPv4+IPv6 |
| 🔴 High | `formatNonUpdate()` only outputs raw | `text.go:283` | Call `FormatOpen`, `FormatNotification`, `FormatKeepalive` |
| 🟡 Medium | Communities not in decode fallback | `buildFilterResultFromDecode` | Add COMMUNITY, LARGE_COMMUNITY, EXT_COMMUNITY |
| 🟡 Medium | `LOCAL_PREF=0` and `MED=0` filtered out | `buildFilterResultFromDecode:227,233` | Use nil pointer for absence |

---

## Design: Eliminate Redundant Types

### Problem

Current flow has redundant intermediate types:
```
parsePathAttributes() → parsedAttrs → DecodeUpdate() → DecodedUpdate → buildFilterResultFromDecode() → FilterResult
```

Three types doing the same thing:
- `parsedAttrs` - internal parsing result
- `DecodedUpdate` - decoded UPDATE with `[]ReceivedRoute`
- `FilterResult` - final output with `map[AttributeCode]Attribute`

Also, `parsedAttrs` uses primitives that can't distinguish "absent" from "value is 0":
```go
type parsedAttrs struct {
    localPref uint32  // default 100 - what if UPDATE has no LOCAL_PREF?
    med       uint32  // default 0 - what if UPDATE has MED=0?
}
```

RFC 4271: `LOCAL_PREF=0` and `MED=0` are valid values.

### Solution: Use FilterResult Directly

**New flow:**
```
DecodeUpdateToFilterResult(body) → FilterResult
```

Single type, no intermediates. `FilterResult` already has:
- `Attributes map[attribute.AttributeCode]attribute.Attribute` - nil = absent
- `Announced []netip.Prefix`
- `Withdrawn []netip.Prefix`
- `NextHopIPv4 netip.Addr`
- `NextHopIPv6 netip.Addr`

### Changes Required

#### 1. New function `DecodeUpdateToFilterResult()` in `decode.go`

```go
// DecodeUpdateToFilterResult parses UPDATE body directly into FilterResult.
// Uses attribute.Attribute types - nil in map means absent.
// RFC 4271: LOCAL_PREF=0 and MED=0 are valid values.
// RFC 4760: Separate next-hops for IPv4 (NEXT_HOP) and IPv6 (MP_REACH).
func DecodeUpdateToFilterResult(body []byte) FilterResult {
    result := FilterResult{
        Attributes: make(map[attribute.AttributeCode]attribute.Attribute),
    }

    if len(body) < 4 {
        return result
    }

    // Parse UPDATE structure
    withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
    offset := 2

    // IPv4 withdrawn
    if withdrawnLen > 0 && offset+withdrawnLen <= len(body) {
        result.Withdrawn = parseIPv4Prefixes(body[offset : offset+withdrawnLen])
    }
    offset += withdrawnLen

    if offset+2 > len(body) {
        return result
    }

    attrLen := int(binary.BigEndian.Uint16(body[offset : offset+2]))
    offset += 2
    if offset+attrLen > len(body) {
        return result
    }

    pathAttrs := body[offset : offset+attrLen]
    nlriOffset := offset + attrLen

    // Parse path attributes into result.Attributes map
    parsePathAttributesToMap(pathAttrs, &result)

    // IPv4 NLRI
    if nlriOffset < len(body) {
        result.Announced = parseIPv4Prefixes(body[nlriOffset:])
    }

    return result
}

// parsePathAttributesToMap parses attributes directly into FilterResult.
func parsePathAttributesToMap(pathAttrs []byte, result *FilterResult) {
    for i := 0; i < len(pathAttrs); {
        if i+2 > len(pathAttrs) {
            break
        }
        flags := pathAttrs[i]
        typeCode := attribute.AttributeCode(pathAttrs[i+1])

        // ... length parsing ...

        attrValue := pathAttrs[i : i+attrValueLen]
        i += attrValueLen

        switch typeCode {
        case attribute.AttrOrigin:
            if o, err := attribute.ParseOrigin(attrValue); err == nil {
                result.Attributes[typeCode] = o
            }
        case attribute.AttrASPath:
            if ap, err := attribute.ParseASPath(attrValue, true); err == nil {
                result.Attributes[typeCode] = ap
            }
        case attribute.AttrNextHop:
            if nh, err := attribute.ParseNextHop(attrValue); err == nil {
                result.Attributes[typeCode] = nh
                result.NextHopIPv4 = nh.Addr  // Quick access
            }
        case attribute.AttrMED:
            if m, err := attribute.ParseMED(attrValue); err == nil {
                result.Attributes[typeCode] = m  // MED=0 stored correctly
            }
        case attribute.AttrLocalPref:
            if lp, err := attribute.ParseLocalPref(attrValue); err == nil {
                result.Attributes[typeCode] = lp  // LOCAL_PREF=0 stored correctly
            }
        case attribute.AttrCommunity:
            if c, err := attribute.ParseCommunities(attrValue); err == nil {
                result.Attributes[typeCode] = &c
            }
        case attribute.AttrExtCommunity:
            if ec, err := attribute.ParseExtendedCommunities(attrValue); err == nil {
                result.Attributes[typeCode] = &ec
            }
        case attribute.AttrLargeCommunity:
            if lc, err := attribute.ParseLargeCommunities(attrValue); err == nil {
                result.Attributes[typeCode] = &lc
            }
        case attribute.AttrMPReachNLRI:
            if mp, err := attribute.ParseMPReachNLRI(attrValue); err == nil {
                // Extract IPv6 NLRI and next-hop
                if mp.AFI == attribute.AFIIPv6 && mp.SAFI == attribute.SAFIUnicast {
                    result.Announced = append(result.Announced, parseIPv6Prefixes(mp.NLRI)...)
                    if len(mp.NextHops) > 0 {
                        result.NextHopIPv6 = mp.NextHops[0]
                    }
                }
            }
        case attribute.AttrMPUnreachNLRI:
            if mp, err := attribute.ParseMPUnreachNLRI(attrValue); err == nil {
                if mp.AFI == attribute.AFIIPv6 && mp.SAFI == attribute.SAFIUnicast {
                    result.Withdrawn = append(result.Withdrawn, parseIPv6Prefixes(mp.NLRI)...)
                }
            }
        }
    }
}
```

#### 2. Update `buildFilterResultFromDecode()` in `text.go`

Replace entire function:
```go
func buildFilterResultFromDecode(body []byte, filter *AttributeFilter) FilterResult {
    // Parse directly to FilterResult
    result := DecodeUpdateToFilterResult(body)

    // Apply filter if not "all"
    if filter.Mode == FilterModeNone {
        result.Attributes = nil
    } else if filter.Mode == FilterModeSelective {
        filtered := make(map[attribute.AttributeCode]attribute.Attribute)
        for code, attr := range result.Attributes {
            if filter.Includes(code) {
                filtered[code] = attr
            }
        }
        result.Attributes = filtered
    }

    return result
}
```

#### 3. Delete obsolete types

- Remove `parsedAttrs` struct
- Remove `parsePathAttributes()` function (replaced by `parsePathAttributesToMap`)
- Keep `DecodedUpdate` and `DecodeUpdate()` for backward compat (used by other code)
- Keep `ReceivedRoute` for `FormatReceivedUpdate()` (legacy API)

#### 4. Update `formatNonUpdate()` in `text.go`

```go
func formatNonUpdate(peer PeerInfo, msg RawMessage, content ContentConfig) string {
    switch msg.Type {
    case message.TypeOPEN:
        decoded := DecodeOpen(msg.RawBytes)
        return FormatOpen(peer.Address, decoded)
    case message.TypeNOTIFICATION:
        decoded := DecodeNotification(msg.RawBytes)
        return FormatNotification(peer.Address, decoded)
    case message.TypeKEEPALIVE:
        return FormatKeepalive(peer.Address)
    default:
        return fmt.Sprintf("peer %s %s raw %x\n",
            peer.Address, strings.ToLower(msg.Type.String()), msg.RawBytes)
    }
}
```

---

**Tasks:**
- [ ] Add `DecodeUpdateToFilterResult()` in `decode.go`
- [ ] Add `parsePathAttributesToMap()` with all attribute types
- [ ] Simplify `buildFilterResultFromDecode()` to call new function
- [ ] Update `formatNonUpdate()` to route to dedicated formatters
- [ ] Verify `LOCAL_PREF=0` and `MED=0` are included in output
- [ ] Verify both IPv4 and IPv6 next-hops extracted
- [ ] Run `make test && make lint && make functional`

### Phase 2: Config Integration

- [ ] Implement `parseAttributeFilter()` in config parser
- [ ] Wire `Attributes` field from PeerContentConfig to ContentConfig
- [ ] Support syntax: `attributes as-path next-hop communities;`
- [ ] Support `all` and `none` keywords
- [ ] Reject `attr-14` / `attr-15` (MP_REACH/UNREACH are structural)
- [ ] Test config parsing with various attribute combinations

### Phase 3: Grouped Output Format (Future)

```go
// FamilyNLRI groups NLRI by AFI/SAFI with family-specific next-hop
type FamilyNLRI struct {
    AFI      uint16
    SAFI     uint8
    NextHop  netip.Addr
    Prefixes []netip.Prefix
}
```

- [ ] Define `FamilyNLRI` struct
- [ ] Update `FilterResult` to use `[]FamilyNLRI`
- [ ] Implement grouped text format: `peer X update announce <attrs> ipv4 unicast next-hop Y nlri ...`
- [ ] Implement grouped JSON format with per-family blocks
- [ ] Handle multi-family UPDATEs correctly

### Phase 4: Remove Version Field (Future)

- [ ] Remove `ContentConfig.Version` field
- [ ] Remove `APIVersionLegacy`, `APIVersionNLRI` constants
- [ ] Use single unified format (grouped style)
- [ ] Update all tests

---

**Created:** 2026-01-01
**Revised:** 2026-01-02 (lazy parsing, legacy removal, type assertion fixes)
**Revised:** 2026-01-02 (eliminate redundant types - use FilterResult directly)
