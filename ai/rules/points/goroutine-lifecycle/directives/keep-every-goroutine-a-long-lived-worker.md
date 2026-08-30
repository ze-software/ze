---
kind: directive
level: MUST
stage:
rationale: ai/rationale/goroutine-lifecycle.md
---
**Every goroutine MUST be a long-lived worker. A per-event goroutine in a hot path is forbidden.** The permitted shapes and the channel-plus-worker skeleton are in `docs/contributing/ze-go-style.md`, "Goroutines".
