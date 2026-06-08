# 810 -- Command-Owned Pipe Filters and Show Grammar

## Context

Ze's CLI had two problems: (1) BGP operational commands used inconsistent grammar
(noun-first `peer list`, `bgp rib show received`, mixed `show bgp peer`), and
(2) command-specific pipe filters like `received`, `advertised`, `peer`, `family`,
`prefix`, `path`, `community` were either hardcoded in generic pipe code or encoded
as positional arguments inside command strings. The generic pipe framework knew about
BGP command names, violating component isolation.

## Decisions

- Chose a registration-based `PipeFilter` type in `command/pipe_filter.go` over YANG
  annotations on pipe segments, because filters need runtime metadata (TakesArg, Leading,
  Description) that YANG leaf definitions do not naturally carry.
- Chose longest-prefix command matching in `lookupPipeFilters` over exact-match, so
  `show bgp rib status` (registered with no filters) takes precedence over `show bgp rib`
  (registered with RIB filters). This prevents status commands from inheriting route filters.
- Chose `FoldFilters` to rewrite command-specific pipes into server-side command arguments
  before dispatch, keeping `ApplyPipes` purely client-side. This means the handler receives
  filter selections before output generation starts.
- Chose `Leading` flag on filters (`received`, `advertised`) to control argument ordering:
  leading filters are prepended before other server args, matching the existing RIB pipeline
  parser expectation of scope keywords first.
- Unified user-facing grammar under `show` verb: `show summary`, `show peer list`,
  `show peer detail <selector>`, `show peer capabilities <selector>`,
  `show peer statistics <selector>`, `show peer history <selector>`, `show bgp rib`,
  `show bgp rib best`. Action keyword precedes identifier per cli-grammar rule.
- Renamed `sent` to `advertised` as the user-facing filter name (internal code may still
  use "sent" in RIB storage).

## Consequences

- Generic pipe code (`pipe.go`, `completer.go`) contains zero BGP command names.
- Completion after `|` merges global pipe suggestions with the resolved command's
  registered filters via `filterSuggestions()`.
- Validation rejects unknown command-specific filters with a message listing valid ones.
- RIB route commands receive `received`/`advertised`/`peer`/`family`/`prefix`/`path`/
  `community` as server-side args, allowing the iterator pipeline to apply them during
  generation rather than post-hoc.
- New commands automatically get pipe filter support by calling `RegisterPipeFilters`
  in their `init()`, following the same registration pattern as RPC handlers.

## Gotchas

- The `lookupPipeFilters` longest-prefix match means registering a parent command with
  filters and a child command with none requires explicit empty registration for the child
  (`RegisterPipeFilters([]string{"show bgp rib status"})`), otherwise the child inherits
  the parent's filters.
- `FoldFilters` must run before `ValidatePipes` because it converts `pipeUnknown` ops
  into either server args or `pipeInvalid` errors. Calling `ValidatePipes` on unfolded
  ops would reject valid command-specific filters as unknown.
- Web CLI and SSH CLI must share the same pipe classification path (`ProcessPipesDetectLog`),
  otherwise filters work in one surface but not the other. This was fixed in commit
  `1e932a529`.

## Files

- `internal/component/command/pipe_filter.go` -- PipeFilter type, registry, RegisterPipeFilters, lookupPipeFilters, filterSuggestions
- `internal/component/command/pipe.go` -- FoldFilters, ParsePipe, ApplyPipes (no BGP knowledge)
- `internal/component/command/completer.go` -- CompletePipe merges global + command-specific suggestions
- `internal/component/bgp/plugins/cmd/rib/rib.go` -- RIB command registration with pipe filters
- `internal/component/bgp/plugins/cmd/peer/peer.go` -- show peer list/detail/teardown/pause/resume handlers
- `internal/component/bgp/plugins/cmd/peer/summary.go` -- show summary, show peer capabilities/statistics
- `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` -- peer command YANG tree
- `internal/component/bgp/plugins/cmd/rib/yang/ze-rib-cmd.yang` -- RIB command YANG tree
- `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` -- show command aliases
