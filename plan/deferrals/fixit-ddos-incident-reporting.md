# Deferrals: fixit-ddos-incident-reporting

One issue per row, recorded not fixed (owner instruction, 2026-08-15). The
aggregate live backlog is folded on read from `plan/deferrals/` by `/ze-status`.
Nothing stores it (`ai/rules/planning.md`).

**Issue:** the ddos incident-reporting plugin loses whole incidents, leaves
incidents open forever on the remote API, and never sends the heartbeat it defines

Naming note: the rows below identify code by SYMBOL rather than by package path,
because the package directory carries a product name the owner has asked to keep
out of these records. Every symbol named is unique in the tree, so `gopls` or a
plain grep resolves each one.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-15 | ad-hoc (ddos detect audit) | **One failed `openIncident` silently discards the entire incident, start to end.** `runEngine` (the reporting plugin's registration file) assigns its `activeUUID` closure variable only after `openIncident` returns success; on error it logs `log.Warn` and returns. Both later handlers guard on `activeUUID == ""` and no-op, so the Ongoing updates and the Cleared resolve are discarded too. `(*client).post` is fire-and-forget: on a transport error or status >= 400 it records a circuit-breaker failure and returns, with no buffer and no re-queue anywhere in the package. A single transport blip during the opening seconds of an attack therefore costs the whole incident, and the operator sees one WARN line. | Not a defect in detection: the detector still detects, mitigates and emits its own events, and this plugin is a reporting sink. The fix is a durable outbound path (bounded queue plus flush, or deferred open on the first successful post), which is its own design decision about ordering and bounding, not a line fix. | `plan/spec-fixit-ddos-incident-outbound-durability.md` | deferred <!-- doc-links: ignore (the destination spec is written when the deferral is taken up) --> |
| 2026-08-15 | ad-hoc (ddos detect audit) | **An open incident is never resolved when the detector is disabled or the daemon stops.** Two independent breaks, either sufficient. (a) The detector's teardown is `(*detector).Stop` (`internal/plugins/ddos/detect/detector.go`): it sets `stopped`, cancels, waits, and calls `saveBaseline`. It never calls `emitCleared`, which is assigned only to `d.sm.OnCleared` (`detector.go`) and so fires only on a rate transition through the state machine. No `AttackCleared` is published, so the reporting plugin's Cleared handler never runs. (b) The one path that would close it, the reporting plugin's own `OnConfigApply`, does call `resolveIncident` first, but it is not invoked for this diff: `(*Server).reloadConfig` (`internal/component/plugin/server/reload.go`) selects plugins by strict root-prefix match on declared roots, and the reporting plugin declares a single `ConfigRoots` entry for its own subtree, so a diff under `ddos/detect` reaches it with zero sections. Same class, third instance: `runEngine`'s exit path calls `unsubscribe()` only, so a clean daemon stop with an attack in progress also leaves the incident open. | Remote state leak rather than a local one: ze frees its own memory correctly. Fixing (a) means deciding whether teardown synthesises a Cleared or whether an incident is resolved by a separate lifecycle hook, which changes what every `ddosevent` consumer sees on reconfigure, not only this plugin. | `plan/spec-fixit-ddos-incident-lifecycle-on-teardown.md` | deferred <!-- doc-links: ignore (the destination spec is written when the deferral is taken up) --> |
| 2026-08-15 | ad-hoc (ddos detect audit) | **`(*client).heartbeat` has no non-test caller.** It is defined in the reporting plugin's client and called only from that client's `_test.go`. ze therefore never reports liveness, baseline readiness, or circuit-breaker state, so a node whose detector has silently stopped is indistinguishable from a node with no attacks. The `circuitBreaker` type itself works and counts genuinely consecutive failures, opening at 5 and clearing after 60s with no half-open probe; nothing publishes its state. | Dead code rather than wrong code, so nothing observable is currently incorrect. Wiring it needs a tick source in `runEngine` and a decision on cadence and payload, which is a small feature rather than a fix. | `plan/spec-fixit-ddos-incident-heartbeat-wiring.md` | deferred <!-- doc-links: ignore (the destination spec is written when the deferral is taken up) --> |
| 2026-08-15 | ad-hoc (ddos detect audit) | **UNVERIFIED, needs its own read.** `activeUUID`, `activeFamily`, `activeConfidence` and the `activePeak*` closure variables in `runEngine` are mutated from event-bus callbacks and from `OnConfigApply` with no mutex. Whether that is a live data race depends on whether `ddosevent` delivers on more than one goroutine, and on whether apply can overlap delivery. The `ddosevent` delivery path was not read. | Recorded as a question, not a finding (`ai/rules/evidence.md`): claiming a race without reading the producer would be fabrication. Cheap to settle by reading the bus's delivery goroutines, and `go test -race` over the package plus a concurrent apply would confirm or clear it. **SETTLED AND FIXED 2026-09-02: it was a LIVE race.** The read the row asked for: `Event[T].Emit` (`internal/core/events/typed.go`) delivers to an engine subscriber synchronously on the publisher's goroutine, and the detector publishes from two of them, its trafficstat rate tick and the characterization goroutine `(*detector).onAttackStart` starts. Those two are ordered against each other by the detector's own `emitMu` and by nothing else. `OnConfigApply` arrives on a third goroutine, the SDK reader loop `(*Plugin).eventLoop` (`pkg/plugin/sdk/sdk_dispatch.go`), which invokes the handler inline. So an apply and a delivery were unordered on every one of the closure variables, and the sharp case was not a torn read: an apply could resolve the incident and clear `activeUUID` between the Ongoing handler's `activeUUID != ""` test and its post, sending an update for an incident the remote side had closed, or swap the client under a handler already using it. The fix gives the state an owner: `reporter` (`internal/plugins/ddos/flowtriq/reporter.go`) holds it behind one mutex, taken for the WHOLE of each of the five writers rather than around a copy, because a copy-under-lock shape leaves the test-then-post window open. `ApplyBudget` moved 10 to 20 seconds, since an apply now waits for a post already in flight and the HTTP client caps one at 10. `TestReporterStateIsOrderedAgainstAConfigApply` drives the four callbacks against an apply from five goroutines and is RED under `-race` with the locks removed, measured through a `go test -overlay` build so no unguarded source ever reached the shared checkout. `TestReporterApplyResolvesTheOpenIncident` holds the resolve-before-swap order. | fixed in place, no destination spec needed | done <!-- doc-links: ignore (the destination spec is written when the deferral is taken up) --> |

## Detail

The first three rows are one coherent fix unit and should be specced together:
all three are about what ze tells the remote API, and all three fail in the same
direction, which is silence.

**Why the open-failure row is the sharpest.** The guard that discards the
incident is correct in isolation. Sending an Ongoing or a Resolve for an incident
the remote side never opened would be worse than sending nothing. The defect is
upstream of the guard: there is no path by which a failed open is ever retried,
so `activeUUID` cannot become non-empty later in the same attack. Any fix has to
choose where durability lives. A bounded outbound queue drained by the existing
circuit breaker keeps the ordering guarantee (open before update before resolve)
and gives the bound the breaker already implies. Deferring the open until the
first successful post is cheaper but changes the remote start timestamp, which
the incident record is presumably keyed on.

**Bounding is not optional.** ze currently has no outbound buffer at all, which
is why it has no unbounded-growth risk. Any queue added here inherits that risk
during a long API outage, so the bound belongs in the spec, not in review.

**What is already right, and must survive the fix.** The circuit breaker counts
consecutive failures and resets on any 2xx or 3xx, which is the behaviour a naive
counter gets wrong by never resetting on success. `(*client).post` caps error
bodies with a 1 KiB `io.LimitReader`, so a hostile or broken API cannot balloon a
log line. The HTTP client carries a 10s timeout. None of these should be
disturbed.

**Test surface that exists today.** `test/plugin/ddos-incident-confidence.ci` and
the other twelve `ddos-*.ci` fixtures cover the detector and its responders;
`./le functional plugin` is the owning gate. There is no functional fixture that
drives the reporting client against a failing API, so the durability fix needs a
new one that asserts an incident survives a failed open, which is the assertion
that would fail today.
