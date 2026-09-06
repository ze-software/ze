# VPP Host-Side Tuning

ze generates VPP's `startup.conf` for cores, buffers and page size. The host
side is tuned here: the hugepage reservation at boot, the CPU isolation the
dataplane workers are pinned inside, and idle-worker behavior, each with a
`ze doctor` readiness check.

<!-- source: internal/component/vpp/register_linux.go -- Linux-only VPP registrations -->
<!-- source: internal/component/vpp/doctor_linux.go -- boot-time hugepage reservation check -->
<!-- source: internal/component/vpp/cpuset.go -- host CPU inventory and worker core placement -->
<!-- source: internal/component/vpp/doctor_cpu_linux.go -- CPU isolation check -->
<!-- source: internal/core/cpulist/cpulist.go -- the CPU list grammar, declared once -->
<!-- source: internal/appliance/kernelargs.go -- derived gokrazy instance config and kernel arguments -->

## YANG groups by operator concern, not by file section

`poll-sleep-usec` is a VPP `unix` section directive, not a `cpu` one. The leaf
still lives under `vpp/cpu`, because ze groups configuration by what the
operator is thinking about, and the emitter writes it into the `unix { }`
block. The `memory` container already works this way: it feeds the buffers,
heapsize and statseg sections.

An explicit `0` is emitted. An absent leaf produces byte-identical output to
before the leaf existed.

## Hugepages are an image fact, not a runtime leaf

Hugepage reservation is `image.hugepages` in the appliance `config.json`, not
YANG. It is consumed on the build host at image-assembly time, before any
target YANG config exists. Optional `image.memory-bytes` bounds the reservation
at 50% and also sizes the QEMU `-m` value for `ze appliance run`.

## CPU isolation is an image fact too

`image.isolated-cpus` sits beside `image.hugepages` in the appliance
`config.json`, for the same reason: it is consumed on the build host at
image-assembly time, before any target YANG config exists. The YANG side
CONSUMES the isolation; the image side REQUESTS it.

## Kernel arguments go through a derived instance config

The checked-in `gokrazy/ze/config.json` is never edited. `gok` resolves
`<parent_dir>/<instance>/config.json`, so `materializeDerivedParent` builds a
temporary parent directory, symlinks every sibling entry, and writes a
raw-JSON-patched `config.json`. The patch keeps unknown fields by decoding into
`map[string]json.RawMessage`, and it excludes `builddir` so the rebuild is cold
and isolated. `kernelargs.go` is the shared seam for this feature and for CPU
isolation.

## Worker cores come from the kernel, not from arithmetic

A VPP worker busy-polls. A worker that shares a CPU with the Linux scheduler
therefore competes with every runnable task on that CPU, which is what
`isolcpus` exists to prevent. So `vpp.cpu.workers N` does not mean "the N cores
after `main-core`". It means "N of the cores the kernel isolated", read from
`/sys/devices/system/cpu/isolated`, lowest first, with `main-core` excluded.

On a host that isolated nothing, the list falls back to the contiguous block
after `main-core`, which is the placement ze emitted before it read the isolated
set. That fallback is not silent: the `vpp-cpu-isolation` doctor check reports
it.

`resolveWorkerCores` is the single producer of the list. `GenerateStartupConf`
writes what it returns, and `CPUSettings.validateAgainst` calls the same
function, so a config that validates is a config ze can write a core list for.
Two producers would let a config pass verify and then fail at startup.

## An empty isolated set is not an unreadable one

`CPUInventory` carries `IsolationKnown` beside `Isolated` because "the kernel
isolated no CPU" and "this host could not tell us" are different answers, and
one empty slice cannot hold both. A reader that takes the second for the first
pins workers to CPUs Linux still schedules on and reports a clean state.

The two files split on the same line. An unreadable `online` file is an ERROR
from `hostCPUInventory`: without it ze cannot say whether a requested core
exists, and config validation refuses rather than accepting a placement nothing
checked. An unreadable `isolated` file is not an error, because isolation
governs placement quality rather than validity; it leaves `IsolationKnown`
false, and the doctor check says so in its own words.

## One grammar for isolcpus, sysfs and corelist-workers

`internal/core/cpulist` parses and renders CPU lists such as `0-3,7`. Three
surfaces speak it: the kernel's `isolcpus` argument, the two sysfs files, and
VPP's `corelist-workers`. It sits in `internal/core/` rather than inside the VPP
component because the appliance builder validates `image.isolated-cpus` with it
before writing the kernel argument, and a second copy of the grammar would be a
future disagreement with nothing to arbitrate it.

The appliance emits three arguments, not one. `isolcpus` takes the CPUs out of
the scheduler's domains, `nohz_full` stops the periodic tick on them, and
`rcu_nocbs` moves their RCU callbacks to a housekeeping CPU. `isolcpus` alone
leaves a tick and a callback in the packet path. CPU 0 is refused: Linux places
the boot CPU, most timer work and the default IRQ affinity there.

## The doctor check owns its roots

The check is owned by the vpp component and is Linux-tagged. It reads sysfs and
procfs through overridable roots, and it collapses to one error when nothing is
reserved rather than reporting per-node noise.

The CPU isolation check reads through the same shape, under
`ze.test.vpp.cpu.root`.

**An override key must be registered.** `env.Get` aborts on an unregistered
key, so the functional test's `ze.test.doctor.hugepages-root` is registered with
`env.MustRegister` at package level. Without the registration `ze doctor`
crashes the moment the check runs, while the unit tests keep passing. The
symptom is a check that produces no diagnostic end to end.

## Test placement

A Linux-tagged test with no `integration` tag runs in the native unit groups on
a Linux host. Adding it to `./le qemu all-tests`, which builds with
`-tags integration`, does not change the unit population.
