---
kind: directive
level: MUST NOT
stage:
---
- **Engine (in-process) subscribers deliver synchronously within `Emit`, and plugin-process subscribers deliver asynchronously, so a request and re-emit correlation MUST NOT assume every subscriber answered by the time `Emit` returns unless every subscriber is in-process.** `Emit` returns the plugin-process delivery count. The redistribute late-join replay holds its `ReplayID -> peer` map for a TTL rather than dropping it right after `Emit`, precisely because an out-of-process producer's re-emit arrives later.
<!-- source: internal/core/events/typed.go -- Emit returns plugin-process delivery count -->
<!-- source: internal/component/bgp/plugins/redistribute_egress/replay.go -- ReplayID token + TTL map -->
