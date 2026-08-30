---
kind: directive
level: MUST
stage:
---
**A legitimate weakening MUST have its row written in `test/weakened.md` BEFORE
the edit is made, and that row MUST name the test THIS edit weakens.** The
detector reads the file from disk, so a row written after the refusal buys
nothing until the edit is retried, and a row naming another test opens nothing.
The row's format is `docs/architecture/testing/test-health.md`.
