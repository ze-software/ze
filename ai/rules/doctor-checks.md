# Doctor Checks

**BLOCKING.** Every feature that adds a new runtime dependency must register
a `ze doctor` check so agents can verify readiness before starting the daemon.

## The Rule

When your implementation introduces any of the following, add a corresponding
check in `cmd/ze/doctor/doctor.go` (or the platform-specific `checks_linux.go`):

| New dependency | Doctor check needed |
|----------------|---------------------|
| Config leaf that references a file path (cert, key, binary) | File existence check |
| Config leaf that names an external service or socket | Reachability probe |
| Kernel module requirement | `/proc/modules` check (Linux) |
| New listen address/port | Port bind probe |
| New service with TLS | Certificate validity + expiry check |
| External binary (plugin, helper) | `exec.LookPath` or `os.Stat` check |

## Diagnostic Code Convention

All doctor codes use the `doctor-` prefix: `doctor-<component>-<condition>`.

Register every new code in `internal/core/diagnostic/codes.go` with title,
description, and examples. The code must be explainable via `ze explain`.

## Mechanical Check

After implementation, run:

```
grep -c 'doctor-' internal/core/diagnostic/codes.go
```

If you added a runtime dependency and the count did not increase, you missed
a doctor check.

## Where to Add Checks

| Dependency in | Check goes in |
|---------------|---------------|
| `environment.web`, `environment.mcp`, `environment.looking-glass` | `checkListeners`, `checkTLS` |
| `environment.ssh` | `checkSSHHostKey` |
| `interface/backend` | `checkIfaceBackend` |
| `plugin/external` | `checkPlugins` |
| Kernel module (IPsec, VPP, conntrack) | `checkKernelModules` (Linux) |
| Blob storage | `checkStorage` |
