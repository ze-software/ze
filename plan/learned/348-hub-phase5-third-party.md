# 348 — Hub Phase 5: Third-Party Plugin Support

## Objective

Create a Go plugin SDK, developer documentation, and a working example plugin to enable external developers to write plugins that extend ZeBGP's configuration schema.

## Decisions

- Chose longest-prefix match for verify/apply handler routing in the SDK — consistent with Hub's routing, so plugin authors see the same semantics.
- Predicate stripping in SDK handler matching: `test[key=val]` matches handler `test` — same logic as Hub.
- Python SDK documented but not implemented — sufficient to show the pattern; Go SDK provides the reference implementation.
- `ze plugin validate` CLI and scaffold generator (`ze plugin init`) documented but not implemented — deferred to reduce scope.

## Patterns

- Plugin SDK follows the same 5-stage protocol as external plugins — no special SDK-only protocol. SDK is a thin layer that handles protocol framing, leaving business logic to handler functions.

## Gotchas

None.

## Files

- `pkg/plugin/plugin.go` — Plugin SDK with `New()`, `SetSchema()`, `OnVerify()`, `OnApply()`, `OnCommand()`, `Run()`
- `docs/plugin-development/` — six documentation files (README, protocol, schema, handlers, commands, testing)
- `examples/plugin/go/main.go` — working example plugin
