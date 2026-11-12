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

## Readiness: the doctor asks the kernel

The `traffic-usage-ebpf` doctor check is a `post-config` check, so it runs in
whichever process answers doctor (`doctor.Answer`, `runChecks`): the daemon on
`show doctor` and on a `ze doctor` the daemon answers, and the operator's own
`ze doctor` process only when no daemon answers. Either way it does not read
what the plugin's attach achieved. It asks the kernel instead: it loads a two-instruction `sched_cls` program and
attaches it with `link.AttachTCX` to an interface index no device holds. A
kernel that parses the TCX request answers `ENODEV` for the missing device,
which proves TCX is there. Nothing stays attached, and the memlock rlimit is
not touched, so the check stays read-only.

| The kernel answers | Verdict | Doctor code |
|--------------------|---------|-------------|
| `ENODEV` for the probe interface | ready | none |
| `ErrNotSupported` (no TCX attach point), or a non-Linux build | absent | `doctor-traffic-usage-ebpf` |
| `EPERM`, `EACCES` or any other error | unknown: the process that ran the check was not allowed to ask | `doctor-traffic-usage-ebpf-unknown`, with `sudo systemctl stop ze && sudo ze install systemd --force --start` as its fix |

The probe measures this kernel and the privileges of the process that ran it.
On the daemon route that is the daemon, the process that loads the programs, so
the unknown finding tells an operator under systemd to stop ze and rewrite the
unit, and one without systemd to start ze with `CAP_BPF` and `CAP_NET_ADMIN`.
On the local fallback it is the operator's own process, so the same finding
names `sudo ze doctor`, and the answer also carries `doctor-runtime-not-checked`
(`docs/architecture/doctor-and-health-checks.md`, "The local route asks the
running daemon first"). A daemon that fails its own attach logs the failure from
the engine run.
<!-- source: internal/plugins/trafficusage/attach_linux.go -- probeTCX -->
<!-- source: internal/plugins/trafficusage/doctor.go -- checkTrafficUsageBPF, classifyTCXProbe, trafficUsageDiagnostic -->
<!-- source: internal/component/doctor/answer.go -- Answer -->

## The capabilities the daemon loads under

The systemd unit Ze installs runs the daemon as a non-root user holding
`privilege.DaemonCapabilities`, `CAP_BPF` among them, and nothing more. Without
`CAP_PERFMON` the verifier judges a program by its unprivileged rules (no pointer
leaks, no uninitialized stack reads, Spectre v1 and v4 sanitizing), which a root
loader never meets. `TestTrafficUsageLoadsUnderTheUnitCapabilities` runs the
plugin's own `Available` and `Attach`, with track-ip off and on at the default map
size, in a helper under uid 65534 whose effective set it first checks is exactly
that one. It runs as root in a QEMU guest (`//ze:guest-root`) and skips anywhere
else.
<!-- source: internal/core/privilege/capabilities.go -- DaemonCapabilities -->
<!-- source: internal/plugins/trafficusage/unit_capabilities_integration_linux_test.go -- TestTrafficUsageLoadsUnderTheUnitCapabilities -->


## The TCX chain order: the ESP form fast path runs first

The ingress program attaches with `link.AttachTCX` and no anchor
(`attach_linux.go`). With no anchor and no flag, `bpf_mprog_attach`
(`kernel/bpf/mprog.c`) inserts the program at the tail of the interface's TCX
chain. Ze's IPsec ESP form fast path attaches at `link.Head()`
(`docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, "Both ESP forms on one
inbound state"), so whichever of the two starts first, the fast path runs
ahead of the counting program. It rewrites a bare ESP datagram for an SPI it
watches into ESP in UDP, adding an 8-byte UDP header whose destination is the
Child SA's encapsulation port, 4500, and answers `TCX_NEXT`. The counting
program then reads the rewritten header: that ESP is counted under protocol
`udp` and port 4500, eight bytes longer per datagram, not under `esp`. Egress,
and bare ESP while the fast path is off, is counted as it is on the wire. The
operator's reading of the figures is in `docs/guide/traffic-usage.md`, "Bare
ESP counted as UDP to port 4500".

<!-- source: internal/plugins/trafficusage/attach_linux.go -- the ingress link.AttachTCX with no anchor -->
<!-- source: internal/component/ike/dataplane/espform_fastpath_linux.go -- linkAdded, the link.Head() anchor -->
<!-- source: internal/component/ike/dataplane/espform_program_linux.go -- espFormFastPathPorts, espFormFastPathNext -->

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
  stack before the map helpers run, and read every packet-pointer field before
  the first helper call, so no packet pointer is dereferenced after a call.
  This sidesteps the verifier's packet-pointer-invalidation rules completely.
- **The L4 port is read with `bpf_skb_load_bytes`, never through a packet
  pointer.** The TCP or UDP header sits at `14 + ihl*4`, a variable offset.
  Adding that register to a packet pointer is refused to a loader without
  `CAP_PERFMON`, which the systemd unit does not grant: in
  `adjust_ptr_min_max_vals` every `BPF_ADD` and `BPF_SUB` on a pointer goes to
  `sanitize_ptr_alu`, whose `can_skip_alu_sanitation` lets it through only for
  `bypass_spec_v1` or an immediate operand (`BPF_SRC(insn->code) == BPF_K`).
  A register operand, even one holding a known constant, reaches
  `retrieve_ptr_limit`, which answers `REASON_TYPE` for every pointer but the
  stack and a map value, and `sanitize_err` prints "R2 has pointer with
  unsupported alu operation, pointer arithmetic with it prohibited for !root"
  (`kernel/bpf/verifier.c` v7.0, lines 14548 to 14584, 14759 and 14961;
  `bpf_check` sets `bypass_spec_v1` from `CAP_PERFMON` at line 25993).
  `TestTrafficUsageLoadsUnderTheUnitCapabilities` met exactly that refusal at
  `r2 += r1`. So each program computes the L4 offset as a scalar and copies the
  two port bytes to the stack with `bpf_skb_load_bytes(skb, offset, stack, 2)`,
  a helper every `sched_cls` program may call. The helper writes only the
  stack, so the read allocates nothing per packet, and it reads paged skb data
  as well as the linear part. The port is read only when the whole TCP
  (20-byte) or UDP (8-byte) header lies inside `skb->len`; otherwise, and for
  any protocol other than TCP and UDP, the key carries port 0. The one bounded
  form the verifier accepts unprivileged, a pointer plus an immediate, would
  need one branch for each of the eleven valid IHL values.
  <!-- source: internal/plugins/trafficusage/program_linux.go -- directionInsns -->
  <!-- source: internal/plugins/trafficusage/program_test.go -- TestProgram_IPOptionsMoveThePort, TestProgram_TruncatedL4HeaderKeysPortZero, TestProgram_L4OffsetBeyondThePacketKeysPortZero -->
- **`BPF_PROG_TEST_RUN` cannot feed a truncated IPv4 frame.** It refuses an
  `ETH_P_IP` frame whose linear part after the Ethernet header is shorter than
  `struct iphdr`, with EINVAL (`bpf_prog_test_run_skb`, `net/bpf/test_run.c`
  v7.2, lines 1172-1175: `if (skb_headlen(skb) < sizeof(struct iphdr))`). So
  `TestProgram_TruncatedPass` sends the 14- to 33-byte frames through a veth
  pair with the programs attached by TCX, and the 34-byte boundary, the
  shortest frame that route runs, is `TestProgram_IPv4HeaderEndingTheFrameCounts`.
  <!-- source: internal/plugins/trafficusage/program_test.go -- TestProgram_TruncatedPass, TestProgram_IPv4HeaderEndingTheFrameCounts -->
- **A counting program answers `TCX_NEXT`, never `TCX_PASS`.** `tcx_run` stops
  at the first program that answers anything but `TCX_NEXT`, and
  `sch_handle_ingress` and `sch_handle_egress` skip the legacy clsact filters on
  any TCX verdict but `TC_ACT_UNSPEC`, which equals `TCX_NEXT` (`net/core/dev.c`
  v7.2). Returning 0, the value of `TC_ACT_OK` and `TCX_PASS`, therefore turned
  off every clsact filter on a link traffic-usage counted: the traffic-control
  ingress policer, the flow-export sFlow sampler and the interface mirror, which
  are legacy tc, not TCX. Every path of both programs returns `TCX_NEXT` now.
  <!-- source: internal/plugins/trafficusage/program_linux.go -- tcxNext, directionInsns -->
  <!-- source: internal/plugins/trafficusage/tcx_next_integration_linux_test.go -- TestProgram_ReturnsTCXNext, TestProgram_LetsAClsactFilterRun -->

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
  equivalent. `./le repo check` flags an exported symbol with no cross-package
  caller.
