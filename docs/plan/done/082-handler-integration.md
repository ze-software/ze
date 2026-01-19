# Spec: handler-integration

## Task

Wire Chunk 1's parser to reactor via `peer <addr> update text ...` handler.

**Chunk 2 of 10** for announce-family-first refactor. Depends on Chunk 1 (parser).

---

## Required Reading

- [ ] `.claude/zebgp/api/ARCHITECTURE.md` - API command patterns, Response structure
- [ ] `.claude/zebgp/api/CAPABILITY_CONTRACT.md` - ReactorInterface contract
- [ ] `.claude/zebgp/UPDATE_BUILDING.md` - UPDATE message construction
- [ ] `.claude/zebgp/wire/NLRI.md` - NLRI types per family
- [ ] `.claude/zebgp/ENCODING_CONTEXT.md` - Peer-specific encoding context

**Key insights:**
- Reactor methods use `*reactorAPIAdapter` receiver, not `*Reactor`
- `sendUpdateWithSplit` handles message size limits (build path)
- `SplitWireUpdate` is for forward path (relay received UPDATEs)
- Existing methods use `lastErr` pattern for multi-peer errors
- `UpdateBuilder` exists for consistent UPDATE construction

---

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| `TestHandleUpdateText_SimpleAnnounce` | `internal/plugin/update_text_test.go` | Single route announced |
| `TestHandleUpdateText_MultipleRoutes` | `internal/plugin/update_text_test.go` | Multiple NLRIs batched in one group |
| `TestHandleUpdateText_MixedAnnounceWithdraw` | `internal/plugin/update_text_test.go` | Add and del in same call |
| `TestHandleUpdateText_MultipleGroups` | `internal/plugin/update_text_test.go` | Different attrs per group |
| `TestHandleUpdateText_WithdrawUnicast` | `internal/plugin/update_text_test.go` | Unicast withdrawal batch |
| `TestHandleUpdateText_WithdrawLabeled` | `internal/plugin/update_text_test.go` | Labeled unicast withdrawal |
| `TestHandleUpdateText_WithdrawL3VPN` | `internal/plugin/update_text_test.go` | L3VPN withdrawal batch |
| `TestHandleUpdateText_ParseError` | `internal/plugin/update_text_test.go` | Invalid input returns error |
| `TestHandleUpdateText_PeerNotFound` | `internal/plugin/update_text_test.go` | Reactor returns no peers error |
| `TestHandleUpdateText_InvalidFamily` | `internal/plugin/update_text_test.go` | Unsupported family error |
| `TestHandleUpdateText_WatchdogDeferred` | `internal/plugin/update_text_test.go` | Watchdog returns error (deferred) |
| `TestHandleUpdateText_EmptyResult` | `internal/plugin/update_text_test.go` | Empty groups returns warning |
| `TestAnnounceNLRIBatch_LargeBatch` | `internal/reactor/reactor_batch_test.go` | Batch exceeding wire limit splits |
| `TestAnnounceNLRIBatch_PeerCapabilities` | `internal/reactor/reactor_batch_test.go` | Respects ExtendedMessage |
| `TestWithdrawNLRIBatch_MultipleNLRIs` | `internal/reactor/reactor_batch_test.go` | Batched withdraw |

### Functional Tests

| Test | Location | Scenario |
|------|----------|----------|
| N/A | - | Handler tests with mock reactor sufficient |

---

## Files to Modify

- `internal/plugin/types.go` - Add `NLRIBatch` type, add methods to `ReactorInterface`
- `internal/plugin/update_text.go` - Add handler functions, batch dispatch
- `internal/plugin/update_text_test.go` - **CREATE** Handler tests
- `internal/plugin/route.go` - Register "update" command in `RegisterRouteHandlers`
- `internal/reactor/reactor.go` - Add `AnnounceNLRIBatch`, `WithdrawNLRIBatch` methods
- `internal/reactor/reactor_batch_test.go` - **CREATE** Batch method tests

---

## Implementation Steps

1. **Write tests** - Create `internal/plugin/update_text_test.go` and `internal/reactor/reactor_batch_test.go`
2. **Run tests** - Verify FAIL (paste output)
3. **Add types** - Add `NLRIBatch` to `internal/plugin/types.go`
4. **Add interface** - Add `AnnounceNLRIBatch`, `WithdrawNLRIBatch` to `ReactorInterface`
5. **Implement reactor** - Add methods to `internal/reactor/reactor.go`
6. **Run reactor tests** - Verify PASS (paste output)
7. **Implement handler** - Add functions to `internal/plugin/update_text.go`
8. **Register command** - Add to `RegisterRouteHandlers` in `internal/plugin/route.go`
9. **Run handler tests** - Verify PASS (paste output)
10. **Verify all** - `make lint && make test && make functional`
11. **RFC refs** - Add RFC comments to protocol code

---

## Design Decisions

### Error Handling
- **Parse phase**: Side-effect free. If parsing fails, return error immediately.
- **Announce phase**: After successful parse, announce all routes.
- **No transaction needed**: Single command is atomic by design (parse-then-announce).

### Error Semantics (Handler vs Reactor)
- **Handler**: Fails fast on first group error, returns immediately.
- **Reactor**: Tries all peers, collects errors via `lastErr` pattern.
- **Implication**: Best-effort per peer is acceptable for API commands.

### Batching Strategy
- **Handler batches logically**: Groups NLRIs with same attributes (from parser's NLRIGroup).
- **Reactor handles wire format**: New method takes batched NLRIs + attrs, builds wire per peer.
- **Splitting in reactor**: Reactor uses `sendUpdateWithSplit` (build path).
- **Why not handler?**: Wire format needs peer-specific context (ADD-PATH, ASN4, max size).

### Splitting Functions (Two Paths)
| Function | Input | Path | When to Use |
|----------|-------|------|-------------|
| `sendUpdateWithSplit` | `*message.Update` | Build | New routes from API/config |
| `SplitWireUpdate` | `*WireUpdate` | Forward | Relay received UPDATEs |

### Why NLRIBatch (not existing route types)
Existing types (`RouteSpec`, `L3VPNRoute`, etc.) are **per-route** - one NLRI each.
`NLRIBatch` groups **multiple NLRIs with shared attributes** for efficient UPDATE building.

### Response Status
- **"done"**: Success with announced/withdrawn counts
- **"error"**: Parse or reactor failure
- **"warning"**: NEW status - empty result (no routes to process)

### Watchdog Support
- **Deferred**: Watchdog must support all families.
- **Current behavior**: Return error if watchdog specified.

---

## RFC Documentation

- `// RFC 4271 Section 4.3` - UPDATE Message Format
- `// RFC 4760` - Multiprotocol Extensions (MP_REACH_NLRI, MP_UNREACH_NLRI)
- `// RFC 8654` - Extended Message Support (65535 byte limit)

If RFC missing:
```bash
curl -o rfc/rfc4271.txt https://www.rfc-editor.org/rfc/rfc4271.txt
curl -o rfc/rfc4760.txt https://www.rfc-editor.org/rfc/rfc4760.txt
curl -o rfc/rfc8654.txt https://www.rfc-editor.org/rfc/rfc8654.txt
```

---

## Existing Code to Reuse

| Function | Location | Purpose |
|----------|----------|---------|
| `message.MaxMessageLength()` | `internal/bgp/message/` | Get max size per peer |
| `message.NewUpdateBuilder()` | `internal/bgp/message/` | Build batched UPDATE |
| `peer.sendUpdateWithSplit()` | `internal/reactor/peer.go:1487` | Size-aware sending |
| `peer.packContext()` | `internal/reactor/peer.go` | Get ADD-PATH/ASN4 context |
| `getMatchingPeers()` | `internal/reactor/reactor.go` | Peer selection |

---

## Code Snippets

### NLRIBatch Type

```go
// NLRIBatch represents a batch of NLRIs with shared attributes.
// Used for efficient UPDATE message generation - reactor builds wire format
// and splits into multiple messages if exceeding peer's max size.
type NLRIBatch struct {
    Family      nlri.Family    // AFI/SAFI for all NLRIs
    NLRIs       []nlri.NLRI    // NLRIs to announce or withdraw
    NextHop     netip.Addr     // Next-hop (announce only)
    NextHopSelf bool           // Use peer's local address (announce only)
    Attrs       PathAttributes // Shared attributes (announce only)
}
```

### ReactorInterface Additions

```go
// AnnounceNLRIBatch announces a batch of NLRIs with shared attributes.
// Builds wire-format UPDATE(s), splits if exceeding peer's max message size.
AnnounceNLRIBatch(peerSelector string, batch NLRIBatch) error

// WithdrawNLRIBatch withdraws a batch of NLRIs.
// Builds wire-format UPDATE(s), splits if exceeding peer's max message size.
WithdrawNLRIBatch(peerSelector string, batch NLRIBatch) error
```

### Reactor Implementation

```go
// AnnounceNLRIBatch announces a batch of NLRIs with shared attributes.
// RFC 4271 Section 4.3: UPDATE Message Format
// RFC 4760: MP_REACH_NLRI for non-IPv4-unicast families
// RFC 8654: Respects peer's max message size (4096 or 65535)
func (a *reactorAPIAdapter) AnnounceNLRIBatch(peerSelector string, batch api.NLRIBatch) error {
    peers := a.getMatchingPeers(peerSelector)
    if len(peers) == 0 {
        return errors.New("no peers match selector")
    }

    var lastErr error
    for _, peer := range peers {
        nc := peer.negotiated.Load()
        maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, nc != nil && nc.ExtendedMessage))
        ctx := peer.packContext(batch.Family)

        update := a.buildBatchUpdate(batch, ctx, peer.sendCtx, peer.Settings())

        if err := peer.sendUpdateWithSplit(update, maxMsgSize, batch.Family); err != nil {
            lastErr = err
            continue
        }
    }
    return lastErr
}

// WithdrawNLRIBatch withdraws a batch of NLRIs.
// RFC 4271 Section 4.3: Withdrawn Routes field
// RFC 4760: MP_UNREACH_NLRI for non-IPv4-unicast families
func (a *reactorAPIAdapter) WithdrawNLRIBatch(peerSelector string, batch api.NLRIBatch) error {
    peers := a.getMatchingPeers(peerSelector)
    if len(peers) == 0 {
        return errors.New("no peers match selector")
    }

    var lastErr error
    for _, peer := range peers {
        nc := peer.negotiated.Load()
        maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, nc != nil && nc.ExtendedMessage))
        ctx := peer.packContext(batch.Family)

        update := a.buildBatchWithdraw(batch, ctx)

        if err := peer.sendUpdateWithSplit(update, maxMsgSize, batch.Family); err != nil {
            lastErr = err
            continue
        }
    }
    return lastErr
}
```

### Handler Functions

```go
// handleUpdate dispatches update subcommands by encoding.
// Syntax: peer <addr> update <encoding> ...
func handleUpdate(ctx *CommandContext, args []string) (*Response, error) {
    if len(args) < 1 {
        return nil, fmt.Errorf("usage: peer <addr> update <text|hex|b64|cbor> ...")
    }

    encoding := strings.ToLower(args[0])
    switch encoding {
    case "text":
        return handleUpdateText(ctx, args[1:])
    case "hex", "b64", "cbor":
        return nil, fmt.Errorf("wire encoding %s not yet implemented", encoding)
    default:
        return nil, fmt.Errorf("unknown encoding: %s", encoding)
    }
}

// handleUpdateText handles: peer <addr> update text ...
func handleUpdateText(ctx *CommandContext, args []string) (*Response, error) {
    result, err := ParseUpdateText(args)
    if err != nil {
        return &Response{Status: "error", Data: err.Error()}, err
    }

    if result.WatchdogName != "" {
        return &Response{
            Status: "error",
            Data:   "watchdog not yet implemented for update text",
        }, fmt.Errorf("watchdog not yet implemented")
    }

    peerSelector := ctx.PeerSelector()

    for _, group := range result.Groups {
        if len(group.Announce) > 0 {
            if err := announceNLRIBatch(ctx, peerSelector, group); err != nil {
                return &Response{Status: "error", Data: err.Error()}, err
            }
        }
        if len(group.Withdraw) > 0 {
            if err := withdrawNLRIBatch(ctx, peerSelector, group); err != nil {
                return &Response{Status: "error", Data: err.Error()}, err
            }
        }
    }

    announced := countAnnounced(result)
    withdrawn := countWithdrawn(result)

    if announced == 0 && withdrawn == 0 {
        return &Response{
            Status: "warning",
            Data:   "no routes to announce or withdraw",
        }, nil
    }

    return &Response{
        Status: "done",
        Data: map[string]any{
            "announced": announced,
            "withdrawn": withdrawn,
        },
    }, nil
}
```

---

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (verified before implementation)
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes (pre-existing deprecated warnings only)
- [x] `make test` passes (pre-existing flaky test unrelated)
- [x] `make functional` passes (28/28)

### Documentation
- [x] Required docs read
- [x] RFC references added (RFC 4271, 4760, 8654)
- [x] `.claude/zebgp/api/ARCHITECTURE.md` updated (new "warning" status)

### Completion
- [x] Spec moved to `docs/plan/done/082-handler-integration.md`

---

## Implementation Notes

### Additional Features (Beyond Original Spec)

1. **Warning on family skip**: Returns `ErrNoPeersAcceptedFamily` when all peers skip due to family not negotiated. Handler converts to warning response.

2. **Partial success**: Mixed success/warning returns `done` status with `warnings` field in response data.

### Files Modified

| File | Changes |
|------|---------|
| `internal/plugin/errors.go` | +`ErrNoPeersAcceptedFamily` |
| `internal/plugin/types.go` | +`NLRIBatch`, +interface methods |
| `internal/plugin/update_text.go` | +handlers, +warning logic |
| `internal/plugin/update_text_test.go` | +18 handler tests |
| `internal/plugin/route.go` | +register "update" command |
| `internal/reactor/reactor.go` | +batch methods with splitting, family check, queue |
| `internal/reactor/reactor_batch_test.go` | +12 reactor tests |
| `internal/plugin/handler_test.go` | +interface stubs |
| `internal/plugin/forward_test.go` | +interface stubs |
| `.claude/zebgp/api/ARCHITECTURE.md` | +warning status docs |
