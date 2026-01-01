# Spec: API Attribute Filter

## Status: Ready for Implementation

## Prerequisites

- `spec-attributes-wire.md` - AttributesWire.GetMultiple()

## Problem

Current API outputs ALL parsed attributes, even if external process only needs a few.

Wasteful:
```json
{
    "as-path": [...],
    "origin": "igp",
    "next-hop": "...",
    "med": 100,
    "local-pref": 200,
    "communities": [...],
    "extended-communities": [...],
    "large-communities": [...],
    "originator-id": "...",
    "cluster-list": [...]
}
```

When process only needs:
```json
{
    "as-path": [...],
    "next-hop": "..."
}
```

## Solution

Config option to limit which attributes are parsed and output:

```
api foo {
    content {
        encoding json;
        attributes as-path next-hop;  # Only parse/output these
    }
    receive { update; }
}
```

## Config Syntax

### Attribute Names

| Name | Code | Description |
|------|------|-------------|
| `origin` | 1 | ORIGIN |
| `as-path` | 2 | AS_PATH |
| `next-hop` | 3 | NEXT_HOP |
| `med` | 4 | MULTI_EXIT_DISC |
| `local-pref` | 5 | LOCAL_PREF |
| `atomic-aggregate` | 6 | ATOMIC_AGGREGATE |
| `aggregator` | 7 | AGGREGATOR |
| `community` | 8 | COMMUNITIES |
| `originator-id` | 9 | ORIGINATOR_ID |
| `cluster-list` | 10 | CLUSTER_LIST |
| `extended-community` | 16 | EXTENDED_COMMUNITIES |
| `large-community` | 32 | LARGE_COMMUNITIES |
| `all` | - | All attributes (default) |
| `none` | - | No attributes (route-id only) |

### Examples

```
# Only AS_PATH and next-hop (minimal for loop detection)
api minimal {
    content {
        attributes as-path next-hop;
    }
    receive { update; }
}

# Community-based routing
api community-router {
    content {
        attributes as-path next-hop community large-community;
    }
    receive { update; }
}

# Route ID only (decision based on RFC 9234 role tag)
api role-based {
    content {
        attributes none;  # Only route-id and role tag
    }
    receive { update; }
}

# Full attributes (default, explicit)
api full {
    content {
        attributes all;
    }
    receive { update; }
}
```

## Implementation

### Config Parsing

```go
// pkg/config/bgp.go

type APIContentConfig struct {
    Encoding   string              // json | text
    Format     string              // parsed | raw | full
    Attributes []AttributeCode     // NEW: which attrs to parse/output
    AllAttrs   bool                // NEW: true if "all" specified
    NoAttrs    bool                // NEW: true if "none" specified
}

func parseAPIContent(tree *Tree) (*APIContentConfig, error) {
    // ... existing parsing ...

    // Parse attributes keyword
    if attrsNode := tree.Get("attributes"); attrsNode != nil {
        attrs, err := parseAttributeList(attrsNode.Value())
        if err != nil {
            return nil, err
        }
        cfg.Attributes = attrs
    } else {
        cfg.AllAttrs = true  // Default to all
    }
    return cfg, nil
}

func parseAttributeList(s string) ([]AttributeCode, error) {
    if s == "all" {
        return nil, nil  // nil means all
    }
    if s == "none" {
        return []AttributeCode{}, nil  // empty slice means none
    }

    names := strings.Fields(s)
    codes := make([]AttributeCode, 0, len(names))
    for _, name := range names {
        code, ok := attributeNameToCode[name]
        if !ok {
            return nil, fmt.Errorf("unknown attribute: %s", name)
        }
        codes = append(codes, code)
    }
    return codes, nil
}

var attributeNameToCode = map[string]AttributeCode{
    "origin":             AttrOrigin,
    "as-path":            AttrASPath,
    "next-hop":           AttrNextHop,
    "med":                AttrMED,
    "local-pref":         AttrLocalPref,
    "atomic-aggregate":   AttrAtomicAggregate,
    "aggregator":         AttrAggregator,
    "community":          AttrCommunity,
    "originator-id":      AttrOriginatorID,
    "cluster-list":       AttrClusterList,
    "extended-community": AttrExtCommunity,
    "large-community":    AttrLargeCommunity,
}
```

### API Binding Storage

```go
// pkg/api/types.go

type APIBinding struct {
    ProcessName string
    Encoding    string
    Format      string
    Receive     MessageTypes
    Send        MessageTypes
    Attributes  []AttributeCode  // NEW
    AllAttrs    bool             // NEW
}
```

### Message Formatting

```go
// pkg/api/format.go

func formatUpdateMessage(route *Route, binding *APIBinding) []byte {
    // Get only requested attributes
    var attrs map[AttributeCode]Attribute
    if binding.AllAttrs {
        attrs = route.attrs.All()
    } else if len(binding.Attributes) > 0 {
        attrs = route.attrs.GetMultiple(binding.Attributes)
    }
    // attrs is nil/empty for "none"

    // Format with only these attributes
    return formatWithAttributes(route, attrs, binding.Encoding)
}
```

## API Output

### With `attributes as-path next-hop;`

```json
{
    "type": "update",
    "route-id": 12345,
    "peer": { "address": "10.0.0.1" },
    "announce": {
        "nlri": { "ipv4 unicast": ["192.168.1.0/24"] },
        "attributes": {
            "as-path": [65001, 65002],
            "next-hop": "10.0.0.1"
        }
    }
}
```

### With `attributes none;`

```json
{
    "type": "update",
    "route-id": 12345,
    "peer": { "address": "10.0.0.1" },
    "announce": {
        "nlri": { "ipv4 unicast": ["192.168.1.0/24"] }
    }
}
```

## Test Plan

```go
// TestParseAttributeList verifies attribute name parsing.
// VALIDATES: Valid names map to correct codes.
// PREVENTS: Typos causing silent failures.
func TestParseAttributeList(t *testing.T)

// TestAPIBindingAttributes verifies config to binding flow.
// VALIDATES: Config attributes stored in binding.
// PREVENTS: Lost config during loading.
func TestAPIBindingAttributes(t *testing.T)

// TestFormatWithAttributes verifies filtered output.
// VALIDATES: Only requested attributes in output.
// PREVENTS: Attribute leakage, missing attributes.
func TestFormatWithAttributes(t *testing.T)

// TestAttributesNone verifies empty attribute output.
// VALIDATES: No attributes section when none requested.
// PREVENTS: Empty attributes object in output.
func TestAttributesNone(t *testing.T)

// TestAttributesAll verifies full attribute output.
// VALIDATES: All attributes included when "all" specified.
// PREVENTS: Missing attributes with explicit "all".
func TestAttributesAll(t *testing.T)
```

## Checklist

- [ ] `attributes` keyword in content block schema
- [ ] parseAttributeList() function
- [ ] attributeNameToCode map
- [ ] APIBinding.Attributes field
- [ ] formatUpdateMessage uses binding.Attributes
- [ ] Tests for parsing
- [ ] Tests for formatting
- [ ] `make test && make lint` pass
- [ ] Functional test

## Dependencies

- `spec-attributes-wire.md` - GetMultiple()

---

**Created:** 2026-01-01
