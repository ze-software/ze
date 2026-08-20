---
kind: note
level:
stage:
---
`ze-lint-changed`, `ze-unit-test-changed` and `ze-precommit-verify-changed` all
scope to ONE answer, and `make ze-verify-scope-selector` prints it. The answer is
the changed packages plus two levels of their importers, and the feature tags the
change can reach.
