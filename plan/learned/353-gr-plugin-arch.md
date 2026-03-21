# 353 — GR Mechanism — Plugin-Based Architecture

## Objective

Move Graceful Restart (RFC 4724) from reactor-embedded logic to the bgp-gr plugin, with generic engine event enhancements enabling the architecture.

## Decisions

- Engine provides generic event enhancements (state-down reason, EOR detection, dependency-ordered delivery) — not GR-specific APIs
- bgp-gr uses inter-plugin commands (`rib retain-routes`, `rib release-routes`) to coordinate with bgp-rib — no direct coupling
- State events delivered sequentially in reverse dependency order so bgp-gr processes peer-down before bgp-rib
- EOR detected in `WireUpdate.IsEOR()` and delivered as distinct `EventEOR` event type

## Patterns

- **Inter-plugin coordination via commands:** plugins communicate through `DispatchCommand()` text commands, not shared memory or direct imports
- **Dependency-ordered delivery:** `sortByReverseDependencyTier()` uses `registry.TopologicalTiers()` — plugins with dependents process events first
- **Reason threading:** state-down reason flows from `OnPeerClosed(peer, reason)` through dispatcher to JSON events — values: `tcp-failure`, `notification`, `hold-timer`, `teardown`
- **EOR as empty UPDATE detection:** IPv4 unicast EOR = empty UPDATE; multiprotocol EOR = MP_UNREACH with AFI/SAFI only

## Gotchas

- Previous attempt embedded GR logic directly in reactor — violated plugin architecture boundary
- `retainedPeers` map in bgp-rib must be checked on peer-down AND cleared on peer-up (fresh session replaces retained state)
- Only TCP failure triggers route retention; notification-based shutdown means intentional close — no retention

## Files

- `internal/component/bgp/plugins/bgp-gr/gr.go`, `gr_state.go` — plugin GR state machine
- `internal/component/bgp/server/event_dispatcher.go`, `events.go` — reason param, EOR delivery, dependency ordering
- `internal/component/bgp/wireu/wire_update.go` — `IsEOR()` method
- `internal/component/bgp/plugins/bgp-rib/rib.go`, `rib_commands.go` — retain/release route commands
- `internal/component/plugin/events.go` — `EventEOR` constant
