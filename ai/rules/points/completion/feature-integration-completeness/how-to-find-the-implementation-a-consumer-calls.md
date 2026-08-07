---
kind: table
level:
stage:
---
| Step | Action |
|------|--------|
| 1 | Grep for the protocol method/handler name across all `.go` files |
| 2 | List every implementation found |
| 3 | For each consumer of the feature: trace which implementation it actually calls |
| 4 | Modify (and test) the implementation the consumer uses, not just any implementation |
