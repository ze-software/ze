---
kind: note
level:
stage:
---
A green unit test on an uncalled guard is worse than no test: it stops the
question being asked. `TestCheckCardinality`
(`internal/component/config/yang/validator_test.go`) passes, including its
count-0 row, while `walkTree` (`internal/component/config/yang/validator.go`)
iterated only present keys and its leaf-value branch skipped non-strings, so
leaf-list `min-elements` was only ever handed exactly 1 and could never reject.
