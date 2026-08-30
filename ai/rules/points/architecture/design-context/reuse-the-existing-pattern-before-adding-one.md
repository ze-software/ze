---
kind: directive
level: MUST NOT
stage:
---
**The reasoning in the left column MUST NOT be acted on. The right column is what the design MUST do instead.**

| Anti-pattern | Instead |
|--------------|---------|
| "Docs say X supports Y" | Read the implementation. A page CAN be stale or aspirational |
| "Industry standard is X" | Grep ze for how it already does X |
| "Good enough for dev" | Do it right. Darwin CAN be production |
| "Overkill for now", comparing compile-time against runtime enforcement | Compile-time enforcement is correctness, not overkill. When the compiler CAN prevent a class of bug, that is the right option, and implementation cost is not a reason to accept a weaker guarantee |
| "A translation layer for a cleaner API" | Explicit beats implicit. Use the native names |
| "Put the registry where it is used" | Check `internal/core/` first |
| "DispatchCommand for cross-plugin calls" | EventBus for broadcast; DirectBridge for request and response |
| "A new direct-call mechanism for internal plugins" | DirectBridge already exists (`pkg/plugin/rpc/bridge.go`). Read it before proposing one |
| "No cleanup needed on stop" | Ze owns what it touches |
| "VPP support CAN be added later" | A feature with a netlink implementation MUST get its VPP implementation in the same work. Ze targets both dataplanes, and a netlink-only feature creates drift |
| "Defaults are suggestions" | Defaults are requirements, and an override MUST be logged |
