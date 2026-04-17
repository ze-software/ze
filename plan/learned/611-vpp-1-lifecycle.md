# 611 -- vpp-1 Lifecycle Management

## Context

Ze needed a VPP component to manage the full VPP process lifecycle: generate
startup.conf from YANG config, bind DPDK NICs to vfio-pci, exec VPP, connect
via GoVPP, monitor health, restart on crash with exponential backoff, and clean
shutdown (SIGTERM, wait, unbind, restore drivers). No systemd or external
process manager, because gokrazy appliance has no init system. This is the
foundation: every other VPP spec (fib, iface, telemetry, MPLS) depends on a
running VPP instance with a healthy GoVPP connection.

## Decisions

- **Self-contained Manager with Run(ctx) loop** over SDK-callback-driven
  lifecycle. The Manager owns configure/start/monitor/restart/stop; the plugin
  registration (`register.go`) is a thin entry point that creates the Manager
  and calls Run in OnStarted. SDK event loop continues for config reload.
- **`vpp.external` config leaf** over a `binary-path=/bin/sleep infinity`
  dev hack. External mode skips exec/supervise and DPDK binding but still
  connects GoVPP, runs stats poller, and emits EventBus events. Three real
  use cases: systemd-managed VPP, container sidecar, ze-test stub harness.
- **EventBus `("vpp", "connected/disconnected/reconnected")` namespace** for
  lifecycle notifications to dependents. Direct import `vpp.Channel()` for
  GoVPP API access. Dependency ordering via `Dependencies: ["rib", "vpp"]`.
- **No env var overrides** for VPP config leaves. VPP config is entirely
  YANG-driven (PCI addresses, hugepages, socket paths are config-time
  decisions, not operational knobs). The spec's AC-11 was struck as not
  applicable.
- **Platform-split for DPDK** (`dpdk_linux.go` for modprobe) following
  existing backend pattern (`fibkernel/backend_linux.go`).

## Consequences

- Every VPP-dependent plugin declares `Dependencies: ["vpp"]` and subscribes
  to EventBus for lifecycle signals. fibvpp uses this for replay-request on
  reconnect.
- startup.conf generation is template-based from YANG leaves, not a config
  file copy. IPng production values are the defaults (128K buffers, 1G heap,
  64MB linux-nl rx-buffer).
- DPDK NIC binding saves and restores original drivers on shutdown. Mellanox
  mlx5 (RDMA) is detected and skipped for vfio binding.
- The `vpp.external` leaf shipped with vpp-7 test harness (learned/610) and
  is documented in `docs/guide/vpp.md`.

## Gotchas

- GoVPP AsyncConnect returns a channel of connection events. The Manager
  must drain this channel or the connection hangs. The 30s connect timeout
  with 10 attempts at 1s intervals is tuned for VPP's typical 2-5s boot.
- VPP's LCP sections (linux-cp, linux-nl) must be conditionally included
  in startup.conf based on `lcp.enabled`. Omitting `linux_nl_plugin.so`
  when LCP is enabled causes silent failures (no TAP mirrors).
- PCI address validation must be strict (DDDD:DD:DD.D hex format, function
  0-7) to prevent sysfs path traversal in the bind/unbind sequence.

## Files

- `internal/component/vpp/vpp.go` -- Manager lifecycle (Run, runOnce)
- `internal/component/vpp/config.go` -- YANG config parsing + validation
- `internal/component/vpp/startupconf.go` -- startup.conf generation
- `internal/component/vpp/dpdk.go` + `dpdk_linux.go` -- DPDK NIC binding
- `internal/component/vpp/conn.go` -- GoVPP connection management
- `internal/component/vpp/register.go` -- Plugin registration
- `internal/component/vpp/events/events.go` -- VPP EventBus namespace
- `internal/component/vpp/schema/ze-vpp-conf.yang` -- YANG module
- `test/vpp/001-boot.ci` -- Functional test (external mode + stub handshake)
