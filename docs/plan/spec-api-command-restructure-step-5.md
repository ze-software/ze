# Spec: API Command Restructure - Step 5: BGP Command Migration

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/ipc_protocol.md` - target protocol spec
4. `internal/plugin/handler.go` - current command registration
5. `internal/plugin/route.go` - update/watchdog handlers
6. `internal/plugin/commit.go` - commit handlers

## Task

Move all BGP-related commands under the `bgp` namespace.

**Command migrations:**

| Old | New |
|-----|-----|
| `daemon shutdown` | `bgp daemon shutdown` |
| `daemon status` | `bgp daemon status` |
| `daemon reload` | `bgp daemon reload` |
| *new* | `bgp daemon restart` |
| `peer list` | `bgp list` |
| `peer show` | `bgp show` |
| `peer show <ip>` | `bgp peer <sel> show` |
| `peer teardown <ip>` | `bgp peer <sel> teardown` |
| `peer <sel> update ...` | `bgp peer <sel> update ...` |
| `peer <sel> borr` | `bgp peer <sel> borr` |
| `peer <sel> eorr` | `bgp peer <sel> eorr` |
| `peer <sel> session api ready` | `bgp peer <sel> ready` |
| *new* | `bgp peer <sel> tcp reset` |
| *new* | `bgp peer <sel> tcp ttl <num>` |
| `commit <name> ...` | `bgp commit <name> ...` |
| `watchdog announce <name>` | `bgp watchdog announce <name>` |
| `watchdog withdraw <name>` | `bgp watchdog withdraw <name>` |
| `raw <type> <enc> <data>` | `bgp raw <type> <enc> <data>` |

**Remove:**
- `neighbor` prefix support - use `peer` only
- `teardown` standalone handler - integrated into `bgp peer <sel> teardown`

**No backward compatibility** - old commands will fail.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/ipc_protocol.md` - target command structure

### Source Files
- [ ] `internal/plugin/handler.go` - daemon/peer handlers
- [ ] `internal/plugin/route.go` - update/watchdog handlers
- [ ] `internal/plugin/commit.go` - commit handlers
- [ ] `internal/plugin/raw.go` - raw handler
- [ ] `internal/plugin/refresh.go` - borr/eorr handlers
- [ ] `internal/plugin/command.go` - dispatcher (neighbor prefix)

## Current State

**handler.go registrations:**
```go
d.Register("daemon shutdown", handleDaemonShutdown, ...)
d.Register("daemon status", handleDaemonStatus, ...)
d.Register("daemon reload", handleDaemonReload, ...)
d.Register("peer list", handlePeerList, ...)
d.Register("peer show", handlePeerShow, ...)
d.Register("peer teardown", handlePeerTeardown, ...)
d.Register("teardown", handleTeardown, ...)  // neighbor <ip> teardown
```

**route.go:**
```go
d.Register("update", handleUpdate, ...)
d.Register("watchdog announce", handleWatchdogAnnounce, ...)
d.Register("watchdog withdraw", handleWatchdogWithdraw, ...)
```

**command.go Dispatch():**
```go
if (prefix == "neighbor" || prefix == "peer") && len(tokens) >= 3 {
    // Extract peer selector
}
```

## Target State

**New registrations in handler.go:**
```go
// BGP daemon control
d.Register("bgp daemon shutdown", handleBgpDaemonShutdown, "Stop BGP subsystem")
d.Register("bgp daemon status", handleBgpDaemonStatus, "BGP subsystem status")
d.Register("bgp daemon reload", handleBgpDaemonReload, "Reload BGP configuration")
d.Register("bgp daemon restart", handleBgpDaemonRestart, "Restart BGP subsystem")

// BGP peer listing
d.Register("bgp list", handleBgpList, "List all peers (brief)")
d.Register("bgp show", handleBgpShow, "Show all peers (detailed)")

// BGP peer operations (use "bgp peer <sel>" prefix in dispatcher)
d.Register("bgp peer show", handleBgpPeerShow, "Show specific peer")
d.Register("bgp peer teardown", handleBgpPeerTeardown, "Graceful close (NOTIFICATION)")
d.Register("bgp peer update", handleBgpPeerUpdate, "Announce/withdraw routes")
d.Register("bgp peer borr", handleBgpPeerBoRR, "Begin-of-Route-Refresh")
d.Register("bgp peer eorr", handleBgpPeerEoRR, "End-of-Route-Refresh")
d.Register("bgp peer ready", handleBgpPeerReady, "Signal peer replay complete")
d.Register("bgp peer tcp reset", handleBgpPeerTcpReset, "Force TCP RST")
d.Register("bgp peer tcp ttl", handleBgpPeerTcpTtl, "Set TTL (multi-hop)")

// BGP commits
d.Register("bgp commit", handleBgpCommit, "Named commit operations")

// BGP watchdog
d.Register("bgp watchdog announce", handleBgpWatchdogAnnounce, "Announce pool routes")
d.Register("bgp watchdog withdraw", handleBgpWatchdogWithdraw, "Withdraw pool routes")

// BGP raw
d.Register("bgp raw", handleBgpRaw, "Send raw BGP message")
```

**Updated dispatcher:**
- Change `neighbor|peer` prefix to `bgp peer`
- Pattern: `bgp peer <sel> <command> [args]`

## 🧪 TDD Test Plan

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| TTL | 1-255 | 255 | 0 | 256 |
| teardown subcode | 0-255 | 255 | N/A (optional) | 256 |

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDispatchBgpDaemonShutdown` | `internal/plugin/handler_test.go` | `bgp daemon shutdown` | |
| `TestDispatchBgpDaemonStatus` | `internal/plugin/handler_test.go` | `bgp daemon status` | |
| `TestDispatchBgpList` | `internal/plugin/handler_test.go` | `bgp list` returns peers | |
| `TestDispatchBgpShow` | `internal/plugin/handler_test.go` | `bgp show` returns details | |
| `TestDispatchBgpPeerShow` | `internal/plugin/handler_test.go` | `bgp peer 10.0.0.1 show` | |
| `TestDispatchBgpPeerTeardown` | `internal/plugin/handler_test.go` | `bgp peer * teardown` | |
| `TestDispatchBgpPeerUpdate` | `internal/plugin/handler_test.go` | `bgp peer * update text ...` | |
| `TestDispatchBgpPeerReady` | `internal/plugin/handler_test.go` | `bgp peer 10.0.0.1 ready` | |
| `TestDispatchBgpPeerTcpReset` | `internal/plugin/handler_test.go` | `bgp peer 10.0.0.1 tcp reset` | |
| `TestDispatchBgpCommit` | `internal/plugin/handler_test.go` | `bgp commit batch1 start` | |
| `TestDispatchBgpWatchdog` | `internal/plugin/handler_test.go` | `bgp watchdog announce pool1` | |
| `TestDispatchBgpRaw` | `internal/plugin/handler_test.go` | `bgp peer 10.0.0.1 raw ...` | |
| `TestBgpPeerTcpTtlBoundary` | `internal/plugin/handler_test.go` | TTL 0 fails, 1 works, 255 works, 256 fails | |
| `TestBgpPeerTeardownSubcodeBoundary` | `internal/plugin/handler_test.go` | Subcode 0-255 valid, 256 fails | |
| `TestOldDaemonCommandsRemoved` | `internal/plugin/handler_test.go` | `daemon shutdown` fails | |
| `TestOldPeerCommandsRemoved` | `internal/plugin/handler_test.go` | `peer list` fails | |
| `TestNeighborPrefixRemoved` | `internal/plugin/handler_test.go` | `neighbor * update` fails | |

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `bgp-peer-update` | `test/data/plugin/bgp-peer-update.ci` | Route injection with new syntax | |
| `bgp-daemon-control` | `test/data/plugin/bgp-daemon-control.ci` | Daemon commands | |

## Files to Modify

| File | Changes |
|------|---------|
| `internal/plugin/handler.go` | Rename handlers, update registrations |
| `internal/plugin/route.go` | Rename handlers, update registrations |
| `internal/plugin/commit.go` | Rename handler, update registration |
| `internal/plugin/raw.go` | Rename handler, update registration |
| `internal/plugin/refresh.go` | Rename handlers, update registrations |
| `internal/plugin/command.go` | Update dispatcher for `bgp peer <sel>` prefix |

## Dispatcher Changes

**Current pattern:**
```
neighbor <sel> <command>
peer <sel> <command>
```

**New pattern:**
```
bgp peer <sel> <command>
```

**command.go Dispatch() update:**
```go
// Check for "bgp peer <sel>" prefix
if len(tokens) >= 3 && tokens[0] == "bgp" && tokens[1] == "peer" {
    if looksLikeIPOrGlob(tokens[2]) {
        peerSelector = tokens[2]
        if ctx != nil {
            ctx.Peer = peerSelector
        }
        // Rebuild: "bgp peer <command>" (without selector)
        newTokens := []string{"bgp", "peer"}
        newTokens = append(newTokens, tokens[3:]...)
        input = strings.Join(newTokens, " ")
    }
}
```

## New Handler: bgp peer tcp reset

```go
func handleBgpPeerTcpReset(ctx *CommandContext, _ []string) (*Response, error) {
    peer := ctx.PeerSelector()
    if peer == "*" {
        return nil, fmt.Errorf("tcp reset requires specific peer")
    }

    addr, err := netip.ParseAddr(peer)
    if err != nil {
        return nil, fmt.Errorf("invalid peer address: %s", peer)
    }

    if err := ctx.Reactor.ResetTCP(addr); err != nil {
        return nil, fmt.Errorf("tcp reset failed: %v", err)
    }

    return NewResponse("done", map[string]any{
        "peer":   peer,
        "action": "tcp reset",
    }), nil
}
```

## New Handler: bgp peer tcp ttl

```go
func handleBgpPeerTcpTtl(ctx *CommandContext, args []string) (*Response, error) {
    if len(args) < 1 {
        return nil, fmt.Errorf("usage: bgp peer <sel> tcp ttl <num>")
    }

    peer := ctx.PeerSelector()
    if peer == "*" {
        return nil, fmt.Errorf("tcp ttl requires specific peer")
    }

    ttl, err := strconv.ParseUint(args[0], 10, 8)
    if err != nil || ttl == 0 || ttl > 255 {
        return nil, fmt.Errorf("invalid TTL: %s (must be 1-255)", args[0])
    }

    addr, err := netip.ParseAddr(peer)
    if err != nil {
        return nil, fmt.Errorf("invalid peer address: %s", peer)
    }

    if err := ctx.Reactor.SetPeerTTL(addr, uint8(ttl)); err != nil {
        return nil, fmt.Errorf("set ttl failed: %v", err)
    }

    return NewResponse("done", map[string]any{
        "peer": peer,
        "ttl":  ttl,
    }), nil
}
```

**Note:** Handlers return `*Response`. The `WrapResponse()` function wraps at serialization time.

## Implementation Steps

1. **Write unit tests** - Create tests for all migrated commands
2. **Run tests** - Verify FAIL (paste output)
3. **Update command.go** - Change dispatcher to handle `bgp peer <sel>` prefix
4. **Rename handlers** - Add `Bgp` prefix to all handlers
5. **Update registrations** - New paths with `bgp` prefix
6. **Add new handlers** - `tcp reset`, `tcp ttl`, `daemon restart`
7. **Remove old registrations** - Delete all old paths
8. **Remove neighbor support** - Only `bgp peer` works now
9. **Run tests** - Verify PASS (paste output)
10. **Verify all** - `make lint && make test && make functional` (paste output)

## Reactor Interface Additions

May need to add to ReactorInterface:
```go
// ResetTCP forces a TCP RST to the peer.
ResetTCP(addr netip.Addr) error

// SetPeerTTL sets the TTL for packets to a peer (multi-hop BGP).
SetPeerTTL(addr netip.Addr, ttl uint8) error

// RestartBGP restarts the BGP subsystem.
RestartBGP() error
```

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

### Completion
- [ ] All files committed together
- [ ] Spec moved to `docs/plan/done/`
