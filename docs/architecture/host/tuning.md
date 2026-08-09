# Hardware tuning: the write side

Detection reads CPU governor, NIC ring sizes, and IRQ affinity. Tuning writes
them. The config declares the desired state, and Ze applies it at startup and on
each config commit, so hardware configuration has one owner.

<!-- source: internal/component/host/tuning.go -- TuningConfig, TuningResult, ApplyTuning -->
<!-- source: internal/component/host/tuning_linux.go -- governor, IRQ affinity, and ethtool ring writes -->
<!-- source: internal/component/host/tuning_other.go -- the ErrUnsupported stub -->
<!-- source: internal/component/host/ethtool_linux.go -- getEthtoolRingParam and setEthtoolRingParam -->

## Decisions

- **The governor is a YANG enum, not a free string.** An invalid governor is
  rejected at parse time instead of at apply time.
- **Apply is idempotent through read-before-write.** Each operation reads the
  current value and writes only on a difference, so a commit with unchanged
  tuning touches no sysfs file.
- **A governor write is verified by reading it back.** Some drivers accept the
  write and ignore the value.
- **`ApplyTuning` returns a result rather than publishing to the report bus.**
  The caller decides what to surface. This matches the diff engine in
  `observability.md`.
- **IRQ affinity is matched by interface name inside `/proc/irq/*/actions`.**
  Iterating the IRQ directories handles drivers that create one vector per
  queue, which a single-IRQ lookup would miss.
- Ring writes extend the existing ethtool ioctl path with the set counterpart of
  the get call.

## Consequences and traps

- Tuning errors are non-fatal. They are reported, and they do not block a config
  commit.
- Non-Linux platforms accept the config and return `ErrUnsupported` at apply
  time.
- The `smp_affinity_list` string is passed to the kernel as written. The kernel
  rejects a bad list with `EINVAL`, and that is the validation.
- A ring-size write can fail on a virtual NIC or on a driver with no ring
  parameter support. Those failures are expected, reported, and not retried.
- Reading the config tree for the interface list returns a map keyed by the YANG
  list key, which is the interface name, not a sequential index.

## Related

- `inventory.md` for the read side these writes verify against
