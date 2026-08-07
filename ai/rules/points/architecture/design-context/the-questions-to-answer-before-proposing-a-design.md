---
kind: directive
level:
stage:
---
- 1. Did I read how ze already handles similar? (grep, not assume)
- 2. Did I check `internal/core/` for an existing shared pattern?
- 3. Did I read the relevant `ai/patterns/` file?
- 4. Does my proposal contradict "Design Principles" below?
- 5. Am I inventing a name when standard/kernel/existing exists?
- 6. Am I proposing a new communication mechanism? Read `pkg/plugin/rpc/bridge.go` first. DirectBridge likely already does it.
- 7. Am I comparing systems or claiming capabilities? Read the implementation for each system being compared. Spawn parallel agents if multiple codepaths need verification. Never answer from docs alone.
