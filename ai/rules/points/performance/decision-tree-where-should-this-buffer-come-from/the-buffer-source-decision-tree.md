---
kind: fence
level:
stage:
---
```
Is this on a per-UPDATE / per-route / per-NLRI path?
├── YES: Does the caller already have a buffer in scope?
│   ├── YES: Add buf/off parameters, write into it (WriteTo pattern)
│   └── NO: Is there a pool for this goroutine shape?
│       ├── YES: Get from pool, use, Put back
│       └── NO: Can the buffer be a struct field reused across calls?
│           ├── YES: Store on the struct, reset between uses
│           └── NO: Create a sync.Pool for this use case
└── NO (startup, config, CLI, one-shot):
    ├── One-shot allocation → make() is fine
    ├── String building → textbuf.Buffer (stack, 128B inline)
    └── fmt.Sprintf → acceptable on cold paths
```
