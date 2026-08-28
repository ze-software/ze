---
kind: note
level:
stage:
---
`./le changed scope` and `./le verify current mode changed` both scope to one
native answer, and `./le changed scope` prints it. The answer is
the changed packages plus two levels of their importers, and the feature tags the
change can reach.
