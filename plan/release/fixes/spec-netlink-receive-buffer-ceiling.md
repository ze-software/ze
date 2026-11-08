# Spec: netlink receive buffers clamped by net.core.rmem_max

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P1 |
| Release reason | VPP's linux-cp netlink socket asks for 64 MiB and gets 8 MiB, because nothing in Ze raises `net.core.rmem_max`. A notification the kernel cannot queue is lost before VPP reads it, so the VPP data plane can miss route, neighbor and link changes the kernel made. Forwarding correctness of a shipped backend. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze writes `nl-rx-buffer-size 67108864` into VPP's `linux-nl` startup section,
and Ze's own iface netlink monitor asks for 1 MiB on each of its sockets. Both
requests use SO_RCVBUF, which the kernel clamps to `net.core.rmem_max`. Ze never
raises that sysctl, so each consumer gets the host's ceiling, not the size its
design chose.

Goal: before VPP starts, and before any Ze netlink socket that asks for a size
opens, `net.core.rmem_max` is at least the largest size a running consumer
asks for, set through the sysctl component. An operator value below that floor
is refused at verify while the consumer that needs it is configured, rather
than silently shrinking the buffer.

VyOS fixed the same class in T9484 (`57a5f6b62` in
`~/Code/github.com/vyos/vyos-1x`): `vpp.py` raises `net.core.rmem_max` to the
requested size before VPP is (re)started, keeps a higher value, and makes
`system sysctl` use the required size with a warning while VPP is configured.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/forwarding.md` - "Kernel tunables go through `sysctl`": a plugin contributes a default over the EventBus and never writes `/proc`
- [ ] `ai/digests/vpp-dataplane.md` - the VPP supervisor and startup sequence
- [ ] `docs/architecture/core-design.md` - the sysctl layers (config over transient over plugin defaults)

**Key insights:**
- SO_RCVBUF is clamped to `rmem_max` with no error; only SO_RCVBUFFORCE with CAP_NET_ADMIN bypasses it.
- The kernel doubles the granted value for bookkeeping, so a clamp at 4 MiB reads back as 8 MiB.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/vpp/startupconf.go` - the `linux-nl` section writes
  `nl-rx-buffer-size 67108864` as a literal.
- [ ] VPP v25.10 `src/plugins/linux-cp/lcp_nl.c` (fetched from
  `FDio/vpp` at tag `v25.10`, the version `third_party/vpp-linux-cp/README.md`
  pins) - `lcp_nl_open_socket` and `lcp_nl_set_buffer_size` size the socket with
  libnl `nl_socket_set_buffer_size`, and libnl `lib/socket.c`
  `nl_socket_set_buffer_size` calls `setsockopt(..., SO_RCVBUF, ...)`. The
  request is therefore clamped to `net.core.rmem_max`.
- [ ] `internal/plugins/iface/netlink/monitor_linux.go` -
  `monitorReceiveBufferBytes` (1 MiB) is passed as `ReceiveBufferSize` with no
  force flag to the link, address and neighbor subscriptions; its comment
  records the choice not to force. `neighbor_watch_linux.go`,
  `address_watch_linux.go` and `counter_linux.go` reuse the same constant
  unforced. The vendored `vishvananda/netlink` `SetReceiveBufferSize` uses
  SO_RCVBUF unless the force flag is set.
- [ ] `internal/component/vpp/vpp.go` - `VPPManager.Run` writes `startup.conf`,
  binds DPDK, then `runOnce` execs VPP. Nothing in the VPP component touches a
  sysctl.
- [ ] `internal/component/sysctl/sysctl.go` - `store.setDefault`: a plugin
  default is last-writer-wins between sources, with a "default overwritten"
  warning; a config value overrides any default.
- Ze's 7.3 kernel (the lab Docker host, `uname -r` 7.3.0-ze) reads
  `net.core.rmem_max` 4194304 and `net.core.rmem_default` 212992, so VPP gets
  8 MiB, not 64 MiB. Ze's 1 MiB monitor request fits under that ceiling on
  Ze's kernel. A stock-kernel host whose ceiling is 212992 clamps the monitor
  too; the kernel versions that carry each default are not yet checked.

**Behavior to preserve:**
- A host whose `rmem_max` is already above the floor keeps its value; Ze never
  lowers it.
- An operator's explicit `sysctl` config still decides the value when it meets
  the floor.

**Behavior to change:**
- Ze raises `net.core.rmem_max` to the largest size its running consumers ask
  for, before those consumers open their sockets.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Plugin sysctl defaults over the EventBus | `internal/component/sysctl/events` `EventDefault`, used by `internal/plugins/fib/kernel/fibkernel.go` and `internal/component/iface/config_sysctl.go` | A plugin registers a required default; the sysctl component writes it | Extends: the VPP component and the iface netlink plugin contribute the `net.core.rmem_max` they need through it |
| Sysctl default precedence | `internal/component/sysctl/sysctl.go` `store.setDefault` | Last default wins between sources, config overrides all | Extends: two sources asking for a ceiling need the larger of the two to win, not the last; the design names how a floor is expressed without a per-key case in the store |
| Applied notification | `internal/component/sysctl/events` `EventApplied` | Tells subscribers a value reached the kernel | Extends: the VPP component waits for it before `runOnce` execs VPP, so the order holds across a process boundary |
| VPP startup sequence | `internal/component/vpp/vpp.go` `VPPManager.Run`, `runOnce` | Writes `startup.conf`, binds DPDK, execs VPP | Extends: the floor is requested and applied before the exec, and again before each restart |
| Exact or refuse at verify | `ai/doors/forwarding.md`, "Exact or refuse, at verify" | A backend that cannot apply config exactly refuses it at verify | Extends: an operator `net.core.rmem_max` below the VPP floor is refused while VPP is enabled |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The `vpp` config block (VPP enabled), and the iface netlink plugin start.

### Transformation Path
1. The consumer emits a sysctl default for `net.core.rmem_max` carrying its required size.
2. The sysctl component resolves the effective value and writes it, then emits `applied`.
3. The VPP component execs VPP only after the applied value meets its size.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| VPP component to sysctl component | EventBus `(sysctl, default)` and `(sysctl, applied)` | No |
| Ze to kernel | `/proc/sys/net/core/rmem_max` | No |
| Ze to VPP | `startup.conf` `linux-nl` section | No |

### Integration Points
- `internal/component/sysctl/events` event types.
- `internal/component/vpp/vpp.go` `VPPManager.Run`.

## Risks & Assumptions

### Assumptions
| Assumption | Validation method | Status |
|------------|-------------------|--------|
| VPP re-sizes its socket on every resync, so a later lower value shrinks it | Read `lcp_nl.c` resync path at v25.10 | unvalidated |
| `net.core.rmem_max` is writable from Ze's netns on the appliance | Read it after the write in a QEMU boot | unvalidated |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `vpp` config enabled | -> | VPP component sysctl floor | `.ci` under `./le test vpp` reading `net.core.rmem_max` before VPP starts (AC-1) |
| operator `sysctl net.core.rmem_max` below the floor | -> | verify refusal | `.ci` commit refusal (AC-3) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | VPP enabled on a host whose `rmem_max` is 4194304 | Before VPP is exec'd, `net.core.rmem_max` reads the size Ze writes into `nl-rx-buffer-size`, and VPP's linux-cp socket reports that receive buffer. Test: a VPP functional test reads the sysctl at the exec point and the socket size from VPP | None |
| AC-2 | The size in `startup.conf` and the floor | Both come from one declaration, so changing one changes the other. Test: a unit test reads both from the startup.conf builder and the floor request | None |
| AC-3 | Operator config sets `net.core.rmem_max` below the floor while VPP is enabled | The commit is refused at verify, naming the key, the floor, and the fix. Test: a `.ci` refusal case | None |
| AC-4 | Host `rmem_max` already above the floor | Ze leaves it unchanged. Test: a unit test over the sysctl store with a higher original value | None |
| AC-5 | iface netlink plugin start on a host whose `rmem_max` is 212992 | `rmem_max` is at least 1 MiB before the monitor subscribes, and the monitor's sockets report 1 MiB granted. Test: a Linux integration test under `./le test integration iface` | None |
| AC-6 | VPP removed from config | The sysctl component releases the VPP default, and the key returns to the next layer's value. Test: a functional test removes the `vpp` block and reads the sysctl | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| one declaration for the buffer size | `internal/component/vpp/` | AC-2 |
| larger floor wins, higher original kept | `internal/component/sysctl/` | AC-4 |

### Functional Tests
| Test | Location | Validates |
|------|----------|-----------|
| VPP start raises the ceiling first | the VPP `.ci` suite | AC-1, AC-6 |
| config below floor refused | the VPP `.ci` suite | AC-3 |
| iface monitor sockets granted 1 MiB | Linux integration, iface | AC-5 |

## Files to Modify

- `internal/component/vpp/startupconf.go` - the size from one declaration.
- `internal/component/vpp/vpp.go` - request and wait for the floor before exec.
- `internal/component/sysctl/sysctl.go` - floor precedence between defaults.
- `internal/plugins/iface/netlink/monitor_linux.go` - request the floor for the monitor sockets.
- `docs/guide/vpp.md` - the sysctl Ze sets for VPP and the refusal.

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 6 | Has a user guide page? | Yes | `docs/guide/vpp.md` |
| 12 | Internal architecture changed? | Yes | the sysctl page that describes default precedence |

## Implementation Steps

1. Write the failing tests for AC-1 to AC-6.
2. Give the sysctl store a floor rule between defaults.
3. Request the floor from the VPP component and the iface monitor, and order the VPP exec after the applied event.
4. Add the verify refusal, update the pages, run the tests.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `bound-too-small-for-its-own-burst.md` | 2026-11-08 | VPP `nl-rx-buffer-size` 64 MiB clamped by `net.core.rmem_max` |

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before the fix
- [ ] Tests PASS after the fix
- [ ] `./le test vpp`
- [ ] `./le test integration iface`
