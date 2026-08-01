# 840 -- web-test-wait-settle

## Context

The web `.wb` runner (`internal/component/web/testing/runner.go`) synchronised
after every `open`/`press`/`click` with `agent-browser wait --load networkidle`.
Every finder page loads `sse-client.js`, which opens a persistent
`EventSource('/events')` (see `templates/page/layout.html`). A long-lived
connection means Playwright's networkidle can never settle, so each call burned
a fixed ~1.44s fallback. Long scenario tests pay this many times over:
`scenario-firewall-setup.wb` did ~22 such waits and ran ~66s, the slowest test
in the suite. The goal was to cut that without making the suite flaky.

## Decisions

- Replaced networkidle with an in-flight `fetch`/XHR counter, installed via an
  `AGENT_BROWSER_INIT_SCRIPTS` init script (`inflightInitJS`) that re-runs on
  every navigation. Chose instrumentation over a DOM signal because cli.js GUI
  mode submits with a plain `fetch('/cli')` then chains `htmx` refresh + a
  second `fetch('/config/changes')`; a `.htmx-request` check alone misses the
  plain fetches. Wrapping both `fetch` and `XMLHttpRequest.send` catches all of
  it.
- Deliberately left `EventSource` un-instrumented: the persistent `/events`
  connection is exactly what must not count as "in flight", or the predicate
  never goes idle.
- `WaitLoad` polls the predicate (`inflightIdleExpr`) from Go via `eval`
  (returns instantly) under a hard 5s deadline, rather than a blocking
  `wait --fn`. On deadline it proceeds; the explicit `action=wait` sleeps and
  the following expectation are the real assertion.
- 120ms debounce in the predicate bridges the microtask gap where the counter
  dips to zero between the chained `fetch -> htmx -> fetch`.

## Consequences

- firewall scenario 66s -> 26s; multi-peer 26.5s -> 17s; router 19.8s -> 12.7s;
  whole web suite faster. Full suite stayed 64/0 across three runs plus a
  networkidle A/B baseline.
- `WaitLoad` can no longer hang: a request that never settles degrades to
  "proceed after the deadline" instead of blocking until a process-kill.
- Falls back to networkidle automatically if the init script cannot be written.

## Gotchas

- `agent-browser wait --fn` IGNORES `--timeout`. A predicate that never becomes
  true blocks until the runner's 30s `agentTimeout` kills the process
  mid-command, which WEDGES the agent-browser daemon and cascades into
  `open ... exit status 1` / "daemon busy or unresponsive" failures across all
  later tests. This was the first (flaky) attempt; Go-side `eval` polling with
  a deadline is the fix. Never bound a browser wait by killing the command.
- `wait --load load` and `--load domcontentloaded` take ~25s on an
  already-loaded page; networkidle (~1.44s) was actually the fastest `--load`
  mode, so "just use a lighter load state" is not an option.
- Validate any change to web `WaitLoad` with multiple FULL-suite runs and a
  networkidle A/B baseline, not a single run. The suite has latent daemon
  flakiness that a single green run hides; the regression here showed up as
  9 failures in one run and 1 in the next.
- The init script must be registered before the daemon's first navigation; it
  rides on `agentEnv()`, which is used by the `open` that starts the daemon.

## Files

None recorded.
