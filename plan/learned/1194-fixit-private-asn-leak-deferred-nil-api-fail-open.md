# Learned: fixit-private-asn-leak-deferred-nil-api-fail-open (egress_inject_filter slice)

**Date:** 2026-07-19
**Spec:** plan/spec-fixit-private-asn-leak-deferred-nil-api-fail-open.md
**Scope of THIS work:** the `egress_inject_filter.go` slice only (originated/injected
egress path). The spec's other sites (`filter_ordered.go:196,222,139`,
`reactor.go:576` SetPluginServerAny) were assigned to sibling agents on the shared
tree and are NOT touched here.

## The bug (this slice)

`exportFilterForBody` (internal/component/bgp/reactor/egress_inject_filter.go) fused
three unrelated conditions into ONE permissive early return:

```go
if facts == nil || len(facts.exportFilters) == 0 || r.api == nil { return false, nil }
```

`facts == nil` (peer not established) and empty filters are legitimate accepts, but
`r.api == nil` while the peer HAS export filters is a guard MISS. Returning
`(false, nil)` = accept = the route goes to the wire UNFILTERED and SILENTLY. This is
the classic fail-open the package already rejects in two siblings
(`filter_chain.go:368-371` policyFilterFunc: Warn + PolicyReject;
`peer_initial_sync.go` default-originate: Warn + fail-closed).

## The fix

Split the fused condition. `facts == nil || len(facts.exportFilters) == 0` keeps its
zero-cost accept. A separate `if r.api == nil` (reached only when facts present AND
filters configured) now `slog.Warn`s and returns `(true, nil)` = suppress. The early
return happens BEFORE `runEgressPolicyChainASN4`, so this function is self-contained
and does not lean on the (out-of-scope) downstream guard at `filter_ordered.go:222`.

## Non-obvious decision: slog.Warn, not reactorLogger().Warn

The spec's test-feasibility note said to mirror `filter_chain.go`'s
`reactorLogger().Warn` and capture it via `slog.SetDefault(warnRecorder)`. That does
NOT work: `reactorLogger` is `slogutil.LazyLogger("bgp.reactor")`
(internal/core/slogutil/slogutil.go:397), a `sync.Once`-cached logger bound to its own
handler built from env/config — NOT the slog default. A WARN through it escapes to
stderr and the recorder captures nothing (observed empirically). The package's ACTUAL
tested fail-closed-miss precedent is `slog.Warn` (api_sync.go:202, asserted by
`TestSignalPeerAPIReadyUnknownPeerWarns`). So the guard uses `slog.Warn`, which is
production-established in this same package (api_sync.go:107,110,169,171,202) and makes
the "or say something" assertion real. Behavior matches the sibling (WARN naming the
peer + suppress); only the emit mechanism differs. Recorded in DECISION.md.

**Trap for the next agent:** `reactorLogger()` warns are NOT capturable via
`slog.SetDefault`. If you need to assert a reactor warn in a test, the code must use
`slog.Warn` (or you must capture the subsystem handler another way).

## Tests (RED-first, mutation-proven)

New file `egress_inject_filter_test.go`, driving the guard from its entry point
`exportFilterForBody`:
- `TestExportFilterForBodyNilAPIWithExportFiltersSuppressesAndWarns` — asserts BOTH
  `suppress == true` AND `warnedPeers` contains the peer. Mutation proof: restoring the
  old fused `|| r.api == nil` guard makes it RED with "Should be true" (route accepted).
- `TestExportFilterForBodyNoExportFiltersAccepts` — empty filters still `(false,nil)`,
  no warn (AC-2).
- `TestExportFilterForBodyNotEstablishedAccepts` — nil facts still `(false,nil)`, no
  warn (not-established is an absent precondition, not a miss).

Helpers: `peerWithExportFilters` stores a `&peerForwardFacts{}` directly via
`peer.fwdFacts.Store` (in-package); `captureWarnPeers` reuses the `warnRecorder` from
api_sync_test.go.

## Environment note

Full `make ze-lint-changed` failed on `no space left on device` (root fs 99% full)
compiling the UNRELATED `internal/plugins/trafficusage` package — not a code issue.
Scoped `golangci-lint run ./internal/component/bgp/reactor/` = 0 issues; `gofmt` clean;
full reactor package `go test` passes.

## Numbering

Picked 1194 under contention (counter at 1183; 1190/1191 taken; siblings likely grab
1192/1193). Run `python3 scripts/dev/learned_numbers.py --fix` at drain to renumber.
