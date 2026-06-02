# Doctor Checks

**BLOCKING.** Every feature that adds a new runtime dependency must register
a `ze doctor` check so agents can verify readiness before starting the daemon.

## The Rule

When your implementation introduces any of the following, add a registered
doctor check in `cmd/ze/doctor/` with explicit phase, order, component,
dependency, platform, diagnostic-code, and check-function metadata. Do not add
new runtime dependency checks by appending another direct call to the central
`runChecks` list.

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
go test ./cmd/ze/doctor -run 'TestDoctorRegisteredCheckCodesHaveMetadata|TestRunChecksExecutesRegistered'
```

If you added a runtime dependency and no registered doctor check declares its
`doctor-*` code, you missed the readiness check or its diagnostic metadata.

## Where to Register Checks

| Dependency in | Check registration |
|---------------|--------------------|
| `environment.web`, `environment.mcp`, `environment.looking-glass` | listener/TLS check registration in `cmd/ze/doctor/` |
| `environment.ssh` | SSH host-key check registration |
| `interface/backend` | interface backend check registration |
| `plugin/external` | plugin binary check registration |
| Kernel module (IPsec, VPP, conntrack) | kernel-module registration in `checks_linux.go` with non-Linux stub where needed |
| Procfs, sysctl, netlink | platform-specific registration in `checks_linux.go` with a stub in `checks_other.go` |
| Blob storage | storage check registration |

## Test Requirement

Every new doctor check needs both:

| Test type | What it proves |
|-----------|----------------|
| Unit test | The check fires only when the relevant config block is present and emits the registered code |
| Functional test | `ze doctor --json <config>` exposes the behavior through the user entry point |

Linux-only checks still need Linux-tagged tests and the package must be covered by the QEMU integration target when new `//go:build linux` code is added.
