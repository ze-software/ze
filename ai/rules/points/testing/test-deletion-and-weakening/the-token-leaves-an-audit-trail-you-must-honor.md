---
kind: note
level:
stage:
---
The token unblocks the edit and leaves an audit trail. Review all relaxations with:
`git grep -n 'test-relax:' -- '*_test.go' '*.ci' '*.et'`. It must be `git grep`: plain
`grep` reads those globs as filenames after `--` and reports nothing, which looks
exactly like no relaxations. Using the token without a real reason is a violation.
