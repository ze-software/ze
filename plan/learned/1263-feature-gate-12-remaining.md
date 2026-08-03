# 1263 -- feature-gate-12: gate the remaining compile-out candidates

## Context

The feature-gate family (learned 980-995, 1177, 1249, 1251) had gated 16 tags
(lg, ssh, web, gnmi, mcp, rest, grpc, telemetry, isis, ldp, ospf, rsvpte, vrrp,
bgp, bmp, mrt). An audit that reused `dep_audit.py`'s own edge collection
(`collect_edges` + `classify`, so the numbers match the tier gate) found ~20
more top-level subsystems matching the same criteria the earlier gates were
chosen by -- a network-facing service, an optional protocol, or an optional
backend -- still pinned into every `ze` binary. The goal: let a `ze` build be
reduced to exactly the features a deployment needs (smaller binary, smaller
attack surface), all default-on so default and appliance builds are unchanged.

20 new tags landed in 7 phases: `ze_ntp` (pattern validation), a 12-tag Group A
of manifest-lines-only plugins (flowexport, ddos, anomaly, as112, geodns,
dhcpserver, pxe=tftp+image, trafficusage, policyroute, cos, copp, mpls),
`ze_tacacs`/`ze_exabgp` (extra composition roots), `ze_bfd`, `ze_vpp`, `ze_ike`,
and `ze_l2tp`+`ze_radius`. The audit's core finding held: 15 of ~20 features
were already reachable only through generated registration and gated with
manifest lines alone. The cost was concentrated in two places -- the hub's
direct subsystem construction (l2tp/pppoe, ike) and cross-plugin convenience
imports (cos/diag -> l2tp, static/ospf -> bfd, the vpp backends).

## Decisions

- **Generator extended from per-tag to per-package dependent constraints.** The
  headline generalization. `ze_radius` MIXES an independent package
  (`component/radius`, RADIUS system auth, usable alone) with a nested one
  (`l2tp/plugins/authradius`, which needs the BNG). The old per-tag
  `parentTagOf`/`buildConstraint` (learned 1251) would have compound-gated the
  plain radius schema too. Replaced with `parentTagOfImport`/
  `constraintForImport`/`constraintGroups`: a tag's imports are split by
  per-import constraint into `all_<tag>.go` (plain) + `all_<tag>_<parent>.go`
  (dependent). Chose this over (a) keeping per-tag (wrong), (b) a
  declared-dependency manifest column (the path already encodes it), (c) putting
  authradius under `ze_l2tp` (pins radius into every BNG build). `ze_bmp`'s
  single-constraint output stays byte-identical.
- **Shared contract leaves stay UNGATED, only the machinery gates.** `bfd/api`
  (the nil-able `SetService`/`GetService` seam BGP/OSPF/static already nil-check)
  AND `bfd/packet` (its ~500-line pure-stdlib State/Diag source) stay always-on;
  `ike/dataplane` stays always-on because it is the XFRM seam OSPF's RFC 4552
  authentication programs through, independent of the IKE daemon. The gate drops
  the engine, not the contract.
- **Dependent FILES get honest not-in-this-build stubs, not silent no-ops.** cos
  dynamic RADIUS-CoS (`//go:build ze_l2tp` -- no BNG sessions, no dynamic CoS),
  the diag l2tp captures (carved into gated helpers), and the web VPN/L2TP pages
  all have `_off` counterparts. The rule the diff added ("a stub must ANSWER
  HONESTLY") was then applied back to itself: the review caught `capture-raw
  start l2tp` returning an empty list with no message.
- **Subsystem-builder seam for hub construction.** The BNG registers engine
  SUBSYSTEMS (parse params, `eng.RegisterSubsystem`), not `Reconfigurable`
  listeners, so the service construction registry does not fit. Used a hub-local
  nil-able `bngRegister` hook (`bng_infra.go`, filled by gated
  `register_l2tp.go`) -- the ssh_infra/gnmi_infra shape, carrying only generic
  values across the boundary.

## Consequences

- **A stripped build's config now fails closed on ALL its input formats.** The
  review's one BLOCKER: the "unknown config rejected" claim held only for brace
  format; the daemon's persisted set-meta format took a lenient pre-migration
  path that PRUNED unknown fields to warnings and booted -- a stripped build
  loading a full build's committed config would silently drop gated blocks
  (tacacs/radius auth degrading to local-only) with no error. Fixed in
  `parseSetWithMigration`: a warning that survives the tree migration is silent
  config loss (a schema-known field makes the strict parse succeed and return
  early, so reaching the guard means the field is genuinely unknown -- the two
  are mutually exclusive, so no legitimate rename-migration breaks). This is a
  general config-loader hardening beyond feature gates.
- **A fourth manifest consumer is now generated.** `.github/workflows/codeql.yml`
  joined `.golangci.yml`, `gokrazy/ze/config.json`, and `docs/guide/quickstart.md`
  as feature_tags.go outputs -- caught by `TestCodeQLBuildUsesShippedTags`, which
  would otherwise have excluded 20 tags' code from SAST.
- A netlink-only build drops vendored govpp (~46k LOC); a BGP-only edge router
  drops the whole 30k-LOC L2TP BNG. Proven by one consolidated nm test.

## Gotchas

- **A-3 broken going in.** `bfd/api` was documented as "a leaf package with no
  runtime dependencies" (registry.go), but it imports `bfd/packet` (State/Diag
  re-exports) and `component/plugin`. Recovery was cheap because packet is
  pure-stdlib: leave BOTH ungated. Lesson: verify a "leaf" claim by reading the
  import block, not the doc comment.
- **Test-binary lanes hide mixed-build coverage.** `build_tag_l2tp_*_test.go`
  carry compound `ze_l2tp && ze_radius` / `!ze_l2tp && !ze_radius` constraints,
  so an l2tp-only or radius-only build compiled NEITHER file -- both advertised
  mixed builds had zero coverage. A generator-split regression would ship them
  broken with both CI lanes green. Fixed with an nm matrix that builds the mixed
  tag sets explicitly (the binaries carry their own `-tags`, so the test file's
  own constraint only controls WHICH unit pass runs it -- constrain it to run
  once, like gate11).
- **GNU make 3.81 MAKEFLAGS.** `make ze-verify ZE_VERIFY_LOG=tmp/x.log` (the
  invocation the bash-output rule prints) writes the override as the first
  MAKEFLAGS word with no `--` separator on macOS's system make; `makeDryRun` read
  the 't' in "tmp" as a `-t` dry-run flag and refused the run. A flags word never
  contains '='. Filed as HOOK-FRICTION F14.
- **Regenerate the indexes before verify, not after.** The stale
  `ai/CODE-TO-DOCS.md`/`ai/DOCS-TO-CODE.md` and a digest line-number anchor
  (`all.go` -> `:119` as gated imports left `all.go`) failed five verify
  stages; `make ze-doc-index` + `ze-discovery-index` + `generate` clear them.

## Files

- `feature-gates.txt` (+~55 lines, 20 tags); `scripts/codegen/plugin_imports.go`
  (per-package constraints), `feature_tags.go` (codeql target); `scripts/status/verify_run.go` (makeDryRun).
- `cmd/ze/hub/{bng_infra,register_l2tp,register_ike}.go`, `cmd/ze/dispatch_{tacacs,exabgp,l2tp}.go`,
  `internal/component/aaa/all/all_ze_{tacacs,radius}.go`, `config/yang/cli/tree_bfd.go`.
- Stubs/splits: `internal/component/web/page_{l2tp,vpn_ipsec}_off.go` + `page_workbench_generic.go`,
  `internal/plugins/cos/handler_off.go`, `internal/plugins/diag/cmd/capture_{l2tp,raw_l2tp}{,_off}.go`,
  `internal/plugins/static/backend_vpp_off_linux.go`, `internal/component/ike/dataplane/register_vpp.go`.
- Fail-closed fix: `internal/component/config/loader.go` + `loader_unknown_test.go`.
- Tests: `cmd/ze/hub/build_tag_{ntp,bfd,ike,vpp,l2tp,gate12*}_*.go`; `ike/dataplane/register_{vpp,novpp}_test.go`.
- Docs/rules: `docs/features.md` (18 rows), `docs/guide/plugins.md`, `ai/rules/plugins.md`.
