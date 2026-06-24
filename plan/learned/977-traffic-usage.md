# 977 -- traffic-usage

## Context
Ported `git.exa.net.uk/tech/development/lan-bandwidth-exporter` into ze as a native
system plugin (`traffic-usage`) that does eBPF TCX per-(port,protocol) and opt-in
per-IP byte accounting on operator-selected interfaces, exported as Prometheus
metrics. The hard constraint (user decision) was that the eBPF programs ship with
**nothing on disk**: assembled in pure Go from `asm.Instructions` and loaded from
memory, with no C source, no committed `.o`, and no clang/LLVM in the build. The
goal was top-talker / per-port visibility without the cardinality and toolchain
cost of the upstream, and without the bpf2go-`.o` pattern the l2tp XDP plugin uses.

## Decisions
- Pure-Go eBPF (`cilium/ebpf` `asm.Instructions`) over bpf2go committed `.o`: nothing on disk, no toolchain change. Cost accepted: hand-assembly with no compiler (see below).
- Standalone plugin over extending flow-export: flow-export is sampled records to external collectors; this is unsampled local byte counters. Different path/output/consumer.
- Generate the program PER-ATTACH from `(maxEntries, trackIP)`: when track-ip is off, the IP maps and the per-packet IP accounting are simply not emitted. "Collected only when track-ip" is enforced in the data plane, not just at publish time.
- Absolute byte totals as GaugeVec `.Set` over CounterVec: ze's `Counter` has no `Set`, and `rate()` works on gauges (matches `rate.go`'s `ze_interface_rx_bytes_total`).
- iface lifecycle via `iface.RegisterCollectNotify` (1 Hz snapshot reconcile) over EventBus topic subscription: same up/down/rename handling, mirrors flow-export, no `ConfigureEventBus` needed.
- No bpffs pinning over upstream's `/sys/fs/bpf` pins: ze owns the whole lifecycle in one process; in-process FDs suffice and avoid read-only-root pin-dir concerns. Confirmed under QEMU.
- Single long-lived `Monitor` reconciled idempotently over flow-export's per-config `Swap`.
- Interfaces under `interfaces { interface <name> { enabled } }` keyed list, mirroring OSPF/ISIS, over a flat `interface` leaf-list: consistent operator-facing shape, per-interface enable.
- Resolve the configured (logical) ze name to the OS device via `iface.Resolve` (honors os-name / mac-match selectors, returns the OS ifindex + state) over a naive `ListInterfaces` name match, which ignores the selectors.

## Consequences
- ze now has a **pure-Go eBPF pattern** (program built in `asm.Instructions`, loaded via `ebpf.NewCollection`, attached via `link.AttachTCX`). Future kernel-dataplane features can follow it instead of bpf2go; the l2tp-12 `.o` plugin could later adopt it.
- `BPF_PROG_TEST_RUN` (`prog.Test`) tests are **load-bearing**, not optional: there is no compiler, so every parse path / byte-order / offset is only proven by feeding crafted packets and asserting map contents under QEMU.
- `github.com/cilium/ebpf` is now a vendored dependency (subpackages `asm`, `link`, `rlimit`, `btf`, `internal/*`). Adding it surfaced and fixed a `changed-pkgs.sh` gap (it linted/tested vendored code).
- Track-ip stays OFF by default for cardinality; per-(port,proto) is always on. LRU caps + stale-timeout bound `/metrics`.

## Gotchas
- `asm.StoreImm` returns `InvalidOpCode` for `asm.DWord` (no 8-byte immediate store). Symptom under QEMU: "instruction N: invalid opcode" (and N counts `LoadMapPtr` as 2 slots, so it points just *after* the offending insn). Fix: build u64 stack values with `Mov.Imm` + `StoreMem`. `asm.Ja.Label` is fine (was a red herring).
- BPF `LDX` from packet memory is **host byte order**: ethertype IPv4 compares to `0x0008` (htons(0x0800)) on LE; L4 ports need `HostTo(BE, reg, Half)` (ntohs) to store host-order; IPv4 addresses are stored raw and decoded little-endian in userspace.
- Structure the program **parse-everything-first, then all map ops**: stage both keys on the stack before any helper call, so no packet pointer is dereferenced after a call. Sidesteps the verifier's packet-pointer-invalidation rules entirely.
- `go mod vendor` failed with `unlinkat ... permission denied`: vendored files are read-only copies from the module cache, and it can't recreate read-only dirs. Fix: `chmod -R u+w vendor` then re-run (NOT git restore -- re-running vendor recreates the tree deterministically).
- The QEMU 9p workspace mount is **live**: a source edit made on the host during a running `qemu-run.py` test is picked up by the in-VM `go test` compile.
- `internal/component/plugin/all/all_test.go` has golden lists (plugin names, wire methods, YANG providers) that MUST be updated when adding a plugin, or `TestRegistered*`/`TestYANGSchemaProviders` fail.
- The eBPF program's flags (track-ip, max-entries) are fixed at generation time, so reconcile must compare the attached params to the new config and detach+re-attach on change. Keying reconcile on interface-presence alone makes a runtime `track-ip` toggle silently produce nothing (found in review; fixed via `attachedIface{trackIP,maxEntries}`). Only `interval` applies without a rebuild.
- The 1 Hz iface callback (RegisterCollectNotify) is used as a periodic tick only; the lifecycle re-resolves each configured name via `iface.Resolve` (whose cache the same link events invalidate), not by reading the snapshot slice, so os-name/mac-match selectors are honored on down/up too.
- `ze-validate` flags exported symbols with no cross-package caller: keep package-internal helpers (newMonitor, getMonitor) unexported even when a sibling plugin (flowexport's GetExporter) exports the equivalent.
- Name the plugin's global on/off leaf `enabled`, not `enable`: 8 of 9 plugins (dhcpserver, flowexport, imageserver, isis, ntp, ospf, tftpserver, + bfd/api) use `leaf enabled`; only iface and (initially) traffic-usage used `enable`. Matching avoids the `enable` (global) vs `enabled` (per-interface) confusion (found in review).
- Drop an interface's metric series on detach (`deleteInterfaceSeriesLocked`), not only via stale-timeout: with stale-timeout 0 (disabled) a removed interface's series would otherwise leak forever, and on re-attach the eBPF maps start fresh so the old values are stale anyway. Series republish on the next poll if still desired (found in review).
- `BPF_PROG_TEST_RUN` enforces a **per-kernel minimum sched_cls input size**: ze's runtime kernel (7.1.1, from `runtime.config`) rejects EVERY frame shorter than a full eth+IPv4 header (34 bytes) carrying the IPv4 ethertype with EINVAL, where stock Alpine ran 18-byte frames fine. A negative-path test feeding a deliberately-truncated frame must probe upward across sizes and skip when the kernel refuses them all (OOB-safety is verifier-guaranteed at load regardless -- `TestProgram_Loads`). Surfaced by AC-15's runtime-kernel proof (`make ze-kernel GOKRAZY_ARCH=arm64` then `make ze-qemu-traffic-usage-test`), which the stock-Alpine QEMU runs had masked. AC-15 now done: all accounting/attach/scrape tests pass on the runtime kernel; only the sub-header truncation TEST_RUN skips there.

## Files
- Created: `internal/plugins/trafficusage/{trafficusage,config,monitor,metrics,show,register,doctor,program_linux,attach_linux,attach_other}.go` + matching `_test.go`, `attach_integration_linux_test.go`, `program_test.go`, `yang/{ze-traffic-usage-conf,ze-traffic-usage-cmd}.yang` (+ generated `embed.go`/`register.go`/`self_containment_test.go`), `test/plugin/traffic-usage-config.ci`, `docs/guide/traffic-usage.md`.
- Modified: `go.mod`/`go.sum` (+vendor), `gokrazy/kernel/runtime.config`, `test/install/kernel-compose.ci`, `tools/kernel-builder/build.sh`, `mk/test-integration.mk`, `Makefile`, `scripts/dev/changed-pkgs.sh`, `internal/core/diagnostic/codes.go`, `internal/component/plugin/all/all{,_test}.go`, and docs (DESIGN, plugin-overview, features, plugins, configuration, command-reference, architecture/api/commands, plugin-development/metrics, functional-tests).
