# 508 -- CLI Route Topology Graph

## Context

Ze's RIB pipeline supported `count`, `json`, and `prefix-summary` terminals for `rib show` and
`rib best` commands, but had no way to visualize AS path topology. Operators debugging route
propagation needed to mentally reconstruct the AS graph from tabular output. The LG web interface
had an AS path graph (SVG) but the logic was embedded in the `lg` package, unavailable to the CLI.

## Decisions

- Extracted shared `internal/graph/` package from LG over duplicating graph logic in the RIB plugin,
  because both the CLI terminal and the LG web view need the same graph construction and layout
- Parameterized `LayoutConfig` (SVG pixel units vs text character units) over separate layout
  implementations, keeping one algorithm for both renderers
- Used Unicode box-drawing art for CLI output over ASCII, because modern terminals handle it and
  it produces cleaner topology diagrams
- Kept LG type aliases (`Graph = graph.Graph`) over mass-renaming all LG references, minimizing
  the diff in a package not being restructured
- Stored rendered text in `PipelineMeta.JSON` following the `prefix-summary` precedent over adding
  a new field, because the pipeline dispatch already handles this path

## Consequences

- `rib show [scope] [filters...] graph` and `rib best [filters...] graph` now produce AS topology
  box-drawing output in the CLI
- `internal/graph/` is available for any future consumer needing AS path graph construction or
  text rendering (e.g., MCP tools, web API text mode)
- LG layout constants are duplicated with `graph.SVGConfig()` values -- if layout constants change,
  both must be updated until `layout_nexthop.go` is also migrated to the shared package
- MaxNodes guard (100) prevents terminal flooding on large topologies

## Gotchas

- `extractASPathFromItem` was duplicated between `rib_pipeline.go` (as `pathFilter.getASPath`)
  and `rib_topology.go` -- caught in review, deduplicated into a single shared function
- `prefix-summary` terminal was added to YANG descriptions but initially omitted from
  `docs/architecture/api/commands.md` -- caught in review, fixed
- The LG's `renderGraphText` (structured `mode aspath\nnode...\nedge...` format for web API) is
  different from `graph.RenderText` (Unicode box-drawing for terminals) -- both are needed

## Files

- `internal/graph/graph.go` -- shared graph data model, `BuildGraphFromPaths`, `DeduplicateASPath`
- `internal/graph/layout.go` -- parameterized layered layout with `SVGConfig`/`TextConfig`
- `internal/graph/text.go` -- Unicode box-drawing renderer (`RenderText`)
- `internal/component/bgp/plugins/rib/rib_topology.go` -- `graphTerminal` pipeline stage
- `internal/component/bgp/plugins/rib/rib_pipeline.go` -- `extractASPathFromItem`, terminal dispatch
- `internal/component/lg/graph.go` -- refactored to type aliases + delegate to shared package
- `internal/component/lg/layout.go` -- refactored to delegate `computeLayout` to shared package
- `test/plugin/rib-graph.ci`, `rib-graph-filtered.ci`, `rib-graph-best.ci` -- functional tests
