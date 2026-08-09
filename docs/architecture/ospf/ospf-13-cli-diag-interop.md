# OSPF CLI, doctor, web and interop surface

The presentation and observability layer over the OSPF engine: `show ospf`
views, `clear ospf` actions, RFC 6987 max-metric config, doctor checks, the web
views and the FRR interop scenarios. Nothing here originates protocol state.

## Decisions

- **The CLI reuses the engine snapshots and adds RPC proxies.** The engine
  already exposed neighbor, interface, database, route, border-router and SPF
  snapshots. This layer added the `ze-show:ospf-*` and `ze-clear:ospf-*` builtin
  proxies and the command-tree YANG that binds them.
  <!-- source: internal/plugins/ospf/cmd_show.go -- pluginserver.RegisterRPCs -->
  <!-- source: internal/plugins/ospf/clear.go -- clear -->
- **The proxy `PluginCommand` matches the engine command name EXACTLY.** A
  mismatch makes the builtin conflict with, or shadow, the route.
- **One transport-agnostic web view and two thin adapters.** The IS-IS and OSPF
  web surfaces are the same read-only neighbor and database views over
  dispatched snapshots. They differ in command, title, path and element id. The
  generic handlers, the SSE loop and the escaped page shell were extracted, and
  IS-IS was moved onto them.
  <!-- source: internal/component/web/snapshot_views.go -- snapshotHandlers -->
  <!-- source: internal/component/web/sse_snapshot.go -- sseSnapshotStream -->
  <!-- source: internal/component/web/page_snapshot.go -- snapshotPageHTML -->
  <!-- source: internal/component/web/handler_ospf.go -- OSPFHandlers -->
- **The doctor check reuses the engine parse path.** It marshals the config
  subtree into the same wrapped shape the runtime is fed and runs the engine
  parser with the live router-id source, so the doctor verdict cannot diverge
  from what the engine resolves.
  <!-- source: internal/plugins/ospf/doctor.go -- checkOSPFConfigSanity -->
- **The metric namespace is asserted with a recording registry.** Every
  engine-registered metric is `ze_ospf_*` and never bare `ospf_*`, proven
  without a live scrape.

## Traps

- **A shared YANG command-tree container copies the description of the existing
  one verbatim.** `show > ip` is contributed by several modules, and a differing
  description warns at merge time.
- **`dupl` attributes a two-file clone to the package line.** A function-level
  `//nolint:dupl` does not suppress the parallel-adapter clone. The directive
  goes on the `package web` line.
- **Config JSON in a test needs the nested list key and the explicit key leaf.**
  `areas:{area:{"0.0.0.0":{area-id:"0.0.0.0"}}}` parses. A flat
  `areas:{"0.0.0.0":{}}` parses to zero areas.
- **`max-metric router-lsa on-startup` and `on-shutdown` are parsed and armed
  nowhere.** Origination reads the always form only, so the summary
  `stub-router.active` field is faithful. The timed windows are an unimplemented
  engine feature.
- **SSE frames are split on newlines defensively.** A valid JSON payload holding
  an embedded newline would otherwise split the `data:` frame.
- The daemon CLI test and the FRR interop scenarios need Linux with raw sockets,
  and Docker for interop. The config-sanity doctor test runs anywhere.
