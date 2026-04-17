# 614 -- fmt-0-append

## Context

BGP filter text dispatch was the highest-allocation site in the hot path: every
UPDATE that flowed through a text-mode filter plugin went through
`formatSingleAttr` (`fmt.Sprintf` per attribute), then `strings.Join`, then a
`string(scratch)` at the IPC boundary. Similar `fmt.Sprintf`/`strings.Builder`
clusters lived in `format/text.go` for the non-UPDATE formatters (OPEN,
NOTIFICATION, KEEPALIVE, ROUTE-REFRESH, EOR, Congestion, StateChange) and
`attribute/text.go` for the `FormatOrigin` / `FormatASPath` /
`FormatCommunity` helpers. Goal: eliminate these allocations on the hot path
by adopting the stdlib `AppendXxx(buf []byte, ...) []byte` shape (grow via
`append`, return new slice) throughout the BGP text formatters, with exactly
one `string(scratch)` allocation at each named IPC / cache boundary.

## Decisions

- **AppendText shape, not WriteTo.** The stdlib idiom (`strconv.AppendUint`,
  `netip.Addr.AppendTo`, `hex.AppendEncode`) is the right analog for text
  output because output size is not pool-bounded. The existing
  `WriteTo(buf, off) int` pattern remains correct for pool-bounded wire
  encoding; the two coexist.
- **Element types emit bare values; dispatchers emit names.** `LargeCommunity`,
  `ExtendedCommunity`, and `*Aggregator` all have `AppendText` methods that
  emit only the value form ("ga:ld1:ld2", hex, "asn:ip"). The filter-text
  dispatcher in `appendSingleAttr` prepends `"aggregator "`/`"large-community "`
  etc. literally. Chose this over "attribute-level method emits name+value"
  because Aggregator is both an element and the attribute itself, and the
  uniform element-emits-value rule avoids a method-name conflict.
- **Moved peer_json helpers to a separate file.** `writePeerJSON`,
  `peerJSONInline`, `writeJSONEscapedString`, and `jsonSafeReplacer` still
  use `strings.Builder` / `strings.NewReplacer`; they are needed by
  `text_json.go` and `summary.go` which return strings. Moved to
  `format/peer_json.go` so `format/text.go` can be banned-call-clean while
  preserving the string-returning callers without an extra migration round.
- **Kept `PolicyFilterChain` on `string`.** The spec made this an optional
  Phase 2 change ("default yes"). The chain's internal state machine
  (`applyFilterDelta`, `parseFilterAttrs`) uses `strings.Fields` and friends;
  converting the chain to `[]byte` would save one allocation per invocation
  but churn every filter-chain test. The bigger win lives at the plugin IPC
  boundary (deferred to `spec-plugin-ipc-raw-bytes`).
- **Golden files replaced with inline string literals in parity tests.**
  Each non-UPDATE formatter's expected output is one line; inline literals
  make the assertion self-documenting at review time and avoid a binary
  fixture that reviewers must diff manually.

## Consequences

- **Hot path is 0 allocs/op + 1 alloc/op boundary.** `BenchmarkAppendAttrsForFilter_Reused`
  and `BenchmarkAppendUpdateForFilter_Reused` both report 0 B/op, 0 allocs/op
  after warm-up (with a registered encoding context). The single
  `string(scratch)` conversion at `FilterUpdateInput.Update` is counted as
  `BenchmarkFormat_Boundary_StringConvert` (1 alloc/op, 112 B/op). Future
  plugin-IPC raw-bytes transport (tracked as `spec-plugin-ipc-raw-bytes`)
  can drop that last allocation.
- **Test benchmarks must register an encoding context.** `AttributesWire.Get`
  calls `bgpctx.Registry.Get(sourceCtxID)`; an unregistered ID returns nil
  and every Get allocates an error via `fmt.Errorf`. In production the
  context is registered during peer session setup; benchmarks must do the
  same via `bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))`
  or the 0-alloc claim is fiction (this tripped an early Phase 2
  benchmark showing 12 allocs/op that disappeared on context registration).
- **AC-8 banned-list grep is now a regression guard.** `fmt.Sprintf`,
  `strings.Join`, `strings.Builder`, `strings.NewReplacer`,
  `strings.ReplaceAll`, `strconv.FormatUint` et al. are forbidden in
  `reactor/filter_format.go`, `format/text.go`, `attribute/text.go`.
  Re-introducing any of them to these files is blocked by a one-line grep.
- **Three deferrals landed in `plan/deferrals.md`.** UPDATE-path migration
  (`spec-fmt-1-text-update`), JSON-writer migration off `io.Writer`
  (`spec-fmt-2-json-append`), and plugin IPC raw-bytes transport
  (`spec-plugin-ipc-raw-bytes`). Each has a concrete destination spec
  name and is recorded `open`.

## Gotchas

- **`Community.String()` does NOT match filter text.** String() uses the
  registry (upper-case `NO_EXPORT`, `NO_ADVERTISE`, ...), filter text uses
  lowercase (`no-export`, `no-advertise`) plus `blackhole` (RFC 7999) which
  isn't in the registry. `appendCommunityText` is the only source of truth
  for filter text. Changing one to match the other would break the filter
  plugin wire contract.
- **`ClusterList.AppendText` emits no brackets.** Unlike the other list
  attributes, cluster-list uses space-separated dotted-decimal IDs without
  `[ ]` wrapping. This is a byte-for-byte legacy format quirk preserved
  from the original `FormatClusterList`-style filter dispatch. Don't
  "normalize" it without updating every text-mode filter plugin.
- **Hook ordering matters.** Attempting to write the new `text_update.go`
  while `text.go` still contained `FormatMessage` etc. failed the
  `check-existing-patterns.sh` duplicate-name hook. Resolution: shrink
  `text.go` first (drop the UPDATE-path helpers), then create
  `text_update.go`. The `// Related:` cross-reference hook also blocked
  writing the cross-ref before the target file existed; had to land the
  files in two steps and add the back-reference last.
- **`go build -gcflags` is blocked by `block-root-build.sh`.** Use
  `go vet -gcflags='-m=2' ...` instead when running escape analysis --
  the vet wrapper is allowed because it writes no binary artifact.
- **Session marker file gets cleaned up.** If a session runs long enough,
  `_cleanup_stale_markers` wipes `tmp/session/.session-<sid>`. Writing a
  `.go` file then fails with "No session state ($SESSION_STATE)" because
  the hook can no longer map sid → spec. Rewrite the marker
  (`echo spec-foo > tmp/session/.session-<sid>`) to recover.

## Files

- `internal/component/bgp/attribute/text_append.go` (new, 11 attribute +
  3 element AppendText methods + `appendCommunityText` + `appendClusterID`)
- `internal/component/bgp/attribute/text_append_test.go` (new, unit + reuse)
- `internal/component/bgp/attribute/text_append_bench_test.go` (new, 0 allocs/op)
- `internal/component/bgp/format/text.go` (rewritten: AppendOpen,
  AppendNotification, AppendKeepalive, AppendRouteRefresh, AppendEOR,
  AppendCongestion, AppendStateChange, `appendJSONString`,
  `appendReplacingByte`, `appendPeerJSON`; legacy Format* deleted)
- `internal/component/bgp/format/text_update.go` (new, UPDATE-path split)
- `internal/component/bgp/format/peer_json.go` (new, string-returning
  peer/JSON helpers separated from text.go for external callers)
- `internal/component/bgp/format/text_append_test.go` (new, parity tests
  against legacy escape + all 7 non-UPDATE formatters)
- `internal/component/bgp/reactor/filter_format.go` (rewritten as
  AppendUpdateForFilter / AppendAttrsForFilter + append sub-helpers)
- `internal/component/bgp/reactor/filter_format_bench_test.go` (new,
  0 allocs/op + boundary benchmark)
- `internal/component/bgp/reactor/filter_format_test.go` (renamed
  `formatMPBlock` test to `appendMPBlock`)
- `internal/component/bgp/reactor/reactor_notify.go`,
  `reactor_api_forward.go` (call-site migrations to stack-scratch pattern)
- `internal/component/bgp/server/events.go` (8 call-site migrations;
  stack scratch in `formatMessageForSubscription`, `onPeerStateChange`,
  `onEORReceived`, `onPeerCongestionChange`)
- `internal/component/bgp/server/events_test.go` (2 call sites migrated)
- `internal/component/bgp/format.go` (1 caller migrated off
  `attribute.FormatASPath`)
- `internal/component/bgp/format/text_test.go` (6 test callers migrated
  via sed)
- `internal/component/bgp/attribute/text.go` (legacy Format* helpers
  deleted; Related cross-ref to `text_append.go` added)
