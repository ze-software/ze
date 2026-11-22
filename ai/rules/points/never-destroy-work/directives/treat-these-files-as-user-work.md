---
kind: directive
level: MUST
stage:
---
**You MUST treat each of these as work the user paid for: any file the user asked you to write, any file you wrote at the user's direction in this session, any in-progress implementation even when it does not compile or pass lint, and any scratchpad the user might be reading (`to-check`, `Stub/`, notes).**
**You MAY act without asking on a build artifact that `make`, `go mod vendor` or codegen regenerates, on a cache under `~/.cache/` or similar, on a file the user explicitly marked disposable, on an unused leftover temporary artifact you created yourself (your own `tmp/session/<id>/scratch/` files, spent `tmp/commit-*.sh` and `tmp/commit-msg-*.txt`, and spent or superseded `tmp/commit-*-approved-*.md`), and on removing a completed `plan/handover/` file through `./le commit create ... remove <path>` once all its items have landed. Care is still owed.**
