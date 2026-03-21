# 384 — RIB Pipeline Commands

## Objective

Unify RIB show commands (`rib show in`, `rib show out`, `rib show best`) into a single `rib show` command with pipeline-based iterator filtering.

## Decisions

- Single `rib show` command replaces separate in/out/best variants
- Scope (received/sent/sent-received) is a positional keyword, not a separate command; default is sent-received (both)
- Server-side pipeline stages: source → filter → terminal, using `PipelineIterator` interface with `Next()` and `Meta()`
- Filter stages: family, cidr, path, community, match (all keyword + value)
- Terminal stages: count (drains without serializing), json (serializes all items)
- Client-side pipes (table, text, yaml, no-more) remain in `command.ApplyPipes`
- `FoldServerPipeline` bridges client pipe syntax to server args — `rib show | path 65001 | count` becomes `rib show path 65001 count`
- Old `rib_show_filter.go` with positional heuristics deleted entirely

## Patterns

- `PipelineIterator` follows Ze's iterator convention: `Next() (RouteItem, bool)` — consumer pulls one at a time
- `RouteItem` carries either `*storage.RouteEntry` (adj-rib-in) or `*Route` (adj-rib-out) — never both
- `countTerminal` drains upstream counting items without serializing — true zero-copy count
- `showPipeline()` holds `RLock` during full pipeline execution — stages must be fast

## Gotchas

- Old positional parsing (family starts with letter, prefix starts with digit) was fragile — explicit keywords are unambiguous
- YANG schema simplified to single `rib show` RPC; all test expectations updated for unified command names
- `inboundSource` buffers items per-peer because `PeerRIB.Iterate` holds a read lock during the callback — can't yield one at a time from inside the callback

## Files

- Created: `internal/component/bgp/plugins/rib/rib_pipeline.go`, `rib_pipeline_test.go`
- Deleted: `internal/component/bgp/plugins/rib/rib_show_filter.go`
- Modified: `rib.go`, `rib_commands.go`, `rib_commands_test.go`, `rib_test.go`, `rib_gr_test.go`, `protocol_test.go`, `ze-rib-api.yang`
- Modified: `test/plugin/rib-show-filter.ci`, `test/reload/reload-add-peer.ci`, `internal/test/runner/runner_validate.go`
