---
kind: note
level:
stage:
---
A native hook check declares the point it enforces with a `// ze point:` line in the Go function's doc comment. The function MUST be a top-level function named in `nativeHookActions`; a binding on an unwired function and a registered check with no binding both fail the gate map.
