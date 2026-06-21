# 953 -- iface-resolve: dispatch-layer translation, dhcp/traffic, and the no-direct-resolution guard

## Context

Final unit of the interface logical-name effort (`spec-iface-resolve-0-umbrella`),
covering umbrella sub-spec 5 (iface-internal: the dispatch ops, iface/dhcp, provision)
and sub-spec 7's guard. Sub-specs 1-4 and 6 had migrated the model, the resolver, IS-IS,
and the routing/protocol consumers (see [[952-iface-resolve-consumers]]); what remained
was the iface component's own by-name ops, the dhcp/traffic backends, and a check that
keeps new direct kernel resolution out of the tree (AC-U1). With this unit the umbrella's
runtime migration is complete: an operator-facing logical interface name honors the
os-name / mac-match selectors through every path that mutates or queries a device.

## Decisions

- **One translation seam covers all ~22 iface ops: the dispatch layer, not the backend.**
  The audit flagged ~16 netlink-backend primitives (`manage/bridge/mirror/xfrm/tunnel/show`)
  as sites to migrate. They are NOT migrated: the backend is the single kernel owner and
  stays raw. Instead `internal/component/iface/dispatch.go` grows a best-effort `resolveOS`
  that every by-name dispatch function (`SetMTU`, `AddAddress`, `SetMACAddress`, `SetAdminUp/Down`,
  `Add/Remove/ListRoutes`, `ReplaceAddressWithLifetime`, `Bridge*`, `*Mirror`, `GetXFRMInfo`,
  `GetStats`, `GetMACAddress`, `LinkSpeedDuplex`, `ResetCounters`, `DeleteInterface`,
  `CreateVLAN`-parent) calls before the backend. One seam, every op.
- **`GetInterface` / `ListInterfaces` / `Create*` stay raw.** The resolver is built ON
  `GetInterface`/`ListInterfaces` (`resolve.go` osDeviceFor / matchByMAC), so routing them
  through `resolveOS` would recurse. A created device's name IS its kernel name, so the
  Create ops translate nothing.
- **Best-effort translation: identity on failure.** `resolveOS(name)` returns `Resolve(name).OsName`
  on success, else the name unchanged. This makes the change **strictly no-worse-than-today**:
  when no backend is loaded or the device is absent, the backend call produces exactly the
  error it would have without translation; only a known binding redirects. `""` (ResetCounters'
  "all interfaces") never resolves and passes through.
- **config_apply is untouched -- and that is what makes the dispatch seam safe.** The iface
  component's own apply engine calls the backend handle directly (`b.SetMTU(e.Name)`,
  `b.AddAddress(osName)`), never the dispatch functions, with config names that ARE os names
  by construction (Phase 1 creates by name; Phase 3 reconciles by `desiredState()` os names).
  So translating the dispatch functions changes only the EXTERNAL callers (RPC handlers, dhcp,
  web, healthcheck, l2tp) -- exactly the operator-facing paths that need it.
- **GetStats / ResetCounters key the counter baseline on the resolved os device.** Both resolve
  first and key `baselines` by the os name, matching `ListInterfaces`/`GetInterface` (which key
  by the backend's os `Name`). A clear-then-read cycle through a selector agrees on the key
  instead of missing the baseline. Identical for the common `name == os` case.
- **DHCP gets its address/route ops fixed for free.** `dhcp_v4/v6` already call the dispatch
  funcs (`iface.AddRoute`/`ReplaceAddressWithLifetime`/...) with the logical name, so the
  dispatch translation covers them. Only the per-iteration existence check needed an explicit
  change: `net.InterfaceByName(c.ifaceName)` -> `iface.Resolve(c.ifaceName)`, feeding
  `binding.OsName` to `nclient4/6.New` (which binds the client socket and itself does a
  `net.InterfaceByName` -- it needs the os device). Dropped the now-unused `net` import.
- **Traffic resolves in the backend LOGIC methods, not the tc kernel adapter.** `tcOps.linkByName`
  stays raw. `Apply` / `RestoreOriginal` / `ListQdiscs` call a best-effort `resolveOSName` so
  the snapshot map (keyed by `link.Attrs().Name` = os in `ensureSnapshot`) and the restore
  lookup agree on the os key even when the caller passes a logical name. Resolving inside the
  adapter instead would store snapshots by os but look them up by logical -> silent restore
  miss. Unit tests use a mock `tcOps`, so they are unaffected (best-effort identity with no
  backend).
- **The guard is a Go checker mirroring `command_ownership.go`.** `scripts/checks/iface_resolution.go`
  (`//go:build ignore`, package main) walks `cmd/`, `internal/`, and `pkg/` for `net.InterfaceByName(`
  / `.LinkByName(` / `SIOCGIFINDEX` CALLS (trailing `(` plus a `stripComment` pass exclude prose
  and `var x = net.InterfaceByName` function-value references) in non-test files and fails for any
  outside a documented allowlist map (directory entries match by prefix, file entries by exact path).
  Wired three ways: `make ze-iface-resolution-check`, both `_ze-verify-impl` / `_ze-verify-changed-impl`,
  and a smoke test (`iface_resolution_test.go`) that runs it under `go test ./scripts/checks/`. Proven
  non-vacuous: delisting provision surfaced exactly its 4 sites.
- **The dispatch translation also let a dead wrapper be removed.** The `iface.Monitor` /
  `NewMonitor` wrapper in dispatch.go had no production caller (production starts the link monitor via
  `Backend.StartMonitor` directly in register.go / config_apply.go) and existed only for the monitor
  integration tests; the `/ze-review` pass surfaced it (ze-validate "no cross-package non-test caller")
  and it was deleted, the tests calling the backend directly via a `startTestMonitor` helper.

## Consequences

- **Three umbrella-listed consumers turned out to be K-sites, not migrations** (the audit
  over-classified them, as [[952-iface-resolve-consumers]] warned about M-classifications):
  - **ppp** (sub-spec 6): `ra_linux.go` / `dhcpv6_linux.go` bind sockets to a `pppN` device
    that the PPP/L2TP server creates per session. The name is kernel-assigned, never a config
    logical name (umbrella assumption **A-5**: kernel-enumerated names stay direct; the model:
    created/point-to-point kinds are identified by their assigned name, no selector). Migrating
    would add a guaranteed-no-op resolver round-trip to per-session socket setup.
  - **provision** (sub-spec 5): `ze provision` is a one-shot PXE/DHCP bootstrap CLI with no
    iface backend loaded and no logical-name config mapping yet -- `--interface` is a raw kernel
    device. `iface.Resolve` would error.
  - **doctor** (sub-spec 7, already flagged in 952): one-shot CLI, no backend.
  All three are allowlisted with a stated reason; none is a silent scope drop.
- The allowlist is now the living record of every legitimate direct-resolution site: the
  resolver/dispatch owner, the iface + traffic kernel backends, the `fib` `LinkByName("lo")`
  literal, the post-resolution `net.InterfaceByName(binding.OsName)` in ldp/diag, and the three
  no-backend/kernel-sourced consumers above.
- `make ze-verify` (and `-changed`) now fail if any new consumer resolves the kernel directly.

## Gotchas

- **The working tree was shared and dirty with several other sessions' uncommitted work**
  (cmd/ze, ospf specs, test runner, ai/rules, docs, test/*.ci). `go test ./scripts/checks/`
  comes back RED on `TestNoOwnerAllowlistIsEnforced` -- a pre-existing command-ownership
  violation in `cmd/ze/ze_core_dispatch.go` owned by the tiers/command-ownership track, NOT
  this migration. The guard's own test (`TestNoDirectInterfaceResolution`) passes in isolation.
  Scope verification to the changed packages; the commit stages only this unit's files via
  explicit `git add -- <file>`.
- **`config_apply` uses `b.<op>` (backend handle), the dispatch funcs use bare `<Op>`.** Both
  live in package `iface`. The difference is load-bearing: only confirm-by-reading that the
  apply engine takes the `b.` path before assuming dispatch translation cannot double-process
  it.
- **Traffic snapshot keying is the subtle trap.** `ensureSnapshot` keys by `link.Attrs().Name`
  (os); `restoreOriginalLocked` looks up `snapshots[ifaceName]` BEFORE resolving. Resolve at the
  top of every public backend method (not just `linkByName`) or restore silently no-ops under a
  selector.
- **VLAN-under-renamed-parent is a known edge.** `manage.go` deletes a unit by the computed
  logical `parent.vid`; `resolveOS` falls back to that name (no kernel device of that exact name
  under an os-name-selected parent), same as today. Out of scope; the common case is identity.

## Verification

- QEMU integration (`make`-style `go test -tags integration ./internal/component/iface/...` via
  `scripts/evidence/qemu-run.py`): new `TestDispatchTranslatesLogicalToOSDevice` (SetMTU +
  SetAdminDown on a logical name land on the os device) and `TestDispatchStatsBaselineKeyedByOSDevice`,
  plus the existing resolve/address/mac integration tests (which now exercise the translated
  `SetAdminUp`/`AddAddress`/`SetMACAddress`) -- regression proof for the dispatch change.
- Guard: `make ze-iface-resolution-check` -> OK; delisting an entry caught exactly its sites
  (non-vacuous); `go test ./scripts/checks/ -run TestNoDirectInterfaceResolution` passes.
- Cross-compile (linux + darwin) clean for `iface` (cross-platform dispatch) and the linux-only
  dhcp/traffic packages; iface cross-platform unit suite green on the darwin host.

## Files

- dispatch translation: `internal/component/iface/dispatch.go`
  (+ `dispatch_resolve_integration_linux_test.go`)
- dhcp: `internal/plugins/iface/dhcp/dhcp_v4_linux.go`, `dhcp_v6_linux.go`
- traffic: `internal/plugins/traffic/netlink/backend_linux.go`
- guard: `scripts/checks/iface_resolution.go`, `scripts/checks/iface_resolution_test.go`,
  `Makefile` (target + both verify paths)

## Umbrella status

With sub-specs 1-7 complete, the umbrella's runtime migration is done: every config-sourced
interface name resolves through the shared resolver or the dispatch ops, the guard keeps it
that way, and the K-site allowlist records the deliberate exceptions. OSPF consumes the
resolver via its own specs (never migrated here, by design).
