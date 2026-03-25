# 419 -- Arch-7: Subsystem Wiring

## Context

The Engine, Bus, and Subsystem interface were built in arch-1 through arch-6 but never wired into the startup path. The reactor was created and started directly by `cmd/ze/hub/main.go`. The arch-5 learned summary explicitly said "Hub.Orchestrator replacement deferred — wiring happens in a follow-up." This spec completed that follow-up.

## Decisions

- BGPSubsystem adapter wraps reactor, implements `ze.Subsystem` — chosen over modifying reactor directly because reactor's Start/Stop signatures don't match the Subsystem interface (no Bus/ConfigProvider params).
- Bus is a **notification layer**, not data transport — chosen over routing UPDATEs through Bus because `OnMessageReceived` returns cache consumer counts (synchronous), and Bus is fire-and-forget. EventDispatcher data path stays unchanged.
- Bus notifications fire in parallel with existing EventDispatcher calls — not replacing them. Dual-path: Bus for cross-component signaling, EventDispatcher for plugin data delivery.
- `cmd/ze/hub/main.go` creates Engine with Bus + ConfigProvider + PluginManager, registers BGPSubsystem, calls `Engine.Start()` — replaces direct `reactor.Start()`.

## Consequences

- Unblocks `spec-iface-bus` — interface plugin can subscribe to `bgp/state` Bus events.
- All future subsystems (BMP, RPKI) follow the same pattern: implement `ze.Subsystem`, register with Engine.
- Bus topics (`bgp/update`, `bgp/state`, `bgp/negotiated`, `bgp/eor`, `bgp/congestion`) are created at startup — cross-component consumers can subscribe.
- Engine supervises shutdown order (subsystems reverse, then plugins) — cleaner than manual orchestration.

## Gotchas

- `check-existing-patterns.sh` hook blocks `func New()` — had to use `NewBGPSubsystem()`.
- EventDispatcher's cache consumer count return value prevents routing UPDATEs through Bus — discovered during critical review, led to the "notification not transport" design.
- `cmd/ze/hub/main.go` `go func()` for reactor.Wait triggers goroutine-lifecycle hook — needs `// lifecycle:` comment.

## Files

- `internal/component/bgp/subsystem/subsystem.go` — BGPSubsystem adapter (new)
- `internal/component/bgp/subsystem/subsystem_test.go` — 8 tests (new)
- `internal/component/bgp/reactor/reactor.go` — Bus field, SetBus, publishBusNotification
- `internal/component/bgp/reactor/reactor_notify.go` — Bus notifications in 5 event functions
- `cmd/ze/hub/main.go` — Engine-supervised startup
- `docs/architecture/subsystem-wiring.md` — architecture doc with Mermaid diagrams (new)
