# 1265 -- ospf-only-daemon-external-plugins

## Context

The `ospf-ldp-sync-*` / `ospf-multiaf*` / `ospfv3-vlink` functional tests failed
with `no TLS acceptor configured (hub config required for external plugins)`
(`plan/handover/21-netlink-suite-recovery.md`). Root-causing it exposed two real
bugs in how a **non-BGP daemon** (an OSPF-only `ze`, or any config without a
`bgp {}` block) handles external plugins and shutdown. Both are product defects,
not test bugs: an operator running OSPF with an external plugin and no BGP would
hit them.

## Decisions

- **A config's routing must key off ANY built-in protocol block, not just `bgp`.**
  `ProbeConfigType` (`internal/component/config/probe.go`) classified any
  `plugin {}` + no-`bgp {}` config as `ConfigTypeHub`, sending it to the
  plugin-hub Orchestrator, which runs no built-in protocol and wires no TLS
  acceptor. A config like `plugin { external observer } + ospf {}` therefore had
  built-in OSPF silently skipped and the observer killed on the missing acceptor.
  Fix: any top-level block outside the hub-native `plugin`/`env` set (e.g. `ospf`,
  `firewall`) routes to the full YANG daemon (which DOES run built-ins and wire an
  acceptor). Applied to BOTH the hierarchical scanner AND `probeSetFormat` -- the
  committed config is serialized in set format and re-probed on `ze start`/reboot,
  so fixing only the hierarchical path leaves the reboot path misrouted (caught in
  review).
- **SUPERSEDED 2026-07-27: the Orchestrator now wires an acceptor.** The sentence
  above is kept as the record of what was true then, but do not act on it. This
  fix routed AROUND the acceptor bug rather than fixing it, so a genuinely pure
  `plugin {}` / `env {}` hub config -- which SHOULD reach the Orchestrator -- still
  could not start an external plugin. That underlying defect is now fixed:
  `NewHubAcceptor` (`internal/component/plugin/acceptor.go`) owns the lifecycle
  once, `Orchestrator` creates and stops it, and `SubsystemHandler.Start` fails
  closed naming the subsystem instead of reaching the process layer's error. The
  probe narrowing here remains correct for its own reason (a non-BGP config needs
  built-in protocols), but it is now belt-and-braces, not load-bearing.
- **`request shutdown` must not require a BGP reactor.** `handleDaemonShutdown`
  (`internal/component/plugin/server/system.go`) called `RequireReactor` +
  `ctx.Reactor().Stop()`. A reactorless daemon could not be stopped by command and
  hung. The trap: `Coordinator.FullReactor` (`internal/component/plugin/
  coordinator.go`) returns the coordinator ITSELF as a no-op fallback, so
  `ctx.Reactor()` is non-nil even without BGP and `RequireReactor` passes -- the
  no-op `coordinator.Stop()` then does nothing. Fix: a reactor-independent
  `Server.shutdownFunc` (wired ungated in `cmd/ze/hub/main.go` to a non-blocking
  `sigCh <- SIGTERM`, mirroring `monitorStdinEOF`), used when the reactor is the
  `*plugin.Coordinator` fallback. Extended the same fix to `handleDaemonQuit`,
  which had the identical no-op-Stop hang.

## Consequences

- An OSPF-only (or any non-BGP) daemon that declares external plugins now runs its
  built-in protocol AND its plugins, and can be stopped/quit by command. The
  `ospf-ldp-sync` cluster (passive interfaces) passes in QEMU.
- The `*plugin.Coordinator`-fallback distinction is now load-bearing in two
  shutdown handlers: a real reactor is stopped directly (graceful BGP shutdown
  preserved), the coordinator fallback goes through `shutdownFunc`.
- **Still blocked (not this fix):** the ACTIVE-interface ospf tests
  (nbma/ptmp/point-to-point on nonexistent links) need their interface provisioned
  via `option=netns-link` under netns mode, but the ospf OBSERVER tests use
  `ze_api`, and running them uid-dropped under the netns launch mode surfaced two
  infra gaps the firewall/policy netns tests never hit: `ze_api` is not importable
  by the uid-dropped observer, and the iface backend does not load under uid-drop.
  See handover 21 -- separable test-infra work.

## Gotchas

- A no-op fallback that satisfies a nil-check is worse than a nil: `RequireReactor`
  "passing" on a reactorless daemon hid the hang. When a guard can be satisfied by
  a do-nothing stand-in, check the concrete type, not just non-nil.
- Fixing a config classifier on the live-parse path is half a fix: the persisted
  (set-format) path is a separate function probed on reboot. Mirror both.
- `ProbeConfigType` is a lightweight brace scanner; the corpus check (only 9 ospf
  configs matched `plugin + non-plugin-block + no-bgp`) is what makes the
  narrowing of the documented hub-config format safe in practice.

## Files

- `internal/component/config/probe.go`, `probe_test.go`
- `internal/component/plugin/server/system.go`, `server.go`, `system_test.go`
- `cmd/ze/hub/main.go`
