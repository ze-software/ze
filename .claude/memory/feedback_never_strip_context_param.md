---
name: Never strip context.Context function parameters
description: When asked to clean "unused context", only remove the `import "context"` line (if truly unreferenced). Never remove `ctx context.Context` parameters from function signatures.
type: feedback
originSessionId: e075c32d-ca43-4f3d-9e9f-75be15a32524
---
"Clean unused context imports" (or similar phrasing) refers to dead `import "context"` statements, not function parameters.

**Why:** Removing `ctx context.Context` parameters is almost always wrong -- it breaks propagation of cancellation, deadlines, and request-scoped values, and changes the function's contract even when the current body doesn't use ctx. Parameters stay even if "unused locally."

**How to apply:**
- Verify the claim first (`go build ./...` and `go test ./... -run=xxx_no_match` compile-checks the whole tree; if both pass, there are no unused imports of any kind).
- Only touch `import (...)` lines, never the function signature.
- If a linter is complaining about a truly-unreferenced `context` import in a file, remove only the import line; leave every `ctx` parameter in every function of that file alone.
- When in doubt, ask before stripping anything around context.
