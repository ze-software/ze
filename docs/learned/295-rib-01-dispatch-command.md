# 295 — RIB-01: Inter-Plugin Command Dispatch

## Objective

Add `ze-plugin-engine:dispatch-command` RPC so plugins can invoke the engine's command dispatcher and receive structured `{status, data}` responses, enabling plugin-to-plugin communication through the engine.

## Decisions

- Follows exact same pattern as `update-route` RPC — same dispatcher call, different response type; the only gap was response type preservation
- `responseToDispatchOutput()` helper JSON-encodes structured Data, passes strings through — handles both string and structured responses from the dispatcher
- Functional test deferred to rib-02: Python `ze_api.py` lacked dispatch-command and execute-command callback support at the time; first real consumer (bgp-rr → bgp-adj-rib-in) would provide end-to-end validation instead
- DirectBridge path registered alongside socket path in both `dispatchPluginRPC` and `dispatchPluginRPCDirect` switch cases

## Patterns

- Engine dispatcher is the universal command router — this spec just exposes it to plugins via a new RPC
- All routing goes through existing `Dispatcher.Dispatch()`, no new dispatch logic needed
- SDK type alias `DispatchCommandOutput` exported so callers don't need to import rpc package

## Gotchas

- None.

## Files

- `internal/yang/modules/ze-plugin-engine.yang` — `dispatch-command` RPC added
- `internal/component/plugin/server_dispatch.go` — `handleDispatchCommandRPC` (socket) + `handleDispatchCommandDirect` (bridge) + switch cases
- `pkg/plugin/sdk/sdk.go` — `DispatchCommand()` method + type alias
- `pkg/plugin/rpc/types.go` — `DispatchCommandInput`/`DispatchCommandOutput` types
