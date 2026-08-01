# family-direction-policy

`bgp-filter-family` is a named-chain RPC filter (`bgp/policy/family-filter NAME { family; action }`,
referenced as `bgp-filter-family:NAME` in `filter { import/export }`). `remove` strips an
AFI/SAFI's MP_REACH/MP_UNREACH (dropping the UPDATE when that empties it); `tear-down`
(import only) sends a Cease/Connection-Rejected NOTIFICATION and closes the session.
Motivating case: a FlowSpec route reflector that never advertises `ipv4/flow` back to edge
peers (`export remove ipv4/flow`).

## What it took beyond the plugin

The plugin itself is a clone of `filter_remove_private_as` (wildcard `FilterDecl{Raw:true}`,
`PolicyFilterChain` -> `CallFilterUpdate`). Two reactor-level problems surfaced only when the
**export** direction was exercised end-to-end:

1. **Originated routes bypassed export filters entirely.** `forwardUpdateCore`
   (`reactor_api_forward.go`) runs the export chain only on *reflected* (received->forwarded)
   routes. Every **originated** route -- configured `update{}` / static, API/plugin injection,
   redistribute, adj-rib-in replay -- is written by the session via `writeUpdate` /
   `SendAnnounce`, which never consulted the export chain. Fix: a single egress gate
   (`reactor/egress_inject_filter.go` `exportFilterForBody`) called from `session.writeUpdate`
   and `SendAnnounce`; `writeRawUpdateBody` (the already-filtered forwarded path) is left alone
   so forwarded routes are not double-filtered.

2. **Multiprotocol EoR markers were being suppressed.** `message.Update.IsEndOfRIB()` only
   recognises the IPv4-unicast EoR (empty UPDATE). An MP-family EoR (FlowSpec, IPv6) is an
   `MP_UNREACH_NLRI` attribute with empty NLRI, so it read as "not an EoR" and the egress gate
   ran a `remove ipv4/flow` filter on it -> the EoR was dropped and the peer hung waiting for
   graceful-restart completion. Fix: `message.IsEndOfRIBAnyFamily()` (MP-aware) gates the
   egress filter. A `remove` filter removes *routes*, never EoR markers.

## Traps for the next agent

- **The route-send subsystem is fragmented.** `forwardUpdateCore` (reflect, raw, ->
  `SendRawUpdateBody`) and the originate paths (-> `writeUpdate`/`SendAnnounce`) are separate;
  there is no single egress chokepoint. If you add behaviour to "all outbound routes", you must
  cover both the forward path and the session write methods. (Route-refresh uses
  `SendRawMessage`; EoRs use `SendUpdate`->`writeUpdate`.)
- **Functional-test observability is a minefield** (this cost the most time):
  - The `.ci` runner **rebuilds `ze` itself** with `TestBuildTags()` (`ze_core ze_distro
    ze_setup` + the FlowSpec/test plugin tag + feature gates). A bare `go build` (no tags) or
    even `make ze`'s tag set produces a *different* binary -- your source edits may not be in
    the binary you think you are testing. Trust `make ze`/the runner, not ad-hoc `go build`.
  - On a **passing** test the runner shows nothing; `CLIENT OUTPUT` only appears on failure and
    is truncated to startup. `expect=stderr` greps the *full* client stderr, but reliably
    surfaces **plugin-relayed** logs, not core `slog`. The plugin stderr relay only flushes on
    **long-running / killed** runs, not fast/graceful exits -- so asserting a plugin log line
    (e.g. "filter-family remove (suppress)") is flaky. **Assert behaviour, not logs**: the
    export test rejects the suppressed FlowSpec NLRI and expects the delivered EoR.
  - ze-peer needs a *positive* expectation to stay alive; a `reject`-only peer block exits
    immediately ("no test data available"). Inject the suppressed route first, then a delivered
    EoR/route as the keep-alive + exit trigger; mind the seq-ordering when the first route is
    dropped.
- **ci-sleep ratchet**: observer scripts must use `api.wait_for_event(timeout=...)` /
  `wait_for_shutdown`, not `time.sleep` (`test/.ci-sleep-baseline`).

## Pre-existing blocker hit during closure

`make generate` regenerates `internal/component/plugin/all/all.go` to include the committed
`internal/component/aihelp` package, but `aihelp` imports `internal/component/cli/client`, which
closes an import cycle (`all -> aihelp -> cli/client -> ... -> plugin/all`). So HEAD's `all.go`
is intentionally stale (excludes aihelp to keep the build alive), and `make generate` /
`ze-plugin-imports-check` cannot produce a building tree until that cycle is broken. This spec's
`all.go` therefore adds only the two `filter_family` imports and does **not** run a full
`make generate`. Unrelated to family-direction-policy; flagged for the aihelp owner.

## Files

None recorded.
