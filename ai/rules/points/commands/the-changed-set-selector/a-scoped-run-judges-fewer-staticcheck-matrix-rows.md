---
kind: note
level:
stage:
---
**A scoped run judges fewer Staticcheck matrix rows too.** `scopeFeatureMatrix` (`internal/le/staticcheckmatrix/staticcheckmatrix.go`) keeps the two rows that omit no feature tag, plus one row per tag the change reached: 3 of 38 for a `ze_ssh`-local change. `all_features` and `core_only` judge the combinations Ze ships, and `validateScopedMatrix` refuses any scope that subtracts one of them.

**`./le staticcheck-feature-matrix check` typed on its own judges every row**, because only a verify run publishes the feature-tag answer that `ZE_VERIFY_SCOPE_TAGS` names. So does an answer that cannot be read, one naming a tag `feature-gates.txt` does not declare, and one naming every tag. An EMPTY answer is a real answer and judges the two shipped rows.

**Suite selection is not scoped: every functional suite runs on every verify, whatever the change set says.** `go list -deps ./cmd/ze` links 562 of the module's 646 packages, so no static signal attributes a `.ci` file to a Go package.
