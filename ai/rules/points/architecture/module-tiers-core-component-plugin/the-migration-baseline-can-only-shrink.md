---
kind: note
level:
stage:
---
`scripts/dev/tier_migration_baseline.txt` holds the engines that are currently in the wrong tier and scheduled to move (child specs `spec-tiers-2`/`-3`). Each row is annotated with the child spec that removes it. The gate fails on new violations and on stale entries, so the file can only shrink. **An empty baseline = full engine-placement enforcement with zero exceptions.** Regenerate after a move with `scripts/dev/dep_audit.py --write-baseline`.
