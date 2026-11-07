# Appliance Host Name

## Meta

| Field | Value |
|-------|-------|
| Name | Appliance Host Name |
| Page | docs/guide/configuration.md |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | cmd/ze/hub/main_system_hostname.go, cmd/ze/hub/main_system_appliance.go, internal/component/config/system/doctor_hostname.go |
| Real-path tests | cmd/ze/hub/main_system_appliance_test.go::TestHostnameApplierOnlyWhenChanged, cmd/ze/hub/main_system_appliance_test.go::TestHostnameApplierSetFailureKeepsNameAndWarns |
| Docs | docs/guide/configuration.md, docs/guide/appliance.md |
| Doc review | 2026-11-07: the "System Host Name" section of docs/guide/configuration.md and the appliance guide's host name paragraph match reconcileKernelHostname (set only when the leaf is configured and differs from the kernel name, one warn line on a refusal, no refused commit), kernelHostnameOwned (true only under ze_appliance) and codeSystemHostNotApplied (doctor-system-host-not-applied) |
| Defect review | 2026-11-07: open: plan/release/fixes/spec-home-cpe-7-system-small-defects.md, the spec that delivers this feature, is not closed |
| Extra criteria | supported: booted appliance proof that uname -n follows system host at boot and on reload (./le test qemu appliance-hostname, internal/le/test/qemu/hostname.go) recorded green = none yet |

## Description

On the appliance build, Ze sets the kernel host name (what `uname -n`, syslog and the shell prompt show) from `system host` at start and on every reload that changes it, so each appliance carries its own name rather than the image's build-time `ze`. With `system host` unset, the kernel keeps the image's name. The value is validated as an RFC 1123 host name before it is applied. A `sethostname` refusal leaves the kernel name as it was, logs one warn line naming the value, the error and the fix, and `ze doctor` reports `doctor-system-host-not-applied`; the commit is not refused. A distro build (any build without `ze_appliance`) never calls `sethostname`: the operating system owns the name there, and `system host` changes only Ze's own identity. The booted proof is `./le test qemu appliance-hostname`. <!-- source: cmd/ze/hub/main_system_hostname.go -- reconcileKernelHostname --> <!-- source: cmd/ze/hub/main_system_appliance.go -- kernelHostnameOwned --> <!-- source: internal/component/config/system/doctor_hostname.go -- codeSystemHostNotApplied -->
