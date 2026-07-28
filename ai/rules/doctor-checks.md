# Doctor Checks

**When:** when an implementation introduces a runtime dependency: a file, socket, port, kernel module, binary, or certificate
**Severity:** blocking

## Directives

Every feature that adds a new runtime dependency must register
a `ze doctor` check so agents can verify readiness before starting the daemon.

## The Rule

When your implementation introduces any of the following, add a registered
doctor check with explicit phase, order, component, dependency, platform,
diagnostic-code, and check-function metadata. Ownership is part of the
requirement: the package, component, or plugin that owns the dependency MUST
own the registration, check function, and unit test.

`internal/component/doctor` owns the runner, output contract, functional coverage through
the user entry point, and checks that have no narrower owner. Do not add new
runtime dependency checks by appending another direct call to the central
`runChecks` list. Do not add owner-specific registrations in `internal/component/doctor`
just because the runner lives there.

**Internal plugins (preferred path):** declare doctor checks in
`registry.Registration.DoctorChecks`. The doctor runner bridges these at
execution time via `checks_plugin_registry.go`. The check function uses
`registry.DoctorCheckContext` (Tree and Platform as `any`) and returns
`[]rpc.DoctorCheckDiagnostic`. Component is set automatically from the plugin
name. See `l2tpauthradius/register.go` for the reference example.

**Components that are not plugins** (e.g., appliance, web, SSH): use
`diagnostic.RegisterDoctorCheck()` from the owning package's init().

| New dependency | Doctor check needed |
|----------------|---------------------|
| Config leaf that references a file path (cert, key, binary) | File existence check |
| Config leaf that names an external service or socket | Reachability probe |
| Kernel module requirement | `/proc/modules` check (Linux) |
| New listen address/port | Port bind probe |
| New UDP listener | UDP `ListenPacket` bind probe |
| New service with TLS | Certificate validity + expiry check |
| Embedded certificate material | Parse certificate and check validity window |
| External binary (plugin, helper) | `exec.LookPath` or `os.Stat` check |
| Procfs/sysctl dependency | Read/write probe for the exact `/proc` path |
| Netlink dependency | Open the specific netlink family/handle |

## Diagnostic Code Convention

All doctor codes use the `doctor-` prefix: `doctor-<component>-<condition>`.

Register every new code in `internal/core/diagnostic/codes.go` with title,
description, and examples. The code must be explainable via `ze explain`.

## Mechanical Check

After implementation, verify the check is registered and explainable:

```
go test ./internal/component/doctor -run 'TestDoctorCoverageCodesRegistered|TestRunChecksExecutesRegisteredPluginCheck'
```

If you added a runtime dependency and no registered doctor check declares its
`doctor-*` code, you missed the readiness check or its diagnostic metadata.

## Where to Register Checks

| Dependency owner | Registration mechanism |
|------------------|-----------------------|
| Internal plugin (registered via `registry.Register`) | `Registration.DoctorChecks` field; bridge runs at doctor execution time |
| Web, MCP, looking-glass, or other listener component | `diagnostic.RegisterDoctorCheck()` from owning component |
| SSH host-key dependency | `diagnostic.RegisterDoctorCheck()` from SSH component |
| Interface backend | `diagnostic.RegisterDoctorCheck()` from backend owner |
| Kernel module, procfs, sysctl, netlink, VPP, or platform-specific backend | `diagnostic.RegisterDoctorCheck()` from owning backend/component, with build-tagged files where needed |
| Blob storage, platform detection, generic runner state, or dependency with no narrower owner | `internal/component/doctor`, with a comment or test name making the lack of owner explicit |

If no plugin, component, backend, or command package owns the dependency, keep
the check and unit test in `internal/component/doctor`. Do not invent an owner package just
to satisfy proximity.

## Test Requirement

Every new doctor check needs both:

| Test type | What it proves | Location |
|-----------|----------------|----------|
| Unit test | The check fires only when the relevant config block is present and emits the registered code | Owning package next to the registration, or `internal/component/doctor` only when there is no narrower owner |
| Functional test | `ze doctor --json <config>` exposes the behavior through the user entry point | `internal/component/doctor` or the existing functional test suite for the user entry |

Linux-only checks still need Linux-tagged tests and the package must be covered by the QEMU integration target when new `//go:build linux` code is added.
