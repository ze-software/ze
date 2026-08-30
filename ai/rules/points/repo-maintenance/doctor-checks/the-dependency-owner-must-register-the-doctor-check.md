---
kind: directive
level: MUST
stage:
---
- **A new runtime dependency MUST get a registered doctor check**, declaring its phase, order, component, dependency, platform, diagnostic code, and check function. The package, component, or plugin that OWNS the dependency MUST own the registration, the check function, and the unit test. `docs/architecture/doctor-and-health-checks.md` says which check each dependency kind needs, where each owner registers it, and which two tests it carries.
