# 1270 -- fixit-load-dependent-functional-failures

## Context

A `make ze-verify` run surfaced 17 functional-test failures "under load". The user framed them as
flaky tests to make load-resistant. Investigation (reproduced with `scripts/dev/stress-repro.py`,
32 burners / 16 cores) showed they are **two distinct problems**, and the "make them load-resistant"
framing held for only one class. Half were **real BGP concurrency bugs the tests correctly catch** --
widening their timeouts would have hidden data corruption (`no-parking`, `no-workarounds`). The goal
became: fix the genuinely-flaky harness class at the deadline, fix the product-race class at source,
and never widen a test that catches a real bug.

## Decisions

- **Split the failures by evidence, not by assumption.** Class B (harness) fails on a *clock*:
  `http-check-failed`, bind/startup timeouts, all under CPU oversubscription. Class A fails on
  *wrong wire bytes*: 372 `AS_PATH [65002 ...]` (local AS 65000 rewritten to peer AS 65002), 378 a
  duplicate announce, 394 a spurious withdraw, 345 a spurious OPEN NOTIFICATION. A reproduction that
  yields valid-but-wrong bytes is a real bug; a port-collision or a slow clock cannot fabricate that.
- **Class B: scale the fixed INNER readiness gates by the SAME `withParallelHeadroom` the OUTER
  per-test budget already uses.** The runner widened the outer budget 3x under parallelism
  (`ParallelTimeoutHeadroom`) and engine-steps, but left the inner gates fixed, so a contended run
  blew them first while the outer budget sat unused. Added `parallelFactor()` as the single source of
  truth for both duration and count budgets, and widened EVERY structurally-identical gate: both
  ze-peer bind barriers, both `daemon.ready` waits, the `await=stderr` fence, the `ze bgp decode`
  fork, and the HTTP wait/retry/client budgets. (The first pass missed the siblings; a review round
  caught them -- the classic `before-writing-code.md` sibling-call-site audit.)
- **345: keep reject-duplicate-router-id as the DEFAULT; add `bgp/session/allow-shared-router-id`
  to opt into ACCEPTING a shared BGP Identifier** (owner decision, over unconditionally relaxing per
  RFC 6286 §2.2, which would delete a deliberate 342-line-tested feature). The Go flag is
  `AllowSharedRouterID bool` -- **zero value false = ENFORCE**, so the strict behavior is fail-closed
  and no `reactor.Config` construction site can silently disable it. The 345 test (one AS112 speaker
  over v4+v6, same router-id) opts in; that also removes its load-dependent check-then-act race
  (`checkRouterIDConflict` only rejected once the *other* peer reached Established).
- **Forward-path family (372/378/394/351) carved into its own spec** (`spec-fixit-bgp-egress-rail-divergence`)
  and deferred (owner decision). Verified root cause: adj-rib-in's peer-up REPLAY re-injects the
  stored raw route as an `update hex … add` announce, which prepends the local AS *before* the write
  gate and runs *only* `facts.exportFilters` (not the in-process role/OTC/community filters) -- the
  reverse order and an incomplete filter set vs the live-forward rail. Fixing it (route replay through
  the forward rail) is a NEW `RelayStoredRoute` reactor primitive + RPC/SDK, not a redirect (the
  `recentUpdates` cache is never populated for a late-joining peer), so it is spec-sized on its own.

## Consequences

- Contended `make ze-verify` runs stop flaking on fixed inner deadlines; serial/single-test debug
  runs keep the tight authored deadlines (headroom is identity when `concurrency <= 1`), so real
  slowdowns still surface fast. Any future fixed inner readiness deadline in `internal/test/runner/`
  should route through `withParallelHeadroom` / `parallelFactor`.
- Operators can peer one speaker over both address families with a shared router-id via
  `bgp/session/allow-shared-router-id true`; the default still flags accidental duplicate router-ids.
- **Known limitations (recorded, not fixed here):** (1) RFC 6286 §2.2's SHOULD-reject on a *zero* or
  *self* BGP Identifier is unimplemented in ze -- PRE-EXISTING, not a regression; the opt-in does not
  newly accept a lone zero/self id. (2) The default-path check-then-act race remains for a config that
  does NOT opt in but genuinely has duplicate router-ids (nondeterministic which peer is rejected) --
  acceptable for a misconfig. (3) ze does NOT implement RFC 6286 §2.3 collision detection; a
  `rfc/short/rfc6286.md` summary + `docs/features/rfc-status.md` row remain follow-up (RFC 6286
  support is partial).

## Gotchas

- **stress-repro reuses prebuilt binaries.** `ensure_binaries` only builds when a binary is *missing*,
  and `bin/ze-test`'s Makefile dep is `*.go` only -- a `.yang`-only edit does NOT trigger a rebuild.
  A first "reproduced" verdict for the 345 fix was the stale 19:28 `bin/ze` rejecting the new leaf
  (`unknown field in session`). Rebuild (`make ze` / `make bin/ze-test`) before trusting any
  `ZE_TEST_NO_BUILD` load verdict after a source change.
- **The `-race` binary blows a different fixed deadline.** Under `-race` (~10x slower) 372 hit a
  plugin-server `stage timeout waiting_for=Capability`, not the double-apply -- and `-race` reported
  NO data race, confirming the Class A bugs are LOGICAL ordering races, not memory races.
- **The stress reproducer is unfaithful for single-test HTTP suites.** Running 8 concurrent copies of
  one web test collides on the fixed per-test port (`bind: address already in use`) -- a reproducer
  artifact, not the real `http-check-failed`. Rely on static analysis + the outer/inner budget
  relationship there.
- **A source anchor added to docs makes `ai/CODE-TO-DOCS.md` stale** -> `make ze-doc-index` before
  `make ze-doc-test`.
- **The `.ci` test-weakening guard counts non-comment config lines and its relax token needs `//`**
  (invalid `.ci` comment syntax) -- so a config-leaf *placement* move that reduces `.ci` lines is
  blocked; keeping the leaf under `bgp/session` avoided the fight (the coupling to the mandatory
  `asn/local` sibling is theoretical -- ze's YANG walker is lenient).

## Files

- Phase 1: `internal/test/runner/{runner_exec_util,runner_exec,runner_validate,await_stderr}.go`,
  `runner_exec_util_test.go` (`TestParallelFactor`).
- 345: `internal/component/bgp/reactor/{reactor,peer,routerid_unique}.go` (+ `routerid_unique_test.go`),
  `internal/component/bgp/config/loader_create.go`, `internal/component/bgp/yang/ze-bgp-conf.yang`,
  `test/plugin/redistribute-as112-announce.ci`, `docs/guide/configuration.md`, `ai/CODE-TO-DOCS.md`.
- Deferred: `plan/spec-fixit-bgp-egress-rail-divergence.md`, `plan/deferrals/fixit-load-dependent-functional-failures.md`.
