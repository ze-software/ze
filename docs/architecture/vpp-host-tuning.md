# VPP Host-Side Tuning

ze generates VPP's `startup.conf` for cores, buffers and page size. The host
side (hugepage reservation at boot, idle-worker behavior) is tuned here, plus a
`ze doctor` readiness check for it.

<!-- source: internal/component/vpp/register_linux.go -- Linux-only VPP registrations -->
<!-- source: internal/component/vpp/doctor_linux.go -- boot-time hugepage reservation check -->
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

## Kernel arguments go through a derived instance config

The checked-in `gokrazy/ze/config.json` is never edited. `gok` resolves
`<parent_dir>/<instance>/config.json`, so `materializeDerivedParent` builds a
temporary parent directory, symlinks every sibling entry, and writes a
raw-JSON-patched `config.json`. The patch keeps unknown fields by decoding into
`map[string]json.RawMessage`, and it excludes `builddir` so the rebuild is cold
and isolated. `kernelargs.go` is the shared seam for this feature and for CPU
isolation.

## The doctor check owns its roots

The check is owned by the vpp component and is Linux-tagged. It reads sysfs and
procfs through overridable roots, and it collapses to one error when nothing is
reserved rather than reporting per-node noise.

**An override key must be registered.** `env.Get` aborts on an unregistered
key, so the functional test's `ze.test.doctor.hugepages-root` is registered with
`env.MustRegister` at package level. Without the registration `ze doctor`
crashes the moment the check runs, while the unit tests keep passing. The
symptom is a check that produces no diagnostic end to end.

## Test placement

A Linux-tagged test with no `integration` tag runs in the native unit groups on
a Linux host. Adding it to `./le qemu all-tests`, which builds with
`-tags integration`, does not change the unit population.
