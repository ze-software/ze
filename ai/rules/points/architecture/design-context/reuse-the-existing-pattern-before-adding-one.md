---
kind: table
level:
stage:
---
| Anti-pattern | Instead |
|--------------|---------|
| "Docs say X supports Y" | Read the implementation. Docs may be stale or aspirational |
| "Industry standard is X" | Grep ze for how it already does X |
| "Good enough for dev" | "Do it right." Darwin could be prod |
| "Overkill for now" when comparing compile-time vs runtime enforcement | Compile-time enforcement is not overkill, it is correctness. If the compiler can prevent a class of bugs, that is the right option. Implementation cost is not a reason to accept weaker guarantees. |
| "Translation layer for cleaner API" | "Explicit > implicit." Use native names |
| "Put the registry where it's used" | Check `internal/core/` first |
| "DispatchCommand for cross-plugin calls" | EventBus for broadcast; DirectBridge for request/response |
| "New direct-call mechanism for internal plugins" | DirectBridge already exists (`pkg/plugin/rpc/bridge.go`). Read it before proposing. |
| "No cleanup needed on stop" | Ze owns what it touches |
| "VPP support can be added later" | If the feature has a netlink implementation, add the VPP implementation in the same work. Ze targets both dataplanes; netlink-only features create drift. |
| "Defaults are suggestions" | Defaults are requirements; log when overridden |
