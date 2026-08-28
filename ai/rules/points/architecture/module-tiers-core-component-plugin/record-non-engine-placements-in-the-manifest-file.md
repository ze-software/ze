---
kind: note
level:
stage:
---
The source of truth for intentional non-engine placements outside `internal/core/` is `internal/le/tier/testdata/tier_non_engine_categories.txt`. It is non-code data consumed by `./le tier check`; do not hide new exceptions in Go code.
