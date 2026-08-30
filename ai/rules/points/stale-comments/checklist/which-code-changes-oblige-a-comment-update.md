---
kind: directive
level: MUST
stage:
---
**Each change below MUST carry its comment update:**

| Change | Action |
|--------|--------|
| Function signature changes (return type, params) | Update all doc comments on the function |
| Control flow changes (new branch, removed path) | Update inline comments describing the flow |
| Error handling changes | Update comments explaining error propagation |
| Callers change behavior | Update comments at the call site |
