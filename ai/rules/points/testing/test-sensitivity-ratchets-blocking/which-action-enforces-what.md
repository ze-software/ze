---
kind: table
level:
stage:
---
| Action | Enforces | Notes |
|--------|----------|-------|
| `./le test-sensitivity check` | The two ratchets, read from the tree | Stage 10 of `./le verify current mode full`, both modes. Independent of the report |
| `./le test-health check` | STRUCTURAL facts only: orphaned test files, unproven RFCs, metric statuses | Inside `./le repository generated-check`. Volume counters are published, not gated, so adding a test does not force a regeneration |
