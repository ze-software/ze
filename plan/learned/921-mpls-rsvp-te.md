# 921 -- mpls-rsvp-te

## Context
Spec `mpls-3-rsvp-te` implements RSVP-TE (RFC 3209/2205): explicitly-routed MPLS
LSPs with bandwidth reservation. The engine, codec, admission control and FSM were
already built and unit-tested, but -- as with LDP -- the component never configured
through real config and `show rsvp-te ...` crashed. Feature gaps at closure: AC-5
RESV refresh, AC-6 link-failure handling, AC-9 ERO/RRO display.

## Decisions
- RESV soft-state refresh: egress/transit re-send RESV upstream on the refresh tick
  (`sendResv`), in addition to ingress PATH refresh. Egress refresh propagates because
  transit re-relays each received RESV, so the whole chain stays alive.
- Link failure → PathErr: subscribe to the iface component's `EventDown`, match LSPs
  by `AdmissionIface`, send a PathErr upstream (transit/egress) or emit a local
  path-err event (ingress head-end), then tear local state. Chosen because ze has no
  IGP -- the iface netlink monitor is the available link-state source. RFC 3209 error
  code 24 / value 5 ("No route available toward destination").
- RRO collection: each node prepends its address to the RRO as the RESV travels
  upstream (RFC 3209 §4.4); the head-end's RSB then holds the full path for
  `show rsvp-te session`.

## Consequences
- LSP soft-state survives when only the ingress refreshes stop; a failed link now
  actively signals the head-end instead of leaving a black-holed LSP "up".
- `show rsvp-te session` exposes ERO (configured) and RRO (recorded) paths.
- `link-failure` handling runs on the iface-event goroutine; it relies on the LSP
  table / admission controller already being concurrency-safe (the cleanup loop
  mutates them off the engine goroutine too).

## Gotchas
- Same plugin-config-shape trap as LDP: root-wrapped, string-typed numbers, and
  **keyed-map** YANG lists (`interface`/`tunnel`/`explicit-route` arrive as maps keyed
  by name/index, not arrays) -- ERO hops must be re-sorted by numeric index to preserve
  order. A parser reading the wrong shape leaves the engine idle; pin the shape in a test.
- Same show-proxy recursion trap (use `PluginCommand` + `ForwardToPlugin`).
- **AC-12 cannot be satisfied as written: FRR ships no `rsvpd` daemon.** "Interop with
  FRR rsvpd" is impossible; coverage is the engine unit tests, and multi-node ze-to-ze
  interop is deferred. Verify a named interop peer actually exists before writing an AC.
- Link-failure matches LSPs by `AdmissionIface`; an LSP whose admission was skipped
  (no interface `address` prefix to resolve) has an empty `AdmissionIface` and is not
  caught -- a known limitation if precise NextHop→iface resolution is later needed.
- The RSVP-TE data plane (the kernel-touching part of LSP setup) IS QEMU-verified:
  `busFIB.ProgramSwap`/`ProgramPop` emit `mplsfibevents` entries that
  `mplsentry_integration_linux_test.go` (Swap, PopWithNextHop, EgressPopNoNextHop)
  programs into a live kernel and reads back. Those tests assert exactly the entries
  the RSVP-TE egress/transit paths emit (the EgressPop case is the no-via ultimate-hop
  pop). What stays unit-only is multi-node PATH/RESV signaling (no FRR rsvpd to peer
  with, AC-12); the FIB output is not unit-only.

## Files
- `internal/component/rsvpte/engine.go` (sendResv, handleLinkDown, RRO recording), `register.go` (parser fix, refreshPaths, link-down subscription, show ERO/RRO), `build.go` (RRO encode, ErrValueNoRouteAvailable), `rro.go` (+test), `doctor*.go` (+tests), `cmd_show.go` (+test), `config_test.go`, `refresh_rro_test.go`, `linkdown_test.go`
- `internal/core/diagnostic/codes.go` (`doctor-rsvpte-rawsock-unavailable`)
- `internal/test/cli/register.go` (`ze-test rsvpte`); `test/rsvpte/*.ci`
