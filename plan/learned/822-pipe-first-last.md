# 822 -- pipe-first-last

## Context

Ze's pipe framework supported count, match, table, json, yaml, resolve, origin, ndjson, and log, but had no way to take only the first or last N items from command output. Any bounded view required the full result to be generated and serialized, then discarded. The goal was to add `| first N` and `| last N` as generic pipe operators with an optimization path for server-side early termination, plus a `"pipe"` metadata dict in JSON output recording which data-shaping modifiers were active.

## Decisions

- Followed the `count` dual-path pattern (generic pipe + command-specific filter with FoldFilters promotion) over a RIB-only implementation, because every `show` command should support `| first N` even without explicit registration.
- Made `first`/`last` filter-class pipeline stages over terminal stages, because filter stages compose naturally with any terminal (`| first 100 | count` returns 100).
- Position in pipeline is user-controlled over enforced ordering, consistent with how all other filters work.
- Positive integer only (no 0, no negative) over 0-as-unbounded sentinel, because `| first 0` has no useful semantics.
- Non-array input passes through unchanged over erroring, because server-side promotion avoids this case for RIB and client-side fallback on multi-key maps has no sensible semantics.
- Pipe metadata as a typed dict (`"pipe": {"first": 100, "received": true}`) over a flat string list, because structured access preserves types without consumer parsing.
- Display-only pipes (json, table, text, yaml, resolve, origin, log, no-more) excluded from metadata over including all, because they don't change what data is present.
- `"pipe"` key absent when no modifiers active over always-present empty dict, for backward compatibility.
- `FoldFilters` return type changed to 3-tuple `(string, []pipeOp, map[string]any)` over side channel, because the 6 callers are all in `pipe.go` and explicit return is cleanest.

## Consequences

- Any `show` command now supports `| first N` and `| last N` client-side without registration.
- Commands that register `first`/`last` as pipe filters (RIB is the first) get server-side early termination.
- `first` stops the iterator early (real performance win); `last` must drain fully (saves only serialization cost). This asymmetry is inherent.
- All `ProcessPipes*` callers now receive and forward a metadata map. New callers must follow the 3-tuple pattern.
- Table and text renderers skip the `"pipe"` key via `pipeMetaKey` constant in `tableSortedKeys` and `renderList`.
- The `"pipe"` metadata dict establishes a precedent: future data-shaping operators should add entries following the same typed-value convention (bool for no-arg, int for numeric, string for string).

## Gotchas

- `collectPipeMeta` must be called on the original ops before `FoldFilters` folds them, because after folding, promoted ops are gone from the ops slice. The metadata records both folded and remaining ops.
- `ApplyPipes` injects metadata before format ops run (table/json/yaml), not after. If injected after, the format op wouldn't see the key to skip it.
- `renderList` builds keys from a union set across all array items, independent of `tableSortedKeys`. Both needed the `pipeMetaKey` skip independently.
- `last` server-side uses a sliding window via `copy(buf, buf[1:])` rather than a ring buffer. Acceptable for CLI-scale N values but would need revisiting for very large N.
- `ProcessPipesChecked` and `ProcessPipesDefaultFunc` identity paths (when all ops fold to server args) must still inject metadata. Initial implementation missed this; review caught it.
- Array outputs are wrapped as `{"data": [...], "pipe": {...}}` when metadata is present. The table/text renderers handle this by unwrapping the data key (stripping the pipe key first) in `renderValue` before dispatching to the list/record renderer. `unwrapSingleKeyArray` (used by `| json`) naturally skips 2-key maps, so the wrapped form is preserved in JSON output.

## Files

- `internal/component/command/pipe.go` -- pipeFirst/pipeLast enums, knownPipeOps, ParsePipe, FoldFilters (3-tuple return + metadata), ApplyPipes (metadata injection), ValidatePipes, applyFirst/applyLast, collectPipeMeta, injectPipeMeta, isFormatOp, truncateItems, sliceN, applyFirstLines, applyLastLines
- `internal/component/command/pipe_table.go` -- pipeMetaKey constant, tableSortedKeys and renderList skip "pipe" key
- `internal/component/command/completer.go` -- first/last in PipeOperators
- `internal/component/command/pipe_test.go` -- TestParsePipeFirst, TestParsePipeLast, TestApplyFirst, TestApplyLast, TestApplyFirstUnderCount, TestApplyFirstNonArray, TestApplyFirstSingleKeyWrapper, TestFirstValidation, TestFoldFiltersFirst, TestFoldFiltersFirstNotRegistered, TestPipeMetadataInjected, TestPipeMetadataAbsentWhenNoPipes, TestPipeMetadataTableSkipped
- `internal/component/bgp/plugins/cmd/rib/rib.go` -- first/last pipe filter registration
- `internal/component/bgp/plugins/rib/rib_pipeline.go` -- firstFilter, lastFilter types, filterKeywords
- `internal/component/bgp/plugins/rib/rib_pipeline_test.go` -- TestShowRibFirstPipeline, TestShowRibLastPipeline, TestShowRibFirstThenCount, TestApplyFirstPositionMatters
- `test/plugin/test-pipe-first-last.ci` -- functional test
- `docs/guide/command-reference.md` -- first/last in pipe operator table
- `docs/features.md` -- Pipe Output Limiting feature row
