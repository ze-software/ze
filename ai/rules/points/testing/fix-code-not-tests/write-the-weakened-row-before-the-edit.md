---
kind: directive
level: MUST
stage:
---
- **A legitimate weakening MUST have its row written in `test/weakened/<session>.md` BEFORE the edit, naming the test THIS edit weakens, and the commit MUST carry that shard.** The shard is the one your own commit session owns (`./le commit session`), no other session reads it, and the gate drops each row once the commit carrying it lands. The detector reads the shard from disk, so a row written after the refusal opens nothing until the edit is retried, and a row naming another test opens nothing at all. The row format is `docs/architecture/testing/test-health.md`.
