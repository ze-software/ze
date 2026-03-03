# 222 — Config Reload 1: RPC Infrastructure

## Objective

Add `config-verify` and `config-apply` RPC types to the plugin protocol so the reload coordinator can drive config changes through plugins before applying them to the reactor.

## Decisions

- Config RPC handlers respond with OK (no-op) when no handler is registered — unlike other RPCs that return errors for unknown types, config changes are optional: not all plugins care about config updates.
- `handleConfigVerify` and `handleConfigApply` had structurally identical bodies; the `dupl` linter correctly flagged them. Extracted `handleConfigRPC(rpcType)` shared helper to eliminate the duplication.

## Patterns

- Graceful no-op (return OK) for optional RPCs is the right default when the absence of a handler is not an error condition — plugins that don't handle config are still valid participants in the reload protocol.
- Trust the linter: `dupl` flagging identical function bodies is a genuine signal, not noise. Always investigate before suppressing.

## Gotchas

- None.

## Files

- `internal/plugin/server/` — RPC server with config-verify and config-apply handlers
- `pkg/plugin/rpc/` — RPC type definitions
