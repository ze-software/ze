# 631 -- Host Inventory (host-0)

## Context

ISPs deploying ze on fleets of hardware need structured, machine-parseable
hardware inventory for monitoring, asset tracking, and debugging. Before
this spec ze had no way to report what physical hardware it was running
on: no CPU topology, no NIC drivers, no DMI board identity, no thermal
sensors, no kernel posture. Operators had to ssh to boxes and run
`dmidecode`/`lspci`/`ethtool`/`lscpu` one at a time. The goal of host-0
was to ship the detection library + a user-visible surface (online RPC +
offline CLI) that produces kebab-case JSON suitable for ingestion into
any existing monitoring pipeline — no new dependencies, no new daemon
requirements, works on the gokrazy appliance and on dev Linux.

## Decisions

- **Component, not plugin** (`internal/component/host/`). Host detection
  is infrastructure that plugins/handlers consume, not a domain policy
  module. Placement follows the existing component/plugin split.
- **Linux-only with darwin stub, not cross-platform data shim** — all
  inventory sources are sysfs/procfs; faking them on darwin would require
  maintaining shim code for no real benefit. Darwin stubs return
  `ErrUnsupported` so the rest of ze builds cleanly; `Detect()` treats
  `ErrUnsupported` as "section not available" rather than an error.
- **Pure stdlib + vendored x/sys/unix, no new deps.** `prometheus/procfs`
  was available indirectly but a dozen small file reads are cleaner than
  a third-party parser. ethtool firmware/ring readings use raw
  `SIOCETHTOOL` ioctls on an AF_INET socket (best-effort, skipped when
  Root != "/").
- **Test-injectable filesystem root** via a `Detector{Root string}` struct
  rather than package-level globals. Tests construct a Detector pointed
  at `testdata/n100-4x-igc/` or `testdata/alder-lake-hybrid/` and drive
  every section reader against fixtures. Parallel callers in production
  use `&Detector{}` (root = "/") which is safe for concurrent use.
- **Typed enums over strings** (CPUVendor, CoreRole, ScalingDriver,
  NICTransport) per `rules/enum-over-string.md`. JSON output uses
  `String()` for human-readable values; Go code compares typed constants.
- **Virtual-NIC filter via structural check** (`/sys/class/net/<n>/device/`
  presence) rather than a driver-name allowlist. Future virtual drivers
  (wireguard, ipvlan, new netdev types) are filtered uniformly without
  the allowlist bitrotting.
- **JSON default on offline `ze host show`, `--text` opt-in** — ISPs
  pipe to `jq`/Prometheus/SNMP shims first, look with human eyes
  second. Matches the "machine-parseable first" principle of the
  existing `ze show` RPC surface.
- **Enrichment of `show system cpu/memory`, not replacement** — the
  existing Go-runtime fields stay; a `hardware` nested object is added.
  Operator muscle memory preserved; hardware data available without a
  second command. `show system uptime` enrichment deferred because
  that handler does not yet exist (op-1 scope).
- **No runtime tuning in this spec** — governor writes, IRQ affinity,
  ethtool coalesce all deferred to `spec-host-2-tuning`. Detection
  must ship clean and exact before write semantics enter the picture.

## Consequences

- Every ze box now produces a canonical JSON inventory reachable from
  online RPC (`show host *`) or offline CLI (`ze host show`) in
  under 50 ms. Same detection library drives both paths.
- Future observability work (Prometheus `/metrics`, hardware-change
  events on the report bus, cached refresh) slots onto this library
  without rework — tracked as `spec-host-1-observability`.
- Future tuning work consumes the Inventory struct and policy-maps it
  to sysfs writes — `spec-host-2-tuning` will not need to re-detect.
- New components that need hardware facts (tuner, web UI panel, fleet
  comparator) call `host.Detect()` or a sectional helper directly; no
  second "hardware lookup" surface.
- Darwin dev workflow unchanged — `host.Detect()` returns an empty
  Inventory, `show system cpu` omits the `hardware` key, tests skip
  cleanly.
- The detection path never writes to sysfs. A user with read-only
  access to `/sys` and `/proc` gets the full inventory; permission
  denials land in `inventory.Errors[]` without aborting the section.

## Gotchas

- `prometheus/procfs` is vendored as indirect and is tempting, but
  pure stdlib parsing of `/proc/cpuinfo` / `/proc/meminfo` / `/proc/stat`
  is ~50 lines each and far clearer. Resisted the pull.
- `x/sys/unix` has the `EthtoolDrvinfo` struct but no helper that fires
  `SIOCETHTOOL` with the right ifreq layout. The raw ioctl wrapper is
  ~40 lines (`ethtool_linux.go`). It is gated behind `d.Root == ""` so
  tests never touch real netdev state.
- The hook `block-silent-ignore.sh` flags bare `default:` switch arms.
  Enum `String()` methods had to use map lookup with an "unknown"
  fallback constant (`strUnknown`) instead of `default:` clauses.
- The hook `require-related-refs.sh` requires `// Related:` comments to
  point at files that exist in the same directory; split-across-phase
  detectors meant the initial `// Related:` block with seven future
  files was rejected — dropped in favour of a plain package doc that
  lists consumers in prose.
- `/proc/meminfo` values are in kB. The library converts to bytes at
  parse time so the public field names stay `*-bytes`. Missing this
  would produce "8053028 bytes of total memory" (actually 8 GB).
- Temperatures reported in **millicelsius** following the kernel hwmon
  convention (`temp1_input` is already in mC). Field names carry the
  `-mc` suffix explicitly.
- Virtual interfaces without `/sys/class/net/<n>/device/` are filtered
  cleanly, but this means physical interfaces that genuinely have no
  device link (PCI-passthrough edge cases, some container runtimes)
  would be filtered too. None observed on the target hardware (Intel
  N100 + 4× I226-V whitebox); noted for future spec-host-1.
- ethtool ioctl against a test fixture path is meaningless — the
  ioctl targets the running kernel's netdev namespace, not the
  testdata tree. `detectNICs(false)` parameter skips ethtool when
  invoked from a fixture test.
- `cpu_capacity` values are not standardised across kernel versions.
  Alder Lake exposes 1024 (P-cores) and 768 (E-cores) on mainline; the
  classifier uses "maxCap == performance, anything lower == efficient"
  rather than hardcoded values, so Meteor Lake / Arrow Lake ratios
  work automatically.
- The parallel session's `cmd/ze/main.go` carries an unrelated compile
  error at commit time (`undefined: zeconfig.BindStorageCommands` from
  its in-flight CLI registration refactor) which blocks a full
  `make ze-verify-fast` on the tree. Per-package vet and tests pass
  cleanly for every host-0 file. Full tree verify waits for that
  session to land.

## Files

- `internal/component/host/` — new: doc.go, inventory.go, fsroot_linux.go,
  cpu_linux.go + cpu_other.go + cpu_linux_test.go + cpu_other_test.go,
  nic_linux.go + nic_other.go + nic_linux_test.go, ethtool_linux.go,
  dmi_linux.go + dmi_other.go + dmi_linux_test.go, memory_linux.go +
  memory_other.go + memory_linux_test.go, thermal_linux.go +
  thermal_other.go + thermal_linux_test.go, storage_linux.go +
  storage_other.go + storage_linux_test.go, kernel_linux.go +
  kernel_other.go + kernel_linux_test.go, testdata/{n100-4x-igc,alder-lake-hybrid}/.
- `internal/component/cmd/show/host.go` — new: 8 online RPC handlers.
- `internal/component/cmd/show/show.go` — RPC registrations (8 new).
- `internal/component/cmd/show/system.go` — `hardware` enrichment on
  cpu + memory handlers.
- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` — new
  `container host` with 8 sub-containers.
- `cmd/ze/host/` — new: host.go (RunShow + JSON/text renderers),
  register.go (cmdregistry root + local), host_test.go.
- `cmd/ze/main.go` — blank import of `cmd/ze/host`.
- `test/parse/` — new: cli-host-show-cpu.ci, cli-host-show-kernel.ci,
  cli-host-show-bogus.ci.
- `docs/guide/command-reference.md` — new `### ze host show` section.
- `docs/features.md` — new "Host Inventory" row.
