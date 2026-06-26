# 994 -- geodns-3-observability-cli

## Context

The third geodns child ([[991-geodns-0-umbrella]], builds on [[993-geodns-2-server]]):
the operator surface around the server -- Prometheus metrics, a `show geodns`
status command, and a doctor check -- minus the reference's Sentry. Each surface is
owned by the geodns plugin so the "delete the folder" self-containment invariant holds.

## Decisions

- **Metrics via `ConfigureMetrics(reg any)` into the host registry**, stored in an
  atomic pointer with a lazy Nop default (so the query path never nil-checks and no
  `init()` is needed -- `init()` is banned outside register.go). Counters
  `ze_geodns_*`: request/response (by zone, bounded qtype, rcode), latency histogram,
  `listener_up{protocol,address}`, `config_reload_total{result}`. `qtypeLabel`
  collapses unknown types to `OTHER` to bound cardinality.
- **`show geodns` owner-owned** via container-merge: `yang/ze-geodns-cmd.yang`
  declares `container show { container geodns { ze:command "ze-show:geodns" } }`;
  the handler (`pluginserver.RegisterRPCs`) reads the same atomic snapshot the server
  uses, so status never drifts.
- **Doctor checks bind *capability*, not the live port.** `geodnsListenDiagnostic`
  warns only for an enabled geodns with a privileged-port (<1024) `listener` that
  fails a bind-probe; the default 5300 produces nothing, avoiding a false positive
  against the running listener. Cross-service port conflicts are detected separately
  by the `ze:listener` extension at parse time. Logic split from tree-navigation so
  it is unit-testable.

## Consequences

- The reference's folder-only counters (parse-error, duplicate-host, critical-file)
  are dropped -- YANG validation at commit makes them obsolete; `config_reload_total`
  replaces `zone_reload_total`.
- The doctor diagnostic code lives in the central `internal/core/diagnostic/codes.go`
  catalog (ldp/rsvpte convention); the central `show` self-containment guard gains a
  banned `ze-show:geodns` token, with the owner presence test in `geodns/yang`.

## Gotchas

- The RPC handler's `(*plugin.Response, error)` signature forces an always-nil error
  (`unparam`) -- `//nolint:unparam` with the reason is the accepted resolution.
- `resp.Data` is the `plugin.ResponseData` interface; assert to `plugin.Map` to index.
- `test/plugin/*.ci` observer tests dispatch through `dispatch-command`, and the command
  string MUST be the full CLI path INCLUDING the `show` prefix (`show geodns`, not
  `geodns`). Without `show`, `Dispatcher.Dispatch` fails `matchBuiltinTokens`, falls
  through to `dispatchPlugin` -> `routeToProcess`, and the call BLOCKS instead of
  returning -- the observer never reaches its assertions, so the test passes on the peer
  exchange + clean exit alone (a silent false pass). This bit `show-system-ntp.ci`
  (dispatched `system ntp`; fixed to `show system ntp`) and this plugin's first cut.
  With the correct command the observer's `runtime_fail` sentinel reaches the runner
  (no syslog directive -> `ze.log.backend` stays stderr -> `checkObserverSentinel` sees
  it), so a failing assertion fails the test (proven by a guaranteed-fail probe).
  `show geodns` is also covered by the `show_test.go` unit test. See [[996-observer-dispatch-show-prefix]].

## Files

- `internal/plugins/geodns/metrics.go`, `show.go`, `doctor.go` (+ tests)
- `internal/plugins/geodns/yang/ze-geodns-cmd.yang`, `yang/self_containment_test.go`
- `internal/core/diagnostic/codes.go`, `internal/component/cmd/show/yang/self_containment_test.go`
- `test/plugin/geodns-show.ci`, `test/ui/doctor-geodns.ci`
