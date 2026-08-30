---
kind: directive
level: MUST
stage:
---
- **`make([]byte, N)` MAY be used in a pool `New` func, session buffer creation, cached encoding, a result copy handed to a caller, JSON marshaling, tests, IPC framing, and config parsing. Anywhere else on the UPDATE path it MUST come from a pool.**
