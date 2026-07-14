# 1120 -- payload-predicate-waits

## Context
`.ci` functional tests eliminated fixed `time.sleep` in two layers: Layer 1 was the
already-shipped `request quiesce` barrier; this spec is Layer 2 -- waiting until an
*observed payload matches a predicate*. Before it, observers either slept-then-asserted
once, or hand-rolled poll loops using `wait_for_event` merely as a backoff. The goal was
symmetric predicate waits on both surfaces: a Python observer SDK (`ze_api.py`) and a
declarative first-class `.ci` engine-step grammar, proven by converting two real tests
and lowering the sleep ratchet, plus discovery wiring. Full migration of the remaining
~450 sleeps was explicitly not part of this spec.

## Decisions
- Three Python primitives over two: `wait_until` (pure predicate poll), `dispatch_until`
  (result predicate), `wait_for_event(predicate)`. `wait_until` is required because
  forked-route-install-kernel polls KERNEL state, which is neither a dispatch nor an
  event. `dispatch_until_done` became a thin wrapper (no dead duplicate loop).
- Module-level `wait_until`/`dispatch_until` are the single implementations; `API.*`
  methods delegate, so both `from ze_api import wait_until` and `api.wait_until(...)`
  work without duplication.
- Go grammar is one predicate-kind-per-step (`Match` enum: ""=contains, matches, absent,
  json) over a multi-needle single step -- keeps parse simple, matches existing chained
  `expect=output` usage. `contains`→`Match==""` + `omitempty` keeps engine-steps.json
  byte-identical for existing steps (no back-compat serialization concern).
- `matches=` regex compiled at PARSE time (fail the test immediately) over compile-in-loop
  (would hang to timeout on a bad regex).
- `json=path=value` splits path/value at the FIRST `=`, value compared as a stringified
  leaf, over typed/numeric compare -- simplest declarative form; string compare covers
  prefixes/IPs/counts. `absent=`/`json=` are `expect=output` only (re-dispatched query);
  `expect=stream` supports `contains`/`matches` only.
- AC-4's functional test uses an **echo-mode ze-peer** (exabgp pattern): the observer
  announces an UPDATE, the echo peer reflects it, the daemon delivers a received-update
  event the predicate matches. Chosen over peer STATE events, which are NOT delivered to
  an external observer here (`receive [ state ]` and `request subscribe bgp event state`
  both yielded `events=0`).

## Consequences
- `.ci` authors now have a payload-predicate vocabulary on both surfaces; prefer it over
  `time.sleep`+assert. Documented in ci-format.md "Engine Steps" and functional-tests.md
  "Payload-predicate waits".
- `absent=` is non-vacuous only by convention: it must follow a step that made the
  substring present, else it passes instantly (false green). Documented (R-1).
- The ci-sleep ratchet only counts `time.sleep(` in `test/**/*.ci` (not `ze_api.py`), and
  is DORMANT until a `.ci` file changes -- so the committed baseline can silently drift
  above the true count and only bites the next `.ci`-touching change.
- Exported symbols in a file you touch are re-checked by `ze-validate` even if pre-existing;
  touching engine_steps.go surfaced 3 unwired exports that had to be unexported/named.

## Gotchas
- **Stale sleep baseline:** committed `test/.ci-sleep-baseline` was 456 but the true HEAD
  count was 462. AC-9's "456→454" was arithmetic on a false premise; correct value is 460
  (462−2). Always compute the true count with the ratchet's own `rglob` logic, not a shell
  `grep 'time.sleep('` (a raw `.` is a regex wildcard and double-glob `git ls-files`
  double-counts). Raising a baseline needs explicit user approval even when "correcting."
- **External observers do NOT receive peer state events** via `receive [ state ]` or
  `request subscribe bgp event state` in a single-daemon+ze-peer test (`events=0`). Use the
  echo-peer reflected-UPDATE pattern to get a deterministic delivered event instead.
- **`// test-relax:` requires the literal `//`** (regex `//[ \t]*test-relax:`), so in a
  `.ci` (Python `#` comments) write `# // test-relax: ...`. The hook flags any `.ci` line
  reduction as test-weakening; a sleep→predicate conversion is "replaced coverage" and
  needs this annotation.
- **Session-id divergence (macOS):** the shell hook lib falls back to `claude-session-fallback`
  but the Python hook's `session_id()` returns the claude pid (stable) -- the two disagree,
  so spec/Go-write gates block until pid-keyed marker symlinks point at the canonical
  `*-claude-session-fallback` markers (the pattern already present in tmp/session/).
- Pre-existing known-red unrelated to this change: full `make ze-lint` fails on
  route_refresh `*_test.go` mocks missing `DrainPeerSync`; scope verification to changed
  packages per git-safety.

## Files
- `internal/test/runner/engine_steps.go` -- Match/Path fields, generalized parse, predicate exec + json path walker
- `internal/test/runner/engine_steps_test.go` -- parse + exec + boundary unit tests
- `internal/test/runner/runner_exec.go`, `internal/test/cli/cmd_engine_steps.go` -- ze-validate wiring fixes (rename, type name at use site)
- `test/scripts/ze_api.py` -- wait_until, dispatch_until, dispatch_until_done wrapper, wait_for_event(predicate)
- `test/plugin/fib-recursive.ci`, `forked-route-install-kernel.ci` -- proof conversions
- `test/plugin/engine-steps-predicates.ci`, `event-predicate-wait.ci` -- new functional tests
- `test/.ci-sleep-baseline` -- 456 → 460
- `docs/architecture/testing/ci-format.md`, `docs/functional-tests.md`, `ai/rules/testing.md`, `ai/INDEX.md` -- discovery + docs
