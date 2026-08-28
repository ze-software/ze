---
kind: fence
level:
stage:
---
```
[ ] Create internal/plugins/<name>/<name>.go (package-level logger with SetLogger)
[ ] Create internal/plugins/<name>/register.go (init() → registry.Register())
[ ] Run ./le repository generate (regenerates all.go)
[ ] Update TestAllPluginsRegistered expected count
[ ] Add YANG schema if config support (schema/ subdir)
[ ] Add EventTypes if plugin produces custom event types (e.g., ["update-rpki"])
[ ] Add DoctorChecks if plugin adds runtime dependencies (see ai/rules/repo-maintenance.md)
[ ] Add functional tests in test/plugin/
[ ] If plugin sets/reads route metadata: register keys in docs/architecture/meta/README.md, create docs/architecture/meta/<name>.md (see template there)
```
