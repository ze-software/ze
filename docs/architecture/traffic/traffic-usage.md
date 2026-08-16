# Traffic Usage: Pure-Go eBPF Byte Accounting

`traffic-usage` does eBPF TCX per-(port, protocol) accounting, and opt-in per-IP
byte accounting, on operator-selected interfaces, exported as Prometheus metrics.

The hard constraint, set by the owner, is that the eBPF programs ship with
NOTHING ON DISK: assembled in pure Go from `asm.Instructions` and loaded from
memory. No C source, no committed `.o`, no clang or LLVM in the build.

## Decisions

### Pure-Go eBPF, not bpf2go

<!-- source: internal/plugins/trafficusage/program_linux.go -- program assembly from asm.Instructions -->
<!-- source: internal/plugins/trafficusage/attach_linux.go -- ebpf.NewCollection, link.AttachTCX -->

The l2tp XDP plugin uses bpf2go with a committed `.o`. This plugin builds the
program in `asm.Instructions`, loads it with `ebpf.NewCollection`, and attaches
it with `link.AttachTCX`. The accepted cost is hand-assembly with no compiler.

Any future kernel-dataplane feature can follow this pattern instead of bpf2go.

**`BPF_PROG_TEST_RUN` tests are load-bearing, not optional.** There is no
compiler, so every parse path, byte order and offset is proven only by feeding
crafted packets and asserting map contents under QEMU.

### A standalone plugin, not part of flow-export

flow-export produces sampled records for external collectors. This produces
unsampled local byte counters. Different path, different output, different
consumer.

### The program is generated per attach

<!-- source: internal/plugins/trafficusage/monitor.go -- attachedIface, reconcile -->

The program is built from `(maxEntries, trackIP)`. When `track-ip` is off, the IP
maps and the per-packet IP accounting are NOT EMITTED. "Collected only when
track-ip is set" is enforced in the data plane, not at publish time.

Because the flags are fixed at generation time, reconcile compares the attached
parameters against the new config and detaches and re-attaches on a change.
Keying reconcile on interface presence alone makes a runtime `track-ip` toggle
silently produce nothing. Only `interval` applies without a rebuild.

### Byte totals are gauges

<!-- source: internal/plugins/trafficusage/metrics.go -- GaugeVec Set -->

Absolute byte totals use `GaugeVec.Set`, not a counter. Ze's `Counter` has no
`Set`, and `rate()` works on gauges. This matches `ze_interface_rx_bytes_total`
in `iface/rate.go`.

### No bpffs pinning

<!-- source: internal/plugins/trafficusage/attach_linux.go -- in-process FD lifecycle -->

The upstream exporter pins into `/sys/fs/bpf`. Ze owns the whole lifecycle in one
process, so in-process file descriptors are enough and there is no read-only-root
pin-directory concern. Confirmed under QEMU.

### Lifecycle from the iface tick

<!-- source: internal/plugins/trafficusage/register.go -- runEngine, iface.SubscribeCollectNotify, iface.UnsubscribeCollectNotify -->

The 1 Hz iface snapshot callback drives an idempotent reconcile of one long-lived
`Monitor`, rather than an EventBus topic subscription. It gives the same up, down
and rename handling, and mirrors flow-export.

The callback is used as a periodic TICK only. The lifecycle re-resolves each
configured name through `iface.Resolve`, whose cache the same link events
invalidate, and does not read the snapshot slice. That is what honors the
`os-name` and `mac-match` selectors on a down and up cycle. A naive
`ListInterfaces` name match ignores those selectors.

### Config shape

Interfaces live under `interfaces { interface <name> { enabled } }`, a keyed
list, mirroring OSPF and IS-IS, and not a flat leaf-list. The operator-facing
shape stays consistent and each interface has its own enable.

`track-ip` is off by default for cardinality. Per-(port, proto) is always on. LRU
caps and a stale timeout bound the `/metrics` output.

## eBPF traps

These cost days if rediscovered:

- **`asm.StoreImm` returns `InvalidOpCode` for `asm.DWord`.** There is no 8-byte
  immediate store. The QEMU symptom is "instruction N: invalid opcode", and N
  counts `LoadMapPtr` as two slots, so it points just AFTER the offending
  instruction. Build a u64 stack value with `Mov.Imm` and `StoreMem`.
- **A BPF `LDX` from packet memory is HOST byte order.** The IPv4 ethertype
  compares against `0x0008` (`htons(0x0800)`) on a little-endian host. L4 ports
  need `HostTo(BE, reg, Half)` to store in host order. IPv4 addresses are stored
  raw and decoded little-endian in userspace.
- **Parse everything first, then do all map operations.** Stage both keys on the
  stack before any helper call, so no packet pointer is dereferenced after a
  call. This sidesteps the verifier's packet-pointer-invalidation rules
  completely.
- **`BPF_PROG_TEST_RUN` enforces a per-kernel minimum sched_cls input size.**
  Ze's runtime kernel rejects every frame shorter than a full eth plus IPv4
  header (34 bytes) carrying the IPv4 ethertype, with EINVAL. Stock Alpine ran
  18-byte frames. A negative-path test that feeds a truncated frame must probe
  upward across sizes and skip when the kernel refuses them all. Out-of-bounds
  safety is verifier-guaranteed at load in any case.

## Build and review traps

- `go mod vendor` fails with `unlinkat ... permission denied`, because vendored
  files are read-only copies from the module cache and it cannot recreate
  read-only directories. Run `chmod -R u+w vendor` and re-run. Do not
  `git restore`: re-running vendor recreates the tree deterministically.
- The QEMU 9p workspace mount is LIVE. A source edit made on the host during a
  running test is picked up by the in-VM `go test` compile.
- Name a plugin's global on/off leaf `enabled`, not `enable`. Eight of nine
  plugins use `enabled`, and the mismatch creates an `enable` (global) versus
  `enabled` (per-interface) confusion.
- Drop an interface's metric series on detach, not only through the stale
  timeout. With the stale timeout disabled, a removed interface's series would
  leak forever, and on re-attach the eBPF maps start fresh so the old values are
  stale anyway.
- Keep package-internal helpers unexported even when a sibling plugin exports the
  equivalent. `ze-repository-check` flags an exported symbol with no cross-package
  caller.
