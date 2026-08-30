---
kind: directive
level: MUST
stage:
---
**You MUST treat each of these as work the user paid for: any file the user asked you to write, any file you wrote at the user's direction in this session, any in-progress implementation even when it does not compile or pass lint, and any scratchpad the user might be reading (`to-check`, `Stub/`, notes).**
**You MAY act without asking on a build artifact that `make`, `go mod vendor` or codegen regenerates, on a cache under `~/.cache/` or similar, and on a file the user EXPLICITLY marked disposable. Care is still owed.**
