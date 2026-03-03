# 252 — Remove Type Aliases

## Objective

Final cleanup: delete `ReactorInterface` composite type, widen `BGPHooks` callbacks from specific types to `any`, and delete 3 type aliases (`RawMessage`, `ContentConfig`, `RIBStatsInfo`) from `types.go`.

## Decisions

- `BGPHooks.OnMessageReceived`/`OnMessageSent` widened from `RawMessage` to `any`.
- `ReactorInterface` composite deleted; callers use `ReactorLifecycle` or `BGPReactor` directly.
- `MessageReceiver` interface in `reactor.go` also needed widening (not listed in spec but required for compilation).

## Patterns

None beyond cleanup.

## Gotchas

- Widening an interface method signature requires updating ALL implementations and call sites — grep for the interface name before widening.
- Secondary interfaces (`MessageReceiver`) that embed or mirror types must be widened simultaneously.

## Files

- `internal/plugin/types.go` — `ReactorInterface` deleted, 3 aliases deleted, `BGPHooks` callbacks widened to `any`
- `internal/plugin/server.go` — `OnMessageReceived`/`OnMessageSent` params widened to `any`
- `internal/plugins/bgp/server/hooks.go` — closures accept `any`, type-assert to `bgptypes.RawMessage`
- `internal/plugins/bgp/reactor/reactor.go` — `MessageReceiver` interface widened, all `plugin.RawMessage` → `bgptypes.RawMessage`
- `internal/plugin/mock_reactor_test.go` — 351 → 87 lines (all `BGPReactor` methods removed)
