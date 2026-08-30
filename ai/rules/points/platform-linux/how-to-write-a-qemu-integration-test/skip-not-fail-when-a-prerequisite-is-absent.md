---
kind: directive
level: MUST
stage:
---
**A test whose prerequisite is absent MUST call `t.Skip`, never `t.Fatal`.** One test file runs in environments with different capabilities, and a fatal there reports a broken product for a missing capability. The worked example is `docs/architecture/testing/qemu-integration.md`.
