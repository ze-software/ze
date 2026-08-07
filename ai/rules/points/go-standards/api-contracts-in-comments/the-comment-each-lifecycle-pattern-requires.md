---
kind: table
level: MUST
stage:
---
| Pattern | Required Comment |
|---------|-----------------|
| Start/Stop/Wait lifecycle | Type doc: full sequence. Stop: "MUST call Wait after". Wait: "Must be called after Stop". |
| Close/cleanup required | "Caller MUST call Close when done" on the constructor |
| Init before use | "MUST call Init before first use" on the type or constructor |
| Call ordering | "MUST be called before/after X" on the dependent function |
| Concurrency safety | "Safe for concurrent use" or "NOT safe for concurrent use" |
| Paired operations (Lock/Unlock, Acquire/Release) | "Caller MUST call Y after X" on X |
