# Host hardware inventory

`internal/component/host` reads the hardware a Ze box runs on and returns one
structured inventory: CPU topology, NICs and their drivers, DMI board identity,
memory, storage, thermal sensors, and kernel posture. It reads sysfs and procfs
and never writes to them.

<!-- source: internal/component/host/inventory.go -- Inventory, Detector, Detect -->
<!-- source: internal/component/host/doc.go -- package scope and consumers -->
<!-- source: internal/plugins/host/host.go -- offline `ze host show` -->
<!-- source: internal/plugins/host-cmd/cmd/show_host.go -- online show handlers -->

## Decisions

- **A component, not a plugin.** Hardware detection is infrastructure that
  plugins and handlers consume, not domain policy.
- **Linux only, with a stub elsewhere.** Every source is sysfs or procfs. A
  cross-platform data shim would be maintenance with no reader. The stubs return
  `ErrUnsupported`, and `Detect` treats that as "section not available" rather
  than an error, so the rest of Ze builds and runs on macOS.
- **Standard library only.** `prometheus/procfs` is vendored indirectly and is
  tempting. A dozen small file reads are about 50 lines each and clearer than a
  third-party parser.
- **A test-injectable filesystem root** on the `Detector` struct, not a
  package-level global. Tests point a detector at a fixture tree and drive every
  section reader against it. Production uses the zero value, which is safe for
  concurrent use.
- **Typed enums, not strings**, for CPU vendor, core role, scaling driver, and
  NIC transport. JSON emits the string form; Go compares constants.
- **A virtual NIC is detected structurally**, by the absence of
  `/sys/class/net/<name>/device/`, not by a driver-name allowlist. New virtual
  drivers are filtered without the list rotting.
- **JSON by default on the offline command, `--text` opt in.** The first reader
  is a pipe into `jq` or a metrics shim.
- **`show system cpu` and `show system memory` are enriched, not replaced.** The
  Go-runtime fields stay and a nested `hardware` object is added.

## Units, and the mistakes they cause

- `/proc/meminfo` values are in kB. The library converts to bytes at parse time,
  so every public field name can end in `-bytes`. Skipping that conversion
  reports 8 GB of memory as "8053028 bytes".
- Temperatures stay in millicelsius, the kernel hwmon convention. The field
  names carry an explicit `-mc` suffix.
- `cpu_capacity` values are not standard across kernel versions. The classifier
  compares against the maximum seen (maximum is a performance core, anything
  lower is efficiency) rather than hardcoding the values one generation
  publishes.

<!-- source: internal/component/host/memory_linux.go -- meminfo parsing and unit conversion -->
<!-- source: internal/component/host/thermal_linux.go -- hwmon sensor reads -->
<!-- source: internal/component/host/cpu_linux.go -- topology and core-role classification -->
<!-- source: internal/component/host/nic_linux.go -- NIC enumeration and the virtual-interface filter -->
<!-- source: internal/component/host/storage_linux.go -- block device enumeration -->
<!-- source: internal/component/host/dmi_linux.go -- board identity -->
<!-- source: internal/component/host/kernel_linux.go -- kernel posture -->
<!-- source: internal/component/host/fsroot_linux.go -- the injectable filesystem root -->

## Traps

**An ethtool ioctl against a fixture path is meaningless.** The ioctl targets
the running kernel's netdev namespace, not the testdata tree. NIC detection
takes a parameter that skips ethtool when a fixture drives it. `x/sys/unix` has
the drvinfo struct but no helper that fires `SIOCETHTOOL` with the right ifreq
layout, so the raw wrapper is about 40 lines here.

<!-- source: internal/component/host/ethtool_linux.go -- the raw SIOCETHTOOL wrapper -->

**The structural virtual-NIC filter has a known blind spot.** A physical
interface with no device link, some PCI-passthrough and container-runtime cases,
is filtered as virtual. None were observed on the target hardware.

**A permission denial does not abort a section.** It lands in the inventory's
error list, so a reader with read-only `/sys` and `/proc` still gets everything
else.

## Related

- `smart.md` for disk health on top of the storage section
- `observability.md` for caching, metrics, and hardware-change events
- `tuning.md` for the write side
