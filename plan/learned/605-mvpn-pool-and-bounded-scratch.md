# 605 -- MVPN Pool Migration, Bounded Scratch, and UpdateBuilder Pool

## Context

Follow-up from `learned/604-update-pool.md`, expanded to close the related
correctness and tooling gaps surfaced in review. Three threads:

1. **MVPN hot-path alloc.** `buildMVPNNLRIBytes` used heap append per route
   during `BuildGroupedMVPN`'s sizing loop and `buildMPReachMVPN`'s
   aggregation.
2. **Unbounded scratch grow.** `UpdateBuilder.alloc` and `Splitter.alloc`
   would `newSize := max(len*2, end)` past RFC 8654's 65535-byte ceiling.
   A single UPDATE cannot legally exceed that bound; the "grow" path was
   masking invalid-packet bugs rather than bounding them.
3. **UpdateBuilder instance churn.** Plugin encoders, reactor static-route
   sends, and `peer_initial_sync` all called `NewUpdateBuilder` per UPDATE,
   paying a 4KB scratch init each time. No pool existed despite the
   `Splitter`/`bufpool` precedents.

## Decisions

- **`writeMVPNNLRI(buf, off, route) int` + `mvpnNLRISize(route) int`** over
  keeping `buildMVPNNLRIBytes() []byte`. Matches `update-pool`'s
  buffer-first shape; `BuildGroupedMVPN` sizes chunks via `mvpnNLRISize`
  (zero alloc), `buildMPReachMVPN` writes NLRIs directly into the
  scratch-backed MP_REACH value. `writeMVPNAddr` dedups the
  IPv4/IPv6 prefix+address write. Paired with a parity unit test
  (`TestMVPN_SizeWriteParity`) that locks `mvpnNLRISize` and `writeMVPNNLRI`
  in agreement per route-type × address-family combination.
- **Scratch grow bounded at `wire.ExtendedMaxSize` + `panic("BUG: ...")`**
  over unbounded doubling. Either the UPDATE fits (grow once, done) or the
  caller is trying to build an invalid packet (panic surfaces the bug at
  the first bad call, not downstream). Same pattern applied to
  `Splitter.alloc`. Panic prefix is allowlisted by `block-panic-error.sh`
  per `go-standards.md`.
- **`GetUpdateBuilder` / `PutUpdateBuilder` + `updateBuilderPool`** over
  `sync.Pool`-ifying internally, over requiring callers to thread a builder
  through the signature, and over leaving `NewUpdateBuilder` as the only
  entry. Follows the `GetSplitter`/`PutSplitter` precedent in
  `update_split.go`. `NewUpdateBuilder` is retained for tests and for
  callers that legitimately want a fresh instance (perf/chaos long-lived
  struct fields). Production callers — plugin encoders (`vpn`, `evpn`,
  `mup`, `labeled`, `flowspec`, `vpls`), `peer_initial_sync` (all 8 sites),
  `cmd/ze/bgp/encode` — moved to Get/Put.
- **Explicit Put over `defer Put` at in-loop sites.** `gocritic`'s
  `deferInLoop` warning is correct: three `peer_initial_sync` sites
  (static-route group, FlowSpec per-route, default-originate per-family)
  acquire a builder inside a loop. `defer` there accumulates one builder
  per iteration until function return — the opposite of pooling.
  Rewrote those three sites to call `PutUpdateBuilder` explicitly on
  each exit path (continue, break, normal completion). Non-loop sites
  keep `defer`.
- **Length-overflow panic in `writeMVPNNLRI`** over silent truncation.
  RFC 6514 Length field is 1 byte; current route types (5/6/7) max data
  is 46 bytes. The guard is unreachable today but prevents a future type
  that exceeds 255 bytes from being silently corrupted.
- **Widened hook to `*/message/update_split*`, `*/reactor/forward_build*`,
  `*/bgp/nlri/{base,inet,rd}*`** with `// pool-fallback` annotations on
  the legitimate result-owning `make([]byte, ...)` sites. Kept
  `bmp/sender.go`, `filter_community/filter.go` out of scope per the
  `2026-04-16` audit precedent — their `make` sites need full pool
  migration (caller API change), not just annotation.
- **Fixed-size buffers in `bmp/sender.go` init/term/discard paths as
  typed arrays** (`var arr [N]byte; buf := arr[:]`) over `make([]byte,
  const)`. Escape analysis confirms all three still move to heap today
  (they flow through `net.Conn.Write`/`Read`, which are interface calls
  — the slice escapes). The pattern is kept for two reasons: a
  size-constant regression fails to compile because the array form
  requires a compile-time `N`; and if the write path ever becomes a
  concrete type or a caller-provided scratch buffer, escape analysis
  can stack-allocate the same code path. No runtime win today.
- **Rewrote `RDNLRIBase.buildData` to pre-sized alloc** instead of
  `append(rd.Bytes(), data...)`. RFC 4364 guarantees RD is 8 bytes, so
  pre-sizing is trivial; this also removes an `append()` the widened
  hook would otherwise flag.

## Consequences

- MVPN UPDATE build is zero heap-alloc on the hot path for any number of
  grouped routes. Previous cost: 1 heap alloc per route (`append` on
  `data []byte`) plus per-route scratch. New cost: one scratch slice for
  the whole MP_REACH value.
- Plugin encoders, peer_initial_sync, and CLI encode now amortize the 4KB
  scratch allocation via `updateBuilderPool`. The first Get pays the init;
  subsequent cycles reuse the retained scratch. `NewUpdateBuilder` (kept
  for tests and long-lived struct fields) lives side by side following the
  `NewSplitter`/`GetSplitter` precedent.
- Invalid-UPDATE bugs in callers now panic with `BUG:` at alloc time
  instead of producing an oversized packet that peers would reject.
  WithMaxSize wrappers cannot intercept this panic (it fires before the
  post-check); that is the correct behavior because > ExtendedMaxSize is a
  caller-side invariant violation, not a recoverable error.
- Encoding-alloc hook now protects `update_split.go`, `forward_build.go`,
  and `bgp/nlri/{base,inet,rd}*` against regressions. Scope remains
  surgical per audit precedent — not widened to subsystems that still
  need proper pool migration.
- BMP session connect/disconnect init/term/discard buffers are now
  compile-time-sized typed arrays. No runtime delta today (they escape
  through net.Conn interface calls), but a wrong-size edit fails to
  compile and the code is forward-compatible with escape-analysis
  improvements or a future switch to a concrete write path.

## Gotchas

- **`writeMVPNNLRI` has no capacity check.** Callers MUST size the output
  buffer with `mvpnNLRISize(route)` first. Matches the buffer-first
  contract where `WriteTo` trusts caller capacity.
- **Pool scratch aliasing.** The `*Update` returned by Build* aliases
  `ub.scratch`. `PutUpdateBuilder` MUST NOT be called until the Update has
  been consumed (packed, serialized to TCP, handed to `SendUpdate`).
  Returning an in-use builder lets a concurrent Get overwrite the live
  bytes. Documented on `GetUpdateBuilder`.
- **`defer Put` in a loop is wrong for pooling.** The defers accumulate
  one builder per iteration until function exit — worse than not pooling
  at all. Use explicit `Put` on every exit path (continue, break, normal
  completion) in loop bodies. `gocritic`'s `deferInLoop` rule catches
  this; trust it.
- **`// pool-fallback` is meaningful**, not decorative. It marks a
  `make([]byte, ...)` whose size is bounded (by RFC limit, per-session
  init, or sync.Pool fallback copy) so the hook's `make([]byte)` rule
  doesn't false-positive. Adding it to a site whose size is actually
  unbounded hides a bug. `bgp/nlri/{base,inet,rd}` sites carry it because
  they produce result slices for JSON / test / RPC-decoder-fallback
  consumers (not wire hot path); `bmp/sender`, `filter_community/filter`
  do NOT qualify — their sites need pool migration.
- **`buildStaticRouteUpdateNew` (peer_static_routes.go) stayed on
  `NewUpdateBuilder`.** Converting to Get/Put would require a signature
  change (the function returns an Update whose scratch the caller must
  release). Session-bring-up path — one alloc per static route — is
  acceptable; pooling here is future work.
- **`internal/perf/sender.go` and `internal/chaos/peer/sender.go`
  long-lived struct fields stayed on `NewUpdateBuilder`.** Pool adds no
  value for already-long-lived holders; the cost is the one-time init,
  which they already amortize.
- **`make ze-verify` log is not safe under concurrent Claude
  sessions.** Both runs redirect to `tmp/ze-verify.log`; interleaved
  output produced a spurious bare `FAIL` summary line that couldn't be
  traced to any specific package. Targeted `go test -race ./pkg/...` +
  `golangci-lint run ./pkg/...` was the authoritative check for these
  changes.

## Files

- `internal/component/bgp/message/update_build.go` -- `GetUpdateBuilder`,
  `PutUpdateBuilder`, `updateBuilderPool`; bounded scratch grow; revised
  doc comment
- `internal/component/bgp/message/update_build_mvpn.go` -- new
  `mvpnNLRISize`, `writeMVPNNLRI`, `writeMVPNAddr`; rewrote
  `buildMPReachMVPN`; deleted `buildMVPNNLRIBytes`; length-overflow panic
- `internal/component/bgp/message/update_build_mvpn_test.go` -- new
  `TestMVPN_SizeWriteParity` (10 cases across type × address family)
- `internal/component/bgp/message/update_build_grouped.go` --
  `BuildGroupedMVPN` uses `mvpnNLRISize`
- `internal/component/bgp/message/update_split.go` -- bounded scratch
  grow + doc
- `internal/component/bgp/plugins/nlri/{vpn,evpn,mup,labeled,flowspec,
  vpls}/encode.go` -- Get/defer Put migration
- `internal/component/bgp/reactor/peer_initial_sync.go` -- Get/Put
  migration (5 non-loop `defer`, 3 in-loop explicit Put)
- `cmd/ze/bgp/encode.go` -- Get/Put migration
- `internal/component/bgp/reactor/forward_build.go` -- `// pool-fallback`
  annotations on legitimate sync.Pool fallback copies
- `internal/component/bgp/nlri/{base,inet,rd}.go` -- annotations +
  pre-sized `buildData` rewrite
- `.claude/hooks/block-encoding-alloc.sh` -- tightened existing globs,
  widened scope to `update_split`, `reactor/forward_build`, `bgp/nlri/
  {base,inet,rd}`
- `internal/component/bgp/plugins/bmp/sender.go` -- init/term/discard
  buffers rewritten as compile-time-sized typed arrays (source-level
  improvement; escape-analysis-equivalent to the prior heap `make`
  through net.Conn calls today)
