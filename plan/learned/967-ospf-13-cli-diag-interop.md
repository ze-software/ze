# 967 - OSPFv2 CLI / diag / web / interop (spec-ospf-13)

## Context

Completed `plan/spec-ospf-13-cli-diag-interop.md`: the presentation, observability and
interop layer over the working OSPFv2 engine (ospf-1..12). `show ospf` process summary
+ neighbor/interface/database/route/border-routers/spf views + six per-LS-type database
subviews + three `clear ospf` actions; RFC 6987 `max-metric router-lsa` config with
stub-router reflection; `ze_ospf_*` metric-namespace verification; two config-sanity
doctor checks; `/ospf` + `/ospf/database` web views with SSE; and six FRR `ospfd` interop
scenarios. Nothing here originates protocol state -- it renders snapshots, exports config,
surfaces checks, and drives end-to-end interop.

## Decisions

- **CLI the IS-IS/LDP way, reusing existing snapshots.** The engine already exposed
  neighbor/interface/database/route/border-routers/spf snapshots via `OnExecuteCommand`.
  ospf-13 added the `ze-show:ospf-*` / `ze-clear:ospf-*` builtin RPC proxies
  (`cmd_show.go`, `RegisterRPCs` + `forwardToOSPF`->`ForwardToPlugin`) and the
  `ze-ospf-cmd.yang` command tree binding them; the proxy's `PluginCommand` must match the
  engine `CommandDecl` name EXACTLY or the builtin conflicts with / shadows the route.
- **One transport-agnostic web view, two thin adapters.** IS-IS and OSPF web surfaces are
  the same read-only neighbor+database SSE views over dispatched snapshots, differing only
  in command/title/path/element-id. Extracted the generic `snapshotHandlers`
  (`snapshot_views.go`), the SSE loop (`sse_snapshot.go`), and the escaped page shell
  (`page_snapshot.go`); refactored IS-IS onto them and added OSPF as a parallel thin
  adapter. The two parallel adapters are a legitimate `//nolint:dupl` case.
- **Doctor reuses the engine's own parse path.** `checkOSPFConfigSanity` marshals
  `ospfTree.ToMap()` to `{"ospf":{...}}` JSON and runs `parseOSPFConfig` with the live
  `systemRouterIDSource` -- the SAME wrapped shape `ExtractConfigSubtree` feeds the runtime
  -- so the doctor verdict cannot diverge from what the engine resolves.
- **A per-chain `extended-sequence`-style reflection for max-metric**: the `max-metric
  router-lsa` YANG (always / on-startup / on-shutdown seconds, range 0..86400) was NOT in
  the schema (the parser read a tree that validation rejected); ospf-13 added it. The
  process summary reflects `always` as the active stub-router state.
- **`ze_ospf_*` namespace verified with a recording `metrics.Registry`** (embeds
  `NopRegistry`, captures names): assert every engine-registered metric is `ze_ospf_*`
  (never bare `ospf_*`) + the canonical series are present, without a live scrape.

## Gotchas

- **Reuse before reinvent, again.** The snapshots, the dispatch path, the raw-socket
  doctor check (ospf-3), the IS-IS web pattern, and the IS-IS FRR interop harness all
  existed -- ospf-13 is wiring + one generic web extraction + config leaves, not new logic.
- **A shared YANG command-tree container must use the EXACT description of the existing
  one.** `show > ip` is contributed by multiple modules; ze-ospf-cmd.yang's `ip` container
  had to copy "IP neighbors (ARP/ND) and kernel routing table" verbatim or the command-tree
  merge warned (`node=ip description mismatch`).
- **`dupl` attributes a two-file clone to the package line.** A function-level
  `//nolint:dupl` did NOT suppress the parallel-adapter clone; it had to go on the
  `package web` line (where golangci-lint reports the issue).
- **Config JSON in tests needs the nested list key + explicit key leaf**:
  `areas:{area:{"0.0.0.0":{area-id:"0.0.0.0"}}}`, `interfaces:{interface:{eth0:{area:"0.0.0.0"}}}`
  -- a flat `areas:{"0.0.0.0":{}}` parses to zero areas.
- **on-startup/on-shutdown max-metric is parsed but NOT armed anywhere** (engine drives
  origination off `RouterLSAAlways` only). The summary's `stub-router.active` is therefore
  faithful (always-only); the timed windows are an unimplemented engine-wide feature
  (ospf-7 territory), not an ospf-13 divergence. Documented, not a bug.
- **SSE frame-splitting defense-in-depth.** A valid-JSON payload with an embedded newline
  would split the `data:` frame; `sse_snapshot.go` continues each newline as a fresh
  `data:` line. Not reachable via SDK-marshaled compact snapshots, but a security-streaming
  path worth hardening (review NOTE).
- **Daemon CLI / FRR interop tests skip darwin.** `ospf-show.ci` and the six
  `ospf-*-frr` scenarios need Linux + root/raw-sockets (+ Docker/FRR for interop); they are
  Linux-CI-verified, exactly like their IS-IS siblings. `ospf-doctor-config-sanity.ci`
  (just `ze explain`) runs on darwin.

## Verification anchors

- Unit: `TestEngineProcessSummary`, `TestDatabaseSubviewFilter`/`MapCovers6Types`,
  `TestTableResetAll`, `TestComputerClearSPFLog`, `TestEngineClearMethods`,
  `TestOSPFDoctorConfigSanity`, `TestOSPFMaxMetricConfig`, `TestOSPFMetricsNamespaced`,
  `TestOSPF*`/`TestSnapshotView*`/`TestSSESnapshotNewlineSafety` (web). `-race` green.
- Functional (darwin): `ospf-doctor-config-sanity.ci`. Linux-CI: `ospf-show.ci`, six
  `ospf-*-frr` interop scenarios (FRROSPF helper, `ospfd=yes`).
- Review gate: independent adversarial review (concurrency + SSE + XSS + doctor) -- 0
  BLOCKER, 0 ISSUE, 3 documented NOTES.

## Files

None recorded.
