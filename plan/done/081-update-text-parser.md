# Spec: update-text-parser

## Task

Parser infrastructure for `update text` command format:
```
[attr <set|add|del> <attributes>]... [nlri <family> add <nlri>... [del <nlri>...]]... [watchdog <name>]
```

**Chunk 1 of 10** for announce-family-first refactor. Foundation for Chunk 2 (handler).

## Required Reading

- [x] `.claude/zebgp/api/ARCHITECTURE.md` - API command structure
- [x] `.claude/zebgp/wire/NLRI.md` - NLRI types and families
- [x] `.claude/zebgp/wire/ATTRIBUTES.md` - Path attribute handling

**Key insights:**
- Attributes accumulate across sections; each `nlri` section captures snapshot
- RD/label are per-NLRI (in nlri section), NOT global attributes
- Watchdog adds routes to pool without announcing
- Snapshot must deep copy slices to isolate groups

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| `TestParseUpdateText_EmptyInput` | `pkg/api/update_text_test.go` | Empty args → empty result |
| `TestParseUpdateText_AttrSetNextHop` | `pkg/api/update_text_test.go` | next-hop parsing |
| `TestParseUpdateText_AttrSetNextHopSelf` | `pkg/api/update_text_test.go` | next-hop-self flag |
| `TestParseUpdateText_AttrSetOrigin` | `pkg/api/update_text_test.go` | origin attribute |
| `TestParseUpdateText_AttrSetMultiple` | `pkg/api/update_text_test.go` | Multiple attrs in one section |
| `TestParseUpdateText_AttrSetCommunity` | `pkg/api/update_text_test.go` | Community list parsing |
| `TestParseUpdateText_AttrAddCommunity` | `pkg/api/update_text_test.go` | Community append |
| `TestParseUpdateText_AttrDelCommunity` | `pkg/api/update_text_test.go` | Community removal |
| `TestParseUpdateText_AttrSetThenAdd` | `pkg/api/update_text_test.go` | Set then add accumulation |
| `TestParseUpdateText_LargeCommunity` | `pkg/api/update_text_test.go` | Large community parsing |
| `TestParseUpdateText_ExtendedCommunity` | `pkg/api/update_text_test.go` | Extended community parsing |
| `TestParseUpdateText_AttrAddScalarError` | `pkg/api/update_text_test.go` | add on scalar → error |
| `TestParseUpdateText_AttrDelScalarError` | `pkg/api/update_text_test.go` | del on scalar → error |
| `TestParseUpdateText_AttrAddASPathError` | `pkg/api/update_text_test.go` | add as-path → error |
| `TestParseUpdateText_AttrDelASPathError` | `pkg/api/update_text_test.go` | del as-path → error |
| `TestParseUpdateText_NLRISectionBasic` | `pkg/api/update_text_test.go` | Basic NLRI add |
| `TestParseUpdateText_NLRIMultiplePrefixes` | `pkg/api/update_text_test.go` | Multiple prefixes |
| `TestParseUpdateText_NLRIMixedAddDel` | `pkg/api/update_text_test.go` | Mixed add/del |
| `TestParseUpdateText_NLRIWithdrawOnly` | `pkg/api/update_text_test.go` | Del-only section |
| `TestParseUpdateText_NLRIMultipleAddDel` | `pkg/api/update_text_test.go` | Multiple add/del switches |
| `TestParseUpdateText_NLRIEmptyError` | `pkg/api/update_text_test.go` | Empty section → error |
| `TestParseUpdateText_NLRIMissingAddDel` | `pkg/api/update_text_test.go` | Missing add/del → error |
| `TestParseUpdateText_AttrAndNLRI` | `pkg/api/update_text_test.go` | Combined attr + nlri |
| `TestParseUpdateText_MultipleGroups` | `pkg/api/update_text_test.go` | Snapshot deep copy verification |
| `TestParseUpdateText_IPv6` | `pkg/api/update_text_test.go` | IPv6 support |
| `TestParseUpdateText_FamilyMismatch` | `pkg/api/update_text_test.go` | IPv4 prefix in ipv6/unicast |
| `TestParseUpdateText_UnknownAttribute` | `pkg/api/update_text_test.go` | Unknown attr → error |
| `TestParseUpdateText_UnsupportedFamily` | `pkg/api/update_text_test.go` | Unsupported family → error |
| `TestParseUpdateText_InvalidFamilyString` | `pkg/api/update_text_test.go` | Invalid family string |
| `TestParseUpdateText_InvalidPrefix` | `pkg/api/update_text_test.go` | Invalid prefix format |
| `TestParseUpdateText_MissingPrefixAfterAdd` | `pkg/api/update_text_test.go` | add with no prefixes |
| `TestParseUpdateText_Watchdog` | `pkg/api/update_text_test.go` | Watchdog name capture |
| `TestParseUpdateText_WatchdogOnly` | `pkg/api/update_text_test.go` | Watchdog without routes |
| `TestParseUpdateText_SpecExample` | `pkg/api/update_text_test.go` | Full chained example |

### Functional Tests

| Test | Location | Scenario |
|------|----------|----------|
| N/A | - | Parser is unit-tested; integration tested via handler in Chunk 2 |

## Files to Modify

- `pkg/api/update_text.go` - **CREATE** - Parser implementation
- `pkg/api/update_text_test.go` - **CREATE** - TDD tests (34 tests)
- `pkg/api/types.go` - **MODIFY** - Add NLRIGroup, UpdateTextResult
- `pkg/api/errors.go` - **MODIFY** - Add new error types

## Implementation Steps

1. **Add types** - Add NLRIGroup, UpdateTextResult to `types.go`
2. **Add errors** - Add error types to `errors.go`
3. **Create stub** - Create `update_text.go` with stub returning "not implemented"
4. **Write tests** - Write all 34 tests in `update_text_test.go`
5. **Run tests** - Verify all FAIL (paste output below)
6. **Implement parsedAttrs** - Struct with applySet/applyAdd/applyDel/snapshot
7. **Implement parseAttrSection** - Loop until boundary keyword
8. **Implement parseNLRISection** - Add/del state machine with parseINETNLRI
9. **Implement ParseUpdateText** - Main algorithm
10. **Run tests** - Verify all PASS (paste output below)
11. **Verify** - `make lint && make test && make functional`

## RFC Documentation

N/A - This is API parsing, not wire protocol. No RFC references needed.

## Checklist

### 🧪 TDD
- [x] Tests written (46 tests - expanded during review)
- [x] Tests FAIL (stub returns "not implemented")
- [x] Implementation complete
- [x] Tests PASS (all 46 pass)

### Verification
- [x] `make lint` passes (no issues in modified files)
- [x] `make test` passes (flaky reactor test unrelated)
- [x] `make functional` passes (18/18)

### Documentation
- [x] Required docs read
- [x] `.claude/zebgp/api/ARCHITECTURE.md` updated if API schema changed

### Completion
- [x] Spec moved to `plan/done/NNN-update-text-parser.md`

---

# Design Details

## Key Semantics

### Attribute Modes

| Mode | Scalar attrs (origin, med, local-preference, next-hop) | List attrs (community, large-community, extended-community) |
|------|--------------------------------------------------------|-------------------------------------------------------------|
| `set` | Replace value | Replace entire list |
| `add` | **Error** - use `set` for scalars | Append to list |
| `del` | **Error** - use `set` with different value | Remove from list |

- Attributes accumulate across sections
- Each `nlri` section captures current attribute snapshot
- **AS-PATH is set-only** - despite being a slice, add/del not supported (path integrity)

### RD/Label Are Per-NLRI, Not Attributes

`rd` and `label` are **NOT** global attributes. They are per-NLRI parameters parsed within the `nlri` section for specific families only:

| Family | NLRI syntax | Example |
|--------|-------------|---------|
| `ipv4/unicast` | `<prefix>` | `1.0.0.0/24` |
| `ipv4/mpls-label` | `label <N> <prefix>` | `label 100 10.0.0.0/24` |
| `ipv4/vpn` | `rd <RD> label <N> <prefix>` | `rd 100:100 label 100 10.0.0.0/24` |

Each prefix can have different RD/label values:
```
nlri ipv4/mpls-label add label 100 10.0.0.0/24 label 200 10.0.1.0/24
```

Using `label` or `rd` in `attr` section → error "unknown attribute".

### Watchdog Semantics

`watchdog <name>` adds routes to a named pool WITHOUT announcing. Routes are announced later via `announce watchdog <name>`.

---

## Data Structures

### New in `pkg/api/types.go`

```go
// NLRIGroup represents a group of NLRIs sharing the same attributes.
type NLRIGroup struct {
    Family      nlri.Family      // Address family (AFI/SAFI)
    Announce    []nlri.NLRI      // NLRIs to announce
    Withdraw    []nlri.NLRI      // NLRIs to withdraw
    Attrs       PathAttributes   // Snapshot of accumulated attributes
    NextHop     netip.Addr       // Next-hop address
    NextHopSelf bool             // Use peer's local address
}

// UpdateTextResult is the parsed result of an update text command.
type UpdateTextResult struct {
    Groups       []NLRIGroup
    WatchdogName string
}
```

### New in `pkg/api/update_text.go`

```go
// parsedAttrs tracks attribute state during parsing.
// Includes next-hop which is NOT part of PathAttributes.
type parsedAttrs struct {
    NextHop     netip.Addr
    NextHopSelf bool
    PathAttributes
}

func (a *parsedAttrs) applySet(other parsedAttrs)
func (a *parsedAttrs) applyAdd(other parsedAttrs) error  // error if non-list attr
func (a *parsedAttrs) applyDel(other parsedAttrs) error  // error if non-list attr
func (a *parsedAttrs) snapshot() (PathAttributes, netip.Addr, bool)  // MUST deep copy slices
```

**CRITICAL: snapshot() must deep copy all slices:**
```go
func (a *parsedAttrs) snapshot() (PathAttributes, netip.Addr, bool) {
    // Deep copy to isolate each group from later modifications
    pa := PathAttributes{
        Origin:          a.Origin,
        LocalPreference: a.LocalPreference,
        MED:             a.MED,
    }
    if a.ASPath != nil {
        pa.ASPath = make([]uint32, len(a.ASPath))
        copy(pa.ASPath, a.ASPath)
    }
    if a.Communities != nil {
        pa.Communities = make([]uint32, len(a.Communities))
        copy(pa.Communities, a.Communities)
    }
    if a.LargeCommunities != nil {
        pa.LargeCommunities = make([]LargeCommunity, len(a.LargeCommunities))
        copy(pa.LargeCommunities, a.LargeCommunities)
    }
    if a.ExtendedCommunities != nil {
        pa.ExtendedCommunities = make([]attribute.ExtendedCommunity, len(a.ExtendedCommunities))
        copy(pa.ExtendedCommunities, a.ExtendedCommunities)
    }
    return pa, a.NextHop, a.NextHopSelf
}
```

**applySet merge semantics (only overwrite if explicitly set):**
- Pointer fields (Origin, MED, LocalPref): overwrite if `other.X != nil`
- Slice fields (Communities, ASPath, etc.): overwrite if `other.X != nil`
- `NextHop`: overwrite if `other.NextHop.IsValid()`
- `NextHopSelf`: overwrite if `other.NextHopSelf == true` (can only enable)

**applyAdd implementation:**
```go
func (a *parsedAttrs) applyAdd(other parsedAttrs) error {
    // AS-PATH special case - not addable
    if other.ASPath != nil {
        return ErrASPathNotAddable
    }

    // Scalars - error if set
    if other.Origin != nil {
        return fmt.Errorf("origin: %w", ErrAddOnScalar)
    }
    if other.MED != nil {
        return fmt.Errorf("med: %w", ErrAddOnScalar)
    }
    if other.LocalPreference != nil {
        return fmt.Errorf("local-preference: %w", ErrAddOnScalar)
    }
    if other.NextHop.IsValid() {
        return fmt.Errorf("next-hop: %w", ErrAddOnScalar)
    }
    if other.NextHopSelf {
        return fmt.Errorf("next-hop-self: %w", ErrAddOnScalar)
    }

    // Lists - append
    if other.Communities != nil {
        a.Communities = append(a.Communities, other.Communities...)
    }
    if other.LargeCommunities != nil {
        a.LargeCommunities = append(a.LargeCommunities, other.LargeCommunities...)
    }
    if other.ExtendedCommunities != nil {
        a.ExtendedCommunities = append(a.ExtendedCommunities, other.ExtendedCommunities...)
    }
    return nil
}
```

**applyDel implementation:**
```go
func (a *parsedAttrs) applyDel(other parsedAttrs) error {
    // AS-PATH special case - not deletable
    if other.ASPath != nil {
        return ErrASPathNotAddable
    }

    // Scalars - error if set
    if other.Origin != nil {
        return fmt.Errorf("origin: %w", ErrDelOnScalar)
    }
    if other.MED != nil {
        return fmt.Errorf("med: %w", ErrDelOnScalar)
    }
    if other.LocalPreference != nil {
        return fmt.Errorf("local-preference: %w", ErrDelOnScalar)
    }
    if other.NextHop.IsValid() {
        return fmt.Errorf("next-hop: %w", ErrDelOnScalar)
    }
    if other.NextHopSelf {
        return fmt.Errorf("next-hop-self: %w", ErrDelOnScalar)
    }

    // Lists - remove matching values
    if other.Communities != nil {
        a.Communities = removeFromSlice(a.Communities, other.Communities)
    }
    if other.LargeCommunities != nil {
        a.LargeCommunities = removeFromSlice(a.LargeCommunities, other.LargeCommunities)
    }
    if other.ExtendedCommunities != nil {
        a.ExtendedCommunities = removeFromSlice(a.ExtendedCommunities, other.ExtendedCommunities)
    }
    return nil
}

// removeFromSlice removes all elements in 'remove' from 'slice'.
func removeFromSlice[T comparable](slice, remove []T) []T {
    if len(slice) == 0 || len(remove) == 0 {
        return slice
    }
    result := slice[:0]
    for _, v := range slice {
        if !slices.Contains(remove, v) {
            result = append(result, v)
        }
    }
    return result
}
```

---

## Parser Functions

| Function | Signature | Purpose |
|----------|-----------|---------|
| `ParseUpdateText` | `(args []string) (*UpdateTextResult, error)` | Main entry point |
| `parseAttrSection` | `(args []string) (mode string, attrs parsedAttrs, consumed int, err error)` | Parse `attr set|add|del ...` |
| `parseNLRISection` | `(args []string) (family nlri.Family, announce, withdraw []nlri.NLRI, consumed int, err error)` | Parse `nlri <family> add ... del ...` |

### Family-Specific NLRI Parsing

`parseNLRISection` dispatches to family-specific parsers:

| Family | Parser | NLRI syntax |
|--------|--------|-------------|
| `ipv4/unicast`, `ipv6/unicast` | `parseINETNLRI` | `<prefix>` |
| `ipv4/multicast`, `ipv6/multicast` | `parseINETNLRI` | `<prefix>` |
| `ipv4/mpls-label`, `ipv6/mpls-label` | `parseLabeledNLRI` | `label <N> <prefix>` |
| `ipv4/vpn`, `ipv6/vpn` | `parseVPNNLRI` | `rd <RD> label <N> <prefix>` |
| Others | Return error | "family not supported in text mode" |

**This spec implements:** `ipv4/unicast`, `ipv6/unicast`, `ipv4/multicast`, `ipv6/multicast`
**Future specs:** labeled, VPN, EVPN, FlowSpec

---

## Algorithm

```
ParseUpdateText(args):
    accum = empty parsedAttrs
    groups = []
    watchdog = ""
    i = 0

    while i < len(args):
        switch args[i]:
        case "attr":
            mode, attrs, consumed, err = parseAttrSection(args[i:])
            if err: return err

            switch mode:
            case "set": accum.applySet(attrs)
            case "add":
                if err := accum.applyAdd(attrs); err: return err
            case "del":
                if err := accum.applyDel(attrs); err: return err

            i += consumed

        case "nlri":
            family, add, del, consumed, err = parseNLRISection(args[i:])
            if err: return err

            attrs, nh, nhSelf := accum.snapshot()
            groups.append(NLRIGroup{
                Family:      family,
                Announce:    add,
                Withdraw:    del,
                Attrs:       attrs,
                NextHop:     nh,
                NextHopSelf: nhSelf,
            })
            i += consumed

        case "watchdog":
            if i+1 >= len(args):
                return error("missing watchdog name")
            watchdog = args[i+1]
            i += 2

        default:
            return error("unexpected token: " + args[i])

    return UpdateTextResult{Groups: groups, WatchdogName: watchdog}
```

---

## Reuse from Existing Code

| Function | Location | Reuse | Note |
|----------|----------|-------|------|
| `parseCommonAttribute` | `route.go:448` | Parse origin, med, as-path, community, etc. | Does NOT handle next-hop |
| `parseBracketedList` | `route.go:632` | Parse `[value1 value2]` syntax | |
| `nlri.ParseFamily` | `nlri/nlri.go:201` | Parse "ipv4/unicast" → Family | |
| `nlri.NewINET` | `nlri/inet.go:53` | Create NLRI from prefix | For unicast/multicast |
| `PathAttributes` | `types.go:86` | Existing attribute struct | Excludes next-hop |

### parseAttrSection Loop Structure

Parses `attr <mode> <key> <value> [<key> <value>]...` until hitting boundary keyword:

```go
func parseAttrSection(args []string) (mode string, attrs parsedAttrs, consumed int, err error) {
    // args[0] = "attr"
    if len(args) < 2 {
        return "", parsedAttrs{}, 0, ErrMissingAttrMode
    }
    mode = args[1]
    if mode != "set" && mode != "add" && mode != "del" {
        return "", parsedAttrs{}, 0, ErrInvalidAttrMode
    }

    consumed = 2  // "attr" + mode
    i := 2

    for i < len(args) {
        key := args[i]

        // Boundary keywords end this section
        if key == "attr" || key == "nlri" || key == "watchdog" {
            break
        }

        // Try next-hop (not in parseCommonAttribute)
        switch key {
        case "next-hop":
            if i+1 >= len(args) {
                return "", parsedAttrs{}, 0, fmt.Errorf("missing next-hop value")
            }
            addr, err := netip.ParseAddr(args[i+1])
            if err != nil {
                return "", parsedAttrs{}, 0, fmt.Errorf("invalid next-hop: %w", err)
            }
            attrs.NextHop = addr
            i += 2
            consumed += 2
            continue

        case "next-hop-self":
            attrs.NextHopSelf = true
            i += 1
            consumed += 1
            continue
        }

        // Try parseCommonAttribute for standard attrs
        extra, err := parseCommonAttribute(key, args, i, &attrs.PathAttributes)
        if err != nil {
            return "", parsedAttrs{}, 0, err
        }
        if extra > 0 {
            i += 1 + extra      // key + extra args consumed
            consumed += 1 + extra
            continue
        }

        // Unknown attribute
        return "", parsedAttrs{}, 0, fmt.Errorf("%w: %s", ErrUnknownAttribute, key)
    }

    return mode, attrs, consumed, nil
}
```

**Note:** `parseCommonAttribute` returns extra args consumed (beyond key). So `origin igp` returns `extra=1`.

### parseNLRISection Loop Structure

Parses `nlri <family> [add <prefix>...]... [del <prefix>...]...` with add/del state machine:

```go
func parseNLRISection(args []string) (family nlri.Family, announce, withdraw []nlri.NLRI, consumed int, err error) {
    // args[0] = "nlri"
    if len(args) < 2 {
        return nlri.Family{}, nil, nil, 0, ErrInvalidFamily
    }

    family, ok := nlri.ParseFamily(args[1])
    if !ok {
        return nlri.Family{}, nil, nil, 0, fmt.Errorf("%w: %s", ErrInvalidFamily, args[1])
    }

    // Check if family is supported
    if !isSupportedFamily(family) {
        return nlri.Family{}, nil, nil, 0, fmt.Errorf("%w: %s", ErrFamilyNotSupported, args[1])
    }

    consumed = 2  // "nlri" + family
    i := 2
    mode := ""  // "", "add", or "del"

    for i < len(args) {
        token := args[i]

        // Boundary keywords end this section
        if token == "attr" || token == "nlri" || token == "watchdog" {
            break
        }

        // Mode switches
        if token == "add" {
            mode = "add"
            i++
            consumed++
            continue
        }
        if token == "del" {
            mode = "del"
            i++
            consumed++
            continue
        }

        // Must have a mode before prefixes
        if mode == "" {
            return nlri.Family{}, nil, nil, 0, fmt.Errorf("%w: got %q", ErrMissingAddDel, token)
        }

        // Parse prefix based on family
        n, extra, err := parseINETNLRI(token, family)
        if err != nil {
            return nlri.Family{}, nil, nil, 0, err
        }

        if mode == "add" {
            announce = append(announce, n)
        } else {
            withdraw = append(withdraw, n)
        }
        i += 1 + extra
        consumed += 1 + extra
    }

    // Must have at least one prefix
    if len(announce) == 0 && len(withdraw) == 0 {
        return nlri.Family{}, nil, nil, 0, ErrEmptyNLRISection
    }

    return family, announce, withdraw, consumed, nil
}

// parseINETNLRI parses a single prefix for unicast/multicast families.
func parseINETNLRI(token string, family nlri.Family) (nlri.NLRI, int, error) {
    prefix, err := netip.ParsePrefix(token)
    if err != nil {
        return nil, 0, fmt.Errorf("%w: %s", ErrInvalidPrefix, token)
    }

    // Validate prefix matches family AFI
    isIPv4 := prefix.Addr().Is4()
    if isIPv4 && family.AFI != nlri.AFIIPv4 {
        return nil, 0, fmt.Errorf("%w: IPv4 prefix for %s", ErrFamilyMismatch, family)
    }
    if !isIPv4 && family.AFI != nlri.AFIIPv6 {
        return nil, 0, fmt.Errorf("%w: IPv6 prefix for %s", ErrFamilyMismatch, family)
    }

    return nlri.NewINET(family, prefix, 0), 0, nil  // 0 extra args consumed
}

func isSupportedFamily(f nlri.Family) bool {
    switch f {
    case nlri.IPv4Unicast, nlri.IPv6Unicast, nlri.IPv4Multicast, nlri.IPv6Multicast:
        return true
    default:
        return false
    }
}
```

**Note:** Multiple add/del switches are allowed: `add X Y del Z add W` → announce=[X,Y,W], withdraw=[Z]

---

## New Errors in `pkg/api/errors.go`

```go
var (
    ErrInvalidAttrMode     = errors.New("invalid attr mode (expected set, add, or del)")
    ErrMissingAttrMode     = errors.New("missing attr mode")
    ErrUnknownAttribute    = errors.New("unknown attribute")
    ErrAddOnScalar         = errors.New("'add' not valid for scalar attribute (use 'set')")
    ErrDelOnScalar         = errors.New("'del' not valid for scalar attribute (use 'set')")
    ErrASPathNotAddable    = errors.New("as-path does not support add/del (use 'set')")
    ErrMissingAddDel       = errors.New("expected 'add' or 'del' before prefix")
    ErrEmptyNLRISection    = errors.New("nlri section has no prefixes")
    ErrInvalidFamily       = errors.New("invalid address family")
    ErrFamilyMismatch      = errors.New("NLRI does not match declared family")
    ErrFamilyNotSupported  = errors.New("family not supported in text mode")
    ErrInvalidPrefix       = errors.New("invalid prefix format")
)
```

---

## Validation Rules

### Family validation
- IPv4 prefix → `ipv4/*` families only
- IPv6 prefix → `ipv6/*` families only
- Mismatch → ErrFamilyMismatch

### Attribute validation
- Unknown attribute keyword → ErrUnknownAttribute
- `label`, `rd` in attr section → ErrUnknownAttribute (they're per-NLRI, not attributes)

### Mode validation
- `set` - always valid for any attribute
- `add` - only for list attrs: community, large-community, extended-community
- `del` - only for list attrs: community, large-community, extended-community
- Scalar with add/del → ErrAddOnScalar / ErrDelOnScalar
- **AS-PATH is set-only** - despite being a slice, add/del not supported (path integrity)

### NLRI section
- Must have `add` or `del` keyword before prefixes
- Prefix without mode → ErrMissingAddDel
- No prefixes at all → ErrEmptyNLRISection
- Family-specific syntax enforced by family parser

### Keywords as section boundaries
- `attr`, `nlri`, `watchdog` start new sections
- Unknown token outside section → error

### Watchdog behavior
- Can appear anywhere in command (not just at end)
- Only one watchdog per command - last one wins if multiple specified
- `watchdog` without routes is valid but useless (Groups=[])
