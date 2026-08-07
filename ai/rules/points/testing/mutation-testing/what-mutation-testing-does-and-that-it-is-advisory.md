---
kind: note
level:
stage:
---
Mutation testing uses [gomu](https://github.com/sivchari/gomu) to verify that
tests actually catch code changes. It modifies the AST (arithmetic, conditional,
logical, bitwise, branch, return value, error handling operators) and checks
whether the test suite detects each mutation. Advisory only, never gates
`ze-verify`.
