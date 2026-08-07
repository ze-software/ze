---
kind: directive
level:
stage:
---
- **`internal/component/doctor` owns the runner, output contract, functional coverage through the user entry point, and checks that have no narrower owner.**
- **Do not add new runtime dependency checks by appending another direct call to the central `runChecks` list.**
- **Do not add owner-specific registrations in `internal/component/doctor` just because the runner lives there.**
- **Internal plugins (preferred path):** declare doctor checks in `registry.Registration.DoctorChecks`. The doctor runner bridges these at execution time via `checks_plugin_registry.go`. The check function uses `registry.DoctorCheckContext` (Tree and Platform as `any`) and returns `[]rpc.DoctorCheckDiagnostic`. Component is set automatically from the plugin name. See `l2tpauthradius/register.go` for the reference example.
- **Components that are not plugins** (e.g., appliance, web, SSH): use `diagnostic.RegisterDoctorCheck()` from the owning package's init().
