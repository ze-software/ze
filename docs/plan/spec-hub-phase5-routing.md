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
   ```
   func (h *Hub) RouteCommand(cmd string) (string, error) {
       // Parse prefix, find handler, forward to plugin
   }

   func (h *Hub) Subscribe(pattern string, plugin *Process)
   func (h *Hub) Publish(event string, data any)
   ```

   → **Review:** Reuses existing dispatcher/server code?

4. **Create socket.go** - Unix socket for CLI
   ```
   func (h *Hub) StartSocket(path string) error
   func (h *Hub) handleCLIConnection(conn net.Conn)
   ```

5. **Modify cmd/ze** - CLI mode
   ```
   // ze bgp peer list → connect to socket, send, receive
   if !isConfigFile(args[0]) {
       return runCLI(args)
   }
   ```

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

### Event patterns

Subscriptions use glob patterns:
- `bgp.event.*` → all BGP events
- `bgp.event.peer.*` → peer events only
- `*` → all events

### CLI socket path

Default: `/var/run/ze/api.sock`
Configurable via: `env { api-socket /path/to/sock; }`

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
