---
kind: directive
level: MUST
stage:
---
- **A surface whose removal would break a different plugin, break the core, or leave a broken or empty command MUST be moved to its owner.** It is in the wrong package. The anti-patterns that fail this test are listed in `docs/architecture/command-ownership.md`, "What the Removal Test Forbids".
