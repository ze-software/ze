---
kind: directive
level: MUST
stage:
rationale: ai/rationale/goroutine-lifecycle.md
---
**Every goroutine MUST be a long-lived worker reading a channel: a goroutine started inside an event loop or per message is forbidden.** `go func()` MAY start a one-time component lifecycle step, a test helper, a process-wait bridge, or a dedicated timer that selects on cancellation. The permitted shapes and the channel-plus-worker skeleton are in `docs/contributing/ze-go-style.md`, "Goroutines".
