---
kind: directive
level:
stage:
---
**Plugin compile-out (routing protocols).** When the feature is already a self-registering plugin discovered by the generator (`register.go` -> `plugin/all`), there is NO new `register_<x>.go` or seam: gating is purely *blank-import partitioning*. List each owned dir as its own `feature-gates.txt` line under the shared tag, because a protocol spans several discovered dirs (engine + `transport` + `cli` + the `*-cmd` command schema), for example:
