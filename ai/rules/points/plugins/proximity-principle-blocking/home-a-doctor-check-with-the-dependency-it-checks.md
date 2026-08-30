---
kind: directive
level: MUST
stage:
---
- **A runtime dependency check, its registration and its unit test MUST live in the plugin, component, backend or command package that owns the dependency.** `internal/component/doctor` keeps only the runner, the user-entry functional tests, and the checks for a dependency with no narrower owner.
