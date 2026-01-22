# API Command Restructure - Overview

## Step Dependencies

```
Step 1: JSON Message Format ✅
    │
    ├── Step 2: Plugin Namespace ✅
    │
    ├── Step 3: System Namespace (ready)
    │
    └── Step 4: BGP Namespace Foundation (ready)
            │
            └── Step 5: BGP Command Migration
                    │
                    └── Step 6: Event Subscription
                            │
                            └── Step 7: RIB Namespace & Plugin Commands
```

**Parallel execution possible:**
- Steps 2, 3, 4 can be done in parallel after Step 1
- Steps 5, 6, 7 must be sequential

## Step Summary

| Step | Spec File | Description | Depends On | Status |
|------|-----------|-------------|------------|--------|
| 1 | `done/140-api-command-restructure-step-1.md` | JSON Message Format: Add `type` field | None | ✅ Done |
| 2 | `done/141-api-command-restructure-step-2.md` | Plugin Namespace: `plugin session *` commands | Step 1 | ✅ Done |
| 3 | `spec-api-command-restructure-step-3.md` | System Namespace: `system version api`, `system shutdown` | Step 1 | Pending |
| 4 | `spec-api-command-restructure-step-4.md` | BGP Foundation: introspection, `bgp plugin *` config | Step 1 | Pending |
| 5 | `spec-api-command-restructure-step-5.md` | BGP Migration: move all commands under `bgp` | Step 4 | Pending |
| 6 | `spec-api-command-restructure-step-6.md` | Event Subscription: `subscribe`/`unsubscribe` | Step 5 | Pending |
| 7 | `spec-api-command-restructure-step-7.md` | RIB Namespace: plugin-provided commands | Step 6 | Pending |

## Files Affected by Step

| Step | Create | Modify | Delete |
|------|--------|--------|--------|
| 1 | - | types.go, handler.go, session.go, commit.go, route.go, forward.go, msgid.go, raw.go, refresh.go, encoder.go | - |
| 2 | plugin.go | handler.go, session.go | - |
| 3 | - | handler.go | - |
| 4 | bgp.go | handler.go, session.go, types.go | - |
| 5 | - | handler.go, route.go, commit.go, raw.go, refresh.go, command.go | - |
| 6 | subscribe.go | handler.go, types.go, process.go | - |
| 7 | rib.go | handler.go, rib/rib.go | msgid.go, forward.go |

## Command Migration Summary

### Before → After

**Daemon Control:**
```
daemon shutdown      → bgp daemon shutdown
daemon status        → bgp daemon status
daemon reload        → bgp daemon reload
                     → bgp daemon restart (new)
```

**Peer Operations:**
```
peer list            → bgp list
peer show            → bgp show
peer show <ip>       → bgp peer <sel> show
peer teardown <ip>   → bgp peer <sel> teardown
peer <sel> update    → bgp peer <sel> update
peer <sel> borr      → bgp peer <sel> borr
peer <sel> eorr      → bgp peer <sel> eorr
                     → bgp peer <sel> tcp reset (new)
                     → bgp peer <sel> tcp ttl (new)
                     → bgp peer <sel> ready (new)
```

**Session Control:**
```
session api ready    → plugin session ready
session ping         → plugin session ping
session bye          → plugin session bye
session reset        → REMOVED
session sync enable  → bgp plugin ack sync
session sync disable → bgp plugin ack async
session api encoding → bgp plugin encoding
                     → bgp plugin format (new)
```

**Batching:**
```
commit <name> ...    → bgp commit <name> ...
watchdog announce    → bgp watchdog announce
watchdog withdraw    → bgp watchdog withdraw
raw ...              → bgp raw ...
```

**Msg-ID/Cache (plugin-provided):**
```
msg-id retain        → bgp cache <id> retain (plugin)
msg-id release       → bgp cache <id> release (plugin)
msg-id expire        → bgp cache <id> expire (plugin)
msg-id list          → bgp cache list (plugin)
forward update-id    → bgp cache <id> forward (plugin)
delete update-id     → REMOVED (use expire)
```

**RIB (plugin-provided):**
```
rib show in          → rib show in (plugin)
rib clear in         → rib clear in (plugin)
```

**System:**
```
system version       → system version software
                     → system version api (new)
                     → system shutdown (new)
                     → system subsystem list (new)
```

**Event Subscription (new):**
```
(config-driven)      → subscribe [peer <sel> | plugin <name>] <ns> event <type> [direction ...]
                     → unsubscribe [peer <sel> | plugin <name>] <ns> event <type> [direction ...]
```

## Removed Features

| Feature | Reason |
|---------|--------|
| `session reset` | Only reset sync/encoding to defaults; not needed |
| `WireEncodingCBOR` | Incompatible with line-delimited protocol |
| `neighbor` prefix | Use `bgp peer` instead |
| Config-driven events | Replaced by `subscribe` commands |
| `delete update-id` | Use `bgp cache <id> expire` |

## Test Updates Required

After completing all steps, update these test files:

### Plugin Test Files (`test/plugin/*.ci`)

| Test File | Old Command | New Command |
|-----------|-------------|-------------|
| `announce.ci` | `update text ...` | `bgp peer * update text ...` |
| `announce.ci` | `commit batch1 eor` | `bgp commit batch1 eor` |
| `teardown-cmd.ci` | `neighbor 127.0.0.1 teardown 4` | `bgp peer 127.0.0.1 teardown 4` |
| `watchdog.ci` | `watchdog announce dnsr` | `bgp watchdog announce dnsr` |
| `watchdog.ci` | `watchdog withdraw dnsr` | `bgp watchdog withdraw dnsr` |
| `refresh.ci` | `peer * borr/eorr` | `bgp peer * borr/eorr` |
| `reconnect*.ci` | `update text ...` | `bgp peer * update text ...` |
| `rib-*.ci` | `update text ...` | `bgp peer * update text ...` |
| `ipv4.ci`, `ipv6.ci` | `update text ...` | `bgp peer * update text ...` |
| `eor.ci` | `commit batch1 eor` | `bgp commit batch1 eor` |

### Python API Library Updates

The `ze_bgp_api.py` helper used by tests may need updates:
- `ready()` → sends `plugin session ready` (Step 2)
- `send()` → may need to prepend `bgp peer *` for updates

### Unit Test Files (`internal/plugin/*_test.go`)

| File | Changes |
|------|---------|
| `handler_test.go` | Add tests for new dispatch paths |
| `session_test.go` | Update to `plugin session *` commands |
| `route_test.go` | Update to `bgp peer * update` commands |
| `commit_test.go` | Update to `bgp commit` commands |

## Verification After Each Step

```bash
make lint && make test && make functional
```

All three must pass before proceeding to next step.

## Rollback Strategy

Each step should be committed separately. If a step fails:
1. `git stash` or `git diff > backup.patch`
2. `git checkout .`
3. Investigate and fix
4. Re-apply changes

## Final Verification

After all steps complete:
1. All tests pass
2. Run a full integration test with a real BGP peer
3. Verify RIB plugin works with new commands
4. Update any external documentation
