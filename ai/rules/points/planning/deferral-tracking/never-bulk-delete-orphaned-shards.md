---
kind: directive
level: MUST NOT
stage:
---
**An orphaned shard is not a defect to sweep.** A shard whose `plan/spec-<stem>.md` is gone while live rows remain is the correct end state of the paragraph above, not leftover mess. MUST NOT bulk-delete orphaned shards to tidy the directory: read the rows first, and delete only a shard in which every row is terminal.
