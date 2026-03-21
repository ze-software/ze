# 387 — RIB Pipeline Filters

## Objective

Replace separate `rib show in`/`rib show out`/`rib show best` commands with a unified `rib show [scope] [filters...] [terminal]` pipeline and `rib best [filters...] [terminal]` for best-path.

## Decisions

- Pull-based `PipelineIterator` interface: `Next() (RouteItem, bool)` + `Meta() PipelineMeta` — stages compose by wrapping upstream iterators
- `RouteItem` carries both `InEntry` (pool-backed) and `OutRoute` (parsed) to avoid source-dependent branching in pipeline
- Scope keywords (`sent`/`received`/`sent-received`) are positional (must be first arg), filters follow
- All `|` stages execute server-side — `FoldServerPipeline` in `internal/component/command/pipe.go` rewrites pipe segments into command args
- Only `no-more` and `table` stay client-side
- `count` terminal reads metadata without serializing routes — pure counting via drain loop
- `match` is a server-side cross-field search (prefix, peer, family, next-hop, origin, AS-path, communities, MED, local-pref)
- Best-path pipeline split to `rib_pipeline_best.go` for modularity (rib_pipeline.go = 889 lines without it)
- `make([]bestResult, 0)` not `var results []bestResult` — Go nil slice marshals to `null`, not `[]`

## Patterns

- Iterator pipeline with metadata: source → filter → filter → terminal, each wrapping upstream
- `validatePathPattern` at parse time prevents invalid ASNs from silently filtering to zero results
- Terminal ordering enforced at parse time: filters before terminals, at most one terminal
- `pipelineStage.apply()` dispatch avoids switch duplication between show and best pipelines

## Gotchas

- Nil slice in Go marshals to JSON `null` — always use `make([]T, 0)` for JSON array fields
- `FoldServerPipeline` must handle both `rib show` AND `rib best` prefixes — easy to miss the second
- Old commands (`rib show in`, `rib show out`) become `parsePipelineArgs` errors (unknown keyword) — tests must match production path where tokenizer sends `handleCommand("rib show", *, ["in"])`
- `matchInEntry` and `matchOutRoute` must check the same fields or cross-field match will have blind spots per direction

## Files

- `internal/component/bgp/plugins/rib/rib_pipeline.go` — core pipeline (sources, filters, terminals, builder)
- `internal/component/bgp/plugins/rib/rib_pipeline_best.go` — best-path pipeline
- `internal/component/bgp/plugins/rib/rib_pipeline_test.go` — 31 tests covering all stages
- `internal/component/bgp/plugins/rib/rib_commands.go` — handleCommand dispatch (dead code removed)
- `internal/component/command/pipe.go` — FoldServerPipeline for CLI pipe rewriting (handles rib show + rib best)
- `cmd/ze/cli/main.go` — tab completion for pipeline keywords (uses `command.CompletePipe()` for general, `ribShowPipeSuggestions` for rib show/best)
- `test/plugin/rib-pipe-filter.ci` — functional test with content assertions
- `internal/component/bgp/plugins/rib/schema/ze-rib-api.yang` — unified show RPC + best/best-status RPCs
