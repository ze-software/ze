# 328 — Arch-6: Eliminate BGPHooks

## Objective

Remove all BGP-specific code from `internal/component/plugin/` by replacing the 7 any-typed `BGPHooks` closures with a type-safe `EventDispatcher` in `bgp/server/` and a generic `RPCFallback` for codec RPCs.

## Decisions

- Chose EventDispatcher over Bus-based delivery: Bus required solving per-subscriber format negotiation (different encoding per process), whereas EventDispatcher makes direct typed calls and sidesteps that complexity entirely.
- EventDispatcher lives in `bgp/server/` (not `internal/component/plugin/`) because `bgp/server` already imports `plugin` — no new import cycle introduced.
- `RPCFallback` typed as `func(string) func(json.RawMessage) (any, error)` — protocol-agnostic, any subsystem can provide codec RPCs without BGP types leaking into plugin infra.
- Exported `CodecRPCHandler` from `codec.go` so it can be passed as the `RPCFallback` value from the reactor side.

## Patterns

- When an import cycle prevents direct calls, move the bridge type to the package that already imports the generic infrastructure.
- Type-safe bridge pattern: EventDispatcher methods accept typed args (e.g., `bgptypes.RawMessage`), replacing `any`-typed closure injection.

## Gotchas

- Original spec required Bus-based delivery; this was marked BLOCKED during implementation due to format negotiation. EventDispatcher achieves the same architectural goal (zero BGP in plugin/) without requiring Bus on the hot path.
- Net change: +77 lines, -567 lines (490 net reduction) — sometimes the right refactor makes things much smaller.
- 4 validate-open tests in `plugin/` deleted: they tested the hooks delegation path which was removed; underlying logic already covered in `bgp/server/validate_test.go`.

## Files

- `internal/component/bgp/server/event_dispatcher.go` — new EventDispatcher (6 typed methods)
- `internal/component/plugin/types.go` — BGPHooks removed, RPCFallback field added to ServerConfig
- `internal/component/bgp/server/hooks.go` — deleted (NewBGPHooks constructor)
