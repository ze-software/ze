# Plan: Enhanced Family Negotiation Configuration

**Created:** 2025-12-21
**Status:** Implemented

---

## Summary

Enhance family configuration to support:
1. Current syntax: `<afi> <safi>;` (enable)
2. Block syntax: `<afi> { <safi> <mode>; }` with per-family options
3. Tertiary mode per-family: `true/enable`, `false/disable`, `require`
4. `require` mode: refuse session if peer doesn't announce family, send NOTIFICATION

---

## Current State

### Config Syntax
```
family {
    ipv4 unicast;
    ipv6 unicast;
    ignore-mismatch enable;  # global option
}
```

### Data Flow
1. `bgp.go:127` - Schema: `Field("family", Freeform())`
2. Parser stores "ipv4 unicast" → "true" in Tree
3. `bgp.go:539-560` - Extracts families as []string in NeighborConfig.Families
4. `loader.go` converts to []capability.Multiprotocol in Neighbor.Capabilities
5. `session.go:532-572` - Negotiates capabilities, stores in Negotiated.families
6. `session.go:494` - IgnoreFamilyMismatch flag controls UPDATE validation

---

## Proposed Syntax

### Inline syntax (current + mode)
```
family {
    ipv4 unicast;               # enable (default)
    ipv4 unicast enable;        # explicit enable
    ipv6 unicast disable;       # disable (don't advertise)
    l2vpn evpn require;         # require (fail if peer doesn't support)
}
```

### Block syntax (AFI as block key, SAFIs inside)
```
family {
    ipv4 {
        unicast;                # enable (default)
        multicast require;      # with mode
    }
    ipv6 {
        unicast;
        mpls-vpn
    }
    l2vpn {
        evpn require
    }
}
```

### One-liner block syntax
```
family { ipv6 { unicast require; mpls-vpn } }
```

### Mixed syntax
```
family {
    ipv4 unicast;               # inline - simple case
    ipv6 {                      # block - multiple SAFIs
        unicast require;
        mpls-vpn
    }
}
```

### Syntax Rules
1. **Semicolons separate entries** on the same line: `unicast require; mpls-vpn`
2. **Semicolons optional** before `}` or at end of line (newline/`}` acts as terminator)
3. Both inline (`ipv4 unicast`) and block (`ipv4 { unicast }`) syntaxes coexist
4. Default mode is `enable` when not specified

---

## Data Structures

### FamilyMode Type
```go
// pkg/config/bgp.go or new file pkg/config/family.go

type FamilyMode int

const (
    FamilyModeDisable FamilyMode = iota  // Don't advertise
    FamilyModeEnable                      // Advertise, accept mismatch
    FamilyModeRequire                     // Advertise, refuse session if not negotiated
)
```

### FamilyConfig Struct
```go
// pkg/config/bgp.go

type FamilyConfig struct {
    AFI  string     // "ipv4", "ipv6", "l2vpn", "bgp-ls"
    SAFI string     // "unicast", "multicast", "mpls-vpn", etc.
    Mode FamilyMode // enable, disable, require
}

// In NeighborConfig, replace:
//   Families             []string
// With:
//   Families             []FamilyConfig
```

### Neighbor Changes
```go
// pkg/reactor/neighbor.go

type Neighbor struct {
    // ... existing fields ...

    // Replace Capabilities usage for families with structured config
    FamilyConfigs []FamilyConfig

    // RequiredFamilies computed from FamilyConfigs with Mode=Require
    RequiredFamilies []capability.Family
}
```

---

## Implementation Phases

### Phase 1: Schema and Parsing

**Files:** `pkg/config/schema.go`, `pkg/config/parser.go`, `pkg/config/bgp.go`

1. Create new `FamilyNode` schema type that handles:
   - `<afi> <safi>;` → enable
   - `<afi> <safi> <mode>;` → mode from last token
   - `<afi> { <safi>; <safi> <mode>; }` → block with per-safi modes

2. Update `bgp.go`:
   - Change `Field("family", Freeform())` to `Field("family", FamilyBlock())`
   - Add `FamilyConfig` struct
   - Update `parseNeighborConfig` to populate `[]FamilyConfig`

3. Parser changes:
   - `parseFamilyBlock()` method
   - Handle inline: `ipv4 unicast require;`
   - Handle block: `ipv4 { unicast; multicast require; }`

**Tests:**
- Parse `ipv4 unicast;` → FamilyConfig{AFI:"ipv4", SAFI:"unicast", Mode:Enable}
- Parse `ipv4 unicast require;` → Mode:Require
- Parse `ipv4 { unicast; multicast disable; }` → two FamilyConfigs
- Parse `ipv6 { unicast require }` → no semicolon before `}`

### Phase 2: Loader and Neighbor

**Files:** `pkg/config/loader.go`, `pkg/reactor/neighbor.go`

1. Update loader to convert FamilyConfig to:
   - Multiprotocol capabilities (for Mode != Disable)
   - RequiredFamilies list (for Mode == Require)

2. Add to Neighbor:
   - `FamilyConfigs []FamilyConfig`
   - `RequiredFamilies []capability.Family`

**Tests:**
- Config with require → Neighbor.RequiredFamilies populated
- Config with disable → no capability generated
- Config with enable → capability generated, not in RequiredFamilies

### Phase 3: Session Validation

**Files:** `pkg/reactor/session.go`, `pkg/bgp/capability/negotiated.go`

1. Add to `Negotiated`:
```go
// CheckRequired returns families that were required but not negotiated.
func (n *Negotiated) CheckRequired(required []Family) []Family {
    var missing []Family
    for _, f := range required {
        if !n.families[f] {
            missing = append(missing, f)
        }
    }
    return missing
}
```

2. Add validation in `session.go` after `negotiate()`:
```go
// In handleOpen(), after negotiate():
if missing := s.negotiated.CheckRequired(s.neighbor.RequiredFamilies); len(missing) > 0 {
    // Send NOTIFICATION: OPEN Message Error (2), Unsupported Capability (7)
    // RFC 5492 Section 3: include capability data
    _ = s.sendNotification(conn, neg,
        message.NotifyOpenMessage,
        message.NotifyOpenUnsupportedCapability,
        buildUnsupportedCapData(missing),
    )
    _ = s.fsm.Event(fsm.EventBGPOpenMsgErr)
    s.closeConn()
    log.Printf("session rejected: required families not negotiated: %v", missing)
    return fmt.Errorf("required families not negotiated: %v", missing)
}
```

3. Add NOTIFICATION subcode if missing:
```go
// pkg/bgp/message/notification.go
const NotifyOpenUnsupportedCapability = 7 // RFC 5492
```

**Tests:**
- Session with require + peer supports → established
- Session with require + peer doesn't support → NOTIFICATION sent, session fails
- Multiple required families, one missing → fails
- Log message includes missing family details

### Phase 4: Backward Compatibility

**Files:** `pkg/config/bgp.go`

1. Keep `ignore-mismatch` as global fallback
2. `ignore-mismatch` applies only to families with Mode=Enable
3. Mode=Require overrides ignore-mismatch

```
family {
    ipv4 unicast;           # affected by ignore-mismatch
    ipv6 unicast require;   # NOT affected by ignore-mismatch
    ignore-mismatch enable;
}
```

---

## NOTIFICATION Details

When a required family is not negotiated:

- **Error Code:** 2 (OPEN Message Error)
- **Error Subcode:** 7 (Unsupported Capability) - RFC 5492
- **Data:** Capability code and value for each missing family

Format per RFC 5492 Section 3:
```
+-------+-------+-------+-------+
| Cap1 Code | Cap1 Len | Cap1 Value...
+-------+-------+-------+-------+
| Cap2 Code | Cap2 Len | Cap2 Value...
```

For Multiprotocol (code 1):
```
+-------+-------+-------+-------+-------+
|   1   |   4   | AFI (2) | Res | SAFI  |
+-------+-------+-------+-------+-------+
```

---

## Log Messages

```
# On require failure:
level=warn msg="session rejected: required address families not negotiated"
    peer=192.168.1.1 missing="ipv6/unicast, l2vpn/evpn"

# On successful negotiation with require:
level=info msg="all required families negotiated"
    peer=192.168.1.1 families="ipv6/unicast"
```

---

## Test Cases

### Config Parsing Tests
1. `ipv4 unicast` → enable
2. `ipv4 unicast enable` → enable
3. `ipv4 unicast true` → enable
4. `ipv4 unicast disable` → disable
5. `ipv4 unicast false` → disable
6. `ipv4 unicast require` → require
7. `ipv4 { unicast; multicast require; }` → two families
8. `ipv6 { unicast require }` → no trailing semicolon
9. Invalid mode → parse error

### Session Tests
1. require + peer supports → ESTABLISHED
2. require + peer doesn't support → NOTIFICATION, session fails
3. Multiple require, all supported → ESTABLISHED
4. Multiple require, one missing → NOTIFICATION, session fails
5. enable + peer doesn't support + ignore-mismatch → ESTABLISHED
6. require + ignore-mismatch → still fails (require overrides)

### NOTIFICATION Tests
1. Verify error code 2, subcode 7
2. Verify capability data format
3. Verify multiple missing families encoded correctly

---

## File Changes Summary

| File | Changes |
|------|---------|
| `pkg/config/schema.go` | Add FamilyNode type |
| `pkg/config/parser.go` | Add parseFamilyBlock() |
| `pkg/config/bgp.go` | FamilyMode, FamilyConfig, update schema |
| `pkg/config/loader.go` | Convert FamilyConfig to capabilities |
| `pkg/reactor/neighbor.go` | Add FamilyConfigs, RequiredFamilies |
| `pkg/reactor/session.go` | Validate required families post-negotiate |
| `pkg/bgp/capability/negotiated.go` | Add CheckRequired() |
| `pkg/bgp/message/notification.go` | Add NotifyOpenUnsupportedCapability |

---

## Open Questions

1. **ExaBGP compatibility:** Does ExaBGP support `require` mode?
   - Check `main/src/exabgp/configuration/neighbor/family.py`
   - If not, this is a ZeBGP extension

2. **Per-family ignore-mismatch:** Should we add `ignore` as fourth mode?
   - `enable` = advertise, fail on mismatch (current default)
   - `ignore` = advertise, ignore mismatch (current ignore-mismatch)
   - `require` = advertise, send NOTIFICATION on mismatch
   - `disable` = don't advertise

3. **Block syntax necessity:** Is `ipv4 { unicast require; }` needed, or is
   `ipv4 unicast require;` sufficient for all use cases?

---

## RFC References

- **RFC 4760** Section 8: Multiprotocol capability negotiation
- **RFC 5492** Section 3: Unsupported Capability NOTIFICATION
- **RFC 4271** Section 4.2: OPEN message format
- **RFC 4271** Section 6.2: NOTIFICATION message format

---

## Implementation Order

1. ✅ Write plan (this document)
2. ✅ Add tests for FamilyConfig parsing
3. ✅ Implement FamilyNode schema and parser
4. ✅ Update loader for FamilyConfigs
5. ✅ Add RequiredFamilies to Neighbor
6. ✅ Add CheckRequired to Negotiated
7. ✅ Implement session require validation
8. ✅ Add NOTIFICATION data builder
9. ✅ All tests pass, lint clean
