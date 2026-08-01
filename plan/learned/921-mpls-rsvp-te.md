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
- **Reload reconciliation needs OnConfigApply, like LDP (920).** OnConfigure is
  startup-only; on reload only OnConfigVerify (stash pending) and OnConfigApply (commit)
  fire. RSVP-TE originally set up tunnels only in OnStarted, so on reload an added tunnel
  did not signal, a changed ERO never rerouted, and a REMOVED tunnel leaked its LSP and
  FIB state. `reconcileTunnels` (called from OnStarted and OnConfigApply) sets up the new
  set and tears down removed tunnels via the head-end `teardownLSP` (the generalized,
  renamed `tearReplaced`). Without OnConfigApply the head-end teardown/reroute paths are
  effectively unreachable -- so the teardown interop test exercises a real production path.
- **AC-12 cannot be satisfied as written: FRR ships no `rsvpd` daemon.** No actively
  maintained open-source daemon implements RSVP-TE signaling (FRR has ldpd not rsvpd;
  BIRD/GoBGP do BGP-signaled MPLS only), so true cross-vendor interop needs a proprietary
  lab container (Juniper cRPD, Cisco IOS XRd, Arista cEOS). Verify a named interop peer
  actually exists before writing an AC. Multi-node ze-to-ze signaling, however, IS now
  covered by `interop_test.go`: it wires two to four real engines through an in-memory
  fabric so each engine's own encoded PATH/RESV/PathErr/PathTear is decoded and acted on
  by the peer -- a direct LSP both ways, a three-node ingress/transit/egress LSP (label
  stack push->swap->pop, RRO across hops), a PathErr rejection, hop-by-hop teardown,
  soft-state refresh (2- and 3-node), admission denial (single- and multi-interface), and
  a make-before-break reroute (direct and config-reload-triggered) -- the fully-open
  substitute for the missing peer. A -race test (register_test.go) proves reconcileTunnels
  is safe to run concurrently with live signaling (the OnConfigApply-vs-run-loop concurrency).
- Link-failure matches LSPs by `AdmissionIface`; an LSP whose admission was skipped
  (no interface `address` prefix to resolve) has an empty `AdmissionIface` and is not
  caught -- a known limitation if precise NextHop→iface resolution is later needed.
- The RSVP-TE data plane (the kernel-touching part of LSP setup) IS QEMU-verified:
  `busFIB.ProgramSwap`/`ProgramPop` emit `mplsfibevents` entries that
  `mplsentry_integration_linux_test.go` (Swap, PopWithNextHop, EgressPopNoNextHop)
  programs into a live kernel and reads back. Those tests assert exactly the entries
  the RSVP-TE egress/transit paths emit (the EgressPop case is the no-via ultimate-hop
  pop). Multi-node PATH/RESV signaling is covered by the ze-to-ze `interop_test.go`
  (one engine's encoder feeding another's decoder); only cross-VENDOR interop remains
  out of reach, for lack of any open-source RSVP-TE peer.

## Files
- `internal/plugins/rsvpte/engine.go` (sendResv, handleLinkDown, RRO recording), `register.go` (parser fix, refreshPaths, link-down subscription, show ERO/RRO), `build.go` (RRO encode, ErrValueNoRouteAvailable), `rro.go` (+test), `doctor*.go` (+tests), `cmd_show.go` (+test), `config_test.go`, `refresh_rro_test.go`, `linkdown_test.go`, `interop_test.go` (ze-to-ze interop: setup/transit/patherr/teardown/refresh/admission/reroute), `register_test.go` (tunnel reconcile + concurrent-with-signaling race); `reroute.go` (`teardownLSP`); register.go `reconcileTunnels`/`tunnelKey` + OnConfigApply reload reconciliation
- `internal/core/diagnostic/codes.go` (`doctor-rsvpte-rawsock-unavailable`)
- `internal/test/cli/register.go` (`ze-test rsvpte`); `test/rsvpte/*.ci`
