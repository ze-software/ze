---
kind: directive
level:
stage:
---
**Two readings, and the one that governs.** "The shard is deleted at closure" and "a homed row stays live" (Status Vocabulary, below) contradict each other for a shard whose rows are homed at OTHER specs. Measured on 2026-08-03 by `scripts/dev/deferral_orphans.py`: 39 shards were in exactly that state, holding 68 live rows between them. Re-run the script rather than re-deriving the number; two hand-counts of it were wrong before the script existed. Deletion-at-closure governs the all-terminal case ONLY. Where a live row remains, the row wins and the shard stays.
