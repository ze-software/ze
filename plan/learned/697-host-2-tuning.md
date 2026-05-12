# 697 -- host-2-tuning

## Context

Ze could detect hardware state (CPU governor, NIC ring sizes, IRQ affinity) but had no write-side operations. Operators wanting performance tuning had to use external scripts or systemd units. The spec adds YANG-configured tuning applied at startup and on config commit, keeping ze as the single source of hardware configuration.

## Decisions

- **YANG enum for governor values** over free-form string. Rejects invalid governors at parse time (AC-5) rather than at apply time. The enum lists the five standard Linux governors.
- **Idempotent apply with read-before-write.** Each operation reads the current state first and only writes if the desired value differs. This prevents unnecessary sysfs churn and makes `ze config commit` with unchanged tuning a no-op.
- **Governor write verified via read-back.** After writing `scaling_governor`, the code reads it back and reports an error if the kernel rejected the value (some drivers silently ignore unsupported governors).
- **TuningResult as return value over report bus emission.** The caller decides whether to surface errors via the report bus. This keeps the tuning engine testable without mocking the bus and matches the DiffEvent pattern from host-1-observability.
- **IRQ affinity by interface name matching in /proc/irq/*/actions.** Iterates all IRQ directories and checks if the actions file mentions the interface name. This handles drivers that create multiple IRQ vectors per NIC (multi-queue).
- **Ethtool ring writes via existing ioctl infrastructure.** Extends ethtool_linux.go with ETHTOOL_SRINGPARAM (0x11) counterpart to the existing ETHTOOL_GRINGPARAM (0x10) read.

## Consequences

- Operators can declare hardware tuning in the config tree and have it applied automatically.
- Tuning errors are non-fatal: they are reported but do not block config commit.
- The YANG schema enforces valid governor names and ring size ranges at parse time.
- Non-Linux platforms accept the config but return ErrUnsupported at apply time.

## Gotchas

- `GetList` on the config tree returns `map[string]*Tree` where keys are the YANG list key values (interface names), not sequential indices.
- The `smp_affinity_list` format (`0,2,4-7`) is passed through to the kernel as-is. Validation of CPU list syntax is left to the kernel, which rejects invalid formats with EINVAL.
- ethtool ring size writes may fail on virtual NICs or drivers that do not support ring parameter changes. These failures are expected and reported, not retried.

## Files

- `internal/component/host/tuning.go` -- TuningConfig, TuningResult, ApplyTuning (cross-platform)
- `internal/component/host/tuning_linux.go` -- governor, IRQ affinity, ethtool ring writes
- `internal/component/host/tuning_other.go` -- ErrUnsupported stub
- `internal/component/host/tuning_test.go` -- platform-aware tests
- `internal/component/host/ethtool_linux.go` -- added openEthtoolSocket, closeEthtoolSocket, getEthtoolRingParam, setEthtoolRingParam, ETHTOOL_SRINGPARAM
- `internal/component/config/system/schema/ze-system-conf.yang` -- tuning container with cpu/governor, irq-affinity list, ethtool list
- `internal/component/config/system/system.go` -- TuningSystemConfig types, extractTuning, wired into ExtractSystemConfig
- `docs/guide/configuration.md` -- hardware tuning section added
