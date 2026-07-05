# 1063 -- ownership-1-rs-invariant

## Context

Route-server (RS) fast-path forwarding was split across two owners: the reactor
natively forwarded RS UPDATEs on the hot path (`reactorForwardRS`, gated only by
the per-peer `RSFastPath` setting), while `plugins/rs/` owned the correctness-critical
lifecycle (peer-down withdrawal, replay-on-peer-up, filtered-peer delivery) and the
enabling config lived in core BGP YANG. Deleting the plugin folder left the hot
forwarding path alive but silently broke withdrawals/replay -- violating the project's
"delete the folder and the feature vanishes" invariant. Goal: make the reactor fast
path a plugin-owned capability and move the RS config surface under the plugin, with
zero perf regression (DESIGN-REVIEW finding #1, RS remnant).

## Decisions

- Chose B1 (plugin-activated capability via the BGP-owned `filterapi` leaf seam) over
  B2 (move `reactorForwardRS` into the plugin) because B2 would need the reactor's
  peer/session internals and direct `bufWriter` access, regressing perf. The rs plugin
  activates at `init()` (`filterapi.EnableRSForwarding()`), exactly like filter plugins
  register their chains; the reactor caches the bool once in `New()` and the gate adds
  one short-circuiting `&& r.rsForwardingEnabled`.
- Chose `filterapi` over a new `rscap` leaf or `registry.Has("bgp-rs")`: `filterapi` is
  the established reactor-capability seam already read at construction, reuses its
  Snapshot/Restore/ResetForTest test isolation, and passes a *value* not a plugin *name*
  (`registry.Has("bgp-rs")` would spell the plugin name in central code -- a
  self-containment violation).
- Chose to move only the YANG *schema* to the plugin (as `augment`s mirroring
  `filter_irr`), keeping the *parse* (map -> `PeerSettings`) in `reactor/config.go`,
  because `rs-client`/`rs-fast-path` drive reactor-owned `PeerSettings` fields consumed
  on the hot path and there is no seam for a plugin to contribute into `PeerSettings`.

## Consequences

- Deleting `plugins/rs/` removes the only `EnableRSForwarding()` caller AND the only
  registrar of the two YANG leaves -> the reactor fast path is inert and the config is
  rejected. The invariant is restored at COMPILE time (folder/blank-import), symmetric
  for code and config.
- The invariant is NOT runtime-gated: a standard `ze` binary always links the rs plugin,
  so the capability is on and the leaves validate whether or not a given config declares
  `use bgp-rs`. Per-config behavior stays gated by the `rs-fast-path` leaf as before.
- Any future reactor capability a BGP plugin must toggle should follow this pattern: a
  bool/value in `filterapi` set at plugin `init()`, cached by the reactor in `New()`,
  never a plugin-name string in reactor/central code.

## Gotchas

- Config-validation schema is a UNION of ALL init()-registered YANG modules
  (`config/yang_schema.go` -> `LoadRegistered`), with zero filtering by which plugins a
  config references. So moving a leaf into a plugin augment does NOT make it rejected
  when the plugin block is absent -- it only vanishes when the plugin is compiled out.
  This flipped assumption A-2 and meant the two `test/parse` RS configs did NOT need
  updating (they pass unchanged). Do not "fix" a parse test to add a plugin block on the
  theory that a moved leaf requires it.
- The reactor's own test binary does NOT link `plugins/rs`, so `filterapi.RSForwardingEnabled()`
  is naturally false there -- which is exactly the "plugin absent" condition. This makes
  the AC-2 inert test trivial (no folder deletion needed): just drive the gate and assert
  `msg.ReactorForwarded` stays false.
- `session_negotiate.go` also reads `RSFastPath` (PATHS-LIMIT suppression) but is NOT a
  forwarding path and needs no capability gate: it is config-driven and already inert
  when the plugin-owned leaf is absent.

## Files

- `internal/component/bgp/filterapi/filterapi.go` -- `EnableRSForwarding`/`RSForwardingEnabled` + snapshot/restore/reset
- `internal/component/bgp/filterapi/filterapi_test.go` -- capability tests
- `internal/component/bgp/reactor/reactor.go` -- `rsForwardingEnabled` field cached in `New()`
- `internal/component/bgp/reactor/reactor_notify.go` -- gate `&& r.rsForwardingEnabled`
- `internal/component/bgp/reactor/forward_rs_test.go` -- `TestNewReadsRSForwardingCapability`, `TestRSFastPathGateRespectsCapability`
- `internal/component/bgp/reactor/peer_initial_sync.go` -- drop "(always loaded)" comment
- `internal/component/bgp/plugins/rs/register.go` -- activate capability at `init()`
- `internal/component/bgp/plugins/rs/yang/ze-rs-conf.yang` -- adopt `rs-client`/`rs-fast-path` as augments
- `internal/component/bgp/plugins/rs/config_ownership_test.go` -- AC-3 ownership guard
- `internal/component/bgp/yang/ze-bgp-conf.yang` -- remove the two leaves
- `test/parse/bgp-rs-augment-paths.ci` -- (ze-review) guards all 6 augment paths (peer/group/group-peer x session/behavior); proven to fail if any augment is dropped
- `ai/patterns/registration.md` -- (ze-review) catalog the filterapi reactor-capability seam so the pattern is discoverable
