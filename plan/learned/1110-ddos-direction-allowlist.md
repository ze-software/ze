# 1110 -- ddos direction + traffic policy (allow/deny)

## Context
Two DDoS-detection enhancements on the `ddos/*` plugin family. (1) **Direction**: classify
each attack's victim as local (an address the box terminates, netfilter INPUT hook) vs
remote (a downstream/transit host, FORWARD hook), surface it on events + incidents +
`show ddos`, and route mitigation by it (local INPUT drop, opt-in FORWARD drop, flowspec
upstream). (2) **Traffic policy**: replace the per-responder destination-only "allowlist"
leaves with one allow/deny policy on `ddos/detect` that adds source-IP exemption and a
per-rule detection-vs-mitigation scope. The originating problem: the old allowlist was
destination-only, mitigation-only, and duplicated across `local`/`flowspec`.

## Decisions
- **Detector is the single enforcement point; the event carries the decision** over each
  responder reading the policy. Plugins receive only their own config subtree
  (`reload.go`), so responders cannot read the detect-owned policy; the detector
  evaluates it once and encodes the outcome on the event (`SuppressMitigation` flag). This
  also lets `observe` record an incident while responders skip mitigation.
- **`SuppressMitigation` (not `Mitigate`)** so the bool zero value = mitigate = fail-safe
  (an unset field defends by default).
- **Longest-prefix-match, NOT first-match/config-order** over an ordered rule list. Plugin
  config is delivered as an unordered map keyed by prefix, so insertion order does not
  survive (see Gotchas); most-specific-wins makes a `/24 deny` beat a covering `/16 allow`
  with no order field. Ties resolve to deny (fail-safe). User-approved change from the spec.
- **New `iface.AddressIsLocal(addr)` helper** (netlink RTN_LOCAL / VPP `FIB_API_PATH_TYPE_LOCAL`)
  over extending `RouteLookup`'s result map: a dedicated "is this my address" is cleaner and
  backend-agnostic. `detect` already imports `iface` (tier-safe plugin->component).
- **Opt-in `ddos/local/forward-mitigation` (default false)** for the FORWARD-hook drop of
  transit victims, over always-on: avoids touching the forwarding plane unless asked.
- **Two-stage evaluation**: destination rules at the fast emit, source rules at
  characterization (sources are unknown at emit); the characterized event is authoritative
  and the local responder withdraws a fast-path drop if a source rule flips it to exempt.

## Consequences
- The `ddos/local` and `ddos/flowspec` `allowlist` leaves are REMOVED. A config still using
  them fails validation ("unknown field ... allowlist") -- the intended migration signal.
  Documented in `docs/guide/ddos-mitigation.md` (allowlist -> policy migration).
- `ddosevent.AttackDetected`/`AttackCharacterized` gained `Direction` + `SuppressMitigation`;
  `observe.incident` gained `Direction`. Responders obey the flag and never read the policy.
- New Prometheus counters `ze_ddos_policy_suppressed_total{scope}` and `ze_ddos_direction_total{direction}`.
- Source-based detection-scope suppression that only resolves at characterization downgrades
  to mitigation-suppression (the fast AttackDetected already opened the incident; it cannot
  be un-emitted). A documented two-stage limitation.

## Gotchas
- **Ordered YANG list order does NOT survive plugin config delivery.** The config Tree keeps
  order in a side-slice (`tree.go` `listOrder`) but plugins receive a plain
  `map[string]any` (JSON), which is unordered. `policyroute` (the exact ordered-by-user rule
  precedent) works around this with an explicit `order` leaf + `sort.Slice` (`config.go`).
  Caught in the implement audit BEFORE writing the parser; resolved by switching to
  longest-prefix-match. Validate the delivery format before committing to ordering semantics.
- **The `daemon.pid`/`daemon.ready` file handshake in `ddos-detect-mitigate.ci` does not
  work** (that test is marked "not been run under QEMU"). The working pattern for a ddos
  behavioral `.ci` is an in-daemon `ze_api` observer probe (5-stage handshake) that queries
  `show ddos ...` via `dispatch(api, ...)`, launched as a `plugin { external ... }` + a
  `bgp peer { process ... }` with a background `ze-peer` -- see `ddos-bps-amplification.ci`.
- **Loopback flood victim resolution needs a sink socket.** Flooding a loopback port with no
  listener makes the kernel emit ICMP port-unreachable backscatter, so trafficusage
  mis-resolves the victim (empty target -> direction defaults to remote). Bind a UDP sink on
  the victim `ip:port` (learned/1109). Without it, victim resolution is non-deterministic.
- **The firewall nft backend must be loaded (`firewall { backend nft }`) for the responder's
  `ApplyAll` to install a drop** -- otherwise "firewall backend not loaded". BUT loading it
  in a ddos `.ci` deadlocked the daemon's command dispatch under the flood (the firewall
  component and the ddos-local responder drive nft concurrently). The direction `.ci`
  therefore asserts the incident `direction=local` classification only; the local->INPUT vs
  remote->FORWARD hook selection is unit-tested (`TestLocalHookByDirection`).
- **Parallel ddos flood tests interfere on shared `lo`** -- each daemon's trafficusage sees
  every test's flood, confusing the dominant-victim resolution. Distinct victims avoid the
  sink bind-collision but not the trafficusage interference; the ddos flood tests must run
  serially (`-p 1`), which is how the suite runs them.
- **Under QEMU, `ze-peer` resolves to `$ZE_TEST_BIN` (default `bin/ze-test`).** A leftover
  host (darwin) `bin/ze-test` gives "exec format error"; `make ze-qemu-debug` sets `ZE_BIN`
  but not `ZE_TEST_BIN`, so pass `ZE_TEST_BIN=bin/ze-test-linux-<arch>` in `RUN`.

## Files
- `internal/core/ddosevent/event.go`: `Direction` type + `Direction`/`SuppressMitigation` fields.
- `internal/component/iface/{backend,dispatch}.go` + `internal/plugins/iface/{netlink/route_linux,netlink/backend_other,vpp/fib}.go`: `AddressIsLocal`.
- `internal/plugins/ddos/detect/{policy,config,characterize,detector,metrics}.go` + `yang/ze-ddos-detect-conf.yang`: policy model, two-stage eval, direction classify, counters.
- `internal/plugins/ddos/local/{config,match,responder}.go` + `yang`: forward-mitigation, direction-aware hook, honor SuppressMitigation, allowlist removed.
- `internal/plugins/ddos/flowspec/{config,match,responder}.go` + `yang`: honor SuppressMitigation + skip local, allowlist removed.
- `internal/plugins/ddos/observe/store.go`: `Direction` on incident.
- `test/parse/ddos-policy.ci`, `test/plugin/ddos-policy.ci`, `test/plugin/ddos-direction.ci`.
- `docs/guide/ddos-mitigation.md`, `ai/digests/flow-ddos.md`.
