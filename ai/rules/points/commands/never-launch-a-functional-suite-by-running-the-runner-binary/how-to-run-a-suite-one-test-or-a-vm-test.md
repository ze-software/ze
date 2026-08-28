---
kind: table
level:
stage:
---
| Want | Use |
|------|-----|
| A whole suite | `./le functional plugin` (or `encode`, `parse`, and the other actions listed by `./le functional list`) |
| One test, iterating | Use the owning compiled fixture's Go test, then rerun the complete `./le functional <suite>` action |
| A kernel-dependent suite in the VM | `./le qemu netns-test suites <comma-separated-suites>` |
