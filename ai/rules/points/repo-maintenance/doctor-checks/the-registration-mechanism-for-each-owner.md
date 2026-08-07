---
kind: table
level:
stage:
---
| Dependency owner | Registration mechanism |
|------------------|-----------------------|
| Internal plugin (registered via `registry.Register`) | `Registration.DoctorChecks` field; bridge runs at doctor execution time |
| Web, MCP, looking-glass, or other listener component | `diagnostic.RegisterDoctorCheck()` from owning component |
| SSH host-key dependency | `diagnostic.RegisterDoctorCheck()` from SSH component |
| Interface backend | `diagnostic.RegisterDoctorCheck()` from backend owner |
| Kernel module, procfs, sysctl, netlink, VPP, or platform-specific backend | `diagnostic.RegisterDoctorCheck()` from owning backend/component, with build-tagged files where needed |
| Blob storage, platform detection, generic runner state, or dependency with no narrower owner | `internal/component/doctor`, with a comment or test name making the lack of owner explicit |
