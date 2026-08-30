---
kind: directive
level: MUST
stage:
---
- **The choice between a `sync.Pool` and a caller-passed buffer MUST be made from the table in `docs/architecture/buffer-architecture.md` ("Caller-owned buffers"), never from habit. A caller that already holds a buffer in scope MUST pass it down.**
