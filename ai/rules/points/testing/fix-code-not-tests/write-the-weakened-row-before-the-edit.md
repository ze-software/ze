---
kind: directive
level: MUST
stage:
---
- **A legitimate weakening MUST have its row written in `test/weakened.md` BEFORE the edit, naming the test THIS edit weakens, and the commit MUST carry the file.** The detector reads the file from disk, so a row written after the refusal opens nothing until the edit is retried, and a row naming another test opens nothing at all. The row format is `docs/architecture/testing/test-health.md`.
