# 937 -- isis-13-cli-diag-interop

## Context
Spec `isis-13-cli-diag-interop` is the presentation / verification / interop layer
over the native IS-IS engine the siblings (isis-1..isis-12) built. It originates no
protocol state: it renders the engine's read-only snapshots (adjacency, LSDB,
hostname, SPF, route) as `show isis ...` CLI commands and two web pages, adds two
runtime `clear isis ...` actions, asserts the full canonical Prometheus metric set,
registers two config-sanity doctor codes, and ships the six FRR `isisd` interop
scenarios that are the umbrella's goal-validation evidence. It was an integration
spec layered on completed siblings, not a from-scratch build. The implementation is
DONE and verified at the unit/build level on darwin; the on-the-wire interop
validation is pending Linux/QEMU execution (see "Interop validation pending").

## Decisions
- **Show/clear are dispatcher proxies, the LDP model.** `cmd_show.go` registers
  ten central-namespace RPCs (`ze-show:isis-*`, `ze-clear:isis-*`) via
  `pluginserver.RegisterRPCs`, each carrying a `PluginCommand` so the engine can
  claim the same command name without a builtin conflict, and each handler forwards
  through `Dispatcher().ForwardToPlugin` (NOT `Dispatch` -- re-dispatching would
  re-match the same builtin and recurse to a stack overflow). The engine
  `OnExecuteCommand` switch (register.go) is the single authority that turns each
  fixed command string into a snapshot. No protocol logic lives in the proxy.
- **No `ze-isis-api.yang`.** Both verbs bind into the CENTRAL `ze-show:` /
  `ze-clear:` namespaces; the owner ships only `yang/ze-isis-cmd.yang` with two
  separate augment-style roots (a `show` container and a SEPARATE `clear`
  container, never nested), matching `command_ownership.go`'s requirement that the
  owner declares the command tokens. A per-component api module is only needed when
  a component coins its OWN RPC namespace (BFD/l2tp do; IS-IS does not).
- **Doctor codes live in the component, not core.** The spec's Files-to-Modify said
  register `doctor-isis-net-missing` / `doctor-isis-system-id-mismatch` in
  `internal/core/diagnostic/codes.go`. The implementation instead registers them in
  `internal/component/isis/codes.go` (data) + `register.go` (the
  `diagnostic.Register` call), with an explicit comment in core `codes.go` noting
  they are deliberately NOT in the central slice. This is a deliberate improvement
  toward plugin-self-containment: deleting the IS-IS component removes its codes.
  `doctor-isis-raw-socket` stays owned by isis-3 (`transport/`) and is only
  surfaced -- one code, one owner.
- **Metrics: assert, do not register.** isis-13 owns NO `ze_isis_*` series. Every
  series is registered by its producing subsystem (isis-3 transport, isis-5..11
  engine). `metrics_test.go` wires a `recordingRegistry` through every owner and
  asserts the EXACT umbrella "Metrics (canonical)" name+label set, that none is a
  bare `isis_*`, and that no unexpected `ze_isis_*` series leaks in -- a two-way
  guard against drift.
- **Web reuses the L2TP dispatcher + SSE-ticker pattern.** `ISISHandlers` holds a
  `CommandDispatcher` and renders `show isis neighbor` / `show isis database` JSON
  into a dependency-light HTML shell with a per-connection SSE ticker that closes
  on client disconnect (no goroutine leak).

## Consequences
- The show layer is removable with the component: command tokens (owner YANG),
  handlers (cmd_show.go), engine dispatch (register.go), doctor codes (codes.go),
  and web views all live under `internal/component/isis/` (+ web). The two
  self-containment guards (central show/clear schemas) assert the tokens never
  drift back into the central schema.
- AC-7 (pipe completeness) is satisfied structurally: each command returns JSON
  through the dispatcher, so the generic `ApplyPipes`/`ProcessPipes` machinery
  applies. The `isis-show.ci` dispatches commands directly (it asserts the
  command-surface, AC-1..AC-6/AC-8) rather than re-asserting the shared pipe
  machinery per noun.

## Gotchas / Traps
- **Web routes are not mounted into the live server mux.** `ISISHandlers` and its
  `HandleISIS*` methods exist and are unit-tested (`handler_isis_test.go`: 5 tests
  pass, including SSE emit+close), but nothing in `handler.go`/`server.go` calls
  `WebServer.HandleFunc("/isis", ...)`. This is NOT an isis regression: the
  existing `L2TPHandlers` follow the identical pattern (struct + methods + tests,
  never instantiated in production, no `/l2tp` mux mount in handler.go). So AC-9's
  handlers + SSE + page rendering are implemented and tested, but the route is not
  yet reachable on the running web server; mounting `/isis` + `/isis/database`
  (and a workbench tab) is the same follow-up the L2TP web surface still needs.
  Recorded as a Partial against AC-9, not "done".
- **`ForwardToPlugin`, never `Dispatch`, in the proxy handlers.** The builtin RPC
  and the engine command share the command string; re-dispatching recurses. The
  file header comment in `cmd_show.go` calls this out explicitly -- keep it.
- **`isis-show.ci` runs against a passive-only engine.** A `passive true` interface
  advertises reachability but opens no raw circuit, so the engine starts cleanly
  WITHOUT `CAP_NET_RAW` on darwin and still originates its own LSP -- which is what
  makes `show isis database` non-empty in a .ci. Live adjacency/SPF over the wire
  needs raw L2 (AF_PACKET) and is the QEMU/interop job, not the .ci.
- **`ze doctor` / `ze explain` are one-shot but the .ci runner treats every
  foreground `ze` as a daemon** and waits ~5s per command for a readiness file the
  one-shot never writes. `isis-doctor.ci` sets a generous 60s timeout on the first
  command so four sequential explain/doctor calls fit the budget.

## Interop validation pending (Linux/QEMU)
The six FRR `isisd` scenarios are WRITTEN in full (each with `ze.conf`, `frr.conf`,
`check.py`; `test/interop/daemons` is `isisd=yes`; `interop.py` ships the `FRRISIS`
runner helper with `wait_adjacency`/`adjacency_up`/`has_database_lsp`). They were
NOT executed: this session ran on a darwin host with no Docker/QEMU and no raw L2,
so adjacency-over-the-wire, route convergence, DIS-on-LAN, dual-stack, HMAC auth,
link-down reconvergence, and IS-IS<->BGP redistribution are proven only by the
unit/functional layer here. The interop ACs (AC-13..AC-19) are "scenario written;
execution pending Linux/QEMU". The Linux-only raw-socket doctor firing path
(AC-11) is likewise a QEMU integration test, surfaced (not executed) on darwin.

## Verification (this session)
- `go build ./...` (darwin): exit 0 (whole tree).
- `go test -race ./internal/component/isis/...`: all 12 isis packages PASS.
- Named spec-13 unit tests PASS: `TestISISShowClearRPCsRegistered`,
  `TestISISProxyNilDispatcher`, `TestISISShowProxyArgsRejected`,
  `TestISISDoctorConfigSanity{NETMissing,Mismatch,Clean,Absent}`,
  `TestISISDoctorChecksRegistered`, `TestISISRawSocketCodeRegistered`,
  `TestISISMetricsRegistered`, `TestISISShow{Hostname,Interface,SPFLog}Render`,
  `TestISISClear{Adjacencies,Counters}`, `TestISISEngineDatabaseSnapshot`.
- Self-containment guards PASS: `TestISISCmdSchemaOwns{ShowISIS,ClearISIS}`
  (owner half), `TestShowSchemaHasNoMigratedOwnerCommands` +
  `TestClearOwnerRemovalLeavesNoResidue` (central-guard half).
- Web PASS: `TestISIS{NeighborsJSON,DatabaseJSON,NeighborsHTML,NoDispatch,SSEEmitsAndCloses}`.

## Files
- `internal/plugins/isis/cmd_show.go` (+test): ten `ze-show:isis-*` /
  `ze-clear:isis-*` proxy RPCs; `forwardToISIS` rejects extra args, nil-dispatcher.
- `internal/plugins/isis/show.go` (+test): hostname/interface/spf-log snapshots,
  clearAdjacencies/clearCounters, hostname sanitizer.
- `internal/plugins/isis/register.go`: `OnExecuteCommand` switch (the engine-side
  authority for all ten commands) + `registerISISDoctor()` + diagnostic-code reg.
- `internal/plugins/isis/doctor.go` (+test): config-sanity check (net-missing,
  system-id-mismatch), no-op when IS-IS absent (R-4).
- `internal/plugins/isis/codes.go`: the two config-sanity code metadata (owned
  here, not in core), with a guard comment added to core `diagnostic/codes.go`.
- `internal/plugins/isis/metrics_test.go`: canonical `ze_isis_*` set assertion.
- `internal/plugins/isis/yang/ze-isis-cmd.yang` (+cmd_schema_test.go): owner
  command tree, separate show/clear roots.
- `internal/component/cmd/show/yang/self_containment_test.go`,
  `internal/component/cmd/clear/yang/self_containment_test.go`: central-guard tokens.
- `internal/component/web/handler_isis.go`, `page_isis.go` (+handler_isis_test.go):
  neighbor/database pages + SSE (route mounting still pending, see Gotchas).
- `test/isis/isis-show.ci`, `test/isis/isis-doctor.ci`: functional surface tests.
- `test/interop/scenarios/isis-{p2p,lan-dis,dualstack,auth,convergence,redist}-frr/`
  (ze.conf/frr.conf/check.py each); `test/interop/daemons` (`isisd=yes`);
  `test/interop/interop.py` (`FRRISIS` helper).
- Docs: `docs/guide/isis.md`, `docs/architecture/wire/isis.md`,
  `docs/features.md`, `docs/comparison.md`, `docs/guide/command-reference.md`,
  `docs/plugin-development/metrics.md`.
