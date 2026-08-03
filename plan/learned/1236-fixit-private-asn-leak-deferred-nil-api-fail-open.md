# 1236 -- fixit-private-asn-leak-deferred-nil-api-fail-open

## Context

Deferred R1 from `1231-fixit-private-asn-leak`. The same `r.api == nil` condition was
guarded in one package (`internal/component/bgp/reactor`) three different ways: two
fail CLOSED and loudly (`filter_chain.go` `policyFilterFunc`, `peer_initial_sync.go`
default-originate), one fails OPEN and silently. The silent one pre-empted the loud ones,
so the correct guard was unreachable. Concretely, the egress/ingress policy chains returned
`accept: true` when the peer HAD export/import filters but the API server (the engine that
runs them) was nil -- sending the route UNFILTERED, leaking whatever the policy exists to
strip (e.g. RFC 6996 private ASNs). This is the zero-value trap verbatim
(`ai/rules/evidence.md`): downstream reads `accept: true` as "the filters ran and
passed." The originated path (`exportFilterForBody`) was already fixed under commit
`1fb231afb`; this slice closed the FORWARDED egress path, the INGRESS path, and the nil-api
producer.

## Decisions

- **Split the fused condition; do not patch the whole branch.** `len(filters) == 0` (no
  policy configured -- an absent precondition) and `r.api == nil` (a guard MISS with policy
  present) were `||`-fused into one permissive early return. They are opposite cases: the
  first keeps its legitimate accept; the second gets its OWN early return that suppresses the
  route AND warns. This is the whole fix, applied identically at three sites.
- **Fail closed AND speak, not "or".** The spec's AC-1 wording "deny OR log" was superseded:
  every fixed site both suppresses (`accept == false` / `suppress == true`) AND emits a
  `slog.Warn(... "-- fail-closed", "peer", addr)`, matching the two siblings exactly. The
  house answer for this condition already existed twice; the fix makes the outlier agree, it
  does not invent a third behavior.
- **One fail-closed point for the two egress paths.** `runEgressPolicyChain` (forwarded)
  only splits `len == 0`, then delegates the `r.api == nil` miss to the shared body
  `runEgressPolicyChainASN4`; `exportFilterForBody` (originated) guards `r.api` itself first
  and returns before reaching the body. Result: exactly one Warn per suppressed route, never
  double-warns, and the shared body is defense-in-depth for the originated path.
- **Ingress is the SAME, not different (AC-4).** An import filter is equally a guard
  (security/ACL policy); silently accepting unfiltered INBOUND routes when the engine is
  absent is the identical fail-open. `filter_chain.go`'s `policyFilterFunc` is
  direction-agnostic, so the sibling already covers both. Smallest self-contained fix: an
  identical one-line split.
- **Make the nil-api producer speak (AC-3).** `SetPluginServerAny` silently no-op'd on a
  failed `s.(*pluginserver.Server)` assertion, leaving `r.api` nil -- the one plausible
  producer of a nil api in a reactor that otherwise started fine. It now logs at Error naming
  the received type (`reflect.TypeOf(s)`) and returns; signature unchanged (the sole caller is
  `register.go`). "Make the miss explicit at the producer" (`evidence.md`) plus the
  now-fail-closed guards give defense in depth.

## Consequences

- No production reachability change: A-1 held (borrow mode hard-fails a nil api at
  `reactor.go`; standalone is closed by the incidental `r.mu` barrier). The fix is
  hygienic -- it removes the silent fail-open so a future ordering change cannot reopen the
  leak without a loud Warn. The undocumented `r.mu` barrier over the inverted bind/start
  ordering (`reactor.go`) is NOT fixed here (R-2, recorded, left for a follow-up).
- AC-5 fallout the spec predicted (~10 nil-api unit tests turning red) did NOT materialize:
  those `&Reactor{}` literals configure no filters AND leave `orderedEgressSteps` empty, so
  they exercise the `len == 0` accept path or skip the egress pass entirely (gated by
  `if len(a.r.orderedEgressSteps) > 0` at `reactor_api_forward.go`). None reach the
  api==nil-with-filters branch, so none turned red -- the full suite stayed green. The
  prediction assumed the literals had filters; they do not.

## Gotchas

- **The chosen logger is bare `slog.Warn`/`slog.Error`, NOT `reactorLogger()`/`fwdLogger()`.**
  The spec's AC-3 note claimed `reactorLogger` "routes through the slog default" -- it does
  not: `slogutil.Logger()` builds its own stderr handler (`createHandler`), so a test's
  `slog.SetDefault(recorder)` never sees a LazyLogger's output. Bare `slog.*` is the ONLY
  capturable path, is what the already-committed sibling `exportFilterForBody` and
  `api_sync.go` use, and is what the `warnRecorder` test helper observes. Deviation from
  the spec's literal "reactorLogger().Error" wording, justified by testability + consistency.
- **`fmt.Sprintf("%T", s)` is blocked by the no-sprintf hook** even though
  `ai/rules/performance.md` lists `%T` as ALLOWED. Use `reflect.TypeOf(s)` as a slog
  attribute instead (slog formats the reflect.Type via its Stringer; nil-safe for a nil
  interface).
- **Drive the guard from the entry point, not the helper (`evidence.md` corollary).**
  The tests call the reactor's own chain methods (`runEgressPolicyChain{,ASN4}`,
  `runIngressPolicyChain`, `exportFilterForBody`), which are the guard's entry points. Their
  real callers are verified present: `reactor_api_forward.go` (forwarded egress),
  `reactor_notify.go` (ingress), `egress_inject_filter.go` (originated). The guard is
  not uncalled.

## Files

- `internal/component/bgp/reactor/filter_ordered.go` -- split `runIngressPolicyChain`,
  `runEgressPolicyChain` (`:196`, delegates the miss), `runEgressPolicyChainASN4` (`:222`,
  the shared fail-closed point); `slog.Warn` on each miss.
- `internal/component/bgp/reactor/reactor.go` -- `SetPluginServerAny` logs on a failed type
  assertion (`slog.Error` + `reflect.TypeOf`) instead of a silent no-op.
- `internal/component/bgp/reactor/filter_ordered_test.go` (new) -- RED-first egress/ingress
  fail-closed + accept-split tests.
- `internal/component/bgp/reactor/reactor_test.go` -- `SetPluginServerAny` wrong-type log test.
- `internal/component/bgp/reactor/api_sync_test.go` -- `warnRecorder` extended to capture
  messages (for the no-"peer"-attr SetPluginServerAny assertion).
- `internal/component/bgp/reactor/egress_inject_filter{,_test}.go` -- verified from `1fb231afb`,
  unchanged (originated path already fail-closed).
