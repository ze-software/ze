# Spec: hub-phase5-routing

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/hub-separation-phases.md` - phase overview
4. `internal/plugin/rib/rib.go` - existing RIB plugin

## Task

Implement event and command routing in the hub:
1. Hub routes commands by handler prefix to plugins
2. Hub routes events via pub/sub to subscribers
3. CLI connects via Unix socket
4. Verify existing RIB plugin works with hub

**Scope:** Inter-plugin communication, CLI socket, RIB integration.

**Depends on:** Phase 4 complete

## Required Reading

### Source Files
- [ ] `internal/plugin/rib/rib.go` - existing RIB plugin
- [ ] `internal/plugin/dispatcher.go` - existing command dispatch
- [ ] `internal/plugin/server.go` - existing event handling

**Key insights:**
- RIB plugin already follows 5-stage protocol
- RIB subscribes to `bgp.event.*`
- Command dispatcher already exists
- Event pub/sub already implemented in plugin.Server

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHubCommandRouting` | `internal/hub/router_test.go` | Command routed to correct plugin | |
| `TestHubEventSubscribe` | `internal/hub/router_test.go` | Plugin can subscribe to events | |
| `TestHubEventPublish` | `internal/hub/router_test.go` | Event delivered to subscribers | |
| `TestHubCLISocket` | `internal/hub/router_test.go` | CLI connects via Unix socket | |
| `TestHubCLICommand` | `internal/hub/router_test.go` | CLI command routed and response returned | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A - no new numeric inputs | | | | |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `hub-event-routing` | `test/data/hub/event-routing.ci` | BGP event reaches RIB | |
| `hub-command-routing` | `test/data/hub/command-routing.ci` | CLI command reaches BGP | |
| `hub-rib-integration` | `test/data/hub/rib-integration.ci` | RIB tracks BGP updates | |

## Files to Modify

- `internal/hub/hub.go` - Add routing methods, socket listener
- `cmd/ze/main.go` - CLI mode connects to socket

## Files to Create

- `internal/hub/router.go` - Command/event routing
- `internal/hub/socket.go` - Unix socket for CLI
- `internal/hub/router_test.go` - Unit tests
- `test/data/hub/*.ci` - Functional tests

## Implementation Steps

1. **Write unit tests** - Test command/event routing

   → **Review:** Test error cases (unknown handler, no subscribers)?

2. **Run tests** - Verify FAIL (paste output)

3. **Create router.go** - Command and event routing

   **Command routing behavior:**
   1. Parse command prefix
   2. Find handler using SchemaRegistry
   3. Forward to plugin that registered handler
   4. Return response to caller

   **Event pub/sub behavior:**
   | Method | Description |
   |--------|-------------|
   | Subscribe | Plugin registers interest in event pattern |
   | Publish | Hub delivers event to all matching subscribers |

   → **Review:** Reuses existing dispatcher/server code?

4. **Create socket.go** - Unix socket for CLI

   **Socket behavior:**
   1. Start listening on Unix socket path
   2. Accept CLI connections
   3. Read commands, route via hub, return responses

5. **Modify cmd/ze** - CLI mode

   **CLI detection:** If first arg is not a config file, run as CLI client connecting to hub socket.

6. **Verify RIB plugin** - Test with hub
   - RIB subscribes to `bgp.event.*`
   - RIB receives peer up/down events
   - RIB receives UPDATE events

7. **Run tests** - Verify PASS (paste output)

8. **Verify** - `make lint && make test && make functional`

## Design Decisions

### Command routing

Commands parsed by prefix:
- `bgp peer list` → prefix `bgp` → route to ze bgp
- `rib show` → prefix `rib` → route to ze rib
- `system process list` → prefix `system` → handle in hub

### Event subscription protocol

**During Stage 1 (static subscription):**
```
declare receive event bgp.event.*
declare receive event bgp.peer.*
```

**During runtime (dynamic subscription):**
```
#1 subscribe bgp.event.*
@1 done
```

**Event delivery format:**
```
event bgp.peer.up {"peer": "192.0.2.1", "state": "established"}
event bgp.peer.down {"peer": "192.0.2.1", "reason": "hold-timer-expired"}
```

### Event patterns

Subscriptions use glob patterns:
- `bgp.event.*` → all BGP events
- `bgp.event.peer.*` → peer events only
- `*` → all events

### CLI socket path

**Default path resolution (in order):**
1. `env { api-socket /path/to/sock; }` - explicit config
2. `$XDG_RUNTIME_DIR/ze/api.sock` - XDG-compliant user directory
3. `$HOME/.ze/api.sock` - fallback user directory
4. `/var/run/ze/api.sock` - system-wide (requires root)

**Why:** `/var/run/ze/` requires root to create. XDG-compliant paths allow non-root operation.

**Implementation:** Check paths in order, use first writable location.

## Implementation Summary

<!-- Fill after implementation -->

### What Was Implemented
- [List actual changes]

### Bugs Found/Fixed
- [Any bugs discovered]

### Deviations from Plan
- [Any differences and why]

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation (during implementation)
- [ ] Required docs read

### Completion (after tests pass)
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
