# 162 — Hub Phase 5: Event and Command Routing

## Objective

Implement inter-plugin communication in the hub: command routing by handler prefix, event pub/sub for plugin-to-plugin events, and Unix socket for CLI connections.

## Decisions

- Command routing by prefix: `bgp peer list` → prefix `bgp` → route to ze-bgp child. Same SchemaRegistry prefix matching used for config routing.
- Static event subscription via Stage 1 (`declare receive event bgp.event.*`) and dynamic subscription via runtime command (`#1 subscribe bgp.event.*`).
- Event delivery format: `event <topic> <json-payload>` — simple text line, topic before payload.
- CLI socket path resolution order: explicit config → `$XDG_RUNTIME_DIR/ze/api.sock` → `$HOME/.ze/api.sock` → `/var/run/ze/api.sock`. XDG-compliant paths allow non-root operation.
- Implementation Summary left blank — actual implementation tracked in 157-hub-separation-phases.

## Patterns

- Event patterns use glob syntax (`bgp.event.*`, `bgp.peer.*`, `*`) — prefix-based with wildcard at end.

## Gotchas

- `/var/run/ze/` requires root to create — must check paths in order and use first writable location. Hardcoding `/var/run/` breaks non-root operation.

## Files

- `internal/hub/router.go` — command and event routing
- `internal/hub/socket.go` — Unix socket for CLI
