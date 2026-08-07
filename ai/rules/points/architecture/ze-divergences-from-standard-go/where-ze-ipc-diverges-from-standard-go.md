---
kind: table
level:
stage:
---
| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| gRPC or HTTP between services | JSON events down, text commands up, over pipes or net.Pipe | `ai/rules/plugins.md` | Plugin SDK is language-agnostic (Go/Python/Rust) |
| Direct function calls for sync | DirectBridge for typed in-process calls | `ai/rules/plugins.md` (DirectBridge) | Bypasses JSON serialization for internal plugins |
| Channel-based pub/sub | EventBus with typed handles (`events.Register[T]`) | `ai/rules/plugins.md` (EventBus) | Type-safe, registered event types, no raw `bus.Subscribe` |
