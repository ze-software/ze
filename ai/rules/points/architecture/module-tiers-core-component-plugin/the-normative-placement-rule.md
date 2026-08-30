---
kind: directive
level: MUST
stage:
---
- **A config-driven engine (one calling `sdk.NewWithConn`) at a top-level subsystem MUST live in `internal/component/` when a feature depends on it, and in `internal/plugins/` otherwise.**
- **A non-engine package outside `internal/core/` MUST either be classified by the existing registration mechanics, or carry a manifest row in `internal/le/tier/testdata/tier_non_engine_categories.txt`.**
