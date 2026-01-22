# Spec: API Command Restructure

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/ipc_protocol.md` - canonical protocol spec
4. `internal/plugin/handler.go` - dispatch implementation

## Task

Restructure API commands with clear namespace separation:
- `plugin` namespace for plugin lifecycle (session ready/ping/bye)
- `bgp` namespace for all BGP operations
- `rib` namespace for RIB operations (plugin-provided)
- `system` namespace for application-level operations
- Event subscription model (API-driven, not config-driven)
- Plugin-provided commands (bgp cache, rib show/clear)
- Remove CBOR encoding, session reset, config-driven event routing

## Concepts

### Subsystems

A **subsystem** is a protocol component that follows the ZE API for plugin communication:
- `bgp` - BGP protocol subsystem
- `rib` - RIB subsystem (protocol-agnostic)
- Future: `bmp`, `rpki`, etc.

### Event Subscription

Plugins subscribe to events via API commands (not config). This allows:
- Plugin declares what it needs (self-describing)
- Dynamic subscribe/unsubscribe
- Config file only specifies which plugins to run

### Plugin-Provided Commands

Some commands are registered by plugins, not built into the engine:
- `bgp cache` commands (requires cache/RIB plugin)
- `rib show/clear` commands (requires RIB plugin)

Engine provides reactor methods; plugins register commands that use them.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/ipc_protocol.md` - canonical protocol spec
- [ ] `docs/architecture/api/architecture.md` - current implementation
- [ ] `docs/architecture/api/commands.md` - current command syntax

### Source Files
- [ ] `internal/plugin/handler.go` - dispatcher, handlers
- [ ] `internal/plugin/route.go` - route handlers
- [ ] `internal/plugin/forward.go` - forward handler
- [ ] `internal/plugin/session.go` - session handlers
- [ ] `internal/plugin/msgid.go` - msg-id handlers
- [ ] `internal/plugin/commit.go` - commit handlers
- [ ] `internal/plugin/rib/rib.go` - RIB plugin

## Command Structure

### Plugin Namespace

| Command | Purpose |
|---------|---------|
| `plugin help` | List plugin subcommands |
| `plugin command list\|help\|complete` | Plugin introspection |
| `plugin session ready` | Init complete |
| `plugin session ping` | Health check |
| `plugin session bye` | Disconnect |

### BGP Namespace

**Introspection:**
| Command | Purpose |
|---------|---------|
| `bgp help` | List bgp subcommands |
| `bgp command list` | Detailed bgp commands |
| `bgp command help "<cmd>"` | Specific command help |
| `bgp command complete "<partial>"` | Completion |
| `bgp event list` | List available BGP event types |

**Plugin Configuration:**
| Command | Purpose |
|---------|---------|
| `bgp plugin encoding json\|text` | Overall structure |
| `bgp plugin format hex\|base64\|parsed\|full` | Wire bytes (JSON only) |
| `bgp plugin ack sync\|async` | ACK timing |

**Peer Operations:**
| Command | Purpose |
|---------|---------|
| `bgp peer <sel> list` | List matching peers (brief) |
| `bgp peer <sel> show` | Show matching peers (detailed) |
| `bgp peer <sel> teardown [subcode]` | Graceful close (NOTIFICATION) |
| `bgp peer <sel> update text\|hex\|base64 ...` | Announce/withdraw routes |
| `bgp peer <sel> borr <family>` | RFC 7313 BoRR marker |
| `bgp peer <sel> eorr <family>` | RFC 7313 EoRR marker |
| `bgp peer <sel> ready` | Peer replay complete |
| `bgp peer <sel> tcp reset` | Force TCP RST |
| `bgp peer <sel> tcp ttl <num>` | Set TTL (multi-hop) |

**Watchdog:**
| Command | Purpose |
|---------|---------|
| `bgp watchdog announce <name>` | Announce pool routes |
| `bgp watchdog withdraw <name>` | Withdraw pool routes |

**Commits (Batching):**
| Command | Purpose |
|---------|---------|
| `bgp commit <name> start` | Begin batch |
| `bgp commit <name> end` | Flush batch |
| `bgp commit <name> eor` | Flush + send EOR |
| `bgp commit <name> rollback` | Discard batch |
| `bgp commit <name> show` | Show queued count |
| `bgp commit list` | List active batches |

**Daemon Control:**
| Command | Purpose |
|---------|---------|
| `bgp daemon shutdown` | Stop BGP subsystem |
| `bgp daemon restart` | Restart BGP |
| `bgp daemon reload` | Reload config |
| `bgp daemon status` | BGP status |

**Raw Passthrough:**
| Command | Purpose |
|---------|---------|
| `bgp raw <type> <enc> <data>` | Send raw BGP message |

**Event Subscription:**
| Command | Purpose |
|---------|---------|
| `subscribe [peer <sel> \| plugin <name>] bgp event <type> [direction received\|sent\|both]` | Subscribe to BGP events |
| `unsubscribe [peer <sel> \| plugin <name>] bgp event <type> [direction received\|sent\|both]` | Unsubscribe |

**BGP Event Types:**
| Event | Has Direction | Description |
|-------|---------------|-------------|
| `update` | ✅ | UPDATE message |
| `open` | ✅ | OPEN message |
| `notification` | ✅ | NOTIFICATION message |
| `keepalive` | ✅ | KEEPALIVE message |
| `refresh` | ✅ | ROUTE-REFRESH message |
| `state` | ❌ | Peer state change |
| `negotiated` | ❌ | Capability negotiation complete |

### System Namespace

| Command | Purpose |
|---------|---------|
| `system help` | List system subcommands |
| `system command list` | System commands only |
| `system command help "<cmd>"` | System command help |
| `system command complete "<partial>"` | System completion |
| `system subsystem list` | List available subsystems |
| `system shutdown` | Stop entire application |
| `system version software` | ZeBGP version |
| `system version api` | IPC protocol version |

### RIB Namespace

**Built-in:**
| Command | Purpose |
|---------|---------|
| `rib help` | List rib subcommands |
| `rib command list` | List rib commands |
| `rib command help "<cmd>"` | Specific command help |
| `rib command complete "<partial>"` | Completion |
| `rib event list` | List available RIB event types |

**Plugin-Provided (cache/RIB plugin):**
| Command | Purpose |
|---------|---------|
| `bgp cache <id> forward <sel>` | Forward cached UPDATE |
| `bgp cache <id> retain` | Prevent eviction |
| `bgp cache <id> release` | Allow eviction |
| `bgp cache <id> expire` | Delete immediately |
| `bgp cache list` | List cached msg-ids |
| `rib show in [peer]` | Show Adj-RIB-In |
| `rib clear in [peer]` | Clear Adj-RIB-In |

**Event Subscription:**
| Command | Purpose |
|---------|---------|
| `subscribe [peer <sel> \| plugin <name>] rib event <type>` | Subscribe to RIB events |
| `unsubscribe [peer <sel> \| plugin <name>] rib event <type>` | Unsubscribe |

**RIB Event Types:**
| Event | Has Direction | Description |
|-------|---------------|-------------|
| `cache` | ❌ | Cache entry (new, eviction) |
| `route` | ❌ | Route change (add/remove) |

## JSON Message Format

**Responses (from commands):**
```json
{"type":"response","response":{"serial":"1","status":"done","data":{...}}}
{"type":"response","response":{"serial":"2","status":"error","data":"invalid command"}}
```

**Events (subscribed):**
```json
{
  "type": "bgp",
  "bgp": {
    "type": "update",
    "message": {"id": 12345, "direction": "received"},
    "peer": {"address": "10.0.0.1", "asn": 65001},
    "attributes": {"origin": "igp", "as-path": [65001]},
    "nlri": {"ipv4/unicast": [{"action": "add", "next-hop": "10.0.0.1", "nlri": ["10.0.0.0/24"]}]},
    "raw": {"attributes": "4001...", "nlri": {"ipv4/unicast": "18..."}, "withdrawn": {}}
  }
}
```

**Message structure:**
- Top-level `type`: `"response"`, `"bgp"`, or `"rib"` (tells you which key to read)
- Payload is under the key matching `type`

**BGP event payload (`bgp` key):**
- `type`: event type (`update`, `open`, `notification`, `state`, etc.)
- `message`: wire metadata (id, direction) for BGP messages
- `peer`: `{"address": "...", "asn": N}`
- `attributes`: path attributes (UPDATE only)
- `nlri`: `{"<family>": [operations]}` (UPDATE only)
- `raw`: wire bytes when format=full

**RIB event payload (`rib` key):**
- `type`: event type (`cache`, `route`)
- `action`: `new`, `evict`, `add`, `remove`
- `msg-id`: cache ID (for cache events)
- `peer`: `{"address": "...", "asn": N}`

## Migration Summary

| Old | New |
|-----|-----|
| `daemon shutdown` | `bgp daemon shutdown` |
| `daemon status` | `bgp daemon status` |
| `daemon reload` | `bgp daemon reload` |
| `peer list` | `bgp peer <sel> list` |
| `peer show` | `bgp peer <sel> show` |
| `peer teardown <ip>` | `bgp peer <sel> teardown` |
| `peer <sel> update ...` | `bgp peer <sel> update ...` |
| `peer <sel> forward update-id <id>` | `bgp cache <id> forward <sel>` *(plugin)* |
| `peer <sel> borr` | `bgp peer <sel> borr` |
| `peer <sel> eorr` | `bgp peer <sel> eorr` |
| `peer <sel> session api ready` | `bgp peer <sel> ready` |
| `msg-id retain <id>` | `bgp cache <id> retain` *(plugin)* |
| `msg-id release <id>` | `bgp cache <id> release` *(plugin)* |
| `msg-id expire <id>` | `bgp cache <id> expire` *(plugin)* |
| `msg-id list` | `bgp cache list` *(plugin)* |
| `session sync enable` | `bgp plugin ack sync` |
| `session sync disable` | `bgp plugin ack async` |
| `session api encoding <fmt>` | `bgp plugin encoding <fmt>` |
| `session api ready` | `plugin session ready` |
| `session ping` | `plugin session ping` |
| `session bye` | `plugin session bye` |
| `session reset` | *removed* |
| `commit <name> ...` | `bgp commit <name> ...` |
| `watchdog announce` | `bgp watchdog announce` |
| `watchdog withdraw` | `bgp watchdog withdraw` |
| `raw <type> <enc> <data>` | `bgp raw <type> <enc> <data>` |
| `system version` | `system version software` |
| *config-driven events* | `subscribe [peer <sel> \| plugin <name>] bgp event <type> [direction ...]` |
| *new* | `system version api` |
| *new* | `system shutdown` |
| *new* | `system subsystem list` |
| *new* | `bgp peer <sel> tcp reset` |
| *new* | `bgp peer <sel> tcp ttl <num>` |
| *new* | `subscribe [peer <sel> \| plugin <name>] rib event <type>` |

## Removed

| Item | Reason |
|------|--------|
| `session reset` | Was resetting BGP settings; no longer needed |
| `session sync enable\|disable` | Moved to `bgp plugin ack` |
| `session api encoding` | Moved to `bgp plugin encoding` |
| CBOR encoding | Incompatible with line-delimited text protocol |
| Config-driven event routing | Replaced by `subscribe` commands |
| `transaction begin/commit/rollback` | Use `bgp commit <name>` instead |

## Data Flow Issues (MUST ADDRESS)

### Issue 1: Event Format Changes

**Current** (json.go):
```json
{
  "message": {"type": "update", "id": 123, "direction": "received"},
  "peer": {"address": "192.0.2.1", "asn": 65001},
  "origin": "igp",
  "as-path": [65001],
  "ipv4/unicast": [...]
}
```

**New** (top-level type as discriminator):
```json
{
  "type": "bgp",
  "bgp": {
    "type": "update",
    "message": {"id": 123, "direction": "received"},
    "peer": {"address": "192.0.2.1", "asn": 65001},
    "attributes": {"origin": "igp", "as-path": [65001]},
    "nlri": {"ipv4/unicast": [...]},
    "raw": {"attributes": "...", "nlri": {...}, "withdrawn": {...}}
  }
}
```

**Changes:**
- Add `type: "bgp"` at root (tells you which key to read)
- Nest all BGP data under `bgp` key
- Move `message.type` to `bgp.type` (event type)
- Keep `message` object for wire metadata (id, direction)
- Keep `peer` as object (address, asn)
- Move attributes into `attributes` object
- Move NLRI families into `nlri` object
- Add `raw` object for wire bytes (format=full)

**Impact:** RIB plugin `event.go` needs update for new nesting. Parsing: read `type`, then read `bgp` or `rib` key.

### Issue 2: Event Format Step Missing

Step 1 only adds `type: "response"` to responses. Event format changes need their own step.

**Decision:** Add to Step 1:
- Update json.go event encoders (nest under `bgp`/`rib` key, add event type, nest attributes/nlri)
- Update rib/event.go parser (handle new nesting)
- Both response AND event format changes

### Issue 3: "sent" Event message.id

Locally-originated routes (from `bgp peer * update`) don't have cached wire bytes.

**Decision:** "sent" events for locally-originated routes have `message.id: 0` (no cache entry). Only received UPDATEs have non-zero id for forwarding.

### Issue 4: Subscription Timing

Plugin subscribing after peers established doesn't get current state.

**Decision:** Document as "future events only". Plugin should:
1. Subscribe to events
2. Query `bgp peer * show` for current state
3. Handle both existing and new peers

## Design Decisions

### Q: What does `format parsed` do?

**A:** `parsed` format omits wire bytes entirely. Events contain only decoded fields.

| Format | Wire Bytes | Parsed Fields |
|--------|------------|---------------|
| `hex` | ✅ as hex string | ❌ |
| `base64` | ✅ as base64 | ❌ |
| `parsed` | ❌ | ✅ |
| `full` | ✅ as hex | ✅ |

Default: `parsed` (most efficient for plugins that don't need raw bytes).

### Q: Encoding/subscription timing?

**A:** Encoding applies at event delivery time, not subscription time.

```
# This is fine:
subscribe bgp event update direction received
bgp plugin encoding json
bgp plugin format full

# Events already subscribed will use the current encoding when delivered
```

Plugins should configure encoding before subscribing for clarity, but it's not required.

### Q: How do `bgp cache <id>` commands route?

**A:** Unlike `bgp peer <sel>`, the `<id>` in `bgp cache <id>` is NOT a selector extracted by the dispatcher.

- `bgp peer 10.0.0.1 show` → dispatcher extracts `10.0.0.1`, routes to `bgp peer show`
- `bgp cache 12345 forward *` → routes to `bgp cache` handler, which parses `12345 forward *`

The cache commands are registered by the RIB plugin with pattern `bgp cache`, and the plugin handler parses the subcommand and msg-id.

## Example Plugin Startup Flow

```
# Stage 1: Declaration
declare cmd rib show in
declare cmd rib clear in
declare cmd bgp cache forward
declare cmd bgp cache retain
declare cmd bgp cache release
declare cmd bgp cache expire
declare cmd bgp cache list
declare done

# Stage 2: Config delivery
config done

# Stage 3: Capabilities
capability done

# Stage 4: Registry
registry done

# Stage 5: Ready
ready

# After ready: configure and subscribe
bgp plugin encoding json
bgp plugin format full
bgp plugin ack async

subscribe bgp event update direction received
subscribe bgp event update direction sent
subscribe peer * bgp event state
subscribe rib event cache
subscribe rib event route
```

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDispatchPluginSessionReady` | `internal/plugin/handler_test.go` | `plugin session ready` | |
| `TestDispatchPluginSessionPing` | `internal/plugin/handler_test.go` | `plugin session ping` | |
| `TestDispatchBgpPeerList` | `internal/plugin/handler_test.go` | `bgp peer <sel> list` | |
| `TestDispatchBgpPeerShow` | `internal/plugin/handler_test.go` | `bgp peer <sel> show` (all) | |
| `TestDispatchBgpPeerShowSpecific` | `internal/plugin/handler_test.go` | `bgp peer <ip> show` (specific) | |
| `TestDispatchBgpPeerUpdate` | `internal/plugin/handler_test.go` | `bgp peer <sel> update text` | |
| `TestDispatchBgpPluginEncoding` | `internal/plugin/handler_test.go` | `bgp plugin encoding json` | |
| `TestDispatchBgpPluginAck` | `internal/plugin/handler_test.go` | `bgp plugin ack sync` | |
| `TestDispatchBgpCommit` | `internal/plugin/handler_test.go` | `bgp commit <name> start` | |
| `TestDispatchBgpDaemonShutdown` | `internal/plugin/handler_test.go` | `bgp daemon shutdown` | |
| `TestDispatchSubscribeBgpEvent` | `internal/plugin/handler_test.go` | `subscribe bgp event update` | |
| `TestDispatchSystemShutdown` | `internal/plugin/handler_test.go` | `system shutdown` | |
| `TestDispatchSystemVersionSoftware` | `internal/plugin/handler_test.go` | `system version software` | |
| `TestDispatchSystemVersionApi` | `internal/plugin/handler_test.go` | `system version api` | |
| `TestDispatchBgpEventList` | `internal/plugin/handler_test.go` | `bgp event list` | |
| `TestDispatchRibEventList` | `internal/plugin/handler_test.go` | `rib event list` | |
| `TestOldCommandsRejected` | `internal/plugin/handler_test.go` | Old syntax returns error | |

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `bgp-peer-update` | `test/data/plugin/bgp-peer-update.ci` | Route injection with new syntax | |
| `subscribe-events` | `test/data/plugin/subscribe-events.ci` | Event subscription | |

## Files to Modify

| File | Changes |
|------|---------|
| `internal/plugin/handler.go` | Restructure dispatch: add `plugin`, `bgp`, update `system` |
| `internal/plugin/session.go` | Move to `plugin session`, add `bgp plugin encoding/format/ack` |
| `internal/plugin/route.go` | Update paths under `bgp peer <sel> update` |
| `internal/plugin/forward.go` | Remove (becomes plugin-provided) |
| `internal/plugin/msgid.go` | Remove (becomes plugin-provided) |
| `internal/plugin/commit.go` | Update paths under `bgp commit` |
| `internal/plugin/types.go` | Remove `WireEncodingCBOR`, add subscription types |
| `internal/plugin/subscribe.go` | New file for subscription handlers |
| `internal/plugin/rib/rib.go` | Update command strings, register cache commands |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/plugin/subscribe.go` | Event subscription handlers |

## Implementation Steps

### Phase 1: Plugin Namespace
1. Create `plugin` dispatch node
2. Move `session ready/ping/bye` to `plugin session`
3. Add `plugin help` and `plugin command` introspection

### Phase 2: BGP Namespace Restructure
1. Create `bgp` dispatch node with all subcommands
2. Move `bgp plugin encoding/format/ack` handlers
3. Move peer, watchdog, commit, daemon handlers under `bgp`
4. Update `bgp peer <sel>` to support new subcommands (tcp reset, tcp ttl)

### Phase 3: Event Subscription
1. Add `subscribe/unsubscribe` handlers
2. Track subscriptions per-process
3. Route events only to subscribed processes
4. Remove config-driven event routing

### Phase 4: Plugin-Provided Commands
1. Remove built-in `msgid.go` and `forward.go` handlers
2. Update RIB plugin to register `bgp cache` commands
3. Update RIB plugin to register `rib show/clear` commands

### Phase 5: System Namespace
1. Update `system version` to `system version software`
2. Add `system version api`
3. Add `system shutdown`
4. Add `system subsystem list`
5. Make `system command` list only system commands

### Phase 6: JSON Format Update
1. Add `type` field to responses, wrap in `response` key
2. Add `type` field to events (`"type":"bgp"` or `"type":"rib"`), nest payload under matching key

### Phase 7: Cleanup
1. Remove CBOR encoding
2. Remove `session reset`
3. Remove config-driven event routing code
4. Update all plugins to use new command syntax

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
- [ ] `ipc_protocol.md` updated
- [ ] `commands.md` updated
- [ ] `architecture.md` updated

### Completion
- [ ] All files committed together
- [ ] Spec moved to `docs/plan/done/`
