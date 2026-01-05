# Spec: wireupdate-split

## Task

Implement wire-level UPDATE splitting without parsing to Route objects.

## Required Reading

- [x] `.claude/zebgp/UPDATE_BUILDING.md` - Build vs Forward paths
- [x] `.claude/zebgp/ENCODING_CONTEXT.md` - Zero-copy wire handling
- [x] `.claude/zebgp/wire/MESSAGES.md` - UPDATE structure
- [x] `.claude/zebgp/wire/NLRI.md` - NLRI formats
- [x] `.claude/zebgp/wire/ATTRIBUTES.md` - MP_REACH/MP_UNREACH

**Key insights:**
- Forward path preserves wire bytes, no Route parsing needed
- WireUpdate holds raw payload, can slice without allocation
- MP_REACH/MP_UNREACH are attributes containing NLRIs
- NLRI format: length-prefix (1 byte for IPv4) + data

## Problem

Current split path:
```
WireUpdate → ConvertToRoutes() → []*Route → Build new UPDATEs
```

Issues:
- `ConvertToRoutes()` requires `Announces`/`Withdraws` fields (never populated)
- Full Route parsing is wasteful for simple split operation
- Creates unnecessary objects

## Target Design

```
WireUpdate → SplitUpdate(maxSize) → []*WireUpdate
```

**Principle:** Rare operation → simplicity over performance. Just copy raw bytes.

**Split order:**
1. **Withdraws UPDATE(s)** — IPv4 withdrawn + MP_UNREACH, no attributes
2. **MP Announces UPDATE(s)** — Attributes + MP_REACH NLRIs
3. **IPv4 Announces UPDATE(s)** — Attributes + IPv4 NLRIs

**Algorithm for each category:**
```
if total_size ≤ max_size:
    copy all at once (fast path - nearly always taken)
else:
    iterate: add one NLRI at a time until full, start new UPDATE, repeat
```

**Why this always works:**
- Withdraws need no attributes
- Each announce UPDATE has attrs + ≥1 NLRI
- Only fails if single NLRI > max size (invalid per RFC)

## UPDATE Structure (RFC 4271)

```
+-----------------------------------------------------+
| Withdrawn Routes Length (2 bytes)                   |
+-----------------------------------------------------+
| Withdrawn Routes (variable)                         |  ← IPv4 withdraws
+-----------------------------------------------------+
| Total Path Attribute Length (2 bytes)               |
+-----------------------------------------------------+
| Path Attributes (variable)                          |  ← Contains MP_REACH/MP_UNREACH
+-----------------------------------------------------+
| NLRI (variable)                                     |  ← IPv4 announces
+-----------------------------------------------------+
```

**Split targets:**
- IPv4 withdraws: Withdrawn Routes field
- IPv4 announces: NLRI field
- IPv6/VPN/etc: MP_REACH_NLRI / MP_UNREACH_NLRI attributes

## Implementation

### 1. New Function in `pkg/api/wire_update.go`

```go
// SplitUpdate splits a WireUpdate into multiple smaller ones.
// Each output payload fits within maxBodySize (UPDATE body, excludes 19-byte header).
// Returns original in single-element slice if no split needed.
// Returns error if UPDATE cannot be split (e.g., single NLRI > maxSize).
func SplitUpdate(wu *WireUpdate, maxBodySize int) ([]*WireUpdate, error)
```

### 2. Split Algorithm

```go
func SplitUpdate(wu *WireUpdate, maxBodySize int) ([]*WireUpdate, error) {
    payload := wu.Payload()

    // Fast path: no split needed
    if len(payload) <= maxBodySize {
        return []*WireUpdate{wu}, nil
    }

    // Parse structure (offsets only)
    withdrawnLen := binary.BigEndian.Uint16(payload[0:2])
    withdrawnEnd := 2 + int(withdrawnLen)
    attrLen := binary.BigEndian.Uint16(payload[withdrawnEnd:withdrawnEnd+2])
    attrStart := withdrawnEnd + 2
    attrEnd := attrStart + int(attrLen)

    // Extract components as wire slices
    ipv4Withdraws := payload[2:withdrawnEnd]
    attrs := payload[attrStart:attrEnd]
    ipv4NLRI := payload[attrEnd:]

    // Separate MP_REACH/MP_UNREACH from base attributes
    baseAttrs, mpReach, mpUnreach := splitMPAttributes(attrs)

    var results []*WireUpdate

    // 1. Withdraws UPDATE(s): IPv4 withdrawn + MP_UNREACH, no attributes
    results = append(results, buildWithdrawUpdates(ipv4Withdraws, mpUnreach, maxBodySize, wu.SourceCtxID())...)

    // 2. MP Announces UPDATE(s): baseAttrs + MP_REACH NLRIs
    results = append(results, buildMPAnnounceUpdates(baseAttrs, mpReach, maxBodySize, wu.SourceCtxID())...)

    // 3. IPv4 Announces UPDATE(s): baseAttrs + IPv4 NLRIs
    results = append(results, buildIPv4AnnounceUpdates(baseAttrs, ipv4NLRI, maxBodySize, wu.SourceCtxID())...)

    return results, nil
}
```

### 3. Helper Functions

```go
// buildWithdrawUpdates creates UPDATE(s) with withdraws only (no attributes).
// Fast path: if all fit, single copy. Slow path: iterate one NLRI at a time.
func buildWithdrawUpdates(ipv4Withdraws, mpUnreach []byte, maxSize int, ctxID bgpctx.ContextID) []*WireUpdate

// buildMPAnnounceUpdates creates UPDATE(s) with baseAttrs + MP_REACH.
// Fast path: if all fit, single copy. Slow path: iterate one NLRI at a time.
func buildMPAnnounceUpdates(baseAttrs, mpReach []byte, maxSize int, ctxID bgpctx.ContextID) []*WireUpdate

// buildIPv4AnnounceUpdates creates UPDATE(s) with baseAttrs + IPv4 NLRIs.
// Fast path: if all fit, single copy. Slow path: iterate one NLRI at a time.
func buildIPv4AnnounceUpdates(baseAttrs, ipv4NLRI []byte, maxSize int, ctxID bgpctx.ContextID) []*WireUpdate

// splitMPAttributes extracts MP_REACH and MP_UNREACH from attributes.
// Returns: baseAttrs (without MP_*), mpReach wire bytes, mpUnreach wire bytes.
func splitMPAttributes(attrs []byte) (base, mpReach, mpUnreach []byte)

// nextNLRI returns the next NLRI from wire bytes and remaining bytes.
// Returns (nlri, remaining, error). NLRI format: length-prefix + data.
func nextNLRI(data []byte) ([]byte, []byte, error)
```

### 4. Update ForwardUpdateByID

```go
// In pkg/reactor/reactor.go ForwardUpdateByID

if updateSize > maxMsgSize {
    // Split path: UPDATE too large for this peer
    maxBody := maxMsgSize - message.HeaderLen
    splits, err := api.SplitUpdate(update.WireUpdate, maxBody)
    if err != nil {
        errs = append(errs, fmt.Errorf("peer %s: split failed: %w", peer.Settings().Address, err))
        continue
    }
    for _, split := range splits {
        if err := peer.SendRawUpdateBody(split.Payload()); err != nil {
            errs = append(errs, fmt.Errorf("peer %s: %w", peer.Settings().Address, err))
        }
    }
}
```

## Files Modified

| File | Changes |
|------|---------|
| `pkg/api/wire_update.go` | Add `SplitUpdate()` and helpers |
| `pkg/api/wire_update_test.go` | Tests for split function |
| `pkg/reactor/reactor.go` | Rewrite split path in `ForwardUpdateByID` |
| `pkg/reactor/received_update.go` | Delete `Announces`, `Withdraws`, `AnnounceWire`, `WithdrawWire`, `ConvertToRoutes()` |
| `pkg/reactor/received_update_test.go` | Delete `ConvertToRoutes` tests |
| `pkg/reactor/forward_split_test.go` | Update to use new split API |

## Edge Cases

| Case | Handling |
|------|----------|
| Single NLRI > maxSize | Return error (cannot split) |
| Empty UPDATE | Return original (fast path) |
| Only attributes (no NLRI) | Return original (fast path) |
| Mixed IPv4 + MP_REACH | Split each category independently |
| All withdraws fit | Single withdraw UPDATE (fast path) |
| All MP announces fit | Single MP announce UPDATE (fast path) |
| All IPv4 announces fit | Single IPv4 announce UPDATE (fast path) |

## TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| `TestSplitUpdateNoSplitNeeded` | `wire_update_test.go` | Small UPDATE returned as-is (fast path) |
| `TestSplitUpdateIPv4WithdrawsFastPath` | `wire_update_test.go` | IPv4 withdraws fit → single copy |
| `TestSplitUpdateIPv4WithdrawsIterate` | `wire_update_test.go` | IPv4 withdraws overflow → iterate |
| `TestSplitUpdateMPUnreachFastPath` | `wire_update_test.go` | MP_UNREACH fits → single copy |
| `TestSplitUpdateMPUnreachIterate` | `wire_update_test.go` | MP_UNREACH overflow → iterate |
| `TestSplitUpdateMPReachFastPath` | `wire_update_test.go` | Attrs + MP_REACH fits → single copy |
| `TestSplitUpdateMPReachIterate` | `wire_update_test.go` | Attrs + MP_REACH overflow → iterate |
| `TestSplitUpdateIPv4NLRIFastPath` | `wire_update_test.go` | Attrs + IPv4 NLRI fits → single copy |
| `TestSplitUpdateIPv4NLRIIterate` | `wire_update_test.go` | Attrs + IPv4 NLRI overflow → iterate |
| `TestSplitUpdateSingleNLRITooLarge` | `wire_update_test.go` | Error when single NLRI > max |
| `TestSplitUpdatePreservesAttributes` | `wire_update_test.go` | Base attrs in all announce splits |
| `TestSplitUpdateWithdrawsNoAttrs` | `wire_update_test.go` | Withdraw UPDATEs have no attributes |

### Functional Tests

Existing `make functional` should pass - split behavior tested via ForwardUpdateByID.

## Implementation Steps

1. **Write tests** - Create unit tests for `SplitUpdate()` and helpers
2. **Run tests** - Verify FAIL (paste output)
3. **Implement** - Minimal code to pass
4. **Run tests** - Verify PASS (paste output)
5. **Verify all** - `make lint && make test && make functional`
6. **RFC refs** - Add RFC 4271 comments to protocol code

## RFC Documentation

- Add `// RFC 4271 Section 4.3` comments to UPDATE parsing
- Reference RFC 4760 for MP_REACH/MP_UNREACH handling

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)

### Implementation
- [ ] Add `SplitUpdate()` to `pkg/api/wire_update.go`
- [ ] Add `buildWithdrawUpdates()` helper
- [ ] Add `buildMPAnnounceUpdates()` helper
- [ ] Add `buildIPv4AnnounceUpdates()` helper
- [ ] Add `splitMPAttributes()` helper
- [ ] Add `nextNLRI()` helper
- [ ] Update `ForwardUpdateByID` to use `SplitUpdate()`
- [ ] Delete `ConvertToRoutes()` and related fields
- [ ] Delete orphaned tests

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] RFC references added
- [ ] Update `MESSAGE_BUFFER_DESIGN.md` if needed

### Completion
- [ ] Spec moved to `plan/done/NNN-<name>.md`

---

**Created:** 2025-01-05
**Status:** 📋 Planned
