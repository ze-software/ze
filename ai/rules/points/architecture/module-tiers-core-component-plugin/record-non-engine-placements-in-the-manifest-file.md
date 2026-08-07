---
kind: note
level:
stage:
---
The source of truth for intentional non-engine placements outside `internal/core/` is `scripts/dev/tier_non_engine_categories.txt`. It is a human-readable manifest consumed by `scripts/dev/dep_audit.py --check`; do not hide new exceptions in Python code.
