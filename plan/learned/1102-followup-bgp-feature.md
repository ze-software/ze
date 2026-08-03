# 1102 -- followup-bgp-feature

## Context

Consolidation umbrella from the 2026-07-06 deferral triage covering seven BGP
protocol/policy/observability follow-ups across the GR, filter, decorator, and
Prometheus surfaces. Each item carried verified still-open evidence at triage.
The umbrella was worked item-by-item (smallest first), each item committed
separately with its own tests; the umbrella file tracked per-item state until
every row was done or re-deferred with a destination.

## Decisions

- Worked and committed per item rather than as one change set: the items were
  independent (GR metrics lifecycle, spy-registry behavioral tests, raw-filter
  default-originate guard, web decorators, Prometheus phase 6), so single-focus
  commits beat one umbrella commit.
- Item 2 chose reject-at-runtime over pre-encoding the synthetic default-route
  UPDATE: default-originate is a pure accept/reject gate and the codebase
  validates these refs at runtime, not config-load.
- Item 3 (AS-Confederation OTC, RFC 9234 §5) re-deferred unchanged by user
  decision 2026-07-08 over implementing it: ze is a single-AS speaker
  (`role.getLocalASN`, role.go) with no confederation-member config, so §5's
  confederation rules are vacuously satisfied and the existing OTC egress
  (otc.go, checkOTCEgress otc.go) is already correct; true confed OTC
  requires building confederation-member support first (future dedicated spec).
- Item 1 (GR advanced) split by RFC/subsystem to `plan/spec-gr-advanced.md`
  (hard-reset RFC 8538 + selection-deferral RFC 4724 §4.1); VPN ATTR_SET
  (RFC 6368) split further to a future L3VPN spec -- it was an L3VPN feature
  mis-bundled into a GR row.

## Consequences

- GR per-peer metric series are deleted on peer removal (reactor emits
  SessionStateDown/ReasonPeerRemoved; tombstone prevents racing re-activation).
- Prometheus counters now have behavioral spy tests driving real producers
  (reload, UPDATE receive, session flap, wire read) with exact-delta
  assertions, plus runtime/process collectors, AS-path-loop and ASPA metrics.
- raw=true filters fail closed on default-originate with an actionable warning.
- Web gained reverse-dns and community-name display decorators (registered in
  service_web.go; reverse-dns gated on resolvers.DNS).
- ze remains GR-Helper-only; restarting-speaker behaviour lives in
  spec-gr-advanced. Confederation support remains an explicit non-goal until a
  dedicated spec exists.

## Gotchas

- A community YANG leaf does not exist in the BGP schema, so the
  community-name decorator is registered but has no leaf wiring (recorded
  deferral at item 4).
- RED-verification for spy-metric tests was done by temporarily neutering the
  producer (session_read.go wire-bytes increment) and reverting -- count-only
  assertions would have passed vacuously.
- The umbrella sat "completed but not closed" for two days because per-item
  commits never triggered the closure flow; `spec-closure-check.py --list` was
  the surface that kept flagging it. Closure finally ran on explicit user
  instruction 2026-07-10.

## Files

- `internal/component/bgp/plugins/gr/` (gr.go, gr_removal_test.go, peer removal lifecycle)
- `internal/component/bgp/reactor/` (reactor_metrics.go, reactor_metrics_behavioral_test.go, default_originate_raw.go, peer_initial_sync_test.go)
- `internal/component/web/` (decorator_reverse_dns.go, decorator_community.go, service_web.go wiring)
- `internal/component/bgp/plugins/` (loop filter metrics, rpki ASPA metrics, runtime collectors)
- Destinations: `plan/spec-gr-advanced.md` (item 1), future L3VPN + confederation specs (items 1/3 remainders)
