---
kind: note
level:
stage:
---
Doctor checks follow the same proximity rule. A runtime dependency check,
its registration, and its unit test belong in the plugin, component, backend,
or command package that owns the dependency. `internal/component/doctor` keeps the runner,
the user-entry functional tests, and checks for dependencies with no narrower
owner.
