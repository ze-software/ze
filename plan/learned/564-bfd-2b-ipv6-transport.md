# 564 -- bfd-2b-ipv6-transport

## Context

Stage 2 (`plan/learned/559-bfd-2-transport-hardening.md`) hardened
the BFD UDP transport for IPv4: GTSM via `IP_TTL=255` outbound and
`IP_RECVTTL` cmsg on receive, plus `SO_BINDTODEVICE` for VRFs and
single-hop interface binding. IPv6 was explicitly deferred because
the existing `UDP` struct binds one family and the refactor to
handle both was non-trivial. This follow-up closes the gap with a
new `transport.Dual` wrapper, an `applySocketOptionsV6` Linux helper
that sets `IPV6_RECVHOPLIMIT` / `IPV6_UNICAST_HOPS=255`, and a YANG
`bfd { bind-v6 true }` opt-in. Operators who add an IPv6 pinned
session get a paired IPv4 + IPv6 socket set per `(vrf, mode)` loop;
deployments that stay IPv4-only keep the old single-socket path.

## Decisions

- **`transport.Dual` wraps two inner `UDP` instances** rather than
  extending `UDP` itself to juggle both families. Keeping the v4
  and v6 instances identical (same struct, same Start/Stop/Send/RX
  methods, same family check inside Start) means the existing
  unit tests and the existing error-handling code continue to
  work without changes. `Dual.Start` brings both sides up in
  sequence with a rollback on the second failure; `Dual.Stop`
  closes both; `Dual.Send` dispatches by destination address
  family; `Dual.RX` merges both channels behind a parent waiter
  goroutine that closes the merged channel once both readers
  exit.

- **`UDP.Start` branches on `u.Bind.Addr().Is6()`** to pick
  `"udp4"` / `"udp6"` network strings and
  `applySocketOptions` / `applySocketOptionsV6`. The branch is
  the only place the IPv6 path differs from v4; everything else
  (the raw-conn Control callback, ListenPacket wrapper, readLoop,
  rxPool) is family-agnostic because `net.UDPConn.ReadMsgUDPAddrPort`
  works on both.

- **`parseReceivedTTL` handles both `IP_TTL` and
  `IPV6_HOPLIMIT` cmsgs.** The readLoop is shared between the
  v4 and v6 inner sockets (via the Dual merger), so a single
  parse function feeds both branches. The helper's level/type
  test accepts either family; downstream code (`passesTTLGate`)
  treats both as a uint8 hop count.

- **`bind-v6` is a top-level `bfd { }` leaf, not per-profile.**
  IPv6 dual-bind is a per-process-per-vrf decision: either the
  plugin opens a second socket per loop or it does not. Making
  the leaf per-profile would force the plugin to inspect the
  profile before constructing the loop, and an operator who
  wanted partial v6 would still need one socket per family per
  loop. Top-level is cleaner.

- **`r.cfg = cfg` before `loopFor` runs.** `runtimeState.loopFor`
  reads `r.cfg.bindV6` when it builds the transport, so the
  config has to be visible BEFORE the first loop is created.
  The previous `applyPinned` ordering set `r.cfg = cfg` at the
  END of the function; Stage 2b moves the assignment to the
  top. Both access paths are guarded by `runtimeStateGuard` so
  the early publish is race-safe.

- **No new `.Device` handling for Dual.** The two inner UDPs
  share the same `Device` value. `SO_BINDTODEVICE` applies
  identically to v4 and v6 sockets on Linux. When a non-default
  VRF binds the loop, both sockets bind to the VRF device.

## Consequences

- **IPv6 BFD works now.** A single-hop pinned `single-hop-session
  2001:db8::9 { profile fast }` with `bfd { bind-v6 true }` at
  the top level creates both sockets, and outgoing packets go
  out the v6 socket with `IPV6_UNICAST_HOPS=255`. `passesTTLGate`
  enforces the v6 hop-limit=255 GTSM check via
  `parseReceivedTTL` reading the `IPV6_HOPLIMIT` cmsg.

- **Opt-in keeps v4-only deployments unchanged.** Operators who
  do not set `bind-v6 true` see the same single-socket
  behaviour as Stage 2. The `newTransport` helper returns a
  bare `*UDP` in that case; the Dual wrapper is skipped.

- **Interop with FRR bfdd for v6 is a follow-up.** Stage 3b
  (`spec-bfd-3b-frr-interop`) already tracks the FRR interop
  scenario; the v6 path is implicitly covered once that scaffold
  lands because FRR's `bfdd` supports v6 natively.

- **Mixed v4/v6 pinned sessions share one Dual per (vrf, mode).**
  A profile with both a v4 and a v6 single-hop session in the
  same VRF runs over one Dual instance, not two. The existing
  `resolveLoopDevices` logic still picks the binding interface
  from pinned sessions; a conflict (v4 wants eth0, v6 wants
  eth1) degrades to `device=""` the same way as two v4
  sessions would.

- **Gauge metrics (`ze_bfd_sessions`) stay accurate.** The Dual
  merge means every inbound from either family looks identical
  to the engine, so the sessions gauge, transitions counter,
  detection-expired counter, tx/rx counters, and auth failures
  counter all increment once per packet regardless of which
  family saw it. No new metric families were needed for IPv6.

## Gotchas

- **`Dual.Start` rollback ordering.** If the v6 inner fails to
  bind (permissions, no v6 kernel support, port conflict on a
  host that has another BFD implementation running), the v4
  inner must be stopped before Start returns. The rollback
  case is tested via the .ci test only indirectly; a unit test
  for the error path would be a useful follow-up.

- **`Dual.merged` channel close timing.** The two inner readers
  push onto `merged` via `spawnMerge`. `close(merged)` waits
  on the WaitGroup of inner readers, not the Dual's own
  wg. Separating `closeWG` keeps the lifecycle goroutines
  distinct.

- **`runtimeState.cfg` is read by loopFor WITHOUT reacquiring
  `runtimeStateGuard`.** The guard is already held by the
  caller (applyPinned or pluginService), so nested re-locking
  would deadlock. The `r.cfg = cfg` assignment at the top of
  applyPinned relies on this invariant.

- **`netip.AddrPort` formatting differs v4 vs v6.** The format
  string `"%s:%d"` produces `[2001:db8::9]:3784` only when the
  address is wrapped in brackets; `UDP.Start` now branches on
  `isV6` to pick `"[%s]:%d"` for IPv6 binds, avoiding a parse
  error inside `net.ListenPacket`.

- **`IPV6_HOPLIMIT` vs `IPV6_RECVHOPLIMIT`.** The setsockopt uses
  the `RECV` form (enabling cmsg delivery); the cmsg itself
  arrives as `IPV6_HOPLIMIT`. Easy to flip the two.

## Files

- `internal/plugins/bfd/transport/dual.go` (new) -- Dual wrapper,
  Wrap helper, Start/Stop/Send/RX methods.
- `internal/plugins/bfd/transport/udp.go` -- Start branches on
  `isV6`; Bind address string formatting branches.
- `internal/plugins/bfd/transport/udp_linux.go` --
  `applySocketOptionsV6` (IPV6_RECVHOPLIMIT + UNICAST_HOPS +
  SO_BINDTODEVICE); `parseReceivedTTL` accepts IPV6_HOPLIMIT.
- `internal/plugins/bfd/transport/udp_other.go` --
  `applySocketOptionsV6` stub for non-Linux builds.
- `internal/plugins/bfd/bfd.go` -- `newTransport` dispatches to
  Dual or bare UDP; `newUDPTransport6` alongside the existing
  v4 helper; `applyPinned` publishes cfg early; loopFor logs
  `ipv6` field.
- `internal/plugins/bfd/config.go` -- `bindV6` field and parse.
- `internal/plugins/bfd/schema/ze-bfd-conf.yang` -- top-level
  `bind-v6` leaf.
- `test/plugin/bfd-ipv6-dual-bind.ci` (new) -- v6 pinned session
  lifecycle test.
- `plan/deferrals.md` -- row closed.
