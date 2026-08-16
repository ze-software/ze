---
kind: table
level:
stage:
---
| Target | Enforces | Notes |
|--------|----------|-------|
| `make ze-test-sensitivity-check` | The two ratchets, read from the tree | Stage 10 of `ze-precommit-verify`, both modes. Independent of the report |
| `make ze-test-health-check` | STRUCTURAL facts only: orphaned test files, unproven RFCs, metric statuses | Inside `ze-generated-files-check`. Volume counters are published, not gated, so adding a test does not force a regeneration |
